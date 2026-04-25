<template>
  <div v-if="commit || isWorkingTree" class="diff-meta-panel">
    <template v-if="isWorkingTree">
      <div class="diff-meta-row diff-meta-row-msg">
        <span class="diff-meta-label">说明</span>
        <span class="diff-meta-value">工作区变更</span>
      </div>
    </template>
    <template v-else-if="commit">
      <div class="diff-meta-row">
        <span class="diff-meta-label">SHA</span>
        <span class="diff-meta-value diff-meta-sha">{{ commit.sha.substring(0, 8) }}</span>
      </div>
      <div class="diff-meta-row">
        <span class="diff-meta-label">作者</span>
        <span class="diff-meta-value">{{ commit.author }}</span>
      </div>
      <div class="diff-meta-row">
        <span class="diff-meta-label">时间</span>
        <span class="diff-meta-value">{{ formatDate(commit.date) }}</span>
      </div>
      <div class="diff-meta-row diff-meta-row-msg">
        <span class="diff-meta-label">说明</span>
        <span class="diff-meta-value">{{ commit.msg }}</span>
      </div>
    </template>
  </div>
</template>

<script setup>
defineProps({
  commit: Object,
  isWorkingTree: Boolean,
})

function formatDate(dateStr) {
  if (!dateStr) return ''
  try {
    const d = new Date(dateStr)
    return d.toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return dateStr
  }
}
</script>

<style scoped>
.diff-meta-panel {
  padding: 12px 14px;
  border-bottom: 1px solid var(--border-color, #dee2e6);
  background: var(--bg-secondary, #f8f9fa);
  display: flex;
  flex-direction: column;
  gap: 5px;
  flex-shrink: 0;
}

.diff-meta-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 13px;
}

.diff-meta-label {
  color: var(--text-muted, #999);
  flex-shrink: 0;
  width: 36px;
  padding-top: 1px;
}

.diff-meta-value {
  color: var(--text-primary, #212529);
  word-break: break-all;
}

.diff-meta-sha {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, monospace;
  font-size: 12px;
  color: var(--accent-color, #4a90d9);
}

.diff-meta-row-msg .diff-meta-value {
  font-weight: 500;
}
</style>
