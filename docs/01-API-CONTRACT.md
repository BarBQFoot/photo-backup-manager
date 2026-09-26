# API Contract — Drive-Based Photo Storage

> ขอบเขตเวอร์ชันส่งงาน: วิเคราะห์/ค้นด้วย Description เท่านั้น; ไม่มี Tags/Tag Cloud. ใช้ Full Path จับคู่ Metadata ชั่วคราว; Hash Identity เลื่อนไปเป็นงานต่อยอด. รูปจริงอยู่บน Drive; SQLite เก็บ Metadata เท่านั้น ห้ามเก็บ Image Binary/Base64/BLOB/Thumbnail Binary.

## กติกากลาง

- Frontend เรียก Go ผ่าน Wails Binding เท่านั้น; Go เป็นผู้เข้าถึง Drive และ SQLite
- JSON ใช้ lower camelCase; เวลาเป็น RFC 3339; Error ใช้ `{code,message,details?}`
- `fileId` เป็น Primary Key ภายใน DB; เวอร์ชันส่งงานจับคู่ด้วย Full Path ที่ Scan พบ
- Drive Scan เป็นแหล่งจริงของรายการรูปที่มีอยู่ ส่วน SQLite เติม Metadata ให้ไฟล์ที่จับคู่กันได้
- Search คืน Metadata และ Path อ้างอิงไฟล์จริงบน Drive ไม่คืน Image Data
- Search เป็นส่วนที่ต้องทำสำหรับ Description-only scope; ระหว่างพัฒนาอาจยังไม่พร้อม แต่ Demo/ส่งงานต้องค้นได้จริง ห้ามส่ง Mock เป็นผลจริง

## Shared Types

```ts
type AppError = { code: string; message: string; details?: Record<string, unknown> }
type AIStatus = 'unanalyzed' | 'analyzing' | 'analyzed' | 'failed'
type FileStatus = 'available' | 'missing'
type DriveFile = {
  fileId: number | null; fileName: string; path: string
  sizeBytes: number; mimeType: string; modifiedAt: string
  fileStatus: FileStatus; aiStatus: AIStatus
  description?: string
}
type FileMetadata = {
  fileId: number; fileName: string; path: string
  sizeBytes: number; mimeType: string; modifiedAt: string
  aiStatus: AIStatus; description?: string
  backupJobId?: number; status: 'active' | 'missing' | 'deleted'
}
type BackupRequest = { sourcePath: string; destinationPath: string; filePaths: string[] }
type BackupItemResult = { sourcePath: string; destinationPath?: string; status: 'moved' | 'skipped' | 'failed'; error?: AppError }
type BackupResult = { jobId: number; totalFiles: number; successCount: number; skippedCount: number; failedCount: number; durationMs: number; items: BackupItemResult[] }
type AIResult = { fileId: number; description: string; status: 'success' }
type SearchResult = { fileId: number; fileName: string; path: string; sizeBytes: number; mimeType: string; fileStatus: FileStatus; description: string }
```

`DriveFile`/`SearchResult` เป็นข้อมูลอ้างอิงไฟล์ ไม่บรรจุ Binary, Base64 หรือ Thumbnail Bytes

## Methods

### `ScanDrive(path: string): DriveFile[]`

- **หน้าที่:** สแกน Drive/Folder แล้วคืนภาพที่พบจริงพร้อม Metadata ที่จับคู่ด้วย Full Path; เพิ่มรายการ DB เก่าภายใต้โฟลเดอร์ที่สแกนซึ่งไม่พบไฟล์จริงเป็น `Missing`
- **Request:** พาธโฟลเดอร์/Drive
- **Response:** รายการไฟล์จริงและ Metadata สำหรับ Gallery รวม Missing Records ที่เกี่ยวกับโฟลเดอร์; ไฟล์ใหม่ใช้ `fileId:null`, `fileStatus:"available"`, `aiStatus:"unanalyzed"`; ไม่คืน Image Binary
- **Errors:** `INVALID_PATH`, `PATH_NOT_DIRECTORY`, `ACCESS_DENIED`, `SCAN_FAILED`
- **ตัวอย่าง:** รับ `E:\\Photos` → `[{"fileId":7,"fileName":"IMG_001.jpg","path":"E:\\Photos\\IMG_001.jpg","sizeBytes":12345,"mimeType":"image/jpeg","modifiedAt":"2026-09-01T10:00:00Z","fileStatus":"available","aiStatus":"analyzed","description":"A cat indoors"}]`
- **Owner:** Backend / Member 1

### `StartBackup(request: BackupRequest): BackupResult`

- **หน้าที่:** ย้ายรายการไฟล์ที่เลือกจาก Source ไป Destination ตาม Requirement เดิม; ตรวจ Duplicate, ย้าย/Copy Fallback, Verify Destination, ลบ Source ตามเกณฑ์ แล้วบันทึก Metadata/Job
- **Request:** Source, Destination และ Absolute Paths ที่เลือก (ไม่ส่ง Binary ผ่าน API)
- **Response:** ผลรายไฟล์, จำนวนรวม และเวลาจริง บันทึกเฉพาะ Metadata ไม่บันทึกรูป
- **Errors:** `INVALID_PATH`, `SOURCE_EQUALS_DESTINATION`, `DESTINATION_WITHIN_SOURCE`, `INVALID_SELECTION`, `DESTINATION_UNAVAILABLE`; Operational Error รายไฟล์อยู่ใน `items`
- **ตัวอย่าง:** รับ `{sourcePath:"C:\\testdata\\source",destinationPath:"D:\\Photos",filePaths:["C:\\testdata\\source\\IMG_001.jpg"]}` → `{jobId:1,totalFiles:1,successCount:1,skippedCount:0,failedCount:0,durationMs:24,items:[{sourcePath:"...",destinationPath:"D:\\Photos\\IMG_001.jpg",status:"moved"}]}`
- **Owner:** Backend / Member 1 ร่วมกับ Database Member 2

### `GetBackupProgress(): BackupProgress`

- **หน้าที่:** อ่านความคืบหน้าปัจจุบัน; ส่ง Event `backup:progress` สำหรับอัปเดตสด
- **Request/Response:** ไม่มี Input; คืน Progress ปัจจุบัน หรือสถานะ Idle
- **Errors:** `INTERNAL_ERROR`
- **ตัวอย่าง:** `{jobId:1,completed:2,total:5,currentPath:"...\\IMG_002.jpg",status:"moving"}`
- **Owner:** Backend / Member 1

### `GetBackupHistory(destinationPath: string): BackupJob[]`

- **หน้าที่:** อ่านประวัติของ Destination ที่ระบุเท่านั้น
- **Request:** พาธปลายทาง
- **Response:** `id,sourcePath,destinationPath,startedAt,completedAt,totalFiles,successCount,failedCount,status,durationMs`
- **Errors:** `DESTINATION_REQUIRED`, `HISTORY_READ_FAILED`
- **ตัวอย่าง:** คืน Job ที่มี `destinationPath` ตรงกับพาธที่รับเท่านั้น
- **Owner:** Database / Member 2

### `GetFileMetadata(fileId: number): FileMetadata`

- **หน้าที่:** อ่าน Metadata ของไฟล์จาก SQLite; UI ใช้ Path ที่ได้เพื่อขอ/แสดงไฟล์จริงจาก Drive ผ่านกลไก Backend/Wails
- **Request:** `fileId`
- **Response:** Metadata เท่านั้น ไม่มีรูปหรือ Thumbnail Binary
- **Errors:** `FILE_NOT_FOUND`, `METADATA_READ_FAILED`; หาก Record มีสถานะ Missing ให้คืน Metadata พร้อม `status:"missing"` เพื่อให้ UI แสดงคำเตือน
- **ตัวอย่าง:** `{fileId:7,fileName:"IMG_001.jpg",path:"E:\\Photos\\IMG_001.jpg",sizeBytes:12345,mimeType:"image/jpeg",aiStatus:"analyzed",description:"A cat indoors",status:"active"}`
- **Owner:** Database / Member 2; เรียกโดย Frontend ผ่าน Wails

### `DeleteFile(fileId: number, destinationPath: string): DeleteResult`

- **หน้าที่:** ลบไฟล์จริงจาก Destination ที่ยืนยัน แล้วปรับ DB หลังลบสำเร็จเท่านั้น
- **Request:** ID และ Destination ที่ต้องตรงกับ Metadata
- **Response:** สถานะ `deleted` และเวลา
- **Errors:** `FILE_NOT_FOUND`, `PATH_SCOPE_MISMATCH`, `DELETE_FAILED`, `DATABASE_UPDATE_FAILED`
- **ตัวอย่าง:** `{fileId:7,destinationPath:"E:\\Photos"}` → `{fileId:7,status:"deleted",deletedAt:"2026-09-01T10:02:00Z"}`
- **Owner:** Backend / Member 1 และ Database / Member 2

### `AnalyzeImage(path: string): AIResult`

- **หน้าที่:** เมื่อผู้ใช้สั่งวิเคราะห์ Go อ่าน Image File จริงจาก Path บน Drive แล้วส่งให้ Gemini 2.5 Flash; SQLite รับเฉพาะ Description หากเป็นไฟล์ใหม่ให้สร้าง FileRecord ด้วย Full Path
- **Request:** Path ของไฟล์จริงที่ผู้ใช้เลือกจาก Scan/Gallery; Go ตรวจ Path และอ่าน Key จาก `GEMINI_API_KEY`
- **Response:** File ID พร้อม Description และสถานะ
- **Errors:** `FILE_NOT_FOUND`, `FILE_MISSING`, `UNSUPPORTED_IMAGE`, `AI_KEY_MISSING`, `AI_UNAVAILABLE`, `AI_RESPONSE_INVALID`, `AI_SAVE_FAILED`
- **ตัวอย่าง:** `{path:"E:\\Photos\\IMG_001.jpg"}` → `{fileId:7,description:"A cat sitting indoors",status:"success"}`
- **Owner:** AI / Member 4 ร่วมกับ Database / Member 2

### `SearchImages(query: string, destinationPath: string): SearchResult[]`

- **หน้าที่:** ค้น Description ใน SQLite แล้วคืน File IDs/Paths สำหรับ Go/UI โหลดรูปจริงจาก Drive
- **Request:** คำค้นและ Destination ปัจจุบันใน Gallery; ห้ามค้นข้าม Destination
- **Response:** Metadata + Path อ้างอิงไฟล์ที่มีอยู่จริง ไม่คืน Image Data; คืนหลายรายการเมื่อหลาย Description ตรงคำค้น
- **Errors:** `DESTINATION_REQUIRED`, `SEARCH_FAILED`
- **ตัวอย่าง:** ค้น `cat` → `[{"fileId":7,"fileName":"IMG_001.jpg","path":"E:\\Photos\\IMG_001.jpg","fileStatus":"available","description":"A cat indoors"}]`
- **Owner:** Database/Search / Member 2 ร่วมกับ Member 4

## Open Questions / Decisions Required

- กลไกส่งไฟล์จริงจาก Go ไป Wails UI เพื่อ Preview โดยไม่เก็บ Thumbnail Binary ใน DB
- Full Path เปลี่ยนเมื่อผู้ใช้ย้ายโฟลเดอร์หรือใช้ Drive บนเครื่องอื่น; Hash Identity เป็นงานต่อยอดหากต้องการรองรับกรณีดังกล่าว
- นโยบาย Duplicate เมื่อ Path เดิมมีไฟล์ใหม่เข้ามา
