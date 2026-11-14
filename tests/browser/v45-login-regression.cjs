// Client validation regression checks against a disposable loopback candidate.
// Every authentication POST is intercepted; this never submits real credentials
// or changes an account, registration policy, or login rate bucket.
const fs=require('fs'),path=require('path');
const {chromium}=require('/tmp/painter-browser/node_modules/playwright');
const base='http://127.0.0.1:18088';
const out=path.resolve('tests/artifacts/v45/login-regression');
const report={origin:base,authentication:'intercepted responses; no account writes',checks:[],screenshots:[],page_errors:[],pattern_errors:[],unexpected_writes:[],limitations:['Chrome desktop and emulated phone; no physical-device or other-engine claim.','These checks isolate client validation. Real server authentication and session behavior require separate integration checks.']};
fs.mkdirSync(out,{recursive:true});
let browser,step='startup';
function check(name,passed){report.checks.push({step,name,passed:!!passed});console.log((passed?'PASS ':'FAIL ')+name);if(!passed)throw Error(name)}
function save(){fs.writeFileSync(path.join(out,'verification.json'),JSON.stringify(report,null,2))}
async function capture(page,name){await page.screenshot({path:path.join(out,name+'.png'),fullPage:true,mask:[page.locator('input[type="password"]')],maskColor:'#e4e4e7'});report.screenshots.push(name+'.png');check(name+' fits viewport',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1))}
async function settle(){await new Promise(resolve=>setTimeout(resolve,120))}
(async()=>{try{
 browser=await chromium.launch({headless:true,executablePath:'/usr/bin/google-chrome',args:['--no-sandbox']});
 for(const [width,theme]of [[1440,'light'],[390,'dark']]){
  const prefix=width<720?'phone':'desktop';
  const context=await browser.newContext({viewport:{width,height:width<720?844:1000},colorScheme:theme,locale:'zh-CN',serviceWorkers:'block'});
  await context.addInitScript(theme=>{try{localStorage.setItem('painter-theme',theme);localStorage.setItem('painter-analytics','off')}catch{}},theme);
  let loginPosts=0,registerPosts=0,lastLoginBytes=0;
  await context.route(base+'/api/v1/**',async route=>{
   const request=route.request(),pathname=new URL(request.url()).pathname;
   if(pathname==='/api/v1/auth/options')return route.fulfill({json:{csrf_token:'isolated-client-test-csrf',registration_open:true,external_login_enabled:true,local_login_enabled:true,github_login_enabled:false,password_min_length:15,password_max_length:1024,username_min_length:2,username_max_length:64}});
   if(request.method()==='POST'&&pathname==='/api/v1/auth/login'){
    loginPosts++;lastLoginBytes=Buffer.byteLength(request.postDataJSON().password,'utf8');
    return route.fulfill({status:401,json:{error:{code:'INVALID_CREDENTIALS',message:'用户名或密码不正确'}}});
   }
   if(request.method()==='POST'&&pathname==='/api/v1/auth/register'){
    registerPosts++;
    return route.fulfill({status:400,json:{error:{code:'VALIDATION_ERROR',message:'隔离测试未创建账号'}}});
   }
   if(!['GET','HEAD','OPTIONS'].includes(request.method())){report.unexpected_writes.push({method:request.method(),path:pathname});return route.abort()}
   return route.continue();
  });
  const page=await context.newPage();page.setDefaultTimeout(10000);
  page.on('pageerror',error=>report.page_errors.push(error.message));
  page.on('console',message=>{if(message.type()==='error'&&/pattern|regular expression|character class/i.test(message.text()))report.pattern_errors.push(message.text())});
  step=prefix+'-login';await page.goto(base+'/login');
  const username=page.getByLabel('用户名',{exact:true}),password=page.getByLabel('密码',{exact:true}),login=page.getByRole('button',{name:'登录',exact:true});
  await username.waitFor();await username.fill('legacy-user_1');
  check('username pattern compiles in Chrome Unicode v mode',await username.evaluate(input=>{try{new RegExp(input.pattern,'v');return true}catch{return false}}));
  check('username pattern accepts supported boundaries',await username.evaluate(input=>{const pattern=new RegExp('^(?:'+input.pattern+')$','v');return ['a0','A_','a-','a'+'x'.repeat(63)].every(value=>pattern.test(value))}));
  check('username pattern rejects unsupported names',await username.evaluate(input=>{const pattern=new RegExp('^(?:'+input.pattern+')$','v');return ['a','_name','-name','bad.name','bad name','逃塔','a'+'x'.repeat(64)].every(value=>!pattern.test(value))}));
  await password.fill('legacy-7');await login.click();await page.getByRole('alert').filter({hasText:'用户名或密码不正确'}).waitFor();
  check('short existing password reaches login endpoint and renders its 401',loginPosts===1&&lastLoginBytes===8);
  await capture(page,prefix+'-login-server-error');
  await password.fill('a'.repeat(1025));await login.click();await settle();
  check('login does not impose new-password 1024-character ceiling',loginPosts===2&&lastLoginBytes===1025);
  await password.fill('a'.repeat(4096));await login.click();await settle();
  check('login permits the 4096-byte resource boundary',loginPosts===3&&lastLoginBytes===4096);
  await password.fill('a'.repeat(4097));await login.click();await page.getByRole('alert').filter({hasText:'密码不能超过 4096 字节。'}).waitFor();
  check('login blocks inputs exceeding its byte resource limit',loginPosts===3);
  await password.fill('密'.repeat(1366));await login.click();await settle();
  check('login byte limit accounts for multibyte characters',loginPosts===3);
  await password.fill('');await login.click();await settle();
  check('empty password is blocked by required input',loginPosts===3&&await password.evaluate(input=>input.validity.valueMissing));
  await password.fill('legacy-7');await username.fill('bad.name');await login.click();await settle();
  check('native username pattern prevents invalid submissions',loginPosts===3&&await username.evaluate(input=>input.validity.patternMismatch));
  await context.close();

  // A fresh context prevents the login-page native validation popup and form
  // state from influencing registration validation.
  const registerContext=await browser.newContext({viewport:{width,height:width<720?844:1000},colorScheme:theme,locale:'zh-CN',serviceWorkers:'block'});
  await registerContext.addInitScript(theme=>{try{localStorage.setItem('painter-theme',theme);localStorage.setItem('painter-analytics','off')}catch{}},theme);
  await registerContext.route(base+'/api/v1/**',async route=>{
   const request=route.request(),pathname=new URL(request.url()).pathname;
   if(pathname==='/api/v1/auth/options')return route.fulfill({json:{csrf_token:'isolated-client-test-csrf',registration_open:true,external_login_enabled:true,local_login_enabled:true,github_login_enabled:false}});
   if(request.method()==='POST'&&pathname==='/api/v1/auth/register'){registerPosts++;return route.fulfill({status:400,json:{error:{code:'VALIDATION_ERROR',message:'隔离测试未创建账号'}}})}
   if(!['GET','HEAD','OPTIONS'].includes(request.method())){report.unexpected_writes.push({method:request.method(),path:pathname});return route.abort()}
   return route.continue();
  });
  const registerPage=await registerContext.newPage();registerPage.setDefaultTimeout(10000);
  registerPage.on('pageerror',error=>report.page_errors.push(error.message));
  registerPage.on('console',message=>{if(message.type()==='error'&&/pattern|regular expression|character class/i.test(message.text()))report.pattern_errors.push(message.text())});
  step=prefix+'-register';await registerPage.goto(base+'/register');
  await registerPage.getByLabel('用户名',{exact:true}).fill('new-user_1');await registerPage.getByLabel('显示名称',{exact:true}).fill('隔离验证账号');
  const newPassword=registerPage.getByLabel('密码',{exact:true}),register=registerPage.getByRole('button',{name:'注册账号',exact:true});
  await newPassword.fill('legacy-7');await register.click();await registerPage.getByRole('alert').filter({hasText:'密码需为 15–1024 个字符'}).waitFor();
  check('registration retains 15-character minimum without a POST',registerPosts===0);
  await capture(registerPage,prefix+'-register-password-error');
  await newPassword.fill('a'.repeat(1025));await register.click();await settle();
  check('registration retains 1024-character ceiling',registerPosts===0);
  await newPassword.fill('a'.repeat(15));await register.click();await registerPage.getByRole('alert').filter({hasText:'隔离测试未创建账号'}).waitFor();
  check('registration submits at its 15-character boundary',registerPosts===1);
  await registerContext.close();
 }
 step='final';check('no browser JavaScript errors',report.page_errors.length===0);check('no invalid pattern errors',report.pattern_errors.length===0);check('no unexpected authentication or other writes',report.unexpected_writes.length===0);
 report.complete=true;save();console.log(JSON.stringify({complete:true,checks:report.checks.length,screenshots:report.screenshots.length}));await browser.close();
}catch(error){report.complete=false;report.failure={step,message:error.message};save();console.error('FAILED '+step+': '+error.message);if(browser)await browser.close();process.exitCode=1}})();
