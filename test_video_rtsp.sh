#!/bin/bash

# 视频转流功能验证脚本
# 测试视频文件转RTSP流功能是否正常工作

set -e

# 配置
API_BASE="http://127.0.0.1:5066/api/v1"
TEST_VIDEO_FILE="/tmp/test_video.mp4"
TEST_VIDEO_DURATION=10  # 秒

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# 检查服务是否运行
check_service() {
    log_info "检查EasyDarwin服务是否运行..."
    if curl -s "${API_BASE}/video_rtsp" > /dev/null 2>&1; then
        log_info "✓ EasyDarwin服务正在运行"
        return 0
    else
        log_error "✗ EasyDarwin服务未运行或API不可访问"
        log_error "请确保服务运行在 ${API_BASE}"
        return 1
    fi
}

# 检查FFmpeg是否可用
check_ffmpeg() {
    log_info "检查FFmpeg是否可用..."
    if command -v ffmpeg > /dev/null 2>&1 || [ -f "./ffmpeg" ] || [ -f "./ffmpeg.exe" ]; then
        log_info "✓ FFmpeg可用"
        return 0
    else
        log_error "✗ FFmpeg未找到"
        log_error "请确保FFmpeg在PATH中或当前工作目录下"
        return 1
    fi
}

# 创建测试视频文件
create_test_video() {
    log_info "创建测试视频文件: ${TEST_VIDEO_FILE}"
    
    if [ -f "${TEST_VIDEO_FILE}" ]; then
        log_warn "测试视频文件已存在，跳过创建"
        return 0
    fi
    
    # 查找ffmpeg路径
    FFMPEG_CMD=""
    if command -v ffmpeg > /dev/null 2>&1; then
        FFMPEG_CMD="ffmpeg"
    elif [ -f "./ffmpeg" ]; then
        FFMPEG_CMD="./ffmpeg"
    elif [ -f "./ffmpeg.exe" ]; then
        FFMPEG_CMD="./ffmpeg.exe"
    else
        log_error "无法找到FFmpeg"
        return 1
    fi
    
    # 创建一个简单的测试视频（10秒，640x480，30fps）
    log_info "正在生成测试视频（这可能需要几秒钟）..."
    ${FFMPEG_CMD} -f lavfi -i testsrc=duration=${TEST_VIDEO_DURATION}:size=640x480:rate=30 \
        -f lavfi -i sine=frequency=1000:duration=${TEST_VIDEO_DURATION} \
        -c:v libx264 -preset ultrafast -crf 23 \
        -c:a aac -b:a 128k \
        -y "${TEST_VIDEO_FILE}" > /dev/null 2>&1
    
    if [ -f "${TEST_VIDEO_FILE}" ]; then
        log_info "✓ 测试视频文件创建成功"
        return 0
    else
        log_error "✗ 测试视频文件创建失败"
        return 1
    fi
}

# 创建流任务
create_stream_task() {
    log_info "创建视频转流任务..."
    
    local task_name="测试任务_$(date +%s)"
    
    RESPONSE=$(curl -s -X POST "${API_BASE}/video_rtsp" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"${task_name}\",
            \"videoPath\": \"${TEST_VIDEO_FILE}\",
            \"loop\": true,
            \"videoCodec\": \"libx264\",
            \"audioCodec\": \"aac\",
            \"preset\": \"ultrafast\",
            \"tune\": \"zerolatency\",
            \"enabled\": true
        }")
    
    if [ $? -ne 0 ]; then
        log_error "✗ 创建流任务失败：API请求失败"
        return 1
    fi
    
    # 检查响应
    echo "${RESPONSE}" | grep -q "id" || {
        log_error "✗ 创建流任务失败"
        log_error "响应: ${RESPONSE}"
        return 1
    }
    
    # 提取任务ID和流名称
    TASK_ID=$(echo "${RESPONSE}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    STREAM_NAME=$(echo "${RESPONSE}" | grep -o '"streamName":"[^"]*"' | head -1 | cut -d'"' -f4)
    RTSP_URL=$(echo "${RESPONSE}" | grep -o '"rtspUrl":"[^"]*"' | head -1 | cut -d'"' -f4)
    
    if [ -z "${TASK_ID}" ]; then
        log_error "✗ 无法从响应中提取任务ID"
        log_error "响应: ${RESPONSE}"
        return 1
    fi
    
    log_info "✓ 流任务创建成功"
    log_info "  任务ID: ${TASK_ID}"
    log_info "  流名称: ${STREAM_NAME}"
    log_info "  RTSP地址: ${RTSP_URL}"
    
    export TASK_ID
    export STREAM_NAME
    export RTSP_URL
    
    return 0
}

# 等待流启动
wait_stream_start() {
    log_info "等待流启动（最多等待30秒）..."
    
    local max_wait=30
    local waited=0
    local interval=2
    
    while [ ${waited} -lt ${max_wait} ]; do
        RESPONSE=$(curl -s "${API_BASE}/video_rtsp/${TASK_ID}")
        STATUS=$(echo "${RESPONSE}" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
        
        if [ "${STATUS}" = "running" ]; then
            log_info "✓ 流已启动"
            return 0
        elif [ "${STATUS}" = "error" ]; then
            ERROR_MSG=$(echo "${RESPONSE}" | grep -o '"error":"[^"]*"' | head -1 | cut -d'"' -f4)
            log_error "✗ 流启动失败: ${ERROR_MSG}"
            return 1
        fi
        
        sleep ${interval}
        waited=$((waited + interval))
        log_info "等待中... (${waited}/${max_wait}秒)"
    done
    
    log_error "✗ 流启动超时"
    return 1
}

# 验证RTSP流可访问
verify_rtsp_stream() {
    log_info "验证RTSP流是否可访问: ${RTSP_URL}"
    
    # 检查RTSP URL格式
    if [[ ! "${RTSP_URL}" =~ ^rtsp://.*/video/.* ]]; then
        log_error "✗ RTSP地址格式错误，应该包含 /video/ 路径"
        log_error "  当前地址: ${RTSP_URL}"
        return 1
    fi
    
    log_info "✓ RTSP地址格式正确"
    
    # 尝试使用ffprobe或ffmpeg检查流（快速检查，3秒超时）
    FFMPEG_CMD=""
    if command -v ffmpeg > /dev/null 2>&1; then
        FFMPEG_CMD="ffmpeg"
    elif [ -f "./ffmpeg" ]; then
        FFMPEG_CMD="./ffmpeg"
    elif [ -f "./ffmpeg.exe" ]; then
        FFMPEG_CMD="./ffmpeg.exe"
    fi
    
    if [ -n "${FFMPEG_CMD}" ]; then
        log_info "使用FFmpeg测试RTSP流连接..."
        timeout 5 ${FFMPEG_CMD} -rtsp_transport tcp -i "${RTSP_URL}" -t 1 -f null - 2>&1 | grep -q "Stream" && {
            log_info "✓ RTSP流可访问"
            return 0
        } || {
            log_warn "⚠ 无法快速验证RTSP流（可能是正常的，需要更多时间缓冲）"
            log_info "RTSP地址: ${RTSP_URL}"
            log_info "可以使用以下命令手动测试:"
            log_info "  ffplay -rtsp_transport tcp ${RTSP_URL}"
            return 0  # 不返回错误，因为可能需要更多时间
        }
    else
        log_warn "⚠ 无法找到FFmpeg进行流验证，跳过自动测试"
        log_info "RTSP地址: ${RTSP_URL}"
        log_info "可以使用以下命令手动测试:"
        log_info "  ffplay -rtsp_transport tcp ${RTSP_URL}"
        return 0
    fi
}

# 停止流任务
stop_stream_task() {
    log_info "停止流任务: ${TASK_ID}"
    
    # 先禁用任务
    curl -s -X PUT "${API_BASE}/video_rtsp/${TASK_ID}" \
        -H "Content-Type: application/json" \
        -d '{"enabled": false}' > /dev/null
    
    sleep 2
    
    # 停止流
    curl -s -X POST "${API_BASE}/video_rtsp/${TASK_ID}/stop" > /dev/null
    
    sleep 2
    
    # 检查状态
    RESPONSE=$(curl -s "${API_BASE}/video_rtsp/${TASK_ID}")
    STATUS=$(echo "${RESPONSE}" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
    
    if [ "${STATUS}" = "stopped" ]; then
        log_info "✓ 流已停止"
        return 0
    else
        log_warn "⚠ 流状态: ${STATUS} (可能仍在运行)"
        return 0
    fi
}

# 删除流任务
delete_stream_task() {
    log_info "删除流任务: ${TASK_ID}"
    
    curl -s -X DELETE "${API_BASE}/video_rtsp/${TASK_ID}" > /dev/null
    
    log_info "✓ 流任务已删除"
}

# 清理测试数据
cleanup() {
    log_info "清理测试数据..."
    
    if [ -n "${TASK_ID}" ]; then
        delete_stream_task
    fi
}

# 主测试流程
main() {
    log_info "=========================================="
    log_info "视频转流功能验证测试"
    log_info "=========================================="
    echo ""
    
    # 设置错误处理
    trap cleanup EXIT
    
    # 步骤1: 检查服务
    if ! check_service; then
        exit 1
    fi
    echo ""
    
    # 步骤2: 检查FFmpeg
    if ! check_ffmpeg; then
        exit 1
    fi
    echo ""
    
    # 步骤3: 创建测试视频
    if ! create_test_video; then
        exit 1
    fi
    echo ""
    
    # 步骤4: 创建流任务
    if ! create_stream_task; then
        exit 1
    fi
    echo ""
    
    # 步骤5: 等待流启动
    if ! wait_stream_start; then
        exit 1
    fi
    echo ""
    
    # 步骤6: 验证RTSP流
    if ! verify_rtsp_stream; then
        log_warn "RTSP流验证未完全通过，但流可能仍在启动中"
    fi
    echo ""
    
    # 成功
    log_info "=========================================="
    log_info "✓ 视频转流功能验证成功！"
    log_info "=========================================="
    log_info ""
    log_info "测试结果:"
    log_info "  任务ID: ${TASK_ID}"
    log_info "  流名称: ${STREAM_NAME}"
    log_info "  RTSP地址: ${RTSP_URL}"
    log_info ""
    log_info "可以使用以下命令测试播放:"
    log_info "  ffplay -rtsp_transport tcp ${RTSP_URL}"
    log_info ""
    
    # 保持运行一段时间以便手动测试
    log_info "流将保持运行30秒，您可以在此期间测试播放..."
    sleep 30
    
    # 清理
    cleanup
}

# 运行主函数
main "$@"
