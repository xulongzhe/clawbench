<template>
  <div class="image-preview-container">
    <div v-if="file.isPdf" class="image-preview-body pdf-preview-body">
      <embed :src="'/api/local-file/' + encodeURIComponent(file.path)" type="application/pdf" class="pdf-preview-embed" />
    </div>
    <div v-else class="image-preview-body">
      <img :src="'/api/local-file/' + encodeURIComponent(file.path)" :alt="file.name" class="image-preview-img" />
    </div>
  </div>
</template>

<script setup>
defineProps({
    file: Object,
})
</script>

<style scoped>
.image-preview-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 0;
}

.image-preview-body {
    flex: 1;
    overflow: auto;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
    background: var(--bg-primary);
}

.image-preview-img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    border-radius: var(--radius-sm);
    cursor: zoom-in;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}

:global([data-theme="dark"]) .image-preview-img {
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.3);
}

.pdf-preview-body {
    flex: 1;
    overflow: auto;
    padding: 0;
    background: #525659;
}

.pdf-preview-embed {
    width: 100%;
    height: 100%;
    min-height: 80vh;
    border: none;
}
</style>
