'use strict';
// PREPARED ONLY. Parent runs sequentially against the guarded current 8043 build.
// Synthetic GET/PATCH consumers only. Build/trial/cancel requests are ABORTED,
// never fulfilled as successful generation/cancellation and never sent upstream.
const {createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs=require('node:fs/promises');
const path=require('node:path');
const ROOT=__dirname, ORIGIN='http://127.0.0.1:8043';
const COOKIES='/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const EXEC='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PC={width:1440,height:900}, PHONE={width:390,height:844};
const R={status:'NOT_RUN',preparedOnly:false,cases:[],requests:[],screenshots:[],pageErrors:[],limitations:[
  'All synthetic GET/PATCH fixtures exercise actual mounted components, not generated DOM. PATCH acceptance is memory-only and is not backend persistence.',
  'Preview fresh/building/stale are synthetic metadata without a projectId or fabricated media URL. No native generation, player playback, rendered video, paid trial, provider request or download is accepted.',
  'Cancel/rebuild requests are deliberately aborted before networking: only rejected-action feedback/retry and separate synthetic GET polling transitions are proven. No successful cancellation is claimed.',
  'Canonical import done/partial, named backdrop identities without native records, native import success, external source discovery and mobile viewer round45 remain uncaptured. See coverage-map.json.',
  'Screenshots are state evidence at 1440x900/390x844, not global pixel parity or visual acceptance. Parent must compare each highest canonical board separately.'
],safety:{nonGETContinued:0,externalContinued:0,downloads:0,websocketsBlocked:0,routeErrors:[]}};
let browser,cookies,native;
const clone=x=>JSON.parse(JSON.stringify(x)), enc=encodeURIComponent;
const target=()=>'/api/templates/'+enc(native.detail.id);
const template=(section,home=false)=>'/#/templates/'+enc(native.detail.id)+(section&&!home?'?section='+section:'');
const libraryPath=(tab,query='')=>'/#/library?tab='+enc(tab)+(query?'&'+query:'');
const json=(route,body,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
const modal=p=>p.locator('[aria-modal="true"]:visible');
const refs=(width,pc,mobile)=>width<768?mobile:pc;
function unsupported(reason){const e=new Error(reason);e.unproven=true;throw e;}
function gate(){let release;const promise=new Promise(r=>{release=r;});return {promise,release};}
async function settle(p){await p.evaluate(async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));});}
async function capture(p,key,record){
  await settle(p);await expect(modal(p),'single interactive dialog layer').toHaveCount(await modal(p).count()?1:0);
  const geometry=await p.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth}));
  expect(geometry.scrollWidth,'no horizontal document overflow').toBeLessThanOrEqual(geometry.width+1);
  const file=key+'.png';await p.screenshot({path:path.join(ROOT,file)});
  record.screenshots.push(file);R.screenshots.push({case:record.key,referenceIds:record.referenceIds,file,geometry});
}
function nativeAPI(p){
  if(/^\/api\/(?:me|events|activity|notifications\/settings|queue|status|owner-channels|channels\/summary|setup\/state|upload-limits|platforms|templates|image-prompts|overlays|backdrop\/channels|backdrop\/videos)$/.test(p))return true;
  if(native&&[target(),target()+'/preview',target()+'/cover-trials',target()+'/backdrop/mix'].includes(p))return true;
  if(native?.coverStatusPaths.has(p))return true;
  if(native?.samplePaths.has(p))return true;
  return /^\/api\/image-prompts\/[^/]+\/samples$/.test(p);
}
async function make(viewport,key,fixture){
  const context=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
  await context.addCookies(cookies.map(c=>({...c,domain:'127.0.0.1',secure:false})));
  if(context.routeWebSocket)await context.routeWebSocket('**/*',socket=>{R.safety.websocketsBlocked++;socket.close();});
  await context.route('**/*',async route=>{
    const req=route.request(),u=new URL(req.url());
    const entry={case:key,method:req.method(),path:u.pathname+u.search};R.requests.push(entry);
    try{
      if(u.origin!==ORIGIN){entry.action='BLOCK_EXTERNAL';return await route.abort('blockedbyclient');}
      if(req.method()!=='GET'){
        if(req.method()==='PATCH'&&native&&u.pathname===target()&&fixture?.patch){entry.action='SYNTHETIC_PATCH_MEMORY_ONLY';return await fixture.patch(route,entry);}
        entry.action='ABORT_NON_GET_BEFORE_NETWORK';
        // Callback may hold an explicitly named request for pending-state proof,
        // but cannot continue/fetch/fulfill it. The sole final operation is abort.
        if(fixture?.blocked)await fixture.blocked(entry,req);
        return await route.abort('failed');
      }
      if(/(?:^|\/)(?:export|download|downloads|discover|discovery|follow|parse)(?:\/|$)/i.test(u.pathname)||req.resourceType()==='media'||/\.(?:zip|ytf|mp4|webm|mp3|wav|m3u8|ts)$/i.test(u.pathname)){entry.action='BLOCK_DOWNLOAD_MEDIA_DISCOVERY';return await route.abort('blockedbyclient');}
      if(fixture?.get&&await fixture.get(route,u,entry)){entry.action='SYNTHETIC_GET';return;}
      if(u.pathname.startsWith('/api/')&&!nativeAPI(u.pathname)){entry.action='BLOCK_UNLISTED_API_GET';return await route.abort('blockedbyclient');}
      entry.action=u.pathname.startsWith('/api/')?'NATIVE_GET':'LOCAL_STATIC_GET';
      if(req.method()!=='GET')R.safety.nonGETContinued++;
      if(u.origin!==ORIGIN)R.safety.externalContinued++;
      return await route.continue();
    }catch(e){entry.routeError=e.message;R.safety.routeErrors.push({case:key,message:e.message});try{await route.abort('failed');}catch{}}
  });
  const p=await context.newPage();p.setDefaultTimeout(10000);p.setDefaultNavigationTimeout(20000);
  p.on('pageerror',e=>R.pageErrors.push({case:key,message:e.message}));
  p.on('download',d=>{R.safety.downloads++;void d.cancel();});p.on('popup',p=>void p.close());
  return {context,p};
}
async function run(key,viewport,referenceIds,fn,fixture){
  const record={key,viewport,referenceIds,proof:'SYNTHETIC_UI_CONSUMER_UNLESS_NOTED',status:'NOT_RUN',screenshots:[]};R.cases.push(record);
  let context,p;
  try{({context,p}=await make(viewport,key,fixture));record.observed=await fn(p,record);await capture(p,key,record);record.status='PASS';}
  catch(e){record.status=e.unproven?'UNPROVEN':'FAIL';record.reason=e.message;if(p)try{await capture(p,key+'-failure',record);}catch(s){record.captureError=s.message;}}
  finally{fixture?.releaseAll?.();if(context)await context.close();}
}
async function go(p,route){await p.goto(ORIGIN+route,{waitUntil:'domcontentloaded'});}
async function readJSON(p,url){return p.evaluate(async url=>{const response=await fetch(url);if(!response.ok)throw new Error('Native prerequisite GET '+response.status);return response.json();},url);}
async function oneModal(p){await expect(modal(p)).toHaveCount(1);return modal(p);}
async function discoverCoverMedia(p){
  const discovery={reads:[],candidates:[],selected:null};
  native.coverMedia=null;R.native.coverMediaDiscovery=discovery;
  const candidates=[];
  const add=(candidate)=>{
    if(!candidate.imageUrl)return;
    const u=new URL(candidate.imageUrl,ORIGIN);
    if(u.origin!==ORIGIN)return;
    if(candidate.expectedPath&&decodeURIComponent(u.pathname)!==decodeURIComponent(candidate.expectedPath))return;
    native.samplePaths.add(u.pathname);candidates.push({...candidate,imageUrl:u.pathname+u.search});
  };
  const readStatus=async url=>{
    try{const value=await readJSON(p,url);expect(Array.isArray(value.history),'native history GET shape: '+url).toBe(true);discovery.reads.push({path:url,status:'READ',historyCount:value.history.length});return value;}
    catch(e){discovery.reads.push({path:url,status:'UNAVAILABLE',reason:e.message});return null;}
  };
  // These are existing-record GETs only. Never start a trial to fill a gap.
  for(const item of native.templates){
    const url='/api/templates/'+enc(item.id)+'/cover-trials',status=await readStatus(url);
    for(const row of status?.history??[])if(row.state==='ready')add({kind:'cover-trial',readPath:url,templateId:item.id,row:clone(row),imageUrl:row.imageUrl,expectedPath:url+'/'+enc(row.id)+'/image'});
  }
  for(const prompt of native.prompts.filter(x=>x.kind==='cover')){
    const url='/api/image-prompts/'+enc(prompt.id)+'/samples',status=await readStatus(url);
    for(const row of status?.history??[])if(row.state==='ready'&&row.promptKind==='cover')add({kind:'prompt-sample',readPath:url,prompt:clone(prompt),row:clone(row),imageUrl:row.imageUrl,expectedPath:url+'/'+enc(row.id)+'/image'});
    for(const imageUrl of prompt.sampleUrls??[])add({kind:'prompt-artwork',readPath:'/api/image-prompts',prompt:clone(prompt),imageUrl});
  }
  for(const candidate of candidates){
    const loaded=await p.evaluate(url=>new Promise(resolve=>{
      const image=new Image(),timer=setTimeout(()=>{image.src='';resolve(false);},10000);
      image.onload=()=>{clearTimeout(timer);resolve(image.naturalWidth>0&&image.naturalHeight>0);};
      image.onerror=()=>{clearTimeout(timer);resolve(false);};image.src=url;
    }),candidate.imageUrl);
    discovery.candidates.push({kind:candidate.kind,readPath:candidate.readPath,imageUrl:candidate.imageUrl,decoded:loaded});
    if(loaded){native.coverMedia=candidate;discovery.selected={kind:candidate.kind,readPath:candidate.readPath,imageUrl:candidate.imageUrl,recordId:candidate.row?.id??candidate.prompt.id};break;}
  }
}
async function discover(){
  await run('native-prerequisites',PC,[],async(p,record)=>{
    record.proof='NATIVE_BACKEND_GET';await go(p,'/#/library?tab=templates');
    const [templates,prompts,overlays,owners]=await Promise.all(['/api/templates','/api/image-prompts','/api/overlays','/api/owner-channels'].map(url=>readJSON(p,url)));
    const item=templates.items.find(t=>t.id);if(!item)unsupported('Native template prerequisite missing; controller never creates one');
    native={detail:{id:item.id},templates:templates.items,prompts:prompts.items,overlays:overlays.items,owners:owners.channels,samplePaths:new Set(),coverStatusPaths:new Set(templates.items.map(x=>'/api/templates/'+enc(x.id)+'/cover-trials'))};
    for(const prompt of native.prompts)for(const url of prompt.sampleUrls??[]){const u=new URL(url,ORIGIN);if(u.origin===ORIGIN)native.samplePaths.add(u.pathname);}
    native.detail=await readJSON(p,target());expect(native.detail.recipe.scenes).toBeTruthy();
    R.native={templateId:item.id,revision:native.detail.revision,promptCount:native.prompts.length,ownerCount:native.owners.length,sameOriginSampleCount:native.samplePaths.size};
    await discoverCoverMedia(p);
    return R.native;
  });
  if(!native?.detail?.recipe)throw new Error('Native baseline unavailable: all reachable fixtures require the real recipe schema');
}
function fixture(options={}){
  const f={detail:clone(native.detail),preview:{state:'fresh',builtAt:null,minutes:3,templateRevision:native.detail.revision,sources:[]},patches:[],...options};
  // Always remove project URLs: no synthetic preview is represented as rendered.
  f.detail.preview={state:f.preview.state,builtAt:f.preview.builtAt??null,minutes:3};
  f.get=async(route,u,entry)=>{
    if(u.pathname===target()){await json(route,f.detail);return true;}
    if(u.pathname===target()+'/preview'){await json(route,f.preview);return true;}
    if(f.extraGET&&await f.extraGET(route,u,entry))return true;
    return false;
  };
  f.patch=async(route,entry)=>{
    const body=route.request().postDataJSON();expect(body.revision).toBe(f.detail.revision);f.patches.push(clone(body));
    f.detail={...f.detail,...(body.name!==undefined?{name:body.name}:{}),...(body.recipe?{recipe:clone(body.recipe)}:{}),revision:f.detail.revision+1,preview:{state:'stale',builtAt:null,minutes:3}};
    // Denormalized labels are derived only from existing prompt records.
    if(body.recipe){const scene=native.prompts.find(p=>p.id===body.recipe.scenes.scenePromptId),cover=native.prompts.find(p=>p.id===body.recipe.scenes.coverPromptId);f.detail.prompts={...f.detail.prompts,...(scene?{sceneId:scene.id,sceneName:scene.name}:{}),...(cover?{coverId:cover.id,coverName:cover.name}:{})};}
    entry.synthetic={revision:f.detail.revision,fields:Object.keys(body),persistence:'MEMORY_ONLY'};await json(route,f.detail);
  };
  return f;
}
const source={jobId:'v38-synthetic-source',title:'SYNTHETIC completed source (no media)',channelName:'SYNTHETIC channel',durationSec:181};
async function previewCases(viewport){
  const width=viewport.width,board=width<768?'MTemplate':'Template';
  // Fresh is badge metadata only; canonical rendered-project parity is unproven.
  const fresh=fixture({preview:{state:'fresh',builtAt:'2026-11-04T00:00:00Z',templateRevision:native.detail.revision,minutes:3,sources:[]}});
  await run(`synthetic-${width}-preview-fresh-locked`,viewport,[board+'.preview.fresh',...(width<768?['MTemplate.screen.list','MTemplate.sheet.none']:['Template.picker.none'])],async(p,r)=>{
    await go(p,template(undefined,true));await expect(p.getByText('Khớp cài đặt',{exact:true}).first()).toBeVisible();
    await expect(p.getByText('Chưa có video nào đã làm xong',{exact:true})).toBeVisible();
    const rebuild=p.getByRole('button',{name:/^Dựng lại/});await expect(rebuild).toBeDisabled();
    await expect(p.locator('video')).toHaveCount(0);
    return {state:'fresh',noSourceLocked:true,noProjectOrRenderedPreview:true,nativeGeneration:'UNPROVEN'};
  },fresh);
  const stale=fixture({preview:{state:'stale',builtAt:null,minutes:3,templateRevision:native.detail.revision-1,sources:[source],error:{code:'v38_synthetic',message:'SYNTHETIC preview failed',recovery:'SYNTHETIC retry from the selected source'}}});let starts=0;
  stale.blocked=async(entry,req)=>{if(entry.method==='POST'&&entry.path===target()+'/preview'){starts++;expect(req.postDataJSON()).toEqual({jobId:source.jobId,minutes:3});}};
  await run(`synthetic-${width}-preview-stale-error-retry`,viewport,[board+'.preview.stale'],async(p,r)=>{
    await go(p,template(undefined,true));await expect(p.getByText(width<768?'Cài đặt đã đổi · dựng lại để xem':'Cần dựng lại',{exact:true}).first()).toBeVisible();
    await expect(p.getByRole('alert').filter({hasText:'SYNTHETIC preview failed'})).toContainText('SYNTHETIC retry from the selected source');await capture(p,r.key+'-server-error',r);
    const build=p.getByRole('button',{name:'Dựng lại bản xem thử',exact:true});await expect(build).toBeEnabled();await build.click();await expect.poll(()=>starts).toBe(1);
    await expect(p.getByRole('alert').filter({hasText:'SYNTHETIC preview failed'})).toHaveCount(0);await expect(p.getByRole('alert').first()).toBeVisible();
    await build.click();await expect.poll(()=>starts).toBe(2);return {state:'stale',selectedJob:source.jobId,rejectedStarts:starts,providerOrBuildStarted:false,errorReplacedByActionFailure:true};
  },stale);
  let cancels=0;const building=fixture({preview:{state:'building',builtAt:null,minutes:3,sources:[source],task:{id:'v38-synthetic-task',state:'running',phase:'SYNTHETIC preparing preview',done:2,total:5}}});
  building.blocked=async entry=>{if(entry.method==='DELETE'&&entry.path===target()+'/preview')cancels++;};
  await run(`synthetic-${width}-preview-building-cancel-rejection`,viewport,[board+'.preview.building'],async(p,r)=>{
    await go(p,template(undefined,true));await expect(p.getByRole('status').filter({hasText:'SYNTHETIC preparing preview'})).toBeVisible();
    await expect(p.locator(width<768?'#mtpl-preview-source':'#vt-preview-source')).toBeDisabled();
    await expect(p.getByRole('button',{name:/^Dựng lại/})).toHaveCount(0);await capture(p,r.key+'-building',r);
    const cancel=p.getByRole('button',{name:'Huỷ dựng',exact:true});await cancel.click();await expect.poll(()=>cancels).toBe(1);await expect(p.getByRole('alert').first()).toBeVisible();
    await cancel.click();await expect.poll(()=>cancels).toBe(2);await capture(p,r.key+'-cancel-rejected',r);
    building.preview={state:'stale',builtAt:null,minutes:3,sources:[source]};
    await expect(p.getByRole('button',{name:'Dựng lại bản xem thử',exact:true})).toBeEnabled();await expect(cancel).toHaveCount(0);
    return {buildingProgress:'2/5',sourceLockedWhileBuilding:true,rejectedCancelAttempts:cancels,separateSyntheticPollingTransition:'building to stale',successfulCancellation:'UNPROVEN'};
  },building);
}
async function imageCases(viewport){
  const width=viewport.width,board=width<768?'MTemplate':'Template',f=fixture();
  await run(`synthetic-${width}-images-composition-and-prompt`,viewport,[board+(width<768?'.screen.images':'.section.images')],async(p,r)=>{
    await go(p,template('images'));const places=p.locator('.vt-places').getByRole('radio',{includeHidden:true});await expect(places).toHaveCount(3);
    for(let i=0;i<3;i++){
      const image=p.locator('.vt-places img').nth(i);await expect(image).toHaveAttribute('src','/illustrations/scene-composition-'+['full','cutout','close'][i]+'.jpg');await expect.poll(()=>image.evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);
      const before=f.patches.length;await places.nth(i).click();await expect(places.nth(i)).toBeChecked();await expect.poll(()=>f.patches.length).toBe(before+1);
      expect(f.patches.at(-1).recipe.scenes.style).toBe(i===0?'full':'cutout');expect(f.patches.at(-1).recipe.scenes.framing).toBe(i===2?'close':'');await capture(p,r.key+'-'+['full','cutout','close'][i],r);
    }
    const candidates=native.prompts.filter(x=>x.kind==='scene'),different=candidates.find(x=>x.id!==f.detail.recipe.scenes.scenePromptId);if(!different)unsupported('No second native scene prompt for exact selection assertion');
    await p.getByRole('radio',{name:different.name,exact:true}).click();await expect.poll(()=>f.detail.recipe.scenes.scenePromptId).toBe(different.id);
    await expect(p.getByRole('radio',{name:different.name,exact:true})).toHaveAttribute('aria-checked','true');
    const selected=p.locator('.vt-pick-detail');await expect(selected).toContainText(different.prompt);await expect(selected.locator('a')).toHaveAttribute('href',`#/library?tab=prompts&prompt=${different.builtIn?'builtin':'mine'}&promptId=${enc(different.id)}`);
    return {placements:['full','cutout','close'],staticIllustrationsOnly:true,selectedPromptId:different.id,patchAcceptance:'MEMORY_ONLY',generation:false};
  },f);
  // Count/motion changes use their actual controls, preserve all flat fields.
  const counts=fixture();
  await run(`synthetic-${width}-images-count-motion`,viewport,[],async(p,r)=>{
    await go(p,template('images'));
    const mode=p.locator('.vt-card').filter({has:p.locator('#vt-count')});
    for(const [index,value] of [[1,'per-minutes'],[2,'none'],[0,'fixed']]){
      const before=counts.patches.length;await mode.getByRole('radio').nth(index).click();await expect.poll(()=>counts.patches.length).toBe(before+1);expect(counts.detail.recipe.scenes.images.mode).toBe(value);
      await expect(mode.getByRole('spinbutton')).toHaveCount(value==='none'?0:1);await capture(p,r.key+'-'+value,r);
    }
    // Copy can evolve; resolve the two source-bound motion radios by their
    // enclosing card rather than guessing the translated wander label.
    const motionGroup=p.locator('.vt-card').filter({has:p.locator('input[type="range"]')}).first().getByRole('radiogroup');
    await motionGroup.getByRole('radio').nth(1).click();await expect.poll(()=>counts.detail.recipe.scenes.motion.kind).toBe('wander');
    const ranges=p.locator('input[type="range"]');if(await ranges.count()){
      const range=ranges.first();await range.focus();await p.keyboard.press('Home');await expect.poll(()=>counts.detail.recipe.scenes.motion.intensity).toBe(0);
    }
    return {modes:['per-minutes','none','fixed'],motion:'wander',fieldContract:'recipe.scenes flat fields, never legacy nested scene settings',memoryOnly:true};
  },counts);
}
function coverStatus(){return {sources:[source],history:[],quota:{provider:'SYNTHETIC',label:'SYNTHETIC quota',paid:true,left:2,total:5},cost:{images:1,provider:'SYNTHETIC',label:'SYNTHETIC',quotaLeft:2},ready:true};}
async function coverCases(viewport){
  const width=viewport.width,board=width<768?'MTemplate':'Template';let status=coverStatus(),trialPosts=0;
  const f=fixture({extraGET:async(route,u)=>{if(u.pathname===target()+'/cover-trials'){await json(route,status);return true;}return false;},blocked:async e=>{if(e.method==='POST'&&e.path===target()+'/cover-trials')trialPosts++;}});
  await run(`synthetic-${width}-cover-ready-confirm-cancel`,viewport,[board+(width<768?'.screen.cover':'.section.cover')],async(p,r)=>{
    await go(p,template('cover'));const draw=p.getByRole('button',{name:'Vẽ thử · 1 ảnh SYNTHETIC',exact:true});await expect(draw).toBeEnabled();await draw.click();const dialog=await oneModal(p);
    await expect(dialog).toContainText(source.title);await expect(dialog).toContainText('2/5');await expect(dialog).toContainText('1 ảnh');await capture(p,r.key+'-confirm',r);
    await dialog.getByRole('button',{name:'Huỷ',exact:true}).click();await expect(modal(p)).toHaveCount(0);expect(trialPosts).toBe(0);
    await draw.click();await oneModal(p);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);expect(trialPosts).toBe(0);
    const prompt=native.prompts.find(p=>p.kind==='cover'&&p.id!==f.detail.recipe.scenes.coverPromptId);if(prompt){await p.getByRole('radio',{name:prompt.name,exact:true}).click();await expect.poll(()=>f.detail.recipe.scenes.coverPromptId).toBe(prompt.id);await expect(p.locator('.vt-pick-detail')).toContainText(prompt.prompt);}
    return {confirmationCostAndSource:true,cancelAndEscapeSubmitNothing:true,paidTrialPOSTs:trialPosts,coverPromptSelection:prompt?.id??'UNPROVEN: second cover prompt missing'};
  },f);
  const blocked=fixture({extraGET:async(route,u)=>{if(u.pathname===target()+'/cover-trials'){await json(route,{...coverStatus(),ready:false,blockedReason:'SYNTHETIC quota exhausted',quota:{provider:'SYNTHETIC',label:'SYNTHETIC quota',paid:true,left:0,total:5}});return true;}return false;}});
  await run(`synthetic-${width}-cover-quota-locked`,viewport,[],async(p,r)=>{
    await go(p,template('cover'));await expect(p.getByRole('button',{name:'Vẽ thử · 1 ảnh SYNTHETIC',exact:true})).toBeDisabled();await expect(p.getByText('SYNTHETIC quota exhausted',{exact:true}).first()).toBeVisible();return {drawDisabled:true,visibleReason:'SYNTHETIC quota exhausted'};
  },blocked);
  let cancels=0;const row={id:'v38-synthetic-trial',jobId:source.jobId,jobTitle:source.title,promptId:native.detail.recipe.scenes.coverPromptId,promptName:'SYNTHETIC trial prompt',composition:'clean',state:'running',costImages:1,createdAt:'2026-11-04T00:00:00Z'};
  let historyStatus={...coverStatus(),history:[row],task:{id:'v38-synthetic-cover-task',state:'running',phase:'SYNTHETIC cover preparation',done:1,total:3}};
  const running=fixture({extraGET:async(route,u)=>{if(u.pathname===target()+'/cover-trials'){await json(route,historyStatus);return true;}return false;},blocked:async e=>{if(e.method==='POST'&&e.path===target()+'/cover-trials/'+row.id+'/cancel')cancels++;}});
  await run(`synthetic-${width}-cover-running-cancel-retry`,viewport,[],async(p,r)=>{
    await go(p,template('cover'));await expect(p.getByRole('status').filter({hasText:'SYNTHETIC cover preparation'})).toBeVisible();await expect(p.locator('#vt-cover-trial-source')).toBeDisabled();await capture(p,r.key+'-running',r);
    const cancel=p.getByRole('button',{name:'Huỷ lần vẽ',exact:true});await cancel.click();await expect.poll(()=>cancels).toBe(1);await expect(p.getByRole('alert').first()).toBeVisible();await cancel.click();await expect.poll(()=>cancels).toBe(2);
    historyStatus={...coverStatus(),history:[{...row,state:'cancelled'}]};await expect(p.getByText('Đã huỷ',{exact:true})).toBeVisible();await capture(p,r.key+'-cancelled-metadata',r);
    await p.locator('.vt-trial').getByRole('button',{name:'Thử lại',exact:true}).click();const dialog=await oneModal(p);await expect(dialog).toContainText(source.title);await dialog.getByRole('button',{name:'Huỷ',exact:true}).click();await expect(modal(p)).toHaveCount(0);
    return {cancelRejections:cancels,separateSyntheticGET:'running to cancelled',retryStillRequiresConfirmation:true,successfulCancellation:'UNPROVEN',trialStartSubmitted:false};
  },running);
  const failed=fixture({extraGET:async(route,u)=>{if(u.pathname===target()+'/cover-trials'){await json(route,{...coverStatus(),history:[{...row,state:'failed',error:{code:'provider_failed',message:'SYNTHETIC trial failure'}}]});return true;}return false;}});
  await run(`synthetic-${width}-cover-failed-retry-confirm`,viewport,[],async(p,r)=>{
    await go(p,template('cover'));await expect(p.locator('.vt-trial[data-state="failed"]')).toBeVisible();await p.locator('.vt-trial').getByRole('button',{name:'Thử lại',exact:true}).click();await oneModal(p);await capture(p,r.key+'-confirm',r);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);return {failedHistoryVisible:true,retryConfirmationAccepted:false,retryOpensConfirmation:true};
  },failed);
}
const now='2026-11-04T00:00:00Z';
function backdropData(){
  const channel={id:'v38-synthetic-channel',platform:'twitch',handle:'synthetic',url:ORIGIN+'/synthetic-provider-link-never-opened',title:'SYNTHETIC backdrop channel',avatarUrl:null,addedAt:now,listedAt:now,listError:null,scanNextPage:null,scanTotal:8,scanStopKnown:true,counts:{clip:8},pausedUntil:null,pauseReason:null,usedBy:[]};
  const video={id:'v38-synthetic-video',platform:'bilibili',url:ORIGIN+'/synthetic-provider-link-never-opened',title:'SYNTHETIC fixed video (metadata only)',durationSec:120,segment:{fromSec:10,toSec:70},state:'failed',usedBy:[],addedAt:now,updatedAt:now};
  return {channel,video};
}
async function backdropCases(viewport){
  const width=viewport.width,board=width<768?'MTemplate':'Template',{channel,video}=backdropData();
  for(const variant of ['healthy','few','capped','missing','empty']){
    const f=fixture();
    f.detail.recipe.backdrop={channels:variant==='empty'?[]:[{channelId:variant==='missing'?'v38-missing-source':channel.id,kinds:['clip'],pinned:[]}],fixedVideoIds:variant==='empty'?[]:[video.id],clipSec:60,clipCount:null};f.detail.recipe.overlays=[];
    const mix={fixedSec:60,perChannel:[{channelId:channel.id,clips:variant==='capped'?40:41,freshVideos:variant==='few'?1:60,repeats:variant==='few'}],loopSec:variant==='capped'?60:0,need:40,capped:variant==='capped'};
    f.extraGET=async(route,u)=>{if(u.pathname==='/api/backdrop/channels'){await json(route,[channel]);return true;}if(u.pathname==='/api/backdrop/videos'){await json(route,[video]);return true;}if(u.pathname===target()+'/backdrop/mix'){await json(route,mix);return true;}return false;};
    const referenceIds=variant==='healthy'?[board+(width<768?'.screen.backdrop':'.section.backdrop'),...(width<768?[]:['Template.fewClips.false'])]:variant==='few'&&width>=768?['Template.fewClips.true']:[];
    await run(`synthetic-${width}-backdrop-${variant}`,viewport,referenceIds,async(p,r)=>{
      await go(p,template('backdrop'));
      if(variant==='empty'){await expect(p.getByText('Chưa có nền',{exact:true})).toBeVisible();await expect(p.locator('.vbd-mixbar')).toHaveCount(0);}
      else{await expect(p.getByText(video.title,{exact:true}).first()).toBeVisible();await expect(p.getByRole('img',{name:/Nền của video 42 phút/})).toBeVisible();}
      if(variant==='healthy'){await expect(p.getByText(/nên nền sẽ lặp nhiều/)).toHaveCount(0);await expect(p.locator('.vbd-mixlegend')).toContainText(channel.title);}
      if(variant==='few')await expect(p.getByText('Chỉ có 1 video để bốc nên nền sẽ lặp nhiều. Vẫn làm video được; gắn thêm kênh để nền đa dạng hơn.',{exact:true})).toBeVisible();
      if(variant==='missing')await expect(p.getByText('Nguồn đã gắn không còn trong Video nền',{exact:true}).first()).toBeVisible();
      if(variant==='capped'){
        const fix=p.getByRole('button',{name:'Tăng mỗi clip lên 62 giây',exact:true});await expect(fix).toBeVisible();await capture(p,r.key+'-warning',r);await fix.click();await expect.poll(()=>f.detail.recipe.backdrop.clipSec).toBe(62);
      }
      await expect(p.getByText('Chưa có lớp phủ nào',{exact:true})).toBeVisible();
      return {variant,mixFixtureOnly:true,videoMetadataOnly:true,clipFix:variant==='capped'?62:null,overlayEmpty:true};
    },f);
  }
  const f=fixture();f.detail.recipe.overlays=[];
  // Use a native overlay definition but no content substitution. Media requests
  // remain blocked; this case accepts metadata/selection, never rendered overlay.
  const overlay=native.overlays[0];
  if(overlay)await run(`synthetic-${width}-overlay-select-remove`,viewport,[],async(p,r)=>{
    await go(p,template('backdrop'));await p.getByRole('button',{name:'Thêm từ thư viện',exact:true}).click();const dialog=await oneModal(p);await expect(dialog).toHaveAccessibleName('Chọn hiệu ứng từ thư viện');await capture(p,r.key+'-picker',r);
    await dialog.getByRole('button').filter({has:p.getByText(overlay.name,{exact:true})}).click();await expect(modal(p)).toHaveCount(0);await expect.poll(()=>f.detail.recipe.overlays).toEqual([overlay.id]);
    const section=p.locator('main .vt-sec');await expect(section.locator('.vbd-ovlist')).toContainText(overlay.name);await section.getByRole('button',{name:'Bỏ hiệu ứng '+overlay.name,exact:true}).click();await expect.poll(()=>f.detail.recipe.overlays).toEqual([]);await expect(section.getByText('Chưa có lớp phủ nào',{exact:true})).toBeVisible();
    return {nativeOverlayMetadata:overlay.id,selectionAndRemovalMemoryOnly:true,renderedOverlayMedia:'UNPROVEN'};
  },f);
  // Choose/video are desktop-only canonical picker values; do not map them to
  // MTemplate.sheet.more, which is the independent mobile action sheet.
  if(width>=768){
    const picker=fixture();picker.detail.recipe.backdrop={channels:[],fixedVideoIds:[],clipSec:60,clipCount:null};
    picker.extraGET=async(route,u)=>{if(u.pathname==='/api/backdrop/channels'){await json(route,[channel]);return true;}if(u.pathname==='/api/backdrop/videos'){await json(route,[video]);return true;}if(u.pathname==='/api/backdrop/channels/'+channel.id+'/videos'){await json(route,{items:[{videoId:'v38-synthetic-clip',kind:'clip',title:'SYNTHETIC catalogue clip',durationSec:60,publishedAt:now,thumbnailUrl:null,url:ORIGIN+'/never-opened',chosenBy:[]}],counts:{clip:1},total:1,unavailable:0});return true;}return false;};
    await run('synthetic-1440-backdrop-picker-choose-video',PC,['Template.picker.choose','Template.picker.video'],async(p,r)=>{
      await go(p,template('backdrop'));await p.getByRole('button',{name:'Gắn kênh hoặc video',exact:true}).click();await oneModal(p);
      const radio=p.locator('.vbd-modes').getByRole('radio',{includeHidden:true}).nth(1);await radio.click();await expect(radio).toBeChecked();await expect(p.locator('.vbd-pick-foot').getByRole('button',{name:'Gắn vào mẫu video',exact:true})).toBeDisabled();await capture(p,r.key+'-choose-empty',r);
      const clip=p.getByRole('checkbox').filter({hasText:'SYNTHETIC catalogue clip'});await clip.click();await expect(clip).toHaveAttribute('aria-checked','true');await expect(p.locator('.vbd-pick-foot').getByRole('button',{name:'Gắn vào mẫu video',exact:true})).toBeEnabled();await capture(p,r.key+'-choose',r);
      await modal(p).getByRole('button').filter({has:p.getByText(video.title,{exact:true})}).click();await expect(p.locator('.vbd-segbox')).toContainText('Đoạn dùng');await expect(p.locator('.vbd-segbox a')).toHaveAttribute('href','#/library?tab=backdrops&nguon=videos');await capture(p,r.key+'-video',r);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);expect(picker.patches).toHaveLength(0);
      return {chooseWithNoPickLocked:true,chooseWithOnePickEnabled:true,videoSegmentMetadata:true,cancelDoesNotAttach:true,canonicalContextDifference:'picker belongs to backdrop section, not images'};
    },picker);
  }
}
async function outputCases(viewport){
  const width=viewport.width,board=width<768?'MTemplate':'Template';const f=fixture();f.detail.recipe.output.compressExport=false;
  await run(`synthetic-${width}-output-compress-and-default-route`,viewport,[board+(width<768?'.screen.output':'.section.output')],async(p,r)=>{
    await go(p,template('output'));const group=p.getByRole('radiogroup',{name:'Nén video sau khi xuất',exact:true}),radios=group.getByRole('radio',{includeHidden:true});const toggle=p.getByRole('switch',{name:'Nén video sau khi xuất',exact:true});
    await expect(radios.nth(1)).toBeChecked();await expect(toggle).not.toBeChecked();await capture(p,r.key+'-original',r);
    await radios.nth(0).click();await expect.poll(()=>f.detail.recipe.output.compressExport).toBe(true);await expect(toggle).toBeChecked();await capture(p,r.key+'-compressed',r);
    await toggle.click();await expect.poll(()=>f.detail.recipe.output.compressExport).toBe(false);await expect(radios.nth(1)).toBeChecked();
    const editorLink=p.locator('.vt-sec').getByRole('link',{name:'Mở trong Biên tập',exact:true});await expect(editorLink).toHaveAttribute('aria-disabled','false');await expect(editorLink).toHaveAttribute('href','#/editor/'+enc(native.detail.id));
    await p.getByRole('button',{name:width<768?'Đặt cho kênh khác':'Đặt làm mặc định cho kênh…',exact:true}).click();const dialog=await oneModal(p);await capture(p,r.key+'-default-picker',r);
    if(!native.owners.length){await expect(dialog).toContainText('Chưa có kênh nào để đặt mẫu video mặc định.');await p.keyboard.press('Escape');return {compressBothValues:true,defaultPicker:'native empty',defaultWrite:'not attempted'};}
    let channel,channelButton;
    for(const owner of native.owners.filter(x=>x.id&&x.id!=='new')){
      // ChannelAvatar contributes the name before copy.attachGo(name).
      const button=dialog.getByRole('button',{name:owner.name+' Mở '+owner.name,exact:true});
      if(await button.count()){channel=owner;channelButton=button;break;}
    }
    if(!channel)unsupported('No routable native owner present in the default picker (reserved id new excluded)');
    await expect(channelButton.locator('.vlb-channel-name')).toHaveText('Mở '+channel.name);await channelButton.click();await expect(p).toHaveURL(new RegExp('#/channels/'+enc(channel.id)+'\\?tab=templates$'));await expect(modal(p)).toHaveCount(0);
    return {compressBothValues:true,defaultPicker:'routes to selected actual channel Template tab',channelId:channel.id,defaultWrite:'not attempted',persistence:'MEMORY_ONLY'};
  },f);
}
async function libraryCases(){
  for(const viewport of [PC,PHONE]){
    const width=viewport.width;
    const empty=fixture({extraGET:async(route,u)=>{if(u.pathname==='/api/templates'){await json(route,{items:[]});return true;}return false;}});
    await run(`synthetic-${width}-library-template-empty`,viewport,width<768?['MLibrary.empty.true']:[],async(p,r)=>{
      await go(p,'/#/library?tab=templates');await expect(p.getByText('Chưa có mẫu video nào',{exact:true})).toBeVisible();await expect(p.getByRole('link',{name:native.detail.name,exact:true})).toHaveCount(0);
      if(width<768){await expect(p.getByRole('button',{name:'Mẫu video mới',exact:true})).toBeVisible();await expect(p.getByRole('button',{name:'Nhập mẫu video',exact:true})).toBeVisible();}return {emptyResponse:true,notLoadFailure:true};
    },empty);
    const error=fixture({extraGET:async(route,u)=>{if(u.pathname==='/api/image-prompts'){await json(route,{error:'SYNTHETIC prompt read failed'},500);return true;}return false;}});
    await run(`synthetic-${width}-library-prompt-error`,viewport,[],async(p,r)=>{
      await go(p,'/#/library?tab=prompts');await expect(p.getByText('Không tải được thư viện. Thử lại.',{exact:true}).first()).toBeVisible();await expect(p.getByText('Không có chỉ dẫn nào khớp bộ lọc và chữ tìm.',{exact:true})).toHaveCount(0);return {promptGET500:true,notFilteredEmpty:true,mobileRetryControl:width<768?'UNSUPPORTED: mobile alert has no local retry button':'present'};
    },error);
    await run(`native-${width}-library-prompt-filters`,viewport,[],async(p,r)=>{
      r.proof='NATIVE_BACKEND_GET';await go(p,'/#/library?tab=prompts');const group=p.getByRole('radiogroup',{name:'Lọc chỉ dẫn',exact:true});
      for(const [index,kind] of [[1,'scene'],[2,'cover']]){await group.getByRole('radio').nth(index).click();await expect(group.getByRole('radio').nth(index)).toHaveAttribute('aria-checked','true');const list=native.prompts.filter(x=>x.kind===kind);if(!list.length)unsupported('Native prompt kind missing: '+kind);await expect(p.getByRole('button').filter({has:p.getByText(list[0].name,{exact:true})}).first()).toBeVisible();await capture(p,r.key+'-'+kind,r);}
      if(width>=768){await p.getByRole('textbox',{name:'Tìm chỉ dẫn',exact:true}).fill('v38-deliberately-unmatched-prompt');await expect(p.getByText('Không có chỉ dẫn nào khớp bộ lọc và chữ tìm.',{exact:true})).toBeVisible();}
      return {filters:['scene','cover'],nativeRecordsOnly:true,searchUnmatched:width>=768,mobileSearch:'not implemented in current source'};
    });
  }
  // Explicitly open both editor modes on the DESKTOP Library board. Mobile
  // prompt editors are covered separately by existing 390 exact-route receipts.
  for(const builtIn of [false,true]){
    const prompt=native.prompts.find(p=>p.builtIn===builtIn);if(!prompt)continue;
    await run('native-1440-prompt-'+(builtIn?'builtin':'mine'),PC,['Library.prompt.'+(builtIn?'builtin':'mine')],async(p,r)=>{
      r.proof='NATIVE_BACKEND_GET';await go(p,`/#/library?tab=prompts&prompt=${builtIn?'builtin':'mine'}&promptId=${enc(prompt.id)}`);await expect(p.locator('#vlb-prompt-name')).toHaveValue(prompt.name);if(builtIn)await expect(p.locator('#vlb-prompt-name')).toHaveAttribute('readonly','');return {promptId:prompt.id,builtIn,exactRecord:true,noPromptMutation:true};
    });
  }
  await run('native-390-library-actions-new-and-template-more',PHONE,['MLibrary.sheet.actions','MLibrary.sheet.new','MTemplate.sheet.more'],async(p,r)=>{
    r.proof='NATIVE_BACKEND_GET';await go(p,'/#/library?tab=templates');const card=p.getByRole('button',{name:'Mở '+native.detail.name,exact:true});await expect(card).toBeVisible();
    // The card's independent menu trigger is source-bound to this template.
    await p.getByRole('button',{name:'Thao tác với '+native.detail.name,exact:true}).click();await oneModal(p);await capture(p,r.key+'-actions',r);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);
    await p.getByRole('button',{name:'Thêm vào thư viện',exact:true}).click();const dialog=await oneModal(p);await expect(dialog.getByRole('button',{name:'Mẫu video mới Bắt đầu từ Mẫu chuẩn, sửa từng mục sau',exact:true})).toBeVisible();await capture(p,r.key+'-new',r);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);
    await go(p,template(undefined,true));await p.getByRole('button',{name:'Thao tác với mẫu video',exact:true}).click();await oneModal(p);await capture(p,r.key+'-template-more',r);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);
    return {actionsAndNewIndependentFromTemplateMore:true,noMutation:true};
  });
  await run('native-1440-library-menu',PC,['Library.menu.true','Library.menu.false'],async(p,r)=>{
    r.proof='NATIVE_BACKEND_GET';await go(p,'/#/library?tab=templates');await p.getByRole('button',{name:'Thêm thao tác cho '+native.detail.name,exact:true}).click();await expect(p.getByRole('menu')).toBeVisible();await capture(p,r.key+'-open',r);await p.keyboard.press('Escape');await expect(p.getByRole('menu')).toHaveCount(0);return {menuOpenedAndCancelled:true,noMutation:true};
  });
  await run('native-1440-import-ready-drag-zip-guard',PC,['Library.import.ready','Library.import.drag','Library.import.zip'],async(p,r)=>{
    r.proof='NATIVE_READ_AND_CLIENT_VALIDATION';await go(p,'/#/library?tab=templates');await p.getByRole('button',{name:'Nhập mẫu video',exact:true}).click();const dialog=await oneModal(p);await expect(dialog).toHaveAccessibleName('Nhập mẫu video cũ');await capture(p,r.key+'-ready',r);
    const drop=dialog.getByRole('button',{name:'Chọn hoặc thả tệp .zip',exact:true});
    const data=await p.evaluateHandle(()=>{const d=new DataTransfer();d.items.add(new File(['client-only'],'v38-invalid.txt',{type:'text/plain'}));return d;});
    await drop.dispatchEvent('dragenter',{dataTransfer:data});await drop.dispatchEvent('dragover',{dataTransfer:data});await expect(drop).toContainText('Thả để nhập 1 tệp');await capture(p,r.key+'-drag',r);
    await dialog.locator('input[type="file"]').setInputFiles({name:'v38-invalid.txt',mimeType:'text/plain',buffer:Buffer.from('invalid, client-only')});await expect(dialog.getByText(/Chỉ nhận tệp .zip/)).toBeVisible();expect(R.requests.filter(x=>x.case===r.key&&x.method!=='GET')).toHaveLength(0);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);
    return {ready:true,drag:'synthetic DOM drag events, not physical drag gesture',zip:'wrong-extension client guard, not invalid ZIP archive parsing',noImportRequest:true};
  });
}
async function additionalStates(){
  for(const viewport of [PC,PHONE]){
    const width=viewport.width;
    const media=native.coverMedia;
    const ready=fixture({extraGET:async(route,u)=>{
      if(u.pathname===target()+'/cover-trials'){
        // Preserve a genuine cover-trial row; prompt samples/artwork need an
        // explicitly synthetic trial wrapper, never a fabricated media URL.
        const row=!media?null:media.kind==='cover-trial'?clone(media.row):media.kind==='prompt-sample'?{
          id:media.row.id,jobId:media.row.videoId,jobTitle:media.row.videoTitle,promptId:media.row.promptId,promptName:media.row.promptName,composition:media.row.composition??'clean',state:'ready',costImages:media.row.costImages,createdAt:media.row.createdAt,finishedAt:media.row.finishedAt,imageUrl:media.imageUrl,
        }:{id:'v38-synthetic-ready-history',jobId:source.jobId,jobTitle:source.title,promptId:media.prompt.id,promptName:media.prompt.name,composition:media.prompt.composition??'clean',state:'ready',costImages:1,createdAt:now,imageUrl:media.imageUrl};
        await json(route,{...coverStatus(),history:row?[row]:[]});return true;
      }return false;
    }});
    await run(`synthetic-${width}-cover-ready-history-native-sample`,viewport,[],async(p,r)=>{
      if(!media)unsupported('No decodable same-origin existing cover-trial image, cover-prompt sample history image or cover-prompt artwork found via native GETs; see native.coverMediaDiscovery for unavailable reads. No artwork fabricated or trial started');
      r.proof='NATIVE_EXISTING_MEDIA_WITH_SYNTHETIC_HISTORY_CONSUMER';
      await go(p,template('cover'));const row=p.locator('.vt-trial[data-state="ready"]');await expect(row).toBeVisible();const image=row.locator('img');await expect(image).toHaveAttribute('src',media.imageUrl);await image.scrollIntoViewIfNeeded();await expect.poll(()=>image.evaluate(el=>el.complete&&el.naturalWidth>0)).toBe(true);
      // Deliberately do not enter the mobile viewer: round45 is excluded.
      return {history:media.kind==='cover-trial'?'native row in synthetic GET envelope':'SYNTHETIC trial wrapper around existing native media',nativeMediaKind:media.kind,nativeReadPath:media.readPath,nativeRecordId:media.row?.id??media.prompt.id,nativeTemplateId:media.templateId??null,genuineExistingImageUrl:media.imageUrl,notGeneratedByThisTrial:true,viewerNotOpened:true};
    },ready);
    const neutralPrompt=native.prompts.find(p=>p.kind==='scene');
    const neutral=fixture({extraGET:async(route,u)=>{
      if(u.pathname==='/api/image-prompts'){
        const kind=u.searchParams.get('kind'),items=native.prompts.filter(p=>!kind||p.kind===kind).map(p=>p.id===neutralPrompt?.id?{...clone(p),sampleUrls:[]}:p);
        await json(route,{items});return true;
      }return false;
    }});
    await run(`synthetic-${width}-prompt-missing-art-neutral`,viewport,[],async(p,r)=>{
      if(!neutralPrompt)unsupported('No native scene prompt to exercise neutral sample fallback');
      await go(p,template('images'));const radio=p.getByRole('radio',{name:neutralPrompt.name,exact:true});await expect(radio).toBeVisible();await expect(radio.locator('img')).toHaveCount(0);await expect(radio.locator('.vt-pick-glyph')).toHaveCount(1);
      if(width>=768)await expect(p.getByRole('button',{name:'Xem lớn '+neutralPrompt.name,exact:true})).toBeDisabled();
      return {nativePromptId:neutralPrompt.id,syntheticMissingSampleMetadata:true,neutralFallback:true,borrowedArtwork:false,staticCompositionImagesRemainOptionIllustrations:true};
    },neutral);
    const noOwners=fixture({extraGET:async(route,u)=>{if(u.pathname==='/api/owner-channels'){await json(route,{channels:[]});return true;}return false;}});
    await run(`synthetic-${width}-default-picker-empty`,viewport,[],async(p,r)=>{
      await go(p,template('output'));await p.getByRole('button',{name:width<768?'Đặt cho kênh khác':'Đặt làm mặc định cho kênh…',exact:true}).click();const dialog=await oneModal(p);await expect(dialog).toContainText('Chưa có kênh nào để đặt mẫu video mặc định.');await expect(dialog.locator('.vlb-channel-pick')).toHaveCount(0);await capture(p,r.key+'-open',r);await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);return {emptyOwnerGET:true,noDefaultMutation:true};
    },noOwners);
    const noOverlays=fixture({extraGET:async(route,u)=>{if(u.pathname==='/api/overlays'){await json(route,{items:[]});return true;}return false;}});noOverlays.detail.recipe.overlays=['v38-synthetic-missing-overlay'];
    await run(`synthetic-${width}-overlay-missing-and-no-candidates`,viewport,[],async(p,r)=>{
    await go(p,template('backdrop'));const row=p.locator('.vbd-ovrow[data-missing]');await expect(row).toHaveCount(1);await expect(row).not.toContainText('v38-synthetic-missing-overlay');await expect(row).toContainText('Lớp phủ không còn trong kho');await expect(row).toContainText('Không tìm thấy lớp phủ đã gắn trong Thư viện.');await capture(p,r.key+'-missing',r);
    },noOverlays);
  }
  // Real import API is never contacted. A held/aborted POST exercises only the
  // genuine pending/offline consumer; it cannot prove ZIP parsing or success.
  let imports=0;const hold=gate(),importFixture=fixture({
    blocked:async e=>{if(e.method==='POST'&&e.path==='/api/templates/import'){imports++;await hold.promise;}},
    releaseAll:()=>hold.release(),
  });
  await run('synthetic-1440-import-pending-offline-retry',PC,['Library.import.importing','Library.import.offline'],async(p,r)=>{
    await go(p,'/#/library?tab=templates');await p.getByRole('button',{name:'Nhập mẫu video',exact:true}).click();const dialog=await oneModal(p);
    // Empty ZIP container, not a claimed valid template. Request is intercepted
    // before networking; its bytes never reach the application's import service.
    await dialog.locator('input[type="file"]').setInputFiles({name:'SYNTHETIC-not-a-template.zip',mimeType:'application/zip',buffer:Buffer.from('504b0506000000000000000000000000000000000000','hex')});
    await expect.poll(()=>imports).toBe(1);await expect(dialog.getByRole('status').filter({hasText:'Đang nhập 0 / 1 tệp…'})).toBeVisible();await capture(p,r.key+'-pending',r);
    hold.release();await expect(dialog.getByRole('button',{name:'Thử lại',exact:true})).toBeVisible();await expect(dialog).toContainText('Mất kết nối máy chủ nên chưa nhập được.');await expect(dialog).toContainText('SYNTHETIC-not-a-template.zip');await capture(p,r.key+'-offline',r);
    await dialog.getByRole('button',{name:'Thử lại',exact:true}).click();await expect.poll(()=>imports).toBe(2);await expect(dialog.getByRole('button',{name:'Thử lại',exact:true})).toBeVisible();await p.keyboard.press('Escape');await expect(modal(p)).toHaveCount(0);
    return {heldImportBeforeNetwork:true,rejectedAttempts:imports,retainsSelectedFile:true,backendImport:false,archiveParsing:'UNPROVEN'};
  },importFixture);
  const {channel,video}=backdropData(),sources=fixture({extraGET:async(route,u)=>{
    if(u.pathname==='/api/backdrop/channels'){await json(route,[channel]);return true;}
    if(u.pathname==='/api/backdrop/videos'){await json(route,[video]);return true;}
    if(u.pathname==='/api/backdrop/channels/'+channel.id+'/videos'){await json(route,{items:[],counts:{clip:8},total:8,unavailable:0});return true;}
    return false;
  }});
  await run('synthetic-1440-library-source-channel-video-routing',PC,['Library.nguon.channels','Library.nguon.videos'],async(p,r)=>{
    await go(p,libraryPath('backdrops','nguon=channels'));const rail=p.locator('nav.vbd-rail');await expect(rail).toBeVisible();await expect(p.getByText(channel.title,{exact:true}).first()).toBeVisible();await capture(p,r.key+'-channels',r);
    await rail.getByRole('button').filter({hasText:'Tất cả video lẻ'}).click();await expect(p).toHaveURL(/nguon=videos$/);await expect(p.getByText(video.title,{exact:true}).first()).toBeVisible();await capture(p,r.key+'-videos',r);
    return {syntheticMetadataOnly:true,routes:['channels','videos'],canonicalNamedChannelIdentity:'UNPROVEN',noSourceDownload:true};
  },sources);
  let parses=0;const parseHold=gate(),adding=fixture({blocked:async e=>{if(e.method==='POST'&&e.path==='/api/sources/parse'){parses++;await parseHold.promise;}},releaseAll:()=>parseHold.release()});
  await run('synthetic-1440-library-source-adding-rejection',PC,['Library.nguon.adding'],async(p,r)=>{
    await go(p,'/#/library?tab=backdrops');await p.locator('#vbd-link').fill('https://example.invalid/SYNTHETIC-never-resolved');await p.locator('#vbd-link').press('Enter');await expect.poll(()=>parses).toBe(1);await expect(p.locator('#vbd-link')).toBeDisabled();await capture(p,r.key+'-adding',r);
    parseHold.release();await expect(p.getByRole('alert').first()).toBeVisible();await expect(p.locator('#vbd-link')).toBeEnabled();await expect(p.locator('#vbd-link')).toHaveValue('https://example.invalid/SYNTHETIC-never-resolved');return {pendingParserRequestHeldThenAborted:true,externalURLNeverResolved:true,errorRetainsInput:true,sourceCreated:false};
  },adding);
}
async function catalogReadError(viewport){
  const f=fixture(),{channel,video}=backdropData();let broken=true;
  f.detail.recipe.backdrop={channels:[{channelId:channel.id,kinds:['clip'],pinned:[]}],fixedVideoIds:[],clipSec:60,clipCount:null};f.detail.recipe.overlays=[];
  f.extraGET=async(route,u)=>{
    if(u.pathname==='/api/backdrop/channels'){await json(route,broken?{error:'STORAGE_INTERNAL_IDENTIFIER',code:'internal'}:[channel],broken?503:200);return true;}
    if(u.pathname==='/api/backdrop/videos'){await json(route,[video]);return true;}
    return false;
  };
  await run(`synthetic-${viewport.width}-catalog-read-recovery`,viewport,[],async(p,r)=>{
    await go(p,template('backdrop'));const section=p.locator('.vt-sec');
    await expect(section).toContainText('Không tải được thư viện. Thử lại.');await expect(section.locator('[data-missing]')).toHaveCount(0);await expect(section).not.toContainText('STORAGE_INTERNAL_IDENTIFIER');await capture(p,r.key+'-failed',r);
    broken=false;await section.getByRole('button',{name:'Thử lại',exact:true}).click();
    await expect(section).not.toContainText('Không tải được thư viện. Thử lại.');await expect(section).toContainText(channel.title);expect(f.patches).toHaveLength(0);await capture(p,r.key+'-recovered',r);
    return {failedCatalogNotMisrepresentedAsMissing:true,retryRecoversWithoutMutation:true,fixtureOnly:true};
  },f);
}
(async()=>{
  try{
    await fs.mkdir(ROOT,{recursive:true});cookies=JSON.parse(await fs.readFile(COOKIES,'utf8'));
    browser=await chromium.launch({executablePath:EXEC,headless:true});await discover();
    for(const viewport of [PC,PHONE]){await previewCases(viewport);await imageCases(viewport);await coverCases(viewport);await backdropCases(viewport);await outputCases(viewport);await catalogReadError(viewport);}
    await libraryCases();await additionalStates();
    R.status='DONE_WITH_CONCERNS'; // Deliberate native-generation/artboard gaps remain even if every prepared UI assertion passes.
  }catch(e){R.status='BLOCKED';R.fatal=e.message;process.exitCode=1;}
  finally{try{if(browser)await browser.close();R.browserClosed=true;}catch(e){R.browserCloseError=e.message;}R.finishedAt=new Date().toISOString();await fs.writeFile(path.join(ROOT,'results.json'),JSON.stringify(R,null,2));}
  const counts=Object.fromEntries(['PASS','FAIL','UNPROVEN','NOT_RUN'].map(s=>[s,R.cases.filter(c=>c.status===s).length]));
  console.log(JSON.stringify({status:R.status,counts,pageErrors:R.pageErrors.length,results:path.join(ROOT,'results.json')}));
  if(counts.FAIL||R.pageErrors.length||R.safety.downloads||R.safety.routeErrors.length)process.exitCode=1;
})().catch(e=>{console.error(e.message);process.exitCode=1;});
