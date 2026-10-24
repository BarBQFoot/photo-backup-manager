<script setup lang="ts">
import { useSharedPhotoManager } from '../composables/useSharedPhotoManager'
import { computed, ref, watch } from 'vue'
import { formatBytes, formatDuration } from '../composables/usePhotoManager'

const manager = useSharedPhotoManager()
const state = manager.state
const confirm = ref(false)
const isCopy = computed(() => state.backupMode === 'copy')
const locked = (file: typeof state.sourceFiles[number]) => !isCopy.value && (file.aiStatus === 'analyzing' || state.ai.pendingIds.has(file.fileId!))
const available = computed(() => state.sourceFiles.filter(file => file.fileStatus === 'available' && !locked(file)))
const selectedSet = computed(() => new Set(state.selected))
watch(available, files => {
  const paths = new Set(files.map(file => file.path))
  const selected = state.selected.filter(path => paths.has(path))
  if (selected.length !== state.selected.length) state.selected = selected
})
function toggleFile(path: string, event: Event) {
  const selected = new Set(state.selected)
  if ((event.target as HTMLInputElement).checked) selected.add(path)
  else selected.delete(path)
  state.selected = [...selected]
}
const allSelected = computed(() => available.value.length > 0 && state.selected.length === available.value.length)
const issues = computed(() => state.result?.items.filter(item => item.status !== 'moved' && item.status !== 'copied') ?? [])
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

      <fieldset class="backup-mode" :disabled="!!state.busy || confirm">
        <legend class="field-label">วิธีจัดเก็บรูป</legend>
        <label :class="{ active: !isCopy }"><input v-model="state.backupMode" type="radio" name="backup-mode" value="move" />ย้ายรูป <small>นำไฟล์ออกจากต้นทางเมื่อสำเร็จ</small></label>
        <label :class="{ active: isCopy }"><input v-model="state.backupMode" type="radio" name="backup-mode" value="copy" />สำรองแบบคัดลอก <small>เก็บไฟล์ต้นฉบับไว้ที่เดิม</small></label>
      </fieldset>
      <div class="toolbar">
        <span class="muted"><AppIcon name="shield" /> ข้ามไฟล์ชื่อซ้ำ โดยไม่เขียนทับ</span>
        <button class="button secondary push-right" :disabled="!state.source || !!state.busy" @click="manager.scanSource()"><AppIcon name="search" /> สแกนรูปภาพ</button>
        <button class="button primary" :disabled="!state.source || !state.destination || !state.selected.length || !!state.busy" @click="confirm = true">{{ isCopy ? 'สำรองรูปที่เลือก' : 'ย้ายรูปที่เลือก' }} <span class="button-count">{{ state.selected.length }}</span><AppIcon name="arrow" /></button>
      </div>
    </section>

    <section v-if="state.busy === 'กำลังย้ายรูป' || state.busy === 'กำลังสำรองรูป'" class="surface" aria-live="polite">
      <div class="section-heading"><h2>{{ state.resultMode === 'copy' ? 'กำลังสำรองรูป' : 'กำลังย้ายรูป' }}</h2><span>{{ state.progress.completed }} / {{ state.progress.total }}</span></div>
      <progress :value="state.progress.completed" :max="state.progress.total || 1" />
      <p class="path muted">{{ state.progress.currentPath || 'กำลังเตรียมไฟล์…' }}</p>
    </section>

    <section v-if="state.result" class="surface" aria-live="polite">
      <div class="section-heading"><h2>{{ state.resultMode === 'copy' ? 'ผลการสำรองรูป' : 'ผลการย้ายรูป' }}</h2><span class="muted">{{ formatDuration(state.result.durationMs) }}</span></div>
      <div class="result-counts"><span>สำเร็จ {{ state.result.successCount }}</span><span>ข้าม {{ state.result.skippedCount }}</span><span>ผิดพลาด {{ state.result.failedCount }}</span></div>
      <ul v-if="issues.length" class="result-list">
        <li v-for="item in issues" :key="item.sourcePath"><span class="path">{{ item.sourcePath }}</span><span>{{ item.status === 'skipped' ? 'ข้าม: ไฟล์ซ้ำ' : `ล้มเหลว: ${item.error?.message || 'จัดเก็บไม่ได้'}` }}</span></li>
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
      <div v-else class="file-list">
        <label v-for="file in state.sourceFiles" :key="file.path" v-memo="[file, selectedSet.has(file.path), locked(file), isCopy, !!state.busy]" class="file-row source-photo-row">
          <input :checked="selectedSet.has(file.path)" @change="toggleFile(file.path, $event)" type="checkbox" :value="file.path" :disabled="file.fileStatus !== 'available' || locked(file) || !!state.busy" :aria-label="`${isCopy ? 'เลือกสำรอง' : 'เลือกย้าย'} ${file.fileName}`" />
          <PhotoPreview class="source-thumbnail" :path="file.path" :name="file.fileName" :status="file.fileStatus" :modified-at="file.modifiedAt" />
          <span class="file-info"><strong>{{ file.fileName }}</strong><small class="path source-file-path">{{ file.path }}</small></span>
          <span class="muted">{{ formatBytes(file.sizeBytes) }}</span>
          <span class="badge" :class="{ warning: file.fileStatus === 'missing' }">{{ file.fileStatus === 'missing' ? 'ไฟล์หาย' : locked(file) ? 'รอวิเคราะห์เสร็จก่อนย้าย' : isCopy ? 'พร้อมสำรอง' : 'พร้อมย้าย' }}</span>
        </label>
      </div>
    </section>

    <ConfirmDialog v-if="confirm" :title="isCopy ? 'ยืนยันการสำรองรูป' : 'ยืนยันการย้ายรูป'" :confirm-label="isCopy ? 'ยืนยันสำรองรูป' : 'ยืนยันย้ายรูป'" @cancel="confirm = false" @confirm="move">
      <p>{{ isCopy ? 'สำรองรูปที่เลือก' : 'ย้ายรูปที่เลือก' }} <strong>{{ state.selected.length }} ไฟล์</strong></p>
      <p>จาก: <span class="path">{{ state.source }}</span></p>
      <p>ไป: <span class="path">{{ state.destination }}</span></p>
      <p class="notice" :class="isCopy ? 'neutral' : 'warning'">{{ isCopy ? 'คัดลอกไฟล์ไปยังปลายทาง โดยเก็บต้นฉบับไว้ที่เดิม' : 'เมื่อย้ายสำเร็จ ไฟล์จะถูกนำออกจากโฟลเดอร์ต้นทาง' }}</p>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.backup-mode { display: flex; flex-wrap: wrap; gap: 12px; border: 0; padding: 0; margin: 24px 0; }
.backup-mode legend { margin-bottom: 10px; }
.backup-mode label { flex: 1 1 240px; display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 14px; border: 1px solid var(--line); border-radius: 12px; cursor: pointer; }
.backup-mode label.active { background: var(--accent-soft); border-color: var(--accent); }
.backup-mode small { flex-basis: 100%; color: var(--muted); margin-left: 24px; }
.backup-mode:disabled label { opacity: .6; cursor: default; }
.source-thumbnail { width: 120px; flex-shrink: 0; border-radius: 8px; }
.source-file-path { white-space: normal; overflow-wrap: anywhere; }
@media (max-width: 760px) {
  .source-thumbnail { width: 88px; }
}
</style>
