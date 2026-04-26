<template>
  <div class="welcome-view" id="welcomeView">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
      <polyline points="14,2 14,8 20,8"/>
      <line x1="16" y1="13" x2="8" y2="13"/>
      <line x1="16" y1="17" x2="8" y2="17"/>
      <polyline points="10,9 9,9 8,9"/>
    </svg>
    <h2>选择文件开始对话</h2>
    <p>打开文件后与 AI 对话，让 AI 直接帮你阅读、编辑和修改文件。</p>
  </div>
</template>

<style scoped>
.welcome-view {
    display: flex;
    flex: 1;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    min-height: 0;
    padding: 0 32px;
    text-align: center;
    color: var(--text-muted);
    border-radius: 0;
    background: transparent;
    box-sizing: border-box;
}

.welcome-view svg {
    width: 60px;
    height: 60px;
    margin-bottom: 20px;
    opacity: 0.4;
}

.welcome-view h2 {
    font-size: 20px;
    font-weight: 500;
    color: var(--text-secondary);
    margin-bottom: 8px;
}

.welcome-view p {
    font-size: 15px;
    max-width: 400px;
    margin-top: 8px;
}
</style>
