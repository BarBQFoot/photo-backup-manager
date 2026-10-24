// Real transfer page + composable, with a controlled desktop bridge and Vue host.
// No filesystem backup operations or API requests are performed.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {createRequire}=require('node:module'),{pathToFileURL}=require('node:url');
const root=path.resolve(__dirname,'../..'),req=createRequire(path.join(root,'frontend/package.json'));
const {parse,compileScript}=req('@vue/compiler-sfc');
const {createRenderer,h,nextTick}=req('vue');
const outputs=['backup-manager-test.mts','backup-page-test.mts'].map(n=>path.join(root,'frontend/.nuxt',n));
const node=(tag='',text='')=>({tag,text,children:[],props:{},listeners:{},addEventListener(k,f){this.listeners[k]=f},removeEventListener(k){delete this.listeners[k]}});
const insert=(n,p,a)=>{n.parent=p;const i=p.children.indexOf(a);p.children.splice(i<0?p.children.length:i,0,n)};
const renderer=createRenderer({createElement:tag=>node(tag),createText:t=>node('',t),createComment:()=>node(),setText:(n,t)=>n.text=t,setElementText:(n,t)=>{n.text=t;n.children=[]},insert,remove:n=>{if(n.parent)n.parent.children.splice(n.parent.children.indexOf(n),1)},parentNode:n=>n.parent,nextSibling:n=>n.parent?.children[n.parent.children.indexOf(n)+1]||null,patchProp:(n,k,o,v)=>{n.props[k]=v;n[k]=v},setScopeId(){},insertStaticContent:(t,p,a)=>{const n=node('',t);insert(n,p,a);return[n,n]}});
const walk=n=>[n,...n.children.flatMap(walk)],text=n=>n.text+n.children.map(text).join(' ');
const photo=(id,aiStatus='unanalyzed')=>({fileId:id,path:`A/${id}.jpg`,fileName:`${id}.jpg`,fileStatus:'available',aiStatus,sizeBytes:10});
(async()=>{
 fs.mkdirSync(path.dirname(outputs[0]),{recursive:true});
 const managerSource=fs.readFileSync(path.join(root,'frontend/app/composables/usePhotoManager.ts'),'utf8').replace(/import \{ desktop, type DriveFile[^\r\n]*\r?\n/,'let desktop: any; type DriveFile=any; type BackupJob=any; type BackupResult=any; type Progress=any; export function setDesktop(value:any){desktop=value}\n');
 fs.writeFileSync(outputs[0],managerSource);const managerModule=await import(pathToFileURL(outputs[0]));
 let request,finish;const response=new Promise(resolve=>{finish=resolve});
 const files=[photo(1),photo(2,'analyzing')];let copied=false;
 managerModule.setDesktop({async scan(folder){return folder==='A'?files.map(f=>({...f})):copied?[{...photo(101),path:'B/1.jpg'}]:[]},async history(){return []},backup(value){request=value;return response}});
 const manager=managerModule.usePhotoManager(),s=manager.state;
 s.source='A';s.destination='B';s.sourceFiles=files.map(f=>({...f}));s.ai.running=true;s.ai.pendingIds.add(2);
 let page=fs.readFileSync(path.join(root,'frontend/app/pages/index.vue'),'utf8').replace(/import \{ useSharedPhotoManager \}[^\r\n]*\r?\n/,'const useSharedPhotoManager=()=>fixture\n').replace(/import \{ formatBytes, formatDuration \}[^\r\n]*\r?\n/,'const formatBytes=String,formatDuration=String\n');
 const {descriptor}=parse(page);fs.writeFileSync(outputs[1],'let fixture;export function setFixture(value:any){fixture=value}\n'+compileScript(descriptor,{id:'backup-test',inlineTemplate:true}).content);
 const mod=await import(pathToFileURL(outputs[1]));mod.setFixture(manager);
 const app=renderer.createApp(mod.default);app.config.warnHandler=()=>{};
 for(const name of ['AppIcon','PhotoPreview'])app.component(name,{render:()=>null});
 app.component('ConfirmDialog',{props:['title','confirmLabel'],emits:['confirm','cancel'],render(){return h('dialog',{},[h('h2',this.title),this.$slots.default?.(),h('button',{onClick:()=>this.$emit('confirm')},this.confirmLabel)])}});
 const host=node('root');app.mount(host);await nextTick();
 const all=()=>walk(host),checkboxes=()=>all().filter(n=>n.tag==='input'&&n.props.type==='checkbox');
 const button=label=>all().find(n=>n.tag==='button'&&text(n).trim().startsWith(label));
 assert.equal(s.backupMode,'move');assert.equal(checkboxes()[1].disabled,true);
 // Drive the actual Vue radio model binding.
 const copy=all().find(n=>n.tag==='input'&&n.props.value==='copy');copy.listeners.change({target:copy});await nextTick();
 assert.equal(s.backupMode,'copy');assert.equal(checkboxes()[1].disabled,false);
 button('เลือกทั้งหมด').props.onClick();await nextTick();assert.equal(s.selected.length,2);
 button('สำรองรูปที่เลือก').props.onClick();await nextTick();
 assert.ok(text(host).includes('เก็บต้นฉบับไว้ที่เดิม'));assert.ok(all().some(n=>n.tag==='dialog'));
 assert.equal(all().find(n=>n.tag==='fieldset').disabled,true);
 button('ยืนยันสำรองรูป').props.onClick();await nextTick();
 assert.equal(request.mode,'copy');assert.deepEqual(request.filePaths,['A/1.jpg','A/2.jpg']);
 assert.equal(s.progress.status,'copying');assert.equal(s.ai.running,true);assert.ok(text(host).includes('กำลังสำรองรูป'));
 copied=true;finish({durationMs:100,successCount:1,skippedCount:1,failedCount:0,items:[{sourcePath:'A/1.jpg',status:'copied'},{sourcePath:'A/2.jpg',status:'skipped'}]});
 for(let i=0;i<5;i++){await new Promise(r=>setImmediate(r));await nextTick()}
 assert.equal(s.busy,'');assert.equal(s.sourceFiles.length,2);assert.equal(s.files[0].path,'B/1.jpg');
 assert.ok(text(host).includes('ผลการสำรองรูป'));assert.ok(text(host).includes('ข้าม: ไฟล์ซ้ำ'));assert.ok(!text(host).includes('ล้มเหลว:'));
 // Changing the next operation must not relabel the previous result as a move.
 const move=all().find(n=>n.tag==='input'&&n.props.value==='move');move.listeners.change({target:move});await nextTick();
 assert.equal(s.backupMode,'move');assert.equal(checkboxes()[1].disabled,true);assert.ok(text(host).includes('ผลการสำรองรูป'));
 button('เลือกทั้งหมด').props.onClick();await nextTick();button('ย้ายรูปที่เลือก').props.onClick();await nextTick();
 assert.ok(text(host).includes('ไฟล์จะถูกนำออกจากโฟลเดอร์ต้นทาง'));
 app.unmount();console.log('PASS: copy/move radio binding, AI-pending copy selection, confirmation, payload, progress, duplicate results, preserved source and stable result labels');
})().catch(e=>{console.error(e);process.exitCode=1}).finally(()=>outputs.forEach(p=>{if(fs.existsSync(p))fs.unlinkSync(p)}));
