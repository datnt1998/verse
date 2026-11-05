'use strict';
const {createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs=require('node:fs/promises'),path=require('node:path'),crypto=require('node:crypto');
const ROOT=__dirname,OUT=path.join(ROOT,'native-template-save-readback'),ORIGIN='http://127.0.0.1:8043';
const hash=x=>crypto.createHash('sha256').update(JSON.stringify(x)).digest('hex');
const R={proof:'NATIVE_UI_PATCH_GET_RELOAD_RESTORE',cases:[],patches:[],blocked:[],pageErrors:[],restored:false};
let browser,context,page,original,target;
async function get(url){return page.evaluate(async url=>{const r=await fetch(url);if(!r.ok)throw new Error('Native GET failed '+r.status);return r.json();},url);}
(async()=>{try{
  await fs.mkdir(OUT,{recursive:true});
  browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  context=await browser.newContext({viewport:{width:1440,height:900},acceptDownloads:false,serviceWorkers:'block'});
  const cookies=JSON.parse(await fs.readFile('/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json','utf8'));
  await context.addCookies(cookies.map(c=>({...c,domain:'127.0.0.1',secure:false})));
  await context.route('**/*',async route=>{
    const q=route.request(),u=new URL(q.url());
    if(u.origin!==ORIGIN){R.blocked.push({kind:'external'});return route.abort('blockedbyclient');}
    if(q.method()!=='GET'){
      if(q.method()==='PATCH'&&target&&u.pathname===target){const body=q.postDataJSON();expect(typeof body.revision).toBe('number');expect(body.recipe).toBeTruthy();R.patches.push({method:'PATCH',path:u.pathname,revision:body.revision,recipeHash:hash(body.recipe)});return route.continue();}
      R.blocked.push({method:q.method(),path:u.pathname});return route.abort('blockedbyclient');
    }
    if(/\/(?:download|export|discover|discovery|parse|follow)(?:\/|$)/.test(u.pathname)||q.resourceType()==='media')return route.abort('blockedbyclient');
    return route.continue();
  });
  page=await context.newPage();page.on('pageerror',e=>R.pageErrors.push(e.message));page.on('download',d=>void d.cancel());
  await page.goto(ORIGIN+'/#/library?tab=templates');
  const list=await get('/api/templates');expect(list.items.length).toBeGreaterThan(0);
  target='/api/templates/'+encodeURIComponent(list.items[0].id);original=await get(target);R.templateId=original.id;R.originalRecipeHash=hash(original.recipe);
  for(const viewport of [{width:1440,height:900},{width:390,height:844}]){
    await page.setViewportSize(viewport);await page.goto(ORIGIN+'/#/templates/'+encodeURIComponent(original.id)+'?section=output');
    const before=await get(target),wanted=!before.recipe.output.compressExport;
    const response=page.waitForResponse(r=>r.url()===ORIGIN+target&&r.request().method()==='PATCH');
    await page.getByRole('radiogroup',{name:'Nén video sau khi xuất',exact:true}).getByRole('radio',{includeHidden:true}).nth(wanted?0:1).click();
    expect((await response).status()).toBe(200);
    const saved=await get(target);expect(saved.recipe.output.compressExport).toBe(wanted);expect(saved.revision).toBeGreaterThan(before.revision);
    await page.reload();await expect(page.getByRole('radiogroup',{name:'Nén video sau khi xuất',exact:true}).getByRole('radio',{includeHidden:true}).nth(wanted?0:1)).toBeChecked();
    await page.evaluate(()=>document.fonts.ready);await page.screenshot({path:path.join(OUT,'native-'+viewport.width+'-output-readback.png')});
    R.cases.push({viewport,status:'PASS',savedRevision:saved.revision,value:wanted,reloaded:true});
  }
  expect(R.pageErrors).toEqual([]);R.status='PASS';
}catch(e){R.status='FAIL';R.error=e.message;process.exitCode=1;}
finally{
  if(page&&original&&target){try{
    let current=await get(target);
    if(hash(current.recipe)!==hash(original.recipe)){
      const result=await page.evaluate(async({target,revision,recipe})=>{const r=await fetch(target,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({revision,recipe})});return {status:r.status};},{target,revision:current.revision,recipe:original.recipe});
      expect(result.status).toBe(200);current=await get(target);
    }
    expect(hash(current.recipe)).toBe(hash(original.recipe));R.restored=true;R.restoredRecipeHash=hash(current.recipe);R.finalRevision=current.revision;
  }catch(e){R.restoreError=e.message;R.status='FAIL';process.exitCode=1;}}
  if(browser)await browser.close();R.browserClosed=true;await fs.mkdir(OUT,{recursive:true});await fs.writeFile(path.join(OUT,'results.json'),JSON.stringify(R,null,2));console.log(JSON.stringify(R));
}
})();
