#!/bin/bash
# 分析处理数量比抽帧数量多的原因

echo "=== 处理数量 vs 抽帧数量分析 ==="
echo ""

EASYDARWIN_URL="http://localhost:5066"

# 1. 获取统计数据
echo "【1. 当前统计数据】"
FRAME_STATS=$(curl -s "$EASYDARWIN_URL/api/v1/frame_extractor/stats")
INFERENCE_STATS=$(curl -s "$EASYDARWIN_URL/api/v1/ai_analysis/inference_stats")

if [ $? -eq 0 ]; then
    TOTAL_FRAMES=$(echo "$FRAME_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('total_frames', 0))" 2>/dev/null)
    PROCESSED_TOTAL=$(echo "$INFERENCE_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('processed_total', 0))" 2>/dev/null)
    
    echo "  抽帧总数: $TOTAL_FRAMES"
    echo "  处理总数: $PROCESSED_TOTAL"
    
    if [ ! -z "$TOTAL_FRAMES" ] && [ ! -z "$PROCESSED_TOTAL" ]; then
        DIFF=$((PROCESSED_TOTAL - TOTAL_FRAMES))
        echo "  差异: $DIFF 张"
        
        if [ "$DIFF" -gt 0 ]; then
            echo ""
            echo "  ⚠️  处理数量比抽帧数量多 $DIFF 张"
            echo ""
            echo "  可能的原因："
            echo "    1. 初始扫描处理了启动前已存在的图片（这些图片不在抽帧统计中）"
            echo "    2. 抽帧服务重启了，但AI分析服务没有重启（抽帧统计重置，处理统计未重置）"
            echo "    3. 统计口径不一致："
            echo "       - 抽帧统计：只统计服务启动后新抽取的帧"
            echo "       - 处理统计：包括初始扫描处理的旧图片 + 新抽取的帧"
            echo ""
            echo "  这是正常现象，因为："
            echo "    - AI分析服务启动时会执行一次初始扫描，处理MinIO中已存在的图片"
            echo "    - 这些图片在服务启动前就已经存在，不在抽帧统计中"
            echo "    - 但会被处理并计入 processed_total"
        elif [ "$DIFF" -lt 0 ]; then
            echo ""
            echo "  ⚠️  处理数量比抽帧数量少 $((-$DIFF)) 张"
            echo ""
            echo "  可能的原因："
            echo "    1. 部分图片在队列中被清理（队列积压时）"
            echo "    2. 部分图片在等待时被Frame Extractor清理"
            echo "    3. 部分图片被跳过（重复或不符合条件）"
        else
            echo ""
            echo "  ✓ 处理数量和抽帧数量一致"
        fi
    fi
else
    echo "  ✗ 无法获取统计数据"
fi
echo ""

# 2. 检查服务启动时间
echo "【2. 服务运行时间】"
if [ -f "logs/sugar.log" ]; then
    START_TIME=$(grep -i "AI analysis plugin started\|frame extractor.*started" logs/sugar.log | tail -1 | awk '{print $1, $2}')
    if [ ! -z "$START_TIME" ]; then
        echo "  服务启动时间: $START_TIME"
    else
        echo "  无法确定服务启动时间"
    fi
    
    # 检查初始扫描日志
    INITIAL_SCAN=$(grep -i "initial scan\|performing initial scan" logs/sugar.log | tail -1)
    if [ ! -z "$INITIAL_SCAN" ]; then
        echo "  发现初始扫描日志:"
        echo "    $INITIAL_SCAN"
        echo ""
        echo "  初始扫描处理的图片数量:"
        INITIAL_COUNT=$(grep -i "initial scan.*images added" logs/sugar.log | tail -1 | grep -oP "added.*?\d+" | grep -oP "\d+" | head -1)
        if [ ! -z "$INITIAL_COUNT" ]; then
            echo "    $INITIAL_COUNT 张"
            if [ ! -z "$DIFF" ] && [ "$DIFF" -gt 0 ]; then
                if [ "$INITIAL_COUNT" -eq "$DIFF" ] || [ "$INITIAL_COUNT" -gt "$DIFF" ]; then
                    echo "    ✓ 初始扫描处理的图片数量与差异接近，说明差异主要来自初始扫描"
                else
                    echo "    ⚠️  初始扫描处理的图片数量少于差异，可能还有其他原因"
                fi
            fi
        else
            echo "    无法确定"
        fi
    else
        echo "  未发现初始扫描日志"
    fi
else
    echo "  ✗ 日志文件不存在"
fi
echo ""

# 3. 检查统计重置情况
echo "【3. 统计重置检查】"
if [ -f "logs/sugar.log" ]; then
    # 检查是否有服务重启
    RESTART_COUNT=$(grep -i "AI analysis plugin started\|frame extractor.*started" logs/sugar.log | wc -l)
    if [ "$RESTART_COUNT" -gt 1 ]; then
        echo "  ⚠️  发现 $RESTART_COUNT 次服务启动记录，服务可能重启过"
        echo "     如果抽帧服务和AI分析服务重启时间不一致，会导致统计口径不一致"
    else
        echo "  ✓ 服务未重启（或只启动了一次）"
    fi
else
    echo "  ✗ 日志文件不存在"
fi
echo ""

echo "=== 分析完成 ==="
echo ""
echo "建议："
echo "1. 如果差异来自初始扫描，这是正常现象，无需处理"
echo "2. 如果差异很大且持续增长，检查是否有重复处理的情况"
echo "3. 如果差异为负数，检查图片是否被提前清理"

