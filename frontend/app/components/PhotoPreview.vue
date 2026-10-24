<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = defineProps<{ path: string; name: string; status: string; modifiedAt?: string; detail?: boolean }>()
const failed = ref(false)
const src = computed(() => `/photo-preview?path=${encodeURIComponent(props.path)}&v=${encodeURIComponent(props.modifiedAt ?? '')}`)
watch(() => [src.value, props.status], () => { failed.value = false })
</script>

<template>
  <div class="photo-preview" :class="{ 'photo-preview-detail': detail }">
    <img v-if="status === 'available' && !failed" :src="src" :alt="name" :loading="detail ? 'eager' : 'lazy'" decoding="async" @error="failed = true" />
    <div v-else class="photo-placeholder" :class="{ missing: status === 'missing' }">
      <AppIcon :name="status === 'missing' ? 'info' : 'image'" />
      <small>{{ status === 'missing' ? 'ไม่พบไฟล์จริง' : 'เปิดภาพตัวอย่างไม่ได้ หรือไม่รองรับรูปชนิดนี้' }}</small>
    </div>
  </div>
</template>

<style scoped>
.photo-preview { aspect-ratio: 4 / 3; background: #f0ece3; overflow: hidden; }
.photo-preview img { display: block; width: 100%; height: 100%; object-fit: cover; }
.photo-preview-detail { margin-bottom: 18px; border-radius: 12px; }
.photo-preview-detail img { object-fit: contain; }
.photo-placeholder { height: 100%; padding: 12px; text-align: center; }
</style>
