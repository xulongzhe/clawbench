<template>
  <div class="setup-step setup-provider">
    <h3 class="step-title">{{ t('setup.selectProvider') }}</h3>

    <!-- Search -->
    <div class="provider-search-wrap">
      <svg class="provider-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>
      </svg>
      <input
        v-model="searchQuery"
        class="provider-search"
        type="text"
        :placeholder="t('setup.searchProvider')"
      />
    </div>

    <!-- Full provider list -->
    <div class="provider-list">
      <button
        v-for="p in filteredProviders"
        :key="p.id"
        class="provider-item"
        :class="{ 'provider-item--selected': modelValue === p.id }"
        @click="selectProvider(p.id)"
      >
        <span class="provider-name">{{ p.name }}</span>
        <svg v-if="modelValue === p.id" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <polyline points="20 6 9 17 4 12"/>
        </svg>
      </button>
    </div>

    <!-- Custom URL option -->
    <button
      class="provider-item provider-item--custom"
      :class="{ 'provider-item--selected': modelValue === '_custom' }"
      @click="selectProvider('_custom')"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="15" height="15">
        <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
        <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
      </svg>
      <span class="provider-name">{{ t('setup.customUrl') }}</span>
      <svg v-if="modelValue === '_custom'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
        <polyline points="20 6 9 17 4 12"/>
      </svg>
    </button>

    <!-- Navigation -->
    <div class="step-nav">
      <button class="setup-btn-secondary" @click="$emit('back')">{{ t('setup.back') }}</button>
      <button class="setup-btn-primary" :disabled="!modelValue" @click="$emit('next')">
        {{ t('setup.next') }}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <path d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  modelValue: string
  providers: { id: string; name: string; envVar: string; apiFormat: string }[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  next: []
  back: []
}>()

const { t } = useI18n()
const searchQuery = ref('')

const filteredProviders = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  const list = props.providers
  if (!q) return list
  return list.filter(p =>
    p.name.toLowerCase().includes(q) ||
    p.id.toLowerCase().includes(q)
  )
})

function selectProvider(id: string) {
  emit('update:modelValue', id)
}
</script>

<style scoped>
.setup-provider {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.provider-search-wrap {
  position: relative;
}

.provider-search-icon {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  width: 14px;
  height: 14px;
  color: var(--text-muted);
  pointer-events: none;
}

.provider-search {
  width: 100%;
  padding: 8px 10px 8px 30px;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-sm, 6px);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.2s;
}

.provider-search:focus {
  border-color: var(--accent-color);
}

.provider-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 280px;
  overflow-y: auto;
}

.provider-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm, 6px);
  background: transparent;
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s;
  width: 100%;
  text-align: left;
}

.provider-item:hover {
  background: var(--bg-tertiary);
}

.provider-item--selected {
  background: color-mix(in srgb, var(--accent-color) 10%, transparent);
  border-color: color-mix(in srgb, var(--accent-color) 30%, transparent);
}

.provider-item .provider-name {
  flex: 1;
  font-weight: 500;
}

.provider-item--custom {
  margin-top: 2px;
  border: 1.5px dashed var(--border-color);
  border-radius: var(--radius-sm, 6px);
  padding: 7px 10px;
}
</style>
