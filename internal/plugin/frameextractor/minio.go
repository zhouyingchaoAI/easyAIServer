package frameextractor

import (
	"bytes"
	"context"
	"easydarwin/internal/conf"
	"fmt"
	"log/slog"
	"net/http"
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

	// 配置自定义的 HTTP Transport 以解决 502 错误
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:    cfg.UseSSL,
		Transport: transport,
		Region:    "",
	})
	if err != nil {
		return fmt.Errorf("failed to create minio client: %w", err)
	}

	// check bucket exists, create if not (增加重试机制)
	var exists bool
	maxRetries := 3
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		exists, err = client.BucketExists(ctx, cfg.Bucket)
		cancel()

		if err == nil {
			break
		}

		if i < maxRetries-1 {
			s.log.Warn("minio bucket check failed, retrying...",
				slog.Int("attempt", i+1),
				slog.Int("max_retries", maxRetries),
				slog.String("error", err.Error()))
			time.Sleep(retryDelay)
			retryDelay *= 2 // 指数退避
		}
	}

	if err != nil {
		return fmt.Errorf("failed to check minio bucket after %d retries: %w", maxRetries, err)
	}

	if !exists {
		s.log.Info("creating minio bucket", slog.String("bucket", cfg.Bucket))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		
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
	
	s.log.Info("minio client initialized",
		slog.String("endpoint", cfg.Endpoint),
		slog.String("bucket", cfg.Bucket),
		slog.Bool("bucket_exists", exists))
	return nil
}

// createMinioPath creates a placeholder object to ensure the path exists in MinIO
func (s *Service) createMinioPath(task conf.FrameExtractTask) error {
	if s.minio == nil {
		return fmt.Errorf("minio not initialized")
	}
	
	// 目录结构：任务类型/任务ID/
	taskType := task.TaskType
	if taskType == "" {
		taskType = "未分类"
	}
	
	// create a .keep file to establish the path
	// use forward slashes for MinIO paths (S3 convention)
	key := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID, ".keep"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	content := []byte(fmt.Sprintf("Task: %s\nType: %s\nCreated: %s\n", task.ID, taskType, time.Now().Format(time.RFC3339)))
	_, err := s.minio.client.PutObject(ctx, s.minio.bucket, key, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{
		ContentType: "text/plain",
	})
	if err != nil {
		return err
	}
	
	s.log.Info("created minio path", slog.String("task", task.ID), slog.String("type", taskType), slog.String("key", key))
	return nil
}

// deleteMinioPath removes all objects under the task's path
func (s *Service) deleteMinioPath(task conf.FrameExtractTask) error {
	if s.minio == nil {
		return fmt.Errorf("minio not initialized")
	}
	
	// 目录结构：任务类型/任务ID/
	taskType := task.TaskType
	if taskType == "" {
		taskType = "未分类"
	}
	
	// use forward slashes for S3/MinIO
	prefix := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID)) + "/"
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
	
	s.log.Info("deleted minio path", slog.String("task", task.ID), slog.String("type", taskType), slog.String("prefix", prefix), slog.Int("objects", count))
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
				// 目录结构：任务类型/任务ID/
				taskType := task.TaskType
				if taskType == "" {
					taskType = "未分类"
				}
				// use forward slashes for MinIO/S3 paths
				key := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID, fmt.Sprintf("%s.jpg", ts)))
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, err = s.minio.client.PutObject(ctx, s.minio.bucket, key, &frame, int64(frame.Len()), minio.PutObjectOptions{
					ContentType: "image/jpeg",
				})
				cancel()
				if err != nil {
					s.log.Warn("minio upload failed", slog.String("task", task.ID), slog.String("key", key), slog.String("err", err.Error()))
				} else {
					s.log.Debug("uploaded snapshot", slog.String("task", task.ID), slog.String("key", key), slog.Int("size", frame.Len()))
					
					// 检查并清理超出限制的旧图片（带限流控制）
					maxCount := getMaxFrameCount(task, s.cfg)
					if maxCount > 0 && s.shouldCleanup(task.ID) {
						// 异步清理，避免阻塞上传流程
						go func(t conf.FrameExtractTask, max int) {
							if err := s.cleanupOldFrames(t, max); err != nil {
								s.log.Warn("cleanup failed", slog.String("task", t.ID), slog.String("err", err.Error()))
							}
						}(task, maxCount)
					}
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

// cleanupOldFrames 清理超出数量限制的旧图片
// 保留最新的maxCount张图片，删除更早的图片
func (s *Service) cleanupOldFrames(task conf.FrameExtractTask, maxCount int) error {
	if s.minio == nil {
		return fmt.Errorf("minio not initialized")
	}
	
	if maxCount <= 0 {
		return nil // 0表示不限制
	}
	
	// 构建任务目录前缀
	taskType := task.TaskType
	if taskType == "" {
		taskType = "未分类"
	}
	prefix := filepath.ToSlash(filepath.Join(s.minio.base, taskType, task.ID)) + "/"
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// 列出所有图片文件（排除.keep和algo_config.json等非图片文件）
	objectCh := s.minio.client.ListObjects(ctx, s.minio.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	
	// 收集所有jpg图片及其时间戳
	type objectInfo struct {
		key     string
		lastMod time.Time
	}
	var objects []objectInfo
	
	for object := range objectCh {
		if object.Err != nil {
			s.log.Warn("list object error during cleanup", slog.String("err", object.Err.Error()))
			continue
		}
		
		// 只处理.jpg文件，排除preview_*.jpg和其他特殊文件
		basename := filepath.Base(object.Key)
		if filepath.Ext(object.Key) == ".jpg" && 
		   len(basename) > 8 && 
		   basename[:8] != "preview_" &&
		   basename != ".keep" {
			objects = append(objects, objectInfo{
				key:     object.Key,
				lastMod: object.LastModified,
			})
		}
	}
	
	// 如果数量未超限，无需清理
	if len(objects) <= maxCount {
		return nil
	}
	
	// 按时间排序（从旧到新）
	// 使用简单的冒泡排序
	for i := 0; i < len(objects)-1; i++ {
		for j := 0; j < len(objects)-i-1; j++ {
			if objects[j].lastMod.After(objects[j+1].lastMod) {
				objects[j], objects[j+1] = objects[j+1], objects[j]
			}
		}
	}
	
	// 删除最旧的图片（保留最新的maxCount张）
	deleteCount := len(objects) - maxCount
	deletedCount := 0
	
	for i := 0; i < deleteCount; i++ {
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.minio.client.RemoveObject(deleteCtx, s.minio.bucket, objects[i].key, minio.RemoveObjectOptions{})
		deleteCancel()
		
		if err != nil {
			s.log.Warn("failed to delete old frame", 
				slog.String("task", task.ID),
				slog.String("key", objects[i].key),
				slog.String("err", err.Error()))
		} else {
			deletedCount++
			s.log.Debug("deleted old frame", 
				slog.String("task", task.ID),
				slog.String("key", objects[i].key))
		}
	}
	
	if deletedCount > 0 {
		s.log.Info("cleaned up old frames", 
			slog.String("task", task.ID),
			slog.Int("deleted", deletedCount),
			slog.Int("remaining", len(objects)-deletedCount),
			slog.Int("limit", maxCount))
	}
	
	return nil
}

// getMaxFrameCount 获取任务的最大图片数量限制
// 优先使用任务级配置，如果为0则使用全局配置
func getMaxFrameCount(task conf.FrameExtractTask, cfg *conf.FrameExtractorConfig) int {
	if task.MaxFrameCount > 0 {
		return task.MaxFrameCount
	}
	return cfg.MaxFrameCount
}

// shouldCleanup 判断是否应该触发清理（限流控制）
// 规则：每上传50张图片或距离上次清理超过5分钟时触发清理
func (s *Service) shouldCleanup(taskID string) bool {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	
	counter, exists := s.cleanupCounters[taskID]
	if !exists {
		counter = &cleanupCounter{
			uploadCount: 0,
			lastCleanup: time.Time{},
		}
		s.cleanupCounters[taskID] = counter
	}
	
	counter.uploadCount++
	
	// 每50张图片触发一次清理
	const cleanupThreshold = 50
	// 或者距离上次清理超过5分钟
	const cleanupInterval = 5 * time.Minute
	
	now := time.Now()
	if counter.uploadCount >= cleanupThreshold || 
	   (counter.lastCleanup.IsZero() == false && now.Sub(counter.lastCleanup) >= cleanupInterval) {
		counter.uploadCount = 0
		counter.lastCleanup = now
		return true
	}
	
	return false
}
