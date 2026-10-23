<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { desktop, type AISetupStatus } from '../services/desktop'
import { errorText } from '../composables/usePhotoManager'

const emit = defineEmits<{ continue: [] }>()
const status = ref<AISetupStatus>({ status: 'idle', phase: 'checking', detail: '', completed: 0, total: 0, unit: '', device: '', error: '' })
const requesting = ref(false)
const stages = [
  { id: 'checking', label: 'ตรวจโปรแกรมและโมเดลที่มีอยู่' },
  { id: 'bootstrap', label: 'เตรียมตัวติดตั้ง' },
  { id: 'python', label: 'เตรียม Python' },
  { id: 'pytorch', label: 'เตรียม PyTorch สำหรับอ่านภาพ' },
  { id: 'packages', label: 'เตรียมไลบรารีของโมเดล' },
  { id: 'model', label: 'ดาวน์โหลด Florence-2' },
  { id: 'verify', label: 'ตรวจสอบความพร้อม' },
]
const stageIndex = computed(() => status.value.status === 'ready' ? stages.length : Math.max(0, stages.findIndex(stage => stage.id === status.value.phase)))
const isSkipped = (id: string) => status.value.skipped?.includes(id) ?? false
const heading = computed(() => status.value.status === 'ready' ? 'พร้อมเก็บเรื่องราวของคุณแล้ว' : status.value.status === 'error' ? 'เตรียมระบบยังไม่สำเร็จ' : 'กำลังเตรียมระบบอ่านภาพ')
const percentage = computed(() => status.value.total > 0 ? Math.min(100, Math.round(status.value.completed * 100 / status.value.total)) : null)
const amount = computed(() => {
  const { completed, total, unit } = status.value
  if (total <= 0) return ''
  if (unit === 'bytes') return `${(completed / 1048576).toFixed(1)} / ${(total / 1048576).toFixed(1)} MB`
  return `${completed} / ${total} ไฟล์`
})
let polling: ReturnType<typeof setInterval> | undefined
let pollingBusy = false
let disposed = false

async function poll() {
  if (pollingBusy || disposed) return
  pollingBusy = true
  try { status.value = await desktop.aiSetupStatus() }
  catch (error) { status.value = { ...status.value, status: 'error', error: errorText(error) } }
  finally { pollingBusy = false }
}
async function start() {
  if (requesting.value) return
  requesting.value = true
  try { status.value = await desktop.startAISetup() }
  catch (error) { status.value = { ...status.value, status: 'error', error: errorText(error) } }
  finally { requesting.value = false }
}
onMounted(async () => {
  await start()
  if (!disposed) polling = setInterval(() => { if (status.value.status === 'running') void poll() }, 500)
})
onUnmounted(() => { disposed = true; if (polling) clearInterval(polling) })
</script>

<template>
  <main class="setup-page" aria-labelledby="setup-heading">
    <section class="setup-card">
      <div class="setup-brand"><AppIcon name="image" /> PhotoBackup</div>
      <h1 id="setup-heading">{{ heading }}</h1>
      <p class="setup-intro">ติดตั้งเฉพาะส่วนที่ยังไม่มี โปรแกรมและโมเดลที่พร้อมแล้วจะถูกข้าม</p>
      <ol class="setup-stages" aria-label="ขั้นตอนเตรียมระบบ">
        <li v-for="(stage, index) in stages" :key="stage.id" :class="{ done: index < stageIndex || isSkipped(stage.id), active: index === stageIndex && !isSkipped(stage.id), failed: index === stageIndex && status.status === 'error' }" :aria-current="index === stageIndex ? 'step' : undefined">
          <span class="stage-marker" aria-hidden="true"><span v-if="index === stageIndex && status.status === 'running' && !isSkipped(stage.id)" class="spinner" /><template v-else>{{ index < stageIndex || isSkipped(stage.id) ? '✓' : index + 1 }}</template></span>
          <span>{{ stage.label }}</span>
          <small v-if="isSkipped(stage.id)">มีอยู่แล้ว · ข้าม</small><small v-else-if="index < stageIndex">พร้อมแล้ว</small>
        </li>
      </ol>
      <div class="setup-progress" aria-live="polite">
        <p>{{ status.detail || 'กำลังตรวจความพร้อม…' }}</p>
        <template v-if="status.status === 'running'">
          <progress aria-label="ความคืบหน้าของขั้นตอนปัจจุบัน" :value="percentage === null ? undefined : percentage" max="100" />
          <div v-if="amount" class="setup-amount"><span>{{ amount }}</span><strong>{{ percentage }}%</strong></div>
        </template>
        <p v-if="status.status === 'ready' && status.device" class="setup-device">อุปกรณ์อ่านภาพ: {{ status.device.toUpperCase() }}</p>
      </div>
      <div v-if="status.status === 'error'" class="setup-error" role="alert">
        <p>{{ status.error }}</p>
        <p>ส่วนที่ดาวน์โหลดสำเร็จแล้วจะถูกนำมาใช้ต่อ หากลองใหม่แล้วยังไม่สำเร็จ กรุณาส่งข้อความผิดพลาดด้านบน</p>
      </div>
      <div class="setup-actions">
        <button v-if="status.status === 'ready'" class="button primary" @click="emit('continue')">เริ่มใช้งาน</button>
        <template v-if="status.status === 'error'">
          <button class="button primary" :disabled="requesting" @click="start">ลองอีกครั้ง</button>
          <button class="button secondary" @click="emit('continue')">เข้าแอปโดยยังไม่ใช้ AI</button>
        </template>
      </div>
      <p class="setup-note">ครั้งแรกใช้อินเทอร์เน็ตและพื้นที่หลาย GB · เปิดครั้งต่อไปจะตรวจและใช้ของเดิม<br />การแปลไทยยังต้องตั้ง Gemini API key ในไฟล์ .env ข้างโปรแกรม</p>
    </section>
  </main>
</template>

<style scoped>
.setup-page { min-height: 100vh; padding: 20px; display: grid; place-items: center; background: var(--paper); }
.setup-card { width: min(100%, 620px); padding: 24px; border: 1px solid var(--line); border-radius: 24px; background: var(--card); }
.setup-brand { display: flex; gap: 10px; align-items: center; color: var(--accent); font-size: 18px; font-weight: 700; margin-bottom: 16px; }
.setup-card h1 { font-size: clamp(23px, 4vw, 30px); letter-spacing: -.4px; }
.setup-intro { margin-top: 8px; color: var(--muted); line-height: 1.7; }
.setup-stages { list-style: none; padding: 0; margin: 18px 0; display: grid; gap: 6px; }
.setup-stages li { display: flex; align-items: center; gap: 12px; padding: 6px 10px; color: var(--muted); border-radius: 10px; }
.setup-stages li.active { color: var(--ink); background: var(--accent-soft); }
.setup-stages li.done { color: #387158; }
.setup-stages li.failed { color: #ad3928; }
.setup-stages small { margin-left: auto; font-size: 11px; }
.stage-marker { width: 24px; height: 24px; flex: 0 0 24px; display: grid; place-items: center; border: 1px solid currentColor; border-radius: 50%; font-size: 12px; }
.stage-marker .spinner { width: 14px; height: 14px; }
.setup-progress { min-height: 44px; font-size: 13px; line-height: 1.7; overflow-wrap: anywhere; }
.setup-progress progress { width: 100%; height: 9px; margin-top: 14px; accent-color: var(--accent); }
.setup-amount { display: flex; justify-content: space-between; color: var(--muted); margin-top: 4px; }
.setup-device { color: #387158; }
.setup-error { margin-top: 14px; padding: 14px; border-radius: 12px; background: #fff1ed; color: #923623; line-height: 1.7; overflow-wrap: anywhere; }
.setup-error p + p { margin-top: 8px; font-size: 12px; }
.setup-error p:first-child { white-space: pre-wrap; user-select: text; }
.setup-actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 16px; }
.setup-note { margin-top: 16px; color: var(--muted); font-size: 11px; line-height: 1.9; }
@media (max-width: 480px) { .setup-page { padding: 16px 10px; } .setup-card { padding: 22px 16px; } }
</style>
