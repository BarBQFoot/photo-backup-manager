# Photo Backup Manager

แอป Desktop สำหรับเลือกรูปจากโฟลเดอร์ต้นทาง แล้วย้ายไปยังปลายทาง พร้อมดูประวัติและตรวจไฟล์ที่หาย ในขอบเขต Final ผู้ใช้ยังวิเคราะห์รูปด้วย AI เพื่อบันทึกคำอธิบายและค้นหารูปจากคำอธิบายได้

> **สถานะปัจจุบัน:** Repository อยู่ในระยะ Wails/Nuxt scaffold ความสามารถในเอกสารเป็นแผนและข้อกำหนด ยังไม่ได้หมายความว่าฟีเจอร์เหล่านั้นพัฒนาเสร็จแล้ว

## เริ่มต้น

- อ่าน [เข้ามาใหม่อ่านอันนี้ก่อน](docs/เข้ามาใหม่อ่านอันนี้ก่อน.md) เพื่อเลือกอ่านตามหน้าที่
- อ่าน [พิมพ์เขียวโครงการ](docs/00-PROJECT-BLUEPRINT.md) เพื่อดูขอบเขตและสถานะงาน
- ดู [คู่มือติดตั้ง](docs/SETUP_GUIDE.md) เพื่อเตรียมเครื่องและรันโปรเจกต์
- ส่วน AI ปัจจุบันใช้ **Florence อ่านภาพบนเครื่อง → Gemini แปลคำบรรยาย** ดู [วิธีรัน AI ในแอป](docs/RUN-APP-LOCAL-AI.md)
- สำหรับ Mac ดู [ตัวติดตั้ง AI แบบดับเบิลคลิก](docs/SETUP-MAC-AI.md)
- การค้นหาคำตรง คำพ้อง และคำพิมพ์ผิด ดู [วิธีค้นหาคำบรรยาย](docs/SEARCH.md)
- การแจก EXE และหน้าจอเตรียม AI อัตโนมัติ ดู [เปิดแอปบน Windows](docs/RUN-WINDOWS-EXE.md)
- การสร้าง `.app` แบบเดียวกันบน Mac ดู [Build Mac App](docs/BUILD-MAC-APP.md)

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


