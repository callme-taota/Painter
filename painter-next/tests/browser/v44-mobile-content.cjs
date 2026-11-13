// Real Chromium rendering and synthesized touch against the isolated V44 app.
// No production origin, account, key or provider is accessed by this test.
const fs=require('fs'),path=require('path');
const {chromium}=require('/tmp/painter-browser/node_modules/playwright');
const base='http://127.0.0.1:18087',out=path.resolve('tests/artifacts/v44/mobile-content');
const fixture=JSON.parse(fs.readFileSync('/tmp/painter-v44-private/mobile-fixtures.json','utf8'));
if(fixture.origin!==base||fixture.isolated_database!=='painter_v44_mobile_20261113')throw Error('Disposable V44 fixture guard failed');
const report={origin:base,isolated_database:fixture.isolated_database,checks:[],screenshots:[],errors:[],limitations:['Chrome with emulated phone viewports and synthesized CDP touch input. This is not physical iPhone/Android or WebKit evidence.','Collections are explicitly labelled disposable fixtures; screenshots do not represent real personal game/music accounts.']};
fs.mkdirSync(out,{recursive:true});let browser;
function check(name,passed,detail){report.checks.push({name,passed:!!passed,...(detail===undefined?{}:{detail})});console.log((passed?'PASS ':'FAIL ')+name);if(!passed)throw Error(name)}
function save(){fs.writeFileSync(path.join(out,'verification.json'),JSON.stringify(report,null,2))}
async function ready(page){await page.evaluate(()=>document.fonts.ready);await page.waitForTimeout(120);await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))))}
async function capture(page,name,fullPage=false){await ready(page);await page.screenshot({path:path.join(out,name+'.png'),fullPage,animations:'disabled'});report.screenshots.push(name+'.png')}
async function navigate(page,route){await page.goto(base+route,{waitUntil:'domcontentloaded'});await ready(page)}
async function noOverflow(page,name){check(name+' keeps overflow inside content tracks',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),await page.evaluate(()=>({viewport:innerWidth,document:document.documentElement.scrollWidth})))}
async function context(width,height=844,theme='light',reducedMotion='no-preference'){
 const ctx=await browser.newContext({viewport:{width,height},isMobile:width<=720,hasTouch:width<=720,colorScheme:theme,reducedMotion,locale:'zh-CN',serviceWorkers:'block'});
 await ctx.addInitScript(theme=>{localStorage.setItem('painter-theme',theme);localStorage.setItem('painter-analytics','off')},theme);
 const page=await ctx.newPage();page.on('pageerror',error=>report.errors.push(error.message));return{ctx,page};
}
async function shelfGesture(page,selector){
 const target=page.locator(selector);await target.evaluate(element=>{element.scrollLeft=0;window.scrollTo({top:window.scrollY+element.getBoundingClientRect().top-190,behavior:'instant'})});await ready(page);
 const box=await target.boundingBox();if(!box)throw Error('Missing touch shelf');const x=Math.min(330,box.x+box.width-24),y=Math.min(510,Math.max(225,box.y+Math.min(160,box.height/2)));
 const client=await page.context().newCDPSession(page);
 const touch=async(type,x,y)=>client.send('Input.dispatchTouchEvent',{type,touchPoints:type==='touchEnd'?[]:[{x,y}],modifiers:0});
 const settled=()=>target.evaluate(element=>new Promise(resolve=>{let last=element.scrollLeft,stable=0;const started=performance.now();function sample(){const current=element.scrollLeft;stable=Math.abs(current-last)<.1?stable+1:0;last=current;if(stable>=12||performance.now()-started>3000)resolve();else requestAnimationFrame(sample)}requestAnimationFrame(sample)}));
 await touch('touchStart',x,y);for(let i=1;i<=9;i++){await touch('touchMove',x-i*25,y);await page.waitForTimeout(18)}await touch('touchEnd',0,0);await page.waitForTimeout(350);await settled();
 check(selector+' responds to a horizontal touch gesture',await target.evaluate(element=>element.scrollLeft>40));
 // Test the page-axis gesture independently of the prior horizontal flick's
 // compositor scroll latch. This also keeps the late, bottom-of-page shelf
 // test independent of momentum and snap positions from previous collections.
 await page.reload({waitUntil:'domcontentloaded'});await ready(page);await page.locator('[data-work-journey-initialized]').waitFor();
 await target.evaluate(element=>{element.scrollLeft=0;window.scrollTo({top:window.scrollY+element.getBoundingClientRect().top-190,behavior:'instant'})});await ready(page);
 const verticalBox=await target.boundingBox(),verticalY=Math.min(510,Math.max(225,verticalBox.y+Math.min(160,verticalBox.height/2)));
 const before=await page.evaluate(()=>scrollY),direction=await page.evaluate(()=>document.documentElement.scrollHeight-innerHeight-scrollY>120?1:-1);
 await touch('touchStart',190,verticalY);for(let i=1;i<=8;i++){await touch('touchMove',190,verticalY-i*23*direction);await page.waitForTimeout(18)}await touch('touchEnd',0,0);await page.waitForTimeout(350);
 check(selector+' allows a vertical swipe to scroll the page',await page.evaluate(({before,direction})=>(scrollY-before)*direction>80,{before,direction}));
 check(selector+' vertical swipe does not switch collection items',await target.evaluate(element=>element.scrollLeft<15),await target.evaluate(element=>({scrollLeft:element.scrollLeft})));
 await client.detach();
}
(async()=>{try{
 browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 for(const [width,height,theme] of [[320,740,'light'],[390,844,'light'],[430,932,'dark'],[710,390,'light']]){
  const {ctx,page}=await context(width,height,theme);await navigate(page,'/');await page.locator('[data-work-journey-initialized]').waitFor();
  check(width+' home runs the phone chapter model',await page.evaluate(()=>!document.querySelector('.work-journey-ready,.home-game-story.is-enhanced')&&!document.documentElement.classList.contains('story-ready')));
  check(width+' all home chapters remain available',await page.locator('[data-home-section]').count()===8);
  check(width+' project cards remain interactive',await page.locator('[data-project-card]').evaluateAll(cards=>cards.every(card=>!card.inert&&card.getAttribute('aria-hidden')!=='true'&&card.offsetHeight>0)));
  check(width+' all game scenes remain available',await page.locator('[data-game-frame]').evaluateAll(frames=>frames.length===3&&frames.every(frame=>!frame.inert&&getComputedStyle(frame).position==='relative')));
  await noOverflow(page,width+' home');await capture(page,width+'-'+theme+'-home');
  await page.locator('#home-music').evaluate(element=>window.scrollTo({top:scrollY+element.getBoundingClientRect().top-90,behavior:'instant'}));await capture(page,width+'-'+theme+'-collections');
  const records=await page.locator('.home-record-tile').evaluateAll(items=>items.map(item=>getComputedStyle(item.querySelector('span')).opacity));check(width+' collection labels require no hover',records.length===13&&records.every(value=>value==='1'));
  const dayBefore=await page.locator('.activity-mobile-navigation span').innerText();await page.getByRole('button',{name:'上一天',exact:true}).click();check(width+' publishing calendar has full-size day controls',await page.getByRole('button',{name:'上一天',exact:true}).evaluate(button=>button.getBoundingClientRect().width>=44&&button.getBoundingClientRect().height>=44));check(width+' day controls update the real date',dayBefore!==await page.locator('.activity-mobile-navigation span').innerText());
  await navigate(page,'/posts/'+fixture.article_id);await noOverflow(page,width+' article');check(width+' article keeps a comfortable reading size',await page.locator('.reading-paper>.prose').evaluate(element=>parseFloat(getComputedStyle(element).fontSize)>=17));
  check(width+' author identity does not overlap the cover',await page.evaluate(()=>document.querySelector('.article-rail').getBoundingClientRect().bottom<=document.querySelector('.reading-paper').getBoundingClientRect().top));
  check(width+' internal references render as cards',await page.locator('.prose .content-card--moment,.prose .content-card--phrase').count()===2);
  await page.locator('.article-info>summary').click();check(width+' metadata disclosure stays inside viewport',await page.locator('.article-info-panel').evaluate(panel=>{const r=panel.getBoundingClientRect();return r.left>=-1&&r.right<=innerWidth+1}));await page.locator('.article-info>summary').click();
  await page.evaluate(()=>scrollTo(0,0));await capture(page,width+'-'+theme+'-article');
  await page.locator('.article-discussion').scrollIntoViewIfNeeded();await page.locator('.discussion-comment').first().waitFor();check(width+' lazy-loaded comments retain phone reading size',await page.locator('.discussion-body').first().evaluate(element=>parseFloat(getComputedStyle(element).fontSize)>=16));await capture(page,width+'-'+theme+'-discussion');
  await navigate(page,'/users/'+fixture.author_id);await noOverflow(page,width+' author');check(width+' author search stays in the author page',await page.locator('.author-search').getAttribute('action')==='/users/'+fixture.author_id);await capture(page,width+'-'+theme+'-author');
  await navigate(page,'/games');await noOverflow(page,width+' games');check(width+' phone library keeps two cover columns',await page.locator('.games-library-grid').evaluate(element=>getComputedStyle(element).gridTemplateColumns.split(' ').length===2));await capture(page,width+'-'+theme+'-games');
  await navigate(page,'/moments');await noOverflow(page,width+' moments');check(width+' moments use a single readable phone feed',await page.locator('.moments-items').evaluate(element=>getComputedStyle(element).gridTemplateColumns.split(' ').length===1));await capture(page,width+'-'+theme+'-moments');
  await navigate(page,'/phrases');await noOverflow(page,width+' phrases');await capture(page,width+'-'+theme+'-phrases');await ctx.close();
 }
 const {ctx,page}=await context(390);await navigate(page,'/');await page.locator('[data-work-journey-initialized]').waitFor();
 for(const selector of ['.project-deck','.home-record-grid','.home-game-window','.home-writing-viewport'])await shelfGesture(page,selector);
 check('phone scrolling never starts document magnet snapping',await page.evaluate(()=>!document.body.dataset.homeSnapping));
 await page.setViewportSize({width:1024,height:900});await page.waitForFunction(()=>!!document.querySelector('.work-journey-ready,.home-game-story.is-enhanced'));check('desktop enhancement returns after crossing the layout boundary',true);await page.setViewportSize({width:390,height:844});await page.waitForFunction(()=>!document.querySelector('.work-journey-ready,.home-game-story.is-enhanced'));check('switching back restores every phone game and project',await page.locator('[data-project-card],[data-game-frame]').evaluateAll(elements=>elements.every(element=>!element.inert&&element.getAttribute('aria-hidden')!=='true')));await ctx.close();
 const reduced=await context(390,844,'dark','reduce');await navigate(reduced.page,'/');check('reduced motion retains all natural-flow phone chapters',await reduced.page.locator('[data-home-section]').count()===8&&!await reduced.page.locator('.work-journey-ready,.home-game-story.is-enhanced').count());await noOverflow(reduced.page,'reduced-motion home');await reduced.ctx.close();
 check('public content has no browser JavaScript errors',report.errors.length===0,report.errors);report.complete=true;save();await browser.close();console.log(JSON.stringify({complete:true,checks:report.checks.length,screenshots:report.screenshots.length}));
}catch(error){report.complete=false;report.failure=error.message;save();console.error('V44 content check failed: '+error.message);await browser?.close();process.exitCode=1}})();
