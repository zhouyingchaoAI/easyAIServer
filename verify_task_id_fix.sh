#!/bin/bash

# 验证任务ID混淆修复效果
# 2025-11-06

echo "========================================"
echo "任务ID混淆修复验证脚本"
echo "========================================"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查项计数
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

function check_item() {
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    local description=$1
    local result=$2
    
    if [ "$result" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $description"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    elif [ "$result" = "FAIL" ]; then
        echo -e "${RED}✗${NC} $description"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    else
        echo -e "${YELLOW}⚠${NC} $description"
    fi
}

# 1. 检查服务是否运行
echo "1. 检查服务状态..."
if ps aux | grep -v grep | grep "easydarwin" > /dev/null; then
    check_item "EasyDarwin 服务运行中" "PASS"
    PID=$(ps aux | grep -v grep | grep "easydarwin" | awk '{print $2}' | head -1)
    echo "   进程ID: $PID"
else
    check_item "EasyDarwin 服务运行中" "FAIL"
    echo -e "${RED}   错误: 服务未运行，请先启动服务${NC}"
    exit 1
fi
echo ""

# 2. 检查日志文件
echo "2. 检查日志文件..."
LOG_DIR="/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511050915/logs"
LOG_FILE="$LOG_DIR/sugar.log"

if [ -f "$LOG_FILE" ]; then
    check_item "日志文件存在" "PASS"
    
    # 检查最近的日志是否包含新的日志格式
    if grep -q "constructing alert image path" "$LOG_FILE"; then
        check_item "发现新版本日志格式（修复已生效）" "PASS"
    else
        check_item "发现新版本日志格式" "WARN"
        echo -e "${YELLOW}   提示: 可能还没有新的告警生成，请等待${NC}"
    fi
else
    check_item "日志文件存在" "FAIL"
    echo -e "${RED}   错误: 未找到日志文件 $LOG_FILE${NC}"
fi
echo ""

# 3. 检查是否有任务ID不匹配错误
echo "3. 检查任务ID不匹配错误..."
if [ -f "$LOG_FILE" ]; then
    MISMATCH_COUNT=$(grep -c "task_id mismatch detected" "$LOG_FILE" 2>/dev/null || echo "0")
    if [ "$MISMATCH_COUNT" -eq 0 ]; then
        check_item "无任务ID不匹配错误" "PASS"
    else
        check_item "无任务ID不匹配错误" "FAIL"
        echo -e "${RED}   发现 $MISMATCH_COUNT 条不匹配错误${NC}"
        echo "   最近的错误:"
        grep "task_id mismatch detected" "$LOG_FILE" | tail -3
    fi
else
    check_item "无任务ID不匹配错误" "WARN"
fi
echo ""

# 4. 检查数据库中的告警记录
echo "4. 检查数据库中的告警记录..."
DB_FILE="/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511050915/configs/data.db"

if [ -f "$DB_FILE" ]; then
    check_item "数据库文件存在" "PASS"
    
    # 检查最近10条告警记录的一致性
    echo "   检查最近10条告警记录..."
    
    # 使用 Python 脚本检查
    python3 << 'EOF'
import sqlite3
import sys

db_path = "/code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-202511050915/configs/data.db"
try:
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    cursor.execute("""
        SELECT id, task_id, image_path, created_at
        FROM alerts
        ORDER BY created_at DESC
        LIMIT 10
    """)
    
    alerts = cursor.fetchall()
    total = len(alerts)
    matched = 0
    mismatched = 0
    
    for alert_id, task_id, image_path, created_at in alerts:
        # 从 image_path 解析 task_id
        parts = image_path.split('/')
        if len(parts) >= 3:
            # alerts/task_type/task_id/filename 格式
            path_task_id = parts[-2]
        else:
            path_task_id = '?'
        
        if task_id == path_task_id:
            matched += 1
        else:
            mismatched += 1
            print(f"   ✗ ID:{alert_id} 记录TaskID:{task_id} != 路径TaskID:{path_task_id}")
    
    conn.close()
    
    print(f"   总记录数: {total}, 匹配: {matched}, 不匹配: {mismatched}")
    
    if mismatched == 0 and total > 0:
        print("   ✓ 所有记录的任务ID都匹配")
        sys.exit(0)
    elif mismatched > 0:
        print(f"   ✗ 发现 {mismatched} 条不匹配记录")
        sys.exit(1)
    else:
        print("   ⚠ 暂无告警记录")
        sys.exit(2)
        
except Exception as e:
    print(f"   ✗ 数据库检查失败: {e}")
    sys.exit(1)
EOF
    
    DB_CHECK_RESULT=$?
    if [ $DB_CHECK_RESULT -eq 0 ]; then
        check_item "数据库记录一致性检查" "PASS"
    elif [ $DB_CHECK_RESULT -eq 2 ]; then
        check_item "数据库记录一致性检查" "WARN"
    else
        check_item "数据库记录一致性检查" "FAIL"
    fi
else
    check_item "数据库文件存在" "FAIL"
    echo -e "${RED}   错误: 未找到数据库文件 $DB_FILE${NC}"
fi
echo ""

# 5. 检查最近的推理日志
echo "5. 检查最近的推理日志..."
if [ -f "$LOG_FILE" ]; then
    RECENT_INFERENCE=$(grep "inference completed and queued" "$LOG_FILE" | tail -1)
    if [ -n "$RECENT_INFERENCE" ]; then
        check_item "发现最近的推理日志" "PASS"
        echo "   最后一条推理:"
        echo "   $RECENT_INFERENCE" | head -c 100
        echo "..."
    else
        check_item "发现最近的推理日志" "WARN"
        echo -e "${YELLOW}   提示: 可能还没有推理任务执行${NC}"
    fi
fi
echo ""

# 6. 检查图片移动日志
echo "6. 检查图片移动日志..."
if [ -f "$LOG_FILE" ]; then
    MOVE_SUCCESS=$(grep -c "async image move succeeded" "$LOG_FILE" 2>/dev/null || echo "0")
    MOVE_FAILED=$(grep -c "async image move failed" "$LOG_FILE" 2>/dev/null || echo "0")
    
    echo "   成功: $MOVE_SUCCESS 次, 失败: $MOVE_FAILED 次"
    
    if [ "$MOVE_FAILED" -eq 0 ]; then
        check_item "图片移动无失败记录" "PASS"
    else
        check_item "图片移动无失败记录" "FAIL"
        echo "   最近的失败:"
        grep "async image move failed" "$LOG_FILE" | tail -3
    fi
fi
echo ""

# 汇总结果
echo "========================================"
echo "验证结果汇总"
echo "========================================"
echo ""
echo "总检查项: $TOTAL_CHECKS"
echo -e "${GREEN}通过: $PASSED_CHECKS${NC}"
echo -e "${RED}失败: $FAILED_CHECKS${NC}"
echo -e "${YELLOW}警告: $((TOTAL_CHECKS - PASSED_CHECKS - FAILED_CHECKS))${NC}"
echo ""

if [ $FAILED_CHECKS -eq 0 ]; then
    echo -e "${GREEN}✓ 修复验证通过！${NC}"
    echo ""
    echo "建议操作:"
    echo "  1. 继续监控日志，确保没有 'task_id mismatch detected' 错误"
    echo "  2. 定期运行此脚本验证系统状态"
    echo ""
    echo "监控命令:"
    echo "  tail -f $LOG_FILE | grep -E '(constructing alert|task_id mismatch|async image move)'"
    exit 0
else
    echo -e "${RED}✗ 发现问题，需要进一步排查${NC}"
    echo ""
    echo "排查建议:"
    echo "  1. 查看完整日志: tail -100 $LOG_FILE"
    echo "  2. 检查服务版本: strings $TARGET_DIR/easydarwin | grep -i version"
    echo "  3. 重新部署修复: bash /code/EasyDarwin/fix_task_id_mismatch.sh"
    exit 1
fi

