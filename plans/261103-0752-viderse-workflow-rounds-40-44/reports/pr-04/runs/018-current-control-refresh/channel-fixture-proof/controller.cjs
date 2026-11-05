'use strict';
// PREPARED ONLY. Parent executes this companion sequentially; no product source is changed.
// Every case is SYNTHETIC browser GET fixture proof, never backend persistence evidence.
const { createRequire } = require('node:module');
const { chromium, expect } = createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');
const zlib = require('node:zlib');
const ROOT = __dirname;
const ORIGIN = 'http://127.0.0.1:8043';
const COOKIE_FILE = '/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const IDS = ['78799ed0-57fa-4cff-ab00-509bdc65e5da','98ae68b2-813e-427a-8238-8eb0e86de231','c76cc725-0e9d-43fd-98d1-4d55412510e8'];
const VIEWPORTS = [{width:1440,height:900},{width:390,height:844}];
const MOTIONS = ['no-preference','reduce'];
const REGEN = ['Vẽ lại ảnh bìa kênh · 1 ảnh','Vẽ lại avatar · 1 ảnh','Đặt tên khác','Tạo lại cả bộ · 2 ảnh'];
const out = {
  status:'NOT_RUN', proofClass:'SYNTHETIC_BROWSER_GET_FIXTURES_ONLY', origin:ORIGIN,
  cases:[], requests:[], fixtureDefinitions:[], sampleInventory:[], unsupported:[],
  unproven:[
    'Native followed rivals, native source associations and native generated avatar/banner remain unproven; prior native inventory had none.',
    'Successful link/unlink/follow/create/default-template save, successful brief persistence and reload after actual mutation are not exercised.',
    'Real paid generation/admission, provider activity/progress, old-to-new completion, failed generation and backend identity persistence are not exercised.',
    'Existing prompt samples are reused as clearly synthetic identity images, NOT native generated channel identity or newly created output.',
    'No download, external navigation, real mutation, production write or deployment is permitted.',
    'This companion does not rerun brief PATCH failure cases or template artwork acceptance. Existing native controller owns those; pending feedback must target span[aria-live="polite"], not duplicate tooltip text.',
    'Finite interaction windows only: request absence is not a claim about arbitrary future background behavior.'
  ]
};
let browser, cookies, data, selectedSample;
const clone = value => JSON.parse(JSON.stringify(value));
const near = (actual, expected, tolerance=1) => expect(Math.abs(actual-expected)).toBeLessThanOrEqual(tolerance);
const endpoint = id => '/api/owner-channels/'+encodeURIComponent(id);
const unsupported = reason => {const error=new Error(reason);error.unproven=true;throw error;};
const originalChannels = () => IDS.map(id=>data.channels.find(c=>c.id===id)).filter(Boolean);
function forbiddenDownload(url) {
  return [...url.searchParams.keys()].some(key=>/^(download|export|attachment)$/i.test(key)) || /\/(?:export|download|localdownload|externaldownload)(?:\/|$)/i.test(url.pathname);
}
async function settle(page) {
  await page.evaluate(async()=>{
    await document.fonts.ready;
    await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect?.getComputedTiming().endTime)).map(a=>a.finished.catch(()=>{})));
    await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
  });
}
async function screenshot(page, row, suffix, locator) {
  await settle(page);
  const name=`SYNTHETIC-${row.viewport.width}-${row.name}-${suffix}.png`;
  await (locator||page).screenshot({path:path.join(ROOT,name),animations:'disabled'});
  row.screenshots.push(name);
  return name;
}
async function guardedContext(viewport, motion, label, overrides=new Map()) {
  const context=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,reducedMotion:motion,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
  try {
    await context.addCookies(cookies);
    // APIRequestContext is deliberately NOT used: native inventory and image reads also use this route fence.
    await context.route('**/*',async route=>{
      const request=route.request(),url=new URL(request.url());
      const event={scope:label,method:request.method(),path:url.pathname,queryKeys:[...url.searchParams.keys()]};
      if(request.method()!=='GET') {out.requests.push({...event,action:'ABORT_NON_GET_BEFORE_NETWORK'});return route.abort('blockedbyclient');}
      if(url.origin!==ORIGIN) {out.requests.push({...event,action:'ABORT_EXTERNAL_BEFORE_NETWORK',host:url.host});return route.abort('blockedbyclient');}
      if(forbiddenDownload(url)) {out.requests.push({...event,action:'ABORT_DOWNLOAD_BEFORE_NETWORK'});return route.abort('blockedbyclient');}
      if(overrides.has(url.pathname)) {out.requests.push({...event,action:'SYNTHETIC_GET_OVERRIDE'});return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(overrides.get(url.pathname))});}
      // Never leak a synthetic rival ID into the genuine backend, even if a future UI asks for another endpoint.
      if(url.pathname.includes('v38-synthetic-rival-')) {out.requests.push({...event,action:'ABORT_UNHANDLED_FIXTURE_GET'});return route.abort('blockedbyclient');}
      out.requests.push({...event,action:'NATIVE_GET_CONTINUED'});
      return route.continue();
    });
    context.on('page',page=>{
      page.on('download',download=>{download.cancel().catch(()=>{});out.requests.push({scope:label,action:'UNEXPECTED_DOWNLOAD_CANCELLED'});});
      page.on('pageerror',error=>out.requests.push({scope:label,action:'PAGEERROR',message:String(error)}));
      page.on('websocket',socket=>out.requests.push({scope:label,action:'UNEXPECTED_WEBSOCKET',url:socket.url()}));
    });
    // No WebSocket connection may bypass the HTTP GET fence.
    await context.routeWebSocket('**/*',socket=>{out.requests.push({scope:label,action:'WEBSOCKET_BLOCKED_BEFORE_NETWORK'});socket.close();});
    return context;
  } catch(error) {await context.close();throw error;}
}
async function withPage(row, overrides, fn) {
  const context=await guardedContext(row.viewport,row.motion,row.name,overrides);
  let page;
  try {
    page=await context.newPage();page.setDefaultTimeout(10000);
    await fn(page);
  } catch(error) {
    if(page) await screenshot(page,row,error.unproven?'unproven':'failure').catch(()=>{});
    throw error;
  } finally {await context.close();row.contextClosed=true;}
}
async function run(name,viewport,motion,fixture,fn) {
  const row={name,viewport,motion,proofClass:'SYNTHETIC_BROWSER_GET_FIXTURE',fixture,status:'running',screenshots:[]};
  out.cases.push(row);
  try {await fn(row);row.status='pass';}
  catch(error) {row.status=error.unproven?'unproven':'failed';row.reason=String(error.message||error);}
}
async function visit(page,id,tab) {
  await page.goto(ORIGIN+'/#/channels/'+id+'?tab='+tab,{waitUntil:'domcontentloaded'});
  await expect(page.locator(tab==='sources'?'#vch-add-rival':'#vch-brief')).toBeVisible();
  await settle(page);
  // Screenshot label is injected only into this private browser document, not product source/artboards.
  await page.evaluate(()=>{
    const label=document.createElement('div');label.id='v38-synthetic-proof-label';label.setAttribute('aria-hidden','true');
    label.textContent='SYNTHETIC GET FIXTURE — NOT BACKEND PERSISTENCE';
    Object.assign(label.style,{position:'fixed',left:'4px',bottom:'4px',zIndex:'2147483647',pointerEvents:'none',font:'10px monospace',color:'white',background:'#6b165c',padding:'3px 5px'});document.body.append(label);
  });
}
async function browserJSON(page,pathname) {
  if(!['/api/owner-channels','/api/rivals','/api/image-prompts'].includes(pathname)) throw new Error('Inventory path not allowlisted');
  const result=await page.evaluate(async pathname=>{
    const response=await fetch(pathname,{method:'GET',credentials:'same-origin',redirect:'error',cache:'no-store'});
    if(!response.ok) throw new Error('Native inventory GET '+pathname+' status '+response.status);
    return response.json();
  },pathname);
  return result;
}
async function inventory() {
  const context=await guardedContext(VIEWPORTS[0],'no-preference','native-read-only-inventory');
  try {
    const page=await context.newPage();page.setDefaultTimeout(10000);
    await page.goto(ORIGIN,{waitUntil:'domcontentloaded'});
    data={channels:(await browserJSON(page,'/api/owner-channels')).channels,rivals:(await browserJSON(page,'/api/rivals')).rivals,prompts:(await browserJSON(page,'/api/image-prompts')).items};
    out.nativeInventory={proofClass:'NATIVE_READ_ONLY_GET_INVENTORY_NOT_UI_ACCEPTANCE',channels:originalChannels().map(c=>({id:c.id,sourceIds:c.sources.map(s=>s.rivalId),identity:c.identity,hasAvatar:!!c.avatarUrl,hasBanner:!!c.bannerUrl})),rivalCount:data.rivals.length,promptCount:data.prompts.length};
    const candidates=new Map();
    for(const prompt of data.prompts) for(const url of prompt.sampleUrls||[]) {
      if(!candidates.has(url)) candidates.set(url,[]);
      candidates.get(url).push(prompt.id);
    }
    for(const [rawUrl,promptIds] of candidates) {
      const item={rawUrl,promptIds,proofClass:'EXISTING_NATIVE_PROMPT_SAMPLE_INVENTORY'};out.sampleInventory.push(item);
      let url;try {url=new URL(rawUrl,ORIGIN);} catch {item.status='SKIPPED_INVALID_URL';continue;}
      item.absoluteUrl=url.href;
      if(url.origin!==ORIGIN) {item.status='SKIPPED_EXTERNAL_NO_REQUEST';continue;}
      if(forbiddenDownload(url)) {item.status='SKIPPED_DOWNLOAD_NO_REQUEST';continue;}
      // Browser fetch traverses the same fence; redirects are refused, not silently followed off-origin.
      try {
        item.get=await page.evaluate(async url=>{
          const response=await fetch(url,{method:'GET',credentials:'same-origin',redirect:'error',cache:'no-store'});
          const info={status:response.status,contentType:response.headers.get('content-type'),contentDisposition:response.headers.get('content-disposition')};
          if(response.ok && /^image\//i.test(info.contentType||'') && !/attachment/i.test(info.contentDisposition||'')) info.bytes=(await response.arrayBuffer()).byteLength;
          return info;
        },url.href);
        if(item.get.status===200&&item.get.bytes>0) {
          item.dimensions=await page.evaluate(async src=>{
            const img=new Image();img.src=src;await img.decode();return {width:img.naturalWidth,height:img.naturalHeight};
          },url.href);
          item.status='EXISTING_SAME_ORIGIN_IMAGE_GET_AND_DECODE_OK';
          if(!selectedSample&&item.dimensions.width>0&&item.dimensions.height>0) selectedSample={url:rawUrl,absoluteUrl:url.href,promptIds,dimensions:item.dimensions};
        } else item.status='UNUSABLE_GET_RESPONSE';
      } catch(error) {item.status='GET_OR_IMAGE_DECODE_FAILED';item.reason=String(error.message||error);}
    }
    out.selectedSample=selectedSample||null;
  } finally {await context.close();out.inventoryContextClosed=true;}
}
function sourceSetup() {
  const originals=originalChannels();if(originals.length<2)return null;
  const target=clone(originals[0]),other=clone(originals[1]),stamp='2026-11-04T00:00:00Z';
  target.name='[SYNTHETIC GET] '+(target.name||'Channel here');other.name='[SYNTHETIC GET] Channel elsewhere';
  const rival=(id,title,subs,videos)=>({id,handle:'@'+id,title,avatarUrl:null,url:ORIGIN+'/#synthetic-fixture-not-a-real-rival',subscriberCount:subs,videoCount:videos,medianViews:25000,outliers:4,newOutliers:1,addedAt:stamp,scannedAt:stamp,scan:{state:'complete',discoveryRunning:false}});
  const here=rival('v38-synthetic-rival-here','[SYNTHETIC] Rival linked here',123000,72);
  const elsewhere=rival('v38-synthetic-rival-elsewhere','[SYNTHETIC] Rival linked elsewhere',456000,96);
  const source=r=>({rivalId:r.id,title:r.title,handle:r.handle,linkedAt:stamp,reference:true});
  target.sources=[source(here)];other.sources=[source(elsewhere)];
  const channels=data.channels.map(c=>c.id===target.id?target:c.id===other.id?other:c);
  const overrides=new Map([['/api/owner-channels',{channels}],[endpoint(target.id),target],[endpoint(other.id),other],['/api/rivals',{rivals:[here,elsewhere],totalVideos:168,newOutliers:2}]]);
  for(const r of [here,elsewhere]) {
    overrides.set('/api/rivals/'+r.id+'/scan',{channelId:r.id,historyCap:150,indexedVideos:r.videoCount,rescanHours:6,history:{state:'complete',jobId:null,scanned:r.videoCount,added:r.videoCount,pages:1,reachedEnd:true,startedAt:stamp,updatedAt:stamp,nextAttemptAt:null,errorCode:null,error:null,recovery:null},discovery:{checkedAt:stamp,caughtUp:true,running:false,nextAt:null}});
    overrides.set('/api/rivals/'+r.id+'/posting-slots',{channelId:r.id,zone:'vn',sample:28,total:28,pending:0,pausedUntil:null,perWeek:3.5,weeksTotal:8,offSlot:0,grid:Array.from({length:7},()=>Array(12).fill(0)),slots:[]});
  }
  return {target,other,here,elsewhere,overrides};
}
async function sourceProof(viewport,motion) {
  const fixture='Two entirely synthetic Rival/scan/posting records; one linked to original channel here, one to a different original channel, solely in GET responses. No backend rivals or associations created.';
  await run('sources-layout-picker-'+motion,viewport,motion,fixture,async row=>{
    const setup=sourceSetup();if(!setup)unsupported('Two original QA owner channels required for GET-only shared-source fixture');
    row.fixtureIds={target:setup.target.id,other:setup.other.id,here:setup.here.id,elsewhere:setup.elsewhere.id};
    await withPage(row,setup.overrides,async page=>{
      await visit(page,setup.target.id,'sources');
      if(viewport.width===1440) {
        const grid=page.locator('.vch-sources');row.columns=await grid.evaluate(el=>getComputedStyle(el).gridTemplateColumns.split(' ').map(parseFloat));
        expect(row.columns).toHaveLength(2);near(row.columns[0]/row.columns[1],1.3,.015);
        const rows=page.locator('.vch-rival');await expect(rows).toHaveCount(1);
        row.layout=await rows.first().evaluate(el=>{
          const top=el.children[0],actions=el.querySelector('.vch-rival__actions'),avatar=top.firstElementChild;
          const rect=node=>{const r=node.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom};};
          return {avatar:rect(avatar),top:rect(top),actions:rect(actions),metadata:el.querySelector('.vch-rival__meta')?.textContent,status:el.querySelector('.vch-chip')?.textContent,reference:el.querySelector('.vch-refbadge')?.textContent};
        });
        near(row.layout.avatar.width,34);near(row.layout.avatar.height,34);
        expect(row.layout.actions.y).toBeGreaterThanOrEqual(row.layout.top.bottom+8);near(row.layout.actions.width,row.layout.top.width);
        expect(row.layout.metadata).toContain('3,5');expect(row.layout.metadata).toContain('72');expect(row.layout.status.length).toBeGreaterThan(0);expect(row.layout.reference.length).toBeGreaterThan(0);
        await expect(rows.first().getByRole('button')).toHaveCount(4);
      } else {
        // Mobile source row intentionally uses 36px, NOT the desktop 34px layout.
        row.mobileLayoutNote='Mobile source layout is a separate 36px grid; 34px/two-row contract measured on desktop only.';
        // Locate the actual grid by its direct unlink button, avoiding broad ancestor matches.
        row.mobileLayout=await page.getByRole('button',{name:'Bỏ liên kết: '+setup.here.title,exact:true}).evaluate(button=>{
          const el=button.parentElement,r=el.getBoundingClientRect(),avatar=el.firstElementChild.getBoundingClientRect();
          return {width:r.width,grid:getComputedStyle(el).gridTemplateColumns,avatarWidth:avatar.width,avatarHeight:avatar.height,scrollWidth:el.scrollWidth,clientWidth:el.clientWidth};
        });
        near(row.mobileLayout.avatarWidth,36);near(row.mobileLayout.avatarHeight,36);expect(row.mobileLayout.scrollWidth).toBeLessThanOrEqual(row.mobileLayout.clientWidth+1);
      }
      await screenshot(page,row,'populated-source');
      const trigger=page.locator('.vch-linkslot').getByRole('button',{name:'Liên kết đối thủ',exact:true});
      const dialog=page.getByRole('dialog',{name:'Liên kết đối thủ với '+setup.target.name,exact:true});
      await trigger.click();await expect(dialog).toBeVisible();await expect(page.locator('[role="dialog"]:visible')).toHaveCount(1);
      await settle(page);
      row.openMotion=await page.evaluate(()=>({reduced:matchMedia('(prefers-reduced-motion: reduce)').matches,animations:document.getAnimations().map(a=>({name:a.animationName||a.transitionProperty||'web-animation',duration:a.effect?.getComputedTiming().duration,iterations:a.effect?.getComputedTiming().iterations}))}));
      expect(row.openMotion.reduced).toBe(motion==='reduce');
      const here=dialog.getByRole('checkbox').filter({has:page.locator('.vch-pickerrow__name',{hasText:setup.here.title})});
      const elsewhere=dialog.getByRole('checkbox').filter({has:page.locator('.vch-pickerrow__name',{hasText:setup.elsewhere.title})});
      await expect(here).toHaveCount(1);await expect(elsewhere).toHaveCount(1);
      await expect(here).toHaveAttribute('aria-disabled','true');await expect(here).toHaveAttribute('aria-checked','true');
      await here.click({force:true});await expect(here).toHaveAttribute('aria-checked','true');
      expect(await elsewhere.getAttribute('aria-disabled')).not.toBe('true');await expect(elsewhere.locator('.vch-pickerrow__sub')).toContainText('dùng chung được');
      await expect(elsewhere.locator('.vch-pickerrow__sub')).toContainText(setup.other.name);
      await elsewhere.click();await expect(elsewhere).toHaveAttribute('aria-checked','true');
      await expect(dialog.getByRole('button',{name:'Liên kết 1',exact:true})).toBeEnabled();
      await elsewhere.press('Space');await expect(elsewhere).toHaveAttribute('aria-checked','false');await elsewhere.press('Space');await expect(elsewhere).toHaveAttribute('aria-checked','true');
      await settle(page);
      row.dialogGeometry=await dialog.evaluate(el=>{const r=el.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom,scrollWidth:el.scrollWidth,clientWidth:el.clientWidth,focusInside:el.contains(document.activeElement),bodyOverflow:document.body.style.overflow};});
      expect(row.dialogGeometry.focusInside).toBe(true);expect(row.dialogGeometry.x).toBeGreaterThanOrEqual(-1);expect(row.dialogGeometry.x+row.dialogGeometry.width).toBeLessThanOrEqual(viewport.width+1);expect(row.dialogGeometry.bottom).toBeLessThanOrEqual(viewport.height+1);expect(row.dialogGeometry.scrollWidth).toBeLessThanOrEqual(row.dialogGeometry.clientWidth+1);
      if(viewport.width<768) {
        await expect(dialog).toHaveClass(/m-sheet/);await expect(dialog).toHaveAttribute('aria-modal','true');
        await expect(page.locator('.m-overlay:visible')).toHaveCount(1);await expect(page.locator('.m-scrim:visible')).toHaveCount(1);expect(row.dialogGeometry.bodyOverflow).toBe('hidden');
        row.sheetMotion=await dialog.evaluate(el=>{
          const panel=getComputedStyle(el),scrim=getComputedStyle(document.querySelector('.m-scrim'));
          return {panelAnimation:panel.animationName,panelDuration:panel.animationDuration,panelTransition:panel.transitionDuration,scrimAnimation:scrim.animationName,scrimDuration:scrim.animationDuration};
        });
        if(motion==='reduce') {
          expect(row.sheetMotion.panelAnimation).toBe('none');expect(row.sheetMotion.scrimAnimation).toBe('none');
          expect(row.sheetMotion.panelTransition.split(',').every(value=>parseFloat(value)<=.001)).toBe(true);
        } else {
          expect(row.sheetMotion.panelAnimation).toBe('m-sheet-in');expect(row.sheetMotion.scrimAnimation).toBe('m-fade');
        }
        const count=await dialog.locator('button:not(:disabled),a[href],input:not(:disabled),[tabindex="0"]').count();
        for(const key of ['Tab','Shift+Tab'])for(let n=0;n<count+2;n++){await page.keyboard.press(key);expect(await dialog.evaluate(el=>el.contains(document.activeElement))).toBe(true);}
      } else {
        await expect(dialog).toHaveClass(/vch-linkpop/);await expect(page.locator('.m-overlay:visible')).toHaveCount(0);
      }
      await screenshot(page,row,'picker-shared-eligible');
      await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);await expect(trigger).toBeFocused();await expect(trigger).toHaveAttribute('aria-expanded','false');
      if(viewport.width<768)await expect.poll(()=>page.evaluate(()=>document.body.style.overflow)).not.toBe('hidden');
      await trigger.click();await expect(dialog).toBeVisible();await settle(page);
      if(viewport.width<768) {
        const box=await dialog.boundingBox();if(!box||box.y<2)unsupported('Mobile Sheet leaves no exposed scrim coordinate');
        await page.locator('.m-scrim').click({position:{x:viewport.width/2,y:Math.max(1,box.y/2)}});
      } else {
        // Top-right 2px inset lies outside this measured content popover, with no actionable control.
        const box=await dialog.boundingBox();expect(box).not.toBeNull();
        expect(viewport.width-2>box.x+box.width || 2<box.y).toBe(true);
        await page.mouse.click(viewport.width-2,2);
      }
      await expect(dialog).toHaveCount(0);await expect(trigger).toBeFocused();
      if(viewport.width<768)await expect.poll(()=>page.evaluate(()=>document.body.style.overflow)).not.toBe('hidden');
      await settle(page);
      row.boundary='Only local checkbox selection, keyboard, Escape and outside/scrim dismissal. Never clicked Link N, follow, scan, make video or unlink.';
    });
  });
}
function identitySetup(state) {
  const original=originalChannels()[0];if(!original||!selectedSample)return null;
  const target=clone(original);target.name='[SYNTHETIC IDENTITY] Existing sample reuse';
  target.avatarUrl=selectedSample.url;target.bannerUrl=selectedSample.url;
  target.identity=state==='generating'?{state:'generating',part:'avatar'}:{state:'ready'};
  return {target,overrides:new Map([[endpoint(target.id),target],['/api/owner-channels',{channels:data.channels.map(c=>c.id===target.id?target:c)}]])};
}
async function renderedImage(img,url) {
  await expect(img).toHaveCount(1);await expect(img).toHaveAttribute('src',url);
  await expect(img).toBeVisible();
  await expect.poll(()=>img.evaluate(el=>el.complete&&el.naturalWidth>0&&el.naturalHeight>0)).toBe(true);
}
async function avatarProof(viewport,motion) {
  const fixture='Original channel GET identity overridden to avatar-generating with EXISTING native prompt sample as synthetic old avatar/banner; no job/progress/provider/completion fabricated.';
  await run('old-avatar-continuity-'+motion,viewport,motion,fixture,async row=>{
    const setup=identitySetup('generating');if(!setup)unsupported('Need original QA channel and actual same-origin prompt sample GET/decode');
    row.sample=selectedSample;
    await withPage(row,setup.overrides,async page=>{
      await visit(page,setup.target.id,'settings');
      const status=page.getByRole('status',{name:'Đang tạo avatar mới; ảnh đang dùng được giữ tới khi xong.',exact:true});await expect(status).toBeVisible();
      const wrapper=status.locator('..');await expect(wrapper).toHaveAttribute('aria-busy','true');
      const img=wrapper.getByRole('img',{name:setup.target.name,exact:true});await expect(img).toHaveCount(1);await expect(img).toBeVisible();
      const background=await img.evaluate(el=>getComputedStyle(el).backgroundImage);
      expect(background).toContain(selectedSample.url);
      await img.evaluate(async(el)=>{const url=getComputedStyle(el).backgroundImage.match(/^url\(["']?(.*?)["']?\)$/)?.[1];if(!url)throw Error('Avatar background missing');const image=new Image();image.src=url;await image.decode();if(!image.naturalWidth)throw Error('Avatar image failed to decode');});
      const imageHandle=await img.elementHandle();
      row.overlay=await status.evaluate(el=>{
        const cs=getComputedStyle(el),before=getComputedStyle(el,'::before'),after=getComputedStyle(el,'::after'),r=el.getBoundingClientRect(),parent=el.parentElement.getBoundingClientRect();
        return {width:r.width,height:r.height,x:r.x,y:r.y,parent:{x:parent.x,y:parent.y,width:parent.width,height:parent.height},radius:cs.borderRadius,opacity:cs.opacity,backgroundImage:cs.backgroundImage,animation:cs.animationName,beforeAnimation:before.animationName,afterAnimation:after.animationName,reduced:matchMedia('(prefers-reduced-motion: reduce)').matches};
      });
      near(row.overlay.width,row.overlay.height);near(row.overlay.parent.width,viewport.width<768?48:56);near(row.overlay.parent.height,row.overlay.parent.width);near(row.overlay.x-row.overlay.parent.x,2);near(row.overlay.y-row.overlay.parent.y,2);
      expect(row.overlay.radius).toBe('50%');expect(Number(row.overlay.opacity)).toBeGreaterThan(0);expect(Number(row.overlay.opacity)).toBeLessThan(1);expect(row.overlay.backgroundImage).toContain('linear-gradient');
      expect(row.overlay.reduced).toBe(motion==='reduce');
      const names=[row.overlay.animation,row.overlay.beforeAnimation,row.overlay.afterAnimation];
      if(motion==='reduce')expect(names.every(name=>name==='none')).toBe(true);else expect(names.some(name=>name!=='none')).toBe(true);
      for(const label of REGEN)await expect(page.getByRole('button',{name:label,exact:true})).toHaveAttribute('aria-disabled','true');
      await screenshot(page,row,'pending-existing-sample-visible');await settle(page);
      // Continuity is DOM/image continuity during the observed fixture-pending interval, not a backend transition.
      expect(await imageHandle.evaluate(el=>el.isConnected)).toBe(true);
      expect(await img.evaluate((el,old)=>el===old,imageHandle)).toBe(true);expect(await img.evaluate(el=>getComputedStyle(el).backgroundImage)).toBe(background);
      await imageHandle.dispose();
      row.boundary='Same loaded synthetic old-image node remains mounted under visible pending shimmer. Actual ready→generating→ready transition and native old media remain UNPROVEN.';
    });
  });
}
// PNG decoding only compares captured UI; no asset is generated, altered, downloaded or written separately.
function png(buffer) {
  let pos=8,width,height,channels,depth,ctype;const chunks=[];
  while(pos<buffer.length){const n=buffer.readUInt32BE(pos),type=buffer.toString('ascii',pos+4,pos+8),body=buffer.subarray(pos+8,pos+8+n);if(type==='IHDR'){width=body.readUInt32BE(0);height=body.readUInt32BE(4);depth=body[8];ctype=body[9];if(body[12]!==0)throw new Error('Interlaced PNG unsupported');}if(type==='IDAT')chunks.push(body);pos+=n+12;if(type==='IEND')break;}
  if(depth!==8||![2,6].includes(ctype))throw new Error('Unsupported screenshot PNG');channels=ctype===6?4:3;
  const raw=zlib.inflateSync(Buffer.concat(chunks)),stride=width*channels,pixels=Buffer.alloc(stride*height);
  const paeth=(a,b,c)=>{const p=a+b-c,pa=Math.abs(p-a),pb=Math.abs(p-b),pc=Math.abs(p-c);return pa<=pb&&pa<=pc?a:pb<=pc?b:c;};
  for(let y=0;y<height;y++)for(let x=0;x<stride;x++){const filter=raw[y*(stride+1)],at=y*stride+x,a=x>=channels?pixels[at-channels]:0,b=y?pixels[at-stride]:0,c=y&&x>=channels?pixels[at-stride-channels]:0,p=filter===0?0:filter===1?a:filter===2?b:filter===3?Math.floor((a+b)/2):filter===4?paeth(a,b,c):NaN;if(!Number.isFinite(p))throw new Error('Unsupported PNG filter');pixels[at]=(raw[y*(stride+1)+1+x]+p)&255;}
  return {width,height,pixel:(x,y)=>[...pixels.subarray((Math.floor(y)*width+Math.floor(x))*channels,(Math.floor(y)*width+Math.floor(x))*channels+3)]};
}
async function bannerProof(viewport,motion) {
  const fixture='Original channel GET ready identity with actual pre-existing same-origin prompt sample reused as clearly synthetic avatar/banner. Safe-area proof is component geometry/compositing, NOT generated channel branding.';
  await run('banner-safe-area-'+motion,viewport,motion,fixture,async row=>{
    const setup=identitySetup('ready');if(!setup)unsupported('Need original QA channel and actual same-origin prompt sample GET/decode');
    row.sample=selectedSample;
    await withPage(row,setup.overrides,async page=>{
      await visit(page,setup.target.id,'settings');
      // Exact src plus direct safe-frame parent distinguishes banner from the same sample reused as avatar.
      const images=page.locator('img');const index=await images.evaluateAll((els,url)=>els.findIndex(el=>el.getAttribute('src')===url&&[...el.parentElement.querySelectorAll('span')].some(span=>getComputedStyle(span).boxShadow.includes('9999px'))),selectedSample.url);
      if(index<0)throw new Error('Synthetic sample banner and safe-area frame not mounted');
      const img=images.nth(index);await renderedImage(img,selectedSample.url);const banner=img.locator('..');await banner.scrollIntoViewIfNeeded();await settle(page);
      const safe=page.getByRole('switch',{name:'Khung vùng hiển thị trên điện thoại 1546×423',exact:true});await expect(safe).toHaveCount(1);await expect(safe).toHaveAttribute('aria-checked','true');
      row.geometry=await banner.evaluate(el=>{
        const parent=el.getBoundingClientRect(),frame=[...el.querySelectorAll('span')].find(span=>getComputedStyle(span).boxShadow.includes('9999px'));if(!frame)return null;
        const r=frame.getBoundingClientRect(),cs=getComputedStyle(frame),image=el.querySelector('img');return {width:r.width,height:r.height,x:r.x-parent.x,y:r.y-parent.y,bannerWidth:parent.width,bannerHeight:parent.height,background:cs.backgroundColor,shadow:cs.boxShadow,overflow:getComputedStyle(el).overflow,objectFit:getComputedStyle(image).objectFit};
      });
      const g=row.geometry;expect(g).not.toBeNull();near(g.bannerWidth/g.bannerHeight,16/9,.01);near(g.width/g.bannerWidth,.604,.001);near(g.height/g.bannerHeight,.294,.001);near(g.x,(g.bannerWidth-g.width)/2);near(g.y,(g.bannerHeight-g.height)/2);expect(g.background).toBe('rgba(0, 0, 0, 0)');expect(g.overflow).toBe('hidden');expect(g.objectFit).toBe('cover');
      const onName=await screenshot(page,row,'banner-safe-on',banner);
      await safe.click();await expect(safe).toHaveAttribute('aria-checked','false');await banner.scrollIntoViewIfNeeded();
      const offName=await screenshot(page,row,'banner-safe-off',banner);
      const on=png(await fs.readFile(path.join(ROOT,onName))),off=png(await fs.readFile(path.join(ROOT,offName)));expect(on.width).toBe(off.width);expect(on.height).toBe(off.height);
      const delta=(x,y)=>{const a=on.pixel(x*on.width,y*on.height),b=off.pixel(x*off.width,y*off.height);return {max:Math.max(...a.map((v,i)=>Math.abs(v-b[i]))),darkening:(b[0]+b[1]+b[2]-a[0]-a[1]-a[2])/3};};
      const inside=[],outside=[];for(const x of [.35,.45,.55,.65])for(const y of [.43,.50,.57])inside.push(delta(x,y));for(const x of [.08,.12,.88,.92])for(const y of [.35,.50,.65])outside.push(delta(x,y));
      expect(inside.filter(d=>d.max<=1).length/inside.length).toBeGreaterThanOrEqual(.95);
      const average=outside.reduce((sum,d)=>sum+d.darkening,0)/outside.length;
      // A uniformly black sample cannot establish dimming pixels; never manufacture a passing image.
      if(average<=1)unsupported('Geometry proved, but chosen existing sample is insufficiently bright at exterior sample points for dimming pixel proof');
      row.pixelProof={source:'Screenshots of existing native prompt sample used ONLY as synthetic identity',inside,outside,averageExteriorDarkening:average};
      await safe.click();await expect(safe).toHaveAttribute('aria-checked','true');await screenshot(page,row,'identity-full');
      row.boundary='Centered 60.4% × 29.4% transparent safe region; exterior dimming sampled against same actual sample image with guide off. No claim about native generated banner.';
    });
  });
}
(async()=>{
  out.startedAt=new Date().toISOString();out.status='running';
  try {
    await fs.mkdir(ROOT,{recursive:true});
    cookies=JSON.parse(await fs.readFile(COOKIE_FILE,'utf8')).map(c=>({...c,domain:'127.0.0.1',secure:false}));
    browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
    await inventory();
    out.fixtureDefinitions=[{kind:'sources',description:'Entirely synthetic Rival, scan, posting and source-association GET objects; originals kept only as owner channel route anchors.'},{kind:'identity',description:'Actual native prompt sample URL discovered/GET-checked/decoded during this run, reused as synthetic old avatar/banner; generating state GET-only with no fake job/activity.'}];
    for(const viewport of VIEWPORTS)for(const motion of MOTIONS){await sourceProof(viewport,motion);await avatarProof(viewport,motion);await bannerProof(viewport,motion);}
    out.unsupported=out.cases.filter(c=>c.status==='unproven').map(c=>({name:c.name,viewport:c.viewport,reason:c.reason}));
    const unsafe=out.requests.filter(e=>['ABORT_NON_GET_BEFORE_NETWORK','ABORT_DOWNLOAD_BEFORE_NETWORK','ABORT_UNHANDLED_FIXTURE_GET','UNEXPECTED_DOWNLOAD_CANCELLED','PAGEERROR','UNEXPECTED_WEBSOCKET'].includes(e.action));
    out.safety={allNonGETAbortedBeforeNetworking:true,nonGETRequestsContinued:0,externalRequestsContinued:0,downloadsClicked:0,unexpectedEvents:unsafe,requestFence:'All browser HTTP requests routed; no APIRequestContext; WebSocket connections closed before connectToServer; browser downloads refused/cancelled as defense in depth.'};
    out.status=out.cases.some(c=>c.status==='failed')||unsafe.length?'FAIL':out.unsupported.length?'DONE_WITH_CONCERNS':'PASS_SYNTHETIC_FIXTURES_ONLY';
    if(out.status==='FAIL')process.exitCode=1;
  } catch(error) {out.status='BLOCKED';out.error=String(error.message||error);process.exitCode=1;}
  finally {
    try {if(browser){await browser.close();out.browserClosed=true;}}
    catch(error){out.browserClosed=false;out.closeError=String(error);out.status='FAIL';process.exitCode=1;}
    out.finishedAt=new Date().toISOString();await fs.writeFile(path.join(ROOT,'results.json'),JSON.stringify(out,null,2));
  }
  console.log(JSON.stringify({status:out.status,proofClass:out.proofClass,results:path.join(ROOT,'results.json'),cases:out.cases.map(c=>({name:c.name,width:c.viewport.width,status:c.status,reason:c.reason}))},null,2));
})().catch(error=>{console.error(String(error));process.exitCode=1;});
