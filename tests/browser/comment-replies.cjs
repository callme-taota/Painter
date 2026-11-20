// Source-only discussion regression: compile the real Vue component graph and
// intercept every request. No running Painter, database, accounts or network.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{createRequire}=require('node:module');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const frontend=createRequire(path.resolve(__dirname,'../../web/frontend/package.json'));
const {build}=frontend('esbuild'),{parse,compileScript}=frontend('@vue/compiler-sfc');
const src=path.resolve(__dirname,'../../web/frontend/src');
const out=process.env.ARTIFACT_DIR||'/tmp/painter-comment-replies-20261120/source-browser';
fs.mkdirSync(out,{recursive:true});
const checks=[],errors=[],fixtures=[];
function check(name,value){assert.ok(value,name);checks.push(name);console.log('PASS '+name)}
const clone=value=>JSON.parse(JSON.stringify(value));
const frame=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
const user=(id,name)=>({id,display_name:name,avatar_url:''});
const author=user('8','作者 Alice'),friend=user('9','朋友 Bob'),reader=user('7','读者 Carol');
function comment(id,body,who=author,extra={}){return{id,body,user:who,created_at:'2026-11-20T01:00:00Z',like_count:0,liked:false,is_self:who.id===reader.id,is_author:who.id===author.id,parent_id:null,reply_to:null,reply_to_deleted:false,reply_count:0,thread_reply_count:0,...extra}}
function defaultState(options={}){
 const roots=[comment('101','一条有回复的根评论',author,{reply_count:2,thread_reply_count:2}),comment('102','另一段讨论',friend)];
 const replies={'101':[comment('201','第一条回复',friend,{parent_id:'101',reply_to:{id:'101',user:author},thread_reply_count:2}),comment('202','回复另一条回复',author,{parent_id:'101',reply_to:{id:'201',user:friend},thread_reply_count:2})]};
 return{authenticated:true,enabled:true,roots,replies,requests:[],postOutcomes:[],saved:new Map(),nextID:300,failReplyReads:0,...options};
}
async function bundle(){
 const result=await build({
  stdin:{contents:"import'./style.css';import'./reading.css';import'./shadcn-theme.css';import'./infiniteScroll.css';import{createApp}from'vue';import PostIsland from'./components/PostIsland.vue';window.fixtureApp=createApp(PostIsland,{postId:'42'});window.fixtureApp.mount('#fixture');",resolveDir:src,loader:'ts'},
  bundle:true,write:false,outfile:'/fixture.js',format:'esm',platform:'browser',loader:{'.woff2':'dataurl'},
  define:{__VUE_OPTIONS_API__:'true',__VUE_PROD_DEVTOOLS__:'false',__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:'false'},
  plugins:[{name:'real-vue-sfc',setup(build){build.onLoad({filter:/\.vue$/},async args=>{const{descriptor,errors}=parse(fs.readFileSync(args.path,'utf8'),{filename:args.path});assert.equal(errors.length,0,args.path);const compiled=compileScript(descriptor,{id:path.basename(args.path),inlineTemplate:true});return{contents:compiled.content,loader:'ts',resolveDir:path.dirname(args.path)}})}}],
 });
 return{js:result.outputFiles.find(file=>file.path.endsWith('.js')).text,css:result.outputFiles.find(file=>file.path.endsWith('.css')).text};
}
async function fixture(browser,assets,options={}){
 const state=defaultState(options),page=await browser.newPage({viewport:{width:1000,height:900},reducedMotion:'reduce'});
 if(state.holdNextReplyRead||state.holdReplyCursor)state.replyReadHeld=new Promise(resolve=>{state.noteReplyReadHeld=resolve});
 page.setDefaultTimeout(7000);
 page.on('pageerror',error=>errors.push(error.message));
 const send=(route,status,data)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
 await page.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url()),method=request.method();
  if(url.origin!=='https://comment-replies.test'){errors.push('Unexpected external request: '+request.url());return route.abort()}
  if(url.pathname==='/fixture.js')return route.fulfill({contentType:'text/javascript',body:assets.js});
  if(url.pathname==='/fixture.css')return route.fulfill({contentType:'text/css',body:assets.css});
  if(url.pathname==='/')return route.fulfill({contentType:'text/html',body:'<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><style>main{width:calc(100% - 32px);max-width:760px;margin:24px auto}.article-discussion{margin:0!important;width:100%!important}</style><main><section class="article-discussion"><div id="fixture"></div></section></main><script type="module" src="/fixture.js"></script></html>'});
  const entry={method,path:url.pathname,query:url.search,headers:request.headers(),body:request.postData()?JSON.parse(request.postData()):null};state.requests.push(entry);
  if(url.pathname==='/api/v1/session')return send(route,200,state.authenticated?{authenticated:true,user:{...reader,role:state.owner?'owner':'reader'},csrf_token:'fixture-csrf'}:{authenticated:false});
  if(url.pathname==='/api/v1/site')return send(route,200,{comments_enabled:state.enabled,collections_enabled:true});
  if(url.pathname==='/api/v1/posts/42/metrics')return send(route,200,{like_count:0,comment_count:state.roots.length});
  if(url.pathname==='/api/v1/me/post-states')return send(route,200,{items:[{post_id:'42',liked:false,bookmarked:false}]});
  if(url.pathname==='/api/v1/posts/42/comments'&&method==='GET'){
   if(state.failRootReads>0){state.failRootReads--;return send(route,503,{error:{code:'FIXTURE_ROOT_READ_FAILED',message:'讨论刷新暂不可用'}})}
   const cursor=url.searchParams.get('cursor')||'';
   if(state.rootPages)return send(route,200,state.rootPages[cursor]||{items:[],has_more:false,next_cursor:''});
   return send(route,200,{items:state.roots,has_more:false,next_cursor:''});
  }
  const match=url.pathname.match(/^\/api\/v1\/comments\/(\d+)\/replies$/);
  if(match&&method==='GET'){
   const cursor=url.searchParams.get('cursor')||'';
   if(state.holdNextReplyRead||state.holdReplyCursor===cursor){state.holdNextReplyRead=false;state.holdReplyCursor=null;const snapshot=clone(state.replyPages?.[match[1]]?.[cursor]||{items:state.replies[match[1]]||[],has_more:false,next_cursor:''});return new Promise(resolve=>{state.releaseReplyRead=async()=>{try{await send(route,200,snapshot)}catch{}resolve()};state.noteReplyReadHeld()})}
   if(state.failReplyReads>0){state.failReplyReads--;return send(route,503,{error:{code:'FIXTURE_READ_FAILED',message:'读取暂不可用'}})}
   if(state.replyPages?.[match[1]])return send(route,200,state.replyPages[match[1]][cursor]||{items:[],has_more:false,next_cursor:''});
   return send(route,200,{items:state.replies[match[1]]||[],has_more:false,next_cursor:''});
  }
  if(url.pathname==='/api/v1/posts/42/comments'&&method==='POST'){
   const outcome=state.postOutcomes.shift();
   if(outcome&&outcome!==201)return send(route,outcome,{error:{code:outcome===404?'NOT_FOUND':'FIXTURE_WRITE_FAILED',message:outcome===404?'评论已不可用':'网络暂不可用，请重试'}});
   const key=entry.headers['idempotency-key'];
   if(state.saved.has(key))return send(route,201,state.saved.get(key));
   const target=[...state.roots,...Object.values(state.replies).flat()].find(item=>item.id===entry.body.reply_to_id);
   if(entry.body.reply_to_id&&!target)return send(route,404,{error:{code:'NOT_FOUND',message:'评论已不可用'}});
   const root=target?state.roots.find(item=>item.id===(target.parent_id||target.id)):null;
   const created=comment(String(++state.nextID),entry.body.body,reader,target?{parent_id:root.id,reply_to:{id:target.id,user:{id:target.user.id,display_name:target.user.display_name}}}:{});
   if(root){root.reply_count++;root.thread_reply_count=root.reply_count;created.thread_reply_count=root.reply_count;(state.replies[root.id]||=[]).push(created);for(const reply of state.replies[root.id])reply.thread_reply_count=root.reply_count}else state.roots.push(created);
   state.saved.set(key,clone(created));if(state.failReadAfterPost)state.failReplyReads++;
   return send(route,201,created);
  }
  const deleted=url.pathname.match(/^\/api\/v1\/comments\/(\d+)$/);
  if(deleted&&method==='DELETE'){
   const id=deleted[1],target=[...state.roots,...Object.values(state.replies).flat()].find(item=>item.id===id);
   if(!target)return send(route,404,{error:{code:'NOT_FOUND',message:'评论已不可用'}});
   if(target.parent_id){
    const root=state.roots.find(item=>item.id===target.parent_id),redact=items=>items.filter(item=>item.id!==id).map(item=>{if(item.reply_to?.id===id){item.reply_to=null;item.reply_to_deleted=true}item.thread_reply_count=root.reply_count;return item});
    root.reply_count=Math.max(0,root.reply_count-1);root.thread_reply_count=root.reply_count;state.replies[root.id]=redact(state.replies[root.id]);
    for(const data of Object.values(state.replyPages?.[root.id]||{}))data.items=redact(data.items);
   }else{state.roots=state.roots.filter(item=>item.id!==id);delete state.replies[id];if(state.failRootReadAfterDelete)state.failRootReads=1}
   return route.fulfill({status:204,body:''});
  }
  errors.push('Unexpected fixture API: '+method+' '+url.pathname);return send(route,500,{error:{message:'Unexpected fixture request'}});
 });
 await page.goto('https://comment-replies.test/');
 await page.locator('#comment-101').waitFor();
 await page.locator('.discussion-loading').waitFor({state:'hidden'});
 if(state.authenticated&&state.enabled)await page.locator('[data-comment-reply]').first().waitFor({state:'visible'});
 await frame(page);fixtures.push({page,state});return{page,state};
}
const posts=state=>state.requests.filter(request=>request.method==='POST');
const replyButton=(page,id)=>page.locator('#comment-'+id+' [data-comment-reply]');
const composer=(page,id)=>page.locator('[data-reply-to="'+id+'"]');
async function openReplies(page){await page.locator('[data-thread-id="101"]>.discussion-replies-toggle').click();await page.locator('#comment-201').waitFor()}
async function submitReply(page,id,text){await replyButton(page,id).click();await composer(page,id).locator('textarea').fill(text);await composer(page,id).getByRole('button',{name:'发表回复',exact:true}).click()}

(async()=>{const assets=await bundle(),browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});try{
 {
  const{page,state}=await fixture(browser,assets,{authenticated:false});
  check('guest sees roots without mutation controls',await page.locator('.discussion-actions,.discussion-composer,.interaction-bar,[data-reply-to]').count()===0);
  await openReplies(page);check('guest can expand and read a one-level thread',await page.locator('.discussion-replies>.discussion-comment').count()===2&&await page.locator('#comment-202 .discussion-reply-context').textContent().then(text=>text.includes(friend.display_name)));
  await page.locator('.discussion-replies-toggle').click();await page.locator('.discussion-replies-toggle').click();
  check('reopening a loaded thread reuses the read without writes',state.requests.filter(request=>request.path.endsWith('/replies')).length===1&&posts(state).length===0);
  await page.screenshot({path:path.join(out,'guest-expanded.png'),fullPage:true});await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets);await openReplies(page);
  await submitReply(page,'101','根评论的回复');await page.locator('#comment-301').waitFor();
  await submitReply(page,'201','子回复的回复 <img src=x onerror=alert(1)>');await page.locator('#comment-302').waitFor();
  check('root and child reply requests preserve their actual targets',posts(state).map(request=>request.body.reply_to_id).join(',')==='101,201');
  check('nested answers remain one level and preserve plain text',await page.locator('.discussion-replies>.discussion-comment').count()===4&&await page.locator('#comment-302 .discussion-reply-context').textContent().then(text=>text.includes(friend.display_name))&&await page.locator('#comment-302 img').count()===0);
  check('writes use session CSRF and distinct idempotency keys',posts(state).every(request=>request.headers['x-csrf-token']==='fixture-csrf'&&request.headers['idempotency-key']?.length>=8)&&posts(state)[0].headers['idempotency-key']!==posts(state)[1].headers['idempotency-key']);
  check('author badge reflects projection rather than current reader',await page.locator('#comment-101 .discussion-author-badge').count()===1&&await page.locator('#comment-301 .discussion-author-badge').count()===0);
  await replyButton(page,'201').click();await composer(page,'201').locator('textarea').fill('暂存草稿');await composer(page,'201').locator('textarea').press('Escape');await frame(page);
  check('Escape returns focus to the triggering reply action',await replyButton(page,'201').evaluate(element=>element===document.activeElement));
  await replyButton(page,'201').click();check('cancelled draft remains attached to its target',await composer(page,'201').locator('textarea').inputValue()==='暂存草稿');
  await page.screenshot({path:path.join(out,'authenticated-replies.png'),fullPage:true});await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{postOutcomes:[503,201]});
  await submitReply(page,'101','网络失败后保留的回复');await page.locator('.painter-toast-message').filter({hasText:'网络暂不可用'}).waitFor();
  check('failed POST preserves the draft and enabled composer',await composer(page,'101').locator('textarea').inputValue()==='网络失败后保留的回复'&&await composer(page,'101').getByRole('button',{name:'发表回复',exact:true}).isEnabled());
  await composer(page,'101').getByRole('button',{name:'发表回复',exact:true}).click();await page.locator('#comment-301').waitFor();
  check('unchanged retry reuses the exact idempotency key',posts(state).length===2&&posts(state)[0].headers['idempotency-key']===posts(state)[1].headers['idempotency-key']);
  check('retry renders one saved reply and uses authoritative total',await page.locator('#comment-301').count()===1&&state.roots[0].reply_count===3&&await page.locator('[data-reply-to]').count()===0);
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{failReadAfterPost:true});
  await submitReply(page,'102','写入成功，读取失败');await page.locator('[data-thread-id="102"] .discussion-replies-error').waitFor();
  check('successful write clears composer even when thread GET fails',posts(state).length===1&&await composer(page,'102').count()===0&&await page.locator('.painter-toast-message').textContent().then(text=>text.includes('回复已发表')));
  await page.locator('[data-thread-id="102"] .discussion-replies-error button').click();await page.locator('#comment-301').waitFor();
  check('read retry after successful POST never writes again',posts(state).length===1&&await page.locator('#comment-301').count()===1);
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{holdNextReplyRead:true});
  await page.locator('[data-thread-id="101"]>.discussion-replies-toggle').click();await state.replyReadHeld;
  await submitReply(page,'101','读取进行中发表的新回复');await page.locator('#comment-301').waitFor();
  await state.releaseReplyRead();await frame(page);
  check('late pre-write reply GET cannot replace the fresh post-write thread',await page.locator('#comment-301').count()===1&&await page.locator('.discussion-replies>.discussion-comment').count()===3&&posts(state).length===1);
  check('writing during a pending read requests a fresh reply projection',state.requests.filter(request=>request.path==='/api/v1/comments/101/replies').length===2);
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{postOutcomes:[404]});
  await submitReply(page,'101','目标不可用时的草稿');await page.locator('.painter-toast-message').filter({hasText:'回复内容已保留'}).waitFor();
  check('target404 keeps draft with an actionable unavailable message',await composer(page,'101').locator('textarea').inputValue()==='目标不可用时的草稿'&&posts(state).length===1);
  await composer(page,'101').getByRole('button',{name:'取消',exact:true}).click();await replyButton(page,'102').click();
  check('switching target does not leak another target draft',await composer(page,'102').locator('textarea').inputValue()==='');await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets);await replyButton(page,'102').click();const input=composer(page,'102').locator('textarea');
  await input.fill('😀'.repeat(5001));
  check('5001 Unicode code points report limit and disable submit',await input.getAttribute('aria-invalid')==='true'&&await composer(page,'102').getByRole('button',{name:'发表回复',exact:true}).isDisabled()&&await composer(page,'102').locator('small').textContent()==='5001 / 5000');
  await composer(page,'102').evaluate(form=>form.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true})));await frame(page);check('programmatic oversize submit is guarded before API',posts(state).length===0);
  await input.fill('😀'.repeat(5000));check('5000 emoji remain valid despite10000 UTF16 units',await input.getAttribute('aria-invalid')==='false'&&await composer(page,'102').getByRole('button',{name:'发表回复',exact:true}).isEnabled());
  await composer(page,'102').getByRole('button',{name:'发表回复',exact:true}).click();await page.locator('#comment-301').waitFor();check('valid Unicode boundary reaches API without truncation',Array.from(posts(state)[0].body.body).length===5000&&posts(state)[0].body.body.length===10000);await page.close();
 }
 {
  const huge='分页前的长回复。'.repeat(800),r1=comment('201',huge,friend,{parent_id:'101',reply_to:{id:'101',user:author},thread_reply_count:2}),r2=comment('202','第二页回复',author,{parent_id:'101',reply_to:null,reply_to_deleted:true,thread_reply_count:2});
  const root=comment('101','分页讨论',author,{reply_count:2}),nextRoot=comment('102','第二页根评论',friend);
  const{page,state}=await fixture(browser,assets,{roots:[root,nextRoot],rootPages:{'':{items:[root],has_more:true,next_cursor:'roots-two'},'roots-two':{items:[root,nextRoot],has_more:false,next_cursor:''}},replyPages:{'101':{'':{items:[r1],has_more:true,next_cursor:'replies-two'},'replies-two':{items:[r1,r2],has_more:false,next_cursor:''}}}});
  await page.locator('#comment-102').waitFor();await openReplies(page);await page.locator('#replies-101 [data-infinite-sentinel]').scrollIntoViewIfNeeded();await page.locator('#comment-202').waitFor();
  check('root and reply infinite pagination send their own cursors',state.requests.some(request=>request.query==='?cursor=roots-two')&&state.requests.some(request=>request.query==='?cursor=replies-two'));
  check('pagination deduplicates overlaps within each collection',await page.locator('.discussion-thread').count()===2&&await page.locator('#comment-201').count()===1&&await page.locator('#comment-202').count()===1);
  check('unavailable reply target renders no stale identity',await page.locator('#comment-202 .discussion-reply-context').textContent().then(text=>text.includes('原评论已不可用')));await page.close();
 }
 {
  const longUser=user('8','长昵称'.repeat(60)),root=comment('101','https://example.invalid/'+('long-unbroken-segment'.repeat(50)),longUser,{reply_count:1}),reply=comment('201','多层讨论也保持阅读宽度 '+('W'.repeat(150)),friend,{parent_id:'101',reply_to:{id:'101',user:longUser},thread_reply_count:1});
  const{page}=await fixture(browser,assets,{roots:[root],replies:{'101':[reply]}});await openReplies(page);await replyButton(page,'201').click();await composer(page,'201').locator('textarea').fill('手机键盘前的回复草稿');
  for(const width of [320,390]){await page.setViewportSize({width,height:844});await page.evaluate(width=>document.documentElement.dataset.theme=width===390?'dark':'light',width);await frame(page);
   const layout=await page.evaluate(()=>({width:innerWidth,scroll:document.documentElement.scrollWidth,buttons:[...document.querySelectorAll('.article-community button')].filter(element=>element.getClientRects().length).map(element=>({text:element.textContent,box:{width:element.getBoundingClientRect().width,height:element.getBoundingClientRect().height}}))}));
   check(width+'px mobile has no horizontal overflow',layout.scroll<=layout.width+1);
   check(width+'px mobile actions retain44px touch targets',layout.buttons.every(button=>button.box.width>=43.9&&button.box.height>=43.9));await page.screenshot({path:path.join(out,'mobile-'+width+'.png'),fullPage:true});
  }await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{owner:true});await openReplies(page);
  await page.locator('#comment-201').getByRole('button',{name:'删除',exact:true}).click();
  await page.getByRole('dialog',{name:'删除这条评论？'}).getByRole('button',{name:'删除评论',exact:true}).click();await page.locator('#comment-201').waitFor({state:'detached'});
  const context=page.locator('#comment-202 .discussion-reply-context');
  check('deleting a reply redacts its identity from surviving references',await context.textContent().then(text=>text.includes('原评论已不可用')&&!text.includes(friend.display_name))&&await context.locator('a').count()===0);
  check('deleting a reply keeps its root and other replies visible',await page.locator('#comment-101').count()===1&&await page.locator('#comment-202').count()===1&&state.requests.filter(request=>request.method==='DELETE').length===1);
  await page.close();
 }
 {
  const r1=comment('201','挂起追加页之前的长回复。'.repeat(700),friend,{parent_id:'101',reply_to:{id:'101',user:author},thread_reply_count:3}),r2=comment('202','保留的引用回复',author,{parent_id:'101',reply_to:{id:'201',user:friend},thread_reply_count:3}),r3=comment('203','追加页中的另一条回复',reader,{parent_id:'101',reply_to:{id:'101',user:author},thread_reply_count:3}),root=comment('101','删除期间有挂起的追加请求',author,{reply_count:3});
  const{page,state}=await fixture(browser,assets,{owner:true,roots:[root],replies:{'101':[r1,r2,r3]},holdReplyCursor:'delete-append',replyPages:{'101':{'':{items:[r1,r2],has_more:true,next_cursor:'delete-append'},'delete-append':{items:[r1,r2,r3],has_more:false,next_cursor:''}}}});
  await openReplies(page);await page.locator('#replies-101 [data-infinite-sentinel]').scrollIntoViewIfNeeded();await state.replyReadHeld;
  await page.locator('#comment-201').getByRole('button',{name:'删除',exact:true}).click();await page.getByRole('dialog',{name:'删除这条评论？'}).getByRole('button',{name:'删除评论',exact:true}).click();await page.locator('#comment-201').waitFor({state:'detached'});
  await state.releaseReplyRead();await frame(page);
  check('late append GET cannot revive a reply deleted while it was pending',await page.locator('#comment-201').count()===0&&await page.locator('#comment-202').count()===1);
  check('late append GET cannot restore a deleted target identity',await page.locator('#comment-202 .discussion-reply-context').textContent().then(text=>text.includes('原评论已不可用')&&!text.includes(friend.display_name)));
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{owner:true,failRootReadAfterDelete:true});await openReplies(page);
  await page.locator('#comment-101').getByRole('button',{name:'删除',exact:true}).click();const dialog=page.getByRole('dialog',{name:'删除这条评论？'});
  check('root deletion confirmation explains thread visibility consequences',await dialog.textContent().then(text=>text.includes('这条评论和其下回复将不再公开展示'))&&state.requests.filter(request=>request.method==='DELETE').length===0);
  await dialog.getByRole('button',{name:'取消',exact:true}).click();check('cancelling root deletion preserves thread without a DELETE',await page.locator('#comment-101').count()===1&&await page.locator('#comment-201').count()===1&&state.requests.filter(request=>request.method==='DELETE').length===0);
  await page.locator('#comment-101').getByRole('button',{name:'删除',exact:true}).click();await page.getByRole('dialog',{name:'删除这条评论？'}).getByRole('button',{name:'删除评论',exact:true}).click();await page.locator('.notice.error').waitFor();
  check('root DELETE removes local root and replies even when refresh GET fails',await page.locator('[data-thread-id="101"],#comment-201,#comment-202').count()===0&&await page.locator('#comment-102').count()===1&&state.requests.filter(request=>request.method==='DELETE').length===1);
  check('failed refresh reports completed deletion without resurrecting content',await page.locator('.notice.error').textContent().then(text=>text.includes('评论已删除'))&&state.requests.filter(request=>request.method==='GET'&&request.path==='/api/v1/posts/42/comments').length===2);
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{enabled:false});await openReplies(page);check('feature off preserves public thread while hiding new reply controls',await page.locator('#comment-201').count()===1&&await page.locator('[data-comment-reply],.discussion-composer,[data-reply-to]').count()===0&&posts(state).length===0);await page.close();
 }
 check('real component graph completed without script errors or unmocked traffic',errors.length===0);
}finally{for(const fixture of fixtures)if(!fixture.page.isClosed())await fixture.page.close();fs.writeFileSync(path.join(out,'comment-replies-report.json'),JSON.stringify({checks,errors,requests:fixtures.map(({state})=>state.requests.map(({method,path,query})=>({method,path,query})))},null,2));await browser.close()}})().catch(error=>{console.error(error);process.exitCode=1});
