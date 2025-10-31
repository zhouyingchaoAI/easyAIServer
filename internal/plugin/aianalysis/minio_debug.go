package aianalysis

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
)

// MinIODebugger MinIO 调试工具
type MinIODebugger struct {
	client *minio.Client
	bucket string
	log    *slog.Logger
}

// NewMinIODebugger 创建 MinIO 调试器
func NewMinIODebugger(client *minio.Client, bucket string, logger *slog.Logger) *MinIODebugger {
	return &MinIODebugger{
		client: client,
		bucket: bucket,
		log:    logger,
	}
}

// Diagnose 全面诊断 MinIO 连接
func (d *MinIODebugger) Diagnose() error {
	d.log.Info("开始 MinIO 诊断...")

	// 1. 测试基础连接
	if err := d.testBasicConnection(); err != nil {
		return fmt.Errorf("基础连接测试失败: %w", err)
	}

	// 2. 测试 Bucket 访问
	if err := d.testBucketAccess(); err != nil {
		return fmt.Errorf("Bucket 访问测试失败: %w", err)
	}

	// 3. 测试预签名 URL 生成
	if err := d.testPresignedURL(); err != nil {
		return fmt.Errorf("预签名 URL 测试失败: %w", err)
	}

	// 4. 测试对象列表
	if err := d.testListObjects(); err != nil {
		return fmt.Errorf("对象列表测试失败: %w", err)
	}

	d.log.Info("✅ MinIO 诊断全部通过")
	return nil
}

// testBasicConnection 测试基础连接
func (d *MinIODebugger) testBasicConnection() error {
	d.log.Info("1️⃣  测试基础连接...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := d.client.BucketExists(ctx, d.bucket)
	if err != nil {
		d.log.Error("连接失败",
			slog.String("error", err.Error()),
			slog.String("error_type", fmt.Sprintf("%T", err)))
		return err
	}

	d.log.Info("连接成功", slog.Bool("bucket_exists", exists))
	return nil
}

// testBucketAccess 测试 Bucket 访问权限
func (d *MinIODebugger) testBucketAccess() error {
	d.log.Info("2️⃣  测试 Bucket 访问权限...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 尝试列出对象（少量）
	objectCh := d.client.ListObjects(ctx, d.bucket, minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: false,
		MaxKeys:   1,
	})

	hasError := false
	for object := range objectCh {
		if object.Err != nil {
			d.log.Error("列出对象失败",
				slog.String("error", object.Err.Error()),
				slog.String("error_type", fmt.Sprintf("%T", object.Err)))
			hasError = true
			break
		}
		// 成功读取到一个对象
		break
	}

	if hasError {
		return fmt.Errorf("无法访问 bucket")
	}

	d.log.Info("Bucket 访问权限正常")
	return nil
}

// testPresignedURL 测试预签名 URL 生成
func (d *MinIODebugger) testPresignedURL() error {
	d.log.Info("3️⃣  测试预签名 URL 生成...")
	
	// 使用一个测试路径
	testPath := "test/presigned-url-test.txt"
	
	// 尝试生成预签名 URL（多次测试）
	maxRetries := 3
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		
		url, err := d.client.PresignedGetObject(ctx, d.bucket, testPath, 1*time.Hour, nil)
		cancel()
		
		if err == nil {
			d.log.Info("预签名 URL 生成成功",
				slog.String("url", url.String()),
				slog.Int("attempt", i+1))
			
			// 测试访问这个 URL（不验证响应内容，只检查是否能连接）
			if err := d.testURLAccess(url.String()); err != nil {
				d.log.Warn("预签名 URL 无法访问",
					slog.String("url", url.String()),
					slog.String("error", err.Error()))
				// 不返回错误，因为测试文件可能不存在
			}
			return nil
		}
		
		lastErr = err
		d.log.Warn("预签名 URL 生成失败，重试中...",
			slog.Int("attempt", i+1),
			slog.String("error", err.Error()),
			slog.String("error_type", fmt.Sprintf("%T", err)))
		
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return fmt.Errorf("预签名 URL 生成失败（重试 %d 次）: %w", maxRetries, lastErr)
}

// testURLAccess 测试 URL 访问
func (d *MinIODebugger) testURLAccess(url string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil
}

// testListObjects 测试对象列表
func (d *MinIODebugger) testListObjects() error {
	d.log.Info("4️⃣  测试对象列表...")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objectCh := d.client.ListObjects(ctx, d.bucket, minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: false,
		MaxKeys:   10,
	})

	count := 0
	hasError := false
	for object := range objectCh {
		if object.Err != nil {
			d.log.Error("列出对象失败",
				slog.String("error", object.Err.Error()),
				slog.String("error_type", fmt.Sprintf("%T", object.Err)))
			hasError = true
			break
		}
		count++
		if count >= 10 {
			break
		}
	}

	if hasError {
		return fmt.Errorf("列出对象时出错")
	}

	d.log.Info("对象列表测试成功", slog.Int("objects_found", count))
	return nil
}

// DiagnoseWithRetry 带重试的诊断
func (d *MinIODebugger) DiagnoseWithRetry(maxRetries int, retryDelay time.Duration) error {
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			d.log.Info("重试诊断...",
				slog.Int("attempt", i+1),
				slog.Int("max_retries", maxRetries))
			time.Sleep(retryDelay)
		}
		
		err := d.Diagnose()
		if err == nil {
			if i > 0 {
				d.log.Info("诊断在重试后成功",
					slog.Int("total_attempts", i+1))
			}
			return nil
		}
		
		lastErr = err
	}
	
	return fmt.Errorf("诊断失败（重试 %d 次）: %w", maxRetries, lastErr)
}

