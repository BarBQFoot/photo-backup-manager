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
