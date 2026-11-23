// Render the real workbench with synthetic, fully intercepted API traffic.
// Includes the editor; never connects to a running Painter or database.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{createRequire}=require('node:module');
const {createHash}=require('node:crypto');
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
const home=()=>{
 const order=['intro','articles','steam','music','photos','closing','activity','projects'];
 return {version:1,max_image_bytes:10485760,config:{order,sections:Object.fromEntries(order.map(key=>[key,{enabled:true,title:key,description:'',limit:0,link_label:'',link_url:''}])),activity:{source:'posts',period:'rolling',timezone:'Asia/Shanghai',year:0},intro_roles:[],intro_card_name:'',use_site_intro:false,motion_enabled:true,warmth_enabled:true,intro_name:'',writing_title:'',writing_description:'',scroll_label:''},steam:{steam_id:'',mode:'owned',auto_sync_enabled:false,api_key_configured:false}};
};
// esbuild's inlined dynamic imports need a module marker so Vue unwraps the
// SFC default export just as it does for Vite's native ESM chunks.
async function bundle(){
 const styles=[...fs.readFileSync(path.join(src,'public.ts'),'utf8').matchAll(/^import '([^']+\.css)'/gm),...fs.readFileSync(path.join(src,'studio.ts'),'utf8').matchAll(/^import '([^']+\.css)'/gm)].map(match=>`import '${match[1]}';`).join('');
 const result=await build({stdin:{contents:styles+"import {createApp} from 'vue';import App from './App.vue';const app=createApp(App);app.mount('#workbench');window.unmountFixture=()=>app.unmount();",resolveDir:src,loader:'ts'},bundle:true,write:false,outfile:'/fixture.js',format:'esm',platform:'browser',loader:{'.woff2':'dataurl'},define:{__VUE_OPTIONS_API__:'true',__VUE_PROD_DEVTOOLS__:'false',__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:'false'},plugins:[{name:'vue-fixture',setup(build){build.onLoad({filter:/\.vue$/},async args=>{const {descriptor,errors}=parse(fs.readFileSync(args.path,'utf8'),{filename:args.path});assert.equal(errors.length,0);return{contents:compileScript(descriptor,{id:path.basename(args.path),inlineTemplate:true}).content+'\nexport const __esModule=true;',loader:'ts',resolveDir:path.dirname(args.path)}})}}]});
 return {js:result.outputFiles.find(f=>f.path.endsWith('.js')).text,css:result.outputFiles.find(f=>f.path.endsWith('.css')).text};
}
async function fixture(browser,assets,route,options={}){
 const state={posts:[post(1,'published'),post(2)],moment:null,phrases:[{id:'5',version:1,text:'合成片语',source:'',kind:'sentence',published:false}],friends:[],series:[],ledger:new Map(),lostReplies:{},malformedReplies:{},requests:[],media:new Map(),home:home(),homeItems:[],homeProjects:[],psn:{version:7,connected:true,online_id:'fixture-online',account_id:'fixture-account',auto_sync:false,status:'idle',last_error:'',game_count:0},products:[],failMedia:0,malformedMedia:0,failList:0,failMomentSave:0,...options};
 const page=await browser.newPage({viewport:{width:1200,height:900},reducedMotion:'reduce'});page.setDefaultTimeout(10000);
 if(options.clock)await page.clock.install();
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
  if(url.pathname.startsWith('/media/'))return intercepted.fulfill({contentType:'image/png',body:fs.readFileSync(path.join(__dirname,'fixtures/avatar.png'))});
  const multipart=request.headers()['content-type']?.startsWith('multipart/form-data');
  const bytes=request.postDataBuffer();
  const entry={path:url.pathname,query:url.search,method,headers:request.headers(),body:bytes&&!multipart?JSON.parse(bytes.toString()):null};state.requests.push(entry);
  if(url.pathname==='/api/v1/media'){
   // Ignore the browser-generated multipart boundary when comparing retries.
   const raw=bytes.toString('latin1'),file=raw.slice(raw.indexOf('\r\n\r\n')+4,raw.lastIndexOf('\r\n--'));
   entry.fileHash=createHash('sha256').update(file,'latin1').digest('hex');
   entry.filename=raw.match(/filename="([^"]*)"/)?.[1];
   const key=entry.headers['idempotency-key'];
   if(!state.media.has(key))state.media.set(key,{state:'ready',media_id:'00000000-0000-4000-8000-'+String(state.media.size+1).padStart(12,'0')});
   if(state.failMedia-->0)return send({error:{code:'LOST_REPLY',message:'合成上传已提交但未确认'}},503);
   if(state.malformedMedia-->0)return send({state:'ready',media_id:''});
   return send(state.media.get(key),201);
  }
  const once=(create,status=200)=>{
   const key=method+' '+url.pathname+' '+entry.headers['idempotency-key'],signature=JSON.stringify([entry.body,entry.headers['if-match']]);
   let saved=state.ledger.get(key);
   if(saved&&saved.signature!==signature)return send({error:{code:'IDEMPOTENCY_KEY_REUSED',message:'幂等键对应不同请求'}},409);
   if(!saved){saved={signature,value:copy(create())};state.ledger.set(key,saved)}
   if(state.lostReplies[url.pathname]>0){state.lostReplies[url.pathname]--;return send({error:{code:'FIXTURE_LOST_REPLY',message:'合成已提交但响应中断'}},503)}
   if(state.malformedReplies[url.pathname]>0){state.malformedReplies[url.pathname]--;return send({})}
   return send(saved.value,status);
  };
  if(url.pathname==='/api/v1/admin/home'&&method==='GET')return send(state.home);
  if(url.pathname==='/api/v1/admin/home/items'&&method==='GET'){
   if(state.failHomeKind===url.searchParams.get('kind'))return send({error:{message:'合成筛选失败'}},503);
   const items=state.homeItems.filter(item=>item.kind===url.searchParams.get('kind'));
   return send({...pageOf(items),...(state.homeCursor?{has_more:true,next_cursor:state.homeCursor}:{})});
  }
  if(url.pathname==='/api/v1/admin/home/items'&&method==='POST')return once(()=>{const row={...entry.body,id:String(state.homeItems.length+1),version:1,playtime_minutes:null,source:'manual'};state.homeItems.push(row);return row},201);
  if(url.pathname==='/api/v1/admin/home'&&method==='PUT')return once(()=>{
   const steam={...state.home.steam,...entry.body.steam};
   steam.api_key_configured=!!steam.api_key||(!steam.clear_api_key&&steam.api_key_configured);delete steam.api_key;delete steam.clear_api_key;
   state.home={...state.home,...entry.body,steam,version:state.home.version+1};return state.home;
  });
  if(url.pathname==='/api/v1/admin/home/psn'&&method==='GET')return send(state.psn);
  if(url.pathname==='/api/v1/admin/home/psn'&&method==='PUT')return once(()=>{state.psn={...state.psn,...entry.body,version:state.psn.version+1};return state.psn});
  if(url.pathname==='/api/v1/admin/home/projects'&&method==='GET')return send(pageOf(state.homeProjects));
  if(url.pathname==='/api/v1/admin/home/projects'&&method==='POST')return once(()=>{
   const row={...entry.body,id:String(state.homeProjects.length+1),version:1,title_override:null,description_override:null,url_override:null,language_override:null};state.homeProjects.push(row);return row;
  },201);
  if(url.pathname==='/api/v1/admin/products'&&method==='GET')return send(pageOf(state.products));
  if(url.pathname==='/api/v1/admin/products'&&method==='POST')return once(()=>{const row={...entry.body,id:String(state.products.length+1),version:1};state.products.push(row);return row},201);
  if(url.pathname==='/api/v1/session')return send({authenticated:true,user:{id:'7',username:'fixture',display_name:'合成站长',role:'owner'},csrf_token:'synthetic-csrf'});
  if(url.pathname==='/api/v1/me/posts'){
   if(state.failList-->0)return send({error:{code:'FIXTURE_FAILURE',message:'合成列表加载失败'}},503);
   const status=url.searchParams.get('status'),q=url.searchParams.get('q')||'';
   return send(pageOf(state.posts.filter(post=>(!status||status==='all'||post.status===status)&&post.title.includes(q))));
  }
  if(url.pathname==='/api/v1/me/moments')return send({...pageOf(state.moment?[state.moment]:[]),max_text_length:500,max_images:9,max_image_bytes:10485760});
  if(url.pathname==='/api/v1/moments'&&method==='POST'){state.moment={...moment(),...entry.body,images:(entry.body.images||[]).map(image=>({...image,url:'/media/'+image.media_id}))};return send(state.moment,201)}
  if(url.pathname==='/api/v1/studio/moments/9/draft'){
   if(state.failMomentSave-->0)return send({error:{code:'VERSION_CONFLICT',message:'版本冲突'}},409);
   Object.assign(state.moment,entry.body,{version:state.moment.version+1});return send(state.moment);
  }
  if(url.pathname==='/api/v1/studio/moments/9/publish'){Object.assign(state.moment,{version:state.moment.version+1,published:true,status:'published',public_text:state.moment.text});return send(state.moment)}
  if(url.pathname==='/api/v1/studio/moments/9/publication'){Object.assign(state.moment,{version:state.moment.version+1,published:false,status:'draft',public_text:null});return send(state.moment)}
  if(url.pathname==='/api/v1/studio/moments/9'&&method==='GET')return send(state.moment);
  if(url.pathname==='/api/v1/studio/moments/9'&&method==='DELETE'){state.moment=null;return intercepted.fulfill({status:204,body:''})}
  if(url.pathname==='/api/v1/admin/phrases'&&method==='GET')return send(pageOf(state.phrases));
  if(url.pathname==='/api/v1/admin/phrases'&&method==='POST')return once(()=>{const row={id:'6',version:1,...entry.body};state.phrases.push(row);return row},201);
  if(url.pathname==='/api/v1/admin/phrases/6'&&method==='PUT'){const row=state.phrases.find(row=>row.id==='6');assert.equal(entry.headers['if-match'],'"'+row.version+'"');Object.assign(row,entry.body,{version:row.version+1});return send(row)}
  if(url.pathname==='/api/v1/studio/friends'&&method==='GET')return send(pageOf(state.friends));
  if(url.pathname==='/api/v1/studio/friend-settings')return send({enabled:true,title:'友链',description:'',version:1});
  if(url.pathname==='/api/v1/studio/friends'&&method==='POST')return once(()=>{const row={id:'8',version:1,...entry.body};state.friends.push(row);return row},201);
  if(url.pathname==='/api/v1/studio/friends/8'&&method==='PUT'){const row=state.friends[0];assert.equal(entry.headers['if-match'],'"'+row.version+'"');Object.assign(row,entry.body,{version:row.version+1});return send(row)}
  if(url.pathname==='/api/v1/me/series')return send(pageOf(state.series));
  const seriesResult=(id,version)=>({id,version,...entry.body,slug:entry.body.slug||'synthetic-series',chapters:entry.body.chapter_post_ids.map(id=>({...post(id),post_id:id}))});
  if(url.pathname==='/api/v1/series'&&method==='POST')return once(()=>{const row=seriesResult('10',1);state.series.push(row);return row},201);
  if(url.pathname==='/api/v1/studio/series/10'&&method==='PUT')return once(()=>{const row=state.series[0];assert.equal(entry.headers['if-match'],'"'+row.version+'"');Object.assign(row,seriesResult('10',row.version+1));return row});
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
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
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
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
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
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const {page,state}=await fixture(browser,assets,'/studio/phrases');await page.locator('.phrase-library-row').filter({hasText:'合成片语'}).waitFor();
   check('phrase manager mounts with shared list state',state.requests.filter(r=>r.path==='/api/v1/admin/phrases').length===1);await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const {page,state}=await fixture(browser,assets,'/studio/series');await page.getByRole('button',{name:'新建专栏',exact:true}).click();await page.getByRole('button',{name:'添加文章',exact:true}).click();await page.locator('.series-post-options li').filter({hasText:'文章 1'}).waitFor();
   check('chapter picker uses the same article list client',state.requests.filter(r=>r.path==='/api/v1/me/posts').length===1);await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const path='/api/v1/admin/phrases', {page,state}=await fixture(browser,assets,'/studio/phrases',{lostReplies:{[path]:1}}),form=page.locator('.phrase-composer');
   await form.locator('#phrase-text').fill('首次片语');await form.getByRole('button',{name:'保存草稿',exact:true}).click();await page.getByRole('button',{name:'重试确认保存',exact:true}).waitFor();
   await page.evaluate(()=>document.querySelectorAll('.painter-toast-close').forEach(button=>button.click()));await capture(page,'phrase-pending-desktop.png');await page.setViewportSize({width:390,height:844});await capture(page,'phrase-pending-mobile.png');await page.setViewportSize({width:1200,height:900});
   await form.locator('#phrase-text').fill('后续片语');await form.getByRole('checkbox').check();await page.getByRole('button',{name:'重试确认保存',exact:true}).click();await page.getByText('上次保存已确认，当前新修改仍未保存。',{exact:true}).waitFor();
   const requests=writes(state,path);check('phrase uncertain create retries original key and body exactly',requests.length===2&&requests[0].headers['idempotency-key']===requests[1].headers['idempotency-key']&&JSON.stringify(requests[0].body)===JSON.stringify(requests[1].body));
   check('phrase retry confirms one draft and preserves newer unpublished input',state.phrases.filter(row=>row.id==='6').length===1&&state.phrases.at(-1).text==='首次片语'&&!state.phrases.at(-1).published&&await form.locator('#phrase-text').inputValue()==='后续片语'&&await form.getByRole('checkbox').isChecked());
   await form.getByRole('button',{name:'保存并公开',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.phrase-unsaved')===null);
   check('phrase subsequent update uses acknowledged ID and version',writes(state,'/api/v1/admin/phrases/6').length===1&&state.phrases.at(-1).text==='后续片语'&&state.phrases.at(-1).published);await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const path='/api/v1/studio/friends',{page,state}=await fixture(browser,assets,'/studio/friends',{malformedReplies:{[path]:1}}),form=page.getByRole('form',{name:'友链编辑'});
   await form.getByLabel('网站名称',{exact:true}).fill('首次友链');await form.getByLabel('网站地址',{exact:true}).fill('https://example.org');await form.getByRole('button',{name:'添加友链',exact:true}).click();await page.getByRole('button',{name:'重试确认保存',exact:true}).waitFor();
   await form.getByLabel('网站名称',{exact:true}).fill('后续友链');await form.getByLabel('网站地址',{exact:true}).fill('not-a-url');await page.getByRole('button',{name:'重试确认保存',exact:true}).click();await page.getByText('上次保存已确认，当前新修改仍未保存。',{exact:true}).waitFor();
   const requests=writes(state,path);check('malformed friend acknowledgement retains exact create attempt',requests.length===2&&requests[0].headers['idempotency-key']===requests[1].headers['idempotency-key']&&JSON.stringify(requests[0].body)===JSON.stringify(requests[1].body)&&state.friends.length===1);
   check('friend can confirm old valid command while newer input is invalid',await form.getByLabel('网站名称',{exact:true}).inputValue()==='后续友链'&&await form.getByLabel('网站地址',{exact:true}).inputValue()==='not-a-url'&&state.friends[0].name==='首次友链');
   await form.getByLabel('网站地址',{exact:true}).fill('https://example.net');
   await form.getByRole('button',{name:'保存修改',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.friends-composer .friends-unsaved')===null);
   check('friend update protocol remains versioned PUT',writes(state,'/api/v1/studio/friends/8').length===1&&state.friends[0].name==='后续友链');await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const path='/api/v1/series',{page,state}=await fixture(browser,assets,'/studio/series',{lostReplies:{[path]:1}});
   await page.getByRole('button',{name:'新建专栏',exact:true}).click();const form=page.locator('.series-editor');await form.getByLabel('专栏名称',{exact:true}).fill('首次专栏');await page.getByRole('button',{name:'添加文章',exact:true}).click();await page.locator('.series-post-options li').filter({hasText:'文章 1'}).getByRole('button',{name:'加入',exact:true}).click();
   await form.getByRole('button',{name:'保存专栏',exact:true}).click();await page.getByRole('button',{name:'重试确认保存',exact:true}).waitFor();
   await page.getByRole('button',{name:'新建专栏',exact:true}).click();await confirm(page,'取消');check('cancelled navigation retains uncertain series command',await page.getByRole('button',{name:'重试确认保存',exact:true}).isVisible()&&writes(state,path).length===1);
   await form.getByLabel('专栏名称',{exact:true}).fill('后续专栏');await page.getByRole('button',{name:'重试确认保存',exact:true}).click();await page.getByText('上次保存已确认，当前新修改仍未保存。',{exact:true}).waitFor();
   const requests=writes(state,path);check('series lost reply never creates a second series',requests.length===2&&requests[0].headers['idempotency-key']===requests[1].headers['idempotency-key']&&state.series.length===1&&state.series[0].title==='首次专栏');
   check('series retry preserves title edits and chapter order',await form.getByLabel('专栏名称',{exact:true}).inputValue()==='后续专栏'&&await form.locator('.series-chapter-name strong').innerText()==='文章 1');
   state.lostReplies['/api/v1/studio/series/10']=1;await form.getByRole('button',{name:'保存专栏',exact:true}).click();await page.getByRole('button',{name:'重试确认保存',exact:true}).click();await page.waitForFunction(()=>!document.body.textContent.includes('暂未确认保存结果'));
   const updates=writes(state,'/api/v1/studio/series/10');check('series update retries retain version and command identity',updates.length===2&&updates[0].headers['if-match']==='"1"'&&updates[1].headers['if-match']==='"1"'&&updates[0].headers['idempotency-key']===updates[1].headers['idempotency-key']&&state.series[0].version===2);await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const {page,state}=await fixture(browser,assets,'/me/bookmarks',{failList:1});await page.getByRole('button',{name:'重新加载',exact:true}).click();await page.getByText('文章 1',{exact:true}).waitFor();
   check('workbench bookmark list retries with shared state',state.requests.filter(r=>r.path==='/api/v1/me/bookmarks').length===2);await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const {page,state}=await fixture(browser,assets,'/studio/posts/2');const title=page.locator('.editor-title');await title.waitFor();await title.fill('统一请求后的标题');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await page.waitForFunction(()=>document.querySelector('.save-state')?.textContent.includes('已保存'));
   const save=writes(state,'/api/v1/studio/posts/2/draft')[0];check('editor save preserves draft version and stable request key',save.body.title==='统一请求后的标题'&&save.body.expected_draft_version===2&&save.headers['if-match']==='"3"'&&!!save.headers['idempotency-key']);await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const {page,state}=await fixture(browser,assets,'/studio/moments',{failMedia:1});
   const image={name:'moment.png',mimeType:'image/png',buffer:fs.readFileSync(path.join(__dirname,'fixtures/avatar.png'))};
   await page.locator('.moment-image-upload input[type=file]').setInputFiles(image);
   await page.locator('.moment-image-item [role=alert]').filter({hasText:'合成上传'}).waitFor();
   check('unconfirmed upload does not attach or save a moment',state.moment===null&&writes(state,'/api/v1/moments').length===0);
   await page.locator('.moment-image-item').getByRole('button',{name:'重试上传第 1 张图片',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('.moment-image-item')?.dataset.uploadState==='ready');
   const uploads=writes(state,'/api/v1/media');
   check('moment upload retry keeps the original key and bytes',uploads.length===2&&uploads[0].headers['idempotency-key']===uploads[1].headers['idempotency-key']&&uploads[0].fileHash===uploads[1].fileHash&&uploads[1].filename==='moment.png');
   await page.getByRole('form',{name:'动态编辑'}).getByRole('button',{name:'保存草稿',exact:true}).click();
   await page.locator('.moment-composer-state').filter({hasText:'已保存'}).waitFor();
   check('upload attaches only through explicit draft save',state.moment.images.length===1&&!state.moment.published&&state.media.size===1);
   await page.close();
  }
  if(process.env.CONTENT_MANAGERS_GROUP!=='home'){
   const {page,state}=await fixture(browser,assets,'/studio/products',{malformedMedia:1});
   await page.setViewportSize({width:390,height:844});
   await page.waitForFunction(()=>document.querySelector('.workbench')?.classList.contains('is-phone'));
   await page.getByRole('button',{name:'添加产品',exact:true}).click();
   await page.getByRole('tab',{name:'图片',exact:true}).click();
   await page.getByLabel('选择产品截图',{exact:true}).setInputFiles({name:'product.png',mimeType:'image/png',buffer:fs.readFileSync(path.join(__dirname,'fixtures/avatar.png'))});
   await page.locator('.product-upload-list').getByRole('button',{name:'重试',exact:true}).waitFor();
   check('malformed upload success cannot attach a product screenshot',await page.locator('.product-screenshot-editor li').count()===0&&state.products.length===0);
   await page.locator('.product-upload-list').getByRole('button',{name:'重试',exact:true}).click();
   await page.locator('.product-screenshot-editor li').waitFor();
   check('product upload retry confirms the same server media',state.media.size===1&&writes(state,'/api/v1/media')[0].headers['idempotency-key']===writes(state,'/api/v1/media')[1].headers['idempotency-key']);
   check('product images remain unsaved until explicit product save',writes(state,'/api/v1/admin/products').length===0);
   await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/home');
   await page.getByRole('tab',{name:'展示墙',exact:true}).click();
   await page.getByRole('combobox',{name:'展示墙',exact:true}).click();
   await page.getByRole('option',{name:'个人照片',exact:true}).click();
   await page.locator('.homepage-photo-batch input[type=file]').setInputFiles({name:'home.png',mimeType:'image/png',buffer:fs.readFileSync(path.join(__dirname,'fixtures/avatar.png'))});
   await page.locator('.homepage-photo-batch').getByRole('button',{name:'编辑并保存',exact:true}).waitFor();
   check('homepage photos remain private local attachments until reviewed',state.homeItems.length===0&&state.media.size===1);
   await page.locator('.homepage-photo-batch').getByRole('button',{name:'编辑并保存',exact:true}).click();
   await page.getByRole('button',{name:'保存条目',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('.homepage-photo-queue')===null);
   check('homepage photo save preserves hidden-by-default semantics',state.homeItems.length===1&&state.homeItems[0].visible===false&&!!state.homeItems[0].media_id);
   await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/home',{lostReplies:{'/api/v1/admin/home':1}});
   const name=page.getByLabel('开场问候',{exact:false});
   await name.fill('第一次保存');await page.getByRole('button',{name:'保存首页设置',exact:true}).click();
   await page.getByRole('button',{name:'重试确认保存',exact:true}).waitFor();
   await name.fill('新'.repeat(81));await page.getByRole('button',{name:'重试确认保存',exact:true}).click();
   await page.getByRole('button',{name:'保存首页设置',exact:true}).waitFor();
   const attempts=writes(state,'/api/v1/admin/home');
   check('home settings retry preserves exact body, key and original version',attempts.length===2&&JSON.stringify(attempts[0].body)===JSON.stringify(attempts[1].body)&&attempts[0].headers['idempotency-key']===attempts[1].headers['idempotency-key']&&attempts[1].headers['if-match']==='"1"');
   check('confirming old home settings preserves newer invalid input',state.home.version===2&&state.home.config.sections.intro.title==='第一次保存'&&await name.inputValue()==='新'.repeat(81));
   await name.fill('后续修改');await page.getByRole('button',{name:'保存首页设置',exact:true}).click();
   await page.locator('.homepage-config-form .homepage-savebar').filter({hasText:'首页设置已保存'}).waitFor();
   check('next home settings save uses acknowledged version with new input',writes(state,'/api/v1/admin/home').at(-1).headers['if-match']==='"2"'&&state.home.config.sections.intro.title==='后续修改'&&state.home.version===3);
   await page.getByRole('tab',{name:'游戏账号',exact:true}).click();
   await page.locator('[name=psn-online-id]').fill('fixture-updated');
   await page.getByRole('button',{name:'保存 PlayStation 设置',exact:true}).click();
   await page.locator('.homepage-psn-form .homepage-savebar').filter({hasText:'PlayStation 设置已保存'}).waitFor();
   const psnWrite=writes(state,'/api/v1/admin/home/psn')[0];
   check('PSN saves keep their independent version and payload',psnWrite.headers['if-match']==='"7"'&&psnWrite.body.online_id==='fixture-updated'&&!('steam' in psnWrite.body)&&!('config' in psnWrite.body)&&state.home.version===3);
   await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/home');
   await page.getByRole('tab',{name:'项目卡片',exact:true}).click();
   await page.getByRole('combobox',{name:'项目来源',exact:true}).click();await page.getByRole('option',{name:'手动维护',exact:true}).click();
   const form=page.locator('form').filter({has:page.getByRole('button',{name:'保存项目',exact:true})});
   await form.getByLabel('标题',{exact:false}).fill('独立项目');await form.getByLabel('项目链接',{exact:false}).fill('https://example.com/project');
   await form.getByRole('button',{name:'保存项目',exact:true}).click();
   await page.locator('.homepage-project-list').getByText('独立项目',{exact:true}).waitFor();
   check('extracted project editor preserves hidden defaults and explicit save',state.homeProjects.length===1&&state.homeProjects[0].visible===false&&state.homeProjects[0].source==='manual'&&!!writes(state,'/api/v1/admin/home/projects')[0].headers['idempotency-key']);
   await page.close();
  }
  {
   const {page,state}=await fixture(browser,assets,'/studio/home',{clock:true,psn:{version:7,connected:true,online_id:'fixture-online',account_id:'fixture-account',auto_sync:false,status:'queued',last_error:'',game_count:0}});
   const reads=()=>state.requests.filter(r=>r.path==='/api/v1/admin/home/psn'&&r.method==='GET').length;
   check('PSN does not poll outside its account view',reads()===0);
   await page.getByRole('tab',{name:'游戏账号',exact:true}).click();await page.locator('.homepage-psn-status').waitFor();
   await page.getByRole('tab',{name:'页面内容',exact:true}).click();await page.clock.runFor(3100);
   check('leaving game accounts stops PSN polling',reads()===1);
   await page.getByRole('tab',{name:'游戏账号',exact:true}).click();await page.locator('.homepage-psn-status').waitFor();
   await page.waitForFunction(()=>!document.querySelector('input[name=psn-online-id]')?.disabled);
   const polled=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/v1/admin/home/psn');await page.clock.runFor(3100);await polled;
   check('returning to game accounts resumes PSN state reads',reads()>=3);
   await page.evaluate(()=>window.unmountFixture());const before=reads();await page.clock.runFor(6100);
   check('unmounting homepage stops the PSN timer and further reads',reads()===before);
   await page.close();
  }
  {
   const items=Array.from({length:20},(_,index)=>({id:String(index+1),version:1,kind:'steam',platform:'steam',platforms:[],title:'游戏 '+index,subtitle:'',alt:'',url:'https://store.steampowered.com/app/1',image_url:'',hero_url:'',logo_url:'',media_id:'',provider_id:'1',position:index,visible:false,playtime_minutes:null,source:'manual'}));
   const {page,state}=await fixture(browser,assets,'/studio/home',{homeItems:items,homeCursor:'games-next',failHomeKind:'photos'});
   await page.getByRole('tab',{name:'展示墙',exact:true}).click();
   await page.getByRole('combobox',{name:'展示墙',exact:true}).click();await page.getByRole('option',{name:'个人照片',exact:true}).click();
   await page.getByRole('button',{name:'刷新条目',exact:true}).waitFor();
   check('failed wall filter invalidates previous pagination',await page.getByRole('button',{name:'重试载入',exact:true}).count()===0&&state.requests.filter(r=>r.path==='/api/v1/admin/home/items'&&r.query.includes('kind=photos')).every(r=>!new URLSearchParams(r.query).has('cursor')));
   state.failHomeKind='';state.homeCursor='';await page.getByRole('button',{name:'刷新条目',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('.homepage-items-table')?.textContent.includes('还没有'));
   check('wall filter retry replaces previous game rows with photo results',await page.locator('.homepage-items-table tbody tr').count()===1);
   await page.close();
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
