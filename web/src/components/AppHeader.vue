<template>
  <header class="header">
    <img class="header-logo" src="/logo.png" alt="ClawBench">

    <button class="project-switch-btn" @click="emit('openProjectDialog')" title="切换项目">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16">
        <rect x="3" y="3" width="7" height="7" rx="1.5"/>
        <rect x="14" y="3" width="7" height="7" rx="1.5"/>
        <rect x="3" y="14" width="7" height="7" rx="1.5"/>
        <rect x="14" y="14" width="7" height="7" rx="1.5"/>
      </svg>
      <span class="project-name">{{ projectName }}</span>
      <svg class="switch-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="12" height="12">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
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
    gap: 8px;
    padding: 5px 8px 5px 10px;
    border: 1px solid var(--border-color);
    background: var(--bg-secondary);
    cursor: pointer;
    color: var(--text-primary);
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    max-width: 220px;
    transition: background 0.15s, border-color 0.15s, box-shadow 0.15s;
    flex-shrink: 0;
    line-height: 1;
}

.project-switch-btn:hover {
    background: var(--bg-primary);
    border-color: var(--accent-color);
    box-shadow: 0 0 0 1px var(--accent-color);
}

.project-switch-btn:active {
    transform: scale(0.97);
}

.project-switch-btn svg:first-child {
    color: var(--accent-color);
    flex-shrink: 0;
}

.switch-chevron {
    color: var(--text-muted);
    margin-left: -2px;
    transition: transform 0.2s;
}

.project-switch-btn:hover .switch-chevron {
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
