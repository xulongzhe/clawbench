<template>
  <BottomSheet ref="bottomSheetRef" :open="open" title="端口转发" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/>
        <path d="M2 17l10 5 10-5"/>
        <path d="M2 12l10 5 10-5"/>
      </svg>
      <span class="bs-header-title">端口转发</span>
    </template>

    <div class="proxy-panel">
      <!-- Tunnel status banner -->
      <div v-if="tunnelStatus === 'disconnected'" class="tunnel-banner error">
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/>
          <line x1="15" y1="9" x2="9" y2="15"/>
          <line x1="9" y1="9" x2="15" y2="15"/>
        </svg>
        <div class="tunnel-banner-content">
          <span class="tunnel-banner-title">SSH 隧道未连接</span>
          <span class="tunnel-banner-detail">端口转发将无法使用，请检查网络或重新打开页面</span>
        </div>
      </div>
      <div v-else-if="tunnelStatus === 'degraded'" class="tunnel-banner warning">
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
          <line x1="12" y1="9" x2="12" y2="13"/>
          <line x1="12" y1="17" x2="12.01" y2="17"/>
        </svg>
        <div class="tunnel-banner-content">
          <span class="tunnel-banner-title">转发端口无响应</span>
          <span class="tunnel-banner-detail">SSH 隧道已连接，但所有端口的服务均未响应</span>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="proxy-loading">加载中...</div>

      <!-- Port list -->
      <div v-else-if="ports.length > 0" class="proxy-list">
        <ProxyPortItem
          v-for="p in ports"
          :key="p.port"
          :port="p.port"
          :name="p.name"
          :protocol="p.protocol"
          :active="p.active"
          :tunnel-disconnected="tunnelStatus === 'disconnected'"
          @open="openPort"
          @remove="handleRemove"
        />
      </div>

      <!-- Empty state -->
      <div v-else class="proxy-empty">
        <div class="proxy-empty-text">暂无转发端口</div>
        <div class="proxy-empty-hint">添加服务器上的端口，即可在浏览器或其他应用中直接访问</div>
      </div>

      <!-- Add port form -->
      <div class="proxy-add">
        <div v-if="showAddForm" class="proxy-add-form">
          <select v-model="newProtocol" class="proxy-add-select">
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
          </select>
          <input
            ref="portInputRef"
            v-model="newPort"
            type="number"
            class="proxy-add-input"
            placeholder="端口号"
            min="1"
            max="65535"
            @keydown.enter="handleAdd"
          />
          <input
            v-model="newName"
            type="text"
            class="proxy-add-input name-input"
            placeholder="名称（可选）"
            @keydown.enter="handleAdd"
          />
          <button class="proxy-add-confirm" @click="handleAdd" :disabled="!isValidPort">确定</button>
          <button class="proxy-add-cancel" @click="showAddForm = false">取消</button>
        </div>
        <div v-else class="proxy-add-buttons">
          <button class="proxy-add-btn" @click="showAddForm = true; nextTick(() => portInputRef?.focus())">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            添加端口
          </button>
          <button class="proxy-add-btn" :class="{ detecting }" @click="handleDetect" :disabled="detecting">
            <span class="detect-icon-wrap">
              <svg class="detect-icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
              <span v-if="detecting" class="radar-ping"></span>
            </span>
            {{ detecting ? '检测中...' : '自动检测' }}
          </button>
        </div>
      </div>

      <!-- Detected ports (suggestion chips) -->
      <div v-if="detectedPorts.length > 0" class="proxy-detected">
        <div class="proxy-detected-label">检测到的端口：</div>
        <div class="proxy-detected-chips">
          <button
            v-for="(p, i) in detectedPortsNotRegistered"
            :key="p.port"
            class="detect-chip"
            :class="p.protocol"
            :style="{ animationDelay: `${i * 60}ms` }"
            @click="handleQuickAdd(p.port, p.protocol)"
          >{{ p.port }} <span class="chip-proto">{{ p.protocol }}</span></button>
          <span v-if="detectedPortsNotRegistered.length === 0" class="detect-all-registered">全部已注册</span>
        </div>
      </div>

      <!-- SSH tunnel info (desktop only: shows command for manual tunnel setup) -->
      <div v-if="sshInfo && sshInfo.enabled && !isAppMode" class="proxy-ssh-section">
        <div class="proxy-ssh-divider"></div>
        <div class="proxy-ssh-header">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
            <path d="M7 11V7a5 5 0 0110 0v4"/>
          </svg>
          <span>SSH 隧道</span>
        </div>
        <div class="proxy-ssh-meta">
          <span class="ssh-label">{{ sshInfo.username }}@{{ sshInfo.host }}:{{ sshInfo.port }}</span>
          <button class="ssh-copy-btn" @click="copySSHCommand" title="复制 SSH 命令">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
              <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
            </svg>
            {{ sshCopied ? '已复制' : '复制命令' }}
          </button>
        </div>
        <div v-if="sshInfo.fingerprint" class="ssh-fingerprint">
          {{ sshInfo.fingerprint }}
        </div>
      </div>
    </div>
  </BottomSheet>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import ProxyPortItem from './ProxyPortItem.vue'
import { usePortForward } from '@/composables/usePortForward.ts'

const props = defineProps({ open: Boolean })
const emit = defineEmits(['close'])

const bottomSheetRef = ref(null)
const showAddForm = ref(false)
const newPort = ref('')
const newName = ref('')
const newProtocol = ref('http')
const detecting = ref(false)
const portInputRef = ref(null)

const { ports, detectedPorts, loading, isAppMode, sshInfo, tunnelStatus, tunnelMessage, registerPort, unregisterPort, detectPorts, checkTunnelHealth, openPort } = usePortForward()

const sshCopied = ref(false)

const isValidPort = computed(() => {
  const p = parseInt(newPort.value)
  return p > 0 && p <= 65535
})

const detectedPortsNotRegistered = computed(() => {
  const registered = new Set(ports.value.map(p => p.port))
  return detectedPorts.value.filter(p => !registered.has(p.port))
})

async function handleAdd() {
  if (!isValidPort.value) return
  await registerPort(parseInt(newPort.value), newName.value || undefined, newProtocol.value)
  newPort.value = ''
  newName.value = ''
  showAddForm.value = false
}

async function handleQuickAdd(port, protocol) {
  await registerPort(port, '自动检测', protocol || 'http')
}

async function handleRemove(port) {
  await unregisterPort(port)
}

async function handleDetect() {
  detecting.value = true
  try {
    await detectPorts()
  } finally {
    detecting.value = false
  }
}

async function copySSHCommand() {
  if (!sshInfo.value?.command) return
  try {
    await navigator.clipboard.writeText(sshInfo.value.command)
    sshCopied.value = true
    setTimeout(() => { sshCopied.value = false }, 2000)
  } catch {}
}

watch(() => props.open, async (val) => {
  if (val) {
    await checkTunnelHealth()
  }
})
</script>

<style scoped>
.proxy-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 6px;
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
}

/* Tunnel status banner */
.tunnel-banner {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  border-left: 3px solid;
  flex-shrink: 0;
}

.tunnel-banner.error {
  background: rgba(239, 68, 68, 0.08);
  border-left-color: #ef4444;
  color: #dc2626;
}

.tunnel-banner.warning {
  background: rgba(245, 158, 11, 0.08);
  border-left-color: #f59e0b;
  color: #d97706;
}

.tunnel-banner svg {
  flex-shrink: 0;
  margin-top: 1px;
}

.tunnel-banner-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tunnel-banner-title {
  font-size: 13px;
  font-weight: 600;
}

.tunnel-banner-detail {
  font-size: 11px;
  opacity: 0.8;
}

.proxy-loading,
.proxy-empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--text-muted, #999);
  font-size: 13px;
}

.proxy-empty-hint {
  font-size: 11px;
  margin-top: 4px;
  color: var(--text-muted, #999);
  opacity: 0.7;
}

.proxy-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.proxy-add {
  border-top: 1px solid var(--border-color, #e5e5e5);
  padding-top: 8px;
}

.proxy-add-buttons {
  display: flex;
  gap: 8px;
}

.proxy-add-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  border: 1px dashed var(--border-color, #e5e5e5);
  border-radius: 6px;
  background: none;
  color: var(--text-secondary, #666);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.proxy-add-btn:hover {
  border-color: var(--accent-color, #0066cc);
  color: var(--accent-color, #0066cc);
  background: var(--bg-tertiary, #f5f5f5);
}

.proxy-add-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.proxy-add-btn.detecting {
  border-color: var(--accent-color, #0066cc);
  color: var(--accent-color, #0066cc);
}

.detect-icon-wrap {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}

.detect-icon-wrap .detect-icon {
  position: relative;
  z-index: 1;
}

.radar-ping {
  position: absolute;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--accent-color, #0066cc);
  opacity: 0;
  animation: radar-ping 1.2s ease-out infinite;
}

@keyframes radar-ping {
  0% {
    transform: scale(0.5);
    opacity: 0.5;
  }
  100% {
    transform: scale(2.5);
    opacity: 0;
  }
}

.proxy-add-form {
  display: flex;
  gap: 4px;
  align-items: center;
}

.proxy-add-input {
  flex: 1;
  min-width: 0;
  padding: 6px 8px;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 4px;
  font-size: 13px;
  background: var(--bg-primary, #fff);
  color: var(--text-primary, #1a1a1a);
  font-family: inherit;
}

.proxy-add-input:focus {
  outline: none;
  border-color: var(--accent-color, #0066cc);
}

.proxy-add-select {
  padding: 6px 4px;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 4px;
  font-size: 13px;
  background: var(--bg-primary, #fff);
  color: var(--text-primary, #1a1a1a);
  font-family: inherit;
  cursor: pointer;
  flex-shrink: 0;
}

.proxy-add-select:focus {
  outline: none;
  border-color: var(--accent-color, #0066cc);
}

.name-input {
  flex: 2;
}

.proxy-add-confirm,
.proxy-add-cancel {
  padding: 6px 10px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.proxy-add-confirm {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

.proxy-add-confirm:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.proxy-add-cancel {
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-secondary, #666);
}

.proxy-detected {
  padding: 4px 0;
}

.proxy-detected-label {
  font-size: 11px;
  color: var(--text-muted, #999);
  margin-bottom: 4px;
}

.proxy-detected-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.detect-chip {
  padding: 3px 8px;
  border: 1px solid var(--accent-color, #0066cc);
  border-radius: 12px;
  background: none;
  color: var(--accent-color, #0066cc);
  font-size: 11px;
  font-family: monospace;
  cursor: pointer;
  transition: all 0.15s;
  display: flex;
  align-items: center;
  gap: 3px;
  animation: chip-appear 0.3s ease-out both;
}

.detect-chip.https {
  border-color: #2563eb;
  color: #2563eb;
}

.detect-chip .chip-proto {
  font-size: 9px;
  font-family: sans-serif;
  padding: 1px 3px;
  border-radius: 3px;
  background: rgba(0, 102, 204, 0.1);
}

.detect-chip.https .chip-proto {
  background: rgba(37, 99, 235, 0.12);
}

.detect-chip:hover {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

.detect-all-registered {
  font-size: 11px;
  color: var(--text-muted, #999);
  opacity: 0.7;
}

@keyframes chip-appear {
  from {
    opacity: 0;
    transform: scale(0.8) translateY(4px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

/* SSH tunnel section */
.proxy-ssh-section {
  margin-top: 4px;
}

.proxy-ssh-divider {
  height: 1px;
  background: var(--border-color, #e5e5e5);
  margin: 8px 0;
}

.proxy-ssh-header {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary, #666);
  margin-bottom: 4px;
}

.proxy-ssh-meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.ssh-label {
  font-family: monospace;
  font-size: 11px;
  color: var(--text-secondary, #666);
}

.ssh-copy-btn {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 6px;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 4px;
  background: none;
  color: var(--text-secondary, #666);
  font-size: 10px;
  cursor: pointer;
  transition: all 0.15s;
}

.ssh-copy-btn:hover {
  border-color: var(--accent-color, #0066cc);
  color: var(--accent-color, #0066cc);
}

.ssh-fingerprint {
  font-family: monospace;
  font-size: 9px;
  color: var(--text-muted, #999);
  margin-top: 2px;
  word-break: break-all;
}
</style>
