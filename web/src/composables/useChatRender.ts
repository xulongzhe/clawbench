import { ref, reactive, nextTick, watch } from 'vue'
import { marked, DOMPurify } from '@/utils/globals.ts'
import { formatToolInput } from '@/utils/renderToolDetail.ts'
import { renderKatexInString, renderMermaidInElement } from '@/composables/useMarkdownRenderer.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'
import { useCommitHashAnnotation } from '@/composables/useCommitHashAnnotation.ts'
import { useLocalhostAnnotation } from '@/composables/useLocalhostAnnotation.ts'
import { store } from '@/stores/app.ts'
import { apiGet } from '@/utils/api.ts'
import { createTaskBlockStore } from '@/utils/taskBlockStore.ts'
import {
  extractScheduledTaskIds,
  stripScheduledTaskTags,
  detectAskQuestion,
  taskChanged,
  StaticBlockCache,
} from '@/utils/streamPerf.ts'
import {
  rewriteImageUrls,
  convertAudioLinks,
  parseAskQuestionContent,
} from '@/utils/chatRenderUtils.ts'
import {
  parseAssistantContent,
  toolCallSummary,
  hasImagesInContent,
  formatMessageTime,
  formatDetailTime,
  humanizeCron,
  repeatLabel,
  truncate,
} from '@/utils/chatBlocks.ts'

export function useChatRender(options) {
  const { messages, theme, currentSessionId } = options
  const { annotateFilePaths, verifyFilePaths } = useFilePathAnnotation()
  const { annotateCommitHashes, verifyCommitHashes } = useCommitHashAnnotation()
  const { annotateLocalhostUrls } = useLocalhostAnnotation()

  const blockTasks = reactive({})
  const blockAskQuestions = reactive({})
  const expandedTools = ref({})
  let lastRenderedCount = 0

  // ── Task block store for batch fetching (ISS-013) ──
  const taskBlockStore = createTaskBlockStore()

  // Sync taskBlockStore.blocks into blockTasks for template rendering
  watch(() => ({ ...taskBlockStore.blocks }), (storeBlocks) => {
    for (const key of Object.keys(storeBlocks)) {
      blockTasks[key] = storeBlocks[key]
    }
  }, { deep: true })

  // ── StaticBlockCache for non-streaming re-renders ──
  const staticBlockCache = new StaticBlockCache()

  // Re-render when theme changes — clear caches since rendering may differ
  watch(theme, () => {
    staticBlockCache.clear()
    updateRenderedContents(true)
  })

  // Clear caches when session changes
  watch(currentSessionId, () => {
    staticBlockCache.clear()
  })

  // Sync blockTasks with latest task data from store (global polling updates store.state.tasks).
  // Use a tasks Map for O(1) lookup, and taskChanged() for semantic comparison.
  watch(() => store.state.tasks, (tasks) => {
    const keys = Object.keys(blockTasks)
    if (keys.length === 0) return
    // Empty tasks list means all tasks were deleted — mark all blockTasks as deleted
    if (!tasks || tasks.length === 0) {
      for (const key of keys) {
        if (!blockTasks[key].deleted) blockTasks[key].deleted = true
        blockTasks[key].loading = false
      }
      return
    }
    const taskMap = new Map(tasks.map(t => [t.id, t]))
    for (const key of keys) {
      const entry = blockTasks[key]
      if (entry.deleted) continue
      const updated = taskMap.get(entry.taskId)
      if (!updated) {
        entry.deleted = true
        entry.loading = false
      } else if (entry.task && taskChanged(entry.task, updated)) {
        entry.task = updated
      } else if (!entry.task) {
        entry.task = updated
        entry.loading = false
      }
    }
  })

  // Batch-fetch task data using the list API to avoid per-task loading flicker.
  // ISS-013: delegates to taskBlockStore which does NOT mark deleted on network error.
  async function fetchBatchTaskData(taskKeys) {
    await taskBlockStore.fetchBatchData(taskKeys)
    // Sync store blocks into our reactive blockTasks
    for (const key of Object.keys(taskBlockStore.blocks)) {
      blockTasks[key] = taskBlockStore.blocks[key]
    }
  }

  async function refreshTaskData(taskId) {
    for (const key of Object.keys(blockTasks)) {
      if (blockTasks[key].taskId === taskId && !blockTasks[key].deleted) {
        try {
          const data = await apiGet(`/api/tasks/${taskId}`)
          blockTasks[key].task = data
        } catch (err: any) {
          if (err?.message?.includes('404') || err?.message?.toLowerCase().includes('not found')) {
            blockTasks[key].deleted = true
            blockTasks[key].task = null
          }
          // Other errors: leave existing data, don't mark deleted
        }
      }
    }
  }

  /**
   * Render markdown to HTML.
   * When skipEnhancements=true (streaming mode), only marked + DOMPurify + table-wrap runs.
   * When skipEnhancements=false (post-streaming), the full pipeline runs:
   * marked → KaTeX → DOMPurify → table-wrap → img → audio → annotateFilePaths → verifyFilePaths.
   */
  function renderMarkdown(text, { skipEnhancements = false } = {}) {
    let html = marked.parse((text || '').trim())

    if (!skipEnhancements) {
      // KaTeX: deferred to post-streaming — formula may be incomplete during streaming
      html = renderKatexInString(html)
    }

    html = DOMPurify.sanitize(html, { ADD_TAGS: ['math', 'button'], ADD_ATTR: ['data-file-path', 'data-commit-sha', 'data-url', 'data-port', 'data-protocol', 'title'] })
    html = html.replace(/<table>/g, '<div class="table-wrap"><table>').replace(/<\/table>/g, '</table></div>')

    if (!skipEnhancements) {
      // Image styling, audio links, file path annotation: deferred to post-streaming
      const projectRoot = store.state.projectRoot
      html = rewriteImageUrls(html, projectRoot)
      html = convertAudioLinks(html)
      const { html: annotatedHtml, detectedPaths } = annotateFilePaths(html, { projectRoot })
      html = annotatedHtml
      if (detectedPaths.length > 0) {
        const uniquePaths = [...new Set(detectedPaths)]
        nextTick(() => {
          const el = document.getElementById('aiChatMessages')
          if (el) verifyFilePaths(uniquePaths, el)
        })
      }
      // Annotate commit hashes (7-40 hex chars with at least one a-f letter)
      const { html: commitAnnotatedHtml, detectedSHAs } = annotateCommitHashes(html)
      html = commitAnnotatedHtml
      if (detectedSHAs.length > 0) {
        const uniqueSHAs = [...new Set(detectedSHAs)]
        nextTick(() => {
          const el = document.getElementById('aiChatMessages')
          if (el) verifyCommitHashes(uniqueSHAs, el)
        })
      }
      // Annotate localhost URLs (e.g. http://localhost:30080) with clickable tags
      html = annotateLocalhostUrls(html)
    }

    return html
  }

  /**
   * Render a text block to HTML.
   *
   * When streaming=true (during streaming):
   *   Only pure markdown rendering — no structured detection.
   *   Tags like <scheduled-task> and <ask-question> remain as visible text.
   *   No KaTeX, no file path annotation, no path verification.
   *
   * When streaming=false (post-streaming / history load):
   *   Full pipeline: scheduled-task extraction, ask-question detection,
   *   tag stripping, and enhanced markdown rendering.
   */
  function renderTextBlock(text, msgId, blockIdx, streaming = false) {
    // ── Streaming: pure markdown only ──
    if (streaming) {
      return renderMarkdown(text, { skipEnhancements: true })
    }

    // ── Post-streaming: full pipeline ──

    // Extract scheduled-task IDs and batch-fetch their data
    const taskIds = extractScheduledTaskIds(text)
    if (taskIds.length > 0) {
      const taskKeys = taskIds.map((tid, tagIdx) => ({
        key: `${msgId}-${blockIdx}-${tagIdx}`,
        taskId: Number(tid),
      }))
      fetchBatchTaskData(taskKeys)
    }

    // Detect ask-question tags
    const askResult = detectAskQuestion(text)

    if (askResult.found) {
      const askKey = `${msgId}-${blockIdx}`
      if (!blockAskQuestions[askKey]) {
        const parsed = parseAskQuestionContent(askResult.content)
        if (parsed) {
          blockAskQuestions[askKey] = parsed
        }
      }
      // Remove the matched tag from the rendered text
      let cleanText
      if (askResult.endIdx !== undefined) {
        cleanText = (text.slice(0, askResult.startIdx) + text.slice(askResult.endIdx)).trim()
      } else {
        cleanText = text.slice(0, askResult.startIdx).trim()
      }
      cleanText = stripScheduledTaskTags(cleanText)
      return cleanText ? renderMarkdown(cleanText) : ''
    }

    // No ask-question: strip scheduled-task tags and render
    const cleanText = stripScheduledTaskTags(text)
    return cleanText ? renderMarkdown(cleanText) : ''
  }

  function extractScheduledTasks(msgs) {
    // Collect all task keys across messages for a single batch fetch
    const allTaskKeys = []
    for (const msg of msgs) {
      if (msg.role === 'assistant' && msg.blocks && !msg.streaming) {
        for (let bi = 0; bi < msg.blocks.length; bi++) {
          const block = msg.blocks[bi]
          if (block.type === 'text') {
            const taskIds = extractScheduledTaskIds(block.text || '')
            for (let tagIdx = 0; tagIdx < taskIds.length; tagIdx++) {
              allTaskKeys.push({
                key: `${msg.id}-${bi}-${tagIdx}`,
                taskId: Number(taskIds[tagIdx]),
              })
            }
          }
        }
      }
    }
    if (allTaskKeys.length > 0) {
      fetchBatchTaskData(allTaskKeys)
    }
  }

  function updateRenderedContents(forceFullRender = false) {
    // Defensive: if count diverged (e.g. loadHistory replaced messages),
    // force a full rebuild.
    if (!forceFullRender && lastRenderedCount > messages.value.length) {
      forceFullRender = true
    }

    // ── Deferred rendering: only render Mermaid when not streaming ──
    // During streaming, Mermaid code blocks are incomplete — rendering them
    // would produce errors. Defer to post-streaming forceFullRender.
    if (forceFullRender) {
      lastRenderedCount = messages.value.length
      nextTick(() => {
        const el = document.getElementById('aiChatMessages')
        if (el) renderMermaidInElement(el, 'chat-mermaid')
      })
    } else {
      const startIdx = lastRenderedCount
      const newMsgCount = messages.value.length - startIdx

      if (newMsgCount <= 0) return

      lastRenderedCount = messages.value.length

      // Skip Mermaid rendering during streaming — it will be rendered
      // when forceFullRender triggers after streaming ends.
    }
  }

  function toggleToolDetail(key) {
    expandedTools.value[key] = !expandedTools.value[key]
  }

  return {
    blockTasks,
    blockAskQuestions,
    expandedTools,
    renderMarkdown,
    renderTextBlock,
    parseAssistantContent,
    extractScheduledTasks,
    refreshTaskData,
    updateRenderedContents,
    toggleToolDetail,
    formatToolInput,
    toolCallSummary,
    hasImagesInContent,
    formatMessageTime,
    formatDetailTime,
    humanizeCron,
    repeatLabel,
    truncate,
    // Expose cache for ContentBlocks.vue integration
    staticBlockCache,
  }
}
