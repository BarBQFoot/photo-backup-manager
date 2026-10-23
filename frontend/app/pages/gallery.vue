<script setup lang="ts">
import { useSharedPhotoManager } from '../composables/useSharedPhotoManager'
import { computed, ref, watch } from 'vue'
import type { DriveFile } from '../services/desktop'
import { aiLabel, formatBytes, formatDate } from '../composables/usePhotoManager'
import { createCaptionSearch } from '../utils/captionSearch'

const manager = useSharedPhotoManager()
const state = manager.state
const query = ref('')
const selectedPaths = ref<string[]>([])
const selectedSet = computed(() => new Set(selectedPaths.value))
const canAnalyze = (file: DriveFile) => !!file.fileId && file.fileStatus === 'available' && file.aiStatus !== 'analyzing'
const selectedFiles = computed(() => filtered.value.filter(file => canAnalyze(file) && selectedSet.value.has(file.path)))
watch(() => state.destination, () => { selectedPaths.value = [] })

function selectVisible() {
  selectedPaths.value = filtered.value.filter(canAnalyze).map(file => file.path)
}

const searchIndex = computed(() => createCaptionSearch(state.files))
const searchMatches = computed(() => searchIndex.value.search(query.value))
const filtered = computed(() => searchMatches.value.map(match => match.item))
const relatedCount = computed(() => searchMatches.value.filter(match => match.kind !== 'exact').length)

const emptyTitle = computed(() => {
  if (!state.destination) return 'เปิดโฟลเดอร์ความทรงจำของคุณ'
  if (query.value.trim()) return 'ยังไม่เจอรูปที่ตรงกับคำนี้'
  return state.destinationScanned ? 'โฟลเดอร์นี้ยังไม่มีรูปภาพ' : 'ยังไม่ได้อ่านรายการรูปภาพ'
})
</script>

<template>
  <div>
    <section class="surface">
      <div class="section-heading">
        <div><h2>ค้นหาช่วงเวลาที่ต้องการ</h2><p>ค้นจากคำอธิบายภาพในโฟลเดอร์ที่เลือก</p></div>
        <span class="count">{{ filtered.length }} ไฟล์</span>
      </div>
      <label for="description-search" class="field-label">คำอธิบายภาพ</label>
      <div class="search-row">
        <AppIcon name="search" />
        <input
          id="description-search"
          v-model="query"
          type="search"
          placeholder="ลองค้นหา แมว ทะเล หรือภาพครอบครัว…"
          :disabled="!state.destination || !!state.busy"
        />
      </div>
      <p class="helper" aria-live="polite">ค้นจากคำอธิบายในโฟลเดอร์นี้ รองรับคำพ้องและคำพิมพ์ผิดเล็กน้อย เรียงคำตรงก่อน<span v-if="query.trim() && relatedCount"> · รวมผลคำพ้องหรือคำใกล้เคียง {{ relatedCount }} รูป</span></p>
    </section>

    <section v-if="state.files.length" class="surface ai-selection" aria-label="เลือกรูปสำหรับ AI">
      <div class="toolbar">
        <strong>เลือกวิเคราะห์ AI {{ selectedFiles.length }} รูป</strong>
        <button class="button secondary" :disabled="!!state.busy || !filtered.some(canAnalyze)" @click="selectVisible">เลือกทั้งหมดที่แสดง</button>
        <button class="text-button" :disabled="!!state.busy || !selectedPaths.length" @click="selectedPaths = []">ล้างการเลือก</button>
        <button class="button primary" :disabled="!!state.busy || !selectedFiles.length" @click="manager.analyzeSelected(selectedFiles.map(file => file.path))">
          <AppIcon name="sparkle" />วิเคราะห์รูปที่เลือก
        </button>
      </div>
      <p class="helper" role="status">{{ state.busy.startsWith('กำลังวิเคราะห์') ? state.busy : 'เลือกภาพที่ต้องการ ระบบอ่านภาพบนเครื่อง แล้วแปลคำบรรยายครั้งละไม่เกิน 100 รูป พักระหว่างชุด 10 วินาที' }}</p>
    </section>

    <div class="gallery-layout">
      <section class="surface no-padding">
        <div v-if="!filtered.length" class="empty-state">
          <span class="empty-icon"><AppIcon :name="query.trim() ? 'search' : 'image'" /></span>
          <h3>{{ emptyTitle }}</h3>
          <p>{{ query.trim() ? 'ลองใช้คำอื่น หรือล้างคำค้นเพื่อดูรูปทั้งหมด' : 'รูปภาพและรายละเอียดในปลายทางจะแสดงตรงนี้' }}</p>
          <button v-if="query" class="button secondary" @click="query = ''">ล้างคำค้น</button>
        </div>
        <div v-else class="photo-grid">
          <div
            v-for="file in filtered"
            :key="file.path"
            v-memo="[file, file.aiStatus, file.fileStatus, selectedSet.has(file.path), state.detail?.path === file.path, !!state.busy]"
            class="photo-card"
            :class="{ selected: selectedSet.has(file.path) || state.detail?.path === file.path }"
          >
            <label class="ai-checkbox">
              <input v-model="selectedPaths" type="checkbox" :value="file.path" :disabled="!!state.busy || !canAnalyze(file)" :aria-label="`เลือก ${file.fileName} เพื่อวิเคราะห์ AI`" />
              เลือกวิเคราะห์ AI
            </label>
            <button
            class="photo-detail-button"
            :disabled="!!state.busy"
            :aria-pressed="state.detail?.path === file.path"
            @click="manager.showDetail(file)"
          >
            <PhotoPreview :path="file.path" :name="file.fileName" :status="file.fileStatus" :modified-at="file.modifiedAt" />
            <div class="photo-caption">
              <strong>{{ file.fileName }}</strong>
              <span class="badge" :class="{ warning: file.fileStatus === 'missing' }">{{ file.fileStatus === 'missing' ? 'ไฟล์หาย' : 'ไฟล์อยู่ในไดรฟ์' }}</span>
              <small>{{ aiLabel(file.aiStatus) }}</small>
            </div>
          </button>
          </div>
        </div>
      </section>

      <aside class="surface detail-panel" aria-label="รายละเอียดรูปภาพ">
        <div class="section-heading">
          <h2>เรื่องราวของรูปนี้</h2>
          <button v-if="state.detail" class="text-button" aria-label="ปิดรายละเอียด" :disabled="!!state.busy" @click="state.detail = null"><AppIcon name="close" /></button>
        </div>
        <div v-if="!state.detail" class="empty-state compact">
          <span class="empty-icon"><AppIcon name="image" /></span>
          <p>เลือกรูปเพื่อดูรายละเอียด<br />และคำอธิบายภาพ</p>
        </div>
        <template v-else>
          <h3>{{ state.detail.fileName }}</h3>
          <p v-if="state.detail.fileStatus === 'missing'" class="notice warning">ไฟล์หายจากปลายทาง ไม่สามารถเปิดภาพตัวอย่างหรือวิเคราะห์ได้</p>
          <PhotoPreview v-else :path="state.detail.path" :name="state.detail.fileName" :status="state.detail.fileStatus" :modified-at="state.detail.modifiedAt" detail />
          <dl>
            <dt>ตำแหน่งไฟล์</dt><dd class="path">{{ state.detail.path }}</dd>
            <dt>ขนาด / ชนิด</dt><dd>{{ formatBytes(state.detail.sizeBytes) }} · {{ state.detail.mimeType }}</dd>
            <dt>แก้ไขล่าสุด</dt><dd>{{ formatDate(state.detail.modifiedAt) }}</dd>
            <dt>สถานะ AI</dt><dd>{{ aiLabel(state.detail.aiStatus) }}</dd>
            <dt>คำอธิบายภาพ</dt><dd class="description">{{ state.detail.description || 'ยังไม่มีคำอธิบายสำหรับรูปนี้' }}</dd>
          </dl>
          <!-- ปุ่มวิเคราะห์รูปด้วย AI -->
          <button
            class="button primary full-width"
            :disabled="!canAnalyze(state.detail) || !!state.busy"
            @click="manager.analyze()"
          >
            <AppIcon name="sparkle" />{{ state.detail.aiStatus === 'analyzing' ? 'กำลังวิเคราะห์…' : state.detail.aiStatus === 'failed' ? 'ลองวิเคราะห์อีกครั้ง' : 'วิเคราะห์รูปด้วย AI' }}
          </button>
          <p class="helper">อ่านภาพบนเครื่อง แล้วส่งเฉพาะข้อความให้ Gemini แปลเป็นไทย</p>
        </template>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.ai-selection { margin-bottom: 20px; }
.ai-checkbox { display: flex; align-items: center; gap: 8px; padding: 10px; font-size: 12px; }
.ai-checkbox input { width: 18px; height: 18px; }
.photo-detail-button { display: block; width: 100%; padding: 0; border: 0; background: transparent; text-align: left; color: inherit; }
</style>
