# ชุดทดสอบ

รวมโค้ดทดสอบทุกภาษาไว้ที่นี่ แยกจากโค้ดหลักของโปรแกรม

```text
tests/
  go/           # Go tests แยกตามแพ็กเกจเดิม
  go-overlay.json
  python/       # Florence worker, ตัวแปล และตัวติดตั้ง AI
  frontend/     # ค้นหา เลือกรูป งานเบื้องหลัง สำรองไฟล์ และหน้าจอติดตั้ง
```

รันจากโฟลเดอร์โปรเจกต์:

```powershell
go test -overlay tests/go-overlay.json ./... -count=1 -timeout 90s
.\.venv\Scripts\python.exe -m unittest discover -s tests/python
npm --prefix frontend test
```

บน macOS/Linux ใช้ `.venv/bin/python` แทน `.venv\Scripts\python.exe`

## Go tests

ต้องรันคำสั่ง Go จากโฟลเดอร์โปรเจกต์พร้อม `-overlay tests/go-overlay.json` โดย Go จะมองไฟล์ใน `tests/go/` เสมือนยังอยู่ข้างแพ็กเกจเดิมเฉพาะตอนทดสอบ จึงยังเข้าถึงฟังก์ชันภายในได้ ไม่ต้องเปลี่ยนโค้ดหลักหรือสร้างไฟล์เทสต์ซ้ำกลับไปในโฟลเดอร์เดิม

`go test ./...` ที่ไม่มี overlay จะไม่พบชุดทดสอบที่ย้ายแล้ว อย่าใช้คำสั่งนั้นเป็นหลักฐานว่าเทสต์ผ่าน ส่วน `go build` และ `wails build` ยังใช้คำสั่งเดิมได้

ไฟล์ `tests/go/go.mod` ใช้แยกโฟลเดอร์เก็บเทสต์ออกจากการบิลด์ปกติ ไม่ใช่โมดูลที่รันทดสอบแยกได้เอง เมื่อเพิ่มหรือเปลี่ยนชื่อ Go test ให้เพิ่มหรือแก้คู่พาธใน `tests/go-overlay.json` ด้วย ตัวอย่าง:

```json
{
  "Replace": {
    "service/example_test.go": "tests/go/service/example_test.go"
  }
}
```

ตัวอย่างข้างบนแสดงรูปแบบหนึ่งรายการ ให้เพิ่มรายการใน mapping เดิมโดยเก็บรายการอื่นไว้

ติดตั้ง dependencies ฝั่ง frontend ด้วย `npm --prefix frontend install` และบิลด์ frontend ด้วย `npm --prefix frontend run build` ก่อนรันเทสต์หน้าจอครั้งแรก เทสต์ส่วนใหญ่ใช้ API จำลองและไฟล์ชั่วคราว ส่วนเทสต์ Go ที่โหลดโมเดลจริงต้องเปิดผ่าน environment เฉพาะตามที่ระบุในไฟล์เทสต์

โค้ดทดสอบไม่ถูกฝังใน EXE หรือ `.app` รูปทดสอบใน `test-images/` และผลลัพธ์ใน `results/` อยู่ตำแหน่งเดิม
