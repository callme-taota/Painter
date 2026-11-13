// Explicitly isolated V49 candidate only. Credentials are read from its private
// fixture directory and are never logged, serialized into reports, or captured.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{randomUUID}=require('node:crypto');
const {chromium}=require(process.env.PAINTER_PLAYWRIGHT_MODULE||'/tmp/painter-browser/node_modules/playwright');
const base='http://127.0.0.1:18091',privateDir='/tmp/painter-v49-private';
const environment=JSON.parse(fs.readFileSync(path.join(privateDir,'environment.json'),'utf8'));
assert(environment.DATABASE_URL==='host=/tmp/painter-v39-card-pg/socket port=65442 user=callumchan dbname=painter_v49_features_20261114 sslmode=disable','Refusing a non-isolated database');
assert(environment.PUBLIC_BASE_URL===base&&environment.ENVIRONMENT==='development','Refusing a non-development candidate');
const credential=JSON.parse(fs.readFileSync(path.join(privateDir,'accounts.json'),'utf8')).reader;
const out=path.resolve(__dirname,'../artifacts/v49/profile-real');fs.mkdirSync(out,{recursive:true});
const checks=[],scenarios=[],errors=[];
function check(name,value){assert.ok(value,name);checks.push({name,passed:true})}
async function data(response,status=200){assert.equal(response.status(),status,'isolated API response status');return response.json()}
async function getProfile(context){return data(await context.request.get(base+'/api/v1/me'))}
async function patchProfile(context,fields){const current=await getProfile(context),session=await data(await context.request.get(base+'/api/v1/session'));return data(await context.request.patch(base+'/api/v1/me',{headers:{Origin:base,'X-CSRF-Token':session.csrf_token,'If-Match':`"${current.version}"`,'Idempotency-Key':randomUUID()},data:fields}))}
async function capture(page,name){await page.screenshot({path:path.join(out,name+'.png'),fullPage:true,mask:[page.locator('input[autocomplete=email]'),page.locator('input[autocomplete=tel]')]})}
(async()=>{const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});try{for(const width of [320,390,1440]){
 const context=await browser.newContext({viewport:{width,height:width<=390?844:1000},isMobile:width<=390,hasTouch:width<=390,reducedMotion:'reduce'}),page=await context.newPage();let original,restored=false,requests=[];
 page.on('pageerror',error=>errors.push({width,message:error.message}));page.on('request',request=>{const url=new URL(request.url());if(url.origin===base&&url.pathname.startsWith('/api/v1/'))requests.push({method:request.method(),path:url.pathname+url.search})});
 try{
  await page.goto(base+'/login');await page.getByLabel('用户名',{exact:true}).fill(credential.username);await page.getByLabel('密码',{exact:true}).fill(credential.password);
  const login=page.waitForResponse(r=>r.url()===base+'/api/v1/auth/login'&&r.request().method()==='POST');await page.getByRole('button',{name:'登录',exact:true}).click();check(width+' real reader password login', (await login).status()===200);await page.waitForURL(base+'/me');await page.getByRole('tab',{name:'我的',exact:true}).waitFor();
  original=await getProfile(context);check(width+' fixture account is a reader',original.role==='reader');
  const privateEmail='private-v49-reader@example.test',privatePhone='+86 19900004949';
  await patchProfile(context,{email:privateEmail,phone:privatePhone});
  requests=[];await page.reload();await page.getByRole('tab',{name:'我的',exact:true}).waitFor();await page.waitForLoadState('networkidle');
  const initial=requests.filter(r=>r.method==='GET');check(width+' initial personal centre has exactly session and me reads',initial.length===2&&initial.some(r=>r.path==='/api/v1/session')&&initial.some(r=>r.path==='/api/v1/me'));
  check(width+' reader has no writing action',await page.getByRole('link',{name:'写新文章',exact:true}).count()===0);
  check(width+' reading shortcuts are visible',await page.getByRole('navigation',{name:'我的阅读'}).getByRole('link').count()===3);
  check(width+' page has no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  await capture(page,width+'-reader-overview');
  await page.getByRole('tab',{name:'公开主页',exact:true}).click();await page.getByRole('heading',{name:'公开主页外观',exact:true}).waitFor();await page.waitForLoadState('networkidle');
  check(width+' appearance reads once when opened',requests.filter(r=>r.method==='GET'&&r.path==='/api/v1/me/author-page').length===1);
  await page.getByRole('tab',{name:'安全',exact:true}).click();await page.getByRole('heading',{name:'两步验证',exact:true}).waitFor();await page.waitForLoadState('networkidle');check(width+' security reads once when opened',requests.filter(r=>r.method==='GET'&&r.path==='/api/v1/me/security').length===1);
  await page.getByRole('tab',{name:'我的',exact:true}).click();await page.getByRole('tab',{name:'公开主页',exact:true}).click();await page.getByRole('tab',{name:'安全',exact:true}).click();
  check(width+' task returns do not reload appearance or security',requests.filter(r=>r.method==='GET'&&(r.path==='/api/v1/me/security'||r.path==='/api/v1/me/author-page')).length===2);
  await capture(page,width+'-reader-security');
  await page.getByRole('tab',{name:'资料',exact:true}).click();const username=page.getByLabel('用户名',{exact:true}),display=page.getByLabel('公开昵称',{exact:true}),unsavedName='V49 未保存的读者昵称';await username.fill('x');await display.fill(unsavedName);
  let save=page.waitForResponse(r=>r.url()===base+'/api/v1/me'&&r.request().method()==='PATCH');await page.getByRole('button',{name:'保存资料',exact:true}).click();check(width+' invalid username returns real 400',(await save).status()===400);await page.getByRole('button',{name:'关闭提示',exact:true}).waitFor();check(width+' 400 keeps editable form and unsaved text',await display.isVisible()&&await display.inputValue()===unsavedName);await page.getByRole('button',{name:'关闭提示',exact:true}).click();await username.fill(original.username);
  const serverName='V49 服务端更新的读者昵称';await patchProfile(context,{display_name:serverName});
  save=page.waitForResponse(r=>r.url()===base+'/api/v1/me'&&r.request().method()==='PATCH');await page.getByRole('button',{name:'保存资料',exact:true}).click();check(width+' concurrent update returns real 409',(await save).status()===409);await page.getByRole('button',{name:'重新读取资料',exact:true}).waitFor();check(width+' 409 keeps editable form and unsaved text',await display.isVisible()&&await display.inputValue()===unsavedName);
  await page.getByRole('button',{name:'重新读取资料',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();check(width+' cancelling reload preserves unsaved text',await display.inputValue()===unsavedName);
  await capture(page,width+'-reader-conflict');
  const retainedTaskReads=requests.filter(r=>r.method==='GET'&&(r.path==='/api/v1/me/author-page'||r.path==='/api/v1/me/security')).length;await page.getByRole('button',{name:'重新读取资料',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'重新读取',exact:true}).click();await page.waitForFunction(expected=>document.querySelector('input[autocomplete=nickname]')?.value===expected,serverName);check(width+' confirmed reload fetches server version',await page.getByLabel('公开昵称',{exact:true}).inputValue()===serverName);await page.waitForLoadState('networkidle');check(width+' explicit profile reload preserves mounted appearance and security',requests.filter(r=>r.method==='GET'&&(r.path==='/api/v1/me/author-page'||r.path==='/api/v1/me/security')).length===retainedTaskReads);
  const personalReads=requests.filter(r=>r.method==='GET');
  const response=await page.goto(base+'/users/'+original.id);check(width+' public profile returns 200',response.status()===200);const html=await response.text();
  check(width+' reader profile has no author article search',await page.locator('.author-search').count()===0);check(width+' reader profile has no fake empty article list',await page.locator('.author-writing,.author-empty').count()===0);check(width+' public reader note exists',await page.locator('.reader-public-note').isVisible());
  check(width+' private contact values absent from SSR',!html.includes(privateEmail)&&!html.includes(privatePhone));check(width+' private contact fields absent from public page',await page.getByLabel('邮箱（仅本人可见）',{exact:true}).count()===0&&await page.getByLabel('电话（仅本人可见）',{exact:true}).count()===0);
  check(width+' public reader profile has no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
  const publicDTO=await data(await context.request.get(base+'/api/v1/users/'+original.id));check(width+' public DTO excludes private contact and versions',!('email'in publicDTO)&&!('phone'in publicDTO)&&!('version'in publicDTO)&&!('avatar_media_id'in publicDTO));
  await capture(page,width+'-reader-public');
  const anonymous=await browser.newContext({viewport:{width,height:844}}),guest=await anonymous.request.get(base+'/users/'+original.id);check(width+' anonymous SSR excludes private contact values',guest.status()===200&&!((await guest.text()).includes(privateEmail))&&!((await guest.text()).includes(privatePhone)));await anonymous.close();
  scenarios.push({width,initial_gets:initial,all_personal_gets:personalReads,private_data_exposed:false,production_credentials_used:false});
  await patchProfile(context,Object.fromEntries(['username','display_name','bio','email','phone','avatar_media_id'].map(k=>[k,original[k]??(k==='avatar_media_id'?null:'')])));const restoredProfile=await getProfile(context);check(width+' original reader fixture profile restored',restoredProfile.username===original.username&&restoredProfile.display_name===original.display_name&&(restoredProfile.email||'')===(original.email||'')&&(restoredProfile.phone||'')===(original.phone||''));restored=true;
 }finally{if(original&&!restored)await patchProfile(context,Object.fromEntries(['username','display_name','bio','email','phone','avatar_media_id'].map(k=>[k,original[k]??(k==='avatar_media_id'?null:'')]))).catch(()=>errors.push({width,message:'Isolated reader profile restoration failed'}));await context.close()}
 }
 check('no application runtime errors',errors.length===0);console.log(JSON.stringify({status:'passed',checks:checks.length,scenarios:scenarios.length,production_credentials_used:false}));
 }finally{fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({base,isolated_database:'painter_v49_features_20261114',checks,scenarios,errors,production_credentials_used:false},null,2));await browser.close()}
})().catch(error=>{console.error(error.message);process.exitCode=1});
