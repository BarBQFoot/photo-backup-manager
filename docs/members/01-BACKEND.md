# สมาชิก 1 — คู่มือ Backend / Backup Engine

> ขอบเขตงาน: พฤติกรรมของระบบไฟล์และการสำรอง/ย้ายไฟล์ งานทั้งหมดในเอกสารนี้ยังเป็นแผน จนกว่าจะมีการพัฒนาและตรวจสอบจริง

## ความรับผิดชอบ

ดูแล Drive/File Scanner, Path Validation, Path-based Metadata Matching, Duplicate Check, Move/Copy Fallback, Progress/Timing, Missing Detection และ Delete ประสาน Metadata กับสมาชิก 2 และเปิดผ่าน Wails ตาม API Contract ห้ามให้ Nuxt จัดการ Drive/File System โดยตรง

## ไฟล์และโมดูลที่รับผิดชอบ

โครงสร้างที่เสนอ: `internal/backup/` และ Go tests ที่เกี่ยวข้อง คุยกับทีมก่อนแก้ `app.go`, DTO ที่ใช้ร่วมกัน, Wails bindings ที่สร้างอัตโนมัติ หรือ Schema ปัจจุบัน Repository ยังไม่มีโมดูล Backup

## เมธอดที่รับผิดชอบ

`ScanDrive`, `StartBackup`, `GetBackupProgress`, `GetFileMetadata`, `DeleteFile` และ Missing Detection ให้ยึด `01-API-CONTRACT.md` ผล Scan มี Path/Description ที่จับคู่ได้ แต่ไม่มี Image Binary

## Input / Output

รับพาธ Drive/Source/Destination และ File Paths ส่งกลับ `DriveFile`, `FileMetadata`, `BackupResult`, `BackupProgress` และ `AppError` ตาม Contract ส่ง Event `backup:progress` ให้ Wails Runtime สมาชิก 2 จัดเตรียม Repository สำหรับ FileRecord/Path/Job ห้ามสร้าง Persistence Model แยกเอง

## Dependency

- API Contract และ Flow Spec
- Repository Interface และความหมายของ `FileRecord` จากสมาชิก 2
- ใช้ Full Path จับคู่ Metadata ตามเวอร์ชันส่งงาน; Hash Identity เป็นงานต่อยอด
- Wails Runtime สำหรับส่ง Progress Event
- API ระบบไฟล์ของ OS; ยังไม่เพิ่ม Package/Dependency โดยไม่มีความจำเป็นชัดเจน

## Mock Data

ใช้ชุดข้อมูลใน `docs/test-data/README.md` เช่น `testdata/source/sample01.jpg` และ `duplicate.jpg` ทดสอบใน Temporary Directory หรือสำเนา Fixture เท่านั้น ผลชื่อซ้ำต้องเป็น `status:"skipped"` และไฟล์ต้นทางยังอยู่

## Test Cases

- สแกนพาธที่ใช้ได้ โฟลเดอร์ว่าง พาธที่ไม่มีอยู่ และพาธที่ไม่มีสิทธิ์อ่าน
- เลือกย้ายบางไฟล์แล้วต้องย้ายเฉพาะรายการที่เลือก
- ชื่อไฟล์ซ้ำในปลายทางหรือมี Active Record อยู่แล้ว ต้องไม่เขียนทับ
- ทดสอบ Cross-drive ด้วย Filesystem Test Double: ลบต้นทางหลังปลายทางคัดลอกครบและตรวจสอบผ่านเท่านั้น
- ปฏิเสธกรณี Source เท่ากับ Destination หรือ Destination อยู่ภายใน Source
- Progress เพิ่มตามจำนวนไฟล์ พร้อมสรุปสำเร็จ/ข้าม/ผิดพลาดและเวลาจริง
- Integrity Check รายงานไฟล์ที่หาย เฉพาะ Destination ที่เลือก
- ลบไฟล์ปลายทางก่อนอัปเดตฐานข้อมูล หากลบไม่สำเร็จ Record ต้องยังเป็น Active
- รายงาน Partial Success และ DB Failure อย่างตรงไปตรงมา
- เมื่อเปิดโฟลเดอร์เดิม ให้จับคู่ Description ด้วย Full Path; ทดสอบ Limit ว่าย้าย/เปลี่ยนชื่อแล้วหา Metadata ไม่เจอ
- ไฟล์จริงที่ไม่มี Metadata เป็น `Unanalyzed`; Metadata ที่ไม่มีไฟล์จริงเป็น `Missing` และห้ามแจ้งว่าพร้อม Preview
- API คืน Path/Metadata เท่านั้น ไม่คืน Binary/Base64/Thumbnail Bytes

## จุดเชื่อมต่อ

- DTO และ Error: `01-API-CONTRACT.md`
- Persistence: Repository ของสมาชิก 2 และ `02-DATABASE-SPEC.md`
- การเลือกไฟล์และ Progress UI: สมาชิก 3
- Integration Scenarios: สมาชิก 4 และคู่มือ Test Data

## ห้ามแก้ไข/ทำ

ห้ามทำ UI, ออกแบบ Schema เอง, เปลี่ยน DTO/API โดยไม่ทบทวน, เขียนทับไฟล์ปลายทาง, ลบต้นทางก่อน Verify สำเร็จ, ใช้รูปส่วนตัวทดสอบ หรือเก็บรูป/Base64/BLOB/Thumbnail ใน SQLite ห้าม Implement Search ในโมดูลนี้

## Definition of Done

ทุกเมธอดตรงกับ Contract, Error ใช้รหัสกลาง, ความผิดพลาดไม่ทำให้ต้นฉบับสูญหาย, ไฟล์ซ้ำไม่ถูกเขียนทับ, Progress/Timing ถูกต้อง, การบันทึกข้อมูลจำกัดตาม Destination, ใช้ Fixture ในการทดสอบ และส่งมอบงานครบ

## สถานะการพัฒนาปัจจุบัน

สถานะนี้อัปเดตตามโค้ดที่มีอยู่จริงใน Repository ไม่ได้หมายความว่างานตาม Contract เสร็จครบทั้งระบบ

### ทำเสร็จแล้ว

- **Scanner ระดับ File System:** สแกนโฟลเดอร์และโฟลเดอร์ย่อย, กรองไฟล์รูป, อ่านชื่อ/Path/ขนาด/MIME Type/Modified Time และคืนสถานะเริ่มต้น `available` กับ `unanalyzed`
- **Path Validation:** ตรวจ Source/Destination, Path ว่างหรือไม่มีอยู่จริง, Path ที่ไม่ใช่โฟลเดอร์, Source กับ Destination เดียวกัน และ Destination ที่อยู่ภายใน Source
- **Selected File Validation:** ตรวจว่ารายการที่เลือกมีอยู่จริง เป็นไฟล์ปกติ และอยู่ภายใต้ Source
- **Duplicate Check ระดับ File System:** หากไฟล์ปลายทางมีอยู่แล้วจะคืน `skipped`, ไม่เขียนทับ และไม่ลบไฟล์ต้นทาง
- **Move/Copy Fallback:** ลอง Rename ก่อน หากไม่สำเร็จจะ Copy ผ่านไฟล์ชั่วคราว ตรวจสอบขนาดไฟล์ปลายทาง แล้วจึงลบต้นทาง
- **StartBackup ระดับ File System:** ย้ายไฟล์ที่เลือกทีละรายการ และคืนผล `moved`, `skipped`, `failed`, จำนวนผลลัพธ์ และ `durationMs`
- **Progress ระดับ Service:** มี `GetBackupProgress`, สถานะ `idle`, `moving`, `completed`, ค่า `completed`, `total` และ `currentPath` พร้อมป้องกันการอ่านข้อมูลพร้อมกัน
- **Delete ระดับ File System:** ตรวจขอบเขต Destination, ตรวจไฟล์จริงและลบไฟล์ พร้อมคืนเวลาที่ลบสำเร็จ
- **Drive Service เบื้องต้น:** เรียก `ScanDrive` และ `StartBackup` จาก Service ได้ โดยยังไม่เชื่อม Wails หรือ Database
- **Unit Tests:** ใช้ Temporary Directory/Fixture ครอบคลุม Scanner, Path Validation, Duplicate, Move/Copy Fallback, StartBackup, Progress และ Delete File System

### ยังขาดและทำต่อได้โดยไม่ต้องรอ Database

- **Progress Event:** ส่ง Event `backup:progress` ผ่าน Wails Runtime ยังไม่ได้ทำ
- **Test Coverage เพิ่มเติม:** Partial Success, ไฟล์หายระหว่างทำงาน, Destination ใช้งานไม่ได้ และกรณีลบต้นทางหลัง Copy ไม่สำเร็จ

### ยังขาดและต้องประสาน Member 2/ทีม

- **Database Job/Metadata:** สร้าง `BackupJob`, ได้ `jobId` จริง และบันทึก FileRecord หลังย้ายสำเร็จ
- **Active Record Duplicate:** ตรวจ Duplicate จาก Database เพิ่มเติมจากการตรวจไฟล์จริง
- **Path-based Metadata Matching:** จับคู่ `fileId`, `description` และ `aiStatus` ด้วย Full Path
- **Missing Detection:** แสดง Metadata ที่ไม่มีไฟล์จริงเป็น `missing` โดยไม่ลบ Record อัตโนมัติ
- **Database-aware Delete:** อัปเดตสถานะเป็น `deleted` หลังลบไฟล์จริงสำเร็จเท่านั้น และคง Active เมื่อการลบล้มเหลว
- **Wails Integration:** เชื่อม Service กับ `app.go`, DTO กลาง, Error Mapping และ Generated Bindings หลังตกลงกับทีม

### ข้อจำกัดปัจจุบัน

- `jobId` ใน `BackupResult` ยังเป็นค่าเริ่มต้น เพราะยังไม่มี Database Repository
- Duplicate ตอนนี้ตรวจเฉพาะไฟล์จริงใน Destination ยังไม่ตรวจ Active Record
- การ Verify ไฟล์ตรวจขนาดไฟล์ ยังไม่ได้ใช้ Checksum/Hash
- ยังไม่มี Metadata Matching, Missing Detection, Progress Event หรือ Wails Binding
- Delete ตอนนี้ยังไม่อัปเดตสถานะ Database เพราะยังไม่มี `fileId` และ Repository

การตรวจล่าสุดของส่วนที่พัฒนาแล้วใช้คำสั่ง:

```text
go test ./...
go vet ./internal/backup ./service
```

## Handoff Checklist

- [ ] แจ้งไฟล์ที่แก้และ Exported Methods
- [ ] ตรวจความเข้ากันได้กับ Contract ร่วมกับสมาชิก 2 และ 3
- [ ] แจ้ง Test Fixture/Scenario และผลที่ได้
- [ ] ระบุพฤติกรรมข้าม Platform หรือ Case Sensitivity ที่ยังไม่สรุป
- [ ] ยืนยันว่าไม่ได้ใช้หรือ Commit ไฟล์ส่วนตัวและ Secret
