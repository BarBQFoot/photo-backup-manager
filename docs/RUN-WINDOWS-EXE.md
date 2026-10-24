# เปิดแอปจาก EXE และเตรียม AI อัตโนมัติ

Build บน Windows ด้วยเครื่องมือพัฒนาที่ติดตั้งไว้แล้ว:

```powershell
wails build -webview2 download -o photo-backup-manager-setup.exe
```

ผลลัพธ์อยู่ที่ `build/bin/photo-backup-manager-setup.exe` ไฟล์เดียวรวมหน้าจอ Go backend และสคริปต์ Python แต่ไม่ได้รวมโมเดลหรือ PyTorch หลาย GB ไว้ใน EXE โปรแกรมปลายทางไม่ต้องมี Go, Node.js หรือ Wails ส่วน WebView2 ใช้กลไกดาวน์โหลด runtime ของ Wails เมื่อจำเป็น

## ตอนเปิดแอป

หน้าจอแสดงขั้นตอนตรวจเครื่อง เตรียม Python, PyTorch, ไลบรารี Florence ดาวน์โหลดโมเดล และทดสอบว่าอ่านภาพได้จริง โดยใช้ภาพเปล่าในหน่วยความจำ ไม่แตะรูปของผู้ใช้และไม่เรียก Gemini

- มี Python environment และไลบรารีที่ใช้ได้อยู่แล้ว จะนำมาใช้และข้ามการดาวน์โหลดส่วนนั้น
- มี PyTorch แต่ขาดไลบรารีอื่น จะติดตั้งเฉพาะส่วนที่ขาด ไม่เปลี่ยน PyTorch เดิมเป็น CPU โดยอัตโนมัติ
- มี Python ระบบแต่ยังขาดไลบรารี จะสร้าง environment ของแอปโดยใช้ interpreter เดิมและอนุญาตใช้แพ็กเกจระบบที่มีแล้ว ไม่ติดตั้งแพ็กเกจลง Python ระบบโดยตรง
- ไม่มี Python จะดาวน์โหลด uv และ Python 3.13 ลงโฟลเดอร์ของแอป โดยตรวจ SHA256 ของ uv เทียบค่าที่ตรึงในซอร์สก่อนใช้
- มีไฟล์ Florence อยู่ในแคช Hugging Face แล้ว จะไม่ดาวน์โหลดซ้ำ ตรวจไฟล์ที่จำเป็นและทำ fingerprint ตาม interpreter, เวอร์ชันไลบรารี และขนาด/เวลาแก้ไขของโมเดล
- ทดสอบโมเดลครั้งแรกหรือเมื่อ fingerprint เปลี่ยน เปิดครั้งถัดไปข้ามการทดสอบหนักเมื่อยังเป็นชุดเดิม
- รู้ขนาดข้อมูลจะแสดง MB/เปอร์เซ็นต์ หรือจำนวนไฟล์ ช่วงติดตั้ง/ตรวจโมเดลที่ไม่รู้ปริมาณงานจะแสดงแถบกำลังทำงาน ไม่ใช้เปอร์เซ็นต์สมมติ ตัวเลขแสดงขั้นตอนปัจจุบัน ไม่ใช่เปอร์เซ็นต์รวมของทุกขั้นตอน
- ดาวน์โหลดสะดุดแสดงข้อผิดพลาดและปุ่มลองใหม่ ใช้แคชที่ดาวน์โหลดสำเร็จแล้วต่อได้ หรือเข้าแอปโดยยังไม่ใช้ AI ได้ ปิดแอปจะยกเลิกงานเตรียมระบบ

การติดตั้งใหม่เลือก PyTorch CUDA 12.6 บน Windows x64 เมื่อพบ NVIDIA driver รุ่น 560 ขึ้นไป มิฉะนั้นใช้ CPU ส่วน Mac ใช้แพ็กเกจมาตรฐานและตรวจ MPS การติดตั้งครั้งแรกต้องมีอินเทอร์เน็ตและพื้นที่หลาย GB

## ข้อมูลและ Gemini key

EXE แบบ production เก็บฐานข้อมูลแยกที่ `%APPDATA%\photo-backup-manager\photo_backup.db` ไม่แก้ฐานข้อมูล dev ที่ `database/photo_backup.db` ตั้ง `PHOTO_BACKUP_DB` เพื่อใช้พาธอื่นได้ ไม่ต้องวาง EXE ในโฟลเดอร์ที่เขียนไฟล์ได้

Python, uv และแคชแพ็กเกจของแอปอยู่ใต้ `%LOCALAPPDATA%\photo-backup-manager\ai` ส่วนโมเดลใช้แคช Hugging Face ของผู้ใช้ตามเดิมเพื่อใช้ไฟล์ที่มีอยู่แล้วได้ หากตั้ง `PHOTO_AI_CACHE` หรือ `HF_HOME` จะใช้ตำแหน่งที่กำหนด

สร้าง `.env` ข้าง EXE สำหรับคีย์ของเครื่องนั้น:

```dotenv
GEMINI_API_KEY=ใส่คีย์ของคุณที่นี่
```

คีย์ไม่ฝังใน EXE และไม่ส่งให้ subprocess ที่ติดตั้ง AI เฉพาะ worker แปลคำบรรยายจึงจะรับคีย์ การเตรียมระบบ AI สำเร็จไม่ได้หมายความว่าคีย์หรือโควตา Gemini ถูกตรวจแล้ว

## ตรวจสอบ

```powershell
go test ./... -count=1
.\.venv\Scripts\python.exe -m unittest tools.test_bootstrap_ai tools.test_app_caption_worker tools.test_translate_captions
node tools/test_setup_ui.cjs
```

ทดสอบดาวน์โหลด Python จริงในแคชทดสอบแยก (ไม่ติดตั้ง PyTorch หรือโหลดโมเดลใหม่):

```powershell
$env:PHOTO_AI_SETUP_BOOTSTRAP_SMOKE = '1'
go test ./service -run '^TestAISetupFreshPythonBootstrap$' -v -count=1
```

ทดสอบใช้ Florence ที่ติดตั้งไว้แล้วบนเครื่อง Windows และตรวจว่ารอบที่สองข้ามการโหลดทดสอบ โดยไม่มี Gemini:

```powershell
$env:PHOTO_AI_SETUP_EXISTING_SMOKE = '1'
go test ./service -run '^TestAISetupRealExistingMachine$' -v -count=1
```

ยังต้องทดสอบ EXE บน Windows เครื่องใหม่จริงเพื่อยืนยันกรณี driver, Visual C++ runtime, proxy/antivirus และสิทธิ์ของเครื่องนั้น ไม่ได้อ้างว่าการทดสอบใน environment แยกแทนเครื่อง Windows สะอาดได้ และยังไม่ยืนยัน build Mac/Linux

แหล่งที่มา: [uv Python management](https://docs.astral.sh/uv/guides/install-python/), [uv 0.13.5](https://github.com/astral-sh/uv/releases/tag/0.13.5), [PyTorch](https://pytorch.org/get-started/locally/), [Florence](https://huggingface.co/florence-community/Florence-2-base-ft), [Wails options](https://wails.io/docs/reference/options/)

## ดาวน์โหลด PyTorch หมดเวลารอ

ตัวติดตั้งกำหนด `UV_HTTP_TIMEOUT=300` วินาทีต่อการอ่าน HTTP, connect timeout 60 วินาที, HTTP retries 3 และดาวน์โหลดพร้อมกันไม่เกิน 2 ไฟล์ เพื่อรองรับ PyTorch CUDA wheel ขนาดหลาย GB บนเครือข่ายที่สะดุด ค่าเริ่มต้น HTTP read timeout ของ uv คือ 30 วินาทีตาม [เอกสาร uv](https://docs.astral.sh/uv/reference/environment/#uv_http_timeout) การเพิ่ม timeout ไม่ได้ทำให้ดาวน์โหลดเร็วขึ้นหรือแก้เครือข่ายที่บล็อกเว็บ

หากล้มเหลว UI จะแสดงข้อความท้ายจากตัวติดตั้งไม่เกิน 8 บรรทัด เพื่อให้เห็นแพ็กเกจ/URL และสาเหตุร่วมกัน ไม่เหลือเพียง `cause: operation timed out` ใช้ปุ่มลองอีกครั้งในรุ่นใหม่โดยไม่ล้างแคชที่สำเร็จแล้ว

## ตรวจ snapshot Florence

ตัวดาวน์โหลดและตัวตรวจแคชออฟไลน์ใช้ `MODEL_PATTERNS` เดียวกัน (`*.json`, `*.safetensors`, `*.txt`, `*.model`, `*.tiktoken`) พร้อมตรวจไฟล์ runtime หลักและขนาดไม่เป็นศูนย์ ก่อนโหลดทดสอบจริง จึงไม่ถือว่าขาดโมเดลเพียงเพราะไม่มี README.md หรือ .gitattributes ที่ไม่ได้เลือกดาวน์โหลด huggingface_hub รุ่นที่ตรวจความครบของทั้ง snapshot จะตรวจเฉพาะไฟล์ตามตัวกรองเดียวกัน
