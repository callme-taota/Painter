// Run only after binary promotion: PAINTER_V28_PROMOTED=1 node this-file.cjs.
// Formal owner data is read only. Browser writes and analytics are intercepted.
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const {spawnSync} = require('child_process');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || '/tmp/painter-browser/node_modules/playwright');
const base = process.env.PAINTER_V28_FORMAL_BASE || 'http://192.168.2.209:18080';
const out = path.resolve(process.env.ARTIFACT_DIR || 'tests/artifacts/v28/formal');
const ownerState = process.env.PAINTER_OWNER_STATE || '/tmp/painter-next-v16-formal-state.json';
const report = {origin:base, started_at:new Date().toISOString(), read_only:true, checks:[], errors:[], console_errors:[], failed_requests:[], blocked_writes:[], analytics_blocked:0, assets:[], evidence:{}, captures:[], clips:[], formal_business_writes:0};
let browser;
const sha = data => crypto.createHash('sha256').update(data).digest('hex');
const cleanURL = raw => {const u=new URL(raw);return u.origin+u.pathname};
const near = (a,b,t=2) => Number.isFinite(a)&&Number.isFinite(b)&&Math.abs(a-b)<=t;
const sameRect = (a,b,t=2) => a&&b&&['left','top','width','height'].every(k=>near(a[k],b[k],t));
const save = () => {fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2)+'\n')};
function check(name,ok,detail){report.checks.push({name,passed:!!ok,...(detail===undefined?{}:{detail})});save();if(!ok)throw Error(name);console.log('PASS '+name)}
const sourceNames = ['public.ts','homeStory.ts','homeStory.css','homeWorkJourney.ts','homeWorkJourney.css','homePointMorph.ts','homePointMorph.css','homeShowcase.ts','homeShowcase.css','homeControlGlass.ts','homeControlGlass.css','momentGallery.ts','momentGallery.css'];
async function settle(p){await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));await p.waitForTimeout(65);await p.evaluate(()=>new Promise(r=>requestAnimationFrame(r)))}
async function protect(c){
  await c.route('**/*',route=>{
    const request=route.request(),u=new URL(request.url());
    if(u.pathname.includes('/analytics/')){report.analytics_blocked++;return route.fulfill({status:200,json:{accepted:0}})}
    if(!['GET','HEAD'].includes(request.method())){report.blocked_writes.push({method:request.method(),url:cleanURL(request.url())});return route.fulfill({status:204,body:''})}
    return route.continue();
  });
}
async function unit(p,u){
  await p.locator('[data-home-work-journey]').evaluate((e,value)=>scrollTo({top:scrollY+e.getBoundingClientRect().top+value*innerHeight,behavior:'instant'}),u);
  await settle(p);return snapshot(p);
}
async function gameUnit(p,u){
  await p.locator('[data-game-story]').evaluate((e,value)=>scrollTo({top:scrollY+e.getBoundingClientRect().top+value*document.querySelector('[data-game-stage]').clientHeight,behavior:'instant'}),u);
  await settle(p);return snapshot(p);
}
async function snapshot(p){return p.evaluate(()=>{
  const rect=e=>{if(!e)return null;const r=e.getBoundingClientRect();return{left:r.left,top:r.top,width:r.width,height:r.height}};
  const painted=e=>{
    if(!e)return null;let alpha=1,hidden=false;for(let n=e;n;n=n.parentElement){const s=getComputedStyle(n);alpha*=Number(s.opacity);hidden||=n.hidden||s.display==='none'||s.visibility==='hidden'}
    const s=getComputedStyle(e);return{rect:rect(e),alpha,hidden,inert:e.inert,background:s.backgroundColor,shadow:s.boxShadow,transform:s.transform};
  };
  const work=document.querySelector('[data-home-work-journey]'),stage=document.querySelector('.home-work-stage'),recordStage=document.querySelector('.home-record-stage'),aperture=document.querySelector('[data-game-window]'),windowRect=rect(aperture);
  const inset=side=>parseFloat(aperture?.style.getPropertyValue('--game-mask-'+side))||0;
  const mask=windowRect?{left:windowRect.left+windowRect.width*inset('left')/100,top:windowRect.top+windowRect.height*inset('top')/100,width:windowRect.width*(1-(inset('left')+inset('right'))/100),height:windowRect.height*(1-(inset('top')+inset('bottom'))/100)}:null;
  return{width:innerWidth,height:innerHeight,theme:document.documentElement.dataset.theme,overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth+1,units:Number(work?.dataset.workUnits),phase:work?.dataset.workPhase,stage:painted(stage),carrier:painted(document.querySelector('[data-work-carrier]')),cards:[...document.querySelectorAll('.project-card-link')].map(e=>({...painted(e),date:e.dataset.workTargetDate})),days:[...document.querySelectorAll('[data-activity-calendar] [data-date]')].map(e=>({date:e.dataset.date,count:Number(e.dataset.count),level:Number(e.dataset.level),...painted(e)})),covers:[...document.querySelectorAll('.point-record-surface')].map(e=>({date:e.dataset.pointSourceDate,index:Number(e.dataset.pointRecordIndex),src:e.querySelector('img')?.currentSrc||e.querySelector('img')?.src,imageAlpha:Number(getComputedStyle(e.querySelector('img')).opacity),imageReady:!!e.querySelector('img')?.complete&&e.querySelector('img')?.naturalWidth>0,...painted(e)})),recordStage:painted(recordStage),records:[...document.querySelectorAll('[data-record-wall] .home-record-tile')].map(e=>({href:e.href,target:e.target,rel:e.rel,src:e.querySelector('img')?.currentSrc||e.querySelector('img')?.src,...painted(e)})),game:{enhanced:document.querySelector('[data-game-story]')?.classList.contains('is-enhanced'),phase:document.querySelector('[data-game-story]')?.dataset.gamePhase,active:Number(document.querySelector('[data-game-story]')?.dataset.gameActive),window:windowRect,mask,origin:rect(document.querySelector('[data-record-game-origin]')),alpha:aperture?Number(getComputedStyle(aperture).opacity):0,inert:aperture?.inert,ui:parseFloat(aperture?.style.getPropertyValue('--game-ui-opacity'))||0,frames:[...document.querySelectorAll('[data-game-frame]')].map(e=>({id:e.dataset.gameId,current:e.classList.contains('is-current'),inert:e.inert,href:e.querySelector('.game-store-link')?.href}))},glass:Number(document.querySelector('.floating-controls')?.dataset.mediaGlassAmount)};
})}
async function ready(p){
  await p.evaluate(()=>document.fonts.ready);
  await p.waitForFunction(()=>[...document.querySelectorAll('.point-record-surface>img,.project-owner-avatar,.work-card-intro-face img')].every(img=>img.complete&&img.naturalWidth>0),null,{timeout:30000});
  const result=await p.evaluate(async()=>{
    const images=[...document.querySelectorAll('.point-record-surface>img,.project-owner-avatar,.work-card-intro-face img')];
    await Promise.all(images.map(img=>img.decode()));
    const fonts=[...new Set([...document.querySelectorAll('.home-work-stage h1,.home-work-stage h2,.home-work-stage h3,.home-work-stage p')].map(e=>{const s=getComputedStyle(e);return `${s.fontStyle} ${s.fontWeight} ${s.fontSize} ${s.fontFamily}`}))];
    return{font_status:document.fonts.status,fonts:fonts.map(font=>({font,ready:document.fonts.check(font)})),decoded_images:images.map(img=>({src:img.currentSrc||img.src,width:img.naturalWidth,height:img.naturalHeight}))};
  });
  check(p.viewportSize().width+' capture assets are decoded and fonts loaded',result.font_status==='loaded'&&result.fonts.every(f=>f.ready)&&result.decoded_images.every(i=>i.width>0&&i.height>0),result);return result;
}
async function savedData(c){
  const routes=['/api/v1/session','/api/v1/admin/home','/api/v1/admin/home/activity','/api/v1/admin/home/projects','/api/v1/admin/home/activity?source=posts','/api/v1/admin/home/activity?source=moments','/api/v1/admin/home/items?kind=music&limit=50','/api/v1/games?limit=50'];
  const responses=await Promise.all(routes.map(async route=>{const response=await c.request.get(base+route);return{route,status:response.status(),data:await response.json()}}));
  check('formal owner read APIs are available',responses.every(r=>r.status===200)&&responses[0].data.user?.role==='owner',responses.map(({route,status})=>({route,status})));
  const [,home,activity,projects,posts,moments,music,games]=responses;
  report.evidence.saved={order:home.data.config.order,activity:activity.data,projects:projects.data.items.filter(i=>i.visible),records:music.data.items.filter(i=>i.visible),game_count:games.data.total};
  check('formal saved sequence and publication totals remain truthful',JSON.stringify(home.data.config.order.slice(0,5))===JSON.stringify(['intro','projects','activity','music','steam'])&&activity.data.total===posts.data.total+moments.data.total,{order:home.data.config.order,total:activity.data.total,posts:posts.data.total,moments:moments.data.total});
  return{home:home.data,activity:activity.data,projects:projects.data.items.filter(i=>i.visible),music:music.data.items,games:games.data};
}
async function assetChecks(c){
  const manifest=JSON.parse(fs.readFileSync('web/assets/.vite/manifest.json','utf8'));
  const names=new Set(['src/public.ts','src/homeStory.ts','src/homeWorkJourney.ts','src/homeShowcase.ts','src/homeControlGlass.ts']);
  const files=[...new Set(Object.values(manifest).filter(v=>names.has(v.src)).flatMap(v=>[v.file,...v.css||[]]))];
  for(const file of files){const response=await c.request.get(base+'/assets/'+file);report.assets.push({file,status:response.status(),sha256:sha(await response.body())})}
  const readyResponse=await c.request.get(base+'/readyz');
  check('formal homepage assets return 200 and readiness is healthy',files.length>0&&report.assets.every(a=>a.status===200)&&readyResponse.status()===200&&(await readyResponse.json()).status==='ready',report.assets);
}
async function verify(p,label,saved){
  const carrier=await p.locator('[data-work-carrier]').elementHandle();
  const dom=await p.evaluate(()=>({projects:[...document.querySelectorAll('.project-card-link')].map(e=>({href:e.href,full_name:e.querySelector('.project-full-name')?.textContent,target:e.target,rel:e.rel})),summary:document.querySelector('.activity-summary')?.textContent,source:document.querySelector('.activity-foot')?.textContent,days:[...document.querySelectorAll('[data-activity-calendar] [data-date]')].map(e=>({date:e.dataset.date,count:Number(e.dataset.count),level:Number(e.dataset.level)}))}));
  check(label+' three actual owner repository links match saved records',dom.projects.length===3&&dom.projects.every(project=>project.href.startsWith('https://github.com/callme-taota/')&&saved.projects.some(s=>s.url===project.href&&s.full_name===project.full_name)&&project.target==='_blank'&&project.rel.includes('noopener')),dom.projects);
  check(label+' all 365 dates, counts and levels match the real publication API',dom.days.length===365&&new Set(dom.days.map(d=>d.date)).size===365&&JSON.stringify(dom.days)===JSON.stringify(saved.activity.days.map(d=>({date:d.date,count:d.count,level:d.level})))&&dom.days.reduce((n,d)=>n+d.count,0)===saved.activity.total&&dom.summary.includes(saved.activity.total+' 次发布')&&dom.source.includes(saved.activity.source_label)&&dom.source.includes(saved.activity.timezone),{total:saved.activity.total,active_days:saved.activity.active_days,start:saved.activity.start,end:saved.activity.end});
  const probes=[];
  for(const u of [1.5,3.2,4,5,5.6,6.01])probes.push({requested:u,state:await unit(p,u)});
  const expected=['paper-project','project-calendar','calendar','calendar-record','records','handoff'];
  check(label+' actual carrier and pinned stage survive the six formal phases',await carrier.evaluate(e=>e===document.querySelector('[data-work-carrier]'))&&probes.every(({requested,state:s},i)=>near(s.units,Math.min(6,requested),.003)&&(s.phase===expected[i]||(requested===5.6&&s.phase==='calendar-record'))&&!s.overflow&&(requested>=6?s.stage.alpha===0:near(s.stage.rect.top,0)&&near(s.stage.rect.height,s.height)))&&probes[0].state.carrier.alpha>.99&&probes[0].state.carrier.rect.width>100&&probes[1].state.carrier.alpha>.99&&probes[1].state.carrier.rect.width<probes[0].state.carrier.rect.width,{phases:probes.map(({requested,state:s})=>({requested,units:s.units,phase:s.phase,stage:s.stage,carrier:s.carrier,overflow:s.overflow}))});
  // Two CSS scroll pixels clear a boundary on both native viewport sizes.
  const calendar=probes[2].state,start=await unit(p,4.4+2/p.viewportSize().height),end=await unit(p,5.6+2/p.viewportSize().height),native=await unit(p,6.01);
  const nativeLocal=tile=>({...tile.rect,left:tile.rect.left-end.recordStage.rect.left,top:tile.rect.top-end.recordStage.rect.top});
  const coverLocal=cover=>({...cover.rect,left:cover.rect.left-end.stage.rect.left,top:cover.rect.top-end.stage.rect.top});
  check(label+' all 24 bridge surfaces begin at real visible dated cells',start.covers.length===24&&new Set(start.covers.map(c=>c.date)).size===24&&start.covers.every(c=>{const day=calendar.days.find(d=>d.date===c.date);return day&&sameRect(c.rect,day.rect,2)&&day.rect.left>=0&&day.rect.left+day.rect.width<=calendar.width&&c.background===day.background&&c.imageAlpha===0}),start.covers.map(c=>({index:c.index,date:c.date,rect:c.rect,background:c.background,imageAlpha:c.imageAlpha})));
  check(label+' all 24 saved covers meet their actual native grid and handoff',end.covers.length===24&&new Set(end.covers.map(c=>c.index)).size===24&&end.covers.every(c=>c.imageReady&&c.imageAlpha===1&&sameRect(coverLocal(c),nativeLocal(end.records[c.index]),2)&&c.src===end.records[c.index].src&&sameRect(c.rect,native.records[c.index].rect,2))&&native.covers.every(c=>c.alpha===0)&&native.records.length===24&&native.records.every(r=>r.alpha>.99&&r.target==='_blank'&&r.rel.includes('noopener')&&saved.music.some(m=>m.url===r.href&&m.image_url===r.src))&&near(native.recordStage.rect.top,0),{end_units:end.units,native_units:native.units,end:end.covers,native:native.records});
  const closed=await gameUnit(p,0),full=await gameUnit(p,1.6);
  check(label+' native album to game aperture, real scene and glass remain intact',closed.game.enhanced&&sameRect(closed.game.mask,closed.game.origin,2)&&closed.game.alpha===0&&closed.game.inert&&full.game.enhanced&&sameRect(full.game.mask,full.game.window,2)&&full.game.active===0&&full.game.ui===1&&!full.game.inert&&full.game.frames.filter(f=>f.current&&!f.inert).length===1&&full.game.frames.every(f=>saved.games.items.some(g=>String(g.provider_id)===f.id||String(g.id)===f.id))&&full.glass>.99&&!closed.overflow&&!full.overflow,{closed:closed.game,full:full.game,glass:full.glass});
  report.evidence[label]={probes,source:start,album_end:end,native_handoff:native,game_closed:closed,game_full:full};save();
  await carrier.dispose();
}
async function animate(p,from,to,duration){await p.evaluate(async({from,to,duration})=>{
  const root=document.querySelector('[data-home-work-journey]'),top=scrollY+root.getBoundingClientRect().top,height=innerHeight,start=performance.now();
  await new Promise(resolve=>{function next(now){const t=Math.min(1,(now-start)/duration);scrollTo({top:top+(from+(to-from)*t)*height,behavior:'instant'});if(t<1)requestAnimationFrame(next);else requestAnimationFrame(resolve)}requestAnimationFrame(next)});
},{from,to,duration})}
async function evidence(width,height,theme,saved){
  const label=width+'-'+theme,c=await browser.newContext({viewport:{width,height},colorScheme:theme,recordVideo:{dir:path.join(out,'.raw'),size:{width,height}}});
  await protect(c);await c.addInitScript(value=>{localStorage.setItem('painter-theme',value);localStorage.setItem('painter-analytics','off')},theme);
  const p=await c.newPage();p.on('pageerror',e=>report.errors.push({page:label,error:e.message}));p.on('console',message=>{if(message.type()==='error')report.console_errors.push({page:label,error:message.text()})});p.on('requestfailed',r=>{if(r.failure()?.errorText!=='net::ERR_ABORTED')report.failed_requests.push({page:label,url:cleanURL(r.url()),error:r.failure()?.errorText})});
  const response=await p.goto(base+'/',{waitUntil:'domcontentloaded'});check(label+' formal homepage returns 200',response.status()===200);
  await p.waitForFunction(()=>document.querySelector('[data-home-work-journey]')?.dataset.workMeasured==='true'&&document.querySelector('[data-game-music-journey]')?.classList.contains('is-record-game-journey'));
  await settle(p);await ready(p);await verify(p,label,saved);
  const stillUnit=width===1440?3.2:5;await unit(p,stillUnit);const readiness=await ready(p),still=label+'-formal.png';
  await p.screenshot({path:path.join(out,still),fullPage:false});report.captures.push({file:still,viewport:{width,height},theme,units:stillUnit,readiness,actual_formal_data:true,visually_reviewed:false});save();
  await unit(p,.85);await p.waitForTimeout(180);const started=Date.now();
  await animate(p,.85,6.1,9000);await p.waitForTimeout(300);await animate(p,6.1,.85,9000);await p.waitForTimeout(200);
  const duration=(Date.now()-started)/1000,video=p.video();await c.close();
  const raw=await video.path(),probe=spawnSync('ffprobe',['-v','error','-show_entries','format=duration','-of','default=noprint_wrappers=1:nokey=1',raw],{encoding:'utf8',timeout:15000});
  if(probe.status!==0)throw Error('Formal video duration probe failed: '+probe.stderr);
  const rawDuration=Number(probe.stdout.trim()),trimStart=Math.max(0,rawDuration-duration),file=label+'-formal-forward-reverse.webm';
  const encoded=spawnSync('ffmpeg',['-y','-loglevel','error','-ss',String(trimStart),'-i',raw,'-t',String(duration),'-an','-c:v','libvpx-vp9','-crf','35','-b:v','0','-deadline','realtime','-cpu-used','5','-row-mt','1','-threads','2',path.join(out,file)],{encoding:'utf8',timeout:120000});
  if(encoded.status!==0)throw Error('Formal video trim failed: '+encoded.stderr);
  report.clips.push({file,viewport:{width,height},theme,duration_seconds:duration,raw_duration_seconds:rawDuration,trim_start_seconds:trimStart,forward:{from:.85,to:6.1,duration_ms:9000},reverse:{from:6.1,to:.85,duration_ms:9000},actual_formal_data:true,native_browser_video:true,processing:'timestamp trim and VP9 encoding only; no animation or compositing added',visually_reviewed:false});save();
}
async function main(){
  if(process.env.PAINTER_V28_PROMOTED!=='1')throw Error('Withheld: set PAINTER_V28_PROMOTED=1 only after root confirms promotion');
  const origin=new URL(base);if(!['192.168.2.209','127.0.0.1','localhost'].includes(origin.hostname)||origin.port!=='18080')throw Error('Formal read-only origin must use local Painter :18080');
  fs.mkdirSync(out,{recursive:true});
  report.source_sha256=Object.fromEntries(sourceNames.map(name=>[name,sha(fs.readFileSync('web/frontend/src/'+name))]));report.test_sha256=sha(fs.readFileSync(__filename));
  const candidate=JSON.parse(fs.readFileSync('tests/artifacts/v28/functional-report.json','utf8')),viewer=JSON.parse(fs.readFileSync('tests/artifacts/v27/viewer/report.json','utf8'));
  check('promoted source matches the passing full candidate functional gate',candidate.summary?.failed===0&&!candidate.fatal&&sourceNames.every(name=>candidate.source_sha256[name]===report.source_sha256[name]),{artifact:'tests/artifacts/v28/functional-report.json',checks:candidate.checks?.length});
  check('shared V27 image viewer source remains byte-identical',viewer.passed&&['momentGallery.ts','momentGallery.css'].every(name=>viewer.sourceSha256[name]===report.source_sha256[name]),{artifact:'tests/artifacts/v27/viewer/report.json',checks:viewer.checks.length});
  browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
  const owner=await browser.newContext({storageState:ownerState});await protect(owner);const saved=await savedData(owner);await assetChecks(owner);await owner.close();
  await evidence(1440,900,'light',saved);await evidence(390,844,'dark',saved);
  check('formal verification made no business writes or browser errors',report.blocked_writes.length===0&&report.errors.length===0&&report.console_errors.length===0,{blocked:report.blocked_writes,errors:report.errors,console_errors:report.console_errors});
  check('bounded formal evidence contains two viewport stills and two real scroll clips',report.captures.length===2&&report.clips.length===2);
  report.passed=true;report.completed_at=new Date().toISOString();report.limitations=['Chrome desktop and mobile viewport emulation; physical touch hardware and Safari are outside this formal smoke.','The root separately verifies candidate and promoted binary hashes.','Visual review of the two actual formal clips and stills is recorded separately; this runner does not fabricate visual approval.'];save();
  await browser.close();console.log(JSON.stringify({passed:true,checks:report.checks.length,captures:2,clips:2,formal_business_writes:0,out}));
}
main().catch(async error=>{report.passed=false;report.failure=String(error.stack||error);save();console.error(String(error));await browser?.close();process.exitCode=1});
