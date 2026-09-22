#!/usr/bin/env python3
"""Finish an interrupted local CI run only for the exact same clean commit/tree."""
import argparse,json,os,subprocess,tempfile,shutil,time,signal,hashlib
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--repo',type=Path,required=True);p.add_argument('--previous',type=Path,required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--tools-dir',type=Path,required=True);a=p.parse_args()
prior=json.loads((a.previous/'summary.json').read_text());commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=a.repo,text=True).strip();tree=subprocess.check_output(['git','rev-parse','HEAD^{tree}'],cwd=a.repo,text=True).strip()
if prior['commit']!=commit or prior['tree']!=tree:raise SystemExit('Source changed; a full new CI run is required')
subprocess.run(['git','diff','--exit-code','HEAD','--','.'],cwd=a.repo,check=True,stdout=subprocess.DEVNULL)
a.output.mkdir(parents=True,exist_ok=True)
if (a.output/'summary.json').exists():raise SystemExit('Use a new evidence directory')
env=os.environ.copy();env['PATH']=str(a.tools_dir)+os.pathsep+env['PATH'];env.update(CI='true',GOMAXPROCS='2',GOFLAGS='-p=1',GOGC='20',GOMEMLIMIT='768MiB',NODE_OPTIONS='--max-old-space-size=1536',HARNESS_ENGINEERING_HOME='/Users/vsiyo/Desktop/Opensource_Tool/Harness_Engineering')
stages=[];minimum=shutil.disk_usage(a.repo).free
reuse=['source-preflight','snapshot','toolchain','acceptance-layout','deploy-guards','version-guards','local-ci-guards','commerce-parity-guards','backend-unit','backend-integration']
for name in reuse:
 old=next(x for x in prior['stages'] if x['stage']==name)
 if old['status']!='PASS':raise SystemExit('Cannot reuse non-passing '+name)
 src=a.previous/old['log'];raw=src.read_bytes();(a.output/src.name).write_bytes(raw)
 stages.append(dict(old,reused_from=str(src),sha256=hashlib.sha256(raw).hexdigest()))
summary={'commit':commit,'tree':tree,'mode':'resume_exact_unchanged_commit','previous_run':str(a.previous),'status':'RUNNING','stages':stages}
def run(name,command,cwd):
 global minimum
 start=time.monotonic()
 with (a.output/(name+'.log')).open('wb') as log:
  proc=subprocess.Popen(command,cwd=cwd,env=env,stdout=log,stderr=log,start_new_session=True)
  try:
   while proc.poll() is None:
    minimum=min(minimum,shutil.disk_usage(a.repo).free)
    if minimum<4*1024**3:os.killpg(proc.pid,signal.SIGTERM);proc.wait(timeout=20);raise RuntimeError('Disk reserve stopped '+name)
    time.sleep(1)
  finally:
   if proc.poll() is None:os.killpg(proc.pid,signal.SIGTERM);proc.wait(timeout=20)
 stages.append({'stage':name,'status':'PASS' if proc.returncode==0 else 'FAIL','exit_code':proc.returncode,'seconds':round(time.monotonic()-start,2),'log':name+'.log','sha256':hashlib.sha256((a.output/(name+'.log')).read_bytes()).hexdigest()})
 print(name,proc.returncode,flush=True)
 if proc.returncode:raise RuntimeError('Stage failed: '+name)
try:
 with tempfile.TemporaryDirectory(prefix='sub2api-ci-resume-') as temp:
  root=Path(temp);archive=root/'source.tar'
  subprocess.run(['git','archive','--output',str(archive),commit],cwd=a.repo,check=True)
  subprocess.run(['tar','-xf',str(archive),'-C',str(root)],check=True);archive.unlink()
  run('backend-lint',['golangci-lint','run','--timeout=30m','--concurrency=1','--max-issues-per-linter=0','--max-same-issues=0','./...'],root/'backend')
  run('frozen-install',['pnpm','--dir','frontend','install','--frozen-lockfile'],root)
  run('browser-extension-tests',['node','--test',*map(str,sorted((root/'tools/ldxp-browser-extension/test').glob('*.test.js')))],root)
  run('frontend-lint',['pnpm','--dir','frontend','run','lint:check'],root)
  run('frontend-typecheck',['pnpm','--dir','frontend','run','typecheck'],root)
  run('frontend-tests',['pnpm','--dir','frontend','exec','vitest','run','--maxWorkers=2','--minWorkers=1'],root)
  run('frontend-build',['pnpm','--dir','frontend','run','build'],root)
  shutil.rmtree(root/'frontend/node_modules')
  run('backend-security',['govulncheck','./...'],root/'backend')
  with (a.output/'pnpm-audit.json').open('wb') as f:
   audit=subprocess.run(['pnpm','--dir','frontend','audit','--prod','--audit-level=high','--json'],cwd=root,env=env,stdout=f)
  run('frontend-security',['python3','tools/check_pnpm_audit_exceptions.py','--audit',str(a.output/'pnpm-audit.json'),'--exceptions','.github/audit-exceptions.yml','--audit-exit-code',str(audit.returncode)],root)
 summary['status']='PASS';summary['exit_code']=0
except Exception as e:
 summary.update(status='FAIL',exit_code=1,error=str(e))
finally:
 summary['minimum_free_gib']=round(minimum/1024**3,3);(a.output/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
if summary['status']!='PASS':raise SystemExit(1)
