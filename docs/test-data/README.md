# คู่มือชุดข้อมูลทดสอบ

> ขอบเขตส่งงาน Description-only: Fixture ภาพอยู่บน Drive; SQLite เก็บ FileRecord/Full Path/AI Description/Job เท่านั้น ห้ามมี Image Binary, Base64, BLOB, Thumbnail Binary, Tag หรือ Hash ใน Schema รุ่นแรก

การทดสอบ Move/Delete ต้องใช้ Fixture ที่ลบทิ้งได้ใน `testdata/` เท่านั้น ห้ามใช้คลังรูปส่วนตัว ยังไม่มีไฟล์ภาพ Fixture ใน Repository; หากจะเพิ่ม ให้ใช้ภาพที่สร้างขึ้นเองหรือได้รับอนุญาตให้แจกจ่าย และตรวจสิทธิ์ก่อน Commit

## โครงสร้างที่แนะนำ

```text
testdata/
├── source/
│   ├── sample01.jpg
│   ├── sample02.jpg
│   ├── duplicate.jpg
│   └── nested/optional-sample.jpg
├── destination/
│   └── (ผลลัพธ์ชั่วคราว เริ่มต้นควรว่าง)
└── destination-other/
    └── (ใช้ตรวจการแยกข้อมูลตามปลายทาง)
```

สร้างโฟลเดอร์ Source/Destination สำหรับการทดสอบในเครื่อง เก็บผลลัพธ์ที่สร้างและ SQLite DB ในเครื่อง ห้าม Commit การทดสอบควร Copy Fixture ไป Temporary Directory เพื่อไม่แก้ Fixture ต้นฉบับ

## Scenarios

| Scenario | วิธีเตรียม/ดำเนินการ | ผลที่คาดหวัง |
| --- | --- | --- |
| Normal Backup | ย้าย Fixture หนึ่งไฟล์ไป Destination ว่าง | มีไฟล์ปลายทาง; ลบ Source หลังตรวจสำเนาสำเร็จ; มี Active Record และ Job หนึ่งรายการ |
| เลือกบางไฟล์ | Scan หลายไฟล์แล้วเลือกย้ายหนึ่งไฟล์ | ย้ายเฉพาะไฟล์ที่เลือก |
| Duplicate | สร้างชื่อเดียวกันในปลายทางและ/หรือ Active DB Record | ข้ามพร้อมเตือน; ข้อมูลเดิมไม่เปลี่ยน; ไม่มี Active Record ซ้ำ |
| Cross-drive | ใช้ Removable Drive หรือ Filesystem Test Double | ตรวจสำเนาให้ครบก่อนลบ Source; หากล้มเหลว Source ยังอยู่ |
| Missing File | ลบ Fixture ใน Destination นอกแอปแล้วสั่ง Integrity Check | รายงาน/ปรับ Record เป็น `missing`; Preview แจ้งว่าไฟล์หาย |
| Delete | ลบ Fixture ปลายทางที่ระบบจัดการผ่านแอป | ไฟล์ถูกลบจริง, Record เป็น `deleted`, ไฟล์อื่นไม่เปลี่ยน |
| AI Analysis | ใช้ภาพตัวอย่างที่ไม่ใช่ภาพส่วนตัวกับ Gemini ที่ได้รับอนุญาต หรือ Mock Adapter | บันทึก Description/AI Status; API Error ไม่กระทบไฟล์ |
| Database Persistence | บันทึก Job/File/Analysis แล้วปิดเปิดแอป | History และ Metadata ยังอ่านได้จาก DB เดิม |
| Application Reopen | เปิดแอปใหม่และเลือก Destination เดิม | ไม่ Crash; โหลด History และ Integrity ตามปลายทาง |
| Destination Isolation | สร้าง Record ในสองปลายทางแล้ว Query แยกกัน | History และ Description Metadata ไม่ปนกัน |
| Drive Reopen | ปิดแอป เปิดโฟลเดอร์เดิม แล้ว Scan ใหม่ | Full Path เดิมจับคู่ Description ที่บันทึกไว้ได้; รูปโหลดจาก Drive |
| New Image | เพิ่มภาพที่ไม่มี Metadata แล้ว Scan | แสดง `Unanalyzed`; ไม่เรียก AI จนผู้ใช้สั่ง |
| Path Changed (known limitation) | เปลี่ยนชื่อไฟล์/ย้ายโฟลเดอร์แล้ว Scan | ระบบไม่พบ Description เดิมด้วย Path; บันทึกเป็นข้อจำกัดรุ่นแรก |
| Missing Metadata Match | เก็บ Metadata ไว้แต่ย้าย/ถอดไฟล์ออกจาก Drive | แสดง `Missing`; Metadata ไม่ถูกลบทันทีและ Preview ใช้ไม่ได้ |
| Database Image Storage Guard | ตรวจ Schema และข้อมูลทดสอบหลัง Save | ไม่มี BLOB/Base64/Image Bytes/Thumbnail Binary ใน SQLite |
| Description Search | ให้หลายภาพมี Description ที่ตรงคำ เช่น `cat`; ค้นและแก้คำค้น | SQLite คืนหลาย File IDs/Paths; ภาพจริงโหลดจาก Drive; ไม่พบผลมีข้อความชัดเจน |

## ความปลอดภัยของ Fixture

- API Key ไม่ใช่ Fixture ให้ตั้ง `GEMINI_API_KEY` ใน Environment ตาม `docs/SETUP_GUIDE.md`
- อย่าทดสอบลบ/ย้ายกับไดรฟ์ที่มีข้อมูลผู้ใช้ ใช้ Removable Drive สำหรับทดสอบโดยเฉพาะหรือ Test Double
- Database Test ใช้ SQLite ชั่วคราวและลบทิ้งเมื่อเสร็จ
- บันทึกชื่อไฟล์ ขนาด และผล Scenario ที่คาดหวัง หลีกเลี่ยงพาธส่วนตัวในไฟล์ Expected Output ที่ Commit

## ประเด็นที่ต้องยืนยัน

- ภาพ Fixture ใดได้รับอนุญาตให้แจกจ่าย และต้องรองรับ Format ใดบ้าง?
- มี Removable Drive และเครื่อง macOS สำหรับตรวจรับขั้นสุดท้ายหรือไม่?
- AI Integration ควรกำหนดขนาดภาพสูงสุดเท่าใด?
- หากเพิ่ม Hash Identity ภายหลัง จะเลือก Algorithm และทดสอบ Rename/Changed อย่างไร?
- จะรองรับ Drive ที่ย้ายไปใช้บนเครื่องอื่นและ Metadata Portability ในรุ่นถัดไปหรือไม่?
