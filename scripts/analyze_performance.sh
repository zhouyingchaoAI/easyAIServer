#!/bin/bash
# 性能分析脚本 - 分析日志中的实际耗时

LOG_FILE="${1:-./logs/20251113_*.log}"

echo "=== AI分析性能分析 ==="
echo "日志文件: $LOG_FILE"
echo ""

# 分析扫描耗时
echo "【扫描阶段耗时】"
echo "----------------------------------------"
grep -h "scan statistics\|found new images" $LOG_FILE 2>/dev/null | tail -20 | python3 -c "
import sys
import json
import re

scans = []
for line in sys.stdin:
    line = line.strip()
    if not line or not line.startswith('{'):
        continue
    try:
        log = json.loads(line)
        msg = log.get('msg', '')
        
        if 'scan statistics' in msg or 'found new images' in msg:
            scan_duration = log.get('total_scan_duration_ms', {})
            list_duration = log.get('list_objects_duration_ms', {})
            queue_duration = log.get('queue_add_duration_ms', {})
            count = log.get('count', log.get('new_images', 0))
            
            def parse_duration(value):
                \"\"\"解析Duration值，支持多种格式\"\"\"
                if isinstance(value, str):
                    # 字符串格式：如\"100ms\", \"1.5s\"
                    if value.endswith('ms'):
                        return float(value[:-2])
                    elif value.endswith('s'):
                        return float(value[:-1]) * 1000
                    elif value.endswith('us'):
                        return float(value[:-2]) / 1000
                    elif value.endswith('ns'):
                        return float(value[:-2]) / 1000000
                    else:
                        try:
                            return float(value) / 1000000
                        except:
                            return 0
                elif isinstance(value, (int, float)):
                    # 数字格式：可能是纳秒数
                    if value > 1000000:
                        return value / 1000000
                    else:
                        return value
                elif isinstance(value, dict):
                    # 对象格式：{\"Duration\": 100000000}
                    if 'Duration' in value:
                        return value['Duration'] / 1000000
                return 0
            
            scan_ms = parse_duration(scan_duration)
            list_ms = parse_duration(list_duration)
            queue_ms = parse_duration(queue_duration)
            
            scans.append({
                'count': count,
                'scan_ms': scan_ms,
                'list_ms': list_ms,
                'queue_ms': queue_ms
            })
    except:
        pass

if scans:
    avg_scan = sum(s['scan_ms'] for s in scans) / len(scans)
    avg_list = sum(s['list_ms'] for s in scans) / len(scans)
    avg_queue = sum(s['queue_ms'] for s in scans) / len(scans)
    total_count = sum(s['count'] for s in scans)
    
    print(f'扫描次数: {len(scans)}')
    print(f'发现图片总数: {total_count}')
    print(f'平均扫描耗时: {avg_scan:.2f}ms')
    print(f'平均ListObjects耗时: {avg_list:.2f}ms')
    print(f'平均队列添加耗时: {avg_queue:.2f}ms')
else:
    print('未找到扫描日志')
" 2>/dev/null

echo ""
echo "【推理执行耗时】"
echo "----------------------------------------"
echo "注意：详细耗时在Debug级别，Info级别只包含algorithm_call_duration_ms"
echo ""
grep -h "inference completed and queued for batch save\|inference detailed timing" $LOG_FILE 2>/dev/null | tail -50 | python3 -c "
import sys
import json

inferences = []
for line in sys.stdin:
    line = line.strip()
    if not line or not line.startswith('{'):
        continue
    try:
        log = json.loads(line)
        msg = log.get('msg', '')
        
        if 'inference completed' in msg or 'inference detailed timing' in msg:
            # 解析Duration字段
            def parse_duration(value):
                \"\"\"解析Duration值，支持多种格式\"\"\"
                if isinstance(value, str):
                    # 字符串格式：如\"100ms\", \"1.5s\"
                    if value.endswith('ms'):
                        return float(value[:-2])
                    elif value.endswith('s'):
                        return float(value[:-1]) * 1000
                    elif value.endswith('us'):
                        return float(value[:-2]) / 1000
                    elif value.endswith('ns'):
                        return float(value[:-2]) / 1000000
                    else:
                        try:
                            return float(value) / 1000000
                        except:
                            return 0
                elif isinstance(value, (int, float)):
                    # 数字格式：可能是纳秒数
                    if value > 1000000:
                        return value / 1000000
                    else:
                        return value
                elif isinstance(value, dict):
                    # 对象格式：{\"Duration\": 100000000}
                    if 'Duration' in value:
                        return value['Duration'] / 1000000
                return 0
            
            # 优先从detailed timing中获取，如果没有则从inference completed中获取
            stat_ms = parse_duration(log.get('stat_duration_ms', 0))
            presign_ms = parse_duration(log.get('presign_duration_ms', 0))
            algo_ms = parse_duration(log.get('algorithm_call_duration_ms', 0))
            save_ms = parse_duration(log.get('save_duration_ms', 0))
            mq_ms = parse_duration(log.get('mq_duration_ms', 0))
            total_ms = parse_duration(log.get('total_infer_duration_ms', 0))
            
            inferences.append({
                'stat': stat_ms,
                'presign': presign_ms,
                'algo': algo_ms,
                'save': save_ms,
                'mq': mq_ms,
                'total': total_ms
            })
    except Exception as e:
        pass

if inferences:
    # 过滤掉0值（说明没有详细耗时信息）
    valid_inferences = [i for i in inferences if i['algo'] > 0]
    
    if valid_inferences:
        avg_stat = sum(i['stat'] for i in valid_inferences) / len(valid_inferences) if any(i['stat'] > 0 for i in valid_inferences) else 0
        avg_presign = sum(i['presign'] for i in valid_inferences) / len(valid_inferences) if any(i['presign'] > 0 for i in valid_inferences) else 0
        avg_algo = sum(i['algo'] for i in valid_inferences) / len(valid_inferences)
        avg_save = sum(i['save'] for i in valid_inferences) / len(valid_inferences) if any(i['save'] > 0 for i in valid_inferences) else 0
        avg_mq = sum(i['mq'] for i in valid_inferences) / len(valid_inferences) if any(i['mq'] > 0 for i in valid_inferences) else 0
        avg_total = sum(i['total'] for i in valid_inferences) / len(valid_inferences) if any(i['total'] > 0 for i in valid_inferences) else avg_algo
        
        print(f'推理次数: {len(inferences)} (有效: {len(valid_inferences)})')
        if avg_stat > 0:
            print(f'平均检查图片耗时: {avg_stat:.2f}ms')
        if avg_presign > 0:
            print(f'平均生成URL耗时: {avg_presign:.2f}ms')
        print(f'平均算法调用耗时: {avg_algo:.2f}ms (主要耗时)')
        if avg_save > 0:
            print(f'平均保存告警耗时: {avg_save:.2f}ms')
        if avg_mq > 0:
            print(f'平均推送Kafka耗时: {avg_mq:.2f}ms')
        if avg_total > 0:
            print(f'平均总推理耗时: {avg_total:.2f}ms')
            if avg_algo > 0:
                print(f'算法调用占比: {avg_algo/avg_total*100:.1f}%')
        print('')
        if len(valid_inferences) < len(inferences):
            print('⚠️  部分详细耗时信息缺失（需要开启Debug日志级别）')
    else:
        print(f'推理次数: {len(inferences)}')
        print('⚠️  未找到详细耗时信息（需要开启Debug日志级别查看详细耗时）')
        print('   当前只有algorithm_call_duration_ms（算法调用耗时）')
else:
    print('未找到推理日志')
" 2>/dev/null

echo ""
echo "【Worker处理耗时】（Debug级别，需要开启Debug日志）"
echo "----------------------------------------"
grep -h "worker processed image" $LOG_FILE 2>/dev/null | tail -50 | python3 -c "
import sys
import json

workers = []
for line in sys.stdin:
    line = line.strip()
    if not line or not line.startswith('{'):
        continue
    try:
        log = json.loads(line)
        msg = log.get('msg', '')
        
        if 'worker processed' in msg:
            def parse_duration(value):
                \"\"\"解析Duration值，支持多种格式\"\"\"
                if isinstance(value, str):
                    # 字符串格式：如\"100ms\", \"1.5s\"
                    if value.endswith('ms'):
                        return float(value[:-2])
                    elif value.endswith('s'):
                        return float(value[:-1]) * 1000
                    elif value.endswith('us'):
                        return float(value[:-2]) / 1000
                    elif value.endswith('ns'):
                        return float(value[:-2]) / 1000000
                    else:
                        try:
                            return float(value) / 1000000
                        except:
                            return 0
                elif isinstance(value, (int, float)):
                    # 数字格式：可能是纳秒数
                    if value > 1000000:
                        return value / 1000000
                    else:
                        return value
                elif isinstance(value, dict):
                    # 对象格式：{\"Duration\": 100000000}
                    if 'Duration' in value:
                        return value['Duration'] / 1000000
                return 0
            
            pop_ms = parse_duration(log.get('pop_duration_ms', {}))
            total_ms = parse_duration(log.get('total_process_duration_ms', {}))
            
            workers.append({
                'pop': pop_ms,
                'total': total_ms
            })
    except:
        pass

if workers:
    avg_pop = sum(w['pop'] for w in workers) / len(workers)
    avg_total = sum(w['total'] for w in workers) / len(workers)
    
    print(f'Worker处理次数: {len(workers)}')
    print(f'平均取队列耗时: {avg_pop:.2f}ms')
    print(f'平均总处理耗时: {avg_total:.2f}ms')
else:
    print('未找到Worker处理日志')
" 2>/dev/null

echo ""
echo "【线程统计】"
echo "----------------------------------------"
echo "扫描器线程: 1个"
echo "Worker线程: 300个 (max_concurrent_infer = 300)"
echo "推理并发: 最多300个 (受信号量限制)"
echo "统计线程: 1个"
echo "总计: 约302个goroutine"

echo ""
echo "分析完成时间: $(date)"

