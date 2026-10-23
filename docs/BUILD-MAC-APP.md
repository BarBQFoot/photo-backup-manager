# สร้างแอป Mac พร้อมหน้าจอเตรียม AI

ZIP ที่ให้จาก Windows เป็นซอร์สโปรเจกต์และไฟล์ช่วย build ยังไม่ใช่ `.app` ที่ compile แล้ว Wails v2 ใช้ Cocoa/WebKit จึงต้อง build บน Mac ที่มี Apple SDK หรือ Mac runner จริง ไม่เปลี่ยน EXE เป็น APP ด้วยการเปลี่ยนนามสกุล

## สำหรับ Mac M4 ของเพื่อน

1. แตก ZIP โปรเจกต์ล่าสุดเป็นโฟลเดอร์ใหม่
2. ดับเบิลคลิก `Build Mac App.command`
3. รอจนขึ้นตำแหน่งไฟล์ และ Finder เปิดไฟล์ ZIP ที่สร้างเสร็จ

ถ้าไฟล์ไม่มีสิทธิ์ execute เปิด Terminal ในโฟลเดอร์โปรเจกต์แล้วทำครั้งเดียว:

```bash
chmod +x "Build Mac App.command"
```

สคริปต์ตรวจ Xcode Command Line Tools ถ้ายังไม่มีจะเปิดตัวติดตั้งของ Apple ให้เสร็จก่อนแล้วรันใหม่ ถ้าขาด Go/Node แต่มี Homebrew จะติดตั้งเฉพาะเครื่องมือที่ขาด โดยไม่วนถามยืนยันแบบตัวติดตั้งก่อนหน้า ใช้ Wails v2.13.0 และ Node.js 24 LTS แนะนำ เครื่องที่เคยรันโปรเจกต์เดิมอยู่แล้วไม่ต้องติดตั้งทุกอย่างใหม่

ไม่ต้องรัน `Setup Mac AI.command` ก่อน build และไม่ต้องมี Python เพื่อ build รุ่นนี้ติดตั้ง Python/PyTorch/Florence ผ่าน UI ของ `.app` ตอนเปิดครั้งแรกเอง

คำสั่ง build ที่สคริปต์ใช้บน M4:

```bash
wails build -platform darwin/arm64 -o photo-backup-manager
```

ผลลัพธ์:

- `build/bin/photo-backup-manager.app` — ดับเบิลคลิกเปิดแอปได้
- `build/bin/mac-release/photo-backup-manager-macos-arm64.zip` — ZIP พร้อมส่งให้ Mac Apple Silicon เครื่องอื่น รวม `.app`, `.env` เปล่า และคำแนะนำ

ZIP ที่สคริปต์สร้างไม่คัดลอก `.env` หรือ API key จากเครื่องผู้ build ใช้ `ditto` เพื่อเก็บโครงสร้าง/สิทธิ์ของ bundle Mac ไม่ใช้ ZIP ธรรมดาที่อาจทำ executable permission หาย สำหรับ Intel สคริปต์เลือก `darwin/amd64` ตามเครื่องที่ build แต่ยังไม่ได้รับรองส่วน AI บน Intel Mac

## ตอนเปิด .app

มีหน้าจอเดียวกับ Windows: ตรวจของเดิม → เตรียมเฉพาะส่วนที่ขาด → ดาวน์โหลด Florence → ตรวจโมเดล → เริ่มใช้งาน รองรับ MPS บน Apple Silicon เมื่อ PyTorch ใช้ได้ หากมี Python/Homebrew เดิม จะค้นแม้ Finder ไม่ได้สืบทอด PATH จาก Terminal และหากเปิด .app ที่อยู่ใน build/bin ของโปรเจกต์จะค้น `.venv` ในโปรเจกต์ด้วย

ถ้ามีไลบรารีหรือโมเดลแล้วจะแสดง “มีอยู่แล้ว · ข้าม” ไม่ดาวน์โหลดซ้ำทั้งหมด ช่วงรู้ขนาดแสดง MB/เปอร์เซ็นต์ ช่วงติดตั้งที่ไม่รู้ปริมาณงานแสดงแถบกำลังทำงาน ครั้งแรกใช้อินเทอร์เน็ตและพื้นที่หลาย GB ไม่เรียก Gemini ระหว่างเตรียมระบบ

## คีย์และข้อมูล

ผู้รับใส่คีย์ของตัวเองใน `.env` **ข้างไฟล์ `.app`** ไม่ต้องแก้ไฟล์ภายใน Contents แล้วเปิดแอปใหม่ อีกตำแหน่งที่ใช้ได้คือ `~/Library/Application Support/photo-backup-manager/.env` โดย environment ที่ตั้งเองยังมีลำดับความสำคัญก่อน

ฐานข้อมูล production แยกจาก dev อยู่ที่ `~/Library/Application Support/photo-backup-manager/photo_backup.db` ส่วน runtime/Python ของแอปอยู่ที่ `~/Library/Caches/photo-backup-manager/ai` โมเดลใช้แคช Hugging Face ของผู้ใช้ตามเดิม

แอปที่ build ยังไม่ได้เซ็น Apple Developer ID หรือ notarize ถ้า macOS ปฏิเสธ ให้ใช้ขั้นตอน Open Anyway ของระบบสำหรับแอปที่คุณ build/ตรวจสอบเอง ไม่ปิด Gatekeeper ทั้งระบบ

## ขอบเขตการตรวจ

ตรวจ Bash syntax, เทสต์การค้นพาธของ bundle/Finder และหน้าจอสถานะ MPS ได้จาก Windows แต่ยังไม่ได้รัน build ด้วย Apple SDK หรือทดสอบโมเดลบน Mac จริง ไฟล์ `.app` จะได้หลังเพื่อนรัน builder บน Mac สำเร็จ

แหล่งที่มา: [Wails v2 crossplatform builds](https://wails.io/docs/guides/crossplatform-build/), [Wails build platforms](https://wails.io/docs/reference/cli/#platforms), [PyTorch MPS](https://docs.pytorch.org/docs/stable/notes/mps.html)
