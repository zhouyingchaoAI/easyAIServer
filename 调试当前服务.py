#!/usr/bin/env python3
"""
调试当前算法服务注册情况
"""
import requests
import json
import time

EASYDARWIN_URL = "http://172.16.5.207:5066"

def check_services():
    """检查当前注册的服务"""
    print("=" * 60)
    print("📊 当前注册的算法服务")
    print("=" * 60)
    
    try:
        # 获取服务列表
        response = requests.get(f"{EASYDARWIN_URL}/api/v1/ai_analysis/services", timeout=3)
        data = response.json()
        
        total = data.get('total', 0)
        services = data.get('services', [])
        
        print(f"\n总计: {total} 个服务\n")
        
        if total == 0:
            print("⚠️  当前没有注册的服务")
            return
        
        # 按端口排序
        services_sorted = sorted(services, key=lambda x: x.get('endpoint', ''))
        
        for i, svc in enumerate(services_sorted, 1):
            port = svc.get('endpoint', '').split(':')[-1].split('/')[0]
            service_id = svc.get('service_id', 'N/A')
            endpoint = svc.get('endpoint', 'N/A')
            task_types = svc.get('task_types', [])
            call_count = svc.get('call_count', 0)
            last_hb = svc.get('last_heartbeat', 0)
            
            # 计算心跳年龄
            now = int(time.time())
            hb_age = now - last_hb
            hb_status = "✅" if hb_age < 60 else "⚠️" if hb_age < 90 else "❌"
            
            print(f"{i}. 端口 {port}")
            print(f"   Service ID: {service_id}")
            print(f"   Endpoint: {endpoint}")
            print(f"   任务类型: {', '.join(task_types[:3])}")
            print(f"   调用次数: {call_count}")
            print(f"   最后心跳: {hb_age}秒前 {hb_status}")
            print()
            
    except requests.exceptions.ConnectionError:
        print("❌ 无法连接到EasyDarwin")
        print(f"   URL: {EASYDARWIN_URL}")
        print("   请检查EasyDarwin是否正在运行")
    except Exception as e:
        print(f"❌ 错误: {e}")

def check_real_services():
    """检查实际运行的算法服务"""
    print("=" * 60)
    print("🔍 检查实际运行的算法服务")
    print("=" * 60)
    print()
    
    running_services = []
    
    for port in range(7901, 7909):
        try:
            response = requests.get(f"http://172.16.5.207:{port}/health", timeout=1)
            if response.status_code == 200:
                data = response.json()
                service_id = data.get('service_id', 'N/A')
                status = data.get('status', 'N/A')
                print(f"✅ 端口 {port}: {service_id} ({status})")
                running_services.append(port)
            else:
                print(f"❌ 端口 {port}: HTTP {response.status_code}")
        except requests.exceptions.ConnectionError:
            print(f"❌ 端口 {port}: 无服务运行")
        except requests.exceptions.Timeout:
            print(f"⚠️  端口 {port}: 超时")
        except Exception as e:
            print(f"❌ 端口 {port}: {e}")
    
    print(f"\n运行中的服务: {len(running_services)} 个")
    print(f"端口列表: {running_services}")
    return running_services

def clear_all_services():
    """清空所有注册"""
    print("\n" + "=" * 60)
    print("🗑️  清空所有服务注册")
    print("=" * 60)
    
    try:
        response = requests.post(f"{EASYDARWIN_URL}/api/v1/ai_analysis/clear_all", timeout=3)
        data = response.json()
        
        if data.get('ok'):
            cleared = data.get('cleared_count', 0)
            print(f"✅ 成功清空 {cleared} 个服务")
        else:
            print(f"❌ 清空失败: {data}")
    except Exception as e:
        print(f"❌ 错误: {e}")

def analyze_load_balance():
    """分析负载均衡情况"""
    print("\n" + "=" * 60)
    print("📈 负载均衡分析")
    print("=" * 60)
    print()
    
    try:
        response = requests.get(f"{EASYDARWIN_URL}/api/v1/ai_analysis/load_balance/analysis", timeout=3)
        data = response.json()
        
        analysis = data.get('analysis', {})
        
        if not analysis:
            print("⚠️  暂无负载均衡数据")
            return
        
        for task_type, stats in analysis.items():
            print(f"\n任务类型: {task_type}")
            print(f"  服务数量: {stats['service_count']}")
            print(f"  总调用次数: {stats['total_calls']}")
            print(f"  平均调用: {stats['avg_calls']:.1f}")
            print(f"  最少调用: {stats['min_calls']}")
            print(f"  最多调用: {stats['max_calls']}")
            print(f"  均衡质量: {stats['balance_quality']}")
            print(f"\n  各服务调用分布:")
            for svc in stats['services']:
                print(f"    • {svc['endpoint']:40s} 调用: {svc['call_count']:4d} 次")
                
    except Exception as e:
        print(f"❌ 错误: {e}")

if __name__ == "__main__":
    print("\n🔧 EasyDarwin 算法服务调试工具\n")
    
    # 1. 检查实际运行的服务
    running = check_real_services()
    
    # 2. 检查注册的服务
    print()
    check_services()
    
    # 3. 分析负载均衡
    analyze_load_balance()
    
    # 4. 提供操作建议
    print("\n" + "=" * 60)
    print("💡 操作建议")
    print("=" * 60)
    print()
    print("如果发现虚假注册，可以:")
    print("  1. 重启EasyDarwin平台（内存清空）")
    print("     pkill easydarwin && ./easydarwin.com")
    print()
    print("  2. 停止心跳脚本")
    print("     pkill -f maintain_heartbeat")
    print()
    print("  3. 使用clear_all API（需要新版本）")
    print("     curl -X POST http://localhost:5066/api/v1/ai_analysis/clear_all")
    print()
    print("如果服务数量正确，检查:")
    print(f"  • 实际运行: {len(running)} 个")
    print("  • 应该注册: 8 个 (7901-7908)")
    print("  • 如果不匹配，检查算法服务的启动配置")
    print()

