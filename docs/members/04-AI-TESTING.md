# สมาชิก 4 — คู่มือ AI / Testing / Integration

> ขอบเขตงาน: วิเคราะห์ภาพด้วย Gemini, ทดสอบการยอมรับร่วมกัน และประสาน Integration ปัจจุบัน Repository ยังไม่มี AI Implementation

## ความรับผิดชอบ

ให้ Go อ่านไฟล์จริงจาก Drive/Filesystem แล้วส่ง Gemini `gemini-2.5-flash`; บันทึกเฉพาะ Description และ AI Status ใน SQLite ห้ามเก็บ Image Bytes/Base64/BLOB เวอร์ชันส่งงานไม่ทำ Category, Tags, Tag Cloud, Cache หรือ Re-analysis อัตโนมัติ

## Gemini Request / Response

- Provider: Google Gemini API; Model: `gemini-2.5-flash`
- Authentication: อ่าน `GEMINI_API_KEY` จาก Environment ของ Process ห้าม Hard-code, Commit, แสดงใน Log หรือส่งไป Frontend
- Request: Go อ่านภาพที่ผู้ใช้เลือกผ่าน Path จาก Destination แล้วส่งให้ Gemini; ห้ามให้ Frontend ส่งภาพหรือเก็บ Image Blob ใน DB นโยบายชนิด/ขนาดภาพยังต้องกำหนด
- Response: ตรวจสอบและแปลงเป็น `AIResult {fileId,description,status}` หากว่างหรือผิดรูปแบบให้ใช้ `AI_RESPONSE_INVALID`
- Analysis เกิดเมื่อผู้ใช้กด Analyze เท่านั้น; ไม่ทำ Cache/Fingerprint หรือ Auto Re-analysis ในเวอร์ชันส่งงาน

## Error Handling

ใช้ Error Code กลาง: `AI_KEY_MISSING`, `AI_UNAVAILABLE`, `UNSUPPORTED_IMAGE`, `FILE_NOT_FOUND`, `FILE_MISSING`, `AI_RESPONSE_INVALID`, `AI_SAVE_FAILED` เมื่อ API ผิดพลาด ห้ามเปลี่ยนหรือลบภาพต้นฉบับ ห้ามบันทึก Key, พาธละเอียด หรือเนื้อหาภาพลง Log โดยไม่จำเป็น

## Mock AI Result

```json
{"fileId":1,"description":"A cat sitting indoors","status":"success"}
```

## Test Images / Test Cases

ใช้ Fixture ใน `docs/test-data/README.md` ทดสอบ New, Existing, Missing, AI Analysis/Error, Description Search หลายผลภายใน Gallery ของ Destination ที่เลือก และ DB Failure ยืนยันว่า DB เก็บเฉพาะ Description ไม่เก็บภาพ ห้ามใช้ภาพส่วนตัวทดสอบ Move/Delete หรือส่ง API โดยไม่มีการอนุญาตแยก

เกณฑ์ Search: ค้นข้อความไทยใน Description ได้, คืนหลายภาพ, ไม่คืน Missing/Deleted เป็นรูปที่เปิดได้

## จุดเชื่อมต่อ

- สมาชิก 2: AI Description/Status, Transaction และ Repository API
- สมาชิก 3: สถานะกำลังวิเคราะห์/สำเร็จ/ล้มเหลว
- สมาชิก 1: Path/Record ที่ใช้ได้จริงและสถานะ Integrity
- ทั้งทีม: Shared Contract และรายการตรวจ Demo ปลายทาง

## ห้ามแก้ไข/ทำ

ห้ามใส่ Key จริงใน Code/Docs/Git, เก็บภาพ/Base64/BLOB ใน DB, เพิ่ม Category/Tags/Tag Cloud/Cache/Re-analyze ในรอบนี้, ทำ Backup Move/Delete หรือ UI, อ้างว่า Mock AI คือวิเคราะห์จริง, สร้าง Search ปลอม, ใช้ไฟล์ส่วนตัวทดสอบลบ หรือเพิ่ม Dependency โดยไม่ทบทวน

## Definition of Done

เชื่อม Gemini ตาม Contract, ใช้ Credential จาก Environment, ตรวจและบันทึก Description, API Error ไม่ทำให้ภาพเสียหาย, ใช้ Fixture ในการทดสอบ และรายงานสถานะ Windows/macOS ตามที่ตรวจจริง Demo ต้องแยกของจริงกับส่วนที่ยัง Mock ให้ชัด

## Handoff Checklist

- [ ] แจ้ง Gemini Adapter/Configuration และ Repository Calls
- [ ] ยืนยันว่าไม่มี Key หรือ Payload อ่อนไหวใน Repository/Log
- [ ] ส่ง Scenario และผลทดสอบทั้ง Mock/เรียก API จริง
- [ ] ระบุข้อสรุปเรื่องชนิด/ขนาดภาพ; Fingerprint ไม่อยู่ในรอบส่งงาน
- [ ] แจ้งเครื่องทดสอบ Platform และประเด็น Search/API ที่ยังค้าง
