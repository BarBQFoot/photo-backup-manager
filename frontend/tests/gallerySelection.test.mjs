import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parse, compileScript } from '@vue/compiler-sfc'
import { transform } from 'esbuild'
import { createRenderer, nextTick, reactive } from 'vue'

// Render the actual gallery, including v-memo and checkbox event handlers.
const filename = new URL('../app/pages/gallery.vue', import.meta.url)
const { descriptor } = parse(readFileSync(filename, 'utf8'))
let source = compileScript(descriptor, { id: 'gallery-test', inlineTemplate: true }).content
source = source.replace(/import \{ useSharedPhotoManager \} from '[^']+'/, "import { inject } from 'vue'; const useSharedPhotoManager = () => inject('manager')")
source = source.replace(/import \{ aiLabel, formatBytes, formatDate \} from '[^']+'/, 'const aiLabel = value => value; const formatBytes = value => value; const formatDate = value => value')
source = source.replace("'../utils/captionSearch'", JSON.stringify(new URL('../app/utils/captionSearch.ts', import.meta.url).href))
const { code } = await transform(source, { loader: 'ts', format: 'esm' })
const compiled = code.replace(/from "vue"/g, `from ${JSON.stringify(import.meta.resolve('vue'))}`)
const { default: Gallery } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`)

globalThis.Document ??= class Document {}
globalThis.ShadowRoot ??= class ShadowRoot {}
function element(type, text = '') {
  return { type, text, props: {}, children: [], parent: null, value: '',
    addEventListener() {}, getRootNode: () => ({}) }
}
const renderer = createRenderer({
  createElement: element,
  createText: text => element('#text', text),
  createComment: text => element('#comment', text),
  setText: (node, text) => { node.text = text },
  setElementText: (node, text) => { node.text = text; node.children = [] },
  patchProp: (node, key, previous, value) => { node.props[key] = value },
  insert(node, parent, anchor = null) {
    if (node.parent) node.parent.children.splice(node.parent.children.indexOf(node), 1)
    const index = anchor ? parent.children.indexOf(anchor) : -1
    parent.children.splice(index < 0 ? parent.children.length : index, 0, node)
    node.parent = parent
  },
  remove(node) { node.parent.children.splice(node.parent.children.indexOf(node), 1); node.parent = null },
  parentNode: node => node.parent,
  nextSibling: node => node.parent?.children[node.parent.children.indexOf(node) + 1] ?? null,
})

function findAll(node, predicate) {
  return [...(predicate(node) ? [node] : []), ...node.children.flatMap(child => findAll(child, predicate))]
}

test('memoized checkboxes retain previous choices, including after search and select visible', async () => {
  const state = reactive({ files: [
    { path: '/a', fileId: 1, fileName: 'a', fileStatus: 'available', aiStatus: 'analyzed', description: 'cat' },
    { path: '/b', fileId: 2, fileName: 'b', fileStatus: 'available', aiStatus: 'analyzed', description: 'sea' },
    { path: '/c', fileId: 3, fileName: 'c', fileStatus: 'available', aiStatus: 'analyzed', description: 'sea' },
  ], busy: '', destination: '/photos', detail: null })
  let analyzed
  const app = renderer.createApp(Gallery)
  app.provide('manager', { state, analyzeSelected: paths => { analyzed = paths }, showDetail: file => { state.detail = file } })
  app.component('AppIcon', { render: () => null })
  app.component('PhotoPreview', { render: () => null })
  const root = element('root')
  app.mount(root)
  const boxes = () => findAll(root, node => node.type === 'input' && node.props.type === 'checkbox')
  const change = async (path, checked) => {
    boxes().find(node => node.props.value === path).props.onChange({ target: { checked } })
    await nextTick()
  }
  const checked = () => boxes().filter(node => node.props.checked).map(node => node.props.value)
  const buttons = () => findAll(root, node => node.type === 'button')
  const click = async text => { buttons().find(node => node.text === text).props.onClick(); await nextTick() }
  await change('/a', true)
  await change('/b', true)
  await change('/c', true)
  assert.deepEqual(checked(), ['/a', '/b', '/c'])
  await change('/b', false)
  assert.deepEqual(checked(), ['/a', '/c'])
  buttons().find(node => node.props.class === 'photo-detail-button').props.onClick()
  await nextTick()
  assert.equal(state.detail.path, '/a')
  assert.deepEqual(checked(), ['/a', '/c'], 'opening a preview keeps the batch selection')
  const search = findAll(root, node => node.type === 'input' && node.props.type === 'search')[0]
  search.props['onUpdate:modelValue']('sea')
  await nextTick()
  await click('เลือกทั้งหมดที่แสดง')
  const analyze = buttons().find(node => node.props.class === 'button primary')
  analyze.props.onClick()
  assert.deepEqual(analyzed, ['/a', '/b', '/c'])
  search.props['onUpdate:modelValue']('')
  await nextTick()
  assert.deepEqual(checked(), ['/a', '/b', '/c'])
  await click('ล้างการเลือก')
  assert.deepEqual(checked(), [])
  app.unmount()
})
