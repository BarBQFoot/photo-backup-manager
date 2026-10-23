// Compile the actual gallery with Vue and exercise an in-memory host.
// No native DOM, photo access, DB writes, or Gemini requests.
const fs = require('node:fs');
const path = require('node:path');
const {createRequire} = require('node:module');
const {pathToFileURL} = require('node:url');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '..');
const localRequire = createRequire(path.join(root, 'frontend/package.json'));
const {parse, compileScript} = localRequire('@vue/compiler-sfc');
const {createRenderer, reactive, nextTick} = localRequire('vue');
const makeNode = (tag='',text='')=>({tagName:tag.toUpperCase(),text,children:[],props:{},nodeType:tag?1:3,style:{},addEventListener(){},removeEventListener(){},getRootNode(){return {activeElement:null}}});
const insert=(child,parent,anchor=null)=>{if(child.parent){const i=child.parent.children.indexOf(child);if(i>=0)child.parent.children.splice(i,1)}child.parent=parent;const i=parent.children.indexOf(anchor);parent.children.splice(i<0?parent.children.length:i,0,child)};
const renderer=createRenderer({createElement:tag=>makeNode(tag),createText:text=>makeNode('',text),createComment:text=>makeNode('',text),setText:(n,text)=>n.text=text,setElementText:(n,text)=>{n.text=text;n.children=[]},insert,remove:n=>{if(n.parent){const i=n.parent.children.indexOf(n);if(i>=0)n.parent.children.splice(i,1)}},parentNode:n=>n.parent,nextSibling:n=>n.parent?.children[n.parent.children.indexOf(n)+1]||null,patchProp:(n,key,old,value)=>{n.props[key]=value;n[key]=value},setScopeId(){},insertStaticContent:(content,parent,anchor)=>{const n=makeNode('static',content);insert(n,parent,anchor);return[n,n]}});
function walk(n){return [n,...n.children.flatMap(walk)]}
async function measure(source,name){
 source=source.replace(/import \{ useSharedPhotoManager \}[^\r\n]*\r?\n/,'function useSharedPhotoManager(){return fixtureManager}\n')
 .replace(/import type \{ DriveFile \}[^\r\n]*\r?\n/,'')
 .replace("from '../utils/captionSearch'", "from '../app/utils/captionSearch.ts'")
 .replace(/import \{ aiLabel, formatBytes, formatDate \}[^\r\n]*\r?\n/,'const aiLabel=String,formatBytes=String,formatDate=String\n')
 .replace('setup lang="ts"','setup').replace('ref<string[]>([])','ref(fixtureManager.initialSelected)').replace('(file: DriveFile)','(file)')
 // Search is driven through its reactive query in this host, not native input events.
 .replace('v-model="query"', ':value="query"')
 .replace('</script>','fixtureManager.setSelected=value=>{selectedPaths.value=value};fixtureManager.setQuery=value=>{query.value=value};\n</script>');
 const {descriptor}=parse(source);
 const compiled=compileScript(descriptor,{id:'gallery-profile',inlineTemplate:true}).content;
 const output=path.join(root,'frontend/.nuxt',`gallery-profile-${name}.mjs`);
 fs.mkdirSync(path.dirname(output),{recursive:true});fs.writeFileSync(output,'let fixtureManager;export function setManager(m){fixtureManager=m}\n'+compiled);
 const module=await import(pathToFileURL(output));
 const files=Array.from({length:1000},(_,i)=>({fileId:i+1,fileName:`photo_${i}.jpg`,path:`C:/source/${i}.jpg`,fileStatus:'available',aiStatus:'analyzing',sizeBytes:1000}));
 const state=reactive({destination:'C:/source',destinationScanned:true,files,busy:'กำลังวิเคราะห์',detail:null});
 const manager={state,initialSelected:files.map(f=>f.path)};module.setManager(manager);
 const app=renderer.createApp(module.default);app.config.warnHandler=()=>{};for(const tag of ['AppIcon','PhotoPreview'])app.component(tag,{render:()=>null});
 const host=makeNode('root');app.mount(host);await nextTick();
 const checkboxes=()=>walk(host).filter(n=>n.tagName==='INPUT'&&n.type==='checkbox');
 assert.equal(checkboxes().length,1000);assert.ok(checkboxes().every(n=>n.checked));
 const start=performance.now();
 for(let i=0;i<20;i++){state.busy=`กำลังวิเคราะห์: อ่านภาพ ${i}/1000 รูป`;await nextTick()}
 const ms=performance.now()-start;
 // A saved result must update its tile even though the other tiles are memoized.
 state.files[0]={...state.files[0],aiStatus:'analyzed',description:'คำบรรยายไทย'};await nextTick();
 assert.ok(walk(host).some(n=>n.text==='analyzed'));
 state.busy='';await nextTick();assert.equal(checkboxes()[0].disabled,false);assert.equal(checkboxes()[1].disabled,true);
 manager.setSelected(files.slice(1).map(f=>f.path));await nextTick();assert.equal(checkboxes()[0].checked,false);assert.equal(checkboxes()[1].checked,true);
 manager.setQuery('คำบรรยายไทย');await nextTick();assert.equal(checkboxes().length,1);
 state.files[0].fileStatus='missing';await nextTick();assert.equal(checkboxes().length,0);
 state.files[0].fileStatus='available';state.files[0].description='สุนัขนั่งบนโซฟา';
 manager.setQuery('หมา');await nextTick();assert.equal(checkboxes().length,1);
 manager.setQuery('สุนัช');await nextTick();assert.equal(checkboxes().length,1);
 state.files[0].description='แมวนั่งบนโซฟา';await nextTick();assert.equal(checkboxes().length,0);
 app.unmount();fs.unlinkSync(output);
 return {variant:name,rows:1000,progress_updates:20,vue_render_ms:Math.round(ms),checks:'selection, saved status, busy controls, exact/synonym/typo search, caption edits and missing filter passed'};
}
(async()=>{
 const current=fs.readFileSync(path.join(root,'frontend/app/pages/gallery.vue'),'utf8');
 const original=current.replace(/\s+v-memo="[^"]+"/,'').replace(/const selectedSet = computed\([^\r\n]+\r?\n/,'').replaceAll('selectedSet.value.has(file.path)','selectedPaths.value.includes(file.path)').replaceAll('selectedSet.has(file.path)','selectedPaths.includes(file.path)');
 const results=[await measure(original,'original'),await measure(current,'optimized')];
 const report={mode:'actual gallery SFC compiled with Vue; in-memory host; excludes native DOM',results};
 fs.mkdirSync(path.join(root,'results/gallery-profile'),{recursive:true});fs.writeFileSync(path.join(root,'results/gallery-profile/render.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));
})().catch(e=>{console.error(e);process.exitCode=1});
