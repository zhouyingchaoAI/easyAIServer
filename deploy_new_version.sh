#!/bin/bash

echo "=========================================="
echo "🚀 EasyDarwin v8.3.6 部署脚本"
echo "=========================================="
echo ""

# 切换到项目目录
cd /code/EasyDarwin

# 1. 停止当前服务
echo "1️⃣ 停止当前服务..."
pkill easydarwin
sleep 2
echo "   ✅ 服务已停止"
echo ""

# 2. 备份旧版本
echo "2️⃣ 备份旧版本..."
if [ -f "./easydarwin" ]; then
    BACKUP_NAME="easydarwin.bak.$(date +%Y%m%d_%H%M%S)"
    cp ./easydarwin "./$BACKUP_NAME"
    echo "   ✅ 已备份到: $BACKUP_NAME"
else
    echo "   ⚠️ 旧版本不存在，跳过备份"
fi
echo ""

# 3. 替换新版本
echo "3️⃣ 替换新版本..."
if [ -f "./easydarwin_fixed" ]; then
    cp ./easydarwin_fixed ./easydarwin
    chmod +x ./easydarwin
    echo "   ✅ 新版本已替换"
else
    echo "   ❌ easydarwin_fixed 不存在！"
    echo "   请先运行: go build -o easydarwin_fixed ./cmd/server"
    exit 1
fi
echo ""

# 4. 检查前端文件
echo "4️⃣ 检查前端文件..."
if [ -d "./build/EasyDarwin-aarch64-v8.3.3-202511040206/web" ]; then
    WEB_INDEX="./build/EasyDarwin-aarch64-v8.3.3-202511040206/web/index.html"
    if [ -f "$WEB_INDEX" ]; then
        WEB_SIZE=$(ls -lh "$WEB_INDEX" | awk '{print $5}')
        WEB_TIME=$(stat -c '%y' "$WEB_INDEX" | cut -d' ' -f1,2 | cut -d'.' -f1)
        echo "   ✅ 前端文件存在"
        echo "   📁 大小: $WEB_SIZE"
        echo "   🕒 时间: $WEB_TIME"
    else
        echo "   ⚠️ index.html不存在"
    fi
else
    echo "   ⚠️ web目录不存在"
fi
echo ""

# 5. 启动服务
echo "5️⃣ 启动服务..."
./easydarwin &
EASYDARWIN_PID=$!
sleep 3

# 检查服务是否启动成功
if ps -p $EASYDARWIN_PID > /dev/null 2>&1; then
    echo "   ✅ 服务启动成功 (PID: $EASYDARWIN_PID)"
else
    echo "   ❌ 服务启动失败"
    echo "   请查看日志: tail -f ./build/*/logs/*.log"
    exit 1
fi
echo ""

# 6. 验证功能
echo "6️⃣ 验证功能..."
sleep 2

# 测试API
API_RESPONSE=$(curl -s http://localhost:5066/api/v1/version 2>&1)
if echo "$API_RESPONSE" | grep -q "version"; then
    echo "   ✅ API可用"
else
    echo "   ⚠️ API未就绪，可能需要更长启动时间"
fi

# 检查算法服务
SERVICES=$(curl -s http://localhost:5066/api/v1/ai_analysis/services 2>&1)
SERVICE_COUNT=$(echo "$SERVICES" | grep -o '"total":[0-9]*' | grep -o '[0-9]*')
if [ -n "$SERVICE_COUNT" ]; then
    echo "   ✅ 算法服务数量: $SERVICE_COUNT"
else
    echo "   ⚠️ 无法获取算法服务信息"
fi
echo ""

# 7. 显示访问信息
echo "=========================================="
echo "✅ 部署完成!"
echo "=========================================="
echo ""
echo "📝 访问地址:"
echo "   Web界面: http://localhost:5066"
echo "   抽帧管理: http://localhost:5066/frame-extractor"
echo "   算法服务: http://localhost:5066/alerts/services"
echo ""
echo "📋 日志位置:"
LATEST_LOG=$(find ./build/*/logs -name "*.log" -type f -mmin -5 | head -1)
if [ -n "$LATEST_LOG" ]; then
    echo "   $LATEST_LOG"
    echo ""
    echo "📊 实时日志:"
    echo "   tail -f $LATEST_LOG"
else
    echo "   ./build/EasyDarwin-aarch64-v8.3.3-202511040206/logs/"
fi
echo ""
echo "🆕 新功能:"
echo "   ✅ 智能负载均衡（基于响应时间）"
echo "   ✅ 性能指标显示（推理时间/总耗时/平均耗时）"
echo "   ✅ 任务列表刷新按钮"
echo "   ✅ 配置回显（从MinIO读取）"
echo "   ✅ 调用次数精确统计"
echo ""
echo "📚 文档:"
echo "   DEPLOYMENT_SUMMARY_2025-11-04.md - 完整部署指南"
echo "   ALGORITHM_SERVICE_PERFORMANCE_METRICS.md - 性能指标功能"
echo "   algorithm_service_with_stats_example.py - Python示例"
echo ""
echo "=========================================="

