// Anonymous, read-only search regression. --baseline records incumbent material.
const fs = require('fs');
const path = require('path');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || '/tmp/painter-browser/node_modules/playwright');

if (process.env.PAINTER_V37_RUN !== '1') throw Error('Set PAINTER_V37_RUN=1 after the target is ready.');
const base = (process.env.PAINTER_BASE || 'http://127.0.0.1:18086').replace(/\/$/, '');
const baseline = process.argv.includes('--baseline');
const out = path.resolve(process.env.PAINTER_V37_ARTIFACTS || 'tests/artifacts/v37');
const prefix = baseline ? 'before-' : 'after-';
const report = {base, stage: baseline ? 'before' : new URL(base).port === '18080' ? 'formal' : 'candidate', read_only: true, checks: [], captures: [], materials: [], touch_targets: [], limitations: [], simulated_states: [], simulated_responses: [], blocked_writes: [], errors: [], bad_responses: []};
let browser;
function check(name, passed, detail) {
  report.checks.push({name, passed: !!passed, ...(detail === undefined ? {} : {detail})});
  console.log(`${passed ? 'PASS' : 'FAIL'} ${name}`);
}
async function ready(page) {
  await page.waitForFunction(() => document.querySelector('#site-search')?.dataset.raycastSearchReady === 'true');
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(100);
}
async function context(options = {}) {
  const c = await browser.newContext({viewport: options.viewport || {width:1440,height:1000}, colorScheme: options.theme || 'light', hasTouch: !!options.mobile, isMobile: !!options.mobile, javaScriptEnabled: options.javaScriptEnabled !== false, ...(options.userAgent ? {userAgent:options.userAgent} : {})});
  await c.addInitScript(({theme, solid}) => { try { localStorage.setItem('painter-theme', theme); localStorage.setItem('painter-solid', solid ? '1' : '0'); localStorage.setItem('painter-analytics', 'off'); } catch {} }, {theme: options.theme || 'light', solid: !!options.solid});
  await c.route('**/*', route => {
    const req = route.request(), u = new URL(req.url());
    if (u.pathname === '/api/v1/analytics/events') return route.fulfill({status:204});
    if (!['GET','HEAD'].includes(req.method())) { report.blocked_writes.push({method:req.method(),path:u.pathname}); return route.fulfill({status:204}); }
    return route.continue();
  });
  c.on('page', p => {
    p.on('pageerror', e => report.errors.push(e.message));
    p.on('response', r => { const u = new URL(r.url()); if (u.origin === new URL(base).origin && r.status() >= 400) { const item={status:r.status(),path:u.pathname}; if(r.headers()['x-painter-test-simulated']==='v37')report.simulated_responses.push(item);else report.bad_responses.push(item); } });
  });
  return c;
}
async function capture(page, name) {
  const file = prefix + name + '.png';
  await page.screenshot({path:path.join(out,file)});
  report.captures.push(file);
}
async function settledSuggestions(page) {
  await page.waitForFunction(() => { const d=document.querySelector('#site-search'); return d?.open && !d.hasAttribute('data-search-loading') && !!d.querySelector('[data-search-status]')?.textContent; });
}
async function dismiss(page) {
  await page.keyboard.press('Escape');
  // HTMLDialogElement queues its close event; observe that event's ARIA update.
  await page.waitForFunction(()=>{const d=document.querySelector('#site-search'),q=document.querySelector('#quick-search');return d&&!d.open&&q?.getAttribute('aria-expanded')==='false';},null,{timeout:1500});
}
async function material(page, label) {
  const value = await page.evaluate(() => {
    const get = sel => { const e=document.querySelector(sel); if(!e)return null; const s=getComputedStyle(e),b=e.querySelector(':scope > .liquid-glass-optics > .liquid-glass-backdrop'); return {mode:e.dataset.liquidGlassMode||null,ready:e.dataset.liquidGlassReady||null,background:s.backgroundColor,backdrop:s.backdropFilter,filter:b?getComputedStyle(b).backdropFilter:null,optics:e.querySelectorAll(':scope > .liquid-glass-optics').length}; };
    return {dialog:get('#site-search'),header:get('.site-header'),dock:get('.dock-glass'),solid:document.documentElement.classList.contains('reduce-transparency')};
  });
  report.materials.push({label,...value});
  if (!baseline) check(`${label}: search uses one shared glass layer`, value.dialog?.ready === 'true' && value.dialog.optics === 1 && ['refract','blur','solid'].includes(value.dialog.mode), value);
  return value;
}
async function bounds(page) {
  return page.evaluate(() => {const d=document.querySelector('#site-search'),r=d.getBoundingClientRect();return {left:r.left,top:r.top,right:r.right,bottom:r.bottom,width:r.width,height:r.height,inside:r.left>=-1&&r.top>=-1&&r.right<=innerWidth+1&&r.bottom<=innerHeight+1,overflow:document.documentElement.scrollWidth>innerWidth+1};});
}
async function glassSurface(page, selector) {
  return page.locator(selector).evaluate(e=>{const r=e.getBoundingClientRect(),o=e.querySelector(':scope > .liquid-glass-optics'),b=o?.getBoundingClientRect();return{mode:e.dataset.liquidGlassMode,ready:e.dataset.liquidGlassReady,optics:e.querySelectorAll(':scope > .liquid-glass-optics').length,width:r.width,height:r.height,texture:e.dataset.glassTextureSize||'',cover:!!b&&r.width>0&&r.height>0&&Math.abs(b.left-r.left)<1&&Math.abs(b.top-r.top)<1&&Math.abs(b.width-r.width)<1&&Math.abs(b.height-r.height)<1};});
}
async function matrix(width,height,theme) {
  const label=`${width}-${theme}`, c=await context({viewport:{width,height},theme,mobile:width<721}), p=await c.newPage();
  await p.goto(base+'/search'); await ready(p);
  check(`${label}: semantic GET search and owned filters remain`, await p.locator('[data-search-filters]').evaluate(e=>e.method==='get'&&new URL(e.action).pathname==='/search'&&!!e.querySelector('[data-tag-filter] summary')&&!!e.querySelector('[data-sort-filter] summary')) && await p.locator('select').count()===0);
  check(`${label}: header retains native search href`, await p.locator('[data-search-open]').getAttribute('href')==='/search');
  check(`${label}: page has no horizontal overflow`, await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  if(!baseline) {
    const surfaces=await p.locator('[data-search-filters] .search-query-field,[data-search-filters] summary').evaluateAll(es=>es.map(e=>({ready:e.dataset.liquidGlassReady,mode:e.dataset.liquidGlassMode,optics:e.querySelectorAll(':scope > .liquid-glass-optics').length})));
    check(`${label}: page keyword field and filter summaries share glass`,surfaces.length===3&&surfaces.every(e=>e.ready==='true'&&e.optics===1&&['refract','blur','solid'].includes(e.mode)),surfaces);
  }
  await capture(p,label+'-page');
  const trigger=p.locator('[data-search-open]'); await trigger.focus();
  await p.keyboard.press('Control+k'); await settledSuggestions(p);
  check(`${label}: shortcut opens combobox with input focus`, await p.locator('#quick-search').evaluate(e=>e===document.activeElement&&e.getAttribute('aria-expanded')==='true') && await p.locator('#site-search').evaluate(e=>e.open));
  const commands=await p.locator('#site-search [data-search-kind=command] a').evaluateAll(es=>es.map(e=>({text:e.textContent,href:e.getAttribute('href')})));
  check(`${label}: real blank-query site entries stay navigable`, ['/posts','/moments','/phrases'].every(href=>commands.some(e=>e.href===href)),commands);
  const b=await bounds(p); check(`${label}: palette is bounded without horizontal overflow`,b.inside&&!b.overflow,b);
  await material(p,label);
  const touchTargets=await p.locator('#site-search .quick-search-row button').evaluateAll(es=>es.map(e=>{const r=e.getBoundingClientRect();return{label:e.getAttribute('aria-label'),width:r.width,height:r.height};}));
  report.touch_targets.push({label,targets:touchTargets});
  if(!baseline&&width<721)check(`${label}: submit and close touch areas are at least 44px`,touchTargets.length===2&&touchTargets.every(e=>e.width>=44&&e.height>=44),touchTargets);
  await capture(p,label+'-palette');
  await p.keyboard.press('ArrowDown');
  check(`${label}: arrow selection owns a real active descendant`,await p.locator('#quick-search').evaluate(e=>{const id=e.getAttribute('aria-activedescendant'),o=id&&document.getElementById(id);return !!o&&o.getAttribute('aria-selected')==='true'&&!!o.querySelector('a[href]');}));
  await dismiss(p);
  check(`${label}: Escape closes and restores opener focus`,await p.locator('#site-search').evaluate(e=>!e.open)&&await trigger.evaluate(e=>e===document.activeElement)&&await p.locator('#quick-search').getAttribute('aria-expanded')==='false');
  await p.keyboard.press('/'); await settledSuggestions(p);
  check(`${label}: slash opens outside editable fields`,await p.locator('#quick-search').evaluate(e=>e===document.activeElement));
  await p.locator('#site-search [data-search-kind=command] a[href="/moments"]').click(); await p.waitForURL('**/moments'); await ready(p);
  check(`${label}: clicking a real entry reaches its page`,new URL(p.url()).pathname==='/moments'&&await p.locator('body').getAttribute('data-page-kind')==='moments');
  await p.locator('[data-search-open]').click(); await settledSuggestions(p);
  await p.locator('#quick-search').fill('__painter_v37_no_match__'); await settledSuggestions(p);
  const status=await p.locator('[data-search-status]').innerText();
  check(`${label}: no-match state retains full search path`,await p.locator('#site-search [role=option]').count()===0&&status.includes('没有建议')&&(await p.locator('.search-dialog-foot a').getAttribute('href'))==='/search?q=__painter_v37_no_match__',status);
  await p.keyboard.press('Enter'); await p.waitForURL('**/search?q=__painter_v37_no_match__'); await ready(p);
  check(`${label}: Enter falls through to real empty search results`,await p.locator('.empty').innerText().then(t=>t.includes('没有找到匹配的文章')));
  await p.locator('[data-sort-filter] summary').click();
  const popup=await p.locator('[data-sort-filter] .search-filter-popover').evaluate(e=>{const r=e.getBoundingClientRect();return{left:r.left,right:r.right,top:r.top,bottom:r.bottom,inside:r.left>=0&&r.right<=innerWidth+1&&r.top>=0&&r.bottom<=innerHeight+1};});
  check(`${label}: custom sort popup stays in viewport`,popup.inside,popup);
  if(!baseline){const s=await glassSurface(p,'[data-sort-filter] .search-filter-popover');check(`${label}: opened sort popup has one measured glass layer`,s.ready==='true'&&s.optics===1&&s.cover,s);}
  await p.getByRole('radio',{name:'最近发布',exact:false}).check();
  check(`${label}: native sort choice updates label and returns focus`,await p.locator('[data-sort-label]').innerText()==='最近发布'&&await p.locator('[data-sort-filter] summary').evaluate(e=>e===document.activeElement));
  await p.locator('[data-tag-filter] summary').click();
  if(!baseline){
    await p.waitForTimeout(100);const s=await glassSurface(p,'[data-tag-filter] .search-filter-popover');
    const b=await p.locator('[data-tag-filter] .search-filter-popover').evaluate(e=>{const r=e.getBoundingClientRect();return{left:r.left,right:r.right,top:r.top,bottom:r.bottom,inside:r.left>=0&&r.right<=innerWidth+1&&r.top>=0&&r.bottom<=innerHeight+1};});
    check(`${label}: opened tags are bounded with a measured glass layer`,s.ready==='true'&&s.optics===1&&s.cover&&b.inside,{surface:s,bounds:b});
    if((width===1440&&theme==='light')||(width===390&&theme==='dark'))await capture(p,label+'-tag-glass');
  }
  const tags=await p.locator('input[name=tag_id]').count();
  if(tags) { await p.locator('input[name=tag_id]').first().check(); check(`${label}: real tag produces owned selection chip`,await p.locator('button.search-selected-tag').count()===1); }
  else report.limitations.push(`${label}: no public tags; fixture creation deliberately omitted.`);
  await p.keyboard.press('Escape');
  check(`${label}: tag Escape restores its disclosure focus`,await p.locator('[data-tag-filter]').evaluate(e=>!e.open)&&await p.locator('[data-tag-filter] summary').evaluate(e=>e===document.activeElement));
  await c.close();
}
async function simulatedStates(width,height,theme) {
  const label=`${width}-${theme}-simulated`,c=await context({viewport:{width,height},theme,mobile:width<721}),p=await c.newPage();
  const fixtureItems=Array.from({length:28},(_,i)=>({kind:i%4===0?'tag':'post',label:`仅浏览器模拟建议 ${String(i+1).padStart(2,'0')} · 长列表滚动与键盘选择`,url:i%2?'/moments':'/posts'}));
  let slowDone=false;
  await c.route('**/api/v1/search/suggestions?*',async route=>{
    if(route.request().method()!=='GET')return route.fallback();
    const q=new URL(route.request().url()).searchParams.get('q');
    if(q==='__v37_long__')return route.fulfill({json:{items:fixtureItems,query:q},headers:{'x-painter-test-simulated':'v37'}});
    if(q==='__v37_short__')return route.fulfill({json:{items:fixtureItems.slice(0,1),query:q},headers:{'x-painter-test-simulated':'v37'}});
    if(q==='__v37_error__')return route.fulfill({status:503,json:{error:{code:'TEST_ONLY'}},headers:{'x-painter-test-simulated':'v37'}});
    if(q==='__v37_slow__'){await new Promise(r=>setTimeout(r,600));slowDone=true;return route.fulfill({json:{items:[{kind:'post',label:'过时的浏览器模拟响应',url:'/posts'}]},headers:{'x-painter-test-simulated':'v37'}}).catch(()=>{});}
    return route.fallback();
  });
  report.simulated_states.push({label,description:'Only suggestion GET responses are browser fixtures; 28 synthetic labels link to real existing /posts or /moments. No published article or database fixture is created.'});
  await p.goto(base+'/search');await ready(p);const trigger=p.locator('[data-search-open]');await trigger.focus();await p.keyboard.press('Meta+k');await settledSuggestions(p);
  check(`${label}: Command+K opens with input focus`,await p.locator('#quick-search').evaluate(e=>e===document.activeElement));
  await p.locator('#quick-search').fill('__v37_long__');await settledSuggestions(p);await p.waitForTimeout(100);
  check(`${label}: all simulated suggestions retain real links`,await p.locator('#site-search [role=option]').count()===28&&await p.locator('#site-search [role=option] a').evaluateAll(es=>es.every(e=>['/posts','/moments'].includes(e.getAttribute('href')))));
  const geometry=()=>p.evaluate(()=>{const d=document.querySelector('#site-search'),r=e=>{const b=e.getBoundingClientRect();return{left:b.left,top:b.top,width:b.width,height:b.height};},l=d.querySelector('[role=listbox]');return{dialog:r(d),form:r(d.querySelector('form')),foot:r(d.querySelector('.search-dialog-foot')),optics:r(d.querySelector(':scope > .liquid-glass-optics')),dialogScroll:d.scrollTop,listScroll:l.scrollTop,listClient:l.clientHeight,listHeight:l.scrollHeight,scroll:scrollY};});
  const before=await geometry(),list=p.locator('#search-suggestions'),lr=await list.boundingBox();
  await p.mouse.move(lr.x+lr.width*.5,lr.y+lr.height*.5);await p.mouse.wheel(0,700);await p.waitForTimeout(150);const after=await geometry();
  const delta=(a,b)=>Math.max(...['left','top','width','height'].map(k=>Math.abs(a[k]-b[k])));
  check(`${label}: only the long suggestion list scrolls`,after.listScroll>100&&after.listHeight>after.listClient&&after.dialogScroll===0&&before.scroll===after.scroll,{before,after});
  check(`${label}: input footer and optics stay fixed while scrolling`,['dialog','form','foot','optics'].every(k=>delta(before[k],after[k])<1),{before,after});
  await capture(p,`${width}-${theme}-simulated-long-list`);
  await p.locator('.search-dialog-foot a').focus();await p.keyboard.press('Tab');
  const tabBoundary=await p.evaluate(()=>({id:document.activeElement?.id,tag:document.activeElement?.tagName,inside:document.querySelector('#site-search').contains(document.activeElement),documentFocused:document.hasFocus()}));
  const browserChrome=tabBoundary.tag==='BODY'&&!tabBoundary.documentFocused;
  if(browserChrome)await p.keyboard.press('Tab');
  check(`${label}: native Tab boundary returns to input without focusing background controls`,(tabBoundary.inside||browserChrome)&&await p.locator('#quick-search').evaluate(e=>e===document.activeElement),{firstBoundary:tabBoundary,browserChromeHop:browserChrome});
  await dismiss(p);check(`${label}: closing a scrolled list restores the opener`,await trigger.evaluate(e=>e===document.activeElement));
  await trigger.click();await settledSuggestions(p);await p.waitForTimeout(100);const reopened=await glassSurface(p,'#site-search');
  check(`${label}: reopening keeps one correctly measured lens`,reopened.ready==='true'&&reopened.optics===1&&reopened.cover&&reopened.texture&&!reopened.texture.startsWith('0x'),reopened);
  await p.locator('#quick-search').fill('__v37_short__');await settledSuggestions(p);await p.waitForTimeout(120);const short=await glassSurface(p,'#site-search');
  check(`${label}: shrinking suggestions remeasures the same lens`,short.cover&&short.optics===1&&short.height<reopened.height,short);
  await p.locator('#quick-search').fill('__v37_error__');await settledSuggestions(p);
  check(`${label}: simulated unavailable suggestions preserve fallback search`,(await p.locator('[data-search-status]').innerText()).includes('建议暂不可用')&&await p.locator('.search-dialog-foot a').getAttribute('href')==='/search?q=__v37_error__');
  await p.locator('#quick-search').fill('__v37_slow__');await p.waitForTimeout(240);await p.locator('#quick-search').fill('__v37_short__');await settledSuggestions(p);await p.waitForTimeout(650);
  check(`${label}: stale simulated response cannot replace newer input`,slowDone&&await p.locator('#site-search [role=option]').count()===1&&!(await p.locator('#search-suggestions').innerText()).includes('过时的浏览器模拟响应'));
  await dismiss(p);await p.locator('#search-query').focus();await p.keyboard.press('Control+k');
  check(`${label}: shortcut yields to an existing editable field`,await p.locator('#site-search').evaluate(e=>!e.open));
  await c.close();
}
async function imageBackground(width,height,theme) {
  const label=`${width}-${theme}-home-image`,c=await context({viewport:{width,height},theme,mobile:width<721}),p=await c.newPage();await p.goto(base+'/');await ready(p);
  await p.waitForFunction(()=>document.querySelector('[data-game-story]')?.classList.contains('is-enhanced'));
  await p.evaluate(()=>{const game=document.querySelector('[data-game-story]'),stage=document.querySelector('[data-game-stage]');scrollTo({top:scrollY+game.getBoundingClientRect().top+1.6*stage.clientHeight,behavior:'instant'});});await p.waitForTimeout(180);
  await p.evaluate(async()=>{for(const i of document.querySelectorAll('[data-game-frame]:first-child img'))try{await i.decode();}catch{}});
  const background=await p.locator('[data-game-frame]:first-child [data-game-hero]').evaluate(e=>({src:e.currentSrc||e.src,decoded:e.complete&&e.naturalWidth>0}));
  await p.evaluate(()=>{if(document.activeElement instanceof HTMLElement)document.activeElement.blur();});await p.keyboard.press('Control+k');await settledSuggestions(p);await p.waitForTimeout(100);
  check(`${label}: palette works over the real homepage image`,background.decoded&&await p.locator('#quick-search').evaluate(e=>e===document.activeElement),background);
  await material(p,label);check(`${label}: image-backed palette stays bounded`,(await bounds(p)).inside);await capture(p,`${width}-${theme}-home-image-palette`);await dismiss(p);
  await c.close();
}
async function preferences() {
  const c=await context({viewport:{width:390,height:844},theme:'dark',mobile:true}),p=await c.newPage(); await p.goto(base+'/search');await ready(p);
  await p.locator('.floating-theme > summary').click(); await p.locator('[data-solid-toggle]').click();
  check('mobile: existing reduced-transparency control persists preference',await p.evaluate(()=>localStorage.getItem('painter-solid')==='1'&&document.documentElement.classList.contains('reduce-transparency')));
  await p.keyboard.press('Escape'); await p.locator('[data-search-open]').click();await settledSuggestions(p);
  const m=await material(p,'390-dark-solid');
  if(!baseline)check('mobile: search adopts the shared solid fallback',m.dialog?.mode==='solid');
  check('mobile: solid preference preserves input and commands',await p.locator('#quick-search').evaluate(e=>e===document.activeElement)&&await p.locator('#site-search [data-search-kind=command]').count()===3);
  await capture(p,'390-dark-solid-palette');
  // Browser-only IME events exercise the existing composition guard; no API writes.
  await p.locator('#quick-search').dispatchEvent('compositionstart');await p.locator('#quick-search').fill('输入');await p.keyboard.press('Escape');
  check('IME composition: Escape does not close ongoing composition',await p.locator('#site-search').evaluate(e=>e.open));
  await p.locator('#quick-search').dispatchEvent('compositionend');await settledSuggestions(p);await dismiss(p);
  check('IME completion: Escape resumes normal dismissal',await p.locator('#site-search').evaluate(e=>!e.open));
  if(!baseline) {
    await p.locator('.floating-theme > summary').click();await p.locator('[data-solid-toggle]').click();await p.keyboard.press('Escape');await p.locator('[data-search-open]').click();await settledSuggestions(p);
    const client=await c.newCDPSession(p);await client.send('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-transparency',value:'reduce'}]});
    await p.waitForFunction(()=>document.querySelector('#site-search')?.dataset.liquidGlassMode==='solid');
    check('live system reduced-transparency: open palette becomes solid without losing focus',await p.locator('#quick-search').evaluate(e=>e===document.activeElement)&&await p.locator('#site-search').evaluate(e=>e.open));
    await client.send('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-transparency',value:'no-preference'}]});await p.waitForFunction(()=>document.querySelector('#site-search')?.dataset.liquidGlassMode!=='solid');
    check('live system reduced-transparency: returning preference restores shared material',await p.locator('#site-search').evaluate(e=>['refract','blur'].includes(e.dataset.liquidGlassMode)));
  }
  await c.close();
  const n=await context({viewport:{width:390,height:844},javaScriptEnabled:false}),q=await n.newPage();await q.goto(base+'/search');
  check('no-JS: header keeps real search navigation',await q.locator('[data-search-open]').getAttribute('href')==='/search');
  await q.locator('#search-query').fill('__painter_v37_no_match__');await q.locator('[data-search-filters] button.primary').click();await q.waitForURL('**/search?q=__painter_v37_no_match__**');
  check('no-JS: semantic keyword form reaches empty result page',new URL(q.url()).searchParams.get('q')==='__painter_v37_no_match__'&&(await q.locator('.empty').innerText()).includes('没有找到匹配的文章'));
  await n.close();
}
async function fallbackBranches() {
  report.limitations.push('UA blur fallback and forced-colors are exercised in Chromium only; this is not a WebKit/Safari, physical phone, or software-keyboard test.');
  const c=await context({viewport:{width:390,height:844},theme:'dark',mobile:true,userAgent:'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15'}),p=await c.newPage();
  await p.goto(base+'/search');await ready(p);await p.locator('[data-search-open]').click();await settledSuggestions(p);const m=await material(p,'390-UA-blur-Chromium');
  check('Chromium UA branch: fallback uses CSS blur without SVG refraction',m.dialog?.mode==='blur'&&m.dialog?.filter?.includes('blur(')&&!m.dialog?.filter?.includes('url('),m.dialog);
  check('Chromium UA branch: commands and input remain available',await p.locator('#quick-search').evaluate(e=>e===document.activeElement)&&await p.locator('#site-search [data-search-kind=command]').count()===3);
  await capture(p,'390-blur-branch-palette');await dismiss(p);await c.close();
  const f=await context({viewport:{width:390,height:844},theme:'light',mobile:true}),q=await f.newPage();await q.emulateMedia({forcedColors:'active'});await q.goto(base+'/search');await ready(q);await q.locator('[data-search-open]').click();await settledSuggestions(q);
  const forced=await material(q,'390-forced-colors-Chromium');const b=await bounds(q);
  check('Chromium forced-colors: solid material preserves combobox and bounds',forced.dialog?.mode==='solid'&&b.inside&&await q.locator('#quick-search').evaluate(e=>e===document.activeElement&&getComputedStyle(e).color!==getComputedStyle(document.querySelector('#site-search > .liquid-glass-optics')).backgroundColor),{material:forced,bounds:b});
  await capture(q,'390-forced-colors-palette');await dismiss(q);check('Chromium forced-colors: Escape still returns focus',await q.locator('[data-search-open]').evaluate(e=>e===document.activeElement));await f.close();
}
(async()=>{
  fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
  for(const theme of ['light','dark'])for(const [w,h]of [[1440,1000],[390,844]])await matrix(w,h,theme);
  await preferences();
  if(!baseline)for(const [w,h,theme]of [[1440,1000,'light'],[390,844,'dark']]){await simulatedStates(w,h,theme);await imageBackground(w,h,theme);}
  if(!baseline)await fallbackBranches();
  check('no browser runtime or unexpected same-origin HTTP errors',!report.errors.length&&!report.bad_responses.length,{errors:report.errors,bad_responses:report.bad_responses,expected_simulated_responses:report.simulated_responses});
})().catch(e=>{report.fatal=String(e.stack||e);console.error(report.fatal);process.exitCode=1;}).finally(async()=>{
  report.passed=!report.fatal&&report.checks.every(c=>c.passed);if(!report.passed)process.exitCode=1;
  const file=path.join(out,baseline?'baseline-report.json':'search-glass-report.json');fs.mkdirSync(out,{recursive:true});fs.writeFileSync(file,JSON.stringify(report,null,2));
  await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,captures:report.captures.length,report:file}));
});
