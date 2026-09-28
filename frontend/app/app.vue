<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { SelectFolder } from '../wailsjs/go/main/App'
import { ScanDrive, StartBackup, GetBackupProgress } from '../wailsjs/go/service/DriveService'

type ScannedFile = {
  fileName: string
  path: string
  sizeBytes: number
  fileStatus: string
}

type BackupItem = {
  sourcePath: string
  destinationPath?: string
  status: string
  error?: { message: string; code: string }
}

type BackupResult = {
  totalFiles: number
  successCount: number
  skippedCount: number
  failedCount: number
  items: BackupItem[]
}

type Progress = { completed: number; total: number; currentPath?: string; status: string }

const source = ref('')
const destination = ref('')
const files = ref<ScannedFile[]>([])
const selected = ref<string[]>([])
const busy = ref(false)
const errorMessage = ref('')
const result = ref<BackupResult | null>(null)
const progress = ref<Progress>({ completed: 0, total: 0, status: 'idle' })
const scanCount = computed(() => files.value.length)
const allSelected = computed(() => files.value.length > 0 && selected.value.length === files.value.length)
let stopProgressListener: (() => void) | undefined

onMounted(async () => {
  try {
    progress.value = await GetBackupProgress()
  } catch {
    // The Wails runtime may not be ready while the page is mounting.
  }
  stopProgressListener = EventsOn('backup:progress', (next: Progress) => {
    progress.value = next
  })
})

onUnmounted(() => stopProgressListener?.())

async function chooseFolder(kind: 'source' | 'destination') {
  errorMessage.value = ''
  try {
    const folder = await SelectFolder(kind === 'source' ? 'เลือกโฟลเดอร์ต้นทาง' : 'เลือกโฟลเดอร์ปลายทาง')
    if (folder) {
      if (kind === 'source') {
        source.value = folder
        files.value = []
        selected.value = []
        result.value = null
      } else {
        destination.value = folder
      }
    }
  } catch (error) {
    errorMessage.value = errorText(error)
  }
}

async function scanSource() {
  if (!source.value) return
  busy.value = true
  errorMessage.value = ''
  result.value = null
  try {
    files.value = await ScanDrive(source.value)
    selected.value = files.value.filter(file => file.fileStatus === 'available').map(file => file.path)
    if (files.value.length === 0) errorMessage.value = 'ไม่พบไฟล์รูปในโฟลเดอร์นี้'
  } catch (error) {
    errorMessage.value = errorText(error)
  } finally {
    busy.value = false
  }
}

function toggleAll() {
  selected.value = allSelected.value
    ? []
    : files.value.filter(file => file.fileStatus === 'available').map(file => file.path)
}

function toggleFile(path: string) {
  selected.value = selected.value.includes(path)
    ? selected.value.filter(selectedPath => selectedPath !== path)
    : [...selected.value, path]
}

async function startMove() {
  if (!source.value || !destination.value || selected.value.length === 0 || busy.value) return
  const approved = window.confirm(`ย้ายรูป ${selected.value.length} ไฟล์\nจาก: ${source.value}\nไป: ${destination.value}\n\nไฟล์ที่ย้ายสำเร็จจะถูกนำออกจากโฟลเดอร์ต้นทาง`)
  if (!approved) return

  busy.value = true
  errorMessage.value = ''
  try {
    const backupResult = await StartBackup({
      sourcePath: source.value,
      destinationPath: destination.value,
      filePaths: [...selected.value],
    })
    await scanSource()
    result.value = backupResult
  } catch (error) {
    errorMessage.value = errorText(error)
  } finally {
    busy.value = false
  }
}

function errorText(error: unknown): string {
  if (typeof error === 'string') return error
  if (error instanceof Error) return error.message
  return 'เกิดข้อผิดพลาด กรุณาลองใหม่'
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
</script>

<template>
  <main class="page">
    <section class="panel">
      <header class="header">
        <div class="brand-icon">▧</div>
        <div>
          <p class="eyebrow">PHOTO BACKUP MANAGER</p>
          <h1>ย้ายรูปภาพ</h1>
          <p class="subtitle">เลือกโฟลเดอร์ต้นทางและปลายทาง จากนั้นเลือกภาพที่ต้องการย้าย</p>
        </div>
      </header>

      <div class="folder-grid">
        <div class="folder-card">
          <span class="label">โฟลเดอร์ต้นทาง</span>
          <div class="folder-row">
            <span class="path" :title="source">{{ source || 'ยังไม่ได้เลือกโฟลเดอร์' }}</span>
            <button class="button secondary" :disabled="busy" @click="chooseFolder('source')">เลือกโฟลเดอร์</button>
          </div>
        </div>
        <div class="folder-card">
          <span class="label">โฟลเดอร์ปลายทาง</span>
          <div class="folder-row">
            <span class="path" :title="destination">{{ destination || 'ยังไม่ได้เลือกโฟลเดอร์' }}</span>
            <button class="button secondary" :disabled="busy" @click="chooseFolder('destination')">เลือกโฟลเดอร์</button>
          </div>
        </div>
      </div>

      <div class="actions">
        <button class="button primary" :disabled="!source || busy" @click="scanSource">
          {{ busy ? 'กำลังทำงาน…' : 'สแกนรูปภาพ' }}
        </button>
        <button class="button move" :disabled="!destination || !selected.length || busy" @click="startMove">
          ย้ายรูปที่เลือก ({{ selected.length }})
        </button>
      </div>

      <div v-if="progress.total > 0 && progress.status === 'moving'" class="progress-wrap">
        <div class="progress-label"><span>กำลังย้าย {{ progress.completed }} / {{ progress.total }}</span><span>{{ progress.currentPath?.split(/[\\/]/).pop() }}</span></div>
        <progress class="progress" :value="progress.completed" :max="progress.total" />
      </div>

      <div v-if="errorMessage" class="notice error">{{ errorMessage }}</div>

      <div v-if="result" class="notice success">
        เสร็จแล้ว: ย้าย {{ result.successCount }} ไฟล์ · ข้าม {{ result.skippedCount }} ไฟล์ · ผิดพลาด {{ result.failedCount }} ไฟล์
      </div>

      <section class="files-section">
        <div class="list-header">
          <div>
            <h2>รูปภาพในต้นทาง</h2>
            <p>{{ scanCount }} รูปภาพ{{ source ? ` · ${source}` : '' }}</p>
          </div>
          <button class="select-all" :disabled="!files.length || busy" @click="toggleAll">
            {{ allSelected ? 'ยกเลิกทั้งหมด' : 'เลือกทั้งหมด' }}
          </button>
        </div>

        <div v-if="!files.length" class="empty">
          <div class="empty-icon">▧</div>
          <strong>ยังไม่มีรายการรูปภาพ</strong>
          <span>เลือกโฟลเดอร์ต้นทางแล้วกด “สแกนรูปภาพ”</span>
        </div>

        <div v-else class="file-list">
          <label v-for="file in files" :key="file.path" class="file-row" :class="{ disabled: file.fileStatus !== 'available' }">
            <input
              type="checkbox"
              :checked="selected.includes(file.path)"
              :disabled="busy || file.fileStatus !== 'available'"
              @change="toggleFile(file.path)"
            />
            <span class="image-icon">▧</span>
            <span class="file-info"><strong>{{ file.fileName }}</strong><small :title="file.path">{{ file.path }}</small></span>
            <span class="size">{{ formatBytes(file.sizeBytes) }}</span>
            <span class="status" :class="file.fileStatus">{{ file.fileStatus === 'available' ? 'พร้อมย้าย' : file.fileStatus }}</span>
          </label>
        </div>
      </section>

      <div class="footer-note">โปรดทดลองกับสำเนารูปก่อน การย้ายจะนำไฟล์ออกจากโฟลเดอร์ต้นทางเมื่อสำเร็จ</div>
    </section>
  </main>
</template>

<style>
:root { font-family: Inter, "Noto Sans Thai", "Segoe UI", sans-serif; color: #1d3038; background: #f2f6f7; font-synthesis: none; }
* { box-sizing: border-box; }
body { margin: 0; min-width: 360px; min-height: 100vh; }
button { font: inherit; }
.page { min-height: 100vh; padding: 36px 24px; }
.panel { max-width: 1040px; margin: 0 auto; padding: 32px; border: 1px solid #d9e4e8; border-radius: 18px; background: #fff; box-shadow: 0 12px 36px #233d4910; }
.header { display: flex; align-items: center; gap: 17px; margin-bottom: 30px; }
.brand-icon { display: grid; place-items: center; width: 52px; height: 52px; border-radius: 14px; background: #e4f2f4; color: #287989; font-size: 30px; }
.eyebrow { margin: 0 0 5px; color: #2a7d8d; font-size: 11px; font-weight: 800; letter-spacing: .12em; }
h1 { margin: 0; font-size: 27px; letter-spacing: -.03em; }
.subtitle { margin: 5px 0 0; color: #72848b; font-size: 14px; }
.folder-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.folder-card { min-width: 0; padding: 17px; border: 1px solid #e0e9ec; border-radius: 12px; background: #fbfdfd; }
.label { display: block; margin-bottom: 10px; color: #667a82; font-size: 12px; font-weight: 700; }
.folder-row { display: flex; align-items: center; gap: 10px; }
.path { flex: 1; min-width: 0; overflow: hidden; color: #314951; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.button { min-height: 39px; padding: 0 15px; border: 0; border-radius: 8px; font-weight: 700; cursor: pointer; transition: .15s ease; }
.button:disabled { cursor: not-allowed; opacity: .48; }
.secondary { flex: 0 0 auto; border: 1px solid #c9dce1; background: #fff; color: #276e7d; }
.secondary:hover:not(:disabled), .select-all:hover:not(:disabled) { background: #edf7f8; }
.actions { display: flex; gap: 10px; margin: 18px 0 20px; }
.primary { background: #267889; color: #fff; }
.primary:hover:not(:disabled), .move:hover:not(:disabled) { background: #1b6473; }
.move { background: #263e48; color: #fff; }
.progress-wrap { margin: 0 0 16px; padding: 13px 15px; border-radius: 9px; background: #eff7f8; }
.progress-label { display: flex; justify-content: space-between; gap: 15px; margin-bottom: 8px; color: #52717a; font-size: 12px; }
.progress { width: 100%; height: 8px; accent-color: #287e8f; }
.notice { margin: 0 0 16px; padding: 12px 14px; border-radius: 9px; font-size: 13px; }
.error { background: #fff0ef; color: #a23d35; }
.success { background: #eaf7f0; color: #26744d; }
.files-section { margin-top: 8px; border: 1px solid #e0e9ec; border-radius: 12px; overflow: hidden; }
.list-header { display: flex; align-items: center; justify-content: space-between; padding: 17px 19px; border-bottom: 1px solid #e6edef; }
h2 { margin: 0; font-size: 16px; }
.list-header p { margin: 4px 0 0; max-width: 650px; overflow: hidden; color: #809198; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.select-all { padding: 8px 11px; border: 1px solid #d2e1e4; border-radius: 7px; background: #fff; color: #367887; font-size: 12px; font-weight: 700; cursor: pointer; }
.select-all:disabled { opacity: .45; cursor: not-allowed; }
.empty { display: flex; min-height: 210px; flex-direction: column; align-items: center; justify-content: center; gap: 8px; color: #819198; font-size: 13px; }
.empty-icon { color: #b7cbd0; font-size: 40px; }
.empty strong { color: #4d636b; }
.file-list { max-height: 430px; overflow: auto; }
.file-row { display: flex; align-items: center; gap: 13px; min-height: 64px; padding: 9px 18px; border-bottom: 1px solid #edf1f2; cursor: pointer; }
.file-row:last-child { border-bottom: 0; }
.file-row:hover { background: #f8fbfb; }
.file-row.disabled { opacity: .55; cursor: default; }
.file-row input { width: 16px; height: 16px; accent-color: #287e8f; }
.image-icon { display: grid; place-items: center; width: 36px; height: 36px; border-radius: 8px; background: #eef5f6; color: #478290; font-size: 21px; }
.file-info { display: flex; min-width: 0; flex: 1; flex-direction: column; gap: 4px; }
.file-info strong { overflow: hidden; color: #30474f; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.file-info small { overflow: hidden; color: #87969b; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.size { width: 70px; color: #64777e; font-size: 12px; text-align: right; }
.status { min-width: 75px; padding: 5px 8px; border-radius: 20px; background: #e8f5ed; color: #28754d; font-size: 10px; font-weight: 700; text-align: center; }
.status.missing { background: #fff4df; color: #9d6b18; }
.footer-note { margin-top: 17px; color: #809198; font-size: 11px; }
@media (max-width: 720px) { .page { padding: 14px; } .panel { padding: 19px; } .folder-grid { grid-template-columns: 1fr; } .folder-row { align-items: flex-start; flex-direction: column; } .path { width: 100%; } .actions { flex-wrap: wrap; } .size { display: none; } }
</style>
