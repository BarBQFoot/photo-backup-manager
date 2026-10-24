// Run the real composable against controlled asynchronous bridge responses.
// No user files, database, network, AI model or API calls are used.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { pathToFileURL } = require('node:url');
const root = path.resolve(__dirname, '..');
const output = path.join(root, 'frontend/.nuxt/background-analysis-test.mts');
const deferred = () => { let resolve, reject; const promise = new Promise((a,b) => { resolve=a; reject=b }); return {promise,resolve,reject}; };
const photo = (id, folder='A') => ({fileId:id,fileName:`${id}.jpg`,path:`${folder}/${id}.jpg`,fileStatus:'available',aiStatus:'unanalyzed',description:''});
const tick = () => new Promise(resolve => setImmediate(resolve));
(async () => {
  const source = fs.readFileSync(path.join(root,'frontend/app/composables/usePhotoManager.ts'),'utf8')
    .replace(/import \{ desktop, type DriveFile[^\r\n]*\r?\n/, 'let desktop: any; type DriveFile=any; type BackupJob=any; type BackupResult=any; type Progress=any; export function setDesktop(value: any) { desktop=value }\n');
  fs.mkdirSync(path.dirname(output), {recursive:true}); fs.writeFileSync(output,source);
  const mod = await import(pathToFileURL(output));
  let event, unsubscriptions=0, requests=0, backups=0;
  let job=deferred(), metadata=deferred(), scanOverride;
  const stored=new Map([[1,photo(1)],[2,photo(2)],[3,photo(3)],[4,photo(4,'B')]]);
  const desktop={
    onAIProgress(cb){event=cb;return()=>{event=null;unsubscriptions++}},
    analyzeImages(){requests++;return job.promise},
    metadata(){return metadata.promise},
    async scan(folder){return scanOverride ? scanOverride(folder) : [...stored.values()].filter(f=>f.path.startsWith(folder+'/')).map(f=>({...f}))},
    async history(){return []}, async selectFolder(){return 'B'},
    async backup(req){backups++; for(const f of stored.values())if(req.filePaths.includes(f.path))f.path=`${req.destinationPath}/${f.fileName}`;return {items:[],successCount:req.filePaths.length}},
  };
  mod.setDesktop(desktop);
  const m=mod.usePhotoManager(), s=m.state;
  s.destination='A'; s.files=[photo(1),photo(2),photo(3)]; s.source='A'; s.sourceFiles=[...s.files];
  const active=m.analyzeSelected(['A/1.jpg','A/2.jpg']);
  assert.equal(s.ai.running,true); assert.equal(s.busy,''); assert.equal(requests,1);
  await m.analyzeSelected(['A/3.jpg']); assert.equal(requests,1);
  const detail=m.showDetail(s.files[0]);
  stored.get(1).description='คำบรรยายไทย';stored.get(1).aiStatus='analyzed';
  event({type:'result',file_id:'file_1',caption:'คำบรรยายไทย'});
  metadata.resolve({description:'old',aiStatus:'analyzing',status:'active'}); await detail;
  assert.equal(s.detail.description,'คำบรรยายไทย'); assert.equal(s.sourceFiles[0].description,'คำบรรยายไทย');
  assert.equal(s.ai.pendingIds.has(1),false); assert.equal(s.ai.pendingIds.has(2),true);
  // Foreground move finishes while the analysis promise is still unresolved.
  s.destination='B';s.selected=['A/1.jpg'];await m.move();
  assert.equal(backups,1);assert.equal(s.ai.running,true);assert.equal(s.files.find(f=>f.fileId===1).description,'คำบรรยายไทย');
  // Viewing a different folder must not inject earlier files when the job completes.
  s.destination='C';s.files=[photo(9,'C')];s.notice='foreground completed';
  s.busy='foreground operation';
  job.resolve({results:[{fileId:1,description:'คำบรรยายไทย'}],failures:[{fileId:2,message:'test failure'}]});await active;
  assert.equal(s.files.length,1);assert.equal(s.files[0].fileId,9);assert.equal(s.busy,'foreground operation');
  assert.equal(s.notice,'foreground completed');assert.equal(s.ai.running,false);assert.equal(s.ai.pendingIds.size,0);assert.equal(unsubscriptions,1);

  // Older metadata must not replace a newer selection.
  s.busy=''; metadata=deferred(); const first=m.showDetail(photo(4,'B'));
  const old=metadata;metadata=deferred();const second=m.showDetail(photo(5,'B'));
  metadata.resolve({description:'new selection',aiStatus:'analyzed',status:'active'});await second;
  old.resolve({description:'stale',aiStatus:'analyzed',status:'active'});await first;
  assert.equal(s.detail.fileId,5);assert.equal(s.detail.description,'new selection');

  // A scan snapshot racing an AI event must retain the newer caption.
  job=deferred();s.destination='A';s.files=[photo(2)];s.sourceFiles=[photo(2)];
  const next=m.analyzeSelected(['A/2.jpg']);const slowScan=deferred();scanOverride=()=>slowScan.promise;
  const refreshing=m.refresh();await tick();
  event({type:'result',file_id:'file_2',caption:'new caption'});
  slowScan.resolve([{...photo(2),aiStatus:'analyzing'}]);await refreshing;
  assert.equal(s.files[0].description,'new caption');assert.equal(s.files[0].aiStatus,'analyzed');
  scanOverride=null;stored.get(2).description='new caption';stored.get(2).aiStatus='analyzed';
  job.resolve({results:[{fileId:2,description:'new caption'}],failures:[]});await next;

  // A final reconciliation scan cannot replace a newly opened folder.
  job=deferred();s.destination='A';s.files=[photo(2)];
  const final=m.analyzeSelected(['A/2.jpg']);const staleScan=deferred();scanOverride=f=>f==='A'?staleScan.promise:Promise.resolve([photo(4,'B')]);
  job.resolve({results:[{fileId:2,description:'done'}],failures:[]});await tick();
  await m.choose('destination');assert.equal(s.destination,'B');
  staleScan.resolve([photo(2)]);await final;
  assert.equal(s.files.length,1);assert.equal(s.files[0].fileId,4);

  // Bridge failure releases pending rows and leaves unrelated UI feedback intact.
  scanOverride=null;job=deferred();s.destination='A';s.files=[photo(2)];s.notice='kept';
  const failed=m.analyzeSelected(['A/2.jpg']);s.busy='foreground';job.reject(new Error('bridge failed'));await failed;
  assert.equal(s.ai.error,'bridge failed');assert.equal(s.ai.running,false);assert.equal(s.ai.pendingIds.size,0);
  assert.equal(s.files[0].aiStatus,'failed');assert.equal(s.busy,'foreground');assert.equal(s.notice,'kept');
  assert.equal(unsubscriptions,4);
  console.log('PASS: background analysis, streaming detail, parallel move, folder/scan races, stale metadata and failure cleanup');
})().catch(error=>{console.error(error);process.exitCode=1}).finally(()=>{if(fs.existsSync(output))fs.unlinkSync(output)});
