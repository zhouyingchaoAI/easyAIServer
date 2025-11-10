# 📚 EasyDarwin 文档索引

> **文档已整理**：根目录从51个文档精简到6个核心文档，技术文档统一放在doc/目录。

---

## 🚀 快速开始

| 文档 | 说明 |
|------|------|
| [README_CN.md](README_CN.md) | **项目介绍**（中文）⭐ 推荐首先阅读 |
| [QUICK_REFERENCE.md](QUICK_REFERENCE.md) | **快速参考卡片** - 常用操作速查 |
| [README_前端修复完成.md](README_前端修复完成.md) | **最新UI修复说明** - 前端问题修复 |

---

## 📖 核心文档

### 用户文档（根目录）

```
EasyDarwin/
├── 📘 README_CN.md              # 中文项目说明⭐
├── 📘 README.md                 # 英文项目说明
├── 🔧 UPGRADE_GUIDE.md          # 升级指南
├── ⚡ QUICK_REFERENCE.md        # 快速参考
├── 🆕 README_前端修复完成.md     # 最新修复
└── 📄 LICENSE.txt               # 开源许可
```

### 技术文档（doc/目录）

```
doc/
├── 📖 README.md                 # 技术文档索引⭐
│
├── 🤖 AI智能分析（6个）
│   ├── AI_ANALYSIS.md
│   ├── AI_ANALYSIS_QUICKSTART.md
│   ├── AI_INFERENCE_AUTO_DELETE.md
│   ├── SMART_INFERENCE_STRATEGY.md
│   ├── SMART_INFERENCE_USAGE.md
│   └── OPTIMIZATION_STRATEGY.md
│
├── 📹 抽帧功能（3个）
│   ├── FRAME_EXTRACTOR.md
│   ├── FRAME_EXTRACTOR_MONITOR.md
│   └── TROUBLESHOOTING_FRAME_EXTRACTOR.md
│
├── 🔌 算法对接（5个）
│   ├── ALGORITHM_INTEGRATION_GUIDE.md
│   ├── ALGORITHM_RESPONSE_FORMAT.md
│   ├── ALGORITHM_CONFIG_SPEC.md
│   └── ...
│
├── ⚙️ 功能详解（10个）
│   ├── FEATURE_SAVE_ONLY_WITH_DETECTION.md
│   ├── LINE_DIRECTION_FEATURE.md
│   ├── TRIPWIRE_COUNTING_ALGORITHM.md
│   └── ...
│
├── 🛠️ 配置管理（3个）
│   ├── CONFIG_MIGRATION_GUIDE.md
│   ├── DATABASE_MIGRATION.md
│   └── DEPLOYMENT_GUIDE_CN.md
│
└── 📡 API文档（2个）
    ├── EasyDarwin.api.html
    └── EasyDarwin.apifox.json
```

---

## 🎯 按角色阅读

### 👤 普通用户
1. [README_CN.md](README_CN.md) - 了解项目
2. [QUICK_REFERENCE.md](QUICK_REFERENCE.md) - 快速操作
3. [doc/AI_ANALYSIS_QUICKSTART.md](doc/AI_ANALYSIS_QUICKSTART.md) - AI功能

### 👨‍💻 算法开发者
1. [doc/ALGORITHM_INTEGRATION_GUIDE.md](doc/ALGORITHM_INTEGRATION_GUIDE.md) - 对接指南
2. [doc/ALGORITHM_RESPONSE_FORMAT.md](doc/ALGORITHM_RESPONSE_FORMAT.md) - 返回格式
3. [doc/ALGORITHM_CONFIG_SPEC.md](doc/ALGORITHM_CONFIG_SPEC.md) - 配置规范

### 🔧 运维人员
1. [doc/DEPLOYMENT_GUIDE_CN.md](doc/DEPLOYMENT_GUIDE_CN.md) - 部署指南
2. [UPGRADE_GUIDE.md](UPGRADE_GUIDE.md) - 升级指南
3. [doc/TROUBLESHOOTING_FRAME_EXTRACTOR.md](doc/TROUBLESHOOTING_FRAME_EXTRACTOR.md) - 故障排查

### 🎨 前端开发者
1. [README_前端修复完成.md](README_前端修复完成.md) - UI修复
2. [doc/CANVAS_LOADING_FIX.md](doc/CANVAS_LOADING_FIX.md) - Canvas问题
3. 查看 `web-src/` 目录

---

## 📂 特殊目录说明

### docs_archive/ - 归档文档

**包含内容**：
- 开发过程文档（CHANGELOG、SUMMARY、IMPLEMENTATION）
- 临时快速指南（重复的QUICK_GUIDE）
- 历史版本文档
- 临时说明文件

**用途**：
- 查找历史功能开发记录
- 了解功能演进过程
- 参考旧版本文档

**如何查找**：
```bash
# 搜索归档文档
grep -r "关键词" docs_archive/

# 查看开发过程
ls docs_archive/process_docs/

# 查看旧指南
ls docs_archive/temp_guides/
```

---

## 🔍 常用文档快速链接

### 功能使用

- **AI智能分析**：[doc/AI_ANALYSIS.md](doc/AI_ANALYSIS.md)
- **抽帧监控**：[doc/FRAME_EXTRACTOR.md](doc/FRAME_EXTRACTOR.md)
- **算法对接**：[doc/ALGORITHM_INTEGRATION_GUIDE.md](doc/ALGORITHM_INTEGRATION_GUIDE.md)
- **绊线统计**：[doc/TRIPWIRE_COUNTING_ALGORITHM.md](doc/TRIPWIRE_COUNTING_ALGORITHM.md)

### 配置管理

- **配置迁移**：[doc/CONFIG_MIGRATION_GUIDE.md](doc/CONFIG_MIGRATION_GUIDE.md)
- **数据库迁移**：[doc/DATABASE_MIGRATION.md](doc/DATABASE_MIGRATION.md)
- **部署指南**：[doc/DEPLOYMENT_GUIDE_CN.md](doc/DEPLOYMENT_GUIDE_CN.md)

### 问题排查

- **抽帧故障**：[doc/TROUBLESHOOTING_FRAME_EXTRACTOR.md](doc/TROUBLESHOOTING_FRAME_EXTRACTOR.md)
- **快速参考**：[QUICK_REFERENCE.md](QUICK_REFERENCE.md)
- **Canvas问题**：[doc/CANVAS_LOADING_FIX.md](doc/CANVAS_LOADING_FIX.md)

---

## 📝 文档更新记录

### 最近更新（2025-10-22）

- ✅ 整理根目录文档：51个 → 6个
- ✅ 扩充doc/目录：25个 → 36个
- ✅ 创建归档目录：49个历史文档
- ✅ 创建文档索引：doc/README.md
- ✅ 创建本导航文档

---

## 🎉 整理效果

**整理前**：
- ❌ 根目录51个文档，查找困难
- ❌ 文档重复多，信息混乱
- ❌ 缺少分类和索引

**整理后**：
- ✅ 根目录6个核心文档，清晰明了
- ✅ doc/目录36个技术文档，分类清晰
- ✅ 完整的文档索引和导航
- ✅ 历史文档归档保留

---

**索引版本**：v2.1  
**更新日期**：2025-10-22  
**维护状态**：✅ 已完成

