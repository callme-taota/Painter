// V24 candidate public acceptance. Read-only: all browser writes/analytics blocked.
const fs = require('fs');
const path = require('path');
const {chromium} = require('/tmp/painter-browser/node_modules/playwright');
const base = 'http://192.168.2.209:18081';
const out = path.resolve('tests/artifacts/v24/visual');
const fixture = JSON.parse(fs.readFileSync('tests/artifacts/v24/article-fixture.json', 'utf8'));
const report = {origin:base, read_only:true, checks:[], errors:[], console_errors:[], blocked_writes:[], analytics_blocked:0, failed_requests:[], bad_responses:[], captures:[], evidence:{}, limitations:[]};
let browser;
function check(name, passed, detail) { report.checks.push({name,passed:!!passed,...(detail===undefined?{}:{detail})}); console.log(`${passed?'PASS':'FAIL'} ${name}`); }
const cleanURL = raw => { const u=new URL(raw); return u.origin+u.pathname; };
async function protect(context) {
  await context.route('**/*', async route => {
    const r=route.request(),u=new URL(r.url());
    if (u.pathname.includes('/analytics/')) { report.analytics_blocked++; return route.fulfill({status:200,json:{accepted:0}}); }
    if (!['GET','HEAD'].includes(r.method())) { report.blocked_writes.push({method:r.method(),url:cleanURL(r.url())}); return route.fulfill({status:204,body:''}); }
    // Verify external href/navigation without loading third-party destination apps.
    if (u.origin!==base && r.isNavigationRequest() && r.resourceType()==='document') return route.fulfill({status:200,contentType:'text/html',body:'<!doctype html><title>Read-only destination verification</title>'});
    return route.continue();
  });
}
async function context(options={}) {
  const c=await browser.newContext({viewport:{width:1440,height:1000},colorScheme:'light',...options});
  c.setDefaultTimeout(15000); c.setDefaultNavigationTimeout(30000);
  await protect(c);
  await c.addInitScript(theme=>localStorage.setItem('painter-theme',theme),options.colorScheme||'light');
  return c;
}
async function page(c,label) {
  const p=await c.newPage();
  p.on('pageerror',e=>report.errors.push({page:label,error:e.message}));
  p.on('console',m=>{if(m.type()==='error')report.console_errors.push({page:label,error:m.text()});});
  p.on('requestfailed',r=>{const error=r.failure()?.errorText;if(error!=='net::ERR_ABORTED')report.failed_requests.push({page:label,url:cleanURL(r.url()),type:r.resourceType(),error});});
  p.on('response',r=>{if(r.status()>=400)report.bad_responses.push({page:label,url:cleanURL(r.url()),status:r.status(),type:r.request().resourceType()});});
  return p;
}
async function get(c,pathname) {
  if(!pathname.startsWith('/')||pathname.startsWith('//'))throw Error('Read-only candidate URL guard');
  const r=await c.request.get(base+pathname);let data;try{data=await r.json()}catch{data={}}return {status:r.status(),data};
}
async function all(c,pathname) {
  const items=[];let cursor='',pages=0;
  do { const r=await get(c,pathname+(pathname.includes('?')?'&':'?')+'limit=50'+(cursor?'&cursor='+encodeURIComponent(cursor):''));if(r.status!==200)throw Error(pathname+' GET '+r.status);items.push(...r.data.items);cursor=r.data.next_cursor;pages++;if(pages>110)throw Error('bounded read pagination exceeded'); }while(cursor);
  return items;
}
async function ready(p) { await p.evaluate(()=>document.fonts.ready); await frames(p); }
async function frames(p) { await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))); }
async function scroll(p,selector,top=80) { await p.locator(selector).first().evaluate((e,offset)=>scrollTo({top:scrollY+e.getBoundingClientRect().top-offset,behavior:'instant'}),top);await frames(p); }
async function noOverflow(p,label) { const g=await p.evaluate(()=>({client:document.documentElement.clientWidth,scroll:document.documentElement.scrollWidth,inner:innerWidth}));check(label,g.scroll<=g.client+1&&g.inner<=g.client+1,g); }
async function visibleImages(p) {
  return p.locator('img').evaluateAll(images=>images.filter(img=>{const r=img.getBoundingClientRect();if(!r.width||!r.height||r.bottom<=0||r.top>=innerHeight||r.right<=0||r.left>=document.documentElement.clientWidth)return false;for(let el=img;el;el=el.parentElement){const s=getComputedStyle(el);if(el.hidden||s.display==='none'||s.visibility==='hidden'||s.opacity==='0')return false}return true}).map(img=>({src:img.currentSrc||img.src,width:img.naturalWidth,height:img.naturalHeight,complete:img.complete,alt:img.alt,rect:{x:img.getBoundingClientRect().x,y:img.getBoundingClientRect().y,width:img.getBoundingClientRect().width,height:img.getBoundingClientRect().height}})));
}
async function assets(p) {
  await p.evaluate(()=>document.fonts.ready);
  await p.waitForFunction(()=>Array.from(document.images).filter(img=>{const r=img.getBoundingClientRect();return !img.hidden&&r.width&&r.height&&r.bottom>0&&r.top<innerHeight&&r.right>0&&r.left<document.documentElement.clientWidth}).every(img=>img.complete),{},{timeout:18000}).catch(()=>{});
  await p.locator('img').evaluateAll(images=>Promise.all(images.filter(img=>{const r=img.getBoundingClientRect();return img.complete&&img.naturalWidth&&r.bottom>0&&r.top<innerHeight}).map(img=>img.decode().catch(()=>{}))));
  await frames(p);
  return visibleImages(p);
}
async function shot(p,name,target) {
  const images=await assets(p);const geometry=await p.evaluate(()=>({client:document.documentElement.clientWidth,scroll:document.documentElement.scrollWidth,inner:innerWidth}));
  const file=name+'.png';await p.screenshot({path:path.join(out,file),fullPage:false});
  report.captures.push({file,target,viewport:p.viewportSize(),url:p.url(),geometry,images,broken_visible_images:images.filter(i=>!i.complete||!i.width||!i.height)});
  check(name+' viewport overflow',geometry.scroll<=geometry.client+1&&geometry.inner<=geometry.client+1,geometry);
  check(name+' target image readiness',images.every(i=>i.complete&&i.width>0&&i.height>0),images.filter(i=>!i.complete||!i.width||!i.height));
  console.log('CAPTURE '+file);
}
async function popup(c,action) { const pending=c.waitForEvent('page',{timeout:15000});await action();const opened=await pending;await opened.waitForLoadState('domcontentloaded');const url=opened.url();await opened.close();return url; }
async function apiChecks(c) {
  const actual=await get(c,'/api/v1/games?limit=50');check('public games API available',actual.status===200);
  report.evidence.games={total:actual.data.total,items:actual.data.items.map(x=>({id:x.id,title:x.title,minutes:x.playtime_minutes,poster:x.poster_url,header:x.image_url,url:x.url}))};
  check('candidate game count derives from actual API',actual.data.total===actual.data.items.length&&actual.data.total>5,{total:actual.data.total,returned:actual.data.items.length});
  check('public games redact mutable/private fields',actual.data.items.every(x=>!['version','local_override','visible','created_at','updated_at','steam_key','api_key'].some(k=>k in x)));
  for(const sort of ['position','playtime','title']){
    let cursor='',ids=[],titles=[],pages=0;
    do{const r=await get(c,'/api/v1/games?limit=2&sort='+sort+(cursor?'&cursor='+encodeURIComponent(cursor):''));check(sort+' page '+(++pages)+' total/limit stable',r.status===200&&r.data.total===actual.data.total&&r.data.limit===2);ids.push(...r.data.items.map(x=>x.id));titles.push(...r.data.items.map(x=>x.title));if(!cursor&&r.data.next_cursor){const changed=await get(c,'/api/v1/games?limit=3&sort='+sort+'&cursor='+encodeURIComponent(r.data.next_cursor));check(sort+' cursor bound to limit',changed.status===400);const other=await get(c,'/api/v1/games?limit=2&sort='+(sort==='title'?'position':'title')+'&cursor='+encodeURIComponent(r.data.next_cursor));check(sort+' cursor bound to sort',other.status===400)}cursor=r.data.next_cursor;if(pages>10)throw Error('unexpected candidate game pages');}while(cursor);
    check(sort+' pagination complete and duplicate-free',ids.length===actual.data.total&&new Set(ids).size===ids.length,{ids,titles});
    if(sort==='title')check('title API sort follows C byte order',titles.every((x,i)=>!i||Buffer.compare(Buffer.from(titles[i-1]),Buffer.from(x))<=0));
  }
  check('invalid game sort rejected',(await get(c,'/api/v1/games?sort=invalid')).status===400);
  check('tampered game cursor rejected',(await get(c,'/api/v1/games?cursor=invalid')).status===400);
  return actual.data;
}
async function ownerRead(publicGames,homePage) {
  const owner=await context({storageState:'/tmp/painter-next-v16-owner-state.json'});
  const settings=await get(owner,'/api/v1/admin/home');if(settings.status!==200)throw Error('Read-only owner fixture session unavailable');
  const activity=await get(owner,'/api/v1/admin/home/activity'),posts=await get(owner,'/api/v1/admin/home/activity?source=posts'),moments=await get(owner,'/api/v1/admin/home/activity?source=moments');
  const privateGames=await all(owner,'/api/v1/admin/home/items?kind=steam');
  const ids=privateGames.filter(x=>x.visible).map(x=>x.id).sort();
  check('public game total excludes hidden owner rows',ids.length===publicGames.total&&JSON.stringify(ids)===JSON.stringify(publicGames.items.map(x=>x.id).sort()),{public:publicGames.total,owner_rows:privateGames.length,hidden:privateGames.filter(x=>!x.visible).length});
  const projects=await get(owner,'/api/v1/admin/home/projects');
  const publicPosts=await all(owner,'/api/v1/posts'),publicMoments=await all(owner,'/api/v1/moments');
  const dom=await homePage.locator('.home-activity').evaluate(e=>({summary:e.querySelector('.activity-summary').innerText,source:e.querySelector('.activity-foot').innerText,days:Array.from(e.querySelectorAll('[data-date]')).map(x=>({date:x.dataset.date,count:Number(x.dataset.count),level:Number(x.dataset.level)}))}));
  report.evidence.activity={...activity.data,days:undefined,weeks:undefined,public_posts:publicPosts.length,public_moments:publicMoments.length,post_total:posts.data.total,moment_total:moments.data.total,rendered_days:dom.days.length,rendered_sum:dom.days.reduce((n,x)=>n+x.count,0)};
  report.evidence.config={order:settings.data.config.order,steam:settings.data.config.sections.steam,activity:settings.data.config.activity,projects:settings.data.config.sections.projects};
  report.evidence.projects=projects.data.items.filter(x=>x.visible).map(x=>({id:x.id,title:x.title,url:x.url,full_name:x.full_name,language:x.language,stars:x.stars,source:x.source}));
  check('rolling calendar contains exact365 unique consecutive days',dom.days.length===365&&new Set(dom.days.map(x=>x.date)).size===365&&dom.days.every((x,i)=>!i||Date.parse(x.date)-Date.parse(dom.days[i-1].date)===86400000));
  check('calendar rendered counts match saved-source daily API',JSON.stringify(dom.days.map(x=>({date:x.date,count:x.count,level:x.level})))===JSON.stringify(activity.data.days.map(x=>({date:x.date,count:x.count,level:x.level}))));
  check('calendar total/active-days equal rendered cells',dom.days.reduce((n,x)=>n+x.count,0)===activity.data.total&&dom.days.filter(x=>x.count>0).length===activity.data.active_days&&dom.summary.includes(activity.data.total+' 次发布'));
  check('current public sources reconcile with calendar total',activity.data.total===posts.data.total+moments.data.total&&posts.data.total===publicPosts.length&&moments.data.total===publicMoments.length,{all:activity.data.total,posts:posts.data.total,moments:moments.data.total,publicPosts:publicPosts.length,publicMoments:publicMoments.length});
  check('calendar basis and timezone are honest',dom.source.includes(activity.data.source_label)&&dom.source.includes(activity.data.timezone)&&dom.source.includes('按首次发布时间计数')&&dom.source.includes('撤回或删除后不计入'));
  const rendered=await homePage.locator('.project-card-link').evaluateAll(es=>es.map(e=>({title:e.querySelector('h3').textContent,href:e.href,target:e.target,rel:e.rel,text:e.innerText})));
  check('three configured real GitHub project destinations',rendered.length===3&&rendered.every(x=>report.evidence.projects.some(y=>y.url===x.href))&&rendered.map(x=>x.href).every(x=>/^https:\/\/github\.com\/callme-taota\/(Painter|Tracker|tolog)$/.test(x)),rendered);
  check('project unknown stars remain absent or real saved counts',rendered.every(x=>{const saved=report.evidence.projects.find(y=>y.url===x.href);return saved.stars===null||x.text.includes(String(saved.stars))}));
  await owner.close();
}
async function gamesChecks(c,p,width) {
  await p.goto(base+'/games');await ready(p);
  const g=await p.locator('.games-library-grid').evaluate(e=>({columns:getComputedStyle(e).gridTemplateColumns.split(' ').length,cards:Array.from(e.querySelectorAll('.game-library-card')).slice(0,3).map(c=>{const a=c.querySelector('.game-library-art').getBoundingClientRect(),h=c.querySelector('h2').getBoundingClientRect(),p=c.querySelector('.game-library-details p').getBoundingClientRect();return {width:a.width,height:a.height,title_height:h.height,hours_top:p.top,title:c.querySelector('h2').textContent,hours:c.querySelector('.game-library-details p').textContent}})}));
  const expected=width===1440?3:width===900?2:1;
  check(width+' game grid '+expected+' columns',g.columns===expected,g);
  check(width+' game art uses large2:3 portrait area',g.cards.every(x=>Math.abs(x.width/x.height-2/3)<.002&&x.width>200));
  if(width===1440)check('desktop game titles and hours share baselines',g.cards.every(x=>Math.abs(x.title_height-g.cards[0].title_height)<1&&Math.abs(x.hours_top-g.cards[0].hours_top)<1),g.cards);
  await noOverflow(p,width+' games no document overflow');
  const links=await p.locator('.game-library-open').evaluateAll(es=>es.map(e=>({href:e.href,target:e.target,rel:e.rel})));
  check(width+' game destinations native external links',links.every(x=>/^https:\/\/store\.steampowered\.com\/app\/\d+\//.test(x.href)&&x.target==='_blank'&&x.rel.includes('noopener')));
  if(width===1440){
    await p.getByRole('link',{name:'游玩时长',exact:true}).click();check('game sort normal-link navigation and current state',new URL(p.url()).searchParams.get('sort')==='playtime'&&await p.getByRole('link',{name:'游玩时长',exact:true}).getAttribute('aria-current')==='page');
    const sorted=(await get(c,'/api/v1/games?sort=playtime&limit=50')).data.items;check('SSR hours/title order matches API',JSON.stringify(await p.locator('.game-library-details h2').allTextContents())===JSON.stringify(sorted.map(x=>x.title)));
    await p.goto(base+'/games?limit=2');const before=await p.locator('.game-library-details h2').allTextContents();await p.getByRole('navigation',{name:'分页',exact:true}).getByRole('link').click();check('game next-page native navigation',new URL(p.url()).searchParams.has('cursor')&&JSON.stringify(before)!==JSON.stringify(await p.locator('.game-library-details h2').allTextContents()));
  }
}
async function homeChecks(c,p,mobile=false) {
  await p.goto(base+'/');await ready(p);await p.locator('[data-project-card]').first().waitFor();
  check((mobile?'mobile':'desktop')+' home actual five games and24 music',await p.locator('[data-game-frame]').count()===5&&await p.locator('.home-record-tile').count()===24);
  await scroll(p,'#home-activity',80);const days=p.locator('[data-activity-calendar] [data-date]');
  check((mobile?'mobile':'desktop')+' activity roving one tab stop',await days.evaluateAll(es=>es.filter(e=>e.tabIndex===0).length===1));
  await days.last().focus();await p.keyboard.press('Home');check('calendar Home reaches first day',await days.first().evaluate(e=>e===document.activeElement));await p.keyboard.press('ArrowRight');check('calendar ArrowRight moves one week',await days.nth(7).evaluate(e=>e===document.activeElement));await p.keyboard.press('ArrowDown');check('calendar ArrowDown moves one day',await days.nth(8).evaluate(e=>e===document.activeElement));await p.keyboard.press('End');
  const end=await days.last().getAttribute('data-date');check('calendar End/live date detail accurate',await days.last().evaluate(e=>e===document.activeElement)&&await p.locator('[data-activity-detail]').innerText().then(t=>t.includes(end))&&await p.locator('[data-activity-detail]').getAttribute('aria-live')==='polite');
  const nonzero=days.locator('[data-count]:not([data-count="0"])');
  if(!mobile){const day=p.locator('[data-activity-calendar] [data-count]:not([data-count="0"])').first();await day.hover();const date=await day.getAttribute('data-date'),count=await day.getAttribute('data-count');check('calendar pointer detail matches real cell',await p.locator('[data-activity-detail]').innerText()===date+' · '+count+' 次发布');}
  check((mobile?'mobile':'desktop')+' activity horizontal overflow contained',await p.locator('.activity-scroll').evaluate(e=>{const r=e.getBoundingClientRect();return r.left>=0&&r.right<=document.documentElement.clientWidth+1&&getComputedStyle(e).overflowX==='auto'}));
  await scroll(p,'#home-projects',70);await p.mouse.move(1,1);await p.locator('h2').first().focus().catch(()=>{});
  const before=await p.locator('[data-project-card]').evaluateAll(es=>es.map(e=>{const r=e.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top}}));
  check((mobile?'mobile':'desktop')+' project cards overlap at rest',before.length===3&&before[1].left<before[0].right);
  const second=p.locator('.project-card-link').nth(1);await second.focus();await p.waitForTimeout(380);
  const active=await p.locator('[data-project-card]').evaluateAll(es=>es.map(e=>({active:e.classList.contains('is-active'),dodge:e.style.getPropertyValue('--dodge-x'),lift:getComputedStyle(e).getPropertyValue('--card-lift')})));
  check((mobile?'mobile':'desktop')+' focused project lifts and dodges neighbors',active.filter(x=>x.active).length===1&&active[1].active&&active[0].dodge==='-18px'&&active[2].dodge==='26px'&&parseFloat(active[1].lift)<0,active);
  check((mobile?'mobile':'desktop')+' project focused link remains visible',await second.evaluate(e=>{const r=e.getBoundingClientRect();return r.right>0&&r.left<document.documentElement.clientWidth&&getComputedStyle(e).outlineStyle!=='none'}));
  const destination=await popup(c,()=>mobile?second.tap():p.keyboard.press('Enter'));check((mobile?'touch':'keyboard')+' project retains original native destination',destination===await second.getAttribute('href'));
  await noOverflow(p,(mobile?'mobile':'desktop')+' home no page overflow');
}
async function articleChecks(c,p,mobile=false) {
  await p.goto(fixture.article_url);await ready(p);
  const cards=await p.locator('.article-reading .prose a.content-card').evaluateAll(es=>es.map(e=>({class:e.className,href:e.href,target:e.target,rel:e.rel})));
  check((mobile?'mobile':'desktop')+' article three canonical provider cards',cards.length===3&&cards[0].href==='https://github.com/callme-taota/Painter'&&cards[1].href==='https://store.steampowered.com/app/413150/'&&cards[2].href.startsWith('https://music.apple.com/us/album/')&&cards.every(x=>x.target==='_blank'&&x.rel.includes('noopener')),cards);
  check('article provider images excluded from gallery',await p.locator('.content-card a[data-article-image]').count()===0&&await p.locator('a[data-article-image]').count()===1&&await p.locator('.moment-lightbox').count()===1);
  const anchor=p.locator('.content-link__anchor').first(),preview=p.locator('.content-link__preview').first();await scroll(p,'.content-link',300);
  if(mobile){let count=0;c.on('page',()=>count++);await anchor.tap();await frames(p);check('real article first touch inspects without external navigation',count===0&&await preview.getAttribute('aria-hidden')==='false');const dest=await popup(c,()=>anchor.tap());check('real article second touch follows canonical Cali URL',dest==='https://cali.so/');}
  else{await anchor.hover();await frames(p);check('real article hover exposes stored Cali description',await preview.getAttribute('aria-hidden')==='false'&&await preview.innerText().then(t=>t.includes('Cali Castle')&&t.includes('设计工程师')));}
  await anchor.focus();await p.keyboard.press('Escape');check((mobile?'mobile':'desktop')+' inline Escape dismisses and retains link focus',await preview.getAttribute('aria-hidden')==='true'&&await anchor.evaluate(e=>document.activeElement===e));
  await p.locator('a[data-article-image]').click();await p.waitForFunction(()=>document.querySelector('.moment-lightbox[open] img')?.naturalWidth>0);check((mobile?'mobile':'desktop')+' actual uploaded article image uses shared viewer',await p.locator('.moment-lightbox[open]').count()===1&&await p.locator('#moment-lightbox-title').innerText()==='文章图片'&&await p.locator('.moment-lightbox-actions a').getAttribute('href').then(x=>x.includes(fixture.media_id)));await p.keyboard.press('Escape');check('article image viewer returns focus',await p.locator('a[data-article-image]').evaluate(e=>e===document.activeElement));
  await noOverflow(p,(mobile?'mobile':'desktop')+' article no document overflow');
}
async function fallbackChecks() {
  const c=await context(),p=await page(c,'poster-fallback');await p.goto(base+'/games');const first=p.locator('[data-library-cover]').first(),poster=await first.getAttribute('src'),saved=await first.getAttribute('data-fallback');await c.route(poster,r=>r.abort('failed'));await p.reload();await p.locator('[data-library-cover]').first().scrollIntoViewIfNeeded();await p.waitForFunction(expected=>{const img=document.querySelector('[data-library-cover]');return img.src===expected&&img.complete&&img.naturalWidth>0},saved);check('broken portrait falls back to actual saved header image',await p.locator('[data-library-cover]').first().getAttribute('src')===saved,{poster,saved});await c.close();
  const r=await context({viewport:{width:390,height:900},reducedMotion:'reduce'}),rp=await page(r,'reduced-motion');await rp.goto(base+'/');await ready(rp);check('reduced motion uses native five scenes and no home enhancement',await rp.locator('[data-game-frame]').count()===5&&await rp.locator('[data-game-story].is-enhanced').count()===0);check('reduced motion project cards have no transform/transition',await rp.locator('[data-project-card]').evaluateAll(es=>es.every(e=>getComputedStyle(e).transform==='none'&&getComputedStyle(e).transitionDuration==='0s')));await noOverflow(rp,'reduced-motion home no overflow');await r.close();
  const n=await context({viewport:{width:390,height:900},javaScriptEnabled:false}),np=await page(n,'noJS');await np.goto(base+'/');check('noJS projects/day labels/native external links remain available',await np.locator('.project-card-link').count()===3&&await np.locator('[data-activity-calendar] [data-date][aria-label]').count()===365&&await np.locator('.project-card-link').first().getAttribute('target')==='_blank');await noOverflow(np,'noJS home no overflow');await np.goto(base+'/games');check('noJS game posters and sort links remain native',await np.locator('[data-library-cover]').count()===report.evidence.games.total&&await np.locator('.games-sort a').count()===3);await np.goto(fixture.article_url);check('noJS article provider and inline destinations preserved',await np.locator('.content-card').count()===3&&await np.locator('.content-link__anchor').getAttribute('href')==='https://cali.so/'&&await np.locator('.moment-lightbox').count()===0);await noOverflow(np,'noJS article no overflow');await n.close();
}
async function captures(desktop,mobile) {
  const [d,m]=[desktop.page,mobile.page];
  await d.goto(base+'/games');await scroll(d,'.games-library',72);await shot(d,'1440-light-games','Three-column library portrait art and aligned titles/hours');
  await m.goto(base+'/games');await scroll(m,'.games-library',64);await shot(m,'390-dark-games','Single-column library portrait card');
  await d.goto(base+'/');await ready(d);await scroll(d,'#home-activity',100);await shot(d,'1440-light-activity','365-day actual publication heatmap');
  await m.goto(base+'/');await ready(m);await scroll(m,'#home-activity',80);await shot(m,'390-dark-activity','Scrollable daily calendar and source/timezone');
  await scroll(d,'#home-projects',90);await d.mouse.move(1,1);await d.locator('.home-projects h2').click();await d.waitForTimeout(380);await shot(d,'1440-light-projects-rest','Three real repository cards overlapping at rest');
  await d.locator('.project-card-link').nth(1).hover();await d.waitForTimeout(380);await shot(d,'1440-light-projects-active','Hovered Tracker card with neighboring dodge');
  await scroll(m,'#home-projects',80);await m.locator('.project-card-link').first().focus();await m.waitForTimeout(380);await shot(m,'390-dark-projects-active','Focused first project and contained horizontal deck');
  await d.goto(fixture.article_url);await ready(d);await scroll(d,'.content-card--github',90);await shot(d,'1440-light-article-cards','Actual GitHub and Steam cards in article flow');
  await scroll(d,'.content-card--apple_music',100);await shot(d,'1440-light-article-music-image','Apple Music card and actual uploaded article image');
  await m.goto(fixture.article_url);await ready(m);await scroll(m,'.content-card--github',90);await shot(m,'390-dark-article-cards','Provider cards in narrow article flow');
  await scroll(m,'.content-card--apple_music',100);await shot(m,'390-dark-article-music-image','Narrow Apple Music card and uploaded photo');
  await scroll(d,'.content-link',300);await d.locator('.content-link__anchor').hover();await frames(d);await shot(d,'1440-light-inline-preview','Canonical Cali inline metadata tooltip');
  await scroll(m,'.content-link',300);await m.locator('.content-link__anchor').tap();await frames(m);await shot(m,'390-dark-inline-preview','First-touch Cali preview with second-tap hint');await m.keyboard.press('Escape');
  await m.locator('a[data-article-image]').tap();await m.waitForFunction(()=>document.querySelector('.moment-lightbox[open] img')?.naturalWidth>0);await shot(m,'390-dark-article-lightbox','Single shared viewer with actual local uploaded image');await m.keyboard.press('Escape');
}
(async()=>{
  if(new URL(base).hostname!=='192.168.2.209'||new URL(base).port!=='18081'||fixture.article_url!==base+'/posts/33')throw Error('Candidate fixture guard');
  fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
  if(process.argv.includes('--calendar-fix-only')){
    report.phase='targeted-public-calendar-fix';report.evidence.initial=[];report.evidence.navigation=[];
    if(process.argv.includes('--skip-capture'))report.captures=JSON.parse(fs.readFileSync(path.join(out,'calendar-fix-public.json'),'utf8')).captures;
    for(const [width,theme]of[[1440,'light'],[390,'dark']]){
      const c=await context({viewport:{width,height:width===1440?1000:900},colorScheme:theme,...(width===390?{isMobile:true,hasTouch:true}:{})}),p=await page(c,width+'-calendar-fix');await p.goto(base+'/');await ready(p);await p.waitForFunction(()=>document.querySelector('[data-activity-calendar] [tabindex="0"]'));
      // This scrolls the page to the section only; the calendar's own horizontal
      // position is entirely the application's initial selection behavior.
      await scroll(p,'#home-activity',width===1440?100:80);
      const initial=await p.locator('#home-activity').evaluate((section,width)=>{const scroller=section.querySelector('.activity-scroll'),box=scroller.getBoundingClientRect(),days=Array.from(section.querySelectorAll('[data-date]')),live=days.filter(e=>!e.classList.contains('is-future')),latest=live.at(-1),selected=days.filter(e=>e.tabIndex===0),visible=e=>{const r=e.getBoundingClientRect();return r.left>=box.left-1&&r.right<=box.right+1&&r.top>=0&&r.bottom<=innerHeight};return {viewport:width,scroll_left:scroller.scrollLeft,scroll_width:scroller.scrollWidth,client_width:scroller.clientWidth,scroller:{left:box.left,right:box.right},latest_date:latest.dataset.date,latest_visible:visible(latest),selected:selected.map(e=>e.dataset.date),days:days.length,total:days.reduce((n,e)=>n+Number(e.dataset.count),0),green:days.filter(e=>Number(e.dataset.count)>0).map(e=>({date:e.dataset.date,count:Number(e.dataset.count),visible:visible(e)})),cue:section.querySelector('[data-activity-detail]').innerText}},width);
      report.evidence.initial.push(initial);check(width+' calendar latest nonfuture date visible before user scroll',initial.latest_visible&&initial.selected.length===1&&initial.selected[0]===initial.latest_date,initial);check(width+' both actual populated dates visible immediately',initial.green.length===2&&initial.green.every(d=>d.visible)&&initial.total===13,initial.green);if(width===390)check('390 initial calendar scrolled to recent end',initial.scroll_left>0,initial);
      check(width+' visible sideways-scroll instruction',await p.locator('[data-activity-detail]').isVisible()&&initial.cue.includes('左右滑动'));
      await noOverflow(p,width+' initial calendar no document overflow');
      const name=width+'-'+theme+'-activity',original=path.join(out,name+'.png'),before=path.join(out,name+'.before-calendar-fix.png');if(!process.argv.includes('--skip-capture')){if(!fs.existsSync(before))fs.copyFileSync(original,before);await shot(p,name,'Calendar initial recent dates and explicit sideways-scroll cue after targeted fix');}
      const days=p.locator('[data-activity-calendar] [data-date]'),scroller=p.locator('.activity-scroll');
      if(width===390){await scroller.hover();await p.mouse.wheel(-1200,0);await p.waitForFunction(()=>document.querySelector('.activity-scroll').scrollLeft<10);check('390 native horizontal wheel exposes older dates',await scroller.evaluate(e=>e.scrollLeft<10));}
      // Park the synthetic mouse before exercising keyboard behavior on the
      // touch-emulated viewport, so a scrolling cell cannot enter its hover.
      await p.mouse.move(1,1);await frames(p);
      await days.last().focus();await p.keyboard.press('Home');const firstDate=await days.first().getAttribute('data-date');check(width+' keyboard Home reaches old first date',await days.first().evaluate(e=>e===document.activeElement)&&await scroller.evaluate(e=>e.scrollLeft<10));
      await p.keyboard.press('End');check(width+' keyboard End restores latest date',await days.last().evaluate(e=>e===document.activeElement)&&await p.locator('[data-activity-detail]').innerText().then(t=>t.includes(initial.latest_date)&&t.includes('左右滑动')));
      await p.keyboard.press('ArrowLeft');const older=await days.nth(initial.days-8).getAttribute('data-date');check(width+' keyboard older-week navigation works',await days.nth(initial.days-8).evaluate(e=>e===document.activeElement)&&Date.parse(initial.latest_date)-Date.parse(older)===7*86400000);
      const activeDay=p.locator('[data-activity-calendar] [data-date][tabindex="0"]');await activeDay.hover();check(width+' hover detail retains sideways-scroll instruction',await p.locator('[data-activity-detail]').innerText().then(t=>t.includes(older)&&t.includes('左右滑动')));
      report.evidence.navigation.push({viewport:width,first_date:firstDate,older_week_date:older,latest_date:initial.latest_date,final_scroll_left:await scroller.evaluate(e=>e.scrollLeft),detail:await p.locator('[data-activity-detail]').innerText()});await noOverflow(p,width+' calendar navigation no document overflow');await c.close();
    }
    check('calendar correction no runtime errors',report.errors.length===0,report.errors);check('calendar correction no business writes',report.blocked_writes.length===0,report.blocked_writes);report.passed=report.checks.every(x=>x.passed);return;
  }
  if(process.argv.includes('--steam-fix-only')){
    const probePath=path.join(out,'steam-mobile-layout-probe.json'),probe=JSON.parse(fs.readFileSync(probePath,'utf8'));probe.after=[];
    for(const width of [390,320]){
      const c=await context({viewport:{width,height:900},colorScheme:'dark',isMobile:true,hasTouch:true}),p=await page(c,width+'-steam-fix');await p.goto(fixture.article_url);await ready(p);
      const geometry=await p.locator('.content-card--steam').evaluate((e,width)=>{const img=e.querySelector('img').getBoundingClientRect(),body=e.querySelector('.content-card__body').getBoundingClientRect();return {viewport:width,columns:getComputedStyle(e).gridTemplateColumns,card_width:e.getBoundingClientRect().width,body_width:body.width,description_height:e.querySelector('.content-card__description').getBoundingClientRect().height,image_bottom:img.bottom,body_top:body.top,image_width:img.width}},width);
      probe.after.push(geometry);check(width+' fixed Steam card has one column',geometry.columns.split(' ').length===1,geometry);check(width+' Steam art above full-width body',geometry.body_width>=geometry.card_width-3&&geometry.image_bottom<=geometry.body_top+1,geometry);check(width+' Steam description no longer narrow strip',geometry.description_height<350,geometry);await noOverflow(p,width+' fixed article no overflow');
      if(width===390){for(const name of ['390-dark-article-cards','390-dark-inline-preview']){const original=path.join(out,name+'.png'),before=path.join(out,name+'.before-steam-fix.png');if(!fs.existsSync(before))fs.copyFileSync(original,before)}await scroll(p,'.content-card--github',90);await shot(p,'390-dark-article-cards','Post-fix mobile GitHub/stacked Steam cards');await scroll(p,'.content-link',300);await p.locator('.content-link__anchor').tap();await frames(p);await shot(p,'390-dark-inline-preview','Post-fix first-touch Cali preview and stacked Steam card below');}
      await c.close();
    }
    fs.writeFileSync(probePath,JSON.stringify(probe,null,2));check('targeted fix no runtime errors',report.errors.length===0,report.errors);check('targeted fix no business writes',report.blocked_writes.length===0,report.blocked_writes);report.passed=report.checks.every(x=>x.passed);report.phase='targeted-steam-fix';return;
  }
  if(process.argv.includes('--capture-only')){
    const previous=JSON.parse(fs.readFileSync(path.join(out,'functional-report.json'),'utf8'));
    if(!previous.passed||previous.origin!==base||previous.captures.length)throw Error('Capture requires passing read-only functional report');
    Object.assign(report,previous,{captures:[],passed:undefined});
    const dc=await context(),dp=await page(dc,'desktop-capture'),mc=await context({viewport:{width:390,height:900},colorScheme:'dark',isMobile:true,hasTouch:true}),mp=await page(mc,'mobile-capture');
    await captures({context:dc,page:dp},{context:mc,page:mp});await dc.close();await mc.close();
    check('no uncaught runtime errors during capture',report.errors.length===0,report.errors);
    check('capture attempted no business writes',report.blocked_writes.length===0,report.blocked_writes);
    report.passed=report.checks.every(x=>x.passed);return;
  }
  const dc=await context(),dp=await page(dc,'desktop'),mc=await context({viewport:{width:390,height:900},colorScheme:'dark',isMobile:true,hasTouch:true}),mp=await page(mc,'mobile');
  const games=await apiChecks(dc);await dp.goto(base+'/');await ready(dp);await ownerRead(games,dp);
  await gamesChecks(dc,dp,1440);await gamesChecks(mc,mp,390);
  const tc=await context({viewport:{width:900,height:1000}}),tp=await page(tc,'tablet');await gamesChecks(tc,tp,900);await tc.close();
  await homeChecks(dc,dp);await homeChecks(mc,mp,true);await articleChecks(dc,dp);await articleChecks(mc,mp,true);
  const nc=await context({viewport:{width:320,height:900}}),np=await page(nc,'320-overflow');for(const [url,label]of[['/','home'],['/games','games'],['/posts/33','article']]){await np.goto(base+url);await ready(np);await noOverflow(np,'320 '+label+' no page overflow');if(label==='home'){await scroll(np,'#home-projects',80);await noOverflow(np,'320 project deck overflow contained');await scroll(np,'#home-activity',80);await noOverflow(np,'320 activity overflow contained')}}await nc.close();
  await fallbackChecks();
  report.limitations.push('Read-only populated candidate: disabled sections, limit-zero full-library policy, and zero-item home templates were not changed to force runtime states; backend/source coverage remains separate.','Touch is Chrome emulation; physical devices and Safari are not covered.','External document navigations are fulfilled locally after observing the actual destination URL; third-party destination apps are not exercised.');
  console.log('FUNCTIONAL COMPLETE '+report.checks.length+' checks; '+report.checks.filter(x=>!x.passed).length+' failures');
  if(!process.argv.includes('--functional-only'))await captures({context:dc,page:dp},{context:mc,page:mp});
  check('no uncaught public runtime errors',report.errors.length===0,report.errors);check('no business write attempted beyond analytics',report.blocked_writes.length===0,report.blocked_writes);
  await dc.close();await mc.close();report.passed=report.checks.every(x=>x.passed);
})().catch(e=>{report.failure=String(e);report.passed=false;console.error(e)}).finally(async()=>{await browser?.close();fs.mkdirSync(out,{recursive:true});const filename=process.argv.includes('--calendar-fix-only')?'calendar-fix-public.json':process.argv.includes('--steam-fix-only')?'steam-fix-report.json':'report.json';fs.writeFileSync(path.join(out,filename),JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,captures:report.captures.length,errors:report.errors.length,failures:report.checks.filter(x=>!x.passed).map(x=>x.name),failure:report.failure}));if(!report.passed)process.exitCode=1});
