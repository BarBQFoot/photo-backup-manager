import Fuse from 'fuse.js'
import synonymGroups from '../data/searchSynonyms.json' with { type: 'json' }

type SearchablePhoto = { description?: string | null; fileStatus: string }
export type MatchKind = 'exact' | 'synonym' | 'fuzzy'
type Alternative = { text: string; kind: MatchKind; cost: number }
type Document<T> = { item: T; text: string; boundaries: Set<number> }
export type CaptionMatch<T> = { item: T; kind: MatchKind; score: number }

const normalize = (text: string) => text.normalize('NFC').toLocaleLowerCase('th')
  .replace(/[\u200B\u200C\u200D\uFEFF]/g, '').replace(/\s+/g, ' ').trim()
const segmenter = typeof Intl.Segmenter === 'function'
  ? new Intl.Segmenter('th', { granularity: 'word' }) : null
const stopWords = new Set(['รูป', 'ภาพ', 'ที่', 'อยู่', 'กำลัง', 'บน', 'ใน', 'ของ', 'กับ', 'และ', 'มี', 'พร้อม'])
const kindRank: Record<MatchKind, number> = { exact: 0, synonym: 1, fuzzy: 2 }
const tokenCache = new WeakMap<SearchablePhoto, { description: string; text: string;
  boundaries: Set<number>; terms: string[] }>()
const synonyms = new Map<string, Set<string>>()
const parent = new Map<string, string>()
function root(word: string): string {
  const previous = parent.get(word)
  if (!previous) { parent.set(word, word); return word }
  if (previous === word) return word
  const found = root(previous)
  parent.set(word, found)
  return found
}
// Join the reviewed senses and aliases so น้องหมา also reaches สุนัข through หมา.
for (const group of synonymGroups) {
  const words = group.map(normalize).filter(Boolean)
  if (!words.length) continue
  for (const word of words) parent.set(root(word), root(words[0]!))
}
const concepts = new Map<string, Set<string>>()
for (const word of parent.keys()) {
  const key = root(word)
  const aliases = concepts.get(key) ?? new Set<string>()
  aliases.add(word)
  concepts.set(key, aliases)
}
for (const aliases of concepts.values()) for (const word of aliases) synonyms.set(word, aliases)

const aliasesByInitial = new Map<string, string[]>()
for (const word of synonyms.keys()) {
  const choices = aliasesByInitial.get(word[0]!) ?? []
  choices.push(word)
  aliasesByInitial.set(word[0]!, choices)
}
for (const choices of aliasesByInitial.values()) choices.sort((a, b) => b.length - a.length)

function words(text: string): { word: string; start: number; end: number }[] {
  if (segmenter) return Array.from(segmenter.segment(text))
    .filter(part => part.isWordLike)
    .map(part => ({ word: part.segment, start: part.index, end: part.index + part.segment.length }))
  return Array.from(text.matchAll(/[\p{L}\p{M}\p{N}]+/gu))
    .map(part => ({ word: part[0], start: part.index, end: part.index + part[0].length }))
}

function queryWords(text: string) {
  const parts = words(text)
  const ends = new Set(parts.map(part => part.end))
  const terms: string[] = []
  for (let index = 0; index < parts.length; index++) {
    const part = parts[index]!
    // Keep dictionary phrases together in multiword queries, e.g. น้องหมา คาเฟ่.
    const alias = aliasesByInitial.get(text[part.start]!)?.find(word =>
      text.startsWith(word, part.start) && ends.has(part.start + word.length))
    const term = alias ?? part.word
    const end = part.start + term.length
    while (index + 1 < parts.length && parts[index + 1]!.start < end) index++
    if (!stopWords.has(term)) terms.push(term)
  }
  return [...new Set(terms)]
}

function contains<T>(document: Document<T>, term: string) {
  let at = document.text.indexOf(term)
  while (at >= 0) {
    if (!segmenter || (document.boundaries.has(at) && document.boundaries.has(at + term.length))) return true
    at = document.text.indexOf(term, at + 1)
  }
  return false
}

function editDistance(left: string, right: string) {
  const a = Array.from(left), b = Array.from(right)
  let previous = Array.from({ length: b.length + 1 }, (_, i) => i)
  let beforePrevious = previous
  for (let i = 1; i <= a.length; i++) {
    const current = [i]
    for (let j = 1; j <= b.length; j++) {
      current[j] = Math.min(current[j - 1]! + 1, previous[j]! + 1,
        previous[j - 1]! + (a[i - 1] === b[j - 1] ? 0 : 1))
      if (i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) {
        current[j] = Math.min(current[j]!, beforePrevious[j - 2]! + 1)
      }
    }
    beforePrevious = previous
    previous = current
  }
  return previous[b.length]!
}

// Build only when file descriptions/status change, never on each progress event or query.
export function createCaptionSearch<T extends SearchablePhoto>(files: readonly T[]) {
  const vocabulary = new Set(synonyms.keys())
  const documents: Document<T>[] = []
  for (const item of files) {
    if (item.fileStatus !== 'available' || !item.description?.trim()) continue
    let cached = tokenCache.get(item)
    if (!cached || cached.description !== item.description) {
      const text = normalize(item.description)
      const parts = words(text)
      const boundaries = new Set([0, text.length])
      for (const part of parts) {
        boundaries.add(part.start)
        boundaries.add(part.end)
      }
      cached = { description: item.description, text, boundaries, terms: parts.map(part => part.word) }
      tokenCache.set(item, cached)
    }
    cached.terms.forEach(word => vocabulary.add(word))
    documents.push({ item, text: cached.text, boundaries: cached.boundaries })
  }
  const fuse = new Fuse([...vocabulary], { includeScore: true, ignoreLocation: true,
    threshold: 0.34, ignoreFieldNorm: true })

  function alternatives(term: string): Alternative[] {
    const found = new Map<string, Alternative>()
    function add(text: string, kind: MatchKind, cost: number) {
      const previous = found.get(text)
      if (!previous || kindRank[kind] * 10 + cost < kindRank[previous.kind] * 10 + previous.cost) {
        found.set(text, { text, kind, cost })
      }
    }
    add(term, 'exact', 0)
    for (const alias of synonyms.get(term) ?? []) add(alias, 'synonym', 0)
    // Short words are too ambiguous for approximate matching (e.g. หมา / หมอ).
    if (!/\s/.test(term) && term.length >= 4 && term.length <= 32) {
      const maxEdits = term.length >= 8 ? 2 : 1
      for (const match of fuse.search(term, { limit: 8 })) {
        // Fuse can match a word prefix; verify the whole word to avoid จักรยาน → จักรยานยนต์.
        const distance = editDistance(term, match.item)
        if (distance > maxEdits) continue
        const score = distance / term.length
        add(match.item, 'fuzzy', score)
        for (const alias of synonyms.get(match.item) ?? []) add(alias, 'fuzzy', score)
      }
      // Bitap counts a transposition as two edits; handle one adjacent swap separately.
      const chars = Array.from(term)
      for (let i = 0; i < chars.length - 1; i++) {
        const swapped = [...chars]
        ;[swapped[i], swapped[i + 1]] = [swapped[i + 1]!, swapped[i]!]
        const word = swapped.join('')
        if (!vocabulary.has(word)) continue
        add(word, 'fuzzy', 1 / term.length)
        for (const alias of synonyms.get(word) ?? []) add(alias, 'fuzzy', 1 / term.length)
      }
    }
    return [...found.values()].sort((a, b) => kindRank[a.kind] - kindRank[b.kind] || a.cost - b.cost)
  }

  return {
    search(query: string): CaptionMatch<T>[] {
      const term = normalize(query)
      if (!term) return files.map(item => ({ item, kind: 'exact', score: 0 }))
      const terms = queryWords(term)
      const plans = [[alternatives(term)]]
      if (terms.length > 0 && !(terms.length === 1 && terms[0] === term)) {
        plans.push(terms.map(alternatives))
      }
      const matches: CaptionMatch<T>[] = []
      for (const document of documents) {
        let best: CaptionMatch<T> | undefined
        for (const plan of plans) {
          let rank = 0, total = 0, complete = true
          for (const choices of plan) {
            const match = choices.find(choice => contains(document, choice.text))
            if (!match) { complete = false; break }
            rank = Math.max(rank, kindRank[match.kind])
            total += match.cost
          }
          if (!complete) continue
          const score = rank * 10 + total / plan.length + (plan.length > 1 ? 0.01 : 0)
          if (!best || score < best.score) best = { item: document.item,
            kind: (['exact', 'synonym', 'fuzzy'] as const)[rank]!, score }
        }
        if (best) matches.push(best)
      }
      return matches.sort((a, b) => a.score - b.score)
    },
  }
}
