import { reactive } from 'vue'
import { desktop, type DriveFile, type BackupJob, type BackupResult, type Progress } from '../services/desktop'

export function usePhotoManager() {
  const state = reactive({
    source: '', destination: '', sourceFiles: [] as DriveFile[], files: [] as DriveFile[],
    jobs: [] as BackupJob[], selected: [] as string[], detail: null as DriveFile | null,
    sourceScanned: false, destinationScanned: false, busy: '', error: '', notice: '',
    result: null as BackupResult | null,
    progress: { jobId: 0, completed: 0, total: 0, status: 'idle' } as Progress,
  })

  async function run(label: string, action: () => Promise<void>) {
    if (state.busy) return
    state.busy = label
    state.error = ''; state.notice = ''
    try { await action() }
    catch (error) { state.error = errorText(error) }
    finally { state.busy = '' }
  }

  async function refreshDestination() {
    const results = await Promise.allSettled([desktop.scan(state.destination), desktop.history(state.destination)])
    if (results[0].status === 'fulfilled') {
      state.files = results[0].value
      state.destinationScanned = true
      state.detail = null
    }
    if (results[1].status === 'fulfilled') state.jobs = results[1].value
    const failed = results.filter((item): item is PromiseRejectedResult => item.status === 'rejected')
    if (failed.length) throw failed[0]!.reason
  }

  async function choose(kind: 'source' | 'destination') {
    await run('กำลังเปิดโฟลเดอร์', async () => {
      const path = await desktop.selectFolder(kind === 'source' ? 'เลือกโฟลเดอร์ต้นทาง' : 'เลือกโฟลเดอร์ปลายทาง')
      if (!path) return
      state[kind] = path
      if (kind === 'source') {
        state.sourceFiles = []; state.selected = []; state.sourceScanned = false; state.result = null
      } else {
        state.files = []; state.jobs = []; state.detail = null; state.destinationScanned = false
        await refreshDestination()
      }
    })
  }

  async function scanSource() {
    if (!state.source) return
    await run('กำลังสแกนต้นทาง', async () => {
      state.sourceFiles = await desktop.scan(state.source)
      state.sourceScanned = true
      state.selected = state.sourceFiles.filter(file => file.fileStatus === 'available').map(file => file.path)
    })
  }

  async function refresh() {
    if (!state.destination) return
    await run('กำลังตรวจสอบปลายทาง', async () => {
      await refreshDestination()
      state.notice = 'ตรวจสอบไฟล์และอัปเดตประวัติแล้ว'
    })
  }

  async function move() {
    if (!state.source || !state.destination || !state.selected.length) return
    await run('กำลังย้ายรูป', async () => {
      state.result = null
      state.progress = { jobId: 0, completed: 0, total: state.selected.length, status: 'moving' }
      try {
        state.result = await desktop.backup({ sourcePath: state.source, destinationPath: state.destination, filePaths: [...state.selected] })
      } finally {
        state.busy = 'กำลังอัปเดตรายการหลังย้าย'
        state.progress = { ...state.progress, status: 'idle' }
        state.selected = []
        const scan = await Promise.allSettled([desktop.scan(state.source), refreshDestination()])
        if (scan[0].status === 'fulfilled') { state.sourceFiles = scan[0].value; state.sourceScanned = true }
        const failed = scan.find(item => item.status === 'rejected')
        if (failed?.status === 'rejected') state.error = `อัปเดตรายการไม่สำเร็จ: ${errorText(failed.reason)}`
      }
    })
  }

  async function showDetail(file: DriveFile) {
    if (state.busy) return
    state.detail = file
    if (!file.fileId) return
    await run('กำลังอ่านรายละเอียด', async () => {
      const metadata = await desktop.metadata(file.fileId!)
      state.detail = { ...file, description: metadata.description, aiStatus: metadata.aiStatus,
        fileStatus: file.fileStatus === 'missing' || metadata.status === 'missing' || metadata.status === 'deleted' ? 'missing' : file.fileStatus }
    })
  }

  async function analyzeSelected(paths: string[]) {
    const selectedPaths = new Set(paths)
    const files = state.files.filter(file => selectedPaths.has(file.path) && file.fileId && file.fileStatus === 'available' && file.aiStatus !== 'analyzing')
    if (!files.length) return
    await run('กำลังวิเคราะห์รูปที่เลือก', async () => {
      const analysisStarted = performance.now()
      const selectedIds = new Set(files.map(file => file.fileId!))
      const update = (id: number, changes: Partial<DriveFile>) => {
        const index = state.files.findIndex(file => file.fileId === id)
        if (index >= 0) state.files[index] = { ...state.files[index]!, ...changes }
        if (state.detail?.fileId === id) state.detail = { ...state.detail, ...changes }
      }
      const unsubscribe = desktop.onAIProgress(event => {
        if (event.type === 'progress' && event.phase !== 'done') {
          const total = event.total ?? files.length
          state.busy = event.detail || (event.phase === 'pipeline'
            ? `กำลังวิเคราะห์: อ่านภาพ ${event.captioned ?? 0}/${total} · คำบรรยายไทย ${event.translated ?? 0}/${total} รูป`
            : `กำลังวิเคราะห์: ${event.phase === 'captioning' ? 'อ่านภาพ' : 'แปลคำบรรยาย'} ${event.completed ?? 0}/${total} รูป`)
        }
        if (event.type === 'result' && event.file_id && event.caption) {
          const id = Number(event.file_id.replace(/^file_/, ''))
          if (selectedIds.has(id)) update(id, { description: event.caption, aiStatus: 'analyzed' })
        }
      })
      for (const file of files) update(file.fileId!, { aiStatus: 'analyzing' })
      try {
        const result = await desktop.analyzeImages(files.map(file => file.fileId!))
        for (const item of result.results ?? []) update(item.fileId, { description: item.description, aiStatus: 'analyzed' })
        for (const item of result.failures ?? []) update(item.fileId, { aiStatus: 'failed' })
        state.notice = `วิเคราะห์สำเร็จ ${result.results.length}/${files.length} รูป${result.failures.length ? ` ไม่สำเร็จ ${result.failures.length} รูป กดเลือกเฉพาะรูปที่ไม่สำเร็จเพื่อทำต่อได้` : ''}`
        if (result.failures.length) {
          const groups = new Map<string, string[]>()
          for (const item of result.failures) {
            const names = groups.get(item.message) ?? []
            names.push(files.find(file => file.fileId === item.fileId)?.fileName ?? String(item.fileId))
            groups.set(item.message, names)
          }
          state.error = [...groups].map(([message, names]) => `${message}\n${names.length} รูป: ${names.slice(0, 3).join(', ')}${names.length > 3 ? ` และอีก ${names.length - 3} รูป` : ''}`).join('\n\n')
        }
      } finally {
        unsubscribe()
        // Reconcile statuses even after a bridge/network error; preserve already saved captions.
        const results = await Promise.allSettled([desktop.scan(state.destination)])
        if (results[0]?.status === 'fulfilled') {
          state.files = results[0].value
          if (state.detail) state.detail = state.files.find(file => file.fileId === state.detail?.fileId) ?? null
        } else {
          for (const file of state.files.filter(file => selectedIds.has(file.fileId!) && file.aiStatus === 'analyzing')) update(file.fileId!, { aiStatus: 'failed' })
        }
        if (state.notice) {
          const elapsed = Math.round((performance.now() - analysisStarted) / 1000)
          state.notice += ` · ใช้เวลา ${Math.floor(elapsed / 60)} นาที ${elapsed % 60} วินาที`
        }
      }
    })
  }

  async function analyze() {
    const file = state.detail
    if (!file?.fileId || file.fileStatus === 'missing') return
    await analyzeSelected([file.path])
  }

  async function remove(file: DriveFile) {
    if (!file.fileId || file.fileStatus === 'missing') return
    await run('กำลังลบไฟล์', async () => {
      try {
        await desktop.deleteFile(file.fileId!, state.destination)
        state.notice = `ลบ ${file.fileName} แล้ว`
      } finally {
        // Reconcile even when the file was removed but the DB update failed.
        const refreshed = await Promise.allSettled([refreshDestination()])
        if (refreshed[0]?.status === 'rejected') state.error = `อัปเดตรายการไม่สำเร็จ กรุณาตรวจสอบอีกครั้ง: ${errorText(refreshed[0].reason)}`
      }
    })
  }

  return { state, choose, scanSource, refresh, move, showDetail, analyze, analyzeSelected, remove }
}

export type PhotoManager = ReturnType<typeof usePhotoManager>

export function errorText(error: unknown): string {
  if (error instanceof Error) return error.message
  if (typeof error === 'string') {
    try { const parsed = JSON.parse(error); if (parsed.message) return parsed.message } catch { /* plain Wails message */ }
    return error
  }
  if (error && typeof error === 'object' && 'message' in error) return String(error.message)
  return 'เกิดข้อผิดพลาด กรุณาลองใหม่'
}

export function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}
export function formatDuration(value: number) { return `${(value / 1000).toFixed(1)} วินาที` }
export function formatDate(value: string) { return value ? new Date(value).toLocaleString('th-TH') : '—' }
export function aiLabel(value: string) {
  return ({ unanalyzed: 'ยังไม่วิเคราะห์', analyzing: 'กำลังวิเคราะห์', analyzed: 'วิเคราะห์แล้ว', failed: 'วิเคราะห์ไม่สำเร็จ' } as Record<string, string>)[value] ?? 'ยังไม่วิเคราะห์'
}
