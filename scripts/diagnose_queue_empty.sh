#!/bin/bash
# 诊断队列为空和推理为0的问题

echo "=== 队列为空和推理为0问题诊断 ==="
echo ""

EASYDARWIN_URL="http://localhost:5066"

# 1. 获取统计数据
echo "【1. 当前统计数据】"
INFERENCE_STATS=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/inference_stats")
FRAME_STATS=$(curl -s "$EASYDARWIN_URL/api/v1/frame_extractor/stats")

if [ $? -eq 0 ]; then
    QUEUE_SIZE=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('queue_size', 0))" 2>/dev/null)
    SUCCESS_RATE=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('success_rate_per_sec', 0))" 2>/dev/null)
    PROCESSED_TOTAL=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('processed_total', 0))" 2>/dev/null)
    FRAMES_PER_SEC=$(echo "$FRAME_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('frames_per_sec', 0))" 2>/dev/null)
    
    echo "  队列大小: $QUEUE_SIZE"
    echo "  每秒推理成功数: $SUCCESS_RATE"
    echo "  累计处理数: $PROCESSED_TOTAL"
    echo "  每秒抽帧数: $FRAMES_PER_SEC"
    
    if [ "$QUEUE_SIZE" -eq 0 ] && [ "$(echo "$SUCCESS_RATE < 0.1" | bc -l 2>/dev/null || echo "0")" -eq 1 ]; then
        echo ""
        echo "  ⚠️  队列为空且推理速度很慢"
    fi
else
    echo "  ✗ 无法获取统计数据"
fi
echo ""

# 2. 检查事件监听器
echo "【2. 检查事件监听器状态】"
if [ -f "logs/sugar.log" ]; then
    EVENT_LISTENER_START=$(grep -i "starting MinIO event listener\|event listener.*started" logs/sugar.log | tail -1)
    if [ ! -z "$EVENT_LISTENER_START" ]; then
        echo "  ✓ 事件监听器已启动"
        echo "    $EVENT_LISTENER_START"
    else
        echo "  ✗ 未发现事件监听器启动日志"
    fi
    
    EVENT_ERRORS=$(grep -i "event.*error\|notification.*error\|listenEvents.*error\|event listener.*error" logs/sugar.log | tail -5)
    if [ ! -z "$EVENT_ERRORS" ]; then
        echo ""
        echo "  ⚠️  发现事件监听器错误："
        echo "$EVENT_ERRORS"
    else
        echo "  ✓ 未发现事件监听器错误"
    fi
    
    # 检查最近是否有图片被添加到队列
    IMAGE_ADDED=$(grep -i "image added to queue" logs/sugar.log | tail -5)
    if [ ! -z "$IMAGE_ADDED" ]; then
        echo ""
        echo "  ✓ 最近有图片被添加到队列："
        echo "$IMAGE_ADDED" | head -3
    else
        echo ""
        echo "  ⚠️  最近没有图片被添加到队列"
    fi
else
    echo "  ✗ 日志文件不存在"
fi
echo ""

# 3. 检查Worker状态
echo "【3. 检查Worker状态】"
if [ -f "logs/sugar.log" ]; then
    WORKER_START=$(grep -i "starting inference workers\|worker.*started" logs/sugar.log | tail -1)
    if [ ! -z "$WORKER_START" ]; then
        echo "  ✓ Worker已启动"
        echo "    $WORKER_START"
    else
        echo "  ✗ 未发现Worker启动日志"
    fi
    
    # 检查队列是否一直为空
    QUEUE_EMPTY=$(grep -i "queue empty\|queue.*empty" logs/sugar.log | tail -5)
    if [ ! -z "$QUEUE_EMPTY" ]; then
        echo ""
        echo "  ⚠️  发现队列为空的日志："
        echo "$QUEUE_EMPTY" | head -3
    fi
    
    # 检查是否有推理调度
    INFERENCE_SCHEDULED=$(grep -i "inference.*scheduled\|scheduling inference" logs/sugar.log | tail -5)
    if [ ! -z "$INFERENCE_SCHEDULED" ]; then
        echo ""
        echo "  ✓ 最近有推理被调度："
        echo "$INFERENCE_SCHEDULED" | head -3
    else
        echo ""
        echo "  ⚠️  最近没有推理被调度"
    fi
else
    echo "  ✗ 日志文件不存在"
fi
echo ""

# 4. 检查抽帧状态
echo "【4. 检查抽帧状态】"
if [ ! -z "$FRAMES_PER_SEC" ]; then
    if [ "$(echo "$FRAMES_PER_SEC > 0" | bc -l 2>/dev/null || echo "0")" -eq 1 ]; then
        echo "  ✓ 抽帧服务正常运行，每秒抽帧: $FRAMES_PER_SEC 张"
        echo ""
        if [ "$QUEUE_SIZE" -eq 0 ]; then
            echo "  ⚠️  抽帧正常但队列为空，可能的原因："
            echo "    1. 事件监听器没有捕获到新图片"
            echo "    2. 新图片被添加到队列后立即被处理（队列为空是正常的）"
            echo "    3. 图片被跳过（重复或不符合条件）"
        fi
    else
        echo "  ⚠️  抽帧服务可能未运行或抽帧速度很慢"
    fi
fi
echo ""

# 5. 检查MinIO事件通知配置
echo "【5. 检查MinIO事件通知配置】"
if [ -f "configs/config.toml" ]; then
    MINIO_ENABLED=$(grep -A 10 "\[minio\]" configs/config.toml | grep "enable" | awk -F'=' '{print $2}' | tr -d ' ' | head -c 10)
    if [ "$MINIO_ENABLED" = "true" ]; then
        echo "  ✓ MinIO已启用"
    else
        echo "  ⚠️  MinIO未启用或配置不正确"
    fi
else
    echo "  ✗ 配置文件不存在"
fi
echo ""

# 6. 建议
echo "【6. 问题分析和建议】"
if [ "$QUEUE_SIZE" -eq 0 ] && [ "$(echo "$SUCCESS_RATE < 0.1" | bc -l 2>/dev/null || echo "0")" -eq 1 ]; then
    echo "  问题：队列为空且推理速度很慢"
    echo ""
    echo "  可能的原因："
    echo "    1. 事件监听器没有正常工作，没有捕获到新图片"
    echo "    2. MinIO事件通知未配置或配置不正确"
    echo "    3. 新图片被添加到队列后立即被处理（队列为空是正常的，但推理速度应该更快）"
    echo "    4. Worker没有正常运行或数量不足"
    echo ""
    echo "  建议检查："
    echo "    1. 检查MinIO事件通知配置"
    echo "    2. 检查事件监听器日志"
    echo "    3. 检查Worker数量和运行状态"
    echo "    4. 检查是否有图片被跳过"
fi
echo ""

echo "=== 诊断完成 ==="

