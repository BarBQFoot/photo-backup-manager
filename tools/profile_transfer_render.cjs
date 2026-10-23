// CPU-only Vue render probe: compile the actual transfer page, use an in-memory host.
// This measures rendering work, not native DOM, disk or real file movement.
const fs = require('node:fs');
const path = require('node:path');
const {createRequire} = require('node:module');
const {pathToFileURL} = require('node:url');
const root = path.resolve(__dirname, '..');
const localRequire = createRequire(path.join(root, 'frontend/package.json'));
const {parse, compileScript} = localRequire('@vue/compiler-sfc');
const {createRenderer, reactive, nextTick} = localRequire('vue');
const node = (tag='', text='') => ({tagName:tag.toUpperCase(), text, children:[], props:{}, nodeType:tag?1:3, style:{}, addEventListener(){}, removeEventListener(){}});
const insert = (child,parent,anchor=null) => { if(child.parent) {const i=child.parent.children.indexOf(child);if(i>=0)child.parent.children.splice(i,1)} child.parent=parent;const i=parent.children.indexOf(anchor);parent.children.splice(i<0?parent.children.length:i,0,child) };
const renderer = createRenderer({
  createElement: tag=>node(tag), createText: text=>node('',text),createComment:text=>node('',text),
  setText:(n,text)=>n.text=text,setElementText:(n,text)=>{n.text=text;n.children=[]},
  insert,remove:n=>{if(n.parent){const i=n.parent.children.indexOf(n);if(i>=0)n.parent.children.splice(i,1)}},
  parentNode:n=>n.parent,nextSibling:n=>n.parent?.children[n.parent.children.indexOf(n)+1]||null,
  patchProp:(n,key,old,value)=>{n.props[key]=value;n[key]=value},setScopeId(){},
  insertStaticContent:(content,parent,anchor)=>{const n=node('static',content);insert(n,parent,anchor);return[n,n]},
});
const updates = Number(process.argv[2] || 100);
async function measure(source,name) {
  source=source.replace(/import \{ useSharedPhotoManager \}[^\r\n]*\r?\n/,'function useSharedPhotoManager() { return fixtureManager }\n')
    .replace(/import \{ formatBytes, formatDuration \}[^\r\n]*\r?\n/,'const formatBytes=String, formatDuration=String\n').replace('] as const',']').replace('setup lang="ts"','setup');
  const {descriptor}=parse(source);
  const compiled=compileScript(descriptor,{id:'transfer-profile',inlineTemplate:true}).content;
  const output=path.join(root,'frontend/.nuxt',`transfer-profile-${name}.mjs`);
  fs.mkdirSync(path.dirname(output),{recursive:true});
  fs.writeFileSync(output,'let fixtureManager; export function setManager(value) { fixtureManager=value }\n'+compiled);
  const module=await import(pathToFileURL(output));
  const files=Array.from({length:1000},(_,i)=>({fileName:`photo_${i}.jpg`,path:`C:/source/${i}.jpg`,fileStatus:'available',sizeBytes:1000}));
  const state=reactive({source:'C:/source',destination:'D:/dest',sourceFiles:files,selected:files.map(f=>f.path),busy:'',result:null,progress:{completed:0,total:1000}});
  module.setManager({state});
  const app=renderer.createApp(module.default);
  app.config.warnHandler=()=>{};
  for(const name of ['AppIcon','PhotoPreview','ConfirmDialog'])app.component(name,{render:()=>null});
  app.mount(node('root'));
  state.busy='กำลังย้ายรูป';await nextTick();
  const start=performance.now();
  for(let i=1;i<=updates;i++){state.progress={completed:i,total:1000,currentPath:files[(i-1)%1000].path,status:'moving'};await nextTick()}
  const ms=performance.now()-start;
  app.unmount();
  fs.unlinkSync(output);
  return {variant:name,rows:1000,progress_updates:updates,vue_render_ms:Math.round(ms)};
}
(async()=>{
 const original=fs.readFileSync(path.join(root,'frontend/app/pages/index.vue'),'utf8').replace(' v-memo="[state.sourceFiles, state.selected, !!state.busy]"','');
 const memo=original.replace('<div v-else class="file-list">','<div v-else v-memo="[state.sourceFiles, state.selected, !!state.busy]" class="file-list">');
 const results=[await measure(original,'original'),await measure(memo,'memo')];
 const report={mode:'actual transfer SFC compiled with Vue; in-memory host; excludes native DOM',results};
 fs.mkdirSync(path.join(root,'results/transfer-profile'),{recursive:true});
 fs.writeFileSync(path.join(root,'results/transfer-profile/vue_render.json'),JSON.stringify(report,null,2));
 console.log(JSON.stringify(report,null,2));
})().catch(e=>{console.error(e);process.exitCode=1});
