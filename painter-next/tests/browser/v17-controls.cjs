// V17 control inventory and read-only UI verification. This never submits data.
// Baseline: PHASE=baseline BASE_URL=http://192.168.2.209:18080 ... node <file>
// Final: PHASE=final BASE_URL=http://192.168.2.209:18081 ... node <file>
// Capture only once after candidate approval: CAPTURE_SCREENSHOTS=1 (12 crops).
const fs=require('fs'),path=require('path');
const{chromium}=require(process.env.PLAYWRIGHT_MODULE||'/tmp/painter-browser/node_modules/playwright');
const phase=process.env.PHASE||'baseline',base=(process.env.BASE_URL||'http://192.168.2.209:18080').replace(/\/$/,''),origin=new URL(base).origin;
if(!['baseline','final'].includes(phase))throw Error('PHASE must be baseline or final');
if(phase==='final'&&new URL(base).port==='18080')throw Error('Final controls verification refuses the formal service');
const capture=process.env.CAPTURE_SCREENSHOTS==='1',runName=capture?'capture':phase;if(capture&&phase!=='final')throw Error('Baseline never captures screenshots');
const stateFile=process.env.OWNER_STATE_FILE||'/tmp/painter-next-v16-formal-state.json',fixtureFile=process.env.FIXTURE_FILE||'/tmp/painter-v16-fixtures.json',out=path.resolve(process.env.ARTIFACT_DIR||'tests/artifacts/v17/controls');
const privateState=JSON.parse(fs.readFileSync(stateFile,'utf8')),fixtures=JSON.parse(fs.readFileSync(fixtureFile,'utf8'));
const privateValues=(privateState.cookies||[]).map(c=>c.value).filter(Boolean),redact=s=>privateValues.reduce((v,p)=>v.split(p).join('[redacted]'),String(s));
const checks=[],metrics=[],errors=[],intercepted=[],captures=[],notes=[];let browser,scenario='setup',editorID='';
function check(name,ok,detail,layout=false){const row={scenario,name,passed:!!ok,enforced:phase==='final'||!layout,...(detail?{detail}: {})};checks.push(row);if(!ok&&row.enforced&&!layout)throw Error(`${scenario}: ${name}${detail?' '+JSON.stringify(detail):''}`)}
const close=(a,b)=>Math.abs(a-b)<=1;
async function settle(p){await p.evaluate(()=>document.fonts.ready);await p.evaluate(()=>new Promise(requestAnimationFrame))}
async function context(width,theme,coarse=false){
 const c=await browser.newContext({storageState:stateFile,viewport:{width,height:1000},colorScheme:theme,hasTouch:coarse,serviceWorkers:'block'});
 const session=await c.request.get(base+'/api/v1/session').then(r=>r.json());check('private owner session is valid for this service',session.authenticated===true&&session.user?.role==='owner');
 await c.addInitScript(t=>localStorage.setItem('painter-theme',t),theme);
 await c.route('**/api/v1/**',async r=>{
  const q=r.request(),u=new URL(q.url());if(u.origin!==origin)return r.continue();
  if(u.pathname==='/api/v1/analytics/events')return r.fulfill({json:{accepted:0}});
  if(['GET','HEAD'].includes(q.method())||['/api/v1/editor/preview','/api/v1/editor/cover-preview'].includes(u.pathname))return r.continue();
  intercepted.push({scenario,method:q.method(),path:u.pathname});
  return r.fulfill({status:409,json:{error:{code:'V17_READ_ONLY',message:'V17 控件验收禁止写入。'}}});
 });
 const p=await c.newPage();p.setDefaultTimeout(10000);p.on('pageerror',e=>errors.push({scenario,message:redact(e.message)}));
 const density=width<=720||coarse?44:36;return{c,p,density};
}
async function goto(p,route,ready){await p.goto(base+route);if(ready)await p.locator(ready).first().waitFor();await settle(p);check('page has no horizontal overflow',await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),{route},true)}
async function measure(p,selector,name,expected={}){
 const values=await p.locator(selector).evaluateAll(els=>els.filter(e=>e.getClientRects().length&&getComputedStyle(e).visibility!=='hidden').map(e=>{
  const r=e.getBoundingClientRect(),s=getComputedStyle(e),walker=document.createTreeWalker(e,NodeFilter.SHOW_TEXT),rects=[];for(let n=walker.nextNode();n;n=walker.nextNode()){if(!n.textContent.trim())continue;const range=document.createRange();range.selectNodeContents(n);rects.push(...Array.from(range.getClientRects()).filter(r=>r.height>0&&r.width>0))}const textHeight=rects.length?Math.max(...rects.map(r=>r.bottom))-Math.min(...rects.map(r=>r.top)):0;return{text:(e.innerText||e.getAttribute('aria-label')||'').trim().slice(0,50),x:r.x,y:r.y,w:r.width,h:r.height,font:parseFloat(s.fontSize),line:parseFloat(s.lineHeight),textHeight,radius:parseFloat(s.borderTopLeftRadius),radii:[s.borderTopLeftRadius,s.borderTopRightRadius,s.borderBottomRightRadius,s.borderBottomLeftRadius].map(parseFloat),padding:[s.paddingTop,s.paddingRight,s.paddingBottom,s.paddingLeft],scrollWidth:e.scrollWidth,clientWidth:e.clientWidth};
 }));
 metrics.push({scenario,name,selector,values});check(name+' exists',values.length>0);
 if(expected.height!==undefined)check(name+' height',values.every(v=>close(v.h,expected.height)||(expected.allowMultiline&&v.h>=expected.height&&v.textHeight>28)),{expected:expected.height,actual:values.map(v=>v.h),textHeight:values.map(v=>v.textHeight)},true);
 if(expected.radius!==undefined)check(name+' radius',values.every(v=>v.radii.every(r=>close(r,expected.radius))),{expected:expected.radius,actual:values.map(v=>v.radii)},true);
 if(expected.font!==false&&expected.font!==undefined)check(name+' type',values.every(v=>close(v.font,expected.font)&&close(v.line,20)),{expected:[expected.font,20],actual:values.map(v=>[v.font,v.line])},true);
 return values;
}
async function bounds(p,selector,name){const r=await p.locator(selector).first().boundingBox();check(name+' stays inside viewport',!!r&&r.x>=7&&r.y>=7&&r.x+r.width<=p.viewportSize().width-7&&r.y+r.height<=1000-7,r,true)}
async function crop(p,name,selectors){
 if(!capture)return;if(captures.length>=12)throw Error('V17 capture budget exceeded');
 // These crops use viewport coordinates while disabling beyond-viewport capture.
 // Focus the control before computing a union that includes its open popup.
 const rect=await p.evaluate(selectors=>{const rs=selectors.flatMap(s=>Array.from(document.querySelectorAll(s))).filter(e=>e.getClientRects().length).map(e=>e.getBoundingClientRect()).filter(r=>r.bottom>0&&r.top<innerHeight);if(!rs.length)return null;const x=Math.max(0,Math.floor(Math.min(...rs.map(r=>r.left))-12)),y=Math.max(0,Math.floor(Math.min(...rs.map(r=>r.top))-12));return{x,y,width:Math.min(innerWidth-x,Math.ceil(Math.max(...rs.map(r=>r.right))+12-x)),height:Math.min(innerHeight-y,Math.ceil(Math.max(...rs.map(r=>r.bottom))+12-y))}},selectors);
 if(!rect)throw Error('No visible crop for '+name);
 const file=`${scenario}-${name}.png`;await p.screenshot({path:path.join(out,file),clip:rect,animations:'disabled'});captures.push(file);
}
async function select(p,trigger,density,name,snap=false){
 await trigger.scrollIntoViewIfNeeded();await trigger.click();await p.locator('.select-menu').waitFor();
 await measure(p,'.select-menu',name+' menu',{radius:8,font:13});await measure(p,'.select-menu [role=option]',name+' options',{height:density,radius:4,font:13});
 check(name+' menu is teleported',await p.locator('.select-menu').evaluate(e=>e.parentElement===document.body||e.parentElement?.tagName==='DIALOG'));
 await bounds(p,'.select-menu',name);const before=await trigger.getAttribute('aria-activedescendant');await trigger.press('ArrowDown');const down=await trigger.getAttribute('aria-activedescendant');
 check(name+' ArrowDown moves active option',await p.locator('.select-menu [role=option]:not([aria-disabled=true])').count()<2||before!==down);
 await trigger.press('ArrowUp');check(name+' ArrowUp restores active option',await trigger.getAttribute('aria-activedescendant')===before);
 if(snap)await crop(p,'paragraph',['.document-block-type','.select-menu']);
 await trigger.press('Escape');check(name+' Escape closes and restores focus',!await p.locator('.select-menu').count()&&await trigger.evaluate(e=>document.activeElement===e));
}
async function search(p,density,snap,theme){
 await goto(p,'/search','.search-page-form');await measure(p,'.search-filter summary','search filter triggers',{height:density,radius:6,font:13});
 await measure(p,'.search-query-field','search query shell',{height:density,radius:6});await measure(p,'#search-query','search query inner field',{height:density-2,font:density===44?16:14});
 const tags=p.locator('[data-tag-filter]');await tags.locator('summary').click();await p.locator('.search-tag-options .check').first().waitFor();
 await measure(p,'.search-tag-options .check','search tag options',{height:density,radius:4,font:13,allowMultiline:true});await measure(p,'.search-tag-options input','search checkbox marks',{height:16,radius:4});await measure(p,'[data-tag-filter] .search-filter-popover','search tags popup',{radius:8});await measure(p,'#filter-tag-query','tag query field',{height:density,radius:6,font:density===44?16:14});await bounds(p,'[data-tag-filter] .search-filter-popover','search tags');
 if(snap&&theme==='light')await crop(p,'search-tags',['[data-tag-filter] summary','[data-tag-filter] .search-filter-popover']);
 await tags.locator('summary').press('Escape');check('search tag Escape closes',!await tags.evaluate(e=>e.open));
 const sort=p.locator('[data-sort-filter]');await sort.locator('summary').click();await measure(p,'.sort-options .check','sort choices',{height:density,radius:4,font:13,allowMultiline:true});await measure(p,'.sort-options','sort popup',{radius:8});
 check('sort radio stays circular',await p.locator('.sort-options input').evaluateAll(els=>els.every(e=>{const s=getComputedStyle(e);return s.borderTopLeftRadius==='50%'})),null,true);await bounds(p,'.sort-options','sort menu');
 await measure(p,'.sort-options input','search radio marks',{height:16});
 if(snap&&theme==='dark')await crop(p,'search-sort',['[data-sort-filter] summary','.sort-options']);await sort.locator('summary').press('Escape');check('sort Escape closes',!await sort.evaluate(e=>e.open));
 const themePanel=p.locator('.floating-theme');await themePanel.locator('summary').click();await measure(p,'.theme-popover','theme popup',{radius:8});await measure(p,'.theme-choices button','theme choices',{height:density,radius:4,font:13});await bounds(p,'.theme-popover','theme menu');
 const chosen=p.locator('[data-theme-option][aria-checked=true]');await chosen.focus();await chosen.press('ArrowDown');check('theme arrows select one radio',await p.locator('[data-theme-option][aria-checked=true]').count()===1);await p.locator(`[data-theme-option="${theme}"]`).click();
 if(snap&&theme==='dark')await crop(p,'theme',['.theme-popover','.floating-theme>summary']);await p.keyboard.press('Escape');check('theme Escape closes',!await themePanel.evaluate(e=>e.open));
 const header=await p.locator('.site-header').evaluate(e=>parseFloat(getComputedStyle(e).borderTopLeftRadius));check('navigation retains its own radius',header===(p.viewportSize().width<=720?20:24),{actual:header},true);
 await goto(p,'/users/'+fixtures.authorID,'.author-search-field');await measure(p,'.author-search-field','author search shell',{height:density,radius:6});await measure(p,'.author-search-field input','author search inner field',{height:density-2,font:density===44?16:14});await measure(p,'.author-search>button','author search action',{height:density,radius:6,font:13});
}
async function posts(p,density){
 await goto(p,'/studio/posts','.manager-table tbody tr');await measure(p,'.manager-scope button','article scope',{height:density,radius:6,font:13});await measure(p,'.manager-status-tabs button','article state tabs',{height:density,radius:0,font:13});await measure(p,'.manager-table-wrap','table container',{radius:12});
 await measure(p,'.manager-search','manager search shell',{height:density,radius:6});await measure(p,'.manager-search input','manager search inner field',{height:density-2,font:density===44?16:14});await measure(p,'.manager-search button','manager inset search action',{height:density-8,radius:6,font:13});
 const links=await p.locator('.manager-title>a').evaluateAll(els=>els.map(e=>e.getAttribute('href'))),known=fixtures.posts?.find(p=>p.role==='owner')?.id;
 if(!editorID){editorID=(known&&links.includes('/studio/posts/'+known)?known:links[0].split('/').pop());notes.push({name:'editor fixture selection',source:known===editorID?'supplied fixture mapping':'first available existing article'})}
 const side=p.locator('.studio-sidebar-bottom .select-trigger');if(await side.isVisible()){await measure(p,'.studio-sidebar-bottom .select-trigger','sidebar theme trigger',{height:density,radius:6,font:13});await select(p,side,density,'bottom sidebar select')}
 const action=p.locator('.row-menu-trigger').first();await action.click();await measure(p,'.row-menu-panel','table actions popup',{radius:8});await measure(p,'.row-menu-panel>*','table actions',{height:density,radius:4,font:13});await bounds(p,'.row-menu-panel','row actions');await p.keyboard.press('Escape');check('table action Escape closes',!await p.locator('.row-menu-panel').count());
}
async function editor(p,density,snap,theme){
 await goto(p,'/studio/posts/'+editorID,'.document-prose');await p.waitForFunction(()=>!document.querySelector('.editor-title')?.disabled);const info=p.locator('.editor-properties');if(!await info.evaluate(e=>e.open))await info.locator('summary').click();
 await measure(p,'.document-block-type .select-trigger','paragraph trigger',{height:density,radius:6,font:13});await measure(p,'.document-toolbar button:not(.select-trigger)','document format buttons',{height:density,radius:6,font:13});
 await select(p,p.getByRole('combobox',{name:'段落样式',exact:true}),density,'paragraph select',snap&&theme==='light'&&p.viewportSize().width===1440);
 await measure(p,'.cover-options .select-trigger','cover setting triggers',{height:density,radius:6,font:13});
 await measure(p,'.tag-control','editor tag control',{height:density,radius:6});await measure(p,'.tag-search,.tag-toggle','tag control inner row',{height:density-8,font:13});
 const selected=await p.locator('.selected-tag').count();check('fixture exercises an already selected tag',selected>0,null,true);if(selected)await measure(p,'.selected-tag','selected editor tags',{height:density-8,radius:4,font:13});
 const input=p.getByRole('combobox',{name:'标签',exact:true});await input.focus();await p.locator('.tag-dropdown').waitFor();await measure(p,'.tag-dropdown','editor tag popup',{radius:8});await measure(p,'.tag-option','editor tag options',{height:density,radius:4,font:13});await bounds(p,'.tag-dropdown','editor tags');
 const first=await input.getAttribute('aria-activedescendant');await input.press('ArrowDown');check('tag arrow moves active choice',first!==await input.getAttribute('aria-activedescendant')||await p.locator('.tag-option').count()===1);await input.press('ArrowUp');
 if(snap&&theme==='dark')await crop(p,'editor-tags',['.tag-picker legend','.tag-control','.tag-dropdown']);await input.press('Escape');check('tag Escape closes',!await p.locator('.tag-dropdown').count());
 await p.evaluate(()=>scrollTo(0,0));const more=p.locator('.editor-more');await measure(p,'.editor-more>summary','editor More trigger',{height:density,radius:6,font:13});await more.locator('summary').click();await measure(p,'.editor-more-menu','editor More popup',{radius:8});await measure(p,'.editor-more-menu button','editor More options',{height:density,radius:4,font:13});await bounds(p,'.editor-more-menu','editor More');await more.locator('summary').press('Escape');check('editor More Escape closes',!await more.evaluate(e=>e.open));
 await more.locator('summary').click();await p.getByRole('button',{name:'编辑 Markdown 源码',exact:true}).click();await p.locator('.cm-content').waitFor();await measure(p,'.view-switch','source segmented frame',{height:density,radius:6});await measure(p,'.view-switch button','source view choices',{height:density-8,radius:4,font:13});
 if(snap&&theme==='light'&&p.viewportSize().width===390){await p.locator('.editor-tools').scrollIntoViewIfNeeded();await crop(p,'source-segments',['.editor-tools','.view-switch'])}
 check('editor page retains no horizontal overflow',await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),null,true);
}
async function palette(p,density,snap,theme){
 await goto(p,'/me','.author-palette-options label');await measure(p,'.author-palette-options label','author palette choices',{height:density,radius:6,font:13});
 const values=await p.locator('.author-palette-options label').evaluateAll(els=>els.map(e=>{const a=e.querySelector('input').getBoundingClientRect(),b=e.querySelector('.author-background-swatch').getBoundingClientRect(),c=e.querySelector('span:last-child').getBoundingClientRect();return{swatch:[b.width,b.height],centers:[a.y+a.height/2,b.y+b.height/2,c.y+c.height/2]}}));metrics.push({scenario,name:'palette row geometry',values});
 check('palette swatch name and radio share one compact row',values.every(v=>Math.max(...v.centers)-Math.min(...v.centers)<2&&v.swatch[1]<=24),values,true);
 check('palette radio remains circular',await p.locator('.author-palette-options input').evaluateAll(els=>els.every(e=>getComputedStyle(e).borderTopLeftRadius==='50%')),null,true);
 await measure(p,'.author-palette-options input','palette radio marks',{height:16});await measure(p,'.work-main .form-grid input:not([type=file]):not([type=checkbox]):not([type=radio])','ordinary profile fields',{height:density,radius:6,font:density===44?16:14});
 if(snap&&theme==='light'){await p.locator('.author-palette-picker').scrollIntoViewIfNeeded();await crop(p,'author-palette',['.author-palette-picker'])}
}
async function managers(p,density){
 await goto(p,'/studio/taxonomy','.manager-toolbar .select-trigger');await measure(p,'.manager-toolbar .select-trigger','tag state trigger',{height:density,radius:6,font:13});await select(p,p.getByRole('combobox',{name:'标签状态',exact:true}),density,'tag state select');
 await goto(p,'/studio/phrases','.phrase-composer .select-trigger');await measure(p,'.phrase-composer .select-trigger','phrase type trigger',{height:density,radius:6,font:13});await select(p,p.getByRole('combobox',{name:'内容类型',exact:true}),density,'phrase type select');await measure(p,'.phrase-library-tools .tabs button','phrase state choices',{height:density,radius:6,font:13});
 await goto(p,'/studio/friends','.friends-library-filter .select-trigger');await measure(p,'.friends-library-filter .select-trigger','friend state trigger',{height:density,radius:6,font:13});await select(p,p.getByRole('combobox',{name:'展示状态',exact:true}),density,'friend state select');await measure(p,'.friends-manager-bar .settings-tabs button','friend section tabs',{height:density,radius:0,font:13});
}
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE||'/usr/bin/google-chrome',headless:true,args:['--no-sandbox','--disable-dev-shm-usage']});
 const widths=(process.env.CONTROL_WIDTHS||(phase==='baseline'||capture?'1440,390':'1440,900,720,390,320')).split(',').map(Number),themes=(process.env.CONTROL_THEMES||'light,dark').split(',');
 for(const width of widths)for(const theme of themes){scenario=`${width}-${theme}`;const{c,p,density}=await context(width,theme);await posts(p,density);await search(p,density,capture&&[1440,390].includes(width),theme);await editor(p,density,capture&&[1440,390].includes(width),theme);await palette(p,density,capture&&[1440,390].includes(width),theme);await managers(p,density);await c.close()}
 if(phase==='final'&&!capture&&process.env.SKIP_COARSE!=='1'){scenario='1440-light-coarse';const{c,p,density}=await context(1440,'light',true);check('coarse pointer contract is active',await p.evaluate(()=>matchMedia('(pointer:coarse)').matches));await posts(p,density);await search(p,density,false,'light');await editor(p,density,false,'light');await palette(p,density,false,'light');await managers(p,density);await c.close()}
 check('no browser runtime errors',errors.length===0);check('no unexpected content or settings write attempts',intercepted.length===0,{intercepted});check('capture budget respected',captures.length<13);
 const failed=checks.filter(c=>!c.passed&&c.enforced),passed=failed.length===0,report={phase,run_kind:capture?'capture':'dom',origin:base,passed,checks,metrics,errors,intercepted_mutations:intercepted,actual_business_writes:0,captures,screenshots_written:capture,notes};fs.writeFileSync(path.join(out,runName+'.json'),JSON.stringify(report,null,2));if(passed)fs.rmSync(path.join(out,runName+'-failure.json'),{force:true});else fs.writeFileSync(path.join(out,runName+'-failure.json'),JSON.stringify({phase,failed,actual_business_writes:0,captures},null,2));console.log(JSON.stringify({phase,run:runName,passed,checks:checks.length,observed_target_mismatches:checks.filter(c=>!c.passed).length,captures:captures.length,actual_business_writes:0}));if(!passed)process.exitCode=1;await browser.close();
})().catch(async e=>{fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,runName+'-failure.json'),JSON.stringify({phase,checks,metrics,errors,intercepted_mutations:intercepted,actual_business_writes:0,captures,error:redact(e.message)},null,2));console.error(redact(e.message));await browser?.close();process.exitCode=1});
