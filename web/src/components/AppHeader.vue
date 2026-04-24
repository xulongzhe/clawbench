<template>
  <header class="header">
    <img class="header-logo" src="/logo.png" alt="ClawBench">

    <button class="project-switch-btn" @click="emit('openProjectDialog')" title="切换项目">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
      </svg>
      <span class="project-name">{{ projectName }}</span>
    </button>

    <button class="theme-toggle" @click="$emit('toggleTheme')" aria-label="Toggle theme">
      <svg v-if="theme === 'dark'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
      </svg>
      <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="5"/>
        <line x1="12" y1="1" x2="12" y2="3"/>
        <line x1="12" y1="21" x2="12" y2="23"/>
        <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
        <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
        <line x1="1" y1="12" x2="3" y2="12"/>
        <line x1="21" y1="12" x2="23" y2="12"/>
        <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
        <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
      </svg>
    </button>
  </header>
</template>

<script setup>
import { computed } from 'vue'
import { baseName } from '@/utils/helpers.ts'

const props = defineProps({
    projectRoot: String,
    theme: String,
})
const emit = defineEmits(['toggleTheme', 'openProjectDialog'])

const projectName = computed(() => {
    if (!props.projectRoot) return '选择项目'
    return baseName(props.projectRoot) || props.projectRoot
})
</script>

<style scoped>
.header-logo {
    width: 28px;
    height: 28px;
    border-radius: 6px;
    flex-shrink: 0;
}

.project-switch-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    border: 1px solid var(--border-color);
    background: var(--bg-tertiary);
    cursor: pointer;
    color: var(--text-primary);
    border-radius: var(--radius-sm);
    font-size: 13px;
    font-weight: 500;
    max-width: 200px;
    transition: background 0.15s, border-color 0.15s;
    flex-shrink: 0;
}

.project-switch-btn:hover {
    background: var(--bg-primary);
    border-color: var(--accent-color);
    color: var(--accent-color);
}

.project-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.theme-toggle {
    padding: 8px;
    border: none;
    background: transparent;
    cursor: pointer;
    color: var(--text-primary);
    border-radius: var(--radius-sm);
    transition: background 0.15s;
    flex-shrink: 0;
    margin-left: auto;
}

.theme-toggle:hover {
    background: var(--bg-tertiary);
}

.theme-toggle svg {
    width: 20px;
    height: 20px;
}
</style>
