#!/bin/bash

# 修复图片移动并发错位问题的部署脚本
# 修复内容: 添加移动锁，确保同一task_id的图片按顺序移动
# 修复日期: 2025-11-06

set -e

echo "========================================"
echo "图片移动并发错位问题修复脚本"
echo "========================================"
echo ""

# 目标运行目录
TARGET_DIR="/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511060831"

# 检查目标目录是否存在
if [ ! -d "$TARGET_DIR" ]; then
    echo "❌ 错误: 目标目录不存在: $TARGET_DIR"
    exit 1
fi

echo "✓ 找到运行目录: $TARGET_DIR"
echo ""

# 确认编译后的程序存在
if [ ! -f "/code/EasyDarwin/easydarwin-fixed-v2" ]; then
    echo "编译修复后的程序..."
    cd /code/EasyDarwin
    if go build -o easydarwin-fixed-v2 ./cmd/server; then
        echo "✓ 编译成功"
    else
        echo "❌ 编译失败"
        exit 1
    fi
else
    echo "✓ 找到编译后的程序"
fi
echo ""

# 停止当前运行的服务
echo "停止运行中的服务..."
cd "$TARGET_DIR"

# 查找进程
PIDS=$(ps aux | grep "easydarwin" | grep -v grep | awk '{print $2}')
if [ -n "$PIDS" ]; then
    for PID in $PIDS; do
        echo "  停止进程 $PID..."
        kill -15 $PID 2>/dev/null || true
    done
    sleep 3
    echo "✓ 服务已停止"
else
    echo "⚠ 未找到运行中的进程"
fi
echo ""

# 备份原程序
echo "备份原程序..."
if [ -f "$TARGET_DIR/easydarwin" ]; then
    BACKUP_NAME="easydarwin.backup.$(date +%Y%m%d_%H%M%S)"
    cp "$TARGET_DIR/easydarwin" "$TARGET_DIR/$BACKUP_NAME"
    echo "✓ 原程序已备份为: $BACKUP_NAME"
else
    echo "⚠ 未找到原程序文件"
fi
echo ""

# 复制新程序
echo "部署修复后的程序..."
cp /code/EasyDarwin/easydarwin-fixed-v2 "$TARGET_DIR/easydarwin"
chmod +x "$TARGET_DIR/easydarwin"
echo "✓ 新程序已部署"
echo ""

# 启动服务
echo "启动服务..."
cd "$TARGET_DIR"
nohup ./easydarwin > logs/easydarwin_$(date +%Y%m%d_%H%M%S).log 2>&1 &
NEW_PID=$!
echo "✓ 服务已启动 (PID: $NEW_PID)"
echo ""

# 等待服务启动
echo "等待服务启动..."
sleep 5

# 检查服务状态
if ps -p $NEW_PID > /dev/null 2>&1; then
    echo "✓ 服务运行正常"
    echo ""
    echo "========================================"
    echo "部署完成！"
    echo "========================================"
    echo ""
    echo "修复内容:"
    echo "  ✓ 修复了任务ID混淆问题（使用已解析的Filename）"
    echo "  ✓ 添加了移动锁机制，确保同一task_id的图片按顺序移动"
    echo "  ✓ 防止并发移动导致的图片内容错位"
    echo "  ✓ 增强了日志记录和一致性验证"
    echo ""
    echo "技术细节:"
    echo "  - 为每个task_id维护一个独立的移动锁"
    echo "  - 同一task_id的图片移动串行执行（保证顺序）"
    echo "  - 不同task_id之间仍然并发执行（保持性能）"
    echo ""
    echo "查看日志:"
    echo "  tail -f $TARGET_DIR/logs/$(ls -t $TARGET_DIR/logs/*.log | head -1 | xargs basename)"
    echo ""
    echo "监控移动操作:"
    echo "  tail -f $TARGET_DIR/logs/*.log | grep 'async image move'"
    echo ""
    echo "验证修复效果:"
    echo "  bash /code/EasyDarwin/verify_move_serialization.sh"
    echo ""
else
    echo "❌ 服务启动失败，请检查日志"
    tail -50 "$TARGET_DIR/logs/$(ls -t $TARGET_DIR/logs/*.log | head -1)"
    exit 1
fi

