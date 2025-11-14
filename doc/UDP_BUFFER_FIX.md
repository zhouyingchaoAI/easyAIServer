# UDP缓冲区大小警告修复指南

## 问题描述

启动服务时出现以下警告：

```
failed to sufficiently increase send buffer size (was: 208 kiB, wanted: 7168 kiB, got: 4096 kiB). 
See https://github.com/quic-go/quic-go/wiki/UDP-Buffer-Sizes for details.
```

这个警告来自 `quic-go` 库，当使用 RTMP over QUIC 功能时会出现。虽然不会影响功能，但可能会影响性能。

## 原因

QUIC 协议需要较大的 UDP 缓冲区来处理高吞吐量的数据传输。系统默认的 UDP 缓冲区大小（通常为 208-4096 kiB）不足以满足 QUIC 的需求（7168 kiB）。

## 解决方案

### 方法1：使用修复脚本（推荐）

```bash
# 需要root权限
sudo ./scripts/fix_udp_buffer.sh
```

### 方法2：手动设置（临时）

```bash
# 需要root权限
sudo sysctl -w net.core.rmem_max=8388608
sudo sysctl -w net.core.wmem_max=8388608
sudo sysctl -w net.core.rmem_default=8388608
sudo sysctl -w net.core.wmem_default=8388608
```

### 方法3：永久设置

```bash
# 需要root权限
echo 'net.core.rmem_max = 8388608' | sudo tee -a /etc/sysctl.conf
echo 'net.core.wmem_max = 8388608' | sudo tee -a /etc/sysctl.conf
echo 'net.core.rmem_default = 8388608' | sudo tee -a /etc/sysctl.conf
echo 'net.core.wmem_default = 8388608' | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### 方法4：Docker容器

如果使用 Docker 容器运行，需要在宿主机上设置，或者在容器启动时添加参数：

```bash
docker run --sysctl net.core.rmem_max=8388608 \
           --sysctl net.core.wmem_max=8388608 \
           --sysctl net.core.rmem_default=8388608 \
           --sysctl net.core.wmem_default=8388608 \
           ...
```

或者在 `docker-compose.yml` 中：

```yaml
services:
  easydarwin:
    sysctls:
      - net.core.rmem_max=8388608
      - net.core.wmem_max=8388608
      - net.core.rmem_default=8388608
      - net.core.wmem_default=8388608
```

## 参数说明

- `net.core.rmem_max`: 最大接收缓冲区大小（8MB = 8388608 bytes）
- `net.core.wmem_max`: 最大发送缓冲区大小（8MB = 8388608 bytes）
- `net.core.rmem_default`: 默认接收缓冲区大小（8MB = 8388608 bytes）
- `net.core.wmem_default`: 默认发送缓冲区大小（8MB = 8388608 bytes）

## 验证

设置后，可以通过以下命令验证：

```bash
sysctl net.core.rmem_max net.core.wmem_max net.core.rmem_default net.core.wmem_default
```

应该看到所有值都是 `8388608`。

## 注意事项

1. **需要root权限**：修改系统参数需要管理员权限
2. **临时 vs 永久**：临时设置重启后失效，永久设置需要写入配置文件
3. **不影响功能**：即使不修复，QUIC 功能仍然可以正常工作，只是可能性能稍差
4. **Docker环境**：如果使用容器，需要在宿主机或容器启动参数中设置

## 相关链接

- [quic-go UDP Buffer Sizes](https://github.com/quic-go/quic-go/wiki/UDP-Buffer-Sizes)
- [Linux sysctl 文档](https://www.kernel.org/doc/Documentation/networking/ip-sysctl.txt)

