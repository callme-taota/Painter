// Real routes, actual built assets, disposable V49 candidate only. No storage
// state, credentials, production configuration or database connection is saved.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {chromium,request}=require('/tmp/painter-browser/node_modules/playwright');
const base='http://127.0.0.1:18091',privateDir='/tmp/painter-v49-private',out=path.resolve('tests/artifacts/v49/sharing-real');
const env=JSON.parse(fs.readFileSync(path.join(privateDir,'environment.json'),'utf8'));
if(env.PUBLIC_BASE_URL!==base||env.LISTEN_ADDR!=='127.0.0.1:18091'||env.RUN_BACKGROUND_WORKERS!=='false'||!env.DATABASE_URL.includes('port=65442')||!env.DATABASE_URL.includes('dbname=painter_v49_'))throw Error('Disposable V49 environment guard failed');
const account=JSON.parse(fs.readFileSync(path.join(privateDir,'accounts.json'),'utf8')).owner;
let checks=0;const records=[],errors=[];function check(value,message){assert.ok(value,message);checks++}
function relative(value){const u=new URL(value);check(u.origin===base,'share target escaped fixed candidate origin');return u.pathname+u.search}
(async()=>{
 fs.mkdirSync(out,{recursive:true});
 const api=await request.newContext({baseURL:base});
 const options=await api.get('/api/v1/auth/options').then(r=>r.json());
 const login=await api.post('/api/v1/auth/login',{headers:{Origin:base,'X-CSRF-Token':options.csrf_token},data:account});
 check(login.status()===200,'isolated owner login');
 const session=await api.get('/api/v1/session').then(r=>r.json());check(session.user?.role==='owner','owner is required');
 async function write(method,url,data,version){const response=await api.fetch('/api/v1'+url,{method,headers:{Origin:base,'X-CSRF-Token':session.csrf_token,'Idempotency-Key':crypto.randomUUID(),...(version?{'If-Match':'"'+version+'"'}:{})},...(data===undefined?{}:{data})});check(response.ok(),method+' '+url+' status '+response.status());return response.status()===204?{}:response.json()}
 const marker='V49 sharing route acceptance '+crypto.randomUUID().slice(0,8);
 const post=await write('POST','/posts',{client_draft_id:crypto.randomUUID(),title:marker,summary:'仅用于隔离候选环境，验收三种分享方式与撤回后的权限变化。',markdown:'BODY_MUST_NOT_APPEAR_IN_SHARED_PREVIEW '+marker,cover:{mode:'auto',template:'editorial',color:'ink'}});
 let response=await api.get('/api/v1/share?type=post&id='+post.id);check(response.status()===404,'owner draft leaked into share manifest');
 const published=await write('POST','/studio/posts/'+post.id+'/publish',{expected_draft_version:post.draft_version||1},post.version);
 const moment=await write('POST','/moments',{text:'V49 隔离分享动态：分享展示当前公开状态，撤回后旧链接不可再读。'});
 await write('POST','/studio/moments/'+moment.id+'/publish',{},moment.version);
 const phrase=await write('POST','/admin/phrases',{text:'V49 隔离分享片语：文字值得被看见',kind:'sentence',source:'V49 分享验收',published:true});
 const series=await write('POST','/series',{title:'V49 隔离分享连载',slug:'v49-sharing-'+crypto.randomUUID().slice(0,8),description:'只用于隔离环境中的专栏分享验收。',status:'published',state:'ongoing',chapter_post_ids:[post.id]});
 const manifests={};
 for(const [kind,id] of [['post',post.id],['moment',moment.id],['phrase',phrase.id],['series',series.id]]){
  response=await api.get('/api/v1/share?type='+kind+'&id='+id);check(response.status()===200,kind+' manifest');const manifest=await response.json();manifests[kind]=manifest;
  const again=await api.get('/api/v1/share?type='+kind+'&id='+id).then(r=>r.json());check(again.short_url===manifest.short_url,kind+' unstable short URL');
  const short=await api.get(relative(manifest.short_url),{maxRedirects:0});check(short.status()===302,kind+' short is not redirect');check(short.headers().location===manifest.canonical_url,kind+' short destination');check(short.headers()['cache-control'].includes('no-store'),kind+' short cache policy');
  for(const key of ['share_url','embed_url']){
   const doc=await api.get(relative(manifest[key]));check(doc.status()===200,kind+' '+key+' visibility');check(doc.headers()['cache-control'].includes('no-store'),kind+' '+key+' cache policy');const body=await doc.text();
   check(!body.includes('<script')&&!body.includes('BODY_MUST_NOT_APPEAR_IN_SHARED_PREVIEW'),kind+' share loaded script or full article body');
   check((body.match(/rel="stylesheet"/g)||[]).length===1,kind+' share should load one small stylesheet');
   if(key==='embed_url')check(!doc.headers()['x-frame-options']&&doc.headers()['content-security-policy'].includes('frame-ancestors *')&&doc.headers()['content-security-policy'].includes("script-src 'none'"),kind+' embed frame/script policy');
   else check(doc.headers()['x-frame-options']==='DENY'&&doc.headers()['content-security-policy'].includes("frame-ancestors 'none'"),kind+' landing frame policy');
  }
 }
 const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 let parent;
 try{
  for(const mobile of [false,true]){
   const context=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1365,height:950},isMobile:mobile,hasTouch:mobile});
   const page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));const calls=[];page.on('request',r=>{if(r.url().startsWith(base))calls.push({path:new URL(r.url()).pathname,type:r.resourceType()})});
   await page.goto(base+'/posts/'+post.id,{waitUntil:'domcontentloaded'});await page.locator('[data-sharing-dialog]').waitFor({state:'attached'});
   check(!calls.some(r=>r.path==='/api/v1/share'),'manifest fetched before opening share');
   await page.locator('[data-share-type=post][data-share-id="'+post.id+'"]').first().click();await page.waitForFunction(()=>!document.querySelector('.sharing-copy').disabled);
   check(calls.filter(r=>r.path==='/api/v1/share').length===1,'share open count');
   await page.locator('[data-sharing-mode=embed]').click();check((await page.locator('.sharing-field textarea').inputValue()).includes('sandbox='),'actual embed copy text');
   await page.locator('[data-sharing-mode=share]').click();check(await page.locator('.sharing-field textarea').inputValue()===manifests.post.share_url,'actual dedicated share link');
   check(calls.filter(r=>r.path==='/api/v1/share').length===1,'tabs added unnecessary requests');
   await page.screenshot({path:path.join(out,(mobile?'mobile':'desktop')+'-panel.png'),fullPage:false,animations:'disabled'});
   if(mobile){await page.goBack();await page.waitForFunction(()=>!document.querySelector('.sharing-dialog').open);check(new URL(page.url()).pathname==='/posts/'+post.id,'mobile Back left article');await page.goForward();await page.waitForFunction(()=>document.querySelector('.sharing-dialog').open&&!document.querySelector('.sharing-copy').disabled);check(calls.filter(r=>r.path==='/api/v1/share').length===2,'mobile Forward did not revalidate');}
   await page.keyboard.press('Escape');await page.waitForFunction(()=>!document.querySelector('.sharing-dialog').open);
   records.push({kind:'article',viewport:mobile?'390x844':'1365x950',requests:calls.slice()});
   for(const [kind,url] of [['moment','/moments/'+moment.id],['phrase','/phrases/'+phrase.id],['series','/series/'+series.id]]){
    await page.goto(base+url,{waitUntil:'domcontentloaded'});await page.locator('[data-sharing-dialog]').waitFor({state:'attached'});const button=page.locator('[data-share-type='+kind+'][data-share-id="'+({moment,phrase,series}[kind].id)+'"]').first();check(await button.count()===1,kind+' missing real share entry');await button.click();await page.waitForFunction(()=>!document.querySelector('.sharing-copy').disabled);check(await page.locator('.sharing-field textarea').inputValue()===manifests[kind].short_url,kind+' incorrect share entry');await page.keyboard.press('Escape');
   }
   await page.goto(manifests.post.share_url,{waitUntil:'domcontentloaded'});check(await page.locator('h1').textContent()===marker,'landing title');check(await page.locator('.share-document-cta').getAttribute('href')===manifests.post.canonical_url,'landing CTA');check(await page.locator('script').count()===0,'landing script DOM');check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'landing horizontal overflow');await page.screenshot({path:path.join(out,(mobile?'mobile':'desktop')+'-landing.png'),fullPage:true});
   // A share preview must retain its CTA with a one-character title and with
   // either long unbroken Latin text or the full 120-character CJK title cap.
   for(const width of mobile?[320,390]:[640,1280]){
    await page.setViewportSize({width,height:360});await page.goto(manifests.post.embed_url,{waitUntil:'domcontentloaded'});
    for(const text of ['字','A'.repeat(120),'字'.repeat(120)]){
     await page.locator('h1').evaluate((el,text)=>{el.textContent=text},text);
     check(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'embed title caused horizontal overflow at '+width);
     check(await page.locator('.share-document-cta').evaluate(el=>el.getBoundingClientRect().bottom<=innerHeight),'embed title pushed CTA outside iframe at '+width);
    }
   }
   await context.close();
  }
  // An unrelated origin frames the exact copied HTML, without changing Painter's
  // frame-src/CSP. Popup escape permissions are tested with a real click.
  parent=http.createServer((req,res)=>{res.setHeader('Content-Type','text/html;charset=utf-8');res.end('<!doctype html><meta charset="utf-8"><title>V49 isolated external embed host</title><main style="max-width:680px;margin:24px auto">'+manifests.post.embed_html+'</main>')});await new Promise(resolve=>parent.listen(0,'127.0.0.1',resolve));const parentURL='http://127.0.0.1:'+parent.address().port;
  const context=await browser.newContext({viewport:{width:1000,height:700}});const page=await context.newPage(),embedCalls=[];page.on('request',r=>embedCalls.push({path:new URL(r.url()).pathname,type:r.resourceType()}));page.on('pageerror',e=>errors.push(e.message));await page.goto(parentURL,{waitUntil:'domcontentloaded'});const frame=page.frameLocator('iframe');await frame.locator('h1').waitFor();check(await frame.locator('h1').textContent()===marker,'cross-origin embed failed to render');check(!embedCalls.some(r=>r.type==='script'||r.path.startsWith('/api/v1/session')),'embed loaded app JS/session');check(embedCalls.filter(r=>r.type==='stylesheet').length===1,'embed did not use standalone CSS');await page.screenshot({path:path.join(out,'cross-origin-embed.png'),fullPage:true});const popupPromise=context.waitForEvent('page');await frame.locator('.share-document-cta').click();const popup=await popupPromise;await popup.waitForLoadState('domcontentloaded');check(new URL(popup.url()).pathname==='/posts/'+post.id,'embed CTA did not open original article');check(await popup.evaluate(()=>window.opener===null),'embed popup retained opener');records.push({kind:'external iframe',requests:embedCalls});await context.close();
  // Withdraw using normal authenticated APIs. Every old entry re-checks public
  // visibility, even if an owner presents a formerly valid signed URL.
  await write('DELETE','/studio/posts/'+post.id+'/publication',undefined,published.version);
  await write('DELETE','/studio/moments/'+moment.id+'/publication',undefined,2);
  await write('DELETE','/admin/phrases/'+phrase.id,undefined,phrase.version);
  await write('PUT','/studio/series/'+series.id,{title:'V49 隔离分享连载',slug:series.slug,description:'只用于隔离环境中的专栏分享验收。',status:'draft',state:'ongoing',chapter_post_ids:[post.id]},series.version);
  for(const [kind,manifest] of Object.entries(manifests)){
   response=await api.get('/api/v1/share?type='+kind+'&id='+manifest.id);check(response.status()===404,kind+' manifest survived withdrawal');
   for(const key of ['short_url','share_url','embed_url']){const withdrawn=await api.get(relative(manifest[key]),{maxRedirects:0});check(withdrawn.status()===404,kind+' '+key+' survived withdrawal');check(withdrawn.headers()['cache-control'].includes('no-store'),kind+' withdrawn cache policy');}
  }
  check(errors.length===0,'browser runtime errors '+errors.join('; '));
  const result={checks,errors,origin:base,production_writes:0,scope:'actual V49 backend and built assets; Chromium desktop/mobile emulation; real cross-origin sandboxed iframe and popup; owner API creates/publishes/withdraws isolated fixtures',records};fs.writeFileSync(path.join(out,'result.json'),JSON.stringify(result,null,2));console.log(JSON.stringify({checks,errors,artifacts:out}));
 }finally{await browser.close();if(parent)await new Promise(resolve=>parent.close(resolve));await api.dispose()}
})().catch(error=>{fs.writeFileSync(path.join(out,'failure.json'),JSON.stringify({checks,errors,message:error.message},null,2));console.error('V49 sharing acceptance: '+error.message);process.exitCode=1});
