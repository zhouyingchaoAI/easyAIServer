package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	endpoint := "10.1.6.230:9000"
	accessKey := "admin"
	secretKey := "admin123"
	bucket := "images"
	useSSL := false

	fmt.Println("========================================")
	fmt.Println("MinIO Go SDK 测试")
	fmt.Println("========================================")
	fmt.Printf("\n配置:\n")
	fmt.Printf("  Endpoint: %s\n", endpoint)
	fmt.Printf("  Bucket: %s\n", bucket)
	fmt.Printf("  SSL: %v\n", useSSL)
	fmt.Println()

	// 创建MinIO客户端
	fmt.Println("1. 创建MinIO客户端...")
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("❌ 创建客户端失败: %v\n", err)
	}
	fmt.Println("   ✅ 客户端创建成功")

	// 测试BucketExists
	fmt.Println("\n2. 测试BucketExists...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		fmt.Printf("   ❌ BucketExists失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ BucketExists成功: %v\n", exists)
	}

	// 测试ListObjects
	fmt.Println("\n3. 测试ListObjects...")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()

	objectCh := client.ListObjects(ctx2, bucket, minio.ListObjectsOptions{
		Prefix:    "",
		Recursive: true,
	})

	count := 0
	hasError := false
	for object := range objectCh {
		if object.Err != nil {
			fmt.Printf("   ❌ ListObjects错误: %v\n", object.Err)
			hasError = true
			break
		}
		count++
		if count <= 5 {
			fmt.Printf("   ✅ %s (%d bytes)\n", object.Key, object.Size)
		}
	}

	if !hasError {
		fmt.Printf("\n   ✅ ListObjects成功，共 %d 个对象\n", count)
	}

	fmt.Println("\n========================================")
	if hasError {
		fmt.Println("❌ 测试失败")
	} else {
		fmt.Println("✅ 所有测试通过")
	}
	fmt.Println("========================================")
}

