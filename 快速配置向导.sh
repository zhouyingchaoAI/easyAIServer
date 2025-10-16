#!/bin/bash

# yanying 快速配置向导 - 交互式配置

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}"
cat << 'EOF'
========================================
   yanying 快速配置向导
========================================
     _     _              _             
    | |   | |            (_)            
 _  | | __| | __ _ _ __  _ _ __   __ _ 
| |_| |/ _` |/ _` | '_ \| | '_ \ / _` |
 \__  | (_| | (_| | | | | | | | | (_| |
    |_/\__,_|\__,_|_| |_|_|_| |_|\__, |
                                  __/ |
                                 |___/ 
    视频智能分析平台
========================================
EOF
echo -e "${NC}"
echo ""

# ============================================
# 配置收集
# ============================================

echo -e "${YELLOW}请回答以下问题来配置您的系统：${NC}"
echo ""

# MinIO配置
echo "【MinIO配置】"
read -p "MinIO地址 [默认: 10.1.6.230:9000]: " MINIO_ENDPOINT
MINIO_ENDPOINT=${MINIO_ENDPOINT:-10.1.6.230:9000}

read -p "MinIO用户名 [默认: admin]: " MINIO_USER
MINIO_USER=${MINIO_USER:-admin}

read -p "MinIO密码 [默认: admin123]: " MINIO_PASS
MINIO_PASS=${MINIO_PASS:-admin123}

read -p "MinIO Bucket [默认: images]: " MINIO_BUCKET
MINIO_BUCKET=${MINIO_BUCKET:-images}

read -p "图片保留天数 [默认: 1]: " RETENTION_DAYS
RETENTION_DAYS=${RETENTION_DAYS:-1}

echo ""

# 性能配置
echo "【性能配置】"
echo "需要的处理速度（张/秒）:"
echo "  1) 1张/秒  (低频监控)"
echo "  2) 5张/秒  (标准监控) ⭐推荐"
echo "  3) 10张/秒 (高频监控)"
echo "  4) 自定义"
read -p "选择 [默认: 2]: " PERF_CHOICE
PERF_CHOICE=${PERF_CHOICE:-2}

case $PERF_CHOICE in
    1)
        FRAME_INTERVAL=1000  # 1张/秒
        SCAN_INTERVAL=5
        CONCURRENT=10
        ;;
    2)
        FRAME_INTERVAL=200   # 5张/秒
        SCAN_INTERVAL=1
        CONCURRENT=50
        ;;
    3)
        FRAME_INTERVAL=100   # 10张/秒
        SCAN_INTERVAL=1
        CONCURRENT=100
        ;;
    4)
        read -p "抽帧间隔(毫秒) [默认: 200]: " FRAME_INTERVAL
        FRAME_INTERVAL=${FRAME_INTERVAL:-200}
        read -p "扫描间隔(秒) [默认: 1]: " SCAN_INTERVAL
        SCAN_INTERVAL=${SCAN_INTERVAL:-1}
        read -p "并发数 [默认: 50]: " CONCURRENT
        CONCURRENT=${CONCURRENT:-50}
        ;;
esac

read -p "算法服务实例数 [默认: 5]: " NUM_INSTANCES
NUM_INSTANCES=${NUM_INSTANCES:-5}

echo ""

# RTSP配置
echo "【视频源配置】"
read -p "RTSP地址 [默认: rtsp://127.0.0.1:15544/live/stream_2]: " RTSP_URL
RTSP_URL=${RTSP_URL:-rtsp://127.0.0.1:15544/live/stream_2}

echo "任务类型:"
echo "  1) 人数统计"
echo "  2) 人员跌倒"
echo "  3) 安全帽检测"
echo "  4) 吸烟检测"
read -p "选择 [默认: 1]: " TASK_CHOICE
TASK_CHOICE=${TASK_CHOICE:-1}

case $TASK_CHOICE in
    1) TASK_TYPE="人数统计" ;;
    2) TASK_TYPE="人员跌倒" ;;
    3) TASK_TYPE="安全帽检测" ;;
    4) TASK_TYPE="吸烟检测" ;;
    *) TASK_TYPE="人数统计" ;;
esac

read -p "任务ID [默认: task_$(date +%Y%m%d)]: " TASK_ID
TASK_ID=${TASK_ID:-task_$(date +%Y%m%d)}

echo ""

# ============================================
# 配置确认
# ============================================

echo -e "${BLUE}"
echo "========================================="
echo "   配置确认"
echo "========================================="
echo -e "${NC}"
echo ""
echo "MinIO:"
echo "  地址: $MINIO_ENDPOINT"
echo "  Bucket: $MINIO_BUCKET"
echo "  保留: ${RETENTION_DAYS}天"
echo ""
echo "性能:"
echo "  抽帧: $(echo "scale=1; 1000/$FRAME_INTERVAL" | bc)张/秒"
echo "  扫描: ${SCAN_INTERVAL}秒/次"
echo "  并发: $CONCURRENT"
echo "  算法实例: $NUM_INSTANCES"
echo ""
echo "视频源:"
echo "  RTSP: $RTSP_URL"
echo "  任务类型: $TASK_TYPE"
echo "  任务ID: $TASK_ID"
echo ""

read -p "确认以上配置并启动？(y/N): " CONFIRM

if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
    echo "取消启动"
    exit 0
fi

echo ""

# ============================================
# 保存配置到文件
# ============================================

cat > /tmp/yanying_startup_config.sh << EOF
#!/bin/bash
# yanying 启动配置（自动生成）
export MINIO_ENDPOINT="$MINIO_ENDPOINT"
export MINIO_ACCESS_KEY="$MINIO_USER"
export MINIO_SECRET_KEY="$MINIO_PASS"
export MINIO_BUCKET="$MINIO_BUCKET"
export RETENTION_DAYS=$RETENTION_DAYS
export FRAME_INTERVAL_MS=$FRAME_INTERVAL
export SCAN_INTERVAL_SEC=$SCAN_INTERVAL
export MAX_CONCURRENT=$CONCURRENT
export NUM_ALGO_INSTANCES=$NUM_INSTANCES
export RTSP_URL="$RTSP_URL"
export TASK_TYPE="$TASK_TYPE"
export TASK_ID="$TASK_ID"
EOF

chmod +x /tmp/yanying_startup_config.sh

echo -e "${GREEN}✅ 配置已保存到: /tmp/yanying_startup_config.sh${NC}"
echo ""

# ============================================
# 调用一键启动脚本
# ============================================

echo -e "${YELLOW}开始启动服务...${NC}"
echo ""

# 导入配置并启动
source /tmp/yanying_startup_config.sh
/code/EasyDarwin/一键启动.sh

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}   配置向导完成！${NC}"
echo -e "${GREEN}=========================================${NC}"

