#!/bin/bash

# 修复任务ID混淆问题的部署脚本
# 修复日期: 2025-11-06

set -e

echo "================================"
echo "EasyDarwin 任务ID混淆修复脚本"
echo "================================"
echo ""

# 目标运行目录
TARGET_DIR="/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511050915"

# 检查目标目录是否存在
if [ ! -d "$TARGET_DIR" ]; then
    echo "❌ 错误: 目标目录不存在: $TARGET_DIR"
    exit 1
fi

echo "✓ 找到运行目录: $TARGET_DIR"
echo ""

# 编译修复后的程序
echo "1. 编译修复后的程序..."
cd /code/EasyDarwin
if go build -o easydarwin-fixed ./cmd/server; then
    echo "✓ 编译成功"
else
    echo "❌ 编译失败"
    exit 1
fi
echo ""

# 停止当前运行的服务
echo "2. 停止运行中的服务..."
cd "$TARGET_DIR"
if [ -f "stop.sh" ]; then
    bash stop.sh
    echo "✓ 服务已停止"
else
    # 手动查找并停止进程
    PID=$(ps aux | grep "easydarwin" | grep -v "grep" | awk '{print $2}' | head -1)
    if [ -n "$PID" ]; then
        kill -15 $PID
        sleep 2
        echo "✓ 进程 $PID 已停止"
    else
        echo "⚠ 未找到运行中的进程"
    fi
fi
echo ""

# 备份原程序
echo "3. 备份原程序..."
if [ -f "$TARGET_DIR/easydarwin" ]; then
    cp "$TARGET_DIR/easydarwin" "$TARGET_DIR/easydarwin.backup.$(date +%Y%m%d_%H%M%S)"
    echo "✓ 原程序已备份"
else
    echo "⚠ 未找到原程序文件"
fi
echo ""

# 复制新程序
echo "4. 部署修复后的程序..."
cp /code/EasyDarwin/easydarwin-fixed "$TARGET_DIR/easydarwin"
chmod +x "$TARGET_DIR/easydarwin"
echo "✓ 新程序已部署"
echo ""

# 启动服务
echo "5. 启动服务..."
cd "$TARGET_DIR"
if [ -f "start.sh" ]; then
    bash start.sh
    echo "✓ 服务已启动"
else
    # 手动启动
    nohup ./easydarwin > logs/easydarwin.log 2>&1 &
    echo "✓ 服务已后台启动"
fi
echo ""

# 等待服务启动
echo "6. 等待服务启动..."
sleep 3

# 检查服务状态
if ps aux | grep -v grep | grep "easydarwin" > /dev/null; then
    echo "✓ 服务运行正常"
    echo ""
    echo "================================"
    echo "部署完成！"
    echo "================================"
    echo ""
    echo "修复内容:"
    echo "  1. 修复了告警图片路径构建时的任务ID混淆问题"
    echo "  2. 使用已解析的 Filename 字段，避免重复解析路径"
    echo "  3. 增强了日志记录，可以追踪任务ID的完整流程"
    echo "  4. 添加了任务ID一致性验证，及时发现问题"
    echo ""
    echo "查看日志:"
    echo "  tail -f $TARGET_DIR/logs/sugar.log"
    echo ""
    echo "监控关键词:"
    echo "  - 'constructing alert image path' - 告警路径构建"
    echo "  - 'task_id mismatch detected' - 任务ID不匹配告警"
    echo "  - 'parsed image path' - 原始路径解析"
    echo ""
else
    echo "❌ 服务启动失败，请检查日志"
    tail -20 "$TARGET_DIR/logs/sugar.log"
    exit 1
fi

