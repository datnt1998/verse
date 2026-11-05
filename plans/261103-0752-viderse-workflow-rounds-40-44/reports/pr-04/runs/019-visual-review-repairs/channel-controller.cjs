'use strict';
// PREPARED ONLY: integration owner runs sequentially against guarded localhost:8043.
// Fixtures exercise the actual shipped React components; NOT backend persistence/provider proof.
const {createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs=require('node:fs/promises'),path=require('node:path');
const ROOT=__dirname,ORIGIN='http://127.0.0.1:8043';
const COOKIE_FILE='/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json';
const PC={width:1440,height:900},PHONE={width:390,height:844};
const R={status:'NOT_RUN',preparedOnly:false,proofClass:'SYNTHETIC_COMPONENT_UI_NOT_PERSISTENCE',cases:[],screenshots:[],requests:[],pageErrors:[],blockers:[],safety:{networkWritesContinued:0,externalRequestsContinued:0,inferenceCallsContinued:0,downloadEvents:0}};
let browser,cookies,native,map;
const clone=x=>JSON.parse(JSON.stringify(x)),enc=encodeURIComponent;
const endpoint=id=>'/api/owner-channels/'+enc(id);
const json=(route,body,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
const failPrerequisite=message=>{const e=new Error(message);e.unproven=true;throw e;};
const visibleModal=p=>p.locator('[aria-modal="true"]:visible');
async function settle(p){await p.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect?.getComputedTiming().endTime)).map(a=>a.finished.catch(()=>{})));await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));});}
async function badge(p){await p.evaluate(()=>{if(document.getElementById('v38-fixture-badge'))return;const b=document.createElement('div');b.id='v38-fixture-badge';b.setAttribute('aria-hidden','true');b.textContent='SYNTHETIC API FIXTURE · NO WRITES / AI · NOT PERSISTENCE';Object.assign(b.style,{position:'fixed',left:'4px',bottom:'4px',zIndex:'2147483647',pointerEvents:'none',font:'9px monospace',color:'#fff',background:'#6b165c',padding:'3px'});document.body.append(b);});}
async function shot(p,row,stage,ids=row.referenceIds){await settle(p);await badge(p);const file=`SYNTHETIC-${row.viewport.width}-${row.key}-${stage}.png`;const geometry=await p.evaluate(()=>({width:innerWidth,height:innerHeight,scrollWidth:document.documentElement.scrollWidth,modalCount:document.querySelectorAll('[aria-modal="true"]').length}));expect(geometry.scrollWidth,'horizontal overflow').toBeLessThanOrEqual(geometry.width+1);expect(await visibleModal(p).count(),'one modal layer maximum').toBeLessThanOrEqual(1);await p.screenshot({path:path.join(ROOT,file),animations:'disabled'});row.screenshots.push({file,stage,referenceIds:ids,geometry});for(const referenceId of ids)R.screenshots.push({referenceId,file,stage,case:row.key,viewport:row.viewport,proofClass:row.proofClass});return file;}
function forbidden(u){return /\/(?:export|download|downloads|localdownload|externaldownload|discover|discovery|follow)(?:\/|$)|\/api\/sources\/parse|\/api\/platforms\//i.test(u.pathname)||[...u.searchParams.keys()].some(k=>/^(download|export|attachment)$/i.test(k));}
async function context(viewport,label,fixture){const c=await browser.newContext({viewport,deviceScaleFactor:1,serviceWorkers:'block',acceptDownloads:false,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});try{await c.addCookies(cookies);await c.route('**/*',async route=>{const q=route.request(),u=new URL(q.url()),event={case:label,method:q.method(),path:u.pathname,queryKeys:[...u.searchParams.keys()]};if(u.origin!==ORIGIN){R.requests.push({...event,action:'ABORT_EXTERNAL'});return route.abort('blockedbyclient');}if(q.method()!=='GET'){if(fixture&&await fixture(route,u,event)){R.requests.push({...event,action:'SYNTHETIC_MUTATION_FULFILLED_BEFORE_NETWORK'});return;}R.requests.push({...event,action:'ABORT_UNLISTED_NON_GET'});return route.abort('blockedbyclient');}if(forbidden(u)){R.requests.push({...event,action:'ABORT_FORBIDDEN_GET'});return route.abort('blockedbyclient');}if(/\/(?:events|stream)$/.test(u.pathname)){R.requests.push({...event,action:'SYNTHETIC_EMPTY_EVENT_STREAM'});return route.fulfill({contentType:'text/event-stream',body:': private fixture isolation\n\n'});}if(fixture&&await fixture(route,u,event)){R.requests.push({...event,action:'SYNTHETIC_GET'});return;}if(/\.(?:mp4|webm|mp3|wav|m4a)(?:$|\?)/i.test(u.pathname)){R.requests.push({...event,action:'ABORT_MEDIA_GET'});return route.abort('blockedbyclient');}if(u.pathname.startsWith('/api/'))R.requests.push({...event,action:'NATIVE_READ_ONLY_GET'});return route.continue();});await c.routeWebSocket('**/*',s=>{R.requests.push({case:label,action:'ABORT_WEBSOCKET'});s.close();});return c;}catch(e){await c.close();throw e;}}
async function visit(p,hash){await p.goto(ORIGIN+'/#'+hash,{waitUntil:'domcontentloaded'});await badge(p);}
async function run(key,viewport,ids,config,fn){const row={key,viewport,referenceIds:ids,proofClass:'SYNTHETIC_API_COMPONENT_UI',status:'RUNNING',screenshots:[],fixture:config.description||'GET overrides on native owner/template IDs; no persisted change',oracles:[]};R.cases.push(row);let c,p;try{c=await context(viewport,key,config.route);if(config.init)await c.addInitScript(config.init);p=await c.newPage();p.setDefaultTimeout(10000);p.on('pageerror',e=>R.pageErrors.push({case:key,message:e.message}));p.on('download',d=>{R.safety.downloadEvents++;void d.cancel();});p.on('popup',page=>void page.close());await fn(p,row);row.status='PASS_COMPONENT_UI_ONLY';}catch(e){row.status=e.unproven?'UNPROVEN':'FAIL';row.reason=e.message;if(e.unproven)R.blockers.push({case:key,referenceIds:ids,reason:e.message});if(p)try{await shot(p,row,'failure');}catch(captureError){row.captureError=captureError.message;}}finally{if(c)await c.close();row.contextClosed=!!c;}}
async function inventory(){const c=await context(PC,'native-read-only-inventory');try{const p=await c.newPage();await p.goto(ORIGIN,{waitUntil:'domcontentloaded'});native=await p.evaluate(async()=>{const get=async u=>{const r=await fetch(u,{redirect:'error'});if(!r.ok)throw new Error('Inventory GET failed '+u+' '+r.status);return r.json();};const [owners,rivals,templates]=await Promise.all(['/api/owner-channels','/api/rivals','/api/templates'].map(get));return {channels:owners.channels.filter(c=>c.id!=='new'),rivals:rivals.rivals,templates:templates.items};});if(!native.channels.length)failPrerequisite('Native owner channel required; do not use ownerId=new.');if(!native.templates.length)failPrerequisite('Native template required.');R.nativeInventory={owners:native.channels.map(c=>({id:c.id,name:c.name,sourceIds:c.sources.map(s=>s.rivalId)})),rivalIds:native.rivals.map(r=>r.id),templateIds:native.templates.map(t=>t.id),excludedReservedOwnerId:'new'};}finally{await c.close();}}
function owner(index=0){
  const anchors=['78799ed0-57fa-4cff-ab00-509bdc65e5da','98ae68b2-813e-427a-8238-8eb0e86de231','c76cc725-0e9d-43fd-98d1-4d55412510e8'];
  const c=native.channels.find(c=>c.id===anchors[index]);
  if(!c)failPrerequisite('Original native QA owner '+anchors[index]+' missing; never substitute ownerId=new or a different owner.');
  return clone(c);
}
// Explicitly synthetic UUIDs only for rival/source component state, authorized by parent.
// Owner/template/sample IDs remain native. These records never reach a native endpoint.
function fixtureRivals(){
  const stamp='2026-11-05T00:00:00Z';
  const records=[
    {id:'38c10001-0000-4000-8000-000000000001',title:'SYNTHETIC Nocturne Stories',handle:'@fixture-nocturne'},
    {id:'38c10002-0000-4000-8000-000000000002',title:'SYNTHETIC Chuyện Đêm',handle:'@fixture-chuyen-dem'},
  ];
  return records.map((r,i)=>({...r,avatarUrl:null,url:ORIGIN+'/#fixture-only',subscriberCount:1240000-i*500000,videoCount:72+i,medianViews:25000,outliers:4,newOutliers:0,addedAt:stamp,scannedAt:stamp,scan:{state:'complete',discoveryRunning:false}}));
}
function sourceRows(){return fixtureRivals().map((r,i)=>({rivalId:r.id,title:r.title,handle:r.handle,linkedAt:'2026-11-05T00:00:00Z',reference:i===0}));}
async function sourceFixture(route,u){
  const match=u.pathname.match(/^\/api\/rivals\/(38c1000[12]-0000-4000-8000-00000000000[12])(?:\/(.*))?$/);
  if(!match)return false;
  const rival=fixtureRivals().find(r=>r.id===match[1]),stamp='2026-11-05T00:00:00Z';
  if(match[2]==='scan')await json(route,{channelId:rival.id,historyCap:150,indexedVideos:rival.videoCount,rescanHours:6,history:{state:'complete',jobId:null,scanned:rival.videoCount,added:rival.videoCount,pages:1,reachedEnd:true,startedAt:stamp,updatedAt:stamp,nextAttemptAt:null,errorCode:null,error:null,recovery:null},discovery:{checkedAt:stamp,caughtUp:true,running:false,nextAt:null}});
  else if(match[2]==='posting-slots')await json(route,{channelId:rival.id,zone:'vn',sample:28,total:28,pending:0,pausedUntil:null,perWeek:3.5,weeksTotal:8,offSlot:0,grid:Array.from({length:7},()=>Array(12).fill(0)),slots:[]});
  else if(match[2]==='videos')await json(route,{videos:[{id:'38c10003-0000-4000-8000-000000000003',channelId:rival.id,title:'SYNTHETIC latest video metadata — no media',durationSec:600,viewCount:10000,publishedAt:stamp,exactTime:true,score:3.2,hasCaptions:null,thumbnailUrl:'',url:ORIGIN+'/#fixture-video-only',usage:[]}],total:1,nextCursor:null});
  else await json(route,{error:'Unlisted synthetic rival resource',code:'fixture_unlisted'},404);
  return true;
}
function activity(channel,done=3,total=5){return {running:[{id:'fixture-identity-status',kind:'identity',agent:done?'artist':'director',title:'SYNTHETIC identity status',objectRef:{type:'channel',id:channel.id,label:channel.name||'Kênh mới'},phase:'SYNTHETIC identity phase',progress:{done,total},etaSec:60,state:'running',action:{label:'Mở kênh',href:'#/channels/'+enc(channel.id)}}],videoJobs:0,queued:0,unread:[]};}
function fixture(channel,{emptyOwners=false,rivals=fixtureRivals(),emptyJobs=false,task,mutation}={}){
  const list=emptyOwners?[]:native.channels.map(c=>c.id===channel.id?channel:c);
  return async(route,u,event)=>{
    if(event.method!=='GET')return mutation?mutation(route,u,event):false;
    if(await sourceFixture(route,u))return true;
    if(u.pathname==='/api/owner-channels'){await json(route,{channels:list});return true;}
    if(u.pathname===endpoint(channel.id)){await json(route,channel);return true;}
    if(u.pathname==='/api/rivals'){await json(route,{rivals:rivals.map(r=>({...r,avatarUrl:null})),totalVideos:rivals.reduce((n,r)=>n+(r.videoCount||0),0),newOutliers:0});return true;}
    if(u.pathname==='/api/activity'&&task){await json(route,task);return true;}
    if(u.pathname==='/api/clone'&&emptyJobs){await json(route,{jobs:[],queuedTotal:0,bulkEligibility:{retry:[],approve:[],bundle:[],markPosted:[],cancel:[],delete:[]}});return true;}
    return false;
  };
}
const initDone=id=>({content:`sessionStorage.setItem('viderse:last-created-channel',${JSON.stringify(id)});sessionStorage.setItem('viderse:onboard-done','1');`});
const refs=(board,...suffixes)=>suffixes.map(s=>board+'.'+s);
const templateControl=(p,vp)=>p.getByRole(vp.width<768?'button':'combobox',{name:'Chọn template',exact:true});
const templateChoices=(p,vp)=>p.getByRole(vp.width<768?'menuitemradio':'option');
const templateMenu=(p,vp)=>p.getByRole(vp.width<768?'menu':'listbox');
async function createProof(board,vp,onboard=false){const base=owner();const prefix=board.toLowerCase();const init=onboard?()=>{sessionStorage.removeItem('viderse:onboard-done');sessionStorage.removeItem('viderse:last-created-channel');}:()=>sessionStorage.setItem('viderse:onboard-done','1');
 await run(prefix+'-empty',vp,refs(board,'rivals.empty'),{init,route:fixture(base,{emptyOwners:onboard,rivals:[]})},async(p,row)=>{await visit(p,'/channels/new');await expect(p.getByRole('checkbox')).toHaveCount(0);await expect(p.getByRole('status').filter({hasText:'Chưa theo dõi đối thủ nào'})).toBeVisible();await expect(p.locator('[aria-live="polite"]').filter({hasText:'Chọn ít nhất 1 đối thủ để tạo kênh'})).toBeVisible();const submit=p.getByRole('button',{name:'Tạo kênh',exact:true});expect(await submit.getAttribute('aria-disabled')==='true'||await submit.isDisabled()).toBe(true);await expect(p.locator(vp.width<768?'#vch-new-name-m':'#vch-new-name')).toHaveValue('');row.oracles.push('Empty GET => zero checkboxes, real empty status, disabled create and blank optional name');await shot(p,row,'empty');});
 await run(prefix+'-none-selected',vp,refs(board,'rivals.none'),{init,route:fixture(base,{emptyOwners:onboard})},async(p,row)=>{const rivals=fixtureRivals();await visit(p,'/channels/new');await expect(p.getByRole('checkbox')).toHaveCount(rivals.length);expect(await p.getByRole('checkbox').evaluateAll(xs=>xs.every(x=>x.getAttribute('aria-checked')!=='true'&&!x.checked))).toBe(true);await expect(p.locator('[aria-live="polite"]').filter({hasText:'Chọn ít nhất 1 đối thủ để tạo kênh'})).toBeVisible();row.oracles.push('Synthetic followed rivals visible, none selected; explicit locally fulfilled UUIDs, NOT native source proof');await shot(p,row,'none-selected');});
 const selectedIds=refs(board,'step.create','rivals.some','tplMenu.false');
 await run(prefix+'-selected-defaults',vp,selectedIds,{init,route:fixture(base,{emptyOwners:onboard})},async(p,row)=>{const rivals=fixtureRivals();await visit(p,'/channels/new');if(onboard)await expect(p.getByRole('heading',{name:'Tạo kênh đầu tiên',exact:true})).toBeVisible();await p.getByRole('checkbox').first().click();await expect(p.getByRole('checkbox').first()).toBeChecked();await expect(p.getByText('Tham chiếu nhận diện',{exact:true})).toBeVisible();const name=p.locator(vp.width<768?'#vch-new-name-m':'#vch-new-name');await expect(name).toHaveValue('');const info=p.getByRole('region',{name:'Thông tin kênh'});const vi=/[ăâđêôơưàáảãạằắẳẵặầấẩẫậèéẻẽẹềếểễệìíỉĩịòóỏõọồốổỗộờớởỡợùúủũụừứửữựỳýỷỹỵ]/i.test(rivals[0].title);await expect(info.getByText(vi?'Tiếng Việt':'Tiếng Anh',{exact:true})).toBeVisible();await expect(p.getByRole('switch',{name:'Tự duyệt ảnh',exact:true})).toHaveAttribute('aria-checked','false');await expect(p.getByRole('spinbutton',{name:'Video mỗi ngày'})).toHaveValue('5');await expect(templateControl(p,vp)).toContainText('Mẫu chuẩn · gợi ý');await expect(p.locator('[aria-live="polite"]').filter({hasText:'Tạo xong, Hoạ sĩ'})).toBeVisible();row.oracles.push('Actual checkbox state/reference badge; language derived from first selected fixture rival; optional blank name; defaults 5/day, approve off, standard template; ready feedback; no native source proof');await shot(p,row,'selected-defaults');await name.fill('SYNTHETIC optional name');await expect(name).toHaveValue('SYNTHETIC optional name');await shot(p,row,'optional-name');const pace=p.getByRole('spinbutton',{name:'Video mỗi ngày'});await pace.fill('21');await pace.press('Enter');await expect(pace).toHaveValue('20');await expect(p.getByRole('button',{name:'Tăng',exact:true})).toBeDisabled();await shot(p,row,'max20');});
 await run(prefix+'-template-menu',vp,refs(board,'tplMenu.true'),{init,route:fixture(base,{emptyOwners:onboard})},async(p,row)=>{
   fixtureRivals();await visit(p,'/channels/new');await p.getByRole('checkbox').first().click();const trigger=templateControl(p,vp);
   await trigger.click();await expect(templateMenu(p,vp)).toHaveCount(1);await expect(templateChoices(p,vp).filter({hasText:native.templates[0].name})).toBeVisible();
   if(vp.width<768)await expect(visibleModal(p)).toHaveCount(1);await shot(p,row,'template-menu');
   await p.keyboard.press('Escape');await expect(templateMenu(p,vp)).toHaveCount(0);await expect(trigger).toBeFocused();
   row.oracles.push('Desktop select/mobile radio action menu expanded with native options, one mobile modal, Escape closes and returns focus');
 });
 await run(prefix+'-submit-error-then-created',vp,refs(board,'step.done'),{init,route:null},async(p,row)=>{fixtureRivals();let calls=0;const created=owner();created.name='SYNTHETIC created channel';created.identity={state:'generating',part:'all'};created.avatarUrl=null;created.bannerUrl=null;created.sources=[{rivalId:fixtureRivals()[0].id,title:fixtureRivals()[0].title,handle:fixtureRivals()[0].handle,reference:true,linkedAt:new Date().toISOString()}];const handler=fixture(created,{emptyOwners:onboard,task:activity(created),mutation:async(route,u,e)=>{if(e.method==='POST'&&u.pathname==='/api/owner-channels'){calls++;const body=route.request().postDataJSON();expect(body.competitorIds).toEqual([fixtureRivals()[0].id]);expect(body.perDay).toBe(5);expect(body.autoApprove).toBe(false);expect(body.defaultTemplateId).toBeUndefined();expect(body.name).toBeUndefined();row.oracles.push('POST '+calls+' fenced before networking; request payload matched actual selected/default state');if(calls===1)await json(route,{error:'SYNTHETIC creation denied',code:'fixture_denied'},400);else await json(route,{channel:created,identityJobId:'fixture-status-only'},201);return true;}return false;}});await p.context().route('**/*',async route=>{const q=route.request(),u=new URL(q.url());if(u.origin===ORIGIN&&await handler(route,u,{method:q.method()})){R.requests.push({case:row.key,method:q.method(),path:u.pathname,action:'SYNTHETIC_SUBMIT_SCENARIO'});return;}await route.fallback();});await visit(p,'/channels/new');await p.getByRole('checkbox').first().click();await p.getByRole('button',{name:'Tạo kênh',exact:true}).click();await expect(p.getByRole('alert')).toContainText('SYNTHETIC creation denied');await expect(p.getByRole('checkbox').first()).toBeChecked();await shot(p,row,'create-error');await p.getByRole('button',{name:'Tạo kênh',exact:true}).click();await expect(p).toHaveURL(/channels\/new\?step=done/);await expect(p.getByRole('heading',{name:'Kênh đã sẵn sàng',exact:true})).toBeVisible();expect(calls).toBe(2);await shot(p,row,'created-route');row.oracles.push('400 visible alert retains selection; second synthetic POST routes real component to done using native owner ID (not creation persistence)');});
 await run(prefix+'-custom-settings',vp,refs(board,'step.create'),{init,route:null},async(p,row)=>{
   const c=owner(),rivals=fixtureRivals();let received=false;
   const handler=fixture(c,{emptyOwners:onboard,mutation:async(route,u,e)=>{
     if(e.method!=='POST'||u.pathname!=='/api/owner-channels')return false;
     expect(route.request().postDataJSON()).toMatchObject({name:'SYNTHETIC named channel',competitorIds:[rivals[1].id],perDay:20,autoApprove:true,defaultTemplateId:native.templates[0].id});received=true;
     await json(route,{error:'SYNTHETIC payload checked; creation intentionally fenced',code:'fixture_payload'},400);return true;
   }});
   await p.context().route('**/*',async route=>{const q=route.request(),u=new URL(q.url());if(u.origin===ORIGIN&&await handler(route,u,{method:q.method()})){R.requests.push({case:row.key,method:q.method(),path:u.pathname,action:'SYNTHETIC_CUSTOM_SETTINGS'});return;}await route.fallback();});
   await visit(p,'/channels/new');
   await p.getByRole('checkbox').nth(1).click();await expect(p.getByRole('checkbox').nth(1)).toBeChecked();
   await expect(p.getByRole('region',{name:'Thông tin kênh'}).getByText('Tiếng Việt',{exact:true})).toBeVisible();
   await p.locator(vp.width<768?'#vch-new-name-m':'#vch-new-name').fill('  SYNTHETIC named channel  ');
   const pace=p.getByRole('spinbutton',{name:'Video mỗi ngày'});await pace.fill('21');await pace.press('Enter');
   await expect(pace).toHaveValue('20');await expect(p.getByRole('button',{name:'Tăng',exact:true})).toBeDisabled();
   await p.getByRole('switch',{name:'Tự duyệt ảnh',exact:true}).click();
   await expect(p.getByRole('switch',{name:'Tự duyệt ảnh',exact:true})).toHaveAttribute('aria-checked','true');
   const trigger=templateControl(p,vp);await trigger.click();
   await templateChoices(p,vp).filter({hasText:native.templates[0].name}).click();
   await expect(templateMenu(p,vp)).toHaveCount(0);await expect(trigger).toContainText(native.templates[0].name);
   await shot(p,row,'custom-settings');
   await p.getByRole('button',{name:'Tạo kênh',exact:true}).click();
   await expect(p.getByRole('alert')).toContainText('SYNTHETIC payload checked');expect(received).toBe(true);
   await shot(p,row,'custom-settings-fenced-feedback');
   row.oracles.push('Vietnamese reference, trimmed optional name, selected native template, approval on and clamped20 match fenced POST; actual rejection retains form');
 });
}
async function doneProof(board,vp,state){
  const c=Object.assign(owner(),{sources:sourceRows()});
  c.avatarUrl=null;c.bannerUrl=null;
  c.identity=state==='running'?{state:'generating',part:'all'}:{state:'failed',part:state==='director'?'name':'all',error:{code:'fixture_error',message:state==='director'?'SYNTHETIC text failure':'SYNTHETIC image quota failure',step:state==='director'?'text':'images'},retryAt:'2026-11-05T07:00:00Z'};
  if(state==='director')c.name='';
  let retries=0;
  const base=fixture(c,{task:activity(c,state==='director'?0:3),mutation:async(route,u,e)=>{
    if(e.method!=='POST'||u.pathname!==endpoint(c.id)+'/branding')return false;
    expect(route.request().postDataJSON()).toEqual({part:'name'});retries++;
    if(retries===1)await json(route,{error:'SYNTHETIC retry unavailable',code:'fixture_retry'},503);
    else{c.identity={state:'generating',part:'name'};await json(route,{channel:c,identityJobId:'fixture-status-only'},202);}
    return true;
  }});
  await run(board.toLowerCase()+'-done-'+state,vp,refs(board,'identity.'+state,...(state==='running'?['step.done']:[])),{init:initDone(c.id),route:base},async(p,row)=>{
    await visit(p,'/channels/new?step=done');await expect(p.locator('.vch-task')).toHaveCount(3);
    if(state==='running'){
      await expect(p.getByRole('heading',{name:'Kênh đã sẵn sàng',exact:true})).toBeVisible();
      await expect(p.locator('.vch-task').nth(0)).toContainText('Xong');
      await expect(p.locator('.vch-task').nth(1)).toContainText('60%');
      await expect(p.locator('.vch-task').nth(1)).toContainText('SYNTHETIC identity phase');
    }else if(state==='failed'){
      await expect(p.locator('.vch-task').nth(0)).toContainText('Xong');
      await expect(p.locator('.vch-task').nth(1)).toContainText('SYNTHETIC image quota failure');
      await expect(p.locator('.vch-task').nth(1)).toContainText('Tự thử lại');
    }else{
      await expect(p.getByRole('heading',{name:'Đã tạo kênh',exact:true})).toBeVisible();
      await expect(p.getByRole('region',{name:'Một việc cần bạn'})).toBeVisible();
      await expect(p.locator('.vch-task').nth(0)).toContainText('SYNTHETIC text failure');
      await expect(p.locator('.vch-task').nth(1)).toContainText('chờ Đạo diễn');
    }
    await expect(p.getByText('SYNTHETIC latest video metadata — no media',{exact:true})).toBeVisible();
    row.oracles.push('DoneStep task rows derive from identity.error.step and activity; latest metadata is local fixture, not downloaded video');
    await shot(p,row,'identity-'+state);
    if(state==='director'){
      await p.getByRole('button',{name:'Thử lại',exact:true}).click();
      await expect(p.getByRole('alert')).toContainText('SYNTHETIC retry unavailable');await shot(p,row,'retry-error');
      await p.getByRole('button',{name:'Thử lại',exact:true}).click();
      await expect(p.getByRole('heading',{name:'Kênh đã sẵn sàng',exact:true})).toBeVisible();
      await expect(p.locator('.vch-task').first()).toContainText('SYNTHETIC identity phase');
      expect(retries).toBe(2);await shot(p,row,'retry-generating');
    }
    await p.getByRole('button',{name:'Mở kênh',exact:true}).click();
    await expect(p).toHaveURL(new RegExp('/channels/'+c.id+'(?:\\?|$)'));
    await shot(p,row,'opened-owner'); row.oracles.push('Open-channel route uses concrete native owner ID; no ownerId=new ambiguity');
  });
}
async function channelProof(vp,state){
  const c=Object.assign(owner(),{sources:sourceRows()});c.avatarUrl=null;c.bannerUrl=null;
  c.identity=state==='failed'?{state:'failed',part:'all',error:{step:'images',code:'fixture_quota',message:'SYNTHETIC image quota failure'},retryAt:'2026-11-05T07:00:00Z'}:{state};
  let retries=0;
  await run('channel-identity-'+state,vp,refs(vp.width<768?'MChannel':'Channel','identity.'+state),{route:fixture(c,{task:activity(c),mutation:async(route,u,e)=>{
    if(e.method!=='POST'||u.pathname!==endpoint(c.id)+'/branding')return false;
    expect(route.request().postDataJSON().part).toBe('all');retries++;
    await json(route,{error:'SYNTHETIC image retry fenced',code:'fixture_retry'},503);return true;
  }})},async(p,row)=>{
    await visit(p,'/channels/'+enc(c.id)+'?tab=settings');await expect(p.locator('#vch-brief')).toBeVisible();
    if(state==='failed'){
      await expect(p.getByRole('alert').filter({hasText:'SYNTHETIC image quota failure'})).toBeVisible();
      await expect(p.getByText(/Tự thử lại lúc/)).toBeVisible();await shot(p,row,'failed');
      await p.getByRole('button',{name:'Thử lại',exact:true}).click();
      await expect(p.getByRole('alert').filter({hasText:'SYNTHETIC image retry fenced'})).toBeVisible();
      expect(retries).toBe(1);await shot(p,row,'retry-error');
      row.oracles.push('Failed-state alert with retryAt; intercepted retry part=all yields visible 503 feedback');
    }else{
      await expect(p.getByRole('status').filter({hasText:'Hoạ sĩ đang vẽ avatar và ảnh bìa kênh…'})).toBeVisible();
      await expect(p.getByRole('button',{name:'Vẽ lại avatar · 1 ảnh',exact:true})).toHaveAttribute('aria-disabled','true');
      await shot(p,row,'generating');row.oracles.push('Generating status and regeneration lock from actual component');
    }
  });
}
async function channelExtras(vp){
  const board=vp.width<768?'MChannel':'Channel',c=owner();
  const baseline=Object.assign(owner(),{sources:sourceRows(),autoApprove:false,createdAt:'2020-01-01T00:00:00Z'});
  await run('channel-baseline',vp,refs(board,'fresh.false',...(vp.width===1440?['autoApprove.false']:[])),{route:fixture(baseline,{emptyJobs:true})},async(p,row)=>{
    await visit(p,'/channels/'+enc(baseline.id)+'?tab=sources');
    await expect(p.getByRole('switch',{name:'Tự duyệt ảnh',exact:true})).toHaveAttribute('aria-checked','false');
    await expect(p.getByRole('tab',{name:/Nguồn/})).toContainText('2');
    await expect(p.getByText('tạo hôm nay',{exact:true})).toHaveCount(0);
    row.oracles.push('Explicit approval-off oracle; old createdAt has no today label; concrete owner and two locally fulfilled sources');
    row.canonicalCaveat='Nonfresh is derived from data, not a flag; this proves old-created state, not the canonical eighteen-job sample inventory.';
    await shot(p,row,'baseline');
  });
  c.sources=[];c.autoApprove=true;c.youtubeUrl=null;c.createdAt=new Date().toISOString();
  await run('channel-empty-source',vp,refs(board,vp.width===1440?'link.empty':'sheet.link'),{route:fixture(c,{rivals:[],emptyJobs:true})},async(p,row)=>{
    await visit(p,'/channels/'+enc(c.id)+'?tab=sources');
    await expect(p.getByRole('status').filter({hasText:'Chưa liên kết đối thủ nào'})).toBeVisible();
    await expect(p.getByRole('switch',{name:'Tự duyệt ảnh',exact:true})).toHaveAttribute('aria-checked','true');
    await expect(p.getByText(vp.width<768?'Kênh chưa có video · xem Bàn dựng':'Kênh chưa có video. Video của kênh nằm ở Bàn dựng, nhóm theo Chờ bạn · Đang làm · Sẵn sàng.',{exact:true})).toBeVisible();
    row.oracles.push('Source-empty GET => paused/no-source status and real zero-jobs list note');await shot(p,row,'empty-source');
    await p.getByRole('button',{name:'Liên kết đối thủ',exact:true}).first().click();
    await expect(p.getByText('Chưa theo dõi đối thủ nào. Theo dõi đối thủ ở Khám phá trước.',{exact:true})).toBeVisible();
    await shot(p,row,'empty-link-picker');await p.keyboard.press('Escape');await expect(p.getByRole('dialog')).toHaveCount(0);
  });
  const fresh=Object.assign(owner(),{sources:sourceRows(),youtubeUrl:null,createdAt:new Date().toISOString(),autoApprove:vp.width===1440});
  await run('channel-fresh',vp,refs(board,'fresh.true',...(vp.width===1440?['autoApprove.true']:[])),{route:fixture(fresh,{emptyJobs:true})},async(p,row)=>{
    await visit(p,'/channels/'+enc(fresh.id)+'?tab=sources');
    await expect(p.getByText('Chưa liên kết YouTube',{exact:false}).first()).toBeVisible();
    await expect(p.getByRole('tab',{name:/Nguồn/})).toContainText('2');
    await expect(p.getByText(vp.width<768?'Kênh chưa có video · xem Bàn dựng':'Kênh chưa có video. Video của kênh nằm ở Bàn dựng, nhóm theo Chờ bạn · Đang làm · Sẵn sàng.',{exact:true})).toBeVisible();
    await expect(p.getByRole('switch',{name:'Tự duyệt ảnh',exact:true})).toHaveAttribute('aria-checked',vp.width===1440?'true':'false');
    row.oracles.push('Fresh data projection: today/no YouTube/zero jobs/two locally fulfilled sources; actual approval state');
    await shot(p,row,'fresh');
  });
  if(vp.width===1440)for(const [index,code]of ['MT','QH','CL'].entries()){
    await run('channel-owner-'+code,vp,refs('Channel','channel.'+code),{},async(p,row)=>{
      const ch=owner(index);await visit(p,'/channels/'+enc(ch.id)+'?tab=sources');
      await expect(p.getByRole('heading',{name:ch.name,exact:true})).toBeVisible();expect(new URL(p.url()).hash).toContain(ch.id);
      row.oracles.push('Concrete native owner route and owner heading');
      const expected={MT:'Midnight Tales',CL:'Chuyện Lạ Đêm Khuya',QH:'Quiet Hours'}[code];
      if(ch.name!==expected){row.canonicalCaveat='Native owner name differs from '+expected+'; slot is not exact canonical persona';R.blockers.push({case:row.key,reason:row.canonicalCaveat});}
      await shot(p,row,'owner');
    });
  }
  if(vp.width>=768)return;
  const source=Object.assign(owner(),{sources:sourceRows()});
  await run('channel-mobile-source-sheets',vp,refs(board,'sheet.rival','sheet.unlink'),{route:fixture(source)},async(p,row)=>{
    const rival=fixtureRivals()[0];await visit(p,'/channels/'+enc(source.id)+'?tab=sources');
    await p.getByRole('button',{name:'Bỏ liên kết: '+rival.title,exact:true}).click();
    await expect(visibleModal(p)).toHaveCount(1);await expect(visibleModal(p).getByRole('button',{name:/^Bỏ liên kết…/})).toBeVisible();
    await shot(p,row,'rival-sheet',refs(board,'sheet.rival'));
    await visibleModal(p).getByRole('button',{name:/^Bỏ liên kết…/}).click();
    await expect(visibleModal(p)).toHaveCount(1);await expect(visibleModal(p)).toContainText('Video đã làm và video đang chờ lượt giữ nguyên');
    await shot(p,row,'unlink-confirm',refs(board,'sheet.unlink'));
    await visibleModal(p).getByRole('button',{name:'Huỷ',exact:true}).click();await expect(visibleModal(p)).toHaveCount(0);
    row.oracles.push('Real action-sheet to confirmation transition, exactly one modal layer, cancel without DELETE');
  });
}
function setupState(state){const ids=['yt-dlp','ffmpeg','transcription','chrome','codex','codex-login'];return {state,complete:false,running:state==='running',disk:{freeBytes:100000000000,requiredBytes:10000000000,approxBytes:4000000000,enough:true},steps:ids.map((id,i)=>({id,state:state==='login'?(i===5?'needs_user':'done'):(i<2?'done':i===2?(state==='failed'?'failed':'running'):i===5?'needs_user':'pending'),progress:i===2&&state!=='login'?{phase:'fixture_download',phaseLabel:'SYNTHETIC download phase',bytesDone:500000000,bytesTotal:1000000000,pct:50,startedAt:'2026-11-05T00:00:00Z'}:null,error:i===2&&state==='failed'?'SYNTHETIC network connection interrupted':null,sizeBytes:1000000000,provided:id==='chrome'}))};}
async function setupProof(state){
  let data=setupState(state),retries=0;
  await run('onboard-setup-'+state,PC,refs('Onboard','setup.'+state,...(state==='running'?['step.setup']:[])),{route:async(route,u,e)=>{
    if(e.method==='GET'&&u.pathname==='/api/setup/state'){await json(route,data);return true;}
    if(e.method==='POST'&&u.pathname==='/api/setup/retry'&&state==='failed'){
      expect(route.request().postDataJSON()).toEqual({id:'transcription'});retries++;
      if(retries===1)await json(route,{error:'SYNTHETIC setup retry fenced',code:'fixture_retry'},503);
      else{data=setupState('running');await json(route,data,202);}
      return true;
    }
    return false;
  }},async(p,row)=>{
    await visit(p,'/setup');await expect(p.locator('.vv-setup__row')).toHaveCount(6);
    if(state==='running'){
      await expect(p.locator('.vv-setup__row.is-running')).toContainText('SYNTHETIC download phase');
      await expect(p.locator('.vv-setup__row.is-running [role="progressbar"]')).toHaveAttribute('aria-valuenow','50');
    }else if(state==='login'){
      await expect(p.locator('.vv-setup__row.is-focus')).toHaveCount(1);
      await expect(p.locator('.vv-setup__row.is-focus').getByRole('button')).toBeVisible();
      row.oracles.push('Login visible; no OAuth/provider/start action clicked');
    }else{
      await expect(p.locator('.vv-setup__row.is-failed')).toHaveCount(1);
      await expect(p.locator('.vv-setup__row.is-failed')).toContainText('Mạng');
    }
    await shot(p,row,'setup-'+state);
    if(state==='failed'){
      await p.locator('.vv-setup__row.is-failed').getByRole('button').click();
      await expect(p.locator('.vv-setup__row.is-failed [role="alert"]')).toContainText('Chưa thử lại được');
      await expect(p.locator('.vv-setup__row.is-failed [role="alert"]')).not.toContainText('SYNTHETIC setup retry fenced');
      expect(retries).toBe(1);await shot(p,row,'setup-retry-503');
      await p.locator('.vv-setup__row.is-failed').getByRole('button').click();
      await expect(p.locator('.vv-setup__row.is-failed')).toHaveCount(0);
      await expect(p.locator('.vv-setup__row.is-running')).toContainText('SYNTHETIC download phase');
      await expect(p.locator('.vv-setup__row.is-running [role="progressbar"]')).toHaveAttribute('aria-valuenow','50');
      await expect(p.locator('.vv-setup__row [role="alert"]')).toHaveCount(0);
      expect(retries).toBe(2);await shot(p,row,'setup-retry-success');
      row.oracles.push('Fenced503 renders human inline alert (not raw503); retry success immediately applies returned setup snapshot without SSE or native install');
    }
    row.oracles.push('Six actual setup rows and status branch; zero installation/download requests continued');
  });
}
(async()=>{R.startedAt=new Date().toISOString();try{await fs.mkdir(ROOT,{recursive:true});map=JSON.parse(await fs.readFile(path.join(ROOT,'coverage-map.json'),'utf8'));cookies=JSON.parse(await fs.readFile(COOKIE_FILE,'utf8')).map(c=>({...c,domain:'127.0.0.1',secure:false}));browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});await inventory();await createProof('CreateChannel',PC);await createProof('MCreate',PHONE);await createProof('Onboard',PC,true);for(const [board,vp]of [['CreateChannel',PC],['MCreate',PHONE],['Onboard',PC]])for(const state of ['running','failed','director'])await doneProof(board,vp,state);for(const vp of [PC,PHONE]){await channelExtras(vp);await channelProof(vp,'generating');await channelProof(vp,'failed');}for(const state of ['running','login','failed'])await setupProof(state);R.coverage=map.rows.map(m=>({...m,runEvidence:R.screenshots.filter(s=>s.referenceId===m.referenceId),runCases:R.cases.filter(c=>c.referenceIds.includes(m.referenceId)).map(c=>({case:c.key,status:c.status,reason:c.reason,canonicalCaveat:c.canonicalCaveat}))}));R.status=R.cases.some(c=>c.status==='FAIL')||R.pageErrors.length||R.safety.downloadEvents?'FAIL':(R.blockers.length || map.rows.some(m=>m.namedBlocker))?'DONE_WITH_CONCERNS':'PASS_COMPONENT_UI_ONLY';if(R.status==='FAIL')process.exitCode=1;}catch(e){R.status='BLOCKED';R.error=e.message;process.exitCode=1;}finally{if(browser)try{await browser.close();R.browserClosed=true;}catch(e){R.browserClosed=false;R.closeError=e.message;R.status='FAIL';process.exitCode=1;}R.finishedAt=new Date().toISOString();await fs.writeFile(path.join(ROOT,'results.json'),JSON.stringify(R,null,2));}console.log(JSON.stringify({status:R.status,results:path.join(ROOT,'results.json'),cases:R.cases.map(c=>({key:c.key,status:c.status,reason:c.reason})),browserClosed:R.browserClosed},null,2));})().catch(e=>{console.error(e.message);process.exitCode=1;});
