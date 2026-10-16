<script setup lang="ts">
import { onMounted, ref } from 'vue'
defineProps<{ title: string; confirmLabel: string; danger?: boolean }>()
const emit = defineEmits<{ confirm: []; cancel: [] }>()
const dialog = ref<HTMLDialogElement>()
let previousFocus: HTMLElement | null = null
onMounted(() => {
  previousFocus = document.activeElement as HTMLElement | null
  dialog.value?.showModal()
})
function finish(confirmed: boolean) {
  dialog.value?.close()
  previousFocus?.focus()
  if (confirmed) emit('confirm'); else emit('cancel')
}
</script>
<template>
  <dialog ref="dialog" class="confirm-dialog" aria-labelledby="confirm-title" @cancel.prevent="finish(false)">
    <p class="eyebrow">โปรดยืนยันก่อนดำเนินการ</p>
    <h2 id="confirm-title">{{ title }}</h2>
    <div class="dialog-content"><slot /></div>
    <div class="dialog-actions">
      <button autofocus class="button secondary" @click="finish(false)">ยกเลิก</button>
      <button class="button" :class="danger ? 'danger' : 'primary'" @click="finish(true)">{{ confirmLabel }}</button>
    </div>
  </dialog>
</template>
