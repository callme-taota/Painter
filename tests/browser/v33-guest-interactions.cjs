// Source-only browser regression. No Painter server, database, credentials,
// deployment build or live APIs: compile just these islands in memory and
// intercept every request with explicit fixtures.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {createRequire}=require('node:module');
const appRequire=createRequire(path.resolve('web/frontend/package.json'));
const esbuild=appRequire('esbuild'),{parse,compileScript}=appRequire('@vue/compiler-sfc');
const {chromium}=require('/tmp/painter-browser/node_modules/playwright');
const origin='https://painter-guest-fixture.test',checks=[];
let browser;
function check(name,condition){assert.ok(condition,name);checks.push(name)}
(async()=>{
 const compiled=await esbuild.build({stdin:{contents:`
  import {createApp} from 'vue';
  import Post from './components/PostIsland.vue';
  import Follow from './components/FollowIsland.vue';
  import {initExperience} from './experience';
  import {toggleLike} from './postLike';
  void initExperience();
  createApp(Post,{postId:'1'}).mount('#post');
  createApp(Follow,{userId:'author'}).mount('#follow');
  window.fixture={toggleLike};window.fixtureMounted=true;
 `,resolveDir:path.resolve('web/frontend/src'),sourcefile:'v33-fixture.ts',loader:'ts'},bundle:true,write:false,platform:'browser',format:'iife',define:{'process.env.NODE_ENV':'"test"'},plugins:[{name:'island-fixture',setup(build){
  build.onLoad({filter:/\.vue$/},args=>{const {descriptor}=parse(fs.readFileSync(args.path,'utf8'),{filename:args.path});const script=compileScript(descriptor,{id:path.basename(args.path),inlineTemplate:true});return{contents:script.content,loader:'ts',resolveDir:path.dirname(args.path)}});
  build.onLoad({filter:/\.css$/},()=>({contents:'',loader:'empty'}));
 }}]});
 const js=compiled.outputFiles[0].text;
 browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 for(const mode of ['guest','reader','session-error','states-expired']){
  const writes=[],privateReads=[],errors=[];let expireNextLike=false,likeCount=4,liked=false;
  const context=await browser.newContext({viewport:{width:mode==='guest'?390:1440,height:1000}});
  await context.route('**/*',async route=>{
   const request=route.request(),url=new URL(request.url()),p=url.pathname;
   if(url.origin!==origin)throw Error('fixture attempted an external request');
   if(p==='/fixture.js')return route.fulfill({contentType:'text/javascript',body:js});
   if(p==='/')return route.fulfill({contentType:'text/html',body:`<!doctype html><html><head><style>button{display:inline-flex}</style></head><body data-post-id="1"><main><div id="post"></div><div id="follow"></div></main>${['reader','states-expired'].includes(mode)?'<button data-floating-action="like" disabled><small data-like-count></small></button><span data-floating-status></span>':''}<script type="module" src="/fixture.js"></script></body></html>`});
   if(p.startsWith('/api/v1/me/'))privateReads.push(p);
   if(!['GET','HEAD'].includes(request.method()))writes.push({method:request.method(),path:p});
   let json;
   if(p==='/api/v1/session'){
    await new Promise(resolve=>setTimeout(resolve,100));
    if(mode==='session-error')return route.fulfill({status:503,json:{error:{code:'UNAVAILABLE',message:'连接暂不可用'}}});
    json=['reader','states-expired'].includes(mode)?{authenticated:true,user:{id:'reader',role:'reader'},csrf_token:'fixture-csrf'}:{authenticated:false};
   }else if(p==='/api/v1/site')json={comments_enabled:true,collections_enabled:true,follows_enabled:true};
   else if(p==='/api/v1/posts/1/comments')json={items:[{id:url.searchParams.has('cursor')?'2':'1',body:url.searchParams.has('cursor')?'下一页公开评论':'公开评论仍然可读',user:{id:'author',display_name:'作者'},created_at:'2026-11-07T00:00:00Z',is_self:true,liked:false,like_count:2}],has_more:!url.searchParams.has('cursor'),next_cursor:url.searchParams.has('cursor')?null:'page-2'};
   else if(p==='/api/v1/posts/1/metrics')json={like_count:likeCount};
   else if(p==='/api/v1/me/post-states'){if(mode==='states-expired')return route.fulfill({status:401,json:{error:{code:'UNAUTHENTICATED',message:'登录已过期'}}});json={items:[{liked,bookmarked:false}]};}
   else if(p==='/api/v1/me/following')json={items:[],has_more:false};
   else if(p==='/api/v1/posts/1/like'){
    if(expireNextLike)return route.fulfill({status:401,json:{error:{code:'UNAUTHENTICATED',message:'登录已过期'}}});
    liked=request.method()==='PUT';likeCount+=liked?1:-1;return route.fulfill({status:204});
   }else return route.fulfill({status:404,json:{error:{message:'unhandled fixture path'}}});
   return route.fulfill({json});
  });
  const page=await context.newPage();page.on('pageerror',error=>errors.push(error.message));
  await page.goto(origin+'/');await page.waitForFunction(()=>window.fixtureMounted);
  check(mode+' initial island frame has no auth-action flash',await page.locator('.interaction-bar,.discussion-composer,.discussion-actions,#follow button').count()===0);
  await page.getByText('公开评论仍然可读',{exact:true}).waitFor();await page.locator('.discussion-loading').waitFor({state:'hidden'});
  if(mode!=='reader'){
   check(mode+' public comments remain visible',await page.locator('.discussion-comment').count()===1);
   check(mode+' hides post/comment/follow actions',await page.locator('.interaction-bar,.discussion-composer,.discussion-actions,#follow button').count()===0);
   check(mode+' has no replacement sign-in CTA',await page.locator('.discussion-signin,a[href*="login"]').count()===0);
   await page.getByRole('button',{name:'更多评论',exact:true}).click();await page.getByText('下一页公开评论',{exact:true}).waitFor();
   check(mode+' comment pagination remains public',await page.locator('.discussion-comment').count()===2);
   await page.evaluate(()=>window.fixture.toggleLike('1'));
   check(mode+' direct like invocation does not navigate or mutate',page.url()===origin+'/'&&writes.length===0);
   if(mode!=='states-expired')check(mode+' does not request personal relationships',privateReads.length===0);
  }else{
   await page.getByRole('button',{name:'关注作者',exact:true}).waitFor();
   check('reader keeps comment and bookmark actions',await page.locator('.discussion-composer').count()===1&&await page.getByRole('button',{name:'收藏',exact:true}).count()===1&&await page.locator('.discussion-actions button').count()===2);
   await page.locator('.interaction-bar button').first().click();
   await page.waitForFunction(()=>document.querySelector('[data-floating-action=like]')?.getAttribute('aria-pressed')==='true');
   check('reader post and dock likes stay synchronized',await page.locator('.interaction-bar button').first().getAttribute('aria-pressed')==='true'&&await page.locator('[data-like-count]').textContent()==='5');
   expireNextLike=true;await page.locator('.interaction-bar button').first().click();
   await page.locator('.interaction-bar').waitFor({state:'detached'});
   check('known expired session hides every auth action',await page.locator('.discussion-composer,.discussion-actions,#follow button').count()===0&&await page.locator('[data-floating-action=like]').isHidden());
   check('expired session preserves public comments and current URL',await page.locator('.discussion-comment').count()===1&&page.url()===origin+'/');
  }
  check(mode+' has no browser runtime errors',errors.length===0);
  await context.close();
 }
 await browser.close();browser=null;
 fs.mkdirSync('tests/artifacts/v33',{recursive:true});fs.writeFileSync('tests/artifacts/v33/guest-islands.json',JSON.stringify({passed:true,checks,fixture_only:true,live_api_requests:0,business_writes:0},null,2));
 console.log(JSON.stringify({passed:true,checks:checks.length,fixture_only:true,business_writes:0}));
})().catch(async error=>{console.error(error);await browser?.close();process.exitCode=1});
