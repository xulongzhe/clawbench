# ClawBench PWA 适配说明

## 概述

ClawBench 现已支持 PWA（渐进式网络应用），用户可以将其安装为独立应用，享受原生应用般的体验。

## 已完成的配置

### 1. Manifest 文件
- **位置**: `public/manifest.json` 和 `web/manifest.json`
- **关键配置**:
  - `display: "standalone"` - 安装后隐藏地址栏
  - 包含多种尺寸的图标（96x96, 180x180, 512x512, 1024x1024）
  - 设置了主题色和背景色
  - 支持快捷方式功能

### 2. HTML 元数据
在 `web/index.html` 和 `public/index.html` 中添加了：
- PWA manifest 链接
- Apple 移动端适配标签
- 不同尺寸的图标链接
- 主题色配置

### 3. Service Worker
- **位置**: `public/sw.js`
- **功能**:
  - 静态资源缓存
  - 离线支持
  - 后台更新

### 4. PWA 安装管理器
- **位置**: `web/src/utils/pwa-install.ts`
- **功能**:
  - 检测安装状态
  - 处理安装提示
  - iOS 安装指南

## 安装方式

### 桌面端 Chrome/Edge (117+)

1. **自动提示**: 当网站满足 PWA 条件后，地址栏右侧会出现安装图标（⊕）
2. **手动安装**:
   - 点击浏览器菜单
   - 选择"保存和分享" → "将页面安装为应用"
   - 或点击地址栏的安装图标

### 移动端 Chrome

1. 访问网站
2. 点击浏览器菜单（三个点）
3. 选择"添加到主屏幕"或"安装应用"
4. 确认安装

### iOS Safari

1. 访问网站
2. 点击底部分享按钮（方框向上箭头）
3. 向下滚动，点击"添加到主屏幕"
4. 点击右上角"添加"

## 开发使用

### 在 Vue 组件中使用安装管理器

```typescript
import { pwaInstallManager } from '@/utils/pwa-install'

// 检查是否可安装
if (pwaInstallManager.canInstall()) {
  // 显示安装按钮
}

// 触发安装
async function handleInstall() {
  const accepted = await pwaInstallManager.promptInstall()
  if (accepted) {
    console.log('用户接受了安装')
  }
}

// 监听安装可用事件
window.addEventListener('pwa-install-available', (e) => {
  console.log('PWA 安装可用:', e.detail.available)
})
```

### iOS 安装提示示例

```vue
<template>
  <div v-if="showIOSHint" class="ios-install-hint">
    <h3>安装到主屏幕</h3>
    <ol>
      <li v-for="step in installSteps" :key="step">{{ step }}</li>
    </ol>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { pwaInstallManager } from '@/utils/pwa-install'

const showIOSHint = ref(false)
const installSteps = ref<string[]>([])

onMounted(() => {
  // iOS 设备检测
  const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent)
  if (isIOS && !pwaInstallManager.isAppInstalled()) {
    showIOSHint.value = true
    installSteps.value = pwaInstallManager.getIOSInstallInstructions()
  }
})
</script>
```

## PWA 要求清单

✅ 已满足的条件：
- [x] HTTPS 或 localhost（生产环境需 HTTPS）
- [x] manifest.json 配置正确
- [x] 包含合适的图标
- [x] Service Worker 注册
- [x] 响应式设计（移动端友好）
- [x] 设置了主题色和背景色

## 调试工具

### Chrome DevTools

1. 打开开发者工具 (F12)
2. 进入 "Application" 标签
3. 左侧菜单：
   - **Manifest**: 查看清单解析
   - **Service Workers**: 管理 Service Worker
   - **Storage**: 查看缓存内容

### Lighthouse 审计

1. 打开开发者工具
2. 点击 "Lighthouse" 标签
3. 勾选 "Progressive Web App"
4. 运行审计

## 更新机制

Service Worker 会在以下情况更新：
- 检测到 `sw.js` 文件变化
- 用户重新加载页面
- 安装新版本后激活

## 常见问题

### Q: 为什么 Chrome 没有显示安装按钮？
A: 检查以下几点：
1. 确保网站使用 HTTPS（或 localhost）
2. manifest.json 正确链接且格式正确
3. 至少有一个图标尺寸 ≥ 192x192
4. start_url 有效

### Q: iOS 上如何隐藏地址栏？
A: iOS 不支持 manifest 的 `display: standalone`，需要用户手动添加到主屏幕才能实现全屏体验。

### Q: 如何测试 Service Worker？
A:
1. 使用 Chrome DevTools → Application → Service Workers
2. 勾选 "Update on reload" 进行测试
3. 使用 "Bypass for network" 跳过缓存

## 下一步建议

1. **添加安装提示**: 在界面中添加友好的安装提示组件
2. **离线页面**: 创建自定义离线错误页面
3. **推送通知**: 如需要，可添加 Web Push 功能
4. **定期更新**: 定期更新 Service Worker 缓存策略

## 相关文件

```
clawbench/
├── public/
│   ├── manifest.json        # PWA 清单
│   ├── sw.js                # Service Worker
│   ├── index.html           # 包含 PWA 元数据
│   └── assets/              # 图标资源
├── web/
│   ├── manifest.json        # 开发环境清单
│   ├── index.html           # 包含 PWA 元数据
│   └── src/utils/
│       └── pwa-install.ts   # 安装管理器
└── docs/
    └── PWA_SETUP.md         # 本文档
```
