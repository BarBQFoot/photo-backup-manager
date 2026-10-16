<script setup lang="ts">
import { useSharedPhotoManager } from '../composables/useSharedPhotoManager'
import { computed, ref } from 'vue'
import { formatBytes, formatDate, formatDuration } from '../composables/usePhotoManager'
import type { DriveFile } from '../services/desktop'

const manager = useSharedPhotoManager()
const state = manager.state
const deleting = ref<DriveFile | null>(null)
const missing = computed(() => state.files.filter(file => file.fileStatus === 'missing').length)
const stats = computed(() => [
  { label: 'การย้ายที่ผ่านมา', value: state.jobs.length, unit: 'ครั้ง' },
  { label: 'ไฟล์ที่อยู่ครบ', value: state.files.length - missing.value, unit: 'ไฟล์' },
  { label: 'ไฟล์ที่หาไม่พบ', value: missing.value, unit: 'ไฟล์' },
])
const statusLabels: Record<string, string> = {
  completed: 'เสร็จสิ้น', partial: 'สำเร็จบางส่วน', failed: 'ล้มเหลว',
  running: 'กำลังทำงาน', interrupted: 'ถูกขัดจังหวะ',
}

function deleteDisabled(file: DriveFile) {
  return !file.fileId || file.fileStatus === 'missing' || !!state.busy
}
function deleteHint(file: DriveFile) {
  if (!file.fileId) return 'ยังไม่มีข้อมูลไฟล์ในฐานข้อมูล จึงยังลบผ่านแอปไม่ได้'
  if (file.fileStatus === 'missing') return 'ไฟล์หาย ไม่สามารถลบไฟล์จริงได้'
  return `ลบ ${file.fileName}`
}
function remove() {
  const file = deleting.value
  deleting.value = null
  if (file) void manager.remove(file)
}
</script>

<template>
  <div>
    <div class="stats-row">
      <div v-for="stat in stats" :key="stat.label" class="stat-card">
        <span>{{ stat.label }}</span>
        <strong>{{ stat.value }} <small>{{ stat.unit }}</small></strong>
      </div>
    </div>

    <section class="surface no-padding">
      <div class="section-heading padded">
        <div><h2>บันทึกการย้ายรูป</h2><p>รายการทั้งหมดของโฟลเดอร์ปลายทางนี้</p></div>
        <span class="badge subtle"><AppIcon name="history" /> ประวัติ</span>
      </div>
      <div v-if="!state.jobs.length" class="empty-state">
        <span class="empty-icon"><AppIcon name="history" /></span>
        <h3>{{ state.destination ? 'ทุกการย้าย จะถูกบันทึกไว้ที่นี่' : 'เริ่มจากเลือกโฟลเดอร์ปลายทาง' }}</h3>
        <p>{{ state.destination ? 'เมื่อย้ายรูปผ่านแอป คุณจะย้อนดูผลการทำงานได้เสมอ' : 'เปิดโฟลเดอร์เพื่อดูประวัติและตรวจสอบไฟล์ของคุณ' }}</p>
      </div>
      <div v-else class="table-wrap">
        <table>
          <thead><tr><th>เวลา / ต้นทาง</th><th>สำเร็จ</th><th>ข้าม</th><th>ล้มเหลว</th><th>เวลาที่ใช้</th><th>สถานะ</th></tr></thead>
          <tbody>
            <tr v-for="job in state.jobs" :key="job.id">
              <td><strong>{{ formatDate(job.startedAt) }}</strong><small class="path table-path">{{ job.sourcePath }}</small></td>
              <td>{{ job.successCount }}</td>
              <td>{{ job.skippedCount }}</td>
              <td>{{ job.failedCount }}</td>
              <td>{{ formatDuration(job.durationMs) }}</td>
              <td><span class="badge" :class="{ warning: job.status !== 'completed' }">{{ statusLabels[job.status] || job.status }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="surface no-padding">
      <div class="section-heading padded">
        <div><h2>รูปภาพยังอยู่ครบไหม?</h2><p>ตรวจไฟล์จริงเทียบกับรายการที่บันทึกไว้</p></div>
        <button class="button secondary" :disabled="!state.destination || !!state.busy" @click="manager.refresh()"><AppIcon name="refresh" /> ตรวจสอบอีกครั้ง</button>
      </div>
      <div v-if="!state.files.length" class="empty-state">
        <span class="empty-icon"><AppIcon name="shield" /></span>
        <h3>{{ state.destination ? 'ยังไม่มีรายการให้ตรวจสอบ' : 'ยังไม่ได้เลือกปลายทาง' }}</h3>
        <p>เลือกโฟลเดอร์ แล้วเราจะแสดงสถานะไฟล์ให้คุณตรงนี้</p>
      </div>
      <div v-else class="file-list">
        <div v-for="file in state.files" :key="file.path" class="file-row">
          <span class="file-symbol"><AppIcon :name="file.fileStatus === 'missing' ? 'info' : 'image'" /></span>
          <span class="file-info"><strong>{{ file.fileName }}</strong><small class="path">{{ file.path }}</small></span>
          <span class="muted">{{ formatBytes(file.sizeBytes) }}</span>
          <span class="badge" :class="{ warning: file.fileStatus === 'missing' }">{{ file.fileStatus === 'missing' ? 'ไฟล์หาย' : 'พบไฟล์' }}</span>
          <button
            class="button delete-button"
            :disabled="deleteDisabled(file)"
            :aria-label="`ลบ ${file.fileName}`"
            :title="deleteHint(file)"
            @click="deleting = file"
          ><AppIcon name="trash" />ลบ</button>
        </div>
      </div>
      <p v-if="state.files.length" class="helper padded">ลบได้เฉพาะไฟล์ที่พบจริงและมีข้อมูลในฐานข้อมูล · ไฟล์หายจะยังเก็บรายละเอียดเดิมไว้</p>
    </section>

    <ConfirmDialog v-if="deleting" title="ยืนยันการลบไฟล์" confirm-label="ลบไฟล์นี้" danger @cancel="deleting = null" @confirm="remove">
      <p>ไฟล์: <strong class="path">{{ deleting.fileName }}</strong></p>
      <p class="path">{{ deleting.path }}</p>
      <p>ปลายทาง: <span class="path">{{ state.destination }}</span></p>
      <p class="notice warning">ไฟล์จริงจะถูกลบออกจากไดรฟ์ การดำเนินการนี้ย้อนกลับผ่านแอปไม่ได้</p>
    </ConfirmDialog>
  </div>
</template>
