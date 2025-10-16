#!/bin/bash

# yanying MinIO 502问题 - 完整修复方案

echo "========================================="
echo "yanying MinIO 502问题 - 完整修复"
echo "========================================="
echo ""

# 方案选择
echo "请选择修复方案:"
echo ""
echo "1. 快速方案 - 切换到本地存储（立即可用）"
echo "2. MinIO方案A - 设置bucket为完全公开"
echo "3. MinIO方案B - 重新编译使用最新MinIO SDK"
echo "4. MinIO方案C - 在本地启动新的MinIO实例"
echo "5. 查看诊断信息"
echo "6. 退出"
echo ""
read -p "请输入选项 (1-6): " choice

case $choice in
  1)
    echo ""
    echo "========================================="
    echo "方案1: 切换到本地存储"
    echo "========================================="
    echo ""
    
    CONFIG_FILE="/code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161428/configs/config.toml"
    
    # 备份配置
    cp "$CONFIG_FILE" "${CONFIG_FILE}.backup.$(date +%Y%m%d%H%M%S)"
    echo "✅ 配置已备份"
    
    # 修改为本地存储
    sed -i "s/store = 'minio'/store = 'local'/" "$CONFIG_FILE"
    sed -i "s/^enable = true  # 启用智能分析/enable = false  # 暂时禁用（需要MinIO）/" "$CONFIG_FILE"
    
    echo "✅ 配置已更新为本地存储模式"
    echo ""
    
    # 重启服务
    cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161428
    pkill -9 easydarwin
    sleep 2
    ./easydarwin > /dev/null 2>&1 &
    sleep 3
    
    echo "✅ 服务已重启"
    echo ""
    echo "配置结果:"
    echo "  - 抽帧: 保存到本地 ./snapshots"
    echo "  - AI分析: 已禁用（需要MinIO）"
    echo "  - 流媒体: 正常工作"
    echo ""
    echo "访问: http://localhost:5066/#/frame-extractor"
    ;;
    
  2)
    echo ""
    echo "========================================="
    echo "方案2: 设置MinIO为完全公开"
    echo "========================================="
    echo ""
    
    # 设置为public
    /tmp/mc anonymous set public test-minio/images
    echo "✅ Bucket设置为public"
    
    # 验证
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://10.1.6.230:9000/images?list-type=2&max-keys=1")
    echo "API状态码: $HTTP_CODE"
    
    # 重启服务
    cd /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161428
    pkill -9 easydarwin
    sleep 2
    ./easydarwin > /dev/null 2>&1 &
    echo ""
    echo "✅ 服务已重启"
    echo "等待20秒后检查日志..."
    sleep 20
    
    # 检查结果
    HAS_502=$(tail -n 30 logs/20251016_08_00_00.log | grep "502 Bad Gateway" | wc -l)
    HAS_FOUND=$(tail -n 30 logs/20251016_08_00_00.log | grep "found new images" | wc -l)
    
    if [ "$HAS_FOUND" -gt 0 ]; then
        echo "✅ 成功！发现新图片"
        tail -n 20 logs/20251016_08_00_00.log | grep "found new"
    elif [ "$HAS_502" -gt 0 ]; then
        echo "❌ 仍有502错误，MinIO可能有其他问题"
    else
        echo "⚠️  请继续观察日志"
    fi
    ;;
    
  3)
    echo ""
    echo "========================================="
    echo "方案3: 重新编译项目"
    echo "========================================="
    echo ""
    
    cd /code/EasyDarwin
    
    echo "1. 更新Go依赖..."
    go get -u github.com/minio/minio-go/v7
    go mod tidy
    
    echo "2. 重新编译..."
    make build/local
    
    echo "3. 复制配置..."
    LATEST_BUILD=$(ls -t build/EasyDarwin-lin-* 2>/dev/null | head -1)
    cp configs/config.toml "$LATEST_BUILD/configs/"
    
    echo "4. 启动新版本..."
    cd "$LATEST_BUILD"
    ./easydarwin &
    
    echo "✅ 重新编译完成"
    ;;
    
  4)
    echo ""
    echo "========================================="
    echo "方案4: 启动本地MinIO"
    echo "========================================="
    echo ""
    
    echo "创建本地MinIO..."
    mkdir -p /tmp/minio-data
    
    echo "下载MinIO..."
    wget -q https://dl.min.io/server/minio/release/linux-amd64/minio -O /tmp/minio
    chmod +x /tmp/minio
    
    echo "启动MinIO..."
    MINIO_ROOT_USER=admin MINIO_ROOT_PASSWORD=admin123 /tmp/minio server /tmp/minio-data --address :19000 --console-address :19001 > /tmp/minio.log 2>&1 &
    sleep 5
    
    echo "配置mc..."
    /tmp/mc alias set local-minio http://localhost:19000 admin admin123
    /tmp/mc mb local-minio/images
    /tmp/mc anonymous set download local-minio/images
    
    echo ""
    echo "✅ 本地MinIO已启动"
    echo ""
    echo "现在修改配置文件:"
    echo "  [frame_extractor.minio]"
    echo "  endpoint = 'localhost:19000'"
    echo ""
    echo "MinIO控制台: http://localhost:19001"
    ;;
    
  5)
    echo ""
    echo "========================================="
    echo "诊断信息"
    echo "========================================="
    echo ""
    
    echo "1. MinIO服务状态:"
    curl -s http://10.1.6.230:9000/minio/health/live > /dev/null && echo "   ✅ 正常" || echo "   ❌ 无法访问"
    
    echo ""
    echo "2. Bucket访问测试:"
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://10.1.6.230:9000/images?list-type=2&max-keys=1")
    echo "   状态码: $HTTP_CODE"
    
    echo ""
    echo "3. yanying服务状态:"
    ps aux | grep easydarwin | grep -v grep | wc -l | xargs echo "   进程数:"
    
    echo ""
    echo "4. 最近的日志:"
    tail -n 20 /code/EasyDarwin/build/EasyDarwin-lin-v8.3.3-202510161428/logs/20251016_08_00_00.log 2>/dev/null | grep -E "502|found new|minio"
    
    echo ""
    echo "5. MinIO中的图片:"
    /tmp/mc ls test-minio/images --recursive 2>/dev/null | wc -l | xargs echo "   图片数量:"
    ;;
    
  6)
    echo "退出"
    exit 0
    ;;
    
  *)
    echo "无效选项"
    exit 1
    ;;
esac

echo ""
echo "========================================="
echo "操作完成"
echo "========================================="

