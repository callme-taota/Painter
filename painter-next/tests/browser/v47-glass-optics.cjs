// Anonymous, read-only optical/runtime checks. This does not authenticate or write content.
// The WebKit build is a real Linux WebKit engine, not Safari or a physical iPhone.
const fs = require('node:fs');
const path = require('node:path');
const playwright = require(process.env.PLAYWRIGHT_MODULE || '/tmp/painter-v47-webkit-tools/node_modules/playwright');
const {PNG}=require(process.env.PAINTER_PNGJS_MODULE||'/tmp/painter-v47-webkit-tools/node_modules/pngjs');
const base = (process.env.PAINTER_BASE || 'http://127.0.0.1:18090').replace(/\/$/, '');
if (base !== 'http://127.0.0.1:18090') throw Error('This regression is restricted to the isolated :18090 candidate.');
const out = path.resolve(process.env.PAINTER_V47_ARTIFACTS || 'tests/artifacts/v47/optics');
const selectedEngines = (process.env.PAINTER_V47_ENGINES || 'chromium,webkit').split(',');
const baseline = process.argv.includes('--baseline');
const headerOnly=process.argv.includes('--header-only');
const remoteImage='https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/620/library_hero.jpg?__painter_v47_fixture=1';
const report = {base, baseline, checks: [], screenshots: [], metrics: [], errors: [], csp: [], httpErrors: [], blockedWrites: [], remoteRequests:[],engines: [], limitations: ['Mobile viewport and touch emulation; Linux WebKit is not physical iOS Safari. GPU timings and battery use require a physical phone.','The external-image regression uses a real cross-origin HTTPS image URL whose response is fulfilled with the repository CC0 JPEG; it does not depend on a Steam network response. Screenshot comparison happens in Node, never via a page canvas.']};
let currentPage;

function check(name, passed, detail) {
  report.checks.push({name, passed: !!passed, ...(detail === undefined ? {} : {detail})});
  console.log(`${passed ? 'PASS' : 'FAIL'} ${name}`);
}
function probe() {
  const contexts = [], known = new WeakMap();
  for (const constructor of [window.HTMLCanvasElement,window.OffscreenCanvas]) {
    if(!constructor)continue;
    const getContext = constructor.prototype.getContext;
    constructor.prototype.getContext = function(type, ...args) {
      const context = getContext.call(this, type, ...args);
      if (context && /^(webgl2?|experimental-webgl)$/.test(type) && !known.has(context)) {
        const record = {canvas:this,context,type,offscreen:constructor===window.OffscreenCanvas,draws:0,uploads:0,created:performance.now(),lastDraw:0};
        contexts.push(record); known.set(context, record);
      }
      return context;
    };
  }
  for (const constructor of [window.WebGLRenderingContext, window.WebGL2RenderingContext]) {
    if (!constructor) continue;
    for (const method of ['drawArrays', 'drawElements', 'drawArraysInstanced', 'drawElementsInstanced', 'texImage2D', 'texSubImage2D']) {
      const original = constructor.prototype[method];
      if (typeof original !== 'function' || original.__painterV47) continue;
      const wrapper = function(...args) {
        const value = original.apply(this, args), record = known.get(this);
        if (record) {
          if (method.startsWith('draw')) { record.draws++; record.lastDraw = performance.now(); }
          else record.uploads++;
        }
        return value;
      };
      Object.defineProperty(wrapper, '__painterV47', {value:true});
      constructor.prototype[method] = wrapper;
    }
  }
  window.__painterV47 = {
    csp: [],styleWrites:0,
    snapshot() {
      return {
        now: performance.now(),styleWrites:this.styleWrites,
        contexts: contexts.map(record => ({type:record.type,offscreen:record.offscreen,draws:record.draws,uploads:record.uploads,lastDraw:record.lastDraw,width:record.context.drawingBufferWidth,height:record.context.drawingBufferHeight,lost:record.context.isContextLost(),connected:record.canvas.isConnected===true,className:record.canvas.className||'OffscreenCanvas'})),
        canvases: [...document.querySelectorAll('canvas')].map(canvas => { const r=canvas.getBoundingClientRect();return {width:canvas.width,height:canvas.height,visible:r.width>0&&r.height>0&&getComputedStyle(canvas).visibility!=='hidden',className:canvas.className,parent:canvas.parentElement?.className}; }),
      };
    },
  };
  document.addEventListener('securitypolicyviolation', event => window.__painterV47.csp.push({directive:event.violatedDirective,blocked:event.blockedURI,source:event.sourceFile}));
  const watch=()=>{if(document.documentElement)new MutationObserver(records=>{for(const record of records)if(record.target instanceof Element&&record.target.closest('.liquid-glass-surface'))window.__painterV47.styleWrites++;}).observe(document.documentElement,{subtree:true,attributes:true,attributeFilter:['style']});};
  if(document.documentElement)watch();else document.addEventListener('DOMContentLoaded',watch,{once:true});
}
async function createContext(browser, engine, theme, options={}) {
  const context=await browser.newContext({viewport:options.desktop?{width:1440,height:1000}:{width:390,height:844},isMobile:!options.desktop,hasTouch:!options.desktop,colorScheme:theme,deviceScaleFactor:1,...(options.reducedMotion?{reducedMotion:'reduce'}:{})});
  await context.addInitScript(probe);
  await context.addInitScript(({theme,solid})=>{try{localStorage.setItem('painter-theme',theme);localStorage.setItem('painter-solid',solid?'1':'0');localStorage.setItem('painter-analytics','off');}catch{}},{theme,solid:!!options.solid});
  await context.route('**/*',route=>{
    const request=route.request(),url=new URL(request.url());
    if(request.url()===remoteImage){report.remoteRequests.push({engine,theme,type:request.resourceType(),method:request.method()});return route.fulfill({status:200,contentType:'image/jpeg',body:fs.readFileSync(path.join(__dirname,'fixtures/moments/lake-cc0.jpg'))});}
    if(url.origin===new URL(base).origin&&!['GET','HEAD'].includes(request.method())){
      if(url.pathname!=='/api/v1/analytics/events')report.blockedWrites.push({engine,theme,path:url.pathname,method:request.method()});
      return route.fulfill({status:204});
    }
    return route.continue();
  });
  context.on('page',page=>{
    page.on('pageerror',error=>report.errors.push({engine,theme,message:error.message}));
    page.on('response',response=>{const url=new URL(response.url());if(url.origin===new URL(base).origin&&response.status()>=500)report.httpErrors.push({engine,theme,path:url.pathname,status:response.status()});});
  });
  return context;
}
async function ready(page) {
  await page.locator('html.mobile-chrome-ready').waitFor();
  await page.evaluate(()=>document.fonts.ready);
  await page.waitForTimeout(1200);
}
async function capture(page,name) {
  const filename=name+'.png';await page.screenshot({path:path.join(out,filename),fullPage:false});report.screenshots.push(filename);
}
async function read(page,label) {
  const value=await page.evaluate(()=>({probe:window.__painterV47.snapshot(),surfaces:[...document.querySelectorAll('.liquid-glass-surface')].map(element=>{const rect=element.getBoundingClientRect(),holder=element.querySelector('.liquid-glass-media');return {tag:element.tagName,classes:element.className,mode:element.dataset.liquidGlassMode,renderer:element.dataset.glassRenderer,mirrors:holder?[...holder.querySelectorAll('img')].map(img=>({src:img.currentSrc||img.src,loaded:img.complete&&img.naturalWidth>0,width:img.width,height:img.height})):[],filters:holder?[holder,...holder.querySelectorAll('img')].map(node=>{const filter=getComputedStyle(node).filter,id=filter.match(/#([^"')]+)/)?.[1];return{filter,id,definition:id?document.getElementById(id)?.tagName:null};}):[],attributes:[...element.attributes].filter(a=>a.name.includes('glass')).map(a=>[a.name,a.value]),visible:rect.width>0&&rect.height>0,overlays:element.querySelectorAll(':scope > .liquid-glass-optics').length};}),overflow:document.documentElement.scrollWidth>innerWidth+1,csp:window.__painterV47.csp}));
  report.metrics.push({label,...value});
  report.csp.push(...value.csp.map(item=>({label,...item})));
  return value;
}
async function idle(page,label) {
  // Wait for source/image/layout invalidations before measuring steady state.
  await page.waitForTimeout(1500);const first=await read(page,label+'-idle-start');
  await page.waitForTimeout(1200);const second=await read(page,label+'-idle-end');
  const delta=second.probe.styleWrites-first.probe.styleWrites;
  check(label+': static viewport performs no optical style writes',delta===0,{styleWritesIn1200ms:delta});
  check(label+': media optics create no WebGL context',second.probe.contexts.length===0,second.probe.contexts);
  return second;
}
function comparePNGs(first,second){
  const a=PNG.sync.read(first),b=PNG.sync.read(second);
  if(a.width!==b.width||a.height!==b.height)return{sameSize:false,changed:0,total:0,maxDifference:0};
  let changed=0,maxDifference=0;
  for(let index=0;index<a.data.length;index+=4){const delta=Math.max(...[0,1,2].map(channel=>Math.abs(a.data[index+channel]-b.data[index+channel])));if(delta>8)changed++;maxDifference=Math.max(maxDifference,delta);}
  return{sameSize:true,changed,total:a.width*a.height,maxDifference};
}
async function mediaImage(page,label,engine,theme){
  const source=await page.evaluate(async url=>{
    const image=document.querySelector('[data-game-hero]');if(!image)return null;
    image.removeAttribute('srcset');image.src=url;image.loading='eager';
    try{await image.decode()}catch{}
    const rect=image.getBoundingClientRect(),nav=document.querySelector('[data-mobile-nav]').getBoundingClientRect();
    scrollTo({top:scrollY+rect.top+rect.height*.5-(nav.top+nav.height*.5),behavior:'instant'});
    return{src:image.currentSrc||image.src,loaded:image.complete&&image.naturalWidth>0};
  },remoteImage);
  check(label+': cross-origin fixture remains an ordinary loaded image',source?.loaded&&source.src===remoteImage,source);
  await page.waitForFunction(url=>{const nav=document.querySelector('[data-mobile-nav]');return nav?.dataset.liquidGlassMode==='media'&&nav.dataset.glassRenderer==='active'&&[...nav.querySelectorAll('.liquid-glass-media img')].some(img=>img.complete&&img.naturalWidth>0&&(img.currentSrc||img.src)===url);},remoteImage,{timeout:6000}).catch(()=>{});
  await page.waitForTimeout(300);const value=await read(page,label+'-external-media');
  const nav=value.surfaces.find(surface=>surface.classes.includes('mobile-primary-nav'));
  const active=nav?.mode==='media'&&nav.renderer==='active';
  check(label+': navigation mirrors the actual image beneath it',active&&nav.mirrors.some(image=>image.loaded&&image.src===remoteImage),nav);
  check(label+': each active media surface uses a loaded image and real SVG filter',value.surfaces.filter(surface=>surface.mode==='media'&&surface.renderer==='active').every(surface=>surface.mirrors.some(image=>image.loaded)&&surface.filters.some(filter=>filter.id&&filter.definition?.toLowerCase()==='filter')),value.surfaces);
  await capture(page,label+'-external-image');
  if(active){
    const target=page.locator('[data-mobile-nav]');
    const before=await target.screenshot({animations:'disabled'});
    await page.evaluate(()=>{
      const holder=document.querySelector('[data-mobile-nav] .liquid-glass-media');
      window.__painterV47FilterBackup=[holder,...holder.querySelectorAll('img')].filter(element=>getComputedStyle(element).filter.includes('url(')).map(element=>({element,style:element.getAttribute('style')}));
      for(const {element}of window.__painterV47FilterBackup)element.style.setProperty('filter','none','important');
    });
    await page.waitForTimeout(80);const after=await target.screenshot({animations:'disabled'});
    await page.evaluate(()=>{for(const {element,style}of window.__painterV47FilterBackup){if(style===null)element.removeAttribute('style');else element.setAttribute('style',style);}delete window.__painterV47FilterBackup;});
    const difference=comparePNGs(before,after);
    check(label+': screenshot A/B proves the SVG filter changes rendered pixels',difference.sameSize&&difference.changed>=Math.max(20,difference.total*.001),difference);
    fs.writeFileSync(path.join(out,label+'-filter-on.png'),before);fs.writeFileSync(path.join(out,label+'-filter-off.png'),after);
    report.screenshots.push(label+'-filter-on.png',label+'-filter-off.png');
  }
  await idle(page,label+'-external-media');
  await page.setViewportSize({width:430,height:844});await page.waitForTimeout(350);
  const resized=await read(page,label+'-resize');
  check(label+': resizing preserves valid media or an empty native fallback',!resized.overflow&&resized.surfaces.every(surface=>surface.mode==='media'?surface.mirrors.some(image=>image.loaded):surface.mirrors.length===0),resized.surfaces);
  await page.setViewportSize({width:390,height:844});await page.waitForTimeout(300);
  const requests=report.remoteRequests.filter(request=>request.engine===engine&&request.theme===theme);
  check(label+': external media uses image requests without fetch or XHR',requests.some(request=>request.type==='image')&&requests.every(request=>request.type==='image'),requests);
}
async function matrix(browser,engine,theme) {
  const label=engine+'-'+theme,context=await createContext(browser,engine,theme),page=await context.newPage();currentPage=page;
  try {
    await page.goto(base+'/');await ready(page);
    check(label+': public bottom navigation stays visible and singular',await page.locator('[data-mobile-nav]').count()===1&&await page.locator('[data-mobile-nav]').isVisible());
    check(label+': original four navigation destinations remain',await page.locator('[data-mobile-nav] > a').count()===4);
    const initial=await read(page,label+'-home');
    const navMaterial=await page.locator('[data-mobile-nav]').evaluate(element=>{const style=getComputedStyle(element);return{tint:style.getPropertyValue('--liquid-glass-tint').trim(),blur:style.getPropertyValue('--liquid-glass-blur').trim()};});
    const alpha=Number(navMaterial.tint.match(/\/\s*(\d*\.?\d+)\s*\)/)?.[1]);
    check(label+': navigation keeps its thin mobile material tokens',Math.abs(alpha-(theme==='light'?.54:.62))<.0001&&parseFloat(navMaterial.blur)===10,navMaterial);
    check(label+': viewport has no horizontal overflow',!initial.overflow);
    check(label+': enhanced surfaces do not duplicate optics',initial.surfaces.every(surface=>surface.overlays===1));
    check(label+': blank backdrop does not create an invented media image',initial.surfaces.filter(surface=>surface.visible&&surface.mode!=='media').every(surface=>surface.mirrors.length===0),initial.surfaces);
    check(label+': enhancement creates no WebGL context',initial.probe.contexts.length===0,initial.probe);
    await capture(page,label+'-home');await idle(page,label+'-home');
    await page.evaluate(()=>scrollTo({top:Math.min(document.documentElement.scrollHeight-innerHeight,innerHeight*.8),behavior:'instant'}));await page.waitForTimeout(700);
    const scrolled=await read(page,label+'-scroll');
    await capture(page,label+'-scroll');await idle(page,label+'-scroll');
    await mediaImage(page,label,engine,theme);
    await page.locator('[data-mobile-more-open]').click();await page.locator('#mobile-more[open]').waitFor();
    check(label+': more retains native modal and scroll lock',await page.locator('#mobile-more').evaluate(element=>element.matches(':modal'))&&await page.locator('html').evaluate(element=>element.classList.contains('mobile-overlay-open')));
    await capture(page,label+'-more');
    const opposite=theme==='light'?'dark':'light';
    const themeDetails=page.locator('#mobile-more .floating-theme');
    if(!await themeDetails.evaluate(element=>element.open))await themeDetails.locator('summary').click();
    await page.locator('#mobile-more [data-theme-option="'+opposite+'"]').click();
    check(label+': theme changes remain touch-operable with media optics',await page.locator('html').getAttribute('data-theme')===opposite);
    if(!await themeDetails.evaluate(element=>element.open))await themeDetails.locator('summary').click();
    await page.locator('#mobile-more [data-theme-option="'+theme+'"]').click();
    if(await themeDetails.evaluate(element=>element.open))await themeDetails.locator('summary').click();
    await page.goBack();await page.waitForFunction(()=>!document.querySelector('#mobile-more').open);
    check(label+': Back dismisses more without navigating',new URL(page.url()).pathname==='/');
    check(label+': more restores its opener focus',await page.locator('[data-mobile-more-open]').evaluate(element=>element===document.activeElement));
    await page.locator('[data-search-open]').click();await page.locator('#site-search[open]').waitFor();
    await page.waitForFunction(()=>document.activeElement?.id==='quick-search');
    check(label+': search remains focused and a native modal',await page.locator('#site-search').evaluate(element=>element.matches(':modal')));
    const backdrop=await page.locator('#site-search').evaluate(element=>{const style=getComputedStyle(element,'::backdrop');return{filter:style.backdropFilter||style.getPropertyValue('-webkit-backdrop-filter'),background:style.backgroundColor};});
    check(label+': search backdrop requests native blur for underlying text',Number(backdrop.filter.match(/blur\(([\d.]+)px\)/)?.[1])>=12,backdrop);
    const mainFilter=await page.locator('main').evaluate(element=>getComputedStyle(element).filter);
    check(label+': mobile search also blurs the original background content',Number(mainFilter.match(/blur\(([\d.]+)px\)/)?.[1])>=12,mainFilter);
    const searchBox=await page.locator('#site-search').boundingBox();
    check(label+': mobile search remains a full-screen task',searchBox.width>=389&&searchBox.height>=840,searchBox);
    await page.locator('#quick-search').fill('__v47_no_match__');await page.waitForTimeout(650);
    check(label+': search text survives optical updates',await page.locator('#quick-search').inputValue()==='__v47_no_match__');
    await capture(page,label+'-search');await read(page,label+'-search');
    await page.locator('[data-search-close]').click();await page.waitForFunction(()=>!document.querySelector('#site-search').open);
    check(label+': cancel returns to the same content',new URL(page.url()).pathname==='/');
    check(label+': closing search restores a sharp reading surface',await page.locator('main').evaluate(element=>getComputedStyle(element).filter==='none'));
    await page.locator('[data-mobile-nav] > a[href="/posts"]').click();await page.waitForURL(base+'/posts');await ready(page);
    check(label+': optical layer does not intercept real navigation',await page.locator('[data-mobile-nav] > a[href="/posts"]').getAttribute('aria-current')==='page');
    await capture(page,label+'-posts');await read(page,label+'-posts');
  } finally {await context.close();}
}
async function privacyAndPreferences(browser,engine) {
  for(const [route,options,label]of [['/login',{},'private-login'],['/',{solid:true},'solid-preference']]){
    const context=await createContext(browser,engine,'light',options),page=await context.newPage();currentPage=page;
    try {
      await page.goto(base+route);await ready(page);const value=await read(page,engine+'-'+label);
      check(engine+': '+label+' does not create a live rasterisation context',value.probe.contexts.length===0,value.probe);
      check(engine+': '+label+' does not duplicate media',value.surfaces.every(surface=>surface.mirrors.length===0),value.surfaces);
      if(options.solid)check(engine+': reduced transparency keeps navigation usable',await page.locator('[data-mobile-nav]').isVisible()&&value.surfaces.filter(surface=>surface.visible).every(surface=>surface.mode==='solid'),value.surfaces);
    } finally {await context.close();}
  }
  const context=await createContext(browser,engine,'light'),page=await context.newPage();currentPage=page;
  try {
    await page.goto(base+'/posts');await ready(page);
    // A browser-only marker verifies privacy invalidation; this never enters an API/database.
    await page.evaluate(()=>{const input=document.createElement('input');input.type='password';input.setAttribute('data-private','');input.setAttribute('aria-label','V47 browser-only privacy probe');document.querySelector('main').append(input);});
    await page.waitForTimeout(500);const before=await read(page,engine+'-private-node-added');
    await page.waitForTimeout(1000);const after=await read(page,engine+'-private-node-settled');
    check(engine+': adding a private field never creates DOM rasterisation or WebGL',after.probe.contexts.length===0&&!after.probe.canvases.some(canvas=>canvas.visible)&&after.surfaces.every(surface=>surface.mirrors.length===0),{before:before.probe,after:after.probe,surfaces:after.surfaces});
  } finally {await context.close();}
}
async function desktop(browser) {
  const context=await createContext(browser,'chromium','light',{desktop:true}),page=await context.newPage();currentPage=page;
  try {
    await page.goto(base+'/search');await page.evaluate(()=>document.fonts.ready);await page.waitForTimeout(400);
    check('desktop: phone navigation remains hidden',!await page.locator('[data-mobile-nav]').isVisible());
    await page.locator('[data-search-open]').click();await page.locator('#site-search[open]').waitFor();
    const box=await page.locator('#site-search').boundingBox();check('desktop: floating search remains bounded',box.width<1000&&box.y>0,box);
    const value=await read(page,'desktop-search');check('desktop: DOM rasterisation and WebGL stay off',value.probe.contexts.length===0,value.probe);
    await capture(page,'desktop-search');
  } finally {await context.close();}
}
async function headerImage(browser,engine){
  const label=engine+'-light-header',context=await createContext(browser,engine,'light'),page=await context.newPage();currentPage=page;
  try{
    await page.goto(base+'/games');await ready(page);
    const source=await page.evaluate(async url=>{
      const image=document.querySelector('[data-library-cover]');if(!image)return null;
      image.src=url;image.loading='eager';try{await image.decode()}catch{}
      const imageRect=image.getBoundingClientRect(),header=document.querySelector('.site-header').getBoundingClientRect();
      scrollTo({top:scrollY+imageRect.top+imageRect.height*.5-(header.top+header.height*.5)+32,behavior:'instant'});
      return{loaded:image.complete&&image.naturalWidth>0,src:image.currentSrc||image.src};
    },remoteImage);
    await page.waitForTimeout(350);await page.evaluate(()=>scrollBy({top:-32,behavior:'instant'}));
    await page.waitForTimeout(900);const value=await read(page,label+'-visible');
    const header=value.surfaces.find(surface=>surface.classes.includes('site-header'));
    check(label+': a small upward scroll reveals the sticky header',await page.locator('.site-header').evaluate(element=>{const rect=element.getBoundingClientRect();return getComputedStyle(element).visibility==='visible'&&element.dataset.scrollHidden==='false'&&rect.top>=0&&rect.bottom<innerHeight;}));
    check(label+': the revealed header refracts a loaded ordinary image',source?.loaded&&header?.mode==='media'&&header.renderer==='active'&&header.mirrors.some(image=>image.loaded&&image.src===remoteImage),header);
    check(label+': revealed header points to an existing native SVG filter',header?.filters.some(filter=>filter.id&&filter.definition?.toLowerCase()==='filter'),header?.filters);
    await capture(page,label+'-image');await idle(page,label);
  }finally{await context.close();}
}
(async()=>{
  fs.mkdirSync(out,{recursive:true});
  for(const engine of selectedEngines){
    if(!['chromium','webkit'].includes(engine))throw Error('Unsupported test engine: '+engine);
    const browser=await playwright[engine].launch(engine==='chromium'?{executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox','--enable-unsafe-swiftshader']}:{headless:true,...(process.env.WEBKIT_EXECUTABLE?{executablePath:process.env.WEBKIT_EXECUTABLE}:{})});
    report.engines.push({engine,version:browser.version(),platform:process.platform});
    try {if(headerOnly)await headerImage(browser,engine);else{for(const theme of ['light','dark'])await matrix(browser,engine,theme);await privacyAndPreferences(browser,engine);if(engine==='chromium')await desktop(browser);}}finally{await browser.close();}
  }
  check('no browser runtime errors',!report.errors.length,report.errors);
  check('no Content Security Policy violations',!report.csp.length,report.csp);
  check('no same-origin server errors',!report.httpErrors.length,report.httpErrors);
  check('anonymous optical checks never request a content write',!report.blockedWrites.length,report.blockedWrites);
})().catch(async error=>{report.fatal=String(error.stack||error);console.error(report.fatal);try{if(currentPage&&!currentPage.isClosed())await capture(currentPage,'failure');}catch{};process.exitCode=1;}).finally(()=>{
  report.passed=!report.fatal&&report.checks.every(check=>check.passed);if(!report.passed)process.exitCode=1;
  const filename=headerOnly?'header-verification.json':baseline?'baseline.json':'verification.json';
  fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,filename),JSON.stringify(report,null,2));
  console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,screenshots:report.screenshots.length,report:path.join(out,filename)}));
});
