<script setup lang="ts">
import { onMounted, provide, ref } from 'vue'
import { usePhotoManager } from './composables/usePhotoManager'
import { photoManagerKey } from './composables/useSharedPhotoManager'
import { isDesktop } from './services/desktop'

provide(photoManagerKey, usePhotoManager())
const setupFinished = ref(false)
onMounted(() => { if (!isDesktop()) setupFinished.value = true })
</script>

<template>
  <AISetupScreen v-if="!setupFinished" @continue="setupFinished = true" />
  <NuxtLayout v-else>
    <NuxtPage keepalive />
  </NuxtLayout>
</template>
