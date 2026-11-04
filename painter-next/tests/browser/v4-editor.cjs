// Isolated component tests: every API request is fulfilled in memory. No login,
// session file, application database, or business write is used.
// Start a separate Vite dev server, then:
// VITE_DEV_URL=http://127.0.0.1:4518 PLAYWRIGHT_MODULE=/path/to/playwright node tests/browser/v4-editor.cjs
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright')
const fs=require('fs'),path=require('path')
const base=process.env.VITE_DEV_URL||'http://127.0.0.1:4518'
const out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v4-editor')
const fixture=path.join(__dirname,'fixtures/avatar.png')
fs.mkdirSync(out,{recursive:true})
const checks=[],errors=[]
const check=(name,ok)=>{checks.push({check:name,passed:!!ok});if(!ok)throw Error(name)}
const wait=ms=>new Promise(resolve=>setTimeout(resolve,ms))
let browser,currentPage
async function sourceText(p){return p.locator('.cm-content').evaluate(e=>Array.from(e.querySelectorAll('.cm-line')).map(x=>x.textContent).join('\n'))}
async function setup(markdown,width=1440){
  const ctx=await browser.newContext({viewport:{width,height:1000}}),p=await ctx.newPage()
  currentPage=p;p.on('pageerror',error=>errors.push(error.message))
  let version=1,document={id:'9001',title:'文档编辑验证',summary:'',markdown,tag_ids:[],cover:{mode:'none'},version,draft_version:version,publication:null}
  const writes=[],uploads=[]
  const transformed=await fetch(base+'/assets/src/components/MarkdownEditor.vue').then(r=>r.text())
  const vueURL=transformed.match(/from "([^"]*deps\/vue.js[^"]*)"/)?.[1]
  if(!vueURL)throw Error('Expected a Vite dev server at VITE_DEV_URL')
  await p.route('**/editor-harness',r=>r.fulfill({contentType:'text/html',body:`<!doctype html><html lang="zh-CN"><meta name="viewport" content="width=device-width,initial-scale=1"><body><main class="work-main" style="margin:40px auto;max-width:1080px;padding:0 20px"><div id="app"></div></main><script type="module">import {createApp} from ${JSON.stringify(vueURL)};import MarkdownEditor from '/assets/src/components/MarkdownEditor.vue';import {session} from '/assets/src/api.ts';import '/assets/src/style.css';Object.assign(session,{loading:false,user:{id:'mock-user'},csrf_token:'mock-csrf',authenticated:true});createApp(MarkdownEditor,{postId:'9001'}).mount('#app');</script></body></html>`}))
  await p.route('**/api/v1/**',async r=>{
    const req=r.request(),url=new URL(req.url()),pathname=url.pathname
    let data={}
    if(pathname==='/api/v1/tags')data={items:[]}
    else if(pathname==='/api/v1/studio/posts/9001')data=document
    else if(pathname==='/api/v1/editor/preview')data={sanitized_html:'<p>规范预览由既有服务负责。</p>'}
    else if(pathname==='/api/v1/studio/posts/9001/draft'){
      const payload=req.postDataJSON();writes.push({payload,headers:req.headers()});version++;document={...document,...payload,version,draft_version:version};data={version,draft_version:version}
    }else if(pathname==='/api/v1/media'){uploads.push(req.postDataBuffer()?.length||0);await wait(150);data={media_id:'mock-image'}}
    else if(pathname==='/api/v1/analytics/events')data={accepted:true}
    else throw Error('Unmocked API request: '+req.method()+' '+pathname)
    await r.fulfill({json:data})
  })
  await p.route('**/media/mock-image',r=>r.fulfill({contentType:'image/png',body:fs.readFileSync(fixture)}))
  await p.goto(base+'/editor-harness')
  await p.locator('[aria-label="文档正文"]').waitFor({state:'attached'})
  return {ctx,p,writes,uploads,doc:p.getByRole('textbox',{name:'文档正文',exact:true})}
}
async function paste(p,data){await p.getByRole('textbox',{name:'文档正文',exact:true}).evaluate((e,data)=>{const dt=new DataTransfer();for(const [type,value] of Object.entries(data))dt.setData(type,value);e.dispatchEvent(new ClipboardEvent('paste',{clipboardData:dt,bubbles:true,cancelable:true}))},data)}
async function imageEvent(p,type){await p.getByRole('textbox',{name:'文档正文',exact:true}).evaluate((e,{type,data})=>{const dt=new DataTransfer(),bytes=Uint8Array.from(atob(data),c=>c.charCodeAt(0));dt.items.add(new File([bytes],'粘贴图片.png',{type:'image/png'}));const event=type==='paste'?new ClipboardEvent('paste',{clipboardData:dt,bubbles:true,cancelable:true}):new DragEvent('drop',{dataTransfer:dt,bubbles:true,cancelable:true,clientX:e.getBoundingClientRect().left+30,clientY:e.getBoundingClientRect().top+30});e.dispatchEvent(event)},{type,data:fs.readFileSync(fixture).toString('base64')})}
async function save(p){await p.getByRole('button',{name:'保存草稿',exact:true}).click();await p.locator('.save-state.saved').waitFor()}

;(async()=>{
  browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--disable-features=LocalNetworkAccessChecks']})
  const raw='原来的标题\n============\n\n__保留粗体源码__ 与 _斜体_。\n\n* 第一项\n* 第二项\n\n> 一段引用\n\n```js\nconst value = 1\n```\n\n| 列一 | 列二 |\n| --- | --- |\n| 甲 | 乙 |\n\n最后一段。\n'
  let t=await setup(raw)
  await t.doc.waitFor();await wait(1300)
  check('document mode is the default and opening does not save or normalize Markdown',await t.doc.isVisible()&&t.writes.length===0)
  check('headings, emphasis, quote, lists, code and table render as editable document nodes',await t.doc.locator('h1').count()===1&&await t.doc.locator('strong').count()===1&&await t.doc.locator('em').count()===1&&await t.doc.locator('blockquote').count()===1&&await t.doc.locator('ul li').count()===2&&await t.doc.locator('pre code').count()===1&&await t.doc.locator('table').count()===1)
  await t.p.getByRole('button',{name:'Markdown 源码',exact:true}).click()
  check('source mode retains the original Markdown bytes',await sourceText(t.p)===raw)
  await t.p.getByRole('button',{name:'文档',exact:true}).click();await wait(1100)
  check('switching modes does not save',t.writes.length===0)
  await t.p.getByRole('textbox',{name:'文章标题',exact:true}).fill('只修改标题')
  await save(t.p)
  check('metadata-only save keeps exact original Markdown',t.writes.at(-1).payload.markdown===raw)
  check('existing CAS and idempotency headers remain present',t.writes.at(-1).headers['if-match']==='"1"'&&!!t.writes.at(-1).headers['idempotency-key'])
  await t.doc.click();await t.p.keyboard.press('Control+End');await t.p.keyboard.press('Enter');await t.p.keyboard.insertText('在文档里直接继续写。')
  await save(t.p)
  check('actual document edit serializes Markdown through the existing save path',t.writes.at(-1).payload.markdown.includes('在文档里直接继续写。')&&t.writes.at(-1).payload.markdown!==raw)
  await t.p.screenshot({path:out+'/desktop-document.png',fullPage:true})
  await t.ctx.close()

  t=await setup('')
  await t.doc.click();await t.p.keyboard.type('/table');await t.p.getByRole('listbox',{name:'插入内容',exact:true}).waitFor();await t.p.keyboard.press('Enter')
  check('slash menu inserts a Markdown table using keyboard selection',await t.doc.locator('table tr').count()===3)
  await t.doc.locator('th').first().click();await t.p.keyboard.insertText('表头')
  await t.p.getByRole('button',{name:'添一行',exact:true}).click()
  check('table row operation works',await t.doc.locator('table tr').count()===4)
  await t.p.getByRole('button',{name:'撤销',exact:true}).click()
  check('document undo restores table structure',await t.doc.locator('table tr').count()===3)
  await t.p.getByRole('button',{name:'重做',exact:true}).click()
  check('document redo restores table operation',await t.doc.locator('table tr').count()===4)
  await save(t.p)
  check('table remains Markdown in saved payload',/\| 表头\s+\|/.test(t.writes.at(-1).payload.markdown))
  await t.ctx.close()

  t=await setup('中文输入与粘贴。')
  await t.doc.click();await t.p.keyboard.press('Control+End')
  await t.doc.dispatchEvent('compositionstart',{data:''})
  await t.p.keyboard.insertText('组合中的中文')
  await wait(1200);await t.p.getByRole('button',{name:'保存草稿',exact:true}).click();await wait(100)
  check('IME composition blocks serialization and save',t.writes.length===0&&await t.p.locator('.save-state').innerText().then(s=>s.includes('正在输入')))
  await t.doc.dispatchEvent('compositionend',{data:'组合中的中文'});await t.p.locator('.save-state.saved').waitFor()
  check('composition end commits final Chinese text',t.writes.at(-1).payload.markdown.includes('组合中的中文'))
  await t.p.keyboard.press('Enter')
  await paste(t.p,{'text/plain':'## 粘贴标题\n\n**粘贴粗体**\n\n- 粘贴列表'})
  check('plain Markdown paste becomes formatted content',await t.doc.locator('h2').filter({hasText:'粘贴标题'}).count()===1&&await t.doc.locator('strong').filter({hasText:'粘贴粗体'}).count()===1)
  await t.p.keyboard.press('Control+End');await t.p.keyboard.press('Enter')
  await paste(t.p,{'text/plain':'来自其他文档','text/html':'<p><strong>来自其他文档</strong> <em>保留格式</em></p>'})
  check('rich HTML clipboard paste keeps supported formatting',await t.doc.locator('strong').filter({hasText:'来自其他文档'}).count()===1)
  await t.p.locator('#editor-upload').setInputFiles(fixture)
  await wait(40) // Allow the file picker focus return to reach ProseMirror.
  await t.p.keyboard.insertText('上传时继续写。')
  await t.doc.locator('img[src="/media/mock-image"]').waitFor({timeout:5000}).catch(async error=>{throw Error(await t.p.locator('.notice.error').allTextContents().then(x=>x.join(' '))||error.message)})
  check('media picker inserts uploaded image into the document',t.uploads.length===1)
  await t.p.keyboard.insertText('上传后继续写。')
  check('asynchronous upload replacement preserves the writing cursor',await t.doc.innerText().then(text=>text.includes('上传时继续写。上传后继续写。')))
  await imageEvent(t.p,'paste');await t.p.waitForFunction(()=>document.querySelectorAll('.document-prose img[src]').length===2)
  check('clipboard image uses the same upload pipeline',t.uploads.length===2)
  await imageEvent(t.p,'drop');await t.p.waitForFunction(()=>document.querySelectorAll('.document-prose img[src]').length===3)
  check('dropped image uses the same upload pipeline',t.uploads.length===3)
  await save(t.p)
  check('uploaded body image uses existing Markdown media URL',t.writes.at(-1).payload.markdown.includes('](/media/mock-image)')&&!t.writes.at(-1).payload.markdown.includes('图片上传中'))
  await t.p.keyboard.press('Control+End');await t.p.keyboard.press('Enter')
  await t.ctx.setOffline(true);await t.p.keyboard.insertText('离线继续写作。');await wait(1350)
  const recovery=await t.p.evaluate(()=>new Promise((resolve,reject)=>{const r=indexedDB.open('painter-next-recovery');r.onsuccess=()=>{const d=r.result,q=d.transaction('drafts').objectStore('drafts').get(`painter:${location.origin}:mock-user:9001`);q.onsuccess=()=>{resolve(q.result);d.close()};q.onerror=()=>reject(q.error)};r.onerror=()=>reject(r.error)}))
  check('document edits participate in existing IndexedDB recovery',recovery?.markdown?.includes('离线继续写作。')&&recovery.seq>recovery.ack_seq)
  await t.ctx.setOffline(false);await t.p.locator('.save-state.saved').waitFor()
  check('online recovery saves final document Markdown',t.writes.at(-1).payload.markdown.includes('离线继续写作。'))
  await t.ctx.close()

  t=await setup('![独立图片](/media/mock-image)\n\n文字与 ![行内图片](/media/mock-image) 一起排版。')
  check('standalone and inline Markdown images both open in document mode',await t.doc.isVisible()&&await t.doc.locator('img[src]').count()===2)
  await t.ctx.close()

  t=await setup('- 上一级\n  - 嵌套条目\n\n待办清单：\n\n- [ ] 待完成\n- [x] 已完成')
  check('nested lists and task lists open without a source fallback',await t.doc.isVisible()&&await t.doc.locator('ul[data-type=taskList] input').count()===2&&await t.doc.locator('ul ul li').count()===1)
  await t.doc.locator('ul[data-type=taskList] input').first().check();await save(t.p)
  check('task checkbox updates Markdown',t.writes.at(-1).payload.markdown.includes('[x] 待完成'))
  await t.ctx.close()

  for(const [label,markdown] of [['HTML comment','普通正文。\n\n<!-- 保留我的注释 -->'],['footnote','带脚注[^one]。\n\n[^one]: 原始脚注内容'],['front matter','---\nauthor: painter\n---\n\n正文。'],['reference link','[链接][ref]\n\n[ref]: https://example.com "保留定义"']]){
    t=await setup(markdown)
    await t.p.locator('.document-compatibility').waitFor()
    check(label+' opens safely in source mode',await t.p.getByRole('button',{name:'Markdown 源码',exact:true}).getAttribute('aria-pressed')==='true'&&await sourceText(t.p)===markdown)
    await t.p.getByRole('textbox',{name:'文章标题',exact:true}).fill(label);await save(t.p)
    check(label+' survives metadata save unchanged',t.writes.at(-1).payload.markdown===markdown)
    await t.ctx.close()
  }

  t=await setup('可以粘贴扩展语法。')
  await t.doc.click();await t.p.keyboard.press('Control+End');await t.p.keyboard.press('Enter')
  await paste(t.p,{'text/plain':'## 扩展内容\n\n脚注[^keep]\n\n[^keep]: 保留脚注'})
  check('unsupported Markdown paste switches to usable source without dropping syntax',await t.p.getByRole('button',{name:'Markdown 源码',exact:true}).getAttribute('aria-pressed')==='true'&&(await sourceText(t.p)).includes('[^keep]: 保留脚注'))
  await t.ctx.close()

  t=await setup('## 在手机上写作\n\n直接编辑正文，工具按屏幕宽度换行。',390)
  await t.doc.waitFor()
  check('narrow document and formatting toolbar stay within viewport',await t.p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth))
  await t.p.screenshot({path:out+'/mobile-document.png',fullPage:true})
  await t.p.evaluate(()=>document.documentElement.dataset.theme='dark');await wait(250)
  await t.p.screenshot({path:out+'/mobile-document-dark.png',fullPage:true})
  await t.ctx.close()
  check('no browser runtime errors',errors.length===0)
  fs.writeFileSync(out+'/report.json',JSON.stringify({mocked_api:true,real_business_writes:false,checks,errors},null,2)+'\n')
  console.log(JSON.stringify({checks:checks.length,allPassed:true,mockedAPI:true}))
})().catch(async error=>{if(currentPage&&!currentPage.isClosed())await currentPage.screenshot({path:out+'/failed.png',fullPage:true}).catch(()=>{});fs.writeFileSync(out+'/report-failed.json',JSON.stringify({checks,errors,failure:error.stack},null,2)+'\n');console.error(error);process.exitCode=1}).finally(async()=>{await browser?.close()})
