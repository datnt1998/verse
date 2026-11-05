'use strict';
const {createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs=require('node:fs/promises');
const path=require('node:path');
const ROOT=__dirname, ORIGIN='http://127.0.0.1:8043';
const receipt={captures:[],blocked:[],errors:[],realWrites:0};
(async()=>{
  const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
  try {
    const cookies=JSON.parse(await fs.readFile('/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json','utf8')).map(c=>({...c,domain:'127.0.0.1',secure:false}));
    for(const width of [1440,390]){
      const context=await browser.newContext({viewport:{width,height:width===1440?900:844},serviceWorkers:'block'});
      try {
        await context.addCookies(cookies);
        await context.route('**/*',route=>{
          const request=route.request(),url=new URL(request.url());
          if(request.method()!=='GET'||url.origin!==ORIGIN){receipt.blocked.push({method:request.method(),path:url.pathname});return route.abort('blockedbyclient');}
          return route.continue();
        });
        const page=await context.newPage();page.on('pageerror',e=>receipt.errors.push(e.message));
        await page.goto(ORIGIN+'/#/library?tab=templates');
        const templates=await page.evaluate(async()=>{const response=await fetch('/api/templates');if(!response.ok)throw Error('Template inventory unavailable');return response.json();});
        const template=templates.items.find(t=>t.name==='Mẫu chuẩn · Tách nền');
        if(!template)throw Error('Original standard template missing');
        await page.goto(ORIGIN+'/#/templates/'+encodeURIComponent(template.id)+'?section=images');
        await expect(page.locator('.vt-place-art img')).toHaveCount(3);
        await page.evaluate(()=>document.fonts.ready);
        const images=[];
        for(const place of ['full','cutout','close']){
          const image=page.locator('[data-place="'+place+'"] img');
          await expect(image).toHaveAttribute('src','/illustrations/scene-composition-'+place+'.jpg');
          await expect.poll(()=>image.evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);
          images.push(await image.evaluate(el=>({src:el.getAttribute('src'),naturalWidth:el.naturalWidth,naturalHeight:el.naturalHeight,width:el.getBoundingClientRect().width,height:el.getBoundingClientRect().height})));
        }
        await page.locator('.vt-places').scrollIntoViewIfNeeded();
        const filename=width+'-composition-illustrations.png';
        await page.screenshot({path:path.join(ROOT,filename),animations:'disabled'});
        receipt.captures.push({width,filename,images,provenance:'Approved static design illustrations of composition options, not generated template artwork.'});
        expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width+1);
      } finally {await context.close();}
    }
    expect(receipt.blocked).toEqual([]);expect(receipt.errors).toEqual([]);receipt.status='PASS';
  }catch(e){receipt.status='FAIL';receipt.error=e.message;process.exitCode=1;}
  finally{await browser.close();await fs.writeFile(path.join(ROOT,'composition-illustration-results.json'),JSON.stringify(receipt,null,2));}
  console.log(JSON.stringify(receipt));
})();
