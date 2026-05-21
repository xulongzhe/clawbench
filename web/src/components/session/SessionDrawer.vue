<template>
  <BottomSheet ref="bottomSheetRef" :open="open" auto :title="t('session.title')" @close="$emit('close')">
    <template #header>
      <Bot :size="16" class="bs-header-icon" />
      <span class="bs-header-title">{{ t('session.title') }}</span>
      <button class="create-btn" @click.stop="handleCreateClick" :title="t('session.newSession')">
        <Plus :size="16" />
      </button>
    </template>

    <div class="session-list" ref="listRef">
      <div v-if="loading" class="session-loading">{{ t('common.loading') }}</div>
      <div v-else-if="sessions.length === 0" class="session-empty">{{ t('session.noSessions') }}</div>
      <template v-else>
        <div
          v-for="session in sessionsWithStatus"
          :key="session.id"
          class="session-item"
          :class="{ active: session.id === currentSessionId, running: session.running }"
          @click="selectSession(session.id, session.backend)"
        >
          <div class="session-item-main">
            <div class="session-item-info">
              <div class="session-item-header">
                <span class="session-item-title">{{ session.title }}</span>
                <span v-if="session.unreadCount > 0" class="session-item-unread">{{ session.unreadCount }}</span>
                <span v-if="session.running" class="session-item-status running">
                  <span class="status-dot"></span>
                  {{ t('session.running') }}
                </span>
              </div>
              <div class="session-item-meta">
                <span class="session-item-time">{{ formatRelativeTime(session.updatedAt) }}</span>
                <span class="session-item-agent">{{ getAgentIcon(session.agentId) }} {{ getAgentName(session.agentId) }}</span>
                <span class="session-item-backend">{{ session.backend }}</span>
                <span v-if="session.model" class="session-item-model">{{ session.model }}</span>
              </div>
            </div>
            <button class="session-item-delete" @click.stop="deleteSession(session.id)" :title="t('common.delete')">
              <Trash2 :size="14" />
            </button>
          </div>
        </div>
        <div ref="sentinelRef" class="session-list-sentinel"></div>
        <div v-if="loadingMore" class="session-loading-more">{{ t('common.loading') }}</div>
        <div v-else-if="!hasMore && sessions.length > 0" class="session-list-end"></div>
      </template>
    </div>
  </BottomSheet>

  <!-- Agent selector dialog -->
  <ModalDialog :open="showAgentSelector" :title="t('session.selectAgent')" @close="showAgentSelector = false">
    <template #header>
      <Bot :size="16" class="modal-header-icon" />
      <span class="modal-title">{{ t('session.selectAgent') }}</span>
    </template>
    <div class="agent-list">
      <button
        v-for="agent in agents"
        :key="agent.id"
        class="agent-option"
        @click="createSession(agent.id)"
      >
        <span class="agent-option-icon">{{ agent.icon }}</span>
        <div class="agent-option-detail">
          <span class="agent-option-name">
            {{ agent.name }}
            <span v-if="isDefaultAgent(agent.id)" class="agent-default-badge">⭐</span>
          </span>
          <span class="agent-option-specialty">{{ agent.specialty }}</span>
          <div class="agent-option-tags">
            <span class="agent-tag backend-tag">{{ agent.backend }}</span>
            <span v-if="agentDefaultModelName(agent.id)" class="agent-tag model-tag">{{ agentDefaultModelName(agent.id) }}</span>
          </div>
        </div>
      </button>
    </div>
    <template #footer>
      <button class="btn btn-secondary" @click="showAgentSelector = false">{{ t('common.cancel') }}</button>
    </template>
  </ModalDialog>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { Bot, Plus, Trash2 } from 'lucide-vue-next'
import { ref, watch, computed, onUnmounted, nextTick } from 'vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import ModalDialog from '@/components/common/ModalDialog.vue'
import { useAgents } from '@/composables/useAgents'
import { useDialog } from '@/composables/useDialog.ts'
import { useSessionIdentity } from '@/composables/useSessionIdentity.ts'
import { formatRelativeTime } from '@/utils/format.ts'
import { store } from '@/stores/app.ts'

const { t } = useI18n()
const props = defineProps({
  open: Boolean,
  currentSessionId: String,
  runningSessionIds: { type: Set, default: () => new Set() },
})

const emit = defineEmits(['close', 'select', 'create', 'delete'])

const bottomSheetRef = ref(null)
const sessions = ref([])
const loading = ref(false)
const loadingMore = ref(false)
const hasMore = ref(false)
const listRef = ref(null)
const sentinelRef = ref(null)
let observer = null
const pageSize = computed(() => store.state.chatSessionPageSize || 10)
const { agents, loadAgents, getAgentIcon, getAgentName, isDefaultAgent, getAgentDefaultModelName } = useAgents()
const dialog = useDialog()
const { runningSessionsVersion } = useSessionIdentity()

/** Get the display name of an agent's default model. */
function agentDefaultModelName(agentId) {
  return getAgentDefaultModelName(agentId)
}
const showAgentSelector = ref(false)
// Guard against accidental clicks right after opening the agent selector
// (touch event propagation race: dialog appears under finger → click lands on option)
let agentSelectorOpenTime = 0

const sessionsWithStatus = computed(() => {
  // Access runningSessionsVersion to establish reactive dependency
  // so the computed re-evaluates when sessions start/stop running
  void runningSessionsVersion.value
  return sessions.value.map(s => ({
    ...s,
    // WS-maintained runningSessionIds is the authoritative source of truth.
    // It is initialized from API on app start (loadSessionsOnce) and updated
    // in real-time via WS session_update events. The API snapshot s.running
    // is stale once the drawer is open — do NOT fall back to it.
    running: props.runningSessionIds.has(s.id)
  }))
})

defineExpose({ loadSessions, openAgentSelector })

async function openAgentSelector() {
  await loadAgents()
  // If only one agent exists, skip the selector and create directly
  if (agents.value.length === 1) {
    emit('create', agents.value[0].id)
    bottomSheetRef.value?.close()
    return
  }
  showAgentSelector.value = true
  agentSelectorOpenTime = Date.now()
}

async function handleCreateClick() {
  await loadAgents()
  // If only one agent exists, skip the selector and create directly
  if (agents.value.length === 1) {
    emit('create', agents.value[0].id)
    bottomSheetRef.value?.close()
    return
  }
  showAgentSelector.value = true
  agentSelectorOpenTime = Date.now()
}

async function loadSessions() {
  loading.value = true
  hasMore.value = false
  try {
    const resp = await fetch(`/api/ai/sessions?limit=${pageSize.value}`)
    const data = await resp.json()
    sessions.value = data.sessions || []
    hasMore.value = !!data.hasMore
  } catch (err) {
    console.error('Failed to load sessions:', err)
    sessions.value = []
  } finally {
    loading.value = false
    await nextTick()
    setupObserver()
  }
}

async function loadMoreSessions() {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  try {
    const last = sessions.value[sessions.value.length - 1]
    if (!last) return
    const cursor = last.updatedAt
    const cursorId = last.id
    const resp = await fetch(`/api/ai/sessions?limit=${pageSize.value}&cursor=${encodeURIComponent(cursor)}&cursor_id=${encodeURIComponent(cursorId)}`)
    const data = await resp.json()
    const more = data.sessions || []
    if (more.length > 0) {
      sessions.value = [...sessions.value, ...more]
    }
    hasMore.value = !!data.hasMore
  } catch (err) {
    console.error('Failed to load more sessions:', err)
  } finally {
    loadingMore.value = false
  }
}

function setupObserver() {
  if (observer) {
    observer.disconnect()
    observer = null
  }
  if (!sentinelRef.value || !listRef.value) return
  observer = new IntersectionObserver((entries) => {
    if (entries[0].isIntersecting && hasMore.value && !loadingMore.value) {
      loadMoreSessions()
    }
  }, { threshold: 0.1, rootMargin: '100px', root: listRef.value })
  observer.observe(sentinelRef.value)
}

function selectSession(sessionId, backend) {
  emit('select', sessionId, backend)
  bottomSheetRef.value?.close()
}

function createSession(agentId) {
  // Ignore clicks within 400ms of opening — prevents accidental session creation
  // from touch events that propagate to the newly rendered dialog
  if (Date.now() - agentSelectorOpenTime < 400) return
  showAgentSelector.value = false
  emit('create', agentId)
  bottomSheetRef.value?.close()
}

async function deleteSession(sessionId) {
  if (!await dialog.confirm(t('session.confirmDelete'), { dangerous: true })) return
  const session = sessions.value.find(s => s.id === sessionId)
  emit('delete', sessionId, session?.backend)
}

// Every time the drawer opens, reload from API.
// This is the simplest and most reliable approach — no stale flags, no
// manual invalidate(), no cache to get out of sync. The API call is cheap
// (first page only, ~10 items) and only happens on user action (open drawer).
watch(() => props.open, async (val) => {
  if (val) {
    await Promise.all([loadSessions(), loadAgents()])
  }
})

onUnmounted(() => {
  if (observer) {
    observer.disconnect()
    observer = null
  }
})
</script>

<style scoped>
.session-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px;
  min-height: 0;
  overflow-y: auto;
  flex: 1;
}

.session-loading {
  min-height: 40vh;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #999);
  font-size: 13px;
}

.session-empty {
  min-height: 40vh;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #999);
  font-size: 13px;
}

.session-item {
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
  border: 1px solid transparent;
}

.session-item:hover {
  background: var(--bg-secondary, #f8f9fa);
}

.session-item.active {
  background: var(--accent-bg, rgba(0, 102, 204, 0.1));
  border-color: var(--accent-color, #0066cc);
}

.session-item-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.session-item-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.session-item-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.session-item-meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.session-item-title {
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  font-weight: 500;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item.active .session-item-title {
  color: var(--accent-color, #0066cc);
}

.session-item-unread {
  font-size: 9px;
  padding: 1px 5px;
  border-radius: 8px;
  font-weight: 600;
  background: #ef4444;
  color: #fff;
  flex-shrink: 0;
  min-width: 14px;
  text-align: center;
}

.session-item-status.running {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  color: #22c55e;
  font-weight: 500;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.session-item.running {
  background: rgba(34, 197, 94, 0.05);
}

.session-item-time {
  font-size: 11px;
  color: var(--text-muted, #999);
}

.session-item-agent {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  background: var(--bg-tertiary, #e9ecef);
  color: var(--text-secondary, #495057);
}

.session-item-backend {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  background: rgba(0, 102, 204, 0.1);
  color: var(--accent-color, #0066cc);
  text-transform: lowercase;
}

.session-item-model {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 3px;
  font-weight: 500;
  flex-shrink: 0;
  background: rgba(100, 100, 100, 0.08);
  color: var(--text-muted, #999);
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item-delete {
  width: 22px;
  height: 22px;
  border: none;
  background: none;
  font-size: 16px;
  color: var(--text-muted, #999);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s;
  flex-shrink: 0;
}

.session-item-delete:hover {
  color: #dc3545;
  background: var(--bg-tertiary, #f0f0f0);
}

.create-btn {
  margin-left: auto;
  width: 24px;
  height: 24px;
  border: none;
  background: none;
  color: var(--accent-color, #0066cc);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: background 0.15s;
}

.create-btn:hover {
  background: rgba(0, 102, 204, 0.1);
}

/* Agent selector content */
.agent-list {
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 2px;
  overflow-y: auto;
}

.agent-option {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border: none;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  border-radius: 0;
  background: none;
  cursor: pointer;
  transition: background 0.12s;
  text-align: left;
}

.agent-option:last-child {
  border-bottom: none;
}

.agent-option:hover {
  background: none;
  border-left: 3px solid var(--accent-color, #0066cc);
  padding-left: 5px;
}

.agent-option:hover .agent-option-name {
  color: var(--accent-color, #0066cc);
}

.agent-option:hover .agent-option-specialty {
  color: var(--text-secondary, #666);
}

.agent-option:hover .agent-tag {
  opacity: 1;
}

.agent-option:active {
  border-left-color: color-mix(in srgb, var(--accent-color, #0066cc) 70%, transparent);
}

.agent-option-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.agent-option-detail {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.agent-option-name {
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  font-weight: 500;
}

.agent-default-badge {
  font-size: 10px;
  margin-left: 2px;
  vertical-align: middle;
}

.agent-option-specialty {
  font-size: 11px;
  color: var(--text-secondary, #666);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-option-tags {
  display: flex;
  gap: 4px;
  margin-top: 2px;
}

.agent-tag {
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 0;
  font-weight: 500;
  flex-shrink: 0;
}

.backend-tag {
  background: rgba(0, 102, 204, 0.1);
  color: var(--accent-color, #0066cc);
  text-transform: lowercase;
}

.model-tag {
  background: rgba(100, 100, 100, 0.08);
  color: var(--text-muted, #999);
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.btn-secondary {
  padding: 5px 14px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-primary, #1a1a1a);
  transition: background 0.15s;
}

.btn-secondary:hover { background: #e0e0e0; }

.session-list-sentinel {
  height: 1px;
}

.session-loading-more {
  padding: 12px;
  text-align: center;
  color: var(--text-muted, #999);
  font-size: 12px;
}

.session-list-end {
  height: 0;
}
</style>
