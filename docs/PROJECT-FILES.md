# ตารางหน้าที่ไฟล์ใน Photo Backup Manager

คู่มือนี้อ้างอิงไฟล์และเส้นทางการเรียกใช้งานในโปรเจกต์ปัจจุบัน สำหรับค้นโค้ดและอธิบายตอนพรีเซนต์

## ภาพรวมโฟลเดอร์

| ตำแหน่ง | รับผิดชอบอะไร |
| --- | --- |
| ไฟล์ Go ที่โฟลเดอร์หลัก | เปิดแอป เชื่อมส่วนต่าง ๆ และตั้งค่าตอนเริ่ม/ปิดโปรแกรม |
| `frontend/app/` | หน้าจอ ปุ่ม รายการรูป สถานะงาน และการค้นหา |
| `service/` | ควบคุมขั้นตอนงาน ประสานไฟล์จริง ฐานข้อมูล และ AI |
| `internal/backup/` | สแกน ย้าย คัดลอก และลบไฟล์จริง |
| `repository/` | อ่านและเขียนข้อมูลใน SQLite |
| `database/` | กำหนดและสร้างตารางฐานข้อมูล |
| `model/` | โครงสร้างข้อมูลและตัวช่วย query ที่ GORM Gen สร้าง |
| `tools/` | สคริปต์ Python สำหรับ AI การติดตั้ง และคำสั่งดูแลระบบ |
| `tests/` | ชุดทดสอบ Go, Python และหน้าจอ |
| `docs/` | คู่มือ ข้อกำหนด และพรอมป์ต์แปลที่แอปใช้ |

## จุดเริ่มต้นและการตั้งค่าแอป

| ไฟล์ | ใช้ทำอะไร | ใช้ตอนไหน |
| --- | --- | --- |
| [`main.go`](../main.go) | โหลด `.env` เปิดฐานข้อมูล สร้างหน้าต่าง Wails ฝังหน้าจอ และเปิดให้ JavaScript เรียก Go | เมื่อเริ่มโปรแกรม |
| [`app.go`](../app.go) | สร้าง `DriveService` กับ repositories เปิดหน้าต่างเลือกโฟลเดอร์ เริ่มตัวติดตั้ง AI และปิด worker เมื่อออกจากแอป | ตลอดอายุแอป |
| [`app_env.go`](../app_env.go) | หาตำแหน่ง `.env` ข้าง EXE หรือข้าง `.app` บน Mac | เมื่อเริ่มโปรแกรม |
| [`app_storage_dev.go`](../app_storage_dev.go) | ให้โหมดพัฒนาใช้ตำแหน่งฐานข้อมูลเดิมของโปรเจกต์ | ตอนพัฒนา ไม่มี build tag `production` |
| [`app_storage_production.go`](../app_storage_production.go) | กำหนดฐานข้อมูลไว้ในโฟลเดอร์ของบัญชีผู้ใช้ที่เขียนได้ | ตอนบิลด์ด้วย tag `production` |
| `.env` | เก็บ `GEMINI_API_KEY` และค่าตั้งเฉพาะเครื่อง | โหลดตอนเปิดแอป ไม่แนบคีย์จริงใน Git/Release |

## Repository, Database และ Model

| ไฟล์ | ใช้ทำอะไร | ตัวอย่างข้อมูลที่เกี่ยวข้อง |
| --- | --- | --- |
| [`repository/dbconnect.go`](../repository/dbconnect.go) | เปิด SQLite ตั้งค่า connection และ foreign keys เรียก migration และกู้สถานะงานจัดเก็บที่ค้าง | ตำแหน่ง `photo_backup.db` |
| [`repository/file_record.go`](../repository/file_record.go) | บันทึก/หาไฟล์ด้วย ID หรือพาธ เปลี่ยนพาธหลังย้าย บันทึกคำบรรยายและสถานะ AI | ชื่อไฟล์ พาธ ขนาด คำบรรยาย `analyzed`/`failed` |
| [`repository/backup_job.go`](../repository/backup_job.go) | สร้างงานจัดเก็บ บันทึกผลเมื่อจบ และอ่านประวัติตามปลายทาง | เวลาเริ่ม จำนวนสำเร็จ ข้าม ล้มเหลว |
| [`database/migrate.go`](../database/migrate.go) | อ่าน `schema.sql` ที่ฝังไว้แล้วรัน SQL ใน transaction | สร้างตาราง/ดัชนีที่ยังไม่มี |
| [`database/schema.sql`](../database/schema.sql) | นิยามตาราง ข้อจำกัด และดัชนี | `file_records`, `backup_jobs`, `schema_migrations` |
| [`model/model/file_records.gen.go`](../model/model/file_records.gen.go) | Go struct ของตารางไฟล์ | `FileRecord` |
| [`model/model/backup_jobs.gen.go`](../model/model/backup_jobs.gen.go) | Go struct ของตารางงานจัดเก็บ | `BackupJob` |
| [`model/query/gen.go`](../model/query/gen.go) | ตัวกลาง query และ transaction ที่สร้างโดย GORM Gen | ถูกใช้จาก repository |
| [`model/query/file_records.gen.go`](../model/query/file_records.gen.go) | ตัวช่วย query ตารางไฟล์แบบมีชนิดข้อมูล | หา/อัปเดต `FileRecord` |
| [`model/query/backup_jobs.gen.go`](../model/query/backup_jobs.gen.go) | ตัวช่วย query ตารางประวัติ | สร้าง/อ่าน/อัปเดต `BackupJob` |
| [`repository/main/model_gen.go`](../repository/main/model_gen.go) | คำสั่งสร้างไฟล์ model/query จากฐานข้อมูล | ใช้ตอนพัฒนาเมื่อสั่ง generate ไม่ได้ทำงานทุกครั้งที่เปิดแอป |

SQLite เก็บข้อมูลประกอบรูปและคำบรรยาย ส่วนไฟล์ภาพจริงอยู่ในโฟลเดอร์บนดิสก์ ไม่ได้บันทึกรูปเป็น BLOB ในฐานข้อมูล

## Service: ควบคุมงานของแอป

| ไฟล์                                                                | ใช้ทำอะไร                                                                                                               | จุดที่ควรค้นในไฟล์                                                                      |
| ------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| [`service/drive_service.go`](../service/drive_service.go)           | ตัวควบคุมหลัก: สแกนและจับคู่ข้อมูล DB ย้าย/สำรองรูป อ่าน metadata ดูประวัติ ลบไฟล์ และกันการย้าย/ลบรูปที่กำลังวิเคราะห์ | `ScanDrive`, `StartBackup`, `GetFileMetadata`, `GetBackupHistory`, `DeleteFile`         |
| [`service/batch_analysis.go`](../service/batch_analysis.go)         | รับรูปหลาย ID ตรวจความพร้อม ตั้งสถานะวิเคราะห์ รับผลแล้วบันทึกทีละรูป พร้อมส่งความคืบหน้า                               | `AnalyzeImages`, `emitAIProgress`                                                       |
| [`service/hybrid_analyzer.go`](../service/hybrid_analyzer.go)       | เชื่อม Go กับขั้นตอน Florence → Gemini หา Python เตรียมสคริปต์ที่ฝังไว้ และสร้างคำขอให้ worker                          | `NewHybridAnalyzer`, `AnalyzeBatch`, `aiPython`, `aiWorkerResources`                    |
| [`service/resident_ai_worker.go`](../service/resident_ai_worker.go) | ดูแล Python process ที่ใช้ซ้ำ รับ/ส่ง JSON events และปิด process เมื่อเลิกใช้ ช่วยไม่ต้องโหลดโมเดลใหม่ทุกงาน            | `runResident`, `Close`                                                                  |
| [`service/ai_setup.go`](../service/ai_setup.go)                     | ควบคุมการเตรียม AI ครั้งแรก ตรวจของเดิม เตรียม Python/PyTorch/ไลบรารี/โมเดล และส่งสถานะให้ UI                           | `AISetup`, `Start`, `setupLocalAIWith`                                                  |
| [`service/ai_setup_download.go`](../service/ai_setup_download.go)   | ดาวน์โหลดและแตกตัวจัดการติดตั้ง `uv` ตรวจ SHA256 และรายงานจำนวนดาวน์โหลด                                                | `prepareUV`, `downloadVerified`, `extractUV`                                            |
| [`service/ai_process_windows.go`](../service/ai_process_windows.go) | ตั้งค่า subprocess บน Windows ให้ไม่มีหน้าต่างรบกวน และปิด process ลูกเมื่อจำเป็น                                       | `configureAICommand`, `terminateAICommand`                                              |
| [`service/ai_process_other.go`](../service/ai_process_other.go)     | จัดการ subprocess สำหรับระบบที่ไม่ใช่ Windows เช่น macOS                                                                | ฟังก์ชันคู่เดียวกับไฟล์ Windows เลือกด้วย build constraint                              |
| [`service/photo_preview.go`](../service/photo_preview.go)           | เปิดให้หน้าจอโหลดรูปจริงผ่าน `/photo-preview` โดยตรวจว่าเป็นไฟล์ที่เคยสแกนและยังเป็นไฟล์เดิม                            | `registerPreviews`, `PhotoPreviewMiddleware`                                            |
| [`service/ai_analyzer.go`](../service/ai_analyzer.go)               | ประกาศ `AIAnalyzer` และ `AIError` ที่ส่วนหลักยังใช้ร่วมกัน ภายในไฟล์ยังมี `GeminiAnalyzer` สำหรับส่งรูปแบบเดิม          | ปัจจุบัน `NewDriveService` เลือก `NewHybridAnalyzer` จึงไม่ได้เลือกเส้นทางส่งรูปแบบเดิม |

เส้นทาง AI ปัจจุบันอ่านภาพบนเครื่องด้วย Florence และส่งเฉพาะข้อความไป Gemini เพื่อแปลไทย ไม่ได้ใช้ YOLO, SigLIP2 หรือ Places365 ในเส้นทางนี้

## Internal/Backup: ทำงานกับไฟล์จริง

| ไฟล์ | ใช้ทำอะไร | พฤติกรรมหลัก |
| --- | --- | --- |
| [`internal/backup/scanner.go`](../internal/backup/scanner.go) | เดินอ่านโฟลเดอร์และโฟลเดอร์ย่อย คืนรายการไฟล์ภาพ | อ่านชื่อ พาธ ขนาด ชนิด และเวลาแก้ไข ส่วนคำบรรยายเติมภายหลังจาก DB |
| [`internal/backup/path_validator.go`](../internal/backup/path_validator.go) | ตรวจต้นทาง ปลายทาง และรายการไฟล์ที่เลือก | ตรวจพาธและขอบเขตไฟล์ก่อนเริ่มงาน |
| [`internal/backup/file_mover.go`](../internal/backup/file_mover.go) | ย้ายไฟล์จริง หาก rename ไม่สำเร็จจะคัดลอก ตรวจขนาด แล้วลบต้นทาง | ข้ามชื่อซ้ำ ไม่เขียนทับ |
| [`internal/backup/file_copier.go`](../internal/backup/file_copier.go) | สำรองด้วยการคัดลอกไฟล์ไปยังปลายทาง | เก็บต้นฉบับไว้ ข้ามชื่อซ้ำ ใช้ขั้นตอนคัดลอก/ตรวจขนาดร่วมกับ mover |
| [`internal/backup/file_deleter.go`](../internal/backup/file_deleter.go) | ตรวจขอบเขตแล้วลบไฟล์จริงในโฟลเดอร์ปลายทางที่ระบุ | Service จึงค่อยอัปเดตสถานะใน DB |

## Tools: สคริปต์ Python

| ไฟล์ | ใช้ทำอะไร | ถูกเรียกจาก |
| --- | --- | --- |
| [`tools/app_caption_worker.py`](../tools/app_caption_worker.py) | โหลด Florence สร้างคำบรรยายอังกฤษ อ่านภาพเป็นชุด ใช้แคช และส่งแปลซ้อนกับการอ่านภาพ ส่งผลกลับเป็น JSON events | Python worker ที่ Go เปิด |
| [`tools/translate_captions.py`](../tools/translate_captions.py) | เรียก Gemini เพื่อแปลข้อความ จัดการการพัก/retry บันทึกชุดที่สำเร็จ และใช้ผลเดิมเมื่อทำต่อ | Worker และ CLI สำหรับแปล |
| [`tools/prepare_translation.py`](../tools/prepare_translation.py) | แบ่งคำบรรยายอังกฤษเป็นชุดที่มี `file_id` และเตรียมพรอมป์ต์ | Worker ก่อนส่งแปล |
| [`tools/validate_captions.py`](../tools/validate_captions.py) | ตรวจ JSON คำตอบ ตรวจ ID ครบ ไม่ซ้ำ ไม่เกิน และรูปแบบคำบรรยาย | ตัวแปล ก่อนรับผลและบันทึก ไม่ใช่ตัววัดความถูกต้องของภาพ |
| [`tools/bootstrap_ai.py`](../tools/bootstrap_ai.py) | ตรวจ dependencies ตรวจไฟล์โมเดล ดาวน์โหลด Florence และทดสอบความพร้อม | หน้าจอติดตั้ง AI ผ่าน Go service |
| [`tools/resources.go`](../tools/resources.go) | ฝัง Python 5 ไฟล์ข้างต้นไว้ใน Go executable | ตอนบิลด์และแตกสคริปต์ไปใช้ใน user cache |
| [`tools/setup_local_ai.py`](../tools/setup_local_ai.py) | ดาวน์โหลดและลอง Florence สำหรับการตั้งค่าด้วยสคริปต์บน Mac | `Setup Mac AI.command` เป็นทางเลือกสำหรับเตรียมเครื่องพัฒนา |
| [`tools/reset_app_ai.py`](../tools/reset_app_ai.py) | สำรองแล้วล้างคำบรรยาย/สถานะ AI ใน DB และแคชคำบรรยาย โดยเก็บรูปกับโมเดล | ผู้พัฒนาสั่งเองเมื่อต้องการเริ่มทดสอบใหม่ ไม่ได้รันอัตโนมัติเมื่อเปิดแอป |
| [`docs/GEMINI-TRANSLATE-CAPTIONS.txt`](GEMINI-TRANSLATE-CAPTIONS.txt) | กฎการแปลจากอังกฤษเป็นไทยและรูปแบบคำตอบ | ตัวแปลใช้เป็นพรอมป์ต์ |
| [`docs/resources.go`](resources.go) | ฝังพรอมป์ต์แปลไว้ใน executable | Go นำไปเตรียมคู่กับสคริปต์ Python |

## Frontend: หน้าจอและสถานะงาน

| ไฟล์ | ใช้ทำอะไร |
| --- | --- |
| [`frontend/app/app.vue`](../frontend/app/app.vue) | จุดเริ่มหน้าจอ สร้างสถานะร่วม แสดงตัวติดตั้ง AI ก่อนเข้าแอป และรักษาหน้าเมื่อสลับเมนู |
| [`frontend/app/layouts/default.vue`](../frontend/app/layouts/default.vue) | โครงหน้าจอ เมนูหลัก โฟลเดอร์ปลายทาง และแถบสถานะ/ผล AI ที่เห็นได้ทุกหน้า |
| [`frontend/app/pages/index.vue`](../frontend/app/pages/index.vue) | หน้าเลือกโฟลเดอร์ สแกน เลือกรูป ย้ายหรือสำรอง และดูผลจัดเก็บ |
| [`frontend/app/pages/gallery.vue`](../frontend/app/pages/gallery.vue) | แกลเลอรี ช่องค้นหา เลือกวิเคราะห์ AI และรายละเอียดคำบรรยาย |
| [`frontend/app/pages/history.vue`](../frontend/app/pages/history.vue) | ประวัติงาน ตรวจไฟล์ที่หาย รีเฟรชสถานะ และลบไฟล์ |
| [`frontend/app/composables/usePhotoManager.ts`](../frontend/app/composables/usePhotoManager.ts) | เก็บสถานะและคำสั่งของหน้าจอ แยกงาน AI เบื้องหลังจากงานจัดเก็บ รับผล AI ระหว่างทำงาน และกันข้อมูลตอบกลับเก่าทับหน้าปัจจุบัน |
| [`frontend/app/composables/useSharedPhotoManager.ts`](../frontend/app/composables/useSharedPhotoManager.ts) | ให้แต่ละหน้าใช้ PhotoManager ตัวเดียวกัน |
| [`frontend/app/services/desktop.ts`](../frontend/app/services/desktop.ts) | ตัวกลางเรียก Go ผ่าน Wails และสมัครรับ events ความคืบหน้า |
| [`frontend/app/utils/captionSearch.ts`](../frontend/app/utils/captionSearch.ts) | ค้นจากคำบรรยาย รองรับคำตรง คำพ้อง และการพิมพ์ผิดเล็กน้อย ใช้ Fuse.js ร่วมกับการตรวจระยะห่างคำและขอบเขตคำ |
| [`frontend/app/data/searchSynonyms.json`](../frontend/app/data/searchSynonyms.json) | คลังกลุ่มคำพ้อง เช่น หมา/สุนัข ที่ตัวค้นหาอ่านมาใช้ |
| [`frontend/app/components/AISetupScreen.vue`](../frontend/app/components/AISetupScreen.vue) | หน้าจอเตรียม AI แสดงขั้นตอน ดาวน์โหลด ข้ามของที่มีแล้ว และลองใหม่เมื่อผิดพลาด |
| [`frontend/app/components/PhotoPreview.vue`](../frontend/app/components/PhotoPreview.vue) | แสดงภาพผ่าน `/photo-preview` และข้อความเมื่อภาพหายหรือเปิดไม่ได้ |
| [`frontend/app/components/ConfirmDialog.vue`](../frontend/app/components/ConfirmDialog.vue) | หน้าต่างยืนยันก่อนย้าย สำรอง หรือลบ |
| [`frontend/app/components/AppIcon.vue`](../frontend/app/components/AppIcon.vue) | ไอคอนที่ใช้ร่วมกันในแอป |
| [`frontend/app/assets/css/main.css`](../frontend/app/assets/css/main.css) | สี ฟอนต์ ระยะห่าง เลย์เอาต์ และรูปแบบหน้าจอตามขนาดหน้าต่าง |
| [`frontend/wailsjs/go/`](../frontend/wailsjs/go/) | JavaScript bindings และชนิดข้อมูลที่ Wails สร้างจาก Go เพื่อให้หน้าจอเรียก backend |
| [`frontend/wailsjs/runtime/`](../frontend/wailsjs/runtime/) | ตัวช่วย runtime ของ Wails เช่นรับ events เป็นไฟล์ที่ Wails สร้าง |

การค้นหาที่หน้าจอปัจจุบันใช้ `captionSearch.ts` กับรายการคำบรรยายที่โหลดมา ส่วน `SearchByDescription` ใน repository เป็นเมธอด SQL ที่มีอยู่ ไม่ใช่ขั้นตอนค้นหาหลักของช่องค้นหาในแกลเลอรี

## Build, การแจกแอป และชุดทดสอบ

| ไฟล์/ตำแหน่ง | ใช้ทำอะไร |
| --- | --- |
| [`wails.json`](../wails.json) | ตั้งชื่อแอปและคำสั่งติดตั้ง/รัน/บิลด์ frontend ให้ Wails |
| [`go.mod`](../go.mod), [`go.sum`](../go.sum) | กำหนดเวอร์ชัน Go และ dependencies พร้อม checksum |
| [`frontend/package.json`](../frontend/package.json), [`frontend/package-lock.json`](../frontend/package-lock.json) | dependencies และคำสั่ง frontend เช่น build, dev, test |
| [`frontend/nuxt.config.ts`](../frontend/nuxt.config.ts) | ตั้งค่า Nuxt แบบ client side, router, CSS และโฟลเดอร์ `dist` ที่ Go ฝังไว้ |
| [`frontend/tsconfig.json`](../frontend/tsconfig.json) | ตั้งค่า TypeScript ของ frontend |
| [`build/appicon.png`](../build/appicon.png), [`build/windows/`](../build/windows/), [`build/darwin/`](../build/darwin/) | ไอคอน ข้อมูลแอป และแม่แบบแพ็กเกจสำหรับระบบปฏิบัติการ |
| [`Build Mac App.command`](<../Build Mac App.command>) | บิลด์ `.app` และ ZIP สำหรับแจกบน Mac |
| [`Setup Mac AI.command`](<../Setup Mac AI.command>) | เตรียม Python/ไลบรารี/โมเดลของเครื่องพัฒนา Mac แบบดับเบิลคลิก |
| [`Start Mac App.command`](<../Start Mac App.command>) | เปิดโหมดพัฒนาบน Mac ด้วย `wails dev` |
| [`tests/go/`](../tests/go/) | ชุดทดสอบ Go แยกตามแพ็กเกจเดิม |
| [`tests/go-overlay.json`](../tests/go-overlay.json) | จับคู่ตำแหน่ง Go tests ที่ย้ายแล้ว ให้ยังทดสอบฟังก์ชันภายในแพ็กเกจเดิมได้ |
| [`tests/go/go.mod`](../tests/go/go.mod) | กันไม่ให้การบิลด์แอปปกติมองโฟลเดอร์เก็บ Go tests เป็นแพ็กเกจของแอป |
| [`tests/python/`](../tests/python/) | ทดสอบ worker ตัวแปล และ bootstrap |
| [`tests/frontend/`](../tests/frontend/) | ทดสอบการค้นหา การเลือกรูป งานเบื้องหลัง สำรองรูป และหน้าติดตั้ง |
| [`tests/README.md`](../tests/README.md) | คำสั่งรันเทสต์แต่ละภาษา |
| `.gitignore` | กันไฟล์คีย์ ฐานข้อมูล แคช รูปทดลอง และบิลด์ไม่ให้ติด Git |
| `build/bin/` | ไฟล์แอปที่บิลด์เสร็จแล้ว ไม่ใช่ซอร์สโค้ด |
| `test-images/` | รูปที่ใช้ทดสอบ ไม่จำเป็นสำหรับผู้ใช้ที่เปิดแอปสำเร็จรูป |
| `results/` | ผลทดสอบ สำเนาก่อนรีเซ็ต และไฟล์แพ็กเกจที่สร้างไว้ |

## เลือกเปิดไฟล์ตามสิ่งที่จะอธิบาย

| อยากอธิบายเรื่อง | เปิดไฟล์ตามลำดับ |
| --- | --- |
| แอปเริ่มอย่างไร | `main.go` → `app.go` |
| การย้ายและสำรอง | `pages/index.vue` → `usePhotoManager.ts` → `desktop.ts` → `drive_service.go` → `file_mover.go` / `file_copier.go` → `file_record.go` / `backup_job.go` |
| การเก็บข้อมูล | `dbconnect.go` → `migrate.go` → `schema.sql` → repositories |
| วิเคราะห์จนได้คำบรรยายไทย | `gallery.vue` → `AnalyzeImages` ใน `batch_analysis.go` → `hybrid_analyzer.go` / `resident_ai_worker.go` → `app_caption_worker.py` → `translate_captions.py` → `file_record.go` |
| ค้นคำพ้องและพิมพ์ผิด | `gallery.vue` → `captionSearch.ts` → `searchSynonyms.json` |
| ติดตั้ง AI ครั้งแรก | `AISetupScreen.vue` → `ai_setup.go` / `ai_setup_download.go` → `bootstrap_ai.py` |
| รูปตัวอย่างแสดงอย่างไร | `PhotoPreview.vue` → `photo_preview.go` → ไฟล์ภาพบนดิสก์ |
