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

  async function analyzeFile(file: DriveFile) {
    const update = (changes: Partial<DriveFile>) => {
      const index = state.files.findIndex(item => item.path === file.path)
      if (index >= 0) state.files[index] = { ...state.files[index]!, ...changes }
      if (state.detail?.path === file.path) state.detail = { ...state.detail, ...changes }
    }
    update({ aiStatus: 'analyzing' })
    try {
      const result = await desktop.analyzeImage(file.fileId!)
      update({ description: result.description, aiStatus: 'analyzed' })
    } catch (error) {
      update({ aiStatus: 'failed' })
      throw error
    }
  }

  async function analyzeSelected(paths: string[]) {
    const selectedPaths = new Set(paths)
    const files = state.files.filter(file => selectedPaths.has(file.path) && file.fileId && file.fileStatus === 'available' && file.aiStatus !== 'analyzing')
    if (!files.length) return
    await run('กำลังวิเคราะห์รูปที่เลือก', async () => {
      let succeeded = 0
      const failures: string[] = []
      for (const [index, file] of files.entries()) {
        state.busy = `กำลังวิเคราะห์ ${index + 1}/${files.length} รูป`
        try { await analyzeFile(file); succeeded++ }
        catch (error) { failures.push(`${file.fileName}: ${errorText(error)}`) }
      }
      state.notice = `วิเคราะห์สำเร็จ ${succeeded}/${files.length} รูป${failures.length ? ` ไม่สำเร็จ ${failures.length} รูป` : ''}`
      if (failures.length) state.error = failures.join('\n')
    })
  }

  async function analyze() {
    const file = state.detail
    if (!file?.fileId || file.fileStatus === 'missing') return
    await run('กำลังวิเคราะห์รูป', async () => {
      await analyzeFile(file)
      state.notice = 'วิเคราะห์รูปและบันทึกคำอธิบายแล้ว'
    })
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
