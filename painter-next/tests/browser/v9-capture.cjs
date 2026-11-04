// Read-only screenshots of the isolated V9 hierarchy, using private login states.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),fs=require('fs'),path=require('path');
if(!process.env.STATE_PREFIX)throw Error('Private STATE_PREFIX required');
const base=process.env.BASE_URL||'http://192.168.2.209:18081',out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v9/visual'),files=[],metrics=[],errors=[];let b;
async function shot(p,file){await p.screenshot({path:out+'/'+file,fullPage:false});files.push(file);metrics.push({file,viewport:p.viewportSize(),overflow:await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth),navigation:await p.locator('.studio-links').evaluate(nav=>({groups:Array.from(nav.querySelectorAll('.studio-section-trigger')).map(e=>({label:e.textContent.trim(),expanded:e.getAttribute('aria-expanded')==='true'})),scrollTop:nav.scrollTop,scrollHeight:nav.scrollHeight,clientHeight:nav.clientHeight}))})}
(async()=>{fs.mkdirSync(out,{recursive:true});b=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox']});
 for(const [size,width,height]of[['desktop',1440,1000],['tablet',900,1000],['mobile',390,900]])for(const theme of['light','dark']){
  const c=await b.newContext({viewport:{width,height},colorScheme:theme,storageState:process.env.STATE_PREFIX+'-owner-state.json'}),p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));await p.addInitScript(t=>localStorage.setItem('painter-theme',t),theme);
  await p.goto(base+'/studio/posts');await p.getByRole('heading',{name:'文章管理',exact:true}).waitFor();await shot(p,`${size}-${theme}-articles.png`);
  if(size==='desktop'&&theme==='light'){for(const group of ['数据与运维','站点管理','个人空间']){const trigger=p.getByRole('button',{name:group,exact:true});if(await trigger.getAttribute('aria-expanded')!=='true')await trigger.click()}await p.waitForFunction(()=>document.querySelectorAll('.studio-section-trigger[aria-expanded=true]').length===4);await shot(p,'desktop-light-all-groups.png')}
  await p.goto(base+'/studio/settings#security');await p.locator('.github-settings input').first().waitFor();await shot(p,`${size}-${theme}-settings.png`);
  if(width<=960){await p.getByRole('button',{name:'打开工作台导航',exact:true}).click();await shot(p,`${size}-${theme}-drawer.png`);await p.keyboard.press('Escape')}
  await c.close();
 }
 for(const [role,width,theme,route]of[['reader',1440,'light','/me/bookmarks'],['author',1440,'dark','/studio/analytics'],['reader',390,'dark','/me/media']]){
  const c=await b.newContext({viewport:{width,height:1000},colorScheme:theme,storageState:process.env.STATE_PREFIX+'-'+role+'-state.json'}),p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));await p.addInitScript(t=>localStorage.setItem('painter-theme',t),theme);await p.goto(base+route);await p.locator('.work-heading h1').waitFor();if(width<961)await p.getByRole('button',{name:'打开工作台导航',exact:true}).click();await shot(p,`${role}-${width}-${theme}.png`);await c.close();
 }
 fs.writeFileSync(out+'/report.json',JSON.stringify({origin:base,files,metrics,errors,capture_mode:'viewport; all-expanded navigation scrolls independently',data_writes:0},null,2));if(errors.length||metrics.some(x=>x.overflow))throw Error('Runtime errors or horizontal overflow');console.log(JSON.stringify({captures:files.length,allViewportsFit:true}));await b.close();
})().catch(async e=>{fs.writeFileSync(out+'/capture-failed.json',JSON.stringify({files,metrics,errors,failure:e.message},null,2));console.error(e.message);await b?.close();process.exitCode=1});
