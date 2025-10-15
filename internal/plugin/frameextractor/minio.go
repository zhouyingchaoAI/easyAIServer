package frameextractor

import (
	"bytes"
	"context"
	"easydarwin/internal/conf"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioClient struct {
	client *minio.Client
	bucket string
	base   string
}

func (s *Service) initMinio() error {
	cfg := s.cfg.MinIO
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return fmt.Errorf("minio endpoint and bucket required")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return err
	}

	// check bucket exists, create if not
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return err
	}
	if !exists {
		s.log.Info("creating minio bucket", slog.String("bucket", cfg.Bucket))
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket %s: %w", cfg.Bucket, err)
		}
		s.log.Info("minio bucket created successfully", slog.String("bucket", cfg.Bucket))
	}

	s.minio = &minioClient{
		client: client,
		bucket: cfg.Bucket,
		base:   cfg.BasePath,
	}
	return nil
}

// createMinioPath creates a placeholder object to ensure the path exists in MinIO
func (s *Service) createMinioPath(task conf.FrameExtractTask) error {
	if s.minio == nil {
		return fmt.Errorf("minio not initialized")
	}
	
	// create a .keep file to establish the path
	// use forward slashes for MinIO paths (S3 convention)
	key := filepath.ToSlash(filepath.Join(s.minio.base, task.OutputPath, ".keep"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	content := []byte(fmt.Sprintf("Task: %s\nCreated: %s\n", task.ID, time.Now().Format(time.RFC3339)))
	_, err := s.minio.client.PutObject(ctx, s.minio.bucket, key, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{
		ContentType: "text/plain",
	})
	if err != nil {
		return err
	}
	
	s.log.Info("created minio path", slog.String("task", task.ID), slog.String("key", key))
	return nil
}

// deleteMinioPath removes all objects under the task's path
func (s *Service) deleteMinioPath(task conf.FrameExtractTask) error {
	if s.minio == nil {
		return fmt.Errorf("minio not initialized")
	}
	
	// use forward slashes for S3/MinIO
	prefix := filepath.ToSlash(filepath.Join(s.minio.base, task.OutputPath)) + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// list and remove all objects with this prefix
	objectCh := s.minio.client.ListObjects(ctx, s.minio.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	
	count := 0
	for object := range objectCh {
		if object.Err != nil {
			s.log.Warn("list object error", slog.String("err", object.Err.Error()))
			continue
		}
		
		if err := s.minio.client.RemoveObject(ctx, s.minio.bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			s.log.Warn("remove object error", slog.String("key", object.Key), slog.String("err", err.Error()))
			continue
		}
		count++
	}
	
	s.log.Info("deleted minio path", slog.String("task", task.ID), slog.String("prefix", prefix), slog.Int("objects", count))
	return nil
}

func (s *Service) runMinioSinkLoopCtx(task conf.FrameExtractTask, stop <-chan struct{}) {
	defer s.wg.Done()

	if s.minio == nil {
		s.log.Error("minio not initialized", slog.String("task", task.ID))
		return
	}

	minBackoff := 1 * time.Second
	maxBackoff := 30 * time.Second
	backoff := minBackoff

	for {
		select {
		case <-s.stop:
			return
		case <-stop:
			return
		default:
		}

		// build and start continuous ffmpeg snapshotter
		args := buildContinuousArgs("", "", getIntervalMs(task, s.cfg))
		// override output to stdout (we'll capture and upload)
		args = []string{
			"-y", "-hide_banner", "-loglevel", "error",
			"-rtsp_transport", "tcp",
			"-stimeout", "5000000",
			"-i", task.RtspURL,
			"-vf", fmt.Sprintf("fps=1/%.6f", float64(getIntervalMs(task, s.cfg))/1000.0),
			"-f", "image2pipe",
			"-vcodec", "mjpeg",
			"pipe:1",
		}
		ff := getFFmpegPath()
		cmd := exec.Command(ff, args...)
		
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			s.log.Error("failed to get stdout", slog.String("task", task.ID), slog.String("err", err.Error()))
			time.Sleep(backoff)
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			s.log.Error("start ffmpeg failed", slog.String("task", task.ID), slog.String("err", err.Error()))
			t := time.NewTimer(backoff)
			select {
			case <-s.stop:
				t.Stop()
				return
			case <-stop:
				t.Stop()
				return
			case <-t.C:
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		// read frames and upload
		go func() {
			buf := make([]byte, 1024*1024) // 1MB buffer for JPEG
			for {
				// read JPEG marker (FF D8)
				_, err := stdout.Read(buf[:2])
				if err != nil {
					return
				}
				if buf[0] != 0xFF || buf[1] != 0xD8 {
					continue
				}

				// read until JPEG end marker (FF D9)
				var frame bytes.Buffer
				frame.Write(buf[:2])
				for {
					n, err := stdout.Read(buf[:1])
					if err != nil {
						return
					}
					frame.WriteByte(buf[0])
					if n > 0 && buf[0] == 0xD9 && frame.Len() > 2 && frame.Bytes()[frame.Len()-2] == 0xFF {
						break
					}
				}

				// upload frame
				ts := time.Now().Format("20060102-150405.000")
				// use forward slashes for MinIO/S3 paths
				key := filepath.ToSlash(filepath.Join(s.minio.base, task.OutputPath, fmt.Sprintf("%s.jpg", ts)))
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, err = s.minio.client.PutObject(ctx, s.minio.bucket, key, &frame, int64(frame.Len()), minio.PutObjectOptions{
					ContentType: "image/jpeg",
				})
				cancel()
				if err != nil {
					s.log.Warn("minio upload failed", slog.String("task", task.ID), slog.String("key", key), slog.String("err", err.Error()))
				} else {
					s.log.Debug("uploaded snapshot", slog.String("task", task.ID), slog.String("key", key), slog.Int("size", frame.Len()))
				}
			}
		}()

		procDone := make(chan error, 1)
		go func() { procDone <- cmd.Wait() }()
		select {
		case <-s.stop:
			_ = cmd.Process.Kill()
			<-procDone
			return
		case <-stop:
			_ = cmd.Process.Kill()
			<-procDone
			return
		case err := <-procDone:
			if err != nil {
				s.log.Warn("ffmpeg exited", slog.String("task", task.ID), slog.String("err", err.Error()), slog.String("stderr", truncate(stderr.String(), 512)))
			} else {
				s.log.Warn("ffmpeg exited normally", slog.String("task", task.ID))
			}
			t := time.NewTimer(backoff)
			select {
			case <-s.stop:
				t.Stop()
				return
			case <-stop:
				t.Stop()
				return
			case <-t.C:
			}
			backoff = nextBackoff(backoff, maxBackoff)
		}
	}
}

