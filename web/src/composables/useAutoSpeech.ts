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
 *   - "summarizing" is only entered when the backend sends a phase event for it.
 *   - "synthesizing" is entered as soon as fetch returns OK (optimistic) or when
 *     the backend explicitly sends the phase event.
 */

import { ref } from 'vue'
import { useToast } from '@/composables/useToast.ts'
import { gt } from '@/composables/useLocale'
import i18n from '@/i18n'

const STORAGE_KEY = 'clawbench-auto-speech'

/** TTS lifecycle states — the single source of truth for UI rendering */
type SpeechState = 'idle' | 'summarizing' | 'synthesizing' | 'playing'

// --- Singleton state (shared across all instances) ---
const enabled = ref(false)
const state = ref<SpeechState>('idle')
const activeId = ref<string>('')
const playingSummary = ref<string>('')
const lastError = ref<string>('')
let abortController: AbortController | null = null
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

  // --- Internal: generate and play TTS for text ---
  async function _speak(id: string, text: string) {
    if (!text) return

    stopAudio()
    lastError.value = ''

    const controller = new AbortController()
    abortController = controller
    activeId.value = id
    // Set state IMMEDIATELY so the user sees "摘要中" right away —
    // don't wait for the fetch to return.
    state.value = 'summarizing'

    try {
      // POST to backend TTS endpoint (SSE streaming)
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

      // Parse SSE stream to get phase updates and the final result
      const reader = resp.body?.getReader()
      if (!reader) throw new Error(gt('autoSpeech.cannotReadStream'))

      const decoder = new TextDecoder()
      let resultData: any = null
      let sseBuffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        sseBuffer += decoder.decode(value, { stream: true })

        // Process complete SSE messages (delimited by \n\n)
        while (sseBuffer.includes('\n\n')) {
          const idx = sseBuffer.indexOf('\n\n')
          const block = sseBuffer.slice(0, idx)
          sseBuffer = sseBuffer.slice(idx + 2)

          for (const line of block.split('\n')) {
            if (!line.startsWith('data: ')) continue
            try {
              const event = JSON.parse(line.slice(6))
              if (event.type === 'phase') {
                const phase = event.phase as string
                if (phase === 'summarizing') {
                  state.value = 'summarizing'
                  await new Promise<void>(resolve => requestAnimationFrame(resolve))
                } else if (phase === 'synthesizing') {
                  state.value = 'synthesizing'
                  // Ensure "合成中" is visible for at least one frame before
                  // we process the result event (which switches to "playing").
                  await new Promise<void>(resolve => requestAnimationFrame(resolve))
                }
              } else if (event.type === 'result') {
                // Don't process result immediately — give the current phase
                // label (e.g. "合成中") time to render.  The result event
                // switches us to the "playing" state, which would overwrite
                // the phase label instantly.
                resultData = event
              }
            } catch { /* ignore malformed SSE lines */ }
          }
        }
      }

      if (!resultData) throw new Error(gt('autoSpeech.noResult'))

      // Handle synthesize failure
      if (resultData.synthesizeFailed) {
        throw new Error(resultData.synthesizeError || gt('autoSpeech.synthesisFailed'))
      }

      if (!resultData.audioPath) throw new Error(gt('autoSpeech.noAudioFile'))

      // Warn if summarization failed (fell back to full text)
      if (resultData.summarizeFailed) {
        toast.show(gt('autoSpeech.summaryFailed'), { icon: '🔊', type: 'info', duration: 3000 })
      }

      // Store the AI-generated summary for display
      if (resultData.summary) {
        playingSummary.value = resultData.summary
      }

      // Ensure "合成中" is visible for at least a short moment before
      // switching to "playing".  Without this, the synthesizing phase
      // label disappears instantly because result follows synthesizing
      // with zero gap.
      if (state.value === 'synthesizing') {
        await new Promise<void>(resolve => setTimeout(resolve, 300))
      }

      // Play audio via HTML5 Audio element
      const audioUrl = `/api/local-file/${encodeURIComponent(resultData.audioPath)}`
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

      await audio.play()
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
