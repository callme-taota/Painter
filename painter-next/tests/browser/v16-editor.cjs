// DOM/interaction checks only. Screenshots belong to the root's V16 batch.
// Article data is a labelled fixture and all business mutations are intercepted.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const fs=require('fs'),path=require('path'),{spawnSync}=require('child_process');
const base=(process.env.BASE_URL||'http://192.168.2.209:18081').replace(/\/$/,''),origin=new URL(base).origin;
if(new URL(base).port==='18080')throw Error('V16 editor harness refuses the formal service');
const stateFile=process.env.OWNER_STATE_FILE,out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v16/editor');
if(!stateFile)throw Error('Private OWNER_STATE_FILE required');
let state;try{state=JSON.parse(fs.readFileSync(stateFile,'utf8'))}catch{throw Error('Invalid private browser state')}
const privateValues=(state.cookies||[]).map(c=>c.value).filter(Boolean),redact=s=>privateValues.reduce((v,p)=>v.split(p).join('[redacted]'),String(s));
const checks=[],geometry=[],errors=[],blocked=[],mockWrites=[];let browser,scenario='setup';
const check=(name,ok)=>{checks.push({scenario,name,passed:!!ok});if(!ok)throw Error(scenario+': '+name)};
const fixture={id:'991',title:'让写作保持连续：从一个想法开始，记录写作、设计与工程中那些值得认真思考的细节',summary:'编辑器 V16 的隔离交互验收。',markdown:'## 写作，从一个想法开始\n\n标题、工具栏与正文保持同一个阅读边界。\n\n元信息不随正文的表示方式改变。\n',tag_ids:['2'],media_ids:[],cover:{mode:'none',template:'editorial',color:'ink'},date_display:{created:true,updated:false,published:true},version:10,draft_version:20,publication:null};
const unsupported='## 原文需要保留\n\n带有脚注的内容[^memo]。\n\n[^memo]: 文档表示暂不支持的脚注。\n';
async function context(width,theme,options={}){
 const c=await browser.newContext({storageState:stateFile,viewport:{width,height:width<720?900:1080},colorScheme:theme,serviceWorkers:'block'});
 await c.addInitScript(t=>localStorage.setItem('painter-theme',t),theme);
 let failed=!!options.fail,raw=options.unsupported?unsupported:fixture.markdown;
 await c.route('**/api/v1/**',async route=>{
  const r=route.request(),u=new URL(r.url()),m=r.method();if(u.origin!==origin)return route.continue();
  const json=(v,status=200)=>route.fulfill({status,json:v});
  if(m==='GET'&&u.pathname==='/api/v1/session')return json({authenticated:true,user:{id:'1',role:'owner',display_name:'V16 编辑验收'},csrf_token:'v16-fixture-csrf'});
  if(m==='GET'&&u.pathname==='/api/v1/tags')return json({items:[{id:'2',name:'写作'},{id:'3',name:'设计'}],has_more:false,next_cursor:''});
  if(m==='GET'&&u.pathname==='/api/v1/studio/posts/991')return failed?json({error:{code:'WRITE_BUSY',message:'V16 隔离加载失败用例'}},503):json({...fixture,markdown:raw});
  if(m==='POST'&&u.pathname==='/api/v1/editor/preview')return json({sanitized_html:'<h2>写作，从一个想法开始</h2><p>V16 隔离规范预览。</p>'});
  if(m==='POST'&&u.pathname==='/api/v1/editor/cover-preview')return route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="675"><rect width="1200" height="675" fill="#173044"/></svg>'});
  if(m==='POST'&&u.pathname==='/api/v1/analytics/events')return json({accepted:0});
  if(m==='PUT'&&u.pathname==='/api/v1/studio/posts/991/draft'){mockWrites.push({scenario,version:r.headers()['if-match'],data:r.postDataJSON()});return json({id:'991',version:11,draft_version:21,ack_mutation_id:r.headers()['idempotency-key']})}
  if(m==='GET'||m==='HEAD')return route.continue();
  blocked.push({scenario,method:m,path:u.pathname});return route.abort();
 });
 const p=await c.newPage();p.setDefaultTimeout(9000);p.on('pageerror',e=>errors.push(redact(e.message)));
 return {c,p,retry:()=>{failed=false}};
}
async function open(p){await p.goto(base+'/studio/posts/991');await p.waitForFunction(()=>document.querySelector('.editor-unified .editor-title')?.disabled===false);await p.evaluate(()=>document.fonts.ready)}
async function more(p){await p.evaluate(()=>scrollTo(0,0));const d=p.locator('.editor-more');if(!await d.evaluate(e=>e.open))await d.locator('summary').click();return d}
async function source(p){await more(p);await p.getByRole('button',{name:'编辑 Markdown 源码',exact:true}).click();await p.getByRole('textbox',{name:'Markdown 正文',exact:true}).waitFor();await p.evaluate(()=>scrollTo(0,0))}
async function info(p){const d=p.locator('.editor-properties');if(!await d.evaluate(e=>e.open))await d.locator('summary').click()}
async function boxes(p){return p.evaluate(()=>{
 const box=s=>{const e=document.querySelector(s),r=e.getBoundingClientRect(),c=getComputedStyle(e);return{x:r.x,y:r.y+scrollY,w:r.width,h:r.height,left:r.x+parseFloat(c.paddingLeft),open:e.open}};
 return{width:innerWidth,title:box('.editor-title'),properties:box('.editor-properties'),writing:box('.editor-writing'),toolbar:box('.document-toolbar'),body:box('.document-prose'),overflow:document.documentElement.scrollWidth>innerWidth};
})}
async function fieldBoxes(p){return p.evaluate(()=>Object.fromEntries(Object.entries({summary:'.editor-metadata>label',tags:'.tag-picker',dates:'.date-display-options',cover:'.cover-editor .select-trigger'}).map(([name,selector])=>{const r=document.querySelector(selector).getBoundingClientRect();return[name,{x:r.x,y:r.y+scrollY,w:r.width}]})))}
async function titleContent(p){return p.locator('.editor-title').evaluate(e=>({tag:e.tagName,value:e.value,height:e.getBoundingClientRect().height,clientHeight:e.clientHeight,scrollHeight:e.scrollHeight,clientWidth:e.clientWidth,scrollWidth:e.scrollWidth,scrollLeft:e.scrollLeft}))}
function titleFits(t){return t.tag==='TEXTAREA'&&t.value===fixture.title&&t.scrollHeight<=t.clientHeight+1&&t.scrollWidth<=t.clientWidth+1&&t.scrollLeft===0}
async function sourceGlyphs(p){
 const lines=p.locator('.cm-content>.cm-line').filter({hasText:/\S/}),result=[];
 for(let i=0;i<Math.min(2,await lines.count());i++){
  const line=lines.nth(i);await line.evaluate(e=>e.scrollIntoView({block:'center',inline:'nearest'}));await p.evaluate(()=>new Promise(requestAnimationFrame));
  result.push(await line.evaluate(e=>{
   const box=r=>({x:r.x,y:r.y,right:r.right,bottom:r.bottom,w:r.width,h:r.height}),gutter=document.querySelector('.cm-gutters').getBoundingClientRect(),scroller=document.querySelector('.cm-scroller').getBoundingClientRect(),walker=document.createTreeWalker(e,NodeFilter.SHOW_TEXT),glyphs=[];
   for(let node=walker.nextNode();node&&glyphs.length<8;node=walker.nextNode())for(let at=0;at<node.textContent.length&&glyphs.length<8;at++){
    const char=node.textContent[at];if(!char.trim())continue;const range=document.createRange();range.setStart(node,at);range.setEnd(node,at+1);const rect=range.getBoundingClientRect(),hit=document.elementFromPoint(rect.x+rect.width/2,rect.y+rect.height/2);
    glyphs.push({char,...box(rect),hit:hit?.className||'',visible:rect.width>0&&rect.x>=gutter.right-.5&&rect.x>=scroller.left-.5&&rect.right<=scroller.right+.5&&rect.y>=0&&rect.bottom<=innerHeight&&!!hit&&!!document.querySelector('.cm-content')?.contains(hit)});
   }
   return{text:e.textContent,gutter:box(gutter),scroller:box(scroller),glyphs};
  }));
 }
 await p.evaluate(()=>scrollTo(0,0));return result;
}
function sameRegion(a,b){return Math.abs(a.x-b.x)<1&&Math.abs(a.y-b.y)<1&&Math.abs(a.w-b.w)<1}
async function matrix(width,theme){
 scenario=width+'-'+theme;const {c,p}=await context(width,theme);await open(p);await p.getByRole('textbox',{name:'文档正文',exact:true}).waitFor();
 check('document is the default and no primary mode tabs remain',await p.locator('.editor-mode-tabs,.editor-mode-bar').count()===0&&await p.locator('.document-editor').isVisible());
 await p.locator('.editor-properties').evaluate(e=>e.dataset.identity='same-metadata-node');
 const before=await boxes(p);geometry.push({scenario,representation:'document',...before});
 check('title, toolbar and prose share a text edge',Math.abs(before.title.left-before.toolbar.left)<1&&Math.abs(before.title.left-before.body.left)<1);
 check('no page horizontal overflow',!before.overflow);
 const title=await titleContent(p);geometry.push({scenario,title_content:title});check('long article title wraps completely without an internal scroll',titleFits(title));
 check('only one summary tag cover and date settings instance',await p.getByLabel('摘要',{exact:true}).count()===1&&await p.locator('.tag-picker').count()===1&&await p.locator('.date-display-options input').count()===3&&await p.locator('.editor-properties .select-trigger[aria-label="封面来源"]').count()===1);
 const menu=await more(p);check('source entry lives in More',await menu.getByRole('button',{name:'编辑 Markdown 源码',exact:true}).isVisible());await p.keyboard.press('Escape');check('Escape closes More and returns focus',!await menu.evaluate(e=>e.open)&&await menu.locator('summary').evaluate(e=>document.activeElement===e));
 await source(p);const after=await boxes(p);geometry.push({scenario,representation:'source',...after});
 check('source keeps title and settings position and width',sameRegion(before.title,after.title)&&sameRegion(before.properties,after.properties));
 check('source preserves metadata DOM and disclosure state',await p.locator('.editor-properties').evaluate(e=>e.dataset.identity==='same-metadata-node')&&before.properties.open===after.properties.open);
 check('source replaces only the body with single editor by default',await p.locator('.editor-panes').getAttribute('data-view')==='edit'&&!await p.locator('.editor-preview').isVisible()&&Math.abs(before.writing.w-after.writing.w)<1);
 const sourceText=await p.locator('.cm-content>.cm-line').first().boundingBox();geometry.push({scenario,source_text_edge:sourceText.x,title_text_edge:before.title.left,gutter:await p.locator('.cm-gutters').evaluate(e=>({width:e.getBoundingClientRect().width,margin:getComputedStyle(e).marginInlineStart}))});check('source text uses the same article text edge',Math.abs(sourceText.x-before.title.left)<2);
 const glyphs=await sourceGlyphs(p);geometry.push({scenario,source_prefix_glyphs:glyphs});check('Markdown prefixes and first text glyphs are painted outside the sticky gutter',glyphs.length===2&&glyphs.every(line=>line.glyphs.length>=4&&line.glyphs.every(g=>g.visible)));
 await info(p);const summary=p.getByLabel('摘要',{exact:true});check('same summary and date values remain available in source',await summary.inputValue()===fixture.summary&&await p.getByLabel('显示最后编辑时间',{exact:true}).isChecked()===false);
 await p.evaluate(()=>scrollTo(0,0));await p.locator('.editor-source-context').getByRole('button',{name:'返回文档编辑',exact:true}).click();await p.evaluate(()=>scrollTo(0,0));
 check('returning to document keeps settings open and values intact',await p.locator('.editor-properties').evaluate(e=>e.open)&&await summary.inputValue()===fixture.summary);
 const openBefore=await boxes(p),fieldsBefore=await fieldBoxes(p);await source(p);await p.evaluate(()=>scrollTo(0,0));const openAfter=await boxes(p),fieldsAfter=await fieldBoxes(p);check('expanded metadata also stays in place across representations',sameRegion(openBefore.properties,openAfter.properties));
 geometry.push({scenario,settings_fields:{document:fieldsBefore,source:fieldsAfter},source_text_edge:sourceText.x});check('summary tags dates and cover each keep their own position and width',Object.keys(fieldsBefore).every(k=>sameRegion(fieldsBefore[k],fieldsAfter[k])));
 if(width<=960){await more(p);const m=await p.locator('.editor-more-menu').boundingBox();check('More popup stays within viewport',m.x>=0&&m.x+m.width<=width+.1);await p.keyboard.press('Escape')}
 if(width===1440){
  await p.setViewportSize({width:390,height:900});await p.waitForFunction(()=>{const e=document.querySelector('.editor-title');return e.scrollHeight<=e.clientHeight+1});const narrow=await titleContent(p);check('title grows after resizing to a narrow viewport',titleFits(narrow)&&narrow.height>title.height);
  await p.setViewportSize({width,height:1080});await p.waitForFunction(h=>document.querySelector('.editor-title').getBoundingClientRect().height<=h+1,title.height);check('title shrinks back after widening without losing text',titleFits(await titleContent(p)));
 }
 await c.close();
}
async function specialStates(){
 scenario='unsupported-markdown';const {c,p}=await context(1440,'dark',{unsupported:true});await open(p);await p.locator('.document-compatibility').waitFor();await p.getByRole('textbox',{name:'Markdown 正文',exact:true}).waitFor();
 check('unsupported syntax falls back to source with preservation notice',(await p.locator('.document-compatibility').innerText()).includes('已保留全部原文')&&(await p.locator('.cm-content').innerText()).includes('[^memo]:'));
 await more(p);check('unsafe document conversion is unavailable',await p.locator('.editor-more-menu').getByRole('button',{name:'返回文档编辑',exact:true}).isDisabled());
 const event=p.waitForEvent('download');await p.getByRole('button',{name:'导出 Markdown',exact:true}).click();const download=await event,stream=await download.createReadStream();let text='';for await(const chunk of stream)text+=chunk;check('export retains the exact unsupported Markdown',text===unsupported);await c.close();
 scenario='failed-load';const failed=await context(390,'light',{fail:true});await failed.p.goto(base+'/studio/posts/991');await failed.p.getByText('编辑器未能安全加载',{exact:true}).waitFor();await info(failed.p);await more(failed.p);
 check('failed load locks source entry title summary and publish',await failed.p.getByRole('button',{name:'编辑 Markdown 源码',exact:true}).isDisabled()&&await failed.p.getByLabel('文章标题',{exact:true}).isDisabled()&&await failed.p.getByLabel('摘要',{exact:true}).isDisabled()&&await failed.p.getByRole('button',{name:'发布文章',exact:true}).isDisabled());
 await failed.p.keyboard.press('Escape');failed.retry();await failed.p.getByRole('button',{name:'重新加载编辑器',exact:true}).click();await failed.p.getByRole('textbox',{name:'文档正文',exact:true}).waitFor();check('retry restores the validated draft before unlocking',await failed.p.getByLabel('文章标题',{exact:true}).inputValue()===fixture.title);await failed.c.close();
}
function safety(){
 if(process.env.RUN_SAFETY==='0')return {skipped:true};
 // Keep the existing load/conflict/recovery/restore workload intact. Adapt
 // only the obsolete mode-tab locator in memory; do not rewrite old evidence.
 let source=fs.readFileSync(path.join(__dirname,'v11-editor-safety.cjs'),'utf8');
 const click="await page.getByRole('button',{name:'Markdown 源码',exact:true}).click();";
 if(source.split(click).length-1!==2)throw Error('V11 mode-entry adapter needs review');
 source="async function enterV16Source(page){await page.evaluate(()=>scrollTo(0,0));const menu=page.locator('.editor-more');if(!await menu.evaluate(e=>e.open))await menu.locator('summary').click();await page.getByRole('button',{name:'编辑 Markdown 源码',exact:true}).click();}\n"+source.split(click).join("await enterV16Source(page);");
 source=source.replace("await page.getByRole('button',{name:'Markdown 源码',exact:true}).isDisabled()","await page.locator('.editor-more-menu button').first().isDisabled()");
 const result=spawnSync(process.execPath,['-e',source],{cwd:path.resolve(__dirname,'../..'),encoding:'utf8',timeout:120000,env:{...process.env,BASE_URL:base,ARTIFACT_DIR:path.join(out,'safety'),CAPTURE_SCREENSHOTS:'0'}});
 if(result.status!==0)throw Error('Editor safety regression failed: '+redact(result.stderr||result.stdout).slice(0,1000));
 return {exit_code:result.status,observed_summary:redact(result.stdout.trim()),mode_locator_adapter_only:true,captures:0};
}
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox','--disable-dev-shm-usage']});
 for(const width of(process.env.EDITOR_WIDTHS||'1920,1440,900,390,320').split(',').map(Number))for(const theme of(process.env.EDITOR_THEMES||'light,dark').split(','))await matrix(width,theme);
 await specialStates();check('no unhandled browser errors',errors.length===0);check('no unexpected business writes',blocked.length===0&&mockWrites.length===0);await browser.close();browser=null;
 const legacy=safety();fs.rmSync(path.join(out,'failure.json'),{force:true});fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({passed:true,checks,geometry,errors,blocked,actual_business_writes:0,captures:[],legacy_safety:legacy},null,2));console.log(JSON.stringify({passed:true,checks:checks.length,captures:0,legacy_safety:legacy}));
})().catch(async e=>{fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'failure.json'),JSON.stringify({checks,geometry,errors,blocked,error:redact(e.message),captures:[]},null,2));console.error(redact(e.message));await browser?.close();process.exitCode=1});
