<template>
  <div class="task-breadcrumb">
    <!-- Root crumb: 任务列表 -->
    <span
      class="crumb"
      :class="{ current: isList, clickable: !isList, first: true }"
      @click="!isList && navigate('list')"
    >{{ t('task.title') }}</span>

    <!-- Task name crumb -->
    <span
      v-if="taskName"
      class="crumb"
      :class="{ current: isSettings, clickable: !isSettings }"
      @click="!isSettings && navigate('settings')"
    >{{ taskName }}</span>

    <!-- History crumb -->
    <span
      v-if="showHistoryCrumb"
      class="crumb"
      :class="{ current: isHistory, clickable: !isHistory }"
      @click="!isHistory && navigate('history')"
    >{{ t('task.exec.title') }}</span>

    <!-- Exec detail crumb -->
    <span
      v-if="execDetailOpen"
      class="crumb current"
    >{{ t('task.exec.detail') }}</span>

    <!-- Form crumb -->
    <span
      v-if="formViewOpen"
      class="crumb current"
    >{{ formMode === 'create' ? t('task.form.createTitle') : t('task.form.editTitle') }}</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTaskTab } from '@/composables/useTaskTab'
import { store } from '@/stores/app'

const { t } = useI18n()
const { currentView, selectedTaskId, execDetailOpen, formViewOpen, formMode, navigateToList, navigateToTaskSettings, navigateToTaskHistory } = useTaskTab()

// Derive task name from store (same pattern as TaskTab)
const taskName = computed(() => {
  if (!selectedTaskId.value) return null
  return (store.state.tasks || []).find(t => t.id === selectedTaskId.value)?.name || null
})

// Derived state
const isList = computed(() => currentView.value === 'list' && !formViewOpen.value)
const isSettings = computed(() => currentView.value === 'settings' && !execDetailOpen.value && !formViewOpen.value)
const isHistory = computed(() => currentView.value === 'history' && !execDetailOpen.value && !formViewOpen.value)

const showHistoryCrumb = computed(() => {
  if (formViewOpen.value) return false
  return currentView.value === 'history'
})

// Centralized navigation
function navigate(target) {
  if (target === 'list') {
    navigateToList()
  } else if (target === 'settings') {
    const tid = selectedTaskId.value
    if (tid) navigateToTaskSettings(tid)
  } else if (target === 'history') {
    const tid = selectedTaskId.value
    if (tid) navigateToTaskHistory(tid)
  }
}
</script>

<style scoped>
.task-breadcrumb {
  display: flex;
  align-items: center;
  overflow-x: auto;
  scrollbar-width: none;
  flex: 1;
  min-width: 0;
}

.task-breadcrumb::-webkit-scrollbar {
  display: none;
}

/* ── Chevron crumb base ── */
.crumb {
  position: relative;
  display: flex;
  align-items: center;
  height: 22px;
  padding: 0 16px 0 18px;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
  cursor: default;
  color: var(--text-secondary, #666);
  background: var(--bg-tertiary, #e9ecef);
  transition: background 0.15s, color 0.15s;
}

/* First crumb: rounded left, no left indent for arrow notch */
.crumb.first {
  padding-left: 10px;
  border-radius: 4px 0 0 4px;
}

/* Right arrow — same color as crumb bg, with light edge seam */
.crumb::after {
  content: '';
  position: absolute;
  right: -8px;
  top: 0;
  width: 0;
  height: 0;
  border-style: solid;
  border-width: 11px 0 11px 8px;
  border-color: transparent transparent transparent var(--bg-tertiary, #e9ecef);
  transition: border-color 0.15s;
  z-index: 1;
  /* Thin light line along the diagonal edge of the arrow */
  filter: drop-shadow(1px 0 0 rgba(255, 255, 255, 0.4));
}

/* ── Clickable crumb ── */
.crumb.clickable {
  cursor: pointer;
}

@media (hover: hover) {
  .crumb.clickable:hover {
    background: var(--bg-secondary, #dde1e6);
    color: var(--accent-color, #4a90d9);
  }

  .crumb.clickable:hover::after {
    border-left-color: var(--bg-secondary, #dde1e6);
  }
}

.crumb.clickable:active {
  background: var(--bg-secondary, #d0d5da);
}

.crumb.clickable:active::after {
  border-left-color: var(--bg-secondary, #d0d5da);
}

/* ── Current (active) crumb — accent color darkened ── */
.crumb.current {
  background: color-mix(in srgb, var(--accent-color, #0066cc) 75%, #000);
  color: #fff;
  font-weight: 600;
}

.crumb.current::after {
  border-left-color: color-mix(in srgb, var(--accent-color, #0066cc) 75%, #000);
}

/* Last crumb: no arrow */
.crumb:last-child::after {
  display: none;
}

@media (hover: hover) {
  .crumb.current:hover {
    background: color-mix(in srgb, var(--accent-color, #0066cc) 75%, #000);
    color: #fff;
  }

  .crumb.current:hover::after {
    border-left-color: color-mix(in srgb, var(--accent-color, #0066cc) 75%, #000);
  }
}
</style>
