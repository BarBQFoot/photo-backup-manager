"""Validate a translated run and build an offline visual review page."""

import argparse
import json
import os
from pathlib import Path
import random
from urllib.parse import quote

try:
    from .translate_captions import read_catalog, existing_response
except ImportError:
    from translate_captions import read_catalog, existing_response


def prepare(run, sample_size=50):
    source, batches = read_catalog(run)
    accepted = {}
    for name, batch in batches:
        response, _ = existing_response(run, name, batch)
        if response is None:
            raise ValueError(f"Missing validated translation: {name}")
        accepted.update({row['file_id']: row['caption'] for row in response})
    merged = json.loads((run / 'captions_translated.json').read_text(encoding='utf-8-sig'))
    if len(merged) != len(source) or len({r['file_id'] for r in merged}) != len(source):
        raise ValueError('Merged IDs are missing or duplicated')
    records = []
    for original, row in zip(source, merged):
        fid = original['file_id']
        if (row['file_id'] != fid or row['caption_en'] != original['caption_en']
                or row['caption'] != accepted[fid]
                or row['translation_status'] != 'translated'
                or not isinstance(row['caption'], str) or not row['caption'].strip()):
            raise ValueError(f'Merged translation mismatch or incomplete: {fid}')
        photo = json.loads((run / 'photos' / f'{fid}.json').read_text(encoding='utf-8'))
        path = Path(photo['path'])
        if photo['file_id'] != fid or photo['caption_en'] != original['caption_en'] or not path.is_file():
            raise ValueError(f'Photo evidence mismatch or missing image: {fid}')
        records.append(dict(row, image=quote(Path(os.path.relpath(path, run)).as_posix(), safe='/')))
    sample = random.Random(20261022).sample([r['file_id'] for r in records], min(sample_size, len(records)))
    data = json.dumps(dict(records=records, sample=sample, run=run.name), ensure_ascii=False).replace('<', '\\u003c')
    page = HTML.replace('__DATA__', data)
    (run / 'caption_review.html').write_text(page, encoding='utf-8')
    report = dict(total=len(records), validated_translations=len(accepted), image_files_found=len(records),
                  sample_size=len(sample), sample_seed=20261022, sample_ids=sample,
                  visual_accuracy='Not assessed; awaiting human review')
    (run / 'caption_review_verification.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
    return report


HTML = '''<!doctype html>
<html lang="th"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>ตรวจคำบรรยายภาพ</title>
<style>
body{font:17px system-ui,sans-serif;background:#f3f5fa;color:#1e293b;max-width:1100px;margin:auto;padding:24px}
button,select,textarea{font:inherit;padding:9px;border:1px solid #b7c3d4;border-radius:8px}button{cursor:pointer;background:white}
nav,.actions{display:flex;gap:10px;flex-wrap:wrap;margin:16px 0}main{display:grid;grid-template-columns:1.2fr 1fr;gap:24px;background:white;padding:20px;border-radius:16px}
img{width:100%;max-height:620px;object-fit:contain;background:#edf0f5}textarea{width:100%;box-sizing:border-box;min-height:100px}label{display:block;margin-top:12px}
small{overflow-wrap:anywhere}#thai{font-size:22px}#english{color:#52637c}#warning{color:#a33122}@media(max-width:750px){main{grid-template-columns:1fr}}
</style>
<h1>ตรวจรูปคู่กับคำบรรยาย</h1>
<p>เริ่มจากชุดสุ่ม 50 รูป ตรวจว่าใจความตรงภาพ และภาษาไทยตรงกับต้นฉบับอังกฤษ แยกข้อผิดพลาดทั้งสองอย่างออกจากกัน</p>
<p id="warning">ผลตรวจเก็บในเบราว์เซอร์เครื่องนี้ กรุณากดดาวน์โหลดผลตรวจเพื่อเก็บไฟล์ หากเปิดไฟล์ในเบราว์เซอร์อื่นผลตรวจอาจไม่ตามไปด้วย</p>
<nav><select id="scope"><option value="sample">ชุดสุ่ม</option><option value="all">ทุกรูป</option></select><button id="prev">ก่อนหน้า</button><button id="next">ถัดไป</button><span id="position"></span><button id="export">ดาวน์โหลดผลตรวจ JSON</button></nav>
<p id="stats"></p><main><img id="photo" alt="รูปที่กำลังตรวจ"><section><small id="fileid"></small><p id="thai"></p><p id="english"></p>
<label>คำบรรยายตรงกับภาพหรือไม่<select id="visual"><option value="pending">ยังไม่ตรวจ</option><option value="pass">ตรงภาพ</option><option value="partial">ถูกบางส่วน</option><option value="fail">ผิดใจความหลัก</option></select></label>
<label>คำแปลไทยตรงกับอังกฤษหรือไม่<select id="translation"><option value="pending">ยังไม่ตรวจ</option><option value="pass">ตรงต้นฉบับ</option><option value="fail">แปลผิด</option></select></label>
<label>คำบรรยายไทยที่แก้แล้ว (ถ้ามี)<textarea id="correction"></textarea></label>
<label>หมายเหตุ<textarea id="note"></textarea></label><p>การแก้ในหน้านี้จะอยู่ในไฟล์ผลตรวจที่ดาวน์โหลด ยังไม่แก้ไฟล์ต้นฉบับ</p></section></main>
<script>
const data=__DATA__;
const byId=new Map(data.records.map(r=>[r.file_id,r]));
const key='caption-review-v1:'+data.run;
let reviews={}, index=0;
try{reviews=JSON.parse(localStorage.getItem(key)||'{}')}catch(e){document.querySelector('#warning').textContent='เบราว์เซอร์ไม่อนุญาตให้เก็บผลตรวจ กรุณาดาวน์โหลดผลตรวจก่อนปิดหน้า'}
const el=id=>document.getElementById(id);
const ids=()=>el('scope').value==='sample'?data.sample:data.records.map(r=>r.file_id);
function stats(){const values=ids().map(id=>reviews[id]||{});const checked=values.filter(r=>r.visual&&r.visual!=='pending');const passed=checked.filter(r=>r.visual==='pass').length;el('stats').textContent=`ตรวจภาพแล้ว ${checked.length}/${values.length} รูป · ตรงภาพ ${passed} · ถูกบางส่วน ${checked.filter(r=>r.visual==='partial').length} · ผิดใจความ ${checked.filter(r=>r.visual==='fail').length} · คะแนนตรงภาพในรูปที่ตรวจ ${checked.length?(passed/checked.length*100).toFixed(1)+'%':'ยังไม่มี'} (ไม่ใช่คะแนนทั้งชุด)`}
function show(){const list=ids(),id=list[index],row=byId.get(id),r=reviews[id]||{};el('photo').src=row.image;el('fileid').textContent=id;el('thai').textContent=row.caption;el('english').textContent=row.caption_en;el('position').textContent=`${index+1} / ${list.length}`;for(const field of ['visual','translation'])el(field).value=r[field]||'pending';for(const field of ['correction','note'])el(field).value=r[field]||'';el('prev').disabled=index===0;el('next').disabled=index===list.length-1;stats()}
function save(){const id=ids()[index];reviews[id]=Object.fromEntries(['visual','translation','correction','note'].map(f=>[f,el(f).value]));try{localStorage.setItem(key,JSON.stringify(reviews))}catch(e){el('warning').textContent='บันทึกในเบราว์เซอร์ไม่ได้ กรุณาดาวน์โหลดผลตรวจก่อนปิดหน้า'}stats()}
for(const f of ['visual','translation','correction','note'])el(f).addEventListener('input',save);
el('scope').onchange=()=>{index=0;show()};el('prev').onclick=()=>{index--;show()};el('next').onclick=()=>{index++;show()};
el('export').onclick=()=>{const rows=data.records.filter(r=>reviews[r.file_id]).map(r=>({file_id:r.file_id,caption_en:r.caption_en,caption:r.caption,...reviews[r.file_id]}));const blob=new Blob([JSON.stringify({run:data.run,created_at:new Date().toISOString(),sample_ids:data.sample,reviews:rows},null,2)],{type:'application/json'});const a=document.createElement('a'),url=URL.createObjectURL(blob);a.href=url;a.download=data.run+'_review.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)};
show();
</script></html>'''


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', required=True, type=Path)
    args = parser.parse_args()
    result = prepare(args.run.resolve())
    print(json.dumps({k: v for k, v in result.items() if k != 'sample_ids'}, indent=2))
