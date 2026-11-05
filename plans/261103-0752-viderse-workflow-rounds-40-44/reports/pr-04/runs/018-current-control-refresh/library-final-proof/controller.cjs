'use strict';
// PRIVATE controller. Prepared only; run by integration owner after final rebuild.
// Every non-GET is aborted unless this controller explicitly fulfills a synthetic
// template PATCH. No intercepted mutation is ever continued to the server.
const {createRequire} = require('node:module');
const {chromium, expect} = createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');
const ROOT = __dirname;
const ORIGIN = 'http://127.0.0.1:8043';
const COOKIES = '/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const EXEC = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PHONE = {width:390,height:844}, PC = {width:1440,height:900};
const R = {status:'NOT_RUN', preparedOnly:false, cases:[], requests:[], fixtures:[], pageErrors:[], unproven:[], screenshots:[], safety:{realMutationRequestsContinued:0, externalRequestsContinued:0, downloadEvents:0}};
let browser, cookies, native;
const enc = encodeURIComponent;
const templatePath = (id,section) => `/#/templates/${enc(id)}${section ? '?section='+section : ''}`;
const promptPath = p => `/#/library?tab=prompts&prompt=${p.builtIn?'builtin':'mine'}&promptId=${enc(p.id)}`;
const modal = p => p.locator('[aria-modal="true"]:visible');
const clone = value => JSON.parse(JSON.stringify(value));
const json = (route,body,status=200) => route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
function unproven(reason){const e=new Error(reason);e.unproven=true;throw e;}
async function settle(page){
  await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect?.getComputedTiming().endTime)).map(a=>a.finished.catch(()=>{})));await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));});
}
async function capture(page,key){
  await settle(page);
  const filename=key.replace(/[^a-zA-Z0-9_-]/g,'-')+'.png';
  await page.screenshot({path:path.join(ROOT,filename)});
  const geometry=await page.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth}));
  R.screenshots.push({case:key,file:filename,geometry});
  expect(geometry.scrollWidth, 'horizontal overflow').toBeLessThanOrEqual(geometry.width+1);
  return filename;
}
async function navigate(page,hash){await page.goto(ORIGIN+hash,{waitUntil:'domcontentloaded'});}
async function readJSON(page,url){return page.evaluate(async url=>{const r=await fetch(url);if(!r.ok)throw new Error(`GET ${url}: ${r.status}`);return r.json();},url);}
async function make(viewport,key,override){
  const context=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
  await context.addCookies(cookies.map(c=>({...c,domain:'127.0.0.1',secure:false})));
  await context.route('**/*',async route=>{
    const req=route.request(),u=new URL(req.url());
    const entry={case:key,method:req.method(),path:u.pathname+u.search};
    // No credentials, full headers, or private response payloads are logged.
    if(u.origin!==ORIGIN){R.requests.push({...entry,action:'BLOCK_EXTERNAL'});return route.abort('blockedbyclient');}
    if(req.method()!=='GET'){
      if(req.method()==='PATCH' && /^\/api\/templates\/[^/]+$/.test(u.pathname) && override){
        if(await override(route,entry)){R.requests.push({...entry,action:'SYNTHETIC_PATCH'});return;}
      }
      R.requests.push({...entry,action:'BLOCK_MUTATION'});return route.abort('blockedbyclient');
    }
    if(/\/(?:export|download|downloads|discover|discovery|follow)(?:\/|$)|\/api\/sources\/parse|\/api\/platforms\//i.test(u.pathname)){
      R.requests.push({...entry,action:'BLOCK_FORBIDDEN_GET'});return route.abort('blockedbyclient');
    }
    if(override && await override(route,entry)){R.requests.push({...entry,action:'SYNTHETIC_GET'});return;}
    if(u.pathname.startsWith('/api/'))R.requests.push({...entry,action:'NATIVE_GET'});
    return route.continue();
  });
  const page=await context.newPage();page.setDefaultTimeout(9000);page.setDefaultNavigationTimeout(20000);
  page.on('pageerror',e=>R.pageErrors.push({case:key,message:e.message}));
  page.on('download',d=>{R.safety.downloadEvents++;void d.cancel();});
  page.on('popup',p=>void p.close());
  return {context,page};
}
async function run(key,viewport,proof,fn,override){
  const record={key,viewport,proof,status:'UNPROVEN',screenshots:[]};R.cases.push(record);
  let context,page;
  try{
    ({context,page}=await make(viewport,key,override));
    const detail=await fn(page,record);
    record.observed=detail??null;
    record.screenshots.push(await capture(page,key));
    record.status='PASS';
  }catch(e){
    record.status=e.unproven?'UNPROVEN':'FAIL';record.reason=e.message;
    if(e.unproven)R.unproven.push({case:key,reason:e.message});
    if(page)try{record.screenshots.push(await capture(page,key+'-failure'));record.body=(await page.locator('body').innerText()).slice(0,6000);}catch(s){record.captureError=s.message;}
  }finally{if(context)await context.close();}
}
async function oneModal(page){await expect(modal(page)).toHaveCount(1);await settle(page);return modal(page);}
async function focusTrap(page){
  const dialog=await oneModal(page);
  const items=dialog.locator('button:not(:disabled), a[href], input:not(:disabled), textarea:not(:disabled), select:not(:disabled), [tabindex="0"]').filter({visible:true});
  const count=await items.count();expect(count,'focusable modal controls').toBeGreaterThan(0);
  await items.last().focus();await page.keyboard.press('Tab');await expect(items.first()).toBeFocused();
  await items.first().focus();await page.keyboard.press('Shift+Tab');await expect(items.last()).toBeFocused();
  expect(await dialog.evaluate(el=>el.contains(document.activeElement))).toBe(true);
  return {focusableControls:count,forwardWrap:true,backwardWrap:true};
}
async function scrimClose(page){
  // Actual pointer in the backdrop, not DOM-dispatched click/force bypass.
  const b=await modal(page).boundingBox();if(!b)throw new Error('No modal bounds');
  if(b.y<=3 && b.x<=3)unproven('No exposed scrim hit area at this viewport');
  await page.mouse.click(b.y>3?2:1,b.y>3?Math.max(1,b.y/2):1);
  await expect(modal(page)).toHaveCount(0);
}
function nativeTemplate(predicate=()=>true){const t=native.templates.find(predicate);if(!t)unproven('No actual template meeting this case prerequisite');return t;}
async function discover(){
  await run('native-data-discovery',PC,'NATIVE_BACKEND_GET',async page=>{
    await navigate(page,'/#/library?tab=templates');
    const [templates,prompts,overlays,channels,videos]=await Promise.all(['/api/templates','/api/image-prompts','/api/overlays','/api/backdrop/channels','/api/backdrop/videos'].map(u=>readJSON(page,u)));
    native={templates:templates.items,prompts:prompts.items,overlays:overlays.items,channels,videos};
    native.details=await Promise.all(native.templates.map(t=>readJSON(page,`/api/templates/${enc(t.id)}`)));
    native.preview={};for(const t of native.templates)native.preview[t.id]=await readJSON(page,`/api/templates/${enc(t.id)}/preview`);
    expect(native.templates.length).toBeGreaterThan(0);expect(native.prompts.length).toBeGreaterThan(0);
    await expect(page.locator('article').filter({has:page.getByRole('link',{name:native.templates[0].name,exact:true})})).toBeVisible();
    // This receipt contains identifiers/counts/reasons, not credentials or entire recipes.
    R.native={templates:native.templates.map(t=>({id:t.id,name:t.name,sceneId:t.prompts.sceneId,imagesMode:t.images.mode})),prompts:native.prompts.map(p=>({id:p.id,name:p.name,kind:p.kind,builtIn:p.builtIn,sampleUrls:p.sampleUrls,usedBy:p.usedBy})),overlayCount:native.overlays.length,sourceChannelCount:channels.length,sourceVideoCount:videos.length,previewSources:Object.fromEntries(Object.entries(native.preview).map(([id,p])=>[id,p.sources]))};
    return {templates:native.templates.length,prompts:native.prompts.length,genuineSamplePrompts:native.prompts.filter(p=>p.sampleUrls?.length).length,channels:channels.length,videos:videos.length};
  });
}
async function nativeLibrary(){
  for(const viewport of [PC,PHONE]){
    for(const tab of ['templates','prompts','backdrops','overlays'])await run(`native-${viewport.width}-tab-${tab}`,viewport,'NATIVE_BACKEND_GET',async page=>{
      await navigate(page,'/#/library?tab='+tab);
      const tabs=page.getByRole(viewport.width<768?'radiogroup':'tablist',{name:'Thư viện',exact:true});await expect(tabs).toBeVisible();
      await expect(tabs.getByRole(viewport.width<768?'radio':'tab',{name:({templates:/Mẫu video/,prompts:/Chỉ dẫn ảnh/,backdrops:/Video nền/,overlays:/Lớp phủ/})[tab]})).toHaveAttribute(viewport.width<768?'aria-checked':'aria-selected','true');
      if(tab==='templates')await expect(page.getByRole('link',{name:native.templates[0].name,exact:true})).toBeVisible();
      if(tab==='prompts')await expect(page.getByRole('button',{name:new RegExp(native.prompts[0].name.replace(/[.*+?^${}()|[\]\\]/g,'\\$&'))}).first()).toBeVisible();
      if(tab==='overlays' && native.overlays.length)await expect(page.getByText(native.overlays[0].name,{exact:true}).first()).toBeVisible();
      if(tab==='backdrops' && !native.channels.length && !native.videos.length)await expect(page.getByText('Chưa có nguồn nền',{exact:true})).toBeVisible();
      return {tab,nativeCounts:{templates:native.templates.length,prompts:native.prompts.length,overlays:native.overlays.length,sourceChannels:native.channels.length,sourceVideos:native.videos.length}};
    });
    await run(`native-${viewport.width}-associated-template-art`,viewport,'NATIVE_BACKEND_GET',async page=>{
      await navigate(page,'/#/library?tab=templates');const art=[];
      for(const t of native.templates){
        const card=page.locator('article').filter({has:page.getByRole('link',{name:t.name,exact:true})});await expect(card).toBeVisible();
        const p=t.images.mode==='none'?undefined:native.prompts.find(p=>p.id===t.prompts.sceneId);const url=p?.sampleUrls?.[0];
        if(url){const image=card.locator('img');await expect(image).toHaveCount(1);await expect(image).toHaveAttribute('src',url);await expect(image).toHaveAttribute('alt',`Ảnh mẫu minh hoạ của chỉ dẫn ${p.name}, không phải bản xem thử đã dựng`);await image.scrollIntoViewIfNeeded();await expect.poll(()=>image.evaluate(el=>el.complete&&el.naturalWidth>0),{message:'genuine sample loaded'}).toBe(true);}
        else {await expect(card.locator('img')).toHaveCount(0);await expect(card.locator('[role="img"][aria-label="Chưa có ảnh mẫu minh hoạ"]')).toHaveCount(1);}
        art.push({templateId:t.id,promptId:p?.id??null,url:url??null,claim:'sample illustration only; never generated preview'});
      }return art;
    });
    await run(`native-${viewport.width}-prompt-card-samples`,viewport,'NATIVE_BACKEND_GET',async page=>{
      await navigate(page,'/#/library?tab=prompts');const checks=[];
      for(const p of native.prompts){
        const button=viewport.width<768?page.getByRole('button').filter({has:page.getByText(p.name,{exact:true})}):page.getByRole('button',{name:`Mở chỉ dẫn ${p.name}`,exact:true});
        await button.scrollIntoViewIfNeeded();await expect(button).toBeVisible();
        if(p.sampleUrls?.[0]){await expect(button.locator('img')).toHaveAttribute('src',p.sampleUrls[0]);await expect.poll(()=>button.locator('img').evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);}else await expect(button.locator('img')).toHaveCount(0);
        checks.push({promptId:p.id,expectedSample:p.sampleUrls?.[0]??null});
      }return checks;
    });
    await run(`native-${viewport.width}-template-exact-prompt-route`,viewport,'NATIVE_BACKEND_GET',async page=>{
      const detail=native.details.find(t=>{const id=t.recipe.scenes.scenePromptId;return id && id!==native.prompts[0].id && native.prompts.some(p=>p.id===id);});if(!detail)unproven('No actual template uses a non-first scene prompt');
      const prompt=native.prompts.find(p=>p.id===detail.recipe.scenes.scenePromptId);
      await navigate(page,templatePath(detail.id,'images'));const link=page.locator(`a[href*="promptId=${enc(prompt.id)}"]`);await expect(link).toHaveCount(1);await link.click();
      await expect(page).toHaveURL(ORIGIN+promptPath(prompt));await expect(page.locator('#vlb-prompt-name')).toHaveValue(prompt.name);
      if(viewport.width<768)await oneModal(page);
      await capture(page,`native-${viewport.width}-exact-prompt-open`);
      await page.getByRole('button',{name:'Về danh sách chỉ dẫn',exact:true}).click();await expect(page).toHaveURL(/#\/library\?tab=prompts$/);await expect(modal(page)).toHaveCount(0);
      return {templateId:detail.id,expectedPromptId:prompt.id,notFirstPromptId:native.prompts[0].id};
    });
    await run(`native-${viewport.width}-new-and-missing-prompt`,viewport,'NATIVE_BACKEND_GET',async page=>{
      const detail=nativeTemplate();await navigate(page,templatePath(detail.id,'images'));
      const newLink=page.locator('a[href="#/library?tab=prompts&prompt=new"]');
      if(viewport.width>=768){await expect(newLink).toBeVisible();await newLink.click();}else {await navigate(page,'/#/library?tab=prompts&prompt=new');}
      await expect(page.locator('#vlb-prompt-name')).toHaveValue('');if(viewport.width<768)await oneModal(page);
      await capture(page,`native-${viewport.width}-new-prompt-empty`);await page.getByRole('button',{name:'Về danh sách chỉ dẫn',exact:true}).click();await expect(page).toHaveURL(/#\/library\?tab=prompts$/);
      await navigate(page,'/#/library?tab=prompts&prompt=mine&promptId=v38-deliberately-absent');await expect(page.getByText(/Không tìm thấy chỉ dẫn ảnh này|Không tìm thấy chỉ dẫn này/).first()).toBeVisible();await expect(page.locator('#vlb-prompt-name')).toHaveCount(0);
      return {newRoute:'empty editor; no create request',missingRoute:'no first-item fallback',mobileTemplateNewLink:viewport.width<768?'UNPROVEN: template new link hidden on phone; direct route exercised':'exercised template link'};
    });
  }
  const detailTemplate=nativeTemplate();let detailReject=true;
  const detailOverride=async(route,e)=>{if(e.method==='GET'&&new URL(route.request().url()).pathname===`/api/templates/${enc(detailTemplate.id)}`&&detailReject){await json(route,{error:'SYNTHETIC V38 template detail GET failure',code:'v38_fixture'},500);return true;}return false;};
  await run('synthetic-pc-template-detail-error-retry',PC,'SYNTHETIC_DETAIL_GET_FAILURE_THEN_NATIVE_GET',async page=>{
    await navigate(page,templatePath(detailTemplate.id));await expect(page.getByText('Không tải được mẫu video. Thử lại.',{exact:true})).toBeVisible();await expect(page.getByText('Không tìm thấy mẫu video này. Có thể nó đã bị xoá.',{exact:true})).toHaveCount(0);await capture(page,'synthetic-pc-detail-get-error');
    // Enforces the named retry criterion; inspected source instead has
    // notFound + back-to-library, so this may report a genuine missing retry.
    const retry=page.getByRole('button',{name:'Thử lại',exact:true});await expect(retry).toBeVisible();detailReject=false;await retry.click();await expect(page.getByRole('textbox',{name:'Tên mẫu video',exact:true})).toHaveValue(detailTemplate.name);
    return {initialDetailError:true,nativeRetry:true};
  },detailOverride);
  await run('native-phone-prompt-overlay-cancellations',PHONE,'NATIVE_BACKEND_GET',async page=>{
    const prompt=native.prompts.find((p,i)=>i>0);if(!prompt)unproven('No non-first actual prompt');
    await navigate(page,'/#/library?tab=prompts');const trigger=page.getByRole('button').filter({has:page.getByText(prompt.name,{exact:true})});await trigger.click();await oneModal(page);await expect(page.locator('#vlb-prompt-name')).toHaveValue(prompt.name);const trap=await focusTrap(page);
    await page.keyboard.press('Escape');await expect(modal(page)).toHaveCount(0);await expect(page).toHaveURL(/#\/library\?tab=prompts$/);await expect(trigger).toBeFocused();
    await trigger.click();await oneModal(page);await scrimClose(page);await expect(page).toHaveURL(/#\/library\?tab=prompts$/);await expect(trigger).toBeFocused();
    const add=page.getByRole('button',{name:'Thêm vào thư viện',exact:true});await add.click();await oneModal(page);await modal(page).getByRole('button',{name:'Chỉ dẫn ảnh mới',exact:true}).click();await oneModal(page);await expect(page.locator('#vlb-prompt-name')).toHaveValue('');await page.keyboard.press('Escape');await expect(modal(page)).toHaveCount(0);await expect(page).toHaveURL(/#\/library\?tab=prompts$/);
    return {promptId:prompt.id,...trap,escape:true,scrim:true,focusReturn:true,newMenuSingleOverlay:true,newMenuCloseFocus:'UNPROVEN: original menu trigger focus not asserted after route handoff'};
  });
}
async function listError(){
  for(const kind of ['500','network']){
    let reject=true,hits=0;
    const override=async(route,e)=>{if(e.method==='GET'&&new URL(route.request().url()).pathname==='/api/templates'){hits++;if(reject){if(kind==='network')await route.abort('failed');else await json(route,{error:'SYNTHETIC V38 template list GET failure',code:'v38_fixture'},500);return true;}}return false;};
    await run(`synthetic-pc-template-list-${kind}-retry`,PC,'SYNTHETIC_GET_FAILURE_THEN_NATIVE_GET',async page=>{
      await navigate(page,'/#/library?tab=templates');await expect(page.getByText('Không tải được thư viện. Thử lại.',{exact:true}).first()).toBeVisible();await expect(page.getByText('Chưa có mẫu video nào',{exact:true})).toHaveCount(0);
      await capture(page,`synthetic-pc-list-${kind}-error`);const before=hits;reject=false;await page.getByRole('button',{name:'Thử lại',exact:true}).click();await expect(page.getByRole('link',{name:native.templates[0].name,exact:true})).toBeVisible();expect(hits).toBeGreaterThan(before);return {injected:kind,retryReadNative:true,hits};
    },override);
  }
}
async function picker(){
  for(const viewport of [PC,PHONE])await run(`native-${viewport.width}-backdrop-picker-wrapper`,viewport,'NATIVE_BACKEND_GET',async page=>{
    // Query and select by recipe capability, not a hard-coded/first fixture ID.
    const detail=native.details.find(t=>t.recipe && t.recipe.backdrop!==undefined)??native.details.find(t=>t.recipe?.appearance);if(!detail)unproven('No actual template with backdrop section recipe');
    await navigate(page,templatePath(detail.id,'backdrop'));const trigger=page.getByRole('button',{name:'Gắn kênh hoặc video',exact:true});await trigger.click();const dialog=await oneModal(page);
    await expect(dialog).toHaveAccessibleName(`Gắn video nền vào ${detail.name}`);await expect(dialog).toHaveClass(viewport.width<768?/m-sheet/:/kit-dlg/);await expect(dialog.locator('.vbd-dlg')).toHaveCount(1);
    await expect(page.locator('#vbd-pick-link')).toBeVisible();const trap=await focusTrap(page);const bounds=await dialog.boundingBox();expect(bounds.x).toBeGreaterThanOrEqual(-1);expect(bounds.y).toBeGreaterThanOrEqual(-1);expect(bounds.x+bounds.width).toBeLessThanOrEqual(viewport.width+1);expect(bounds.y+bounds.height).toBeLessThanOrEqual(viewport.height+1);
    await capture(page,`native-${viewport.width}-backdrop-picker-open`);await page.keyboard.press('Escape');await expect(modal(page)).toHaveCount(0);await expect(trigger).toBeFocused();
    await trigger.click();await oneModal(page);await scrimClose(page);await expect(trigger).toBeFocused();
    return {templateId:detail.id,wrapper:viewport.width<768?'Sheet':'Dialog',bounds,...trap,escape:true,scrim:true,focusReturn:true,attachNotSubmitted:true};
  });
}
async function renameFailures(){
  for(const viewport of [PC,PHONE])for(const failure of ['400','409','network']){
    const t=nativeTemplate();let patches=[];
    const override=async(route,e)=>{if(e.method==='PATCH'&&new URL(route.request().url()).pathname===`/api/templates/${enc(t.id)}`){patches.push(route.request().postDataJSON());if(failure==='network')await route.abort('failed');else await json(route,{error:`SYNTHETIC V38 ${failure} save rejection`,code:failure==='409'?'conflict':'validation',...(failure==='400'?{fields:[{field:'name',message:'SYNTHETIC V38 name rejected'}]}:{})},Number(failure));return true;}return false;};
    await run(`synthetic-${viewport.width}-rename-${failure}`,viewport,'SYNTHETIC_PATCH_REJECTION_NO_SERVER_WRITE',async page=>{
      await navigate(page,templatePath(t.id));const entered=`V38 synthetic rejected ${failure}`;
      let input;
      if(viewport.width<768){await page.getByRole('button',{name:'Thao tác với mẫu video',exact:true}).click();await oneModal(page);await page.getByRole('button',{name:'Đổi tên',exact:true}).click();await oneModal(page);input=page.locator('#mtpl-rename');await input.fill(entered);await page.getByRole('button',{name:'Lưu tên',exact:true}).click();}
      else {input=page.getByRole('textbox',{name:'Tên mẫu video',exact:true});await input.fill(entered);await input.press('Tab');}
      await expect.poll(()=>patches.length).toBe(1);await expect(input).toHaveValue(entered);
      if(failure==='400')await expect(page.getByText('SYNTHETIC V38 name rejected',{exact:true}).first()).toBeVisible();
      if(failure==='409')await expect(page.getByRole('button',{name:'Tải lại',exact:true}).first()).toBeVisible();
      if(failure==='network')await expect(page.getByText(/Failed to fetch|fetch|mạng|kết nối|Network/).first()).toBeVisible();
      await expect(page.getByText('Đã đổi tên mẫu video.',{exact:true})).toHaveCount(0);
      if(viewport.width<768){await expect(page.locator('#mtpl-rename')).toBeVisible();await oneModal(page);await expect(page.locator('[role="status"]').filter({hasText:/Chưa lưu|thay đổi|xung đột|mới hơn|được sửa/}).first()).toBeVisible();}
      else await expect(page.locator('.vt-saved')).toHaveAttribute('data-state',failure==='409'?'conflict':'error');
      expect(patches[0].name).toBe(entered);return {failure,patchCount:patches.length,inputRetained:true,dialogRetained:viewport.width<768,successClaimAbsent:true};
    },override);
  }
}
async function savedRevision(){
  for(const viewport of [PC,PHONE])for(const metadata of ['old-revision','missing-revision']){
    const detail=clone(native.details.find(t=>t.recipe));const id=detail.id;let current=clone(detail),patches=0,gets=0;
    current.preview={state:'fresh',builtAt:null,minutes:3};
    const override=async(route,e)=>{
      const p=new URL(route.request().url()).pathname;
      if(e.method==='GET'&&p===`/api/templates/${enc(id)}`){await json(route,current);return true;}
      if(e.method==='GET'&&p===`/api/templates/${enc(id)}/preview`){gets++;await json(route,{state:'fresh',builtAt:null,minutes:3,sources:[],...(metadata==='old-revision'?{templateRevision:detail.revision}:{})});return true;}
      if(e.method==='PATCH'&&p===`/api/templates/${enc(id)}`){patches++;const body=route.request().postDataJSON();current={...current,name:body.name??current.name,recipe:body.recipe??current.recipe,revision:current.revision+1,preview:{state:'stale',builtAt:null,minutes:3}};await json(route,current);return true;}return false;
    };
    await run(`synthetic-${viewport.width}-cached-fresh-${metadata}`,viewport,'SYNTHETIC_GET_AND_PATCH_ACCEPTANCE_NO_MEDIA_NO_SERVER_WRITE',async page=>{
      await navigate(page,templatePath(id));await expect(page.getByText('Khớp cài đặt',{exact:true}).first()).toBeVisible();await expect.poll(()=>gets).toBeGreaterThan(0);
      const name='V38 synthetic saved revision';
      if(viewport.width<768){await page.getByRole('button',{name:'Thao tác với mẫu video',exact:true}).click();await page.getByRole('button',{name:'Đổi tên',exact:true}).click();await expect(page.locator('#mtpl-rename')).toBeVisible();await page.locator('#mtpl-rename').fill(name);await page.getByRole('button',{name:'Lưu tên',exact:true}).click();await expect(modal(page)).toHaveCount(0);await expect(page.getByText('Cài đặt đã đổi · dựng lại để xem',{exact:true})).toBeVisible();await expect(page.getByText(/Đã lưu · .*bản xem thử cần dựng lại/).first()).toBeVisible();}
      else {const input=page.getByRole('textbox',{name:'Tên mẫu video',exact:true});await input.fill(name);await input.press('Tab');await expect(page.locator('.vt-saved')).toHaveAttribute('data-state','saved');await expect(page.getByText('Cần dựng lại',{exact:true}).first()).toBeVisible();}
      await expect.poll(()=>patches).toBe(1);await expect.poll(()=>gets).toBeGreaterThan(1);await expect(page.getByText('Khớp cài đặt',{exact:true})).toHaveCount(0);
      return {metadata,originalRevision:detail.revision,syntheticSavedRevision:current.revision,previewGETs:gets,patches,syntheticStateOnly:true,noPreviewURLInvented:true};
    },override);
  }
}
async function previewSources(){
  for(const t of native.templates)await run(`native-phone-preview-sources-${t.id}`,PHONE,'NATIVE_BACKEND_GET',async page=>{
    const status=native.preview[t.id];await navigate(page,templatePath(t.id));
    if(!status.sources?.length){await expect(page.getByText('Chưa có video nào đã làm xong',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:/^Dựng lại/})).toBeDisabled();return {sources:[],selection:'UNPROVEN: no actual ready source',build:'not invoked'};}
    const trigger=page.locator('#mtpl-preview-source');await trigger.click();await oneModal(page);const disabled=[];
    for(const s of status.sources){const row=page.getByRole('option').filter({hasText:s.title});await expect(row).toHaveCount(1);if(s.disabled){await expect(row).toBeDisabled();if(s.reason)await expect(row).toContainText(s.reason);disabled.push({jobId:s.jobId,reason:s.reason??null});}}
    await capture(page,`native-phone-sources-${t.id}-open`);const enabled=status.sources.filter(s=>!s.disabled);const writes=R.requests.filter(r=>r.case===`native-phone-preview-sources-${t.id}`&&r.method!=='GET').length;
    if(enabled.length){const choice=enabled[enabled.length-1];await page.getByRole('option').filter({hasText:choice.title}).click();await expect(modal(page)).toHaveCount(0);await expect(trigger).toContainText(choice.title);expect(R.requests.filter(r=>r.case===`native-phone-preview-sources-${t.id}`&&r.method!=='GET')).toHaveLength(writes);return {selectedJobId:choice.jobId,disabled,noBuildPOST:true};}
    await page.keyboard.press('Escape');await expect(modal(page)).toHaveCount(0);return {disabled,selection:'UNPROVEN: all actual sources disabled',noBuildPOST:true};
  });
  const t=nativeTemplate();const sources=[{jobId:'v38-synthetic-ready-a',title:'SYNTHETIC ready source A',channelName:'SYNTHETIC channel',durationSec:181},{jobId:'v38-synthetic-ready-b',title:'SYNTHETIC ready source B',channelName:'SYNTHETIC channel',durationSec:247},{jobId:'v38-synthetic-unavailable',title:'SYNTHETIC unavailable source',durationSec:90,disabled:true,reason:'SYNTHETIC missing required source media'}];
  const override=async(route,e)=>{if(e.method==='GET'&&new URL(route.request().url()).pathname===`/api/templates/${enc(t.id)}/preview`){await json(route,{state:'none',builtAt:null,sources});return true;}return false;};
  await run('synthetic-phone-source-pick-not-build',PHONE,'SYNTHETIC_PREVIEW_GET_READY_ROWS_BUILD_POST_BLOCKED',async page=>{
    await navigate(page,templatePath(t.id));const trigger=page.locator('#mtpl-preview-source');await trigger.click();const trap=await focusTrap(page);await expect(page.getByRole('option').filter({hasText:sources[2].title})).toBeDisabled();await expect(page.getByRole('option').filter({hasText:sources[2].title})).toContainText(sources[2].reason);
    await page.keyboard.press('Escape');await expect(modal(page)).toHaveCount(0);await expect(trigger).toBeFocused();await trigger.click();await scrimClose(page);await expect(trigger).toBeFocused();
    await trigger.click();await page.getByRole('option').filter({hasText:sources[1].title}).click();await expect(modal(page)).toHaveCount(0);await expect(trigger).toContainText(sources[1].title);expect(R.requests.filter(r=>r.case==='synthetic-phone-source-pick-not-build'&&r.method==='POST')).toHaveLength(0);
    const requestPromise=page.waitForRequest(req=>req.method()==='POST'&&new URL(req.url()).pathname===`/api/templates/${enc(t.id)}/preview`);await page.getByRole('button',{name:'Dựng lại bản xem thử',exact:true}).click();const request=await requestPromise;expect(request.postDataJSON().jobId).toBe(sources[1].jobId);await expect(page.getByRole('alert').first()).toBeVisible();return {...trap,synthetic:true,selected:sources[1].jobId,pickEmitsPOST:false,explicitBuildPOST:'aborted before server',buildOutput:'UNPROVEN; forbidden'};
  },override);
}
async function assetCases(){
  const assets=[...native.channels.map(value=>({kind:'channel',value})),...native.videos.map(value=>({kind:'video',value})),...native.overlays.map(value=>({kind:'overlay',value}))];
  if(!native.channels.length)R.unproven.push({case:'native-source-channel-detail',reason:'Actual native backend has no backdrop channels; synthetic source details below are not backend proof'});
  if(!native.videos.length)R.unproven.push({case:'native-source-video-detail',reason:'Actual native backend has no backdrop videos; synthetic source details below are not backend proof'});
  for(const asset of assets)await assetCase(asset,'native',undefined);
  const t=nativeTemplate(),usage={templateId:t.id,name:t.name,mode:'random',kinds:['clip'],chosenCount:0};
  const now='2026-11-04T00:00:00Z';
  const channels=[{id:'v38-synthetic-used-channel',platform:'twitch',handle:'synthetic',url:ORIGIN+'/v38-synthetic-provider-link-not-opened',title:'SYNTHETIC used source channel',avatarUrl:null,addedAt:now,listedAt:now,listError:null,scanNextPage:null,scanTotal:0,scanStopKnown:true,counts:{clip:0},pausedUntil:null,pauseReason:null,usedBy:[usage]}];
  const videos=[{id:'v38-synthetic-unused-video',platform:'bilibili',url:ORIGIN+'/v38-synthetic-provider-link-not-opened',title:'SYNTHETIC unused failed source video',durationSec:120,segment:{fromSec:10,toSec:70},state:'failed',errorDetail:{code:'v38_fixture',message:'SYNTHETIC source unavailable; no download attempted'},usedBy:[],addedAt:now,updatedAt:now}];
  const overlay=native.overlays.find(o=>o.builtIn);const fixtureOverlays=overlay?[{...clone(overlay),id:'v38-synthetic-used-overlay',name:'SYNTHETIC associated overlay',usedBy:[{id:t.id,name:t.name}]}]:[];
  const override=async(route,e)=>{if(e.method!=='GET')return false;const p=new URL(route.request().url()).pathname;if(p==='/api/backdrop/channels'){await json(route,channels);return true;}if(p==='/api/backdrop/videos'){await json(route,videos);return true;}if(p==='/api/overlays'&&fixtureOverlays.length){await json(route,{items:fixtureOverlays});return true;}return false;};
  for(const asset of [...channels.map(value=>({kind:'channel',value})),...videos.map(value=>({kind:'video',value})),...fixtureOverlays.map(value=>({kind:'overlay',value}))])await assetCase(asset,'synthetic',override);
}
async function assetCase(asset,mode,override){
  const v=asset.value,name=asset.kind==='channel'?(v.title??v.handle):(v.name??v.title),tab=asset.kind==='overlay'?'overlays':'backdrops';
  await run(`${mode}-phone-${asset.kind}-${v.id}`,PHONE,mode==='native'?'NATIVE_BACKEND_GET':'SYNTHETIC_ASSET_GET_AND_USAGE_NOT_BACKEND_PROOF',async page=>{
    await navigate(page,'/#/library?tab='+tab);const trigger=page.getByRole('button').filter({has:page.getByText(name,{exact:true})});await expect(trigger).toHaveCount(1);await trigger.focus();await page.keyboard.press('Enter');const dialog=await oneModal(page);await expect(dialog).toHaveAccessibleName(name);const trap=await focusTrap(page);
    if(asset.kind==='overlay'){
      if(v.mime.startsWith('image/'))await expect(dialog.locator('img')).toHaveAttribute('src',v.url);else await expect(dialog.locator('video')).toHaveAttribute('src',v.url);
      if(v.builtIn){await expect(dialog.getByText('Hiệu ứng có sẵn không xoá được.',{exact:true})).toBeVisible();await expect(dialog.getByRole('button',{name:'Xoá',exact:true})).toHaveCount(0);}
    }
    if(mode==='synthetic'&&asset.kind==='channel')await expect(dialog.getByText('Chưa có video dùng được.',{exact:true})).toBeVisible();
    const usages=v.usedBy.map(ref=>({id:ref.id??ref.templateId,name:ref.name}));
    if(asset.kind!=='overlay'&&usages.length)await expect(dialog.getByRole('button',{name:/Bỏ|Xoá/})).toBeDisabled();
    await capture(page,`${mode}-phone-${asset.kind}-${v.id}-detail`);await page.keyboard.press('Escape');await expect(modal(page)).toHaveCount(0);await expect(trigger).toBeFocused();
    await trigger.focus();await page.keyboard.press('Space');await oneModal(page);await scrimClose(page);await expect(trigger).toBeFocused();await trigger.click();await oneModal(page);
    if(usages.length){const dest=usages[usages.length-1];await modal(page).getByRole('button',{name:dest.name+' · Nền & hiệu ứng',exact:true}).click();await expect(page).toHaveURL(ORIGIN+templatePath(dest.id,'backdrop'));await expect(modal(page)).toHaveCount(0);return {id:v.id,...trap,builtinUndeletable:!!v.builtIn,usedByDestination:dest.id,keyboardEnterSpace:true,escape:true,scrim:true};}
    if(asset.kind==='overlay'){
      const dest=native.templates.find((t,i)=>i>0&&t.id!==native.templates[0].id);if(!dest)unproven('No non-first actual template to prove unused-overlay destination');await modal(page).getByRole('button',{name:dest.name,exact:true}).click();await expect(page).toHaveURL(ORIGIN+templatePath(dest.id,'backdrop'));await expect(modal(page)).toHaveCount(0);return {id:v.id,...trap,builtinUndeletable:!!v.builtIn,unusedDestination:dest.id,noApplyMutation:true};
    }
    // Cancellation only; never click the destructive confirmation action.
    await modal(page).getByRole('button',{name:/Bỏ|Xoá/}).click();await oneModal(page);await expect(page.getByRole('alertdialog')).toBeVisible();await page.getByRole('button',{name:'Huỷ',exact:true}).click();await oneModal(page);await expect(modal(page)).toHaveAccessibleName(name);
    await modal(page).getByRole('button',{name:/Bỏ|Xoá/}).click();await oneModal(page);await page.keyboard.press('Escape');await oneModal(page);await expect(modal(page)).toHaveAccessibleName(name);return {id:v.id,...trap,confirmSingleOverlay:true,cancelRestoresDetail:true,escapeRestoresDetail:true,deleteNeverSubmitted:true};
  },override);
}
(async()=>{
  await fs.mkdir(ROOT,{recursive:true});cookies=JSON.parse(await fs.readFile(COOKIES,'utf8'));
  try{
    browser=await chromium.launch({executablePath:EXEC,headless:true});await discover();
    if(!native)throw new Error('Native GET discovery failed; no fixture identities available');
    R.fixtures=[{scope:'template list errors',method:'GET',changes:'500/network failure, then retry restored to native response'},{scope:'rename failures',method:'PATCH',changes:'400 fields.name, 409 conflict, or network abort; never server writes'},{scope:'cached fresh vs saved revision',method:'GET/PATCH',changes:'synthetic fresh status without media/project URL; accepted in-memory name/revision and stale detail'},{scope:'ready source selection',method:'GET',changes:'clearly synthetic job rows; explicit build POST aborted'},{scope:'asset details',method:'GET',changes:'clearly synthetic channel/video/overlay usage; real overlay URL reused only for visual rendering'}];
    await nativeLibrary();await listError();await picker();await renameFailures();await savedRevision();await previewSources();await assetCases();
    R.unproven.push(...[
      'No actual create/save/delete/import/duplicate/attach/default/provider/generation/download/build request is allowed; backend mutation success and persisted readback are unproven.',
      'Ready-source build completion, media output, preview player continuity and genuine preview freshness are unproven; revision cases explicitly use synthetic metadata without project or preview URLs.',
      'Source refresh/retry, provider links, media play/pause, source download, and destructive confirmation submission are intentionally unproven.',
      'Failed PATCH recipe/name pending-queue merge, conflict reload failure/success, all-four-section save failure propagation and viewport-breakpoint transition are unproven by this controller.',
      'Overlay template-list loading/failure/empty states, broken sample fallback, prompt-list failure and prompt create/duplicate/delete round trips are unproven.',
      'No visual design score, contrast audit, screen-reader audit, drag gesture, short viewport, reduced-motion preference, or cross-browser certification is claimed.'
    ].map(reason=>({case:'explicit-scope-limit',reason})));
    R.status=R.cases.some(c=>c.status==='FAIL')||R.pageErrors.length||R.safety.downloadEvents?'DONE_WITH_CONCERNS':R.unproven.length?'DONE_WITH_CONCERNS':'DONE';
  }catch(e){R.status='BLOCKED';R.fatal=e.message;process.exitCode=1;}
  finally{
    if(browser)await browser.close();R.browserClosed=true;R.finishedAt=new Date().toISOString();
    await fs.writeFile(path.join(ROOT,'results.json'),JSON.stringify(R,null,2));
  }
  const counts=Object.fromEntries(['PASS','FAIL','UNPROVEN'].map(s=>[s,R.cases.filter(c=>c.status===s).length]));
  console.log(JSON.stringify({status:R.status,counts,pageErrors:R.pageErrors.length,unproven:R.unproven.length,results:path.join(ROOT,'results.json')}));
  if(counts.FAIL||R.pageErrors.length||R.safety.downloadEvents)process.exitCode=1;
})().catch(e=>{console.error(e.message);process.exitCode=1;});
