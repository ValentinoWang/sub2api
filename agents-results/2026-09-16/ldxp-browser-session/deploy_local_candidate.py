from pathlib import Path
import json,subprocess,os,time,urllib.request,shutil,hashlib
root=Path('/Users/vsiyo/Desktop/Opensource_Tool/Sub2api')
out=root/'agents-results/2026-09-16/ldxp-browser-session/acceptance/local-runtime';out.mkdir(exist_ok=True)
summary=json.loads((out.parent/'native-build-1/summary.json').read_text())
assert summary['status']=='PASS'
image=json.loads(subprocess.check_output(['docker','image','inspect',summary['image']],text=True))[0]
assert image['Id']==summary['image_id'] and image['Config']['Labels']['org.opencontainers.image.revision']==summary['commit']
os.umask(0o077)
private=Path.home()/'.local/share/sub2api/ldxp-http-restock'
backup=private/'backups'/('browser-session-'+time.strftime('%Y%m%dT%H%M%S'));backup.mkdir(parents=True,exist_ok=False)
for name in ['state.json','device.json','dedup-capability.json','pause']:
 if (private/name).exists():shutil.copy2(private/name,backup/name)
shutil.copy2(root/'deploy/.env',backup/'deploy.env')
record={'backup_path':str(backup),'previous_image':json.loads(subprocess.check_output(['docker','inspect','sub2api'],text=True))[0]['Config']['Image'],'image':summary['image'],'image_id':summary['image_id'],'commit':summary['commit'],'production_changed':False}
env=os.environ.copy();env['SUB2API_IMAGE']=summary['image']
with (out/'compose.log').open('w') as f:
 p=subprocess.run(['docker','compose','-f','docker-compose.local.yml','up','-d','--no-deps','--force-recreate','sub2api'],cwd=root/'deploy',env=env,stdout=f,stderr=subprocess.STDOUT)
record['compose_exit']=p.returncode
assert p.returncode==0
healthy=False;deadline=time.monotonic()+90
while time.monotonic()<deadline:
 try:
  with urllib.request.urlopen('http://127.0.0.1:8080/health',timeout=3) as r:
   if r.status==200:healthy=True;break
 except (OSError,TimeoutError):pass
 time.sleep(1)
record['healthy']=healthy
(out/'deployment.json').write_text(json.dumps(record,indent=2)+'\n')
assert healthy
path=root/'deploy/.env';lines=path.read_text().splitlines();assert any(x.startswith('SUB2API_IMAGE=') for x in lines)
path.write_text('\n'.join('SUB2API_IMAGE='+summary['image'] if x.startswith('SUB2API_IMAGE=') else x for x in lines)+'\n')
print(json.dumps({k:v for k,v in record.items() if k!='backup_path'}))
