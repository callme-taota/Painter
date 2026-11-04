// Read-only acceptance of the formal V25 binary; no business requests are written.
const fs=require('fs'),crypto=require('crypto'),{chromium}=require('/tmp/painter-browser/node_modules/playwright');
const base='http://192.168.2.209:18080',out='tests/artifacts/v25',checks=[],errors=[],blocked=[],captures=[];
let browser;
const check=(name,ok)=>{checks.push({name,passed:!!ok});if(!ok)throw Error(name)};
const settle=async p=>{await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));await p.waitForTimeout(300)};
const read=p=>p.locator('.floating-controls').evaluate(e=>{const s=getComputedStyle(e),b=getComputedStyle(e,'::before');return{target:Number(e.dataset.mediaGlassAmount),amount:Number(s.getPropertyValue('--painter-media-glass')),opacity:Number(b.opacity),filter:b.backdropFilter,enhanced:e.classList.contains('has-media-glass'),background:s.backgroundColor}});
async function geometry(p){return p.evaluate(()=>{const rect=e=>{const r=e.getBoundingClientRect();return{top:scrollY+r.top,bottom:scrollY+r.bottom}},r=document.querySelector('.floating-controls').getBoundingClientRect();return{game:rect(document.querySelector('[data-game-story]')),record:rect(document.querySelector('[data-record-wall]')),height:innerHeight,rail:{top:r.top,height:r.height}}})}
async function scroll(p,y){await p.evaluate(top=>scrollTo({top,behavior:'instant'}),y);await settle(p)}
async function shot(p,name){await p.evaluate(()=>document.fonts.ready);await p.waitForFunction(()=>Array.from(document.querySelectorAll('[data-game-frame].is-current [data-game-hero],[data-game-frame].is-current [data-game-logo]')).every(i=>i.hidden||(i.complete&&i.naturalWidth>0)),null,{timeout:20000});const images=await p.locator('img').evaluateAll(async es=>{const visible=es.filter(i=>{const r=i.getBoundingClientRect();if(i.hidden||!r.width||!r.height||r.top>=innerHeight||r.bottom<=0)return false;for(let e=i;e;e=e.parentElement){const s=getComputedStyle(e);if(s.display==='none'||s.visibility==='hidden'||Number(s.opacity)===0)return false}return true});await Promise.all(visible.map(i=>i.decode().catch(()=>{})));return visible.map(i=>({loaded:i.complete&&i.naturalWidth>0,alt:i.alt}))});check(name+' visible images ready',images.every(i=>i.loaded));const file=out+'/visual/formal-'+name+'.png';await p.screenshot({path:file,fullPage:false});captures.push({file,url:p.url(),viewport:p.viewportSize(),images})}
(async()=>{
 browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 for(const[width,height,theme]of[[1440,900,'light'],[390,844,'dark']]){
  const c=await browser.newContext({viewport:{width,height},colorScheme:theme});
  await c.addInitScript(t=>{localStorage.setItem('painter-theme',t);localStorage.setItem('painter-analytics','off')},theme);
  await c.route('**/*',r=>{const q=r.request(),u=new URL(q.url());if(u.pathname.includes('/analytics/'))return r.fulfill({json:{accepted:0}});if(!['GET','HEAD'].includes(q.method())){blocked.push({method:q.method(),path:u.pathname});return r.abort()}return r.continue()});
  const p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));await p.goto(base+'/');
  await p.waitForFunction(()=>document.querySelector('.floating-controls')?.dataset.mediaGlassReady==='true');await settle(p);
  let s=await read(p);check(width+' intro original solid',s.target===0&&s.opacity===0);
  const g=await geometry(p);await scroll(p,g.game.top+.8*g.height);s=await read(p);check(width+' full game lens',s.target>.999&&s.opacity>.999&&s.filter.includes('blur(24px)'));
  if(width===1440)await shot(p,'1440-light-game');
  await scroll(p,g.record.top+.8*g.height);s=await read(p);check(width+' music lens',s.target>.999&&s.opacity>.999);
  if(width===390)await shot(p,'390-dark-music');
  await p.locator('.floating-theme>summary').click();check(width+' theme menu usable',await p.locator('.floating-theme').evaluate(e=>e.open));
  await p.locator('[data-solid-toggle]').click();await settle(p);check(width+' solid preference respected',await p.locator('.floating-controls').evaluate(e=>getComputedStyle(e,'::before').display==='none'));
  await p.locator('[data-solid-toggle]').click();await p.keyboard.press('Escape');
  await scroll(p,g.record.bottom-(g.rail.top+g.rail.height*.5));s=await read(p);check(width+' fractional lens leaving',s.target>.1&&s.target<.9&&Math.abs(s.opacity-s.target)<.005);
  await scroll(p,g.record.bottom-g.rail.top+48);s=await read(p);check(width+' restored original rail',s.target===0&&s.opacity===0);
  check(width+' no horizontal overflow',await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  if(width===1440)await shot(p,'1440-light-after-exit');
  await p.goto(base+'/games');await settle(p);check(width+' other page not enhanced',!await p.locator('.floating-controls').evaluate(e=>e.classList.contains('has-media-glass')));
  await c.close();
 }
 const c=await browser.newContext(),manifest=JSON.parse(fs.readFileSync('web/assets/.vite/manifest.json'));let assets=0;
 for(const v of Object.values(manifest).filter(v=>v.isEntry||v.src==='src/homeControlGlass.ts'||v.src==='src/homeShowcase.ts'))for(const f of[v.file,...v.css||[]]){const r=await c.request.get(base+'/assets/'+f);check('current served asset '+f,r.ok()&&crypto.createHash('sha256').update(await r.body()).digest('hex')===crypto.createHash('sha256').update(fs.readFileSync('web/assets/'+f)).digest('hex'));assets++}
 check('PostgreSQL ready',(await c.request.get(base+'/readyz').then(r=>r.json())).status==='ready');check('no runtime errors',!errors.length);check('no business write attempts',!blocked.length);
 const report={passed:true,origin:base,checks,errors,blocked,captures,assets,business_writes:0};fs.writeFileSync(out+'/lan-smoke.json',JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({passed:true,checks:checks.length,captures:captures.length,assets,business_writes:0}));await browser.close();
})().catch(async e=>{fs.writeFileSync(out+'/lan-smoke-failure.json',JSON.stringify({error:String(e),checks,errors,blocked},null,2));console.error(String(e));await browser?.close();process.exitCode=1});
