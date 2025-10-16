#!/bin/bash

# 测试一键启动功能（不实际启动，只验证配置）

echo "========================================="
echo "测试一键启动脚本"
echo "========================================="
echo ""

# 检查脚本文件
echo "1. 检查脚本文件..."
if [ -f "/code/EasyDarwin/一键启动.sh" ]; then
    echo "  ✅ 一键启动.sh"
else
    echo "  ❌ 一键启动.sh 不存在"
fi

if [ -f "/code/EasyDarwin/快速配置向导.sh" ]; then
    echo "  ✅ 快速配置向导.sh"
else
    echo "  ❌ 快速配置向导.sh 不存在"
fi

echo ""

# 检查依赖
echo "2. 检查依赖..."

# mc工具
if [ -f "/tmp/mc" ]; then
    echo "  ✅ mc工具已安装"
else
    echo "  ⚠️  mc工具未安装"
fi

# curl
if command -v curl &> /dev/null; then
    echo "  ✅ curl已安装"
else
    echo "  ❌ curl未安装"
fi

# python3
if command -v python3 &> /dev/null; then
    echo "  ✅ python3已安装"
else
    echo "  ❌ python3未安装"
fi

echo ""

# 检查MinIO
echo "3. 检查MinIO..."
if curl -s -f "http://10.1.6.230:9000/minio/health/live" > /dev/null 2>&1; then
    echo "  ✅ MinIO服务可访问"
else
    echo "  ❌ MinIO服务无法访问"
fi

echo ""

# 检查构建目录
echo "4. 检查构建目录..."
BUILD_DIR=$(ls -td /code/EasyDarwin/build/EasyDarwin-lin-* 2>/dev/null | head -1)
if [ -n "$BUILD_DIR" ]; then
    echo "  ✅ 构建目录: $BUILD_DIR"
    if [ -f "$BUILD_DIR/easydarwin" ]; then
        echo "  ✅ 可执行文件存在"
    else
        echo "  ❌ 可执行文件不存在"
    fi
else
    echo "  ❌ 未找到构建目录"
fi

echo ""
echo "========================================="
echo "测试完成"
echo "========================================="
echo ""
echo "如果所有检查都通过，可以执行："
echo "  cd /code/EasyDarwin"
echo "  ./一键启动.sh"
echo ""
