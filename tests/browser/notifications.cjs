// Real private workbench, synthetic API only. Never reads local accounts or a running service.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{createRequire}=require('node:module');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const frontend=createRequire(path.resolve(__dirname,'../../web/frontend/package.json'));
const {build}=frontend('esbuild'),{parse,compileScript}=frontend('@vue/compiler-sfc');
const src=path.resolve(__dirname,'../../web/frontend/src'),out=process.env.ARTIFACT_DIR||'/tmp/painter-notifications-browser';fs.mkdirSync(out,{recursive:true});
const checks=[],errors=[];function check(name,value){assert.ok(value,name);checks.push(name);console.log('PASS '+name)}
const copy=value=>JSON.parse(JSON.stringify(value));
const row=(id,extra={})=>({id:String(id),kind:'reply',created_at:'2026-11-28T10:00:00Z',unread:true,available:true,actor:{id:'9',display_name:'朋友 Alice'},post_title:'一起写作，也继续讨论',body:'回复里的 <script> 只是普通文字。新的一段想法，值得接着聊下去。',post_id:'42',comment_id:String(200+id),...extra});
async function bundle(){
 const styles=[...fs.readFileSync(path.join(src,'public.ts'),'utf8').matchAll(/^import '([^']+\.css)'/gm),...fs.readFileSync(path.join(src,'studio.ts'),'utf8').matchAll(/^import '([^']+\.css)'/gm)].map(match=>`import '${match[1]}';`).join('');
 const built=await build({stdin:{contents:styles+"import{createApp}from'vue';import App from'./App.vue';const app=createApp(App);app.mount('#fixture');window.unmountFixture=()=>app.unmount();",resolveDir:src,loader:'ts'},bundle:true,write:false,outfile:'/fixture.js',format:'esm',platform:'browser',loader:{'.woff2':'dataurl'},define:{__VUE_OPTIONS_API__:'true',__VUE_PROD_DEVTOOLS__:'false',__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:'false'},plugins:[{name:'vue',setup(build){build.onLoad({filter:/\.vue$/},args=>{const {descriptor}=parse(fs.readFileSync(args.path,'utf8'),{filename:args.path});return{contents:compileScript(descriptor,{id:path.basename(args.path),inlineTemplate:true}).content+'\nexport const __esModule=true;',loader:'ts',resolveDir:path.dirname(args.path)}})}}]});
 return{js:built.outputFiles.find(f=>f.path.endsWith('.js')).text,css:built.outputFiles.find(f=>f.path.endsWith('.css')).text};
}
async function fixture(browser,assets,options={}){
 const state={viewer:'7',authenticated:true,responseViewer:'7',rows:[row(5,{actor:{id:'9',display_name:'一个很长的朋友名字，用来确认小屏幕仍能完整换行'},kind:'comment'}),row(4,{available:false,actor:undefined,post_title:undefined,body:undefined,post_id:undefined,comment_id:undefined}),row(3,{unread:false}),row(2)],requests:[],failAppend:0,loseRead:0,...options};
 const page=await browser.newPage({viewport:{width:1200,height:900},reducedMotion:'reduce'});page.setDefaultTimeout(12000);page.on('pageerror',e=>errors.push(e.message));
 await page.addInitScript(()=>localStorage.setItem('painter-analytics','off'));
 await page.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url()),method=request.method();
  const send=(data,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
  if(url.origin!=='https://notifications.test'){errors.push('External '+url);return route.abort()}
  if(url.pathname==='/fixture.js')return route.fulfill({contentType:'text/javascript',body:assets.js});
  if(url.pathname==='/fixture.css')return route.fulfill({contentType:'text/css',body:assets.css});
  if(url.pathname==='/login')return route.fulfill({contentType:'text/html',body:'<!doctype html><p>Sign in</p>'});
  if(request.isNavigationRequest())return route.fulfill({contentType:'text/html',body:`<!doctype html><html lang="${options.english?'en':'zh-CN'}"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><body data-page-kind="studio"><div id="fixture"></div><script type="module" src="/fixture.js"></script></body></html>`});
  if(url.pathname==='/assets/images/painter-logo.png')return route.fulfill({contentType:'image/png',body:fs.readFileSync(path.join(src,'../public/images/painter-logo.png'))});
  if(url.pathname.startsWith('/assets/'))return route.fulfill({status:204,body:''});
  const entry={path:url.pathname,query:url.search,method,headers:request.headers()};state.requests.push(entry);
  if(url.pathname==='/api/v1/session')return send({authenticated:state.authenticated,user:{id:state.viewer,role:'reader',username:'fixture',display_name:'测试读者'},csrf_token:'fixture-csrf'});
  if(url.pathname==='/api/v1/me/notifications'){
   if(state.status)return send({error:{message:'登录已失效'}},state.status);
   if(url.searchParams.has('cursor')&&state.failAppend-->0)return send({error:{message:'后续通知载入失败'}},503);
   const rows=state.rows.filter(row=>url.searchParams.get('filter')!=='unread'||row.unread),append=url.searchParams.has('cursor');
   const data={viewer_id:state.responseViewer,filter:url.searchParams.get('filter'),unread_count:state.rows.filter(row=>row.unread).length,items:copy(append?rows.slice(2):rows.slice(0,3)),has_more:!append&&rows.length>3,next_cursor:!append&&rows.length>3?'older':''};
   if(state.holdNext){const release=state.holdNext;state.holdNext=null;state.readHeld=true;await release;try{return await send(data)}catch{return}}
   return send(data);
  }
  const read=url.pathname.match(/^\/api\/v1\/me\/notifications\/(\d+)\/read$/);
  if(read){state.rows.find(row=>row.id===read[1]).unread=false;if(state.loseRead-->0)return send({error:{message:'尚未确认已读状态'}},503);return route.fulfill({status:204,body:''})}
  errors.push('Unexpected '+method+' '+url.pathname);return send({},500);
 });
 await page.goto('https://notifications.test/me/notifications');await page.locator('.notifications-toolbar').waitFor();await page.waitForFunction(()=>!document.querySelector('.notifications-loading'));
 return{page,state};
}
(async()=>{const assets=await bundle(),browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});try{
 {
  const {page,state}=await fixture(browser,assets,{failAppend:1});
  check('reader reaches inbox through personal-space navigation',await page.locator('.studio-links a[href="/me/notifications"][aria-current=page]').count()===1);
  check('inbox shows authoritative unread count',await page.locator('.notifications-toolbar [role=status]').innerText()==='3 条未读');
  check('source excerpts remain plain text',await page.locator('.notification-excerpt').first().innerText().then(text=>text.includes('<script>'))&&await page.locator('.notifications script').count()===0);
  const unavailable=page.locator('[data-notification-id="4"]');check('unavailable notification has no identity or deep link',await unavailable.locator('a').count()===0&&!(await unavailable.innerText()).includes('Alice'));
  check('view action targets the precise comment',await page.locator('[data-notification-id="5"] a').getAttribute('href')==='/posts/42?comment=205#comment-205');
  await page.locator('[data-infinite-sentinel]').scrollIntoViewIfNeeded();await page.getByRole('button',{name:'重试载入',exact:true}).waitFor();
  check('append failure keeps previously loaded notifications',await page.locator('.notification-row').count()===3);
  await page.getByRole('button',{name:'重试载入',exact:true}).click();await page.locator('[data-notification-id="2"]').waitFor();
  check('append retry retains cursor and removes overlaps',state.requests.filter(r=>r.query.includes('cursor=')).every(r=>new URLSearchParams(r.query).get('cursor')==='older')&&await page.locator('.notification-row').count()===4);
  await page.evaluate(()=>scrollTo(0,0));await page.screenshot({path:path.join(out,'notifications-desktop.png'),fullPage:true});
  for(const width of [320,390]){await page.setViewportSize({width,height:844});await page.waitForFunction(()=>document.querySelector('.workbench')?.classList.contains('is-phone'));check(width+'px inbox has no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));check(width+'px read actions retain 44px targets',await page.locator('.notification-actions button').first().evaluate(el=>el.getBoundingClientRect().height>=44))}
  await page.screenshot({path:path.join(out,'notifications-mobile.png'),fullPage:true});
  await page.evaluate(()=>document.documentElement.dataset.theme='dark');await page.screenshot({path:path.join(out,'notifications-mobile-dark.png'),fullPage:true});await page.close();
 }
 {
  const {page,state}=await fixture(browser,assets,{rows:[row(5),row(4,{available:false}),row(3,{unread:false})],loseRead:1});
  await page.getByRole('button',{name:'未读',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.notification-row').length===2);
  await page.locator('[data-notification-id="5"]').getByRole('button',{name:'标为已读',exact:true}).click();await page.getByText('尚未确认已读状态',{exact:true}).waitFor();
  check('uncertain mark-read keeps the row and allows retry',await page.locator('[data-notification-id="5"]').getByRole('button',{name:'标为已读',exact:true}).isEnabled());
  await page.locator('[data-notification-id="5"]').getByRole('button',{name:'标为已读',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('[data-notification-id="5"]'));
  const writes=state.requests.filter(r=>r.method==='PUT');check('read mutation keeps CSRF and uses only the intended recipient resource',writes.length===2&&writes.every(r=>r.path==='/api/v1/me/notifications/5/read'&&r.headers['x-csrf-token']==='fixture-csrf'));
  check('read retry refreshes unread count without double decrement',await page.locator('.notifications-toolbar [role=status]').innerText()==='1 条未读');
  await page.locator('[data-notification-id="4"]').getByRole('button',{name:'标为已读',exact:true}).click();await page.getByText('没有未读通知',{exact:true}).waitFor();
  await page.getByRole('button',{name:'查看全部通知',exact:true}).click();await page.waitForFunction(()=>document.querySelectorAll('.notification-row').length===3);check('empty unread view can return to retained read history',await page.locator('.notification-meta').filter({hasText:'已读'}).count()===3);await page.close();
 }
 {
  const {page,state}=await fixture(browser,assets,{rows:[row(5),row(4)]});let release;state.holdNext=new Promise(resolve=>release=resolve);
  await page.getByRole('button',{name:'刷新通知',exact:true}).click();for(let i=0;!state.readHeld&&i<100;i++)await new Promise(resolve=>setTimeout(resolve,10));assert.ok(state.readHeld);
  await page.locator('[data-notification-id="5"]').getByRole('button',{name:'标为已读',exact:true}).click();await page.waitForFunction(()=>document.querySelector('[data-notification-id="5"] .notification-meta')?.textContent.includes('已读'));
  release();await page.waitForTimeout(100);check('late pre-write read cannot restore unread status',await page.locator('[data-notification-id="5"] .notification-meta').innerText().then(text=>text.includes('已读')));await page.close();
 }
 {
  const {page,state}=await fixture(browser,assets,{rows:[row(5)]});await page.evaluate(()=>dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true})));
  check('pagehide synchronously conceals private inbox content',await page.locator('.notifications').evaluate(el=>el.hidden));
  state.viewer='8';await page.evaluate(()=>dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true})));await page.waitForURL('**/login?**');check('changed-account BFCache restore never reveals previous notifications',!(await page.locator('body').innerText()).includes('朋友 Alice'));await page.close();
 }
 {
  const {page,state}=await fixture(browser,assets,{rows:[row(5)]});state.status=401;await page.getByRole('button',{name:'刷新通知',exact:true}).click();await page.waitForURL('**/login?**');check('expired session clears inbox before login navigation',!(await page.locator('body').innerText()).includes('朋友 Alice'));await page.close();
 }
 {
  const {page,state}=await fixture(browser,assets,{rows:[row(5)]});state.responseViewer='8';await page.getByRole('button',{name:'刷新通知',exact:true}).click();await page.waitForURL('**/login?**');check('changed-viewer response conceals previous account even without a session event',!(await page.locator('body').innerText()).includes('朋友 Alice'));await page.close();
 }
 {
  const {page}=await fixture(browser,assets,{english:true,rows:[]});check('English empty state and controls are translated',await page.getByText('No notifications yet',{exact:true}).count()===1&&await page.getByRole('button',{name:'Refresh notifications',exact:true}).count()===1);await page.close();
 }
 check('all inbox traffic is intercepted with no runtime errors',errors.length===0);
 fs.writeFileSync(path.join(out,'notifications-report.json'),JSON.stringify({status:'passed',checks,errors},null,2));
}finally{for(const context of browser.contexts())for(const page of context.pages()){await page.screenshot({path:path.join(out,'notifications-unfinished.png'),fullPage:true});fs.writeFileSync(path.join(out,'notifications-unfinished.html'),await page.content())}await browser.close()}})().catch(error=>{console.error(error);process.exitCode=1});
