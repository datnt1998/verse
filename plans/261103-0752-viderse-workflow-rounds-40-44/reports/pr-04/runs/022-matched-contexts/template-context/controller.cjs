#!/usr/bin/env node
'use strict';
// Prepared only. Parent explicitly runs: node controller.cjs --execute
// Mounted native UI + typed local GET fixtures. No mutations are sent or faked.
const {createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs=require('node:fs/promises');
const path=require('node:path');
const {createHash}=require('node:crypto');
const ROOT=__dirname, REPO='/Users/dat/Projects/verse';
const ORIGIN='http://127.0.0.1:8043';
const COOKIES='/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const EXEC='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PC={width:1440,height:900}, PHONE={width:390,height:844};
const stamp='2026-11-04T00:00:00Z';
const clone=value=>JSON.parse(JSON.stringify(value)), enc=encodeURIComponent;
const modal=p=>p.locator('[aria-modal="true"]:visible');
const json=(route,body)=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
const sourcePaths=[
  'apps/viderse/web/src/screens/library/template/BackdropSection.tsx',
  'apps/viderse/web/src/screens/library/template/BackdropPicker.tsx',
  'apps/viderse/web/src/screens/library/backdrops/SourceRail.tsx',
  'apps/viderse/web/src/api/backdrop.ts',
  'apps/viderse/docs/design/v1/artboards/Template.dc.html',
  'apps/viderse/docs/design/v1/artboards/MTemplate.dc.html',
  'plans/reports/orchestrate-261105-095440-v38-acceptance/desktop-tweak-truth.md',
  'plans/reports/orchestrate-261105-095440-v38-acceptance/mobile-tweak-truth.md',
  'plans/reports/orchestrate-261105-095440-v38-acceptance/template-visual-acceptance.md',
];
const mappings=['add','choose','video'].map(picker=>({
  key:'Template.picker.'+picker+'@backdrop',
  legacyReferenceId:'Template.picker.'+picker,
  referenceProps:{section:'backdrop',preview:'fresh',fewClips:false,picker},
  legacyProps:{section:'images',preview:'fresh',fewClips:false,picker},
  viewport:PC,
  selectedFile:'fixture-1440-template-picker-'+picker+'-backdrop-open.png',
  stage:{add:'open-lofilanes-unlinked-random-clip-highlight-attach',choose:'open-nightdrivefm-linked-pick-clip-four-selected-save',video:'open-ferry-unlinked-segment-0-120-attach'}[picker],
  declaredValueAlreadyExists:true,newDeclaredValue:false,
}));
// MTemplate has ONE attach-source interaction sheet (sS), not desktop picker enums.
// It is a simple channel list, with no choose/video detailed counterparts.
const mobileMapping={
  key:'MTemplate.screen.backdrop@attach-source',legacyReferenceId:null,
  baseReferenceId:'MTemplate.screen.backdrop',
  referenceProps:{screen:'backdrop',preview:'fresh',sheet:'none'},
  canonicalInteraction:{action:'Gắn kênh hoặc video',state:'sS=true',sheetLabel:'Gắn kênh hoặc video nền'},
  viewport:PHONE,selectedFile:'fixture-390-template-backdrop-attach-source-open.png',
  stage:'open-attach-source-sheet-before-selection',
  declaredValueAlreadyExists:false,newDeclaredValue:false,
  comparisonQualification:'INTERACTION_ONLY; canonical simple list differs from native detailed BackdropPicker Sheet. Not a desktop add/choose/video equivalence claim.',
};
const R={status:'NOT_RUN',scope:'COMPARISON_CONTEXT_ONLY',canonicalPropsAreNewDeclaredValues:false,
  mappings:[...mappings,mobileMapping],cases:[],requests:[],screenshots:[],pageErrors:[],
  provenance:{fixtureLabel:'UI FIXTURE · metadata only · 0 server writes',nativePersistenceProven:false,
    canonicalData:'Template.dc.html:1502–1520,1578–1609; MTemplate.dc.html:955–966,991–994',
    schema:'api/backdrop.ts BackdropChannel, ChannelVideoItem, BackdropVideoPage, BackdropVideo; api/library.ts TemplateDetail, PreviewStatus',
    sources:[],fixtureIdsAreNative:false,templateAndOwnerIdsAreNative:true},
  concerns:[
    'Prepared script is not execution evidence. PASS and screenshots exist only after parent execution.',
    'Named backdrop identities, catalogue metadata, links and healthy mix are local typed GET fixtures, not native records or persisted links.',
    'Native template identity/revision/recipe and owner IDs are discovered with guarded same-origin GETs. Other native recipe fields are preserved; they are not canonical fixture defaults.',
    'Fresh preview is UI metadata without projectId, rendered frame or media URL. No preview build, cover generation, provider discovery, source download or export is accepted.',
    'All fixture thumbnails/avatars are null; standalone videos have ready metadata but no file. Neutral native slots are used. Ferry player/media-region parity remains unproven; do not synthesize artwork.',
    'Current source initializes unlinked Twitch kinds including zero-count VOD, while canonical add uses Clip+Highlight. If its selected VOD chip is aria-disabled, add records BLOCKED_CANONICAL_CONTEXT instead of bypassing actionability or claiming a normal user journey. Other contexts continue.',
    'Mobile canonical has only the attach-source interaction list. No MTemplate.picker.add/choose/video or sheet=source/video is invented. Native detailed-sheet vs canonical simple-list mismatch is retained, not repaired.',
    'No generic final/closed screenshot is generated. Only exact asserted open stages can populate selectedFile. Parent captures matching corrected canonical props separately.',
  ],
  safety:{realWrites:0,nonGETContinued:0,externalContinued:0,localMutationFulfillments:0,downloads:0,websocketsBlocked:0,routeErrors:[]},
};
let browser,cookies,native;
const target=()=>'/api/templates/'+enc(native.detail.id);
const templateRoute=()=>'/#/templates/'+enc(native.detail.id)+'?section=backdrop';
const nativeBootstrap=new Set(['/api/me','/api/activity','/api/events','/api/notifications/settings','/api/queue','/api/status','/api/owner-channels','/api/channels/summary','/api/setup/state','/api/upload-limits','/api/platforms','/api/templates','/api/image-prompts','/api/overlays']);
function allowedNativeAPI(p){return nativeBootstrap.has(p)||/^\/api\/templates\/[^/]+$/.test(p);}

/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').BackdropChannel} BackdropChannel */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').ChannelVideoItem} ChannelVideoItem */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').BackdropVideoPage} BackdropVideoPage */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').BackdropVideo} BackdropVideo */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/library').TemplateDetail} TemplateDetail */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/library').PreviewStatus} PreviewStatus */
const definitions=[
  {key:'slow',name:'slowroads',platform:'twitch',counts:{clip:248,highlight:64,vod:0},rows:[
    ['Đường đèo trong mưa đêm','clip',58],['Cao tốc vắng lúc 3 giờ','clip',60],['Đèn hầm chạy dài','highlight',522],['Mưa trên kính lái','clip',151],['Cầu vượt sương mù','clip',60],['Trạm xăng ven đường','highlight',397],['Đường tuyết không người','clip',55],['Phà đêm qua hồ','highlight',730]]},
  {key:'drive',name:'nightdrivefm',platform:'twitch',counts:{clip:186,highlight:22,vod:9},rows:[
    ['Phố đêm qua kính xe','clip',60],['Đèn neon ngã tư','clip',52],['Đường ven biển','clip',60],['Mưa phùn trên phố','clip',44],['Đèn hậu xe tải','clip',58],['Hầm chui thành phố','highlight',435],['Cao tốc ra biển','clip',60],['Cầu sông lúc 2 giờ','clip',50]]},
  {key:'rain',name:'雨夜电台',platform:'bilibili',counts:{video:300},rows:[
    ['雨夜窗边 · 3小时白噪音','video',10812],['书房听雨','video',4360],['深夜车站下雨','video',3500],['屋檐雨声','video',7500],['小店深夜','video',2710],['铁皮屋顶雨','video',5400],['湖边雾夜','video',3600],['雪夜公路','video',7200]]},
  {key:'lofi',name:'lofilanes',platform:'twitch',counts:{clip:41,highlight:7,vod:0},rows:[
    ['Phòng học đèn vàng','clip',60],['Thành phố nhìn từ ban công','clip',59],['Bàn làm việc đêm','highlight',320],['Trạm xe buýt','clip',46],['Đường ray chiều','clip',60],['Mưa ngoài cửa sổ','clip',51],['Hồ đêm yên','highlight',242],['Tuyết rơi ngoài phố','clip',60]]},
];
const id=key=>'v38-context-fixture-'+key;
function fixture(){
  /** @type {TemplateDetail} */
  const detail=clone(native.detail);
  /** @type {BackdropChannel[]} */
  const channels=definitions.map(d=>({id:id(d.key),platform:d.platform,handle:d.name,url:'https://example.invalid/v38-metadata-only/'+d.key,title:d.name,avatarUrl:null,addedAt:stamp,listedAt:stamp,listError:null,scanNextPage:null,scanTotal:Object.values(d.counts).reduce((a,b)=>a+b,0),scanStopKnown:true,counts:clone(d.counts),pausedUntil:null,pauseReason:null,usedBy:[]}));
  /** @type {Map<string,ChannelVideoItem[]>} */
  const catalogues=new Map(definitions.map(d=>[id(d.key),d.rows.map((row,i)=>({videoId:id(d.key+'-clip-'+i),kind:row[1],title:row[0],durationSec:row[2],publishedAt:stamp,thumbnailUrl:null,url:'https://example.invalid/v38-catalogue-never-open/'+d.key+'/'+i,chosenBy:d.key==='drive'&&i<4?[{templateId:detail.id,name:detail.name}]:[]}))]));
  /** @type {BackdropVideo[]} */
  const videos=[['pass','Đường đèo sương mù','bilibili',760,90],['glass','Mưa trên kính xe','twitch',80,80],['ferry','Phà đêm trong sương','bilibili',1450,120]].map(v=>({id:id(v[0]),title:v[1],platform:v[2],url:'https://example.invalid/v38-metadata-only/'+v[0],durationSec:v[3],segment:{fromSec:0,toSec:v[4]},state:'ready',usedBy:v[0]!=='ferry'?[{templateId:detail.id,name:detail.name}]:[],addedAt:stamp,updatedAt:stamp}));
  detail.recipe.backdrop={channels:[
    {channelId:id('slow'),kinds:['clip','highlight'],pinned:[]},
    {channelId:id('drive'),kinds:['clip'],pinned:catalogues.get(id('drive')).slice(0,4).map(row=>row.videoId)},
  ],fixedVideoIds:[id('pass'),id('glass')],clipSec:60,clipCount:null};
  for(const c of channels){const link=detail.recipe.backdrop.channels.find(l=>l.channelId===c.id);if(link)c.usedBy=[{templateId:detail.id,name:detail.name,mode:link.pinned.length?'pick':'random',kinds:link.kinds,chosenCount:link.pinned.length}];}
  // Choose a real existing snow overlay if available. Do not fabricate an asset ID.
  const snow=native.overlays.find(o=>/snow|tuyết/i.test(o.name));
  detail.recipe.overlays=snow?[snow.id]:[];
  detail.overlayCount=detail.recipe.overlays.length;
  detail.preview={state:'fresh',builtAt:null,minutes:3};
  /** @type {PreviewStatus} */
  const preview={state:'fresh',builtAt:null,minutes:3,templateRevision:detail.revision,sources:[]};
  const mix={fixedSec:170,perChannel:[{channelId:id('slow'),clips:36,freshVideos:312,repeats:false},{channelId:id('drive'),clips:4,freshVideos:4,repeats:false}],loopSec:0,need:40,capped:false};
  return {detail,channels,videos,catalogues,preview,mix,snowOverlayId:snow?.id??null};
}
async function fixtureGET(route,u,f,entry){
  let value,type;
  if(u.pathname===target()){value=f.detail;type='TemplateDetail (native ID/revision + local recipe reference links)';}
  else if(u.pathname===target()+'/preview'){value=f.preview;type='PreviewStatus (fresh metadata; no media)';}
  else if(u.pathname===target()+'/backdrop/mix'){value=f.mix;type='BackdropMix (healthy/fewClips=false context)';}
  else if(u.pathname==='/api/backdrop/channels'){value=f.channels;type='BackdropChannel[]';}
  else if(u.pathname==='/api/backdrop/videos'){value=f.videos;type='BackdropVideo[] (metadata only; no file)';}
  else {
    const c=f.channels.find(c=>u.pathname==='/api/backdrop/channels/'+enc(c.id)+'/videos');
    if(!c)return false;
    const kinds=u.searchParams.get('kind')?.split(',');
    /** @type {BackdropVideoPage} */
    const page={items:f.catalogues.get(c.id).filter(row=>!kinds||kinds.includes(row.kind)),counts:c.counts,total:Object.values(c.counts).reduce((a,b)=>a+b,0),unavailable:0};
    value=page;type='BackdropVideoPage (canonical eight visible metadata rows; no next page)';
  }
  entry.action='LOCAL_TYPED_GET_FIXTURE';entry.fixtureType=type;
  await json(route,value);return true;
}
async function make(viewport,key,f){
  const context=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
  try{
    await context.addCookies(cookies.map(c=>({...c,domain:'127.0.0.1',secure:false})));
    if(!context.routeWebSocket)throw new Error('routeWebSocket is required; refusing unfenced socket traffic');
    await context.routeWebSocket('**/*',socket=>{R.safety.websocketsBlocked++;socket.close();});
    await context.route('**/*',async route=>{
      const req=route.request(),u=new URL(req.url());
      const entry={case:key,method:req.method(),path:u.pathname,queryKeys:[...u.searchParams.keys()]};R.requests.push(entry);
      try{
        if(u.origin!==ORIGIN){entry.action='ABORT_EXTERNAL';return await route.abort('blockedbyclient');}
        if(req.method()!=='GET'){entry.action='ABORT_ALL_NON_GET_BEFORE_NETWORK';return await route.abort('blockedbyclient');}
        if(req.resourceType()==='media'||req.resourceType()==='image'||/(?:^|\/)(?:file|export|downloads?|discover|discovery|parse|follow)(?:\/|$)/i.test(u.pathname)||/\.(?:zip|ytf|mp4|webm|mp3|wav|m3u8|ts)$/i.test(u.pathname)){
          entry.action='ABORT_MEDIA_DOWNLOAD_DISCOVERY';return await route.abort('blockedbyclient');
        }
        if(f&&await fixtureGET(route,u,f,entry))return;
        if(u.pathname.startsWith('/api/')&&!allowedNativeAPI(u.pathname)){entry.action='ABORT_UNLISTED_API_GET';return await route.abort('blockedbyclient');}
        // This is the ONLY continue site: method and origin were checked above.
        entry.action=u.pathname.startsWith('/api/')?'NATIVE_SAME_ORIGIN_GET':'SAME_ORIGIN_APP_STATIC_GET';
        if(req.method()!=='GET'){R.safety.nonGETContinued++;R.safety.realWrites++;throw new Error('Non-GET fence invariant failed');}
        if(u.origin!==ORIGIN){R.safety.externalContinued++;throw new Error('External fence invariant failed');}
        await route.continue();
      }catch(e){entry.error=e.message;R.safety.routeErrors.push({case:key,message:e.message});try{await route.abort('failed');}catch{}}
    });
    const p=await context.newPage();p.setDefaultTimeout(15000);p.setDefaultNavigationTimeout(25000);
    p.on('pageerror',e=>R.pageErrors.push({case:key,message:e.message}));
    p.on('download',d=>{R.safety.downloads++;void d.cancel();});p.on('popup',popup=>void popup.close());
    return {context,p};
  }catch(e){await context.close();throw e;}
}
async function readJSON(p,url){return p.evaluate(async url=>{const response=await fetch(url);if(!response.ok)throw new Error('Native prerequisite GET failed: '+response.status+' '+url);return response.json();},url);}
async function discover(){
  const {context,p}=await make(PC,'native-prerequisites',null);
  try{
    await p.goto(ORIGIN+'/#/library?tab=templates',{waitUntil:'domcontentloaded'});
    const [templates,owners,overlays]=await Promise.all(['/api/templates','/api/owner-channels','/api/overlays'].map(url=>readJSON(p,url)));
    expect(Array.isArray(templates.items),'native template list').toBe(true);
    expect(owners.channels.length,'at least one real native owner prerequisite').toBeGreaterThan(0);
    // Choose a real recipe-capable template; never create a missing prerequisite.
    for(const item of templates.items){
      if(!item.id)continue;
      const detail=await readJSON(p,'/api/templates/'+enc(item.id));
      if(detail.id===item.id&&detail.recipe?.scenes&&Number.isInteger(detail.revision)){
        native={detail,owners:owners.channels,overlays:overlays.items};break;
      }
    }
    if(!native)throw new Error('No native recipe-capable template. No fixture template identity or server creation fallback is allowed.');
    R.native={templateId:native.detail.id,templateName:native.detail.name,revision:native.detail.revision,ownerIds:native.owners.map(o=>o.id),templateOwnerReferences:native.detail.channels.map(c=>c.id),overlayCount:native.overlays.length,proof:'GUARDED_NATIVE_GET_ONLY'};
    R.provenance.nativeGETs=['/api/templates','/api/owner-channels','/api/overlays',target()];
  }finally{await context.close();}
}
async function badge(p,key){
  await p.evaluate(({label,key})=>{
    const node=document.createElement('div');node.dataset.v38Fixture='true';node.textContent=label+' · '+key;
    Object.assign(node.style,{position:'fixed',top:'4px',right:'4px',zIndex:'2147483647',pointerEvents:'none',maxWidth:'calc(100vw - 8px)',padding:'4px 6px',borderRadius:'4px',background:'#211b0b',border:'1px solid #ae9140',color:'#ffe39c',font:'10px/1.3 system-ui,sans-serif'});
    document.body.append(node);
  },{label:R.provenance.fixtureLabel,key});
}
async function settle(p){
  await p.evaluate(async()=>{
    await document.fonts.ready;
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
    const finite=document.getAnimations().filter(a=>a.playState==='running'&&Number.isFinite(a.effect?.getComputedTiming().endTime));
    await Promise.all(finite.map(a=>a.finished.catch(()=>{})));
    await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
  });
}
async function open(p,f,kind){
  await p.goto(ORIGIN+templateRoute(),{waitUntil:'domcontentloaded'});
  await expect(p.getByRole('button',{name:'Gắn kênh hoặc video',exact:true})).toBeVisible();
  await expect(p.locator('.vbd-srcrow')).toHaveCount(4);
  await expect(p.getByText('Chỉ có 1 video để bốc nên nền sẽ lặp nhiều.',{exact:false})).toHaveCount(0);
  if(kind==='choose'){
    const row=p.locator('.vbd-srcrow').filter({has:p.getByText('nightdrivefm',{exact:true})});
    await expect(row).toHaveCount(1);
    await row.getByRole('button',{name:'Chọn video',exact:true}).click();
  }else await p.getByRole('button',{name:'Gắn kênh hoặc video',exact:true}).click();
  await expect(modal(p)).toHaveCount(1);
  const dialog=modal(p);
  await expect(dialog).toHaveAccessibleName('Gắn video nền vào '+f.detail.name);
  if(kind==='add'){
    await dialog.locator('.vbd-rail-row').filter({hasText:'lofilanes'}).click();
    // Reach Clip+Highlight only through normally actionable native controls.
    // A selected zero-count VOD with aria-disabled is a source/default defect,
    // not permission to force a click or rewrite component state.
    const vod=dialog.locator('.vbd-kindchip').filter({hasText:'Phát lại'});
    if(await vod.getAttribute('aria-pressed')==='true'){
      if(await vod.getAttribute('aria-disabled')==='true'||await vod.isDisabled()){
        const error=new Error('Canonical add blocked: unlinked lofilanes defaults to selected zero-count VOD but the real chip is aria-disabled. Parent must correct the native default/semantics before this exact comparison can pass.');
        error.comparisonBlocked=true;throw error;
      }
      await vod.click();
    }
  }else if(kind==='video'){
    await dialog.locator('.vbd-rail-row').filter({hasText:'Phà đêm trong sương'}).click();
  }
  return dialog;
}
async function assertChannel(dialog,f,key,mode,kinds,picked,linked,cta){
  await expect(dialog.locator('.vbd-pick-name')).toHaveText(key==='drive'?'nightdrivefm':key==='lofi'?'lofilanes':'slowroads');
  await expect(dialog.locator('.vbd-rail-row[aria-current="true"]')).toHaveCount(1);
  await expect(dialog.locator('.vbd-rail-row[aria-current="true"]')).toContainText(key==='drive'?'nightdrivefm':key==='lofi'?'lofilanes':'slowroads');
  const radios=dialog.locator('.vbd-modes').getByRole('radio',{includeHidden:true});
  await expect(radios).toHaveCount(2);await expect(radios.nth(mode==='pick'?1:0)).toBeChecked();await expect(radios.nth(mode==='pick'?0:1)).not.toBeChecked();
  const pressed=await dialog.locator('.vbd-kindchip[aria-pressed="true"]').allTextContents();
  expect(pressed.map(s=>s.trim().replace(/\s*[\d.,]+\s*$/,'')).sort(),'exact enabled kinds').toEqual([...kinds].sort());
  const recipeLink=f.detail.recipe.backdrop.channels.find(c=>c.channelId===id(key));
  expect(Boolean(recipeLink),'fixture link is local UI reference only').toBe(linked);
  expect(recipeLink?.pinned.length??0,'local fixture selected count').toBe(picked);
  if(mode==='pick'){
    await expect(dialog.locator('.vbd-picknote')).toHaveText('Đã chọn '+picked+' video');
    await expect(dialog.locator('.vbd-pick-item[aria-checked="true"]')).toHaveCount(picked);
    // Pin identity, not just a count or a shared case name.
    for(const item of f.catalogues.get(id(key)).slice(0,picked))await expect(dialog.getByRole('checkbox',{name:'Chọn '+item.title,exact:true})).toHaveAttribute('aria-checked','true');
  }else{
    await expect(dialog.getByRole('checkbox')).toHaveCount(0);
    await expect(dialog.locator('.vbd-picknote')).toHaveText('Xem trước 8 / '+(key==='lofi'?48:312)+' video');
  }
  if(linked)await expect(dialog.locator('.vbd-pick-meta')).toContainText('đã gắn vào mẫu video này');
  else await expect(dialog.locator('.vbd-pick-meta')).not.toContainText('đã gắn vào mẫu video này');
  await expect(dialog.locator('.vbd-pick-foot').getByRole('button',{name:cta,exact:true})).toBeEnabled();
  await expect(dialog.locator('.vbd-pick-foot').getByRole('button',{name:linked?'Gắn vào mẫu video':'Lưu',exact:true})).toHaveCount(0);
  return {selectedSourceId:id(key),sourceKind:'channel',linked,mode,kinds,pickedCount:picked,cta};
}
async function assertVideo(dialog,f){
  await expect(dialog.locator('.vbd-pick-name')).toHaveText('Phà đêm trong sương');
  await expect(dialog.locator('.vbd-rail-row[aria-current="true"]')).toHaveCount(1);
  await expect(dialog.locator('.vbd-rail-row[aria-current="true"]')).toContainText('Phà đêm trong sương');
  await expect(dialog.locator('.vbd-modes')).toHaveCount(0);await expect(dialog.getByRole('checkbox')).toHaveCount(0);await expect(dialog.locator('.vbd-kindchip')).toHaveCount(0);
  expect(f.detail.recipe.backdrop.fixedVideoIds).not.toContain(id('ferry'));
  await expect(dialog.locator('.vbd-pick-meta')).not.toContainText('đã gắn');
  await expect(dialog.locator('.vbd-segbox-label')).toHaveText('Đoạn dùng · dài 2:00');
  await expect(dialog.locator('.vbd-segbox-range')).toContainText('0:00–2:00');
  await expect(dialog.locator('.vbd-segbox a')).toHaveAttribute('href','#/library?tab=backdrops&nguon=videos');
  await expect(dialog.locator('.vbd-pick-foot').getByRole('button',{name:'Gắn vào mẫu video',exact:true})).toBeEnabled();
  await expect(dialog.locator('.vbd-pick-foot').getByRole('button',{name:'Đã gắn',exact:true})).toHaveCount(0);
  await expect(dialog.locator('video')).toHaveCount(0);
  return {selectedSourceId:id('ferry'),canonicalSourceKey:'ferry',sourceKind:'standalone-video',linked:false,mode:null,kinds:[],pickedCount:0,segment:{fromSec:0,toSec:120},cta:'Gắn vào mẫu video',media:'NEUTRAL_NATIVE_SLOT_NO_FILE_NO_MEDIA_ACQUISITION'};
}
async function run(mapping,kind){
  const f=fixture(),record={key:mapping.key,status:'NOT_RUN',mapping:clone(mapping),selectedFile:null,proof:'LOCAL_TYPED_UI_FIXTURE_NOT_NATIVE_PERSISTENCE'};
  R.cases.push(record);let context,p;
  try{
    ({context,p}=await make(mapping.viewport,mapping.key,f));
    const dialog=await open(p,f,kind);
    const before=R.requests.filter(e=>e.case===mapping.key&&e.method!=='GET').length;
    expect(before,'opening/selecting metadata must not submit any mutation').toBe(0);
    record.fixture={nativeTemplateId:f.detail.id,nativeRevision:f.detail.revision,nativeOwnerIds:R.native.ownerIds,backdrop:clone(f.detail.recipe.backdrop),preview:clone(f.preview),mix:clone(f.mix),channels:clone(f.channels),videos:clone(f.videos),catalogues:Object.fromEntries(f.catalogues),snowOverlayId:f.snowOverlayId,persistence:'LOCAL_REFERENCE_ONLY; fixture() clones native detail and replaces only the local GET response recipe backdrop; no PATCH/POST occurs'};
    if(!f.snowOverlayId)record.concern='No native snow overlay metadata found; overlay underlay remains empty. No fake native asset is created.';
    const assertState=async()=>{
      await expect(modal(p)).toHaveCount(1);
      expect(new URL(p.url()).hash).toContain('section=backdrop');
      if(kind==='video')return assertVideo(dialog,f);
      if(kind==='choose')return assertChannel(dialog,f,'drive','pick',['Clip'],4,true,'Lưu');
      if(kind==='add')return assertChannel(dialog,f,'lofi','random',['Clip','Highlight'],0,false,'Gắn vào mẫu video');
      // Mobile has the real canonical attach entry only, not the desktop contexts.
      await expect(dialog).toHaveClass(/m-sheet/);
      record.canonicalMobileList={names:['slowroads','nightdrivefm','雨夜电台','lofilanes'],linkedNames:['slowroads','nightdrivefm'],selectedKindsAndCTA:'NOT_APPLICABLE_IN_CANONICAL_SIMPLE_LIST'};
      for(const name of record.canonicalMobileList.names)await expect(dialog.locator('.vbd-rail-row').filter({hasText:name})).toHaveCount(1);
      return assertChannel(dialog,f,'slow','random',['Clip','Highlight'],0,true,'Lưu');
    };
    record.observed=await assertState();
    await badge(p,mapping.key);await settle(p);
    // Reassert the exact state AFTER fonts/finite animations and BEFORE the file.
    record.observed=await assertState();
    const geometry=await p.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth}));
    expect(geometry).toEqual({width:mapping.viewport.width,height:mapping.viewport.height,scrollWidth:mapping.viewport.width});
    expect(R.requests.filter(e=>e.case===mapping.key&&e.method!=='GET'),'no fixture attachment/save submits').toHaveLength(0);
    await p.screenshot({path:path.join(ROOT,mapping.selectedFile),fullPage:false});
    record.selectedFile=mapping.selectedFile;record.geometry=geometry;record.status='PASS';
    R.screenshots.push({key:mapping.key,file:mapping.selectedFile,stage:mapping.stage,referenceProps:mapping.referenceProps,canonicalInteraction:mapping.canonicalInteraction??null,observed:record.observed});
    // Deliberately do not press CTA or close then capture another generic stage.
  }catch(e){record.status=e.comparisonBlocked?'BLOCKED_CANONICAL_CONTEXT':'FAIL';record.reason=e.message;process.exitCode=1;}
  finally{if(context)await context.close();}
}
async function main(){
  if(!process.argv.includes('--execute'))throw new Error('Preparation-only guard: parent must review then run node controller.cjs --execute. No browser started.');
  await fs.mkdir(ROOT,{recursive:true});
  for(const relative of sourcePaths){const bytes=await fs.readFile(path.join(REPO,relative));R.provenance.sources.push({path:relative,sha256:createHash('sha256').update(bytes).digest('hex')});}
  R.provenance.controllerSHA256=createHash('sha256').update(await fs.readFile(__filename)).digest('hex');
  // Cookie contents are consumed only by browser context; never printed/receipted.
  cookies=JSON.parse(await fs.readFile(COOKIES,'utf8'));
  browser=await chromium.launch({executablePath:EXEC,headless:true});
  await discover();
  for(let i=0;i<mappings.length;i++)await run(mappings[i],['add','choose','video'][i]);
  await run(mobileMapping,'mobile-source');
  expect(R.safety.realWrites).toBe(0);expect(R.safety.nonGETContinued).toBe(0);expect(R.safety.externalContinued).toBe(0);expect(R.safety.localMutationFulfillments).toBe(0);expect(R.safety.downloads).toBe(0);expect(R.safety.routeErrors).toHaveLength(0);
  R.status=R.cases.every(c=>c.status==='PASS')&&R.pageErrors.length===0?'CAPTURED_WITH_COMPARISON_CONCERNS':'FAILED';
  if(R.status==='FAILED')process.exitCode=1;
}
if(process.argv.includes('--execute')){
  main().catch(e=>{R.status='BLOCKED';R.fatal=e.message;process.exitCode=1;}).finally(async()=>{
    try{if(browser)await browser.close();R.browserClosed=true;}catch(e){R.browserClosed=false;R.browserCloseError=e.message;process.exitCode=1;}
    R.finishedAt=new Date().toISOString();
    await fs.writeFile(path.join(ROOT,'results.json'),JSON.stringify(R,null,2));
    console.log(JSON.stringify({status:R.status,cases:R.cases.map(c=>({key:c.key,status:c.status,selectedFile:c.selectedFile})),safety:R.safety,results:path.join(ROOT,'results.json')}));
  }).catch(e=>{console.error(e.message);process.exitCode=1;});
}else{console.error('Prepared only. Parent: node '+__filename+' --execute');process.exitCode=2;}
