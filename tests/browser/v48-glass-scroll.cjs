// Read-only mobile scroll regression against the isolated, real Painter HTML.
// This measures real browser rAF intervals. It is not a physical-phone FPS test.
const fs=require('node:fs');
const path=require('node:path');
const playwright=require(process.env.PLAYWRIGHT_MODULE||'/tmp/painter-v47-webkit-tools/node_modules/playwright');
const base=(process.env.PAINTER_BASE||'http://127.0.0.1:18090').replace(/\/$/,'');
if(base!=='http://127.0.0.1:18090')throw Error('Use only the isolated :18090 candidate; this test never authenticates or changes content.');
const out=path.resolve(process.env.PAINTER_V48_ARTIFACTS||'tests/artifacts/v48/scroll');
const engines=(process.env.PAINTER_V48_ENGINES||'chromium,webkit').split(',');
const report={base,checks:[],runs:[],screenshots:[],errors:[],csp:[],httpErrors:[],blockedWrites:[],limitations:[
 '390x844 touch viewport in desktop Chromium/Linux WebKit; no physical iPhone/Android claim.',
 'Scroll positions are changed in actual requestAnimationFrame callbacks. Intervals come from browser timestamps, never mocked clocks.',
 'rAF cadence is a main-thread rendering callback measurement, not GPU presentation time or input-to-photon latency.',
 'Chrome CPU throttling is a developer-tools stress condition. Its timings do not predict a particular phone or guarantee FPS.',
 'Mutation attribution covers the decorative optics subtree; unrelated header, lazy-image, and page animation callbacks are not labelled as glass work.'
]};
function check(name,passed,detail){report.checks.push({name,passed:!!passed,...(detail===undefined?{}:{detail})});console.log(`${passed?'PASS':'FAIL'} ${name}`);}
function installProbe(){
 const state={styleWrites:0,treeWrites:0,sourceWrites:0,mirrorsEver:0,definitionsEver:0,canvasExports:0,webglRequests:0,scrollEvents:0};
 const optics=node=>node instanceof Element?!!node.closest('.liquid-glass-optics'):node?.parentElement?!!node.parentElement.closest('.liquid-glass-optics'):false;
 const inspect=node=>{
  if(!(node instanceof Element))return;
  state.mirrorsEver+=(node.matches('.liquid-glass-media')?1:0)+node.querySelectorAll('.liquid-glass-media').length;
  state.definitionsEver+=(node.matches('.liquid-glass-definitions,filter[id^="painter-liquid-glass-"]')?1:0)+node.querySelectorAll('.liquid-glass-definitions,filter[id^="painter-liquid-glass-"]').length;
 };
 const watch=()=>new MutationObserver(records=>{
  for(const record of records){
   if(record.type==='childList'){
    if(optics(record.target))state.treeWrites+=record.addedNodes.length+record.removedNodes.length;
    record.addedNodes.forEach(inspect);
   }else if(optics(record.target)){
    if(record.attributeName==='style')state.styleWrites++;
    else state.sourceWrites++;
   }
  }
 }).observe(document.documentElement,{subtree:true,childList:true,attributes:true,attributeFilter:['style','src','srcset']});
 if(document.documentElement)watch();else document.addEventListener('DOMContentLoaded',watch,{once:true});
 const exportCanvas=HTMLCanvasElement.prototype.toDataURL;
 HTMLCanvasElement.prototype.toDataURL=function(...args){state.canvasExports++;return exportCanvas.apply(this,args);};
 for(const constructor of [window.HTMLCanvasElement,window.OffscreenCanvas]){
  if(!constructor)continue;
  const original=constructor.prototype.getContext;
  constructor.prototype.getContext=function(type,...args){if(/^(?:webgl2?|experimental-webgl)$/.test(type))state.webglRequests++;return original.call(this,type,...args);};
 }
 addEventListener('scroll',()=>state.scrollEvents++,{passive:true});
 window.__painterV48={
  csp:[],
  snapshot(){return{...state,mediaNodes:document.querySelectorAll('.liquid-glass-media').length,opticalImages:document.querySelectorAll('.liquid-glass-optics img').length,definitions:document.querySelectorAll('.liquid-glass-definitions,filter[id^="painter-liquid-glass-"]').length,textured:document.querySelectorAll('[data-glass-texture-size]').length,modes:[...document.querySelectorAll('.liquid-glass-surface')].map(element=>({classes:element.className,mode:element.dataset.liquidGlassMode,filter:getComputedStyle(element).backdropFilter||getComputedStyle(element).getPropertyValue('-webkit-backdrop-filter')})),y:scrollY};},
  resetWrites(){state.styleWrites=state.treeWrites=state.sourceWrites=state.scrollEvents=0;}
 };
 document.addEventListener('securitypolicyviolation',event=>window.__painterV48.csp.push({directive:event.violatedDirective,blocked:event.blockedURI}));
}
async function snapshot(page){return page.evaluate(()=>window.__painterV48.snapshot());}
function nativeOnly(label,value){
 check(label+': no image mirror or decorative clone exists',value.mediaNodes===0&&value.opticalImages===0&&value.mirrorsEver===0,{mediaNodes:value.mediaNodes,opticalImages:value.opticalImages,mirrorsEver:value.mirrorsEver});
 check(label+': phone creates no SVG lens definition or normal-map export',value.definitions===0&&value.definitionsEver===0&&value.textured===0&&value.canvasExports===0,{definitions:value.definitions,definitionsEver:value.definitionsEver,textured:value.textured,canvasExports:value.canvasExports});
 check(label+': app does not request WebGL',value.webglRequests===0,value.webglRequests);
 check(label+': phone uses native blur or its solid preference',value.modes.length>0&&value.modes.every(item=>['blur','solid'].includes(item.mode))&&value.modes.every(item=>!item.filter?.includes('url(')),value.modes);
}
async function capture(page,name){const file=name+'.png';await page.screenshot({path:path.join(out,file),fullPage:false});report.screenshots.push(file);}
function intervals(values){
 const ordered=[...values].sort((a,b)=>a-b),quantile=q=>ordered[Math.min(ordered.length-1,Math.floor((ordered.length-1)*q))];
 return{samples:values.length,min_ms:ordered[0],median_ms:quantile(.5),p95_ms:quantile(.95),max_ms:ordered.at(-1),over_25ms:values.filter(value=>value>25).length,over_50ms:values.filter(value=>value>50).length};
}
async function createContext(browser,label){
 const context=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,deviceScaleFactor:1,colorScheme:'light'});
 await context.addInitScript(installProbe);
 await context.addInitScript(()=>{try{localStorage.setItem('painter-theme','light');localStorage.setItem('painter-solid','0');localStorage.setItem('painter-analytics','off');}catch{}});
 await context.route('**/*',route=>{
  const request=route.request(),url=new URL(request.url());
  if(url.origin===new URL(base).origin&&!['GET','HEAD'].includes(request.method())){
   if(url.pathname!=='/api/v1/analytics/events')report.blockedWrites.push({label,path:url.pathname,method:request.method()});
   return route.fulfill({status:204});
  }
  return route.continue();
 });
 context.on('page',page=>{
  page.on('pageerror',error=>report.errors.push({label,message:error.message}));
  page.on('response',response=>{const url=new URL(response.url());if(url.origin===new URL(base).origin&&response.status()>=500)report.httpErrors.push({label,path:url.pathname,status:response.status()});});
 });
 return context;
}
async function interactions(page,label){
 const originalURL=page.url();
 await page.locator('[data-mobile-more-open]').click();await page.locator('#mobile-more[open]').waitFor();
 check(label+': more is a native modal',await page.locator('#mobile-more').evaluate(element=>element.matches(':modal')));
 const themes=page.locator('#mobile-more .floating-theme');
 if(!await themes.evaluate(element=>element.open))await themes.locator('summary').click();
 await page.locator('#mobile-more [data-theme-option="dark"]').click();
 check(label+': theme remains touch-operable',await page.locator('html').getAttribute('data-theme')==='dark');
 if(!await themes.evaluate(element=>element.open))await themes.locator('summary').click();
 await page.locator('#mobile-more [data-solid-toggle]').click();
 await page.waitForFunction(()=>document.querySelector('[data-mobile-nav]')?.dataset.liquidGlassMode==='solid');
 check(label+': reduced transparency uses the solid fallback',await page.evaluate(()=>localStorage.getItem('painter-solid')==='1'&&document.documentElement.classList.contains('reduce-transparency')));
 await page.locator('#mobile-more [data-solid-toggle]').click();
 await page.waitForFunction(()=>document.querySelector('[data-mobile-nav]')?.dataset.liquidGlassMode==='blur');
 if(await themes.evaluate(element=>element.open))await themes.locator('summary').click();
 await page.goBack();await page.waitForFunction(()=>!document.querySelector('#mobile-more').open);
 check(label+': Back closes more and restores its opener',page.url()===originalURL&&await page.locator('[data-mobile-more-open]').evaluate(element=>element===document.activeElement));
 await page.locator('[data-search-open]').click();await page.locator('#site-search[open]').waitFor();
 await page.locator('#quick-search').fill('__v48_scroll_check__');await page.waitForTimeout(250);
 check(label+': search retains focused input and native modal ownership',await page.locator('#quick-search').evaluate(element=>element===document.activeElement)&&await page.locator('#site-search').evaluate(element=>element.matches(':modal')));
 await capture(page,label+'-search');
 await page.goBack();await page.waitForFunction(()=>!document.querySelector('#site-search').open);
 check(label+': Back dismisses search on the same page',page.url()===originalURL);
 nativeOnly(label+' after interactions',await snapshot(page));
}
async function run(browser,engine,cpuRate){
 const label=engine+(cpuRate>1?'-cpu'+cpuRate:'-normal'),context=await createContext(browser,label),page=await context.newPage();let session;
 try{
  await page.goto(base+'/');await page.locator('html.mobile-chrome-ready').waitFor();await page.evaluate(()=>document.fonts.ready);await page.waitForTimeout(900);
  check(label+': the real public homepage and phone navigation load',await page.locator('body').getAttribute('data-page-kind')==='home'&&await page.locator('[data-mobile-nav]').isVisible());
  nativeOnly(label+' before scroll',await snapshot(page));
  if(cpuRate>1){session=await context.newCDPSession(page);await session.send('Emulation.setCPUThrottlingRate',{rate:cpuRate});}
  await page.evaluate(()=>window.__painterV48.resetWrites());
  const workload=await page.evaluate(()=>new Promise(resolve=>{
   const frames=120,positions=[],times=[],requested=[],range=Math.max(0,document.documentElement.scrollHeight-innerHeight),step=Math.max(120,Math.min(300,range/8));
   let index=0,y=0,direction=1;
   function advance(timestamp){
    if(y+direction*step>range){direction=-1;y=range;}else if(y+direction*step<0){direction=1;y=0;}else y+=direction*step;
    scrollTo({top:y,behavior:'instant'});requested.push(y);positions.push(scrollY);times.push(timestamp);
    if(++index<frames)requestAnimationFrame(advance);else resolve({frames,range,step,requested,positions,timestamps_ms:times,intervals_ms:times.slice(1).map((time,index)=>time-times[index])});
   }
   requestAnimationFrame(advance);
  }));
  await page.waitForTimeout(0);const stopped=await snapshot(page);
  await page.waitForTimeout(800);const after800=await snapshot(page);
  await page.waitForTimeout(800);const after1600=await snapshot(page);
  const statistics=intervals(workload.intervals_ms);
  report.runs.push({label,engine,browser:browser.version(),cpu_rate:cpuRate,workload,frame_intervals:statistics,stopped,after_800ms:after800,after_1600ms:after1600});
  check(label+': 120 real rAF scroll frames traverse the document',workload.frames===120&&workload.intervals_ms.length===119&&new Set(workload.positions.map(Math.round)).size>=8&&Math.max(...workload.positions)-Math.min(...workload.positions)>=Math.min(600,workload.range*.8),{range:workload.range,unique_positions:new Set(workload.positions.map(Math.round)).size,...statistics});
  check(label+': scrolling performs no writes inside glass optics',stopped.styleWrites===0&&stopped.treeWrites===0&&stopped.sourceWrites===0,stopped);
  check(label+': no glass work chases the page after scrolling stops',[after800,after1600].every(value=>value.styleWrites===stopped.styleWrites&&value.treeWrites===stopped.treeWrites&&value.sourceWrites===stopped.sourceWrites),{stopped,after800,after1600});
  nativeOnly(label+' after fast scroll',after1600);
  if(session){await session.send('Emulation.setCPUThrottlingRate',{rate:1});await session.detach();session=undefined;}
  if(cpuRate===1){await capture(page,label+'-after-scroll');await interactions(page,label);}
  report.csp.push(...await page.evaluate(()=>window.__painterV48.csp));
 }finally{if(session){try{await session.send('Emulation.setCPUThrottlingRate',{rate:1});await session.detach();}catch{}}await context.close();}
}
(async()=>{
 fs.mkdirSync(out,{recursive:true});
 for(const engine of engines){
  if(!['chromium','webkit'].includes(engine))throw Error('Unsupported engine '+engine);
  const browser=await playwright[engine].launch(engine==='chromium'?{executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']}:{headless:true,executablePath:process.env.WEBKIT_EXECUTABLE||'/tmp/painter-v47-webkit-tools/webkit-local.sh'});
  try{await run(browser,engine,1);if(engine==='chromium')await run(browser,engine,8);}finally{await browser.close();}
 }
 check('no JavaScript or server errors',report.errors.length===0&&report.httpErrors.length===0,{js:report.errors,http:report.httpErrors});
 check('CSP remains intact',report.csp.length===0,report.csp);
 check('read-only checks never request a content write',report.blockedWrites.length===0,report.blockedWrites);
})().catch(error=>{report.fatal=String(error.stack||error);console.error(report.fatal);process.exitCode=1;}).finally(()=>{
 report.passed=!report.fatal&&report.checks.every(check=>check.passed);if(!report.passed)process.exitCode=1;
 fs.mkdirSync(out,{recursive:true});const file=path.join(out,'verification.json');fs.writeFileSync(file,JSON.stringify(report,null,2));
 console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,runs:report.runs.length,screenshots:report.screenshots.length,report:file}));
});
