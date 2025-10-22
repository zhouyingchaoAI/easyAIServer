#!/bin/bash
# 配置迁移脚本 - Shell版本
# 用于快速迁移 config.toml

CONFIG_FILE="${1:-../configs/config.toml}"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 配置文件不存在: $CONFIG_FILE"
    echo "使用方法: ./migrate_config.sh <config.toml路径>"
    exit 1
fi

# 创建备份
BACKUP_FILE="${CONFIG_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "✓ 已创建备份: $BACKUP_FILE"

# 使用 awk 迁移配置
awk '
BEGIN { 
    in_task = 0
    has_config_status = 0
    has_preview_image = 0
    task_lines = ""
}

# 检测任务开始
/^\[\[frame_extractor\.tasks\]\]/ {
    if (in_task && task_lines != "") {
        # 输出上一个任务
        print task_lines
        if (!has_config_status) {
            print "config_status = '\''configured'\''"
        }
        if (!has_preview_image) {
            print "preview_image = '\'''\''"
        }
    }
    in_task = 1
    has_config_status = 0
    has_preview_image = 0
    task_lines = $0
    next
}

# 在任务中
in_task == 1 {
    # 检测任务结束（空行或新section）
    if ($0 ~ /^$/ || $0 ~ /^\[/) {
        # 输出任务
        print task_lines
        if (!has_config_status) {
            print "config_status = '\''configured'\''"
        }
        if (!has_preview_image) {
            print "preview_image = '\'''\''"
        }
        print $0
        in_task = 0
        task_lines = ""
        next
    }
    
    # 检查字段
    if ($0 ~ /config_status/) {
        has_config_status = 1
    }
    if ($0 ~ /preview_image/) {
        has_preview_image = 1
    }
    
    task_lines = task_lines "\n" $0
    next
}

# 非任务部分，直接输出
{
    print
}

END {
    # 处理最后一个任务
    if (in_task && task_lines != "") {
        print task_lines
        if (!has_config_status) {
            print "config_status = '\''configured'\''"
        }
        if (!has_preview_image) {
            print "preview_image = '\'''\''"
        }
    }
}
' "$CONFIG_FILE" > "${CONFIG_FILE}.tmp"

# 替换原文件
mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"

echo "✓ 配置迁移完成: $CONFIG_FILE"
echo ""
echo "迁移内容:"
echo "  - 为所有任务添加 config_status = 'configured'"
echo "  - 为所有任务添加 preview_image = ''"
echo ""
echo "请检查配置文件，确认无误后重启服务。"

