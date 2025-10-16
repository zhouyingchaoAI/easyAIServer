#!/bin/bash
# EasyDarwin 服务重启脚本

BUILD_DIR="/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136"

echo "===== 重启 EasyDarwin 服务 ====="
echo ""

# 1. 停止所有 EasyDarwin 进程
echo "1. 停止旧进程..."
pkill -9 easydarwin 2>/dev/null
pkill -9 easydarwin.com 2>/dev/null
sleep 2
echo "✅ 旧进程已停止"
echo ""

# 2. 验证配置
echo "2. 验证配置文件..."
echo "Frame Extractor Enable:"
grep -A 1 "^\[frame_extractor\]" "$BUILD_DIR/configs/config.toml" | grep enable
echo "Frame Extractor Store:"
grep "^store = " "$BUILD_DIR/configs/config.toml" | head -1
echo "AI Analysis Enable:"
grep -A 1 "^\[ai_analysis\]" "$BUILD_DIR/configs/config.toml" | grep enable
echo "AI Analysis MQ Type:"
grep "^mq_type = " "$BUILD_DIR/configs/config.toml"
echo ""

# 3. 启动服务
echo "3. 启动 EasyDarwin 服务..."
cd "$BUILD_DIR"
nohup ./easydarwin > /dev/null 2>&1 &
sleep 3
echo ""

# 4. 检查进程
echo "4. 检查进程状态..."
if ps aux | grep -v grep | grep easydarwin > /dev/null; then
    echo "✅ EasyDarwin 服务启动成功"
    ps aux | grep -v grep | grep easydarwin | head -2
else
    echo "❌ EasyDarwin 启动失败"
    exit 1
fi
echo ""

# 5. 查看启动日志
echo "5. 查看最新日志（前20行）..."
sleep 2
tail -20 "$BUILD_DIR/logs/sugar.log" 2>/dev/null || tail -20 "$BUILD_DIR/logs/"*.log 2>/dev/null | tail -20
echo ""

echo "===== 重启完成 ====="
echo ""
echo "📊 访问地址："
echo "  - Web UI: http://10.1.6.230:5066"
echo "  - AI服务: http://10.1.6.230:5066/#/ai-services"
echo "  - 告警页面: http://10.1.6.230:5066/#/alerts"
echo ""
echo "📝 查看实时日志："
echo "  tail -f $BUILD_DIR/logs/sugar.log"

