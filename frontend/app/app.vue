<script setup lang="ts">
import { provide } from 'vue'
import { usePhotoManager } from './composables/usePhotoManager'
import { photoManagerKey } from './composables/useSharedPhotoManager'

provide(photoManagerKey, usePhotoManager())
</script>

<template>
  <NuxtLayout>
    <NuxtPage keepalive />
  </NuxtLayout>
</template>
