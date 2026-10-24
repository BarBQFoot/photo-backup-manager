<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { errorText } from '../composables/usePhotoManager'
import { useSharedPhotoManager } from '../composables/useSharedPhotoManager'
import { desktop, isDesktop } from '../services/desktop'

const manager = useSharedPhotoManager()
const state = manager.state
const route = useRoute()
const connected = ref(false)
const pages = [
  { id: 'move', path: '/', label: 'ย้าย / สำรองรูป', icon: 'transfer', title: 'ให้ทุกรูป มีที่ของมัน', description: 'เลือกย้ายหรือสำรองรูปไปยังโฟลเดอร์ที่ใช่ แล้วเก็บความทรงจำให้เป็นระเบียบ' },
  { id: 'gallery', path: '/gallery', label: 'แกลเลอรี', icon: 'image', title: 'ความทรงจำ ในที่เดียว', description: 'เปิดดูรูปภาพ อ่านรายละเอียด และค้นหาช่วงเวลาที่ต้องการ' },
  { id: 'history', path: '/history', label: 'ประวัติและตรวจสอบ', icon: 'history', title: 'ทุกรายการ ตรวจสอบได้', description: 'ดูประวัติการจัดเก็บและเช็กว่ารูปภาพของคุณยังอยู่ครบ' },
]
const current = computed(() => pages.find(item => item.path === route.path) ?? pages[0]!)
function focusMain() {
  document.getElementById('main-content')?.focus()
}
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
    <a class="skip-link" href="#main-content" @click.prevent="focusMain">ข้ามไปเนื้อหา</a>
    <header class="app-header">
      <NuxtLink class="brand" to="/">
        <span class="brand-mark"><AppIcon name="image" /></span>
        <span>Photo<span class="brand-light">Backup</span><small>พื้นที่เล็ก ๆ สำหรับความทรงจำ</small></span>
      </NuxtLink>
      <nav class="main-nav" aria-label="เมนูหลัก">
        <NuxtLink
          v-for="item in pages"
          :key="item.id"
          class="nav-item"
          :class="{ active: current.id === item.id }"
          :aria-current="current.id === item.id ? 'page' : undefined"
          :to="item.path"
        >
          <AppIcon :name="item.icon" />{{ item.label }}
        </NuxtLink>
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
      <section v-if="current.id !== 'move'" class="destination-bar">
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
      <div v-if="state.ai.running" class="notice neutral" role="status">
        <span class="spinner" /><div><strong>{{ state.ai.message }}</strong><small class="path" style="display:block">{{ state.ai.folder }}</small><small>ทำงานเบื้องหลัง · เปิดดูรูปที่เสร็จแล้วหรือย้ายรูปอื่นได้</small></div>
      </div>
      <div v-if="state.ai.error" class="notice error" role="alert">
        <AppIcon name="info" /><span>{{ state.ai.error }}</span>
        <button class="text-button push-right" aria-label="ปิดข้อผิดพลาด AI" @click="state.ai.error = ''"><AppIcon name="close" /></button>
      </div>
      <div v-if="state.ai.notice" class="notice success" role="status">
        <AppIcon name="check" />{{ state.ai.notice }}
        <button class="text-button push-right" aria-label="ปิดผลวิเคราะห์" @click="state.ai.notice = ''"><AppIcon name="close" /></button>
      </div>
      <div v-if="state.error" class="notice error" role="alert">
        <AppIcon name="info" /><span>{{ state.error }}</span>
        <button class="text-button push-right" aria-label="ปิดข้อความผิดพลาด" @click="state.error = ''"><AppIcon name="close" /></button>
      </div>
      <div v-if="state.notice" class="notice success" role="status"><AppIcon name="check" />{{ state.notice }}</div>
      <slot />
      <footer class="workspace-footer">
        <span><AppIcon name="shield" />รูปของคุณ อยู่ในไดรฟ์ของคุณ</span>
        <span>Made for your memories.</span>
      </footer>
    </main>
  </div>
</template>
