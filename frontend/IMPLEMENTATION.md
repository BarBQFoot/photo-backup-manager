# Frontend handoff — Member 3

Nuxt SPA สำหรับ Wails มีเมนูด้านบนสลับ 3 หน้าและใช้ Destination ร่วมกัน ไม่มี Mock Data

## รูปแบบหน้าจอ

ธีมพื้นครีม สีหลักส้มอิฐ ใช้ Tailwind/DaisyUI และ CSS ใน app/assets/css/main.css ไอคอน SVG รวมอยู่ใน AppIcon.vue

- app/app.vue สร้าง PhotoManager ชุดเดียวให้ layout และทุกหน้าใช้ร่วมกัน และแสดง NuxtLayout / NuxtPage พร้อม keepalive เพื่อเก็บสถานะเมื่อสลับหน้า
- app/layouts/default.vue เก็บเมนู ส่วนหัว สถานะ Desktop ข้อความแจ้งเตือน และ footer
- app/pages/index.vue เป็นหน้าย้ายรูป (/), gallery.vue เป็นแกลเลอรี (/gallery), history.vue เป็นประวัติและตรวจสอบ (/history)
- app/components/ เก็บ UI ย่อยที่ใช้ร่วมกัน เช่น AppIcon และ ConfirmDialog
- app/composables/useSharedPhotoManager.ts รับ PhotoManager จาก app.vue ผ่าน provide/inject โดยไม่สร้างข้อมูลแยกในแต่ละหน้า
- ใช้ NuxtLink และ hash routing สำหรับ Wails เช่น /#/gallery

- **Move Photos:** SelectFolder, ScanDrive, StartBackup, GetBackupProgress และ backup:progress; ยืนยันย้ายและสรุปผลพร้อมเวลาจริง
- **Gallery:** ScanDrive, GetFileMetadata; อ่าน AI Status / Description และกรองคำอธิบายที่โหลดแล้วภายในปลายทางปัจจุบัน
- **History & Check:** GetBackupHistory, ScanDrive, DeleteFile; ตรวจ Missing และยืนยันชื่อ/พาธก่อนลบ

## รัน

จากโฟลเดอร์โปรเจกต์หลัก รัน `wails dev` เพื่อใช้งาน Backend และไฟล์จริง

จาก frontend รัน `npm run dev` เพื่อดูหน้าจอในเบราว์เซอร์ โดยไม่เข้าถึงไฟล์จริง บน PowerShell ที่บล็อก npm.ps1 ใช้ `npm.cmd run dev`

`npm run build` สร้าง Static SPA ใน frontend/dist ให้ตรงกับ Go Embed ของ Wails

ก่อนรัน Wails ครั้งแรกหรือเมื่อ dist ว่าง ให้รัน `npm.cmd --prefix frontend run build` จากโฟลเดอร์หลัก แล้วจึง `wails dev` โดย Nuxt ตั้งค่า dist เฉพาะ production เพื่อไม่ให้ Dev Server ล้างไฟล์ที่ Go ต้อง Embed

## จุดเชื่อมต่อที่รอทีม

- app/services/desktop.ts ครอบ Generated Bindings โดยไม่แก้ Generated Files
- app/composables/usePhotoManager.ts เก็บ State, Error และอัปเดตรายการหลังย้าย/ลบ
- ปุ่ม Analyze ปิดไว้ รอ Member 4 เปิด AnalyzeImage(path) ตาม Contract และสร้าง Bindings ใหม่ แล้วจึงเพิ่ม Adapter และสถานะ Loading/Success/Error ของคำขอ AI
- ช่องค้น Gallery กรอง Description จากข้อมูลสแกนจริง เมื่อมี SearchImages(query, destinationPath) ให้เปลี่ยนไปเรียกผ่าน Adapter โดยคงขอบเขต Destination และไม่เปิดไฟล์ Missing/Deleted
- Thumbnail/Preview เป็น Placeholder พร้อมข้อความ รอทีมกำหนดกลไกแสดงไฟล์จริงผ่าน Backend/Wails ไม่ใช้ file:// หรืออ่านระบบไฟล์จาก Browser และ Missing ไม่แสดง Preview

## ตรวจใน Wails

1. เปิด History โดยไม่เลือกปลายทาง ต้องเห็นคำแนะนำ
2. เลือกโฟลเดอร์ทดสอบ สแกน เลือกทั้งหมด และกด Escape เพื่อยกเลิกกล่องยืนยัน
3. ย้ายรูปทดสอบ ตรวจ Progress ผลรวม ไฟล์ซ้ำ และรายการต้นทางหลังย้าย
4. เปิด Gallery เลือกรูป ตรวจรายละเอียด และค้น Description ภาษาไทยถ้ามีข้อมูลเดิม
5. เปิด History ตรวจประวัติ นำไฟล์ทดสอบออกจากปลายทาง แล้วกดตรวจสอบเพื่อดู Missing
6. ลบไฟล์ทดสอบที่มี File ID หลังตรวจชื่อและพาธในกล่องยืนยัน ตรวจว่ารายการอัปเดต
7. ทดสอบปลายทางว่าง/เข้าถึงไม่ได้ และ Keyboard Navigation
