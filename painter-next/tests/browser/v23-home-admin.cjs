// V23 admin contract: all real writes stay on the isolated :18081 candidate.
// No screenshots, formal state reads, or existing-item deletion.
const fs=require('fs'),crypto=require('crypto'),{chromium}=require('/tmp/painter-browser/node_modules/playwright');
const base='http://192.168.2.209:18081',out='tests/artifacts/v23/admin-browser.json';
const playlistURL='https://music.apple.com/us/playlist/%E9%80%83%E5%A1%94%E5%96%9C%E6%AC%A2%E7%9A%84%E9%9F%B3%E4%B9%90/pl.u-KVXBkAWuZqb1BXV';
const checks=[],errors=[],expectedNetworkFailures=[],evidence={};let browser,owner,tempID='',csrf='',page;
const check=(name,passed,detail)=>{checks.push({name,passed:!!passed,...(detail===undefined?{}:{detail})});if(!passed)throw Error(name)};
async function api(method,path,data,version){
 if(new URL(base).port!=='18081')throw Error('Candidate-only safety guard');
 const r=await owner.request.fetch(base+'/api/v1'+path,{method,headers:{Origin:base,'X-CSRF-Token':csrf,'Idempotency-Key':crypto.randomUUID(),...(version===undefined?{}:{'If-Match':'"'+version+'"'})},...(data===undefined?{}:{data})});
 if(!r.ok())throw Error(method+' '+path+' HTTP '+r.status());return r.status()===204?{}:r.json();
}
async function allMusic(){let cursor='',rows=[];do{const data=await api('GET','/admin/home/items?kind=music&limit=50'+(cursor?'&cursor='+encodeURIComponent(cursor):''));rows.push(...data.items);cursor=data.has_more?(data.next_cursor||data.cursor||''):'';if(data.has_more&&!cursor)throw Error('Pagination cursor absent')}while(cursor);return rows}
async function ready(p){await p.locator('.homepage-sections-table tbody tr').nth(5).waitFor();await p.getByRole('button',{name:'保存首页设置',exact:true}).waitFor()}
async function idle(p){await p.waitForFunction(()=>!document.querySelector('.homepage-wall-toolbar [role=combobox]')?.hasAttribute('disabled')&&!document.querySelector('.homepage-table-scroll[aria-busy="true"]'))}
async function choose(p,label){await p.getByRole('combobox',{name:'展示墙',exact:true}).click();await p.getByRole('option',{name:label,exact:true}).click();await idle(p)}
async function openImport(p){const details=p.locator('.homepage-import');if(await details.getAttribute('open')===null)await details.locator('summary').click()}
async function confirmImport(p){await p.locator('.homepage-playlist-actions button.primary').click();const d=p.getByRole('dialog');await d.waitFor();check('confirm discloses visible/public and partial snapshot',await d.innerText().then(t=>t.includes('新增条目设为显示')&&t.includes('按首页数量公开展示')&&t.includes('只保存已读取部分')));await d.getByRole('button',{name:'确认保存本次专辑',exact:true}).click()}
async function readPreview(p,button='读取歌单预览',snapshot=false){const response=p.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/admin/home/music-playlist/preview'&&r.request().method()==='POST');await p.getByRole('button',{name:button,exact:true}).click();const r=await response;check((snapshot?'geometry snapshot':'real playlist preview')+' HTTP 200',r.status()===200);const data=await r.json();await p.locator('.homepage-playlist-preview').waitFor();await idle(p);return data}
async function cleanup(){if(owner&&tempID){const latest=await api('GET','/admin/home/items/'+tempID);await api('DELETE','/admin/home/items/'+tempID,undefined,latest.version);tempID=''}}
function save(passed,error){fs.mkdirSync('tests/artifacts/v23',{recursive:true});fs.writeFileSync(out,JSON.stringify({passed,origin:base,checks,errors,expected_network_failures:expectedNetworkFailures,evidence,formal_business_writes:0,screenshots:0,...(error?{error:String(error)}:{})},null,2))}
(async()=>{
 browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
 owner=await browser.newContext({storageState:'/tmp/painter-next-v16-owner-state.json',viewport:{width:1440,height:1000}});
 owner.setDefaultTimeout(20000);owner.setDefaultNavigationTimeout(30000);
 await owner.route('**/api/v1/analytics/events',r=>r.fulfill({json:{accepted:0}}));
 const session=await owner.request.get(base+'/api/v1/session').then(r=>r.json());check('candidate owner session',session.user?.role==='owner');csrf=session.csrf_token;
 page=await owner.newPage();page.on('pageerror',e=>errors.push(e.message));page.on('dialog',d=>d.type()==='beforeunload'?d.accept():d.dismiss());
 page.on('requestfailed',r=>{if(r.failure()?.errorText!=='net::ERR_ABORTED')expectedNetworkFailures.push({path:new URL(r.url()).pathname,error:r.failure()?.errorText})});
 const response=await page.goto(base+'/studio/home');check('owner homepage manager HTTP 200',response.status()===200);await ready(page);
 evidence.asset=await page.locator('script[src]').evaluateAll(es=>es.map(e=>e.getAttribute('src')).filter(x=>x.includes('studio-')));
 check('three existing admin tabs',await page.getByRole('tablist',{name:'首页管理分区'}).getByRole('tab').count()===3);
 check('no native select',await page.locator('select').count()===0);
 await page.getByRole('button',{name:'编辑游戏墙内容',exact:true}).click();
 const section=page.locator('.homepage-section-editor'),homeLink=section.getByRole('textbox',{name:/^链接地址/});
 check('home games max-five help preserves legacy max24',await section.innerText().then(t=>t.includes('首页最多5项')&&t.includes('完整游戏墙展示全部'))&&await section.getByRole('spinbutton',{name:/^显示数量/}).getAttribute('max')==='24');
 check('games destination readonly /games',await homeLink.inputValue()==='/games'&&await homeLink.getAttribute('readonly')!==null);
 const homeLabel=section.getByRole('textbox',{name:/^链接文字/}),originalHomeLabel=await homeLabel.inputValue();
 await page.getByRole('tab',{name:'展示墙',exact:true}).click();await choose(page,'Apple Music 唱片');
 await openImport(page);const input=page.getByRole('textbox',{name:'Apple Music 专辑或公开歌单链接',exact:true});await input.fill(playlistURL);
 const before=await allMusic();evidence.music_rows_before=before.length;
 const preview=await readPreview(page);
 check('real supplied playlist parses 300 tracks / 291 albums / 24 shown',preview.track_count===300&&preview.album_count===291&&preview.preview_count===24&&preview.items.length===24);
 check('real playlist snapshot explicitly partial',preview.partial===true&&await page.locator('.homepage-playlist-partial').innerText().then(t=>t.includes('未读取完整歌单')));
 check('three distinct count labels',await page.locator('.homepage-playlist-counts').innerText().then(t=>t.includes('本次读取曲目')&&t.includes('本次去重专辑')&&t.includes('当前预览专辑')));
 check('twenty-four preview covers with source',await page.locator('.homepage-playlist-albums li').count()===24&&await page.locator('.homepage-playlist-source').getAttribute('href')===playlistURL);
 evidence.real_preview={track_count:preview.track_count,album_count:preview.album_count,preview_count:preview.preview_count,partial:preview.partial};
 const previewTitle=await page.locator('.homepage-playlist-preview h3').innerText();
 const previewRoute='**/api/v1/admin/home/music-playlist/preview';
 await page.route(previewRoute,r=>r.fulfill({status:502,json:{error:{code:'UPSTREAM_FAILED',message:'V23 controlled upstream failure'}}}));
 await page.getByRole('button',{name:'重新读取最新预览',exact:true}).click();await page.getByText(/未能读取新歌单，原预览和当前条目已保留/).waitFor();
 check('502 reread preserves prior preview and exact input',await page.locator('.homepage-playlist-preview h3').innerText()===previewTitle&&await page.locator('.homepage-playlist-albums li').count()===24&&await input.inputValue()===playlistURL);
 await page.unroute(previewRoute);
 const itemEditor=page.locator('.homepage-item-editor'),itemTitle=itemEditor.getByRole('textbox',{name:/^标题/}),importButton=page.locator('.homepage-playlist-actions button.primary');
 await itemTitle.fill('V23 unsaved local album title');check('unsaved item blocks playlist import',await importButton.isDisabled());await itemTitle.fill('');
 await page.getByRole('tab',{name:'页面内容',exact:true}).click();await homeLabel.fill(originalHomeLabel+' V23 unsaved');await page.getByRole('tab',{name:'展示墙',exact:true}).click();check('unsaved home config blocks playlist import',await importButton.isDisabled());
 await page.getByRole('tab',{name:'页面内容',exact:true}).click();await homeLabel.fill(originalHomeLabel);await page.getByRole('tab',{name:'展示墙',exact:true}).click();check('reverting unsaved fields allows explicit import',await importButton.isEnabled());
 const importRoute='**/api/v1/admin/home/music-playlist/import';
 await page.route(importRoute,r=>r.fulfill({status:409,json:{error:{code:'VERSION_CONFLICT',message:'V23 controlled version conflict'}}}));
 await confirmImport(page);await page.getByText('歌单状态已变化，原预览和当前条目已保留。请刷新导入状态后再次确认。',{exact:true}).waitFor();
 check('CAS blocks blind retry without losing preview/input',await importButton.isDisabled()&&await input.inputValue()===playlistURL&&await page.locator('.homepage-playlist-preview h3').innerText()===previewTitle);
 await page.unroute(importRoute);await itemTitle.fill('V23 retained while refreshing status');await page.getByRole('button',{name:'刷新导入状态',exact:true}).click();await idle(page);
 await page.getByRole('button',{name:'刷新导入状态',exact:true}).waitFor({state:'hidden'});check('status refresh preserves unsaved item and original preview',await itemTitle.inputValue()==='V23 retained while refreshing status'&&await page.locator('.homepage-playlist-preview h3').innerText()===previewTitle);await itemTitle.fill('');
 let lose=true,committed=null;const attempts=[];
 await page.route(importRoute,async r=>{const req=r.request(),h=req.headers();attempts.push({key:h['idempotency-key'],version:h['if-match'],payload:req.postDataJSON()});if(lose){lose=false;try{const res=await r.fetch({timeout:120000});if(res.status()!==200)throw Error('Real import HTTP '+res.status());committed=await res.json();await r.abort('failed')}catch(e){errors.push('controlled lost-ack setup: '+e.message);await r.abort('failed')}}else await r.continue()});
 await confirmImport(page);await page.getByRole('button',{name:'重试保存本次专辑',exact:true}).waitFor({timeout:150000});
 check('first real import committed before lost acknowledgement',!!committed&&Number.isSafeInteger(committed.imported_count));
 check('lost acknowledgement retains preview/input and retry action',await input.inputValue()===playlistURL&&await page.locator('.homepage-playlist-preview h3').innerText()===previewTitle);
 const afterLost=await allMusic();await page.getByRole('button',{name:'重试保存本次专辑',exact:true}).click();await page.getByRole('button',{name:'本次已保存',exact:true}).waitFor({timeout:120000});
 const afterRetry=await allMusic();
 check('retry uses identical key/version/preview payload',attempts.length===2&&attempts[0].key===attempts[1].key&&attempts[0].version===attempts[1].version&&JSON.stringify(attempts[0].payload)===JSON.stringify(attempts[1].payload)&&attempts[0].payload.preview_id===preview.preview_id);
 check('replayed import adds no duplicate rows',afterLost.map(x=>x.id).sort().join()===afterRetry.map(x=>x.id).sort().join());
 check('import preserves all pre-existing rows',before.every(x=>afterRetry.some(y=>x.id===y.id)));
 check('new imported rows accurately visible',afterRetry.filter(x=>!before.some(y=>y.id===x.id)).every(x=>x.source==='music_playlist'&&x.visible===true));
 check('import confirmation reports actual new/preserved counts',await page.locator('.homepage-manager>.homepage-feedback[role=status]').innerText().then(t=>t.includes('新增 '+committed.new_count+' 张')&&t.includes('保留 '+committed.preserved_count+' 张')));
 evidence.import={imported_count:committed.imported_count,new_count:committed.new_count,preserved_count:committed.preserved_count,rows_after:afterRetry.length,identical_retry:true};
 await page.unroute(importRoute);
 // Create and remove only this script's own hidden manual game.
 await choose(page,'Steam 游戏');await page.getByRole('button',{name:'新建条目',exact:true}).click();
 const art=page.locator('.homepage-game-art');await art.locator('summary').click();
 check('raw blank Hero/Logo mean automatic metadata',await art.getByRole('textbox',{name:/^Steam Hero 地址/}).inputValue()===''&&await art.getByRole('textbox',{name:/^Steam Logo 地址/}).inputValue()===''&&await art.innerText().then(t=>t.includes('留空自动')));
 check('provider minutes are output, never editable',await art.locator('output[aria-label="累计游玩时长"]').innerText()==='未知'&&await art.getByRole('spinbutton').count()===0&&await art.innerText().then(t=>t.includes('未知不按0计算')));
 const title='V23 isolated admin contract '+Date.now();await itemTitle.fill(title);await itemEditor.getByRole('textbox',{name:/^目标链接/}).fill('https://store.steampowered.com/app/620/');await itemEditor.locator('.homepage-item-advanced summary').click();await itemEditor.getByRole('textbox',{name:'平台编号',exact:true}).fill('620');await itemEditor.getByRole('textbox',{name:'平台图片地址',exact:true}).fill('https://cdn.akamai.steamstatic.com/steam/apps/620/header.jpg');
 const createWait=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/admin/home/items'&&r.request().method()==='POST');await itemEditor.getByRole('button',{name:'保存条目',exact:true}).click();const createdRes=await createWait,created=await createdRes.json();check('temporary hidden Steam item real create',createdRes.status()===201,{status:createdRes.status(),...(created.error?{error:created.error}:{})});tempID=created.id;await idle(page);check('temporary Steam item remains hidden',created.visible===false&&created.playtime_minutes===null);
 await itemTitle.fill(title+' updated');const updateWait=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/admin/home/items/'+tempID&&r.request().method()==='PUT');await itemEditor.getByRole('button',{name:'保存条目',exact:true}).click();const updateRes=await updateWait,put=updateRes.request().postDataJSON();check('Steam real PUT succeeds',updateRes.status()===200);await updateRes.json();await idle(page);
 check('Steam payload blank art resets automatic and omits provider minutes',put.hero_url===''&&put.logo_url===''&&!Object.hasOwn(put,'playtime_minutes'));
 await cleanup();check('temporary manual item deleted without touching existing library',tempID==='');
 await choose(page,'Apple Music 唱片');await openImport(page);await input.fill(playlistURL);
 // Replay the already-verified preview solely for geometry; no further upstream request.
 await page.route(previewRoute,r=>r.fulfill({json:preview}));await readPreview(page,'读取歌单预览',true);await page.unroute(previewRoute);
 for(const width of[320,390])for(const theme of['light','dark']){await page.setViewportSize({width,height:844});await page.evaluate(t=>{document.documentElement.dataset.theme=t;localStorage.setItem('painter-theme',t)},theme);check(width+' '+theme+' manager page has no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));check(width+' '+theme+' playlist actions stay inside viewport',await page.locator('.homepage-playlist-actions').evaluate(e=>{const r=e.getBoundingClientRect();return r.left>=-1&&r.right<=innerWidth+1}));}
 check('final UI still contains no native selects',await page.locator('select').count()===0);
 const author=await browser.newContext({storageState:'/tmp/painter-next-v16-author-state.json'});await author.route('**/api/v1/analytics/events',r=>r.fulfill({json:{accepted:0}}));const authorPage=await author.newPage();check('author admin route readonly denial HTTP403',(await authorPage.goto(base+'/studio/home')).status()===403);check('author homepage API HTTP403',(await author.request.get(base+'/api/v1/admin/home')).status()===403);await author.close();
 check('no runtime page errors',errors.length===0);save(true);console.log(JSON.stringify({passed:true,checks:checks.length,formal_business_writes:0,screenshots:0}));await browser.close();
})().catch(async e=>{try{await cleanup()}catch(cleanupError){errors.push('cleanup: '+cleanupError.message)}save(false,e);console.error(e.message);await browser?.close();process.exitCode=1});
