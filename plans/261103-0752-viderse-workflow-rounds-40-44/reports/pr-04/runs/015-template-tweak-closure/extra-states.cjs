#!/usr/bin/env node
'use strict';
// PREPARED ONLY: Main runs this script. No native mutation, discovery or media.
const {createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs=require('node:fs/promises');
const path=require('node:path');
const ROOT=__dirname, OUT=path.join(ROOT,'extra-states');
const SOURCE='/Users/dat/Projects/verse/apps/viderse/web/src';
const ARTBOARD='/Users/dat/Projects/verse/apps/viderse/docs/design/v1/artboards/Library.dc.html';
const MANIFEST='/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v38-acceptance-xxkx9151/remaining-v38-state-manifest.json';
const ORIGIN='http://127.0.0.1:8043';
const EXEC=process.env.EXTRA_STATES_CHROME||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PC={width:1440,height:900}, PHONE={width:390,height:844};
const LABEL='SYNTHETIC UI metadata';
const stamp='2026-11-04T00:00:00Z';
const R={status:'NOT_RUN',proof:'LOCAL_FIXTURE_UI_CONSUMER_ONLY',nativeBackendProvenance:false,cases:[],requests:[],screenshots:[],pageErrors:[],limitations:[
  'Import POST is fulfilled locally with TemplateImportResult, never continued. Empty ZIP containers are fixture carriers, not valid Viderse exports. No archive parsing, media ingest or backend persistence is proven.',
  'Named channel identities, usage, catalogue counts, pause and progress are explicitly labelled metadata fixtures. All avatar/thumbnail URLs are null; no provider discovery or media is fetched.',
  'Canonical Library boards are desktop 1440x900. The manifest has no mobile import.done/partial, nguon.addvideo or kenh.* board. Mobile import captures are supplemental; mobile backdrop capture references only MLibrary.tab.backdrops.',
  'Reference IDs identify comparison targets, not pixel-parity acceptance. Real artboard thumbnails, native records, native import success and actual downloads remain prerequisites for native/visual parity.'
],safety:{nonGETContinued:0,externalContinued:0,downloads:0,websocketsBlocked:0,routeErrors:[]}};
let browser,manifest,cookies=[];
const clone=x=>JSON.parse(JSON.stringify(x));
const json=(route,body)=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
const modal=p=>p.locator('[aria-modal="true"]:visible');
const enc=encodeURIComponent;
const label=name=>name+' · '+LABEL;

/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/library').TemplateImportItem} TemplateImportItem */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/library').TemplateImportResult} TemplateImportResult */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').BackdropChannel} BackdropChannel */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').ChannelVideoItem} ChannelVideoItem */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').BackdropVideoPage} BackdropVideoPage */
/** @typedef {import('/Users/dat/Projects/verse/apps/viderse/web/src/api/backdrop').BackdropVideo} BackdropVideo */

// Titles, durations, counts and usage modes: Library.dc.html:1533-1552.
// IDs and timestamps are synthetic. No local fixture is presented as native.
const definitions=[
  {key:'slowroads',platform:'twitch',handle:'slowroads',name:'slowroads',url:'https://twitch.tv/slowroads',counts:{clip:248,highlight:64,vod:0},use:[['Mẫu chuẩn · Tách nền','random',['clip','highlight','vod'],0],['Truyện kể đêm','random',['clip'],0]],rows:[
    ['Đường đèo trong mưa đêm','clip',58],['Cao tốc vắng lúc 3 giờ','clip',60],['Đèn hầm chạy dài','highlight',522],['Mưa trên kính lái','clip',151],['Cầu vượt sương mù','clip',60],['Trạm xăng ven đường','highlight',397],['Đường tuyết không người','clip',55],['Phà đêm qua hồ','highlight',730],['Rừng thông lúc nửa đêm','clip',60],['Đèn đường nhoè mưa','clip',48],['Ngã ba trong sương','clip',60],['Nhà ven hồ tắt đèn','clip',40,'off']]},
  {key:'nightdrivefm',platform:'twitch',handle:'nightdrivefm',name:'nightdrivefm',url:'https://twitch.tv/nightdrivefm',counts:{clip:186,highlight:22,vod:9},use:[['Mẫu chuẩn · Tách nền','pick',['clip','highlight','vod'],4]],rows:[
    ['Phố đêm qua kính xe','clip',60,'pin'],['Đèn neon ngã tư','clip',52,'pin'],['Đường ven biển','clip',60,'pin'],['Mưa phùn trên phố','clip',44,'pin'],['Đèn hậu xe tải','clip',58],['Hầm chui thành phố','highlight',435],['Cao tốc ra biển','clip',60],['Cầu sông lúc 2 giờ','clip',50],['Phát lại · chạy đêm 3 tiếng','vod',10931],['Đèn giao thông nhấp nháy','clip',36],['Mưa đá trên kính','clip',60],['Đường vành đai vắng','clip',57]]},
  {key:'bilibili',platform:'bilibili',handle:'3493118',name:'雨夜电台',url:'https://space.bilibili.com/3493118',counts:{video:300},use:[['Ru ngủ · Không ảnh','random',['video'],0]],rows:[
    ['雨夜窗边 · 3小时白噪音','video',10812],['书房听雨','video',4360],['深夜车站下雨','video',3500],['屋檐雨声','video',7500],['小店深夜','video',2710],['铁皮屋顶雨','video',5400],['湖边雾夜','video',3600],['雪夜公路','video',7200]]},
  {key:'lofilanes',platform:'twitch',handle:'lofilanes',name:'lofilanes',url:'https://twitch.tv/lofilanes',counts:{clip:41,highlight:7,vod:0},use:[],rows:[
    ['Phòng học đèn vàng','clip',60],['Thành phố nhìn từ ban công','clip',59],['Bàn làm việc đêm','highlight',320],['Trạm xe buýt','clip',46],['Đường ray chiều','clip',60],['Mưa ngoài cửa sổ','clip',51],['Hồ đêm yên','highlight',242],['Tuyết rơi ngoài phố','clip',60]]},
];
/** @type {BackdropChannel[]} */
const channels=definitions.map(d=>({id:'v38-fixture-'+d.key,platform:d.platform,handle:d.handle,url:d.url,title:label(d.name),avatarUrl:null,addedAt:stamp,listedAt:stamp,listError:null,scanNextPage:d.key==='bilibili'?2:null,scanTotal:Object.values(d.counts).reduce((n,c)=>n+c,0),scanStopKnown:d.key!=='bilibili',counts:d.counts,pausedUntil:null,pauseReason:null,usedBy:d.use.map((u,i)=>({templateId:'v38-fixture-template-'+d.key+'-'+i,name:label(u[0]),mode:u[1],kinds:u[2],chosenCount:u[3]}))}));
/** @type {Map<string, ChannelVideoItem[]>} */
const catalogues=new Map(definitions.map((d,di)=>[channels[di].id,d.rows.map((row,i)=>({videoId:'v38-fixture-'+d.key+'-'+i,kind:row[1],title:label(row[0]),durationSec:row[2],publishedAt:stamp,thumbnailUrl:null,url:d.url,chosenBy:row[3]==='pin'?[{templateId:channels[di].usedBy[0].templateId,name:channels[di].usedBy[0].name}]:[],...(row[3]==='off'?{unavailable:{reason:'unavailable',at:stamp}}:{})}))]));
// Library.nguon.addvideo is a newly downloading video card, NOT parse/segment.
// Source: Library.dc.html:1561; BackdropVideo state/progress: api/backdrop.ts.
/** @type {BackdropVideo[]} */
const videos=[
  {id:'v38-fixture-snow',platform:'bilibili',url:'https://example.invalid/v38-metadata-only-snow',title:label('雪夜公路 · 4K 环境音'),durationSec:3492,segment:{fromSec:0,toSec:60},state:'downloading',progress:0.62,usedBy:[],addedAt:stamp,updatedAt:stamp},
  ...[['Đường đèo sương mù','bilibili',760,90],['Mưa trên kính xe','twitch',80,80],['Phà đêm trong sương','bilibili',1450,120]].map((v,i)=>({id:'v38-fixture-fixed-'+i,platform:v[1],url:'https://example.invalid/v38-metadata-only-'+i,title:label(v[0]),durationSec:v[2],segment:{fromSec:0,toSec:v[3]},state:'ready',usedBy:i<2?[{templateId:'v38-fixture-template-fixed',name:label('Mẫu chuẩn · Tách nền')}]:[],addedAt:stamp,updatedAt:stamp})),
];
const importRows=[
  {file:'SYNTHETIC-truyen-ke-dem.ytf-template.zip',name:label('Truyện kể đêm'),media:14,mediaReused:0,warnings:[]},
  {file:'SYNTHETIC-phim-tai-lieu.ytf-template.zip',name:label('Phim tài liệu'),media:9,mediaReused:3,warnings:['SYNTHETIC fixture · Font “SVN-Gilroy” chưa có trên máy chủ · dùng font mặc định Be Vietnam Pro','SYNTHETIC fixture · Prompt ảnh bìa “Chữ hook vàng” đã có sẵn · dùng lại bản trong thư viện']},
  {file:'SYNTHETIC-hop-dem-cu.ytf-template.zip',name:label('Hộp đêm cũ'),media:6,mediaReused:1,warnings:[]},
];
function fixture(){return {channels:clone(channels),videos:clone(videos),imports:[],importMode:null};}
async function fixtureGET(route,u,f,entry){
  if(u.pathname==='/api/templates'){entry.fixture='TemplateSummary[] intentionally empty; no import persistence';await json(route,{items:[]});return true;}
  if(u.pathname==='/api/image-prompts'||u.pathname==='/api/overlays'){await json(route,{items:[]});return true;}
  if(u.pathname==='/api/platforms'){await json(route,[]);return true;}
  if(u.pathname==='/api/backdrop/channels'){entry.fixture='BackdropChannel[]';await json(route,f.channels);return true;}
  if(u.pathname==='/api/backdrop/videos'){entry.fixture='BackdropVideo[] metadata only';await json(route,f.videos);return true;}
  for(const channel of f.channels){
    if(u.pathname==='/api/backdrop/channels/'+enc(channel.id)+'/videos'){
      const kinds=u.searchParams.get('kind')?.split(',');
      const items=catalogues.get(channel.id).filter(v=>!kinds||kinds.includes(v.kind));
      /** @type {BackdropVideoPage} */
      const page={items,counts:channel.counts,total:Object.values(channel.counts).reduce((n,c)=>n+c,0),unavailable:items.filter(i=>i.unavailable).length};
      entry.fixture='BackdropVideoPage canonical visible subset; no further page fixture';await json(route,page);return true;
    }
  }
  return false;
}
const nativeGET=new Set(['/api/me','/api/activity','/api/notifications/settings','/api/queue','/api/status','/api/owner-channels','/api/channels/summary','/api/setup/state','/api/upload-limits']);
async function make(viewport,key,f){
  const context=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
  try{
    if(cookies.length)await context.addCookies(cookies.map(c=>({...c,domain:'127.0.0.1',secure:false})));
    if(!context.routeWebSocket)throw new Error('Playwright routeWebSocket required; cannot guarantee zero websocket egress');
    await context.routeWebSocket('**/*',socket=>{R.safety.websocketsBlocked++;socket.close();});
    await context.route('**/*',async route=>{
      const req=route.request(),u=new URL(req.url());
      const entry={case:key,method:req.method(),path:u.origin===ORIGIN?u.pathname:'[external URL redacted]'};R.requests.push(entry);
      try{
        if(u.origin!==ORIGIN){entry.action='BLOCK_EXTERNAL';return await route.abort('blockedbyclient');}
        if(req.method()!=='GET'){
          if(req.method()==='POST'&&u.pathname==='/api/sources/parse'&&f.unsupported){
            expect(req.postDataJSON()).toEqual({input:'https://example.invalid/SYNTHETIC-unsupported'});
            entry.action='LOCAL_FULFILL_UNSUPPORTED_PARSE';return await json(route,{kind:'unsupported',host:'example.invalid',label:'SYNTHETIC nền tảng chưa hỗ trợ'});
          }
          if(req.method()==='POST'&&u.pathname==='/api/templates/import'&&f.importMode){
            // Import responses are local only; no archive reaches the service.
            // Inspect only fixture filename, never store multipart bytes/headers.
            const row=importRows[f.imports.length];
            expect(row,'one request per selected fixture file').toBeTruthy();
            expect(req.headers()['content-type']).toContain('multipart/form-data');
            expect(req.postDataBuffer().includes(Buffer.from(row.file))).toBe(true);
            const ok=f.importMode==='done'||f.imports.length<2;
            /** @type {TemplateImportItem} */
            const item={...clone(row),ok,...(ok?{templateId:'v38-local-import-'+f.imports.length}:{media:0,mediaReused:0,error:'SYNTHETIC fixture · tệp hỏng hoặc không phải bản xuất template'})};
            /** @type {TemplateImportResult} */
            const result={items:[item],imported:ok?1:0,failed:ok?0:1};
            f.imports.push(result);entry.action='LOCAL_FULFILL_IMPORT_ONLY';entry.fixture={file:row.file,ok,provenance:'SYNTHETIC_LOCAL_RESPONSE',backendPersistence:false};
            return await json(route,result);
          }
          entry.action='ABORT_NON_GET_BEFORE_NETWORK';return await route.abort('blockedbyclient');
        }
        if(/(?:^|\/)(?:export|download|downloads|discover|discovery|follow|parse|file)(?:\/|$)/i.test(u.pathname)||['media','eventsource'].includes(req.resourceType())||/\.(?:zip|ytf|mp4|webm|mp3|wav|m3u8|ts)$/i.test(u.pathname)){entry.action='BLOCK_PROVIDER_MEDIA_DOWNLOAD_STREAM';return await route.abort('blockedbyclient');}
        if(await fixtureGET(route,u,f,entry)){entry.action='LOCAL_FULFILL_GET_FIXTURE';return;}
        if(u.pathname.startsWith('/api/')&&!nativeGET.has(u.pathname)){entry.action='BLOCK_UNLISTED_API_GET';return await route.abort('blockedbyclient');}
        entry.action=u.pathname.startsWith('/api/')?'NATIVE_SHELL_GET_ONLY':'SAME_ORIGIN_STATIC_GET';
        // Sole continuation site: same-origin GET, after all unsafe checks.
        expect(req.method()).toBe('GET');expect(u.origin).toBe(ORIGIN);return await route.continue();
      }catch(e){entry.action='ROUTE_ERROR_ABORT';R.safety.routeErrors.push({case:key,message:e.message});try{await route.abort('failed');}catch{}}
    });
    const p=await context.newPage();p.setDefaultTimeout(15000);p.setDefaultNavigationTimeout(20000);
    p.on('pageerror',e=>R.pageErrors.push({case:key,message:e.message}));
    p.on('download',d=>{R.safety.downloads++;void d.cancel();});p.on('popup',popup=>void popup.close());
    return {context,p};
  }catch(e){await context.close();throw e;}
}
async function capture(p,record,suffix=''){
  await p.evaluate(async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));});
  const geometry=await p.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth}));
  expect(geometry.width).toBe(record.viewport.width);expect(geometry.height).toBe(record.viewport.height);
  expect(geometry.scrollWidth,'no horizontal document overflow').toBeLessThanOrEqual(geometry.width+1);
  expect(await modal(p).count(),'at most one active modal').toBeLessThanOrEqual(1);
  const file=record.key+suffix+'.png';await p.screenshot({path:path.join(OUT,file)});
  record.screenshots.push(file);R.screenshots.push({case:record.key,file,referenceIds:record.referenceIds,comparisonTargetIds:record.comparisonTargetIds,provenance:record.provenance,geometry});
}
async function run(key,viewport,referenceIds,comparisonTargetIds,f,fn){
  const record={key,viewport,referenceIds,comparisonTargetIds,status:'NOT_RUN',screenshots:[],provenance:{kind:'SYNTHETIC_LOCAL_FIXTURE_UI',nativeBackend:false,mediaFetched:false}};R.cases.push(record);
  let context,p;
  try{
    for(const id of [...referenceIds,...comparisonTargetIds])expect(manifest.references.some(r=>r.id===id),'canonical reference '+id).toBe(true);
    ({context,p}=await make(viewport,key,f));record.observed=await fn(p,record);
    const requests=R.requests.filter(e=>e.case===key);
    expect(requests.filter(e=>e.action==='NATIVE_SHELL_GET_ONLY').every(e=>e.method==='GET')).toBe(true);
    expect(requests.filter(e=>e.method!=='GET').every(e=>['LOCAL_FULFILL_IMPORT_ONLY','LOCAL_FULFILL_UNSUPPORTED_PARSE','ABORT_NON_GET_BEFORE_NETWORK'].includes(e.action))).toBe(true);
    expect(record.provenance.nativeBackend).toBe(false);await capture(p,record);record.status='PASS';
  }catch(e){record.status='FAIL';record.reason=e.message;if(p)try{await capture(p,record,'-failure');}catch(err){record.captureError=err.message;}}
  finally{if(context)await context.close();}
}
const go=(p,route)=>p.goto(ORIGIN+'/#'+route,{waitUntil:'domcontentloaded'});
async function importCase(viewport,mode){
  const f=fixture();f.importMode=mode;
  const ref='Library.import.'+mode;
  await run(`fixture-${viewport.width}-import-${mode}`,viewport,viewport.width===1440?[ref]:[],viewport.width===390?[ref]:[],f,async(p,r)=>{
    await go(p,'/library?tab=templates');
    if(viewport.width<768){
      await p.getByRole('button',{name:'Thêm vào thư viện',exact:true}).click();
      // ActionSheet includes its description in the accessible button name.
      await expect(modal(p)).toHaveCount(1);await modal(p).getByRole('button',{name:/^Nhập mẫu video(?:\s|$)/}).click();
    }else await p.getByRole('button',{name:'Nhập mẫu video',exact:true}).click();
    const dialog=modal(p);await expect(dialog).toHaveCount(1);await expect(dialog).toHaveAccessibleName('Nhập mẫu video cũ');
    await dialog.locator('input[type="file"]').setInputFiles(importRows.map(row=>({name:row.file,mimeType:'application/zip',buffer:Buffer.from('504b0506000000000000000000000000000000000000','hex')})));
    const summary=mode==='done'?'Đã nhập 3 mẫu video. Mẫu video mới nằm đầu danh sách, chưa là mặc định của kênh nào.':'Đã nhập 2 mẫu video, 1 tệp chưa nhập được.';
    await expect(dialog.getByRole('status')).toHaveText(summary);expect(f.imports).toHaveLength(3);
    for(const row of importRows)await expect(dialog.getByText(row.file,{exact:true})).toBeVisible();
    await expect(dialog).toContainText('14 media vào kho media');await expect(dialog).toContainText('9 media mới, 3 media đã có dùng lại');
    await expect(dialog.getByText('2 điều cần biết',{exact:true})).toBeVisible();
    if(mode==='partial')await expect(dialog).toContainText('Chưa nhập được · SYNTHETIC fixture · tệp hỏng hoặc không phải bản xuất template');
    else await expect(dialog).toContainText('6 media mới, 1 media đã có dùng lại');
    await expect(dialog.getByRole('button',{name:'Nhập tệp khác',exact:true})).toBeVisible();await expect(dialog.getByRole('button',{name:'Xong',exact:true})).toBeEnabled();
    const posts=R.requests.filter(e=>e.case===r.key&&e.method!=='GET');expect(posts).toHaveLength(3);expect(posts.every(e=>e.action==='LOCAL_FULFILL_IMPORT_ONLY')).toBe(true);
    return {phase:mode,summary,locallyFulfilledPosts:3,sourceContract:'api/library.ts:344-361; TemplateImportDialog.tsx:82-114,157-223',archiveParsing:'UNPROVEN',backendPersistence:false,mobileExactCanonicalBoard:viewport.width===390?'ABSENT_FROM_MANIFEST':undefined};
  });
}
async function namedCase(index){
  const f=fixture(),channel=f.channels[index],definition=definitions[index];
  if(definition.key==='bilibili'){
    // Derived runtime future pause exercises actual Date.now-based pause logic.
    // Not a claim about the provider's real quota or the artboard's 02:10 time.
    channel.pausedUntil=new Date(Date.now()+3600_000).toISOString();channel.pauseReason=LABEL+' · canonical paused channel';
  }
  await run('fixture-1440-channel-'+definition.key,PC,['Library.kenh.'+definition.key],[],f,async(p,r)=>{
    await go(p,'/library?tab=backdrops&nguon=channels&kenh='+enc(channel.id));
    const pane=p.getByRole('region',{name:channel.title,exact:true});await expect(pane).toBeVisible();
    await expect(pane.getByRole('heading',{name:channel.title,exact:true})).toBeVisible();
    await expect(pane.getByRole('link',{name:channel.url.replace(/^https?:\/\//,''),exact:true})).toHaveAttribute('href',channel.url);
    const rail=p.locator('.vbd-rail');await expect(rail.locator('button[aria-current="true"]')).toContainText(channel.title);
    const list=pane.getByRole('list',{name:'Video của '+channel.title,exact:true});await expect(list).toBeVisible();
    await expect(list.getByText(catalogues.get(channel.id)[0].title,{exact:true})).toBeVisible();
    await expect(pane.locator('img,video')).toHaveCount(0);
    for(const usage of channel.usedBy)await expect(pane.locator('.vbd-used')).toContainText(usage.name);
    if(definition.key==='bilibili'){
      await expect(pane.locator('.vbd-status')).toHaveAttribute('data-tone','paused');await expect(pane.getByRole('button',{name:'Làm mới',exact:true})).toBeDisabled();
    }else await expect(pane.getByRole('button',{name:'Làm mới',exact:true})).toBeEnabled();
    if(definition.key==='nightdrivefm')await expect(list.locator('.vbd-pinc')).toHaveCount(4);
    if(definition.key==='lofilanes')await expect(pane.locator('.vbd-used-none')).toBeVisible();
    expect(R.requests.filter(e=>e.case===r.key&&e.method!=='GET')).toHaveLength(0);
    return {canonicalIdentity:definition.key,displayTitle:channel.title,counts:channel.counts,catalogueFixtureRows:catalogues.get(channel.id).length,canonicalMetadataSource:'Library.dc.html:1533-1552',typedContract:'api/backdrop.ts:43-100',nativeRecord:false,thumbnails:null,syntheticPauseUntil:channel.pausedUntil,noDiscoveryOrDownload:true};
  });
}
async function videoCase(){
  const f=fixture();
  await run('fixture-1440-library-addvideo',PC,['Library.nguon.addvideo'],[],f,async(p,r)=>{
    await go(p,'/library?tab=backdrops&nguon=videos');
    await expect(p.getByRole('heading',{name:'Video lẻ',exact:true})).toBeVisible();
    const card=p.locator('.vbd-vlcard').filter({hasText:videos[0].title});await expect(card).toBeVisible();
    await expect(card.getByRole('group',{name:'Đang tải '+videos[0].title,exact:true})).toBeVisible();
    await expect(card).toContainText('Đang tải 62% · còn khoảng 1 phút');
    await expect(card.locator('[role="progressbar"]')).toHaveAttribute('aria-valuenow','62');
    await expect(p.locator('video')).toHaveCount(0);await expect(p.locator('.vbd-vlcard')).toHaveCount(4);
    await expect(p.locator('.vbd-rail button[aria-current="true"]')).toContainText('Tất cả video lẻ');
    expect(R.requests.filter(e=>e.case===r.key&&e.method!=='GET')).toHaveLength(0);
    return {state:'downloading',progress:0.62,metadataOnly:true,nativeDownload:false,sourceContract:'api/backdrop.ts:137-152; VideosPage.tsx:34-53',canonicalSource:'Library.dc.html:1556-1561',readyCompanionFiles:'OMITTED: no fabricated media paths'};
  });
}
async function mobileBackdropCase(){
  const f=fixture();
  await run('fixture-390-library-backdrop-metadata-list',PHONE,['MLibrary.tab.backdrops'],[],f,async(p,r)=>{
    await go(p,'/library?tab=backdrops');
    for(const channel of f.channels)await expect(p.getByText(channel.title,{exact:true})).toBeVisible();
    await expect(p.getByText(videos[0].title,{exact:true})).toBeVisible();await expect(p.locator('video')).toHaveCount(0);
    expect(R.requests.filter(e=>e.case===r.key&&e.method!=='GET')).toHaveLength(0);
    return {namedFixturesVisible:f.channels.map(c=>c.title),canonicalScope:'MLibrary.tab.backdrops only',desktopNamedChannelAndDownloadDetailParity:'NOT_CLAIMED: mobile uses SourceChannelRow/SourceVideoRow instead of desktop ChannelPage/VideosPage',nativeBackend:false};
  });
}
async function unsupportedCase(){
  const f=fixture();f.unsupported=true;
  await run('fixture-1440-source-unsupported',PC,['Library.nguon.unsupported'],[],f,async(p,r)=>{
    await go(p,'/library?tab=backdrops');const input=p.locator('#vbd-link');
    await input.fill('https://example.invalid/SYNTHETIC-unsupported');await input.press('Enter');
    await expect(p.getByRole('alert')).toContainText('Chưa hỗ trợ SYNTHETIC nền tảng chưa hỗ trợ');
    await expect(p.getByRole('button',{name:'Gửi yêu cầu hỗ trợ SYNTHETIC nền tảng chưa hỗ trợ',exact:true})).toBeVisible();
    await expect(input).toBeEnabled();await expect(input).toHaveValue('https://example.invalid/SYNTHETIC-unsupported');
    return {parse:'locally fulfilled unsupported response',externalURLNeverResolved:true,backendSourceCreated:false};
  });
}
(async()=>{
  try{
    await fs.mkdir(OUT,{recursive:true});manifest=JSON.parse(await fs.readFile(MANIFEST,'utf8'));
    // Optional existing browser-session file supplied by Main. Never printed,
    // copied to reports, opened by tools during preparation, or used for APIs.
    if(process.env.EXTRA_STATES_COOKIES)cookies=JSON.parse(await fs.readFile(process.env.EXTRA_STATES_COOKIES,'utf8'));
    R.sources={root:SOURCE,artboard:ARTBOARD,manifest:MANIFEST};
    browser=await chromium.launch({executablePath:EXEC,headless:true});
    for(const viewport of [PC,PHONE])for(const mode of ['done','partial'])await importCase(viewport,mode);
    for(let i=0;i<definitions.length;i++)await namedCase(i);
    await videoCase();await mobileBackdropCase();await unsupportedCase();
    R.status=R.cases.some(c=>c.status!=='PASS')?'FAIL':'DONE_WITH_CONCERNS';
  }catch(e){R.status='BLOCKED';R.fatal=e.message;process.exitCode=1;}
  finally{
    try{if(browser)await browser.close();R.browserClosed=true;}catch(e){R.browserCloseError=e.message;process.exitCode=1;}
    R.finishedAt=new Date().toISOString();await fs.writeFile(path.join(OUT,'results.json'),JSON.stringify(R,null,2));
  }
  const counts=Object.fromEntries(['PASS','FAIL','NOT_RUN'].map(s=>[s,R.cases.filter(c=>c.status===s).length]));
  console.log(JSON.stringify({status:R.status,counts,pageErrors:R.pageErrors.length,results:path.join(OUT,'results.json')}));
  if(counts.FAIL||R.pageErrors.length||R.safety.downloads||R.safety.routeErrors.length)process.exitCode=1;
})().catch(e=>{console.error(e.message);process.exitCode=1;});
