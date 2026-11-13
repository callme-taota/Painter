// V35 candidate-only acceptance. Anonymous browser; no application writes.
const fs = require('node:fs')
const path = require('node:path')
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || '/tmp/painter-browser/node_modules/playwright')

const base = (process.env.PAINTER_BASE || 'http://127.0.0.1:18086').replace(/\/$/, '')
const out = path.resolve('tests/artifacts/v35')
const capture = !process.argv.includes('--functional-only')
const portraitOnly = process.argv.includes('--portrait-only')
const staticBoundaryOnly = process.argv.includes('--static-boundary')
const report = {base,read_only:true,checks:[],samples:[],captures:[],errors:[],console_errors:[],bad_responses:[],blocked_writes:[],blocked_analytics:0,image_readiness:[],limitations:[]}
let browser
const reportName=staticBoundaryOnly?'continuity-static-boundary-report.json':portraitOnly?'continuity-portrait-report.json':'continuity-report.json'
const save = () => {fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,reportName),JSON.stringify(report,null,2)+'\n')}
const check = (name,passed,detail) => {report.checks.push({name,passed:!!passed,...(detail===undefined?{}:{detail})});console.log(`${passed?'PASS':'FAIL'} ${name}`);return !!passed}
const near = (a,b,t=1.5) => Number.isFinite(a)&&Number.isFinite(b)&&Math.abs(a-b)<=t
const rectDiff = (a,b) => !a||!b?Infinity:Math.max(...['left','top','width','height'].map(k=>Math.abs(a[k]-b[k])))
const clean = raw => {try{const u=new URL(raw);return u.origin+u.pathname}catch{return raw}}
const frames = async p => p.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))

async function context(options={}) {
  const {motionOff=false,staticSubset=false,...browserOptions}=options
  const c=await browser.newContext({viewport:{width:1440,height:1000},colorScheme:'light',...browserOptions})
  c.setDefaultTimeout(15000)
  await c.route('**/*',async route=>{
    const request=route.request(),url=new URL(request.url())
    if(url.pathname.includes('/analytics/')) {report.blocked_analytics++;return route.fulfill({status:204,body:''})}
    if(!['GET','HEAD'].includes(request.method())) {report.blocked_writes.push({method:request.method(),url:clean(request.url())});return route.fulfill({status:204,body:''})}
    if((motionOff||staticSubset)&&request.isNavigationRequest()&&url.origin===new URL(base).origin&&url.pathname==='/') {
      const response=await route.fetch(),headers={...response.headers()}
      delete headers['content-length'];delete headers['content-encoding']
      let html=await response.text()
      if(motionOff)html=html.replace(/<body\b([^>]*)>/i,(_,attrs)=>'<body'+attrs.replace(/\sdata-home-motion=(?:"[^"]*"|'[^']*')/i,'')+' data-home-motion="false">')
      if(staticSubset) {
        // The known SSR grid contains only flat anchor children. Preserve the
        // exact first-album and game markup; remove other albums in this one
        // browser response only. Never submit configuration or content writes.
        const grid=html.match(/(<div\b[^>]*class="[^"]*\bhome-record-grid\b[^"]*"[^>]*>)([\s\S]*?)(<\/div>)/)
        if(!grid)throw Error('static fixture: SSR grid was not found')
        const anchors=grid[2].match(/<a\b[\s\S]*?<\/a>/g)||[]
        const album=anchors.find(a=>!a.includes('data-record-game-tile')),game=anchors.find(a=>a.includes('data-record-game-tile'))
        if(!album||!game||anchors.length!==25)throw Error('static fixture: expected 24 albums and one real game')
        html=html.replace(grid[0],grid[1]+album+game+grid[3])
      }
      return route.fulfill({status:response.status(),headers,body:html})
    }
    return route.continue()
  })
  if(options.javaScriptEnabled!==false)await c.addInitScript(theme=>{localStorage.setItem('painter-theme',theme);localStorage.setItem('painter-analytics','off')},options.colorScheme||'light')
  return c
}
async function page(c,label) {
  const p=await c.newPage()
  p.on('pageerror',error=>report.errors.push({label,error:error.message}))
  p.on('console',message=>{if(message.type()==='error')report.console_errors.push({label,error:message.text()})})
  p.on('response',response=>{if(response.status()>=400&&new URL(response.url()).origin===new URL(base).origin)report.bad_responses.push({label,url:clean(response.url()),status:response.status()})})
  return p
}
async function ready(p,enhanced=true) {
  const response=await p.goto(base+'/',{waitUntil:'domcontentloaded'})
  if(!response?.ok())throw Error('homepage status '+response?.status())
  if(enhanced)await p.waitForFunction(()=>document.querySelector('[data-home-work-journey]')?.dataset.workMeasured==='true'&&document.querySelector('[data-game-story]')?.classList.contains('is-enhanced'))
  await p.evaluate(()=>document.fonts.ready)
  if(enhanced)await frames(p)
}
async function assets(p,label) {
  const result=await p.evaluate(async()=>{
    const visible=img=>{const r=img.getBoundingClientRect();if(r.bottom<=0||r.top>=innerHeight||r.right<=0||r.left>=innerWidth||!r.width||!r.height)return false;for(let n=img;n;n=n.parentElement){const s=getComputedStyle(n);if(s.display==='none'||s.visibility==='hidden'||Number(s.opacity)<.01)return false}return true}
    const images=[...new Set([...document.images].filter(visible).concat([...document.querySelectorAll('[data-game-tile-hero],[data-game-frame]:first-child [data-game-hero]')]))]
    return Promise.all(images.map(async img=>{img.loading='eager';let error='';try{await Promise.race([img.decode(),new Promise((_,reject)=>setTimeout(()=>reject(Error('decode timeout')),12000))])}catch(e){error=e.message}return{src:img.currentSrc||img.src,complete:img.complete,width:img.naturalWidth,error}}))
  })
  report.image_readiness.push({label,images:result.map(x=>({...x,src:clean(x.src)}))})
  return result.every(x=>x.complete&&x.width>0)
}
async function state(p) {
  return p.evaluate(()=>{
    const q=s=>document.querySelector(s),r=e=>{if(!e)return null;const b=e.getBoundingClientRect();return{left:b.left,top:b.top,width:b.width,height:b.height,right:b.right,bottom:b.bottom}},paint=e=>{if(!e)return null;let alpha=1,hidden=false;for(let n=e;n;n=n.parentElement){const s=getComputedStyle(n);alpha*=Number(s.opacity);hidden||=n.hidden||s.display==='none'||s.visibility==='hidden'}const s=getComputedStyle(e);return{rect:r(e),alpha:hidden?0:alpha,position:s.position,transform:s.transform,background:s.backgroundColor,radius:s.borderRadius,inert:e.inert,hidden}}
    const work=q('[data-home-work-journey]'),stage=q('.home-work-stage'),record=q('[data-record-wall]'),recordStage=q('.home-record-stage'),grid=q('.home-record-grid'),game=q('[data-game-story]'),gameStage=q('[data-game-stage]'),tile=q('[data-record-game-tile]'),art=tile?.querySelector('img'),hero=q('[data-game-frame] [data-game-hero]'),aperture=q('[data-game-window]'),gridBox=r(grid),tileBox=r(tile),fixed=[]
    for(let n=tile;n;n=n.parentElement)if(getComputedStyle(n).position==='fixed')fixed.push(n.className||n.tagName)
    const crop=img=>{if(!img)return null;const b=img.getBoundingClientRect(),s=getComputedStyle(img),scale=Math.max(b.width/Math.max(1,img.naturalWidth),b.height/Math.max(1,img.naturalHeight));return{rect:r(img),src:img.currentSrc||img.src,loaded:img.complete&&img.naturalWidth>0,fit:s.objectFit,position:s.objectPosition,visibleX:b.width/(scale*Math.max(1,img.naturalWidth)),visibleY:b.height/(scale*Math.max(1,img.naturalHeight)),alpha:paint(img).alpha}}
    const albums=[...document.querySelectorAll('[data-record-wall] .home-record-tile:not([data-record-game-tile])')]
    const days=[...document.querySelectorAll('[data-activity-calendar] [data-date]')]
    return{scroll:scrollY,viewport:{width:innerWidth,height:innerHeight},theme:document.documentElement.dataset.theme,overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth+1,snapping:document.body.dataset.homeSnapping||'',work:{units:Number(work?.dataset.workUnits),phase:work?.dataset.workPhase,rect:r(work),stage:paint(stage),carrier:paint(q('[data-work-carrier]'))},record:{rect:r(record),stage:paint(recordStage),grid:paint(grid),albums:albums.map(a=>({href:a.href,src:a.querySelector('img')?.src,rect:r(a),gridLocal:{left:r(a).left-gridBox.left,top:r(a).top-gridBox.top,width:r(a).width,height:r(a).height}}))},tile:{...paint(tile),crop:crop(art),key:tile?.dataset.gameKey,href:tile?.href,gridChild:tile?.parentElement===grid,count:document.querySelectorAll('[data-record-game-tile]').length,fixedAncestors:fixed,sameDOM:window.__v35Tile===tile,visibleFraction:tileBox?Math.max(0,Math.min(innerWidth,tileBox.right)-Math.max(0,tileBox.left))*Math.max(0,Math.min(innerHeight,tileBox.bottom)-Math.max(0,tileBox.top))/Math.max(1,tileBox.width*tileBox.height):0,shade:tile?getComputedStyle(tile,':after').opacity:null},game:{rect:r(game),stage:r(gameStage),aperture:paint(aperture),hero:crop(hero),phase:game?.dataset.gamePhase,current:game?.dataset.gameActive,frameCount:document.querySelectorAll('[data-game-frame]').length},identity:{albums:window.__v35Albums?.every((a,i)=>a===albums[i]),carrier:window.__v35Carrier===q('[data-work-carrier]'),projects:window.__v35Projects?.every((a,i)=>a===document.querySelectorAll('.project-card-link')[i]),days:window.__v35Days?.every((a,i)=>a===days[i]),dayData:window.__v35DayData===JSON.stringify(days.map(e=>[e.dataset.date,e.dataset.count,e.dataset.level]))},activity:{count:days.length,total:days.reduce((sum,e)=>sum+Number(e.dataset.count),0),dates:days.map(e=>e.dataset.date)},dotNav:document.querySelectorAll('[data-game-nav],[data-game-jump],.game-scene-nav').length}
  })
}
async function move(p,kind,unit,label) {
  await p.evaluate(({kind,unit})=>{const target=document.querySelector(kind==='work'?'[data-home-work-journey]':kind==='record'?'[data-record-wall]':'[data-game-story]'),stage=document.querySelector(kind==='work'?'.home-work-stage':'[data-game-stage]');scrollTo({top:scrollY+target.getBoundingClientRect().top+unit*stage.clientHeight,behavior:'instant'})},{kind,unit})
  await frames(p)
  const s=await state(p);report.samples.push({label,kind,unit,...s});return s
}
async function screenshot(p,label) {
  if(!capture)return
  const decoded=await assets(p,label)
  check(label+' visible artwork decoded for evidence',decoded)
  const filename=label+'.png';await p.screenshot({path:path.join(out,filename),animations:'disabled'});report.captures.push(filename)
}
const albumSignature=s=>JSON.stringify(s.record.albums.map(({href,src})=>({href,src})))

async function geometryVariant(viewport,theme) {
  const label=`${viewport.width}-${theme}`,c=await context({viewport,colorScheme:theme,isMobile:viewport.width<500,hasTouch:viewport.width<500}),p=await page(c,label)
  try {
    await ready(p)
    const initialized=await p.evaluate(()=>{window.__v35Tile=document.querySelector('[data-record-game-tile]');window.__v35Albums=[...document.querySelectorAll('[data-record-wall] .home-record-tile:not([data-record-game-tile])')];window.__v35Carrier=document.querySelector('[data-work-carrier]');window.__v35Projects=[...document.querySelectorAll('.project-card-link')];window.__v35Days=[...document.querySelectorAll('[data-activity-calendar] [data-date]')];window.__v35DayData=JSON.stringify(window.__v35Days.map(e=>[e.dataset.date,e.dataset.count,e.dataset.level]));return{tile:!!window.__v35Tile,albums:window.__v35Albums.length,projects:window.__v35Projects.length,days:window.__v35Days.length}})
    check(label+' SSR-derived media and work sequence are present',initialized.tile&&initialized.albums===24&&initialized.projects>0&&initialized.days===365,initialized)
    if(!initialized.tile)return
    const first=await move(p,'work',.2,label)
    check(label+' no horizontal dot UI or page overflow',first.dotNav===0&&!first.overflow)
    const article=await move(p,'work',.85,label)
    check(label+' intro and Painter carrier keep their original DOM',article.identity.carrier&&article.identity.projects&&article.identity.days)
    await screenshot(p,label+'-intro-card')
    for(const unit of [1.55,2.25,3.2,4.05,5,5.8]) {
      const s=await move(p,'work',unit,label)
      check(label+' actual project/day DOM and count survive work '+unit,s.identity.carrier&&s.identity.projects&&s.identity.days&&s.identity.dayData&&s.activity.total===first.activity.total&&s.activity.count===365&&!s.overflow,{phase:s.work.phase,total:s.activity.total})
      if(unit===4.05)await screenshot(p,label+'-calendar')
    }
    const zero=await move(p,'record',0,label);await screenshot(p,label+'-record-0')
    const half=await move(p,'record',.5,label),one=await move(p,'record',1,label)
    check(label+' grid itself scrolls upward one actual viewport',near(zero.record.grid.rect.top-one.record.grid.rect.top,viewport.height,2)&&near(zero.record.grid.rect.top-half.record.grid.rect.top,viewport.height*.5,2),{start:zero.record.grid.rect.top,half:half.record.grid.rect.top,end:one.record.grid.rect.top,height:viewport.height})
    check(label+' scrolling keeps square album geometry and fixed grid dimensions',zero.record.albums.every((a,i)=>rectDiff(a.gridLocal,one.record.albums[i]?.gridLocal)<1)&&near(zero.record.grid.rect.height,one.record.grid.rect.height,1),{gridStart:zero.record.grid.rect,gridEnd:one.record.grid.rect})
    let start=await move(p,'game',0,label)
    check(label+' real first game is a visible native grid cell',start.tile.gridChild&&start.tile.count===1&&start.tile.sameDOM&&start.tile.visibleFraction>.98&&start.tile.fixedAncestors.length===0,{tile:start.tile.rect,visible:start.tile.visibleFraction,fixed:start.tile.fixedAncestors})
    check(label+' game opening starts after one screen of real record travel',near(start.scroll-zero.scroll,viewport.height,2),{record0:zero.scroll,game0:start.scroll})
    const signature=albumSignature(start),forward=new Map()
    await assets(p,label+'-game-art')
    start=await state(p)
    await screenshot(p,label+'-game-0')
    for(const direction of ['forward','reverse']) {
      const units=direction==='forward'?[0,.3,.6,1,1.19,1.199,1.2,1.22]:[1.22,1.2,1.199,1.19,1,.6,.3,0]
      for(const unit of units) {
        const s=await move(p,'game',unit,label+'-'+direction)
        check(`${label} ${direction} game ${unit} retains same tile, source and 24 albums`,s.tile.sameDOM&&s.tile.gridChild&&s.tile.crop.src===s.game.hero.src&&s.identity.albums&&albumSignature(s)===signature&&s.tile.fixedAncestors.length===0&&!s.overflow,{tile:s.tile.rect,source:s.tile.crop.src,frame:s.game.hero.src})
        check(`${label} ${direction} game ${unit} preserves source crop geometry`,s.tile.crop.fit==='cover'&&s.tile.crop.position===s.game.hero.position&&near(s.tile.crop.rect.width/s.tile.rect.width,s.game.hero.rect.width/s.game.stage.width,.01)&&near(s.tile.crop.rect.height/s.tile.rect.height,s.game.hero.rect.height/s.game.stage.height,.01),{tile:s.tile.rect,image:s.tile.crop.rect,fit:s.tile.crop.fit,position:s.tile.crop.position})
        if(direction==='forward')forward.set(unit,s)
        else if(forward.has(unit))check(`${label} reverse returns to the same game geometry at ${unit}`,rectDiff(s.tile.rect,forward.get(unit).tile.rect)<1.5,{forward:forward.get(unit).tile.rect,reverse:s.tile.rect})
        if(unit<1.2)check(`${label} ${direction} game ${unit} has one visible game surface`,s.tile.alpha>.99&&s.game.aperture.alpha<.01,{tile:s.tile.alpha,full:s.game.aperture.alpha})
        if(unit===1.19||unit===1.199)check(`${label} ${direction} closing game edges already meet the full viewport at ${unit}`,rectDiff(s.tile.crop.rect,s.game.hero.rect)<1,{tileImage:s.tile.crop.rect,fullImage:s.game.hero.rect})
        if(unit===1.2)check(`${label} ${direction} full-screen handoff has exact image and shade continuity`,rectDiff(s.tile.crop.rect,s.game.hero.rect)<1&&s.tile.alpha<.01&&s.game.aperture.alpha>.99&&s.tile.shade==='1',{tileImage:s.tile.crop.rect,fullImage:s.game.hero.rect,tileAlpha:s.tile.alpha,fullAlpha:s.game.aperture.alpha,shade:s.tile.shade})
        if(direction==='forward'&&unit===.6)await screenshot(p,label+'-game-mid')
        if(direction==='forward'&&unit===1.2)await screenshot(p,label+'-game-full')
      }
    }
    const mid=forward.get(.6),end=forward.get(1.2)
    check(label+' zoom grows monotonically while crop follows the changing surface',start.tile.rect.width<=mid.tile.rect.width&&mid.tile.rect.width<=end.tile.rect.width&&start.tile.rect.height<=mid.tile.rect.height&&mid.tile.rect.height<=end.tile.rect.height&&mid.tile.crop.visibleX>=Math.min(start.tile.crop.visibleX,end.tile.crop.visibleX)-.005&&mid.tile.crop.visibleX<=Math.max(start.tile.crop.visibleX,end.tile.crop.visibleX)+.005,{start:start.tile.crop.visibleX,mid:mid.tile.crop.visibleX,end:end.tile.crop.visibleX})
    const program=await move(p,'work',4.05-.06,label);await p.waitForTimeout(800)
    check(label+' programmatic positioning does not trigger magnetic scrolling',near((await state(p)).scroll,program.scroll,1))
    save()
  } finally {await c.close()}
}

async function inputChecks() {
  const label='1440-input',c=await context(),p=await page(c,label)
  try {
    await ready(p)
    const anchor=await p.locator('[data-home-work-journey]').evaluate(e=>scrollY+e.getBoundingClientRect().top+4.05*document.querySelector('.home-work-stage').clientHeight)
    const position=async y=>{await p.evaluate(top=>scrollTo({top,behavior:'instant'}),y);await frames(p)}
    const wheel=async(start,delta)=>{await position(start);await p.mouse.move(4,500);await p.mouse.wheel(0,delta);await p.waitForTimeout(100);const native=await state(p);await p.waitForTimeout(750);return{native,final:await state(p)}}
    const down=await wheel(anchor-80,30)
    check('real downward wheel settles only forward within a small range',down.final.scroll>=down.native.scroll-1&&down.final.scroll-down.native.scroll<=121&&near(down.final.scroll,anchor,2),{anchor,native:down.native.scroll,final:down.final.scroll})
    const up=await wheel(anchor+80,-30)
    check('real upward wheel settles only backward within a small range',up.final.scroll<=up.native.scroll+1&&up.native.scroll-up.final.scroll<=121&&near(up.final.scroll,anchor,2),{anchor,native:up.native.scroll,final:up.final.scroll})
    const past=await wheel(anchor+20,35)
    check('a stable anchor behind a downward gesture never pulls backward',past.final.scroll>=past.native.scroll-1&&near(past.final.scroll,past.native.scroll,2),{native:past.native.scroll,final:past.final.scroll})
    await position(anchor-60);const plain=await state(p);await p.waitForTimeout(800)
    check('scrollTo alone never opens a snap intent',near((await state(p)).scroll,plain.scroll,1))
    await position(anchor-80);await p.keyboard.press('ArrowDown');await p.waitForTimeout(200);const keyboard=await state(p);await p.waitForTimeout(750)
    check('keyboard page movement is not pulled into an anchor',near((await state(p)).scroll,keyboard.scroll,1))
    await position(anchor-80);await p.mouse.move(4,500);await p.mouse.wheel(0,30);await p.waitForTimeout(255);await p.mouse.wheel(0,420);await p.waitForTimeout(750)
    check('new wheel input interrupts magnetic motion without trapping the page',(await state(p)).scroll>anchor+200)
  } finally {await c.close()}

  const mc=await context({viewport:{width:390,height:844},hasTouch:true,isMobile:true}),mp=await page(mc,'390-nested')
  try {
    await ready(mp);await move(mp,'work',4.05,'390-nested')
    const scroller=mp.locator('.activity-scroll'),before=await scroller.evaluate(e=>({left:e.scrollLeft,range:e.scrollWidth-e.clientWidth,y:scrollY})),box=await scroller.boundingBox()
    if(before.range>2&&box) {
      await mp.mouse.move(Math.max(24,box.x+Math.min(box.width/2,100)),box.y+box.height/2)
      await mp.mouse.wheel(before.left>20?-80:80,0);await mp.waitForTimeout(800)
      const after=await scroller.evaluate(e=>({left:e.scrollLeft,y:scrollY}))
      check('native internal horizontal calendar scrolling keeps page ownership',Math.abs(after.left-before.left)>2&&near(after.y,before.y,1),{before,after})
    } else check('mobile calendar keeps an independently scrollable native range',false,before)
  } finally {await mc.close()}
}

async function fallback(mode,viewport) {
  const label=`${viewport.width}-${mode}`,c=await context({viewport,reducedMotion:mode==='reduced'?'reduce':'no-preference',javaScriptEnabled:mode!=='nojs',motionOff:mode==='off',isMobile:viewport.width<500,hasTouch:viewport.width<500}),p=await page(c,label)
  try {
    await ready(p,false);await p.waitForTimeout(120)
    const s=await p.evaluate(()=>{const visible=e=>{const b=e.getBoundingClientRect(),s=getComputedStyle(e);return b.width>0&&b.height>0&&s.display!=='none'&&s.visibility!=='hidden'&&Number(s.opacity)>.99&&!e.closest('[inert]')};return{work:document.querySelector('[data-home-work-journey]')?.classList.contains('work-journey-ready'),game:document.querySelector('[data-game-story]')?.classList.contains('is-enhanced'),albums:[...document.querySelectorAll('[data-record-wall] .home-record-tile:not([data-record-game-tile])')].filter(visible).length,tile:document.querySelector('[data-record-game-tile]')?.parentElement?.classList.contains('home-record-grid'),frames:[...document.querySelectorAll('[data-game-frame]')].map(e=>({visible:visible(e),position:getComputedStyle(e).position,href:!!e.querySelector('a[href]')})),overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth+1}})
    check(label+' ordinary document flow retains all records and games',!s.work&&!s.game&&s.albums===24&&s.tile&&s.frames.length>0&&s.frames.every(e=>e.visible&&e.position!=='fixed')&&!s.overflow,s)
  } finally {await c.close()}
}

async function ssr() {
  const c=await context({javaScriptEnabled:false}),p=await page(c,'SSR')
  try {
    await p.goto(base+'/',{waitUntil:'domcontentloaded'})
    const s=await p.evaluate(()=>{const grid=document.querySelector('.home-record-grid'),tile=grid?.querySelector('[data-record-game-tile]'),first=document.querySelector('[data-game-frame]'),art=tile?.querySelector('img'),hero=first?.querySelector('[data-game-hero]');return{tile:!!tile,last:tile===grid?.lastElementChild,inside:tile?.parentElement===grid,albums:grid?.querySelectorAll('.home-record-tile:not([data-record-game-tile])').length,src:art?.src,gameSrc:hero?.src,key:tile?.dataset.gameKey,gameKey:first?.dataset.gameKey,href:tile?.getAttribute('href'),gameHref:first?.querySelector('.game-store-link')?.getAttribute('href'),cta:document.querySelectorAll('.record-game-origin').length,dots:document.querySelectorAll('[data-game-nav],[data-game-jump]').length}})
    check('SSR owns one real extra game tile inside the saved 24-album grid',s.tile&&s.last&&s.inside&&s.albums===24&&s.src===s.gameSrc&&s.key===s.gameKey&&s.href===(s.gameHref||'/games')&&s.cta===0&&s.dots===0,s)
  } finally {await c.close()}
}

async function portraitViewportCheck(viewport) {
  const label=`${viewport.width}x${viewport.height}-portrait`,c=await context({viewport}),p=await page(c,label)
  try {
    await ready(p)
    await p.evaluate(()=>{window.__v35Tile=document.querySelector('[data-record-game-tile]');window.__v35Albums=[...document.querySelectorAll('[data-record-wall] .home-record-tile:not([data-record-game-tile])')]})
    const zero=await move(p,'record',0,label),one=await move(p,'record',1,label)
    check(label+' real record grid still travels a full viewport',near(zero.record.grid.rect.top-one.record.grid.rect.top,viewport.height,2),{start:zero.record.grid.rect.top,end:one.record.grid.rect.top,viewport:viewport.height})
    check(label+' square album geometry stays intact throughout travel',zero.record.albums.length===24&&zero.record.albums.every((a,i)=>rectDiff(a.gridLocal,one.record.albums[i]?.gridLocal)<1&&near(a.rect.width,a.rect.height,1)))
    const signature=albumSignature(one)
    await assets(p,label+'-source')
    let start
    for(const unit of [0,.6,1.199,1.2,1.199,.6,0]) {
      const s=await move(p,'game',unit,label)
      check(label+' same grid game and all album destinations survive zoom '+unit,s.tile.sameDOM&&s.tile.gridChild&&s.identity.albums&&albumSignature(s)===signature&&s.tile.crop.src===s.game.hero.src&&s.tile.fixedAncestors.length===0&&!s.overflow,{tile:s.tile.rect,visible:s.tile.visibleFraction})
      if(unit===0) {
        check(label+' source game cell is actually visible before growth',s.tile.visibleFraction>.98,{tile:s.tile.rect,visible:s.tile.visibleFraction})
        if(start)check(label+' reverse zoom returns to the original real cell',rectDiff(start.tile.rect,s.tile.rect)<1.5)
        else start=s
      }
      if(unit>=1.199)check(label+' near and exact fullscreen crop edges stay continuous '+unit,rectDiff(s.tile.crop.rect,s.game.hero.rect)<1,{cell:s.tile.crop.rect,frame:s.game.hero.rect})
    }
    if(capture) {await move(p,'game',0,label);await screenshot(p,label+'-game-source')}
  } finally {await c.close()}
}

async function staticBoundaries() {
  report.case_scope='Response-only static fallback boundary: full saved 24 albums plus first game; sparse fixture retains the exact original first-album and game SSR anchors. No data, configuration, session or content writes.'
  for(const viewport of [{width:1440,height:1000},{width:390,height:844}]) {
    let original
    for(const sparse of [false,true])for(const mode of sparse?['nojs','reduced']:['nojs','reduced','off']) {
      const label=`${viewport.width}-${mode}-${sparse?'one-album-one-game':'saved-25'}`
      const c=await context({viewport,javaScriptEnabled:mode!=='nojs',reducedMotion:mode==='reduced'?'reduce':'no-preference',motionOff:mode==='off',staticSubset:sparse,isMobile:viewport.width<500,hasTouch:viewport.width<500}),p=await page(c,label)
      try {
        await ready(p,false);await p.waitForTimeout(100)
        const s=await p.evaluate(()=>{const grid=document.querySelector('.home-record-grid'),tile=grid.querySelector('[data-record-game-tile]'),album=grid.querySelector('.home-record-tile:not([data-record-game-tile])'),items=[...grid.children],r=e=>{const b=e.getBoundingClientRect();return{left:b.left,top:b.top,width:b.width,height:b.height,right:b.right,bottom:b.bottom}},art=e=>e.querySelector('img')?.src;const ts=getComputedStyle(tile),gs=getComputedStyle(grid);return{children:items.length,albums:items.filter(e=>!e.hasAttribute('data-record-game-tile')).length,gameCount:items.filter(e=>e.hasAttribute('data-record-game-tile')).length,album:r(album),game:r(tile),previous:r(tile.previousElementSibling),columns:gs.gridTemplateColumns.split(' ').length,row:ts.gridRowStart,column:ts.gridColumnStart,position:ts.position,hrefs:{album:album.href,albumSrc:art(album),game:tile.href,gameSrc:art(tile)},enhanced:document.querySelector('[data-game-story]').classList.contains('is-enhanced')||tile.classList.contains('is-record-game-tile-enhanced'),work:document.querySelector('[data-home-work-journey]')?.classList.contains('work-journey-ready'),inert:!!tile.closest('[inert]'),display:ts.display,overflow:document.documentElement.scrollWidth>document.documentElement.clientWidth+1}})
        report.samples.push({label,mode,sparse,...s})
        if(!original&&!sparse)original=s.hrefs
        check(label+' retains the exact native links and image sources',JSON.stringify(s.hrefs)===JSON.stringify(original),s.hrefs)
        check(label+' remains an unenhanced native collection',!s.enhanced&&!s.work&&!s.inert&&s.gameCount===1&&s.children===(sparse?2:25)&&s.albums===(sparse?1:24)&&s.position!=='fixed'&&s.display!=='none'&&!s.overflow,{children:s.children,albums:s.albums,row:s.row,column:s.column,enhanced:s.enhanced})
        check(label+' game has no forced grid row or column',s.row==='auto'&&s.column==='auto',{row:s.row,column:s.column})
        if(sparse)check(label+' first game naturally neighbors its one album without empty rows',near(s.game.top,s.album.top,1)&&near(s.game.left,s.album.right,1)&&near(s.game.width,s.album.width,1)&&near(s.game.height,s.album.height,1),{album:s.album,game:s.game,columns:s.columns})
        else check(label+' last game naturally follows the 24 saved albums',near(s.game.left,s.album.left,1)&&near(s.game.top,s.previous.top+s.previous.height,1),{first:s.album,previous:s.previous,game:s.game})
      } finally {await c.close()}
    }
  }
}

async function main() {
  if(process.env.PAINTER_V35_RUN!=='1') {console.log('Prepared. Run PAINTER_V35_RUN=1 [PAINTER_BASE=http://127.0.0.1:18086] node tests/browser/v35-home-continuity.cjs after candidate readiness.');return}
  fs.mkdirSync(out,{recursive:true})
  browser=await chromium.launch({executablePath:process.env.CHROME_BIN||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']})
  try {
    if(staticBoundaryOnly) {
      await staticBoundaries()
    } else if(portraitOnly) {
      for(const viewport of [{width:768,height:1024},{width:768,height:1366}])await portraitViewportCheck(viewport)
    } else {
    await ssr()
    for(const viewport of [{width:1440,height:1000},{width:390,height:844}])for(const theme of ['light','dark']) {
      try{await geometryVariant(viewport,theme)}catch(error){report.errors.push({label:`${viewport.width}-${theme}`,error:error.stack||error.message});check(`${viewport.width}-${theme} scenario completed`,false,error.message);save()}
    }
    await inputChecks()
    for(const viewport of [{width:1440,height:1000},{width:390,height:844}])for(const mode of ['reduced','nojs','off'])await fallback(mode,viewport)
    }
    check('all candidate contexts remained free of runtime errors',report.errors.length===0,report.errors)
    check('candidate pages and public assets returned no HTTP errors',report.bad_responses.length===0,report.bad_responses)
  } finally {await browser.close();report.passed=report.checks.every(c=>c.passed)&&report.errors.length===0;save()}
  console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,failed:report.checks.filter(c=>!c.passed).length,captures:report.captures.length,report:path.join(out,reportName)}))
  if(!report.passed)process.exitCode=1
}
main().catch(async error=>{report.errors.push({error:error.stack||error.message});report.passed=false;save();console.error(error);try{await browser?.close()}catch{}process.exitCode=1})
