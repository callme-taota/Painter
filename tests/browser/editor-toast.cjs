// Real editor UI, with every business mutation intercepted. Run against an
// explicitly disposable loopback app and keep its owner storage state private.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const base=(process.env.PAINTER_QA_URL||process.env.BASE_URL||'').replace(/\/$/,''),state=process.env.OWNER_STATE_FILE;
if(!base||!['127.0.0.1','localhost'].includes(new URL(base).hostname)||!state)throw Error('Explicit loopback PAINTER_QA_URL and isolated OWNER_STATE_FILE required');
const origin=new URL(base).origin,out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/editor-toast');
const report={checks:[],errors:[],blocked:[],screenshots:[],actual_business_writes:0};let browser,scenario='startup';
const original={id:'1',title:'初始文章标题',summary:'初始摘要',markdown:'## 原有章节\n\n必须保留的原始正文。\n',tag_ids:[],cover:{mode:'none',template:'editorial',color:'ink'},date_display:{created:true,updated:true,published:true},version:10,draft_version:20,publication:null};
const restored={...original,title:'历史版本标题',markdown:'## 历史章节\n\n恢复后的正文。\n',version:14,draft_version:24};
const json=(route,data,status=200)=>route.fulfill({status,json:data});
const fail=(route,status,message)=>json(route,{error:{code:status===409?'VERSION_CONFLICT':'SIMULATED_FAILURE',message}},status);
function check(name,passed,detail){assert.ok(passed,scenario+': '+name);report.checks.push({scenario,name,...(detail===undefined?{}:{detail})});console.log('PASS '+scenario+': '+name)}
function gate(){let release;return{promise:new Promise(resolve=>release=resolve),release:()=>release()}}
function ack(route,version=11,draftVersion=21,extra={}){return json(route,{id:'1',version,draft_version:draftVersion,ack_mutation_id:route.request().headers()['idempotency-key'],...extra})}
async function setup(handler=()=>false,width=1440){
 const context=await browser.newContext({storageState:state,viewport:{width,height:width<720?844:960},locale:'zh-CN',reducedMotion:'reduce',colorScheme:width<720?'dark':'light',serviceWorkers:'block',acceptDownloads:true});
 await context.route('**/api/v1/**',async route=>{
  const req=route.request(),u=new URL(req.url()),method=req.method();if(u.origin!==origin)return route.abort();
  if(await handler(route,u,method))return;
  if(method==='GET'&&u.pathname==='/api/v1/session')return json(route,{authenticated:true,user:{id:'1',role:'owner',display_name:'隔离反馈测试'},csrf_token:'editor-toast-mock-csrf'});
  if(method==='GET'&&u.pathname==='/api/v1/studio/posts/1')return json(route,original);
  if(method==='GET'&&u.pathname==='/api/v1/tags')return json(route,{items:[],has_more:false});
  if(method==='GET'&&u.pathname==='/api/v1/studio/posts/1/revisions')return json(route,{items:[{id:'991',title:'历史版本',reason:'checkpoint',created_at:'2026-11-01T00:00:00Z'}],has_more:false});
  if(method==='GET'&&u.pathname==='/api/v1/editor/import-format')return json(route,{example:'{"format":"painter.article","version":1,"title":"示例","blocks":[]}',schema:'{}',prompt:'将给定内容整理为 Painter 文章 JSON。'});
  if(method==='POST'&&u.pathname==='/api/v1/editor/preview')return json(route,{sanitized_html:'<p>规范预览正文。</p>'});
  if(method==='POST'&&u.pathname.includes('/analytics/'))return json(route,{accepted:true});
  if(method==='GET'||method==='HEAD')return route.continue();
  report.blocked.push({scenario,method,path:u.pathname});return fail(route,503,'未授权的业务写入已拦截');
 });
 const page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',error=>report.errors.push({scenario,message:error.message}));
 return{context,page};
}
async function open(page,source=false){
 await page.goto(base+'/studio/posts/1',{waitUntil:'domcontentloaded'});await page.waitForFunction(()=>document.querySelector('.editor-title')?.disabled===false);
 await page.getByRole('textbox',{name:'文档正文',exact:true}).waitFor();
 await page.evaluate(()=>{
  window.__editorToastEvents=[];
  new MutationObserver(records=>{for(const record of records){const text=record.target;if(text instanceof Element&&text.matches('.painter-toast-message')){const item=text.closest('[data-toast-kind]');window.__editorToastEvents.push({kind:item?.getAttribute('data-toast-kind'),message:text.textContent})}}}).observe(document.documentElement,{childList:true,subtree:true});
 });
 if(source){await page.locator('.editor-more>summary').click();await page.getByRole('button',{name:'编辑 Markdown 源码',exact:true}).click();await page.getByRole('textbox',{name:'Markdown 正文',exact:true}).waitFor()}
}
const events=page=>page.evaluate(()=>window.__editorToastEvents);
const messages=async(page,kind)=>(await events(page)).filter(event=>!kind||event.kind===kind);
async function toast(page,text,kind){const item=page.locator('[data-toast-kind'+(kind?'="'+kind+'"':'')+']').filter({hasText:text});await item.waitFor();return item}
async function clearToasts(page){await page.evaluate(()=>{for(const button of document.querySelectorAll('.painter-toast-close'))button.click();window.__editorToastEvents=[]})}
async function shot(page,name){const file=path.join(out,name+'.png');await page.screenshot({path:file,animations:'disabled'});report.screenshots.push(file)}
async function manualAcknowledgement(){
 scenario='manual-ack-and-newer-edits';let puts=[];const entered=gate(),release=gate();
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'&&u.pathname.endsWith('/draft')){puts.push({body:route.request().postDataJSON(),headers:route.request().headers()});if(puts.length===1){entered.release();await release.promise}await ack(route,10+puts.length,20+puts.length);return true}return false});
 try{await open(page);await page.getByLabel('文章标题',{exact:true}).fill('第一份待保存内容');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await entered.promise;
  check('no success before server acknowledgement',(await messages(page,'success')).length===0);
  check('save action is disabled while its request is pending',await page.getByRole('button',{name:'保存草稿',exact:true}).isDisabled());
  await page.getByLabel('文章标题',{exact:true}).fill('请求期间新增的内容');release.release();await toast(page,'此版本已保存，仍有新修改待保存。','info');
  check('old snapshot acknowledgement does not claim newer edits saved',(await messages(page,'success')).length===0&&await page.getByLabel('文章标题',{exact:true}).inputValue()==='请求期间新增的内容');
  await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,'草稿已保存。','success');
  check('explicit save acknowledges current snapshot exactly once',(await messages(page,'success')).length===1&&puts.length===2&&puts[1].body.title==='请求期间新增的内容');
  check('save keeps CAS versions and distinct mutation identity',puts[1].headers['if-match']==='"11"'&&puts[1].body.expected_draft_version===21&&puts[0].headers['idempotency-key']!==puts[1].headers['idempotency-key']);
  check('successful feedback is outside the editor form',await page.locator('.editor-page>.notice').filter({hasText:'草稿已保存。'}).count()===0);
  await shot(page,'editor-manual-saved');
 }finally{release.release();await context.close()}
}
async function failureAndRetry(){
 scenario='manual-network-failure-and-idempotent-retry';const puts=[];
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'){puts.push({body:route.request().postDataJSON(),key:route.request().headers()['idempotency-key']});if(puts.length===1)await route.abort('failed');else await ack(route);return true}return false});
 try{await open(page);await page.getByLabel('文章标题',{exact:true}).fill('网络中断也不能丢失');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,/草稿保存失败/,'error');
  check('network failure never emits success',(await messages(page,'success')).length===0);
  check('failed save keeps input and dirty header',await page.getByLabel('文章标题',{exact:true}).inputValue()==='网络中断也不能丢失'&&(await page.locator('.save-state').innerText()).includes('本地内容已保留'));
  check('ordinary network result is not inserted as a form error',await page.locator('.editor-page>.notice.error').count()===0);
  const local=await page.evaluate(()=>new Promise((resolve,reject)=>{const r=indexedDB.open('painter-next-recovery',1);r.onerror=()=>reject(r.error);r.onsuccess=()=>{const db=r.result,t=db.transaction('drafts'),get=t.objectStore('drafts').get(`painter:${location.origin}:1:1`);get.onsuccess=()=>resolve(get.result);t.oncomplete=()=>db.close()}}));
  check('failed content remains in the browser recovery record',local?.title==='网络中断也不能丢失'&&local.seq>local.ack_seq);
  await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,'草稿已保存。','success');
  check('retry reuses exact snapshot and mutation key',puts.length===2&&puts[0].key===puts[1].key&&JSON.stringify(puts[0].body)===JSON.stringify(puts[1].body));
  check('retry emits only one success',(await messages(page,'success')).length===1);
 }finally{await context.close()}
}
async function autosaveFeedback(){
 scenario='autosave-quiet-failure-dedup-and-recovery';let mode='success',puts=[];
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'){puts.push({body:route.request().postDataJSON(),key:route.request().headers()['idempotency-key']});if(mode==='fail')await fail(route,503,'模拟保存连接中断');else await ack(route,10+puts.length,20+puts.length);return true}return false});
 try{await open(page);await page.getByLabel('文章标题',{exact:true}).fill('安静的自动保存');await page.locator('.save-state.saved').waitFor();
  check('normal autosave only updates header',(await messages(page)).length===0&&(await page.locator('.save-state').innerText()).includes('自动保存'));
  mode='fail';await page.getByLabel('文章标题',{exact:true}).fill('连接中断时的快照');await toast(page,/草稿保存失败/,'error');
  await page.evaluate(()=>dispatchEvent(new Event('online')));await page.waitForFunction(()=>document.querySelector('.save-state')?.textContent?.includes('失败'));
  await page.waitForTimeout(120);check('repeated automatic failure emits one error',(await messages(page,'error')).length===1&&puts.length===3);
  await page.getByLabel('文章标题',{exact:true}).fill('较早快照之后的新修改');mode='success';await page.evaluate(()=>dispatchEvent(new Event('online')));await toast(page,'保存连接已恢复，此版本已同步，仍有新修改待保存。','info');
  check('recovery message identifies the older saved snapshot',(await messages(page,'success')).length===0&&puts[2].key===puts[3].key&&puts[3].body.title==='连接中断时的快照');
  await page.evaluate(()=>dispatchEvent(new Event('online')));await page.locator('.save-state.saved').waitFor();
  check('subsequent autosave stays quiet and saves newer content',(await messages(page,'success')).length===0&&(await messages(page,'info')).length===1&&puts.at(-1).body.title==='较早快照之后的新修改');
 }finally{await context.close()}
}
async function invalidAcknowledgement(){
 scenario='malformed-and-wrong-mutation-ack';let writes=0;
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'){writes++;if(writes===1)await json(route,{});else if(writes===2)await json(route,{version:11,draft_version:21,ack_mutation_id:'different-request'});else await ack(route);return true}return false});
 try{await open(page);await page.getByLabel('文章标题',{exact:true}).fill('必须有正确确认的保存');
  for(let i=0;i<2;i++){await clearToasts(page);await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,/服务器尚未确认这个保存版本/,'error');check((i?'wrong mutation':'incomplete response')+' cannot mark draft saved',(await messages(page,'success')).length===0&&await page.locator('.save-state.saved').count()===0)}
  await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,'草稿已保存。','success');check('correct acknowledged retry recovers normally',writes===3);
 }finally{await context.close()}
}
async function publicationActions(){
 scenario='publish-checkpoint-withdraw';let version=10,dv=20,publishes=0,puts=0,revisionReads=0;const entered=gate(),release=gate();
 const{context,page}=await setup(async(route,u,m)=>{
  if(m==='PUT'){puts++;await ack(route,++version,++dv);return true}
  if(m==='POST'&&u.pathname.endsWith('/publish')){publishes++;entered.release();await release.promise;await ack(route,++version,dv,{revision_id:'201'});return true}
  if(m==='POST'&&u.pathname.endsWith('/checkpoints')){await ack(route,++version,dv,{revision_id:'202'});return true}
  if(m==='DELETE'&&u.pathname.endsWith('/publication')){await ack(route,++version,dv);return true}
  if(m==='GET'&&u.pathname.endsWith('/revisions')){revisionReads++;await fail(route,503,'模拟历史列表读取失败');return true}return false;
 });
 try{await open(page);await page.getByLabel('文章标题',{exact:true}).fill('发布前需先保存的内容');await page.getByRole('button',{name:'发布文章',exact:true}).click();await entered.promise;
  check('publish flush emits no premature draft or publication success',puts===1&&(await messages(page,'success')).length===0);
  check('pending publish prevents duplicate action',await page.getByRole('button',{name:'处理中…',exact:true}).isDisabled());release.release();await toast(page,'文章已发布。后续草稿修改不会改变当前公开版本。','success');
  check('publication acknowledgement emits one success',publishes===1&&(await messages(page,'success')).length===1);await clearToasts(page);
  await page.getByRole('button',{name:'保存历史版本',exact:true}).click();await toast(page,'已创建历史版本，可随时恢复为草稿。','success');await page.waitForTimeout(100);
  check('checkpoint success survives history-list read failure',revisionReads===1&&(await messages(page,'success')).length===1&&(await messages(page,'error')).length===0);
  await page.getByRole('button',{name:'查看历史版本',exact:true}).click();await page.getByRole('button',{name:'重新载入历史版本',exact:true}).waitFor();check('history read failure keeps an inline retry action',true);
  await clearToasts(page);await page.getByRole('button',{name:'撤回',exact:true}).click();await toast(page,'文章已撤回，草稿和历史版本保留。','success');check('withdraw acknowledgement emits one success',(await messages(page,'success')).length===1);
 }finally{release.release();await context.close()}
}
async function failedPublicationActions(){
 for(const action of ['publish','checkpoints','publication']){
  scenario='failed-'+action;let mutations=0;
  const{context,page}=await setup(async(route,u,m)=>{if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1'&&action==='publication'){await json(route,{...original,publication:{revision_id:'200'}});return true}if((m==='POST'||m==='DELETE')&&u.pathname.endsWith('/'+action)){mutations++;await fail(route,503,'模拟操作连接中断');return true}return false});
  try{await open(page);await page.getByRole('button',{name:action==='publish'?'发布文章':action==='checkpoints'?'保存历史版本':'撤回',exact:true}).click();await toast(page,/未完成：模拟操作连接中断/,'error');check('network failure announces no operation success',mutations===1&&(await messages(page,'success')).length===0);check('network result stays out of form blocks',await page.locator('.editor-page>.notice.error').count()===0&&await page.getByLabel('文章标题',{exact:true}).inputValue()===original.title)}finally{await context.close()}
 }
}
async function restoreConfirmation(){
 scenario='restore-followup-read';let restoredOnServer=false,readFails=true,restores=0;
 const{context,page}=await setup(async(route,u,m)=>{
  if(m==='POST'&&u.pathname.endsWith('/restore')){restores++;restoredOnServer=true;await ack(route,14,24);return true}
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1'&&restoredOnServer){if(readFails)await fail(route,503,'模拟恢复后读取失败');else await json(route,restored);return true}return false;
 });
 try{await open(page);await page.getByRole('button',{name:'查看历史版本',exact:true}).click();await page.getByRole('button',{name:'恢复为草稿',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'恢复为草稿',exact:true}).click();await page.getByRole('button',{name:'重新载入当前草稿',exact:true}).waitFor();
  check('restore cannot announce success before authoritative content read',(await messages(page,'success')).length===0&&await page.getByLabel('文章标题',{exact:true}).isDisabled());
  check('restore failure preserves local content and explicit retry',await page.getByLabel('文章标题',{exact:true}).inputValue()===original.title&&(await messages(page,'error')).length===1);
  readFails=false;await page.getByRole('button',{name:'重新载入当前草稿',exact:true}).click();await toast(page,'已载入服务器当前草稿。恢复操作不会自动发布。','success');
  check('read retry acknowledges restored content without another restore',restores===1&&(await messages(page,'success')).length===1&&await page.getByLabel('文章标题',{exact:true}).inputValue()===restored.title);
 }finally{await context.close()}
}
async function conflictControls(){
 scenario='conflict-and-manual-merge';let puts=0,reads=0;
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'){puts++;if(puts===1)await fail(route,409,'模拟版本冲突');else await ack(route,12,22);return true}if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1'){reads++;await json(route,reads===1?original:{...original,title:'另一处服务器修改',version:11,draft_version:21});return true}return false});
 try{await open(page);await page.getByLabel('文章标题',{exact:true}).fill('冲突中保留的本地标题');await page.getByRole('heading',{name:'另一处修改已先保存',exact:true}).waitFor();await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,/请先比较并合并/,'warning');
  check('conflict retains comparison and local inputs instead of hiding in toast',await page.locator('.conflict-panel').isVisible()&&await page.getByLabel('文章标题',{exact:true}).inputValue()==='冲突中保留的本地标题'&&puts===1);
  await page.getByRole('button',{name:'以服务器最新版本为基准，继续合并',exact:true}).click();await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,'合并后的内容已手动保存。','success');check('manual merge success clears conflict only after acknowledgement',puts===2&&await page.locator('.conflict-panel').count()===0);
 }finally{await context.close()}
}
async function importAndExport(){
 scenario='import-apply-and-export-mobile';let previews=0,applyFails=true,puts=0;const entered=gate(),release=gate();
 const imported='## 导入的新内容\n\n导入后仍需保存与发布。';
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'){puts++;await ack(route,10+puts,20+puts);return true}if(m==='POST'&&u.pathname==='/api/v1/editor/import-preview'){previews++;if(previews>1){entered.release();await release.promise;if(applyFails){await fail(route,503,'模拟导入应用中断');return true}}await json(route,{title:'导入标题',summary:'',markdown:previews>1?original.markdown+'\n\n'+imported:imported,block_count:1,card_count:0,sanitized_html:'<h2>导入的新内容</h2><p>导入后仍需保存与发布。</p>'});return true}return false},390);
 try{await open(page);await page.locator('.editor-more>summary').click();await page.locator('[data-open-article-import]').click();const panel=page.locator('[data-import-dialog]');await panel.locator('[data-import-json]').fill(JSON.stringify({format:'painter.article',version:1,title:'导入标题',blocks:[{type:'markdown',markdown:imported}]}));await panel.locator('[data-import-preview]').click();await panel.locator('[data-import-result]').waitFor();await panel.locator('[data-import-apply]').click();await entered.promise;
  check('import applies no document change or success before revalidation',(await messages(page,'success')).length===0&&(await page.getByRole('textbox',{name:'文档正文',exact:true}).innerText()).includes('必须保留的原始正文'));
  release.release();await toast(page,/导入尚未应用/,'error');check('failed import keeps input and retry without a duplicate inline network result',await panel.isVisible()&&(await panel.locator('[data-import-json]').inputValue()).includes('导入的新内容')&&await panel.locator('[data-import-apply-error]').count()===0);
  applyFails=false;await panel.locator('[data-import-apply]').click();await panel.waitFor({state:'detached'});await toast(page,'正文已导入当前草稿，将沿用自动保存。确认内容后再发布。','success');
  check('import success says applied to draft, without claiming server save',(await messages(page,'success')).length===1&&(await page.getByRole('textbox',{name:'文档正文',exact:true}).innerText()).includes('导入后仍需保存与发布'));
  await clearToasts(page);await page.locator('.editor-more>summary').click();const downloading=page.waitForEvent('download');await page.getByRole('button',{name:'导出 Markdown',exact:true}).click();const download=await downloading;await toast(page,'已开始导出 Markdown，请查看浏览器下载。','success');const file=await download.path();check('export starts a download containing the current unsaved document',fs.readFileSync(file,'utf8').includes(imported));await shot(page,'editor-mobile-export');
 }finally{release.release();await context.close()}
}
async function cardOperations(source){
 scenario=source?'source-card-apply':'document-card-apply';let failing=true,puts=0;
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'){puts++;await ack(route,10+puts,20+puts);return true}if(m==='POST'&&u.pathname==='/api/v1/editor/preview'&&route.request().postDataJSON().markdown.includes('painter-card')){if(failing)await fail(route,503,'模拟卡片校验中断');else await json(route,{sanitized_html:'<div class="content-panel content-panel--callout"><p class="content-panel__body">卡片中的说明文字</p></div>'});return true}return false});
 try{await open(page,source);if(source)await page.locator('[data-open-source-cards]').click();else{await page.getByRole('textbox',{name:'文档正文',exact:true}).click();await page.keyboard.press('Control+End');await page.locator('[data-open-cards]').click()}
  const panel=page.locator('[data-card-composer]');await panel.getByRole('combobox',{name:'卡片类型',exact:true}).click();await page.getByRole('option',{name:'提示',exact:true}).click();await panel.locator('[data-card-body]').fill('卡片中的说明文字');await panel.locator('[data-card-apply]').click();await toast(page,'模拟卡片校验中断','error');
  check('failed card validation preserves editable fields and inserts no card',await panel.isVisible()&&await panel.locator('[data-card-body]').inputValue()==='卡片中的说明文字'&&(await messages(page,'success')).length===0);
  failing=false;await panel.locator('[data-card-apply]').click();await panel.waitFor({state:'detached'});await toast(page,'卡片已插入当前草稿，尚未发布。','success');check('card insertion acknowledges local application exactly once',(await messages(page,'success')).length===1);
  const body=await page.getByRole('textbox',{name:source?'Markdown 正文':'文档正文',exact:true}).innerText();check('card application preserves original body and adds intended text',body.includes('必须保留的原始正文')&&body.includes('卡片中的说明文字'));
 }finally{await context.close()}
}
async function compositionAndDownloadFailure(){
 scenario='composition-and-export-failure';let puts=[];
 const{context,page}=await setup(async(route,u,m)=>{if(m==='PUT'){puts.push(route.request().postDataJSON());await ack(route,10+puts.length,20+puts.length);return true}return false});
 try{await open(page);const prose=page.getByRole('textbox',{name:'文档正文',exact:true});await prose.focus();await page.keyboard.press('Control+End');await prose.dispatchEvent('compositionstart');await page.keyboard.insertText('组合输入期间的中文');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await toast(page,'请先完成当前输入，组合结束后会继续保存。','info');
  check('explicit save during IME cannot acknowledge incomplete input',puts.length===0&&(await messages(page,'success')).length===0);
  await prose.dispatchEvent('compositionend');await page.locator('.save-state.saved').waitFor();check('composition completion saves full text quietly',puts.length===1&&puts[0].markdown.includes('组合输入期间的中文')&&(await messages(page,'success')).length===0);
  await clearToasts(page);await page.evaluate(()=>{URL.createObjectURL=()=>{throw new Error('simulated download unavailable')}});await page.locator('.editor-more>summary').click();await page.getByRole('button',{name:'导出 Markdown',exact:true}).click();await toast(page,'Markdown 导出未能开始，请重试。','error');check('failed download never claims exported success',(await messages(page,'success')).length===0&&(await prose.innerText()).includes('组合输入期间的中文'));
 }finally{await context.close()}
}
(async()=>{fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE,args:['--no-sandbox','--disable-dev-shm-usage']});
 try{await manualAcknowledgement();await failureAndRetry();await autosaveFeedback();await invalidAcknowledgement();await publicationActions();await failedPublicationActions();await restoreConfirmation();await conflictControls();await importAndExport();await cardOperations(false);await cardOperations(true);await compositionAndDownloadFailure();check('no unhandled browser errors',report.errors.length===0,report.errors);check('no unmocked business mutations',report.blocked.length===0,report.blocked);report.passed=true}
 catch(error){report.failure=error.stack||String(error);throw error}
 finally{await browser.close();fs.writeFileSync(path.join(out,'editor-toast-report.json'),JSON.stringify(report,null,2)+'\n')}
 console.log(JSON.stringify({checks:report.checks.length,passed:report.passed,actual_business_writes:0}));
})().catch(error=>{console.error(error);process.exitCode=1});
