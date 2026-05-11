<template>
  <div class="task-detail-page">
    <!-- Compact header: breadcrumb only -->
    <div class="detail-header">
      <TaskBreadcrumb
        currentView="settings"
        :taskName="task?.name"
        @navigate="onBreadcrumbNavigate"
      />
    </div>
    <!-- Settings content -->
    <div class="detail-content">
      <TaskOverviewTab :task="task" @deleted="$emit('deleted')" @edit="$emit('edit')" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import TaskBreadcrumb from '@/components/task/TaskBreadcrumb.vue'
import TaskOverviewTab from '@/components/task/TaskOverviewTab.vue'
import { useTaskTab } from '@/composables/useTaskTab'

const { t } = useI18n()
const { goBack } = useTaskTab()

defineProps<{
  task: any
}>()

defineEmits<{
  back: []
  edit: []
  deleted: []
}>()

function onBreadcrumbNavigate(view: string) {
  if (view === 'list') {
    goBack()
  }
}
</script>

<style scoped>
.task-detail-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.detail-header {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  flex-shrink: 0;
}

.detail-content {
  flex: 1;
  overflow-y: auto;
}
</style>
