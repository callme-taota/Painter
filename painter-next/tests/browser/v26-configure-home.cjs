// Explicit owner-requested sequence change, separate from program promotion.
const fs=require('fs'),crypto=require('crypto'),{request}=require('/tmp/painter-browser/node_modules/playwright');
const formal=process.argv.includes('--formal'),base='http://192.168.2.209:'+(formal?'18080':'18081');
const file='tests/artifacts/v26/'+(formal?'formal-order-change':'candidate-order-change')+'.json';
(async()=>{
 if(fs.existsSync(file))throw Error('Sequence report exists; inspect rather than replay');
 const c=await request.newContext({storageState:formal?'/tmp/painter-next-v16-formal-state.json':'/tmp/painter-next-v16-owner-state.json'});
 const sr=await c.get(base+'/api/v1/session'),session=await sr.json();if(session.user?.role!=='owner')throw Error('Normal owner session required');
 const before=await c.get(base+'/api/v1/admin/home').then(r=>r.json());if(!before.config?.sections)throw Error('Home config not available');
 const config=structuredClone(before.config);config.order=['intro','projects','activity','photos','music','steam','closing','articles'];
 config.sections.closing.title='保持好奇。\n接下来，在文字里见。';config.sections.closing.link_label='继续阅读';config.sections.closing.link_url='#home-writing';
 const r=await c.put(base+'/api/v1/admin/home',{headers:{Origin:base,'X-CSRF-Token':session.csrf_token,'Idempotency-Key':crypto.randomUUID(),'If-Match':'"'+before.version+'"'},data:{config,steam:{steam_id:before.steam.steam_id,mode:before.steam.mode,auto_sync_enabled:before.steam.auto_sync_enabled}}});
 if(!r.ok())throw Error('Sequence update failed: '+r.status());
 const after=await c.get(base+'/api/v1/admin/home').then(r=>r.json());if(JSON.stringify(after.config)!==JSON.stringify(config))throw Error('Saved config differs');
 fs.writeFileSync(file,JSON.stringify({origin:base,updated_at:new Date().toISOString(),normal_owner_api:true,formal,before:{version:before.version,config:before.config},after:{version:after.version,config:after.config},steam_identity_or_sync_changed:JSON.stringify(before.steam)!==JSON.stringify(after.steam),scope:'Only homepage order and closing reading invitation; all visibility/limits/providers preserved'},null,2)+'\n');
 console.log(JSON.stringify({saved:true,formal,order:config.order,only_order_and_reading_invitation:true}));await c.dispose();
})().catch(e=>{console.error(e.message);process.exitCode=1});
