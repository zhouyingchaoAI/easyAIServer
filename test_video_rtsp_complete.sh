#!/bin/bash
# 视频转流功能完整验证脚本
set -e

API_BASE="http://127.0.0.1:5066/api/v1"
TEST_VIDEO="/tmp/test_video_rtsp.mp4"

echo "=== 视频转流功能完整验证 ==="

# 查找FFmpeg
FFMPEG=$(find ./build ./deploy -name "ffmpeg" -type f 2>/dev/null | head -1 || command -v ffmpeg || echo "")
[ -z "$FFMPEG" ] && echo "错误: 未找到FFmpeg" && exit 1
echo "✓ FFmpeg: $FFMPEG"

# 检查服务
curl -s "${API_BASE}/video_rtsp" >/dev/null || { echo "错误: 服务未运行"; exit 1; }
echo "✓ 服务运行中"

# 创建测试视频
if [ ! -f "$TEST_VIDEO" ]; then
    echo "创建测试视频..."
    $FFMPEG -f lavfi -i testsrc=duration=10:size=640x480:rate=30 \
        -f lavfi -i sine=frequency=1000:duration=10 \
        -c:v libx264 -preset ultrafast -crf 23 \
        -c:a aac -b:a 128k \
        -y "$TEST_VIDEO" 2>/dev/null
fi
echo "✓ 测试视频: $TEST_VIDEO"

# 创建任务
echo "创建流任务..."
RESP=$(curl -s -X POST "${API_BASE}/video_rtsp" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"测试_$(date +%s)\",\"videoPath\":\"${TEST_VIDEO}\",\"loop\":true,\"enabled\":true}")

TASK_ID=$(echo "$RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
RTSP_URL=$(echo "$RESP" | grep -o '"rtspUrl":"[^"]*"' | head -1 | cut -d'"' -f4)

[ -z "$TASK_ID" ] && { echo "错误: 创建失败"; echo "$RESP"; exit 1; }
echo "✓ 任务ID: $TASK_ID"
echo "✓ RTSP: $RTSP_URL"

# 验证路径
if [[ "$RTSP_URL" =~ /video/ ]]; then
    echo "✓ RTSP路径正确: /video/"
elif [[ "$RTSP_URL" =~ /live/ ]]; then
    echo "⚠ RTSP路径为 /live/ (服务可能需要重启以应用新代码)"
fi

# 等待启动
echo "等待流启动..."
for i in {1..15}; do
    sleep 2
    STATUS=$(curl -s "${API_BASE}/video_rtsp/${TASK_ID}" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    [ "$STATUS" = "running" ] && echo "✓ 流已启动" && break
    [ "$STATUS" = "error" ] && ERROR=$(curl -s "${API_BASE}/video_rtsp/${TASK_ID}" | grep -o '"error":"[^"]*"' | cut -d'"' -f4) && echo "错误: $ERROR" && exit 1
done

[ "$STATUS" != "running" ] && echo "错误: 启动超时" && exit 1

echo ""
echo "=== 验证成功 ==="
echo "RTSP: $RTSP_URL"
echo "测试播放: ffplay -rtsp_transport tcp $RTSP_URL"
sleep 20

# 清理
curl -s -X DELETE "${API_BASE}/video_rtsp/${TASK_ID}" >/dev/null
echo "✓ 清理完成"
