<template>
  <Teleport to="body">
    <div v-if="show" class="metadata-modal-overlay" @click="$emit('close')">
      <div class="metadata-modal" @click.stop>
        <div class="metadata-modal-header">
          <h3>响应详情</h3>
          <button class="metadata-close-btn" @click="$emit('close')">×</button>
        </div>
        <div class="metadata-content">
          <div v-if="createdAt" class="metadata-item">
            <span class="metadata-label">时间:</span>
            <span class="metadata-value">{{ formatDetailTime(createdAt) }}</span>
          </div>
          <div v-if="filePath" class="metadata-item">
            <span class="metadata-label">关联文件:</span>
            <span class="metadata-value metadata-value-copyable" @click="copyValue(filePath, $event)">{{ filePath }}</span>
          </div>
          <div v-if="backend" class="metadata-item">
            <span class="metadata-label">后端:</span>
            <span class="metadata-value">{{ backend }}</span>
          </div>
          <div v-if="data.model" class="metadata-item">
            <span class="metadata-label">模型:</span>
            <span class="metadata-value">{{ data.model }}</span>
          </div>
          <div v-if="data.inputTokens" class="metadata-item">
            <span class="metadata-label">输入Token:</span>
            <span class="metadata-value">{{ data.inputTokens.toLocaleString() }}</span>
          </div>
          <div v-if="data.outputTokens" class="metadata-item">
            <span class="metadata-label">输出Token:</span>
            <span class="metadata-value">{{ data.outputTokens.toLocaleString() }}</span>
          </div>
          <div v-if="data.durationMs" class="metadata-item">
            <span class="metadata-label">耗时:</span>
            <span class="metadata-value">{{ (data.durationMs / 1000).toFixed(2) }}s</span>
          </div>
          <div v-if="data.costUsd" class="metadata-item">
            <span class="metadata-label">成本:</span>
            <span class="metadata-value">${{ data.costUsd.toFixed(6) }}</span>
          </div>
          <div v-if="data.sessionId" class="metadata-item metadata-copyable" @click="copyValue(data.sessionId, $event)">
            <span class="metadata-label">会话ID:</span>
            <div class="metadata-value-wrap">
              <span class="metadata-value metadata-session-id metadata-value-copyable">{{ data.sessionId }}</span>
              <button class="metadata-copy-btn" @click.stop="copyValue(data.sessionId, $event)" title="复制">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                </svg>
              </button>
            </div>
          </div>
          <div v-if="data.stopReason" class="metadata-item">
            <span class="metadata-label">停止原因:</span>
            <span class="metadata-value">{{ data.stopReason }}</span>
          </div>
          <div v-if="data.isError" class="metadata-item">
            <span class="metadata-label">错误:</span>
            <span class="metadata-value metadata-error">{{ data.errorMessage || '未知错误' }}</span>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { useToast } from '@/composables/useToast.ts'

const props = defineProps({
  show: Boolean,
  data: { type: Object, default: () => ({}) },
  backend: String,
  createdAt: String,
  filePath: String,
  formatDetailTime: Function,
})

const emit = defineEmits(['close'])

const toast = useToast()

function copyValue(value, event) {
  const wrap = event.currentTarget.closest('.metadata-value-wrap') || event.currentTarget
  const btn = wrap.querySelector?.('.metadata-copy-btn')
  const txt = wrap.querySelector?.('.metadata-session-id')
  const doCopy = () => {
    if (btn) { btn.classList.add('copied'); setTimeout(() => btn.classList.remove('copied'), 800) }
    if (txt) { txt.classList.add('copied'); setTimeout(() => txt.classList.remove('copied'), 800) }
    toast.show('已复制', { icon: '📋', duration: 1500 })
  }
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(value).then(doCopy).catch(() => {
      const ta = document.createElement('textarea')
      ta.value = value
      ta.style.cssText = 'position:fixed;opacity:0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      doCopy()
    })
  } else {
    const ta = document.createElement('textarea')
    ta.value = value
    ta.style.cssText = 'position:fixed;opacity:0'
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
    doCopy()
  }
}
</script>

<style scoped>
.metadata-modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2500;
    animation: fadeIn 0.15s ease;
}

.metadata-modal {
    background: var(--bg-primary);
    border-radius: 8px;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.15);
    max-width: 480px;
    width: 90%;
    max-height: 80vh;
    overflow: hidden;
    animation: slideUp 0.2s ease;
}

.metadata-modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color);
}

.metadata-modal-header h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
}

.metadata-close-btn {
    width: 28px;
    height: 28px;
    padding: 0;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    font-size: 24px;
    cursor: pointer;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s;
}

.metadata-close-btn:hover {
    background: var(--bg-tertiary);
}

.metadata-content {
    padding: 16px 20px;
    overflow-y: auto;
    max-height: calc(80vh - 60px);
}

.metadata-item {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 10px 0;
    border-bottom: 1px solid var(--border-color);
}

.metadata-item:last-child {
    border-bottom: none;
}

.metadata-label {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary);
    min-width: 90px;
    flex-shrink: 0;
}

.metadata-value {
    font-size: 13px;
    color: var(--text-primary);
    word-break: break-all;
}

.metadata-session-id {
    font-family: monospace;
    font-size: 12px;
    background: var(--bg-tertiary);
    padding: 2px 6px;
    border-radius: 3px;
}

.metadata-value-wrap {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
}

.metadata-value-copyable {
    cursor: pointer;
}

.metadata-copyable {
    user-select: none;
}

.metadata-copyable:hover {
    background: var(--bg-tertiary, #f5f5f5);
}

.metadata-value-copyable:hover {
    color: var(--accent-color, #4a90d9);
}

.metadata-value-copyable.copied {
    color: #22c55e;
}

.metadata-error {
    color: #ef4444;
    word-break: break-all;
}

.metadata-copy-btn {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text-muted, #999);
    padding: 2px;
    border-radius: 3px;
    transition: color 0.15s, background 0.15s;
}

.metadata-copy-btn:hover {
    color: var(--accent-color, #4a90d9);
    background: var(--bg-tertiary, #f0f0f0);
}

.metadata-copy-btn.copied {
    color: #22c55e;
}

@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

@keyframes slideUp {
    from {
        opacity: 0;
        transform: translateY(10px);
    }
    to {
        opacity: 1;
        transform: translateY(0);
    }
}
</style>
