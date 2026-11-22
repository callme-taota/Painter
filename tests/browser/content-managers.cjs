// Render the real workbench with synthetic, fully intercepted API traffic.
// Includes the editor; never connects to a running Painter or database.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{createRequire}=require('node:module');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const frontend=createRequire(path.resolve(__dirname,'../../web/frontend/package.json'));
const {build}=frontend('esbuild'),{parse,compileScript}=frontend('@vue/compiler-sfc');
const src=path.resolve(__dirname,'../../web/frontend/src'),out=process.env.ARTIFACT_DIR||'/tmp/painter-content-managers';
fs.mkdirSync(out,{recursive:true});
const checks=[],errors=[];
function check(name,value){assert.ok(value,name);checks.push(name);console.log('PASS '+name)}
const copy=value=>JSON.parse(JSON.stringify(value));
const post=(id,status='draft')=>({id:String(id),version:3,draft_version:2,title:'文章 '+id,summary:'摘要',markdown:'正文',tag_ids:[],cover:{mode:'none'},status,updated_at:'2026-11-22T01:00:00Z',author_name:'测试作者',publication:status==='published'?{revision_id:'r1'}:null});
const moment=()=>({id:'9',version:1,text:'动态草稿',images:[],public_images:[],public_text:null,published:false,status:'draft'});
const pageOf=items=>({items,next_cursor:'',has_more:false});
// esbuild's inlined dynamic imports need a module marker so Vue unwraps the
// SFC default export just as it does for Vite's native ESM chunks.
async function bundle(){
 const styles=[...fs.readFileSync(path.join(src,'public.ts'),'utf8').matchAll(/^import '([^']+\.css)'/gm),...fs.readFileSync(path.join(src,'studio.ts'),'utf8').matchAll(/^import '([^']+\.css)'/gm)].map(match=>`import '${match[1]}';`).join('');
 const result=await build({stdin:{contents:styles+"import {createApp} from 'vue';import App from './App.vue';const app=createApp(App);app.mount('#workbench');window.unmountFixture=()=>app.unmount();",resolveDir:src,loader:'ts'},bundle:true,write:false,outfile:'/fixture.js',format:'esm',platform:'browser',loader:{'.woff2':'dataurl'},define:{__VUE_OPTIONS_API__:'true',__VUE_PROD_DEVTOOLS__:'false',__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:'false'},plugins:[{name:'vue-fixture',setup(build){build.onLoad({filter:/\.vue$/},async args=>{const {descriptor,errors}=parse(fs.readFileSync(args.path,'utf8'),{filename:args.path});assert.equal(errors.length,0);return{contents:compileScript(descriptor,{id:path.basename(args.path),inlineTemplate:true}).content+'\nexport const __esModule=true;',loader:'ts',resolveDir:path.dirname(args.path)}})}}]});
 return {js:result.outputFiles.find(f=>f.path.endsWith('.js')).text,css:result.outputFiles.find(f=>f.path.endsWith('.css')).text};
}
async function fixture(browser,assets,route,options={}){
 const state={posts:[post(1,'published'),post(2)],moment:null,requests:[],failList:0,failMomentSave:0,...options};
 const page=await browser.newPage({viewport:{width:1200,height:900},reducedMotion:'reduce'});page.setDefaultTimeout(10000);
 await page.addInitScript(()=>localStorage.setItem('painter-analytics','off'));
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('**/*',async intercepted=>{
  const request=intercepted.request(),url=new URL(request.url()),method=request.method();
  const send=(data,status=200)=>intercepted.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
  if(url.origin!=='https://content.test'){errors.push('External request: '+url.origin);return intercepted.abort()}
  if(url.pathname==='/fixture.js')return intercepted.fulfill({contentType:'text/javascript',body:assets.js});
  if(url.pathname==='/fixture.css')return intercepted.fulfill({contentType:'text/css',body:assets.css});
  if(request.isNavigationRequest())return intercepted.fulfill({contentType:'text/html',body:'<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><body data-page-kind="studio"><div id="workbench"></div><script type="module" src="/fixture.js"></script></html>'});
  if(url.pathname==='/assets/images/painter-logo.png')return intercepted.fulfill({contentType:'image/png',body:fs.readFileSync(path.join(src,'../public/images/painter-logo.png'))});
  if(url.pathname.startsWith('/assets/'))return intercepted.fulfill({status:204,body:''});
  const entry={path:url.pathname,query:url.search,method,headers:request.headers(),body:request.postData()?JSON.parse(request.postData()):null};state.requests.push(entry);
  if(url.pathname==='/api/v1/session')return send({authenticated:true,user:{id:'7',username:'fixture',display_name:'合成站长',role:'owner'},csrf_token:'synthetic-csrf'});
  if(url.pathname==='/api/v1/me/posts'){
   if(state.failList-->0)return send({error:{code:'FIXTURE_FAILURE',message:'合成列表加载失败'}},503);
   const status=url.searchParams.get('status'),q=url.searchParams.get('q')||'';
   return send(pageOf(state.posts.filter(post=>(!status||status==='all'||post.status===status)&&post.title.includes(q))));
  }
  if(url.pathname==='/api/v1/me/moments')return send({...pageOf(state.moment?[state.moment]:[]),max_text_length:500,max_images:9,max_image_bytes:10485760});
  if(url.pathname==='/api/v1/moments'&&method==='POST'){state.moment={...moment(),...entry.body};return send(state.moment,201)}
  if(url.pathname==='/api/v1/studio/moments/9/draft'){
   if(state.failMomentSave-->0)return send({error:{code:'VERSION_CONFLICT',message:'版本冲突'}},409);
   Object.assign(state.moment,entry.body,{version:state.moment.version+1});return send(state.moment);
  }
  if(url.pathname==='/api/v1/studio/moments/9/publish'){Object.assign(state.moment,{version:state.moment.version+1,published:true,status:'published',public_text:state.moment.text});return send(state.moment)}
  if(url.pathname==='/api/v1/studio/moments/9/publication'){Object.assign(state.moment,{version:state.moment.version+1,published:false,status:'draft',public_text:null});return send(state.moment)}
  if(url.pathname==='/api/v1/studio/moments/9'&&method==='GET')return send(state.moment);
  if(url.pathname==='/api/v1/studio/moments/9'&&method==='DELETE'){state.moment=null;return intercepted.fulfill({status:204,body:''})}
  if(url.pathname==='/api/v1/admin/phrases')return send(pageOf([{id:'5',version:1,text:'合成片语',source:'',kind:'sentence',published:false}]));
  if(url.pathname==='/api/v1/me/series')return send(pageOf([]));
  if(url.pathname==='/api/v1/tags')return send(pageOf([]));
  if(url.pathname==='/api/v1/editor/preview')return send({sanitized_html:'<p>正文</p>'});
  if(url.pathname==='/api/v1/me/bookmarks'){
   if(state.failList-->0)return send({error:{code:'FIXTURE_FAILURE',message:'合成收藏加载失败'}},503);
   return send(pageOf([{...post(1),post_id:'1'}]));
  }
  const match=url.pathname.match(/^\/api\/v1\/studio\/posts\/(\d+)(.*)$/);
  if(match){
   const row=state.posts.find(p=>p.id===match[1]);
   if(method==='GET')return send(row);
   if(match[2]==='/draft'){Object.assign(row,entry.body,{version:row.version+1,draft_version:row.draft_version+1});return send({...row,request_key:entry.headers['idempotency-key']})}
   if(match[2]==='/publication'){row.status='draft';row.publication=null;row.version++;return send(row)}
   if(match[2]==='/recover'){row.status='draft';row.version++;return send(row)}
   if(match[2]==='/publish'){row.status='published';row.version++;return send({...row,revision_id:'published-revision'})}
  }
  errors.push('Unexpected request: '+method+' '+url.pathname);return send({error:{message:'Unexpected fixture request'}},500);
 });
 await page.goto('https://content.test'+route);
 await page.waitForFunction(()=>!document.querySelector('.work-main > .loading-line'));
 return{page,state};
}
async function confirm(page,label){await page.getByRole('dialog').getByRole('button',{name:label,exact:true}).click()}
function writes(state,path){return state.requests.filter(r=>r.path===path&&r.method!=='GET')}
async function capture(page,name){
 await page.evaluate(()=>window.scrollTo(0,0));
 await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
 await page.screenshot({path:path.join(out,name),fullPage:true});
}
(async()=>{
 const assets=await bundle(),browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 try{
  {
   const {page,state}=await fixture(browser,assets,'/studio/posts',{failList:1});
   await page.getByRole('button',{name:'重新加载',exact:true}).click();await page.locator('.posts-table tbody tr').filter({hasText:'文章 1'}).waitFor();
   check('article list retries through shared loader',state.requests.filter(r=>r.path==='/api/v1/me/posts').length===2);
   await page.getByRole('button',{name:'草稿',exact:true}).click();await page.locator('.posts-table tbody tr').filter({hasText:'文章 2'}).waitFor();
   check('article status filter reaches content client',new URLSearchParams(state.requests.filter(r=>r.path==='/api/v1/me/posts').at(-1).query).get('status')==='draft');
   await page.getByRole('button',{name:'全部',exact:true}).click();await page.locator('.posts-table tbody tr').filter({hasText:'文章 1'}).waitFor();
   await page.getByRole('button',{name:'更多操作：文章 1',exact:true}).click();await page.getByRole('button',{name:'撤回文章',exact:true}).click();await confirm(page,'撤回文章');
   await page.waitForFunction(()=>document.querySelector('.posts-table tbody tr')?.textContent.includes('草稿'));
   const write=writes(state,'/api/v1/studio/posts/1/publication')[0];check('article withdrawal preserves version and CSRF',write.method==='DELETE'&&write.headers['if-match']==='"3"'&&write.headers['x-csrf-token']==='synthetic-csrf');
   await page.evaluate(()=>document.querySelectorAll('.painter-toast-close').forEach(button=>button.click()));await capture(page,'posts-desktop.png');await page.setViewportSize({width:390,height:844});await page.waitForFunction(()=>document.querySelector('.workbench')?.classList.contains('is-phone'));await capture(page,'posts-mobile.png');await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/moments');const form=page.getByRole('form',{name:'动态编辑'});
   await form.getByLabel('动态内容',{exact:true}).fill('新的合成动态');await form.getByRole('button',{name:'保存草稿',exact:true}).click();await page.locator('.moment-composer-state').filter({hasText:'已保存'}).waitFor();
   check('moment create retains caller payload and idempotency',writes(state,'/api/v1/moments')[0].body.text==='新的合成动态'&&!!writes(state,'/api/v1/moments')[0].headers['idempotency-key']);
   await form.getByRole('button',{name:'发布动态',exact:true}).click();await page.locator('.moment-composer-state').filter({hasText:'已发布'}).waitFor();
   check('moment publish uses saved version',writes(state,'/api/v1/studio/moments/9/publish')[0].headers['if-match']==='"1"');
   await form.getByRole('button',{name:'撤回动态',exact:true}).click();await confirm(page,'撤回动态');await page.locator('.moment-composer-state').filter({hasText:'草稿'}).waitFor();
   check('moment withdrawal retains independent draft',state.moment.text==='新的合成动态'&&state.moment.published===false);
   state.failMomentSave=1;await form.getByLabel('动态内容',{exact:true}).fill('冲突时保留输入');await form.getByRole('button',{name:'保存草稿',exact:true}).click();await page.getByRole('button',{name:'读取最新版本',exact:true}).click();await page.getByRole('button',{name:'保留我的输入，继续编辑',exact:true}).click();
   check('moment conflict preserves unsaved input',await form.getByLabel('动态内容',{exact:true}).inputValue()==='冲突时保留输入');
   await form.getByRole('button',{name:'保存草稿',exact:true}).click();await page.locator('.moment-composer-state').filter({hasText:'已保存'}).waitFor();
   await page.evaluate(()=>document.querySelectorAll('.painter-toast-close').forEach(button=>button.click()));await capture(page,'moments-desktop.png');await page.setViewportSize({width:390,height:844});await page.waitForFunction(()=>document.querySelector('.workbench')?.classList.contains('is-phone'));await capture(page,'moments-mobile.png');await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/phrases');await page.locator('.phrase-library-row').filter({hasText:'合成片语'}).waitFor();
   check('phrase manager mounts with shared list state',state.requests.filter(r=>r.path==='/api/v1/admin/phrases').length===1);await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/series');await page.getByRole('button',{name:'新建专栏',exact:true}).click();await page.getByRole('button',{name:'添加文章',exact:true}).click();await page.locator('.series-post-options li').filter({hasText:'文章 1'}).waitFor();
   check('chapter picker uses the same article list client',state.requests.filter(r=>r.path==='/api/v1/me/posts').length===1);await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/me/bookmarks',{failList:1});await page.getByRole('button',{name:'重新加载',exact:true}).click();await page.getByText('文章 1',{exact:true}).waitFor();
   check('workbench bookmark list retries with shared state',state.requests.filter(r=>r.path==='/api/v1/me/bookmarks').length===2);await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/posts/2');const title=page.locator('.editor-title');await title.waitFor();await title.fill('统一请求后的标题');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.save-state')?.textContent.includes('已保存'));
   const save=writes(state,'/api/v1/studio/posts/2/draft')[0];check('editor save preserves draft version and stable request key',save.body.title==='统一请求后的标题'&&save.body.expected_draft_version===2&&save.headers['if-match']==='"3"'&&!!save.headers['idempotency-key']);await page.close();
  }
  check('all workbench routes finish without runtime errors or unexpected requests',errors.length===0);
  fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({status:'passed',checks,errors},null,2));
 }finally{
  if(errors.length)console.error(errors);
  for(const context of browser.contexts())for(const page of context.pages()){
   fs.writeFileSync(path.join(out,'unfinished-page.html'),await page.content());
   await page.screenshot({path:path.join(out,'unfinished-page.png'),fullPage:true});
  }
  await browser.close();
 }
})().catch(error=>{console.error(error);process.exitCode=1});
