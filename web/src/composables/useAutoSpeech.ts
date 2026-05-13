/**
 * useAutoSpeech
 *
 * Manages the auto-speech toggle state and audio playback for AI messages.
 * When enabled, AI replies are automatically summarized and read aloud via TTS.
 * Toggle state is persisted in localStorage.
 *
 * Uses module-level singleton state so all consumers share the same toggle/audio state.
 * Should only be instantiated once (in ChatPanel.vue) and provided via inject to children.
 *
 * State machine: idle → summarizing → synthesizing → playing → idle
 *   - Phase transitions are driven by EventSource SSE events from the backend.
 *   - Cache hits skip SSE entirely and play audio immediately.
 */

import { ref } from 'vue'
import { useToast } from '@/composables/useToast.ts'
import { gt } from '@/composables/useLocale'
import i18n from '@/i18n'

const STORAGE_KEY = 'clawbench-auto-speech'

/**
 * Extract speakable text from chat message blocks.
 * Includes both text blocks and AskUserQuestion tool_use blocks
 * (structured questions) so TTS can read the question and options.
 */
export function extractSpeakableText(blocks: any[]): string {
  const parts: string[] = []
  for (const b of blocks) {
    if (b.type === 'text') {
      const t = (b.text || '').trim()
      if (t) parts.push(t)
    } else if (b.type === 'tool_use' && b.name === 'AskUserQuestion' && b.input?.questions) {
      const questions = b.input.questions
      for (const q of questions) {
        let s = q.question || ''
        if (q.header) s += ` (${q.header})`
        const opts = Array.isArray(q.options) ? q.options : []
        if (opts.length > 0) {
          s += ': ' + opts.map((o: any) => {
            const label = typeof o === 'string' ? o : (o.label || '')
            const desc = typeof o === 'object' ? (o.description || '') : ''
            return desc && desc !== label ? `${label} — ${desc}` : label
          }).join(', ')
        }
        if (s) parts.push(s)
      }
    }
  }
  return parts.join('\n').trim()
}

/** TTS lifecycle states — the single source of truth for UI rendering */
type SpeechState = 'idle' | 'summarizing' | 'synthesizing' | 'playing'

// --- Singleton state (shared across all instances) ---
const enabled = ref(false)
const state = ref<SpeechState>('idle')
const activeId = ref<string>('')
const playingSummary = ref<string>('')
const lastError = ref<string>('')
let abortController: AbortController | null = null
let currentEventSource: EventSource | null = null
let currentAudioEl: HTMLAudioElement | null = null

// Load persisted state once at module level
try {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved !== null) enabled.value = saved === 'true'
} catch {
  // localStorage may be unavailable (e.g. private browsing)
}

// Module-level toast instance (shared, not per-component)
const toast = useToast()

export function useAutoSpeech() {
  // --- Persistence ---
  function saveState() {
    try {
      localStorage.setItem(STORAGE_KEY, String(enabled.value))
    } catch {
      // Silently ignore
    }
  }

  function toggle() {
    enabled.value = !enabled.value
    saveState()
    if (!enabled.value) stopAudio()
  }

  // --- Audio Playback ---
  function stopAudio() {
    abortController?.abort()
    abortController = null
    if (currentEventSource) {
      currentEventSource.close()
      currentEventSource = null
    }
    if (currentAudioEl) {
      currentAudioEl.pause()
      currentAudioEl.currentTime = 0
      currentAudioEl.onended = null
      currentAudioEl.onerror = null
      currentAudioEl = null
    }
    activeId.value = ''
    playingSummary.value = ''
    state.value = 'idle'
  }

  function reportError(message: string) {
    lastError.value = message
    toast.show(message, { icon: '🔊', type: 'error', duration: 5000 })
  }

  // --- Internal: play audio from a path ---
  function playAudio(audioPath: string) {
    const audioUrl = `/api/local-file/${encodeURIComponent(audioPath)}`
    const audio = new Audio(audioUrl)
    currentAudioEl = audio
    state.value = 'playing'

    audio.onended = () => {
      if (currentAudioEl === audio) {
        currentAudioEl = null
        activeId.value = ''
        playingSummary.value = ''
        state.value = 'idle'
      }
    }
    audio.onerror = () => {
      if (currentAudioEl === audio) {
        currentAudioEl = null
        activeId.value = ''
        playingSummary.value = ''
        state.value = 'idle'
        reportError(gt('autoSpeech.playbackFailed'))
      }
    }

    audio.play().catch((err: any) => {
      if (err?.name === 'AbortError') return
      let message = gt('autoSpeech.generateFailedGeneric')
      if (err?.name === 'NotAllowedError') {
        message = gt('autoSpeech.autoplayBlocked')
      }
      reportError(message)
      activeId.value = ''
      playingSummary.value = ''
      state.value = 'idle'
    })
  }

  // --- Internal: generate and play TTS for text ---
  async function _speak(id: string, text: string) {
    if (!text) return

    stopAudio()
    lastError.value = ''

    const controller = new AbortController()
    abortController = controller
    activeId.value = id

    try {
      // Step 1: POST to create TTS job (or get cached result)
      const resp = await fetch('/api/tts/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text, language: i18n.global.locale.value }),
        signal: controller.signal,
      })

      if (!resp.ok) {
        let errorMsg = gt('autoSpeech.generateFailed', { status: resp.status })
        try {
          const errData = await resp.json()
          if (errData.error) errorMsg = errData.error
        } catch { /* ignore parse error */ }
        throw new Error(errorMsg)
      }

      const data = await resp.json()

      // Cache hit — play audio immediately, no SSE needed
      if (data.cached && data.audioPath) {
        if (data.summary) {
          playingSummary.value = data.summary
        }
        playAudio(data.audioPath)
        return
      }

      // Cache miss — set state to summarizing immediately so the user sees
      // feedback while the EventSource connection is being established.
      state.value = 'summarizing'

      // Connect to EventSource for phase updates
      if (!data.jobId) throw new Error(gt('autoSpeech.noResult'))

      const es = new EventSource(`/api/tts/stream/${data.jobId}`)
      currentEventSource = es

      let resultData: any = null

      es.addEventListener('phase', (e: MessageEvent) => {
        try {
          const event = JSON.parse(e.data)
          if (event.phase === 'summarizing') {
            state.value = 'summarizing'
          } else if (event.phase === 'synthesizing') {
            state.value = 'synthesizing'
          }
        } catch { /* ignore malformed data */ }
      })

      es.addEventListener('result', (e: MessageEvent) => {
        try {
          resultData = JSON.parse(e.data)
        } catch { /* ignore malformed data */ }
        // Close EventSource — we have the result
        es.close()
        currentEventSource = null
        handleResult(resultData)
      })

      es.onerror = () => {
        es.close()
        if (currentEventSource === es) {
          currentEventSource = null
        }
        // If we already have result data, process it
        if (resultData) {
          handleResult(resultData)
          return
        }
        // Otherwise report error (unless aborted)
        if (controller.signal.aborted) return
        reportError(gt('autoSpeech.generateFailedGeneric'))
        activeId.value = ''
        playingSummary.value = ''
        state.value = 'idle'
      }

      function handleResult(result: any) {
        if (!result) {
          reportError(gt('autoSpeech.noResult'))
          activeId.value = ''
          playingSummary.value = ''
          state.value = 'idle'
          return
        }

        // Handle synthesize failure
        if (result.synthesizeFailed) {
          reportError(result.synthesizeError || gt('autoSpeech.synthesisFailed'))
          activeId.value = ''
          playingSummary.value = ''
          state.value = 'idle'
          return
        }

        if (!result.audioPath) {
          reportError(gt('autoSpeech.noAudioFile'))
          activeId.value = ''
          playingSummary.value = ''
          state.value = 'idle'
          return
        }

        // Warn if summarization failed (fell back to full text)
        if (result.summarizeFailed) {
          toast.show(gt('autoSpeech.summaryFailed'), { icon: '🔊', type: 'info', duration: 3000 })
        }

        // Store the AI-generated summary for display
        if (result.summary) {
          playingSummary.value = result.summary
        }

        playAudio(result.audioPath)
      }

    } catch (err: any) {
      if (err?.name === 'AbortError') return

      let message = gt('autoSpeech.generateFailedGeneric')
      if (err?.name === 'NotAllowedError') {
        message = gt('autoSpeech.autoplayBlocked')
      } else if (err?.message) {
        message = err.message
      }
      reportError(message)
      activeId.value = ''
      playingSummary.value = ''
      state.value = 'idle'
    } finally {
      if (abortController === controller) {
        abortController = null
      }
    }
  }

  function speakMessage(id: string, text: string) {
    if (!enabled.value) return
    _speak(id, text)
  }

  function speakText(id: string, text: string) {
    _speak(id, text)
  }

  function isActive(id: string): boolean {
    return activeId.value === id && state.value !== 'idle'
  }

  function getSummary(id: string): string {
    return activeId.value === id ? playingSummary.value : ''
  }

  function getPhaseLabel(id: string): string {
    if (activeId.value !== id) return ''
    switch (state.value) {
      case 'summarizing': return 'summarizing'
      case 'synthesizing': return 'synthesizing'
      case 'playing': return 'playing'
      default: return ''
    }
  }

  function isGeneratingText(id: string): boolean {
    return activeId.value === id
      && (state.value === 'summarizing' || state.value === 'synthesizing')
  }

  function isPlayingAudio(id: string): boolean {
    return activeId.value === id && state.value === 'playing'
  }

  return {
    enabled,
    state,
    lastError,
    toggle,
    speakMessage,
    speakText,
    stopAudio,
    isActive,
    getSummary,
    getPhaseLabel,
    isGeneratingText,
    isPlayingAudio,
  }
}
