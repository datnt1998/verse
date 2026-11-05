'use strict';
// PRIVATE, PREPARED ONLY. Integration owner runs sequentially after final rebuild.
// Synthetic PATCH responses exercise browser consumers, NEVER backend persistence.
const {createRequire} = require('node:module');
const {chromium, expect} = createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs = require('node:fs/promises');
const path = require('node:path');
const ROOT = __dirname;
const ORIGIN = 'http://127.0.0.1:8043';
const COOKIES = '/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const EXEC = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PC = {width:1440,height:900}, PHONE = {width:390,height:844};
const R = {status:'NOT_RUN',preparedOnly:false,cases:[],requests:[],screenshots:[],pageErrors:[],limitations:[
  'All PATCH rejection/acceptance and queued-save normalization responses are browser-local synthetic fixtures; no backend writes or persistence proof.',
  'Native template IDs and baseline recipes are discovered through actual authenticated GETs at execution time. Conflict/detail recovery returns to actual GET responses, not fabricated backend state.',
  'Only images placement plus name exercise pending-queue consumers; all-four-section propagation, remount persistence, viewport transition, multi-client concurrency, build/inference, media output, metadata parsing/download and exports are unproven.',
  'No visual design score, full accessibility audit, cross-browser certification, or genuine generated preview freshness is claimed.'
],safety:{nonGETContinued:0,externalContinued:0,downloads:0,websocketsBlocked:0,unexpectedMutations:0,routeErrors:[]}};
let browser, cookies, baseline;
const clone = x => JSON.parse(JSON.stringify(x));
const enc = encodeURIComponent;
const routeFor = id => '/#/templates/'+enc(id)+'?section=images';
const targetFor = id => '/api/templates/'+enc(id);
const json = (route,body,status=200) => route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
const modal = page => page.locator('[aria-modal="true"]:visible');
function gate(){let resolve;const promise=new Promise(r=>{resolve=r;});return {promise,release:()=>resolve()};}
async function settle(page){await page.evaluate(async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));});}
async function capture(page,key,record){
  await settle(page);const file=key+'.png';await page.screenshot({path:path.join(ROOT,file)});
  const geometry=await page.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth}));
  R.screenshots.push({case:record.key,file,geometry});record.screenshots.push(file);
  expect(geometry.scrollWidth,'horizontal overflow').toBeLessThanOrEqual(geometry.width+1);
}
function allowedAPI(p){
  return /^\/api\/(?:me|events|activity|notifications\/settings|queue|status|owner-channels|channels\/summary|setup\/state|templates|image-prompts|overlays|backdrop\/channels|backdrop\/videos)$/.test(p)
    || (baseline && (p===targetFor(baseline.id)||p===targetFor(baseline.id)+'/preview'))
    || /^\/api\/image-prompts\/[^/]+\/samples\/[^/]+\/image$/.test(p);
}
async function make(viewport,key,fixture){
  const context=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
  await context.addCookies(cookies.map(c=>({...c,domain:'127.0.0.1',secure:false})));
  if(context.routeWebSocket)await context.routeWebSocket('**/*',socket=>{R.safety.websocketsBlocked++;socket.close();});
  await context.route('**/*',async route=>{
    const req=route.request(),u=new URL(req.url());
    const entry={case:key,method:req.method(),path:u.pathname+u.search,resource:req.resourceType(),scope:u.origin===ORIGIN?'LOCAL':'EXTERNAL'};R.requests.push(entry);
    try{
      if(u.origin!==ORIGIN){entry.action='BLOCK_EXTERNAL';return await route.abort('blockedbyclient');}
      if(req.method()!=='GET'){
        // Only this exact native template PATCH can be fulfilled synthetically.
        // There is deliberately no mutation route.continue/fallback/fetch path.
        if(req.method()==='PATCH'&&baseline&&u.pathname===targetFor(baseline.id)&&fixture?.patch){entry.action='SYNTHETIC_PATCH';return await fixture.patch(route,entry);}
        entry.action='BLOCK_MUTATION';R.safety.unexpectedMutations++;return await route.abort('blockedbyclient');
      }
      if(/(?:^|\/)(?:export|download|downloads|discover|discovery|follow|parse)(?:\/|$)/i.test(u.pathname)||/\.(?:zip|ytf|mp4|webm|mp3|wav|m3u8|ts)$/i.test(u.pathname)||['media'].includes(req.resourceType())){entry.action='BLOCK_DOWNLOAD_MEDIA_DISCOVERY';return await route.abort('blockedbyclient');}
      if(u.pathname.startsWith('/api/')&&!allowedAPI(u.pathname)){entry.action='BLOCK_UNLISTED_API_GET';return await route.abort('blockedbyclient');}
      if(baseline&&u.pathname===targetFor(baseline.id)&&fixture?.get&&await fixture.get(route,entry)){entry.action='SYNTHETIC_GET';return;}
      entry.action=u.pathname.startsWith('/api/')?'NATIVE_GET':'LOCAL_STATIC_GET';
      // Count at the sole continuation boundary, not from assumed fixture success.
      if(req.method()!=='GET')R.safety.nonGETContinued++;
      if(u.origin!==ORIGIN)R.safety.externalContinued++;
      return await route.continue();
    }catch(error){entry.routeError=error.message;R.safety.routeErrors.push({case:key,message:error.message});try{await route.abort('failed');}catch{} }
  });
  const page=await context.newPage();page.setDefaultTimeout(10000);page.setDefaultNavigationTimeout(20000);
  page.on('pageerror',e=>R.pageErrors.push({case:key,message:e.message}));
  page.on('download',d=>{R.safety.downloads++;void d.cancel();});
  page.on('popup',p=>void p.close());
  return {context,page};
}
async function run(key,viewport,proof,fn,fixture){
  const record={key,viewport,proof,status:'UNPROVEN',screenshots:[]};R.cases.push(record);let context,page;
  try{({context,page}=await make(viewport,key,fixture));record.observed=await fn(page,record);await capture(page,key,record);record.status='PASS';}
  catch(error){record.status='FAIL';record.reason=error.message;if(page)try{await capture(page,key+'-failure',record);}catch(e){record.captureError=e.message;}}
  finally{
    // Release all deliberate holds before closing context, including assertion failure.
    fixture?.releaseAll?.();if(context)await context.close();
  }
}
async function nativeJSON(page,url){return page.evaluate(async url=>{const response=await fetch(url);if(!response.ok)throw new Error('Native GET '+response.status);return response.json();},url);}
async function discover(){
  await run('native-template-prerequisite',PC,'NATIVE_BACKEND_GET',async page=>{
    await page.goto(ORIGIN+'/#/library?tab=templates',{waitUntil:'domcontentloaded'});
    const list=await nativeJSON(page,'/api/templates');
    const item=list.items.find(t=>t.id);if(!item)throw new Error('No native template: recovery proof cannot fabricate IDs');
    // Allow the selected ID before its real detail GET, without copying a recipe.
    baseline={id:item.id};baseline=await nativeJSON(page,targetFor(item.id));
    expect(baseline.recipe.scenes).toBeTruthy();expect(Number.isInteger(baseline.revision)).toBe(true);
    R.native={id:baseline.id,name:baseline.name,revision:baseline.revision,placement:placementOf(baseline.recipe)};
    return {id:baseline.id,revision:baseline.revision,nativeDetailRead:true};
  });
  if(!baseline?.recipe)throw new Error('Native discovery did not provide a usable real template recipe');
}
function placementOf(recipe){return recipe.scenes.style==='full'?0:recipe.scenes.framing==='close'?2:1;}
function recipeAt(index){const recipe=clone(baseline.recipe);recipe.scenes={...recipe.scenes,style:index===0?'full':'cutout',framing:index===2?'close':''};return recipe;}
const places = page => page.locator('.vt-places').getByRole('radio',{includeHidden:true});
async function choose(page,index){await places(page).nth(index).click();await expect(places(page).nth(index)).toBeChecked();}
async function editor(page){await page.goto(ORIGIN+routeFor(baseline.id),{waitUntil:'domcontentloaded'});await expect(places(page)).toHaveCount(3);await expect(places(page).nth(placementOf(baseline.recipe))).toBeChecked();}
async function rename(page,viewport,name){
  if(viewport.width<768){
    await page.getByRole('button',{name:'Thao tác với mẫu video',exact:true}).click();
    await page.getByRole('button',{name:'Đổi tên',exact:true}).click();
    await expect(modal(page)).toHaveCount(1);await page.locator('#mtpl-rename').fill(name);
    await page.getByRole('button',{name:'Lưu tên',exact:true}).click();
    await expect(page.locator('#mtpl-rename')).toBeDisabled();
  }else{const input=page.getByRole('textbox',{name:'Tên mẫu video',exact:true});await input.fill(name);await input.press('Tab');await expect(input).not.toBeFocused();}
  await settle(page);
}
async function checkName(page,viewport,name){await expect(viewport.width<768?page.locator('#mtpl-rename'):page.getByRole('textbox',{name:'Tên mẫu video',exact:true})).toHaveValue(name);}
async function closeRename(page,viewport){if(viewport.width<768){await page.keyboard.press('Escape');await expect(modal(page)).toHaveCount(0);}}
async function clean(page,viewport){
  if(viewport.width<768){await expect(page.locator('[role="status"]').filter({hasText:/Đã lưu/}).first()).toBeVisible();}
  else await expect(page.locator('.vt-saved')).toHaveAttribute('data-state','saved');
  await expect(page.getByRole('button',{name:'Tải lại',exact:true})).toHaveCount(0);
}
async function verifyAcceptedName(page,viewport,name){
  if(viewport.width<768){
    await expect(modal(page)).toHaveCount(0);await page.getByRole('button',{name:'Thao tác với mẫu video',exact:true}).click();await page.getByRole('button',{name:'Đổi tên',exact:true}).click();
    await expect(page.locator('#mtpl-rename')).toHaveValue(name);await closeRename(page,viewport);
  }else await checkName(page,viewport,name);
}
function patchFixture(mode){
  let current=clone(baseline),active=0,maxActive=0;
  const first=gate(),second=gate(),patches=[];
  const fixture={patches,first,second,get current(){return current;},get maxActive(){return maxActive;},releaseAll(){first.release();second.release();},
    async patch(route,entry){
      const body=route.request().postDataJSON();patches.push(clone(body));active++;maxActive=Math.max(active,maxActive);
      const number=patches.length;entry.synthetic={number,revision:body.revision,hasName:Object.hasOwn(body,'name'),hasRecipe:Object.hasOwn(body,'recipe'),placement:body.recipe?placementOf(body.recipe):null};
      let finished=false;
      // A response is no longer pending when handed to the browser. Do not
      // count the route.fulfill RPC acknowledgement as overlapping app work.
      const deliver=async action=>{finished=true;active--;return await action();};
      try{
        if(number===1){await first.promise;if(mode!=='accept'){
          entry.synthetic.outcome=mode;
          if(mode==='network')return await deliver(()=>route.abort('failed'));
          return await deliver(()=>json(route,{error:'SYNTHETIC_V38_SAVE_REJECT',code:mode==='409'?'conflict':'v38_synthetic_rejection'},Number(mode)));
        }}
        if(number===2&&mode==='accept')await second.promise;
        expect(body.revision,'serialized PATCH revision').toBe(current.revision);
        current={...current,...(body.name!==undefined?{name:body.name}:{}),...(body.recipe!==undefined?{recipe:clone(body.recipe)}:{}),revision:current.revision+1,preview:{...current.preview,state:'stale'}};
        entry.synthetic.outcome='ACCEPT_IN_MEMORY_ONLY';await deliver(()=>json(route,current));
      }finally{if(!finished)active--;}
    }
  };return fixture;
}
async function queuedEdits(page,viewport,fixture,name){
  await editor(page);const original=placementOf(baseline.recipe),first=(original+1)%3,latest=(original+2)%3;
  await choose(page,first);await expect.poll(()=>fixture.patches.length).toBe(1);
  await choose(page,latest);await rename(page,viewport,name);
  expect(fixture.patches.length,'queued edits must not send concurrent PATCH').toBe(1);
  expect(fixture.patches[0].recipe).toEqual(recipeAt(first));
  return {original,first,latest,recipe:recipeAt(latest)};
}
async function failedRetry(viewport,mode){
  const key=`synthetic-${viewport.width}-failed-${mode}-merged-retry`,fixture=patchFixture(mode);
  await run(key,viewport,'SYNTHETIC_PATCH_REJECTION_AND_ACCEPTANCE_NO_SERVER_WRITE',async(page,record)=>{
    const name=`V38 recovery ${viewport.width} ${mode}`,edit=await queuedEdits(page,viewport,fixture,name);
    fixture.first.release();
    if(mode==='network'){
      if(viewport.width<768)await expect(page.locator('#mtpl-rename')).toBeEnabled();
      else await expect(page.locator('.vt-saved')).toHaveAttribute('data-state','error');
    }else await expect(page.getByText('SYNTHETIC_V38_SAVE_REJECT',{exact:true}).first()).toBeVisible();
    await checkName(page,viewport,name);if(viewport.width<768)await expect(page.locator('#mtpl-rename')).toBeEnabled();
    expect(fixture.patches.length).toBe(1);await capture(page,key+'-unsaved',record);await closeRename(page,viewport);
    await expect(places(page).nth(edit.latest)).toBeChecked();
    if(viewport.width<768)await expect(page.getByRole('status').filter({hasText:'Chưa lưu được. Thử lại.'}).first()).toBeVisible();
    else await expect(page.locator('.vt-saved')).toHaveAttribute('data-state','error');
    await page.getByRole('button',{name:'Thử lại',exact:true}).click();await expect.poll(()=>fixture.patches.length).toBe(2);await clean(page,viewport);
    expect(fixture.patches[1]).toEqual({revision:baseline.revision,name,recipe:edit.recipe});
    expect(fixture.current.name).toBe(name);expect(fixture.current.recipe).toEqual(edit.recipe);
    await expect(places(page).nth(edit.latest)).toBeChecked();await verifyAcceptedName(page,viewport,name);
    expect(fixture.maxActive).toBe(1);
    return {nativeId:baseline.id,rejection:mode,patches:2,failedDraftMerged:true,nameAndEntireRecipeRetained:true,retryRevision:fixture.patches[1].revision,maxActive:fixture.maxActive,backendPersistence:'UNPROVEN_SYNTHETIC_ONLY'};
  },fixture);
}
async function serialized(viewport){
  const key=`synthetic-${viewport.width}-serialized-newest-draft`,fixture=patchFixture('accept');
  await run(key,viewport,'SYNTHETIC_SERIALIZED_PATCH_ACCEPTANCE_NO_SERVER_WRITE',async(page,record)=>{
    let name=`V38 newest ${viewport.width}`;const edit=await queuedEdits(page,viewport,fixture,name);
    if(viewport.width>=768){name+=' final';await rename(page,viewport,name);expect(fixture.patches.length).toBe(1);}
    await capture(page,key+'-first-pending',record);fixture.first.release();
    await expect.poll(()=>fixture.patches.length).toBe(2);
    expect(fixture.patches[1]).toEqual({revision:baseline.revision+1,name,recipe:edit.recipe});
    // Second response is deliberately held. The older first response must not
    // overwrite newer visible recipe/name while the queue drains.
    await checkName(page,viewport,name);
    if(viewport.width<768){await expect(page.locator('#mtpl-rename')).toBeDisabled();}
    await expect(places(page).nth(edit.latest)).toBeChecked();
    await capture(page,key+'-second-pending',record);fixture.second.release();await clean(page,viewport);
    await expect(places(page).nth(edit.latest)).toBeChecked();await verifyAcceptedName(page,viewport,name);
    expect(fixture.current.recipe).toEqual(edit.recipe);expect(fixture.current.name).toBe(name);expect(fixture.maxActive).toBe(1);expect(fixture.patches.length).toBe(2);
    return {patches:2,maxActive:fixture.maxActive,revisions:fixture.patches.map(p=>p.revision),latestRecipeAndNameVisible:true,queuedCoalescing:true,backendPersistence:'UNPROVEN_SYNTHETIC_ONLY'};
  },fixture);
}
async function nativeReload(page,record,fixture){
  const before=R.requests.filter(e=>e.case===record.key&&e.action==='NATIVE_GET'&&e.path===targetFor(baseline.id)).length;
  const responsePromise=page.waitForResponse(r=>new URL(r.url()).pathname===targetFor(baseline.id)&&r.request().method()==='GET'&&r.status()===200);
  await page.getByRole('button',{name:'Tải lại',exact:true}).filter({visible:true}).last().click();
  const response=await responsePromise,fresh=await response.json();
  expect(fresh.id).toBe(baseline.id);expect(fresh.recipe).toBeTruthy();
  const after=R.requests.filter(e=>e.case===record.key&&e.action==='NATIVE_GET'&&e.path===targetFor(baseline.id)).length;
  expect(after,'reload must issue actual native detail GET').toBeGreaterThan(before);
  // Continue synthetic acceptance from the *actual recovery* revision.
  fixture.nativeFresh=fresh;return fresh;
}
async function conflictReload(viewport,failFirst){
  const key=`synthetic-${viewport.width}-conflict-${failFirst?'failed-then-native':'native'}-reload`,fixture=patchFixture('409');
  let failReload=failFirst,reloadFailures=0;
  fixture.get=async(route,entry)=>{if(failReload&&fixture.patches.length){reloadFailures++;entry.synthetic={outcome:'RELOAD_GET_500'};await json(route,{error:'SYNTHETIC_V38_RELOAD_REJECT',code:'v38_synthetic_reload'},500);return true;}return false;};
  await run(key,viewport,'SYNTHETIC_CONFLICT_THEN_NATIVE_GET_RECOVERY',async(page,record)=>{
    const name=`V38 conflict ${viewport.width}`,edit=await queuedEdits(page,viewport,fixture,name);fixture.first.release();
    await expect(page.getByRole('button',{name:'Tải lại',exact:true}).filter({visible:true}).last()).toBeVisible();await checkName(page,viewport,name);
    if(viewport.width<768)await expect(page.getByRole('button',{name:'Lưu tên',exact:true})).toBeDisabled();
    else await expect(page.locator('.vt-saved')).toHaveAttribute('data-state','conflict');
    await expect(places(page).nth(edit.latest)).toBeChecked();
    await capture(page,key+'-conflict-kept',record);
    if(failFirst){
      await page.getByRole('button',{name:'Tải lại',exact:true}).filter({visible:true}).last().click();
      await expect(page.getByText('SYNTHETIC_V38_RELOAD_REJECT',{exact:true}).first()).toBeVisible();expect(reloadFailures).toBeGreaterThan(0);
      await checkName(page,viewport,name);expect(fixture.patches.length,'failed reload must not replay PATCH').toBe(1);
      if(viewport.width<768){await expect(page.locator('#mtpl-rename')).toBeEnabled();await expect(page.getByRole('button',{name:'Lưu tên',exact:true})).toBeDisabled();}
      await expect(places(page).nth(edit.latest)).toBeChecked();
      await capture(page,key+'-reload-failed-kept',record);
    }
    failReload=false;const fresh=await nativeReload(page,record,fixture);
    await checkName(page,viewport,fresh.name);if(viewport.width<768){await expect(page.getByRole('button',{name:'Lưu tên',exact:true})).toBeEnabled();await closeRename(page,viewport);}
    await expect(places(page).nth(placementOf(fresh.recipe))).toBeChecked();await clean(page,viewport);
    expect(fixture.patches.length,'native recovery must discard, not replay, conflict queue').toBe(1);
    const post=(placementOf(fresh.recipe)+1)%3;
    // New edit after recovery must be accepted at the native revision and must
    // not leak the old pending name/recipe into the next request.
    fixture.patch=async(route,entry)=>{
      const body=route.request().postDataJSON();fixture.patches.push(clone(body));entry.synthetic={outcome:'POST_RELOAD_ACCEPT_IN_MEMORY_ONLY',revision:body.revision};
      expect(body.revision).toBe(fresh.revision);expect(Object.hasOwn(body,'name')).toBe(false);
      const saved={...fresh,recipe:body.recipe,revision:fresh.revision+1,preview:{...fresh.preview,state:'stale'}};await json(route,saved);
    };
    await choose(page,post);await expect.poll(()=>fixture.patches.length).toBe(2);await clean(page,viewport);
    const expected=clone(fresh.recipe);expected.scenes={...expected.scenes,style:post===0?'full':'cutout',framing:post===2?'close':''};expect(fixture.patches[1].recipe).toEqual(expected);
    await verifyAcceptedName(page,viewport,fresh.name);
    return {rejection:'409 conflict',failedReloadExercised:failFirst,reloadFailures,actualRecoveryGET:true,recoveredNativeRevision:fresh.revision,discardedConflictQueue:true,postRecoveryEditUnlocked:true,noStaleNameLeak:true,backendWrite:false};
  },fixture);
}
async function detailFailure(viewport,kind){
  const key=`synthetic-${viewport.width}-detail-${kind}-${kind==='404'?'notfound-navigation':'native-recovery'}`;let reject=true,hits=0;
  const marker='V38_INTERNAL_DETAIL_DO_NOT_EXPOSE',code='v38_private_detail_code';
  const fixture={get:async(route,entry)=>{hits++;if(!reject)return false;entry.synthetic={outcome:kind};if(kind==='network')await route.abort('failed');else await json(route,{error:marker,code},Number(kind));return true;}};
  await run(key,viewport,kind==='404'?'SYNTHETIC_DETAIL_404_DISTINCT_FROM_TRANSIENT':'SYNTHETIC_DETAIL_GET_FAILURE_THEN_NATIVE_RESOURCE_REFRESH',async(page,record)=>{
    await page.goto(ORIGIN+routeFor(baseline.id),{waitUntil:'domcontentloaded'});
    const retry=page.getByRole('button',{name:'Thử lại',exact:true});
    if(kind==='404'){
      await expect(page.getByText('Không tìm thấy mẫu video này. Có thể nó đã bị xoá.',{exact:true})).toBeVisible();
      await expect(retry).toHaveCount(0);await expect(page.getByRole('button',{name:'Về Thư viện',exact:true})).toBeVisible();
      await expect(page.locator('body')).not.toContainText(marker);await expect(page.locator('body')).not.toContainText(code);
      await expect(places(page)).toHaveCount(0);await capture(page,key+'-not-found',record);
      await page.getByRole('button',{name:'Về Thư viện',exact:true}).click();await expect(page).toHaveURL(/#\/library\?tab=templates$/);
      return {syntheticStatus:404,notFoundDistinct:true,noRetry:true,backNavigation:true,rawInternalMessageAbsent:true};
    }
    await expect(retry).toBeVisible();await expect(places(page)).toHaveCount(0);
    // Record generic/raw-message assertions without stopping recovery proof.
    const issues=[];
    try{await expect(page.getByText('Không tải được mẫu video. Thử lại.',{exact:true})).toBeVisible();}catch{issues.push('Generic Vietnamese loadFail absent');}
    const body=await page.locator('body').innerText();if(body.includes(marker)||body.includes(code)||body.includes('ApiError:'))issues.push('Raw internal detail error exposed');
    await expect(page.getByText('Không tìm thấy mẫu video này. Có thể nó đã bị xoá.',{exact:true})).toHaveCount(0);
    await capture(page,key+'-transient-error',record);const before=hits;reject=false;
    const responsePromise=page.waitForResponse(r=>new URL(r.url()).pathname===targetFor(baseline.id)&&r.request().method()==='GET'&&r.status()===200);
    // This click exercises the existing useResource.refresh path. No reload,
    // direct injected fetch, location reassignment or synthetic recovery GET.
    await retry.click();const response=await responsePromise,fresh=await response.json();
    expect(hits).toBeGreaterThan(before);expect(fresh.id).toBe(baseline.id);
    await expect(places(page).nth(placementOf(fresh.recipe))).toBeChecked();
    if(viewport.width>=768)await checkName(page,viewport,fresh.name);
    else await verifyAcceptedName(page,viewport,fresh.name);
    record.recovery={actualNativeDetailGET:true,revision:fresh.revision,retryHits:hits-before,editorRestored:true};
    expect(issues,'detail error classification/privacy').toEqual([]);
    return {injected:kind,...record.recovery,notFoundAbsent:true,genericMessage:true,rawInternalMessageAbsent:true};
  },fixture);
}
async function reloadEditGuard(viewport){
  const fixture=patchFixture('409'),hold=gate();
  const originalRelease=fixture.releaseAll;fixture.releaseAll=()=>{originalRelease();hold.release();};
  let entered=false;
  fixture.get=async(route)=>{if(fixture.patches.length){entered=true;await hold.promise;await json(route,baseline);return true;}return false;};
  await run(`review-${viewport.width}-reload-edit-guard`,viewport,'SYNTHETIC_CONFLICT_AND_HELD_RELOAD_GET',async(page,record)=>{
    await queuedEdits(page,viewport,fixture,'V38 reload cancelled draft');fixture.first.release();
    await expect(page.getByRole('button',{name:'Tải lại',exact:true}).filter({visible:true}).last()).toBeVisible();
    await closeRename(page,viewport);
    await page.getByRole('button',{name:'Tải lại',exact:true}).filter({visible:true}).last().click();
    await expect.poll(()=>entered).toBe(true);
    await expect(places(page).first()).toBeDisabled();
    if(viewport.width>=768)await expect(page.getByRole('textbox',{name:'Tên mẫu video',exact:true})).toBeDisabled();
    await capture(page,record.key+'-pending',record);hold.release();
    await expect(places(page).first()).toBeEnabled();await expect(places(page).nth(placementOf(baseline.recipe))).toBeChecked();
    return {editingDisabledDuringReload:true,reloadDiscardsOnlyPreviouslyConfirmedDraft:true};
  },fixture);
}
async function escapeNameGuard(){
  const fixture=patchFixture('accept');fixture.first.release();fixture.second.release();
  await run('review-pc-name-escape',PC,'NATIVE_GET_SYNTHETIC_PATCH_FENCE',async page=>{
    await editor(page);const input=page.getByRole('textbox',{name:'Tên mẫu video',exact:true});
    await input.fill('V38 MUST NOT SAVE CANCELLED NAME');await input.press('Escape');await settle(page);
    await expect(input).toHaveValue(baseline.name);expect(fixture.patches).toEqual([]);
    return {cancelledNameNotSubmitted:true};
  },fixture);
}
async function dirtyBuildGuard(){
  const fixture=patchFixture('400');fixture.first.release();
  await run('review-pc-unsaved-build-guard',PC,'SYNTHETIC_SAVE_FAILURE_AND_READY_SOURCE_GET',async page=>{
    await page.route('**/api/templates/'+enc(baseline.id)+'/preview',route=>json(route,{state:'none',builtAt:null,minutes:3,sources:[{jobId:'v38-explicit-synthetic-review-source',title:'SYNTHETIC source for eligibility only',durationSec:180}]}));
    await editor(page);await rename(page,PC,'V38 failed draft');await expect(page.locator('.vt-saved')).toHaveAttribute('data-state','error');
    await expect(page.getByRole('button',{name:'Dựng lại bản xem thử',exact:true})).toHaveAttribute('aria-disabled','true');
    return {unsavedBuildLocked:true,buildNeverSubmitted:true};
  },fixture);
}
(async()=>{
  await fs.mkdir(ROOT,{recursive:true});
  try{
    cookies=JSON.parse(await fs.readFile(COOKIES,'utf8'));browser=await chromium.launch({executablePath:EXEC,headless:true});await discover();
    if(process.argv.includes('--review-probes')){
      await reloadEditGuard(PC);await reloadEditGuard(PHONE);await escapeNameGuard();await dirtyBuildGuard();
    }else for(const viewport of [PC,PHONE]){
      for(const mode of ['500','400','network'])await failedRetry(viewport,mode);
      await serialized(viewport);await conflictReload(viewport,false);await conflictReload(viewport,true);
      for(const kind of ['500','network','404'])await detailFailure(viewport,kind);
    }
    R.status=R.cases.some(c=>c.status!=='PASS')||R.pageErrors.length||R.safety.downloads||R.safety.unexpectedMutations||R.safety.routeErrors.length?'DONE_WITH_CONCERNS':'DONE';
  }catch(error){R.status='BLOCKED';R.fatal=error.message;process.exitCode=1;}
  finally{
    try{if(browser)await browser.close();R.browserClosed=true;}catch(error){R.browserCloseError=error.message;R.status='DONE_WITH_CONCERNS';process.exitCode=1;}
    R.safety.receipt={continuedNonGET:R.safety.nonGETContinued,continuedExternal:R.safety.externalContinued,syntheticPATCH:R.requests.filter(e=>e.action==='SYNTHETIC_PATCH').length,nativeDetailGET:R.requests.filter(e=>e.action==='NATIVE_GET'&&baseline&&e.path===targetFor(baseline.id)).length,blockedMutations:R.requests.filter(e=>e.action==='BLOCK_MUTATION').length,blockedExternal:R.requests.filter(e=>e.action==='BLOCK_EXTERNAL').length,blockedDownloadsMedia:R.requests.filter(e=>e.action==='BLOCK_DOWNLOAD_MEDIA_DISCOVERY').length,blockedUnlistedAPI:R.requests.filter(e=>e.action==='BLOCK_UNLISTED_API_GET').length};
    R.finishedAt=new Date().toISOString();await fs.writeFile(path.join(ROOT,process.argv.includes('--review-probes')?'review-probe-results.json':'results.json'),JSON.stringify(R,null,2));
  }
  const counts=Object.fromEntries(['PASS','FAIL','UNPROVEN'].map(s=>[s,R.cases.filter(c=>c.status===s).length]));
  console.log(JSON.stringify({status:R.status,counts,safety:R.safety.receipt,results:path.join(ROOT,'results.json')}));
  if(counts.FAIL||R.pageErrors.length||R.safety.downloads||R.safety.unexpectedMutations||R.safety.routeErrors.length)process.exitCode=1;
})().catch(error=>{console.error(error.message);process.exitCode=1;});
