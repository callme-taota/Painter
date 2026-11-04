// Formal LAN smoke: normal login/session and telemetry only; no content writes.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),fs=require('fs'),path=require('path');
const base=process.env.BASE_URL||'http://192.168.2.209:18080',out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v5');
if(!process.env.ACCOUNT_FILE)throw Error('A private ACCOUNT_FILE is required');
const accounts=JSON.parse(fs.readFileSync(process.env.ACCOUNT_FILE,'utf8')).accounts,owner=accounts.find(a=>a.role==='owner');
const redact=s=>accounts.reduce((t,a)=>t.split(a.password).join('[redacted]'),String(s));
let browser;const checks=[],errors=[];const check=(name,ok)=>{checks.push({check:name,passed:!!ok});if(!ok)throw Error(name)};
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--enable-unsafe-swiftshader']});
 const c=await browser.newContext({viewport:{width:390,height:900}}),p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));
 const ready=await c.request.get(base+'/readyz');check('formal PostgreSQL service is ready',ready.ok()&&(await ready.json()).database==='postgresql18');
 await p.goto(base+'/');await p.locator('[data-home-story][data-renderer]').waitFor();check('formal home retains cinematic story',await p.locator('[data-home-story]').getAttribute('data-renderer')==='webgl');
 const publicAsset=await p.locator('script[type=module]').getAttribute('src');check('formal origin serves final V5 public asset',publicAsset.includes('public-D27FaqmA.js'));
 check('personal introduction restores original name',await p.locator('.personal-name').innerText().then(s=>s.includes('逃塔')));check('all seven original identity labels are present',await p.locator('.story-roles li').count()===7);check('header retains original Painter brand',await p.locator('.brand').innerText().then(s=>s.trim()==='Painter'));
 await p.goto(base+'/about');check('about restores sourced introduction',await p.locator('.prose').innerText().then(s=>s.includes('逃塔')&&!s.includes('示例站点')));
 await p.goto(base+'/posts/1');check('article navigation is transparent in document flow',await p.locator('.site-header').evaluate(e=>getComputedStyle(e).position==='relative'&&getComputedStyle(e).backgroundColor==='rgba(0, 0, 0, 0)'));
 check('all three date labels render on existing article',await p.locator('.article-dates dt').allTextContents().then(x=>x.join(',')==='创建时间,最后编辑,发布时间'));
 check('article date values have machine-readable timestamps',await p.locator('.article-dates time[datetime]').count()===3);
 const pub=await c.request.get(base+'/api/v1/posts/1').then(r=>r.json()),svg=await c.request.get(base+pub.cover_url).then(r=>r.text());check('existing published cover uses gradient renderer',svg.includes('data-renderer="gradient-v1"')&&svg.includes('<linearGradient')&&!/<(?:circle|path|polygon|line)\b/.test(svg));
 check('published cover is decoded in real browser',await p.locator('.article-cover').evaluate(async e=>{await e.decode();return e.naturalWidth===1200}));
 check('anonymous access to private editor data remains unauthorized',(await c.request.get(base+'/api/v1/studio/posts/1')).status()===401);
 await p.goto(base+'/login');await p.getByLabel('用户名',{exact:true}).fill(owner.username);await p.getByLabel('密码',{exact:true}).fill(owner.password);await p.getByRole('button',{name:'登录',exact:true}).click();await p.waitForURL('**/me');check('normal owner local login works on mobile formal origin',await p.getByRole('heading',{name:'个人资料',exact:true}).isVisible());
 let contentWrites=0;p.on('request',r=>{if(['POST','PUT','DELETE'].includes(r.method())&&/^\/api\/v1\/(posts$|studio\/posts)/.test(new URL(r.url()).pathname))contentWrites++});
 await p.goto(base+'/studio/posts/new');await p.getByRole('textbox',{name:'文档正文',exact:true}).waitFor();await p.waitForTimeout(1300);check('editor defaults to directly editable document',await p.getByRole('textbox',{name:'文档正文',exact:true}).getAttribute('contenteditable')==='true');
 await p.locator('.editor-properties>summary').click();check('metadata disclosure uses updated label',await p.locator('.editor-properties>summary').innerText().then(s=>s.includes('文章信息与展示')));
 check('new article exposes three independently enabled date choices',(await Promise.all(['显示创建时间','显示最后编辑时间','显示发布时间'].map(name=>p.getByLabel(name,{exact:true}).isChecked()))).every(Boolean));
 check('gradient cover choices use actual style names',await p.getByLabel('版式',{exact:false}).locator('option').allTextContents().then(x=>x.includes('深邃渐变')&&x.includes('流彩渐变')&&x.includes('柔光渐变')));
 await p.getByRole('button',{name:'Markdown 源码',exact:true}).click();await p.locator('.cm-content').waitFor();check('source mode remains available',await p.locator('.cm-content').isVisible());check('opening and switching blank editor creates no content',contentWrites===0);
 await p.goto(base+'/studio/settings');await p.getByLabel('介绍姓名',{exact:true}).waitFor();check('personal name remains editable in site settings',await p.getByLabel('介绍姓名',{exact:true}).inputValue()==='逃塔');check('all original labels survive settings reload',await p.getByLabel('身份与兴趣',{exact:false}).inputValue().then(s=>s.includes('Golang工程师')&&s.includes('学生')));
 check('mobile page has no horizontal overflow',await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));check('no browser runtime errors',errors.length===0);
 if(process.env.OUTPUT_STATE_FILE){await c.storageState({path:process.env.OUTPUT_STATE_FILE});fs.chmodSync(process.env.OUTPUT_STATE_FILE,0o600)}
 fs.writeFileSync(path.join(out,'promoted-lan-smoke.json'),JSON.stringify({origin:base,publicAsset,checks,errors,content_writes:contentWrites},null,2));console.log(JSON.stringify({checks:checks.length,allPassed:true,contentWrites}));await browser.close();
})().catch(async e=>{const message=redact(e.stack);fs.writeFileSync(path.join(out,'promoted-lan-smoke-failed.json'),JSON.stringify({checks,errors:errors.map(redact),failure:message},null,2));console.error(message);await browser?.close();process.exitCode=1});
