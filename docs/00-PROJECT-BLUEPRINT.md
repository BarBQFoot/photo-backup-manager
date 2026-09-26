# Photo Backup Manager — พิมพ์เขียวโครงการ

> สถานะ: แผนตั้งต้น เอกสารนี้แยกส่วนที่มีอยู่ใน Repository ออกจากส่วนที่วางแผนไว้ ปัจจุบัน Repository ยังเป็น Wails/Nuxt Scaffold ระยะเริ่มต้น

## ภาพรวมโครงการ

Photo Backup Manager เป็นแอป Desktop สำหรับเลือกโฟลเดอร์จากสื่อบันทึกแบบถอดได้ ย้ายไฟล์ไปยัง Drive/Storage ปลายทาง และเก็บประวัติแยกตามปลายทาง โครงการ Final เพิ่มการวิเคราะห์ภาพและค้นหาด้วย **Description เท่านั้น** ในเวอร์ชันส่งงาน **ไฟล์รูปจริงอยู่บน Drive เท่านั้น SQLite/GORM เก็บเฉพาะ Metadata, File Path, AI Description และ Backup History** เกณฑ์กลางภาคกำหนด `/move` เป็นการย้าย: ลบต้นทางหลังไฟล์ปลายทางพร้อมใช้งานเท่านั้น กรณีข้ามไดรฟ์ให้คัดลอก ตรวจสอบความครบถ้วน แล้วจึงลบต้นทาง

### เป้าหมายและความสามารถหลัก

- เลือกพาธต้นทาง/ปลายทาง สแกนรายการไฟล์จริง และตรวจสถานะการเข้าถึง
- เลือกย้ายบางไฟล์หรือทั้งหมด พร้อมแสดง Progress รายไฟล์ ชื่อซ้ำ ข้อผิดพลาด และเวลารวม
- ไม่เขียนทับไฟล์ชื่อซ้ำในปลายทาง และไม่สร้าง Active Record ซ้ำภายใต้ปลายทางเดียวกัน
- บันทึกประวัติ Job และ Metadata ด้วย SQLite/GORM โดยแยกข้อมูลตามปลายทาง
- ตรวจไฟล์ปลายทางที่หายด้วยการเทียบ Active Record กับไฟล์จริง และลบไฟล์พร้อมปรับสถานะฐานข้อมูล
- วิเคราะห์ภาพที่ผู้ใช้เลือกด้วย Gemini 2.5 Flash และบันทึก Description
- เปิดแอปใหม่แล้วอ่านประวัติและ AI Metadata ที่บันทึกไว้ได้
- Scan โฟลเดอร์แล้วจับคู่ไฟล์กับ Metadata ด้วย Full Path ในเวอร์ชันส่งงาน; Hash Identity เป็นงานต่อยอด
- แสดง Gallery เป็น Drive Browser + Metadata Viewer โดยอ่านรูปจริงจาก Drive และ Metadata จาก SQLite
- แสดงสถานะ `Unanalyzed` และ `Missing`; เก็บ Description ของ Missing ไว้แต่ไม่แสดงว่าไฟล์เปิดได้
- Search ค้น Description ใน SQLite แล้วใช้ File ID/Path ไปโหลดรูปจริงจาก Drive; ต้องคืนภาพที่ตรงกันได้หลายรายการ

### ขอบเขต

**ขอบเขตลดรูปสำหรับกำหนดส่ง:** เลือกวิเคราะห์/ค้นจาก Description อย่างเดียว ใช้ Full Path จับคู่ Metadata ชั่วคราว ให้ผู้ใช้เริ่ม AI เอง ไม่มี Category, Tags/Tag Cloud, Hash Identity, Rename/Changed Detection, Analysis Cache/Re-analysis อัตโนมัติ, Thumbnail Cache หรือ Metadata Portability ในรอบนี้

**อยู่ในขอบเขต:** แอป Desktop สำหรับ Windows/macOS ตามเป้าหมายโครงการ, Workflow โฟลเดอร์/ไดรฟ์ถอดได้, ย้ายไฟล์และ Fallback ข้ามไดรฟ์, ตรวจชื่อซ้ำ, History แยกปลายทาง, Integrity Check, Delete, AI Metadata, ชุดข้อมูลทดสอบ และ UI สำหรับ Flow เหล่านี้

**ห้ามทำ:** เก็บภาพจริง, Image Binary, Base64, BLOB หรือ Thumbnail Binary ใน SQLite; ให้ Frontend อ่าน SQLite/จัดการระบบไฟล์; Gallery ที่โหลดรูปจาก DB. **ลด/เลื่อนไปหลังส่งงาน:** Tags/Tag Cloud, Hash Identity, Rename/Changed Detection, Thumbnail Cache, Metadata Portability และ Advanced Search Filters. **นอกขอบเขต:** Cloud Sync, Protocol นำเข้ามือถือเฉพาะทาง, สร้างภาพด้วย AI และลบต้นทางเมื่อการย้ายไม่สำเร็จ

## สถานะปัจจุบันใน Repository

| ส่วนงาน | สถานะ | หลักฐาน/หมายเหตุ |
| --- | --- | --- |
| Wails v2 Entry Point | มีอยู่ | `main.go`, Dependency Wails v2 |
| Go App Binding | มีอยู่ | `app.go` มีเฉพาะเมธอดตัวอย่าง `Greet` |
| Nuxt Frontend | Scaffold มีอยู่ | `frontend/`; ยังไม่พบ Product Pages |
| SQLite/GORM Models/Repository | วางแผนไว้ | ยังไม่พบ Implementation หรือ GORM ใน `go.mod` |
| Backup, Scan, Delete, Integrity | วางแผนไว้ | ยังไม่พบ Implementation |
| Gemini Integration | วางแผนไว้ | ยังไม่มี AI Code; Setup Guide ระบุ `GEMINI_API_KEY` |
| Product Screens และ Contracts | วางแผนไว้ | ระบุในชุดเอกสารนี้ |

## Technology Stack และ Architecture

เทคโนโลยีที่ยืนยันจาก Repository: Go, Wails v2 และ Nuxt ข้อกำหนดระบุ SQLite, GORM และ Gemini ซึ่งยังเป็นแผนจนกว่าจะ Implement จริง

```text
Nuxt Pages/Components
       │ Wails Bindings + Runtime Events
       ▼
Wails App Facade (ตรวจ Input และประสาน Service)
       ├── Backup Service ──┐
       ├── File Scanner ─────┤── Drive/Filesystem: รูปจริง
       ├── AI Service ───────┤   (อ่าน/ย้าย/แสดงไฟล์)
       ├── Search Service ───┤
       └── Metadata Service ─┘
                  │                 │
                  ▼                 ▼
        SQLite + GORM          Destination Drive
        Metadata/Path/         IMG_001.jpg ...
        Description/History    (ไม่มี Image Binary ใน DB)
```

Nuxt รับผิดชอบ Presentation และ View State; Go รับผิดชอบ Drive/File System, Path Matching, การตรวจสอบ, การเรียก AI และประสานงาน; Repository รับผิดชอบ Metadata Persistence เท่านั้น DTO อยู่ใน `01-API-CONTRACT.md` และ Schema อยู่ใน `02-DATABASE-SPEC.md` Gallery คือ Drive Browser + Metadata Viewer: Scan รูปจริง, จับคู่ Full Path กับ SQLite, แสดงภาพจริงและ Description แยกจากกัน

เอกสาร UI Design ที่ใช้ร่วมกัน: `05-UI-DESIGN.md` กำหนดหน้า Move Photos, Gallery (รวม Description Search) และ History & Check; ไม่ใช่ Implementation

แนวทาง UX อ้างอิง Workflow/UI จาก `FinalPhotoMover` ที่ผู้ใช้แนบ โดยดูรายการฟีเจอร์ฐานและส่วนที่จะเพิ่มใน `REFERENCE_APP_AND_ADDITIONS.md`; โปรแกรมอ้างอิงเป็นคนละ ZIP/คนละ Source กับ Repository นี้

## โครงสร้างโฟลเดอร์ที่เสนอ

```text
app.go                         # Wails Facade (ไฟล์เดิม)
main.go                        # เริ่ม Wails และ Bindings (ไฟล์เดิม)
internal/backup/               # สแกน ย้าย ตรวจชื่อซ้ำ Integrity และ Delete
internal/metadata/             # Path-based Metadata Matching และสถานะไฟล์
internal/ai/                   # Gemini Client และตรวจ Response
internal/search/               # ค้น Metadata (วางแผนไว้)
internal/database/             # SQLite, GORM Models, Repositories
internal/domain/                # Domain Types/Errors ที่ใช้ร่วมกัน
frontend/app/pages/             # Nuxt Pages
frontend/app/components/        # UI Components
frontend/wailsjs/               # Generated Bindings (ห้ามแก้มือ)
docs/                           # Specs และคู่มือสมาชิก
```

เป็นข้อเสนอ ไม่ใช่คำอธิบายโครงสร้างที่มีอยู่แล้ว ให้ปรับเข้ากับ Repository เมื่อเริ่มพัฒนาโดยไม่ย้ายไฟล์เดิมเกินจำเป็น

## หน้าที่ของแต่ละ Module

| Module | รับผิดชอบ | ไม่รับผิดชอบ |
| --- | --- | --- |
| Wails Facade | DTO Boundary, Validation, ประสาน Service และส่ง Event | UI Rendering, รายละเอียด SQL |
| Backup | Scan/Move/Copy Fallback/Duplicate/Progress/Integrity/Delete | UI, การตัดสิน Schema |
| File Scanner/Metadata | อ่านไฟล์จริง, จับคู่ด้วย Path, ตรวจ Missing | จัดเก็บ Image Binary ใน DB |
| Database | Metadata/Description/History, Migration, Query แยก Drive | ระบบไฟล์, รูป Binary, UI |
| AI | อ่านไฟล์จริงผ่าน Go, Gemini Request, เก็บเฉพาะผลวิเคราะห์ | เก็บ Image Payload ใน SQLite |
| Search | ค้น Description ใน SQLite แล้วคืน File IDs/Paths | โหลด Image Blob จาก DB หรือเข้ DB จาก Frontend |
| Frontend | Pages, Components, View State, Error/Progress | ระบบไฟล์หรือ SQLite Logic |
| Testing/Integration | Fixtures, Acceptance Scenarios, ประสาน Integration | เปลี่ยน Contract ของ Module อื่นฝ่ายเดียว |

## การแบ่งงานทีม 4 คนและการทำงานคู่ขนาน

| สมาชิก | ความรับผิดชอบหลัก | Contract/Dependency |
| --- | --- | --- |
| 1 — Backend / Backup Engine | Drive/File Scanner, Path Matching, Missing, Move/Delete | `01-API-CONTRACT.md`; Database Repository |
| 2 — Database / GORM | Metadata-only SQLite, FileRecord/Path Index, AI Description, History | `02-DATABASE-SPEC.md`; DTO ใน API Contract |
| 3 — Frontend / Nuxt | Move Photos, Gallery ที่ค้น Description ใน Destination เดียวกัน, History & Check | Wails Bindings/API Contract; ห้ามแตะ DB/File System |
| 4 — AI / Testing / Integration | Gemini Description, AI Error, New/Existing/Missing cases | API Contract และ Database Repository |

สมาชิกเริ่มงานตาม Contract และ Mock Data ได้โดยไม่ต้องรอสมาชิกอื่น งาน Integration ให้ผ่านการ Review และมีผู้ประสานงานการแก้ `app.go`/Generated Bindings ร่วมกัน

## กติกาการทำงานร่วมกัน

- ใช้ `01-API-CONTRACT.md` เป็นแหล่งอ้างอิงกลางของ Interface; Database/UI/Flow ต้องใช้ชื่อและรูปแบบข้อมูลให้ตรงกัน
- ระบุสถานะ **Existing**, **Planned** หรือ **Needs Refactor** อย่าเขียนให้เข้าใจว่างานที่วางแผนไว้ทำเสร็จแล้ว
- Go Exported Names ใช้ PascalCase, JSON Field ใช้ lower camelCase, SQL ใช้ snake_case ตาม GORM เว้นแต่ Migration ต้องกำหนดต่างออกไป
- ใช้ Typed/Domain Errors ภายใน และแปลงที่ Wails Boundary เป็น `{code,message,details?}` ห้ามเขียนทับโดยไม่แจ้ง หรือแจ้งย้ายสำเร็จทั้งที่ยังไม่ครบ
- แยก Branch ตามงาน (`feat/...`, `fix/...`, `docs/...`) และใช้ Conventional Commit เช่น `feat:`, `fix:`, `docs:`, `test:` การเปลี่ยน Contract ต้องให้ผู้ได้รับผลกระทบทบทวน
- ทดสอบ Move/Delete ด้วย Fixture ใน `testdata/` เท่านั้น ห้ามใช้รูปส่วนตัว เก็บ Secret นอก Source/Docs/Log/Git; Gemini Key อ่านจาก `GEMINI_API_KEY`
- แสดงเวลาที่วัดได้จริง ห้ามรับรองอัตราความเร็วหากไม่มีเครื่องและชุดข้อมูลอ้างอิง

## Definition of Done

Feature ถือว่าเสร็จเมื่อ Contract กับ Implementation ตรงกัน, จัดการ Error/Empty/Loading/Progress, ขอบเขตข้อมูลแยกตามปลายทาง, การทำงานแบบทำลายข้อมูลปลอดภัย, ทดสอบด้วย Fixture ที่เกี่ยวข้อง, Checklist ของเจ้าของงานครบ และผ่าน Integration Review จะอ้างว่ารองรับ Platform ใดได้เฉพาะ Platform ที่ Build และตรวจแล้ว

## Dependency Matrix

| Module | Depends On | Used By |
| --- | --- | --- |
| Backend | API Contract, Database Repository Contract | Wails Facade / Frontend |
| Database | API/Domain Data Contract, Database Spec | Backend, AI, Search |
| Frontend | API Contract, Wails Bindings | ผู้ใช้ |
| AI | API Contract, Database Repository | Frontend ผ่าน Facade |
| Search | API Contract, Database Description Metadata | Frontend |
| Testing | ทุก Contract และ Module; Fixture Guide | ทั้งทีม |

## ประเด็นที่ต้องยืนยัน (Open Questions)

1. Final ต้องลบต้นทางเหมือน `/move` กลางภาคหรืออนุญาต Copy Mode เพิ่ม? ค่าเริ่มต้นในเอกสารนี้คือ Move; การ Copy เป็นกลไก Fallback ข้ามไดรฟ์เท่านั้น
2. Final ย้ายไฟล์ทุกชนิดหรือเฉพาะรูป? ค่าเริ่มต้นคือแกนย้ายรองรับไฟล์ทั่วไป ส่วน Gallery/AI ใช้กับรูป
3. Delete ทำได้เฉพาะไฟล์ปลายทางหรือต้องเลือกลบต้นทางด้วย? ค่าเริ่มต้น UI คือ Delete จากปลายทาง ให้ยืนยันกับอาจารย์
4. อนุญาตเรียก Gemini API ภายนอกสำหรับ Demo หรือไม่ รวมถึง Network/ค่าใช้จ่าย? ข้อกำหนดระบุ Gemini 2.5 Flash แต่การใช้งานจริงยังต้องยืนยัน
5. มีโทรศัพท์รุ่น/วิธี Import และเครื่อง macOS สำหรับทดสอบใดบ้าง? ระหว่างรอยืนยันให้ใช้ Flash Drive เป็นฐานทดสอบ
6. เวอร์ชันส่งงานใช้ Full Path จับคู่ Metadata; หากโฟลเดอร์ถูกย้าย/ไฟล์เปลี่ยนชื่อ การจับคู่อาจหาย Hash Identity เป็นงานต่อยอด
7. กฎชื่อซ้ำที่ต่างกันเฉพาะตัวพิมพ์เล็ก/ใหญ่และ Record ที่เคยลบแล้วเป็นอย่างไร? กฎข้าม Platform ยังต้องสรุป
