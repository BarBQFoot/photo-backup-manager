import { test } from 'node:test'
import assert from 'node:assert/strict'
import { registerHooks } from 'node:module'

// Match Nuxt's extensionless source imports without changing generated files.
registerHooks({
  resolve(specifier, context, next) {
    if (context.parentURL?.includes('/frontend/app/') && specifier.startsWith('.') && !/\.[a-z]+$/i.test(specifier)) {
      for (const extension of ['.ts', '.js']) {
        try { return next(specifier + extension, context) } catch (error) {
          if (error.code !== 'ERR_MODULE_NOT_FOUND') throw error
        }
      }
    }
    return next(specifier, context)
  },
})
const { usePhotoManager } = await import('../app/composables/usePhotoManager.ts')

const photo = { fileId: 1, fileName: 'cat.jpg', path: 'D:/Photos/cat.jpg', sizeBytes: 100, mimeType: 'image/jpeg', modifiedAt: '', fileStatus: 'available', aiStatus: 'unanalyzed' }
function setup(overrides = {}) {
  const calls = []
  globalThis.window = { go: {
    main: { App: { SelectFolder: async () => 'D:/Photos' } },
    service: { DriveService: {
      ScanDrive: async path => { calls.push(['scan', path]); return [photo] },
      GetBackupHistory: async path => { calls.push(['history', path]); return [] },
      ...overrides,
    } },
  } }
  const manager = usePhotoManager()
  return { manager, calls, s: manager.state }
}

test('choosing a destination replaces old data and scopes scan/history to the selected folder', async () => {
  const { manager, calls, s } = setup()
  s.files = [{ ...photo, path: 'E:/Old/old.jpg' }]
  s.detail = s.files[0]
  await manager.choose('destination')
  assert.equal(s.destination, 'D:/Photos')
  assert.deepEqual(calls, [['scan', 'D:/Photos'], ['history', 'D:/Photos']])
  assert.equal(s.files[0].path, photo.path)
  assert.equal(s.detail, null)
})

test('source scanning selects available files and excludes missing records', async () => {
  const { manager, s } = setup({ ScanDrive: async () => [photo, { ...photo, path: 'D:/Photos/missing.jpg', fileStatus: 'missing' }] })
  s.source = 'D:/Photos'
  await manager.scanSource()
  assert.deepEqual([...s.selected], [photo.path])
})

test('a failed move clears selection and reconciles disk state without retrying the move', async () => {
  let attempts = 0
  const { manager, s, calls } = setup({ StartBackup: async () => { attempts++; throw new Error('DATABASE_UPDATE_FAILED') } })
  s.source = 'C:/Source'; s.destination = 'D:/Photos'; s.selected = ['C:/Source/cat.jpg']
  await manager.move()
  assert.equal(attempts, 1)
  assert.equal(s.error, 'DATABASE_UPDATE_FAILED')
  assert.equal(s.busy, '')
  assert.equal(s.progress.status, 'idle')
  assert.equal(s.selected.length, 0)
  assert.ok(calls.some(call => call[0] === 'scan' && call[1] === 'C:/Source'))
  assert.ok(calls.some(call => call[0] === 'scan' && call[1] === 'D:/Photos'))
})

test('successful moves retain per-file outcomes and real duration', async () => {
  const result = { jobId: 2, totalFiles: 2, successCount: 1, skippedCount: 1, failedCount: 0, durationMs: 812, items: [{ sourcePath: 'C:/Source/cat.jpg', status: 'moved' }, { sourcePath: 'C:/Source/dog.jpg', status: 'skipped' }] }
  const { manager, s } = setup({ StartBackup: async () => result })
  s.source = 'C:/Source'; s.destination = 'D:/Photos'; s.selected = result.items.map(item => item.sourcePath)
  await manager.move()
  assert.equal(s.result.durationMs, 812)
  assert.equal(s.result.skippedCount, 1)
  assert.equal(s.result.items[1].status, 'skipped')
})

test('failed DB updates after deletion trigger a refresh and preserve the failure message', async () => {
  const { manager, s, calls } = setup({ DeleteFile: async (id, path) => {
    assert.equal(id, 1); assert.equal(path, 'D:/Photos'); throw new Error('DATABASE_UPDATE_FAILED')
  } })
  s.destination = 'D:/Photos'
  await manager.remove(photo)
  assert.equal(s.error, 'DATABASE_UPDATE_FAILED')
  assert.ok(calls.some(call => call[0] === 'scan'))
})

test('missing files and records without IDs cannot be deleted', async () => {
  let calls = 0
  const { manager } = setup({ DeleteFile: async () => { calls++ } })
  await manager.remove({ ...photo, fileStatus: 'missing' })
  await manager.remove({ ...photo, fileId: null })
  assert.equal(calls, 0)
})

test('missing scans remain missing even if stored metadata says active', async () => {
  const { manager, s } = setup({ GetFileMetadata: async () => ({ description: 'แมว', aiStatus: 'analyzed', status: 'active' }) })
  await manager.showDetail({ ...photo, fileStatus: 'missing' })
  assert.equal(s.detail.fileStatus, 'missing')
  assert.equal(s.detail.description, 'แมว')
})

test('a history failure preserves the successful scan and reports an error', async () => {
  const { manager, s } = setup({ GetBackupHistory: async () => { throw new Error('HISTORY_READ_FAILED') } })
  s.destination = 'D:/Photos'
  await manager.refresh()
  assert.equal(s.files.length, 1)
  assert.equal(s.error, 'HISTORY_READ_FAILED')
  assert.equal(s.busy, '')
})

test('browser-only mode explains desktop requirements without accessing files', async () => {
  delete globalThis.window
  const manager = usePhotoManager()
  await manager.choose('source')
  assert.match(manager.state.error, /Wails/)
  assert.equal(manager.state.source, '')
})
