<template>
  <div class="raw-content-viewer">
    <CodePreview
      :content="file.content"
      :language="fileLanguage"
      :file-path="file.path"
      :editable="true"
      @content-change="file.content = $event"
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import CodePreview from './CodePreview.vue'
import { getFileType } from '@/utils/helpers.ts'

const props = defineProps({
    file: Object,
})

const fileLanguage = computed(() => getFileType(props.file.name)?.lang || 'plaintext')
</script>

<style scoped>
/* scoped only to keep sticky reference local */
</style>
