# Flow Spec — Drive-Based Photo Storage

> รูปจริงอยู่บน Drive/Filesystem; SQLite เก็บเฉพาะ File Metadata/Path/AI Description/Backup History ไม่มี Tag ในเวอร์ชันส่งงาน

## เปิดโฟลเดอร์ซ้ำและจับคู่ Metadata (เวอร์ชันส่งงาน)

```text
ผู้ใช้เปิดโปรแกรมและเลือก Drive/ปลายทาง
  → Go Scan Drive และค้น Image Files จริง
  → อ่าน Path/Filename/Size/ชนิด/Modified Time
  → ค้น FileRecord ใน SQLite ด้วย Full Path
      → พบ Path: จับคู่ Description เดิม
      → ไม่พบ Path: แสดง New File/Unanalyzed; ยังไม่เรียก AI จนผู้ใช้สั่ง
      → DB มี Record ใต้โฟลเดอร์เดิมแต่ไฟล์หาย: แสดง Missing และคง Description
  → Gallery แสดงรูปจริงจาก Drive พร้อม Description จาก SQLite
```

การจับคู่ด้วย Full Path ทำงานเมื่อโฟลเดอร์ยังอยู่ที่เดิมเท่านั้น หากย้ายโฟลเดอร์หรือเปลี่ยนชื่อไฟล์จะไม่พบ Description เดิม เวอร์ชันส่งงานยอมรับข้อจำกัดนี้; Hash Identity เลื่อนไปเป็นงานต่อยอด

## Missing File Flow

```text
SQLite มี FileRecord/Metadata
  → สแกน Drive แล้วไม่พบไฟล์ที่ตรงกับ Identity/Path
  → ตั้งสถานะ Missing และคง Metadata ไว้
  → Gallery แสดง ⚠ File Missing และไม่เปิด Preview
  → ไม่ลบ Metadata อัตโนมัติ; Remove/Delete ต้องทำตาม Business Rule ที่ยืนยัน
```

## Backup Flow

```text
เลือก Source และ Destination
  → ตรวจพาธ/ปฏิเสธ Source=Destination หรือ Destination ซ้อนใน Source
  → Scan Source และแสดงไฟล์จริง
  → Check Duplicate จากไฟล์จริงปลายทางและ Metadata ที่เกี่ยวข้อง
  → Create BackupJob
  → Move ไฟล์; หากข้ามไดรฟ์ Copy ไปปลายทางและ Verify ก่อนลบ Source ตาม Requirement
  → บันทึก File Metadata/Full Path หลัง Destination ผ่าน Verify; ไม่บันทึก Image Binary
  → AI Analysis เกิดเฉพาะเมื่อผู้ใช้กด Analyze
  → สรุป Success/Skip/Failure และเวลาจริง
```

ห้ามลบ Source เมื่อ Move/Copy/Verify ล้มเหลว ห้ามเขียนทับไฟล์จริงโดยไม่แจ้ง และไม่ถือว่าบันทึกสำเร็จหาก DB Update ล้มเหลว

## Gallery Flow

```text
ผู้ใช้เลือก/เปิด Drive
  → Scan ไฟล์รูปจริง
  → อ่าน Path/Filename
  → Match Metadata ใน SQLite
  → Found: แสดงรูปจริง + Description/AI Status
  → Not Found: แสดง New File/Unanalyzed
  → เปิด Image Detail โดยอ้างไฟล์จริงจาก Drive
```

Gallery คือ Drive Browser + Metadata Viewer ไม่ใช่ Image Storage ภาพ Thumbnail ใช้ไฟล์จริงหรือ Cache ที่กำหนดภายหลัง และห้ามเก็บ Thumbnail Binary ใน DB

## Description Search ใน Gallery

```text
ผู้ใช้เปิด Gallery ของ Destination ที่เลือก แล้วพิมพ์/แก้คำค้น Description
  → ค้น Description ใน SQLite
  → จำกัดผลตาม Destination เดียวกับ Gallery
  → คืน File IDs และ Paths/Description
  → ตรวจไฟล์จริงบน Drive และกรอง Missing/Deleted
  → แสดงผลเป็นรูปจริงจาก Drive
```

Gallery แสดงรูปทั้งหมดเมื่อช่องค้นหาว่าง และกรองเป็นผลหลายรูปเมื่อมีคำค้น ต้องไม่แสดง Missing/Deleted เป็นไฟล์ที่เปิดได้

## AI Analysis Flow

```text
เลือกภาพใน Gallery
  → Go ตรวจว่ามีไฟล์จริงบน Drive
  → อ่าน Image Bytes จาก Filesystem ในหน่วยความจำเพื่อส่ง Gemini (ไม่บันทึก Bytes ลง DB)
  → รับ Description
  → Validate Description
  → บันทึก/อัปเดต FileRecord และเฉพาะ Description/AI Status/Model/เวลาใน SQLite (ไม่เก็บ Image Bytes)
  → UI แสดง Description ประกอบภาพจริงจาก Drive
```

เมื่อ API ล้มเหลว ห้ามแก้/ลบไฟล์ภาพ Key อ่านจาก `GEMINI_API_KEY` เท่านั้น

## Open Questions / งานต่อยอด (ไม่บล็อกเวอร์ชันส่งงาน)

- Hash Identity เพื่อรองรับการย้าย/เปลี่ยนชื่อไฟล์
- Thumbnail/Preview ผ่าน Wails โดยไม่เก็บใน DB
- Database Location และการใช้ Drive บนเครื่องอื่น
- Metadata Portability/Export-Import
- เกณฑ์ลบ Metadata เมื่อไฟล์หายหรือผู้ใช้ Remove
