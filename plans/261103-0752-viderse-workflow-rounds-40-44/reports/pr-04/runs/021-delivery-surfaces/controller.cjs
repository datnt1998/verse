const {createRequire}=require('node:module');
const {chromium,expect}=createRequire('/Users/dat/Projects/verse/package.json')('@playwright/test');
const fs=require('node:fs/promises');
const root=__dirname+'/wide-surfaces',origin='http://127.0.0.1:8043';
(async()=>{
const fixtureIds=new Set(['78799ed0-57fa-4cff-ab00-509bdc65e5da','98ae68b2-813e-427a-8238-8eb0e86de231','c76cc725-0e9d-43fd-98d1-4d55412510e8']);
 await fs.mkdir(root,{recursive:true});
 const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
 const results={captures:[],blocked:[],missing:[],errors:[]};
 try{
  for(const viewport of [{width:1440,height:900},{width:390,height:844}]){
   const context=await browser.newContext({viewport,deviceScaleFactor:1,...(viewport.width<768?{isMobile:true,hasTouch:true}:{})});
   try{
    const cookies=JSON.parse(await fs.readFile('/var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v36-owner-safe-yqz0cpa_/browser-cookies.json','utf8'));
    await context.addCookies(cookies.map(c=>({...c,domain:'127.0.0.1',secure:false})));
    await context.route('**/*',route=>{const r=route.request(),u=new URL(r.url());if(!['GET','HEAD','OPTIONS'].includes(r.method())){results.blocked.push({method:r.method(),path:u.pathname});return route.abort('blockedbyclient');}if(u.origin!==origin)return route.abort('blockedbyclient');return route.continue();});
    const page=await context.newPage();
    page.on('pageerror',error=>results.errors.push({viewport,message:error.message}));
    await page.goto(origin+'/#/library',{waitUntil:'domcontentloaded'});
    const fixtures=await page.evaluate(async()=>{const [c,t]=await Promise.all([fetch('/api/owner-channels'),fetch('/api/templates')]);if(!c.ok||!t.ok)throw new Error('Fixture read failed');return {channels:(await c.json()).channels,templates:(await t.json()).items};});
    const cases=[];
    for(const [key,name] of [['MT','Midnight Tales'],['QH','Quiet Hours'],['CL','Chuyện Lạ Đêm Khuya']]){
     const channel=fixtures.channels.find(c=>fixtureIds.has(c.id)&&c.name===name);
     if(!channel){results.missing.push({kind:'channel',key});continue;}
     for(const tab of ['sources','templates','settings'])cases.push({key:'channel-'+key+'-'+tab,path:'/#/channels/'+encodeURIComponent(channel.id)+'?tab='+tab,ready:()=>page.getByRole('tablist',{name:'Trang kênh',exact:true})});
    }
    for(const tab of ['templates','prompts','backdrops','overlays'])cases.push({key:'library-'+tab,path:'/#/library?tab='+tab,ready:()=>page.getByRole(viewport.width<768?'radiogroup':'tablist',{name:'Thư viện',exact:true})});
    cases.push({key:'create-channel',path:'/#/channels/new',ready:()=>viewport.width<768?page.getByText('Tạo kênh',{exact:true}).filter({visible:true}).first():page.getByRole('heading',{name:'Tạo kênh',exact:true})});
    const template=fixtures.templates.find(t=>t.name==='Mẫu chuẩn · Tách nền')??fixtures.templates[0];
    if(template)for(const [section,label] of [['images','Ảnh cảnh'],['cover','Ảnh bìa'],['backdrop','Nền & hiệu ứng'],['output','Xuất & kênh']])cases.push({key:'template-'+section,path:'/#/templates/'+encodeURIComponent(template.id)+'?section='+section,ready:()=>viewport.width<768?page.getByText(label,{exact:true}).first():page.getByRole('textbox',{name:'Tên mẫu video',exact:true})});
    else results.missing.push({kind:'template'});
    for(const entry of cases){
     await page.goto(origin+entry.path,{waitUntil:'domcontentloaded'});
     try{await entry.ready().waitFor({timeout:5000});}catch(error){await page.screenshot({path:root+'/failure.png'});results.failure={key:entry.key,viewport,url:page.url(),body:(await page.locator('body').innerText()).slice(0,3000)};throw error;}
     await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>Number.isFinite(a.effect?.getComputedTiming().endTime)).map(a=>a.finished.catch(()=>{})));await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));});
     const geometry=await page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth}));
     const filename=viewport.width+'-'+entry.key+'.png';
     await page.screenshot({path:root+'/'+filename});
     results.captures.push({viewport,key:entry.key,filename,geometry,overflow:geometry.scrollWidth>geometry.width});
    }
   }finally{await context.close();}
  }
  if(results.blocked.length||results.errors.length||results.captures.some(c=>c.overflow))throw new Error('Native surface gate failed; inspect JSON');
  results.status='passed';
 }finally{await browser.close();await fs.writeFile(root+'/results.json',JSON.stringify(results,null,2));}
 console.log(JSON.stringify({status:results.status,count:results.captures.length,missing:results.missing,blocked:results.blocked.length,errors:results.errors.length}));
})().catch(error=>{console.error(error);process.exitCode=1});
