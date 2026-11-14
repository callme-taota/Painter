// Isolated behavior checks: no server, business data, screenshots, or writes.
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const {chromium} = require('/tmp/painter-browser/node_modules/playwright')
const ts = require('../../web/frontend/node_modules/typescript')

;(async () => {
  const source = fs.readFileSync(path.join(__dirname, '../../web/frontend/src/homeSnap.ts'), 'utf8')
  const code = ts.transpileModule(source, {compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}}).outputText
  const browser = await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']})
  const results = []
  try {
    const page = await browser.newPage({viewport:{width:390,height:844},hasTouch:true,isMobile:true})
    await page.setContent('<!doctype html><meta name="viewport" content="width=device-width"><body data-page-kind="home"><style>body{margin:0;height:6000px}#nested{position:absolute;top:1150px;width:200px;height:100px;overflow-x:auto}#nested>div{width:800px;height:100px}button{position:absolute;top:1180px;left:230px}</style><main id="scene"></main><div id="nested"><div></div></div><button>Action</button>')
    await page.addScriptTag({type:'module',content:code+'\nregisterHomeSnapSource(document.querySelector("#scene"),()=>[{key:"calendar",top:1000,from:800,to:1200}]);initHomeSnap();window.snapReady=true;'})
    await page.waitForFunction(() => window.snapReady)
    await page.waitForTimeout(1100)
    const y = () => page.evaluate(() => scrollY)
    const go = async value => { await page.evaluate(y => scrollTo({top:y,behavior:'instant'}),value); await page.waitForTimeout(680) }
    const check = (name, value) => { assert.ok(value,name); results.push(name) }

    await go(1100)
    check('resting stable scene settles to its published anchor',Math.abs(await y()-1000)<2)
    await go(1400)
    check('transition range remains at the visitor scroll position',Math.abs(await y()-1400)<2)
    await page.evaluate(() => scrollTo(0,1150))
    await page.waitForTimeout(235)
    await page.mouse.move(340,600)
    await page.mouse.wheel(0,500)
    await page.waitForTimeout(650)
    check('new wheel input interrupts a running settle without locking scroll',await y()>1200)

    await page.evaluate(() => scrollTo(0,1000))
    await page.waitForTimeout(700)
    await page.locator('#nested').dispatchEvent('wheel',{deltaX:60,deltaY:0,bubbles:true})
    await go(1080)
    check('nested horizontal scrolling suppresses page settling',Math.abs(await y()-1080)<2)
    await page.waitForTimeout(250)
    await page.locator('button').focus()
    await go(1120)
    check('focused native controls retain the visitor position',Math.abs(await y()-1120)<2)
    await page.locator('button').evaluate(e=>e.blur())
    await page.waitForTimeout(600)

    const touch = async (kind,x,y) => page.evaluate(({kind,x,y}) => {
      const event=new Event(kind,{bubbles:true});Object.defineProperty(event,'touches',{value:kind==='touchend'?[]:[{clientX:x,clientY:y}]});document.body.dispatchEvent(event)
    },{kind,x,y})
    await touch('touchstart',300,600)
    await go(1090)
    check('an active touch never starts page settling',Math.abs(await y()-1090)<2)
    await touch('touchend',300,600)
    await page.waitForTimeout(680)
    check('released vertical touch can settle gently',Math.abs(await y()-1000)<2)
    await touch('touchstart',200,500)
    await touch('touchmove',310,505)
    await page.evaluate(()=>scrollTo(0,1100))
    await touch('touchend',310,505)
    await page.waitForTimeout(680)
    check('horizontal touch leaves page position alone',Math.abs(await y()-1100)<2)

    await page.emulateMedia({reducedMotion:'reduce'})
    await go(1140)
    check('reduced motion removes automatic spatial settling',Math.abs(await y()-1140)<2)
    await page.emulateMedia({reducedMotion:'no-preference'})
    await page.evaluate(()=>document.body.dataset.homeMotion='false')
    await go(1160)
    check('configured motion off removes automatic settling',Math.abs(await y()-1160)<2)
    console.log(JSON.stringify({passed:results.length,checks:results},null,2))
  } finally { await browser.close() }
})().catch(error => { console.error(error); process.exitCode=1 })
