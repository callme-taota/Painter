// Labelled, disposable fixtures only. This script refuses every origin/database
// except the V44 candidate; neither production credentials nor providers are read.
const fs=require('fs'),path=require('path'),cp=require('child_process'),crypto=require('crypto');
const {request}=require('/tmp/painter-browser/node_modules/playwright');
const base='http://127.0.0.1:18087',db='painter_v44_mobile_20261113',privateDir='/tmp/painter-v44-private';
const metadataPath=path.join(privateDir,'mobile-fixtures.json');
const env=JSON.parse(fs.readFileSync(path.join(privateDir,'environment.json'),'utf8'));
if(env.PUBLIC_BASE_URL!==base||env.LISTEN_ADDR!=='127.0.0.1:18087'||env.RUN_BACKGROUND_WORKERS!=='false'||env.DATABASE_URL!=='host=/tmp/painter-v39-card-pg/socket port=65442 user=callumchan dbname='+db+' sslmode=disable')throw Error('V44 disposable database guard rejected configuration');
const pgEnv={...process.env,PGHOST:'/tmp/painter-v39-card-pg/socket',PGPORT:'65442',PGUSER:'callumchan',PGDATABASE:db,PGSSLMODE:'disable'};
const psql=path.join(env.POSTGRES_BIN_DIR,'psql');
function sql(text){return cp.execFileSync(psql,['-XAt','-v','ON_ERROR_STOP=1'],{env:pgEnv,input:text,encoding:'utf8',stdio:['pipe','pipe','pipe']}).trim()}
function literal(value){return "'"+String(value).replaceAll("'","''")+"'"}
if(sql('SELECT current_database();')!==db)throw Error('V44 connection guard rejected database');
(async()=>{
 if(fs.existsSync(metadataPath)){console.log('V44 disposable mobile fixtures already present');return}
 const credentials=JSON.parse(fs.readFileSync(path.join(privateDir,'credentials.json'),'utf8'));
 const ctx=await request.newContext({baseURL:base});
 const options=await ctx.get('/api/v1/auth/options').then(r=>r.json());
 const login=await ctx.post('/api/v1/auth/login',{headers:{Origin:base,'X-CSRF-Token':options.csrf_token},data:credentials});
 if(login.status()!==200)throw Error('Disposable owner login failed: '+login.status());
 const session=await ctx.get('/api/v1/session').then(r=>r.json());if(session.user?.role!=='owner')throw Error('Disposable owner required');
 async function api(method,url,data,version){const response=await ctx.fetch('/api/v1'+url,{method,headers:{Origin:base,'X-CSRF-Token':session.csrf_token,'Idempotency-Key':crypto.randomUUID(),...(version?{'If-Match':'"'+version+'"'}:{})},...(data===undefined?{}:{data})});if(!response.ok())throw Error(method+' '+url+' returned '+response.status());return response.status()===204?{}:response.json()}
 const upload=await ctx.post('/api/v1/media',{headers:{Origin:base,'X-CSRF-Token':session.csrf_token,'Idempotency-Key':crypto.randomUUID()},multipart:{file:{name:'V44-isolated-lake.jpg',mimeType:'image/jpeg',buffer:fs.readFileSync('tests/browser/fixtures/moments/lake-cc0.jpg')}}});
 if(upload.status()!==201)throw Error('Disposable media upload failed');const mediaID=(await upload.json()).media_id;
 const moment=await api('POST','/moments',{text:'V44 隔离验收动态：图片、短文字与触屏阅读。这里只是测试内容，不会发布到正式站点。',images:[{media_id:mediaID,alt:'V44 隔离验收湖景图片'}]});await api('POST','/studio/moments/'+moment.id+'/publish',{},moment.version);
 const phrase=await api('POST','/admin/phrases',{text:'把内容放在手边，让阅读按自己的节奏继续',kind:'sentence',source:'V44 隔离验收',published:true});
 const fence=card=>'```painter-card\n'+JSON.stringify(card)+'\n```';
 const paragraph='手机上的阅读需要清晰的结构与自然的纵向节奏。页面提供足够大的操作区域，正文保留安静的阅读宽度，附加信息在需要时展开。这里的文字仅用于隔离环境验收。';
 const markdown='# 阅读从内容开始\n\n'+paragraph+'\n\n## 站内内容引用\n\n'+fence({v:1,type:'moment',ref_id:moment.id,url:'/moments/'+moment.id})+'\n\n'+fence({v:1,type:'phrase',ref_id:phrase.id,url:'/phrases/'+phrase.id})+'\n\n## 段落与引用\n\n'+Array.from({length:5},()=>paragraph).join('\n\n')+'\n\n> 在一个明确的阅读节奏里，图片、表格与链接都不应该挤压正文。\n\n## 横向内容\n\n| 内容 | 用途 | 长字符串 |\n| --- | --- | --- |\n| 一个测试表格 | 确认表格只在内部滚动 | abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz |\n\n```go\nfunc mobileReadingKeepsTheDocumentReadableAcrossSmallPhonesAndLandscapeViews() {\n    println("V44 isolated rendering and horizontal-content verification")\n}\n```\n\n## 图片与结尾\n\n![V44 隔离验收湖景](/media/'+mediaID+')\n\n'+paragraph;
 const tags=(await api('GET','/admin/tags?limit=50')).items;
 const draft=await api('POST','/posts',{client_draft_id:crypto.randomUUID(),title:'V44 手机交互验收：让内容按手边的节奏展开',summary:'仅用于手机页面与交互的隔离验收，正式环境不会出现。',markdown,tag_ids:tags.slice(0,2).map(t=>t.id),media_ids:[mediaID],cover:{mode:'auto',template:'editorial',color:'ink'},date_display:{created:true,updated:true,published:true}});
 const published=await api('POST','/studio/posts/'+draft.id+'/publish',{expected_draft_version:draft.draft_version||1},draft.version);
 await api('POST','/posts/'+draft.id+'/comments',{body:'V44 隔离验收评论：这是一条用来检查手机头像、时间、正文以及触屏操作布局的真实测试评论。'});
 const home=await api('GET','/admin/home');const cfg=structuredClone(home.config);
 cfg.order=['intro','projects','activity','music','steam','photos','closing','articles'];cfg.use_site_intro=false;cfg.intro_name='V44 测试作者';cfg.intro_card_name='Callum Chan';cfg.intro_roles=['隔离环境验收'];cfg.sections.intro.description='分享技术笔记，以及日常';cfg.motion_enabled=true;cfg.warmth_enabled=false;
 const titles={projects:'开源项目',activity:'写下来的日子',music:'唱片墙',steam:'游戏墙',photos:'图片墙',articles:'看下最近我写的',closing:'看下最近我写的'};
 for(const key of cfg.order){cfg.sections[key].enabled=true;if(titles[key])cfg.sections[key].title=titles[key];if(['projects','steam','music','photos','articles'].includes(key))cfg.sections[key].limit=key==='music'?12:key==='photos'?2:5;}
 cfg.sections.closing.description='';cfg.sections.closing.link_label='阅读文章';cfg.sections.closing.link_url='#home-writing';
 await api('PUT','/admin/home',{config:cfg,steam:{steam_id:home.steam.steam_id,mode:home.steam.mode,auto_sync_enabled:false}},home.version);
 const publicPosts=(await api('GET','/posts?limit=6')).items;const covers=publicPosts.map(p=>p.cover_url).filter(Boolean);if(!covers.length)throw Error('Disposable cover fixtures unavailable');
 const now=Date.now(),rows=[];
 for(let i=0;i<12;i++)rows.push(`('music',${literal('V44 隔离唱片 '+(i+1))},'测试封面与收藏滑轨','V44 测试唱片封面','https://music.apple.com/',${literal(covers[i%covers.length])},'',${i},true,${now},${now},'', '',NULL,'', ARRAY[]::text[])`);
 for(let i=0;i<3;i++)rows.push(`('steam',${literal('V44 隔离游戏 '+(i+1))},'测试艺术图与游戏浏览','V44 测试游戏封面','https://store.steampowered.com/',${literal('/media/'+mediaID)},'',${i},true,${now},${now},${literal('/media/'+mediaID)},'',${(i+1)*180},'steam',ARRAY[]::text[])`);
 sql('BEGIN; INSERT INTO homepage_items(kind,title,subtitle,alt,url,image_url,provider_id,position,visible,created_at_ms,updated_at_ms,hero_url,logo_url,playtime_minutes,platform,platforms) VALUES '+rows.join(',')+';'+
 Array.from({length:3},(_,i)=>`INSERT INTO homepage_projects(source,title_override,description_override,url_override,language_override,metadata,position,visible,created_at_ms,updated_at_ms) VALUES('manual',${literal(['V44 Painter','V44 身份服务','V44 轻量工具'][i])},'隔离验收项目，检查长标题、说明和手机收藏滑轨，不代表真实开源项目。','https://github.com/','Go','{}',${i},true,${now},${now});`).join('')+' COMMIT;');
 for(let i=0;i<2;i++)await api('POST','/admin/home/items',{kind:'photos',title:'V44 隔离图片 '+(i+1),subtitle:'测试相册与灯箱交互',alt:'V44 隔离验收湖景',url:'',image_url:'',media_id:mediaID,provider_id:'',position:i,visible:true});
 const metadata={origin:base,isolated_database:db,article_id:draft.id,revision_id:published.revision_id,author_id:session.user.id,moment_id:moment.id,phrase_id:phrase.id,media_id:mediaID,collections:{projects:3,records:12,games:3,photos:2},production_writes:0,private_provider_requests:0,description:'Clearly labelled synthetic collections using local generated article covers and a repository CC0 lake photo; no external account or secret copied.'};
 fs.writeFileSync(metadataPath,JSON.stringify(metadata,null,2),{mode:0o600});await ctx.dispose();console.log(JSON.stringify({complete:true,article_id:draft.id,collections:metadata.collections,production_writes:0}));
})().catch(error=>{console.error('V44 fixture setup failed: '+error.message);process.exitCode=1});
