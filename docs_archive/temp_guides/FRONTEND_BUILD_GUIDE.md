# 前端构建指南

## 📦 构建前端（算法配置界面）

### 前提条件

- Node.js >= 18.19.0
- npm 或 yarn

---

## 🚀 快速构建

### 1. 安装依赖

```bash
cd /code/EasyDarwin/web-src
npm install
```

**新增依赖**:
- `fabric: ^5.3.0` - Canvas绘图库

### 2. 开发模式（可选）

```bash
npm run dev
```

访问：`http://localhost:5173`

### 3. 生产构建

```bash
npm run build
```

构建产物输出到：`../web/`

### 4. 重启服务

```bash
cd ..
./easydarwin
```

---

## 📂 新增文件

### 组件

- `web-src/src/components/AlgoConfigModal/index.vue` - 算法配置弹窗（500+行）

### API

- `web-src/src/api/frameextractor.js` - 新增4个API方法

### 页面

- `web-src/src/views/frame-extractor/index.vue` - 更新任务列表

---

## 🎨 算法配置组件功能

### 左侧：绘图区域

- ✅ 加载预览图片
- ✅ Fabric.js画布
- ✅ 绘制工具栏（线/矩形/多边形）
- ✅ 删除/清空/重置功能

### 右侧：配置面板

- ✅ 区域列表（折叠面板）
- ✅ 区域属性编辑（名称、颜色、透明度、阈值）
- ✅ 算法参数配置
- ✅ 保存按钮

---

## 🔧 技术栈

- **Vue 3** - Composition API
- **Ant Design Vue** - UI组件库
- **Fabric.js** - Canvas绘图
- **Axios** - HTTP请求

---

## 📋 构建后验证

### 1. 检查文件

```bash
ls -lh web/assets/*.js web/assets/*.css
# 应该看到新的bundle文件
```

### 2. 访问界面

访问：`http://localhost:5066`

进入【抽帧管理】页面，应该看到：
- 任务列表多了"配置状态"列
- 操作栏多了"算法配置"按钮（齿轮图标）

### 3. 测试功能

1. 添加一个任务
2. 等待预览图生成
3. 点击"算法配置"
4. 验证绘图功能

---

## 🐛 常见问题

### Q1: npm install失败？

**解决**:
```bash
# 清理缓存
npm cache clean --force

# 使用国内镜像
npm install --registry=https://registry.npmmirror.com
```

### Q2: 构建失败？

**解决**:
```bash
# 检查Node版本
node --version  # 应该 >= 18.19.0

# 删除node_modules重新安装
rm -rf node_modules package-lock.json
npm install
```

### Q3: 组件加载失败？

**检查**:
1. 确认 `fabric` 包已安装：`npm list fabric`
2. 查看浏览器控制台是否有错误
3. 检查组件路径是否正确

---

## 📈 开发模式调试

### 1. 启动开发服务器

```bash
cd web-src
npm run dev
```

### 2. 配置代理（如需要）

编辑 `web-src/vite.config.js`:
```javascript
export default {
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:5066',
        changeOrigin: true
      }
    }
  }
}
```

### 3. 热重载

修改代码后会自动刷新，无需重启

---

## 🎯 下一步

构建完成后：

1. 启动EasyDarwin服务
2. 添加抽帧任务
3. 配置算法参数
4. 启动算法服务
5. 验证完整流程

---

**构建愉快！** 🚀

