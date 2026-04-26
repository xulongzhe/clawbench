import { ref } from 'vue'
import { useToast } from '@/composables/useToast.ts'
import { store } from '@/stores/app.ts'

export function useFileUpload(options) {
  const { inputDisabled } = options
  const toast = useToast()

  const pendingFiles = ref([])
  const attachedFiles = ref([])

  async function handleFileSelect(e) {
    const files = Array.from(e.target.files || [])
    if (files.length === 0) return
    e.target.value = ''

    const maxFiles = store.state.uploadMaxFiles
    const remaining = maxFiles - pendingFiles.value.length
    if (remaining <= 0) {
      toast.show(`最多上传 ${maxFiles} 个文件`, { icon: '⚠️' })
      return
    }

    const toUpload = files.slice(0, remaining)
    if (files.length > remaining) {
      toast.show(`已选择 ${files.length} 个文件，但仅剩 ${remaining} 个名额`, { icon: '⚠️' })
    }

    const maxSizeBytes = store.state.uploadMaxSizeMB * 1024 * 1024

    for (const file of toUpload) {
      if (file.size > maxSizeBytes) {
        toast.show(`${file.name} 超过 ${store.state.uploadMaxSizeMB}MB 限制`, { icon: '⚠️' })
        continue
      }

      const isImage = file.type.startsWith('image/')
      const previewUrl = isImage ? URL.createObjectURL(file) : null

      const formData = new FormData()
      formData.append('file', file)

      try {
        const resp = await fetch('/api/upload/file', {
          method: 'POST',
          body: formData
        })
        const data = await resp.json()
        if (data.ok) {
          pendingFiles.value.push({
            path: data.path,
            previewUrl,
            isImage
          })
        } else {
          if (previewUrl) URL.revokeObjectURL(previewUrl)
          toast.show('上传失败: ' + (data.error || '未知错误'), { icon: '⚠️' })
        }
      } catch (err) {
        if (previewUrl) URL.revokeObjectURL(previewUrl)
        toast.show('上传失败: ' + err.message, { icon: '⚠️' })
      }
    }
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
    removeFile,
    addAttachedFile,
    removeAttachedFile,
    cleanupPreviewUrls,
    clearPendingFiles,
  }
}
