// Read-only public checks and one batched visual inspection matrix.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const fs=require('fs'),path=require('path');
const base=process.env.BASE_URL||'http://192.168.2.209:18081',out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v4/public');
fs.mkdirSync(out,{recursive:true});
const checks=[],errors=[],captures=[],metrics=[];let browser;
const check=(name,ok)=>{checks.push({check:name,passed:!!ok});if(!ok)throw Error(name)};
async function shot(p,name){await p.screenshot({path:path.join(out,name+'.png')});captures.push(name+'.png')}
async function stage(p,progress){await p.evaluate(progress=>{const e=document.querySelector('[data-home-story]');scrollTo(0,e.offsetTop+(e.offsetHeight-innerHeight)*progress)},progress);await p.waitForTimeout(180)}
async function settle(p){await p.locator('[data-home-story][data-motion]').waitFor();await p.waitForTimeout(100)}
(async()=>{
 browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--enable-unsafe-swiftshader']});
 const ctx=await browser.newContext({viewport:{width:1440,height:1000}}),p=await ctx.newPage();p.on('pageerror',e=>errors.push(e.message));
 await p.addInitScript(()=>{window.__lensDraws=0;const draw=WebGLRenderingContext.prototype.drawArrays;WebGLRenderingContext.prototype.drawArrays=function(...a){window.__lensDraws++;return draw.apply(this,a)}});
 await p.goto(base+'/');await settle(p);
 check('home starts with configured identity and no archive in first viewport',await p.locator('.story-title').isVisible()&&await p.locator('.home-writing').evaluate(e=>e.getBoundingClientRect().top>=innerHeight));
 check('native scroll story initializes a real WebGL renderer',await p.locator('[data-home-story]').getAttribute('data-renderer')==='webgl');
 check('home includes at most two real articles and six tags',await p.locator('.home-writing-entry').count()<=2&&await p.locator('.home-tag-index a').count()<=6);
 const initial=await p.locator('[data-story-scene]').first().evaluate(e=>getComputedStyle(e).transform);
 await stage(p,.42);check('scroll changes scene transforms and chapter',await p.locator('[data-home-story]').getAttribute('data-chapter')==='intro'&&initial!==await p.locator('[data-story-scene]').first().evaluate(e=>getComputedStyle(e).transform));
 await stage(p,.83);check('writing chapter becomes visible and inactive links are inert',await p.locator('[data-home-story]').getAttribute('data-chapter')==='writing'&&await p.locator('[data-story-scene]').first().evaluate(e=>e.inert));
 const drawCount=await p.evaluate(()=>window.__lensDraws);await p.waitForTimeout(300);check('lens has no idle rendering loop',await p.evaluate(()=>window.__lensDraws)===drawCount);
 check('desktop shader pixel budget is bounded',await p.locator('.story-art canvas').evaluate(e=>e.width*e.height<=1002000));
 await p.evaluate(()=>scrollTo(0,0));await p.waitForTimeout(100);
 const trigger=p.locator('[data-search-open]'),hint=p.locator('.search-shortcut-hint');
 await p.mouse.move(0,0);check('search shortcut tooltip is hidden at rest',!await hint.isVisible());await trigger.hover();await hint.waitFor();check('search shortcut appears on hover',await hint.isVisible());
 await p.mouse.move(0,0);await trigger.focus();check('search shortcut appears on keyboard focus',await hint.isVisible());await p.keyboard.press('Control+k');await p.locator('#quick-search').waitFor();check('Ctrl K still opens the search dialog',await p.locator('#site-search').evaluate(e=>e.open));await p.keyboard.press('Escape');
 const theme=p.locator('.floating-theme');await theme.locator('summary').click();
 check('theme opens own three-choice panel without native select',await theme.locator('[role=radio]').count()===3&&await theme.locator('select').count()===0);
 await theme.locator('[data-theme-option=dark]').click();check('theme button applies dark immediately',await p.locator('html').getAttribute('data-theme')==='dark');
 await p.keyboard.press('ArrowUp');check('theme choices support arrow keys',await theme.locator('[data-theme-option=light]').getAttribute('aria-checked')==='true');
 await p.keyboard.press('Escape');check('Escape closes panel and returns focus',!await theme.evaluate(e=>e.open)&&await theme.locator('summary').evaluate(e=>e===document.activeElement));
 await theme.locator('summary').click();await theme.locator('[data-theme-option=system]').click();await p.emulateMedia({colorScheme:'dark'});await p.waitForFunction(()=>document.documentElement.dataset.theme==='dark');check('system theme follows operating system changes',await p.locator('html').getAttribute('data-theme')==='dark');await p.mouse.click(10,100);check('outside click closes theme panel',!await theme.evaluate(e=>e.open));
 await p.goto(base+'/posts');const article=await p.locator('.post-row h2 a').first().getAttribute('href');check('real published article is available',!!article);await p.goto(base+article);
 const header=await p.locator('.site-header').evaluate(e=>{const s=getComputedStyle(e);return {top:e.getBoundingClientRect().top,bg:s.backgroundColor,shadow:s.boxShadow,filter:s.backdropFilter}});
 check('article top has no white plate or outer band',header.top===0&&header.bg==='rgba(0, 0, 0, 0)'&&header.shadow==='none'&&header.filter==='none');
 check('article retains its primary navigation',await p.locator('.site-nav a').allTextContents().then(x=>x.join(',')==='文章,标签'));
 await ctx.close();
 // Capture every state in both themes and shipping sizes before making any visual fixes.
 for(const [size,width,height] of [['desktop',1440,1000],['mobile',390,900]])for(const mode of ['light','dark']){
  const c=await browser.newContext({viewport:{width,height},colorScheme:mode}),page=await c.newPage();page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(mode=>localStorage.setItem('painter-theme',mode),mode);await page.goto(base+'/');await settle(page);
  for(const [name,progress] of [['start',0],['intro',.43],['writing',.84]]){await stage(page,progress);await shot(page,`${size}-${mode}-home-${name}`)}
  metrics.push({size,theme:mode,canvas:await page.locator('canvas').evaluate(e=>({width:e.width,height:e.height})),overflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)});
  await page.locator('#home-writing').scrollIntoViewIfNeeded();await page.waitForTimeout(100);await shot(page,`${size}-${mode}-home-links`);
  await page.locator('.home-close').scrollIntoViewIfNeeded();await shot(page,`${size}-${mode}-home-close`);
  await page.goto(base+article);await shot(page,`${size}-${mode}-article`);
  await page.locator('.floating-theme summary').click();await shot(page,`${size}-${mode}-theme`);await page.keyboard.press('Escape');
  await page.locator('[data-search-open]').hover();await page.waitForTimeout(180);await shot(page,`${size}-${mode}-search-hint`);
  await page.locator('[data-search-open]').click();await page.locator('#site-search [role=option]').first().waitFor();await shot(page,`${size}-${mode}-search-dialog`);
  await c.close();
 }
 check('all public sizes and themes have no horizontal overflow',metrics.every(x=>!x.overflow));
 const reduced=await browser.newContext({viewport:{width:390,height:900},reducedMotion:'reduce'}),r=await reduced.newPage();await r.goto(base+'/');await settle(r);
 check('reduced motion shows normal static chapters',await r.locator('[data-home-story]').getAttribute('data-motion')==='static'&&await r.locator('[data-story-scene]').evaluateAll(es=>es.every(e=>getComputedStyle(e).position==='relative'&&!e.inert)));
 await shot(r,'mobile-reduced-motion');await reduced.close();
 const fallback=await browser.newContext(),f=await fallback.newPage();await f.addInitScript(()=>{const original=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){return type==='webgl'?null:original.call(this,type,...args)}});await f.goto(base+'/');await settle(f);check('WebGL failure uses local static fallback and readable content',await f.locator('[data-home-story]').getAttribute('data-renderer')==='static'&&await f.locator('.story-fallback img').isVisible());await fallback.close();
 const nojs=await browser.newContext({javaScriptEnabled:false,viewport:{width:390,height:900}}),n=await nojs.newPage();await n.goto(base+'/');check('without JavaScript all three chapters and real links remain readable',await n.locator('[data-story-scene]').count()===3&&await n.locator('[data-story-scene]').evaluateAll(es=>es.every(e=>getComputedStyle(e).position==='relative'&&getComputedStyle(e).visibility==='visible')));await n.goto(base+article);check('article remains rendered without JavaScript',await n.locator('.prose').innerText().then(s=>s.trim().length>0));await n.goto(base+'/search?q=中文');check('search results filters work as an HTML form without JavaScript',await n.locator('.filter-bar form,form.filter-bar').count()===1);await nojs.close();
 check('no browser runtime errors',errors.length===0);
 fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({origin:base,article,checks,errors,metrics,captures,renderer_note:'Headless Chromium uses --enable-unsafe-swiftshader; WebGL and deliberate no-WebGL fallback both checked.'},null,2));console.log(JSON.stringify({checks:checks.length,allPassed:true,captures:captures.length}));await browser.close();
})().catch(async e=>{fs.writeFileSync(path.join(out,'report-failed.json'),JSON.stringify({checks,errors,captures,failure:e.stack},null,2));console.error(e.stack);await browser?.close();process.exitCode=1});
