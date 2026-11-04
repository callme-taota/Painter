// Viewport captures on the isolated V8 clone. Fixture copy is explicitly synthetic.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),fs=require('fs'),path=require('path');
if(!process.env.BROWSER_STATE_FILE)throw Error('Private owner state required');
const base=process.env.BASE_URL||'http://192.168.2.209:18081',out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v8/visual'),report=JSON.parse(fs.readFileSync('tests/artifacts/v8/experience.json','utf8')),files=[],metrics=[],errors=[];let b;
async function shot(p,name){await p.screenshot({path:out+'/'+name,fullPage:false});files.push(name);metrics.push({file:name,overflow:await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth),viewport:p.viewportSize()})}
(async()=>{fs.mkdirSync(out,{recursive:true});b=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true,args:['--no-sandbox','--enable-unsafe-swiftshader']});
 for(const [size,width,height]of[['desktop',1440,1000],['tablet',900,1000],['mobile',390,900]])for(const theme of['light','dark']){
  const c=await b.newContext({storageState:process.env.BROWSER_STATE_FILE,viewport:{width,height},colorScheme:theme}),p=await c.newPage();p.on('pageerror',e=>errors.push(e.message));await p.addInitScript(t=>localStorage.setItem('painter-theme',t),theme);
  await p.goto(base+'/studio/posts');await p.locator('.post-management-table').waitFor();await shot(p,`${size}-${theme}-posts.png`);
  await p.goto(base+'/studio/posts/'+report.qa_post_id);await p.getByRole('textbox',{name:'文档正文',exact:true}).waitFor();await p.locator('.editor-properties>summary').click();await p.getByRole('combobox',{name:'版式',exact:true}).click();await shot(p,`${size}-${theme}-editor-select.png`);await p.keyboard.press('Escape');
  await p.goto(base+'/studio/phrases');await p.locator('.phrase-library-row').first().waitFor();await shot(p,`${size}-${theme}-manager.png`);
  await p.goto(base+'/phrases');await p.locator('[data-phrase-item]').first().waitFor();await p.waitForTimeout(100);await shot(p,`${size}-${theme}-phrases.png`);
  if(size==='desktop'&&theme==='light'||size==='mobile'&&theme==='dark'){
   await p.goto(base+'/studio/settings#security');await p.locator('.github-settings input').first().waitFor();await shot(p,`${size}-${theme}-settings.png`);
   await p.goto(base+'/studio/members');await p.getByRole('combobox',{name:/^角色 · /}).first().click();await shot(p,`${size}-${theme}-members-select.png`);await p.keyboard.press('Escape');
   await p.goto(base+'/phrases#phrase-'+report.qa_phrase_ids[1]);await p.locator('.phrase-word').scrollIntoViewIfNeeded();await p.waitForTimeout(150);await shot(p,`${size}-${theme}-word.png`);
   await p.goto(base+'/phrases?cursor=1');await p.locator('.phrase-public-empty').waitFor();await shot(p,`${size}-${theme}-public-empty-cursor.png`);
  }
  if(size==='mobile'){await p.goto(base+'/studio/phrases');await p.getByRole('button',{name:'打开工作台导航',exact:true}).click();await p.getByRole('dialog',{name:'工作台导航',exact:true}).getByRole('combobox',{name:'外观主题',exact:true}).click();await shot(p,`${size}-${theme}-drawer-theme.png`)}
  await c.close();
 }
 fs.writeFileSync(out+'/report.json',JSON.stringify({origin:base,files,metrics,errors,capture_mode:'viewport; interactive editor and word states may be scrolled',fixture_content:'synthetic QA copy in clone only; no formal phrase seeded',empty_view:'actual SSR cursor=1 with no results'},null,2));if(errors.length||metrics.some(m=>m.overflow))throw Error('Browser errors or horizontal overflow');console.log(JSON.stringify({captures:files.length,allViewportsFit:true}));await b.close();
})().catch(async e=>{fs.writeFileSync(out+'/capture-failed.json',JSON.stringify({files,metrics,errors,failure:e.message},null,2));console.error(e.message);await b?.close();process.exitCode=1});
