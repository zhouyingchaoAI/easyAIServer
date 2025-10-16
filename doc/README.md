# yanying 文档中心

欢迎来到yanying视频智能分析平台的文档中心！

---

## 📚 快速导航

### 🚀 新手入门

| 文档 | 描述 | 适合人群 |
|------|------|---------|
| [中文README](../README_CN.md) | 项目概览、快速开始 | 所有用户 |
| [快速开始指南](../QUICKSTART.md) | 5分钟上手教程 | 新手用户 |
| [项目介绍](PROJECT_INTRO_CN.md) | 深入了解项目愿景和技术创新 | 决策者、架构师 |

### 🤖 智能分析

| 文档 | 描述 | 适合人群 |
|------|------|---------|
| [AI分析插件](AI_ANALYSIS.md) | AI分析功能完整文档 | 开发者、集成商 |
| [任务类型系统](TASK_TYPES.md) | 20+预设任务类型说明 | 算法开发者 |
| [任务类型示例](TASK_TYPE_EXAMPLES.md) | 各场景配置示例 | 系统集成商 |

### 🎬 抽帧功能

| 文档 | 描述 | 适合人群 |
|------|------|---------|
| [抽帧插件文档](FRAME_EXTRACTOR.md) | 抽帧功能完整指南 | 开发者 |
| [问题排查指南](TROUBLESHOOTING_FRAME_EXTRACTOR.md) | 常见问题解决方案 | 运维人员 |

### 🔧 部署运维

| 文档 | 描述 | 适合人群 |
|------|------|---------|
| [部署指南](DEPLOYMENT_GUIDE_CN.md) | 从开发到生产的完整部署方案 | 运维工程师 |
| [E2E测试指南](E2E_TEST.md) | 端到端测试流程 | 测试人员 |

### 🔗 其他文档

| 文档 | 描述 | 适合人群 |
|------|------|---------|
| [直播流地址说明](LIVE_STREAM_URL.md) | 各种协议的流地址格式 | 开发者 |

---

## 🎯 按角色查找

### 👨‍💼 决策者 / 项目经理

**推荐阅读顺序**：
1. [项目介绍](PROJECT_INTRO_CN.md) - 了解项目价值
2. [中文README](../README_CN.md) - 核心功能概览
3. [部署指南](DEPLOYMENT_GUIDE_CN.md) - 了解部署成本

**关注重点**：
- ✅ 项目能解决什么问题
- ✅ 技术架构和扩展性
- ✅ 部署成本和维护成本
- ✅ 应用场景和案例

---

### 👨‍💻 算法开发者

**推荐阅读顺序**：
1. [任务类型系统](TASK_TYPES.md) - 了解预设场景
2. [AI分析插件](AI_ANALYSIS.md) - API接口规范
3. [任务类型示例](TASK_TYPE_EXAMPLES.md) - 参考示例

**关注重点**：
- ✅ 如何开发算法服务
- ✅ HTTP推理接口规范
- ✅ 服务注册与心跳机制
- ✅ 结果格式要求

**开发示例**：
```python
# examples/algorithm_service.py
python algorithm_service.py \
  --service-id my_algorithm \
  --task-types 人数统计 \
  --port 8000
```

---

### 🏗️ 系统集成商

**推荐阅读顺序**：
1. [中文README](../README_CN.md) - 功能全览
2. [快速开始](../QUICKSTART.md) - 快速部署测试
3. [部署指南](DEPLOYMENT_GUIDE_CN.md) - 生产环境部署
4. [AI分析插件](AI_ANALYSIS.md) - 集成算法服务

**关注重点**：
- ✅ 如何为客户快速搭建系统
- ✅ 如何集成现有算法
- ✅ 如何定制化配置
- ✅ 性能优化和监控

---

### 🔧 运维工程师

**推荐阅读顺序**：
1. [部署指南](DEPLOYMENT_GUIDE_CN.md) - 部署方案
2. [问题排查](TROUBLESHOOTING_FRAME_EXTRACTOR.md) - 常见问题
3. [E2E测试](E2E_TEST.md) - 测试验证

**关注重点**：
- ✅ 系统要求和资源规划
- ✅ 高可用部署架构
- ✅ 监控告警配置
- ✅ 备份恢复方案
- ✅ 性能优化

**运维工具**：
```bash
# 查看服务状态
systemctl status yanying

# 查看日志
journalctl -u yanying -f

# 健康检查
curl http://localhost:10086/api/v1/health
```

---

### 🧪 测试人员

**推荐阅读顺序**：
1. [E2E测试指南](E2E_TEST.md) - 完整测试流程
2. [问题排查](TROUBLESHOOTING_FRAME_EXTRACTOR.md) - 问题诊断

**关注重点**：
- ✅ 功能测试用例
- ✅ 性能测试指标
- ✅ 异常场景测试
- ✅ 日志分析

---

## 🎓 学习路径

### 入门级（第1周）

**目标**：能够运行起来，理解基本概念

**学习计划**：
1. **Day 1-2**：阅读[中文README](../README_CN.md)，了解项目整体架构
2. **Day 3-4**：跟随[快速开始](../QUICKSTART.md)完成本地部署
3. **Day 5-6**：创建第一个抽帧任务，查看[抽帧文档](FRAME_EXTRACTOR.md)
4. **Day 7**：运行示例算法服务，查看[AI分析文档](AI_ANALYSIS.md)

**实践项目**：
- ✅ 本地部署yanying
- ✅ 添加1个RTSP摄像头
- ✅ 创建1个抽帧任务
- ✅ 运行1个示例算法服务
- ✅ 查看告警结果

---

### 进阶级（第2-3周）

**目标**：能够开发算法服务，配置复杂场景

**学习计划**：
1. **Week 2**：
   - 深入学习[任务类型系统](TASK_TYPES.md)
   - 开发自己的算法服务（参考[AI分析文档](AI_ANALYSIS.md)）
   - 配置MinIO和Kafka
2. **Week 3**：
   - 学习多算法协同工作
   - 配置复杂的业务场景
   - 性能调优

**实践项目**：
- ✅ 开发3个不同场景的算法服务
- ✅ 配置5路以上摄像头
- ✅ 实现多算法协同分析
- ✅ 配置Kafka消息推送

---

### 高级级（第4周+）

**目标**：生产环境部署，性能优化，故障排查

**学习计划**：
1. 深入学习[部署指南](DEPLOYMENT_GUIDE_CN.md)
2. 掌握高可用架构设计
3. 性能测试与优化
4. 监控告警配置
5. 故障排查与恢复

**实践项目**：
- ✅ 搭建生产环境（多节点）
- ✅ 配置Nginx负载均衡
- ✅ 搭建MinIO和Kafka集群
- ✅ 配置Prometheus监控
- ✅ 压力测试（100路+摄像头）

---

## 📖 文档贡献

### 发现文档问题？

- 📝 [提交Issue](https://github.com/EasyDarwin/EasyDarwin/issues)
- ✏️ 提交Pull Request改进文档
- 💬 在社区讨论

### 文档编写规范

1. **使用中文**：所有中文文档使用简体中文
2. **格式统一**：使用Markdown格式，遵循统一的排版风格
3. **代码示例**：提供可运行的完整代码示例
4. **图文并茂**：适当使用图表、流程图增强可读性
5. **保持更新**：代码更新时同步更新文档

---

## 🔍 文档搜索技巧

### 按关键词查找

| 关键词 | 相关文档 |
|--------|---------|
| 安装、部署 | [部署指南](DEPLOYMENT_GUIDE_CN.md) |
| 算法、AI | [AI分析](AI_ANALYSIS.md)、[任务类型](TASK_TYPES.md) |
| 抽帧、图片 | [抽帧插件](FRAME_EXTRACTOR.md) |
| 错误、问题 | [问题排查](TROUBLESHOOTING_FRAME_EXTRACTOR.md) |
| RTSP、流 | [直播流地址](LIVE_STREAM_URL.md) |

### 使用GitHub搜索

在项目页面按 `s` 键，然后输入关键词搜索所有文档。

---

## 📞 获取帮助

### 在线资源

- 📚 **文档中心**：你正在这里
- 💻 **示例代码**：`examples/` 目录
- 🐛 **Issue跟踪**：[GitHub Issues](https://github.com/EasyDarwin/EasyDarwin/issues)
- 💬 **社区讨论**：[GitHub Discussions](https://github.com/EasyDarwin/EasyDarwin/discussions)

### 常见问题快速解答

**Q: 从哪里开始？**
A: 先看[中文README](../README_CN.md)，然后跟随[快速开始](../QUICKSTART.md)

**Q: 如何开发算法服务？**
A: 查看[AI分析文档](AI_ANALYSIS.md)的API接口部分

**Q: 遇到错误怎么办？**
A: 先查看[问题排查指南](TROUBLESHOOTING_FRAME_EXTRACTOR.md)

**Q: 如何部署到生产环境？**
A: 参考[部署指南](DEPLOYMENT_GUIDE_CN.md)的生产环境部分

**Q: 性能不够怎么办？**
A: 查看[部署指南](DEPLOYMENT_GUIDE_CN.md)的性能优化部分

---

## 📅 文档更新日志

### 2024-10-16
- ✅ 新增完整的中文README
- ✅ 新增项目介绍文档
- ✅ 新增部署指南
- ✅ 新增文档中心导航

### 2024-10-15
- ✅ 新增E2E测试指南
- ✅ 更新AI分析文档

### 历史更新
- 查看Git提交历史了解更多更新

---

<div align="center">

**📖 持续完善中，感谢您的支持！**

如有疑问或建议，欢迎[提交Issue](https://github.com/EasyDarwin/EasyDarwin/issues)

[⬆ 返回顶部](#yanying-文档中心)

</div>

