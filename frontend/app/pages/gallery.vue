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
const detailPanel = ref<HTMLElement | null>(null)
watch(() => state.detail?.path, () => {
  if (detailPanel.value) detailPanel.value.scrollTop = 0
}, { flush: 'post' })
const canAnalyze = (file: DriveFile) => !!file.fileId && file.fileStatus === 'available' && file.aiStatus !== 'analyzing' && !state.ai.pendingIds.has(file.fileId)
const selectedFiles = computed(() => state.files.filter(file => canAnalyze(file) && selectedSet.value.has(file.path)))
const visiblePaths = computed(() => new Set(filtered.value.map(file => file.path)))
const hiddenSelectedCount = computed(() => selectedFiles.value.filter(file => !visiblePaths.value.has(file.path)).length)
watch(() => state.destination, () => { selectedPaths.value = [] })

function selectVisible() {
  selectedPaths.value = [...new Set([...selectedPaths.value, ...filtered.value.filter(canAnalyze).map(file => file.path)])]
}

function toggleSelection(file: DriveFile, event: Event) {
  if (state.busy || !canAnalyze(file)) return
  const paths = new Set(selectedPaths.value)
  if ((event.target as HTMLInputElement).checked) paths.add(file.path)
  else paths.delete(file.path)
  selectedPaths.value = [...paths]
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
  <div class="gallery-page" :class="{ 'has-detail': state.detail }">
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
        <span v-if="hiddenSelectedCount" class="muted">นอกผลค้นหา {{ hiddenSelectedCount }} รูป</span>
        <button class="button secondary" :disabled="!!state.busy || !filtered.some(canAnalyze)" @click="selectVisible">เลือกทั้งหมดที่แสดง</button>
        <button class="text-button" :disabled="!!state.busy || !selectedPaths.length" @click="selectedPaths = []">ล้างการเลือก</button>
        <button class="button primary" :disabled="!!state.busy || state.ai.running || !selectedFiles.length" @click="manager.analyzeSelected(selectedFiles.map(file => file.path))">
          <AppIcon name="sparkle" />วิเคราะห์รูปที่เลือก
        </button>
      </div>
      <p class="helper" role="status">{{ state.ai.running ? state.ai.message : 'เลือกภาพที่ต้องการ ระบบอ่านภาพบนเครื่อง แล้วแปลคำบรรยายครั้งละไม่เกิน 100 รูป พักระหว่างชุด 10 วินาที' }}</p>
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
            v-memo="[file, file.description, file.aiStatus, file.fileStatus, selectedSet.has(file.path), state.detail?.path === file.path, !!state.busy]"
            class="photo-card"
            :class="{ selected: selectedSet.has(file.path), viewing: state.detail?.path === file.path }"
          >
            <label class="ai-checkbox">
              <input :checked="selectedSet.has(file.path)" type="checkbox" :value="file.path" :disabled="!!state.busy || !canAnalyze(file)" :aria-label="`เลือก ${file.fileName} เพื่อวิเคราะห์ AI`" @change="toggleSelection(file, $event)" />
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
              <p v-if="file.description" class="caption-description">{{ file.description }}</p>
            </div>
          </button>
          </div>
        </div>
      </section>

      <aside ref="detailPanel" class="surface detail-panel" :class="{ 'is-open': state.detail }" aria-label="รายละเอียดรูปภาพ" tabindex="0">
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
          <section class="ai-description" aria-labelledby="ai-description-heading">
            <h3 id="ai-description-heading">คำบรรยายจาก AI</h3>
            <p class="description" aria-live="polite">{{ state.detail.description || 'ยังไม่มีคำอธิบายสำหรับรูปนี้' }}</p>
            <small class="muted">{{ aiLabel(state.detail.aiStatus) }}</small>
          </section>
          <details class="file-metadata" :key="state.detail.path">
            <summary>ข้อมูลไฟล์</summary>
            <dl>
            <dt>ตำแหน่งไฟล์</dt><dd class="path">{{ state.detail.path }}</dd>
            <dt>ขนาด / ชนิด</dt><dd>{{ formatBytes(state.detail.sizeBytes) }} · {{ state.detail.mimeType }}</dd>
            <dt>แก้ไขล่าสุด</dt><dd>{{ formatDate(state.detail.modifiedAt) }}</dd>
            <dt>สถานะ AI</dt><dd>{{ aiLabel(state.detail.aiStatus) }}</dd>
            </dl>
          </details>
          <!-- ปุ่มวิเคราะห์รูปด้วย AI -->
          <button
            class="button primary full-width"
            :disabled="!canAnalyze(state.detail) || !!state.busy || state.ai.running"
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
.gallery-layout { grid-template-columns: minmax(0, 1fr) 340px; }
.detail-panel { position: sticky; top: 16px; max-height: calc(100dvh - 32px); overflow-y: auto; scrollbar-gutter: stable; }
.detail-panel :deep(.photo-preview-detail) { max-height: 150px; }
.ai-description { padding: 16px; margin-bottom: 18px; border-radius: 12px; background: var(--accent-soft); }
.ai-description h3 { margin-bottom: 10px; font-size: 14px; }
.ai-description .description { font-size: 14px; line-height: 1.9; overflow-wrap: anywhere; margin-bottom: 10px; }
.file-metadata { margin-bottom: 18px; }
.file-metadata summary { cursor: pointer; color: var(--muted); font-size: 12px; }
.caption-description { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3; overflow: hidden; color: var(--muted); font-size: 12px; line-height: 1.8; }
.photo-card.viewing .photo-detail-button { background: var(--accent-soft); }
@media (min-width: 761px) and (max-width: 900px) {
  .gallery-layout { grid-template-columns: minmax(0, 1fr) 300px; gap: 16px; }
}
@media (max-width: 760px) {
  .gallery-layout { grid-template-columns: 1fr; }
  .has-detail { padding-bottom: calc(42dvh + 24px); }
  .detail-panel { display: none; }
  .detail-panel.is-open { display: block; position: fixed; z-index: 5; top: auto; bottom: 0; left: 0; right: 0; height: 42dvh; max-height: 42dvh; margin: 0; padding: 18px 20px calc(18px + env(safe-area-inset-bottom)); border-radius: 18px 18px 0 0; box-shadow: 0 -8px 32px #302f2c18; }
  .detail-panel .section-heading { position: sticky; top: -18px; z-index: 1; background: var(--card); padding-block: 12px; margin-top: -12px; margin-bottom: 12px; }
  .detail-panel :deep(.photo-preview-detail) { display: none; }
}
.ai-selection { margin-bottom: 20px; }
.ai-checkbox { display: flex; align-items: center; gap: 8px; padding: 10px; font-size: 12px; }
.ai-checkbox input { width: 18px; height: 18px; }
.photo-detail-button { display: block; width: 100%; padding: 0; border: 0; background: transparent; text-align: left; color: inherit; }
</style>
