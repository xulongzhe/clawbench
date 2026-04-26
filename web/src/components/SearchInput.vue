<template>
  <div class="search-pill" :class="{ focused }">
    <svg class="search-pill-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
    </svg>
    <input
      ref="inputRef"
      type="text"
      :value="modelValue"
      :placeholder="placeholder"
      @input="$emit('update:modelValue', $event.target.value)"
      @focus="focused = true"
      @blur="focused = false"
      @keydown.enter="$emit('enter')"
      @dblclick="$emit('dblclick')"
    />
    <button v-if="modelValue" class="search-pill-clear" @click="$emit('update:modelValue', '')" title="清除">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="12" height="12">
        <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
      </svg>
    </button>
  </div>
</template>

<script setup>
import { ref } from 'vue'

defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '搜索...' },
})

defineEmits(['update:modelValue', 'enter', 'dblclick'])

const inputRef = ref(null)
const focused = ref(false)

function focus() {
  inputRef.value?.focus()
}

defineExpose({ focus, inputRef })
</script>

<style scoped>
.search-pill {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 999px;
  padding: 5px 12px;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.search-pill.focused {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(74, 144, 217, 0.12);
}

.search-pill-icon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  color: var(--text-muted);
}

.search-pill input {
  flex: 1;
  min-width: 0;
  border: none;
  background: none;
  outline: none;
  font-size: 13px;
  color: var(--text-primary);
  padding: 0;
  line-height: 1.4;
}

.search-pill input::placeholder {
  color: var(--text-muted);
}

.search-pill-clear {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border: none;
  background: var(--bg-tertiary);
  border-radius: 50%;
  cursor: pointer;
  color: var(--text-muted);
  flex-shrink: 0;
  padding: 0;
  transition: background 0.15s, color 0.15s;
}

.search-pill-clear:hover {
  background: var(--accent-color);
  color: #fff;
}
</style>
