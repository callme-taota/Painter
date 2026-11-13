// Isolated regression for session teardown and IndexedDB recovery cleanup.
// This creates disposable, unpublished author drafts; no production account is used.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PAINTER_PLAYWRIGHT_MODULE||'/tmp/painter-browser/node_modules/playwright');
const base='http://127.0.0.1:18091',privateDir='/tmp/painter-v49-private',environment=JSON.parse(fs.readFileSync(privateDir+'/environment.json'));
assert(environment.DATABASE_URL==='host=/tmp/painter-v39-card-pg/socket port=65442 user=callumchan dbname=painter_v49_features_20261114 sslmode=disable'&&environment.PUBLIC_BASE_URL===base&&environment.ENVIRONMENT==='development','Refusing a non-isolated candidate');
const credential=JSON.parse(fs.readFileSync(privateDir+'/accounts.json')).author;
const out=path.resolve(__dirname,'../artifacts/v49/logout-recovery');fs.mkdirSync(out,{recursive:true});const checks=[],scenarios=[],errors=[];
function check(name,value){assert.ok(value,name);checks.push({name,passed:true})}
async function recoveryKeys(page){return page.evaluate(()=>new Promise((resolve,reject)=>{const request=indexedDB.open('painter-next-recovery',1);request.onupgradeneeded=()=>request.result.createObjectStore('drafts');request.onsuccess=()=>{const db=request.result,tx=db.transaction('drafts'),query=tx.objectStore('drafts').getAllKeys();query.onsuccess=()=>resolve(query.result.map(String));query.onerror=()=>reject(new Error('Unable to read isolated recovery keys'));tx.oncomplete=()=>db.close()};request.onerror=()=>reject(new Error('Unable to open isolated recovery store'))}))}
(async()=>{const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});try{for(const scenario of ['normal','delayed','expired']){const delayed=scenario==='delayed',expired=scenario==='expired';
 const context=await browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce'}),page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(()=>{const original=indexedDB.open.bind(indexedDB),descriptor=Object.getOwnPropertyDescriptor(IDBRequest.prototype,'onsuccess');indexedDB.open=function(name,...args){const request=original(name,...args);if(name==='painter-next-recovery'&&window.__v49RecoveryDelay){const delay=window.__v49RecoveryDelay;window.__v49RecoveryDelay=0;window.__v49RecoveryRelease=Date.now()+delay;Object.defineProperty(request,'onsuccess',{configurable:true,get(){return descriptor.get.call(request)},set(callback){descriptor.set.call(request,event=>setTimeout(()=>callback?.call(request,event),delay))}})}return request}});
 try{
  await page.goto(base+'/login?return_to=%2Fstudio%2Fposts%2Fnew');await page.getByLabel('用户名',{exact:true}).fill(credential.username);await page.getByLabel('密码',{exact:true}).fill(credential.password);await page.getByRole('button',{name:'登录',exact:true}).click();await page.waitForURL(base+'/studio/posts/new');
  const session=await(await context.request.get(base+'/api/v1/session')).json();check('author session is an isolated fixture',session.authenticated&&session.user.role==='author');
  await page.locator('.editor-title').fill('V49 isolated logout recovery '+(delayed?'pending write':'normal'));await page.waitForFunction(()=>document.querySelector('.editor-page .save-state')?.textContent.includes('已保存'));await page.waitForTimeout(250);
  const before=await recoveryKeys(page);check('saved draft has an owner-scoped recovery record',before.some(key=>key.includes(':'+session.user.id+':')));
  if(expired){
   await page.evaluate(()=>history.replaceState(history.state,'',location.pathname+'?from=v49-expired#recovery-v49'));
   const returnTo=new URL(page.url()).pathname+'?from=v49-expired#recovery-v49';
   const revoked=await context.request.delete(base+'/api/v1/session',{headers:{Origin:base,'X-CSRF-Token':session.csrf_token,'Idempotency-Key':require('node:crypto').randomUUID()}});check('isolated session revoked outside the page',revoked.status()===204);
   const pendingTitle='V49 expired session unsaved title';await page.evaluate(()=>{window.__v49RecoveryDelay=1200});await page.locator('.editor-title').fill(pendingTitle);
   const rejectedSave=page.waitForResponse(response=>response.status()===401&&new URL(response.url()).pathname.startsWith('/api/v1/studio/posts/'));
   await page.getByRole('button',{name:'保存草稿',exact:true}).click();await page.waitForFunction(()=>window.__v49RecoveryRelease>0);const release=await page.evaluate(()=>window.__v49RecoveryRelease);await rejectedSave;
   await page.waitForURL(url=>url.pathname==='/login');const returnedAt=Date.now();check('expiry redirects to login with current path query and hash',new URL(page.url()).searchParams.get('return_to')===returnTo);
   const after=await recoveryKeys(page);check('expiry retains the original actor recovery namespace',after.some(key=>key.includes(':'+session.user.id+':')));check('expiry creates no undefined actor records',after.every(key=>!key.includes(':undefined:')));
   const savedLatest=await page.evaluate(({id,title})=>new Promise((resolve,reject)=>{const request=indexedDB.open('painter-next-recovery',1);request.onsuccess=()=>{const db=request.result,tx=db.transaction('drafts'),cursor=tx.objectStore('drafts').openCursor();let found=false;cursor.onsuccess=()=>{const item=cursor.result;if(item){if(String(item.key).includes(':'+id+':')&&item.value.title===title)found=true;item.continue()}};tx.oncomplete=()=>{db.close();resolve(found)};tx.onerror=()=>reject(new Error('Unable to verify isolated expired-session recovery'))};request.onerror=()=>reject(new Error('Unable to open isolated expired-session recovery'))}),{id:session.user.id,title:pendingTitle});
   check('expiry flushes the latest unsaved snapshot before redirect',savedLatest);check('expiry waits for the delayed local write',returnedAt>=release-30);
   scenarios.push({session_expiry:true,records_before:before.length,records_after:after.length,original_actor_snapshot_preserved:savedLatest,return_path_query_hash_preserved:new URL(page.url()).searchParams.get('return_to')===returnTo,awaited_pending_write:returnedAt>=release-30});continue;
  }
  let release=0;
  if(delayed){await page.evaluate(()=>{window.__v49RecoveryDelay=1200});await page.locator('.editor-title').fill('V49 isolated pending recovery write');await page.waitForFunction(()=>window.__v49RecoveryRelease>0);release=await page.evaluate(()=>window.__v49RecoveryRelease);check('delayed local recovery write is in flight',release>Date.now())}
  await page.getByRole('button',{name:'退出登录',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'退出登录',exact:true}).click();await page.waitForURL(base+'/');const returnedAt=Date.now();
  const after=await recoveryKeys(page);check('sign-out removes the actor recovery namespace',after.every(key=>!key.includes(':'+session.user.id+':')));check('sign-out creates no undefined actor records',after.every(key=>!key.includes(':undefined:')));check('sign-out leaves the disposable browser recovery store empty',after.length===0);
  if(delayed)check('navigation waited for the pending local write before cleanup',returnedAt>=release-30);
  const ended=await(await context.request.get(base+'/api/v1/session')).json();check('server session is revoked',ended.authenticated===false);
  scenarios.push({delayed_recovery_write:delayed,records_before:before.length,records_after:after.length,undefined_actor_records:after.filter(key=>key.includes(':undefined:')).length,awaited_pending_write:!delayed||returnedAt>=release-30});
 }finally{await context.close()}
 }
 check('no application runtime errors',errors.length===0);console.log(JSON.stringify({status:'passed',checks:checks.length,scenarios:scenarios.length,production_credentials_used:false}));
 }finally{fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({base,isolated_database:'painter_v49_features_20261114',checks,scenarios,errors,production_credentials_used:false},null,2));await browser.close()}
})().catch(error=>{console.error(error.message);process.exitCode=1});
