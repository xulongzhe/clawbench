<template>
  <div v-if="loading" class="git-diff-loading">
    <div class="spinner" style="width:24px;height:24px;border-width:2px;margin:0 auto;" />
  </div>
  <div v-else-if="empty" class="git-diff-empty">无变更</div>
  <div v-else :class="['git-diff-scroll', { 'no-wrap': noWrap }]" v-html="html" />
</template>

<script setup>
defineProps({
  loading: Boolean,
  empty: Boolean,
  html: { type: String, default: '' },
  noWrap: Boolean,
})
</script>

<style scoped>
.git-diff-loading {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.git-diff-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, #999);
  font-size: 14px;
}

.git-diff-scroll {
  padding: 12px;
}

.git-diff-scroll.no-wrap {
  overflow-x: auto;
}

.git-diff-scroll :deep(.diff-card-view) {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.git-diff-scroll :deep(.diff-hunk-loc) {
  font-size: 12px;
  color: var(--text-muted, #999);
  padding: 0 4px;
}

.git-diff-scroll :deep(.diff-card-pair) {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.git-diff-scroll :deep(.diff-card) {
  overflow-x: auto;
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
}

.git-diff-scroll :deep(.diff-card-add) {
  background: rgba(34, 197, 94, 0.08);
  border-left: 3px solid #22c55e;
}

.git-diff-scroll :deep(.diff-card-del) {
  background: rgba(239, 68, 68, 0.08);
  border-left: 3px solid #ef4444;
}

.git-diff-scroll :deep(.diff-card-label) {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 0;
  font-family: system-ui, sans-serif;
  letter-spacing: 0.02em;
  display: inline-block;
  margin-bottom: 4px;
}

.git-diff-scroll :deep(.diff-card-add .diff-card-label) {
  background: rgba(34, 197, 94, 0.15);
  color: #16a34a;
}

.git-diff-scroll :deep(.diff-card-del .diff-card-label) {
  background: rgba(239, 68, 68, 0.15);
  color: #dc2626;
}

.git-diff-scroll :deep(.diff-card-line) {
  padding: 2px 10px;
  white-space: pre;
  color: inherit;
}

.git-diff-scroll :deep(.diff-raw) {
  font-family: 'SF Mono', 'Fira Code', Menlo, Monaco, 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--text-primary, #212529);
  margin: 0;
}
</style>
