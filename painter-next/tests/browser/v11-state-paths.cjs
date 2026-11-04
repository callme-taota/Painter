// Browser regression of state-dependent controls. All writes are mocked or blocked.
// BASE_URL=http://192.168.2.209:18081 OWNER_STATE_FILE=/private/owner-state.json \
// PLAYWRIGHT_MODULE=/path/to/playwright CHROMIUM_EXECUTABLE=/usr/bin/google-chrome \
// node tests/browser/v11-state-paths.cjs
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright')
const fs=require('fs'),path=require('path')
const base=(process.env.BASE_URL||'http://192.168.2.209:18081').replace(/\/$/,''),origin=new URL(base).origin
const stateFile=process.env.OWNER_STATE_FILE||'/tmp/painter-next-v9-owner-state.json'
const out=path.resolve(process.env.ARTIFACT_DIR||'/tmp/painter-v11-state-paths')
const storageState=JSON.parse(fs.readFileSync(stateFile,'utf8'))
const secrets=[...storageState.cookies.map(c=>c.value),...(storageState.origins||[]).flatMap(o=>(o.localStorage||[]).map(item=>item.value))].filter(s=>s&&s.length>=8)
const scrub=value=>secrets.reduce((s,secret)=>s.split(secret).join('[redacted]'),String(value))
const checks=[],errors=[],blocked=[],requests=[]
let browser,scenario='startup'
function check(name,passed,detail){checks.push({scenario,check:name,passed:!!passed,...(detail===undefined?{}:{detail})})}
const json=(route,data,status=200)=>route.fulfill({status,json:data})
const failure=(route,status,code,message)=>json(route,{error:{code,message}},status)
async function setup(width=1440,mock=()=>false,touch=false){
 const context=await browser.newContext({storageState,viewport:{width,height:touch?844:1000},isMobile:touch,hasTouch:touch,serviceWorkers:'block',colorScheme:'light'})
 await context.addInitScript(()=>localStorage.setItem('painter-theme','light'))
 await context.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url()),method=request.method()
  if(url.origin!==origin)return route.continue()
  if(url.pathname.startsWith('/api/v1/')){
   if(url.pathname==='/api/v1/session'&&method==='GET')return json(route,{authenticated:true,user:{id:'1',role:'owner',display_name:'状态路径验收'},csrf_token:'mock-state-paths-csrf'})
   if(url.pathname==='/api/v1/analytics/events')return json(route,{accepted:true})
   if(await mock(route,url,method))return
   if(!['GET','HEAD'].includes(method)){blocked.push({scenario,method,path:url.pathname});return route.abort('blockedbyclient')}
  }
  if(url.pathname.startsWith('/posts/state-path-proof-'))return route.fulfill({contentType:'text/html',body:'<!doctype html><title>Native link destination</title><p>Native link destination</p>'})
  return route.continue()
 })
 const page=await context.newPage();page.setDefaultTimeout(6000)
 page.on('pageerror',error=>errors.push({scenario,message:scrub(error.message)}))
 return {context,page}
}
async function capture(page,name){if(process.env.CAPTURE_SCREENSHOTS==='1')await page.screenshot({path:path.join(out,name+'.png'),fullPage:true})}
async function media(width){
 scenario='media-'+width
 const fixtures=[
  {media_id:'state-ready',original_name:'ready.png',state:'ready',in_use:false},
  {media_id:'state-in-use',original_name:'referenced.png',state:'ready',in_use:true},
  {media_id:'state-staging',original_name:'uploading.png',state:'staging',in_use:false},
  {media_id:'state-failed',original_name:'failed.png',state:'failed',in_use:false},
  {media_id:'state-deleting',original_name:'deleting.png',state:'deleting',in_use:false}
 ].map(item=>({...item,byte_size:1024}))
 const {context,page}=await setup(width,async(route,url,method)=>{
  if(url.pathname==='/api/v1/me/media'&&method==='GET'){await json(route,{items:fixtures,has_more:false,next_cursor:null});return true}return false
 })
 await page.goto(base+'/me/media');await page.getByText('ready.png',{exact:true}).waitFor()
 for(const fixture of fixtures){
  const row=page.getByRole('row').filter({has:page.getByText(fixture.original_name,{exact:true})})
  check(fixture.state+'/'+fixture.in_use+': preview only for ready media',(await row.getByRole('link',{name:'预览',exact:true}).count())===(fixture.state==='ready'?1:0))
  check(fixture.state+'/'+fixture.in_use+': delete follows state and references',(await row.getByRole('button',{name:'删除未引用文件',exact:true}).isEnabled())===(!fixture.in_use&&['ready','failed'].includes(fixture.state)))
 }
 check('referenced media explains why deletion is unavailable',await page.getByRole('row').filter({has:page.getByText('referenced.png',{exact:true})}).getByText('· 已被引用',{exact:false}).isVisible())
 await capture(page,scenario);await context.close()
}
async function comments(width){
 scenario='comments-'+width
 const {context,page}=await setup(width,async(route,url,method)=>{
  if(url.pathname==='/api/v1/admin/comments'&&method==='GET'){await json(route,{items:['visible','hidden','deleted'].map((state,i)=>({id:String(i+1),post_id:'1',state,body:state+' comment fixture',user:{display_name:'状态验收'},created_at:'2026-11-01T00:00:00Z'})),has_more:false});return true}return false
 })
 await page.goto(base+'/studio/comments');await page.getByText('deleted comment fixture',{exact:true}).waitFor()
 const row=state=>page.locator('.comment').filter({has:page.getByText(state+' comment fixture',{exact:true})})
 check('visible comment offers hide',await row('visible').getByRole('button',{name:'隐藏评论',exact:true}).isVisible())
 check('hidden comment offers restore',await row('hidden').getByRole('button',{name:'恢复可见',exact:true}).isVisible())
 check('deleted comment remains read-only with an explicit explanation',(await row('deleted').getByRole('button').count())===0&&await row('deleted').getByText('已删除，无法恢复',{exact:true}).isVisible())
 await capture(page,scenario);await context.close()
}
async function search(){
 scenario='search-native-links'
 const suggestions=Array.from({length:18},(_,i)=>({kind:i%2?'tag':'post',label:'搜索建议 '+(i+1),url:'/posts/state-path-proof-'+(i+1)}))
 const mock=async(route,url)=>{if(url.pathname==='/api/v1/search/suggestions'){await json(route,{items:suggestions});return true}return false}
 const {context,page}=await setup(1440,mock)
 await page.goto(base+'/tags');await page.locator('[data-search-open]').first().click()
 const input=page.locator('#site-search input[name=q]'),link=page.locator('#search-option-0 a')
 await link.waitFor();const initialURL=page.url()
 await link.dispatchEvent('pointerdown',{pointerType:'mouse',button:2,buttons:2})
 check('right pointerdown does not navigate',page.url()===initialURL)
 await link.click({button:'right'});await page.keyboard.press('Escape')
 check('native right click leaves the current page in place',page.url()===initialURL)
 if(!await page.locator('#site-search').evaluate(el=>el.open))await page.locator('[data-search-open]').first().click()
 await link.waitFor()
 const popupPromise=context.waitForEvent('page')
 await link.click({button:'middle'});const popup=await popupPromise;await popup.waitForLoadState()
 check('middle click opens the anchor in another page',popup.url()===base+suggestions[0].url&&page.url()===initialURL)
 await popup.close()
 const modifiedPromise=context.waitForEvent('page')
 await link.click({modifiers:['Control']});const modified=await modifiedPromise;await modified.waitForLoadState()
 check('modified click retains native new-tab behavior',modified.url()===base+suggestions[0].url&&page.url()===initialURL)
 await modified.close()
 await input.fill('keyboard');await link.waitFor();await input.press('ArrowDown')
 check('keyboard selection exposes the selected option and active descendant',await page.locator('#search-option-0').getAttribute('aria-selected')==='true'&&await input.getAttribute('aria-activedescendant')==='search-option-0')
 await input.press('ArrowUp')
 check('ArrowUp wraps and updates ARIA selection',await page.locator('#search-option-17').getAttribute('aria-selected')==='true'&&await input.getAttribute('aria-activedescendant')==='search-option-17'&&await page.locator('#search-option-0').getAttribute('aria-selected')==='false')
 await input.press('ArrowDown');await Promise.all([page.waitForURL(base+suggestions[0].url),input.press('Enter')])
 check('Enter activates the selected anchor',page.url()===base+suggestions[0].url)
 await context.close()
 scenario='search-touch-scroll'
 const touch=await setup(390,mock,true);await touch.page.goto(base+'/tags');await touch.page.locator('[data-search-open]').first().click()
 const first=touch.page.locator('#search-option-0 a');await first.waitFor()
 const box=await first.boundingBox(),startURL=touch.page.url(),cdp=await touch.context.newCDPSession(touch.page)
 await cdp.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:box.x+40,y:box.y+25}]})
 check('touchstart waits for a completed anchor activation',touch.page.url()===startURL)
 for(let i=1;i<=5;i++)await cdp.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:box.x+40,y:box.y+25-i*24}]})
 await cdp.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]})
 await touch.page.waitForTimeout(150)
 const touchState=await touch.page.locator('#site-search').evaluate(el=>({open:el.open,scrollTop:el.scrollTop}))
 check('touch drag scroll does not navigate or change keyboard selection',touch.page.url()===startURL&&touchState.open&&touchState.scrollTop>0&&await touch.page.locator('#site-search input[name=q]').getAttribute('aria-activedescendant')===null,touchState)
 await capture(touch.page,scenario);await touch.context.close()
}
async function phrases(){
 scenario='phrase-exact-reload'
 const fixtures=Array.from({length:205},(_,i)=>({id:String(205-i),version:1,text:'片语记录 '+(205-i),source:'来源 '+(205-i),kind:'sentence',published:false}))
 let listRequests=0,detailRequests=0,detailMode='success',detailRelease,detailStarted,startedResolve
 const startDetail=()=>{detailStarted=new Promise(resolve=>{startedResolve=resolve})}
 const {context,page}=await setup(1440,async(route,url,method)=>{
  if(url.pathname==='/api/v1/admin/phrases'&&method==='GET'){
   listRequests++;const older=url.searchParams.has('cursor');await json(route,{items:older?fixtures.slice(200):fixtures.slice(0,200),has_more:!older,next_cursor:older?null:'6'});return true
  }
  if(url.pathname==='/api/v1/admin/phrases/1'&&method==='PUT'){
   requests.push({method,path:url.pathname,ifMatch:route.request().headers()['if-match']});await failure(route,409,'VERSION_CONFLICT','片语已变化，请重新加载后再保存');return true
  }
  if(url.pathname==='/api/v1/admin/phrases/1'&&method==='GET'){
   detailRequests++;startedResolve?.();await new Promise(resolve=>{detailRelease=resolve})
   if(detailMode==='success')await json(route,{...fixtures[204],version:2,text:'服务器最新的旧片语',source:'服务器最新来源'})
   else if(detailMode==='malformed')await json(route,{})
   else await failure(route,detailMode==='missing'?404:503,detailMode==='missing'?'NOT_FOUND':'STORAGE_UNAVAILABLE',detailMode==='missing'?'未找到':'重新加载暂不可用')
   return true
  }
  return false
 })
 await page.goto(base+'/studio/phrases');await page.getByRole('button',{name:'加载更多片语',exact:true}).click()
 const oldRow=page.locator('.phrase-library-row').filter({has:page.getByText('片语记录 1',{exact:true})})
 await oldRow.getByRole('button',{name:'编辑',exact:true}).click()
 const text=page.locator('#phrase-text'),source=page.locator('#phrase-source')
 const save=page.locator('.phrase-composer button.primary')
 const formSnapshot=()=>page.locator('.phrase-composer').evaluate(form=>JSON.stringify({text:form.querySelector('textarea').value,source:form.querySelector('#phrase-source').value,kind:form.querySelector('[role=combobox]').textContent.trim(),published:form.querySelector('[type=checkbox]').checked}))
 async function conflict(textValue){await text.fill(textValue);await source.fill('本地来源');await save.click();await page.getByRole('button',{name:'载入最新版本',exact:true}).waitFor()}
 async function beginReload(){startDetail();await page.getByRole('button',{name:'载入最新版本',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'载入最新',exact:true}).click();await detailStarted}
 await conflict('不应丢失的本地输入');await beginReload()
 check('reload locks the form and editing controls while exact GET is pending',await text.isDisabled()&&await source.isDisabled()&&await page.getByRole('button',{name:'新建',exact:true}).isDisabled()&&await page.locator('.phrase-row-actions button').first().isDisabled())
 detailRelease();await page.getByText('已载入服务器最新版本。',{exact:true}).waitFor()
 check('old record reloads latest content by exact ID',await text.inputValue()==='服务器最新的旧片语'&&await source.inputValue()==='服务器最新来源'&&detailRequests===1)
 check('exact reload preserves loaded pagination and list',listRequests===2&&await page.locator('.phrase-library-row').count()===205&&await page.getByText('服务器最新的旧片语',{exact:true}).count()===1)
 await page.getByRole('combobox',{name:'内容类型',exact:true}).click();await page.getByRole('option',{name:'词语',exact:true}).click()
 await page.getByRole('checkbox',{name:'在片语页公开展示',exact:true}).check()
 for(const mode of ['unavailable','missing','malformed']){
  detailMode=mode;const local='保留本地输入 '+mode
  if(mode==='unavailable')await conflict(local);else{await text.fill(local);await source.fill('本地来源')}
  const before=await formSnapshot();await beginReload();detailRelease()
  await page.waitForFunction(()=>!document.querySelector('#phrase-text').disabled)
  check(mode+': failed reload preserves text, source, kind and publication choice',await formSnapshot()===before&&await page.getByRole('button',{name:'载入最新版本',exact:true}).isVisible()&&await save.isDisabled())
 }
 check('post-reload writes use the loaded current version',requests.map(r=>r.ifMatch).join(',')==='"1","2"')
 check('reloads never restart the collection or discard older records',listRequests===2&&detailRequests===4&&await page.locator('.phrase-library-row').count()===205)
 await capture(page,scenario);await context.close()
}
;(async()=>{
 fs.mkdirSync(out,{recursive:true})
 browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--disable-dev-shm-usage']})
 await search();await media(1440);await media(390);await comments(1440);await comments(390);await phrases()
 check('no unmocked business mutations were attempted',blocked.length===0,blocked)
 check('no browser runtime errors',errors.length===0,errors)
 const report={base,checks,requests,blocked,errors,passed:checks.every(c=>c.passed)}
 fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2))
 if(!report.passed)process.exitCode=1
})().catch(error=>{console.error(scrub(error.stack||error));process.exitCode=1}).finally(async()=>{await browser?.close()})
