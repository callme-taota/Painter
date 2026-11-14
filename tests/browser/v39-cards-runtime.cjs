// Intercepted synthetic fixture only; no app/service/database or business writes.
const fs=require('fs'),path=require('path');
const ts=require('../../web/frontend/node_modules/typescript');
const {chromium}=require('/tmp/painter-browser/node_modules/playwright');
const origin='http://internal-cards.test',out=process.env.V39_CARD_ARTIFACTS||'/tmp/painter-v39-cards-qa';
const source=name=>fs.readFileSync(path.resolve('web/frontend/src',name),'utf8');
const modules=Object.fromEntries(['contentCards','momentGallery'].map(name=>['./'+name,ts.transpileModule(source(name+'.ts'),{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.CommonJS}}).outputText]));
const runtime=`(()=>{const sources=${JSON.stringify(modules)},cache={};function require(name){if(name.endsWith('.css'))return {};if(cache[name])return cache[name].exports;const m={exports:{}};cache[name]=m;new Function('require','module','exports',sources[name])(require,m,m.exports);return m.exports}window.cardRuntime=require('./contentCards');window.cardRuntime.initArticleCards()})()`;
const styles=['style.css','shadcn-theme.css','articleCards.css','momentGallery.css'].map(source).join('\n');
const photo='/media/0123456789abcdef0123456789abcdef';
const momentText='散步回来，记下这一刻。\n隔离测试保留换行与完整内容。<script>window.injected=true</script>';
const phraseText='先把今天的想法写下来，让问题在文字中慢慢变得清楚。';
const ref=(kind,id)=>`<a class="content-card content-card--${kind}" href="/${kind==='moment'?'moments':'phrases'}/${id}" id="${kind}-${id}"><span class="content-card__body">读取公开内容…</span></a>`;
function fixture(theme){return `<!doctype html><html lang="zh-CN" data-theme="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>${styles}\nmain{width:min(760px,calc(100% - 48px));margin:28px auto}h1{font:600 20px/1.5 var(--font-ui)}.qa-note{font:400 12px/20px var(--font-ui);color:var(--muted)}</style></head><body data-page-kind="article"><main><h1>站内引用 · 隔离界面测试</h1><p class="qa-note">合成测试内容，未连接应用或正式数据库。</p><article class="article-reading"><div class="prose">${ref('moment',1)}${ref('phrase',2)}${ref('moment',9)}${ref('moment',10)}<a class="content-card content-card--moment" href="https://evil.test/moments/1" id="external">非法外部引用</a><a class="content-card content-card--phrase" href="/phrases/11" data-internal-card-state="ready" id="ssr-ready"><span class="content-card__body"><span class="content-card__quote">SSR已经提供的公开片语</span></span></a></div></article></main><script>${runtime}</script></body></html>`}
const report={mode:'isolated intercepted synthetic fixture; no app/service/database',checks:[],screenshots:[],errors:[]};
function check(name,passed){report.checks.push({name,passed:!!passed});console.log((passed?'PASS ':'FAIL ')+name);if(!passed)throw Error(name)}
async function run(browser,width,theme){
 let withdrawn=false;const calls=[];
 const ctx=await browser.newContext({viewport:{width,height:1000},colorScheme:theme,locale:'zh-CN'});
 await ctx.route('**/*',route=>{
  const request=route.request(),url=new URL(request.url());
  if(url.origin!==origin||request.method()!=='GET')throw Error('fixture attempted external I/O or mutation');
  const match=/^\/api\/v1\/content-cards\/(moment|phrase)\/(\d+)$/.exec(url.pathname);
  if(match){calls.push(url.pathname);const [,kind,id]=match;
   if(id==='9'||id==='1'&&withdrawn)return route.fulfill({status:404,contentType:'application/json',body:JSON.stringify({text:'private-never-render',image_url:'/media/private-never-render'})});
   if(id==='10')return route.fulfill({status:503,contentType:'application/json',body:'{"error":"private-never-render"}'});
   const reference={id,type:kind,url:`/${kind==='moment'?'moments':'phrases'}/${id}`,text:kind==='moment'?momentText:phraseText,source:kind==='phrase'?'作者保存的来源 <script>literal</script>':undefined,kind:'sentence',author:{display_name:'隔离测试作者'},published_at:'2026-11-08T06:30:00Z',images:kind==='moment'?Array.from({length:5},(_,i)=>({url:photo,alt:'测试照片 '+(i+1)})):undefined};
   return route.fulfill({contentType:'application/json',body:JSON.stringify({reference})});
  }
  if(url.pathname===photo)return route.fulfill({contentType:'image/png',body:fs.readFileSync('tests/browser/fixtures/avatar.png')});
  if(url.pathname==='/assets/geist-latin.woff2')return route.fulfill({contentType:'font/woff2',body:fs.readFileSync('web/frontend/src/assets/geist-latin.woff2')});
  return route.fulfill({contentType:'text/html',body:fixture(theme)});
 });
 const page=await ctx.newPage();page.on('pageerror',e=>report.errors.push(e.message));await page.goto(origin+'/');
 await page.waitForSelector('#moment-1[data-internal-card-state="ready"]');await page.waitForSelector('#phrase-2[data-internal-card-state="ready"]');
 await page.waitForFunction(()=>document.querySelectorAll('.content-card--unavailable').length===2);
 await page.evaluate(async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))});
 check(width+' complete literal content',await page.locator('#moment-1').textContent().then(t=>t.includes(momentText)&&t.includes('共5张图片'))&&await page.locator('#phrase-2').textContent().then(t=>t.includes(phraseText)));
 check(width+' injection stays text, image preview bounded',await page.evaluate(()=>!window.injected&&!document.querySelector('.content-card script')&&document.querySelectorAll('#moment-1 .content-card__photo').length===3));
 check(width+' private/error response content never rendered',!await page.locator('body').textContent().then(t=>t.includes('private-never-render'))&&await page.locator('.content-card--unavailable img').count()===0);
 check(width+' no external/SSR-ready fetch',calls.length===4&&!calls.some(p=>p.endsWith('/11')));
 check(width+' no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth&&[...document.querySelectorAll('.content-card')].every(n=>n.getBoundingClientRect().right<=innerWidth+1)));
 const shot=path.join(out,`cards-${width}-${theme}.png`);await page.screenshot({path:shot,fullPage:true});report.screenshots.push(shot);
 withdrawn=true;await page.evaluate(()=>{const old=document.querySelector('#moment-1'),replacement=document.createElement('a');replacement.className='content-card content-card--moment';replacement.href='/moments/1';replacement.id='moment-new-read';replacement.textContent='重新读取';old.replaceWith(replacement);window.cardRuntime.initArticleCards()});
 await page.waitForSelector('#moment-new-read.content-card--unavailable');
 check(width+' withdrawal re-read has no cached content/image',await page.locator('#moment-new-read').textContent().then(t=>t.includes('暂不可用')&&!t.includes('散步'))&&await page.locator('#moment-new-read img').count()===0);
 await ctx.close();
}
(async()=>{fs.mkdirSync(out,{recursive:true});const browser=await chromium.launch({headless:true});try{await run(browser,1440,'light');await run(browser,390,'dark');await run(browser,320,'light');check('no browser errors',report.errors.length===0);report.passed=report.checks.length;fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify({passed:report.passed,out}))}finally{await browser.close()}})().catch(e=>{report.failure=e.stack;fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'report.json'),JSON.stringify(report,null,2));console.error(e);process.exit(1)});
