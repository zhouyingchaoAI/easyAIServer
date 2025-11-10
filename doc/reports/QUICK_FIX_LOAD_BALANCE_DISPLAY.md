# 负载均衡监控界面问题修复

**问题**: 没有看到负载均衡监控界面  
**原因**: 前端文件没有正确部署到运行目录  
**状态**: ✅ 已修复

---

## 🔍 问题排查

### 发现的问题

1. ✅ API可用（http://localhost:5066/api/v1/ai_analysis/load_balance/info）
2. ✅ 后端编译完成（easydarwin_fixed）
3. ✅ 前端编译完成（web-src/web/）
4. ❌ 前端文件没有部署到运行目录

### 原因分析

```
编译后的文件位置：
  ./web-src/web/assets/js/monitor-DH814K7q.1762225789000.js ✅ (新)

运行目录应该是：
  ./build/EasyDarwin-aarch64-v8.3.3-202511040311/web/ ❌ (缺失)

→ 文件没有复制到运行目录
→ 浏览器加载的是旧版本
→ 看不到负载均衡监控界面
```

---

## ✅ 解决方案

### 已执行的操作

```bash
# 复制最新编译的前端文件到运行目录
cd /code/EasyDarwin
rm -rf ./build/EasyDarwin-aarch64-v8.3.3-202511040311/web
cp -r ./web-src/web ./build/EasyDarwin-aarch64-v8.3.3-202511040311/
```

### 验证步骤

```bash
# 1. 检查文件是否存在
ls ./build/EasyDarwin-aarch64-v8.3.3-202511040311/web/assets/js/monitor*.js

# 2. 验证包含负载均衡功能
grep "loadBalanceInfo" ./build/EasyDarwin-aarch64-v8.3.3-202511040311/web/assets/js/monitor*.js
```

---

## 🚀 现在可以查看

### 1. 刷新浏览器

```
访问：http://localhost:5066/frame-extractor/monitor
按 Ctrl + F5 强制刷新（清除缓存）
```

### 2. 应该看到

```
┌──────────────────────────────────────────────────────────┐
│ 📊 抽帧服务监控                                          │
├──────────────────────────────────────────────────────────┤
│                                                          │
│ [总任务数] [运行中] [已停止] [待配置]                   │
│                                                          │
│ ┌────────────────────────────────────────────────────┐  │
│ │ 🎯 负载均衡策略          [加权轮询（WRR）]         │  │
│ ├────────────────────────────────────────────────────┤  │
│ │ 📌 人数统计 | 12个服务 | 总权重: 63                │  │
│ ├────────────────────────────────────────────────────┤  │
│ │ 端点        │ 平均响应 │ 权重 │ 分配比例 │ 调用  │  │
│ │ :7912/infer │ ✅138ms  │  7   │ ████ 11% │  51   │  │
│ │ :7904/infer │ ✅156ms  │  6   │ ███  10% │  41   │  │
│ │ :7901/infer │ 🔵161ms  │  6   │ ███  10% │  94   │  │
│ │ :7909/infer │ 🔵177ms  │  5   │ ██    8% │  53   │  │
│ │ :7910/infer │ 🔵179ms  │  5   │ ██    8% │  55   │  │
│ │ ...更多服务                                        │  │
│ └────────────────────────────────────────────────────┘  │
│   ℹ️ 权重计算：weight = 1000 / 响应时间(ms)            │
│      最小权重1，最大权重100                            │
│                                                          │
│ ┌────────────────────────────────────────────────────┐  │
│ │ AI推理队列                                          │  │
│ │ ...                                                 │  │
│ └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### 3. 如果还是看不到

可能原因和解决方案：

#### 原因1：浏览器缓存
```
解决：
  Ctrl + Shift + Delete → 清除缓存
  或 Ctrl + F5 强制刷新
```

#### 原因2：没有算法服务在线
```
检查：
  curl http://localhost:5066/api/v1/ai_analysis/services
  
如果services为空：
  → 没有算法服务注册
  → 负载均衡卡片不会显示（v-if条件）
```

#### 原因3：程序没有重启
```
解决：
  pkill easydarwin
  ./easydarwin
```

---

## 📋 完整检查清单

### 前端文件检查
```bash
# 1. 确认编译输出
ls -lh ./web-src/web/index.html
# 应该显示最新时间（03:10或更晚）

# 2. 确认运行目录文件
ls -lh ./build/EasyDarwin-aarch64-v8.3.3-202511040311/web/index.html
# 应该显示相同时间

# 3. 检查monitor.js
ls -lh ./build/EasyDarwin-aarch64-v8.3.3-202511040311/web/assets/js/monitor*.js
# 应该存在

# 4. 验证包含新功能
grep "loadBalanceInfo" ./build/EasyDarwin-aarch64-v8.3.3-202511040311/web/assets/js/monitor*.js
# 应该有多个匹配
```

### API检查
```bash
# 测试负载均衡API
curl -s http://localhost:5066/api/v1/ai_analysis/load_balance/info | jq

# 应该返回：
{
  "load_balance": {
    "人数统计": {
      "total_services": 12,
      "services": [...],
      "total_weight": 63
    }
  },
  "total_task_types": 3
}
```

### 浏览器检查
```
1. 访问 http://localhost:5066/frame-extractor/monitor
2. 按 F12 打开开发者工具
3. 查看 Console 是否有错误
4. 查看 Network 标签，确认加载的JS文件时间是否最新
5. 强制刷新：Ctrl + F5
```

---

## ✅ 当前状态

- ✅ API可用（返回正确数据）
- ✅ 后端编译完成
- ✅ 前端编译完成
- ✅ 文件已复制到运行目录
- ⏳ 需要刷新浏览器

---

**下一步**：
1. 访问 `http://localhost:5066/frame-extractor/monitor`
2. 按 `Ctrl + F5` 强制刷新浏览器
3. 应该能看到"负载均衡策略"卡片

如果还是看不到，请告诉我浏览器Console中的错误信息。

