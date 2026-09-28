# สมาชิก 2 — คู่มือ Database / GORM

> ขอบเขตงาน: จัดเก็บ Metadata ด้วย SQLite/GORM เท่านั้น ปัจจุบันมี Database Schema, Migration, Generated Models/Queries และ Repository เบื้องต้นแล้ว สมาชิก 1 เชื่อม Repository เข้ากับ `DriveService` แล้ว ส่วนการเชื่อมกับ AI Service ให้ประสานสมาชิก 4

## ความรับผิดชอบ

ดูแล SQLite/GORM สำหรับ Metadata เท่านั้น: FileRecord/Path Index, AI Description, Backup History และการเปิดแอปใหม่ ไม่มี Tags/Tag Cloud ในขอบเขตนี้ ตรวจ Schema ให้แน่ใจว่าไม่มี Image Binary/Base64/BLOB/Thumbnail Binary

## Models และ Tables

- `BackupJob` → `backup_jobs`: ต้นทาง/ปลายทาง เวลาเริ่ม/จบ จำนวนไฟล์ ระยะเวลา และสถานะ
- `FileRecord` → `file_records`: File ID, Filename, Path, Size, MimeType, ModifiedAt, AI Status/Description, BackupJobID และ Status

รายละเอียด Column, Constraint, Relationship, Index และ Status ให้อ้างอิง `02-DATABASE-SPEC.md` ห้ามสร้าง Schema ที่แยกจาก Contract กลาง

Models และ Query Helpers สร้างจาก GORM Gen อยู่ใน `model/model/` และ `model/query/` ตามลำดับ ไม่แก้ไฟล์ `.gen.go` ด้วยมือ ให้แก้ Schema/Generator แล้ว Generate ใหม่เมื่อจำเป็น

## Repository Methods

เมธอดที่มีอยู่:

- `BackupJobStore` ใน `repository/backup_job.go`: `Create`, `Finish`, `ListByDestination`
- `FileRecordStore` ใน `repository/file_record.go`: `SaveByPath`, `GetByID`, `GetByPath`, `UpdateDescription`, `UpdateStatus`, `SearchByDescription`

`SearchByDescription` จำกัดผลตาม Destination จาก Full Path และคืนเฉพาะ Record สถานะ `active` ส่วนการค้นและการทำงานกับไฟล์จริงเป็นหน้าที่ของ Service ที่เกี่ยวข้อง ไม่ใช่ Repository

### เมธอดที่แต่ละสมาชิกใช้

- **สมาชิก 1 — Backup:** ใช้ `BackupJobStore.Create`, `Finish`, `ListByDestination` และ `FileRecordStore.SaveByPath`, `GetByPath`, `UpdateStatus` สำหรับประวัติงานและสถานะ Metadata
- **สมาชิก 4 — AI:** ใช้ `FileRecordStore.SaveByPath`, `GetByID`, `UpdateDescription`, `SearchByDescription` สำหรับผูกคำอธิบายกับ FileRecord และค้นใน Gallery
- `SearchByDescription(ctx, destination, keyword)` ค้นเฉพาะ FileRecord สถานะ `active` ที่ Full Path อยู่ภายใน Destination ที่ระบุ
- Signatures และชนิดพารามิเตอร์จริงให้อ้างอิง Interface ใน `repository/backup_job.go` และ `repository/file_record.go`; หาก Service ต้องการเมธอดหรือ Signature เพิ่ม ให้ตกลงกับสมาชิก 2 ก่อนเปลี่ยน

## Dependency

ใช้ SQLite ผ่าน `github.com/glebarez/sqlite`, GORM และ GORM Gen โดยมี Dependencies ใน `go.mod` แล้ว เก็บ DB ไว้หลัง Repository และไม่ให้ Frontend เรียก GORM โดยตรง

## โครงสร้างแพ็กเกจตามที่เรียนในห้อง

ยึดแนวแบ่งชั้นจาก `foodie-app` โดยปรับให้เหมาะกับข้อมูลรูปภาพ ไม่ต้องยกโค้ดหรือระบบ User มาทั้งชุด:

```text
database/
  schema.sql         DDL ของตารางและ Index
  migrate.go         อ่าน Schema ที่ฝังในโปรแกรมและติดตั้งแบบ transaction
model/model/
  file_records.gen.go  Generated GORM Model ของ FileRecord
  backup_jobs.gen.go  Generated GORM Model ของ BackupJob
model/query/
  *.gen.go           Generated Query Helpers
repository/
  dbconnect.go       เปิด SQLite, เปิด Foreign Keys และเรียก Migration
  file_record.go     อ่าน/บันทึก FileRecord และค้น Description
  backup_job.go      สร้าง/ปิด Job และอ่าน History
  main/model_gen.go  สคริปต์ GORM Gen
main.go              เปิด DB/Migration ก่อนเริ่ม Wails
  app.go               เปิด DB, สร้าง Repository และส่งให้ DriveService
```

- `database` เก็บ Schema และ Migration; `repository/dbconnect.go` เปิดการเชื่อมต่อ
- `model/model` และ `model/query` เป็นไฟล์ที่ GORM Gen สร้างจากตาราง SQLite
- `repository` ห่อ query และ transaction; สมาชิก 2 เป็นเจ้าของส่วนนี้และประสาน signature กับ Backend/AI
- `service` เป็นชั้น use case; ประสานกับผู้ดูแล Backend/AI ว่าใครเป็นเจ้าของแต่ละ service เพื่อไม่ให้ทำซ้ำ
- `main.go` เปิด DB และเรียก Migration ก่อนเปิดหน้าต่าง Wails; `app.go` สร้าง Repository แล้วส่งให้ `DriveService` ใช้กับ Backup/Metadata
- สคริปต์ Generate คือ `go run ./repository/main`; รันจากโฟลเดอร์รากโปรเจกต์เมื่อแก้ Schema แล้วต้องสร้าง Models/Queries ใหม่
- ไม่คัดลอก `.env`, ฐานข้อมูล `foodie.db`, credentials หรือข้อมูล User จากโปรเจกต์ตัวอย่างมาใช้

## สถานะปัจจุบัน

- Migration ถูกเรียกจาก `main.go` ผ่าน `repository.NewDbConnection()` โดยค่าเริ่มต้นใช้ `database/photo_backup.db`; กำหนดไฟล์อื่นได้ด้วยตัวแปร `PHOTO_BACKUP_DB`
- ตรวจ Schema ของ `photo_backup.db` แล้วพบตาราง, CHECK constraints, Foreign Key, Index และ `schema_migrations` ตาม `database/schema.sql`; Integrity และ Foreign Key checks ผ่านในขณะตรวจ
- ทดลอง Repository ในฐานข้อมูล SQLite ในหน่วยความจำแล้ว: อ่านตาม ID, อัปเดต Description/Status, ค้นแยก Destination และปิด/อ่าน Job ผ่าน ตัวอย่างชั่วคราวถูกลบหลังทดสอบ
- สมาชิก 1 เชื่อม `FileRecordStore` และ `BackupJobStore` เข้ากับ `DriveService` แล้ว; ฟังก์ชัน Drive/Backup ถูกเปิดผ่าน Wails
- การเชื่อม Repository เข้ากับ AI Service และการใช้ AI Description/Search ยังต้องประสานกับสมาชิก 4
- การป้องกัน Path ซ้ำยังไม่มี Unique Constraint ใน SQLite; `SaveByPath` ตรวจและบันทึกภายใน Transaction ตามนโยบายรุ่นแรกด้านล่าง หากต้องรองรับการเขียนพร้อมกันหลาย Process ต้องทบทวนเพิ่ม
- นโยบายรุ่นแรก: Path เดิมในปลายทางเดิมอัปเดต Record เดิม; Path ต่างกันเป็นคนละ Record การเขียนพร้อมกันหลาย Process ยังไม่รับประกันว่าจะไม่มี Path ซ้ำ
- DB ระหว่างพัฒนาใช้ `database/photo_backup.db` แบบ Relative Path จาก Working Directory; `PHOTO_BACKUP_DB` override ได้ ก่อนแจกจ่าย EXE ต้องกำหนดตำแหน่ง DB/Working Directory ให้แน่นอน
- ยังไม่ได้ตรวจพฤติกรรมปิด/เปิดแอปเพื่อกู้ Job ที่ค้างเป็น `interrupted`

## Mock Data

สร้างเฉพาะใน Test Database ชั่วคราว: Job ของ `testdata/destination`, Active `sample01.jpg` พร้อม Description, Record ที่ Missing หนึ่งรายการ และ Deleted หนึ่งรายการ ห้ามใส่พาธเครื่องจริงหรือ Metadata ส่วนตัวใน Fixture ที่ Commit

## Test Cases

- Migration สร้างตารางและ Constraint ใน SQLite ชั่วคราวได้
- Foreign Key และ Path Index ทำงานตาม Spec
- History และ Metadata ของปลายทาง A ไม่ปนกับปลายทาง B
- ตรวจพฤติกรรม Path ซ้ำ; ปัจจุบัน Transaction ช่วยจัดลำดับการทำงาน แต่ไม่มี Unique Constraint ป้องกันกรณีเรียกพร้อมกัน
- สถานะ `deleted` และ `missing` แตกต่างกันและจัดการ Metadata ตาม Contract
- ตรวจ Schema แล้วไม่มี Binary, Base64, BLOB หรือ Thumbnail Bytes
- Path ที่เดิมเปิดซ้ำแล้วจับคู่ Description ได้; ย้าย/เปลี่ยนชื่อเป็นข้อจำกัดที่ทราบ
- Job, จำนวน, สถานะ และเวลาอยู่ครบหลังปิด/เปิดใหม่; Job ค้างเปลี่ยนเป็น `interrupted` ได้
- AI Description/Status บันทึกและอ่านได้โดยผูกกับ FileRecord ที่ถูกต้อง; AI Model ไม่อยู่ใน Schema เวอร์ชันปัจจุบัน
- Search Description ต้องรองรับคำค้นภาษาไทยและไม่แสดงไฟล์ Deleted/Missing เป็นไฟล์ที่เปิดได้

## จุดเชื่อมต่อ

- สมาชิก 1 ใช้ Backup Job/File Repository
- สมาชิก 4 ใช้ AI Description Repository
- สมาชิก 3 รับ DTO ผ่าน Wails เท่านั้น ไม่ส่ง GORM Model เป็น UI Contract โดยไม่ทบทวน
- Schema/Migration ต้องให้ทีมทบทวนก่อนเปลี่ยน

## ห้ามแก้ไข/ทำ

ห้ามเก็บรูปจริง/Thumbnail ใน DB, ทำระบบไฟล์หรือ UI, เปลี่ยน DTO/API โดยไม่ทบทวน, Query ข้าม Drive โดยไม่ตั้งใจ, เพิ่ม Tag/Hash Schema ที่อยู่นอกขอบเขตโดยไม่ทบทวน หรือใช้ Cascade Delete ที่ทำลาย Metadata โดยไม่กำหนดนโยบาย

## Definition of Done

Models/Migrations ตรง Spec, CRUD ใช้ GORM, Key/Relationship/Unique/Index บังคับใช้จริง, Repository จำกัดข้อมูลตาม Destination, เปิดแอปใหม่แล้วยังอ่านข้อมูลได้ และ Transaction ปกป้องข้อมูลที่เกี่ยวข้อง

## Handoff Checklist

- [ ] แจ้ง Models, Tables, Migrations และ Repository Signatures
- [ ] อธิบายวิธี Migration/Reopen และนโยบายตำแหน่งไฟล์ DB
- [ ] ยืนยัน Query แบบแยก Destination กับสมาชิก 1 และ 4
- [ ] ส่งผลทดสอบจาก Test Database แยกต่างหาก
- [ ] แจ้งข้อจำกัดการจับคู่ด้วย Full Path และยืนยันตำแหน่ง DB สำหรับ EXE ก่อนแพ็กส่ง (ปัจจุบันค่าเริ่มต้นเป็น Relative Path จาก Working Directory)
