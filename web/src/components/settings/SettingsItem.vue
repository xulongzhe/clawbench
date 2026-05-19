<template>
  <div class="settings-item" :class="{ 'settings-item--disabled': disabled }" @click="handleClick">
    <div class="settings-item__left">
      <span class="settings-item__label">{{ label }}</span>
      <span v-if="needsRestart" class="settings-item__badge">{{ t('settings.needsRestart') }}</span>
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
        <span class="settings-item__arrow" :class="{ 'settings-item__arrow--open': editing }">›</span>
      </template>
      <template v-else-if="type === 'action'">
        <span class="settings-item__arrow">›</span>
      </template>
      <template v-else-if="type === 'info'">
        <span class="settings-item__value">{{ displayValue }}</span>
      </template>
    </div>
  </div>
  <!-- Inline editor -->
  <div v-if="editing" class="settings-item__editor" @click.stop>
    <!-- Select editor: radio-style option list -->
    <template v-if="type === 'select'">
      <div
        v-for="opt in options"
        :key="opt.value"
        class="settings-item__option"
        :class="{ 'settings-item__option--active': editValue === opt.value }"
        @click="selectOption(opt.value)"
      >
        <span class="settings-item__option-label">{{ opt.label }}</span>
        <span v-if="editValue === opt.value" class="settings-item__option-check">✓</span>
      </div>
    </template>
    <!-- Number editor -->
    <template v-else-if="type === 'number'">
      <div class="settings-item__input-row">
        <input
          type="number"
          class="settings-item__number-input"
          :value="editValue"
          :min="min"
          :max="max"
          :step="step"
          @input="editValue = ($event.target as HTMLInputElement).value"
          @keydown.enter="confirmEdit"
        />
        <button class="settings-item__editor-confirm" @click="confirmEdit">{{ t('common.ok') }}</button>
      </div>
    </template>
    <!-- Text editor -->
    <template v-else-if="type === 'text'">
      <div class="settings-item__input-row">
        <input
          type="text"
          class="settings-item__text-input"
          :value="editValue"
          :placeholder="placeholder"
          @input="editValue = ($event.target as HTMLInputElement).value"
          @keydown.enter="confirmEdit"
        />
        <button class="settings-item__editor-confirm" @click="confirmEdit">{{ t('common.ok') }}</button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface Props {
  label: string
  type: 'switch' | 'select' | 'number' | 'text' | 'slider' | 'action' | 'info'
  modelValue?: any
  options?: { label: string; value: any }[]
  min?: number
  max?: number
  step?: number
  placeholder?: string
  needsRestart?: boolean
  disabled?: boolean
  forceClose?: boolean
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
  forceClose: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: any]
  click: []
  editToggle: [open: boolean]
}>()

const editing = ref(false)
const editValue = ref<any>(null)

// Close editor when parent forces close (another editor opened)
watch(() => props.forceClose, (val) => {
  if (val && editing.value) {
    editing.value = false
    emit('editToggle', false)
  }
})

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
  if (props.type === 'switch' || props.type === 'slider' || props.type === 'info') return
  if (props.type === 'action') {
    emit('click')
    return
  }
  // select / number / text: toggle inline editor
  editing.value = !editing.value
  if (editing.value) {
    editValue.value = props.modelValue
  }
  emit('editToggle', editing.value)
}

function selectOption(value: any) {
  editValue.value = value
  emit('update:modelValue', value)
  editing.value = false
  emit('editToggle', false)
}

function confirmEdit() {
  if (props.type === 'number') {
    const num = Number(editValue.value)
    if (!isNaN(num)) {
      emit('update:modelValue', num)
    }
  } else {
    emit('update:modelValue', editValue.value)
  }
  editing.value = false
  emit('editToggle', false)
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
  background: var(--bg-primary);
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
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.settings-item__badge {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
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
  color: var(--text-secondary);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.settings-item__arrow {
  font-size: 20px;
  color: var(--text-muted);
  line-height: 1;
  transition: transform 0.2s ease;
}

.settings-item__arrow--open {
  transform: rotate(90deg);
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
  background: var(--bg-tertiary);
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
  background: var(--color-green);
}

.settings-item__switch-input:checked + .settings-item__switch-track::after {
  transform: translateX(20px);
}

/* Slider */
.settings-item__slider {
  width: 120px;
  cursor: pointer;
  accent-color: var(--accent-color);
}

/* ── Inline Editor ── */
.settings-item__editor {
  background: var(--bg-primary);
  border-top: 0.5px solid var(--border-color);
  padding: 4px 0;
}

/* Select options */
.settings-item__option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  cursor: pointer;
  min-height: 40px;
}

@media (hover: hover) {
  .settings-item__option:hover {
    background: var(--bg-secondary);
  }
}

.settings-item__option:active {
  background: var(--bg-tertiary);
}

.settings-item__option--active {
  background: var(--bg-secondary);
}

.settings-item__option-label {
  font-size: 14px;
  color: var(--text-primary);
}

.settings-item__option-check {
  font-size: 15px;
  color: var(--accent-color);
  font-weight: 600;
}

/* Input row (number / text) */
.settings-item__input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
}

.settings-item__number-input,
.settings-item__text-input {
  flex: 1;
  min-width: 0;
  padding: 8px 12px;
  font-size: 14px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-secondary);
  color: var(--text-primary);
  outline: none;
}

.settings-item__number-input:focus,
.settings-item__text-input:focus {
  border-color: var(--accent-color);
}

.settings-item__editor-confirm {
  flex-shrink: 0;
  padding: 8px 16px;
  border: none;
  border-radius: 8px;
  background: var(--accent-color);
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}

@media (hover: hover) {
  .settings-item__editor-confirm:hover {
    background: var(--accent-hover);
  }
}

.settings-item__editor-confirm:active {
  background: var(--accent-hover);
}
</style>
