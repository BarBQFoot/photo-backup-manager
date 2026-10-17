<script setup lang="ts">
import { useSharedPhotoManager } from '../composables/useSharedPhotoManager'
import { computed, ref } from 'vue'
import { aiLabel, formatBytes, formatDate } from '../composables/usePhotoManager'

const manager = useSharedPhotoManager()
const state = manager.state
const query = ref('')

const filtered = computed(() => {
  const term = query.value.trim().toLocaleLowerCase('th')
  if (!term) return state.files

  return state.files.filter(file =>
    file.fileStatus === 'available' && file.description?.toLocaleLowerCase('th').includes(term),
  )
})

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
      <p class="helper">ค้นจากคำอธิบายที่โหลดแล้วในโฟลเดอร์ที่เลือก</p>
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
          <button
            v-for="file in filtered"
            :key="file.path"
            class="photo-card"
            :class="{ selected: state.detail?.path === file.path }"
            :disabled="!!state.busy"
            :aria-pressed="state.detail?.path === file.path"
            @click="manager.showDetail(file)"
          >
            <div class="photo-placeholder" :class="{ missing: file.fileStatus === 'missing' }">
              <AppIcon :name="file.fileStatus === 'missing' ? 'info' : 'image'" />
              <small>{{ file.fileStatus === 'missing' ? 'ไม่พบไฟล์จริง' : 'ยังไม่มีภาพตัวอย่าง' }}</small>
            </div>
            <div class="photo-caption">
              <strong>{{ file.fileName }}</strong>
              <span class="badge" :class="{ warning: file.fileStatus === 'missing' }">{{ file.fileStatus === 'missing' ? 'ไฟล์หาย' : 'ไฟล์อยู่ในไดรฟ์' }}</span>
              <small>{{ aiLabel(file.aiStatus) }}</small>
            </div>
          </button>
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
          <p v-else class="notice neutral">ภาพตัวอย่างยังไม่พร้อมใช้งาน</p>
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
            :disabled="state.detail.fileStatus === 'missing' || state.detail.aiStatus === 'analyzing' || !!state.busy"
            @click="manager.analyze()"
          >
            <AppIcon name="sparkle" />{{ state.detail.aiStatus === 'analyzing' ? 'กำลังวิเคราะห์…' : state.detail.aiStatus === 'failed' ? 'ลองวิเคราะห์อีกครั้ง' : 'วิเคราะห์รูปด้วย AI' }}
          </button>
          <p class="helper">ภาพจะถูกอ่านโดย Go และส่งให้ Gemini</p>
        </template>
      </aside>
    </div>
  </div>
</template>
