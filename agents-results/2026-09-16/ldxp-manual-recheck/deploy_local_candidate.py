from pathlib import Path
import json,subprocess,os,time,urllib.request
root=Path('/Users/vsiyo/Desktop/Opensource_Tool/Sub2api')
out=root/'agents-results/2026-09-16/ldxp-manual-recheck/acceptance/local-runtime';out.mkdir(exist_ok=True)
summary=json.loads((out.parent/'native-build-4/summary.json').read_text())
assert summary['status']=='PASS'
meta=json.loads(subprocess.check_output(['docker','image','inspect',summary['image']],text=True))[0]
assert meta['Id']==summary['image_id'] and meta['Config']['Labels']['org.opencontainers.image.revision']==summary['commit']
old=json.loads(subprocess.check_output(['docker','inspect','sub2api'],text=True))[0]
record={'previous_image':old['Config']['Image'],'image':summary['image'],'image_id':summary['image_id'],'commit':summary['commit'],'production_changed':False}
env=os.environ.copy();env['SUB2API_IMAGE']=summary['image']
with (out/'compose-final.log').open('w') as f:
 p=subprocess.run(['docker','compose','-f','docker-compose.local.yml','up','-d','--no-deps','--force-recreate','sub2api'],cwd=root/'deploy',env=env,stdout=f,stderr=subprocess.STDOUT)
record['compose_exit']=p.returncode
assert p.returncode==0
healthy=False
for _ in range(30):
 try:
  with urllib.request.urlopen('http://127.0.0.1:8080/health',timeout=3) as r:
   if r.status==200: healthy=True;break
 except Exception: pass
 time.sleep(1)
record['healthy']=healthy
(out/'deployment-final.json').write_text(json.dumps(record,indent=2)+'\n')
assert healthy
path=root/'deploy/.env';lines=path.read_text().splitlines();found=False
for i,line in enumerate(lines):
 if line.startswith('SUB2API_IMAGE='):lines[i]='SUB2API_IMAGE='+summary['image'];found=True
if not found:lines.append('SUB2API_IMAGE='+summary['image'])
path.write_text('\n'.join(lines)+'\n')
print(json.dumps(record))
