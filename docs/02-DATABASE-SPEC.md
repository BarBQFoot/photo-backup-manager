# Database Spec — เวอร์ชันส่งงาน (Description-only)

> **ข้อบังคับ:** SQLite/GORM ไม่เก็บรูปจริง, Image Binary, Base64, BLOB หรือ Thumbnail Binary รูปจริงอยู่บน Drive/Storage ปลายทาง ฐานข้อมูลเก็บเฉพาะ File Metadata, Full Path สำหรับจับคู่ในเวอร์ชันแรก, AI Description และ Backup History
>
> **ลดขอบเขต:** ไม่มี Tag/Tag Cloud, Hash Index, Changed/Renamed Detection หรือ Metadata Portability ในเวอร์ชันส่งงาน Hash Identity เป็นงานต่อยอด

## หลักการ

- Go เข้าถึง SQLite ผ่าน GORM; Frontend ห้าม Query DB โดยตรง
- `FileRecord.ID` เป็น ID ภายใน DB; เวอร์ชันแรกจับคู่ไฟล์ที่สแกนพบด้วย Full Path
- Path ใช้ได้เมื่อโฟลเดอร์/ไฟล์ยังอยู่ตำแหน่งเดิม หากย้ายหรือเปลี่ยนชื่อ จะจับคู่ Metadata เดิมไม่ได้จนกว่าจะเพิ่ม Hash Identity
- บันทึกเวลา UTC; เปิด SQLite Foreign Keys
- ไม่มีคอลัมน์ Binary/Blob หรือ Tag ใน Schema เวอร์ชันส่งงาน

## Tables และ Fields

### `file_records`

| Column | Type | Constraint / ความหมาย |
| --- | --- | --- |
| `id` | integer | PK; File ID ภายในระบบ |
| `file_name` | text | ห้ามว่าง; ชื่อไฟล์ปัจจุบัน |
| `path` | text | ห้ามว่าง; Full Path ใช้จับคู่รายการ Scan |
| `size_bytes` | integer | ห้ามว่าง, ไม่ติดลบ |
| `mime_type` | text | ชนิดไฟล์ เช่น `image/jpeg` |
| `modified_at` | datetime | เวลาแก้ไขล่าสุดเมื่อเข้าถึงได้ |
| `backup_job_id` | integer | FK ไป `backup_jobs.id`, ว่างได้ |
| `description` | text | AI Description, ว่างได้ก่อนวิเคราะห์ |
| `ai_status` | text | `unanalyzed`, `analyzing`, `analyzed`, `failed` |
| `status` | text | `active`, `missing`, `deleted` |

### `backup_jobs`

| Column | Type | Constraint / ความหมาย |
| --- | --- | --- |
| `id` | integer | PK |
| `source` | text | พาธต้นทาง |
| `destination` | text | พาธปลายทาง, ห้ามว่าง |
| `started_at` | datetime | เวลาเริ่ม |
| `completed_at` | datetime | เวลาจบ, ว่างได้ระหว่างทำงาน |
| `total_files` | integer | จำนวนไฟล์ที่ร้องขอ |
| `success_count` | integer | จำนวนสำเร็จ |
| `failed_count` | integer | จำนวนผิดพลาด |
| `skipped_count` | integer | จำนวนข้ามจาก Duplicate |
| `duration_ms` | integer | เวลารวมที่วัดได้จริง |
| `status` | text | `running`, `completed`, `partial`, `failed`, `interrupted` |

## Index และความสัมพันธ์

- Index `file_records(path)` เพื่อจับคู่รายการที่ Scan พบ
- Index `file_records(status)` สำหรับแสดง Missing/Active
- Index `backup_jobs(destination, started_at)` สำหรับ History แยกปลายทาง
- `BackupJob 1 ─── * FileRecord` ผ่าน `backup_job_id`
- ไม่สร้าง `tags`/`file_tags` ในเวอร์ชันนี้

## พฤติกรรม Metadata

- เมื่อ Scan พบ Path ที่มี Record ให้ใช้ Description เดิม
- เมื่อพบรูปใหม่ ให้แสดง `Unanalyzed`; บันทึก FileRecord เมื่อเริ่ม/จบ AI Analysis ตาม Implementation โดยไม่เรียก AI อัตโนมัติ
- เมื่อ Metadata มี Path แต่ไฟล์จริงไม่พบ ให้เปลี่ยนเป็น `missing` เก็บ Description ไว้ และไม่แสดงว่าเปิดรูปได้
- เมื่อผู้ใช้ Delete ผ่านแอป ให้ลบไฟล์จริงก่อน แล้วจึงปรับสถานะ Record ตาม Contract
- หาก DB เปิดไม่ได้ ห้ามสร้าง DB ว่างทับ DB เดิมโดยไม่แจ้ง

## Open Questions / งานต่อยอด

- **Hash Identity:** เพิ่มเมื่อจำเป็นต้องรักษาการจับคู่หลังย้าย/เปลี่ยนชื่อไฟล์; Algorithm และนโยบายยังไม่กำหนด
- **Database Location:** อยู่บนเครื่องหรือย้ายตาม Drive; เวอร์ชันส่งงานใช้ตำแหน่งเครื่องตาม Implementation
- **Metadata Portability:** การนำ Metadata ไปใช้กับ Drive บนเครื่องอื่นยังไม่รองรับในขอบเขตแรก
- **Thumbnail/Preview:** รูปต้องโหลดจาก Drive; Cache หากเพิ่มภายหลังต้องไม่เก็บใน DB
