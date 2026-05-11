<template>
  <div class="task-breadcrumb">
    <!-- Root crumb: 任务列表 -->
    <span
      class="task-crumb"
      :class="{ current: currentView === 'list' && !formOpen, clickable: currentView !== 'list' || formOpen }"
      @click="(currentView !== 'list' || formOpen) && $emit('navigate', 'list')"
    >{{ t('task.title') }}</span>

    <!-- Task crumb (shown when a task is selected) -->
    <template v-if="taskName">
      <span class="task-crumb-sep">›</span>
      <span
        class="task-crumb"
        :class="{ current: (currentView === 'settings') && !execDetailOpen && !formOpen, clickable: (currentView === 'history' || currentView === 'exec') || formOpen }"
        @click="((currentView === 'history' || currentView === 'exec') || formOpen) && $emit('navigate', 'settings')"
      >{{ taskName }}</span>
    </template>

    <!-- History crumb -->
    <template v-if="currentView === 'history' && !execDetailOpen && !formOpen">
      <span class="task-crumb-sep">›</span>
      <span class="task-crumb current">{{ t('task.exec.title') }}</span>
    </template>

    <!-- Exec crumb (shown when viewing execution detail) -->
    <template v-if="execDetailOpen">
      <span class="task-crumb-sep">›</span>
      <span class="task-crumb current">{{ t('task.exec.title') }}</span>
    </template>

    <!-- Form crumb -->
    <template v-if="formOpen">
      <span class="task-crumb-sep">›</span>
      <span class="task-crumb current">{{ formMode === 'create' ? t('task.form.createTitle') : t('task.form.editTitle') }}</span>
    </template>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps({
  currentView: { type: String, default: 'list' },
  taskName: String,
  execDetailOpen: Boolean,
  formOpen: Boolean,
  formMode: { type: String, default: 'create' },
})

defineEmits(['navigate'])
</script>

<style scoped>
.task-breadcrumb {
  display: flex;
  align-items: center;
  gap: 2px;
  overflow-x: auto;
  font-size: 12px;
  color: var(--text-muted, #999);
  scrollbar-width: none;
  flex: 1;
  min-width: 0;
}

.task-breadcrumb::-webkit-scrollbar {
  display: none;
}

.task-crumb {
  padding: 1px 4px;
  border-radius: 3px;
  white-space: nowrap;
  transition: background 0.15s, color 0.15s;
}

/* Clickable crumb: has navigation target */
.task-crumb.clickable {
  cursor: pointer;
}

.task-crumb.clickable:hover {
  background: var(--bg-secondary, #e0e0e0);
  color: var(--accent-color, #4a90d9);
}

.task-crumb.current {
  font-weight: 600;
  color: var(--text-primary, #1a1a1a);
  cursor: default;
}

.task-crumb.current:hover {
  background: none;
  color: var(--text-primary, #1a1a1a);
}

.task-crumb-sep {
  color: var(--text-muted, #999);
  font-size: 10px;
}
</style>
