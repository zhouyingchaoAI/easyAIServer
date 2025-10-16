#!/usr/bin/env python3
"""
测试MinIO连接和配置
"""
from minio import Minio
from minio.error import S3Error
import sys

# MinIO配置（从config.toml读取）
MINIO_ENDPOINT = "10.1.6.230:9000"
ACCESS_KEY = "admin"
SECRET_KEY = "admin123"
BUCKET_NAME = "images"
USE_SSL = False

print("="*60)
print("MinIO 连接测试工具")
print("="*60)
print(f"\n配置信息:")
print(f"  Endpoint: {MINIO_ENDPOINT}")
print(f"  Access Key: {ACCESS_KEY}")
print(f"  Secret Key: {'*' * len(SECRET_KEY)}")
print(f"  Bucket: {BUCKET_NAME}")
print(f"  Use SSL: {USE_SSL}")
print()

try:
    # 创建MinIO客户端
    print("1️⃣  创建MinIO客户端...")
    client = Minio(
        MINIO_ENDPOINT,
        access_key=ACCESS_KEY,
        secret_key=SECRET_KEY,
        secure=USE_SSL
    )
    print("   ✅ 客户端创建成功")
    
    # 检查bucket是否存在
    print(f"\n2️⃣  检查bucket '{BUCKET_NAME}' 是否存在...")
    bucket_exists = client.bucket_exists(BUCKET_NAME)
    
    if bucket_exists:
        print(f"   ✅ Bucket '{BUCKET_NAME}' 存在")
    else:
        print(f"   ❌ Bucket '{BUCKET_NAME}' 不存在")
        
        # 尝试创建bucket
        print(f"\n3️⃣  尝试创建bucket '{BUCKET_NAME}'...")
        try:
            client.make_bucket(BUCKET_NAME)
            print(f"   ✅ Bucket '{BUCKET_NAME}' 创建成功")
            bucket_exists = True
        except S3Error as e:
            print(f"   ❌ 创建失败: {e}")
            sys.exit(1)
    
    # 列出所有buckets
    print("\n4️⃣  列出所有buckets...")
    buckets = client.list_buckets()
    if buckets:
        for bucket in buckets:
            marker = "✅" if bucket.name == BUCKET_NAME else "  "
            print(f"   {marker} {bucket.name} (创建于: {bucket.creation_date})")
    else:
        print("   ⚠️  没有找到任何bucket")
    
    # 测试上传
    print("\n5️⃣  测试上传文件...")
    test_content = b"yanying test file"
    test_object = "test/test.txt"
    
    from io import BytesIO
    client.put_object(
        BUCKET_NAME,
        test_object,
        BytesIO(test_content),
        len(test_content)
    )
    print(f"   ✅ 上传测试文件成功: {test_object}")
    
    # 测试列出对象
    print(f"\n6️⃣  列出bucket中的对象...")
    objects = client.list_objects(BUCKET_NAME, prefix="test/", recursive=True)
    found = False
    for obj in objects:
        print(f"   ✅ {obj.object_name} ({obj.size} bytes)")
        found = True
    
    if not found:
        print("   ⚠️  bucket中没有对象")
    
    # 测试下载
    print(f"\n7️⃣  测试下载文件...")
    response = client.get_object(BUCKET_NAME, test_object)
    data = response.read()
    response.close()
    
    if data == test_content:
        print("   ✅ 下载测试文件成功，内容匹配")
    else:
        print("   ❌ 下载内容不匹配")
    
    # 清理测试文件
    print(f"\n8️⃣  清理测试文件...")
    client.remove_object(BUCKET_NAME, test_object)
    print("   ✅ 清理完成")
    
    print("\n" + "="*60)
    print("✅ MinIO 连接测试全部通过！")
    print("="*60)
    print("\n配置建议:")
    print("1. ✅ MinIO服务正常运行")
    print(f"2. ✅ Bucket '{BUCKET_NAME}' 已就绪")
    print("3. ✅ 读写权限正常")
    print("\nyanying平台可以正常使用MinIO存储了！")
    print()
    
except S3Error as e:
    print(f"\n❌ MinIO错误: {e}")
    print(f"\n可能的原因:")
    print("1. Access Key 或 Secret Key 不正确")
    print("2. MinIO服务未启动或无法访问")
    print("3. 网络连接问题")
    print("\n请检查配置文件中的MinIO设置")
    sys.exit(1)
    
except Exception as e:
    print(f"\n❌ 未知错误: {e}")
    import traceback
    traceback.print_exc()
    sys.exit(1)

