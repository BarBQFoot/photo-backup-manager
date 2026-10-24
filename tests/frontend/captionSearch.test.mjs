import test from 'node:test'
import assert from 'node:assert/strict'
import { createCaptionSearch } from '../../frontend/app/utils/captionSearch.ts'

const photo = (id, description, fileStatus = 'available') => ({ id, description, fileStatus })
const ids = matches => matches.map(match => match.item.id)

test('exact spelling ranks above synonyms; expansion works in both directions', () => {
  const search = createCaptionSearch([photo(1, 'สุนัขนั่งบนโซฟา'), photo(2, 'หมานอนบนโซฟา')])
  assert.deepEqual(ids(search.search('หมา')), [2, 1])
  assert.deepEqual(search.search('หมา').map(row => row.kind), ['exact', 'synonym'])
  assert.deepEqual(ids(search.search('สุนัข')), [1, 2])
})

test('handles missing/replaced letters and one adjacent transposition in Thai', () => {
  const search = createCaptionSearch([photo(1, 'คนขี่จักรยานริมทะเล'), photo(2, 'คนนั่งบนมอเตอร์ไซค์')])
  assert.deepEqual(ids(search.search('จักยาน')), [1])
  assert.deepEqual(ids(search.search('ทะลเ')), [1])
  assert.equal(search.search('จักยาน')[0].kind, 'fuzzy')
  assert.deepEqual(ids(createCaptionSearch([photo(1, 'หมานอนบนพื้น')]).search('สุนัช')), [1])
})

test('multiword and unspaced Thai searches require every content word on the same photo', () => {
  const search = createCaptionSearch([photo(1, 'สุนัขนั่งบนโซฟา'), photo(2, 'แมวนั่งบนโซฟา'),
    photo(3, 'สุนัขเดินริมทะเล'), photo(4, 'โซฟาในห้อง')])
  assert.deepEqual(ids(search.search('หมา โซฟา')), [1])
  assert.deepEqual(ids(search.search('หมาบนโซฟา')), [1])
  assert.deepEqual(ids(search.search('หมา ทะเล')), [3])
  assert.deepEqual(ids(search.search('หมา เครื่องบิน')), [])
})

test('short words and unrelated word prefixes do not expand into false positives', () => {
  const search = createCaptionSearch([photo(1, 'หมอยืนข้างโต๊ะ'), photo(2, 'คนเล่นหมากรุก'),
    photo(3, 'ชายหาดริมทะเล'), photo(4, 'ผู้หญิงขี่มอเตอร์ไซค์')])
  assert.deepEqual(ids(search.search('หมา')), [])
  assert.deepEqual(ids(search.search('ผู้ชาย')), [])
  assert.deepEqual(ids(search.search('จักรยาน')), [])
  assert.deepEqual(ids(search.search('จักยาน')), [])
  assert.deepEqual(ids(search.search('รถจักรยานยนต์')), [4])
})

test('empty query preserves gallery order; active search excludes missing/deleted/untranslated photos', () => {
  const files = [photo(1, 'สุนัข'), photo(2, 'สุนัข', 'missing'), photo(3, 'สุนัข', 'deleted'), photo(4, null)]
  const search = createCaptionSearch(files)
  assert.deepEqual(ids(search.search('  ')), [1, 2, 3, 4])
  assert.deepEqual(ids(search.search('หมา')), [1])
})

test('normalizes case, Unicode and whitespace without deleting Thai vowels or tone marks', () => {
  const search = createCaptionSearch([photo(1, 'A café with a laptop'), photo(2, 'คนถือไม้'), photo(3, 'คนถ่ายรูป')])
  assert.deepEqual(ids(search.search('  CAFE\u0301  ')), [1])
  assert.deepEqual(ids(search.search('ไม้')), [2])
  assert.deepEqual(ids(search.search('ไม')), [])
  assert.deepEqual(ids(search.search('สุนัขไม่มีอยู่ที่นี่')), [])
  assert.deepEqual(ids(search.search('!!!')), [])
})

test('rebuilt index reflects a newly saved caption and file status change', () => {
  const files = [photo(1, '')]
  assert.deepEqual(ids(createCaptionSearch(files).search('หมา')), [])
  files[0].description = 'สุนัขนั่งบนโซฟา'
  assert.deepEqual(ids(createCaptionSearch(files).search('หมา')), [1])
  files[0].fileStatus = 'missing'
  assert.deepEqual(ids(createCaptionSearch(files).search('หมา')), [])
})

test('queries do not mutate photo descriptions, order or metadata', () => {
  const files = Object.freeze([Object.freeze(photo(1, 'สุนัข')), Object.freeze(photo(2, 'หมา'))])
  const before = JSON.stringify(files)
  createCaptionSearch(files).search('หมา')
  assert.equal(JSON.stringify(files), before)
})

test('dictionary vocabulary works in both directions for distinct photographic objects', () => {
  const search = createCaptionSearch([
    photo(1, 'เครื่องบินจอดที่ท่าอากาศยาน'), photo(2, 'คนอ่านหนังสือในหอสมุด'),
    photo(3, 'นกเค้าแมวอยู่บนต้นไม้'), photo(4, 'รองเท้าวางบนพื้น'),
  ])
  for (const [query, expected] of [['สนามบิน', [1]], ['ห้องสมุด', [2]], ['นกฮูก', [3]], ['เกือก', [4]]]) {
    assert.deepEqual(ids(search.search(query)), expected, query)
  }
  assert.deepEqual(ids(createCaptionSearch([photo(5, 'นกฮูกอยู่ในป่า')]).search('นกเค้าแมว')), [5])
})

test('colloquial pet names join dictionary aliases and stay grouped in multiword queries', () => {
  const search = createCaptionSearch([
    photo(1, 'สุนัขนอนบนโซฟาในร้านกาแฟ'), photo(2, 'แมวนั่งในร้านกาแฟ'),
    photo(3, 'สุนัขเดินริมทะเล'), photo(4, 'คนเดินในร้านกาแฟ'),
  ])
  assert.deepEqual(ids(search.search('น้อนหมา')), [1, 3])
  assert.deepEqual(ids(search.search('เจ้าตูบ คาเฟ่')), [1])
  assert.deepEqual(ids(search.search('น้องหมา คาเฟ่')), [1])
  assert.deepEqual(ids(search.search('น้อนหมาบนโซฟา')), [1])
  assert.deepEqual(ids(search.search('น้องเหมียว คาเฟ่')), [2])
  assert.deepEqual(ids(createCaptionSearch([photo(5, 'น้องหมานั่งอยู่')]).search('สุนัข')), [5])
})

test('common slang and spelling aliases match only captions containing the corresponding concept', () => {
  const search = createCaptionSearch([
    photo(1, 'คนขี่รถจักรยานยนต์'), photo(2, 'แล็ปท็อปวางบนโต๊ะ'),
    photo(3, 'เด็กกินไอศกรีม'), photo(4, 'คนพักผ่อนริมทะเล'),
    photo(5, 'แมวน่ารักบนเก้าอี้'), photo(6, 'ถ่ายเซลฟี่ในสวน'),
    photo(7, 'ต้นไม้ริมทะเล'),
  ])
  for (const [query, expected] of [
    ['มอไซค์', [1]], ['โน๊ตบุ๊ค', [2]], ['ไอติม', [3]], ['ชิล', [4]],
    ['ตะมุตะมิ', [5]], ['เซลฟี', [6]], ['โน้ตบุค โต๊ะ', [2]],
  ]) assert.deepEqual(ids(search.search(query)), expected, query)
})

test('dictionary senses do not bridge pronouns, calendar dates, food, chess or unrelated sports', () => {
  const search = createCaptionSearch([
    photo(1, 'เขายืนข้างรถยนต์'), photo(2, 'ปฏิทินของเดือนนี้'),
    photo(3, 'หมอถือกระเป๋า'), photo(4, 'อัศวินบนกระดานหมากรุก'),
    photo(5, 'คนเล่นกีฬาสเก็ต'), photo(6, 'คนซื้อเนื้อหมู'), photo(7, 'เครื่องดื่มน้ำนม'),
    photo(8, 'ภาพหน้าอกของคน'),
  ])
  for (const query of ['ภูเขา', 'พระจันทร์', 'น้อนหมา', 'อาชา', 'เบสบอล']) {
    assert.deepEqual(ids(search.search(query)), [], query)
  }
  assert.deepEqual(ids(search.search('หมู')), [6])
  assert.deepEqual(ids(search.search('น้ำนม')), [7])
})

test('expanded everyday vocabulary covers places, transport, food and equipment', () => {
  for (const [query, caption] of [
    ['โกดัง', 'รถจอดหน้าคลังสินค้า'], ['ทางข้ามม้าลาย', 'คนเดินบนทางม้าลาย'],
    ['รถสิบล้อ', 'รถบรรทุกสิบล้อจอดอยู่'], ['วีลแชร์', 'คนใช้รถเข็นคนพิการ'],
    ['อควาเรียม', 'เด็กชมปลาในพิพิธภัณฑ์สัตว์น้ำ'], ['น้องยีราฟ', 'ยีราฟอยู่ในสวนสัตว์'],
    ['กีต้าร์', 'คนเล่นกีตาร์'], ['มอนิเตอร์', 'จอคอมพิวเตอร์อยู่บนโต๊ะ'],
    ['ไดร์เป่าผม', 'คนใช้เครื่องเป่าผม'], ['หมวกกันน็อค', 'คนสวมหมวกนิรภัย'],
    ['แซนวิช', 'แซนด์วิชวางบนจาน'], ['สตรอเบอร์รี่', 'เด็กกินสตรอว์เบอร์รี'],
    ['ครัวซอง', 'ครัวซองต์อยู่บนจาน'], ['จุมพิต', 'คนสองคนจูบกัน'],
    ['ตากล้อง', 'ช่างภาพกำลังถ่ายรูป'], ['งานวิวาห์', 'คนร่วมงานแต่งงาน'],
  ]) {
    assert.deepEqual(ids(createCaptionSearch([photo(1, caption)]).search(query)), [1], query)
  }
})

test('expanded aliases retain distinctions between species, subtypes and unrelated senses', () => {
  for (const [query, caption] of [
    ['กบ', 'คางคกอยู่บนพื้น'], ['ส้ม', 'คนใส่เสื้อสีแสด'],
    ['รถบรรทุกเทท้าย', 'รถบรรทุกสิบล้อ'], ['แกะ', 'คนกำลังสลักไม้'],
    ['น้องแพะ', 'คนถูกใส่ร้าย'], ['กุญแจ', 'คนเล่นเปียโนคีย์สีขาว'],
    ['เรือใบ', 'เรือคายัคลอยอยู่'], ['ตั๊กแตน', 'ผีเสื้อบินอยู่ในสวน'],
  ]) assert.deepEqual(ids(createCaptionSearch([photo(1, caption)]).search(query)), [], query)
})
