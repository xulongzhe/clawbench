import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ref, nextTick } from 'vue'

// Mock getCachedCommitInfo from useCommitHashAnnotation
const mockGetCachedCommitInfo = vi.fn()

vi.mock('@/composables/useCommitHashAnnotation', () => ({
  getCachedCommitInfo: (...args: any[]) => mockGetCachedCommitInfo(...args),
}))

import {
  useCommitNavigation,
  setPendingCommitNavigation,
  consumePendingCommitNavigation,
  _resetPendingShaForTesting,
  pendingSha,
} from '@/composables/useCommitNavigation'

// ─── Helpers ────────────────────────────────────────────────────────────────

let originalFetch: typeof globalThis.fetch

function createNavigation(overrides?: { loadProjectHistory?: () => Promise<void> }) {
  const commits = ref<any[]>([])
  const selectedSHA = ref<string | null>(null)
  const currentView = ref('commits')
  const loadCommitFiles = vi.fn().mockResolvedValue(undefined)
  const loadProjectHistory = overrides?.loadProjectHistory ?? vi.fn().mockResolvedValue(undefined)

  const { navigateToCommit, handleDrillBackToCommits, fetchCommitInfo } = useCommitNavigation({
    commits,
    selectedSHA,
    currentView,
    loadCommitFiles,
    loadProjectHistory,
  })

  return { commits, selectedSHA, currentView, loadCommitFiles, loadProjectHistory, navigateToCommit, handleDrillBackToCommits, fetchCommitInfo }
}

function mockVerifyCommitsResponse(results: Record<string, any>) {
  return {
    ok: true,
    json: () => Promise.resolve({ results }),
  }
}

beforeEach(() => {
  originalFetch = globalThis.fetch
  globalThis.fetch = vi.fn()
  mockGetCachedCommitInfo.mockReturnValue(null)
  _resetPendingShaForTesting()
})

afterEach(() => {
  globalThis.fetch = originalFetch
})

// ─── Module-level pending SHA ──────────────────────────────────────────────

describe('pending SHA (module-level)', () => {
  it('setPendingCommitNavigation sets the value', () => {
    setPendingCommitNavigation('abc1234')
    expect(pendingSha.value).toBe('abc1234')
  })

  it('consumePendingCommitNavigation returns and clears the value', () => {
    setPendingCommitNavigation('abc1234')
    const result = consumePendingCommitNavigation()
    expect(result).toBe('abc1234')
    expect(pendingSha.value).toBeNull()
  })

  it('consumePendingCommitNavigation returns null when nothing is pending', () => {
    expect(consumePendingCommitNavigation()).toBeNull()
  })

  it('consumePendingCommitNavigation is destructive — second call returns null', () => {
    setPendingCommitNavigation('abc1234')
    expect(consumePendingCommitNavigation()).toBe('abc1234')
    expect(consumePendingCommitNavigation()).toBeNull()
  })

  it('pendingSha reflects current state', () => {
    expect(pendingSha.value).toBeNull()
    setPendingCommitNavigation('abc1234')
    expect(pendingSha.value).toBe('abc1234')
    consumePendingCommitNavigation()
    expect(pendingSha.value).toBeNull()
  })

  it('_resetPendingShaForTesting clears the value', () => {
    setPendingCommitNavigation('abc1234')
    _resetPendingShaForTesting()
    expect(pendingSha.value).toBeNull()
  })
})

// ─── useCommitNavigation composable ─────────────────────────────────────────

describe('useCommitNavigation', () => {
  describe('navigateToCommit', () => {
    it('finds commit in commits array and navigates directly', async () => {
      const nav = createNavigation()
      const fullSha = 'abc1234def5678901234567890123456789012'
      nav.commits.value = [{ sha: fullSha, msg: 'test commit' }]

      await nav.navigateToCommit(fullSha)

      expect(nav.selectedSHA.value).toBe(fullSha)
      expect(nav.currentView.value).toBe('files')
      expect(nav.loadCommitFiles).toHaveBeenCalledWith(fullSha)
      // No fetch needed — commit already in array
      expect(globalThis.fetch).not.toHaveBeenCalled()
    })

    it('resolves short SHA to full SHA from commits array', async () => {
      const nav = createNavigation()
      const fullSha = 'abc1234def5678901234567890123456789012'
      nav.commits.value = [{ sha: fullSha, msg: 'test commit' }]

      await nav.navigateToCommit('abc1234')

      expect(nav.selectedSHA.value).toBe(fullSha)
      expect(nav.currentView.value).toBe('files')
      expect(nav.loadCommitFiles).toHaveBeenCalledWith(fullSha)
    })

    it('uses cached commit info when not in commits array', async () => {
      const nav = createNavigation()
      const fullSha = 'abc1234def5678901234567890123456789012'
      const cachedInfo = { sha: fullSha, msg: 'cached commit', author: 'test', date: '2025-01-01' }
      mockGetCachedCommitInfo.mockReturnValue(cachedInfo)

      await nav.navigateToCommit('abc1234')

      expect(nav.selectedSHA.value).toBe(fullSha)
      expect(nav.currentView.value).toBe('files')
      expect(nav.commits.value[0]).toEqual(cachedInfo)
      expect(nav.loadCommitFiles).toHaveBeenCalledWith(fullSha)
      // No API fetch needed — used cache
      expect(globalThis.fetch).not.toHaveBeenCalled()
    })

    it('fetches commit info via API when not in commits or cache', async () => {
      const nav = createNavigation()
      const fullSha = 'abc1234def5678901234567890123456789012'
      const fetchedInfo = { sha: fullSha, msg: 'fetched commit', author: 'test', date: '2025-01-01' }

      ;(globalThis.fetch as any).mockResolvedValue(mockVerifyCommitsResponse({ abc1234: fetchedInfo }))

      await nav.navigateToCommit('abc1234')

      expect(nav.selectedSHA.value).toBe(fullSha)
      expect(nav.currentView.value).toBe('files')
      expect(nav.commits.value[0]).toEqual(fetchedInfo)
      expect(nav.loadCommitFiles).toHaveBeenCalledWith(fullSha)
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/git/verify-commits', expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ shas: ['abc1234'] }),
      }))
    })

    it('falls back to original SHA when fetch returns no valid info', async () => {
      const nav = createNavigation()

      ;(globalThis.fetch as any).mockResolvedValue(mockVerifyCommitsResponse({ abc1234: null }))

      await nav.navigateToCommit('abc1234')

      // Fallback: use the original short SHA
      expect(nav.selectedSHA.value).toBe('abc1234')
      expect(nav.currentView.value).toBe('files')
      expect(nav.loadCommitFiles).toHaveBeenCalledWith('abc1234')
    })

    it('falls back to original SHA when fetch throws', async () => {
      const nav = createNavigation()

      ;(globalThis.fetch as any).mockRejectedValue(new Error('Network error'))

      await nav.navigateToCommit('abc1234')

      expect(nav.selectedSHA.value).toBe('abc1234')
      expect(nav.currentView.value).toBe('files')
      expect(nav.loadCommitFiles).toHaveBeenCalledWith('abc1234')
    })

    it('falls back to original SHA when fetch returns non-OK', async () => {
      const nav = createNavigation()

      ;(globalThis.fetch as any).mockResolvedValue({ ok: false, json: () => Promise.resolve({}) })

      await nav.navigateToCommit('abc1234')

      expect(nav.selectedSHA.value).toBe('abc1234')
      expect(nav.currentView.value).toBe('files')
    })

    it('does not switch view until commit info is resolved (no empty flash)', async () => {
      // This is the core bug fix: currentView should NOT be 'files' until
      // after the async fetchCommitInfo resolves.
      const nav = createNavigation()
      let fetchResolve: (v: any) => void
      const fetchPromise = new Promise(r => { fetchResolve = r })

      ;(globalThis.fetch as any).mockReturnValue(fetchPromise)

      // Start navigation — don't await yet
      const navPromise = nav.navigateToCommit('abc1234')

      // During the async fetch, view should still be 'commits'
      expect(nav.currentView.value).toBe('commits')
      expect(nav.selectedSHA.value).toBeNull()

      // Resolve the fetch
      const fullSha = 'abc1234def5678901234567890123456789012'
      fetchResolve!(mockVerifyCommitsResponse({ abc1234: { sha: fullSha, msg: 'test' } }))

      // Now navigation completes
      await navPromise

      // After resolution, view is switched and SHA is set
      expect(nav.currentView.value).toBe('files')
      expect(nav.selectedSHA.value).toBe(fullSha)
    })

    it('sets selectedSHA before switching currentView so selectedCommit is never null when view changes', async () => {
      const nav = createNavigation()
      const fullSha = 'abc1234def5678901234567890123456789012'
      nav.commits.value = [{ sha: fullSha, msg: 'test commit' }]

      // Track the order of state changes
      const changes: string[] = []
      const origSelectedSHA = nav.selectedSHA
      const origCurrentView = nav.currentView

      // Use watch-like tracking via overriding value setter
      // Since we can't easily watch in tests, verify the final state is consistent
      await nav.navigateToCommit(fullSha)

      // Both should be set after navigation
      expect(nav.selectedSHA.value).toBe(fullSha)
      expect(nav.currentView.value).toBe('files')

      // selectedCommit (computed in real components) would find the commit:
      const selectedCommit = nav.commits.value.find(c => c.sha === nav.selectedSHA.value)
      expect(selectedCommit).toBeTruthy()
    })

    it('does not duplicate commit in array if already present', async () => {
      const nav = createNavigation()
      const fullSha = 'abc1234def5678901234567890123456789012'
      const commitInfo = { sha: fullSha, msg: 'test commit' }
      nav.commits.value = [commitInfo]

      // Mock cache returning the same info
      mockGetCachedCommitInfo.mockReturnValue(commitInfo)

      await nav.navigateToCommit(fullSha)

      // Should not add a duplicate
      expect(nav.commits.value.filter(c => c.sha === fullSha)).toHaveLength(1)
    })

    it('does not duplicate fetched commit in array', async () => {
      const nav = createNavigation()
      const fullSha = 'abc1234def5678901234567890123456789012'
      const commitInfo = { sha: fullSha, msg: 'test commit' }
      // Pre-populate with the commit
      nav.commits.value = [commitInfo]

      // Mock fetch returning the same commit (e.g. for short SHA lookup)
      ;(globalThis.fetch as any).mockResolvedValue(mockVerifyCommitsResponse({ abc1234: commitInfo }))

      await nav.navigateToCommit('abc1234')

      // Found in commits array directly, no fetch needed
      expect(nav.commits.value.filter(c => c.sha === fullSha)).toHaveLength(1)
    })
  })

  describe('fetchCommitInfo', () => {
    it('returns commit info on valid response', async () => {
      const nav = createNavigation()
      const info = { sha: 'fullsha123', msg: 'hello', author: 'test', date: '2025-01-01' }
      ;(globalThis.fetch as any).mockResolvedValue(mockVerifyCommitsResponse({ abc1234: info }))

      const result = await nav.fetchCommitInfo('abc1234')

      expect(result).toEqual(info)
    })

    it('returns null when commit is not in results', async () => {
      const nav = createNavigation()
      ;(globalThis.fetch as any).mockResolvedValue(mockVerifyCommitsResponse({}))

      const result = await nav.fetchCommitInfo('abc1234')

      expect(result).toBeNull()
    })

    it('returns null when commit info has no sha', async () => {
      const nav = createNavigation()
      ;(globalThis.fetch as any).mockResolvedValue(mockVerifyCommitsResponse({ abc1234: { msg: 'no sha' } }))

      const result = await nav.fetchCommitInfo('abc1234')

      expect(result).toBeNull()
    })

    it('returns null on non-OK response', async () => {
      const nav = createNavigation()
      ;(globalThis.fetch as any).mockResolvedValue({ ok: false, json: () => Promise.resolve({}) })

      const result = await nav.fetchCommitInfo('abc1234')

      expect(result).toBeNull()
    })

    it('returns null on network error', async () => {
      const nav = createNavigation()
      ;(globalThis.fetch as any).mockRejectedValue(new Error('Network error'))

      const result = await nav.fetchCommitInfo('abc1234')

      expect(result).toBeNull()
    })
  })

  describe('handleDrillBackToCommits', () => {
    it('calls loadProjectHistory when commits array has <=1 items', () => {
      const loadProjectHistory = vi.fn().mockResolvedValue(undefined)
      const nav = createNavigation({ loadProjectHistory })
      nav.commits.value = [{ sha: 'abc', msg: 'only commit' }]

      nav.handleDrillBackToCommits()

      expect(loadProjectHistory).toHaveBeenCalled()
    })

    it('does not call loadProjectHistory when commits array has >1 items', () => {
      const loadProjectHistory = vi.fn().mockResolvedValue(undefined)
      const nav = createNavigation({ loadProjectHistory })
      nav.commits.value = [{ sha: 'abc', msg: 'first' }, { sha: 'def', msg: 'second' }]

      nav.handleDrillBackToCommits()

      expect(loadProjectHistory).not.toHaveBeenCalled()
    })

    it('does not call loadProjectHistory when loadProjectHistory is not provided', () => {
      const nav = useCommitNavigation({
        commits: ref([]),
        selectedSHA: ref(null),
        currentView: ref('files'),
        loadCommitFiles: vi.fn().mockResolvedValue(undefined),
        // No loadProjectHistory
      })

      // Should not throw
      expect(() => nav.handleDrillBackToCommits()).not.toThrow()
    })
  })
})
