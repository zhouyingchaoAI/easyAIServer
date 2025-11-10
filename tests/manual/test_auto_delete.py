#!/usr/bin/env python3
"""
测试自动删除MinIO图片功能

此脚本用于测试：
1. 算法推理返回结果
2. 当 total_count = 0 时，自动删除MinIO图片
3. 当 total_count > 0 时，保留图片并保存告警

运行：
    python3 test_auto_delete.py
"""

import json
import requests
import time
from datetime import datetime
from minio import Minio
from minio.error import S3Error

# 配置
EASYDARWIN_URL = "http://localhost:5066"
ALGORITHM_ENDPOINT = "http://localhost:8000/infer"
MINIO_ENDPOINT = "10.1.6.230:9000"
MINIO_ACCESS_KEY = "admin"
MINIO_SECRET_KEY = "admin123"
MINIO_BUCKET = "images"
MINIO_USE_SSL = False

print("="*70)
print("测试 AI推理自动删除MinIO图片功能")
print("="*70)
print()

# 初始化MinIO客户端
print("📦 初始化MinIO客户端...")
try:
    minio_client = Minio(
        MINIO_ENDPOINT,
        access_key=MINIO_ACCESS_KEY,
        secret_key=MINIO_SECRET_KEY,
        secure=MINIO_USE_SSL
    )
    print("✅ MinIO客户端初始化成功")
except Exception as e:
    print(f"❌ MinIO客户端初始化失败: {e}")
    exit(1)

print()

def check_image_exists(image_path):
    """检查MinIO中图片是否存在"""
    try:
        minio_client.stat_object(MINIO_BUCKET, image_path)
        return True
    except S3Error as e:
        if e.code == 'NoSuchKey':
            return False
        raise

def list_images_in_path(prefix):
    """列出指定路径下的所有图片"""
    try:
        objects = minio_client.list_objects(MINIO_BUCKET, prefix=prefix, recursive=True)
        images = [obj.object_name for obj in objects if obj.object_name.endswith('.jpg')]
        return images
    except Exception as e:
        print(f"❌ 列出图片失败: {e}")
        return []

def upload_test_image(task_type, task_id):
    """上传一个测试图片到MinIO"""
    from io import BytesIO
    from PIL import Image, ImageDraw, ImageFont
    
    # 创建一个简单的测试图片
    img = Image.new('RGB', (640, 480), color=(73, 109, 137))
    d = ImageDraw.Draw(img)
    d.text((10, 10), f"Test Image\n{task_type}\n{task_id}", fill=(255, 255, 255))
    
    # 转换为字节
    img_bytes = BytesIO()
    img.save(img_bytes, format='JPEG')
    img_bytes.seek(0)
    
    # 上传到MinIO
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S.%f")[:-3]
    image_path = f"frames/{task_type}/{task_id}/{timestamp}.jpg"
    
    try:
        minio_client.put_object(
            MINIO_BUCKET,
            image_path,
            img_bytes,
            img_bytes.getbuffer().nbytes,
            content_type='image/jpeg'
        )
        print(f"  ✅ 测试图片上传成功: {image_path}")
        return image_path
    except Exception as e:
        print(f"  ❌ 图片上传失败: {e}")
        return None

def test_inference(task_type, task_id, expected_count):
    """测试推理和自动删除功能"""
    print(f"\n{'='*70}")
    print(f"测试场景: {task_type} (预期检测数: {expected_count})")
    print(f"{'='*70}")
    
    # 1. 上传测试图片
    print("\n1️⃣  上传测试图片...")
    image_path = upload_test_image(task_type, task_id)
    if not image_path:
        print("❌ 测试失败：无法上传图片")
        return False
    
    # 2. 等待扫描器扫描
    print("\n2️⃣  等待AI分析插件扫描（5-10秒）...")
    time.sleep(8)
    
    # 3. 检查图片是否还存在
    print("\n3️⃣  检查MinIO中的图片...")
    exists = check_image_exists(image_path)
    
    # 4. 验证结果
    print("\n4️⃣  验证结果...")
    if expected_count == 0:
        # 预期图片应该被删除
        if not exists:
            print(f"  ✅ 测试通过：图片已被自动删除（检测对象=0）")
            return True
        else:
            print(f"  ❌ 测试失败：图片未被删除（预期应删除）")
            return False
    else:
        # 预期图片应该保留
        if exists:
            print(f"  ✅ 测试通过：图片已保留（检测对象>0）")
            
            # 查询告警记录（可选）
            try:
                resp = requests.get(f"{EASYDARWIN_URL}/api/v1/ai_analysis/alerts", 
                                   params={"task_id": task_id}, timeout=5)
                if resp.status_code == 200:
                    alerts = resp.json().get('data', {}).get('items', [])
                    if len(alerts) > 0:
                        print(f"  ✅ 告警已保存：共 {len(alerts)} 条")
                    else:
                        print(f"  ⚠️  告警未找到（可能正在处理中）")
            except Exception as e:
                print(f"  ⚠️  无法查询告警: {e}")
            
            return True
        else:
            print(f"  ❌ 测试失败：图片被删除（预期应保留）")
            return False

def test_minio_stats():
    """统计MinIO存储情况"""
    print(f"\n{'='*70}")
    print("MinIO 存储统计")
    print(f"{'='*70}")
    
    try:
        # 统计各任务类型的图片数量
        task_types = ['人数统计', '人员跌倒', '车辆检测', '吸烟检测', '安全帽检测']
        
        print("\n各任务类型图片数量:")
        total_images = 0
        for task_type in task_types:
            images = list_images_in_path(f"frames/{task_type}/")
            count = len(images)
            total_images += count
            print(f"  {task_type}: {count} 张")
        
        print(f"\n总计: {total_images} 张图片")
        
    except Exception as e:
        print(f"❌ 统计失败: {e}")

def main():
    print("\n🚀 开始测试...")
    
    # 检查AI分析服务是否运行
    print("\n📡 检查AI分析服务...")
    try:
        resp = requests.get(f"{EASYDARWIN_URL}/api/v1/ai_analysis/services", timeout=5)
        if resp.status_code == 200:
            services = resp.json().get('data', {}).get('services', [])
            if len(services) > 0:
                print(f"✅ 发现 {len(services)} 个算法服务:")
                for svc in services:
                    print(f"  - {svc.get('name')} ({svc.get('service_id')})")
            else:
                print("⚠️  未发现算法服务，请先启动算法服务")
                print("运行: python3 examples/algorithm_service.py")
        else:
            print(f"❌ AI分析服务未响应 (HTTP {resp.status_code})")
            return
    except Exception as e:
        print(f"❌ 无法连接到EasyDarwin: {e}")
        print("请确保EasyDarwin正在运行")
        return
    
    # 测试用例
    test_cases = [
        # (任务类型, 任务ID, 预期检测数量)
        ('人数统计', 'test_count_001', 3),     # 应该保留图片
        ('人员跌倒', 'test_fall_001', 0),      # 应该删除图片
        ('车辆检测', 'test_vehicle_001', 2),   # 应该保留图片
        ('吸烟检测', 'test_smoking_001', 1),   # 应该保留图片
        ('安全帽检测', 'test_helmet_001', 0),  # 应该删除图片（示例中返回0）
    ]
    
    results = []
    for task_type, task_id, expected_count in test_cases:
        result = test_inference(task_type, task_id, expected_count)
        results.append((task_type, result))
        time.sleep(2)  # 间隔2秒
    
    # 统计MinIO存储
    test_minio_stats()
    
    # 汇总结果
    print(f"\n{'='*70}")
    print("测试结果汇总")
    print(f"{'='*70}")
    
    passed = sum(1 for _, result in results if result)
    total = len(results)
    
    for task_type, result in results:
        status = "✅ 通过" if result else "❌ 失败"
        print(f"  {status} - {task_type}")
    
    print(f"\n总计: {passed}/{total} 通过")
    
    if passed == total:
        print("\n🎉 所有测试通过！")
    else:
        print(f"\n⚠️  {total - passed} 个测试失败")
    
    # 提示
    print(f"\n{'='*70}")
    print("💡 提示")
    print(f"{'='*70}")
    print("1. 查看EasyDarwin日志了解详细推理过程")
    print("2. 访问MinIO控制台查看图片存储情况")
    print(f"   http://{MINIO_ENDPOINT}")
    print("3. 检测对象为0的图片会被自动删除")
    print("4. 检测对象>0的图片会保留并生成告警记录")
    print()

if __name__ == '__main__':
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n⚠️  测试被中断")
    except Exception as e:
        print(f"\n❌ 测试异常: {e}")
        import traceback
        traceback.print_exc()

