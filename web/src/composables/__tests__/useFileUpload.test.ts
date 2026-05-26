import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useFileUpload } from '@/composables/useFileUpload'

// Mock dependencies
vi.mock('@/composables/useToast.ts', () => ({
  useToast: () => ({
    show: vi.fn(),
  }),
}))

vi.mock('@/composables/useLocale', () => ({
  gt: (key: string) => key,
}))

vi.mock('@/stores/app.ts', () => ({
  store: {
    state: {
      uploadMaxFiles: 10,
      uploadMaxSizeMB: 10,
    },
  },
}))

describe('useFileUpload', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('exposes all expected refs and functions', () => {
    const upload = useFileUpload()
    expect(upload.pendingFiles).toBeDefined()
    expect(upload.attachedFiles).toBeDefined()
    expect(upload.dirUploading).toBeDefined()
    expect(upload.dirUploadProgress).toBeDefined()
    expect(upload.dirUploadTotal).toBeDefined()
    expect(upload.dirUploadDone).toBeDefined()
    expect(typeof upload.handleFileSelect).toBe('function')
    expect(typeof upload.handleFileDrop).toBe('function')
    expect(typeof upload.handleFileSelectToDir).toBe('function')
    expect(typeof upload.handleFileDropToDir).toBe('function')
    expect(typeof upload.removeFile).toBe('function')
    expect(typeof upload.addAttachedFile).toBeDefined()
    expect(typeof upload.removeAttachedFile).toBe('function')
    expect(typeof upload.cleanupPreviewUrls).toBe('function')
    expect(typeof upload.clearPendingFiles).toBe('function')
    expect(typeof upload.uploadFilesToDir).toBe('function')
  })

  it('dirUploading starts as false', () => {
    const upload = useFileUpload()
    expect(upload.dirUploading.value).toBe(false)
  })

  it('dirUploadProgress starts as 0', () => {
    const upload = useFileUpload()
    expect(upload.dirUploadProgress.value).toBe(0)
  })

  it('dirUploadTotal starts as 0', () => {
    const upload = useFileUpload()
    expect(upload.dirUploadTotal.value).toBe(0)
  })

  it('dirUploadDone starts as 0', () => {
    const upload = useFileUpload()
    expect(upload.dirUploadDone.value).toBe(0)
  })

  it('pendingFiles starts empty', () => {
    const upload = useFileUpload()
    expect(upload.pendingFiles.value).toHaveLength(0)
  })

  it('attachedFiles starts empty', () => {
    const upload = useFileUpload()
    expect(upload.attachedFiles.value).toHaveLength(0)
  })

  it('addAttachedFile adds a file path', () => {
    const upload = useFileUpload()
    upload.addAttachedFile('/some/path.txt')
    expect(upload.attachedFiles.value).toContain('/some/path.txt')
  })

  it('addAttachedFile does not add duplicates', () => {
    const upload = useFileUpload()
    upload.addAttachedFile('/some/path.txt')
    upload.addAttachedFile('/some/path.txt')
    expect(upload.attachedFiles.value).toHaveLength(1)
  })

  it('removeAttachedFile removes by index', () => {
    const upload = useFileUpload()
    upload.addAttachedFile('/a.txt')
    upload.addAttachedFile('/b.txt')
    upload.removeAttachedFile(0)
    expect(upload.attachedFiles.value).toHaveLength(1)
    expect(upload.attachedFiles.value[0]).toBe('/b.txt')
  })

  it('clearPendingFiles empties the array', () => {
    const upload = useFileUpload()
    // Manually add an entry to test clearing
    upload.pendingFiles.value.push({ path: '', previewUrl: null, isImage: false, uploading: false, progress: 0 })
    expect(upload.pendingFiles.value).toHaveLength(1)
    upload.clearPendingFiles()
    expect(upload.pendingFiles.value).toHaveLength(0)
  })

  describe('handleFileSelectToDir', () => {
    it('does nothing when no files selected', async () => {
      const upload = useFileUpload()
      const mockEvent = {
        target: {
          files: [],
          value: 'fake',
        },
      }
      await upload.handleFileSelectToDir(mockEvent as any, '/some/dir')
      // No XHR should be made, dirUploading should remain false
      expect(upload.dirUploading.value).toBe(false)
    })

    it('resets the input value after selection', async () => {
      const upload = useFileUpload()
      const mockEvent = {
        target: {
          files: [],
          value: 'fake',
        },
      }
      await upload.handleFileSelectToDir(mockEvent as any, '/some/dir')
      expect(mockEvent.target.value).toBe('')
    })
  })

  describe('handleFileDropToDir', () => {
    it('does nothing when empty file list', async () => {
      const upload = useFileUpload()
      await upload.handleFileDropToDir([], '/some/dir')
      expect(upload.dirUploading.value).toBe(false)
    })
  })

  describe('uploadFilesToDir', () => {
    it('is aliased to uploadFiles function', () => {
      const upload = useFileUpload()
      expect(typeof upload.uploadFilesToDir).toBe('function')
    })
  })
})
