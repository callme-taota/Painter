// Real Vue source, synthetic SSR bootstrap, and fully intercepted requests.
// This suite never contacts a running Painter, database or external service.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{createRequire}=require('node:module');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const frontend=createRequire(path.resolve(__dirname,'../../web/frontend/package.json'));
const {build}=frontend('esbuild'),{parse,compileScript}=frontend('@vue/compiler-sfc');
const src=path.resolve(__dirname,'../../web/frontend/src');
const out=process.env.ARTIFACT_DIR||'/tmp/painter-following-feed-20261121/source-browser';
fs.mkdirSync(out,{recursive:true});
const checks=[],errors=[],fixtures=[];
function check(name,value){assert.ok(value,name);checks.push(name);console.log('PASS '+name)}
const clone=value=>JSON.parse(JSON.stringify(value));
const frame=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function until(predicate,message){const deadline=Date.now()+7000;while(!predicate()){if(Date.now()>deadline)throw new Error(message);await new Promise(resolve=>setTimeout(resolve,10))}}
function item(type,id,extra={}){return{id:type+':'+id,content_id:String(id),type,url:'/'+(type==='post'?'posts':'moments')+'/'+id,title:type==='post'?'文章 '+id:'',summary:type==='post'?'公开文章的摘要':'公开动态的文字',cover_url:'',image_count:0,published_at:'2026-11-21T01:00:00Z',author:{id:'8',display_name:'朋友 Alice',avatar_url:''},...extra}}
function feed(items=[item('post',1),item('moment',1)],extra={}){return{items,next_cursor:'',has_more:false,type:'all',available_types:['post','moment'],following_count:1,viewer_id:'7',...extra}}
const feedRequests=state=>state.requests.filter(request=>request.path==='/api/v1/me/feed');
async function bundle(){
 const result=await build({
  stdin:{contents:"import'./style.css';import'./shadcn-theme.css';import'./infiniteScroll.css';import'./followingFeed.css';import{createApp}from'vue';import FollowingFeed from'./components/FollowingFeed.vue';window.fixtureApp=createApp(FollowingFeed,{initial:window.fixtureInitial,viewerId:'7'});window.fixtureApp.mount('[data-following-feed]');",resolveDir:src,loader:'ts'},
  bundle:true,write:false,outfile:'/fixture.js',format:'esm',platform:'browser',loader:{'.woff2':'dataurl'},
  define:{__VUE_OPTIONS_API__:'true',__VUE_PROD_DEVTOOLS__:'false',__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:'false'},
  plugins:[{name:'real-vue-sfc',setup(build){build.onLoad({filter:/\.vue$/},async args=>{const{descriptor,errors}=parse(fs.readFileSync(args.path,'utf8'),{filename:args.path});assert.equal(errors.length,0,args.path);return{contents:compileScript(descriptor,{id:path.basename(args.path),inlineTemplate:true}).content,loader:'ts',resolveDir:path.dirname(args.path)}})}}],
 });
 return{js:result.outputFiles.find(file=>file.path.endsWith('.js')).text,css:result.outputFiles.find(file=>file.path.endsWith('.css')).text};
}
async function fixture(browser,assets,options={}){
 const state={initial:feed(),session:{authenticated:true,user:{id:'7',role:'reader'},csrf_token:'synthetic-csrf'},feedPlans:[],sessionPlans:[],requests:[],redirects:[],holds:{},aborted:[],...options};
 const page=await browser.newPage({viewport:{width:1100,height:900},reducedMotion:'reduce'});page.setDefaultTimeout(7000);
 // Keep the old document inspectable at the privacy boundary. Aborting a
 // document request would otherwise replace it with Chromium's error page.
 await page.addInitScript(()=>{window.fixtureNavigations=[];navigation.addEventListener('navigate',event=>{const url=new URL(event.destination.url);if(url.pathname==='/login'){event.preventDefault();window.fixtureNavigations.push(url.pathname+url.search)}})});
 page.on('pageerror',error=>errors.push(error.message));
 page.on('requestfailed',request=>{if(new URL(request.url()).pathname==='/api/v1/me/feed')state.aborted.push(request.url())});
 const send=(route,status,data)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
 function reply(route,plan){
  const status=plan.status||200,data=clone(plan.data||{error:{code:'FIXTURE_FAILURE',message:'合成网络错误'}});
  if(plan.hold)return new Promise(resolve=>{state.holds[plan.hold]={release:async()=>{try{await send(route,status,data)}catch{}resolve()}}});
  return send(route,status,data);
 }
 await page.route('**/*',async route=>{
  const request=route.request(),url=new URL(request.url());
  if(url.origin!=='https://following-feed.test'){errors.push('Unexpected external request: '+request.url());return route.abort()}
  if(url.pathname==='/fixture.js')return route.fulfill({contentType:'text/javascript',body:assets.js});
  if(url.pathname==='/fixture.css')return route.fulfill({contentType:'text/css',body:assets.css});
  if(url.pathname==='/fixture-cover.svg')return route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="#68826b"/><circle cx="240" cy="70" r="40" fill="#d4d4ae"/></svg>'});
  if(url.pathname==='/login'){state.redirects.push(url.pathname+url.search);return route.abort()}
  if(url.pathname==='/following'){
   const initial=JSON.stringify(state.initial).replace(/</g,'\\u003c');
   return route.fulfill({contentType:'text/html',body:'<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><body data-page-kind="following"><main class="wrap site-main"><div data-following-feed></div></main><script>window.fixtureInitial='+initial+'</script><script type="module" src="/fixture.js"></script></body></html>'});
  }
  const entry={method:request.method(),path:url.pathname,query:url.search};state.requests.push(entry);
  if(entry.method!=='GET'){errors.push('Unexpected mutation: '+entry.method+' '+entry.path);return send(route,500,{error:{message:'Read-only fixture'}})}
  if(url.pathname==='/api/v1/session')return reply(route,state.sessionPlans.shift()||{data:state.session});
  if(url.pathname==='/api/v1/me/feed'){
   const kind=url.searchParams.get('type')||'all',cursor=url.searchParams.get('cursor')||'';
   const index=state.feedPlans.findIndex(plan=>(plan.kind===undefined||plan.kind===kind)&&(plan.cursor===undefined||plan.cursor===cursor));
   const plan=index<0?{data:feed([item(kind==='moment'?'moment':'post',9)],{type:kind})}:state.feedPlans.splice(index,1)[0];
   return reply(route,plan);
  }
  errors.push('Unexpected fixture API: '+entry.method+' '+entry.path);return send(route,500,{error:{message:'Unexpected fixture request'}});
 });
 await page.goto('https://following-feed.test/following'+(state.query||''));
 await page.getByRole('button',{name:'刷新',exact:true}).waitFor();
 await page.waitForFunction(()=>{const root=document.querySelector('[data-following-feed]'),refresh=root?.querySelector('.following-refresh');return root&&!root.hidden&&refresh&&!refresh.disabled});
 await frame(page);fixtures.push({page,state});return{page,state};
}
const keys=page=>page.locator('.following-item').evaluateAll(items=>items.map(item=>item.dataset.listKey));
const filter=(page,name)=>page.getByRole('navigation',{name:'内容类型'}).getByRole('link',{name,exact:true});
async function release(state,key){await until(()=>state.holds[key],'held request did not start: '+key);await state.holds[key].release()}
async function assertPrivateRedirect(page,state){await page.waitForFunction(()=>window.fixtureNavigations.length>0);state.redirects=await page.evaluate(()=>window.fixtureNavigations);await frame(page);return state.redirects[0]==='/login?return_to=%2Ffollowing'&&await page.locator('[data-following-feed]').evaluate(root=>root.hidden)&&await page.locator('.following-item').count()===0}

(async()=>{const assets=await bundle(),browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});try{
 {
  const{page,state}=await fixture(browser,assets);
  check('SSR bootstrap verifies session without fetching the first feed again',state.requests.filter(request=>request.path==='/api/v1/session').length===1&&feedRequests(state).length===0);
  check('same numeric ID in post and moment remains two navigable entries',JSON.stringify(await keys(page))===JSON.stringify(['post:1','moment:1'])&&await page.locator('.following-item-post h2 a').getAttribute('href')==='/posts/1'&&await page.locator('.following-moment-text').getAttribute('href')==='/moments/1');
  await page.screenshot({path:path.join(out,'following-desktop.png'),fullPage:true});await page.close();
 }
 {
  const initial=feed(undefined,{has_more:true,next_cursor:'page-two'});
  const{page,state}=await fixture(browser,assets,{initial,feedPlans:[{cursor:'page-two',status:503},{cursor:'page-two',data:feed([item('moment',1),item('post',2)])}]});
  await page.getByRole('button',{name:'重试载入',exact:true}).waitFor();
  check('append failure preserves visible bootstrap entries and cursor context',JSON.stringify(await keys(page))===JSON.stringify(['post:1','moment:1'])&&feedRequests(state).length===1&&new URLSearchParams(feedRequests(state)[0].query).get('cursor')==='page-two');
  await page.getByRole('button',{name:'重试载入',exact:true}).click();await page.locator('[data-list-key="post:2"]').waitFor();
  check('append retry uses the same cursor and deduplicates only matching composite IDs',JSON.stringify(await keys(page))===JSON.stringify(['post:1','moment:1','post:2'])&&feedRequests(state).length===2&&feedRequests(state).every(request=>new URLSearchParams(request.query).get('cursor')==='page-two'));
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{feedPlans:[{kind:'post',hold:'old-post-filter',data:feed([item('post',20)],{type:'post'})},{kind:'moment',data:feed([item('moment',21)],{type:'moment'})}]});
  await filter(page,'文章').click();await until(()=>state.holds['old-post-filter'],'post filter was not requested');await filter(page,'动态').click();await page.locator('[data-list-key="moment:21"]').waitFor();await release(state,'old-post-filter');await frame(page);
  check('filter switch aborts and isolates the late response from an earlier type',JSON.stringify(await keys(page))===JSON.stringify(['moment:21'])&&await filter(page,'动态').getAttribute('aria-current')==='page'&&state.aborted.length>=1);
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{query:'?type=all&cursor=bootstrap-page',feedPlans:[{kind:'all',hold:'old-refresh',data:feed([item('post',30)])},{kind:'post',data:feed([item('post',31)],{type:'post'})}]});
  await page.getByRole('button',{name:'刷新',exact:true}).click();await until(()=>state.holds['old-refresh'],'refresh was not requested');
  check('refresh resets SSR cursor in both request and address',!new URLSearchParams(feedRequests(state)[0].query).has('cursor')&&!new URL(page.url()).searchParams.has('cursor'));
  await filter(page,'文章').click();await page.locator('[data-list-key="post:31"]').waitFor();await release(state,'old-refresh');await frame(page);
  check('a late refresh cannot overwrite the newer selected filter',JSON.stringify(await keys(page))===JSON.stringify(['post:31'])&&state.aborted.length>=1);
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{feedPlans:[{status:503},{data:feed([item('post',40)])}]});
  await page.getByRole('button',{name:'刷新',exact:true}).click();await page.locator('.following-error').waitFor();
  check('refresh error leaves the displayed feed available for retry',JSON.stringify(await keys(page))===JSON.stringify(['post:1','moment:1']));
  await page.locator('.following-error').getByRole('button',{name:'重试',exact:true}).click();await page.locator('[data-list-key="post:40"]').waitFor();
  check('refresh retry replaces rather than appends the old first page',JSON.stringify(await keys(page))===JSON.stringify(['post:40'])&&feedRequests(state).length===2);await page.close();
 }
 {
  const{page}=await fixture(browser,assets,{initial:feed([],{following_count:0})});
  check('no-follow empty state offers author discovery',await page.locator('.following-empty h2').textContent()==='还没有关注作者'&&await page.locator('.following-empty a').getAttribute('href')==='/discover');await page.close();
  const active=await fixture(browser,assets,{initial:feed([],{following_count:2})});
  check('followed authors without publications have a distinct empty state',await active.page.locator('.following-empty h2').textContent()==='还没有可阅读的更新'&&await active.page.locator('.following-empty a[href="/discover"]').count()===0);await active.page.close();
 }
 for(const variant of ['401','changed-viewer']){
  const plan=variant==='401'?{status:401}:{data:feed([item('post',50)],{viewer_id:'another-account'})};
  const{page,state}=await fixture(browser,assets,{feedPlans:[plan]});await page.getByRole('button',{name:'刷新',exact:true}).click();
  check(variant+' response clears and conceals private entries before login navigation',await assertPrivateRedirect(page,state));await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets,{query:'?type=all&cursor=restored-page',feedPlans:[{data:feed([item('post',60)])}]});
  state.sessionPlans.push({hold:'bfcache-identity',data:state.session});
  await page.evaluate(()=>dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true})));
  check('pagehide synchronously conceals the private feed',await page.locator('[data-following-feed]').evaluate(root=>root.hidden));
  await page.evaluate(()=>dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true})));await until(()=>state.holds['bfcache-identity'],'BFCache restore did not recheck identity');
  check('BFCache restore keeps private entries unavailable while identity check is pending',await page.locator('.following-item').count()===0&&await page.locator('.following-status').textContent().then(text=>text.includes('正在确认登录状态'))&&feedRequests(state).length===0);
  await release(state,'bfcache-identity');await page.locator('[data-list-key="post:60"]').waitFor();
  check('same-account BFCache restore reloads a fresh first page and removes old cursor',JSON.stringify(await keys(page))===JSON.stringify(['post:60'])&&state.requests.filter(request=>request.path==='/api/v1/session').length===2&&feedRequests(state).length===1&&!new URLSearchParams(feedRequests(state)[0].query).has('cursor')&&!new URL(page.url()).searchParams.has('cursor'));
  await page.close();
 }
 {
  const{page,state}=await fixture(browser,assets);state.session={authenticated:true,user:{id:'new-account',role:'reader'}};
  await page.evaluate(()=>{dispatchEvent(new PageTransitionEvent('pagehide',{persisted:true}));dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true}))});
  check('changed-account BFCache restore clears old feed without querying it again',await assertPrivateRedirect(page,state)&&feedRequests(state).length===0);await page.close();
 }
 {
  const longAuthor={id:'8',display_name:'很长的作者名称'.repeat(30),avatar_url:''};
  const{page}=await fixture(browser,assets,{initial:feed([item('post',70,{title:'https://example.invalid/'+('long-unbroken-title'.repeat(12)),summary:'正文摘要'.repeat(45),author:longAuthor,cover_url:'/fixture-cover.svg'}),item('moment',70,{summary:'手机动态 '+('W'.repeat(200)),author:longAuthor,cover_url:'/fixture-cover.svg',image_count:3})])});
  for(const width of [320,390]){await page.setViewportSize({width,height:844});await page.evaluate(width=>document.documentElement.dataset.theme=width===390?'dark':'light',width);await frame(page);
   const layout=await page.evaluate(()=>({width:innerWidth,scroll:document.documentElement.scrollWidth,targets:[...document.querySelectorAll('.following-filters>a,.following-actions>a,.following-refresh')].map(element=>({width:element.getBoundingClientRect().width,height:element.getBoundingClientRect().height}))}));
   check(width+'px mobile has no horizontal overflow and keeps 44px toolbar targets',layout.scroll<=layout.width+1&&layout.targets.every(target=>target.width>=43.9&&target.height>=43.9));await page.screenshot({path:path.join(out,'following-mobile-'+width+'.png'),fullPage:true});
  }await page.close();
 }
 check('all fixture traffic is intercepted and the real component has no script errors',errors.length===0);
}finally{for(const fixture of fixtures)if(!fixture.page.isClosed())await fixture.page.close();fs.writeFileSync(path.join(out,'following-feed-report.json'),JSON.stringify({checks,errors,requests:fixtures.map(({state})=>state.requests)},null,2));await browser.close()}})().catch(error=>{console.error(error);process.exitCode=1});
