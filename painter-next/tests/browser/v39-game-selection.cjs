// Source-only Vue regression. All HTTP responses and writes are in memory;
// no Painter server, production assets, account session or database is used.
const fs=require('fs'),path=require('path'),assert=require('node:assert/strict');
const frontend=path.resolve('web/frontend'),out=path.resolve('tests/artifacts/v39');
const {build}=require(path.join(frontend,'node_modules/esbuild'));
const {parse,compileScript}=require(path.join(frontend,'node_modules/@vue/compiler-sfc'));
const {chromium}=require('/tmp/painter-browser/node_modules/playwright');
const origin='https://painter-v39.test';
const checks=[],writes=[];
function check(name,ok,details){checks.push({name,passed:!!ok,...(details?{details}:{})});assert(ok,name);}
const sections=Object.fromEntries(['intro','articles','steam','music','photos','closing','activity','projects'].map(k=>[k,{enabled:true,title:k,description:'',limit:['intro','closing','activity'].includes(k)?0:5,link_label:'',link_url:''}]));
const home={version:1,max_image_bytes:10485760,config:{order:Object.keys(sections),sections,activity:{source:'posts_moments',timezone:'Asia/Shanghai',period:'rolling',year:0},intro_name:'Fixture',intro_card_name:'Fixture card',intro_roles:[],use_site_intro:false,writing_title:'',writing_description:'',scroll_label:'',motion_enabled:true,warmth_enabled:false},steam:{steam_id:'',mode:'owned',auto_sync_enabled:false,api_key_configured:false,last_sync_at:null,last_error:''}};
const row=(id,title,platform,provider,media,image)=>({id:String(id),version:7,kind:'steam',platform,platforms:platform==='psn'?['PS5']:[],provider_id:provider,source:platform==='psn'?'psn_sync':'steam_sync',title,subtitle:'',alt:'',url:platform==='psn'?'':'https://store.steampowered.com/app/'+provider+'/',media_id:media,image_url:image,hero_url:'',logo_url:'',playtime_minutes:null,position:id,visible:true,local_override:!!media});
let rows=[row(101,'Saved uploaded Steam','steam','730','saved-cover','/media/saved-cover'),row(102,'Existing remote Steam','steam','620','','https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/620/hash/original-header.jpg'),row(103,'Saved uploaded PSN','psn','PPSA_REAL_FORMAT_00','psn-cover','/media/psn-cover')],conflict=false;
const png=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==','base64');
let browser;
(async()=>{
 const result=await build({absWorkingDir:frontend,stdin:{contents:"import{createApp}from'vue';import Manager from'./src/components/HomepageManager.vue';import{session}from'./src/api';Object.assign(session,{loading:false,authenticated:true,user:{id:'fixture-owner',role:'owner'},csrf_token:'fixture-csrf'});createApp(Manager).mount('#app');",resolveDir:frontend,loader:'ts'},bundle:true,write:false,format:'iife',platform:'browser',plugins:[{name:'vue-in-memory',setup(b){b.onLoad({filter:/\.vue$/},async({path:p})=>{const {descriptor}=parse(fs.readFileSync(p,'utf8'),{filename:p});const s=compileScript(descriptor,{id:'v39-'+path.basename(p),inlineTemplate:true});return{contents:s.content,loader:'ts',resolveDir:path.dirname(p)}});b.onLoad({filter:/\.css$/},()=>({contents:'',loader:'js'}));}}]});
 const code=result.outputFiles[0].text,css=['style.css','homepageManager.css','gamePlatforms.css','controls.css'].map(f=>fs.readFileSync(path.join(frontend,'src',f),'utf8')).join('\n');
 browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});const c=await browser.newContext({viewport:{width:1440,height:1000}}),p=await c.newPage(),errors=[];p.on('pageerror',e=>errors.push(e.message));
 await c.route('**/*',async route=>{
  const req=route.request(),u=new URL(req.url()),method=req.method(),url=u.pathname;
  const json=(data,status=200)=>route.fulfill({status,json:data});
  if(u.origin!==origin)return route.fulfill({status:200,contentType:'image/png',body:png});
  if(url==='/')return route.fulfill({contentType:'text/html',body:'<!doctype html><html data-theme="light"><head><style>'+css+'</style></head><body data-page-kind="studio"><main style="max-width:1160px;margin:24px auto"><div id="app"></div></main><script src="/fixture.js"></script></body></html>'});
  if(url==='/fixture.js')return route.fulfill({contentType:'application/javascript',body:code});
  if(url.startsWith('/media/'))return route.fulfill({contentType:'image/png',body:png});
  if(method==='GET'){
   if(url==='/api/v1/admin/home')return json(home);
   if(url==='/api/v1/admin/home/projects')return json({items:[]});
   if(url==='/api/v1/admin/home/items'){const platform=u.searchParams.get('platform')||'all';return json({items:rows.filter(r=>platform==='all'||r.platform===platform),has_more:false,next_cursor:'',total:rows.length});}
   return json({error:{message:'Unexpected fixture GET '+url}},404);
  }
  // Every mutation is intercepted here and cannot reach a real application.
  const payload=(req.headers()['content-type']||'').includes('application/json')?req.postDataJSON():null,headers=req.headers();writes.push({url,method,payload,version:headers['if-match'],csrf:headers['x-csrf-token'],key:headers['idempotency-key']});
  if(url==='/api/v1/media'&&method==='POST')return json({state:'ready',media_id:'new-upload'});
  if(url.startsWith('/api/v1/admin/home/items/')&&method==='PUT'){
   if(conflict){conflict=false;return json({error:{code:'VERSION_CONFLICT',message:'Fixture version conflict'}},409);}
   const id=url.split('/').at(-1),index=rows.findIndex(r=>r.id===id),old=rows[index];
   if(headers['if-match']!==`"${old.version}"`)return json({error:{message:'Fixture stale version'}},409);
   rows[index]={...old,...payload,version:old.version+1,image_url:payload.media_id?'/media/'+payload.media_id:payload.image_url};return json(rows[index]);
  }
  return json({error:{message:'Unexpected fixture mutation'}},400);
 });
 await p.goto(origin+'/');await p.getByRole('tab',{name:'展示墙',exact:true}).click();await p.locator('.homepage-item-cell').first().waitFor();
 const table=p.locator('.homepage-items-table'),editor=p.locator('.homepage-item-editor');
 const findRow=title=>table.locator('tbody tr').filter({hasText:title});
 let checkbox=findRow('Saved uploaded Steam').getByRole('checkbox');await checkbox.uncheck();await p.waitForFunction(()=>document.querySelector('.homepage-feedback')?.textContent.includes('隐藏'));
 let write=writes.at(-1);check('visibility uses the current CAS version and CSRF/idempotency headers',write.version==='"7"'&&write.csrf==='fixture-csrf'&&!!write.key);
 check('visibility preserves the chosen media reference and platform',write.payload.media_id==='saved-cover'&&write.payload.platform==='steam'&&write.payload.image_url===''&&write.payload.visible===false);
 await findRow('Saved uploaded Steam').getByRole('button',{name:'编辑',exact:true}).click();await editor.locator('input[id$="-item-title"]').fill('Unsaved title');check('the same row visibility control cannot overwrite an unsaved draft',await findRow('Saved uploaded Steam').getByRole('checkbox').isDisabled());await editor.locator('input[id$="-item-title"]').fill('Saved uploaded Steam');
 await editor.getByRole('button',{name:'移除自选图片',exact:true}).click();await editor.locator('.homepage-item-advanced summary').click();
 check('removing a saved Steam upload restores its own validated official cover',await editor.getByLabel('平台图片地址',{exact:true}).inputValue()==='https://cdn.akamai.steamstatic.com/steam/apps/730/header.jpg');
 check('the restored remote cover is previewed',await editor.locator('.homepage-cover-preview img').getAttribute('src')==='https://cdn.akamai.steamstatic.com/steam/apps/730/header.jpg');
 await editor.getByRole('button',{name:'保存条目',exact:true}).click();await p.waitForFunction(()=>document.querySelector('.homepage-item-save>span')?.textContent==='没有未保存的修改');write=writes.at(-1);check('removal submits a nonempty official image with no media attachment',write.payload.media_id===''&&write.payload.image_url==='https://cdn.akamai.steamstatic.com/steam/apps/730/header.jpg'&&write.version==='"8"');
 await findRow('Existing remote Steam').getByRole('button',{name:'编辑',exact:true}).click();const original=await editor.getByLabel('平台图片地址',{exact:true}).inputValue();
 await editor.locator('input[type=file]').setInputFiles({name:'cover.png',mimeType:'image/png',buffer:png});await editor.getByRole('button',{name:'移除自选图片',exact:true}).waitFor();await editor.getByRole('button',{name:'移除自选图片',exact:true}).click();
 check('undoing an unsaved upload restores the exact prior remote cover',await editor.getByLabel('平台图片地址',{exact:true}).inputValue()===original&&await editor.getByRole('button',{name:'保存条目',exact:true}).isDisabled());
 conflict=true;await findRow('Existing remote Steam').getByRole('checkbox').uncheck();await p.getByRole('alert').filter({hasText:'Fixture version conflict'}).waitFor();check('a rejected visibility change restores its previous checked state',await findRow('Existing remote Steam').getByRole('checkbox').isChecked());
 await p.locator('.homepage-platform-filter').getByRole('button',{name:'PlayStation',exact:true}).click();await findRow('Saved uploaded PSN').waitFor();check('platform filtering keeps only its own real rows',await table.locator('.homepage-item-cell').count()===1);
 await findRow('Saved uploaded PSN').getByRole('button',{name:'编辑',exact:true}).click();await editor.getByRole('button',{name:'移除自选图片',exact:true}).click();await editor.getByRole('button',{name:'保存条目',exact:true}).click();await p.waitForFunction(()=>document.querySelector('.homepage-item-save>span')?.textContent==='没有未保存的修改');write=writes.at(-1);check('PSN may keep missing art without receiving a fabricated Steam cover',write.payload.platform==='psn'&&write.payload.media_id===''&&write.payload.image_url===''&&write.payload.platforms[0]==='PS5');
 check('game visibility wording describes its public-library scope',await p.locator('.homepage-wall-toolbar').first().innerText().then(t=>t.includes('公开展示同时用于首页和完整游戏墙'))&&await editor.getByRole('checkbox',{name:'在游戏墙公开展示',exact:true}).count()===1);
 check('no component runtime errors',!errors.length,{errors});
 fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'source-selection-report.json'),JSON.stringify({passed:true,in_memory_only:true,checks,writes:writes.map(w=>({url:w.url,method:w.method,version:w.version,has_csrf:!!w.csrf,has_idempotency_key:!!w.key}))},null,2));console.log(JSON.stringify({passed:true,checks:checks.length,in_memory_only:true}));await browser.close();
})().catch(async e=>{console.error(e.stack);await browser?.close();process.exitCode=1});
