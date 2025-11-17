#!/bin/bash
# 诊断抽帧和处理速率差异

echo "=== 抽帧和处理速率诊断 ==="
echo ""

EASYDARWIN_URL="http://localhost:5066"

# 1. 获取抽帧统计
echo "【1. 抽帧统计】"
FRAME_STATS=$(curl -s "$EASYDARWIN_URL/api/v1/frame_extractor/stats")
if [ $? -eq 0 ]; then
    TOTAL_FRAMES=$(echo "$FRAME_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('total_frames', 0))" 2>/dev/null)
    FRAMES_PER_SEC=$(echo "$FRAME_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('frames_per_sec', 0))" 2>/dev/null)
    TOTAL_TASKS=$(echo "$FRAME_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('total_tasks', 0))" 2>/dev/null)
    RUNNING_TASKS=$(echo "$FRAME_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('running_tasks', 0))" 2>/dev/null)
    
    echo "  总抽帧数: $TOTAL_FRAMES"
    echo "  每秒抽帧数: $FRAMES_PER_SEC"
    echo "  总任务数: $TOTAL_TASKS"
    echo "  运行中任务数: $RUNNING_TASKS"
else
    echo "  ✗ 无法获取抽帧统计"
fi
echo ""

# 2. 获取推理统计
echo "【2. 推理统计】"
INFERENCE_STATS=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/inference_stats")
if [ $? -eq 0 ]; then
    PROCESSED_TOTAL=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('processed_total', 0))" 2>/dev/null)
    SUCCESS_RATE=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('success_rate_per_sec', 0))" 2>/dev/null)
    QUEUE_SIZE=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('queue_size', 0))" 2>/dev/null)
    DROPPED_TOTAL=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('dropped_total', 0))" 2>/dev/null)
    AVG_TIME=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('avg_inference_ms', 0))" 2>/dev/null)
    
    echo "  累计处理数: $PROCESSED_TOTAL"
    echo "  每秒处理数: $SUCCESS_RATE"
    echo "  当前队列大小: $QUEUE_SIZE"
    echo "  累计丢弃数: $DROPPED_TOTAL"
    echo "  平均推理时间: ${AVG_TIME}ms"
else
    echo "  ✗ 无法获取推理统计"
fi
echo ""

# 3. 计算差异
echo "【3. 差异分析】"
if [ ! -z "$TOTAL_FRAMES" ] && [ ! -z "$PROCESSED_TOTAL" ]; then
    DIFF=$((TOTAL_FRAMES - PROCESSED_TOTAL))
    if [ "$TOTAL_FRAMES" -gt 0 ]; then
        PROCESS_RATE=$(echo "scale=2; $PROCESSED_TOTAL * 100 / $TOTAL_FRAMES" | bc -l 2>/dev/null || echo "0")
    else
        PROCESS_RATE="0"
    fi
    
    echo "  抽帧总数: $TOTAL_FRAMES"
    echo "  处理总数: $PROCESSED_TOTAL"
    echo "  未处理数: $DIFF"
    if [ ! -z "$PROCESS_RATE" ] && [ "$PROCESS_RATE" != "0" ]; then
        echo "  处理率: ${PROCESS_RATE}%"
    fi
    
    if [ "$DIFF" -gt 0 ]; then
        echo ""
        echo "  ⚠️  存在未处理的图片（${DIFF} 张）"
        echo ""
        echo "  可能的原因："
        echo "    1. 图片在队列中被清理（队列积压时）"
        echo "    2. 图片在等待时被Frame Extractor清理"
        echo "    3. 图片在推理前被删除"
        echo "    4. 事件监听器未捕获到所有图片"
        echo "    5. 图片被跳过（重复或不符合条件）"
    else
        echo "  ✓ 所有图片都已处理"
    fi
fi
echo ""

# 4. 速率对比
echo "【4. 速率对比】"
if [ ! -z "$FRAMES_PER_SEC" ] && [ ! -z "$SUCCESS_RATE" ]; then
    echo "  抽帧速率: ${FRAMES_PER_SEC} 张/秒"
    echo "  处理速率: ${SUCCESS_RATE} 张/秒"
    
    if [ ! -z "$FRAMES_PER_SEC" ] && [ ! -z "$SUCCESS_RATE" ]; then
        RATE_DIFF=$(echo "$FRAMES_PER_SEC - $SUCCESS_RATE" | bc -l 2>/dev/null || echo "0")
        if [ ! -z "$RATE_DIFF" ]; then
            if [ $(echo "$RATE_DIFF > 0" | bc -l 2>/dev/null || echo "0") -eq 1 ]; then
                echo "  速率差: +${RATE_DIFF} 张/秒（抽帧快于处理）"
                echo "  ⚠️  处理速度跟不上抽帧速度，会导致积压"
            else
                echo "  速率差: ${RATE_DIFF} 张/秒（处理快于抽帧）"
                echo "  ✓ 处理速度可以跟上抽帧速度"
            fi
        fi
    fi
fi
echo ""

# 5. 检查日志
echo "【5. 检查日志中的线索】"
if [ -f "logs/sugar.log" ]; then
    echo "  最近20条图片相关日志:"
    tail -500 logs/sugar.log | grep -E "(image added|image.*deleted|image.*skipped|image.*removed|processed)" | tail -20
    echo ""
    
    echo "  检查图片被删除的情况:"
    DELETED_COUNT=$(tail -1000 logs/sugar.log | grep -i "image.*deleted\|delete.*image" | wc -l)
    if [ "$DELETED_COUNT" -gt 0 ]; then
        echo "    ⚠️  发现 $DELETED_COUNT 条删除日志"
        tail -1000 logs/sugar.log | grep -i "image.*deleted\|delete.*image" | tail -5
    else
        echo "    ✓ 未发现大量删除日志"
    fi
    echo ""
    
    echo "  检查图片被跳过的情况:"
    SKIPPED_COUNT=$(tail -1000 logs/sugar.log | grep -i "image.*skipped\|skip.*image" | wc -l)
    if [ "$SKIPPED_COUNT" -gt 0 ]; then
        echo "    ⚠️  发现 $SKIPPED_COUNT 条跳过日志"
        tail -1000 logs/sugar.log | grep -i "image.*skipped\|skip.*image" | tail -5
    else
        echo "    ✓ 未发现大量跳过日志"
    fi
else
    echo "  ✗ 日志文件不存在"
fi
echo ""

# 6. 检查配置
echo "【6. 检查配置】"
if [ -f "configs/config.toml" ]; then
    MAX_CONCURRENT=$(grep -A 10 "\[ai_analysis\]" configs/config.toml | grep "max_concurrent_infer" | awk -F'=' '{print $2}' | tr -d ' ' | head -c 10)
    MAX_QUEUE=$(grep -A 10 "\[ai_analysis\]" configs/config.toml | grep "max_queue_size" | awk -F'=' '{print $2}' | tr -d ' ' | head -c 10)
    SAVE_ONLY=$(grep -A 10 "\[ai_analysis\]" configs/config.toml | grep "save_only_with_detection" | awk -F'=' '{print $2}' | tr -d ' ' | head -c 10)
    
    echo "  最大并发推理数: $MAX_CONCURRENT"
    echo "  最大队列大小: $MAX_QUEUE"
    echo "  仅保存有检测结果: $SAVE_ONLY"
    
    if [ "$SAVE_ONLY" = "true" ]; then
        echo "  ⚠️  配置为仅保存有检测结果的图片，无检测结果的图片会被删除"
        echo "     这可能导致processed_total小于抽帧总数"
    fi
else
    echo "  ✗ 配置文件不存在"
fi
echo ""

echo "=== 诊断完成 ==="

