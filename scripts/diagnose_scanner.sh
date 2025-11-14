#!/bin/bash
# 扫描器诊断脚本 - 检查为什么图片没有推送到队列

echo "=== 扫描器诊断 ==="
echo ""

# 检查最近的扫描日志
echo "【最近的扫描日志】"
tail -100 /code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-*/logs/20251113_*.log 2>/dev/null | grep -E "scan statistics|found new images|images added to queue|skipping image with invalid path" | tail -20
echo ""

# 检查队列状态
echo "【当前队列状态】"
curl -s http://localhost:5066/api/v1/ai_analysis/inference_stats 2>/dev/null | python3 -m json.tool 2>/dev/null | grep -E "queue_size|queue_max_size|processed_total|dropped_total"
echo ""

# 检查配置
echo "【扫描器配置】"
grep -E "scan_interval_sec|base_path|basePath" /code/EasyDarwin/configs/config.toml 2>/dev/null | head -5
echo ""

# 检查最近的错误日志
echo "【最近的错误日志】"
tail -100 /code/EasyDarwin/build/EasyDarwin-aarch64-v8.3.3-*/logs/20251113_*.log 2>/dev/null | grep -iE "error|failed|scan minio" | tail -10
echo ""

echo "=== 诊断完成 ==="
echo ""
echo "💡 提示："
echo "1. 如果看到 'skipping image with invalid path structure'，说明图片路径格式不对"
echo "2. 如果看到 'skipped_processed' 很多，说明图片已经被处理过了"
echo "3. 如果看到 'scan statistics' 但 'new_images' 为0，说明没有发现新图片"
echo "4. 检查 base_path 配置是否正确"

