# UI Spec — Photo Backup Manager

> อ้างอิง Workflow ของ FinalPhotoMover และเพิ่ม Gallery, AI Description และการค้น Description ภายใน Gallery สำหรับรุ่นส่งงาน

## หลักการร่วม

- Desktop UI ผ่าน Nuxt/Wails; Go เป็นผู้เข้าถึง File System และ SQLite
- ภาพจริงแสดงจาก Drive; SQLite เก็บ Metadata/Path/Description/History เท่านั้น
- Full Path ใช้จับคู่ Metadata ในรุ่นแรก
- สถานะสำคัญต้องมีข้อความกำกับ; Missing ห้ามแสดงเป็นภาพพร้อมเปิด
- ออกแบบ Empty/Working/Result-or-Error ภายในหน้าหลัก ไม่สร้างหน้าจอแยกทุกสถานะ

## Move Photos

- เลือก Source/Destination ผ่าน native Folder Picker และแสดงพาธ
- สแกน Source แสดงรายการรูปจริง, จำนวน, Thumbnail และขนาด
- เลือกบางไฟล์หรือทั้งหมด; ตรวจชื่อซ้ำจากปลายทางและฐานข้อมูล
- ยืนยันจำนวนและพาธก่อนเริ่ม Move
- แสดง Progress รายไฟล์และผลรวม Success/Skipped/Failed/Duration
- ใช้ `ScanDrive`, `StartBackup`, `GetBackupProgress`/`backup:progress`

## Gallery / Image Detail

- Gallery เป็น Drive Browser + Metadata Viewer สำหรับ Destination ที่เลือก; ช่องค้นหาในหน้านี้ค้น Description เฉพาะ Destination เดียวกัน
- Grid เรียบง่าย: Thumbnail จริง, Filename, AI Status, File Status
- เลือกภาพแล้วแสดง Filename, Full Path, Description และสถานะใน Detail Panel
- `Unanalyzed` มีปุ่ม Analyze ที่ผู้ใช้สั่งเอง; `Missing` แสดงคำเตือนและไม่มี Preview
- เมื่อช่องค้นหาว่างให้แสดงรูปในโฟลเดอร์ตามปกติ; เมื่อพิมพ์คำค้นให้กรอง Grid เดิมด้วย Description
- ใช้ `ScanDrive`, `GetFileMetadata`, `AnalyzeImage(path)`, `SearchImages(query, destinationPath)`

## History & Check

- แสดง Backup Jobs ของ Destination: เวลา, ต้นทาง/ปลายทาง, จำนวนที่ย้าย/ข้าม/ล้มเหลว และ Duration
- ใช้ `ScanDrive(destinationPath)` แสดงไฟล์จริงและ Missing Records; ใช้ปุ่ม Refresh/Check เพื่อสแกนสถานะใหม่
- รายการไฟล์ปลายทางที่ลบได้ต้องแสดงชื่อ/พาธและยืนยันก่อนทำ
- ใช้ `GetBackupHistory(destinationPath)`, `ScanDrive(destinationPath)` และ `DeleteFile(fileId, destinationPath)`

## ขอบเขตออกแบบ

ไม่ออกแบบ Tag/Tag Cloud/Category/Hash/Changed Detection/Advanced Filters/Settings/Cloud Sync หรือ Phone Import เฉพาะทาง
