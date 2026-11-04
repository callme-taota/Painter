// Isolated browser verification of the shared image viewer. No service, build or database writes.
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const {spawnSync} = require('child_process');
const ts = require('../../web/frontend/node_modules/typescript');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || '/tmp/painter-browser/node_modules/playwright');
const origin = 'http://v27-viewer.test';
const out = path.resolve('tests/artifacts/v27/viewer');
const photo = fs.readFileSync('tests/browser/fixtures/moments/lake-cc0.jpg');
const avatar = fs.readFileSync('tests/browser/fixtures/avatar.png');
const photographPaths = new Set(['lake-1.jpg','lake-2.jpg','lake-3.jpg','article-1.jpg','article-2.jpg','lake-good.jpg'].map(name => '/photos/' + name));
const report = {
  origin, mode: 'isolated intercepted DOM; actual source transpiled in memory; no service/build/database',
  photo: {file: 'tests/browser/fixtures/moments/lake-cc0.jpg', provenance: 'Pixnio CC0/public-domain photograph; see fixtures/moments/README.md'},
  checks: [], errors: [], screenshots: [], unexpectedRequests: [], businessWrites: 0
};
function check(name, passed, detail) {
  report.checks.push({name, passed: !!passed, ...(detail === undefined ? {} : {detail})});
  console.log(`${passed ? 'PASS' : 'FAIL'} ${name}`);
  if (!passed) throw Error(name);
}
const source = name => fs.readFileSync(path.resolve('web/frontend/src', name), 'utf8');
report.sourceSha256 = Object.fromEntries(['momentGallery.ts','momentGallery.css'].map(name => [name, crypto.createHash('sha256').update(source(name)).digest('hex')]));
const modules = Object.fromEntries(['contentCards', 'momentGallery'].map(name => ['./' + name, ts.transpileModule(source(name + '.ts'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText]));
const script = `(() => {const sources=${JSON.stringify(modules)},cache={};function require(name){if(name.endsWith('.css'))return {};if(cache[name])return cache[name].exports;const m={exports:{}};cache[name]=m;new Function('require','module','exports',sources[name])(require,m,m.exports);return m.exports}window.cardRuntime=require('./contentCards');window.galleryRuntime=require('./momentGallery');for(let i=0;i<5;i++){cardRuntime.initArticleCards();galleryRuntime.initMomentGalleries()}window.runtimeReady=true})()`;
const styles = ['style.css', 'shadcn-theme.css', 'articleCards.css', 'momentGallery.css'].map(source).join('\n');
function imageLink(id, filename, alt, extra = '') {
  return `<a id="${id}" href="/photos/${filename}" target="_blank" rel="noopener" data-moment-image data-image-alt="${alt}" ${extra}><img src="/thumbnail.jpg" alt="${alt}"></a>`;
}
function fixture(theme) {
  return `<!doctype html><html lang="zh-CN" data-theme="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>${styles}
main{width:min(760px,calc(100% - 32px));margin:32px auto;font-family:var(--font-ui)}h1{font-size:24px;margin-bottom:16px}.runtime-grid{display:flex;gap:12px;flex-wrap:wrap;margin:20px 0}.runtime-grid>a,.runtime-grid img,.prose>a,.prose>img{display:block;width:112px;height:84px;object-fit:cover}.prose{display:flex;flex-wrap:wrap;gap:12px}.runtime-control{margin:20px 0}.runtime-note{font-size:14px;color:var(--muted)}.runtime-scroller{height:110vh}</style></head><body data-page-kind="article"><main><h1>Painter 图片预览</h1><p class="runtime-note">共享预览隔离验证 · 实际摄影素材</p><button id="outside" type="button" class="runtime-control">页面按钮</button><div id="moment-gallery" class="runtime-grid" data-moment-gallery>${imageLink('moment-one','lake-1.jpg','山湖相映 · 第一张')}${imageLink('moment-two','lake-2.jpg','山湖相映 · 第二张')}${imageLink('moment-three','lake-3.jpg','山湖相映 · 第三张')}</div><div id="home-gallery" class="home-photo-grid runtime-grid" data-moment-gallery data-gallery-title="图片墙">${imageLink('home-one','avatar.png','')}</div><article class="article-reading"><div id="article-gallery" class="prose"><a id="article-one-link" href="/photos/article-1.jpg" target="_blank" rel="noopener"><img id="article-one" src="/photos/article-1.jpg" alt="文章摄影 · 第一张"></a><img id="article-two" src="/photos/article-2.jpg" alt="文章摄影 · 第二张"><img id="article-failure" src="/photos/broken.jpg" alt="文章摄影 · 无法加载"><a id="article-navigation" href="/destination" target="_blank"><img src="/thumbnail.jpg" alt="独立导航插图"></a></div></article><div id="delayed-gallery" class="runtime-grid" data-moment-gallery data-gallery-title="异步图片">${imageLink('slow-one','slow-1.jpg','延迟第一张')}${imageLink('slow-two','slow-2.jpg','延迟第二张')}${imageLink('slow-good','lake-good.jpg','恢复后的图片')}</div><div class="runtime-scroller"></div></main><script>${script}</script></body></html>`;
}
async function newPage(browser, {theme = 'light', width = 1440, height = 900, noJS = false, noDialog = false} = {}) {
  const context = await browser.newContext({viewport: {width, height}, deviceScaleFactor: 1, isMobile: width < 500, hasTouch: width < 500, locale: 'zh-CN', colorScheme: theme, javaScriptEnabled: !noJS});
  const held = new Map();
  const settledRequests = new Map();
  context.on('requestfinished', request => settledRequests.set(new URL(request.url()).pathname, 'finished'));
  context.on('requestfailed', request => settledRequests.set(new URL(request.url()).pathname, 'failed'));
  if (noDialog) await context.addInitScript(() => { HTMLDialogElement.prototype.showModal = undefined; });
  await context.route('**/*', async route => {
    const request = route.request(), url = new URL(request.url());
    if (request.method() !== 'GET' || url.origin !== origin) {
      report.unexpectedRequests.push({method: request.method(), url: request.url()});
      if (request.method() !== 'GET') report.businessWrites++;
      return route.abort();
    }
    if (url.pathname === '/assets/geist-latin.woff2') return route.fulfill({contentType: 'font/woff2', body: fs.readFileSync('web/frontend/src/assets/geist-latin.woff2')});
    if (url.pathname === '/photos/broken.jpg') return route.fulfill({status: 404, body: 'Expected missing photograph'});
    if (url.pathname === '/photos/slow-1.jpg' || url.pathname === '/photos/slow-2.jpg') {
      held.set(url.pathname, route);
      return;
    }
    if (url.pathname === '/photos/avatar.png') return route.fulfill({contentType: 'image/png', body: avatar});
    if (photographPaths.has(url.pathname) || url.pathname === '/thumbnail.jpg') return route.fulfill({contentType: 'image/jpeg', body: photo});
    if (url.pathname === '/destination') return route.fulfill({contentType: 'text/html', body: '<!doctype html><title>Native destination</title>'});
    if (url.pathname !== '/' && url.pathname !== '/favicon.ico') report.unexpectedRequests.push({method: request.method(), url: request.url()});
    return route.fulfill({contentType: 'text/html', body: fixture(theme)});
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  page.on('pageerror', error => report.errors.push(error.message));
  await page.goto(origin + '/');
  if (!noJS) await page.waitForFunction(() => window.runtimeReady);
  await page.evaluate(() => document.fonts.ready);
  return {context, page, held, settledRequests};
}
const viewer = page => page.locator('.moment-lightbox');
const closeButton = page => page.getByRole('button', {name: '关闭图片预览', exact: true});
const previousButton = page => page.getByRole('button', {name: '上一张图片', exact: true});
const nextButton = page => page.getByRole('button', {name: '下一张图片', exact: true});
async function loaded(page) {
  await page.waitForFunction(() => { const d = document.querySelector('.moment-lightbox'); const i = d?.querySelector('.moment-lightbox-stage img'); return d?.open && i && !i.hidden && i.naturalWidth > 0; });
  await page.locator('.moment-lightbox-stage img').evaluate(async image => { await image.decode(); await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))); });
}
async function paintedPhoto(page) {
  const samples = await page.locator('.moment-lightbox-stage img').evaluate(image => {
    const r=image.getBoundingClientRect(),canvas=document.createElement('canvas');canvas.width=Math.round(r.width);canvas.height=Math.round(r.height);const c=canvas.getContext('2d');c.drawImage(image,0,0,canvas.width,canvas.height);
    return [.25,.5,.75].flatMap(y=>[.25,.5,.75].map(x=>({x:Math.floor(r.left+r.width*x),y:Math.floor(r.top+r.height*y),expected:[...c.getImageData(Math.floor(canvas.width*x),Math.floor(canvas.height*y),1,1).data].slice(0,3)})));
  });
  const png=await page.screenshot({fullPage:false,animations:'disabled'});
  const result=spawnSync('python3',['-c','import sys,json,io; from PIL import Image; im=Image.open(io.BytesIO(sys.stdin.buffer.read())).convert("RGB"); print(json.dumps([list(im.getpixel((p["x"],p["y"]))) for p in json.loads(sys.argv[1])]))',JSON.stringify(samples)],{input:png,maxBuffer:1024*1024});
  if(result.status!==0)throw Error('Screenshot pixel inspection failed: '+result.stderr.toString());
  const actual=JSON.parse(result.stdout.toString());
  return samples.map((sample,index)=>({...sample,actual:actual[index],difference:Math.max(...sample.expected.map((value,channel)=>Math.abs(value-actual[index][channel])))}));
}
async function position(page, value) { await page.waitForFunction(value => document.querySelector('.moment-lightbox-position')?.textContent === value, value); }
async function until(predicate) { const deadline=Date.now()+3000;while(!predicate()&&Date.now()<deadline)await new Promise(resolve=>setTimeout(resolve,10)); }
async function twoFrames(page) { await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))); }
async function closeWithEscape(page) { await page.keyboard.press('Escape'); await viewer(page).waitFor({state: 'hidden'}); await twoFrames(page); }
async function nativePopup(context, action) {
  const pending = context.waitForEvent('page'); await action(); const popup = await pending; await popup.waitForLoadState(); const url = popup.url(); await popup.close(); return url;
}
async function functional(browser) {
  const {context, page, held, settledRequests} = await newPage(browser);
  check('three consumers and repeated initialization produce one shared native dialog', await page.locator('.moment-lightbox').count() === 1 && await viewer(page).evaluate(el => el instanceof HTMLDialogElement));
  check('ordinary article images enhanced; independent navigation illustration stays native', await page.locator('a[data-article-image]').count() === 3 && await page.locator('#article-navigation').getAttribute('href') === '/destination' && await page.locator('#article-navigation[data-article-image]').count() === 0);
  await page.locator('#moment-two').click(); await loaded(page); await position(page, '2 / 3');
  check('requested moment opens in document order with exact source, full alt and accessible title/count', await page.getByRole('dialog',{name:'动态图片',exact:true}).count()===1 && await viewer(page).getAttribute('aria-labelledby') === 'moment-lightbox-title' && await viewer(page).getAttribute('aria-describedby') === 'moment-lightbox-position' && await page.locator('#moment-lightbox-title').textContent() === '动态图片' && await viewer(page).locator('img').getAttribute('src') === origin + '/photos/lake-2.jpg' && await viewer(page).locator('img').getAttribute('alt') === '山湖相映 · 第二张');
  check('native modal enters top layer and initially focuses close', await viewer(page).evaluate(el => el.matches(':modal')) && await closeButton(page).evaluate(el => el === document.activeElement));
  check('normal loaded multi-image view exposes exactly three icon buttons and no original link', await viewer(page).locator('button:visible').count() === 3 && await viewer(page).locator('a:visible').count() === 0 && await viewer(page).locator('button').evaluateAll(es => es.every(e => e.querySelector('svg') && !e.textContent.trim())));
  check('screen-reader title/count preserved without visible heading or caption', await page.locator('#moment-lightbox-title').evaluate(el => {const s=getComputedStyle(el);const r=el.getBoundingClientRect();return s.position==='absolute'&&s.display!=='none'&&s.visibility==='visible'&&s.overflow==='hidden'&&(s.clip!=='auto'||s.clipPath!=='none')&&r.width>0&&r.width<=1&&r.height>0&&r.height<=1}) && await page.locator('.moment-lightbox-position').evaluate(el => {const s=getComputedStyle(el);const r=el.getBoundingClientRect();return el.getAttribute('aria-live')==='polite'&&s.position==='absolute'&&s.display!=='none'&&s.visibility==='visible'&&s.overflow==='hidden'&&(s.clip!=='auto'||s.clipPath!=='none')&&r.width>0&&r.width<=1&&r.height>0&&r.height<=1}) && await viewer(page).locator('.moment-lightbox-header,.moment-lightbox-footer,.moment-lightbox-caption,.moment-lightbox-shell').count() === 0);
  await viewer(page).locator('img').click(); check('clicking the picture keeps modal open', await viewer(page).evaluate(el => el.open));
  const trapped = [];
  for (let i = 0; i < 5; i++) { await page.keyboard.press('Tab'); trapped.push(await page.evaluate(() => document.activeElement?.closest('dialog')?.classList.contains('moment-lightbox') || document.activeElement === document.body)); }
  check('native Tab cycle never focuses an inert page control', trapped.every(Boolean), trapped);
  await page.keyboard.press('Home'); await position(page, '1 / 3');
  check('Home moves first and disables previous while retaining next', await previousButton(page).isDisabled() && await nextButton(page).isEnabled());
  await page.keyboard.press('ArrowRight'); await position(page, '2 / 3');
  await page.keyboard.press('End'); await position(page, '3 / 3');
  check('ArrowRight and End follow order and disable next at final image', await nextButton(page).isDisabled() && await previousButton(page).isEnabled());
  await page.keyboard.press('ArrowLeft'); await position(page, '2 / 3');
  const modifiedKey = await page.evaluate(() => {const d=document.querySelector('.moment-lightbox');for(const key of ['altKey','ctrlKey','metaKey'])d.dispatchEvent(new KeyboardEvent('keydown',{key:'ArrowRight',[key]:true,bubbles:true,cancelable:true}));return document.querySelector('.moment-lightbox-position').textContent});
  check('modified arrow shortcuts retain native behavior', modifiedKey === '2 / 3');
  await closeWithEscape(page);
  check('Escape closes modal and returns focus to exact opener', await page.locator('#moment-two').evaluate(el => el === document.activeElement));
  await page.locator('#moment-one').click(); await loaded(page); await page.mouse.click(8, 8); await viewer(page).waitFor({state: 'hidden'});
  check('dark background click closes and returns focus', await page.locator('#moment-one').evaluate(el => el === document.activeElement));
  await page.locator('#home-one').click(); await loaded(page);
  check('home photo wall shares same dialog and single image hides both arrows', await page.locator('.moment-lightbox').count() === 1 && await page.locator('#moment-lightbox-title').textContent() === '图片墙' && await page.locator('.moment-lightbox-position').textContent() === '1 / 1' && await viewer(page).locator('button:visible').count() === 1 && await viewer(page).locator('img').getAttribute('src') === origin + '/photos/avatar.png' && await viewer(page).locator('img').getAttribute('alt') === '图片墙 1');
  await closeButton(page).click(); await viewer(page).waitFor({state:'hidden'});
  check('close icon restores photo-wall opener focus', await page.locator('#home-one').evaluate(el => el === document.activeElement));
  await page.locator('#article-one').click(); await loaded(page);
  check('article images share same viewer with exact source, article title and alt', await page.locator('.moment-lightbox').count() === 1 && await page.locator('#moment-lightbox-title').textContent() === '文章图片' && await page.locator('.moment-lightbox-position').textContent() === '1 / 3' && await viewer(page).locator('img').getAttribute('src') === origin + '/photos/article-1.jpg' && await viewer(page).locator('img').getAttribute('alt') === '文章摄影 · 第一张');
  await nextButton(page).click(); await position(page, '2 / 3'); await loaded(page);
  check('article next button follows enhanced-image document order', await viewer(page).locator('img').getAttribute('src') === origin + '/photos/article-2.jpg');
  await page.keyboard.press('End'); await page.getByText('图片暂时无法加载。', {exact: true}).waitFor();
  const recovery = viewer(page).getByRole('link', {name: '打开原图', exact: true});
  check('failed image hides picture and provides its exact native original-link recovery', await viewer(page).locator('img').isHidden() && await page.locator('.moment-lightbox-status').getAttribute('role')==='status' && await recovery.isVisible() && await recovery.getAttribute('href') === origin + '/photos/broken.jpg' && await recovery.getAttribute('target') === '_blank');
  const failedOriginal = await nativePopup(context, () => recovery.click());
  check('failure original link opens native destination', failedOriginal === origin + '/photos/broken.jpg');
  await page.keyboard.press('Home'); await loaded(page);
  check('switching from failed to loaded clears error and hides original recovery', await recovery.isHidden() && await page.locator('.moment-lightbox-status').isHidden());
  await closeWithEscape(page);
  check('article Escape restores generated/existing image anchor', await page.locator('#article-one-link').evaluate(el => el === document.activeElement));
  await page.locator('#slow-one').click(); await page.waitForFunction(() => document.querySelector('.moment-lightbox-status')?.textContent === '正在加载图片…');
  await until(() => held.has('/photos/slow-1.jpg'));
  check('delayed first image request is held for loading and stale-response verification', held.has('/photos/slow-1.jpg'));
  check('loading has live status and hidden picture without original link', await page.locator('.moment-lightbox-status').getAttribute('role')==='status' && await viewer(page).locator('img').isHidden() && await viewer(page).locator('a:visible').count() === 0);
  await page.keyboard.press('ArrowRight'); await position(page, '2 / 3');
  await page.waitForFunction(() => document.querySelector('.moment-lightbox-stage img')?.src.endsWith('/slow-2.jpg'));
  await until(() => held.has('/photos/slow-2.jpg'));
  check('delayed second image request is held while navigation remains responsive', held.has('/photos/slow-2.jpg'));
  // Finish the former request after switching; it must not replace the current image/status.
  const firstHeld = held.get('/photos/slow-1.jpg'); if (firstHeld) await firstHeld.fulfill({contentType: 'image/jpeg', body: photo}).catch(() => {});
  await until(() => settledRequests.has('/photos/slow-1.jpg')); await twoFrames(page);
  check('former delayed image request settles or cancels after switch', settledRequests.has('/photos/slow-1.jpg'));
  check('late previous image response cannot clear current loading state', await viewer(page).locator('img').getAttribute('src') === origin + '/photos/slow-2.jpg' && await viewer(page).locator('img').isHidden() && await page.locator('.moment-lightbox-status').isVisible());
  await page.keyboard.press('End'); await loaded(page);
  const secondHeld = held.get('/photos/slow-2.jpg'); if (secondHeld) await secondHeld.fulfill({status: 404, body: 'Late expected failure'}).catch(() => {});
  await until(() => settledRequests.has('/photos/slow-2.jpg')); await twoFrames(page);
  check('former delayed error request settles or cancels after successful switch', settledRequests.has('/photos/slow-2.jpg'));
  check('late previous error cannot replace successful current image with error recovery', await viewer(page).locator('img').getAttribute('src') === origin + '/photos/lake-good.jpg' && await viewer(page).locator('img').isVisible() && await page.locator('.moment-lightbox-status').isHidden() && await viewer(page).locator('a:visible').count() === 0);
  await closeWithEscape(page);
  const modifiers = await page.evaluate(() => {
    const a=document.querySelector('#moment-one'), results=[];
    for(const key of ['ctrlKey','metaKey','shiftKey','altKey','middle']) {
      const e=new MouseEvent('click',{bubbles:true,cancelable:true,button:key==='middle'?1:0,[key]:true});
      a.addEventListener('click',event=>{results.push({key,intercepted:event.defaultPrevented});event.preventDefault()},{once:true}); a.dispatchEvent(e);
    }
    const prior=new MouseEvent('click',{bubbles:true,cancelable:true,button:0});prior.preventDefault();a.dispatchEvent(prior);
    return {results,open:document.querySelector('.moment-lightbox').open};
  });
  check('modified/middle/prior-cancelled link activations never open viewer or intercept native navigation', modifiers.results.every(r => !r.intercepted) && !modifiers.open, modifiers);
  await page.locator('#moment-one').focus(); await page.keyboard.press('Enter'); await loaded(page);
  check('native keyboard Enter on photo link opens viewer', await viewer(page).evaluate(el => el.open));
  await closeWithEscape(page);
  await context.close();
}
async function geometry(browser) {
  for (const [width, height, theme] of [[1440,900,'light'],[1440,900,'dark'],[390,844,'light'],[390,844,'dark'],[320,568,'light'],[844,390,'dark']]) {
    const {context,page} = await newPage(browser,{width,height,theme});
    await page.locator('#moment-two').click(); await loaded(page);
    const metrics = await viewer(page).evaluate(d => {
      const image=d.querySelector('img'),stage=d.querySelector('.moment-lightbox-stage'),r=d.getBoundingClientRect(),i=image.getBoundingClientRect(),s=getComputedStyle(d),is=getComputedStyle(image),ss=getComputedStyle(stage),b=getComputedStyle(d,'::backdrop');
      return {viewport:{width:innerWidth,height:innerHeight},dialog:{x:r.x,y:r.y,width:r.width,height:r.height,border:[s.borderTopWidth,s.borderRightWidth,s.borderBottomWidth,s.borderLeftWidth],radius:s.borderRadius,shadow:s.boxShadow,background:s.backgroundColor},stage:{background:ss.backgroundColor,border:ss.borderTopWidth,shadow:ss.boxShadow},image:{x:i.x,y:i.y,right:i.right,bottom:i.bottom,width:i.width,height:i.height,naturalWidth:image.naturalWidth,naturalHeight:image.naturalHeight,fit:is.objectFit,border:is.borderTopWidth,radius:is.borderRadius,shadow:is.boxShadow},backdrop:b.backgroundColor,overflow:document.documentElement.scrollWidth>innerWidth,controls:[...d.querySelectorAll('button')].filter(e=>!e.hidden).map(e=>{const r=e.getBoundingClientRect();return {label:e.getAttribute('aria-label'),x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom}})};
    });
    const tag=`${width}x${height}/${theme}`,d=metrics.dialog,i=metrics.image;
    check(`${tag} fullscreen transparent borderless dialog and dark backdrop`, Math.abs(d.x)<1&&Math.abs(d.y)<1&&Math.abs(d.width-width)<1&&Math.abs(d.height-height)<1&&d.border.every(v=>v==='0px')&&d.radius==='0px'&&d.shadow==='none'&&d.background==='rgba(0, 0, 0, 0)'&&metrics.backdrop.includes('0, 0, 0')&&metrics.backdrop!=='rgba(0, 0, 0, 0)',metrics);
    check(`${tag} image fully contained with no frame, clipping or document overflow`, i.x>=-0.5&&i.y>=-0.5&&i.right<=width+0.5&&i.bottom<=height+0.5&&i.fit==='contain'&&i.border==='0px'&&i.radius==='0px'&&i.shadow==='none'&&!metrics.overflow&&metrics.stage.background==='rgba(0, 0, 0, 0)'&&metrics.stage.border==='0px'&&metrics.stage.shadow==='none');
    check(`${tag} three icon hit targets stay in viewport at least 44px`, metrics.controls.length===3&&metrics.controls.every(c=>c.width>=44&&c.height>=44&&c.x>=0&&c.y>=0&&c.right<=width&&c.bottom<=height));
    const samples=await paintedPhoto(page);check(`${tag} actual photograph pixels paint above the backdrop`,samples.filter(sample=>sample.difference<=45).length>=6,samples);
    if (width===1440 || width===390) {
      const file=`${width===1440?'desktop':'mobile'}-${theme}.png`;
      await page.screenshot({path:path.join(out,file),fullPage:false,animations:'disabled'});
      report.screenshots.push({file,viewport:{width,height},theme,photo:'lake-cc0.jpg',state:'second of three loaded; all three icon controls visible'});
    }
    await context.close();
  }
}
async function fallbacks(browser) {
  const plain = await newPage(browser,{noJS:true});
  check('no-JavaScript moments, home photos and prelinked article image retain native original links', await plain.page.locator('.moment-lightbox').count()===0 && await plain.page.locator('#moment-one').getAttribute('href')==='/photos/lake-1.jpg' && await plain.page.locator('#home-one').getAttribute('href')==='/photos/avatar.png' && await plain.page.locator('#article-one-link').getAttribute('href')==='/photos/article-1.jpg');
  check('no-JavaScript original photo activation opens native image tab', await nativePopup(plain.context,()=>plain.page.locator('#moment-one').click())===origin+'/photos/lake-1.jpg');
  await plain.context.close();
  const unsupported = await newPage(browser,{noDialog:true});
  check('unsupported showModal leaves native links and article image enhancement intact without dialog', await unsupported.page.locator('.moment-lightbox').count()===0 && await unsupported.page.locator('a[data-article-image]').count()===3);
  check('unsupported dialog activation opens native original', await nativePopup(unsupported.context,()=>unsupported.page.locator('#home-one').click())===origin+'/photos/avatar.png');
  await unsupported.context.close();
}
(async()=>{
  fs.mkdirSync(out,{recursive:true});
  const browser = await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE || '/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
  try { await functional(browser); await fallbacks(browser); await geometry(browser); check('no uncaught browser runtime errors',report.errors.length===0,report.errors); check('isolated route boundary has zero business writes or external requests',report.businessWrites===0&&report.unexpectedRequests.length===0,report.unexpectedRequests);report.passed=true; }
  catch(error){report.passed=false;report.failure=String(error);console.error(error.stack||String(error));}
  finally{await browser.close();fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,screenshots:report.screenshots.length,errors:report.errors,failed:report.checks.filter(c=>!c.passed).map(c=>c.name)}));if(!report.passed)process.exitCode=1;}
})();
