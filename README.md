# Photo Backup Manager

แอป Desktop สำหรับเลือกรูปจากโฟลเดอร์ต้นทาง แล้วย้ายหรือสำรองแบบคัดลอกไปยังปลายทาง พร้อมดูประวัติ ตรวจไฟล์ที่หาย วิเคราะห์รูปด้วย AI และค้นหารูปจากคำบรรยาย

วิเคราะห์ภาพเบื้องหลังได้ระหว่างเปิดดูรูป ค้นหา และจัดเก็บรูปอื่น เอกสารแผนเดิมใน `docs` เก็บไว้เป็นข้อกำหนดโครงงาน

## เริ่มต้น

- อ่าน [เข้ามาใหม่อ่านอันนี้ก่อน](docs/เข้ามาใหม่อ่านอันนี้ก่อน.md) เพื่อเลือกอ่านตามหน้าที่
- อ่าน [พิมพ์เขียวโครงการ](docs/00-PROJECT-BLUEPRINT.md) เพื่อดูขอบเขตและสถานะงาน
- ดู [คู่มือติดตั้ง](docs/SETUP_GUIDE.md) เพื่อเตรียมเครื่องและรันโปรเจกต์
- ส่วน AI ปัจจุบันใช้ **Florence อ่านภาพบนเครื่อง → Gemini แปลคำบรรยาย** ดู [วิธีรัน AI ในแอป](docs/RUN-APP-LOCAL-AI.md)
- สำหรับ Mac ดู [ตัวติดตั้ง AI แบบดับเบิลคลิก](docs/SETUP-MAC-AI.md)
- การค้นหาคำตรง คำพ้อง และคำพิมพ์ผิด ดู [วิธีค้นหาคำบรรยาย](docs/SEARCH.md)
- การแจก EXE และหน้าจอเตรียม AI อัตโนมัติ ดู [เปิดแอปบน Windows](docs/RUN-WINDOWS-EXE.md)
- การสร้าง `.app` แบบเดียวกันบน Mac ดู [Build Mac App](docs/BUILD-MAC-APP.md)
- การเลือกย้ายหรือสำรองแบบคัดลอก ดู [วิธีจัดเก็บรูป](docs/BACKUP-MODES.md)
- การทำงานระหว่างรอ AI ดู [วิเคราะห์ภาพเบื้องหลัง](docs/BACKGROUND-ANALYSIS.md)
- ตำแหน่งและคำสั่งรันชุดทดสอบ ดู [ชุดทดสอบ](tests/README.md)

## พัฒนาและ Build

```sh
cd frontend
npm install
cd ..
wails dev
```

สร้างแอปสำหรับแจกจ่ายด้วย `wails build` ดูข้อกำหนดของระบบปฏิบัติการเพิ่มเติมได้ใน [คู่มือติดตั้ง Wails](https://wails.io/docs/gettingstarted/installation)

## เทคโนโลยี

Go · Wails v2 · Nuxt · SQLite/GORM · Python/PyTorch · Florence · Gemini
## ดาวน์โหลดโปรแกรม

[ดาวน์โหลดโปรแกรมสำหรับ Windows](https://github.com/BarBQFoot/photo-backup-manager/releases/latest)

แตก ZIP แล้วเปิดไฟล์ .exe เพื่อใช้งาน

