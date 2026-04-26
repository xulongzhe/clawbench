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
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'
import { store } from '@/stores/app.ts'
import { dirName, splitPath } from '@/utils/helpers.ts'

const props = defineProps({
    file: Object,
    viewMode: String,
})

const renderedHtml = ref('')
const bodyRef = ref(null)
let currentRenderId = 0
const { handleDblClick } = useDoubleClickCopy()
const { renderMarkdown, renderMermaidInElement } = useMarkdownRenderer()
const { annotateFilePaths, verifyFilePaths, resolveRelativePath, openFilePath } = useFilePathAnnotation()

function handleClick(event) {
    // Check for file-open button click first
    const btn = (event.target).closest('.chat-file-open-btn')
    if (btn) {
        event.preventDefault()
        event.stopPropagation()
        const filePath = btn.getAttribute('data-file-path')
        if (filePath) {
            openFilePath(filePath)
        }
        return
    }
    // Handle <a> link clicks (relative paths) + double-click copy
    handleDblClick(event, (href) => {
        const currentDir = props.file?.path ? dirName(props.file.path) : ''
        const resolvedPath = resolveRelativePath(href, currentDir)
        openFilePath(resolvedPath)
    })
}

function fixLocalImagePaths(html) {
    const currentDir = props.file?.path ? dirName(props.file.path) : ''
    return html.replace(/<img\s+([^>]*src=[^>]*)>/gi, (match, attrs) => {
        const srcMatch = attrs.match(/src="([^"]*)"/)
        if (!srcMatch) return match
        const src = srcMatch[1]
        if (/^(https?:|\/\/|^\/)/i.test(src)) return match
        let resolved = currentDir ? currentDir + '/' + src : src
        const parts = splitPath(resolved)
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
    let html = renderMarkdown(f.content, {
        sanitize: false, // MarkdownPreview不需要净化，因为是受信任的文件内容
        fixImagePaths: fixLocalImagePaths
    })

    // Annotate file paths with open buttons
    const currentDir = f?.path ? dirName(f.path) : ''
    const { html: annotatedHtml, detectedPaths } = annotateFilePaths(html, {
        projectRoot: store.state.projectRoot,
        baseDir: currentDir
    })
    renderedHtml.value = annotatedHtml

    if (renderId !== currentRenderId) return
    await nextTick()
    if (renderId !== currentRenderId) return
    const el = bodyRef.value
    if (!el) return

    // Verify file existence and hide buttons for non-existent files
    if (detectedPaths.length > 0) {
        const uniquePaths = [...new Set(detectedPaths)]
        verifyFilePaths(uniquePaths, el)
    }

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
.markdown-preview {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}
</style>
