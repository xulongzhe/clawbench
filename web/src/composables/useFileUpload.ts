import { ref } from 'vue'
import { useToast } from '@/composables/useToast.ts'
import { store } from '@/stores/app.ts'

export function useFileUpload(options) {
  const { inputDisabled } = options
  const toast = useToast()

  const pendingFiles = ref([])
  const attachedFiles = ref([])

  function uploadOneFile(file) {
    return new Promise((resolve) => {
      const isImage = file.type.startsWith('image/')
      const previewUrl = isImage ? URL.createObjectURL(file) : null

      // Push entry then get reactive proxy from array
      const idx = pendingFiles.value.length
      pendingFiles.value.push({
        path: '',
        previewUrl,
        isImage,
        uploading: true,
        progress: 0,
      })
      const entry = pendingFiles.value[idx]

      const formData = new FormData()
      formData.append('file', file)

      const xhr = new XMLHttpRequest()
      xhr.open('POST', '/api/upload/file')
      xhr.timeout = 300000

      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          entry.progress = Math.round((e.loaded / e.total) * 100)
        }
      }

      xhr.onload = () => {
        entry.uploading = false
        entry.progress = 100
        try {
          const data = JSON.parse(xhr.responseText)
          if (data.ok) {
            entry.path = data.path
            resolve(true)
          } else {
            if (previewUrl) URL.revokeObjectURL(previewUrl)
            const i = pendingFiles.value.indexOf(entry)
            if (i !== -1) pendingFiles.value.splice(i, 1)
            toast.show('上传失败: ' + (data.error || '未知错误'), { icon: '⚠️', type: 'error' })
            resolve(false)
          }
        } catch {
          if (previewUrl) URL.revokeObjectURL(previewUrl)
          const i = pendingFiles.value.indexOf(entry)
          if (i !== -1) pendingFiles.value.splice(i, 1)
          toast.show('上传失败: 响应解析错误', { icon: '⚠️', type: 'error' })
          resolve(false)
        }
      }

      xhr.onerror = () => {
        entry.uploading = false
        if (previewUrl) URL.revokeObjectURL(previewUrl)
        const i = pendingFiles.value.indexOf(entry)
        if (i !== -1) pendingFiles.value.splice(i, 1)
        toast.show('上传失败: 网络错误', { icon: '⚠️', type: 'error' })
        resolve(false)
      }

      xhr.ontimeout = () => {
        entry.uploading = false
        if (previewUrl) URL.revokeObjectURL(previewUrl)
        const i = pendingFiles.value.indexOf(entry)
        if (i !== -1) pendingFiles.value.splice(i, 1)
        toast.show('上传超时，请重试', { icon: '⚠️', type: 'error' })
        resolve(false)
      }

      xhr.send(formData)
    })
  }

  async function uploadFiles(files) {
    const maxFiles = store.state.uploadMaxFiles
    const currentCount = pendingFiles.value.filter(f => !f.uploading).length
    const remaining = maxFiles - currentCount
    if (remaining <= 0) {
      toast.show(`最多上传 ${maxFiles} 个文件`, { icon: '⚠️', type: 'error' })
      return
    }

    const toUpload = files.slice(0, remaining)
    if (files.length > remaining) {
      toast.show(`已选择 ${files.length} 个文件，但仅剩 ${remaining} 个名额`, { icon: '⚠️', type: 'error' })
    }

    const maxSizeBytes = store.state.uploadMaxSizeMB * 1024 * 1024

    for (const file of toUpload) {
      if (file.size > maxSizeBytes) {
        toast.show(`${file.name} 超过 ${store.state.uploadMaxSizeMB}MB 限制`, { icon: '⚠️', type: 'error' })
        continue
      }
      await uploadOneFile(file)
    }
  }

  async function handleFileSelect(e) {
    const files = Array.from(e.target.files || [])
    if (files.length === 0) return
    e.target.value = ''
    await uploadFiles(files)
  }

  async function handleFileDrop(files) {
    if (files.length === 0) return
    await uploadFiles(files)
  }

  function removeFile(index) {
    const f = pendingFiles.value[index]
    if (f?.previewUrl) {
      URL.revokeObjectURL(f.previewUrl)
    }
    pendingFiles.value.splice(index, 1)
  }

  function addAttachedFile(filePath) {
    if (filePath && !attachedFiles.value.includes(filePath)) {
      attachedFiles.value.push(filePath)
    }
  }

  function removeAttachedFile(index) {
    attachedFiles.value.splice(index, 1)
  }

  function cleanupPreviewUrls() {
    pendingFiles.value.forEach(f => {
      if (f.previewUrl) URL.revokeObjectURL(f.previewUrl)
    })
  }

  function clearPendingFiles() {
    cleanupPreviewUrls()
    pendingFiles.value = []
  }

  return {
    pendingFiles,
    attachedFiles,
    handleFileSelect,
    handleFileDrop,
    removeFile,
    addAttachedFile,
    removeAttachedFile,
    cleanupPreviewUrls,
    clearPendingFiles,
  }
}
