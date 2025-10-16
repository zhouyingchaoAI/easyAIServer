#!/bin/bash

# MinIO配置修复脚本
# 尝试不同的endpoint格式来解决502问题

CONFIG_FILE="/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350/configs/config.toml"

echo "========================================="
echo "MinIO 配置修复工具"
echo "========================================="
echo ""

# 备份配置文件
echo "1. 备份配置文件..."
cp "$CONFIG_FILE" "${CONFIG_FILE}.backup.$(date +%Y%m%d%H%M%S)"
echo "   ✅ 备份完成: ${CONFIG_FILE}.backup.*"
echo ""

# 方案1: 标准格式（不带协议）
echo "2. 应用配置方案1: 标准格式"
cat > /tmp/minio_config.toml <<'EOF'
[frame_extractor]
enable = true
interval_ms = 1000
output_dir = './snapshots'
store = 'minio'
task_types = ['人数统计', '人员跌倒', '人员离岗', '吸烟检测', '区域入侵', '徘徊检测', '物品遗留', '安全帽检测']

[frame_extractor.minio]
endpoint = '10.1.6.230:9000'
bucket = 'images'
access_key = 'admin'
secret_key = 'admin123'
use_ssl = false
base_path = ''
EOF

# 替换配置文件中的frame_extractor部分
sed -i '/^\[frame_extractor\]/,/^\[ai_analysis\]/{ 
  /^\[frame_extractor\]/r /tmp/minio_config.toml
  /^\[frame_extractor\]/,/^\[ai_analysis\]/{
    /^\[ai_analysis\]/!d
  }
}' "$CONFIG_FILE"

echo "   ✅ 配置已更新"
echo ""

# 显示当前配置
echo "3. 当前MinIO配置:"
grep -A 7 "^\[frame_extractor.minio\]" "$CONFIG_FILE"
echo ""

# 重启服务
echo "4. 重启yanying服务..."
cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161350
pkill -9 -f easydarwin
sleep 2
./easydarwin > /dev/null 2>&1 &
sleep 5

echo "   ✅ 服务已重启"
echo ""

# 检查日志
echo "5. 检查启动日志..."
tail -n 20 logs/20251016_08_00_00.log | grep -E "(minio|502|frame|error)" || echo "   暂无错误"
echo ""

echo "========================================="
echo "配置修复完成！"
echo "========================================="
echo ""
echo "如果还有502错误，请尝试以下操作:"
echo "1. 查看详细日志: tail -f logs/20251016_08_00_00.log"
echo "2. 检查MinIO版本: curl http://10.1.6.230:9000/minio/health/live"
echo "3. 运行完整测试: ./test_minio.sh"
echo ""

