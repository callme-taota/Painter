// Independent history regression for the explicitly disposable V44 candidate.
// Reads existing QA browser state only; creates no accounts, drafts or settings.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('/tmp/painter-browser/node_modules/playwright');
const base='http://127.0.0.1:18087',isolatedDatabase='painter_v44_mobile_20261113';
const privateDir='/tmp/painter-v44-private',out=path.resolve(__dirname,'../artifacts/v44/history');
const fixture=JSON.parse(fs.readFileSync(path.join(privateDir,'mobile-fixtures.json'),'utf8'));
const env=JSON.parse(fs.readFileSync(path.join(privateDir,'environment.json'),'utf8'));
if(fixture.origin!==base||fixture.isolated_database!==isolatedDatabase||env.PUBLIC_BASE_URL!==base||env.LISTEN_ADDR!=='127.0.0.1:18087'||env.RUN_BACKGROUND_WORKERS!=='false'||env.DATABASE_URL!=='host=/tmp/painter-v39-card-pg/socket port=65442 user=callumchan dbname='+isolatedDatabase+' sslmode=disable')throw Error('Disposable V44 history guard rejected configuration');
const report={origin:base,isolated_database:isolatedDatabase,created_at:new Date().toISOString(),checks:[],errors:[],limitations:['Chrome with emulated phone viewports and touch; no physical iOS/Android, WebKit or operating-system keyboard claim.','The external-origin return target is fulfilled entirely by the test browser; no external website or production origin is accessed.','Only the existing disposable QA session is read for workbench navigation.']};
let browser,page,knownTag;
function check(name,value,detail){report.checks.push({name,passed:!!value,...(detail===undefined?{}:{detail})});assert.ok(value,name);console.log('PASS '+name)}
function save(){fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'verification.json'),JSON.stringify(report,null,2))}
async function context(extra={}){const ctx=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,locale:'zh-CN',serviceWorkers:'block',...extra});const p=await ctx.newPage();p.setDefaultTimeout(8000);p.on('pageerror',error=>report.errors.push(error.message));return{ctx,p}}
async function visit(p,route){await p.goto(base+route,{waitUntil:'domcontentloaded'});await p.waitForFunction(()=>!!document.documentElement.classList.contains('mobile-chrome-ready')||!!document.querySelector('.workbench'));}
async function openMore(p){await p.locator('[data-mobile-more-open]').click();await p.locator('#mobile-more[open]').waitFor();await p.waitForFunction(()=>!!history.state?.painterMobileOverlay)}
async function noMarker(p){await p.waitForFunction(()=>!history.state?.painterMobileOverlay);}
async function back(p){await p.goBack({waitUntil:'domcontentloaded'});}
async function forward(p){await p.goForward({waitUntil:'domcontentloaded'});}
async function stable(p){await p.waitForTimeout(180)}

(async()=>{try{
 browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 {
  const {ctx,p}=await context();page=p;
  await visit(p,'/posts');await visit(p,'/search');
  await p.locator('[data-tag-filter]>summary').click();await p.locator('.mobile-filter-sheet[open]').waitFor();await p.waitForFunction(()=>!!history.state?.painterMobileOverlay);
  const choice=p.locator('.mobile-filter-sheet input[name=tag_id]').first(),tag=await choice.getAttribute('value');knownTag=tag;await choice.check();await p.locator('[data-tag-query]').fill('');await p.locator('[data-tag-query]').press('Enter');
  await p.waitForURL(url=>url.pathname==='/search'&&url.searchParams.getAll('tag_id').includes(tag));await noMarker(p);
  check('filter keyboard Enter applies selected tags after dismissing its task',new URL(p.url()).searchParams.getAll('tag_id').includes(tag));
  await back(p);await p.waitForURL(base+'/search');await stable(p);
  check('Back after filter keyboard submit returns to a clean search page',await p.evaluate(()=>!history.state?.painterMobileOverlay&&![...document.querySelectorAll('dialog')].some(d=>d.open)));
  await back(p);await p.waitForURL(base+'/posts');check('filter keyboard submit leaves no extra temporary history step',new URL(p.url()).pathname==='/posts');
  await ctx.close();
 }
 {
  const {ctx,p}=await context();page=p;await visit(p,'/posts');await openMore(p);const token=await p.evaluate(()=>history.state.painterMobileOverlay);
  await p.locator('[data-mobile-more-close]').click();await noMarker(p);await forward(p);await p.locator('#mobile-more[open]').waitFor();
  check('Forward restores more with the original owned history entry',await p.evaluate(token=>history.state?.painterMobileOverlay===token&&document.querySelector('#mobile-more').dataset.mobileOverlayToken===token,token));
  await back(p);await noMarker(p);check('Back dismisses restored more without leaving content',p.url()===base+'/posts'&&!await p.locator('#mobile-more').evaluate(d=>d.open));
  await p.locator('[data-search-open]').click();await p.waitForFunction(()=>!!history.state?.painterMobileOverlay);await p.locator('#quick-search').fill('工程');await p.locator('[data-search-close]').click();await noMarker(p);await forward(p);await p.locator('#site-search[open]').waitFor();
  check('Forward restores the search task and its input state',await p.locator('#quick-search').inputValue()==='工程'&&await p.locator('#quick-search').getAttribute('aria-expanded')==='true');
  await back(p);await noMarker(p);await visit(p,'/search?tag_id='+encodeURIComponent(knownTag)+'&sort=recent');
  const before=await p.evaluate(()=>({tags:[...document.querySelectorAll('input[name=tag_id]:checked')].map(x=>x.value),sort:document.querySelector('input[name=sort]:checked')?.value}));
  await p.locator('[data-tag-filter]>summary').click();await p.waitForFunction(()=>!!history.state?.painterMobileOverlay);const selected=p.locator('.mobile-filter-sheet input[name=tag_id]:checked');if(await selected.count())await selected.first().uncheck();
  await p.locator('[data-filter-cancel]').click();await noMarker(p);await forward(p);await p.locator('.mobile-filter-sheet[open]').waitFor();
  check('Forward restores a usable filter task without losing committed choices',await p.locator('.mobile-filter-sheet .search-filter-popover').count()===1&&JSON.stringify(await p.evaluate(()=>({tags:[...document.querySelectorAll('input[name=tag_id]:checked')].map(x=>x.value),sort:document.querySelector('input[name=sort]:checked')?.value})))===JSON.stringify(before));
  await back(p);await noMarker(p);await ctx.close();
 }
 {
  const {ctx,p}=await context();page=p;await visit(p,'/posts');await p.locator('[data-search-open]').click();await p.waitForFunction(()=>!!history.state?.painterMobileOverlay);await p.locator('#quick-search').fill('工程');await p.setViewportSize({width:1440,height:1000});await p.locator('#quick-search').press('Enter');
  await p.waitForURL(url=>url.pathname==='/search'&&url.searchParams.get('q')==='工程');await noMarker(p);check('search submit survives phone-to-desktop resize',new URL(p.url()).searchParams.get('q')==='工程');
  await back(p);await p.waitForURL(base+'/posts');await stable(p);check('cross-breakpoint search leaves no hidden temporary entry',await p.evaluate(()=>!history.state?.painterMobileOverlay));
  await p.setViewportSize({width:390,height:844});await openMore(p);await p.setViewportSize({width:820,height:1180});await noMarker(p);await stable(p);
  check('crossing to tablet closes more and returns tools to the side rail',!await p.locator('#mobile-more').evaluate(d=>d.open)&&await p.locator('[data-site-dock] .dock-tools').isVisible()&&!await p.locator('.mobile-primary-nav').isVisible());
  await p.setViewportSize({width:390,height:844});await visit(p,'/search');await p.locator('[data-tag-filter]>summary').click();await p.waitForFunction(()=>!!history.state?.painterMobileOverlay);await p.locator('.mobile-filter-sheet input[name=tag_id]').first().check();await p.setViewportSize({width:820,height:1180});await noMarker(p);await stable(p);
  check('filter breakpoint close cancels provisional choices and restores anchored controls',!await p.locator('.mobile-filter-sheet').evaluate(d=>d.open)&&await p.locator('[data-tag-filter] .search-filter-popover').count()===1&&await p.locator('input[name=tag_id]:checked').count()===0);
  await ctx.close();
 }
 {
  const {ctx,p}=await context();page=p;await visit(p,'/posts');const articlePath='/posts/'+fixture.article_id;await visit(p,articlePath);await openMore(p);await p.locator('.mobile-contents>summary').click();const entry=p.locator('.mobile-contents nav a').first(),hash=await entry.getAttribute('href');await entry.click();await p.waitForURL(base+articlePath+hash);await noMarker(p);await stable(p);
  check('article directory navigation retains its target hash after dismissing more',new URL(p.url()).hash===hash&&!await p.locator('#mobile-more').evaluate(d=>d.open));
  await back(p);await p.waitForURL(base+articlePath);await stable(p);check('Back from an article hash returns to the same article without a ghost task',await p.evaluate(()=>!history.state?.painterMobileOverlay&&!document.querySelector('#mobile-more').open));
  await back(p);await p.waitForURL(base+'/posts');check('article hash navigation preserves the preceding article list',new URL(p.url()).pathname==='/posts');await ctx.close();
 }
 {
  const {ctx,p}=await context();page=p;const external='https://painter-history-external.invalid/source';await ctx.route('https://painter-history-external.invalid/**',route=>route.fulfill({status:200,contentType:'text/html',body:'<!doctype html><html><title>Disposable external history source</title><body><a href="'+base+'/posts">Open disposable Painter</a></body></html>'}));
  await p.goto(external,{waitUntil:'domcontentloaded'});await p.getByRole('link',{name:'Open disposable Painter'}).click();await p.waitForURL(base+'/posts');await openMore(p);await back(p);await noMarker(p);check('Back from more retains a page reached from another origin',p.url()===base+'/posts');await back(p);await p.waitForURL(external);check('the next Back reaches the original external entry without being swallowed',p.url()===external);await ctx.close();
 }
 {
  const statePath=path.join(privateDir,'browser-state.json'),state=JSON.parse(fs.readFileSync(statePath,'utf8'));if(!state.cookies.every(cookie=>cookie.domain==='127.0.0.1'))throw Error('QA browser state contains an unexpected cookie domain');
  const {ctx,p}=await context({storageState:statePath});page=p;await visit(p,'/studio/posts');const all=p.getByRole('navigation',{name:'工作台常用功能'}).getByRole('button',{name:'全部',exact:true});await all.click();await p.locator('dialog[open]').waitFor();await p.waitForFunction(()=>!!history.state?.painterMobileOverlay);await p.keyboard.press('Escape');await noMarker(p);await forward(p);await p.locator('dialog[open]').waitFor();
  check('Forward restores the authenticated workbench navigation drawer',await p.locator('dialog[open]').evaluate(d=>d.matches(':modal')));await back(p);await noMarker(p);check('Back closes a restored workbench drawer before leaving its task',new URL(p.url()).pathname==='/studio/posts'&&await p.locator('dialog[open]').count()===0);await ctx.close();
 }
 check('history regression has no browser runtime errors',report.errors.length===0,report.errors);report.complete=true;save();console.log(JSON.stringify({complete:true,checks:report.checks.length,errors:report.errors.length}));
 }catch(error){report.complete=false;report.failure=error.message;if(page&&!page.isClosed()){fs.mkdirSync(out,{recursive:true});await page.screenshot({path:path.join(out,'failure.png'),fullPage:false}).catch(()=>{});}save();console.error('V44 history regression failed: '+error.message);process.exitCode=1;}finally{await browser?.close();}
})();
