<template>
  <ModalDialog :open="open" title="定时任务" @close="$emit('close')">
    <!-- Tabs -->
    <div class="dialog-tabs-row">
      <div class="dialog-tabs">
        <button class="dialog-tab" :class="{ active: tab === 'details' }" @click="tab = 'details'">详情</button>
        <button class="dialog-tab" :class="{ active: tab === 'executions' }" @click="tab = 'executions'">执行记录</button>
      </div>
    </div>

    <!-- Details tab -->
    <div v-if="tab === 'details'" class="details-content">
      <div v-if="saving" class="saving-indicator">保存中...</div>

      <div class="form-group">
        <label class="form-label">任务名称</label>
        <input type="text" class="form-input" v-model="form.name" placeholder="任务名称" />
      </div>

      <div class="form-group">
        <label class="form-label">Cron 表达式</label>
        <input type="text" class="form-input" v-model="form.cronExpr" placeholder="0 */10 * * *" />
        <div class="form-hint">示例: 0 */10 * * * (每10分钟), 0 9 * * * (每天9点)</div>
      </div>

      <div class="form-group">
        <label class="form-label">执行 Agent</label>
        <select class="form-select" v-model="form.agentId">
          <option v-for="agent in agents" :key="agent.id" :value="agent.id">
            {{ agent.icon }} {{ agent.name }}
          </option>
        </select>
      </div>

      <div class="form-group">
        <label class="form-label">执行模式</label>
        <div class="radio-group">
          <label class="radio-label">
            <input type="radio" v-model="form.repeatMode" value="once" />
            <span>单次执行</span>
          </label>
          <label class="radio-label">
            <input type="radio" v-model="form.repeatMode" value="limited" />
            <span>限制次数</span>
          </label>
          <label class="radio-label">
            <input type="radio" v-model="form.repeatMode" value="unlimited" />
            <span>不限次数</span>
          </label>
        </div>
      </div>

      <div v-if="form.repeatMode === 'limited'" class="form-group">
        <label class="form-label">最大执行次数</label>
        <input type="number" class="form-input" v-model.number="form.maxRuns" min="1" />
      </div>

      <div class="form-group">
        <label class="form-label">提示词 (Prompt)</label>
        <textarea class="form-textarea" v-model="form.prompt" rows="10" placeholder="输入要发送给AI的提示词..."></textarea>
      </div>

      <div class="form-group">
        <label class="form-label">描述</label>
        <textarea class="form-textarea" v-model="form.description" rows="3" placeholder="任务描述（可选）"></textarea>
      </div>
    </div>

    <!-- Executions tab -->
    <div v-if="tab === 'executions'" class="executions-content">
      <div v-if="executionsLoading" class="dialog-loading">加载中...</div>
      <div v-else-if="executions.length === 0" class="dialog-empty">暂无执行记录</div>
      <div v-for="(exec, idx) in executions" :key="idx" class="execution-item">
        <div class="execution-time">{{ exec.createdAt }}</div>
        <div class="execution-reply">{{ exec.summary }}</div>
      </div>
    </div>

    <template #footer>
      <button v-if="tab === 'details'" class="btn btn-primary" :disabled="saving" @click="saveTask">
        保存
      </button>
      <button class="btn btn-secondary" @click="$emit('close')">关闭</button>
    </template>
  </ModalDialog>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'
import ModalDialog from './ModalDialog.vue'

const props = defineProps({
  open: Boolean,
  task: Object,
})

const emit = defineEmits(['close', 'saved'])

const tab = ref('details')
const saving = ref(false)
const agents = ref([])
const executions = ref([])
const executionsLoading = ref(false)

// Form data (initialized from props.task)
const form = ref({
  id: '',
  name: '',
  cronExpr: '',
  agentId: '',
  prompt: '',
  description: '',
  repeatMode: 'unlimited',
  maxRuns: 0,
})

async function loadAgents() {
  try {
    const resp = await fetch('/api/agents')
    const data = await resp.json()
    agents.value = data.agents || []
  } catch (err) {
    console.error('Failed to load agents:', err)
  }
}

async function loadExecutions() {
  if (!props.task?.id) return
  executionsLoading.value = true
  try {
    const resp = await fetch(`/api/tasks/${props.task.id}/executions`)
    const data = await resp.json()
    const rawExecutions = data.executions || []
    executions.value = rawExecutions.map(exec => {
      let summary = ''
      try {
        const parsed = JSON.parse(exec.content)
        if (parsed.blocks && Array.isArray(parsed.blocks)) {
          for (const block of parsed.blocks) {
            if (block.type === 'text' && block.text) {
              summary = block.text
              break
            }
          }
        }
      } catch {
        summary = exec.content || ''
      }
      if (summary.length > 200) {
        summary = summary.substring(0, 200) + '...'
      }
      return { ...exec, summary }
    })
  } catch (err) {
    console.error('Failed to load executions:', err)
  } finally {
    executionsLoading.value = false
  }
}

async function saveTask() {
  if (saving.value) return
  saving.value = true

  try {
    const resp = await fetch(`/api/tasks/${form.value.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: form.value.name,
        cron_expr: form.value.cronExpr,
        agent_id: form.value.agentId,
        prompt: form.value.prompt,
        description: form.value.description,
        repeat_mode: form.value.repeatMode,
        max_runs: form.value.maxRuns,
      }),
    })

    if (!resp.ok) {
      const err = await resp.json()
      alert('保存失败: ' + (err.error || '未知错误'))
      return
    }

    emit('saved')
  } catch (err) {
    alert('保存失败: ' + err.message)
  } finally {
    saving.value = false
  }
}

// Initialize form when task changes
watch(() => props.task, (task) => {
  if (task) {
    form.value = {
      id: task.id,
      name: task.name,
      cronExpr: task.cronExpr,
      agentId: task.agentId,
      prompt: task.prompt,
      description: task.description || '',
      repeatMode: task.repeatMode || 'unlimited',
      maxRuns: task.maxRuns || 0,
    }
    loadExecutions()
  }
})

// Load agents when dialog opens
watch(() => props.open, (isOpen) => {
  if (isOpen && agents.value.length === 0) {
    loadAgents()
  }
})

onMounted(() => {
  // Initial load handled by watch
})
</script>

<style scoped>
/* Tabs */
.dialog-tabs-row {
  display: flex;
  align-items: center;
  padding: 0 10px;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
  background: var(--bg-tertiary, #f5f5f5);
  flex-shrink: 0;
}

.dialog-tabs {
  display: flex;
  gap: 0;
}

.dialog-tab {
  padding: 5px 12px;
  border: none;
  background: transparent;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  color: var(--text-muted, #999);
  border-bottom: 2px solid transparent;
  transition: color 0.2s, border-color 0.2s;
}

.dialog-tab:hover { color: var(--text-secondary, #666); }
.dialog-tab.active { color: var(--accent-color, #0066cc); border-bottom-color: var(--accent-color, #0066cc); }

/* Details content */
.details-content {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
}

.saving-indicator {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  text-align: center;
  margin-bottom: 8px;
}

.form-group {
  margin-bottom: 10px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-primary, #1a1a1a);
  margin-bottom: 3px;
}

.form-input,
.form-select,
.form-textarea {
  width: 100%;
  padding: 6px 8px;
  border: 1px solid var(--border-color, #e5e5e5);
  border-radius: 4px;
  font-size: 13px;
  background: var(--bg-primary, #fff);
  color: var(--text-primary, #1a1a1a);
  box-sizing: border-box;
  outline: none;
  transition: border-color 0.15s;
}

.form-input:focus,
.form-select:focus,
.form-textarea:focus {
  border-color: var(--accent-color, #0066cc);
}

.form-textarea {
  resize: vertical;
  min-height: 60px;
  font-family: inherit;
}

.form-hint {
  font-size: 11px;
  color: var(--text-muted, #999);
  margin-top: 2px;
}

.radio-group {
  display: flex;
  flex-direction: row;
  gap: 12px;
}

.radio-label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--text-primary, #1a1a1a);
  cursor: pointer;
}

/* Executions content */
.executions-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px 10px;
}

.execution-item {
  padding: 8px 0;
  border-bottom: 1px solid var(--border-color, #e5e5e5);
}

.execution-item:last-child {
  border-bottom: none;
}

.execution-time {
  font-size: 11px;
  color: var(--text-muted, #999);
  margin-bottom: 3px;
}

.execution-reply {
  font-size: 12px;
  color: var(--text-primary, #1a1a1a);
  white-space: pre-wrap;
  word-break: break-word;
}

.dialog-loading,
.dialog-empty {
  text-align: center;
  padding: 20px 12px;
  color: var(--text-muted, #999);
  font-size: 13px;
}

/* Buttons */
.btn {
  padding: 5px 14px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
}

.btn-primary {
  background: var(--accent-color, #0066cc);
  color: #fff;
}

.btn-primary:hover { background: #0055aa; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-secondary {
  background: var(--bg-tertiary, #f0f0f0);
  color: var(--text-primary, #1a1a1a);
}

.btn-secondary:hover { background: #e0e0e0; }
</style>
