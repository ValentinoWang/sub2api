#!/usr/bin/env python3
import argparse,json,os,subprocess,tempfile,shutil,time,signal,hashlib
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--repo',type=Path,required=True);p.add_argument('--commit',required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--tools-dir',type=Path,required=True);a=p.parse_args()
commit=subprocess.check_output(['git','rev-parse',a.commit+'^{commit}'],cwd=a.repo,text=True).strip();tree=subprocess.check_output(['git','rev-parse',commit+'^{tree}'],cwd=a.repo,text=True).strip();current=subprocess.check_output(['git','rev-parse','HEAD'],cwd=a.repo,text=True).strip();currenttree=subprocess.check_output(['git','rev-parse','HEAD^{tree}'],cwd=a.repo,text=True).strip()
if current!=commit or currenttree!=tree:raise SystemExit('Source changed; frontend rereview invalidated')
a.output.mkdir(parents=True,exist_ok=True);env=os.environ.copy();env.update(PATH=str(a.tools_dir)+os.pathsep+env.get('PATH',''),GOGC='20',GOMEMLIMIT='1GiB',NODE_OPTIONS='--max-old-space-size=3072',CI='true',VITEST_MAX_THREADS='2',VITEST_MIN_THREADS='1')
minimum=shutil.disk_usage(a.repo).free;stages=[]
def run(name,cmd,cwd):
 global minimum
 started=time.monotonic()
 with (a.output/(name+'.log')).open('wb') as log:
  proc=subprocess.Popen(cmd,cwd=cwd,env=env,stdout=log,stderr=log,start_new_session=True)
  try:
   while proc.poll() is None:
    minimum=min(minimum,shutil.disk_usage(a.repo).free)
    if minimum<4*1024**3:os.killpg(proc.pid,signal.SIGTERM);proc.wait(timeout=20);raise RuntimeError('space guard stopped '+name)
    time.sleep(1)
  finally:
   if proc.poll() is None:os.killpg(proc.pid,signal.SIGTERM);proc.wait(timeout=20)
 stages.append({'stage':name,'status':'PASS' if proc.returncode==0 else 'FAIL','exit_code':proc.returncode,'seconds':round(time.monotonic()-started,2),'sha256':hashlib.sha256((a.output/(name+'.log')).read_bytes()).hexdigest()})
 if proc.returncode:raise RuntimeError(name)
try:
 with tempfile.TemporaryDirectory(prefix='sub2api-front-resume-') as t:
  root=Path(t);archive=root/'src.tar';subprocess.run(['git','archive','--output',str(archive),commit],cwd=a.repo,check=True);subprocess.run(['tar','-xf',str(archive),'-C',str(root)],check=True);archive.unlink()
  run('frozen-install',['pnpm','--dir','frontend','install','--frozen-lockfile'],root)
  run('frontend-typecheck',['pnpm','--dir','frontend','run','typecheck'],root)
  run('frontend-tests',['pnpm','--dir','frontend','exec','vitest','run','--maxWorkers=2','--minWorkers=1'],root)
  run('frontend-build',['pnpm','--dir','frontend','run','build'],root)
  shutil.rmtree(root/'frontend/node_modules')
  run('backend-security',['govulncheck','./...'],root/'backend')
  auditfile=a.output/'pnpm-audit.json';code=subprocess.run(['pnpm','--dir','frontend','audit','--prod','--audit-level=high','--json'],cwd=root,env=env,stdout=auditfile.open('wb')).returncode
  run('frontend-security',['python3','tools/check_pnpm_audit_exceptions.py','--audit',str(auditfile),'--exceptions','.github/audit-exceptions.yml','--audit-exit-code',str(code)],root)
 status='PASS'
except Exception as e: status='FAIL';(a.output/'error.txt').write_text(str(e))
summary={'commit':commit,'tree':tree,'mode':'resume_exact_unchanged_commit','status':status,'stages':stages,'minimum_free_gib':round(minimum/1024**3,3)}
(a.output/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
if status!='PASS':raise SystemExit(1)
