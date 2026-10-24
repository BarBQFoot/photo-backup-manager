// Compile the real startup screen with Vue and exercise it without native UI or downloads.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { createRequire } = require('node:module');
const { pathToFileURL } = require('node:url');
const root = path.resolve(__dirname, '..');
const localRequire = createRequire(path.join(root, 'frontend/package.json'));
const { parse, compileScript, compileStyle } = localRequire('@vue/compiler-sfc');
const { createRenderer, nextTick } = localRequire('vue');
const makeNode = (tag = '', text = '') => ({ tag, text, children: [], props: {} });
const insert = (child, parent, anchor = null) => {
  if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1);
  child.parent = parent;
  const at = parent.children.indexOf(anchor);
  parent.children.splice(at < 0 ? parent.children.length : at, 0, child);
};
const renderer = createRenderer({
  createElement: tag => makeNode(tag), createText: text => makeNode('', text), createComment: () => makeNode(),
  setText: (n, text) => n.text = text, setElementText: (n, text) => { n.text = text; n.children = []; },
  insert, remove: n => { if (n.parent) n.parent.children.splice(n.parent.children.indexOf(n), 1); },
  parentNode: n => n.parent, nextSibling: n => n.parent?.children[n.parent.children.indexOf(n) + 1] || null,
  patchProp: (n, key, old, value) => n.props[key] = value,
  setScopeId: (n, id) => n.props[id] = '',
  insertStaticContent: (content, parent, anchor) => { const n = makeNode('span', content); insert(n, parent, anchor); return [n, n]; },
});
const walk = n => [n, ...n.children.flatMap(walk)];
const textOf = n => n.text + n.children.map(textOf).join(' ');
const escape = value => String(value).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('"', '&quot;');
function html(n) {
  if (!n.tag) return escape(n.text);
  const attributes = Object.entries(n.props).filter(([key, value]) => !key.startsWith('on') && value !== undefined && value !== false)
    .map(([key, value]) => `${key}="${escape(value)}"`).join(' ');
  return `<${n.tag} ${attributes}>${escape(n.text)}${n.children.map(html).join('')}</${n.tag}>`;
}
(async () => {
  let source = fs.readFileSync(path.join(root, 'frontend/app/components/AISetupScreen.vue'), 'utf8')
    .replace(/import \{ desktop, type AISetupStatus \}[^\r\n]*\r?\n/, 'const desktop = fixture.desktop\n')
    .replace(/import \{ errorText \}[^\r\n]*\r?\n/, 'const errorText = String\n')
    .replace('</script>', 'fixture.setStatus = value => { status.value = value };\n</script>');
  const { descriptor } = parse(source);
  const compiled = compileScript(descriptor, { id: 'ai-setup-test', inlineTemplate: true }).content;
  const output = path.join(root, 'frontend/.nuxt/setup-ui-test.mts');
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, 'let fixture; export function setFixture(value) { fixture = value }\n' + compiled);
  const module = await import(pathToFileURL(output));
  module.default.__scopeId = 'data-v-ai-setup-test';
  const initial = { status: 'running', phase: 'checking', detail: 'กำลังตรวจสอบ', completed: 0, total: 0, unit: '', error: '', device: '', skipped: [] };
  let starts = 0, entered = false;
  const fixture = { desktop: { startAISetup: async () => { starts++; return { ...initial }; }, aiSetupStatus: async () => ({ ...initial }) } };
  module.setFixture(fixture);
  const app = renderer.createApp(module.default, { onContinue: () => entered = true });
  app.config.warnHandler = () => {};
  app.component('AppIcon', { render: () => null });
  const host = makeNode('div');
  app.mount(host);
  await new Promise(setImmediate);
  await nextTick();
  assert.equal(starts, 1);
  assert.equal(walk(host).find(n => n.tag === 'progress').props.value, undefined, 'unknown total must be indeterminate');
  const previews = [];
  fixture.setStatus({ ...initial, phase: 'model', detail: 'Downloading Florence weights', completed: 25 * 1048576, total: 100 * 1048576, unit: 'bytes', skipped: ['python', 'pytorch'] });
  await nextTick();
  assert.equal(walk(host).find(n => n.tag === 'progress').props.value, 25);
  assert.ok(textOf(host).includes('25.0 / 100.0 MB'));
  assert.ok(textOf(host).includes('มีอยู่แล้ว · ข้าม'));
  previews.push(html(host));
  fixture.setStatus({ ...initial, status: 'error', phase: 'model', error: 'ดาวน์โหลดสะดุด กรุณาลองใหม่' });
  await nextTick();
  assert.ok(textOf(host).includes('ดาวน์โหลดสะดุด'));
  previews.push(html(host));
  walk(host).find(n => n.tag === 'button' && n.text === 'ลองอีกครั้ง').props.onClick();
  await new Promise(setImmediate);
  await nextTick();
  assert.equal(starts, 2);
  fixture.setStatus({ ...initial, status: 'ready', phase: 'ready', device: 'cuda', skipped: ['python', 'pytorch', 'model', 'verify'] });
  await nextTick();
  assert.ok(textOf(host).includes('CUDA'));
  assert.equal(walk(host).filter(n => n.tag === 'progress').length, 0);
  previews.push(html(host));
  fixture.setStatus({ ...initial, status: 'ready', phase: 'ready', device: 'mps' });
  await nextTick();
  assert.ok(textOf(host).includes('MPS'));
  walk(host).find(n => n.tag === 'button' && n.text === 'เริ่มใช้งาน').props.onClick();
  assert.equal(entered, true);
  app.unmount();
  fs.unlinkSync(output);
  const css = compileStyle({ source: descriptor.styles[0].content, id: 'data-v-ai-setup-test', scoped: true }).code;
  const dist = path.join(root, 'frontend/dist/_nuxt');
  const entry = fs.readdirSync(dist).find(name => name.startsWith('entry.') && name.endsWith('.css'));
  const globalCSS = fs.readFileSync(path.join(dist, entry), 'utf8');
  fs.mkdirSync(path.join(root, 'results/setup-ui'), { recursive: true });
  fs.writeFileSync(path.join(root, 'results/setup-ui/preview.html'), `<!doctype html><html lang="th"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Startup UI states</title><style>${globalCSS}\n${css}</style>${previews.join('')}</html>`);
  console.log('PASS: indeterminate progress, MB/percentage, skipped components, error/retry, ready/device and enter-app actions. No API calls or downloads.');
})().catch(error => { console.error(error); process.exitCode = 1; });
