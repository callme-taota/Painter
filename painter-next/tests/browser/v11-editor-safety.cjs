// All business mutations are intercepted. Exercises the formerly destructive paths.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const fs=require('fs'),path=require('path');
const base=(process.env.BASE_URL||'http://192.168.2.209:18081').replace(/\/$/,''),origin=new URL(base).origin;
const out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v11/editor-safety');
const stateFile=process.env.OWNER_STATE_FILE;
if(!stateFile)throw Error('OWNER_STATE_FILE required');
const checks=[],errors=[],blocked=[];let browser,scenario='startup';
const original={id:'1',title:'初始文章标题',summary:'初始摘要',markdown:'## 本地原稿\n\n原有正文段落。\n',tag_ids:[],cover:{mode:'none',template:'editorial',color:'ink'},date_display:{created:true,updated:true,published:true},version:10,draft_version:20,publication:null};
const latest={...original,title:'服务器新标题',markdown:original.markdown+'\n服务器独有的新段落。\n',version:11,draft_version:21};
const restored={...original,title:'恢复后的历史标题',markdown:'## 历史正文\n\n历史版本的正文。\n',version:14,draft_version:24};
const json=(r,v,status=200)=>r.fulfill({status,json:v});
const fail=(r,status,message)=>json(r,{error:{code:status===409?'VERSION_CONFLICT':'SIMULATED_FAILURE',message}},status);
function check(name,passed){checks.push({scenario,check:name,passed:!!passed});if(!passed)throw Error(scenario+': '+name)}
function gate(){let release;return {promise:new Promise(r=>release=r),release:()=>release()}}
async function setup(width=1440,theme='light',handler=()=>false){
 const context=await browser.newContext({storageState:stateFile,viewport:{width,height:width<720?844:1000},colorScheme:theme,serviceWorkers:'block'});
 await context.addInitScript(theme=>localStorage.setItem('painter-theme',theme),theme);
 await context.route('**/api/v1/**',async route=>{
  const r=route.request(),u=new URL(r.url()),m=r.method();if(u.origin!==origin)return route.continue();
  if(await handler(route,u,m))return;
  if(m==='GET'&&u.pathname==='/api/v1/session')return json(route,{authenticated:true,user:{id:'1',role:'owner',display_name:'回归检查'},csrf_token:'v11-mock-csrf'});
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1')return json(route,original);
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1/revisions')return json(route,{items:[{id:'991',title:'历史检查点',reason:'checkpoint',created_at:'2026-11-01T00:00:00Z',renderer_version:'test'}],has_more:false});
  if(m==='POST'&&u.pathname==='/api/v1/editor/preview')return json(route,{sanitized_html:'<h2>规范预览</h2><p>正文内容。</p>'});
  if(m==='POST'&&u.pathname.includes('/analytics/'))return json(route,{accepted:true});
  if(m==='GET'||m==='HEAD')return route.continue();
  blocked.push({scenario,method:m,path:u.pathname});return fail(route,503,'未授权的测试写入已拦截');
 });
 const page=await context.newPage();page.setDefaultTimeout(8000);page.on('pageerror',e=>errors.push({scenario,message:e.message.slice(0,180)}));
 return{context,page};
}
async function open(page,mode='document'){
 await page.goto(base+'/studio/posts/1');await page.getByRole('textbox',{name:'文章标题',exact:true}).waitFor();
 await page.waitForFunction(()=>!document.querySelector('.editor-title')?.disabled);
 if(mode==='source')await page.getByRole('button',{name:'Markdown 源码',exact:true}).click();
 await page.getByRole('textbox',{name:mode==='source'?'Markdown 正文':'文档正文',exact:true}).waitFor();
}
async function info(page){const d=page.locator('.editor-properties');if(!await d.evaluate(e=>e.open))await d.locator('summary').click()}
async function capture(page,name){if(process.env.CAPTURE_SCREENSHOTS!=='1')return;await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:path.join(out,name+'.png'),animations:'disabled'});if(name==='conflict-manual-and-recovery')await page.locator('.conflict-panel').screenshot({path:path.join(out,name+'-comparison.png'),animations:'disabled'})}
async function initialization(width,theme,failingPath){
 scenario='init-'+failingPath+'-'+width+'-'+theme;let failing=true,puts=[];
 const{context,page}=await setup(width,theme,async(r,u,m)=>{
  if(m==='GET'&&u.pathname===(failingPath==='tags'?'/api/v1/tags':'/api/v1/studio/posts/1')&&failing){await fail(r,503,'模拟加载失败');return true}
  if(m==='PUT'){puts.push({data:r.request().postDataJSON(),version:r.request().headers()['if-match']});await json(r,{version:11,draft_version:21});return true}return false;
 });
 await page.goto(base+'/studio/posts/1');await page.getByText('编辑器未能安全加载',{exact:true}).waitFor();await info(page);
 check('failed load keeps title and summary locked',await page.getByLabel('文章标题',{exact:true}).isDisabled()&&await page.getByLabel('摘要',{exact:true}).isDisabled());
 check('failed load exposes no writable document',await page.locator('.document-prose[contenteditable=true],.cm-content[contenteditable=true]').count()===0);
 check('save/publish/mode/cover controls are disabled',await page.getByRole('button',{name:'保存草稿',exact:true}).isDisabled()&&await page.getByRole('button',{name:'发布文章',exact:true}).isDisabled()&&await page.getByRole('button',{name:'Markdown 源码',exact:true}).isDisabled()&&await page.getByRole('combobox',{name:'配色',exact:true}).isDisabled());
 await page.keyboard.press('Control+s');await page.evaluate(()=>window.dispatchEvent(new Event('online')));await page.waitForTimeout(1250);
 check('no write on shortcut online or timer after load failure',puts.length===0);await capture(page,scenario);
 failing=false;await page.getByRole('button',{name:'重新加载编辑器',exact:true}).click();await page.getByRole('textbox',{name:'文档正文',exact:true}).waitFor();
 check('retry loads original title and body before enabling',await page.getByLabel('文章标题',{exact:true}).inputValue()===original.title&&(await page.getByRole('textbox',{name:'文档正文',exact:true}).innerText()).includes('原有正文段落'));
 await page.getByLabel('文章标题',{exact:true}).fill('重新载入后修改');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await page.locator('.save-state.saved').waitFor();
 check('first write uses loaded versions and original body',puts.length===1&&puts[0].version==='"10"'&&puts[0].data.expected_draft_version===20&&puts[0].data.markdown===original.markdown);
 await context.close();
}
async function malformedInitialization(){
 scenario='malformed-successful-load';let malformed=true,writes=0;
 const{context,page}=await setup(1440,'light',async(r,u,m)=>{
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1'&&malformed){await r.fulfill({status:200,contentType:'text/html',body:'<!doctype html><title>Proxy response</title>'});return true}
  if(m==='PUT'){writes++;await fail(r,503,'Unexpected write');return true}return false;
 });
 await page.goto(base+'/studio/posts/1');await page.getByText('编辑器未能安全加载',{exact:true}).waitFor();
 check('successful non-JSON response cannot unlock a blank existing draft',await page.getByLabel('文章标题',{exact:true}).isDisabled()&&await page.locator('.document-prose[contenteditable=true]').count()===0);
 malformed=false;await page.getByRole('button',{name:'重新加载编辑器',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('.editor-title').disabled);
 check('malformed response can recover through validated retry',await page.getByLabel('文章标题',{exact:true}).inputValue()===original.title&&writes===0);await context.close();
}
async function conflictRecovery(){
 scenario='conflict-manual-and-recovery';let gets=0,puts=[],server=original;
 const{context,page}=await setup(1440,'dark',async(r,u,m)=>{
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1'){gets++;await json(r,gets===1?original:server);return true}
  if(m==='PUT'&&u.pathname.endsWith('/draft')){const data=r.request().postDataJSON();puts.push({data,version:r.request().headers()['if-match']});if(puts.length===1){server=latest;await fail(r,409,'模拟版本冲突')}else{server={...server,...data,version:server.version+1,draft_version:server.draft_version+1};await json(r,{version:server.version,draft_version:server.draft_version})}return true}return false;
 });
 await open(page,'source');await page.getByLabel('文章标题',{exact:true}).fill('本地标题修改');await page.getByRole('heading',{name:'另一处修改已先保存',exact:true}).waitFor();
 await page.getByRole('button',{name:'以服务器最新版本为基准，继续合并',exact:true}).click();
 check('comparison remains visible through manual merge',await page.locator('.conflict-panel').isVisible()&&(await page.locator('.conflict-panel').innerText()).includes('服务器独有的新段落'));
 const merged=original.markdown+'\n服务器独有的新段落。\n\n手动合并的段落。';await page.getByRole('textbox',{name:'Markdown 正文',exact:true}).fill(merged);await page.getByLabel('文章标题',{exact:true}).fill('手动合并标题');
 await page.evaluate(()=>{window.dispatchEvent(new Event('online'));document.querySelector('.cm-content').dispatchEvent(new CompositionEvent('compositionend',{bubbles:true}))});
 await page.waitForTimeout(10700);
 check('change composition online and interval cannot auto-save a merge',puts.length===1);
 check('publish and checkpoint wait for explicit merge save',await page.getByRole('button',{name:'发布文章',exact:true}).isDisabled()&&await page.getByRole('button',{name:'留检查点',exact:true}).isDisabled());await capture(page,scenario);
 await page.reload();await page.getByRole('button',{name:'加载恢复点',exact:true}).waitFor();await page.getByRole('button',{name:'加载恢复点',exact:true}).click();
 await page.waitForTimeout(1300);await page.evaluate(()=>window.dispatchEvent(new Event('online')));
 check('manual merge requirement survives a browser reload and recovery',puts.length===1&&await page.locator('.conflict-panel').isVisible()&&await page.getByLabel('文章标题',{exact:true}).inputValue()==='手动合并标题');
 await page.getByRole('button',{name:'Markdown 源码',exact:true}).click();await page.getByRole('textbox',{name:'Markdown 正文',exact:true}).click();await page.keyboard.press('Control+s');await page.locator('.save-state.saved').waitFor();
 check('explicit shortcut saves full merge with latest CAS versions',puts.length===2&&puts[1].version==='"11"'&&puts[1].data.expected_draft_version===21&&puts[1].data.markdown===merged);
 check('comparison clears only after explicit acknowledged save',await page.locator('.conflict-panel').count()===0);
 await page.getByLabel('文章标题',{exact:true}).fill('合并完成后继续写');await page.locator('.save-state.saved').waitFor();
 check('normal autosave resumes after acknowledged merge',puts.length===3&&puts[2].version==='"12"'&&puts[2].data.expected_draft_version===22);
 await context.close();
}
async function identicalManualRecovery(){
 scenario='content-equal-manual-recovery';let puts=0;
 const{context,page}=await setup(1440,'light',async(r,u,m)=>{if(m==='PUT'){puts++;await json(r,{version:11,draft_version:21});return true}return false});
 await open(page);
 await page.evaluate(async original=>{const req=indexedDB.open('painter-next-recovery',1),db=await new Promise((res,rej)=>{req.onsuccess=()=>res(req.result);req.onerror=()=>rej(req.error)});await new Promise((res,rej)=>{const t=db.transaction('drafts','readwrite');t.objectStore('drafts').put({...original,post_id:'1',client_draft_id:'1',seq:1,ack_seq:0,base_version:10,base_draft_version:20,manual_merge_required:true,conflict_snapshot:original,saved_at:Date.now()},`painter:${location.origin}:1:1`);t.oncomplete=()=>{db.close();res()};t.onerror=()=>rej(t.error)})},original);
 await page.reload();await page.getByRole('button',{name:'加载恢复点',exact:true}).waitFor();await page.getByRole('button',{name:'加载恢复点',exact:true}).click();
 await page.getByLabel('文章标题',{exact:true}).fill('相同内容恢复后继续合并');await page.evaluate(()=>window.dispatchEvent(new Event('online')));await page.waitForTimeout(1300);
 check('equal server content still offers and preserves manual phase',puts===0&&await page.locator('.conflict-panel').isVisible());
 await page.getByRole('button',{name:'保存草稿',exact:true}).click();await page.locator('.save-state.saved').waitFor();
 check('equal-content manual phase clears only on explicit save',puts===1&&await page.locator('.conflict-panel').count()===0);await context.close();
}
async function failedConflictRead(){
 scenario='conflict-latest-read-failure';let gets=0,failRead=true,puts=0;
 const{context,page}=await setup(390,'light',async(r,u,m)=>{
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1'){gets++;if(gets>1&&failRead)await fail(r,503,'服务器版本暂不可用');else await json(r,gets===1?original:latest);return true}
  if(m==='PUT'){puts++;await fail(r,409,'模拟冲突');return true}return false;
 });
 await open(page);await page.getByLabel('文章标题',{exact:true}).fill('必须保留的本地标题');await page.getByRole('button',{name:'重新载入服务器版本',exact:true}).waitFor();
 check('failed comparison read cannot adopt unknown versions',await page.getByRole('button',{name:'以服务器最新版本为基准，继续合并',exact:true}).count()===0);
 failRead=false;await page.getByRole('button',{name:'重新载入服务器版本',exact:true}).click();await page.getByRole('button',{name:'以服务器最新版本为基准，继续合并',exact:true}).click();await page.waitForTimeout(1300);
 check('retry preserves local inputs and still requires explicit save',await page.getByLabel('文章标题',{exact:true}).inputValue()==='必须保留的本地标题'&&puts===1);await capture(page,scenario);await context.close();
}
async function restoreLock(mode,width,theme){
 scenario='restore-'+mode+'-'+width+'-'+theme;let puts=[],restores=0,reads=0,postDone=false,failGet=true;
 const entered=gate(),releasePost=gate(),getEntered=gate(),releaseGet=gate();
 const{context,page}=await setup(width,theme,async(r,u,m)=>{
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/1'){reads++;if(postDone&&failGet){getEntered.release();await releaseGet.promise;if(mode==='source')await json(r,{});else await fail(r,503,'恢复后的读取暂时失败')}else await json(r,postDone?restored:original);return true}
  if(m==='PUT'&&u.pathname.endsWith('/draft')){puts.push(r.request().postDataJSON());await json(r,{version:12,draft_version:22});return true}
  if(m==='POST'&&u.pathname.endsWith('/restore')){restores++;entered.release();await releasePost.promise;postDone=true;await json(r,{version:14,draft_version:24});return true}return false;
 });
 try{
  await open(page,mode);await page.getByLabel('文章标题',{exact:true}).fill('恢复前刚完成的输入');await page.getByRole('button',{name:'查看修订记录',exact:true}).click();await page.getByRole('button',{name:'恢复为草稿',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'恢复为草稿',exact:true}).click();await entered.promise;await info(page);
  check('pending restore flushes unsaved inputs before posting',puts.length===1&&puts[0].title==='恢复前刚完成的输入'&&puts[0].markdown===original.markdown);
  check('pending restore locks title summary cover dates',await page.getByLabel('文章标题',{exact:true}).isDisabled()&&await page.getByLabel('摘要',{exact:true}).isDisabled()&&await page.getByLabel('显示创建时间',{exact:true}).isDisabled()&&await page.getByRole('combobox',{name:'封面来源',exact:true}).isDisabled());
  const body=page.getByRole('textbox',{name:mode==='source'?'Markdown 正文':'文档正文',exact:true});
  check('pending restore locks document and CodeMirror',await body.getAttribute('contenteditable')==='false'&&await page.locator('.cm-content').getAttribute('contenteditable')==='false');
  await page.getByLabel('文章标题',{exact:true}).evaluate(e=>e.focus());await page.keyboard.type('不应进入');await page.evaluate(()=>window.dispatchEvent(new Event('online')));await page.waitForTimeout(1150);
  check('pending restore rejects typing and autosave',await page.getByLabel('文章标题',{exact:true}).inputValue()==='恢复前刚完成的输入'&&puts.length===1);
  await capture(page,scenario+'-pending');releasePost.release();await getEntered.promise;
  check('GET after restore stays locked until authoritative content arrives',await page.getByLabel('文章标题',{exact:true}).isDisabled());releaseGet.release();await page.getByRole('button',{name:'重新载入当前草稿',exact:true}).waitFor();
  check('failed follow-up read keeps local content and lock',await page.getByLabel('文章标题',{exact:true}).isDisabled()&&await page.getByLabel('文章标题',{exact:true}).inputValue()==='恢复前刚完成的输入');await capture(page,scenario+'-read-failed');
  failGet=false;await page.getByRole('button',{name:'重新载入当前草稿',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('.editor-title').disabled);
  check('retry reads current content without repeating restore',restores===1&&await page.getByLabel('文章标题',{exact:true}).inputValue()===restored.title&&(await body.innerText()).includes('历史版本的正文'));
  check('restored content is acknowledged without spurious save',await page.locator('.save-state').innerText()==='已保存'&&puts.length===1);
 }finally{releasePost.release();releaseGet.release();await context.close()}
}
(async()=>{fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--disable-dev-shm-usage']});
 await initialization(1440,'light','tags');await initialization(390,'dark','post');await malformedInitialization();await conflictRecovery();await identicalManualRecovery();await failedConflictRead();await restoreLock('document',1440,'dark');await restoreLock('source',390,'light');
 check('no unhandled browser errors',errors.length===0);check('no unexpected business mutation requests',blocked.length===0);
 fs.rmSync(path.join(out,'failure.json'),{force:true});fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({checks,errors,blocked,actual_business_writes:0,passed:checks.every(c=>c.passed)},null,2)+'\n');console.log(JSON.stringify({checks:checks.length,passed:true,actual_business_writes:0}));await browser.close();
})().catch(async e=>{fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'failure.json'),JSON.stringify({checks,errors,blocked,error:String(e).slice(0,1400)},null,2));console.error(String(e));await browser?.close();process.exitCode=1});
