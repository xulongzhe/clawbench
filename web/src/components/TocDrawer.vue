<template>
  <BottomSheet :open="open" title="目录" @close="$emit('close')">
    <template #header>
      <svg class="bs-header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
        <line x1="8" y1="6" x2="21" y2="6"/>
        <line x1="8" y1="12" x2="21" y2="12"/>
        <line x1="8" y1="18" x2="21" y2="18"/>
        <line x1="3" y1="6" x2="3.01" y2="6"/>
        <line x1="3" y1="12" x2="3.01" y2="12"/>
        <line x1="3" y1="18" x2="3.01" y2="18"/>
      </svg>
      <span class="bs-header-title">目录</span>
      <div v-if="file?.path" class="bs-header-description">
        <span class="bs-header-description-inner" :title="file.path">
          {{ file.path }}
        </span>
      </div>
      <button class="bs-close" @click.stop="$emit('close')" title="关闭">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
          <line x1="18" y1="6" x2="6" y2="18"/>
          <line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    </template>

    <div class="toc-body">
      <div class="toc-search">
        <input
          type="text"
          v-model="searchQuery"
          placeholder="搜索目录..."
          @input="handleSearch"
          @dblclick="clearSearch"
        />
      </div>
      <div v-if="filteredToc.length === 0" class="toc-empty">{{ searchQuery ? '无匹配结果' : '无标题' }}</div>
      <a
        v-for="item in filteredToc"
        :key="item.id"
        class="toc-item"
        :class="{ active: activeId === item.id }"
        :data-level="item.level"
        @click.prevent="scrollTo(item)"
      >{{ item.text }}</a>
    </div>

  </BottomSheet>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'
import BottomSheet from './BottomSheet.vue'
import { extractToc, getFileType } from '@/utils/helpers.ts'

const props = defineProps({
    file: Object,
    open: Boolean,
})
const emit = defineEmits(['close'])

const toc = ref([])
const activeId = ref('')
const isCode = ref(false)
const searchQuery = ref('')
const filteredToc = ref([])

watch(() => props.file, (file) => {
    if (!file?.content) {
        toc.value = []
        filteredToc.value = []
        isCode.value = false
        return
    }
    const lang = getFileType(file.name)?.lang || 'plaintext'
    isCode.value = lang !== 'markdown'
    toc.value = extractToc(file.content, lang)
    activeId.value = toc.value[0]?.id || ''
    searchQuery.value = ''
    filteredToc.value = toc.value
}, { immediate: true })

function handleSearch() {
    const query = searchQuery.value.toLowerCase().trim()
    if (!query) {
        filteredToc.value = toc.value
        return
    }
    filteredToc.value = toc.value.filter(item =>
        item.text.toLowerCase().includes(query)
    )
}

function clearSearch() {
    searchQuery.value = ''
    filteredToc.value = toc.value
}

function scrollTo(item) {
    const elById = document.getElementById(item.id)
    if (elById) {
        elById.scrollIntoView({ behavior: 'smooth', block: 'start' })
        activeId.value = item.id
        emit('close')
        return
    }
    if (item.line) {
        const el = document.querySelector(`[data-line="${item.line}"]`)
        if (el) {
            el.scrollIntoView({ behavior: 'smooth', block: 'start' })
            activeId.value = item.id
        }
    }
    emit('close')
}

let observer = null
watch(() => props.open, (val) => {
    if (!val) {
        observer?.disconnect()
        return
    }
    nextTick(() => {
        observer?.disconnect()
        if (isCode.value) {
            observer = new IntersectionObserver((entries) => {
                for (const entry of entries) {
                    if (entry.isIntersecting) {
                        const line = entry.target.getAttribute('data-line')
                        const match = toc.value.find(t => t.line == line)
                        if (match) activeId.value = match.id
                        break
                    }
                }
            }, { rootMargin: '-60px 0px -70% 0px' })
            toc.value.forEach(item => {
                const el = document.querySelector(`[data-line="${item.line}"]`)
                if (el) observer.observe(el)
            })
        } else {
            observer = new IntersectionObserver((entries) => {
                for (const entry of entries) {
                    if (entry.isIntersecting) {
                        activeId.value = entry.target.id
                        break
                    }
                }
            }, { rootMargin: '-60px 0px -70% 0px' })
            toc.value.forEach(item => {
                const el = document.getElementById(item.id)
                if (el) observer.observe(el)
            })
        }
    })
})
</script>

<style scoped>
.toc-header-row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
}

.toc-body {
    flex: 1;
    overflow-y: auto;
    padding: 8px 6px;
    display: flex;
    flex-direction: column;
}

.toc-search {
    position: relative;
    margin-bottom: 8px;
    padding: 0 2px;
}

.toc-search input {
    width: 100%;
    padding: 8px 32px 8px 10px;
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    font-size: 13px;
    background: var(--bg-primary);
    color: var(--text-primary);
    outline: none;
    box-sizing: border-box;
    transition: border-color 0.2s;
}

.toc-search input:focus {
    border-color: var(--accent-color);
}

.toc-search input::placeholder {
    color: var(--text-muted);
}

.toc-empty {
    text-align: center;
    padding: 32px 16px;
    color: var(--text-muted);
    font-size: 13px;
}

.toc-item {
    display: block;
    padding: 6px 8px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: 13px;
    color: var(--text-secondary);
    transition: background 0.15s, color 0.15s;
    border-left: 2px solid transparent;
    white-space: nowrap;
    text-decoration: none;
}
.toc-item:hover { background: var(--bg-tertiary); color: var(--accent-color); }
.toc-item.active { color: var(--accent-color); border-left-color: var(--accent-color); background: var(--bg-tertiary); border-radius: 0; }
.toc-item[data-level="2"] { padding-left: 20px; }
.toc-item[data-level="3"] { padding-left: 32px; }
.toc-item[data-level="4"] { padding-left: 44px; }
.toc-item[data-level="5"] { padding-left: 56px; }
.toc-item[data-level="6"] { padding-left: 68px; }

</style>
