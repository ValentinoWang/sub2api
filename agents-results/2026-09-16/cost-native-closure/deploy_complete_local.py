#!/usr/bin/env python3
"""Apply a machine-green committed local candidate; preserve the existing self-use source."""
import argparse
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.request
from urllib.parse import quote, urlencode

p=argparse.ArgumentParser();p.add_argument('--build',required=True,type=Path);p.add_argument('--ci',required=True,type=Path);p.add_argument('--repo',required=True,type=Path);p.add_argument('--output',required=True,type=Path);args=p.parse_args()
PROJECT=Path('/Users/vsiyo/Desktop/Opensource_Tool/Sub2api');STATE=Path.home()/'.local/share/sub2api-cost-local'
build=json.loads(args.build.read_text());ci=json.loads(args.ci.read_text())
if build.get('status')!='PASS' or ci.get('status')!='PASS' or build['commit']!=ci['commit']:raise SystemExit('Matching committed build and complete local CI PASS required')
args.output.mkdir(parents=True,exist_ok=True)
def docker(*command,input=None):
 r=subprocess.run(['docker',*command],input=input,text=True,capture_output=True,timeout=60)
 if r.returncode:raise RuntimeError('Docker operation failed: '+command[0])
 return r.stdout
def private(path,value):
 path.parent.mkdir(parents=True,exist_ok=True);fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600)
 with os.fdopen(fd,'w') as f:f.write(value)
 os.chmod(path,0o600)
def dsn(user,password,host,db,readonly=False):
 query={'sslmode':'disable','connect_timeout':'5'}
 if readonly:query['default_transaction_read_only']='on'
 return 'postgres://'+quote(user,safe='')+':'+quote(password,safe='')+'@'+host+':5432/'+quote(db,safe='')+'?'+urlencode(query)
current=json.loads(docker('inspect','sub2api'))[0]
if current['Image']!=build['base_image_id']:raise SystemExit('The local app changed after the candidate baseline; rebase before rollout')
for key in ['image','collector_image']:
 meta=json.loads(docker('image','inspect',build[key]))[0]
 if meta['Id']!=build[key+'_id'] or meta['Config']['Labels'].get('org.opencontainers.image.revision')!=build['commit']:raise SystemExit('Image provenance mismatch')
backup=STATE/('rollback-'+build['commit'][:12]);backup.mkdir(mode=0o700,exist_ok=False)
private(backup/'container.json',json.dumps(current))
private(backup/'deploy.env',(PROJECT/'deploy/.env').read_text())
config=json.loads((STATE/'database-private.json').read_text())
config.setdefault('traffic_password',secrets.token_urlsafe(32));private(STATE/'database-private.json',json.dumps(config))
source_env=dict(value.split('=',1) for value in current['Config']['Env'])
paths=subprocess.check_output(['git','ls-tree','--name-only',build['commit']+':backend/internal/costing/migrations'],cwd=args.repo,text=True).splitlines()
for name in sorted(paths):
 ddl=subprocess.check_output(['git','show',build['commit']+':backend/internal/costing/migrations/'+name],cwd=args.repo,text=True)
 docker('exec','-i','sub2api-cost-local-postgres','psql','-U','cost_owner','-d','cost_ledger','-v','ON_ERROR_STOP=1',input=ddl)
password="'"+config['traffic_password'].replace("'","''")+"'"
sql=f"""DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='cost_traffic') THEN CREATE ROLE cost_traffic LOGIN; END IF; END $$;
ALTER ROLE cost_traffic PASSWORD {password};
GRANT CONNECT ON DATABASE cost_ledger TO cost_traffic;
GRANT USAGE ON SCHEMA public TO cost_traffic;
GRANT SELECT,INSERT ON cost_center_traffic_samples TO cost_traffic;
GRANT SELECT,INSERT ON cost_center_account_observations,cost_center_source_snapshots,cost_center_traffic_samples TO cost_runtime;
"""
docker('exec','-i','sub2api-cost-local-postgres','psql','-U','cost_owner','-d','cost_ledger','-v','ON_ERROR_STOP=1',input=sql)
vnstat=STATE/'vnstat';vnstat.mkdir(mode=0o700,exist_ok=True)
cost_env={
 'SUB2API_IMAGE':build['image'],
 'SUB2API_COST_COLLECTOR_IMAGE':build['collector_image'],
 'SUB2API_COST_LEDGER_DSN':dsn('cost_runtime',config['runtime_password'],'sub2api-cost-local-postgres','cost_ledger'),
 'SUB2API_COST_SOURCE_DSN':dsn(source_env['DATABASE_USER'],source_env['DATABASE_PASSWORD'],source_env['DATABASE_HOST'],source_env['DATABASE_DBNAME'],True),
 'SUB2API_COST_TRAFFIC_DSN':dsn('cost_traffic',config['traffic_password'],'sub2api-cost-local-postgres','cost_ledger'),
 'SUB2API_COST_VNSTAT_DIR':str(vnstat),
}
private(PROJECT/'deploy/.env.cost','\n'.join(k+'='+v for k,v in cost_env.items())+'\n')
override=subprocess.check_output(['git','show',build['commit']+':deploy/docker-compose.cost.yml'],cwd=args.repo,text=True)
private(STATE/'compose.cost.yml',override)
command=['docker','compose','--env-file',str(PROJECT/'deploy/.env'),'--env-file',str(PROJECT/'deploy/.env.cost'),'-f',str(PROJECT/'deploy/docker-compose.local.yml'),'-f',str(STATE/'compose.cost.yml'),'up','-d','--no-deps','--no-build','--force-recreate','sub2api','cost-collector']
with (args.output/'compose.log').open('w') as log:
 result=subprocess.run(command,cwd=PROJECT/'deploy',stdout=log,stderr=subprocess.STDOUT,timeout=120)
if result.returncode:raise SystemExit('Local compose rollout failed; rollback metadata preserved privately')
healthy=False
for _ in range(45):
 try:
  with urllib.request.urlopen('http://127.0.0.1:8080/health',timeout=3) as response:
   healthy=response.status==200 and json.load(response).get('status')=='ok'
  state=json.loads(docker('inspect','sub2api'))[0]['State']
  if healthy and state.get('Health',{}).get('Status')=='healthy':break
 except Exception:healthy=False
 time.sleep(1)
if not healthy:raise SystemExit('Local app health not established; rollback metadata preserved privately')
key=docker('exec','-i','sub2api-postgres','sh','-c','exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At',input="SELECT value FROM settings WHERE key='admin_api_key';").strip()
def request(path,payload=None):
 headers={'x-api-key':key,'Content-Type':'application/json'}
 req=urllib.request.Request('http://127.0.0.1:8080/api/v1/admin/cost-center/'+path,data=None if payload is None else json.dumps(payload).encode(),headers=headers)
 with urllib.request.urlopen(req,timeout=30) as response:return json.load(response)['data']
catalog=request('catalog');ledger=request('ledger/health')
from datetime import datetime,timezone,timedelta
now=datetime.now(timezone.utc);start=now.replace(day=1,hour=0,minute=0,second=0,microsecond=0);end=now.replace(hour=0,minute=0,second=0,microsecond=0)+timedelta(days=1)
sync=request('source/sync',{'start':start.isoformat(),'end':end.isoformat(),'model':''})
record={'status':'PASS','commit':build['commit'],'image':build['image'],'image_id':build['image_id'],'catalog_plans':len(catalog['plans']),'ledger':ledger,'source_sync':{k:sync[k] for k in ['id','status','account_count','request_count','stored_request_count']},'source_latest_usage_at':sync['report']['source_latest_usage_at'],'production_changed':False,'rollback_metadata':str(backup)}
(args.output/'readback.json').write_text(json.dumps(record,indent=2)+'\n')
# Switch the development page only after full 8080 authenticated reads and source write/readback pass.
envpath=PROJECT/'frontend/.env.local'
if envpath.exists():private(envpath,'\n'.join(line for line in envpath.read_text().splitlines() if not line.startswith('VITE_COST_LOCAL_PROXY='))+'\n')
old=subprocess.run(['docker','inspect','sub2api-cost-local'],capture_output=True,text=True)
if old.returncode==0:docker('stop','sub2api-cost-local')
lines=(PROJECT/'deploy/.env').read_text().splitlines();lines=[line for line in lines if not line.startswith('SUB2API_IMAGE=')];lines.append('SUB2API_IMAGE='+build['image']);private(PROJECT/'deploy/.env','\n'.join(lines)+'\n')
print(json.dumps(record))
