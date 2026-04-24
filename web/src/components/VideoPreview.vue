<template>
  <div class="video-preview-container">
    <div class="video-preview-body">
      <video
        ref="videoRef"
        :src="'/api/local-file/' + encodeURIComponent(file.path)"
        controls
        class="video-player"
        @loadedmetadata="onLoaded"
      >
        您的浏览器不支持视频播放
      </video>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
    file: Object,
})

const videoRef = ref(null)

function onLoaded() {
    // Video is ready to play
}
</script>

<style scoped>
.video-preview-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 0;
}

.video-preview-body {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;
    background: #000;
}

.video-player {
    max-width: 100%;
    max-height: 100%;
    border-radius: var(--radius-sm);
    outline: none;
}
</style>
