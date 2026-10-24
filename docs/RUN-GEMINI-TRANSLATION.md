# แปลคำบรรยาย Florence ด้วย Gemini API

สคริปต์ `tools/translate_captions.py` ส่งเฉพาะ file_id และคำบรรยายภาษาอังกฤษครั้งละ 50 รายการตามไฟล์ที่เตรียมไว้ ไม่ส่งรูปหรือพาธรูป และไม่แก้ฐานข้อมูลแอป ใช้ Python standard library ไม่ต้องติดตั้ง SDK เพิ่ม

## ดูแผนโดยไม่เรียก API

```powershell
.\.venv\Scripts\python.exe -u tools/translate_captions.py --run results/florence_20261022_222031_582707
```

ค่าเริ่มต้นเป็น dry run อ่าน Key จาก environment ก่อน แล้วจึงอ่าน GEMINI_API_KEY จาก `.env` ที่ root โปรเจกต์ แสดงเพียงว่ามี Key หรือไม่ ไม่แสดงค่า Key และไม่มีการเขียนผลหรือส่งคำขอในโหมดนี้

ตรวจแผนกับข้อมูลจริงแล้ว: 1000 รายการ, 20 ชุด, พบคำตอบชุดแรกครบ 50 รายการ จึงเหลือ 19 ชุด (`batch_002`–`batch_020`) การตรวจแผนยังไม่ได้ยืนยันว่า Key มีสิทธิ์เรียกโมเดลหรือเหลือโควตาเพียงพอ

## ทดลองส่งจริงเพียงชุดถัดไป

```powershell
.\.venv\Scripts\python.exe -u tools/translate_captions.py --run results/florence_20261022_222031_582707 --execute --max-batches 1 --retries 0
```

คำสั่งนี้ใช้โควตา Gemini API จริง ข้ามชุดที่บันทึกคำตอบครบแล้ว ส่งชุดที่ยังขาดเพียงหนึ่งชุด และไม่ retry ในการทดลองครั้งนี้ หากชุดถัดไปสำเร็จ จะรวมเป็นคำแปลที่บันทึกแล้ว 100 รูปและค้างอีก 900 รูปในข้อมูลปัจจุบัน

## รันต่อส่วนที่เหลือ

หลังตรวจผลการทดลองหนึ่งชุดแล้ว:

```powershell
.\.venv\Scripts\python.exe -u tools/translate_captions.py --run results/florence_20261022_222031_582707 --execute
```

โหลดโมเดลเริ่มต้น `gemini-2.5-flash` ตามแอป Go เดิม เปลี่ยนด้วย `--model <ชื่อโมเดล>` หากบัญชีใช้งานโมเดลนั้นไม่ได้ Google ระบุว่า 2.5 ยังให้บริการต่อ แต่จำกัดสิทธิ์กับผู้ที่ใช้งานมาก่อน จึงต้องตรวจสิทธิ์ของโปรเจกต์จริง ไม่เปลี่ยนไฟล์แอป Go อัตโนมัติ

ค่าเริ่มต้นเว้นคำขออย่างน้อย 15 วินาทีและ retry เฉพาะ HTTP ชั่วคราวได้อีกหนึ่งครั้ง ตั้ง `--retries 0` เพื่อไม่ retry เลยได้ การรวมเป็นชุดไม่ได้ยกเลิกลิมิต RPM/TPM/RPD และยังไม่รับประกันว่าโควตาฟรีเพียงพอ

หากตรวจพบลิมิตรายวัน หรือผู้ให้บริการขอให้รอนานกว่า 60 วินาที จะหยุดเพื่อรันต่อภายหลัง หาก timeout/เครือข่ายขาดจะไม่ retry อัตโนมัติ เพราะคำขอเดิมอาจประมวลผลไปแล้ว กด Ctrl+C เพื่อหยุดระหว่างรันได้ และตรวจผลที่บันทึกก่อนรันต่อ

## การเก็บและตรวจผล

- ข้ามชุดที่มีคำตอบครบและรหัสถูกต้อง รวมถึงชุดที่คัดลอกกลับมาด้วยมือ คำแปลที่มีไฟล์ `_translation_edited.json` ได้ลำดับความสำคัญก่อน
- เก็บ hash ของข้อความขาเข้า ถ้าข้อความเดิมเปลี่ยนหลังมีบันทึก hash จะหยุดแทนการนำคำแปลเก่ามาจับกับข้อความใหม่ ชุด manual ที่ยังไม่มี hash จะรับจากการตรวจรหัสและบันทึก hash เมื่อ execute ครั้งแรก
- ใช้ API Key ผ่าน header ไม่วางใน URL ไม่พิมพ์ Key หรือบันทึก header และลบสำเนา Key ที่อาจปรากฏในข้อมูลตอบกลับก่อนบันทึก
- ขอ JSON schema จาก Gemini แล้วตรวจ ID ครบ ไม่ซ้ำ ไม่เกินรายการที่ส่ง ไม่ใช้ลำดับ array ในการจับคู่ ปฏิเสธคำตอบที่ finishReason ไม่ใช่ STOP รวมถึง MAX_TOKENS แม้ JSON ดูเหมือนครบ
- คำตอบรหัสผิดจะเก็บ provider reply ไว้ตรวจ แต่ไม่ถือว่าชุดนั้นเสร็จ การรันครั้งถัดไปจะส่งชุดที่ยังไม่สำเร็จใหม่ ไม่ได้ retry เฉพาะ ID ที่ขาดภายในชุด
- มี OS file lock เพื่อป้องกันการรันตัวแปลซ้อนในโฟลเดอร์เดียวกัน ถ้าโปรเซสหยุด lock ของ OS จะถูกปล่อยโดยอัตโนมัติ

ผลในโฟลเดอร์รอบนั้น:

- `translation_responses/batch_XXX_api_response.json`: คำตอบที่ผ่านการตรวจรูปแบบ
- `translation_responses/batch_XXX_attempt_<เวลา>.json`: สำเนา provider reply รวมถึงกรณีที่ตรวจไม่ผ่าน
- `translation_responses/batch_XXX_api_validation.json`: รายงานตรวจรหัสและรูปแบบ
- `translation_responses/batch_XXX_meta.json`: hash ข้อความขาเข้า โมเดล และข้อมูล token usage เมื่อมี
- `captions_translated.json`: รวมทุก file_id พร้อม caption_en, caption ภาษาไทย และ translation_status เป็น translated/pending/needs_review
- `translation_summary.json`: จำนวนสำเร็จ ค้าง จำนวนคำขอในรอบนี้ และสาเหตุหยุดถ้ามี

การผ่านการตรวจ JSON และรหัสไม่ได้ยืนยันว่าคำแปลตรงต้นฉบับหรือคำบรรยายตรงภาพ ข้อผิดพลาดจาก Florence ยังอาจติดมาในภาษาไทย และยังไม่ได้นำคำแปลเข้าฐานข้อมูล Go/Wails

## การทดสอบ

```powershell
.\.venv\Scripts\python.exe -m unittest discover -s tests/python -p test_translate_captions.py -v
```

16 tests ใช้ provider จำลองทั้งหมด ครอบคลุม resume, merge ด้วย ID, โควตา/เวลารอ, timeout, คำตอบถูกตัด, รหัสผิด, การเปลี่ยน source, dry run ไม่ส่ง/ไม่เขียนข้อมูล, การไม่พิมพ์ Key, schema ขนาดเล็ก, รายละเอียด HTTP 400 แบบปิดบัง Key และ file lock ยังไม่ได้ทดสอบการสร้างคำตอบจาก provider สำเร็จจริงหรือรัน Mac/Linux

## กรณี HTTP 400 ในการทดลองชุดแรก

ผู้ใช้ทดลองส่ง batch_002 หนึ่งครั้งแล้วได้ HTTP 400 โดยโค้ดรุ่นแรกแสดงเพียงข้อความทั่วไป ผล 50 รูปที่บันทึกไว้ไม่สูญหายและยังเหลือ 950 รูปรอแปล

เพิ่มรายละเอียดจาก error.message/status/reason ของ Google โดยปิดบัง Key และเพิ่มการตรวจ metadata แบบไม่สร้างคำบรรยาย:

```powershell
.\.venv\Scripts\python.exe -u tools/translate_captions.py --run results/florence_20261022_222031_582707 --check-access
```

ตรวจ metadata จริงผ่าน และ countTokens รับคำขอเดิมได้ 2570 โทเคน โดยไม่ได้ส่ง generateContent เพิ่ม การตรวจสองอย่างนี้ไม่ได้ยืนยันว่า generateContent จะผ่านหรือมีโควตาเพียงพอ ยังไม่สามารถสรุปสาเหตุ HTTP 400 เดิมได้เพราะไม่ได้เก็บรายละเอียดจาก provider ไว้

ลด schema ฝั่ง provider ให้เหลือ array ของ file_id/caption โดยนำ enum รหัส 50 ค่าและ minItems/maxItems ออก การตรวจจำนวนและ ID ครบ/ไม่ซ้ำยังทำในเครื่องเช่นเดิม ต้องลองคำสั่ง --execute --max-batches 1 --retries 0 อีกครั้งเพื่อยืนยันผล หากยังผิดพลาด ให้ดูข้อความ Provider ที่เพิ่มเข้ามาแทนการสรุปว่าเป็นปัญหา Key หรือโควตาจาก HTTP 400 เพียงอย่างเดียว

อ้างอิง: [Structured outputs](https://ai.google.dev/gemini-api/docs/structured-output), [generateContent](https://ai.google.dev/api/generate-content), [Rate limits](https://ai.google.dev/gemini-api/docs/rate-limits), [สถานะโมเดล](https://ai.google.dev/gemini-api/docs/deprecations/)

## เมื่อโมเดลเดิมตอบ 503 ซ้ำ

ผู้ใช้แปล batch_002 ผ่านแล้ว แต่ batch_003 ติด 503 high demand สองครั้ง คำแปลที่บันทึกแล้ว 100 รูปยังอยู่ครบ ปรับให้ HTTP 503 รออย่างน้อย 60 วินาทีก่อน retry และไม่บวกระยะ interval ซ้ำอีกชั้น ทดสอบจำลองทั้งหมด 17 กรณีผ่าน

ตรวจ metadata ของ `gemini-3.5-flash-lite` ด้วย Key เดิมผ่าน และพบ generateContent ในรายการความสามารถ โดยไม่ได้สร้างคำแปลจริง Google ระบุว่าโมเดลนี้รองรับข้อความและ structured output การตรวจ metadata ไม่ยืนยันว่า generation จะไม่ติดโควตาหรือ high demand

ลองใช้โมเดลทางเลือกกับชุดที่ค้าง โดยข้าม 100 รูปเดิม:

```powershell
.\.venv\Scripts\python.exe -u tools/translate_captions.py --run results/florence_20261022_222031_582707 --model gemini-3.5-flash-lite --execute --retries 1 --interval 30
```

รันครั้งเดียวส่งทีละชุดจนครบหรือพบข้อผิดพลาดที่ retry แล้วไม่ผ่าน จะบันทึกโมเดลที่ใช้ใน metadata ของแต่ละชุด ไม่ได้เปลี่ยนโมเดลเริ่มต้นของแอป Go และไม่ได้อ้างว่าโควตาหรือราคาเหมือนโมเดลเดิม ดู [เอกสารโมเดลทางเลือก](https://ai.google.dev/gemini-api/docs/models/gemini-3.5-flash-lite)
