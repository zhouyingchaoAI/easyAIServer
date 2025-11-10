package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	fmt.Println("==========================================")
	fmt.Println("MinIO 连接诊断工具 (自动排查 502 问题)")
	fmt.Println("==========================================\n")

	endpoint := "10.1.6.230:9000"
	bucket := "images"
	accessKey := "admin"
	secretKey := "admin123"
	useSSL := false

	fmt.Printf("配置:\n")
	fmt.Printf("  Endpoint: %s\n", endpoint)
	fmt.Printf("  Bucket: %s\n", bucket)
	fmt.Printf("  UseSSL: %v\n\n", useSSL)

	// 配置自定义 Transport
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 创建客户端
	fmt.Println("1️⃣  创建 MinIO 客户端...")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:    useSSL,
		Transport: transport,
		Region:    "",
	})
	if err != nil {
		log.Fatalf("❌ 创建客户端失败: %v", err)
	}
	fmt.Println("   ✅ 客户端创建成功\n")

	// 测试 1: 基础连接
	fmt.Println("2️⃣  测试基础连接...")
	testBasicConnection(client, bucket)

	// 测试 2: Bucket 权限
	fmt.Println("\n3️⃣  测试 Bucket 访问权限...")
	testBucketAccess(client, bucket)

	// 测试 3: 预签名 URL 生成
	fmt.Println("\n4️⃣  测试预签名 URL 生成...")
	testPresignedURL(client, bucket)

	// 测试 4: 对象列表
	fmt.Println("\n5️⃣  测试对象列表...")
	testListObjects(client, bucket)

	// 测试 5: 压力测试（多次生成预签名 URL）
	fmt.Println("\n6️⃣  压力测试（50次预签名URL生成）...")
	stressTestPresignedURL(client, bucket)

	fmt.Println("\n==========================================")
	fmt.Println("✅ 所有测试完成！")
	fmt.Println("==========================================")
}

func testBasicConnection(client *minio.Client, bucket string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		fmt.Printf("   ❌ 连接失败: %v\n", err)
		fmt.Printf("   错误类型: %T\n", err)
		log.Fatal("基础连接失败")
	}

	fmt.Printf("   ✅ 连接成功 (bucket存在: %v)\n", exists)
}

func testBucketAccess(client *minio.Client, bucket string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objectCh := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: false,
		MaxKeys:   1,
	})

	hasError := false
	for object := range objectCh {
		if object.Err != nil {
			fmt.Printf("   ❌ 访问失败: %v\n", object.Err)
			fmt.Printf("   错误类型: %T\n", object.Err)
			hasError = true
			break
		}
		break
	}

	if hasError {
		log.Fatal("Bucket 访问失败")
	}
	fmt.Printf("   ✅ Bucket 访问正常\n")
}

func testPresignedURL(client *minio.Client, bucket string) {
	testPath := "test/debug-presigned-url.txt"

	maxRetries := 3
	retryDelay := 1 * time.Second
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		url, err := client.PresignedGetObject(ctx, bucket, testPath, 1*time.Hour, nil)
		cancel()

		if err == nil {
			if i > 0 {
				fmt.Printf("   ✅ 第 %d 次尝试成功\n", i+1)
			} else {
				fmt.Printf("   ✅ 预签名 URL 生成成功\n")
			}
			fmt.Printf("   URL: %s\n", url.String())

			// 测试 URL 访问
			if err := testURLAccess(url.String()); err != nil {
				fmt.Printf("   ⚠️  URL 无法访问（文件可能不存在）: %v\n", err)
			} else {
				fmt.Printf("   ✅ URL 可访问\n")
			}
			return
		}

		lastErr = err
		fmt.Printf("   ❌ 第 %d 次尝试失败: %v\n", i+1, err)
		fmt.Printf("   错误类型: %T\n", err)

		if i < maxRetries-1 {
			fmt.Printf("   ⏳ 等待 %v 后重试...\n", retryDelay)
			time.Sleep(retryDelay)
			retryDelay *= 2
		}
	}

	fmt.Printf("   ❌ 预签名 URL 生成失败（重试 %d 次）\n", maxRetries)
	fmt.Printf("   最后错误: %v\n", lastErr)
	fmt.Printf("   错误类型: %T\n", lastErr)
}

func testURLAccess(urlStr string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", urlStr, nil)
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

func testListObjects(client *minio.Client, bucket string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objectCh := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: false,
		MaxKeys:   10,
	})

	count := 0
	hasError := false
	for object := range objectCh {
		if object.Err != nil {
			fmt.Printf("   ❌ 列出对象失败: %v\n", object.Err)
			hasError = true
			break
		}
		count++
		if count >= 10 {
			break
		}
	}

	if hasError {
		fmt.Printf("   ❌ 列出对象时出错\n")
	} else {
		fmt.Printf("   ✅ 成功列出 %d 个对象\n", count)
	}
}

func stressTestPresignedURL(client *minio.Client, bucket string) {
	totalTests := 50
	successCount := 0
	errorCount := 0

	startTime := time.Now()

	for i := 0; i < totalTests; i++ {
		path := fmt.Sprintf("test/stress-test-%d.txt", i)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		url, err := client.PresignedGetObject(ctx, bucket, path, 1*time.Hour, nil)
		cancel()

		if err == nil {
			successCount++
			_ = url
		} else {
			errorCount++
		}

		// 每10次显示进度
		if (i+1)%10 == 0 {
			fmt.Printf("   进度: %d/%d (成功: %d, 失败: %d)\n",
				i+1, totalTests, successCount, errorCount)
		}
	}

	duration := time.Since(startTime)
	successRate := float64(successCount) / float64(totalTests) * 100

	fmt.Printf("   ✅ 压力测试完成\n")
	fmt.Printf("   总数: %d\n", totalTests)
	fmt.Printf("   成功: %d (%.1f%%)\n", successCount, successRate)
	fmt.Printf("   失败: %d\n", errorCount)
	fmt.Printf("   耗时: %v\n", duration)
	fmt.Printf("   平均: %v/次\n", duration/time.Duration(totalTests))

	if successRate < 95 {
		fmt.Printf("   ⚠️  成功率较低，可能存在连接问题\n")
	} else {
		fmt.Printf("   ✅ 成功率良好\n")
	}
}
