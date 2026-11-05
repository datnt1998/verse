'use strict';
const fs=require('node:fs/promises'),path=require('node:path'),crypto=require('node:crypto'),{createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const REPO='/Users/dat/Projects/verse',DOC=path.join(REPO,'apps/viderse/docs/design/v1'),ORIGIN='http://127.0.0.1:8041';
const INPUT=process.env.CANONICAL_INPUT||path.join(REPO,'plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/runs/002-completed-canonical-reference/reference-captures.json');
const OUT=process.env.CANONICAL_OUT||path.join(__dirname,'canonical-current-authenticated'),digest=b=>crypto.createHash('sha256').update(b).digest('hex');
(async()=>{let browser;const R={captures:[],pageErrors:[],externalBlocked:[],sourceHashes:{},fontProvenance:'Existing Be Vietnam Pro WOFF2 from native app pack, served locally by route; no font download',claims:'Props/bytes/fonts/captured-frame proof only; not pixel parity or every visible state acceptance'};try{
 await fs.mkdir(OUT,{recursive:true});const input=JSON.parse(await fs.readFile(INPUT,'utf8'));
 browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
 const context=await browser.newContext({serviceWorkers:'block',acceptDownloads:false});
 await context.route('**/*',async route=>{const q=route.request(),u=new URL(q.url());
  if(u.origin!==ORIGIN||q.method()!=='GET'){R.externalBlocked.push({host:u.host,method:q.method()});return route.abort('blockedbyclient');}
  if(u.pathname.startsWith('/__qa-fonts/')){const m=u.pathname.match(/^\/__qa-fonts\/be-vietnam-pro-(400|500|600|700)\.woff2$/);if(!m)return route.abort();return route.fulfill({status:200,contentType:'font/woff2',body:await fs.readFile(path.join(REPO,'apps/viderse/web/public/fonts','be-vietnam-pro-'+m[1]+'.woff2'))});}
  return route.continue();
 });
 for(const row of input.captures){
  const p=await context.newPage();p.on('pageerror',e=>R.pageErrors.push({id:row.id,message:e.message}));
  try{
   await p.setViewportSize(row.viewport);const file=row.board+'.dc.html',url=ORIGIN+'/artboards/'+file;
   const response=await p.goto(url);expect(response.status()).toBe(200);const served=digest(await response.body()),local=digest(await fs.readFile(path.join(DOC,'artboards',file)));expect(served).toBe(local);R.sourceHashes[file]=local;
   await p.waitForFunction(()=>typeof window.__dcSetProps==='function'&&typeof window.__dcRootName==='function'&&document.body.innerText.length>100);
   await p.addStyleTag({content:[400,500,600,700].map(w=>'@font-face{font-family:"Be Vietnam Pro";font-style:normal;font-weight:'+w+';font-display:block;src:url("'+ORIGIN+'/__qa-fonts/be-vietnam-pro-'+w+'.woff2") format("woff2");}').join('\n')});
   const proof=await p.evaluate(async props=>{
    const name=window.__dcRootName();window.__dcSetProps(name,props);
    await Promise.all([400,500,600,700].map(w=>document.fonts.load(w+' 14px "Be Vietnam Pro"')));await document.fonts.ready;
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
    const finite=document.getAnimations().filter(a=>a.playState==='running'&&Number.isFinite(a.effect?.getComputedTiming().endTime));await Promise.all(finite.map(a=>a.finished.catch(()=>{})));
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
    return {name,fonts:[...document.fonts].filter(f=>f.family.includes('Be Vietnam Pro')).map(f=>({family:f.family,weight:f.weight,status:f.status})),text:document.body.innerText,activeControls:[...document.querySelectorAll('[aria-pressed="true"],[aria-checked="true"],[aria-selected="true"]')].map(e=>({role:e.getAttribute('role'),text:e.innerText})),brokenImages:[...document.querySelectorAll('img')].filter(i=>i.complete&&i.naturalWidth===0).map(i=>i.getAttribute('src')),bounds:{width:innerWidth,height:innerHeight}};
   },row.props);
   expect(proof.fonts.filter(f=>f.status==='loaded').length).toBeGreaterThanOrEqual(4);
   const filename=row.id+'.png';await p.screenshot({path:path.join(OUT,filename)});
   R.captures.push({id:row.id,board:row.board,props:row.props,viewport:row.viewport,file:filename,sourceSha256:served,propsSubmitted:true,propsSetter:'__dcSetProps(__dcRootName(), props)',fontsLoaded:proof.fonts,brokenImages:proof.brokenImages,visibleText:proof.text,activeControls:proof.activeControls,visibleStateVerified:false,status:'CAPTURED_CURRENT_BYTES_PROPS_SUBMITTED'});
  }finally{await p.close();}
 }
 R.status='CAPTURED';
}catch(e){R.status='FAIL';R.error=e.message;process.exitCode=1;}finally{if(browser)await browser.close();R.browserClosed=true;await fs.mkdir(OUT,{recursive:true});await fs.writeFile(path.join(OUT,'results.json'),JSON.stringify(R,null,2));console.log(JSON.stringify({status:R.status,count:R.captures.length,error:R.error,pageErrors:R.pageErrors.length,externalBlocked:R.externalBlocked.length,results:path.join(OUT,'results.json')}));}})();
