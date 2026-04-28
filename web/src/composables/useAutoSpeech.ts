/**
 * useAutoSpeech
 *
 * Manages the auto-speech toggle state and audio playback for AI messages.
 * When enabled, AI replies are automatically summarized and read aloud via TTS.
 * Toggle state is persisted in localStorage.
 *
 * Uses module-level singleton state so all consumers share the same toggle/audio state.
 * Should only be instantiated once (in ChatPanel.vue).
 */

import { ref, onUnmounted } from 'vue'

const STORAGE_KEY = 'clawbench-auto-speech'

// --- Singleton state (shared across all instances) ---
const enabled = ref(false)
const isGenerating = ref(false)
const currentAudio = ref<HTMLAudioElement | null>(null)
const playingText = ref<string>('')
const playingSummary = ref<string>('')
let abortController: AbortController | null = null

// Load persisted state once at module level
try {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved !== null) enabled.value = saved === 'true'
} catch {
  // localStorage may be unavailable (e.g. private browsing)
}

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
    // If toggled OFF, stop any playing audio and pending requests
    if (!enabled.value) stopAudio()
  }

  // --- Audio Playback ---
  function stopAudio() {
    // Cancel any in-flight TTS request
    abortController?.abort()
    abortController = null
    // Stop currently playing audio
    if (currentAudio.value) {
      currentAudio.value.pause()
      currentAudio.value.currentTime = 0
      currentAudio.value = null
    }
    playingText.value = ''
    playingSummary.value = ''
  }

  // --- Internal: generate and play TTS for text ---
  async function _speak(text: string) {
    if (!text) return

    // Interrupt any currently playing audio and pending request
    stopAudio()

    // Set up new abort controller for this request
    const controller = new AbortController()
    abortController = controller
    isGenerating.value = true
    playingText.value = text

    try {
      // POST to backend TTS endpoint
      const resp = await fetch('/api/tts/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
        signal: controller.signal,
      })
      const data = await resp.json()
      if (!resp.ok) throw new Error(data.error || 'TTS failed')

      // Store the AI-generated summary for display in TtsPopover
      if (data.summary) {
        playingSummary.value = data.summary
      }

      // Play audio via HTML5 Audio element
      const audioUrl = `/api/local-file/${encodeURIComponent(data.audioPath)}`
      const audio = new Audio(audioUrl)
      currentAudio.value = audio

      audio.onended = () => {
        currentAudio.value = null
        playingText.value = ''
        playingSummary.value = ''
      }
      audio.onerror = () => {
        console.warn('Auto-speech: audio playback failed')
        currentAudio.value = null
        playingText.value = ''
        playingSummary.value = ''
      }

      await audio.play()
    } catch (err: any) {
      // Ignore AbortError (interrupted by a newer request)
      if (err?.name === 'AbortError') return
      console.warn('Auto-speech: generation failed:', err)
    } finally {
      // Only clear generating state if this is still the active request
      if (abortController === controller) {
        isGenerating.value = false
        abortController = null
      }
    }
  }

  // --- Auto-speech trigger (respects toggle state) ---
  function speakMessage(text: string) {
    if (!enabled.value) return
    _speak(text)
  }

  // --- Manual play trigger (always works, regardless of toggle) ---
  function speakText(text: string) {
    _speak(text)
  }

  // --- Check if a specific text is currently generating ---
  function isGeneratingText(text: string): boolean {
    return playingText.value === text && isGenerating.value
  }

  // --- Check if a specific text is currently playing audio ---
  function isPlayingAudio(text: string): boolean {
    return playingText.value === text && !isGenerating.value && currentAudio.value !== null
  }

  // --- Check if a specific text is in any active state (generating or playing) ---
  function isActive(text: string): boolean {
    return playingText.value === text && (isGenerating.value || currentAudio.value !== null)
  }

  // --- Get the AI-generated summary for the currently playing text ---
  function getSummary(text: string): string {
    return playingText.value === text ? playingSummary.value : ''
  }

  // --- Lifecycle ---
  onUnmounted(() => {
    stopAudio()
  })

  return {
    enabled,
    isGenerating,
    toggle,
    speakMessage,
    speakText,
    stopAudio,
    isGeneratingText,
    isPlayingAudio,
    isActive,
    getSummary,
  }
}
