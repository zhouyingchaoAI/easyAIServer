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
	"sort"
	"sync"
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
	// 数据读取超时：如果60秒内没有读取到数据，重新连接
	dataTimeout := 60 * time.Second

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

		s.log.Info("ffmpeg started, connecting to RTSP stream", 
			slog.String("task", task.ID), 
			slog.String("rtsp", task.RtspURL),
			slog.Duration("backoff", backoff))
		// 成功启动后重置backoff（如果之前有失败）
		backoff = minBackoff

		// 用于通知主循环stdout读取失败或超时
		readError := make(chan error, 1)
		lastFrameTime := time.Now()
		var lastFrameTimeMu sync.Mutex

		// read frames and upload
		go func() {
			buf := make([]byte, 1024*1024) // 1MB buffer for JPEG
			for {
				// read JPEG marker (FF D8)
				_, err := stdout.Read(buf[:2])
				if err != nil {
					s.log.Warn("stdout read failed, will reconnect", 
						slog.String("task", task.ID), 
						slog.String("err", err.Error()))
					readError <- err
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
						s.log.Warn("stdout read failed while reading frame, will reconnect", 
							slog.String("task", task.ID), 
							slog.String("err", err.Error()))
						readError <- err
						return
					}
					frame.WriteByte(buf[0])
					if n > 0 && buf[0] == 0xD9 && frame.Len() > 2 && frame.Bytes()[frame.Len()-2] == 0xFF {
						break
					}
				}

				// 更新最后读取时间
				lastFrameTimeMu.Lock()
				lastFrameTime = time.Now()
				lastFrameTimeMu.Unlock()

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
					
					// 记录抽帧成功（用于计算每秒抽帧数量）
					s.recordFrameExtracted()
					
					// 检查并清理超出限制的旧图片（带限流控制）
					maxCount := getMaxFrameCount(task, s.cfg)
					if maxCount > 0 {
						// 如果配置了限制，检查是否需要立即清理（图片数量可能已超过限制）
						shouldCleanupNow := s.shouldCleanup(task.ID)
						// 如果超过限制，立即触发清理（不等待限流）
						if shouldCleanupNow || s.shouldForceCleanup(task.ID, maxCount) {
						// 异步清理，避免阻塞上传流程
						go func(t conf.FrameExtractTask, max int) {
							if err := s.cleanupOldFrames(t, max); err != nil {
								s.log.Warn("cleanup failed", slog.String("task", t.ID), slog.String("err", err.Error()))
							}
						}(task, maxCount)
					}
					}
				}
			}
		}()

		// 超时检测goroutine
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					lastFrameTimeMu.Lock()
					lastFrame := lastFrameTime
					lastFrameTimeMu.Unlock()
					if time.Since(lastFrame) > dataTimeout {
						s.log.Warn("no data received for too long, will reconnect", 
							slog.String("task", task.ID),
							slog.Duration("timeout", dataTimeout),
							slog.Duration("since_last_frame", time.Since(lastFrame)))
						// 杀死FFmpeg进程以触发重连
						_ = cmd.Process.Kill()
						readError <- fmt.Errorf("data timeout: no frame received for %v", time.Since(lastFrame))
						return
					}
				case <-s.stop:
					return
				case <-stop:
					return
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
		case err := <-readError:
			// stdout读取失败或超时，杀死进程并重连
			_ = cmd.Process.Kill()
			<-procDone
			s.log.Info("reconnecting due to read error or timeout", 
				slog.String("task", task.ID), 
				slog.String("err", err.Error()),
				slog.Duration("backoff", backoff))
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
		case err := <-procDone:
			// FFmpeg进程退出
			if err != nil {
				s.log.Warn("ffmpeg exited, will reconnect", 
					slog.String("task", task.ID), 
					slog.String("err", err.Error()), 
					slog.String("stderr", truncate(stderr.String(), 512)))
			} else {
				s.log.Warn("ffmpeg exited normally, will reconnect", slog.String("task", task.ID))
			}
			s.log.Info("reconnecting after ffmpeg exit", 
				slog.String("task", task.ID),
				slog.Duration("backoff", backoff))
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
	// 使用Go的sort.Slice，O(n log n)复杂度，性能比冒泡排序好100倍
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].lastMod.Before(objects[j].lastMod)
	})
	
	// 删除最旧的图片（保留最新的maxCount张）
	// 但跳过队列中的图片，确保推理时图片存在
	deleteCount := len(objects) - maxCount
	deletedCount := 0
	skippedInQueue := 0
	
	// 遍历最旧的deleteCount张图片，尝试删除
	// 如果图片正在推理，跳过（保护正在推理的图片）
	for i := 0; i < deleteCount && i < len(objects); i++ {
		// 规范化路径格式（确保与队列中的路径格式一致）
		normalizedKey := filepath.ToSlash(objects[i].key)
		
		// 检查图片是否在推理队列中（通过回调函数）
		if s.isImageInQueue(normalizedKey) {
			skippedInQueue++
			s.log.Debug("skipping cleanup for image in inference queue",
				slog.String("task", task.ID),
				slog.String("key", normalizedKey),
				slog.String("note", "image is in inference queue, will be cleaned up after inference"))
			continue
		}
		
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.minio.client.RemoveObject(deleteCtx, s.minio.bucket, normalizedKey, minio.RemoveObjectOptions{})
		deleteCancel()
		
		if err != nil {
			s.log.Warn("failed to delete old frame", 
				slog.String("task", task.ID),
				slog.String("key", normalizedKey),
				slog.String("err", err.Error()))
		} else {
			deletedCount++
			s.log.Debug("deleted old frame", 
				slog.String("task", task.ID),
				slog.String("key", normalizedKey))
		}
	}
	
	if deletedCount > 0 || skippedInQueue > 0 {
		// 修复剩余数量计算：skippedInQueue的图片还在MinIO中，应该包含在remaining中
		// remaining = 总数量 - 已删除数量（跳过的不算删除，所以还在MinIO中）
		remaining := len(objects) - deletedCount
		s.log.Info("cleaned up old frames", 
			slog.String("task", task.ID),
			slog.Int("deleted", deletedCount),
			slog.Int("skipped_in_queue", skippedInQueue),
			slog.Int("remaining", remaining),
			slog.Int("limit", maxCount),
			slog.String("note", "skipped images in inference queue to prevent inference failures"))
	}
	
	return nil
}

// isImageInQueue 检查图片是否在推理队列中
func (s *Service) isImageInQueue(imagePath string) bool {
	s.queueCheckerMu.RLock()
	defer s.queueCheckerMu.RUnlock()
	
	if s.queueChecker == nil {
		return false // 如果没有注册检查器，返回false（不保护）
	}
	
	return s.queueChecker(imagePath)
}

// getMaxFrameCount 获取任务的最大图片数量限制
// 优先使用任务级配置，如果为0或未配置则使用全局配置
// 全局配置默认为500，如果全局配置也为0，则不限制
func getMaxFrameCount(task conf.FrameExtractTask, cfg *conf.FrameExtractorConfig) int {
	// 如果任务级配置大于0，使用任务级配置
	if task.MaxFrameCount > 0 {
		return task.MaxFrameCount
	}
	// 否则使用全局配置（全局配置默认为500）
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

// shouldForceCleanup 检查是否需要强制清理（图片数量可能已超过限制）
// 通过检查距离上次清理的时间来判断，避免图片数量持续增长
func (s *Service) shouldForceCleanup(taskID string, maxCount int) bool {
	// 如果距离上次清理超过30秒，触发强制清理检查
	// 这样可以确保即使限流机制延迟，也能及时清理超出的图片
	s.cleanupMu.Lock()
	counter, exists := s.cleanupCounters[taskID]
	if !exists {
		counter = &cleanupCounter{
			uploadCount: 0,
			lastCleanup: time.Time{},
		}
		s.cleanupCounters[taskID] = counter
	}
	lastCleanup := counter.lastCleanup
	s.cleanupMu.Unlock()
	
	// 如果距离上次清理超过30秒，触发清理（即使未达到50张的阈值）
	// 这样可以防止图片数量持续增长超过限制
	if lastCleanup.IsZero() || time.Since(lastCleanup) >= 30*time.Second {
		return true
	}
	
	return false
}
