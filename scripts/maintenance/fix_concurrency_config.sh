#!/bin/bash
# 修复并发和批量写入配置

echo "========================================================================"
echo "修复并发和批量写入配置"
echo "========================================================================"

cd /code/EasyDarwin

# 1. 备份配置
cp configs/config.toml configs/config.toml.bak.$(date +%Y%m%d_%H%M%S)
echo "✓ 已备份配置文件"

# 2. 修改批量写入配置（加快刷新）
sed -i 's/alert_batch_size = 100/alert_batch_size = 10/' configs/config.toml
sed -i 's/alert_batch_interval_sec = 2/alert_batch_interval_sec = 1/' configs/config.toml
echo "✓ 已优化批量写入配置"
echo "  - batch_size: 100 -> 10"
echo "  - batch_interval: 2s -> 1s"

# 3. 临时关闭save_only_with_detection（用于调试）
echo ""
echo "是否临时关闭save_only_with_detection？(保存所有推理结果用于调试)"
echo "  当前: true (只保存有检测结果的告警)"
echo "  改为: false (保存所有推理结果)"
read -p "输入 yes 关闭，或按回车跳过: " response

if [ "$response" = "yes" ]; then
    sed -i 's/save_only_with_detection = true/save_only_with_detection = false/' configs/config.toml
    echo "✓ 已关闭 save_only_with_detection"
else
    echo "✓ 保持 save_only_with_detection = true"
fi

# 4. 显示新配置
echo ""
echo "新配置:"
echo "------------------------------------------------------------------------"
grep -E "(alert_batch_size|alert_batch_interval_sec|save_only_with_detection)" configs/config.toml | grep -v "^#"

# 5. 重启服务
echo ""
echo "是否立即重启服务？"
read -p "输入 yes 重启: " restart

if [ "$restart" = "yes" ]; then
    echo "停止服务..."
    ./stop.sh
    sleep 3
    
    echo "启动服务..."
    ./start.sh
    
    echo "✓ 服务已重启"
    echo ""
    echo "等待60秒后检查新告警..."
    sleep 60
    
    # 检查新告警
    echo "最新告警:"
    sqlite3 configs/config.toml "SELECT COUNT(*), MAX(created_at) FROM alerts;"
else
    echo "跳过重启，请手动重启服务"
fi

echo ""
echo "========================================================================"
echo "修复完成"
echo "========================================================================"
echo "建议监控："
echo "  1. 每分钟检查新告警: watch -n 60 'sqlite3 configs/data.db \"SELECT COUNT(*), MAX(created_at) FROM alerts;\"'"
echo "  2. 查看日志: tail -f logs/sugar.log | grep -E '(batch insert|收到推理请求)'"
echo "  3. 检查告警分布: sqlite3 configs/data.db \"SELECT task_id, COUNT(*) FROM alerts WHERE created_at > datetime('now', '-1 hour') GROUP BY task_id;\""

