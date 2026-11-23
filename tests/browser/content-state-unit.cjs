// Exercise shared content behavior with the real Vue reactivity/lifecycle and
// controlled out-of-order API responses. No server, accounts or database.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const {createRequire}=require('node:module');
const frontend=createRequire(path.resolve(__dirname,'../../web/frontend/package.json'));
const ts=frontend('typescript'),vue=frontend('vue'),root=path.resolve(__dirname,'../../web/frontend/src');
let checks=0;
function equal(actual,expected,label){assert.deepEqual(actual,expected,label);checks++}
function ok(value,label){assert.ok(value,label);checks++}
function deferred(){let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return{promise,resolve,reject}}
function harness(){
 const calls=[],cache={};let respond=()=>Promise.resolve({});
 const api={
  api:(path,options={})=>{calls.push({path,options});return respond(path,options)},
  body:JSON.stringify,
 };
 function load(name){
  if(cache[name])return cache[name];
  const module={exports:{}};cache[name]=module.exports;
  const code=ts.transpileModule(fs.readFileSync(path.join(root,name+'.ts'),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
  const require=id=>id==='vue'?vue:id==='./api'?api:id==='./i18n'?{locale:'zh-CN',t:value=>value}:id==='./publicSession'?{observePublicSession(){}}:load(id.replace('./',''));
  vm.runInThisContext(`(function(require,module,exports){${code}\n})`)(require,module,module.exports);
  return module.exports;
 }
 const sharedAPI=load('api');
 api.list=sharedAPI.list;api.appendUnique=sharedAPI.appendUnique;api.APIError=sharedAPI.APIError;
 return{calls,load,respond:fn=>respond=fn};
}
function setup(options){const h=harness(),scope=vue.effectScope();const state=scope.run(()=>h.load('useCursorList').useCursorList(options));return{state,scope}}
(async()=>{
 {
  const h=harness(),{imageFileIssue,uploadMedia,releaseUploadPreview}=h.load('mediaUpload');
  const file=new File(['pixels'],'photo.png',{type:'image/png'});
  equal(imageFileIssue(file,6),undefined,'image exactly at size limit is accepted');
  equal(imageFileIssue(file,5),'size','image size uses the page limit');
  equal(imageFileIssue(new File([],'empty.png',{type:'image/png'})),'empty','empty image is rejected');
  equal(imageFileIssue(new File(['x'],'image.svg',{type:'image/svg+xml'})),'type','unsupported image format is rejected');
  for(const type of ['image/jpeg','image/png','image/webp','image/gif'])equal(imageFileIssue(new File(['x'],'image',{type})),undefined,'supported image format '+type);
  h.respond(async()=>({state:'ready',media_id:'image-1'}));
  const controller=new AbortController();
  equal(await uploadMedia(file,{key:'same-file-key',signal:controller.signal}),{state:'ready',media_id:'image-1'},'only confirmed media is returned');
  const call=h.calls[0];
  equal([call.path,call.options.method,call.options.key],['/media','POST','same-file-key'],'upload retains endpoint, method and caller key');
  equal(call.options.signal,controller.signal,'upload retains cancellation signal');
  equal(await call.options.body.get('file').text(),'pixels','multipart retains exact file bytes');
  equal(call.options.body.get('file').name,'photo.png','multipart retains filename');
  for(const response of [null,{}, {media_id:'x'}, {state:'staging',media_id:'x'}, {state:'ready',media_id:''}, {state:'ready',media_id:1}]){
   h.respond(async()=>response);await assert.rejects(()=>uploadMedia(file));checks++;
  }
  const preview={preview:URL.createObjectURL(file)};releaseUploadPreview(preview);releaseUploadPreview(preview);
  equal(preview.preview,'','preview release is idempotent');
 }
 {
  const h=harness(),scope=vue.effectScope(),first=deferred(),uploads=[1,2].map(id=>({id,state:'queued',error:'',file:new File(['bytes-'+id],'image-'+id+'.png',{type:'image/png'}),key:'key-'+id})),attached=[];
  h.respond(async(_path,options)=>{if(options.key==='key-1')return first.promise;return{state:'ready',media_id:'second'}});
  const queue=scope.run(()=>h.load('useMediaUploadQueue').useMediaUploadQueue({next:()=>uploads.find(u=>u.state==='queued'),file:u=>u.file,key:u=>u.key,uploaded:(u,result)=>attached.push([u.id,result.media_id])}));
  const running=queue.run();await queue.run();
  equal(h.calls.length,1,'concurrent queue runs do not duplicate requests');
  equal(uploads.map(u=>u.state),['uploading','queued'],'second file waits for the first');
  first.reject(new Error('lost reply'));await running;
  equal(uploads.map(u=>u.state),['failed','ready'],'failed upload stays retryable while later files proceed');
  equal(attached,[[2,'second']],'failed upload never attaches media');
  uploads[0].state='queued';h.respond(async()=>({state:'ready',media_id:'first'}));await queue.run();
  equal(h.calls.map(c=>c.options.key),['key-1','key-2','key-1'],'manual retry reuses the original file key');
  equal(await h.calls[2].options.body.get('file').text(),'bytes-1','manual retry sends the original file');
  equal(attached,[[2,'second'],[1,'first']],'retry attaches confirmed file exactly once');
  scope.stop();
 }
 {
  const h=harness(),scope=vue.effectScope(),pending=deferred(),uploads=[1,2].map(id=>({id,state:'queued',error:'',file:new File(['x'],'image.png'),key:'key-'+id}));let attached=false;
  h.respond(()=>pending.promise);
  const queue=scope.run(()=>h.load('useMediaUploadQueue').useMediaUploadQueue({next:()=>uploads.find(u=>u.state==='queued'),file:u=>u.file,key:u=>u.key,uploaded:()=>{attached=true}}));
  const running=queue.run(),signal=h.calls[0].options.signal;scope.stop();
  ok(signal.aborted,'disposing a page aborts its active upload');
  pending.resolve({state:'ready',media_id:'late'});await running;
  equal(attached,false,'late success after disposal cannot change attachments');
  equal(h.calls.length,1,'disposing a page prevents queued uploads');
  equal(await queue.send(uploads[1]),false,'disposed queues cannot start a single upload');
 }
 {
  const h=harness(),scope=vue.effectScope(),upload={state:'queued',error:'',file:new File(['x'],'image.png'),key:'original'};let attached=false;
  h.respond(async()=>({state:'ready',media_id:''}));
  const queue=scope.run(()=>h.load('useMediaUploadQueue').useMediaUploadQueue({next:()=>undefined,file:u=>u.file,key:u=>u.key,uploaded:()=>{attached=true}}));
  equal(await queue.send(upload),false,'incomplete success cannot finish single upload');
  equal([upload.state,attached],['failed',false],'incomplete success keeps the file without attaching it');
  scope.stop();
 }
 const {withCursor}=harness().load('pagination');
 equal(withCursor('/me/posts?tag_id=1&tag_id=2&q=a+b&cursor=old','x+y/='),'/me/posts?tag_id=1&tag_id=2&q=a+b&cursor=x%2By%2F%3D','cursor replacement preserves repeated filters and encoding');
 equal(withCursor('/me/posts?cursor=old'),'/me/posts','refresh removes previous cursor');
 {
  const h=harness(),{APIError}=h.load('api'),attempt=h.load('useMutationAttempt').useMutationAttempt();
  const payload={title:'original',chapters:['1','2']};
  const first=attempt.begin(()=>({path:'/series',method:'POST',version:3,payload}));
  payload.title='newer';payload.chapters.reverse();
  equal(JSON.parse(first.body),{title:'original',chapters:['1','2']},'pending command snapshots nested input at the wire boundary');
  equal(attempt.begin(()=>{throw new Error('must not rebuild pending command')}),first,'retry reuses exact pending command without rebuilding');
  ok(Object.isFrozen(first),'pending command cannot be mutated');
  for(const failure of [new TypeError('network failed'),new Error('malformed acknowledgement'),... [408,429,500,503].map(status=>new APIError(status,'FAILURE','unconfirmed'))]){
   attempt.failed(failure);equal(attempt.pending.value.key,first.key,'ambiguous failure preserves original key');
  }
  equal(attempt.pending.value.version,3,'retry preserves optimistic version');
  for(const status of [400,401,403,409,412,413,422]){
   attempt.failed(new APIError(status,'REJECTED','rejected'));equal(attempt.pending.value,null,'definitive rejection releases pending command');
   attempt.begin(()=>({path:'/series',method:'POST',payload}));
  }
  attempt.clear();equal(attempt.pending.value,null,'validated acknowledgement or explicit abandonment clears command');
  const next=attempt.begin(()=>({path:'/series',method:'POST',payload}));ok(next.key!==first.key,'next independent command gets a new key');
  equal(JSON.parse(next.body),payload,'new independent command captures current edits');
  attempt.clear();assert.throws(()=>attempt.begin(()=>({path:'/series',method:'POST',payload:undefined})),TypeError);checks++;
  equal(attempt.pending.value,null,'unserializable command never becomes pending');
 }
 {
  let phase=0;const calls=[],{state,scope}=setup({request:async cursor=>{calls.push(cursor);if(phase===1)throw new Error('retry me');return phase===0?{items:[{id:'1'}],next_cursor:'two',has_more:true}:{items:[{id:'1'},{id:'2'},{id:'2'}],next_cursor:'ignored',has_more:false}}});
  await state.load();phase=1;
  equal(await state.load(true),false,'append failure returns false');
  equal(state.items.value.map(x=>x.id),['1'],'append failure preserves content');
  equal(state.cursor.value,'two','append failure preserves retry cursor');
  equal(state.error.value,'retry me','failure is visible');phase=2;
  await state.load(true);
  equal(calls,['','two','two'],'retry uses same page');
  equal(state.items.value.map(x=>x.id),['1','2'],'append deduplicates IDs');
  equal(state.hasMore.value,false,'explicit end ignores spurious cursor');
  equal(state.error.value,'','successful retry clears error');scope.stop();
 }
 {
  const old=deferred(),fresh=deferred(),signals=[];let n=0,received=[];
  const {state,scope}=setup({request:(_cursor,signal)=>{signals.push(signal);return ++n===1?old.promise:fresh.promise},received:page=>received.push(page.limit)});
  const before=state.load(),after=state.load();
  ok(signals[0].aborted,'replacement aborts previous request');
  old.resolve({items:[{id:'old'}],limit:10});await before;
  equal(state.loading.value,true,'late prior response cannot finish newer loading state');
  fresh.resolve({items:[{id:'new'}],limit:20});await after;
  equal(state.items.value.map(x=>x.id),['new'],'newest filter owns rows');
  equal(received,[20],'stale metadata is ignored');scope.stop();
 }
 {
  let fail=false,calls=[];const {state,scope}=setup({request:async cursor=>{calls.push(cursor);if(fail)throw new Error('new filter failed');return{items:[{id:'first'}],next_cursor:'old-filter'}}});
  await state.load();fail=true;await state.load();
  equal(state.cursor.value,'','failed new filter cannot reuse previous filter cursor');
  await state.load(true);equal(calls,['',''],'append blocked until replacement succeeds');scope.stop();
 }
 {
  const pending=deferred();let signal,received=false;
  const {state,scope}=setup({request:(_cursor,s)=>{signal=s;return pending.promise},received:()=>received=true});
  const reading=state.load();scope.stop();ok(signal.aborted,'unmount aborts request');
  pending.resolve({items:[{id:'late'}]});await reading;
  equal(state.items.value,[],'unmounted list cannot be mutated');equal(received,false,'unmounted metadata callback is skipped');
  equal(await state.load(),false,'disposed list cannot start another read');
 }
 {
  let blocked=false,n=0;const pending=deferred();
  const {state,scope}=setup({disabled:()=>blocked,request:async cursor=>{n++;return cursor?pending.promise:{items:[],next_cursor:'two'}}});
  await state.load();blocked=true;await state.load(true);equal(n,1,'mutations block automatic append');blocked=false;
  const one=state.load(true);await state.load(true);equal(n,2,'only one append may be in flight');
  pending.resolve({items:[],next_cursor:'two'});await one;
  equal(state.hasMore.value,false,'repeated cursor cannot spin auto-loading');scope.stop();
 }
 {
  let invalid=false;const {state,scope}=setup({request:async()=>({items:[{id:'1',valid:!invalid}],next_cursor:'two'}),parse:row=>{if(!row.valid)throw new Error('invalid document');return row}});
  await state.load();invalid=true;await state.load(true);
  equal(state.items.value.length,1,'invalid page cannot partially replace rows');equal(state.cursor.value,'two','invalid page can be retried');scope.stop();
 }
 {
  const h=harness(),{posts,moments}=h.load('contentClient'),signal=new AbortController().signal;
  h.respond(async()=>({id:'9',version:3}));
  await posts.list(new URLSearchParams({scope:'mine',q:'a & b'}),'next/',{signal});
  const request=h.calls[0];equal(new URLSearchParams(request.path.split('?')[1]).get('q'),'a & b','content filters survive request construction');equal(request.options.signal,signal,'caller cancellation reaches transport');
  for(const client of [posts,moments]){
   const payload={text:'draft',expected_draft_version:2};
   await client.create(payload,'stable-create');await client.saveDraft('9',payload,{version:3,key:'stable-save'});
   await client.publish('9',{version:4},payload);await client.withdraw('9',{version:5},payload);await client.remove('9',{version:6});
   const writes=h.calls.slice(-5);
   equal(writes.map(c=>c.options.method),['POST','PUT','POST','DELETE','DELETE'],'shared lifecycle preserves HTTP methods');
   equal(writes.map(c=>c.options.version),[undefined,3,4,5,6],'writes retain optimistic concurrency versions');
   equal(writes.slice(0,2).map(c=>c.options.key),['stable-create','stable-save'],'uncertain create/save keeps caller idempotency key');
   equal(JSON.parse(writes[1].options.body),payload,'article-specific draft version is retained');
   equal(JSON.parse(writes[3].options.body),payload,'withdraw retains expected draft version');
   equal(writes[4].options.body,undefined,'delete does not manufacture a payload');
  }
  await posts.read('9/child');equal(h.calls.at(-1).path,'/studio/posts/9%2Fchild','resource IDs are escaped');
  await posts.checkpoint('9',7,{expected_draft_version:2});await posts.restore('9',8,'revision-1');await posts.recover('9',9);
  equal(h.calls.slice(-3).map(c=>c.path),['/studio/posts/9/checkpoints','/studio/posts/9/restore','/studio/posts/9/recover'],'article-only operations stay explicit');
  equal(moments.restore,undefined,'moments do not acquire article recovery semantics');
  const conflict=new Error('version conflict');h.respond(()=>Promise.reject(conflict));await assert.rejects(posts.saveDraft('9',{}, {version:1}),error=>error===conflict);checks++;
 }
 console.log(JSON.stringify({status:'passed',checks}));
})().catch(error=>{console.error(error);process.exitCode=1});
