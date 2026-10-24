# เตรียม AI บน Mac ด้วยการดับเบิลคลิก

**วิธีใหม่สำหรับแจกเป็นแอป:** ดับเบิลคลิก `Build Mac App.command` บน Mac เพื่อสร้าง `.app` ที่มี UI ดาวน์โหลด AI อัตโนมัติ ไม่ต้องทำขั้น Python ด้านล่างก่อน build ดู [วิธีสร้างแอป Mac](BUILD-MAC-APP.md)

เอาโปรเจกต์ล่าสุดทั้งโฟลเดอร์ไปยัง Mac แล้วดับเบิลคลิก `Setup Mac AI.command` ที่ root ของโปรเจกต์ ต้องมีอินเทอร์เน็ตระหว่างติดตั้ง

ตัวติดตั้งสร้าง `.venv` ของ Mac ติดตั้ง PyTorch, torchvision, Transformers และไลบรารีที่จำเป็น ดาวน์โหลด `florence-community/Florence-2-base-ft` ไปยัง Hugging Face cache ของบัญชีที่รัน แล้วลองสร้างคำบรรยายบน MPS หรือ CPU โดยใช้ภาพเปล่าที่สร้างในหน่วยความจำ ไม่เรียก Gemini ไม่แก้ DB รูปภาพ หรือแคชคำบรรยายเดิม กดซ้ำได้หากดาวน์โหลดสะดุด

ต้องมี Python 3.11–3.13 ถ้ายังไม่มี แต่มี Homebrew ตัวติดตั้งจะใช้ `HOMEBREW_NO_ASK=1 brew install python@3.13` เพื่อไม่ให้ค้างที่คำถามยืนยัน หากไม่มีทั้งสองอย่าง จะเปิดหน้าดาวน์โหลด Python ให้ติดตั้งด้วยตนเองก่อน จากนั้นดับเบิลคลิกใหม่ ไม่คัดลอก `.venv` ของ Windows ไปใช้

หากไฟล์เปิดไม่ได้เพราะไม่มีสิทธิ์ execute เปิด Terminal ในโฟลเดอร์โปรเจกต์แล้วทำครั้งเดียว:

```bash
chmod +x "Setup Mac AI.command" "Start Mac App.command"
```

หาก macOS บล็อกไฟล์ที่ดาวน์โหลด ให้ใช้ขั้นตอน Open Anyway ของระบบกับไฟล์จากโปรเจกต์ที่คุณตรวจสอบแล้ว ไม่ต้องปิด Gatekeeper ทั้งระบบ

หลังขึ้น `Florence ready` และ `AI setup complete` ให้ดับเบิลคลิก `Start Mac App.command` เครื่องต้องมี Go, Node.js, Wails v2 และ Xcode Command Line Tools ของแอปเดิมอยู่แล้ว ตัวติดตั้งนี้เตรียมส่วน AI ไม่ติดตั้งเครื่องมือ build เหล่านั้น และไม่สร้าง API key ให้

ตั้ง `GEMINI_API_KEY` ใน `.env` ของโปรเจกต์เหมือนเดิม รุ่นนี้ส่งข้อความไปแปลชุดละ 250 รายการ พัก 10 วินาทีระหว่างชุด เพดานคำตอบ 16,384 tokens เริ่มทดสอบด้วยสำเนารูป 1 รูปก่อน โปรแกรมเลือก MPS เมื่อใช้ได้ แต่ยังต้องตรวจการทำงานจริงบน Mac เป้าหมาย

แหล่งที่มา: [Florence](https://huggingface.co/florence-community/Florence-2-base-ft), [Hugging Face downloads](https://huggingface.co/docs/huggingface_hub/guides/download), [PyTorch MPS](https://docs.pytorch.org/docs/stable/notes/mps.html)
