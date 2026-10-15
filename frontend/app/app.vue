<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { usePhotoManager, errorText } from './composables/usePhotoManager'
import { desktop, isDesktop } from './services/desktop'

const manager = usePhotoManager()
const state = manager.state
const page = ref('move')
const connected = ref(false)
const pages = [
  { id: 'move', label: 'ย้ายรูปภาพ', icon: 'transfer', title: 'ให้ทุกรูป มีที่ของมัน', description: 'ย้ายรูปไปยังโฟลเดอร์ที่ใช่ แล้วเก็บความทรงจำให้เป็นระเบียบ' },
  { id: 'gallery', label: 'แกลเลอรี', icon: 'image', title: 'ความทรงจำ ในที่เดียว', description: 'เปิดดูรูปภาพ อ่านรายละเอียด และค้นหาช่วงเวลาที่ต้องการ' },
  { id: 'history', label: 'ประวัติและตรวจสอบ', icon: 'history', title: 'ทุกรายการ ตรวจสอบได้', description: 'ดูประวัติการย้ายและเช็กว่ารูปภาพของคุณยังอยู่ครบ' },
]
const current = computed(() => pages.find(item => item.id === page.value)!)
let unsubscribe: (() => void) | undefined

onMounted(async () => {
  connected.value = isDesktop()
  if (!connected.value) return
  unsubscribe = desktop.onProgress(value => { state.progress = value })
  try {
    state.progress = await desktop.progress()
  } catch (error) {
    state.error = errorText(error)
  }
})
onUnmounted(() => unsubscribe?.())
</script>

<template>
  <div class="app-shell">
    <a class="skip-link" href="#main-content">ข้ามไปเนื้อหา</a>
    <header class="app-header">
      <a class="brand" href="#" @click.prevent="page = 'move'">
        <span class="brand-mark"><AppIcon name="image" /></span>
        <span>Photo<span class="brand-light">Backup</span><small>พื้นที่เล็ก ๆ สำหรับความทรงจำ</small></span>
      </a>
      <nav class="main-nav" aria-label="เมนูหลัก">
        <button
          v-for="item in pages"
          :key="item.id"
          class="nav-item"
          :class="{ active: page === item.id }"
          :aria-current="page === item.id ? 'page' : undefined"
          @click="page = item.id"
        >
          <AppIcon :name="item.icon" />{{ item.label }}
        </button>
      </nav>
      <span class="connection-status">
        <span class="connection-dot" :class="{ online: connected }" />
        {{ connected ? 'Desktop พร้อมใช้งาน' : 'โหมดดูหน้าจอ' }}
      </span>
    </header>

    <main id="main-content" tabindex="-1" :aria-busy="!!state.busy">
      <section class="page-heading">
        <div>
          <p class="eyebrow">YOUR PHOTOS. YOUR SPACE.</p>
          <h1>{{ current.title }}</h1>
          <p class="page-description">{{ current.description }}</p>
        </div>
        <div class="memory-art" aria-hidden="true">
          <div class="art-photo back"><AppIcon name="image" /></div>
          <div class="art-photo front"><AppIcon name="image" /><span>A little more organized.</span></div>
          <span class="art-spark">✳</span>
        </div>
      </section>

      <div v-if="!connected" class="notice neutral">
        <AppIcon name="info" />เปิดแอป Desktop เพื่อเลือกโฟลเดอร์และจัดการรูปภาพจริง
      </div>
      <section v-if="page !== 'move'" class="destination-bar">
        <span class="folder-icon"><AppIcon name="folder" /></span>
        <div>
          <span class="field-label">โฟลเดอร์ที่กำลังเปิด</span>
          <p class="path">{{ state.destination || 'ยังไม่ได้เลือกโฟลเดอร์ปลายทาง' }}</p>
        </div>
        <button class="button secondary" :disabled="!!state.busy" @click="manager.choose('destination')">เลือกโฟลเดอร์</button>
        <button class="button secondary" :disabled="!state.destination || !!state.busy" @click="manager.refresh()">
          <AppIcon name="refresh" />รีเฟรช
        </button>
      </section>
      <div v-if="state.busy" class="notice neutral" role="status"><span class="spinner" />{{ state.busy }}…</div>
      <div v-if="state.error" class="notice error" role="alert">
        <AppIcon name="info" /><span>{{ state.error }}</span>
        <button class="text-button push-right" aria-label="ปิดข้อความผิดพลาด" @click="state.error = ''"><AppIcon name="close" /></button>
      </div>
      <div v-if="state.notice" class="notice success" role="status"><AppIcon name="check" />{{ state.notice }}</div>
      <KeepAlive>
        <MovePhotos v-if="page === 'move'" :manager="manager" />
        <PhotoGallery v-else-if="page === 'gallery'" :manager="manager" />
        <HistoryCheck v-else :manager="manager" />
      </KeepAlive>
      <footer class="workspace-footer">
        <span><AppIcon name="shield" />รูปของคุณ อยู่ในไดรฟ์ของคุณ</span>
        <span>Made for your memories.</span>
      </footer>
    </main>
  </div>
</template>
