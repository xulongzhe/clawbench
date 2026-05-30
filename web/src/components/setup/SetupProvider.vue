<template>
  <div class="setup-step setup-provider">
    <h3 class="step-title">{{ t('setup.selectProvider') }}</h3>

    <!-- Recommended providers -->
    <div v-if="recommended.length" class="provider-section">
      <p class="provider-section-label">{{ t('setup.recommended') }}</p>
      <div class="provider-grid">
        <button
          v-for="p in recommended"
          :key="p.id"
          class="provider-card provider-card--recommended"
          :class="{ 'provider-card--selected': modelValue === p.id }"
          @click="selectProvider(p.id)"
        >
          <span class="provider-name">{{ p.name }}</span>
          <span class="provider-env">{{ p.envVar }}</span>
        </button>
      </div>
    </div>

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
    <div class="provider-section">
      <p class="provider-section-label">{{ t('setup.allProviders') }}</p>
      <div class="provider-list">
        <button
          v-for="p in filteredProviders"
          :key="p.id"
          class="provider-item"
          :class="{ 'provider-item--selected': modelValue === p.id }"
          @click="selectProvider(p.id)"
        >
          <span class="provider-name">{{ p.name }}</span>
          <span class="provider-env">{{ p.envVar }}</span>
          <svg v-if="modelValue === p.id" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
        </button>
      </div>
    </div>

    <!-- Custom URL option -->
    <button
      class="provider-item provider-item--custom"
      :class="{ 'provider-item--selected': modelValue === '_custom' }"
      @click="selectProvider('_custom')"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
        <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
        <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
      </svg>
      <span class="provider-name">{{ t('setup.customUrl') }}</span>
      <svg v-if="modelValue === '_custom'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <polyline points="20 6 9 17 4 12"/>
      </svg>
    </button>

    <!-- Navigation -->
    <div class="step-nav">
      <button class="setup-btn-secondary" @click="$emit('back')">{{ t('setup.back') }}</button>
      <button class="setup-btn-primary" :disabled="!modelValue" @click="$emit('next')">
        {{ t('setup.next') }}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
          <path d="M5 12h14M12 5l7 7-7 7"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { recommendedProviders } from '@/composables/useSetup'

const props = defineProps<{
  modelValue: string
  providers: { id: string; name: string; envVar: string }[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  next: []
  back: []
}>()

const { t } = useI18n()
const searchQuery = ref('')

const recommended = computed(() =>
  props.providers.filter(p => recommendedProviders.includes(p.id))
)

const otherProviders = computed(() =>
  props.providers.filter(p => !recommendedProviders.includes(p.id))
)

const filteredProviders = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  const list = otherProviders.value
  if (!q) return list
  return list.filter(p =>
    p.name.toLowerCase().includes(q) ||
    p.id.toLowerCase().includes(q) ||
    p.envVar.toLowerCase().includes(q)
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
  gap: 16px;
}

.step-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 4px;
}

.provider-section-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0 0 8px;
}

.provider-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.provider-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 14px 8px;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-secondary);
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
}

.provider-card:hover {
  border-color: var(--accent-color);
}

.provider-card--selected {
  border-color: var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 10%, var(--bg-secondary));
}

.provider-card .provider-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.provider-card .provider-env {
  font-size: 10px;
  color: var(--text-muted);
}

.provider-search-wrap {
  position: relative;
}

.provider-search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 16px;
  height: 16px;
  color: var(--text-muted);
  pointer-events: none;
}

.provider-search {
  width: 100%;
  padding: 10px 12px 10px 36px;
  border: 1.5px solid var(--border-color);
  border-radius: var(--radius-md, 10px);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
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
  gap: 4px;
  max-height: 240px;
  overflow-y: auto;
}

.provider-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm, 6px);
  background: transparent;
  color: var(--text-primary);
  font-size: 14px;
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

.provider-item .provider-env {
  font-size: 11px;
  color: var(--text-muted);
}

.provider-item--custom {
  margin-top: 4px;
  border: 1.5px dashed var(--border-color);
  border-radius: var(--radius-md, 10px);
  padding: 12px 14px;
}
</style>
