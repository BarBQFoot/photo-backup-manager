# สมาชิก 2 — คู่มือ Database / GORM

> ขอบเขตงาน: การจัดเก็บข้อมูลด้วย SQLite ปัจจุบัน `go.mod` และ Source Code ยังไม่มี GORM/SQLite implementation

## ความรับผิดชอบ

ดูแล SQLite/GORM สำหรับ Metadata เท่านั้น: FileRecord/Path Index, AI Description, Backup History และการเปิดแอปใหม่ ไม่มี Tags/Tag Cloud ในขอบเขตนี้ ตรวจ Schema ให้แน่ใจว่าไม่มี Image Binary/Base64/BLOB/Thumbnail Binary

## Models และ Tables

- `BackupJob` → `backup_jobs`: ต้นทาง/ปลายทาง เวลาเริ่ม/จบ จำนวนไฟล์ ระยะเวลา และสถานะ
- `FileRecord` → `file_records`: File ID, Filename, Path, Size, MimeType, ModifiedAt, AI Status/Description, BackupJobID และ Status

รายละเอียด Column, Constraint, Relationship, Index และ Status ให้อ้างอิง `02-DATABASE-SPEC.md` ห้ามสร้าง Schema ที่แยกจาก Contract กลาง

## Repository Methods

ออกแบบเมธอดสำหรับสร้าง/ปิด Job, เพิ่ม/อัปเดต File Metadata, ค้น FileRecord ด้วย File ID/Full Path, อ่าน History ตาม Destination, ปรับสถานะ Missing/Deleted, อ่าน/บันทึก AI Description และค้น Description โดยจำกัดตาม Destination

## Dependency

ใช้ Shared Schema/API Specs, SQLite และ GORM (เป็น Dependency ที่วางแผนไว้แต่ยังไม่มี), Go Domain DTOs และความต้องการจาก Backup/AI Service แยก DB ไว้หลัง Repository และไม่ผูกกับ UI

## Mock Data

สร้างเฉพาะใน Test Database ชั่วคราว: Job ของ `testdata/destination`, Active `sample01.jpg` พร้อม Description, Record ที่ Missing หนึ่งรายการ และ Deleted หนึ่งรายการ ห้ามใส่พาธเครื่องจริงหรือ Metadata ส่วนตัวใน Fixture ที่ Commit

## Test Cases

- Migration สร้างตารางและ Constraint ใน SQLite ชั่วคราวได้
- Foreign Key และ Path Index ทำงานตาม Spec
- History และ Metadata ของปลายทาง A ไม่ปนกับปลายทาง B
- ป้องกัน Active Record ซ้ำด้วย Transaction
- สถานะ `deleted` และ `missing` แตกต่างกันและจัดการ Metadata ตาม Contract
- ตรวจ Schema แล้วไม่มี Binary, Base64, BLOB หรือ Thumbnail Bytes
- Path ที่เดิมเปิดซ้ำแล้วจับคู่ Description ได้; ย้าย/เปลี่ยนชื่อเป็นข้อจำกัดที่ทราบ
- Job, จำนวน, สถานะ และเวลาอยู่ครบหลังปิด/เปิดใหม่; Job ค้างเปลี่ยนเป็น `interrupted` ได้
- AI Description/Status/Model บันทึกและอ่านได้โดยผูกกับ FileRecord ที่ถูกต้อง
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
- [ ] ระบุข้อจำกัด Path Matching และประเด็น Database Location ที่ยังรอสรุป
