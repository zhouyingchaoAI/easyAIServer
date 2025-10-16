#!/bin/bash
# 一键修复并重启 EasyDarwin

BUILD_DIR="/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136"
CONFIG_FILE="$BUILD_DIR/configs/config.toml"

echo "===== EasyDarwin 一键修复 ====="
echo ""

# 1. 停止服务
echo "1. 停止旧进程..."
pkill -9 easydarwin 2>/dev/null
pkill -9 easydarwin.com 2>/dev/null
sleep 2
echo "✅ 已停止"
echo ""

# 2. 备份配置
echo "2. 备份配置文件..."
cp "$CONFIG_FILE" "$CONFIG_FILE.backup.$(date +%Y%m%d%H%M%S)"
echo "✅ 已备份"
echo ""

# 3. 修复配置
echo "3. 修复配置..."

# 启用 Frame Extractor
sed -i '/^\[frame_extractor\]/,/^\[frame_extractor.minio\]/ s/^enable = false/enable = true/' "$CONFIG_FILE"
sed -i '/^\[frame_extractor\]/,/^\[frame_extractor.minio\]/ s/^store = '\''local'\''/store = '\''minio'\''/' "$CONFIG_FILE"

# 启用 AI Analysis
sed -i '/^\[ai_analysis\]/,/^$/ s/^enable = false/enable = true/' "$CONFIG_FILE"

# 修复 mq_type（不能为空）
sed -i '/^\[ai_analysis\]/,/^$/ s/^mq_type = '\'''\''/mq_type = '\''kafka'\''/' "$CONFIG_FILE"

# 清空 mq_address（禁用 Kafka）
sed -i '/^\[ai_analysis\]/,/^$/ s/^mq_address = '\''localhost:9092'\''/mq_address = '\'''\''/' "$CONFIG_FILE"

echo "✅ 配置已修复"
echo ""

# 4. 验证配置
echo "4. 验证关键配置..."
echo "Frame Extractor:"
grep -A 3 "^\[frame_extractor\]" "$CONFIG_FILE" | grep -E "enable|store"
echo ""
echo "AI Analysis:"
grep -A 5 "^\[ai_analysis\]" "$CONFIG_FILE" | grep -E "enable|mq_type|mq_address"
echo ""

# 5. 启动服务
echo "5. 启动服务..."
cd "$BUILD_DIR"
nohup ./easydarwin > /dev/null 2>&1 &
echo "等待服务启动..."
sleep 5
echo ""

# 6. 检查进程
echo "6. 检查进程..."
if ps aux | grep -v grep | grep easydarwin > /dev/null; then
    echo "✅ 服务启动成功"
    ps aux | grep -v grep | grep easydarwin | head -2
else
    echo "❌ 服务启动失败"
    echo "查看日志："
    tail -20 "$BUILD_DIR/logs/sugar.log"
    exit 1
fi
echo ""

# 7. 验证 AI 插件
echo "7. 验证 AI 分析插件..."
sleep 2
if grep -q "AI analysis plugin started successfully" "$BUILD_DIR/logs/sugar.log"; then
    echo "✅ AI 分析插件启动成功"
elif grep -q "minio client initialized" "$BUILD_DIR/logs/sugar.log"; then
    echo "✅ MinIO 客户端初始化成功"
    echo "⚠️  等待 AI 分析插件完全启动..."
else
    echo "⚠️  AI 分析插件可能未成功启动"
    echo "最近日志："
    tail -10 "$BUILD_DIR/logs/sugar.log" | grep -i "ai\|error\|fatal"
fi
echo ""

# 8. 测试 API
echo "8. 测试 API..."
RESPONSE=$(curl -s http://10.1.6.230:5066/api/v1/ai_analysis/services 2>/dev/null)
if echo "$RESPONSE" | grep -q '"services"'; then
    echo "✅ AI 分析 API 正常"
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
else
    echo "❌ AI 分析 API 异常"
    echo "响应: $RESPONSE"
fi
echo ""

echo "===== 修复完成 ====="
echo ""
echo "📋 下一步："
echo "1. 启动算法服务："
echo "   cd /code/EasyDarwin"
echo "   python3 examples/algorithm_service.py \\"
echo "     --service-id yolo11x_head_detector \\"
echo "     --name 'YOLO11X头部检测' \\"
echo "     --task-types 人数统计 \\"
echo "     --port 8000 \\"
echo "     --easydarwin http://10.1.6.230:5066"
echo ""
echo "2. 查看算法服务："
echo "   http://10.1.6.230:5066/#/ai-services"
echo ""
echo "3. 查看实时日志："
echo "   tail -f $BUILD_DIR/logs/sugar.log"
echo ""
echo "⚠️  重要提示："
echo "- 从 Web UI 保存 MinIO 配置后，需要重新运行此脚本"
echo "- 或者保存后手动执行: pkill -9 easydarwin && cd $BUILD_DIR && ./easydarwin &"

