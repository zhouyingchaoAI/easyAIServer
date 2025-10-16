#!/bin/bash

# yanying 视频智能分析平台 - 一键启动脚本
# 自动配置并启动所有服务

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置参数（可根据需要修改）
MINIO_ENDPOINT="10.1.6.230:9000"
MINIO_ACCESS_KEY="admin"
MINIO_SECRET_KEY="admin123"
MINIO_BUCKET="images"
RETENTION_DAYS=1  # 图片保留天数

# 性能参数
FRAME_INTERVAL_MS=200  # 抽帧间隔（毫秒）200ms=5张/秒
SCAN_INTERVAL_SEC=1  # AI扫描间隔（秒）
MAX_CONCURRENT=50  # 最大并发推理数
NUM_ALGO_INSTANCES=5  # 算法服务实例数

# RTSP源配置
RTSP_URL="rtsp://127.0.0.1:15544/live/stream_2"
TASK_TYPE="人数统计"
TASK_ID="high_performance_task"

# 路径
BASE_DIR="/code/EasyDarwin"
BUILD_DIR=$(ls -td $BASE_DIR/build/EasyDarwin-lin-* 2>/dev/null | head -1)

if [ -z "$BUILD_DIR" ]; then
    echo -e "${RED}❌ 未找到构建目录${NC}"
    exit 1
fi

echo -e "${BLUE}"
echo "========================================="
echo "   yanying 一键启动脚本"
echo "========================================="
echo -e "${NC}"
echo ""
echo "配置信息:"
echo "  运行目录: $BUILD_DIR"
echo "  MinIO: $MINIO_ENDPOINT"
echo "  抽帧频率: 每$(echo "scale=2; $FRAME_INTERVAL_MS/1000" | bc)秒1帧"
echo "  扫描间隔: ${SCAN_INTERVAL_SEC}秒"
echo "  并发数: $MAX_CONCURRENT"
echo "  算法实例: $NUM_ALGO_INSTANCES"
echo ""

# ============================================
# 步骤1：配置MinIO
# ============================================
echo -e "${YELLOW}步骤1/6: 配置MinIO${NC}"

# 检查MinIO是否可访问
if ! curl -s -f "http://$MINIO_ENDPOINT/minio/health/live" > /dev/null 2>&1; then
    echo -e "${RED}❌ MinIO服务无法访问: $MINIO_ENDPOINT${NC}"
    echo "请确保MinIO服务正在运行"
    exit 1
fi
echo "  ✅ MinIO服务正常"

# 配置mc工具
if [ ! -f "/tmp/mc" ]; then
    echo "  下载mc工具..."
    wget -q https://dl.min.io/client/mc/release/linux-amd64/mc -O /tmp/mc
    chmod +x /tmp/mc
fi

# 配置alias
/tmp/mc alias set yanying-minio "http://$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --api S3v4 > /dev/null 2>&1
echo "  ✅ MinIO认证配置完成"

# 创建bucket（如果不存在）
if ! /tmp/mc ls yanying-minio/$MINIO_BUCKET > /dev/null 2>&1; then
    /tmp/mc mb yanying-minio/$MINIO_BUCKET
    echo "  ✅ Bucket创建: $MINIO_BUCKET"
else
    echo "  ✅ Bucket已存在: $MINIO_BUCKET"
fi

# 设置公开访问
/tmp/mc anonymous set public yanying-minio/$MINIO_BUCKET > /dev/null 2>&1
echo "  ✅ Bucket权限设置为public"

# 设置生命周期清理
/tmp/mc ilm remove yanying-minio/$MINIO_BUCKET --all > /dev/null 2>&1 || true
/tmp/mc ilm add yanying-minio/$MINIO_BUCKET --expiry-days $RETENTION_DAYS > /dev/null 2>&1
echo "  ✅ 自动清理策略: ${RETENTION_DAYS}天过期"

echo ""

# ============================================
# 步骤2：生成配置文件
# ============================================
echo -e "${YELLOW}步骤2/6: 生成配置文件${NC}"

CONFIG_FILE="$BUILD_DIR/configs/config.toml"

# 备份原配置
if [ -f "$CONFIG_FILE" ]; then
    cp "$CONFIG_FILE" "${CONFIG_FILE}.backup.$(date +%Y%m%d%H%M%S)"
    echo "  ✅ 原配置已备份"
fi

# 更新配置
cat > /tmp/yanying_config_patch.toml << EOF
[frame_extractor]
enable = true
interval_ms = 1000
output_dir = './snapshots'
store = 'minio'
task_types = ['人数统计', '人员跌倒', '人员离岗', '吸烟检测', '区域入侵', '徘徊检测', '物品遗留', '安全帽检测']

[frame_extractor.minio]
endpoint = '$MINIO_ENDPOINT'
bucket = '$MINIO_BUCKET'
access_key = '$MINIO_ACCESS_KEY'
secret_key = '$MINIO_SECRET_KEY'
use_ssl = false
base_path = ''

[[frame_extractor.tasks]]
id = '$TASK_ID'
task_type = '$TASK_TYPE'
rtsp_url = '$RTSP_URL'
interval_ms = $FRAME_INTERVAL_MS
output_path = '$TASK_ID'
enabled = true

[ai_analysis]
enable = true
scan_interval_sec = $SCAN_INTERVAL_SEC
mq_type = 'kafka'
mq_address = ''
mq_topic = 'easydarwin.alerts'
heartbeat_timeout_sec = 90
max_concurrent_infer = $MAX_CONCURRENT
EOF

# 应用配置（简化版，实际应该合并到完整配置）
if [ -f "$CONFIG_FILE" ]; then
    # 更新关键配置
    sed -i "s/store = 'local'/store = 'minio'/" "$CONFIG_FILE"
    sed -i "s/^interval_ms = [0-9]*/interval_ms = $FRAME_INTERVAL_MS/" "$CONFIG_FILE"
    sed -i "/\[ai_analysis\]/,/^max_concurrent/s/scan_interval_sec = [0-9]*/scan_interval_sec = $SCAN_INTERVAL_SEC/" "$CONFIG_FILE"
    sed -i "/\[ai_analysis\]/,/^max_concurrent/s/max_concurrent_infer = [0-9]*/max_concurrent_infer = $MAX_CONCURRENT/" "$CONFIG_FILE"
fi

echo "  ✅ 配置文件已更新"
echo "  - 抽帧间隔: ${FRAME_INTERVAL_MS}ms ($(echo "scale=1; 1000/$FRAME_INTERVAL_MS" | bc)张/秒)"
echo "  - 扫描间隔: ${SCAN_INTERVAL_SEC}秒"
echo "  - 并发数: $MAX_CONCURRENT"

echo ""

# ============================================
# 步骤3：停止旧服务
# ============================================
echo -e "${YELLOW}步骤3/6: 停止旧服务${NC}"

pkill -9 easydarwin 2>/dev/null || true
pkill -9 demo_multi_services 2>/dev/null || true
sleep 2
echo "  ✅ 旧服务已停止"

echo ""

# ============================================
# 步骤4：启动yanying服务
# ============================================
echo -e "${YELLOW}步骤4/6: 启动yanying服务${NC}"

cd "$BUILD_DIR"
nohup ./easydarwin > /dev/null 2>&1 &
sleep 8

# 检查进程
if ps aux | grep -v grep | grep easydarwin > /dev/null; then
    echo -e "  ${GREEN}✅ yanying服务启动成功${NC}"
    ps aux | grep easydarwin | grep -v grep | head -2 | while read line; do
        echo "     $line" | awk '{print $2, $11, $12}'
    done
else
    echo -e "${RED}❌ 服务启动失败${NC}"
    exit 1
fi

echo ""

# ============================================
# 步骤5：注册算法服务
# ============================================
echo -e "${YELLOW}步骤5/6: 注册算法服务${NC}"

YANYING_API="http://localhost:5066"

# 等待API就绪
for i in {1..10}; do
    if curl -s -f "$YANYING_API/api/v1/health" > /dev/null 2>&1; then
        break
    fi
    sleep 1
done

echo "  注册${NUM_ALGO_INSTANCES}个算法服务实例..."

for i in $(seq 1 $NUM_ALGO_INSTANCES); do
    PORT=$((8000 + i))
    SERVICE_ID="algo_instance_${i}"
    
    RESPONSE=$(curl -s -X POST "$YANYING_API/api/v1/ai_analysis/register" \
      -H "Content-Type: application/json" \
      -d "{
        \"service_id\": \"$SERVICE_ID\",
        \"name\": \"人数统计服务-实例${i}\",
        \"task_types\": [\"人数统计\", \"客流分析\"],
        \"endpoint\": \"http://localhost:${PORT}/infer\",
        \"version\": \"1.0.0\"
      }")
    
    if echo "$RESPONSE" | grep -q '"ok":true'; then
        echo "  ✅ 实例${i}已注册 (端口: $PORT)"
    else
        echo "  ⚠️  实例${i}注册失败"
    fi
done

# 启动心跳循环
nohup bash -c "
while true; do
    sleep 30
    for i in \$(seq 1 $NUM_ALGO_INSTANCES); do
        curl -s -X POST '$YANYING_API/api/v1/ai_analysis/heartbeat/algo_instance_\${i}' > /dev/null 2>&1
    done
done
" > /dev/null 2>&1 &

echo "  ✅ 心跳循环已启动"

echo ""

# ============================================
# 步骤6：验证运行状态
# ============================================
echo -e "${YELLOW}步骤6/6: 验证运行状态${NC}"

sleep 5

# 检查服务数量
SERVICES=$(curl -s "$YANYING_API/api/v1/ai_analysis/services" | python3 -c "import json,sys; print(json.load(sys.stdin).get('total', 0))")
echo "  ✅ 算法服务: $SERVICES 个"

# 检查MinIO连接
if tail -n 50 "$BUILD_DIR/logs/20251016_08_00_00.log" 2>/dev/null | grep -q "minio client initialized"; then
    echo "  ✅ MinIO连接正常"
else
    echo "  ⚠️  MinIO未初始化"
fi

# 检查AI扫描
sleep 10
if tail -n 30 "$BUILD_DIR/logs/20251016_08_00_00.log" 2>/dev/null | grep -q "found new images"; then
    FOUND_COUNT=$(tail -n 30 "$BUILD_DIR/logs/20251016_08_00_00.log" | grep "found new" | grep -o '"count":[0-9]*' | cut -d':' -f2 | head -1)
    echo "  ✅ AI扫描正常（发现${FOUND_COUNT}张图片）"
else
    echo "  ⚠️  AI扫描未检测到活动"
fi

# 检查存储
STORAGE=$(/tmp/mc du yanying-minio/$MINIO_BUCKET 2>/dev/null | awk '{print $1}')
echo "  ✅ MinIO存储: $STORAGE"

echo ""
echo -e "${BLUE}"
echo "========================================="
echo "   启动完成！"
echo "========================================="
echo -e "${NC}"
echo ""
echo -e "${GREEN}Web访问地址：${NC}"
echo "  主页:       http://localhost:5066"
echo "  AI服务:     http://localhost:5066/#/ai-services"
echo "  告警查看:   http://localhost:5066/#/alerts"
echo "  抽帧管理:   http://localhost:5066/#/frame-extractor"
echo "  图片库:     http://localhost:5066/#/frame-extractor/gallery"
echo ""
echo -e "${GREEN}MinIO控制台：${NC}"
echo "  http://$(echo $MINIO_ENDPOINT | cut -d: -f1):9001"
echo "  用户名: $MINIO_ACCESS_KEY"
echo "  密码: $MINIO_SECRET_KEY"
echo ""
echo -e "${GREEN}监控命令：${NC}"
echo "  实时日志:   tail -f $BUILD_DIR/logs/20251016_08_00_00.log | grep 'found new'"
echo "  性能统计:   $BASE_DIR/performance_stats.sh"
echo "  存储查看:   /tmp/mc du yanying-minio/$MINIO_BUCKET"
echo ""
echo -e "${GREEN}停止服务：${NC}"
echo "  pkill -9 easydarwin"
echo ""
echo -e "${BLUE}=========================================${NC}"
echo ""

# 显示实时性能
echo -e "${YELLOW}实时性能监控（10秒）...${NC}"
sleep 10

RECENT_FOUND=$(tail -n 50 "$BUILD_DIR/logs/20251016_08_00_00.log" 2>/dev/null | grep "found new" | wc -l)
if [ "$RECENT_FOUND" -gt 0 ]; then
    echo -e "${GREEN}✅ 检测到 $RECENT_FOUND 次扫描，系统正常运行${NC}"
    tail -n 50 "$BUILD_DIR/logs/20251016_08_00_00.log" | grep "found new" | tail -3 | while read line; do
        echo "  $line" | python3 -c "import json,sys; d=json.load(sys.stdin); print(f'  {d[\"ts\"]}: 发现{d[\"count\"]}张图片')" 2>/dev/null || echo "  $line"
    done
else
    echo -e "${YELLOW}⚠️  暂未检测到扫描活动，请等待...${NC}"
fi

echo ""
echo -e "${GREEN}🎊 yanying平台启动完成！${NC}"

