// Read-only new-editor regression. Safe for candidate/formal: every content or
// configuration mutation is blocked; only canonical preview POSTs may proceed.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),fs=require('fs'),path=require('path');
const base=(process.env.BASE_URL||'http://192.168.2.209:18081').replace(/\/$/,''),state=process.env.OWNER_STATE_FILE,out=path.resolve(process.env.ARTIFACT_FILE||'tests/artifacts/v24/empty-editor/report.json');
if(!state)throw Error('Private OWNER_STATE_FILE required');
const report={origin:base,read_only:true,checks:[],blocked:[],errors:[]};let browser;
function check(name,passed,detail){report.checks.push({name,passed:!!passed,...(detail===undefined?{}:{detail})});if(!passed)throw Error(name)}
(async()=>{browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--disable-dev-shm-usage']});
for(const [width,height,theme]of [[1440,1000,'light'],[390,900,'dark']]){
 const context=await browser.newContext({storageState:state,viewport:{width,height},colorScheme:theme,serviceWorkers:'block'});await context.addInitScript(t=>localStorage.setItem('painter-theme',t),theme);
 await context.route('**/api/v1/**',route=>{const r=route.request(),u=new URL(r.url());if(['GET','HEAD'].includes(r.method())||['/api/v1/editor/preview','/api/v1/editor/cover-preview'].includes(u.pathname))return route.continue();if(u.pathname==='/api/v1/analytics/events')return route.fulfill({json:{accepted:0}});report.blocked.push({method:r.method(),path:u.pathname});return route.abort()});
 const page=await context.newPage();page.setDefaultTimeout(10000);page.on('pageerror',e=>report.errors.push(e.message));await page.goto(base+'/studio/posts/new');await page.waitForFunction(()=>document.querySelector('.document-prose')?.editor&&!document.querySelector('.editor-title')?.disabled);
 const prose=page.getByRole('textbox',{name:'文档正文',exact:true}),prefix=width+'-'+theme+': ';
 check(prefix+'new article remains rich and editable',await prose.isVisible()&&await prose.getAttribute('contenteditable')==='true'&&await page.locator('.document-compatibility').count()===0);
 const serialized=await prose.evaluate(el=>({markdown:el.editor.getMarkdown(),json:el.editor.getJSON()}));check(prefix+'empty document uses the paragraph default',serialized.markdown===''&&serialized.json.content[0].type==='paragraph',serialized);
 await page.locator('[data-open-cards]').click();const panel=page.locator('[data-card-composer]');await panel.waitFor();check(prefix+'card toolbar is accessible without inserting content',await panel.locator('[data-card-url]').isEditable());await panel.getByRole('button',{name:'取消',exact:true}).click();await page.waitForTimeout(1300);
 check(prefix+'opening/cancelling causes no autosave',report.blocked.length===0&&await prose.evaluate(el=>el.editor.getMarkdown()===''));await context.close();
}
check('no browser errors',report.errors.length===0);check('zero attempted content/configuration writes',report.blocked.length===0);report.passed=true;fs.mkdirSync(path.dirname(out),{recursive:true});fs.writeFileSync(out,JSON.stringify(report,null,2));console.log(JSON.stringify({passed:true,origin:base,checks:report.checks.length,actual_business_writes:0}));await browser.close();
})().catch(async error=>{report.passed=false;report.error=String(error);fs.mkdirSync(path.dirname(out),{recursive:true});fs.writeFileSync(out,JSON.stringify(report,null,2));console.error(String(error));await browser?.close();process.exitCode=1});
