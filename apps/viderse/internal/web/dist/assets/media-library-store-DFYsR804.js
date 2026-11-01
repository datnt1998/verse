import{c as Oe,a as Lt}from"./export-render.worker-8NI-VOsY.js";import{g as nt,i as dh,S as Jn,a as mh,o as y,j as _i,s as w,l as Z,n as M,_ as z,d as Oi,b as F,c as B,r as kt,e as Zn,t as An,u as dr,f as nd}from"./storage-backend-XJH0Ed_z.js";import{c as Ie,Z as ph,d as fh,u as ae,D as yr,g as hh,e as gh,E as Ml,f as yh,n as Cl,h as vh}from"./compositions-store-W6Flo6Er.js";const ad=1920,id=1080,Jr=new Map;function jo(e,t,r){Jr.set(e,{blob:t,metadata:r})}function iT(e){return Jr.get(e)?.blob??null}function oT(e){return Jr.get(e)?.metadata??null}function sT(e){return Jr.has(e)}function lT(e){const t=Jr.get(e)?.metadata;return t?.opfsPath?{storageType:"opfs",opfsPath:t.opfsPath,fileSize:t.fileSize}:null}function od(e){Jr.delete(e)}const Tl=.999;function Tr(e){return!Number.isFinite(e)||e<=0?0:e>=1?1:e}function Ms(e){return Number.isFinite(e)?e<=-1?-1:e>=1?1:e:0}function El(e,t){const r=e+t;if(r<=Tl)return[e,t];if(r<=0)return[0,0];const n=Tl/r;return[e*n,t*n]}function Fl(e,t,r){if(!Number.isFinite(r)||r<=0)return[0,0];const n=Math.max(0,Math.min(e,r)),a=Math.max(0,Math.min(t,r)),i=n+a;if(i<=r)return[n,a];if(i<=0)return[0,0];const o=r/i;return[n*o,a*o]}function bh(e,t){const r=Number.isFinite(e)&&e>0?e:0,n=Number.isFinite(t)&&t>0?t:0;return r===0||n===0?Math.max(r,n):Math.min(r,n)}function Cs(e){const[t,r]=El(Tr(e?.left??0),Tr(e?.right??0)),[n,a]=El(Tr(e?.top??0),Tr(e?.bottom??0)),i=Ms(e?.softness??0);return{left:t,right:r,top:n,bottom:a,softness:i}}function Sh(e){if(!e)return;const t=Cs(e);if(!(t.left===0&&t.right===0&&t.top===0&&t.bottom===0))return t}function cT(e){const t=Cs(e);return t.left>0||t.right>0||t.top>0||t.bottom>0}function uT(e,t){return!Number.isFinite(t)||t<=0?0:Tr(e??0)*t}function dT(e,t){return!Number.isFinite(t)||t<=0?0:Tr(e/t)}function Ih(e,t){return!Number.isFinite(t)||t<=0?0:Ms(e??0)*t}function mT(e,t){return!Number.isFinite(t)||t<=0?0:Ms(e/t)}function sd(e,t,r,n){if(!Number.isFinite(e)||e<=0||!Number.isFinite(t)||t<=0||!Number.isFinite(r)||r<=0||!Number.isFinite(n)||n<=0)return{x:0,y:0,width:Math.max(0,r),height:Math.max(0,n)};const a=Math.min(r/e,n/t),i=e*a,o=t*a;return{x:(r-i)/2,y:(n-o)/2,width:i,height:o}}function xh(e){return Number.isFinite(e)&&e>0}function wh(e,t,r,n,a){const i=Math.max(1e-6,1-a.left-a.right),o=Math.max(1e-6,1-a.top-a.bottom),s=e*i,l=t*o,c=sd(s,l,r,n),u=c.width/s;return{x:c.x-e*a.left*u,y:c.y-t*a.top*u,width:e*u,height:t*u}}function kh(e,t,r,n,a,i,o){const s=[e,t,r,n].every(xh);return a?.refit===!0&&s?wh(e,t,r,n,i):o==="fill"?{x:0,y:0,width:Math.max(0,r),height:Math.max(0,n)}:sd(e,t,r,n)}function Ph(e,t,r,n,a,i="contain"){const o=Cs(a),s=kh(e,t,r,n,a,o,i),l={left:s.width*o.left,right:s.width*o.right,top:s.height*o.top,bottom:s.height*o.bottom},c={x:s.x+l.left,y:s.y+l.top,width:Math.max(0,s.width-l.left-l.right),height:Math.max(0,s.height-l.top-l.bottom)},u=Ih(o.softness,bh(s.width,s.height)),d=Math.abs(u),m=u>0?{left:l.left>0?Math.min(d,l.left):0,right:l.right>0?Math.min(d,l.right):0,top:l.top>0?Math.min(d,l.top):0,bottom:l.bottom>0?Math.min(d,l.bottom):0}:{left:0,right:0,top:0,bottom:0},p=c.x-m.left,f=c.y-m.top,h=p+Math.max(0,c.width+m.left+m.right),g=f+Math.max(0,c.height+m.top+m.bottom),v={x:Math.floor(p),y:Math.floor(f),width:Math.ceil(h)-Math.floor(p),height:Math.ceil(g)-Math.floor(f)},S={left:l.left>0?u>0?m.left:d:0,right:l.right>0?u>0?m.right:d:0,top:l.top>0?u>0?m.top:d:0,bottom:l.bottom>0?u>0?m.bottom:d:0},[b,x]=Fl(S.left,S.right,v.width),[T,C]=Fl(S.top,S.bottom,v.height);return{mediaRect:s,cropViewportRect:c,viewportRect:v,cropPixels:l,crop:o,softnessPixels:d,featherPixels:{left:b,right:x,top:T,bottom:C}}}const Mh=/^(?:(\d{1,2}):)?(\d{1,2}):(\d{2})([,.])(\d{1,3})\s+-->\s+(?:(\d{1,2}):)?(\d{1,2}):(\d{2})([,.])(\d{1,3})(?:\s+.*)?$/;function Al(e){return ld(cd(e).replace(/\r\n?/g,`
`),"srt")}function _l(e){const r=cd(e).replace(/\r\n?/g,`
`).replace(/^WEBVTT(?:[^\n]*)\n+/,"");return ld(r,"vtt")}function pT(e){return`WEBVTT

${Eh(e).map(r=>`${Dl(r.startSeconds)} --> ${Dl(r.endSeconds)}
${r.text}`).join(`

`)}`}function ld(e,t){const r=[],n=[],a=e.split(/\n{2,}/).map(o=>o.trim()).filter(Boolean);for(const[o,s]of a.entries()){const l=s.split(`
`),c=l.findIndex(p=>p.includes("-->"));if(c===-1){Ah(l[0]??"",t)||r.push(`Skipped block ${o+1}: missing timestamp`);continue}const u=Ch(l[c].trim());if(!u){r.push(`Skipped block ${o+1}: invalid timestamp`);continue}if(u.endSeconds<=u.startSeconds){r.push(`Skipped block ${o+1}: end time must be after start time`);continue}const m=l.slice(c+1).map(p=>Fh(p).trimEnd()).join(`
`).trim();if(!m){r.push(`Skipped block ${o+1}: empty cue text`);continue}n.push({id:`cue-${n.length+1}`,startSeconds:u.startSeconds,endSeconds:u.endSeconds,text:m})}const i=[...n].sort((o,s)=>o.startSeconds-s.startSeconds);for(let o=1;o<i.length;o++){const s=i[o-1];i[o].startSeconds<s.endSeconds&&r.push(`Cue ${o+1} overlaps the previous cue`)}return{cues:i,warnings:r}}function Ch(e){const t=e.match(Mh);if(!t)return null;const r=Ol(t[1],t[2],t[3],t[5]),n=Ol(t[6],t[7],t[8],t[10]);return!Number.isFinite(r)||!Number.isFinite(n)?null:{startSeconds:r,endSeconds:n}}function Ol(e,t,r,n){const a=Number.parseInt(e??"0",10),i=Number.parseInt(t??"0",10),o=Number.parseInt(r??"0",10),s=Number.parseInt((n??"0").padEnd(3,"0").slice(0,3),10);return a*3600+i*60+o+s/1e3}function Dl(e){return Th(e,".")}function Th(e,t){const r=Math.max(0,Math.round(e*1e3)),n=r%1e3,a=Math.floor(r/1e3),i=a%60,o=Math.floor(a/60),s=o%60,l=Math.floor(o/60);return`${Gi(l)}:${Gi(s)}:${Gi(i)}${t}${String(n).padStart(3,"0")}`}function Eh(e){return e.filter(t=>t.text.trim().length>0&&t.endSeconds>t.startSeconds).map(t=>({...t,text:t.text.trim()})).sort((t,r)=>t.startSeconds-r.startSeconds)}function cd(e){return e.charCodeAt(0)===65279?e.slice(1):e}function Fh(e){return e.replace(/<[^>]+>/g,"")}function Ah(e,t){return t!=="vtt"?!1:e.startsWith("NOTE")||e==="STYLE"||e==="REGION"}function Gi(e){return String(e).padStart(2,"0")}var qe=Uint8Array,Er=Uint16Array,_h=Int32Array,ud=new qe([0,0,0,0,0,0,0,0,1,1,1,1,2,2,2,2,3,3,3,3,4,4,4,4,5,5,5,5,0,0,0,0]),dd=new qe([0,0,0,0,1,1,2,2,3,3,4,4,5,5,6,6,7,7,8,8,9,9,10,10,11,11,12,12,13,13,0,0]),Oh=new qe([16,17,18,0,8,7,9,6,10,5,11,4,12,3,13,2,14,1,15]),md=function(e,t){for(var r=new Er(31),n=0;n<31;++n)r[n]=t+=1<<e[n-1];for(var a=new _h(r[30]),n=1;n<30;++n)for(var i=r[n];i<r[n+1];++i)a[i]=i-r[n]<<5|n;return{b:r,r:a}},pd=md(ud,2),fd=pd.b,Dh=pd.r;fd[28]=258,Dh[258]=28;var Rh=md(dd,0),Lh=Rh.b,$o=new Er(32768);for(var ue=0;ue<32768;++ue){var At=(ue&43690)>>1|(ue&21845)<<1;At=(At&52428)>>2|(At&13107)<<2,At=(At&61680)>>4|(At&3855)<<4,$o[ue]=((At&65280)>>8|(At&255)<<8)>>1}var _n=(function(e,t,r){for(var n=e.length,a=0,i=new Er(t);a<n;++a)e[a]&&++i[e[a]-1];var o=new Er(t);for(a=1;a<t;++a)o[a]=o[a-1]+i[a-1]<<1;var s;if(r){s=new Er(1<<t);var l=15-t;for(a=0;a<n;++a)if(e[a])for(var c=a<<4|e[a],u=t-e[a],d=o[e[a]-1]++<<u,m=d|(1<<u)-1;d<=m;++d)s[$o[d]>>l]=c}else for(s=new Er(n),a=0;a<n;++a)e[a]&&(s[a]=$o[o[e[a]-1]++]>>15-e[a]);return s}),ea=new qe(288);for(var ue=0;ue<144;++ue)ea[ue]=8;for(var ue=144;ue<256;++ue)ea[ue]=9;for(var ue=256;ue<280;++ue)ea[ue]=7;for(var ue=280;ue<288;++ue)ea[ue]=8;var hd=new qe(32);for(var ue=0;ue<32;++ue)hd[ue]=5;var Bh=_n(ea,9,1),zh=_n(hd,5,1),Ki=function(e){for(var t=e[0],r=1;r<e.length;++r)e[r]>t&&(t=e[r]);return t},Xe=function(e,t,r){var n=t/8|0;return(e[n]|e[n+1]<<8)>>(t&7)&r},Vi=function(e,t){var r=t/8|0;return(e[r]|e[r+1]<<8|e[r+2]<<16)>>(t&7)},Uh=function(e){return(e+7)/8|0},Ts=function(e,t,r){return(t==null||t<0)&&(t=0),(r==null||r>e.length)&&(r=e.length),new qe(e.subarray(t,r))},Hh=["unexpected EOF","invalid block type","invalid length/literal","invalid distance","stream finished","no stream handler",,"no callback","invalid UTF-8 data","extra field too long","date not in range 1980-2099","filename too long","stream finishing","invalid zip data"],Ke=function(e,t,r){var n=new Error(t||Hh[e]);if(n.code=e,Error.captureStackTrace&&Error.captureStackTrace(n,Ke),!r)throw n;return n},Nh=function(e,t,r,n){var a=e.length,i=n?n.length:0;if(!a||t.f&&!t.l)return r||new qe(0);var o=!r,s=o||t.i!=2,l=t.i;o&&(r=new qe(a*3));var c=function(wl){var kl=r.length;if(wl>kl){var Pl=new qe(Math.max(kl*2,wl));Pl.set(r),r=Pl}},u=t.f||0,d=t.p||0,m=t.b||0,p=t.l,f=t.d,h=t.m,g=t.n,v=a*8;do{if(!p){u=Xe(e,d,1);var S=Xe(e,d+1,3);if(d+=3,S)if(S==1)p=Bh,f=zh,h=9,g=5;else if(S==2){var C=Xe(e,d,31)+257,k=Xe(e,d+10,15)+4,_=C+Xe(e,d+5,31)+1;d+=14;for(var P=new qe(_),R=new qe(19),E=0;E<k;++E)R[Oh[E]]=Xe(e,d+E*3,7);d+=k*3;for(var D=Ki(R),L=(1<<D)-1,U=_n(R,D,1),E=0;E<_;){var V=U[Xe(e,d,L)];d+=V&15;var b=V>>4;if(b<16)P[E++]=b;else{var ne=0,ee=0;for(b==16?(ee=3+Xe(e,d,3),d+=2,ne=P[E-1]):b==17?(ee=3+Xe(e,d,7),d+=3):b==18&&(ee=11+Xe(e,d,127),d+=7);ee--;)P[E++]=ne}}var ze=P.subarray(0,C),xe=P.subarray(C);h=Ki(ze),g=Ki(xe),p=_n(ze,h,1),f=_n(xe,g,1)}else Ke(1);else{var b=Uh(d)+4,x=e[b-4]|e[b-3]<<8,T=b+x;if(T>a){l&&Ke(0);break}s&&c(m+x),r.set(e.subarray(b,T),m),t.b=m+=x,t.p=d=T*8,t.f=u;continue}if(d>v){l&&Ke(0);break}}s&&c(m+131072);for(var O=(1<<h)-1,N=(1<<g)-1,ie=d;;ie=d){var ne=p[Vi(e,d)&O],oe=ne>>4;if(d+=ne&15,d>v){l&&Ke(0);break}if(ne||Ke(2),oe<256)r[m++]=oe;else if(oe==256){ie=d,p=null;break}else{var ke=oe-254;if(oe>264){var E=oe-257,De=ud[E];ke=Xe(e,d,(1<<De)-1)+fd[E],d+=De}var jt=f[Vi(e,d)&N],Ye=jt>>4;jt||Ke(3),d+=jt&15;var xe=Lh[Ye];if(Ye>3){var De=dd[Ye];xe+=Vi(e,d)&(1<<De)-1,d+=De}if(d>v){l&&Ke(0);break}s&&c(m+131072);var an=m+ke;if(m<xe){var xl=i-xe,uh=Math.min(xe,an);for(xl+m<0&&Ke(3);m<uh;++m)r[m]=n[xl+m]}for(;m<an;++m)r[m]=r[m-xe]}}t.l=p,t.p=ie,t.b=m,t.f=u,p&&(u=1,t.m=h,t.d=f,t.n=g)}while(!u);return m!=r.length&&o?Ts(r,0,m):r.subarray(0,m)},qh=new qe(0),ut=function(e,t){return e[t]|e[t+1]<<8},tt=function(e,t){return(e[t]|e[t+1]<<8|e[t+2]<<16|e[t+3]<<24)>>>0},Wi=function(e,t){return tt(e,t)+tt(e,t+4)*4294967296};function Gh(e,t){return Nh(e,{i:2},t&&t.out,t&&t.dictionary)}var Yo=typeof TextDecoder<"u"&&new TextDecoder,Kh=0;try{Yo.decode(qh,{stream:!0}),Kh=1}catch{}var Vh=function(e){for(var t="",r=0;;){var n=e[r++],a=(n>127)+(n>223)+(n>239);if(r+a>e.length)return{s:t,r:Ts(e,r-1)};a?a==3?(n=((n&15)<<18|(e[r++]&63)<<12|(e[r++]&63)<<6|e[r++]&63)-65536,t+=String.fromCharCode(55296|n>>10,56320|n&1023)):a&1?t+=String.fromCharCode((n&31)<<6|e[r++]&63):t+=String.fromCharCode((n&15)<<12|(e[r++]&63)<<6|e[r++]&63):t+=String.fromCharCode(n)}};function Wh(e,t){if(t){for(var r="",n=0;n<e.length;n+=16384)r+=String.fromCharCode.apply(null,e.subarray(n,n+16384));return r}else{if(Yo)return Yo.decode(e);var a=Vh(e),i=a.s,r=a.r;return r.length&&Ke(8),i}}var jh=function(e,t){return t+30+ut(e,t+26)+ut(e,t+28)},$h=function(e,t,r){var n=ut(e,t+28),a=Wh(e.subarray(t+46,t+46+n),!(ut(e,t+8)&2048)),i=t+46+n,o=tt(e,t+20),s=r&&o==4294967295?Yh(e,i):[o,tt(e,t+24),tt(e,t+42)],l=s[0],c=s[1],u=s[2];return[ut(e,t+10),l,c,a,i+ut(e,t+30)+ut(e,t+32),u]},Yh=function(e,t){for(;ut(e,t)!=1;t+=4+ut(e,t+2));return[Wi(e,t+12),Wi(e,t+4),Wi(e,t+20)]};function Es(e,t){for(var r={},n=e.length-22;tt(e,n)!=101010256;--n)(!n||e.length-n>65558)&&Ke(13);var a=ut(e,n+8);if(!a)return{};var i=tt(e,n+16),o=i==4294967295||a==65535;if(o){var s=tt(e,n-12);o=tt(e,s)==101075792,o&&(a=tt(e,s+32),i=tt(e,s+48))}for(var l=0;l<a;++l){var c=$h(e,i,o),u=c[0],d=c[1],m=c[2],p=c[3],f=c[4],h=c[5],g=jh(e,h);i=f,u?u==8?r[p]=Gh(e.subarray(g,g+d),{out:new qe(m)}):Ke(14,"unknown compression type "+u):r[p]=Ts(e,g,g+d)}return r}function gd(e){const t=typeof e=="string"?(()=>{try{return JSON.parse(e)}catch{return null}})():e;if(!t||typeof t!="object")return null;const r=t.w,n=t.h,a=t.fr,i=t.ip,o=t.op;if(typeof r!="number"||typeof n!="number"||typeof a!="number"||typeof o!="number"||!Array.isArray(t.layers))return null;const l=Math.max(1,Math.round(o-(typeof i=="number"?i:0))),c=a>0?a:30;return{width:Math.round(r),height:Math.round(n),frameRate:c,totalFrames:l,durationSeconds:l/c}}function Fs(e){return e.length>=4&&e[0]===80&&e[1]===75&&e[2]===3&&e[3]===4}function Xh(e,t){const r=`${t}.json`.toLowerCase();return e.find(n=>n.split("/").pop()?.toLowerCase()===r)}function yd(e,t){const r=Object.keys(e).filter(o=>/(^|\/)(?:animations|a)\/[^/]+\.json$/i.test(o));if(r.length===0)return[];const n=o=>{if(!o)return null;const s=Xh(r,o);return s?[s,...r.filter(l=>l!==s)]:null},a=n(t);if(a)return a;const i=Object.keys(e).find(o=>/(^|\/)manifest\.json$/i.test(o));if(!i)return r;try{const o=JSON.parse(new TextDecoder().decode(e[i]));return n(o.animations?.[0]?.id)??r}catch{return r}}function Qh(e,t){let r;try{r=Es(e)}catch{return null}const n=new TextDecoder;for(const a of yd(r,t))try{const i=gd(JSON.parse(n.decode(r[a])));if(i)return i}catch{}return null}const Jh={png:"image/png",jpg:"image/jpeg",jpeg:"image/jpeg",gif:"image/gif",webp:"image/webp",svg:"image/svg+xml",bmp:"image/bmp"};function Zh(e){const t=e.split(".").pop()?.toLowerCase()??"";return Jh[t]??"application/octet-stream"}function e0(e){let t="";for(let n=0;n<e.length;n+=32768)t+=String.fromCharCode(...e.subarray(n,n+32768));return btoa(t)}function t0(e,t){const r=e.assets;if(!Array.isArray(r))return;const n=Object.keys(t);for(const a of r){const i=a,o=i?.p;if(typeof o!="string"||o.startsWith("data:"))continue;const l=[`${typeof i.u=="string"?i.u:""}${o}`,`images/${o}`,o].map(u=>u.toLowerCase()),c=n.find(u=>{const d=u.toLowerCase();return l.includes(d)||l.some(m=>d.endsWith(`/${m}`))});if(c)try{i.p=`data:${Zh(o)};base64,${e0(t[c])}`,i.u="",i.e=1}catch{}}}function Rl(e){return e&&typeof e=="object"&&Array.isArray(e.layers)?e:null}function r0(e,t=!1,r){if(Fs(e)){let n;try{n=Es(e)}catch{return null}const a=new TextDecoder;for(const i of yd(n,r))try{const o=Rl(JSON.parse(a.decode(n[i])));if(!o)continue;return t&&t0(o,n),o}catch{}return null}try{return Rl(JSON.parse(new TextDecoder().decode(e)))}catch{return null}}async function fT(e,t=!1,r){try{const n=await(await fetch(e)).arrayBuffer();return r0(new Uint8Array(n),t,r)}catch{return null}}function n0(e){if(!Fs(e))return null;try{return Es(e)}catch{return null}}function a0(e,t){const r=n0(e);if(!r)return null;const n=`themes/${t}.json`.toLowerCase(),a=Object.keys(r).find(i=>{const o=i.toLowerCase();return o===n||o.endsWith(`/${n}`)});if(!a)return null;try{return new TextDecoder().decode(r[a])}catch{return null}}async function hT(e,t){try{const r=await(await fetch(e)).arrayBuffer();return a0(new Uint8Array(r),t)}catch{return null}}function i0(e,t){if(Fs(e))return Qh(e,t);try{return gd(new TextDecoder().decode(e))}catch{return null}}const Ll=Oe("BlobUrlManager");class o0{entries=new Map;version=0;listeners=new Set;notify(){this.version++;for(const t of this.listeners)t()}subscribe=t=>(this.listeners.add(t),()=>this.listeners.delete(t));getSnapshot=()=>this.version;acquire(t,r,n){const a=this.entries.get(t);if(a)return a.refCount++,n&&a.blob&&(a.metadata=n,jo(a.url,a.blob,n)),a.url;const i=URL.createObjectURL(r);return jo(i,r,n),this.entries.set(t,{url:i,refCount:1,blob:r,metadata:n}),this.notify(),i}registerUrl(t,r){const n=this.entries.get(t);return n?(n.refCount++,n.url):(this.entries.set(t,{url:r,refCount:1,external:!0}),this.notify(),r)}get(t){return this.entries.get(t)?.url??null}has(t){return this.entries.has(t)}getMediaIdByUrl(t){for(const[r,n]of this.entries)if(n.url===t)return r;return null}invalidate(t){const r=this.entries.get(t);r&&(this.revokeEntry(r),this.entries.delete(t),this.notify())}revokeEntry(t){t.external||(od(t.url),URL.revokeObjectURL(t.url))}release(t){const r=this.entries.get(t);r&&(r.refCount--,r.refCount<=0&&(this.revokeEntry(r),this.entries.delete(t),this.notify(),Ll.debug(`Revoked blob URL for media ${t}`)))}invalidateAll(){for(const t of this.entries.values())this.revokeEntry(t);this.entries.clear(),this.notify()}releaseAll(){for(const[t,r]of this.entries)this.revokeEntry(r),Ll.debug(`Revoked blob URL for media ${t}`);this.entries.clear(),this.notify()}get size(){return this.entries.size}}const Un=new o0,Bl={BASE_URL:"/",DEV:!1,MODE:"production",PROD:!0,SSR:!1},zl=e=>!!e.dispatchFromDevtools&&typeof e.dispatch=="function",Hn=new Map,ha=e=>{const t=Hn.get(e);return t?Object.fromEntries(Object.entries(t.stores).map(([r,n])=>[r,n.getState()])):{}},s0=(e,t,r)=>{if(e===void 0)return{type:"untracked",connection:t.connect(r)};const n=Hn.get(r.name);if(n)return{type:"tracked",store:e,...n};const a={connection:t.connect(r),stores:{}};return Hn.set(r.name,a),{type:"tracked",store:e,...a}},l0=(e,t)=>{if(t===void 0)return;const r=Hn.get(e);r&&(delete r.stores[t],Object.keys(r.stores).length===0&&Hn.delete(e))},c0=e=>{var t,r;if(!e)return;const n=e.split(`
`),a=n.findIndex(o=>o.includes("api.setState"));if(a<0)return;const i=((t=n[a+1])==null?void 0:t.trim())||"";return(r=/.+ (.+) .+/.exec(i))==null?void 0:r[1]},u0=(e,t={})=>(r,n,a)=>{const{enabled:i,anonymousActionType:o,store:s,...l}=t;let c;try{c=(i??(Bl?"production":void 0)!=="production")&&window.__REDUX_DEVTOOLS_EXTENSION__}catch{}if(!c)return e(r,n,a);const{connection:u,...d}=s0(s,c,l);let m=!0;a.setState=((h,g,v)=>{const S=r(h,g);if(!m)return S;const b=v===void 0?{type:o||c0(new Error().stack)||"anonymous"}:typeof v=="string"?{type:v}:v;return s===void 0?(u?.send(b,n()),S):(u?.send({...b,type:`${s}/${b.type}`},{...ha(l.name),[s]:a.getState()}),S)}),a.devtools={cleanup:()=>{u&&typeof u.unsubscribe=="function"&&u.unsubscribe(),l0(l.name,s)}};const p=(...h)=>{const g=m;m=!1,r(...h),m=g},f=e(a.setState,n,a);if(d.type==="untracked"?u?.init(f):(d.stores[d.store]=a,u?.init(Object.fromEntries(Object.entries(d.stores).map(([h,g])=>[h,h===d.store?f:g.getState()])))),zl(a)){let h=!1;const g=a.dispatch;a.dispatch=(...v)=>{(Bl?"production":void 0)!=="production"&&v[0].type==="__setState"&&!h&&(console.warn('[zustand devtools middleware] "__setState" action type is reserved to set state from the devtools. Avoid using it.'),h=!0),g(...v)}}return u.subscribe(h=>{var g;switch(h.type){case"ACTION":if(typeof h.payload!="string"){console.error("[zustand devtools middleware] Unsupported action format");return}return ji(h.payload,v=>{if(v.type==="__setState"){if(s===void 0){p(v.state);return}Object.keys(v.state).length!==1&&console.error(`
                    [zustand devtools middleware] Unsupported __setState action format.
                    When using 'store' option in devtools(), the 'state' should have only one key, which is a value of 'store' that was passed in devtools(),
                    and value of this only key should be a state object. Example: { "type": "__setState", "state": { "abc123Store": { "foo": "bar" } } }
                    `);const S=v.state[s];if(S==null)return;JSON.stringify(a.getState())!==JSON.stringify(S)&&p(S);return}zl(a)&&a.dispatch(v)});case"DISPATCH":switch(h.payload.type){case"RESET":return p(f),s===void 0?u?.init(a.getState()):u?.init(ha(l.name));case"COMMIT":if(s===void 0){u?.init(a.getState());return}return u?.init(ha(l.name));case"ROLLBACK":return ji(h.state,v=>{if(s===void 0){p(v),u?.init(a.getState());return}p(v[s]),u?.init(ha(l.name))});case"JUMP_TO_STATE":case"JUMP_TO_ACTION":return ji(h.state,v=>{if(s===void 0){p(v);return}JSON.stringify(a.getState())!==JSON.stringify(v[s])&&p(v[s])});case"IMPORT_STATE":{const{nextLiftedState:v}=h.payload,S=(g=v.computedStates.slice(-1)[0])==null?void 0:g.state;if(!S)return;p(s===void 0?S:S[s]),u?.send(null,v);return}case"PAUSE_RECORDING":return m=!m}return}}),f},vd=u0,ji=(e,t)=>{let r;try{r=JSON.parse(e)}catch(n){console.error("[zustand devtools middleware] Could not parse the received json",n)}r!==void 0&&t(r)};function bd(e,t){let r;try{r=e()}catch{return}return{getItem:a=>{var i;const o=l=>l===null?null:JSON.parse(l,void 0),s=(i=r.getItem(a))!=null?i:null;return s instanceof Promise?s.then(o):o(s)},setItem:(a,i)=>r.setItem(a,JSON.stringify(i,void 0)),removeItem:a=>r.removeItem(a)}}const Xo=e=>t=>{try{const r=e(t);return r instanceof Promise?r:{then(n){return Xo(n)(r)},catch(n){return this}}}catch(r){return{then(n){return this},catch(n){return Xo(n)(r)}}}},d0=(e,t)=>(r,n,a)=>{let i={storage:bd(()=>window.localStorage),partialize:g=>g,version:0,merge:(g,v)=>({...v,...g}),...t},o=!1,s=0;const l=new Set,c=new Set;let u=i.storage;if(!u)return e((...g)=>{console.warn(`[zustand persist middleware] Unable to update item '${i.name}', the given storage is currently unavailable.`),r(...g)},n,a);const d=()=>{const g=i.partialize({...n()});return u.setItem(i.name,{state:g,version:i.version})},m=a.setState;a.setState=(g,v)=>(m(g,v),d());const p=e((...g)=>(r(...g),d()),n,a);a.getInitialState=()=>p;let f;const h=()=>{var g,v;if(!u)return;const S=++s;o=!1,l.forEach(x=>{var T;return x((T=n())!=null?T:p)});const b=((v=i.onRehydrateStorage)==null?void 0:v.call(i,(g=n())!=null?g:p))||void 0;return Xo(u.getItem.bind(u))(i.name).then(x=>{if(x)if(typeof x.version=="number"&&x.version!==i.version){if(i.migrate){const T=i.migrate(x.state,x.version);return T instanceof Promise?T.then(C=>[!0,C]):[!0,T]}console.error("State loaded from storage couldn't be migrated since no migrate function was provided")}else return[!1,x.state];return[!1,void 0]}).then(x=>{var T;if(S!==s)return;const[C,k]=x;if(f=i.merge(k,(T=n())!=null?T:p),r(f,!0),C)return d()}).then(()=>{S===s&&(b?.(n(),void 0),f=n(),o=!0,c.forEach(x=>x(f)))}).catch(x=>{S===s&&b?.(void 0,x)})};return a.persist={setOptions:g=>{i={...i,...g},g.storage&&(u=g.storage)},clearStorage:()=>{u?.removeItem(i.name)},getOptions:()=>i,rehydrate:()=>h(),hasHydrated:()=>o,onHydrate:g=>(l.add(g),()=>{l.delete(g)}),onFinishHydration:g=>(c.add(g),()=>{c.delete(g)})},i.skipHydration||h(),f||p},Sd=d0;function dt(){return import("./media-library-service-H0-RziZM.js")}const Id=["video/mp4","video/webm","video/quicktime","video/x-matroska","video/matroska","video/x-msvideo"],xd=["audio/mp3","audio/mpeg","audio/wav","audio/aac","audio/x-m4a","audio/mp4","audio/ogg","audio/webm"],wd=["image/jpeg","image/jpg","image/png","image/gif","image/webp","image/svg+xml"],As=["application/lottie+json"],m0=new Set(["","application/octet-stream","binary/octet-stream"]),p0=new Set([".mkv",".m4a",".json"]),f0={".mp4":"video/mp4",".webm":"video/webm",".mov":"video/quicktime",".mkv":"video/x-matroska",".avi":"video/x-msvideo",".mp3":"audio/mpeg",".wav":"audio/wav",".aac":"audio/aac",".m4a":"audio/x-m4a",".ogg":"audio/ogg",".opus":"audio/ogg",".jpg":"image/jpeg",".jpeg":"image/jpeg",".png":"image/png",".gif":"image/gif",".webp":"image/webp",".svg":"image/svg+xml",".json":"application/lottie+json",".lottie":"application/lottie+json"};function _s(e){const t=e.name.toLowerCase().match(/\.[^.]+$/)?.[0],r=t?f0[t]:void 0;return t&&r&&p0.has(t)?r:m0.has(e.type)?r||"":e.type}function h0(e){if(e.size===0)return{valid:!1,error:"File is empty"};const t=[...Id,...xd,...wd,...As],r=_s(e);return t.includes(r)?e.name.length>255?{valid:!1,error:"Filename too long (max 255 characters)"}:{valid:!0}:{valid:!1,error:`Unsupported file type: ${r||e.name.split(".").pop()}. Supported types: video (mp4, webm, mov, mkv, avi), audio (mp3, wav, aac, m4a, ogg/opus), image (jpg/jpeg, png, gif, webp, svg), lottie (json, lottie)`}}async function gT(e){const t=h0(e);if(!t.valid)return t;if(y0(_s(e))){const r=new Uint8Array(await e.arrayBuffer());if(!i0(r))return{valid:!1,error:`Not a valid Lottie animation: ${e.name}`}}return t}function g0(e){const t=e.split(";")[0]?.trim()??e;return Id.includes(t)?"video":xd.includes(t)?"audio":wd.includes(t)?"image":As.includes(t)?"lottie":"unknown"}function y0(e){return As.includes(e)}function yT(e,t){e.forEach(r=>{r.reject(t)}),e.clear()}function v0(e){let t=null,r=null;function n(){const s=e.createWorker();return r=e.setupWorker?.(s)??null,t=s,s}function a(){return t??n()}function i(){return t}function o(){if(!t)return;const s=t;t=null;try{r?.()}finally{r=null,s.terminate()}}return{getWorker:a,peekWorker:i,terminate:o}}const b0={kind:"editor-opfs-cache"};async function kd(e){const t=nt(),[r,n]=await Promise.all([t.workspaceId(),t.getMedia(e)]);return{workspaceId:r,mediaId:e,contentHash:n.contentHash}}async function Pd(e,t){const r=t?.workspaceId??await nt().workspaceId();if(e[0]==="media"&&e[1]){const n=t??await kd(e[1]);if(n.mediaId!==e[1])throw new Error("Media không khớp danh tính cache");return["ytf-editor-cache",encodeURIComponent(r),"media",encodeURIComponent(n.mediaId),encodeURIComponent(n.contentHash),...e.slice(2)]}return["ytf-editor-cache",encodeURIComponent(r),...e]}async function ta(e,t=!1,r){let n=await navigator.storage.getDirectory();for(const a of await Pd(e,r)){if(!a||a==="."||a===".."||a.includes("/"))throw new Error("Đường dẫn cache không hợp lệ");n=await n.getDirectoryHandle(a,{create:t})}return n}async function Qo(e,t){try{return await(await(await ta(e.slice(0,-1),!1,t)).getFileHandle(e[e.length-1])).getFile()}catch{return null}}async function S0(e,t,r){try{const i=await(await(await ta(e.slice(0,-1),!0,r)).getFileHandle(e[e.length-1],{create:!0})).createWritable();try{await i.write(t),await i.close()}catch(o){throw await i.abort().catch(()=>{}),o}}catch{}}async function Jo(e,t=!1,r){try{await(await ta(e.slice(0,-1),!1,r)).removeEntry(e[e.length-1],{recursive:t})}catch{}}const Md="proxy.mp4",I0="meta.json",x0=4;function w0(e){return[...e,Md].join("/")}const Cd="@";function Td(e){if(!e.contentHash)throw new Error(`Media ${e.id} has no content hash; cannot derive its proxy key`);return`${e.id}${Cd}${e.contentHash}`}function Zo(e){const t=e.indexOf(Cd);return t<=0||t===e.length-1?null:{mediaId:e.slice(0,t),contentHash:e.slice(t+1)}}async function yn(e){const t=Zo(e);if(!t)throw new Error(`Proxy key "${e}" does not identify a media version`);return{workspaceId:await nt().workspaceId(),...t}}async function Ul(e){const t=await yn(e);return Pd(["media",t.mediaId,"proxy"],t)}function ra(e){return Ie()(t=>({...e.initialState(),setPreview:r=>t(e.normalizePreview?.(r)??r),clearPreview:()=>t(e.initialState()),...e.createActions?.(t)??{}}))}const k0=()=>({trimmedItemId:null,handle:null,trackId:null,downstreamItemIds:new Set,delta:0,trimDelta:0}),Ed=ra({initialState:k0,createActions:e=>({setDeltas:(t,r)=>e({delta:t,trimDelta:r})})});function Fd(e,t){const r={...e};for(const[n,a]of Object.entries(t))r[n]=r[n]??a;return r}const P0=()=>({trimmedItemId:null,neighborItemId:null,handle:null,neighborDelta:0,constrained:!1}),Ad=ra({initialState:P0,normalizePreview:e=>Fd(e,{constrained:!1}),createActions:e=>({setNeighborDelta:(t,r)=>e({neighborDelta:t,constrained:r??!1})})}),M0=()=>({itemId:null,trackId:null,leftNeighborId:null,rightNeighborId:null,slideDelta:0,minDelta:0,maxDelta:0}),_d=ra({initialState:M0,normalizePreview:e=>Fd(e,{minDelta:0,maxDelta:0}),createActions:e=>({setSlideDelta:t=>e({slideDelta:t}),setSlideRange:(t,r)=>e({minDelta:t,maxDelta:r})})}),C0=()=>({itemId:null,trackId:null,slipDelta:0}),Od=ra({initialState:C0,createActions:e=>({setSlipDelta:t=>e({slipDelta:t})})}),T0=()=>({anchorItemId:null,trackId:null,shiftedItemIds:new Set,delta:0}),Dd=ra({initialState:T0,createActions:e=>({setDelta:t=>e({delta:t})})}),E0=50;function F0(e,t,r){return Math.max(t,Math.min(r,e))}function A0(e,t){const r=Math.max(10,t),n=Math.max(0,e-E0);return F0(n/(r*100),fh,ph)}const _0=[1,2,4];function O0(e,t){if(!Number.isFinite(e)||Math.sign(e)!==t)return t;const r=Math.abs(e),n=_0.find(a=>a>r)??4;return t*n}const $i=new Map,D0={getItem:e=>$i.get(e)??null,setItem:(e,t)=>{$i.set(e,t)},removeItem:e=>{$i.delete(e)}};function Wa(e){return Number.isFinite(e)?Math.max(0,Math.round(e)):0}function R0(e){return e===.5||e===.33||e===.25?e:1}function Rd(e){if(e.isPlaying&&e.previewFrame===null&&e.previewItemId===null)return e;if(e.previewFrame===null&&e.previewItemId===null)return{isPlaying:!0};const t=e.frameUpdateEpoch+1;return{isPlaying:!0,previewFrame:null,previewItemId:null,previewFrameEpoch:t,frameUpdateEpoch:t}}function Hl(e){const t=Rd(e);return t===e?e.playbackRate===1&&e.transportMode==="normal"?e.playbackScrubResumeTransport===null?e:{playbackScrubResumeTransport:null}:{playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}:{...t,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}}function Nl(e,t){const r=Rd(e);return{...r===e?{}:r,playbackRate:e.isPlaying?O0(e.playbackRate,t):t,transportMode:"shuttle",playbackScrubResumeTransport:null}}function L0(e,t,r){const n=Wa(t),a=r??null;if(e.currentFrame===n&&e.previewFrame===null&&a===null||e.currentFrame===n&&e.previewFrame===n&&e.previewItemId===a)return e;const i=e.frameUpdateEpoch+1;return{currentFrame:n,currentFrameEpoch:i,previewFrame:n,previewItemId:a,previewFrameEpoch:i,frameUpdateEpoch:i}}const me=Ie()(Sd(e=>({currentFrame:0,currentFrameEpoch:0,isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null,loop:!1,volume:1,muted:!1,masterBusDb:0,busAudioEq:void 0,zoom:-1,previewFrame:null,previewFrameEpoch:0,frameUpdateEpoch:0,previewItemId:null,useProxy:!0,previewQuality:1,compositionVisualFrozen:!1,setCurrentFrame:t=>e(r=>{const n=Wa(t);if(r.currentFrame===n)return r;const a=r.frameUpdateEpoch+1;return{currentFrame:n,currentFrameEpoch:a,frameUpdateEpoch:a}}),setScrubFrame:(t,r)=>e(n=>n.isPlaying?n:L0(n,t,r)),finishScrub:t=>e(r=>{const n=Wa(t);if(r.currentFrame===n&&r.previewFrame===null&&r.previewItemId===null&&!r.compositionVisualFrozen)return r;const a=r.frameUpdateEpoch+1;return{currentFrame:n,currentFrameEpoch:a,previewFrame:null,previewItemId:null,previewFrameEpoch:a,frameUpdateEpoch:a,compositionVisualFrozen:!1}}),beginPlaybackScrub:()=>e(t=>({isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:t.isPlaying?{playbackRate:t.playbackRate,transportMode:t.transportMode}:null})),resumePlaybackAfterScrub:()=>e(t=>{const r=t.playbackScrubResumeTransport;return r?t.isPlaying?{playbackScrubResumeTransport:null}:{isPlaying:!0,playbackRate:r.playbackRate,transportMode:r.transportMode,playbackScrubResumeTransport:null}:t}),cancelPlaybackScrubResume:()=>e(t=>t.playbackScrubResumeTransport===null?t:{playbackScrubResumeTransport:null}),play:()=>e(Hl),pause:()=>e(t=>t.isPlaying||t.playbackRate!==1||t.transportMode!=="normal"||t.playbackScrubResumeTransport!==null?{isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}:t),togglePlayPause:()=>e(t=>t.isPlaying?{isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}:Hl(t)),shuttleForward:()=>e(t=>Nl(t,1)),shuttleReverse:()=>e(t=>Nl(t,-1)),setPlaybackRate:t=>e({playbackRate:t,playbackScrubResumeTransport:null}),toggleLoop:()=>e(t=>({loop:!t.loop})),setVolume:t=>e({volume:t}),toggleMute:()=>e(t=>({muted:!t.muted})),setMuted:t=>e(r=>r.muted===t?r:{muted:t}),setMasterBusDb:t=>e({masterBusDb:Math.max(-60,Math.min(12,t))}),setBusAudioEq:t=>e({busAudioEq:t}),setZoom:t=>e({zoom:t}),setPreviewFrame:(t,r)=>e(n=>{if(n.isPlaying&&t!==null)return n;const a=t==null?null:Wa(t),i=t==null?null:r??null;if(n.previewFrame===a&&n.previewItemId===i)return n;const o=n.frameUpdateEpoch+1;return{previewFrame:a,previewItemId:i,previewFrameEpoch:o,frameUpdateEpoch:o}}),toggleUseProxy:()=>e(t=>({useProxy:!t.useProxy})),setPreviewQuality:t=>e(r=>{const n=R0(t);return r.previewQuality===n?r:{previewQuality:n}}),setCompositionVisualFrozen:t=>e(r=>r.compositionVisualFrozen===t?r:{compositionVisualFrozen:t})}),{name:"playback-storage",storage:bd(()=>typeof localStorage>"u"||typeof localStorage.setItem!="function"?D0:localStorage),partialize:e=>({zoom:e.zoom,volume:e.volume,muted:e.muted,loop:e.loop,useProxy:e.useProxy,previewQuality:e.previewQuality}),merge:(e,t)=>({...t,...e,isPlaying:!1,playbackRate:1,transportMode:"normal"})})),B0=Ie(()=>({contentLevel:1,contentPixelsPerSecond:100})),ql=120,z0=200,Gl=200;let rr=0,ct=null,Br=null,zr=null,On=null,Ur=null,xt=null,Ht=null,vn=0;function ai(e){return e*100}function Os(){zr&&(clearTimeout(zr),zr=null)}function ii(){On&&(clearTimeout(On),On=null),Ur!==null&&(typeof cancelAnimationFrame=="function"&&cancelAnimationFrame(Ur),Ur=null),xt!==null&&(typeof cancelIdleCallback=="function"&&cancelIdleCallback(xt),xt=null),Ht!==null&&(clearTimeout(Ht),Ht=null)}function ja(e,t,r){e({level:t,pixelsPerSecond:ai(t),isZoomInteracting:r})}function U0(e){if(ct===null)return;Os(),rr=performance.now();const t=ct;ct=null,ja(e,t,!0)}function $a(e){if(Ur=null,xt=null,Ht!==null&&(clearTimeout(Ht),Ht=null),Br===null){e({isZoomInteracting:!1});return}const t=Br;Br=null,U0(e),e({level:t,pixelsPerSecond:ai(t),contentLevel:t,contentPixelsPerSecond:ai(t),isZoomInteracting:!1})}function H0(e){if(!me.getState().isPlaying||typeof requestAnimationFrame!="function"||typeof requestIdleCallback!="function"){$a(e);return}Ur=requestAnimationFrame(()=>{Ur=null;const t=()=>{xt=null,$a(e)};xt=requestIdleCallback(t,{timeout:Gl}),Ht=setTimeout(()=>{Ht=null,xt!==null&&typeof cancelIdleCallback=="function"&&(cancelIdleCallback(xt),xt=null),$a(e)},Gl)})}function N0(e){ii(),!(vn>0)&&(On=setTimeout(()=>{On=null,H0(e)},z0))}function Kl(e,t,r=!1){Br=t,r||e({isZoomInteracting:!0}),N0(e)}function ga(e,t){ct=null,Br=null,Os(),ii(),rr=performance.now();const r=ai(t);e({level:t,pixelsPerSecond:r,contentLevel:t,contentPixelsPerSecond:r,isZoomInteracting:!1})}const mr=Ie((e,t)=>({level:1,pixelsPerSecond:100,contentLevel:1,contentPixelsPerSecond:100,isZoomInteracting:!1,beginZoomGesture:()=>{vn+=1,ii(),t().isZoomInteracting||e({isZoomInteracting:!0})},endZoomGesture:()=>{vn>0&&(vn-=1),!(vn>0)&&(Br!==null?(ii(),$a(e)):t().isZoomInteracting&&e({isZoomInteracting:!1}))},setZoomLevel:r=>{const n=performance.now();if(ct=r,Kl(e,r),n-rr>=ql){rr=n,ja(e,r,!0),ct=null;return}zr||(zr=setTimeout(()=>{if(zr=null,ct!==null){const a=ct;ct=null,rr=performance.now(),ja(e,a,!0)}},ql-(n-rr)))},setZoomLevelImmediate:r=>{Os(),ct=null,rr=performance.now(),ja(e,r,!0),Kl(e,r,!0)},setZoomLevelSynchronized:r=>{ga(e,r)},zoomIn:()=>{const r=Math.min(t().level*1.1,50);ga(e,r)},zoomOut:()=>{const r=Math.max(t().level/1.1,.01);ga(e,r)},zoomToFit:(r,n)=>{const a=A0(r,n);ga(e,a)}}));mr.subscribe((e,t)=>{e.contentLevel===t.contentLevel&&e.contentPixelsPerSecond===t.contentPixelsPerSecond||B0.setState({contentLevel:e.contentLevel,contentPixelsPerSecond:e.contentPixelsPerSecond})});function q0(e,t){if(e.length!==t.length)return!1;for(let r=0;r<e.length;r+=1)if(e[r]!==t[r])return!1;return!0}function G0(e){return e?{...e,draggedItemIdSet:e.draggedItemIdSet??new Set(e.draggedItemIds),draggedTrackIdSet:e.draggedTrackIdSet??new Set(e.draggedTrackIds??[])}:null}const rt=Ie(e=>({selectedItemIds:[],selectedItemIdSet:new Set,selectedMarkerId:null,selectedTransitionId:null,selectedTrackId:null,selectedTrackIds:[],activeTrackId:null,selectionType:null,activeTool:"select",activeSnapTarget:null,activeLinkedDropTarget:null,dragState:null,editKeyframePanelOpen:!1,expandedKeyframeLanes:new Set,selectItems:t=>e(r=>{const n=t.length>0?"item":r.selectedTrackIds.length>0?"track":null;return q0(r.selectedItemIds,t)&&r.selectedMarkerId===null&&r.selectedTransitionId===null&&r.selectionType===n?r:{selectedItemIds:t,selectedItemIdSet:new Set(t),expandedKeyframeLanes:r.editKeyframePanelOpen?t.length===1?new Set([t[0]]):new Set:r.expandedKeyframeLanes,selectedMarkerId:null,selectedTransitionId:null,selectionType:n}}),selectMarker:t=>e({selectedMarkerId:t,selectedTransitionId:null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectionType:t?"marker":null}),selectTransition:t=>e({selectedTransitionId:t,selectedMarkerId:null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectionType:t?"transition":null}),selectTrack:t=>e({selectedTrackId:t,activeTrackId:t,selectedTrackIds:t?[t]:[],selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:t?"track":null}),selectTracks:(t,r=!1)=>e(n=>{const a=r?Array.from(new Set([...n.selectedTrackIds,...t])):t;return{selectedTrackIds:a,activeTrackId:t[0]||null,selectedTrackId:t[0]||null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:a.length>0?"track":null}}),setActiveTrack:t=>e({activeTrackId:t,selectedTrackId:t,selectedTrackIds:t?[t]:[],selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:t?"track":null}),toggleTrackSelection:t=>e(r=>{const a=r.selectedTrackIds.includes(t)?r.selectedTrackIds.filter(i=>i!==t):[...r.selectedTrackIds,t];return{selectedTrackIds:a,activeTrackId:a[0]||null,selectedTrackId:a[0]||null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:a.length>0?"track":null}}),clearSelection:()=>e({selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectedTransitionId:null,selectedTrackId:null,selectedTrackIds:[],activeTrackId:null,selectionType:null}),clearItemSelection:()=>e(t=>({selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectionType:t.selectedTrackIds.length>0?"track":null})),setDragState:t=>e(r=>({dragState:G0(t),activeSnapTarget:t?r.activeSnapTarget:null,activeLinkedDropTarget:t?r.activeLinkedDropTarget:null})),setActiveSnapTarget:t=>e({activeSnapTarget:t}),setActiveLinkedDropTarget:t=>e({activeLinkedDropTarget:t}),setActiveTool:t=>e({activeTool:t}),toggleEditKeyframePanel:()=>e(t=>{const r=!t.editKeyframePanelOpen,n=t.selectedItemIds.length===1?t.selectedItemIds[0]:null;return{editKeyframePanelOpen:r,expandedKeyframeLanes:r&&n?new Set([n]):new Set}}),setEditKeyframePanelOpen:t=>e(r=>{if(r.editKeyframePanelOpen===t)return r;const n=r.selectedItemIds.length===1?r.selectedItemIds[0]:null;return{editKeyframePanelOpen:t,expandedKeyframeLanes:t&&n?new Set([n]):new Set}}),toggleKeyframeLanes:t=>e(r=>{const n=r.editKeyframePanelOpen&&r.expandedKeyframeLanes.has(t);return{selectedItemIds:[t],selectedItemIdSet:new Set([t]),selectedMarkerId:null,selectedTransitionId:null,selectionType:"item",editKeyframePanelOpen:!n,expandedKeyframeLanes:n?new Set:new Set([t])}}),setKeyframeLanesExpanded:(t,r)=>e(n=>r?{editKeyframePanelOpen:!0,expandedKeyframeLanes:new Set([t])}:n.expandedKeyframeLanes.has(t)?{editKeyframePanelOpen:!1,expandedKeyframeLanes:new Set}:n)})),K0=1200,es=new Map,Vl=new Set;function V0(){return!!rt.getState().dragState?.isDragging}function W0(){const e=Ad.getState();if(e.trimmedItemId!==null||e.neighborItemId!==null||e.handle!==null)return!0;const t=Ed.getState();return t.trimmedItemId!==null||t.handle!==null||Od.getState().itemId!==null||_d.getState().itemId!==null?!0:Dd.getState().anchorItemId!==null}function Ld(e,t){return t<e.releaseAtMs?!0:e.releaseRequested?!1:e.expiresAtMs===null||t<e.expiresAtMs}function j0(e=Date.now()){let t=!1;for(const[r,n]of es)Ld(n,e)||(es.delete(r),t=!0);return t}function $0(){j0();const e=Date.now();for(const t of es.values())if(Ld(t,e))return!0;return!1}function Yi(e={}){return mr.getState().isZoomInteracting||V0()||W0()||!e.ignoreAudioStartupHold&&$0()}function Y0(e){Vl.add(e);const t=[mr.subscribe(e),rt.subscribe(e),Ad.subscribe(e),Ed.subscribe(e),Od.subscribe(e),_d.subscribe(e),Dd.subscribe(e)];return()=>{Vl.delete(e);for(const r of t)r()}}function X0(e,t,r){let n=null,a=null,i=!1;const o=()=>{if(!i){if(typeof requestIdleCallback=="function"){a=requestIdleCallback(()=>{a=null,i||e()},{timeout:r});return}n=setTimeout(()=>{n=null,i||e()},0)}};return t<=0?o():n=setTimeout(()=>{n=null,o()},t),()=>{i=!0,n!==null&&clearTimeout(n),a!==null&&typeof cancelIdleCallback=="function"&&cancelIdleCallback(a)}}function Q0(e,t={}){const r=t.delayMs??0,n=t.idleTimeoutMs??K0,a={ignoreAudioStartupHold:t.ignoreAudioStartupHold??!1};let i=!1,o=()=>{},s=()=>{};const l=()=>{o(),s()},c=()=>{o(),o=X0(()=>{if(!i){if(Yi(a)){d();return}e()}},r,n)},u=()=>{i||Yi(a)||(s(),s=()=>{},c())},d=()=>{s(),s=Y0(u)};return Yi(a)?d():c(),()=>{i=!0,l()}}const J0=Oe("BackgroundMediaWork"),Bd=1;let Z0=1,eg=1,bn=0,Di=null,oi=()=>{};const Nt=[];function tg(){Nt.sort((e,t)=>e.priority!==t.priority?e.priority-t.priority:e.readyAtMs!==t.readyAtMs?e.readyAtMs-t.readyAtMs:e.sequence-t.sequence)}function zd(){oi(),oi=()=>{},Di=null}function rg(e){if(Di=null,oi=()=>{},bn>=Bd){Fr();return}const t=Nt.findIndex(n=>n.id===e);if(t===-1){Fr();return}const[r]=Nt.splice(t,1);if(!r){Fr();return}bn+=1,Promise.resolve().then(r.run).catch(n=>{J0.warn("Background media work failed:",n)}).finally(()=>{bn=Math.max(0,bn-1),Fr()})}function Fr(){if(zd(),bn>=Bd||Nt.length===0)return;tg();const e=Nt[0];if(!e)return;const t=Math.max(0,e.readyAtMs-Date.now());Di=e.id,oi=Q0(()=>{rg(e.id)},{delayMs:t})}function Wl(e,t={}){const r=Z0++,n=t.priority==="heavy"?1:0;return Nt.push({id:r,sequence:eg++,readyAtMs:Date.now()+Math.max(0,t.delayMs??0),priority:n,run:e}),Fr(),()=>{const a=Nt.findIndex(i=>i.id===r);a!==-1&&Nt.splice(a,1),Di===r&&(zd(),Fr())}}const Re=Oe("ProxyService");function jl(e){od(e),URL.revokeObjectURL(e)}let ya=null;function ng(e){if(!e||e==="avc")return!0;if(e==="hevc"){if(ya===null)try{ya=typeof document<"u"&&document.createElement("video").canPlayType('video/mp4; codecs="hvc1.1.6.L93.B0"')==="probably"}catch{ya=!1}return ya}return!0}const $l=150,ag=.01,ig=1,og=12,sg=0,lg=900;class cg{proxyBlobUrlByKey=new Map;proxyKeyByMediaId=new Map;mediaIdsByProxyKey=new Map;progressByProxyKey=new Map;pendingJobsByKey=new Map;pendingJobOrder=[];activeJobPhaseByKey=new Map;activeJobPriorityByKey=new Map;progressEmissionByProxyKey=new Map;statusListener=null;mediaResolver=null;filmstripPrewarm=null;generatingProxyKeys=new Set;isRefreshing=!1;maxConcurrentJobs=1;workerManager=v0({createWorker:()=>new Worker(new URL("/assets/proxy-generation-worker-C51YFpzb.js",import.meta.url),{type:"module"}),setupWorker:t=>(t.onmessage=r=>{this.handleWorkerMessage(r.data)},t.onerror=r=>{Re.error("Proxy worker error:",r)},()=>{t.onmessage=null,t.onerror=null})});onStatusChange(t){this.statusListener=t}setMediaResolver(t){this.mediaResolver=t}setFilmstripPrewarm(t){this.filmstripPrewarm=t}setProxyKey(t,r){const n=this.proxyKeyByMediaId.get(t);if(n===r)return;if(n){const i=this.mediaIdsByProxyKey.get(n);i?.delete(t),i&&i.size===0&&this.mediaIdsByProxyKey.delete(n)}this.proxyKeyByMediaId.set(t,r);let a=this.mediaIdsByProxyKey.get(r);a||(a=new Set,this.mediaIdsByProxyKey.set(r,a)),a.add(t),this.proxyBlobUrlByKey.has(r)?this.statusListener?.(t,"ready"):this.generatingProxyKeys.has(r)&&this.statusListener?.(t,"generating",this.progressByProxyKey.get(r)??0)}clearProxyKey(t){const r=this.proxyKeyByMediaId.get(t);if(!r)return;this.proxyKeyByMediaId.delete(t);const n=this.mediaIdsByProxyKey.get(r);n?.delete(t),n&&n.size===0&&this.mediaIdsByProxyKey.delete(r)}getProxyKey(t){return this.proxyKeyByMediaId.get(t)}canGenerateProxy(t){return t.startsWith("video/")}generateProxy(t,r,n,a,i,o){i&&this.setProxyKey(t,i);const s=this.resolveProxyKey(t,i);if(this.proxyBlobUrlByKey.has(s)){this.emitStatusForProxyKey(s,"ready");return}if(this.generatingProxyKeys.has(s)){const l=this.pendingJobsByKey.get(s);l&&(o?.priority??"user")==="user"&&l.priority!=="user"&&(this.removePendingJob(s),l.priority="user",this.insertPendingJob(l)),(o?.priority??"user")==="user"&&this.activeJobPriorityByKey.set(s,"user"),this.emitStatusForProxyKey(s,"generating",this.progressByProxyKey.get(s)??0);return}this.generatingProxyKeys.add(s),this.progressByProxyKey.set(s,0),this.emitStatusForProxyKey(s,"generating",0),this.insertPendingJob({mediaId:t,proxyKey:s,sourceWidth:n,sourceHeight:a,source:this.normalizeSource(r),priority:o?.priority??"user"}),this.drainQueue()}cancelProxy(t,r){const n=this.resolveProxyKey(t,r);if(!this.generatingProxyKeys.has(n))return;if(this.generatingProxyKeys.delete(n),this.progressByProxyKey.delete(n),this.clearProgressEmissionState(n),this.emitStatusForProxyKey(n,"idle"),this.pendingJobsByKey.has(n)){this.removePendingJob(n),this.activeJobPriorityByKey.delete(n);return}const a=this.activeJobPhaseByKey.get(n);if(!a)return;if(a==="loading"){this.activeJobPhaseByKey.delete(n),this.activeJobPriorityByKey.delete(n),this.drainQueue();return}this.getWorker().postMessage({type:"cancel",mediaId:n})}cancelBackgroundProxy(t,r){const n=this.resolveProxyKey(t,r),a=this.pendingJobsByKey.get(n)?.priority,i=this.activeJobPriorityByKey.get(n);a!=="background"&&i!=="background"||this.cancelProxy(t,n)}getProxyBlobUrl(t,r){const n=this.resolveProxyKey(t,r);return this.proxyBlobUrlByKey.get(n)??null}getMediaIdByProxyUrl(t){for(const[r,n]of this.proxyBlobUrlByKey)if(n===t)return this.mediaIdsByProxyKey.get(r)?.values().next().value??null;return null}hasProxy(t,r){const n=this.resolveProxyKey(t,r);return this.proxyBlobUrlByKey.has(n)}async deleteProxy(t,r){const n=this.resolveProxyKey(t,r);if(this.cancelProxy(t,n),this.releaseProxyUrl(n),!Zo(n))return;const a=await yn(n);await Jo(["media",a.mediaId,"proxy"],!0,a),Re.debug(`Deleted proxy for ${n}`)}clearMemory(t){const r=this.resolveProxyKey(t);this.cancelProxy(t,r),this.releaseProxyUrl(r),this.clearProxyKey(t)}async loadExistingProxies(t){const r=[],n=new Set;for(const o of t)n.add(this.resolveProxyKey(o));const a=o=>{const s=this.mediaIdsByProxyKey.get(o);return!s||s.size===0?[]:[...s]},i=async(o,s)=>{try{const l=await yn(o);await Jo(["media",l.mediaId,"proxy"],!0,l),Re.debug(`Removed ${s} proxy for ${o}`)}catch(l){Re.warn(`Failed to remove ${s} proxy for ${o}:`,l)}};for(const o of n)if(Zo(o))try{const s=await this.readProxyMetadata(o);if(!s)continue;if(s.status==="generating"){r.push(...a(o)),await i(o,"interrupted");continue}if(s.status==="error"){await i(o,"failed");continue}if(s.status!=="ready"){r.push(...a(o)),await i(o,"invalid-status");continue}if(s.version!==x0){r.push(...a(o)),await i(o,"stale");continue}if(!ng(s.codec)){Re.debug(`Skipping proxy ${o}: codec '${s.codec}' not playable here; using source`);continue}const l=await this.readProxyFile(o);if(!l||l.size===0){r.push(...a(o)),await i(o,"empty");continue}await this.setProxyUrl(o,l),this.emitStatusForProxyKey(o,"ready"),Re.debug(`Loaded existing proxy for ${o}`)}catch(s){Re.warn(`Failed to load existing proxy for ${o}:`,s),r.push(...a(o)),await i(o,"invalid")}return r}getWorker(){return this.workerManager.getWorker()}handleWorkerMessage(t){const r=t.mediaId;switch(t.type){case"progress":{if(!this.generatingProxyKeys.has(r))break;this.progressByProxyKey.set(r,t.progress),this.emitProgressThrottled(r,t.progress);break}case"complete":{const n=!this.generatingProxyKeys.has(r);if(this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.generatingProxyKeys.delete(r),this.progressByProxyKey.delete(r),this.clearProgressEmissionState(r),this.drainQueue(),n)break;this.loadCompletedProxy(r);break}case"cancelled":{this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.generatingProxyKeys.delete(r),this.progressByProxyKey.delete(r),this.clearProgressEmissionState(r),this.drainQueue();break}case"error":{const n=!this.generatingProxyKeys.has(r);if(this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.generatingProxyKeys.delete(r),this.progressByProxyKey.delete(r),this.clearProgressEmissionState(r),this.drainQueue(),n)break;Re.error(`Proxy generation failed for ${r}:`,t.error),this.emitStatusForProxyKey(r,"error");break}}}async loadCompletedProxy(t){try{const r=await this.readProxyFile(t);if(!r||r.size===0){this.emitStatusForProxyKey(t,"error");return}await this.setProxyUrl(t,r),this.emitStatusForProxyKey(t,"ready"),this.prewarmFilmstripFromProxy(t,r),Re.debug(`Proxy ready for ${t}`)}catch(r){Re.error(`Failed to load completed proxy for ${t}:`,r),this.emitStatusForProxyKey(t,"error")}}prewarmFilmstripFromProxy(t,r){const n=[...this.mediaIdsByProxyKey.get(t)??[]];if(n.length!==0)for(const a of n){const i=this.mediaResolver?.(a);if(!i||!i.mimeType.startsWith("video/")||i.duration<=0)continue;const o=Math.min(i.duration,ig);Wl(()=>this.filmstripPrewarm?.(a,r,i.duration,{startTime:0,endTime:o}),{priority:"warm",delayMs:sg});const s=Math.min(i.duration,og);Wl(()=>this.filmstripPrewarm?.(a,r,i.duration,{startTime:0,endTime:s}),{priority:"warm",delayMs:lg})}}async refreshAllBlobUrls(){if(this.isRefreshing)return 0;const t=[...this.proxyBlobUrlByKey.keys()];if(t.length===0)return 0;this.isRefreshing=!0;let r=0;try{for(const n of t)try{const a=await this.readProxyFile(n);if(!a||a.size===0){this.releaseProxyUrl(n);continue}await this.setProxyUrl(n,a),r++}catch{this.releaseProxyUrl(n)}}catch(n){Re.warn("Failed to refresh proxy blob URLs:",n)}finally{this.isRefreshing=!1}return r>0&&Re.debug(`Refreshed ${r} proxy blob URLs from OPFS`),r}async readProxyFile(t){const r=await yn(t);return Qo(["media",r.mediaId,"proxy",Md],r)}async readProxyMetadata(t){const r=await yn(t),n=await Qo(["media",r.mediaId,"proxy",I0],r);return n?JSON.parse(await n.text()):null}async setProxyUrl(t,r){const n=w0(await Ul(t)),a=this.proxyBlobUrlByKey.get(t);a&&jl(a);const i=URL.createObjectURL(r);jo(i,r,{opfsPath:n,fileSize:r.size}),this.proxyBlobUrlByKey.set(t,i)}releaseProxyUrl(t){const r=this.proxyBlobUrlByKey.get(t);r&&jl(r),this.proxyBlobUrlByKey.delete(t)}resolveProxyKey(t,r){return r||(this.proxyKeyByMediaId.get(t)??t)}normalizeSource(t){return typeof t=="function"?{load:t}:{load:async()=>t}}insertPendingJob(t){this.pendingJobsByKey.set(t.proxyKey,t);const r=this.pendingJobOrder.indexOf(t.proxyKey);if(r>=0&&this.pendingJobOrder.splice(r,1),t.priority==="background"){this.pendingJobOrder.push(t.proxyKey);return}const n=this.pendingJobOrder.findIndex(a=>this.pendingJobsByKey.get(a)?.priority==="background");if(n===-1){this.pendingJobOrder.push(t.proxyKey);return}this.pendingJobOrder.splice(n,0,t.proxyKey)}removePendingJob(t){this.pendingJobsByKey.delete(t);const r=this.pendingJobOrder.indexOf(t);r>=0&&this.pendingJobOrder.splice(r,1)}drainQueue(){for(;this.activeJobPhaseByKey.size<this.maxConcurrentJobs&&this.pendingJobOrder.length>0;){const t=this.pendingJobOrder.shift();if(!t)break;const r=this.pendingJobsByKey.get(t);r&&(this.pendingJobsByKey.delete(t),this.runQueuedJob(r))}}async runQueuedJob(t){const{proxyKey:r}=t;this.activeJobPriorityByKey.set(r,t.priority);try{const n=this.getWorker();this.activeJobPhaseByKey.set(r,"loading");let a=null;try{a=await t.source.load()}catch(o){if(!this.generatingProxyKeys.has(r)){this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.drainQueue();return}this.failQueuedJob(r,`Failed to load source for ${r}`,o);return}if(!this.generatingProxyKeys.has(r)){this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.drainQueue();return}if(!a){this.failQueuedJob(r,`Source media unavailable for ${r}`);return}const i=await Ul(r);if(!this.generatingProxyKeys.has(r)){this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.drainQueue();return}this.activeJobPhaseByKey.set(r,"processing"),n.postMessage({type:"generate",mediaId:r,dirSegments:i,source:a,sourceWidth:t.sourceWidth,sourceHeight:t.sourceHeight})}catch(n){this.failQueuedJob(r,`Failed to start proxy generation for ${r}`,n)}}failQueuedJob(t,r,n){this.activeJobPhaseByKey.delete(t),this.activeJobPriorityByKey.delete(t),this.generatingProxyKeys.delete(t),this.progressByProxyKey.delete(t),this.clearProgressEmissionState(t),n?Re.error(r,n):Re.error(r),this.emitStatusForProxyKey(t,"error"),this.drainQueue()}emitProgressThrottled(t,r){const n=Date.now(),a=this.progressEmissionByProxyKey.get(t)??{lastEmitAt:0,lastProgress:0,pendingProgress:null,timeoutId:null};if(r>=1||r<=0||r-a.lastProgress>=ag||n-a.lastEmitAt>=$l){a.timeoutId!==null&&(clearTimeout(a.timeoutId),a.timeoutId=null),a.pendingProgress=null,a.lastEmitAt=n,a.lastProgress=r,this.progressEmissionByProxyKey.set(t,a),this.emitStatusForProxyKey(t,"generating",r);return}a.pendingProgress=r,a.timeoutId===null&&(a.timeoutId=setTimeout(()=>{const o=this.progressEmissionByProxyKey.get(t);if(!o||(o.timeoutId=null,o.pendingProgress==null))return;const s=o.pendingProgress;o.pendingProgress=null,o.lastEmitAt=Date.now(),o.lastProgress=s,this.progressEmissionByProxyKey.set(t,o),this.emitStatusForProxyKey(t,"generating",s)},$l)),this.progressEmissionByProxyKey.set(t,a)}clearProgressEmissionState(t){const r=this.progressEmissionByProxyKey.get(t);r?.timeoutId!=null&&clearTimeout(r.timeoutId),this.progressEmissionByProxyKey.delete(t)}emitStatusForProxyKey(t,r,n){const a=this.mediaIdsByProxyKey.get(t);if(a&&a.size>0){for(const i of a)this.statusListener?.(i,r,n);return}this.statusListener?.(t,r,n)}}const pt=new cg;function Ud(){return{mint:()=>crypto.randomUUID()}}let Ya=Ud();function $(){return Ya.mint()}function Yl(e,t){const r=Ya;Ya=e;try{return t()}finally{Ya=r}}function Kr(e,t,r){return e.flatMap(n=>{const a=Math.max(t,n.startFrame),i=Math.min(r,n.endFrame);return i>a?[{...n,startFrame:a-t,endFrame:i-t}]:[]})}function ug(e,t,r){return[Kr(e,0,t),Kr(e,t,r)]}function dg(e,t,r){if(!(t>0&&r>0))throw new Error("Tốc độ phải lớn hơn 0.");const n=t/r;return e.map(a=>({...a,startFrame:Math.round(a.startFrame*n),endFrame:Math.max(Math.round(a.startFrame*n)+1,Math.round(a.endFrame*n))}))}const mg=.1,pg=16,si=1,Ct=30;function Vr(e,t){return e===void 0||!Number.isFinite(e)||e<=0?t:e}function na(e){return qt(e)?{sourceStart:e.sourceStart??e.trimStart??0,sourceEnd:e.sourceEnd,sourceDuration:e.sourceDuration,sourceFps:e.sourceFps,speed:e.speed??si}:{sourceStart:0,sourceEnd:void 0,sourceDuration:void 0,sourceFps:void 0,speed:si}}function vT(e,t){if(e.loop!==!0)return null;const r=t??e.sourceStart??e.trimStart??e.offset??0,n=e.sourceEnd??e.sourceDuration;return n===void 0||!Number.isFinite(n)||n<=r?null:{startFrames:r,endFrames:n}}function Ds(e,t=!1){if(e.type!=="video")return"not-video";if(e.embeddedAudioMuted!==!0||t)return"unmuted";const r=e.sourceStart??e.trimStart??e.offset??0,n=e.sourceEnd??e.sourceDuration;return n===void 0||!Number.isFinite(n)||n<=r?"no-window":null}function bT(e,t){const r=t.endSeconds-t.startSeconds;return!Number.isFinite(r)||r<=0?e:t.startSeconds+((e-t.startSeconds)%r+r)%r}function we(e,t,r=Ct,n=r){const a=Vr(r,Ct),i=Vr(n,a),o=e/a;return Math.round(o*i*t)}function Tt(e,t,r=Ct,n=r){const a=Vr(r,Ct),i=Vr(n,a),o=e/a;return Math.floor(o*i/t)}function Hd(e){const t=e.wrapperSpeed??si,r=e.wrapperSourceFps??e.fallbackSourceFps,n=e.wrapperSourceEnd??e.wrapperSourceStart+we(e.wrapperDuration,t,e.timelineFps,r),a=e.itemStart+e.itemDuration,i=Math.max(e.itemStart,e.wrapperSourceStart),o=Math.min(a,n);if(o<=i)return null;const s=Tt(i-e.wrapperSourceStart,t,r,e.timelineFps),l=Tt(o-e.wrapperSourceStart,t,r,e.timelineFps);return{overlapStart:i,overlapEnd:o,mappedFrom:s,mappedEnd:l,mappedDuration:Math.max(1,l-s),clippedStartFrames:i-e.itemStart,clippedEndFrames:a-o,wrapperSpeed:t,wrapperSourceFps:r}}function Nd(e,t){return Math.max(0,e-t)}function fg(e,t,r,n=Ct,a=n){const i=Nd(e,t);return Tt(i,r,n,a)}function Xi(e,t,r=Ct,n=r){return Tt(e,t,r,n)}function hg(e,t,r=Ct,n=r){if(t<=0)return si;const a=Vr(r,Ct),i=Vr(n,a);return e*i/(t*a)}function gg(e){return Math.max(mg,Math.min(pg,e))}function qt(e){return e.type==="video"||e.type==="audio"||e.type==="composition"||e.type==="shape"&&!!e.waveformVisualizer}function yg(e,t,r,n,a=Ct,i=a){const o=we(t,n,a,i),s=we(t+r,n,a,i);return{left:{sourceEnd:e+o},right:{sourceStart:e+o,sourceEnd:e+s}}}function vg(e,t){return e.type==="video"&&t.type==="audio"||e.type==="audio"&&t.type==="video"}function bg(e,t){return!vg(e,t)||!e.originId||e.originId!==t.originId||!e.mediaId||e.mediaId!==t.mediaId?!1:e.from===t.from&&e.durationInFrames===t.durationInFrames}function Sg(e,t,r){return t.id===e.id||t.type!==r?!1:e.linkedGroupId&&t.linkedGroupId?e.linkedGroupId===t.linkedGroupId:bg(e,t)}function Wr(e){return e.type==="audio"&&typeof e.compositionId=="string"&&e.compositionId.length>0}function Rs(e,t){return t.linkedGroupId?e.find(r=>r.id!==t.id&&Wr(r)&&r.linkedGroupId===t.linkedGroupId&&r.compositionId===t.compositionId)??null:null}function qd(e,t){return t.linkedGroupId?e.find(r=>r.type==="composition"&&r.linkedGroupId===t.linkedGroupId&&r.compositionId===t.compositionId)??null:null}function Gd(e,t){return t.type==="composition"?Rs(e,t):t.type!=="video"?null:e.find(r=>Sg(t,r,"audio"))??null}function ts(e,t){return Gd(e,t)!==null}function ST(e){const t=new Set;for(const r of e)r.type==="video"&&ts(e,r)&&t.add(r.id);return t}function Kd(e){const t=e.type==="composition"||Wr(e)?e.compositionId:null;if(t){const r=ae.getState().getComposition(t);if(r)return r.durationInFrames}return na(e).sourceDuration}function Nn(e,t,r,n=30){let a=r,i=null;if(qt(e)){const{sourceStart:o,sourceEnd:s,sourceFps:l,speed:c}=na(e),u=Kd(e),d=l??n,m=e.type==="video"&&e.loop===!0;if(t==="start")r<0&&!m&&(i=e.isReversed===!0&&u!==void 0?Xi(u-(s??u),c,d,n):Xi(o,c,d,n),-r>i&&(a=-i));else if(!m){if(e.isReversed===!0){const p=e.durationInFrames+Xi(o,c,d,n);i=p-e.durationInFrames,e.durationInFrames+r>p&&(a=p-e.durationInFrames)}else if(u!==void 0){const p=fg(u,o,c,d,n);i=p-e.durationInFrames,e.durationInFrames+r>p&&(a=p-e.durationInFrames)}}}return a=Ig(e.durationInFrames,t,a),{clampedAmount:a,maxExtend:i}}function qn(e,t,r,n,a){const i=e.from+e.durationInFrames;if(t==="end"&&r>0){let o=1/0;for(const s of n)s.id!==e.id&&s.trackId===e.trackId&&(a?.has(s.id)||s.from>=i&&(o=Math.min(o,s.from)));if(o!==1/0){const s=o-i;if(r>s)return s}}else if(t==="start"&&r<0){let o=-1/0;for(const s of n){if(s.id===e.id||s.trackId!==e.trackId||a?.has(s.id))continue;const l=s.from+s.durationInFrames;l<=e.from&&(o=Math.max(o,l))}if(o!==-1/0){const s=e.from-o;if(-r>s)return s>0?-s:0}}return r}function Ig(e,t,r){if(t==="start"){if(e-r<=0)return e-1}else if(e+r<=0)return-e+1;return r}function Xl(e,t,r,n,a=30){if(!qt(e)||e.type==="video"&&e.loop===!0)return null;const{sourceStart:i,sourceEnd:o,sourceFps:s,speed:l}=na(e),c=Kd(e),u=s??a;if(t==="start"){const d=we(r,l,a,u);if(e.isReversed===!0){const p=(o??c??i+1)-d;return{sourceEnd:Math.max(i+1,c!==void 0?Math.min(c,p):p)}}return{sourceStart:i+d}}else{const d=we(r,l,a,u);if(e.isReversed===!0){const S=o??c??i+1;return{sourceStart:Math.min(S-1,Math.max(0,i-d))}}const m=e.sourceEnd,p=i+we(n,l,a,u),f=m!==void 0?m+d:p,h=i+1,g=Math.max(h,f);return{sourceEnd:c!==void 0?Math.min(g,c):g}}}function Vd(e){return!!e.compositionId&&(e.type==="composition"||e.type==="audio")}function Ls(e){const t=new Set;for(const r of e)Vd(r)&&t.add(r.compositionId);return[...t]}function Wd(e,t,r,n=new Set){if(e===t)return!0;if(n.has(e))return!1;n.add(e);const a=r[e];if(!a)return!1;for(const i of Ls(a.items))if(i===t||Wd(i,t,r,n))return!0;return!1}function jd(e){const{parentCompositionId:t,insertedCompositionId:r,compositionById:n}=e;return t?Wd(r,t,n):!1}function $d(e,t,r=new Set){const n=[];for(const a of Ls(e)){if(r.has(a))continue;r.add(a),n.push(a);const i=t[a];i&&n.push(...$d(i.items,t,r))}return n}function IT(e,t){const r=e.flatMap(n=>n.items??[]);return $d(r,t)}const Yd=/^V(\d+)$/i,Xd=/^A(\d+)$/i;function Qd(e){return e==="video"?Yd:Xd}function xg(e){return e==="video"?"V":"A"}function Wt(e){return e.kind==="video"||e.kind==="audio"?e.kind:Yd.test(e.name)?"video":Xd.test(e.name)?"audio":null}function Bs(e,t){const r=Qd(t),n=new Set;for(const i of e){if(Wt(i)!==t)continue;const s=i.name.match(r),l=s?.[1]?Number.parseInt(s[1],10):NaN;Number.isFinite(l)&&l>0&&n.add(l)}let a=1;for(;n.has(a);)a+=1;return`${xg(t)}${a}`}function wg(e){const t=new Map,r=i=>e.filter(o=>!o.isGroup&&Wt(o)===i&&Qd(i).test(o.name));r("video").sort((i,o)=>o.order-i.order).forEach((i,o)=>t.set(i.id,`V${o+1}`)),r("audio").sort((i,o)=>i.order-o.order).forEach((i,o)=>t.set(i.id,`A${o+1}`));let n=!1;const a=e.map(i=>{const o=t.get(i.id);return o&&o!==i.name?(n=!0,{...i,name:o}):i});return n?a:e}function Jd(e,t,r){const n=[...e].sort((o,s)=>o.order-s.order),a=n.findIndex(o=>o.id===t.id);if(a===-1)return r==="above"?(t.order??0)-1:(t.order??0)+1;if(r==="above"){const o=n[a-1];return o?(o.order+t.order)/2:t.order-1}const i=n[a+1];return i?(t.order+i.order)/2:t.order+1}function Zd(e){const{tracks:t,kind:r,order:n,height:a=yr}=e;return{id:`track-${$()}`,name:Bs(t,r),kind:r,height:a,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,volume:0,order:n,items:[]}}function em(e){const{tracks:t,targetTrack:r,kind:n,direction:a}=e;return t.filter(o=>!o.isGroup).filter(o=>Wt(o)===n).filter(o=>a==="above"?o.order<r.order:o.order>r.order).sort((o,s)=>a==="above"?s.order-o.order:o.order-s.order)[0]??null}function tm(e){const t=new Set(e.filter(n=>!n.isGroup&&n.parentTrackId).map(n=>n.parentTrackId)),r=e.filter(n=>!n.isGroup||t.has(n.id));return r.length===e.length?e:r}function kg(e,t){const r=new Set(t.map(a=>a.trackId)),n=e.filter(a=>a.isGroup||!a.parentTrackId||r.has(a.id));return tm(n)}function rm(e){const t=new Map(e.filter(r=>r.isGroup).map(r=>[r.id,r]));return e.filter(r=>!r.isGroup).map(r=>{const n=r.parentTrackId?t.get(r.parentTrackId):void 0;return n?{...r,locked:r.locked||n.locked,muted:r.muted||n.muted,visible:r.visible!==!1&&n.visible!==!1,solo:r.solo||n.solo}:r})}const Pg={edit:{colorScopesOpen:!1,clipInspectorTab:"video",activeTab:"media",propertiesFullColumn:!1},color:{colorScopesOpen:!0,clipInspectorTab:"effects",activeTab:"effects",propertiesFullColumn:!0},motion:{colorScopesOpen:!1,clipInspectorTab:"video",activeTab:"media",propertiesFullColumn:!1}},Mg="edit";function Ql(e){return e==="color"?"color":e==="animate"||e==="motion"||e==="compose"?"motion":Mg}const Cg=["media","text","shapes","effects","transitions","lottie","transcript","ai"],Tg=["video","motion","audio","effects"];function Eg(e){return Cg.includes(e)}function Fg(e){return Tg.includes(e)}function Ag(e,t){const r=Pg[t];if(!e||typeof e!="object")return r;const n=e;return{colorScopesOpen:typeof n.colorScopesOpen=="boolean"?n.colorScopesOpen:r.colorScopesOpen,clipInspectorTab:Fg(n.clipInspectorTab)?n.clipInspectorTab:r.clipInspectorTab,activeTab:Eg(n.activeTab)?n.activeTab:r.activeTab,propertiesFullColumn:typeof n.propertiesFullColumn=="boolean"?n.propertiesFullColumn:r.propertiesFullColumn}}const Jl=320,nm="editor:workspace",am="editor:trackSizePreset";function _g(){try{const e=localStorage.getItem(am);if(e==="compact"||e==="medium"||e==="large")return e}catch{}return"medium"}function im(e){return`editor:workspaceLayout:${e}`}function Og(){try{return Ql(localStorage.getItem(nm))}catch{return Ql(null)}}function Dg(){try{return localStorage.getItem("editor:propertiesFullColumn")==="true"}catch{return!1}}function om(e){let t=null;try{const a=localStorage.getItem(im(e));t=a===null?null:JSON.parse(a)}catch{t=null}const r=Ag(t,e),n=typeof t?.propertiesFullColumn=="boolean";return e==="edit"&&!n?{...r,propertiesFullColumn:Dg()}:r}function br(e){return{colorScopesOpen:e.colorScopesOpen,clipInspectorTab:e.clipInspectorTab,activeTab:e.activeTab,propertiesFullColumn:e.propertiesFullColumn}}function Sr(e,t){try{localStorage.setItem(im(e),JSON.stringify(t))}catch{}}const sm=Og(),va=om(sm);function Zl(e,t,r){if(!Number.isFinite(e))return t;const n=e===Jl&&t!==Jl?t:e;return Math.min(r.maxWidth,Math.max(r.minWidth,n))}function ec(e,t){try{const r=localStorage.getItem(e);if(r!==null){const n=Number(r);return Number.isFinite(n)?n:t}}catch{}return t}const zs=Ie(e=>({activePanel:null,leftSidebarOpen:!0,rightSidebarOpen:!0,keyframeEditorShortcutScopeActive:!1,transcriptEditorShortcutScopeActive:!1,workspace:sm,activeTab:va.activeTab,clipInspectorTab:va.clipInspectorTab,sidebarWidth:ec("editor:sidebarWidth",Ml.leftSidebarDefaultWidth),rightSidebarWidth:ec("editor:rightSidebarWidth",Ml.rightSidebarDefaultWidth),timelineHeight:250,sourcePreviewMediaId:null,mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null,compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null,transcriptionDialogDepth:0,sourcePatchVideoEnabled:!0,sourcePatchAudioEnabled:!0,sourcePatchVideoTrackId:null,sourcePatchAudioTrackId:null,linkedSelectionEnabled:!0,colorScopesOpen:va.colorScopesOpen,mixerFloating:(()=>{try{return localStorage.getItem("editor:mixerFloating")==="true"}catch{return!1}})(),propertiesFullColumn:va.propertiesFullColumn,mediaFullColumn:(()=>{try{const t=localStorage.getItem("editor:mediaFullColumn");return t===null?!0:t==="true"}catch{return!0}})(),trackSizePreset:_g(),setActivePanel:t=>e({activePanel:t}),setLeftSidebarOpen:t=>e({leftSidebarOpen:t}),setRightSidebarOpen:t=>e({rightSidebarOpen:t}),setKeyframeEditorShortcutScopeActive:t=>e({keyframeEditorShortcutScopeActive:t}),setTranscriptEditorShortcutScopeActive:t=>e({transcriptEditorShortcutScopeActive:t}),toggleLeftSidebar:()=>e(t=>({leftSidebarOpen:!t.leftSidebarOpen})),toggleRightSidebar:()=>e(t=>({rightSidebarOpen:!t.rightSidebarOpen})),setWorkspace:t=>e(r=>{if(r.workspace===t)return r;const n=me.getState();n.pause(),n.setPreviewFrame(null),Sr(r.workspace,br(r));try{localStorage.setItem(nm,t)}catch{}return{workspace:t,...om(t)}}),setActiveTab:t=>e(r=>{const n={...r,activeTab:t};return Sr(r.workspace,br(n)),{activeTab:t}}),setClipInspectorTab:t=>e(r=>{const n={...r,clipInspectorTab:t};return Sr(r.workspace,br(n)),{clipInspectorTab:t}}),setSidebarWidth:t=>{try{localStorage.setItem("editor:sidebarWidth",String(t))}catch{}e({sidebarWidth:t})},setRightSidebarWidth:t=>{try{localStorage.setItem("editor:rightSidebarWidth",String(t))}catch{}e({rightSidebarWidth:t})},syncSidebarLayout:t=>e(r=>({sidebarWidth:Zl(r.sidebarWidth,t.leftSidebarDefaultWidth,gh({leftSidebarMinWidth:t.leftSidebarMinWidth,leftSidebarMaxWidth:t.leftSidebarMaxWidth})),rightSidebarWidth:Zl(r.rightSidebarWidth,t.rightSidebarDefaultWidth,hh({rightSidebarMinWidth:t.rightSidebarMinWidth,rightSidebarMaxWidth:t.rightSidebarMaxWidth}))})),setTimelineHeight:t=>e({timelineHeight:t}),setSourcePreviewMediaId:t=>e({sourcePreviewMediaId:t,mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null,compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null}),setMediaSkimPreview:(t,r=null)=>e(n=>{const a=t?r:null;return n.mediaSkimPreviewMediaId===t&&n.mediaSkimPreviewFrame===a&&n.compoundClipSkimPreviewCompositionId===null&&n.compoundClipSkimPreviewFrame===null?n:{mediaSkimPreviewMediaId:t,mediaSkimPreviewFrame:a,compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null}}),clearMediaSkimPreview:()=>e(t=>t.mediaSkimPreviewMediaId===null&&t.mediaSkimPreviewFrame===null?t:{mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null}),setCompoundClipSkimPreview:(t,r=null)=>e(n=>{const a=t?r:null;return n.compoundClipSkimPreviewCompositionId===t&&n.compoundClipSkimPreviewFrame===a&&n.mediaSkimPreviewMediaId===null&&n.mediaSkimPreviewFrame===null?n:{compoundClipSkimPreviewCompositionId:t,compoundClipSkimPreviewFrame:a,mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null}}),clearCompoundClipSkimPreview:()=>e(t=>t.compoundClipSkimPreviewCompositionId===null&&t.compoundClipSkimPreviewFrame===null?t:{compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null}),beginTranscriptionDialog:()=>e(t=>({transcriptionDialogDepth:t.transcriptionDialogDepth+1})),endTranscriptionDialog:()=>e(t=>({transcriptionDialogDepth:Math.max(0,t.transcriptionDialogDepth-1)})),setSourcePatchVideoEnabled:t=>e({sourcePatchVideoEnabled:t}),setSourcePatchAudioEnabled:t=>e({sourcePatchAudioEnabled:t}),setSourcePatchVideoTrackId:t=>e({sourcePatchVideoTrackId:t}),setSourcePatchAudioTrackId:t=>e({sourcePatchAudioTrackId:t}),toggleSourcePatchVideoEnabled:()=>e(t=>({sourcePatchVideoEnabled:!t.sourcePatchVideoEnabled})),toggleSourcePatchAudioEnabled:()=>e(t=>({sourcePatchAudioEnabled:!t.sourcePatchAudioEnabled})),setLinkedSelectionEnabled:t=>e({linkedSelectionEnabled:t}),toggleLinkedSelectionEnabled:()=>e(t=>({linkedSelectionEnabled:!t.linkedSelectionEnabled})),setColorScopesOpen:t=>e(r=>{const n={...r,colorScopesOpen:t};return Sr(r.workspace,br(n)),{colorScopesOpen:t}}),toggleColorScopesOpen:()=>e(t=>{const r=!t.colorScopesOpen,n={...t,colorScopesOpen:r};return Sr(t.workspace,br(n)),{colorScopesOpen:r}}),setMixerFloating:t=>{try{localStorage.setItem("editor:mixerFloating",String(t))}catch{}e({mixerFloating:t})},toggleMixerFloating:()=>e(t=>{const r=!t.mixerFloating;try{localStorage.setItem("editor:mixerFloating",String(r))}catch{}return{mixerFloating:r}}),togglePropertiesFullColumn:()=>e(t=>{const r=!t.propertiesFullColumn;try{localStorage.setItem("editor:propertiesFullColumn",String(r))}catch{}const n={...t,propertiesFullColumn:r};return Sr(t.workspace,br(n)),{propertiesFullColumn:r}}),toggleMediaFullColumn:()=>e(t=>{const r=!t.mediaFullColumn;try{localStorage.setItem("editor:mediaFullColumn",String(r))}catch{}return{mediaFullColumn:r}}),setTrackSizePreset:t=>{try{localStorage.setItem(am,t)}catch{}e({trackSizePreset:t})}}));let Rg=new Map;function lm(){return yh[zs.getState().trackSizePreset]}function Lg(e){return Rg.get(e)??lm()}let cm=null;function Us(){return cm}function Dt(e){cm=e}const W=Ie()((e,t)=>({fps:30,scrollPosition:0,snapEnabled:!0,audioSkimmingEnabled:!0,isDirty:!1,isTimelineLoading:!0,setFps:r=>e({fps:r}),setScrollPosition:r=>e({scrollPosition:r}),setSnapEnabled:r=>e({snapEnabled:r}),toggleSnap:()=>e(r=>({snapEnabled:!r.snapEnabled})),setAudioSkimmingEnabled:r=>e({audioSkimmingEnabled:r}),toggleAudioSkimming:()=>e(r=>({audioSkimmingEnabled:!r.audioSkimmingEnabled})),setIsDirty:r=>e({isDirty:r}),markDirty:()=>{t().isDirty||e({isDirty:!0})},markClean:()=>e({isDirty:!1}),setTimelineLoading:r=>e({isTimelineLoading:r})})),$e=Ie()(e=>({markers:[],inPoint:null,outPoint:null,setMarkers:t=>e({markers:t}),setInPoint:t=>e({inPoint:t}),setOutPoint:t=>e({outPoint:t}),setInOutPoints:(t,r)=>e({inPoint:t,outPoint:r}),addMarker:(t,r="#3B82F6",n="")=>e(a=>({markers:[...a.markers,{id:$(),frame:t,color:r,label:n}]})),updateMarker:(t,r)=>e(n=>({markers:n.markers.map(a=>a.id===t?{...a,...r}:a)})),removeMarker:t=>e(r=>({markers:r.markers.filter(n=>n.id!==t)})),clearAllMarkers:()=>e({markers:[]}),clearInOutPoints:()=>e({inPoint:null,outPoint:null})})),Bg=10;function tc(e,t,r){return Math.max(t,Math.min(r,e))}function rc(e){return e==null||!Number.isFinite(e)?null:Math.round(e)}function um(e,t){const r=e.reduce((i,o)=>Math.max(i,o.from+o.durationInFrames),0),n=Number.isFinite(t)&&t>0?t:30,a=Math.max(1,Math.floor(Bg*n));return Math.max(r,a)}function zg(e){const t=Number.isFinite(e)?Math.floor(e):0;return Math.max(1,t)}function dm(e){const t=Number.isFinite(e.maxFrame)?e.maxFrame:1,r=Math.max(1,Math.floor(t));let n=rc(e.inPoint),a=rc(e.outPoint);return n!==null&&(n=tc(n,0,r)),a!==null&&(a=tc(a,1,r)),n!==null&&a!==null&&n>=a?n>=r?{inPoint:Math.max(0,r-1),outPoint:r}:{inPoint:n,outPoint:Math.min(r,n+1)}:{inPoint:n,outPoint:a}}const Ug=-20,Hg=20,Ng=.3,qg=10.3,Gg=["low-shelf","peaking","high-shelf","high-pass"],Kg=["low-shelf","peaking","high-shelf","notch"],Vg=["low-pass","low-shelf","peaking","high-shelf"],Wg=[6,12,18,24],li=30,ci=20,ui=399,mm=120,pm=20,fm=22e3,hm=400,gm=20,ym=22e3,vm=1e3,bm=.9,cr=1.1,Sm=1600,Im=20,xm=22e3,Xa=1.1,wm=2800,km=20,Pm=22e3,di=22e3,mi=1400,pi=22e3,jg=1,ge=.001,$g=1.1,Yg=2.3,Xg=2.3,Qg=1.1,Y=Object.freeze({outputGainDb:0,band1Enabled:!1,band1Type:"high-pass",band1FrequencyHz:li,band1GainDb:0,band1Q:$g,band1SlopeDbPerOct:12,lowCutEnabled:!1,lowCutFrequencyHz:li,lowCutSlopeDbPerOct:12,lowEnabled:!0,lowType:"low-shelf",lowGainDb:0,lowFrequencyHz:mm,lowQ:Yg,lowMidEnabled:!0,lowMidType:"peaking",lowMidGainDb:0,lowMidFrequencyHz:hm,lowMidQ:cr,midGainDb:0,highMidEnabled:!0,highMidType:"peaking",highMidGainDb:0,highMidFrequencyHz:Sm,highMidQ:Xa,highEnabled:!0,highType:"high-shelf",highGainDb:0,highFrequencyHz:wm,highQ:Xg,band6Enabled:!1,band6Type:"low-pass",band6FrequencyHz:di,band6GainDb:0,band6Q:Qg,band6SlopeDbPerOct:12,highCutEnabled:!1,highCutFrequencyHz:di,highCutSlopeDbPerOct:12});function Ri(e,t){return Math.max(20,Math.min(e,t*.45))}function ot(e){return Number.isFinite(e)?Math.max(Ug,Math.min(Hg,e)):0}function bt(e,t=cr){return Number.isFinite(e)?Math.max(Ng,Math.min(qg,e)):t}function Jg(e,t=Y.band1Type){return Gg.includes(e)?e:t}function ba(e,t=Y.lowMidType){return Kg.includes(e)?e:t}function Zg(e,t=Y.band6Type){return Vg.includes(e)?e:t}function yt(e,t,r,n){return Number.isFinite(e)?Math.max(t,Math.min(r,e)):n}function Sn(e,t=12){return Wg.includes(e)?e:t}function _t(e,t){return e===void 0?t:e}function ey(e){return e==null?!1:"audioEqEnabled"in e&&e.audioEqEnabled===!1||"enabled"in e&&e.enabled===!1}function K(e,t,r,n){return e?.[r]??t?.[n]}function xT(e){return{enabled:e?.audioEqEnabled,outputGainDb:e?.audioEqOutputGainDb,band1Enabled:e?.audioEqBand1Enabled,band1Type:e?.audioEqBand1Type,band1FrequencyHz:e?.audioEqBand1FrequencyHz,band1GainDb:e?.audioEqBand1GainDb,band1Q:e?.audioEqBand1Q,band1SlopeDbPerOct:e?.audioEqBand1SlopeDbPerOct,lowCutEnabled:e?.audioEqLowCutEnabled,lowCutFrequencyHz:e?.audioEqLowCutFrequencyHz,lowCutSlopeDbPerOct:e?.audioEqLowCutSlopeDbPerOct,lowEnabled:e?.audioEqLowEnabled,lowType:e?.audioEqLowType,lowGainDb:e?.audioEqLowGainDb,lowFrequencyHz:e?.audioEqLowFrequencyHz,lowQ:e?.audioEqLowQ,lowMidEnabled:e?.audioEqLowMidEnabled,lowMidType:e?.audioEqLowMidType,lowMidGainDb:e?.audioEqLowMidGainDb,lowMidFrequencyHz:e?.audioEqLowMidFrequencyHz,lowMidQ:e?.audioEqLowMidQ,midGainDb:e?.audioEqMidGainDb,highMidEnabled:e?.audioEqHighMidEnabled,highMidType:e?.audioEqHighMidType,highMidGainDb:e?.audioEqHighMidGainDb,highMidFrequencyHz:e?.audioEqHighMidFrequencyHz,highMidQ:e?.audioEqHighMidQ,highEnabled:e?.audioEqHighEnabled,highType:e?.audioEqHighType,highGainDb:e?.audioEqHighGainDb,highFrequencyHz:e?.audioEqHighFrequencyHz,highQ:e?.audioEqHighQ,band6Enabled:e?.audioEqBand6Enabled,band6Type:e?.audioEqBand6Type,band6FrequencyHz:e?.audioEqBand6FrequencyHz,band6GainDb:e?.audioEqBand6GainDb,band6Q:e?.audioEqBand6Q,band6SlopeDbPerOct:e?.audioEqBand6SlopeDbPerOct,highCutEnabled:e?.audioEqHighCutEnabled,highCutFrequencyHz:e?.audioEqHighCutFrequencyHz,highCutSlopeDbPerOct:e?.audioEqHighCutSlopeDbPerOct}}function Hs(e){const t=e,r=e,n=_t(K(t,r,"lowCutEnabled","audioEqLowCutEnabled"),Y.lowCutEnabled),a=yt(Number(K(t,r,"lowCutFrequencyHz","audioEqLowCutFrequencyHz")),ci,ui,Y.lowCutFrequencyHz),i=Sn(Number(K(t,r,"lowCutSlopeDbPerOct","audioEqLowCutSlopeDbPerOct")),Y.lowCutSlopeDbPerOct),o=K(t,r,"band1Enabled","audioEqBand1Enabled"),s=K(t,r,"band1Type","audioEqBand1Type"),l=K(t,r,"band1FrequencyHz","audioEqBand1FrequencyHz"),c=K(t,r,"band1GainDb","audioEqBand1GainDb"),u=K(t,r,"band1Q","audioEqBand1Q"),d=K(t,r,"band1SlopeDbPerOct","audioEqBand1SlopeDbPerOct"),m=[o,s,l,c,u,d].some(xe=>xe!==void 0),p=m?Jg(s,Y.band1Type):"high-pass",f=_t(m?o:n,Y.band1Enabled),h=yt(Number(m?l:a),ci,ui,Y.band1FrequencyHz),g=ot(Number(c)),v=bt(Number(u),Y.band1Q),S=Sn(Number(m?d:i),Y.band1SlopeDbPerOct),b=_t(K(t,r,"highCutEnabled","audioEqHighCutEnabled"),Y.highCutEnabled),x=yt(Number(K(t,r,"highCutFrequencyHz","audioEqHighCutFrequencyHz")),mi,pi,Y.highCutFrequencyHz),T=Sn(Number(K(t,r,"highCutSlopeDbPerOct","audioEqHighCutSlopeDbPerOct")),Y.highCutSlopeDbPerOct),C=K(t,r,"band6Enabled","audioEqBand6Enabled"),k=K(t,r,"band6Type","audioEqBand6Type"),_=K(t,r,"band6FrequencyHz","audioEqBand6FrequencyHz"),P=K(t,r,"band6GainDb","audioEqBand6GainDb"),R=K(t,r,"band6Q","audioEqBand6Q"),E=K(t,r,"band6SlopeDbPerOct","audioEqBand6SlopeDbPerOct"),D=[C,k,_,P,R,E].some(xe=>xe!==void 0),L=D?Zg(k,Y.band6Type):"low-pass",U=_t(D?C:b,Y.band6Enabled),V=yt(Number(D?_:x),mi,pi,Y.band6FrequencyHz),ne=ot(Number(P)),ee=bt(Number(R),Y.band6Q),ze=Sn(Number(D?E:T),Y.band6SlopeDbPerOct);return{outputGainDb:ot(Number(K(t,r,"outputGainDb","audioEqOutputGainDb"))),band1Enabled:f,band1Type:p,band1FrequencyHz:h,band1GainDb:g,band1Q:v,band1SlopeDbPerOct:S,lowCutEnabled:f&&p==="high-pass",lowCutFrequencyHz:h,lowCutSlopeDbPerOct:S,lowEnabled:_t(K(t,r,"lowEnabled","audioEqLowEnabled"),Y.lowEnabled),lowType:ba(K(t,r,"lowType","audioEqLowType"),Y.lowType),lowGainDb:ot(Number(K(t,r,"lowGainDb","audioEqLowGainDb"))),lowFrequencyHz:yt(Number(K(t,r,"lowFrequencyHz","audioEqLowFrequencyHz")),pm,fm,Y.lowFrequencyHz),lowQ:bt(Number(K(t,r,"lowQ","audioEqLowQ")),Y.lowQ),lowMidEnabled:_t(K(t,r,"lowMidEnabled","audioEqLowMidEnabled"),Y.lowMidEnabled),lowMidType:ba(K(t,r,"lowMidType","audioEqLowMidType"),Y.lowMidType),lowMidGainDb:ot(Number(K(t,r,"lowMidGainDb","audioEqLowMidGainDb"))),lowMidFrequencyHz:yt(Number(K(t,r,"lowMidFrequencyHz","audioEqLowMidFrequencyHz")),gm,ym,Y.lowMidFrequencyHz),lowMidQ:bt(Number(K(t,r,"lowMidQ","audioEqLowMidQ")),Y.lowMidQ),midGainDb:ot(Number(K(t,r,"midGainDb","audioEqMidGainDb"))),highMidEnabled:_t(K(t,r,"highMidEnabled","audioEqHighMidEnabled"),Y.highMidEnabled),highMidType:ba(K(t,r,"highMidType","audioEqHighMidType"),Y.highMidType),highMidGainDb:ot(Number(K(t,r,"highMidGainDb","audioEqHighMidGainDb"))),highMidFrequencyHz:yt(Number(K(t,r,"highMidFrequencyHz","audioEqHighMidFrequencyHz")),Im,xm,Y.highMidFrequencyHz),highMidQ:bt(Number(K(t,r,"highMidQ","audioEqHighMidQ")),Y.highMidQ),highEnabled:_t(K(t,r,"highEnabled","audioEqHighEnabled"),Y.highEnabled),highType:ba(K(t,r,"highType","audioEqHighType"),Y.highType),highGainDb:ot(Number(K(t,r,"highGainDb","audioEqHighGainDb"))),highFrequencyHz:yt(Number(K(t,r,"highFrequencyHz","audioEqHighFrequencyHz")),km,Pm,Y.highFrequencyHz),highQ:bt(Number(K(t,r,"highQ","audioEqHighQ")),Y.highQ),band6Enabled:U,band6Type:L,band6FrequencyHz:V,band6GainDb:ne,band6Q:ee,band6SlopeDbPerOct:ze,highCutEnabled:U&&L==="low-pass",highCutFrequencyHz:V,highCutSlopeDbPerOct:ze}}function ty(e,t){return t==null||ey(t)?[...e??[]]:[...e??[],Hs(t)]}function wT(e,...t){let r=[...e??[]];for(const n of t)n!=null&&(r=ty(r,n));return r}function ry(e){if(e!=null)return{...e&&"enabled"in e&&e.enabled!==void 0?{enabled:!!e.enabled}:{},...Hs(e)}}function rs(e){return e?Math.abs(e.outputGainDb??0)>ge||!!e.band1Enabled&&e.band1Type==="high-pass"||!!e.band1Enabled&&e.band1Type!=="high-pass"&&Math.abs(e.band1GainDb??0)>ge||!!e.lowEnabled&&e.lowType==="notch"||!!e.lowEnabled&&Math.abs(e.lowGainDb??0)>ge||!!e.lowMidEnabled&&e.lowMidType==="notch"||!!e.lowMidEnabled&&Math.abs(e.lowMidGainDb??0)>ge||Math.abs(e.midGainDb??0)>ge||!!e.highMidEnabled&&e.highMidType==="notch"||!!e.highMidEnabled&&Math.abs(e.highMidGainDb??0)>ge||!!e.highEnabled&&e.highType==="notch"||!!e.highEnabled&&Math.abs(e.highGainDb??0)>ge||!!e.band6Enabled&&e.band6Type==="low-pass"||!!e.band6Enabled&&e.band6Type!=="low-pass"&&Math.abs(e.band6GainDb??0)>ge:!1}function kT(e,t){if(e===t)return!0;if(!e||!t)return!e&&!t;if(e.length!==t.length)return!1;for(let r=0;r<e.length;r++){const n=e[r],a=t[r];if(!n||!a||n.outputGainDb!==a.outputGainDb||n.band1Enabled!==a.band1Enabled||n.band1Type!==a.band1Type||n.band1FrequencyHz!==a.band1FrequencyHz||n.band1GainDb!==a.band1GainDb||n.band1Q!==a.band1Q||n.band1SlopeDbPerOct!==a.band1SlopeDbPerOct||n.lowCutEnabled!==a.lowCutEnabled||n.lowCutFrequencyHz!==a.lowCutFrequencyHz||n.lowCutSlopeDbPerOct!==a.lowCutSlopeDbPerOct||n.lowEnabled!==a.lowEnabled||n.lowType!==a.lowType||n.lowGainDb!==a.lowGainDb||n.lowFrequencyHz!==a.lowFrequencyHz||n.lowQ!==a.lowQ||n.lowMidEnabled!==a.lowMidEnabled||n.lowMidType!==a.lowMidType||n.lowMidGainDb!==a.lowMidGainDb||n.lowMidFrequencyHz!==a.lowMidFrequencyHz||n.lowMidQ!==a.lowMidQ||n.midGainDb!==a.midGainDb||n.highMidEnabled!==a.highMidEnabled||n.highMidType!==a.highMidType||n.highMidGainDb!==a.highMidGainDb||n.highMidFrequencyHz!==a.highMidFrequencyHz||n.highMidQ!==a.highMidQ||n.highEnabled!==a.highEnabled||n.highType!==a.highType||n.highGainDb!==a.highGainDb||n.highFrequencyHz!==a.highFrequencyHz||n.highQ!==a.highQ||n.band6Enabled!==a.band6Enabled||n.band6Type!==a.band6Type||n.band6FrequencyHz!==a.band6FrequencyHz||n.band6GainDb!==a.band6GainDb||n.band6Q!==a.band6Q||n.band6SlopeDbPerOct!==a.band6SlopeDbPerOct||n.highCutEnabled!==a.highCutEnabled||n.highCutFrequencyHz!==a.highCutFrequencyHz||n.highCutSlopeDbPerOct!==a.highCutSlopeDbPerOct)return!1}return!0}function Fe(e,t,r){return{id:e,label:t,settings:Object.freeze(Hs(r))}}Object.freeze([Fe("flat","Flat",{}),Fe("voice-clarity","Voice Clarity",{lowCutEnabled:!0,lowCutFrequencyHz:80,lowCutSlopeDbPerOct:12,lowGainDb:-1.5,lowFrequencyHz:120,lowMidGainDb:-2.5,lowMidFrequencyHz:320,lowMidQ:1.2,highMidGainDb:4.5,highMidFrequencyHz:2800,highMidQ:1.1,highGainDb:2,highFrequencyHz:7200}),Fe("podcast","Podcast",{lowCutEnabled:!0,lowCutFrequencyHz:70,lowCutSlopeDbPerOct:12,lowGainDb:-1,lowFrequencyHz:120,lowMidGainDb:-1.5,lowMidFrequencyHz:250,lowMidQ:1,highMidGainDb:5.5,highMidFrequencyHz:3e3,highMidQ:1.25,highGainDb:2.5,highFrequencyHz:9e3}),Fe("warmth","Warmth",{lowGainDb:4,lowFrequencyHz:110,lowMidGainDb:2,lowMidFrequencyHz:280,lowMidQ:.9,highMidGainDb:-1.5,highMidFrequencyHz:2600,highMidQ:1,highGainDb:-2.5,highFrequencyHz:6500}),Fe("bass-boost","Bass Boost",{lowGainDb:7,lowFrequencyHz:90,lowMidGainDb:2.5,lowMidFrequencyHz:180,lowMidQ:.85,highMidGainDb:-1,highMidFrequencyHz:2500,highMidQ:.9,highGainDb:.5,highFrequencyHz:7e3}),Fe("de-mud","De-Mud",{lowCutEnabled:!0,lowCutFrequencyHz:55,lowCutSlopeDbPerOct:12,lowGainDb:-1,lowFrequencyHz:120,lowMidGainDb:-5,lowMidFrequencyHz:300,lowMidQ:1.4,highMidGainDb:2,highMidFrequencyHz:2600,highMidQ:1,highGainDb:1,highFrequencyHz:7e3}),Fe("smile","Smile",{lowGainDb:3.5,lowFrequencyHz:95,lowMidGainDb:-2,lowMidFrequencyHz:420,lowMidQ:1.2,highMidGainDb:2.5,highMidFrequencyHz:2500,highMidQ:.9,highGainDb:4.5,highFrequencyHz:9500}),Fe("sparkle","Sparkle",{lowCutEnabled:!0,lowCutFrequencyHz:60,lowCutSlopeDbPerOct:6,lowGainDb:-2,lowFrequencyHz:100,lowMidGainDb:-1.5,lowMidFrequencyHz:350,lowMidQ:1.1,highMidGainDb:4,highMidFrequencyHz:3400,highMidQ:1,highGainDb:6,highFrequencyHz:9500}),Fe("air","Air",{lowCutEnabled:!0,lowCutFrequencyHz:75,lowCutSlopeDbPerOct:12,lowGainDb:-3,lowFrequencyHz:110,lowMidGainDb:-1.5,lowMidFrequencyHz:350,lowMidQ:.95,highMidGainDb:2,highMidFrequencyHz:2800,highMidQ:.85,highGainDb:7,highFrequencyHz:11e3}),Fe("soften","Soften",{lowGainDb:1.5,lowFrequencyHz:120,lowMidGainDb:1,lowMidFrequencyHz:280,lowMidQ:1,highMidGainDb:-2.5,highMidFrequencyHz:3200,highMidQ:1.05,highGainDb:-4.5,highFrequencyHz:8e3,highCutEnabled:!0,highCutFrequencyHz:14e3,highCutSlopeDbPerOct:12}),Fe("radio","Radio",{lowCutEnabled:!0,lowCutFrequencyHz:220,lowCutSlopeDbPerOct:18,lowGainDb:-6,lowFrequencyHz:250,lowMidGainDb:-3,lowMidFrequencyHz:500,lowMidQ:1.3,highMidGainDb:2,highMidFrequencyHz:1800,highMidQ:1,highGainDb:-6,highFrequencyHz:3500,highCutEnabled:!0,highCutFrequencyHz:4200,highCutSlopeDbPerOct:18}),Fe("telephone","Telephone",{outputGainDb:5,lowCutEnabled:!0,lowCutFrequencyHz:320,lowCutSlopeDbPerOct:24,lowGainDb:-8,lowFrequencyHz:240,lowMidGainDb:-3.5,lowMidFrequencyHz:520,lowMidQ:1.15,highMidGainDb:4.5,highMidFrequencyHz:1700,highMidQ:1.05,highGainDb:-9,highFrequencyHz:3600,highCutEnabled:!0,highCutFrequencyHz:3200,highCutSlopeDbPerOct:24}),Fe("dialog-lift","Dialog Lift",{lowCutEnabled:!0,lowCutFrequencyHz:75,lowCutSlopeDbPerOct:12,lowGainDb:-2,lowFrequencyHz:120,lowMidGainDb:-2,lowMidFrequencyHz:280,lowMidQ:1.1,highMidGainDb:3.5,highMidFrequencyHz:2400,highMidQ:1.2,highGainDb:1.5,highFrequencyHz:7e3}),Fe("rumble-cut","Rumble Cut",{lowCutEnabled:!0,lowCutFrequencyHz:100,lowCutSlopeDbPerOct:18,lowGainDb:-3,lowFrequencyHz:110,lowMidGainDb:-1,lowMidFrequencyHz:250,lowMidQ:1}),Fe("brighten","Brighten",{lowCutEnabled:!0,lowCutFrequencyHz:50,lowCutSlopeDbPerOct:6,lowGainDb:-1,lowFrequencyHz:120,lowMidGainDb:-1,lowMidFrequencyHz:350,lowMidQ:1,highMidGainDb:2.5,highMidFrequencyHz:3e3,highMidQ:1,highGainDb:5,highFrequencyHz:8500})]);function Ns(e,t,r,n){const a=Ri(e,r),i=bt(n,cr),o=Math.pow(10,t/40),s=2*Math.PI*(a/r),l=Math.cos(s),c=Math.sin(s)/(2*i),u=1+c*o,d=-2*l,m=1-c*o,p=1+c/o,f=-2*l,h=1-c/o;return{b0:u/p,b1:d/p,b2:m/p,a1:f/p,a2:h/p}}function ny(e,t,r){const n=Ri(e,t),a=bt(r,cr),i=2*Math.PI*(n/t),o=Math.cos(i),s=Math.sin(i)/(2*a),l=1,c=-2*o,u=1,d=1+s,m=-2*o,p=1-s;return{b0:l/d,b1:c/d,b2:u/d,a1:m/d,a2:p/d}}function ay(e,t,r,n){const a=Ri(t,n),i=Math.pow(10,r/40),o=2*Math.PI*(a/n),s=Math.cos(o),l=Math.sin(o),c=Math.sqrt(i),u=l/2*Math.sqrt((i+1/i)*(1/jg-1)+2);let d,m,p,f,h,g;return e==="lowshelf"?(d=i*(i+1-(i-1)*s+2*c*u),m=2*i*(i-1-(i+1)*s),p=i*(i+1-(i-1)*s-2*c*u),f=i+1+(i-1)*s+2*c*u,h=-2*(i-1+(i+1)*s),g=i+1+(i-1)*s-2*c*u):(d=i*(i+1+(i-1)*s+2*c*u),m=-2*i*(i-1+(i+1)*s),p=i*(i+1+(i-1)*s-2*c*u),f=i+1-(i-1)*s+2*c*u,h=2*(i-1-(i+1)*s),g=i+1-(i-1)*s-2*c*u),{b0:d/f,b1:m/f,b2:p/f,a1:h/f,a2:g/f}}function Mm(e){return e==="high-pass"}function Cm(e){return e==="low-pass"}function Tm(e){switch(e){case"low-shelf":return"lowshelf";case"peaking":return"peaking";case"high-shelf":return"highshelf";default:return null}}function Em(e){switch(e){case"low-shelf":return"lowshelf";case"peaking":return"peaking";case"high-shelf":return"highshelf";default:return null}}function Bt(e){switch(e){case"low-shelf":return"lowshelf";case"high-shelf":return"highshelf";case"notch":return"notch";default:return"peaking"}}function Ve(e,t,r,n,a){switch(e){case"lowshelf":case"highshelf":return ay(e,t,r,n);case"notch":return ny(t,n,a);default:return Ns(t,r,n,a)}}function Fm(e,t,r){const n=Ri(t,r),a=Math.tan(Math.PI*(n/r)),i=1/(1+a);return e==="highpass"?{b0:i,b1:-i,a1:(a-1)*i}:{b0:a*i,b1:a*i,a1:(a-1)*i}}function Am(e){return Math.max(1,Math.round(e/6))}function $t(e,t){const r=new Float32Array(e.length);let n=0,a=0,i=0,o=0;for(let s=0;s<e.length;s++){const l=e[s]??0,c=t.b0*l+t.b1*n+t.b2*a-t.a1*i-t.a2*o;r[s]=c,a=n,n=l,o=i,i=c}return r}function iy(e,t){const r=new Float32Array(e.length);let n=0,a=0;for(let i=0;i<e.length;i++){const o=e[i]??0,s=t.b0*o+t.b1*n-t.a1*a;r[i]=s,n=o,a=s}return r}function oy(e,t){if(Math.abs(t-1)<=ge)return e;const r=new Float32Array(e.length);for(let n=0;n<e.length;n++)r[n]=(e[n]??0)*t;return r}function nc(e,t,r,n,a){const i=Fm(t,r,a);let o=e;for(let s=0;s<Am(n);s++)o=iy(o,i);return o}function sy(e,t,r){let n=e;if(r.band1Enabled&&Mm(r.band1Type))n=nc(n,"highpass",r.band1FrequencyHz,r.band1SlopeDbPerOct,t);else if(r.band1Enabled){const a=Tm(r.band1Type);a&&(n=$t(n,Ve(a,r.band1FrequencyHz,r.band1GainDb,t,r.band1Q)))}if(r.lowEnabled&&(r.lowType==="notch"||Math.abs(r.lowGainDb)>ge)&&(n=$t(n,Ve(Bt(r.lowType),r.lowFrequencyHz,r.lowGainDb,t,r.lowQ))),r.lowMidEnabled&&(r.lowMidType==="notch"||Math.abs(r.lowMidGainDb)>ge)&&(n=$t(n,Ve(Bt(r.lowMidType),r.lowMidFrequencyHz,r.lowMidGainDb,t,r.lowMidQ))),Math.abs(r.midGainDb)>ge&&(n=$t(n,Ns(vm,r.midGainDb,t,bm))),r.highMidEnabled&&(r.highMidType==="notch"||Math.abs(r.highMidGainDb)>ge)&&(n=$t(n,Ve(Bt(r.highMidType),r.highMidFrequencyHz,r.highMidGainDb,t,r.highMidQ))),r.highEnabled&&(r.highType==="notch"||Math.abs(r.highGainDb)>ge)&&(n=$t(n,Ve(Bt(r.highType),r.highFrequencyHz,r.highGainDb,t,r.highQ))),r.band6Enabled&&Cm(r.band6Type))n=nc(n,"lowpass",r.band6FrequencyHz,r.band6SlopeDbPerOct,t);else if(r.band6Enabled){const a=Em(r.band6Type);a&&(n=$t(n,Ve(a,r.band6FrequencyHz,r.band6GainDb,t,r.band6Q)))}return Math.abs(r.outputGainDb)>ge&&(n=oy(n,Math.pow(10,r.outputGainDb/20))),n}function PT(e,t,r){return!r||r.length===0||!r.some(rs)?e:e.map(n=>{let a=n;for(const i of r)rs(i)&&(a=sy(a,t,i));return a})}class ly{constructor(t){this.coefficients=t}coefficients;x1=0;x2=0;y1=0;y2=0;processInto(t,r){const{b0:n,b1:a,b2:i,a1:o,a2:s}=this.coefficients;let{x1:l,x2:c,y1:u,y2:d}=this;for(let m=0;m<t.length;m++){const p=t[m],f=n*p+a*l+i*c-o*u-s*d;r[m]=f,c=l,l=p,d=u,u=f}this.x1=l,this.x2=c,this.y1=u,this.y2=d}}class cy{constructor(t){this.coefficients=t}coefficients;x1=0;y1=0;processInto(t,r){const{b0:n,b1:a,a1:i}=this.coefficients;let{x1:o,y1:s}=this;for(let l=0;l<t.length;l++){const c=t[l],u=n*c+a*o-i*s;r[l]=u,o=c,s=u}this.x1=o,this.y1=s}}class uy{filters=[];outputGain;constructor(t,r){const n=(i,o,s)=>{const l=Fm(i,o,t);for(let c=0;c<Am(s);c++)this.filters.push(new cy(l))},a=i=>{this.filters.push(new ly(i))};if(r.band1Enabled&&Mm(r.band1Type))n("highpass",r.band1FrequencyHz,r.band1SlopeDbPerOct);else if(r.band1Enabled){const i=Tm(r.band1Type);i&&a(Ve(i,r.band1FrequencyHz,r.band1GainDb,t,r.band1Q))}if(r.lowEnabled&&(r.lowType==="notch"||Math.abs(r.lowGainDb)>ge)&&a(Ve(Bt(r.lowType),r.lowFrequencyHz,r.lowGainDb,t,r.lowQ)),r.lowMidEnabled&&(r.lowMidType==="notch"||Math.abs(r.lowMidGainDb)>ge)&&a(Ve(Bt(r.lowMidType),r.lowMidFrequencyHz,r.lowMidGainDb,t,r.lowMidQ)),Math.abs(r.midGainDb)>ge&&a(Ns(vm,r.midGainDb,t,bm)),r.highMidEnabled&&(r.highMidType==="notch"||Math.abs(r.highMidGainDb)>ge)&&a(Ve(Bt(r.highMidType),r.highMidFrequencyHz,r.highMidGainDb,t,r.highMidQ)),r.highEnabled&&(r.highType==="notch"||Math.abs(r.highGainDb)>ge)&&a(Ve(Bt(r.highType),r.highFrequencyHz,r.highGainDb,t,r.highQ)),r.band6Enabled&&Cm(r.band6Type))n("lowpass",r.band6FrequencyHz,r.band6SlopeDbPerOct);else if(r.band6Enabled){const i=Em(r.band6Type);i&&a(Ve(i,r.band6FrequencyHz,r.band6GainDb,t,r.band6Q))}this.outputGain=Math.abs(r.outputGainDb)>ge?Math.pow(10,r.outputGainDb/20):1}}class MT{chains;scratchA=new Float32Array(0);scratchB=new Float32Array(0);constructor(t,r,n){const a=(n??[]).filter(rs);this.chains=[];for(let i=0;i<t;i++)this.chains.push(a.map(o=>new uy(r,o)))}get hasActiveStages(){return(this.chains[0]?.length??0)>0}processChunk(t,r){const n=this.chains[r]??[];if(n.length===0)return t;this.scratchA.length<t.length&&(this.scratchA=new Float32Array(t.length),this.scratchB=new Float32Array(t.length));let a=t,i=this.scratchA;const o=s=>s===this.scratchA?this.scratchB:this.scratchA;for(const s of n){for(const l of s.filters){const c=a===t?i:o(a),u=a.length===t.length?a:a.subarray(0,t.length);l.processInto(u,c),a!==t&&(i=a),a=c}if(s.outputGain!==1){const l=o(a),c=s.outputGain;for(let u=0;u<t.length;u++)l[u]=a[u]*c;a!==t&&(i=a),a=l}}return a}}function aa(e,t,r){const n=e.transform,a=r?.width??t.width,i=r?.height??t.height,o=t.width/a,s=t.height/i,l=Math.min(o,s),c=a*l,u=i*l,d=n?.width??c,m=n?.height??u,p=n?.anchorX??d/2,f=n?.anchorY??m/2;return{x:n?.x??0,y:n?.y??0,width:d,height:m,anchorX:p,anchorY:f,rotation:n?.rotation??0,opacity:n?.opacity??1,cornerRadius:n?.cornerRadius??0}}function Zr(e){if(e.type==="video"){const t=e;if(t.sourceWidth&&t.sourceHeight)return{width:t.sourceWidth,height:t.sourceHeight};if(t.mediaId){const r=Me.getState().mediaById[t.mediaId];if(r?.width&&r?.height)return{width:r.width,height:r.height}}}if(e.type==="image"||e.type==="lottie"){const t=e;if(t.sourceWidth&&t.sourceHeight)return{width:t.sourceWidth,height:t.sourceHeight};if(t.mediaId){const r=Me.getState().mediaById[t.mediaId];if(r?.width&&r?.height)return{width:r.width,height:r.height}}}if(e.type==="composition"){const t=e;return t.compositionWidth&&t.compositionHeight?{width:t.compositionWidth,height:t.compositionHeight}:void 0}}function _m(e,t){if(e.shapeType!=="path")return e;const r=t?.(e.id);return!r||r===e.pathVertices?e:{...e,pathVertices:r}}function CT(e,t){return e.type!=="shape"?e:_m(e,t)}const dy=[{value:"Roboto",label:"Roboto",family:"Roboto",weights:[100,200,300,400,500,600,700,800,900]},{value:"Open Sans",label:"Open Sans",family:"Open Sans",weights:[300,400,500,600,700,800]},{value:"Google Sans",label:"Google Sans",family:"Google Sans",weights:[400,500,600,700]},{value:"Noto Sans JP",label:"Noto Sans JP",family:"Noto Sans JP",weights:[100,200,300,400,500,600,700,800,900]},{value:"Inter",label:"Inter",family:"Inter",weights:[100,200,300,400,500,600,700,800,900]},{value:"Montserrat",label:"Montserrat",family:"Montserrat",weights:[100,200,300,400,500,600,700,800,900]},{value:"Poppins",label:"Poppins",family:"Poppins",weights:[100,200,300,400,500,600,700,800,900]},{value:"Lato",label:"Lato",family:"Lato",weights:[100,300,400,700,900]},{value:"Roboto Condensed",label:"Roboto Condensed",family:"Roboto Condensed",weights:[100,200,300,400,500,600,700,800,900]},{value:"Roboto Mono",label:"Roboto Mono",family:"Roboto Mono",weights:[100,200,300,400,500,600,700]},{value:"Arimo",label:"Arimo",family:"Arimo",weights:[400,500,600,700]},{value:"Oswald",label:"Oswald",family:"Oswald",weights:[200,300,400,500,600,700]},{value:"Noto Sans",label:"Noto Sans",family:"Noto Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Raleway",label:"Raleway",family:"Raleway",weights:[100,200,300,400,500,600,700,800,900]},{value:"Nunito",label:"Nunito",family:"Nunito",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Nunito Sans",label:"Nunito Sans",family:"Nunito Sans",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Playfair Display",label:"Playfair Display",family:"Playfair Display",weights:[400,500,600,700,800,900]},{value:"Rubik",label:"Rubik",family:"Rubik",weights:[300,400,500,600,700,800,900]},{value:"Ubuntu",label:"Ubuntu",family:"Ubuntu",weights:[300,400,500,700]},{value:"Roboto Slab",label:"Roboto Slab",family:"Roboto Slab",weights:[100,200,300,400,500,600,700,800,900]},{value:"DM Sans",label:"DM Sans",family:"DM Sans",weights:[100,200,300,400,500,600,700,800,900,1e3]},{value:"Merriweather",label:"Merriweather",family:"Merriweather",weights:[300,400,700,900]},{value:"Noto Sans KR",label:"Noto Sans KR",family:"Noto Sans KR",weights:[100,200,300,400,500,600,700,800,900]},{value:"Work Sans",label:"Work Sans",family:"Work Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"PT Sans",label:"PT Sans",family:"PT Sans",weights:[400,700]},{value:"Kanit",label:"Kanit",family:"Kanit",weights:[100,200,300,400,500,600,700,800,900]},{value:"Lora",label:"Lora",family:"Lora",weights:[400,500,600,700]},{value:"Mulish",label:"Mulish",family:"Mulish",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Fjalla One",label:"Fjalla One",family:"Fjalla One",weights:[400]},{value:"Manrope",label:"Manrope",family:"Manrope",weights:[200,300,400,500,600,700,800]},{value:"Archivo",label:"Archivo",family:"Archivo",weights:[100,200,300,400,500,600,700,800,900]},{value:"Quicksand",label:"Quicksand",family:"Quicksand",weights:[300,400,500,600,700]},{value:"Outfit",label:"Outfit",family:"Outfit",weights:[100,200,300,400,500,600,700,800,900]},{value:"Figtree",label:"Figtree",family:"Figtree",weights:[300,400,500,600,700,800,900]},{value:"Fira Sans",label:"Fira Sans",family:"Fira Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Inconsolata",label:"Inconsolata",family:"Inconsolata",weights:[200,300,400,500,600,700,800,900]},{value:"Barlow",label:"Barlow",family:"Barlow",weights:[100,200,300,400,500,600,700,800,900]},{value:"Bebas Neue",label:"Bebas Neue",family:"Bebas Neue",weights:[400]},{value:"Noto Sans TC",label:"Noto Sans TC",family:"Noto Sans TC",weights:[100,200,300,400,500,600,700,800,900]},{value:"Archivo Black",label:"Archivo Black",family:"Archivo Black",weights:[400]},{value:"Source Sans 3",label:"Source Sans 3",family:"Source Sans 3",weights:[200,300,400,500,600,700,800,900]},{value:"Hind Siliguri",label:"Hind Siliguri",family:"Hind Siliguri",weights:[300,400,500,600,700]},{value:"IBM Plex Sans",label:"IBM Plex Sans",family:"IBM Plex Sans",weights:[100,200,300,400,500,600,700]},{value:"Prompt",label:"Prompt",family:"Prompt",weights:[100,200,300,400,500,600,700,800,900]},{value:"Bungee",label:"Bungee",family:"Bungee",weights:[400]},{value:"Titillium Web",label:"Titillium Web",family:"Titillium Web",weights:[200,300,400,600,700,900]},{value:"Karla",label:"Karla",family:"Karla",weights:[200,300,400,500,600,700,800]},{value:"Heebo",label:"Heebo",family:"Heebo",weights:[100,200,300,400,500,600,700,800,900]},{value:"Saira",label:"Saira",family:"Saira",weights:[100,200,300,400,500,600,700,800,900]},{value:"Noto Serif",label:"Noto Serif",family:"Noto Serif",weights:[100,200,300,400,500,600,700,800,900]},{value:"PT Serif",label:"PT Serif",family:"PT Serif",weights:[400,700]},{value:"Jost",label:"Jost",family:"Jost",weights:[100,200,300,400,500,600,700,800,900]},{value:"Plus Jakarta Sans",label:"Plus Jakarta Sans",family:"Plus Jakarta Sans",weights:[200,300,400,500,600,700,800]},{value:"Bricolage Grotesque",label:"Bricolage Grotesque",family:"Bricolage Grotesque",weights:[200,300,400,500,600,700,800]},{value:"Noto Color Emoji",label:"Noto Color Emoji",family:"Noto Color Emoji",weights:[400]},{value:"Share Tech",label:"Share Tech",family:"Share Tech",weights:[400]},{value:"Smooch Sans",label:"Smooch Sans",family:"Smooch Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Source Code Pro",label:"Source Code Pro",family:"Source Code Pro",weights:[200,300,400,500,600,700,800,900]},{value:"Libre Baskerville",label:"Libre Baskerville",family:"Libre Baskerville",weights:[400,700]},{value:"Dancing Script",label:"Dancing Script",family:"Dancing Script",weights:[400,500,600,700]},{value:"Lobster Two",label:"Lobster Two",family:"Lobster Two",weights:[400,700]},{value:"EB Garamond",label:"EB Garamond",family:"EB Garamond",weights:[400,500,600,700,800]},{value:"Josefin Sans",label:"Josefin Sans",family:"Josefin Sans",weights:[100,200,300,400,500,600,700]},{value:"Cairo",label:"Cairo",family:"Cairo",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Libre Franklin",label:"Libre Franklin",family:"Libre Franklin",weights:[100,200,300,400,500,600,700,800,900]},{value:"Gravitas One",label:"Gravitas One",family:"Gravitas One",weights:[400]},{value:"Noto Serif JP",label:"Noto Serif JP",family:"Noto Serif JP",weights:[200,300,400,500,600,700,800,900]},{value:"Public Sans",label:"Public Sans",family:"Public Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Anton",label:"Anton",family:"Anton",weights:[400]},{value:"Mukta",label:"Mukta",family:"Mukta",weights:[200,300,400,500,600,700,800]},{value:"Schibsted Grotesk",label:"Schibsted Grotesk",family:"Schibsted Grotesk",weights:[400,500,600,700,800,900]},{value:"Dosis",label:"Dosis",family:"Dosis",weights:[200,300,400,500,600,700,800]},{value:"Barlow Condensed",label:"Barlow Condensed",family:"Barlow Condensed",weights:[100,200,300,400,500,600,700,800,900]},{value:"Bitter",label:"Bitter",family:"Bitter",weights:[100,200,300,400,500,600,700,800,900]},{value:"Roboto Flex",label:"Roboto Flex",family:"Roboto Flex",weights:[100,200,300,400,500,600,700,800,900,1e3]},{value:"Cabin",label:"Cabin",family:"Cabin",weights:[400,500,600,700]},{value:"Noto Sans SC",label:"Noto Sans SC",family:"Noto Sans SC",weights:[100,200,300,400,500,600,700,800,900]},{value:"Ramabhadra",label:"Ramabhadra",family:"Ramabhadra",weights:[400]},{value:"Noto Sans Telugu",label:"Noto Sans Telugu",family:"Noto Sans Telugu",weights:[100,200,300,400,500,600,700,800,900]},{value:"Nanum Gothic",label:"Nanum Gothic",family:"Nanum Gothic",weights:[400,700,800]},{value:"Space Grotesk",label:"Space Grotesk",family:"Space Grotesk",weights:[300,400,500,600,700]},{value:"Changa One",label:"Changa One",family:"Changa One",weights:[400]},{value:"Anek Telugu",label:"Anek Telugu",family:"Anek Telugu",weights:[100,200,300,400,500,600,700,800]},{value:"Assistant",label:"Assistant",family:"Assistant",weights:[200,300,400,500,600,700,800]},{value:"Oxygen",label:"Oxygen",family:"Oxygen",weights:[300,400,700]},{value:"Pacifico",label:"Pacifico",family:"Pacifico",weights:[400]},{value:"Alfa Slab One",label:"Alfa Slab One",family:"Alfa Slab One",weights:[400]},{value:"Lexend",label:"Lexend",family:"Lexend",weights:[100,200,300,400,500,600,700,800,900]},{value:"Hind",label:"Hind",family:"Hind",weights:[300,400,500,600,700]},{value:"M PLUS Rounded 1c",label:"M PLUS Rounded 1c",family:"M PLUS Rounded 1c",weights:[100,300,400,500,700,800,900]},{value:"Red Hat Display",label:"Red Hat Display",family:"Red Hat Display",weights:[300,400,500,600,700,800,900]},{value:"Exo 2",label:"Exo 2",family:"Exo 2",weights:[100,200,300,400,500,600,700,800,900]},{value:"Slabo 27px",label:"Slabo 27px",family:"Slabo 27px",weights:[400]},{value:"Lobster",label:"Lobster",family:"Lobster",weights:[400]},{value:"Cormorant Garamond",label:"Cormorant Garamond",family:"Cormorant Garamond",weights:[300,400,500,600,700]},{value:"Sora",label:"Sora",family:"Sora",weights:[100,200,300,400,500,600,700,800]},{value:"Crimson Text",label:"Crimson Text",family:"Crimson Text",weights:[400,600,700]},{value:"Inter Tight",label:"Inter Tight",family:"Inter Tight",weights:[100,200,300,400,500,600,700,800,900]},{value:"Overpass",label:"Overpass",family:"Overpass",weights:[100,200,300,400,500,600,700,800,900]},{value:"Caveat",label:"Caveat",family:"Caveat",weights:[400,500,600,700]},{value:"Urbanist",label:"Urbanist",family:"Urbanist",weights:[100,200,300,400,500,600,700,800,900]},{value:"Comfortaa",label:"Comfortaa",family:"Comfortaa",weights:[300,400,500,600,700]},{value:"Tajawal",label:"Tajawal",family:"Tajawal",weights:[200,300,400,500,700,800,900]},{value:"Arvo",label:"Arvo",family:"Arvo",weights:[400,700]},{value:"PT Sans Narrow",label:"PT Sans Narrow",family:"PT Sans Narrow",weights:[400,700]},{value:"Rajdhani",label:"Rajdhani",family:"Rajdhani",weights:[300,400,500,600,700]},{value:"DM Serif Display",label:"DM Serif Display",family:"DM Serif Display",weights:[400]},{value:"Abel",label:"Abel",family:"Abel",weights:[400]},{value:"Teko",label:"Teko",family:"Teko",weights:[300,400,500,600,700]},{value:"Source Serif 4",label:"Source Serif 4",family:"Source Serif 4",weights:[200,300,400,500,600,700,800,900]},{value:"Noto Sans Arabic",label:"Noto Sans Arabic",family:"Noto Sans Arabic",weights:[100,200,300,400,500,600,700,800,900]},{value:"Almarai",label:"Almarai",family:"Almarai",weights:[300,400,700,800]},{value:"Merriweather Sans",label:"Merriweather Sans",family:"Merriweather Sans",weights:[300,400,500,600,700,800]},{value:"Lexend Deca",label:"Lexend Deca",family:"Lexend Deca",weights:[100,200,300,400,500,600,700,800,900]},{value:"Satisfy",label:"Satisfy",family:"Satisfy",weights:[400]},{value:"Orbitron",label:"Orbitron",family:"Orbitron",weights:[400,500,600,700,800,900]},{value:"Barlow Semi Condensed",label:"Barlow Semi Condensed",family:"Barlow Semi Condensed",weights:[100,200,300,400,500,600,700,800,900]},{value:"Domine",label:"Domine",family:"Domine",weights:[400,500,600,700]},{value:"Shadows Into Light",label:"Shadows Into Light",family:"Shadows Into Light",weights:[400]},{value:"M PLUS 1p",label:"M PLUS 1p",family:"M PLUS 1p",weights:[100,300,400,500,700,800,900]}],my={normal:400,medium:500,semibold:600,bold:700},ac=new Map;function py(e){const t=[...new Set(e)].filter(r=>Number.isFinite(r)&&r>0).sort((r,n)=>r-n);return t.length>0?t:[400]}function ic(e,t){return{family:e,weights:py(t),display:"swap"}}function fy(e,t=e,r=[400]){if(typeof t=="string"){ac.set(e,ic(t,r));return}ac.set(e,ic(e,t))}function hy(e){for(const t of e)fy(t.value,t.family,t.weights)}hy(dy);function gy(e){if(Array.isArray(e.textSpans)&&e.textSpans.length>0){const t=e.textSpans.filter(r=>typeof r.text=="string").map(r=>({...r}));if(t.length>0)return t}return[{text:e.text??""}]}function yy(e){return(e.split(`
`)[0]?.trim()??"")||"Text"}const He={fontSize:60,fontFamily:"Inter",fontWeight:"normal",fontStyle:"normal",lineHeight:1.2,letterSpacing:0,textAlign:"center",verticalAlign:"middle",textPadding:16,color:"#ffffff",underline:!1};function vy(e,t,r,n){return`${e} ${t} ${r}px "${n}", sans-serif`}function Om(e){return{lineHeight:e.lineHeight??He.lineHeight,letterSpacing:e.letterSpacing??He.letterSpacing,textAlign:e.textAlign??He.textAlign,verticalAlign:e.verticalAlign??He.verticalAlign,textPadding:Math.max(0,e.textPadding??He.textPadding),color:e.color??He.color,backgroundColor:e.backgroundColor,backgroundRadius:Math.max(0,e.backgroundRadius??0),textShadow:e.textShadow,stroke:e.stroke}}function by(e){const t=e.fontSize??He.fontSize,r=e.fontFamily??He.fontFamily,n=e.fontStyle??He.fontStyle,a=e.fontWeight??He.fontWeight,i=e.letterSpacing??He.letterSpacing,o=e.color??He.color,s=e.underline??He.underline;return gy(e).map(l=>{const c=l.fontSize??t,u=l.fontFamily??r,d=l.fontStyle??n,m=l.fontWeight??a,p=my[m]??400;return{text:l.text??"",fontSize:c,fontFamily:u,fontStyle:d,fontWeightName:m,fontWeight:p,letterSpacing:l.letterSpacing??i,color:l.color??o,underline:l.underline??s,cssFont:vy(d,p,c,u)}})}function Sy(e,t,r,n,a){const i=[];let o="";for(const s of e){const l=o+s;a.measure(l,t,r)>n&&o?(i.push(o),o=s):o=l}return o&&i.push(o),i}function Iy(e,t,r,n,a){const i=[];for(const o of e.split(`
`)){if(o===""){i.push("");continue}let s="";for(const l of o.split(" ")){const c=s?`${s} ${l}`:l;if(a.measure(c,t,r)>n&&s){if(i.push(s),s=l,a.measure(l,t,r)>n){const u=Sy(l,t,r,n,a);for(let d=0;d<u.length-1;d++)i.push(u[d]??"");s=u[u.length-1]??""}}else s=c}s&&i.push(s)}return i.length>0?i:[""]}function xy(e){const t=[[]];let r=null,n="",a=[];const i=()=>{const l={text:"",charStyles:[],leading:n,leadingStyles:a};return n="",a=[],l},o=()=>{r&&t[t.length-1].push(r),r=null},s=()=>{n.length>0&&t[t.length-1].push(i())};for(let l=0;l<e.length;l++)for(const c of e[l].text)c===`
`?(o(),s(),t.push([])):c===" "?(o(),n+=c,a.push(l)):(r||(r=i()),r.text+=c,r.charStyles.push(l));return o(),s(),t}function wy(e,t,r,n){const a=[];let i=0;for(let o=1;o<=e.length;o++){if(o!==e.length&&t[o]===t[i])continue;const s=r[t[i]??0]??r[0],l=i===0?0:n(e.slice(0,i));a.push({text:e.slice(i,o),color:s.color,underline:s.underline,offsetX:l,width:n(e.slice(0,o))-l}),i=o}return a}function ky(e,t,r,n){const a=e[0],i=n.fontMetrics(a.cssFont),o=a.fontSize*t,l=(o-(i.ascent+i.descent))/2+i.ascent,c=xy(e),u=p=>n.measure(p,a.cssFont,a.letterSpacing),d=[],m=(p,f=!1)=>{let h="";const g=[];for(const[S,b]of p.entries())(S>0||!f)&&(h+=b.leading,g.push(...b.leadingStyles)),h+=b.text,g.push(...b.charStyles);const v=wy(h,g,e,u);d.push({text:h,cssFont:a.cssFont,fontSize:a.fontSize,color:a.color,letterSpacing:a.letterSpacing,underline:!1,width:u(h),top:0,baselineY:l,startX:0,lineHeightPx:o,runs:v})};for(const p of c){if(p.length===0){m([]);continue}let f=[],h="",g=!1;for(const v of p){const S=f.length>0||!g?v.leading:"",b=h+S+v.text;u(b)>r&&f.length>0?(m(f,g),g=!0,f=[v],h=v.text):(f.push(v),h=b)}f.length>0&&m(f,g)}return d}function Py(e,t,r,n){const a=Om(e),i=by(e),o=a.textPadding,s=Math.max(1,t-o*2),l=r-o*2,c=e.spanLayout==="inline"&&i.length>0,u=c?ky(i,a.lineHeight,s,n):[];if(!c)for(const h of i){const g=n.fontMetrics(h.cssFont),v=h.fontSize*a.lineHeight,b=(v-(g.ascent+g.descent))/2+g.ascent;for(const x of Iy(h.text,h.cssFont,h.letterSpacing,s,n))u.push({text:x,cssFont:h.cssFont,fontSize:h.fontSize,color:h.color,letterSpacing:h.letterSpacing,underline:h.underline,width:n.measure(x,h.cssFont,h.letterSpacing),top:0,baselineY:b,startX:0,lineHeightPx:v})}const d=u.reduce((h,g)=>h+g.lineHeightPx,0),m=a.verticalAlign==="top"?o:a.verticalAlign==="bottom"?r-o-d:o+(l-d)/2;let p=m;for(const h of u)h.top=p,h.baselineY=p+h.baselineY,h.startX=a.textAlign==="left"?o:a.textAlign==="right"?t-o-h.width:(t-h.width)/2,p+=h.lineHeightPx;let f;if(a.backgroundColor&&u.length>0){const h=Math.max(...u.map(b=>b.width)),g=a.textAlign==="left"?o+h/2:a.textAlign==="right"?t-o-h/2:t/2,v=Math.min(t,h+o*2),S=d+o*2;f={x:g-v/2,y:m-o,width:v,height:S,radius:Math.max(0,Math.min(a.backgroundRadius,v/2,S/2))}}return{lines:u,totalHeight:d,background:f}}function TT(e){return Math.max(0,e.width-e.letterSpacing)}function ns(e){const t=/(\d+(?:\.\d+)?)px/.exec(e);return t?parseFloat(t[1]):16}function My(e,t){"letterSpacing"in e&&(e.letterSpacing=`${t}px`)}function Cy(e,t){return"fontKerning"in e&&(e.fontKerning="normal"),{measure(r,n,a){return e.font!==n&&(e.font=n),My(e,a),t?t(r,a):e.measureText(r).width},fontMetrics(r){e.font!==r&&(e.font=r);const n=ns(r),a=e.measureText("Hg");return{ascent:a.fontBoundingBoxAscent||n*.8,descent:a.fontBoundingBoxDescent||n*.2}}}}let ht;function Ty(){return ht!==void 0||typeof OffscreenCanvas<"u"&&(ht=new OffscreenCanvas(1,1).getContext("2d"),ht)?ht:typeof document<"u"?(ht=document.createElement("canvas").getContext("2d"),ht):(ht=null,ht)}function Ey(){const e=Ty();return e?Cy(e):{measure:(t,r,n)=>{const a=ns(r);return t.length*a*.6+t.length*n},fontMetrics:t=>{const r=ns(t);return{ascent:r*.8,descent:r*.2}}}}function Fy(e,t){return e}function Ay(e,t,r){const n=Fy(e),a=Om(n),i=Py(n,t,0,Ey()),o=(a.stroke?.width??0)*2,s=a.textShadow?Math.abs(a.textShadow.offsetY)+a.textShadow.blur:0;return i.totalHeight+a.textPadding*2+o+s*2}function _y(e,t,r){const n=Ay(e,t.width);return n<=t.height+.5?t:{...t,height:n}}const be=2,Oy=new Set(["position","scale","anchor"]);function pr(e){return Oy.has(e)}const Dy=new Set(["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius"]);function Li(e){return Dy.has(e)}const Ry=new Set(["trimPathStart","trimPathEnd","trimPathOffset","taperStartWidth","taperEndWidth","taperStartLength","taperEndLength"]);function Dm(e){return Ry.has(e)}function Ly(e){return Li(e)||Dm(e)}function Rm(e){return Ly(e)||pr(e)}const as={position:["x","y"],scale:["width","height"],anchor:["anchorX","anchorY"]};function By(e){return as[e]}function zy(e,t){return e===t?!0:pr(e)?as[e].includes(t):pr(t)?as[t].includes(e):!1}function mt(e){if(!e)return[];const t=(e.expressions??[]).filter(r=>r.type==="link");return[...e.propertyLinks??[],...t]}function Ar(e){return(e?.expressions??[]).filter(t=>t.type==="expression")}function Uy(e,t={}){return{...e,...t,value:{...e.value},...e.temporalEase&&{temporalEase:{...e.temporalEase.in&&{in:{...e.temporalEase.in}},...e.temporalEase.out&&{out:{...e.temporalEase.out}}}},...e.spatial&&{spatial:{...e.spatial,inTangent:{...e.spatial.inTangent},outTangent:{...e.spatial.outTangent}}}}}const qs={x:"X Position",y:"Y Position",width:"Width",height:"Height",anchorX:"Anchor X",anchorY:"Anchor Y",rotation:"Rotation",opacity:"Opacity",cornerRadius:"Layer Radius",cropLeft:"Crop Left",cropRight:"Crop Right",cropTop:"Crop Top",cropBottom:"Crop Bottom",cropSoftness:"Crop Softness",volume:"Volume (dB)",textStyleScale:"Style Preset Scale",fontSize:"Font Size",lineHeight:"Line Height",textPadding:"Text Padding",backgroundRadius:"Background Radius",textShadowOffsetX:"Shadow X",textShadowOffsetY:"Shadow Y",textShadowBlur:"Shadow Blur",strokeWidth:"Stroke Width",trimPathStart:"Trim Paths Start",trimPathEnd:"Trim Paths End",trimPathOffset:"Trim Paths Offset",taperStartWidth:"Taper Start Width",taperEndWidth:"Taper End Width",taperStartLength:"Taper Start Length",taperEndLength:"Taper End Length"},Hy=new Set(Object.keys(qs));function Ny(e){return Hy.has(e)}function Gs(e,t,r){return`effect:${e}:${t}:${r}`}function Gn(e){if(!e.startsWith("effect:"))return null;const[,t="",r="",n=""]=e.split(":");return!t||!r||!n?null:{gpuEffectType:t,effectId:r,paramKey:n}}function qy(e,t){return`pathVertex:${e}:${t}`}function Ks(e){const t=/^pathVertex:(\d+):(positionX|positionY|inX|inY|outX|outY)$/.exec(e);return t?{vertexIndex:Number(t[1]),component:t[2]}:null}function Gy(e){if(Ny(e))return qs[e];const t=Ks(e);if(t){const n={positionX:"X",positionY:"Y",inX:"In X",inY:"In Y",outX:"Out X",outY:"Out Y"};return`Vertex ${t.vertexIndex+1} ${n[t.component]}`}const r=Gn(e);return r?r.paramKey:e}new Proxy({...qs},{get(e,t){if(typeof t=="string")return e[t]??Gy(t)}});const Lm={tension:170,friction:26,mass:1},Bm={x1:.42,y1:0,x2:.58,y2:1};function Ky(e){return e?e.topLeft[0]!==0||e.topLeft[1]!==0||e.topRight[0]!==0||e.topRight[1]!==0||e.bottomRight[0]!==0||e.bottomRight[1]!==0||e.bottomLeft[0]!==0||e.bottomLeft[1]!==0:!1}function ET(e,t,r){if(!e)return;const n=e.referenceWidth&&e.referenceWidth>1e-6?e.referenceWidth:t,a=e.referenceHeight&&e.referenceHeight>1e-6?e.referenceHeight:r,i=n>1e-6?t/n:1,o=a>1e-6?r/a:1,s=([l,c])=>[l*i,c*o];return{topLeft:s(e.topLeft),topRight:s(e.topRight),bottomRight:s(e.bottomRight),bottomLeft:s(e.bottomLeft)}}function Vy(e,t,r){return r?.sourceWidth!==void 0&&r.sourceWidth>0&&r.sourceHeight!==void 0&&r.sourceHeight>0?Ph(r.sourceWidth,r.sourceHeight,e,t,r.crop,r.fitMode).mediaRect:{x:0,y:0,width:e,height:t}}function zm(e,t,r){const n=r.topLeft[0],a=r.topLeft[1],i=e+r.topRight[0],o=r.topRight[1],s=e+r.bottomRight[0],l=t+r.bottomRight[1],c=r.bottomLeft[0],u=t+r.bottomLeft[1],d=i-s,m=c-s,p=n-i+s-c,f=o-l,h=u-l,g=a-o+l-u,v=d*h-f*m;if(Math.abs(v)<1e-10)return[1,0,0,0,1,0,0,0,1];const S=(p*h-g*m)/v,b=(d*g-f*p)/v,x=(i-n+S*i)/e,T=(c-n+b*c)/t,C=n,k=(o-a+S*o)/e,_=(u-a+b*u)/t;return[x,T,C,k,_,a,S/e,b/t,1]}function Wy(e){const[t,r,n,a,i,o,s,l,c]=e,u=i*c-o*l,d=n*l-r*c,m=r*o-n*i,p=o*s-a*c,f=t*c-n*s,h=n*a-t*o,g=a*l-i*s,v=r*s-t*l,S=t*i-r*a,b=t*u+r*p+n*g;if(Math.abs(b)<1e-10)return null;const x=1/b;return[u*x,d*x,m*x,p*x,f*x,h*x,g*x,v*x,S*x]}function In(e,t,r){const n=e[6]*t+e[7]*r+e[8];return Math.abs(n)<1e-10?[t,r]:[(e[0]*t+e[1]*r+e[2])/n,(e[3]*t+e[4]*r+e[5])/n]}function oc(e,t,r,n,a,i,o,s,l,c,u,d,m,p){e.save();const f=(l+u+m)/3,h=(c+d+p)/3,g=1.5,v=(ne,ee)=>{const ze=ne-f,xe=ee-h,O=Math.sqrt(ze*ze+xe*xe);return O<1e-6?[ne,ee]:[ne+ze/O*g,ee+xe/O*g]},[S,b]=v(l,c),[x,T]=v(u,d),[C,k]=v(m,p);e.beginPath(),e.moveTo(S,b),e.lineTo(x,T),e.lineTo(C,k),e.closePath(),e.clip();const _=r*(i-s)+a*(s-n)+o*(n-i);if(Math.abs(_)<1e-10){e.restore();return}const P=1/_,R=(l*(i-s)+u*(s-n)+m*(n-i))*P,E=(c*(i-s)+d*(s-n)+p*(n-i))*P,D=(l*(o-a)+u*(r-o)+m*(a-r))*P,L=(c*(o-a)+d*(r-o)+p*(a-r))*P,U=(l*(a*s-o*i)+u*(o*n-r*s)+m*(r*i-a*n))*P,V=(c*(a*s-o*i)+d*(o*n-r*s)+p*(r*i-a*n))*P;e.setTransform(R,E,D,L,U,V),e.drawImage(t,0,0),e.restore()}function FT(e,t,r,n,a,i,o,s=16,l="mesh"){if(l==="projective"){Yy(e,t,r,n,a,i,o);return}const c=zm(r,n,o);for(let u=0;u<s;u++)for(let d=0;d<s;d++){const m=d/s*r,p=u/s*n,f=(d+1)/s*r,h=(u+1)/s*n,[g,v]=In(c,m,p),[S,b]=In(c,f,p),[x,T]=In(c,f,h),[C,k]=In(c,m,h);oc(e,t,m,p,f,p,f,h,a+g,i+v,a+S,i+b,a+x,i+T),oc(e,t,m,p,f,h,m,h,a+g,i+v,a+x,i+T,a+C,i+k)}}const sc=16384;function lc(e,t){const r=Math.max(1,Math.ceil(e)),n=Math.max(1,Math.ceil(t));if(typeof OffscreenCanvas<"u"){const o=new OffscreenCanvas(r,n),s=o.getContext("2d");return s?{canvas:o,ctx:s}:null}if(typeof document>"u")return null;const a=document.createElement("canvas");a.width=r,a.height=n;const i=a.getContext("2d");return i?{canvas:a,ctx:i}:null}function jy(e,t,r,n){const a=Math.floor(r),i=Math.floor(n),o=Math.min(t-1,a+1),s=Math.min(Math.floor(e.length/(t*4))-1,i+1),l=r-a,c=n-i,u=(i*t+a)*4,d=(i*t+o)*4,m=(s*t+a)*4,p=(s*t+o)*4,f=e[u+3]/255,h=e[d+3]/255,g=e[m+3]/255,v=e[p+3]/255,S=(k,_,P,R)=>{const E=k*(1-l)+_*l,D=P*(1-l)+R*l;return E*(1-c)+D*c},b=S(e[u]*f,e[d]*h,e[m]*g,e[p]*v),x=S(e[u+1]*f,e[d+1]*h,e[m+1]*g,e[p+1]*v),T=S(e[u+2]*f,e[d+2]*h,e[m+2]*g,e[p+2]*v),C=S(f,h,g,v);return C<=0?[0,0,0,0]:[b/C,x/C,T/C,C*255]}function $y(e,t,r,n){const a=lc(t,r);if(!a)return null;a.ctx.clearRect(0,0,t,r),a.ctx.drawImage(e,0,0,t,r);let i;try{i=a.ctx.getImageData(0,0,t,r)}catch{return null}const o=[[n.topLeft[0],n.topLeft[1]],[t+n.topRight[0],n.topRight[1]],[t+n.bottomRight[0],r+n.bottomRight[1]],[n.bottomLeft[0],r+n.bottomLeft[1]]],s=Math.floor(Math.min(...o.map(([g])=>g)))-1,l=Math.floor(Math.min(...o.map(([,g])=>g)))-1,c=Math.ceil(Math.max(...o.map(([g])=>g)))+1,u=Math.ceil(Math.max(...o.map(([,g])=>g)))+1,d=Math.max(1,c-s),m=Math.max(1,u-l);if(d>sc||m>sc)return null;const p=lc(d,m);if(!p)return null;const f=p.ctx.createImageData(d,m),h=Wy(zm(t,r,n));if(!h)return null;for(let g=0;g<m;g++){const v=l+g+.5;for(let S=0;S<d;S++){const b=s+S+.5,[x,T]=In(h,b,v);if(x<0||T<0||x>t-1||T>r-1)continue;const C=jy(i.data,t,x,T),k=(g*d+S)*4;f.data[k]=C[0],f.data[k+1]=C[1],f.data[k+2]=C[2],f.data[k+3]=C[3]}}return p.ctx.putImageData(f,0,0),{canvas:p.canvas,offsetX:s,offsetY:l}}function Yy(e,t,r,n,a,i,o){const s=$y(t,r,n,o);s&&e.drawImage(s.canvas,a+s.offsetX,i+s.offsetY)}function Um(e){return e}function Xy(e){return 0}function Hm(e){return e*e}function Nm(e){return e*(2-e)}function qm(e){return e<.5?2*e*e:-1+(4-2*e)*e}function Bi(e,t){const{x1:r,y1:n,x2:a,y2:i}=t;if(e===0)return 0;if(e===1)return 1;const o=3*r,s=3*(a-r)-o,l=1-o-s,c=3*n,u=3*(i-n)-c,d=1-c-u;let m=e;for(let p=0;p<8;p++){const h=((l*m+s)*m+o)*m-e;if(Math.abs(h)<1e-6)break;const g=(3*l*m+2*s)*m+o;if(Math.abs(g)<1e-6)break;m-=h/g,m=Math.max(0,Math.min(1,m))}return((d*m+u)*m+c)*m}function Gm(e,t){const{tension:r,friction:n,mass:a}=t;if(e===0)return 0;if(e===1)return 1;const i=Math.sqrt(r/a),o=n/(2*Math.sqrt(r*a)),s=4/(o*i),l=e*s;let c;if(o<1){const u=i*Math.sqrt(1-o*o);c=1-Math.exp(-o*i*l)*(Math.cos(u*l)+o*i/u*Math.sin(u*l))}else if(o===1)c=1-Math.exp(-i*l)*(1+i*l);else{const u=-i*(o-Math.sqrt(o*o-1)),d=-i*(o+Math.sqrt(o*o-1));c=1-(d*Math.exp(u*l)-u*Math.exp(d*l))/(d-u)}return Math.max(0,Math.min(1.2,c))}const Qy={linear:Um,"ease-in":Hm,"ease-out":Nm,"ease-in-out":qm,hold:Xy,"cubic-bezier":e=>Bi(e,Bm),spring:e=>Gm(e,Lm)};function Km(e){return Qy[e]??Um}function Vs(e,t){const r=Math.max(0,Math.min(1,e));return Km(t)(r)}function Ws(e,t){const r=Math.max(0,Math.min(1,e));switch(t.type){case"cubic-bezier":return Bi(r,t.bezier??Bm);case"spring":return Gm(r,t.spring??Lm);default:return Km(t.type)(r)}}function Jy(e,t,r){const n=t.frame-e.frame;if(n<=0)return e.value;const a=(r-e.frame)/n,i=e.easingConfig?Ws(a,e.easingConfig):Vs(a,e.easing);return e.value+(t.value-e.value)*i}function en(e,t,r){if(e.length===0)return r;const n=e[0];if(e.length===1||t<=n.frame)return n.value;const a=e[e.length-1];if(t>=a.frame)return a.value;for(let i=0;i<e.length-1;i++){const o=e[i],s=e[i+1];if(o.frame<=t&&s.frame>t)return Jy(o,s,t)}return r}function ia(e,t){return e?e.properties.find(n=>n.property===t)?.keyframes??[]:[]}const AT=["trimPathStart","trimPathEnd","trimPathOffset","taperStartWidth","taperEndWidth","taperStartLength","taperEndLength"],Zy={trimPathStart:0,trimPathEnd:100,trimPathOffset:0,taperStartWidth:100,taperEndWidth:100,taperStartLength:0,taperEndLength:0};function ev(e,t){return e[t]??Zy[t]}const tv=30,rv=100/3;function nv(e){return Math.max(0,Math.min(100,e))/100}function cc(e,t,r){return r?Ws(e,r):Vs(e,t)}function av(e,t){return Math.hypot(t.x-e.x,t.y-e.y)}function uc(e,t){return{influence:nv(e?.influence??rv),speed:Math.max(0,e?.speed??t)}}function iv(e,t,r,n){const a=e.temporalEase?.out,i=t.temporalEase?.in;if(!a&&!i)return cc(r,e.easing,e.easingConfig);const s=(t.frame-e.frame)/Math.max(1,n),l=av(e.value,t.value);if(s<=0||l<=Number.EPSILON)return cc(r,e.easing,e.easingConfig);const c=l/s,u=uc(a,c),d=uc(i,c);return Bi(r,{x1:u.influence,y1:u.influence*(u.speed/c),x2:1-d.influence,y2:1-d.influence*(d.speed/c)})}function ov(e,t,r){return{x:e.x+(t.x-e.x)*r,y:e.y+(t.y-e.y)*r}}function sv(e,t,r,n,a){const i=1-a,o=i*i*i,s=3*i*i*a,l=3*i*a*a,c=a*a*a;return{x:o*e.x+s*t.x+l*r.x+c*n.x,y:o*e.y+s*t.y+l*r.y+c*n.y}}function lv(e,t,r){return e==="position"&&!!(t.spatial||r.spatial)}function cv(e,t,r){const n=e.spatial?.outTangent??{x:0,y:0},a=t.spatial?.inTangent??{x:0,y:0};return sv(e.value,{x:e.value.x+n.x,y:e.value.y+n.y},{x:t.value.x+a.x,y:t.value.y+a.y},t.value,r)}function uv(e,t,r,n,a){const i=r.frame-t.frame;if(i<=0||t.easing==="hold")return t.value;const o=(n-t.frame)/i,s=iv(t,r,o,a);return lv(e,t,r)?cv(t,r,s):ov(t.value,r.value,s)}function Qa(e,t,r,n,a=tv){if(t.length===0)return n;const i=t[0];if(t.length===1||r<=i.frame)return i.value;const o=t[t.length-1];if(r>=o.frame)return o.value;for(let s=0;s<t.length-1;s++){const l=t[s],c=t[s+1];if(l.frame<=r&&c.frame>r)return uv(e,l,c,r,a)}return n}function Qi(e,t){return e?.vectorProperties?.find(r=>r.property===t)?.keyframes??[]}const dv=2048,mv=512,pv=64;function wt(e){return typeof e!="number"}function Vm(e,t){return wt(e)?{x:t(e.x),y:t(e.y)}:t(e)}function jr(e,t,r){if(wt(e)&&wt(t))return{x:r(e.x,t.x),y:r(e.y,t.y)};if(wt(e)){const n=t;return{x:r(e.x,n),y:r(e.y,n)}}if(wt(t)){const n=e;return{x:r(n,t.x),y:r(n,t.y)}}return r(e,t)}function fv(e){if((wt(e)?[e.x,e.y]:[e]).some(r=>!Number.isFinite(r)))throw new Error("Expression produced a non-finite value");return e}function hv(e,t){let r=t,n=0;for(;r<e.length&&/[0-9.]/.test(e[r]);)e[r]==="."&&(n+=1),r+=1;const a=e.slice(t,r);if(n>1||a==="."||!Number.isFinite(Number(a)))throw new Error(`Invalid number at ${t+1}`);return{token:{type:"number",value:a,position:t},nextIndex:r}}function gv(e,t){let r=t+1;for(;r<e.length&&/[A-Za-z0-9_]/.test(e[r]);)r+=1;return{token:{type:"identifier",value:e.slice(t,r),position:t},nextIndex:r}}function yv(e,t){const r=e[t];let n=t+1,a="";for(;n<e.length&&e[n]!==r;){if(e[n]==="\\"&&(n+=1),n>=e.length)throw new Error(`Unterminated string at ${t+1}`);a+=e[n],n+=1}if(e[n]!==r)throw new Error(`Unterminated string at ${t+1}`);return{token:{type:"string",value:a,position:t},nextIndex:n+1}}function vv(e,t){const r=e[t];if(/[0-9.]/.test(r))return hv(e,t);if(/[A-Za-z_]/.test(r))return gv(e,t);if(r==='"'||r==="'")return yv(e,t);if("+-*/(),[]".includes(r))return{token:{type:"punctuation",value:r,position:t},nextIndex:t+1};throw new Error(`Unexpected character "${r}" at ${t+1}`)}function bv(e){if(e.length>dv)throw new Error("Expression is too long");const t=[];let r=0;for(;r<e.length;){if(/\s/.test(e[r])){r+=1;continue}const n=vv(e,r);if(t.push(n.token),r=n.nextIndex,t.length>mv)throw new Error("Expression has too many tokens")}return t.push({type:"eof",value:"",position:e.length}),t}function Ji(e,t){if(t.length!==1)throw new Error(`${e} expects 1 argument`);const r={abs:Math.abs,sin:Math.sin,cos:Math.cos};return Vm(t[0],r[e])}function dc(e,t){if(t.length<2)throw new Error(`${e} expects at least 2 arguments`);const r=e==="min"?Math.min:Math.max;return t.slice(1).reduce((n,a)=>jr(n,a,r),t[0])}function Sv(e){if(e.length!==3)throw new Error("clamp expects 3 arguments");return jr(jr(e[0],e[1],Math.max),e[2],Math.min)}function Iv(e,t){if(e.length!==3)throw new Error("lerp expects 3 arguments");const r=t(e[2],"lerp amount");return jr(e[0],e[1],(n,a)=>n+(a-n)*r)}const xv={abs:e=>Ji("abs",e),sin:e=>Ji("sin",e),cos:e=>Ji("cos",e),min:e=>dc("min",e),max:e=>dc("max",e),clamp:Sv,lerp:Iv};function wv(e,t,r){const n=xv[e];if(n)return n(t,r);throw new Error(`Unknown function "${e}"`)}class kv{constructor(t,r){this.tokens=t,this.context=r}tokens;context;index=0;depth=0;parse(){const t=this.parseAdditive();if(this.current().type!=="eof")throw new Error(`Unexpected token at ${this.current().position+1}`);return fv(t)}current(){return this.tokens[this.index]}consume(t){const r=this.current();if(t!==void 0&&r.value!==t)throw new Error(`Expected "${t}" at ${r.position+1}`);return this.index+=1,r}withDepth(t){if(this.depth+=1,this.depth>pv)throw new Error("Expression is too deeply nested");try{return t()}finally{this.depth-=1}}parseAdditive(){let t=this.parseMultiplicative();for(;this.current().value==="+"||this.current().value==="-";){const r=this.consume().value,n=this.parseMultiplicative();t=jr(t,n,r==="+"?(a,i)=>a+i:(a,i)=>a-i)}return t}parseMultiplicative(){let t=this.parseUnary();for(;this.current().value==="*"||this.current().value==="/";){const r=this.consume().value,n=this.parseUnary();if(r==="/"&&(wt(n)?n.x===0||n.y===0:n===0))throw new Error("Division by zero");t=jr(t,n,r==="*"?(a,i)=>a*i:(a,i)=>a/i)}return t}parseUnary(){return this.current().value==="+"?(this.consume("+"),this.parseUnary()):this.current().value==="-"?(this.consume("-"),Vm(this.parseUnary(),t=>-t)):this.parsePrimary()}parsePrimary(){return this.withDepth(()=>this.parsePrimaryValue())}parsePrimaryValue(){const t=this.current();if(t.type==="number")return Number(this.consume().value);if(t.value==="(")return this.parseGroupedValue();if(t.value==="[")return this.parseVectorValue();if(t.type==="identifier")return this.parseIdentifierValue();throw new Error(`Expected a value at ${t.position+1}`)}parseGroupedValue(){this.consume("(");const t=this.parseAdditive();return this.consume(")"),t}parseVectorValue(){this.consume("[");const t=this.requireScalar(this.parseAdditive(),"Vector X");this.consume(",");const r=this.requireScalar(this.parseAdditive(),"Vector Y");return this.consume("]"),{x:t,y:r}}parseIdentifierValue(){const t=this.consume().value;if(t==="value"||t==="preValue")return this.context.preValue;if(t==="frame")return this.context.globalFrame;if(t==="time")return this.context.globalFrame/Math.max(1,this.context.fps);if(this.current().value!=="(")throw new Error(`Unknown identifier "${t}"`);return this.parseCall(t)}parseCall(t){if(this.consume("("),t==="prop")return this.parsePropertyReference();const r=this.parseCallArguments();return this.consume(")"),wv(t,r,(n,a)=>this.requireScalar(n,a))}parseCallArguments(){const t=[];if(this.current().value===")")return t;for(;;){if(t.push(this.parseAdditive()),this.current().value!==",")return t;this.consume(",")}}parsePropertyReference(){const t=this.consumeString("Layer ID");this.consume(",");const r=this.consumeString("Property");if(this.consume(")"),!Rm(r))throw new Error(`Unknown property "${r}"`);const n=this.context.resolveProperty(t,r);if(n===null)throw new Error("Property reference is unavailable");return n}consumeString(t){const r=this.current();if(r.type!=="string")throw new Error(`${t} must be a quoted string`);return this.consume(),r.value}requireScalar(t,r){if(wt(t))throw new Error(`${r} must be a scalar`);return t}}function Pv(e,t){return pr(e)===wt(t)}function Mv(e,t){try{return{value:new kv(bv(e),t).parse()}}catch(r){return{value:t.preValue,error:r instanceof Error?r.message:"Expression failed"}}}const Cv=["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius"];function Wm(e,t,r){let n;if(Li(t))return Ys(aa(e,r.canvas,Zr(e)),r.getKeyframes(e.id),r.globalFrame-e.from)[t];if(Dm(t)&&e.type==="shape")n=ev(e,t);else return null;const a=ia(r.getKeyframes(e.id),t),i=r.globalFrame-e.from;return en(a,i,n)}function js(e,t,r,n,a={cache:new Map,vectorCache:new Map,expressionCache:new Map,active:new Set}){const i=`${e}:${t}`,o=`${i}@${n.globalFrame}`,s=a.cache.get(o);if(s!==void 0)return s;if(a.active.has(i))return r;const l=mt(n.getKeyframes(e)).find(f=>f.targetProperty===t&&f.enabled);if(!l)return a.cache.set(o,r),r;const c=n.getItem(l.sourceItemId);if(!c)return a.cache.set(o,r),r;a.active.add(i);const u=l.timeOffsetFrames===0?n:{...n,globalFrame:n.globalFrame-l.timeOffsetFrames,getPreviewTransform:void 0};if(pr(l.sourceProperty))return a.active.delete(i),a.cache.set(o,r),r;const d=Wm(c,l.sourceProperty,u);if(d===null)return a.active.delete(i),a.cache.set(o,r),r;const m=js(c.id,l.sourceProperty,d,u,a),p=Tv(c.id,l.sourceProperty,m,u);return a.active.delete(i),a.cache.set(o,p),p}function jm(e,t,r){const n=aa(e,r.canvas,Zr(e)),a=Ys(n,r.getKeyframes(e.id),r.globalFrame-e.from);return t==="position"?{x:a.x,y:a.y}:t==="anchor"?{x:a.anchorX,y:a.anchorY}:{x:n.width===0?100:a.width/n.width*100,y:n.height===0?100:a.height/n.height*100}}function Tv(e,t,r,n){if(!Li(t))return r;const a=n.getPreviewTransform?.(e)?.[t];return typeof a=="number"?a:r}function Ev(e,t,r,n){const a=n.getPreviewTransform?.(e.id);return a?t==="position"?Fv(a,r):t==="anchor"?Av(a,r):_v(e,a,r,n.canvas):r}function Fv(e,t){return{x:e.x??t.x,y:e.y??t.y}}function Av(e,t){return{x:e.anchorX??t.x,y:e.anchorY??t.y}}function _v(e,t,r,n){const a=aa(e,n,Zr(e));return{x:mc(t.width,r.x,a.width),y:mc(t.height,r.y,a.height)}}function mc(e,t,r){return e===void 0||r===0?t:e/r*100}function $s(e,t,r,n,a){const i=`${e}:${t}`,o=`${i}@${n.globalFrame}`,s=a.vectorCache.get(o);if(s)return s;if(a.active.has(i))return r;const l=mt(n.getKeyframes(e)).find(f=>f.targetProperty===t&&f.enabled);if(!l||!pr(l.sourceProperty))return a.vectorCache.set(o,r),r;const c=n.getItem(l.sourceItemId);if(!c)return a.vectorCache.set(o,r),r;a.active.add(i);const u=l.timeOffsetFrames===0?n:{...n,globalFrame:n.globalFrame-l.timeOffsetFrames,getPreviewTransform:void 0},d=jm(c,l.sourceProperty,u),m=$s(c.id,l.sourceProperty,d,u,a),p=Ev(c,l.sourceProperty,m,u);return a.active.delete(i),a.vectorCache.set(o,p),p}function Ov(e,t,r,n){const a=r.getItem(e);if(!a)return null;if(pr(t)){const s=jm(a,t,r),l=$s(a.id,t,s,r,n);return fi(a.id,t,l,r,n).value}const i=Wm(a,t,r);if(i===null)return null;const o=js(a.id,t,i,r,n);return fi(a.id,t,o,r,n).value}function fi(e,t,r,n,a={cache:new Map,vectorCache:new Map,expressionCache:new Map,active:new Set}){const i=Ar(n.getKeyframes(e)).find(u=>u.targetProperty===t&&u.enabled);if(!i)return{value:r};const o=`expression:${e}:${t}`,s=`${o}@${n.globalFrame}`,l=a.expressionCache.get(s);if(l!==void 0)return{value:l};if(a.active.has(o))return{value:r,error:"Expression dependency cycle"};a.active.add(o);const c=Mv(i.source,{preValue:r,globalFrame:n.globalFrame,fps:n.canvas.fps,resolveProperty:(u,d)=>Ov(u,d,n,a)});return a.active.delete(o),c.error||!Pv(t,c.value)?{value:r,error:c.error??"Expression result has the wrong value type"}:(a.expressionCache.set(s,c.value),c)}function Dv(e,t,r,n){const a={...e},i=Qi(t,"position"),o=Qi(t,"scale"),s=Qi(t,"anchor");if(i.length>0){const l=Qa("position",i,r,{x:e.x,y:e.y},n);a.x=l.x,a.y=l.y}if(o.length>0){const l=Qa("scale",o,r,{x:100,y:100},n);a.width=e.width*(l.x/100),a.height=e.height*(l.y/100)}if(s.length>0){const l=Qa("anchor",s,r,{x:e.anchorX,y:e.anchorY},n);a.anchorX=l.x,a.anchorY=l.y}return{result:a,hasPosition:i.length>0,hasScale:o.length>0,hasAnchor:s.length>0}}function Rv(e,t){return t.hasPosition&&(e==="x"||e==="y")||t.hasScale&&(e==="width"||e==="height")?!0:t.hasAnchor&&(e==="anchorX"||e==="anchorY")}function $m(e,t,r){return e==="position"?{x:r.result.x,y:r.result.y}:e==="anchor"?{x:r.result.anchorX,y:r.result.anchorY}:{x:t.width===0?100:r.result.width/t.width*100,y:t.height===0?100:r.result.height/t.height*100}}function Ym(e,t,r,n){if(e==="position"){n.result.x=t.x,n.result.y=t.y,n.hasPosition=!0;return}if(e==="anchor"){n.result.anchorX=t.x,n.result.anchorY=t.y,n.hasAnchor=!0;return}n.result.width=r.width*(t.x/100),n.result.height=r.height*(t.y/100),n.hasScale=!0}function Lv(e){const{baseResolved:t,itemKeyframes:r,vectorAnimation:n,context:a,state:i}=e;for(const o of["position","scale","anchor"]){if(!mt(r).some(u=>u.targetProperty===o&&u.enabled))continue;const l=$m(o,t,n),c=$s(r.itemId,o,l,a,i);Ym(o,c,t,n)}}function Bv(e){const{baseResolved:t,itemKeyframes:r,vectorAnimation:n,context:a,state:i}=e;for(const o of["position","scale","anchor"]){if(!Ar(r).some(u=>u.targetProperty===o&&u.enabled))continue;const l=$m(o,t,n),c=fi(r.itemId,o,l,a,i).value;typeof c!="number"&&Ym(o,c,t,n)}}function zv(e){const{property:t,baseResolved:r,vectorAnimation:n,itemKeyframes:a,frame:i,expressionContext:o,expressionState:s}=e,l=ia(a,t),c=Rv(t,n),u=c?n.result[t]:r[t],d=c?u:en(l,i,u);if(o){const m=js(a.itemId,t,d,o,s),p=fi(a.itemId,t,m,o,s).value;return typeof p=="number"?p:m}return l.length>0?d:n.result[t]}function Ys(e,t,r,n){if(!t)return e;const a=Dv(e,t,r,n?.canvas.fps),i={cache:new Map,vectorCache:new Map,expressionCache:new Map,active:new Set};n&&(Lv({baseResolved:e,itemKeyframes:t,vectorAnimation:a,context:n,state:i}),Bv({baseResolved:e,itemKeyframes:t,vectorAnimation:a,context:n,state:i}));for(const o of Cv)a.result[o]=zv({property:o,baseResolved:e,vectorAnimation:a,itemKeyframes:t,frame:r,expressionContext:n,expressionState:i});return a.result}function Uv(e){return e?e.properties.some(t=>t.keyframes.length>0)||(e.vectorProperties?.some(t=>t.keyframes.length>0)??!1)||mt(e).some(t=>t.enabled)||(e.expressions?.some(t=>t.type==="expression"&&t.enabled)??!1):!1}const Ne=Math.PI*2,Hv={"float-drift":["x","y","rotation"],"breath-pulse":["width","height","opacity"],"micro-shake":["x","y","rotation"],sway:["rotation"],spin:["rotation"],wander:["x","y","width","height"]};function Vt(e,t,r){return Math.max(t,Math.min(r,e))}function _e(e,t){if(!Hv[e.type].includes(t))return 0;const r=e.channelGains?.[t];return typeof r=="number"&&Number.isFinite(r)?Vt(r,0,2):1}function Dn(e){const t=Math.sin(e*12.9898)*43758.5453;return(t-Math.floor(t))*2-1}function Nv(e){return e*e*(3-2*e)}function Zi(e,t){const r=Math.floor(t),n=t-r,a=Dn(e+r),i=Dn(e+r+1);return a+(i-a)*Nv(n)}const pc={dx:0,dy:0,dRotation:0,dOpacity:0,scaleWidth:1,scaleHeight:1};function qv(e,t,r){const n=t.frame/Math.max(1,t.fps),a=e.phaseFrames/Math.max(1,t.fps)*e.frequency*Ne,i=Vt(t.frameWidth*.008,4,18)*e.amplitude,o=Vt(t.frameHeight*.014,6,28)*e.amplitude,s=1.2*e.amplitude;r.dx+=_e(e,"x")*i*Math.sin(Ne*e.frequency*n+a+Math.PI/2),r.dy+=_e(e,"y")*o*Math.sin(Ne*e.frequency*n+a),r.dRotation+=_e(e,"rotation")*s*Math.sin(Math.PI*e.frequency*n+a+Math.PI)}function Gv(e,t,r){const n=t.frame/Math.max(1,t.fps),a=e.phaseFrames/Math.max(1,t.fps)*e.frequency*Ne,i=.035*e.amplitude,o=Math.min(.08,.04*e.amplitude),s=Math.sin(Ne*e.frequency*n+a);r.scaleWidth*=1+_e(e,"width")*i*s,r.scaleHeight*=1+_e(e,"height")*i*s,r.dOpacity+=_e(e,"opacity")*o*s}function Kv(e,t,r){const a=t.frame/Math.max(1,t.fps)*e.frequency,i=Vt(t.frameWidth*.004,2,10)*e.amplitude,o=Vt(t.frameHeight*.004,2,10)*e.amplitude,s=.55*e.amplitude,l=e.seed*97;r.dx+=_e(e,"x")*Zi(l+11,a)*i,r.dy+=_e(e,"y")*Zi(l+23,a)*o,r.dRotation+=_e(e,"rotation")*Zi(l+37,a)*s}function Vv(e,t,r){const n=t.frame/Math.max(1,t.fps),a=e.phaseFrames/Math.max(1,t.fps)*e.frequency*Ne;r.dRotation+=_e(e,"rotation")*4*e.amplitude*Math.sin(Ne*e.frequency*n+a)}function Wv(e,t,r){const n=t.frame/Math.max(1,t.fps);r.dRotation+=_e(e,"rotation")*360*e.frequency*e.amplitude*n}const fc=[1,1.618,2.414],jv=[.55,.3,.15],hc=[.6,.97],$v=[.65,.35];function Yv(e,t,r){const n=(t.frame+e.phaseFrames)/Math.max(1,t.fps),a=Vt(t.frameWidth*.012,6,32)*e.amplitude,i=Vt(t.frameHeight*.012,6,24)*e.amplitude,o=e.seed*97;let s=0,l=0;for(let d=0;d<fc.length;d+=1){const m=fc[d],p=jv[d];s+=p*Math.sin(Ne*e.frequency*m*n+Ne*Dn(o+d)),l+=p*Math.sin(Ne*e.frequency*m*n+Ne*Dn(o+16+d))}r.dx+=_e(e,"x")*a*s,r.dy+=_e(e,"y")*i*l;let c=0;for(let d=0;d<hc.length;d+=1)c+=$v[d]*Math.sin(Ne*e.frequency*hc[d]*n+Ne*Dn(o+32+d));const u=.025*e.amplitude*c;r.scaleWidth*=1+_e(e,"width")*u,r.scaleHeight*=1+_e(e,"height")*u}function Xv(e,t,r){if(!(!e.enabled||e.amplitude<=0))switch(e.type){case"float-drift":return qv(e,t,r);case"breath-pulse":return Gv(e,t,r);case"micro-shake":return Kv(e,t,r);case"sway":return Vv(e,t,r);case"spin":return Wv(e,t,r);case"wander":return Yv(e,t,r);default:return e.type}}function Qv(e,t){if(!e||e.length===0)return pc;const r={...pc};for(const n of e)Xv(n,t,r);return r}function Jv(e,t,r){if(!t||t.length===0)return e;const n=Qv(t,r);return n.dx===0&&n.dy===0&&n.dRotation===0&&n.dOpacity===0&&n.scaleWidth===1&&n.scaleHeight===1?e:{...e,x:e.x+n.dx,y:e.y+n.dy,rotation:e.rotation+n.dRotation,width:Math.max(1,e.width*n.scaleWidth),height:Math.max(1,e.height*n.scaleHeight),opacity:Vt(e.opacity+n.dOpacity,0,1)}}const gc={dx:0,dy:0,dRotation:0,dOpacity:0,scaleWidth:1,scaleHeight:1};function Zv(e){return e==="multiply"?1:0}function eb(e,t={}){return{...e,id:t.freshIds?$():e.id,tracks:e.tracks.map(r=>({...r,keyframes:r.keyframes.map(n=>({...n,id:t.freshIds?$():n.id,...n.easingConfig&&{easingConfig:{...n.easingConfig,...n.easingConfig.bezier&&{bezier:{...n.easingConfig.bezier}},...n.easingConfig.spring&&{spring:{...n.easingConfig.spring}}}}}))}))}}function tb(e,t){if(!e||e.length===0)return gc;const r={...gc};for(const n of e)if(n.enabled)for(const a of n.tracks){if(a.keyframes.length===0)continue;const i=en(a.keyframes,t,Zv(a.blend));switch(a.property){case"x":r.dx+=i;break;case"y":r.dy+=i;break;case"rotation":r.dRotation+=i;break;case"opacity":r.dOpacity+=i;break;case"width":r.scaleWidth*=i;break;case"height":r.scaleHeight*=i;break}}return r}function rb(e,t,r){return Math.max(t,Math.min(r,e))}function nb(e,t,r){if(!t||t.length===0)return e;const n=tb(t,r);return{...e,x:e.x+n.dx,y:e.y+n.dy,rotation:e.rotation+n.dRotation,width:Math.max(1,e.width*n.scaleWidth),height:Math.max(1,e.height*n.scaleHeight),opacity:rb(e.opacity+n.dOpacity,0,1)}}const Xm={"clean-title":{id:"clean-title",label:"Clean",previewKind:"clean",layout:"single",sample:{title:"Main",subtitle:"Title"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title"},color:"#ffffff",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.95,letterSpacing:"tight",textPadding:"sm",textShadow:{token:"md",color:"#111827"},stroke:void 0}},poster:{id:"poster",label:"Poster",previewKind:"poster",layout:"single",sample:{title:"Tonight"},style:{fontFamily:"Anton",fontWeight:"normal",fontStyle:"normal",underline:!1,size:{token:"display",multiplier:.98},color:"#fef3c7",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.88,letterSpacing:"tight",textPadding:"sm",textShadow:{token:"xl",color:"#7f1d1d"},stroke:{width:2,color:"#991b1b"}}},"outline-pill":{id:"outline-pill",label:"Outline",previewKind:"outline-pill",layout:"single",sample:{title:"Featured"},style:{fontFamily:"Inter",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"badge",multiplier:.96},color:"#e2e8f0",backgroundColor:"#0f172a",backgroundRadius:"pill",textAlign:"center",verticalAlign:"middle",lineHeight:1,letterSpacing:"badge",textPadding:"badge",textShadow:{token:"sm",color:"#020617"},stroke:{width:1,color:"#38bdf8"}}},"lower-third":{id:"lower-third",label:"Lower Third",previewKind:"lower-third",layout:"two",sample:{title:"Name",subtitle:"Role or subtitle"},style:{fontFamily:"Inter",fontWeight:"semibold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.587},color:"#f9fafb",backgroundColor:"#111827",backgroundRadius:"md",textAlign:"left",verticalAlign:"middle",lineHeight:1.05,letterSpacing:"normal",textPadding:"lg",textShadow:{token:"sm",color:"#030712"},stroke:void 0}},"speaker-card":{id:"speaker-card",label:"Speaker",previewKind:"speaker",layout:"two",sample:{title:"Alex Morgan",subtitle:"Product Designer"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.61},color:"#f8fafc",backgroundColor:"#1e293b",backgroundRadius:"lg",textAlign:"left",verticalAlign:"middle",lineHeight:1,letterSpacing:"tight",textPadding:"xl",textShadow:{token:"md",color:"#020617"},stroke:void 0}},cinematic:{id:"cinematic",label:"Cinematic",previewKind:"cinematic",layout:"single",sample:{title:"CINEMA",subtitle:"PRESENTS"},style:{fontFamily:"Bebas Neue",fontWeight:"normal",fontStyle:"normal",underline:!1,size:{token:"display"},color:"#f8e6b8",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.92,letterSpacing:"cinematic",textPadding:"sm",textShadow:{token:"lg",color:"#111827"},stroke:{width:1,color:"#2b2112"}}},quote:{id:"quote",label:"Quote",previewKind:"quote",layout:"two",sample:{title:"Quote",subtitle:"Attribution"},style:{fontFamily:"Playfair Display",fontWeight:"semibold",fontStyle:"italic",underline:!1,size:{token:"title",multiplier:.83},color:"#f8fafc",backgroundColor:"#1f2937",backgroundRadius:"lg",textAlign:"center",verticalAlign:"middle",lineHeight:1.08,letterSpacing:"normal",textPadding:"quote",textShadow:{token:"lg",color:"#020617"},stroke:void 0}},neon:{id:"neon",label:"Neon",previewKind:"neon",layout:"single",sample:{title:"NEON",subtitle:"Glow"},style:{fontFamily:"Orbitron",fontWeight:"semibold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.935},color:"#67e8f9",backgroundColor:"#082f49",backgroundRadius:"sm",textAlign:"center",verticalAlign:"middle",lineHeight:1,letterSpacing:"wide",textPadding:"md",textShadow:{token:"glow",color:"#22d3ee"},stroke:{width:1,color:"#22d3ee"}}},"headline-stack":{id:"headline-stack",label:"Headline",previewKind:"stacked",layout:"three",sample:{eyebrow:"TOP STORY",title:"Headline",subtitle:"Subhead"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.913},color:"#f8fafc",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.96,letterSpacing:"tight",textPadding:"sm",textShadow:{token:"lg",color:"#020617"},stroke:void 0}},"breaking-update":{id:"breaking-update",label:"Breaking",previewKind:"breaking",layout:"three",sample:{eyebrow:"BREAKING",title:"Major Update",subtitle:"Developing now"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.92},color:"#f8fafc",backgroundColor:"#111827",backgroundRadius:"md",textAlign:"left",verticalAlign:"middle",lineHeight:.94,letterSpacing:"tight",textPadding:"lg",textShadow:{token:"lg",color:"#020617"},stroke:void 0}},"event-card":{id:"event-card",label:"Event",previewKind:"event",layout:"three",sample:{eyebrow:"LIVE",title:"Summer Fest",subtitle:"Friday 8 PM"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.848},color:"#f8fafc",backgroundColor:"#0f172a",backgroundRadius:"lg",textAlign:"center",verticalAlign:"middle",lineHeight:.98,letterSpacing:-.5,textPadding:"xl",textShadow:{token:"xl",color:"#020617"},stroke:void 0}},"launch-stack":{id:"launch-stack",label:"Launch",previewKind:"launch",layout:"three",sample:{eyebrow:"NOW LIVE",title:"New Collection",subtitle:"Shop the drop"},style:{fontFamily:"Space Grotesk",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.88},color:"#f8fafc",backgroundColor:"#0f172a",backgroundRadius:"lg",textAlign:"center",verticalAlign:"middle",lineHeight:.96,letterSpacing:"tight",textPadding:"xl",textShadow:{token:"glow",color:"#60a5fa"},stroke:{width:1,color:"#1d4ed8"}}},badge:{id:"badge",label:"Badge",previewKind:"badge",layout:"single",sample:{title:"NEW DROP",subtitle:"Tag"},style:{fontFamily:"Inter",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"badge"},color:"#f8fafc",backgroundColor:"#111827",backgroundRadius:"pill",textAlign:"center",verticalAlign:"middle",lineHeight:1,letterSpacing:"badge",textPadding:"badge",textShadow:{token:"sm",color:"#020617"},stroke:{width:1,color:"#334155"}}}};Object.values(Xm).map(({id:e,label:t,previewKind:r,layout:n,sample:a})=>({id:e,label:t,previewKind:r,layout:n,sample:a}));function Qm(e,t,r){return Math.min(r,Math.max(t,e))}function eo(e,t,r,n){return Qm(Math.round(e.height*t),r,n)}function gt(e,t,r,n){const a=Math.min(e.width,e.height);return Qm(Math.round(a*t),r,n)}function Yt(e){return e.map(t=>t.text).join(`
`)}function Jm(e){const t=Xm[e];if(!t)throw new Error(`Unknown text style preset: ${e}`);return t}function ab(e,t){const r=e.sizes[t.token];return Math.round(r*(t.multiplier??1))}function ib(e,t){return typeof t=="number"?t:e.tracking[t]}function ob(e){return{sizes:{badge:eo(e,.048,32,68),title:eo(e,.085,56,132),display:eo(e,.11,72,164)},spacing:{sm:gt(e,.0148,16,16),badge:gt(e,.0167,18,18),md:gt(e,.0185,20,20),lg:gt(e,.0222,24,24),xl:gt(e,.0259,28,28),quote:gt(e,.0278,30,30)},radius:{none:0,sm:gt(e,.0167,18,18),md:gt(e,.0185,20,20),lg:gt(e,.0259,28,28),pill:999},tracking:{tight:-1,normal:0,wide:1,badge:2,cinematic:4},shadows:{sm:{offsetX:0,offsetY:4,blur:14},md:{offsetX:0,offsetY:6,blur:18},lg:{offsetX:0,offsetY:8,blur:24},xl:{offsetX:0,offsetY:10,blur:24},glow:{offsetX:0,offsetY:0,blur:22}}}}function sb(e,t,r=1){const n=Jm(e),a=ob(t),i=n.style.textShadow?a.shadows[n.style.textShadow.token]:void 0;return{fontFamily:n.style.fontFamily,fontWeight:n.style.fontWeight,fontStyle:n.style.fontStyle,underline:n.style.underline,fontSize:Math.round(ab(a,n.style.size)*r),color:n.style.color,backgroundColor:n.style.backgroundColor,backgroundRadius:n.style.backgroundRadius==="pill"?a.radius[n.style.backgroundRadius]:Math.round(a.radius[n.style.backgroundRadius]*r),textAlign:n.style.textAlign,verticalAlign:n.style.verticalAlign,lineHeight:n.style.lineHeight,letterSpacing:ib(a,n.style.letterSpacing)*r,textPadding:Math.round(a.spacing[n.style.textPadding]*r),textShadow:i?{offsetX:i.offsetX*r,offsetY:i.offsetY*r,blur:i.blur*r,color:n.style.textShadow?.color??"#000000"}:void 0,stroke:n.style.stroke?{...n.style.stroke,width:n.style.stroke.width*r}:void 0,textStylePresetId:e,textStyleScale:r}}function lb(e,t,r=1){const n=Jm(e),a=sb(e,t,r),i=a.fontSize??60;switch(e){case"speaker-card":{const o=[{text:n.sample.title,fontWeight:"bold"},{text:n.sample.subtitle??"Product Designer",fontSize:Math.max(20,Math.round(i*.44)),fontWeight:"medium",color:"#cbd5e1"}];return{...a,label:n.label,text:Yt(o),textSpans:o}}case"lower-third":{const o=[{text:n.sample.title,fontWeight:"bold"},{text:n.sample.subtitle??"Role or subtitle",fontSize:Math.max(22,Math.round(i*.54)),fontWeight:"medium",color:"#cbd5e1"}];return{...a,label:n.label,text:Yt(o),textSpans:o}}case"quote":{const o=[{text:n.sample.title,fontStyle:"italic"},{text:n.sample.subtitle??"Attribution",fontSize:Math.max(18,Math.round(i*.4)),fontStyle:"normal",fontWeight:"medium",color:"#cbd5e1",letterSpacing:1}];return{...a,label:n.label,text:Yt(o),textSpans:o}}case"breaking-update":{const o=[{text:n.sample.eyebrow??"BREAKING",fontSize:Math.max(16,Math.round(i*.28)),fontWeight:"bold",color:"#fca5a5",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Developing now",fontSize:Math.max(20,Math.round(i*.38)),fontWeight:"semibold",color:"#fde68a"}];return{...a,label:n.label,text:Yt(o),textSpans:o}}case"headline-stack":{const o=[{text:n.sample.eyebrow??"TOP STORY",fontSize:Math.max(16,Math.round(i*.3)),fontWeight:"semibold",color:"#fbbf24",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Subhead",fontSize:Math.max(20,Math.round(i*.42)),fontWeight:"medium",color:"#cbd5e1"}];return{...a,label:n.label,text:Yt(o),textSpans:o}}case"launch-stack":{const o=[{text:n.sample.eyebrow??"NOW LIVE",fontSize:Math.max(16,Math.round(i*.26)),fontWeight:"bold",color:"#67e8f9",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Shop the drop",fontSize:Math.max(20,Math.round(i*.4)),fontWeight:"medium",color:"#bfdbfe"}];return{...a,label:n.label,text:Yt(o),textSpans:o}}case"event-card":{const o=[{text:n.sample.eyebrow??"LIVE",fontSize:Math.max(18,Math.round(i*.28)),fontWeight:"bold",color:"#fca5a5",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Friday 8 PM",fontSize:Math.max(22,Math.round(i*.38)),fontWeight:"semibold",color:"#bfdbfe",letterSpacing:1}];return{...a,label:n.label,text:Yt(o),textSpans:o}}case"badge":return{...a,label:n.label,text:n.sample.title,textSpans:[{text:n.sample.title,letterSpacing:2}]};default:return{...a,label:n.label,text:n.sample.title,textSpans:void 0}}}function cb(e){if(!e)return{};const t={...e};return delete t.text,t}function ub(e,t,r,n=1){const a=lb(t,r,n),i=Array.isArray(e.textSpans)&&e.textSpans.length>0?e.textSpans:void 0,o=Array.isArray(a.textSpans)&&a.textSpans.length>0?a.textSpans:void 0,s=i?i.map((c,u)=>{const d=o?.[Math.min(u,Math.max(0,(o?.length??1)-1))];return{...c,...cb(d),text:c.text}}):void 0,l=s?s.map(c=>c.text).join(`
`):e.text;return{...a,text:l,textSpans:s,label:yy(l)}}const Zm=["textStyleScale","fontSize","lineHeight","textPadding","backgroundRadius","textShadowOffsetX","textShadowOffsetY","textShadowBlur","strokeWidth"];new Set(Zm);const db="#000000",mb="#111827",pb={textStyleScale:e=>e.textStyleScale??1,fontSize:e=>e.fontSize??60,lineHeight:e=>e.lineHeight??1.2,textPadding:e=>e.textPadding??16,backgroundRadius:e=>e.backgroundRadius??0,textShadowOffsetX:e=>e.textShadow?.offsetX??0,textShadowOffsetY:e=>e.textShadow?.offsetY??0,textShadowBlur:e=>e.textShadow?.blur??0,strokeWidth:e=>e.stroke?.width??0};function fb(e,t){return pb[t](e)}function hb(e,t,r,n){return en(ia(t,n),r,fb(e,n))}function gb(e,t){return ia(e,t).length>0}function yb(e,t,r,n,a,i){const o=gb(t,n);if(!o)return{isAnimated:o,value:a};const s=hb(e,t,r,n);return{isAnimated:o,value:i===void 0?s:Math.max(i,s)}}function vb(e){if(!(e.offsetX===0&&e.offsetY===0&&e.blur===0))return e}function bb(e){if(!(e.width<=0))return e}function Sb(e,t,r){return!e.textStylePresetId||!r.isAnimated?e:{...e,...ub(e,e.textStylePresetId,t,r.value??1)}}function Ib(e,t,r){return e.textStylePresetId&&r.isAnimated?r.value:t.textStyleScale}function xb(e){return e.some(t=>t.isAnimated)}function to(e,t){return e.value===void 0?t:e.value}function wb(e,t){return t.textShadow?.color?t.textShadow.color:e.textShadow?.color?e.textShadow.color:db}function kb(e,t,r,n,a){if(!(!xb([r,n,a])&&!t.textShadow))return vb({offsetX:to(r,0),offsetY:to(n,0),blur:to(a,0),color:wb(e,t)})}function Pb(e,t,r){if(!(!r.isAnimated&&!t.stroke))return bb({width:r.value??0,color:t.stroke?.color??e.stroke?.color??mb})}function Mb(e,t,r,n){const a=(h,g,v)=>yb(e,t,r,h,g,v),i=a("textStyleScale",e.textStyleScale),o=Sb(e,n,i),s=a("fontSize",o.fontSize,1),l=a("lineHeight",o.lineHeight,.1),c=a("textPadding",o.textPadding,0),u=a("backgroundRadius",o.backgroundRadius,0),d=a("textShadowOffsetX",o.textShadow?.offsetX),m=a("textShadowOffsetY",o.textShadow?.offsetY),p=a("textShadowBlur",o.textShadow?.blur,0),f=a("strokeWidth",o.stroke?.width,0);return{...o,fontSize:s.value,lineHeight:l.value,textPadding:c.value,backgroundRadius:u.value,textStyleScale:Ib(e,o,i),textShadow:kb(e,o,d,m,p),stroke:Pb(e,o,f)}}const Cb=["positionX","positionY","inX","inY","outX","outY"];function Tb(e,t){switch(t){case"positionX":return e.position[0];case"positionY":return e.position[1];case"inX":return e.inHandle[0];case"inY":return e.inHandle[1];case"outX":return e.outHandle[0];case"outY":return e.outHandle[1]}}function Eb(e,t,r){switch(t){case"positionX":e.position[0]=r;return;case"positionY":e.position[1]=r;return;case"inX":e.inHandle[0]=r;return;case"inY":e.inHandle[1]=r;return;case"outX":e.outHandle[0]=r;return;case"outY":e.outHandle[1]=r}}function Fb(e){return e?e.flatMap((t,r)=>Cb.map(n=>qy(r,n))):[]}function _T(e,t){const r=Ks(t),n=r?e?.[r.vertexIndex]:void 0;return r&&n?Tb(n,r.component):0}function OT(e,t,r){const n=Ks(t),a=n?e[n.vertexIndex]:void 0;return!n||!a?!1:(Eb(a,n.component,r),!0)}function DT(e){return e.map(t=>({...t,position:[...t.position],inHandle:[...t.inHandle],outHandle:[...t.outHandle]}))}function Ab(e,t,r,n){const a=Math.max(1,t-1),i=Math.max(0,Math.min(1,e/a));switch(r){case"ease-in":return Hm(i);case"ease-out":return Nm(i);case"ease-in-out":return qm(i);case"cubic-bezier":return n?Bi(i,n):i;default:return i}}function _b({transitionWindows:e,frame:t}){const r=[],n=new Set;for(const a of e){if(t<a.startFrame||t>=a.endFrame)continue;const i=t-a.startFrame;r.push({transition:a.transition,leftClip:a.leftClip,rightClip:a.rightClip,progress:Ab(i,a.durationInFrames,a.transition.timing,a.transition.bezierPoints),transitionStart:a.startFrame,transitionEnd:a.endFrame,durationInFrames:a.durationInFrames,leftPortion:a.leftPortion,rightPortion:a.rightPortion,cutPoint:a.cutPoint}),n.add(a.transition.leftClipId),n.add(a.transition.rightClipId)}return{activeTransitions:r,transitionClipIds:n}}function Ob(e){return e?(e.frames?.length??0)>0||(e.ranges?.length??0)>0:!1}function Db(e,t){return e>=t.startFrame&&e<t.endFrame}function Rb(e,t){return t.some(r=>Db(e,r))}const xn=1e-6;function hi(e,t){return{a:e.a*t.a+e.c*t.b,b:e.b*t.a+e.d*t.b,c:e.a*t.c+e.c*t.d,d:e.b*t.c+e.d*t.d,e:e.a*t.e+e.c*t.f+e.e,f:e.b*t.e+e.d*t.f+e.f}}function ep(e){const t=e.a*e.d-e.b*e.c;return Math.abs(t)<xn?null:{a:e.d/t,b:-e.b/t,c:-e.c/t,d:e.a/t,e:(e.c*e.f-e.d*e.e)/t,f:(e.b*e.e-e.a*e.f)/t}}function _r(e){const t=e.rotation*Math.PI/180,r=Math.cos(t),n=Math.sin(t);return{a:r*e.width,b:n*e.width,c:-n*e.height,d:r*e.height,e:e.x,f:e.y}}function Lb(e,t){const r=Math.max(xn,Math.hypot(t.a,t.b)),n=t.a*t.d-t.b*t.c,a=Math.max(xn,Math.abs(n)/r),i=r/Math.max(xn,e.width),o=a/Math.max(xn,e.height);return{...e,x:t.e,y:t.f,width:r,height:a,anchorX:e.anchorX*i,anchorY:e.anchorY*o,rotation:Math.atan2(t.b,t.a)*180/Math.PI,cornerRadius:e.cornerRadius*Math.sqrt(i*o)}}function Bb(e,t){const r=ep(_r(t.childLocalReference));return r?hi(_r(t.childWorldReference),hi(r,_r(e))):_r(e)}function ro(e){return{x:e.x,y:e.y,width:e.width,height:e.height,rotation:e.rotation}}function zb(e){return{...e.parentItemId&&{parentItemId:e.parentItemId},...e.parentWorld&&{parentReference:ro(e.parentWorld)},childLocalReference:ro(e.childLocal),childWorldReference:ro(e.childWorld)}}function Ub(e,t,r){let n=Bb(e,t);if(r&&t.parentReference){const a=ep(_r(t.parentReference));if(a){const i=hi(_r(r),a);n=hi(i,n)}}return Lb(e,n)}function tp(e,t,r={cache:new Map,active:new Set}){const n=r.cache.get(e.id);if(n)return n;const a=t.resolveLocal(e),i=e.transformParent;if(!i||r.active.has(e.id))return a;r.active.add(e.id);const o=i.parentItemId?t.getItem(i.parentItemId):void 0,s=o?tp(o,t,r):void 0,l=Ub(a,i,s);return r.active.delete(e.id),r.cache.set(e.id,l),l}function Hb(e,t,r,n=()=>{}){return rp(e,t,r,n)}function rp(e,t,r,n){const a=new Set,i=[t];for(;i.length>0;){const o=i.pop();if(o===e)return!0;if(a.has(o))continue;a.add(o);const s=r(o)?.transformParent?.parentItemId;s&&i.push(s);for(const l of mt(n(o)))l.enabled&&i.push(l.sourceItemId)}return!1}const Nb=new Set(["position","scale","x","y","width","height","rotation"]);function np(e){return Nb.has(e)}function qb(e,t,r){const n=new Set;let a=r(e)?.transformParent?.parentItemId;for(;a;){if(a===t)return!0;if(n.has(a))return!1;n.add(a),a=r(a)?.transformParent?.parentItemId}return!1}function Gb(e,t,r,n){const a=new Set,i=new Set;let o=t;for(;o&&!i.has(o);)i.add(o),a.add(o),o=r(o)?.transformParent?.parentItemId;return mt(n(e)).some(s=>s.enabled&&np(s.targetProperty)&&a.has(s.sourceItemId))}function Kb(e,t){if(!t)return e;const r=Object.prototype.hasOwnProperty.call(t,"anchorX"),n=Object.prototype.hasOwnProperty.call(t,"anchorY");return{...e,...t,anchorX:r&&t.anchorX===void 0?(t.width??e.width)/2:t.anchorX??e.anchorX,anchorY:n&&t.anchorY===void 0?(t.height??e.height)/2:t.anchorY??e.anchorY,opacity:t.opacity??e.opacity,cornerRadius:t.cornerRadius??e.cornerRadius}}function ap(e,{canvas:t,relativeFrame:r,keyframes:n,previewTransform:a,expressionContext:i}){const o=aa(e,t,Zr(e)),s=n&&Uv(n)?Ys(o,n,r,i):o,l=nb(s,e.motionLayers,r),c=Jv(l,e.motionModifiers,{frame:r,fps:t.fps,frameWidth:t.width,frameHeight:t.height}),u=Kb(c,a);return e.type==="text"&&!Ky(e.cornerPin)?_y(Mb(e,n,r,t),u):u}function Xs(e,{canvas:t,frame:r,keyframes:n,previewTransform:a,getItem:i,getKeyframes:o,getPreviewTransform:s}){const l=c=>ap(c,{canvas:t,relativeFrame:r-c.from,keyframes:c.id===e.id?n:o?.(c.id),previewTransform:c.id===e.id?a??s?.(c.id):s?.(c.id),expressionContext:i&&o?{globalFrame:r,canvas:t,getItem:i,getKeyframes:o,getPreviewTransform:s}:void 0});return i?tp(e,{getItem:i,resolveLocal:l}):l(e)}function Vb(e,{canvas:t,frame:r,getKeyframes:n,getItem:a,getPreviewTransform:i,getPreviewPathVertices:o}){return e.length===0?[]:e.map(s=>"mask"in s?s:{mask:s,trackOrder:0}).filter(({mask:s})=>{const l=s.from,c=s.from+s.durationInFrames;return r>=l&&r<c}).map(({mask:s,trackOrder:l})=>{const c=_m(s,o);return{shape:c,trackOrder:l,transform:Xs(c,{canvas:t,frame:r,keyframes:n?.(s.id),previewTransform:i?.(s.id),getItem:a,getKeyframes:n,getPreviewTransform:i})}})}function Wb({renderPlan:e,frame:t,canvas:r,getKeyframes:n,getItem:a,getPreviewTransform:i,getPreviewPathVertices:o}){return{frame:t,activeShapeMasks:Vb(e.visibleShapeMasks,{canvas:r,frame:t,getKeyframes:n,getItem:a,getPreviewTransform:i,getPreviewPathVertices:o}),transitionFrameState:_b({transitionWindows:e.transitionWindows,frame:t})}}function RT(){let e=null,t=-1,r,n=null,a=-1,i=-1,o=-1,s,l,c,u;return{resolve(d,m){const p=a===d.canvas.width&&i===d.canvas.height&&o===d.canvas.fps,f=s===d.getKeyframes&&l===d.getItem&&c===d.getPreviewTransform&&u===d.getPreviewPathVertices;return e&&t===d.frame&&r===m&&n===d.renderPlan&&p&&f||(e=Wb(d),t=d.frame,r=m,n=d.renderPlan,a=d.canvas.width,i=d.canvas.height,o=d.canvas.fps,s=d.getKeyframes,l=d.getItem,c=d.getPreviewTransform,u=d.getPreviewPathVertices),e},invalidate(d){if(e&&d&&Ob(d)){const m=d.frames?.includes(t)??!1,p=d.ranges?Rb(t,d.ranges):!1;if(!m&&!p)return}e=null,t=-1,r=void 0,n=null,a=-1,i=-1,o=-1,s=void 0,l=void 0,c=void 0,u=void 0}}}const jb={conflict:new Set},Kn=Ie()(e=>({conflicts:{},reportConflict:t=>{e(r=>({conflicts:{...r.conflicts,[t.projectId]:t}}));for(const r of[...jb.conflict])r(t)},clearConflict:t=>e(r=>{if(!(t in r.conflicts))return r;const n={...r.conflicts};return delete n[t],{conflicts:n}})}));function $b(e){return Kn.getState().conflicts[e]}const Et=new Map,Sa=new Map;async function Qs(e,t){const n=(Sa.get(e)??Promise.resolve()).catch(()=>{}).then(t),a=n.catch(()=>{});Sa.set(e,a);try{return await n}finally{Sa.get(e)===a&&Sa.delete(e)}}const Js=e=>`project:${e}`;function Yb(e){return e instanceof Jn&&e.code==="not_found"}function Xb(e){const t=$b(e);if(t)throw new mh("Dự án đã được thay đổi ở nơi khác; hãy tải lại hoặc giữ bản của bạn trước khi lưu tiếp.",t.serverRevision)}function ip(e){return Qs(Js(e),async()=>{try{const{data:t,revision:r}=await nt().loadProject(e);return Et.set(e,{project:t,revision:r}),Kn.getState().clearConflict(e),t}catch(t){if(!Yb(t))throw t;Et.delete(e),Kn.getState().clearConflict(e);return}})}function Qb(e){return Et.get(e)?.project}async function Jb(e){return Et.get(e)?.project??ip(e)}function op(e,t){return Qs(Js(e),async()=>{Xb(e);const r=Et.get(e);if(!r)throw new Error(`Project not loaded: ${e}`);const n={...r.project,...t,id:e,updatedAt:Date.now()};let a;try{a=await nt().saveProject(n,r.revision)}catch(i){throw dh(i)&&Kn.getState().reportConflict({projectId:e,baseRevision:r.revision,serverRevision:i.serverRevision,timestamp:Date.now()}),i}return Et.set(e,{project:n,revision:a}),n})}async function Zb(e){const{data:t,revision:r}=await nt().createProject(e);return Et.set(t.id,{project:t,revision:r}),t}async function eS(e,t){const{data:r,revision:n}=await nt().duplicateProject(e,t);return Et.set(r.id,{project:r,revision:n}),r}function sp(e){return Qs(Js(e),async()=>{await nt().deleteProject(e),Et.delete(e),Kn.getState().clearConflict(e)})}const fr=Oe("WorkspaceFS:Projects"),tS=6;async function rS(){const e=nt();try{const t=await e.listProjects(),r=[];let n=0;const a=async()=>{for(;n<t.length;){const i=t[n++];try{r.push((await e.loadProject(i.id)).data)}catch(o){if(!(o instanceof Jn&&o.code==="not_found"))throw o;fr.warn(`getAllProjects: project ${i.id} disappeared`,o)}}};return await Promise.all(Array.from({length:Math.min(tS,t.length)},a)),r.sort((i,o)=>o.updatedAt-i.updatedAt)}catch(t){throw fr.error("getAllProjects failed",t),new Error("Failed to load projects from server",{cause:t})}}async function nS(e){try{return await ip(e)}catch(t){throw fr.error(`getProject(${e}) failed`,t),t}}async function aS(e){const t=await Zb({name:e.name}),r={...t.metadata,width:e.width,height:e.height,fps:e.fps,...e.backgroundColor!==void 0&&{backgroundColor:e.backgroundColor}},n=e.description??"";if(n===t.description&&r.width===t.metadata.width&&r.height===t.metadata.height&&r.fps===t.metadata.fps&&r.backgroundColor===t.metadata.backgroundColor)return t;try{return await op(t.id,{description:n,metadata:r}),Qb(t.id)??t}catch(i){throw await sp(t.id).catch(o=>{fr.warn(`createProject: could not remove ${t.id} after failed setup`,o)}),i}}async function iS(e,t){try{return await op(e,t)}catch(r){throw fr.error(`updateProject(${e}) failed`,r),r}}async function oS(e,t){try{return await eS(e,t)}catch(r){throw fr.error(`duplicateProject(${e}) failed`,r),r}}async function sS(e){try{await sp(e)}catch(t){throw fr.error(`deleteProject(${e}) failed`,t),new Error(`Failed to delete project: ${e}`,{cause:t})}}const Ia=new Map;async function lp(e,t){const a=(Ia.get(e)??Promise.resolve()).catch(()=>{}).then(t),i=a.catch(()=>{});Ia.set(e,i);try{return await a}finally{Ia.get(e)===i&&Ia.delete(e)}}const yc=new Map,is=new Set;function cp(e,t){const r=yc.get(e);if(yc.set(e,t),r===void 0||r===t)return!1;for(const n of[...is])n({mediaId:e,previousHash:r,contentHash:t});return!0}function lS(e){return is.add(e),()=>{is.delete(e)}}const no=new Map;function Zs(e){const t=no.get(e);if(t)return t;const r=kd(e).then(n=>(cp(e,n.contentHash),n)).finally(()=>{no.delete(e)});return no.set(e,r),r}async function LT(e){try{return await Zs(e)}catch(t){if(t instanceof Jn&&t.code==="not_found")return null;throw t}}const cS=["duration","width","height","fps","codec","bitrate","audioCodec","audioCodecSupported","videoCodecSupported","keyframeTimestamps","gopInterval","thumbnailId","aiCaptions"],up="derived.json",uS=Object.fromEntries(cS.map(e=>[e,!0]));function dp(e){return uS[e]===!0}async function mp(e,t){return cp(e,t),{workspaceId:await nt().workspaceId(),mediaId:e,contentHash:t}}function dS(e){if(typeof e!="object"||e===null)return{};const t={};for(const[r,n]of Object.entries(e))dp(r)&&n!==void 0&&(t[r]=n);return t}async function mS(e,t){const r=await Qo(["media",e,up],await mp(e,t));if(!r)return{};try{return dS(JSON.parse(await r.text()))}catch{return{}}}async function BT(e,t,r){return lp(`media-derived:${e}`,async()=>{const n={...await mS(e,t)};for(const[a,i]of Object.entries(r))dp(a)&&(i===void 0?delete n[a]:n[a]=i);return await S0(["media",e,up],JSON.stringify(n),await mp(e,t)),n})}async function zT(e){await Jo(["media",e],!0)}function hr(){return b0}const pS=Oe("CacheFS");class pp extends Error{constructor(t,r){super(`Corrupt JSON at ${t}`),this.path=t,this.name="WorkspaceFileCorruptError",this.cause=r}path}const fS=new Set(["NotFoundError","NotReadableError","TypeMismatchError"]),hS=new Set(["QuotaExceededError","NotFoundError","NotReadableError","NoModificationAllowedError","InvalidStateError"]);function ft(e){return e instanceof Jn?e.code==="not_found":e instanceof DOMException&&fS.has(e.name)}function gS(e){return e instanceof Jn?e.code==="not_found":e instanceof DOMException&&hS.has(e.name)}async function el(e,t,r,n){const a=t[0]==="media"?t[1]:void 0,i=n??(a?await Zs(a):void 0);return ta(t,r,i)}async function zi(e,t,r,n){if(t.length===0)throw new Error("fs-primitives: empty path segments");return{parent:await el(e,t.slice(0,-1),r,n),fileName:t[t.length-1]}}async function yS(e,t,r){const n=await fp(e,t,r);if(!n)return null;const a=await n.text();if(a.length===0)return null;try{return JSON.parse(a)}catch(i){throw new pp(t.join("/"),i)}}async function fp(e,t,r){try{const{parent:n,fileName:a}=await zi(e,t,!1,r);return await(await n.getFileHandle(a,{create:!1})).getFile()}catch(n){if(ft(n))return null;throw n}}async function UT(e,t,r){const n=await fp(e,t,r);return n?n.arrayBuffer():null}async function hp(e,t,r,n){return lp(`cache-write:${t.join("/")}`,async()=>{let a=null;try{const{parent:i,fileName:o}=await zi(e,t,!0,n),s=await i.getFileHandle(o,{create:!1}).then(()=>!0).catch(()=>!1),l=await i.getFileHandle(o,{create:!0});s||(a={parent:i,fileName:o});const c=await l.createWritable();try{await c.write(r),await c.close()}catch(u){throw await c.abort().catch(()=>{}),u}return!0}catch(i){if(!gS(i))throw i;return pS.warn(`Cache write dropped for ${t.join("/")}`,i),a&&await a.parent.removeEntry(a.fileName).catch(()=>{}),!1}})}async function vS(e,t,r,n){const a=JSON.stringify(r,null,2);return await hp(e,t,a,n)?a.length:0}async function bS(e,t,r,n){await hp(e,t,r,n)}async function gp(e,t,r={}){if(t.length===0)throw new Error("fs-primitives: refusing to remove empty path");try{const{parent:n,fileName:a}=await zi(e,t,!1);await n.removeEntry(a,{recursive:r.recursive??!1})}catch(n){if(ft(n))return;throw n}}async function HT(e,t,r){try{const n=await el(e,t,!1,r),a=[];for await(const i of n.values())a.push({name:i.name,kind:i.kind});return a}catch(n){if(ft(n))return[];throw n}}async function NT(e,t,r){try{const n=await el(e,t,!1),a=[];for await(const o of n.values())o.kind==="file"&&(r&&!r({name:o.name,kind:o.kind})||a.push({name:o.name,handle:o}));return(await Promise.all(a.map(async({name:o,handle:s})=>{try{return{name:o,blob:await s.getFile()}}catch(l){if(ft(l))return null;throw l}}))).filter(o=>o!==null)}catch(n){if(ft(n))return[];throw n}}async function yp(e,t){try{const{parent:r,fileName:n}=await zi(e,t,!1);try{return await r.getFileHandle(n,{create:!1}),!0}catch(a){if(!ft(a))throw a}try{return await r.getDirectoryHandle(n,{create:!1}),!0}catch(a){if(!ft(a))throw a}return!1}catch(r){if(ft(r))return!1;throw r}}async function qT(e,t){if(t.length===0)throw new Error("fs-primitives: refusing to remove empty path");const r=t.slice(0,-1),n=t[t.length-1];try{const a=await ta(["media"],!1),i=[];for await(const o of a.values())o.kind==="directory"&&i.push(o);for(const{name:o}of i){const s=await a.getDirectoryHandle(o),l=[];for await(const c of s.values())c.kind==="directory"&&l.push(c.name);for(const c of l)try{let u=await s.getDirectoryHandle(c);for(const d of r)u=await u.getDirectoryHandle(d);await u.removeEntry(n,{recursive:!0})}catch(u){if(!ft(u))throw u}}}catch(a){if(!ft(a))throw a}}const SS="media",IS="content",xS="thumbnail.jpg",wS="cache",kS="waveform",PS="filmstrip",MS="gif-frames",CS="decoded-audio",vp="ai",TS="preview-audio.wav",ES="multi-res.bin",bp="captions-thumbs",Sp="meta.json",FS="refs.json";function Ip(e){return[SS,e]}function GT(e){return[...Ip(e),xS]}function tn(e){return[...Ip(e),wS]}function xp(e){return[...tn(e),kS]}function KT(e,t){return[...xp(e),`bin-${t}.bin`]}function VT(e){return[...xp(e),ES]}function wp(e){return[...tn(e),PS]}function WT(e,t,r){return[...wp(e),`${t}.${r}`]}function jT(e){return[...wp(e),Sp]}function gi(e){return[...tn(e),TS]}function AS(e){return[...tn(e),MS]}function $T(e,t){return[...AS(e),`frame-${t}.png`]}function _S(e){return[...tn(e),CS]}function YT(e,t,r){return[..._S(e),`${t}-${r}.bin`]}function Ui(e){return[...tn(e),vp]}function Hi(e,t){return[...Ui(e),`${t}.json`]}function XT(e){return[...Ui(e),bp]}function QT(e){return[...Ui(e),"captions-embeddings.bin"]}function JT(e){return[...Ui(e),"captions-image-embeddings.bin"]}function vc(e){return[...DS(e),vp]}function OS(e){return e===void 0||!Number.isFinite(e)||e<=0?null:`si-${Math.round(e*100)}`}function oa(e,t){const r=OS(t);return r?[...vc(e),r]:vc(e)}function ZT(e,t){return[...oa(e,t),FS]}function eE(e,t){return[...oa(e,t),"captions.json"]}function tE(e,t){return[...oa(e,t),"captions-embeddings.bin"]}function rE(e,t){return[...oa(e,t),"captions-image-embeddings.bin"]}function nE(e,t){return[...oa(e,t),bp]}function aE(e){return[...e,Sp]}function DS(e){const t=e.slice(0,2);return[IS,t,e]}const RS=1,yi=Oe("WorkspaceFS:AiOutputs");async function iE(e,t){return tl(Hi(e,t),t,`readAiOutput(${e}, ${t})`)}async function tl(e,t,r=`readAiOutputAt(${e.join("/")}, ${t})`){const n=hr();try{return await yS(n,e)??void 0}catch(a){if(a instanceof pp){yi.warn(`${r}: corrupt cache entry treated as a miss`,a);return}throw yi.error(`${r} failed`,a),new Error(`Failed to load AI output ${t}`)}}async function oE(e){return LS(Hi(e.mediaId,e.kind),e)}async function LS(e,t){const r=hr(),n=Date.now(),a=await tl(e,t.kind),i={schemaVersion:RS,kind:t.kind,mediaId:t.mediaId,service:t.service,model:t.model,params:t.params??{},createdAt:a?.createdAt??n,updatedAt:n,data:t.data};try{return await vS(r,e,i),i}catch(o){throw yi.error(`writeAiOutputAt(${e.join("/")}, ${t.kind}) failed`,o),new Error(`Failed to save AI output ${t.kind}`)}}async function BS(e,t){return zS(Hi(e,t),`deleteAiOutput(${e}, ${t})`)}async function zS(e,t=`deleteAiOutputAt(${e.join("/")})`){const r=hr();try{await gp(r,e)}catch(n){throw yi.error(`${t} failed`,n),new Error("Failed to delete AI output")}}async function US(e,t){if(e.length===0)return new Set;const r=new Set,n=await Promise.all(e.map(async a=>await tl(Hi(a,t),t)?a:null));for(const a of n)a&&r.add(a);return r}const kp=Oe("WorkspaceFS:Transcripts");async function HS(e){if(e.length===0)return new Set;try{return await US(e,"transcript")}catch(t){throw kp.error("getTranscriptMediaIds failed",t),new Error("Failed to enumerate transcripts")}}async function sE(e){try{await BS(e,"transcript")}catch(t){throw kp.error(`deleteTranscript(${e}) failed`,t),new Error(`Failed to delete transcript: ${e}`)}}const Pp=["typewriter","fade-up","rise","cascade","pop","blur-in","slide-mask","wave-in"],Mp=["fade-down","sink","pop-out","blur-out","typewriter-erase"],Cp=["pulse","wave","shimmer","swing"];function ao(e){const t=Math.max(-1,Math.min(1,e));return t<0?Math.round(t*32768):Math.round(t*32767)}const rl=44;function Tp(e,t,r){const n=e*2,a=t*n,i=new ArrayBuffer(rl+a),o=new DataView(i),s=(l,c)=>{for(let u=0;u<c.length;u+=1)o.setUint8(l+u,c.charCodeAt(u))};return s(0,"RIFF"),o.setUint32(4,36+a,!0),s(8,"WAVE"),s(12,"fmt "),o.setUint32(16,16,!0),o.setUint16(20,1,!0),o.setUint16(22,e,!0),o.setUint32(24,r,!0),o.setUint32(28,r*n,!0),o.setUint16(32,n,!0),o.setUint16(34,16,!0),s(36,"data"),o.setUint32(40,a,!0),i}function NS(e){const t=Math.max(1,Math.min(2,e.numberOfChannels)),r=e.length,n=Tp(t,r,e.sampleRate),a=new Int16Array(n,rl,r*t),i=e.getChannelData(0);if(t>1){const o=e.getChannelData(1);let s=0;for(let l=0;l<r;l+=1)a[s]=ao(i[l]??0),a[s+1]=ao(o[l]??0),s+=2}else for(let o=0;o<r;o+=1)a[o]=ao(i[o]??0);return new Blob([n],{type:"audio/wav"})}function qS(e,t,r){const n=Math.min(e.length,t.length),a=Tp(2,n,r),i=new Int16Array(a,rl,n*2);let o=0;for(let s=0;s<n;s+=1)i[o]=e[s],i[o+1]=t[s],o+=2;return new Blob([a],{type:"audio/wav"})}const GS=Oe("PreviewAudioConform"),KS=new Map,Ja=new Map;function Ep(e){return`preview-audio:${e}`}function Fp(e){KS.delete(e),Ja.delete(e),Un.invalidate(Ep(e))}lS(({mediaId:e})=>Fp(e));async function lE(e){try{return await yp(hr(),gi(e))}catch{return!1}}function Ap(e,t,r){const n=Ja.get(e);if(n)return n;const a=(async()=>{const i=r??await Zs(e);if(await yp(hr(),gi(e)))return;const o=Ep(e),s=t();Un.get(o)||Un.acquire(o,s),await bS(hr(),gi(e),new Uint8Array(await s.arrayBuffer()),i)})().catch(i=>{GS.warn("Failed to persist preview audio conform asset",{mediaId:e,err:i})}).finally(()=>{Ja.delete(e)});return Ja.set(e,a),a}function cE(e,t,r){return Ap(e,()=>NS(t),r)}function uE(e,t,r,n,a){return Ap(e,()=>qS(t,r,n),a)}async function dE(e){const t=typeof e=="string"?e:e.id;Fp(t),await gp(hr(),gi(t))}const VS=-1,WS=1,jS=.04,$S=.96,YS=.52,vi=1e-4,bc=12;function bi(e){const t=typeof e=="number"&&Number.isFinite(e)?e:0;return Math.max(VS,Math.min(WS,Math.round(t*100)/100))}function $r(e){const t=typeof e=="number"&&Number.isFinite(e)?e:YS;return Math.max(jS,Math.min($S,Math.round(t*1e3)/1e3))}function XS(e,t){const r=$r(t),n=bi(e),a=r,i=1-a,o=a;return n>=0?a+n*i:a+n*o}function QS(e,t){const r=$r(t),n=bi(e),a=1-r,i=1-a,o=a;return n>=0?a+n*i:a+n*o}function nl(e){return Math.max(0,Math.min(1,e))}function Sc(e){return Math.max(vi,Math.min(1-vi,e))}function Si(e,t){const r=Math.log(Sc(t))/Math.log(Sc(e));return Number.isFinite(r)?Math.max(1,Math.min(bc,r)):bc}function Ii(e,t){return Math.pow(nl(e),t)}function mE(e,t,r){const n=nl(e),a=$r(r),i=XS(t,r);if(Math.abs(i-a)<=vi)return n;if(i>a){const s=Si(1-a,1-i);return 1-Ii(1-n,s)}const o=Si(a,i);return Ii(n,o)}function pE(e,t,r){const n=nl(e),a=$r(r),i=QS(t,r),o=1-a;if(Math.abs(i-o)<=vi)return 1-n;if(i>o){const l=Si(a,1-i);return 1-Ii(n,l)}const s=Si(1-a,i);return Ii(1-n,s)}const JS=-12,ZS=12,eI=-100,tI=100,rI=1e-4;function nI(e){return Number.isFinite(e)?e??0:0}function _p(e){return Number.isFinite(e)?Math.max(JS,Math.min(ZS,Math.round(e))):0}function Op(e){return Number.isFinite(e)?Math.max(eI,Math.min(tI,Math.round(e))):0}function fE(e){return e?_p(e.audioPitchSemitones??0)+Op(e.audioPitchCents??0)/100:0}function hE(e){return Math.abs(e??0)>rI}function gE(e){const t=nI(e);return Math.pow(2,t/12)}function Pe(e,t=0){return Number.isFinite(e)?Math.max(0,Math.round(e)):t}function nr(e,t=1){return Number.isFinite(e)?Math.max(1,Math.round(e)):t}function aI(e){if(!(e===void 0||!Number.isFinite(e)||e<=0))return Math.round(e*1e3)/1e3}const iI=[{prefix:"audioEqBand1",hasEnabled:!0,hasGain:!0,hasSlope:!0,qDefault:cr,freq:{min:ci,max:ui,def:li}},{prefix:"audioEqLowCut",hasEnabled:!0,hasSlope:!0,freq:{min:ci,max:ui,def:li}},{prefix:"audioEqLow",hasEnabled:!0,hasGain:!0,qDefault:cr,freq:{min:pm,max:fm,def:mm}},{prefix:"audioEqLowMid",hasEnabled:!0,hasGain:!0,qDefault:cr,freq:{min:gm,max:ym,def:hm}},{prefix:"audioEqMid",hasGain:!0},{prefix:"audioEqHighMid",hasEnabled:!0,hasGain:!0,qDefault:Xa,freq:{min:Im,max:xm,def:Sm}},{prefix:"audioEqHigh",hasEnabled:!0,hasGain:!0,qDefault:Xa,freq:{min:km,max:Pm,def:wm}},{prefix:"audioEqBand6",hasEnabled:!0,hasGain:!0,hasSlope:!0,qDefault:Xa,freq:{min:mi,max:pi,def:di}},{prefix:"audioEqHighCut",hasEnabled:!0,hasSlope:!0,freq:{min:mi,max:pi,def:di}}];function oI(e){const t=[];if(e.hasEnabled&&t.push({key:`${e.prefix}Enabled`,clamp:r=>!!r}),e.hasGain&&t.push({key:`${e.prefix}GainDb`,clamp:r=>ot(r)}),e.freq){const{min:r,max:n,def:a}=e.freq;t.push({key:`${e.prefix}FrequencyHz`,clamp:i=>yt(i,r,n,a)})}if(e.qDefault!==void 0){const r=e.qDefault;t.push({key:`${e.prefix}Q`,clamp:n=>bt(n,r)})}return e.hasSlope&&t.push({key:`${e.prefix}SlopeDbPerOct`,clamp:r=>Sn(r)}),t}const sI=[{key:"trimStart",clamp:e=>Pe(e)},{key:"trimEnd",clamp:e=>Pe(e)},{key:"sourceStart",clamp:e=>Pe(e)},{key:"sourceEnd",clamp:e=>Pe(e)},{key:"sourceDuration",clamp:e=>Pe(e)},{key:"sourceFps",clamp:e=>aI(e)},{key:"crop",clamp:e=>Sh(e)},{key:"audioFadeInCurve",clamp:e=>bi(e)},{key:"audioFadeOutCurve",clamp:e=>bi(e)},{key:"audioFadeInCurveX",clamp:e=>$r(e)},{key:"audioFadeOutCurveX",clamp:e=>$r(e)},{key:"audioPitchSemitones",clamp:e=>_p(e)},{key:"audioPitchCents",clamp:e=>Op(e)},{key:"audioEqOutputGainDb",clamp:e=>ot(e)},...iI.flatMap(oI)];function Dp(e){for(const{key:t,clamp:r}of sI){const n=e[t];n!==void 0&&(e[t]=r(n))}}const xi=1;function xa(e,t=100){return e===void 0?void 0:Math.max(0,Math.min(t,e))}function fe(e){const t={...e};t.from=Pe(e.from),t.durationInFrames=nr(e.durationInFrames),Dp(t);const r=t;if(r.cornerPin){const n=Vy(r.transform?.width??0,r.transform?.height??0,r.type==="video"||r.type==="image"?{sourceWidth:r.sourceWidth,sourceHeight:r.sourceHeight,crop:r.crop}:void 0);r.cornerPin={...r.cornerPin,referenceWidth:r.cornerPin.referenceWidth??(n.width>0?n.width:void 0),referenceHeight:r.cornerPin.referenceHeight??(n.height>0?n.height:void 0)}}return r.type==="shape"&&(r.taperStartWidth=xa(r.taperStartWidth,200),r.taperEndWidth=xa(r.taperEndWidth,200),r.taperStartLength=xa(r.taperStartLength),r.taperEndLength=xa(r.taperEndLength),r.shapeType==="path"&&(r.pathClosed=r.isMask?!0:r.pathClosed??!0,r.pathClosed===!1?r.fillEnabled=!1:r.fillEnabled??=!0,r.strokeEnabled??=!0,r.strokeLineCap??="butt",r.strokeLineJoin??="miter",r.strokeMiterLimit??=4,r.pathVertices=r.pathVertices?.map(n=>({...n,tangentMode:n.tangentMode??(n.inHandle[0]===0&&n.inHandle[1]===0&&n.outHandle[0]===0&&n.outHandle[1]===0?"corner":"smooth")}))),r.strokeEnabled===!0&&(r.strokeWidth??0)<xi&&(r.strokeWidth=xi),r.isMask&&(r.blendMode="normal")),(r.type==="video"||r.type==="audio")&&r.sourceEnd!==void 0&&r.sourceStart===void 0&&(r.sourceStart=0),r}function Ic(e){const t={...e};return t.from!==void 0&&(t.from=Pe(t.from)),t.durationInFrames!==void 0&&(t.durationInFrames=nr(t.durationInFrames)),Dp(t),t.strokeEnabled===!0&&typeof t.strokeWidth=="number"&&t.strokeWidth<xi&&(t.strokeWidth=xi),t.sourceEnd!==void 0&&t.sourceStart===void 0&&(t.sourceStart=0),t}function lI(e){return{...e,volume:e.volume===void 0?void 0:Math.max(-60,Math.min(12,e.volume)),audioEq:ry(e.audioEq)}}function cI(e,t,r){if(t===0)return null;const n=t/r,a=[];for(const i of e.cues){if(i.endSeconds<=n)continue;const o=Math.max(0,i.startSeconds-n),s=i.endSeconds-n;s<=o||a.push({...i,startSeconds:o,endSeconds:s})}return{cues:a}}function uI(e,t,r){const n=t/r,a=[];for(const i of e.cues){if(i.startSeconds>=n)continue;const o=Math.min(i.endSeconds,n);o<=i.startSeconds||a.push({...i,endSeconds:o})}return{cues:a}}const xc=e=>{let t;const r=new Set,n=(c,u)=>{const d=typeof c=="function"?c(t):c;if(!Object.is(d,t)){const m=t;t=u??(typeof d!="object"||d===null)?d:Object.assign({},t,d),r.forEach(p=>p(t,m))}},a=()=>t,s={setState:n,getState:a,getInitialState:()=>l,subscribe:c=>(r.add(c),()=>r.delete(c))},l=t=e(n,a,s);return s},dI=(e=>e?xc(e):xc);var wc=(e,t,r)=>(a,i)=>({pastStates:r?.pastStates||[],futureStates:r?.futureStates||[],undo:(o=1)=>{if(i().pastStates.length){const s=r?.partialize?.(t())||t(),l=i().pastStates.splice(-o,o),c=l.shift();e(c),a({pastStates:i().pastStates,futureStates:i().futureStates.concat(r?.diff?.(s,c)||s,l.reverse())})}},redo:(o=1)=>{if(i().futureStates.length){const s=r?.partialize?.(t())||t(),l=i().futureStates.splice(-o,o),c=l.shift();e(c),a({pastStates:i().pastStates.concat(r?.diff?.(s,c)||s,l.reverse()),futureStates:i().futureStates})}},clear:()=>a({pastStates:[],futureStates:[]}),isTracking:!0,pause:()=>a({isTracking:!1}),resume:()=>a({isTracking:!0}),setOnSave:o=>a({_onSave:o}),_onSave:r?.onSave,_handleSet:(o,s,l,c)=>{r?.limit&&i().pastStates.length>=r?.limit&&i().pastStates.shift(),i()._onSave?.(o,l),a({pastStates:i().pastStates.concat(c||o),futureStates:[]})}}),mI=(e,t)=>(n,a,i)=>{i.temporal=dI(t?.wrapTemporal?.(wc(n,a,t))||wc(n,a,t));const o=t?.handleSet?.(i.temporal.getState()._handleSet)||i.temporal.getState()._handleSet,s=c=>{if(!i.temporal.getState().isTracking)return;const u=t?.partialize?.(a())||a(),d=t?.diff?.(c,u);d===null||t?.equality?.(c,u)||o(c,void 0,u,d)},l=i.setState;return i.setState=(...c)=>{const u=t?.partialize?.(a())||a();l(...c),s(u)},e((...c)=>{const u=t?.partialize?.(a())||a();n(...c),s(u)},a,i)};const Vn={PLAY_PAUSE:"space",PREVIOUS_FRAME:"left",NEXT_FRAME:"right",GO_TO_START:"home",GO_TO_END:"end",NEXT_SNAP_POINT:"down",PREVIOUS_SNAP_POINT:"up",SPLIT_AT_PLAYHEAD_ALT:"alt+c",JOIN_ITEMS:"shift+j",DELETE_SELECTED:"delete",DELETE_SELECTED_ALT:"backspace",RIPPLE_DELETE:"mod+delete",RIPPLE_DELETE_ALT:"mod+backspace",FREEZE_FRAME:"shift+f",LINK_AUDIO_VIDEO:"mod+alt+l",UNLINK_AUDIO_VIDEO:"alt+shift+l",TOGGLE_LINKED_SELECTION:"shift+l",NUDGE_LEFT:"shift+left",NUDGE_RIGHT:"shift+right",NUDGE_UP:"shift+up",NUDGE_DOWN:"shift+down",NUDGE_LEFT_LARGE:"mod+shift+left",NUDGE_RIGHT_LARGE:"mod+shift+right",NUDGE_UP_LARGE:"mod+shift+up",NUDGE_DOWN_LARGE:"mod+shift+down",UNDO:"mod+z",REDO:"mod+shift+z",ZOOM_IN:"mod+equal",ZOOM_OUT:"mod+minus",ZOOM_TO_FIT:"backslash",ZOOM_TO_100:"shift+backslash",ZOOM_TO_100_ALT:"mod+0",COPY:"mod+c",CUT:"mod+x",PASTE:"mod+v",SELECTION_TOOL:"v",TRIM_EDIT_TOOL:"t",RAZOR_TOOL:"c",SPLIT_AT_CURSOR:"shift+c",RATE_STRETCH_TOOL:"r",SLIP_TOOL:"y",SLIDE_TOOL:"u",SAVE:"mod+s",EXPORT:"mod+shift+e",TOGGLE_SNAP:"s",TOGGLE_CANVAS_SNAP:"shift+s",OPEN_SCENE_BROWSER:"mod+shift+f",WORKSPACE_EDIT:"alt+1",WORKSPACE_COLOR:"alt+2",WORKSPACE_ANIMATE:"alt+3",ADD_MARKER:"m",REMOVE_MARKER:"shift+m",PREVIOUS_MARKER:"bracketleft",NEXT_MARKER:"bracketright",CLEAR_KEYFRAMES:"shift+a",KEYFRAME_EDITOR_GRAPH:"1",KEYFRAME_EDITOR_DOPESHEET:"2",KEYFRAME_EDITOR_SPLIT:"3",EDIT_KEYFRAME_ADD:"k",KEYFRAME_PREVIOUS:"alt+bracketleft",KEYFRAME_NEXT:"alt+bracketright",KEYFRAME_TOGGLE_AUTO:"a",KEYFRAME_FIT:"f",MARK_IN:"i",MARK_OUT:"o",CLEAR_IN_OUT:"alt+x",INSERT_EDIT:"comma",OVERWRITE_EDIT:"period"},Rp=["mod","alt","shift"],Lp=new Set(Rp),kc=new Map(Rp.map((e,t)=>[e,t])),pI={cmd:"mod",command:"mod",ctrl:"mod",control:"mod",option:"alt",return:"enter",esc:"escape",del:"delete","=":"equal",equals:"equal","-":"minus",arrowleft:"left",arrowright:"right",arrowup:"up",arrowdown:"down"},fI=[{binding:"alt+left",browserAction:"Back navigation"},{binding:"alt+right",browserAction:"Forward navigation"},{binding:"f5",browserAction:"Reload page"},{binding:"mod+r",browserAction:"Reload page"},{binding:"mod+shift+r",browserAction:"Hard reload page"},{binding:"mod+t",browserAction:"New tab"},{binding:"mod+shift+t",browserAction:"Reopen closed tab"},{binding:"mod+w",browserAction:"Close tab"},{binding:"mod+n",browserAction:"New window"},{binding:"mod+shift+n",browserAction:"New private window"},{binding:"mod+l",browserAction:"Focus address bar"},{binding:"mod+shift+l",browserAction:"Focus address bar or search in some browsers"},{binding:"mod+d",browserAction:"Bookmark page or focus address bar"},{binding:"mod+e",browserAction:"Focus search or address bar in some browsers"},{binding:"mod+p",browserAction:"Print page"},{binding:"mod+f",browserAction:"Find in page"},{binding:"mod+equal",browserAction:"Browser zoom in"},{binding:"mod+minus",browserAction:"Browser zoom out"},{binding:"mod+0",browserAction:"Reset browser zoom"},{binding:"mod+1",browserAction:"Switch to tab 1"},{binding:"mod+2",browserAction:"Switch to tab 2"},{binding:"mod+3",browserAction:"Switch to tab 3"},{binding:"mod+4",browserAction:"Switch to tab 4"},{binding:"mod+5",browserAction:"Switch to tab 5"},{binding:"mod+6",browserAction:"Switch to tab 6"},{binding:"mod+7",browserAction:"Switch to tab 7"},{binding:"mod+8",browserAction:"Switch to tab 8"},{binding:"mod+9",browserAction:"Switch to last tab"}];new Map(fI.map(e=>[e.binding,e]));const hI={PLAY_PAUSE:"Play/Pause",PREVIOUS_FRAME:"Previous frame",NEXT_FRAME:"Next frame",GO_TO_START:"Go to start",GO_TO_END:"Go to end",NEXT_SNAP_POINT:"Next snap point",PREVIOUS_SNAP_POINT:"Previous snap point",SPLIT_AT_PLAYHEAD_ALT:"Split at playhead",JOIN_ITEMS:"Join selected clips",DELETE_SELECTED:"Delete selected items",DELETE_SELECTED_ALT:"Delete selected items (alternative)",RIPPLE_DELETE:"Ripple delete selected items",RIPPLE_DELETE_ALT:"Ripple delete selected items (alternative)",FREEZE_FRAME:"Insert freeze frame at playhead",LINK_AUDIO_VIDEO:"Link selected clips",UNLINK_AUDIO_VIDEO:"Unlink selected clips",TOGGLE_LINKED_SELECTION:"Toggle linked selection",NUDGE_LEFT:"Nudge selected visual items left (1px)",NUDGE_RIGHT:"Nudge selected visual items right (1px)",NUDGE_UP:"Nudge selected visual items up (1px)",NUDGE_DOWN:"Nudge selected visual items down (1px)",NUDGE_LEFT_LARGE:"Nudge selected visual items left (10px)",NUDGE_RIGHT_LARGE:"Nudge selected visual items right (10px)",NUDGE_UP_LARGE:"Nudge selected visual items up (10px)",NUDGE_DOWN_LARGE:"Nudge selected visual items down (10px)",UNDO:"Undo",REDO:"Redo",ZOOM_IN:"Zoom in timeline",ZOOM_OUT:"Zoom out timeline",ZOOM_TO_FIT:"Zoom to fit all content",ZOOM_TO_100:"Zoom to 100% at cursor or playhead",ZOOM_TO_100_ALT:"Zoom to 100% at cursor or playhead (alternative)",COPY:"Copy selected items or keyframes",CUT:"Cut selected items or keyframes",PASTE:"Paste items or keyframes",SELECTION_TOOL:"Selection tool",TRIM_EDIT_TOOL:"Trim edit tool",RAZOR_TOOL:"Razor tool",SPLIT_AT_CURSOR:"Split at cursor",RATE_STRETCH_TOOL:"Rate stretch tool",SLIP_TOOL:"Slip tool",SLIDE_TOOL:"Slide tool",SAVE:"Save project",EXPORT:"Export video",TOGGLE_SNAP:"Toggle snap",TOGGLE_CANVAS_SNAP:"Toggle canvas (gizmo) snap",OPEN_SCENE_BROWSER:"Open Scene Browser (search AI captions)",WORKSPACE_EDIT:"Switch to Edit workspace",WORKSPACE_COLOR:"Switch to Color workspace",WORKSPACE_ANIMATE:"Switch to Motion workspace",ADD_MARKER:"Add marker at playhead",REMOVE_MARKER:"Remove selected marker",PREVIOUS_MARKER:"Jump to previous marker",NEXT_MARKER:"Jump to next marker",CLEAR_KEYFRAMES:"Clear all keyframes from selected items",KEYFRAME_EDITOR_GRAPH:"Switch keyframe editor to graph view",KEYFRAME_EDITOR_DOPESHEET:"Switch keyframe editor to dopesheet view",KEYFRAME_EDITOR_SPLIT:"Switch keyframe editor to split view",EDIT_KEYFRAME_ADD:"Add keyframe at playhead for selected Edit layer",KEYFRAME_PREVIOUS:"Jump to previous property keyframe",KEYFRAME_NEXT:"Jump to next property keyframe",KEYFRAME_TOGGLE_AUTO:"Toggle auto-key for active property",KEYFRAME_FIT:"Fit selected keyframes in view",MARK_IN:"Mark In point",MARK_OUT:"Mark Out point",CLEAR_IN_OUT:"Clear In/Out points",INSERT_EDIT:"Insert edit",OVERWRITE_EDIT:"Overwrite edit"};bI();function gI(e){return e.trim()===""}function yI(e){return e in Vn}function vI(e){return e.trim().toLowerCase()}function bI(){const e=new Map,t=new Map;for(const r of Object.keys(Vn))e.set(vI(hI[r]),r),t.set(al(Vn[r]),r);return{byLabel:e,byDefaultBinding:t}}function SI(e){const t=e.trim().toLowerCase();return t?pI[t]??t:""}function Bp(e){return e.split("+").map(t=>SI(t)).filter(Boolean)}function al(e){const t=new Set,r=[];for(const a of Bp(e)){if(Lp.has(a)){t.add(a);continue}r.includes(a)||r.push(a)}return[...Array.from(t).sort((a,i)=>(kc.get(a)??99)-(kc.get(i)??99)),...r].join("+")}function Pc(e){if(!e||typeof e!="object")return{};const t={};for(const[r,n]of Object.entries(e)){if(!yI(r)||typeof n!="string")continue;if(gI(n)){t[r]="";continue}const a=al(n);!a||!II(a)||a!==Vn[r]&&(t[r]=a)}return t}function II(e){return Bp(e).some(t=>!Lp.has(t))}const zp=[{id:"netflix",label:"Netflix",hintKey:"editor.captionPresets.netflixHint",patch:{fontFamily:"Inter",fontWeight:"semibold",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:"rgba(0, 0, 0, 0.55)",backgroundRadius:4,textAlign:"center",verticalAlign:"middle",lineHeight:1.15,letterSpacing:0,textPadding:12,textShadow:{offsetX:0,offsetY:2,blur:6,color:"rgba(0, 0, 0, 0.6)"},stroke:void 0},layout:{fontSizeRatio:.04,yRatio:.36,widthRatio:.7,heightRatio:.16}},{id:"youtube",label:"YouTube",hintKey:"editor.captionPresets.youtubeHint",patch:{fontFamily:"Roboto",fontWeight:"medium",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.2,letterSpacing:0,textPadding:0,textShadow:{offsetX:0,offsetY:4,blur:14,color:"rgba(0, 0, 0, 0.85)"},stroke:void 0},layout:{fontSizeRatio:.045,yRatio:.34,widthRatio:.85,heightRatio:.18}},{id:"bold-yellow",label:"Bold Yellow",hintKey:"editor.captionPresets.boldYellowHint",patch:{fontFamily:"Roboto Slab",fontWeight:"bold",fontStyle:"normal",underline:!1,color:"#FFD400",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.1,letterSpacing:0,textPadding:0,textShadow:{offsetX:0,offsetY:3,blur:5,color:"rgba(0, 0, 0, 1)"},stroke:{width:1.5,color:"#000000"}},layout:{fontSizeRatio:.05,yRatio:.38,widthRatio:.85,heightRatio:.18}},{id:"minimal-stroke",label:"Outlined",hintKey:"editor.captionPresets.outlinedHint",patch:{fontFamily:"Manrope",fontWeight:"medium",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.2,letterSpacing:0,textPadding:0,textShadow:void 0,stroke:{width:1,color:"#000000"}},layout:{fontSizeRatio:.04,yRatio:.34,widthRatio:.85,heightRatio:.16}},{id:"tiktok",label:"TikTok",hintKey:"editor.captionPresets.tiktokHint",patch:{fontFamily:"Anton",fontWeight:"normal",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.05,letterSpacing:1,textPadding:0,textShadow:{offsetX:0,offsetY:4,blur:8,color:"rgba(0, 0, 0, 0.9)"},stroke:{width:2,color:"#000000"}},layout:{fontSizeRatio:.075,yRatio:0,widthRatio:.9,heightRatio:.22}}];function xI(e){return e==="semantic"?"semantic":"keyword"}const Up=zp[0]?.id??"netflix";function Mc(e){return typeof e=="string"&&zp.some(t=>t.id===e)?e:Up}const wI={seconds:{min:.5,max:60},frames:{min:1,max:1800}},Hp=3;function Cc(e){return e==="frames"?"frames":"seconds"}function io(e,t){const r=wI[t],n=t==="seconds"?Hp:90,a=typeof e=="number"&&Number.isFinite(e)?e:n;return Math.min(r.max,Math.max(r.min,a))}function kI(e,t){const r=Object.keys(e),n=Object.keys(t);return r.length!==n.length?!1:r.every(a=>e[a]===t[a])}const Tc={snapEnabled:!0,timelineSectionDividerPosition:null,canvasSnapEnabled:!0,showWaveforms:!0,showFilmstrips:!0,enableFilmstripExtraction:!0,editorDensity:vh,maxUndoHistory:50,autoSaveInterval:5,captioningIntervalUnit:"seconds",captioningIntervalValue:Hp,captionSearchMode:"keyword",defaultCaptionStylePresetId:Up,hotkeyOverrides:{}},Np=Ie()(Sd(e=>({...Tc,setSetting:(t,r)=>e(n=>{if(t==="captioningIntervalUnit"){const a=Cc(r);return{captioningIntervalUnit:a,captioningIntervalValue:io(n.captioningIntervalValue,a)}}return t==="captioningIntervalValue"?{captioningIntervalValue:io(r,n.captioningIntervalUnit)}:t==="editorDensity"?{editorDensity:Cl(r)}:t==="defaultCaptionStylePresetId"?{defaultCaptionStylePresetId:Mc(r)}:{[t]:r}}),setHotkeyBinding:(t,r)=>e(n=>{const a=al(r);if(!a||a===Vn[t]){if(!(t in n.hotkeyOverrides))return n;const i={...n.hotkeyOverrides};return delete i[t],{hotkeyOverrides:i}}return n.hotkeyOverrides[t]===a?n:{hotkeyOverrides:{...n.hotkeyOverrides,[t]:a}}}),unbindHotkeyBinding:t=>e(r=>r.hotkeyOverrides[t]===""?r:{hotkeyOverrides:{...r.hotkeyOverrides,[t]:""}}),replaceHotkeyOverrides:t=>e(r=>{const n=Pc(t);return kI(r.hotkeyOverrides,n)?r:{hotkeyOverrides:n}}),resetHotkeyBinding:t=>e(r=>{if(!(t in r.hotkeyOverrides))return r;const n={...r.hotkeyOverrides};return delete n[t],{hotkeyOverrides:n}}),resetHotkeys:()=>e(t=>Object.keys(t.hotkeyOverrides).length===0?t:{hotkeyOverrides:{}}),resetToDefaults:()=>e(Tc)}),{name:"freecut-settings",version:2,migrate:(e,t)=>{let r=e??{};return t<1&&(r.autoSaveInterval==null||r.autoSaveInterval<=0)&&(r={...r,autoSaveInterval:5}),r},merge:(e,t)=>{const r=e??{},n=Cc(r.captioningIntervalUnit);return{...t,...r,hotkeyOverrides:Pc(r.hotkeyOverrides),editorDensity:Cl(r.editorDensity),captioningIntervalUnit:n,captioningIntervalValue:io(r.captioningIntervalValue,n),captionSearchMode:xI(r.captionSearchMode),defaultCaptionStylePresetId:Mc(r.defaultCaptionStylePresetId)}}})),Ec={"projects.copySuffix":"{{name}} (Bản sao)","projects.upgradeBackupName":"{{name}} (Bản sao lưu trước khi nâng cấp v{{from}} lên v{{to}})","projects.relativeTime.justNow":"vừa xong","projects.relativeTime.minutesAgo":"{{count}} phút trước","projects.relativeTime.hoursAgo":"{{count}} giờ trước","projects.relativeTime.daysAgo":"{{count}} ngày trước","projects.relativeTime.weeksAgo":"{{count}} tuần trước","projects.relativeTime.monthsAgo":"{{count}} tháng trước","projects.relativeTime.yearsAgo":"{{count}} năm trước","projects.validation.nameRequired":"Tên project là bắt buộc","projects.validation.nameTooLong":"Tên project phải ngắn hơn 100 ký tự","projects.validation.nameWhitespace":"Tên project không được chỉ gồm khoảng trắng","projects.validation.descriptionTooLong":"Mô tả phải ngắn hơn 500 ký tự","projects.validation.widthInteger":"Chiều rộng phải là số nguyên","projects.validation.widthMin":"Chiều rộng tối thiểu 320px","projects.validation.widthMax":"Chiều rộng tối đa 7680px (8K)","projects.validation.heightInteger":"Chiều cao phải là số nguyên","projects.validation.heightMin":"Chiều cao tối thiểu 240px","projects.validation.heightMax":"Chiều cao tối đa 4320px (8K)","projects.validation.fpsInteger":"FPS phải là số nguyên","projects.validation.fpsMin":"FPS tối thiểu 1","projects.validation.fpsMax":"FPS tối đa 240","projects.validation.fpsUnsupported":"FPS phải là một tốc độ khung hình được hỗ trợ","projects.validation.invalidHexColor":"Phải là mã màu hex hợp lệ (ví dụ #000000)","preview.mediaOffline.title":"Media ngoại tuyến","preview.mediaOffline.relink":"Liên kết lại trong panel Media","preview.mediaOffline.loading":"Đang tải…"};function PI(e,t){typeof t=="string"&&(t={defaultValue:t});let n=Ec[e]??Ec[e.replace(/_(one|other|few|many|zero|two)$/,"")]??(typeof t?.defaultValue=="string"?t.defaultValue:e);if(t)for(const[a,i]of Object.entries(t))i!=null&&(n=n.replaceAll(`{{${a}}}`,String(i)));return n}const MI={t:PI,language:"vi"},Pt=Ie()(vd(mI((e,t)=>({projects:[],currentProject:null,isLoading:!1,error:null,searchQuery:"",sortField:"updatedAt",sortDirection:"desc",filterResolution:void 0,filterFps:void 0,loadProjects:async()=>{e({isLoading:!0,error:null});try{const r=await rS();e({projects:r,isLoading:!1})}catch(r){const n=r instanceof Error?r.message:"Failed to load projects";throw e({error:n,isLoading:!1}),r}},loadProject:async r=>{e({isLoading:!0,error:null});try{const n=await nS(r);return n?(e({currentProject:n,isLoading:!1}),n):(e({error:`Project not found: ${r}`,isLoading:!1}),null)}catch(n){const a=n instanceof Error?n.message:"Failed to load project";throw e({error:a,isLoading:!1}),n}},createProject:async r=>{e({error:null});try{const n=await aS({name:r.name,description:r.description,width:r.width,height:r.height,fps:r.fps,backgroundColor:r.backgroundColor});return e(a=>({projects:[...a.projects,n],currentProject:n})),n}catch(n){const a=n instanceof Error?n.message:"Failed to create project";throw e({error:a}),n}},updateProject:async(r,n)=>{e({error:null});const a=t().projects,i=t().currentProject,o=a.findIndex(c=>c.id===r);let s=null;if(o!==-1?s=a[o]??null:i?.id===r&&(s=i),!s)throw e({error:`Project not found: ${r}`}),new Error(`Project not found: ${r}`);const l={...s,name:n.name??s.name,description:n.description??s.description,metadata:{width:n.width??s.metadata.width,height:n.height??s.metadata.height,fps:n.fps??s.metadata.fps,backgroundColor:n.backgroundColor??s.metadata.backgroundColor},updatedAt:Date.now()};if(o!==-1){const c=[...a];c[o]=l,e({projects:c})}i?.id===r&&e({currentProject:l});try{await Jb(r);const c=await iS(r,{name:l.name,description:l.description,metadata:l.metadata});return t().currentProject?.id===r&&e({currentProject:c}),c}catch(c){o!==-1&&e({projects:a}),i?.id===r&&e({currentProject:i});const u=c instanceof Error?c.message:"Failed to update project";throw e({error:u}),c}},deleteProject:async r=>{e({error:null});const n=t().projects,a=t().currentProject;e({projects:n.filter(i=>i.id!==r),currentProject:a?.id===r?null:a});try{await sS(r),e(i=>({projects:i.projects.filter(o=>o.id!==r),currentProject:i.currentProject?.id===r?null:i.currentProject}))}catch(i){e({projects:n,currentProject:a});const o=i instanceof Error?i.message:"Failed to delete project";throw e({error:o}),i}},duplicateProject:async r=>{e({error:null});const n=t().projects.find(a=>a.id===r);if(!n)throw e({error:`Project not found: ${r}`}),new Error(`Project not found: ${r}`);try{const a=await oS(r,MI.t("projects.copySuffix",{name:n.name}));return e(i=>({projects:[...i.projects,a]})),a}catch(a){const i=a instanceof Error?a.message:"Failed to duplicate project";throw e({error:i}),a}},setCurrentProject:r=>e({currentProject:r}),setSearchQuery:r=>e({searchQuery:r}),setSortField:r=>e({sortField:r}),setSortDirection:r=>e({sortDirection:r}),setFilterResolution:r=>e({filterResolution:r}),setFilterFps:r=>e({filterFps:r}),clearFilters:()=>e({searchQuery:"",filterResolution:void 0,filterFps:void 0}),clearError:()=>e({error:null})}),{partialize:e=>({projects:e.projects,currentProject:e.currentProject})}),{name:"ProjectStore",enabled:!1}));Pt.temporal.getState().setOnSave(()=>{const e=Np.getState().maxUndoHistory,{pastStates:t,futureStates:r}=Pt.temporal.getState();(t.length>e||r.length>e)&&Pt.temporal.setState({pastStates:t.slice(-e),futureStates:r.slice(-e)})});Np.subscribe((e,t)=>{if(e.maxUndoHistory!==t.maxUndoHistory){const{pastStates:r,futureStates:n}=Pt.temporal.getState();(r.length>e.maxUndoHistory||n.length>e.maxUndoHistory)&&Pt.temporal.setState({pastStates:r.slice(-e.maxUndoHistory),futureStates:n.slice(-e.maxUndoHistory)})}});function CI(e,t,r,n){const a=r/e,i=n/t,o=Math.min(a,i);return{x:0,y:0,width:Math.round(e*o),height:Math.round(t*o),rotation:0}}function il(e,t,r){const n={id:e.id,from:e.from+t,durationInFrames:e.durationInFrames-t};if(e.type==="video"||e.type==="audio"||e.type==="composition"){const a=e.sourceStart??0,i=e.sourceEnd,o=e.speed??1,s=e.sourceFps??r,l=we(t,o,r,s);n.sourceStart=a+l,i!==void 0&&(n.sourceEnd=i)}return n}function ol(e,t,r){const n={id:e.id,durationInFrames:e.durationInFrames+t};if(e.type==="video"||e.type==="audio"||e.type==="composition"){const a=e.sourceStart??0,i=e.speed??1,o=e.sourceFps??r,s=we(n.durationInFrames??e.durationInFrames,i,r,o);n.sourceEnd=a+s}return n}function TI(e,t){return{id:e.id,from:e.from+t}}function qp(e,t){return e.type==="video"&&t.type==="audio"||e.type==="audio"&&t.type==="video"}function EI(e,t){return!qp(e,t)||!e.originId||e.originId!==t.originId||!e.mediaId||e.mediaId!==t.mediaId?!1:e.from===t.from&&e.durationInFrames===t.durationInFrames}function sa(e,t){const r=e.find(a=>a.id===t);if(!r)return[];if(r.linkedGroupId)return e.filter(a=>a.linkedGroupId===r.linkedGroupId);const n=e.filter(a=>a.id===t||EI(r,a));return n.length>1?n:[r]}function la(e,t){return sa(e,t).map(r=>r.id)}function sl(e,t){const r=e.find(n=>n.id===t);return!r||r.type==="text"?[]:e.filter(n=>n.type==="text"&&(n.textRole==="caption"||n.captionSource!==void 0)&&n.captionSource?.clipId===r.id).map(n=>n.id)}function Wn(e,t){const r=new Set,n=new Set;for(const a of t){r.add(a);for(const i of sl(e,a))n.add(i)}for(const a of n)r.add(a);return Array.from(r)}function Gp(e,t){const r=[],n=new Set;for(const a of t){if(n.has(a))continue;const i=la(e,a);if(i.length!==0){r.push(a);for(const o of i)n.add(o)}}return r}function jn(e,t){const r=sa(e,t),n=r.find(i=>i.id===t);if(!n)return[];const a=r.filter(i=>i.id===n.id||i.from===n.from&&i.durationInFrames===n.durationInFrames&&(i.sourceStart??null)===(n.sourceStart??null)&&(i.sourceEnd??null)===(n.sourceEnd??null)&&(i.speed??1)===(n.speed??1));return a.length>0?a:[n]}function FI(e,t,r,n){return jn(e,t).find(a=>a.id!==t&&a.trackId===r&&a.type===n)??null}function AI(e,t,r){const n=jn(e,t).filter(i=>i.id!==t),a=jn(e,r).filter(i=>i.id!==r);for(const i of n){const o=a.find(s=>s.trackId===i.trackId&&s.type===i.type);if(o)return{leftCounterpart:i,rightCounterpart:o}}return null}function _I(e){if(e.length!==2)return!1;const[t,r]=e;return!(!t||!r||!qp(t,r)||!t.mediaId||t.mediaId!==r.mediaId||t.from!==r.from||t.durationInFrames!==r.durationInFrames||(t.sourceStart??null)!==(r.sourceStart??null)||(t.sourceEnd??null)!==(r.sourceEnd??null))}function OI(e,t){const r=Array.from(new Set(t)).filter(o=>e.some(s=>s.id===o));if(r.length<2)return!1;const n=$n(e,r);if(n.length<2)return!1;const[a]=n;if(!a)return!1;const i=new Set(la(e,a));return i.size!==n.length||n.some(o=>!i.has(o))}function $n(e,t){const r=new Set;for(const n of t)for(const a of la(e,n))r.add(a);return Array.from(r)}const Kp={crossfade:{label:"Crossfade",description:"Smooth opacity blend between clips",defaultDuration:30,minDuration:30,maxDuration:90}};function os(e){return Number.isFinite(e)?Math.max(1,Math.round(e)):1}function ss(e){return typeof e!="number"||!Number.isFinite(e)?.5:Math.max(0,Math.min(1,e))}function Fc(e){const t=e.timing==="spring"?"linear":e.timing;return{...e,durationInFrames:os(e.durationInFrames),alignment:ss(e.alignment),timing:t??"linear"}}function DI(e,t){if(!e||e.length!==t.length)return!1;for(let r=0;r<t.length;r++)if(e[r]!==t[r])return!1;return!0}function RI(e,t){const r={};for(const a of e)(r[a.trackId]??=[]).push(a);const n={};for(const[a,i]of Object.entries(r)){const o=t[a];n[a]=o&&DI(o,i)?o:i}return n}function LI(e,t){const r={};for(const a of e){const i=Math.min(1,Math.max(0,a.alignment??.5)),o=Math.floor(a.durationInFrames*i),s=a.durationInFrames-o,l=r[a.leftClipId]??{left:0,right:0};l.right=Math.max(l.right,o),r[a.leftClipId]=l;const c=r[a.rightClipId]??{left:0,right:0};c.left=Math.max(c.left,s),r[a.rightClipId]=c}const n={};for(const[a,i]of Object.entries(r)){const o=t[a];n[a]=o&&o.left===i.left&&o.right===i.right?o:i}return n}function Ir(e,t){return{transitions:e,transitionsByTrackId:RI(e,t.transitionsByTrackId),transitionOverlapByItemId:LI(e,t.transitionOverlapByItemId)}}const q=Ie()(e=>({transitions:[],transitionsByTrackId:{},transitionOverlapByItemId:{},setTransitions:t=>e(r=>{const n=t.map(a=>Fc(a));return Ir(n,r)}),_addTransition:(t,r,n,a="crossfade",i,o="fade",s,l=.5)=>{const c=Kp[a],u=os(i??c.defaultDuration),d=$(),m={id:d,leftClipId:t,rightClipId:r,trackId:n,type:a,durationInFrames:u,presentation:o,timing:"linear",direction:s,alignment:ss(l)};return e(p=>{const f=[...p.transitions,m];return Ir(f,p)}),d},_updateTransition:(t,r)=>e(n=>{const a=n.transitions.map(i=>{if(i.id!==t)return i;const o={...r};return r.durationInFrames!==void 0&&(o.durationInFrames=os(r.durationInFrames)),r.alignment!==void 0&&(o.alignment=ss(r.alignment)),Fc({...i,...o})});return Ir(a,n)}),_removeTransition:t=>e(r=>{const n=r.transitions.filter(a=>a.id!==t);return Ir(n,r)}),_removeTransitions:t=>e(r=>{const n=new Set(t),a=r.transitions.filter(i=>!n.has(i.id));return Ir(a,r)}),_removeTransitionsForItems:t=>e(r=>{const n=new Set(t),a=r.transitions.filter(i=>!n.has(i.leftClipId)&&!n.has(i.rightClipId));return Ir(a,r)})}));function ll(e,t){if(!e||e.length!==t.length)return!1;for(let r=0;r<t.length;r++)if(e[r]!==t[r])return!1;return!0}function BI(e,t){const r={};for(const a of e)(r[a.trackId]??=[]).push(a);const n={};for(const[a,i]of Object.entries(r)){const o=t[a];n[a]=o&&ll(o,i)?o:i}return n}function zI(e,t){const r={};for(const a of e)a.linkedGroupId&&(r[a.linkedGroupId]??=[]).push(a);const n={};for(const[a,i]of Object.entries(r)){const o=t[a];n[a]=o&&ll(o,i)?o:i}return n}function UI(e){return(e.type==="audio"||e.type==="video")&&typeof e.mediaId=="string"&&e.mediaId.length>0}function HI(e,t){return e.type==="video"&&t.type==="audio"||e.type==="audio"&&t.type==="video"}function NI(e){return!e.linkedGroupId&&UI(e)&&typeof e.originId=="string"&&e.originId.length>0}function qI(e,t){return!HI(e,t)||!e.originId||e.originId!==t.originId||!e.mediaId||e.mediaId!==t.mediaId?!1:e.from===t.from&&e.durationInFrames===t.durationInFrames}function GI(e,t,r){const n={};for(const i of Object.values(t))if(!(i.length<=1))for(const o of i)n[o.id]=i;const a={};for(const i of e){if(!NI(i))continue;const o=`${i.originId}|${i.mediaId}|${i.from}|${i.durationInFrames}`;(a[o]??=[]).push(i)}for(const i of Object.values(a))if(!(i.length<=1))for(const o of i){const s=i.filter(c=>c.id===o.id||qI(o,c));if(s.length<=1)continue;const l=r[o.id];n[o.id]=l&&ll(l,s)?l:s}return n}function KI(e,t){const r={};for(const n of e){const a=t[n.id];r[n.id]=a!==void 0&&a===n?a:n}return r}function VI(e){const t=new Set;for(const r of e)r.mediaId&&t.add(r.mediaId);return[...t].sort()}function Vp(e){return e.join("|")}function WI(e){let t=0;for(const r of e){const n=r.from+r.durationInFrames;n>t&&(t=n)}return t}function pe(e,t){const r=zI(e,t.itemsByLinkedGroupId);return{items:e,itemsByTrackId:BI(e,t.itemsByTrackId),itemById:KI(e,t.itemById),itemsByLinkedGroupId:r,linkedItemsByItemId:GI(e,r,t.linkedItemsByItemId),maxItemEndFrame:WI(e)}}function ls(e){const t=q.getState().transitions,r=new Set;for(const n of t)n.leftClipId===e&&r.add(n.rightClipId),n.rightClipId===e&&r.add(n.leftClipId);return r}function jI(e,t){const r=new Map;for(const a of e){let i=0;for(const o of t)o.trackId===a.trackId&&o.from+o.durationInFrames<=a.from&&(i+=o.durationInFrames);r.set(a.id,i)}const n=new Set;for(const a of e){if(n.has(a.id))continue;const i=sa(e,a.id);for(const s of i)n.add(s.id);if(i.length<=1)continue;let o=0;for(const s of i)o=Math.max(o,r.get(s.id)??0);if(!(o<=0))for(const s of i)r.set(s.id,o)}return r}function Ac(){return Oe("ItemsStore")}function Wp(){const e=Us(),t=e?ae.getState().getComposition(e):void 0,r=Pt.getState().currentProject?.metadata;return{width:t?.width??r?.width??ad,height:t?.height??r?.height??id,fps:W.getState().fps}}function oo(e){return{...aa(e,Wp(),Zr(e)),...e.transform}}function $I(e){const t=Wp(),r=Zr(e);return CI(r?.width??t.width,r?.height??t.height,t.width,t.height)}function wa(e,t,r){const n=e.items.map(a=>a.id!==t||a.type==="audio"?a:{...a,effects:r(a.effects||[])});return pe(n,e)}const I=Ie()((e,t)=>({items:[],itemsByTrackId:{},itemById:{},itemsByLinkedGroupId:{},linkedItemsByItemId:{},maxItemEndFrame:0,mediaDependencyIds:[],mediaDependencyVersion:0,tracks:[],setItems:r=>e(n=>{const a=r.map(i=>fe(i));return pe(a,n)}),setTracks:r=>e(n=>{const a=new Map(n.tracks.map(l=>[l.id,l])),i=tm(r).map(l=>{const c=a.get(l.id),u=c===l?c:lI(l),d=Lg(u.id);return u.height===d?u:{...u,height:d}}).sort((l,c)=>(l.order??0)-(c.order??0)),o=wg(i);return{tracks:o.length===n.tracks.length&&o.every((l,c)=>l===n.tracks[c])?n.tracks:o}}),_addItem:r=>e(n=>{const a=[...n.items,fe(r)];return pe(a,n)}),_addItems:r=>e(n=>{const a=[...n.items,...r.map(i=>fe(i))];return pe(a,n)}),_updateItem:(r,n)=>{const a=Ic(n);return e(i=>{const o=i.items.map(s=>s.id===r?fe({...s,...a}):s);return pe(o,i)})},_removeItems:r=>e(n=>{const a=new Set(r),i=n.items.filter(o=>!a.has(o.id));return pe(i,n)}),_rippleDeleteItems:r=>e(n=>{const a=new Set(r),i=n.items.filter(c=>a.has(c.id));if(i.length===0)return n;const o=n.items.filter(c=>!a.has(c.id)),s=jI(o,i),l=o.map(c=>{const u=s.get(c.id)??0;return u>0?fe({...c,from:c.from-u}):c});return pe(l,n)}),_closeGapAtPosition:(r,n)=>e(a=>{const i=Pe(n),o=a.items.filter(d=>d.trackId===r).sort((d,m)=>d.from-m.from);if(o.length===0)return a;let s=0,l=0;for(const d of o){if(i>=s&&i<d.from){l=d.from;break}s=d.from+d.durationInFrames}if(l<=s)return a;const c=l-s,u=a.items.map(d=>d.trackId===r&&d.from>=l?fe({...d,from:d.from-c}):d);return pe(u,a)}),_moveItem:(r,n,a)=>{const i=Pe(n);return e(o=>{const s=o.items.map(l=>l.id===r?fe({...l,from:i,...a&&{trackId:a}}):l);return pe(s,o)})},_moveItems:r=>e(n=>{const a=new Map(r.map(o=>[o.id,{...o,from:Pe(o.from)}])),i=n.items.map(o=>{const s=a.get(o.id);return s?fe({...o,from:s.from,...s.trackId&&{trackId:s.trackId}}):o});return pe(i,n)}),_duplicateItems:(r,n)=>{const a=t(),i=new Map(a.items.map(m=>[m.id,m])),o=[],s=Us(),l=ae.getState().compositionById,c=new Map,u=new Map;for(let m=0;m<r.length;m++){const p=i.get(r[m]),f=n[m];if(!p||!f||s!==null&&Vd(p)&&jd({parentCompositionId:s,insertedCompositionId:p.compositionId,compositionById:l}))continue;const h={...p,id:$(),from:Pe(f.from),trackId:f.trackId,originId:$(),linkedGroupId:p.linkedGroupId?c.get(p.linkedGroupId)??c.set(p.linkedGroupId,$()).get(p.linkedGroupId):void 0};o.push(fe(h)),u.set(p.id,h.id)}const d=o.map(m=>{const p=m.transformParent?.parentItemId,f=p?u.get(p):void 0;return f?{...m,transformParent:{...m.transformParent,parentItemId:f}}:m});return e(m=>{const p=[...m.items,...d];return pe(p,m)}),d},_trimItemStart:(r,n,a)=>e(i=>{const o=i.items.map(s=>{if(s.id!==r)return s;const l=W.getState().fps;let{clampedAmount:c}=Nn(s,"start",n,l);if(!a?.skipAdjacentClamp){const h=ls(r);c=qn(s,"start",c,i.items,h)}const u=s.from+c,d=s.durationInFrames-c;if(d<=0)return s;const m=Xl(s,"start",c,d,l),p=s.type==="subtitle"?cI(s,c,l):null,f=(s.type==="text"||s.type==="subtitle")&&s.words?{words:Kr(s.words,c,c+d)}:null;return{...s,from:Pe(u),durationInFrames:nr(d),...m,...p??{},...f??{}}});return pe(o,i)}),_trimItemEnd:(r,n,a)=>e(i=>{const o=i.items.map(s=>{if(s.id!==r)return s;const l=W.getState().fps;let{clampedAmount:c}=Nn(s,"end",n,l);if(!a?.skipAdjacentClamp){const f=ls(r);c=qn(s,"end",c,i.items,f)}const u=s.durationInFrames+c;if(u<=0)return s;const d=Xl(s,"end",c,u,l),m=s.type==="subtitle"?uI(s,u,l):null,p=(s.type==="text"||s.type==="subtitle")&&s.words?{words:Kr(s.words,0,u)}:null;return{...s,durationInFrames:nr(u),...d,...m??{},...p??{}}});return pe(o,i)}),_splitItem:(r,n)=>{const i=t().items.find(h=>h.id===r);if(!i)return null;const o=Pe(n),s=Pe(i.from),l=nr(i.durationInFrames),c=s+l;if(o<=s||o>=c)return null;const u=o-s,d=c-o,m=i.originId??i.id,p={...i,from:s,originId:m,durationInFrames:u},f={...i,id:$(),originId:m,from:o,durationInFrames:d};if(i.type==="subtitle"){const h=W.getState().fps,g=u/h,v=[],S=[];for(const b of i.cues){const x=b.startSeconds<g,T=b.endSeconds>g;x&&!T?v.push(b):x?(v.push({...b,endSeconds:g}),S.push({...b,id:`${b.id}-r`,startSeconds:0,endSeconds:b.endSeconds-g})):S.push({...b,startSeconds:b.startSeconds-g,endSeconds:b.endSeconds-g})}p.cues=v,f.cues=S}if((i.type==="text"||i.type==="subtitle")&&i.words){const[h,g]=ug(i.words,u,l);p.words=h,f.words=g}if(qt(i)&&!(i.type==="video"&&i.loop===!0)){const h=W.getState().fps,{sourceStart:g,speed:v,sourceFps:S}=na(i),x=yg(g,u,d,v,h,S??h);if(i.isReversed===!0){const T=x.right.sourceEnd-g,C=x.left.sourceEnd-g,k=T-C,_=g+k;if(p.sourceStart=_,p.sourceEnd=x.right.sourceEnd,f.sourceStart=g,f.sourceEnd=_,i.reverseConformPreviewIsSourceLevel===!0)p.reverseConformLocalStart=void 0,f.reverseConformLocalStart=void 0;else{const P=i.reverseConformLocalStart??0;p.reverseConformLocalStart=P,f.reverseConformLocalStart=P+u}}else p.sourceStart=g,p.sourceEnd=x.left.sourceEnd,f.sourceStart=x.right.sourceStart,f.sourceEnd=x.right.sourceEnd;Ac().debug(`_splitItem: Original sourceStart:${g} speed:${v} leftDuration:${u} rightDuration:${d} reversed:${i.isReversed===!0}`),Ac().debug(`_splitItem: leftItem.sourceStart:${p.sourceStart} leftItem.sourceEnd:${p.sourceEnd} rightItem.sourceStart:${f.sourceStart} rightItem.sourceEnd:${f.sourceEnd}`)}return e(h=>{const g=h.items.map(v=>v.id===r?fe(p):v).concat(fe(f));return pe(g,h)}),{leftItem:fe(p),rightItem:fe(f)}},_joinItems:r=>e(n=>{if(r.length<2)return n;const a=n.items.filter(p=>r.includes(p.id)).sort((p,f)=>p.from-f.from);if(a.length<2)return n;const i=a[0],o=a[a.length-1],s=a.every(p=>p.type===i.type),l=a.every(p=>p.trackId===i.trackId);if(!s||!l)return n;const c=o.from+o.durationInFrames-i.from,u={...i,from:Pe(i.from),durationInFrames:nr(c),sourceEnd:o.sourceEnd,trimEnd:o.trimEnd},d=new Set(a.slice(1).map(p=>p.id)),m=n.items.filter(p=>!d.has(p.id)).map(p=>p.id===i.id?fe(u):p);return pe(m,n)}),_rateStretchItem:(r,n,a,i)=>e(o=>{const s=o.items.map(l=>{if(l.id!==r)return l;const c=l.type==="image"&&l.label?.toLowerCase().endsWith(".gif");if(!qt(l)&&!c)return l;const u=qt(l)&&l.sourceEnd!==void 0,d=l.sourceStart??0,m=W.getState().fps,p=l.sourceFps??m,f=nr(a);let h=i;if(u){const x=Math.max(1,(l.sourceEnd??d)-d);h=gg(hg(x,f,p,m))}const g=we(f,h,m,p),v=d+g,S=l.sourceDuration?Math.min(v,l.sourceDuration):v,b={...l,from:Pe(n),durationInFrames:f,speed:h};return u||(b.sourceEnd=Pe(S)),b});return pe(s,o)}),_updateItemTransform:(r,n)=>e(a=>{const i=a.items.map(o=>o.id!==r||o.type==="audio"?o:{...o,transform:{...oo(o),...n}});return pe(i,a)}),_resetItemTransform:r=>e(n=>{const a=n.items.map(i=>i.id!==r||i.type==="audio"?i:{...i,transform:$I(i)});return pe(a,n)}),_updateItemsTransform:(r,n)=>e(a=>{const i=new Set(r),o=a.items.map(s=>!i.has(s.id)||s.type==="audio"?s:{...s,transform:{...oo(s),...n}});return pe(o,a)}),_updateItemsTransformMap:(r,n)=>e(a=>{const i=a.items.map(o=>{const s=r.get(o.id),l=n?.get(o.id);if(!s&&!l)return o;const c=l?fe({...o,...Ic(l)}):o;return!s||o.type==="audio"?c:fe({...c,transform:{...oo(c),...s}})});return pe(i,a)}),_addEffect:(r,n)=>e(a=>wa(a,r,i=>{const o={id:$(),effect:n,enabled:!0};return[...i,o]})),_addEffects:r=>e(n=>{const a=new Map(r.map(o=>[o.itemId,o.effects])),i=n.items.map(o=>{const s=a.get(o.id);if(!s||o.type==="audio")return o;const l=o.effects||[],c=s.map(u=>({id:$(),effect:u,enabled:!0}));return{...o,effects:[...l,...c]}});return pe(i,n)}),_updateEffect:(r,n,a)=>e(i=>wa(i,r,o=>o.map(s=>s.id===n?{...s,...a.effect&&{effect:a.effect},...a.enabled!==void 0&&{enabled:a.enabled}}:s))),_removeEffect:(r,n)=>e(a=>wa(a,r,i=>i.filter(o=>o.id!==n))),_toggleEffect:(r,n)=>e(a=>wa(a,r,i=>i.map(o=>o.id===n?{...o,enabled:!o.enabled}:o))),_setItemEffects:r=>e(n=>{const a=new Map(r.map(o=>[o.itemId,o.effects])),i=n.items.map(o=>{const s=a.get(o.id);return!s||o.type==="audio"?o:{...o,effects:s}});return pe(i,n)})}));let _c=I.getState().items,cs=I.getState().mediaDependencyIds,Oc=Vp(cs);I.subscribe(e=>{if(e.items===_c)return;_c=e.items;const t=VI(e.items),r=Vp(t);r!==Oc&&(cs=t,Oc=r,I.setState({mediaDependencyIds:cs,mediaDependencyVersion:e.mediaDependencyVersion+1}))});function jp(e,t){const r=$e.getState(),n=dm({inPoint:r.inPoint,outPoint:r.outPoint,maxFrame:um(e,t)});n.inPoint===r.inPoint&&n.outPoint===r.outPoint||$e.setState({inPoint:n.inPoint,outPoint:n.outPoint})}function YI(e){const t=rt.getState(),r=t.selectedItemIds.filter(s=>e.itemById[s]!==void 0);if(r.length!==t.selectedItemIds.length&&t.selectItems(r),t.selectedTrackIds.length===0&&t.activeTrackId===null)return;const n=new Set(e.tracks.map(s=>s.id)),a=t.selectedTrackIds.filter(s=>n.has(s)),i=t.activeTrackId===null||n.has(t.activeTrackId);if(a.length===t.selectedTrackIds.length&&i)return;const o=i?t.activeTrackId:a[0]??null;rt.setState({selectedTrackIds:a,selectedTrackId:a[0]??null,activeTrackId:o,selectionType:t.selectedItemIds.length>0?t.selectionType:a.length>0?"track":null})}let Dc=I.getState().items,Rc=I.getState().tracks;I.subscribe(e=>{e.items===Dc&&e.tracks===Rc||(Dc=e.items,Rc=e.tracks,YI(e))});let Lc=I.getState().maxItemEndFrame;I.subscribe(e=>{e.maxItemEndFrame!==Lc&&(Lc=e.maxItemEndFrame,jp(e.items,W.getState().fps))});W.subscribe((e,t)=>{e.fps!==t.fps&&jp(I.getState().items,e.fps)});function $p(e){const t={};for(const r of e)t[r.itemId]=r;return t}function so(e,t=new Set){const r=new Map;for(const n of e){const a=r.get(n.frame);if(!a){r.set(n.frame,n);continue}const i=t.has(a.id);(t.has(n.id)||!i)&&r.set(n.frame,n)}return Array.from(r.values()).sort((n,a)=>n.frame-a.frame)}function Qe(e){return e.properties.some(r=>r.keyframes.length>0)||e.vectorProperties?.some(r=>r.keyframes.length>0)?!0:[e.separatedVectorProperties,e.propertyLinks,e.expressions].some(r=>r!==void 0&&r.length>0)}function Bc(e,t,r){const n=new Map(e.map(i=>[i.id,i.frame])),a=new Map;for(const i of e){const o={...i,frame:Math.min(r,Math.max(0,Math.round(i.frame*t)))},s=a.get(o.frame);(!s||(n.get(o.id)??-1)>(n.get(s.id)??-1))&&a.set(o.frame,o)}return Array.from(a.values()).sort((i,o)=>i.frame-o.frame)}const H=Ie()((e,t)=>({keyframes:[],keyframesByItemId:{},setKeyframes:r=>e({keyframes:r,keyframesByItemId:$p(r)}),_addKeyframe:(r,n,a,i,o="linear",s)=>{const l=$(),c={id:l,frame:a,value:i,easing:o,easingConfig:s};let u=l;return e(d=>{const m=d.keyframes.find(p=>p.itemId===r);if(m){const p=m.properties.find(f=>f.property===n);if(p){const f=p.keyframes.find(h=>h.frame===a);return f?(u=f.id,{keyframes:d.keyframes.map(h=>h.itemId===r?{...h,animationVersion:be,properties:h.properties.map(g=>g.property===n?{...g,keyframes:g.keyframes.map(v=>v.frame===a?{...v,value:i,easing:o,easingConfig:s}:v)}:g)}:h)}):{keyframes:d.keyframes.map(h=>h.itemId===r?{...h,animationVersion:be,properties:h.properties.map(g=>g.property===n?{...g,keyframes:[...g.keyframes,c].sort((v,S)=>v.frame-S.frame)}:g)}:h)}}return{keyframes:d.keyframes.map(f=>f.itemId===r?{...f,animationVersion:be,properties:[...f.properties,{property:n,keyframes:[c]}]}:f)}}return{keyframes:[...d.keyframes,{itemId:r,animationVersion:be,properties:[{property:n,keyframes:[c]}]}]}}),u},_addKeyframes:r=>{if(r.length===0)return[];const n=[];return e(a=>{let i=[...a.keyframes];for(const o of r){const{itemId:s,property:l,frame:c,value:u,easing:d="linear",easingConfig:m,source:p}=o,f=$(),h={id:f,frame:c,value:u,easing:d,easingConfig:m,source:p},g=i.findIndex(v=>v.itemId===s);if(g!==-1){const v=i[g],S=v.properties.findIndex(b=>b.property===l);if(S!==-1){const b=v.properties[S],x=b.keyframes.findIndex(T=>T.frame===c);if(x!==-1){const T=[...b.keyframes];T[x]={...T[x],value:u,easing:d,easingConfig:m},n.push(T[x].id);const C=[...v.properties];C[S]={...b,keyframes:T},i=i.map((k,_)=>_===g?{...v,animationVersion:be,properties:C}:k)}else{const T=[...b.keyframes,h].sort((k,_)=>k.frame-_.frame);n.push(f);const C=[...v.properties];C[S]={...b,keyframes:T},i=i.map((k,_)=>_===g?{...v,animationVersion:be,properties:C}:k)}}else{const b=[...v.properties,{property:l,keyframes:[h]}];n.push(f),i=i.map((x,T)=>T===g?{...v,animationVersion:be,properties:b}:x)}}else n.push(f),i=[...i,{itemId:s,animationVersion:be,properties:[{property:l,keyframes:[h]}]}]}return{keyframes:i}}),n},_updateKeyframe:(r,n,a,i)=>e(o=>({keyframes:o.keyframes.map(s=>s.itemId===r?{...s,properties:s.properties.map(l=>l.property===n?{...l,keyframes:so(l.keyframes.map(c=>c.id===a?{...c,...i}:c),new Set([a]))}:l)}:s)})),_removeKeyframe:(r,n,a)=>e(i=>({keyframes:i.keyframes.map(o=>o.itemId===r?{...o,properties:o.properties.map(s=>s.property===n?{...s,keyframes:s.keyframes.filter(l=>l.id!==a)}:s).filter(s=>s.keyframes.length>0)}:o).filter(Qe)})),_removeKeyframesForItem:r=>e(n=>({keyframes:n.keyframes.map(a=>a.itemId===r?{...a,properties:[],...a.vectorProperties&&{vectorProperties:[]}}:a).filter(Qe)})),_removeKeyframesForItems:r=>e(n=>{const a=new Set(r);return{keyframes:n.keyframes.filter(i=>!a.has(i.itemId)).map(i=>({...i,propertyLinks:i.propertyLinks?.filter(o=>!a.has(o.sourceItemId)),expressions:i.expressions?.filter(o=>o.type!=="link"||!a.has(o.sourceItemId))})).filter(Qe)}}),_removeKeyframesForProperty:(r,n)=>e(a=>({keyframes:a.keyframes.map(i=>i.itemId===r?{...i,properties:i.properties.filter(o=>o.property!==n)}:i).filter(Qe)})),_removeVectorKeyframesForProperty:(r,n)=>e(a=>({keyframes:a.keyframes.map(i=>i.itemId===r?{...i,vectorProperties:i.vectorProperties?.filter(o=>o.property!==n)}:i).filter(Qe)})),_removeKeyframesByApplication:(r,n)=>e(a=>({keyframes:a.keyframes.map(i=>i.itemId===r?{...i,properties:i.properties.map(o=>({...o,keyframes:o.keyframes.filter(s=>s.source?.applicationId!==n)})).filter(o=>o.keyframes.length>0),vectorProperties:i.vectorProperties?.map(o=>({...o,keyframes:o.keyframes.filter(s=>s.source?.applicationId!==n)})).filter(o=>o.keyframes.length>0)}:i).filter(Qe)})),_removeManualKeyframes:r=>e(n=>({keyframes:n.keyframes.map(a=>a.itemId===r?{...a,properties:a.properties.map(i=>({...i,keyframes:i.keyframes.filter(o=>o.source)})).filter(i=>i.keyframes.length>0),vectorProperties:a.vectorProperties?.map(i=>({...i,keyframes:i.keyframes.filter(o=>o.source)})).filter(i=>i.keyframes.length>0)}:a).filter(Qe)})),_setDirectPropertyLink:(r,n)=>e(a=>a.keyframes.find(o=>o.itemId===r)?{keyframes:a.keyframes.map(o=>o.itemId===r?{...o,animationVersion:be,propertyLinks:[...mt(o).filter(s=>!zy(s.targetProperty,n.targetProperty)),n],expressions:[...Ar(o)]}:o)}:{keyframes:[...a.keyframes,{itemId:r,animationVersion:be,properties:[],propertyLinks:[n]}]}),_removeDirectPropertyLink:(r,n)=>e(a=>({keyframes:a.keyframes.map(i=>i.itemId===r?{...i,propertyLinks:mt(i).filter(o=>o.targetProperty!==n),expressions:[...Ar(i)]}:i).filter(Qe)})),_setPropertyExpression:(r,n)=>e(a=>a.keyframes.find(o=>o.itemId===r)?{keyframes:a.keyframes.map(o=>o.itemId===r?{...o,animationVersion:be,propertyLinks:[...mt(o)],expressions:[...Ar(o).filter(s=>s.targetProperty!==n.targetProperty),n]}:o)}:{keyframes:[...a.keyframes,{itemId:r,animationVersion:be,properties:[],expressions:[n]}]}),_removePropertyExpression:(r,n)=>e(a=>({keyframes:a.keyframes.map(i=>i.itemId===r?{...i,propertyLinks:[...mt(i)],expressions:Ar(i).filter(o=>o.targetProperty!==n)}:i).filter(Qe)})),_upsertVectorKeyframe:(r,n,a)=>{const i=$();let o=i;const s={id:i,frame:a.frame,value:a.value,easing:a.easing??"linear",easingConfig:a.easingConfig,temporalEase:a.temporalEase,spatial:a.spatial,source:a.source};return e(l=>{const c=l.keyframes.find(m=>m.itemId===r);if(!c)return{keyframes:[...l.keyframes,{itemId:r,animationVersion:be,properties:[],vectorProperties:[{property:n,keyframes:[s]}]}]};const u=c.vectorProperties?.find(m=>m.property===n),d=u?.keyframes.find(m=>m.frame===a.frame);return d&&(o=d.id),{keyframes:l.keyframes.map(m=>{if(m.itemId!==r)return m;const p=m.vectorProperties??[];return u?{...m,animationVersion:be,vectorProperties:p.map(f=>f.property===n?{...f,keyframes:d?f.keyframes.map(h=>h.frame===a.frame?{...s,id:h.id}:h):[...f.keyframes,s].sort((h,g)=>h.frame-g.frame)}:f)}:{...m,animationVersion:be,vectorProperties:[...p,{property:n,keyframes:[s]}]}})}}),o},_updateVectorKeyframe:(r,n,a,i)=>e(o=>({keyframes:o.keyframes.map(s=>s.itemId===r?{...s,animationVersion:be,vectorProperties:s.vectorProperties?.map(l=>l.property===n?{...l,keyframes:so(l.keyframes.map(c=>c.id===a?{...c,...i}:c),new Set([a]))}:l)}:s)})),_removeVectorKeyframe:(r,n,a)=>e(i=>({keyframes:i.keyframes.map(o=>o.itemId===r?{...o,vectorProperties:o.vectorProperties?.map(s=>s.property===n?{...s,keyframes:s.keyframes.filter(l=>l.id!==a)}:s).filter(s=>s.keyframes.length>0)}:o).filter(Qe)})),_replaceScalarPropertiesWithVectorProperty:(r,n,a)=>e(i=>{const o=new Set(a);return i.keyframes.find(l=>l.itemId===r)?{keyframes:i.keyframes.map(l=>l.itemId===r?{...l,animationVersion:be,properties:l.properties.filter(c=>!o.has(c.property)),vectorProperties:[...(l.vectorProperties??[]).filter(c=>c.property!==n.property),n]}:l)}:{keyframes:[...i.keyframes,{itemId:r,animationVersion:be,properties:[],vectorProperties:[n]}]}}),_setVectorDimensionsSeparated:(r,n,a)=>e(i=>{const o=new Set(By(n)),s=c=>{const u=new Set(c.separatedVectorProperties??[]);a.separated?u.add(n):u.delete(n);const d=a.separated?(a.scalarProperties??[]).map(p=>({...p,keyframes:p.keyframes.map(f=>({...f}))})):[],m=(c.vectorProperties??[]).filter(p=>p.property!==n);return!a.separated&&a.vectorProperty&&m.push(a.vectorProperty),{...c,animationVersion:be,properties:[...c.properties.filter(p=>!o.has(p.property)),...d],vectorProperties:m,separatedVectorProperties:Array.from(u)}};return i.keyframes.find(c=>c.itemId===r)?{keyframes:i.keyframes.map(c=>c.itemId===r?s(c):c)}:{keyframes:[...i.keyframes,s({itemId:r,animationVersion:be,properties:[]})]}}),_scaleKeyframesForItem:(r,n,a)=>{if(n===a||n<=0||a<=0)return;const i=a/n,o=a-1;e(s=>s.keyframes.find(c=>c.itemId===r)?{keyframes:s.keyframes.map(c=>c.itemId!==r?c:{...c,properties:c.properties.map(u=>u.keyframes.length===0?u:{...u,keyframes:Bc(u.keyframes,i,o)}),vectorProperties:c.vectorProperties?.map(u=>({...u,keyframes:Bc(u.keyframes,i,o)}))})}:s)},_updateKeyframes:r=>{if(r.length===0)return;const n=new Map;for(const a of r){const i=`${a.itemId}:${a.property}`;n.has(i)||n.set(i,new Set),n.get(i).add(a.keyframeId)}e(a=>{let i=[...a.keyframes];for(const o of r)i=i.map(s=>s.itemId===o.itemId?{...s,properties:s.properties.map(l=>l.property===o.property?{...l,keyframes:so(l.keyframes.map(c=>c.id===o.keyframeId?{...c,...o.updates}:c),n.get(`${o.itemId}:${o.property}`)??new Set)}:l)}:s);return{keyframes:i}})},_moveKeyframes:r=>{if(r.length===0)return;const n=r.map(a=>({itemId:a.ref.itemId,property:a.ref.property,keyframeId:a.ref.keyframeId,updates:{frame:Math.max(0,a.newFrame)}}));t()._updateKeyframes(n)},_removeKeyframes:r=>{if(r.length===0)return;const n=new Map;for(const a of r){const i=`${a.itemId}:${a.property}`;n.has(i)||n.set(i,new Set),n.get(i).add(a.keyframeId)}e(a=>({keyframes:a.keyframes.map(i=>({...i,properties:i.properties.map(o=>{const s=n.get(`${i.itemId}:${o.property}`);return s?{...o,keyframes:o.keyframes.filter(l=>!s.has(l.id))}:o}).filter(o=>o.keyframes.length>0)})).filter(Qe)}))},_duplicateKeyframes:(r,n,a,i)=>{if(r.length===0)return[];const o=[],s=t(),l=[];for(const c of r){const u=s.keyframes.find(p=>p.itemId===c.itemId);if(!u)continue;const d=u.properties.find(p=>p.property===c.property);if(!d)continue;const m=d.keyframes.find(p=>p.id===c.keyframeId);m&&l.push({property:i??c.property,keyframe:m,targetItemId:a??c.itemId})}for(const{property:c,keyframe:u,targetItemId:d}of l){const m=s._addKeyframe(d,c,u.frame+n,u.value,u.easing,u.easingConfig);o.push(m)}return o},getKeyframesForItem:r=>t().keyframesByItemId[r],getKeyframeById:(r,n,a)=>{const i=t().keyframesByItemId[r];if(!i)return;const o=i.properties.find(s=>s.property===n);if(o)return o.keyframes.find(s=>s.id===a)},getAllKeyframesForProperty:(r,n)=>{const a=t().keyframesByItemId[r];if(!a)return[];const i=a.properties.find(o=>o.property===n);return i?i.keyframes:[]},hasKeyframesAtFrame:(r,n,a)=>{const i=t().keyframesByItemId[r];if(!i)return!1;const o=i.properties.find(s=>s.property===n);return o?o.keyframes.some(s=>s.frame===a):!1},getVectorKeyframesForProperty:(r,n)=>t().keyframesByItemId[r]?.vectorProperties?.find(a=>a.property===n)?.keyframes??[]}));H.subscribe((e,t)=>{e.keyframes!==t.keyframes&&e.keyframesByItemId===t.keyframesByItemId&&H.setState({keyframesByItemId:$p(e.keyframes)})});function XI(e){return[...new Set(e)]}const ir=Ie()((e,t)=>({topLevelSequenceIds:[],sequenceViewById:{},setTopLevelSequenceIds:r=>e({topLevelSequenceIds:XI(r)}),addTopLevelSequence:r=>e(n=>n.topLevelSequenceIds.includes(r)?n:{topLevelSequenceIds:[...n.topLevelSequenceIds,r]}),removeTopLevelSequence:r=>e(n=>{if(!n.topLevelSequenceIds.includes(r))return n;const{[r]:a,...i}=n.sequenceViewById;return{topLevelSequenceIds:n.topLevelSequenceIds.filter(o=>o!==r),sequenceViewById:i}}),reorderTopLevelSequences:(r,n)=>e(a=>{const i=a.topLevelSequenceIds;if(r<0||r>=i.length||n<0||n>=i.length||r===n)return a;const o=[...i],[s]=o.splice(r,1);return o.splice(n,0,s),{topLevelSequenceIds:o}}),isTopLevelSequence:r=>t().topLevelSequenceIds.includes(r),pruneToValidSequenceIds:r=>e(n=>{const a=new Set(r),i=n.topLevelSequenceIds.filter(o=>a.has(o));if(i.length===n.topLevelSequenceIds.length){const o=zc(n.sequenceViewById,a);return o===n.sequenceViewById?n:{sequenceViewById:o}}return{topLevelSequenceIds:i,sequenceViewById:zc(n.sequenceViewById,a)}}),saveSequenceView:(r,n)=>e(a=>({sequenceViewById:{...a.sequenceViewById,[r]:n}})),getSequenceView:r=>t().sequenceViewById[r],removeSequenceView:r=>e(n=>{if(!(r in n.sequenceViewById))return n;const{[r]:a,...i}=n.sequenceViewById;return{sequenceViewById:i}}),reset:()=>e({topLevelSequenceIds:[],sequenceViewById:{}})}));function zc(e,t){const r=Object.entries(e).filter(([n])=>t.has(n));return r.length===Object.keys(e).length?e:Object.fromEntries(r)}const QI="__root__";function us(e,t){const r={...e,...t};if(e.entities||t.entities){r.entities={...e.entities};for(const[n,a]of Object.entries(t.entities??{})){const i=n,o=r.entities[i];r.entities[i]={...o,...a,values:{...o?.values,...a.values}}}}return r}class JI{contexts=new Map;stacks(t){let r=this.contexts.get(t);return r||(r={undo:[],redo:[]},this.contexts.set(t,r)),r}record(t){const r=this.stacks(t.contextKey),n=r.undo.at(-1);if(t.mergeKey&&n?.mergeKey===t.mergeKey)if(n.commands=t.commands,n.redoPatch=us(n.redoPatch,t.redoPatch),n.inverses.every(a=>a.kind==="patch")&&t.inverses.every(a=>a.kind==="patch")){let a={};for(const i of[...t.inverses,...n.inverses].reverse())i.kind==="patch"&&(a=us(i.patch,a));n.inverses=[{kind:"patch",patch:a}]}else n.inverses=[...t.inverses,...n.inverses];else r.undo.push(t);r.redo.length=0}clear(){this.contexts.clear()}removeContext(t){this.contexts.delete(t)}}function ZI(e){return e[0]?.compositionId??null}function ka(e){return e??QI}function ex(){return{currentFrame:me.getState().currentFrame,zoomLevel:mr.getState().level,scrollPosition:W.getState().scrollPosition,selectedItemIds:rt.getState().selectedItemIds}}function lo(e){e?(mr.getState().setZoomLevel(e.zoomLevel),W.getState().setScrollPosition(e.scrollPosition),me.getState().setCurrentFrame(e.currentFrame),rt.getState().selectItems(e.selectedItemIds)):(mr.getState().setZoomLevel(1),W.getState().setScrollPosition(0))}function co(e){const t=$e.getState();return{compositionId:e,items:I.getState().items,tracks:I.getState().tracks,transitions:q.getState().transitions,keyframes:H.getState().keyframes,currentFrame:me.getState().currentFrame,zoomLevel:mr.getState().level,scrollPosition:W.getState().scrollPosition,busAudioEq:me.getState().busAudioEq,markers:t.markers,inPoint:t.inPoint,outPoint:t.outPoint}}function uo(e){I.getState().setItems(e.items),I.getState().setTracks(e.tracks),q.getState().setTransitions(e.transitions),H.getState().setKeyframes(e.keyframes),rt.getState().clearSelection(),me.getState().setCurrentFrame(e.currentFrame),me.getState().setBusAudioEq(e.busAudioEq),$e.getState().setMarkers(e.markers),$e.getState().setInOutPoints(e.inPoint,e.outPoint)}function wn(e){const t=I.getState().items,r=ae.getState().getComposition(e),n=t.length>0?Math.max(...t.map(o=>o.from+o.durationInFrames)):0,a=r?.editorKind==="composite-2d"?Math.max(1,r.durationInFrames):n,i=$e.getState();ae.getState().updateComposition(e,{items:t,tracks:I.getState().tracks,transitions:q.getState().transitions,keyframes:H.getState().keyframes,durationInFrames:a,busAudioEq:me.getState().busAudioEq,markers:i.markers,inPoint:i.inPoint,outPoint:i.outPoint})}function mo(e){const t=ae.getState().getComposition(e);return t?(I.getState().setItems(t.items),I.getState().setTracks(t.tracks),q.getState().setTransitions(t.transitions??[]),H.getState().setKeyframes(t.keyframes??[]),rt.getState().clearSelection(),me.getState().setBusAudioEq(t.busAudioEq),$e.getState().setMarkers(t.markers??[]),$e.getState().setInOutPoints(t.inPoint??null,t.outPoint??null),!0):!1}function tx(e,t,r){if(r){const a=e.find(i=>i.id===r&&i.compositionId===t&&(i.type==="composition"||i.type==="audio"));if(a)return a}const n=e.find(a=>a.type==="composition"&&a.compositionId===t);return n||(e.find(a=>a.type==="audio"&&a.compositionId===t)??null)}const rx=16,de=Ie()((e,t)=>({breadcrumbs:[{compositionId:null,label:"Main Timeline"}],activeCompositionId:null,stashStack:[],mainHolder:null,enterComposition:(r,n,a)=>{me.getState().pause();const i=t();if(i.breadcrumbs.length>=rx||i.activeCompositionId===r||i.breadcrumbs.some(u=>u.compositionId===r))return;i.activeCompositionId!==null&&wn(i.activeCompositionId);const o=co(i.activeCompositionId);if(!mo(r))return;const s=me.getState().currentFrame,l=tx(o.items,r,a);let c=0;if(l){const u=s-l.from;u>=0&&u<l.durationInFrames&&(c=u)}me.getState().setCurrentFrame(c),Dt(r),e({breadcrumbs:[...i.breadcrumbs,{compositionId:r,label:n,...l?.id&&{entryItemId:l.id}}],activeCompositionId:r,stashStack:[...i.stashStack,o]})},exitComposition:()=>{me.getState().pause();const r=t();if(r.breadcrumbs.length<=1||r.stashStack.length===0)return;r.activeCompositionId!==null&&wn(r.activeCompositionId);const n=r.stashStack[r.stashStack.length-1];uo(n);const a=r.breadcrumbs.slice(0,-1),i=a[a.length-1];Dt(i.compositionId),e({breadcrumbs:a,activeCompositionId:i.compositionId,stashStack:r.stashStack.slice(0,-1)})},navigateTo:r=>{me.getState().pause();const n=t();if(r<0||r>=n.breadcrumbs.length||r===n.breadcrumbs.length-1)return;n.activeCompositionId!==null&&wn(n.activeCompositionId);const a=n.breadcrumbs.length-1-r,i=n.stashStack[n.stashStack.length-a];i&&uo(i);const o=n.breadcrumbs.slice(0,r+1),s=o[o.length-1];Dt(s.compositionId),e({breadcrumbs:o,activeCompositionId:s.compositionId,stashStack:n.stashStack.slice(0,n.stashStack.length-a)})},resetToRoot:()=>{for(me.getState().pause();t().breadcrumbs.length>1;)t().exitComposition();const r=t();r.mainHolder&&(r.activeCompositionId!==null&&wn(r.activeCompositionId),uo(r.mainHolder)),Dt(null),e({breadcrumbs:[{compositionId:null,label:"Main Timeline"}],activeCompositionId:null,stashStack:[],mainHolder:null})},switchToSequence:r=>{me.getState().pause();const n=t(),a=ZI(n.breadcrumbs);if(a===r){if(r!==null&&n.mainHolder===null){const i=ae.getState().getComposition(r);if(!i)return;const o=co(null);mo(r),me.getState().setCurrentFrame(0),Dt(r),e({breadcrumbs:[{compositionId:r,label:i.name}],activeCompositionId:r,stashStack:[],mainHolder:o}),lo(ir.getState().getSequenceView(ka(r)));return}n.breadcrumbs.length>1&&t().navigateTo(0);return}if(ir.getState().saveSequenceView(ka(a),ex()),t().resetToRoot(),r!==null){const i=ae.getState().getComposition(r);if(!i){lo(ir.getState().getSequenceView(ka(null)));return}const o=co(null);mo(r),me.getState().setCurrentFrame(0),Dt(r),e({breadcrumbs:[{compositionId:r,label:i.name}],activeCompositionId:r,stashStack:[],mainHolder:o})}lo(ir.getState().getSequenceView(ka(r)))}})),Yp=["items","tracks","transitions","keyframes","markers","compositions","mediaItems"];function ds(e,t){if(!e||typeof e!="object")throw new Error(`Invalid ${t} entity`);const r=Reflect.get(e,t==="keyframes"?"itemId":"id");if(typeof r!="string")throw new Error(`Missing ${t} entity id`);return r}function Le(){const e=I.getState(),t=$e.getState(),r=W.getState(),n=me.getState(),a=Pt.getState().currentProject,i=Me.getState(),o=new Set(i.importingIds);return{projectName:a?.name,mediaItems:o.size?i.mediaItems.filter(s=>!o.has(s.id)):i.mediaItems,stashStack:de.getState().stashStack,mainHolder:de.getState().mainHolder,items:e.items,tracks:e.tracks,transitions:q.getState().transitions,keyframes:H.getState().keyframes,markers:t.markers,compositions:ae.getState().compositions,topLevelSequenceIds:ir.getState().topLevelSequenceIds,inPoint:t.inPoint,outPoint:t.outPoint,fps:r.fps,scrollPosition:r.scrollPosition,snapEnabled:r.snapEnabled,currentFrame:n.currentFrame,busAudioEq:n.busAudioEq,masterBusDb:n.masterBusDb,projectId:a?.id??null,projectMetadata:a?{...a.metadata}:null}}function er(e){const t={...e};if(e.entities){const r=Le();for(const n of Yp){const a=e.entities[n];if(!a)continue;const i=new Map((r[n]??[]).map(s=>[ds(s,n),s]));for(const[s,l]of Object.entries(a.values))l===null?i.delete(s):i.set(s,l);const o=a.order??[...i.keys()];Reflect.set(t,n,o.flatMap(s=>i.has(s)?[i.get(s)]:[]))}}if("items"in t&&I.getState().setItems(t.items),"tracks"in t&&I.setState({tracks:t.tracks}),"transitions"in t&&q.getState().setTransitions(t.transitions),"keyframes"in t&&H.getState().setKeyframes(t.keyframes),"compositions"in t&&ae.getState().setCompositions(t.compositions),"topLevelSequenceIds"in t&&ir.getState().setTopLevelSequenceIds(t.topLevelSequenceIds),"stashStack"in t&&de.setState({stashStack:t.stashStack??[]}),"mainHolder"in t&&de.setState({mainHolder:t.mainHolder??null}),"markers"in t&&$e.getState().setMarkers(t.markers),"inPoint"in t&&$e.getState().setInPoint(t.inPoint),"outPoint"in t&&$e.getState().setOutPoint(t.outPoint),"fps"in t&&W.getState().setFps(t.fps),"scrollPosition"in t&&W.getState().setScrollPosition(t.scrollPosition),"snapEnabled"in t&&W.getState().setSnapEnabled(t.snapEnabled),"currentFrame"in t&&me.getState().setCurrentFrame(t.currentFrame),"busAudioEq"in t&&me.getState().setBusAudioEq(t.busAudioEq),"masterBusDb"in t&&me.getState().setMasterBusDb(t.masterBusDb),"mediaItems"in t){const r=Me.getState(),n=new Set(r.importingIds),i=[...r.mediaItems.filter(o=>n.has(o.id)),...(t.mediaItems??[]).filter(o=>!n.has(o.id))];Me.setState({mediaItems:i,mediaById:Object.fromEntries(i.map(o=>[o.id,o]))})}("projectId"in t||"projectMetadata"in t||"projectName"in t)&&Pt.setState(r=>{if("projectId"in t&&t.projectId===null)return{currentProject:null};const n=r.currentProject,a=t.projectId??n?.id??"command-project",i=t.projectMetadata??n?.metadata??{width:1920,height:1080,fps:t.fps??30},o={description:"",createdAt:0,updatedAt:0,duration:0,...n,id:a,name:t.projectName??n?.name??"Dự án",metadata:i};return{currentProject:o,projects:r.projects.map(s=>s.id===a?o:s)}})}function nx(){const e={};for(const t of Object.keys(Le()))Object.defineProperty(e,t,{enumerable:!0,get:()=>Le()[t],set:r=>er({[t]:r})});return Object.defineProperty(e,"nav",{get:()=>de.getState()}),Object.defineProperty(e,"sequenceViews",{get:()=>ir.getState().sequenceViewById}),e}function on(e,t){const r={};for(const n of Object.keys(e))if(e[n]!==t[n])if(Yp.includes(n)){const a=n,i=e[a]??[],o=t[a]??[],s=new Map(i.map(p=>[ds(p,a),p])),l=new Map(o.map(p=>[ds(p,a),p])),c={};for(const[p,f]of s){const h=l.get(p);f!==h&&JSON.stringify(f)!==JSON.stringify(h)&&(c[p]=f)}for(const p of l.keys())s.has(p)||(c[p]=null);const u=[...s.keys()],d=[...l.keys()],m=u.length!==d.length||u.some((p,f)=>p!==d[f]);(Object.keys(c).length||m)&&(r.entities??={},r.entities[a]={values:c,...m?{order:u}:{}})}else JSON.stringify(e[n])!==JSON.stringify(t[n])&&Reflect.set(r,n,e[n]);return r}function ax(e,t){const r=de.getState();if((r.activeCompositionId??"__root__")===e)return Le();r.activeCompositionId&&wn(r.activeCompositionId);const n=Le(),a=r.stashStack.find(i=>(i.compositionId??"__root__")===e)??(e==="__root__"?r.mainHolder:null);return a?{...n,items:a.items,tracks:a.tracks,transitions:a.transitions,keyframes:a.keyframes,markers:a.markers,inPoint:a.inPoint,outPoint:a.outPoint,currentFrame:a.currentFrame,busAudioEq:a.busAudioEq,stashStack:t.stashStack,mainHolder:t.mainHolder}:n}const Xp=new Map;function A(e){Xp.set(e.type,e)}function ms(e){return Xp.get(e)}const ps=Ie(()=>({canUndo:!1,canRedo:!1,revision:0}));let Pa;class ix{constructor(t=Ud()){this.ids=t}ids;history=new JI;depth=0;listeners=new Set;get contextKey(){return de.getState().activeCompositionId??"__root__"}get canUndo(){return this.history.stacks(this.contextKey).undo.length>0}get canRedo(){return this.history.stacks(this.contextKey).redo.length>0}on(t,r){return this.listeners.add(r),()=>{this.listeners.delete(r)}}executeCommands(t,r={}){const n=Le(),a=this.contextKey,i=de.getState(),o=W.getState().isDirty,s=this.ids.checkpoint?.();let l=-1;const c=[],u=t.map((g,v)=>{const S=ms(g.type);if(!S)return c.push({message:`Unknown command: ${g.type}`,commandIndex:v}),null;const b=S.params.safeParse(g.params);return b.success?{definition:S,params:b.data,command:{type:g.type,params:b.data}}:(c.push(...b.error.issues.map(x=>({message:x.message,path:x.path.join("."),commandIndex:v}))),null)});if(c.length)return{ok:!1,issues:c};const d=[],m=new Map,p=this.depth===0;this.depth++;try{Yl(this.ids,()=>{for(let g=0;g<u.length;g++){l=g;const v=u[g];if(!v)continue;const S=Le(),b=this.contextKey,x=v.definition.validate?.(S,v.params)??[];if(x.length)throw c.push(...x.map(k=>({...k,commandIndex:g}))),new Error(x.map(k=>k.message).join("; "));const T=v.definition.apply({draft:nx(),ids:this.ids,contextKey:b},v.params),C="inverse"in T?T.inverse:T;if(d.push("inverse"in T?T.result:void 0),v.definition.history!=="none"){const k=ax(b,S),_=on(S,k);if(Object.keys(_).length){let P=m.get(b);P||(P={commands:[],inverses:[],redoPatch:{},contextKey:b,mergeKey:u.length===1?Pa!==void 0?Pa:v.definition.mergeKey?.(v.params)??null:null,timestamp:Date.now()},m.set(b,P)),P.commands.push(v.command),P.inverses.unshift(C.kind==="patch"&&Object.keys(C.patch).length===0?{kind:"patch",patch:_}:C),P.redoPatch=us(P.redoPatch,on(k,S))}}}})}catch(g){return er(n),de.setState(i),Dt(i.activeCompositionId),W.getState().setIsDirty(o),this.ids.checkpoint&&this.ids.restore&&this.ids.restore(s),{ok:!1,issues:c.length?c:[{message:g instanceof Error?g.message:String(g),commandIndex:l}],error:g instanceof Error?g.message:String(g)}}finally{this.depth--}const f=Le();if(r.dryRun)return er(n),de.setState(i),Dt(i.activeCompositionId),W.getState().setIsDirty(o),this.ids.checkpoint&&this.ids.restore&&this.ids.restore(s),{ok:!0,commands:t,results:d};const h=u.flatMap(g=>g?[g.command]:[]);if(p){for(const g of m.values())this.history.record(g);this.emit({kind:"execute",commands:h,contextKey:a,before:n,after:f})}return{ok:!0,commands:h,results:d}}executeAnonymous(t,r,n){if(this.depth)return r();const a=Le(),i=this.contextKey;this.depth++;let o;try{o=Yl(this.ids,r)}catch(u){throw er(a),u}finally{this.depth--}const s=Le(),l=on(a,s),c=[{type:t,params:n??{}}];return Object.keys(l).length&&this.history.record({commands:c,inverses:[{kind:"patch",patch:l}],redoPatch:on(s,a),contextKey:i,mergeKey:Pa??null,timestamp:Date.now()}),this.emit({kind:"execute",commands:c,contextKey:i,before:a,after:s}),o}undo(){const t=this.history.stacks(this.contextKey),r=t.undo.at(-1);if(!r)return{ok:!1,issues:[{message:"Nothing to undo"}]};const n=Le();this.depth++;try{for(const a of r.inverses)if(a.kind==="patch")er(a.patch);else{const i=this.executeCommands(a.commands);if(!i.ok)throw new Error(i.issues.map(o=>o.message).join("; "))}}catch(a){return er(n),{ok:!1,issues:[{message:String(a)}]}}finally{this.depth--}return t.undo.pop(),t.redo.push(r),this.emit({kind:"undo",commands:r.commands,contextKey:r.contextKey,before:n,after:Le()}),{ok:!0,commands:r.commands,results:[]}}redo(){const t=this.history.stacks(this.contextKey),r=t.redo.pop();if(!r)return{ok:!1,issues:[{message:"Nothing to redo"}]};const n=Le();return er(r.redoPatch),t.undo.push(r),this.emit({kind:"redo",commands:r.commands,contextKey:r.contextKey,before:n,after:Le()}),{ok:!0,commands:r.commands,results:[]}}closeMergeGroup(){Pa=void 0;const t=this.history.stacks(this.contextKey).undo.at(-1);t&&(t.mergeKey=null)}clearHistory(){this.history.clear(),ps.setState(t=>({canUndo:!1,canRedo:!1,revision:t.revision+1}))}emit(t){t.commands.some(n=>{const a=ms(n.type);return a?.dirty??a?.history!=="none"})&&Object.keys(on(t.before,t.after)).length&&W.getState().markDirty(),ps.setState(n=>({canUndo:this.canUndo,canRedo:this.canRedo,revision:n.revision+1}));for(const n of this.listeners)n(t)}}let Uc,Hc=!1;function cl(){return Uc??=new ix,Hc||(Hc=!0,de.subscribe((e,t)=>{if(e.activeCompositionId===t.activeCompositionId)return;const r=cl();ps.setState(n=>({canUndo:r.canUndo,canRedo:r.canRedo,revision:n.revision+1}))})),Uc}function vr(e,t){return cl().executeCommands(e,t)}y({type:w().min(1),params:_i()}).strict();function Rn(e,t,r){return Math.max(t,Math.min(r,e))}function ox(e){if(e.length<=1)return[0];const t=e.length-1,r=new Array(t);for(let a=0;a<t;a+=1){const i=e[a],o=e[a+1],s=Math.max(1e-6,o.x-i.x);r[a]=(o.y-i.y)/s}const n=new Array(e.length);n[0]=r[0],n[e.length-1]=r[t-1];for(let a=1;a<e.length-1;a+=1){const i=r[a-1],o=r[a];n[a]=i*o<=0?0:(i+o)/2}for(let a=0;a<t;a+=1){const i=r[a];if(Math.abs(i)<1e-6){n[a]=0,n[a+1]=0;continue}const o=n[a]/i,s=n[a+1]/i,l=o*o+s*s;if(l>9){const c=3/Math.sqrt(l);n[a]=c*o*i,n[a+1]=c*s*i}}return n}function sx(e){if(!e||e.length===0)return[{x:0,y:0},{x:1,y:1}];const t=e.map(a=>({x:Rn(a.x,0,1),y:Rn(a.y,0,1)})).sort((a,i)=>a.x-i.x),r=[];for(const a of t){const i=r[r.length-1];(!i||Math.abs(i.x-a.x)>5e-4)&&r.push(a)}if(r.length===0)return[{x:0,y:0},{x:1,y:1}];r[0].x>1e-4?r.unshift({x:0,y:r[0].y}):r[0]={x:0,y:r[0].y};const n=r[r.length-1];return n.x<.9999?r.push({x:1,y:n.y}):Math.abs(n.x-1)>1e-4&&(r[r.length-1]={x:1,y:n.y}),r}function Ma(e,t){const r=sx(e),n=Rn(t,0,1);if(n<=r[0].x)return r[0].y;if(n>=r[r.length-1].x)return r[r.length-1].y;let a=r.length-2;for(let b=0;b<r.length-1;b+=1){const x=r[b],T=r[b+1];if(n>=x.x&&n<=T.x){a=b;break}}const i=r[a],o=r[a+1],s=Math.max(1e-6,o.x-i.x),l=Rn((n-i.x)/s,0,1),c=l*l,u=c*l,d=ox(r),m=d[a],p=d[a+1],f=2*u-3*c+1,h=u-2*c+l,g=-2*u+3*c,v=u-c,S=f*i.y+h*s*m+g*o.y+v*s*p;return Rn(S,0,1)}const Ee={shadow:.25,highlight:.75},fs=["master","red","green","blue"],po=.02,fo=.98,Mr=.04;function We(e,t,r){return Math.max(t,Math.min(r,e))}function Ce(e,t){return typeof e=="number"&&Number.isFinite(e)?e:t}function lx(e,t){const r=Ce(e[t],0);return We(r/200,-.5,.5)}function cx(e,t){const r=1-e;return e+r*r*t*.5}function ux(e,t){const r=4*e*(1-e);return e+r*t*.25}function dx(e,t){return e+e*e*t*.5}function mx(e,t){return(e-.5)*(1+t)+.5}function Nc(e,t){const r=Ce(e.shadows,0)/100,n=Ce(e.midtones,0)/100,a=Ce(e.highlights,0)/100,i=Ce(e.contrast,0)/100;let o=t;return o=cx(o,r),o=ux(o,n),o=dx(o,a),o=mx(o,i),We(o,0,1)}function ul(e){const t=e.charAt(0).toUpperCase()+e.slice(1);return{shadowX:`${e}ShadowX`,shadowY:`${e}ShadowY`,highlightX:`${e}HighlightX`,highlightY:`${e}HighlightY`,prefix:t}}function Qp(){return{shadow:{x:Ee.shadow,y:Ee.shadow},highlight:{x:Ee.highlight,y:Ee.highlight}}}function Za(e){const t=Qp();let r=We(Ce(e.shadow.x,t.shadow.x),po,fo),n=We(Ce(e.highlight.x,t.highlight.x),po,fo);if(r>n&&([r,n]=[n,r]),n-r<Mr){const a=We((r+n)/2,po+Mr/2,fo-Mr/2);r=a-Mr/2,n=a+Mr/2}return{shadow:{x:r,y:We(Ce(e.shadow.y,t.shadow.y),0,1)},highlight:{x:n,y:We(Ce(e.highlight.y,t.highlight.y),0,1)}}}function px(e){const t=Za(e);return[{x:0,y:0},t.shadow,t.highlight,{x:1,y:1}]}function fx(e,t){const r=ul(t);if([r.shadowX,r.shadowY,r.highlightX,r.highlightY].some(i=>typeof e[i]=="number"&&Number.isFinite(e[i])))return Za({shadow:{x:Ce(e[r.shadowX],Ee.shadow),y:Ce(e[r.shadowY],Ee.shadow)},highlight:{x:Ce(e[r.highlightX],Ee.highlight),y:Ce(e[r.highlightY],Ee.highlight)}});if(t==="master")return Za({shadow:{x:Ee.shadow,y:Nc(e,Ee.shadow)},highlight:{x:Ee.highlight,y:Nc(e,Ee.highlight)}});const a=lx(e,t);return Za({shadow:{x:Ee.shadow,y:Ee.shadow+a},highlight:{x:Ee.highlight,y:Ee.highlight+a}})}const hs=256,hx=16;function wi(e){return`${e}Points`}function gx(e){const t=e.filter(n=>Number.isFinite(n.x)&&Number.isFinite(n.y)).map(n=>({x:We(n.x,0,1),y:We(n.y,0,1)})).sort((n,a)=>n.x-a.x).slice(0,hx),r=[];for(const n of t){const a=r[r.length-1];a&&n.x-a.x<Mr/2||r.push(n)}return r.length<2?[{x:0,y:0},{x:1,y:1}]:r}function yx(e){try{const t=JSON.parse(e);if(!Array.isArray(t))return null;const r=[];for(const n of t){if(!Array.isArray(n)||n.length<2)return null;const[a,i]=n;if(typeof a!="number"||typeof i!="number")return null;r.push({x:a,y:i})}return r.length>=2?gx(r):null}catch{return null}}function Ca(e,t){const r=e[wi(t)];if(typeof r=="string"&&r.length>0){const n=yx(r);if(n)return n}return px(fx(e,t))}function vx(e){const t=hs,r=Ca(e,"master"),n=Ca(e,"red"),a=Ca(e,"green"),i=Ca(e,"blue"),o=new Uint8Array(t*4);for(let s=0;s<t;s++){const l=s/(t-1),c=Ma(r,l);o[s*4]=Math.round(We(Ma(n,c),0,1)*255),o[s*4+1]=Math.round(We(Ma(a,c),0,1)*255),o[s*4+2]=Math.round(We(Ma(i,c),0,1)*255),o[s*4+3]=255}return o}function bx(e){const t=[];for(const r of fs){const n=ul(r);t.push(Ce(e[n.shadowX],-1),Ce(e[n.shadowY],-1),Ce(e[n.highlightX],-1),Ce(e[n.highlightY],-1),typeof e[wi(r)]=="string"?e[wi(r)]:"")}return t.join("|")}function Ue(e,t,r){const n=e[t];return typeof n=="number"?n:r}function Sx(e,t){if(typeof e!="string"||!e.startsWith("#"))return t;const r=e.slice(1),n=r.length===3?r.split("").map(s=>s+s).join(""):r;if(!/^[0-9a-fA-F]{6}$/.test(n))return t;const a=parseInt(n.slice(0,2),16)/255,i=parseInt(n.slice(2,4),16)/255,o=parseInt(n.slice(4,6),16)/255;return[a,i,o].every(Number.isFinite)?[a,i,o]:t}const Ix={id:"gpu-brightness",name:"Brightness",category:"color",entryPoint:"brightnessFragment",uniformSize:16,shader:`
struct BrightnessParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: BrightnessParams;
@fragment
fn brightnessFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let adjusted = color.rgb + params.amount;
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:0,min:-1,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??0,0,0,0])},xx={id:"gpu-contrast",name:"Contrast",category:"color",entryPoint:"contrastFragment",uniformSize:16,shader:`
struct ContrastParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ContrastParams;
@fragment
fn contrastFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let adjusted = (color.rgb - 0.5) * params.amount + 0.5;
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:3,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},wx={id:"gpu-exposure",name:"Exposure",category:"color",entryPoint:"exposureFragment",uniformSize:16,shader:`
struct ExposureParams { exposure: f32, offset: f32, gamma: f32, _pad: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ExposureParams;
@fragment
fn exposureFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  var adjusted = color.rgb * pow(2.0, params.exposure);
  adjusted += params.offset;
  adjusted = pow(max(adjusted, vec3f(0.0)), vec3f(1.0 / params.gamma));
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{exposure:{type:"number",label:"Exposure (EV)",default:0,min:-3,max:3,step:.1,animatable:!0},offset:{type:"number",label:"Offset",default:0,min:-.5,max:.5,step:.01,animatable:!0},gamma:{type:"number",label:"Gamma",default:1,min:.2,max:3,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.exposure??0,e.offset??0,e.gamma??1,0])},kx={id:"gpu-hue-shift",name:"Hue Shift",category:"color",entryPoint:"hueShiftFragment",uniformSize:16,shader:`
struct HueShiftParams { shift: f32, span: f32, flow: f32, time: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: HueShiftParams;
@fragment
fn hueShiftFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  var hsv = rgb2hsv(color.rgb);
  // span compresses (<1) or expands (>1) the hue range around the shift offset;
  // span = 1 is a plain hue rotation (backward compatible), span = 0 maps every
  // pixel to a single hue (monochrome tint). flow cycles the offset over time.
  hsv.x = fract(params.shift + params.flow * params.time + hsv.x * params.span);
  return vec4f(hsv2rgb(hsv), color.a);
}`,params:{shift:{type:"number",label:"Shift",default:0,min:0,max:1,step:.01,animatable:!0},span:{type:"number",label:"Span",default:1,min:0,max:2,step:.01,animatable:!0},flow:{type:"number",label:"Flow",default:0,min:0,max:2,step:.05,animatable:!1}},packUniforms:e=>new Float32Array([e.shift??0,e.span??1,e.flow??0,performance.now()/1e3])},Px={id:"gpu-invert",name:"Invert",category:"color",entryPoint:"invertFragment",uniformSize:0,shader:`
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@fragment
fn invertFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  return vec4f(1.0 - color.rgb, color.a);
}`,params:{},packUniforms:()=>null},Mx={id:"gpu-levels",name:"Levels",category:"color",entryPoint:"levelsFragment",uniformSize:32,shader:`
struct LevelsParams {
  inputBlack: f32, inputWhite: f32, gamma: f32, outputBlack: f32,
  outputWhite: f32, _p1: f32, _p2: f32, _p3: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: LevelsParams;
@fragment
fn levelsFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  var adjusted = (color.rgb - vec3f(params.inputBlack)) /
                 (params.inputWhite - params.inputBlack);
  adjusted = clamp(adjusted, vec3f(0.0), vec3f(1.0));
  adjusted = pow(adjusted, vec3f(1.0 / params.gamma));
  adjusted = mix(vec3f(params.outputBlack), vec3f(params.outputWhite), adjusted);
  return vec4f(adjusted, color.a);
}`,params:{inputBlack:{type:"number",label:"Input Black",default:0,min:0,max:1,step:.01,animatable:!0},inputWhite:{type:"number",label:"Input White",default:1,min:0,max:1,step:.01,animatable:!0},gamma:{type:"number",label:"Gamma",default:1,min:.1,max:3,step:.01,animatable:!0},outputBlack:{type:"number",label:"Output Black",default:0,min:0,max:1,step:.01,animatable:!0},outputWhite:{type:"number",label:"Output White",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.inputBlack??0,e.inputWhite??1,e.gamma??1,e.outputBlack??0,e.outputWhite??1,0,0,0])},Cx={id:"gpu-saturation",name:"Saturation",category:"color",entryPoint:"saturationFragment",uniformSize:16,shader:`
struct SaturationParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: SaturationParams;
@fragment
fn saturationFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let gray = luminance601(color.rgb);
  let adjusted = mix(vec3f(gray), color.rgb, params.amount);
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:3,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},Tx={id:"gpu-temperature",name:"Temperature",category:"color",entryPoint:"temperatureFragment",uniformSize:16,shader:`
struct TemperatureParams { temperature: f32, tint: f32, _p1: f32, _p2: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: TemperatureParams;
@fragment
fn temperatureFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  var adjusted = color.rgb;
  adjusted.r += params.temperature * 0.1;
  adjusted.b -= params.temperature * 0.1;
  adjusted.g -= params.tint * 0.1;
  adjusted.r += params.tint * 0.05;
  adjusted.b += params.tint * 0.05;
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{temperature:{type:"number",label:"Temperature",default:0,min:-1,max:1,step:.01,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-1,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.temperature??0,e.tint??0,0,0])},Ex={id:"gpu-grayscale",name:"Grayscale",category:"color",entryPoint:"grayscaleFragment",uniformSize:16,shader:`
struct GrayscaleParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: GrayscaleParams;
@fragment
fn grayscaleFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let gray = luminance601(color.rgb);
  let adjusted = mix(color.rgb, vec3f(gray), params.amount);
  return vec4f(adjusted, color.a);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},Fx={id:"gpu-sepia",name:"Sepia",category:"color",entryPoint:"sepiaFragment",uniformSize:16,shader:`
struct SepiaParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: SepiaParams;
@fragment
fn sepiaFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let sepiaR = dot(color.rgb, vec3f(0.393, 0.769, 0.189));
  let sepiaG = dot(color.rgb, vec3f(0.349, 0.686, 0.168));
  let sepiaB = dot(color.rgb, vec3f(0.272, 0.534, 0.131));
  let sepiaColor = vec3f(sepiaR, sepiaG, sepiaB);
  let adjusted = mix(color.rgb, sepiaColor, params.amount);
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},Ax={id:"gpu-curves",name:"Curves",category:"color",entryPoint:"curvesFragment",uniformSize:0,shader:`
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(3) var curveLut: texture_2d<f32>;

fn sampleCurveLut(value: f32) -> vec3f {
  let lutWidth = ${hs}.0;
  let u = (clamp(value, 0.0, 1.0) * (lutWidth - 1.0) + 0.5) / lutWidth;
  return textureSample(curveLut, texSampler, vec2f(u, 0.5)).rgb;
}

@fragment
fn curvesFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let c = vec3f(
    sampleCurveLut(color.r).r,
    sampleCurveLut(color.g).g,
    sampleCurveLut(color.b).b,
  );
  return vec4f(c, color.a);
}`,params:Object.fromEntries([...fs.flatMap(e=>{const t=ul(e),r=Qp(),n=e.charAt(0).toUpperCase()+e.slice(1);return[[t.shadowX,{type:"number",label:`${n} Shadow X`,default:r.shadow.x,min:0,max:1,step:.001,animatable:!0}],[t.shadowY,{type:"number",label:`${n} Shadow Y`,default:r.shadow.y,min:0,max:1,step:.001,animatable:!0}],[t.highlightX,{type:"number",label:`${n} Highlight X`,default:r.highlight.x,min:0,max:1,step:.001,animatable:!0}],[t.highlightY,{type:"number",label:`${n} Highlight Y`,default:r.highlight.y,min:0,max:1,step:.001,animatable:!0}]]}),...fs.map(e=>{const t=e.charAt(0).toUpperCase()+e.slice(1);return[wi(e),{type:"json",label:`${t} Points`,default:""}]})]),packUniforms:()=>null,dataTexture:{dimension:"2d",key:bx,build:e=>({width:hs,height:1,depth:1,data:vx(e)})}},_x={id:"gpu-color-wheels",name:"Color Wheels",category:"color",entryPoint:"colorWheelsFragment",uniformSize:112,shader:`
struct WheelsParams {
  shHue: f32, shAmount: f32, midHue: f32, midAmount: f32,
  hlHue: f32, hlAmount: f32, temperature: f32, tint: f32,
  saturation: f32, exposure: f32, contrast: f32, pivot: f32,
  lift: f32, gamma: f32, gain: f32, offset: f32,
  blackPoint: f32, whitePoint: f32, offHue: f32, offAmount: f32,
  midDetail: f32, colorBoost: f32, shadows: f32, highlights: f32,
  hue: f32, lumMix: f32, _pad1: f32, _pad2: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: WheelsParams;

fn wheelTint(color: vec3f, hue: f32, amount: f32, mask: f32) -> vec3f {
  if (amount < 0.001) { return color; }
  let rad = hue * TAU / 360.0;
  let tintColor = hsv2rgb(vec3f(hue / 360.0, 1.0, 1.0));
  return mix(color, color * mix(vec3f(1.0), tintColor, amount), mask);
}

@fragment
fn colorWheelsFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  var c = color.rgb;
  let luma = luminance601(c);
  let shadowMask = 1.0 - smoothstep(0.0, 0.5, luma);
  let highlightMask = smoothstep(0.5, 1.0, luma);
  let midtoneMask = 1.0 - shadowMask - highlightMask;
  c = wheelTint(c, params.shHue, params.shAmount, shadowMask);
  c = wheelTint(c, params.midHue, params.midAmount, midtoneMask);
  c = wheelTint(c, params.hlHue, params.hlAmount, highlightMask);
  c = wheelTint(c, params.offHue, params.offAmount, 1.0);
  let temp = params.temperature / 100.0;
  c.r += temp * 0.1;
  c.b -= temp * 0.1;
  let ti = params.tint / 100.0;
  c.g -= ti * 0.1;
  c.r += ti * 0.05;
  c.b += ti * 0.05;

  c *= pow(2.0, params.exposure);
  c = (c - vec3f(params.pivot)) * params.contrast + vec3f(params.pivot);
  if (abs(params.midDetail) > 0.001) {
    let detailLuma = luminance601(c);
    let detailAdjusted = vec3f(detailLuma) +
      (c - vec3f(detailLuma)) * (1.0 + params.midDetail / 100.0);
    c = mix(c, detailAdjusted, midtoneMask);
  }
  c = (c + vec3f(params.lift) + vec3f(params.offset)) * params.gain;
  c = pow(max(c, vec3f(0.0)), vec3f(1.0 / max(params.gamma, 0.05)));
  c = (c - vec3f(params.blackPoint)) /
      vec3f(max(params.whitePoint - params.blackPoint, 0.001));
  c += vec3f(params.shadows / 100.0) * shadowMask;
  c += vec3f(params.highlights / 100.0) * highlightMask;

  let sat = 1.0 + params.saturation / 100.0;
  let gray = luminance601(c);
  c = mix(vec3f(gray), c, sat);
  let colorBoost = params.colorBoost / 100.0;
  if (abs(colorBoost) > 0.001) {
    let boostedGray = luminance601(c);
    let chroma = c - vec3f(boostedGray);
    c = vec3f(boostedGray) + chroma * (1.0 + colorBoost * (1.0 - clamp(length(chroma), 0.0, 1.0)));
  }
  if (abs(params.hue - 50.0) > 0.001) {
    var hsv = rgb2hsv(c);
    hsv.x = fract(hsv.x + ((params.hue - 50.0) / 100.0));
    c = hsv2rgb(hsv);
  }
  let postLuma = luminance601(c);
  c = mix(vec3f(postLuma), c, clamp(params.lumMix / 100.0, 0.0, 1.0));
  return vec4f(clamp(c, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{shadowsHue:{type:"number",label:"Lift Hue",default:0,min:0,max:360,step:1,animatable:!0},shadowsAmount:{type:"number",label:"Lift Amount",default:0,min:0,max:1,step:.01,animatable:!0},midtonesHue:{type:"number",label:"Gamma Hue",default:0,min:0,max:360,step:1,animatable:!0},midtonesAmount:{type:"number",label:"Gamma Amount",default:0,min:0,max:1,step:.01,animatable:!0},highlightsHue:{type:"number",label:"Gain Hue",default:0,min:0,max:360,step:1,animatable:!0},highlightsAmount:{type:"number",label:"Gain Amount",default:0,min:0,max:1,step:.01,animatable:!0},offsetHue:{type:"number",label:"Offset Hue",default:0,min:0,max:360,step:1,animatable:!0},offsetAmount:{type:"number",label:"Offset Amount",default:0,min:0,max:1,step:.01,animatable:!0},temperature:{type:"number",label:"Temperature",default:0,min:-100,max:100,step:1,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-100,max:100,step:1,animatable:!0},saturation:{type:"number",label:"Saturation",default:0,min:-100,max:100,step:1,animatable:!0},exposure:{type:"number",label:"Exposure",default:0,min:-3,max:3,step:.05,animatable:!0},contrast:{type:"number",label:"Contrast",default:1,min:0,max:2,step:.01,animatable:!0},pivot:{type:"number",label:"Pivot",default:.5,min:0,max:1,step:.01,animatable:!0},lift:{type:"number",label:"Lift",default:0,min:-2,max:2,step:.01,animatable:!0},gamma:{type:"number",label:"Gamma",default:1,min:0,max:4,step:.01,animatable:!0},gain:{type:"number",label:"Gain",default:1,min:0,max:16,step:.01,animatable:!0},offset:{type:"number",label:"Offset",default:0,min:-2,max:2,step:.0025,animatable:!0},blackPoint:{type:"number",label:"Black Point",default:0,min:0,max:.5,step:.005,animatable:!0},whitePoint:{type:"number",label:"White Point",default:1,min:.5,max:1.5,step:.005,animatable:!0},midDetail:{type:"number",label:"Mid/Detail",default:0,min:-100,max:100,step:1,animatable:!0},colorBoost:{type:"number",label:"Color Boost",default:0,min:-100,max:100,step:1,animatable:!0},shadows:{type:"number",label:"Shadows",default:0,min:-100,max:100,step:1,animatable:!0},highlights:{type:"number",label:"Highlights",default:0,min:-100,max:100,step:1,animatable:!0},hue:{type:"number",label:"Hue",default:50,min:0,max:100,step:1,animatable:!0},lumMix:{type:"number",label:"Lum Mix",default:100,min:0,max:100,step:1,animatable:!0}},packUniforms:e=>new Float32Array(Ox.map(([t,r])=>Ue(e,t,r)))},Ox=[["shadowsHue",0],["shadowsAmount",0],["midtonesHue",0],["midtonesAmount",0],["highlightsHue",0],["highlightsAmount",0],["temperature",0],["tint",0],["saturation",0],["exposure",0],["contrast",1],["pivot",.5],["lift",0],["gamma",1],["gain",1],["offset",0],["blackPoint",0],["whitePoint",1],["offsetHue",0],["offsetAmount",0],["midDetail",0],["colorBoost",0],["shadows",0],["highlights",0],["hue",50],["lumMix",100],["_pad1",0],["_pad2",0]],Dx={id:"gpu-secondary-qualifier",name:"Secondary Qualifier",category:"color",entryPoint:"secondaryQualifierFragment",uniformSize:64,shader:`
struct SecondaryQualifierParams {
  hueCenter: f32, hueWidth: f32, hueSoftness: f32, satLow: f32,
  satHigh: f32, satSoftness: f32, lumaLow: f32, lumaHigh: f32,
  lumaSoftness: f32, invertMask: f32, showMask: f32, exposure: f32,
  saturation: f32, temperature: f32, tint: f32, strength: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: SecondaryQualifierParams;

fn circularHueDistance(hue: f32, center: f32) -> f32 {
  let diff = abs(hue - center);
  return min(diff, 1.0 - diff);
}

fn centeredRangeMask(value: f32, lowValue: f32, highValue: f32, softness: f32) -> f32 {
  let low = min(lowValue, highValue);
  let high = max(lowValue, highValue);
  let soft = max(softness, 0.0001);
  let lowMask = smoothstep(low - soft, low, value);
  let highMask = 1.0 - smoothstep(high, high + soft, value);
  return clamp(lowMask * highMask, 0.0, 1.0);
}

@fragment
fn secondaryQualifierFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let hsv = rgb2hsv(color.rgb);
  let luma = luminance601(color.rgb);
  let hueDistance = circularHueDistance(hsv.x, fract(params.hueCenter / 360.0));
  let hueWidth = clamp(params.hueWidth / 360.0, 0.0, 0.5);
  let hueSoftness = max(params.hueSoftness / 360.0, 0.0001);
  var mask = 1.0 - smoothstep(hueWidth, hueWidth + hueSoftness, hueDistance);
  mask *= centeredRangeMask(hsv.y, params.satLow, params.satHigh, params.satSoftness);
  mask *= centeredRangeMask(luma, params.lumaLow, params.lumaHigh, params.lumaSoftness);
  if (params.invertMask > 0.5) {
    mask = 1.0 - mask;
  }
  mask = clamp(mask * params.strength, 0.0, 1.0);

  if (params.showMask > 0.5) {
    return vec4f(vec3f(mask), color.a);
  }

  var corrected = color.rgb;
  corrected *= pow(2.0, params.exposure);
  let temp = params.temperature / 100.0;
  corrected.r += temp * 0.1;
  corrected.b -= temp * 0.1;
  let ti = params.tint / 100.0;
  corrected.g -= ti * 0.1;
  corrected.r += ti * 0.05;
  corrected.b += ti * 0.05;
  let sat = 1.0 + params.saturation / 100.0;
  let gray = luminance601(corrected);
  corrected = mix(vec3f(gray), corrected, sat);

  return vec4f(clamp(mix(color.rgb, corrected, mask), vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{hueCenter:{type:"number",label:"Hue Center",default:0,min:0,max:360,step:1,animatable:!0},hueWidth:{type:"number",label:"Hue Width",default:35,min:0,max:180,step:1,animatable:!0},hueSoftness:{type:"number",label:"Hue Softness",default:20,min:0,max:120,step:1,animatable:!0},satLow:{type:"number",label:"Sat Low",default:0,min:0,max:1,step:.01,animatable:!0},satHigh:{type:"number",label:"Sat High",default:1,min:0,max:1,step:.01,animatable:!0},satSoftness:{type:"number",label:"Sat Softness",default:.1,min:0,max:1,step:.01,animatable:!0},lumaLow:{type:"number",label:"Luma Low",default:0,min:0,max:1,step:.01,animatable:!0},lumaHigh:{type:"number",label:"Luma High",default:1,min:0,max:1,step:.01,animatable:!0},lumaSoftness:{type:"number",label:"Luma Softness",default:.1,min:0,max:1,step:.01,animatable:!0},invertMask:{type:"boolean",label:"Invert Mask",default:!1},showMask:{type:"boolean",label:"Show Mask",default:!1},exposure:{type:"number",label:"Exposure",default:0,min:-3,max:3,step:.05,animatable:!0},saturation:{type:"number",label:"Saturation",default:0,min:-100,max:100,step:1,animatable:!0},temperature:{type:"number",label:"Temperature",default:0,min:-100,max:100,step:1,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-100,max:100,step:1,animatable:!0},strength:{type:"number",label:"Strength",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array(Rx.map(([t,r])=>{const n=e[t];return typeof n=="boolean"?n?1:0:typeof n=="number"?n:r}))},Rx=[["hueCenter",0],["hueWidth",35],["hueSoftness",20],["satLow",0],["satHigh",1],["satSoftness",.1],["lumaLow",0],["lumaHigh",1],["lumaSoftness",.1],["invertMask",0],["showMask",0],["exposure",0],["saturation",0],["temperature",0],["tint",0],["strength",1]],Lx={ellipse:0,rectangle:1},Bx={id:"gpu-power-window",name:"Power Window",category:"color",entryPoint:"powerWindowFragment",uniformSize:64,shader:`
struct PowerWindowParams {
  shapeKind: f32, centerX: f32, centerY: f32, sizeX: f32,
  sizeY: f32, rotation: f32, feather: f32, invertMask: f32,
  showMask: f32, exposure: f32, saturation: f32, temperature: f32,
  tint: f32, strength: f32, sourceWidth: f32, sourceHeight: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: PowerWindowParams;

fn rotateWindowPoint(point: vec2f, angleDeg: f32) -> vec2f {
  let angle = -angleDeg * PI / 180.0;
  let c = cos(angle);
  let s = sin(angle);
  return vec2f(point.x * c - point.y * s, point.x * s + point.y * c);
}

fn powerWindowMask(uv: vec2f) -> f32 {
  let aspect = max(params.sourceWidth / max(params.sourceHeight, 1.0), 0.0001);
  var local = uv - vec2f(params.centerX, params.centerY);
  local.x *= aspect;
  local = rotateWindowPoint(local, params.rotation);

  let size = max(vec2f(params.sizeX * aspect, params.sizeY) * 0.5, vec2f(0.0001));
  let normalized = local / size;
  let shapeKind = i32(params.shapeKind + 0.5);
  var dist = length(normalized);
  if (shapeKind == 1) {
    dist = max(abs(normalized.x), abs(normalized.y));
  }
  let feather = clamp(params.feather, 0.001, 1.0);
  return clamp(1.0 - smoothstep(1.0 - feather, 1.0, dist), 0.0, 1.0);
}

@fragment
fn powerWindowFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  var mask = powerWindowMask(input.uv);
  if (params.invertMask > 0.5) {
    mask = 1.0 - mask;
  }
  mask = clamp(mask * params.strength, 0.0, 1.0);

  if (params.showMask > 0.5) {
    return vec4f(vec3f(mask), color.a);
  }

  var corrected = color.rgb;
  corrected *= pow(2.0, params.exposure);
  let temp = params.temperature / 100.0;
  corrected.r += temp * 0.1;
  corrected.b -= temp * 0.1;
  let ti = params.tint / 100.0;
  corrected.g -= ti * 0.1;
  corrected.r += ti * 0.05;
  corrected.b += ti * 0.05;
  let sat = 1.0 + params.saturation / 100.0;
  let gray = luminance601(corrected);
  corrected = mix(vec3f(gray), corrected, sat);

  return vec4f(clamp(mix(color.rgb, corrected, mask), vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{shape:{type:"select",label:"Shape",default:"ellipse",options:[{value:"ellipse",label:"Ellipse"},{value:"rectangle",label:"Rectangle"}]},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},sizeX:{type:"number",label:"Width",default:.5,min:.02,max:1.5,step:.01,animatable:!0},sizeY:{type:"number",label:"Height",default:.5,min:.02,max:1.5,step:.01,animatable:!0},rotation:{type:"number",label:"Rotation",default:0,min:-180,max:180,step:1,animatable:!0},feather:{type:"number",label:"Feather",default:.3,min:0,max:1,step:.01,animatable:!0},invertMask:{type:"boolean",label:"Invert Mask",default:!1},showMask:{type:"boolean",label:"Show Mask",default:!1},exposure:{type:"number",label:"Exposure",default:.3,min:-3,max:3,step:.05,animatable:!0},saturation:{type:"number",label:"Saturation",default:0,min:-100,max:100,step:1,animatable:!0},temperature:{type:"number",label:"Temperature",default:0,min:-100,max:100,step:1,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-100,max:100,step:1,animatable:!0},strength:{type:"number",label:"Strength",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([Lx[e.shape]??0,Ue(e,"centerX",.5),Ue(e,"centerY",.5),Ue(e,"sizeX",.5),Ue(e,"sizeY",.5),Ue(e,"rotation",0),Ue(e,"feather",.3),e.invertMask===!0?1:0,e.showMask===!0?1:0,Ue(e,"exposure",.3),Ue(e,"saturation",0),Ue(e,"temperature",0),Ue(e,"tint",0),Ue(e,"strength",1),t,r])},zx={id:"gpu-vibrance",name:"Vibrance",category:"color",entryPoint:"vibranceFragment",uniformSize:16,shader:`
struct VibranceParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: VibranceParams;
@fragment
fn vibranceFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let maxC = max(max(color.r, color.g), color.b);
  let minC = min(min(color.r, color.g), color.b);
  let sat = (maxC - minC) / (maxC + 0.001);
  let vibrance = params.amount * (1.0 - sat);
  let gray = luminance601(color.rgb);
  let adjusted = mix(vec3f(gray), color.rgb, 1.0 + vibrance);
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:0,min:-1,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??0,0,0,0])},ki={inferno:["#000004","#420a68","#932667","#dd513a","#fca50a","#f0f921"],magma:["#000004","#3b0f70","#8c2981","#de4968","#fe9f6d","#fcfdbf"],plasma:["#0d0887","#6a00a8","#b12a90","#e16462","#fca636","#f0f921"],viridis:["#440154","#3b528b","#21918c","#5ec962","#fde725"],turbo:["#30123b","#4675ed","#1bcfd4","#a4fc3b","#fe9b2d","#cb2a04","#7a0403"],fire:["#000000","#7a0000","#ff4800","#ffd000","#ffffff"],ice:["#000010","#003b6f","#1b78c2","#7ec8ff","#ffffff"],sunset:["#241634","#c2456b","#ffd9a0"],grayscale:["#000000","#ffffff"]},Ux=ki.inferno.join(", ");function Hx(e,t){const n=(e==="custom"?t.split(",").map(a=>a.trim()).filter(a=>a.length>0):ki[e]??ki.inferno).map(a=>Sx(a,[0,0,0]));return n.length===0?[[0,0,0],[1,1,1]]:n.length===1?[n[0],n[0]]:n}function Nx(e){const r=new Uint8Array(1024),n=e.length-1;for(let a=0;a<256;a++){const o=a/255*n,s=Math.min(Math.floor(o),n-1),l=o-s,c=e[s],u=e[s+1];r[a*4]=Math.round((c[0]+(u[0]-c[0])*l)*255),r[a*4+1]=Math.round((c[1]+(u[1]-c[1])*l)*255),r[a*4+2]=Math.round((c[2]+(u[2]-c[2])*l)*255),r[a*4+3]=255}return{width:256,height:1,depth:1,data:r}}const qx={id:"gpu-gradient-map",name:"Gradient Map",category:"color",entryPoint:"gradientMapFragment",uniformSize:16,shader:`
struct GradientMapParams { mix: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: GradientMapParams;
@group(0) @binding(3) var gradientLut: texture_2d<f32>;
@fragment
fn gradientMapFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let lum = clamp(luminance601(color.rgb), 0.0, 1.0);
  let mapped = textureSampleLevel(gradientLut, texSampler, vec2f(lum, 0.5), 0.0).rgb;
  let outRgb = mix(color.rgb, mapped, clamp(params.mix, 0.0, 1.0));
  return vec4f(outRgb, color.a);
}`,params:{preset:{type:"select",label:"Palette",default:"inferno",options:[{value:"inferno",label:"Inferno"},{value:"magma",label:"Magma"},{value:"plasma",label:"Plasma"},{value:"viridis",label:"Viridis"},{value:"turbo",label:"Turbo"},{value:"fire",label:"Fire"},{value:"ice",label:"Ice"},{value:"sunset",label:"Sunset"},{value:"grayscale",label:"Grayscale"},{value:"custom",label:"Custom"}]},customStops:{type:"text",label:"Custom Stops",default:Ux,visibleWhen:e=>e.preset==="custom"},mix:{type:"number",label:"Mix",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([Ue(e,"mix",1),0,0,0]),dataTexture:{dimension:"2d",key:e=>{const t=e.preset??"inferno";return t==="custom"?`custom:${e.customStops??""}`:`preset:${t}`},build:e=>Nx(Hx(e.preset??"inferno",e.customStops??""))}};var Gx=Object.freeze({__proto__:null,GRADIENT_MAP_PRESETS:ki,brightness:Ix,colorWheels:_x,contrast:xx,curves:Ax,exposure:wx,gradientMap:qx,grayscale:Ex,hueShift:kx,invert:Px,levels:Mx,powerWindow:Bx,saturation:Cx,secondaryQualifier:Dx,sepia:Fx,temperature:Tx,vibrance:zx});function Kx(e){const t=atob(e),r=new Uint8Array(t.length);for(let n=0;n<t.length;n++)r[n]=t.charCodeAt(n);return r}function Vx(e){const t=new Uint8Array(e*e*e*4),r=e-1;let n=0;for(let a=0;a<e;a++){const i=Math.round(a/r*255);for(let o=0;o<e;o++){const s=Math.round(o/r*255);for(let l=0;l<e;l++)t[n]=Math.round(l/r*255),t[n+1]=s,t[n+2]=i,t[n+3]=255,n+=4}}return t}const kn=2,Wx={id:"gpu-lut",name:"LUT (.cube)",category:"color",entryPoint:"lutFragment",uniformSize:16,shader:`
struct LutParams { intensity: f32, size: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: LutParams;
@group(0) @binding(3) var lutTex: texture_3d<f32>;
@fragment
fn lutFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let size = max(params.size, 2.0);
  let coords = (clamp(color.rgb, vec3f(0.0), vec3f(1.0)) * (size - 1.0) + vec3f(0.5)) / size;
  let graded = textureSample(lutTex, texSampler, coords).rgb;
  return vec4f(mix(color.rgb, graded, clamp(params.intensity, 0.0, 1.0)), color.a);
}`,params:{intensity:{type:"number",label:"Intensity",default:1,min:0,max:1,step:.01,animatable:!0},lutName:{type:"json",label:"LUT Name",default:""},lutSize:{type:"json",label:"LUT Size",default:"0"},lutData:{type:"json",label:"LUT Data",default:""}},packUniforms:e=>{const t=typeof e.intensity=="number"?e.intensity:1,r=qc(e);return new Float32Array([t,r,0,0])},dataTexture:{dimension:"3d",key:e=>typeof e.lutData=="string"?e.lutData:"",build:e=>{const t=qc(e),r=typeof e.lutData=="string"?e.lutData:"";if(t>=2&&r.length>0)try{const n=Kx(r);if(n.length===t*t*t*4)return{width:t,height:t,depth:t,data:n}}catch{}return{width:kn,height:kn,depth:kn,data:Vx(kn)}}}};function qc(e){const t=e.lutSize,r=typeof t=="number"?t:typeof t=="string"?Number(t):0;return!Number.isFinite(r)||r<2||r>129?kn:Math.floor(r)}var jx=Object.freeze({__proto__:null,lut3d:Wx});const $x={id:"gpu-gaussian-blur",name:"Gaussian Blur",category:"blur",entryPoint:"gaussianBlurFragment",uniformSize:16,shader:`
struct GaussianBlurParams { radius: f32, width: f32, height: f32, samples: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: GaussianBlurParams;
@fragment
fn gaussianBlurFragment(input: VertexOutput) -> @location(0) vec4f {
  if (params.radius < 0.5) {
    return textureSample(inputTex, texSampler, input.uv);
  }
  let texelSize = vec2f(1.0 / params.width, 1.0 / params.height);
  let sampleRadius = i32(clamp(params.samples, 1.0, 64.0));
  var color = vec4f(0.0);
  var totalWeight = 0.0;
  let sigma = params.radius / 3.0;
  let twoSigmaSq = 2.0 * sigma * sigma;
  for (var x = -sampleRadius; x <= sampleRadius; x++) {
    for (var y = -sampleRadius; y <= sampleRadius; y++) {
      let offset = vec2f(f32(x), f32(y)) * texelSize * (params.radius / f32(sampleRadius));
      let distSq = f32(x * x + y * y);
      let weight = exp(-distSq / twoSigmaSq);
      color += textureSample(inputTex, texSampler, input.uv + offset) * weight;
      totalWeight += weight;
    }
  }
  return color / totalWeight;
}`,params:{radius:{type:"number",label:"Radius",default:10,min:0,max:50,step:1,animatable:!0},samples:{type:"number",label:"Samples",default:5,min:1,max:64,step:1,animatable:!1,quality:!0}},packUniforms:(e,t,r)=>new Float32Array([e.radius??10,t,r,e.samples??5])},Yx={id:"gpu-box-blur",name:"Box Blur",category:"blur",entryPoint:"boxBlurFragment",uniformSize:16,shader:`
struct BoxBlurParams { radius: f32, width: f32, height: f32, _pad: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: BoxBlurParams;
@fragment
fn boxBlurFragment(input: VertexOutput) -> @location(0) vec4f {
  if (params.radius < 0.5) {
    return textureSample(inputTex, texSampler, input.uv);
  }
  let texelSize = vec2f(1.0 / params.width, 1.0 / params.height);
  let samples = i32(params.radius);
  var color = vec4f(0.0);
  var count = 0.0;
  for (var x = -samples; x <= samples; x++) {
    for (var y = -samples; y <= samples; y++) {
      let offset = vec2f(f32(x), f32(y)) * texelSize;
      color += textureSample(inputTex, texSampler, input.uv + offset);
      count += 1.0;
    }
  }
  return color / count;
}`,params:{radius:{type:"number",label:"Radius",default:5,min:0,max:20,step:1,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.radius??5,t,r,0])},Xx={id:"gpu-motion-blur",name:"Motion Blur",category:"blur",entryPoint:"motionBlurFragment",uniformSize:32,shader:`
struct MotionBlurParams {
  amount: f32,
  angle: f32,
  shutterAngle: f32,
  samples: f32,
  maxRadius: f32,
  _pad0: f32,
  _pad1: f32,
  _pad2: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: MotionBlurParams;
@fragment
fn motionBlurFragment(input: VertexOutput) -> @location(0) vec4f {
  let exposure = clamp(params.shutterAngle / 360.0, 0.0, 1.0);
  let radius = min(max(params.amount, 0.0) * exposure, params.maxRadius);
  if (radius < 0.001 || exposure < 0.001) {
    return textureSample(inputTex, texSampler, input.uv);
  }
  let direction = vec2f(cos(params.angle), sin(params.angle));
  // Quality is deliberately bounded: shutter blur is on a per-layer hot path.
  let samples = i32(clamp(params.samples, 4.0, 32.0));
  var color = vec4f(0.0);
  var totalWeight = 0.0;
  for (var i = 0; i < samples; i++) {
    let t = (f32(i) / f32(samples - 1) - 0.5) * 2.0;
    let offset = direction * t * radius;
    let weight = exp(-t * t * 2.0);
    color += textureSample(inputTex, texSampler, input.uv + offset) * weight;
    totalWeight += weight;
  }
  return color / totalWeight;
}`,params:{amount:{type:"number",label:"Amount",default:.05,min:0,max:.3,step:.005,animatable:!0},angle:{type:"number",label:"Angle",default:0,min:0,max:6.28318,step:.01,animatable:!0},samples:{type:"number",label:"Samples",default:16,min:4,max:32,step:1,animatable:!1,quality:!0},shutterAngle:{type:"number",label:"Shutter Angle",default:180,min:0,max:360,step:1,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.05,e.angle??0,e.shutterAngle??(Object.keys(e).length>0?360:180),e.samples??16,.2,0,0,0])},Qx={id:"gpu-radial-blur",name:"Radial Blur",category:"blur",entryPoint:"radialBlurFragment",uniformSize:16,shader:`
struct RadialBlurParams { amount: f32, centerX: f32, centerY: f32, samples: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: RadialBlurParams;
@fragment
fn radialBlurFragment(input: VertexOutput) -> @location(0) vec4f {
  let center = vec2f(params.centerX, params.centerY);
  let dir = input.uv - center;
  let dist = length(dir);
  if (params.amount < 0.01) {
    return textureSample(inputTex, texSampler, input.uv);
  }
  var color = vec4f(0.0);
  let samples = i32(clamp(params.samples, 4.0, 256.0));
  let amount = params.amount * 0.2;
  var totalWeight = 0.0;
  for (var i = 0; i < samples; i++) {
    let t = f32(i) / f32(samples - 1);
    let scale = 1.0 - amount * t * dist;
    let weight = 1.0 - t * 0.5;
    let samplePos = center + dir * scale;
    color += textureSample(inputTex, texSampler, samplePos) * weight;
    totalWeight += weight;
  }
  return color / totalWeight;
}`,params:{amount:{type:"number",label:"Amount",default:.5,min:0,max:2,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},samples:{type:"number",label:"Samples",default:32,min:4,max:256,step:1,animatable:!1,quality:!0}},packUniforms:e=>new Float32Array([e.amount??.5,e.centerX??.5,e.centerY??.5,e.samples??32])},Jx={id:"gpu-zoom-blur",name:"Zoom Blur",category:"blur",entryPoint:"zoomBlurFragment",uniformSize:16,shader:`
struct ZoomBlurParams { amount: f32, centerX: f32, centerY: f32, samples: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ZoomBlurParams;
@fragment
fn zoomBlurFragment(input: VertexOutput) -> @location(0) vec4f {
  let center = vec2f(params.centerX, params.centerY);
  let dir = input.uv - center;
  var color = vec4f(0.0);
  let samples = i32(clamp(params.samples, 4.0, 256.0));
  let amount = params.amount * 0.5;
  for (var i = 0; i < samples; i++) {
    let t = f32(i) / f32(samples - 1);
    let scale = 1.0 + amount * t;
    let samplePos = center + dir * scale;
    color += textureSample(inputTex, texSampler, samplePos);
  }
  return color / f32(samples);
}`,params:{amount:{type:"number",label:"Amount",default:.3,min:0,max:1,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},samples:{type:"number",label:"Samples",default:16,min:4,max:256,step:1,animatable:!1,quality:!0}},packUniforms:e=>new Float32Array([e.amount??.3,e.centerX??.5,e.centerY??.5,e.samples??16])};var Zx=Object.freeze({__proto__:null,boxBlur:Yx,gaussianBlur:$x,motionBlur:Xx,radialBlur:Qx,zoomBlur:Jx});function Gt(e,t){if(!e.startsWith("#"))return t;const r=e.slice(1);if(r.length===3||r.length===4){const n=r.split("").map(a=>parseInt(a+a,16)/255);return n.slice(0,3).every(a=>Number.isFinite(a))?[n[0]??t[0],n[1]??t[1],n[2]??t[2],n[3]??1]:t}if(r.length===6||r.length===8){const n=[parseInt(r.slice(0,2),16)/255,parseInt(r.slice(2,4),16)/255,parseInt(r.slice(4,6),16)/255,r.length===8?parseInt(r.slice(6,8),16)/255:1];if(n.every(a=>Number.isFinite(a)))return n}return t}const e1={id:"gpu-pixelate",name:"Pixelate",category:"distort",entryPoint:"pixelateFragment",uniformSize:16,shader:`
struct PixelateParams { pixelSize: f32, width: f32, height: f32, _p: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: PixelateParams;
@fragment
fn pixelateFragment(input: VertexOutput) -> @location(0) vec4f {
  let pixelX = params.pixelSize / params.width;
  let pixelY = params.pixelSize / params.height;
  let uv = vec2f(
    floor(input.uv.x / pixelX) * pixelX + pixelX * 0.5,
    floor(input.uv.y / pixelY) * pixelY + pixelY * 0.5
  );
  return textureSample(inputTex, texSampler, uv);
}`,params:{size:{type:"number",label:"Pixel Size",default:8,min:1,max:64,step:1,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.size??8,t,r,0])},t1={id:"gpu-rgb-split",name:"RGB Split",category:"distort",entryPoint:"rgbSplitFragment",uniformSize:16,shader:`
struct RGBSplitParams { amount: f32, angle: f32, _p1: f32, _p2: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: RGBSplitParams;
@fragment
fn rgbSplitFragment(input: VertexOutput) -> @location(0) vec4f {
  let offset = vec2f(cos(params.angle), sin(params.angle)) * params.amount;
  let r = textureSample(inputTex, texSampler, input.uv + offset).r;
  let g = textureSample(inputTex, texSampler, input.uv).g;
  let b = textureSample(inputTex, texSampler, input.uv - offset).b;
  let a = textureSample(inputTex, texSampler, input.uv).a;
  return vec4f(r, g, b, a);
}`,params:{amount:{type:"number",label:"Amount",default:.01,min:0,max:.1,step:.001,animatable:!0},angle:{type:"number",label:"Angle",default:0,min:0,max:6.28318,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.01,e.angle??0,0,0])},r1={id:"gpu-twirl",name:"Twirl",category:"distort",entryPoint:"twirlFragment",uniformSize:16,shader:`
struct TwirlParams { amount: f32, radius: f32, centerX: f32, centerY: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: TwirlParams;
@fragment
fn twirlFragment(input: VertexOutput) -> @location(0) vec4f {
  let center = vec2f(params.centerX, params.centerY);
  let delta = input.uv - center;
  let dist = length(delta);
  let safeRadius = max(params.radius, 0.0001);
  let factor = 1.0 - min(dist / safeRadius, 1.0);
  let angle = params.amount * factor * factor;
  let s = sin(angle);
  let c = cos(angle);
  let rotated = vec2f(delta.x * c - delta.y * s, delta.x * s + delta.y * c);
  let twirledUV = center + rotated;
  let inRadius = dist < params.radius;
  let finalUV = select(input.uv, twirledUV, inRadius);
  return textureSample(inputTex, texSampler, finalUV);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:-10,max:10,step:.1,animatable:!0},radius:{type:"number",label:"Radius",default:.5,min:.1,max:1,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,e.radius??.5,e.centerX??.5,e.centerY??.5])},n1={id:"gpu-wave",name:"Wave",category:"distort",entryPoint:"waveFragment",uniformSize:16,shader:`
struct WaveParams { amplitudeX: f32, amplitudeY: f32, frequencyX: f32, frequencyY: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: WaveParams;
@fragment
fn waveFragment(input: VertexOutput) -> @location(0) vec4f {
  var uv = input.uv;
  uv.y += sin(uv.x * params.frequencyX * TAU) * params.amplitudeX;
  uv.x += sin(uv.y * params.frequencyY * TAU) * params.amplitudeY;
  return textureSample(inputTex, texSampler, uv);
}`,params:{amplitudeX:{type:"number",label:"Horizontal Amp",default:.02,min:0,max:.1,step:.001,animatable:!0},amplitudeY:{type:"number",label:"Vertical Amp",default:.02,min:0,max:.1,step:.001,animatable:!0},frequencyX:{type:"number",label:"Horizontal Freq",default:5,min:1,max:20,step:.5,animatable:!0},frequencyY:{type:"number",label:"Vertical Freq",default:5,min:1,max:20,step:.5,animatable:!0}},packUniforms:e=>new Float32Array([e.amplitudeX??.02,e.amplitudeY??.02,e.frequencyX??5,e.frequencyY??5])},a1={id:"gpu-trigger-wave",name:"Trigger Wave",category:"distort",entryPoint:"triggerWaveFragment",uniformSize:64,shader:`
struct TriggerWaveParams {
  settingsA: vec4f,
  settingsB: vec4f,
  settingsC: vec4f,
  settingsD: vec4f,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: TriggerWaveParams;

@fragment
fn triggerWaveFragment(input: VertexOutput) -> @location(0) vec4f {
  let strength = params.settingsA.x;
  let radius = max(params.settingsA.y, 0.001);
  let frequency = max(params.settingsA.z, 0.001);
  let decay = max(params.settingsA.w, 0.001);

  let center = vec2f(params.settingsB.x, params.settingsB.y);
  let phase = fract(params.settingsB.z + params.settingsB.w * params.settingsC.z);
  let chroma = params.settingsC.x;
  let scanlineMix = clamp(params.settingsC.y, 0.0, 1.0);
  let aspect = max(params.settingsC.w, 0.001);
  let glowColor = params.settingsD.rgb * params.settingsD.a;

  let aspectDelta = vec2f((input.uv.x - center.x) * aspect, input.uv.y - center.y);
  let dist = length(aspectDelta);
  let safeDist = max(dist, 0.0001);
  let direction = vec2f(aspectDelta.x / aspect, aspectDelta.y) / safeDist;

  let ringRadius = phase * radius;
  let band = exp(-abs(dist - ringRadius) / decay);
  let tail = 1.0 - smoothstep(0.2, 1.0, phase);
  let carrier = sin((dist - ringRadius) * frequency * TAU);
  let force = carrier * band * tail * strength;
  let warpedUv = input.uv + direction * force;

  var color = textureSample(inputTex, texSampler, warpedUv);
  if (chroma > 0.0) {
    let chromaOffset = direction * chroma * band * (0.25 + abs(strength) * 20.0);
    let red = textureSample(inputTex, texSampler, warpedUv + chromaOffset).r;
    let blue = textureSample(inputTex, texSampler, warpedUv - chromaOffset).b;
    color = vec4f(red, color.g, blue, color.a);
  }

  if (scanlineMix > 0.0) {
    let line = 0.78 + 0.22 * sin(input.position.y * 2.4 + phase * TAU * 8.0);
    color = vec4f(mix(color.rgb, color.rgb * line, scanlineMix), color.a);
  }

  let glow = band * tail * clamp(abs(strength) * 12.0, 0.0, 1.0);
  color = vec4f(color.rgb + glowColor * glow, color.a);
  return vec4f(clamp(color.rgb, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{strength:{type:"number",label:"Strength",default:.035,min:-.15,max:.15,step:.001,animatable:!0},radius:{type:"number",label:"Radius",default:.85,min:.1,max:1.5,step:.01,animatable:!0},frequency:{type:"number",label:"Frequency",default:18,min:2,max:64,step:1,animatable:!0},decay:{type:"number",label:"Decay",default:.08,min:.01,max:.3,step:.01,animatable:!0},phase:{type:"number",label:"Phase",default:0,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:0,max:4,step:.1,animatable:!1},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},chroma:{type:"number",label:"Chroma",default:.006,min:0,max:.05,step:.001,animatable:!0},scanlineMix:{type:"number",label:"Scanlines",default:.18,min:0,max:1,step:.01,animatable:!0},glowColor:{type:"color",label:"Glow Color",default:"#2e6b8c",animatable:!0}},packUniforms:(e,t,r)=>{const n=performance.now()/1e3,a=Gt(e.glowColor??"#2e6b8c",[.18,.42,.55,1]);return new Float32Array([e.strength??.035,e.radius??.85,e.frequency??18,e.decay??.08,e.centerX??.5,e.centerY??.5,e.phase??0,e.speed??1,e.chroma??.006,e.scanlineMix??.18,n,t/Math.max(r,1),a[0],a[1],a[2],a[3]])}},i1={id:"gpu-bulge",name:"Bulge/Pinch",category:"distort",entryPoint:"bulgeFragment",uniformSize:16,shader:`
struct BulgeParams { amount: f32, radius: f32, centerX: f32, centerY: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: BulgeParams;
@fragment
fn bulgeFragment(input: VertexOutput) -> @location(0) vec4f {
  let center = vec2f(params.centerX, params.centerY);
  let delta = input.uv - center;
  let dist = length(delta);
  let safeDist = max(dist, 0.0001);
  let normalizedDist = safeDist / params.radius;
  let factor = pow(normalizedDist, params.amount);
  let newDist = factor * params.radius;
  let direction = delta / safeDist;
  let bulgedUV = center + direction * newDist;
  let inRadius = dist < params.radius && dist > 0.0;
  let finalUV = select(input.uv, bulgedUV, inRadius);
  return textureSample(inputTex, texSampler, finalUV);
}`,params:{amount:{type:"number",label:"Amount",default:.5,min:.1,max:3,step:.1,animatable:!0},radius:{type:"number",label:"Radius",default:.5,min:.1,max:1,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.5,e.radius??.5,e.centerX??.5,e.centerY??.5])},o1={id:"gpu-kaleidoscope",name:"Kaleidoscope",category:"distort",entryPoint:"kaleidoscopeFragment",uniformSize:16,shader:`
struct KaleidoscopeParams { segments: f32, rotation: f32, _p1: f32, _p2: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: KaleidoscopeParams;
@fragment
fn kaleidoscopeFragment(input: VertexOutput) -> @location(0) vec4f {
  var uv = input.uv - 0.5;
  let angle = atan2(uv.y, uv.x) + params.rotation;
  let radius = length(uv);
  let segmentAngle = TAU / params.segments;
  var a = fract(angle / segmentAngle) * segmentAngle;
  if (a > segmentAngle * 0.5) { a = segmentAngle - a; }
  uv = vec2f(cos(a), sin(a)) * radius + 0.5;
  return textureSample(inputTex, texSampler, uv);
}`,params:{segments:{type:"number",label:"Segments",default:6,min:2,max:16,step:1,animatable:!0},rotation:{type:"number",label:"Rotation",default:0,min:0,max:6.28318,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.segments??6,e.rotation??0,0,0])},s1={id:"gpu-mirror",name:"Mirror",category:"distort",entryPoint:"mirrorFragment",uniformSize:16,shader:`
struct MirrorParams { horizontal: f32, vertical: f32, _p1: f32, _p2: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: MirrorParams;
@fragment
fn mirrorFragment(input: VertexOutput) -> @location(0) vec4f {
  var uv = input.uv;
  if (params.horizontal > 0.5 && uv.x > 0.5) { uv.x = 1.0 - uv.x; }
  if (params.vertical > 0.5 && uv.y > 0.5) { uv.y = 1.0 - uv.y; }
  return textureSample(inputTex, texSampler, uv);
}`,params:{horizontal:{type:"boolean",label:"Horizontal",default:!0},vertical:{type:"boolean",label:"Vertical",default:!1}},packUniforms:e=>new Float32Array([e.horizontal!==!1?1:0,e.vertical===!0?1:0,0,0])},l1={id:"gpu-fluted-glass",name:"Fluted Glass",category:"distort",entryPoint:"flutedGlassFragment",uniformSize:128,shader:`
struct FlutedGlassParams {
  colorBack: vec4f,
  colorShadow: vec4f,
  colorHighlight: vec4f,
  settingsA: vec4f,
  settingsB: vec4f,
  settingsC: vec4f,
  settingsD: vec4f,
  margins: vec4f,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: FlutedGlassParams;

fn rotate2d(p: vec2f, angle: f32) -> vec2f {
  let c = cos(angle);
  let s = sin(angle);
  return vec2f(p.x * c - p.y * s, p.x * s + p.y * c);
}

fn rotateAspect(p: vec2f, angle: f32, aspect: f32) -> vec2f {
  var q = p;
  q.x *= aspect;
  q = rotate2d(q, angle);
  q.x /= aspect;
  return q;
}

fn smoothFract(x: f32) -> f32 {
  let f = fract(x);
  let w = fwidth(x);
  let edge = abs(f - 0.5) - 0.5;
  let band = smoothstep(-w, w, edge);
  return mix(f, 1.0 - f, band);
}

fn getUvFrame(uv: vec2f, softness: f32) -> f32 {
  let aax = 2.0 * fwidth(uv.x);
  let aay = 2.0 * fwidth(uv.y);
  let left = smoothstep(0.0, aax + softness, uv.x);
  let right = 1.0 - smoothstep(1.0 - softness - aax, 1.0, uv.x);
  let bottom = smoothstep(0.0, aay + softness, uv.y);
  let top = 1.0 - smoothstep(1.0 - softness - aay, 1.0, uv.y);
  return left * right * bottom * top;
}

fn samplePremultiplied(uv: vec2f) -> vec4f {
  let c = textureSampleLevel(inputTex, texSampler, uv, 0.0);
  return vec4f(c.rgb * c.a, c.a);
}

fn getBlur(uv: vec2f, texelSize: vec2f, dir: vec2f, sigma: f32) -> vec4f {
  if (sigma <= 0.5) {
    return textureSampleLevel(inputTex, texSampler, uv, 0.0);
  }

  let maxRadius = 50;
  let radius = i32(min(f32(maxRadius), ceil(3.0 * sigma)));
  let twoSigma2 = 2.0 * sigma * sigma;
  let gaussianNorm = 1.0 / sqrt(TAU * sigma * sigma);

  var sum = samplePremultiplied(uv) * gaussianNorm;
  var weightSum = gaussianNorm;

  for (var i = 1; i <= maxRadius; i = i + 1) {
    if (i > radius) {
      break;
    }

    let x = f32(i);
    let w = exp(-(x * x) / twoSigma2) * gaussianNorm;
    let offset = dir * texelSize * x;
    let s1 = samplePremultiplied(uv + offset);
    let s2 = samplePremultiplied(uv - offset);
    sum += (s1 + s2) * w;
    weightSum += 2.0 * w;
  }

  let result = sum / weightSum;
  if (result.a > 0.0) {
    return vec4f(result.rgb / result.a, result.a);
  }
  return result;
}

fn flutedOverlayNoise(p: vec2f) -> f32 {
  let coarse = noise2d(p * 0.83 + vec2f(4.1, -7.3));
  let medium = noise2d(vec2f(
    p.x * 1.27 - p.y * 0.58,
    p.x * 0.71 + p.y * 1.19
  ) + vec2f(-10.2, 5.4));
  let fine = noise2d(vec2f(
    p.x * -0.92 + p.y * 1.11,
    p.x * -1.06 - p.y * 0.82
  ) + vec2f(7.8, 9.6));
  return coarse * 0.45 + medium * 0.35 + fine * 0.2;
}

@fragment
fn flutedGlassFragment(input: VertexOutput) -> @location(0) vec4f {
  let width = max(params.settingsD.y, 1.0);
  let height = max(params.settingsD.z, 1.0);
  let aspect = max(params.settingsD.w, 0.0001);

  let size = clamp(params.settingsA.x, 0.0, 1.0);
  let shadowsAmount = clamp(params.settingsA.y, 0.0, 1.0);
  let angle = params.settingsA.z * PI / 180.0;
  let stretchAmount = clamp(params.settingsA.w, 0.0, 1.0);

  let shape = i32(params.settingsB.x);
  let distortionAmount = clamp(params.settingsB.y, 0.0, 1.0);
  let highlightsAmount = clamp(params.settingsB.z, 0.0, 1.0);
  let distortionShape = i32(params.settingsB.w);

  let shiftAmount = params.settingsC.x;
  let blurAmount = clamp(params.settingsC.y, 0.0, 1.0);
  let edgesAmount = clamp(params.settingsC.z, 0.0, 1.0);
  let grainMixer = clamp(params.settingsC.w, 0.0, 1.0);
  let grainOverlay = clamp(params.settingsD.x, 0.0, 1.0);

  let marginLeft = params.margins.x;
  let marginTop = params.margins.y;
  let marginRight = params.margins.z;
  let marginBottom = params.margins.w;

  let patternRotation = -angle;
  let patternSize = mix(200.0, 5.0, size);

  var uv = input.uv;
  let uvMask = input.position.xy / vec2f(width, height);
  let sw = vec2f(0.005);
  let mask =
    smoothstep(marginLeft, marginLeft + sw.x, uvMask.x + sw.x) *
    smoothstep(marginRight, marginRight + sw.x, 1.0 - uvMask.x + sw.x) *
    smoothstep(marginTop, marginTop + sw.y, uvMask.y + sw.y) *
    smoothstep(marginBottom, marginBottom + sw.y, 1.0 - uvMask.y + sw.y);
  let maskOuter =
    smoothstep(marginLeft - sw.x, marginLeft, uvMask.x + sw.x) *
    smoothstep(marginRight - sw.x, marginRight, 1.0 - uvMask.x + sw.x) *
    smoothstep(marginTop - sw.y, marginTop, uvMask.y + sw.y) *
    smoothstep(marginBottom - sw.y, marginBottom, 1.0 - uvMask.y + sw.y);
  let maskStroke = maskOuter - mask;
  let maskInner =
    smoothstep(marginLeft - 2.0 * sw.x, marginLeft, uvMask.x) *
    smoothstep(marginRight - 2.0 * sw.x, marginRight, 1.0 - uvMask.x) *
    smoothstep(marginTop - 2.0 * sw.y, marginTop, uvMask.y) *
    smoothstep(marginBottom - 2.0 * sw.y, marginBottom, 1.0 - uvMask.y);
  let maskStrokeInner = maskInner - mask;

  uv -= 0.5;
  uv *= patternSize;
  uv = rotateAspect(uv, patternRotation, aspect);

  var curve = 0.0;
  let patternY = uv.y / aspect;
  if (shape == 5) {
    curve = 0.5 + 0.5 * sin(0.5 * PI * uv.x) * cos(0.5 * PI * patternY);
  } else if (shape == 4) {
    curve = 10.0 * abs(fract(0.1 * patternY) - 0.5);
  } else if (shape == 3) {
    curve = 4.0 * sin(0.23 * patternY);
  } else if (shape == 2) {
    curve = 0.5 + 0.5 * sin(0.5 * uv.x) * sin(1.7 * uv.x);
  }

  let uvToFract = uv + curve;
  var fractOrigUV = fract(uv);
  var floorOrigUV = floor(uv);
  var x = smoothFract(uvToFract.x);
  let xNonSmooth = fract(uvToFract.x) + 0.0001;

  var highlightsWidth = 2.0 * max(0.001, fwidth(uvToFract.x));
  highlightsWidth += 2.0 * maskStrokeInner;
  var highlights = smoothstep(0.0, highlightsWidth, xNonSmooth);
  highlights *= smoothstep(1.0, 1.0 - highlightsWidth, xNonSmooth);
  highlights = 1.0 - highlights;
  highlights *= highlightsAmount;
  highlights = clamp(highlights, 0.0, 1.0);
  highlights *= mask;

  var shadows = pow(x, 1.3);
  var distortion = 0.0;
  var fadeX = 1.0;
  var frameFade = 0.0;

  var aa = fwidth(xNonSmooth);
  aa = max(aa, fwidth(uv.x));
  aa = max(aa, fwidth(uvToFract.x));
  aa = max(aa, 0.0001);

  if (distortionShape == 1) {
    distortion = -pow(1.5 * x, 3.0);
    distortion += 0.5 - shiftAmount;
    frameFade = pow(1.5 * x, 3.0);
    aa = max(0.2, aa);
    aa += mix(0.2, 0.0, size);
    fadeX = smoothstep(0.0, aa, xNonSmooth) * smoothstep(1.0, 1.0 - aa, xNonSmooth);
    distortion = mix(0.5, distortion, fadeX);
  } else if (distortionShape == 2) {
    distortion = 2.0 * pow(x, 2.0);
    distortion -= 0.5 + shiftAmount;
    frameFade = pow(abs(x - 0.5), 4.0);
    aa = max(0.2, aa);
    aa += mix(0.2, 0.0, size);
    fadeX = smoothstep(0.0, aa, xNonSmooth) * smoothstep(1.0, 1.0 - aa, xNonSmooth);
    distortion = mix(0.5, distortion, fadeX);
    frameFade = mix(1.0, frameFade, 0.5 * fadeX);
  } else if (distortionShape == 3) {
    distortion = pow(2.0 * (xNonSmooth - 0.5), 6.0);
    distortion -= 0.25;
    distortion -= shiftAmount;
    frameFade = 1.0 - 2.0 * pow(abs(x - 0.4), 2.0);
    aa = 0.15;
    aa += mix(0.1, 0.0, size);
    fadeX = smoothstep(0.0, aa, xNonSmooth) * smoothstep(1.0, 1.0 - aa, xNonSmooth);
    frameFade = mix(1.0, frameFade, fadeX);
  } else if (distortionShape == 4) {
    x = xNonSmooth;
    distortion = sin((x + 0.25) * TAU);
    shadows = 0.5 + 0.5 * asin(distortion) / (0.5 * PI);
    distortion *= 0.5;
    distortion -= shiftAmount;
    frameFade = 0.5 + 0.5 * sin(x * TAU);
  } else if (distortionShape == 5) {
    distortion -= pow(abs(x), 0.2) * x;
    distortion += 0.33;
    distortion -= 3.0 * shiftAmount;
    distortion *= 0.33;
    frameFade = 0.3 * smoothstep(0.0, 1.0, x);
    shadows = pow(x, 2.5);
    aa = max(0.1, aa);
    aa += mix(0.1, 0.0, size);
    fadeX = smoothstep(0.0, aa, xNonSmooth) * smoothstep(1.0, 1.0 - aa, xNonSmooth);
    distortion *= fadeX;
  }

  // Grain UV (and its derivatives) are only needed when grain or grain overlay
  // are active. grainMixer/grainOverlay are uniform, so these branches are
  // coherent across the draw — no divergence — and the dpdx/dpdy derivatives
  // stay in uniform control flow. Skips ~36 sin-based hash() calls per pixel
  // when grain is off (the common case), which dominates this shader's ALU.
  let grainActive = grainMixer > 0.0 || grainOverlay > 0.0;
  var grainUV = input.uv;
  if (grainActive) {
    let dudx = dpdx(input.uv);
    let dudy = dpdy(input.uv);
    var gUV = input.uv - 0.5;
    let derivativeScale = 0.8 / max(vec2f(length(dudx), length(dudy)), vec2f(0.0001));
    gUV *= derivativeScale;
    gUV += 0.5;
    grainUV = gUV;
  }
  if (grainMixer > 0.0) {
    var grain = flutedOverlayNoise(grainUV);
    grain = smoothstep(0.4, 0.7, grain);
    grain *= grainMixer;
    distortion = mix(distortion, 0.0, grain);
  }

  shadows = min(shadows, 1.0);
  shadows += maskStrokeInner;
  shadows *= mask;
  shadows = min(shadows, 1.0);
  shadows *= pow(shadowsAmount, 2.0);
  shadows = clamp(shadows, 0.0, 1.0);

  distortion *= 3.0 * distortionAmount;
  frameFade *= distortionAmount;

  fractOrigUV = vec2f(fractOrigUV.x + distortion, fractOrigUV.y);
  floorOrigUV = rotateAspect(floorOrigUV, -patternRotation, aspect);
  fractOrigUV = rotateAspect(fractOrigUV, -patternRotation, aspect);

  uv = (floorOrigUV + fractOrigUV) / patternSize;
  uv += vec2f(pow(maskStroke, 4.0));
  uv += 0.5;

  uv = mix(input.uv, uv, smoothstep(0.0, 0.7, mask));
  var blur = mix(0.0, 50.0, blurAmount);
  blur = mix(0.0, blur, smoothstep(0.5, 1.0, mask));

  var edgeDistortion = mix(0.0, 0.04, edgesAmount);
  edgeDistortion += 0.06 * frameFade * edgesAmount;
  edgeDistortion *= mask;
  let frame = getUvFrame(uv, edgeDistortion);

  // stretchAmount is uniform — skip the stretch warp (and its getUvFrame /
  // fwidth work) entirely when stretch is off.
  if (stretchAmount > 0.0) {
    var stretch = 1.0 - smoothstep(0.0, 0.5, xNonSmooth) * smoothstep(1.0, 0.5, xNonSmooth);
    stretch = pow(stretch, 2.0);
    stretch *= mask;
    stretch *= getUvFrame(uv, 0.1 + 0.05 * mask * frameFade);
    uv = vec2f(uv.x, mix(uv.y, 0.5, stretchAmount * stretch));
  }

  let imageSample = getBlur(uv, 1.0 / vec2f(width, height), vec2f(0.0, 1.0), blur);
  let image = vec4f(imageSample.rgb * imageSample.a, imageSample.a);
  let backColor = vec4f(params.colorBack.rgb * params.colorBack.a, params.colorBack.a);
  let highlightColor = vec4f(params.colorHighlight.rgb * params.colorHighlight.a, params.colorHighlight.a);
  let shadowColor = params.colorShadow;

  var color = highlightColor.rgb * highlights;
  var opacity = highlightColor.a * highlights;

  shadows = mix(shadows * shadowColor.a, 0.0, highlights);
  color = mix(color, shadowColor.rgb * shadowColor.a, 0.5 * shadows);
  color += 0.5 * pow(shadows, 0.5) * shadowColor.rgb;
  opacity += shadows;
  color = clamp(color, vec3f(0.0), vec3f(1.0));
  opacity = clamp(opacity, 0.0, 1.0);

  color += image.rgb * (1.0 - opacity) * frame;
  opacity += image.a * (1.0 - opacity) * frame;
  color += backColor.rgb * (1.0 - opacity);
  opacity += backColor.a * (1.0 - opacity);

  // grainOverlay is uniform — the two-octave overlay noise (24 sin-based
  // hash() calls) only runs when the overlay is actually dialed in.
  if (grainOverlay > 0.0) {
    var grainOverlayNoise = flutedOverlayNoise(rotate2d(grainUV, 1.0) + vec2f(3.0));
    grainOverlayNoise = mix(grainOverlayNoise, flutedOverlayNoise(rotate2d(grainUV, 2.0) + vec2f(-1.0)), 0.5);
    grainOverlayNoise = pow(grainOverlayNoise, 1.3);

    let grainOverlayV = grainOverlayNoise * 2.0 - 1.0;
    let grainOverlayColor = vec3f(select(0.0, 1.0, grainOverlayV >= 0.0));
    var grainOverlayStrength = grainOverlay * abs(grainOverlayV);
    grainOverlayStrength = pow(grainOverlayStrength, 0.8);
    grainOverlayStrength *= mask;
    color = mix(color, grainOverlayColor, 0.35 * grainOverlayStrength);
    opacity += 0.5 * grainOverlayStrength;
  }
  opacity = clamp(opacity, 0.0, 1.0);

  return vec4f(color, opacity);
}`,params:{colorBack:{type:"color",label:"Back Color",default:"#00000000",animatable:!0},colorShadow:{type:"color",label:"Shadow Color",default:"#000000",animatable:!0},colorHighlight:{type:"color",label:"Highlight Color",default:"#ffffff",animatable:!0},shadows:{type:"number",label:"Shadows",default:.25,min:0,max:1,step:.01,animatable:!0},highlights:{type:"number",label:"Highlights",default:.1,min:0,max:1,step:.01,animatable:!0},size:{type:"number",label:"Size",default:.5,min:0,max:1,step:.01,animatable:!0},shape:{type:"select",label:"Shape",default:"lines",options:[{value:"lines",label:"Lines"},{value:"linesIrregular",label:"Irregular Lines"},{value:"wave",label:"Wave"},{value:"zigzag",label:"Zigzag"},{value:"pattern",label:"Pattern"}]},angle:{type:"number",label:"Angle",default:0,min:0,max:180,step:1,animatable:!0},distortionShape:{type:"select",label:"Distortion Shape",default:"prism",options:[{value:"prism",label:"Prism"},{value:"lens",label:"Lens"},{value:"contour",label:"Contour"},{value:"cascade",label:"Cascade"},{value:"flat",label:"Flat"}]},distortion:{type:"number",label:"Distortion",default:.5,min:0,max:1,step:.01,animatable:!0},shift:{type:"number",label:"Shift",default:0,min:-1,max:1,step:.01,animatable:!0},stretch:{type:"number",label:"Stretch",default:0,min:0,max:1,step:.01,animatable:!0},blur:{type:"number",label:"Blur",default:0,min:0,max:1,step:.01,animatable:!0},edges:{type:"number",label:"Edges",default:.25,min:0,max:1,step:.01,animatable:!0},margin:{type:"number",label:"Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginLeft:{type:"number",label:"Left Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginRight:{type:"number",label:"Right Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginTop:{type:"number",label:"Top Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginBottom:{type:"number",label:"Bottom Margin",default:0,min:0,max:1,step:.01,animatable:!0},grainMixer:{type:"number",label:"Grain Mixer",default:0,min:0,max:1,step:.01,animatable:!0},grainOverlay:{type:"number",label:"Grain Overlay",default:0,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n={lines:1,linesIrregular:2,wave:3,zigzag:4,pattern:5},a={prism:1,lens:2,contour:3,cascade:4,flat:5},i=e.margin??0,o=e.marginLeft??i,s=e.marginRight??i,l=e.marginTop??i,c=e.marginBottom??i,u=Gt(e.colorBack??"#00000000",[0,0,0,0]),d=Gt(e.colorShadow??"#000000",[0,0,0,1]),m=Gt(e.colorHighlight??"#ffffff",[1,1,1,1]);return new Float32Array([u[0],u[1],u[2],u[3],d[0],d[1],d[2],d[3],m[0],m[1],m[2],m[3],e.size??.5,e.shadows??.25,e.angle??0,e.stretch??0,n[e.shape]??1,e.distortion??.5,e.highlights??.1,a[e.distortionShape]??1,e.shift??0,e.blur??0,e.edges??.25,e.grainMixer??0,e.grainOverlay??0,t,r,t/Math.max(r,1),o,l,s,c])}},c1={id:"gpu-ripple-glass",name:"Ripple Glass",category:"distort",entryPoint:"rippleGlassFragment",uniformSize:80,shader:`
struct RippleGlassParams {
  colorShadow: vec4f,
  colorHighlight: vec4f,
  settingsA: vec4f,
  settingsB: vec4f,
  settingsC: vec4f,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: RippleGlassParams;

@fragment
fn rippleGlassFragment(input: VertexOutput) -> @location(0) vec4f {
  let amount = params.settingsA.x;
  let rings = max(params.settingsA.y, 1.0);
  let shadowsAmount = clamp(params.settingsA.z, 0.0, 1.0);
  let highlightsAmount = clamp(params.settingsA.w, 0.0, 1.0);

  let origin = vec2f(params.settingsB.x, params.settingsB.y);
  let phase = params.settingsB.z;
  let falloff = max(params.settingsB.w, 0.001);

  let aberration = params.settingsC.x;
  let aspect = max(params.settingsC.w, 0.0001);

  // Aspect-corrected radial vector from the ripple origin.
  var p = input.uv - origin;
  p.x *= aspect;
  let dist = length(p);
  let dir = p / max(dist, 1e-4);
  // Radial offset expressed back in uv space (undo the aspect scaling on x).
  let radialUv = vec2f(dir.x / aspect, dir.y);

  let ringWidth = 1.0 / rings;
  let ringCoord = dist / ringWidth - phase;   // integer part = ring index
  let x = fract(ringCoord);                     // 0..1 within the ring
  let centered = x - 0.5;

  // Cylindrical lens bend: soft at the ring centre, steep toward the seams,
  // pulling samples back toward each ring centre (magnifying the band).
  let bend = -sign(centered) * pow(abs(centered) * 2.0, 1.5);

  // Reach envelope — fades the ripple away from the origin.
  let envelope = exp(-dist / falloff);

  let push = bend * amount * ringWidth * 1.5 * envelope;
  let offsetUv = radialUv * push;

  var color: vec4f;
  if (aberration > 0.0) {
    let ca = radialUv * aberration * ringWidth * envelope;
    let r = textureSample(inputTex, texSampler, input.uv + offsetUv + ca).r;
    let g = textureSample(inputTex, texSampler, input.uv + offsetUv).g;
    let b = textureSample(inputTex, texSampler, input.uv + offsetUv - ca).b;
    let a = textureSample(inputTex, texSampler, input.uv + offsetUv).a;
    color = vec4f(r, g, b, a);
  } else {
    color = textureSample(inputTex, texSampler, input.uv + offsetUv);
  }

  // Thin bright seam between rings + groove shadow that deepens toward it.
  let aa = 2.0 * max(0.001, fwidth(ringCoord));
  var highlights = 1.0 - (smoothstep(0.0, aa, x) * smoothstep(1.0, 1.0 - aa, x));
  highlights = clamp(highlights * highlightsAmount * envelope, 0.0, 1.0);

  var shadows = pow(abs(centered) * 2.0, 1.3);
  shadows = clamp(shadows * shadowsAmount * envelope, 0.0, 1.0);

  let shadowColor = params.colorShadow;
  let highlightColor = params.colorHighlight;

  var rgb = color.rgb;
  rgb = mix(rgb, shadowColor.rgb, 0.5 * shadows * shadowColor.a);
  rgb += highlightColor.rgb * highlights * highlightColor.a;
  rgb = clamp(rgb, vec3f(0.0), vec3f(1.0));

  return vec4f(rgb, color.a);
}`,params:{colorShadow:{type:"color",label:"Shadow Color",default:"#000000",animatable:!0},colorHighlight:{type:"color",label:"Highlight Color",default:"#ffffff",animatable:!0},amount:{type:"number",label:"Amount",default:.5,min:0,max:1,step:.01,animatable:!0},rings:{type:"number",label:"Rings",default:14,min:1,max:64,step:1,animatable:!0},shadows:{type:"number",label:"Shadows",default:.25,min:0,max:1,step:.01,animatable:!0},highlights:{type:"number",label:"Highlights",default:.1,min:0,max:1,step:.01,animatable:!0},originX:{type:"number",label:"Origin X",default:.5,min:0,max:1,step:.01,animatable:!0},originY:{type:"number",label:"Origin Y",default:.5,min:0,max:1,step:.01,animatable:!0},phase:{type:"number",label:"Phase",default:0,min:-1,max:1,step:.01,animatable:!0},falloff:{type:"number",label:"Falloff",default:.35,min:.05,max:2,step:.01,animatable:!0},aberration:{type:"number",label:"Aberration",default:0,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n=Gt(e.colorShadow??"#000000",[0,0,0,1]),a=Gt(e.colorHighlight??"#ffffff",[1,1,1,1]);return new Float32Array([n[0],n[1],n[2],n[3],a[0],a[1],a[2],a[3],e.amount??.5,e.rings??14,e.shadows??.25,e.highlights??.1,e.originX??.5,e.originY??.5,e.phase??0,e.falloff??.35,e.aberration??0,t,r,t/Math.max(r,1)])}},u1={id:"gpu-glass-mosaic",name:"Glass Mosaic",category:"distort",entryPoint:"glassMosaicFragment",uniformSize:64,shader:`
struct GlassMosaicParams {
  colorShadow: vec4f,
  colorHighlight: vec4f,
  settingsA: vec4f,
  settingsB: vec4f,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: GlassMosaicParams;

@fragment
fn glassMosaicFragment(input: VertexOutput) -> @location(0) vec4f {
  let amount = params.settingsA.x;
  let cells = max(params.settingsA.y, 1.0);
  let shadowsAmount = clamp(params.settingsA.z, 0.0, 1.0);
  let highlightsAmount = clamp(params.settingsA.w, 0.0, 1.0);
  let aberration = params.settingsB.x;
  let aspect = max(params.settingsB.w, 0.0001);

  // Square-ish cells: cellsX counts columns across the width; rows scale by
  // aspect so each tile stays roughly square regardless of frame proportions.
  let cellUvSize = vec2f(1.0 / cells, aspect / cells);
  let grid = input.uv / cellUvSize;
  let local = fract(grid) - 0.5;                 // -0.5..0.5 within the cell
  let localUv = local * cellUvSize;              // same offset, in uv space

  // Spherical lens: magnify toward each cell centre, fading out near the rim.
  let dd = dot(local * 2.0, local * 2.0);        // 0 centre .. up to 2 at corners
  let lens = amount * pow(clamp(1.0 - dd, 0.0, 1.0), 0.5);
  let sampleUv = input.uv - localUv * lens;

  var color: vec4f;
  if (aberration > 0.0) {
    let ca = localUv * aberration;
    let r = textureSample(inputTex, texSampler, sampleUv - ca).r;
    let g = textureSample(inputTex, texSampler, sampleUv).g;
    let b = textureSample(inputTex, texSampler, sampleUv + ca).b;
    let a = textureSample(inputTex, texSampler, sampleUv).a;
    color = vec4f(r, g, b, a);
  } else {
    color = textureSample(inputTex, texSampler, sampleUv);
  }

  // Rounded-square edge factor: 0 at the cell centre, 1 at the rim.
  let edge = max(abs(local.x), abs(local.y)) * 2.0;
  let fw = fwidth(edge) + 0.001;

  // Bright bevel just inside each cell border.
  var highlights = smoothstep(1.0 - 6.0 * fw, 1.0 - 2.0 * fw, edge);
  highlights *= highlightsAmount;

  // Darker mortar at the seams + a gentle vignette toward the rim.
  let gap = smoothstep(1.0 - 2.0 * fw, 1.0, edge);
  var shadows = pow(edge, 3.0) * 0.5 + gap;
  shadows = clamp(shadows * shadowsAmount, 0.0, 1.0);

  // Diagonal bevel (light from top-left) gives each tile a glassy roundness.
  let bevel = (-local.x - local.y) * amount * 0.5;

  let shadowColor = params.colorShadow;
  let highlightColor = params.colorHighlight;

  var rgb = color.rgb * (1.0 + bevel);
  rgb = mix(rgb, shadowColor.rgb, 0.5 * shadows * shadowColor.a);
  rgb += highlightColor.rgb * highlights * highlightColor.a;
  rgb = clamp(rgb, vec3f(0.0), vec3f(1.0));

  return vec4f(rgb, color.a);
}`,params:{colorShadow:{type:"color",label:"Shadow Color",default:"#000000",animatable:!0},colorHighlight:{type:"color",label:"Highlight Color",default:"#ffffff",animatable:!0},amount:{type:"number",label:"Amount",default:.55,min:0,max:1,step:.01,animatable:!0},cells:{type:"number",label:"Cells",default:18,min:2,max:80,step:1,animatable:!0},shadows:{type:"number",label:"Shadows",default:.3,min:0,max:1,step:.01,animatable:!0},highlights:{type:"number",label:"Highlights",default:.12,min:0,max:1,step:.01,animatable:!0},aberration:{type:"number",label:"Aberration",default:0,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n=Gt(e.colorShadow??"#000000",[0,0,0,1]),a=Gt(e.colorHighlight??"#ffffff",[1,1,1,1]);return new Float32Array([n[0],n[1],n[2],n[3],a[0],a[1],a[2],a[3],e.amount??.55,e.cells??18,e.shadows??.3,e.highlights??.12,e.aberration??0,t,r,t/Math.max(r,1)])}},d1={id:"gpu-blocks",name:"Blocks",category:"distort",entryPoint:"blocksFragment",uniformSize:32,shader:`
struct BlocksParams {
  size: f32, depth: f32, studSize: f32, gap: f32,
  width: f32, height: f32, pad0: f32, pad1: f32
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: BlocksParams;
@fragment
fn blocksFragment(input: VertexOutput) -> @location(0) vec4f {
  let cellX = max(params.size, 1.0) / params.width;
  let cellY = max(params.size, 1.0) / params.height;
  let cellIndex = vec2f(floor(input.uv.x / cellX), floor(input.uv.y / cellY));
  let cellCenter = vec2f((cellIndex.x + 0.5) * cellX, (cellIndex.y + 0.5) * cellY);
  let color = textureSample(inputTex, texSampler, cellCenter);

  // local position within the cell, centered at 0 (range -0.5..0.5)
  let local = vec2f(fract(input.uv.x / cellX), fract(input.uv.y / cellY)) - vec2f(0.5);
  let edge = max(abs(local.x), abs(local.y));

  // bevel: light from top-left, shadow toward bottom-right
  let shade = (-local.x - local.y) * params.depth;

  // raised stud at the cell center with its own bevel
  let studR = clamp(params.studSize, 0.0, 1.0) * 0.4;
  let stud = smoothstep(studR, studR - 0.03, length(local));
  let studShade = stud * ((-local.x - local.y) * params.depth * 2.0 + params.depth * 0.18);

  var rgb = color.rgb * (1.0 + shade) + color.rgb * studShade;

  // darken the mortar gap between blocks
  let gapMask = step(edge, 0.5 - clamp(params.gap, 0.0, 0.4));
  rgb = rgb * mix(0.55, 1.0, gapMask);

  return vec4f(clamp(rgb, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{size:{type:"number",label:"Block Size",default:24,min:4,max:120,step:1,animatable:!0},depth:{type:"number",label:"Depth",default:.5,min:0,max:1.5,step:.01,animatable:!0},studSize:{type:"number",label:"Stud Size",default:.55,min:0,max:1,step:.01,animatable:!0},gap:{type:"number",label:"Gap",default:.06,min:0,max:.4,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.size??24,e.depth??.5,e.studSize??.55,e.gap??.06,t,r,0,0])},m1={id:"gpu-droste",name:"Droste",category:"distort",entryPoint:"drosteFragment",uniformSize:32,shader:`
struct DrosteParams {
  strength: f32, scale: f32, centerX: f32, centerY: f32,
  spin: f32, width: f32, height: f32, pad: f32
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: DrosteParams;
@fragment
fn drosteFragment(input: VertexOutput) -> @location(0) vec4f {
  let aspect = params.width / max(params.height, 1.0);
  let center = vec2f(params.centerX, params.centerY);
  let p = (input.uv - center) * vec2f(aspect, 1.0);

  let r = max(length(p), 1e-4);
  let a = atan2(p.y, p.x);
  var z = vec2f(log(r), a);

  let period = log(max(params.scale, 1.0001));
  // Escher twist; strength dials from plain recursive zoom (0) to full spiral
  let alpha = atan2(period, TAU) * clamp(params.strength, 0.0, 2.0);
  let co = max(cos(alpha), 1e-3);
  let si = sin(alpha);
  z = vec2f(z.x * co - z.y * si, z.x * si + z.y * co) / co;

  // tile the log-radius into a single repeating band
  z.x = z.x - period * floor(z.x / period);

  let er = exp(z.x);
  let na = z.y + params.spin;
  let uv = center + vec2f(cos(na), sin(na)) * er / vec2f(aspect, 1.0);
  return textureSample(inputTex, texSampler, fract(uv));
}`,params:{strength:{type:"number",label:"Spiral",default:1,min:0,max:2,step:.01,animatable:!0},scale:{type:"number",label:"Scale",default:2,min:1.1,max:6,step:.05,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},spin:{type:"number",label:"Spin",default:0,min:-6.28318,max:6.28318,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.strength??1,e.scale??2,e.centerX??.5,e.centerY??.5,e.spin??0,t,r,0])};var p1=Object.freeze({__proto__:null,blocks:d1,bulge:i1,droste:m1,flutedGlass:l1,glassMosaic:u1,kaleidoscope:o1,mirror:s1,pixelate:e1,rgbSplit:t1,rippleGlass:c1,triggerWave:a1,twirl:r1,wave:n1});const f1={id:"gpu-vignette",name:"Vignette",category:"stylize",entryPoint:"vignetteFragment",uniformSize:16,shader:`
struct VignetteParams { amount: f32, size: f32, softness: f32, roundness: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: VignetteParams;
@fragment
fn vignetteFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let center = input.uv - 0.5;
  let aspect = vec2f(1.0, params.roundness);
  let dist = length(center * aspect) * 2.0;
  let vig = 1.0 - smoothstep(params.size, params.size + params.softness, dist);
  let vigColor = mix(vec3f(0.0), color.rgb, mix(1.0, vig, params.amount));
  return vec4f(vigColor, color.a);
}`,params:{amount:{type:"number",label:"Amount",default:.5,min:0,max:1,step:.01,animatable:!0},size:{type:"number",label:"Size",default:.5,min:0,max:1.5,step:.01,animatable:!0},softness:{type:"number",label:"Softness",default:.5,min:0,max:1,step:.01,animatable:!0},roundness:{type:"number",label:"Roundness",default:1,min:.5,max:2,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.5,e.size??.5,e.softness??.5,e.roundness??1])},h1={id:"gpu-grain",name:"Film Grain",category:"stylize",entryPoint:"grainFragment",uniformSize:16,shader:`
struct GrainParams { amount: f32, size: f32, speed: f32, time: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: GrainParams;
fn grainNoise(uv: vec2f, t: f32) -> f32 {
  let seed = uv + vec2f(t * 0.1, t * 0.07);
  return fract(sin(dot(seed, vec2f(12.9898, 78.233))) * 43758.5453);
}
@fragment
fn grainFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let grainUV = input.uv * (100.0 / params.size);
  // Wrap the time seed: the sin() inside grainNoise loses precision once the
  // unbounded (time * speed) term grows large, freezing the grain over long
  // sessions. The 600s period is imperceptible but keeps the seed bounded.
  let gt = params.time * params.speed;
  let noise = grainNoise(grainUV, gt - floor(gt / 600.0) * 600.0) * 2.0 - 1.0;
  let luma = luminance(color.rgb);
  let grainIntensity = params.amount * (1.0 - luma * 0.5);
  let grainColor = color.rgb + vec3f(noise * grainIntensity);
  return vec4f(clamp(grainColor, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:.1,min:0,max:.5,step:.01,animatable:!0},size:{type:"number",label:"Size",default:1,min:.5,max:5,step:.1,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:0,max:5,step:.1,animatable:!1}},packUniforms:e=>{const t=performance.now()/1e3;return new Float32Array([e.amount??.1,e.size??1,e.speed??1,t])}},g1={id:"gpu-sharpen",name:"Sharpen",category:"stylize",entryPoint:"sharpenFragment",uniformSize:16,shader:`
struct SharpenParams { amount: f32, radius: f32, width: f32, height: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: SharpenParams;
@fragment
fn sharpenFragment(input: VertexOutput) -> @location(0) vec4f {
  let texelSize = vec2f(1.0 / params.width, 1.0 / params.height);
  let center = textureSample(inputTex, texSampler, input.uv);
  var blur = vec4f(0.0);
  var totalWeight = 0.0;
  let samples = 3;
  let sigma = params.radius * 0.5 + 0.5;
  for (var x = -samples; x <= samples; x++) {
    for (var y = -samples; y <= samples; y++) {
      let offset = vec2f(f32(x), f32(y)) * texelSize * params.radius;
      let distSq = f32(x * x + y * y);
      let weight = exp(-distSq / (2.0 * sigma * sigma));
      blur += textureSample(inputTex, texSampler, input.uv + offset) * weight;
      totalWeight += weight;
    }
  }
  blur /= totalWeight;
  let sharpened = center.rgb + (center.rgb - blur.rgb) * params.amount;
  return vec4f(clamp(sharpened, vec3f(0.0), vec3f(1.0)), center.a);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:5,step:.1,animatable:!0},radius:{type:"number",label:"Radius",default:1,min:.5,max:5,step:.1,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.amount??1,e.radius??1,t,r])},y1={id:"gpu-posterize",name:"Posterize",category:"stylize",entryPoint:"posterizeFragment",uniformSize:16,shader:`
struct PosterizeParams { levels: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: PosterizeParams;
@fragment
fn posterizeFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let levels = max(params.levels, 2.0);
  let posterized = floor(color.rgb * levels) / (levels - 1.0);
  return vec4f(posterized, color.a);
}`,params:{levels:{type:"number",label:"Levels",default:6,min:2,max:32,step:1,animatable:!0}},packUniforms:e=>new Float32Array([e.levels??6,0,0,0])},v1={id:"gpu-glow",name:"Glow",category:"stylize",entryPoint:"glowFragment",uniformSize:32,shader:`
struct GlowParams {
  amount: f32, threshold: f32, radius: f32, softness: f32,
  width: f32, height: f32, rings: f32, samplesPerRing: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: GlowParams;
@fragment
fn glowFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let texelSize = vec2f(1.0 / params.width, 1.0 / params.height);
  var glowVal = vec3f(0.0);
  var totalWeight = 0.0;
  let rings = i32(clamp(params.rings, 1.0, 32.0));
  let samplesPerRing = i32(clamp(params.samplesPerRing, 4.0, 64.0));
  for (var ring = 1; ring <= rings; ring++) {
    let ringRadius = f32(ring) * params.radius * texelSize.x * 10.0;
    let ringWeight = gaussian(f32(ring) / f32(rings), params.softness + 0.3);
    for (var i = 0; i < samplesPerRing; i++) {
      let angle = f32(i) * TAU / f32(samplesPerRing) + f32(ring) * 0.5;
      let offset = vec2f(cos(angle), sin(angle)) * ringRadius;
      let sampleColor = textureSample(inputTex, texSampler, input.uv + offset);
      let sampleLuma = luminance(sampleColor.rgb);
      let brightFactor = smoothstep(params.threshold - 0.1, params.threshold + 0.1, sampleLuma);
      let brightColor = sampleColor.rgb * brightFactor;
      glowVal += brightColor * ringWeight;
      totalWeight += ringWeight;
    }
  }
  let centerLuma = luminance(color.rgb);
  let centerBright = smoothstep(params.threshold - 0.1, params.threshold + 0.1, centerLuma);
  glowVal += color.rgb * centerBright * 2.0;
  totalWeight += 2.0;
  glowVal /= totalWeight;
  let result = color.rgb + glowVal * params.amount * 2.0;
  return vec4f(clamp(result, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:5,step:.1,animatable:!0},threshold:{type:"number",label:"Threshold",default:.6,min:0,max:1,step:.01,animatable:!0},radius:{type:"number",label:"Radius",default:20,min:1,max:100,step:1,animatable:!0},softness:{type:"number",label:"Softness",default:.5,min:.1,max:1,step:.05,animatable:!0},rings:{type:"number",label:"Rings",default:4,min:1,max:32,step:1,animatable:!1,quality:!0},samplesPerRing:{type:"number",label:"Samples/Ring",default:16,min:4,max:64,step:1,animatable:!1,quality:!0}},packUniforms:(e,t,r)=>new Float32Array([e.amount??1,e.threshold??.6,e.radius??20,e.softness??.5,t,r,e.rings??4,e.samplesPerRing??16])},b1={id:"gpu-edge-detect",name:"Edge Detect",category:"stylize",entryPoint:"edgeDetectFragment",uniformSize:16,shader:`
struct EdgeDetectParams { strength: f32, width: f32, height: f32, invertFlag: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: EdgeDetectParams;
@fragment
fn edgeDetectFragment(input: VertexOutput) -> @location(0) vec4f {
  let texelSize = vec2f(1.0 / params.width, 1.0 / params.height);
  let tl = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(-texelSize.x, -texelSize.y)).rgb);
  let t  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(0.0, -texelSize.y)).rgb);
  let tr = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(texelSize.x, -texelSize.y)).rgb);
  let l  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(-texelSize.x, 0.0)).rgb);
  let r  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(texelSize.x, 0.0)).rgb);
  let bl = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(-texelSize.x, texelSize.y)).rgb);
  let b  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(0.0, texelSize.y)).rgb);
  let br = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(texelSize.x, texelSize.y)).rgb);
  let gx = -tl - 2.0*l - bl + tr + 2.0*r + br;
  let gy = -tl - 2.0*t - tr + bl + 2.0*b + br;
  var edge = sqrt(gx*gx + gy*gy) * params.strength;
  edge = clamp(edge, 0.0, 1.0);
  if (params.invertFlag > 0.5) { edge = 1.0 - edge; }
  return vec4f(vec3f(edge), 1.0);
}`,params:{strength:{type:"number",label:"Strength",default:1,min:0,max:5,step:.1,animatable:!0},invert:{type:"boolean",label:"Invert",default:!1}},packUniforms:(e,t,r)=>new Float32Array([e.strength??1,t,r,e.invert?1:0])},S1={id:"gpu-scanlines",name:"Scanlines",category:"stylize",entryPoint:"scanlinesFragment",uniformSize:16,shader:`
struct ScanlinesParams { density: f32, opacity: f32, speed: f32, time: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ScanlinesParams;
@fragment
fn scanlinesFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let scrollOffset = params.time * params.speed * 0.1;
  let scanline = sin((input.uv.y + scrollOffset) * params.density * 100.0) * 0.5 + 0.5;
  let darken = 1.0 - params.opacity * (1.0 - scanline);
  return vec4f(color.rgb * darken, color.a);
}`,params:{density:{type:"number",label:"Density",default:5,min:1,max:20,step:.5,animatable:!0},opacity:{type:"number",label:"Opacity",default:.3,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Scroll Speed",default:0,min:0,max:5,step:.1,animatable:!1}},packUniforms:e=>{const t=performance.now()/1e3;return new Float32Array([e.density??5,e.opacity??.3,e.speed??0,t])}},I1={id:"gpu-color-glitch",name:"Color Glitch",category:"stylize",entryPoint:"colorGlitchFragment",uniformSize:16,shader:`
struct ColorGlitchParams { intensity: f32, speed: f32, time: f32, _pad: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ColorGlitchParams;
@fragment
fn colorGlitchFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let amt = clamp(params.intensity, 0.0, 1.0);

  // Discrete time steps give the stuttering digital-glitch cadence and keep the
  // look readable even when playback is paused (the seed stays fixed per step).
  // The seed MUST be wrapped into a small range: the shared sin() hash loses all
  // precision with large inputs, so an unbounded (time * speed) seed collapses
  // bandNoise to a constant at higher speeds / long sessions and the effect looks
  // frozen. Wrapping keeps the seed usable at any speed.
  let rawStep = floor(params.time * params.speed * 12.0);
  let t = rawStep - floor(rawStep / 64.0) * 64.0;

  // Tear the frame into horizontal bands; each band glitches independently.
  let band = floor(uv.y * 28.0);
  let bandNoise = hash(vec2f(band, t));

  // More bands corrupt as intensity rises (full frame at intensity = 1).
  let glitchOn = step(1.0 - amt, bandNoise);

  // Per-band horizontal block displacement.
  let shift = (hash(vec2f(band * 1.7, t + 3.0)) - 0.5) * 0.15 * amt * glitchOn;

  // RGB channel separation — the signature colour-glitch fringing, scaled so even
  // a single still frame reads clearly.
  let split = (0.004 + 0.02 * amt) * glitchOn;
  let baseUv = vec2f(uv.x + shift, uv.y);
  let r = textureSample(inputTex, texSampler, baseUv + vec2f(split, 0.0)).r;
  let g = textureSample(inputTex, texSampler, baseUv).g;
  let b = textureSample(inputTex, texSampler, baseUv - vec2f(split, 0.0)).b;
  let a = textureSample(inputTex, texSampler, baseUv).a;
  var rgb = vec3f(r, g, b);

  // Hard hue corruption on the strongest bands for the "colour" in colour glitch.
  let hueHit = step(0.82, bandNoise) * glitchOn;
  var hsv = rgb2hsv(rgb);
  hsv.x = fract(hsv.x + hash(vec2f(band, t + 7.0)));
  rgb = mix(rgb, hsv2rgb(hsv), hueHit);

  return vec4f(rgb, a);
}`,params:{intensity:{type:"number",label:"Intensity",default:.5,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:.1,max:5,step:.1,animatable:!1}},packUniforms:e=>{const t=performance.now()/1e3;return new Float32Array([e.intensity??.5,e.speed??1,t,0])}},x1={id:"gpu-block-glitch",name:"Block Glitch",category:"stylize",entryPoint:"blockGlitchFragment",uniformSize:32,shader:`
struct BlockGlitchParams {
  coverage: f32, intensity: f32, blockSize: f32, speed: f32,
  time: f32, width: f32, height: f32, pad: f32
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: BlockGlitchParams;
@fragment
fn blockGlitchFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let amt = clamp(params.intensity, 0.0, 1.0);
  let cov = clamp(params.coverage, 0.0, 1.0);

  // Discrete, wrapped time step — keeps the sin() hash seed in its precise range
  // so the glitch stays alive at any speed / session length.
  let rawStep = floor(params.time * params.speed * 8.0);
  let t = rawStep - floor(rawStep / 64.0) * 64.0;

  // Block grid in pixels.
  let cell = max(params.blockSize, 2.0);
  let cols = max(params.width / cell, 1.0);
  let rows = max(params.height / cell, 1.0);
  let block = vec2f(floor(uv.x * cols), floor(uv.y * rows));

  // Per-block, per-step decision: does this block glitch?
  let r1 = hash(vec2f(block.x + block.y * 3.0, t));
  let glitchOn = step(1.0 - cov, r1);

  // Block displacement: a horizontal datamosh slab plus a smaller vertical jump.
  let dispX = (hash(vec2f(block.y, t + 5.0)) - 0.5) * 0.25 * amt * glitchOn;
  let vJump = step(0.6, hash(vec2f(block.x + block.y, t + 13.0)));
  let dispY = (hash(vec2f(block.x, t + 9.0)) - 0.5) * 0.06 * amt * glitchOn * vJump;
  let srcUv = vec2f(uv.x + dispX, uv.y + dispY);

  // RGB channel split on glitched blocks.
  let split = (0.01 + 0.03 * amt) * glitchOn;
  let rr = textureSample(inputTex, texSampler, srcUv + vec2f(split, 0.0)).r;
  let gg = textureSample(inputTex, texSampler, srcUv).g;
  let bb = textureSample(inputTex, texSampler, srcUv - vec2f(split, 0.0)).b;
  let aa = textureSample(inputTex, texSampler, srcUv).a;
  let rgb = vec3f(rr, gg, bb);

  // Digital corruption on a subset of glitched blocks: channel rotate / invert /
  // posterize. Sampling already done above, so this control flow is safe.
  let corrupt = step(0.7, hash(vec2f(block.x * 1.3 + block.y, t + 21.0))) * glitchOn;
  let mode = hash(vec2f(block.y * 2.1 + block.x, t + 27.0));
  var c = rgb;
  if (mode < 0.34) {
    c = rgb.gbr;
  } else if (mode < 0.67) {
    c = vec3f(1.0) - rgb;
  } else {
    c = floor(rgb * 4.0) / 4.0;
  }

  return vec4f(mix(rgb, c, corrupt), aa);
}`,params:{coverage:{type:"number",label:"Coverage",default:.3,min:0,max:1,step:.01,animatable:!0},intensity:{type:"number",label:"Intensity",default:.6,min:0,max:1,step:.01,animatable:!0},blockSize:{type:"number",label:"Block Size",default:40,min:8,max:200,step:1,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:.1,max:5,step:.1,animatable:!1}},packUniforms:(e,t,r)=>new Float32Array([e.coverage??.3,e.intensity??.6,e.blockSize??40,e.speed??1,performance.now()/1e3,t,r,0])},w1={id:"gpu-crt",name:"CRT",category:"stylize",entryPoint:"crtFragment",uniformSize:32,shader:`
struct CrtParams {
  curvature: f32, scanlines: f32, vignette: f32, chroma: f32,
  width: f32, height: f32, pad0: f32, pad1: f32
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: CrtParams;
@fragment
fn crtFragment(input: VertexOutput) -> @location(0) vec4f {
  // Barrel-warp the sampling coordinate around the screen centre.
  let curveAmt = clamp(params.curvature, 0.0, 1.0) * 0.35;
  let cc = input.uv * 2.0 - 1.0;
  let r2 = dot(cc, cc);
  let warped = cc * (1.0 + r2 * curveAmt);
  let cuv = warped * 0.5 + 0.5;

  // Soft black border where the warped image leaves the tube.
  let edge = 0.004 + curveAmt * 0.02;
  let mask = smoothstep(0.0, edge, cuv.x) * smoothstep(0.0, edge, 1.0 - cuv.x)
           * smoothstep(0.0, edge, cuv.y) * smoothstep(0.0, edge, 1.0 - cuv.y);

  // Chromatic aberration that grows toward the edges.
  let ca = params.chroma * 0.012 * r2;
  let dir = normalize(cc + vec2f(0.00001, 0.00001));
  let rC = textureSample(inputTex, texSampler, cuv + dir * ca).r;
  let gC = textureSample(inputTex, texSampler, cuv).g;
  let bC = textureSample(inputTex, texSampler, cuv - dir * ca).b;
  let aC = textureSample(inputTex, texSampler, cuv).a;
  var rgb = vec3f(rC, gC, bC);

  // Scanlines (resolution-independent line count).
  let sl = 0.5 + 0.5 * sin(cuv.y * 320.0 * PI);
  rgb = rgb * (1.0 - clamp(params.scanlines, 0.0, 1.0) * (1.0 - sl));

  // Vignette toward the corners.
  rgb = rgb * (1.0 - clamp(params.vignette, 0.0, 1.0) * smoothstep(0.35, 1.5, r2));

  return vec4f(rgb * mask, aC);
}`,params:{curvature:{type:"number",label:"Curvature",default:.3,min:0,max:1,step:.01,animatable:!0},scanlines:{type:"number",label:"Scanlines",default:.3,min:0,max:1,step:.01,animatable:!0},vignette:{type:"number",label:"Vignette",default:.3,min:0,max:1,step:.01,animatable:!0},chroma:{type:"number",label:"Chroma",default:.4,min:0,max:2,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.curvature??.3,e.scanlines??.3,e.vignette??.3,e.chroma??.4,t,r,0,0])};function k1(e){return Math.min(1,Math.max(0,e))}function Yr(e,t){if(!e.startsWith("#"))return t;const r=e.slice(1);if(r.length===3||r.length===4){const n=r.split("").map(a=>parseInt(a+a,16)/255);return[n[0]??t[0],n[1]??t[1],n[2]??t[2],n[3]??1]}if(r.length===6||r.length===8){const n=[parseInt(r.slice(0,2),16)/255,parseInt(r.slice(2,4),16)/255,parseInt(r.slice(4,6),16)/255,r.length===8?parseInt(r.slice(6,8),16)/255:1];if(n.every(a=>Number.isFinite(a)))return n}return t}function P1(e,t){if(!e||e<=0||!Number.isFinite(e))return .5;const r=Math.max(7,Math.min(300,t/e)),n=k1((300-r)/293);return Math.pow(n,1/.7)}function M1(e,t){return!e||!t||t<=0?1.25:Math.max(0,Math.min(2,e/t*2))}const C1={id:"gpu-halftone",name:"Halftone",category:"stylize",entryPoint:"halftoneFragment",uniformSize:80,shader:`
struct HalftoneParams {
  colorFront: vec4f,
  colorBack: vec4f,
  primary: vec4f,
  secondary: vec4f,
  tertiary: vec4f,
};

struct HalftoneSample {
  shape: f32,
  color: vec4f,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: HalftoneParams;

fn rotateHalftonePoint(p: vec2f, angleRad: f32) -> vec2f {
  let c = cos(angleRad);
  let s = sin(angleRad);
  return vec2f(p.x * c - p.y * s, p.x * s + p.y * c);
}

fn halftoneLinearStep(edge0: f32, edge1: f32, x: f32) -> f32 {
  return clamp((x - edge0) / (edge1 - edge0), 0.0, 1.0);
}

fn halftoneSmoothStep(edge0: f32, edge1: f32, x: f32) -> f32 {
  return smoothstep(edge0, edge1, x);
}

fn halftoneSigmoid(x: f32, k: f32) -> f32 {
  return 1.0 / (1.0 + exp(-k * (x - 0.5)));
}

fn getCircle(uv: vec2f, r: f32, baseR: f32) -> f32 {
  let rr = mix(0.25 * baseR, 0.0, r);
  let d = length(uv - 0.5);
  let aa = 0.02;
  return 1.0 - smoothstep(rr - aa, rr + aa, d);
}

fn getCell(uv: vec2f) -> f32 {
  let insideX = step(0.0, uv.x) * (1.0 - step(1.0, uv.x));
  let insideY = step(0.0, uv.y) * (1.0 - step(1.0, uv.y));
  return insideX * insideY;
}

fn getCircleWithHole(uv: vec2f, r: f32, baseR: f32) -> f32 {
  let cell = getCell(uv);
  let rr = mix(0.75 * baseR, 0.0, r);
  let rMod = rr - floor(rr / 0.5) * 0.5;
  let d = length(uv - 0.5);
  let aa = 0.02;
  let circle = 1.0 - smoothstep(rMod - aa, rMod + aa, d);
  if (rr < 0.5) {
    return circle;
  }
  return cell - circle;
}

fn getGooeyBall(uv: vec2f, r: f32, baseR: f32, gridType: i32) -> f32 {
  var d = length(uv - 0.5);
  var sizeRadius = 0.3;
  if (gridType == 1) {
    sizeRadius = 0.42;
  }
  sizeRadius = mix(sizeRadius * baseR, 0.0, r);
  d = 1.0 - halftoneSmoothStep(0.0, sizeRadius, d);
  d = pow(d, 2.0 + baseR);
  return d;
}

fn getSoftBall(uv: vec2f, r: f32, baseR: f32) -> f32 {
  var d = length(uv - 0.5);
  var sizeRadius = clamp(baseR, 0.0, 1.0);
  sizeRadius = mix(0.5 * sizeRadius, 0.0, r);
  d = 1.0 - halftoneLinearStep(0.0, sizeRadius, d);
  let powRadius = 1.0 - halftoneLinearStep(0.0, 2.0, baseR);
  d = pow(d, 4.0 + 3.0 * powRadius);
  return d;
}

fn getLumAtPx(uv: vec2f, contrast: f32, inverted: bool) -> f32 {
  let tex = textureSampleLevel(inputTex, texSampler, uv, 0.0);
  if (tex.a <= 1e-4) {
    return select(0.0, 1.0, inverted);
  }
  let color = vec3f(
    halftoneSigmoid(tex.r, contrast),
    halftoneSigmoid(tex.g, contrast),
    halftoneSigmoid(tex.b, contrast)
  );
  var lum = luminance(color);
  if (inverted) {
    lum = 1.0 - lum;
  }
  return lum;
}

fn getLumBall(
  p: vec2f,
  pad: vec2f,
  texelSize: vec2f,
  inCellOffset: vec2f,
  contrast: f32,
  baseR: f32,
  stepSize: f32,
  styleType: i32,
  gridType: i32,
  inverted: bool,
  grainMixer: f32,
  grainSizeParam: f32
) -> HalftoneSample {
  let pp = p + inCellOffset;
  let uv_i = floor(pp);
  let uv_f = fract(pp);
  let samplingUV = (uv_i + 0.5 - inCellOffset) * pad + 0.5;
  let safeSamplingUV = clamp(samplingUV, texelSize * 0.5, vec2f(1.0) - texelSize * 0.5);
  var lum = getLumAtPx(safeSamplingUV, contrast, inverted);
  if (grainMixer > 0.001) {
    let grainSizeCurve = pow(grainSizeParam, 0.72);
    let grainDomainScale = mix(2600.0, 55.0, grainSizeCurve);
    let grainDomain = safeSamplingUV * grainDomainScale + inCellOffset * 37.0 + vec2f(21.0, -14.0);
    let grainPrimary = halftoneOverlayNoise(grainDomain * mix(1.15, 0.2, grainSizeCurve));
    let grainSecondary = halftoneNoise(
      grainDomain * mix(2.1, 0.38, grainSizeCurve) +
      uv_f * mix(14.0, 4.0, grainSizeCurve)
    );
    let edgeWeight = 1.0 - abs(lum * 2.0 - 1.0);
    let lumJitter = (grainSecondary * 2.0 - 1.0) * (0.08 + 0.32 * grainMixer) * (0.3 + 0.7 * edgeWeight);
    let lumCut = smoothstep(0.45, 0.85 - 0.2 * grainMixer, grainPrimary) * grainMixer * (0.1 + 0.8 * edgeWeight);
    lum = clamp(lum + lumJitter - lumCut, 0.0, 1.0);
  }
  let sampledColor = textureSampleLevel(inputTex, texSampler, safeSamplingUV, 0.0);
  let sourceCoverage = sampledColor.a;
  if (sourceCoverage <= 1e-4) {
    return HalftoneSample(0.0, vec4f(0.0));
  }
  var ballColor = vec4f(sampledColor.rgb * sourceCoverage, sourceCoverage);
  var ball = 0.0;
  if (styleType == 0) {
    ball = getCircle(uv_f, lum, baseR);
  } else if (styleType == 1) {
    ball = getGooeyBall(uv_f, lum, baseR, gridType);
  } else if (styleType == 2) {
    ball = getCircleWithHole(uv_f, lum, baseR);
  } else {
    ball = getSoftBall(uv_f, lum, baseR);
  }
  return HalftoneSample(ball * sourceCoverage, ballColor);
}

fn halftoneNoise(p: vec2f) -> f32 {
  let layerA = noise2d(p);
  let layerB = noise2d(vec2f(
    p.x * 1.31 + p.y * 0.74,
    p.x * -0.68 + p.y * 1.27
  ) + vec2f(11.7, 3.9));
  let layerC = noise2d(vec2f(
    p.x * -0.57 + p.y * 1.43,
    p.x * 1.19 + p.y * 0.53
  ) + vec2f(-7.4, 13.1));
  return (layerA + layerB + layerC) / 3.0;
}

fn halftoneOverlayNoise(p: vec2f) -> f32 {
  let coarse = halftoneNoise(p * 0.73 + vec2f(5.31, -8.17));
  let medium = halftoneNoise(vec2f(
    p.x * 1.41 - p.y * 0.52,
    p.x * 0.67 + p.y * 1.28
  ) + vec2f(-11.4, 4.6));
  let fine = halftoneNoise(vec2f(
    p.x * -0.88 + p.y * 1.19,
    p.x * -1.07 - p.y * 0.79
  ) + vec2f(8.2, 10.7));
  return coarse * 0.45 + medium * 0.35 + fine * 0.2;
}

fn getStepCount(styleType: i32) -> i32 {
  if (styleType == 1) {
    return 6;
  }
  if (styleType == 3) {
    return 6;
  }
  if (styleType == 0) {
    return 2;
  }
  return 1;
}

@fragment
fn halftoneFragment(input: VertexOutput) -> @location(0) vec4f {
  let sourceSample = textureSampleLevel(inputTex, texSampler, input.uv, 0.0);
  let sourceAlpha = sourceSample.a;
  if (sourceAlpha <= 1e-4) {
    return vec4f(0.0);
  }

  let dims = vec2f(textureDimensions(inputTex));
  let aspect = max(dims.x / max(dims.y, 1.0), 0.0001);
  let size = clamp(params.primary.x, 0.0, 1.0);
  let radius = clamp(params.primary.y, 0.0, 2.0);
  let contrastParam = clamp(params.primary.z, 0.0, 1.0);
  let originalColors = params.secondary.x > 0.5;
  let inverted = params.secondary.y > 0.5;
  let grainMixer = clamp(params.secondary.z, 0.0, 1.0);
  let grainOverlay = clamp(params.secondary.w, 0.0, 1.0);
  let grainSizeParam = clamp(params.tertiary.x, 0.0, 1.0);
  let gridType = i32(params.tertiary.y);
  let styleType = i32(params.tertiary.z);

  let stepCount = getStepCount(styleType);
  let stepSize = 1.0 / f32(stepCount);

  var cellsPerSide = mix(300.0, 7.0, pow(size, 0.7));
  cellsPerSide /= f32(stepCount);
  let cellSizeY = 1.0 / cellsPerSide;
  var pad = cellSizeY * vec2f(1.0 / aspect, 1.0);
  if (styleType == 1 && gridType == 1) {
    pad *= 0.7;
  }
  // Snap to whole cells AND force odd counts.  With an odd cell count the
  // centered grid puts uv = ±N.5 at the texture edges, which lands at
  // fract = 0.5 — right on the dot center.  Even counts land at fract = 0
  // (cell boundary), exposing the paper/background as a rectangular border.
  var rawCols = max(1.0, round(1.0 / max(pad.x, 1e-4)));
  var rawRows = max(1.0, round(1.0 / max(pad.y, 1e-4)));
  let cols = rawCols + select(0.0, 1.0, fract(rawCols * 0.5) < 0.25);
  let rows = rawRows + select(0.0, 1.0, fract(rawRows * 0.5) < 0.25);
  pad = vec2f(1.0 / cols, 1.0 / rows);
  let texelSize = 1.0 / max(dims, vec2f(1.0));

  var uv = input.uv - 0.5;
  uv /= pad;

  var contrast = mix(0.0, 15.0, pow(contrastParam, 1.5));
  var baseRadius = radius;
  if (originalColors) {
    contrast = mix(0.1, 4.0, pow(contrastParam, 2.0));
    baseRadius = 2.0 * pow(0.5 * radius, 0.3);
  }

  var totalShape = 0.0;
  var totalColor = vec3f(0.0);
  var totalOpacity = 0.0;

  for (var xi = 0; xi < 6; xi = xi + 1) {
    if (xi >= stepCount) {
      continue;
    }

    for (var yi = 0; yi < 6; yi = yi + 1) {
      if (yi >= stepCount) {
        continue;
      }

      var offset = vec2f(f32(xi) / f32(stepCount) - 0.5, f32(yi) / f32(stepCount) - 0.5);
      if (gridType == 1) {
        var rowIndex = f32(yi);
        var colIndex = f32(xi);
        if (stepCount == 1) {
          rowIndex = floor(uv.y + offset.y + 1.0);
          if (styleType == 1) {
            colIndex = floor(uv.x + offset.x + 1.0);
          }
        }
        if (styleType == 1) {
          if (fract((rowIndex + colIndex) * 0.5) >= 0.5) {
            continue;
          }
        } else if (fract(rowIndex * 0.5) >= 0.5) {
          offset.x += 0.5 * stepSize;
        }
      }

      let sample = getLumBall(
        uv,
        pad,
        texelSize,
        offset,
        contrast,
        baseRadius,
        stepSize,
        styleType,
        gridType,
        inverted,
        grainMixer,
        grainSizeParam
      );
      totalColor += sample.color.rgb * sample.shape;
      totalShape += sample.shape;
      totalOpacity += sample.shape;
    }
  }

  let eps = 1e-4;
  totalColor /= max(totalShape, eps);
  totalOpacity /= max(totalShape, eps);

  var finalShape = 0.0;
  if (styleType == 0) {
    finalShape = min(1.0, totalShape);
  } else if (styleType == 1) {
    let aa = 0.08;
    finalShape = smoothstep(0.5 - aa, 0.5 + aa, totalShape);
  } else if (styleType == 2) {
    finalShape = min(1.0, totalShape);
  } else {
    finalShape = totalShape;
  }

  let grainSizeCurve = pow(grainSizeParam, 0.72);
  let grainScale = mix(3200.0, 42.0, grainSizeCurve) * vec2f(1.0, 1.0 / aspect);
  let grainUV = input.uv * grainScale + vec2f(13.1, -9.7);
  let edgeBand = pow(clamp(1.0 - abs(finalShape * 2.0 - 1.0), 0.0, 1.0), 0.55);
  let grainField = halftoneOverlayNoise(grainUV * mix(0.95, 0.16, grainSizeCurve));
  let grainDetail = halftoneNoise(
    grainUV * mix(1.9, 0.28, grainSizeCurve) +
    vec2f(-17.3, 6.4)
  );
  let grainCut = smoothstep(0.42, 0.9, grainField);
  let grainWarp = (grainDetail * 2.0 - 1.0) * edgeBand * grainMixer * 0.24;
  let grainErode = edgeBand * grainCut * grainMixer * (0.35 + 1.75 * grainMixer);
  finalShape = clamp(finalShape + grainWarp - grainErode, 0.0, 1.0);

  var color = vec3f(0.0);
  var opacity = 0.0;
  if (originalColors) {
    color = totalColor * finalShape;
    opacity = totalOpacity * finalShape;
    let bgColor = params.colorBack.rgb * params.colorBack.a;
    color += bgColor * (1.0 - opacity);
    opacity += params.colorBack.a * (1.0 - opacity);
  } else {
    let fgColor = params.colorFront.rgb * params.colorFront.a;
    let bgColor = params.colorBack.rgb * params.colorBack.a;
    color = fgColor * finalShape;
    opacity = params.colorFront.a * finalShape;
    color += bgColor * (1.0 - opacity);
    opacity += params.colorBack.a * (1.0 - opacity);
  }

  var grainOverlayNoise = halftoneOverlayNoise(grainUV * mix(0.9, 0.14, grainSizeCurve));
  grainOverlayNoise = pow(grainOverlayNoise, 1.3);
  let grainOverlayV = grainOverlayNoise * 2.0 - 1.0;
  let grainOverlayColor = vec3f(select(0.0, 1.0, grainOverlayV >= 0.0));
  var grainOverlayStrength = grainOverlay * abs(grainOverlayV);
  grainOverlayStrength = pow(grainOverlayStrength, 0.8);
  color = mix(color, grainOverlayColor, 0.5 * grainOverlayStrength);
  opacity += 0.5 * grainOverlayStrength;

  opacity = clamp(opacity, 0.0, 1.0) * sourceAlpha;

  // Output STRAIGHT alpha (RGB not premultiplied) — the final blit premultiplies
  // once for the premultiplied output canvas. Multiplying RGB by sourceAlpha here
  // too would premultiply twice on masked/transparent content (sourceAlpha^2),
  // darkening the mask edge into a visible seam. Opaque pixels (sourceAlpha = 1)
  // are unchanged.
  return vec4f(clamp(color, vec3f(0.0), vec3f(1.0)), opacity);
}`,params:{colorFront:{type:"color",label:"Front Color",default:"#2b2b2b",animatable:!0},colorBack:{type:"color",label:"Back Color",default:"#f2f1e8",animatable:!0},originalColors:{type:"boolean",label:"Original Colors",default:!1},inverted:{type:"boolean",label:"Inverted",default:!1},grid:{type:"select",label:"Grid",default:"hex",options:[{value:"hex",label:"Hex"},{value:"square",label:"Square"}]},type:{type:"select",label:"Type",default:"gooey",options:[{value:"classic",label:"Classic"},{value:"gooey",label:"Gooey"},{value:"holes",label:"Holes"},{value:"soft",label:"Soft"}]},size:{type:"number",label:"Size",default:.5,min:0,max:1,step:.01,animatable:!0},radius:{type:"number",label:"Radius",default:1.25,min:0,max:2,step:.01,animatable:!0},contrast:{type:"number",label:"Contrast",default:.4,min:0,max:1,step:.01,animatable:!0},grainMixer:{type:"number",label:"Grain Mixer",default:.2,min:0,max:1,step:.01,animatable:!0},grainOverlay:{type:"number",label:"Grain Overlay",default:.2,min:0,max:1,step:.01,animatable:!0},grainSize:{type:"number",label:"Grain Size",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n={square:0,hex:1},a={classic:0,gooey:1,holes:2,soft:3},i=Yr(e.colorFront??"#2b2b2b",[43/255,43/255,43/255,1]),o=Yr(e.colorBack??"#f2f1e8",[242/255,241/255,232/255,1]),s=e.size??P1(e.spacing,r),l=e.radius??M1(e.dotSize,e.spacing),c=e.contrast??.4,u=e.originalColors??!1,d=e.inverted??e.invert??!1,m=e.grainMixer??.2,p=e.grainOverlay??.2,f=e.grainSize??.5,h=e.grid??"hex",g=e.type??e.dotStyle??"gooey";return new Float32Array([i[0],i[1],i[2],i[3],o[0],o[1],o[2],o[3],s,l,c,t/Math.max(r,1),u?1:0,d?1:0,m,p,f,n[h]??1,a[g]??1,0])}},Gc={bayer2:0,bayer4:1,bayer8:2,halftone:3,lines:4,crosses:5,dots:6,grid:7,scales:8},Kc={image:0,linear:1,radial:2},Vc={threshold:0,scaled:1},Wc={circle:0,square:1,diamond:2},jc={bw:0,gameboy:1,cga:2,sepia:3},$c={standard:0,simple:1,blocks:2,dots:3,minimal:4},T1={ascii:"@%#*+=-:. ",dense:"@WB#$oahkbn+=-:. ",binary:"01",symbols:"#@&$%*+!=;:-. "},E1=24,Jp=64,Yc={monospace:"monospace",courier:'"Courier New", Courier, monospace',consolas:'Consolas, "Lucida Console", monospace',lucida:'"Lucida Console", Monaco, monospace'};function Ta(e,t){if(e==="custom"){const r=[...t].slice(0,Jp);return r.length>0?r.join(""):null}return T1[e]??null}function F1(e,t){const r=[...e].slice(0,Jp),n=E1,i=Math.max(r.length,1)*n,o=n,s=new Uint8Array(i*o*4),c=(typeof OffscreenCanvas<"u"?new OffscreenCanvas(i,o):null)?.getContext("2d",{willReadFrequently:!0});if(!c)return s.fill(255),{width:i,height:o,depth:1,data:s};const u=Yc[t]??Yc.monospace;c.clearRect(0,0,i,o),c.fillStyle="#ffffff",c.font=`${Math.round(n*.82)}px ${u}`,c.textAlign="center",c.textBaseline="middle";for(let m=0;m<r.length;m++)c.fillText(r[m],m*n+n/2,n/2+1);const d=c.getImageData(0,0,i,o).data;for(let m=0;m<i*o;m++){const p=d[m*4+3]??0;s[m*4]=p,s[m*4+1]=p,s[m*4+2]=p,s[m*4+3]=p}return{width:i,height:o,depth:1,data:s}}const A1={id:"gpu-dither",name:"Dither",category:"stylize",entryPoint:"ditherFragment",uniformSize:48,shader:`
struct DitherParams {
  cellSize: f32, angleDeg: f32, scalePercent: f32, width: f32,
  height: f32, offsetX: f32, offsetY: f32, patternKind: f32,
  modeKind: f32, styleKind: f32, shapeKind: f32, paletteKind: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: DitherParams;

fn bayer2Threshold(cell: vec2i) -> f32 {
  let x = cell.x % 2;
  let y = cell.y % 2;
  var raw = 0.0;
  if (y == 0) {
    raw = select(0.0, 2.0, x == 1);
  } else {
    raw = select(3.0, 1.0, x == 1);
  }
  return (raw + 0.5) / 4.0;
}

fn bayer4Index(cell: vec2i) -> i32 {
  let x = cell.x % 4;
  let y = cell.y % 4;
  if (y == 0) {
    if (x == 0) { return 0; }
    if (x == 1) { return 8; }
    if (x == 2) { return 2; }
    return 10;
  }
  if (y == 1) {
    if (x == 0) { return 12; }
    if (x == 1) { return 4; }
    if (x == 2) { return 14; }
    return 6;
  }
  if (y == 2) {
    if (x == 0) { return 3; }
    if (x == 1) { return 11; }
    if (x == 2) { return 1; }
    return 9;
  }
  if (x == 0) { return 15; }
  if (x == 1) { return 7; }
  if (x == 2) { return 13; }
  return 5;
}

fn bayer4Threshold(cell: vec2i) -> f32 {
  return (f32(bayer4Index(cell)) + 0.5) / 16.0;
}

fn bayer8Threshold(cell: vec2i) -> f32 {
  let base = bayer4Index(vec2i(cell.x % 4, cell.y % 4));
  let quad = select(0, 1, (cell.x % 8) >= 4) + select(0, 2, (cell.y % 8) >= 4);
  let offset = select(0, select(2, select(3, 1, quad == 3), quad == 2), quad > 0);
  let raw = 4 * base + offset;
  return (f32(raw) + 0.5) / 64.0;
}

fn patternThreshold(patternKind: i32, cell: vec2f, patternCellSize: f32) -> f32 {
  if (patternKind == 0) {
    return bayer2Threshold(vec2i(cell));
  }
  if (patternKind == 1) {
    return bayer4Threshold(vec2i(cell));
  }
  if (patternKind == 2) {
    return bayer8Threshold(vec2i(cell));
  }

  let safeCellSize = max(2.0, patternCellSize);
  let nx = fract(cell.x / safeCellSize);
  let ny = fract(cell.y / safeCellSize);
  let cx = 0.5;
  let cy = 0.5;

  if (patternKind == 3 || patternKind == 6) {
    let dx = nx - cx;
    let dy = ny - cy;
    return sqrt(dx * dx + dy * dy) * 1.41421356237;
  }
  if (patternKind == 4) {
    return ny;
  }
  if (patternKind == 5) {
    let distX = abs(nx - cx);
    let distY = abs(ny - cy);
    return min(distX, distY) * 2.0;
  }
  if (patternKind == 7) {
    let distX = abs(nx - cx);
    let distY = abs(ny - cy);
    return max(distX, distY) * 2.0;
  }
  if (patternKind == 8) {
    let sx = fract(nx * 2.0);
    let sy = fract(ny * 2.0);
    let dx = sx - 0.5;
    let dy = sy - 0.5;
    return sqrt(dx * dx + dy * dy) * 1.41421356237;
  }
  return 0.5;
}

fn paletteLastIndex(paletteKind: i32) -> i32 {
  if (paletteKind == 0) { return 1; }
  return 3;
}

fn paletteIndex(value: f32, paletteKind: i32) -> i32 {
  if (paletteKind == 0) {
    return select(1, 0, value <= 0.5);
  }
  if (value <= 0.25) { return 0; }
  if (value <= 0.5) { return 1; }
  if (value <= 0.75) { return 2; }
  return 3;
}

fn paletteColor(paletteKind: i32, colorIndex: i32) -> vec3f {
  if (paletteKind == 0) {
    if (colorIndex == 0) { return vec3f(0.0, 0.0, 0.0); }
    return vec3f(1.0, 1.0, 1.0);
  }
  if (paletteKind == 1) {
    if (colorIndex == 0) { return vec3f(0.0588, 0.2196, 0.0588); }
    if (colorIndex == 1) { return vec3f(0.1882, 0.3843, 0.1882); }
    if (colorIndex == 2) { return vec3f(0.5451, 0.6745, 0.0588); }
    return vec3f(0.6078, 0.7373, 0.0588);
  }
  if (paletteKind == 2) {
    if (colorIndex == 0) { return vec3f(0.0, 0.0, 0.0); }
    if (colorIndex == 1) { return vec3f(0.3333, 1.0, 1.0); }
    if (colorIndex == 2) { return vec3f(1.0, 0.3333, 1.0); }
    return vec3f(1.0, 1.0, 1.0);
  }
  if (colorIndex == 0) { return vec3f(0.1686, 0.1137, 0.0549); }
  if (colorIndex == 1) { return vec3f(0.4196, 0.2588, 0.1490); }
  if (colorIndex == 2) { return vec3f(0.7686, 0.5843, 0.4157); }
  return vec3f(0.9608, 0.9020, 0.7843);
}

fn clampTexelCoord(coord: vec2i, texSize: vec2i) -> vec2i {
  return vec2i(
    clamp(coord.x, 0, max(texSize.x - 1, 0)),
    clamp(coord.y, 0, max(texSize.y - 1, 0))
  );
}

fn loadInputTexel(coord: vec2i, texSize: vec2i) -> vec4f {
  return textureLoad(inputTex, clampTexelCoord(coord, texSize), 0);
}

fn sampleCellBrightness(cell: vec2f, cellSize: f32, texSizeI: vec2i) -> f32 {
  let sampleOffsets = array<vec2f, 4>(
    vec2f(0.25, 0.25),
    vec2f(0.75, 0.25),
    vec2f(0.25, 0.75),
    vec2f(0.75, 0.75)
  );
  var luminanceSum = 0.0;
  var alphaSum = 0.0;
  for (var i = 0; i < 4; i++) {
    let sampleCoord = vec2i((cell + sampleOffsets[i]) * cellSize);
    let sampleColor = loadInputTexel(sampleCoord, texSizeI);
    luminanceSum += luminance601(sampleColor.rgb) * sampleColor.a;
    alphaSum += sampleColor.a;
  }
  if (alphaSum <= 0.0001) {
    return 0.0;
  }
  return luminanceSum / alphaSum;
}

fn applyMode(brightness: f32, cell: vec2f, gridSize: vec2f, modeKind: i32, angleDeg: f32, scalePercent: f32, offsetX: f32, offsetY: f32) -> f32 {
  var adjusted = brightness;
  let nx = cell.x / max(gridSize.x, 1.0);
  let ny = cell.y / max(gridSize.y, 1.0);
  if (modeKind == 1) {
    let angleRad = angleDeg * PI / 180.0;
    let gradient = nx * cos(angleRad) + ny * sin(angleRad);
    adjusted = clamp(adjusted * 0.7 + gradient * 0.3, 0.0, 1.0);
  } else if (modeKind == 2) {
    let ox = offsetX / 100.0;
    let oy = offsetY / 100.0;
    let dx = nx - (0.5 + ox);
    let dy = ny - (0.5 + oy);
    let dist = length(vec2f(dx, dy)) * (scalePercent / 100.0) * 2.0;
    adjusted = clamp(adjusted * 0.7 + dist * 0.3, 0.0, 1.0);
  }
  return adjusted;
}

fn shapeMask(shapeKind: i32, localUv: vec2f, sizeFactor: f32, cellSize: f32) -> f32 {
  let centered = abs(localUv - 0.5) * 2.0;
  let radius = clamp(sizeFactor, 0.0, 1.0);
  let aa = max(1.0 / max(cellSize, 1.0), 0.003);

  if (shapeKind == 0) {
    return 1.0 - smoothstep(radius, radius + aa, length(centered));
  }
  if (shapeKind == 2) {
    return 1.0 - smoothstep(radius, radius + aa, centered.x + centered.y);
  }
  return 1.0 - smoothstep(radius, radius + aa, max(centered.x, centered.y));
}

@fragment
fn ditherFragment(input: VertexOutput) -> @location(0) vec4f {
  let texSize = vec2f(params.width, params.height);
  let texSizeI = vec2i(max(i32(params.width), 1), max(i32(params.height), 1));
  let cellSize = max(params.cellSize, 1.0);
  let pixelPos = input.uv * texSize;
  let base = loadInputTexel(vec2i(pixelPos), texSizeI);
  if (base.a <= 0.0001) {
    return vec4f(0.0);
  }

  let cell = floor(pixelPos / cellSize);
  let localUv = fract(pixelPos / cellSize);
  let gridSize = vec2f(
    max(1.0, ceil(texSize.x / cellSize)),
    max(1.0, ceil(texSize.y / cellSize))
  );

  let modeKind = i32(params.modeKind + 0.5);
  let styleKind = i32(params.styleKind + 0.5);
  let shapeKind = i32(params.shapeKind + 0.5);
  let paletteKind = i32(params.paletteKind + 0.5);
  let patternKind = i32(params.patternKind + 0.5);

  var brightness = sampleCellBrightness(cell, cellSize, texSizeI);
  brightness = applyMode(
    brightness,
    cell,
    gridSize,
    modeKind,
    params.angleDeg,
    params.scalePercent,
    params.offsetX,
    params.offsetY
  );

  var quantized = brightness;
  var sizeFactor = 1.0;
  if (styleKind == 1) {
    sizeFactor = 1.0 - brightness;
  } else {
    let threshold = patternThreshold(patternKind, cell, max(2.0, floor(cellSize * 0.5)));
    quantized = clamp(brightness + (threshold - 0.5) * 0.5, 0.0, 1.0);
  }

  let colorIndex = paletteIndex(quantized, paletteKind);
  let background = paletteColor(paletteKind, paletteLastIndex(paletteKind));
  let foreground = paletteColor(paletteKind, colorIndex);
  let mask = shapeMask(shapeKind, localUv, sizeFactor, cellSize);
  let color = mix(background, foreground, mask);

  return vec4f(color, base.a);
}`,params:{pattern:{type:"select",label:"Pattern",default:"bayer4",options:[{value:"bayer2",label:"Bayer 2x2"},{value:"bayer4",label:"Bayer 4x4"},{value:"bayer8",label:"Bayer 8x8"},{value:"halftone",label:"Halftone"},{value:"lines",label:"Lines"},{value:"crosses",label:"Crosses"},{value:"dots",label:"Dots"},{value:"grid",label:"Grid"},{value:"scales",label:"Scales"}]},mode:{type:"select",label:"Mode",default:"image",options:[{value:"image",label:"Image"},{value:"linear",label:"Linear"},{value:"radial",label:"Radial"}]},style:{type:"select",label:"Style",default:"threshold",options:[{value:"threshold",label:"Threshold"},{value:"scaled",label:"Scaled"}]},shape:{type:"select",label:"Shape",default:"square",options:[{value:"circle",label:"Circle"},{value:"square",label:"Square"},{value:"diamond",label:"Diamond"}]},palette:{type:"select",label:"Palette",default:"gameboy",options:[{value:"bw",label:"B&W"},{value:"gameboy",label:"Game Boy"},{value:"cga",label:"CGA"},{value:"sepia",label:"Sepia"}]},cellSize:{type:"number",label:"Cell Size",default:8,min:2,max:32,step:1,animatable:!0},angle:{type:"number",label:"Angle",default:45,min:0,max:360,step:1,animatable:!0,visibleWhen:e=>e.mode==="linear"},scale:{type:"number",label:"Scale",default:100,min:25,max:200,step:1,animatable:!0,visibleWhen:e=>e.mode==="radial"},offsetX:{type:"number",label:"Offset X",default:0,min:-100,max:100,step:1,animatable:!0,visibleWhen:e=>e.mode==="radial"},offsetY:{type:"number",label:"Offset Y",default:0,min:-100,max:100,step:1,animatable:!0,visibleWhen:e=>e.mode==="radial"}},packUniforms:(e,t,r)=>new Float32Array([e.cellSize??8,e.angle??45,e.scale??100,t,r,e.offsetX??0,e.offsetY??0,Gc[e.pattern]??Gc.bayer4??0,Kc[e.mode]??Kc.image??0,Vc[e.style]??Vc.threshold??0,Wc[e.shape]??Wc.square??0,jc[e.palette]??jc.gameboy??0])},_1={id:"gpu-ascii",name:"ASCII",category:"stylize",entryPoint:"asciiFragment",uniformSize:96,shader:`
struct AsciiParams {
  fontSize: f32, letterSpacing: f32, lineHeight: f32, charsetKind: f32,
  matchSourceColor: f32, invert: f32, asciiOpacity: f32, originalOpacity: f32,
  contrast: f32, brightness: f32, saturation: f32, width: f32,
  height: f32, transparentBg: f32, edgeDetect: f32, glyphCount: f32,
  textColor: vec4f,
  bgColor: vec4f,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: AsciiParams;
@group(0) @binding(3) var asciiAtlas: texture_2d<f32>;

fn asciiClampTexelCoord(coord: vec2i, texSize: vec2i) -> vec2i {
  return vec2i(
    clamp(coord.x, 0, max(texSize.x - 1, 0)),
    clamp(coord.y, 0, max(texSize.y - 1, 0))
  );
}

fn asciiLoadTexel(coord: vec2i, texSize: vec2i) -> vec4f {
  return textureLoad(inputTex, asciiClampTexelCoord(coord, texSize), 0);
}

fn asciiAdjustColor(color: vec3f, contrast: f32, brightness: f32) -> vec3f {
  return clamp((color - 0.5) * contrast + 0.5 + vec3f(brightness), vec3f(0.0), vec3f(1.0));
}

fn asciiApplySaturation(color: vec3f, saturation: f32) -> vec3f {
  let gray = vec3f(luminance601(color));
  return clamp(gray + (color - gray) * saturation, vec3f(0.0), vec3f(1.0));
}

fn asciiSegmentDistance(p: vec2f, a: vec2f, b: vec2f) -> f32 {
  let pa = p - a;
  let ba = b - a;
  let denom = max(dot(ba, ba), 0.0001);
  let h = clamp(dot(pa, ba) / denom, 0.0, 1.0);
  return length(pa - ba * h);
}

fn asciiLineMask(p: vec2f, a: vec2f, b: vec2f, thickness: f32, blur: f32) -> f32 {
  let d = asciiSegmentDistance(p, a, b);
  return 1.0 - smoothstep(thickness, thickness + blur, d);
}

fn asciiBoxMask(p: vec2f, center: vec2f, halfSize: vec2f, blur: f32) -> f32 {
  let d = abs(p - center) - halfSize;
  return 1.0 - smoothstep(0.0, blur, max(d.x, d.y));
}

fn asciiCircleMask(p: vec2f, center: vec2f, radius: f32, blur: f32) -> f32 {
  let d = distance(p, center);
  return 1.0 - smoothstep(radius, radius + blur, d);
}

fn asciiRingMask(p: vec2f, center: vec2f, outerRadius: f32, innerRadius: f32, blur: f32) -> f32 {
  let outer = asciiCircleMask(p, center, outerRadius, blur);
  let inner = asciiCircleMask(p, center, innerRadius, blur);
  return clamp(outer - inner, 0.0, 1.0);
}

fn asciiBayer4Index(cell: vec2i) -> i32 {
  let x = cell.x % 4;
  let y = cell.y % 4;
  if (y == 0) {
    if (x == 0) { return 0; }
    if (x == 1) { return 8; }
    if (x == 2) { return 2; }
    return 10;
  }
  if (y == 1) {
    if (x == 0) { return 12; }
    if (x == 1) { return 4; }
    if (x == 2) { return 14; }
    return 6;
  }
  if (y == 2) {
    if (x == 0) { return 3; }
    if (x == 1) { return 11; }
    if (x == 2) { return 1; }
    return 9;
  }
  if (x == 0) { return 15; }
  if (x == 1) { return 7; }
  if (x == 2) { return 13; }
  return 5;
}

fn asciiBayer4Threshold(cell: vec2i) -> f32 {
  return (f32(asciiBayer4Index(cell)) + 0.5) / 16.0;
}

fn asciiBlocksMask(glyphIndex: i32, localUv: vec2f) -> f32 {
  if (glyphIndex <= 0) {
    return 0.0;
  }
  if (glyphIndex >= 4) {
    return 1.0;
  }

  var density = 0.25;
  if (glyphIndex == 2) {
    density = 0.5;
  } else if (glyphIndex == 3) {
    density = 0.75;
  }
  let patternCell = vec2i(floor(localUv * 4.0));
  return select(0.0, 1.0, density > asciiBayer4Threshold(patternCell));
}

fn asciiStandardMask(glyphIndex: i32, localUv: vec2f, blur: f32) -> f32 {
  if (glyphIndex <= 0) {
    return 0.0;
  }
  if (glyphIndex == 1) {
    return asciiCircleMask(localUv, vec2f(0.5, 0.76), 0.06, blur);
  }
  if (glyphIndex == 2) {
    let topDot = asciiCircleMask(localUv, vec2f(0.5, 0.34), 0.05, blur);
    let bottomDot = asciiCircleMask(localUv, vec2f(0.5, 0.72), 0.05, blur);
    return clamp(topDot + bottomDot, 0.0, 1.0);
  }
  if (glyphIndex == 3) {
    return asciiBoxMask(localUv, vec2f(0.5, 0.56), vec2f(0.23, 0.05), blur);
  }
  if (glyphIndex == 4) {
    let topLine = asciiBoxMask(localUv, vec2f(0.5, 0.38), vec2f(0.23, 0.04), blur);
    let bottomLine = asciiBoxMask(localUv, vec2f(0.5, 0.66), vec2f(0.23, 0.04), blur);
    return clamp(topLine + bottomLine, 0.0, 1.0);
  }
  if (glyphIndex == 5) {
    let horizontal = asciiBoxMask(localUv, vec2f(0.5, 0.52), vec2f(0.23, 0.04), blur);
    let vertical = asciiBoxMask(localUv, vec2f(0.5, 0.52), vec2f(0.04, 0.23), blur);
    return clamp(horizontal + vertical, 0.0, 1.0);
  }
  if (glyphIndex == 6) {
    let horizontal = asciiBoxMask(localUv, vec2f(0.5, 0.52), vec2f(0.22, 0.035), blur);
    let vertical = asciiBoxMask(localUv, vec2f(0.5, 0.52), vec2f(0.035, 0.22), blur);
    let diagA = asciiLineMask(localUv, vec2f(0.22, 0.22), vec2f(0.78, 0.78), 0.03, blur);
    let diagB = asciiLineMask(localUv, vec2f(0.78, 0.22), vec2f(0.22, 0.78), 0.03, blur);
    return clamp(horizontal + vertical + diagA + diagB, 0.0, 1.0);
  }
  if (glyphIndex == 7) {
    let left = asciiBoxMask(localUv, vec2f(0.34, 0.52), vec2f(0.03, 0.26), blur);
    let right = asciiBoxMask(localUv, vec2f(0.66, 0.52), vec2f(0.03, 0.26), blur);
    let top = asciiBoxMask(localUv, vec2f(0.5, 0.36), vec2f(0.24, 0.03), blur);
    let bottom = asciiBoxMask(localUv, vec2f(0.5, 0.68), vec2f(0.24, 0.03), blur);
    return clamp(left + right + top + bottom, 0.0, 1.0);
  }
  if (glyphIndex == 8) {
    let diag = asciiLineMask(localUv, vec2f(0.18, 0.82), vec2f(0.82, 0.18), 0.03, blur);
    let topCircle = asciiRingMask(localUv, vec2f(0.3, 0.3), 0.12, 0.065, blur);
    let bottomCircle = asciiRingMask(localUv, vec2f(0.7, 0.7), 0.12, 0.065, blur);
    return clamp(diag + topCircle + bottomCircle, 0.0, 1.0);
  }

  let ring = asciiRingMask(localUv, vec2f(0.5, 0.5), 0.34, 0.19, blur);
  let inner = asciiCircleMask(localUv, vec2f(0.55, 0.49), 0.1, blur);
  let tail = asciiLineMask(localUv, vec2f(0.53, 0.52), vec2f(0.74, 0.58), 0.035, blur);
  return clamp(ring + inner + tail, 0.0, 1.0);
}

fn asciiGlyphCount(charsetKind: i32) -> i32 {
  if (charsetKind == 1) { return 6; }
  if (charsetKind == 2) { return 5; }
  if (charsetKind == 3) { return 4; }
  if (charsetKind == 4) { return 3; }
  return 10;
}

fn asciiGlyphMask(charsetKind: i32, glyphIndex: i32, localUv: vec2f, blur: f32) -> f32 {
  if (charsetKind == 0) {
    return asciiStandardMask(glyphIndex, localUv, blur);
  }
  if (charsetKind == 1) {
    let mapped = array<i32, 6>(0, 1, 3, 5, 6, 7);
    return asciiStandardMask(mapped[clamp(glyphIndex, 0, 5)], localUv, blur);
  }
  if (charsetKind == 2) {
    return asciiBlocksMask(glyphIndex, localUv);
  }
  if (charsetKind == 3) {
    if (glyphIndex <= 0) { return 0.0; }
    if (glyphIndex == 1) { return asciiCircleMask(localUv, vec2f(0.5, 0.56), 0.05, blur); }
    if (glyphIndex == 2) { return asciiCircleMask(localUv, vec2f(0.5, 0.54), 0.1, blur); }
    return asciiCircleMask(localUv, vec2f(0.5, 0.52), 0.16, blur);
  }
  if (glyphIndex <= 0) {
    return 0.0;
  }
  if (glyphIndex == 1) {
    return asciiCircleMask(localUv, vec2f(0.5, 0.58), 0.055, blur);
  }
  let diagA = asciiLineMask(localUv, vec2f(0.24, 0.24), vec2f(0.76, 0.76), 0.035, blur);
  let diagB = asciiLineMask(localUv, vec2f(0.76, 0.24), vec2f(0.24, 0.76), 0.035, blur);
  return clamp(diagA + diagB, 0.0, 1.0);
}

fn asciiGlyphIndex(brightness: f32, charsetKind: i32) -> i32 {
  let maxIndex = max(asciiGlyphCount(charsetKind) - 1, 0);
  let scaled = floor(clamp(brightness, 0.0, 1.0) * f32(maxIndex));
  return i32(clamp(scaled, 0.0, f32(maxIndex)));
}

// Sobel edge magnitude sampled at cell scale (steps = one cell), used to drive
// glyph density in edge-detection mode so glyphs trace contours instead of tone.
fn asciiEdgeStrength(centerPx: vec2f, texSize: vec2i, stepX: f32, stepY: f32) -> f32 {
  let sx = max(stepX, 1.0);
  let sy = max(stepY, 1.0);
  let tl = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(-sx, -sy)), texSize).rgb);
  let tc = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(0.0, -sy)), texSize).rgb);
  let tr = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(sx, -sy)), texSize).rgb);
  let ml = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(-sx, 0.0)), texSize).rgb);
  let mr = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(sx, 0.0)), texSize).rgb);
  let bl = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(-sx, sy)), texSize).rgb);
  let bc = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(0.0, sy)), texSize).rgb);
  let br = luminance601(asciiLoadTexel(vec2i(centerPx + vec2f(sx, sy)), texSize).rgb);
  let gx = (tr + 2.0 * mr + br) - (tl + 2.0 * ml + bl);
  let gy = (bl + 2.0 * bc + br) - (tl + 2.0 * tc + tr);
  return clamp(length(vec2f(gx, gy)), 0.0, 1.0);
}

@fragment
fn asciiFragment(input: VertexOutput) -> @location(0) vec4f {
  let texSize = vec2f(params.width, params.height);
  let texSizeI = vec2i(max(i32(params.width), 1), max(i32(params.height), 1));
  let pixelPos = input.uv * texSize;
  let base = asciiLoadTexel(vec2i(pixelPos), texSizeI);
  if (base.a <= 0.0001) {
    return vec4f(0.0);
  }

  let adjustedBase = asciiAdjustColor(base.rgb, params.contrast, params.brightness);
  let background = mix(params.bgColor.rgb, adjustedBase, params.originalOpacity);

  let charAspect = max(0.25, 0.6 + params.letterSpacing * 0.05);
  let cellWidth = max(params.fontSize * charAspect, 1.0);
  let cellHeight = max(params.fontSize * max(params.lineHeight, 0.25), 1.0);
  let cols = max(1.0, floor(params.width / cellWidth));
  let rows = max(1.0, floor(params.height / cellHeight));
  let gridSize = vec2f(cols * cellWidth, rows * cellHeight);
  let origin = (texSize - gridSize) * 0.5;

  let transparent = params.transparentBg >= 0.5;

  if (
    pixelPos.x < origin.x ||
    pixelPos.y < origin.y ||
    pixelPos.x >= origin.x + gridSize.x ||
    pixelPos.y >= origin.y + gridSize.y
  ) {
    // Border (grid letterboxing): no glyphs here. Transparent mode shows only the
    // original at its opacity; opaque mode shows the filled background.
    if (transparent) {
      return vec4f(adjustedBase, base.a * params.originalOpacity);
    }
    return vec4f(background, base.a);
  }

  let gridPos = (pixelPos - origin) / vec2f(cellWidth, cellHeight);
  let cell = floor(gridPos);
  let localUv = fract(gridPos);
  let samplePos = origin + (cell + vec2f(0.5)) * vec2f(cellWidth, cellHeight);
  let sampleColor = asciiLoadTexel(vec2i(samplePos), texSizeI);
  let adjustedSample = asciiAdjustColor(sampleColor.rgb, params.contrast, params.brightness);

  // Glyph density is driven by tone, or by edge magnitude in edge-detection mode.
  var density = luminance601(adjustedSample);
  if (params.edgeDetect >= 0.5) {
    density = asciiEdgeStrength(samplePos, texSizeI, cellWidth, cellHeight);
  }
  if (params.invert >= 0.5) {
    density = 1.0 - density;
  }

  // Atlas charsets (glyphCount > 0) sample real glyphs; otherwise fall back to
  // the procedural mask shapes of the legacy charsets.
  var mask = 0.0;
  if (params.glyphCount > 0.5) {
    let n = i32(params.glyphCount + 0.5);
    let gi = clamp(i32(floor(density * f32(n))), 0, n - 1);
    let lx = clamp(localUv.x, 0.04, 0.96);
    let atlasUv = vec2f((f32(gi) + lx) / f32(n), localUv.y);
    mask = textureSampleLevel(asciiAtlas, texSampler, atlasUv, 0.0).a;
  } else {
    let charsetKind = i32(params.charsetKind + 0.5);
    let glyphIndex = asciiGlyphIndex(density, charsetKind);
    let blur = max(0.5 / min(cellWidth, cellHeight), 0.002);
    mask = asciiGlyphMask(charsetKind, glyphIndex, localUv, blur);
  }

  var glyphColor = params.textColor.rgb;
  if (params.matchSourceColor >= 0.5) {
    glyphColor = asciiApplySaturation(adjustedSample, params.saturation);
  }

  let inkAlpha = clamp(mask * params.asciiOpacity, 0.0, 1.0);

  if (transparent) {
    // Composite glyph ink over the (optional) original underlay over transparency.
    let underA = params.originalOpacity * (1.0 - inkAlpha);
    let outA = inkAlpha + underA;
    let outRGB = (glyphColor * inkAlpha + adjustedBase * underA) / max(outA, 0.0001);
    return vec4f(outRGB, base.a * outA);
  }

  let color = mix(background, glyphColor, inkAlpha);
  return vec4f(color, base.a);
}`,params:{charSet:{type:"select",label:"Character Set",default:"ascii",options:[{value:"ascii",label:"ASCII Ramp"},{value:"dense",label:"Dense"},{value:"binary",label:"Binary"},{value:"symbols",label:"Symbols"},{value:"custom",label:"Custom"},{value:"standard",label:"Standard (shapes)"},{value:"simple",label:"Simple (shapes)"},{value:"blocks",label:"Blocks (shapes)"},{value:"dots",label:"Dots (shapes)"},{value:"minimal",label:"Minimal (shapes)"}]},customChars:{type:"text",label:"Custom Characters",default:"FREECUT 01",visibleWhen:e=>e.charSet==="custom"},font:{type:"select",label:"Font",default:"monospace",options:[{value:"monospace",label:"Monospace"},{value:"courier",label:"Courier"},{value:"consolas",label:"Consolas"},{value:"lucida",label:"Lucida Console"}],visibleWhen:e=>Ta(e.charSet??"ascii",e.customChars??"")!==null},fontSize:{type:"number",label:"Font Size",default:8,min:4,max:24,step:1,animatable:!0},letterSpacing:{type:"number",label:"Letter Spacing",default:0,min:-2,max:5,step:.1,animatable:!0},lineHeight:{type:"number",label:"Line Height",default:1,min:.5,max:2,step:.1,animatable:!0},matchSourceColor:{type:"boolean",label:"Match Source Color",default:!0},textColor:{type:"color",label:"Text Color",default:"#ffffff",animatable:!0,visibleWhen:e=>e.matchSourceColor!==!0},bgColor:{type:"color",label:"Background",default:"#0a0a0f",animatable:!0,visibleWhen:e=>e.transparentBg!==!0},transparentBg:{type:"boolean",label:"Transparent Background",default:!1},edgeDetect:{type:"boolean",label:"Edge Detection",default:!1},colorSaturation:{type:"number",label:"Saturation",default:100,min:0,max:200,step:1,animatable:!0,visibleWhen:e=>e.matchSourceColor===!0},asciiOpacity:{type:"number",label:"ASCII Opacity",default:100,min:0,max:100,step:1,animatable:!0},originalOpacity:{type:"number",label:"Original Opacity",default:0,min:0,max:100,step:1,animatable:!0},contrast:{type:"number",label:"Contrast",default:100,min:50,max:200,step:1,animatable:!0},brightness:{type:"number",label:"Brightness",default:0,min:-100,max:100,step:1,animatable:!0},invert:{type:"boolean",label:"Invert",default:!1}},packUniforms:(e,t,r)=>{const n=Yr(e.textColor??"#ffffff",[1,1,1,1]),a=Yr(e.bgColor??"#0a0a0f",[10/255,10/255,15/255,1]),i=Ta(e.charSet??"ascii",e.customChars??""),o=i?[...i].length:0;return new Float32Array([e.fontSize??8,e.letterSpacing??0,e.lineHeight??1,$c[e.charSet]??$c.standard??0,e.matchSourceColor===!1?0:1,e.invert===!0?1:0,(e.asciiOpacity??100)/100,(e.originalOpacity??0)/100,(e.contrast??100)/100,(e.brightness??0)/255,(e.colorSaturation??100)/100,t,r,e.transparentBg===!0?1:0,e.edgeDetect===!0?1:0,o,n[0],n[1],n[2],n[3],a[0],a[1],a[2],a[3]])},dataTexture:{dimension:"2d",key:e=>{const t=Ta(e.charSet??"ascii",e.customChars??"");return t?`${t}|${e.font??"monospace"}`:""},build:e=>{const t=Ta(e.charSet??"ascii",e.customChars??"");return t?F1(t,e.font??"monospace"):{width:1,height:1,depth:1,data:new Uint8Array([0,0,0,0])}}}},O1={id:"gpu-threshold",name:"Threshold",category:"stylize",entryPoint:"thresholdFragment",uniformSize:16,shader:`
struct ThresholdParams { level: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ThresholdParams;
@fragment
fn thresholdFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let luma = luminance(color.rgb);
  let result = select(0.0, 1.0, luma > params.level);
  return vec4f(vec3f(result), color.a);
}`,params:{level:{type:"number",label:"Level",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.level??.5,0,0,0])},D1={id:"gpu-vhs",name:"VHS",category:"stylize",entryPoint:"vhsFragment",uniformSize:32,shader:`
struct VhsParams {
  bleed: f32, waviness: f32, noise: f32, scanline: f32,
  time: f32, width: f32, height: f32, pad: f32
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: VhsParams;
@fragment
fn vhsFragment(input: VertexOutput) -> @location(0) vec4f {
  let t = params.time;
  var uv = input.uv;

  // horizontal tracking wobble (sin-based, stable at any phase)
  let wave = (sin(uv.y * 120.0 + t * 5.0) + sin(uv.y * 17.0 - t * 2.3)) * 0.5;
  uv.x = uv.x + wave * params.waviness * 0.015;

  // occasional tracking-band jump. Wrap every time-derived term before it feeds
  // hash() — the sin()-based hash collapses to a constant once the unbounded
  // (time * speed) seed grows large, killing the jumps over a session / at speed.
  let bandScroll = (t * 0.7) - floor((t * 0.7) / 64.0) * 64.0;
  let jumpStep = floor(t * 3.0) - floor(floor(t * 3.0) / 64.0) * 64.0;
  let bandId = floor(uv.y * 6.0 + bandScroll);
  let bandHit = step(0.92, hash(vec2f(bandId, jumpStep)));
  uv.x = uv.x + bandHit * (hash(vec2f(bandId, 7.0)) - 0.5) * 0.06;

  // chroma bleed (luma/chroma drift)
  let off = params.bleed * 0.012;
  let r = textureSample(inputTex, texSampler, vec2f(uv.x + off, uv.y)).r;
  let g = textureSample(inputTex, texSampler, uv).g;
  let b = textureSample(inputTex, texSampler, vec2f(uv.x - off, uv.y)).b;
  let a = textureSample(inputTex, texSampler, uv).a;
  var rgb = vec3f(r, g, b);

  // scanlines
  let sl = 0.82 + 0.18 * sin(input.position.y * PI);
  rgb = mix(rgb, rgb * sl, clamp(params.scanline, 0.0, 1.0));

  // tape noise — wrap the (unbounded) time addends so the per-pixel seed stays
  // bounded and the noise doesn't go static after ~30min of playback.
  let tnx = (t * 120.0) - floor((t * 120.0) / 512.0) * 512.0;
  let tny = (t * 60.0) - floor((t * 60.0) / 512.0) * 512.0;
  let n = hash(uv * vec2f(params.width, params.height) * 0.5 + vec2f(tnx, tny)) - 0.5;
  rgb = rgb + n * params.noise * 0.5;

  return vec4f(clamp(rgb, vec3f(0.0), vec3f(1.0)), a);
}`,params:{bleed:{type:"number",label:"Chroma Bleed",default:.4,min:0,max:2,step:.01,animatable:!0},waviness:{type:"number",label:"Tracking",default:.3,min:0,max:2,step:.01,animatable:!0},noise:{type:"number",label:"Tape Noise",default:.25,min:0,max:1,step:.01,animatable:!0},scanline:{type:"number",label:"Scanlines",default:.35,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:0,max:4,step:.1,animatable:!1}},packUniforms:(e,t,r)=>new Float32Array([e.bleed??.4,e.waviness??.3,e.noise??.25,e.scanline??.35,performance.now()/1e3*(e.speed??1),t,r,0])},R1={id:"gpu-ink",name:"Ink",category:"stylize",entryPoint:"inkFragment",uniformSize:64,shader:`
struct InkParams {
  strength: f32, spacing: f32, thickness: f32, edgeStrength: f32,
  tone: f32, width: f32, height: f32, _pad0: f32,
  inkR: f32, inkG: f32, inkB: f32,
  paperR: f32, paperG: f32, paperB: f32,
  _pad1: f32, _pad2: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: InkParams;

// Coverage of a single hatch-line set at \`angle\`, in screen-space pixels.
// Returns 1 on a line, fading to 0 between lines (smoothstep gives spatial AA).
fn inkHatch(p: vec2f, angle: f32, spacing: f32, thickness: f32) -> f32 {
  let s = sin(angle);
  let c = cos(angle);
  let coord = p.x * c - p.y * s;
  let m = coord - floor(coord / spacing) * spacing;
  let d = min(m, spacing - m);
  return 1.0 - smoothstep(thickness * 0.5, thickness * 0.5 + 1.0, d);
}

// A hatch layer that fades in as tone drops below \`hi\` (over a soft window) so
// the four tonal bands blend smoothly instead of popping on gradients.
fn inkLayer(p: vec2f, angle: f32, spacing: f32, thickness: f32, lum: f32, hi: f32) -> f32 {
  let w = 1.0 - smoothstep(hi - 0.15, hi, lum);
  return inkHatch(p, angle, spacing, thickness) * w;
}

@fragment
fn inkFragment(input: VertexOutput) -> @location(0) vec4f {
  let src = textureSample(inputTex, texSampler, input.uv);
  let texel = vec2f(1.0 / params.width, 1.0 / params.height);

  // Sobel magnitude → contour outlines.
  let tl = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(-texel.x, -texel.y)).rgb);
  let t  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(0.0, -texel.y)).rgb);
  let tr = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(texel.x, -texel.y)).rgb);
  let l  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(-texel.x, 0.0)).rgb);
  let r  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(texel.x, 0.0)).rgb);
  let bl = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(-texel.x, texel.y)).rgb);
  let b  = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(0.0, texel.y)).rgb);
  let br = luminance(textureSample(inputTex, texSampler, input.uv + vec2f(texel.x, texel.y)).rgb);
  let gx = -tl - 2.0 * l - bl + tr + 2.0 * r + br;
  let gy = -tl - 2.0 * t - tr + bl + 2.0 * b + br;
  let edge = clamp(sqrt(gx * gx + gy * gy) * params.edgeStrength, 0.0, 1.0);

  let lum = clamp(luminance(src.rgb) * params.tone, 0.0, 1.0);
  let p = input.uv * vec2f(params.width, params.height);
  let sp = max(params.spacing, 1.0);
  let th = params.thickness;

  var hatch = 0.0;
  hatch = max(hatch, inkLayer(p, 0.7854, sp, th, lum, 0.85));   // 45°
  hatch = max(hatch, inkLayer(p, -0.7854, sp, th, lum, 0.65));  // -45°
  hatch = max(hatch, inkLayer(p, 0.0, sp, th, lum, 0.45));      // horizontal
  hatch = max(hatch, inkLayer(p, 1.5708, sp, th, lum, 0.25));   // vertical

  let inkAmt = clamp(max(hatch, edge) * params.strength, 0.0, 1.0);
  let paper = vec3f(params.paperR, params.paperG, params.paperB);
  let inkColor = vec3f(params.inkR, params.inkG, params.inkB);
  return vec4f(mix(paper, inkColor, inkAmt), src.a);
}`,params:{strength:{type:"number",label:"Ink Amount",default:1,min:0,max:1,step:.01,animatable:!0},spacing:{type:"number",label:"Line Spacing",default:6,min:2,max:24,step:.5,animatable:!0},thickness:{type:"number",label:"Line Width",default:1.2,min:.5,max:4,step:.1,animatable:!0},edgeStrength:{type:"number",label:"Outline",default:1.5,min:0,max:5,step:.1,animatable:!0},tone:{type:"number",label:"Shading",default:1,min:.2,max:2.5,step:.05,animatable:!0},inkColor:{type:"color",label:"Ink",default:"#141414",animatable:!0},paperColor:{type:"color",label:"Paper",default:"#f4f1e8",animatable:!0}},packUniforms:(e,t,r)=>{const n=Yr(e.inkColor??"#141414",[.08,.08,.08,1]),a=Yr(e.paperColor??"#f4f1e8",[.96,.95,.91,1]);return new Float32Array([e.strength??1,e.spacing??6,e.thickness??1.2,e.edgeStrength??1.5,e.tone??1,t,r,0,n[0],n[1],n[2],a[0],a[1],a[2],0,0])}},L1={id:"gpu-pixel-sort",name:"Pixel Sort (Streak)",category:"stylize",entryPoint:"pixelSortFragment",uniformSize:32,shader:`
struct PixelSortParams {
  dirX: f32, dirY: f32, low: f32, high: f32,
  length: f32, order: f32, width: f32, height: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: PixelSortParams;

// Statically-bounded scan cap keeps the loop compilable and its cost bounded;
// the animatable \`length\` param clamps the effective steps below this ceiling.
const PS_MAX_STEPS: i32 = 512;

// Sort key: carry the brightest value (order>0.5) or the darkest (order<=0.5).
fn psKey(lum: f32, order: f32) -> f32 {
  return select(-lum, lum, order > 0.5);
}

@fragment
fn pixelSortFragment(input: VertexOutput) -> @location(0) vec4f {
  let src = textureSample(inputTex, texSampler, input.uv);
  let lum0 = luminance(src.rgb);

  // Pixels outside the brightness band are left untouched — only masked spans
  // are sorted (the classic pixel-sort threshold behaviour).
  if (lum0 < params.low || lum0 > params.high) {
    return src;
  }

  let texel = vec2f(1.0 / params.width, 1.0 / params.height);
  let stepUv = vec2f(params.dirX, params.dirY) * texel;
  let maxN = i32(clamp(params.length, 1.0, f32(PS_MAX_STEPS)));

  var bestKey = psKey(lum0, params.order);
  var bestColor = src.rgb;

  for (var i: i32 = 1; i <= PS_MAX_STEPS; i = i + 1) {
    if (i > maxN) { break; }
    let uv = input.uv + stepUv * f32(i);
    if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0) { break; }
    let s = textureSampleLevel(inputTex, texSampler, uv, 0.0);
    let l = luminance(s.rgb);
    if (l < params.low || l > params.high) { break; } // span boundary
    let k = psKey(l, params.order);
    if (k > bestKey) {
      bestKey = k;
      bestColor = s.rgb;
    }
  }
  return vec4f(bestColor, src.a);
}`,params:{direction:{type:"select",label:"Direction",default:"right",options:[{value:"right",label:"Right"},{value:"left",label:"Left"},{value:"down",label:"Down"},{value:"up",label:"Up"}]},order:{type:"select",label:"Carry",default:"bright",options:[{value:"bright",label:"Bright streaks"},{value:"dark",label:"Dark streaks"}]},low:{type:"number",label:"Threshold Low",default:.25,min:0,max:1,step:.01,animatable:!0},high:{type:"number",label:"Threshold High",default:1,min:0,max:1,step:.01,animatable:!0},length:{type:"number",label:"Length",default:60,min:2,max:400,step:1,quality:!0}},packUniforms:(e,t,r)=>{const n=e.direction??"right";let a=1,i=0;return n==="left"?(a=-1,i=0):n==="down"?(a=0,i=1):n==="up"&&(a=0,i=-1),new Float32Array([a,i,e.low??.25,e.high??1,e.length??60,e.order==="dark"?0:1,t,r])}},B1={id:"gpu-pixel-sort-hq",name:"Pixel Sort",category:"stylize",entryPoint:"pixelSortHqMain",uniformSize:32,compute:{dispatch:(e,t)=>[Math.ceil(e/8),Math.ceil(t/8),1]},shader:`
struct PixelSortHqParams {
  low: f32, high: f32, vertical: f32, descending: f32,
  width: f32, height: f32, _p0: f32, _p1: f32,
};
@group(0) @binding(0) var inputTex: texture_2d<f32>;
@group(0) @binding(1) var outputTex: texture_storage_2d<rgba8unorm, write>;
@group(0) @binding(2) var<uniform> params: PixelSortHqParams;

fn psInBand(l: f32) -> bool {
  return l >= params.low && l <= params.high;
}

// Luminance at an axis index \`i\` (perpendicular coordinate fixed at \`perp\`).
fn psLumAt(i: i32, perp: i32, vertical: bool) -> f32 {
  let c = select(vec2i(i, perp), vec2i(perp, i), vertical);
  return luminance(textureLoad(inputTex, c, 0).rgb);
}

@compute @workgroup_size(8, 8, 1)
fn pixelSortHqMain(@builtin(global_invocation_id) gid: vec3u) {
  let W = i32(params.width);
  let H = i32(params.height);
  let x = i32(gid.x);
  let y = i32(gid.y);
  if (x >= W || y >= H) { return; }

  let selfColor = textureLoad(inputTex, vec2i(x, y), 0);
  let selfLum = luminance(selfColor.rgb);

  // Pixels outside the brightness band stay exactly where they are.
  if (!psInBand(selfLum)) {
    textureStore(outputTex, vec2i(x, y), selfColor);
    return;
  }

  let vertical = params.vertical > 0.5;
  let axisLen = select(W, H, vertical);
  let perp = select(y, x, vertical);
  let pos = select(x, y, vertical);

  // Grow the contiguous masked span [s, e] that contains this pixel.
  var s = pos;
  loop {
    if (s == 0) { break; }
    if (!psInBand(psLumAt(s - 1, perp, vertical))) { break; }
    s = s - 1;
  }
  var e = pos;
  loop {
    if (e == axisLen - 1) { break; }
    if (!psInBand(psLumAt(e + 1, perp, vertical))) { break; }
    e = e + 1;
  }

  // Rank = count of span pixels that sort before this one. Equal luminances are
  // ordered by original index so ranks are unique across the span (a bijection
  // onto [s, e]) — every destination texel receives exactly one writer.
  var rank = 0;
  for (var i = s; i <= e; i = i + 1) {
    if (i == pos) { continue; }
    let li = psLumAt(i, perp, vertical);
    if (li < selfLum || (li == selfLum && i < pos)) {
      rank = rank + 1;
    }
  }

  // Ascending places the darkest at \`s\`; descending flips the span end-for-end.
  let dstIndex = select(s + rank, e - rank, params.descending > 0.5);
  let outCoord = select(vec2i(dstIndex, perp), vec2i(perp, dstIndex), vertical);
  textureStore(outputTex, outCoord, selfColor);
}`,params:{orientation:{type:"select",label:"Orientation",default:"horizontal",options:[{value:"horizontal",label:"Horizontal"},{value:"vertical",label:"Vertical"}]},order:{type:"select",label:"Order",default:"ascending",options:[{value:"ascending",label:"Dark → Bright"},{value:"descending",label:"Bright → Dark"}]},low:{type:"number",label:"Threshold Low",default:.25,min:0,max:1,step:.01,animatable:!0},high:{type:"number",label:"Threshold High",default:.9,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.low??.25,e.high??.9,e.orientation==="vertical"?1:0,e.order==="descending"?1:0,t,r,0,0])};var z1=Object.freeze({__proto__:null,ascii:_1,blockGlitch:x1,colorGlitch:I1,crt:w1,dither:A1,edgeDetect:b1,glow:v1,grain:h1,halftone:C1,ink:R1,pixelSort:L1,pixelSortHq:B1,posterize:y1,scanlines:S1,sharpen:g1,threshold:O1,vhs:D1,vignette:f1});function U1(e){if(typeof e=="string"){const t=/^#([0-9a-f]{6})(?:[0-9a-f]{2})?$/i.exec(e.trim());if(t){const r=t[1];return[parseInt(r.slice(0,2),16)/255,parseInt(r.slice(2,4),16)/255,parseInt(r.slice(4,6),16)/255]}}return[0,1,0]}const H1={id:"gpu-chroma-key",name:"Chroma Key",category:"keying",entryPoint:"chromaKeyFragment",uniformSize:32,shader:`
struct ChromaKeyParams {
  keyR: f32, keyG: f32, keyB: f32, tolerance: f32,
  softness: f32, spillSuppression: f32, _p1: f32, _p2: f32,
};
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ChromaKeyParams;

fn rgb2ycbcr(rgb: vec3f) -> vec3f {
  let y = 0.299 * rgb.r + 0.587 * rgb.g + 0.114 * rgb.b;
  let cb = 0.564 * (rgb.b - y);
  let cr = 0.713 * (rgb.r - y);
  return vec3f(y, cb, cr);
}

@fragment
fn chromaKeyFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let keyColor = vec3f(params.keyR, params.keyG, params.keyB);
  let colorYCbCr = rgb2ycbcr(color.rgb);
  let keyYCbCr = rgb2ycbcr(keyColor);
  let cbcrDist = length(colorYCbCr.yz - keyYCbCr.yz);
  let innerTolerance = params.tolerance;
  let outerTolerance = params.tolerance + params.softness;
  var alpha = smoothstep(innerTolerance, outerTolerance, cbcrDist);
  var finalColor = color.rgb;
  if (params.spillSuppression > 0.0) {
    // Suppress spill on whichever channel dominates the key colour; otherwise
    // fall back to a generic desaturation keyed to hue distance.
    if (params.keyG >= params.keyR && params.keyG >= params.keyB) {
      let spillAmount = max(0.0, finalColor.g - max(finalColor.r, finalColor.b)) * params.spillSuppression;
      finalColor.g -= spillAmount;
      finalColor.r += spillAmount * 0.5;
      finalColor.b += spillAmount * 0.5;
    } else if (params.keyB >= params.keyR && params.keyB >= params.keyG) {
      let spillAmount = max(0.0, finalColor.b - max(finalColor.r, finalColor.g)) * params.spillSuppression;
      finalColor.b -= spillAmount;
      finalColor.r += spillAmount * 0.5;
      finalColor.g += spillAmount * 0.5;
    } else {
      let spillAmount = max(0.0, finalColor.r - max(finalColor.g, finalColor.b)) * params.spillSuppression;
      finalColor.r -= spillAmount;
      finalColor.g += spillAmount * 0.5;
      finalColor.b += spillAmount * 0.5;
    }
  }
  return vec4f(finalColor, color.a * alpha);
}`,params:{keyColor:{type:"color",label:"Key Color",default:"#00ff00",animatable:!0},tolerance:{type:"number",label:"Tolerance",default:.2,min:0,max:1,step:.01,animatable:!0},softness:{type:"number",label:"Edge Softness",default:.1,min:0,max:.5,step:.01,animatable:!0},spillSuppression:{type:"number",label:"Spill Suppression",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>{const[t,r,n]=U1(e.keyColor);return new Float32Array([t,r,n,e.tolerance??.2,e.softness??.1,e.spillSuppression??.5,0,0])}};var N1=Object.freeze({__proto__:null,chromaKey:H1});const Zp=new Map,q1={color:[],blur:[],distort:[],stylize:[],keying:[]};function G1(e){return typeof e=="object"&&e!==null&&"id"in e&&"name"in e&&"category"in e&&"shader"in e&&"entryPoint"in e&&"packUniforms"in e}function rn(e){Object.values(e).forEach(t=>{G1(t)&&(Zp.set(t.id,t),q1[t.category]?.push(t))})}rn(Gx);rn(jx);rn(Zx);rn(p1);rn(z1);rn(N1);function K1(e){return Zp.get(e)}const V1={id:"dissolve",name:"Cross Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"dissolveFragment",uniformSize:16,shader:`
struct DissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  _pad: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: DissolveParams;

@fragment
fn dissolveFragment(input: VertexOutput) -> @location(0) vec4f {
  let left = textureSample(leftTex, texSampler, input.uv);
  let right = textureSample(rightTex, texSampler, input.uv);
  let t = 0.5 - 0.5 * cos(clamp(params.progress, 0.0, 1.0) * PI);
  return mix(left, right, t);
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},ca=`
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
`,ef=`
fn sampleSoft(tex: texture_2d<f32>, uv: vec2f, radius: vec2f) -> vec4f {
  let center = textureSample(tex, texSampler, uv);
  let a = textureSample(tex, texSampler, clamp(uv + vec2f(radius.x, 0.0), vec2f(0.0), vec2f(1.0)));
  let b = textureSample(tex, texSampler, clamp(uv - vec2f(radius.x, 0.0), vec2f(0.0), vec2f(1.0)));
  let c = textureSample(tex, texSampler, clamp(uv + vec2f(0.0, radius.y), vec2f(0.0), vec2f(1.0)));
  let d = textureSample(tex, texSampler, clamp(uv - vec2f(0.0, radius.y), vec2f(0.0), vec2f(1.0)));
  return center * 0.36 + (a + b + c + d) * 0.16;
}
`,W1={id:"additiveDissolve",name:"Additive Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"additiveDissolveFragment",uniformSize:16,shader:`
struct AdditiveDissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  _pad: f32,
};

${ca}
@group(0) @binding(3) var<uniform> params: AdditiveDissolveParams;

@fragment
fn additiveDissolveFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);
  let left = textureSample(leftTex, texSampler, input.uv);
  let right = textureSample(rightTex, texSampler, input.uv);
  let base = left.rgb * (1.0 - p) + right.rgb * p;
  let flash = (left.rgb + right.rgb) * sin(p * PI) * 0.22;
  return vec4f(clamp(base + flash, vec3f(0.0), vec3f(1.0)), mix(left.a, right.a, p));
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},j1={id:"blurDissolve",name:"Blur Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"blurDissolveFragment",uniformSize:16,shader:`
struct BlurDissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  strength: f32,
};

${ca}
@group(0) @binding(3) var<uniform> params: BlurDissolveParams;
${ef}

@fragment
fn blurDissolveFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);
  let envelope = sin(p * PI);
  let radius = vec2f(1.0 / params.width, 1.0 / params.height) * params.strength * envelope;
  let left = sampleSoft(leftTex, input.uv, radius);
  let right = sampleSoft(rightTex, input.uv, radius);
  let t = 0.5 - 0.5 * cos(p * PI);
  return mix(left, right, t);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.strength??9;return new Float32Array([e,t,r,i])}},$1={id:"dipToColorDissolve",name:"Dip To Color Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"dipToColorDissolveFragment",uniformSize:32,shader:`
struct DipToColorDissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  colorR: f32,
  colorG: f32,
  colorB: f32,
  _pad1: f32,
  _pad2: f32,
};

${ca}
@group(0) @binding(3) var<uniform> params: DipToColorDissolveParams;

@fragment
fn dipToColorDissolveFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);
  let left = textureSample(leftTex, texSampler, input.uv);
  let right = textureSample(rightTex, texSampler, input.uv);
  let color = vec4f(params.colorR, params.colorG, params.colorB, 1.0);
  let firstHalf = mix(left, color, smoothstep(0.0, 0.5, p));
  let secondHalf = mix(color, right, smoothstep(0.5, 1.0, p));
  return select(secondHalf, firstHalf, p < 0.5);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.color,o=Array.isArray(i)?i:[0,0,0];return new Float32Array([e,t,r,o[0]??0,o[1]??0,o[2]??0,0,0])}},Y1={id:"nonAdditiveDissolve",name:"Non-Additive Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"nonAdditiveDissolveFragment",uniformSize:16,shader:`
struct NonAdditiveDissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  _pad: f32,
};

${ca}
@group(0) @binding(3) var<uniform> params: NonAdditiveDissolveParams;

@fragment
fn nonAdditiveDissolveFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);
  let left = textureSample(leftTex, texSampler, input.uv);
  let right = textureSample(rightTex, texSampler, input.uv);
  let neutral = mix(left.rgb, right.rgb, p);
  let luma = dot(neutral, vec3f(0.2126, 0.7152, 0.0722));
  let guarded = mix(neutral, min(neutral, vec3f(luma + 0.18)), sin(p * PI) * 0.18);
  return vec4f(clamp(guarded, vec3f(0.0), vec3f(1.0)), mix(left.a, right.a, p));
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},X1={id:"smoothCut",name:"Smooth Cut",category:"dissolve",hasDirection:!1,entryPoint:"smoothCutFragment",uniformSize:16,shader:`
struct SmoothCutParams {
  progress: f32,
  width: f32,
  height: f32,
  strength: f32,
};

${ca}
@group(0) @binding(3) var<uniform> params: SmoothCutParams;
${ef}

fn smoothCutWarp(uv: vec2f, p: f32, envelope: f32, strength: f32) -> vec2f {
  let low = fbm(uv * vec2f(2.4, 1.8) + vec2f(p * 1.15, -p * 0.8));
  let mid = fbm(uv * vec2f(6.2, 4.8) + vec2f(-p * 1.65, p * 1.25));
  let horizontalBands =
    sin((uv.x * 4.6 + low * 1.8 + p * 1.25) * TAU) * 0.62 +
    sin((uv.x * 8.2 + uv.y * 0.22 + mid * 1.1 - p * 0.9) * TAU) * 0.28 +
    (low - 0.5) * 0.48 +
    (mid - 0.5) * 0.22;
  let horizontalWarp = horizontalBands;
  let verticalWarp = (mid - 0.5) * 0.14;
  let edgeFade = smoothstep(0.03, 0.18, uv.x) * smoothstep(0.97, 0.82, uv.x)
    * smoothstep(0.03, 0.18, uv.y) * smoothstep(0.97, 0.82, uv.y);
  return vec2f(horizontalWarp, verticalWarp) * envelope * strength * 0.052 * edgeFade;
}

@fragment
fn smoothCutFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);
  let strength = clamp(params.strength, 0.0, 2.0);
  let envelope = sin(p * PI);
  let warp = smoothCutWarp(input.uv, p, envelope, strength);
  let drift = vec2f((p - 0.5) * 0.018 * envelope * strength, 0.0);
  let radius = vec2f(1.0 / params.width, 1.0 / params.height) * envelope * strength * 2.4;
  let leftWarped = sampleSoft(leftTex, clamp(input.uv + warp - drift, vec2f(0.0), vec2f(1.0)), radius);
  let rightWarped = sampleSoft(rightTex, clamp(input.uv - warp + drift, vec2f(0.0), vec2f(1.0)), radius);
  let leftClean = textureSample(leftTex, texSampler, input.uv);
  let rightClean = textureSample(rightTex, texSampler, input.uv);
  let warpMix = smoothstep(0.12, 0.85, envelope);
  let left = mix(leftClean, leftWarped, warpMix);
  let right = mix(rightClean, rightWarped, warpMix);
  let blendWidth = mix(0.18, 0.36, strength);
  let t = smoothstep(0.5 - blendWidth, 0.5 + blendWidth, p);
  return mix(left, right, t);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.strength??.9;return new Float32Array([e,t,r,i])}},Q1={id:"sparkles",name:"Sparkles",category:"custom",hasDirection:!1,entryPoint:"sparklesFragment",uniformSize:32,shader:`
struct SparklesParams {
  progress: f32,
  width: f32,
  height: f32,
  sparkleScale: f32,
  intensity: f32,
  density: f32,
  glow: f32,
  _pad: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: SparklesParams;

fn rotateVec2(v: vec2f, angle: f32) -> vec2f {
  let c = cos(angle);
  let s = sin(angle);
  return vec2f(
    (v.x * c) - (v.y * s),
    (v.x * s) + (v.y * c)
  );
}

fn sparkleShape(local: vec2f, size: f32) -> f32 {
  let absLocal = abs(local);
  let core = smoothstep(size * 0.34, 0.0, length(local));
  let horizontal = smoothstep(size * 0.14, 0.0, absLocal.y)
    * smoothstep(size, 0.0, absLocal.x);
  let vertical = smoothstep(size * 0.14, 0.0, absLocal.x)
    * smoothstep(size, 0.0, absLocal.y);
  let diagonalA = smoothstep(size * 0.22, 0.0, abs(local.x - local.y))
    * smoothstep(size * 0.9, 0.0, length(local));
  let diagonalB = smoothstep(size * 0.22, 0.0, abs(local.x + local.y))
    * smoothstep(size * 0.9, 0.0, length(local));
  return max(core, max(horizontal, max(vertical, max(diagonalA, diagonalB) * 0.7)));
}

fn sparkleLayer(
  scaledUv: vec2f,
  progress: f32,
  density: f32,
  sizeBase: f32,
  sizeVariance: f32,
  motionScale: f32,
  threshold: f32,
  phase: f32,
) -> vec4f {
  let cellUv = scaledUv * density;
  let cell = floor(cellUv);
  let local = fract(cellUv) - vec2f(0.5, 0.5);
  let seed = hash(cell + vec2f(phase * 1.37, phase * 2.11));
  let centerSeed = vec2f(
    hash(cell + vec2f(phase + 1.7, phase + 6.2)),
    hash(cell + vec2f(phase + 8.4, phase + 3.1))
  ) - vec2f(0.5, 0.5);
  let sizeSeed = hash(cell + vec2f(phase + 2.4, phase + 9.7));
  let orbitSeed = hash(cell + vec2f(phase + 4.6, phase + 11.2));
  let ignitePoint = clamp(
    0.04
      + (seed * 0.72)
      + (noise2d((cell * 0.17) + vec2f(phase * 0.31, phase * 0.67)) * 0.16),
    0.04,
    0.94
  );
  let igniteDuration = 0.14 + (sizeSeed * 0.18);
  let igniteProgress = clamp((progress - ignitePoint) / igniteDuration, 0.0, 1.0);
  let igniteIn = smoothstep(0.0, 0.16, igniteProgress);
  let igniteOut = 1.0 - smoothstep(0.3, 0.95, igniteProgress);
  let pulse = igniteIn * igniteOut;
  let afterglow = smoothstep(0.06, 0.72, igniteProgress);

  let directionAngle = seed * TAU;
  let direction = vec2f(cos(directionAngle), sin(directionAngle));
  let motionEnvelope = pulse * (0.6 + (0.4 * sin(progress * PI)));
  let drift = direction
    * motionScale
    * (0.42 + (sizeSeed * 1.45))
    * motionEnvelope;
  let orbit = rotateVec2(
    vec2f(0.0, 1.0),
    directionAngle + (igniteProgress * (1.5 + (orbitSeed * 2.6)) * PI)
  ) * motionScale * 0.62 * (0.28 + orbitSeed) * motionEnvelope;

  let center = (centerSeed * 0.72) + drift + orbit;
  let rotation = (seed * TAU) + (igniteProgress * (1.2 + (sizeSeed * 3.1)) * PI);
  let size = sizeBase + (sizeSeed * sizeVariance);
  let twinkle = 0.35 + (0.65 * ((sin((igniteProgress * (2.8 + (sizeSeed * 4.5)) + seed) * TAU) + 1.0) * 0.5));
  let activation = smoothstep(threshold, 1.0, seed);

  let starLocal = rotateVec2(local - center, rotation);
  let main = sparkleShape(starLocal, size) * activation * twinkle * pulse;

  let trailCenter = center - (
    direction
    * motionScale
    * (0.6 + sizeSeed)
    * (0.25 + (pulse * 0.95))
  );
  let trailLocal = rotateVec2(local - trailCenter, rotation - 0.4);
  let trailShape = vec2f(trailLocal.x * 1.7, trailLocal.y * 0.58);
  let trail = sparkleShape(trailShape, size * 0.72)
    * activation
    * twinkle
    * pulse
    * (0.42 + (sizeSeed * 0.28));

  let dustNoise = noise2d((cell * 0.85) + vec2f((igniteProgress * 4.2) + phase, phase * 0.37));
  let dust = smoothstep(0.62, 1.0, dustNoise)
    * afterglow
    * activation
    * (0.16 + (sizeSeed * 0.34));
  let reveal = clamp(
    (main * 0.72)
      + (trail * 0.34)
      + (afterglow * activation * 0.24)
      + (dust * 0.12),
    0.0,
    1.0
  );
  let glow = max(main, trail * 0.88) * (0.65 + (sizeSeed * 0.75)) + (dust * 0.3);

  return vec4f(main, reveal, glow, seed);
}

@fragment
fn sparklesFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = params.progress;
  let aspect = params.width / max(params.height, 1.0);
  let scaledUv = vec2f(uv.x * aspect, uv.y);

  let left = textureSample(leftTex, texSampler, uv);
  let right = textureSample(rightTex, texSampler, uv);

  let coarseLayer = sparkleLayer(
    scaledUv,
    p,
    5.5 + (params.density * 5.0),
    0.15 * params.sparkleScale,
    0.28 * params.sparkleScale,
    0.18,
    0.58,
    3.7
  );
  let microLayer = sparkleLayer(
    scaledUv,
    p,
    10.0 + (params.density * 9.0),
    0.06 * params.sparkleScale,
    0.14 * params.sparkleScale,
    0.09,
    0.7,
    11.4
  );

  let heroMix = step(microLayer.z, coarseLayer.z);
  let heroSeed = mix(microLayer.w, coarseLayer.w, heroMix);
  let sparkleCore = max(coarseLayer.x, microLayer.x * 0.78);
  let sparkleField = max(coarseLayer.y, microLayer.y * 0.86);
  let glowField = max(coarseLayer.z, microLayer.z * 0.74);
  let macroNoise = fbm((scaledUv * (2.6 + (params.density * 0.9))) + vec2f(0.0, p * 0.7));
  let dustNoise = noise2d(
    (scaledUv * (13.0 + (params.density * 7.0)))
      + vec2f((p * 5.2) + (heroSeed * 3.1), heroSeed * 7.4)
  );
  let dissolveCurve = smoothstep(0.03, 0.97, p);
  let sparkleWindow = smoothstep(0.02, 0.28, p) * (1.0 - smoothstep(0.8, 1.0, p));
  let dustField = smoothstep(0.58, 1.0, dustNoise)
    * (0.12 + (sparkleField * 0.88))
    * sin(p * PI);
  let thresholdMap = clamp(
    (macroNoise * 0.58)
      + (dustNoise * 0.14)
      + ((1.0 - sparkleField) * 0.18)
      + ((1.0 - glowField) * 0.08),
    0.0,
    1.0
  );
  let dissolveProgress = clamp(
    (dissolveCurve * 1.08) - 0.04
      + (sparkleField * (0.28 + (params.intensity * 0.12)) * sparkleWindow)
      + (glowField * (0.1 + (params.glow * 0.08)))
      + (dustField * 0.12),
    0.0,
    1.0
  );
  let edge = 0.075 + (0.018 * params.sparkleScale);
  let leftPresence = 1.0 - smoothstep(thresholdMap - edge, thresholdMap + edge, dissolveProgress);
  let rightPresence = 1.0 - leftPresence;

  var color = mix(right, left, leftPresence);
  let dissolveEdge = clamp(leftPresence * rightPresence * 4.0, 0.0, 1.0);
  let sparkleEnvelope = sin(p * PI);
  let edgeGlow = dissolveEdge
    * glowField
    * params.intensity
    * params.glow
    * (0.5 + (sparkleEnvelope * 0.4));
  let sparkleFlash = sparkleCore
    * (0.38 + (params.intensity * 0.52))
    * (0.62 + (sparkleEnvelope * 0.38));
  let glowColor = mix(vec3f(1.0, 0.97, 0.88), vec3f(1.0, 0.82, 0.56), heroSeed);
  let warmVeil = glowColor * glowField * params.glow * (0.06 + (rightPresence * 0.12));
  let incomingLift = right.rgb * (glowField * rightPresence * 0.05 * params.glow);
  let lifted = color.rgb
    + warmVeil
    + incomingLift
    + (glowColor * edgeGlow * 0.95)
    + (glowColor * sparkleFlash * 0.72);
  let compressed = 1.0 - exp(-lifted * (1.0 + (edgeGlow * 0.45)));

  return vec4f(
    clamp(mix(lifted, compressed, 0.46), vec3f(0.0), vec3f(1.0)),
    color.a
  );
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.sparkleScale??1,o=a?.intensity??1,s=a?.density??1,l=a?.glow??1;return new Float32Array([e,t,r,i,o,s,l,0])}},J1={id:"glitch",name:"Glitch",category:"custom",hasDirection:!1,entryPoint:"glitchFragment",uniformSize:32,shader:`
struct GlitchParams {
  progress: f32,
  width: f32,
  height: f32,
  intensity: f32,
  blockSize: f32,
  rgbSplit: f32,
  _pad1: f32,
  _pad2: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: GlitchParams;

@fragment
fn glitchFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = params.progress;

  // Intensity envelope — ramps up, holds, eases out
  let envelope = smoothstep(0.0, 0.2, p) * (1.0 - smoothstep(0.8, 1.0, p));
  let strength = envelope * params.intensity;

  // --- Block displacement (two scales) ---
  let bigBlockH = params.blockSize * 2.0 / params.height;
  let bigBlockY = floor(uv.y / bigBlockH);
  let bigSeed = hash(vec2f(bigBlockY * 17.3, floor(p * 8.0)));
  let bigActive = step(0.55 - strength * 0.35, bigSeed);
  let bigShift = (hash(vec2f(bigBlockY * 31.7, floor(p * 10.0))) - 0.5)
                 * strength * 0.18 * bigActive;

  let sliceH = max(2.0, params.blockSize * 0.3) / params.height;
  let sliceY = floor(uv.y / sliceH);
  let sliceSeed = hash(vec2f(sliceY * 53.1, floor(p * 12.0)));
  let sliceActive = step(0.65 - strength * 0.25, sliceSeed);
  let sliceShift = (hash(vec2f(sliceY * 71.3, floor(p * 14.0))) - 0.5)
                   * strength * 0.1 * sliceActive;

  let totalShift = bigShift + sliceShift;
  let dUv = vec2f(clamp(uv.x + totalShift, 0.0, 1.0), uv.y);

  // --- RGB split (horizontal chromatic aberration) ---
  let split = params.rgbSplit * strength * 0.015 + abs(totalShift) * 0.2;
  let uvR = vec2f(clamp(dUv.x + split, 0.0, 1.0), dUv.y);
  let uvB = vec2f(clamp(dUv.x - split, 0.0, 1.0), dUv.y);

  // --- Per-block clip switching ---
  let switchBlockH = bigBlockH * 0.7;
  let switchY = floor(uv.y / switchBlockH);
  let switchSeed = hash(vec2f(switchY * 7.3, floor(p * 6.0)));
  let threshold = switchSeed * 0.7 + 0.15;
  let useRight = smoothstep(threshold - 0.12, threshold + 0.12, p);

  // Sample both clips with chromatic aberration
  let lR = textureSample(leftTex, texSampler, uvR).r;
  let lG = textureSample(leftTex, texSampler, dUv).g;
  let lB = textureSample(leftTex, texSampler, uvB).b;
  let lA = textureSample(leftTex, texSampler, dUv).a;
  let leftColor = vec4f(lR, lG, lB, lA);

  let rR = textureSample(rightTex, texSampler, uvR).r;
  let rG = textureSample(rightTex, texSampler, dUv).g;
  let rB = textureSample(rightTex, texSampler, uvB).b;
  let rA = textureSample(rightTex, texSampler, dUv).a;
  let rightColor = vec4f(rR, rG, rB, rA);

  // Detect truly empty samples using the alpha channel; fall back to
  // luminance only when alpha is not a reliable signal.
  let leftEmpty  = leftColor.a  < 0.01;
  let rightEmpty = rightColor.a < 0.01;
  let safeLeft  = select(leftColor,  rightColor, leftEmpty  && !rightEmpty);
  let safeRight = select(rightColor, leftColor,  rightEmpty && !leftEmpty);

  var color = mix(safeLeft, safeRight, useRight);

  // --- Digital noise on glitched regions ---
  let noiseSeed = hash(vec2f(uv.x * params.width * 0.5,
                              uv.y * params.height * 0.5 + p * 1000.0));
  let noiseAmt = strength * 0.1 * max(bigActive, sliceActive);
  color = vec4f(mix(color.rgb, vec3f(noiseSeed), noiseAmt), color.a);

  // --- Subtle posterization on displaced blocks ---
  let levels = mix(256.0, 24.0, strength * bigActive * 0.4);
  color = vec4f(floor(color.rgb * levels + 0.5) / levels, color.a);

  return color;
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.intensity??1,o=a?.blockSize??30,s=a?.rgbSplit??1;return new Float32Array([e,t,r,i,o,s,0,0])}},Z1={id:"pixelate",name:"Pixelate",category:"custom",hasDirection:!1,entryPoint:"pixelateFragment",uniformSize:32,shader:`
struct PixelateParams {
  progress: f32,
  // Pre-computed on CPU: block size in UV space (1/width * blockPx, 1/height * blockPx)
  blockU: f32,
  blockV: f32,
  crossfade: f32,
  _pad1: f32,
  _pad2: f32,
  _pad3: f32,
  _pad4: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: PixelateParams;

@fragment
fn pixelateFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;

  // Snap UV to block grid center (block size pre-computed on CPU)
  let snappedUv = clamp(
    floor(uv / vec2f(params.blockU, params.blockV)) * vec2f(params.blockU, params.blockV)
      + vec2f(params.blockU, params.blockV) * 0.5,
    vec2f(0.0),
    vec2f(1.0)
  );

  // Sample both clips at the pixelated UV — only 2 texture reads
  let left = textureSample(leftTex, texSampler, snappedUv);
  let right = textureSample(rightTex, texSampler, snappedUv);

  return mix(left, right, params.crossfade);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.maxBlockSize??48,o=1-Math.abs(e*2-1),s=o*o,l=Math.max(1,s*i),c=l/t,u=l/r,d=Math.max(0,Math.min(1,(e-.45)/.1)),m=d*d*(3-2*d);return new Float32Array([e,c,u,m,0,0,0,0])}},ew={id:"chromatic",name:"Chromatic",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"chromaticFragment",uniformSize:32,shader:`
struct ChromaticParams {
  progress: f32,
  width: f32,
  height: f32,
  direction: f32,
  spread: f32,
  intensity: f32,
  _pad1: f32,
  _pad2: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: ChromaticParams;

@fragment
fn chromaticFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = params.progress;

  // Intensity envelope — strongest at midpoint
  let envelope = sin(p * PI);
  let strength = envelope * params.intensity;

  // Direction vector for the aberration spread
  let dir = u32(params.direction);
  var aberrationDir: vec2f;
  if (dir == 0u) { aberrationDir = vec2f(1.0, 0.0); }
  else if (dir == 1u) { aberrationDir = vec2f(-1.0, 0.0); }
  else if (dir == 2u) { aberrationDir = vec2f(0.0, 1.0); }
  else { aberrationDir = vec2f(0.0, -1.0); }

  // RGB channel offsets — each channel shifts at different rate
  let spreadAmount = params.spread * strength * 0.02;
  let rOffset = aberrationDir * spreadAmount * 1.0;
  let gOffset = aberrationDir * spreadAmount * 0.0;  // Green stays centered
  let bOffset = aberrationDir * spreadAmount * -1.0;

  // Add slight radial component for lens-like feel
  let center = uv - vec2f(0.5);
  let radialOffset = center * strength * 0.01;

  // Sample outgoing clip (left) with aberration
  let leftR = textureSample(leftTex, texSampler, clamp(uv + rOffset + radialOffset, vec2f(0.0), vec2f(1.0))).r;
  let leftG = textureSample(leftTex, texSampler, clamp(uv + gOffset, vec2f(0.0), vec2f(1.0))).g;
  let leftB = textureSample(leftTex, texSampler, clamp(uv + bOffset - radialOffset, vec2f(0.0), vec2f(1.0))).b;
  let leftA = textureSample(leftTex, texSampler, uv).a;
  let leftColor = vec4f(leftR, leftG, leftB, leftA);

  // Sample incoming clip (right) with aberration
  let rightR = textureSample(rightTex, texSampler, clamp(uv + rOffset + radialOffset, vec2f(0.0), vec2f(1.0))).r;
  let rightG = textureSample(rightTex, texSampler, clamp(uv + gOffset, vec2f(0.0), vec2f(1.0))).g;
  let rightB = textureSample(rightTex, texSampler, clamp(uv + bOffset - radialOffset, vec2f(0.0), vec2f(1.0))).b;
  let rightA = textureSample(rightTex, texSampler, uv).a;
  let rightColor = vec4f(rightR, rightG, rightB, rightA);

  // Directional wipe for the crossfade (not a hard cut)
  var sweepPos: f32;
  if (dir == 0u) { sweepPos = uv.x; }
  else if (dir == 1u) { sweepPos = 1.0 - uv.x; }
  else if (dir == 2u) { sweepPos = uv.y; }
  else { sweepPos = 1.0 - uv.y; }

  // Soft directional crossfade
  let t = smoothstep(p * 1.3 - 0.15, p * 1.3 + 0.15, sweepPos);

  var color = mix(rightColor, leftColor, t);

  // Slight brightness boost at transition edge
  let edgeDist = abs(sweepPos - p);
  let edgeGlow = exp(-edgeDist * edgeDist * 40.0) * 0.08 * envelope;
  color = vec4f(min(color.rgb + edgeGlow, vec3f(1.0)), color.a);

  return color;
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.spread??1.5,o=a?.intensity??1;return new Float32Array([e,t,r,n,i,o,0,0])}},tw={id:"radialBlur",name:"Radial Blur",category:"custom",hasDirection:!1,entryPoint:"radialBlurFragment",uniformSize:32,shader:`
struct RadialBlurParams {
  progress: f32,
  width: f32,
  height: f32,
  blurStrength: f32,
  spin: f32,
  samples: f32,
  _pad1: f32,
  _pad2: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: RadialBlurParams;

fn sampleWithRadialBlur(tex: texture_2d<f32>, uv: vec2f, strength: f32, spinAmount: f32) -> vec4f {
  let center = vec2f(0.5, 0.5);
  let dir = uv - center;
  let dist = length(dir);

  // Combine zoom blur + spin blur
  let numSamples = u32(params.samples);
  var color = vec4f(0.0);
  var totalWeight = 0.0;

  for (var i = 0u; i < numSamples; i++) {
    let t = f32(i) / f32(numSamples - 1u) - 0.5;

    // Zoom: offset along radial direction
    let zoomOffset = dir * t * strength;

    // Spin: rotate around center
    let angle = t * spinAmount;
    let cosA = cos(angle);
    let sinA = sin(angle);
    let rotatedDir = vec2f(
      dir.x * cosA - dir.y * sinA,
      dir.x * sinA + dir.y * cosA
    ) - dir;
    let spinOffset = rotatedDir * strength;

    let sampleUv = clamp(uv + zoomOffset + spinOffset, vec2f(0.0), vec2f(1.0));

    // Gaussian-ish weight (center samples contribute more)
    let weight = exp(-t * t * 4.0);
    color += textureSample(tex, texSampler, sampleUv) * weight;
    totalWeight += weight;
  }

  return color / totalWeight;
}

@fragment
fn radialBlurFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = params.progress;

  // Blur envelope: ramp up → peak at midpoint → ramp down
  let blurEnvelope = sin(p * PI);
  let strength = blurEnvelope * params.blurStrength * 0.15;
  let spinAmount = blurEnvelope * params.spin * 0.3;

  // Sample both clips with radial blur
  let left = sampleWithRadialBlur(leftTex, uv, strength, spinAmount);
  let right = sampleWithRadialBlur(rightTex, uv, strength, spinAmount);

  // Crossfade with smooth S-curve
  let t = smoothstep(0.3, 0.7, p);

  var color = mix(left, right, t);

  // Subtle vignette darkening during blur peak
  let center = uv - vec2f(0.5);
  let vignette = 1.0 - dot(center, center) * blurEnvelope * 0.5;
  color = vec4f(color.rgb * vignette, color.a);

  return color;
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.blurStrength??1,o=a?.spin??.3,s=a?.samples??12;return new Float32Array([e,t,r,i,o,s,0,0])}},rw={id:"fade",name:"Fade",category:"basic",hasDirection:!1,entryPoint:"fadeFragment",uniformSize:16,shader:`
struct FadeParams {
  progress: f32,
  width: f32,
  height: f32,
  _pad: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: FadeParams;

@fragment
fn fadeFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);

  // cos²/sin² weights summing to 1, plus a 4% scale drift — mirrors the
  // Canvas2D fade renderer so GPU preview matches CPU export exactly.
  let c = cos(p * PI / 2.0);
  let outgoingWeight = c * c;
  let incomingWeight = 1.0 - c * c;
  let outgoingScale = 1.0 - 0.04 * p;
  let incomingScale = 1.04 - 0.04 * p;

  let outgoingUv = (input.uv - vec2f(0.5)) / outgoingScale + vec2f(0.5);
  let incomingUv = (input.uv - vec2f(0.5)) / incomingScale + vec2f(0.5);
  let left = textureSample(leftTex, texSampler, clamp(outgoingUv, vec2f(0.0), vec2f(1.0)));
  let right = textureSample(rightTex, texSampler, clamp(incomingUv, vec2f(0.0), vec2f(1.0)));
  let outgoingMask = f32(all(outgoingUv >= vec2f(0.0)) && all(outgoingUv <= vec2f(1.0)));
  let incomingMask = f32(all(incomingUv >= vec2f(0.0)) && all(incomingUv <= vec2f(1.0)));

  let wOut = outgoingWeight * outgoingMask;
  let wIn = incomingWeight * incomingMask;
  let color = left.rgb * wOut + right.rgb * wIn;
  let alpha = clamp(left.a * wOut + right.a * wIn, 0.0, 1.0);

  return vec4f(color, alpha);
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},nw={id:"wipe",name:"Wipe",category:"wipe",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"wipeFragment",uniformSize:16,shader:`
struct WipeParams {
  progress: f32,
  width: f32,
  height: f32,
  direction: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: WipeParams;

@fragment
fn wipeFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let dir = u32(params.direction);

  // Sample both textures upfront (uniform control flow required)
  let left = textureSample(leftTex, texSampler, uv);
  let right = textureSample(rightTex, texSampler, uv);

  // Sweep position along the wipe axis (0→1 in sweep direction)
  var sweepPos: f32;
  if (dir == 0u) { sweepPos = uv.x; }             // from-left
  else if (dir == 1u) { sweepPos = 1.0 - uv.x; }  // from-right
  else if (dir == 2u) { sweepPos = uv.y; }         // from-top
  else { sweepPos = 1.0 - uv.y; }                  // from-bottom

  // Hard edge: swept region shows incoming, rest shows outgoing
  let t = step(sweepPos, params.progress);
  return mix(left, right, t);
}`,packUniforms:(e,t,r,n)=>new Float32Array([e,t,r,n])},aw={id:"slide",name:"Slide",category:"slide",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"slideFragment",uniformSize:16,shader:`
struct SlideParams {
  progress: f32,
  width: f32,
  height: f32,
  direction: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: SlideParams;

@fragment
fn slideFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = params.progress;
  let dir = u32(params.direction);

  // Push-slide: both clips move together.
  // Compute offset UVs for both clips, then sample upfront.

  // Offset vectors per direction:
  // from-left:   outgoing shifts right by p, incoming shifts from left (offset = p-1)
  // from-right:  outgoing shifts left by p, incoming shifts from right
  // from-top:    outgoing shifts down by p, incoming shifts from top
  // from-bottom: outgoing shifts up by p, incoming shifts from bottom
  var leftUv: vec2f;
  var rightUv: vec2f;
  var splitTest: f32;

  if (dir == 0u) {
    leftUv = vec2f(uv.x - p, uv.y);
    rightUv = vec2f(uv.x - p + 1.0, uv.y);
    splitTest = step(uv.x, p);
  } else if (dir == 1u) {
    leftUv = vec2f(uv.x + p, uv.y);
    rightUv = vec2f(uv.x - (1.0 - p), uv.y);
    splitTest = step(1.0 - p, uv.x);
  } else if (dir == 2u) {
    leftUv = vec2f(uv.x, uv.y - p);
    rightUv = vec2f(uv.x, uv.y - p + 1.0);
    splitTest = step(uv.y, p);
  } else {
    leftUv = vec2f(uv.x, uv.y + p);
    rightUv = vec2f(uv.x, uv.y - (1.0 - p));
    splitTest = step(1.0 - p, uv.y);
  }

  // Sample both textures upfront (uniform control flow required)
  let left = textureSample(leftTex, texSampler, leftUv);
  let right = textureSample(rightTex, texSampler, rightUv);

  return mix(left, right, splitTest);
}`,packUniforms:(e,t,r,n)=>new Float32Array([e,t,r,n])},iw={id:"flip",name:"Flip",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"flipFragment",uniformSize:16,shader:`
struct FlipParams {
  progress: f32,
  width: f32,
  height: f32,
  direction: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: FlipParams;

@fragment
fn flipFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = params.progress;
  let dir = u32(params.direction);

  // Horizontal flip (from-left/from-right) scales X, vertical scales Y
  let isHorizontal = (dir == 0u || dir == 1u);
  let midpoint = 0.5;
  let centered = uv - vec2f(0.5, 0.5);

  // Phase 1: outgoing scales 1→0; Phase 2: incoming scales 0→1
  let flipProgress1 = p / midpoint;
  let flipProgress2 = (p - midpoint) / midpoint;
  let scale1 = max(cos(flipProgress1 * PI * 0.5), 0.001);
  let scale2 = max(sin(flipProgress2 * PI * 0.5), 0.001);
  let scale = select(scale2, scale1, p < midpoint);

  // Distort UV from center based on axis
  let hDistorted = vec2f(centered.x / scale + 0.5, uv.y);
  let vDistorted = vec2f(uv.x, centered.y / scale + 0.5);
  let distorted = select(vDistorted, hDistorted, isHorizontal);

  // Sample both textures at the distorted UV (uniform control flow)
  let left = textureSample(leftTex, texSampler, distorted);
  let right = textureSample(rightTex, texSampler, distorted);

  // Out of bounds = black
  let oob = distorted.x < 0.0 || distorted.x > 1.0 || distorted.y < 0.0 || distorted.y > 1.0;
  let black = vec4f(0.0, 0.0, 0.0, 1.0);

  // Phase 1 shows outgoing (left), Phase 2 shows incoming (right)
  let texColor = select(right, left, p < midpoint);
  return select(texColor, black, oob);
}`,packUniforms:(e,t,r,n)=>new Float32Array([e,t,r,n])},ow=`
struct VertexOutput {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

@vertex
fn vertexMain(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
  var positions = array<vec2f, 6>(
    vec2f(-1.0, -1.0),
    vec2f(1.0, -1.0),
    vec2f(-1.0, 1.0),
    vec2f(-1.0, 1.0),
    vec2f(1.0, -1.0),
    vec2f(1.0, 1.0)
  );
  var uvs = array<vec2f, 6>(
    vec2f(0.0, 1.0),
    vec2f(1.0, 1.0),
    vec2f(0.0, 0.0),
    vec2f(0.0, 0.0),
    vec2f(1.0, 1.0),
    vec2f(1.0, 0.0)
  );
  var output: VertexOutput;
  output.position = vec4f(positions[vertexIndex], 0.0, 1.0);
  output.uv = uvs[vertexIndex];
  return output;
}
`,yE=`
${ow}

const PI: f32 = 3.14159265359;
const TAU: f32 = 6.28318530718;

fn hash(p: vec2f) -> f32 {
  let p2 = vec2f(dot(p, vec2f(127.1, 311.7)), dot(p, vec2f(269.5, 183.3)));
  return fract(sin(dot(p2, vec2f(12.9898, 78.233))) * 43758.5453);
}

fn noise2d(p: vec2f) -> f32 {
  let i = floor(p);
  let f = fract(p);
  let u = f * f * (3.0 - 2.0 * f);
  return mix(
    mix(hash(i + vec2f(0.0, 0.0)), hash(i + vec2f(1.0, 0.0)), u.x),
    mix(hash(i + vec2f(0.0, 1.0)), hash(i + vec2f(1.0, 1.0)), u.x),
    u.y
  );
}

fn fbm(p: vec2f) -> f32 {
  var value = 0.0;
  var amplitude = 0.5;
  var pos = p;
  for (var i = 0; i < 5; i++) {
    value += amplitude * noise2d(pos);
    pos *= 2.0;
    amplitude *= 0.5;
  }
  return value;
}
`,tf=`
fn scaleUv(uv: vec2f, scale: f32) -> vec2f {
  let safeScale = max(scale, 0.001);
  return ((uv - vec2f(0.5, 0.5)) / safeScale) + vec2f(0.5, 0.5);
}
`,sw={id:"clockWipe",name:"Clock Wipe",category:"mask",hasDirection:!1,entryPoint:"clockWipeFragment",uniformSize:16,shader:`
struct ClockWipeParams {
  progress: f32,
  width: f32,
  height: f32,
  edgeSoftness: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: ClockWipeParams;

${tf}

fn clockSweepMask(angle: f32, sweepAngle: f32, feather: f32) -> f32 {
  if (sweepAngle <= 0.0) {
    return 0.0;
  }
  if (sweepAngle >= TAU) {
    return 1.0;
  }
  if (feather <= 0.0001) {
    return select(0.0, 1.0, angle <= sweepAngle);
  }
  return 1.0 - smoothstep(sweepAngle - feather, sweepAngle + feather, angle);
}

@fragment
fn clockWipeFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = clamp(params.progress, 0.0, 1.0);

  // Compute angle from center in pixel space (preserves aspect ratio)
  let pixelPos = uv * vec2f(params.width, params.height);
  let center = vec2f(params.width * 0.5, params.height * 0.5);
  let delta = pixelPos - center;

  // atan2(x, -y) gives angle from 12 o'clock (top), clockwise positive
  let angle = atan2(delta.x, -delta.y);

  // Normalize angle from [-PI, PI] to [0, TAU]
  let normalizedAngle = select(angle, angle + TAU, angle < 0.0);
  let sweepAngle = p * TAU;
  let feather = max(0.0, min(params.edgeSoftness * TAU / 360.0, min(sweepAngle, TAU - sweepAngle)));
  let outgoingScale = 1.0 - (0.04 * p);
  let incomingScale = 1.04 - (0.04 * p);
  let outgoingOpacity = 1.0 - (0.1 * p);
  let incomingOpacity = 0.85 + (0.15 * p);
  let leftUv = scaleUv(uv, outgoingScale);
  let rightUv = scaleUv(uv, incomingScale);

  // Sample both textures upfront (uniform control flow required)
  let left = textureSample(leftTex, texSampler, leftUv);
  let right = textureSample(rightTex, texSampler, rightUv);
  let outgoingColor = vec4f(left.rgb * outgoingOpacity, left.a * outgoingOpacity);
  let incomingColor = vec4f(right.rgb * incomingOpacity, right.a * incomingOpacity);

  // Swept region with soft edge: incoming; un-swept: outgoing
  let swept = clockSweepMask(normalizedAngle, sweepAngle, feather);
  return mix(outgoingColor, incomingColor, swept);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.edgeSoftness??8;return new Float32Array([e,t,r,i])}},lw={id:"iris",name:"Iris",category:"iris",hasDirection:!1,entryPoint:"irisFragment",uniformSize:16,shader:`
struct IrisParams {
  progress: f32,
  width: f32,
  height: f32,
  edgeSoftness: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: IrisParams;

${tf}

fn circleMask(distanceFromCenter: f32, radius: f32, feather: f32) -> f32 {
  if (radius <= 0.0) {
    return 0.0;
  }
  if (feather <= 0.001) {
    return select(0.0, 1.0, distanceFromCenter <= radius);
  }
  return 1.0 - smoothstep(radius - feather, radius + feather, distanceFromCenter);
}

@fragment
fn irisFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = clamp(params.progress, 0.0, 1.0);

  // Compute distance from center in pixel space
  let pixelPos = uv * vec2f(params.width, params.height);
  let halfW = params.width * 0.5;
  let halfH = params.height * 0.5;
  let center = vec2f(halfW, halfH);
  let dist = length(pixelPos - center);

  // Max radius = diagonal from center to corner * 1.2 (matches CPU)
  let maxRadius = sqrt(halfW * halfW + halfH * halfH) * 1.2;
  let radius = p * maxRadius;
  let feather = max(0.0, min(params.edgeSoftness, min(radius, maxRadius - radius)));
  let outgoingScale = 1.0 - (0.04 * p);
  let incomingScale = 1.04 - (0.04 * p);
  let outgoingOpacity = 1.0 - (0.1 * p);
  let incomingOpacity = 0.85 + (0.15 * p);
  let leftUv = scaleUv(uv, outgoingScale);
  let rightUv = scaleUv(uv, incomingScale);

  // Sample both textures upfront (uniform control flow required)
  let left = textureSample(leftTex, texSampler, leftUv);
  let right = textureSample(rightTex, texSampler, rightUv);
  let outgoingColor = vec4f(left.rgb * outgoingOpacity, left.a * outgoingOpacity);
  let incomingColor = vec4f(right.rgb * incomingOpacity, right.a * incomingOpacity);

  // Inside circle with soft edge: incoming; outside: outgoing
  let inside = circleMask(dist, radius, feather);
  return mix(outgoingColor, incomingColor, inside);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.edgeSoftness??6;return new Float32Array([e,t,r,i])}},cw={id:"liquidDistort",name:"Liquid Distort",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"liquidDistortFragment",uniformSize:48,shader:`
struct LiquidDistortParams {
  progress: f32,
  width: f32,
  height: f32,
  direction: f32,
  intensity: f32,
  scale: f32,
  turbulence: f32,
  edgeSoftness: f32,
  chroma: f32,
  swirl: f32,
  shine: f32,
  _pad: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: LiquidDistortParams;

fn liquidAxis(uv: vec2f, dir: u32) -> f32 {
  if (dir == 0u) { return uv.x; }
  if (dir == 1u) { return 1.0 - uv.x; }
  if (dir == 2u) { return uv.y; }
  return 1.0 - uv.y;
}

fn liquidFlow(uv: vec2f, p: f32, scale: f32, turbulence: f32) -> vec2f {
  let aspect = max(params.width / max(params.height, 1.0), 0.001);
  let pos = vec2f(uv.x * aspect, uv.y) * scale;
  let slow = fbm(pos + vec2f(p * 1.8, -p * 1.15));
  let fast = fbm(pos * 1.9 + vec2f(-p * 2.7, p * 1.55));
  let curlX = noise2d(pos + vec2f(slow * 2.5, p * 3.0)) - 0.5;
  let curlY = noise2d(pos + vec2f(p * -2.0, fast * 2.5)) - 0.5;
  return vec2f(curlX, curlY) * turbulence + vec2f(slow - 0.5, fast - 0.5) * 0.45;
}

@fragment
fn liquidDistortFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = clamp(params.progress, 0.0, 1.0);
  let dir = u32(params.direction);
  let envelope = sin(p * PI);
  let axis = liquidAxis(uv, dir);
  let flow = liquidFlow(uv, p, max(params.scale, 0.001), params.turbulence);

  let center = uv - vec2f(0.5);
  let swirlAngle = params.swirl * envelope * 0.42;
  let s = sin(swirlAngle);
  let c = cos(swirlAngle);
  let rotated = vec2f(center.x * c - center.y * s, center.x * s + center.y * c);
  let swirlOffset = (rotated - center) * 0.32;

  let baseStrength = params.intensity * envelope * 0.052;
  let leftOffset = (flow + swirlOffset) * baseStrength;
  let rightOffset = (-flow * 0.82 + swirlOffset * 0.55) * baseStrength;

  let frontNoise = fbm(uv * max(params.scale * 0.72, 0.001) + vec2f(p * 2.2, -p * 1.7));
  let front = axis + (frontNoise - 0.5) * 0.28 * params.intensity * envelope;
  let softness = max(params.edgeSoftness, 0.001);
  let reveal = smoothstep(p - softness, p + softness, front);

  let leftUv = clamp(uv + leftOffset, vec2f(0.0), vec2f(1.0));
  let rightUv = clamp(uv + rightOffset, vec2f(0.0), vec2f(1.0));
  let leftColor = textureSampleLevel(leftTex, texSampler, leftUv, 0.0);
  let rightColor = textureSampleLevel(rightTex, texSampler, rightUv, 0.0);

  let chromaOffset = flow * params.chroma * envelope * 0.018;
  let rightR = textureSampleLevel(rightTex, texSampler, clamp(rightUv + chromaOffset, vec2f(0.0), vec2f(1.0)), 0.0).r;
  let rightB = textureSampleLevel(rightTex, texSampler, clamp(rightUv - chromaOffset, vec2f(0.0), vec2f(1.0)), 0.0).b;
  let refractedRight = vec4f(rightR, rightColor.g, rightB, rightColor.a);

  let caustic = pow(max(0.0, 1.0 - abs(front - p) / max(softness * 2.5, 0.001)), 2.0);
  let shimmerNoise = fbm(uv * max(params.scale * 2.4, 0.001) + vec2f(p * 5.0, p * -4.0));
  let shine = vec3f(0.72, 0.88, 1.0) * caustic * shimmerNoise * params.shine * envelope * 0.22;

  let color = mix(refractedRight, leftColor, reveal);
  let glassMix = smoothstep(0.0, 1.0, caustic * 0.65 + envelope * 0.2);
  let glassed = mix(color.rgb, color.rgb + shine, glassMix);
  return vec4f(min(glassed, vec3f(1.0)), color.a);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.intensity??1,o=a?.scale??4.5,s=a?.turbulence??1,l=a?.edgeSoftness??.18,c=a?.chroma??.75,u=a?.swirl??.8,d=a?.shine??1;return new Float32Array([e,t,r,n,i,o,s,l,c,u,d,0])}},uw={id:"lensWarpZoom",name:"Lens Warp Zoom",category:"custom",hasDirection:!1,entryPoint:"lensWarpZoomFragment",uniformSize:48,shader:`
struct LensWarpZoomParams {
  progress: f32,
  width: f32,
  height: f32,
  zoomStrength: f32,
  warpStrength: f32,
  blurStrength: f32,
  chroma: f32,
  vignette: f32,
  centerX: f32,
  centerY: f32,
  glow: f32,
  _pad: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: LensWarpZoomParams;

fn barrelWarp(uv: vec2f, center: vec2f, amount: f32) -> vec2f {
  let aspect = max(params.width / max(params.height, 1.0), 0.001);
  var p = uv - center;
  p.x *= aspect;
  let r2 = dot(p, p);
  let warped = p * (1.0 + amount * r2);
  return vec2f(warped.x / aspect, warped.y) + center;
}

fn zoomAround(uv: vec2f, center: vec2f, zoom: f32) -> vec2f {
  return center + (uv - center) / max(zoom, 0.001);
}

fn sampleZoomBlur(tex: texture_2d<f32>, uv: vec2f, center: vec2f, strength: f32) -> vec4f {
  let dir = center - uv;
  var color = vec4f(0.0);
  var weightSum = 0.0;
  for (var i = 0u; i < 7u; i++) {
    let t = f32(i) / 6.0;
    let weight = 1.0 - abs(t - 0.5) * 0.8;
    let sampleUv = clamp(uv + dir * strength * (t - 0.5), vec2f(0.0), vec2f(1.0));
    color += textureSampleLevel(tex, texSampler, sampleUv, 0.0) * weight;
    weightSum += weight;
  }
  return color / weightSum;
}

@fragment
fn lensWarpZoomFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = clamp(params.progress, 0.0, 1.0);
  let center = vec2f(params.centerX, params.centerY);
  let envelope = sin(p * PI);
  let punch = smoothstep(0.0, 0.46, p) * (1.0 - smoothstep(0.58, 1.0, p));
  let reveal = smoothstep(0.36, 0.64, p);

  let outgoingZoom = 1.0 + params.zoomStrength * p * 0.42 + punch * 0.18;
  let incomingZoom = 1.0 + params.zoomStrength * (1.0 - p) * 0.58;
  let warp = params.warpStrength * envelope;
  let blur = params.blurStrength * envelope * 0.11;

  let leftWarped = barrelWarp(zoomAround(uv, center, outgoingZoom), center, warp);
  let rightWarped = barrelWarp(zoomAround(uv, center, incomingZoom), center, -warp * 0.65);

  let leftColor = sampleZoomBlur(leftTex, leftWarped, center, blur);
  let rightBase = sampleZoomBlur(rightTex, rightWarped, center, blur * 0.82);

  let chromaDir = normalize((uv - center) + vec2f(0.0001));
  let chromaOffset = chromaDir * params.chroma * envelope * 0.012;
  let rightR = textureSampleLevel(rightTex, texSampler, clamp(rightWarped + chromaOffset, vec2f(0.0), vec2f(1.0)), 0.0).r;
  let rightB = textureSampleLevel(rightTex, texSampler, clamp(rightWarped - chromaOffset, vec2f(0.0), vec2f(1.0)), 0.0).b;
  let rightColor = vec4f(rightR, rightBase.g, rightB, rightBase.a);

  let dist = distance(uv, center);
  let ring = exp(-pow((dist - 0.22 - p * 0.18) * 8.0, 2.0)) * params.glow * envelope;
  let vignette = 1.0 - smoothstep(0.36, 0.9, dist) * params.vignette * envelope * 0.45;
  let edgeLight = vec3f(0.78, 0.9, 1.0) * ring * 0.18;

  let color = mix(leftColor, rightColor, reveal);
  return vec4f(min(color.rgb * vignette + edgeLight, vec3f(1.0)), color.a);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.zoomStrength??1,o=a?.warpStrength??.75,s=a?.blurStrength??1,l=a?.chroma??.65,c=a?.vignette??.7,u=a?.centerX??.5,d=a?.centerY??.5,m=a?.glow??1;return new Float32Array([e,t,r,i,o,s,l,c,u,d,m,0])}},dw={id:"lightLeakBurn",name:"Light Leak Burn",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"lightLeakBurnFragment",uniformSize:48,shader:`
struct LightLeakBurnParams {
  progress: f32,
  width: f32,
  height: f32,
  direction: f32,
  intensity: f32,
  spread: f32,
  warmth: f32,
  burn: f32,
  edgeSoftness: f32,
  grain: f32,
  _pad1: f32,
  _pad2: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: LightLeakBurnParams;

fn burnAxis(uv: vec2f, dir: u32) -> f32 {
  if (dir == 0u) { return uv.x; }
  if (dir == 1u) { return 1.0 - uv.x; }
  if (dir == 2u) { return uv.y; }
  return 1.0 - uv.y;
}

@fragment
fn lightLeakBurnFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = clamp(params.progress, 0.0, 1.0);
  let dir = u32(params.direction);
  let axis = burnAxis(uv, dir);
  let envelope = sin(p * PI);

  let left = textureSampleLevel(leftTex, texSampler, uv, 0.0);
  let right = textureSampleLevel(rightTex, texSampler, uv, 0.0);

  let organic = fbm(uv * vec2f(4.0, 3.0) + vec2f(p * 2.7, -p * 1.9));
  let fine = noise2d(uv * vec2f(params.width, params.height) * 0.45 + vec2f(p * 431.0));
  let noisyAxis = axis + (organic - 0.5) * params.spread * 0.28;
  let reveal = smoothstep(p - params.edgeSoftness, p + params.edgeSoftness, noisyAxis);
  // Reveal is 1 ahead of the moving front and 0 behind it. Keep the outgoing
  // frame ahead of the front so p=0 starts on left, then reveal the incoming
  // frame behind it until p=1 ends on right.
  let base = mix(right, left, reveal);

  let frontDist = abs(noisyAxis - p);
  let hotCore = exp(-frontDist * frontDist / max(0.0001, params.edgeSoftness * params.edgeSoftness * 0.38));
  let warmHalo = exp(-frontDist * frontDist / max(0.0001, params.edgeSoftness * params.edgeSoftness * 3.5));
  let warm = mix(vec3f(1.0, 0.88, 0.58), vec3f(1.0, 0.48, 0.16), params.warmth);
  let whiteHot = vec3f(1.0, 0.96, 0.86);
  let grain = (fine - 0.5) * params.grain * envelope * 0.08;
  let burnLight = warm * warmHalo * params.intensity * envelope * 1.15
    + whiteHot * hotCore * params.burn * envelope * 1.35;

  let overexposed = 1.0 - exp(-(base.rgb + burnLight + grain) * (1.0 + hotCore * params.burn));
  let color = mix(base.rgb, overexposed, clamp((warmHalo + hotCore) * envelope, 0.0, 1.0));
  return vec4f(clamp(color, vec3f(0.0), vec3f(1.0)), base.a);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.intensity??1.25,o=a?.spread??1,s=a?.warmth??.75,l=a?.burn??1.1,c=a?.edgeSoftness??.16,u=a?.grain??.5;return new Float32Array([e,t,r,n,i,o,s,l,c,u,0,0])}},mw={id:"filmGateSlip",name:"Film Gate Slip",category:"custom",hasDirection:!1,entryPoint:"filmGateSlipFragment",uniformSize:48,shader:`
struct FilmGateSlipParams {
  progress: f32,
  width: f32,
  height: f32,
  slip: f32,
  shake: f32,
  exposure: f32,
  gateWidth: f32,
  grain: f32,
  chroma: f32,
  roll: f32,
  _pad1: f32,
  _pad2: f32,
};

@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
@group(0) @binding(3) var<uniform> params: FilmGateSlipParams;

fn sampleFilm(tex: texture_2d<f32>, uv: vec2f, chromaOffset: vec2f) -> vec4f {
  let baseUv = clamp(uv, vec2f(0.0), vec2f(1.0));
  let r = textureSampleLevel(tex, texSampler, clamp(baseUv + chromaOffset, vec2f(0.0), vec2f(1.0)), 0.0).r;
  let g = textureSampleLevel(tex, texSampler, baseUv, 0.0).g;
  let b = textureSampleLevel(tex, texSampler, clamp(baseUv - chromaOffset, vec2f(0.0), vec2f(1.0)), 0.0).b;
  let a = textureSampleLevel(tex, texSampler, baseUv, 0.0).a;
  return vec4f(r, g, b, a);
}

@fragment
fn filmGateSlipFragment(input: VertexOutput) -> @location(0) vec4f {
  let uv = input.uv;
  let p = clamp(params.progress, 0.0, 1.0);
  let envelope = sin(p * PI);
  let frame = floor(p * 18.0);
  let jitterA = hash(vec2f(frame, 19.7));
  let jitterB = hash(vec2f(frame + 3.0, 41.3));
  let gatePulse = smoothstep(0.08, 0.22, p) * (1.0 - smoothstep(0.78, 0.96, p));

  let slipOffset = (p - 0.5) * params.slip * envelope * 0.32
    + (jitterA - 0.5) * params.shake * envelope * 0.05;
  let lateral = (jitterB - 0.5) * params.shake * envelope * 0.025;
  let roll = params.roll * envelope * 0.04 * sin(p * TAU * 2.0);
  let outgoingUv = vec2f(uv.x + lateral + roll * (uv.y - 0.5), uv.y + slipOffset);
  let incomingUv = vec2f(uv.x - lateral * 0.65 - roll * (uv.y - 0.5), uv.y - slipOffset * 0.55);

  let chromaOffset = vec2f(params.chroma * envelope * 0.006, 0.0);
  let left = sampleFilm(leftTex, outgoingUv, chromaOffset);
  let right = sampleFilm(rightTex, incomingUv, chromaOffset * 0.7);
  var color = mix(left, right, smoothstep(0.42, 0.58, p));

  let gateTop = smoothstep(params.gateWidth, 0.0, uv.y);
  let gateBottom = smoothstep(1.0 - params.gateWidth, 1.0, uv.y);
  let gateFlash = max(gateTop, gateBottom) * gatePulse * 0.2;
  let flicker = 1.0 + (hash(vec2f(frame, 8.1)) - 0.42) * params.exposure * envelope * 0.38;
  let grain = (hash(floor(uv * vec2f(params.width, params.height) * 0.7) + vec2f(frame)) - 0.5)
    * params.grain * envelope * 0.12;
  let vignette = 1.0 - dot(uv - vec2f(0.5), uv - vec2f(0.5)) * envelope * 0.35;

  color = vec4f(color.rgb * flicker * vignette + grain + vec3f(gateFlash), color.a);
  return vec4f(clamp(color.rgb, vec3f(0.0), vec3f(1.0)), color.a);
}`,packUniforms:(e,t,r,n,a)=>{const i=a?.slip??1,o=a?.shake??1,s=a?.exposure??.85,l=a?.gateWidth??.075,c=a?.grain??.6,u=a?.chroma??.55,d=a?.roll??.75;return new Float32Array([e,t,r,i,o,s,l,c,u,d,0,0])}},rf=new Map;function ve(e){rf.set(e.id,e)}ve(V1);ve(W1);ve(j1);ve($1);ve(Y1);ve(X1);ve(Q1);ve(J1);ve(Z1);ve(ew);ve(tw);ve(rw);ve(nw);ve(aw);ve(iw);ve(sw);ve(lw);ve(cw);ve(uw);ve(dw);ve(mw);function vE(e){return rf.get(e)}const pw={normal:0,dissolve:1,darken:2,multiply:3,"color-burn":4,"linear-burn":5,lighten:6,screen:7,"color-dodge":8,"linear-dodge":9,overlay:10,"soft-light":11,"hard-light":12,"vivid-light":13,"linear-light":14,"pin-light":15,"hard-mix":16,difference:17,exclusion:18,subtract:19,divide:20,hue:21,saturation:22,color:23,luminosity:24},he={fixedImages:{min:1,max:60},perMinutes:{min:1,max:60},motionIntensity:{min:.25,max:2},motionSpeed:{min:.25,max:3},imagePrompt:{max:2e3},sceneOpacity:{min:0,max:1},backgroundAppearance:{opacity:{min:0,max:1},blurRadius:{min:0,max:50},brightness:{min:-1,max:1},contrast:{min:0,max:3},saturation:{min:0,max:3},hueShift:{min:0,max:1},speed:{min:.25,max:3},zoom:{min:1,max:3}},backdropChannels:{min:1,max:20},kindsPerChannel:{min:1,max:3},pinnedPerChannel:{max:200},backdropClips:{min:1,max:40},backdropClipSec:{min:10,max:600}},fw=["clean","hook","circle-arrow","hook-arrow","split","closeup"],nf=y({style:z(["full","cutout","cutout-close"]),images:Oi("mode",[y({mode:Z("fixed"),count:M().int().min(he.fixedImages.min).max(he.fixedImages.max)}).strict(),y({mode:Z("per-minutes"),minutes:M().int().min(he.perMinutes.min).max(he.perMinutes.max)}).strict(),y({mode:Z("none")}).strict()]),motion:y({kind:z(["ken-burns","wander"]),intensity:M().min(he.motionIntensity.min).max(he.motionIntensity.max),speed:M().min(he.motionSpeed.min).max(he.motionSpeed.max)}).strict(),thumbnail:z(fw).optional(),imagePrompt:w().min(1).max(he.imagePrompt.max).refine(e=>e===e.trim(),"imagePrompt must be trimmed").optional(),opacity:M().finite().min(he.sceneOpacity.min).max(he.sceneOpacity.max).optional(),coverMediaId:w().min(1).optional(),compress:Z(!1).optional()}).strict(),af=y({channels:F(y({channelId:w().regex(/^(twitch:[a-z0-9_]{1,25}|bilibili:\d{1,20})$/),kinds:F(z(["clip","highlight","vod","video"])).min(he.kindsPerChannel.min).max(he.kindsPerChannel.max),pinned:F(w().min(1).max(64)).max(he.pinnedPerChannel.max)}).strict()).min(he.backdropChannels.min).max(he.backdropChannels.max),clipCount:M().int().min(he.backdropClips.min).max(he.backdropClips.max).optional(),clipSec:M().int().min(he.backdropClipSec.min).max(he.backdropClipSec.max)}).strict(),Te=he.backgroundAppearance,of=y({opacity:M().finite().min(Te.opacity.min).max(Te.opacity.max),blurRadius:M().finite().min(Te.blurRadius.min).max(Te.blurRadius.max),brightness:M().finite().min(Te.brightness.min).max(Te.brightness.max),contrast:M().finite().min(Te.contrast.min).max(Te.contrast.max),saturation:M().finite().min(Te.saturation.min).max(Te.saturation.max),hueShift:M().finite().min(Te.hueShift.min).max(Te.hueShift.max),speed:M().finite().min(Te.speed.min).max(Te.speed.max),zoom:M().finite().min(Te.zoom.min).max(Te.zoom.max),flipX:B(),flipY:B()}).strict(),ei=50,Xc=1440*60,gs=y({windowSeconds:M().finite().min(.1).max(10),bars:M().int().min(4).max(256),amplitude:M().finite().min(0).max(2),color:w().regex(/^#[a-fA-F0-9]{6}$/),strokeWidth:M().finite().min(.5).max(20),baseline:M().finite().min(0).max(1),visible:B()}).strict(),ar=y({sourceHash:w().regex(/^sha256:[a-f0-9]{64}$/),samplesPerSecond:Z(ei),durationSeconds:M().finite().positive().max(Xc),envelopeBase64:w().max(Math.ceil(Xc*ei/3)*4).regex(/^[A-Za-z0-9+/]*={0,2}$/).refine(e=>e.length%4===0,"Base64 cần đủ nhóm bốn ký tự"),normalization:M().finite().nonnegative()}).strict().superRefine((e,t)=>{const r=e.envelopeBase64.endsWith("==")?2:e.envelopeBase64.endsWith("=")?1:0;e.envelopeBase64.length/4*3-r!==Math.ceil(e.durationSeconds*ei-1e-8)&&t.addIssue({code:"custom",path:["envelopeBase64"],message:"Số mẫu sóng không khớp thời lượng nguồn"})}),hw=gs.extend({version:Z(1),sourceHash:ar.shape.sourceHash,samplesPerSecond:Z(ei),durationSeconds:ar.shape.durationSeconds.optional(),envelopeBase64:ar.shape.envelopeBase64.optional(),normalization:ar.shape.normalization.optional()}).superRefine((e,t)=>{if(e.envelopeBase64!==void 0||e.durationSeconds!==void 0||e.normalization!==void 0){const r=ar.safeParse({sourceHash:e.sourceHash,samplesPerSecond:e.samplesPerSecond,durationSeconds:e.durationSeconds,envelopeBase64:e.envelopeBase64,normalization:e.normalization});if(!r.success)for(const n of r.error.issues)t.addIssue({code:"custom",path:n.path,message:n.message})}}),Qc={windowSeconds:1,bars:64,amplitude:1,color:"#ffffff",strokeWidth:3,baseline:.5,visible:!0};function sf(e,t){if(e.waveformVisualizer===void 0)return null;if(e.type!=="shape"||e.shapeType!=="rectangle"||typeof e.mediaId!="string"||!e.mediaId||e.isMask)return"Sóng giọng cần shape chữ nhật, nguồn giọng và không được làm mask";const r=hw.safeParse(e.waveformVisualizer);return r.success?t!==void 0&&r.data.sourceHash!==t&&r.data.envelopeBase64!==void 0?"Dữ liệu sóng giọng đã cũ; tạo lại từ nguồn hiện tại":null:`Dữ liệu sóng giọng không hợp lệ: ${r.error.issues[0]?.message}`}const gw=typeof performance<"u"&&typeof performance.mark=="function"&&typeof performance.measure=="function";let yw=0;function lf(e,t){if(!gw||!globalThis.__TL_PERF__)return t();const r=++yw,n=`${e}:s:${r}`,a=`${e}:e:${r}`;try{performance.mark(n)}catch{return t()}try{return t()}finally{try{performance.mark(a),performance.measure(e,n,a),performance.clearMarks(n),performance.clearMarks(a)}catch{}}}class vw{listeners=new Map;on(t,r){let n=this.listeners.get(t);return n||(n=new Set,this.listeners.set(t,n)),n.add(r),()=>{const a=this.listeners.get(t);a&&(a.delete(r),a.size===0&&this.listeners.delete(t))}}emit(t,r){const n=this.listeners.get(t);if(!(!n||n.size===0))for(const a of[...n])a(r)}clear(){this.listeners.clear()}}const bw=new vw;function Sw(e,t){bw.emit(e,t)}const cf=1;function Iw(e,t){return Math.abs(e-t)<=cf}function xw(e,t){return t<e-cf}function ww(e){return Math.max(0,Math.min(1,e??.5))}function Xr(e,t){const r=Math.max(1,Math.floor(e)),n=ww(t),a=Math.floor(r*n),i=r-a;return{leftPortion:a,rightPortion:i}}function uf(e,t,r){const n=Math.max(0,Math.floor(e)),a=Math.max(0,Math.floor(t)),i=Math.max(0,Math.floor(r)),o=a+i;if(o<=n)return{incomingPortion:a,outgoingPortion:i};if(n===0)return{incomingPortion:0,outgoingPortion:0};const s=n/o;let l=Math.floor(a*s),c=Math.floor(i*s),u=n-(l+c);for(;u>0;){const d=a-l,m=i-c;if(d===0&&m===0)break;d>=m&&d>0?l+=1:m>0?c+=1:d>0&&(l+=1),u-=1}return{incomingPortion:l,outgoingPortion:c}}function bE(e,t){const r=new Map,n=new Map,a=new Map;for(const o of e){const s=t.get(o.leftClipId),l=t.get(o.rightClipId);if(!s||!l)continue;const c=s.from+s.durationInFrames;if(Iw(c,l.from)){const d=l.from,m=Xr(o.durationInFrames,o.alignment);r.set(o.id,{transition:o,leftClip:s,rightClip:l,cutPoint:d,leftPortion:m.leftPortion,rightPortion:m.rightPortion,mode:"cut-centered"}),a.set(s.id,o.id),n.set(l.id,o.id);continue}if(!xw(c,l.from))continue;const u=c-l.from;r.set(o.id,{transition:o,leftClip:s,rightClip:l,cutPoint:c,leftPortion:u,rightPortion:u,mode:"legacy-overlap"}),a.set(s.id,o.id),n.set(l.id,o.id)}for(const[o,s]of n){const l=a.get(o);if(!l)continue;const c=r.get(s),u=r.get(l),d=t.get(o);if(!c||!u||!d)continue;const m=uf(d.durationInFrames,c.rightPortion,u.leftPortion);c.rightPortion=m.incomingPortion,u.leftPortion=m.outgoingPortion}const i=[];for(const o of r.values()){const s=o.leftClip,l=o.rightClip,c=s.from+s.durationInFrames,u=o.mode==="cut-centered"?o.cutPoint-Math.max(0,o.leftPortion):l.from,d=o.mode==="cut-centered"?o.cutPoint+Math.max(0,o.rightPortion):c,m=Math.max(1,d-u);i.push({transition:o.transition,leftClip:o.leftClip,rightClip:o.rightClip,cutPoint:o.cutPoint,startFrame:u,endFrame:d,durationInFrames:m,leftPortion:Math.max(0,o.leftPortion),rightPortion:Math.max(0,o.rightPortion)})}return i.toSorted((o,s)=>o.startFrame!==s.startFrame?o.startFrame-s.startFrame:o.cutPoint!==s.cutPoint?o.cutPoint-s.cutPoint:o.transition.id.localeCompare(s.transition.id))}const df=1;function Qr(e,t){return Math.abs(e-t)<=df}function Pi(e,t){return t<e-df}function Jc(e,t){if(e.originId!==t.originId||e.trackId!==t.trackId)return!1;const r=e.compositionId??e.mediaId,n=t.compositionId??t.mediaId;if(!r||r!==n||!Qr(e.from+e.durationInFrames,t.from)||(e.speed||1)!==(t.speed||1))return!1;const a=e.sourceEnd??(e.sourceStart??0)+e.durationInFrames*(e.speed||1),i=t.sourceStart??0;return!(Math.abs(a-i)>.5)}function mf(e,t,r,n){if(t===void 0)return 0;let a=n;return e+a<0&&(a=-e),r!==void 0&&t+a>r&&(a=r-t),a}function pf(e,t,r,n,a){if(n===0||!t||!r||!Jc(t,e)||!Jc(e,r)||!qt(e)||e.sourceEnd===void 0)return 0;const i=e.speed??1,o=e.sourceFps??a,s=e.sourceStart??0,l=e.sourceEnd,c=we(n,i,a,o);return mf(s,l,e.sourceDuration,c)}function ff(e,t,r,n=30){const a=Math.floor(Math.min(e.durationInFrames,t.durationInFrames)-1);if(a<1)return 0;const i=Mi(e,"end",n),o=Mi(t,"start",n),s=d=>{const m=Xr(d,r);return m.rightPortion<=i&&m.leftPortion<=o};let l=1,c=a,u=0;for(;l<=c;){const d=Math.floor((l+c)/2);s(d)?(u=d,l=d+1):c=d-1}return u}function ua(e,t,r,n,a=30){if(e.trackId!==t.trackId)return{canAdd:!1,reason:"Clips must be on the same track"};const i=e.from+e.durationInFrames,o=Qr(i,t.from),s=Pi(i,t.from);if(!o&&!s)return{canAdd:!1,reason:"Clips must meet at the cut"};const l=["video","image","composition"];if(!l.includes(e.type)||!l.includes(t.type))return{canAdd:!1,reason:"Transitions only work with video and image clips"};const c=Math.min(e.durationInFrames,t.durationInFrames)-1;if(r>c)return{canAdd:!1,reason:`Transition too long. Max: ${c} frames (shorter clip duration - 1)`};const u=Mi(e,"end",a),d=Mi(t,"start",a);if(o){const m=Xr(r,n),p=m.rightPortion,f=m.leftPortion;if(p>u||f>d)return{canAdd:!1,reason:`Insufficient handle for transition: ${[p>u?`left clip needs ${p} tail-handle frames but only has ${u}`:null,f>d?`right clip needs ${f} head-handle frames but only has ${d}`:null].filter(Boolean).join("; ")}`,leftHandle:u,rightHandle:d}}return{canAdd:!0,leftHandle:u,rightHandle:d}}function kw(e,t,r,n,a=30){if(t===0)return t;const i=n.filter(s=>s.leftClipId===e.id||s.rightClipId===e.id);return i.length===0?t:hf(t,s=>{const l=Pw(e,s);return i.every(c=>{const u=c.leftClipId===e.id?l:r.find(m=>m.id===c.leftClipId)??null,d=c.rightClipId===e.id?l:r.find(m=>m.id===c.rightClipId)??null;return!u||!d?!0:ua(u,d,c.durationInFrames,c.alignment,a).canAdd})})}function Zc(e,t,r,n,a,i,o=30){if(t===0)return t;const s=new Set([e.id]);r&&s.add(r.id),n&&s.add(n.id);const l=i.filter(d=>s.has(d.leftClipId)||s.has(d.rightClipId));if(l.length===0)return t;const c=new Map(a.map(d=>[d.id,d]));return hf(t,d=>{const m=new Map;r&&m.set(r.id,ho(r,ol(r,d,o))),n&&m.set(n.id,ho(n,il(n,d,o)));let p=ho(e,TI(e,d));const f=pf(e,r,n,d,o);return f!==0&&(p.type==="video"||p.type==="audio"||p.type==="composition")&&p.sourceEnd!==void 0&&(p={...p,sourceStart:(p.sourceStart??0)+f,sourceEnd:p.sourceEnd+f}),m.set(e.id,p),l.every(h=>{const g=m.get(h.leftClipId)??c.get(h.leftClipId)??null,v=m.get(h.rightClipId)??c.get(h.rightClipId)??null;return!g||!v?!0:ua(g,v,h.durationInFrames,h.alignment,o).canAdd})})}function hf(e,t){if(!t(0))return 0;if(t(e))return e;const r=e<0?-1:1;let n=0,a=Math.abs(e);for(;n<a;){const i=Math.ceil((n+a)/2),o=r*i;t(o)?n=i:a=i-1}return r*n}function Mi(e,t,r){if(e.type==="text"||e.type==="shape"||e.type==="adjustment"||e.type==="image")return 1/0;if(e.type==="audio")return 0;const{sourceStart:n,sourceEnd:a,sourceDuration:i,sourceFps:o,speed:s}=na(e),l=o??r,c=a??i??0,u=i??0;if(t==="start")return Tt(n,s,l,r);{const d=Nd(u,c);return Tt(d,s,l,r)}}function Pw(e,t){return{...e,sourceStart:(e.sourceStart??0)+t,sourceEnd:e.sourceEnd!==void 0?e.sourceEnd+t:e.sourceEnd}}function ho(e,t){return{...e,...t}}const Ci=new Set(["video","image","composition"]);function gf(e,t,r,n=new Set,a=30){const i=[],o=[],s=[],l=new Map;for(const d of t)l.set(d.id,d);const c=new Map;for(const d of t)Ci.has(d.type)&&(c.has(d.trackId)||c.set(d.trackId,[]),c.get(d.trackId).push(d));for(const d of c.values())d.sort((m,p)=>m.from-p.from);const u=new Set(e);for(const d of r){const m=u.has(d.leftClipId),p=u.has(d.rightClipId);if(!m&&!p){i.push(d);continue}const f=Mw(d,l,n,c,a);f.status==="valid"?i.push(d):f.status==="repaired"?o.push({original:d,repaired:f.repaired,action:f.action}):s.push(f.breakage)}return{valid:i,repaired:o,broken:s}}function Mw(e,t,r,n,a){const i=t.get(e.leftClipId),o=t.get(e.rightClipId);if(!i||!o){const d=Cw(e,i,o,t,r,n);if(d)return d;const m=[];return i||m.push(e.leftClipId),o||m.push(e.rightClipId),{status:"broken",breakage:Xt(e,"clip_deleted",`Transition removed: clip${m.length>1?"s":""} deleted`,m)}}if(i.trackId!==o.trackId)return{status:"broken",breakage:Xt(e,"cross_track","Transition removed: clips moved to different tracks",[e.leftClipId,e.rightClipId])};const s=i.from+i.durationInFrames,l=Qr(s,o.from),c=Pi(s,o.from);if(!l&&!c)return{status:"broken",breakage:Xt(e,"not_adjacent","Transition removed: clips no longer meet at the cut",[e.leftClipId,e.rightClipId])};if(!Ci.has(i.type)||!Ci.has(o.type))return{status:"broken",breakage:Xt(e,"invalid_type","Transition removed: clip type does not support transitions",[e.leftClipId,e.rightClipId])};if(l){const d=ff(i,o,e.alignment,a);return d<=0?{status:"broken",breakage:Xt(e,"insufficient_handle","Transition removed: clips do not have enough handle for any duration",[e.leftClipId,e.rightClipId])}:e.durationInFrames>d?{status:"repaired",repaired:{...e,durationInFrames:d},action:`Duration adjusted from ${e.durationInFrames} to ${d} frames (handle limited)`}:{status:"valid"}}const u=s-o.from;if(u<e.durationInFrames){const d=Math.max(2,Math.floor(u));return d>=2&&d<i.durationInFrames&&d<o.durationInFrames?{status:"repaired",repaired:{...e,durationInFrames:d},action:`Duration adjusted from ${e.durationInFrames} to ${d} frames (legacy overlap shrunk)`}:{status:"broken",breakage:Xt(e,"invalid_duration","Transition removed: legacy overlap too small for any transition duration",[e.leftClipId,e.rightClipId])}}if(e.durationInFrames>=i.durationInFrames||e.durationInFrames>=o.durationInFrames){const d=Math.min(i.durationInFrames,o.durationInFrames)-1,m=Math.max(2,d);return m>=2?{status:"repaired",repaired:{...e,durationInFrames:m},action:`Duration adjusted from ${e.durationInFrames} to ${m} frames (clip too short)`}:{status:"broken",breakage:Xt(e,"invalid_duration","Transition removed: clips too short for any transition duration",[e.leftClipId,e.rightClipId])}}return{status:"valid"}}function Cw(e,t,r,n,a,i){if(!t&&!r)return null;const o=t||r;if(!o)return null;const s=!t,l=i.get(o.trackId);if(!l)return null;const c=l.findIndex(m=>m.id===o.id);if(c===-1)return null;let u;if(s){if(u=l[c-1],u){const m=u.from+u.durationInFrames;!Qr(m,o.from)&&!Pi(m,o.from)&&(u=void 0)}}else if(u=l[c+1],u){const m=o.from+o.durationInFrames;!Qr(m,u.from)&&!Pi(m,u.from)&&(u=void 0)}return!u||!Ci.has(u.type)?null:{status:"repaired",repaired:{...e,leftClipId:s?u.id:e.leftClipId,rightClipId:s?e.rightClipId:u.id},action:`Clip reassigned: ${s?"left":"right"} clip replaced with adjacent clip`}}function Xt(e,t,r,n){return{transitionId:e.id,transition:e,reason:t,message:r,affectedClipIds:n}}function yf(e,t){const r=e.findLast(a=>a.frame<t),n=e.find(a=>a.frame>t);return r?n?r.source?.applicationId===n.source?.applicationId?n.source:void 0:r.source:n?.source}function Tw(e,t,r){const n=e.toSorted((l,c)=>l.frame-c.frame),a=n.filter(l=>l.frame>t);if(a.length===0)return{keyframes:e,removedCount:0,insertedBoundary:!1};const i=n.filter(l=>l.frame<=t);if(i.some(l=>l.frame===t))return{keyframes:i,removedCount:a.length,insertedBoundary:!1};const o=i.at(-1)??a[0],s={id:r(),frame:t,value:en(n,t,o.value),easing:o.easing,easingConfig:o.easingConfig,source:yf(n,t)};return{keyframes:[...i,s],removedCount:a.length,insertedBoundary:!0}}function Ew(e,t,r,n){const a=e.keyframes.toSorted((c,u)=>c.frame-u.frame),i=a.filter(c=>c.frame>t);if(i.length===0)return{property:e,removedCount:0,insertedBoundary:!1};const o=a.filter(c=>c.frame<=t);if(o.some(c=>c.frame===t))return{property:{...e,keyframes:o},removedCount:i.length,insertedBoundary:!1};const s=o.at(-1)??i[0],l={id:n(),frame:t,value:Qa(e.property,a,t,s.value,r),easing:s.easing,easingConfig:s.easingConfig,source:yf(a,t)};return{property:{...e,keyframes:[...o,l]},removedCount:i.length,insertedBoundary:!0}}function Fw(e,t,r,n=()=>$()){const a=Math.max(0,t-1);let i=0,o=0;const s=e.properties.map(c=>{const u=Tw(c.keyframes,a,n);return i+=u.removedCount,o+=u.insertedBoundary?1:0,u.removedCount>0?{...c,keyframes:u.keyframes}:c}),l=e.vectorProperties?.map(c=>{const u=Ew(c,a,r,n);return i+=u.removedCount,o+=u.insertedBoundary?1:0,u.property});return i===0?{itemKeyframes:e,removedCount:0,insertedBoundaryCount:0}:{itemKeyframes:{...e,properties:s,vectorProperties:l},removedCount:i,insertedBoundaryCount:o}}function Aw(e,t,r){const n=[];let a,i,o=0,s=0;for(const l of r){if(l.leftClipId===e){const c=Xr(l.durationInFrames,l.alignment);i=l,s=c.leftPortion}if(l.rightClipId===e){const c=Xr(l.durationInFrames,l.alignment);a=l,o=c.rightPortion}}if(a&&i){const l=uf(t.durationInFrames,o,s);o=l.incomingPortion,s=l.outgoingPortion}return a&&o>0&&n.push({start:0,end:Math.min(o,t.durationInFrames),transition:a,role:"incoming"}),i&&s>0&&n.push({start:Math.max(0,t.durationInFrames-s),end:t.durationInFrames,transition:i,role:"outgoing"}),n}function Yn(e,t,r,n){const a=Aw(t,r,n);for(const i of a)if(e>=i.start&&e<i.end)return i}const eu=0,go=16777215,Ft=4294967296,_w=Ft+4294967295,Ow=512,sn=new Map;function Xn(e,t,r){return Math.max(t,Math.min(r,e))}function Ea(e){return Xn(Math.round(e),0,255)}function St(e){if(!Number.isFinite(e))return eu;const t=Math.round(e);return t<=go?Xn(t,eu,go):t<Ft?go:Xn(t,Ft,_w)}function Dw({r:e,g:t,b:r,a:n,hasAlpha:a}){const i=Ea(e),o=Ea(t),s=Ea(r);if(!a)return i*65536+o*256+s;const l=Ea(n);return Ft+i*16777216+o*65536+s*256+l}function ys(e){const t=St(e);if(t>=Ft){const r=t-Ft;return{r:Math.floor(r/16777216)&255,g:Math.floor(r/65536)&255,b:Math.floor(r/256)&255,a:r&255,hasAlpha:!0}}return{r:Math.floor(t/65536)&255,g:Math.floor(t/256)&255,b:t&255,a:255,hasAlpha:!1}}function yo(e){const t=Xn(e/255,0,1);return t<=.04045?t/12.92:((t+.055)/1.055)**2.4}function vo(e){const t=e<=.0031308?e*12.92:1.055*e**.4166666666666667-.055;return Xn(t,0,1)*255}function Rw(e){const t=yo(e.r),r=yo(e.g),n=yo(e.b),a=Math.cbrt(.4122214708*t+.5363325363*r+.0514459929*n),i=Math.cbrt(.2119034982*t+.6806995451*r+.1073969566*n),o=Math.cbrt(.0883024619*t+.2817188376*r+.6299787005*n),s=.2104542553*a+.793617785*i-.0040720468*o,l=1.9779984951*a-2.428592205*i+.4505937099*o,c=.0259040371*a+.7827717662*i-.808675766*o;return{l:s,c:Math.hypot(l,c),h:(Math.atan2(c,l)*180/Math.PI+360)%360}}function tu(e){const t=St(e),r=sn.get(t);if(r)return{...r};const n=Rw(ys(t));if(sn.size>=Ow){const a=sn.keys().next().value;a!==void 0&&sn.delete(a)}return sn.set(t,n),{...n}}function Lw(e){const t=e.h*Math.PI/180,r=e.c*Math.cos(t),n=e.c*Math.sin(t),a=e.l+.3963377774*r+.2158037573*n,i=e.l-.1055613458*r-.0638541728*n,o=e.l-.0894841775*r-1.291485548*n,s=a**3,l=i**3,c=o**3;return{r:vo(4.0767416621*s-3.3077115913*l+.2309699292*c),g:vo(-1.2684380046*s+2.6097574011*l-.3413193965*c),b:vo(-.0041960863*s-.7034186147*l+1.707614701*c)}}function Bw(e){const t=e.trim(),r=/^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.exec(t);if(!r)return null;const n=r[1];return n.length===3||n.length===4?`#${n.split("").map(a=>a+a).join("").toLowerCase()}`:`#${n.toLowerCase()}`}function dl(e){const t=Bw(e);if(!t)return null;const r=t.slice(1);return r.length===6?Number.parseInt(r,16):Ft+Number.parseInt(r,16)}function vf(e){const t=St(e);return t>=Ft?`#${(t-Ft).toString(16).padStart(8,"0")}`:`#${t.toString(16).padStart(6,"0")}`}function zw(e,t,r){const n=ys(e),a=ys(t),i=tu(e),o=tu(t);i.c<1e-7&&(i.h=o.h),o.c<1e-7&&(o.h=i.h);const s=(o.h-i.h+540)%360-180,l=Lw({l:i.l+(o.l-i.l)*r,c:i.c+(o.c-i.c)*r,h:i.h+s*r});return Dw({...l,a:n.a+(a.a-n.a)*r,hasAlpha:n.hasAlpha||a.hasAlpha})}function Uw(e,t,r){if(e.length===0)return St(r);const n=e[0];if(e.length===1||t<=n.frame)return St(n.value);const a=e[e.length-1];if(t>=a.frame)return St(a.value);for(let i=0;i<e.length-1;i+=1){const o=e[i],s=e[i+1];if(o.frame>t||s.frame<=t)continue;const l=s.frame-o.frame;if(l<=0)return St(o.value);const c=(t-o.frame)/l,u=o.easingConfig?Ws(c,o.easingConfig):Vs(c,o.easing);return zw(o.value,s.value,u)}return St(r)}function Hw(e,t,r){const n=dl(r);return n===null?null:vf(Uw(e,t,n))}function bf(e,t,r){return Math.max(t,Math.min(r,e))}function ru(e,t,r){const n=bf(r,0,1),a=e>>16&255,i=e>>8&255,o=e&255,s=t>>16&255,l=t>>8&255,c=t&255;return(Math.round(a+(s-a)*n)<<16)+(Math.round(i+(l-i)*n)<<8)+Math.round(o+(c-o)*n)}function vs(e){const t=bf(e,0,1);return t*t*(3-2*t)}function Nw(e){return e<.15?vs(e/.15):1-vs((e-.15)/.85)}function qw(e,t){if(!e.enabled||e.beats.length===0)return null;const r=Math.max(1,e.durationFrames);let n=0,a=0,i=0,o=!1;for(const l of e.beats){const c=(t-l.frame)/r;if(c<0||c>=1)continue;const u=Nw(c)*(.5+.5*l.amplitude);u>n&&(n=u,a=c,i=l.amplitude,o=!0)}if(!o)return{strength:0,chroma:0,phase:0,glowColor:e.glowColorBase};const s=ru(e.glowColorBase,16777215,.18+.28*i);return{strength:e.strength*n,chroma:e.chroma*(1-a)*(.5+.5*i),phase:a,glowColor:ru(e.glowColorBase,s,vs(n))}}function Gw(e,t){const r=e.audioPulse;return!r?.enabled||e.effect.type!=="gpu-effect"||e.effect.gpuEffectType!=="gpu-trigger-wave"?null:qw(r,t)}function Kw(e,t){switch(t){case"strength":return e.strength;case"chroma":return e.chroma;case"phase":return e.phase;case"glowColor":return vf(e.glowColor);default:return null}}const Vw={"gpu-gaussian-blur":new Set(["samples"]),"gpu-motion-blur":new Set(["samples"]),"gpu-radial-blur":new Set(["samples"]),"gpu-zoom-blur":new Set(["samples"]),"gpu-grain":new Set(["speed"]),"gpu-glow":new Set(["rings","samplesPerRing"]),"gpu-scanlines":new Set(["speed"]),"gpu-color-glitch":new Set(["speed"]),"gpu-block-glitch":new Set(["speed"]),"gpu-pixel-sort":new Set(["length"])};function Sf(e,t,r){switch(e){case"gpu-dither":return r==="angle"?t.mode==="linear":r==="scale"||r==="offsetX"||r==="offsetY"?t.mode==="radial":!0;case"gpu-ascii":return r==="textColor"?t.matchSourceColor!==!0:r==="colorSaturation"?t.matchSourceColor===!0:!0;default:return!0}}function Ww(e,t){return typeof e.params[t]=="number"&&!Vw[e.gpuEffectType]?.has(t)}function If(e,t){const n=K1(e.gpuEffectType)?.params[t],a=e.params[t];return n?.type==="color"&&n.animatable===!0&&typeof a=="string"&&dl(a)!==null}function xf(e,t){return Ww(e,t)||If(e,t)}function jw(e,t){const r=e.params[t];return typeof r=="number"?r:null}function $w(e,t){const r=e.params[t];return typeof r!="string"||dl(r)===null?null:r}function Yw(e,t){if(e.effect.type!=="gpu-effect"||!t)return[];const r=new Set;for(const n of t.properties){const a=Gn(n.property);a&&a.effectId===e.id&&a.gpuEffectType===e.effect.gpuEffectType&&r.add(a.paramKey)}return[...r]}function Xw(e){if(!e.effects||e.effects.length===0)return[];const t=[];for(const r of e.effects){if(r.effect.type!=="gpu-effect")continue;const n=r.effect;for(const[a,i]of Object.entries(n.params))typeof i!="number"&&typeof i!="string"||!xf(n,a)||Sf(n.gpuEffectType,n.params,a)&&t.push(Gs(n.gpuEffectType,r.id,a))}return t}function Qw(e,t,r){const n=e?.some(o=>o.audioPulse?.enabled)??!1;if(!e||e.length===0||!t&&!n)return e;let a=!1;const i=e.map(o=>{if(o.effect.type!=="gpu-effect")return o;const s=o.effect;let l=s.params,c=!1;const u=Gw(o,r),d=new Set([...Object.keys(s.params),...Yw(o,t)]);for(const m of d){if(!xf(s,m)||!Sf(s.gpuEffectType,l,m))continue;const p=u?Kw(u,m):null,f=p!==null?p:(()=>{const h=Gs(s.gpuEffectType,o.id,m),g=ia(t,h);return g.length===0?null:If(s,m)?(()=>{const v=$w(s,m);return v===null?null:Hw(g,r,v)})():(()=>{const v=jw(s,m);return v===null?null:en(g,r,v)})()})();f!==null&&l[m]!==f&&(c||(l={...s.params},c=!0),l[m]=f)}return c?(a=!0,{...o,effect:{...s,params:l}}):o});return a?i:e}const SE=Qw,ln=["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius"],nu=["cropLeft","cropRight","cropTop","cropBottom","cropSoftness"],bo=["volume"],Jw=["x","y","width","height","rotation"];function wf(e){const t=Xw(e);switch(e.type){case"audio":return[...bo,...t];case"video":return[...ln,...nu,...bo,...t];case"composition":return[...ln,...nu,...bo,...t];case"text":return[...ln,...Zm,...t];case"shape":return[...ln,"trimPathStart","trimPathEnd","trimPathOffset","taperStartWidth","taperEndWidth","taperStartLength","taperEndLength",...e.shapeType==="path"?Fb(e.pathVertices):[],...t];case"controller":return Jw;default:return[...ln,...t]}}function Zw(e,t){if(e.sourceItemType!==t.type)return{compatible:!1,reason:"type-mismatch"};const r=new Set(wf(t));for(const n of e.properties)if(!Gn(n.property)&&!r.has(n.property))return{compatible:!1,reason:"missing-property"};for(const n of e.vectorProperties??[])if(!(n.property==="position"?["x","y"]:n.property==="scale"?["width","height"]:["anchorX","anchorY"]).every(i=>r.has(i)))return{compatible:!1,reason:"missing-property"};return{compatible:!0}}function Se(){return Oe("TimelineActions")}function ml(e,t,r){return lf(`tl.action.${e}`,()=>cl().executeAnonymous(e,t,r))}function le(e,t){const r=I.getState().items,n=q.getState().transitions,{valid:a,repaired:i,broken:o}=lf("tl.repairTransitions",()=>gf(e,r,n,t,W.getState().fps)),s=i.map(l=>l.repaired);if(q.getState().setTransitions([...a,...s]),i.length>0)for(const l of i)Se().info(`[TransitionRepair] ${l.action}`);o.length>0&&Sw("timeline.transitionBreakagesDetected",{breakages:[...o]})}function Ge(e,t){const n=I.getState().items.find(o=>o.id===e);if(!n)return!1;const a=q.getState().transitions;return Yn(t,e,n,a)===void 0}function Ni(){return{items:I.getState().items,tracks:I.getState().tracks,transitions:q.getState().transitions,keyframes:H.getState().keyframes}}function ek(e){const t=de.getState(),r=(t.breadcrumbs[0]?.compositionId??null)===null,n={compositionId:null,items:[],tracks:[],transitions:[],keyframes:[]};if(r){if(t.activeCompositionId===null)return{compositionId:null,...e};const i=t.stashStack[0];return i?{compositionId:i.compositionId,items:i.items,tracks:i.tracks,transitions:i.transitions,keyframes:i.keyframes}:n}const a=t.mainHolder;return a?{compositionId:null,items:a.items,tracks:a.tracks,transitions:a.transitions,keyframes:a.keyframes}:n}function tk(e){const{activeCompositionId:t}=de.getState();return ae.getState().compositions.map(n=>n.id!==t?n:{...n,items:e.items,tracks:e.tracks,transitions:e.transitions,keyframes:e.keyframes})}function kf(e,t){const r=e.find(n=>n.id===t);return r?[r]:[]}function ur(e,t,r){return r?Wn(e,$n(e,t)):Wn(e,Array.from(new Set(t)))}function da(e,t,r){return r?sa(e,t):kf(e,t)}function nn(e,t,r){return r?jn(e,t):kf(e,t)}function Pf(e,t,r,n){return n?AI(e,t,r):null}function au(e,t,r,n,a){return a?FI(e,t,r,n):null}function rk(e,t,r){if(!r){const i=new Map(t);for(const[o,s]of t)if(!(s<=0))for(const l of Wn(e,[o]))i.set(l,Math.max(i.get(l)??0,s));return e.flatMap(o=>{const s=i.get(o.id)??0;return s>0?[{id:o.id,from:o.from-s}]:[]})}const n=new Map(t),a=new Set;for(const i of e){if(a.has(i.id))continue;const o=sa(e,i.id);for(const l of o)a.add(l.id);if(o.length<=1)continue;let s=0;for(const l of o)s=Math.max(s,t.get(l.id)??0);if(!(s<=0))for(const l of o)n.set(l.id,s)}for(const[i,o]of n)if(!(o<=0))for(const s of Wn(e,[i]))n.set(s,Math.max(n.get(s)??0,o));return e.flatMap(i=>{const o=n.get(i.id)??0;return o>0?[{id:i.id,from:i.from-o}]:[]})}function Ti(e){return e?!e.locked&&e.syncLock!==!1:!0}function nk(e){if(e.length===0)return;const t=new Map(e.map(n=>[n.originalId,n.result.rightItem.id])),r=q.getState().transitions.map(n=>{const a=t.get(n.leftClipId);return a?{...n,leftClipId:a}:(t.has(n.rightClipId),n)});q.getState().setTransitions(r)}function ak(e){const t=e.filter(i=>!!i.originalLinkedGroupId);if(t.length===0)return;const r=I.getState(),n=t.length>1?$():void 0,a=t.length>1?$():void 0;for(const i of t)r._updateItem(i.result.leftItem.id,{linkedGroupId:n}),r._updateItem(i.result.rightItem.id,{linkedGroupId:a})}function ma(e){e.length!==0&&(nk(e),ak(e))}function Qn(e){return Array.from(new Set(e))}function ik(e){const t=e.map(n=>({start:Math.max(0,Math.round(n.start)),end:Math.max(0,Math.round(n.end))})).filter(n=>n.end>n.start).sort((n,a)=>n.start-a.start);if(t.length===0)return[];const r=[t[0]];for(let n=1;n<t.length;n+=1){const a=t[n],i=r[r.length-1];if(a.start<=i.end){i.end=Math.max(i.end,a.end);continue}r.push({...a})}return r}function ok(e,t,r){const n=new Set;for(const a of t)!r.has(a.id)&&Ti(a)&&n.add(a.id);for(const a of e){if(r.has(a.trackId)||n.has(a.trackId))continue;const i=t.find(o=>o.id===a.trackId);Ti(i)&&n.add(a.trackId)}return[...n]}function Mf(e){const{items:t,tracks:r}=I.getState();return ok(t,r,e)}function Pn(e,t){const r=I.getState().itemById[e];if(!r)return null;const n=I.getState()._splitItem(e,t);return n?(ma([{originalId:r.id,originalLinkedGroupId:r.linkedGroupId,result:n}]),n):null}function sk(e,t){const r=I.getState(),n=[],a=[],i=I.getState().items.filter(o=>o.trackId===e&&o.from<t.end&&o.from+o.durationInFrames>t.start).sort((o,s)=>o.from-s.from);for(const o of i){const s=I.getState().itemById[o.id];if(!s||s.trackId!==e)continue;const l=s.from+s.durationInFrames,c=s.from<t.start,u=l>t.end;if(!c&&!u){r._removeItems([s.id]),a.push(s.id);continue}if(c&&u){const m=Pn(s.id,t.start);if(!m)continue;n.push(m.leftItem.id,m.rightItem.id);const p=Pn(m.rightItem.id,t.end);if(!p)continue;r._removeItems([p.leftItem.id]),a.push(p.leftItem.id),n.push(p.rightItem.id);continue}if(c){const m=Pn(s.id,t.start);if(!m)continue;r._removeItems([m.rightItem.id]),a.push(m.rightItem.id),n.push(m.leftItem.id);continue}const d=Pn(s.id,t.end);d&&(r._removeItems([d.leftItem.id]),a.push(d.leftItem.id),n.push(d.rightItem.id))}return{affectedIds:Qn(n),removedIds:Qn(a)}}function Cf(e,t,r){if(r===0)return[];const n=I.getState(),a=I.getState().items.filter(i=>i.trackId===e&&t(i)).map(i=>({id:i.id,from:Math.max(0,i.from+r)}));return a.length>0&&n._moveItems(a),a.map(i=>i.id)}function qi(e){const t=ik(e.intervals);if(t.length===0)return{affectedIds:[],removedIds:[]};const r=Mf(e.editedTrackIds),n=[],a=[];for(const i of r){let o=0;for(const s of t){const l={start:s.start-o,end:s.end-o},c=l.end-l.start;if(c<=0)continue;const u=sk(i,l);n.push(...u.affectedIds),a.push(...u.removedIds),n.push(...Cf(i,d=>d.from>=l.end,-c)),o+=c}}return{affectedIds:Qn(n),removedIds:Qn(a)}}function lk(e){const t=Math.max(0,Math.round(e.cutFrame)),r=Math.max(0,Math.round(e.amount));if(r===0)return{affectedIds:[],removedIds:[]};const n=Mf(e.editedTrackIds),a=[];for(const i of n){const o=I.getState().items.filter(s=>s.trackId===i&&s.from<t&&s.from+s.durationInFrames>t).sort((s,l)=>s.from-l.from);for(const s of o){const l=I.getState().itemById[s.id];if(!l||l.trackId!==i)continue;const c=Pn(l.id,t);c&&a.push(c.leftItem.id,c.rightItem.id)}a.push(...Cf(i,s=>s.from>=t,r))}return{affectedIds:Qn(a),removedIds:[]}}function ck(e,t,r){let n=Math.max(0,e);for(const a of r){const i=a.from+a.durationInFrames;if(!(i<=n)){if(a.from>=n+t)break;n=i}}return n}function bs(e){const t=new Map,r=[];for(const a of I.getState().items){const i=t.get(a.trackId);i?i.push(a):t.set(a.trackId,[a])}for(const a of t.values())a.sort((i,o)=>i.from-o.from);const n=new Map;for(const a of e){const i=n.get(a.trackId);i?i.push(a):n.set(a.trackId,[a])}for(const a of n.values())a.sort((i,o)=>i.from-o.from);for(const a of n.values())for(const i of a){let o=t.get(i.trackId);o||(o=[],t.set(i.trackId,o));const s=ck(i.from,i.durationInFrames,o),l=s===i.from?i:{...i,from:s};r.push(l),o.push({trackId:l.trackId,from:l.from,durationInFrames:l.durationInFrames}),o.sort((c,u)=>c.from-u.from)}return r}function uk(e){return e.type==="video"||e.type==="composition"}function dk(e){return[...e].sort((t,r)=>(t.order??0)-(r.order??0))}function mk(e,t){return Math.max(0,t-e-1)}function Tf(e){return e.type!=="audio"}function pk(e,t){const r=e.map(o=>{const s=t.get(o.id)??[],l=s.some(Tf),c=s.some(u=>u.type==="audio");return l?"video":c?"audio":Wt(o)}),n=r.findIndex(o=>o==="audio"),a=n===0?"audio":"video",i=new Map;return e.forEach((o,s)=>{const l=r[s]??(n!==-1&&s>=n?"audio":a);i.set(o.id,l)}),i}function So(e){const{sourceTrack:t,kind:r,name:n,order:a,usedTrackIds:i,createId:o}=e,s=t&&!i.has(t.id)?t.id:`track-${o()}`;return i.add(s),{id:s,name:n,kind:r,height:t?.height??yr,locked:t?.locked??!1,syncLock:t?.syncLock??!0,visible:t?.visible??!0,muted:t?.muted??!1,solo:t?.solo??!1,volume:t?.volume??0,color:t?.color,order:a,items:[]}}function fk(e){const t=e.items.filter(i=>uk(i)).toSorted((i,o)=>{const s=e.trackOrderMap.get(i.trackId)??0,l=e.trackOrderMap.get(o.trackId)??0;return s!==l?s-l:i.from!==o.from?i.from-o.from:i.id.localeCompare(o.id)}),r=e.items.filter(i=>i.type==="audio"),n=new Set,a=[];for(const i of t){const o=e.videoTrackLaneIndex.get(i.trackId);if(o===void 0)continue;const s=Gd(e.items,i);let l=s&&!n.has(s.id)?s:null;!l&&i.type==="video"&&(l=r.find(c=>!n.has(c.id)&&_I([i,c]))??null),l&&(n.add(l.id),a.push({video:i,audio:l,videoLaneIndex:o}))}return a}function iu(e,t){const r=e?.properties.find(a=>a.property==="volume");if(!r||r.keyframes.length===0)return null;const n={property:"volume",keyframes:r.keyframes.map(a=>({...a}))};return{itemId:t,properties:[n]}}function Ef(e,t,r){return{id:r(),type:"audio",trackId:t,from:e.from,durationInFrames:e.durationInFrames,label:e.label,mediaId:e.mediaId,originId:e.originId,linkedGroupId:e.linkedGroupId,src:e.src,trimStart:e.trimStart,trimEnd:e.trimEnd,sourceStart:e.sourceStart,sourceEnd:e.sourceEnd,sourceDuration:e.sourceDuration,sourceFps:e.sourceFps,speed:e.speed,volume:e.volume,audioFadeIn:e.audioFadeIn,audioFadeOut:e.audioFadeOut,audioFadeInCurve:e.audioFadeInCurve,audioFadeOutCurve:e.audioFadeOutCurve,audioFadeInCurveX:e.audioFadeInCurveX,audioFadeOutCurveX:e.audioFadeOutCurveX,audioPitchSemitones:e.audioPitchSemitones,audioPitchCents:e.audioPitchCents,audioEqEnabled:e.audioEqEnabled,audioEqOutputGainDb:e.audioEqOutputGainDb,audioEqBand1Enabled:e.audioEqBand1Enabled,audioEqBand1Type:e.audioEqBand1Type,audioEqBand1FrequencyHz:e.audioEqBand1FrequencyHz,audioEqBand1GainDb:e.audioEqBand1GainDb,audioEqBand1Q:e.audioEqBand1Q,audioEqBand1SlopeDbPerOct:e.audioEqBand1SlopeDbPerOct,audioEqLowCutEnabled:e.audioEqLowCutEnabled,audioEqLowCutFrequencyHz:e.audioEqLowCutFrequencyHz,audioEqLowCutSlopeDbPerOct:e.audioEqLowCutSlopeDbPerOct,audioEqLowEnabled:e.audioEqLowEnabled,audioEqLowType:e.audioEqLowType,audioEqLowGainDb:e.audioEqLowGainDb,audioEqLowFrequencyHz:e.audioEqLowFrequencyHz,audioEqLowQ:e.audioEqLowQ,audioEqLowMidEnabled:e.audioEqLowMidEnabled,audioEqLowMidType:e.audioEqLowMidType,audioEqLowMidGainDb:e.audioEqLowMidGainDb,audioEqLowMidFrequencyHz:e.audioEqLowMidFrequencyHz,audioEqLowMidQ:e.audioEqLowMidQ,audioEqMidGainDb:e.audioEqMidGainDb,audioEqHighMidEnabled:e.audioEqHighMidEnabled,audioEqHighMidType:e.audioEqHighMidType,audioEqHighMidGainDb:e.audioEqHighMidGainDb,audioEqHighMidFrequencyHz:e.audioEqHighMidFrequencyHz,audioEqHighMidQ:e.audioEqHighMidQ,audioEqHighEnabled:e.audioEqHighEnabled,audioEqHighType:e.audioEqHighType,audioEqHighGainDb:e.audioEqHighGainDb,audioEqHighFrequencyHz:e.audioEqHighFrequencyHz,audioEqHighQ:e.audioEqHighQ,audioEqBand6Enabled:e.audioEqBand6Enabled,audioEqBand6Type:e.audioEqBand6Type,audioEqBand6FrequencyHz:e.audioEqBand6FrequencyHz,audioEqBand6GainDb:e.audioEqBand6GainDb,audioEqBand6Q:e.audioEqBand6Q,audioEqBand6SlopeDbPerOct:e.audioEqBand6SlopeDbPerOct,audioEqHighCutEnabled:e.audioEqHighCutEnabled,audioEqHighCutFrequencyHz:e.audioEqHighCutFrequencyHz,audioEqHighCutSlopeDbPerOct:e.audioEqHighCutSlopeDbPerOct,offset:e.offset}}function hk(e){const t=e.createId??$,r=dk(e.tracks),n=new Map(r.map(O=>[O.id,O])),a=new Map;for(const O of e.items){const N=a.get(O.trackId);N?N.push(O):a.set(O.trackId,[O])}const i=pk(r,a),o=r.filter(O=>i.get(O.id)==="video").map(O=>O.id),s=r.filter(O=>i.get(O.id)==="audio").map(O=>O.id),l=new Map(o.map((O,N)=>[O,mk(N,o.length)])),c=[...o].reverse(),u=new Map(r.map(O=>[O.id,O.order??0])),d=fk({items:e.items,videoTrackLaneIndex:l,trackOrderMap:u}),m=new Map(d.map(O=>[O.video.id,O.audio])),p=new Map;for(const O of d)p.has(O.videoLaneIndex)||p.set(O.videoLaneIndex,O.audio.trackId);const f=new Set(d.map(O=>O.videoLaneIndex));for(const O of e.items){if(O.type!=="video"||!O.mediaId||e.videoHasAudioByMediaId[O.mediaId]!==!0)continue;const N=l.get(O.trackId);N!==void 0&&f.add(N)}const h=[...f].sort((O,N)=>O-N),g=h.length,v=s.filter(O=>(a.get(O)?.length??0)===0),S=new Set,b=[];for(const O of h){const N=p.get(O)??v.shift()??c[O]??null;N?(b.push(N),S.add(N)):b.push(c[O]??`generated-audio-source-${O}`)}const x=new Set(d.map(O=>O.audio.id)),T=e.items.filter(O=>O.type==="audio"&&!x.has(O.id)).toSorted((O,N)=>{const ie=u.get(O.trackId)??0,oe=u.get(N.trackId)??0;return ie!==oe?ie-oe:O.from!==N.from?O.from-N.from:O.id.localeCompare(N.id)}).map(O=>O.trackId).filter((O,N,ie)=>ie.indexOf(O)===N),C=s.filter(O=>!S.has(O)),k=[...T,...C.filter(O=>!T.includes(O))],_=new Set,P=[],R=new Map,E=new Map,D=new Map;o.forEach((O,N)=>{const ie=l.get(O)??0,oe=So({sourceTrack:n.get(O)??null,kind:"video",name:`V${ie+1}`,order:N,usedTrackIds:_,createId:t});P.push(oe),R.set(ie,oe.id)}),b.forEach((O,N)=>{const ie=h[N]??N,oe=So({sourceTrack:n.get(O)??null,kind:"audio",name:`A${N+1}`,order:o.length+N,usedTrackIds:_,createId:t});P.push(oe),E.set(ie,oe.id)}),k.forEach((O,N)=>{const ie=g+N+1,oe=So({sourceTrack:n.get(O)??null,kind:"audio",name:`A${ie}`,order:o.length+g+N,usedTrackIds:_,createId:t});P.push(oe),D.set(O,oe.id)});const L=new Map(e.keyframes.map(O=>[O.itemId,O])),U=[...e.keyframes.map(O=>({...O,properties:O.properties.map(N=>({...N,keyframes:N.keyframes.map(ie=>({...ie}))}))}))],V=[],ne=[],ee=new Map,ze=k[0]?D.get(k[0]):E.get(0);for(const O of e.items){if(O.type==="audio"){const N=d.find(oe=>oe.audio.id===O.id);if(N){const oe=E.get(N.videoLaneIndex)??O.trackId,ke=ee.get(N.video.id)??N.video.linkedGroupId??N.audio.linkedGroupId??t();ee.set(N.video.id,ke);const De=N.audio.trackId===oe&&N.audio.linkedGroupId===ke?N.audio:{...N.audio,trackId:oe,linkedGroupId:ke};if(!L.get(O.id)?.properties.some(Ye=>Ye.property==="volume")){const Ye=iu(L.get(N.video.id),O.id);Ye&&U.push(Ye)}V.push(De);continue}const ie=D.get(O.trackId)??ze;V.push(ie&&O.trackId!==ie?{...O,trackId:ie}:O);continue}if(O.type==="video"){const N=l.get(O.trackId)??0,ie=R.get(N)??O.trackId,oe=m.get(O.id);let ke=ie!==O.trackId?{...O,trackId:ie}:O;if(oe){const De=ee.get(O.id)??ke.linkedGroupId??oe.linkedGroupId??t();ke.linkedGroupId!==De&&(ke={...ke,linkedGroupId:De}),ee.set(O.id,De)}if(!oe&&O.mediaId&&e.videoHasAudioByMediaId[O.mediaId]===!0){const De=E.get(N);if(De){const jt=ke.linkedGroupId??t();ke.linkedGroupId!==jt&&(ke={...ke,linkedGroupId:jt});const Ye=Ef(ke,De,t);ne.push(Ye);const an=iu(L.get(O.id),Ye.id);an&&U.push(an)}}V.push(ke);continue}if(Tf(O)){const N=l.get(O.trackId)??0,ie=R.get(N)??O.trackId;V.push(ie!==O.trackId?{...O,trackId:ie}:O);continue}V.push(O)}V.push(...ne);const xe=JSON.stringify({tracks:e.tracks,items:e.items,keyframes:e.keyframes})!==JSON.stringify({tracks:P,items:V,keyframes:U});return{tracks:P,items:V,keyframes:U,changed:xe}}function gk(e,t,r,n){return e<r+n&&r<e+t}function yk(e,t){const r=e.reduce((n,a)=>Math.max(n,a.order??0),-1);return{id:`track-${t()}`,name:Bs(e,"audio"),kind:"audio",height:yr,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,volume:0,order:r+1,items:[]}}function vk(e){const t=e.createId??(()=>$()),r=e.video.linkedGroupId??t(),n=e.video.linkedGroupId===r?e.video:{...e.video,linkedGroupId:r},{from:a,durationInFrames:i}=n;let o=null;const s=e.tracks.filter(u=>!u.isGroup&&Wt(u)==="audio").toSorted((u,d)=>(u.order??0)-(d.order??0));for(const u of s)if(!(e.itemsByTrackId.get(u.id)??[]).some(p=>gk(a,i,p.from,p.durationInFrames))){o=u.id;break}let l=null;o||(l=yk(e.tracks,t),o=l.id);const c=Ef(n,o,t);return{updatedVideo:n,audioItem:c,newTrack:l}}function bk(e){const{trackId:t,from:r,durationInFrames:n,canvasWidth:a,canvasHeight:i}=e,o=Math.max(48,Math.min(a,i)*.08);return{id:$(),type:"controller",controllerKind:"null",trackId:t,from:r,durationInFrames:n,label:"Null Object",transform:{x:0,y:0,width:o,height:o,rotation:0,opacity:1,aspectRatioLocked:!0}}}function Sk(e){const t=new Map,r=new Map,n=new Map;for(const a of e)n.set(a.id,a),r.has(a.trackId)||r.set(a.trackId,[]),r.get(a.trackId).push(a),t.has(a.leftClipId)||t.set(a.leftClipId,{}),t.get(a.leftClipId).outgoing=a,t.has(a.rightClipId)||t.set(a.rightClipId,{}),t.get(a.rightClipId).incoming=a;return{transitionsByClipId:t,transitionsByTrackId:r,transitionsById:n}}function Ik(e,t){const r=new Set(e),n=new Set,a=[];for(const i of e){if(n.has(i))continue;let o=i;const s=new Set;for(;;){const u=t.get(o);if(u?.incoming&&r.has(u.incoming.leftClipId)&&!n.has(u.incoming.leftClipId)&&!s.has(u.incoming.leftClipId))s.add(o),o=u.incoming.leftClipId;else break}const l=[];let c=o;for(;c&&r.has(c)&&!n.has(c);)n.add(c),l.push(c),c=t.get(c)?.outgoing?.rightClipId;l.length>0&&a.push(l)}return a}function xk(e){if(e<=0)return{cols:0,rows:0};const t=Math.ceil(Math.sqrt(e)),r=Math.ceil(e/t);return{cols:t,rows:r}}function Hr(e,t,r,n){const a=Math.min(r/e,n/t);return{width:Math.round(e*a),height:Math.round(t*a)}}function wk(e,t,r,n){const a=new Map;if(e.length===0)return null;const i=n.gap??0,o=n.padding??0;return{result:a,gap:i,padding:o,availableWidth:t-o*2,availableHeight:r-o*2,canvasCenterX:t/2,canvasCenterY:r/2}}function kk(e,t,r){const{result:n,gap:a,padding:i,availableWidth:o,availableHeight:s,canvasCenterX:l,canvasCenterY:c}=r;let u,d;switch(t.preset){case"row":u=e.length,d=1;break;case"column":u=1,d=e.length;break;case"grid":u=t.cols??2,d=t.rows??2;break;default:{const f=xk(e.length);u=f.cols,d=f.rows;break}}const m=(o-a*(u-1))/u,p=(s-a*(d-1))/d;for(let f=0;f<e.length;f++){const h=e[f],g=f%u,v=Math.floor(f/u),S=i+g*(m+a),b=i+v*(p+a),x=Hr(h.sourceWidth,h.sourceHeight,m,p),T=S+m/2,C=b+p/2;n.set(h.id,{x:T-l,y:C-c,width:x.width,height:x.height,rotation:0})}return n}function Pk(e,t){const{result:r,gap:n,padding:a,availableWidth:i,availableHeight:o,canvasCenterX:s,canvasCenterY:l}=t,c=e[0],u=Hr(c.sourceWidth,c.sourceHeight,i,o);if(r.set(c.id,{x:0,y:0,width:u.width,height:u.height,rotation:0}),e.length<2)return r;const d=i/4,m=e.slice(1),p=m.reduce((S,b)=>{const x=Hr(b.sourceWidth,b.sourceHeight,d,d);return S+x.height},0)+n*(m.length-1),f=a+i;let h=a+o;const g=o,v=p>g?g/p:1;for(let S=m.length-1;S>=0;S--){const b=m[S],x=d*v,T=Hr(b.sourceWidth,b.sourceHeight,x,x),C=f-x/2-n,k=h-T.height/2-n;r.set(b.id,{x:C-s,y:k-l,width:T.width,height:T.height,rotation:0}),h-=T.height+n*v}return r}function Mk(e,t){const{result:r,gap:n,padding:a,availableWidth:i,availableHeight:o,canvasCenterX:s,canvasCenterY:l}=t,c=(i-n)*2/3,u=o,d=e[0],m=Hr(d.sourceWidth,d.sourceHeight,c,u),p=a+c/2,f=a+u/2;if(r.set(d.id,{x:p-s,y:f-l,width:m.width,height:m.height,rotation:0}),e.length<2)return r;const h=(i-n)/3,g=a+c+n,v=e.slice(1),S=(o-n*(v.length-1))/v.length;for(let b=0;b<v.length;b++){const x=v[b],T=Hr(x.sourceWidth,x.sourceHeight,h,S),C=a+b*(S+n),k=g+h/2,_=C+S/2;r.set(x.id,{x:k-s,y:_-l,width:T.width,height:T.height,rotation:0})}return r}function Ck(e,t,r,n){const a=wk(e,t,r,n);if(!a)return new Map;switch(n.preset){case"pip":return Pk(e,a);case"focus-sidebar":return Mk(e,a);case"auto":case"row":case"column":case"grid":return kk(e,n,a)}}const se=w().min(1),Je=F(se).min(1),Kt=M().finite(),Tk=kt(w(),_i()),Ek=["video","audio","text","image","lottie","shape","adjustment","controller","composition","subtitle"],Or=Zn({id:se.optional(),type:z(Ek),trackId:se,from:Kt,durationInFrames:M().finite().positive(),label:w(),templateAnchor:z(["full","start","end"]).optional()}).superRefine((e,t)=>{const r=sf(e);if(r&&t.addIssue({code:"custom",path:["waveformVisualizer"],message:r}),e.templateAnchor==="full"&&e.type==="video"&&Ds(e)!==null&&t.addIssue({code:"custom",path:["templateAnchor"],message:"Clip neo toàn video phải lặp được (đã tắt tiếng, có khoảng nguồn)"}),e.loop!==!0)return;if(e.type!=="video"){t.addIssue({code:"custom",path:["loop"],message:"Chỉ clip video mới lặp được"});return}if(e.embeddedAudioMuted!==!0){t.addIssue({code:"custom",path:["loop"],message:"Chỉ lặp được clip đã tắt tiếng"});return}const n=e.sourceEnd??e.sourceDuration,a=e.sourceStart??e.trimStart??e.offset??0;(n===void 0||!Number.isFinite(n)||n<=a)&&t.addIssue({code:"custom",path:["loop"],message:"Clip cần biết khoảng nguồn để lặp"})}),Ff=z(["scenes","captions","source-audio","background-video"]),Nr=Zn({id:se,name:w(),height:M().finite().positive(),locked:B(),visible:B(),muted:B(),solo:B(),order:M().finite(),kind:z(["video","audio"]).optional(),templateSlot:Ff.optional(),templateLoudnessLufs:M().min(-24).max(-9).optional(),templateScenes:nf.optional(),templateBackdrop:af.optional(),templateBackgroundAppearance:of.optional()}).superRefine((e,t)=>{e.templateBackgroundAppearance!==void 0&&(e.templateSlot!=="background-video"||e.kind==="audio")&&t.addIssue({code:"custom",path:["templateBackgroundAppearance"],message:"Cài đặt nền chỉ đặt trên track video vai trò Nền video"})}),ou=y({id:se,from:Kt,trackId:se.optional()}).strict(),Fk=y({from:Kt,trackId:se}).strict(),su=y({width:M().finite().positive(),height:M().finite().positive(),fps:M().finite().positive()}).strict(),Fa=y({x:M().finite().optional(),y:M().finite().optional(),width:M().finite().optional(),height:M().finite().optional(),anchorX:M().finite().optional(),anchorY:M().finite().optional(),rotation:M().finite().optional(),flipHorizontal:B().optional(),flipVertical:B().optional(),opacity:M().finite().optional(),cornerRadius:M().finite().optional(),aspectRatioLocked:B().optional()}).strict(),Aa=z(["move","resize","rotate","opacity","corner_radius","transform"]),lu=z(["linear","ease-in","ease-out","ease-in-out","hold","cubic-bezier","spring"]),cu=y({x:M().finite(),y:M().finite()}).strict(),Io=Oi("type",[y({type:Z("add"),itemId:se,property:w().min(1),frame:Kt,value:M().finite(),easing:lu.optional()}).strict(),y({type:Z("update"),itemId:se,property:w().min(1),keyframeId:se,updates:y({value:M().finite().optional()}).strict()}).strict(),y({type:Z("vector-add"),itemId:se,property:z(["position","scale","anchor"]),frame:Kt,value:cu,easing:lu.optional()}).strict(),y({type:Z("vector-update"),itemId:se,property:z(["position","scale","anchor"]),keyframeId:se,updates:y({value:cu}).strict()}).strict()]),Ak=y({position:An([M().finite(),M().finite()]),inHandle:An([M().finite(),M().finite()]),outHandle:An([M().finite(),M().finite()]),tangentMode:z(["corner","smooth","continuous","broken"]).optional()}).strict(),_k=y({preset:z(["auto","row","column","pip","focus-sidebar","grid"]),cols:M().int().positive().optional(),rows:M().int().positive().optional(),gap:M().finite().optional(),padding:M().finite().optional()}).strict(),Ok=y({src:w(),path:w(),key:w(),quality:z(["preview","full"]),usesProxy:B(),isSourceLevel:B(),sourceDuration:M().finite().optional(),conformFps:M().finite().optional()}).strict(),Dk=["reverseConformSrc","reverseConformPath","reverseConformKey","reverseConformPreviewSrc","reverseConformPreviewPath","reverseConformPreviewKey","reverseConformPreviewUsesProxy","reverseConformPreviewIsSourceLevel","reverseConformPreviewSourceDuration","reverseConformPreviewFps","reverseConformStatus"],j=()=>({kind:"patch",patch:{}});function _a(e,t){return{...t,id:t.id??e.ids.mint()}}function or(e,t){const r={...e};for(const n of t??[])r[n]=void 0;return r}function Ae(e,t,r="itemIds"){const n=new Set(e.items.map(a=>a.id));return t.filter(a=>!n.has(a)).map(a=>({message:`Không tìm thấy clip ${a}`,code:"item.notFound",path:r}))}function cn(e,t,r="itemIds"){const n=new Set(t);return[...Ae(e,t,r),...e.items.filter(a=>n.has(a.id)&&a.type==="audio").map(a=>({message:`Clip âm thanh ${a.id} không hỗ trợ biến đổi hình ảnh`,code:"item.unsupportedTransform",path:r}))]}function tr(e,t,r="trackId"){const n=new Set(e.map(a=>a.id));return[...new Set(t)].filter(a=>a!==void 0&&!n.has(a)).map(a=>({message:`Không tìm thấy track ${a}`,code:"track.notFound",path:r}))}function Af(e,t){return new Set(e).size===e.length?[]:[{message:"Danh sách id bị trùng lặp",code:"id.duplicate",path:t}]}function Rk(e,t,r){const n=new Set(e.items.map(a=>a.id));return t.filter(a=>a!==void 0&&n.has(a)).map(a=>({message:`Clip ${a} đã tồn tại`,code:"item.exists",path:r}))}function _f(){const e=I.getState(),t=kg(e.tracks,e.items);t!==e.tracks&&e.setTracks(t)}function Lk(e){const t=I.getState().items,n=q.getState().transitions.map(a=>{if(!e.has(a.leftClipId)||!e.has(a.rightClipId))return a;const i=t.find(s=>s.id===a.leftClipId),o=t.find(s=>s.id===a.rightClipId);return i&&o&&i.trackId===o.trackId?{...a,trackId:i.trackId}:a});q.getState().setTransitions(n)}function pl(e){return e.type!=="audio"&&e.type!=="adjustment"}function Oa(e,t,r){const n=t.flatMap(a=>a.id?[a.id]:[]);return[...Af(n,"items"),...Rk(e,n,"items"),...tr(r,t.map(a=>a.trackId),"items.trackId"),...t.flatMap(a=>a.type!=="video"||a.templateAnchor!=="full"||!a.linkedGroupId?[]:[...e.items,...t].some(o=>o.type==="audio"&&o.linkedGroupId===a.linkedGroupId)?[{message:"Clip neo toàn video phải tắt tiếng và không gắn âm thanh",path:"items.templateAnchor"}]:[]),...t.flatMap(a=>{if(!a.waveformVisualizer)return[];const i=e.mediaItems?.find(s=>s.id===a.mediaId);if(!i||!i.mimeType.startsWith("audio/"))return[{message:"Chọn tệp âm thanh làm nguồn giọng",path:"items.mediaId"}];const o=a.waveformVisualizer;return o.envelopeBase64!==void 0&&o.sourceHash!==i.contentHash?[{message:"Dữ liệu sóng giọng đã cũ; tạo lại từ nguồn hiện tại",path:"items.waveformVisualizer"}]:[]})]}function uu(e){I.getState()._removeItems(e),q.getState()._removeTransitionsForItems(e),H.getState()._removeKeyframesForItems(e),_f()}function Bk(e,t){const r=I.getState().items,n=ur(r,e,t);if(n.length===0)return[];const a=new Set(n),i=r.filter(C=>!a.has(C.id)),o=new Map,s=new Set(r.filter(C=>a.has(C.id)).map(C=>C.trackId)),l=r.filter(C=>a.has(C.id)).map(C=>({start:C.from,end:C.from+C.durationInFrames}));for(const C of i){const k=r.filter(_=>a.has(_.id)).filter(_=>_.trackId===C.trackId&&_.from+_.durationInFrames<=C.from).reduce((_,P)=>_+P.durationInFrames,0);k>0&&o.set(C.id,k)}const c=new Map(I.getState().tracks.map(C=>[C.id,C])),u=new Map(i.map(C=>[C.id,C])),d=new Map;for(const[C,k]of o){if(k<=0)continue;const _=ur(i,[C],t);for(const P of _){const R=u.get(P);!R||!s.has(R.trackId)&&Ti(c.get(R.trackId))||d.set(P,Math.max(d.get(P)??0,k))}}const m=i.flatMap(C=>{const k=d.get(C.id)??0;return k>0?[{id:C.id,from:C.from-k}]:[]}),p=new Map(m.map(C=>[C.id,C.from])),f=[];for(const C of i){if(p.has(C.id)||a.has(C.id))continue;const k=C.from+C.durationInFrames;for(const _ of i){const P=p.get(_.id);if(P===void 0||_.trackId!==C.trackId)continue;const R=P+_.durationInFrames;if(P<k&&R>C.from){f.push(C.id);break}}}const h=ur(i,f,t),g=[...n,...h],v=new Set(h),S=v.size>0?m.filter(C=>!v.has(C.id)):m;I.getState()._removeItems(g),S.length>0&&I.getState()._moveItems(S);const b=qi({editedTrackIds:s,intervals:l}),x=Array.from(new Set([...g,...b.removedIds]));q.getState()._removeTransitionsForItems(x),H.getState()._removeKeyframesForItems(x),S.length>0&&le(S.map(C=>C.id));const T=Array.from(new Set([...m.map(C=>C.id),...b.affectedIds]));return T.length>0&&le(T,new Set(x)),_f(),g}function zk(e,t){const r=I.getState().items,n=Math.max(0,Math.round(t)),a=r.filter(d=>d.trackId===e).sort((d,m)=>d.from-m.from);if(a.length===0)return;let i=0,o=0;for(const d of a){if(n>=i&&n<d.from){o=d.from;break}i=d.from+d.durationInFrames}if(o<=i)return;const s=o-i,l=r.filter(d=>d.trackId===e&&d.from>=o).map(d=>({id:d.id,from:d.from-s}));if(l.length===0)return;I.getState()._moveItems(l);const c=qi({editedTrackIds:new Set([e]),intervals:[{start:i,end:o}]}),u=c.removedIds;u.length>0&&(q.getState()._removeTransitionsForItems(u),H.getState()._removeKeyframesForItems(u)),le(Array.from(new Set([...l.map(d=>d.id),...c.affectedIds])),u.length>0?new Set(u):void 0)}function Uk(e,t){const r=I.getState().items,n=r.filter(s=>s.trackId===e).sort((s,l)=>s.from-l.from);if(n.length===0)return;let a=0;const i=new Map;for(const s of n){const l=s.from>a?a:s.from,c=s.from-l;c>0&&i.set(s.id,c),a=l+s.durationInFrames}const o=rk(r,i,t);o.length!==0&&(I.getState()._moveItems(o),le(o.map(s=>s.id)))}function Hk(e,t,r){if(r.length===0)return;const n=new Map;for(let i=0;i<t.length;i+=1){const o=r[i];o&&n.set(t[i],o)}const a=[];for(const i of q.getState().transitions){const o=n.get(i.leftClipId),s=n.get(i.rightClipId);!o||!s||o.trackId!==s.trackId||a.push({...i,id:e.ids.mint(),leftClipId:o.id,rightClipId:s.id,trackId:o.trackId})}a.length>0&&q.getState().setTransitions([...q.getState().transitions,...a])}function Of(e){if(e)return{...e,...e.bezier&&{bezier:{...e.bezier}},...e.spring&&{spring:{...e.spring}}}}function Nk(e,t){return{...e,id:t(),easingConfig:Of(e.easingConfig)}}function qk(e,t){return{...e,id:t(),value:{...e.value},easingConfig:Of(e.easingConfig),temporalEase:e.temporalEase?{...e.temporalEase.in&&{in:{...e.temporalEase.in}},...e.temporalEase.out&&{out:{...e.temporalEase.out}}}:void 0,spatial:e.spatial?{...e.spatial,inTangent:{...e.spatial.inTangent},outTangent:{...e.spatial.outTangent}}:void 0}}function Gk(e,t){return{property:e.property,keyframes:e.keyframes.map(r=>Nk(r,t))}}function Kk(e,t){return{property:e.property,keyframes:e.keyframes.map(r=>qk(r,t))}}function Df(e,t){return{...e,sourceItemId:t.get(e.sourceItemId)??e.sourceItemId}}function Vk(e,t){return e.type==="link"?Df(e,t):{...e}}function Wk(e,t,r,n){const a={itemId:t,properties:e.properties.map(i=>Gk(i,n))};return e.animationVersion&&(a.animationVersion=e.animationVersion),e.expressions?.length&&(a.expressions=e.expressions.map(i=>Vk(i,r))),e.propertyLinks?.length&&(a.propertyLinks=e.propertyLinks.map(i=>Df(i,r))),e.vectorProperties?.length&&(a.vectorProperties=e.vectorProperties.map(i=>Kk(i,n))),e.separatedVectorProperties?.length&&(a.separatedVectorProperties=[...e.separatedVectorProperties]),a}function jk(e,t,r){if(r.length===0)return;const n=H.getState(),a=new Map(t.flatMap((s,l)=>{const c=r[l];return c?[[s,c.id]]:[]})),i=()=>e.ids.mint(),o=t.flatMap((s,l)=>{const c=r[l],u=n.keyframesByItemId[s];return!c||!u?[]:[Wk(u,c.id,a,i)]});o.length>0&&n.setKeyframes([...n.keyframes,...o])}function du(e,t){const r=new Set;for(const a of t){const i=jn(e,a);if(i.length===0){r.add(a);continue}for(const o of i)r.add(o.id)}const n=Array.from(r).map(a=>e.find(i=>i.id===a)).filter(a=>a!==void 0&&(a.type==="video"||a.type==="audio"));return{reversibleItems:n,shouldReverse:!n.every(a=>a.isReversed===!0)}}function $k(e,t,r){return!t||!pl(t)?!0:!!(r.parentItemId&&(Hb(e,r.parentItemId,r.getItem,r.getKeyframes)||Gb(e,r.parentItemId,r.getItem,r.getKeyframes)))}function Yk(e,t,r){return $k(e,t,r)?{status:"invalid"}:r.behavior==="preserve-world"&&t?.transformParent?.parentItemId===r.parentItemId?{status:"unchanged"}:!r.parentItemId&&r.behavior==="restore-local"?t?.transformParent?{status:"update",binding:void 0}:{status:"unchanged"}:null}function Xk(e,t,r,n){return e==="snap-to-parent"&&n?{...t,x:n.x,y:n.y}:e==="restore-local"?r:t}function Qk(e,t){const{parentItemId:r,parentWorld:n,behavior:a,frame:i,canvas:o,getItem:s,getKeyframes:l}=t,c=s(e),u=Yk(e,c,t);if(u)return u;if(!c)return{status:"invalid"};const d=l(c.id),m=Xs(c,{canvas:o,frame:i,keyframes:d,getItem:s,getKeyframes:l}),p=ap(c,{canvas:o,relativeFrame:i-c.from,keyframes:d,expressionContext:{globalFrame:i,canvas:o,getItem:s,getKeyframes:l}});return{status:"update",binding:zb({childLocal:p,childWorld:Xk(a,m,p,n),parentItemId:r,parentWorld:n})}}function mu(e,t){const r=new Map;for(const n of e){const a=Qk(n,t);if(a.status==="invalid")return null;a.status==="update"&&r.set(n,a.binding)}return r}function pu(e){const{parentItemId:t,parent:r,behavior:n="preserve-world",frame:a,canvas:i,getItem:o,getKeyframes:s}=e,l=r?Xs(r,{canvas:i,frame:a,keyframes:s(r.id),getItem:o,getKeyframes:s}):void 0;return{parentItemId:t,parentWorld:l,behavior:n,frame:a,canvas:i,getItem:o,getKeyframes:s}}function Jk(e,t,r){return e.length===0||t&&e.includes(t)?!1:t?!!(r&&pl(r)):!0}function Zk(e){const t="Null Object",r=new Set(e.map(a=>a.label));if(!r.has(t))return t;let n=2;for(;r.has(`${t} ${n}`);)n+=1;return`${t} ${n}`}function e2(e,t){const r=new Map(t.map(u=>[u.id,u])),n=e.map(u=>r.get(u.trackId)).filter(u=>!!u);if(n.length===0)return{order:t.reduce((u,d)=>Math.max(u,d.order),-1)+1};const a=new Set(n.map(u=>u.parentTrackId??null)),i=a.size===1?n[0]?.parentTrackId??void 0:void 0,o=a.size===1?n:n.map(u=>u.parentTrackId&&r.get(u.parentTrackId)||u),s=t.filter(u=>(u.parentTrackId??null)===(i??null)),l=o.filter(u=>(u.parentTrackId??null)===(i??null)).map(u=>u.order);return{order:l.length>0?Math.min(...l):s.reduce((u,d)=>Math.max(u,d.order),-1)+1,...i&&{parentTrackId:i}}}function t2(e,t){const{children:r,items:n,tracks:a,canvas:i}=t,o=e.ids.mint(),s=Math.min(...r.map(p=>p.from)),l=Math.max(...r.map(p=>p.from+p.durationInFrames)),c=Zk(n),u=e2(r,a),d={...bk({trackId:o,from:s,durationInFrames:Math.max(1,l-s),canvasWidth:i.width,canvasHeight:i.height,fps:i.fps}),id:e.ids.mint(),label:c},m={id:o,name:c,kind:"video",height:34,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,order:u.order,...u.parentTrackId&&{parentTrackId:u.parentTrackId},items:[]};return{parent:d,track:m}}function fu(){const e=I.getState(),t=H.getState();return{getItem:r=>e.itemById[r],getKeyframes:r=>t.keyframesByItemId[r]}}function xo(e){if(e.length===0)return!1;const t=H.getState();let r=!1;for(const n of e){if(n.type==="vector-update"){t._updateVectorKeyframe(n.itemId,n.property,n.keyframeId,n.updates),r=!0;continue}if(n.type==="update"){t._updateKeyframe(n.itemId,n.property,n.keyframeId,n.updates),r=!0;continue}if(!Ge(n.itemId,n.frame)){Se().warn("Cannot add auto keyframe in transition region",{itemId:n.itemId,property:n.property,frame:n.frame});continue}n.type==="vector-add"?t._upsertVectorKeyframe(n.itemId,n.property,{frame:n.frame,value:n.value,easing:n.easing}):t._addKeyframe(n.itemId,n.property,n.frame,n.value,n.easing),r=!0}return r}const r2=["x","y","width","height","rotation"];function wo(e){return Object.keys(e).sort().join(",")}function ko(e,t){return Ae(e,[...new Set((t??[]).map(r=>r.itemId))],"autoKeyframeOperations")}function Da(e,t){const r=e.items.filter(a=>t.has(a.id)).map(a=>a.id);if(r.length===0)return{...e,removedItemIds:r};const n=new Set(r);return{...e,items:e.items.filter(a=>!n.has(a.id)),transitions:e.transitions.filter(a=>!n.has(a.leftClipId)&&!n.has(a.rightClipId)),keyframes:e.keyframes.filter(a=>!n.has(a.itemId)),removedItemIds:r}}function Ra(e,t,r){let n=!1;const a=e.map(i=>i.id!==t?i:(n=!0,{...i,...r}));return{items:n?a:e,changed:n}}function n2(e){const t=new Set(e),r=Ni(),n=Da(r,t);let a=n.removedItemIds.length>0;n.removedItemIds.length>0&&(I.getState().setItems(n.items),q.getState().setTransitions(n.transitions),H.getState().setKeyframes(n.keyframes));const i=de.getState().activeCompositionId,o=ae.getState().compositions.map(d=>{if(d.id===i&&n.removedItemIds.length>0)return{...d,items:n.items,tracks:n.tracks,transitions:n.transitions,keyframes:n.keyframes};const m=d.id===i?{items:n.items,tracks:n.tracks,transitions:n.transitions,keyframes:n.keyframes}:{items:d.items,tracks:d.tracks,transitions:d.transitions,keyframes:d.keyframes},p=Da(m,t);return p.removedItemIds.length>0&&(a=!0),p.removedItemIds.length===0?d:{...d,items:p.items,tracks:p.tracks,transitions:p.transitions,keyframes:p.keyframes}}),s=de.getState(),l=s.stashStack.map(d=>{const m=Da(d,t);return m.removedItemIds.length>0&&(a=!0),m.removedItemIds.length===0?d:{...d,items:m.items,transitions:m.transitions,keyframes:m.keyframes}}),c=s.mainHolder;let u=c;if(c){const d=Da(c,t);d.removedItemIds.length>0&&(a=!0,u={...c,items:d.items,transitions:d.transitions,keyframes:d.keyframes})}return a&&(ae.getState().setCompositions(o),de.setState(d=>({...d,stashStack:l,mainHolder:u}))),a}function a2(e,t){const r=Ni(),n=Ra(r.items,e,t);let a=n.changed;n.changed&&I.getState().setItems(n.items);const i=de.getState().activeCompositionId,o=ae.getState().compositions.map(m=>{if(m.id===i&&n.changed)return{...m,items:n.items};const p=m.id===i?n.items:m.items,f=Ra(p,e,t);return f.changed&&(a=!0),f.changed?{...m,items:f.items}:m}),s=de.getState(),l=s.stashStack.map(m=>{const p=Ra(m.items,e,t);return p.changed&&(a=!0),p.changed?{...m,items:p.items}:m}),c=s.mainHolder;let u=c;if(c){const m=Ra(c.items,e,t);m.changed&&(a=!0,u={...c,items:m.items})}return a?(ae.getState().setCompositions(o),de.setState(m=>({...m,stashStack:l,mainHolder:u})),("from"in t||"durationInFrames"in t||"trackId"in t)&&n.changed&&le([e]),!0):!1}const Po={updates:Tk,clearFields:F(w().min(1)).optional()};function Mo(e,t){const r=[];return"templateAnchor"in e&&!z(["full","start","end"]).safeParse(e.templateAnchor).success&&r.push({message:"Mốc neo template không hợp lệ",path:"updates.templateAnchor"}),("id"in e||t?.includes("id"))&&r.push({message:"Không thể đổi id của clip",code:"item.idImmutable",path:"updates.id"}),"from"in e&&!(typeof e.from=="number"&&Number.isFinite(e.from))&&r.push({message:"from phải là số frame hợp lệ",path:"updates.from"}),"durationInFrames"in e&&!(typeof e.durationInFrames=="number"&&Number.isFinite(e.durationInFrames)&&e.durationInFrames>0)&&r.push({message:"durationInFrames phải lớn hơn 0",path:"updates.durationInFrames"}),r}const hu={waveformVisualizer:!0,type:!0,shapeType:!0,mediaId:!0,isMask:!0};function fl(e){return[e.items,...e.compositions.map(t=>t.items),...(e.stashStack??[]).map(t=>t.items),...e.mainHolder?[e.mainHolder.items]:[]]}function gu(e,t,r,n){return r.loop!==!0?[]:fl(e).flatMap(a=>a.filter(i=>i.id===t).flatMap(i=>{if(i.type!=="video")return[{message:"Chỉ clip video mới lặp được",code:"item.loopVideoOnly",path:"updates.loop"}];const o={...i,...or(r,n)},s=a.some(c=>c.type==="audio"&&!!i.linkedGroupId&&c.linkedGroupId===i.linkedGroupId),l=Ds(o,s);return l==="unmuted"?[{message:"Chỉ lặp được clip đã tắt tiếng",code:"item.loopNeedsMuted",path:"updates.loop"}]:l==="no-window"?[{message:"Clip cần biết khoảng nguồn để lặp",code:"item.loopNeedsWindow",path:"updates.loop"}]:[]}))}function i2(e,t,r,n){return fl(e).flatMap(a=>a.filter(i=>i.id===t).flatMap(i=>{const o={...i,...or(r,n)};if(o.type!=="video"||o.templateAnchor!=="full")return[];const s=a.some(c=>c.type==="audio"&&!!o.linkedGroupId&&c.linkedGroupId===o.linkedGroupId),l=Ds(o,s);return l==="unmuted"?[{message:"Clip neo toàn video phải tắt tiếng",code:"item.anchorNeedsMuted",path:"updates.templateAnchor"}]:l==="no-window"?[{message:"Clip neo toàn video cần biết khoảng nguồn",code:"item.anchorNeedsWindow",path:"updates.templateAnchor"}]:[]}))}function yu(e,t){if(e.type!=="video"||e.loop!==!0)return{};const r={...e,...t};if(r.loop===!0&&r.embeddedAudioMuted===!0)return{};const n={loop:void 0},a=W.getState().fps,i=e.sourceFps??a,o=e.speed??1,s=e.sourceStart??e.trimStart??e.offset??0,l=e.sourceEnd??e.sourceDuration;if(l!==void 0&&Number.isFinite(l)&&l>s&&r.durationInFrames!==void 0){const c=Math.max(1,Tt(l-s,o,i,a));r.durationInFrames>c&&(n.durationInFrames=c)}return n}function vu(e,t,r,n){let a=!1;for(const i in r)if(hu[i]){a=!0;break}if(!a&&n){for(const i of n)if(hu[i]){a=!0;break}}return a?fl(e).flatMap(i=>i.filter(o=>o.id===t).flatMap(o=>{const s={...o,...or(r,n)},l=e.mediaItems?.find(u=>u.id===s.mediaId),c=sf(s,l?.contentHash);return s.waveformVisualizer&&(!l||!l.mimeType.startsWith("audio/"))?[{message:"Chọn tệp âm thanh làm nguồn giọng",path:"updates.mediaId"}]:c?[{message:c,path:"updates.waveformVisualizer"}]:[]})):[]}function o2(){A({type:"item.add",params:y({item:Or}).strict(),validate:(r,n)=>Oa(r,[n.item],r.tracks),apply(r,n){const[a]=bs([_a(r,n.item)]);return a?(I.getState()._addItem(a),{inverse:j(),result:a}):{inverse:j(),result:null}}}),A({type:"item.addMany",params:y({items:F(Or).min(1)}).strict(),validate:(r,n)=>Oa(r,n.items,r.tracks),apply(r,n){const a=bs(n.items.map(i=>_a(r,i)));return I.getState()._addItems(a),{inverse:j(),result:a}}}),A({type:"item.addWithLinkedAudio",params:y({video:Or.refine(r=>r.type==="video",{message:"Chỉ hỗ trợ clip video"}).refine(r=>r.loop!==!0,{message:"Clip lặp không gắn âm thanh được"})}).strict(),validate:(r,n)=>Oa(r,[n.video],r.tracks),apply(r,n){const a=I.getState(),i=new Map;for(const c of a.items){const u=i.get(c.trackId);u?u.push(c):i.set(c.trackId,[c])}const{updatedVideo:o,audioItem:s,newTrack:l}=vk({video:_a(r,n.video),tracks:a.tracks,itemsByTrackId:i,createId:()=>r.ids.mint()});return l&&a.setTracks([...a.tracks,l]),a._addItems([o,s]),{inverse:j(),result:{video:o,audio:s,track:l}}}}),A({type:"item.addOnNewTracks",params:y({items:F(Or).min(1),tracks:F(Nr).min(1)}).strict(),validate:(r,n)=>Oa(r,n.items,n.tracks),apply(r,n){const a=I.getState(),i=n.items.map(o=>_a(r,o));return a.setTracks(n.tracks),a._addItems(i),{inverse:j(),result:i}}}),A({type:"item.update",params:y({id:se,...Po}).strict(),validate:(r,n)=>[...Ae(r,[n.id],"id"),...Mo(n.updates,n.clearFields),...vu(r,n.id,n.updates,n.clearFields),...gu(r,n.id,n.updates,n.clearFields),...i2(r,n.id,n.updates,n.clearFields),...typeof n.updates.trackId=="string"?tr(r.tracks,[n.updates.trackId],"updates.trackId"):[]],mergeKey:r=>`item.update:${r.id}:${[...Object.keys(r.updates),...r.clearFields??[]].sort().join(",")}`,apply(r,n){const a=I.getState().itemById[n.id],i={...or(n.updates,n.clearFields),...a?yu(a,or(n.updates,n.clearFields)):{}};return I.getState()._updateItem(n.id,i),("from"in i||"durationInFrames"in i||"trackId"in i)&&le([n.id]),j()}}),A({type:"item.move",params:ou,validate:(r,n)=>[...Ae(r,[n.id],"id"),...tr(r.tracks,[n.trackId],"trackId")],mergeKey:r=>`item.move:${r.id}`,apply(r,n){return I.getState()._moveItem(n.id,n.from,n.trackId),le([n.id]),j()}}),A({type:"item.moveMany",params:y({updates:F(ou).min(1),tracks:F(Nr).optional()}).strict(),validate:(r,n)=>[...Ae(r,n.updates.map(a=>a.id),"updates.id"),...Af(n.updates.map(a=>a.id),"updates.id"),...tr(n.tracks??r.tracks,n.updates.map(a=>a.trackId),"updates.trackId")],mergeKey:r=>`item.moveMany:${r.updates.map(n=>n.id).sort().join(",")}${r.tracks?":tracks":""}`,apply(r,n){const a=I.getState();return n.tracks&&a.setTracks(n.tracks),a._moveItems(n.updates),Lk(new Set(n.updates.map(i=>i.id))),le(n.updates.map(i=>i.id)),j()}}),A({type:"item.trackPush",params:y({anchorId:se,delta:M().finite().refine(r=>r!==0,{message:"delta phải khác 0"})}).strict(),validate:(r,n)=>Ae(r,[n.anchorId],"anchorId"),mergeKey:r=>`item.trackPush:${r.anchorId}`,apply(r,n){const a=I.getState().items,i=a.find(s=>s.id===n.anchorId);if(!i)return j();const o=a.filter(s=>s.from>=i.from).map(s=>({id:s.id,from:Math.max(0,s.from+n.delta)}));return o.length===0||(I.getState()._moveItems(o),le(o.map(s=>s.id))),j()}}),A({type:"item.duplicate",params:y({itemIds:Je,positions:F(Fk).min(1),tracks:F(Nr).optional()}).strict(),validate:(r,n)=>[...Ae(r,n.itemIds),...n.itemIds.length===n.positions.length?[]:[{message:"positions phải cùng số lượng với itemIds",path:"positions"}],...tr(n.tracks??r.tracks,n.positions.map(a=>a.trackId),"positions.trackId")],apply(r,n){const a=I.getState();n.tracks&&a.setTracks(n.tracks);const i=a._duplicateItems(n.itemIds,n.positions);return Hk(r,n.itemIds,i),jk(r,n.itemIds,i),{inverse:j(),result:i}}});const e=(r,n)=>{const a=new Set(r.items.map(i=>i.id));return n.some(i=>a.has(i))?[]:[{message:"Không tìm thấy clip nào để xóa",code:"item.notFound",path:"itemIds"}]};A({type:"item.remove",params:y({itemIds:Je,linkedSelection:B()}).strict(),validate:(r,n)=>e(r,n.itemIds),apply(r,n){const a=ur(I.getState().items,n.itemIds,n.linkedSelection);return a.length>0&&uu(a),{inverse:j(),result:a}}}),A({type:"item.removeExact",params:y({itemIds:Je}).strict(),validate:(r,n)=>e(r,n.itemIds),apply(r,n){const a=new Set(I.getState().items.map(o=>o.id)),i=[...new Set(n.itemIds)].filter(o=>a.has(o));return i.length>0&&uu(i),{inverse:j(),result:i}}}),A({type:"item.rippleDelete",params:y({itemIds:Je,linkedSelection:B()}).strict(),validate:(r,n)=>e(r,n.itemIds),apply(r,n){const a=Bk(n.itemIds,n.linkedSelection);return{inverse:j(),result:a}}}),A({type:"item.closeGap",params:y({trackId:se,frame:Kt}).strict(),validate:(r,n)=>tr(r.tracks,[n.trackId],"trackId"),apply(r,n){return zk(n.trackId,n.frame),j()}}),A({type:"item.closeAllGaps",params:y({trackId:se,linkedSelection:B()}).strict(),validate:(r,n)=>tr(r.tracks,[n.trackId],"trackId"),apply(r,n){return Uk(n.trackId,n.linkedSelection),j()}}),A({type:"item.link",params:y({itemIds:Je}).strict(),validate:(r,n)=>{const a=Ae(r,n.itemIds);if(a.length>0)return a;const i=$n(r.items,n.itemIds);return OI(r.items,n.itemIds)&&i.length>=2?[]:[{message:"Không thể liên kết các clip đã chọn",code:"item.cannotLink",path:"itemIds"}]},apply(r,n){const a=I.getState().items,i=$n(a,n.itemIds).map(c=>a.find(u=>u.id===c)).filter(c=>c!==void 0),o=r.ids.mint(),s=I.getState(),l=i.some(c=>c.type==="audio");for(const c of i)s._updateItem(c.id,{linkedGroupId:o,...c.type==="video"&&{embeddedAudioMuted:void 0,...l&&{loop:void 0}}});return{inverse:j(),result:i.map(c=>c.id)}}}),A({type:"item.unlink",params:y({itemIds:Je}).strict(),validate:(r,n)=>Ae(r,n.itemIds),apply(r,n){const a=I.getState().items,i=new Set;for(const c of n.itemIds)for(const u of la(a,c))i.add(u);const o=a.filter(c=>i.has(c.id)&&c.linkedGroupId);if(o.length===0)return{inverse:j(),result:[]};const s=new Set;for(const c of o){if(c.type!=="video")continue;o.some(d=>d.type==="audio"&&d.linkedGroupId===c.linkedGroupId)&&s.add(c.id)}const l=I.getState();for(const c of o)l._updateItem(c.id,{linkedGroupId:c.id,...s.has(c.id)&&{embeddedAudioMuted:!0}});return{inverse:j(),result:o.map(c=>c.id)}}}),A({type:"item.reverse",params:y({itemIds:Je}).strict(),validate:(r,n)=>{const a=Ae(r,n.itemIds);return a.length>0?a:du(r.items,n.itemIds).reversibleItems.length>0?[]:[{message:"Chỉ clip video hoặc âm thanh mới đảo ngược được",code:"item.notReversible",path:"itemIds"}]},apply(r,n){const{reversibleItems:a,shouldReverse:i}=du(I.getState().items,n.itemIds),o=I.getState();for(const s of a){const l={isReversed:i?!0:void 0};if(!i)for(const c of Dk)l[c]=void 0;o._updateItem(s.id,l)}return{inverse:j(),result:{ids:a.map(s=>s.id),reversed:i}}}}),A({type:"item.commitPreparedReverse",params:y({entries:F(y({itemId:se,conform:Ok.optional()}).strict()).min(1)}).strict(),validate:(r,n)=>Ae(r,n.entries.map(a=>a.itemId),"entries.itemId"),apply(r,n){const a=I.getState();for(const i of n.entries){const o=a.itemById[i.itemId];if(!o)continue;const s=i.conform,l=o.type==="video"&&s?{...s.quality==="full"?{reverseConformSrc:s.src,reverseConformPath:s.path,reverseConformKey:s.key}:{reverseConformPreviewSrc:s.src,reverseConformPreviewPath:s.path,reverseConformPreviewKey:s.key,reverseConformPreviewUsesProxy:s.usesProxy,reverseConformPreviewIsSourceLevel:s.isSourceLevel,reverseConformPreviewSourceDuration:s.sourceDuration,reverseConformPreviewFps:s.conformFps},reverseConformStatus:"ready"}:{};a._updateItem(o.id,{isReversed:!0,...l})}return j()}});const t=y({childItemIds:Je,parentItemId:se.optional(),behavior:z(["preserve-world","snap-to-parent","restore-local"]).optional(),frame:Kt,canvas:su}).strict();A({type:"item.setTransformParents",params:t,validate:(r,n)=>{const a=Ae(r,[...n.childItemIds,...n.parentItemId?[n.parentItemId]:[]]);if(a.length>0)return a;const i=r.items.find(o=>o.id===n.parentItemId);return Jk([...new Set(n.childItemIds)],n.parentItemId,i)?[]:[{message:"Yêu cầu gán parent transform không hợp lệ",code:"item.invalidTransformParent",path:"parentItemId"}]},apply(r,n){const a=[...new Set(n.childItemIds)],i=I.getState(),o=n.parentItemId?i.itemById[n.parentItemId]:void 0,{getItem:s,getKeyframes:l}=fu(),c=pu({parentItemId:n.parentItemId,parent:o,behavior:n.behavior,frame:n.frame,canvas:n.canvas,getItem:s,getKeyframes:l}),u=mu(a,c);if(!u||u.size===0)return{inverse:j(),result:!1};for(const[d,m]of u)i._updateItem(d,{transformParent:m});return{inverse:j(),result:!0}}}),A({type:"item.createNullParent",params:y({childItemIds:Je,frame:Kt,canvas:su}).strict(),validate:(r,n)=>{const a=Ae(r,n.childItemIds,"childItemIds");return a.length>0?a:n.childItemIds.every(i=>{const o=r.items.find(s=>s.id===i);return o!==void 0&&pl(o)})?[]:[{message:"Clip này không thể có parent transform",code:"item.invalidTransformParent",path:"childItemIds"}]},apply(r,n){const a=[...new Set(n.childItemIds)],i=I.getState(),o=a.map(p=>i.itemById[p]).filter(p=>p!==void 0);if(o.length!==a.length)return{inverse:j(),result:null};const{parent:s,track:l}=t2(r,{children:o,items:i.items,tracks:i.tracks,canvas:n.canvas}),{getItem:c,getKeyframes:u}=fu(),d=pu({parentItemId:s.id,parent:s,frame:n.frame,canvas:n.canvas,getItem:p=>p===s.id?s:c(p),getKeyframes:u}),m=mu(a,d);if(!m||m.size!==a.length)return{inverse:j(),result:null};i.setTracks([...i.tracks.map(p=>(p.parentTrackId??null)===(l.parentTrackId??null)&&p.order>=l.order?{...p,order:p.order+1}:p),l]),i._addItem(s);for(const[p,f]of m)i._updateItem(p,{transformParent:f});return{inverse:j(),result:s}}}),A({type:"item.updateTransform",params:y({id:se,transform:Fa,operation:Aa.optional(),autoKeyframeOperations:F(Io).optional()}).strict(),validate:(r,n)=>[...cn(r,[n.id],"id"),...ko(r,n.autoKeyframeOperations)],mergeKey:r=>`item.updateTransform:${r.id}:${wo(r.transform)}:${r.autoKeyframeOperations?.length?"kf":""}`,apply(r,n){return Object.keys(n.transform).length>0&&I.getState()._updateItemTransform(n.id,n.transform),xo(n.autoKeyframeOperations??[]),j()}}),A({type:"item.commitMaskEdit",params:y({id:se,pathVertices:F(Ak).optional(),transform:Fa.optional(),operation:Aa.optional(),autoKeyframeOperations:F(Io).optional()}).strict(),validate:(r,n)=>[...cn(r,[n.id],"id"),...ko(r,n.autoKeyframeOperations)],mergeKey:r=>`item.commitMaskEdit:${r.id}:${r.pathVertices?"path":""}:${wo(r.transform??{})}:${r.autoKeyframeOperations?.length?"kf":""}`,apply(r,n){const a=I.getState();return n.pathVertices&&a._updateItem(n.id,{pathVertices:n.pathVertices}),n.transform&&Object.keys(n.transform).length>0&&a._updateItemTransform(n.id,n.transform),xo(n.autoKeyframeOperations??[]),j()}}),A({type:"item.resetTransform",params:y({id:se}).strict(),validate:(r,n)=>cn(r,[n.id],"id"),apply(r,n){return I.getState()._resetItemTransform(n.id),j()}}),A({type:"item.updateTransforms",params:y({ids:Je,transform:Fa,operation:Aa.optional()}).strict(),validate:(r,n)=>cn(r,n.ids,"ids"),mergeKey:r=>`item.updateTransforms:${[...r.ids].sort().join(",")}:${wo(r.transform)}`,apply(r,n){return I.getState()._updateItemsTransform(n.ids,n.transform),j()}}),A({type:"item.updateTransformsMap",params:y({entries:F(y({id:se,transform:Fa}).strict()),itemUpdates:F(y({id:se,...Po}).strict()).optional(),operation:Aa.optional(),autoKeyframeOperations:F(Io).optional()}).strict(),validate:(r,n)=>[...Ae(r,[...n.entries.map(a=>a.id),...(n.itemUpdates??[]).map(a=>a.id)],"entries"),...cn(r,n.entries.map(a=>a.id),"entries"),...(n.itemUpdates??[]).flatMap(a=>Mo(a.updates,a.clearFields)),...ko(r,n.autoKeyframeOperations)],mergeKey:r=>`item.updateTransformsMap:${[...r.entries.map(n=>n.id),...(r.itemUpdates??[]).map(n=>`u:${n.id}`)].sort().join(",")}:${r.autoKeyframeOperations?.length?"kf":""}`,apply(r,n){const a=new Map(n.entries.map(o=>[o.id,o.transform])),i=n.itemUpdates?.length?new Map(n.itemUpdates.map(o=>[o.id,or(o.updates,o.clearFields)])):void 0;return I.getState()._updateItemsTransformMap(a,i),xo(n.autoKeyframeOperations??[]),j()}}),A({type:"item.applyBentoLayout",params:y({itemIds:F(se).min(2),canvasWidth:M().finite().positive(),canvasHeight:M().finite().positive(),config:_k.optional(),orderedChains:F(F(se)).optional()}).strict(),validate:(r,n)=>[...Ae(r,n.itemIds),...n.itemIds.filter(a=>r.items.find(i=>i.id===a)?.type!=="audio").length>=2?[]:[{message:"Cần ít nhất 2 clip hình ảnh để xếp bố cục",code:"item.bentoNeedsVisuals",path:"itemIds"}]],apply(r,n){const a=I.getState().items,i=n.itemIds.filter(m=>{const p=a.find(f=>f.id===m);return p!=null&&p.type!=="audio"});if(i.length<2)return j();let o;if(n.orderedChains&&n.orderedChains.length>0){const m=new Set(i);o=n.orderedChains.map(p=>p.filter(f=>m.has(f))).filter(p=>p.length>0)}else{const{transitionsByClipId:m}=Sk(q.getState().transitions);o=Ik(i,m)}const s=o.map(m=>{const p=a.find(g=>g.id===m[0]),f=p&&"sourceWidth"in p&&p.sourceWidth||n.canvasWidth,h=p&&"sourceHeight"in p&&p.sourceHeight||n.canvasHeight;return{id:m[0],sourceWidth:f,sourceHeight:h}}),l=n.config??{preset:"auto"},c=Ck(s,n.canvasWidth,n.canvasHeight,l),u=new Map;for(const m of o){const p=c.get(m[0]);if(p)for(const f of m)u.set(f,p)}const d=H.getState();for(const m of i)for(const p of r2)d._removeKeyframesForProperty(m,p);return I.getState()._updateItemsTransformMap(u),j()}}),A({type:"item.updateProjectItem",params:y({itemId:se,...Po}).strict(),validate:(r,n)=>[...Mo(n.updates,n.clearFields),...vu(r,n.itemId,n.updates,n.clearFields),...gu(r,n.itemId,n.updates,n.clearFields)],apply(r,n){const a=or(n.updates,n.clearFields),i=I.getState().itemById[n.itemId],o=a2(n.itemId,{...a,...i?yu(i,a):{}});return{inverse:j(),result:o}}}),A({type:"item.removeProjectItems",params:y({itemIds:Je}).strict(),apply(r,n){const a=n2([...new Set(n.itemIds)]);return{inverse:j(),result:a}}})}const Ei=M().finite(),at=B().optional(),Q=Ei.optional(),La=dr([Z(6),Z(12),Z(18),Z(24)]).optional(),Ba=z(["low-shelf","peaking","high-shelf","notch"]).optional(),qr=y({enabled:at,outputGainDb:Q,band1Enabled:at,band1Type:z(["low-shelf","peaking","high-shelf","high-pass"]).optional(),band1FrequencyHz:Q,band1GainDb:Q,band1Q:Q,band1SlopeDbPerOct:La,lowCutEnabled:at,lowCutFrequencyHz:Q,lowCutSlopeDbPerOct:La,lowEnabled:at,lowType:Ba,lowGainDb:Q,lowFrequencyHz:Q,lowQ:Q,lowMidEnabled:at,lowMidType:Ba,lowMidGainDb:Q,lowMidFrequencyHz:Q,lowMidQ:Q,midGainDb:Q,highMidEnabled:at,highMidType:Ba,highMidGainDb:Q,highMidFrequencyHz:Q,highMidQ:Q,highEnabled:at,highType:Ba,highGainDb:Q,highFrequencyHz:Q,highQ:Q,band6Enabled:at,band6Type:z(["low-pass","low-shelf","peaking","high-shelf"]).optional(),band6FrequencyHz:Q,band6GainDb:Q,band6Q:Q,band6SlopeDbPerOct:La,highCutEnabled:at,highCutFrequencyHz:Q,highCutSlopeDbPerOct:La}).strict(),s2=Object.fromEntries(Object.entries(qr.shape).map(([e,t])=>[`audioEq${e[0].toUpperCase()}${e.slice(1)}`,t])),l2=y({...s2,volume:Q,audioFadeIn:Q,audioFadeOut:Q,audioFadeInCurve:Q,audioFadeOutCurve:Q,audioFadeInCurveX:Q,audioFadeOutCurveX:Q,audioPitchSemitones:Q,audioPitchCents:Q,embeddedAudioMuted:at,audioDucking:y({duckOthersDb:Ei,attackSec:Q,releaseSec:Q,targetTrackIds:F(w().min(1)).optional()}).strict().nullable().optional()}).strict(),xr=w().min(1),Ot=()=>({kind:"patch",patch:{}});function bu(e,t){const r=e.items.find(n=>n.id===t.itemId);return r?r.type!=="audio"&&r.type!=="video"&&r.type!=="composition"?[{message:"Clip này không có âm thanh",path:"itemId"}]:[]:[{message:`Không tìm thấy clip ${t.itemId}`,path:"itemId"}]}function za(e,t){return e.tracks.some(r=>r.id===t.trackId)?[]:[{message:`Không tìm thấy rãnh ${t.trackId}`,path:"trackId"}]}function c2(e){return Object.fromEntries(Object.keys(qr.shape).map(t=>[`audioEq${t[0].toUpperCase()}${t.slice(1)}`,e?.[t]]))}function u2(){A({type:"audio.setClip",params:y({itemId:xr,updates:l2}).strict(),validate:(e,t)=>bu(e,t),mergeKey:e=>`audio:clip:${e.itemId}:${Object.keys(e.updates).sort().join(",")}`,apply(e,t){const r={...t.updates};return"audioDucking"in t.updates&&(r.audioDucking=t.updates.audioDucking??void 0),r.embeddedAudioMuted===!1&&(r.loop=void 0),e.draft.items=e.draft.items.map(n=>n.id===t.itemId?fe({...n,...r}):n),Ot()}}),A({type:"audio.setClipEq",params:y({itemId:xr,eq:qr.nullable()}).strict(),validate:(e,t)=>bu(e,t),mergeKey:e=>`audio:clip-eq:${e.itemId}`,apply(e,t){const r=c2(t.eq);return e.draft.items=e.draft.items.map(n=>n.id===t.itemId?fe({...n,...r}):n),Ot()}}),A({type:"audio.setTrackVolume",params:y({trackId:xr,volume:Ei}).strict(),validate:(e,t)=>za(e,t),mergeKey:e=>`audio:track-volume:${e.trackId}`,apply(e,t){return I.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,volume:t.volume}:r)),Ot()}}),A({type:"audio.setTrackMute",params:y({trackId:xr,muted:B()}).strict(),validate:(e,t)=>za(e,t),apply(e,t){return I.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,muted:t.muted}:r)),Ot()}}),A({type:"audio.setTrackSolo",params:y({trackId:xr,solo:B()}).strict(),validate:(e,t)=>za(e,t),apply(e,t){return I.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,solo:t.solo}:r)),Ot()}}),A({type:"audio.setTrackEq",params:y({trackId:xr,eq:qr.nullable()}).strict(),validate:(e,t)=>za(e,t),mergeKey:e=>`audio:track-eq:${e.trackId}`,apply(e,t){return I.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,audioEq:t.eq??void 0}:r)),Ot()}}),A({type:"audio.setBusEq",params:y({eq:qr.nullable()}).strict(),mergeKey:()=>"audio:bus-eq",apply(e,t){return e.draft.busAudioEq=t.eq??void 0,Ot()}}),A({type:"audio.setMasterGain",params:y({db:Ei}).strict(),mergeKey:()=>"audio:master-gain",apply(e,t){return e.draft.masterBusDb=Math.max(-60,Math.min(12,t.db)),Ot()}})}const zt=w().min(1),d2=y({id:zt,frame:M().int().nonnegative(),color:w(),label:w().optional()}),Rf=Zn({id:zt,leftClipId:zt,rightClipId:zt,trackId:zt,durationInFrames:M().positive()}),Lf=Zn({itemId:zt,properties:F(_i()).optional()}),m2=Zn({id:zt,name:w(),items:F(Or),tracks:F(Nr),transitions:F(Rf),keyframes:F(Lf),fps:M().positive(),width:M().positive(),height:M().positive(),durationInFrames:M().nonnegative()}),p2=y({items:F(Or).optional(),tracks:F(Nr).optional(),transitions:F(Rf).optional(),keyframes:F(Lf).optional(),markers:F(d2).optional(),compositions:F(m2).optional(),topLevelSequenceIds:F(zt).optional(),inPoint:M().nonnegative().nullable().optional(),outPoint:M().nonnegative().nullable().optional(),fps:M().positive().max(240).optional(),scrollPosition:M().nonnegative().optional(),snapEnabled:B().optional(),currentFrame:M().int().nonnegative().optional(),busAudioEq:qr.optional(),masterBusDb:M().min(-60).max(12).optional()}).strict();function f2(){A({type:"project.setView",history:"none",dirty:!1,params:y({scrollPosition:M().nonnegative()}).strict(),apply({draft:e},t){return e.scrollPosition=t.scrollPosition,{kind:"patch",patch:{}}}}),A({type:"project.clearTimeline",history:"none",dirty:!1,params:y({}).strict(),apply({draft:e}){return e.items=[],e.tracks=[],e.transitions=[],e.keyframes=[],e.markers=[],e.inPoint=null,e.outPoint=null,{kind:"patch",patch:{}}}}),A({type:"project.rename",params:y({name:w().trim().min(1)}).strict(),apply({draft:e},t){return e.projectName=t.name,{kind:"patch",patch:{}}}}),A({type:"project.updateSettings",params:y({width:M().int().positive().optional(),height:M().int().positive().optional(),fps:M().positive().max(240).optional(),backgroundColor:w().optional()}).strict(),apply({draft:e},t){const r=e.projectMetadata,n={width:r?.width??1920,height:r?.height??1080,fps:r?.fps??e.fps,...r,...t};return e.projectMetadata=n,t.fps!==void 0&&(e.fps=t.fps),{kind:"patch",patch:{}}}}),A({type:"project.setTimelineSettings",params:y({snapEnabled:B().optional(),scrollPosition:M().nonnegative().optional()}).strict(),apply({draft:e},t){return t.snapEnabled!==void 0&&(e.snapEnabled=t.snapEnabled),t.scrollPosition!==void 0&&(e.scrollPosition=t.scrollPosition),{kind:"patch",patch:{}}}}),A({type:"project.patch",params:y({patch:p2}).strict(),apply({draft:e},{patch:t}){for(const[r,n]of Object.entries(t))Reflect.set(e,r,n);return{kind:"patch",patch:{}}}})}const Su=y({name:w().optional(),height:M().positive().optional(),locked:B().optional(),visible:B().optional(),muted:B().optional(),solo:B().optional(),syncLock:B().optional(),parentTrackId:w().nullable().optional(),isGroup:B().optional(),isCollapsed:B().optional(),kind:z(["video","audio"]).optional(),order:M().optional(),templateSlot:Ff.nullable().optional(),templateLoudnessLufs:M().min(-24).max(-9).nullable().optional(),templateScenes:nf.nullable().optional(),templateBackdrop:af.nullable().optional(),templateBackgroundAppearance:of.nullable().optional()}).strict(),Iu={scenes:"Ảnh cảnh",captions:"Phụ đề","source-audio":"Âm thanh gốc","background-video":"Nền video"};function Co(e,t,r,n,a="updates.templateSlot"){const i=[];(t==="source-audio"?r!=="audio":r==="audio")&&i.push({code:"track.slotKind",message:t==="source-audio"?"Vai trò Âm thanh gốc chỉ gán cho track âm thanh":`Vai trò ${Iu[t]} chỉ gán cho track video`,path:a});const o=e.find(s=>s.templateSlot===t&&s.id!==n);return o&&i.push({code:"track.slotTaken",message:`Vai trò ${Iu[t]} đã thuộc track "${o.name??o.id}"`,path:a}),i}function To(e,t){return e==="scenes"?[]:[{code:"track.scenesOnly",message:"Công thức ảnh chỉ đặt trên track vai trò Ảnh cảnh",path:t}]}function Eo(e,t,r){const n=[];e!=="background-video"&&n.push({code:"track.backdropOnly",message:"Kênh nền chỉ đặt trên track vai trò Nền video",path:r});const a=t.channels.map(i=>i.channelId);return new Set(a).size!==a.length&&n.push({code:"track.backdropDuplicate",message:"Một kênh nền chỉ được thêm một lần",path:r}),n}function h2(){A({type:"track.add",params:Su.extend({id:w().optional(),beforeTrackId:w().nullable().optional()}).strict(),validate:(e,t)=>[...t.templateSlot==null?[]:Co(e.tracks,t.templateSlot,t.kind,t.id,"templateSlot"),...t.templateScenes?To(t.templateSlot??void 0,"templateScenes"):[],...t.templateBackdrop?Eo(t.templateSlot??void 0,t.templateBackdrop,"templateBackdrop"):[],...t.templateBackgroundAppearance&&t.templateSlot!=="background-video"?[{code:"track.backgroundAppearanceOnly",message:"Cài đặt nền chỉ đặt trên track vai trò Nền video",path:"templateBackgroundAppearance"}]:[]],apply({draft:e,ids:t},r){const{id:n,beforeTrackId:a,parentTrackId:i,templateSlot:o,templateLoudnessLufs:s,templateScenes:l,templateBackdrop:c,templateBackgroundAppearance:u,...d}=r,m={...d,id:n??t.mint(),name:d.name??"Track mới",height:d.height??lm(),locked:d.locked??!1,visible:d.visible??!0,muted:d.muted??!1,solo:d.solo??!1,order:d.order??0,items:[],...i?{parentTrackId:i}:{},...o?{templateSlot:o}:{},...s!=null?{templateLoudnessLufs:s}:{},...l?{templateScenes:l}:{},...c?{templateBackdrop:c}:{},...u?{templateBackgroundAppearance:u}:{}};if(e.tracks.some(g=>g.id===m.id))throw new Error(`Duplicate track: ${m.id}`);const p=a?e.tracks.findIndex(g=>g.id===a):0,f=Math.max(0,p),h=[...e.tracks];return h.splice(f,0,m),e.tracks=h.map((g,v)=>({...g,order:v})),{inverse:{kind:"patch",patch:{}},result:m}}}),A({type:"track.remove",params:y({ids:F(w()).min(1)}).strict(),apply({draft:e},t){const r=new Set(t.ids);let n=!0;for(;n;){n=!1;for(const i of e.tracks)i.parentTrackId&&r.has(i.parentTrackId)&&!r.has(i.id)&&(r.add(i.id),n=!0)}const a=new Set(e.items.filter(i=>r.has(i.trackId)).map(i=>i.id));return e.items=e.items.filter(i=>!a.has(i.id)),e.transitions=e.transitions.filter(i=>!a.has(i.leftClipId)&&!a.has(i.rightClipId)),e.keyframes=e.keyframes.filter(i=>!a.has(i.itemId)),e.tracks=e.tracks.filter(i=>!r.has(i.id)),{kind:"patch",patch:{}}}}),A({type:"track.update",params:y({id:w(),updates:Su}).strict(),validate:(e,t)=>{const r=e.tracks.find(c=>c.id===t.id);if(!r)return[{message:`Unknown track: ${t.id}`}];const n=t.updates.templateSlot===null?void 0:t.updates.templateSlot??r.templateSlot,a=t.updates.kind??r.kind,i=n?Co(e.tracks,n,a,t.id):[],o=t.updates.templateScenes===null?void 0:t.updates.templateScenes??(n==="scenes"?r.templateScenes:void 0);i.push(...o?To(n,"updates.templateScenes"):[]);const s=t.updates.templateBackdrop===null?void 0:t.updates.templateBackdrop??(n==="background-video"?r.templateBackdrop:void 0);return i.push(...s?Eo(n,s,"updates.templateBackdrop"):[]),t.updates.templateBackgroundAppearance!==null&&(t.updates.templateBackgroundAppearance??(n==="background-video"?r.templateBackgroundAppearance:void 0))&&n!=="background-video"&&i.push({code:"track.backgroundAppearanceOnly",message:"Cài đặt nền chỉ đặt trên track vai trò Nền video",path:"updates.templateBackgroundAppearance"}),i},mergeKey:e=>`track.update:${e.id}`,apply({draft:e},t){if(!e.tracks.some(n=>n.id===t.id))throw new Error(`Unknown track: ${t.id}`);if(t.updates.parentTrackId){let n=e.tracks.find(a=>a.id===t.updates.parentTrackId);if(!n?.isGroup)throw new Error("Parent track must be a group");for(;n;){if(n.id===t.id)throw new Error("Circular track group");n=e.tracks.find(a=>a.id===n?.parentTrackId)}}const r=t.updates.templateSlot;return e.tracks=e.tracks.map(n=>n.id===t.id?{...n,...t.updates,templateSlot:r===null?void 0:r??n.templateSlot,templateLoudnessLufs:t.updates.templateLoudnessLufs===null?void 0:t.updates.templateLoudnessLufs??n.templateLoudnessLufs,templateScenes:t.updates.templateScenes===null||r!==void 0&&r!=="scenes"?void 0:t.updates.templateScenes??n.templateScenes,templateBackdrop:t.updates.templateBackdrop===null||r!==void 0&&r!=="background-video"?void 0:t.updates.templateBackdrop??n.templateBackdrop,templateBackgroundAppearance:t.updates.templateBackgroundAppearance===null||r!==void 0&&r!=="background-video"?void 0:t.updates.templateBackgroundAppearance??n.templateBackgroundAppearance,parentTrackId:t.updates.parentTrackId===null?void 0:t.updates.parentTrackId??n.parentTrackId}:n),{kind:"patch",patch:{}}}}),A({type:"track.reorder",params:y({ids:F(w())}).strict(),apply({draft:e},t){if(new Set(t.ids).size!==e.tracks.length||t.ids.length!==e.tracks.length)throw new Error("Track order must contain every track once");return e.tracks=t.ids.map((r,n)=>{const a=e.tracks.find(i=>i.id===r);if(!a)throw new Error(`Unknown track: ${r}`);return{...a,order:n}}),{kind:"patch",patch:{}}}}),A({type:"track.set",params:y({tracks:F(Nr)}).strict(),validate:(e,t)=>t.tracks.flatMap(r=>[...r.templateSlot?Co(t.tracks,r.templateSlot,r.kind,r.id,"tracks.templateSlot"):[],...r.templateScenes?To(r.templateSlot,"tracks.templateScenes"):[],...r.templateBackdrop?Eo(r.templateSlot,r.templateBackdrop,"tracks.templateBackdrop"):[]]),apply({draft:e},t){return e.tracks=t.tracks.map(r=>({...r,items:[]})),{kind:"patch",patch:{}}}})}function g2(e){const t=rm(e),r=t.some(n=>n.solo);return t.filter(n=>r?n.solo===!0:n.visible!==!1)}function y2(e){const t=g2(e.tracks),r=new Set(t.map(a=>a.id)),n=new Map(t.map(a=>[a.id,a.order??0]));return e.items.filter(a=>r.has(a.trackId)).toSorted((a,i)=>{const o=n.get(a.trackId)??0,s=n.get(i.trackId)??0;return o!==s?o-s:a.from!==i.from?a.from-i.from:a.id.localeCompare(i.id)})}function Fi(e){return e.type==="video"||e.type==="audio"?e.sourceStart??e.trimStart??e.offset??0:e.type==="composition"?e.sourceStart??e.trimStart??0:0}function v2(e){const{subItem:t,wrapper:r,parentFps:n,subCompFps:a}=e,i=Fi(r),o=Hd({itemStart:t.from,itemDuration:t.durationInFrames,wrapperDuration:r.durationInFrames,wrapperSpeed:r.speed,wrapperSourceFps:r.sourceFps,wrapperSourceStart:i,wrapperSourceEnd:r.sourceEnd,timelineFps:n,fallbackSourceFps:a});if(!o)return null;const s={...t,from:o.mappedFrom,durationInFrames:o.mappedDuration,speed:(t.speed??1)*o.wrapperSpeed};if(t.type==="video"||t.type==="audio"||t.type==="composition"){const l=t.sourceFps??a,c=t.speed??1,u=Fi(t)+we(o.clippedStartFrames,c,a,l);s.sourceStart=u,s.sourceFps=l,t.sourceEnd!==void 0&&(s.sourceEnd=Math.max(u+1,t.sourceEnd-we(o.clippedEndFrames,c,a,l)))}return s}function xu(e){const{wrapper:t,parentFps:r,compositionById:n,activeCompositionPath:a,mediaFpsById:i}=e;if(!n)return[];if(a.has(t.compositionId))return[];const o=n[t.compositionId];if(!o)return[];const s=new Set(a);s.add(t.compositionId);const l=o.items.flatMap(c=>{const u=v2({subItem:c,wrapper:t,parentFps:r,subCompFps:o.fps});return u?[u]:[]});return Bf({items:l,tracks:o.tracks,fps:r,mediaFpsById:i,compositionById:n,activeCompositionPath:s})}function Bf(e){const t=y2(e),r=new Map(rm(e.tracks).map(a=>[a.id,a])),n=e.activeCompositionPath??new Set;return t.flatMap(a=>r.get(a.trackId)?.muted?[]:a.type==="audio"&&a.compositionId?xu({wrapper:a,parentFps:e.fps,compositionById:e.compositionById,activeCompositionPath:n,mediaFpsById:e.mediaFpsById}):a.type==="composition"?ts(t,a)?[]:xu({wrapper:a,parentFps:e.fps,compositionById:e.compositionById,activeCompositionPath:n,mediaFpsById:e.mediaFpsById}):a.mediaId?a.type==="audio"?[{itemId:a.id,mediaId:a.mediaId,from:a.from,durationInFrames:a.durationInFrames,sourceStart:Fi(a),sourceFps:a.sourceFps??e.mediaFpsById?.[a.mediaId]??e.fps,speed:a.speed??1}]:a.type==="video"&&!a.embeddedAudioMuted&&!ts(t,a)?[{itemId:a.id,mediaId:a.mediaId,from:a.from,durationInFrames:a.durationInFrames,sourceStart:Fi(a),sourceFps:a.sourceFps??e.mediaFpsById?.[a.mediaId]??e.fps,speed:a.speed??1}]:[]:[])}function Ai(e,t){return Wt(e??{name:""})??(t.every(r=>r.type==="audio")?"audio":"video")}function b2(e){return e.some(t=>t.type!=="audio")}function zf(e){return{sourceStart:0,sourceEnd:e.durationInFrames,sourceDuration:e.durationInFrames,sourceFps:e.fps,speed:1}}function S2(e,t){const r=new Set(t);for(const n of e)if(r.has(n.id)){if(n.type==="composition"){const a=Rs(e,n);a&&r.add(a.id);continue}if(Wr(n)){const a=qd(e,n);a&&r.add(a.id)}}return Array.from(r)}function Ua(e,t,r){const n=new Set([t]);let a=!1;const i=e.map(o=>!Uf(o,n)||o.label===r?o:(a=!0,{...o,label:r}));return a?i:e}function Uf(e,t){return!!e.compositionId&&(e.type==="composition"||Wr(e))&&t.has(e.compositionId)}function wu(e,t){return e.filter(r=>Uf(r,t)).length}function I2(e){const t=new Set(e.filter(Boolean));if(t.size===0)return{rootReferenceCount:0,nestedReferenceCount:0,totalReferenceCount:0};const r=Ni(),n=wu(ek(r).items,t),a=tk(r).filter(i=>!t.has(i.id)).reduce((i,o)=>i+wu(o.items,t),0);return{rootReferenceCount:n,nestedReferenceCount:a,totalReferenceCount:n+a}}function x2(e,t){const r=t.trim();return!e||!r?!1:ml("RENAME_COMPOUND_CLIP",()=>{const n=ae.getState().getComposition(e);if(!n||n.name===r)return!1;ae.getState().updateComposition(e,{name:r});const a=Ni(),i=Ua(a.items,e,r);i!==a.items&&I.getState().setItems(i);const o=de.getState().activeCompositionId,s=ae.getState().compositions.map(l=>{if(l.id===e)return l;const c=l.id===o?i:l.items,u=Ua(c,e,r);return u===c?l:{...l,items:u}});return ae.getState().setCompositions(s),de.setState(l=>({breadcrumbs:l.breadcrumbs.map(c=>c.compositionId===e?{...c,label:r}:c),stashStack:l.stashStack.map(c=>({...c,items:Ua(c.items,e,r)})),mainHolder:l.mainHolder?{...l.mainHolder,items:Ua(l.mainHolder.items,e,r)}:null})),!0},{compositionId:e,nextName:r})}function ku(e){const{subTracks:t,subItems:r,anchorTrackId:n,kind:a,compFrom:i,existingTracks:o,currentItems:s,trackIdMapping:l,newTracks:c}=e;if(t.length===0||!n)return;const u=[...t].sort((h,g)=>(h.order??0)-(g.order??0)),d=[...o,...c].find(h=>h.id===n);if(!d)return;const m=new Set(l.values()),p=u.length-1;l.set(u[p].id,n),m.add(n);const f=[...o,...c].filter(h=>h.id!==n&&Wt(h)===a).filter(h=>(h.order??0)<(d.order??0)).sort((h,g)=>(g.order??0)-(h.order??0));for(let h=p-1;h>=0;h-=1){const g=u[h],v=r.filter(T=>T.trackId===g.id).map(T=>({from:T.from+i,end:T.from+i+T.durationInFrames}));let S=null;for(const T of f){if(m.has(T.id))continue;if(!s.filter(_=>_.trackId===T.id).some(_=>{const P=_.from+_.durationInFrames;return v.some(R=>R.from<P&&_.from<R.end)})){S=T.id,m.add(T.id);break}}if(S){l.set(g.id,S);continue}const b=p-h,x=$();l.set(g.id,x),m.add(x),c.push({...g,id:x,kind:a,order:(d.order??0)-b*.01})}}function w2(e){const{subItem:t,wrapper:r,timelineFps:n,subCompFps:a}=e,i=r.sourceStart??r.trimStart??0,o=Hd({itemStart:t.from,itemDuration:t.durationInFrames,wrapperDuration:r.durationInFrames,wrapperSpeed:r.speed,wrapperSourceFps:r.sourceFps,wrapperSourceStart:i,wrapperSourceEnd:r.sourceEnd,timelineFps:n,fallbackSourceFps:a});if(!o)return null;const s=r.from+o.mappedFrom,l={...t,from:s,durationInFrames:o.mappedDuration,speed:(t.speed??1)*o.wrapperSpeed};if(t.type==="video"||t.type==="audio"||t.type==="composition"){const c=t.sourceFps??a,u=t.speed??1,d=(t.sourceStart??0)+we(o.clippedStartFrames,u,a,c);l.sourceStart=d,t.sourceEnd!==void 0&&(l.sourceEnd=Math.max(d+1,t.sourceEnd-we(o.clippedEndFrames,u,a,c)))}return l}function k2(e,t,r){const n=r?$n(e,t):Array.from(new Set(t));return S2(e,n)}function P2(e,t,r){return t===null?!1:Ls(e).some(n=>jd({parentCompositionId:t,insertedCompositionId:n,compositionById:r}))}function M2(e){const{items:t,tracks:r}=I.getState(),n=q.getState().transitions,a=H.getState().keyframes,i=W.getState().fps,o=e??rt.getState().selectedItemIds,s=k2(t,o,zs.getState().linkedSelectionEnabled);if(s.length===0)return null;const l=new Set(s),c=t.filter(m=>l.has(m.id));if(c.length===0)return null;const u=de.getState().activeCompositionId,d=ae.getState().compositionById;return P2(c,u,d)?null:{items:t,tracks:r,transitions:n,keyframes:a,fps:i,selectedIds:s,selectedItems:c,activeCompositionId:u,compositionById:d}}function C2(e){const t=Math.min(...e.map(n=>n.from)),r=Math.max(...e.map(n=>n.from+n.durationInFrames));return{minFrom:t,durationInFrames:r-t}}function T2(){const e=Pt.getState().currentProject?.metadata;return{width:e?.width??ad,height:e?.height??id,backgroundColor:e?.backgroundColor}}function E2(e,t,r,n){const a=r??{name:`Track ${t+1}`,kind:Ai(void 0,n),height:yr,visible:!0,muted:!1,solo:!1,volume:0};return{id:$(),name:a.name,kind:Ai(r,n),height:a.height,locked:!1,visible:a.visible,muted:a.muted,solo:a.solo,volume:a.volume,color:a.color,order:t,items:[]}}function F2(e,t){const r=new Map(t.map(o=>[o.id,o])),n=[...new Set(e.map(o=>o.trackId))].sort((o,s)=>(r.get(o)?.order??0)-(r.get(s)?.order??0)),a=n.map((o,s)=>E2(o,s,r.get(o),e.filter(l=>l.trackId===o))),i=new Map;return n.forEach((o,s)=>{i.set(o,a[s].id)}),{sourceTrackIds:n,subCompTracks:a,trackIdMapping:i}}function A2(e,t,r,n){const a=e.map(o=>({...o,id:$(),from:o.from-t,trackId:r.get(o.trackId)??n})),i=new Map;return e.forEach((o,s)=>i.set(o.id,a[s].id)),{items:a,itemIdMapping:i}}function _2(e,t,r,n){return e.filter(a=>t.has(a.leftClipId)&&t.has(a.rightClipId)).map(a=>({...a,id:$(),leftClipId:r.get(a.leftClipId)??a.leftClipId,rightClipId:r.get(a.rightClipId)??a.rightClipId,trackId:n.get(a.trackId)??a.trackId}))}function O2(e,t,r){return e.filter(n=>t.has(n.itemId)).map(n=>({...n,itemId:r.get(n.itemId)??n.itemId}))}function D2(e,t){return e.editorKind?e.editorKind:t===null?"sequence":ae.getState().getComposition(t)?.editorKind==="composite-2d"?"composite-2d":"sequence"}function R2(e,t,r){const{minFrom:n,durationInFrames:a}=C2(e.selectedItems),i=T2(),{sourceTrackIds:o,subCompTracks:s,trackIdMapping:l}=F2(e.selectedItems,e.tracks),{items:c,itemIdMapping:u}=A2(e.selectedItems,n,l,s[0].id),d=new Set(e.selectedIds);return{composition:{id:$(),name:t??`Compound Clip ${ae.getState().compositions.length+1}`,editorKind:D2(r,e.activeCompositionId),items:c,tracks:s,transitions:_2(e.transitions,d,u,l),keyframes:O2(e.keyframes,d,u),fps:e.fps,width:i.width,height:i.height,durationInFrames:a,backgroundColor:i.backgroundColor},minFrom:n,durationInFrames:a,selectedItemIds:d,sourceTrackIds:o}}function Pu(e,t,r){return e.filter(n=>t.some(a=>r==="audio"?a.trackId===n&&a.type==="audio":a.trackId===n&&a.type!=="audio"))}function L2(e,t){return t?e.find(r=>r.id===t)??null:null}function B2(e,t){return t?em({tracks:e,targetTrack:t,kind:"audio",direction:"below"}):e.filter(r=>!r.isGroup).filter(r=>Wt(r)==="audio").sort((r,n)=>(r.order??0)-(n.order??0)).at(-1)??null}function z2(e,t){const r=t??e.at(-1)??null,n=r?Jd(e,r,"below"):0;return Zd({tracks:e,kind:"audio",order:n,height:r?.height??yr})}function U2(e){if(!e.hasOwnedAudio)return{tracks:e.tracks,audioTargetTrackId:null};const t=e.audioSourceTrackIds.at(-1)??null;if(t)return{tracks:e.tracks,audioTargetTrackId:t};const r=L2(e.tracks,e.visualTargetTrackId),n=B2(e.tracks,r);if(n)return{tracks:e.tracks,audioTargetTrackId:n.id};const a=z2(e.tracks,r);return{tracks:[...e.tracks,a],audioTargetTrackId:a.id}}function H2(e,t){const r=b2(t.composition.items),n=Bf({items:t.composition.items,tracks:t.composition.tracks,fps:e.fps,compositionById:e.compositionById}).length>0,a=Pu(t.sourceTrackIds,e.selectedItems,"visual"),i=Pu(t.sourceTrackIds,e.selectedItems,"audio"),o=r?a.at(-1)??null:null,s=U2({tracks:e.tracks,hasOwnedAudio:n,audioSourceTrackIds:i,visualTargetTrackId:o});return{hasVisualWrapper:r,hasOwnedAudio:n,visualTargetTrackId:o,audioTargetTrackId:s.audioTargetTrackId,tracks:s.tracks}}function N2(e,t){I.getState()._removeItems(e.selectedIds),q.getState().setTransitions(e.transitions.filter(r=>!t.selectedItemIds.has(r.leftClipId)&&!t.selectedItemIds.has(r.rightClipId))),H.getState()._removeKeyframesForItems(e.selectedIds)}function q2(e,t,r,n){return!t||!r?null:{id:$(),type:"composition",trackId:r,from:e.minFrom,durationInFrames:e.durationInFrames,label:e.composition.name,compositionId:e.composition.id,linkedGroupId:n,compositionWidth:e.composition.width,compositionHeight:e.composition.height,transform:{x:0,y:0,rotation:0,opacity:1},...zf(e.composition)}}function G2(e,t,r,n){return!t||!r?null:{id:$(),type:"audio",trackId:r,from:e.minFrom,durationInFrames:e.durationInFrames,label:e.composition.name,compositionId:e.composition.id,linkedGroupId:n,src:"",...zf(e.composition)}}function K2(e){return e.hasVisualWrapper&&e.hasOwnedAudio?$():void 0}function V2(e,t){const r=[e,t].filter(n=>n!==null);for(const n of r)I.getState()._addItem(n);return r.length>0&&rt.getState().selectItems(r.map(n=>n.id)),r[0]??null}function W2(e,t){const r=K2(t),n=q2(e,t.hasVisualWrapper,t.visualTargetTrackId,r),a=G2(e,t.hasOwnedAudio,t.audioTargetTrackId,r);return V2(n,a)}function j2(e,t,r){const n=M2(t);if(!n)return null;const a=R2(n,e,r);ae.getState().addComposition(a.composition);const i=H2(n,a);return N2(n,a),i.tracks!==n.tracks&&I.getState().setTracks(i.tracks),W2(a,i)}function $2(e,t,r={}){const n=ml("CREATE_PRE_COMP",()=>j2(e,t,r),{name:e,editorKind:r.editorKind});return r.openAfterCreate&&n?.compositionId&&X2(n.compositionId,n.label,n.id),n}function Y2(e){return ml("DISSOLVE_PRE_COMP",()=>{const{items:t}=I.getState(),r=t.find(E=>E.id===e&&(E.type==="composition"||Wr(E)));if(!r)return!1;const n=r.compositionId;if(!n)return!1;const a=Wr(r),i=r.type==="composition"?r:a?qd(t,r):null,o=r.type==="composition"?Rs(t,r):a?r:null,s=ae.getState().getComposition(n);if(!s)return!1;const{tracks:l}=I.getState(),c=[i?.id,o?.id].filter(E=>!!E),u=i??o;if(!u)return!1;const d=u.from,m=W.getState().fps;let p=l;const f=(E,D,L,U)=>{if(E)return E;if(!D)return null;const V=p.find(ze=>ze.id===D)??null;if(!V)return null;const ne=em({tracks:p,targetTrack:V,kind:L,direction:U});if(ne)return ne.id;const ee=Zd({tracks:p,kind:L,order:Jd(p,V,U),height:V.height??yr});return p=[...p,ee],ee.id},h=[...s.tracks].sort((E,D)=>(E.order??0)-(D.order??0)),g=h.filter(E=>{const D=s.items.filter(L=>L.trackId===E.id);return Ai(E,D)==="video"}),v=h.filter(E=>{const D=s.items.filter(L=>L.trackId===E.id);return Ai(E,D)==="audio"}),S=f(i?.trackId??null,o?.trackId??null,"video","above"),b=f(o?.trackId??null,i?.trackId??null,"audio","below"),x=new Map,T=[],C=t.filter(E=>!c.includes(E.id));ku({subTracks:g,subItems:s.items,anchorTrackId:S,kind:"video",compFrom:d,existingTracks:p,currentItems:C,trackIdMapping:x,newTracks:T}),ku({subTracks:v,subItems:s.items,anchorTrackId:b,kind:"audio",compFrom:d,existingTracks:p,currentItems:C,trackIdMapping:x,newTracks:T}),T.length>0&&(p=[...p,...T],I.getState().setTracks(p));const k=new Map,_=s.items.flatMap(E=>{const D=w2({subItem:E,wrapper:u,timelineFps:m,subCompFps:s.fps});if(!D)return[];const L=$();return k.set(E.id,L),[{...D,id:L,trackId:x.get(E.trackId)??S??b??E.trackId}]});c.length>0&&I.getState()._removeItems(c),_.length>0&&I.getState()._addItems(_),le(c,new Set(c));const P=s.transitions??[];if(P.length>0){const E=q.getState().transitions,D=P.flatMap(L=>{const U=k.get(L.leftClipId),V=k.get(L.rightClipId);return!U||!V?[]:[{...L,id:$(),leftClipId:U,rightClipId:V,trackId:x.get(L.trackId)??L.trackId}]});q.getState().setTransitions([...E,...D]),D.length>0&&le(_.map(L=>L.id))}const R=s.keyframes??[];if(R.length>0){const E=H.getState().keyframes,D=R.map(L=>({...L,itemId:k.get(L.itemId)??L.itemId}));H.getState().setKeyframes([...E,...D])}return I2([n]).totalReferenceCount===0&&ae.getState().removeComposition(n),rt.getState().selectItems(_.map(E=>E.id)),!0},{compositionItemId:e})}function X2(e,t,r){const n=vr([{type:"composition.enter",params:{compositionId:e,...t===void 0?{}:{label:t},...r===void 0?{}:{entryItemId:r}}}]);if(!n.ok)throw new Error(n.issues.map(a=>a.message).join("; "));typeof window<"u"&&ae.getState().getComposition(e)?.editorKind==="composite-2d"&&zs.getState().setWorkspace("motion")}function Q2(e){const t=ae.getState().getComposition(e.compositionId);if(t?.editorKind!=="composite-2d")return 0;const r=new Set(e.editorialItemIds),n=new Set(t.items.filter(p=>r.has(p.id)).map(p=>p.id));if(n.size===0)return 0;const a=t.items.filter(p=>!n.has(p.id)),i=new Map(t.tracks.map(p=>[p.id,p])),o=new Set(a.map(p=>p.trackId));for(const p of[...o]){let f=i.get(p)?.parentTrackId;for(;f&&!o.has(f);)o.add(f),f=i.get(f)?.parentTrackId}const s=t.tracks.filter(p=>o.has(p.id)),l=t.transitions.filter(p=>!n.has(p.leftClipId)&&!n.has(p.rightClipId)),c=t.keyframes.filter(p=>!n.has(p.itemId)),u=a.reduce((p,f)=>Math.max(p,f.from+f.durationInFrames),1),d=e.suggestedDurationInFrames,m=typeof d=="number"&&Number.isFinite(d)?Math.max(u,Math.round(d),1):t.durationInFrames;return ae.getState().updateComposition(e.compositionId,{items:a,tracks:s,transitions:l,keyframes:c,durationInFrames:m}),de.getState().activeCompositionId===e.compositionId&&(I.getState().setItems(a),I.getState().setTracks(s),q.getState().setTransitions(l),H.getState().setKeyframes(c)),n.size}function J2(){A({type:"composition.create",params:y({name:w().optional(),itemIds:F(w()).min(1),openAfterCreate:B().optional(),editorKind:z(["sequence","composite-2d"]).optional()}).strict(),apply({draft:e},t){for(const n of t.itemIds)if(!e.items.some(a=>a.id===n))throw new Error(`Unknown item: ${n}`);const r=$2(t.name,t.itemIds,{openAfterCreate:!1,editorKind:t.editorKind});if(!r)throw new Error("Unable to create composition from selection");return t.openAfterCreate&&r.compositionId&&(t.editorKind==="composite-2d"?e.nav.switchToSequence(r.compositionId):e.nav.enterComposition(r.compositionId,t.name??r.label,r.id)),{inverse:{kind:"patch",patch:{}},result:r}}}),A({type:"composition.dissolve",params:y({itemId:w()}).strict(),apply(e,t){if(!Y2(t.itemId))throw new Error(`Unable to dissolve composition: ${t.itemId}`);return{kind:"patch",patch:{}}}}),A({type:"composition.rename",params:y({compositionId:w(),name:w().trim().min(1)}).strict(),apply(e,t){if(!x2(t.compositionId,t.name))throw new Error(`Unknown composition: ${t.compositionId}`);return{kind:"patch",patch:{}}}}),A({type:"composition.updateSettings",params:y({compositionId:w(),width:M().int().positive().optional(),height:M().int().positive().optional(),fps:M().positive().max(240).optional(),durationInFrames:M().int().positive().optional(),backgroundColor:w().optional()}).strict(),apply({draft:e},t){if(!e.compositions.some(a=>a.id===t.compositionId))throw new Error(`Unknown composition: ${t.compositionId}`);const{compositionId:r,...n}=t;return e.compositions=e.compositions.map(a=>a.id===r?{...a,...n}:a),{kind:"patch",patch:{}}}}),A({type:"composition.enter",history:"none",params:y({compositionId:w(),label:w().optional(),entryItemId:w().optional()}).strict(),apply({draft:e},t){const r=e.compositions.find(n=>n.id===t.compositionId);if(!r)throw new Error(`Unknown composition: ${t.compositionId}`);return r.editorKind==="composite-2d"||e.topLevelSequenceIds.includes(r.id)?e.nav.switchToSequence(r.id):e.nav.enterComposition(r.id,t.label??r.name,t.entryItemId),{kind:"commands",commands:[{type:"composition.exit",params:{}}]}}}),A({type:"composition.exit",history:"none",params:y({}).strict(),apply({draft:e}){const t=e.nav.activeCompositionId;return e.nav.exitComposition(),{kind:"commands",commands:t?[{type:"composition.enter",params:{compositionId:t}}]:[]}}}),A({type:"composition.openTab",history:"none",params:y({compositionId:w()}).strict(),apply({draft:e},t){const r=e.compositions.find(n=>n.id===t.compositionId);if(!r||r.editorKind==="composite-2d")throw new Error("Sequence tab requires an editorial composition");return e.topLevelSequenceIds.includes(t.compositionId)||(e.topLevelSequenceIds=[...e.topLevelSequenceIds,t.compositionId]),e.nav.switchToSequence(t.compositionId),{kind:"commands",commands:[{type:"composition.closeTab",params:t}]}}}),A({type:"composition.closeTab",history:"none",params:y({compositionId:w()}).strict(),apply({draft:e},t){return e.nav.breadcrumbs[0]?.compositionId===t.compositionId&&e.nav.switchToSequence(null),e.topLevelSequenceIds=e.topLevelSequenceIds.filter(r=>r!==t.compositionId),{kind:"commands",commands:[{type:"composition.openTab",params:t}]}}}),A({type:"composition.repairEditorialLeak",params:y({compositionId:w(),editorialItemIds:F(w()),suggestedDurationInFrames:M().optional()}).strict(),apply(e,t){const r=Q2(t);return{inverse:{kind:"patch",patch:{}},result:r}}}),A({type:"composition.switchSequence",history:"none",params:y({compositionId:w().nullable()}).strict(),apply(e,t){return de.getState().switchToSequence(t.compositionId),{kind:"patch",patch:{}}}})}function Fo(e,t){const r=Us();if(r!==null){const n=ae.getState().getComposition(r);if(n)return zg(n.durationInFrames)}return um(I.getState().items,W.getState().fps)}const Mu=y({frame:M().int().nonnegative(),color:w().default("#3B82F6"),label:w().default("")});function Z2(){A({type:"marker.add",params:Mu.extend({id:w().optional()}).strict(),apply({draft:e,ids:t},r){const n={...r,id:r.id??t.mint()};if(e.markers.some(a=>a.id===n.id))throw new Error(`Duplicate marker: ${n.id}`);return e.markers=[...e.markers,n],{inverse:{kind:"commands",commands:[{type:"marker.remove",params:{id:n.id}}]},result:n.id}}}),A({type:"marker.update",params:y({id:w(),updates:Mu.partial()}).strict(),mergeKey:e=>`marker.update:${e.id}`,apply({draft:e},t){if(!e.markers.find(n=>n.id===t.id))throw new Error(`Unknown marker: ${t.id}`);return e.markers=e.markers.map(n=>n.id===t.id?{...n,...t.updates}:n),{kind:"patch",patch:{}}}}),A({type:"marker.remove",params:y({id:w()}).strict(),apply({draft:e},t){if(!e.markers.some(r=>r.id===t.id))throw new Error(`Unknown marker: ${t.id}`);return e.markers=e.markers.filter(r=>r.id!==t.id),{kind:"patch",patch:{}}}}),A({type:"marker.clear",params:y({}).strict(),apply({draft:e}){return e.markers=[],{kind:"patch",patch:{}}}}),A({type:"marker.setInOut",params:y({inPoint:M().int().nonnegative().nullable(),outPoint:M().int().nonnegative().nullable()}).strict(),apply({draft:e},t){const r=Fo(),n=dm({...t,maxFrame:r});return e.inPoint=n.inPoint,e.outPoint=n.outPoint,{kind:"patch",patch:{}}}}),A({type:"marker.setIn",params:y({frame:M().finite()}).strict(),apply({draft:e},t){const r=Fo(),n=Math.max(0,Math.min(t.frame,r));return(e.outPoint===null||n>=e.outPoint)&&(e.outPoint=r),e.inPoint=n,{kind:"patch",patch:{}}}}),A({type:"marker.setOut",params:y({frame:M().finite()}).strict(),apply({draft:e},t){const r=Fo(),n=Math.max(1,Math.min(t.frame,r));return(e.inPoint===null||n<=e.inPoint)&&(e.inPoint=0),e.outPoint=n,{kind:"patch",patch:{}}}})}function eP(e,t,r,n){if(e===0)return 0;const a=new Map;for(const c of t)for(const u of[c.item,c.leftNeighbor,c.rightNeighbor]){if(!u||a.has(u.id))continue;const d=n[u.id];if(!d)continue;const m=d.properties.flatMap(p=>p.keyframes).map(p=>p.frame).filter(p=>p>=0&&p<u.durationInFrames&&!Yn(p,u.id,u,r));m.length>0&&a.set(u.id,{item:u,frames:m})}if(a.size===0)return e;const i=c=>{const u=new Map;for(const d of t)u.set(d.item.id,{...d.item,from:d.item.from+c}),d.leftNeighbor&&u.set(d.leftNeighbor.id,{...d.leftNeighbor,durationInFrames:d.leftNeighbor.durationInFrames+c}),d.rightNeighbor&&u.set(d.rightNeighbor.id,{...d.rightNeighbor,from:d.rightNeighbor.from+c,durationInFrames:d.rightNeighbor.durationInFrames-c});for(const[d,m]of a){const p=u.get(d)??m.item;for(const f of m.frames)if(f>=p.durationInFrames||Yn(f,d,p,r))return!1}return!0};if(i(e))return e;const o=e<0?-1:1;let s=0,l=Math.abs(e);for(;s<l;){const c=Math.ceil((s+l)/2);i(o*c)?s=c:l=c-1}return o*s}function Ss(e,t){return{...e,...t}}function Hf(e,t){if(!t(0))return 0;if(t(e))return e;const r=e<0?-1:1;let n=0,a=Math.abs(e);for(;n<a;){const i=Math.ceil((n+a)/2);t(r*i)?n=i:a=i-1}return r*n}function tP(e,t,r,n){const a=new Map;for(const i of e){const o=t.get(i),s=n[i];if(!o||!s)continue;const l=s.properties.flatMap(c=>c.keyframes).map(c=>c.frame).filter(c=>c>=0&&c<o.durationInFrames&&!Yn(c,i,o,r));l.length>0&&a.set(i,l)}return a}function Nf({items:e,transitions:t,affectedIds:r,keyframesByItemId:n,timelineFps:a,preserveKeyframes:i,buildPreview:o}){const s=new Map(e.map(u=>[u.id,u])),l=t.filter(u=>r.has(u.leftClipId)||r.has(u.rightClipId)),c=i?tP(r,s,t,n):new Map;return u=>{const d=o(u);for(const m of l){const p=d.get(m.leftClipId)??s.get(m.leftClipId),f=d.get(m.rightClipId)??s.get(m.rightClipId);if(p&&f&&!ua(p,f,m.durationInFrames,m.alignment,a).canAdd)return!1}for(const[m,p]of c){const f=d.get(m)??s.get(m);if(f){for(const h of p)if(h>=f.durationInFrames||Yn(h,m,f,t))return!1}}return!0}}function Cu(e,t,r,n,a,i,o,s=30,l=!0){if(r===0)return 0;const c=e,u=n,d=new Set([c.id,u.id]),m=Nf({items:a,transitions:i,affectedIds:d,keyframesByItemId:o,timelineFps:s,preserveKeyframes:l,buildPreview:p=>new Map([[c.id,Ss(c,ol(c,p,s))],[u.id,Ss(u,il(u,p,s))]])});return Hf(r,m)}function rP(e,t,r,n,a,i,o=30,s=new Set,l=!0){if(r===0)return 0;const c=e.from+e.durationInFrames,u=new Set(n.filter(p=>p.id!==e.id&&!s.has(p.id)&&p.trackId===e.trackId&&p.from>=c).map(p=>p.id));for(const p of a)p.leftClipId===e.id&&!s.has(p.rightClipId)&&u.add(p.rightClipId);const d=new Set([e.id,...u]),m=Nf({items:n,transitions:a,affectedIds:d,keyframesByItemId:i,timelineFps:o,preserveKeyframes:l,buildPreview:p=>{const f=new Map,h=t==="end"?ol(e,p,o):{...il(e,p,o),from:e.from};f.set(e.id,Ss(e,h));const g=t==="end"?p:-p;for(const v of u){const S=n.find(b=>b.id===v);S&&f.set(v,{...S,from:S.from+g})}return f}});return Hf(r,m)}function sr(e,t){return e<0?Math.max(e,t):Math.min(e,t)}function nP(e,t,r,n){const a=[t,...(r??[]).filter(o=>o!==t)],i=new Map;if(r!==void 0){const o=new Map(e.map(s=>[s.id,s]));for(const s of a){const l=o.get(s);l&&i.set(l.id,l)}return Array.from(i.values())}for(const o of a)for(const s of nn(e,o,n))i.set(s.id,s);return Array.from(i.values())}function Tu(e,t,r,n,a,i){let o=Math.max(e,-t.from);const s=new Set([t.id,r?.id,n?.id].filter(Boolean));if(r){const l=Nn(r,"end",o,i).clampedAmount;o=sr(o,l),o=sr(o,qn(r,"end",o,a,s))}if(n){const l=Nn(n,"start",o,i).clampedAmount;o=sr(o,l),o=sr(o,qn(n,"start",o,a,s))}for(const l of a){if(l.trackId!==t.trackId||s.has(l.id))continue;const c=l.from+l.durationInFrames;c<=t.from&&o<0&&(o=Math.max(o,c-t.from));const u=t.from+t.durationInFrames;l.from>=u&&o>0&&(o=Math.min(o,l.from-u))}return o}function aP(e){const t=I.getState(),r=t.items,n=[],a=new Set;for(const i of e){const o=t.itemById[i];if(!o||o.type==="text")continue;const s=o.from,l=o.from+o.durationInFrames;for(const c of sl(r,i)){const u=t.itemById[c];if(!u||u.type!=="text")continue;const d=u.from,m=u.from+u.durationInFrames,p=Math.max(d,s),f=Math.min(m,l);if(f<=p){a.add(u.id);continue}const h=f-p;(p!==u.from||h!==u.durationInFrames)&&n.push({id:u.id,from:p,durationInFrames:h,words:u.words?Kr(u.words,p-d,f-d):void 0})}}if(a.size>0){const i=Array.from(a);t._removeItems(i),H.getState()._removeKeyframesForItems(i)}for(const i of n)a.has(i.id)||t._updateItem(i.id,{from:i.from,durationInFrames:i.durationInFrames,...i.words?{words:i.words}:{}});return[...n.map(i=>i.id),...a]}function iP(e,t,r,n,a){const i=I.getState(),o=i.items,s=nP(o,e,n,a),l=s.find(g=>g.id===e);if(!l)return null;const c=W.getState().fps;let u=r;for(const g of s){const v=Nn(g,t,u,c).clampedAmount;u=sr(u,qn(g,t,v,o,ls(g.id)))}t==="start"?i._trimItemStart(e,u,{skipAdjacentClamp:!0}):i._trimItemEnd(e,u,{skipAdjacentClamp:!0});const d=I.getState().itemById[e],m=t==="start"?d?d.from-l.from:0:d?d.durationInFrames-l.durationInFrames:0;if(m!==0)for(const g of s)g.id!==e&&(t==="start"?i._trimItemStart(g.id,m,{skipAdjacentClamp:!0}):i._trimItemEnd(g.id,m,{skipAdjacentClamp:!0}));const f=(t==="start"?m>0:m<0)?aP(s.map(g=>g.id)):[],h=s.map(g=>g.id);return le(h),[...h,...f]}function Ao(e){const{id:t,handle:r,amount:n,itemIds:a,linkedSelection:i,removeTransitionIds:o}=e,s=(o?.length??0)>0;return s&&q.getState()._removeTransitions([...o]),iP(t,r,n,a,i)??(s?[]:null)}function oP(e){const{id:t,handle:r,delta:n,linkedSelection:a}=e;if(n===0)return null;const i=I.getState(),o=i.items.find(_=>_.id===t);if(!o)return null;const s=nn(i.items,t,a),l=new Set(s.map(_=>_.id)),c=new Map(s.map(_=>[_.id,_])),u=q.getState().transitions,d=H.getState().keyframesByItemId,m=W.getState().fps;let p=n;for(const _ of s)p=sr(p,rP(_,r,p,i.items,u,d,m,l,!1));if(p===0)return null;const f=o.from,h=o.from+o.durationInFrames;r==="start"?i._trimItemStart(t,p,{skipAdjacentClamp:!0}):i._trimItemEnd(t,p,{skipAdjacentClamp:!0});const g=I.getState().itemById[t];if(!g)return null;let v=0,S=null,b=null;if(r==="end"){const _=g.from+g.durationInFrames;v=_-h,v<0?S={start:_,end:h}:v>0&&(b=h);for(const P of s)P.id!==t&&v!==0&&i._trimItemEnd(P.id,v,{skipAdjacentClamp:!0})}else{const _=g.from-f;if(_!==0){i._moveItem(t,f);for(const P of s){if(P.id===t)continue;i._trimItemStart(P.id,_,{skipAdjacentClamp:!0});const R=c.get(P.id);R&&i._moveItem(P.id,R.from)}}v=-_,v<0?S={start:h+v,end:h}:v>0&&(b=h)}let x=[],T=[];if(v!==0){const _=I.getState().items,P=new Map;for(const D of s){const L=c.get(D.id);if(!L)continue;const U=L.from+L.durationInFrames,V=new Set;for(const ne of u)ne.leftClipId===D.id&&V.add(ne.rightClipId);for(const ne of _)l.has(ne.id)||ne.trackId!==L.trackId||(ne.from>=U||V.has(ne.id))&&P.set(ne.id,v)}const R=_.flatMap(D=>{const L=P.get(D.id)??0;return L===0?[]:[{id:D.id,from:D.from+L}]});R.length>0&&i._moveItems(R);const E=new Set(s.map(D=>D.trackId));if(S){const D=qi({editedTrackIds:E,intervals:[S]});x=D.affectedIds,T=D.removedIds}else b!==null&&(x=lk({editedTrackIds:E,cutFrame:b,amount:v}).affectedIds)}const C=I.getState().items,k=Array.from(new Set([...s.map(_=>_.id),...x,...C.filter(_=>!l.has(_.id)&&s.some(P=>_.trackId===P.trackId&&_.from>=P.from)).map(_=>_.id)]));return T.length>0&&(q.getState()._removeTransitionsForItems(T),H.getState()._removeKeyframesForItems(T)),le(k,T.length>0?new Set(T):void 0),k}function sP(e){const{leftId:t,rightId:r,delta:n,linkedSelection:a}=e;if(n===0)return null;const i=I.getState(),o=i.items,s=Pf(o,t,r,a),l=o.find(v=>v.id===r),c=o.find(v=>v.id===t);if(!c||!l)return null;const u=q.getState().transitions,d=H.getState().keyframesByItemId,m=W.getState().fps;let p=Cu(c,"end",n,l,o,u,d,m,!1);if(s&&(p=sr(p,Cu(s.leftCounterpart,"end",p,s.rightCounterpart,o,u,d,m,!1))),p===0)return null;p>0?(i._trimItemStart(r,p),i._trimItemEnd(t,p)):(i._trimItemEnd(t,p),i._trimItemStart(r,p));const f=I.getState().itemById[r],h=f?f.from-l.from:0;s&&h!==0&&(h>0?(i._trimItemStart(s.rightCounterpart.id,h,{skipAdjacentClamp:!0}),i._trimItemEnd(s.leftCounterpart.id,h,{skipAdjacentClamp:!0})):(i._trimItemEnd(s.leftCounterpart.id,h,{skipAdjacentClamp:!0}),i._trimItemStart(s.rightCounterpart.id,h,{skipAdjacentClamp:!0})));const g=s?[t,r,s.leftCounterpart.id,s.rightCounterpart.id]:[t,r];return le(g),g}function lP(e){const{id:t,delta:r,linkedSelection:n}=e;if(r===0)return null;const a=I.getState(),i=a.items,o=i.find(f=>f.id===t);if(!o||o.type!=="video"&&o.type!=="audio"&&o.type!=="composition")return null;const s=nn(i,t,n),l=o.sourceStart??0,c=o.sourceEnd;if(c===void 0)return null;const u=q.getState().transitions,d=W.getState().fps;let m=r;for(const f of s)m=mf(f.sourceStart??0,f.sourceEnd,f.sourceDuration,m),m=kw(f,m,i,u,d);if(m===0)return null;a._updateItem(t,{sourceStart:l+m,sourceEnd:c+m});for(const f of s)f.id===t||f.sourceEnd===void 0||a._updateItem(f.id,{sourceStart:(f.sourceStart??0)+m,sourceEnd:f.sourceEnd+m});const p=s.map(f=>f.id);return le(p),p}function cP(e){const{id:t,delta:r,leftNeighborId:n,rightNeighborId:a,linkedSelection:i}=e;if(r===0)return null;const o=I.getState(),s=o.items,l=q.getState().transitions,c=s.find(E=>E.id===t);if(!c)return null;const u=n?s.find(E=>E.id===n)??null:null,d=a?s.find(E=>E.id===a)??null:null,m=nn(s,t,i).find(E=>E.id!==t)??null,p=m&&n?au(s,n,m.trackId,m.type,i):null,f=m&&a?au(s,a,m.trackId,m.type,i):null,h=m?m.from+m.durationInFrames:0,g=m?s.find(E=>E.trackId===m.trackId&&E.id!==m.id&&E.from+E.durationInFrames===m.from)??p:null,v=m?s.find(E=>E.trackId===m.trackId&&E.id!==m.id&&E.from===h)??f:null,S=W.getState().fps;let b=Tu(r,c,u,d,s,S);if(m&&(b=Tu(b,m,g,v,s,S)),b=Zc(c,b,u,d,s,l,S),m&&(b=Zc(m,b,g,v,s,l,S)),b=eP(b,[{item:c,leftNeighbor:u,rightNeighbor:d},...m?[{item:m,leftNeighbor:g,rightNeighbor:v}]:[]],l,H.getState().keyframesByItemId),b===0)return null;const x=c.from,T=c.sourceStart??0,C=pf(c,u,d,b,S);b>0?(a&&o._trimItemStart(a,b,{skipAdjacentClamp:!0}),n&&o._trimItemEnd(n,b,{skipAdjacentClamp:!0})):(n&&o._trimItemEnd(n,b,{skipAdjacentClamp:!0}),a&&o._trimItemStart(a,b,{skipAdjacentClamp:!0})),o._moveItem(t,c.from+b),C!==0&&(c.type==="video"||c.type==="audio"||c.type==="composition")&&c.sourceEnd!==void 0&&o._updateItem(t,{sourceStart:(c.sourceStart??0)+C,sourceEnd:c.sourceEnd+C});const k=I.getState().itemById[t],_=k?k.from-x:0,P=k&&k.sourceStart!==void 0?k.sourceStart-T:0;m&&_!==0&&(_>0?(v&&o._trimItemStart(v.id,_,{skipAdjacentClamp:!0}),g&&o._trimItemEnd(g.id,_,{skipAdjacentClamp:!0})):(g&&o._trimItemEnd(g.id,_,{skipAdjacentClamp:!0}),v&&o._trimItemStart(v.id,_,{skipAdjacentClamp:!0})),o._moveItem(m.id,m.from+_),P!==0&&qt(m)&&m.sourceEnd!==void 0&&o._updateItem(m.id,{sourceStart:(m.sourceStart??0)+P,sourceEnd:m.sourceEnd+P}));const R=[t];return n&&R.push(n),a&&R.push(a),m&&(R.push(m.id),g&&R.push(g.id),v&&R.push(v.id)),le(R),R}function hl(e,t,r,n){return e.some(a=>{const i=Xr(a.durationInFrames,a.alignment);return a.leftClipId===t&&r>=n-i.leftPortion||a.rightClipId===t&&r<i.rightPortion})}function gl(e,t,r){return hl(q.getState().transitions,e,t,r)}function uP(e,t,r,n,a){const i=da([...e],r,a);if(i.length===0)return"missing-item";for(const o of i){if(n<=o.from||n>=o.from+o.durationInFrames)return"out-of-range";if(hl(t,o.id,n-o.from,o.durationInFrames))return"transition-overlap"}return null}function yl(e,t){return e.map(r=>({originalId:r.id,originalLinkedGroupId:r.linkedGroupId,result:I.getState()._splitItem(r.id,t)})).filter(r=>r.result!==null)}function dP(e){const{id:t,frame:r,linkedSelection:n}=e,a=da(I.getState().items,t,n),i=yl(a,r),o=i.find(s=>s.originalId===t)?.result??null;return o?(ma(i),{...o,leftItemIds:i.map(s=>s.result.leftItem.id)}):null}function mP(e){const{frame:t,linkedSelection:r}=e,n=I.getState().items,a=n.filter(s=>t>s.from&&t<s.from+s.durationInFrames).map(s=>s.id),i=Gp(n,a),o={splitCount:0,blockedByTransitionCount:0,leftItemIds:[]};for(const s of i){const l=da(I.getState().items,s,r);if(l.length===0)continue;let c=!1;if(!l.every(p=>t<=p.from||t>=p.from+p.durationInFrames?!1:gl(p.id,t-p.from,p.durationInFrames)?(c=!0,!1):!0)){c&&(o.blockedByTransitionCount+=1);continue}const d=yl(l,t);(d.find(p=>p.originalId===s)?.result??null)&&(ma(d),o.leftItemIds=d.map(p=>p.result.leftItem.id),o.splitCount+=1)}return o}function pP(e){const{id:t,linkedSelection:r}=e,n=[...e.frames].sort((s,l)=>l-s);let a=0;const i=da(I.getState().items,t,r);if(i.length===0)return{splitCount:a};const o=new Map(i.map(s=>[s.id,[]]));for(const s of n){const l=I.getState().itemById;if(!i.every(d=>{const m=l[d.id];return!m||s<=m.from||s>=m.from+m.durationInFrames?!1:!gl(m.id,s-m.from,m.durationInFrames)}))continue;const u=yl(i,s);if(u.length===i.length){ma(u),a++;for(const d of u)o.get(d.originalId)?.push(d.result.rightItem.id),le([d.result.leftItem.id,d.result.rightItem.id])}}if(a>0){const s=I.getState();for(const[l,c]of o){for(const u of c)s._updateItem(u,{fadeIn:0});for(let u=1;u<c.length;u+=1)s._updateItem(c[u],{fadeOut:0});s._updateItem(l,{fadeOut:0})}}return{splitCount:a}}function fP(e){const{itemIds:t,linkedSelection:r}=e,n=I.getState().items,a=n.filter(c=>t.includes(c.id)).toSorted((c,u)=>c.from-u.from);if(a.length<2)return null;const i=[t];if(a.length===2){const[c,u]=a;if(c&&u){const d=Pf(n,c.id,u.id,r);d&&i.push([d.leftCounterpart.id,d.rightCounterpart.id])}}const o=i.map(c=>n.filter(u=>c.includes(u.id)).toSorted((u,d)=>u.from-d.from)).filter(c=>c.length>=2).map(c=>({itemIds:c.map(u=>u.id),primaryId:c[0].id,removedIds:c.slice(1).map(u=>u.id)}));for(const c of o)I.getState()._joinItems(c.itemIds);const s=new Map;for(const c of o)for(const u of c.removedIds)s.set(u,c.primaryId);if(s.size>0){const u=q.getState().transitions.flatMap(d=>{const m={...d,leftClipId:s.get(d.leftClipId)??d.leftClipId,rightClipId:s.get(d.rightClipId)??d.rightClipId};return m.leftClipId===m.rightClipId?[]:[m]});q.getState().setTransitions(u),le(o.map(d=>d.primaryId))}const l=o.flatMap(c=>c.removedIds);return l.length>0&&H.getState()._removeKeyframesForItems(l),o.map(c=>c.primaryId)}function Mn(e,t,r,n,a){const i=e.find(s=>s.id===t);if(!i||n.has(t))return;n.add(t),a.push({id:t,from:r(i.from)});const o=Wn(e,la(e,t));for(const s of o){if(s===t||n.has(s))continue;const l=e.find(c=>c.id===s);l&&(n.add(s),a.push({id:s,from:r(l.from)}))}}function qf(e,t,r,n,a){if(r<=0||a<=0)return[];const i=I.getState(),o=a/r,s=n+a,l=[];for(const c of sl(i.items,e)){const u=i.itemById[c];if(!u||u.type!=="text")continue;const d=n+Math.round((u.from-t)*o),m=n+Math.round((u.from+u.durationInFrames-t)*o),p=Math.max(d,n),f=Math.min(m,s);if(f<=p){i._removeItems([u.id]),H.getState()._removeKeyframesForItems([u.id]),l.push(u.id);continue}const h=f-p,g=p!==u.from||h!==u.durationInFrames,v=u.words&&(g||o!==1)?Kr(dg(u.words,m-d,u.durationInFrames),p-d,f-d):void 0;!g&&v===void 0||(i._updateItem(u.id,{from:p,durationInFrames:h,...v?{words:v}:{}}),l.push(u.id))}return l}function hP(e){const{id:t,newFrom:r,newDuration:n,newSpeed:a,linkedSelection:i}=e,o=I.getState(),s=o.items,l=nn(s,t,i),c=l.find(E=>E.id===t);if(!c)return null;const u=c.durationInFrames,d=c.from,m=d+u;o._rateStretchItem(t,r,n,a);const p=I.getState().itemById[t];if(!p)return null;const f=p.from,h=p.durationInFrames,g=p.speed??a,v=f-c.from;for(const E of l)E.id!==t&&o._rateStretchItem(E.id,E.from+v,h,g);if(u!==h)for(const E of l)H.getState()._scaleKeyframesForItem(E.id,E.durationInFrames,h);const S=[];if(u!==h||v!==0)for(const E of l){const D=I.getState().itemById[E.id];D&&S.push(...qf(E.id,E.from,E.durationInFrames,D.from,D.durationInFrames))}const b=f+h-m,x=new Set(l.map(E=>E.id)),T=I.getState().items,C=q.getState().transitions,k=new Set,_=[],P=new Set;for(const E of l){const D=T.find(L=>L.id===E.id);D&&P.add(D.trackId)}if(b!==0){const E=D=>D+b;for(const D of P){const L=T.filter(U=>U.trackId===D&&!x.has(U.id)&&U.from>=m).sort((U,V)=>U.from-V.from);for(const U of L)Mn(T,U.id,E,k,_)}for(const D of l)for(const L of C)L.leftClipId===D.id&&!x.has(L.rightClipId)&&Mn(T,L.rightClipId,E,k,_)}if(v!==0){const E=D=>Math.max(0,D+v);for(const D of P){const L=T.filter(U=>U.trackId!==D||x.has(U.id)?!1:U.from+U.durationInFrames<=d).sort((U,V)=>U.from-V.from);for(const U of L)Mn(T,U.id,E,k,_)}for(const D of l)for(const L of C)L.rightClipId===D.id&&!x.has(L.leftClipId)&&Mn(T,L.leftClipId,E,k,_)}_.length>0&&I.getState()._moveItems(_);const R=[...x,...k,...S];return le(R),R}const gP=.01;function yP(e){const{itemIds:t,linkedSelection:r}=e,n=I.getState(),a=W.getState().fps,i=new Set,o=[];for(const p of t){if(i.has(p))continue;const f=n.items.find(b=>b.id===p);if(!f||f.type!=="video"&&f.type!=="audio")continue;const h=f.speed||1;if(Math.abs(h-1)<=gP)continue;const g=nn(n.items,p,r);for(const b of g)i.add(b.id);const v=f.sourceFps??a,S=f.sourceEnd!==void 0&&f.sourceStart!==void 0?f.sourceEnd-f.sourceStart:we(f.durationInFrames,h,a,v);o.push({id:p,oldEnd:f.from+f.durationInFrames,newDuration:Math.max(1,Tt(S,1,v,a)),synchronizedIds:g.map(b=>b.id)})}if(o.length===0)return null;const s=[];for(const p of o){const f=n.items.find(x=>x.id===p.id);if(!f)continue;const h=f.durationInFrames,g=new Map(I.getState().items.filter(x=>p.synchronizedIds.includes(x.id)).map(x=>[x.id,{from:x.from,durationInFrames:x.durationInFrames}]));n._rateStretchItem(p.id,f.from,p.newDuration,1);const v=I.getState().itemById[p.id];if(!v)continue;const S=v.durationInFrames,b=v.from-f.from;for(const x of p.synchronizedIds){if(x===p.id)continue;const T=I.getState().items.find(C=>C.id===x);T&&n._rateStretchItem(x,T.from+b,S,v.speed??1)}if(h!==S)for(const x of p.synchronizedIds)H.getState()._scaleKeyframesForItem(x,h,S);for(const x of p.synchronizedIds){const T=g.get(x),C=I.getState().itemById[x];!T||!C||T.durationInFrames===C.durationInFrames&&T.from===C.from||s.push(...qf(x,T.from,T.durationInFrames,C.from,C.durationInFrames))}}const l=I.getState().items,c=new Set(o.flatMap(p=>p.synchronizedIds)),u=[],d=new Set;for(const p of o){const f=l.find(v=>v.id===p.id);if(!f)continue;const h=f.from+f.durationInFrames-p.oldEnd;if(h<=0)continue;const g=new Set;for(const v of p.synchronizedIds){const S=l.find(b=>b.id===v);S&&g.add(S.trackId)}for(const v of g){const S=l.filter(b=>b.trackId===v&&!c.has(b.id)&&b.from>=p.oldEnd).sort((b,x)=>b.from-x.from);for(const b of S)Mn(l,b.id,x=>x+h,d,u)}}u.length>0&&I.getState()._moveItems(u);const m=[...c,...d,...s];return le(m),m}function vP(e){if(!e)return;const t=Me.getState().mediaById[e]?.fps;return typeof t=="number"&&Number.isFinite(t)&&t>0?t:void 0}function Gf(e,t){if(e.type!=="video"&&e.type!=="audio"&&e.type!=="composition")return t;if(e.sourceFps!==void 0)return e.sourceFps;const r=e.type==="composition"?void 0:e.mediaId;return vP(r)??t}function Is(e){return e.type==="video"||e.type==="audio"||e.type==="composition"?e.speed??1:1}function Eu(e,t,r){const n=Gf(e,r),a=Math.round(t*n),i=e.type==="video"||e.type==="audio"?e.sourceStart??0:0,o=e.type==="video"||e.type==="audio"?e.sourceEnd??i+we(e.durationInFrames,Is(e),r,n):i,s=e.isReversed?o-a:a-i,l=Tt(s,Is(e),n,r);return Math.round(e.from+l)}function bP(e,t){if(e.type!=="video"&&e.type!=="audio")return null;const r=Gf(e,t),n=e.sourceStart??0,a=we(e.durationInFrames,Is(e),t,r),i=e.sourceEnd??n+a;return{start:n/r,end:i/r}}const SP=.75;function Fu(){return{analyzedItemCount:0,removedRangeCount:0,removedItemCount:0,splitCount:0}}function IP(e,t){const r=e.end-e.start;return r<=0?!1:t.reduce((a,i)=>{const o=Math.max(e.start,i.start),s=Math.min(e.end,i.end);return a+Math.max(0,s-o)},0)/r>=SP}function xP(e,t){const r=I.getState().items,n=ur(r,e,t);if(n.length===0)return{removedIds:[],affectedIds:[]};const a=new Set(n),i=r.filter(k=>!a.has(k.id)),o=new Map,s=new Set(r.filter(k=>a.has(k.id)).map(k=>k.trackId)),l=r.filter(k=>a.has(k.id)).map(k=>({start:k.from,end:k.from+k.durationInFrames}));for(const k of i){const _=r.filter(P=>a.has(P.id)).filter(P=>P.trackId===k.trackId&&P.from+P.durationInFrames<=k.from).reduce((P,R)=>P+R.durationInFrames,0);_>0&&o.set(k.id,_)}const c=new Map(I.getState().tracks.map(k=>[k.id,k])),u=new Map(i.map(k=>[k.id,k])),d=new Map;for(const[k,_]of o){if(_<=0)continue;const P=ur(i,[k],t);for(const R of P){const E=u.get(R);!E||!s.has(E.trackId)&&Ti(c.get(E.trackId))||d.set(R,Math.max(d.get(R)??0,_))}}const m=i.flatMap(k=>{const _=d.get(k.id)??0;return _>0?[{id:k.id,from:k.from-_}]:[]}),p=new Map(m.map(k=>[k.id,k.from])),f=[];for(const k of i){if(p.has(k.id))continue;const _=k.from+k.durationInFrames;for(const P of i){const R=p.get(P.id);if(R===void 0||P.trackId!==k.trackId)continue;const E=R+P.durationInFrames;if(R<_&&E>k.from){f.push(k.id);break}}}const h=ur(i,f,t),g=[...n,...h],v=new Set(h),S=v.size>0?m.filter(k=>!v.has(k.id)):m,b=I.getState();b._removeItems(g),S.length>0&&b._moveItems(S);const x=qi({editedTrackIds:s,intervals:l}),T=Array.from(new Set([...g,...x.removedIds]));q.getState()._removeTransitionsForItems(T),H.getState()._removeKeyframesForItems(T),S.length>0&&le(S.map(k=>k.id));const C=Array.from(new Set([...m.map(k=>k.id),...x.affectedIds]));return C.length>0&&le(C,new Set(T)),{removedIds:T,affectedIds:Array.from(new Set([...C,...S.map(k=>k.id)]))}}function wP(e){const{itemIds:t,rangesByMediaId:r,linkedSelection:n}=e;if(t.length===0)return{summary:Fu(),affectedIds:[]};const a=W.getState().fps,i=I.getState().items,s=Gp(i,t).map(f=>i.find(h=>h.id===f)).filter(f=>f!==void 0&&(f.type==="video"||f.type==="audio")&&!!f.mediaId&&(r[f.mediaId]?.length??0)>0);if(s.length===0)return{summary:Fu(),affectedIds:[]};const l=s.map(f=>({id:f.id,mediaId:f.mediaId,originId:f.originId??f.id}));let c=0;for(const f of s){const h=r[f.mediaId];if(!h||h.length===0)continue;const g=Array.from(new Set(h.flatMap(S=>[Eu(f,S.start,a),Eu(f,S.end,a)]))).filter(S=>S>f.from&&S<f.from+f.durationInFrames).sort((S,b)=>b-S);if(g.length===0)continue;const v=da(I.getState().items,f.id,n);if(v.length!==0)for(const S of g){const b=I.getState().itemById;if(!v.every(C=>{const k=b[C.id];return!k||S<=k.from||S>=k.from+k.durationInFrames?!1:!gl(k.id,S-k.from,k.durationInFrames)}))continue;const T=v.map(C=>({originalId:C.id,originalLinkedGroupId:C.linkedGroupId,result:I.getState()._splitItem(C.id,S)})).filter(C=>C.result!==null);if(T.length===v.length){ma(T),c+=1;for(const C of T)le([C.result.leftItem.id,C.result.rightItem.id])}}}const u=I.getState().items,d=new Set,m=new Set;for(const f of l){const h=r[f.mediaId];if(!(!h||h.length===0))for(const g of u){if(g.type!=="video"&&g.type!=="audio"||g.mediaId!==f.mediaId||(g.originId??g.id)!==f.originId)continue;const v=bP(g,a);v!==null&&IP(v,h)&&(d.add(g.id),h.forEach((S,b)=>{S.end>v.start&&S.start<v.end&&m.add(`${f.originId}:${b}`)}))}}if(d.size===0)return{summary:{analyzedItemCount:s.length,removedRangeCount:0,removedItemCount:0,splitCount:c},affectedIds:[]};const p=xP(Array.from(d),n);return{summary:{analyzedItemCount:s.length,removedRangeCount:m.size,removedItemCount:p.removedIds.length,splitCount:c},affectedIds:Array.from(new Set([...d,...p.affectedIds]))}}function kP(e,t,r,n){const a=e.find(i=>i.id===r);return!a||a.type!=="video"?"not-video":n<=a.from||n>=a.from+a.durationInFrames?"out-of-range":hl(t,r,n-a.from,a.durationInFrames)?"transition-overlap":null}function PP(e){const{itemId:t,playheadFrame:r,freezeDurationFrames:n,image:a}=e,i=I.getState().itemById[t];if(!i)return null;const o=I.getState()._splitItem(t,r);if(!o)return null;const{leftItem:s,rightItem:l}=o,c=q.getState().transitions;q.getState().setTransitions(c.map(m=>m.leftClipId===t?{...m,leftClipId:l.id}:m));const u={id:$(),type:"image",trackId:i.trackId,from:r,durationInFrames:n,label:a.label,mediaId:a.mediaId,src:a.src,sourceWidth:a.sourceWidth,sourceHeight:a.sourceHeight,transform:i.transform?{...i.transform}:void 0};I.getState()._addItem(u),I.getState()._moveItem(l.id,l.from+n);const d=I.getState().items.filter(m=>m.trackId===i.trackId&&m.id!==l.id&&m.id!==s.id&&m.id!==u.id&&m.from>r);for(const m of d)I.getState()._moveItem(m.id,m.from+n);return le([s.id,l.id]),{freezeFrameItemId:u.id}}function MP(e){const{insertFrame:t,clipDurationFrames:r,tracks:n,items:a}=e,i=I.getState();i.setTracks(n);const o=Array.from(new Set(a.map(u=>u.trackId))),s=[],l=[];for(const u of o){const d=I.getState().items.find(p=>p.trackId===u&&p.from<t&&p.from+p.durationInFrames>t);if(d){const p=i._splitItem(d.id,t);p&&s.push(p.leftItem.id,p.rightItem.id)}const m=I.getState().items.filter(p=>p.trackId===u&&p.from>=t);for(const p of m)i._moveItem(p.id,p.from+r),l.push(p.id)}for(const u of a)i._addItem(u);const c=[...a.map(u=>u.id),...l,...s];return le(c),c}function CP(e){const{overwriteStart:t,overwriteEnd:r,tracks:n,items:a}=e,i=I.getState();i.setTracks(n);const o=Array.from(new Set(a.map(l=>l.trackId))),s=[];for(const l of o){const c=I.getState().items.filter(u=>u.trackId===l&&u.from<r&&u.from+u.durationInFrames>t);for(const u of c){const d=u.from+u.durationInFrames,m=u.from<t,p=d>r;if(!m&&!p)i._removeItems([u.id]);else if(m&&p){const f=i._splitItem(u.id,t);if(f){s.push(f.leftItem.id);const h=I.getState()._splitItem(f.rightItem.id,r);h&&(i._removeItems([h.leftItem.id]),s.push(h.rightItem.id))}}else if(m){const f=i._splitItem(u.id,t);f&&(i._removeItems([f.rightItem.id]),s.push(f.leftItem.id))}else{const f=i._splitItem(u.id,r);f&&(i._removeItems([f.leftItem.id]),s.push(f.rightItem.id))}}}for(const l of a)i._addItem(l),s.push(l.id);return le(s),s}function TP(e){const{items:t,tracks:r}=I.getState(),n=hk({tracks:r,items:t,keyframes:H.getState().keyframes,fps:W.getState().fps,videoHasAudioByMediaId:e.videoHasAudioByMediaId,createId:$});if(!n.changed)return!1;I.getState().setTracks(n.tracks),I.getState().setItems(n.items),H.getState().setKeyframes(n.keyframes);const a=de.getState().activeCompositionId;return a&&ae.getState().updateComposition(a,{tracks:n.tracks,items:n.items,keyframes:n.keyframes}),!0}const Be=w().min(1),ye=M().finite(),pa=F(Be),je=B(),Kf=z(["start","end"]);function Mt(e){return{inverse:{kind:"patch",patch:{}},result:e}}function lt(e){return Mt(e?{affectedIds:e}:null)}function st(e,t,r){return e.items.some(n=>n.id===t)?[]:[{message:`Không tìm thấy clip ${t}`,code:"missing-item",path:r}]}const vl={id:Be,amount:ye,itemIds:pa.optional(),linkedSelection:je},EP=y({...vl,handle:Z("start")}).strict(),FP=y({...vl,handle:Z("end")}).strict(),AP=y({...vl,handle:Kf,removeTransitionIds:pa}).strict();function _P(){A({type:"edit.trimItemStart",params:EP,validate:(a,i)=>st(a,i.id,"id"),apply:(a,i)=>lt(Ao(i))}),A({type:"edit.trimItemEnd",params:FP,validate:(a,i)=>st(a,i.id,"id"),apply:(a,i)=>lt(Ao(i))}),A({type:"edit.trimItemBreakingTransition",params:AP,validate:(a,i)=>{const o=new Set(a.transitions.map(s=>s.id));return[...st(a,i.id,"id"),...i.removeTransitionIds.filter(s=>!o.has(s)).map(s=>({message:`Không tìm thấy transition ${s}`,code:"missing-transition",path:"removeTransitionIds"}))]},apply:(a,i)=>lt(Ao(i))});const e=y({id:Be,handle:Kf,delta:ye,linkedSelection:je}).strict();A({type:"edit.rippleTrim",params:e,validate:(a,i)=>st(a,i.id,"id"),apply:(a,i)=>lt(oP(i))});const t=y({leftId:Be,rightId:Be,delta:ye,linkedSelection:je}).strict();A({type:"edit.rollingTrim",params:t,validate:(a,i)=>[...st(a,i.leftId,"leftId"),...st(a,i.rightId,"rightId")],apply:(a,i)=>lt(sP(i))});const r=y({id:Be,delta:ye,linkedSelection:je}).strict();A({type:"edit.slip",params:r,validate:(a,i)=>st(a,i.id,"id"),apply:(a,i)=>lt(lP(i))});const n=y({id:Be,delta:ye,leftNeighborId:Be.nullable(),rightNeighborId:Be.nullable(),linkedSelection:je}).strict();A({type:"edit.slide",params:n,validate:(a,i)=>st(a,i.id,"id"),apply:(a,i)=>lt(cP(i))})}function OP(){const e=y({id:Be,frame:ye,linkedSelection:je}).strict();A({type:"edit.splitItem",params:e,validate:(a,i)=>{const o=uP(a.items,a.transitions,i.id,i.frame,i.linkedSelection);return o?[{message:o==="transition-overlap"?"Không thể cắt bên trong vùng chuyển cảnh":o==="out-of-range"?"Điểm cắt nằm ngoài clip":`Không tìm thấy clip ${i.id}`,code:o,path:"frame"}]:[]},apply:(a,i)=>Mt(dP(i))});const t=y({frame:ye,linkedSelection:je}).strict();A({type:"edit.splitAllAtFrame",params:t,apply:(a,i)=>Mt(mP(i))});const r=y({id:Be,frames:F(ye).min(1),linkedSelection:je}).strict();A({type:"edit.splitItemAtFrames",params:r,validate:(a,i)=>st(a,i.id,"id"),apply:(a,i)=>Mt(pP(i))});const n=y({itemIds:pa.min(2),linkedSelection:je}).strict();A({type:"edit.joinItems",params:n,apply:(a,i)=>lt(fP(i))})}function DP(){const e=y({id:Be,newFrom:ye,newDuration:ye,newSpeed:ye.positive(),linkedSelection:je,mergeKey:w().min(1).optional()}).strict();A({type:"edit.rateStretch",params:e,validate:(r,n)=>st(r,n.id,"id"),apply:(r,n)=>lt(hP(n)),mergeKey:r=>r.mergeKey??null});const t=y({itemIds:pa,linkedSelection:je}).strict();A({type:"edit.resetSpeedWithRipple",params:t,apply:(r,n)=>lt(yP(n))})}const RP=y({start:ye,end:ye}).strict(),LP=y({itemIds:pa,rangesByMediaId:kt(w(),F(RP)),linkedSelection:je}).strict();function BP(){for(const e of["edit.removeSilence","edit.removeFillerWords","edit.removeTranscriptRanges"])A({type:e,params:LP,apply:(t,r)=>Mt(wP(r))})}function zP(){const e=y({itemId:Be,playheadFrame:ye,freezeDurationFrames:ye.min(1),image:y({label:w(),mediaId:Be,src:w().min(1),sourceWidth:ye.positive(),sourceHeight:ye.positive()}).strict()}).strict();A({type:"edit.insertFreezeFrame",params:e,validate:(t,r)=>{const n=kP(t.items,t.transitions,r.itemId,r.playheadFrame);return n?[{message:`Không thể chèn khung hình tĩnh (${n})`,code:n,path:"playheadFrame"}]:[]},apply:(t,r)=>Mt(PP(r))})}const Au=nd(e=>{if(typeof e!="object"||e===null)return!1;const t=e;return typeof t.id=="string"&&t.id.length>0&&typeof t.type=="string"&&typeof t.trackId=="string"&&typeof t.from=="number"&&Number.isFinite(t.from)&&typeof t.durationInFrames=="number"&&Number.isFinite(t.durationInFrames)&&t.durationInFrames>0},{message:"Clip không hợp lệ"}),_u=nd(e=>typeof e=="object"&&e!==null&&typeof e.id=="string",{message:"Track không hợp lệ"});function Ou(e,t,r){const n=[],a=new Set(t.map(s=>s.id)),i=new Set(e.items.map(s=>s.id)),o=new Set;for(const s of r)if((i.has(s.id)||o.has(s.id))&&n.push({message:`Clip ${s.id} đã tồn tại`,code:"duplicate-id",path:"items"}),o.add(s.id),a.has(s.trackId)||n.push({message:`Không tìm thấy track ${s.trackId}`,code:"missing-track",path:"items"}),s.type==="video"||s.type==="audio"){const{sourceStart:l,sourceEnd:c}=s;(l!==void 0&&(!Number.isFinite(l)||l<0)||c!==void 0&&(!Number.isFinite(c)||c<=(l??0)))&&n.push({message:`Giới hạn nguồn của clip ${s.id} không hợp lệ`,code:"source-bounds",path:"items"})}return n}function UP(){const e=y({insertFrame:ye.min(0),clipDurationFrames:ye.min(1),tracks:F(_u),items:F(Au).min(1)}).strict();A({type:"edit.sourceInsert",params:e,validate:(n,a)=>Ou(n,a.tracks,a.items),apply:(n,a)=>Mt(MP(a))});const t=y({overwriteStart:ye.min(0),overwriteEnd:ye.min(1),tracks:F(_u),items:F(Au).min(1)}).strict();A({type:"edit.sourceOverwrite",params:t,validate:(n,a)=>[...a.overwriteEnd>a.overwriteStart?[]:[{message:"Vùng ghi đè phải có độ dài dương",code:"empty-range",path:"overwriteEnd"}],...Ou(n,a.tracks,a.items)],apply:(n,a)=>Mt(CP(a))});const r=y({videoHasAudioByMediaId:kt(w(),B())}).strict();A({type:"edit.repairLegacyAvTracks",params:r,apply:(n,a)=>Mt(TP(a))})}function HP(){_P(),OP(),DP(),BP(),zP(),UP()}function NP(e,t){const r=I.getState(),n=H.getState();if(t.enabled&&(rp(e,t.sourceItemId,a=>r.itemById[a],a=>n.keyframesByItemId[a])||np(t.targetProperty)&&qb(e,t.sourceItemId,a=>r.itemById[a]))){Se().warn("Cannot create a cyclic or redundant property link",{itemId:e,property:t.targetProperty,sourceItemId:t.sourceItemId,sourceProperty:t.sourceProperty});return}H.getState()._setDirectPropertyLink(e,t)}function qP(e,t){H.getState()._removeDirectPropertyLink(e,t)}function GP(e,t){H.getState()._setPropertyExpression(e,t)}function KP(e,t){H.getState()._removePropertyExpression(e,t)}function VP(e,t,r){return Ge(e,r.frame)?H.getState()._upsertVectorKeyframe(e,t,r):(Se().warn("Cannot add vector keyframe in transition region",{itemId:e,property:t,frame:r.frame}),"")}function WP(e,t,r,n){if(typeof n.frame=="number"&&!Ge(e,n.frame)){Se().warn("Cannot move vector keyframe into transition region",{itemId:e,property:t,keyframeId:r,frame:n.frame});return}H.getState()._updateVectorKeyframe(e,t,r,n)}function jP(e,t,r){H.getState()._removeVectorKeyframe(e,t,r)}function $P(e,t,r){H.getState()._replaceScalarPropertiesWithVectorProperty(e,t,r)}function YP(e,t,r,n={}){H.getState()._setVectorDimensionsSeparated(e,t,{separated:r,...n})}function _o(e){if(e.length===0)return[];const t=e.filter(r=>Ge(r.itemId,r.frame));return t.length===0?(Se().warn("All keyframes blocked by transition regions",{originalCount:e.length}),[]):(t.length<e.length&&Se().warn("Some keyframes blocked by transition regions",{originalCount:e.length,validCount:t.length}),H.getState()._addKeyframes(t))}const XP={position:["x","y"],scale:["width","height"],anchor:["anchorX","anchorY"]};function QP(e,t=[],r=[]){if(e.length===0&&r.length===0)return[];const n=e.filter(o=>Ge(o.itemId,o.frame)),a=r.flatMap(o=>o.keyframes.map(s=>({itemId:o.itemId,frame:s.frame}))),i=a.filter(o=>Ge(o.itemId,o.frame));return n.length<e.length||i.length<a.length?(Se().warn("Preset keyframes blocked by transition regions; skipping apply",{originalCount:e.length,validCount:n.length,vectorCount:a.length,validVectorCount:i.length}),[]):(()=>{const o=H.getState();for(const l of t){if(l.fromFrame===void 0||l.toFrame===void 0){o._removeKeyframesForProperty(l.itemId,l.property);continue}const c=o.keyframesByItemId[l.itemId]?.properties.find(d=>d.property===l.property);if(!c)continue;const u=c.keyframes.filter(d=>d.frame>=l.fromFrame&&d.frame<=l.toFrame).map(d=>({itemId:l.itemId,property:l.property,keyframeId:d.id}));u.length>0&&o._removeKeyframes(u)}const s=o._addKeyframes(n);for(const l of r){const c=o.keyframesByItemId[l.itemId]?.vectorProperties?.find(d=>d.property===l.property)?.keyframes??[],u=new Map(c.filter(d=>!l.replaceRange||d.frame<l.replaceRange.fromFrame||d.frame>l.replaceRange.toFrame).map(d=>[d.frame,d]));for(const d of l.keyframes){if(!l.replaceRange&&u.has(d.frame))continue;const p=u.get(d.frame)?.id??$();u.set(d.frame,{id:p,frame:d.frame,value:d.value,easing:d.easing??"linear",easingConfig:d.easingConfig,temporalEase:d.temporalEase,spatial:d.spatial,source:d.source}),s.push(p)}o._replaceScalarPropertiesWithVectorProperty(l.itemId,{property:l.property,keyframes:[...u.values()].sort((d,m)=>d.frame-m.frame)},XP[l.property])}return s})()}function JP(e,t,r,n){if(typeof n.frame=="number"&&!Ge(e,n.frame)){Se().warn("Cannot move keyframe into transition region",{itemId:e,property:t,keyframeId:r,frame:n.frame});return}H.getState()._updateKeyframe(e,t,r,n)}function Du(e){if(e.length===0)return;const t=e.filter(r=>{if(typeof r.updates.frame!="number")return!0;const n=Ge(r.itemId,r.updates.frame);return n||Se().warn("Cannot move keyframe into transition region",{itemId:r.itemId,property:r.property,keyframeId:r.keyframeId,frame:r.updates.frame}),n});t.length!==0&&H.getState()._updateKeyframes(t)}function ZP(e){e.length!==0&&(()=>{const t=H.getState();for(const r of e){if(r.type==="vector-update"){t._updateVectorKeyframe(r.itemId,r.property,r.keyframeId,r.updates);continue}if(r.type==="vector-add"){if(!Ge(r.itemId,r.frame)){Se().warn("Cannot add vector auto keyframe in transition region",{itemId:r.itemId,property:r.property,frame:r.frame});continue}t._upsertVectorKeyframe(r.itemId,r.property,{frame:r.frame,value:r.value,easing:r.easing});continue}if(r.type==="update"){t._updateKeyframe(r.itemId,r.property,r.keyframeId,r.updates);continue}if(!Ge(r.itemId,r.frame)){Se().warn("Cannot add auto keyframe in transition region",{itemId:r.itemId,property:r.property,frame:r.frame});continue}t._addKeyframe(r.itemId,r.property,r.frame,r.value,r.easing)}})()}function eM(e,t,r){H.getState()._removeKeyframe(e,t,r)}function tM(e){H.getState()._removeKeyframesForItem(e)}function rM(e,t){H.getState()._removeKeyframesForProperty(e,t)}function nM(e,t){H.getState()._removeKeyframesByApplication(e,t)}function aM(e){H.getState()._removeManualKeyframes(e)}function iM(e){return(()=>{const t=I.getState().itemById[e],r=H.getState(),n=r.getKeyframesForItem(e);if(!t||!n)return 0;const a=Fw(n,t.durationInFrames,W.getState().fps);return a.removedCount===0?0:(r.setKeyframes(r.keyframes.map(i=>i.itemId===e?a.itemKeyframes:i)),a.removedCount)})()}function oM(e){e.length!==0&&H.getState()._removeKeyframes(e)}function Vf(){return Oe("MotionModifierActions")}function sM(e){return e.length===0?0:(()=>{const t=I.getState();let r=0;for(const{itemId:n,layer:a}of e){const i=t.itemById[n];i&&(t._updateItem(n,{motionLayers:[...i.motionLayers??[],a]}),r+=1)}return r})()}function lM(e,t){return e.length===0?0:(()=>{const r=I.getState();let n=0;for(const a of e){const i=r.itemById[a];i?.motionLayers?.some(o=>o.id===t)&&(r._updateItem(a,{motionLayers:i.motionLayers.filter(o=>o.id!==t)}),n+=1)}return n})()}function cM(e,t){return[...(e??[]).filter(n=>n.type!==t.type),t]}function uM(e){if(e.length===0)return 0;const t=Vf().startEvent("applyMotionModifiers",Lt());t.merge({requested:e.length,modifierTypes:[...new Set(e.map(r=>r.modifier.type))]});try{const r=(()=>{const n=I.getState();let a=0;for(const{itemId:i,modifier:o}of e){const s=n.itemById[i];s&&(n._updateItem(i,{motionModifiers:cM(s.motionModifiers,o)}),a+=1)}return a})();return t.success({updated:r}),r}catch(r){throw t.failure(r),r}}function dM(e){if(e.length===0)return 0;const t=Vf().startEvent("bakeMotionToKeyframes",Lt());t.merge({plannedItems:e.length,plannedKeyframes:e.reduce((r,n)=>r+n.keyframes.length,0),clearedProperties:e.reduce((r,n)=>r+n.clearProperties.length,0)});try{let r=0,n=0;const a=(()=>{const i=I.getState(),o=H.getState();let s=0;for(const l of e){const c=i.itemById[l.itemId];if(!c)continue;for(const m of l.clearProperties)o._removeKeyframesForProperty(l.itemId,m);const u=l.keyframes.filter(m=>Ge(m.itemId,m.frame));n+=l.keyframes.length-u.length,u.length>0&&(o._addKeyframes(u),r+=u.length);const d={};if(l.clearMotionModifiers&&(d.motionModifiers=[]),l.clearMotionLayers&&(d.motionLayers=[]),l.clearAudioPulseEffectIds.length>0&&c.effects){const m=new Set(l.clearAudioPulseEffectIds);d.effects=c.effects.map(p=>m.has(p.id)?{...p,audioPulse:void 0}:p)}Object.keys(d).length>0&&i._updateItem(l.itemId,d),s+=1}return s})();return t.success({baked:a,addedKeyframes:r,blockedKeyframes:n}),a}catch(r){throw t.failure(r),r}}function mM(e,t){return e.length===0?0:(()=>{const r=I.getState();let n=0;for(const a of e){const i=r.itemById[a];i?.motionModifiers?.some(o=>o.type===t)&&(r._updateItem(a,{motionModifiers:i.motionModifiers.filter(o=>o.type!==t)}),n+=1)}return n})()}function pM(e){return e.length===0?0:(()=>{const t=I.getState();let r=0;for(const n of e){const a=t.itemById[n];a?.effects?.some(i=>i.audioPulse?.enabled)&&(t._updateItem(n,{effects:a.effects.map(i=>i.audioPulse?{...i,audioPulse:void 0}:i)}),r+=1)}return r})()}function Ru(){return Oe("TextMotionActions")}const fM={in:Pp,out:Mp,loop:Cp};function hM(e,t){return fM[e].includes(t.presetId)}function Wf(e,t,r){return{...e,[t]:r}}function gM(e,t){if(!e)return;const r={...e};if(delete r[t],!(!r.in&&!r.out&&!r.loop))return r}function yM(e,t,r){if(e.length===0)return 0;if(!hM(t,r))return Ru().warn(`applyTextMotionEffect: preset '${r.presetId}' is not a '${t}' preset`),0;const n=Ru().startEvent("applyTextMotionEffect",Lt());n.merge({requested:e.length,slot:t,presetId:r.presetId});try{const a=(()=>{const i=I.getState();let o=0;for(const s of e){const l=i.itemById[s];!l||l.type!=="text"||(i._updateItem(s,{textMotion:Wf(l.textMotion,t,r)}),o+=1)}return o})();return n.success({updated:a}),a}catch(a){throw n.failure(a),a}}function vM(e,t,r){if(e.length===0)return;const n=I.getState();for(const a of e){const i=n.itemById[a];if(!i||i.type!=="text")continue;const o=i.textMotion?.[t];o&&n._updateItem(a,{textMotion:Wf(i.textMotion,t,{...o,...r})})}}function bM(e,t){return e.length===0?0:(()=>{const r=I.getState();let n=0;for(const a of e){const i=r.itemById[a];!i||i.type!=="text"||!i.textMotion?.[t]||(r._updateItem(a,{textMotion:gM(i.textMotion,t)}),n+=1)}return n})()}function SM(e){return{...e,id:$(),...e.channelGains&&{channelGains:{...e.channelGains}}}}function IM(e){return{...e.in&&{in:{...e.in}},...e.out&&{out:{...e.out}},...e.loop&&{loop:{...e.loop}}}}function xM(e,t){const r=new Set(t.map(n=>n.type));return[...(e??[]).filter(n=>!r.has(n.type)),...t]}const wM={position:["x","y"],scale:["width","height"],anchor:["anchorX","anchorY"]};function kM(e){const{targetItemId:t,keyframes:r,anchorFrame:n,maxFrame:a,retimeFrame:i,source:o,timeScale:s}=e,l=new Map;let c=0,u=0;for(const d of r){const m=Math.max(0,Math.min(a,n+i(d.frame)));if(!Ge(t,m)){u+=1;continue}l.set(m,{...Uy(d,{id:$(),frame:m}),source:o,...d.temporalEase&&{temporalEase:{...d.temporalEase.in&&{in:{...d.temporalEase.in,speed:s>0?d.temporalEase.in.speed/s:d.temporalEase.in.speed}},...d.temporalEase.out&&{out:{...d.temporalEase.out,speed:s>0?d.temporalEase.out.speed/s:d.temporalEase.out.speed}}}}}),c+=1}return{keyframes:[...l.values()].sort((d,m)=>d.frame-m.frame),applied:c,skipped:u}}function PM(e,t,r,n,a){const i=new Map;for(const o of t)n&&a&&o.frame>=a.fromFrame&&o.frame<=a.toFrame||i.set(o.frame,o);for(const o of r)i.get(o.frame)||i.set(o.frame,o);return{property:e,keyframes:[...i.values()].sort((o,s)=>o.frame-s.frame)}}function MM(e,t,r=0,n={}){const a=I.getState().itemById[e];if(!a)return{applied:0,addedEffects:0,appliedProcedural:0,skipped:0,incompatible:!0,reason:"no-item"};const i=Zw(t,a);if(!i.compatible)return{applied:0,addedEffects:0,appliedProcedural:0,skipped:0,incompatible:!0,reason:i.reason};const s={applicationId:$(),kind:"saved-preset",presetId:t.id,presetName:t.name},l=new Set;for(const P of t.properties){const R=Gn(P.property);R&&l.add(R.gpuEffectType)}const c=new Map;for(const P of l){const R=a.effects?.find(E=>E.effect.gpuEffectType===P);R&&c.set(P,R.id)}const u=[];for(const P of l){if(c.has(P))continue;const R=t.effects.find(E=>E.gpuEffectType===P);R&&u.push({...R,params:{...R.params}})}let d=0;if(u.length>0){I.getState()._addEffects([{itemId:e,effects:u}]),d=u.length;const P=I.getState().itemById[e]?.effects??[];for(const R of l)if(!c.has(R))for(let E=P.length-1;E>=0;E--){const D=P[E];if(D&&D.effect.gpuEffectType===R){c.set(R,D.id);break}}}const m=I.getState().itemById[e]??a,p=new Set(wf(m)),f=Math.max(0,m.durationInFrames-1),h=Math.max(1,t.sourceDurationInFrames-1),g=Math.max(0,f-r),v=n.retime===!1?1:g/h,S=P=>n.retime===!1?P:Math.round(P*v),b=[];let x=0;for(const P of t.properties){let R=P.property;const E=Gn(P.property);if(E){const D=c.get(E.gpuEffectType);if(!D){x+=P.keyframes.length;continue}R=Gs(E.gpuEffectType,D,E.paramKey)}else if(!p.has(R)){x+=P.keyframes.length;continue}for(const D of P.keyframes){const L=Math.max(0,Math.min(f,r+S(D.frame)));if(!Ge(e,L)){x+=1;continue}b.push({itemId:e,property:R,frame:L,value:D.value,easing:D.easing,easingConfig:D.easingConfig,source:s})}}let T=0;if(b.length>0){const P=H.getState();let R=b;if(n.replace){const E=new Set(b.map(D=>D.property));for(const D of E){const L=b.filter(ee=>ee.property===D),U=Math.min(...L.map(ee=>ee.frame)),V=Math.max(...L.map(ee=>ee.frame)),ne=P.getKeyframesForItem(e)?.properties.find(ee=>ee.property===D)?.keyframes.filter(ee=>ee.frame>=U&&ee.frame<=V).map(ee=>({itemId:e,property:D,keyframeId:ee.id}))??[];ne.length>0&&P._removeKeyframes(ne)}}else{const E=P.getKeyframesForItem(e);R=b.filter(D=>!E?.properties.find(U=>U.property===D.property)?.keyframes.some(U=>U.frame===D.frame)),x+=b.length-R.length}T=P._addKeyframes(R).length}const C=H.getState();for(const P of t.vectorProperties??[]){const R=kM({targetItemId:e,keyframes:P.keyframes,anchorFrame:r,maxFrame:f,retimeFrame:S,source:s,timeScale:v});if(x+=R.skipped,R.keyframes.length===0)continue;const E=C.getKeyframesForItem(e)?.vectorProperties?.find(U=>U.property===P.property)?.keyframes??[],D=n.replace?R.keyframes:R.keyframes.filter(U=>!E.some(V=>V.frame===U.frame));if(x+=R.keyframes.length-D.length,D.length===0)continue;const L=n.replace===!0?{fromFrame:Math.min(...D.map(U=>U.frame)),toFrame:Math.max(...D.map(U=>U.frame))}:void 0;C._replaceScalarPropertiesWithVectorProperty(e,PM(P.property,E,D,n.replace===!0,L),wM[P.property]),T+=D.length}let k=0;const _=I.getState();if(t.motionModifiers&&t.motionModifiers.length>0){const P=t.motionModifiers.map(SM);_._updateItem(e,{motionModifiers:n.replace?P:xM(a.motionModifiers,P)}),k+=P.length}if(t.motionLayers&&t.motionLayers.length>0){const P=t.motionLayers.map(R=>({...eb(R,{freshIds:!0}),source:"saved-preset",sourcePresetId:t.id}));_._updateItem(e,{motionLayers:n.replace?P:[...a.motionLayers??[],...P]}),k+=P.length}if(t.textMotion&&a.type==="text"){const P=IM(t.textMotion);_._updateItem(e,{textMotion:n.replace?P:{...a.textMotion,...P}}),k+=+!!P.in,k+=+!!P.out,k+=+!!P.loop}return T>0||d>0||k>0||Se().warn("applyAnimationPreset produced no animation",{targetItemId:e,skipped:x}),{applied:T,addedEffects:d,appliedProcedural:k,skipped:x,incompatible:!1}}const G=w().min(1),Rt=M().int().nonnegative(),te=M().finite(),Ut=dr([z(["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius","cropLeft","cropRight","cropTop","cropBottom","cropSoftness","volume","textStyleScale","fontSize","lineHeight","textPadding","backgroundRadius","textShadowOffsetX","textShadowOffsetY","textShadowBlur","strokeWidth","trimPathStart","trimPathEnd","trimPathOffset","taperStartWidth","taperEndWidth","taperStartLength","taperEndLength"]),w().regex(/^effect:[^:]+:[^:]+:[^:]+$/),w().regex(/^pathVertex:\d+:(positionX|positionY|inX|inY|outX|outY)$/)]),It=z(["position","scale","anchor"]),un=w().refine(Rm),jf=w().refine(Li),gr=z(["linear","ease-in","ease-out","ease-in-out","hold","cubic-bezier","spring"]),ti=y({x:te,y:te}),$f=y({type:gr,bezier:y({x1:te.min(0).max(1),y1:te,x2:te.min(0).max(1),y2:te}).optional(),spring:y({tension:te.nonnegative(),friction:te.nonnegative(),mass:te.positive()}).optional()}),Yf=y({applicationId:G,kind:z(["built-in-preset","saved-preset"]),presetId:G,presetName:w()}),bl=y({frame:Rt,value:te,easing:gr.optional(),easingConfig:$f.optional(),source:Yf.optional()}),Ln=y({frame:Rt,value:ti,easing:gr.optional(),easingConfig:$f.optional(),temporalEase:y({in:y({speed:te,influence:te.min(0).max(100)}).optional(),out:y({speed:te,influence:te.min(0).max(100)}).optional()}).optional(),spatial:y({inTangent:ti,outTangent:ti,continuous:B().optional()}).optional(),source:Yf.optional()}),xs=bl.extend({id:G}),Xf=Ln.extend({id:G,easing:gr}),Dr=y({itemId:G,property:Ut,keyframeId:G}),Bn=bl.extend({itemId:G,property:Ut}),Lu=Dr.extend({updates:bl.partial(),clearFields:F(z(["easingConfig","source"])).optional()}),CM=Oi("type",[Bn.extend({type:Z("add")}),Dr.extend({type:Z("update"),updates:y({value:te.optional()})}),Ln.extend({type:Z("vector-add"),itemId:G,property:It}),y({type:Z("vector-update"),itemId:G,property:It,keyframeId:G,updates:y({value:ti})})]);function Ze(e){if(!I.getState().itemById[e])throw new Error(`Không tìm thấy clip: ${e}`)}function X(e,t,r,n,a){A({type:e,params:t,validate:a,mergeKey:n,apply:(i,o)=>({inverse:{kind:"patch",patch:{}},result:r(o)})})}function Oo(e,t=[]){return{...e,...Object.fromEntries(t.map(r=>[r,void 0]))}}function TM(){X("keyframe.add",Bn,r=>(Ze(r.itemId),_o([r])[0]??"")),X("keyframe.addMany",y({payloads:F(Bn)}),r=>{for(const n of r.payloads)Ze(n.itemId);return _o(r.payloads)}),X("keyframe.update",Lu,r=>JP(r.itemId,r.property,r.keyframeId,Oo(r.updates,r.clearFields)),r=>`keyframe:${r.itemId}:${r.property}:${r.keyframeId}`),X("keyframe.updateMany",y({updates:F(Lu)}),r=>Du(r.updates.map(n=>({...n,updates:Oo(n.updates,n.clearFields)}))),r=>`keyframes:${r.updates.map(n=>`${n.itemId}:${n.property}:${n.keyframeId}`).sort().join(",")}`),X("keyframe.move",y({moves:F(y({ref:Dr,newFrame:Rt}))}),r=>Du(r.moves.map(n=>({...n.ref,updates:{frame:n.newFrame}}))),r=>`keyframes:${r.moves.map(n=>`${n.ref.itemId}:${n.ref.property}:${n.ref.keyframeId}`).sort().join(",")}`),X("keyframe.remove",Dr,r=>eM(r.itemId,r.property,r.keyframeId)),X("keyframe.removeMany",y({refs:F(Dr)}),r=>oM(r.refs)),X("keyframe.removeForItem",y({itemId:G}),r=>tM(r.itemId)),X("keyframe.removeForProperty",y({itemId:G,property:Ut}),r=>rM(r.itemId,r.property)),X("keyframe.removePresetApplication",y({itemId:G,applicationId:G}),r=>nM(r.itemId,r.applicationId)),X("keyframe.removeManual",y({itemId:G}),r=>aM(r.itemId)),X("keyframe.trimToBounds",y({itemId:G}),r=>iM(r.itemId)),X("keyframe.auto",y({operations:F(CM)}),r=>{for(const n of r.operations)Ze(n.itemId);return ZP(r.operations)}),X("keyframe.vectorUpsert",y({itemId:G,property:It,input:Ln}),r=>(Ze(r.itemId),VP(r.itemId,r.property,r.input))),X("keyframe.vectorUpdate",y({itemId:G,property:It,keyframeId:G,updates:Ln.partial(),clearFields:F(z(["easingConfig","temporalEase","spatial","source"])).optional()}),r=>WP(r.itemId,r.property,r.keyframeId,Oo(r.updates,r.clearFields)),r=>`vector:${r.itemId}:${r.property}:${r.keyframeId}`),X("keyframe.vectorRemove",y({itemId:G,property:It,keyframeId:G}),r=>jP(r.itemId,r.property,r.keyframeId));const e=y({property:It,keyframes:F(Xf)}),t=y({property:Ut,keyframes:F(xs.extend({easing:gr}))});X("keyframe.vectorPromote",y({itemId:G,vectorProperty:e,removeScalarProperties:F(jf)}),r=>(Ze(r.itemId),$P(r.itemId,r.vectorProperty,r.removeScalarProperties))),X("keyframe.vectorSeparate",y({itemId:G,property:It,separated:B(),conversion:y({scalarProperties:F(t).optional(),vectorProperty:e.optional()}).optional()}),r=>(Ze(r.itemId),YP(r.itemId,r.property,r.separated,r.conversion))),X("keyframe.setLink",y({itemId:G,link:y({type:Z("link"),targetProperty:un,sourceItemId:G,sourceProperty:un,enabled:B(),timeOffsetFrames:M().int()})}),r=>(Ze(r.itemId),Ze(r.link.sourceItemId),NP(r.itemId,r.link))),X("keyframe.removeLink",y({itemId:G,property:un}),r=>qP(r.itemId,r.property)),X("keyframe.setExpression",y({itemId:G,expression:y({type:Z("expression"),targetProperty:un,source:w(),enabled:B()})}),r=>(Ze(r.itemId),GP(r.itemId,r.expression))),X("keyframe.removeExpression",y({itemId:G,property:un}),r=>KP(r.itemId,r.property)),X("keyframe.applyMotionPreset",y({payloads:F(Bn),clearProperties:F(y({itemId:G,property:Ut,fromFrame:Rt.optional(),toFrame:Rt.optional()})).default([]),vectorApplies:F(y({itemId:G,property:It,keyframes:F(Ln),replaceRange:y({fromFrame:Rt,toFrame:Rt}).optional()})).default([])}),r=>{for(const n of[...r.payloads.map(a=>a.itemId),...r.vectorApplies.map(a=>a.itemId)])Ze(n);return QP(r.payloads,r.clearProperties,r.vectorApplies)}),X("keyframe.duplicate",y({refs:F(Dr),frameOffset:M().int(),targetItemId:G.optional(),targetProperty:Ut.optional()}),r=>{const n=H.getState(),a=r.refs.flatMap(i=>{const o=n.getKeyframeById(i.itemId,i.property,i.keyframeId);return o?[{...o,itemId:r.targetItemId??i.itemId,property:r.targetProperty??i.property,frame:o.frame+r.frameOffset}]:[]});if(a.some(i=>i.frame<0))throw new Error("Khung hình keyframe không được âm");for(const i of a)Ze(i.itemId);return _o(a)}),EM()}function EM(){const e=z(["float-drift","breath-pulse","micro-shake","sway","spin","wander"]),t=y({version:Z(2).optional(),id:G,type:e,enabled:B(),amplitude:te,frequency:te,phaseFrames:M().int(),seed:te,channelGains:y({x:te.optional(),y:te.optional(),width:te.optional(),height:te.optional(),rotation:te.optional(),opacity:te.optional()}).optional()}),r=y({id:G,name:w(),enabled:B(),source:z(["built-in-preset","saved-preset"]),sourcePresetId:G,tracks:F(y({property:jf,blend:z(["add","multiply"]),keyframes:F(xs.extend({easing:gr}))}))}),n=F(y({itemId:G,modifier:t}));X("keyframe.motionLayersAdd",y({assignments:F(y({itemId:G,layer:r}))}),d=>sM(d.assignments)),X("keyframe.motionLayerRemove",y({itemIds:F(G),layerId:G}),d=>lM(d.itemIds,d.layerId)),X("keyframe.motionModifiersSet",y({assignments:n,gestureId:G.optional()}),d=>uM(d.assignments),d=>d.gestureId?`motion:${d.gestureId}`:null),X("keyframe.motionModifierRemove",y({itemIds:F(G),type:e}),d=>mM(d.itemIds,d.type)),X("keyframe.audioPulseRemove",y({itemIds:F(G)}),d=>pM(d.itemIds)),X("keyframe.motionBake",y({plan:F(y({itemId:G,keyframes:F(Bn),clearProperties:F(Ut),clearMotionModifiers:B(),clearMotionLayers:B(),clearAudioPulseEffectIds:F(G)}))}),d=>dM(d.plan));const a=z(["in","out","loop"]),i=y({durationFrames:te.nonnegative(),offsetFrames:te.nonnegative().optional(),staggerFrames:te.nonnegative(),intensity:te,order:z(["forward","backward","center","random"]),easing:z(["linear","ease-in","ease-out","ease-in-out","overshoot"]),seed:te,unit:z(["character","word","line","whole-clip"]).optional()}),o=i.extend({presetId:z(Pp)}),s=i.extend({presetId:z(Mp)}),l=i.extend({presetId:z(Cp)}),c=dr([o,s,l]);X("keyframe.textMotionSet",y({itemIds:F(G),slot:a,effect:c}),d=>yM(d.itemIds,d.slot,d.effect)),X("keyframe.textMotionUpdate",y({itemIds:F(G),slot:a,partial:i.partial(),gestureId:G.optional()}),d=>vM(d.itemIds,d.slot,d.partial),d=>d.gestureId?`text-motion:${d.gestureId}`:null),X("keyframe.textMotionRemove",y({itemIds:F(G),slot:a}),d=>bM(d.itemIds,d.slot));const u=y({id:G,name:w(),sourceItemType:z(["video","audio","image","lottie","text","shape","adjustment","composition","controller","subtitle"]),properties:F(y({property:Ut,keyframes:F(xs.extend({easing:gr}))})),vectorProperties:F(y({property:It,keyframes:F(Xf)})).optional(),effects:F(y({type:Z("gpu-effect"),gpuEffectType:G,params:kt(w(),dr([te,B(),w()]))})),motionModifiers:F(t).optional(),motionLayers:F(r).optional(),textMotion:y({in:o.optional(),out:s.optional(),loop:l.optional()}).optional(),sourceDurationInFrames:M().int().positive(),createdAt:te});X("keyframe.applyPreset",y({targetItemId:G,preset:u,anchorFrame:Rt.default(0),options:y({replace:B().optional(),retime:B().optional()}).default({})}),d=>MM(d.targetItemId,d.preset,d.anchorFrame,d.options))}function FM(e,t,r="crossfade",n,a,i,o=.5){return(()=>{const s=I.getState().items,l=q.getState().transitions,c=s.find(x=>x.id===e),u=s.find(x=>x.id===t);if(!c||!u)return Se().warn("[addTransition] Clips not found"),!1;const d=Math.floor(Math.min(c.durationInFrames,u.durationInFrames)-1);if(d<1)return Se().warn("[addTransition] Cannot add transition: clips are too short"),!1;const m=Kp[r],p=n??m.defaultDuration;let f=Math.max(1,Math.min(Math.round(p),d));const h=c.from+c.durationInFrames,g=Qr(h,u.from),v=W.getState().fps;if(g){const x=ff(c,u,o,v);if(x<1)return Se().warn("[addTransition] Cannot add transition: insufficient source handle at cut"),!1;f=Math.min(f,x)}const S=ua(c,u,f,o,v);return S.canAdd?l.find(x=>x.leftClipId===e&&x.rightClipId===t)?(Se().warn("[addTransition] Transition already exists between these clips"),!1):(q.getState()._addTransition(e,t,c.trackId,r,f,a,i,o),!0):(Se().warn("[addTransition] Cannot add transition:",S.reason),!1)})()}function Qf(e,t){const n=q.getState().transitions.find(l=>l.id===e);if(!n)return!1;const a=I.getState().items,i=a.find(l=>l.id===n.leftClipId),o=a.find(l=>l.id===n.rightClipId),s={...n,...t};if(i&&o){const l=ua(i,o,s.durationInFrames,s.alignment,W.getState().fps);if(!l.canAdd)return Se().warn("[updateTransition] Cannot update transition:",l.reason),!1}return q.getState()._updateTransition(e,t),!0}function Bu(e,t){Qf(e,t)}function AM(e){e.length!==0&&(()=>{for(const{id:t,updates:r}of e){if(r.durationInFrames!==void 0||r.alignment!==void 0){const n=q.getState().transitions.find(a=>a.id===t);if(n&&(r.durationInFrames!==void 0&&r.durationInFrames!==n.durationInFrames||r.alignment!==void 0&&r.alignment!==n.alignment)){Qf(t,r);continue}}q.getState()._updateTransition(t,r)}})()}function _M(e){q.getState()._removeTransition(e)}const et=w().min(1),Cn=M().finite(),Jf=z(["from-left","from-right","from-top","from-bottom"]),Zf=Cn.min(0).max(1),Do=y({durationInFrames:M().int().positive().optional(),type:Z("crossfade").optional(),presentation:et.optional(),direction:Jf.optional(),timing:z(["linear","ease-in","ease-out","ease-in-out","cubic-bezier"]).optional(),alignment:Zf.optional(),bezierPoints:y({x1:Cn.min(0).max(1),y1:Cn,x2:Cn.min(0).max(1),y2:Cn}).optional(),presetId:et.optional(),properties:kt(w(),_i()).optional()});function wr(e,t,r,n){A({type:e,params:t,mergeKey:n,apply:(a,i)=>({inverse:{kind:"patch",patch:{}},result:r(i)})})}function OM(){wr("transition.add",y({leftClipId:et,rightClipId:et,type:Z("crossfade").default("crossfade"),durationInFrames:M().int().positive().optional(),presentation:et.optional(),direction:Jf.optional(),alignment:Zf.default(.5),timing:Do.shape.timing}),e=>{const t=FM(e.leftClipId,e.rightClipId,e.type,e.durationInFrames,e.presentation,e.direction,e.alignment);if(t&&e.timing){const r=q.getState().transitions.find(n=>n.leftClipId===e.leftClipId&&n.rightClipId===e.rightClipId);r&&Bu(r.id,{timing:e.timing})}return t}),wr("transition.update",y({id:et,updates:Do}),e=>Bu(e.id,e.updates),e=>`transition:${e.id}`),wr("transition.updateMany",y({updates:F(y({id:et,updates:Do}))}),e=>AM(e.updates),e=>`transitions:${e.updates.map(t=>t.id).sort().join(",")}`),wr("transition.remove",y({id:et}),e=>_M(e.id)),wr("transition.removeMany",y({ids:F(et)}),e=>q.getState()._removeTransitions(e.ids)),wr("transition.repair",y({changedClipIds:F(et),deletedClipIds:F(et).optional()}),e=>{const{valid:t,repaired:r,broken:n}=gf(e.changedClipIds,I.getState().items,q.getState().transitions,e.deletedClipIds?new Set(e.deletedClipIds):void 0,W.getState().fps);return q.getState().setTransitions([...t,...r.map(a=>a.repaired)]),{repaired:r,broken:n}})}const ri=y({type:Z("gpu-effect"),gpuEffectType:w().min(1),params:kt(w(),dr([M().finite(),B(),w()]))}).strict(),DM=y({id:w().min(1),effect:ri,enabled:B(),audioPulse:y({enabled:B(),beats:F(y({frame:M().finite(),amplitude:M().finite()}).strict()),durationFrames:M().finite(),strength:M().finite(),chroma:M().finite(),glowColorBase:M().finite()}).strict().optional()}).strict(),zn=w().min(1),ws=y({itemId:zn}).strict(),Ro=ws.extend({effectId:zn}),Qt=()=>({kind:"patch",patch:{}});function Tn(e,t){return t.flatMap(r=>{const n=e.items.find(a=>a.id===r);return n?n.type==="audio"?[{message:"Clip âm thanh không hỗ trợ hiệu ứng hình ảnh",path:"itemId"}]:[]:[{message:`Không tìm thấy clip ${r}`,path:"itemId"}]})}function Lo(e,t){const r=Tn(e,[t.itemId]);return r.length?r:e.items.find(n=>n.id===t.itemId)?.effects?.some(n=>n.id===t.effectId)?[]:[{message:`Không tìm thấy hiệu ứng ${t.effectId}`,path:"effectId"}]}function dn(e,t,r){e.draft.items=e.draft.items.map(n=>n.id===t&&n.type!=="audio"?r(n):n)}function RM(){A({type:"effect.add",params:ws.extend({effect:ri,effectId:zn.optional()}),validate:(e,t)=>Tn(e,[t.itemId]),apply(e,t){const r=t.effectId??e.ids.mint();return dn(e,t.itemId,n=>({...n,effects:[...n.effects??[],{id:r,effect:t.effect,enabled:!0}]})),{inverse:Qt(),result:r}}}),A({type:"effect.addMany",params:y({updates:F(y({itemId:zn,effects:F(ri)}).strict())}).strict(),validate:(e,t)=>Tn(e,t.updates.map(r=>r.itemId)),apply(e,t){const r=new Map(t.updates.map(n=>[n.itemId,n.effects]));return e.draft.items=e.draft.items.map(n=>{const a=r.get(n.id);return!a||n.type==="audio"?n:{...n,effects:[...n.effects??[],...a.map(i=>({id:e.ids.mint(),effect:i,enabled:!0}))]}}),Qt()}}),A({type:"effect.update",params:Ro.extend({updates:y({effect:ri.optional(),enabled:B().optional()}).strict()}),validate:(e,t)=>Lo(e,t),mergeKey:e=>`effect:${e.itemId}:${e.effectId}:${[...Object.keys(e.updates).filter(t=>t!=="effect"),...Object.keys(e.updates.effect?.params??{}).map(t=>`${e.updates.effect?.gpuEffectType}:${t}`)].sort().join(",")}`,apply(e,t){return dn(e,t.itemId,r=>({...r,effects:(r.effects??[]).map(n=>n.id===t.effectId?{...n,...t.updates}:n)})),Qt()}}),A({type:"effect.remove",params:Ro,validate:Lo,apply(e,t){return dn(e,t.itemId,r=>({...r,effects:(r.effects??[]).filter(n=>n.id!==t.effectId)})),Qt()}}),A({type:"effect.toggle",params:Ro,validate:Lo,apply(e,t){return dn(e,t.itemId,r=>({...r,effects:(r.effects??[]).map(n=>n.id===t.effectId?{...n,enabled:!n.enabled}:n)})),Qt()}}),A({type:"effect.setItems",params:y({updates:F(y({itemId:zn,effects:F(DM)}).strict())}).strict(),validate:(e,t)=>Tn(e,t.updates.map(r=>r.itemId)),apply(e,t){const r=new Map(t.updates.map(n=>[n.itemId,n.effects]));return e.draft.items=e.draft.items.map(n=>r.has(n.id)&&n.type!=="audio"?{...n,effects:r.get(n.id)}:n),Qt()}}),A({type:"effect.setBlendMode",params:ws.extend({blendMode:z(Object.keys(pw))}),validate:(e,t)=>Tn(e,[t.itemId]),apply(e,t){return dn(e,t.itemId,r=>({...r,blendMode:r.type==="shape"&&r.isMask?"normal":t.blendMode})),Qt()}})}const J=M().finite(),Bo=w().min(1),eh=y({fontSize:J.optional(),fontFamily:w().optional(),fontWeight:z(["normal","medium","semibold","bold"]).optional(),fontStyle:z(["normal","italic"]).optional(),underline:B().optional(),color:w().optional(),letterSpacing:J.optional()}).strict(),Ha=eh.extend({text:w()}),ks=eh.extend({backgroundColor:w().optional(),backgroundRadius:J.optional(),textAlign:z(["left","center","right"]).optional(),verticalAlign:z(["top","middle","bottom"]).optional(),lineHeight:J.optional(),textPadding:J.optional(),textShadow:y({offsetX:J,offsetY:J,blur:J,color:w()}).strict().optional(),stroke:y({width:J,color:w()}).strict().optional()}),zu=ks.extend({text:w().optional(),textSpans:F(Ha).optional(),spanLayout:z(["stack","inline"]).optional(),textLayoutDrafts:y({single:Ha.optional(),twoSpans:F(Ha).optional(),threeSpans:F(Ha).optional()}).strict().optional(),textStylePresetId:w().optional(),textStyleScale:J.optional()}),ni=An([J,J]),Uu=y({shapeType:z(["rectangle","circle","triangle","ellipse","star","polygon","heart","path"]).optional(),fillColor:w().optional(),fillEnabled:B().optional(),fillType:z(["solid","linear"]).optional(),gradientStartColor:w().optional(),gradientEndColor:w().optional(),gradientAngle:J.optional(),strokeColor:w().optional(),strokeWidth:J.optional(),strokeEnabled:B().optional(),strokeLineCap:z(["butt","round","square"]).optional(),strokeLineJoin:z(["miter","round","bevel"]).optional(),strokeMiterLimit:J.optional(),trimPathStart:J.optional(),trimPathEnd:J.optional(),trimPathOffset:J.optional(),taperStartWidth:J.optional(),taperEndWidth:J.optional(),taperStartLength:J.optional(),taperEndLength:J.optional(),cornerRadius:J.optional(),direction:z(["up","down","left","right"]).optional(),points:J.optional(),innerRadius:J.optional(),pathVertices:F(y({position:ni,inHandle:ni,outHandle:ni,tangentMode:z(["corner","smooth","continuous","broken"]).optional()}).strict()).optional(),pathClosed:B().optional(),isMask:B().optional(),maskType:z(["clip","alpha"]).optional(),maskFeather:J.optional(),maskOpacity:J.optional(),maskInvert:B().optional()}).strict(),Hu=y({frameRate:J.optional(),totalFrames:J.optional(),loop:B().optional(),reversed:B().optional(),loopMode:z(["loop","pingpong"]).optional(),speed:J.optional(),segmentStart:J.optional(),segmentEnd:J.optional(),animationId:w().optional(),themeId:w().optional(),textOverrides:kt(w(),w()).optional(),colorOverrides:kt(w(),w()).optional(),slotOverrides:kt(w(),dr([J,ni])).optional()}).strict();function zo(e,t,r){const n=e.items.find(a=>a.id===t);return n?n.type!==r?[{message:`Clip không phải loại ${r}`,path:"itemId"}]:[]:[{message:`Không tìm thấy clip ${t}`,path:"itemId"}]}function Uo(e,t,r,n){const a={...r};for(const i of n)a[i]=void 0;return e.draft.items=e.draft.items.map(i=>i.id===t?fe({...i,...a}):i),{kind:"patch",patch:{}}}function Ho(e,t){return Object.keys(e.shape).filter(r=>!t.includes(r))}function LM(){A({type:"text.update",params:y({itemId:Bo,updates:zu,clearFields:F(z(Ho(zu,["text","color"]))).default([])}).strict(),validate:(e,t)=>zo(e,t.itemId,"text"),mergeKey:e=>`text:${e.itemId}:${[...Object.keys(e.updates),...e.clearFields].sort().join(",")}`,apply:(e,t)=>Uo(e,t.itemId,t.updates,t.clearFields)}),A({type:"text.updateShape",params:y({itemId:Bo,updates:Uu,clearFields:F(z(Ho(Uu,["shapeType","fillColor"]))).default([])}).strict(),validate:(e,t)=>zo(e,t.itemId,"shape"),mergeKey:e=>`shape:${e.itemId}:${[...Object.keys(e.updates),...e.clearFields].sort().join(",")}`,apply:(e,t)=>Uo(e,t.itemId,t.updates,t.clearFields)}),A({type:"text.updateLottie",params:y({itemId:Bo,updates:Hu,clearFields:F(z(Ho(Hu,["frameRate","totalFrames"]))).default([])}).strict(),validate:(e,t)=>zo(e,t.itemId,"lottie"),mergeKey:e=>`lottie:${e.itemId}:${[...Object.keys(e.updates),...e.clearFields].sort().join(",")}`,apply:(e,t)=>Uo(e,t.itemId,t.updates,t.clearFields)})}const Na="__virtual-transcript-captions__";function BM(e,t){const r=e.speed??1,n=e.sourceStart??0,a=e.sourceFps??t,i=n+we(e.durationInFrames,r,t,a);return{sourceStart:n,sourceEnd:e.sourceEnd??i,sourceFps:a,speed:r}}function zM(e){const t=e.reduce((r,n)=>Math.max(r,n.order),-1);return{id:`track-captions-${Date.now()}`,name:Bs([...e],"video"),kind:"video",height:yr,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,volume:0,order:t+1,items:[]}}function UM(e,t,r="transcript",n){return{type:r,mediaId:e,clipId:t,...n}}function HM({trackId:e,cues:t,timelineFps:r,canvasWidth:n,canvasHeight:a,fileName:i,format:o,startFrame:s=0,styleTemplate:l,sourceType:c="subtitle-import",idFactory:u=()=>crypto.randomUUID(),importedAt:d=Date.now()}){return t.flatMap(m=>{const p=s+Math.max(0,Math.round(m.startSeconds*r)),f=Math.max(p+1,s+Math.round(m.endSeconds*r)),h=Math.max(1,f-p);return[{...{id:u(),type:"text",textRole:"caption",trackId:e,from:p,durationInFrames:h,mediaId:"",captionSource:UM("","",c,{fileName:i,format:o,importedAt:d}),label:m.text.slice(0,48),text:m.text,fontSize:Math.max(36,Math.round(a*.045)),fontFamily:"Inter",fontWeight:"semibold",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:"rgba(0, 0, 0, 0.55)",textAlign:"center",verticalAlign:"middle",lineHeight:1.15,letterSpacing:0,textShadow:{offsetX:0,offsetY:3,blur:10,color:"rgba(0, 0, 0, 0.75)"},transform:{x:0,y:Math.round(a*.32),width:n*.82,height:a*.16,rotation:0,opacity:1}},...l}]})}function NM(e){const{clip:t,cues:r,timelineFps:n,canvasWidth:a,canvasHeight:i,trackId:o,source:s,styleTemplate:l,label:c}=e,{sourceStart:u,sourceEnd:d,sourceFps:m,speed:p}=BM(t,n),f=u/m,h=d/m,g=[];let v=Number.POSITIVE_INFINITY,S=0;for(const P of r){const R=Math.max(P.startSeconds,f),E=Math.min(P.endSeconds,h);if(E<=R)continue;const D=(R-f)/p,L=(E-f)/p,U=Math.floor(D*n),V=Math.ceil(L*n);V<=U||(g.push({id:P.id,startSeconds:D,endSeconds:L,text:P.text}),U<v&&(v=U),V>S&&(S=V))}if(g.length===0)return null;const b=Math.max(0,Math.min(v,t.durationInFrames-1)),x=Math.min(t.durationInFrames,Math.max(S,b+1)),T=t.from+b,C=Math.max(1,x-b),k=g.map(P=>({id:P.id,startSeconds:P.startSeconds-b/n,endSeconds:P.endSeconds-b/n,text:P.text})),_={fontSize:Math.max(36,Math.round(i*.045)),fontFamily:"Inter",fontWeight:"semibold",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:"rgba(0, 0, 0, 0.55)",textAlign:"center",verticalAlign:"middle",lineHeight:1.15,letterSpacing:0,textShadow:{offsetX:0,offsetY:3,blur:10,color:"rgba(0, 0, 0, 0.75)"},transform:{x:0,y:Math.round(i*.32),width:a*.82,height:i*.16,rotation:0,opacity:1}};return{id:crypto.randomUUID(),type:"subtitle",trackId:o,from:T,durationInFrames:C,label:c??(s.type==="embedded-subtitles"?s.trackName??s.language??"Subtitles":s.type==="subtitle-import"?s.fileName:"Transcript"),mediaId:t.mediaId,linkedGroupId:t.linkedGroupId,sourceLabel:c,source:s,cues:k,..._,...l}}function IE(e,t,r,n){const a=e.filter(m=>m.id!==Na),i=new Set(a.flatMap(m=>(m.items??[]).filter(p=>(p.type==="video"||p.type==="audio")&&p.isReversed===!0).map(p=>p.id))),o=a.map(m=>{const p=(m.items??[]).filter(f=>f.type==="text"&&f.captionSource?.clipId?!i.has(f.captionSource.clipId):f.type==="subtitle"&&(f.source.type==="transcript"||f.source.type==="embedded-subtitles")?!i.has(f.source.clipId):!0);return p.length===(m.items??[]).length?m:{...m,items:p}}),s=o.some(m=>m.solo),l=o.filter(m=>s?m.solo===!0:m.visible!==!1),c=[];for(const m of l)for(const p of m.items??[]){if(p.type!=="video"&&p.type!=="audio"||!p.mediaId||p.isReversed===!0)continue;const f=p.transcriptCaptions;if(!f?.enabled||f.type!=="transcript"||f.cues.length===0)continue;const h=NM({trackId:Na,cues:f.cues,clip:p,timelineFps:t,canvasWidth:r,canvasHeight:n,label:"Transcript",source:{type:"transcript",mediaId:f.mediaId||p.mediaId,clipId:p.id},styleTemplate:f.style});h&&c.push({...h,id:`virtual-transcript-${p.id}`,trackId:Na,linkedGroupId:void 0})}if(c.length===0)return o;const u=o.length>0?Math.min(...o.map(m=>m.order??0)):0,d={id:Na,name:"Transcript Captions",kind:"video",height:1,locked:!0,syncLock:!1,visible:!0,muted:!0,solo:s,volume:0,order:u-1,items:c};return[...o,d]}const Cr=w().min(1),Rr=M().finite(),Nu=y({id:Cr,startSeconds:Rr.nonnegative(),endSeconds:Rr.nonnegative(),text:w()}).strict().refine(e=>e.endSeconds>e.startSeconds,{message:"Thời điểm kết thúc phải sau thời điểm bắt đầu",path:["endSeconds"]}),qa=()=>({kind:"patch",patch:{}});function No(e,t){const r=e.items.find(n=>n.id===t.itemId);return r?r.type!=="subtitle"?[{message:"Clip không phải đoạn phụ đề",path:"itemId"}]:[]:[{message:`Không tìm thấy phụ đề ${t.itemId}`,path:"itemId"}]}function qM(){A({type:"subtitle.import",params:y({trackId:Cr,fileName:w().min(1),format:z(["srt","vtt"]),content:w(),startFrame:Rr.nonnegative().default(0),importedAt:Rr.default(0),createTrack:B().default(!1),style:ks.optional()}).strict(),validate(e,t){const r=[],n=e.tracks.find(i=>i.id===t.trackId);return!n&&!t.createTrack&&r.push({message:`Không tìm thấy rãnh ${t.trackId}`,path:"trackId"}),(n?.locked||n?.isGroup||n?.kind==="audio")&&r.push({message:"Rãnh này không nhận phụ đề",path:"trackId"}),(t.format==="srt"?Al(t.content):_l(t.content)).cues.length||r.push({message:"Tệp không có phụ đề hợp lệ",path:"content"}),r},apply(e,t){const r=t.format==="srt"?Al(t.content):_l(t.content);e.draft.tracks.some(a=>a.id===t.trackId)||(e.draft.tracks=[...e.draft.tracks,{...zM(e.draft.tracks),id:t.trackId}]);const n=HM({trackId:t.trackId,cues:r.cues,timelineFps:e.draft.fps,canvasWidth:e.draft.projectMetadata?.width??1920,canvasHeight:e.draft.projectMetadata?.height??1080,fileName:t.fileName,format:t.format,startFrame:t.startFrame,styleTemplate:t.style?{...t.style,color:t.style.color??"#ffffff"}:void 0,idFactory:()=>e.ids.mint(),importedAt:t.importedAt});return e.draft.items=[...e.draft.items,...bs(n).map(fe)],{inverse:qa(),result:{itemIds:n.map(a=>a.id),warnings:r.warnings}}}}),A({type:"subtitle.setCues",params:y({itemId:Cr,cues:F(Nu)}).strict(),validate(e,t){const r=No(e,t);return new Set(t.cues.map(n=>n.id)).size!==t.cues.length&&r.push({message:"Mã câu phụ đề bị trùng",path:"cues"}),r},apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="subtitle"?{...r,cues:t.cues}:r),qa()}}),A({type:"subtitle.updateCue",params:y({itemId:Cr,cueId:Cr,updates:y({startSeconds:Rr.nonnegative().optional(),endSeconds:Rr.nonnegative().optional(),text:w().optional()}).strict()}).strict(),mergeKey:e=>`subtitle:${e.itemId}:${e.cueId}:${Object.keys(e.updates).sort().join(",")}`,validate(e,t){const r=No(e,t);if(r.length)return r;const n=e.items.find(i=>i.id===t.itemId),a=n?.type==="subtitle"?n.cues.find(i=>i.id===t.cueId):void 0;return a?Nu.safeParse({...a,...t.updates}).success?[]:[{message:"Thời điểm kết thúc phải sau thời điểm bắt đầu",path:"updates"}]:[{message:`Không tìm thấy câu phụ đề ${t.cueId}`,path:"cueId"}]},apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="subtitle"?{...r,cues:r.cues.map(n=>n.id===t.cueId?{...n,...t.updates}:n)}:r),qa()}}),A({type:"subtitle.setStyle",params:y({itemId:Cr,style:ks}).strict(),validate:(e,t)=>No(e,t),mergeKey:e=>`subtitle:style:${e.itemId}:${Object.keys(e.style).sort().join(",")}`,apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="subtitle"?fe({...r,...t.style}):r),qa()}})}const GM="ytf-editor-media-sources",KM="ytf-editor-media-source-changed",VM=["thumbnailUrl","waveformData","reverseConformSrc","reverseConformPath","reverseConformKey","reverseConformPreviewSrc","reverseConformPreviewPath","reverseConformPreviewKey","reverseConformPreviewUsesProxy","reverseConformStatus","reverseConformPreviewIsSourceLevel","reverseConformPreviewSourceDuration","reverseConformPreviewFps","reverseConformLocalStart"];function WM(e,t,r,n,a){for(const i of e)if(i.mediaId===t){if(i.type==="shape"){if(!i.waveformVisualizer)continue;if(i.waveformVisualizer.sourceHash!==n){const o={...i.waveformVisualizer};delete o.envelopeBase64,delete o.durationSeconds,delete o.normalization,i.waveformVisualizer=o}continue}for(const o of VM)Reflect.deleteProperty(i,o);i.src=r,(i.type==="video"||i.type==="audio")&&(i.audioSrc=r),a&&(Reflect.set(i,"sourceWidth",a.width),Reflect.set(i,"sourceHeight",a.height),Reflect.set(i,"sourceFps",a.fps),Reflect.set(i,"sourceDuration",Math.round(a.duration*a.fps)))}}function xE(e){if(!(typeof window>"u")&&(window.dispatchEvent(new CustomEvent(KM,{detail:e})),typeof BroadcastChannel<"u")){const t=new BroadcastChannel(GM);t.postMessage(e),t.close()}}const Lr=w().min(1),ce=M().finite(),jM=y({provider:w(),author:w().optional(),authorUrl:w().optional(),sourceUrl:w().optional(),sourceId:w().optional(),license:w().optional()}).strict(),Ga=y({id:Lr,storageType:Z("workspace"),contentHash:w().optional(),fileLastModified:ce.optional(),fileName:w(),fileSize:ce.nonnegative(),mimeType:w(),duration:ce,width:ce,height:ce,fps:ce,codec:w(),bitrate:ce,audioCodec:w().optional(),audioCodecSupported:B().optional(),videoCodecSupported:B().optional(),previewAudioOpfsPath:w().optional(),previewAudioMimeType:w().optional(),previewAudioConformedAt:ce.optional(),keyframeTimestamps:F(ce).optional(),gopInterval:ce.optional(),thumbnailId:w().optional(),tags:F(w()),attribution:jM.optional(),aiCaptions:F(y({timeSec:ce,text:w(),sceneData:y({caption:w().optional(),shotType:w().optional(),subjects:F(w()).optional(),action:w().optional(),setting:w().optional(),lighting:w().optional(),timeOfDay:w().optional(),weather:w().optional()}).strict().optional(),thumbRelPath:w().optional(),embedding:F(ce).optional(),palette:F(y({l:ce,a:ce,b:ce,weight:ce}).strict()).optional()}).strict()).optional(),createdAt:ce,updatedAt:ce,background:y({kind:z(["black","green","none"]),keyColor:w().optional(),version:ce,detectedAt:ce,contentHash:w()}).strict().optional(),overlay:B().optional()}).strict(),$M=y({mediaId:Lr.optional(),label:w().optional(),src:w().optional(),audioSrc:w().optional(),thumbnailUrl:w().optional(),waveformData:F(ce).optional(),sourceWidth:ce.optional(),sourceHeight:ce.optional(),sourceFps:ce.optional(),sourceDuration:ce.optional()}).strict(),YM=z(["src","audioSrc","thumbnailUrl","waveformData","reverseConformSrc","reverseConformPath","reverseConformKey","reverseConformPreviewSrc","reverseConformPreviewPath","reverseConformPreviewKey","reverseConformPreviewUsesProxy","reverseConformStatus","reverseConformPreviewIsSourceLevel","reverseConformPreviewSourceDuration","reverseConformPreviewFps","reverseConformLocalStart"]),Jt=()=>({kind:"patch",patch:{}});function XM(e,t){return e.mediaItems?.some(r=>r.id===t.mediaId)?[]:[{message:`Không tìm thấy tệp ${t.mediaId}`,path:"mediaId"}]}function qu(e){const t=vr(e);if(!t.ok)throw new Error(t.issues.map(r=>r.message).join("; "))}function QM(){A({type:"media.add",history:"none",dirty:!0,params:y({media:Ga}).strict(),apply(e,t){const r=e.draft.mediaItems??[];return e.draft.mediaItems=r.some(n=>n.id===t.media.id)?r.map(n=>n.id===t.media.id?t.media:n):[t.media,...r],Jt()}}),A({type:"media.refreshSource",history:"none",dirty:!1,params:y({media:Ga,src:w().min(1)}).strict(),apply(e,t){e.draft.mediaItems=(e.draft.mediaItems??[]).map(a=>a.id===t.media.id?t.media:a);const r={width:t.media.width,height:t.media.height,fps:t.media.fps||e.draft.fps,duration:t.media.duration},n=a=>a.map(i=>{if(i.mediaId!==t.media.id)return i;const o={...i};return WM([o],t.media.id,t.src,t.media.contentHash,r),o});return e.draft.items=n(e.draft.items),e.draft.compositions=e.draft.compositions.map(a=>({...a,items:n(a.items)})),e.draft.stashStack=e.draft.stashStack?.map(a=>({...a,items:n(a.items)})),e.draft.mainHolder&&(e.draft.mainHolder={...e.draft.mainHolder,items:n(e.draft.mainHolder.items)}),Jt()}}),A({type:"media.update",history:"none",dirty:!0,params:y({mediaId:Lr,updates:Ga.omit({id:!0}).partial()}).strict(),validate:(e,t)=>XM(e,t),apply(e,t){return e.draft.mediaItems=(e.draft.mediaItems??[]).map(r=>r.id===t.mediaId?{...r,...t.updates}:r),Jt()}}),A({type:"media.setRecords",history:"none",params:y({mediaItems:F(Ga),resetImporting:B().default(!1)}).strict(),validate:(e,t)=>new Set(t.mediaItems.map(r=>r.id)).size===t.mediaItems.length?[]:[{message:"Danh sách tệp có mã trùng nhau",path:"mediaItems"}],apply(e,t){return t.resetImporting&&Me.setState({importingIds:[]}),e.draft.mediaItems=t.mediaItems,Jt()}}),A({type:"media.removeRecords",history:"none",dirty:!0,params:y({mediaIds:F(Lr)}).strict(),apply(e,t){const r=new Set(t.mediaIds);return e.draft.mediaItems=(e.draft.mediaItems??[]).filter(n=>!r.has(n.id)),Jt()}}),A({type:"media.remove",params:y({mediaIds:F(Lr)}).strict(),apply(e,t){const r=new Set(t.mediaIds),n=[e.draft.items,...e.draft.compositions.map(i=>i.items),...(e.draft.stashStack??[]).map(i=>i.items),...e.draft.mainHolder?[e.draft.mainHolder.items]:[]],a=[...new Set(n.flatMap(i=>i.filter(o=>o.mediaId&&r.has(o.mediaId)).map(o=>o.id)))];return a.length&&qu([{type:"item.removeProjectItems",params:{itemIds:a}}]),e.draft.mediaItems=(e.draft.mediaItems??[]).filter(i=>!r.has(i.id)),{inverse:Jt(),result:a}}}),A({type:"media.relinkItems",params:y({itemIds:F(Lr),updates:$M,clearFields:F(YM).default([])}).strict(),validate(e,t){const r=[e.items,...e.compositions.map(a=>a.items),...(e.stashStack??[]).map(a=>a.items),...e.mainHolder?[e.mainHolder.items]:[]],n=new Set(r.flatMap(a=>a.map(i=>i.id)));return t.itemIds.filter(a=>!n.has(a)).map(a=>({message:`Không tìm thấy clip ${a}`,path:"itemIds"}))},apply(e,t){return qu(t.itemIds.map(r=>({type:"item.updateProjectItem",params:{itemId:r,updates:t.updates,clearFields:t.clearFields}}))),Jt()}})}const re=()=>M().finite(),Sl=y({frequencyBins:F(re().nonnegative()).min(2).max(8193),magnitudes:F(re().nonnegative()).min(2).max(8193),standardDeviations:F(re().nonnegative()).min(2).max(8193),sampleRate:re().int().min(8e3).max(192e3),fftSize:re().int().min(256).max(16384).refine(e=>(e&e-1)===0)}).strict().superRefine((e,t)=>{const r=e.fftSize/2+1;(e.frequencyBins.length!==r||e.magnitudes.length!==r||e.standardDeviations.length!==r)&&t.addIssue({code:"custom",message:"Số dải tần của mẫu nhiễu không khớp FFT"});for(let n=0;n<e.frequencyBins.length;n++)if(Math.abs(e.frequencyBins[n]-n*e.sampleRate/e.fftSize)>1e-6){t.addIssue({code:"custom",message:"Dải tần của mẫu nhiễu không hợp lệ"});break}}),lr=()=>re().min(0).max(1),JM=y({threshold:re().min(-60).max(0),ratio:re().min(1).max(20),attack:re().min(.001).max(1),release:re().min(.01).max(3),knee:re().min(0).max(40),makeupGain:re().min(0).max(24)}).strict(),ZM=y({threshold:re().min(-60).max(0),reduction:lr(),smoothing:lr(),profile:Sl.optional()}).strict(),eC=y({roomSize:lr(),damping:lr(),wetLevel:lr(),dryLevel:lr(),preDelay:re().min(0).max(100)}).strict(),tC=y({time:re().min(0).max(2),feedback:re().min(0).max(.95),wetLevel:lr()}).strict(),Ka={id:w().min(1),enabled:B()},qo=Oi("type",[y({...Ka,type:Z("compressor"),params:JM}).strict(),y({...Ka,type:Z("noiseReduction"),params:ZM}).strict(),y({...Ka,type:Z("reverb"),params:eC}).strict(),y({...Ka,type:Z("delay"),params:tC}).strict()]).superRefine((e,t)=>{e.type==="noiseReduction"&&e.enabled&&!e.params.profile&&t.addIssue({code:"custom",message:"Cần lấy mẫu nhiễu trước khi bật giảm nhiễu"})}),rC=y({threshold:re().optional(),ratio:re().optional(),attack:re().optional(),release:re().optional(),knee:re().optional(),makeupGain:re().optional(),reduction:re().optional(),smoothing:re().optional(),profile:Sl.optional(),roomSize:re().optional(),damping:re().optional(),wetLevel:re().optional(),dryLevel:re().optional(),preDelay:re().optional(),time:re().optional(),feedback:re().optional()}).strict(),mn=dr([y({itemId:w().min(1)}).strict(),y({trackId:w().min(1)}).strict()]);function it(e,t){return"itemId"in t?e.items.find(r=>r.id===t.itemId)?.audioFx:e.tracks.find(r=>r.id===t.trackId)?.audioFx}function pn(e,t,r){const n="itemId"in t?e.items.find(a=>a.id===t.itemId):e.tracks.find(a=>a.id===t.trackId);return n?"itemId"in t&&"type"in n&&n.type!=="audio"&&n.type!=="video"?[{code:"invalid-target",message:"Hiệu ứng âm thanh chỉ áp dụng cho clip âm thanh hoặc video"}]:r&&!it(e,t)?.some(a=>a.id===r)?[{code:"not-found",message:"Không tìm thấy hiệu ứng âm thanh"}]:[]:[{code:"not-found",message:"Không tìm thấy clip hoặc lớp âm thanh"}]}function fn(e,t,r){"itemId"in t?e.draft.items=e.draft.items.map(n=>n.id===t.itemId?{...n,audioFx:r}:n):e.draft.tracks=e.draft.tracks.map(n=>n.id===t.trackId?{...n,audioFx:r}:n)}const hn=()=>({kind:"patch",patch:{}});function nC(){A({type:"audioFx.add",params:y({target:mn,effect:qo}).strict(),validate(e,t){const r=pn(e,t.target);return it(e,t.target)?.some(n=>n.id===t.effect.id)&&r.push({code:"duplicate-id",message:"Mã hiệu ứng âm thanh đã tồn tại"}),r},apply(e,t){return fn(e,t.target,[...it(e.draft,t.target)??[],t.effect]),hn()}}),A({type:"audioFx.remove",params:y({target:mn,effectId:w().min(1)}).strict(),validate:(e,t)=>pn(e,t.target,t.effectId),apply(e,t){return fn(e,t.target,it(e.draft,t.target).filter(r=>r.id!==t.effectId)),hn()}}),A({type:"audioFx.update",params:y({target:mn,effectId:w().min(1),params:rC.optional(),enabled:B().optional()}).strict(),validate(e,t){const r=pn(e,t.target,t.effectId),n=it(e,t.target)?.find(i=>i.id===t.effectId);if(!n)return r;const a=qo.safeParse({...n,enabled:t.enabled??n.enabled,params:{...n.params,...t.params}});return a.success||r.push(...a.error.issues.map(i=>({code:"invalid-effect",message:i.message}))),r},apply(e,t){return fn(e,t.target,it(e.draft,t.target).map(r=>r.id!==t.effectId?r:qo.parse({...r,enabled:t.enabled??r.enabled,params:{...r.params,...t.params}}))),hn()},mergeKey:e=>`audio-fx:${JSON.stringify(e.target)}:${e.effectId}`}),A({type:"audioFx.reorder",params:y({target:mn,effectIds:F(w().min(1))}).strict(),validate(e,t){const r=pn(e,t.target),n=it(e,t.target)??[];return(t.effectIds.length!==n.length||new Set(t.effectIds).size!==n.length||t.effectIds.some(a=>!n.some(i=>i.id===a)))&&r.push({code:"invalid-order",message:"Thứ tự phải chứa mỗi hiệu ứng đúng một lần"}),r},apply(e,t){const r=it(e.draft,t.target);return fn(e,t.target,t.effectIds.map(n=>r.find(a=>a.id===n))),hn()}}),A({type:"audioFx.learnNoiseProfile",params:y({target:mn,effectId:w().min(1),profile:Sl,range:y({startSeconds:M().finite().nonnegative(),endSeconds:M().finite().positive()}).strict().refine(e=>e.endSeconds>e.startSeconds)}).strict(),validate(e,t){const r=pn(e,t.target,t.effectId),n=it(e,t.target)?.find(a=>a.id===t.effectId);return n&&n.type!=="noiseReduction"&&r.push({code:"invalid-effect",message:"Mẫu nhiễu chỉ áp dụng cho hiệu ứng giảm nhiễu"}),(t.range.endSeconds-t.range.startSeconds)*t.profile.sampleRate<t.profile.fftSize&&r.push({code:"invalid-range",message:"Khoảng nhiễu phải chứa ít nhất một khung FFT"}),r},apply(e,t){return fn(e,t.target,it(e.draft,t.target).map(r=>r.id===t.effectId&&r.type==="noiseReduction"?{...r,params:{...r.params,profile:t.profile}}:r)),hn()}})}const th=y({text:w().min(1),startFrame:M().int().nonnegative(),endFrame:M().int().positive(),estimated:B().optional()}).refine(e=>e.endFrame>e.startFrame,{message:"Mốc kết thúc phải sau mốc bắt đầu."}),aC=z(["none","word-highlight","word-by-word","karaoke","typewriter","bounce","pop","fade"]),iC=y({words:F(th),estimated:B(),cues:F(y({id:w(),text:w(),startSeconds:M().nonnegative(),endSeconds:M().positive()})),durationInFrames:M().int().nonnegative()});function gn(e,t,r){const n=e.items.find(a=>a.id===t);return!n||n.type!=="text"&&n.type!=="subtitle"?[{code:"caption-target",message:"Chọn một mục chữ hoặc phụ đề."}]:r?.some((a,i)=>a.endFrame>n.durationInFrames||a.endFrame<=a.startFrame||i>0&&a.startFrame<r[i-1].startFrame)?[{code:"caption-timing",message:"Mốc từ phải tăng dần và nằm trong thời lượng mục."}]:[]}function oC(){A({type:"caption.setWords",params:y({itemId:w(),words:F(th),estimated:B().optional()}),validate:(e,t)=>gn(e,t.itemId,t.words),apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&(r.type==="text"||r.type==="subtitle")?{...r,words:t.words,wordsEstimated:t.estimated??!1}:r),{kind:"patch",patch:{}}),mergeKey:e=>`caption-words:${e.itemId}`}),A({type:"caption.updateWord",params:y({itemId:w(),index:M().int().nonnegative(),word:y({text:w().min(1).optional(),startFrame:M().int().nonnegative().optional(),endFrame:M().int().positive().optional()})}),validate:(e,t)=>{const r=gn(e,t.itemId),n=e.items.find(a=>a.id===t.itemId);return!n||n.type!=="text"&&n.type!=="subtitle"||!n.words?.[t.index]?[...r,{message:"Không tìm thấy từ."}]:gn(e,t.itemId,n.words.map((a,i)=>i===t.index?{...a,...t.word}:a))},apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&(r.type==="text"||r.type==="subtitle")?{...r,words:r.words?.map((n,a)=>a===t.index?{...n,...t.word}:n)}:r),{kind:"patch",patch:{}}),mergeKey:e=>`caption-word:${e.itemId}:${e.index}`}),A({type:"caption.setPreset",params:y({itemId:w(),preset:aC}),validate:(e,t)=>gn(e,t.itemId),apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&(r.type==="text"||r.type==="subtitle")?{...r,captionPreset:t.preset}:r),{kind:"patch",patch:{}})}),A({type:"caption.importTimed",params:y({itemId:w(),data:iC}),validate:(e,t)=>gn(e,t.itemId,t.data.words),apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>{if(r.id!==t.itemId||r.type!=="text"&&r.type!=="subtitle")return r;const n={...r,words:t.data.words,wordsEstimated:t.data.estimated};if(n.type==="subtitle")return{...n,cues:t.data.cues};if(!t.data.cues.length)return{...n,text:t.data.words.map(f=>f.text).join(" ")};const{text:a,textSpans:i,spanLayout:o,textLayoutDrafts:s,textStylePresetId:l,textStyleScale:c,textMotion:u,textRole:d,captionSource:m,...p}=n;return{...p,type:"subtitle",cues:t.data.cues,source:{type:"subtitle-import",fileName:m?.fileName??r.label,format:m?.format??(r.label.toLowerCase().endsWith(".srt")?"srt":"vtt"),importedAt:m?.importedAt??0}}}),{kind:"patch",patch:{}})})}const wE="#facc15",sC=7,lC=42,Go=350,cC=e=>/<|\{|\}/.test(e);function rh(e,t,r,n){const a=e.reduce((s,l)=>s+l.length,0);if(a===0||r<=t)return[];const i=[];let o=0;for(const[s,l]of e.entries()){const c=t+Math.floor((r-t)*o/a);o+=l.length;const u=s===e.length-1?r:t+Math.floor((r-t)*o/a);u>c&&i.push({text:l,startFrame:c,endFrame:u,estimated:n})}return i}function uC(e,t,r){const n=e.text.trim().split(/\s+/).filter(Boolean);return n.length===0?[]:n.length===1?[{text:n[0],startFrame:t,endFrame:r,estimated:e.estimated}]:rh(n,t,r,e.estimated===!0)}function dC(e,t){const r=Math.max(0,e.durationInFrames),n=e.cues;if(n?.length){const o=[...n].filter(l=>l.endSeconds>l.startSeconds&&l.endSeconds>0).sort((l,c)=>l.startSeconds-c.startSeconds),s=[];for(const[l,c]of o.entries()){const u=Math.max(0,Math.round(c.startSeconds*t)),d=o[l+1]?Math.round(o[l+1].startSeconds*t):r,m=Math.min(Math.round(c.endSeconds*t),d,r);m>u&&s.push({id:c.id,text:c.text,startFrame:u,endFrame:m})}return s}const a=e.words??[];if(a.length===0)return[];const i=Math.min(...a.map(o=>o.startFrame));return[{id:"words",text:a.map(o=>o.text.trim()).filter(Boolean).join(" "),startFrame:Math.max(0,i),endFrame:r}]}function nh(e,t){const r=dC(e,t),n=[...e.words??[]].sort((s,l)=>s.startFrame-l.startFrame||s.endFrame-l.endFrame),a=[];let i=e.wordsEstimated===!0,o=0;for(const s of r){for(;o<n.length&&n[o].startFrame<s.startFrame;)i=!0,o++;const l=[];for(;o<n.length&&n[o].startFrame<s.endFrame;)l.push(n[o++]);const c=[];if(l.length===0)s.text.trim()&&!cC(s.text)&&(c.push(...rh(s.text.trim().split(/\s+/).filter(Boolean),s.startFrame,s.endFrame,!0)),c.length&&(i=!0));else for(const p of l){const f=Math.min(Math.max(p.startFrame,s.startFrame),s.endFrame),h=Math.min(Math.max(p.endFrame,s.startFrame),s.endFrame);if(h<=f){i=!0;continue}const g=uC(p,f,h);if(g.length===0){i=!0;continue}c.push(...g)}const u=[];let d=[],m=0;for(const p of c){const f=p.text.length+(d.length>0?1:0),h=d.at(-1),g=h&&!e.wordsEstimated&&!h.estimated&&!p.estimated&&(p.startFrame-h.endFrame)*1e3/t>=Go;d.length>0&&(g||d.length>=sC||m+f>lC)&&(u.push(d),d=[],m=0),d.push(p),m+=p.text.length+(d.length>1?1:0)}if(d.length>0&&u.push(d),u.length===0){if(!s.text.trim())continue;a.push({id:s.id,text:s.text,startFrame:s.startFrame,endFrame:s.endFrame,words:[],estimated:e.wordsEstimated===!0});continue}for(const[p,f]of u.entries()){const h=f[0].startFrame,g=u[p+1]?.[0],v=f.at(-1),b=g&&!e.wordsEstimated&&!v.estimated&&!g.estimated&&(g.startFrame-v.endFrame)*1e3/t>=Go?v.endFrame:g?.startFrame??s.endFrame;a.push({id:p===0?s.id:`${s.id}.${p+1}`,text:f.map(x=>x.text).join(" "),startFrame:h,endFrame:b,words:f,estimated:e.wordsEstimated===!0||f.some(x=>x.estimated===!0)})}}o<n.length&&(i=!0);for(let s=a.length-1;s>=0;s--){const l=a[s];if(l.endFrame>l.startFrame)continue;const c=a[s-1]??a[s+1];c&&c.words.length>0&&l.words.length>0&&(c.words.push(...l.words),c.text=c.words.map(u=>u.text).join(" "),c.estimated=c.estimated||l.estimated),a.splice(s,1)}for(let s=0;s+1<a.length;s++){const l=a[s+1].startFrame-a[s].endFrame;l>0&&l*1e3/t<Go&&(a[s].endFrame=a[s+1].startFrame)}return{pages:a,degraded:i}}const mC=[],pC=[],Gu=new WeakMap;function kE(e,t){const r=e.words??mC,n=e.cues??pC;let a=Gu.get(r);const i=a?.get(n);if(i&&i.fps===t&&i.duration===e.durationInFrames&&i.wordsEstimated===e.wordsEstimated)return i.pages;const o=nh(e,t).pages;return a||(a=new WeakMap,Gu.set(r,a)),a.set(n,{fps:t,duration:e.durationInFrames,wordsEstimated:e.wordsEstimated,pages:o}),o}function PE(e,t){let r=0,n=e.length-1;for(;r<=n;){const a=r+n>>1,i=e[a];if(t<i.startFrame)n=a-1;else if(t>=i.endFrame)r=a+1;else return i}return null}function ME(e,t){const r=e.wordsEstimated===!0||t?.estimated===!0,n=e.captionPreset??"none";return n==="karaoke"&&r?"fade":n}function fC(e,t){if(!t.idPrefix||!Number.isInteger(t.durationInFrames)||t.durationInFrames<=0||t.fps<=0||t.width<=0||t.height<=0)throw new Error("Thông số mẫu không hợp lệ.");if(t.sections.some(r=>r.from<0||r.durationInFrames<=0||r.from+r.durationInFrames>t.durationInFrames))throw new Error("Phân đoạn phải nằm trong thời lượng mẫu.");return e.resolve(t)}function hC(e,t,r,n,a){const i=[];for(const o of["width","height"]){const s=o==="width"?t.width:t.height;i.push({type:"keyframe.add",params:{itemId:e,property:o,frame:0,value:s*n,easing:"linear"}},{type:"keyframe.add",params:{itemId:e,property:o,frame:r-1,value:s*a,easing:"linear"}})}return i}function Ko(e,t){const r=[],n=e.idPrefix,a=e.from??0,i=(u,d,m,p="video")=>{const f={id:`${n}-${u}`,name:d,order:m,height:64,locked:!1,visible:!0,muted:!1,solo:!1,kind:p};return r.push({type:"track.add",params:f}),f.id},o=i("visuals","Hình ảnh",0),s=i("titles","Tiêu đề",1),l=i("captions","Phụ đề",2),c=u=>r.push({type:"item.add",params:{item:u}});for(const[u,d]of e.sections.entries()){if(d.image){const m=`${n}-still-${u}`;c({id:m,type:"image",trackId:o,from:a+d.from,durationInFrames:d.durationInFrames,label:d.title,mediaId:d.image.mediaId,src:d.image.src,transform:{x:0,y:0,width:e.width,height:e.height}}),r.push(...hC(m,{width:e.width,height:e.height},d.durationInFrames,1,1.12))}if(t!=="narration"){const m=t==="documentary";c({id:`${n}-title-${u}`,type:"text",trackId:s,from:a+d.from,durationInFrames:Math.min(d.durationInFrames,Math.round(e.fps*(m?6:3))),label:d.title,text:m?d.lowerThird??d.title:`${u+1}. ${d.title}`,fontSize:e.height*(m?.05:.08),color:"#ffffff",backgroundColor:"#111827",textAlign:m?"left":"center",verticalAlign:"middle",transform:{x:m?-e.width*.2:0,y:m?e.height*.3:0,width:e.width*(m?.5:.8),height:e.height*.18}})}}if(e.narration){const u=i("narration","Lời dẫn",3,"audio");c({id:`${n}-voice`,type:"audio",trackId:u,from:a,durationInFrames:e.durationInFrames,label:"Lời dẫn",src:e.narration.src,mediaId:e.narration.mediaId})}if(e.words?.length){const{pages:u,degraded:d}=nh({words:e.words,wordsEstimated:e.wordsEstimated,durationInFrames:e.durationInFrames},e.fps),m=e.wordsEstimated===!0||d,p=u.map((h,g)=>({id:`${n}-cue-${g}`,text:h.text,startSeconds:h.startFrame/e.fps,endSeconds:h.endFrame/e.fps})),f=u.flatMap(h=>h.words.map(g=>({text:g.text,startFrame:g.startFrame,endFrame:g.endFrame,...g.estimated?{estimated:!0}:{}})));c({id:`${n}-caption`,type:"subtitle",trackId:l,from:a,durationInFrames:e.durationInFrames,label:"Phụ đề theo từ",source:{type:"subtitle-import",fileName:"captions.vtt",format:"vtt",importedAt:0},cues:p,words:f,wordsEstimated:m,captionPreset:m?"fade":"karaoke",color:"#ffffff",fontSize:e.height*.055,textAlign:"center",verticalAlign:"middle",backgroundColor:"#111827",transform:{x:0,y:e.height*.35,width:e.width*.85,height:e.height*.2}})}return r}const gC=[{id:"narration-ken-burns-captions",name:"Lời dẫn và ảnh chuyển động",description:"Video dài với lời dẫn, ảnh Ken Burns và phụ đề karaoke theo từ.",category:"narration",resolve:e=>Ko(e,"narration")},{id:"listicle-sections",name:"Danh sách theo phân đoạn",description:"Mỗi mục có thẻ tiêu đề đánh số, hình ảnh chuyển động và phụ đề.",category:"listicle",resolve:e=>Ko(e,"listicle")},{id:"documentary-lower-thirds",name:"Tư liệu và chú thích",description:"Video tư liệu dài với chú thích lower-third, lời dẫn và phụ đề.",category:"documentary",resolve:e=>Ko(e,"documentary")}],yC=Object.fromEntries(gC.map(e=>[e.id,e]));function Ku(e){return yC[e]}const Vu={introSec:.4,outroSec:.3},ah=y({idPrefix:w().min(1),trackIds:An([w().min(1),w().min(1),w().min(1)]),from:M().int().nonnegative(),durationInFrames:M().int().positive(),fps:M().positive(),width:M().positive(),height:M().positive(),label:w().optional(),accentColor:w().regex(/^#[a-fA-F0-9]{6}$/).optional(),textColor:w().regex(/^#[a-fA-F0-9]{6}$/).optional(),x:M().finite().optional(),y:M().finite().optional()}).strict();function vC(e,t){const r=Math.max(1,Math.min(Math.round(Vu.introSec*t),Math.max(1,e-1))),n=Math.max(e>r?1:0,Math.min(Math.round(Vu.outroSec*t),e-r)),a=Math.max(0,e-r-n);return{introFrames:r,holdFrames:a,outroFrames:n}}function bC(e){const t=Math.round(e.height*.064),r=Math.round(e.width*.156),n=Math.round(e.width*.033),a=e.x??e.width/2-n-r/2,i=e.y??-(e.height/2)+n+t/2,o=Math.round(t*.32);return{buttonWidth:r,buttonHeight:t,margin:n,x:a,y:i,pad:o}}function SC(e){const t=ah.parse(e),r=t.idPrefix,n=bC(t),a=vC(t.durationInFrames,t.fps),i=t.accentColor??"#cc3327",o=t.textColor??"#ffffff",s=t.label??"Subscribe",l=`${r}-subscribe-group`,c={from:t.from,durationInFrames:t.durationInFrames,linkedGroupId:l},u={...c,id:`${r}-subscribe-backdrop`,type:"shape",shapeType:"rectangle",label:"Nền nút Subscribe",trackId:t.trackIds[0],fillColor:i,cornerRadius:n.buttonHeight/2,transform:{x:n.x,y:n.y,width:n.buttonWidth,height:n.buttonHeight}},d=Math.round(n.buttonHeight*.46),m={...c,id:`${r}-subscribe-play`,type:"shape",shapeType:"triangle",direction:"right",label:"Biểu tượng phát",trackId:t.trackIds[1],fillColor:o,transform:{x:n.x-n.buttonWidth/2+n.pad+d/2,y:n.y,width:d,height:d}},p={...c,id:`${r}-subscribe-text`,type:"text",label:"Chữ Subscribe",trackId:t.trackIds[2],text:s,color:o,fontSize:Math.round(n.buttonHeight*.5),fontWeight:"semibold",textAlign:"center",verticalAlign:"middle",transform:{x:n.x+(n.pad+d)/2,y:n.y,width:n.buttonWidth-n.pad-d,height:n.buttonHeight}},f=[{type:"item.add",params:{item:u}},{type:"item.add",params:{item:m}},{type:"item.add",params:{item:p}}],{holdFrames:h,outroFrames:g}=a,v=n.buttonWidth+n.margin,S=t.durationInFrames-1,b=Math.min(a.introFrames,S),x=Math.min(S,t.durationInFrames-g);for(const T of[u,m,p]){const C=T.transform.x,k=[[0,0],[b,1]];h>0&&k.push([x,1]),k.push([S,0]);const _=[[0,C+v],[b,C]];h>0&&_.push([x,C]),_.push([S,C]);const P=(R,E)=>{const D=[];for(const L of E){const U=D[D.length-1];U&&U[0]===L[0]?U[1]=L[1]:D.push([...L])}for(const[L,U]of D)f.push({type:"keyframe.add",params:{itemId:T.id,property:R,frame:L,value:U,easing:L===b?"ease-out":"linear"}})};P("opacity",k),P("x",_)}return{commands:f,itemIds:[u.id,m.id,p.id],groupId:l,timing:a}}const Wu=y({mediaId:w(),src:w()}),IC=y({idPrefix:w().min(1),fps:M().positive(),width:M().positive(),height:M().positive(),durationInFrames:M().int().positive(),from:M().int().nonnegative().optional(),title:w().optional(),narration:Wu.optional(),sections:F(y({title:w(),from:M().int().nonnegative(),durationInFrames:M().int().positive(),image:Wu.optional(),lowerThird:w().optional()})),words:F(y({text:w(),startFrame:M().int().nonnegative(),endFrame:M().int().positive(),estimated:B().optional()})).optional(),wordsEstimated:B().optional()});function ju(e,t){for(const r of t){const n=ms(r.type);if(!n)throw new Error(`Lệnh chưa đăng ký: ${r.type}`);const a=n.params.parse(r.params),i=n.validate?.(e.draft,a)??[];if(i.length)throw new Error(i.map(o=>o.message).join("; "));n.apply(e,a)}}function xC(){A({type:"template.apply",params:y({templateId:w(),context:IC}),validate:(e,t)=>Ku(t.templateId)?[]:[{code:"template-missing",message:"Không tìm thấy mẫu."}],apply:(e,t)=>{const r=Ku(t.templateId);if(!r)throw new Error("Không tìm thấy mẫu.");const n=fC(r,t.context);return ju(e,n),{inverse:{kind:"patch",patch:{}},result:{templateId:r.id,commands:n}}}}),A({type:"template.subscribe",params:ah,validate:(e,t)=>t.trackIds.map(r=>e.tracks.find(n=>n.id===r)).flatMap((r,n)=>r&&r.kind!=="audio"&&!r.isGroup?[]:[{code:"subscribe-track",message:`Rãnh Subscribe ${n+1} không tồn tại hoặc là rãnh âm thanh.`,path:`trackIds.${n}`}]),apply:(e,t)=>{const{commands:r,itemIds:n,groupId:a,timing:i}=SC(t);return ju(e,r),{inverse:{kind:"patch",patch:{}},result:{itemIds:n,groupId:a,timing:i,commands:r}}}})}const kr=w().min(1);function $u(e,t){const r=e.items.find(n=>n.id===t);return r?.type==="shape"&&r.waveformVisualizer?[]:[{path:"itemId",message:"Không tìm thấy mục Sóng giọng"}]}function Yu(e,t,r){const n=e.mediaItems?.find(a=>a.id===t);return!n||!n.mimeType.startsWith("audio/")?[{path:"mediaId",message:"Chọn tệp âm thanh làm nguồn giọng"}]:n.contentHash===r?[]:[{path:"sourceHash",message:"Dữ liệu sóng giọng không khớp hash nguồn hiện tại"}]}function wC(){A({type:"waveform.add",params:y({id:kr.optional(),trackId:kr,from:M().int().nonnegative(),durationInFrames:M().int().positive(),mediaId:kr,envelope:ar,style:gs.partial().optional(),transform:y({x:M().finite().optional(),y:M().finite().optional(),width:M().positive().optional(),height:M().positive().optional()}).strict().optional()}).strict(),validate:(e,t)=>Yu(e,t.mediaId,t.envelope.sourceHash),apply(e,t){const r={id:t.id??e.ids.mint(),type:"shape",shapeType:"rectangle",label:"Sóng giọng",trackId:t.trackId,from:t.from,durationInFrames:t.durationInFrames,mediaId:t.mediaId,fillColor:t.style?.color??Qc.color,sourceStart:0,sourceEnd:t.durationInFrames,sourceFps:e.draft.fps,sourceDuration:t.envelope.durationSeconds*e.draft.fps,transform:{width:960,height:160,x:0,y:360,...t.transform},waveformVisualizer:{version:1,...Qc,...t.style,...t.envelope}},n=vr([{type:"item.add",params:{item:r}}]);if(!n.ok)throw new Error(n.issues.map(a=>a.message).join("; "));return{inverse:{kind:"patch",patch:{}},result:r.id}}}),A({type:"waveform.setStyle",params:y({itemId:kr,style:gs.partial()}).strict(),validate:(e,t)=>$u(e,t.itemId),mergeKey:e=>`waveform.setStyle:${e.itemId}:${Object.keys(e.style).sort().join(",")}`,apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="shape"&&r.waveformVisualizer?{...r,waveformVisualizer:{...r.waveformVisualizer,...t.style}}:r),{kind:"patch",patch:{}}}}),A({type:"waveform.setEnvelope",params:y({itemId:kr,mediaId:kr.optional(),envelope:ar}).strict(),validate(e,t){const r=e.items.find(n=>n.id===t.itemId);return[...$u(e,t.itemId),...Yu(e,t.mediaId??r?.mediaId??"",t.envelope.sourceHash)]},apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="shape"&&r.waveformVisualizer?{...r,mediaId:t.mediaId??r.mediaId,sourceDuration:t.envelope.durationSeconds*(r.sourceFps??e.draft.fps),waveformVisualizer:{...r.waveformVisualizer,...t.envelope}}:r),{kind:"patch",patch:{}}}})}function kC(){f2(),h2(),J2(),Z2(),o2(),HP(),TM(),OM(),RM(),u2(),LM(),qM(),QM(),nC(),oC(),xC(),wC()}kC();const PC=[{description:"Media files",accept:{"video/*":[".mp4",".webm",".mov",".avi",".mkv"],"audio/*":[".mp3",".wav",".ogg",".m4a",".aac"],"image/*":[".jpg",".jpeg",".png",".gif",".webp",".svg"],"application/lottie+json":[".json",".lottie"]}}];function MC(e={}){const{promise:t,resolve:r,reject:n}=Promise.withResolvers(),a=document.createElement("input");a.type="file",a.multiple=e.multiple??!0,a.accept=Object.values(PC[0].accept).flat().join(","),a.hidden=!0;const i=()=>a.remove();return a.addEventListener("change",()=>{const o=Array.from(a.files??[]);i(),o.length?r(o):n(new DOMException("Đã hủy chọn media","AbortError"))},{once:!0}),a.addEventListener("cancel",()=>{i(),n(new DOMException("Đã hủy chọn media","AbortError"))},{once:!0}),document.body.append(a),a.click(),t}function Vo(e,t){return`${e}:${t}`}const fa=Ie()(e=>({tasks:new Map,queueTask:(t,r)=>e(n=>{const a=Vo(t,r),i=new Map(n.tasks);return i.set(a,{id:a,mediaId:t,type:r,status:"queued",progress:0,updatedAt:Date.now()}),{tasks:i}}),updateTask:(t,r,n)=>e(a=>{const i=Vo(t,r),o=a.tasks.get(i);if(!o)return a;const s=new Map(a.tasks);return s.set(i,{...o,...n,progress:Math.max(0,Math.min(1,n.progress??o.progress)),updatedAt:Date.now()}),{tasks:s}}),completeTask:(t,r)=>e(n=>{const a=new Map(n.tasks);return a.delete(Vo(t,r)),{tasks:a}}),clearMedia:t=>e(r=>{const n=new Map(r.tasks);for(const[a,i]of n)i.mediaId===t&&n.delete(a);return{tasks:n}}),clearAll:()=>e({tasks:new Map})})),En=Oe("MediaImport"),CC=2;function TC(e,t){const r=Date.now();return{id:t,storageType:"workspace",fileName:e.name,fileSize:e.size,fileLastModified:e.lastModified,mimeType:_s(e),duration:0,width:0,height:0,fps:30,codec:"importing...",bitrate:0,tags:[],createdAt:r,updatedAt:r}}function EC(e){const t={...e};return delete t.isDuplicate,delete t.hasUnsupportedCodec,t}function ih(e){const t=vr([{type:"media.add",params:{media:EC(e)}}]);if(!t.ok)throw new Error(t.issues.map(r=>r.message).join("; ")||t.error)}function oh(e,t){fa.getState().clearMedia(t),e(r=>({mediaItems:r.mediaItems.filter(n=>n.id!==t),importingIds:r.importingIds.filter(n=>n!==t)}))}function FC(e,t,r,n){oh(e,r);const a=t().mediaItems.some(i=>i.id===n.id);return ih(n),a}function Xu(e,t){ih(t),e({error:null,errorLink:null})}function sh(e){pt.canGenerateProxy(e.mimeType)&&pt.setProxyKey(e.id,Td(e))}function AC(e){const t=fa.getState();t.queueTask(e,"import"),t.updateTask(e,"import",{status:"queued",progress:.05})}function _C(e){fa.getState().updateTask(e,"import",{status:"running",progress:.2})}function Qu(e,t,r,n,a){const i=[],o=[],s=[];let l=0,c=0;return e.forEach((u,d)=>{const m=t[d];if(m)if(u.status==="fulfilled"){const{metadata:p,tempId:f,file:h}=u.value,g=FC(r,n,f,p);p.isDuplicate&&g?(o.push(h.name),a?.includeDuplicatesInResults&&i.push(p)):(sh(p),i.push(p),l+=1,p.hasUnsupportedCodec&&p.audioCodec&&s.push({fileName:h.name,audioCodec:p.audioCodec}))}else c++,oh(r,m.tempId),En.error(`Failed to import ${m.file.name}`,u.reason)}),{results:i,importedCount:l,duplicateNames:o,unsupportedCodecFiles:s,failedCount:c}}function OC(e){return e.length===0?"":e.length<=3?e.join(", "):`${e.slice(0,3).join(", ")} và ${e.length-3} tệp khác`}function DC({importedCount:e,duplicateNames:t,unsupportedCodecFiles:r,failedCount:n}){if(!(t.length>0||r.length>0||n>0))return null;const i=[];if(e>0&&i.push(`Đã nhập ${e} tệp.`),t.length>0&&(e===0&&r.length===0&&n===0?i.push(t.length===1?`"${t[0]}" đã có trong thư viện`:`${t.length} tệp đã có trong thư viện`):t.length===1?i.push(`Đã bỏ qua 1 tệp trùng: ${t[0]}.`):i.push(`Đã bỏ qua ${t.length} tệp trùng: ${OC(t)}.`)),r.length>0){const o=[...new Set(r.map(s=>s.audioCodec))].join(", ");i.push(`${r.length} tệp có codec âm thanh không được hỗ trợ (${o}). Có thể không có dạng sóng âm thanh.`)}return n>0&&i.push(n===1?"1 tệp nhập thất bại. Hãy kiểm tra tệp rồi thử lại.":`${n} tệp nhập thất bại. Hãy kiểm tra các tệp rồi thử lại.`),i.length>0?i.join(" "):null}function Pr(e,t,r,n,a){const i=DC({importedCount:e,duplicateNames:t,unsupportedCodecFiles:r,failedCount:n});if(!i)return;const o=n>0||r.length>0?"warning":"info";a().showNotification({type:o,message:i})}function RC(e,t){const r=i=>{const o=[];for(const s of i){const l=crypto.randomUUID(),c=TC(s,l);e(u=>({mediaItems:[c,...u.mediaItems],importingIds:[...u.importingIds,l],error:null})),AC(l),o.push({tempId:l,file:s})}return o},n=async(i,o,s)=>{const l=new Array(i.length);let c=0;const{mediaLibraryService:u}=await s,d=async()=>{for(;c<i.length;){const p=c++,f=i[p];if(f)try{_C(f.tempId);const h=await u.importFile(f.file,{projectId:o});l[p]={status:"fulfilled",value:{metadata:h,tempId:f.tempId,file:f.file}}}catch(h){l[p]={status:"rejected",reason:h}}}},m=Math.min(CC,i.length);return await Promise.all(Array.from({length:m},d)),l},a=async(i,o)=>{const{currentProjectId:s}=t();if(!s)return e({error:"Chưa chọn dự án"}),[];const l=Lt(),c=En.startEvent("import",l);c.merge({source:"drag-drop",projectId:s,fileCount:i.length});const u=dt(),d=r(i),m=await n(d,s,u),{results:p,importedCount:f,duplicateNames:h,unsupportedCodecFiles:g,failedCount:v}=Qu(m,d,e,t,o);if(Pr(f,h,g,v,t),o?.waitForPreparation&&p.length>0){const{mediaLibraryService:S}=await u;await S.waitForMediaPreparation(p.map(b=>b.id))}return c.success({imported:f,duplicates:h.length,failed:v,unsupportedCodecs:g.length}),p};return{importMedia:async()=>{const{currentProjectId:i}=t();if(!i)return e({error:"Chưa chọn dự án"}),[];const o=Lt(),s=En.startEvent("import",o);s.set("source","picker"),s.set("projectId",i);try{const l=await MC({multiple:!0});s.set("fileCount",l.length);const c=dt(),u=r(l),d=await n(u,i,c),{results:m,importedCount:p,duplicateNames:f,unsupportedCodecFiles:h,failedCount:g}=Qu(d,u,e,t);return Pr(p,f,h,g,t),s.success({imported:p,duplicates:f.length,failed:g,unsupportedCodecs:h.length}),m}catch(l){return l instanceof Error&&l.name!=="AbortError"?(e({error:l.message}),s.failure(l)):s.success({outcome:"cancelled",imported:0,duplicates:0,failed:0,unsupportedCodecs:0}),[]}},importMediaFromUrl:async i=>{const{currentProjectId:o}=t(),s=i.trim();if(!o)return e({error:"Chưa chọn dự án",errorLink:null}),[];if(s.length===0)return e({error:"Hãy nhập URL media.",errorLink:null}),[];e({error:null,errorLink:null});const l=Lt(),c=En.startEvent("import",l);c.set("source","url"),c.set("projectId",o);try{const u=new URL(s);c.set("urlHost",u.hostname)}catch{c.set("urlHost","invalid")}try{const{mediaLibraryService:u}=await dt(),d=await u.importMediaFromUrl(s,o);if(d.isDuplicate)return Pr(0,[d.fileName],[],0,t),c.success({imported:0,duplicates:1,failed:0,unsupportedCodecs:0}),[];Xu(e,d),sh(d);const m=d.hasUnsupportedCodec&&d.audioCodec?[{fileName:d.fileName,audioCodec:d.audioCodec}]:[];return Pr(1,[],m,0,t),c.success({imported:1,duplicates:0,failed:0,unsupportedCodecs:m.length}),[d]}catch(u){const d=u instanceof Error?u:new Error(String(u));return e({error:d.message,errorLink:null}),c.failure(d),[]}},importRemoteLottie:async i=>{const{currentProjectId:o}=t();if(!o)return e({error:"Chưa chọn dự án",errorLink:null}),null;e({error:null,errorLink:null});const s=Lt(),l=En.startEvent("import",s);l.set("source","lottiefiles"),l.set("projectId",o),l.set("provider",i.attribution?.provider??"unknown");try{const{mediaLibraryService:c}=await dt(),u=await c.importLottieFromUrl(i.url,o,{fileName:i.fileName,attribution:i.attribution});return u.isDuplicate?(Pr(0,[u.fileName],[],0,t),l.success({imported:0,duplicates:1,failed:0,unsupportedCodecs:0}),u):(Xu(e,u),Pr(1,[],[],0,t),l.success({imported:1,duplicates:0,failed:0,unsupportedCodecs:0}),u)}catch(c){const u=c instanceof Error?c:new Error(String(c));return e({error:u.message,errorLink:null}),l.failure(u),null}},importFiles:i=>a(i),importFilesForPlacement:async i=>a(i,{includeDuplicatesInResults:!0,waitForPreparation:!0})}}const LC=new Set;function BC(e,t=[]){for(const r of LC)r(e,t)}function lh(e){const t=vr([e]);if(!t.ok)throw new Error(t.issues.map(r=>r.message).join("; "))}function zC(e,t){lh({type:"media.removeRecords",params:{mediaIds:t}}),e(r=>({selectedMediaIds:r.selectedMediaIds.filter(n=>!t.includes(n))}))}function UC(e){return e?.aiCaptions?.map(t=>t.thumbRelPath)??[]}function HC(e,t){const r=new Map(t.map(n=>[n.id,n]));for(const n of e)Un.release(n),pt.clearProxyKey(n),BC(n,UC(r.get(n))),fa.getState().clearMedia(n)}function NC(e,t){const r=async(n,a,i)=>{e({error:null});const o=new Set(t().importingIds),s=t().mediaItems.filter(c=>!o.has(c.id)),l=t().selectedMediaIds;zC(e,n);try{await a(),HC(n,s)}catch(c){throw lh({type:"media.setRecords",params:{mediaItems:s}}),e({selectedMediaIds:l,error:c instanceof Error?c.message:i}),c}};return{deleteMedia:async n=>{const{currentProjectId:a}=t();await r([n],async()=>{const{mediaLibraryService:i}=await dt();return a?i.deleteMediaFromProject(a,n):i.deleteMedia(n)},"Không thể xoá tệp")},deleteMediaBatch:async n=>{const{currentProjectId:a}=t();await r(n,async()=>{const{mediaLibraryService:i}=await dt();return a?i.deleteMediaBatchFromProject(a,n):i.deleteMediaBatch(n)},"Không thể xoá các tệp")}}}let qC=null;function Gr(){return qC}const vt=Oe("MediaRelinkingActions");function GC(e,t){const r={...t};delete r.id;const n=vr([{type:"media.update",params:{mediaId:e,updates:r}}]);if(!n.ok)throw new Error(n.issues.map(a=>a.message).join("; "))}function Ju(e,t,r){Un.invalidate(t),GC(t,r);const n=Gr();if(n)for(const a of n.getItemsState().items)a.mediaId===t&&n.updateProjectItem(a.id,{src:void 0,reverseConformSrc:void 0,reverseConformPath:void 0,reverseConformKey:void 0,reverseConformPreviewSrc:void 0,reverseConformPreviewPath:void 0,reverseConformPreviewKey:void 0,reverseConformPreviewUsesProxy:void 0,reverseConformPreviewIsSourceLevel:void 0,reverseConformPreviewSourceDuration:void 0,reverseConformPreviewFps:void 0,reverseConformStatus:void 0,reverseConformLocalStart:void 0});e().markMediaHealthy(t)}function KC(e,t){const r={mediaId:e.id,label:e.fileName,src:void 0,thumbnailUrl:void 0,waveformData:void 0};if(e.width>0&&e.height>0&&(r.sourceWidth=e.width,r.sourceHeight=e.height),e.duration>0){const n=e.fps>0?e.fps:t;r.sourceFps=n,r.sourceDuration=Math.round(e.duration*n)}return r}function ch(e,t){const r=Gr();if(!r)return vt.error("[getOrphanedRelinkTargetIds] Timeline relinking actions are not registered"),[e];const{items:n,itemById:a}=r.getItemsState(),i=a[e];if(!i)return[e];const o=r.getSynchronizedLinkedItems(n,e),s=i.mediaId;if(!s||s===t){const c=o.filter(u=>u.mediaId===t).map(u=>u.id);return c.length>0?c:[e]}const l=o.filter(c=>c.mediaId===s).map(c=>c.id);return l.length>0?l:[e]}function VC(e){const t=new Set;for(const r of e){const a=Gr()?.getItemsState().itemById[r];ch(r,a?.mediaId??"").forEach(o=>t.add(o))}return t.size>0?Array.from(t):e}function WC(e,t){return{markMediaBroken:(r,n)=>{e(a=>{if(a.brokenMediaIds.includes(r))return a;const i=new Map(a.brokenMediaInfo);return i.set(r,n),{brokenMediaIds:[...a.brokenMediaIds,r],brokenMediaInfo:i}})},markMediaHealthy:r=>{e(n=>{if(!n.brokenMediaIds.includes(r)&&!n.brokenMediaInfo.has(r)&&!(n.dismissedMissingMediaIds??[]).includes(r))return n;const a=new Map(n.brokenMediaInfo);return a.delete(r),{brokenMediaIds:n.brokenMediaIds.filter(i=>i!==r),brokenMediaInfo:a,dismissedMissingMediaIds:(n.dismissedMissingMediaIds??[]).filter(i=>i!==r)}})},dismissMissingMediaWarnings:r=>{e(n=>({dismissedMissingMediaIds:Array.from(new Set([...n.dismissedMissingMediaIds??[],...r])),showMissingMediaDialog:!1}))},relinkMedia:async(r,n)=>{try{const{mediaLibraryService:a}=await dt(),i=await a.relinkMediaFile(r,n);return Ju(t,r,i),t().showNotification({type:"success",message:`Đã liên kết lại "${i.fileName}"`}),!0}catch(a){return vt.error("[relinkMedia] error:",a),t().showNotification({type:"error",message:a instanceof Error?a.message:"Không thể liên kết lại tệp"}),!1}},relinkMediaBatch:async r=>{const n=[],a=[],{mediaLibraryService:i}=await dt();for(const{mediaId:o,file:s}of r)try{const l=await i.relinkMediaFile(o,s);Ju(t,o,l),n.push(o)}catch(l){vt.error(`[relinkMediaBatch] error for ${o}:`,l),a.push(o)}return n.length>0?t().showNotification({type:a.length>0?"warning":"success",message:`Đã liên kết lại ${n.length} tệp${a.length>0?`, ${a.length} tệp lỗi`:""}`}):a.length>0&&t().showNotification({type:"error",message:`Không thể liên kết lại ${a.length} tệp`}),{success:n,failed:a}},openMissingMediaDialog:()=>e({showMissingMediaDialog:!0}),closeMissingMediaDialog:()=>e({showMissingMediaDialog:!1}),setOrphanedClips:r=>e({orphanedClips:r}),clearOrphanedClips:()=>e({orphanedClips:[]}),openOrphanedClipsDialog:()=>e({showOrphanedClipsDialog:!0}),closeOrphanedClipsDialog:()=>e({showOrphanedClipsDialog:!1}),relinkOrphanedClip:async(r,n)=>{try{const{mediaLibraryService:a}=await dt(),i=await a.getMedia(n);if(!i)return vt.error(`[relinkOrphanedClip] Media not found: ${n}`),t().showNotification({type:"error",message:"Selected media not found"}),!1;const o=ch(r,n);if(o.length===0)return!1;if(o.every(d=>Gr()?.getItemsState().itemById[d]?.mediaId===n))return e(d=>({orphanedClips:d.orphanedClips.filter(m=>!o.includes(m.itemId))})),!0;const l=Gr();if(!l)return vt.error("[relinkOrphanedClip] Timeline relinking actions are not registered"),t().showNotification({type:"error",message:"Timeline actions are unavailable"}),!1;const c=KC(i,l.getFps()),u=o.filter(d=>l.updateProjectItem(d,c));return u.length===0?(t().showNotification({type:"error",message:"Clip not found"}),!1):(vt.debug(`[relinkOrphanedClip] Relinked clip ${r} to media ${n}`,{relinkedItemIds:u}),e(d=>({orphanedClips:d.orphanedClips.filter(m=>!u.includes(m.itemId))})),t().showNotification({type:"success",message:u.length>1?`Linked clips relinked to "${i.fileName}"`:`Clip relinked to "${i.fileName}"`}),!0)}catch(a){return vt.error("[relinkOrphanedClip] error:",a),t().showNotification({type:"error",message:a instanceof Error?a.message:"Failed to relink clip"}),!1}},removeOrphanedClips:r=>{try{const n=VC(r),a=Gr();if(!a){vt.error("[removeOrphanedClips] Timeline relinking actions are not registered"),t().showNotification({type:"error",message:"Timeline actions are unavailable"});return}a.removeProjectItems(n),e(i=>({orphanedClips:i.orphanedClips.filter(o=>!r.includes(o.itemId))})),t().showNotification({type:"info",message:`Removed ${r.length} orphaned clip${r.length!==1?"s":""}`})}catch(n){vt.error("[removeOrphanedClips] error:",n)}}}}async function jC(e,t){if(e.length===0)return{healthyIds:[],broken:[]};const r=await nt().listMedia(t),n=new Set(r.map(o=>o.id)),a=[],i=[];for(const o of e)n.has(o.id)?a.push(o.id):i.push({mediaId:o.id,fileName:o.fileName,errorType:"file_missing"});return{healthyIds:a,broken:i}}function Il(e){return Math.max(0,Math.min(1,e))}function Zu(e){return e==="downloading"||e==="preparing"}function Wo(e){return e.stage==="preparing"&&Il(e.progress)>=1}function ed(e){const t=Il(e.progress);switch(e.stage){case"queued":return 0;case"downloading":return t*.6;case"preparing":return .6+t*.15;case"decoding":return .75+t*.05;case"transcribing":return .8+t*.2}}function $C(e,t){const r={...t,progress:Il(t.progress)};if(!e||e.stage==="queued")return r;const n=Zu(r.stage),a=Zu(e.stage);return n===a?r.stage==="downloading"&&r.restarted&&e.stage==="preparing"&&!Wo(e)||ed(r)>=ed(e)?r:e:n?Wo(r)?e:r:Wo(e)?r:e}const Fn=Oe("MediaLibraryStore");let Zt=null;function Ps(e,t){const r=vr([{type:e,params:t}]);if(!r.ok)throw new Error(r.issues.map(n=>n.message).join("; "))}function td(e){Ps("media.setRecords",{mediaItems:e,resetImporting:!0})}function Va(e,t){if(e===t)return!0;if(e.length!==t.length)return!1;for(let r=0;r<e.length;r+=1)if(e[r]!==t[r])return!1;return!0}function YC(e){const t={};for(const r of e)t[r.id]=r;return t}function rd(e,t){const r=new Map;for(const n of e)r.set(n.id,t?.has(n.id)?"ready":"idle");return r}async function XC(e){try{const t=await HS(e.map(r=>r.id));return rd(e,t)}catch(t){return Fn.warn("[MediaLibraryStore] Failed to load transcript availability:",t),rd(e)}}function QC(e){return e.filter(t=>pt.canGenerateProxy(t.mimeType))}async function JC(e){const t=QC(e);if(t.length!==0){for(const r of t)pt.setProxyKey(r.id,Td(r));await pt.loadExistingProxies(t.map(r=>r.id))}}const ZC=(e,t)=>({currentProjectId:null,mediaItems:[],mediaById:{},isLoading:!1,importingIds:[],error:null,errorLink:null,notification:null,selectedMediaIds:[],selectedCompositionIds:[],searchQuery:"",filterByType:null,sortBy:"date",viewMode:"grid",mediaItemSize:1,brokenMediaIds:[],brokenMediaInfo:new Map,dismissedMissingMediaIds:[],showMissingMediaDialog:!1,isScanningMediaHealth:!1,orphanedClips:[],showOrphanedClipsDialog:!1,unsupportedCodecFiles:[],showUnsupportedCodecDialog:!1,unsupportedCodecResolver:null,proxyStatus:new Map,proxyProgress:new Map,interpolationStatus:new Map,interpolationProgress:new Map,interpolationStage:new Map,interpolationEtaSeconds:new Map,upscaleStatus:new Map,upscaleProgress:new Map,upscaleStage:new Map,upscaleEtaSeconds:new Map,transcriptStatus:new Map,transcriptProgress:new Map,taggingMediaIds:new Set,analysisProgress:null,setCurrentProject:r=>{t().currentProjectId;const n=t().mediaItems.map(a=>a.id);for(const a of n)pt.clearProxyKey(a);td([]),e({currentProjectId:r,selectedMediaIds:[],selectedCompositionIds:[],isLoading:!!r,proxyStatus:new Map,proxyProgress:new Map,interpolationStatus:new Map,interpolationProgress:new Map,interpolationStage:new Map,interpolationEtaSeconds:new Map,upscaleStatus:new Map,upscaleProgress:new Map,upscaleStage:new Map,upscaleEtaSeconds:new Map,transcriptStatus:new Map,transcriptProgress:new Map,taggingMediaIds:new Set,analysisProgress:null,dismissedMissingMediaIds:[],isScanningMediaHealth:!1}),fa.getState().clearAll()},loadMediaItems:async()=>{const{currentProjectId:r}=t();if(!r){e({isLoading:!1});return}e({isLoading:!0,error:null});const n=Lt(),a=Fn.startEvent("loadMediaItems",n);a.set("projectId",r);try{const{mediaLibraryService:i}=await dt(),o=await i.getMediaForProject(r);td(o),e({isLoading:!1}),a.set("mediaCount",o.length),i.prefetchThumbnails(o.map(l=>({id:l.id,thumbnailId:l.thumbnailId}))).catch(l=>Fn.warn("[MediaLibraryStore] Thumbnail prefetch failed:",l)),t().scanMediaHealth();const s=await XC(o);e({transcriptStatus:s,transcriptProgress:new Map}),a.set("transcriptsReady",[...s.values()].filter(l=>l==="ready").length);try{await JC(o)}catch(l){Fn.warn("[MediaLibraryStore] Proxy initialization failed:",l)}a.success({mediaCount:o.length})}catch(i){a.failure(i instanceof Error?i:new Error(String(i)));const o=i instanceof Error?i.message:"Failed to load media";throw e({error:o,isLoading:!1}),i}},...RC(e,t),...NC(e,t),...WC(e,t),scanMediaHealth:async()=>{const{currentProjectId:r,mediaItems:n}=t();if(!r||n.length===0){e({isScanningMediaHealth:!1});return}e({isScanningMediaHealth:!0});try{const a=await jC(n,r);if(t().currentProjectId!==r)return;for(const i of a.healthyIds)t().markMediaHealthy(i);for(const i of a.broken)t().markMediaBroken(i.mediaId,i)}catch(a){Fn.warn("[MediaLibraryStore] Workspace health scan failed:",a)}finally{t().currentProjectId===r&&e({isScanningMediaHealth:!1})}},prependMediaItem:r=>{Ps("media.add",{media:r})},setSelection:({mediaIds:r,compositionIds:n})=>{const a=t(),i=Va(a.selectedMediaIds,r),o=Va(a.selectedCompositionIds,n);i&&o||e({selectedMediaIds:i?a.selectedMediaIds:r,selectedCompositionIds:o?a.selectedCompositionIds:n})},selectMedia:r=>{const n=t();Va(n.selectedMediaIds,r)||e({selectedMediaIds:r})},selectCompositions:r=>{const n=t();Va(n.selectedCompositionIds,r)||e({selectedCompositionIds:r})},toggleMediaSelection:r=>e(n=>({selectedMediaIds:n.selectedMediaIds.includes(r)?n.selectedMediaIds.filter(a=>a!==r):[...n.selectedMediaIds,r]})),toggleCompositionSelection:r=>e(n=>({selectedCompositionIds:n.selectedCompositionIds.includes(r)?n.selectedCompositionIds.filter(a=>a!==r):[...n.selectedCompositionIds,r]})),clearSelection:()=>{const r=t();r.selectedMediaIds.length===0&&r.selectedCompositionIds.length===0||e({selectedMediaIds:[],selectedCompositionIds:[]})},setSearchQuery:r=>e({searchQuery:r}),setFilterByType:r=>e({filterByType:r}),setSortBy:r=>e({sortBy:r}),setViewMode:r=>e({viewMode:r}),setMediaItemSize:r=>e({mediaItemSize:Math.max(1,Math.min(5,r))}),clearError:()=>e({error:null,errorLink:null}),showNotification:r=>{e({notification:r}),Zt!==null&&clearTimeout(Zt),Zt=setTimeout(()=>{Zt=null,e(n=>n.notification===r?{notification:null}:n)},4e3)},clearNotification:()=>{Zt!==null&&(clearTimeout(Zt),Zt=null),e({notification:null})},confirmUnsupportedCodecs:r=>new Promise(n=>{e({unsupportedCodecFiles:r,showUnsupportedCodecDialog:!0,unsupportedCodecResolver:n})}),resolveUnsupportedCodecDialog:r=>{const{unsupportedCodecResolver:n}=t();n&&n(r),e({unsupportedCodecFiles:[],showUnsupportedCodecDialog:!1,unsupportedCodecResolver:null})},setProxyStatus:(r,n)=>{e(a=>{const i=new Map(a.proxyStatus);return i.set(r,n),{proxyStatus:i}})},clearProxyStatus:r=>{e(n=>{const a=new Map(n.proxyStatus);a.delete(r);const i=new Map(n.proxyProgress);return i.delete(r),{proxyStatus:a,proxyProgress:i}})},setProxyProgress:(r,n)=>{e(a=>{const i=new Map(a.proxyProgress);return i.set(r,n),{proxyProgress:i}})},setInterpolationStatus:(r,n)=>{e(a=>{const i=new Map(a.interpolationStatus);return i.set(r,n),{interpolationStatus:i}})},clearInterpolationStatus:r=>{e(n=>{const a=new Map(n.interpolationStatus);a.delete(r);const i=new Map(n.interpolationProgress);i.delete(r);const o=new Map(n.interpolationStage);o.delete(r);const s=new Map(n.interpolationEtaSeconds);return s.delete(r),{interpolationStatus:a,interpolationProgress:i,interpolationStage:o,interpolationEtaSeconds:s}})},setInterpolationProgress:(r,n,a,i)=>{e(o=>{const s=new Map(o.interpolationProgress);s.set(r,n);const l=new Map(o.interpolationStage);l.set(r,a);const c=new Map(o.interpolationEtaSeconds);return typeof i=="number"&&c.set(r,i),{interpolationProgress:s,interpolationStage:l,interpolationEtaSeconds:c}})},setUpscaleStatus:(r,n)=>{e(a=>{const i=new Map(a.upscaleStatus);return i.set(r,n),{upscaleStatus:i}})},clearUpscaleStatus:r=>{e(n=>{const a=new Map(n.upscaleStatus);a.delete(r);const i=new Map(n.upscaleProgress);i.delete(r);const o=new Map(n.upscaleStage);o.delete(r);const s=new Map(n.upscaleEtaSeconds);return s.delete(r),{upscaleStatus:a,upscaleProgress:i,upscaleStage:o,upscaleEtaSeconds:s}})},setUpscaleProgress:(r,n,a,i)=>{e(o=>{const s=new Map(o.upscaleProgress);s.set(r,n);const l=new Map(o.upscaleStage);l.set(r,a);const c=new Map(o.upscaleEtaSeconds);return typeof i=="number"&&c.set(r,i),{upscaleProgress:s,upscaleStage:l,upscaleEtaSeconds:c}})},setTranscriptStatus:(r,n)=>{e(a=>{const i=new Map(a.transcriptStatus);return i.set(r,n),{transcriptStatus:i}})},setTranscriptProgress:(r,n)=>{e(a=>{const i=new Map(a.transcriptProgress);return i.set(r,$C(i.get(r),n)),{transcriptProgress:i}})},clearTranscriptProgress:r=>{e(n=>{const a=new Map(n.transcriptProgress);return a.delete(r),{transcriptProgress:a}})},setTaggingMedia:(r,n)=>{e(a=>{const i=new Set(a.taggingMediaIds);return n?i.add(r):i.delete(r),{taggingMediaIds:i}})},updateMediaCaptions:(r,n)=>{t().mediaItems.some(a=>a.id===r)&&Ps("media.update",{mediaId:r,updates:{aiCaptions:n,updatedAt:Date.now()}})},beginAnalysisRun:r=>{r<=0||e(n=>{const a=n.analysisProgress;return a?{analysisProgress:{total:a.total+r,completed:a.completed,cancelRequested:a.cancelRequested}}:{analysisProgress:{total:r,completed:0,cancelRequested:!1}}})},incrementAnalysisCompleted:(r=1)=>{e(n=>n.analysisProgress?{analysisProgress:{...n.analysisProgress,completed:Math.min(n.analysisProgress.total,n.analysisProgress.completed+r)}}:n)},requestAnalysisCancel:()=>{e(r=>r.analysisProgress?{analysisProgress:{...r.analysisProgress,cancelRequested:!0}}:r)},endAnalysisRun:()=>{e({analysisProgress:null})}}),eT=Ie()(vd(ZC,{name:"MediaLibraryStore",enabled:!1})),Me=eT;{let e=Me.getState().mediaItems;Me.subscribe(t=>{t.mediaItems!==e&&(e=t.mediaItems,Me.setState({mediaById:YC(t.mediaItems)}))}),pt.onStatusChange((t,r,n)=>{const a=Me.getState();if(r==="idle"){a.clearProxyStatus(t);return}a.setProxyStatus(t,r),n!==void 0&&a.setProxyProgress(t,n)}),pt.setMediaResolver(t=>Me.getState().mediaById[t]),pt.setFilmstripPrewarm(async(t,r,n,a)=>{const{importFilmstripCache:i}=await import("./timeline-services-IPtwXfVx.js"),{filmstripCache:o}=await i();await o.prewarmPriorityWindow(t,r,n,a)})}const tT=()=>{const e=Me(o=>o.mediaItems),t=Me(o=>o.searchQuery),r=Me(o=>o.filterByType),n=Me(o=>o.sortBy);let a=e;if(t){const o=t.toLowerCase();a=a.filter(s=>s.fileName.toLowerCase().includes(o)||s.aiCaptions?.some(l=>l.text.toLowerCase().includes(o)))}return r&&(a=a.filter(o=>g0(o.mimeType)===r)),[...a].sort((o,s)=>{switch(n){case"name":return o.fileName.localeCompare(s.fileName);case"date":return s.createdAt-o.createdAt;case"size":return s.fileSize-o.fileSize;default:return 0}})};var CE=Object.freeze({__proto__:null,useFilteredMediaItems:tT,useMediaLibraryStore:Me});export{pE as $,BT as A,jT as B,wp as C,WT as D,NT as E,jo as F,od as G,Me as H,hE as I,gE as J,Wr as K,wT as L,ia as M,xT as N,fE as O,Rs as P,PT as Q,ST as R,MT as S,Un as T,Tt as U,bE as V,we as W,rs as X,IT as Y,en as Z,mE as _,gd as a,yE as a$,Gd as a0,kT as a1,GT as a2,_S as a3,YT as a4,lp as a5,ZT as a6,OS as a7,oa as a8,vc as a9,$T as aA,aE as aB,AS as aC,cT as aD,pT as aE,fT as aF,hT as aG,Sh as aH,Ih as aI,uT as aJ,mT as aK,dT as aL,AT as aM,ev as aN,js as aO,fi as aP,DT as aQ,Fb as aR,_T as aS,OT as aT,ow as aU,Zp as aV,K1 as aW,bT as aX,vT as aY,Xs as aZ,rf as a_,eE as aa,tE as ab,rE as ac,nE as ad,iE as ae,BS as af,oE as ag,LS as ah,zS as ai,XT as aj,QT as ak,JT as al,lE as am,cE as an,uE as ao,yT as ap,_s as aq,sE as ar,pt as as,Td as at,dE as au,Wl as av,y0 as aw,gT as ax,xE as ay,LT as az,hr as b,vE as b0,pw as b1,Pp as b2,Mp as b3,Cp as b4,bf as b5,qm as b6,Nm as b7,Hm as b8,Py as b9,RT as bA,IE as bB,Q0 as bC,CE as bD,TT as ba,ns as bb,Vb as bc,ET as bd,Ky as be,FT as bf,SE as bg,Ph as bh,kE as bi,PE as bj,ME as bk,Cy as bl,My as bm,wE as bn,_m as bo,Mb as bp,_y as bq,Vy as br,CT as bs,$y as bt,zm as bu,Wy as bv,id as bw,ad as bx,Ob as by,Rb as bz,vS as c,bS as d,KT as e,yS as f,UT as g,VT as h,sT as i,Zs as j,fp as k,HT as l,yp as m,qT as n,v0 as o,i0 as p,lS as q,gp as r,lT as s,iT as t,oT as u,cp as v,xp as w,mS as x,zT as y,dp as z};
