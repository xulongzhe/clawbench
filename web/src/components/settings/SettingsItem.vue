<template>
  <div class="settings-item" :class="{ 'settings-item--disabled': disabled }" @click="handleClick">
    <div class="settings-item__left">
      <span class="settings-item__label">{{ label }}</span>
      <span v-if="needsRestart" class="settings-item__badge">需重启</span>
    </div>
    <div class="settings-item__right">
      <template v-if="type === 'switch'">
        <label class="settings-item__switch">
          <input
            type="checkbox"
            class="settings-item__switch-input"
            :checked="!!modelValue"
            :disabled="disabled"
            @change="onSwitchChange"
            @click.stop
          />
          <span class="settings-item__switch-track"></span>
        </label>
      </template>
      <template v-else-if="type === 'slider'">
        <input
          type="range"
          class="settings-item__slider"
          :value="modelValue"
          :min="min"
          :max="max"
          :step="step"
          :disabled="disabled"
          @input="onSliderInput"
          @click.stop
        />
      </template>
      <template v-else-if="type === 'select' || type === 'number' || type === 'text'">
        <span class="settings-item__value">{{ displayValue }}</span>
        <span class="settings-item__arrow">›</span>
      </template>
      <template v-else-if="type === 'action'">
        <span class="settings-item__arrow">›</span>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  label: string
  type: 'switch' | 'select' | 'number' | 'text' | 'slider' | 'action'
  modelValue?: any
  options?: { label: string; value: any }[]
  min?: number
  max?: number
  step?: number
  placeholder?: string
  needsRestart?: boolean
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: undefined,
  options: undefined,
  min: undefined,
  max: undefined,
  step: undefined,
  placeholder: '',
  needsRestart: false,
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: any]
  click: []
}>()

const displayValue = computed(() => {
  if (props.type === 'select' && props.options?.length) {
    const opt = props.options.find(o => o.value === props.modelValue)
    return opt?.label ?? props.modelValue ?? props.placeholder
  }
  if (props.modelValue !== undefined && props.modelValue !== '') {
    return String(props.modelValue)
  }
  return props.placeholder
})

function onSwitchChange(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  emit('update:modelValue', checked)
}

function onSliderInput(e: Event) {
  const value = Number((e.target as HTMLInputElement).value)
  emit('update:modelValue', value)
}

function handleClick() {
  if (props.type !== 'switch' && props.type !== 'slider') {
    emit('click')
  }
}
</script>

<style scoped>
.settings-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  height: 48px;
  cursor: pointer;
  gap: 12px;
  background: #fff;
  position: relative;
}

.settings-item--disabled {
  opacity: 0.5;
  pointer-events: none;
}

.settings-item__left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 1;
  min-width: 0;
}

.settings-item__label {
  font-size: 15px;
  color: var(--text-primary, #1a1a1a);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.settings-item__badge {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: transparent;
  color: #8e8e93;
  white-space: nowrap;
  flex-shrink: 0;
}

.settings-item__right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.settings-item__value {
  font-size: 14px;
  color: var(--text-secondary, #8e8e93);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.settings-item__arrow {
  font-size: 20px;
  color: var(--text-tertiary, #c7c7cc);
  line-height: 1;
}

/* iOS-style switch toggle */
.settings-item__switch {
  position: relative;
  display: inline-block;
  width: 51px;
  height: 31px;
  cursor: pointer;
}

.settings-item__switch-input {
  opacity: 0;
  width: 0;
  height: 0;
  position: absolute;
}

.settings-item__switch-track {
  position: absolute;
  inset: 0;
  border-radius: 15.5px;
  background: #e9e9ea;
  transition: background 0.2s ease;
}

.settings-item__switch-track::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 27px;
  height: 27px;
  border-radius: 50%;
  background: #fff;
  transition: transform 0.2s ease;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
}

.settings-item__switch-input:checked + .settings-item__switch-track {
  background: #34c759;
}

.settings-item__switch-input:checked + .settings-item__switch-track::after {
  transform: translateX(20px);
}

/* Slider */
.settings-item__slider {
  width: 120px;
  cursor: pointer;
  accent-color: #007aff;
}

/* Dark mode */
[data-theme="dark"] .settings-item {
  background: #1c1c1e;
}

[data-theme="dark"] .settings-item__label {
  color: var(--text-primary, #e0e0e0);
}

[data-theme="dark"] .settings-item__badge {
  color: #8e8e93;
}

[data-theme="dark"] .settings-item__value {
  color: var(--text-secondary, #8e8e93);
}

[data-theme="dark"] .settings-item__arrow {
  color: var(--text-tertiary, #48484a);
}

[data-theme="dark"] .settings-item__switch-track {
  background: #39393d;
}

[data-theme="dark"] .settings-item__switch-input:checked + .settings-item__switch-track {
  background: #34c759;
}

[data-theme="dark"] .settings-item__switch-track::after {
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}
</style>
