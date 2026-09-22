#!/usr/bin/env python3
"""Build both local runtime images from a single explicit committed source snapshot."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time

parser=argparse.ArgumentParser()
parser.add_argument('--repo',required=True,type=Path)
parser.add_argument('--commit',required=True)
parser.add_argument('--base-image',required=True)
parser.add_argument('--tools-dir',required=True,type=Path)
parser.add_argument('--output',required=True,type=Path)
args=parser.parse_args()
args.output.mkdir(parents=True,exist_ok=True)
if (args.output/'summary.json').exists():raise SystemExit('Use a new output directory')
env=os.environ.copy();env['PATH']=str(args.tools_dir)+os.pathsep+env['PATH']
env.update(GOMAXPROCS='2',GOGC='30',GOMEMLIMIT='2GiB',NODE_OPTIONS='--max-old-space-size=3072')
commit=subprocess.check_output(['git','rev-parse',args.commit+'^{commit}'],cwd=args.repo,text=True).strip()
base=json.loads(subprocess.check_output(['docker','image','inspect',args.base_image],text=True))[0]
minimum=shutil.disk_usage(args.repo).free
stages=[]
summary={'commit':commit,'base_image':args.base_image,'base_image_id':base['Id'],'status':'RUNNING','required_free_gib':3}
def run(stage,command,cwd,extra=None,output=None):
 global minimum
 minimum=min(minimum,shutil.disk_usage(args.repo).free)
 if minimum<4*1024**3:raise RuntimeError('Insufficient disk reserve before '+stage)
 started=time.monotonic()
 with (args.output/(stage+'.log')).open('wb') as log:
  proc=subprocess.Popen(command,cwd=cwd,env=env|(extra or {}),stdout=output or log,stderr=log,start_new_session=True)
  try:
   while proc.poll() is None:
    free=shutil.disk_usage(args.repo).free;minimum=min(minimum,free)
    if free<4*1024**3:
     os.killpg(proc.pid,signal.SIGTERM);proc.wait(timeout=20);raise RuntimeError('Disk reserve stopped '+stage)
    time.sleep(1)
  finally:
   if proc.poll() is None:os.killpg(proc.pid,signal.SIGTERM);proc.wait(timeout=20)
 stages.append({'stage':stage,'exit_code':proc.returncode,'seconds':round(time.monotonic()-started,2),'minimum_free_gib':round(minimum/1024**3,3)})
 (args.output/'progress.json').write_text(json.dumps(stages,indent=2)+'\n')
 print(json.dumps(stages[-1]),flush=True)
 if proc.returncode:raise RuntimeError('Failed stage '+stage)
try:
 with tempfile.TemporaryDirectory(prefix='sub2api-cost-build-') as temporary:
  temp=Path(temporary);archive=temp/'source.tar';source=temp/'source';source.mkdir()
  with archive.open('wb') as output:run('archive',['git','archive',commit],args.repo,output=output)
  run('extract',['tar','-xf',str(archive),'-C',str(source)],args.repo);archive.unlink()
  version=(source/'backend/cmd/server/VERSION').read_text().strip();date=subprocess.check_output(['git','show','-s','--format=%cI',commit],cwd=args.repo,text=True).strip()
  run('dependencies',['pnpm','--dir','frontend','install','--frozen-lockfile','--prefer-offline'],source)
  run('frontend',['pnpm','--dir','frontend','run','build'],source)
  shutil.rmtree(source/'frontend/node_modules')
  main=temp/'main-image';main.mkdir();collector=temp/'collector-image';collector.mkdir()
  flags=f'-s -w -X main.Version={version} -X main.Commit={commit} -X main.Date={date} -X main.BuildType=release'
  run('go',['go','build','-p','1','-trimpath','-tags','embed','-ldflags='+flags,'-o',str(main/'sub2api'),'./cmd/server'],source/'backend',{'CGO_ENABLED':'0','GOOS':'linux','GOARCH':base['Architecture']})
  run('collector-go',['go','build','-p','1','-trimpath','-o',str(collector/'cost-collect'),'./cmd/cost-collect'],source/'backend',{'CGO_ENABLED':'0','GOOS':'linux','GOARCH':'arm64'})
  shutil.copytree(source/'backend/resources',main/'resources')
  labels=f'LABEL org.opencontainers.image.source="https://github.com/ValentinoWang/sub2api"\nLABEL org.opencontainers.image.revision="{commit}"\nLABEL org.opencontainers.image.version="{version}"\nLABEL org.opencontainers.image.created="{date}"\n'
  (main/'Dockerfile').write_text(f'FROM {args.base_image}\n'+labels+'COPY --chown=sub2api:sub2api sub2api /app/sub2api\nCOPY --chown=sub2api:sub2api resources /app/resources\n')
  for name in ['cost-collector-entrypoint.sh','cost-vnstat.conf']:shutil.copy2(source/'deploy'/name,collector/name)
  vnstat='vergoh/vnstat@sha256:65e3d940c7d7292cdcd5a1ecf803c939dbd14efa38cf7117073b5d73d418c1bc'
  (collector/'Dockerfile').write_text(f'FROM {vnstat}\n'+labels+'COPY cost-collect /opt/cost-collect\nCOPY cost-collector-entrypoint.sh /opt/cost-collector-entrypoint.sh\nCOPY cost-vnstat.conf /etc/vnstat.conf\nENTRYPOINT ["/bin/sh","/opt/cost-collector-entrypoint.sh"]\n')
  image=f'sub2api-local:{version}-{commit[:12]}';sampler=f'sub2api-cost-collector:{version}-{commit[:12]}'
  run('image',['docker','build','--pull=false','--network=none','--platform=linux/'+base['Architecture'],'-t',image,'.'],main)
  run('collector-image',['docker','build','--pull=false','--network=none','--platform=linux/arm64','-t',sampler,'.'],collector)
  summary.update(status='PASS',version=version,image=image,image_id=json.loads(subprocess.check_output(['docker','image','inspect',image],text=True))[0]['Id'],collector_image=sampler,collector_image_id=json.loads(subprocess.check_output(['docker','image','inspect',sampler],text=True))[0]['Id'],binary_sha256=hashlib.sha256((main/'sub2api').read_bytes()).hexdigest(),collector_sha256=hashlib.sha256((collector/'cost-collect').read_bytes()).hexdigest())
except Exception as exc:
 summary.update(status='FAIL',error=str(exc))
finally:
 summary.update(stages=stages,minimum_free_gib=round(minimum/1024**3,3))
 (args.output/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
if summary['status']!='PASS':raise SystemExit(1)
