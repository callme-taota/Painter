import json,subprocess,pathlib,os,uuid,urllib.request,urllib.error,selectors,time
binary=os.environ.get('PAINTER_BINARY','/tmp/painter-next-v15-cli');origin=os.environ.get('BASE_URL','http://192.168.2.209:18081');config='/tmp/painter-v15-admin.json'
from urllib.parse import urlparse
location=urlparse(origin)
if location.port!=18081 or location.username or location.password or location.path not in ('','/') or location.query or location.fragment:raise RuntimeError('Disposable candidate origin on port18081 required')
account_file=pathlib.Path(os.environ['ACCOUNT_FILE'])
account=next(x for x in json.loads(account_file.read_text())['accounts']if x['role']=='owner')
checks=[]
def check(name,condition):
 checks.append({'name':name,'passed':bool(condition)})
 if not condition:raise RuntimeError(name)
def cli(*args,input=None,ok=True):
 p=subprocess.run([binary,'admin',*args,'--config',config],input=input,text=True,capture_output=True,timeout=40)
 if ok and p.returncode:raise RuntimeError('CLI failed: '+str(args[:2])+' code='+str(p.returncode))
 if not ok:return p.returncode,p.stderr
 return json.loads(p.stdout)
def key():return 'v15-'+uuid.uuid4().hex
def public(id):
 try:
  with urllib.request.urlopen(origin+'/api/v1/posts/'+id,timeout=10)as r:return r.status,json.load(r)
 except urllib.error.HTTPError as e:return e.code,{}
try:
 session=cli('login','--url',origin,'--username',account['username'],'--password-stdin','--allow-http',input=account['password']+'\n')
 check('real owner login without database config',session['user']['role']=='owner')
 check('session saved owner-only',os.stat(config).st_mode&0o777==0o600)
 check('session secrets omitted from output','csrf_token'not in session and 'session'not in session)
 check('whoami owner',cli('whoami')['user']['role']=='owner')
 posts=cli('posts','list','--limit','1');check('site article pagination available',len(posts['items'])==1 and posts['next_cursor'])
 nextpage=cli('posts','list','--limit','1','--cursor',posts['next_cursor']);check('article cursor advances',nextpage['items'][0]['id']!=posts['items'][0]['id'])
 tags=cli('tags','list','--limit','1');check('tag pagination available',len(tags['items'])==1 and tags['next_cursor'])
 nexttags=cli('tags','list','--limit','1','--cursor',tags['next_cursor']);check('tag cursor advances',nexttags['items'][0]['id']!=tags['items'][0]['id'])
 tagkey=key();tag=cli('tags','create','--name','V15CLI验收 '+uuid.uuid4().hex[:6],'--idempotency-key',tagkey)
 tagid=tag['id'];check('tag creation through business API',bool(tagid))
 draft={'title':'V15CLI验收 · 文章链路','summary':'仅存在隔离数据库','markdown':'# CLI 实际发布验收\n\nV15CLI验收 正文。','tag_ids':[tagid]}
 f=pathlib.Path('/tmp/painter-v15-draft.json');f.write_text(json.dumps(draft));f.chmod(0o600)
 createkey=key();created=cli('posts','create','--file',str(f),'--idempotency-key',createkey);id=created['id']
 again=cli('posts','create','--file',str(f),'--idempotency-key',createkey);check('create replay creates one draft',again['id']==id)
 state=cli('posts','status','--id',id);check('new article is private draft',state['status']=='draft'and public(id)[0]==404)
 full=cli('posts','get','--id',id);check('full content and draft version returned',full['markdown']==draft['markdown']and full['draft_version']==1)
 publishkey=key();pubargs=('posts','publish','--id',id,'--version',str(full['version']),'--draft-version',str(full['draft_version']),'--idempotency-key',publishkey)
 published=cli(*pubargs);replay=cli(*pubargs);check('publication replay same revision',published['revision_id']==replay['revision_id'])
 check('published article public',public(id)[0]==200)
 fail,err=cli('posts','publish','--id',id,'--version','1','--draft-version','1','--idempotency-key',key(),ok=False);check('stale publish fails nonzero',fail!=0 and 'VERSION_CONFLICT'in err)
 current=cli('posts','get','--id',id);snapshot={k:current[k]for k in ['title','summary','markdown','tag_ids','media_ids','cover','date_display']};snapshot['title']='V15CLI验收 · 未发布工作副本';f.write_text(json.dumps(snapshot))
 saved=cli('posts','save','--id',id,'--version',str(current['version']),'--draft-version',str(current['draft_version']),'--file',str(f),'--idempotency-key',key());check('save retains public revision',public(id)[1]['title']==draft['title'])
 current=cli('posts','get','--id',id);cli('posts','publish','--id',id,'--version',str(current['version']),'--draft-version',str(current['draft_version']),'--idempotency-key',key());check('new publication carries saved title',public(id)[1]['title']==snapshot['title'])
 current=cli('posts','status','--id',id);cli('posts','withdraw','--id',id,'--version',str(current['version']),'--idempotency-key',key());check('withdraw removes public article',public(id)[0]==404 and cli('posts','status','--id',id)['status']=='draft')
 def mcp(write):
  p=subprocess.Popen([binary,'mcp','--config',config]+(['--allow-write']if write else []),stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,bufsize=1)
  def send(method,params=None,id=None):
   msg={'jsonrpc':'2.0','method':method};
   if params is not None:msg['params']=params
   if id is not None:msg['id']=id
   p.stdin.write(json.dumps(msg)+'\n');p.stdin.flush()
  def receive():
   sel=selectors.DefaultSelector();sel.register(p.stdout,selectors.EVENT_READ)
   if not sel.select(20):p.kill();raise RuntimeError('MCP response timeout')
   return json.loads(p.stdout.readline())
  send('initialize',{'protocolVersion':'2025-11-25','capabilities':{},'clientInfo':{'name':'V15 acceptance','version':'1'}},1);r=receive();check('MCP initialize '+str(write),r['result']['protocolVersion']=='2025-11-25')
  send('notifications/initialized');send('tools/list',{},2);names=[x['name']for x in receive()['result']['tools']];check('MCP tool scope '+str(write),len(names)==(9 if write else 4))
  send('tools/call',{'name':'painter_post_status','arguments':{'id':id}},3);r=receive();check('MCP actual article status '+str(write),not r['result']['isError']and json.loads(r['result']['content'][0]['text'])['status']=='draft')
  if write:
   send('tools/call',{'name':'painter_post_publish','arguments':{'id':id,'version':cli('posts','status','--id',id)['version'],'draft_version':current.get('draft_version',2),'idempotency_key':key()}},4);r=receive();check('MCP actual publish',not r['result']['isError']and public(id)[0]==200)
   state=cli('posts','status','--id',id);cli('posts','withdraw','--id',id,'--version',str(state['version']),'--idempotency-key',key())
  else:
   send('tools/call',{'name':'painter_post_publish','arguments':{}},4);check('MCP default rejects write tool',receive().get('error',{}).get('code')==-32602)
  p.stdin.close();p.wait(timeout=10);check('MCP clean stdio shutdown '+str(write),p.returncode==0 and p.stderr.read()=='')
 mcp(False);mcp(True)
 cli('logout');check('logout removes local session config',not pathlib.Path(config).exists())
 # A reader can log into the website, but the administrator CLI must refuse.
 reader=next(x for x in json.loads(account_file.read_text())['accounts']if x['role']=='reader')
 code,err=cli('login','--url',origin,'--username',reader['username'],'--password-stdin','--allow-http',input=reader['password']+'\n',ok=False)
 check('reader denied CLI and no credential file retained',code!=0 and not pathlib.Path(config).exists())
 report={'passed':True,'checks':checks,'origin':origin,'content_writes':'isolated candidate only','mcp_tools':{'readonly':4,'write_enabled':9}}
 path=pathlib.Path('tests/artifacts/v15/cli-proof.json');path.write_text(json.dumps(report,ensure_ascii=False,indent=2));print(json.dumps({'passed':True,'checks':len(checks)}))
except Exception as e:
 pathlib.Path('tests/artifacts/v15/cli-failure.json').write_text(json.dumps({'checks':checks,'error':str(e)},ensure_ascii=False,indent=2));print('Candidate CLI proof failed: '+str(e));raise SystemExit(1)
