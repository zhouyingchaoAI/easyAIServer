#!/usr/bin/env python3
"""
清理MinIO中的旧抽帧图片
保留alerts目录中的告警图片
"""

from minio import Minio
import sys

# MinIO配置（从config.toml中获取）
MINIO_ENDPOINT = "172.16.5.207:9000"
MINIO_ACCESS_KEY = "admin"
MINIO_SECRET_KEY = "admin123"
MINIO_BUCKET = "images"
ALERT_PREFIX = "alerts/"

def main():
    print("=" * 60)
    print("清理MinIO中的旧抽帧图片")
    print("=" * 60)
    print(f"MinIO: {MINIO_ENDPOINT}")
    print(f"Bucket: {MINIO_BUCKET}")
    print(f"保留: {ALERT_PREFIX}* (告警图片)")
    print("=" * 60)
    
    # 连接MinIO
    try:
        client = Minio(
            MINIO_ENDPOINT,
            access_key=MINIO_ACCESS_KEY,
            secret_key=MINIO_SECRET_KEY,
            secure=False
        )
        print("✓ MinIO连接成功")
    except Exception as e:
        print(f"✗ MinIO连接失败: {e}")
        sys.exit(1)
    
    # 确认操作
    print("\n⚠️  警告：此操作将删除所有抽帧图片（不包括告警图片）！")
    confirm = input("是否继续？(yes/no): ")
    if confirm.lower() != 'yes':
        print("操作已取消")
        sys.exit(0)
    
    # 列出所有对象
    print("\n正在扫描对象...")
    objects = list(client.list_objects(MINIO_BUCKET, recursive=True))
    print(f"找到 {len(objects)} 个对象")
    
    # 分类对象
    to_delete = []
    to_keep = []
    
    for obj in objects:
        if obj.object_name.startswith(ALERT_PREFIX):
            to_keep.append(obj.object_name)
        else:
            to_delete.append(obj.object_name)
    
    print(f"\n将删除: {len(to_delete)} 个抽帧图片")
    print(f"将保留: {len(to_keep)} 个告警图片")
    
    if len(to_delete) == 0:
        print("\n没有需要删除的对象")
        return
    
    # 显示部分待删除对象
    print("\n待删除对象示例（前10个）：")
    for i, obj_name in enumerate(to_delete[:10]):
        print(f"  {i+1}. {obj_name}")
    if len(to_delete) > 10:
        print(f"  ... 还有 {len(to_delete)-10} 个")
    
    # 最后确认
    print()
    confirm2 = input("确认删除？(yes/no): ")
    if confirm2.lower() != 'yes':
        print("操作已取消")
        sys.exit(0)
    
    # 执行删除
    print("\n开始删除...")
    deleted_count = 0
    failed_count = 0
    
    for obj_name in to_delete:
        try:
            client.remove_object(MINIO_BUCKET, obj_name)
            deleted_count += 1
            if deleted_count % 100 == 0:
                print(f"  已删除 {deleted_count}/{len(to_delete)}...")
        except Exception as e:
            print(f"  删除失败: {obj_name} - {e}")
            failed_count += 1
    
    # 结果统计
    print("\n" + "=" * 60)
    print("清理完成！")
    print("=" * 60)
    print(f"成功删除: {deleted_count} 个")
    print(f"删除失败: {failed_count} 个")
    print(f"保留对象: {len(to_keep)} 个")
    print("=" * 60)
    
    print("\n✓ 旧图片已清理完成")
    print("  建议：")
    print("  1. 重启EasyDarwin服务")
    print("  2. 等待新图片生成")
    print("  3. 检查告警是否正确关联")

if __name__ == "__main__":
    main()

