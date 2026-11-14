// Isolated DOM/runtime tests only: no app build, candidate URL or business writes.
const fs = require('fs');
const path = require('path');
const ts = require('../../web/frontend/node_modules/typescript');
const {chromium} = require('/tmp/painter-browser/node_modules/playwright');
const base = 'http://card-runtime.test';
const out = path.resolve('tests/artifacts/v24/card-runtime');
const report = {origin: base, mode: 'isolated intercepted browser fixture; TypeScript transpileModule in memory; no app build/service/database', checks: [], errors: [], screenshots: []};
const check = (name, passed, detail) => { report.checks.push({name, passed: !!passed, ...(detail === undefined ? {} : {detail})}); console.log(`${passed ? 'PASS' : 'FAIL'} ${name}`); if (!passed) throw Error(name); };
const source = name => fs.readFileSync(path.resolve('web/frontend/src', name), 'utf8');
const modules = Object.fromEntries(['contentCards', 'momentGallery'].map(name => ['./' + name, ts.transpileModule(source(name + '.ts'), {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText]));
const script = `(() => {const sources=${JSON.stringify(modules)},cache={};function require(name){if(name.endsWith('.css'))return {};if(cache[name])return cache[name].exports;const m={exports:{}};cache[name]=m;new Function('require','module','exports',sources[name])(require,m,m.exports);return m.exports}window.cardRuntime=require('./contentCards');window.galleryRuntime=require('./momentGallery');window.listenerCounts={};const oldDocument=document.addEventListener.bind(document),oldWindow=window.addEventListener.bind(window);document.addEventListener=(type,...args)=>{window.listenerCounts['document:'+type]=(window.listenerCounts['document:'+type]||0)+1;return oldDocument(type,...args)};window.addEventListener=(type,...args)=>{window.listenerCounts['window:'+type]=(window.listenerCounts['window:'+type]||0)+1;return oldWindow(type,...args)};for(let i=0;i<5;i++){window.cardRuntime.initArticleCards();window.galleryRuntime.initMomentGalleries()}})();`;
const styles = ['style.css','shadcn-theme.css','articleCards.css','momentGallery.css'].map(source).join('\n');
const inline = (id, text = '预览链接') => `<span class="content-link" id="${id}"><a class="content-link__anchor" href="/destination?from=${id}" target="_blank" rel="noopener noreferrer">${text}</a><span class="content-link__preview"><span class="content-card__title">Painter · 中文与 emoji 🌿</span><span class="content-card__description">这是隔离运行时测试数据，用于检查提示、触摸与键盘行为。</span><span class="content-card__url">card-runtime.test</span></span></span>`;
function fixture(theme = 'light') {
  return `<!doctype html><html lang="zh-CN" data-theme="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>${styles}\nmain{max-width:760px;margin:auto;padding:24px}.runtime-title{font:600 20px/1.4 var(--font-ui);margin:0 0 20px}.runtime-note{font-size:13px;color:var(--muted)}.runtime-edge{text-align:right}.runtime-images{display:flex;flex-wrap:wrap;gap:12px}.runtime-images img{display:block;width:120px;height:90px;margin:0}.runtime-images .article-image-link{display:block;width:120px}.runtime-spacer{height:20px}.content-link__hint{font:400 12px/18px var(--font-ui);color:var(--muted)}[data-runtime-editable]{border:1px solid var(--line);padding:12px;margin:20px 0}</style></head><body data-page-kind="article"><main><h1 class="runtime-title">文章交互测试 · 隔离数据</h1><p class="runtime-note">此页面只用于共享运行时回归，未连接 Painter 服务。</p><article class="article-reading"><div class="prose"><p class="runtime-edge">${inline('first')}</p><p>${inline('second', '第二处预览')}</p><p><button type="button" id="outside">外部焦点</button></p><div class="runtime-spacer"></div><div class="runtime-images"><img id="ordinary-one" src="/image-one.svg" alt="第一张文章图片"><img id="ordinary-two" src="/image-two.svg" alt="第二张文章图片"><img id="broken" src="/broken.png" alt="加载失败测试"><a id="linked-navigation" href="/destination?from=illustration" target="_blank"><img src="/image-one.svg" alt="有独立导航目的的插图"></a></div><a class="content-card" id="block-card" href="/destination?from=block" target="_blank"><img class="content-card__image" src="/image-one.svg" alt=""><span class="content-card__body"><span class="content-card__title">区块卡片原始链接</span></span></a></div></article><div data-runtime-editable contenteditable="true">${inline('editable','编辑中的链接')}</div><div data-moment-gallery data-gallery-title="动态测试"><a href="/image-two.svg" data-moment-image data-image-alt="已有动态图片" id="moment-link">动态原图</a></div></main><script>${script}</script></body></html>`;
}
async function newPage(browser, {mobile = false, theme = 'light', noJS = false, noDialog = false} = {}) {
  const context = await browser.newContext({viewport: {width: mobile ? 390 : 1440, height: mobile ? 844 : 900}, deviceScaleFactor: 1, isMobile: mobile, hasTouch: mobile, locale: 'zh-CN', colorScheme: theme, javaScriptEnabled: !noJS});
  if (noDialog) await context.addInitScript(() => { HTMLDialogElement.prototype.showModal = undefined; });
  await context.route('**/*', route => {
    const url = new URL(route.request().url());
    if (url.origin !== base) throw Error('Runtime test escaped isolated fixture');
    if (route.request().method() !== 'GET') throw Error('Runtime test attempted mutation');
    if (url.pathname === '/assets/geist-latin.woff2') return route.fulfill({contentType:'font/woff2',body:fs.readFileSync('web/frontend/src/assets/geist-latin.woff2')});
    if (url.pathname.startsWith('/image-')) return route.fulfill({contentType:'image/svg+xml',body:`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="90"><rect width="120" height="90" fill="${url.pathname.includes('one') ? '#50756a' : '#745f4b'}"/></svg>`});
    if (url.pathname === '/broken.png') return route.fulfill({status:404,body:'expected missing fixture image'});
    if (url.pathname === '/destination') return route.fulfill({contentType:'text/html',body:'<!doctype html><title>Original destination</title><p>Native link destination.</p>'});
    return route.fulfill({contentType:'text/html',body:fixture(theme)});
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  page.setDefaultNavigationTimeout(10000);
  page.on('pageerror', error => report.errors.push(error.message));
  await page.goto(base + '/');
  if (!noJS) await page.waitForFunction(() => window.cardRuntime && document.querySelector('#first a')?.getAttribute('aria-expanded') === 'false');
  if (!noJS) await page.evaluate(async () => { await document.fonts.ready; await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))); });
  return {context,page};
}
async function twoFrames(page) { await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)))); }
async function bounds(page, selector) { return page.locator(selector).evaluate(el => { const box = el.getBoundingClientRect(); return {left:box.left,right:box.right,top:box.top,bottom:box.bottom,width:document.documentElement.clientWidth,height:document.documentElement.clientHeight,inner_width:innerWidth}; }); }
async function nativePopup(context, action) { const pending=context.waitForEvent('page');await action();const popup=await pending;await popup.waitForLoadState();const url=popup.url();await popup.close();return url; }
async function desktop(browser) {
  const {context,page} = await newPage(browser);
  const first=page.locator('#first .content-link__anchor'),preview=page.locator('#first .content-link__preview');
  check('repeated initialization binds one document/global handler each',await page.evaluate(() => ['document:pointerdown','document:keydown','document:scroll','window:resize','window:blur'].every(type=>listenerCounts[type]===1)),await page.evaluate(()=>listenerCounts));
  check('ordinary images enhanced; navigation/card images retained',await page.locator('a[data-article-image]').count()===3&&await page.locator('#block-card a,#linked-navigation a').count()===0&&await page.locator('#linked-navigation').getAttribute('href')==='/destination?from=illustration');
  check('one shared viewer after moment and article initialization',await page.locator('.moment-lightbox').count()===1);
  await first.hover();await twoFrames(page);
  check('pointer hover opens linked accessible tooltip',await first.getAttribute('aria-expanded')==='true'&&await preview.getAttribute('aria-hidden')==='false'&&await preview.getAttribute('role')==='tooltip'&&await first.getAttribute('aria-describedby')===await preview.getAttribute('id'));
  let box=await bounds(page,'#first .content-link__preview');check('edge tooltip stays inside viewport',box.left>=15&&box.right<=box.width-15&&box.top>=15&&box.bottom<=box.height-15,box);
  const linkBox=await first.boundingBox();await page.mouse.move(linkBox.x+linkBox.width/2,linkBox.y+linkBox.height+5);await page.waitForTimeout(80);await preview.hover();await page.waitForTimeout(250);check('crossing hover gap and entering popup keeps it visible',await preview.getAttribute('aria-hidden')==='false');
  await page.keyboard.press('Escape');check('Escape dismisses while retaining link focus destination',await preview.getAttribute('aria-hidden')==='true'&&await first.getAttribute('href')==='/destination?from=first');
  await page.mouse.move(0,0);await page.locator('#outside').focus();await first.focus();await twoFrames(page);
  check('keyboard focus opens preview',await first.getAttribute('aria-expanded')==='true');
  await page.keyboard.press('Escape');check('Escape can dismiss a focused preview',await first.evaluate(el=>el===document.activeElement)&&await preview.getAttribute('aria-hidden')==='true');
  await page.locator('#outside').focus();await first.focus();await page.locator('#outside').click();check('outside pointer/focus dismisses preview',await preview.getAttribute('aria-hidden')==='true');
  const opened=await nativePopup(context,()=>first.click());check('ordinary desktop click retains native new-tab target',opened===base+'/destination?from=first');
  await first.focus();const entered=await nativePopup(context,()=>page.keyboard.press('Enter'));check('keyboard Enter retains original navigation',entered===base+'/destination?from=first');
  const modified=await page.evaluate(()=>{const anchor=document.querySelector('#first a');const results=[];for(const modifier of ['ctrlKey','metaKey','shiftKey','altKey']){const event=new MouseEvent('click',{bubbles:true,cancelable:true,button:0,detail:1,[modifier]:true});anchor.addEventListener('click',e=>{results.push({modifier,prevented:e.defaultPrevented});e.preventDefault()},{once:true});anchor.dispatchEvent(event)}return results});check('modifier navigation is never intercepted',modified.every(result=>!result.prevented),modified);
  await page.locator('#ordinary-one').click();await page.waitForFunction(()=>document.querySelector('.moment-lightbox[open] .moment-lightbox-stage img')?.naturalWidth>0);
  check('article image uses existing viewer with real alt and group count',await page.locator('#moment-lightbox-title').innerText()==='文章图片'&&await page.locator('.moment-lightbox-position').innerText()==='1 / 3'&&await page.locator('.moment-lightbox-stage img').getAttribute('alt')==='第一张文章图片');
  await page.getByRole('button',{name:'下一张图片',exact:true}).click();check('article next control follows image order',await page.locator('.moment-lightbox-position').innerText()==='2 / 3');
  await page.keyboard.press('End');await page.getByText('图片暂时无法加载，请尝试打开原图。',{exact:true}).waitFor();check('broken image exposes original-link recovery',await page.locator('.moment-lightbox-actions a').getAttribute('href')===base+'/broken.png');
  await page.keyboard.press('Home');await page.waitForFunction(()=>!document.querySelector('.moment-lightbox-stage img').hidden);
  await page.keyboard.press('Escape');check('viewer Escape returns focus to article image link',await page.locator('.moment-lightbox[open]').count()===0&&await page.locator('#ordinary-one').evaluate(el=>el.closest('a')===document.activeElement));
  await page.locator('#moment-link').click();check('moment still uses same viewer and own title',await page.locator('.moment-lightbox').count()===1&&await page.locator('#moment-lightbox-title').innerText()==='动态测试');await page.getByRole('button',{name:'关闭图片预览',exact:true}).click();
  const dynamic=await page.evaluate(()=>{const holder=document.createElement('div');holder.innerHTML=document.querySelector('#second').outerHTML.replace('id="second"','id="dynamic"');const node=holder.firstElementChild;document.body.append(node);cardRuntime.initArticleCards(node);cardRuntime.initArticleCards(node);const uniqueID=node.querySelector('.content-link__preview').id!==document.querySelector('#second .content-link__preview').id;const describedBy=node.querySelector('a').getAttribute('aria-describedby');const currentID=node.querySelector('.content-link__preview').id;node.querySelector('a').focus();node.remove();cardRuntime.initArticleCards();return {hintCount:node.querySelectorAll('.content-link__hint').length,uniqueID,descriptionMatches:describedBy===currentID,listenerCounts};});check('dynamic root scans are idempotent and removed roots do not add handlers',dynamic.hintCount===1&&dynamic.uniqueID&&dynamic.descriptionMatches&&dynamic.listenerCounts['document:pointerdown']===1,dynamic);
  const detached=await page.evaluate(()=>{const fragment=document.createDocumentFragment();const holder=document.createElement('div');holder.innerHTML=document.querySelector('#second').outerHTML;fragment.append(holder);cardRuntime.initArticleCards(fragment);return !!fragment.querySelector('.content-link__preview').id});check('unmounted fragments initialize without loops',detached);
  const editable=await page.evaluate(()=>{const anchor=document.querySelector('#editable a');const event=new MouseEvent('click',{bubbles:true,cancelable:true,button:0,detail:1});let prevented;anchor.addEventListener('click',e=>{prevented=e.defaultPrevented;e.preventDefault()},{once:true});anchor.dispatchEvent(event);return {editable:anchor.isContentEditable,prevented};});check('editable inline links keep editor click handling',editable.editable&&!editable.prevented,editable);
  await page.evaluate(()=>window.scrollTo({top:0,behavior:'instant'}));await first.focus();await twoFrames(page);
  const file='desktop-light-inline.png';await page.screenshot({path:path.join(out,file),fullPage:false});report.screenshots.push(file);
  check('desktop has no document overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth));
  await context.close();
}
async function mobile(browser) {
  const {context,page}=await newPage(browser,{mobile:true,theme:'dark'});const first=page.locator('#first .content-link__anchor'),preview=page.locator('#first .content-link__preview');
  check('closed previews do not expand the initial mobile document',await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth));
  let popups=0;context.on('page',()=>popups++);
  await first.tap();await twoFrames(page);check('first real touch opens preview without navigation',popups===0&&await first.getAttribute('aria-expanded')==='true'&&await preview.getAttribute('aria-hidden')==='false'&&await page.locator('#first .content-link__hint').isVisible());
  const box=await bounds(page,'#first .content-link__preview');check('touch preview is inside narrow viewport',box.left>=15&&box.right<=box.width-15&&box.top>=15&&box.bottom<=box.height-15,box);
  const openViewport=await page.evaluate(()=>({scroll:document.documentElement.scrollWidth,client:document.documentElement.clientWidth,inner:innerWidth}));
  check('open touch preview does not widen the document or visual viewport',openViewport.scroll<=openViewport.client&&openViewport.inner===openViewport.client,openViewport);
  const file='mobile-dark-inline.png';await page.screenshot({path:path.join(out,file),fullPage:false});report.screenshots.push(file);
  const destination=await nativePopup(context,()=>first.tap());check('second real touch opens original new-tab destination',destination===base+'/destination?from=first');
  await first.tap();await page.locator('#outside').tap();check('touch outside closes preview',await preview.getAttribute('aria-hidden')==='true');
  await first.tap();await page.keyboard.press('Escape');check('touch preview also dismisses with Escape',await preview.getAttribute('aria-hidden')==='true');
  await page.locator('#ordinary-one').tap();await page.waitForFunction(()=>document.querySelector('.moment-lightbox[open] .moment-lightbox-stage img')?.naturalWidth>0);check('touch article image opens shared viewer',await page.locator('.moment-lightbox[open]').count()===1);await page.getByRole('button',{name:'关闭图片预览',exact:true}).tap();
  check('mobile has no document overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=document.documentElement.clientWidth));await context.close();
}
async function fallbacks(browser) {
  const plain=await newPage(browser,{noJS:true});check('no-JavaScript card and original gallery destinations remain native',await plain.page.locator('#first a').getAttribute('href')==='/destination?from=first'&&await plain.page.locator('#first a').getAttribute('target')==='_blank'&&await plain.page.locator('#moment-link').getAttribute('href')==='/image-two.svg'&&await plain.page.locator('.moment-lightbox').count()===0);await plain.context.close();
  const unsupported=await newPage(browser,{noDialog:true});check('unsupported dialog retains ordinary image links',await unsupported.page.locator('a[data-article-image]').count()===3&&await unsupported.page.locator('.moment-lightbox').count()===0&&await unsupported.page.locator('#ordinary-one').evaluate(el=>el.closest('a').href.endsWith('/image-one.svg')));await unsupported.context.close();
}
(async()=>{fs.mkdirSync(out,{recursive:true});const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});try{await desktop(browser);await mobile(browser);await fallbacks(browser);check('no uncaught browser runtime errors',report.errors.length===0,report.errors);report.passed=report.checks.every(c=>c.passed)}catch(error){report.failure=String(error);report.passed=false;console.error(String(error))}finally{await browser.close();fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors,screenshots:report.screenshots,failed:report.checks.filter(c=>!c.passed).map(c=>c.name)}));if(!report.passed)process.exitCode=1}})();
