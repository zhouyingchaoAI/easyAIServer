#!/bin/bash
# 启用 Frame Extractor 和 AI Analysis 插件

echo "===== 启用插件 ====="

# 等待服务完全启动
sleep 3

# 启用 Frame Extractor
echo "正在启用 Frame Extractor..."
curl -X POST http://10.1.6.230:5066/api/v1/frame_extractor/config \
  -H 'Content-Type: application/json' \
  -d '{
    "enable": true,
    "store": "minio",
    "minio": {
      "endpoint": "10.1.6.230:9000",
      "bucket": "images",
      "access_key": "admin",
      "secret_key": "admin123",
      "use_ssl": false,
      "base_path": ""
    }
  }'
echo ""
echo "✅ Frame Extractor 已启用"
echo ""

echo "请重启服务以使 AI Analysis 生效："
echo "  pkill -9 easydarwin && cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161136 && ./easydarwin &"

