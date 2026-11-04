#!/usr/bin/env python3
"""Creates and stops its OWN temporary PG18 cluster; never accepts a business DSN.
Requires PostgreSQL18 server/client+pg_trgm binaries and psycopg2.
Usage: python3 verify_postgres.py --pg-bin /path/to/pg18/bin --result /tmp/report.json
"""
import argparse, hashlib, json, os, subprocess, tempfile, threading, time
from pathlib import Path
import psycopg2
parser=argparse.ArgumentParser();parser.add_argument('--pg-bin',required=True);parser.add_argument('--result',default='/tmp/painter-postgres-validation.json');args=parser.parse_args()
pgbin=Path(args.pg_bin).resolve()
repo=Path(__file__).resolve().parents[3]
schema=repo/'docs/next/schema.sql'
work=Path(tempfile.mkdtemp(prefix='painter-pg-check-')); data=work/'data';sock=work/'sock';sock.mkdir()
def command(name,*values):return subprocess.run([str(pgbin/name),*map(str,values)],check=True,capture_output=True,text=True)
command('initdb','-D',data,'--auth-local=trust','--auth-host=reject','--encoding=UTF8','--locale=C.UTF-8')
command('pg_ctl','-D',data,'-l',work/'server.log','-o',f"-k {sock} -p 65439 -c listen_addresses='' -c shared_buffers=16MB -c max_connections=20",'start')
started=True
import atexit
@atexit.register
def stop_cluster():
 if started:
  subprocess.run([str(pgbin/'pg_ctl'),'-D',str(data),'-m','fast','stop'],capture_output=True)
def conn(db='postgres'):
 x=psycopg2.connect(host=str(sock),port=65439,dbname=db,user=os.environ.get('USER','callumchan'));x.autocommit=True;return x
admin=conn();admin.cursor().execute("CREATE DATABASE checks TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'")
class SQL:
 def __init__(self,raw):self.raw=raw
 def execute(self,q,params=None):
  cur=self.raw.cursor();cur.execute(q.replace('?', '%s'),params);return cur
 def close(self):self.raw.close()
c=SQL(conn('checks')); c.execute(schema.read_text())
results=[]
def ok(name,condition):
 assert condition,name
 results.append(name)
def rejects(name,sql,args=()):
 try:c.execute(sql,args)
 except psycopg2.IntegrityError: results.append(name)
 else: raise AssertionError('expected rejection: '+name)
c.execute("INSERT INTO users(id,username,display_name,role,created_at_ms,updated_at_ms) VALUES(1,'owner','站主','owner',1,1)")
c.execute("INSERT INTO users(id,username,display_name,role,created_at_ms,updated_at_ms) VALUES(2,'author','旧作者','author',1,1)")
c.execute("INSERT INTO categories(id,name,created_at_ms,updated_at_ms) VALUES(1,'旧分类',1,1)")
c.execute("INSERT INTO tags(id,name,created_at_ms,updated_at_ms) VALUES(1,'旧标签',1,1)")
c.execute("INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms,ready_at_ms) VALUES('m1',1,'images/m1.png','x.png','image/png',1,?,'ready',1,1,1)",('a'*64,))
c.execute("INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,state,created_at_ms,updated_at_ms) VALUES('m2',1,'images/m2.png','x.png','image/png','staging',1,1)")
c.execute("INSERT INTO posts(id,author_user_id,client_draft_id,created_at_ms,updated_at_ms) VALUES(1,2,'draft-1',1,1)")
c.execute("INSERT INTO post_drafts(post_id,title,markdown,category_id,updated_by,updated_at_ms) VALUES(1,'published title','公开知识内容',1,2,1)")
c.execute("INSERT INTO post_draft_tags VALUES(1,1)")
c.execute("INSERT INTO post_draft_media_refs VALUES(1,'m1')")
ok('draft absent from public/search',c.execute('SELECT count(*) FROM public_posts').fetchone()[0]==0 and c.execute('SELECT count(*) FROM search_documents').fetchone()[0]==0)
rejects('staging media cannot be referenced',"INSERT INTO post_draft_media_refs VALUES(1,'m2')")
rejects('referenced media cannot enter deletion',"UPDATE media_assets SET state='deleting' WHERE id='m1'")
rejects('ready media bytes immutable',"UPDATE media_assets SET byte_size=2 WHERE id='m1'")
def revision(rid,no,dv,title,body):
 title,summary,body,category_id=c.execute('SELECT title,summary,markdown,category_id FROM post_drafts WHERE post_id=1').fetchone()
 category_name=c.execute('SELECT name FROM categories WHERE id=?',(category_id,)).fetchone()[0] if category_id is not None else None
 for tag_id,name in c.execute('SELECT dt.tag_id,t.name FROM post_draft_tags dt JOIN tags t ON t.id=dt.tag_id WHERE dt.post_id=1').fetchall():
  c.execute('INSERT INTO post_revision_tags VALUES(?,?,?)',(rid,tag_id,name))
 for (media_id,) in c.execute('SELECT media_id FROM post_draft_media_refs WHERE post_id=1').fetchall():
  c.execute('INSERT INTO post_revision_media_refs VALUES(?,?)',(rid,media_id))
 c.execute('''INSERT INTO post_revisions(id,post_id,revision_no,source_draft_version,reason,title,summary,markdown,sanitized_html,plain_text,renderer_version,content_hash,search_title,search_summary,search_body,category_id,category_name,created_by,created_at_ms)
 VALUES(?,1,?,?, 'publish',?,?,?,?,?,'render-v1',?,?,?,?,?,?,2,?)''',(rid,no,dv,title,summary,body,'<p>'+body+'</p>',body,'b'*64,title,summary,body,category_id,category_name,no))
c.execute('BEGIN')
revision('r1',1,1,'published title','公开知识内容')
c.execute('UPDATE posts SET version=version+1,first_published_at_ms=2 WHERE id=1 AND version=1')
c.execute("INSERT INTO post_publications VALUES(1,'r1',2,2)")
c.execute('COMMIT')
ok('publication advances search generation',c.execute('SELECT generation FROM search_state').fetchone()[0]==1)
ok('publication triggers normalized projection',c.execute('SELECT body,revision_id FROM search_documents').fetchone()==('公开知识内容','r1'))
ok('published substring searchable',len(c.execute("SELECT post_id FROM search_documents WHERE body LIKE ?",('%公开知%',)).fetchall())==1)
rejects('sealed revision cannot mutate',"UPDATE post_revisions SET markdown='leak' WHERE id='r1'")
rejects('sealed snapshot tags cannot mutate',"UPDATE post_revision_tags SET tag_name='new' WHERE revision_id='r1'")
rejects('sealed snapshot tags cannot append',"INSERT INTO post_revision_tags VALUES('r1',99,'new')")
rejects('public index cannot take arbitrary draft text',"UPDATE search_documents SET body='秘密内容' WHERE post_id=1")
c.execute("UPDATE categories SET name='新分类' WHERE id=1")
c.execute("UPDATE tags SET name='新标签' WHERE id=1")
ok('taxonomy snapshots survive renaming',c.execute("SELECT category_name FROM post_revisions WHERE id='r1'").fetchone()[0]=='旧分类' and c.execute("SELECT tag_name FROM post_revision_tags WHERE revision_id='r1'").fetchone()[0]=='旧标签')
c.execute('BEGIN')
c.execute('UPDATE posts SET version=version+1 WHERE id=1 AND version=2')
c.execute("UPDATE post_drafts SET draft_version=draft_version+1,title='new draft secret',markdown='未发布秘密',updated_at_ms=3 WHERE post_id=1")
c.execute('COMMIT')
ok('draft save does not advance search generation',c.execute('SELECT generation FROM search_state').fetchone()[0]==1)
ok('save published draft does not change live revision',c.execute('SELECT revision_id,title FROM public_posts').fetchone()==('r1','published title'))
ok('new draft absent from index',c.execute('SELECT count(*) FROM search_documents WHERE strpos(body,?)>0',('秘密',)).fetchone()[0]==0)
ok('stale aggregate CAS affects zero rows',c.execute('UPDATE posts SET version=version+1 WHERE id=1 AND version=2').rowcount==0)
rejects('aggregate version monotonic',"UPDATE posts SET updated_at_ms=999 WHERE id=1")
rejects('historical author cannot change',"UPDATE posts SET author_user_id=1,version=version+1 WHERE id=1")
rejects('last owner cannot be disabled',"UPDATE users SET disabled_at_ms=99 WHERE id=1")
c.execute('BEGIN')
c.execute("UPDATE users SET role='owner' WHERE id=2")
c.execute("UPDATE users SET role='reader' WHERE id=1")
c.execute('COMMIT')
ok('owner handover can be atomic without changing authors',c.execute("SELECT id FROM users WHERE role='owner'").fetchall()==[(2,)])
# Deferred FK+parent-last seals: partial children cannot be committed.
c.execute('BEGIN')
c.execute("INSERT INTO post_revision_tags VALUES('missing-revision',1,'x')")
try:c.execute('COMMIT')
except psycopg2.IntegrityError:c.execute('ROLLBACK');results.append('orphan deferred snapshot cannot commit')
else:raise AssertionError('orphan snapshot committed')
# Publication update and FTS are rolled back as one transaction.
c.execute('BEGIN')
revision('r2',2,2,'new published','下一版公开知识')
c.execute("UPDATE post_publications SET revision_id='r2',published_at_ms=4 WHERE post_id=1")
c.execute('ROLLBACK')
ok('search generation rolls back with publication',c.execute('SELECT generation FROM search_state').fetchone()[0]==1)
ok('publication/projection rollback restores old revision',c.execute('SELECT revision_id FROM search_documents').fetchone()[0]=='r1' and c.execute("SELECT count(*) FROM post_revisions WHERE id='r2'").fetchone()[0]==0)
c.execute('BEGIN')
revision('r2',2,2,'new published','下一版公开知识')
c.execute('UPDATE posts SET version=version+1 WHERE id=1 AND version=3')
c.execute("UPDATE post_publications SET revision_id='r2',published_at_ms=4 WHERE post_id=1")
c.execute('COMMIT')
ok('switch publication updates projection',c.execute('SELECT revision_id FROM search_documents').fetchone()[0]=='r2')
c.execute('BEGIN')
c.execute('UPDATE posts SET deleted_at_ms=5,version=version+1 WHERE id=1 AND version=4')
c.execute('COMMIT')
ok('soft delete removes live pointer/projection/GIN',c.execute('SELECT count(*) FROM post_publications').fetchone()[0]==0 and c.execute('SELECT count(*) FROM search_documents').fetchone()[0]==0 and c.execute("SELECT count(*) FROM search_documents WHERE strpos(body,'未发布')>0").fetchone()[0]==0)
ok('soft delete advances search generation',c.execute('SELECT generation FROM search_state').fetchone()[0]==3)
rejects('persistent client draft key prevents duplicate after idempotency retention',"INSERT INTO posts(id,author_user_id,client_draft_id,created_at_ms,updated_at_ms) VALUES(99,2,'draft-1',1,1)")
rejects('deleted post cannot publish',"INSERT INTO post_publications VALUES(1,'r2',2,5)")
c.execute('UPDATE posts SET deleted_at_ms=NULL,version=version+1 WHERE id=1 AND version=5')
ok('restore does not silently republish',c.execute('SELECT count(*) FROM post_publications').fetchone()[0]==0)
c.execute("INSERT INTO posts(id,author_user_id,client_draft_id,created_at_ms,updated_at_ms) VALUES(2,1,'draft-2',1,1)")
rejects('publication cannot refer to another posts revision',"INSERT INTO post_publications VALUES(2,'r2',1,6)")
c.execute('BEGIN')
c.execute('UPDATE posts SET version=version+1 WHERE id=1 AND version=6')
c.execute("INSERT INTO idempotency_keys VALUES(2,'save','mutation-1','hash',200,'{\"version\":7}',7,100)")
c.execute('COMMIT')
rejects('duplicate mutation key rejected',"INSERT INTO idempotency_keys VALUES(2,'save','mutation-1','different-hash',200,'{}',8,100)")
c.execute("INSERT INTO outbox(id,topic,aggregate_id,dedupe_key,payload_json,available_at_ms,created_at_ms) VALUES('event-1','content.published','1','publish:1:r2','{}',10,10)")
def consume_fact(fail=False):
 c.execute('BEGIN')
 added=c.execute("INSERT INTO outbox_consumptions VALUES('business-daily.v1','event-1',10) ON CONFLICT DO NOTHING").rowcount
 if added:
  c.execute("INSERT INTO business_daily(day_key,posts_published,updated_at_ms) VALUES('2026-10-20',1,10) ON CONFLICT(day_key) DO UPDATE SET posts_published=business_daily.posts_published+1,updated_at_ms=10")
 if fail:c.execute('ROLLBACK')
 else:c.execute('COMMIT')
consume_fact(True)
ok('business ledger and daily roll back together',c.execute('SELECT count(*) FROM business_daily').fetchone()[0]==0 and c.execute('SELECT count(*) FROM outbox_consumptions').fetchone()[0]==0)
consume_fact();consume_fact()
ok('outbox business consumer replay exactly-once local delta',c.execute('SELECT posts_published FROM business_daily').fetchone()[0]==1)
c.execute("INSERT INTO sessions(token_hash,management_id,user_id,csrf_secret_hash,created_at_ms,expires_at_ms,last_seen_at_ms) VALUES('token-1','management-000000001',2,'csrfhash',1,100,1)")
rejects('session management id independently unique',"INSERT INTO sessions(token_hash,management_id,user_id,csrf_secret_hash,created_at_ms,expires_at_ms,last_seen_at_ms) VALUES('token-2','management-000000001',2,'csrfhash',1,100,1)")
# Two real READ COMMITTED connections with a held lock, not mocked callbacks.
def race(name,first,second):
 a,b=SQL(conn('checks')),SQL(conn('checks'));a.execute('BEGIN');b.execute('BEGIN')
 first(a); outcome={};started_wait=threading.Event()
 def run():
  try:
   started_wait.set();outcome['value']=second(b);b.execute('COMMIT')
  except BaseException as e:
   outcome['error']=e;b.execute('ROLLBACK')
 t=threading.Thread(target=run);t.start();started_wait.wait(2);time.sleep(.08)
 blocked=t.is_alive();a.execute('COMMIT');t.join(5)
 assert not t.is_alive(),name+' deadlocked'
 a.close();b.close();return blocked,outcome
version=c.execute('SELECT version FROM posts WHERE id=1').fetchone()[0]
def first_cas(x):
 x.execute('SELECT id FROM posts WHERE id=1 FOR UPDATE')
 x.execute('UPDATE posts SET version=version+1 WHERE id=1 AND version=?',(version,))
def second_cas(x):
 x.execute('SELECT id FROM posts WHERE id=1 FOR UPDATE')
 return x.execute('UPDATE posts SET version=version+1 WHERE id=1 AND version=?',(version,)).rowcount
blocked,result=race('CAS',first_cas,second_cas)
ok('two-connection CAS serializes and stale writer loses',blocked and result.get('value')==0)
# Owner changes take exclusive policy barrier BEFORE user row locks.
c.execute("INSERT INTO users(id,username,display_name,role,created_at_ms,updated_at_ms) VALUES(3,'owner3','Owner 3','owner',1,1)")
def owner_demote(x,user):
 x.execute('SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE')
 x.execute("UPDATE users SET role='reader',version=version+1 WHERE id=?",(user,))
blocked,result=race('last owner',lambda x:owner_demote(x,2),lambda x:owner_demote(x,3))
ok('concurrent owner revocation cannot remove last owner',blocked and isinstance(result.get('error'),psycopg2.IntegrityError) and c.execute("SELECT count(*) FROM users WHERE role='owner' AND disabled_at_ms IS NULL").fetchone()[0]==1)
# A site feature toggle cannot overtake an authorized mutation holding the shared policy barrier.
blocked,result=race('policy barrier',lambda x:x.execute('SELECT id FROM policy_barrier WHERE id=1 FOR SHARE'),lambda x:x.execute('SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE').fetchone()[0])
ok('site/member barrier excludes policy changes during writes',blocked and result.get('value')==1)
c.execute("INSERT INTO post_drafts(post_id,title,markdown,updated_by,updated_at_ms) VALUES(2,'race draft','并发媒体',1,1)")
for mid in ['m3','m4']:
 c.execute("INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms,ready_at_ms) VALUES(?,1,?,'x.png','image/png',1,?,'ready',1,1,1)",(mid,'images/'+mid+'.png','c'*64))
def reference_first(x):
 x.execute('SELECT id FROM posts WHERE id=2 FOR UPDATE');x.execute("INSERT INTO post_draft_media_refs VALUES(2,'m3')")
def gc_after(x):
 x.execute("SELECT id FROM media_assets WHERE id='m3' FOR UPDATE")
 x.execute("UPDATE media_assets SET state='deleting' WHERE id='m3'")
blocked,result=race('media reference vs GC',reference_first,gc_after)
ok('media lock plus fresh reference check blocks racing GC',blocked and isinstance(result.get('error'),psycopg2.IntegrityError) and c.execute("SELECT state FROM media_assets WHERE id='m3'").fetchone()[0]=='ready')
c.execute("UPDATE media_assets SET state='deleting' WHERE id='m4'")
rejects('reference loses after GC claimed media',"INSERT INTO post_draft_media_refs VALUES(2,'m4')")
# Out-of-order commits: sequence is NOT progress. Claim only aggregated_at_ms IS NULL.
ms=1792429200000
insert_event="""INSERT INTO analytics_events(event_id,semantic_key,kind,page_instance_id,received_at_ms,day_key,visitor_day_key,traffic_class,normalized_path)
VALUES(?,?,'page_view.v1',?,?,'2026-10-20',?,'public','/') RETURNING seq"""
a=SQL(conn('checks'));a.execute('BEGIN')
seq_a=a.execute(insert_event,('event-a-0000000001','1'*64,'page-a-0000000001',ms,'a'*64)).fetchone()[0]
seq_b=c.execute(insert_event,('event-b-0000000001','2'*64,'page-b-0000000001',ms,'b'*64)).fetchone()[0]
def rollup(x,rollback=False):
 x.execute('BEGIN')
 rows=x.execute('SELECT seq,day_key,visitor_day_key FROM analytics_events WHERE aggregated_at_ms IS NULL ORDER BY received_at_ms,seq LIMIT 100 FOR UPDATE SKIP LOCKED').fetchall()
 for seq,day,visitor in rows:
  x.execute("INSERT INTO analytics_daily(day_key,resource_key,page_views,identified_page_views,updated_at_ms) VALUES(?,'site',1,1,?) ON CONFLICT(day_key,resource_key) DO UPDATE SET page_views=analytics_daily.page_views+1,identified_page_views=analytics_daily.identified_page_views+1",(day,ms))
  added=x.execute("INSERT INTO analytics_scope_visitors(day_key,resource_key,visitor_day_key,first_event_seq) VALUES(?,'site',?,?) ON CONFLICT DO NOTHING",(day,visitor,seq)).rowcount
  if added:x.execute("UPDATE analytics_daily SET unique_visitors=unique_visitors+1 WHERE day_key=? AND resource_key='site'",(day,))
  x.execute('UPDATE analytics_events SET aggregated_at_ms=? WHERE seq=?',(ms,seq))
  x.execute('UPDATE analytics_rollup_state SET last_seq=GREATEST(last_seq,?),updated_at_ms=? WHERE id=1',(seq,ms))
 x.execute('ROLLBACK' if rollback else 'COMMIT');return [r[0] for r in rows]
seen_b=rollup(c);a.execute('COMMIT');a.close();seen_a=rollup(c)
ok('late commit lower sequence is still aggregated',seq_a<seq_b and seen_b==[seq_b] and seen_a==[seq_a] and c.execute("SELECT page_views FROM analytics_daily WHERE resource_key='site'").fetchone()[0]==2)
seq_c=c.execute(insert_event,('event-c-0000000001','3'*64,'page-c-0000000001',ms,'b'*64)).fetchone()[0]
rollup(c,True)
ok('analytics deltas and pending marker roll back together',c.execute('SELECT aggregated_at_ms FROM analytics_events WHERE seq=?',(seq_c,)).fetchone()[0] is None and c.execute("SELECT page_views FROM analytics_daily WHERE resource_key='site'").fetchone()[0]==2)
rollup(c);rollup(c)
ok('analytics pending replay is once per committed raw row',c.execute("SELECT page_views,unique_visitors FROM analytics_daily WHERE resource_key='site'").fetchone()==(3,2))
# A pending row locked by worker A is skipped, then recoverable after A rolls back.
seq_d=c.execute(insert_event,('event-d-0000000001','4'*64,'page-d-0000000001',ms,'d'*64)).fetchone()[0]
a=SQL(conn('checks'));a.execute('BEGIN');a.execute('SELECT seq FROM analytics_events WHERE seq=? FOR UPDATE',(seq_d,))
ok('SKIP LOCKED avoids duplicate concurrent claim',rollup(c)==[])
a.execute('ROLLBACK');a.close();ok('rolled-back claim is recoverable',rollup(c)==[seq_d])
# Query planner evidence on the same normalized Chinese fields; forced scan only demonstrates eligibility.
c.execute('UPDATE posts SET version=version+1 WHERE id=1')
c.execute("INSERT INTO post_publications VALUES(1,'r2',2,12)")
ok('Chinese substring query against public projection',c.execute('SELECT count(*) FROM search_documents WHERE body LIKE ?',('%未发布%',)).fetchone()[0]==1)
c.execute('SET enable_seqscan=off')
plan='\n'.join(x[0] for x in c.execute('EXPLAIN SELECT post_id FROM search_documents WHERE body LIKE ?',('%未发布%',)).fetchall())
ok('Chinese pg_trgm GIN index is eligible', 'search_body_trgm' in plan)
c.execute('RESET enable_seqscan')
ok('short Chinese term explicit strpos works',c.execute('SELECT count(*) FROM search_documents WHERE strpos(body,?)>0',('秘密',)).fetchone()[0]==1)
ok('native JSONB / BYTEA / BOOLEAN / DATE types', c.execute("SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name='site_settings' AND column_name='registration_enabled'").fetchone()[0]=='boolean' and c.execute("SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name='post_revisions' AND column_name='toc_json'").fetchone()[0]=='jsonb')
ok('all PostgreSQL FK/CHECK constraints validated',c.execute("SELECT count(*) FROM pg_constraint WHERE connamespace='public'::regnamespace AND NOT convalidated").fetchone()[0]==0)
# Logical dump+restore in another isolated database, including data, functions, triggers and GIN indexes.
dump=work/'checks.dump'
command('pg_dump','-h',sock,'-p',65439,'-d','checks','-Fc','--no-owner','--no-acl','-f',dump)
admin.cursor().execute("CREATE DATABASE restored TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'")
command('pg_restore','-h',sock,'-p',65439,'-d','restored','--no-owner','--no-acl','--exit-on-error',dump)
r=SQL(conn('restored'))
tables=[x[0] for x in c.execute("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename").fetchall()]
counts=lambda x:{table:x.execute('SELECT count(*) FROM '+table).fetchone()[0] for table in tables}
ok('pg_dump/pg_restore all table row counts match',counts(c)==counts(r))
ok('restored publication and Chinese search usable',r.execute('SELECT revision_id FROM public_posts WHERE post_id=1').fetchone()[0]=='r2' and r.execute('SELECT count(*) FROM search_documents WHERE body LIKE ?',('%未发布%',)).fetchone()[0]==1)
try:r.execute("UPDATE post_revisions SET title='wrong' WHERE id='r2'")
except psycopg2.IntegrityError:results.append('restored immutable revision triggers enforced')
else:raise AssertionError('restored trigger missing')
report={'status':'passed','server_version':c.execute('SHOW server_version').fetchone()[0], 'encoding':c.execute('SHOW server_encoding').fetchone()[0],'lc_ctype':c.execute('SELECT datctype FROM pg_database WHERE datname=current_database()').fetchone()[0], 'schema_sha256':hashlib.sha256(schema.read_bytes()).hexdigest(),'checks_passed':len(results),'checks':results,'tables':len(tables),'trigram_plan':plan,'temporary_cluster':str(work),'scope':'Real PostgreSQL 18 sequential invariants, two-connection READ COMMITTED lock races, analytics commit reordering, and isolated logical dump/restore. No application HTTP, media fsync/power-loss, performance or deployment-capacity claim.'}
r.close();c.close();admin.close()
Path(args.result).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(report,ensure_ascii=False))
