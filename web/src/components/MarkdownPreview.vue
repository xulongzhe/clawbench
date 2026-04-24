<template>
  <div class="markdown-preview">
    <!-- Rendered markdown -->
    <div v-if="viewMode === 'rendered'" class="markdown-body" ref="bodyRef" v-html="renderedHtml" @click="handleClick" />

    <!-- Raw markdown -->
    <CodePreview
      v-else
      :content="file.content"
      language="markdown"
      :file-path="file.path"
      :editable="true"
      @content-change="file.content = $event"
    />
  </div>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'
import CodePreview from './CodePreview.vue'
import { useMarkdownRenderer } from '@/composables/useMarkdownRenderer.ts'
import { useDoubleClickCopy } from '@/composables/useDoubleClickCopy.ts'
import { store } from '@/stores/app.ts'

const props = defineProps({
    file: Object,
    viewMode: String,
})

const emit = defineEmits(['openFile', 'openDir'])

const renderedHtml = ref('')
const bodyRef = ref(null)
let currentRenderId = 0
const { handleDblClick } = useDoubleClickCopy()
const { renderMarkdown, renderMermaidInElement } = useMarkdownRenderer()

// 处理相对路径链接点击
async function handleOpenFile(href) {
    const currentDir = props.file?.path ? props.file.path.split('/').slice(0, -1).join('/') : ''
    let resolvedPath = href
    
    // 解析相对路径
    if (currentDir) {
        const parts = (currentDir + '/' + href).split('/')
        const normalized = []
        for (const part of parts) {
            if (part === '.' || part === '') continue
            if (part === '..') { normalized.pop(); continue }
            normalized.push(part)
        }
        resolvedPath = normalized.join('/')
    }
    
    // 检查路径是否为目录
    try {
        const resp = await fetch(`/api/dir?path=${encodeURIComponent(resolvedPath)}`)
        if (resp.ok) {
            // 是目录，导航到该目录并打开文件管理器
            await store.navigateToDir(resolvedPath)
            window.dispatchEvent(new CustomEvent('open-sidebar'))
            return
        }
    } catch {
        // 忽略，回退到打开文件
    }
    
    // 通过 store 打开文件
    store.selectFile(resolvedPath)
}

function handleClick(event) {
    handleDblClick(event, handleOpenFile)
}

function fixLocalImagePaths(html) {
    const currentDir = props.file?.path ? props.file.path.split('/').slice(0, -1).join('/') : ''
    return html.replace(/<img\s+([^>]*src=[^>]*)>/gi, (match, attrs) => {
        const srcMatch = attrs.match(/src="([^"]*)"/)
        if (!srcMatch) return match
        const src = srcMatch[1]
        if (/^(https?:|\/\/|^\/)/i.test(src)) return match
        let resolved = currentDir ? currentDir + '/' + src : src
        const parts = resolved.split('/')
        const normalized = []
        for (const part of parts) {
            if (part === '.' || part === '') continue
            if (part === '..') { normalized.pop(); continue }
            normalized.push(part)
        }
        return match.replace(`src="${src}"`, `src="/api/local-file/${normalized.join('/')}"`)
    })
}

async function doRender(f) {
    const renderId = ++currentRenderId
    renderedHtml.value = renderMarkdown(f.content, {
        sanitize: false, // MarkdownPreview不需要净化，因为是受信任的文件内容
        fixImagePaths: fixLocalImagePaths
    })
    if (renderId !== currentRenderId) return
    await nextTick()
    if (renderId !== currentRenderId) return
    const el = bodyRef.value
    if (!el) return
    // 【注意】KaTeX 已在 renderMarkdown 内的 renderKatexInString 完成字符串级渲染，
    // 这里只做 Mermaid 的 DOM 级渲染（Mermaid 是整体节点替换，与 v-html 不冲突）
    await renderMermaidInElement(el, 'md-preview')
}

watch(() => props.file, (f, oldF) => {
    if (!f || f.error) return
    if (!f.content) return
    doRender(f)
}, { immediate: true })

watch(() => props.file?.content, (content) => {
    if (!content) return
    const f = props.file
    if (!f || f.error) return
    doRender(f)
})

// 当 viewMode 切换回 rendered 时，DOM 会被 v-if 重建，
// Mermaid 的 SVG 渲染结果丢失，需要重新执行 DOM 级渲染
watch(() => props.viewMode, async (mode) => {
    if (mode !== 'rendered') return
    const f = props.file
    if (!f || f.error || !f.content) return
    // renderedHtml 已有值，只需等 DOM 挂载后重新渲染 Mermaid
    await nextTick()
    const el = bodyRef.value
    if (!el) return
    await renderMermaidInElement(el, 'md-preview')
})

</script>

<style scoped>
/* scoped only to keep sticky reference local */
</style>