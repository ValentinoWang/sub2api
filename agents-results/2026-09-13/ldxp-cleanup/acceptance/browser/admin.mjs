import { chromium, expect } from '/Users/vsiyo/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright/test.mjs';
import { writeFile } from 'node:fs/promises';
import assert from 'node:assert/strict';
const out='agents-results/2026-09-13/ldxp-cleanup/acceptance/browser';
const browser=await chromium.launch({headless:true,executablePath:'/Users/vsiyo/Library/Caches/ms-playwright/chromium-1234/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing'});
try {
 const context=await browser.newContext({locale:'zh-CN',viewport:{width:1440,height:1000}});
 const user={id:777,email:'fixture@example.invalid',username:'本地验证',role:'admin',status:'active',balance:0,concurrency:1};
 await context.addInitScript(user=>{localStorage.setItem('auth_token','isolated-fixture');localStorage.setItem('auth_user',JSON.stringify(user));localStorage.setItem('locale','zh');},user);
 const paths=[];
 await context.route('**/api/**',async route=>{
  const path=new URL(route.request().url()).pathname;
  if (!path.startsWith('/api/')) { await route.continue(); return; }
  paths.push(path);
  assert.equal(route.request().method(),'GET','read-only UI fixture');
  let data={};
  if(path.endsWith('/auth/me')) data=user;
  else if(path.endsWith('/settings/public')) data={site_name:'本地验证',custom_menu_items:[],payment_enabled:false,purchase_subscription_enabled:true};
  else if(path.endsWith('/admin/compliance')) data={required:false};
  else if(path.endsWith('/announcements')) data=[];
  else if(path.endsWith('/browser/status')) data={enabled:false,paused_reason:'',products:[5,10,20,50,100].map((amount,i)=>({goods_id:100+i,cny_amount:amount,usd_credit:amount,external_url:`https://wzyp.cn/item/fixture${i}`,target_stock:999,batch_size:20,enabled:true,current_stock:999,identity_verified:true})),devices:[],batches:[]};
  else if(path.includes('subscriptions')) data=[];
  await route.fulfill({contentType:'application/json',body:JSON.stringify({code:0,data})});
 });
 const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('http://127.0.0.1:4174/admin/tools/ldxp');
 await expect(page.getByRole('heading',{name:'联动小铺补货',exact:true})).toBeVisible({timeout:20000}).catch(async error=>{ console.log(JSON.stringify({url:page.url(),paths,errors,text:(await page.locator('body').innerText()).slice(0,2000)}));throw error; });
 await expect(page.locator('[data-testid="chrome-restock"]')).toHaveCount(1);
 await expect(page.getByText('目标库存不超过100张',{exact:false})).toHaveCount(0);
 for(const label of ['Merchant-Token','安装 / 修复工具包','预览与执行','默认 50000']) await expect(page.getByText(label,{exact:false})).toHaveCount(0);
 const layouts=[];
 for(const width of [1440,390]) {
  await page.setViewportSize({width,height:1000});
  await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)).toBe(false);
  const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
  await page.screenshot({path:`${out}/admin-${width}.png`,fullPage:true});
  if(overflow) console.log(JSON.stringify(await page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,elements:[...document.querySelectorAll('body *')].map(e=>({tag:e.tagName,classes:e.className,left:e.getBoundingClientRect().left,right:e.getBoundingClientRect().right})).filter(e=>e.right>innerWidth+1).slice(-25)}))));
  assert.equal(overflow,false);layouts.push({width,overflow});
 }
 assert.ok(paths.includes('/api/v1/admin/tools/ldxp/browser/status'));
 assert.equal(paths.some(p=>p.includes('/admin/tools/ldxp/')&&!p.endsWith('/browser/status')),false);
 assert.deepEqual(errors,[]);
 const result={result:'PASS',boundary:'real Vite page rendered with fictional read-only API responses in isolated Chromium; no live user data or writes',paths,layouts,errors};
 await writeFile(`${out}/admin-result.json`,JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result));
} finally {await browser.close();}
