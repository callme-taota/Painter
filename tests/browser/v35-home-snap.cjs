// Isolated native-input checks. No server, database, assets, or network writes.
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const {chromium} = require('/tmp/painter-browser/node_modules/playwright')
const ts = require('../../web/frontend/node_modules/typescript')

const file = path.join(__dirname, '../../web/frontend/src/homeSnap.ts')
const code = ts.transpileModule(fs.readFileSync(file, 'utf8'), {compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}}).outputText
const report = {checks:[], errors:[]}
const output = path.join(__dirname, '../artifacts/v35/home-snap-isolated.json')
const point = {key:'stable',top:1000,from:800,to:1200}
let browser

async function fixture(mobile = false) {
  const context = await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:900},hasTouch:mobile,isMobile:mobile})
  await context.route('**/*', route => route.abort())
  const page = await context.newPage()
  page.on('pageerror', error => report.errors.push(error.message))
  await page.setContent(`<!doctype html><meta name="viewport" content="width=device-width"><body data-page-kind="home"><style>
    html{overflow-x:clip}body{margin:0;min-height:6000px;overflow-y:auto}
    #horizontal,#vertical{position:fixed;left:20px;width:230px;height:100px;background:#eee}
    #horizontal{top:120px;overflow-x:auto}#horizontal>div{width:900px;height:80px}
    #vertical{top:240px;overflow-y:auto}#vertical>div{height:700px}
    #control,#day{position:fixed;right:20px;top:16px}#day{top:65px}
  </style><main id="scene"></main><div id="horizontal"><div></div></div><div id="vertical"><div></div></div><button id="control">Action</button><span id="day" tabindex="0">Focusable day</span><dialog id="dialog">Dialog</dialog></body>`)
  await page.addScriptTag({type:'module',content:code+`
    window.points=${JSON.stringify([point])};
    window.dispose=registerHomeSnapSource(document.querySelector('#scene'),()=>window.points);
    window.registerSnap=registerHomeSnapSource;
    window.trace=[];
    addEventListener('scroll',()=>window.trace.push({y:scrollY,snap:document.body.dataset.homeSnapping||''}));
    initHomeSnap();window.ready=true;
  `})
  await page.waitForFunction(() => window.ready)
  const y = () => page.evaluate(() => scrollY)
  const seed = async (value, points = [point]) => {
    await page.keyboard.press('Shift') // Clear any prior gesture without scrolling.
    await page.evaluate(({value,points}) => {
      document.querySelector('dialog').close();document.activeElement?.blur()
      document.body.dataset.homeMotion='true';window.points=points
      scrollTo({top:value,behavior:'instant'});window.trace=[]
    },{value,points})
    await page.waitForTimeout(60)
  }
  const wheel = async (deltaY, deltaX = 0, at) => {
    const viewport=page.viewportSize()
    await page.mouse.move(at?.x??viewport.width-30,at?.y??viewport.height-60)
    await page.mouse.wheel(deltaX,deltaY)
  }
  const finish = async () => page.waitForTimeout(580)
  return {page,context,y,seed,wheel,finish}
}

;(async () => {
  browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']})
  const check=(name,ok,details={})=>{assert.ok(ok,name+' '+JSON.stringify(details));report.checks.push({name,passed:true,...details})}
  const f=await fixture()
  const {page,y,seed,wheel,finish}=f

  await seed(940);await finish()
  check('programmatic scroll without new input never snaps',await y()===940)
  await seed(750);await wheel(180);await finish()
  check('downward native wheel settles only forward',Math.abs(await y()-1000)<2,{y:await y(),trace:await page.evaluate(()=>window.trace)})
  const count=await page.evaluate(()=>window.trace.length);await page.waitForTimeout(500)
  check('completed gesture produces no idle scroll loop',await page.evaluate(()=>window.trace.length)===count)
  await page.evaluate(()=>scrollTo(0,940));await finish()
  check('consumed intent cannot replay on a later programmatic scroll',await y()===940)
  await seed(1230);await wheel(-180);await finish()
  check('upward native wheel settles only upward',Math.abs(await y()-1000)<2)
  await seed(900);await wheel(150);await finish()
  check('downward overshoot is never pulled backward',Math.abs(await y()-1050)<2)
  await seed(700);await wheel(150);await finish()
  check('distance over 0.12 viewport is left to the visitor',Math.abs(await y()-850)<2)
  await seed(1100);await wheel(220);await finish()
  check('unregistered transition range never snaps',Math.abs(await y()-1320)<2)
  await seed(880,[{key:'behind',top:990,from:800,to:1200},{key:'ahead',top:1060,from:800,to:1200}]);await wheel(140);await finish()
  check('nearest same-direction point wins over closer reverse point',Math.abs(await y()-1060)<2)
  await seed(750);await wheel(180)
  await page.waitForFunction(()=>document.body.dataset.homeSnapping==='stable')
  await wheel(-240);await finish()
  check('new native input immediately cancels a running settle',await y()<900&&!await page.locator('body').getAttribute('data-home-snapping'))
  await seed(750);await wheel(180)
  await page.waitForFunction(()=>document.body.dataset.homeSnapping==='stable')
  await page.evaluate(()=>scrollTo(0,1700));await finish()
  check('external programmatic movement also interrupts an active settle',Math.abs(await y()-1700)<2&&!await page.locator('body').getAttribute('data-home-snapping'))
  await seed(750);await wheel(180);await page.waitForTimeout(80);await page.keyboard.press('Shift');await finish()
  check('keyboard cancels pending intent',Math.abs(await y()-930)<2)
  await seed(750);await page.locator('#control').focus();await wheel(180);await finish()
  check('focused native control owns scrolling',Math.abs(await y()-930)<2)
  await seed(750);await page.locator('#day').focus();await wheel(180);await finish()
  check('generic tabindex focus also owns scrolling',Math.abs(await y()-930)<2)
  await seed(750);await wheel(180);await page.waitForTimeout(80);await page.locator('#dialog').evaluate(e=>e.showModal());await finish()
  check('open dialog cancels pending settling',Math.abs(await y()-930)<2)
  await seed(750);await wheel(180,0,{x:120,y:160});await finish()
  check('wheel over nested horizontal scroller does not activate page snap',Math.abs(await y()-1000)>10)
  await seed(930);await wheel(100,0,{x:120,y:270});await finish()
  check('nested vertical scrolling neither moves nor snaps page',Math.abs(await y()-930)<2)
  await seed(930);await wheel(25,120);await finish()
  check('horizontal-dominant wheel yields control',Math.abs(await y()-1000)>10)
  await seed(930);await page.evaluate(()=>{dispatchEvent(new WheelEvent('wheel',{deltaY:20}));scrollTo(0,950)});await finish()
  check('synthetic wheel cannot arm a programmatic scroll',Math.abs(await y()-950)<2)
  await seed(750);await wheel(180);await page.waitForTimeout(80);await page.evaluate(()=>location.hash='no-target');await finish()
  check('hash navigation cancels pending intent',Math.abs(await y()-930)<2)
  await seed(750);await wheel(180);await page.waitForTimeout(80);await page.setViewportSize({width:1440,height:910});await finish()
  check('resize cancels pending intent',Math.abs(await y()-930)<2)
  await page.setViewportSize({width:1440,height:900})
  await seed(750);await page.emulateMedia({reducedMotion:'reduce'});await wheel(180);await finish()
  check('reduced motion never arms snapping',Math.abs(await y()-930)<2)
  await page.emulateMedia({reducedMotion:'no-preference'})
  await seed(750);await page.evaluate(()=>document.body.dataset.homeMotion='false');await wheel(180);await finish()
  check('configured motion off never arms snapping',Math.abs(await y()-930)<2)
  await seed(750,[{key:'invalid',top:1000,from:800,to:980}]);await wheel(180);await finish()
  check('point outside its own stable interval is rejected',Math.abs(await y()-930)<2)
  await page.evaluate(()=>window.dispose());await seed(750);await wheel(180);await finish()
  check('disposed source no longer participates',Math.abs(await y()-930)<2)
  await f.context.close()

  const m=await fixture(true),cdp=await m.context.newCDPSession(m.page)
  const touch=async(type,x=170,yy=650)=>cdp.send('Input.dispatchTouchEvent',{type,touchPoints:type==='touchEnd'?[]:[{x,y:yy}]})
  await m.seed(800)
  await touch('touchStart')
  for(const yy of[635,620,605,590]){await touch('touchMove',170,yy);await m.page.waitForTimeout(80)}
  await m.page.waitForTimeout(180)
  const held=await m.y()
  await m.page.evaluate(y=>window.points=[{key:'touch-forward',top:y+40,from:y-100,to:y+100}],held)
  await m.page.waitForTimeout(400)
  check('active native touch does not start snapping',Math.abs(await m.y()-held)<2)
  await touch('touchEnd');await m.finish()
  check('released touch uses its actual page direction',Math.abs(await m.y()-(held+40))<2,{held,after:await m.y()})
  await m.seed(930)
  await touch('touchStart',80,650);await touch('touchMove',180,645);await touch('touchEnd');await m.finish()
  check('horizontal native touch does not arm page snap',Math.abs(await m.y()-930)<2)
  await m.seed(930)
  await touch('touchStart',120,270);await touch('touchMove',120,210);await touch('touchEnd');await m.finish()
  check('touch within nested scrolling yields page control',Math.abs(await m.y()-930)<2)
  await m.context.close()
  check('no runtime errors',report.errors.length===0)
  report.passed=true
})().catch(error=>{report.passed=false;report.failure=error.message;process.exitCode=1}).finally(async()=>{
  await browser?.close();fs.mkdirSync(path.dirname(output),{recursive:true});fs.writeFileSync(output,JSON.stringify(report,null,2)+'\n')
  console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,failure:report.failure,output},null,2))
})
