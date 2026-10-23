<script setup lang="ts">
import { useSharedPhotoManager } from '../composables/useSharedPhotoManager'
import { computed, ref } from 'vue'
import { formatBytes, formatDuration } from '../composables/usePhotoManager'

const manager = useSharedPhotoManager()
const state = manager.state
const confirm = ref(false)
const available = computed(() => state.sourceFiles.filter(file => file.fileStatus === 'available'))
const allSelected = computed(() => available.value.length > 0 && state.selected.length === available.value.length)
const issues = computed(() => state.result?.items.filter(item => item.status !== 'moved') ?? [])
const folders = [
  { kind: 'source', title: 'เริ่มจากที่นี่', label: 'โฟลเดอร์ต้นทาง', button: 'เลือกต้นทาง', number: '01' },
  { kind: 'destination', title: 'เก็บไว้ที่นี่', label: 'โฟลเดอร์ปลายทาง', button: 'เลือกปลายทาง', number: '02' },
] as const

function toggleAll() {
  state.selected = allSelected.value ? [] : available.value.map(file => file.path)
}
function move() {
  confirm.value = false
  void manager.move()
}
</script>

<template>
  <div>
    <section class="transfer-panel surface">
      <div class="section-heading">
        <div><h2>วันนี้ จะจัดเก็บรูปจากที่ไหน?</h2><p>เลือกต้นทางและปลายทาง แล้วเริ่มสแกนรูปได้เลย</p></div>
        <span class="badge subtle"><AppIcon name="folder" /> บนเครื่องของคุณ</span>
      </div>

      <div class="folder-grid">
        <button
          v-for="folder in folders"
          :key="folder.kind"
          class="folder-card"
          :class="{ filled: state[folder.kind] }"
          :disabled="!!state.busy"
          @click="manager.choose(folder.kind)"
        >
          <span class="folder-card-top"><span class="folder-icon"><AppIcon name="folder" /></span><span class="folder-number">{{ folder.number }}</span></span>
          <span class="folder-title">{{ folder.title }}<small>{{ folder.label }}</small></span>
          <span class="path folder-path">{{ state[folder.kind] || 'ยังไม่ได้เลือกโฟลเดอร์' }}</span>
          <span class="folder-action">{{ state[folder.kind] ? 'เปลี่ยนโฟลเดอร์' : folder.button }}<AppIcon name="arrow" /></span>
        </button>
      </div>

      <div class="toolbar">
        <span class="muted"><AppIcon name="shield" /> ข้ามไฟล์ชื่อซ้ำ โดยไม่เขียนทับ</span>
        <button class="button secondary push-right" :disabled="!state.source || !!state.busy" @click="manager.scanSource()"><AppIcon name="search" /> สแกนรูปภาพ</button>
        <button class="button primary" :disabled="!state.source || !state.destination || !state.selected.length || !!state.busy" @click="confirm = true">ย้ายรูปที่เลือก <span class="button-count">{{ state.selected.length }}</span><AppIcon name="arrow" /></button>
      </div>
    </section>

    <section v-if="state.busy === 'กำลังย้ายรูป'" class="surface" aria-live="polite">
      <div class="section-heading"><h2>กำลังย้ายรูป</h2><span>{{ state.progress.completed }} / {{ state.progress.total }}</span></div>
      <progress :value="state.progress.completed" :max="state.progress.total || 1" />
      <p class="path muted">{{ state.progress.currentPath || 'กำลังเตรียมไฟล์…' }}</p>
    </section>

    <section v-if="state.result" class="surface" aria-live="polite">
      <div class="section-heading"><h2>ผลการย้ายรูป</h2><span class="muted">{{ formatDuration(state.result.durationMs) }}</span></div>
      <div class="result-counts"><span>สำเร็จ {{ state.result.successCount }}</span><span>ข้าม {{ state.result.skippedCount }}</span><span>ผิดพลาด {{ state.result.failedCount }}</span></div>
      <ul v-if="issues.length" class="result-list">
        <li v-for="item in issues" :key="item.sourcePath"><span class="path">{{ item.sourcePath }}</span><span>{{ item.status === 'skipped' ? 'ข้าม: ไฟล์ซ้ำ' : `ล้มเหลว: ${item.error?.message || 'ย้ายไม่ได้'}` }}</span></li>
      </ul>
    </section>

    <section class="surface no-padding">
      <div class="section-heading padded">
        <div class="inline-heading"><h2>รูปภาพในต้นทาง</h2><span class="count">{{ state.sourceFiles.length }} ไฟล์</span></div>
        <button class="text-button" :disabled="!available.length || !!state.busy" @click="toggleAll">{{ allSelected ? 'ยกเลิกทั้งหมด' : 'เลือกทั้งหมด' }}</button>
      </div>
      <div v-if="!state.sourceFiles.length" class="empty-state">
        <span class="empty-icon"><AppIcon name="image" /></span>
        <h3>{{ state.sourceScanned ? 'โฟลเดอร์นี้ยังไม่มีรูปภาพ' : 'รูปภาพของคุณจะอยู่ตรงนี้' }}</h3>
        <p>{{ state.sourceScanned ? 'ลองเลือกโฟลเดอร์อื่น แล้วสแกนอีกครั้ง' : 'เลือกโฟลเดอร์ต้นทาง แล้วกดสแกนรูปภาพเพื่อเริ่มต้น' }}</p>
      </div>
      <div v-else v-memo="[state.sourceFiles, state.selected, !!state.busy]" class="file-list">
        <label v-for="file in state.sourceFiles" :key="file.path" class="file-row source-photo-row">
          <input v-model="state.selected" type="checkbox" :value="file.path" :disabled="file.fileStatus !== 'available' || !!state.busy" :aria-label="`เลือกย้าย ${file.fileName}`" />
          <PhotoPreview class="source-thumbnail" :path="file.path" :name="file.fileName" :status="file.fileStatus" :modified-at="file.modifiedAt" />
          <span class="file-info"><strong>{{ file.fileName }}</strong><small class="path source-file-path">{{ file.path }}</small></span>
          <span class="muted">{{ formatBytes(file.sizeBytes) }}</span>
          <span class="badge" :class="{ warning: file.fileStatus === 'missing' }">{{ file.fileStatus === 'missing' ? 'ไฟล์หาย' : 'พร้อมย้าย' }}</span>
        </label>
      </div>
    </section>

    <ConfirmDialog v-if="confirm" title="ยืนยันการย้ายรูป" confirm-label="ยืนยันย้ายรูป" @cancel="confirm = false" @confirm="move">
      <p>ย้ายรูปที่เลือก <strong>{{ state.selected.length }} ไฟล์</strong></p>
      <p>จาก: <span class="path">{{ state.source }}</span></p>
      <p>ไป: <span class="path">{{ state.destination }}</span></p>
      <p class="notice warning">เมื่อย้ายสำเร็จ ไฟล์จะถูกนำออกจากโฟลเดอร์ต้นทาง</p>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.source-thumbnail { width: 120px; flex-shrink: 0; border-radius: 8px; }
.source-file-path { white-space: normal; overflow-wrap: anywhere; }
@media (max-width: 760px) {
  .source-thumbnail { width: 88px; }
}
</style>
