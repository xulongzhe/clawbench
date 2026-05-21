<template>
  <div class="settings-category">
    <SettingsItem
      v-for="item in items"
      :key="item.key"
      :label="item.label"
      :description="item.description"
      :type="item.type"
      :model-value="getItemValue(item)"
      :options="item.options"
      :min="item.min"
      :max="item.max"
      :step="item.step"
      :needs-restart="item.needsRestart"
      :force-close="openEditorKey !== null && openEditorKey !== item.key"
      @update:model-value="(v: any) => handleUpdate(item, v)"
      @click="handleClick(item)"
      @edit-toggle="(open: boolean) => handleEditToggle(item.key, open)"
      @discard="handleDiscard"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import SettingsItem from './SettingsItem.vue'
import { useSettingsConfig } from '@/composables/useSettingsConfig'
import { useAgents } from '@/composables/useAgents'
import { useToast } from '@/composables/useToast'
import { useAppMode } from '@/composables/useAppMode'
import { categoryItems, type ItemSpec } from './settingsFieldMap'

const props = defineProps<{
  categoryId: string
}>()

const emit = defineEmits<{
  navigate: [categoryId: string]
  restartNeeded: [changedFields: string[]]
}>()

const { t } = useI18n()
const toast = useToast()
const { localConfig, serverConfig, setLocalConfig, getServerValueWithDefault, setServerValue, patchAgentPref, getAgentModelPref, getAgentThinkingPref } = useSettingsConfig()
const { agents, loadAgents, getAgentModels, getAgentThinkingEffortLevels, hasThinkingEffortLevels, getDefaultModelId } = useAgents()
const { isAppMode } = useAppMode()

const openEditorKey = ref<string | null>(null)

// Load agents when this category is shown
watch(() => props.categoryId, (id) => {
  if (id === 'chat' || id === 'agents') loadAgents(true)
}, { immediate: true })

function resolveConfigValue(key: string): any {
  if (key in localConfig) return localConfig[key]
  return getServerValueWithDefault(key)
}

function isDependsOnMet(dependsOn: ItemSpec['dependsOn']): boolean {
  if (!dependsOn) return true
  const currentValue = resolveConfigValue(dependsOn.key)
  if ('value' in dependsOn) return currentValue === dependsOn.value
  return dependsOn.values!.includes(currentValue)
}

// categoryItems is imported from the shared settingsFieldMap module
// (single source of truth — also used by SettingsRestartDialog for field translation)

// Resolve i18n labels at runtime, and dynamically inject agent options
const items = computed(() => {
  // For the 'agents' category, dynamically build items from the agent list
  if (props.categoryId === 'agents') {
    const result: any[] = []

    for (const agent of agents.value) {
      // Agent group header — just shows the agent icon + name as a label
      result.push({
        key: `agent-header-${agent.id}`,
        label: `${agent.icon} ${agent.name}`,
        labelKey: '',
        type: 'info' as const,
        source: 'local' as const,
        modelValue: '',
      })
      // Model preference (only if agent has multiple models)
      const models = getAgentModels(agent.id)
      if (models.length > 1) {
        const savedModel = getAgentModelPref(agent.id)
        const currentModel = savedModel || getDefaultModelId(agent.id)
        result.push({
          key: `agent-model-${agent.id}`,
          label: t('settings.items.agentModel'),
          labelKey: 'settings.items.agentModel',
          type: 'select' as const,
          source: 'local' as const,
          modelValue: currentModel,
          options: models.map((m: any) => ({
            label: m.name || m.id,
            value: m.id,
          })),
        })
      }
      // Thinking effort preference (only if agent supports it)
      if (hasThinkingEffortLevels(agent.id)) {
        const levels = getAgentThinkingEffortLevels(agent.id)
        const savedThinking = getAgentThinkingPref(agent.id)
        const currentThinking = savedThinking || agent.preferredThinkingEffort || agent.thinkingEffort || ''
        result.push({
          key: `agent-thinking-${agent.id}`,
          label: t('settings.items.agentThinking'),
          labelKey: 'settings.items.agentThinking',
          type: 'select' as const,
          source: 'local' as const,
          modelValue: currentThinking,
          options: levels.map((level: string) => ({
            label: level,
            value: level,
          })),
        })
      }
    }
    return result
  }

  const raw = categoryItems[props.categoryId] ?? []
  // Filter by dependsOn and inject header pseudo-items
  const expanded: any[] = []
  for (const item of raw) {
    if (!isDependsOnMet(item.dependsOn)) continue
    // Hide appVersion row when not in Android App mode (no AndroidNative bridge)
    if (item.key === 'appVersion' && !isAppMode.value) continue
    if (item.sectionHeader) {
      expanded.push({
        key: `header-${item.key}`,
        label: t(item.sectionHeader),
        labelKey: item.sectionHeader,
        type: 'header' as const,
        source: 'local' as const,
      })
    }
    expanded.push(item)
  }
  return expanded.map(item => {
    // Dynamically build options for default_agent from the agents list
    let resolvedOptions = item.options
    if (item.key === 'default_agent') {
      resolvedOptions = agents.value.map(a => ({
        labelKey: '',
        value: a.id,
        label: `${a.icon} ${a.name}`,
      }))
    }

    return {
      ...item,
      label: item.label || t(item.labelKey),
      description: item.descriptionKey ? t(item.descriptionKey) : '',
      options: resolvedOptions?.map(opt => ({
        ...opt,
        label: opt.label || resolveOptionLabel(item.key, opt),
      })),
    }
  })
})

function resolveOptionLabel(_itemKey: string, opt: { labelKey: string; value: any }): string {
  // All select options should have labelKey set to the i18n key
  if (opt.labelKey) return t(opt.labelKey)
  return String(opt.value)
}

function getItemValue(item: any): any {
  // Header pseudo-items have no value
  if (item.type === 'header') return undefined
  // Agent model/thinking prefs are handled specially
  if (item.key?.startsWith('agent-model-')) {
    return item.modelValue
  }
  if (item.key?.startsWith('agent-thinking-')) {
    return item.modelValue
  }
  if (item.key?.startsWith('agent-header-')) {
    return ''
  }
  // Version info items
  if (item.key === 'serverVersion') {
    return serverConfig.value?.version ?? '-'
  }
  if (item.key === 'appVersion') {
    try {
      const native = (window as any).AndroidNative
      if (native?.getAppVersion) return native.getAppVersion() ?? '-'
    } catch { /* not in app mode */ }
    return '-'
  }
  if (item.source === 'local') {
    return localConfig[item.key]
  }
  return getServerValueWithDefault(item.key)
}

async function handleUpdate(item: any, value: any) {
  // Agent model preference
  if (item.key?.startsWith('agent-model-')) {
    const agentId = item.key.replace('agent-model-', '')
    try { await patchAgentPref(agentId, 'preferred_model', value) } catch { toast.show(t('settings.saveFailed'), { icon: '⚠️', type: 'error', duration: 3000 }) }
    return
  }
  // Agent thinking preference
  if (item.key?.startsWith('agent-thinking-')) {
    const agentId = item.key.replace('agent-thinking-', '')
    try { await patchAgentPref(agentId, 'preferred_thinking_effort', value) } catch { toast.show(t('settings.saveFailed'), { icon: '⚠️', type: 'error', duration: 3000 }) }
    return
  }
  // Password type: skip if empty or still masked (contains bullet chars)
  if (item.type === 'password') {
    if (!value || value.includes('•')) return
  }
  if (item.source === 'local') {
    setLocalConfig(item.key, value)
    return
  }
  // Server config: auto-save immediately
  try {
    const result = await setServerValue(item.key, value)
    if (result.needsRestart && result.changedColdFields.length > 0) {
      emit('restartNeeded', result.changedColdFields)
    }
  } catch {
    toast.show(t('settings.saveFailed'), { icon: '⚠️', type: 'error', duration: 3000 })
  }
}

function handleClick(item: any) {
  if (item.key === 'restart') {
    emit('restartNeeded', [])
  }
  if (item.key === 'reconfigureServer') {
    try {
      ;(window as any).AndroidNative?.showServerDialog?.()
    } catch { /* not in app mode */ }
  }
}

function handleEditToggle(key: string, open: boolean) {
  if (open) {
    openEditorKey.value = key
  } else if (openEditorKey.value === key) {
    openEditorKey.value = null
  }
}

function handleDiscard() {
  toast.show(t('settings.passwordDiscarded'), { icon: 'ℹ️', type: 'info', duration: 3000 })
}
</script>

<style scoped>
.settings-category {
  padding: 8px 0;
  background: var(--bg-secondary);
  min-height: 100%;
}
</style>
