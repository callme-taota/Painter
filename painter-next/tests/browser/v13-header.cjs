// Direction-aware navigation proof. All business writes are blocked; screenshots
// are opt-in. Requires a published article and an authenticated SSR studio shell.
// BASE_URL=http://127.0.0.1:18081 OWNER_STATE_FILE=/private/owner-state.json \
// PLAYWRIGHT_MODULE=/path/to/playwright CHROMIUM_EXECUTABLE=/usr/bin/google-chrome \
// node tests/browser/v13-header.cjs
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright')
const fs=require('fs'),path=require('path')
const base=(process.env.BASE_URL||'http://127.0.0.1:18081').replace(/\/$/,''),origin=new URL(base).origin
const out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v13/header')
if(!process.env.OWNER_STATE_FILE)throw Error('Private OWNER_STATE_FILE required for studio SSR')
const storageState=JSON.parse(fs.readFileSync(process.env.OWNER_STATE_FILE,'utf8'))
const secrets=[...storageState.cookies.map(c=>c.value),...(storageState.origins||[]).flatMap(o=>(o.localStorage||[]).map(item=>item.value))].filter(s=>s&&s.length>=8)
const scrub=value=>secrets.reduce((s,secret)=>s.split(secret).join('[redacted]'),String(value))
let browser,scenario='startup',articlePath=process.env.ARTICLE_PATH
const checks=[],metrics=[],errors=[],blocked=[],mocked=[],captures=[]
function check(name,passed,detail){checks.push({scenario,name,passed:!!passed,...(detail===undefined?{}:{detail})});if(!passed)throw Error(scenario+': '+name)}
async function context(width=1440,options={}){
 const c=await browser.newContext({storageState,viewport:{width,height:900},colorScheme:'light',serviceWorkers:'block',...options})
 await c.addInitScript(()=>localStorage.setItem('painter-theme','light'))
 await c.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url()),method=request.method()
  if(url.origin!==origin)return route.continue()
  if(url.pathname.startsWith('/api/v1/')){
   if(method==='GET'&&url.pathname==='/api/v1/session')return route.fulfill({status:200,json:{authenticated:true,user:{id:'1',role:'owner',display_name:'导航验收'},csrf_token:'mock-header-proof'}})
   if(method==='GET'&&url.pathname==='/api/v1/me/posts')return route.fulfill({status:200,json:{items:Array.from({length:30},(_,i)=>({id:'header-proof-'+i,title:'导航验收文章 '+(i+1),status:'draft',version:1,draft_version:1,updated_at:'2026-11-02T00:00:00Z'})),has_more:false}})
   if(method==='GET'&&url.pathname==='/api/v1/search/suggestions')return route.fulfill({status:200,json:{items:[]}})
   if(url.pathname==='/api/v1/analytics/events'){mocked.push({scenario,method,path:url.pathname});return route.fulfill({status:200,json:{accepted:0}})}
   if(!['GET','HEAD'].includes(method)){blocked.push({scenario,method,path:url.pathname});return route.abort('blockedbyclient')}
  }
  return route.continue()
 })
 const p=await c.newPage();p.setDefaultTimeout(8000);p.on('pageerror',e=>errors.push({scenario,message:scrub(e.message)}))
 return {c,p}
}
async function settle(page){await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))}
async function y(page,value){await page.evaluate(top=>scrollTo({top,behavior:'instant'}),value);await settle(page)}
async function overscroll(page,value){await page.evaluate(top=>{window.__headerProofScrollY=Object.getOwnPropertyDescriptor(window,'scrollY');Object.defineProperty(window,'scrollY',{configurable:true,value:top});dispatchEvent(new Event('scroll'))},value);await settle(page)}
async function restoreScroll(page){await page.evaluate(()=>{Object.defineProperty(window,'scrollY',window.__headerProofScrollY);delete window.__headerProofScrollY;dispatchEvent(new Event('scroll'))});await settle(page)}
async function room(page){await page.evaluate(()=>{if(document.documentElement.scrollHeight<innerHeight+1800){const spacer=document.createElement('div');spacer.dataset.headerProofSpace='';spacer.setAttribute('aria-hidden','true');spacer.style.height='2200px';document.querySelector('main').append(spacer)}});await settle(page)}
async function state(page,selector='.site-header'){return page.locator(selector).evaluate(el=>{const r=el.getBoundingClientRect(),s=getComputedStyle(el);return{hidden:el.dataset.scrollHidden==='true',inert:el.inert,aria:el.getAttribute('aria-hidden'),top:r.top,bottom:r.bottom,height:r.height,position:s.position,visibility:s.visibility,transition:s.transitionDuration,documentHeight:document.documentElement.scrollHeight,mainStart:document.querySelector('main').getBoundingClientRect().top+scrollY}})}
async function shot(page,name){if(process.env.CAPTURE_SCREENSHOTS!=='1')return;await page.screenshot({path:path.join(out,name+'.png'),animations:'disabled'});captures.push(name+'.png')}
async function exercise(page,selector='.site-header'){
 await page.locator(selector+'[data-scroll-header]').waitFor();await room(page);await y(page,0)
 const initial=await state(page,selector);metrics.push({scenario,state:'initial',...initial})
 check('visible native sticky header starts in normal flow',!initial.hidden&&!initial.inert&&initial.position==='sticky'&&initial.height>40)
 await y(page,400);let s=await state(page,selector)
 check('downward scroll hides the header and its tab stops',s.hidden&&s.inert&&s.aria==='true')
 await page.waitForTimeout(230);s=await state(page,selector)
 check('hidden navigation is outside the viewport and not painted',s.bottom<=0&&s.visibility==='hidden')
 check('hiding preserves document height and main position',s.documentHeight===initial.documentHeight&&Math.abs(s.mainStart-initial.mainStart)<1)
 for(const top of[399,400,398,400,399])await y(page,top)
 check('tiny alternating upward jitter leaves it hidden',(await state(page,selector)).hidden)
 await y(page,380);s=await state(page,selector)
 check('scrolling upward reveals accessible navigation',!s.hidden&&!s.inert&&s.aria!=='true')
 for(const top of[381,380,382,381,383])await y(page,top)
 check('tiny alternating downward jitter leaves it visible',!(await state(page,selector)).hidden)
 for(let top=386;top<=422;top+=3)await y(page,top)
 check('small cumulative downward movements cross the hide threshold',(await state(page,selector)).hidden)
 for(let top=419;top>=398;top-=3)await y(page,top)
 check('small cumulative upward movements cross the reveal threshold',!(await state(page,selector)).hidden)
 await y(page,500);check('header can hide again',(await state(page,selector)).hidden)
 await page.keyboard.press('Tab');check('Tab reveals navigation before native focus advances',!(await state(page,selector)).hidden&&!(await state(page,selector)).inert)
 await y(page,0);await page.locator(selector+' a').first().evaluate(el=>el.focus({preventScroll:true}));await y(page,400)
 check('focused header controls remain visible',!(await state(page,selector)).hidden)
 await page.evaluate(()=>document.activeElement?.blur());await y(page,450)
 check('normal hiding resumes after focus leaves',(await state(page,selector)).hidden)
 const bottom=await page.evaluate(()=>document.documentElement.scrollHeight-innerHeight);await y(page,bottom);await overscroll(page,bottom+90);await restoreScroll(page)
 check('bottom overscroll bounce cannot produce a false upward reveal',(await state(page,selector)).hidden)
 await y(page,0);check('the top always reveals navigation',!(await state(page,selector)).hidden)
 await overscroll(page,-90);await restoreScroll(page)
 check('top overscroll keeps the header visible',!(await state(page,selector)).hidden)
 await shot(page,scenario)
}
async function publicPages(width){
 scenario='public-'+width
 const {c,p}=await context(width);await p.goto(base+'/posts');await p.locator('.site-header[data-scroll-header]').waitFor()
 if(!articlePath)articlePath=await p.locator('.post-row h2 a').first().getAttribute('href')
 await exercise(p)
 await p.getByRole('navigation',{name:'主导航'}).getByRole('link',{name:'标签',exact:true}).click();await p.waitForURL(base+'/tags')
 check('real primary navigation loads another enhanced page',await p.locator('.site-header[data-scroll-header]').count()===1)
 await room(p);await y(p,400);check('navigated page hides on downward scroll',(await state(p)).hidden)
 await p.keyboard.press('Control+k');await p.locator('#site-search[open]').waitFor();await settle(p);await y(p,600)
 check('open native search dialog reveals and holds the header',!(await state(p)).hidden)
 await p.keyboard.press('Escape');await p.locator('#site-search[open]').waitFor({state:'hidden'});await p.evaluate(()=>document.activeElement?.blur());await y(p,680)
 check('closing search permits direction behavior again',(await state(p)).hidden)
 await p.evaluate(()=>{const menu=document.createElement('details');menu.dataset.headerProofMenu='';menu.innerHTML='<summary>导航验收</summary><nav><a href="/posts">文章</a></nav>';document.querySelector('.site-header').append(menu);menu.open=true});await settle(p);await y(p,760)
 check('an open native header menu holds navigation visible',!(await state(p)).hidden)
 await p.evaluate(()=>document.querySelector('[data-header-proof-menu]').remove());await settle(p);await y(p,820)
 check('closing the menu restores normal hiding',(await state(p)).hidden)
 for(const route of['/phrases','/search','/about','/login','/register','/setup',articlePath]){
  scenario='route-'+width+'-'+route;await p.goto(base+route);await p.locator('.site-header[data-scroll-header]').waitFor();await room(p);await y(p,350)
  check('route hides on down',(await state(p)).hidden);await y(p,320);check('route reveals on up',!(await state(p)).hidden)
  if(route===articlePath)check('article navigation stays readable over prose',await p.locator('.site-header').evaluate(el=>getComputedStyle(el).backgroundColor!=='rgba(0, 0, 0, 0)'))
 }
 scenario='home-exception-'+width;await p.goto(base+'/');await p.locator('[data-home-story]').waitFor();const homeBefore=await state(p);await y(p,400);await y(p,370);const homeAfter=await state(p)
 check('homepage retains its own header behavior',await p.locator('.site-header[data-scroll-header]').count()===0&&homeBefore.position==='absolute'&&homeAfter.position==='absolute'&&!homeAfter.inert&&homeAfter.aria!=='true')
 await c.close()
}
async function studio(){
 scenario='studio-mobile';const {c,p}=await context(390,{isMobile:true,hasTouch:true});await p.goto(base+'/studio/posts');await p.getByText('导航验收文章 30',{exact:true}).waitFor()
 check('studio uses only its mobile workbench bar',await p.locator('.site-header').count()===0)
 await exercise(p,'.studio-mobile-bar');await y(p,400);await p.keyboard.press('Tab');await y(p,0);await p.getByRole('button',{name:'打开工作台导航',exact:true}).click();await p.locator('.studio-drawer[open]').waitFor();await y(p,600)
 check('native studio drawer holds the mobile header visible',!(await state(p,'.studio-mobile-bar')).hidden)
 await p.getByRole('button',{name:'关闭工作台导航',exact:true}).click();await p.evaluate(()=>document.activeElement?.blur());await y(p,680)
 check('closing studio drawer restores hiding',(await state(p,'.studio-mobile-bar')).hidden)
 await p.setViewportSize({width:1440,height:900});await settle(p)
 check('desktop studio has no extra public or mobile header',await p.locator('.site-header,.studio-mobile-bar').count()===0)
 await p.setViewportSize({width:390,height:900});await p.locator('.studio-mobile-bar[data-scroll-header]').waitFor();await settle(p)
 check('mobile bar reattaches visibly after breakpoint change',!(await state(p,'.studio-mobile-bar')).hidden)
 await c.close()
}
async function resilience(){
 scenario='initial-midpage';let {c,p}=await context()
 await c.addInitScript(()=>addEventListener('DOMContentLoaded',()=>{document.querySelector('main').style.minHeight='3000px';scrollTo({top:600,behavior:'instant'})},{once:true}))
 await p.goto(base+'/tags');await p.locator('.site-header[data-scroll-header]').waitFor();await settle(p)
 check('initial midpage load starts with visible navigation',await p.evaluate(()=>scrollY>=500)&&!(await state(p)).hidden)
 await y(p,650);check('midpage loading still observes the next downward movement',(await state(p)).hidden)
 await c.close()
 scenario='reduced-motion';({c,p}=await context(390,{reducedMotion:'reduce'}));await p.goto(base+'/tags');await p.locator('.site-header[data-scroll-header]').waitFor();await room(p);await y(p,400)
 check('reduced motion uses immediate direction changes',(await state(p)).hidden&&(await state(p)).transition.split(',').every(x=>parseFloat(x)===0));await y(p,380);check('reduced motion still reveals on upward movement',!(await state(p)).hidden);await c.close()
 scenario='no-javascript';({c,p}=await context(390,{javaScriptEnabled:false}));await p.goto(base+'/posts');const s=await state(p)
 check('server-rendered navigation is visible without JavaScript',s.height>40&&s.top>=0&&s.visibility==='visible'&&!s.inert&&await p.locator('.site-header[data-scroll-header]').count()===0);await c.close()
}
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--disable-dev-shm-usage']})
 for(const width of[1440,390])await publicPages(width)
 await studio();await resilience();scenario='summary';check('no attempted business writes',blocked.length===0);check('no browser errors',errors.length===0)
 fs.rmSync(path.join(out,'failure.json'),{force:true});fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({checks,metrics,errors,blocked,mocked,captures,actual_business_writes:0,passed:true},null,2));console.log(JSON.stringify({checks:checks.length,captures:captures.length,passed:true}));await browser.close()
})().catch(async e=>{fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'failure.json'),JSON.stringify({checks,metrics,errors,blocked,mocked,captures,error:scrub(e)},null,2));console.error(scrub(e));await browser?.close();process.exitCode=1})
