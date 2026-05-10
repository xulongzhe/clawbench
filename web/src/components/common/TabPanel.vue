<template>
  <div
    v-if="everOpened"
    v-show="isActive"
    class="tab-panel"
    :class="{ 'tab-panel-active': isActive }"
  >
    <!-- Header -->
    <div v-if="!noHeader" class="bs-header" @click="handleHeaderClick">
      <slot name="header">
        <span class="bs-title">{{ title }}</span>
      </slot>
    </div>
    <!-- Body -->
    <div class="bs-body">
      <slot />
    </div>
    <!-- Footer slot -->
    <footer v-if="$slots.footer" class="bs-footer">
      <slot name="footer" />
    </footer>
  </div>
</template>

<script setup>
import { ref, watch, computed } from 'vue'

const props = defineProps({
  tabId: {
    type: String,
    required: true,
  },
  activeTab: {
    type: String,
    required: true,
  },
  title: {
    type: String,
    default: '',
  },
  noHeader: Boolean,
})

const emit = defineEmits(['header-click'])

const isActive = computed(() => props.activeTab === props.tabId)
const everOpened = ref(false)

watch(isActive, (val) => {
  if (val) {
    everOpened.value = true
  }
}, { immediate: true })

function handleHeaderClick() {
  emit('header-click')
}
</script>

<style>
.tab-panel {
  position: absolute;
  inset: 0;
  background: var(--bg-secondary, #fff);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  opacity: 0;
  transition: opacity 150ms ease;
  pointer-events: none;
}

.tab-panel-active {
  opacity: 1;
  pointer-events: auto;
}
</style>
