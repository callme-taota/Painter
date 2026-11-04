// V8 replay of V4 assertions, adapted only for the project-owned restore dialog.
// Real integration acceptance. Run only against the disposable candidate DB.
// Uses a normal owner's browser state; never prints or copies credentials/state.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'/tmp/painter-browser/node_modules/playwright')
const fs=require('fs'),path=require('path'),crypto=require('crypto')
const base=process.env.BASE_URL||'http://192.168.2.209:18081'
const stateFile=process.env.BROWSER_STATE_FILE||'/tmp/painter-next-v3-lan-browser-state.json'
const out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v8/document')
fs.mkdirSync(out,{recursive:true})
const checks=[],errors=[],httpErrors=[]
let browser,ctx,page,anonymous,id,originalRevision,editedRevision,mediaURL,editedMarkdown,wasPublished=false,withdrawn=false
const check=(name,passed)=>{checks.push({check:name,passed:!!passed});if(!passed)throw Error(name)}
async function request(endpoint,method='GET',data,version){
  const session=await ctx.request.get(base+'/api/v1/session').then(r=>r.json())
  const headers={'Origin':base,'Content-Type':'application/json','X-CSRF-Token':session.csrf_token}
  if(method!=='GET'){headers['Idempotency-Key']=crypto.randomUUID();if(version!==undefined)headers['If-Match']='"'+version+'"'}
  const response=await ctx.request.fetch(base+'/api/v1'+endpoint,{method,headers,data})
  const result=await response.json().catch(()=>({}))
  if(!response.ok())throw Error(method+' '+endpoint+' returned '+response.status()+' '+(result.error?.code||''))
  return result
}
async function command(button,endpoint,method='POST'){
  const response=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1'+endpoint&&r.request().method()===method)
  await page.getByRole('button',{name:button,exact:true}).click()
  const result=await response;if(!result.ok())throw Error(button+' returned '+result.status())
  return result.json()
}
async function save(){await command('保存草稿','/studio/posts/'+id+'/draft','PUT');await page.locator('.save-state.saved').waitFor()}
async function restore(rid){
  if(await page.getByRole('heading',{name:'不可变修订',exact:true}).count())await page.getByRole('button',{name:'收起修订记录',exact:true}).click()
  const listed=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/studio/posts/'+id+'/revisions'&&r.request().method()==='GET')
  await page.getByRole('button',{name:'查看修订记录',exact:true}).click();await listed
  const revisions=(await request('/studio/posts/'+id+'/revisions')).items
  const index=revisions.findIndex(r=>String(r.id||r.revision_id)===String(rid));if(index<0)throw Error('Restore revision missing')
  const panel=page.locator('.panel').filter({has:page.getByRole('heading',{name:'不可变修订',exact:true})})
  const response=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/studio/posts/'+id+'/restore'&&r.request().method()==='POST')
  await panel.locator('.user-row').nth(index).getByRole('button',{name:'恢复为草稿',exact:true}).click()
  await page.getByRole('dialog').getByRole('button',{name:'恢复为草稿',exact:true}).click()
  const result=await response;if(!result.ok())throw Error('Revision restore returned '+result.status())
  await page.getByText('历史版本已恢复为草稿，尚未发布。',{exact:true}).waitFor()
}
async function cleanupPublication(){
  if(!ctx||!id)return
  const current=await request('/studio/posts/'+id)
  if(current.publication){await request('/studio/posts/'+id+'/publication','DELETE',undefined,current.version);withdrawn=true}
}
;(async()=>{
  browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox','--disable-dev-shm-usage']})
  ctx=await browser.newContext({storageState:stateFile,viewport:{width:1440,height:1000}})
  page=await ctx.newPage();page.on('pageerror',e=>errors.push(e.message));page.on('response',r=>{if(r.url().includes('/api/v1/')&&r.status()>=400)httpErrors.push({path:new URL(r.url()).pathname,status:r.status()})})
  const raw='原样的 Setext 标题\n=================\n\n__保留原来的粗体写法__，以及 _原来的斜体_。\n\n> 引用里的文字不该在打开时改写。\n\n* 第一条清单\n* 第二条清单\n\n```html\n<script>window.__painterUnsafe = true</script>\n```\n\n| 格式 | 状态 |\n| ---- | ---- |\n| Markdown | 原样保留 |\n\n最后一段，准备在文档里继续写作。\n'
  const title='文档真实链路验收 '+Date.now()
  const created=await request('/posts','POST',{title,summary:'文档编辑与 Markdown 保存链路的候选环境验收。',markdown:raw,tag_ids:[],cover:{mode:'none'},client_draft_id:crypto.randomUUID()})
  id=String(created.id||created.post_id);check('real API creates an exact Markdown QA draft',(await request('/studio/posts/'+id)).markdown===raw)
  const draftWrites=[];page.on('request',r=>{if(new URL(r.url()).pathname==='/api/v1/studio/posts/'+id+'/draft'&&r.method()==='PUT')draftWrites.push(r.postDataJSON())})
  await page.goto(base+'/studio/posts/'+id);const doc=page.getByRole('textbox',{name:'文档正文',exact:true});await doc.waitFor();await page.waitForTimeout(1300)
  check('opening an existing real draft does not PUT or normalize Markdown',draftWrites.length===0&&(await request('/studio/posts/'+id)).markdown===raw)
  await page.getByRole('button',{name:'Markdown 源码',exact:true}).click();await page.locator('.cm-content').waitFor()
  check('source mode displays the original Markdown exactly',await page.locator('.cm-content').evaluate(e=>Array.from(e.querySelectorAll('.cm-line')).map(x=>x.textContent).join('\n'))===raw)
  await page.getByRole('button',{name:'文档',exact:true}).click();await page.waitForTimeout(1100)
  check('switching real editor modes does not PUT',draftWrites.length===0)
  await page.getByRole('textbox',{name:'文章标题',exact:true}).fill(title+' · 已验证')
  await save();check('metadata-only real save preserves exact Markdown',(await request('/studio/posts/'+id)).markdown===raw)
  const checkpoint=await command('留检查点','/studio/posts/'+id+'/checkpoints');originalRevision=checkpoint.revision_id
  check('original Markdown has an immutable checkpoint',!!originalRevision&&(await request('/studio/posts/'+id+'/revisions/'+originalRevision)).markdown===raw)
  const automatic=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/studio/posts/'+id+'/draft'&&r.request().method()==='PUT')
  await doc.locator(':scope > p').last().click();await page.waitForTimeout(50);await page.keyboard.press('End');await page.keyboard.press('Enter');await page.keyboard.insertText('这是直接在文档模式输入并自动保存的中文段落。')
  check('real document edit auto-saves through the draft endpoint',(await automatic).ok()&&(await request('/studio/posts/'+id)).markdown.includes('直接在文档模式输入并自动保存'))
  await page.keyboard.press('Enter');await page.keyboard.insertText('这段文字通过手动保存确认。')
  await save();check('manual real document save stores Markdown',(await request('/studio/posts/'+id)).markdown.includes('通过手动保存确认'))
  await page.locator('#editor-upload').setInputFiles(process.env.UPLOAD_FIXTURE_FILE||path.join(__dirname,'fixtures/avatar.png'))
  const image=doc.locator('img[src^="/media/"]');await image.waitFor();await image.evaluate(i=>i.complete?Promise.resolve():new Promise(resolve=>{i.onload=resolve;i.onerror=resolve}))
  mediaURL=await image.getAttribute('src');check('real upload replaces the document placeholder with ready media',!!mediaURL&&await image.evaluate(i=>i.naturalWidth>0))
  await save();const saved=await request('/studio/posts/'+id);editedMarkdown=saved.markdown
  check('real uploaded image survives Markdown save',editedMarkdown.includes(']('+mediaURL+')')&&!editedMarkdown.includes('图片上传中'))
  const edited=await command('留检查点','/studio/posts/'+id+'/checkpoints');editedRevision=edited.revision_id
  check('edited checkpoint keeps the same Markdown and media',(await request('/studio/posts/'+id+'/revisions/'+editedRevision)).markdown===editedMarkdown)
  await command('发布文章','/studio/posts/'+id+'/publish');wasPublished=true
  const published=await request('/posts/'+id)
  check('real publish uses an immutable public revision',!!published.revision_id&&published.sanitized_html.includes(mediaURL))
  anonymous=await browser.newContext({viewport:{width:1440,height:1000}});const publicPage=await anonymous.newPage();publicPage.on('pageerror',e=>errors.push(e.message))
  await publicPage.goto(base+'/posts/'+id);await publicPage.locator('.prose').waitFor();const publicImage=publicPage.locator('.prose img[src="'+mediaURL+'"]');await publicImage.waitFor();await publicPage.waitForFunction(src=>{const image=document.querySelector('.prose img[src="'+src+'"]');return image?.complete&&image.naturalWidth>0},mediaURL)
  check('anonymous public reader sees safely rendered document and media',await publicPage.locator('.prose h1').filter({hasText:'原样的 Setext 标题'}).count()===1&&await publicPage.locator('.prose table').count()===1&&await publicPage.locator('.prose blockquote').count()===1&&await publicPage.locator('.prose strong').count()===1&&await publicImage.evaluate(i=>i.naturalWidth>0))
  check('code with script text remains inert public content',await publicPage.locator('.prose script,.prose [onerror]').count()===0&&await publicPage.locator('.prose pre').innerText().then(text=>text.includes('<script>'))&&await publicPage.evaluate(()=>!window.__painterUnsafe))
  check('anonymous media request is publicly available while published',(await anonymous.request.get(base+mediaURL)).status()===200)
  await publicPage.screenshot({path:out+'/public-qa-before-withdrawal.png',fullPage:true})
  await restore(originalRevision)
  const restored=await request('/studio/posts/'+id),stillPublished=await request('/posts/'+id)
  check('revision restore returns original Markdown to draft',restored.markdown===raw)
  check('revision restore leaves public revision and HTML unchanged',stillPublished.revision_id===published.revision_id&&stillPublished.sanitized_html===published.sanitized_html)
  await command('撤回','/studio/posts/'+id+'/publication','DELETE');withdrawn=true
  check('withdraw removes public access and leaves draft',(await anonymous.request.get(base+'/posts/'+id)).status()===404&&!(await request('/studio/posts/'+id)).publication)
  check('withdraw removes anonymous access to body media',(await anonymous.request.get(base+mediaURL)).status()===404)
  await restore(editedRevision)
  const finalDraft=await request('/studio/posts/'+id)
  check('unpublished QA retains edited document for review',!finalDraft.publication&&finalDraft.markdown===editedMarkdown)
  await page.getByRole('button',{name:'文档',exact:true}).click();await page.evaluate(()=>scrollTo(0,0));await page.waitForTimeout(250)
  await page.screenshot({path:out+'/unpublished-document-qa.png',fullPage:true})
  check('no real browser runtime errors',errors.length===0)
  fs.writeFileSync(out+'/report.json',JSON.stringify({origin:base,real_api:true,created_post_id:id,qa_publication_withdrawn:withdrawn,retained_private_edited_draft:true,original_revision_id:originalRevision,edited_revision_id:editedRevision,media_url:mediaURL,checks,errors,httpErrors},null,2)+'\n')
  console.log(JSON.stringify({checks:checks.length,allPassed:true,created_post_id:id,qa_publication_withdrawn:withdrawn}))
})().catch(async error=>{
  let cleanup={};try{await cleanupPublication();cleanup={publication_withdrawn:withdrawn}}catch(cleanupError){cleanup={error:cleanupError.message}}
  if(page&&!page.isClosed())await page.screenshot({path:out+'/failed.png',fullPage:true}).catch(()=>{})
  fs.writeFileSync(out+'/report-failed.json',JSON.stringify({origin:base,created_post_id:id,was_published:wasPublished,checks,errors,httpErrors,failure:error.stack,cleanup},null,2)+'\n')
  console.error(error.message);process.exitCode=1
}).finally(async()=>{await browser?.close()})
