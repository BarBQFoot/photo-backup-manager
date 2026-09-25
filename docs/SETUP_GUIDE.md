# คู่มือติดตั้ง Photo Backup Manager

คู่มือนี้แนะนำการ Clone และเตรียมโปรเจกต์ **Photo Backup Manager** สำหรับนักศึกษาที่เพิ่งเริ่มต้นใช้งาน Git และ Go

## ข้อมูลโปรเจกต์

- **ชื่อโปรเจกต์:** Photo Backup Manager
- **GitHub Repository:** [BarBQFoot/photo-backup-manager](https://github.com/BarBQFoot/photo-backup-manager)
- **เทคโนโลยีหลัก:** Go, Wails และ Nuxt (ทำงานผ่าน Node.js/npm)

## โปรแกรมที่ต้องติดตั้งก่อน

ติดตั้งโปรแกรมต่อไปนี้ให้เรียบร้อย และเปิด Terminal ใหม่หลังติดตั้งหากระบบยังไม่รู้จักคำสั่ง

- **Git** สำหรับดาวน์โหลดและจัดการ source code
- **Go** สำหรับส่วน backend (โปรเจกต์นี้กำหนด Go `1.25.0` ใน `go.mod`)
- **Node.js** ซึ่งมาพร้อม **npm** ใช้จัดการ frontend; ใช้ Node.js รุ่น LTS ที่ยังได้รับการสนับสนุน
- **Wails CLI รุ่น 2** สำหรับรันแอปพลิเคชัน (โปรเจกต์ใช้ Wails v2)
- **ข้อกำหนดตามระบบปฏิบัติการ:** Wails ต้องใช้เครื่องมือ build และไลบรารีระบบที่เหมาะกับ OS ของคุณ โปรดดู [คู่มือติดตั้ง Wails](https://wails.io/docs/gettingstarted/installation) เลือก Windows, macOS หรือ Linux ให้ตรงกับเครื่อง

### ตรวจสอบเวอร์ชัน

เปิด Terminal (บน Windows ใช้ PowerShell หรือ Windows Terminal) แล้วรันคำสั่งเหล่านี้:

```sh
git --version
go version
node --version
npm --version
wails version
```

ถ้าคำสั่งใดแจ้งว่าไม่รู้จักคำสั่ง ให้ตรวจว่าติดตั้งโปรแกรมนั้นแล้ว และ PATH ถูกตั้งค่าเรียบร้อย จากนั้นปิดและเปิด Terminal ใหม่

## Clone โปรเจกต์

```sh
git clone https://github.com/BarBQFoot/photo-backup-manager.git
cd photo-backup-manager
```

## ติดตั้ง Dependencies

ติดตั้งแพ็กเกจของ frontend ก่อน แล้วกลับมาที่โฟลเดอร์หลักเพื่อตรวจสอบและจัดเตรียม Go modules:

```sh
cd frontend
npm install
cd ..
go mod tidy
```

`npm install` จะดาวน์โหลดแพ็กเกจที่ระบุใน `frontend/package.json` ส่วน `go mod tidy` จะจัด dependencies ของ Go ตาม source code และบันทึกข้อมูลลง `go.mod`/`go.sum` หากจำเป็น

## รันโปรเจกต์

ตรวจว่า Terminal อยู่ที่โฟลเดอร์หลัก `photo-backup-manager` แล้วรัน:

```sh
wails dev
```

Wails จะเริ่ม frontend development server และเปิดแอปในโหมดพัฒนา การรันครั้งแรกอาจใช้เวลาสักครู่เพื่อดาวน์โหลดหรือ build ส่วนประกอบที่จำเป็น

## ไฟล์ที่ไม่ได้ใส่ไว้ใน GitHub

ไฟล์และโฟลเดอร์ด้านล่างถูกระบุไว้ใน `.gitignore` หลักของโปรเจกต์ จึงไม่ถูกเพิ่มเข้า Git ตามปกติ:

| รายการ | เหตุผลที่ไม่ Commit |
| --- | --- |
| `node_modules/` | แพ็กเกจที่ติดตั้งในเครื่อง มีขนาดใหญ่และสร้างใหม่ได้ด้วย `npm install` |
| `frontend/.nuxt/` | ไฟล์ชั่วคราวและไฟล์ที่ Nuxt สร้างระหว่างพัฒนา |
| `frontend/.output/` | ผลลัพธ์ที่ Nuxt สร้างสำหรับการ build/รัน |
| `frontend/dist/` | ไฟล์ผลลัพธ์ที่ build แล้ว สร้างใหม่จาก source ได้ |
| `build/bin/` | ไฟล์โปรแกรมที่ Wails build ออกมา ซึ่งขึ้นกับเครื่องและสร้างใหม่ได้ |
| `data/` | โฟลเดอร์ข้อมูลในเครื่อง อาจมีข้อมูลผู้ใช้หรือข้อมูลที่ไม่ควรแจกจ่าย |
| `*.db`, `*.sqlite`, `*.sqlite3` | ไฟล์ฐานข้อมูลที่อาจมีข้อมูลส่วนบุคคลหรือข้อมูลเฉพาะเครื่อง |
| `.env`, `.env.*` | ไฟล์ตั้งค่า environment ในเครื่อง ซึ่งอาจเก็บ secrets หรือค่าที่ไม่เหมือนกันในแต่ละเครื่อง |

เมื่อ Clone ใหม่ โฟลเดอร์หรือไฟล์เหล่านี้อาจยังไม่มี ซึ่งเป็นเรื่องปกติ โปรแกรมหรือคำสั่งติดตั้ง/build จะสร้างส่วนที่จำเป็นขึ้นใหม่ตามการใช้งาน

หมายเหตุ: `frontend/.gitignore` มีข้อยกเว้น `!.env.example` สำหรับไฟล์ตัวอย่าง environment แต่ไฟล์ดังกล่าวไม่ใช่ที่เก็บ API Key จริง

## การตั้งค่า Gemini API Key

Repository นี้ไม่ได้ใส่ Gemini API Key จริงไว้ใน GitHub หากต้องการใช้ฟีเจอร์ที่เรียก Gemini ให้สร้าง API Key ของตนเอง และตั้งค่าผ่าน environment variable ชื่อ `GEMINI_API_KEY` ใน environment ที่ใช้เปิดแอป

ตัวอย่างการตั้งค่าชั่วคราวสำหรับ Terminal ปัจจุบัน:

**PowerShell (Windows):**

```powershell
$env:GEMINI_API_KEY = Read-Host "กรอก Gemini API Key ของคุณ"
```

**macOS/Linux (bash หรือ zsh):**

```sh
read -s GEMINI_API_KEY
export GEMINI_API_KEY
```

หลังตั้งค่าแล้ว ให้รัน `wails dev` ใน Terminal เดิม เพื่อให้โปรเซสได้รับ environment variable นี้ การตั้งค่านี้มีผลกับ Terminal ปัจจุบันเท่านั้น

ห้ามเขียน API Key จริงลงใน source code, เอกสาร หรือไฟล์ที่จะ Commit และห้าม Commit API Key ขึ้น GitHub หากเผลอเผยแพร่ Key ให้เพิกถอนหรือหมุนเวียน Key นั้นทันที ทั้งนี้การตั้งค่า environment variable เป็นวิธีส่งค่าให้โปรเซสเท่านั้น ความพร้อมใช้งานของฟีเจอร์ยังขึ้นกับการ implement ใน source code ของเวอร์ชันที่กำลังใช้งาน

## คำสั่งทั้งหมดตั้งแต่ Clone จนถึง Run

รันตามลำดับจาก Terminal:

```sh
git clone https://github.com/BarBQFoot/photo-backup-manager.git
cd photo-backup-manager
cd frontend
npm install
cd ..
go mod tidy
wails dev
```

หากใช้ Gemini ให้ตั้ง `GEMINI_API_KEY` ใน Terminal ก่อนคำสั่ง `wails dev` ตามวิธีด้านบน

## Troubleshooting เบื้องต้น

### `npm install` ไม่ผ่าน

- ตรวจการเชื่อมต่ออินเทอร์เน็ต และตรวจเวอร์ชันด้วย `node --version` และ `npm --version`
- ตรวจว่าอยู่ในโฟลเดอร์ `frontend` ซึ่งมี `package.json` ก่อนรันคำสั่ง
- หากยังไม่สำเร็จ ให้อ่านข้อความ error เต็ม ๆ และตรวจว่ามี Node.js รุ่นที่ Nuxt รองรับ

### Go dependencies มีปัญหา

- ตรวจ `go version` และให้ตรงตามข้อกำหนดใน `go.mod` (Go `1.25.0`)
- ตรวจการเชื่อมต่ออินเทอร์เน็ต แล้วลอง `go mod tidy` อีกครั้งจากโฟลเดอร์หลักที่มี `go.mod`
- ตรวจว่า Go เปิดใช้งาน module mode ตามค่าเริ่มต้น และอ่านบรรทัดแรกของ error เพื่อหาชื่อแพ็กเกจหรือสาเหตุที่ขาด

### `wails` command not found

- ตรวจว่าติดตั้ง Wails CLI รุ่น 2 แล้ว และทำตามขั้นตอนเพิ่ม executable ของ Go ลง PATH ในคู่มือ Wails
- ปิดและเปิด Terminal ใหม่ แล้วตรวจด้วย `wails version`
- ตรวจว่าติดตั้ง Go และตั้งค่า PATH ถูกต้องด้วย `go version`

### โปรแกรมรันไม่ได้

- ตรวจว่ารัน `wails dev` จากโฟลเดอร์หลักของโปรเจกต์ (ที่มี `wails.json` และ `go.mod`)
- ตรวจว่าติดตั้ง Git, Go, Node.js/npm, Wails CLI และข้อกำหนด build ของระบบปฏิบัติการครบแล้ว
- ตรวจ error ใน Terminal; ถ้าเกี่ยวกับ frontend ให้ลอง `npm install` ใน `frontend` แล้วกลับมา root และรัน `wails dev` อีกครั้ง
- หากระบบแจ้งว่า port ถูกใช้งาน ให้ปิดโปรเซส development เก่าที่ค้างอยู่แล้วลองใหม่
