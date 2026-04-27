import { ref, reactive, nextTick, watch } from 'vue'
import { escapeHtml } from '@/utils/html.ts'
import { baseName, splitPath } from '@/utils/path.ts'
import { marked, DOMPurify, hljs, mermaid } from '@/utils/globals.ts'
import { renderKatexInString, renderMermaidInElement } from '@/composables/useMarkdownRenderer.ts'
import { useFilePathAnnotation } from '@/composables/useFilePathAnnotation.ts'
import { useToast } from '@/composables/useToast.ts'
import { store } from '@/stores/app.ts'

export function useChatRender(options) {
  const { messages, theme, currentSessionId } = options
  const toast = useToast()
  const { annotateFilePaths, verifyFilePaths } = useFilePathAnnotation()

  const renderedContents = ref([])
  const renderCache = new Map()
  const RENDER_CACHE_MAX = 200
  const blockProposals = reactive({})
  const expandedTools = ref({})

  function trimRenderCache() {
    if (renderCache.size > RENDER_CACHE_MAX) {
      const keys = renderCache.keys()
      for (let i = 0; i < renderCache.size - RENDER_CACHE_MAX; i++) {
        renderCache.delete(keys.next().value)
      }
    }
  }

  // Clear cache and re-render when theme changes
  watch(theme, () => {
    renderCache.clear()
    updateRenderedContents(true)
  })

  function renderMarkdown(text) {
    let html = marked.parse((text || '').trim())
    html = renderKatexInString(html)
    html = DOMPurify.sanitize(html, { ADD_TAGS: ['math', 'button'], ADD_ATTR: ['data-file-path', 'title'] })
    html = html.replace(/<table>/g, '<div class="table-wrap"><table>').replace(/<\/table>/g, '</table></div>')
    html = html.replace(/<img([^>]*)>/g, (match, attrs) => {
      let cleanAttrs = attrs.replace(/\s*style="[^"]*"/i, '').replace(/\s*class="[^"]*"/i, '')
      return `<img${cleanAttrs} style="max-width: 200px; max-height: 200px; object-fit: cover; border-radius: 6px; margin: 4px 0; cursor: pointer;" class="chat-img-thumbnail">`
    })
    const audioExts = ['.mp3', '.wav', '.ogg', '.m4a', '.aac', '.flac', '.wma', '.opus']
    html = html.replace(/<a href="([^"]+)">([^<]*)<\/a>/g, (match, href, text) => {
      const lower = href.toLowerCase()
      if (audioExts.some(ext => lower.endsWith(ext))) {
        return `<div class="chat-audio-wrapper"><audio src="${href}" controls class="chat-audio-player"></audio></div>`
      }
      return match
    })
    const { html: annotatedHtml, detectedPaths } = annotateFilePaths(html, { projectRoot: store.state.projectRoot })
    html = annotatedHtml
    if (detectedPaths.length > 0) {
      const uniquePaths = [...new Set(detectedPaths)]
      nextTick(() => {
        const el = document.getElementById('aiChatMessages')
        if (el) verifyFilePaths(uniquePaths, el)
      })
    }
    return html
  }

  function renderTextBlock(text, msgId, blockIdx) {
    const proposalMatch = text.match(/<schedule-proposal(\s+confirmed)?>([\s\S]*?)<\/schedule-proposal>/)
    if (proposalMatch) {
      const proposalKey = `${msgId}-${blockIdx}`
      if (!blockProposals[proposalKey]) {
        try {
          const proposal = JSON.parse(proposalMatch[2].trim())
          blockProposals[proposalKey] = { proposal }
        } catch (e) {
          console.error('Failed to parse schedule proposal:', e)
        }
      }
      const cleanText = text.replace(/<schedule-proposal(\s+confirmed)?>[\s\S]*?<\/schedule-proposal>/, '').trim()
      return cleanText ? renderMarkdown(cleanText) : ''
    }
    return renderMarkdown(text)
  }

  async function createScheduledTask(proposal) {
    try {
      const body = { ...proposal, session_id: currentSessionId.value || undefined }
      const resp = await fetch('/api/tasks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data = await resp.json()
      if (resp.ok && data.ok) {
        toast.show('定时任务已创建', { icon: '✅', duration: 2000 })
      } else {
        toast.show('任务创建失败: ' + (data.error || resp.statusText), { icon: '⚠️' })
      }
    } catch (err) {
      toast.show('任务创建失败: ' + err.message, { icon: '⚠️' })
    }
  }

  function renderMsg(msg) {
    return renderMarkdown(msg.content)
  }

  function parseAssistantContent(content) {
    if (!content) return { blocks: [], metadata: null, scheduledTask: null }
    try {
      const parsed = JSON.parse(content)
      if (parsed.blocks && Array.isArray(parsed.blocks)) {
        return {
          blocks: parsed.blocks.map(b => {
            if (b.type === 'tool_use' && b.done === undefined) b.done = true
            return b
          }),
          metadata: parsed.metadata || null,
          cancelled: parsed.cancelled || false,
          scheduledTask: parsed.scheduledTask || null
        }
      }
    } catch {}
    return { blocks: [{ type: 'text', text: content }], metadata: null, scheduledTask: null }
  }

  function extractScheduleProposals(msgs) {
    for (const msg of msgs) {
      if (msg.role === 'assistant' && msg.blocks && !msg.streaming) {
        for (let bi = 0; bi < msg.blocks.length; bi++) {
          const block = msg.blocks[bi]
          if (block.type === 'text') {
            const proposalKey = `${msg.id}-${bi}`
            if (blockProposals[proposalKey]) continue
            const proposalMatch = block.text.match(/<schedule-proposal(\s+confirmed)?>([\s\S]*?)<\/schedule-proposal>/)
            if (proposalMatch) {
              try {
                const proposal = JSON.parse(proposalMatch[2].trim())
                blockProposals[proposalKey] = { proposal }
              } catch (e) {
                console.error('Failed to parse schedule proposal:', e)
              }
            }
          }
        }
      }
    }
  }

  function updateRenderedContents(forceFullRender = false) {
    if (forceFullRender) {
      renderedContents.value = messages.value.map(msg => {
        if (msg.role === 'assistant' && msg.blocks) {
          return ''
        }
        const key = msg.content || ''
        if (key && renderCache.has(key)) {
          return renderCache.get(key)
        }
        const html = renderMsg(msg)
        if (key) {
          renderCache.set(key, html)
          trimRenderCache()
        }
        return html
      })
      nextTick(() => {
        const el = document.getElementById('aiChatMessages')
        if (el) renderMermaidInElement(el, 'chat-mermaid')
      })
    } else {
      const startIdx = renderedContents.value.length
      const newMsgs = messages.value.slice(startIdx)

      if (newMsgs.length === 0) return

      const newContents = newMsgs.map(msg => {
        if (msg.role === 'assistant' && msg.blocks) {
          return ''
        }
        const key = msg.content || ''
        if (key && renderCache.has(key)) {
          return renderCache.get(key)
        }
        const html = renderMsg(msg)
        if (key) {
          renderCache.set(key, html)
          trimRenderCache()
        }
        return html
      })

      renderedContents.value = [...renderedContents.value, ...newContents]

      nextTick(() => {
        const el = document.getElementById('aiChatMessages')
        if (el) {
          const newBlocks = el.querySelectorAll(`.chat-message:nth-last-child(n+${startIdx + 1}) pre.mermaid:not([data-rendered])`)
          if (newBlocks.length > 0) {
            renderMermaidInElement(el, 'chat-mermaid', newBlocks)
          }
        }
      })
    }
  }

  function toggleToolDetail(key) {
    expandedTools.value[key] = !expandedTools.value[key]
  }

  function formatToolInput(input) {
    if (!input) return ''
    try {
      const json = JSON.stringify(input, null, 2)
      return hljs.highlight(json, { language: 'json' }).value
    } catch {
      return JSON.stringify(input, null, 2)
    }
  }

  function toolCallSummary(block) {
    if (!block.input) return ''
    const obj = block.input
    if (obj.file_path) return baseName(obj.file_path)
    if (obj.command) return obj.command.length > 60 ? obj.command.slice(0, 57) + '...' : obj.command
    if (obj.path) return baseName(obj.path)
    if (obj.src_path && obj.dst_path) return `${baseName(obj.src_path)} → ${baseName(obj.dst_path)}`
    const firstVal = Object.values(obj)[0]
    if (typeof firstVal === 'string' && firstVal.length < 80) return firstVal
    return ''
  }

  function hasImagesInContent(content) {
    return content && content.includes('![')
  }

  function formatMessageTime(createdAt) {
    const date = new Date(createdAt)
    const now = new Date()
    const diffMs = now - date
    const diffMins = Math.floor(diffMs / 60000)

    if (diffMins < 1) return '刚刚'
    if (diffMins < 60) return `${diffMins}分钟前`

    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours}小时前`

    const diffDays = Math.floor(diffHours / 24)
    if (diffDays < 7) return `${diffDays}天前`

    const month = date.getMonth() + 1
    const day = date.getDate()
    const hour = date.getHours().toString().padStart(2, '0')
    const minute = date.getMinutes().toString().padStart(2, '0')
    return `${month}/${day} ${hour}:${minute}`
  }

  function formatDetailTime(createdAt) {
    const date = new Date(createdAt)
    const year = date.getFullYear()
    const month = (date.getMonth() + 1).toString().padStart(2, '0')
    const day = date.getDate().toString().padStart(2, '0')
    const hour = date.getHours().toString().padStart(2, '0')
    const minute = date.getMinutes().toString().padStart(2, '0')
    const second = date.getSeconds().toString().padStart(2, '0')
    return `${year}-${month}-${day} ${hour}:${minute}:${second}`
  }

  function humanizeCron(expr) {
    const parts = expr.split(' ')
    if (parts.length !== 5) return expr
    const [min, hour, day, month, weekday] = parts
    if (min.startsWith('*/') && hour === '*') return `每 ${min.slice(2)} 分钟`
    if (hour.startsWith('*/') && min === '0') return `每 ${hour.slice(2)} 小时`
    if (min === '0' && !hour.includes('/') && day === '*' && month === '*' && weekday === '*') return `每天 ${hour}:00`
    if (min === '0' && weekday === '1-5') return `工作日 ${hour}:00`
    return expr
  }

  function repeatLabel(mode, maxRuns) {
    if (mode === 'once') return '单次执行'
    if (mode === 'limited') return `${maxRuns} 次后停止`
    return '不限次数'
  }

  function truncate(str, len) {
    if (!str) return ''
    const runes = [...str]
    return runes.length > len ? runes.slice(0, len).join('') + '...' : str
  }

  return {
    renderedContents,
    blockProposals,
    expandedTools,
    renderMarkdown,
    renderTextBlock,
    parseAssistantContent,
    extractScheduleProposals,
    updateRenderedContents,
    createScheduledTask,
    toggleToolDetail,
    formatToolInput,
    toolCallSummary,
    hasImagesInContent,
    formatMessageTime,
    formatDetailTime,
    humanizeCron,
    repeatLabel,
    truncate,
    renderMsg,
  }
}
