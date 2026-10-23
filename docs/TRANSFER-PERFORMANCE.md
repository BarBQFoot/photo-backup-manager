# ตรวจช่วงรอหลังย้าย 1000 รูป วันที่ 23 ตุลาคม 2026

เริ่มจากตรวจอย่างเดียว ต่อมาแก้จุด render/progress ตามรายการด้านล่าง ไม่มีการย้ายรูปจริงของผู้ใช้ซ้ำ

## สิ่งที่ยืนยันได้

- งานล่าสุดใน SQLite: 1000 รูป จาก `test-images/1000` ไป `D:\test\New folder`, status completed, duration_ms 7623
- `StartBackup` เริ่มจับเวลาหลัง validate paths/selection แต่ก่อนสร้าง job และวนย้ายไฟล์ การบันทึก metadata รายรูปอยู่ในช่วงเวลานี้ด้วย หยุดจับเวลาก่อน Finish job
- ดังนั้น 7.623 วินาทีไม่ได้หมายถึงเฉพาะ filesystem move และไม่รวม frontend refresh หลังคืนผล
- frontend `move()` คง busy=กำลังย้ายรูปไว้จน `finally` รอ scan source และ refreshDestination (scan + history) เสร็จ ทำให้ข้อความกำลังย้ายครอบคลุมขั้นอัปเดตรายการด้วย
- Go `setProgress` ส่ง Wails event ทุกไฟล์ ประมาณ 1000 events ต่อรอบ ไม่มีการลดความถี่
- default layout รับ event แล้วเปลี่ยน state.progress หน้า index อ่าน progress ใน render เดียวกับ source list 1000 แถว แต่ละ checkbox ใช้ v-model กับ selected array 1000 ค่า ทำให้ render ซ้ำและตรวจสมาชิกซ้ำทุกแถว

## การวัดแบบไม่แก้ข้อมูลจริง

`results/transfer-profile/read_only_scan.json`: เปิด SQLite mode=ro, เดิน directory ปลายทาง 1000 ไฟล์ 1.85 ms และ stat+lookup metadata ทีละรูป 167.88 ms ใช้ Python sqlite ไม่ใช่การจับเวลาครบทั้ง Go ScanDrive จึงไม่ควรเรียกตัวเลขนี้ว่าเวลาสแกนแอปทั้งหมด

`node tools/profile_transfer_render.cjs 100`: compile source หน้า index จริงด้วย Vue compiler และรันด้วย in-memory renderer, stub รูป preview/ไอคอน ไม่เรียก DB หรือ filesystem moves

| รายการ | อัปเดต progress | โค้ดเดิม | ทดลอง v-memo ครอบ source list |
| --- | --- | --- | --- |
| 1000 แถว | 100 ครั้ง | 41509 ms | 4 ms |
| 1000 แถว | 10 ครั้ง ยืนยันซ้ำ | 4045 ms | 1 ms |

v-memo ที่ทดลองใน source string ของ benchmark คือ dependencies `[state.sourceFiles, state.selected, !!state.busy]` โดยไม่ได้แก้ source ของหน้าแอป

ผลชี้ว่าการ render รายการซ้ำมีต้นทุน CPU สูงมากและอธิบายอาการแถบ progress ตามหลังงานจริงได้ แต่ **ไม่ใช่เวลาที่วัดใน native Wails webview ของรอบผู้ใช้** ไม่ควรอนุมานว่าแอปรอบนั้นรอเท่าตัวเลข benchmark หรือยืนยันว่ามีสาเหตุเดียว การลอง browser fixture ติดปัญหาการเข้าถึง localhost จึงไม่มีผล DOM benchmark และได้หยุดเซิร์ฟเวอร์แล้ว

## จุดแก้ที่ตรงสาเหตุ

1. แยก progress ออกจากรายการรูป หรือใช้ memo/pagination เพื่อไม่ประเมิน 1000 checkbox ซ้ำทุก progress event
2. ลดความถี่ event ที่ส่งไปหน้าจอ เช่น 10 ครั้ง/วินาที แต่เก็บ snapshot backend ทุกไฟล์ และส่งสถานะสุดท้ายเสมอ
3. เปลี่ยนข้อความระหว่าง refresh เป็นกำลังอัปเดตรายการ เพื่อไม่แสดงว่ากำลังย้ายหลังย้ายเสร็จ
4. หากจะรายงานเวลารวม ให้แยก move+metadata, refresh และเวลารวมจน UI พร้อม แทนการใช้ duration_ms ตัวเดียว

## การแก้ที่ทำแล้ว

- ใส่ v-memo ครอบ source list โดยติดตาม sourceFiles, selected และ busy เพื่อให้เปลี่ยนการเลือก/สแกน/สถานะปิดปุ่มได้ตามเดิม แต่ไม่ render รายการซ้ำทุก progress
- Go เก็บ snapshot ทุกไฟล์ แต่ส่ง event ไปหน้าจอห่างอย่างน้อย 100 ms ยกเว้นเริ่มงาน เปลี่ยนสถานะ และไฟล์สุดท้ายที่ต้องส่งทันที
- หลัง backend คืนผล เปลี่ยน busy เป็นกำลังอัปเดตรายการหลังย้าย เพื่อแยกจากช่วงย้ายจริง
- เพิ่มเทสต์ progress ด้วยเวลาแบบกำหนดได้ ตรวจว่าการลด event ไม่ทำให้ snapshot ล่าสุดหรือ completion หาย
- ทดสอบ render ซ้ำหลังแก้กับ 1000 แถว/10 updates: โค้ดเดิมจำลอง 4533 ms, แบบ memo 2 ms ไม่ใช่เวลารวม native app
- ยังไม่ได้เพิ่มเวลารวมใหม่บนหน้าจอ ตัวเลขผลการย้ายยังเป็น backend duration ตามเดิม
