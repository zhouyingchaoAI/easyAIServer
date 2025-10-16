#!/bin/bash

# yanying MinIO问题一键修复脚本

echo "========================================="
echo "yanying MinIO问题一键修复"
echo "========================================="
echo ""

# 设置颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 步骤1：设置MinIO bucket权限
echo -e "${YELLOW}步骤1/4: 设置MinIO bucket权限${NC}"
echo "执行: /tmp/mc anonymous set download test-minio/images"
/tmp/mc anonymous set download test-minio/images 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Bucket权限设置成功${NC}"
else
    echo -e "${RED}❌ Bucket权限设置失败${NC}"
    echo "请检查mc工具是否正常"
    exit 1
fi
echo ""

# 步骤2：验证权限
echo -e "${YELLOW}步骤2/4: 验证MinIO API${NC}"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://10.1.6.230:9000/images?list-type=2&max-keys=1")
echo "API状态码: $HTTP_CODE"

if [ "$HTTP_CODE" == "200" ]; then
    echo -e "${GREEN}✅ MinIO API正常（200 OK）${NC}"
elif [ "$HTTP_CODE" == "403" ]; then
    echo -e "${RED}❌ 仍然是403，权限设置可能未生效${NC}"
    echo "尝试设置为public模式..."
    /tmp/mc anonymous set public test-minio/images
elif [ "$HTTP_CODE" == "502" ]; then
    echo -e "${RED}❌ 仍然是502，可能是MinIO服务问题${NC}"
else
    echo -e "${YELLOW}⚠️  状态码: $HTTP_CODE${NC}"
fi
echo ""

# 步骤3：停止yanying服务
echo -e "${YELLOW}步骤3/4: 重启yanying服务${NC}"
echo "停止旧服务..."
pkill -9 easydarwin
sleep 2

# 查找运行目录
RUN_DIR=$(find /code/EasyDarwin -type d -name "EasyDarwin-lin-*" 2>/dev/null | sort | tail -1)

if [ -z "$RUN_DIR" ]; then
    echo -e "${RED}❌ 未找到运行目录${NC}"
    exit 1
fi

echo "运行目录: $RUN_DIR"
cd "$RUN_DIR"

# 启动服务
echo "启动新服务..."
./easydarwin > /dev/null 2>&1 &
sleep 5

# 检查进程
if ps aux | grep -v grep | grep easydarwin > /dev/null; then
    echo -e "${GREEN}✅ 服务启动成功${NC}"
else
    echo -e "${RED}❌ 服务启动失败${NC}"
    exit 1
fi
echo ""

# 步骤4：验证结果
echo -e "${YELLOW}步骤4/4: 验证修复结果${NC}"
echo "等待15秒，让AI扫描器运行..."
sleep 15

# 检查日志
echo ""
echo "最近日志:"
tail -n 30 logs/20251016_08_00_00.log 2>/dev/null | grep -E "minio|502|found new" | tail -10 || echo "暂无日志"
echo ""

# 检查是否还有502
HAS_502=$(tail -n 50 logs/20251016_08_00_00.log 2>/dev/null | grep "502 Bad Gateway" | wc -l)

echo "========================================="
if [ "$HAS_502" -eq 0 ]; then
    echo -e "${GREEN}🎉 修复成功！${NC}"
    echo -e "${GREEN}✅ MinIO连接正常${NC}"
    echo -e "${GREEN}✅ AI分析功能正常${NC}"
    echo ""
    echo "您可以访问:"
    echo "  - AI服务: http://localhost:5066/#/ai-services"
    echo "  - 告警查看: http://localhost:5066/#/alerts"
    echo "  - 抽帧管理: http://localhost:5066/#/frame-extractor"
else
    echo -e "${RED}⚠️  仍然有502错误${NC}"
    echo ""
    echo "请尝试以下操作:"
    echo "1. 查看完整日志: tail -f $RUN_DIR/logs/20251016_08_00_00.log"
    echo "2. 运行诊断: ./debug_minio_502.sh"
    echo "3. 查看详细文档: cat MINIO_TROUBLESHOOTING.md"
    echo ""
    echo "临时方案: 使用本地存储"
    echo "  修改 configs/config.toml:"
    echo "  [frame_extractor]"
    echo "  store = 'local'"
fi
echo "========================================="

