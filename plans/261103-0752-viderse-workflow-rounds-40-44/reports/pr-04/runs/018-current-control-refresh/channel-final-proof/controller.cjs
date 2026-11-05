'use strict';
// Prepared, NOT executed. Native read-only UI evidence and explicitly labeled GET/failure fixtures.
const { createRequire } = require('node:module');
const { chromium, expect } = createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');
const zlib = require('node:zlib');
const ROOT = __dirname;
const ORIGIN = 'http://127.0.0.1:8043';
const COOKIE_FILE = '/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const IDS = ['78799ed0-57fa-4cff-ab00-509bdc65e5da', '98ae68b2-813e-427a-8238-8eb0e86de231', 'c76cc725-0e9d-43fd-98d1-4d55412510e8'];
const VIEWPORTS = [{width:1440,height:900}, {width:390,height:844}];
const REGEN = ['Vẽ lại ảnh bìa kênh · 1 ảnh','Vẽ lại avatar · 1 ảnh','Đặt tên khác','Tạo lại cả bộ · 2 ảnh'];
const out = {status:'NOT_RUN', startedAt:null, origin:ORIGIN, fixtureChannelIds:IDS, cases:[], requests:[], fixtures:[], unsupported:[], limits:[
  'No actual writes, generation, AI/provider admission, preview building, discovery/follow or downloads are permitted.',
  'Native GET data is separate from GET-only association/identity overrides and synthetic PATCH failures.',
  'Successful persistence, reload after a real save, successful link/create/default-template changes, paid identity transitions and all other unexercised states remain UNPROVEN.',
  'External requests are aborted, including remote avatars. A blocked/missing image is not proof of a genuine rendered artwork.',
  'Request absence is scoped to the completed UI actions, finite-animation/font settlement and two animation frames; no arbitrary timeout is used as proof.'
]};
let browser, cookies, data;
const clone = value => JSON.parse(JSON.stringify(value));
const closeTo = (actual, expected, tolerance=1) => expect(Math.abs(actual-expected)).toBeLessThanOrEqual(tolerance);
const endpoint = id => '/api/owner-channels/'+id;
const unproven = reason => { const e=new Error(reason); e.unproven=true; throw e; };
const nativeChannels = () => IDS.map(id=>data.channels.find(c=>c.id===id)).filter(Boolean);
const sourceChannel = () => [...nativeChannels()].sort((a,b)=>b.sources.length-a.sources.length)[0];
const profileChannel = () => [...nativeChannels()].sort((a,b)=>Number(!!b.avatarUrl&&!!b.bannerUrl)-Number(!!a.avatarUrl&&!!a.bannerUrl))[0];
async function getJSON(context, pathname) {
  // APIRequestContext does not use browser routes. This helper is the ONLY API request path.
  if (!['/api/owner-channels','/api/templates','/api/image-prompts','/api/rivals'].includes(pathname)) throw new Error('Non-allowlisted API read');
  const response = await context.request.get(ORIGIN+pathname, {maxRedirects:0});
  if (!response.ok()) throw new Error('Native GET '+pathname+' returned '+response.status());
  return response.json();
}
async function settle(page) {
  await page.evaluate(async()=>{
    await document.fonts.ready;
    await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect?.getComputedTiming().endTime)).map(a=>a.finished.catch(()=>{})));
    await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
  });
}
async function shot(page, name, locator) {
  await settle(page);
  const filename=name+'.png';
  await (locator||page).screenshot({path:path.join(ROOT,filename),animations:'disabled'});
  return filename;
}
async function withPage(viewport, label, options, fn) {
  const context=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,reducedMotion:options.motion||'no-preference',...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
  await context.addCookies(cookies);
  const scope={label,fixture:options.fixture||null,events:[],overrides:options.overrides||new Map(),patch:null,gates:[]};
  await context.route('**/*', async route=>{
    const req=route.request(), url=new URL(req.url()), method=req.method();
    const event={scope:label,method,path:url.pathname,queryKeys:[...url.searchParams.keys()]};
    if (url.origin!==ORIGIN) { out.requests.push({...event,action:'ABORT_EXTERNAL',host:url.host}); return route.abort('blockedbyclient'); }
    if (method!=='GET') {
      // NEVER continue any mutation; only the exact expected brief PATCH can be synthetically rejected.
      if (method==='PATCH' && scope.patch && url.pathname===scope.patch.path) {
        let body; try { body=req.postDataJSON(); } catch {}
        if (body && Object.keys(body).length===1 && typeof body.brief==='string') {
          const current=scope.patch; scope.patch=null;
          const observed={...event,action:current.mode==='400'?'SYNTHETIC_400':'SYNTHETIC_NETWORK_FAILURE',bodyKeys:['brief'],draftLength:body.brief.length};
          scope.events.push(observed); out.requests.push(observed); current.entered=true;
          await current.gate;
          if (current.mode==='400') return route.fulfill({status:400,contentType:'application/json',body:JSON.stringify({error:'Acceptance synthetic brief rejection',code:'validation',fields:[{field:'brief',message:'Acceptance synthetic brief rejection'}]})});
          return route.abort('failed');
        }
      }
      const denied={...event,action:'ABORT_MUTATION'}; scope.events.push(denied); out.requests.push(denied);
      return route.abort('blockedbyclient');
    }
    if (url.searchParams.has('download') || /\/(?:export|download|localdownload|externaldownload)(?:\/|$)/i.test(url.pathname)) {
      out.requests.push({...event,action:'ABORT_DOWNLOAD'}); return route.abort('blockedbyclient');
    }
    if (scope.overrides.has(url.pathname)) {
      out.requests.push({...event,action:'GET_OVERRIDE',fixture:scope.fixture});
      return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(scope.overrides.get(url.pathname))});
    }
    return route.continue();
  });
  const page=await context.newPage();
  page.setDefaultTimeout(10000);
  page.on('download',download=>{download.cancel().catch(()=>{});out.requests.push({scope:label,action:'UNEXPECTED_DOWNLOAD_CANCELLED'});});
  page.on('pageerror',error=>out.requests.push({scope:label,action:'PAGEERROR',message:String(error)}));
  try { return await fn(page,scope); }
  catch(error) {
    const filename=viewport.width+'-'+label+'-'+(error.unproven?'unproven':'failure')+'.png';
    await page.screenshot({path:path.join(ROOT,filename),animations:'disabled'}).catch(()=>{});
    const row=out.cases.findLast(c=>c.name===label&&c.viewport.width===viewport.width);
    if(row) row.screenshots.push(filename);
    throw error;
  }
  finally { for(const release of scope.gates) release(); await context.close(); }
}
async function run(name, viewport, proof, fn) {
  const row={name,viewport,proof,status:'running',screenshots:[]}; out.cases.push(row);
  try { await fn(row); row.status='pass'; }
  catch(error) { row.status=error.unproven?'unproven':'failed'; row.reason=String(error.message||error); }
}
async function visit(page, id, tab) {
  await page.goto(ORIGIN+'/#/channels/'+id+'?tab='+tab,{waitUntil:'domcontentloaded'});
  if(tab==='settings') await expect(page.locator('#vch-brief')).toBeVisible();
  if(tab==='sources') await expect(page.locator('#vch-add-rival')).toBeVisible();
  if(tab==='templates') await expect(page.getByRole('combobox',{name:'Đổi template',exact:true})).toBeVisible();
  await settle(page);
}
async function blankForm(page, inputId, errorId, scope) {
  const input=page.locator(inputId), form=page.locator('form').filter({has:input});
  await expect(form).toHaveCount(1);
  const before=scope.events.length;
  for(const value of ['', '   ']) {
    await input.fill(value);
    await form.getByRole('button').click();
    await expect(input).toBeFocused();
    await expect(input).toHaveAttribute('aria-invalid','true');
    await expect(input).toHaveAttribute('aria-describedby',errorId.slice(1));
    await expect(page.locator(errorId)).toBeVisible();
    await expect(page.locator(errorId)).toHaveText('Dán @handle hoặc link kênh YouTube của đối thủ trước');
    await settle(page);
    expect(scope.events.slice(before)).toEqual([]);
  }
  // Keyboard form submission is independently covered, not only the pointer path.
  await input.press('Enter');
  await expect(page.locator(errorId)).toBeVisible(); await expect(input).toBeFocused();
  await settle(page); expect(scope.events.slice(before)).toEqual([]);
}
async function sourceLayout(viewport) {
  await run('sources-native-layout-and-blank-follow',viewport,'native GET / no writes',async row=>{
    const channel=sourceChannel(); if(!channel) unproven('Original QA channels absent');
    await withPage(viewport,row.name,{ },async(page,scope)=>{
      await visit(page,channel.id,'sources'); row.channelId=channel.id;
      if(viewport.width===1440) {
        const grid=page.locator('.vch-sources');
        const columns=await grid.evaluate(el=>getComputedStyle(el).gridTemplateColumns.split(' ').map(parseFloat));
        expect(columns).toHaveLength(2); closeTo(columns[0]/columns[1],1.3,0.015); row.columns=columns;
        const rows=page.locator('.vch-rival');
        if(!channel.sources.length) { row.layout='UNPROVEN: no native linked rivals'; }
        else {
          await expect(rows).toHaveCount(channel.sources.length);
          row.rivals=await rows.evaluateAll(els=>els.map(el=>{
            const top=el.children[0], actions=el.querySelector('.vch-rival__actions'), avatar=top.firstElementChild;
            const a=avatar.getBoundingClientRect(), t=top.getBoundingClientRect(), b=actions.getBoundingClientRect();
            return {avatar:{width:a.width,height:a.height},top:{x:t.x,y:t.y,width:t.width,height:t.height},actions:{x:b.x,y:b.y,width:b.width,height:b.height},metadata:el.querySelector('.vch-rival__meta')?.textContent,status:el.querySelector('.vch-chip')?.textContent};
          }));
          for(const rival of row.rivals) { closeTo(rival.avatar.width,34);closeTo(rival.avatar.height,34);expect(rival.actions.y).toBeGreaterThanOrEqual(rival.top.y+rival.top.height+8);closeTo(rival.actions.width,rival.top.width); }
        }
      }
      await blankForm(page,'#vch-add-rival','#vch-add-rival-error',scope);
      row.screenshots.push(await shot(page,viewport.width+'-sources-native-invalid'));
      if(!channel.sources.length) unproven('Grid/validation measured, but native 34px populated rival/two-row state absent');
    });
  });
}
function sharedSetup() {
  const channels=nativeChannels(), rivals=data.rivals;
  for(const target of channels) {
    const here=rivals.find(r=>target.sources.some(s=>s.rivalId===r.id));
    const elsewhere=rivals.find(r=>!target.sources.some(s=>s.rivalId===r.id)&&channels.some(other=>other.id!==target.id&&other.sources.some(s=>s.rivalId===r.id)));
    if(here&&elsewhere) return {target,here,elsewhere,fixture:null,overrides:new Map()};
  }
  if(channels.length<2||rivals.length<2) return null;
  const target=clone(channels.find(c=>c.id===IDS[0])||channels[0]);
  const other=clone(channels.find(c=>c.id!==target.id));
  const sorted=[...rivals].sort((a,b)=>a.id.localeCompare(b.id)), here=sorted[0], elsewhere=sorted[1];
  const source=r=>({rivalId:r.id,title:r.title,handle:r.handle,linkedAt:target.createdAt,reference:true});
  target.sources=[source(here)]; other.sources=[source(elsewhere)];
  const overridden=data.channels.map(c=>c.id===target.id?target:c.id===other.id?other:c);
  const fixture='GET-only rival association fixture: real rivals/owner channel IDs; target has one source, other original channel has second source; no backend association written';
  return {target,here,elsewhere,fixture,overrides:new Map([['/api/owner-channels',{channels:overridden}],[endpoint(target.id),target],[endpoint(other.id),other]])};
}
async function linkPicker(viewport, motion='no-preference') {
  const setup=sharedSetup();
  await run('sources-picker-'+motion,viewport,setup?.fixture||'native shared-rival data',async row=>{
    if(!setup) unproven('Need two original channels and two genuine followed rivals for shared eligibility state');
    row.fixture=setup.fixture; if(setup.fixture) out.fixtures.push({name:row.name,description:setup.fixture});
    await withPage(viewport,row.name,{fixture:setup.fixture,overrides:setup.overrides,motion},async(page,scope)=>{
      await visit(page,setup.target.id,'sources');
      const trigger=page.locator('.vch-linkslot').getByRole('button',{name:'Liên kết đối thủ',exact:true});
      await trigger.click();
      const dialog=page.getByRole('dialog',{name:'Liên kết đối thủ với '+setup.target.name,exact:true});
      await expect(page.locator('[role="dialog"]:visible')).toHaveCount(1); await expect(dialog).toBeVisible();
      const here=dialog.getByRole('checkbox').filter({hasText:setup.here.title});
      const elsewhere=dialog.getByRole('checkbox').filter({hasText:setup.elsewhere.title});
      await expect(here).toHaveCount(1);await expect(elsewhere).toHaveCount(1);
      await expect(here).toHaveAttribute('aria-disabled','true');await expect(here).toHaveAttribute('aria-checked','true');
      await here.click({force:true});await expect(here).toHaveAttribute('aria-checked','true');
      expect(await elsewhere.getAttribute('aria-disabled')).not.toBe('true');
      await expect(elsewhere).toContainText('dùng chung được');await elsewhere.click();
      await expect(elsewhere).toHaveAttribute('aria-checked','true');
      await expect(dialog.getByRole('button',{name:'Liên kết 1',exact:true})).toBeEnabled();
      if(viewport.width<768) {
        await expect(dialog).toHaveClass(/m-sheet/); await expect(dialog).toHaveAttribute('aria-modal','true');
        await expect(page.locator('.m-overlay:visible')).toHaveCount(1);await expect(page.locator('.m-scrim:visible')).toHaveCount(1);
        await settle(page);
        row.geometry=await dialog.evaluate(el=>{const r=el.getBoundingClientRect();return {x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom,viewportWidth:innerWidth,viewportHeight:innerHeight,scrollWidth:el.scrollWidth,clientWidth:el.clientWidth,bodyOverflow:document.body.style.overflow,focusInside:el.contains(document.activeElement)};});
        expect(row.geometry.x).toBeGreaterThanOrEqual(-1);expect(row.geometry.width).toBeLessThanOrEqual(viewport.width+1);expect(row.geometry.bottom).toBeLessThanOrEqual(viewport.height+1);expect(row.geometry.scrollWidth).toBeLessThanOrEqual(row.geometry.clientWidth+1);expect(row.geometry.bodyOverflow).toBe('hidden');expect(row.geometry.focusInside).toBe(true);
        const count=await dialog.locator('button:not(:disabled),a[href],input:not(:disabled),[tabindex="0"]').count();
        for(let n=0;n<count+2;n++) {await page.keyboard.press('Tab');expect(await dialog.evaluate(el=>el.contains(document.activeElement))).toBe(true);}
        for(let n=0;n<count+2;n++) {await page.keyboard.press('Shift+Tab');expect(await dialog.evaluate(el=>el.contains(document.activeElement))).toBe(true);}
      }
      row.screenshots.push(await shot(page,viewport.width+'-sources-picker-'+motion));
      await page.keyboard.press('Escape');await expect(page.locator('[role="dialog"]:visible')).toHaveCount(0);await expect(trigger).toBeFocused();
      if(viewport.width<768) {
        await expect.poll(()=>page.evaluate(()=>document.body.style.overflow)).not.toBe('hidden');
        await trigger.click();await expect(dialog).toBeVisible();await settle(page);
        const box=await dialog.boundingBox();if(!box||box.y<2) unproven('No exposed scrim coordinate for mobile dismissal');
        await page.locator('.m-scrim').click({position:{x:viewport.width/2,y:Math.max(1,box.y/2)}});
        await expect(page.locator('[role="dialog"]:visible')).toHaveCount(0);await expect(trigger).toBeFocused();
      }
      await settle(page);expect(scope.events).toEqual([]);
      row.boundary='Only selection toggled; never clicked Link N or follow/explore';
    });
  });
}
async function createBlank(viewport) {
  await run('create-native-blank-follow',viewport,'native GET / local validation only',async row=>{
    await withPage(viewport,row.name,{},async(page,scope)=>{
      await page.goto(ORIGIN+'/#/channels/new',{waitUntil:'domcontentloaded'});await expect(page.locator('#vch-new-rival')).toBeVisible();
      await blankForm(page,'#vch-new-rival','#vch-new-rival-error',scope);
      row.screenshots.push(await shot(page,viewport.width+'-create-native-invalid'));
      row.boundary='No rival selected; never clicked Create channel';
    });
  });
}
function profileSetup(generating=false) {
  const real=profileChannel(); if(!real) return null;
  const target=clone(real), reasons=[];
  if(generating) {if(target.identity.state!=='generating'||target.identity.part!=='avatar') {target.identity={state:'generating',part:'avatar'};reasons.push('identity.state=generating, part=avatar; no invented activity/progress/job');}}
  else if(target.identity.state!=='ready') {target.identity={state:'ready'};reasons.push('identity.state=ready for dirty/save lock isolation');}
  const fixture=reasons.length?'GET-only identity override: '+reasons.join('; ')+'; all old media URLs and branding remain native':null;
  const overrides=fixture?new Map([[endpoint(real.id),target],['/api/owner-channels',{channels:data.channels.map(c=>c.id===real.id?target:c)}]]):new Map();
  return {real,target,fixture,overrides};
}
async function assertLocks(page) {
  for(const label of REGEN) await expect(page.getByRole('button',{name:label,exact:true})).toHaveAttribute('aria-disabled','true');
}
async function briefFailure(viewport,mode) {
  const setup=profileSetup();
  await run('brief-dirty-pending-'+mode,viewport,'synthetic PATCH rejection; '+(setup?.fixture||'native ready identity'),async row=>{
    if(!setup) unproven('No original channel for brief editor');
    row.fixture=setup.fixture; if(setup.fixture) out.fixtures.push({name:row.name,description:setup.fixture});
    await withPage(viewport,row.name,{fixture:setup.fixture,overrides:setup.overrides},async(page,scope)=>{
      await visit(page,setup.target.id,'settings');const brief=page.locator('#vch-brief');
      const draft='Acceptance '+mode+' — bản nháp giữ nguyên\nKhông tạo nhận diện.';
      await brief.fill(draft);await assertLocks(page);await expect(brief).toHaveValue(draft);
      row.screenshots.push(await shot(page,viewport.width+'-brief-dirty-'+mode));
      let release;const gate=new Promise(resolve=>{release=resolve;});scope.gates.push(release);
      const hold={path:endpoint(setup.target.id),mode,gate,entered:false};scope.patch=hold;
      // Blur/click race: the real UI attempts PATCH, intercepted BEFORE networking, held without sleeps.
      await page.getByRole('button',{name:REGEN[3],exact:true}).click({force:true});
      await expect.poll(()=>hold.entered).toBe(true);
      await expect(page.locator('span[aria-live="polite"]').filter({hasText:/^Đang lưu mô tả kênh\. Chờ lưu xong trước khi tạo lại\.$/})).toBeVisible();
      await assertLocks(page);
      for(const label of REGEN) await page.getByRole('button',{name:label,exact:true}).click({force:true});
      expect(scope.events.filter(e=>e.method==='POST')).toEqual([]);
      row.screenshots.push(await shot(page,viewport.width+'-brief-pending-'+mode));release();
      await expect(brief).toHaveValue(draft);await expect(brief).toHaveAttribute('aria-invalid','true');
      if(mode==='400') await expect(page.getByText('Acceptance synthetic brief rejection',{exact:true})).toBeVisible();
      else await expect(page.getByText('Không kết nối được máy chủ. Vui lòng thử lại.',{exact:true})).toBeVisible();
      await expect(page.getByRole('button',{name:'Thử lưu lại mô tả',exact:true})).toBeVisible();await assertLocks(page);
      for(const label of REGEN) await page.getByRole('button',{name:label,exact:true}).click({force:true});
      await settle(page);expect(scope.events.filter(e=>e.method==='POST')).toEqual([]);expect(scope.events.filter(e=>e.method==='PATCH')).toHaveLength(1);
      row.screenshots.push(await shot(page,viewport.width+'-brief-error-'+mode));
      row.requestProof=scope.events;row.boundary='Error/draft/locks only, NOT real backend save or successful persistence';
    });
  });
}
// Minimal dependency-free decoder for Chromium PNG screenshot pixel comparisons (8-bit RGB/RGBA).
function png(buffer) {
  let pos=8,width,height,channels,depth,ctype;const chunks=[];
  while(pos<buffer.length) { const n=buffer.readUInt32BE(pos),type=buffer.toString('ascii',pos+4,pos+8),body=buffer.subarray(pos+8,pos+8+n);if(type==='IHDR'){width=body.readUInt32BE(0);height=body.readUInt32BE(4);depth=body[8];ctype=body[9];if(body[12]!==0) throw new Error('Interlaced PNG unsupported');}if(type==='IDAT')chunks.push(body);pos+=n+12;if(type==='IEND')break; }
  if(depth!==8||![2,6].includes(ctype)) throw new Error('Unsupported screenshot PNG encoding');channels=ctype===6?4:3;
  const raw=zlib.inflateSync(Buffer.concat(chunks)),stride=width*channels,pixels=Buffer.alloc(stride*height);
  const paeth=(a,b,c)=>{const p=a+b-c,pa=Math.abs(p-a),pb=Math.abs(p-b),pc=Math.abs(p-c);return pa<=pb&&pa<=pc?a:pb<=pc?b:c;};
  for(let y=0;y<height;y++){const filter=raw[y*(stride+1)];for(let x=0;x<stride;x++){const at=y*stride+x,a=x>=channels?pixels[at-channels]:0,b=y?pixels[at-stride]:0,c=y&&x>=channels?pixels[at-stride-channels]:0;const predictor=filter===0?0:filter===1?a:filter===2?b:filter===3?Math.floor((a+b)/2):filter===4?paeth(a,b,c):NaN;if(!Number.isFinite(predictor))throw new Error('Unsupported PNG filter');pixels[at]=(raw[y*(stride+1)+1+x]+predictor)&255;}}
  return {width,height,pixel:(x,y)=>[...pixels.subarray((Math.floor(y)*width+Math.floor(x))*channels,(Math.floor(y)*width+Math.floor(x))*channels+3)]};
}
async function profileGeometry(viewport) {
  await run('profile-native-layout-safe-area',viewport,'native identity/media GET; no writes',async row=>{
    const channel=profileChannel();if(!channel) unproven('Original QA profile absent');
    await withPage(viewport,row.name,{},async(page)=>{
      await visit(page,channel.id,'settings');row.channelId=channel.id;
      const bannerButton=page.getByRole('button',{name:REGEN[0],exact:true}),avatarButton=page.getByRole('button',{name:REGEN[1],exact:true}),nameButton=page.getByRole('button',{name:REGEN[2],exact:true});
      if(viewport.width<768) {
        const [b,a,n]=await Promise.all([bannerButton.boundingBox(),avatarButton.boundingBox(),nameButton.boundingBox()]);if(!b||!a||!n)throw new Error('Profile actions missing geometry');
        closeTo(a.y,n.y,1);closeTo(a.width,n.width,1);closeTo(b.width,n.x+n.width-a.x,1);expect(b.width).toBeGreaterThan(a.width*1.9);for(const box of [b,a,n])expect(box.height).toBeGreaterThanOrEqual(44);row.actions={banner:b,avatar:a,rename:n};
        const download=page.getByRole('button',{name:'Tải avatar + ảnh bìa kênh',exact:true});if(await download.count()){const d=await download.boundingBox();closeTo(d.width,b.width);row.actions.download=d;}
      }
      row.screenshots.push(await shot(page,viewport.width+'-profile-native-layout'));
      if(!channel.bannerUrl || new URL(channel.bannerUrl,ORIGIN).origin!==ORIGIN) unproven('Native real same-origin banner unavailable; safe-area pixel proof unsupported');
      // Select by the exact real API URL, not a fabricated art asset or first page image.
      const exactImage=page.locator('img');
      const found=await exactImage.evaluateAll((els,url)=>els.findIndex(el=>el.getAttribute('src')===url),channel.bannerUrl);
      if(found<0) unproven('Real banner image is not mounted');
      const img=exactImage.nth(found);await expect.poll(()=>img.evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);
      const banner=img.locator('..');await banner.scrollIntoViewIfNeeded();
      const safe=page.getByRole('switch',{name:'Khung vùng hiển thị trên điện thoại 1546×423',exact:true});
      if(await safe.count()!==1) unproven('Safe-area switch accessible name/count differs; inspect screenshot rather than guessing');
      await expect(safe).toHaveAttribute('aria-checked','true');
      const frame=await banner.evaluate(el=>{const parent=el.getBoundingClientRect();const found=[...el.querySelectorAll('span')].find(node=>getComputedStyle(node).boxShadow.includes('9999px'));if(!found)return null;const r=found.getBoundingClientRect(),cs=getComputedStyle(found);return {width:r.width,height:r.height,x:r.x-parent.x,y:r.y-parent.y,bannerWidth:parent.width,bannerHeight:parent.height,background:cs.backgroundColor,shadow:cs.boxShadow,overflow:getComputedStyle(el).overflow};});
      expect(frame).not.toBeNull();closeTo(frame.width/frame.bannerWidth,.604,.001);closeTo(frame.height/frame.bannerHeight,.294,.001);closeTo(frame.x,(frame.bannerWidth-frame.width)/2,1);closeTo(frame.y,(frame.bannerHeight-frame.height)/2,1);expect(frame.background).toBe('rgba(0, 0, 0, 0)');expect(frame.overflow).toBe('hidden');row.safeArea=frame;
      const onName=await shot(page,viewport.width+'-safe-area-on',banner);row.screenshots.push(onName);
      await safe.click();await expect(safe).toHaveAttribute('aria-checked','false');const offName=await shot(page,viewport.width+'-safe-area-off',banner);row.screenshots.push(offName);
      const on=png(await fs.readFile(path.join(ROOT,onName))),off=png(await fs.readFile(path.join(ROOT,offName)));
      const delta=(x,y)=>{const a=on.pixel(x*on.width,y*on.height),b=off.pixel(x*off.width,y*off.height);return {max:Math.max(...a.map((v,i)=>Math.abs(v-b[i]))),darkening:(b[0]+b[1]+b[2]-a[0]-a[1]-a[2])/3};};
      const inside=[],outside=[];for(const x of [.35,.45,.55,.65])for(const y of [.43,.50,.57])inside.push(delta(x,y));for(const x of [.08,.12,.88,.92])for(const y of [.35,.50,.65])outside.push(delta(x,y));
      expect(inside.filter(d=>d.max<=1).length/inside.length).toBeGreaterThanOrEqual(.95);expect(outside.reduce((sum,d)=>sum+d.darkening,0)/outside.length).toBeGreaterThan(1);row.pixelProof={inside,outside,source:'actual real banner screenshots, guide on/off'};
    });
  });
}
async function avatarPending(viewport,motion) {
  const setup=profileSetup(true);
  await run('profile-old-avatar-pending-'+motion,viewport,'native old media + '+(setup?.fixture||'native avatar-generating identity'),async row=>{
    if(!setup?.real.avatarUrl||new URL(setup.real.avatarUrl,ORIGIN).origin!==ORIGIN) unproven('Need native old same-origin avatar; external/missing media is not a rendered-avatar pass');
    row.fixture=setup.fixture;if(setup.fixture)out.fixtures.push({name:row.name,description:setup.fixture});
    await withPage(viewport,row.name,{fixture:setup.fixture,overrides:setup.overrides,motion},async(page)=>{
      await visit(page,setup.target.id,'settings');
      const status=page.getByRole('status',{name:'Đang tạo avatar mới; ảnh đang dùng được giữ tới khi xong.',exact:true});await expect(status).toBeVisible();
      const wrapper=status.locator('..');await expect(wrapper).toHaveAttribute('aria-busy','true');
      const imgs=wrapper.locator('img');await expect(imgs).toHaveCount(1);await expect(imgs).toHaveAttribute('src',setup.real.avatarUrl);await expect.poll(()=>imgs.evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);
      row.overlay=await status.evaluate(el=>{const cs=getComputedStyle(el),before=getComputedStyle(el,'::before'),after=getComputedStyle(el,'::after'),r=el.getBoundingClientRect();return {width:r.width,height:r.height,borderRadius:cs.borderRadius,background:cs.backgroundColor,animation:cs.animationName,beforeAnimation:before.animationName,afterAnimation:after.animationName};});
      closeTo(row.overlay.width,row.overlay.height,1);expect(row.overlay.width).toBeGreaterThan(0);
      if(motion==='reduce') expect([row.overlay.animation,row.overlay.beforeAnimation,row.overlay.afterAnimation].every(name=>name==='none')).toBe(true);
      else expect([row.overlay.animation,row.overlay.beforeAnimation,row.overlay.afterAnimation].some(name=>name!=='none')).toBe(true);
      row.oldAvatarUrl=setup.real.avatarUrl;row.screenshots.push(await shot(page,viewport.width+'-old-avatar-pending-'+motion));
      row.boundary='Generating state rendered by GET override when necessary; no transition, provider run, completion or matching activity progress claimed';
    });
  });
}
async function templateProof(viewport) {
  await run('template-native-card-menu-and-images-route',viewport,'native templates/prompts and original channel defaults; no PATCH',async row=>{
    const association=data.templates.find(t=>t.images.mode!=='none'&&data.prompts.some(p=>p.id===t.prompts.sceneId&&p.sampleUrls?.length));
    const channel=nativeChannels().find(c=>c.defaultTemplateId===association?.id)||nativeChannels().find(c=>data.templates.some(t=>t.id===c.defaultTemplateId));
    if(!channel) unproven('Original channel with native default-template summary unavailable');
    row.channelId=channel.id;row.defaultTemplateId=channel.defaultTemplateId;
    await withPage(viewport,row.name,{},async(page,scope)=>{
      await visit(page,channel.id,'templates');
      const surface=page.locator('[aria-label="Template mặc định"]').filter({has:page.getByRole('combobox',{name:'Đổi template',exact:true})});
      await expect(surface).toHaveCount(1);
      // Card's first aria-hidden span is the explicit artwork wrapper in TemplatesTab.
      const art=surface.locator('span[aria-hidden="true"]').first();await expect(art).toBeVisible();
      const box=await art.boundingBox();closeTo(box.width,viewport.width<768?112:128,.6);closeTo(box.height,viewport.width<768?63:72,.6);row.cardArtwork=box;
      const current=data.templates.find(t=>t.id===channel.defaultTemplateId),prompt=data.prompts.find(p=>p.id===current?.prompts.sceneId),url=current?.images.mode!=='none'?prompt?.sampleUrls?.[0]:null;
      row.artworkProvenance={templateId:current?.id,promptId:prompt?.id,sampleUrl:url||null,claim:'Associated prompt sample illustration, NOT a generated template preview'};
      if(url&&new URL(url,ORIGIN).origin!==ORIGIN) unproven('Associated real sample URL is external and intentionally blocked by same-origin-only fence');
      if(url) {
        const img=art.locator('img');await expect(img).toHaveCount(1);await expect(img).toHaveAttribute('src',url);await expect.poll(()=>img.evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);
      } else {row.artwork='UNPROVEN: current default has no genuine associated sample; honest fallback only';}
      row.screenshots.push(await shot(page,viewport.width+'-template-native-card'));
      const trigger=page.getByRole('combobox',{name:'Đổi template',exact:true}),before=await trigger.boundingBox();
      await trigger.click();await expect(page.getByRole('option')).toHaveCount(data.templates.length);await settle(page);
      const after=await trigger.boundingBox();closeTo(after.height,before.height,.5);row.trigger={before,after};
      row.options=[];
      for(const template of data.templates) {
        const option=page.getByRole('option').filter({hasText:template.name});await expect(option).toHaveCount(1);
        if(template.prompts.sceneName)await expect(option).toContainText(template.prompts.sceneName);
        await expect(option).toContainText(template.channels.length+' kênh đang dùng');
        if(template.backdrop.enabled&&!template.backdrop.channelCount&&!template.backdrop.videoCount)await expect(option).toContainText('Cần sửa · thiếu clip nền');
        // The rich option image wrapper can be nested in the shared select's icon wrapper.
        const measurements=await option.evaluate(el=>[...el.querySelectorAll('span[aria-hidden="true"]')].map(node=>{const r=node.getBoundingClientRect();return {width:r.width,height:r.height,hasArtwork:!!node.querySelector('img,[role="img"]')};}).filter(r=>r.hasArtwork));
        expect(measurements.some(r=>Math.abs(r.width-48)<.6&&Math.abs(r.height-27)<.6)).toBe(true);
        const boundPrompt=data.prompts.find(p=>p.id===template.prompts.sceneId),sample=template.images.mode!=='none'?boundPrompt?.sampleUrls?.[0]:null;
        if(sample){const optionImg=option.locator('img');await expect(optionImg).toHaveAttribute('src',sample);await optionImg.scrollIntoViewIfNeeded();await expect.poll(()=>optionImg.evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);}
        row.options.push({id:template.id,description:await option.innerText(),thumbs:measurements,sampleUrl:sample||null});
      }
      if(viewport.width<768){await expect(page.locator('.m-overlay:visible')).toHaveCount(1);await expect(page.locator('[role="dialog"]:visible')).toHaveCount(1);}
      row.screenshots.push(await shot(page,viewport.width+'-template-native-menu'));
      await page.keyboard.press('Escape');await expect(page.getByRole('option')).toHaveCount(0);
      await page.getByRole('button',{name:'Sửa template',exact:true}).click();await expect(page).toHaveURL(new RegExp('/templates/'+channel.defaultTemplateId+'\\?section=images$'));
      await expect(page.getByText('Chỉ dẫn ảnh cảnh',{exact:true}).first()).toBeVisible();row.screenshots.push(await shot(page,viewport.width+'-template-images-route'));await settle(page);expect(scope.events).toEqual([]);
      if(!url)unproven('Card/menu geometry and images route exercised; native default lacks associated sample, so populated artwork state not proved');
    });
  });
}
(async()=>{
  out.startedAt=new Date().toISOString();out.status='running';
  try {
    cookies=JSON.parse(await fs.readFile(COOKIE_FILE,'utf8')).map(c=>({...c,domain:'127.0.0.1',secure:false}));
    browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
    const probe=await browser.newContext({serviceWorkers:'block'});
    try {await probe.addCookies(cookies);data={channels:(await getJSON(probe,'/api/owner-channels')).channels,templates:(await getJSON(probe,'/api/templates')).items,prompts:(await getJSON(probe,'/api/image-prompts')).items,rivals:(await getJSON(probe,'/api/rivals')).rivals};}
    finally {await probe.close();}
    out.nativeInventory={originalChannels:nativeChannels().map(c=>({id:c.id,sources:c.sources.map(s=>s.rivalId),identityState:c.identity.state,defaultTemplateId:c.defaultTemplateId,hasAvatar:!!c.avatarUrl,hasBanner:!!c.bannerUrl})),templates:data.templates.length,prompts:data.prompts.length,promptsWithSamples:data.prompts.filter(p=>p.sampleUrls?.length).length,templateSampleBindings:data.templates.filter(t=>t.images.mode!=='none'&&data.prompts.some(p=>p.id===t.prompts.sceneId&&p.sampleUrls?.length)).map(t=>({templateId:t.id,promptId:t.prompts.sceneId}))};
    for(const viewport of VIEWPORTS) {
      await sourceLayout(viewport);await linkPicker(viewport);if(viewport.width<768)await linkPicker(viewport,'reduce');
      await createBlank(viewport);await briefFailure(viewport,'400');await briefFailure(viewport,'network');await profileGeometry(viewport);
      await avatarPending(viewport,'no-preference');await avatarPending(viewport,'reduce');await templateProof(viewport);
    }
    const unsafe=out.requests.filter(e=>['ABORT_MUTATION','UNEXPECTED_DOWNLOAD_CANCELLED','ABORT_DOWNLOAD','PAGEERROR'].includes(e.action));
    out.safety={allActualWritesBlocked:true,actualWriteRequestsContinued:0,unexpectedEvents:unsafe,requestFence:'browser route handles every non-GET before continue; APIRequestContext is restricted to four literal GET inventory endpoints'};
    out.unsupported=out.cases.filter(c=>c.status==='unproven').map(c=>({name:c.name,viewport:c.viewport,reason:c.reason}));
    out.status=out.cases.some(c=>c.status==='failed')||unsafe.length?'FAIL':out.unsupported.length?'DONE_WITH_CONCERNS':'PASS_BOUNDED_SCOPE';
    if(out.status==='FAIL')process.exitCode=1;
  } catch(error) {out.status='BLOCKED';out.error=String(error.message||error);process.exitCode=1;}
  finally {if(browser)await browser.close();out.finishedAt=new Date().toISOString();await fs.writeFile(path.join(ROOT,'results.json'),JSON.stringify(out,null,2));}
  console.log(JSON.stringify({status:out.status,results:path.join(ROOT,'results.json'),cases:out.cases.map(c=>({name:c.name,width:c.viewport.width,status:c.status,reason:c.reason}))},null,2));
})().catch(error=>{console.error(String(error));process.exitCode=1;});
