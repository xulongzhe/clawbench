<template>
  <div v-if="showPrompt" class="pwa-install-prompt">
    <div class="prompt-content">
      <div class="prompt-icon">
        <img src="/assets/logo-180.png" alt="ClawBench" />
      </div>
      <div class="prompt-text">
        <h3>安装 ClawBench</h3>
        <p>添加到主屏幕，享受更好的体验</p>
      </div>
      <div class="prompt-actions">
        <button class="btn-install" @click="handleInstall">安装</button>
        <button class="btn-dismiss" @click="dismissPrompt">稍后</button>
      </div>
    </div>
  </div>

  <!-- iOS 安装提示 -->
  <div v-if="showIOSPrompt" class="ios-install-guide">
    <div class="guide-content">
      <h3>安装到主屏幕</h3>
      <ol>
        <li>点击底部的<span class="icon-share">↑</span>分享按钮</li>
        <li>向下滚动，点击"添加到主屏幕"</li>
        <li>点击右上角"添加"</li>
      </ol>
      <button @click="closeIOSGuide">知道了</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { pwaInstallManager } from '@/utils/pwa-install'

const showPrompt = ref(false)
const showIOSPrompt = ref(false)

onMounted(() => {
  // 检查是否是 iOS 设备
  const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent)
  const isSafari = /Safari/.test(navigator.userAgent) && !/CriOS|FxiOS/.test(navigator.userAgent)

  if (isIOS && isSafari && !pwaInstallManager.isAppInstalled()) {
    // iOS Safari 显示手动安装指南
    showIOSPrompt.value = true
  } else {
    // 其他浏览器监听安装提示
    window.addEventListener('pwa-install-available', (e: CustomEvent) => {
      showPrompt.value = e.detail.available
    })

    // 初始检查
    showPrompt.value = pwaInstallManager.canInstall()
  }
})

async function handleInstall() {
  const accepted = await pwaInstallManager.promptInstall()
  if (accepted) {
    showPrompt.value = false
  }
}

function dismissPrompt() {
  showPrompt.value = false
}

function closeIOSGuide() {
  showIOSPrompt.value = false
}
</script>

<style scoped>
.pwa-install-prompt,
.ios-install-guide {
  position: fixed;
  bottom: 20px;
  left: 20px;
  right: 20px;
  max-width: 400px;
  margin: 0 auto;
  background: var(--color-bg-secondary);
  border-radius: 12px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
  z-index: 1000;
  padding: 16px;
}

.prompt-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.prompt-icon img {
  width: 48px;
  height: 48px;
  border-radius: 12px;
}

.prompt-text {
  flex: 1;
}

.prompt-text h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
}

.prompt-text p {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.prompt-actions {
  display: flex;
  gap: 8px;
}

button {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
}

.btn-install {
  background: var(--color-primary);
  color: white;
}

.btn-dismiss {
  background: transparent;
  color: var(--color-text-secondary);
}

.ios-install-guide h3 {
  margin: 0 0 12px 0;
  font-size: 16px;
}

.ios-install-guide ol {
  margin: 0 0 16px 0;
  padding-left: 20px;
}

.ios-install-guide li {
  margin-bottom: 8px;
  font-size: 14px;
}

.icon-share {
  display: inline-block;
  width: 20px;
  height: 20px;
  background: var(--color-primary);
  color: white;
  border-radius: 4px;
  text-align: center;
  line-height: 20px;
  font-size: 12px;
}
</style>
