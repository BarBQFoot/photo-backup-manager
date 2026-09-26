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

## Handoff Checklist

- [ ] แจ้งไฟล์ที่แก้และ Exported Methods
- [ ] ตรวจความเข้ากันได้กับ Contract ร่วมกับสมาชิก 2 และ 3
- [ ] แจ้ง Test Fixture/Scenario และผลที่ได้
- [ ] ระบุพฤติกรรมข้าม Platform หรือ Case Sensitivity ที่ยังไม่สรุป
- [ ] ยืนยันว่าไม่ได้ใช้หรือ Commit ไฟล์ส่วนตัวและ Secret
