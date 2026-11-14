// Read-only layout regression against an existing draft on an isolated clone.
// No screenshots or fixtures are created. Accidental business mutations are aborted.
// BASE_URL=http://192.168.2.209:18081 OWNER_STATE_FILE=/private/owner-state.json \
// PLAYWRIGHT_MODULE=/path/to/playwright CHROMIUM_EXECUTABLE=/usr/bin/google-chrome \
// node tests/browser/v10-editor-layout.cjs
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright')
const fs=require('fs'),path=require('path'),crypto=require('crypto')
const base=(process.env.BASE_URL||'http://192.168.2.209:18081').replace(/\/$/,''),origin=new URL(base).origin
const stateFile=process.env.OWNER_STATE_FILE||'/tmp/painter-next-v9-owner-state.json'
const out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v10'),postId=process.env.POST_ID||'1'
const storageState=JSON.parse(fs.readFileSync(stateFile,'utf8'))
const secrets=[...storageState.cookies.map(c=>c.value),...storageState.origins.flatMap(o=>(o.localStorage||[]).map(x=>x.value))].filter(s=>s&&s.length>=8)
const scrub=value=>secrets.reduce((s,secret)=>s.split(secret).join('[redacted]'),String(value))
const checks=[],metrics=[],errors=[],blockedMutations=[],previewRequests=[],serverFingerprints=[]
let browser,suppressedTelemetry=0,currentScenario='startup'
const canonical=value=>Array.isArray(value)?value.map(canonical):value&&typeof value==='object'?Object.fromEntries(Object.keys(value).sort().map(k=>[k,canonical(value[k])])):value
const fingerprint=value=>crypto.createHash('sha256').update(JSON.stringify(canonical(value))).digest('hex')
const check=(name,passed,detail)=>checks.push({scenario:currentScenario,check:name,passed:!!passed,...(detail===undefined?{}:{detail})})
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms))
async function ready(){
 const deadline=Date.now()+Number(process.env.READINESS_TIMEOUT_MS||60000)
 while(Date.now()<deadline){try{const response=await fetch(base+'/readyz',{signal:AbortSignal.timeout(2000)});if(response.ok)return}catch{}await pause(1000)}
 throw Error('Candidate did not become ready within the readiness timeout')
}
async function serverDraft(context){
 const response=await context.request.get(base+'/api/v1/studio/posts/'+encodeURIComponent(postId))
 if(!response.ok())throw Error('Read-only draft GET failed with HTTP '+response.status())
 return response.json()
}
async function setup(width,height=1000){
 const context=await browser.newContext({viewport:{width,height},storageState,colorScheme:'light',serviceWorkers:'block'})
 await context.addInitScript(()=>localStorage.setItem('painter-theme','light'))
 await context.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url()),method=request.method()
  if(!['POST','PUT','PATCH','DELETE'].includes(method))return route.continue()
  if(url.origin===origin&&method==='POST'&&['/api/v1/editor/cover-preview','/api/v1/editor/preview'].includes(url.pathname)){
   previewRequests.push({method,path:url.pathname});return route.continue()
  }
  if(url.origin===origin&&method==='POST'&&url.pathname==='/api/v1/analytics/events'){
   suppressedTelemetry++;return route.fulfill({json:{accepted:true}})
  }
  blockedMutations.push({scenario:currentScenario,method,path:url.pathname})
  return route.abort('blockedbyclient')
 })
 const page=await context.newPage();page.setDefaultTimeout(5000)
 page.on('pageerror',error=>errors.push({scenario:currentScenario,message:scrub(error.message)}))
 const before=fingerprint(await serverDraft(context))
 await page.goto(base+'/studio/posts/'+encodeURIComponent(postId))
 await page.getByRole('textbox',{name:'文章标题',exact:true}).waitFor()
 await page.getByRole('textbox',{name:'文章标题',exact:true}).waitFor({state:'visible'})
 await page.waitForFunction(()=>{const input=document.querySelector('.editor-title');return input&&!input.disabled})
 await page.getByRole('textbox',{name:'文档正文',exact:true}).waitFor({state:'visible'})
 await page.waitForFunction(()=>document.fonts.status==='loaded')
 return {context,page,before}
}
async function finish(test){
 await pause(1300) // Includes the editor's one-second autosave debounce.
 const after=fingerprint(await serverDraft(test.context))
 serverFingerprints.push({scenario:currentScenario,before:test.before,after,unchanged:test.before===after})
 check('existing server draft content, metadata and versions stay unchanged',test.before===after)
 await test.context.close()
}
async function metadata(page){
 return page.locator('.editor-page').evaluate(root=>({
  title:root.querySelector('.editor-title')?.value,
  summary:root.querySelector('.editor-metadata textarea')?.value,
  tags:Array.from(root.querySelectorAll('.selected-tag')).map(e=>e.getAttribute('aria-label')).sort(),
  dates:Array.from(root.querySelectorAll('.date-display-options input')).map(e=>e.checked),
  cover:Array.from(root.querySelectorAll('.cover-editor .select-trigger')).map(e=>[e.getAttribute('aria-label'),e.textContent.trim()]),
  propertySummary:root.querySelector('.property-summary')?.textContent,
  saveState:root.querySelector('.save-state')?.textContent,
  document:root.querySelector('.document-prose')?.textContent
 }))
}
async function layout(page,label){
 const value=await page.evaluate(()=>{
  const rect=selector=>{const element=document.querySelector(selector),r=element.getBoundingClientRect(),s=getComputedStyle(element);return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height,paddingLeft:parseFloat(s.paddingLeft)||0,textLeft:r.left+(parseFloat(s.paddingLeft)||0)}}
  return {viewport:{width:innerWidth,height:innerHeight},workspace:rect('.editor-workspace'),title:rect('.editor-title'),titleArea:rect('.editor-title-area'),body:rect('.document-prose'),writing:rect('.editor-writing'),properties:rect('.editor-properties'),propertiesOpen:document.querySelector('.editor-properties').open,gridColumns:getComputedStyle(document.querySelector('.editor-workspace')).gridTemplateColumns,scrollWidth:document.documentElement.scrollWidth,horizontalOverflow:document.documentElement.scrollWidth>innerWidth+1}
 })
 metrics.push({scenario:currentScenario,state:label,...value})
 check(label+': title and body share the text edge within two CSS pixels',Math.abs(value.title.textLeft-value.body.textLeft)<=2,{titleTextLeft:value.title.textLeft,bodyTextLeft:value.body.textLeft,delta:Math.abs(value.title.textLeft-value.body.textLeft)})
 check(label+': no horizontal page overflow',!value.horizontalOverflow,{viewportWidth:value.viewport.width,scrollWidth:value.scrollWidth})
 return value
}
async function targets(page,label){
 const value=await page.locator('.editor-page').evaluate(root=>Array.from(root.querySelectorAll('button,.button,summary')).map(element=>{
  const r=element.getBoundingClientRect(),s=getComputedStyle(element)
  const details=element.closest('details'),insideClosedDetails=details&&!details.open&&!details.querySelector('summary')?.contains(element)
  return {label:element.getAttribute('aria-label')||element.getAttribute('title')||element.textContent.trim().slice(0,60),width:r.width,height:r.height,visible:!insideClosedDetails&&element.checkVisibility({checkVisibilityCSS:true})&&!!element.getClientRects().length&&r.width>0&&r.height>0&&s.visibility!=='hidden'}
 }).filter(target=>target.visible))
 const small=value.filter(target=>target.width<43.99||target.height<43.99)
 check(label+': every rendered editor command and metadata summary has a 44 by 44 CSS pixel target',small.length===0,{measured:value.length,undersized:small})
}
async function disclosure(page){
 const details=page.locator('.editor-properties'),summary=details.locator('summary'),before=await metadata(page),wasOpen=await details.getAttribute('open')!==null
 await summary.focus();await page.keyboard.press('Enter')
 await page.waitForFunction(open=>document.querySelector('.editor-properties').open!==open,wasOpen)
 check('metadata summary toggles with Enter and retains keyboard focus',await summary.evaluate(e=>e===document.activeElement))
 await page.keyboard.press('Space')
 await page.waitForFunction(open=>document.querySelector('.editor-properties').open===open,wasOpen)
 check('metadata summary toggles back with Space without changing metadata',fingerprint(before)===fingerprint(await metadata(page)))
 if(!wasOpen)await summary.click()
 await page.waitForFunction(()=>document.querySelector('.editor-properties').open)
 await targets(page,'open metadata')
 const tag=page.getByRole('combobox',{name:'标签',exact:true})
 await tag.fill('V10-layout-query-no-selection')
 check('typing in the tag picker opens a read-only query',await tag.getAttribute('aria-expanded')==='true'&&await tag.inputValue()==='V10-layout-query-no-selection')
 await tag.press('Escape')
 check('Escape closes and clears the tag query without selecting a tag',await tag.getAttribute('aria-expanded')==='false'&&await tag.inputValue()===''&&fingerprint(before)===fingerprint(await metadata(page)))
 const select=page.getByRole('combobox',{name:'封面来源',exact:true})
 await select.focus();await select.press('ArrowDown')
 const selectedText=await select.innerText(),list=page.getByRole('listbox',{name:'封面来源',exact:true})
 await list.waitFor()
 const active=await select.getAttribute('aria-activedescendant')
 await select.press('ArrowDown')
 check('custom select opens and moves active option with keyboard',await select.getAttribute('aria-expanded')==='true'&&await select.getAttribute('aria-activedescendant')!==active)
 await select.press('End')
 check('custom select End reaches the last available option',await list.getByRole('option').last().getAttribute('data-active')==='true')
 await select.press('Home')
 check('custom select Home reaches the first available option',await list.getByRole('option').first().getAttribute('data-active')==='true')
 await select.press('Escape')
 check('custom select Escape closes, preserves selection and restores focus',await select.getAttribute('aria-expanded')==='false'&&await select.innerText()===selectedText&&await select.evaluate(e=>e===document.activeElement))
 await select.press('Enter');await list.waitFor();await select.press('Enter')
 check('custom select can confirm its current option with Enter without metadata changes',await select.getAttribute('aria-expanded')==='false'&&fingerprint(before)===fingerprint(await metadata(page)))
 if(!wasOpen)await summary.click()
 await pause(1300)
 check('metadata disclosure, tag search and select navigation do not change draft fields or save state',fingerprint(before)===fingerprint(await metadata(page)))
}
async function sourceModes(page){
 const before=await metadata(page)
 await page.getByRole('button',{name:'Markdown 源码',exact:true}).click()
 await page.locator('.cm-content').waitFor({state:'visible'})
 const value=await page.locator('.editor-workspace').evaluate(workspace=>{const r=workspace.getBoundingClientRect(),writing=workspace.querySelector('.editor-writing').getBoundingClientRect();return {workspaceWidth:r.width,writingWidth:writing.width,writingLeft:writing.left,workspaceLeft:r.left,propertiesOpen:workspace.querySelector('.editor-properties').open}})
 check('Markdown source closes article properties and uses the full workspace width',!value.propertiesOpen&&Math.abs(value.workspaceWidth-value.writingWidth)<=2&&Math.abs(value.workspaceLeft-value.writingLeft)<=2,value)
 for(const [label,mode]of[['编辑','edit'],['预览','preview'],['分屏','split']]){
  await page.getByRole('button',{name:label,exact:true}).click()
  const panes=page.locator('.editor-panes'),source=page.locator('.editor-source'),preview=page.locator('.editor-preview')
  const sourceVisible=await source.isVisible(),previewVisible=await preview.isVisible()
  check('source '+mode+' view switches to the requested usable panes',await panes.getAttribute('data-view')===mode&&sourceVisible===(mode!=='preview')&&previewVisible===(mode!=='edit'))
  const sizes=await panes.evaluate(root=>Array.from(root.children).filter(e=>getComputedStyle(e).display!=='none').map(e=>{const r=e.getBoundingClientRect();return {className:e.className,width:r.width,height:r.height}}))
  check('source '+mode+' visible panes have usable width and height',sizes.length>0&&sizes.every(s=>s.width>=140&&s.height>=300),sizes)
  if(sourceVisible)check('source '+mode+' editor accepts focus without editing',await page.locator('.cm-content').evaluate(e=>{e.focus();return e.isContentEditable&&e===document.activeElement}))
  if(previewVisible)check('source '+mode+' renders a completed read-only preview',await preview.locator(':scope > div').evaluate(e=>!!e.innerHTML.trim()))
  check('source '+mode+' has no horizontal page overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1))
  await targets(page,'source '+mode)
 }
 await page.getByRole('button',{name:'文档',exact:true}).click()
 await page.getByRole('textbox',{name:'文档正文',exact:true}).waitFor({state:'visible'})
 await pause(1300)
 check('all view switches preserve draft fields, rendered document and save state',fingerprint(before)===fingerprint(await metadata(page)))
}
async function scenario(name,run){
 currentScenario=name
 try{await run()}catch(error){check('scenario completes',false,{message:scrub(error.message)})}
}
async function viewportScenario(width,height){
 let test
 try{
  test=await setup(width,height);const {page}=test,value=await layout(page,'fresh document')
  check('document mode is the default',await page.getByRole('button',{name:'文档',exact:true}).getAttribute('aria-pressed')==='true')
  if(width>=1360)check('desktop article inspector opens beside the writing surface',value.propertiesOpen&&value.properties.left>=value.writing.right+10&&Math.abs(value.properties.top-value.titleArea.top)<=2&&value.properties.width>=240)
  else check('narrow viewport starts with collapsed metadata in a single column',!value.propertiesOpen&&Math.abs(value.titleArea.width-value.writing.width)<=2&&Math.abs(value.properties.left-value.titleArea.left)<=2&&value.properties.top>=value.titleArea.bottom-2&&value.writing.top>=value.properties.bottom-2)
  await targets(page,'fresh document')
  await disclosure(page)
  await sourceModes(page)
  await layout(page,'document after source views')
 }finally{if(test)await finish(test)}
}
async function resizeScenario(){
 let test
 try{
  test=await setup(1920,1080);const {page}=test,before=await metadata(page)
  for(const width of[900,390,1440,1920]){
   await page.setViewportSize({width,height:width===390?900:1000})
   await page.waitForFunction(w=>document.querySelector('.editor-properties').open===(w>=1360),width)
   const value=await layout(page,'resize to '+width)
   check('resize to '+width+' follows the inspector breakpoint',width>=1360?value.properties.left>=value.writing.right+10:Math.abs(value.properties.left-value.titleArea.left)<=2)
   check('resize to '+width+' preserves all current article settings',fingerprint(before)===fingerprint(await metadata(page)))
  }
  await page.getByRole('button',{name:'Markdown 源码',exact:true}).click()
  for(const width of[390,1440]){
   await page.setViewportSize({width,height:1000})
   check('source resize to '+width+' keeps metadata closed and current settings',!await page.locator('.editor-properties').evaluate(e=>e.open)&&fingerprint(before)===fingerprint(await metadata(page)))
   check('source resize to '+width+' has no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1))
  }
  await page.getByRole('button',{name:'文档',exact:true}).click()
  await page.waitForFunction(()=>document.querySelector('.editor-properties').open)
  check('returning to desktop document reopens the inspector without metadata changes',fingerprint(before)===fingerprint(await metadata(page)))
 }finally{if(test)await finish(test)}
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true});await ready()
 browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']})
 for(const [width,height]of[[1440,1000],[1920,1080],[900,1000],[390,900]])await scenario('viewport '+width,()=>viewportScenario(width,height))
 await scenario('responsive settings',resizeScenario)
 currentScenario='whole run'
 check('zero browser runtime errors',errors.length===0,errors)
 check('zero attempted business mutations',blockedMutations.length===0,blockedMutations)
 check('read-only render endpoints were exercised',previewRequests.some(r=>r.path==='/api/v1/editor/preview'))
})().catch(error=>check('test harness completes',false,{message:scrub(error.message)})).finally(async()=>{
 await browser?.close()
 const passed=checks.length>0&&checks.every(c=>c.passed)
 fs.mkdirSync(out,{recursive:true})
 fs.writeFileSync(path.join(out,'layout.json'),JSON.stringify({origin,post_id:postId,generated_at:new Date().toISOString(),read_only:true,screenshots:false,all_passed:passed,checks,metrics,server_fingerprints:serverFingerprints,page_errors:errors,blocked_business_mutations:blockedMutations,real_business_writes:0,read_only_preview_posts:previewRequests.length,suppressed_telemetry_posts:suppressedTelemetry},null,2)+'\n')
 console.log(JSON.stringify({checks:checks.length,passed:checks.filter(c=>c.passed).length,failed:checks.filter(c=>!c.passed).map(c=>({scenario:c.scenario,check:c.check,detail:c.detail})),pageErrors:errors.length,blockedBusinessMutations:blockedMutations.length,realBusinessWrites:0,artifact:path.relative(process.cwd(),path.join(out,'layout.json'))}))
 if(!passed)process.exitCode=1
})
