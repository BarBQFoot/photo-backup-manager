import { SelectFolder } from '../../wailsjs/go/main/App'
import * as drive from '../../wailsjs/go/service/DriveService'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { backup, service } from '../../wailsjs/go/models'

export type DriveFile = Omit<backup.DriveFile, 'fileId'> & { fileId?: number | null }
export type BackupResult = service.BackupResult
export type BackupJob = service.BackupJob
export type Progress = service.BackupProgress

export function isDesktop() {
  return typeof window !== 'undefined' && !!(window as Window & { go?: unknown }).go
}

function requireDesktop() {
  if (!isDesktop()) throw new Error('กรุณาเปิดแอปผ่าน Wails เพื่อเข้าถึงโฟลเดอร์และไฟล์จริง')
}

// Keep the generated bindings untouched. Add AnalyzeImage/SearchImages here
// after their owners expose the methods and regenerate the Wails bindings.
export const desktop = {
  async selectFolder(title: string) { requireDesktop(); return SelectFolder(title) },
  async scan(path: string): Promise<DriveFile[]> { requireDesktop(); return (await drive.ScanDrive(path)) ?? [] },
  async backup(request: service.BackupRequest) { requireDesktop(); return drive.StartBackup(request) },
  async history(path: string) { requireDesktop(); return (await drive.GetBackupHistory(path)) ?? [] },
  async metadata(id: number) { requireDesktop(); return drive.GetFileMetadata(id) },
  async deleteFile(id: number, path: string) { requireDesktop(); return drive.DeleteFile(id, path) },
  async progress() { requireDesktop(); return drive.GetBackupProgress() },
  onProgress(callback: (value: Progress) => void) { requireDesktop(); return EventsOn('backup:progress', callback) },
}
