#!/bin/bash
# 修复UDP缓冲区大小警告的脚本
# 这个警告来自quic-go库，需要增加系统UDP缓冲区大小限制

echo "=== 修复UDP缓冲区大小警告 ==="
echo ""
echo "警告信息："
echo "failed to sufficiently increase send buffer size (was: 208 kiB, wanted: 7168 kiB, got: 4096 kiB)"
echo ""
echo "解决方案："
echo "需要增加系统UDP缓冲区大小限制（需要root权限）"
echo ""

# 检查是否有root权限
if [ "$EUID" -ne 0 ]; then 
    echo "⚠️  需要root权限来修改系统参数"
    echo ""
    echo "请使用以下命令（需要root权限）："
    echo ""
    echo "# 临时设置（重启后失效）"
    echo "sudo sysctl -w net.core.rmem_max=8388608"
    echo "sudo sysctl -w net.core.wmem_max=8388608"
    echo "sudo sysctl -w net.core.rmem_default=8388608"
    echo "sudo sysctl -w net.core.wmem_default=8388608"
    echo ""
    echo "# 永久设置（需要写入配置文件）"
    echo "echo 'net.core.rmem_max = 8388608' | sudo tee -a /etc/sysctl.conf"
    echo "echo 'net.core.wmem_max = 8388608' | sudo tee -a /etc/sysctl.conf"
    echo "echo 'net.core.rmem_default = 8388608' | sudo tee -a /etc/sysctl.conf"
    echo "echo 'net.core.wmem_default = 8388608' | sudo tee -a /etc/sysctl.conf"
    echo "sudo sysctl -p"
    echo ""
    echo "或者运行此脚本时使用sudo："
    echo "sudo $0"
    exit 1
fi

echo "✅ 检测到root权限，开始设置..."
echo ""

# 设置UDP缓冲区大小（8MB = 8388608 bytes，约7168 kiB）
sysctl -w net.core.rmem_max=8388608
sysctl -w net.core.wmem_max=8388608
sysctl -w net.core.rmem_default=8388608
sysctl -w net.core.wmem_default=8388608

echo ""
echo "✅ 临时设置完成"
echo ""
echo "当前值："
sysctl net.core.rmem_max net.core.wmem_max net.core.rmem_default net.core.wmem_default
echo ""

# 询问是否永久设置
read -p "是否永久设置（写入/etc/sysctl.conf）？[y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    # 检查是否已存在
    if ! grep -q "net.core.rmem_max" /etc/sysctl.conf; then
        echo "net.core.rmem_max = 8388608" >> /etc/sysctl.conf
        echo "net.core.wmem_max = 8388608" >> /etc/sysctl.conf
        echo "net.core.rmem_default = 8388608" >> /etc/sysctl.conf
        echo "net.core.wmem_default = 8388608" >> /etc/sysctl.conf
        echo "✅ 已写入/etc/sysctl.conf"
    else
        echo "⚠️  配置已存在，跳过写入"
    fi
fi

echo ""
echo "📝 说明："
echo "  - rmem_max/wmem_max: 最大接收/发送缓冲区大小（8MB）"
echo "  - rmem_default/wmem_default: 默认接收/发送缓冲区大小（8MB）"
echo "  - 这些设置可以解决QUIC/UDP缓冲区大小警告"
echo ""
echo "💡 如果使用Docker容器，需要在宿主机上设置，或者在容器启动时添加参数："
echo "   --sysctl net.core.rmem_max=8388608"
echo "   --sysctl net.core.wmem_max=8388608"

