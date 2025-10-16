# yanying 部署指南

本文档详细说明如何在不同环境下部署yanying视频智能分析平台。

---

## 目录

- [系统要求](#系统要求)
- [快速部署](#快速部署)
- [生产环境部署](#生产环境部署)
- [Docker部署](#docker部署)
- [Kubernetes部署](#kubernetes部署)
- [性能优化](#性能优化)
- [监控告警](#监控告警)
- [常见问题](#常见问题)

---

## 系统要求

### 最低配置

| 组件 | 要求 |
|------|------|
| CPU | 2核 |
| 内存 | 4GB |
| 磁盘 | 50GB SSD |
| 操作系统 | Linux/Windows/macOS |
| 网络 | 100Mbps |

**支持规模**：10-20路摄像头

### 推荐配置

| 组件 | 要求 |
|------|------|
| CPU | 8核+ |
| 内存 | 16GB+ |
| 磁盘 | 500GB SSD |
| 操作系统 | Linux (Ubuntu 20.04+ / CentOS 7+) |
| 网络 | 千兆网卡 |

**支持规模**：50-100路摄像头

### 生产环境配置

| 组件 | 要求 |
|------|------|
| CPU | 16核+ (Intel Xeon / AMD EPYC) |
| 内存 | 64GB+ ECC内存 |
| 磁盘 | 2TB+ NVMe SSD (RAID 10) |
| 操作系统 | Linux (Ubuntu 22.04 LTS) |
| 网络 | 万兆网卡 + 冗余链路 |
| GPU | (可选) NVIDIA T4/V100 |

**支持规模**：100-500路摄像头

---

## 快速部署

### 方式1：源码编译（推荐）

#### 1. 安装Go环境

```bash
# Ubuntu/Debian
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version  # 应该显示 go version go1.23.0 linux/amd64
```

#### 2. 克隆项目

```bash
git clone https://github.com/EasyDarwin/EasyDarwin.git
cd EasyDarwin
```

#### 3. 编译

```bash
# Linux
make build/linux

# Windows (使用 git bash)
make build/windows

# macOS
make build/darwin
```

编译完成后，在 `build/` 目录下会生成可执行文件。

#### 4. 启动服务

```bash
cd build/EasyDarwin-lin-v8.3.3-*
./easydarwin

# 或者指定配置文件
./easydarwin -conf /path/to/config.toml
```

#### 5. 访问Web界面

打开浏览器访问：`http://localhost:10086`

默认端口可在配置文件中修改。

---

## 生产环境部署

### 架构设计

```
                        ┌─────────────┐
                        │   用户      │
                        └──────┬──────┘
                               │
                        ┌──────▼──────┐
                        │  Nginx LB   │
                        │  (SSL/TLS)  │
                        └──────┬──────┘
                               │
               ┌───────────────┼───────────────┐
               │               │               │
        ┌──────▼──────┐ ┌─────▼──────┐ ┌─────▼──────┐
        │  yanying-1  │ │ yanying-2  │ │ yanying-3  │
        │  (主节点)    │ │  (节点)     │ │  (节点)     │
        └──────┬──────┘ └─────┬──────┘ └─────┬──────┘
               │               │               │
               └───────────────┼───────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
   ┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐
   │   MinIO     │     │    Kafka    │     │  Prometheus │
   │   集群      │     │    集群     │     │   + Grafana │
   └─────────────┘     └─────────────┘     └─────────────┘
```

### 步骤1：准备基础设施

#### 1.1 部署MinIO集群

```bash
# 使用Docker Compose部署MinIO集群
version: '3.8'

services:
  minio1:
    image: minio/minio
    command: server --console-address ":9001" http://minio{1...4}/data
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: admin123456
    volumes:
      - /data/minio1:/data
    ports:
      - "9000:9000"
      - "9001:9001"

  minio2:
    image: minio/minio
    command: server --console-address ":9001" http://minio{1...4}/data
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: admin123456
    volumes:
      - /data/minio2:/data

  minio3:
    image: minio/minio
    command: server --console-address ":9001" http://minio{1...4}/data
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: admin123456
    volumes:
      - /data/minio3:/data

  minio4:
    image: minio/minio
    command: server --console-address ":9001" http://minio{1...4}/data
    environment:
      MINIO_ROOT_USER: admin
      MINIO_ROOT_PASSWORD: admin123456
    volumes:
      - /data/minio4:/data

# 启动
docker-compose up -d
```

#### 1.2 部署Kafka集群

```bash
# 使用Docker Compose部署Kafka集群
version: '3.8'

services:
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    volumes:
      - /data/zookeeper:/var/lib/zookeeper

  kafka1:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka1:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 3
    volumes:
      - /data/kafka1:/var/lib/kafka
    ports:
      - "9092:9092"

  kafka2:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    environment:
      KAFKA_BROKER_ID: 2
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka2:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 3
    volumes:
      - /data/kafka2:/var/lib/kafka
    ports:
      - "9093:9092"

  kafka3:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    environment:
      KAFKA_BROKER_ID: 3
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka3:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 3
    volumes:
      - /data/kafka3:/var/lib/kafka
    ports:
      - "9094:9092"

# 启动
docker-compose up -d

# 创建topic
docker exec -it kafka1 kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --replication-factor 3 \
  --partitions 10 \
  --topic easydarwin.alerts
```

### 步骤2：配置yanying

创建生产环境配置文件 `config.prod.toml`：

```toml
[server]
http_port = 10086
rtsp_port = 554
rtmp_port = 1935

[database]
path = "/var/lib/yanying/data.db"
max_open_conns = 100
max_idle_conns = 10
conn_max_lifetime = 3600

[frame_extractor]
enable = true
store = 'minio'
scan_only = false

[frame_extractor.minio]
endpoint = 'minio-lb.internal:9000'  # MinIO负载均衡地址
access_key = 'admin'
secret_key = 'admin123456'
bucket = 'snapshots'
use_ssl = false
region = 'us-east-1'

[ai_analysis]
enable = true
scan_interval_sec = 5  # 生产环境缩短扫描间隔
mq_type = 'kafka'
mq_address = 'kafka1:9092,kafka2:9092,kafka3:9092'  # Kafka集群地址
mq_topic = 'easydarwin.alerts'
heartbeat_timeout_sec = 90
max_concurrent_infer = 20  # 增加并发数

[log]
level = 'info'
file = '/var/log/yanying/app.log'
max_size = 100  # MB
max_backups = 10
max_age = 30  # days
```

### 步骤3：部署yanying实例

#### 3.1 创建systemd服务

创建 `/etc/systemd/system/yanying.service`：

```ini
[Unit]
Description=yanying Video Analysis Platform
After=network.target

[Service]
Type=simple
User=yanying
Group=yanying
WorkingDirectory=/opt/yanying
ExecStart=/opt/yanying/easydarwin -conf /etc/yanying/config.prod.toml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

# 安全选项
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

#### 3.2 启动服务

```bash
# 创建用户和目录
sudo useradd -r -s /bin/false yanying
sudo mkdir -p /opt/yanying /etc/yanying /var/lib/yanying /var/log/yanying
sudo chown -R yanying:yanying /opt/yanying /var/lib/yanying /var/log/yanying

# 复制文件
sudo cp easydarwin /opt/yanying/
sudo cp config.prod.toml /etc/yanying/
sudo chmod +x /opt/yanying/easydarwin

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable yanying
sudo systemctl start yanying

# 查看状态
sudo systemctl status yanying
sudo journalctl -u yanying -f
```

### 步骤4：配置Nginx负载均衡

创建 `/etc/nginx/conf.d/yanying.conf`：

```nginx
upstream yanying_backend {
    # 负载均衡策略：ip_hash（会话保持）
    ip_hash;
    
    server 192.168.1.10:10086 weight=1 max_fails=3 fail_timeout=30s;
    server 192.168.1.11:10086 weight=1 max_fails=3 fail_timeout=30s;
    server 192.168.1.12:10086 weight=1 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;
    server_name yanying.example.com;
    
    # 重定向到HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yanying.example.com;
    
    # SSL证书配置
    ssl_certificate /etc/nginx/ssl/yanying.crt;
    ssl_certificate_key /etc/nginx/ssl/yanying.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # 日志
    access_log /var/log/nginx/yanying-access.log;
    error_log /var/log/nginx/yanying-error.log;
    
    # 客户端上传限制
    client_max_body_size 100M;
    
    location / {
        proxy_pass http://yanying_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    # 健康检查
    location /health {
        proxy_pass http://yanying_backend/api/v1/health;
        access_log off;
    }
}
```

重启Nginx：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

### 步骤5：部署算法服务

#### 5.1 创建算法服务Dockerfile

```dockerfile
# Dockerfile
FROM python:3.9-slim

WORKDIR /app

# 安装依赖
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 复制代码
COPY algorithm_service.py .
COPY models/ ./models/

# 暴露端口
EXPOSE 8000

# 启动服务
CMD ["python", "algorithm_service.py", \
     "--service-id", "people_counter_v1", \
     "--task-types", "人数统计", \
     "--port", "8000", \
     "--model", "models/yolov8n.pt"]
```

#### 5.2 使用Docker Compose部署多个算法服务

```yaml
# docker-compose.algorithm.yml
version: '3.8'

services:
  people-counter:
    build: ./algorithm-services/people-counter
    environment:
      - YANYING_HOST=http://yanying-lb.internal
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '2'
          memory: 4G
    restart: always

  helmet-detector:
    build: ./algorithm-services/helmet-detector
    environment:
      - YANYING_HOST=http://yanying-lb.internal
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 4G
    restart: always

  fall-detector:
    build: ./algorithm-services/fall-detector
    environment:
      - YANYING_HOST=http://yanying-lb.internal
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 4G
    restart: always
```

启动：

```bash
docker-compose -f docker-compose.algorithm.yml up -d
```

---

## Docker部署

### 单机Docker部署

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  yanying:
    image: easydarwin/yanying:latest
    ports:
      - "10086:10086"
      - "554:554"
      - "1935:1935"
    volumes:
      - ./config.toml:/app/config.toml
      - yanying-data:/var/lib/yanying
    environment:
      - YANYING_CONFIG=/app/config.toml
    restart: unless-stopped

  minio:
    image: minio/minio
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    command: server /data --console-address ":9001"
    volumes:
      - minio-data:/data
    restart: unless-stopped

  kafka:
    image: confluentinc/cp-kafka:latest
    ports:
      - "9092:9092"
    environment:
      - KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181
      - KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092
    depends_on:
      - zookeeper
    restart: unless-stopped

  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      - ZOOKEEPER_CLIENT_PORT=2181
    restart: unless-stopped

volumes:
  yanying-data:
  minio-data:
```

启动：

```bash
docker-compose up -d
```

---

## Kubernetes部署

### 准备Helm Chart

创建 `yanying-chart/values.yaml`：

```yaml
replicaCount: 3

image:
  repository: easydarwin/yanying
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 10086

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: yanying.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: yanying-tls
      hosts:
        - yanying.example.com

resources:
  limits:
    cpu: 4000m
    memory: 8Gi
  requests:
    cpu: 1000m
    memory: 2Gi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 100Gi

config:
  frame_extractor:
    enable: true
    store: minio
  ai_analysis:
    enable: true
    scan_interval_sec: 5
  minio:
    endpoint: minio-service:9000
    access_key: minioadmin
    secret_key: minioadmin
    bucket: snapshots
  kafka:
    address: kafka-service:9092
    topic: easydarwin.alerts
```

部署：

```bash
# 安装Helm
curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash

# 部署yanying
helm install yanying ./yanying-chart

# 更新
helm upgrade yanying ./yanying-chart

# 查看状态
kubectl get pods -l app=yanying
kubectl logs -f deployment/yanying
```

---

## 性能优化

### 1. 系统级优化

```bash
# 增加文件描述符限制
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# 网络参数优化
cat >> /etc/sysctl.conf <<EOF
net.core.somaxconn = 32768
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.tcp_tw_reuse = 1
net.ipv4.ip_local_port_range = 1024 65535
EOF
sysctl -p
```

### 2. yanying配置优化

```toml
[server]
# 增加工作协程数
max_workers = 1000

# 启用连接池
enable_conn_pool = true
max_conn_per_host = 100

[frame_extractor]
# 批量处理
batch_size = 10

[ai_analysis]
# 增加并发数
max_concurrent_infer = 50

# 启用结果缓存
enable_cache = true
cache_ttl_sec = 300
```

### 3. 数据库优化

```bash
# SQLite优化
sqlite3 /var/lib/yanying/data.db <<EOF
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = 10000;
PRAGMA temp_store = MEMORY;
EOF
```

---

## 监控告警

### 1. 部署Prometheus

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'yanying'
    static_configs:
      - targets: ['yanying-1:10086', 'yanying-2:10086', 'yanying-3:10086']
    metrics_path: '/metrics'
```

### 2. 部署Grafana

```bash
docker run -d \
  -p 3000:3000 \
  -e GF_SECURITY_ADMIN_PASSWORD=admin \
  -v grafana-data:/var/lib/grafana \
  grafana/grafana
```

### 3. 配置告警

创建告警规则：

```yaml
# alerts.yml
groups:
  - name: yanying
    rules:
      - alert: YanyingDown
        expr: up{job="yanying"} == 0
        for: 1m
        annotations:
          summary: "yanying实例 {{ $labels.instance }} 已下线"
          
      - alert: HighCPU
        expr: process_cpu_seconds_total{job="yanying"} > 0.8
        for: 5m
        annotations:
          summary: "yanying实例 {{ $labels.instance }} CPU使用率过高"
          
      - alert: HighMemory
        expr: process_resident_memory_bytes{job="yanying"} > 8e9
        for: 5m
        annotations:
          summary: "yanying实例 {{ $labels.instance }} 内存使用过高"
```

---

## 常见问题

### Q1: 启动失败，提示端口被占用

**解决方案**：

```bash
# 查找占用端口的进程
sudo lsof -i :10086
sudo netstat -tulnp | grep 10086

# 修改配置文件中的端口
vim config.toml
# [server]
# http_port = 10087  # 改为其他端口
```

### Q2: MinIO连接失败

**解决方案**：

```bash
# 检查MinIO服务状态
docker ps | grep minio
curl http://localhost:9000

# 检查网络连接
ping minio-server
telnet minio-server 9000

# 检查配置文件
vim config.toml
# [frame_extractor.minio]
# endpoint = 'localhost:9000'  # 确保地址正确
```

### Q3: 算法服务注册失败

**解决方案**：

```bash
# 检查网络连通性
curl http://yanying-server:10086/api/v1/health

# 检查算法服务日志
docker logs algorithm-service

# 手动测试注册
curl -X POST http://yanying-server:10086/api/v1/ai_analysis/register \
  -H 'Content-Type: application/json' \
  -d '{
    "service_id": "test",
    "name": "测试服务",
    "task_types": ["人数统计"],
    "endpoint": "http://localhost:8000/infer",
    "version": "1.0.0"
  }'
```

### Q4: 推理调度缓慢

**解决方案**：

1. 增加并发数：

```toml
[ai_analysis]
max_concurrent_infer = 20  # 增大并发数
```

2. 优化算法服务：

```python
# 使用批量推理
@app.route('/infer_batch', methods=['POST'])
def infer_batch():
    images = request.json['images']
    # 批量处理，提高吞吐量
    results = model.predict_batch(images)
    return jsonify(results)
```

3. 增加算法服务实例：

```bash
# 启动多个实例
docker-compose up -d --scale algorithm-service=5
```

---

## 备份与恢复

### 数据备份

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/yanying/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# 备份数据库
cp /var/lib/yanying/data.db $BACKUP_DIR/

# 备份配置文件
cp /etc/yanying/config.toml $BACKUP_DIR/

# 备份MinIO数据（使用mc工具）
mc mirror minio/snapshots $BACKUP_DIR/snapshots/

# 打包压缩
cd /backup/yanying
tar -czf yanying-backup-$(date +%Y%m%d).tar.gz $(date +%Y%m%d)/

# 上传到远程存储
# aws s3 cp yanying-backup-$(date +%Y%m%d).tar.gz s3://backup-bucket/
```

### 数据恢复

```bash
#!/bin/bash
# restore.sh

BACKUP_FILE=$1

# 解压备份
tar -xzf $BACKUP_FILE -C /tmp/

# 停止服务
systemctl stop yanying

# 恢复数据库
cp /tmp/*/data.db /var/lib/yanying/

# 恢复配置
cp /tmp/*/config.toml /etc/yanying/

# 恢复MinIO数据
mc mirror /tmp/*/snapshots/ minio/snapshots/

# 启动服务
systemctl start yanying
```

---

<div align="center">

**生产环境部署就绪 ✓**

[⬆ 返回顶部](#yanying-部署指南)

</div>

