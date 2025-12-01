#!/bin/bash
# FFmpeg命令调试脚本
# 用于测试和调试视频转流的FFmpeg命令

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== FFmpeg命令调试工具 ===${NC}"
echo ""

# 默认参数
VIDEO_PATH="${1:-/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511261444/vod_data/vodsrc/lJC_4mZvg.mp4}"
RTMP_URL="${2:-rtmp://127.0.0.1:21935/video/test_stream}"
VIDEO_CODEC="${3:-libx264}"
AUDIO_CODEC="${4:-aac}"
LOOP="${5:-true}"

# 检查视频文件
if [ ! -f "$VIDEO_PATH" ]; then
    echo -e "${RED}错误: 视频文件不存在: $VIDEO_PATH${NC}"
    echo ""
    echo "用法: $0 [视频路径] [RTMP URL] [视频编码] [音频编码] [是否循环]"
    echo "示例: $0 /path/to/video.mp4 rtmp://127.0.0.1:21935/video/test libx264 aac true"
    exit 1
fi

# 检查FFmpeg
FFMPEG_PATH=""
if [ -f "./ffmpeg" ]; then
    FFMPEG_PATH="./ffmpeg"
elif [ -f "./build/EasyDarwin-aarch64-v8.3.3-202511261444/ffmpeg" ]; then
    FFMPEG_PATH="./build/EasyDarwin-aarch64-v8.3.3-202511261444/ffmpeg"
elif command -v ffmpeg > /dev/null 2>&1; then
    FFMPEG_PATH=$(which ffmpeg)
else
    echo -e "${RED}错误: 找不到FFmpeg${NC}"
    exit 1
fi

echo -e "${GREEN}FFmpeg路径: $FFMPEG_PATH${NC}"
echo -e "${GREEN}视频文件: $VIDEO_PATH${NC}"
echo -e "${GREEN}RTMP URL: $RTMP_URL${NC}"
echo -e "${GREEN}视频编码: $VIDEO_CODEC${NC}"
echo -e "${GREEN}音频编码: $AUDIO_CODEC${NC}"
echo -e "${GREEN}循环播放: $LOOP${NC}"
echo ""

# 构建FFmpeg命令
ARGS=()

# 循环播放
if [ "$LOOP" = "true" ]; then
    ARGS+=("-stream_loop" "-1")
fi

# 实时模式
ARGS+=("-re")

# 生成PTS时间戳
ARGS+=("-fflags" "+genpts")

# 输入文件
ARGS+=("-i" "$VIDEO_PATH")

# 映射流
ARGS+=("-map" "0:v:0")
if [ -n "$AUDIO_CODEC" ]; then
    ARGS+=("-map" "0:a?")
fi

# 视频编码
ARGS+=("-c:v" "$VIDEO_CODEC")
if [ "$VIDEO_CODEC" != "copy" ]; then
    ARGS+=("-preset" "ultrafast")
    ARGS+=("-tune" "zerolatency")
    ARGS+=("-g" "50")
    ARGS+=("-b:v" "2000k")
    ARGS+=("-maxrate" "2000k")
    ARGS+=("-bufsize" "4000k")
    ARGS+=("-pix_fmt" "yuv420p")
    ARGS+=("-force_key_frames" "expr:gte(n,n_forced*1)")
    if [ "$VIDEO_CODEC" = "libx264" ]; then
        ARGS+=("-x264-params" "keyint=50:min-keyint=50:scenecut=0:force-cfr=1")
    fi
fi

# 音频编码
if [ -n "$AUDIO_CODEC" ] && [ "$AUDIO_CODEC" != "copy" ]; then
    ARGS+=("-c:a" "$AUDIO_CODEC")
    if [ "$AUDIO_CODEC" = "aac" ]; then
        ARGS+=("-b:a" "128k")
        ARGS+=("-ar" "44100")
        ARGS+=("-ac" "2")
        ARGS+=("-bsf:a" "aac_adtstoasc")
    fi
else
    ARGS+=("-c:a" "copy")
    ARGS+=("-bsf:a" "aac_adtstoasc")
fi

# 输出格式
ARGS+=("-f" "flv")

# 日志级别
ARGS+=("-loglevel" "info")

# RTMP URL
ARGS+=("$RTMP_URL")

# 显示完整命令
echo -e "${YELLOW}=== 完整FFmpeg命令 ===${NC}"
echo ""
echo "$FFMPEG_PATH ${ARGS[*]}"
echo ""

# 显示命令（每行一个参数，便于阅读）
echo -e "${YELLOW}=== 命令参数（每行一个） ===${NC}"
echo "$FFMPEG_PATH \\"
for i in "${!ARGS[@]}"; do
    if [ $((i + 1)) -eq ${#ARGS[@]} ]; then
        echo "  \"${ARGS[$i]}\""
    else
        echo "  \"${ARGS[$i]}\" \\"
    fi
done
echo ""

# 询问是否执行
read -p "是否执行此命令? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消"
    exit 0
fi

echo ""
echo -e "${BLUE}开始执行FFmpeg命令...${NC}"
echo -e "${YELLOW}提示: 按 Ctrl+C 停止${NC}"
echo ""

# 执行命令
exec "$FFMPEG_PATH" "${ARGS[@]}"

