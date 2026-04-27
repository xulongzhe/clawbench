// Global application state (singleton reactive store)
import { reactive } from 'vue'
import { apiGet, apiPost } from '@/utils/api.ts'
import { baseName, dirName } from '@/utils/helpers.ts'

interface DirEntry {
    name: string
    type: 'dir' | 'file'
    size?: number
    modTime?: string
}

interface CurrentFile {
    name: string
    path: string
    content?: string | null
    isImage?: boolean
    isPdf?: boolean
    isAudio?: boolean
    isVideo?: boolean
    tooLarge?: boolean
    size?: number
    error?: string
}

interface AppState {
    // Project
    projectRoot: string
    projectName: string
    watchDir: string

    // Upload config
    uploadMaxSizeMB: number
    uploadMaxFiles: number

    // Chat UI config
    chatInitialMessages: number
    chatPageSize: number
    chatCollapsedHeight: number

    // File browser
    currentDir: string
    dirEntries: DirEntry[]
    allItems: DirEntry[]
    currentFileList: unknown[]

    // Current file
    currentFile: CurrentFile | null

    // File history (browser-like navigation)
    fileHistory: string[]
    fileHistoryIndex: number

    // Theme
    theme: string

}

const state = reactive<AppState>({
    // Project
    projectRoot: '',
    projectName: '',
    watchDir: '',

    // Upload config
    uploadMaxSizeMB: 100,
    uploadMaxFiles: 20,

    // Chat UI config
    chatInitialMessages: 20,
    chatPageSize: 20,
    chatCollapsedHeight: 150,

    // File browser
    currentDir: '',
    dirEntries: [],
    allItems: [],
    currentFileList: [],

    // Current file
    currentFile: null,

    // File history (browser-like navigation)
    fileHistory: [],
    fileHistoryIndex: -1,

    // Theme
    theme: 'light',

})

// =============================================
// Project
// =============================================

async function loadProject(): Promise<void> {
    try {
        console.log('[loadProject] 开始加载项目...')
        try {
            const wd = await apiGet<{ watchDir: string; uploadMaxSizeMB: number; uploadMaxFiles: number; chatInitialMessages?: number; chatPageSize?: number; chatCollapsedHeight?: number }>('/api/watch-dir')
            state.watchDir = wd.watchDir || ''
            if (wd.uploadMaxSizeMB > 0) state.uploadMaxSizeMB = wd.uploadMaxSizeMB
            if (wd.uploadMaxFiles > 0) state.uploadMaxFiles = wd.uploadMaxFiles
            if (wd.chatInitialMessages > 0) state.chatInitialMessages = wd.chatInitialMessages
            if (wd.chatPageSize > 0) state.chatPageSize = wd.chatPageSize
            if (wd.chatCollapsedHeight > 0) state.chatCollapsedHeight = wd.chatCollapsedHeight
            console.log('[loadProject] watchDir 加载成功:', state.watchDir)
        } catch (error) {
            console.error('[loadProject] watchDir 加载失败:', error)
        }
        const data = await apiGet<{ path: string }>('/api/project')
        console.log('[loadProject] /api/project 响应:', data)
        if (!data.path) {
            console.warn('[loadProject] 项目路径为空，停止加载')
            return
        }
        state.projectRoot = data.path
        state.projectName = baseName(data.path)
        localStorage.setItem('currentProjectPath', data.path)
        console.log('[loadProject] 项目加载成功:', state.projectRoot, state.projectName)
        // Add to recent projects
        apiPost('/api/recent-projects', { path: data.path }).catch(() => {})
    } catch (error) {
        console.error('[loadProject] 加载项目失败:', error)
    }
}

async function setProject(path: string): Promise<void> {
    await apiPost('/api/project', { path })
    window.location.reload()
}

// =============================================
// File browser
// =============================================

async function loadFiles(dir = ''): Promise<void> {
    try {
        const url = dir ? `/api/dir?path=${encodeURIComponent(dir)}` : '/api/dir?path='
        const data = await apiGet<{ items: DirEntry[] }>(url)
        state.currentDir = dir
        state.dirEntries = data.items || []
        state.allItems = state.dirEntries.slice()
    } catch (err) {
        console.error('Failed to load directory:', err)
    }
}

async function selectFile(path: string, isImageFile = false, isAudioFile = false, addToHistory = true): Promise<void> {
    const key = 'clawbenchLastFile_' + state.projectRoot
    if (key !== 'clawbenchLastFile_') localStorage.setItem(key, path)

    // Add to file history (like browser history)
    if (addToHistory) {
        // If we're not at the end of history, truncate forward history
        if (state.fileHistoryIndex < state.fileHistory.length - 1) {
            state.fileHistory = state.fileHistory.slice(0, state.fileHistoryIndex + 1)
        }
        // Add new path to history (avoid consecutive duplicates)
        if (state.fileHistory[state.fileHistory.length - 1] !== path) {
            state.fileHistory.push(path)
            state.fileHistoryIndex = state.fileHistory.length - 1
        }
    }

    // Detect image files by extension (avoids dynamic import)
    const imageExts = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg', '.bmp', '.ico', '.tiff', '.tif', '.avif', '.pdf']
    const audioExts = ['.mp3', '.wav', '.ogg', '.m4a', '.aac', '.flac', '.wma', '.opus']
    const videoExts = ['.mp4', '.mkv', '.avi', '.mov', '.webm', '.flv', '.wmv', '.m4v', '.3gp', '.m3u8']
    const lower = path.toLowerCase()
    const isImage = isImageFile || imageExts.some(ext => lower.endsWith(ext))
    const isAudio = isAudioFile || audioExts.some(ext => lower.endsWith(ext))
    const isVideo = videoExts.some(ext => lower.endsWith(ext))
    if (isImage) {
        const fileName = baseName(path)
        const isPdf = fileName.toLowerCase().endsWith('.pdf')
        state.currentFile = { name: fileName, path, content: null, isImage: true, isPdf }
        return
    }
    if (isAudio) {
        const fileName = baseName(path)
        state.currentFile = { name: fileName, path, content: null, isAudio: true }
        return
    }
    if (isVideo) {
        const fileName = baseName(path)
        state.currentFile = { name: fileName, path, content: null, isVideo: true }
        return
    }

    try {
        const resp = await fetch(`/api/file/${encodeURIComponent(path)}`)
        if (!resp.ok) {
            const err = await resp.json() as { error?: string }
            if (err.error && err.error.includes('文件过大')) {
                const fileName = baseName(path)
                const sizeInfo = state.dirEntries.find(e => e.name === fileName)
                state.currentFile = { name: fileName, path, content: null, tooLarge: true, size: sizeInfo?.size }
                return
            }
            throw new Error(err.error || 'Failed')
        }
        const data = await resp.json() as CurrentFile
        state.currentFile = data
    } catch (err) {
        state.currentFile = { path, name: baseName(path), error: (err as Error).message }
    }
}

async function deleteFile(filePath: string): Promise<void> {
    if (!confirm(`确定要删除"${baseName(filePath)}"吗？`)) return
    await apiPost('/api/file/delete', { path: filePath })
    if (state.currentFile?.path === filePath) {
        state.currentFile = null
    }
    await loadFiles(state.currentDir)
}

async function renameFile(path: string, newName: string): Promise<void> {
    await apiPost('/api/file/rename', { path, name: newName })
    await loadFiles(state.currentDir)
}

// =============================================
// Navigation
// =============================================

async function navigateToDir(dirPath: string): Promise<void> {
    state.currentDir = dirPath
    await loadFiles(dirPath)
}

async function navigateUp(): Promise<void> {
    if (!state.currentDir) return
    state.currentDir = dirName(state.currentDir)
    await loadFiles(state.currentDir)
}

async function navigateToRoot(): Promise<void> {
    state.currentDir = ''
    await loadFiles('')
}

// =============================================
// File Navigation (Browser-like history)
// =============================================

// Navigate to previous file in history
async function navigateToPrevFile(): Promise<void> {
    if (state.fileHistoryIndex > 0) {
        state.fileHistoryIndex--
        const path = state.fileHistory[state.fileHistoryIndex]
        await selectFile(path, false, false, false)
    }
}

// Navigate to next file in history
async function navigateToNextFile(): Promise<void> {
    if (state.fileHistoryIndex < state.fileHistory.length - 1) {
        state.fileHistoryIndex++
        const path = state.fileHistory[state.fileHistoryIndex]
        await selectFile(path, false, false, false)
    }
}

// Check if can navigate back
function canNavigateBack(): boolean {
    return state.fileHistoryIndex > 0
}

// Check if can navigate forward
function canNavigateForward(): boolean {
    return state.fileHistoryIndex < state.fileHistory.length - 1
}

export const store = {
    state,
    loadProject,
    setProject,
    loadFiles,
    selectFile,
    deleteFile,
    renameFile,
    navigateToDir,
    navigateUp,
    navigateToRoot,
    navigateToNextFile,
    navigateToPrevFile,
    canNavigateBack,
    canNavigateForward,
}
