# สมาชิก 3 — คู่มือ Frontend / Nuxt

> ขอบเขตงาน: UI และสถานะการแสดงผล ปัจจุบัน `frontend/` ยังเป็น Nuxt/Wails Scaffold แบบเริ่มต้น หน้าผลิตภัณฑ์ทั้งหมดเป็นแผน

## ความรับผิดชอบ

พัฒนา 3 หน้าหลักตาม UI Spec: Move Photos, Gallery และ History & Check ไม่แยก Dashboard/Search เป็นหน้าใหม่; Gallery มีช่องค้น Description ที่จำกัดผลตาม Destination ที่เลือก แสดงรูปจริงจาก Drive และเรียก Go ผ่าน Wails เท่านั้น

## ไฟล์/โมดูลที่รับผิดชอบ

โครงสร้างที่เสนอ: `frontend/app/pages/`, `frontend/app/components/` และ Frontend State/Composable ที่เกี่ยวข้อง ปรับตามโครงสร้าง Nuxt ปัจจุบัน ก่อนแก้ `frontend/wailsjs/` ซึ่งเป็นไฟล์สร้างอัตโนมัติ หรือ Shared Go Bindings ให้ประสานทีมก่อน

## Methods / Data

เรียก `ScanDrive`, `StartBackup`, `GetBackupProgress`/event `backup:progress`, `GetBackupHistory`, `GetFileMetadata`, `DeleteFile`, `AnalyzeImage(path)` และ `SearchImages` ตาม `01-API-CONTRACT.md` ห้ามอ่าน SQLite หรือทำ File System Operation ใน Browser Code

## Dependency

Nuxt Scaffold, Wails Runtime/Bindings, API Contract, UI Spec และ Mock Fixtures การจัดหน้าและพัฒนาสถานะไม่จำเป็นต้องรอ Backend/Database จริง

## Mock Data

ใช้ตัวอย่างจาก `01-API-CONTRACT.md` และ `docs/test-data/README.md` แยก Mock Mode ให้ชัดเจน เพื่อให้สลับเป็น Wails Calls ได้ ห้ามแสดง Mock ว่าเป็นผลการทำงานจริงหรือข้อมูลที่บันทึกแล้ว

## Test Cases

- ทุกหน้ามีสถานะ Loading, Empty, Success และ Error ตามกรณี
- เปิด History โดยยังไม่เลือก Destination แล้วเห็นข้อความแนะนำ ไม่เกิด Crash
- Progress แสดงชื่อไฟล์และจำนวนสำเร็จ/ข้าม/ผิดพลาด; เมื่อจบแสดงเวลารวม
- สถานะ Duplicate/Missing มีข้อความกำกับ ไม่สื่อด้วยสีอย่างเดียว
- Delete Confirmation ระบุชื่อไฟล์และปลายทางชัดเจน
- ช่องค้นใน Gallery พิมพ์/แก้ Description Query ได้ รองรับภาษาไทย และคืนหลายรูปจาก Destination เดียวกัน
- รองรับ Keyboard Navigation/Focus และกรณีเปิด Preview ไม่ได้
- Gallery แสดงรูปจริงผ่าน Path/กลไก Wails; API ส่ง Metadata/Path ไม่ส่ง Image Blob
- New/Unanalyzed และ Missing แสดงต่างกัน; Missing ห้าม Preview

## จุดเชื่อมต่อ

- สมาชิก 1: DTO การเลือกไฟล์, ผลย้าย และ Progress Events
- สมาชิก 2: History/File/AI DTO ผ่าน Public Contract เท่านั้น
- สมาชิก 4: สถานะ AI, Error และขั้นตอน Demo
- การเปลี่ยน Contract ต้องให้สมาชิกที่ได้รับผลกระทบทบทวน

## ห้ามแก้ไข/ทำ

ห้ามเขียน File System หรือ SQLite/GORM Logic ใน Nuxt, โหลดรูปจาก DB, เพิ่ม Tag Cloud หรือ Search Filters นอกขอบเขต, แก้ Generated Bindings ด้วยมือ, สร้าง Method นอก Contract, เปิดเผย API Key/Stack Trace หรือทำปุ่มลบให้กำกวม

## Definition of Done

หน้าจอตรง `03-UI-SPEC.md`, เรียก API ตาม Contract, เปลี่ยนจาก Mock ได้, ครบทุกสถานะและ Error, ไม่มี Logic ระบบไฟล์/ฐานข้อมูลใน Frontend และใช้งาน Navigation/Accessibility พื้นฐานได้

## Handoff Checklist

- [ ] แจ้งหน้า/Component และ Contract Methods ที่ใช้
- [ ] อธิบาย Mock Adapter และวิธีสลับเป็น Wails Bindings
- [ ] สาธิตสถานะ Empty, Loading, Error และ Success
- [ ] ยืนยันว่าไม่ได้แก้ Generated Files ด้วยมือ
- [ ] แจ้งช่องว่าง API/UI ให้สมาชิก 1, 2 และ 4 ทราบ
