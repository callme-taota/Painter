// Real owner UI writes against a disposable candidate only. Never run on :18080.
// BASE_URL=http://192.168.2.209:18081 ACCOUNT_FILE=/tmp/private-accounts.json \
// PLAYWRIGHT_MODULE=/path/to/playwright CHROMIUM_EXECUTABLE=/usr/bin/google-chrome \
// ARTIFACT_DIR=tests/artifacts/v14/business OUTPUT_STATE_FILE=/tmp/private-owner-state.json \
// node tests/browser/v14-friends.cjs
// Default: delete only this run's fixtures and restore the original page settings.
// KEEP_FIXTURES=1 retains four explicitly named V14验收 records for visual review.
const fs=require('fs'),path=require('path'),crypto=require('crypto')
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright')
const base=(process.env.BASE_URL||'http://192.168.2.209:18081').replace(/\/$/,''),location=new URL(base),origin=location.origin
if(location.username||location.password||location.pathname!=='/'||location.search||location.hash)throw Error('BASE_URL must be an origin without credentials, query, or fragment')
if(location.port==='18080'||(location.port!=='18081'&&process.env.ALLOW_CANDIDATE_WRITES!=='I_UNDERSTAND_DISPOSABLE_CANDIDATE'))throw Error('Real fixture writes require disposable candidate port 18081; formal port 18080 is always refused')
if(!process.env.ACCOUNT_FILE)throw Error('Private ACCOUNT_FILE required')
const accounts=JSON.parse(fs.readFileSync(process.env.ACCOUNT_FILE,'utf8')).accounts
if(!Array.isArray(accounts))throw Error('Private account file must contain an accounts array')
const owner=accounts.find(account=>account.role==='owner'&&account.username&&account.password)
if(!owner)throw Error('A local owner account is required')
const secrets=accounts.flatMap(account=>[account.username,account.password]).filter(value=>typeof value==='string'&&value.length>0)
const scrub=value=>secrets.reduce((text,secret)=>text.split(secret).join('[redacted]'),String(value))
const out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v14/business'),keep=process.env.KEEP_FIXTURES==='1'
const runTag='v14-'+Date.now().toString(36)+'-'+crypto.randomBytes(3).toString('hex')
const names={a:'V14验收 · 慢读 '+runTag,b:'V14验收 · 折页 '+runTag,c:'V14验收 · 星图 '+runTag,d:'V14验收 · 稍后再来 '+runTag,e:'V14验收 · 暂时隐藏 '+runTag}
const checks=[],errors=[],writes=[],blocked=[],navigations=[],ownedIDs=new Set()
let browser,context,page,anonymous,nojs,csrf='',initialSettings,originalItems=[],failure=null,failureState,scenario='startup',telemetrySuppressed=0,retainedFixtures=0
function check(name,passed,detail){checks.push({scenario,check:name,passed:!!passed,...(detail===undefined?{}:{detail})});if(!passed)throw Error(name)}
function fingerprint(value){return crypto.createHash('sha256').update(JSON.stringify(value)??'null').digest('hex')}
function fields(row){return {name:row.name,url:row.url,description:row.description,avatar_url:row.avatar_url,sort_order:row.sort_order,visible:row.visible}}
async function jsonResponse(response){const value=await response.json().catch(()=>({}));if(!response.ok())throw Error('Candidate API returned HTTP '+response.status()+': '+(value.error?.message||'request failed'));return value}
async function api(method,route,data,version){
 if(!route.startsWith('/api/v1/studio/friends')&&route!=='/api/v1/studio/friend-settings')throw Error('Business API helper is limited to friend links and their page settings')
 if(!['GET','PUT','DELETE'].includes(method))throw Error('Fixture creation must use the actual UI')
 const response=await context.request.fetch(origin+route,{method,headers:{Origin:origin,'X-CSRF-Token':csrf,...(version?{'If-Match':'"'+version+'"'}:{}),...(data?{'Content-Type':'application/json'}:{})},...(data?{data}:{}),failOnStatusCode:false})
 if(method!=='GET')writes.push({source:'same-session-request',method,path:route,status:response.status()})
 if(response.status()===204)return {}
 return jsonResponse(response)
}
async function routeContext(ctx){
 await ctx.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url()),method=request.method()
  if(url.origin!==origin){
   if(url.hostname==='example.invalid'&&request.isNavigationRequest())return route.fulfill({contentType:'text/html',body:'<!doctype html><html lang="zh-CN"><title>V14验收外链目的地</title><p>V14验收外链目的地</p></html>'})
   if(request.resourceType()==='image')return route.fulfill({status:404,body:''})
   return route.abort('blockedbyclient')
  }
  if(url.pathname==='/api/v1/analytics/events'){telemetrySuppressed++;return route.fulfill({json:{accepted:true}})}
  if(!['GET','HEAD','OPTIONS'].includes(method)&&!['/api/v1/auth/login','/api/v1/studio/friends','/api/v1/studio/friend-settings'].includes(url.pathname)&&!/^\/api\/v1\/studio\/friends\/\d+$/.test(url.pathname)){
   blocked.push({method,path:url.pathname});return route.abort('blockedbyclient')
  }
  return route.continue()
 })
 ctx.on('page',p=>{p.setDefaultTimeout(10000);p.on('pageerror',error=>errors.push({scenario,message:scrub(error.message)}));p.on('framenavigated',frame=>{if(frame===p.mainFrame())navigations.push({scenario,origin:new URL(p.url()).origin,path:new URL(p.url()).pathname})})})
}
async function exportedPrivateState(){
 if(!process.env.OUTPUT_STATE_FILE)return
 const target=path.resolve(process.env.OUTPUT_STATE_FILE)
 if(!target.startsWith('/tmp/'))throw Error('OUTPUT_STATE_FILE must stay under /tmp')
 fs.mkdirSync(path.dirname(target),{recursive:true,mode:0o700})
 const parent=fs.realpathSync(path.dirname(target)),tmp=fs.realpathSync('/tmp')
 if(parent!==tmp&&!parent.startsWith(tmp+path.sep))throw Error('OUTPUT_STATE_FILE parent resolves outside /tmp')
 if(fs.existsSync(target)&&fs.lstatSync(target).isSymbolicLink())throw Error('OUTPUT_STATE_FILE must not be a symbolic link')
 const state=await context.storageState()
 const fd=fs.openSync(target,fs.constants.O_WRONLY|fs.constants.O_CREAT|fs.constants.O_TRUNC|fs.constants.O_NOFOLLOW,0o600)
 try{fs.fchmodSync(fd,0o600);fs.writeFileSync(fd,JSON.stringify(state))}finally{fs.closeSync(fd)}
 check('owner storage state is exported only to a private temporary file',(fs.statSync(target).mode&0o777)===0o600)
}
const row=name=>page.locator('.friends-library-row').filter({has:page.getByRole('heading',{name,exact:true})})
async function loaded(){await page.locator('.friend-manager').waitFor();await page.waitForFunction(()=>{const input=document.querySelector('#friend-name');return input&&!input.disabled})}
async function uiMutation(method,route,click,expected=200){
 const pending=page.waitForResponse(response=>new URL(response.url()).origin===origin&&new URL(response.url()).pathname===route&&response.request().method()===method)
 await click();const response=await pending
 writes.push({source:'UI',method,path:route,status:response.status()})
 check(method+' '+route+' returns expected status',response.status()===expected,{actual:response.status(),expected})
 const value=response.status()===204?{}:await response.json()
 if(method==='POST'&&route==='/api/v1/studio/friends'&&response.status()===201)ownedIDs.add(value.id)
 if(expected<400)await page.waitForFunction(()=>{const form=document.querySelector('.friends-composer fieldset');return form&&!form.disabled&&!!document.querySelector('.notice.success')})
 return value
}
async function edit(name){await row(name).getByRole('button',{name:'编辑 '+name,exact:true}).click();await page.waitForFunction(value=>document.querySelector('#friend-name').value===value,name)}
async function save(id,expected=200){return uiMutation('PUT','/api/v1/studio/friends/'+id,()=>page.locator('.friends-composer .form-actions button.primary').click(),expected)}
async function create(name,order,avatar=''){
 await page.locator('.friends-library-heading button.primary').click()
 await page.locator('#friend-name').fill(name);await page.locator('#friend-url').fill('https://example.invalid/'+runTag+'/'+order)
 await page.locator('#friend-description').fill('V14验收简介 '+order+'\n一处值得慢慢阅读的网站。')
 await page.locator('#friend-avatar').fill(avatar);await page.locator('#friend-order').fill(String(order))
 await page.getByRole('checkbox',{name:'在友链页展示',exact:true}).check()
 return uiMutation('POST','/api/v1/studio/friends',()=>page.locator('.friends-composer .form-actions button.primary').click(),201)
}
async function chooseFilter(label){const trigger=page.getByRole('combobox',{name:'展示状态',exact:true});await trigger.click();await page.getByRole('option',{name:label,exact:true}).click();await page.waitForFunction(()=>document.querySelector('.friends-library-filter [role=combobox]').getAttribute('aria-expanded')==='false')}
async function expectPublic(ctx,expectedNames){
 const p=await ctx.newPage();const response=await p.goto(origin+'/friends')
 check('anonymous directory is already server rendered',response.status()===200&&(await response.text()).includes('friends-list'))
 const entries=p.locator('.friend-entry').filter({has:p.locator('.friend-copy h2').filter({hasText:runTag})})
 const actual=await entries.locator('h2').allTextContents()
 check('anonymous and no-JS links retain visibility and server order',JSON.stringify(actual)===JSON.stringify(expectedNames),{expectedCount:expectedNames.length,actualCount:actual.length})
 check('anonymous directory has no horizontal overflow',await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1))
 return {page:p,entries}
}
async function entryPoints(enabled){
 for(const route of ['/','/about']){
  const response=await anonymous.request.get(origin+route),text=await response.text()
  check(route+' discovery entry follows the page switch',response.status()===200&&text.includes('href="/friends"')===enabled)
 }
 const response=await anonymous.request.get(origin+'/sitemap.xml'),text=await response.text()
 check('sitemap follows the page switch',response.status()===200&&text.includes('<loc>'+origin+'/friends</loc>')===enabled)
}
async function pageSettingsSave(expected=200){return uiMutation('PUT','/api/v1/studio/friend-settings',()=>page.locator('.friends-page-settings button.primary').click(),expected)}
async function run(){
 check('disposable candidate is ready',(await context.request.get(origin+'/readyz')).ok())
 await page.goto(origin+'/login');await page.getByLabel('用户名',{exact:true}).fill(owner.username);await page.getByLabel('密码',{exact:true}).fill(owner.password)
 await page.getByRole('button',{name:'登录',exact:true}).click();await page.waitForURL('**/me')
 const session=await jsonResponse(await context.request.get(origin+'/api/v1/session'));csrf=session.csrf_token
 secrets.push(csrf,...(await context.cookies()).map(cookie=>cookie.value));check('normal local account login has owner permissions',session.authenticated&&session.user?.role==='owner')
 initialSettings=await api('GET','/api/v1/studio/friend-settings');originalItems=(await api('GET','/api/v1/studio/friends')).items
 await page.goto(origin+'/studio/friends');await loaded()
 scenario='create-edit-persist'
 const a=await create(names.a,10,'https://example.invalid/'+runTag+'/broken-avatar.png'),b=await create(names.b,20,origin+'/assets/images/painter-logo.png'),c=await create(names.c,30)
 check('three fixtures are created through actual UI',ownedIDs.size===3)
 await edit(names.a);await page.locator('#friend-description').fill('V14验收修改后的简介\n第二行仍然保留。');await page.locator('#friend-order').fill('40')
 await page.waitForFunction(()=>{const image=document.querySelector('.friends-preview img');return image&&image.hidden})
 check('broken editor avatar leaves a visible initial fallback',await page.locator('.friends-preview .friend-avatar>span').isVisible())
 await save(a.id)
 await edit(names.b);await page.locator('#friend-order').fill('-20');await save(b.id)
 await edit(names.c);await page.getByRole('checkbox',{name:'在友链页展示',exact:true}).uncheck();await save(c.id)
 await page.locator('#friend-search').fill(runTag);await chooseFilter('已隐藏')
 check('custom hidden-state filter shows only the hidden test record',await page.locator('.friends-library-row').count()===1&&await row(names.c).isVisible()&&await page.locator('select').count()===0)
 await chooseFilter('展示中');check('visible-state filter excludes hidden record',await page.locator('.friends-library-row').count()===2&&await row(names.c).count()===0)
 await chooseFilter('全部友链');await page.locator('#friend-search').fill('V14验收修改后的简介')
 check('search matches edited descriptions',await page.locator('.friends-library-row').count()===1&&await row(names.a).isVisible())
 await page.locator('#friend-search').fill('/'+runTag+'/20');check('search matches saved destination URLs',await page.locator('.friends-library-row').count()===1&&await row(names.b).isVisible())
 await page.locator('#friend-search').fill('V14验收不可能匹配-'+runTag)
 check('empty filter explains the result and offers reset',await page.getByRole('heading',{name:'没有匹配的友链',exact:true}).isVisible())
 await page.getByRole('button',{name:'清除筛选',exact:true}).click();await page.locator('#friend-search').fill(runTag)
 // The page may have started disabled. Enable it through its actual settings UI.
 await page.getByRole('tab',{name:'页面设置',exact:true}).click();await page.getByRole('checkbox',{name:'开启友链页面',exact:true}).check();await pageSettingsSave()
 await page.getByRole('tab',{name:'友链列表',exact:true}).click()
 let publicState=await expectPublic(anonymous,[names.b,names.a]);await publicState.page.close()
 await edit(names.c);await page.getByRole('checkbox',{name:'在友链页展示',exact:true}).check();await page.locator('#friend-order').fill('0');await save(c.id)
 await page.reload();await loaded();await page.locator('#friend-search').fill(runTag)
 check('saved sorting and visibility persist after a full refresh',JSON.stringify(await page.locator('.friends-library-row h2').allTextContents())===JSON.stringify([names.b,names.c,names.a]))
 await edit(names.a);check('multi-line edited content persists after refresh',await page.locator('#friend-description').inputValue()==='V14验收修改后的简介\n第二行仍然保留。')
 scenario='dirty-switch-and-delete'
 await page.locator('#friend-name').fill('V14验收未保存的本地输入 '+runTag)
 await row(names.b).getByRole('button',{name:'编辑 '+names.b,exact:true}).click();const dialog=page.locator('dialog.studio-confirm')
 await dialog.getByRole('button',{name:'取消',exact:true}).click();check('canceling a dirty selection switch preserves input',await page.locator('#friend-name').inputValue()==='V14验收未保存的本地输入 '+runTag)
 await row(names.b).getByRole('button',{name:'编辑 '+names.b,exact:true}).click();await dialog.getByRole('button',{name:'放弃修改',exact:true}).click();await page.waitForFunction(value=>document.querySelector('#friend-name').value===value,names.b)
 check('explicit dirty-discard switches to the selected saved record',await page.locator('#friend-name').inputValue()===names.b)
 const d=await create(names.d,90)
 await row(names.d).getByRole('button',{name:'删除 '+names.d,exact:true}).click();await dialog.getByRole('button',{name:'取消',exact:true}).click()
 check('canceling delete keeps the persisted record',(await api('GET','/api/v1/studio/friends')).items.some(item=>item.id===d.id))
 await row(names.d).getByRole('button',{name:'删除 '+names.d,exact:true}).click()
 await uiMutation('DELETE','/api/v1/studio/friends/'+d.id,()=>dialog.getByRole('button',{name:'删除友链',exact:true}).click(),204)
 check('confirmed delete removes only the selected record',!(await api('GET','/api/v1/studio/friends')).items.some(item=>item.id===d.id)&&await row(names.d).count()===0)
 scenario='link-version-conflict'
 await edit(names.a);const localName='V14验收冲突时的本地输入 '+runTag;await page.locator('#friend-name').fill(localName)
 const serverA=(await api('GET','/api/v1/studio/friends')).items.find(item=>item.id===a.id),serverName='V14验收服务器最新版本 '+runTag
 await api('PUT','/api/v1/studio/friends/'+a.id,{...fields(serverA),name:serverName,description:'V14验收服务器的新简介'},serverA.version)
 await save(a.id,409);await page.getByRole('button',{name:'载入最新版本',exact:true}).waitFor()
 check('a real CAS conflict preserves local inputs and blocks stale saving',await page.locator('#friend-name').inputValue()===localName&&await page.locator('.friends-composer button.primary').isDisabled())
 await page.getByRole('button',{name:'载入最新版本',exact:true}).click();await dialog.getByRole('button',{name:'取消',exact:true}).click()
 check('canceling latest reload keeps the local conflict copy',await page.locator('#friend-name').inputValue()===localName)
 await page.getByRole('button',{name:'载入最新版本',exact:true}).click();await dialog.getByRole('button',{name:'载入最新',exact:true}).click()
 await page.waitForFunction(value=>document.querySelector('#friend-name').value===value,serverName)
 check('explicit latest reload resolves the actual server version',await page.locator('#friend-description').inputValue()==='V14验收服务器的新简介'&&await page.locator('.friends-composer button.primary').isEnabled())
 await page.locator('#friend-name').fill(names.a);await page.locator('#friend-description').fill('V14验收修改后的简介\n第二行仍然保留。');await save(a.id)
 scenario='page-settings-and-conflict'
 await page.getByRole('tab',{name:'页面设置',exact:true}).click()
 await page.locator('#friends-page-title').fill('V14验收本地设置 '+runTag)
 await page.getByRole('tab',{name:'友链列表',exact:true}).click();await page.getByRole('tab',{name:'页面设置',exact:true}).click()
 check('switching manager tabs retains unsaved page settings',await page.locator('#friends-page-title').inputValue()==='V14验收本地设置 '+runTag)
 const serverSettings=await api('GET','/api/v1/studio/friend-settings')
 await api('PUT','/api/v1/studio/friend-settings',{enabled:true,title:'V14验收服务器页面设置 '+runTag,description:'服务器说明'},serverSettings.version)
 await pageSettingsSave(409);await page.getByRole('button',{name:'载入最新设置',exact:true}).waitFor()
 check('settings CAS conflict preserves unsaved title',await page.locator('#friends-page-title').inputValue()==='V14验收本地设置 '+runTag&&await page.locator('.friends-page-settings button.primary').isDisabled())
 await page.getByRole('button',{name:'载入最新设置',exact:true}).click();await dialog.getByRole('button',{name:'载入最新',exact:true}).click()
 await page.waitForFunction(value=>document.querySelector('#friends-page-title').value===value,'V14验收服务器页面设置 '+runTag)
 const finalTitle='V14验收 · 网上邻居',finalDescription='从这里，走向朋友的世界。\n每个链接，都是另一扇窗。'
 await page.locator('#friends-page-title').fill(finalTitle);await page.locator('#friends-page-description').fill(finalDescription);await pageSettingsSave()
 publicState=await expectPublic(anonymous,[names.b,names.c,names.a]);check('public heading and multi-line page description use saved settings',await publicState.page.locator('#friends-title').innerText()===finalTitle&&(await publicState.page.locator('.friends-heading p').textContent())===finalDescription)
 const badAvatar=publicState.entries.filter({has:publicState.page.getByRole('heading',{name:names.a,exact:true})}).locator('img')
 await badAvatar.waitFor({state:'attached'});await badAvatar.locator('..').scrollIntoViewIfNeeded();await badAvatar.evaluate(image=>new Promise(resolve=>{if(image.complete)return resolve();image.addEventListener('error',resolve,{once:true})}))
 check('public broken-avatar fallback stays visible',await badAvatar.evaluate(image=>image.hidden)&&await badAvatar.locator('..').locator('span').isVisible())
 const popupPromise=publicState.page.waitForEvent('popup');await publicState.entries.first().click();const popup=await popupPromise;await popup.waitForURL('https://example.invalid/**');await popup.waitForLoadState()
 check('native public external link opens a separate tab',new URL(popup.url()).hostname==='example.invalid'&&await popup.evaluate(()=>window.opener===null))
 await popup.close();await publicState.page.close();await entryPoints(true)
 await page.getByRole('checkbox',{name:'开启友链页面',exact:true}).uncheck();await pageSettingsSave();await entryPoints(false)
 const disabled=await anonymous.request.get(origin+'/friends');check('disabled public directory returns 404 and hides its content',disabled.status()===404&&!(await disabled.text()).includes(runTag))
 check('closing the page preserves all three friend records',(await api('GET','/api/v1/studio/friends')).items.filter(item=>ownedIDs.has(item.id)).length===3)
 await page.getByRole('checkbox',{name:'开启友链页面',exact:true}).check();await pageSettingsSave();await entryPoints(true)
 await page.reload();await loaded();await page.getByRole('tab',{name:'页面设置',exact:true}).click()
 check('page settings persist after refresh',await page.locator('#friends-page-title').inputValue()===finalTitle&&await page.locator('#friends-page-description').inputValue()===finalDescription&&await page.getByRole('checkbox',{name:'开启友链页面',exact:true}).isChecked())
 const native=await expectPublic(nojs,[names.b,names.c,names.a]);check('no-JS public entries have real destination anchors and safe new-tab attributes',await native.entries.evaluateAll(entries=>entries.every(entry=>entry.tagName==='A'&&entry.href.startsWith('https://example.invalid/')&&entry.target==='_blank'&&entry.relList.contains('noopener')&&entry.relList.contains('noreferrer'))));await native.page.close()
 await page.getByRole('tab',{name:'友链列表',exact:true}).click()
 const hidden=await create(names.e,70);await page.getByRole('checkbox',{name:'在友链页展示',exact:true}).uncheck();await save(hidden.id)
 const publicWithHidden=await anonymous.request.get(origin+'/friends')
 check('a clearly named hidden fixture is retained only in the manager',publicWithHidden.status()===200&&!(await publicWithHidden.text()).includes(names.e))
 await page.setViewportSize({width:390,height:900});await page.locator('.studio-mobile-bar').waitFor();await page.evaluate(()=>new Promise(r=>requestAnimationFrame(r)))
 check('mobile manager exposes editor inputs without horizontal overflow',await page.locator('#friend-name').isVisible()&&await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1))
 await exportedPrivateState()
}
async function cleanup(){
 if(!context||!csrf||!initialSettings)return
 scenario='cleanup'
 let current=(await api('GET','/api/v1/studio/friends')).items
 const ours=current.filter(item=>ownedIDs.has(item.id)||(item.name.includes(runTag)&&item.url.startsWith('https://example.invalid/'+runTag+'/')))
 const retain=keep&&!failure
 if(!retain){for(const item of ours)await api('DELETE','/api/v1/studio/friends/'+item.id,undefined,item.version)}
 const settings=await api('GET','/api/v1/studio/friend-settings')
 if(settings.enabled!==initialSettings.enabled||settings.title!==initialSettings.title||settings.description!==initialSettings.description)await api('PUT','/api/v1/studio/friend-settings',{enabled:initialSettings.enabled,title:initialSettings.title,description:initialSettings.description},settings.version)
 current=(await api('GET','/api/v1/studio/friends')).items
 const originalByID=new Map(current.map(item=>[item.id,item]))
 check('all pre-existing friend records remain byte-for-byte unchanged',originalItems.every(item=>fingerprint(item)===fingerprint(originalByID.get(item.id))))
 retainedFixtures=current.filter(item=>ownedIDs.has(item.id)||item.name.includes(runTag)).length
 check(retain?'four clearly named review fixtures are retained':'all fixtures created by this run are removed',retainedFixtures===(retain?4:0))
 const restored=await api('GET','/api/v1/studio/friend-settings')
 check('original page settings are restored',restored.enabled===initialSettings.enabled&&restored.title===initialSettings.title&&restored.description===initialSettings.description)
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 try{
  browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--disable-dev-shm-usage','--enable-unsafe-swiftshader']})
  context=await browser.newContext({viewport:{width:1440,height:1000},serviceWorkers:'block'});await routeContext(context);page=await context.newPage()
  anonymous=await browser.newContext({viewport:{width:390,height:900},serviceWorkers:'block'});await routeContext(anonymous)
  nojs=await browser.newContext({viewport:{width:390,height:900},javaScriptEnabled:false,serviceWorkers:'block'});await routeContext(nojs)
  await run()
 }catch(error){failure=scrub(error.stack||error);failureState=await page?.evaluate(()=>({path:location.pathname,kind:document.body.dataset.pageKind,manager:!!document.querySelector('.friend-manager'),tabs:[...document.querySelectorAll('.friends-manager-bar [role=tab]')].map(element=>({text:element.textContent,hidden:!element.checkVisibility(),inert:!!element.closest('[inert]'),ariaHidden:!!element.closest('[aria-hidden=true]')})),dialogs:[...document.querySelectorAll('dialog[open]')].map(element=>({className:element.className,title:element.querySelector('h2')?.textContent}))})).catch(()=>({unavailable:true}))}
 finally{
  try{await cleanup()}catch(error){failure=(failure?failure+'\n':'')+'Cleanup: '+scrub(error.stack||error)}
  try{check('no unrelated business writes were attempted',blocked.length===0,blocked);check('no browser runtime errors occurred',errors.length===0,errors)}catch(error){failure=(failure?failure+'\n':'')+scrub(error.message)}
  const report={origin,runTag,keepFixtures:keep,passed:!failure&&checks.every(item=>item.passed),checks,writes,blocked,errors,navigations,telemetrySuppressed,...(failure?{failure,failureState}:{})}
  fs.writeFileSync(path.join(out,'report.json'),scrub(JSON.stringify(report,null,2)))
  console.log(JSON.stringify({checks:checks.length,allPassed:report.passed,fixturesRetained:retainedFixtures,report:path.join(out,'report.json')}))
  if(failure){console.error(scrub(failure));process.exitCode=1}
  await browser?.close()
 }
})().catch(error=>{console.error(scrub(error.stack||error));process.exitCode=1})
