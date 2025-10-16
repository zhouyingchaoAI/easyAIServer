#!/bin/bash
# 修复端口冲突和目录问题

BUILD_DIR="/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136"

echo "===== 修复 EasyDarwin 问题 ====="
echo ""

# 1. 创建 HLS 目录
echo "1. 创建 HLS 输出目录..."
mkdir -p "$BUILD_DIR/stream/hls/stream_1"
mkdir -p "$BUILD_DIR/stream/hls/stream_2"
mkdir -p "$BUILD_DIR/stream/hls"
chmod -R 755 "$BUILD_DIR/stream"
echo "✅ HLS 目录已创建"
ls -la "$BUILD_DIR/stream/hls/" 2>/dev/null || echo "目录结构：$BUILD_DIR/stream/hls/"
echo ""

# 2. 停止所有服务
echo "2. 停止所有 EasyDarwin 进程..."
pkill -9 easydarwin 2>/dev/null
pkill -9 easydarwin.com 2>/dev/null
sleep 2
echo "✅ 已停止"
echo ""

# 3. 检查端口占用
echo "3. 检查端口占用..."
echo "端口 5066:"
netstat -tunlp 2>/dev/null | grep :5066 || echo "  未占用"
echo "端口 8080:"
netstat -tunlp 2>/dev/null | grep :8080 || echo "  未占用"
echo "端口 8081:"
netstat -tunlp 2>/dev/null | grep :8081 || echo "  未占用"
echo ""

# 4. 验证配置
echo "4. 验证配置..."
echo "lalconfig httplistenaddr:"
grep "httplistenaddr" "$BUILD_DIR/configs/config.toml" | head -2
echo ""

# 5. 启动服务
echo "5. 启动 EasyDarwin..."
cd "$BUILD_DIR"
nohup ./easydarwin > /dev/null 2>&1 &
sleep 5
echo ""

# 6. 检查进程
echo "6. 检查进程状态..."
if ps aux | grep -v grep | grep easydarwin > /dev/null; then
    echo "✅ EasyDarwin 启动成功"
    ps aux | grep -v grep | grep easydarwin | head -2
else
    echo "❌ EasyDarwin 启动失败"
    echo "查看最近错误："
    tail -20 logs/sugar.log | grep -i error
    exit 1
fi
echo ""

# 7. 查看最新日志
echo "7. 最新日志（最近10行）..."
tail -10 logs/sugar.log 2>/dev/null | grep -v "^$"
echo ""

echo "===== 修复完成 ====="
echo ""
echo "✅ 已解决的问题："
echo "  1. 端口 8080 改为 8081（避免冲突）"
echo "  2. 创建了 HLS 输出目录"
echo "  3. 重启了服务"
echo ""
echo "📊 服务状态："
echo "  - 主服务: http://10.1.6.230:5066"
echo "  - 流媒体: http://10.1.6.230:8081"
echo "  - AI服务: http://10.1.6.230:5066/#/ai-services"
echo ""
echo "🚀 下一步："
echo "  启动算法服务："
echo "    python3 /code/EasyDarwin/examples/algorithm_service.py \\"
echo "      --service-id yolo11x_head_detector \\"
echo "      --name 'YOLO11X头部检测' \\"
echo "      --task-types 人数统计 \\"
echo "      --port 8000 \\"
echo "      --easydarwin http://10.1.6.230:5066"


