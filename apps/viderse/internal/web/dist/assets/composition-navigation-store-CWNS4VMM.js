import{c as He,a as rr}from"./audio-measurement.worker-DBGOBuQt.js";import{c as _e,u as ae,D as jr,g as q0,b as K0,E as Uc,d as G0,Z as W0,e as V0,f as Z0,n as Nc,h as Y0}from"./compositions-store-DLrn8Bqe.js";const Wf=1;function X0(e,t){return Math.abs(e-t)<=Wf}function Q0(e,t){return t<e-Wf}function J0(e){return Math.max(0,Math.min(1,e??.5))}function gn(e,t){const r=Math.max(1,Math.floor(e)),n=J0(t),i=Math.floor(r*n),o=r-i;return{leftPortion:i,rightPortion:o}}function Vf(e,t,r){const n=Math.max(0,Math.floor(e)),i=Math.max(0,Math.floor(t)),o=Math.max(0,Math.floor(r)),a=i+o;if(a<=n)return{incomingPortion:i,outgoingPortion:o};if(n===0)return{incomingPortion:0,outgoingPortion:0};const s=n/a;let l=Math.floor(i*s),c=Math.floor(o*s),u=n-(l+c);for(;u>0;){const d=i-l,f=o-c;if(d===0&&f===0)break;d>=f&&d>0?l+=1:f>0?c+=1:d>0&&(l+=1),u-=1}return{incomingPortion:l,outgoingPortion:c}}function rD(e,t){const r=new Map,n=new Map,i=new Map;for(const a of e){const s=t.get(a.leftClipId),l=t.get(a.rightClipId);if(!s||!l)continue;const c=s.from+s.durationInFrames;if(X0(c,l.from)){const d=l.from,f=gn(a.durationInFrames,a.alignment);r.set(a.id,{transition:a,leftClip:s,rightClip:l,cutPoint:d,leftPortion:f.leftPortion,rightPortion:f.rightPortion,mode:"cut-centered"}),i.set(s.id,a.id),n.set(l.id,a.id);continue}if(!Q0(c,l.from))continue;const u=c-l.from;r.set(a.id,{transition:a,leftClip:s,rightClip:l,cutPoint:c,leftPortion:u,rightPortion:u,mode:"legacy-overlap"}),i.set(s.id,a.id),n.set(l.id,a.id)}for(const[a,s]of n){const l=i.get(a);if(!l)continue;const c=r.get(s),u=r.get(l),d=t.get(a);if(!c||!u||!d)continue;const f=Vf(d.durationInFrames,c.rightPortion,u.leftPortion);c.rightPortion=f.incomingPortion,u.leftPortion=f.outgoingPortion}const o=[];for(const a of r.values()){const s=a.leftClip,l=a.rightClip,c=s.from+s.durationInFrames,u=a.mode==="cut-centered"?a.cutPoint-Math.max(0,a.leftPortion):l.from,d=a.mode==="cut-centered"?a.cutPoint+Math.max(0,a.rightPortion):c,f=Math.max(1,d-u);o.push({transition:a.transition,leftClip:a.leftClip,rightClip:a.rightClip,cutPoint:a.cutPoint,startFrame:u,endFrame:d,durationInFrames:f,leftPortion:Math.max(0,a.leftPortion),rightPortion:Math.max(0,a.rightPortion)})}return o.toSorted((a,s)=>a.startFrame!==s.startFrame?a.startFrame-s.startFrame:a.cutPoint!==s.cutPoint?a.cutPoint-s.cutPoint:a.transition.id.localeCompare(s.transition.id))}function Zf(e){const t=new Set(e.filter(n=>!n.isGroup&&n.parentTrackId).map(n=>n.parentTrackId)),r=e.filter(n=>!n.isGroup||t.has(n.id));return r.length===e.length?e:r}function ey(e,t){const r=new Set(t.map(i=>i.trackId)),n=e.filter(i=>i.isGroup||!i.parentTrackId||r.has(i.id));return Zf(n)}function Yf(e){const t=new Map(e.filter(r=>r.isGroup).map(r=>[r.id,r]));return e.filter(r=>!r.isGroup).map(r=>{const n=r.parentTrackId?t.get(r.parentTrackId):void 0;return n?{...r,locked:r.locked||n.locked,muted:r.muted||n.muted,visible:r.visible!==!1&&n.visible!==!1,solo:r.solo||n.solo}:r})}const ty=.1,ry=16,qo=1,qt=30;function yn(e,t){return e===void 0||!Number.isFinite(e)||e<=0?t:e}function Fi(e){return sr(e)?{sourceStart:e.sourceStart??e.trimStart??0,sourceEnd:e.sourceEnd,sourceDuration:e.sourceDuration,sourceFps:e.sourceFps,speed:e.speed??qo}:{sourceStart:0,sourceEnd:void 0,sourceDuration:void 0,sourceFps:void 0,speed:qo}}function zl(e,t=!1){if(e.type!=="video")return"not-video";if(e.embeddedAudioMuted!==!0||t)return"unmuted";const r=e.sourceStart??e.trimStart??e.offset??0,n=e.sourceEnd??e.sourceDuration;return n===void 0||!Number.isFinite(n)||n<=r?"no-window":null}function De(e,t,r=qt,n=r){const i=yn(r,qt),o=yn(n,i),a=e/i;return Math.round(a*o*t)}function Kt(e,t,r=qt,n=r){const i=yn(r,qt),o=yn(n,i),a=e/i;return Math.floor(a*o/t)}function Xf(e){const t=e.wrapperSpeed??qo,r=e.wrapperSourceFps??e.fallbackSourceFps,n=e.wrapperSourceEnd??e.wrapperSourceStart+De(e.wrapperDuration,t,e.timelineFps,r),i=e.itemStart+e.itemDuration,o=Math.max(e.itemStart,e.wrapperSourceStart),a=Math.min(i,n);if(a<=o)return null;const s=Kt(o-e.wrapperSourceStart,t,r,e.timelineFps),l=Kt(a-e.wrapperSourceStart,t,r,e.timelineFps);return{overlapStart:o,overlapEnd:a,mappedFrom:s,mappedEnd:l,mappedDuration:Math.max(1,l-s),clippedStartFrames:o-e.itemStart,clippedEndFrames:i-a,wrapperSpeed:t,wrapperSourceFps:r}}function Qf(e,t){return Math.max(0,e-t)}function ny(e,t,r,n=qt,i=n){const o=Qf(e,t);return Kt(o,r,n,i)}function Na(e,t,r=qt,n=r){return Kt(e,t,r,n)}function iy(e,t,r=qt,n=r){if(t<=0)return qo;const i=yn(r,qt),o=yn(n,i);return e*o/(t*i)}function oy(e){return Math.max(ty,Math.min(ry,e))}function sr(e){return e.type==="video"||e.type==="audio"||e.type==="composition"||e.type==="shape"&&!!e.waveformVisualizer}function ay(e,t,r,n,i=qt,o=i){const a=De(t,n,i,o),s=De(t+r,n,i,o);return{left:{sourceEnd:e+a},right:{sourceStart:e+a,sourceEnd:e+s}}}let Jf=null;function Dl(){return Jf}function er(e){Jf=e}const ep=1920,tp=1080;function rp(){return{mint:()=>crypto.randomUUID()}}let _o=rp();function Z(){return _o.mint()}function $c(e,t){const r=_o;_o=e;try{return t()}finally{_o=r}}const at=_e()(e=>({markers:[],inPoint:null,outPoint:null,setMarkers:t=>e({markers:t}),setInPoint:t=>e({inPoint:t}),setOutPoint:t=>e({outPoint:t}),setInOutPoints:(t,r)=>e({inPoint:t,outPoint:r}),addMarker:(t,r="#3B82F6",n="")=>e(i=>({markers:[...i.markers,{id:Z(),frame:t,color:r,label:n}]})),updateMarker:(t,r)=>e(n=>({markers:n.markers.map(i=>i.id===t?{...i,...r}:i)})),removeMarker:t=>e(r=>({markers:r.markers.filter(n=>n.id!==t)})),clearAllMarkers:()=>e({markers:[]}),clearInOutPoints:()=>e({inPoint:null,outPoint:null})}));function sy(e){return[...new Set(e)]}const _r=_e()((e,t)=>({topLevelSequenceIds:[],sequenceViewById:{},setTopLevelSequenceIds:r=>e({topLevelSequenceIds:sy(r)}),addTopLevelSequence:r=>e(n=>n.topLevelSequenceIds.includes(r)?n:{topLevelSequenceIds:[...n.topLevelSequenceIds,r]}),removeTopLevelSequence:r=>e(n=>{if(!n.topLevelSequenceIds.includes(r))return n;const{[r]:i,...o}=n.sequenceViewById;return{topLevelSequenceIds:n.topLevelSequenceIds.filter(a=>a!==r),sequenceViewById:o}}),reorderTopLevelSequences:(r,n)=>e(i=>{const o=i.topLevelSequenceIds;if(r<0||r>=o.length||n<0||n>=o.length||r===n)return i;const a=[...o],[s]=a.splice(r,1);return a.splice(n,0,s),{topLevelSequenceIds:a}}),isTopLevelSequence:r=>t().topLevelSequenceIds.includes(r),pruneToValidSequenceIds:r=>e(n=>{const i=new Set(r),o=n.topLevelSequenceIds.filter(a=>i.has(a));if(o.length===n.topLevelSequenceIds.length){const a=Hc(n.sequenceViewById,i);return a===n.sequenceViewById?n:{sequenceViewById:a}}return{topLevelSequenceIds:o,sequenceViewById:Hc(n.sequenceViewById,i)}}),saveSequenceView:(r,n)=>e(i=>({sequenceViewById:{...i.sequenceViewById,[r]:n}})),getSequenceView:r=>t().sequenceViewById[r],removeSequenceView:r=>e(n=>{if(!(r in n.sequenceViewById))return n;const{[r]:i,...o}=n.sequenceViewById;return{sequenceViewById:o}}),reset:()=>e({topLevelSequenceIds:[],sequenceViewById:{}})}));function Hc(e,t){const r=Object.entries(e).filter(([n])=>t.has(n));return r.length===Object.keys(e).length?e:Object.fromEntries(r)}function vn(e,t,r){return e.flatMap(n=>{const i=Math.max(t,n.startFrame),o=Math.min(r,n.endFrame);return o>i?[{...n,startFrame:i-t,endFrame:o-t}]:[]})}function ly(e,t,r){return[vn(e,0,t),vn(e,t,r)]}function cy(e,t,r){if(!(t>0&&r>0))throw new Error("Tốc độ phải lớn hơn 0.");const n=t/r;return e.map(i=>({...i,startFrame:Math.round(i.startFrame*n),endFrame:Math.max(Math.round(i.startFrame*n)+1,Math.round(i.endFrame*n))}))}function uy(e,t){return e.type==="video"&&t.type==="audio"||e.type==="audio"&&t.type==="video"}function dy(e,t){return!uy(e,t)||!e.originId||e.originId!==t.originId||!e.mediaId||e.mediaId!==t.mediaId?!1:e.from===t.from&&e.durationInFrames===t.durationInFrames}function fy(e,t,r){return t.id===e.id||t.type!==r?!1:e.linkedGroupId&&t.linkedGroupId?e.linkedGroupId===t.linkedGroupId:dy(e,t)}function bn(e){return e.type==="audio"&&typeof e.compositionId=="string"&&e.compositionId.length>0}function Rl(e,t){return t.linkedGroupId?e.find(r=>r.id!==t.id&&bn(r)&&r.linkedGroupId===t.linkedGroupId&&r.compositionId===t.compositionId)??null:null}function np(e,t){return t.linkedGroupId?e.find(r=>r.type==="composition"&&r.linkedGroupId===t.linkedGroupId&&r.compositionId===t.compositionId)??null:null}function ip(e,t){return t.type==="composition"?Rl(e,t):t.type!=="video"?null:e.find(r=>fy(t,r,"audio"))??null}function Gs(e,t){return ip(e,t)!==null}function nD(e){const t=new Set;for(const r of e)r.type==="video"&&Gs(e,r)&&t.add(r.id);return t}function op(e){const t=e.type==="composition"||bn(e)?e.compositionId:null;if(t){const r=ae.getState().getComposition(t);if(r)return r.durationInFrames}return Fi(e).sourceDuration}function yi(e,t,r,n=30){let i=r,o=null;if(sr(e)){const{sourceStart:a,sourceEnd:s,sourceFps:l,speed:c}=Fi(e),u=op(e),d=l??n,f=e.type==="video"&&e.loop===!0;if(t==="start")r<0&&!f&&(o=e.isReversed===!0&&u!==void 0?Na(u-(s??u),c,d,n):Na(a,c,d,n),-r>o&&(i=-o));else if(!f){if(e.isReversed===!0){const p=e.durationInFrames+Na(a,c,d,n);o=p-e.durationInFrames,e.durationInFrames+r>p&&(i=p-e.durationInFrames)}else if(u!==void 0){const p=ny(u,a,c,d,n);o=p-e.durationInFrames,e.durationInFrames+r>p&&(i=p-e.durationInFrames)}}}return i=py(e.durationInFrames,t,i),{clampedAmount:i,maxExtend:o}}function vi(e,t,r,n,i){const o=e.from+e.durationInFrames;if(t==="end"&&r>0){let a=1/0;for(const s of n)s.id!==e.id&&s.trackId===e.trackId&&(i?.has(s.id)||s.from>=o&&(a=Math.min(a,s.from)));if(a!==1/0){const s=a-o;if(r>s)return s}}else if(t==="start"&&r<0){let a=-1/0;for(const s of n){if(s.id===e.id||s.trackId!==e.trackId||i?.has(s.id))continue;const l=s.from+s.durationInFrames;l<=e.from&&(a=Math.max(a,l))}if(a!==-1/0){const s=e.from-a;if(-r>s)return s>0?-s:0}}return r}function py(e,t,r){if(t==="start"){if(e-r<=0)return e-1}else if(e+r<=0)return-e+1;return r}function jc(e,t,r,n,i=30){if(!sr(e)||e.type==="video"&&e.loop===!0)return null;const{sourceStart:o,sourceEnd:a,sourceFps:s,speed:l}=Fi(e),c=op(e),u=s??i;if(t==="start"){const d=De(r,l,i,u);if(e.isReversed===!0){const p=(a??c??o+1)-d;return{sourceEnd:Math.max(o+1,c!==void 0?Math.min(c,p):p)}}return{sourceStart:o+d}}else{const d=De(r,l,i,u);if(e.isReversed===!0){const x=a??c??o+1;return{sourceStart:Math.min(x-1,Math.max(0,o-d))}}const f=e.sourceEnd,p=o+De(n,l,i,u),m=f!==void 0?f+d:p,h=o+1,g=Math.max(h,m);return{sourceEnd:c!==void 0?Math.min(g,c):g}}}function ap(e){return!!e.compositionId&&(e.type==="composition"||e.type==="audio")}function Ll(e){const t=new Set;for(const r of e)ap(r)&&t.add(r.compositionId);return[...t]}function sp(e,t,r,n=new Set){if(e===t)return!0;if(n.has(e))return!1;n.add(e);const i=r[e];if(!i)return!1;for(const o of Ll(i.items))if(o===t||sp(o,t,r,n))return!0;return!1}function lp(e){const{parentCompositionId:t,insertedCompositionId:r,compositionById:n}=e;return t?sp(r,t,n):!1}function cp(e,t,r=new Set){const n=[];for(const i of Ll(e)){if(r.has(i))continue;r.add(i),n.push(i);const o=t[i];o&&n.push(...cp(o.items,t,r))}return n}function iD(e,t){const r=e.flatMap(n=>n.items??[]);return cp(r,t)}const up=/^V(\d+)$/i,dp=/^A(\d+)$/i;function fp(e){return e==="video"?up:dp}function my(e){return e==="video"?"V":"A"}function gr(e){return e.kind==="video"||e.kind==="audio"?e.kind:up.test(e.name)?"video":dp.test(e.name)?"audio":null}function Bl(e,t){const r=fp(t),n=new Set;for(const o of e){if(gr(o)!==t)continue;const s=o.name.match(r),l=s?.[1]?Number.parseInt(s[1],10):NaN;Number.isFinite(l)&&l>0&&n.add(l)}let i=1;for(;n.has(i);)i+=1;return`${my(t)}${i}`}function hy(e){const t=new Map,r=o=>e.filter(a=>!a.isGroup&&gr(a)===o&&fp(o).test(a.name));r("video").sort((o,a)=>a.order-o.order).forEach((o,a)=>t.set(o.id,`V${a+1}`)),r("audio").sort((o,a)=>o.order-a.order).forEach((o,a)=>t.set(o.id,`A${a+1}`));let n=!1;const i=e.map(o=>{const a=t.get(o.id);return a&&a!==o.name?(n=!0,{...o,name:a}):o});return n?i:e}function pp(e,t,r){const n=[...e].sort((a,s)=>a.order-s.order),i=n.findIndex(a=>a.id===t.id);if(i===-1)return r==="above"?(t.order??0)-1:(t.order??0)+1;if(r==="above"){const a=n[i-1];return a?(a.order+t.order)/2:t.order-1}const o=n[i+1];return o?(t.order+o.order)/2:t.order+1}function mp(e){const{tracks:t,kind:r,order:n,height:i=jr}=e;return{id:`track-${Z()}`,name:Bl(t,r),kind:r,height:i,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,volume:0,order:n,items:[]}}function hp(e){const{tracks:t,targetTrack:r,kind:n,direction:i}=e;return t.filter(a=>!a.isGroup).filter(a=>gr(a)===n).filter(a=>i==="above"?a.order<r.order:a.order>r.order).sort((a,s)=>i==="above"?s.order-a.order:a.order-s.order)[0]??null}const gy={edit:{colorScopesOpen:!1,clipInspectorTab:"video",activeTab:"media",propertiesFullColumn:!1},color:{colorScopesOpen:!0,clipInspectorTab:"effects",activeTab:"effects",propertiesFullColumn:!0},motion:{colorScopesOpen:!1,clipInspectorTab:"video",activeTab:"media",propertiesFullColumn:!1}},yy="edit";function qc(e){return e==="color"?"color":e==="animate"||e==="motion"||e==="compose"?"motion":yy}const vy=["media","text","shapes","effects","transitions","lottie","transcript","ai"],by=["video","motion","audio","effects"];function Sy(e){return vy.includes(e)}function Iy(e){return by.includes(e)}function xy(e,t){const r=gy[t];if(!e||typeof e!="object")return r;const n=e;return{colorScopesOpen:typeof n.colorScopesOpen=="boolean"?n.colorScopesOpen:r.colorScopesOpen,clipInspectorTab:Iy(n.clipInspectorTab)?n.clipInspectorTab:r.clipInspectorTab,activeTab:Sy(n.activeTab)?n.activeTab:r.activeTab,propertiesFullColumn:typeof n.propertiesFullColumn=="boolean"?n.propertiesFullColumn:r.propertiesFullColumn}}const Kc={BASE_URL:"/",DEV:!1,MODE:"production",PROD:!0,SSR:!1},Gc=e=>!!e.dispatchFromDevtools&&typeof e.dispatch=="function",bi=new Map,Vi=e=>{const t=bi.get(e);return t?Object.fromEntries(Object.entries(t.stores).map(([r,n])=>[r,n.getState()])):{}},wy=(e,t,r)=>{if(e===void 0)return{type:"untracked",connection:t.connect(r)};const n=bi.get(r.name);if(n)return{type:"tracked",store:e,...n};const i={connection:t.connect(r),stores:{}};return bi.set(r.name,i),{type:"tracked",store:e,...i}},ky=(e,t)=>{if(t===void 0)return;const r=bi.get(e);r&&(delete r.stores[t],Object.keys(r.stores).length===0&&bi.delete(e))},Py=e=>{var t,r;if(!e)return;const n=e.split(`
`),i=n.findIndex(a=>a.includes("api.setState"));if(i<0)return;const o=((t=n[i+1])==null?void 0:t.trim())||"";return(r=/.+ (.+) .+/.exec(o))==null?void 0:r[1]},Ty=(e,t={})=>(r,n,i)=>{const{enabled:o,anonymousActionType:a,store:s,...l}=t;let c;try{c=(o??(Kc?"production":void 0)!=="production")&&window.__REDUX_DEVTOOLS_EXTENSION__}catch{}if(!c)return e(r,n,i);const{connection:u,...d}=wy(s,c,l);let f=!0;i.setState=((h,g,v)=>{const x=r(h,g);if(!f)return x;const b=v===void 0?{type:a||Py(new Error().stack)||"anonymous"}:typeof v=="string"?{type:v}:v;return s===void 0?(u?.send(b,n()),x):(u?.send({...b,type:`${s}/${b.type}`},{...Vi(l.name),[s]:i.getState()}),x)}),i.devtools={cleanup:()=>{u&&typeof u.unsubscribe=="function"&&u.unsubscribe(),ky(l.name,s)}};const p=(...h)=>{const g=f;f=!1,r(...h),f=g},m=e(i.setState,n,i);if(d.type==="untracked"?u?.init(m):(d.stores[d.store]=i,u?.init(Object.fromEntries(Object.entries(d.stores).map(([h,g])=>[h,h===d.store?m:g.getState()])))),Gc(i)){let h=!1;const g=i.dispatch;i.dispatch=(...v)=>{(Kc?"production":void 0)!=="production"&&v[0].type==="__setState"&&!h&&(console.warn('[zustand devtools middleware] "__setState" action type is reserved to set state from the devtools. Avoid using it.'),h=!0),g(...v)}}return u.subscribe(h=>{var g;switch(h.type){case"ACTION":if(typeof h.payload!="string"){console.error("[zustand devtools middleware] Unsupported action format");return}return $a(h.payload,v=>{if(v.type==="__setState"){if(s===void 0){p(v.state);return}Object.keys(v.state).length!==1&&console.error(`
                    [zustand devtools middleware] Unsupported __setState action format.
                    When using 'store' option in devtools(), the 'state' should have only one key, which is a value of 'store' that was passed in devtools(),
                    and value of this only key should be a state object. Example: { "type": "__setState", "state": { "abc123Store": { "foo": "bar" } } }
                    `);const x=v.state[s];if(x==null)return;JSON.stringify(i.getState())!==JSON.stringify(x)&&p(x);return}Gc(i)&&i.dispatch(v)});case"DISPATCH":switch(h.payload.type){case"RESET":return p(m),s===void 0?u?.init(i.getState()):u?.init(Vi(l.name));case"COMMIT":if(s===void 0){u?.init(i.getState());return}return u?.init(Vi(l.name));case"ROLLBACK":return $a(h.state,v=>{if(s===void 0){p(v),u?.init(i.getState());return}p(v[s]),u?.init(Vi(l.name))});case"JUMP_TO_STATE":case"JUMP_TO_ACTION":return $a(h.state,v=>{if(s===void 0){p(v);return}JSON.stringify(i.getState())!==JSON.stringify(v[s])&&p(v[s])});case"IMPORT_STATE":{const{nextLiftedState:v}=h.payload,x=(g=v.computedStates.slice(-1)[0])==null?void 0:g.state;if(!x)return;p(s===void 0?x:x[s]),u?.send(null,v);return}case"PAUSE_RECORDING":return f=!f}return}}),m},gp=Ty,$a=(e,t)=>{let r;try{r=JSON.parse(e)}catch(n){console.error("[zustand devtools middleware] Could not parse the received json",n)}r!==void 0&&t(r)};function yp(e,t){let r;try{r=e()}catch{return}return{getItem:i=>{var o;const a=l=>l===null?null:JSON.parse(l,void 0),s=(o=r.getItem(i))!=null?o:null;return s instanceof Promise?s.then(a):a(s)},setItem:(i,o)=>r.setItem(i,JSON.stringify(o,void 0)),removeItem:i=>r.removeItem(i)}}const Ws=e=>t=>{try{const r=e(t);return r instanceof Promise?r:{then(n){return Ws(n)(r)},catch(n){return this}}}catch(r){return{then(n){return this},catch(n){return Ws(n)(r)}}}},Ey=(e,t)=>(r,n,i)=>{let o={storage:yp(()=>window.localStorage),partialize:g=>g,version:0,merge:(g,v)=>({...v,...g}),...t},a=!1,s=0;const l=new Set,c=new Set;let u=o.storage;if(!u)return e((...g)=>{console.warn(`[zustand persist middleware] Unable to update item '${o.name}', the given storage is currently unavailable.`),r(...g)},n,i);const d=()=>{const g=o.partialize({...n()});return u.setItem(o.name,{state:g,version:o.version})},f=i.setState;i.setState=(g,v)=>(f(g,v),d());const p=e((...g)=>(r(...g),d()),n,i);i.getInitialState=()=>p;let m;const h=()=>{var g,v;if(!u)return;const x=++s;a=!1,l.forEach(w=>{var E;return w((E=n())!=null?E:p)});const b=((v=o.onRehydrateStorage)==null?void 0:v.call(o,(g=n())!=null?g:p))||void 0;return Ws(u.getItem.bind(u))(o.name).then(w=>{if(w)if(typeof w.version=="number"&&w.version!==o.version){if(o.migrate){const E=o.migrate(w.state,w.version);return E instanceof Promise?E.then(k=>[!0,k]):[!0,E]}console.error("State loaded from storage couldn't be migrated since no migrate function was provided")}else return[!1,w.state];return[!1,void 0]}).then(w=>{var E;if(x!==s)return;const[k,T]=w;if(m=o.merge(T,(E=n())!=null?E:p),r(m,!0),k)return d()}).then(()=>{x===s&&(b?.(n(),void 0),m=n(),a=!0,c.forEach(w=>w(m)))}).catch(w=>{x===s&&b?.(void 0,w)})};return i.persist={setOptions:g=>{o={...o,...g},g.storage&&(u=g.storage)},clearStorage:()=>{u?.removeItem(o.name)},getOptions:()=>o,rehydrate:()=>h(),hasHydrated:()=>a,onHydrate:g=>(l.add(g),()=>{l.delete(g)}),onFinishHydration:g=>(c.add(g),()=>{c.delete(g)})},o.skipHydration||h(),m||p},vp=Ey,Cy=[1,2,4];function My(e,t){if(!Number.isFinite(e)||Math.sign(e)!==t)return t;const r=Math.abs(e),n=Cy.find(i=>i>r)??4;return t*n}const Ha=new Map,_y={getItem:e=>Ha.get(e)??null,setItem:(e,t)=>{Ha.set(e,t)},removeItem:e=>{Ha.delete(e)}};function Fo(e){return Number.isFinite(e)?Math.max(0,Math.round(e)):0}function Fy(e){return e===.5||e===.33||e===.25?e:1}function bp(e){if(e.isPlaying&&e.previewFrame===null&&e.previewItemId===null)return e;if(e.previewFrame===null&&e.previewItemId===null)return{isPlaying:!0};const t=e.frameUpdateEpoch+1;return{isPlaying:!0,previewFrame:null,previewItemId:null,previewFrameEpoch:t,frameUpdateEpoch:t}}function Wc(e){const t=bp(e);return t===e?e.playbackRate===1&&e.transportMode==="normal"?e.playbackScrubResumeTransport===null?e:{playbackScrubResumeTransport:null}:{playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}:{...t,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}}function Vc(e,t){const r=bp(e);return{...r===e?{}:r,playbackRate:e.isPlaying?My(e.playbackRate,t):t,transportMode:"shuttle",playbackScrubResumeTransport:null}}function Ay(e,t,r){const n=Fo(t),i=r??null;if(e.currentFrame===n&&e.previewFrame===null&&i===null||e.currentFrame===n&&e.previewFrame===n&&e.previewItemId===i)return e;const o=e.frameUpdateEpoch+1;return{currentFrame:n,currentFrameEpoch:o,previewFrame:n,previewItemId:i,previewFrameEpoch:o,frameUpdateEpoch:o}}const be=_e()(vp(e=>({currentFrame:0,currentFrameEpoch:0,isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null,loop:!1,volume:1,muted:!1,masterBusDb:0,busAudioEq:void 0,zoom:-1,previewFrame:null,previewFrameEpoch:0,frameUpdateEpoch:0,previewItemId:null,useProxy:!0,previewQuality:1,compositionVisualFrozen:!1,setCurrentFrame:t=>e(r=>{const n=Fo(t);if(r.currentFrame===n)return r;const i=r.frameUpdateEpoch+1;return{currentFrame:n,currentFrameEpoch:i,frameUpdateEpoch:i}}),setScrubFrame:(t,r)=>e(n=>n.isPlaying?n:Ay(n,t,r)),finishScrub:t=>e(r=>{const n=Fo(t);if(r.currentFrame===n&&r.previewFrame===null&&r.previewItemId===null&&!r.compositionVisualFrozen)return r;const i=r.frameUpdateEpoch+1;return{currentFrame:n,currentFrameEpoch:i,previewFrame:null,previewItemId:null,previewFrameEpoch:i,frameUpdateEpoch:i,compositionVisualFrozen:!1}}),beginPlaybackScrub:()=>e(t=>({isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:t.isPlaying?{playbackRate:t.playbackRate,transportMode:t.transportMode}:null})),resumePlaybackAfterScrub:()=>e(t=>{const r=t.playbackScrubResumeTransport;return r?t.isPlaying?{playbackScrubResumeTransport:null}:{isPlaying:!0,playbackRate:r.playbackRate,transportMode:r.transportMode,playbackScrubResumeTransport:null}:t}),cancelPlaybackScrubResume:()=>e(t=>t.playbackScrubResumeTransport===null?t:{playbackScrubResumeTransport:null}),play:()=>e(Wc),pause:()=>e(t=>t.isPlaying||t.playbackRate!==1||t.transportMode!=="normal"||t.playbackScrubResumeTransport!==null?{isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}:t),togglePlayPause:()=>e(t=>t.isPlaying?{isPlaying:!1,playbackRate:1,transportMode:"normal",playbackScrubResumeTransport:null}:Wc(t)),shuttleForward:()=>e(t=>Vc(t,1)),shuttleReverse:()=>e(t=>Vc(t,-1)),setPlaybackRate:t=>e({playbackRate:t,playbackScrubResumeTransport:null}),toggleLoop:()=>e(t=>({loop:!t.loop})),setVolume:t=>e({volume:t}),toggleMute:()=>e(t=>({muted:!t.muted})),setMuted:t=>e(r=>r.muted===t?r:{muted:t}),setMasterBusDb:t=>e({masterBusDb:Math.max(-60,Math.min(12,t))}),setBusAudioEq:t=>e({busAudioEq:t}),setZoom:t=>e({zoom:t}),setPreviewFrame:(t,r)=>e(n=>{if(n.isPlaying&&t!==null)return n;const i=t==null?null:Fo(t),o=t==null?null:r??null;if(n.previewFrame===i&&n.previewItemId===o)return n;const a=n.frameUpdateEpoch+1;return{previewFrame:i,previewItemId:o,previewFrameEpoch:a,frameUpdateEpoch:a}}),toggleUseProxy:()=>e(t=>({useProxy:!t.useProxy})),setPreviewQuality:t=>e(r=>{const n=Fy(t);return r.previewQuality===n?r:{previewQuality:n}}),setCompositionVisualFrozen:t=>e(r=>r.compositionVisualFrozen===t?r:{compositionVisualFrozen:t})}),{name:"playback-storage",storage:yp(()=>typeof localStorage>"u"||typeof localStorage.setItem!="function"?_y:localStorage),partialize:e=>({zoom:e.zoom,volume:e.volume,muted:e.muted,loop:e.loop,useProxy:e.useProxy,previewQuality:e.previewQuality}),merge:(e,t)=>({...t,...e,isPlaying:!1,playbackRate:1,transportMode:"normal"})})),Zc=320,Sp="editor:workspace",Ip="editor:trackSizePreset";function Oy(){try{const e=localStorage.getItem(Ip);if(e==="compact"||e==="medium"||e==="large")return e}catch{}return"medium"}function xp(e){return`editor:workspaceLayout:${e}`}function zy(){try{return qc(localStorage.getItem(Sp))}catch{return qc(null)}}function Dy(){try{return localStorage.getItem("editor:propertiesFullColumn")==="true"}catch{return!1}}function wp(e){let t=null;try{const i=localStorage.getItem(xp(e));t=i===null?null:JSON.parse(i)}catch{t=null}const r=xy(t,e),n=typeof t?.propertiesFullColumn=="boolean";return e==="edit"&&!n?{...r,propertiesFullColumn:Dy()}:r}function Kr(e){return{colorScopesOpen:e.colorScopesOpen,clipInspectorTab:e.clipInspectorTab,activeTab:e.activeTab,propertiesFullColumn:e.propertiesFullColumn}}function Gr(e,t){try{localStorage.setItem(xp(e),JSON.stringify(t))}catch{}}const kp=zy(),Zi=wp(kp);function Yc(e,t,r){if(!Number.isFinite(e))return t;const n=e===Zc&&t!==Zc?t:e;return Math.min(r.maxWidth,Math.max(r.minWidth,n))}function Xc(e,t){try{const r=localStorage.getItem(e);if(r!==null){const n=Number(r);return Number.isFinite(n)?n:t}}catch{}return t}const Ul=_e(e=>({activePanel:null,leftSidebarOpen:!0,rightSidebarOpen:!0,keyframeEditorShortcutScopeActive:!1,transcriptEditorShortcutScopeActive:!1,workspace:kp,activeTab:Zi.activeTab,clipInspectorTab:Zi.clipInspectorTab,sidebarWidth:Xc("editor:sidebarWidth",Uc.leftSidebarDefaultWidth),rightSidebarWidth:Xc("editor:rightSidebarWidth",Uc.rightSidebarDefaultWidth),timelineHeight:250,sourcePreviewMediaId:null,mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null,compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null,transcriptionDialogDepth:0,sourcePatchVideoEnabled:!0,sourcePatchAudioEnabled:!0,sourcePatchVideoTrackId:null,sourcePatchAudioTrackId:null,linkedSelectionEnabled:!0,colorScopesOpen:Zi.colorScopesOpen,mixerFloating:(()=>{try{return localStorage.getItem("editor:mixerFloating")==="true"}catch{return!1}})(),propertiesFullColumn:Zi.propertiesFullColumn,mediaFullColumn:(()=>{try{const t=localStorage.getItem("editor:mediaFullColumn");return t===null?!0:t==="true"}catch{return!0}})(),trackSizePreset:Oy(),setActivePanel:t=>e({activePanel:t}),setLeftSidebarOpen:t=>e({leftSidebarOpen:t}),setRightSidebarOpen:t=>e({rightSidebarOpen:t}),setKeyframeEditorShortcutScopeActive:t=>e({keyframeEditorShortcutScopeActive:t}),setTranscriptEditorShortcutScopeActive:t=>e({transcriptEditorShortcutScopeActive:t}),toggleLeftSidebar:()=>e(t=>({leftSidebarOpen:!t.leftSidebarOpen})),toggleRightSidebar:()=>e(t=>({rightSidebarOpen:!t.rightSidebarOpen})),setWorkspace:t=>e(r=>{if(r.workspace===t)return r;const n=be.getState();n.pause(),n.setPreviewFrame(null),Gr(r.workspace,Kr(r));try{localStorage.setItem(Sp,t)}catch{}return{workspace:t,...wp(t)}}),setActiveTab:t=>e(r=>{const n={...r,activeTab:t};return Gr(r.workspace,Kr(n)),{activeTab:t}}),setClipInspectorTab:t=>e(r=>{const n={...r,clipInspectorTab:t};return Gr(r.workspace,Kr(n)),{clipInspectorTab:t}}),setSidebarWidth:t=>{try{localStorage.setItem("editor:sidebarWidth",String(t))}catch{}e({sidebarWidth:t})},setRightSidebarWidth:t=>{try{localStorage.setItem("editor:rightSidebarWidth",String(t))}catch{}e({rightSidebarWidth:t})},syncSidebarLayout:t=>e(r=>({sidebarWidth:Yc(r.sidebarWidth,t.leftSidebarDefaultWidth,K0({leftSidebarMinWidth:t.leftSidebarMinWidth,leftSidebarMaxWidth:t.leftSidebarMaxWidth})),rightSidebarWidth:Yc(r.rightSidebarWidth,t.rightSidebarDefaultWidth,q0({rightSidebarMinWidth:t.rightSidebarMinWidth,rightSidebarMaxWidth:t.rightSidebarMaxWidth}))})),setTimelineHeight:t=>e({timelineHeight:t}),setSourcePreviewMediaId:t=>e({sourcePreviewMediaId:t,mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null,compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null}),setMediaSkimPreview:(t,r=null)=>e(n=>{const i=t?r:null;return n.mediaSkimPreviewMediaId===t&&n.mediaSkimPreviewFrame===i&&n.compoundClipSkimPreviewCompositionId===null&&n.compoundClipSkimPreviewFrame===null?n:{mediaSkimPreviewMediaId:t,mediaSkimPreviewFrame:i,compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null}}),clearMediaSkimPreview:()=>e(t=>t.mediaSkimPreviewMediaId===null&&t.mediaSkimPreviewFrame===null?t:{mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null}),setCompoundClipSkimPreview:(t,r=null)=>e(n=>{const i=t?r:null;return n.compoundClipSkimPreviewCompositionId===t&&n.compoundClipSkimPreviewFrame===i&&n.mediaSkimPreviewMediaId===null&&n.mediaSkimPreviewFrame===null?n:{compoundClipSkimPreviewCompositionId:t,compoundClipSkimPreviewFrame:i,mediaSkimPreviewMediaId:null,mediaSkimPreviewFrame:null}}),clearCompoundClipSkimPreview:()=>e(t=>t.compoundClipSkimPreviewCompositionId===null&&t.compoundClipSkimPreviewFrame===null?t:{compoundClipSkimPreviewCompositionId:null,compoundClipSkimPreviewFrame:null}),beginTranscriptionDialog:()=>e(t=>({transcriptionDialogDepth:t.transcriptionDialogDepth+1})),endTranscriptionDialog:()=>e(t=>({transcriptionDialogDepth:Math.max(0,t.transcriptionDialogDepth-1)})),setSourcePatchVideoEnabled:t=>e({sourcePatchVideoEnabled:t}),setSourcePatchAudioEnabled:t=>e({sourcePatchAudioEnabled:t}),setSourcePatchVideoTrackId:t=>e({sourcePatchVideoTrackId:t}),setSourcePatchAudioTrackId:t=>e({sourcePatchAudioTrackId:t}),toggleSourcePatchVideoEnabled:()=>e(t=>({sourcePatchVideoEnabled:!t.sourcePatchVideoEnabled})),toggleSourcePatchAudioEnabled:()=>e(t=>({sourcePatchAudioEnabled:!t.sourcePatchAudioEnabled})),setLinkedSelectionEnabled:t=>e({linkedSelectionEnabled:t}),toggleLinkedSelectionEnabled:()=>e(t=>({linkedSelectionEnabled:!t.linkedSelectionEnabled})),setColorScopesOpen:t=>e(r=>{const n={...r,colorScopesOpen:t};return Gr(r.workspace,Kr(n)),{colorScopesOpen:t}}),toggleColorScopesOpen:()=>e(t=>{const r=!t.colorScopesOpen,n={...t,colorScopesOpen:r};return Gr(t.workspace,Kr(n)),{colorScopesOpen:r}}),setMixerFloating:t=>{try{localStorage.setItem("editor:mixerFloating",String(t))}catch{}e({mixerFloating:t})},toggleMixerFloating:()=>e(t=>{const r=!t.mixerFloating;try{localStorage.setItem("editor:mixerFloating",String(r))}catch{}return{mixerFloating:r}}),togglePropertiesFullColumn:()=>e(t=>{const r=!t.propertiesFullColumn;try{localStorage.setItem("editor:propertiesFullColumn",String(r))}catch{}const n={...t,propertiesFullColumn:r};return Gr(t.workspace,Kr(n)),{propertiesFullColumn:r}}),toggleMediaFullColumn:()=>e(t=>{const r=!t.mediaFullColumn;try{localStorage.setItem("editor:mediaFullColumn",String(r))}catch{}return{mediaFullColumn:r}}),setTrackSizePreset:t=>{try{localStorage.setItem(Ip,t)}catch{}e({trackSizePreset:t})}}));let Ry=new Map;function Pp(){return G0[Ul.getState().trackSizePreset]}function Ly(e){return Ry.get(e)??Pp()}const W=_e()((e,t)=>({fps:30,scrollPosition:0,snapEnabled:!0,audioSkimmingEnabled:!0,isDirty:!1,isTimelineLoading:!0,setFps:r=>e({fps:r}),setScrollPosition:r=>e({scrollPosition:r}),setSnapEnabled:r=>e({snapEnabled:r}),toggleSnap:()=>e(r=>({snapEnabled:!r.snapEnabled})),setAudioSkimmingEnabled:r=>e({audioSkimmingEnabled:r}),toggleAudioSkimming:()=>e(r=>({audioSkimmingEnabled:!r.audioSkimmingEnabled})),setIsDirty:r=>e({isDirty:r}),markDirty:()=>{t().isDirty||e({isDirty:!0})},markClean:()=>e({isDirty:!1}),setTimelineLoading:r=>e({isTimelineLoading:r})}));function By(e,t){if(e.length!==t.length)return!1;for(let r=0;r<e.length;r+=1)if(e[r]!==t[r])return!1;return!0}function Uy(e){return e?{...e,draggedItemIdSet:e.draggedItemIdSet??new Set(e.draggedItemIds),draggedTrackIdSet:e.draggedTrackIdSet??new Set(e.draggedTrackIds??[])}:null}const yt=_e(e=>({selectedItemIds:[],selectedItemIdSet:new Set,selectedMarkerId:null,selectedTransitionId:null,selectedTrackId:null,selectedTrackIds:[],activeTrackId:null,selectionType:null,activeTool:"select",activeSnapTarget:null,activeLinkedDropTarget:null,dragState:null,editKeyframePanelOpen:!1,expandedKeyframeLanes:new Set,selectItems:t=>e(r=>{const n=t.length>0?"item":r.selectedTrackIds.length>0?"track":null;return By(r.selectedItemIds,t)&&r.selectedMarkerId===null&&r.selectedTransitionId===null&&r.selectionType===n?r:{selectedItemIds:t,selectedItemIdSet:new Set(t),expandedKeyframeLanes:r.editKeyframePanelOpen?t.length===1?new Set([t[0]]):new Set:r.expandedKeyframeLanes,selectedMarkerId:null,selectedTransitionId:null,selectionType:n}}),selectMarker:t=>e({selectedMarkerId:t,selectedTransitionId:null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectionType:t?"marker":null}),selectTransition:t=>e({selectedTransitionId:t,selectedMarkerId:null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectionType:t?"transition":null}),selectTrack:t=>e({selectedTrackId:t,activeTrackId:t,selectedTrackIds:t?[t]:[],selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:t?"track":null}),selectTracks:(t,r=!1)=>e(n=>{const i=r?Array.from(new Set([...n.selectedTrackIds,...t])):t;return{selectedTrackIds:i,activeTrackId:t[0]||null,selectedTrackId:t[0]||null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:i.length>0?"track":null}}),setActiveTrack:t=>e({activeTrackId:t,selectedTrackId:t,selectedTrackIds:t?[t]:[],selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:t?"track":null}),toggleTrackSelection:t=>e(r=>{const i=r.selectedTrackIds.includes(t)?r.selectedTrackIds.filter(o=>o!==t):[...r.selectedTrackIds,t];return{selectedTrackIds:i,activeTrackId:i[0]||null,selectedTrackId:i[0]||null,selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectionType:i.length>0?"track":null}}),clearSelection:()=>e({selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectedMarkerId:null,selectedTransitionId:null,selectedTrackId:null,selectedTrackIds:[],activeTrackId:null,selectionType:null}),clearItemSelection:()=>e(t=>({selectedItemIds:[],selectedItemIdSet:new Set,expandedKeyframeLanes:new Set,selectionType:t.selectedTrackIds.length>0?"track":null})),setDragState:t=>e(r=>({dragState:Uy(t),activeSnapTarget:t?r.activeSnapTarget:null,activeLinkedDropTarget:t?r.activeLinkedDropTarget:null})),setActiveSnapTarget:t=>e({activeSnapTarget:t}),setActiveLinkedDropTarget:t=>e({activeLinkedDropTarget:t}),setActiveTool:t=>e({activeTool:t}),toggleEditKeyframePanel:()=>e(t=>{const r=!t.editKeyframePanelOpen,n=t.selectedItemIds.length===1?t.selectedItemIds[0]:null;return{editKeyframePanelOpen:r,expandedKeyframeLanes:r&&n?new Set([n]):new Set}}),setEditKeyframePanelOpen:t=>e(r=>{if(r.editKeyframePanelOpen===t)return r;const n=r.selectedItemIds.length===1?r.selectedItemIds[0]:null;return{editKeyframePanelOpen:t,expandedKeyframeLanes:t&&n?new Set([n]):new Set}}),toggleKeyframeLanes:t=>e(r=>{const n=r.editKeyframePanelOpen&&r.expandedKeyframeLanes.has(t);return{selectedItemIds:[t],selectedItemIdSet:new Set([t]),selectedMarkerId:null,selectedTransitionId:null,selectionType:"item",editKeyframePanelOpen:!n,expandedKeyframeLanes:n?new Set:new Set([t])}}),setKeyframeLanesExpanded:(t,r)=>e(n=>r?{editKeyframePanelOpen:!0,expandedKeyframeLanes:new Set([t])}:n.expandedKeyframeLanes.has(t)?{editKeyframePanelOpen:!1,expandedKeyframeLanes:new Set}:n)})),Ny=10;function Qc(e,t,r){return Math.max(t,Math.min(r,e))}function Jc(e){return e==null||!Number.isFinite(e)?null:Math.round(e)}function Tp(e,t){const r=e.reduce((o,a)=>Math.max(o,a.from+a.durationInFrames),0),n=Number.isFinite(t)&&t>0?t:30,i=Math.max(1,Math.floor(Ny*n));return Math.max(r,i)}function $y(e){const t=Number.isFinite(e)?Math.floor(e):0;return Math.max(1,t)}function Ep(e){const t=Number.isFinite(e.maxFrame)?e.maxFrame:1,r=Math.max(1,Math.floor(t));let n=Jc(e.inPoint),i=Jc(e.outPoint);return n!==null&&(n=Qc(n,0,r)),i!==null&&(i=Qc(i,1,r)),n!==null&&i!==null&&n>=i?n>=r?{inPoint:Math.max(0,r-1),outPoint:r}:{inPoint:n,outPoint:Math.min(r,n+1)}:{inPoint:n,outPoint:i}}const Hy=-20,jy=20,qy=.3,Ky=10.3,Gy=["low-shelf","peaking","high-shelf","high-pass"],Wy=["low-shelf","peaking","high-shelf","notch"],Vy=["low-pass","low-shelf","peaking","high-shelf"],Zy=[6,12,18,24],Ko=30,Go=20,Wo=399,Cp=120,Mp=20,_p=22e3,Fp=400,Ap=20,Op=22e3,zp=1e3,Dp=.9,zr=1.1,Rp=1600,Lp=20,Bp=22e3,Ao=1.1,Up=2800,Np=20,$p=22e3,Vo=22e3,Zo=1400,Yo=22e3,Yy=1,Pe=.001,Xy=1.1,Qy=2.3,Jy=2.3,ev=1.1,Y=Object.freeze({outputGainDb:0,band1Enabled:!1,band1Type:"high-pass",band1FrequencyHz:Ko,band1GainDb:0,band1Q:Xy,band1SlopeDbPerOct:12,lowCutEnabled:!1,lowCutFrequencyHz:Ko,lowCutSlopeDbPerOct:12,lowEnabled:!0,lowType:"low-shelf",lowGainDb:0,lowFrequencyHz:Cp,lowQ:Qy,lowMidEnabled:!0,lowMidType:"peaking",lowMidGainDb:0,lowMidFrequencyHz:Fp,lowMidQ:zr,midGainDb:0,highMidEnabled:!0,highMidType:"peaking",highMidGainDb:0,highMidFrequencyHz:Rp,highMidQ:Ao,highEnabled:!0,highType:"high-shelf",highGainDb:0,highFrequencyHz:Up,highQ:Jy,band6Enabled:!1,band6Type:"low-pass",band6FrequencyHz:Vo,band6GainDb:0,band6Q:ev,band6SlopeDbPerOct:12,highCutEnabled:!1,highCutFrequencyHz:Vo,highCutSlopeDbPerOct:12});function Pa(e,t){return Math.max(20,Math.min(e,t*.45))}function It(e){return Number.isFinite(e)?Math.max(Hy,Math.min(jy,e)):0}function Lt(e,t=zr){return Number.isFinite(e)?Math.max(qy,Math.min(Ky,e)):t}function tv(e,t=Y.band1Type){return Gy.includes(e)?e:t}function Yi(e,t=Y.lowMidType){return Wy.includes(e)?e:t}function rv(e,t=Y.band6Type){return Vy.includes(e)?e:t}function Dt(e,t,r,n){return Number.isFinite(e)?Math.max(t,Math.min(r,e)):n}function Gn(e,t=12){return Zy.includes(e)?e:t}function Yt(e,t){return e===void 0?t:e}function nv(e){return e==null?!1:"audioEqEnabled"in e&&e.audioEqEnabled===!1||"enabled"in e&&e.enabled===!1}function G(e,t,r,n){return e?.[r]??t?.[n]}function oD(e){return{enabled:e?.audioEqEnabled,outputGainDb:e?.audioEqOutputGainDb,band1Enabled:e?.audioEqBand1Enabled,band1Type:e?.audioEqBand1Type,band1FrequencyHz:e?.audioEqBand1FrequencyHz,band1GainDb:e?.audioEqBand1GainDb,band1Q:e?.audioEqBand1Q,band1SlopeDbPerOct:e?.audioEqBand1SlopeDbPerOct,lowCutEnabled:e?.audioEqLowCutEnabled,lowCutFrequencyHz:e?.audioEqLowCutFrequencyHz,lowCutSlopeDbPerOct:e?.audioEqLowCutSlopeDbPerOct,lowEnabled:e?.audioEqLowEnabled,lowType:e?.audioEqLowType,lowGainDb:e?.audioEqLowGainDb,lowFrequencyHz:e?.audioEqLowFrequencyHz,lowQ:e?.audioEqLowQ,lowMidEnabled:e?.audioEqLowMidEnabled,lowMidType:e?.audioEqLowMidType,lowMidGainDb:e?.audioEqLowMidGainDb,lowMidFrequencyHz:e?.audioEqLowMidFrequencyHz,lowMidQ:e?.audioEqLowMidQ,midGainDb:e?.audioEqMidGainDb,highMidEnabled:e?.audioEqHighMidEnabled,highMidType:e?.audioEqHighMidType,highMidGainDb:e?.audioEqHighMidGainDb,highMidFrequencyHz:e?.audioEqHighMidFrequencyHz,highMidQ:e?.audioEqHighMidQ,highEnabled:e?.audioEqHighEnabled,highType:e?.audioEqHighType,highGainDb:e?.audioEqHighGainDb,highFrequencyHz:e?.audioEqHighFrequencyHz,highQ:e?.audioEqHighQ,band6Enabled:e?.audioEqBand6Enabled,band6Type:e?.audioEqBand6Type,band6FrequencyHz:e?.audioEqBand6FrequencyHz,band6GainDb:e?.audioEqBand6GainDb,band6Q:e?.audioEqBand6Q,band6SlopeDbPerOct:e?.audioEqBand6SlopeDbPerOct,highCutEnabled:e?.audioEqHighCutEnabled,highCutFrequencyHz:e?.audioEqHighCutFrequencyHz,highCutSlopeDbPerOct:e?.audioEqHighCutSlopeDbPerOct}}function Nl(e){const t=e,r=e,n=Yt(G(t,r,"lowCutEnabled","audioEqLowCutEnabled"),Y.lowCutEnabled),i=Dt(Number(G(t,r,"lowCutFrequencyHz","audioEqLowCutFrequencyHz")),Go,Wo,Y.lowCutFrequencyHz),o=Gn(Number(G(t,r,"lowCutSlopeDbPerOct","audioEqLowCutSlopeDbPerOct")),Y.lowCutSlopeDbPerOct),a=G(t,r,"band1Enabled","audioEqBand1Enabled"),s=G(t,r,"band1Type","audioEqBand1Type"),l=G(t,r,"band1FrequencyHz","audioEqBand1FrequencyHz"),c=G(t,r,"band1GainDb","audioEqBand1GainDb"),u=G(t,r,"band1Q","audioEqBand1Q"),d=G(t,r,"band1SlopeDbPerOct","audioEqBand1SlopeDbPerOct"),f=[a,s,l,c,u,d].some(je=>je!==void 0),p=f?tv(s,Y.band1Type):"high-pass",m=Yt(f?a:n,Y.band1Enabled),h=Dt(Number(f?l:i),Go,Wo,Y.band1FrequencyHz),g=It(Number(c)),v=Lt(Number(u),Y.band1Q),x=Gn(Number(f?d:o),Y.band1SlopeDbPerOct),b=Yt(G(t,r,"highCutEnabled","audioEqHighCutEnabled"),Y.highCutEnabled),w=Dt(Number(G(t,r,"highCutFrequencyHz","audioEqHighCutFrequencyHz")),Zo,Yo,Y.highCutFrequencyHz),E=Gn(Number(G(t,r,"highCutSlopeDbPerOct","audioEqHighCutSlopeDbPerOct")),Y.highCutSlopeDbPerOct),k=G(t,r,"band6Enabled","audioEqBand6Enabled"),T=G(t,r,"band6Type","audioEqBand6Type"),O=G(t,r,"band6FrequencyHz","audioEqBand6FrequencyHz"),C=G(t,r,"band6GainDb","audioEqBand6GainDb"),R=G(t,r,"band6Q","audioEqBand6Q"),M=G(t,r,"band6SlopeDbPerOct","audioEqBand6SlopeDbPerOct"),D=[k,T,O,C,R,M].some(je=>je!==void 0),L=D?rv(T,Y.band6Type):"low-pass",H=Yt(D?k:b,Y.band6Enabled),J=Dt(Number(D?O:w),Zo,Yo,Y.band6FrequencyHz),fe=It(Number(C)),se=Lt(Number(R),Y.band6Q),At=Gn(Number(D?M:E),Y.band6SlopeDbPerOct);return{outputGainDb:It(Number(G(t,r,"outputGainDb","audioEqOutputGainDb"))),band1Enabled:m,band1Type:p,band1FrequencyHz:h,band1GainDb:g,band1Q:v,band1SlopeDbPerOct:x,lowCutEnabled:m&&p==="high-pass",lowCutFrequencyHz:h,lowCutSlopeDbPerOct:x,lowEnabled:Yt(G(t,r,"lowEnabled","audioEqLowEnabled"),Y.lowEnabled),lowType:Yi(G(t,r,"lowType","audioEqLowType"),Y.lowType),lowGainDb:It(Number(G(t,r,"lowGainDb","audioEqLowGainDb"))),lowFrequencyHz:Dt(Number(G(t,r,"lowFrequencyHz","audioEqLowFrequencyHz")),Mp,_p,Y.lowFrequencyHz),lowQ:Lt(Number(G(t,r,"lowQ","audioEqLowQ")),Y.lowQ),lowMidEnabled:Yt(G(t,r,"lowMidEnabled","audioEqLowMidEnabled"),Y.lowMidEnabled),lowMidType:Yi(G(t,r,"lowMidType","audioEqLowMidType"),Y.lowMidType),lowMidGainDb:It(Number(G(t,r,"lowMidGainDb","audioEqLowMidGainDb"))),lowMidFrequencyHz:Dt(Number(G(t,r,"lowMidFrequencyHz","audioEqLowMidFrequencyHz")),Ap,Op,Y.lowMidFrequencyHz),lowMidQ:Lt(Number(G(t,r,"lowMidQ","audioEqLowMidQ")),Y.lowMidQ),midGainDb:It(Number(G(t,r,"midGainDb","audioEqMidGainDb"))),highMidEnabled:Yt(G(t,r,"highMidEnabled","audioEqHighMidEnabled"),Y.highMidEnabled),highMidType:Yi(G(t,r,"highMidType","audioEqHighMidType"),Y.highMidType),highMidGainDb:It(Number(G(t,r,"highMidGainDb","audioEqHighMidGainDb"))),highMidFrequencyHz:Dt(Number(G(t,r,"highMidFrequencyHz","audioEqHighMidFrequencyHz")),Lp,Bp,Y.highMidFrequencyHz),highMidQ:Lt(Number(G(t,r,"highMidQ","audioEqHighMidQ")),Y.highMidQ),highEnabled:Yt(G(t,r,"highEnabled","audioEqHighEnabled"),Y.highEnabled),highType:Yi(G(t,r,"highType","audioEqHighType"),Y.highType),highGainDb:It(Number(G(t,r,"highGainDb","audioEqHighGainDb"))),highFrequencyHz:Dt(Number(G(t,r,"highFrequencyHz","audioEqHighFrequencyHz")),Np,$p,Y.highFrequencyHz),highQ:Lt(Number(G(t,r,"highQ","audioEqHighQ")),Y.highQ),band6Enabled:H,band6Type:L,band6FrequencyHz:J,band6GainDb:fe,band6Q:se,band6SlopeDbPerOct:At,highCutEnabled:H&&L==="low-pass",highCutFrequencyHz:J,highCutSlopeDbPerOct:At}}function iv(e,t){return t==null||nv(t)?[...e??[]]:[...e??[],Nl(t)]}function aD(e,...t){let r=[...e??[]];for(const n of t)n!=null&&(r=iv(r,n));return r}function ov(e){if(e!=null)return{...e&&"enabled"in e&&e.enabled!==void 0?{enabled:!!e.enabled}:{},...Nl(e)}}function Vs(e){return e?Math.abs(e.outputGainDb??0)>Pe||!!e.band1Enabled&&e.band1Type==="high-pass"||!!e.band1Enabled&&e.band1Type!=="high-pass"&&Math.abs(e.band1GainDb??0)>Pe||!!e.lowEnabled&&e.lowType==="notch"||!!e.lowEnabled&&Math.abs(e.lowGainDb??0)>Pe||!!e.lowMidEnabled&&e.lowMidType==="notch"||!!e.lowMidEnabled&&Math.abs(e.lowMidGainDb??0)>Pe||Math.abs(e.midGainDb??0)>Pe||!!e.highMidEnabled&&e.highMidType==="notch"||!!e.highMidEnabled&&Math.abs(e.highMidGainDb??0)>Pe||!!e.highEnabled&&e.highType==="notch"||!!e.highEnabled&&Math.abs(e.highGainDb??0)>Pe||!!e.band6Enabled&&e.band6Type==="low-pass"||!!e.band6Enabled&&e.band6Type!=="low-pass"&&Math.abs(e.band6GainDb??0)>Pe:!1}function sD(e,t){if(e===t)return!0;if(!e||!t)return!e&&!t;if(e.length!==t.length)return!1;for(let r=0;r<e.length;r++){const n=e[r],i=t[r];if(!n||!i||n.outputGainDb!==i.outputGainDb||n.band1Enabled!==i.band1Enabled||n.band1Type!==i.band1Type||n.band1FrequencyHz!==i.band1FrequencyHz||n.band1GainDb!==i.band1GainDb||n.band1Q!==i.band1Q||n.band1SlopeDbPerOct!==i.band1SlopeDbPerOct||n.lowCutEnabled!==i.lowCutEnabled||n.lowCutFrequencyHz!==i.lowCutFrequencyHz||n.lowCutSlopeDbPerOct!==i.lowCutSlopeDbPerOct||n.lowEnabled!==i.lowEnabled||n.lowType!==i.lowType||n.lowGainDb!==i.lowGainDb||n.lowFrequencyHz!==i.lowFrequencyHz||n.lowQ!==i.lowQ||n.lowMidEnabled!==i.lowMidEnabled||n.lowMidType!==i.lowMidType||n.lowMidGainDb!==i.lowMidGainDb||n.lowMidFrequencyHz!==i.lowMidFrequencyHz||n.lowMidQ!==i.lowMidQ||n.midGainDb!==i.midGainDb||n.highMidEnabled!==i.highMidEnabled||n.highMidType!==i.highMidType||n.highMidGainDb!==i.highMidGainDb||n.highMidFrequencyHz!==i.highMidFrequencyHz||n.highMidQ!==i.highMidQ||n.highEnabled!==i.highEnabled||n.highType!==i.highType||n.highGainDb!==i.highGainDb||n.highFrequencyHz!==i.highFrequencyHz||n.highQ!==i.highQ||n.band6Enabled!==i.band6Enabled||n.band6Type!==i.band6Type||n.band6FrequencyHz!==i.band6FrequencyHz||n.band6GainDb!==i.band6GainDb||n.band6Q!==i.band6Q||n.band6SlopeDbPerOct!==i.band6SlopeDbPerOct||n.highCutEnabled!==i.highCutEnabled||n.highCutFrequencyHz!==i.highCutFrequencyHz||n.highCutSlopeDbPerOct!==i.highCutSlopeDbPerOct)return!1}return!0}function Be(e,t,r){return{id:e,label:t,settings:Object.freeze(Nl(r))}}Object.freeze([Be("flat","Flat",{}),Be("voice-clarity","Voice Clarity",{lowCutEnabled:!0,lowCutFrequencyHz:80,lowCutSlopeDbPerOct:12,lowGainDb:-1.5,lowFrequencyHz:120,lowMidGainDb:-2.5,lowMidFrequencyHz:320,lowMidQ:1.2,highMidGainDb:4.5,highMidFrequencyHz:2800,highMidQ:1.1,highGainDb:2,highFrequencyHz:7200}),Be("podcast","Podcast",{lowCutEnabled:!0,lowCutFrequencyHz:70,lowCutSlopeDbPerOct:12,lowGainDb:-1,lowFrequencyHz:120,lowMidGainDb:-1.5,lowMidFrequencyHz:250,lowMidQ:1,highMidGainDb:5.5,highMidFrequencyHz:3e3,highMidQ:1.25,highGainDb:2.5,highFrequencyHz:9e3}),Be("warmth","Warmth",{lowGainDb:4,lowFrequencyHz:110,lowMidGainDb:2,lowMidFrequencyHz:280,lowMidQ:.9,highMidGainDb:-1.5,highMidFrequencyHz:2600,highMidQ:1,highGainDb:-2.5,highFrequencyHz:6500}),Be("bass-boost","Bass Boost",{lowGainDb:7,lowFrequencyHz:90,lowMidGainDb:2.5,lowMidFrequencyHz:180,lowMidQ:.85,highMidGainDb:-1,highMidFrequencyHz:2500,highMidQ:.9,highGainDb:.5,highFrequencyHz:7e3}),Be("de-mud","De-Mud",{lowCutEnabled:!0,lowCutFrequencyHz:55,lowCutSlopeDbPerOct:12,lowGainDb:-1,lowFrequencyHz:120,lowMidGainDb:-5,lowMidFrequencyHz:300,lowMidQ:1.4,highMidGainDb:2,highMidFrequencyHz:2600,highMidQ:1,highGainDb:1,highFrequencyHz:7e3}),Be("smile","Smile",{lowGainDb:3.5,lowFrequencyHz:95,lowMidGainDb:-2,lowMidFrequencyHz:420,lowMidQ:1.2,highMidGainDb:2.5,highMidFrequencyHz:2500,highMidQ:.9,highGainDb:4.5,highFrequencyHz:9500}),Be("sparkle","Sparkle",{lowCutEnabled:!0,lowCutFrequencyHz:60,lowCutSlopeDbPerOct:6,lowGainDb:-2,lowFrequencyHz:100,lowMidGainDb:-1.5,lowMidFrequencyHz:350,lowMidQ:1.1,highMidGainDb:4,highMidFrequencyHz:3400,highMidQ:1,highGainDb:6,highFrequencyHz:9500}),Be("air","Air",{lowCutEnabled:!0,lowCutFrequencyHz:75,lowCutSlopeDbPerOct:12,lowGainDb:-3,lowFrequencyHz:110,lowMidGainDb:-1.5,lowMidFrequencyHz:350,lowMidQ:.95,highMidGainDb:2,highMidFrequencyHz:2800,highMidQ:.85,highGainDb:7,highFrequencyHz:11e3}),Be("soften","Soften",{lowGainDb:1.5,lowFrequencyHz:120,lowMidGainDb:1,lowMidFrequencyHz:280,lowMidQ:1,highMidGainDb:-2.5,highMidFrequencyHz:3200,highMidQ:1.05,highGainDb:-4.5,highFrequencyHz:8e3,highCutEnabled:!0,highCutFrequencyHz:14e3,highCutSlopeDbPerOct:12}),Be("radio","Radio",{lowCutEnabled:!0,lowCutFrequencyHz:220,lowCutSlopeDbPerOct:18,lowGainDb:-6,lowFrequencyHz:250,lowMidGainDb:-3,lowMidFrequencyHz:500,lowMidQ:1.3,highMidGainDb:2,highMidFrequencyHz:1800,highMidQ:1,highGainDb:-6,highFrequencyHz:3500,highCutEnabled:!0,highCutFrequencyHz:4200,highCutSlopeDbPerOct:18}),Be("telephone","Telephone",{outputGainDb:5,lowCutEnabled:!0,lowCutFrequencyHz:320,lowCutSlopeDbPerOct:24,lowGainDb:-8,lowFrequencyHz:240,lowMidGainDb:-3.5,lowMidFrequencyHz:520,lowMidQ:1.15,highMidGainDb:4.5,highMidFrequencyHz:1700,highMidQ:1.05,highGainDb:-9,highFrequencyHz:3600,highCutEnabled:!0,highCutFrequencyHz:3200,highCutSlopeDbPerOct:24}),Be("dialog-lift","Dialog Lift",{lowCutEnabled:!0,lowCutFrequencyHz:75,lowCutSlopeDbPerOct:12,lowGainDb:-2,lowFrequencyHz:120,lowMidGainDb:-2,lowMidFrequencyHz:280,lowMidQ:1.1,highMidGainDb:3.5,highMidFrequencyHz:2400,highMidQ:1.2,highGainDb:1.5,highFrequencyHz:7e3}),Be("rumble-cut","Rumble Cut",{lowCutEnabled:!0,lowCutFrequencyHz:100,lowCutSlopeDbPerOct:18,lowGainDb:-3,lowFrequencyHz:110,lowMidGainDb:-1,lowMidFrequencyHz:250,lowMidQ:1}),Be("brighten","Brighten",{lowCutEnabled:!0,lowCutFrequencyHz:50,lowCutSlopeDbPerOct:6,lowGainDb:-1,lowFrequencyHz:120,lowMidGainDb:-1,lowMidFrequencyHz:350,lowMidQ:1,highMidGainDb:2.5,highMidFrequencyHz:3e3,highMidQ:1,highGainDb:5,highFrequencyHz:8500})]);function $l(e,t,r,n){const i=Pa(e,r),o=Lt(n,zr),a=Math.pow(10,t/40),s=2*Math.PI*(i/r),l=Math.cos(s),c=Math.sin(s)/(2*o),u=1+c*a,d=-2*l,f=1-c*a,p=1+c/a,m=-2*l,h=1-c/a;return{b0:u/p,b1:d/p,b2:f/p,a1:m/p,a2:h/p}}function av(e,t,r){const n=Pa(e,t),i=Lt(r,zr),o=2*Math.PI*(n/t),a=Math.cos(o),s=Math.sin(o)/(2*i),l=1,c=-2*a,u=1,d=1+s,f=-2*a,p=1-s;return{b0:l/d,b1:c/d,b2:u/d,a1:f/d,a2:p/d}}function sv(e,t,r,n){const i=Pa(t,n),o=Math.pow(10,r/40),a=2*Math.PI*(i/n),s=Math.cos(a),l=Math.sin(a),c=Math.sqrt(o),u=l/2*Math.sqrt((o+1/o)*(1/Yy-1)+2);let d,f,p,m,h,g;return e==="lowshelf"?(d=o*(o+1-(o-1)*s+2*c*u),f=2*o*(o-1-(o+1)*s),p=o*(o+1-(o-1)*s-2*c*u),m=o+1+(o-1)*s+2*c*u,h=-2*(o-1+(o+1)*s),g=o+1+(o-1)*s-2*c*u):(d=o*(o+1+(o-1)*s+2*c*u),f=-2*o*(o-1+(o+1)*s),p=o*(o+1+(o-1)*s-2*c*u),m=o+1-(o-1)*s+2*c*u,h=2*(o-1-(o+1)*s),g=o+1-(o-1)*s-2*c*u),{b0:d/m,b1:f/m,b2:p/m,a1:h/m,a2:g/m}}function Hp(e){return e==="high-pass"}function jp(e){return e==="low-pass"}function qp(e){switch(e){case"low-shelf":return"lowshelf";case"peaking":return"peaking";case"high-shelf":return"highshelf";default:return null}}function Kp(e){switch(e){case"low-shelf":return"lowshelf";case"peaking":return"peaking";case"high-shelf":return"highshelf";default:return null}}function nr(e){switch(e){case"low-shelf":return"lowshelf";case"high-shelf":return"highshelf";case"notch":return"notch";default:return"peaking"}}function rt(e,t,r,n,i){switch(e){case"lowshelf":case"highshelf":return sv(e,t,r,n);case"notch":return av(t,n,i);default:return $l(t,r,n,i)}}function Gp(e,t,r){const n=Pa(t,r),i=Math.tan(Math.PI*(n/r)),o=1/(1+i);return e==="highpass"?{b0:o,b1:-o,a1:(i-1)*o}:{b0:i*o,b1:i*o,a1:(i-1)*o}}function Wp(e){return Math.max(1,Math.round(e/6))}function br(e,t){const r=new Float32Array(e.length);let n=0,i=0,o=0,a=0;for(let s=0;s<e.length;s++){const l=e[s]??0,c=t.b0*l+t.b1*n+t.b2*i-t.a1*o-t.a2*a;r[s]=c,i=n,n=l,a=o,o=c}return r}function lv(e,t){const r=new Float32Array(e.length);let n=0,i=0;for(let o=0;o<e.length;o++){const a=e[o]??0,s=t.b0*a+t.b1*n-t.a1*i;r[o]=s,n=a,i=s}return r}function cv(e,t){if(Math.abs(t-1)<=Pe)return e;const r=new Float32Array(e.length);for(let n=0;n<e.length;n++)r[n]=(e[n]??0)*t;return r}function eu(e,t,r,n,i){const o=Gp(t,r,i);let a=e;for(let s=0;s<Wp(n);s++)a=lv(a,o);return a}function uv(e,t,r){let n=e;if(r.band1Enabled&&Hp(r.band1Type))n=eu(n,"highpass",r.band1FrequencyHz,r.band1SlopeDbPerOct,t);else if(r.band1Enabled){const i=qp(r.band1Type);i&&(n=br(n,rt(i,r.band1FrequencyHz,r.band1GainDb,t,r.band1Q)))}if(r.lowEnabled&&(r.lowType==="notch"||Math.abs(r.lowGainDb)>Pe)&&(n=br(n,rt(nr(r.lowType),r.lowFrequencyHz,r.lowGainDb,t,r.lowQ))),r.lowMidEnabled&&(r.lowMidType==="notch"||Math.abs(r.lowMidGainDb)>Pe)&&(n=br(n,rt(nr(r.lowMidType),r.lowMidFrequencyHz,r.lowMidGainDb,t,r.lowMidQ))),Math.abs(r.midGainDb)>Pe&&(n=br(n,$l(zp,r.midGainDb,t,Dp))),r.highMidEnabled&&(r.highMidType==="notch"||Math.abs(r.highMidGainDb)>Pe)&&(n=br(n,rt(nr(r.highMidType),r.highMidFrequencyHz,r.highMidGainDb,t,r.highMidQ))),r.highEnabled&&(r.highType==="notch"||Math.abs(r.highGainDb)>Pe)&&(n=br(n,rt(nr(r.highType),r.highFrequencyHz,r.highGainDb,t,r.highQ))),r.band6Enabled&&jp(r.band6Type))n=eu(n,"lowpass",r.band6FrequencyHz,r.band6SlopeDbPerOct,t);else if(r.band6Enabled){const i=Kp(r.band6Type);i&&(n=br(n,rt(i,r.band6FrequencyHz,r.band6GainDb,t,r.band6Q)))}return Math.abs(r.outputGainDb)>Pe&&(n=cv(n,Math.pow(10,r.outputGainDb/20))),n}function lD(e,t,r){return!r||r.length===0||!r.some(Vs)?e:e.map(n=>{let i=n;for(const o of r)Vs(o)&&(i=uv(i,t,o));return i})}class dv{constructor(t){this.coefficients=t}coefficients;x1=0;x2=0;y1=0;y2=0;processInto(t,r){const{b0:n,b1:i,b2:o,a1:a,a2:s}=this.coefficients;let{x1:l,x2:c,y1:u,y2:d}=this;for(let f=0;f<t.length;f++){const p=t[f],m=n*p+i*l+o*c-a*u-s*d;r[f]=m,c=l,l=p,d=u,u=m}this.x1=l,this.x2=c,this.y1=u,this.y2=d}}class fv{constructor(t){this.coefficients=t}coefficients;x1=0;y1=0;processInto(t,r){const{b0:n,b1:i,a1:o}=this.coefficients;let{x1:a,y1:s}=this;for(let l=0;l<t.length;l++){const c=t[l],u=n*c+i*a-o*s;r[l]=u,a=c,s=u}this.x1=a,this.y1=s}}class pv{filters=[];outputGain;constructor(t,r){const n=(o,a,s)=>{const l=Gp(o,a,t);for(let c=0;c<Wp(s);c++)this.filters.push(new fv(l))},i=o=>{this.filters.push(new dv(o))};if(r.band1Enabled&&Hp(r.band1Type))n("highpass",r.band1FrequencyHz,r.band1SlopeDbPerOct);else if(r.band1Enabled){const o=qp(r.band1Type);o&&i(rt(o,r.band1FrequencyHz,r.band1GainDb,t,r.band1Q))}if(r.lowEnabled&&(r.lowType==="notch"||Math.abs(r.lowGainDb)>Pe)&&i(rt(nr(r.lowType),r.lowFrequencyHz,r.lowGainDb,t,r.lowQ)),r.lowMidEnabled&&(r.lowMidType==="notch"||Math.abs(r.lowMidGainDb)>Pe)&&i(rt(nr(r.lowMidType),r.lowMidFrequencyHz,r.lowMidGainDb,t,r.lowMidQ)),Math.abs(r.midGainDb)>Pe&&i($l(zp,r.midGainDb,t,Dp)),r.highMidEnabled&&(r.highMidType==="notch"||Math.abs(r.highMidGainDb)>Pe)&&i(rt(nr(r.highMidType),r.highMidFrequencyHz,r.highMidGainDb,t,r.highMidQ)),r.highEnabled&&(r.highType==="notch"||Math.abs(r.highGainDb)>Pe)&&i(rt(nr(r.highType),r.highFrequencyHz,r.highGainDb,t,r.highQ)),r.band6Enabled&&jp(r.band6Type))n("lowpass",r.band6FrequencyHz,r.band6SlopeDbPerOct);else if(r.band6Enabled){const o=Kp(r.band6Type);o&&i(rt(o,r.band6FrequencyHz,r.band6GainDb,t,r.band6Q))}this.outputGain=Math.abs(r.outputGainDb)>Pe?Math.pow(10,r.outputGainDb/20):1}}class cD{chains;scratchA=new Float32Array(0);scratchB=new Float32Array(0);constructor(t,r,n){const i=(n??[]).filter(Vs);this.chains=[];for(let o=0;o<t;o++)this.chains.push(i.map(a=>new pv(r,a)))}get hasActiveStages(){return(this.chains[0]?.length??0)>0}processChunk(t,r){const n=this.chains[r]??[];if(n.length===0)return t;this.scratchA.length<t.length&&(this.scratchA=new Float32Array(t.length),this.scratchB=new Float32Array(t.length));let i=t,o=this.scratchA;const a=s=>s===this.scratchA?this.scratchB:this.scratchA;for(const s of n){for(const l of s.filters){const c=i===t?o:a(i),u=i.length===t.length?i:i.subarray(0,t.length);l.processInto(u,c),i!==t&&(o=i),i=c}if(s.outputGain!==1){const l=a(i),c=s.outputGain;for(let u=0;u<t.length;u++)l[u]=i[u]*c;i!==t&&(o=i),i=l}}return i}}function Tt(){return import("./media-library-service-Bm8iuyQ_.js")}var Qe=Uint8Array,en=Uint16Array,mv=Int32Array,Vp=new Qe([0,0,0,0,0,0,0,0,1,1,1,1,2,2,2,2,3,3,3,3,4,4,4,4,5,5,5,5,0,0,0,0]),Zp=new Qe([0,0,0,0,1,1,2,2,3,3,4,4,5,5,6,6,7,7,8,8,9,9,10,10,11,11,12,12,13,13,0,0]),hv=new Qe([16,17,18,0,8,7,9,6,10,5,11,4,12,3,13,2,14,1,15]),Yp=function(e,t){for(var r=new en(31),n=0;n<31;++n)r[n]=t+=1<<e[n-1];for(var i=new mv(r[30]),n=1;n<30;++n)for(var o=r[n];o<r[n+1];++o)i[o]=o-r[n]<<5|n;return{b:r,r:i}},Xp=Yp(Vp,2),Qp=Xp.b,gv=Xp.r;Qp[28]=258,gv[258]=28;var yv=Yp(Zp,0),vv=yv.b,Zs=new en(32768);for(var ge=0;ge<32768;++ge){var Xt=(ge&43690)>>1|(ge&21845)<<1;Xt=(Xt&52428)>>2|(Xt&13107)<<2,Xt=(Xt&61680)>>4|(Xt&3855)<<4,Zs[ge]=((Xt&65280)>>8|(Xt&255)<<8)>>1}var si=(function(e,t,r){for(var n=e.length,i=0,o=new en(t);i<n;++i)e[i]&&++o[e[i]-1];var a=new en(t);for(i=1;i<t;++i)a[i]=a[i-1]+o[i-1]<<1;var s;if(r){s=new en(1<<t);var l=15-t;for(i=0;i<n;++i)if(e[i])for(var c=i<<4|e[i],u=t-e[i],d=a[e[i]-1]++<<u,f=d|(1<<u)-1;d<=f;++d)s[Zs[d]>>l]=c}else for(s=new en(n),i=0;i<n;++i)e[i]&&(s[i]=Zs[a[e[i]-1]++]>>15-e[i]);return s}),Ai=new Qe(288);for(var ge=0;ge<144;++ge)Ai[ge]=8;for(var ge=144;ge<256;++ge)Ai[ge]=9;for(var ge=256;ge<280;++ge)Ai[ge]=7;for(var ge=280;ge<288;++ge)Ai[ge]=8;var Jp=new Qe(32);for(var ge=0;ge<32;++ge)Jp[ge]=5;var bv=si(Ai,9,1),Sv=si(Jp,5,1),ja=function(e){for(var t=e[0],r=1;r<e.length;++r)e[r]>t&&(t=e[r]);return t},ut=function(e,t,r){var n=t/8|0;return(e[n]|e[n+1]<<8)>>(t&7)&r},qa=function(e,t){var r=t/8|0;return(e[r]|e[r+1]<<8|e[r+2]<<16)>>(t&7)},Iv=function(e){return(e+7)/8|0},Hl=function(e,t,r){return(t==null||t<0)&&(t=0),(r==null||r>e.length)&&(r=e.length),new Qe(e.subarray(t,r))},xv=["unexpected EOF","invalid block type","invalid length/literal","invalid distance","stream finished","no stream handler",,"no callback","invalid UTF-8 data","extra field too long","date not in range 1980-2099","filename too long","stream finishing","invalid zip data"],tt=function(e,t,r){var n=new Error(t||xv[e]);if(n.code=e,Error.captureStackTrace&&Error.captureStackTrace(n,tt),!r)throw n;return n},wv=function(e,t,r,n){var i=e.length,o=n?n.length:0;if(!i||t.f&&!t.l)return r||new Qe(0);var a=!r,s=a||t.i!=2,l=t.i;a&&(r=new Qe(i*3));var c=function(Rc){var Lc=r.length;if(Rc>Lc){var Bc=new Qe(Math.max(Lc*2,Rc));Bc.set(r),r=Bc}},u=t.f||0,d=t.p||0,f=t.b||0,p=t.l,m=t.d,h=t.m,g=t.n,v=i*8;do{if(!p){u=ut(e,d,1);var x=ut(e,d+1,3);if(d+=3,x)if(x==1)p=bv,m=Sv,h=9,g=5;else if(x==2){var k=ut(e,d,31)+257,T=ut(e,d+10,15)+4,O=k+ut(e,d+5,31)+1;d+=14;for(var C=new Qe(O),R=new Qe(19),M=0;M<T;++M)R[hv[M]]=ut(e,d+M*3,7);d+=T*3;for(var D=ja(R),L=(1<<D)-1,H=si(R,D,1),M=0;M<O;){var J=H[ut(e,d,L)];d+=J&15;var b=J>>4;if(b<16)C[M++]=b;else{var fe=0,se=0;for(b==16?(se=3+ut(e,d,3),d+=2,fe=C[M-1]):b==17?(se=3+ut(e,d,7),d+=3):b==18&&(se=11+ut(e,d,127),d+=7);se--;)C[M++]=fe}}var At=C.subarray(0,k),je=C.subarray(k);h=ja(At),g=ja(je),p=si(At,h,1),m=si(je,g,1)}else tt(1);else{var b=Iv(d)+4,w=e[b-4]|e[b-3]<<8,E=b+w;if(E>i){l&&tt(0);break}s&&c(f+w),r.set(e.subarray(b,E),f),t.b=f+=w,t.p=d=E*8,t.f=u;continue}if(d>v){l&&tt(0);break}}s&&c(f+131072);for(var z=(1<<h)-1,j=(1<<g)-1,le=d;;le=d){var fe=p[qa(e,d)&z],ce=fe>>4;if(d+=fe&15,d>v){l&&tt(0);break}if(fe||tt(2),ce<256)r[f++]=ce;else if(ce==256){le=d,p=null;break}else{var Fe=ce-254;if(ce>264){var M=ce-257,qe=Vp[M];Fe=ut(e,d,(1<<qe)-1)+Qp[M],d+=qe}var vr=m[qa(e,d)&j],ct=vr>>4;vr||tt(3),d+=vr&15;var je=vv[ct];if(ct>3){var qe=Zp[ct];je+=qa(e,d)&(1<<qe)-1,d+=qe}if(d>v){l&&tt(0);break}s&&c(f+131072);var zn=f+Fe;if(f<je){var Dc=o-je,j0=Math.min(je,zn);for(Dc+f<0&&tt(3);f<j0;++f)r[f]=n[Dc+f]}for(;f<zn;++f)r[f]=r[f-je]}}t.l=p,t.p=le,t.b=f,t.f=u,p&&(u=1,t.m=h,t.d=m,t.n=g)}while(!u);return f!=r.length&&a?Hl(r,0,f):r.subarray(0,f)},kv=new Qe(0),Pt=function(e,t){return e[t]|e[t+1]<<8},ht=function(e,t){return(e[t]|e[t+1]<<8|e[t+2]<<16|e[t+3]<<24)>>>0},Ka=function(e,t){return ht(e,t)+ht(e,t+4)*4294967296};function Pv(e,t){return wv(e,{i:2},t&&t.out,t&&t.dictionary)}var Ys=typeof TextDecoder<"u"&&new TextDecoder,Tv=0;try{Ys.decode(kv,{stream:!0}),Tv=1}catch{}var Ev=function(e){for(var t="",r=0;;){var n=e[r++],i=(n>127)+(n>223)+(n>239);if(r+i>e.length)return{s:t,r:Hl(e,r-1)};i?i==3?(n=((n&15)<<18|(e[r++]&63)<<12|(e[r++]&63)<<6|e[r++]&63)-65536,t+=String.fromCharCode(55296|n>>10,56320|n&1023)):i&1?t+=String.fromCharCode((n&31)<<6|e[r++]&63):t+=String.fromCharCode((n&15)<<12|(e[r++]&63)<<6|e[r++]&63):t+=String.fromCharCode(n)}};function Cv(e,t){if(t){for(var r="",n=0;n<e.length;n+=16384)r+=String.fromCharCode.apply(null,e.subarray(n,n+16384));return r}else{if(Ys)return Ys.decode(e);var i=Ev(e),o=i.s,r=i.r;return r.length&&tt(8),o}}var Mv=function(e,t){return t+30+Pt(e,t+26)+Pt(e,t+28)},_v=function(e,t,r){var n=Pt(e,t+28),i=Cv(e.subarray(t+46,t+46+n),!(Pt(e,t+8)&2048)),o=t+46+n,a=ht(e,t+20),s=r&&a==4294967295?Fv(e,o):[a,ht(e,t+24),ht(e,t+42)],l=s[0],c=s[1],u=s[2];return[Pt(e,t+10),l,c,i,o+Pt(e,t+30)+Pt(e,t+32),u]},Fv=function(e,t){for(;Pt(e,t)!=1;t+=4+Pt(e,t+2));return[Ka(e,t+12),Ka(e,t+4),Ka(e,t+20)]};function Av(e,t){for(var r={},n=e.length-22;ht(e,n)!=101010256;--n)(!n||e.length-n>65558)&&tt(13);var i=Pt(e,n+8);if(!i)return{};var o=ht(e,n+16),a=o==4294967295||i==65535;if(a){var s=ht(e,n-12);a=ht(e,s)==101075792,a&&(i=ht(e,s+32),o=ht(e,s+48))}for(var l=0;l<i;++l){var c=_v(e,o,a),u=c[0],d=c[1],f=c[2],p=c[3],m=c[4],h=c[5],g=Mv(e,h);o=m,u?u==8?r[p]=Pv(e.subarray(g,g+d),{out:new Qe(f)}):tt(14,"unknown compression type "+u):r[p]=Hl(e,g,g+d)}return r}function em(e){const t=typeof e=="string"?(()=>{try{return JSON.parse(e)}catch{return null}})():e;if(!t||typeof t!="object")return null;const r=t.w,n=t.h,i=t.fr,o=t.ip,a=t.op;if(typeof r!="number"||typeof n!="number"||typeof i!="number"||typeof a!="number"||!Array.isArray(t.layers))return null;const l=Math.max(1,Math.round(a-(typeof o=="number"?o:0))),c=i>0?i:30;return{width:Math.round(r),height:Math.round(n),frameRate:c,totalFrames:l,durationSeconds:l/c}}function Ov(e){return e.length>=4&&e[0]===80&&e[1]===75&&e[2]===3&&e[3]===4}function zv(e,t){const r=`${t}.json`.toLowerCase();return e.find(n=>n.split("/").pop()?.toLowerCase()===r)}function Dv(e,t){const r=Object.keys(e).filter(a=>/(^|\/)(?:animations|a)\/[^/]+\.json$/i.test(a));if(r.length===0)return[];const n=a=>{if(!a)return null;const s=zv(r,a);return s?[s,...r.filter(l=>l!==s)]:null},i=n(t);if(i)return i;const o=Object.keys(e).find(a=>/(^|\/)manifest\.json$/i.test(a));if(!o)return r;try{const a=JSON.parse(new TextDecoder().decode(e[o]));return n(a.animations?.[0]?.id)??r}catch{return r}}function Rv(e,t){let r;try{r=Av(e)}catch{return null}const n=new TextDecoder;for(const i of Dv(r,t))try{const o=em(JSON.parse(n.decode(r[i])));if(o)return o}catch{}return null}function Lv(e,t){if(Ov(e))return Rv(e,t);try{return em(new TextDecoder().decode(e))}catch{return null}}const tm=["video/mp4","video/webm","video/quicktime","video/x-matroska","video/matroska","video/x-msvideo"],rm=["audio/mp3","audio/mpeg","audio/wav","audio/aac","audio/x-m4a","audio/mp4","audio/ogg","audio/webm"],nm=["image/jpeg","image/jpg","image/png","image/gif","image/webp","image/svg+xml"],jl=["application/lottie+json"],Bv=new Set(["","application/octet-stream","binary/octet-stream"]),Uv=new Set([".mkv",".m4a",".json"]),Nv={".mp4":"video/mp4",".webm":"video/webm",".mov":"video/quicktime",".mkv":"video/x-matroska",".avi":"video/x-msvideo",".mp3":"audio/mpeg",".wav":"audio/wav",".aac":"audio/aac",".m4a":"audio/x-m4a",".ogg":"audio/ogg",".opus":"audio/ogg",".jpg":"image/jpeg",".jpeg":"image/jpeg",".png":"image/png",".gif":"image/gif",".webp":"image/webp",".svg":"image/svg+xml",".json":"application/lottie+json",".lottie":"application/lottie+json"};function ql(e){const t=e.name.toLowerCase().match(/\.[^.]+$/)?.[0],r=t?Nv[t]:void 0;return t&&r&&Uv.has(t)?r:Bv.has(e.type)?r||"":e.type}function $v(e){if(e.size===0)return{valid:!1,error:"File is empty"};const t=[...tm,...rm,...nm,...jl],r=ql(e);return t.includes(r)?e.name.length>255?{valid:!1,error:"Filename too long (max 255 characters)"}:{valid:!0}:{valid:!1,error:`Unsupported file type: ${r||e.name.split(".").pop()}. Supported types: video (mp4, webm, mov, mkv, avi), audio (mp3, wav, aac, m4a, ogg/opus), image (jpg/jpeg, png, gif, webp, svg), lottie (json, lottie)`}}async function uD(e){const t=$v(e);if(!t.valid)return t;if(jv(ql(e))){const r=new Uint8Array(await e.arrayBuffer());if(!Lv(r))return{valid:!1,error:`Not a valid Lottie animation: ${e.name}`}}return t}function Hv(e){const t=e.split(";")[0]?.trim()??e;return tm.includes(t)?"video":rm.includes(t)?"audio":nm.includes(t)?"image":jl.includes(t)?"lottie":"unknown"}function jv(e){return jl.includes(e)}function dD(e,t){e.forEach(r=>{r.reject(t)}),e.clear()}function qv(e){let t=null,r=null;function n(){const s=e.createWorker();return r=e.setupWorker?.(s)??null,t=s,s}function i(){return t??n()}function o(){return t}function a(){if(!t)return;const s=t;t=null;try{r?.()}finally{r=null,s.terminate()}}return{getWorker:i,peekWorker:o,terminate:a}}const Cn=new Map;function Xs(e,t,r){Cn.set(e,{blob:t,metadata:r})}function fD(e){return Cn.get(e)?.blob??null}function pD(e){return Cn.get(e)?.metadata??null}function mD(e){return Cn.has(e)}function hD(e){const t=Cn.get(e)?.metadata;return t?.opfsPath?{storageType:"opfs",opfsPath:t.opfsPath,fileSize:t.fileSize}:null}function im(e){Cn.delete(e)}class $t extends Error{code;status;extra;constructor(t,r,n,i){super(r),this.name="StorageClientError",this.code=t,this.status=n,this.extra=i}}class Xo extends $t{serverRevision;constructor(t,r,n){super("conflict",t,409,n),this.name="StorageConflictError",this.serverRevision=r}}function Kv(e){return e instanceof Xo}const Gv=1500,om="/api/editor",Wv=(e,t)=>{const{promise:r,resolve:n,reject:i}=Promise.withResolvers();if(t?.aborted)return i(t.reason??new DOMException("Đã huỷ tác vụ","AbortError")),r;const o=()=>{clearTimeout(a),i(t?.reason??new DOMException("Đã huỷ tác vụ","AbortError"))},a=setTimeout(()=>{t?.removeEventListener("abort",o),n()},e);return t?.addEventListener("abort",o,{once:!0}),r};async function Vv(e,t={}){const n=await(t.request??fetch).call(globalThis,`${t.baseUrl??om}/jobs/${encodeURIComponent(e)}`,{cache:"no-store",signal:t.signal}),i=await n.json().catch(()=>({}));if(!n.ok||!i.job)throw new Error(typeof i.error=="string"?i.error:`Không đọc được tác vụ (${n.status})`);return i.job}async function Zv(e,t={}){for(;;){if(t.signal?.aborted)throw t.signal.reason??new DOMException("Đã huỷ tác vụ","AbortError");const r=await Vv(e,t);if(r.state==="succeeded")return r.result??{};if(r.state==="failed")throw new Error(r.error?.error??"Không xong được tác vụ");if(r.state==="cancelled")throw new Error(r.error?.error??"Đã huỷ tác vụ");await Wv(t.intervalMs??Gv,t.signal)}}async function Yv(e,t={}){const r=t.request??fetch;try{await r.call(globalThis,`${t.baseUrl??om}/jobs/${encodeURIComponent(e)}`,{method:"DELETE",cache:"no-store"})}catch{}}function Qs(e){const t=Object.values(e).filter(n=>typeof n=="number");return Object.entries(e).filter(([n,i])=>t.indexOf(+n)===-1).map(([n,i])=>i)}function tu(e,t="|"){return e.map(r=>um(r)).join(t)}function Js(e,t){return typeof t=="bigint"?t.toString():t}class Xv{constructor(t){this._getter=t,this._value=void 0}get value(){const t=this._getter;return t!==void 0&&(this._value=t(),this._getter=void 0),this._value}}function Ta(e){return new Xv(e)}function Qv(e){return e==null}function Kl(e){const t=e.startsWith("^")?1:0,r=e.endsWith("$")?e.length-1:e.length;return e.slice(t,r)}function Jv(e,t){const r=e/t,n=Math.round(r),i=4*Number.EPSILON*Math.max(Math.abs(r),1);return Math.abs(r-n)<i?0:r-n}const ru=Symbol("evaluating");function eb(e,t,r){let n;Object.defineProperty(e,t,{get(){if(n!==ru)return n===void 0&&(n=ru,n=r()),n},set(i){Object.defineProperty(e,t,{value:i})},configurable:!0})}function Zt(e,t,r){Object.defineProperty(e,t,{value:r,writable:!0,enumerable:!0,configurable:!0})}function Gl(e){const t=Object.getOwnPropertyDescriptor(e,"shape");return t?.get?t.get.raw:t?.value}function Gt(e){return Gl(e._zod.def)??e._zod.def.shape}function am(e,t,r){Object.defineProperty(e,t,{get(){const n=r();return Zt(this,t,n),n},enumerable:!0,configurable:!0})}function sm(e,t,r){t in e?Zt(e,t,r):e[t]=r}function Lr(e,t,r,n){const i=Gt(t);for(const o of r){const a=Object.getOwnPropertyDescriptor(i,o);a.enumerable&&(a.get?am(e,o,()=>{const s=t._zod.def.shape[o];return n?n(s,o):s}):sm(e,o,n?n(a.value,o):a.value))}}function tb(e,t){for(const r of Reflect.ownKeys(t)){const n=Object.getOwnPropertyDescriptor(t,r);n.enumerable&&(n.get?am(e,r,()=>t[r]):sm(e,r,n.value))}}function Je(...e){const t={};for(const r of e){const n=Object.getOwnPropertyDescriptors(r);Object.assign(t,n)}return Object.defineProperties({},t)}function rb(e){return JSON.stringify(e)}function nb(e){return e.toLowerCase().trim().replace(/[^\w\s-]/g,"").replace(/[\s_-]+/g,"-").replace(/^-+|-+$/g,"")}const lm="captureStackTrace"in Error?Error.captureStackTrace:(...e)=>{};function Si(e){return typeof e=="object"&&e!==null&&!Array.isArray(e)}const ib=Ta(()=>{if(st.jitless||typeof navigator<"u"&&navigator?.userAgent?.includes("Cloudflare"))return!1;try{const e=Function;return new e(""),!0}catch{return!1}});function Sn(e){if(Si(e)===!1)return!1;const t=e.constructor;if(t===void 0||typeof t!="function")return!0;const r=t.prototype;return!(Si(r)===!1||Object.prototype.hasOwnProperty.call(r,"isPrototypeOf")===!1)}function cm(e){return Sn(e)?{...e}:Array.isArray(e)?[...e]:e instanceof Map?new Map(e):e instanceof Set?new Set(e):e}const ob=new Set(["string","number","symbol"]);function In(e){return e.replace(/[.*+?^${}()|[\]\\]/g,"\\$&")}function yr(e,t,r){const n=new e._zod.constr(t??e._zod.def);return(!t||r?.parent)&&(n._zod.parent=e),n}function N(e){const t=e;if(!t)return{};if(typeof t=="string")return{error:()=>t};if(t?.message!==void 0){if(t?.error!==void 0)throw new Error("Cannot specify both `message` and `error` params");t.error=t.message}return delete t.message,typeof t.error=="string"?{...t,error:()=>t.error}:t}function um(e){return typeof e=="bigint"?e.toString()+"n":typeof e=="string"?`"${e}"`:`${e}`}function ab(e){return Object.keys(e).filter(t=>e[t]._zod.optin!==void 0&&e[t]._zod.optout==="optional")}const dm={safeint:[Number.MIN_SAFE_INTEGER,Number.MAX_SAFE_INTEGER],int32:[-2147483648,2147483647],uint32:[0,4294967295],float32:[-34028234663852886e22,34028234663852886e22],float64:[-Number.MAX_VALUE,Number.MAX_VALUE]},sb={int64:[BigInt("-9223372036854775808"),BigInt("9223372036854775807")],uint64:[BigInt(0),BigInt("18446744073709551615")]};function lb(e,t){const r=e._zod.def,n=r.checks;if(n&&n.length>0)throw new Error(".pick() cannot be used on object schemas containing refinements");const o={};return Lr(o,e,Ea(e,t)),yr(e,Je(r,{shape:o,checks:[]}))}function Ea(e,t){const r=Gt(e),n=[];for(const i of Reflect.ownKeys(t)){if(!Object.getOwnPropertyDescriptor(r,i)?.enumerable)throw new Error(`Unrecognized key: "${String(i)}"`);t[i]&&n.push(i)}return n}function cb(e,t){const r=e._zod.def,n=r.checks;if(n&&n.length>0)throw new Error(".omit() cannot be used on object schemas containing refinements");const o=new Set(Ea(e,t)),a={};return Lr(a,e,Reflect.ownKeys(Gt(e)).filter(s=>!o.has(s))),yr(e,Je(r,{shape:a,checks:[]}))}function ub(e,t){if(!Sn(t))throw new Error("Invalid input to extend: expected a plain object");const r=e._zod.def.checks;if(r&&r.length>0){const i=Gt(e);for(const o of Reflect.ownKeys(t))if(Object.getOwnPropertyDescriptor(i,o)!==void 0)throw new Error("Cannot overwrite keys on object schemas containing refinements. Use `.safeExtend()` instead.")}return yr(e,Je(e._zod.def,{shape:fm(e,t)}))}function fm(e,t){const r={};return Lr(r,e,Reflect.ownKeys(Gt(e))),tb(r,t),r}function db(e,t){if(!Sn(t))throw new Error("Invalid input to safeExtend: expected a plain object");return yr(e,Je(e._zod.def,{shape:fm(e,t)}))}function fb(e,t){if(!t?._zod?.def)throw new Error("Invalid input to merge: expected an object schema. To merge a plain shape, use `.extend()`.");if(e._zod.def.checks?.length)throw new Error(".merge() cannot be used on object schemas containing refinements. Use .safeExtend() instead.");const r={};Lr(r,e,Reflect.ownKeys(Gt(e))),Lr(r,t,Reflect.ownKeys(Gt(t)));const n=Je(e._zod.def,{shape:r,get catchall(){return t._zod.def.catchall},checks:t._zod.def.checks??[]});return yr(e,n)}function nu(e,t,r,n="partial"){const o=t._zod.def.checks;if(o&&o.length>0)throw new Error(`.${n}() cannot be used on object schemas containing refinements`);const s=r?new Set(Ea(t,r)):void 0,l={};return Lr(l,t,Reflect.ownKeys(Gt(t)),e&&((c,u)=>s&&!s.has(u)?c:new e({type:"optional",innerType:c}))),yr(t,Je(t._zod.def,{shape:l,checks:[]}))}function pb(e,t,r){const n=r?new Set(Ea(t,r)):void 0,i={};return Lr(i,t,Reflect.ownKeys(Gt(t)),(o,a)=>n&&!n.has(a)?o:new e({type:"nonoptional",innerType:o})),yr(t,Je(t._zod.def,{shape:i}))}function gt(e,t=0){if(e.aborted===!0)return!0;for(let r=t;r<e.issues.length;r++)if(e.issues[r]?.continue!==!0)return!0;return!1}function mb(e,t=0){if(e.aborted===!0)return!0;for(let r=t;r<e.issues.length;r++)if(e.issues[r]?.continue===!1)return!0;return!1}function ir(e,t){return t.map(r=>{var n;return(n=r).path??(n.path=[]),r.path.unshift(e),r})}function Dn(e){return typeof e=="string"?e:e?.message}function iu(e){const t=[];for(const r of e??[]){const n=r._zod.def;if(!/_(length|size)$|_equals$/.test(n.check))break;const i=n.maximum??n.length??n.size;i>=0&&t.push([i,r])}return t}function el(e,t,r){var n;for(let i=t;i<e.length;i++)(n=e[i]).schema??(n.schema=r)}function fr(e,t,r){var n;const i=e.inst?._zod?.traits;i?.has("$ZodType")&&(i.has("$ZodCheck")?(n=e).schema??(n.schema=e.inst):e.schema=e.inst);const o=e.schema!==e.inst?e.schema?._zod.def?.error:void 0,a=e.message?e.message:Dn(e.inst?._zod.def?.error?.(e))??Dn(o?.(e))??Dn(t?.error?.(e))??Dn(r.customError?.(e))??Dn(r.localeError?.(e))??"Invalid input",s={};for(const l of Object.keys(e))l==="inst"||l==="schema"||l==="continue"||l==="input"||l==="__proto__"||(s[l]=e[l]);return s.path??(s.path=[]),s.message=a,t?.reportInput&&(s.input=e.input),s}const hb=/[\uD800-\uDBFF]/;function Wl(e){const t=e.length;if(!hb.test(e))return t;let r=t;for(let n=0;n<t-1;n++)(e.charCodeAt(n)&64512)===55296&&(e.charCodeAt(n+1)&64512)===56320&&(r--,n++);return r}function Vl(e){return Array.isArray(e)?"array":typeof e=="string"?"string":"unknown"}function gb(e){const t=typeof e;switch(t){case"number":return Number.isNaN(e)?"nan":"number";case"object":{if(e===null)return"null";if(Array.isArray(e))return"array";const r=e;if(r&&Object.getPrototypeOf(r)!==Object.prototype&&"constructor"in r&&r.constructor)return r.constructor.name}}return t}function Ii(...e){const[t,r,n]=e;return typeof t=="string"?{message:t,code:"custom",input:r,inst:n}:{...t}}function yb(e,t){for(const r in t){const n=Object.getOwnPropertyDescriptor(t,r);n.get?Object.defineProperty(e,r,{...n,enumerable:!1}):vb(e,r,n.value)}}function pr(e,t,r,n=!0){return Object.defineProperty(e,t,{configurable:!0,writable:!0,enumerable:n,value:r}),r}function pm(e,t,r){return pr(e,t,r,!1)}function mm(e,t){for(const r in e){const n=e[r];Object.defineProperty(t,r,{configurable:!0,enumerable:!0,get(){return pr(this,r,n(this))},set(i){pr(this,r,i)}})}return t}function vb(e,t,r){Object.defineProperty(e,t,{configurable:!0,get(){return this==null?r:pr(this,t,r.bind(this))},set(n){pr(this,t,n)}})}function bb(e,t){const r=Object.getPrototypeOf(e);return t in r?void 0:r}let Ga,Jt=!1;const Sb={configurable:!0,get(){Jt=!0}};function X(e,t,r){const n=Object.getPrototypeOf(e._zod);if(t in n&&Ga!==e._zod){Ga=void 0;return}Ga=e._zod,Object.defineProperty(n,t,{configurable:!0,get(){Object.defineProperty(this,t,Sb);const i=Jt;Jt=!1;try{const o=r(this);return Jt?delete this[t]:Object.defineProperty(this,t,{configurable:!0,writable:!0,value:o}),Jt=Jt||i,o}catch(o){throw delete this[t],Jt=Jt||i,o}},set(i){Object.defineProperty(this,t,{configurable:!0,writable:!0,value:i})}})}function Ib(e,t,r,n){const i=bb(e,t);i&&Object.defineProperty(i,t,{configurable:!0,get(){const o={configurable:!0,writable:!0,enumerable:n,value:void 0};return Object.defineProperty(this,t,o),o.value=r(this),Object.defineProperty(this,t,o),o.value},set(o){Object.defineProperty(this,t,{configurable:!0,writable:!0,enumerable:n,value:o})}})}const xb="~constantCatch";function wb(e){const t=()=>e;return t[xb]=!0,t}var ou;const Wa={value:void 0,enumerable:!1};let au="captureStackTrace"in Error?Error:null;function kb(e){const t=au;if(t){const r=t.stackTraceLimit;if(typeof r=="number"){try{t.stackTraceLimit=0}catch{return au=null,new e}try{return new e}finally{t.stackTraceLimit=r}}}return new e}function _(e,t,r,n){const i={};function o(f){this.def=f,this.constr=d,this.traits=new Set}o.prototype=i;const a=r,s=a&&new WeakSet;function l(f,p){if(f._zod){if(f._zod.traits.has(e))return}else{Wa.value=new o(p);try{Object.defineProperty(f,"_zod",Wa)}finally{Wa.value=void 0}}if(f._zod.traits.add(e),t(f,p),s){const h=Object.getPrototypeOf(f),g=f._zod.constr.prototype;let v=h;for(;v&&v!==g;)v=Object.getPrototypeOf(v);const x=v??h;s.has(x)||(s.add(x),yb(x,a))}const m=d.prototype;for(const h in m)Object.prototype.hasOwnProperty.call(m,h)&&(h in f||(f[h]=m[h].bind(f)))}const c=n?.Parent??Object;class u extends c{}Object.defineProperty(u,"name",{value:e});function d(f){const p=n?.Parent?kb(u):this;l(p,f);const m=p._zod.deferred;if(m){for(const g of m)g();p._zod.deferred=void 0}const h=globalThis.__zod_globalConfig?.postProcessor;return h&&h(p),p}return Object.defineProperty(d,"init",{value:l}),Object.defineProperty(d,Symbol.hasInstance,{value:f=>n?.Parent&&f instanceof n.Parent?!0:f?._zod?.traits?.has(e)}),Object.defineProperty(d,"name",{value:e}),d}class Dr extends Error{constructor(){super("Encountered Promise during synchronous parse. Use .parseAsync() instead.")}}class hm extends Error{constructor(t){super(`Encountered unidirectional transform during encode: ${t}`),this.name="ZodEncodeError"}}(ou=globalThis).__zod_globalConfig??(ou.__zod_globalConfig={});const st=globalThis.__zod_globalConfig;function Ft(e){return e&&Object.assign(st,e),st}function Pb(){const e=this._zod;return e.message??(e.message=JSON.stringify(e.def,Js,2)),e.message}function Tb(e){this._zod.message=e}const Eb={get:Pb,set:Tb,enumerable:!0,configurable:!0},Va={value:void 0,enumerable:!1},su=new WeakSet([Object.prototype,Error.prototype]),Cb=(e,t)=>{e.name="$ZodError",Va.value=t,Object.defineProperty(e,"issues",Va),Va.value=void 0,Object.defineProperty(e,"message",Eb);const r=Object.getPrototypeOf(e);su.has(r)||(su.add(r),Object.defineProperty(r,"toString",{configurable:!0,enumerable:!1,get(){const n=()=>this.message;return Object.defineProperty(this,"toString",{value:n,configurable:!0,writable:!0}),n},set(n){Object.defineProperty(this,"toString",{value:n,configurable:!0,writable:!0})}}))},Mb=_("$ZodError",Cb);function _b(e,t,r){return Object.prototype.hasOwnProperty.call(e,t)||(t==="__proto__"?Object.defineProperty(e,t,{value:r(),writable:!0,enumerable:!0,configurable:!0}):e[t]=r()),e[t]}function Fb(e,t=r=>r.message){const r={},n=[];for(const i of e.issues)i.path.length>0?_b(r,i.path[0],()=>[]).push(t(i)):n.push(t(i));return{formErrors:n,fieldErrors:r}}function Ab(e,t=r=>r.message){const r={_errors:[]},n=(i,o=[])=>{for(const a of i.issues)if(a.code==="invalid_union"&&a.errors.length)a.errors.map(s=>n({issues:s},[...o,...a.path]));else if(a.code==="invalid_key")n({issues:a.issues},[...o,...a.path]);else if(a.code==="invalid_element")n({issues:a.issues},[...o,...a.path]);else{const s=[...o,...a.path];if(s.length===0)r._errors.push(t(a));else{let l=r,c=0;for(;c<s.length;){const u=s[c],d=c===s.length-1;if(u==="_errors"){d&&l._errors.push(t(a)),c++;continue}Object.prototype.hasOwnProperty.call(l,u)||Object.defineProperty(l,u,{value:{_errors:[]},enumerable:!0,writable:!0,configurable:!0});const f=l[u];d&&f._errors.push(t(a)),l=f,c++}}}};return n(e),r}function Ca(e,t){return{callee:t?.callee??e,Err:t?.Err}}const Zl=e=>{const t=(r,n,i,o)=>{const a=i?{...i,async:!1}:{async:!1},s=r._zod.run({value:n,issues:[]},a);if(s instanceof Promise)throw new Dr;if(s.issues.length){const l=new(o?.Err??e)(s.issues.map(c=>fr(c,a,Ft())));throw lm(l,o?.callee??t),l}return s.value};return t},Yl=e=>{const t=async(r,n,i,o)=>{const a=i?{...i,async:!0}:{async:!0};let s=r._zod.run({value:n,issues:[]},a);if(s instanceof Promise&&(s=await s),s.issues.length){const l=new(o?.Err??e)(s.issues.map(c=>fr(c,a,Ft())));throw lm(l,o?.callee??t),l}return s.value};return t},Xl=e=>(t,r,n)=>{const i=n?{...n,async:!1}:{async:!1},o=t._zod.run({value:r,issues:[]},i);if(o instanceof Promise)throw new Dr;return o.issues.length?gm(e,o.issues,i):{success:!0,data:o.value}};function gm(e,t,r){let n;return{success:!1,get error(){return n||(n=new e(t.map(i=>fr(i,r,Ft()))),t=void 0,r=void 0),n},set error(i){n=i,t=void 0,r=void 0}}}const Ql=e=>async(t,r,n)=>{const i=n?{...n,async:!0}:{async:!0};let o=t._zod.run({value:r,issues:[]},i);return o instanceof Promise&&(o=await o),o.issues.length?gm(e,o.issues,i):{success:!0,data:o.value}},Ob=Symbol.for("zod.compile.invalid"),zb=Symbol.for("zod.compile.fallback"),Db=((e,t,r)=>{const n=e._zod.bag.validator;if(n!==void 0){if(n(t)!==Ob)return!0;if(n.definite===!0&&r===void 0)return!1}return Rb(e,t,r)});function Rb(e,t,r){const n=r?{...r,async:!1,abortEarly:!0}:{async:!1,abortEarly:!0},i=e._zod.bag.fallbackRun;let o;if(i?(n[zb]=!0,o=i({value:t,issues:[]},n)):o=e._zod.run({value:t,issues:[]},n),o instanceof Promise)throw new Dr;return o.issues.length===0}const Lb=async(e,t,r)=>{const n=r?{...r,async:!0,abortEarly:!0}:{async:!0,abortEarly:!0};let i=e._zod.run({value:t,issues:[]},n);return i instanceof Promise&&(i=await i),i.issues.length===0},Bb=e=>{const t=Zl(e),r=(n,i,o,a)=>{const s=o?{...o,direction:"backward"}:{direction:"backward"};return t(n,i,s,Ca(r,a))};return r},Ub=e=>{const t=Zl(e),r=(n,i,o,a)=>t(n,i,o,Ca(r,a));return r},Nb=e=>{const t=Yl(e),r=async(n,i,o,a)=>{const s=o?{...o,direction:"backward"}:{direction:"backward"};return await t(n,i,s,Ca(r,a))};return r},$b=e=>{const t=Yl(e),r=async(n,i,o,a)=>await t(n,i,o,Ca(r,a));return r},Hb=e=>(t,r,n)=>{const i=n?{...n,direction:"backward"}:{direction:"backward"};return Xl(e)(t,r,i)},jb=e=>(t,r,n)=>Xl(e)(t,r,n),qb=e=>async(t,r,n)=>{const i=n?{...n,direction:"backward"}:{direction:"backward"};return Ql(e)(t,r,i)},Kb=e=>async(t,r,n)=>Ql(e)(t,r,n),Gb=/^[cC][0-9a-z]{6,}$/,Wb=/^[0-9a-z]+$/,Vb=/^[0-7][0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{25}$/,Zb=/^[0-9a-vA-V]{20}$/,Yb=/^[A-Za-z0-9]{27}$/,Xb=/^[a-zA-Z0-9_-]{21}$/;function Qb(e){return new RegExp(`^[a-zA-Z0-9_-]{${e}}$`)}const Jb=/^P(?:(\d+W)|(?!.*W)(?=\d|T\d)(\d+Y)?(\d+M)?(\d+D)?(T(?=\d)(\d+H)?(\d+M)?(\d+([.,]\d+)?S)?)?)$/,eS=/^([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$/,lu=e=>e?new RegExp(`^([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-${e}[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12})$`):/^([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}|00000000-0000-0000-0000-000000000000|ffffffff-ffff-ffff-ffff-ffffffffffff)$/,tS=/^(?:[A-Za-z0-9_'+\-]+\.)*[A-Za-z0-9_'+\-]*[A-Za-z0-9_+-]@(?:[A-Za-z0-9][A-Za-z0-9\-]*\.)+[A-Za-z]{2,}$/,rS="^(?=[\\s\\S]*[\\p{Extended_Pictographic}\\p{Regional_Indicator}\\u20E3])[\\p{Extended_Pictographic}\\p{Emoji_Component}]+$";function nS(){return new RegExp(rS,"u")}const iS=/^(?:(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]|[0-9])\.){3}(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]|[0-9])$/,oS=/^(([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$/,aS=/^((25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]|[0-9])\.){3}(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]|[0-9])\/([0-9]|[1-2][0-9]|3[0-2])$/,sS=/^(([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))\/(12[0-8]|1[01][0-9]|[1-9]?[0-9])$/,lS=/^$|^(?:[0-9a-zA-Z+/]{4})*(?:(?:[0-9a-zA-Z+/]{2}==)|(?:[0-9a-zA-Z+/]{3}=))?$/,cS=/^(?:[A-Za-z0-9_-]{4})*(?:[A-Za-z0-9_-]{2,3})?$/,uS=/^https?$/,dS=/^\+[1-9]\d{6,14}$/,ym="(?:(?:\\d\\d[2468][048]|\\d\\d[13579][26]|\\d\\d0[48]|[02468][048]00|[13579][26]00)-02-29|\\d{4}-(?:(?:0[13578]|1[02])-(?:0[1-9]|[12]\\d|3[01])|(?:0[469]|11)-(?:0[1-9]|[12]\\d|30)|(?:02)-(?:0[1-9]|1\\d|2[0-8])))";function fS(e){return new RegExp(`^${e}$`)}const pS=fS(ym);function tl(e){const t="(?:[01]\\d|2[0-3]):[0-5]\\d";return typeof e.precision=="number"?e.precision===-1?`${t}`:e.precision===0?`${t}:[0-5]\\d`:`${t}:[0-5]\\d\\.\\d{${e.precision}}`:e.seconds?`${t}:[0-5]\\d(?:\\.\\d+)?`:`${t}(?::[0-5]\\d(?:\\.\\d+)?)?`}function mS(e){return new RegExp(`^${tl(e)}$`)}function hS(e){const t=["Z"];e.offset&&t.push("([+-](?:[01]\\d|2[0-3]):[0-5]\\d)");const r=`${tl({precision:e.precision,seconds:!0})}(?:${t.join("|")})`,n=e.local?`${r}|${tl({precision:e.precision})}`:r;return new RegExp(`^${ym}T(?:${n})$`)}const gS=/^[\s\S]{0,}$/,yS=/^-?\d+$/,Jl=/^-?\d+(?:\.\d+)?$/,vS=/^(?:true|false)$/i,bS=/^null$/i,SS=/^[^A-Z]*$/,IS=/^[^a-z]*$/,Ve=_("$ZodCheck",(e,t)=>{var r;e._zod??(e._zod={}),e._zod.def=t,(r=e._zod).onattach??(r.onattach=[])}),ec=e=>{const t=e.value;return!Qv(t)&&t.length!==void 0},Qo={number:"number",bigint:"bigint",object:"date"},vm=_("$ZodCheckLessThan",(e,t)=>{Ve.init(e,t);const r=Qo[typeof t.value];e._zod.check=n=>{(t.inclusive?n.value<=t.value:n.value<t.value)||n.issues.push({origin:Qo[typeof n.value]??r,code:"too_big",maximum:typeof t.value=="object"?t.value.getTime():t.value,input:n.value,inclusive:t.inclusive,inst:e,continue:!t.abort})}}),bm=_("$ZodCheckGreaterThan",(e,t)=>{Ve.init(e,t);const r=Qo[typeof t.value];e._zod.check=n=>{(t.inclusive?n.value>=t.value:n.value>t.value)||n.issues.push({origin:Qo[typeof n.value]??r,code:"too_small",minimum:typeof t.value=="object"?t.value.getTime():t.value,input:n.value,inclusive:t.inclusive,inst:e,continue:!t.abort})}}),xS=_("$ZodCheckMultipleOf",(e,t)=>{Ve.init(e,t),e._zod.check=r=>{if(typeof r.value!=typeof t.value)throw new Error("Cannot mix number and bigint in multiple_of check.");(typeof r.value=="bigint"?t.value!==BigInt(0)&&r.value%t.value===BigInt(0):Jv(r.value,t.value)===0)||r.issues.push({origin:typeof r.value,code:"not_multiple_of",divisor:t.value,input:r.value,inst:e,continue:!t.abort})}}),wS=_("$ZodCheckNumberFormat",(e,t)=>{Ve.init(e,t),t.format=t.format||"float64";const r=t.format?.includes("int"),n=r?"int":"number",[i,o]=dm[t.format];e._zod.check=a=>{const s=a.value;if(r){if(!Number.isInteger(s)){a.issues.push({expected:n,format:t.format,code:"invalid_type",continue:!1,input:s,inst:e});return}if(!Number.isSafeInteger(s)){s>0?a.issues.push({input:s,code:"too_big",maximum:Number.MAX_SAFE_INTEGER,note:"Integers must be within the safe integer range.",inst:e,origin:n,inclusive:!0,continue:!t.abort}):a.issues.push({input:s,code:"too_small",minimum:Number.MIN_SAFE_INTEGER,note:"Integers must be within the safe integer range.",inst:e,origin:n,inclusive:!0,continue:!t.abort});return}}s<i&&a.issues.push({origin:"number",input:s,code:"too_small",minimum:i,inclusive:!0,inst:e,continue:!t.abort}),s>o&&a.issues.push({origin:"number",input:s,code:"too_big",maximum:o,inclusive:!0,inst:e,continue:!t.abort})}}),kS=_("$ZodCheckMaxLength",(e,t)=>{var r;Ve.init(e,t),(r=e._zod.def).when??(r.when=ec),e._zod.check=n=>{const i=n.value,o=i.length;if((typeof i=="string"&&o>t.maximum?Wl(i):o)<=t.maximum)return;const s=Vl(i);n.issues.push({origin:s,code:"too_big",maximum:t.maximum,inclusive:!0,input:i,inst:e,continue:!t.abort})}}),PS=_("$ZodCheckMinLength",(e,t)=>{var r;Ve.init(e,t),(r=e._zod.def).when??(r.when=ec),e._zod.check=n=>{const i=n.value,o=i.length;if((typeof i=="string"&&o>=t.minimum&&o<t.minimum*2?Wl(i):o)>=t.minimum)return;const s=Vl(i);n.issues.push({origin:s,code:"too_small",minimum:t.minimum,inclusive:!0,input:i,inst:e,continue:!t.abort})}}),TS=_("$ZodCheckLengthEquals",(e,t)=>{var r;Ve.init(e,t),(r=e._zod.def).when??(r.when=ec),e._zod.check=n=>{const i=n.value,o=i.length,a=typeof i=="string"&&o>=t.length&&o<=t.length*2?Wl(i):o;if(a===t.length)return;const s=Vl(i),l=a>t.length;n.issues.push({origin:s,...l?{code:"too_big",maximum:t.length}:{code:"too_small",minimum:t.length},inclusive:!0,exact:!0,input:n.value,inst:e,continue:!t.abort})}}),Ma=_("$ZodCheckStringFormat",(e,t)=>{var r,n;Ve.init(e,t),t.pattern?(r=e._zod).check??(r.check=i=>{t.pattern.lastIndex=0,!t.pattern.test(i.value)&&i.issues.push({origin:"string",code:"invalid_format",format:t.format,input:i.value,...t.pattern?{pattern:t.pattern.toString()}:{},inst:e,continue:!t.abort})}):(n=e._zod).check??(n.check=()=>{})}),ES=_("$ZodCheckRegex",(e,t)=>{Ma.init(e,t),e._zod.check=r=>{t.pattern.lastIndex=0,!t.pattern.test(r.value)&&r.issues.push({origin:"string",code:"invalid_format",format:"regex",input:r.value,pattern:t.pattern.toString(),inst:e,continue:!t.abort})}}),CS=_("$ZodCheckLowerCase",(e,t)=>{t.pattern??(t.pattern=SS),Ma.init(e,t)}),MS=_("$ZodCheckUpperCase",(e,t)=>{t.pattern??(t.pattern=IS),Ma.init(e,t)}),_S=_("$ZodCheckIncludes",(e,t)=>{Ve.init(e,t);const r=In(t.includes),n=new RegExp(typeof t.position=="number"?`^.{${t.position},}${r}`:r);t.pattern=n,e._zod.check=i=>{i.value.includes(t.includes,t.position)||i.issues.push({origin:"string",code:"invalid_format",format:"includes",includes:t.includes,input:i.value,inst:e,continue:!t.abort})}}),FS=_("$ZodCheckStartsWith",(e,t)=>{Ve.init(e,t);const r=new RegExp(`^${In(t.prefix)}.*`);t.pattern??(t.pattern=r),e._zod.check=n=>{n.value.startsWith(t.prefix)||n.issues.push({origin:"string",code:"invalid_format",format:"starts_with",prefix:t.prefix,input:n.value,inst:e,continue:!t.abort})}}),AS=_("$ZodCheckEndsWith",(e,t)=>{Ve.init(e,t);const r=new RegExp(`.*${In(t.suffix)}$`);t.pattern??(t.pattern=r),e._zod.check=n=>{n.value.endsWith(t.suffix)||n.issues.push({origin:"string",code:"invalid_format",format:"ends_with",suffix:t.suffix,input:n.value,inst:e,continue:!t.abort})}}),OS=_("$ZodCheckOverwrite",(e,t)=>{Ve.init(e,t),e._zod.check=r=>{r.value=t.tx(r.value)}});class zS{constructor(t=[],r={}){this.content=[],this.indent=0,this.args=t,this.closed=r}indented(t){this.indent+=1;try{t(this)}finally{this.indent-=1}}write(t){if(typeof t=="function"){t(this,{execution:"sync"}),t(this,{execution:"async"});return}const n=t.split(`
`).filter(a=>a),i=Math.min(...n.map(a=>a.length-a.trimStart().length)),o=n.map(a=>a.slice(i)).map(a=>" ".repeat(this.indent*2)+a);for(const a of o)this.content.push(a)}compile(){const t=Function,r=this?.content??[""];return new t(...Object.keys(this.closed),`return function (${this.args.join(", ")}) {
${r.join(`
`)}
};`)(...Object.values(this.closed))}}const DS={major:4,minor:6,patch:7},de=_("$ZodType",(e,t)=>{var r;e??(e={}),e._zod.def=t,e._zod.bag=e._zod.bag||{},e._zod.version=DS;const n=e._zod.def.checks,i=e._zod.traits.has("$ZodCheck")?[e,...n??[]]:n?.length?[...n]:[];for(const o of i)for(const a of o._zod.onattach)a(e);if(i.length===0)(r=e._zod).deferred??(r.deferred=[]),e._zod.deferred?.push(()=>{e._zod.run=e._zod.parse});else{const o=(s,l,c)=>{if(s.memo)return s;let u=gt(s),d;for(const f of l){if(f._zod.def.when){if(mb(s)||!f._zod.def.when(s))continue}else if(u)continue;const p=s.issues.length,m=f._zod.check(s);if(m instanceof Promise&&c?.async===!1)throw new Dr;if(d||m instanceof Promise)d=(d??Promise.resolve()).then(async()=>{await m,s.issues.length!==p&&(el(s.issues,p,e),u||(u=gt(s,p)))});else{if(s.issues.length===p)continue;el(s.issues,p,e),u||(u=gt(s,p))}}return d?d.then(()=>s):s},a=(s,l,c)=>{if(gt(s))return s.aborted=!0,s;const u=o(l,i,c);if(u instanceof Promise){if(c.async===!1)throw new Dr;return u.then(d=>e._zod.parse(d,c))}return e._zod.parse(u,c)};e._zod.run=(s,l)=>{if(l.skipChecks)return e._zod.parse(s,l);if(l.direction==="backward"){const u=e._zod.parse({value:s.value,issues:[]},{...l,skipChecks:!0});return u instanceof Promise?u.then(d=>a(d,s,l)):a(u,s,l)}const c=e._zod.parse(s,l);if(c instanceof Promise){if(l.async===!1)throw new Dr;return c.then(u=>o(u,i,l))}return o(c,i,l)}}},{get"~standard"(){return pm(this,"~standard",Im(this))},set"~standard"(e){pr(this,"~standard",e)}}),Sm=(e,t)=>e.issues.length?{issues:e.issues.map(r=>fr(r,t,Ft()))}:{value:e.value};async function RS(e,t){const r={async:!0};return Sm(await e._zod.run({value:t,issues:[]},r),r)}function Im(e){return{validate:t=>{const r={async:!1};try{const n=e._zod.run({value:t,issues:[]},r);if(!(n instanceof Promise))return Sm(n,r)}catch{}return RS(e,t)},vendor:"zod",version:1}}const tc=_("$ZodString",(e,t)=>{de.init(e,t),e._zod.pattern=t.pattern??gS,e._zod.parse=(r,n)=>{if(t.coerce)try{r.value=String(r.value)}catch{}return typeof r.value=="string"||r.issues.push({expected:"string",code:"invalid_type",input:r.value,inst:e}),r}}),ve=_("$ZodStringFormat",(e,t)=>{Ma.init(e,t),tc.init(e,t)}),LS=_("$ZodGUID",(e,t)=>{t.pattern??(t.pattern=eS),ve.init(e,t)}),BS=_("$ZodUUID",(e,t)=>{if(t.version){const n={v1:1,v2:2,v3:3,v4:4,v5:5,v6:6,v7:7,v8:8}[t.version];if(n===void 0)throw new Error(`Invalid UUID version: "${t.version}"`);t.pattern??(t.pattern=lu(n))}else t.pattern??(t.pattern=lu());ve.init(e,t)}),US=_("$ZodEmail",(e,t)=>{t.pattern??(t.pattern=tS),ve.init(e,t)}),xm=1,Jo=2;function wm(e){try{return typeof URL<"u"&&typeof URL.canParse=="function"?URL.canParse(e):(new URL(e),!0)}catch{return!1}}function NS(e,t){return!("normalize"in t)&&!("hostname"in t)&&!("protocol"in t)?wm(e)||Jo:$S(e,t)}function $S(e,t){if(!t.normalize&&t.protocol?.source===uS.source&&!/^https?:\/\//i.test(e))return xm;try{if(typeof URL<"u"){const r=URL;if(typeof r.parse=="function")return r.parse(e)??Jo}return new URL(e)}catch{return Jo}}const HS=/[\t\n\r]/g;function cu(e){return e.replace(HS,"")}function jS(e,t){return t.lastIndex=0,t.test(e.hostname)}function qS(e,t){return t.lastIndex=0,t.test(e.protocol.endsWith(":")?e.protocol.slice(0,-1):e.protocol)}const KS=_("$ZodURL",(e,t)=>{ve.init(e,t),e._zod.check=r=>{try{const n=r.value.trim(),i=NS(n,t);if(i===xm){r.issues.push({code:"invalid_format",format:"url",note:"Invalid URL format",input:r.value,inst:e,continue:!t.abort});return}if(i===Jo){r.issues.push({code:"invalid_format",format:"url",input:r.value,inst:e,continue:!t.abort});return}if(i===!0){r.value=cu(n);return}t.hostname&&!jS(i,t.hostname)&&r.issues.push({code:"invalid_format",format:"url",note:"Invalid hostname",pattern:t.hostname.source,input:r.value,inst:e,continue:!t.abort}),t.protocol&&!qS(i,t.protocol)&&r.issues.push({code:"invalid_format",format:"url",note:"Invalid protocol",pattern:t.protocol.source,input:r.value,inst:e,continue:!t.abort}),r.value=t.normalize?i.href:cu(n);return}catch{r.issues.push({code:"invalid_format",format:"url",input:r.value,inst:e,continue:!t.abort})}}}),GS=_("$ZodEmoji",(e,t)=>{t.pattern??(t.pattern=nS()),ve.init(e,t)}),WS=_("$ZodNanoID",(e,t)=>{if(t.length!==void 0&&(!Number.isInteger(t.length)||t.length<1))throw new Error(`Invalid nanoid length: ${t.length}`);t.pattern??(t.pattern=t.length===void 0?Xb:Qb(t.length)),ve.init(e,t)}),VS=_("$ZodCUID",(e,t)=>{t.pattern??(t.pattern=Gb),ve.init(e,t)}),ZS=_("$ZodCUID2",(e,t)=>{t.pattern??(t.pattern=Wb),ve.init(e,t)}),YS=_("$ZodULID",(e,t)=>{t.pattern??(t.pattern=Vb),ve.init(e,t)}),XS=_("$ZodXID",(e,t)=>{t.pattern??(t.pattern=Zb),ve.init(e,t)}),QS=_("$ZodKSUID",(e,t)=>{t.pattern??(t.pattern=Yb),ve.init(e,t)}),JS=_("$ZodISODateTime",(e,t)=>{t.pattern??(t.pattern=hS(t)),ve.init(e,t)}),eI=_("$ZodISODate",(e,t)=>{t.pattern??(t.pattern=pS),ve.init(e,t)}),tI=_("$ZodISOTime",(e,t)=>{t.pattern??(t.pattern=mS(t)),ve.init(e,t)}),rI=_("$ZodISODuration",(e,t)=>{t.pattern??(t.pattern=Jb),ve.init(e,t)}),nI=_("$ZodIPv4",(e,t)=>{t.pattern??(t.pattern=iS),ve.init(e,t)}),iI=/^[0-9a-fA-F:.]+$/;function km(e){return iI.test(e)?wm(`http://[${e}]`):!1}const oI=_("$ZodIPv6",(e,t)=>{t.pattern??(t.pattern=oS),ve.init(e,t),e._zod.check=r=>{km(r.value)||r.issues.push({code:"invalid_format",format:"ipv6",input:r.value,inst:e,continue:!t.abort})}}),aI=_("$ZodCIDRv4",(e,t)=>{t.pattern??(t.pattern=aS),ve.init(e,t)});function sI(e){const t=e.split("/");if(t.length!==2)return!1;const[r,n]=t;if(!n)return!1;const i=Number(n);return`${i}`!==n||i<0||i>128?!1:km(r)}const lI=_("$ZodCIDRv6",(e,t)=>{t.pattern??(t.pattern=sS),ve.init(e,t),e._zod.check=r=>{sI(r.value)||r.issues.push({code:"invalid_format",format:"cidrv6",input:r.value,inst:e,continue:!t.abort})}});function Pm(e){if(e==="")return!0;if(/\s/.test(e)||e.length%4!==0)return!1;try{return atob(e),!0}catch{return!1}}const Tm=/^[0-9a-zA-Z+/]*={0,2}$/,cI=_("$ZodBase64",(e,t)=>{t.pattern??(t.pattern=Tm),ve.init(e,t),e._zod.check=r=>{Pm(r.value)||r.issues.push({code:"invalid_format",format:"base64",input:r.value,inst:e,continue:!t.abort})}}),rc=/^[A-Za-z0-9_-]*$/;function uI(e){if(!rc.test(e))return!1;const t=e.replace(/[-_]/g,n=>n==="-"?"+":"/"),r=t.padEnd(Math.ceil(t.length/4)*4,"=");return Pm(r)}const dI=_("$ZodBase64URL",(e,t)=>{t.pattern??(t.pattern=rc),ve.init(e,t),e._zod.check=r=>{uI(r.value)||r.issues.push({code:"invalid_format",format:"base64url",input:r.value,inst:e,continue:!t.abort})}}),fI=_("$ZodE164",(e,t)=>{t.pattern??(t.pattern=dS),ve.init(e,t)});function pI(e,t=null){try{const r=e.split(".");if(r.length!==3)return!1;const[n]=r;if(!n)return!1;const i=JSON.parse(atob(n));return!("typ"in i&&i?.typ!=="JWT"||!i.alg||t&&(!("alg"in i)||i.alg!==t))}catch{return!1}}const mI=_("$ZodJWT",(e,t)=>{ve.init(e,t),e._zod.check=r=>{pI(r.value,t.alg)||r.issues.push({code:"invalid_format",format:"jwt",input:r.value,inst:e,continue:!t.abort})}}),Em=_("$ZodNumber",(e,t)=>{de.init(e,t),e._zod.pattern=Jl,e._zod.parse=(r,n)=>{if(t.coerce)try{r.value=Number(r.value)}catch{}const i=r.value;if(typeof i=="number"&&!Number.isNaN(i)&&Number.isFinite(i))return r;const o=typeof i=="number"?Number.isNaN(i)?"NaN":Number.isFinite(i)?void 0:String(i):void 0;return r.issues.push({expected:"number",code:"invalid_type",input:i,inst:e,...o?{received:o}:{}}),r}}),hI=_("$ZodNumberFormat",(e,t)=>{wS.init(e,t),Em.init(e,t)}),gI=_("$ZodBoolean",(e,t)=>{de.init(e,t),e._zod.pattern=vS,e._zod.parse=(r,n)=>{if(t.coerce)try{r.value=!!r.value}catch{}const i=r.value;return typeof i=="boolean"||r.issues.push({expected:"boolean",code:"invalid_type",input:i,inst:e}),r}}),yI=_("$ZodNull",(e,t)=>{de.init(e,t),e._zod.pattern=bS,e._zod.values=new Set([null]),e._zod.parse=(r,n)=>{const i=r.value;return i===null||r.issues.push({expected:"null",code:"invalid_type",input:i,inst:e}),r}}),vI=_("$ZodUnknown",(e,t)=>{de.init(e,t),e._zod.parse=r=>r}),bI=_("$ZodNever",(e,t)=>{de.init(e,t),e._zod.parse=(r,n)=>(r.issues.push({expected:"never",code:"invalid_type",input:r.value,inst:e}),r)});function SI(e,t){const r=iu(e._zod.def.checks);if(!r.length)return;const n=Math.min(...r.map(o=>o[0])),i=e._zod.parse;e._zod.parse=(o,a)=>{const s=t(o.value);if(s>n&&!a?.skipChecks&&a?.direction!=="backward"){for(const[l,c]of iu(e._zod.def.checks))if(s>l&&c._zod.def.when(o)){const u=o.issues.length;return c._zod.check(o),o.issues[u].continue=!1,el(o.issues,u,e),o}}return i(o,a)}}const II=e=>Array.isArray(e)?e.length:-1;function uu(e,t,r){e.issues.length&&t.issues.push(...ir(r,e.issues)),t.value[r]=e.value}const xI=_("$ZodArray",(e,t)=>{de.init(e,t);const r=st.memoizer;r?.attach(e),e._zod.parse=(n,i)=>{const o=n.value;if(!Array.isArray(o))return n.issues.push({expected:"array",code:"invalid_type",input:o,inst:e}),n;n.value=r?r.alloc(e,n,Array(o.length),i):Array(o.length);const a=[],s=i?.abortEarly;for(let l=0;l<o.length;l++){const c=o[l],u=t.element._zod.run({value:c,issues:[]},i);if(u instanceof Promise)a.push(u.then(d=>uu(d,n,l)));else if(uu(u,n,l),s&&u.issues.length!==0&&gt(u))break}return a.length?Promise.all(a).then(()=>n):n},SI(e,II)});function ea(e,t,r,n,i,o){const a=r in n,s=o==="optional";if(!(!a&&s&&i==="optional")){if(e.issues.length){if(i!==void 0&&s&&!a)return;t.issues.push(...ir(r,e.issues))}if(!a&&i===void 0){e.issues.length||t.issues.push({code:"invalid_type",expected:"nonoptional",input:void 0,path:[r]});return}e.value===void 0?(a||i==="defaulted"&&!s)&&(t.value[r]=void 0):t.value[r]=e.value}}const wI=[];function Cm(e){const t=Object.keys(e.shape),r=Object.getOwnPropertySymbols(e.shape),n=r.length?r:wI,i=n.length?[...t,...n]:t;for(const a of i)if(!e.shape?.[a]?._zod?.traits?.has("$ZodType"))throw new Error(`Invalid element at key "${String(a)}": expected a Zod schema`);const o=ab(e.shape);return{...e,allKeys:i,symbolKeys:n,keySet:new Set(t),numKeys:t.length,optionalKeys:new Set(o)}}function Mm(e,t,r,n,i,o,a){const s=[],l=i.keySet,c=i.catchall._zod,u=c.def.type,d=c.optin,f=c.optout;let p=0;for(const m in t){if(a&&r.issues.length!==p){if(gt(r,p))break;p=r.issues.length}if(l.has(m))continue;if(m==="__proto__"){u==="never"&&s.push(m);continue}if(u==="never"){s.push(m);continue}const h=c.run({value:t[m],issues:[]},n);h instanceof Promise?e.push(h.then(g=>ea(g,r,m,t,d,f))):ea(h,r,m,t,d,f)}return s.length&&r.issues.push({code:"unrecognized_keys",keys:s,input:t,inst:o,continue:!0}),e.length?Promise.all(e).then(()=>r):r}const kI=_("$ZodObject",(e,t)=>{de.init(e,t);const r=Object.getOwnPropertyDescriptor(t,"shape"),n=r?.get?r.get.raw:t.shape??{};if(n){const c=()=>{const u={...n};return Object.defineProperty(t,"shape",{value:u}),c.raw=u,u};c.raw=n,Object.defineProperty(t,"shape",{get:c})}const i=Ta(()=>Cm(t));X(e,"propValues",c=>{const u=c.def.shape,d={};for(const f in u){const p=u[f]._zod;if(p.values){Object.prototype.hasOwnProperty.call(d,f)||Zt(d,f,new Set);for(const m of p.values)d[f].add(m);p.optin!==void 0&&d[f].add(void 0)}}return d});const o=Si,a=t.catchall;let s;const l=st.memoizer;l?.attach(e),e._zod.parse=(c,u)=>{s??(s=i.value);const d=c.value;if(!o(d))return c.issues.push({expected:"object",code:"invalid_type",input:d,inst:e}),c;c.value=l?l.alloc(e,c,{},u):{};const f=[],p=s.shape,m=u?.abortEarly;let h=c.issues.length;for(const g of s.allKeys){if(m&&c.issues.length!==h){if(gt(c,h))break;h=c.issues.length}if(g==="__proto__")continue;const v=p[g],x=v._zod.optin,b=v._zod.optout,w=v._zod.run({value:d[g],issues:[]},u);w instanceof Promise?f.push(w.then(E=>ea(E,c,g,d,x,b))):ea(w,c,g,d,x,b)}return a?Mm(f,d,c,u,i.value,e,m===!0):f.length?Promise.all(f).then(()=>c):c}}),PI=_("$ZodObjectJIT",(e,t)=>{kI.init(e,t);const r=e._zod.parse,n=Ta(()=>Cm(t)),i=st.memoizer,o=p=>{const m=n.value,h=m.symbolKeys,g=new zS(["payload","ctx"],{shape:p,inst:e,memo:i,syms:h}),v=E=>`shape[${E}]._zod.run({ value: input[${E}], issues: [] }, ctx)`,x=(E,k)=>`
          let ${E}_ab = false;
          for (let i = 0; i < ${E}.issues.length; i++) {
            const iss = ${E}.issues[i];
            iss.path = iss.path ? [${k}, ...iss.path] : [${k}];
            payload.issues.push(iss);
            if (iss.continue !== true) ${E}_ab = true;
          }
          if (${E}_ab && ctx && ctx.abortEarly) {
            payload.value = newResult;
            return payload;
          }`;g.write("const input = payload.value;");const b=Object.create(null);let w=0;for(const E of m.allKeys)b[E]=`key_${w++}`;g.write(i?"const newResult = memo.alloc(inst, payload, {}, ctx);":"const newResult = {};");for(const E of m.allKeys){if(E==="__proto__")continue;const k=b[E],T=typeof E=="symbol"?`syms[${h.indexOf(E)}]`:rb(E),O=`${T} in input`,C=p[E],R=C?._zod?.optin,M=R!==void 0,D=C?._zod?.optout==="optional";if(g.write(`const ${k} = ${v(T)};`),M&&D){const L=R==="optional"?`${k}_present`:`${k}.value !== undefined || ${k}_present`;g.write(`
        const ${k}_present = ${O};
        if (!${k}.issues.length || ${k}_present) {
          if (${k}.issues.length) {${x(k,T)}
          }

          if (${L}) {
            newResult[${T}] = ${k}.value;
          }
        }

      `)}else M?(g.write(`
        if (${k}.issues.length) {${x(k,T)}
        }
      `),R==="defaulted"?g.write(`newResult[${T}] = ${k}.value;`):g.write(`
        if (${k}.value !== undefined || ${O}) {
          newResult[${T}] = ${k}.value;
        }
      `)):g.write(`
        const ${k}_present = ${O};
        if (${k}.issues.length) {${x(k,T)}
        }
        if (!${k}_present && !${k}.issues.length) {
          payload.issues.push({
            code: "invalid_type",
            expected: "nonoptional",
            input: undefined,
            path: [${T}]
          });
          if (ctx && ctx.abortEarly) {
            payload.value = newResult;
            return payload;
          }
        }

        if (${k}_present) {
          newResult[${T}] = ${k}.value;
        }

      `)}return g.write("payload.value = newResult;"),g.write("return payload;"),g.compile()};let a;const s=Si,l=!st.jitless,u=l&&ib.value,d=t.catchall;let f;e._zod.parse=(p,m)=>{f??(f=n.value);const h=p.value;return s(h)?l&&u&&m?.async===!1&&m.jitless!==!0?(a||(a=o(t.shape)),p=a(p,m),d?Mm([],h,p,m,f,e,m?.abortEarly===!0):p):r(p,m):(p.issues.push({expected:"object",code:"invalid_type",input:h,inst:e}),p)}});function du(e,t,r,n){for(const o of e)if(o.issues.length===0)return t.value=o.value,t;const i=e.filter(o=>!gt(o));return i.length===1?(t.value=i[0].value,i[0]):(t.issues.push({code:"invalid_union",input:t.value,inst:r,errors:e.map(o=>o.issues.map(a=>fr(a,n,Ft())))}),t)}const _m=_("$ZodUnion",(e,t)=>{de.init(e,t),X(e,"optin",n=>n.def.options.some(i=>i._zod.optin==="defaulted")?"defaulted":n.def.options.some(i=>i._zod.optin!==void 0)?"optional":void 0),X(e,"optout",n=>n.def.options.some(i=>i._zod.optout==="optional")?"optional":void 0),X(e,"values",n=>{if(n.def.options.every(i=>i._zod.values))return new Set(n.def.options.flatMap(i=>Array.from(i._zod.values)))}),X(e,"pattern",n=>{if(n.def.options.every(i=>i._zod.pattern)){const i=n.def.options.map(o=>o._zod.pattern);return new RegExp(`^(${i.map(o=>Kl(o.source)).join("|")})$`)}});const r=t.options.length===1?t.options[0]._zod.run:null;e._zod.parse=(n,i)=>{if(r)return r(n,i);let o=!1;const a=[];for(const s of t.options){const l=s._zod.run({value:n.value,issues:[]},i);if(l instanceof Promise)a.push(l),o=!0;else{if(l.issues.length===0)return l;a.push(l)}}return o?Promise.all(a).then(s=>du(s,n,e,i)):du(a,n,e,i)}});function TI(e){const t=new Map;for(const r of e.options){const n=r._zod.propValues?.[e.discriminator];if(!n||n.size===0)throw new Error(`Invalid discriminated union option at index "${e.options.indexOf(r)}"`);for(const i of n)if(t.has(i)){if(i!==void 0)throw new Error(`Duplicate discriminator value "${String(i)}"`);t.set(i,null)}else t.set(i,r)}return t}const EI=_("$ZodDiscriminatedUnion",(e,t)=>{t.inclusive=!1,_m.init(e,t);const r=e._zod.parse;X(e,"propValues",i=>{const o={};let a=0;for(const s of i.def.options){const l=s._zod.propValues;if(!l||Object.keys(l).length===0)throw new Error(`Invalid discriminated union option at index "${i.def.options.indexOf(s)}"`);l[i.def.discriminator]?.has(void 0)&&a++;for(const[c,u]of Object.entries(l)){Object.prototype.hasOwnProperty.call(o,c)||Zt(o,c,new Set);for(const d of u)o[c].add(d)}}return!i.def.unionFallback&&a>1&&o[i.def.discriminator]?.delete(void 0),o}),t.options.forEach((i,o)=>{const a=Gl(i._zod.def);if(a&&!Object.prototype.hasOwnProperty.call(a,t.discriminator))throw new Error(`Invalid discriminated union option at index "${o}"`)});const n=Ta(()=>TI(t));e._zod.parse=(i,o)=>{const a=i.value;if(!Si(a))return i.issues.push({code:"invalid_type",expected:"object",input:a,inst:e}),i;const s=a?.[t.discriminator],l=n.value.get(s);return l&&(s!==void 0||o.direction!=="backward")?l._zod.run(i,o):t.unionFallback||o.direction==="backward"?r(i,o):(i.issues.push({code:"invalid_union",errors:[],note:"No matching discriminator",discriminator:t.discriminator,options:Array.from(n.value.keys()).filter(c=>n.value.get(c)!==null),input:a,path:[t.discriminator],inst:e}),i)}}),CI=_("$ZodIntersection",(e,t)=>{de.init(e,t),e._zod.parse=(r,n)=>{const i=r.value,o=t.left._zod.run({value:i,issues:[]},n),a=t.right._zod.run({value:i,issues:[]},n);return o instanceof Promise||a instanceof Promise?Promise.all([o,a]).then(([l,c])=>fu(r,l,c)):fu(r,o,a)}});function rl(e,t){if(e===t)return{valid:!0,data:e};if(e instanceof Date&&t instanceof Date&&+e==+t)return{valid:!0,data:e};if(Sn(e)&&Sn(t)){const r=Object.keys(t),n=Object.keys(e).filter(o=>r.indexOf(o)!==-1),i={...e,...t};Object.prototype.hasOwnProperty.call(i,"__proto__")&&delete i.__proto__;for(const o of n){if(o==="__proto__")continue;const a=rl(e[o],t[o]);if(!a.valid)return{valid:!1,mergeErrorPath:[o,...a.mergeErrorPath]};i[o]=a.data}return{valid:!0,data:i}}if(Array.isArray(e)&&Array.isArray(t)){if(e.length!==t.length)return{valid:!1,mergeErrorPath:[]};const r=[];for(let n=0;n<e.length;n++){const i=e[n],o=t[n],a=rl(i,o);if(!a.valid)return{valid:!1,mergeErrorPath:[n,...a.mergeErrorPath]};r.push(a.data)}return{valid:!0,data:r}}return{valid:!1,mergeErrorPath:[]}}function fu(e,t,r){const n=new Map;let i;const o=new Map,a=(c,u)=>{let d;if(c.code==="unrecognized_keys"&&!c.path?.length)i??(i=c),d=c.keys;else if(c.code==="invalid_key"&&c.origin==="record"&&c.path?.length===1){const f=String(c.path[0]);o.has(f)||o.set(f,c),d=[f]}else return!1;for(const f of d)n.has(f)||n.set(f,{}),n.get(f)[u]=!0;return!0};for(const c of t.issues)a(c,"l")||e.issues.push(c);for(const c of r.issues)a(c,"r")||e.issues.push(c);const s=[...n].filter(([,c])=>c.l&&c.r).map(([c])=>c);if(s.length){const c=i?s.filter(u=>i.keys.includes(u)):[];c.length&&e.issues.push({...i,keys:c});for(const u of s)!c.includes(u)&&o.has(u)&&e.issues.push(o.get(u))}const l=rl(t.value,r.value);if(!l.valid){if(gt(e))return e;throw new Error(`Unmergable intersection. Error path: ${JSON.stringify(l.mergeErrorPath)}`)}return e.value=l.data,e}const MI=_("$ZodTuple",(e,t)=>{de.init(e,t);const r=t.items,n=st.memoizer;n?.attach(e),e._zod.parse=(i,o)=>{const a=i.value;if(!Array.isArray(a))return i.issues.push({input:a,inst:e,expected:"tuple",code:"invalid_type"}),i;i.value=n?n.alloc(e,i,[],o):[];const s=[],l=pu(r,"optin"),c=pu(r,"optout");if(!t.rest){if(a.length<l)return i.issues.push({code:"too_small",minimum:l,inclusive:!0,input:a,inst:e,origin:"array"}),i;a.length>r.length&&i.issues.push({code:"too_big",maximum:r.length,inclusive:!0,input:a,inst:e,origin:"array"})}const u=new Array(r.length),d=t.rest?o?.abortEarly:void 0;let f=!1;for(let p=0;p<r.length;p++){const m=r[p]._zod.run({value:a[p],issues:[]},o);m instanceof Promise?s.push(m.then(h=>{u[p]=h})):(u[p]=m,d&&!f&&m.issues.length&&(f=gt(m)))}if(t.rest&&!f){let p=r.length-1;const m=a.slice(r.length);let h=i.issues.length;for(const g of m){if(d&&i.issues.length!==h){if(gt(i,h))break;h=i.issues.length}p++;const v=t.rest._zod.run({value:g,issues:[]},o);v instanceof Promise?s.push(v.then(x=>mu(x,i,p))):mu(v,i,p)}}return s.length?Promise.all(s).then(()=>hu(u,i,r,a,c)):hu(u,i,r,a,c)}});function pu(e,t){for(let r=e.length-1;r>=0;r--)if(!(t==="optin"?e[r]._zod.optin!==void 0:e[r]._zod.optout==="optional"))return r+1;return 0}function mu(e,t,r){e.issues.length&&t.issues.push(...ir(r,e.issues)),t.value[r]=e.value}function hu(e,t,r,n,i){for(let o=0;o<r.length;o++){const a=e[o],s=o<n.length;if(!s&&o>=i&&r[o]._zod.optin==="optional"){t.value.length=o;break}if(a.issues.length){if(!s&&o>=i){t.value.length=o;break}t.issues.push(...ir(o,a.issues))}t.value[o]=a.value}for(let o=t.value.length-1;o>=n.length&&(r[o]._zod.optout==="optional"&&t.value[o]===void 0);o--)t.value.length=o;return t}const _I=_("$ZodRecord",(e,t)=>{de.init(e,t);const r=st.memoizer;r?.attach(e),e._zod.parse=(n,i)=>{const o=n.value;if(!Sn(o))return n.issues.push({expected:"record",code:"invalid_type",input:o,inst:e}),n;const a=[],s=t.keyType._zod.values;if(s&&!t.partial){n.value=r?r.alloc(e,n,{},i):{};const l=new Set;for(const u of s)if(typeof u=="string"||typeof u=="number"||typeof u=="symbol"){if(l.add(typeof u=="number"?u.toString():u),u==="__proto__")continue;const d=t.keyType._zod.run({value:u,issues:[]},i);if(d instanceof Promise)throw new Error("Async schemas not supported in object keys currently");if(d.issues.length){n.issues.push({code:"invalid_key",origin:"record",issues:d.issues.map(m=>fr(m,i,Ft())),input:u,path:[u],inst:e});continue}const f=d.value;if(f==="__proto__")continue;const p=t.valueType._zod.run({value:o[u],issues:[]},i);p instanceof Promise?a.push(p.then(m=>{m.issues.length&&n.issues.push(...ir(u,m.issues)),n.value[f]=m.value})):(p.issues.length&&n.issues.push(...ir(u,p.issues)),n.value[f]=p.value)}let c;for(const u in o)if(!l.has(u))if(t.mode==="loose"){if(u==="__proto__")continue;n.value[u]=o[u]}else c=c??[],c.push(u);c&&c.length>0&&n.issues.push({code:"unrecognized_keys",input:o,inst:e,keys:c,continue:!0})}else{n.value=r?r.alloc(e,n,{},i):{};let l;for(const c of Reflect.ownKeys(o)){if(c==="__proto__"||!Object.prototype.propertyIsEnumerable.call(o,c))continue;let u=t.keyType._zod.run({value:c,issues:[]},i);if(u instanceof Promise)throw new Error("Async schemas not supported in object keys currently");if(typeof c=="string"&&Jl.test(c)&&u.issues.length){const m=t.keyType._zod.run({value:Number(c),issues:[]},i);if(m instanceof Promise)throw new Error("Async schemas not supported in object keys currently");m.issues.length===0&&(u=m)}if(u.issues.length){t.mode==="loose"?n.value[c]=o[c]:s?(l=l??[],l.push(c)):n.issues.push({code:"invalid_key",origin:"record",issues:u.issues.map(m=>fr(m,i,Ft())),input:c,path:[c],inst:e});continue}const f=u.value;if(f==="__proto__")continue;const p=t.valueType._zod.run({value:o[c],issues:[]},i);p instanceof Promise?a.push(p.then(m=>{m.issues.length&&n.issues.push(...ir(c,m.issues)),n.value[f]=m.value})):(p.issues.length&&n.issues.push(...ir(c,p.issues)),n.value[f]=p.value)}l&&l.length>0&&n.issues.push({code:"unrecognized_keys",input:o,inst:e,keys:l,continue:!0})}return a.length?Promise.all(a).then(()=>n):n}}),FI=_("$ZodEnum",(e,t)=>{de.init(e,t);const r=Qs(t.entries),n=new Set(r);e._zod.values=n,X(e,"pattern",i=>{const o=Qs(i.def.entries).filter(a=>ob.has(typeof a));return new RegExp(o.length?`^(${o.map(a=>In(a.toString())).join("|")})$`:"^[^\\s\\S]$")}),e._zod.parse=(i,o)=>{const a=i.value;return n.has(a)||i.issues.push({code:"invalid_value",values:r,input:a,inst:e}),i}}),AI=_("$ZodLiteral",(e,t)=>{de.init(e,t);const r=new Set(t.values);e._zod.values=r,X(e,"pattern",n=>{const i=n.def.values;return new RegExp(i.length?`^(${i.map(o=>typeof o=="string"?In(o):o?In(o.toString()):String(o)).join("|")})$`:"^[^\\s\\S]$")}),e._zod.parse=(n,i)=>{const o=n.value;return r.has(o)||n.issues.push({code:"invalid_value",values:t.values,input:o,inst:e}),n}}),OI=_("$ZodTransform",(e,t)=>{de.init(e,t),e._zod.optin="optional",st.memoizer?.guard(e),e._zod.parse=(r,n)=>{if(n.direction==="backward")throw new hm(e.constructor.name);const i=t.transform(r.value,r);if(n.async)return(i instanceof Promise?i:Promise.resolve(i)).then(a=>(r.value=a,r));if(i instanceof Promise)throw new Dr;return r.value=i,r}});function gu(e,t){return e.value=t.issues.length?void 0:t.value,e}const Fm=_("$ZodOptional",(e,t)=>{de.init(e,t),X(e,"optin",r=>r.def.innerType._zod.optin==="defaulted"?"defaulted":"optional"),e._zod.optout="optional",X(e,"values",r=>{const n=r.def.innerType._zod.values;return n?new Set([...n,void 0]):void 0}),X(e,"pattern",r=>{const n=r.def.innerType._zod.pattern;return n?new RegExp(`^(${Kl(n.source)})?$`):void 0}),e._zod.parse=(r,n)=>{if(r.value===void 0){if(t.innerType._zod.optin!=="defaulted")return r;const i=t.innerType._zod.run({value:r.value,issues:[]},n);return i instanceof Promise?i.then(o=>gu(r,o)):gu(r,i)}return t.innerType._zod.run(r,n)}}),zI=_("$ZodExactOptional",(e,t)=>{Fm.init(e,t),X(e,"values",r=>r.def.innerType._zod.values),X(e,"pattern",r=>r.def.innerType._zod.pattern),e._zod.parse=(r,n)=>t.innerType._zod.run(r,n)}),DI=_("$ZodNullable",(e,t)=>{de.init(e,t),X(e,"optin",r=>r.def.innerType._zod.optin),X(e,"optout",r=>r.def.innerType._zod.optout),X(e,"pattern",r=>{const n=r.def.innerType._zod.pattern;return n?new RegExp(`^(${Kl(n.source)}|null)$`):void 0}),X(e,"values",r=>r.def.innerType._zod.values?new Set([...r.def.innerType._zod.values,null]):void 0),e._zod.parse=(r,n)=>r.value===null?r:t.innerType._zod.run(r,n)}),RI=_("$ZodDefault",(e,t)=>{de.init(e,t),e._zod.optin="defaulted",X(e,"values",r=>r.def.innerType._zod.values),e._zod.parse=(r,n)=>{if(n.direction==="backward")return t.innerType._zod.run(r,n);if(r.value===void 0)return r.value=t.defaultValue,r;const i=t.innerType._zod.run(r,n);return i instanceof Promise?i.then(o=>yu(o,t)):yu(i,t)}});function yu(e,t){return e.value===void 0&&(e.value=t.defaultValue),e}const LI=_("$ZodPrefault",(e,t)=>{de.init(e,t),e._zod.optin="defaulted",X(e,"values",r=>r.def.innerType._zod.values),e._zod.parse=(r,n)=>(n.direction==="backward"||r.value===void 0&&(r.value=t.defaultValue),t.innerType._zod.run(r,n))}),BI=_("$ZodNonOptional",(e,t)=>{de.init(e,t),X(e,"values",r=>{const n=r.def.innerType._zod.values;return n?new Set([...n].filter(i=>i!==void 0)):void 0}),e._zod.parse=(r,n)=>{const i=t.innerType._zod.run(r,n);return i instanceof Promise?i.then(o=>vu(o,e)):vu(i,e)}});function vu(e,t){return!e.issues.length&&e.value===void 0&&e.issues.push({code:"invalid_type",expected:"nonoptional",input:e.value,inst:t}),e}function bu(e,t,r,n){return t.issues.length?(e.value=r.catchValue({...t,value:e.value,error:{issues:t.issues.map(i=>fr(i,n,Ft()))},input:e.value}),e):(e.value=t.value,t.memo&&(e.memo=!0),e)}const UI=_("$ZodCatch",(e,t)=>{de.init(e,t),X(e,"optin",r=>r.def.innerType._zod.optin==="defaulted"?"defaulted":"optional"),X(e,"optout",r=>r.def.innerType._zod.optout),X(e,"values",r=>r.def.innerType._zod.values),e._zod.parse=(r,n)=>{if(n.direction==="backward")return t.innerType._zod.run(r,n);const i=t.innerType._zod.run({value:r.value,issues:[]},n);return i instanceof Promise?i.then(o=>bu(r,o,t,n)):bu(r,i,t,n)}}),NI=_("$ZodPipe",(e,t)=>{de.init(e,t),X(e,"values",r=>r.def.in._zod.values),X(e,"optin",r=>r.def.in._zod.optin),X(e,"optout",r=>r.def.out._zod.optout),X(e,"propValues",r=>r.def.in._zod.propValues),e._zod.parse=(r,n)=>{if(n.direction==="backward"){const o=t.out._zod.run(r,n);return o instanceof Promise?o.then(a=>Xi(a,t.in,n)):Xi(o,t.in,n)}const i=t.in._zod.run(r,n);return i instanceof Promise?i.then(o=>Xi(o,t.out,n)):Xi(i,t.out,n)}});function Xi(e,t,r){return e.issues.some(n=>n.code!=="unrecognized_keys")?(e.aborted=!0,e):t._zod.run({value:e.value,issues:e.issues},r)}const $I=_("$ZodReadonly",(e,t)=>{de.init(e,t),X(e,"propValues",r=>r.def.innerType._zod.propValues),X(e,"values",r=>r.def.innerType._zod.values),X(e,"optin",r=>r.def.innerType?._zod?.optin),X(e,"optout",r=>r.def.innerType?._zod?.optout),e._zod.parse=(r,n)=>{if(n.direction==="backward")return t.innerType._zod.run(r,n);const i=t.innerType._zod.run(r,n);return i instanceof Promise?i.then(Su):Su(i)}});function Su(e){return e.memo||(e.value=Object.freeze(e.value)),e}const HI=_("$ZodLazy",(e,t)=>{de.init(e,t),eb(e._zod,"innerType",()=>{const r=t;return r._cachedInner||(r._cachedInner=t.getter()),r._cachedInner}),X(e,"pattern",r=>r.innerType?._zod?.pattern),X(e,"propValues",r=>r.innerType?._zod?.propValues),X(e,"optin",r=>r.innerType?._zod?.optin??void 0),X(e,"optout",r=>r.innerType?._zod?.optout??void 0),e._zod.parse=(r,n)=>e._zod.innerType._zod.run(r,n)}),jI=_("$ZodCustom",(e,t)=>{Ve.init(e,t),de.init(e,t),e._zod.parse=(r,n)=>r,e._zod.check=r=>{const n=r.value,i=t.fn(n);if(i instanceof Promise)return i.then(o=>Iu(o,r,n,e));Iu(i,r,n,e)}});function Iu(e,t,r,n){if(!e){const i={code:"custom",input:r,inst:n,path:[...n._zod.def.path??[]],continue:!n._zod.def.abort};n._zod.def.params&&(i.params=n._zod.def.params),t.issues.push(Ii(i))}}class qI extends Error{constructor(){super("Cannot parse a reference cycle that closes through a transform"),this.name="ZodCyclicError"}}const nl="~memo",xu=[];function Am(e){return e!==null&&typeof e=="object"}function Za(e){return e.map(t=>t.path?{...t,path:t.path.slice()}:{...t})}const Om=new WeakMap,Wn=0,Oo=1,li=2;function zo(e,t,r){const n=Om.get(e);if(n!==void 0)return n?li:Wn;if(t.has(e))return li;t.add(e);let i=Wn;const o=u=>{if(i!==li&&u?._zod){const d=zo(u,t);d>i&&(i=d)}},a=(u,d)=>{let f=Wn;for(const p of Reflect.ownKeys(u)){const m=Object.getOwnPropertyDescriptor(u,p);if(!m.enumerable)continue;const h=m.get?Oo:m.value?._zod?zo(m.value,t):Wn;h>f&&(f=h)}return f},s=u=>{u>i&&(i=u)},l=e._zod.def;switch(l.type){case"object":{const u=Gl(l);s(u?a(u):Oo),o(l.catchall);break}case"array":o(l.element);break;case"tuple":for(const u of l.items)o(u);o(l.rest);break;case"record":case"map":o(l.keyType),o(l.valueType);break;case"set":o(l.valueType);break;case"union":for(const u of l.options)o(u);break;case"intersection":o(l.left),o(l.right);break;case"optional":case"nullable":case"default":case"prefault":case"catch":case"readonly":case"nonoptional":case"promise":case"success":o(l.innerType);break;case"pipe":o(l.in),o(l.out);break;case"function":o(l.input),o(l.output);break;case"lazy":{const u=l._cachedInner??void 0;s(u?zo(u,t):Oo);break}case"template_literal":case"string":case"number":case"int":case"boolean":case"bigint":case"symbol":case"undefined":case"null":case"void":case"never":case"any":case"unknown":case"date":case"nan":case"enum":case"literal":case"file":case"transform":case"custom":break;default:for(const u in l){const d=Object.getOwnPropertyDescriptor(l,u);if(!d||d.get)continue;const f=d.value;if(!(!f||typeof f!="object")){if(f._zod)o(f);else if(Array.isArray(f))for(const p of f)o(p)}}}return t.delete(e),KI(e,i)}function KI(e,t){return t!==Oo&&Om.set(e,t===li),t}function GI(e,t){let r=e.buckets.get(t);return r||(r=new WeakMap,e.buckets.set(t,r)),r}let Qi;const Ji=[],WI={alloc(e,t,r){const n=Qi;if(!n)return r;Qi=void 0;const i={value:r,issues:null};return n.set(t.value,i),Ji.push(i),r},guard(e){var t;(t=e._zod).deferred??(t.deferred=[]),e._zod.deferred.push(()=>{const r=e._zod.parse,n=(i,o)=>{if(o.direction!=="backward"&&ZI(o,i.value))throw new qI;return r(i,o)};e._zod.parse=n,e._zod.run===r&&(e._zod.run=n)})},attach(e){var t;let r,n=!1,i,o;(t=e._zod).deferred??(t.deferred=[]),e._zod.deferred.push(()=>{const a=e._zod.parse,s=(l,c)=>{if(r===void 0){const v=zo(e,new Set);if(v===Wn)return e._zod.parse=a,e._zod.run===s&&(e._zod.run=a),a(l,c);v===li||n?r=!0:n=!0}const u=l.value;if(!Am(u))return a(l,c);let d=c[nl];d||(d={buckets:new WeakMap,backEdges:void 0},c[nl]=d);let f;i===c?f=o:(f=GI(d,e),i=c,o=f);const p=f.get(u);if(p)return l.value=p.value,p.issues?p.issues.length&&l.issues.push(...Za(p.issues)):(l.memo=!0,d.backEdges??(d.backEdges=new WeakSet),d.backEdges.add(p.value)),l;Qi=f;const m=Ji.length,h=a(l,c);Qi=void 0;const g=Ji.length>m?Ji.pop():void 0;return h instanceof Promise?h.then(v=>(g&&(g.issues=v.issues.length?Za(v.issues):xu),v)):(g&&(g.issues=h.issues.length?Za(h.issues):xu),h)};e._zod.parse=s,e._zod.run===a&&(e._zod.run=s)})}};function VI(){return WI}function ZI(e,t){const r=e[nl]?.backEdges;return r!==void 0&&Am(t)&&r.has(t)}const YI=()=>{const e={string:{unit:"characters",verb:"to have"},file:{unit:"bytes",verb:"to have"},array:{unit:"items",verb:"to have"},set:{unit:"items",verb:"to have"},map:{unit:"entries",verb:"to have"}};function t(o){return e[o]??null}const r={regex:"input",email:"email address",url:"URL",emoji:"emoji",uuid:"UUID",uuidv4:"UUIDv4",uuidv6:"UUIDv6",nanoid:"nanoid",guid:"GUID",cuid:"cuid",cuid2:"cuid2",ulid:"ULID",xid:"XID",ksuid:"KSUID",datetime:"ISO datetime",date:"ISO date",time:"ISO time",duration:"ISO duration",ipv4:"IPv4 address",ipv6:"IPv6 address",mac:"MAC address",cidrv4:"IPv4 range",cidrv6:"IPv6 range",base64:"base64-encoded string",base64url:"base64url-encoded string",json_string:"JSON string",e164:"E.164 number",currency_code:"currency code",credit_card:"credit card number",iban:"IBAN",jwt:"JWT",template_literal:"input"},n={nan:"NaN"};function i(o,a){return o==="number"&&typeof a=="number"&&!Number.isFinite(a)?String(a):n[o]??o}return o=>{switch(o.code){case"invalid_type":{const a=i(o.expected),s=gb(o.input),l=i(s,o.input);return`Invalid input: expected ${a}, received ${l}`}case"invalid_value":return o.values.length===1?`Invalid input: expected ${um(o.values[0])}`:`Invalid option: expected one of ${tu(o.values,"|")}`;case"too_big":{const a=o.exact?"exactly ":o.inclusive?"<=":"<",s=t(o.origin);return s?`Too big: expected ${o.origin??"value"} to have ${a}${o.maximum.toString()} ${s.unit??"elements"}`:`Too big: expected ${o.origin??"value"} to be ${a}${o.maximum.toString()}`}case"too_small":{const a=o.exact?"exactly ":o.inclusive?">=":">",s=t(o.origin);return s?`Too small: expected ${o.origin} to have ${a}${o.minimum.toString()} ${s.unit}`:`Too small: expected ${o.origin} to be ${a}${o.minimum.toString()}`}case"invalid_format":{const a=o;return a.format==="starts_with"?`Invalid string: must start with "${a.prefix}"`:a.format==="ends_with"?`Invalid string: must end with "${a.suffix}"`:a.format==="includes"?`Invalid string: must include "${a.includes}"`:a.format==="regex"?`Invalid string: must match pattern ${a.pattern}`:`Invalid ${r[a.format]??o.format}`}case"not_multiple_of":return`Invalid number: must be a multiple of ${o.divisor}`;case"unrecognized_keys":return`Unrecognized key${o.keys.length>1?"s":""}: ${tu(o.keys,", ")}`;case"invalid_key":return`Invalid key in ${o.origin}`;case"invalid_union":return o.options&&Array.isArray(o.options)&&o.options.length>0?`Invalid discriminator value. Expected ${o.options.map(s=>`'${s}'`).join(" | ")}`:o.inclusive===!1?"Invalid input: more than one option matched":"Invalid input";case"invalid_element":return`Invalid value in ${o.origin}`;default:return"Invalid input"}}};function XI(){return{localeError:YI()}}var wu;class QI{constructor(){this._map=new WeakMap,this._idmap=new Map}add(t,...r){const n=r[0];return this._map.set(t,n),n&&typeof n=="object"&&"id"in n&&this._idmap.set(n.id,t),this}clear(){return this._map=new WeakMap,this._idmap=new Map,this}remove(t){const r=this._map.get(t);return r&&typeof r=="object"&&"id"in r&&this._idmap.delete(r.id),this._map.delete(t),this}get(t){const r=t._zod.parent;if(r){const n={...this.get(r)??{}};delete n.id;const i={...n,...this._map.get(t)};return Object.keys(i).length?i:void 0}return this._map.get(t)}has(t){return this._map.has(t)}}function JI(){return new QI}(wu=globalThis).__zod_globalRegistry??(wu.__zod_globalRegistry=JI());const Vn=globalThis.__zod_globalRegistry;function zm(e){return e.checks&&(e.checks=[...e.checks]),e}function ex(e,t){return new e(zm({type:"string",...N(t)}))}function tx(e,t){return new e({type:"string",format:"email",check:"string_format",abort:!1,...N(t)})}function rx(e,t){return new e({type:"string",format:"guid",check:"string_format",abort:!1,...N(t)})}function nx(e,t){return new e({type:"string",format:"uuid",check:"string_format",abort:!1,...N(t)})}function ix(e,t){return new e({type:"string",format:"uuid",check:"string_format",abort:!1,version:"v4",...N(t)})}function ox(e,t){return new e({type:"string",format:"uuid",check:"string_format",abort:!1,version:"v6",...N(t)})}function ax(e,t){return new e({type:"string",format:"uuid",check:"string_format",abort:!1,version:"v7",...N(t)})}function sx(e,t){return new e({type:"string",format:"url",check:"string_format",abort:!1,...N(t)})}function lx(e,t){return new e({type:"string",format:"emoji",check:"string_format",abort:!1,...N(t)})}function cx(e,t){return new e({type:"string",format:"nanoid",check:"string_format",abort:!1,...N(t)})}function ux(e,t){return new e({type:"string",format:"cuid",check:"string_format",abort:!1,...N(t)})}function dx(e,t){return new e({type:"string",format:"cuid2",check:"string_format",abort:!1,...N(t)})}function fx(e,t){return new e({type:"string",format:"ulid",check:"string_format",abort:!1,...N(t)})}function px(e,t){return new e({type:"string",format:"xid",check:"string_format",abort:!1,...N(t)})}function mx(e,t){return new e({type:"string",format:"ksuid",check:"string_format",abort:!1,...N(t)})}function hx(e,t){return new e({type:"string",format:"ipv4",check:"string_format",abort:!1,...N(t)})}function gx(e,t){return new e({type:"string",format:"ipv6",check:"string_format",abort:!1,...N(t)})}function yx(e,t){return new e({type:"string",format:"cidrv4",check:"string_format",abort:!1,...N(t)})}function vx(e,t){return new e({type:"string",format:"cidrv6",check:"string_format",abort:!1,...N(t)})}function bx(e,t){return new e({type:"string",format:"base64",check:"string_format",abort:!1,...N(t)})}function Sx(e,t){return new e({type:"string",format:"base64url",check:"string_format",abort:!1,...N(t)})}function Ix(e,t){return new e({type:"string",format:"e164",check:"string_format",abort:!1,...N(t)})}function xx(e,t){return new e({type:"string",format:"jwt",check:"string_format",abort:!1,...N(t)})}function wx(e,t){return new e({type:"string",format:"datetime",check:"string_format",offset:!1,local:!1,precision:null,...N(t)})}function kx(e,t){return new e({type:"string",format:"date",check:"string_format",...N(t)})}function Px(e,t){return new e({type:"string",format:"time",check:"string_format",precision:null,...N(t)})}function Tx(e,t){return new e({type:"string",format:"duration",check:"string_format",...N(t)})}function Ex(e,t){return new e(zm({type:"number",checks:[],...N(t)}))}function Cx(e,t){return new e({type:"number",check:"number_format",abort:!1,format:"safeint",...N(t)})}function Mx(e,t){return new e({type:"boolean",...N(t)})}function _x(e,t){return new e({type:"null",...N(t)})}function Fx(e){return new e({type:"unknown"})}function Ax(e,t){return new e({type:"never",...N(t)})}function ku(e,t){return new vm({check:"less_than",...N(t),value:e,inclusive:!1})}function Ya(e,t){return new vm({check:"less_than",...N(t),value:e,inclusive:!0})}function Pu(e,t){return new bm({check:"greater_than",...N(t),value:e,inclusive:!1})}function Xa(e,t){return new bm({check:"greater_than",...N(t),value:e,inclusive:!0})}function Tu(e,t){return new xS({check:"multiple_of",...N(t),value:e})}function Dm(e,t){return new kS({check:"max_length",...N(t),maximum:e})}function ta(e,t){return new PS({check:"min_length",...N(t),minimum:e})}function Rm(e,t){return new TS({check:"length_equals",...N(t),length:e})}function Ox(e,t){return new ES({check:"string_format",format:"regex",...N(t),pattern:e})}function zx(e){return new CS({check:"string_format",format:"lowercase",...N(e)})}function Dx(e){return new MS({check:"string_format",format:"uppercase",...N(e)})}function Rx(e,t){return new _S({check:"string_format",format:"includes",...N(t),includes:e})}function Lx(e,t){return new FS({check:"string_format",format:"starts_with",...N(t),prefix:e})}function Bx(e,t){return new AS({check:"string_format",format:"ends_with",...N(t),suffix:e})}function Mn(e){return new OS({check:"overwrite",tx:e})}function Ux(e){return Mn(t=>t.normalize(e))}function Nx(){return Mn(e=>e.trim())}function $x(){return Mn(e=>e.toLowerCase())}function Hx(){return Mn(e=>e.toUpperCase())}function jx(){return Mn(e=>nb(e))}function qx(e,t,r){return new e({type:"array",element:t,...N(r)})}function Kx(e,t,r){const n=N(r);return n.abort??(n.abort=!0),new e({type:"custom",check:"custom",fn:t,...n})}function Gx(e,t,r){return new e({type:"custom",check:"custom",fn:t,...N(r)})}function Wx(e,t){const r=Vx(n=>(n.addIssue=i=>{if(typeof i=="string")n.issues.push(Ii(i,n.value,r._zod.def));else{const o=i;o.fatal&&(o.continue=!1),o.code??(o.code="custom"),"input"in o||(o.input=n.value),o.inst??(o.inst=r),o.continue??(o.continue=!r._zod.def.abort),n.issues.push(Ii(o))}},e(n.value,n)),t);return r}function Vx(e,t){const r=new Ve({check:"custom",...N(t)});return r._zod.check=e,r}function ci(e,...t){for(const r of t)for(const n of Reflect.ownKeys(r))Object.prototype.propertyIsEnumerable.call(r,n)&&Zt(e,n,r[n]);return e}function Lm(e){let t=e?.target??"draft-2020-12";return t==="draft-4"&&(t="draft-04"),t==="draft-7"&&(t="draft-07"),{processors:e.processors??{},metadataRegistry:e?.metadata??Vn,target:t,unrepresentable:e?.unrepresentable??"throw",override:e?.override??(()=>{}),io:e?.io??"output",counter:0,seen:new Map,sharedDefsExtractedFor:void 0,sharedEmitDoneFor:void 0,cycles:e?.cycles??"ref",reused:e?.reused??"inline",intersections:[],deferred:[],external:e?.external??void 0}}function mr(e,t,r,n,i){const o=typeof t.unrepresentable=="function"?t.unrepresentable({zodSchema:e,path:n.path,message:i}):t.unrepresentable;if(o==="any")return!1;if(o===void 0||o==="throw")throw new Error(i);return Object.assign(r,o),!0}function Se(e,t,r={path:[],schemaPath:[]}){var n;const i=e._zod.def,o=t.seen.get(e);if(o)return o.count++,r.schemaPath.includes(e)&&(o.cycle=r.path),o.schema;const a={schema:{},count:1,cycle:void 0,path:r.path};t.seen.set(e,a),t.sharedDefsExtractedFor=void 0,t.sharedEmitDoneFor=void 0;const s=e._zod.toJSONSchema?.();if(s)a.schema=s;else{const u={...r,schemaPath:[...r.schemaPath,e],path:r.path};if(e._zod.processJSONSchema)e._zod.processJSONSchema(t,a.schema,u);else{const f=a.schema,p=t.processors[i.type];if(!p)throw new Error(`[toJSONSchema]: Non-representable type encountered: ${i.type}`);p(e,t,f,u)}const d=e._zod.parent;d&&(a.ref||(a.ref=d),Se(d,t,u),t.seen.get(d).isParent=!0)}const l=t.metadataRegistry.get(e);return l&&ci(a.schema,l),t.io==="input"&&Ue(e)&&(delete a.schema.examples,delete a.schema.default),t.io==="input"&&"_prefault"in a.schema&&((n=a.schema).default??(n.default=a.schema._prefault)),delete a.schema._prefault,t.seen.get(e).schema}function Eu(e){return e.replace(/~/g,"~0").replace(/\//g,"~1")}function Bm(e,t){const r=e.seen.get(t);if(!r)throw new Error("Unprocessed schema. This is a bug in Zod.");if(e.external&&e.sharedDefsExtractedFor===e.external)return;const n=new Map;for(const a of e.seen.entries()){const s=e.metadataRegistry.get(a[0])?.id;if(s){const l=n.get(s);if(l&&l!==a[0])throw new Error(`Duplicate schema id "${s}" detected during JSON Schema conversion. Two different schemas cannot share the same id when converted together.`);n.set(s,a[0])}}const i=a=>{const s=e.target==="draft-2020-12"?"$defs":"definitions";if(e.external){const d=e.external.registry.get(a[0])?.id,f=e.external.uri??(m=>m);if(d)return{ref:f(d)};const p=a[1].defId??a[1].schema.id??`schema${e.counter++}`;return a[1].defId=p,{defId:p,ref:`${f("__shared")}#/${s}/${Eu(p)}`}}const l="#",c=`${l}/${s}/`;if(a[1]===r&&!a[1].schema.id)return{ref:l};const u=a[1].schema.id??`__schema${e.counter++}`;return{defId:u,ref:c+Eu(u)}},o=a=>{if(a[1].schema.$ref)return;const s=a[1],{ref:l,defId:c}=i(a);s.def={...s.schema},c&&(s.defId=c);const u=s.schema;for(const d in u)delete u[d];u.$ref=l};if(e.cycles==="throw")for(const a of e.seen.entries()){const s=a[1];if(s.cycle)throw new Error(`Cycle detected: #/${s.cycle?.join("/")}/<root>

Set the \`cycles\` parameter to \`"ref"\` to resolve cyclical schemas with defs.`)}for(const a of e.seen.entries()){const s=a[1];if(t===a[0]){o(a);continue}if(e.external){const c=e.external.registry.get(a[0])?.id;if(t!==a[0]&&c){o(a);continue}}if(e.metadataRegistry.get(a[0])?.id){o(a);continue}if(s.cycle){o(a);continue}s.count>1&&e.reused==="ref"&&o(a)}e.external&&(e.sharedDefsExtractedFor=e.external)}function Um(e){const t=e.anyOf;if(!Array.isArray(t)||t.length===0||e.type!==void 0)return;const r=[];for(const n of t){if(!n||typeof n!="object")return;Um(n);const i=Object.keys(n);if(i.length!==1||i[0]!=="type")return;const o=n.type;for(const a of Array.isArray(o)?o:[o]){if(typeof a!="string")return;r.includes(a)||r.push(a)}}delete e.anyOf,e.type=r.length===1?r[0]:r}const Nm=new Set(["type","properties","required","additionalProperties"]),Cu=["oneOf","anyOf"];function Mu(e){const t=e.additionalProperties;return t===void 0||t===!1||typeof t!="object"||t===null?null:Object.keys(t).length?t:null}function il(e){const t=[];for(const o of e){if(typeof o!="object"||o.type!=="object")return null;for(const a in o)if(!Nm.has(a))return null;t.push(o)}const r={},n=new Set;for(const o of t){for(const a in o.properties){if(Object.prototype.hasOwnProperty.call(r,a))continue;const s=[];for(const c of t){const u=c.properties?.[a]??Mu(c);u!=null&&(s.some(d=>JSON.stringify(d)===JSON.stringify(u))||s.push(u))}const l=s.length===1?s[0]:il(s)??{allOf:s};Zt(r,a,l)}for(const a of o.required??[])n.add(a)}const i={type:"object",properties:r};if(n.size&&(i.required=[...n]),t.every(o=>o.additionalProperties===!1))i.additionalProperties=!1;else{const o=[];for(const a of t){const s=Mu(a);s&&!o.some(l=>JSON.stringify(l)===JSON.stringify(s))&&o.push(s)}o.length===1?i.additionalProperties=o[0]:o.length>1&&(i.additionalProperties={allOf:o})}return i}function Zx(e){const t=e.allOf;if(!Array.isArray(t)||t.length<2)return;for(const i of Nm)if(i in e)return;const r=t.filter(i=>Cu.some(o=>Array.isArray(i[o])));let n=null;if(!r.length)n=il(t);else{const i=r[0],o=Cu.find(l=>Array.isArray(i[l]));if(Object.keys(i).length!==1)return;const a=t.filter(l=>l!==i),s=i[o].map(l=>il([...a,l]));if(s.some(l=>!l))return;n={[o]:s}}n&&(delete e.allOf,ci(e,n))}function $m(e,t){const r=e.seen.get(t);if(!r)throw new Error("Unprocessed schema. This is a bug in Zod.");const n=s=>{const l=e.seen.get(s);if(l.ref===null)return;const c=l.def??l.schema,u={...c},d=l.ref;if(l.ref=null,d){n(d);const p=e.seen.get(d),m=p.schema;if(m.$ref&&(e.target==="draft-07"||e.target==="draft-04"||e.target==="openapi-3.0")?(c.allOf=c.allOf??[],c.allOf.push(m)):ci(c,m),ci(c,u),s._zod.parent===d)for(const g in c)g==="$ref"||g==="allOf"||g in u||delete c[g];if(m.$ref&&p.def)for(const g in c)g==="$ref"||g==="allOf"||g in p.def&&JSON.stringify(c[g])===JSON.stringify(p.def[g])&&delete c[g]}const f=s._zod.parent;if(f&&f!==d){n(f);const p=e.seen.get(f);if(p?.schema.$ref&&(c.$ref=p.schema.$ref,p.def))for(const m in c)m==="$ref"||m==="allOf"||m in p.def&&JSON.stringify(c[m])===JSON.stringify(p.def[m])&&delete c[m]}e.override({zodSchema:s,jsonSchema:c,path:l.path??[]})};if(!e.external||e.sharedEmitDoneFor!==e.external){for(const s of[...e.seen.entries()].reverse())n(s[0]);if(e.target!=="openapi-3.0")for(const s of e.seen.entries())Um(s[1].def??s[1].schema);for(const s of e.deferred)s();if(e.intersections.length){const s=new Map;for(const l of e.seen.values())for(const c of[l.schema,l.def]){const u=c?.allOf;if(!Array.isArray(u))continue;const d=s.get(u);d?d.push(c):s.set(u,[c])}for(const l of e.intersections)for(const c of s.get(l)??[])Zx(c)}}const i={};if(e.target==="draft-2020-12"?i.$schema="https://json-schema.org/draft/2020-12/schema":e.target==="draft-07"?i.$schema="http://json-schema.org/draft-07/schema#":e.target==="draft-04"?i.$schema="http://json-schema.org/draft-04/schema#":e.target,e.external?.uri){const s=e.external.registry.get(t)?.id;if(!s)throw new Error("Schema is missing an `id` property");i.$id=e.external.uri(s)}ci(i,r.defId?r.schema:r.def??r.schema);const o=e.metadataRegistry.get(t)?.id;o!==void 0&&i.id===o&&delete i.id;const a=e.external?.defs??{};if(!e.external||e.sharedEmitDoneFor!==e.external)for(const s of e.seen.entries()){const l=s[1];l.def&&l.defId&&(l.def.id===l.defId&&delete l.def.id,Zt(a,l.defId,l.def))}e.external&&(e.sharedEmitDoneFor=e.external),e.external||Object.keys(a).length>0&&(e.target==="draft-2020-12"?i.$defs=a:i.definitions=a);try{const s=JSON.parse(JSON.stringify(i));return Object.defineProperty(s,"~standard",{value:{...t["~standard"],jsonSchema:{input:ra(t,"input",e.processors),output:ra(t,"output",e.processors)}},enumerable:!1,writable:!1}),s}catch{throw new Error("Error converting schema to JSON.")}}function Ue(e,t){const r=t??{seen:new Set};if(r.seen.has(e))return!1;r.seen.add(e);const n=e._zod.def;if(n.type==="transform")return!0;if(n.type==="array")return Ue(n.element,r);if(n.type==="set")return Ue(n.valueType,r);if(n.type==="lazy")return Ue(n.getter(),r);if(n.type==="promise"||n.type==="optional"||n.type==="nonoptional"||n.type==="nullable"||n.type==="readonly"||n.type==="default"||n.type==="prefault"||n.type==="catch")return Ue(n.innerType,r);if(n.type==="intersection")return Ue(n.left,r)||Ue(n.right,r);if(n.type==="record"||n.type==="map")return Ue(n.keyType,r)||Ue(n.valueType,r);if(n.type==="pipe")return e._zod.traits.has("$ZodCodec")?!0:Ue(n.in,r)||Ue(n.out,r);if(n.type==="object"){for(const i in n.shape)if(Ue(n.shape[i],r))return!0;return!1}if(n.type==="union"){for(const i of n.options)if(Ue(i,r))return!0;return!1}if(n.type==="tuple"){for(const i of n.items)if(Ue(i,r))return!0;return!!(n.rest&&Ue(n.rest,r))}return!1}const Yx=(e,t={})=>r=>{const n=Lm({...r,processors:t});return Se(e,n),Bm(n,e),$m(n,e)},ra=(e,t,r={})=>n=>{const{libraryOptions:i,target:o}=n??{},a=Lm({...i??{},target:o,io:t,processors:r});return Se(e,a),Bm(a,e),$m(a,e)},xn=(e,t,r)=>{(e[t]===void 0||r>e[t])&&(e[t]=r)},wn=(e,t,r)=>{(e[t]===void 0||r<e[t])&&(e[t]=r)},_u=(e,t)=>{xn(e,"minimum",t),wn(e,"maximum",t)},Hm=(e,t)=>{e.multipleOf??(e.multipleOf=[]),e.multipleOf.includes(t)||e.multipleOf.push(t)},jm=(e,t)=>{e.patterns??(e.patterns=new Set),e.patterns.add(t)},qm=(e,t)=>{e.mime=e.mime?e.mime.filter(r=>t.includes(r)):[...t]},Km=(e,t)=>{e.format=t,t.includes("int")&&(e.isInt=!0)},Fu=(e,t)=>xn(e,"minimum",t.minimum),Au=(e,t)=>wn(e,"maximum",t.maximum),Ou=e=>(t,r)=>{Km(t,r.format);const[n,i]=e[r.format];xn(t,"minimum",n),wn(t,"maximum",i)},Xx={greater_than:(e,t)=>xn(e,t.inclusive?"minimum":"exclusiveMinimum",t.value),less_than:(e,t)=>wn(e,t.inclusive?"maximum":"exclusiveMaximum",t.value),multiple_of:(e,t)=>Hm(e,t.value),number_format:Ou(dm),bigint_format:Ou(sb),min_length:Fu,max_length:Au,length_equals:(e,t)=>_u(e,t.length),min_size:Fu,max_size:Au,size_equals:(e,t)=>_u(e,t.size),string_format:(e,t)=>{Km(e,t.format),t.pattern&&jm(e,t.pattern),(t.format==="base64"||t.format==="base64url")&&(e.contentEncoding=t.format),(t.local||t.precision===-1)&&(e.laxFormat=!0)},mime_type:(e,t)=>qm(e,t.mime)};function it(e){const t={},r=e._zod.def,n=e._zod.traits.has("$ZodCheck")?[e,...r.checks??[]]:r.checks??[];for(const o of n)Xx[o._zod.def.check]?.(t,o._zod.def);const i=e._zod.bag;i.minimum!==void 0&&xn(t,"minimum",i.minimum),i.exclusiveMinimum!==void 0&&xn(t,"exclusiveMinimum",i.exclusiveMinimum),i.maximum!==void 0&&wn(t,"maximum",i.maximum),i.exclusiveMaximum!==void 0&&wn(t,"exclusiveMaximum",i.exclusiveMaximum),i.multipleOf!==void 0&&Hm(t,i.multipleOf),i.format!==void 0&&(t.format??(t.format=i.format),i.format.includes("int")&&(t.isInt=!0)),i.mime&&qm(t,i.mime);for(const o of i.patterns??[])jm(t,o);return t}const Qx={guid:"uuid",url:"uri",datetime:"date-time",json_string:"json-string",regex:""},Jx=new Map([[Tm,lS],[rc,cS]]),Gm=e=>Jx.get(e)??e,ew=(e,t,r,n)=>{const i=r;i.type="string";const{minimum:o,maximum:a,format:s,patterns:l,contentEncoding:c,laxFormat:u}=it(e);if(typeof o=="number"&&(i.minLength=o),typeof a=="number"&&(i.maxLength=a),s&&(i.format=Qx[s]??s,i.format===""&&delete i.format,(s==="time"||u)&&delete i.format),c&&(i.contentEncoding=c),l&&l.size>0){const d=[...l].map(Gm);d.length===1?i.pattern=d[0].source:d.length>1&&(i.allOf=[...d.map(f=>({...t.target==="draft-07"||t.target==="draft-04"||t.target==="openapi-3.0"?{type:"string"}:{},pattern:f.source}))])}},tw=(e,t,r,n)=>{const i=r,{minimum:o,maximum:a,multipleOf:s,exclusiveMaximum:l,exclusiveMinimum:c,isInt:u}=it(e);i.type=u?"integer":"number";const d=typeof c=="number"&&c>=(o??Number.NEGATIVE_INFINITY),f=typeof l=="number"&&l<=(a??Number.POSITIVE_INFINITY),p=t.target==="draft-04"||t.target==="openapi-3.0";if(d?p?(i.minimum=c,i.exclusiveMinimum=!0):i.exclusiveMinimum=c:typeof o=="number"&&(i.minimum=o),f?p?(i.maximum=l,i.exclusiveMaximum=!0):i.exclusiveMaximum=l:typeof a=="number"&&(i.maximum=a),s){const m=new Set;for(const v of s)Number.isFinite(v)&&v!==0?m.add(Math.abs(v)):mr(e,t,i,n,`A multipleOf divisor of ${v} cannot be represented in JSON Schema`);const[h,...g]=m;h!==void 0&&(i.multipleOf=h),g.length&&(i.allOf=[...i.allOf??[],...g.map(v=>({multipleOf:v}))])}},rw=(e,t,r,n)=>{r.type="boolean"},nw=(e,t,r,n)=>{t.target==="openapi-3.0"?(r.type="string",r.nullable=!0,r.enum=[null]):r.type="null"},iw=(e,t,r,n)=>{r.not={}},ow=(e,t,r,n)=>{},aw=(e,t,r,n)=>{const i=e._zod.def,o=Qs(i.entries);if(o.length===0){r.not={};return}o.every(a=>typeof a=="number")&&(r.type="number"),o.every(a=>typeof a=="string")&&(r.type="string"),r.enum=o},sw=(e,t,r,n)=>{const i=e._zod.def;if(i.values.length===0){r.not={};return}const o=[];for(const a of i.values)if(a===void 0){if(mr(e,t,r,n,"Literal `undefined` cannot be represented in JSON Schema"))return}else if(typeof a=="bigint"){if(mr(e,t,r,n,"BigInt literals cannot be represented in JSON Schema"))return;o.push(Number(a))}else o.push(a);if(o.length!==0)if(o.length===1){const a=o[0];r.type=a===null?"null":typeof a,t.target==="draft-04"||t.target==="openapi-3.0"?r.enum=[a]:r.const=a}else o.every(a=>typeof a=="number")&&(r.type="number"),o.every(a=>typeof a=="string")&&(r.type="string"),o.every(a=>typeof a=="boolean")&&(r.type="boolean"),o.every(a=>a===null)&&(r.type="null"),r.enum=o},lw=(e,t,r,n)=>{mr(e,t,r,n,"Custom types cannot be represented in JSON Schema")},cw=(e,t,r,n)=>{mr(e,t,r,n,"Transforms cannot be represented in JSON Schema")},uw=(e,t,r,n)=>{const i=r,o=e._zod.def,{minimum:a,maximum:s}=it(e);typeof a=="number"&&(i.minItems=a),typeof s=="number"&&(i.maxItems=s),i.type="array",i.items=Se(o.element,t,{...n,path:[...n.path,"items"]})};function xi(e){const t=e._zod.def;return t.type==="pipe"&&t.in._zod.traits.has("$ZodTransform")?xi(t.out):t.type==="catch"?xi(t.innerType):e._zod.optin}const dw=(e,t,r,n)=>{const i=r,o=e._zod.def,a=o.shape;if(Object.getOwnPropertySymbols(a).length&&mr(e,t,i,n,"Symbol keys cannot be represented in JSON Schema"))return;i.type="object",i.properties={};for(const c in a)Zt(i.properties,c,Se(a[c],t,{...n,path:[...n.path,"properties",c]}));const l=[];for(const c of Object.keys(a)){const u=o.shape[c];(t.io==="input"?xi(u)===void 0:u._zod.optout===void 0)&&l.push(c)}l.length>0&&(i.required=l),o.catchall?._zod.def.type==="never"?i.additionalProperties=!1:o.catchall?o.catchall&&(i.additionalProperties=Se(o.catchall,t,{...n,path:[...n.path,"additionalProperties"]})):t.io==="output"&&(i.additionalProperties=!1)},fw=(e,t,r,n)=>{const i=e._zod.def,o=i.inclusive===!1,a=i.options.map((s,l)=>Se(s,t,{...n,path:[...n.path,o?"oneOf":"anyOf",l]}));o?r.oneOf=a:r.anyOf=a},pw=(e,t,r,n)=>{const i=e._zod.def,o=Se(i.left,t,{...n,path:[...n.path,"allOf",0]}),a=Se(i.right,t,{...n,path:[...n.path,"allOf",1]}),s=c=>"allOf"in c&&Object.keys(c).length===1,l=[...s(o)?o.allOf:[o],...s(a)?a.allOf:[a]];r.allOf=l,t.intersections.push(l)},mw=(e,t,r,n)=>{const i=r,o=e._zod.def;i.type="array";const a=t.target==="draft-2020-12"?"prefixItems":"items",s=t.target==="draft-2020-12"||t.target==="openapi-3.0"?"items":"additionalItems",l=o.items.map((h,g)=>Se(h,t,{...n,path:[...n.path,a,g]})),c=o.rest?Se(o.rest,t,{...n,path:[...n.path,s,...t.target==="openapi-3.0"?[o.items.length]:[]]}):null;let u=o.items.length;for(;u>0;){const h=o.items[u-1];if(!(t.io==="input"?xi(h)!==void 0:h._zod.optout==="optional"))break;u--}const d=o.items.length,f=!o.rest;t.target==="draft-2020-12"?(i.prefixItems=l,f?i.items=!1:c&&(i.items=c),u>0&&(i.minItems=u),f&&(i.maxItems=d)):t.target==="openapi-3.0"?(i.items={anyOf:l},c&&i.items.anyOf.push(c),u>0&&(i.minItems=u),f&&(i.maxItems=d)):(i.items=l,f?i.additionalItems=!1:c&&(i.additionalItems=c),u>0&&(i.minItems=u),f&&(i.maxItems=d));const{minimum:p,maximum:m}=it(e);typeof p=="number"&&(i.minItems=p),typeof m=="number"&&(i.maxItems=m)};function ol(e,t,r){if(t.$ref){if(r.has(t))return t;r.add(t);const m=e.get(t)?.def;if(!m)return t;const h=ol(e,m,r);return h===m?t:h}for(const m of["anyOf","oneOf"]){const h=t[m];if(!Array.isArray(h))continue;const g=h.map(v=>ol(e,v,r));g.some((v,x)=>v!==h[x])&&(t={...t,[m]:g})}const n=Array.isArray(t.type)?t.type:[t.type],i=!n.includes("string")&&n.some(m=>m==="number"||m==="integer"),o=t.enum??(t.const!==void 0?[t.const]:void 0);if(!i&&!o?.some(m=>typeof m=="number"))return t;const{minimum:a,maximum:s,exclusiveMinimum:l,exclusiveMaximum:c,multipleOf:u,format:d,id:f,...p}=t;return p.enum?p.enum=p.enum.map(m=>typeof m=="number"?String(m):m):typeof p.const=="number"&&(p.const=String(p.const)),i&&(p.type="string",o||(p.pattern=(n.includes("number")?Jl:yS).source)),p}const al=new WeakMap;function hw(e){const t=new Map;for(const n of e.seen.values())n.def&&!t.has(n.schema)&&t.set(n.schema,n);const r=new Map;for(const n of al.get(e)??[]){const i=e.seen.get(n),o=(i?.def??i?.schema)?.propertyNames;if(!o||o===!0||r.has(o))continue;const a=ol(t,o,new Set);a!==o&&r.set(o,a)}if(r.size)for(const n of e.seen.values())for(const i of[n.schema,n.def]){const o=i&&r.get(i.propertyNames);o&&(i.propertyNames=o)}}const gw=(e,t,r,n)=>{const i=r,o=e._zod.def;i.type="object";const a=o.keyType,s=it(a).patterns;if(o.mode==="loose"&&s&&s.size>0){const u=Se(o.valueType,t,{...n,path:[...n.path,"patternProperties","*"]});i.patternProperties={};for(const d of s)Zt(i.patternProperties,Gm(d).source,u)}else{if(t.target==="draft-07"||t.target==="draft-2020-12"){i.propertyNames=Se(o.keyType,t,{...n,path:[...n.path,"propertyNames"]});let u=al.get(t);u||(u=[],al.set(t,u),t.deferred.push(()=>hw(t))),u.push(e)}i.additionalProperties=Se(o.valueType,t,{...n,path:[...n.path,"additionalProperties"]})}const l=a._zod.values,c=t.io==="input"&&xi(o.valueType)!==void 0;if(l&&!o.partial&&!c){const u=[...l].filter(d=>typeof d=="string"||typeof d=="number");u.length>0&&(i.required=u.map(String))}},yw=(e,t,r,n)=>{const i=e._zod.def,o=Se(i.innerType,t,n),a=t.seen.get(e);t.target==="openapi-3.0"?(a.ref=i.innerType,r.nullable=!0):r.anyOf=[o,{type:"null"}]},vw=(e,t,r,n)=>{const i=e._zod.def;Se(i.innerType,t,n);const o=t.seen.get(e);o.ref=i.innerType},nc=Symbol();function Wm(e,t,r,n,i){let o=!1;const a=JSON.stringify(e,(s,l)=>typeof l!="bigint"?l:(o=!0,null));return o?(mr(t,r,n,i,"BigInt defaults cannot be represented in JSON Schema"),nc):JSON.parse(a)}const bw=(e,t,r,n)=>{const i=e._zod.def;Se(i.innerType,t,n);const o=t.seen.get(e);o.ref=i.innerType;const a=Wm(i.defaultValue,e,t,r,n);a!==nc&&(r.default=a)},Sw=(e,t,r,n)=>{const i=e._zod.def;Se(i.innerType,t,n);const o=t.seen.get(e);if(o.ref=i.innerType,t.io!=="input")return;const a=Wm(i.defaultValue,e,t,r,n);a!==nc&&(r._prefault=a)},Iw=(e,t,r,n)=>{const i=e._zod.def;Se(i.innerType,t,n);const o=t.seen.get(e);o.ref=i.innerType;let a;try{a=i.catchValue(void 0)}catch{mr(e,t,r,n,"Dynamic catch values are not supported in JSON Schema");return}r.default=a},xw=(e,t,r,n)=>{const i=e._zod.def,o=i.in._zod.traits.has("$ZodTransform"),a=t.io==="input"?o?i.out:i.in:i.out;Se(a,t,n);const s=t.seen.get(e);s.ref=a},ww=(e,t,r,n)=>{const i=e._zod.def;Se(i.innerType,t,n);const o=t.seen.get(e);o.ref=i.innerType,r.readOnly=!0},Vm=(e,t,r,n)=>{const i=e._zod.def;Se(i.innerType,t,n);const o=t.seen.get(e);o.ref=i.innerType},kw=(e,t,r,n)=>{const i=e._zod.innerType;Se(i,t,n);const o=t.seen.get(e);o.ref=i},zu=new WeakSet([Object.prototype,Error.prototype]);function eo(e,t,r){Object.defineProperty(e,t,{configurable:!0,enumerable:!1,get(){const n=r(this);return Object.defineProperty(this,t,{value:n,configurable:!0,writable:!0}),n},set(n){Object.defineProperty(this,t,{value:n,configurable:!0,writable:!0})}})}const Pw=(e,t)=>{Mb.init(e,t),e.name="ZodError";const r=Object.getPrototypeOf(e);zu.has(r)||(zu.add(r),eo(r,"format",n=>i=>Ab(n,i)),eo(r,"flatten",n=>i=>Fb(n,i)),eo(r,"addIssue",n=>i=>{n.issues.push(i),n.message=JSON.stringify(n.issues,Js,2)}),eo(r,"addIssues",n=>i=>{n.issues.push(...i),n.message=JSON.stringify(n.issues,Js,2)}),Object.defineProperty(r,"isEmpty",{configurable:!0,enumerable:!1,get(){return this.issues.length===0}}))},lt=_("ZodError",Pw,void 0,{Parent:Error}),Tw=Zl(lt),Ew=Yl(lt),Cw=Xl(lt),Mw=Ql(lt),_w=Bb(lt),Fw=Ub(lt),Aw=Nb(lt),Ow=$b(lt),zw=Hb(lt),Dw=jb(lt),Rw=qb(lt),Lw=Kb(lt);function Bw(){st.localeError||Ft(XI())}function Oi(){st.memoizer||Ft({memoizer:VI()})}const me=_("ZodType",(e,t)=>(Bw(),de.init(e,t),e.def=t,e.type=t.type,e),{check(...e){const t=this.def;return this.clone(Je(t,{checks:[...t.checks??[],...e.map(r=>typeof r=="function"?{_zod:{check:r,def:{check:"custom"},onattach:[]}}:r)]}),{parent:!0})},with(...e){return this.check(...e)},clone(e,t){return yr(this,e,t)},brand(){return this},register(e,t){return e.add(this,t),this},refine(e,t){return this.check(Lk(e,t))},superRefine(e,t){return this.check(Bk(e,t))},overwrite(e){return this.check(Mn(e))},optional(){return Lu(this)},exactOptional(){return wk(this)},nullable(){return Bu(this)},nullish(){return Lu(Bu(this))},nonoptional(e){return Mk(this,e)},array(){return F(this)},or(e){return Wt([this,e])},and(e){return vk(this,e)},transform(e){return Uu(this,xk(e))},default(e){return Tk(this,e)},prefault(e){return Ck(this,e)},catch(e){return Fk(this,e)},pipe(e){return Uu(this,e)},readonly(){return zk(this)},describe(e){const t=this.clone();return Vn.add(t,{description:e}),t},meta(...e){if(e.length===0)return Vn.get(this);const t=this.clone();return Vn.add(t,e[0]),t},isOptional(){return this.safeParse(void 0).success},isNullable(){return this.safeParse(null).success},apply(e,...t){return t.length===0?e(this):e(this,...t)},get"~standard"(){return pm(this,"~standard",{...Im(this),jsonSchema:{input:ra(this,"input"),output:ra(this,"output")}})},set"~standard"(e){pr(this,"~standard",e)},parse:function e(t,r){return Tw(this,t,r,{callee:e})},parseAsync:async function e(t,r){return await Ew(this,t,r,{callee:e})},safeParse(e,t){return Cw(this,e,t)},async safeParseAsync(e,t){return Mw(this,e,t)},get spa(){return this?.safeParseAsync},set spa(e){pr(this,"spa",e)},validate(e,t){return Db(this,e,t)},validateAsync(e,t){return Lb(this,e,t)},encode:function e(t,r){return _w(this,t,r,{callee:e})},decode:function e(t,r){return Fw(this,t,r,{callee:e})},encodeAsync:async function e(t,r){return await Aw(this,t,r,{callee:e})},decodeAsync:async function e(t,r){return await Ow(this,t,r,{callee:e})},safeEncode(e,t){return zw(this,e,t)},safeDecode(e,t){return Dw(this,e,t)},async safeEncodeAsync(e,t){return Rw(this,e,t)},async safeDecodeAsync(e,t){return Lw(this,e,t)},toJSONSchema(e){return Yx(this,{})(e)},get description(){return Vn.get(this)?.description},get _def(){return this._zod.def}}),Zm=_("_ZodString",(e,t)=>{tc.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>ew(e,r,n)},mm({format:e=>it(e).format??null,minLength:e=>it(e).minimum??null,maxLength:e=>it(e).maximum??null},{regex(...e){return this.check(Ox(...e))},includes(...e){return this.check(Rx(...e))},startsWith(...e){return this.check(Lx(...e))},endsWith(...e){return this.check(Bx(...e))},min(...e){return this.check(ta(...e))},max(...e){return this.check(Dm(...e))},length(...e){return this.check(Rm(...e))},nonempty(...e){return this.check(ta(1,...e))},lowercase(e){return this.check(zx(e))},uppercase(e){return this.check(Dx(e))},trim(){return this.check(Nx())},normalize(...e){return this.check(Ux(...e))},toLowerCase(){return this.check($x())},toUpperCase(){return this.check(Hx())},slugify(){return this.check(jx())}})),Uw=_("ZodString",(e,t)=>{tc.init(e,t),Zm.init(e,t)},{email(e){return this.check(tx(qw,e))},url(e){return this.check(sx(Gw,e))},jwt(e){return this.check(xx(sk,e))},emoji(e){return this.check(lx(Ww,e))},guid(e){return this.check(rx(Kw,e))},uuid(e){return this.check(nx(to,e))},uuidv4(e){return this.check(ix(to,e))},uuidv6(e){return this.check(ox(to,e))},uuidv7(e){return this.check(ax(to,e))},nanoid(e){return this.check(cx(Vw,e))},cuid(e){return this.check(ux(Zw,e))},cuid2(e){return this.check(dx(Yw,e))},ulid(e){return this.check(fx(Xw,e))},base64(e){return this.check(bx(ik,e))},base64url(e){return this.check(Sx(ok,e))},xid(e){return this.check(px(Qw,e))},ksuid(e){return this.check(mx(Jw,e))},ipv4(e){return this.check(hx(ek,e))},ipv6(e){return this.check(gx(tk,e))},cidrv4(e){return this.check(yx(rk,e))},cidrv6(e){return this.check(vx(nk,e))},e164(e){return this.check(Ix(ak,e))},datetime(e){return this.check(wx(Nw,e))},date(e){return this.check(kx($w,e))},time(e){return this.check(Px(Hw,e))},duration(e){return this.check(Tx(jw,e))}});function I(e){return ex(Uw,e)}const Ie=_("ZodStringFormat",(e,t)=>{ve.init(e,t),Zm.init(e,t)}),Nw=_("ZodISODateTime",(e,t)=>{JS.init(e,t),Ie.init(e,t)}),$w=_("ZodISODate",(e,t)=>{eI.init(e,t),Ie.init(e,t)}),Hw=_("ZodISOTime",(e,t)=>{tI.init(e,t),Ie.init(e,t)}),jw=_("ZodISODuration",(e,t)=>{rI.init(e,t),Ie.init(e,t)}),qw=_("ZodEmail",(e,t)=>{US.init(e,t),Ie.init(e,t)}),Kw=_("ZodGUID",(e,t)=>{LS.init(e,t),Ie.init(e,t)}),to=_("ZodUUID",(e,t)=>{BS.init(e,t),Ie.init(e,t)}),Gw=_("ZodURL",(e,t)=>{KS.init(e,t),Ie.init(e,t)}),Ww=_("ZodEmoji",(e,t)=>{GS.init(e,t),Ie.init(e,t)}),Vw=_("ZodNanoID",(e,t)=>{WS.init(e,t),Ie.init(e,t)}),Zw=_("ZodCUID",(e,t)=>{VS.init(e,t),Ie.init(e,t)}),Yw=_("ZodCUID2",(e,t)=>{ZS.init(e,t),Ie.init(e,t)}),Xw=_("ZodULID",(e,t)=>{YS.init(e,t),Ie.init(e,t)}),Qw=_("ZodXID",(e,t)=>{XS.init(e,t),Ie.init(e,t)}),Jw=_("ZodKSUID",(e,t)=>{QS.init(e,t),Ie.init(e,t)}),ek=_("ZodIPv4",(e,t)=>{nI.init(e,t),Ie.init(e,t)}),tk=_("ZodIPv6",(e,t)=>{oI.init(e,t),Ie.init(e,t)}),rk=_("ZodCIDRv4",(e,t)=>{aI.init(e,t),Ie.init(e,t)}),nk=_("ZodCIDRv6",(e,t)=>{lI.init(e,t),Ie.init(e,t)}),ik=_("ZodBase64",(e,t)=>{cI.init(e,t),Ie.init(e,t)}),ok=_("ZodBase64URL",(e,t)=>{dI.init(e,t),Ie.init(e,t)}),ak=_("ZodE164",(e,t)=>{fI.init(e,t),Ie.init(e,t)}),sk=_("ZodJWT",(e,t)=>{mI.init(e,t),Ie.init(e,t)}),Ym=_("ZodNumber",(e,t)=>{Em.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>tw(e,r,n,i),e.isFinite=!0},mm({minValue:e=>{const{minimum:t,exclusiveMinimum:r}=it(e);return Math.max(t??Number.NEGATIVE_INFINITY,r??Number.NEGATIVE_INFINITY)},maxValue:e=>{const{maximum:t,exclusiveMaximum:r}=it(e);return Math.min(t??Number.POSITIVE_INFINITY,r??Number.POSITIVE_INFINITY)},isInt:e=>{const{isInt:t,multipleOf:r}=it(e);return!!t||!!r?.some(Number.isSafeInteger)},format:e=>it(e).format??null},{gt(e,t){return this.check(Pu(e,t))},gte(e,t){return this.check(Xa(e,t))},min(e,t){return this.check(Xa(e,t))},lt(e,t){return this.check(ku(e,t))},lte(e,t){return this.check(Ya(e,t))},max(e,t){return this.check(Ya(e,t))},int(e){return this.check(Du(e))},safe(e){return this.check(Du(e))},positive(e){return this.check(Pu(0,e))},nonnegative(e){return this.check(Xa(0,e))},negative(e){return this.check(ku(0,e))},nonpositive(e){return this.check(Ya(0,e))},multipleOf(e,t){return this.check(Tu(e,t))},step(e,t){return this.check(Tu(e,t))},finite(){return this}}));function P(e){return Ex(Ym,e)}const lk=_("ZodNumberFormat",(e,t)=>{hI.init(e,t),Ym.init(e,t)});function Du(e){return Cx(lk,e)}const ck=_("ZodBoolean",(e,t)=>{gI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>rw(e,r,n)});function B(e){return Mx(ck,e)}const uk=_("ZodNull",(e,t)=>{yI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>nw(e,r,n)});function dk(e){return _x(uk,e)}const fk=_("ZodUnknown",(e,t)=>{vI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>ow()});function sl(){return Fx(fk)}const pk=_("ZodNever",(e,t)=>{bI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>iw(e,r,n)});function mk(e){return Ax(pk,e)}const hk=_("ZodArray",(e,t)=>{Oi(),xI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>uw(e,r,n,i),e.element=t.element},{min(e,t){return this.check(ta(e,t))},nonempty(e){return this.check(ta(1,e))},max(e,t){return this.check(Dm(e,t))},length(e,t){return this.check(Rm(e,t))},unwrap(){return this.element}});function F(e,t){return qx(hk,e,t)}const Xm=_("ZodObject",(e,t)=>{Oi(),PI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>dw(e,r,n,i),Ib(e,"shape",r=>r._zod.def.shape,!1)},{keyof(){return U(Object.keys(this._zod.def.shape))},catchall(e){return this.clone(Je(this._zod.def,{catchall:e}))},passthrough(){return this.clone(Je(this._zod.def,{catchall:sl()}))},loose(){return this.clone(Je(this._zod.def,{catchall:sl()}))},strict(){return this.clone(Je(this._zod.def,{catchall:mk()}))},strip(){return this.clone(Je(this._zod.def,{catchall:void 0}))},extend(e){return ub(this,e)},safeExtend(e){return db(this,e)},merge(e){return fb(this,e)},pick(e){return lb(this,e)},omit(e){return cb(this,e)},partial(...e){return nu(ic,this,e[0])},exactPartial(...e){return nu(Jm,this,e[0],"exactPartial")},required(...e){return pb(eh,this,e[0])}});function y(e,t){const r={type:"object",shape:e??{},...N(t)};return new Xm(r)}function zi(e,t){return new Xm({type:"object",shape:e,catchall:sl(),...N(t)})}const Qm=_("ZodUnion",(e,t)=>{_m.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>fw(e,r,n,i),e.options=t.options});function Wt(e,t){return new Qm({type:"union",options:e,...N(t)})}const gk=_("ZodDiscriminatedUnion",(e,t)=>{Qm.init(e,t),EI.init(e,t)});function _a(e,t,r){return new gk({type:"union",options:t,discriminator:e,...N(r)})}const yk=_("ZodIntersection",(e,t)=>{CI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>pw(e,r,n,i)});function vk(e,t){return new yk({type:"intersection",left:e,right:t})}const bk=_("ZodTuple",(e,t)=>{Oi(),MI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>mw(e,r,n,i)},{rest(e){return this.clone({...this._zod.def,rest:e})},partial(){const e=this._zod.def;if(e.checks?.length)throw new Error(".partial() cannot be used on tuple schemas containing refinements");return this.clone({...e,items:e.items.map(t=>new ic({type:"optional",innerType:t}))})}});function ui(e,t,r){const n=t instanceof de,i=n?r:t,o=n?t:null;return new bk({type:"tuple",items:e,rest:o,...N(i)})}const Ru=_("ZodRecord",(e,t)=>{Oi(),_I.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>gw(e,r,n,i),e.keyType=t.keyType,e.valueType=t.valueType});function Ct(e,t,r){return!t||!t._zod?new Ru({type:"record",keyType:I(),valueType:e,...N(t)}):new Ru({type:"record",keyType:e,valueType:t,...N(r)})}const ll=_("ZodEnum",(e,t)=>{FI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(n,i,o)=>aw(e,n,i),e.enum=t.entries,e.options=[...e._zod.values];const r=new Set(Object.keys(t.entries));e.extract=(n,i)=>{const o={};for(const a of n)if(r.has(a))o[a]=t.entries[a];else throw new Error(`Key ${a} not found in enum`);return new ll({...t,checks:[],...N(i),entries:o})},e.exclude=(n,i)=>{const o={...t.entries};for(const a of n)if(r.has(a))delete o[a];else throw new Error(`Key ${a} not found in enum`);return new ll({...t,checks:[],...N(i),entries:o})}});function U(e,t){const r=Array.isArray(e)?Object.fromEntries(e.map(n=>[n,n])):e;return new ll({type:"enum",entries:r,...N(t)})}const Sk=_("ZodLiteral",(e,t)=>{AI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>sw(e,r,n,i),e.values=new Set(t.values),Object.defineProperty(e,"value",{get(){if(t.values.length>1)throw new Error("This schema contains multiple valid literal values. Use `.values` instead.");return t.values[0]}})});function ne(e,t){return new Sk({type:"literal",values:Array.isArray(e)?e:[e],...N(t)})}const Ik=_("ZodTransform",(e,t)=>{Oi(),OI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>cw(e,r,n,i),e._zod.parse=(r,n)=>{if(n.direction==="backward")throw new hm(e.constructor.name);r.addIssue=o=>{if(typeof o=="string")r.issues.push(Ii(o,r.value,t));else{const a=o;a.fatal&&(a.continue=!1),a.code??(a.code="custom"),"input"in a||(a.input=r.value),a.inst??(a.inst=e),r.issues.push(Ii(a))}};const i=t.transform(r.value,r);return i instanceof Promise?i.then(o=>(r.value=o,r)):(r.value=i,r)}});function xk(e){return new Ik({type:"transform",transform:e})}const ic=_("ZodOptional",(e,t)=>{Fm.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>Vm(e,r,n,i),e.unwrap=()=>e._zod.def.innerType});function Lu(e){return new ic({type:"optional",innerType:e})}const Jm=_("ZodExactOptional",(e,t)=>{zI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>Vm(e,r,n,i),e.unwrap=()=>e._zod.def.innerType});function wk(e){return new Jm({type:"optional",innerType:e})}const kk=_("ZodNullable",(e,t)=>{DI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>yw(e,r,n,i),e.unwrap=()=>e._zod.def.innerType});function Bu(e){return new kk({type:"nullable",innerType:e})}const Pk=_("ZodDefault",(e,t)=>{RI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>bw(e,r,n,i),e.unwrap=()=>e._zod.def.innerType,e.removeDefault=e.unwrap});function Tk(e,t){return new Pk({type:"default",innerType:e,get defaultValue(){return typeof t=="function"?t():cm(t)}})}const Ek=_("ZodPrefault",(e,t)=>{LI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>Sw(e,r,n,i),e.unwrap=()=>e._zod.def.innerType});function Ck(e,t){return new Ek({type:"prefault",innerType:e,get defaultValue(){return typeof t=="function"?t():cm(t)}})}const eh=_("ZodNonOptional",(e,t)=>{BI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>vw(e,r,n,i),e.unwrap=()=>e._zod.def.innerType});function Mk(e,t){return new eh({type:"nonoptional",innerType:e,...N(t)})}const _k=_("ZodCatch",(e,t)=>{UI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>Iw(e,r,n,i),e.unwrap=()=>e._zod.def.innerType,e.removeCatch=e.unwrap});function Fk(e,t){return new _k({type:"catch",innerType:e,catchValue:typeof t=="function"?t:wb(t)})}const Ak=_("ZodPipe",(e,t)=>{NI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>xw(e,r,n,i),e.in=t.in,e.out=t.out});function Uu(e,t){return new Ak({type:"pipe",in:e,out:t})}const Ok=_("ZodReadonly",(e,t)=>{$I.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>ww(e,r,n,i),e.unwrap=()=>e._zod.def.innerType});function zk(e){return new Ok({type:"readonly",innerType:e})}const Dk=_("ZodLazy",(e,t)=>{HI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>kw(e,r,n,i),e.unwrap=()=>e._zod.def.getter()});function Rk(e){return new Dk({type:"lazy",getter:e})}const th=_("ZodCustom",(e,t)=>{jI.init(e,t),me.init(e,t),e._zod.processJSONSchema=(r,n,i)=>lw(e,r,n,i)});function rh(e,t){return Kx(th,e??(()=>!0),t)}function Lk(e,t={}){return Gx(th,e,t)}function Bk(e,t){return Wx(e,t)}function Fa(e){const t=Rk(()=>Wt([I(e),P(),B(),dk(),F(t),Ct(I(),t)]));return t}const ee=encodeURIComponent,Qa=192*1024*1024,Nu=y({id:I(),background:y({kind:U(["black","green","none"]),keyColor:I().optional(),version:P(),detectedAt:P(),contentHash:I()}).optional(),overlay:B().optional()}).passthrough();class Uk{constructor(t="/api/editor",r=fetch){this.baseUrl=t,this.request=r}baseUrl;request;workspacePromise=null;async response(t,r){const n=await this.request.call(globalThis,`${this.baseUrl}${t}`,{cache:"no-store",...r});if(n.ok)return n;const i=await n.json().catch(()=>({})),o=typeof i.error=="string"?i.error:`Lỗi lưu trữ (${n.status})`;throw n.status===409?new Xo(o,typeof i.revision=="string"?i.revision:null,i):new $t(typeof i.code=="string"?i.code:"internal_error",o,n.status,i)}async json(t,r){return(await this.response(t,r)).json()}jsonWrite(t,r,n){return{method:t,headers:{"content-type":"application/json",...n&&{"if-match":n}},body:JSON.stringify(r)}}upload(t,r={}){const n={method:"PUT",body:t,signal:r.signal,headers:{"content-type":(r.mime??(t instanceof Blob?t.type:""))||"application/octet-stream"}};return t instanceof Blob||(n.duplex="half"),n}async uploadResponse(t,r,n,i={}){if(!(n instanceof Blob)||this.request!==fetch||typeof XMLHttpRequest>"u")return this.response(t,{...this.upload(n,i),method:r});if(i.signal?.aborted)throw new DOMException("Đã hủy tải lên","AbortError");const{promise:o,resolve:a,reject:s}=Promise.withResolvers(),l=new XMLHttpRequest,c=()=>l.abort(),u=()=>i.signal?.removeEventListener("abort",c);l.open(r,`${this.baseUrl}${t}`),l.setRequestHeader("content-type",i.mime||n.type||"application/octet-stream"),l.upload.onprogress=f=>i.onProgress?.(f.loaded,f.lengthComputable?f.total:n.size),l.onload=()=>{u(),a(new Response(l.status===204?null:l.responseText,{status:l.status,headers:{"content-type":l.getResponseHeader("content-type")||"application/json"}}))},l.onerror=()=>{u(),s(new $t("network","Không thể tải dữ liệu lên máy chủ",0))},l.onabort=()=>{u(),s(new DOMException("Đã hủy tải lên","AbortError"))},i.signal?.addEventListener("abort",c,{once:!0}),l.send(n);const d=await o;if(!d.ok){const f=await d.json().catch(()=>({})),p=typeof f.error=="string"?f.error:`Lỗi tải lên (${d.status})`;throw d.status===409?new Xo(p,typeof f.revision=="string"?f.revision:null,f):new $t(typeof f.code=="string"?f.code:"internal_error",p,d.status,f)}return i.onProgress?.(n.size,n.size),d}workspaceId(){return this.workspacePromise||(this.workspacePromise=this.json("/workspace").then(t=>t.workspaceId).catch(t=>{throw this.workspacePromise=null,t})),this.workspacePromise}async listProjects(){return(await this.json("/projects")).projects}async createProject(t){const r=await this.json("/projects",this.jsonWrite("POST",t));return{data:r.project,revision:r.revision}}async loadProject(t){const r=await this.response(`/projects/${ee(t)}`);return{data:await r.json(),revision:r.headers.get("etag")?.replace(/^"|"$/g,"")??""}}async saveProject(t,r){return(await this.json(`/projects/${ee(t.id)}`,this.jsonWrite("PUT",t,r))).revision}async renameProject(t,r,n){return(await this.json(`/projects/${ee(t)}`,this.jsonWrite("PATCH",{name:r},n))).revision}async duplicateProject(t,r){const n=await this.json(`/projects/${ee(t)}/duplicate`,this.jsonWrite("POST",{name:r}));return{data:n.project,revision:n.revision}}async deleteProject(t){await this.response(`/projects/${ee(t)}`,{method:"DELETE"})}async loadSidecar(t,r){const n=await this.response(`/projects/${ee(t)}/sidecars/${r}`);return{data:await n.json(),revision:n.headers.get("etag")?.replace(/^"|"$/g,"")??"empty"}}async saveSidecar(t,r,n,i){return(await this.json(`/projects/${ee(t)}/sidecars/${r}`,this.jsonWrite("PUT",n,i))).revision}async loadWorkspaceEffectPresets(){const t=await this.response("/workspace/effect-presets");return{data:await t.json(),revision:t.headers.get("etag")?.replace(/^"|"$/g,"")??"empty"}}async saveWorkspaceEffectPresets(t,r){return(await this.json("/workspace/effect-presets",this.jsonWrite("PUT",t,r))).revision}async listMedia(t){return(await this.json(`/media${t?`?projectId=${ee(t)}`:""}`)).media}getMedia(t){return this.json(`/media/${ee(t)}/metadata`)}async updateMediaMetadata(t,r){return(await this.json(`/media/${ee(t)}/metadata`,this.jsonWrite("PATCH",r))).media}async detectMediaBackground(t,r={}){const n=r.refresh?"?refresh=1":"",i={};r.overlay!==void 0&&(i.overlay=r.overlay);const o=await this.json(`/media/${ee(t)}/background${n}`,this.jsonWrite("POST",i));return Nu.parse(o.media)}async importMedia(t,r){const n=new URLSearchParams({fileName:r.fileName,...r.projectId&&{projectId:r.projectId}});return(await(await this.uploadResponse(`/media?${n}`,"POST",t,r)).json()).media}async importMediaFromLink(t){const{signal:r,...n}=t,i=await this.json("/media/link",{...this.jsonWrite("POST",n),signal:r});try{const o=await Zv(i.job.id,{signal:r,request:this.request,baseUrl:this.baseUrl});return{media:Nu.parse(o.media),reused:o.reused===!0}}catch(o){throw r?.aborted&&await Yv(i.job.id,{request:this.request,baseUrl:this.baseUrl}),o}}async replaceMedia(t,r,n){return(await(await this.uploadResponse(`/media/${ee(t)}${n?.fileName?`?fileName=${ee(n.fileName)}`:""}`,"PUT",r,n)).json()).media}mediaUrl(t,r){return`${this.baseUrl}/media/${ee(t)}?v=${ee(r)}`}async deleteMedia(t){await this.response(`/media/${ee(t)}`,{method:"DELETE"})}async linkMedia(t,r){await this.response(`/projects/${ee(t)}/media/${ee(r)}`,{method:"PUT"})}async unlinkMedia(t,r){await this.response(`/projects/${ee(t)}/media/${ee(r)}`,{method:"DELETE"})}async listOverlayLibrary(){return(await this.json("/overlays")).media}async removeFromOverlayLibrary(t){return(await this.json(`/overlays/${ee(t)}`,{method:"DELETE"})).media}async beginExport(t,r){return(await this.json(`/projects/${ee(t)}/exports`,this.jsonWrite("POST",r))).exportId}async uploadExport(t,r,n,i){if(n instanceof Blob&&n.size>Qa&&this.request===fetch&&typeof XMLHttpRequest<"u"){for(let o=0;o<n.size;o+=Qa){const a=n.slice(o,Math.min(o+Qa,n.size),n.type);await this.uploadResponse(`/projects/${ee(t)}/exports/${ee(r)}?offset=${o}`,"PUT",a,{...i,onProgress:i?.onProgress?s=>i.onProgress(o+s,n.size):void 0})}i?.onProgress?.(n.size,n.size);return}await this.uploadResponse(`/projects/${ee(t)}/exports/${ee(r)}`,"PUT",n,i)}async commitExport(t,r,n){return(await this.json(`/projects/${ee(t)}/exports/${ee(r)}?commit`,n===void 0?{method:"POST"}:this.jsonWrite("POST",n))).export}abortExport(t,r){return this.deleteExport(t,r)}async listExports(t){return(await this.json(`/projects/${ee(t)}/exports`)).exports}exportUrl(t,r){return`${this.baseUrl}/projects/${ee(t)}/exports/${ee(r)}`}async deleteExport(t,r){await this.response(`/projects/${ee(t)}/exports/${ee(r)}`,{method:"DELETE"})}}let Nk=new Uk;function vt(){return Nk}const $k={kind:"editor-opfs-cache"};async function nh(e){const t=vt(),[r,n]=await Promise.all([t.workspaceId(),t.getMedia(e)]);return{workspaceId:r,mediaId:e,contentHash:n.contentHash}}async function ih(e,t){const r=t?.workspaceId??await vt().workspaceId();if(e[0]==="media"&&e[1]){const n=t??await nh(e[1]);if(n.mediaId!==e[1])throw new Error("Media không khớp danh tính cache");return["ytf-editor-cache",encodeURIComponent(r),"media",encodeURIComponent(n.mediaId),encodeURIComponent(n.contentHash),...e.slice(2)]}return["ytf-editor-cache",encodeURIComponent(r),...e]}async function Di(e,t=!1,r){let n=await navigator.storage.getDirectory();for(const i of await ih(e,r)){if(!i||i==="."||i===".."||i.includes("/"))throw new Error("Đường dẫn cache không hợp lệ");n=await n.getDirectoryHandle(i,{create:t})}return n}async function cl(e,t){try{return await(await(await Di(e.slice(0,-1),!1,t)).getFileHandle(e[e.length-1])).getFile()}catch{return null}}async function Hk(e,t,r){try{const o=await(await(await Di(e.slice(0,-1),!0,r)).getFileHandle(e[e.length-1],{create:!0})).createWritable();try{await o.write(t),await o.close()}catch(a){throw await o.abort().catch(()=>{}),a}}catch{}}async function ul(e,t=!1,r){try{await(await Di(e.slice(0,-1),!1,r)).removeEntry(e[e.length-1],{recursive:t})}catch{}}const oh="proxy.mp4",jk="meta.json",qk=4;function Kk(e){return[...e,oh].join("/")}const ah="@";function sh(e){if(!e.contentHash)throw new Error(`Media ${e.id} has no content hash; cannot derive its proxy key`);return`${e.id}${ah}${e.contentHash}`}function dl(e){const t=e.indexOf(ah);return t<=0||t===e.length-1?null:{mediaId:e.slice(0,t),contentHash:e.slice(t+1)}}async function Zn(e){const t=dl(e);if(!t)throw new Error(`Proxy key "${e}" does not identify a media version`);return{workspaceId:await vt().workspaceId(),...t}}async function $u(e){const t=await Zn(e);return ih(["media",t.mediaId,"proxy"],t)}function Ri(e){return _e()(t=>({...e.initialState(),setPreview:r=>t(e.normalizePreview?.(r)??r),clearPreview:()=>t(e.initialState()),...e.createActions?.(t)??{}}))}const Gk=()=>({trimmedItemId:null,handle:null,trackId:null,downstreamItemIds:new Set,delta:0,trimDelta:0}),lh=Ri({initialState:Gk,createActions:e=>({setDeltas:(t,r)=>e({delta:t,trimDelta:r})})});function ch(e,t){const r={...e};for(const[n,i]of Object.entries(t))r[n]=r[n]??i;return r}const Wk=()=>({trimmedItemId:null,neighborItemId:null,handle:null,neighborDelta:0,constrained:!1}),uh=Ri({initialState:Wk,normalizePreview:e=>ch(e,{constrained:!1}),createActions:e=>({setNeighborDelta:(t,r)=>e({neighborDelta:t,constrained:r??!1})})}),Vk=()=>({itemId:null,trackId:null,leftNeighborId:null,rightNeighborId:null,slideDelta:0,minDelta:0,maxDelta:0}),dh=Ri({initialState:Vk,normalizePreview:e=>ch(e,{minDelta:0,maxDelta:0}),createActions:e=>({setSlideDelta:t=>e({slideDelta:t}),setSlideRange:(t,r)=>e({minDelta:t,maxDelta:r})})}),Zk=()=>({itemId:null,trackId:null,slipDelta:0}),fh=Ri({initialState:Zk,createActions:e=>({setSlipDelta:t=>e({slipDelta:t})})}),Yk=()=>({anchorItemId:null,trackId:null,shiftedItemIds:new Set,delta:0}),ph=Ri({initialState:Yk,createActions:e=>({setDelta:t=>e({delta:t})})}),Xk=50;function Qk(e,t,r){return Math.max(t,Math.min(r,e))}function Jk(e,t){const r=Math.max(10,t),n=Math.max(0,e-Xk);return Qk(n/(r*100),V0,W0)}const e1=_e(()=>({contentLevel:1,contentPixelsPerSecond:100})),Hu=120,t1=200,ju=200;let Er=0,kt=null,cn=null,un=null,di=null,dn=null,Ut=null,lr=null,Yn=0;function na(e){return e*100}function oc(){un&&(clearTimeout(un),un=null)}function ia(){di&&(clearTimeout(di),di=null),dn!==null&&(typeof cancelAnimationFrame=="function"&&cancelAnimationFrame(dn),dn=null),Ut!==null&&(typeof cancelIdleCallback=="function"&&cancelIdleCallback(Ut),Ut=null),lr!==null&&(clearTimeout(lr),lr=null)}function Do(e,t,r){e({level:t,pixelsPerSecond:na(t),isZoomInteracting:r})}function r1(e){if(kt===null)return;oc(),Er=performance.now();const t=kt;kt=null,Do(e,t,!0)}function Ro(e){if(dn=null,Ut=null,lr!==null&&(clearTimeout(lr),lr=null),cn===null){e({isZoomInteracting:!1});return}const t=cn;cn=null,r1(e),e({level:t,pixelsPerSecond:na(t),contentLevel:t,contentPixelsPerSecond:na(t),isZoomInteracting:!1})}function n1(e){if(!be.getState().isPlaying||typeof requestAnimationFrame!="function"||typeof requestIdleCallback!="function"){Ro(e);return}dn=requestAnimationFrame(()=>{dn=null;const t=()=>{Ut=null,Ro(e)};Ut=requestIdleCallback(t,{timeout:ju}),lr=setTimeout(()=>{lr=null,Ut!==null&&typeof cancelIdleCallback=="function"&&(cancelIdleCallback(Ut),Ut=null),Ro(e)},ju)})}function i1(e){ia(),!(Yn>0)&&(di=setTimeout(()=>{di=null,n1(e)},t1))}function qu(e,t,r=!1){cn=t,r||e({isZoomInteracting:!0}),i1(e)}function ro(e,t){kt=null,cn=null,oc(),ia(),Er=performance.now();const r=na(t);e({level:t,pixelsPerSecond:r,contentLevel:t,contentPixelsPerSecond:r,isZoomInteracting:!1})}const Br=_e((e,t)=>({level:1,pixelsPerSecond:100,contentLevel:1,contentPixelsPerSecond:100,isZoomInteracting:!1,beginZoomGesture:()=>{Yn+=1,ia(),t().isZoomInteracting||e({isZoomInteracting:!0})},endZoomGesture:()=>{Yn>0&&(Yn-=1),!(Yn>0)&&(cn!==null?(ia(),Ro(e)):t().isZoomInteracting&&e({isZoomInteracting:!1}))},setZoomLevel:r=>{const n=performance.now();if(kt=r,qu(e,r),n-Er>=Hu){Er=n,Do(e,r,!0),kt=null;return}un||(un=setTimeout(()=>{if(un=null,kt!==null){const i=kt;kt=null,Er=performance.now(),Do(e,i,!0)}},Hu-(n-Er)))},setZoomLevelImmediate:r=>{oc(),kt=null,Er=performance.now(),Do(e,r,!0),qu(e,r,!0)},setZoomLevelSynchronized:r=>{ro(e,r)},zoomIn:()=>{const r=Math.min(t().level*1.1,50);ro(e,r)},zoomOut:()=>{const r=Math.max(t().level/1.1,.01);ro(e,r)},zoomToFit:(r,n)=>{const i=Jk(r,n);ro(e,i)}}));Br.subscribe((e,t)=>{e.contentLevel===t.contentLevel&&e.contentPixelsPerSecond===t.contentPixelsPerSecond||e1.setState({contentLevel:e.contentLevel,contentPixelsPerSecond:e.contentPixelsPerSecond})});const o1=1200,fl=new Map,Ku=new Set;function a1(){return!!yt.getState().dragState?.isDragging}function s1(){const e=uh.getState();if(e.trimmedItemId!==null||e.neighborItemId!==null||e.handle!==null)return!0;const t=lh.getState();return t.trimmedItemId!==null||t.handle!==null||fh.getState().itemId!==null||dh.getState().itemId!==null?!0:ph.getState().anchorItemId!==null}function mh(e,t){return t<e.releaseAtMs?!0:e.releaseRequested?!1:e.expiresAtMs===null||t<e.expiresAtMs}function l1(e=Date.now()){let t=!1;for(const[r,n]of fl)mh(n,e)||(fl.delete(r),t=!0);return t}function c1(){l1();const e=Date.now();for(const t of fl.values())if(mh(t,e))return!0;return!1}function Ja(e={}){return Br.getState().isZoomInteracting||a1()||s1()||!e.ignoreAudioStartupHold&&c1()}function u1(e){Ku.add(e);const t=[Br.subscribe(e),yt.subscribe(e),uh.subscribe(e),lh.subscribe(e),fh.subscribe(e),dh.subscribe(e),ph.subscribe(e)];return()=>{Ku.delete(e);for(const r of t)r()}}function d1(e,t,r){let n=null,i=null,o=!1;const a=()=>{if(!o){if(typeof requestIdleCallback=="function"){i=requestIdleCallback(()=>{i=null,o||e()},{timeout:r});return}n=setTimeout(()=>{n=null,o||e()},0)}};return t<=0?a():n=setTimeout(()=>{n=null,a()},t),()=>{o=!0,n!==null&&clearTimeout(n),i!==null&&typeof cancelIdleCallback=="function"&&cancelIdleCallback(i)}}function f1(e,t={}){const r=t.delayMs??0,n=t.idleTimeoutMs??o1,i={ignoreAudioStartupHold:t.ignoreAudioStartupHold??!1};let o=!1,a=()=>{},s=()=>{};const l=()=>{a(),s()},c=()=>{a(),a=d1(()=>{if(!o){if(Ja(i)){d();return}e()}},r,n)},u=()=>{o||Ja(i)||(s(),s=()=>{},c())},d=()=>{s(),s=u1(u)};return Ja(i)?d():c(),()=>{o=!0,l()}}const p1=He("BackgroundMediaWork"),hh=1;let m1=1,h1=1,Xn=0,Aa=null,oa=()=>{};const cr=[];function g1(){cr.sort((e,t)=>e.priority!==t.priority?e.priority-t.priority:e.readyAtMs!==t.readyAtMs?e.readyAtMs-t.readyAtMs:e.sequence-t.sequence)}function gh(){oa(),oa=()=>{},Aa=null}function y1(e){if(Aa=null,oa=()=>{},Xn>=hh){tn();return}const t=cr.findIndex(n=>n.id===e);if(t===-1){tn();return}const[r]=cr.splice(t,1);if(!r){tn();return}Xn+=1,Promise.resolve().then(r.run).catch(n=>{p1.warn("Background media work failed:",n)}).finally(()=>{Xn=Math.max(0,Xn-1),tn()})}function tn(){if(gh(),Xn>=hh||cr.length===0)return;g1();const e=cr[0];if(!e)return;const t=Math.max(0,e.readyAtMs-Date.now());Aa=e.id,oa=f1(()=>{y1(e.id)},{delayMs:t})}function Gu(e,t={}){const r=m1++,n=t.priority==="heavy"?1:0;return cr.push({id:r,sequence:h1++,readyAtMs:Date.now()+Math.max(0,t.delayMs??0),priority:n,run:e}),tn(),()=>{const i=cr.findIndex(o=>o.id===r);i!==-1&&cr.splice(i,1),Aa===r&&(gh(),tn())}}const Ke=He("ProxyService");function Wu(e){im(e),URL.revokeObjectURL(e)}let no=null;function v1(e){if(!e||e==="avc")return!0;if(e==="hevc"){if(no===null)try{no=typeof document<"u"&&document.createElement("video").canPlayType('video/mp4; codecs="hvc1.1.6.L93.B0"')==="probably"}catch{no=!1}return no}return!0}const Vu=150,b1=.01,S1=1,I1=12,x1=0,w1=900;class k1{proxyBlobUrlByKey=new Map;proxyKeyByMediaId=new Map;mediaIdsByProxyKey=new Map;progressByProxyKey=new Map;pendingJobsByKey=new Map;pendingJobOrder=[];activeJobPhaseByKey=new Map;activeJobPriorityByKey=new Map;progressEmissionByProxyKey=new Map;statusListener=null;mediaResolver=null;filmstripPrewarm=null;generatingProxyKeys=new Set;isRefreshing=!1;maxConcurrentJobs=1;workerManager=qv({createWorker:()=>new Worker(new URL("/assets/proxy-generation-worker-BhQ8Qoz1.js",import.meta.url),{type:"module"}),setupWorker:t=>(t.onmessage=r=>{this.handleWorkerMessage(r.data)},t.onerror=r=>{Ke.error("Proxy worker error:",r)},()=>{t.onmessage=null,t.onerror=null})});onStatusChange(t){this.statusListener=t}setMediaResolver(t){this.mediaResolver=t}setFilmstripPrewarm(t){this.filmstripPrewarm=t}setProxyKey(t,r){const n=this.proxyKeyByMediaId.get(t);if(n===r)return;if(n){const o=this.mediaIdsByProxyKey.get(n);o?.delete(t),o&&o.size===0&&this.mediaIdsByProxyKey.delete(n)}this.proxyKeyByMediaId.set(t,r);let i=this.mediaIdsByProxyKey.get(r);i||(i=new Set,this.mediaIdsByProxyKey.set(r,i)),i.add(t),this.proxyBlobUrlByKey.has(r)?this.statusListener?.(t,"ready"):this.generatingProxyKeys.has(r)&&this.statusListener?.(t,"generating",this.progressByProxyKey.get(r)??0)}clearProxyKey(t){const r=this.proxyKeyByMediaId.get(t);if(!r)return;this.proxyKeyByMediaId.delete(t);const n=this.mediaIdsByProxyKey.get(r);n?.delete(t),n&&n.size===0&&this.mediaIdsByProxyKey.delete(r)}getProxyKey(t){return this.proxyKeyByMediaId.get(t)}canGenerateProxy(t){return t.startsWith("video/")}generateProxy(t,r,n,i,o,a){o&&this.setProxyKey(t,o);const s=this.resolveProxyKey(t,o);if(this.proxyBlobUrlByKey.has(s)){this.emitStatusForProxyKey(s,"ready");return}if(this.generatingProxyKeys.has(s)){const l=this.pendingJobsByKey.get(s);l&&(a?.priority??"user")==="user"&&l.priority!=="user"&&(this.removePendingJob(s),l.priority="user",this.insertPendingJob(l)),(a?.priority??"user")==="user"&&this.activeJobPriorityByKey.set(s,"user"),this.emitStatusForProxyKey(s,"generating",this.progressByProxyKey.get(s)??0);return}this.generatingProxyKeys.add(s),this.progressByProxyKey.set(s,0),this.emitStatusForProxyKey(s,"generating",0),this.insertPendingJob({mediaId:t,proxyKey:s,sourceWidth:n,sourceHeight:i,source:this.normalizeSource(r),priority:a?.priority??"user"}),this.drainQueue()}cancelProxy(t,r){const n=this.resolveProxyKey(t,r);if(!this.generatingProxyKeys.has(n))return;if(this.generatingProxyKeys.delete(n),this.progressByProxyKey.delete(n),this.clearProgressEmissionState(n),this.emitStatusForProxyKey(n,"idle"),this.pendingJobsByKey.has(n)){this.removePendingJob(n),this.activeJobPriorityByKey.delete(n);return}const i=this.activeJobPhaseByKey.get(n);if(!i)return;if(i==="loading"){this.activeJobPhaseByKey.delete(n),this.activeJobPriorityByKey.delete(n),this.drainQueue();return}this.getWorker().postMessage({type:"cancel",mediaId:n})}cancelBackgroundProxy(t,r){const n=this.resolveProxyKey(t,r),i=this.pendingJobsByKey.get(n)?.priority,o=this.activeJobPriorityByKey.get(n);i!=="background"&&o!=="background"||this.cancelProxy(t,n)}getProxyBlobUrl(t,r){const n=this.resolveProxyKey(t,r);return this.proxyBlobUrlByKey.get(n)??null}getMediaIdByProxyUrl(t){for(const[r,n]of this.proxyBlobUrlByKey)if(n===t)return this.mediaIdsByProxyKey.get(r)?.values().next().value??null;return null}hasProxy(t,r){const n=this.resolveProxyKey(t,r);return this.proxyBlobUrlByKey.has(n)}async deleteProxy(t,r){const n=this.resolveProxyKey(t,r);if(this.cancelProxy(t,n),this.releaseProxyUrl(n),!dl(n))return;const i=await Zn(n);await ul(["media",i.mediaId,"proxy"],!0,i),Ke.debug(`Deleted proxy for ${n}`)}clearMemory(t){const r=this.resolveProxyKey(t);this.cancelProxy(t,r),this.releaseProxyUrl(r),this.clearProxyKey(t)}async loadExistingProxies(t){const r=[],n=new Set;for(const a of t)n.add(this.resolveProxyKey(a));const i=a=>{const s=this.mediaIdsByProxyKey.get(a);return!s||s.size===0?[]:[...s]},o=async(a,s)=>{try{const l=await Zn(a);await ul(["media",l.mediaId,"proxy"],!0,l),Ke.debug(`Removed ${s} proxy for ${a}`)}catch(l){Ke.warn(`Failed to remove ${s} proxy for ${a}:`,l)}};for(const a of n)if(dl(a))try{const s=await this.readProxyMetadata(a);if(!s)continue;if(s.status==="generating"){r.push(...i(a)),await o(a,"interrupted");continue}if(s.status==="error"){await o(a,"failed");continue}if(s.status!=="ready"){r.push(...i(a)),await o(a,"invalid-status");continue}if(s.version!==qk){r.push(...i(a)),await o(a,"stale");continue}if(!v1(s.codec)){Ke.debug(`Skipping proxy ${a}: codec '${s.codec}' not playable here; using source`);continue}const l=await this.readProxyFile(a);if(!l||l.size===0){r.push(...i(a)),await o(a,"empty");continue}await this.setProxyUrl(a,l),this.emitStatusForProxyKey(a,"ready"),Ke.debug(`Loaded existing proxy for ${a}`)}catch(s){Ke.warn(`Failed to load existing proxy for ${a}:`,s),r.push(...i(a)),await o(a,"invalid")}return r}getWorker(){return this.workerManager.getWorker()}handleWorkerMessage(t){const r=t.mediaId;switch(t.type){case"progress":{if(!this.generatingProxyKeys.has(r))break;this.progressByProxyKey.set(r,t.progress),this.emitProgressThrottled(r,t.progress);break}case"complete":{const n=!this.generatingProxyKeys.has(r);if(this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.generatingProxyKeys.delete(r),this.progressByProxyKey.delete(r),this.clearProgressEmissionState(r),this.drainQueue(),n)break;this.loadCompletedProxy(r);break}case"cancelled":{this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.generatingProxyKeys.delete(r),this.progressByProxyKey.delete(r),this.clearProgressEmissionState(r),this.drainQueue();break}case"error":{const n=!this.generatingProxyKeys.has(r);if(this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.generatingProxyKeys.delete(r),this.progressByProxyKey.delete(r),this.clearProgressEmissionState(r),this.drainQueue(),n)break;Ke.error(`Proxy generation failed for ${r}:`,t.error),this.emitStatusForProxyKey(r,"error");break}}}async loadCompletedProxy(t){try{const r=await this.readProxyFile(t);if(!r||r.size===0){this.emitStatusForProxyKey(t,"error");return}await this.setProxyUrl(t,r),this.emitStatusForProxyKey(t,"ready"),this.prewarmFilmstripFromProxy(t,r),Ke.debug(`Proxy ready for ${t}`)}catch(r){Ke.error(`Failed to load completed proxy for ${t}:`,r),this.emitStatusForProxyKey(t,"error")}}prewarmFilmstripFromProxy(t,r){const n=[...this.mediaIdsByProxyKey.get(t)??[]];if(n.length!==0)for(const i of n){const o=this.mediaResolver?.(i);if(!o||!o.mimeType.startsWith("video/")||o.duration<=0)continue;const a=Math.min(o.duration,S1);Gu(()=>this.filmstripPrewarm?.(i,r,o.duration,{startTime:0,endTime:a}),{priority:"warm",delayMs:x1});const s=Math.min(o.duration,I1);Gu(()=>this.filmstripPrewarm?.(i,r,o.duration,{startTime:0,endTime:s}),{priority:"warm",delayMs:w1})}}async refreshAllBlobUrls(){if(this.isRefreshing)return 0;const t=[...this.proxyBlobUrlByKey.keys()];if(t.length===0)return 0;this.isRefreshing=!0;let r=0;try{for(const n of t)try{const i=await this.readProxyFile(n);if(!i||i.size===0){this.releaseProxyUrl(n);continue}await this.setProxyUrl(n,i),r++}catch{this.releaseProxyUrl(n)}}catch(n){Ke.warn("Failed to refresh proxy blob URLs:",n)}finally{this.isRefreshing=!1}return r>0&&Ke.debug(`Refreshed ${r} proxy blob URLs from OPFS`),r}async readProxyFile(t){const r=await Zn(t);return cl(["media",r.mediaId,"proxy",oh],r)}async readProxyMetadata(t){const r=await Zn(t),n=await cl(["media",r.mediaId,"proxy",jk],r);return n?JSON.parse(await n.text()):null}async setProxyUrl(t,r){const n=Kk(await $u(t)),i=this.proxyBlobUrlByKey.get(t);i&&Wu(i);const o=URL.createObjectURL(r);Xs(o,r,{opfsPath:n,fileSize:r.size}),this.proxyBlobUrlByKey.set(t,o)}releaseProxyUrl(t){const r=this.proxyBlobUrlByKey.get(t);r&&Wu(r),this.proxyBlobUrlByKey.delete(t)}resolveProxyKey(t,r){return r||(this.proxyKeyByMediaId.get(t)??t)}normalizeSource(t){return typeof t=="function"?{load:t}:{load:async()=>t}}insertPendingJob(t){this.pendingJobsByKey.set(t.proxyKey,t);const r=this.pendingJobOrder.indexOf(t.proxyKey);if(r>=0&&this.pendingJobOrder.splice(r,1),t.priority==="background"){this.pendingJobOrder.push(t.proxyKey);return}const n=this.pendingJobOrder.findIndex(i=>this.pendingJobsByKey.get(i)?.priority==="background");if(n===-1){this.pendingJobOrder.push(t.proxyKey);return}this.pendingJobOrder.splice(n,0,t.proxyKey)}removePendingJob(t){this.pendingJobsByKey.delete(t);const r=this.pendingJobOrder.indexOf(t);r>=0&&this.pendingJobOrder.splice(r,1)}drainQueue(){for(;this.activeJobPhaseByKey.size<this.maxConcurrentJobs&&this.pendingJobOrder.length>0;){const t=this.pendingJobOrder.shift();if(!t)break;const r=this.pendingJobsByKey.get(t);r&&(this.pendingJobsByKey.delete(t),this.runQueuedJob(r))}}async runQueuedJob(t){const{proxyKey:r}=t;this.activeJobPriorityByKey.set(r,t.priority);try{const n=this.getWorker();this.activeJobPhaseByKey.set(r,"loading");let i=null;try{i=await t.source.load()}catch(a){if(!this.generatingProxyKeys.has(r)){this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.drainQueue();return}this.failQueuedJob(r,`Failed to load source for ${r}`,a);return}if(!this.generatingProxyKeys.has(r)){this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.drainQueue();return}if(!i){this.failQueuedJob(r,`Source media unavailable for ${r}`);return}const o=await $u(r);if(!this.generatingProxyKeys.has(r)){this.activeJobPhaseByKey.delete(r),this.activeJobPriorityByKey.delete(r),this.drainQueue();return}this.activeJobPhaseByKey.set(r,"processing"),n.postMessage({type:"generate",mediaId:r,dirSegments:o,source:i,sourceWidth:t.sourceWidth,sourceHeight:t.sourceHeight})}catch(n){this.failQueuedJob(r,`Failed to start proxy generation for ${r}`,n)}}failQueuedJob(t,r,n){this.activeJobPhaseByKey.delete(t),this.activeJobPriorityByKey.delete(t),this.generatingProxyKeys.delete(t),this.progressByProxyKey.delete(t),this.clearProgressEmissionState(t),n?Ke.error(r,n):Ke.error(r),this.emitStatusForProxyKey(t,"error"),this.drainQueue()}emitProgressThrottled(t,r){const n=Date.now(),i=this.progressEmissionByProxyKey.get(t)??{lastEmitAt:0,lastProgress:0,pendingProgress:null,timeoutId:null};if(r>=1||r<=0||r-i.lastProgress>=b1||n-i.lastEmitAt>=Vu){i.timeoutId!==null&&(clearTimeout(i.timeoutId),i.timeoutId=null),i.pendingProgress=null,i.lastEmitAt=n,i.lastProgress=r,this.progressEmissionByProxyKey.set(t,i),this.emitStatusForProxyKey(t,"generating",r);return}i.pendingProgress=r,i.timeoutId===null&&(i.timeoutId=setTimeout(()=>{const a=this.progressEmissionByProxyKey.get(t);if(!a||(a.timeoutId=null,a.pendingProgress==null))return;const s=a.pendingProgress;a.pendingProgress=null,a.lastEmitAt=Date.now(),a.lastProgress=s,this.progressEmissionByProxyKey.set(t,a),this.emitStatusForProxyKey(t,"generating",s)},Vu)),this.progressEmissionByProxyKey.set(t,i)}clearProgressEmissionState(t){const r=this.progressEmissionByProxyKey.get(t);r?.timeoutId!=null&&clearTimeout(r.timeoutId),this.progressEmissionByProxyKey.delete(t)}emitStatusForProxyKey(t,r,n){const i=this.mediaIdsByProxyKey.get(t);if(i&&i.size>0){for(const o of i)this.statusListener?.(o,r,n);return}this.statusListener?.(t,r,n)}}const Mt=new k1,yh={crossfade:{label:"Crossfade",description:"Smooth opacity blend between clips",defaultDuration:30,minDuration:30,maxDuration:90}};function pl(e){return Number.isFinite(e)?Math.max(1,Math.round(e)):1}function ml(e){return typeof e!="number"||!Number.isFinite(e)?.5:Math.max(0,Math.min(1,e))}function Zu(e){const t=e.timing==="spring"?"linear":e.timing;return{...e,durationInFrames:pl(e.durationInFrames),alignment:ml(e.alignment),timing:t??"linear"}}function P1(e,t){if(!e||e.length!==t.length)return!1;for(let r=0;r<t.length;r++)if(e[r]!==t[r])return!1;return!0}function T1(e,t){const r={};for(const i of e)(r[i.trackId]??=[]).push(i);const n={};for(const[i,o]of Object.entries(r)){const a=t[i];n[i]=a&&P1(a,o)?a:o}return n}function E1(e,t){const r={};for(const i of e){const o=Math.min(1,Math.max(0,i.alignment??.5)),a=Math.floor(i.durationInFrames*o),s=i.durationInFrames-a,l=r[i.leftClipId]??{left:0,right:0};l.right=Math.max(l.right,a),r[i.leftClipId]=l;const c=r[i.rightClipId]??{left:0,right:0};c.left=Math.max(c.left,s),r[i.rightClipId]=c}const n={};for(const[i,o]of Object.entries(r)){const a=t[i];n[i]=a&&a.left===o.left&&a.right===o.right?a:o}return n}function Wr(e,t){return{transitions:e,transitionsByTrackId:T1(e,t.transitionsByTrackId),transitionOverlapByItemId:E1(e,t.transitionOverlapByItemId)}}const q=_e()(e=>({transitions:[],transitionsByTrackId:{},transitionOverlapByItemId:{},setTransitions:t=>e(r=>{const n=t.map(i=>Zu(i));return Wr(n,r)}),_addTransition:(t,r,n,i="crossfade",o,a="fade",s,l=.5)=>{const c=yh[i],u=pl(o??c.defaultDuration),d=Z(),f={id:d,leftClipId:t,rightClipId:r,trackId:n,type:i,durationInFrames:u,presentation:a,timing:"linear",direction:s,alignment:ml(l)};return e(p=>{const m=[...p.transitions,f];return Wr(m,p)}),d},_updateTransition:(t,r)=>e(n=>{const i=n.transitions.map(o=>{if(o.id!==t)return o;const a={...r};return r.durationInFrames!==void 0&&(a.durationInFrames=pl(r.durationInFrames)),r.alignment!==void 0&&(a.alignment=ml(r.alignment)),Zu({...o,...a})});return Wr(i,n)}),_removeTransition:t=>e(r=>{const n=r.transitions.filter(i=>i.id!==t);return Wr(n,r)}),_removeTransitions:t=>e(r=>{const n=new Set(t),i=r.transitions.filter(o=>!n.has(o.id));return Wr(i,r)}),_removeTransitionsForItems:t=>e(r=>{const n=new Set(t),i=r.transitions.filter(o=>!n.has(o.leftClipId)&&!n.has(o.rightClipId));return Wr(i,r)})})),Ce=2,C1=new Set(["position","scale","anchor"]);function Ur(e){return C1.has(e)}const M1=new Set(["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius"]);function Oa(e){return M1.has(e)}const _1=new Set(["trimPathStart","trimPathEnd","trimPathOffset","taperStartWidth","taperEndWidth","taperStartLength","taperEndLength"]);function vh(e){return _1.has(e)}function F1(e){return Oa(e)||vh(e)}function bh(e){return F1(e)||Ur(e)}const hl={position:["x","y"],scale:["width","height"],anchor:["anchorX","anchorY"]};function A1(e){return hl[e]}function O1(e,t){return e===t?!0:Ur(e)?hl[e].includes(t):Ur(t)?hl[t].includes(e):!1}function Et(e){if(!e)return[];const t=(e.expressions??[]).filter(r=>r.type==="link");return[...e.propertyLinks??[],...t]}function rn(e){return(e?.expressions??[]).filter(t=>t.type==="expression")}function z1(e,t={}){return{...e,...t,value:{...e.value},...e.temporalEase&&{temporalEase:{...e.temporalEase.in&&{in:{...e.temporalEase.in}},...e.temporalEase.out&&{out:{...e.temporalEase.out}}}},...e.spatial&&{spatial:{...e.spatial,inTangent:{...e.spatial.inTangent},outTangent:{...e.spatial.outTangent}}}}}const ac={x:"X Position",y:"Y Position",width:"Width",height:"Height",anchorX:"Anchor X",anchorY:"Anchor Y",rotation:"Rotation",opacity:"Opacity",cornerRadius:"Layer Radius",cropLeft:"Crop Left",cropRight:"Crop Right",cropTop:"Crop Top",cropBottom:"Crop Bottom",cropSoftness:"Crop Softness",volume:"Volume (dB)",textStyleScale:"Style Preset Scale",fontSize:"Font Size",lineHeight:"Line Height",textPadding:"Text Padding",backgroundRadius:"Background Radius",textShadowOffsetX:"Shadow X",textShadowOffsetY:"Shadow Y",textShadowBlur:"Shadow Blur",strokeWidth:"Stroke Width",trimPathStart:"Trim Paths Start",trimPathEnd:"Trim Paths End",trimPathOffset:"Trim Paths Offset",taperStartWidth:"Taper Start Width",taperEndWidth:"Taper End Width",taperStartLength:"Taper Start Length",taperEndLength:"Taper End Length"},D1=new Set(Object.keys(ac));function R1(e){return D1.has(e)}function Sh(e,t,r){return`effect:${e}:${t}:${r}`}function aa(e){if(!e.startsWith("effect:"))return null;const[,t="",r="",n=""]=e.split(":");return!t||!r||!n?null:{gpuEffectType:t,effectId:r,paramKey:n}}function L1(e,t){return`pathVertex:${e}:${t}`}function B1(e){const t=/^pathVertex:(\d+):(positionX|positionY|inX|inY|outX|outY)$/.exec(e);return t?{vertexIndex:Number(t[1]),component:t[2]}:null}function U1(e){if(R1(e))return ac[e];const t=B1(e);if(t){const n={positionX:"X",positionY:"Y",inX:"In X",inY:"In Y",outX:"Out X",outY:"Out Y"};return`Vertex ${t.vertexIndex+1} ${n[t.component]}`}const r=aa(e);return r?r.paramKey:e}new Proxy({...ac},{get(e,t){if(typeof t=="string")return e[t]??U1(t)}});const Ih={tension:170,friction:26,mass:1},xh={x1:.42,y1:0,x2:.58,y2:1};function wh(e){const t={};for(const r of e)t[r.itemId]=r;return t}function es(e,t=new Set){const r=new Map;for(const n of e){const i=r.get(n.frame);if(!i){r.set(n.frame,n);continue}const o=t.has(i.id);(t.has(n.id)||!o)&&r.set(n.frame,n)}return Array.from(r.values()).sort((n,i)=>n.frame-i.frame)}function dt(e){return e.properties.some(r=>r.keyframes.length>0)||e.vectorProperties?.some(r=>r.keyframes.length>0)?!0:[e.separatedVectorProperties,e.propertyLinks,e.expressions].some(r=>r!==void 0&&r.length>0)}function Yu(e,t,r){const n=new Map(e.map(o=>[o.id,o.frame])),i=new Map;for(const o of e){const a={...o,frame:Math.min(r,Math.max(0,Math.round(o.frame*t)))},s=i.get(a.frame);(!s||(n.get(a.id)??-1)>(n.get(s.id)??-1))&&i.set(a.frame,a)}return Array.from(i.values()).sort((o,a)=>o.frame-a.frame)}const $=_e()((e,t)=>({keyframes:[],keyframesByItemId:{},setKeyframes:r=>e({keyframes:r,keyframesByItemId:wh(r)}),_addKeyframe:(r,n,i,o,a="linear",s)=>{const l=Z(),c={id:l,frame:i,value:o,easing:a,easingConfig:s};let u=l;return e(d=>{const f=d.keyframes.find(p=>p.itemId===r);if(f){const p=f.properties.find(m=>m.property===n);if(p){const m=p.keyframes.find(h=>h.frame===i);return m?(u=m.id,{keyframes:d.keyframes.map(h=>h.itemId===r?{...h,animationVersion:Ce,properties:h.properties.map(g=>g.property===n?{...g,keyframes:g.keyframes.map(v=>v.frame===i?{...v,value:o,easing:a,easingConfig:s}:v)}:g)}:h)}):{keyframes:d.keyframes.map(h=>h.itemId===r?{...h,animationVersion:Ce,properties:h.properties.map(g=>g.property===n?{...g,keyframes:[...g.keyframes,c].sort((v,x)=>v.frame-x.frame)}:g)}:h)}}return{keyframes:d.keyframes.map(m=>m.itemId===r?{...m,animationVersion:Ce,properties:[...m.properties,{property:n,keyframes:[c]}]}:m)}}return{keyframes:[...d.keyframes,{itemId:r,animationVersion:Ce,properties:[{property:n,keyframes:[c]}]}]}}),u},_addKeyframes:r=>{if(r.length===0)return[];const n=[];return e(i=>{let o=[...i.keyframes];for(const a of r){const{itemId:s,property:l,frame:c,value:u,easing:d="linear",easingConfig:f,source:p}=a,m=Z(),h={id:m,frame:c,value:u,easing:d,easingConfig:f,source:p},g=o.findIndex(v=>v.itemId===s);if(g!==-1){const v=o[g],x=v.properties.findIndex(b=>b.property===l);if(x!==-1){const b=v.properties[x],w=b.keyframes.findIndex(E=>E.frame===c);if(w!==-1){const E=[...b.keyframes];E[w]={...E[w],value:u,easing:d,easingConfig:f},n.push(E[w].id);const k=[...v.properties];k[x]={...b,keyframes:E},o=o.map((T,O)=>O===g?{...v,animationVersion:Ce,properties:k}:T)}else{const E=[...b.keyframes,h].sort((T,O)=>T.frame-O.frame);n.push(m);const k=[...v.properties];k[x]={...b,keyframes:E},o=o.map((T,O)=>O===g?{...v,animationVersion:Ce,properties:k}:T)}}else{const b=[...v.properties,{property:l,keyframes:[h]}];n.push(m),o=o.map((w,E)=>E===g?{...v,animationVersion:Ce,properties:b}:w)}}else n.push(m),o=[...o,{itemId:s,animationVersion:Ce,properties:[{property:l,keyframes:[h]}]}]}return{keyframes:o}}),n},_updateKeyframe:(r,n,i,o)=>e(a=>({keyframes:a.keyframes.map(s=>s.itemId===r?{...s,properties:s.properties.map(l=>l.property===n?{...l,keyframes:es(l.keyframes.map(c=>c.id===i?{...c,...o}:c),new Set([i]))}:l)}:s)})),_removeKeyframe:(r,n,i)=>e(o=>({keyframes:o.keyframes.map(a=>a.itemId===r?{...a,properties:a.properties.map(s=>s.property===n?{...s,keyframes:s.keyframes.filter(l=>l.id!==i)}:s).filter(s=>s.keyframes.length>0)}:a).filter(dt)})),_removeKeyframesForItem:r=>e(n=>({keyframes:n.keyframes.map(i=>i.itemId===r?{...i,properties:[],...i.vectorProperties&&{vectorProperties:[]}}:i).filter(dt)})),_removeKeyframesForItems:r=>e(n=>{const i=new Set(r);return{keyframes:n.keyframes.filter(o=>!i.has(o.itemId)).map(o=>({...o,propertyLinks:o.propertyLinks?.filter(a=>!i.has(a.sourceItemId)),expressions:o.expressions?.filter(a=>a.type!=="link"||!i.has(a.sourceItemId))})).filter(dt)}}),_removeKeyframesForProperty:(r,n)=>e(i=>({keyframes:i.keyframes.map(o=>o.itemId===r?{...o,properties:o.properties.filter(a=>a.property!==n)}:o).filter(dt)})),_removeVectorKeyframesForProperty:(r,n)=>e(i=>({keyframes:i.keyframes.map(o=>o.itemId===r?{...o,vectorProperties:o.vectorProperties?.filter(a=>a.property!==n)}:o).filter(dt)})),_removeKeyframesByApplication:(r,n)=>e(i=>({keyframes:i.keyframes.map(o=>o.itemId===r?{...o,properties:o.properties.map(a=>({...a,keyframes:a.keyframes.filter(s=>s.source?.applicationId!==n)})).filter(a=>a.keyframes.length>0),vectorProperties:o.vectorProperties?.map(a=>({...a,keyframes:a.keyframes.filter(s=>s.source?.applicationId!==n)})).filter(a=>a.keyframes.length>0)}:o).filter(dt)})),_removeManualKeyframes:r=>e(n=>({keyframes:n.keyframes.map(i=>i.itemId===r?{...i,properties:i.properties.map(o=>({...o,keyframes:o.keyframes.filter(a=>a.source)})).filter(o=>o.keyframes.length>0),vectorProperties:i.vectorProperties?.map(o=>({...o,keyframes:o.keyframes.filter(a=>a.source)})).filter(o=>o.keyframes.length>0)}:i).filter(dt)})),_setDirectPropertyLink:(r,n)=>e(i=>i.keyframes.find(a=>a.itemId===r)?{keyframes:i.keyframes.map(a=>a.itemId===r?{...a,animationVersion:Ce,propertyLinks:[...Et(a).filter(s=>!O1(s.targetProperty,n.targetProperty)),n],expressions:[...rn(a)]}:a)}:{keyframes:[...i.keyframes,{itemId:r,animationVersion:Ce,properties:[],propertyLinks:[n]}]}),_removeDirectPropertyLink:(r,n)=>e(i=>({keyframes:i.keyframes.map(o=>o.itemId===r?{...o,propertyLinks:Et(o).filter(a=>a.targetProperty!==n),expressions:[...rn(o)]}:o).filter(dt)})),_setPropertyExpression:(r,n)=>e(i=>i.keyframes.find(a=>a.itemId===r)?{keyframes:i.keyframes.map(a=>a.itemId===r?{...a,animationVersion:Ce,propertyLinks:[...Et(a)],expressions:[...rn(a).filter(s=>s.targetProperty!==n.targetProperty),n]}:a)}:{keyframes:[...i.keyframes,{itemId:r,animationVersion:Ce,properties:[],expressions:[n]}]}),_removePropertyExpression:(r,n)=>e(i=>({keyframes:i.keyframes.map(o=>o.itemId===r?{...o,propertyLinks:[...Et(o)],expressions:rn(o).filter(a=>a.targetProperty!==n)}:o).filter(dt)})),_upsertVectorKeyframe:(r,n,i)=>{const o=Z();let a=o;const s={id:o,frame:i.frame,value:i.value,easing:i.easing??"linear",easingConfig:i.easingConfig,temporalEase:i.temporalEase,spatial:i.spatial,source:i.source};return e(l=>{const c=l.keyframes.find(f=>f.itemId===r);if(!c)return{keyframes:[...l.keyframes,{itemId:r,animationVersion:Ce,properties:[],vectorProperties:[{property:n,keyframes:[s]}]}]};const u=c.vectorProperties?.find(f=>f.property===n),d=u?.keyframes.find(f=>f.frame===i.frame);return d&&(a=d.id),{keyframes:l.keyframes.map(f=>{if(f.itemId!==r)return f;const p=f.vectorProperties??[];return u?{...f,animationVersion:Ce,vectorProperties:p.map(m=>m.property===n?{...m,keyframes:d?m.keyframes.map(h=>h.frame===i.frame?{...s,id:h.id}:h):[...m.keyframes,s].sort((h,g)=>h.frame-g.frame)}:m)}:{...f,animationVersion:Ce,vectorProperties:[...p,{property:n,keyframes:[s]}]}})}}),a},_updateVectorKeyframe:(r,n,i,o)=>e(a=>({keyframes:a.keyframes.map(s=>s.itemId===r?{...s,animationVersion:Ce,vectorProperties:s.vectorProperties?.map(l=>l.property===n?{...l,keyframes:es(l.keyframes.map(c=>c.id===i?{...c,...o}:c),new Set([i]))}:l)}:s)})),_removeVectorKeyframe:(r,n,i)=>e(o=>({keyframes:o.keyframes.map(a=>a.itemId===r?{...a,vectorProperties:a.vectorProperties?.map(s=>s.property===n?{...s,keyframes:s.keyframes.filter(l=>l.id!==i)}:s).filter(s=>s.keyframes.length>0)}:a).filter(dt)})),_replaceScalarPropertiesWithVectorProperty:(r,n,i)=>e(o=>{const a=new Set(i);return o.keyframes.find(l=>l.itemId===r)?{keyframes:o.keyframes.map(l=>l.itemId===r?{...l,animationVersion:Ce,properties:l.properties.filter(c=>!a.has(c.property)),vectorProperties:[...(l.vectorProperties??[]).filter(c=>c.property!==n.property),n]}:l)}:{keyframes:[...o.keyframes,{itemId:r,animationVersion:Ce,properties:[],vectorProperties:[n]}]}}),_setVectorDimensionsSeparated:(r,n,i)=>e(o=>{const a=new Set(A1(n)),s=c=>{const u=new Set(c.separatedVectorProperties??[]);i.separated?u.add(n):u.delete(n);const d=i.separated?(i.scalarProperties??[]).map(p=>({...p,keyframes:p.keyframes.map(m=>({...m}))})):[],f=(c.vectorProperties??[]).filter(p=>p.property!==n);return!i.separated&&i.vectorProperty&&f.push(i.vectorProperty),{...c,animationVersion:Ce,properties:[...c.properties.filter(p=>!a.has(p.property)),...d],vectorProperties:f,separatedVectorProperties:Array.from(u)}};return o.keyframes.find(c=>c.itemId===r)?{keyframes:o.keyframes.map(c=>c.itemId===r?s(c):c)}:{keyframes:[...o.keyframes,s({itemId:r,animationVersion:Ce,properties:[]})]}}),_scaleKeyframesForItem:(r,n,i)=>{if(n===i||n<=0||i<=0)return;const o=i/n,a=i-1;e(s=>s.keyframes.find(c=>c.itemId===r)?{keyframes:s.keyframes.map(c=>c.itemId!==r?c:{...c,properties:c.properties.map(u=>u.keyframes.length===0?u:{...u,keyframes:Yu(u.keyframes,o,a)}),vectorProperties:c.vectorProperties?.map(u=>({...u,keyframes:Yu(u.keyframes,o,a)}))})}:s)},_updateKeyframes:r=>{if(r.length===0)return;const n=new Map;for(const i of r){const o=`${i.itemId}:${i.property}`;n.has(o)||n.set(o,new Set),n.get(o).add(i.keyframeId)}e(i=>{let o=[...i.keyframes];for(const a of r)o=o.map(s=>s.itemId===a.itemId?{...s,properties:s.properties.map(l=>l.property===a.property?{...l,keyframes:es(l.keyframes.map(c=>c.id===a.keyframeId?{...c,...a.updates}:c),n.get(`${a.itemId}:${a.property}`)??new Set)}:l)}:s);return{keyframes:o}})},_moveKeyframes:r=>{if(r.length===0)return;const n=r.map(i=>({itemId:i.ref.itemId,property:i.ref.property,keyframeId:i.ref.keyframeId,updates:{frame:Math.max(0,i.newFrame)}}));t()._updateKeyframes(n)},_removeKeyframes:r=>{if(r.length===0)return;const n=new Map;for(const i of r){const o=`${i.itemId}:${i.property}`;n.has(o)||n.set(o,new Set),n.get(o).add(i.keyframeId)}e(i=>({keyframes:i.keyframes.map(o=>({...o,properties:o.properties.map(a=>{const s=n.get(`${o.itemId}:${a.property}`);return s?{...a,keyframes:a.keyframes.filter(l=>!s.has(l.id))}:a}).filter(a=>a.keyframes.length>0)})).filter(dt)}))},_duplicateKeyframes:(r,n,i,o)=>{if(r.length===0)return[];const a=[],s=t(),l=[];for(const c of r){const u=s.keyframes.find(p=>p.itemId===c.itemId);if(!u)continue;const d=u.properties.find(p=>p.property===c.property);if(!d)continue;const f=d.keyframes.find(p=>p.id===c.keyframeId);f&&l.push({property:o??c.property,keyframe:f,targetItemId:i??c.itemId})}for(const{property:c,keyframe:u,targetItemId:d}of l){const f=s._addKeyframe(d,c,u.frame+n,u.value,u.easing,u.easingConfig);a.push(f)}return a},getKeyframesForItem:r=>t().keyframesByItemId[r],getKeyframeById:(r,n,i)=>{const o=t().keyframesByItemId[r];if(!o)return;const a=o.properties.find(s=>s.property===n);if(a)return a.keyframes.find(s=>s.id===i)},getAllKeyframesForProperty:(r,n)=>{const i=t().keyframesByItemId[r];if(!i)return[];const o=i.properties.find(a=>a.property===n);return o?o.keyframes:[]},hasKeyframesAtFrame:(r,n,i)=>{const o=t().keyframesByItemId[r];if(!o)return!1;const a=o.properties.find(s=>s.property===n);return a?a.keyframes.some(s=>s.frame===i):!1},getVectorKeyframesForProperty:(r,n)=>t().keyframesByItemId[r]?.vectorProperties?.find(i=>i.property===n)?.keyframes??[]}));$.subscribe((e,t)=>{e.keyframes!==t.keyframes&&e.keyframesByItemId===t.keyframesByItemId&&$.setState({keyframesByItemId:wh(e.keyframes)})});var Xu=(e,t,r)=>(i,o)=>({pastStates:r?.pastStates||[],futureStates:r?.futureStates||[],undo:(a=1)=>{if(o().pastStates.length){const s=r?.partialize?.(t())||t(),l=o().pastStates.splice(-a,a),c=l.shift();e(c),i({pastStates:o().pastStates,futureStates:o().futureStates.concat(r?.diff?.(s,c)||s,l.reverse())})}},redo:(a=1)=>{if(o().futureStates.length){const s=r?.partialize?.(t())||t(),l=o().futureStates.splice(-a,a),c=l.shift();e(c),i({pastStates:o().pastStates.concat(r?.diff?.(s,c)||s,l.reverse()),futureStates:o().futureStates})}},clear:()=>i({pastStates:[],futureStates:[]}),isTracking:!0,pause:()=>i({isTracking:!1}),resume:()=>i({isTracking:!0}),setOnSave:a=>i({_onSave:a}),_onSave:r?.onSave,_handleSet:(a,s,l,c)=>{r?.limit&&o().pastStates.length>=r?.limit&&o().pastStates.shift(),o()._onSave?.(a,l),i({pastStates:o().pastStates.concat(c||a),futureStates:[]})}}),N1=(e,t)=>(n,i,o)=>{o.temporal=Z0(t?.wrapTemporal?.(Xu(n,i,t))||Xu(n,i,t));const a=t?.handleSet?.(o.temporal.getState()._handleSet)||o.temporal.getState()._handleSet,s=c=>{if(!o.temporal.getState().isTracking)return;const u=t?.partialize?.(i())||i(),d=t?.diff?.(c,u);d===null||t?.equality?.(c,u)||a(c,void 0,u,d)},l=o.setState;return o.setState=(...c)=>{const u=t?.partialize?.(i())||i();l(...c),s(u)},e((...c)=>{const u=t?.partialize?.(i())||i();n(...c),s(u)},i,o)};const wi={PLAY_PAUSE:"space",PREVIOUS_FRAME:"left",NEXT_FRAME:"right",GO_TO_START:"home",GO_TO_END:"end",NEXT_SNAP_POINT:"down",PREVIOUS_SNAP_POINT:"up",SPLIT_AT_PLAYHEAD_ALT:"alt+c",JOIN_ITEMS:"shift+j",DELETE_SELECTED:"delete",DELETE_SELECTED_ALT:"backspace",RIPPLE_DELETE:"mod+delete",RIPPLE_DELETE_ALT:"mod+backspace",FREEZE_FRAME:"shift+f",LINK_AUDIO_VIDEO:"mod+alt+l",UNLINK_AUDIO_VIDEO:"alt+shift+l",TOGGLE_LINKED_SELECTION:"shift+l",NUDGE_LEFT:"shift+left",NUDGE_RIGHT:"shift+right",NUDGE_UP:"shift+up",NUDGE_DOWN:"shift+down",NUDGE_LEFT_LARGE:"mod+shift+left",NUDGE_RIGHT_LARGE:"mod+shift+right",NUDGE_UP_LARGE:"mod+shift+up",NUDGE_DOWN_LARGE:"mod+shift+down",UNDO:"mod+z",REDO:"mod+shift+z",ZOOM_IN:"mod+equal",ZOOM_OUT:"mod+minus",ZOOM_TO_FIT:"backslash",ZOOM_TO_100:"shift+backslash",ZOOM_TO_100_ALT:"mod+0",COPY:"mod+c",CUT:"mod+x",PASTE:"mod+v",SELECTION_TOOL:"v",TRIM_EDIT_TOOL:"t",RAZOR_TOOL:"c",SPLIT_AT_CURSOR:"shift+c",RATE_STRETCH_TOOL:"r",SLIP_TOOL:"y",SLIDE_TOOL:"u",SAVE:"mod+s",EXPORT:"mod+shift+e",TOGGLE_SNAP:"s",TOGGLE_CANVAS_SNAP:"shift+s",OPEN_SCENE_BROWSER:"mod+shift+f",WORKSPACE_EDIT:"alt+1",WORKSPACE_COLOR:"alt+2",WORKSPACE_ANIMATE:"alt+3",ADD_MARKER:"m",REMOVE_MARKER:"shift+m",PREVIOUS_MARKER:"bracketleft",NEXT_MARKER:"bracketright",CLEAR_KEYFRAMES:"shift+a",KEYFRAME_EDITOR_GRAPH:"1",KEYFRAME_EDITOR_DOPESHEET:"2",KEYFRAME_EDITOR_SPLIT:"3",EDIT_KEYFRAME_ADD:"k",KEYFRAME_PREVIOUS:"alt+bracketleft",KEYFRAME_NEXT:"alt+bracketright",KEYFRAME_TOGGLE_AUTO:"a",KEYFRAME_FIT:"f",MARK_IN:"i",MARK_OUT:"o",CLEAR_IN_OUT:"alt+x",INSERT_EDIT:"comma",OVERWRITE_EDIT:"period"},kh=["mod","alt","shift"],Ph=new Set(kh),Qu=new Map(kh.map((e,t)=>[e,t])),$1={cmd:"mod",command:"mod",ctrl:"mod",control:"mod",option:"alt",return:"enter",esc:"escape",del:"delete","=":"equal",equals:"equal","-":"minus",arrowleft:"left",arrowright:"right",arrowup:"up",arrowdown:"down"},H1=[{binding:"alt+left",browserAction:"Back navigation"},{binding:"alt+right",browserAction:"Forward navigation"},{binding:"f5",browserAction:"Reload page"},{binding:"mod+r",browserAction:"Reload page"},{binding:"mod+shift+r",browserAction:"Hard reload page"},{binding:"mod+t",browserAction:"New tab"},{binding:"mod+shift+t",browserAction:"Reopen closed tab"},{binding:"mod+w",browserAction:"Close tab"},{binding:"mod+n",browserAction:"New window"},{binding:"mod+shift+n",browserAction:"New private window"},{binding:"mod+l",browserAction:"Focus address bar"},{binding:"mod+shift+l",browserAction:"Focus address bar or search in some browsers"},{binding:"mod+d",browserAction:"Bookmark page or focus address bar"},{binding:"mod+e",browserAction:"Focus search or address bar in some browsers"},{binding:"mod+p",browserAction:"Print page"},{binding:"mod+f",browserAction:"Find in page"},{binding:"mod+equal",browserAction:"Browser zoom in"},{binding:"mod+minus",browserAction:"Browser zoom out"},{binding:"mod+0",browserAction:"Reset browser zoom"},{binding:"mod+1",browserAction:"Switch to tab 1"},{binding:"mod+2",browserAction:"Switch to tab 2"},{binding:"mod+3",browserAction:"Switch to tab 3"},{binding:"mod+4",browserAction:"Switch to tab 4"},{binding:"mod+5",browserAction:"Switch to tab 5"},{binding:"mod+6",browserAction:"Switch to tab 6"},{binding:"mod+7",browserAction:"Switch to tab 7"},{binding:"mod+8",browserAction:"Switch to tab 8"},{binding:"mod+9",browserAction:"Switch to last tab"}];new Map(H1.map(e=>[e.binding,e]));const j1={PLAY_PAUSE:"Play/Pause",PREVIOUS_FRAME:"Previous frame",NEXT_FRAME:"Next frame",GO_TO_START:"Go to start",GO_TO_END:"Go to end",NEXT_SNAP_POINT:"Next snap point",PREVIOUS_SNAP_POINT:"Previous snap point",SPLIT_AT_PLAYHEAD_ALT:"Split at playhead",JOIN_ITEMS:"Join selected clips",DELETE_SELECTED:"Delete selected items",DELETE_SELECTED_ALT:"Delete selected items (alternative)",RIPPLE_DELETE:"Ripple delete selected items",RIPPLE_DELETE_ALT:"Ripple delete selected items (alternative)",FREEZE_FRAME:"Insert freeze frame at playhead",LINK_AUDIO_VIDEO:"Link selected clips",UNLINK_AUDIO_VIDEO:"Unlink selected clips",TOGGLE_LINKED_SELECTION:"Toggle linked selection",NUDGE_LEFT:"Nudge selected visual items left (1px)",NUDGE_RIGHT:"Nudge selected visual items right (1px)",NUDGE_UP:"Nudge selected visual items up (1px)",NUDGE_DOWN:"Nudge selected visual items down (1px)",NUDGE_LEFT_LARGE:"Nudge selected visual items left (10px)",NUDGE_RIGHT_LARGE:"Nudge selected visual items right (10px)",NUDGE_UP_LARGE:"Nudge selected visual items up (10px)",NUDGE_DOWN_LARGE:"Nudge selected visual items down (10px)",UNDO:"Undo",REDO:"Redo",ZOOM_IN:"Zoom in timeline",ZOOM_OUT:"Zoom out timeline",ZOOM_TO_FIT:"Zoom to fit all content",ZOOM_TO_100:"Zoom to 100% at cursor or playhead",ZOOM_TO_100_ALT:"Zoom to 100% at cursor or playhead (alternative)",COPY:"Copy selected items or keyframes",CUT:"Cut selected items or keyframes",PASTE:"Paste items or keyframes",SELECTION_TOOL:"Selection tool",TRIM_EDIT_TOOL:"Trim edit tool",RAZOR_TOOL:"Razor tool",SPLIT_AT_CURSOR:"Split at cursor",RATE_STRETCH_TOOL:"Rate stretch tool",SLIP_TOOL:"Slip tool",SLIDE_TOOL:"Slide tool",SAVE:"Save project",EXPORT:"Export video",TOGGLE_SNAP:"Toggle snap",TOGGLE_CANVAS_SNAP:"Toggle canvas (gizmo) snap",OPEN_SCENE_BROWSER:"Open Scene Browser (search AI captions)",WORKSPACE_EDIT:"Switch to Edit workspace",WORKSPACE_COLOR:"Switch to Color workspace",WORKSPACE_ANIMATE:"Switch to Motion workspace",ADD_MARKER:"Add marker at playhead",REMOVE_MARKER:"Remove selected marker",PREVIOUS_MARKER:"Jump to previous marker",NEXT_MARKER:"Jump to next marker",CLEAR_KEYFRAMES:"Clear all keyframes from selected items",KEYFRAME_EDITOR_GRAPH:"Switch keyframe editor to graph view",KEYFRAME_EDITOR_DOPESHEET:"Switch keyframe editor to dopesheet view",KEYFRAME_EDITOR_SPLIT:"Switch keyframe editor to split view",EDIT_KEYFRAME_ADD:"Add keyframe at playhead for selected Edit layer",KEYFRAME_PREVIOUS:"Jump to previous property keyframe",KEYFRAME_NEXT:"Jump to next property keyframe",KEYFRAME_TOGGLE_AUTO:"Toggle auto-key for active property",KEYFRAME_FIT:"Fit selected keyframes in view",MARK_IN:"Mark In point",MARK_OUT:"Mark Out point",CLEAR_IN_OUT:"Clear In/Out points",INSERT_EDIT:"Insert edit",OVERWRITE_EDIT:"Overwrite edit"};W1();function q1(e){return e.trim()===""}function K1(e){return e in wi}function G1(e){return e.trim().toLowerCase()}function W1(){const e=new Map,t=new Map;for(const r of Object.keys(wi))e.set(G1(j1[r]),r),t.set(sc(wi[r]),r);return{byLabel:e,byDefaultBinding:t}}function V1(e){const t=e.trim().toLowerCase();return t?$1[t]??t:""}function Th(e){return e.split("+").map(t=>V1(t)).filter(Boolean)}function sc(e){const t=new Set,r=[];for(const i of Th(e)){if(Ph.has(i)){t.add(i);continue}r.includes(i)||r.push(i)}return[...Array.from(t).sort((i,o)=>(Qu.get(i)??99)-(Qu.get(o)??99)),...r].join("+")}function Ju(e){if(!e||typeof e!="object")return{};const t={};for(const[r,n]of Object.entries(e)){if(!K1(r)||typeof n!="string")continue;if(q1(n)){t[r]="";continue}const i=sc(n);!i||!Z1(i)||i!==wi[r]&&(t[r]=i)}return t}function Z1(e){return Th(e).some(t=>!Ph.has(t))}const Eh=[{id:"netflix",label:"Netflix",hintKey:"editor.captionPresets.netflixHint",patch:{fontFamily:"Inter",fontWeight:"semibold",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:"rgba(0, 0, 0, 0.55)",backgroundRadius:4,textAlign:"center",verticalAlign:"middle",lineHeight:1.15,letterSpacing:0,textPadding:12,textShadow:{offsetX:0,offsetY:2,blur:6,color:"rgba(0, 0, 0, 0.6)"},stroke:void 0},layout:{fontSizeRatio:.04,yRatio:.36,widthRatio:.7,heightRatio:.16}},{id:"youtube",label:"YouTube",hintKey:"editor.captionPresets.youtubeHint",patch:{fontFamily:"Roboto",fontWeight:"medium",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.2,letterSpacing:0,textPadding:0,textShadow:{offsetX:0,offsetY:4,blur:14,color:"rgba(0, 0, 0, 0.85)"},stroke:void 0},layout:{fontSizeRatio:.045,yRatio:.34,widthRatio:.85,heightRatio:.18}},{id:"bold-yellow",label:"Bold Yellow",hintKey:"editor.captionPresets.boldYellowHint",patch:{fontFamily:"Roboto Slab",fontWeight:"bold",fontStyle:"normal",underline:!1,color:"#FFD400",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.1,letterSpacing:0,textPadding:0,textShadow:{offsetX:0,offsetY:3,blur:5,color:"rgba(0, 0, 0, 1)"},stroke:{width:1.5,color:"#000000"}},layout:{fontSizeRatio:.05,yRatio:.38,widthRatio:.85,heightRatio:.18}},{id:"minimal-stroke",label:"Outlined",hintKey:"editor.captionPresets.outlinedHint",patch:{fontFamily:"Manrope",fontWeight:"medium",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.2,letterSpacing:0,textPadding:0,textShadow:void 0,stroke:{width:1,color:"#000000"}},layout:{fontSizeRatio:.04,yRatio:.34,widthRatio:.85,heightRatio:.16}},{id:"tiktok",label:"TikTok",hintKey:"editor.captionPresets.tiktokHint",patch:{fontFamily:"Anton",fontWeight:"normal",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:void 0,backgroundRadius:0,textAlign:"center",verticalAlign:"middle",lineHeight:1.05,letterSpacing:1,textPadding:0,textShadow:{offsetX:0,offsetY:4,blur:8,color:"rgba(0, 0, 0, 0.9)"},stroke:{width:2,color:"#000000"}},layout:{fontSizeRatio:.075,yRatio:0,widthRatio:.9,heightRatio:.22}}];function Y1(e){return e==="semantic"?"semantic":"keyword"}const Ch=Eh[0]?.id??"netflix";function ed(e){return typeof e=="string"&&Eh.some(t=>t.id===e)?e:Ch}const X1={seconds:{min:.5,max:60},frames:{min:1,max:1800}},Mh=3;function td(e){return e==="frames"?"frames":"seconds"}function ts(e,t){const r=X1[t],n=t==="seconds"?Mh:90,i=typeof e=="number"&&Number.isFinite(e)?e:n;return Math.min(r.max,Math.max(r.min,i))}function Q1(e,t){const r=Object.keys(e),n=Object.keys(t);return r.length!==n.length?!1:r.every(i=>e[i]===t[i])}const rd={snapEnabled:!0,timelineSectionDividerPosition:null,canvasSnapEnabled:!0,showWaveforms:!0,showFilmstrips:!0,enableFilmstripExtraction:!0,editorDensity:Y0,maxUndoHistory:50,autoSaveInterval:5,captioningIntervalUnit:"seconds",captioningIntervalValue:Mh,captionSearchMode:"keyword",defaultCaptionStylePresetId:Ch,hotkeyOverrides:{}},_h=_e()(vp(e=>({...rd,setSetting:(t,r)=>e(n=>{if(t==="captioningIntervalUnit"){const i=td(r);return{captioningIntervalUnit:i,captioningIntervalValue:ts(n.captioningIntervalValue,i)}}return t==="captioningIntervalValue"?{captioningIntervalValue:ts(r,n.captioningIntervalUnit)}:t==="editorDensity"?{editorDensity:Nc(r)}:t==="defaultCaptionStylePresetId"?{defaultCaptionStylePresetId:ed(r)}:{[t]:r}}),setHotkeyBinding:(t,r)=>e(n=>{const i=sc(r);if(!i||i===wi[t]){if(!(t in n.hotkeyOverrides))return n;const o={...n.hotkeyOverrides};return delete o[t],{hotkeyOverrides:o}}return n.hotkeyOverrides[t]===i?n:{hotkeyOverrides:{...n.hotkeyOverrides,[t]:i}}}),unbindHotkeyBinding:t=>e(r=>r.hotkeyOverrides[t]===""?r:{hotkeyOverrides:{...r.hotkeyOverrides,[t]:""}}),replaceHotkeyOverrides:t=>e(r=>{const n=Ju(t);return Q1(r.hotkeyOverrides,n)?r:{hotkeyOverrides:n}}),resetHotkeyBinding:t=>e(r=>{if(!(t in r.hotkeyOverrides))return r;const n={...r.hotkeyOverrides};return delete n[t],{hotkeyOverrides:n}}),resetHotkeys:()=>e(t=>Object.keys(t.hotkeyOverrides).length===0?t:{hotkeyOverrides:{}}),resetToDefaults:()=>e(rd)}),{name:"freecut-settings",version:2,migrate:(e,t)=>{let r=e??{};return t<1&&(r.autoSaveInterval==null||r.autoSaveInterval<=0)&&(r={...r,autoSaveInterval:5}),r},merge:(e,t)=>{const r=e??{},n=td(r.captioningIntervalUnit);return{...t,...r,hotkeyOverrides:Ju(r.hotkeyOverrides),editorDensity:Nc(r.editorDensity),captioningIntervalUnit:n,captioningIntervalValue:ts(r.captioningIntervalValue,n),captionSearchMode:Y1(r.captionSearchMode),defaultCaptionStylePresetId:ed(r.defaultCaptionStylePresetId)}}})),J1={conflict:new Set},ki=_e()(e=>({conflicts:{},reportConflict:t=>{e(r=>({conflicts:{...r.conflicts,[t.projectId]:t}}));for(const r of[...J1.conflict])r(t)},clearConflict:t=>e(r=>{if(!(t in r.conflicts))return r;const n={...r.conflicts};return delete n[t],{conflicts:n}})}));function e2(e){return ki.getState().conflicts[e]}const Vt=new Map,io=new Map;async function lc(e,t){const n=(io.get(e)??Promise.resolve()).catch(()=>{}).then(t),i=n.catch(()=>{});io.set(e,i);try{return await n}finally{io.get(e)===i&&io.delete(e)}}const cc=e=>`project:${e}`;function t2(e){return e instanceof $t&&e.code==="not_found"}function r2(e){const t=e2(e);if(t)throw new Xo("Dự án đã được thay đổi ở nơi khác; hãy tải lại hoặc giữ bản của bạn trước khi lưu tiếp.",t.serverRevision)}function Fh(e){return lc(cc(e),async()=>{try{const{data:t,revision:r}=await vt().loadProject(e);return Vt.set(e,{project:t,revision:r}),ki.getState().clearConflict(e),t}catch(t){if(!t2(t))throw t;Vt.delete(e),ki.getState().clearConflict(e);return}})}function n2(e){return Vt.get(e)?.project}async function i2(e){return Vt.get(e)?.project??Fh(e)}function Ah(e,t){return lc(cc(e),async()=>{r2(e);const r=Vt.get(e);if(!r)throw new Error(`Project not loaded: ${e}`);const n={...r.project,...t,id:e,updatedAt:Date.now()};let i;try{i=await vt().saveProject(n,r.revision)}catch(o){throw Kv(o)&&ki.getState().reportConflict({projectId:e,baseRevision:r.revision,serverRevision:o.serverRevision,timestamp:Date.now()}),o}return Vt.set(e,{project:n,revision:i}),n})}async function o2(e){const{data:t,revision:r}=await vt().createProject(e);return Vt.set(t.id,{project:t,revision:r}),t}async function a2(e,t){const{data:r,revision:n}=await vt().duplicateProject(e,t);return Vt.set(r.id,{project:r,revision:n}),r}function Oh(e){return lc(cc(e),async()=>{await vt().deleteProject(e),Vt.delete(e),ki.getState().clearConflict(e)})}const Nr=He("WorkspaceFS:Projects"),s2=6;async function l2(){const e=vt();try{const t=await e.listProjects(),r=[];let n=0;const i=async()=>{for(;n<t.length;){const o=t[n++];try{r.push((await e.loadProject(o.id)).data)}catch(a){if(!(a instanceof $t&&a.code==="not_found"))throw a;Nr.warn(`getAllProjects: project ${o.id} disappeared`,a)}}};return await Promise.all(Array.from({length:Math.min(s2,t.length)},i)),r.sort((o,a)=>a.updatedAt-o.updatedAt)}catch(t){throw Nr.error("getAllProjects failed",t),new Error("Failed to load projects from server",{cause:t})}}async function c2(e){try{return await Fh(e)}catch(t){throw Nr.error(`getProject(${e}) failed`,t),t}}async function u2(e){const t=await o2({name:e.name}),r={...t.metadata,width:e.width,height:e.height,fps:e.fps,...e.backgroundColor!==void 0&&{backgroundColor:e.backgroundColor}},n=e.description??"";if(n===t.description&&r.width===t.metadata.width&&r.height===t.metadata.height&&r.fps===t.metadata.fps&&r.backgroundColor===t.metadata.backgroundColor)return t;try{return await Ah(t.id,{description:n,metadata:r}),n2(t.id)??t}catch(o){throw await Oh(t.id).catch(a=>{Nr.warn(`createProject: could not remove ${t.id} after failed setup`,a)}),o}}async function d2(e,t){try{return await Ah(e,t)}catch(r){throw Nr.error(`updateProject(${e}) failed`,r),r}}async function f2(e,t){try{return await a2(e,t)}catch(r){throw Nr.error(`duplicateProject(${e}) failed`,r),r}}async function p2(e){try{await Oh(e)}catch(t){throw Nr.error(`deleteProject(${e}) failed`,t),new Error(`Failed to delete project: ${e}`,{cause:t})}}const oo=new Map;async function zh(e,t){const i=(oo.get(e)??Promise.resolve()).catch(()=>{}).then(t),o=i.catch(()=>{});oo.set(e,o);try{return await i}finally{oo.get(e)===o&&oo.delete(e)}}const nd=new Map,gl=new Set;function Dh(e,t){const r=nd.get(e);if(nd.set(e,t),r===void 0||r===t)return!1;for(const n of[...gl])n({mediaId:e,previousHash:r,contentHash:t});return!0}function m2(e){return gl.add(e),()=>{gl.delete(e)}}const rs=new Map;function uc(e){const t=rs.get(e);if(t)return t;const r=nh(e).then(n=>(Dh(e,n.contentHash),n)).finally(()=>{rs.delete(e)});return rs.set(e,r),r}async function gD(e){try{return await uc(e)}catch(t){if(t instanceof $t&&t.code==="not_found")return null;throw t}}const h2=["duration","width","height","fps","codec","bitrate","audioCodec","audioCodecSupported","videoCodecSupported","keyframeTimestamps","gopInterval","thumbnailId","aiCaptions"],Rh="derived.json",g2=Object.fromEntries(h2.map(e=>[e,!0]));function Lh(e){return g2[e]===!0}async function Bh(e,t){return Dh(e,t),{workspaceId:await vt().workspaceId(),mediaId:e,contentHash:t}}function y2(e){if(typeof e!="object"||e===null)return{};const t={};for(const[r,n]of Object.entries(e))Lh(r)&&n!==void 0&&(t[r]=n);return t}async function v2(e,t){const r=await cl(["media",e,Rh],await Bh(e,t));if(!r)return{};try{return y2(JSON.parse(await r.text()))}catch{return{}}}async function yD(e,t,r){return zh(`media-derived:${e}`,async()=>{const n={...await v2(e,t)};for(const[i,o]of Object.entries(r))Lh(i)&&(o===void 0?delete n[i]:n[i]=o);return await Hk(["media",e,Rh],JSON.stringify(n),await Bh(e,t)),n})}async function vD(e){await ul(["media",e],!0)}function $r(){return $k}const b2=He("CacheFS");class Uh extends Error{constructor(t,r){super(`Corrupt JSON at ${t}`),this.path=t,this.name="WorkspaceFileCorruptError",this.cause=r}path}const S2=new Set(["NotFoundError","NotReadableError","TypeMismatchError"]),I2=new Set(["QuotaExceededError","NotFoundError","NotReadableError","NoModificationAllowedError","InvalidStateError"]);function _t(e){return e instanceof $t?e.code==="not_found":e instanceof DOMException&&S2.has(e.name)}function x2(e){return e instanceof $t?e.code==="not_found":e instanceof DOMException&&I2.has(e.name)}async function dc(e,t,r,n){const i=t[0]==="media"?t[1]:void 0,o=n??(i?await uc(i):void 0);return Di(t,r,o)}async function za(e,t,r,n){if(t.length===0)throw new Error("fs-primitives: empty path segments");return{parent:await dc(e,t.slice(0,-1),r,n),fileName:t[t.length-1]}}async function w2(e,t,r){const n=await Nh(e,t,r);if(!n)return null;const i=await n.text();if(i.length===0)return null;try{return JSON.parse(i)}catch(o){throw new Uh(t.join("/"),o)}}async function Nh(e,t,r){try{const{parent:n,fileName:i}=await za(e,t,!1,r);return await(await n.getFileHandle(i,{create:!1})).getFile()}catch(n){if(_t(n))return null;throw n}}async function bD(e,t,r){const n=await Nh(e,t,r);return n?n.arrayBuffer():null}async function $h(e,t,r,n){return zh(`cache-write:${t.join("/")}`,async()=>{let i=null;try{const{parent:o,fileName:a}=await za(e,t,!0,n),s=await o.getFileHandle(a,{create:!1}).then(()=>!0).catch(()=>!1),l=await o.getFileHandle(a,{create:!0});s||(i={parent:o,fileName:a});const c=await l.createWritable();try{await c.write(r),await c.close()}catch(u){throw await c.abort().catch(()=>{}),u}return!0}catch(o){if(!x2(o))throw o;return b2.warn(`Cache write dropped for ${t.join("/")}`,o),i&&await i.parent.removeEntry(i.fileName).catch(()=>{}),!1}})}async function k2(e,t,r,n){const i=JSON.stringify(r,null,2);return await $h(e,t,i,n)?i.length:0}async function P2(e,t,r,n){await $h(e,t,r,n)}async function Hh(e,t,r={}){if(t.length===0)throw new Error("fs-primitives: refusing to remove empty path");try{const{parent:n,fileName:i}=await za(e,t,!1);await n.removeEntry(i,{recursive:r.recursive??!1})}catch(n){if(_t(n))return;throw n}}async function SD(e,t,r){try{const n=await dc(e,t,!1,r),i=[];for await(const o of n.values())i.push({name:o.name,kind:o.kind});return i}catch(n){if(_t(n))return[];throw n}}async function ID(e,t,r){try{const n=await dc(e,t,!1),i=[];for await(const a of n.values())a.kind==="file"&&(r&&!r({name:a.name,kind:a.kind})||i.push({name:a.name,handle:a}));return(await Promise.all(i.map(async({name:a,handle:s})=>{try{return{name:a,blob:await s.getFile()}}catch(l){if(_t(l))return null;throw l}}))).filter(a=>a!==null)}catch(n){if(_t(n))return[];throw n}}async function jh(e,t){try{const{parent:r,fileName:n}=await za(e,t,!1);try{return await r.getFileHandle(n,{create:!1}),!0}catch(i){if(!_t(i))throw i}try{return await r.getDirectoryHandle(n,{create:!1}),!0}catch(i){if(!_t(i))throw i}return!1}catch(r){if(_t(r))return!1;throw r}}async function xD(e,t){if(t.length===0)throw new Error("fs-primitives: refusing to remove empty path");const r=t.slice(0,-1),n=t[t.length-1];try{const i=await Di(["media"],!1),o=[];for await(const a of i.values())a.kind==="directory"&&o.push(a);for(const{name:a}of o){const s=await i.getDirectoryHandle(a),l=[];for await(const c of s.values())c.kind==="directory"&&l.push(c.name);for(const c of l)try{let u=await s.getDirectoryHandle(c);for(const d of r)u=await u.getDirectoryHandle(d);await u.removeEntry(n,{recursive:!0})}catch(u){if(!_t(u))throw u}}}catch(i){if(!_t(i))throw i}}const T2="media",E2="content",C2="thumbnail.jpg",M2="cache",_2="waveform",F2="filmstrip",A2="gif-frames",O2="decoded-audio",qh="ai",z2="preview-audio.wav",D2="multi-res.bin",Kh="captions-thumbs",Gh="meta.json",R2="refs.json";function Wh(e){return[T2,e]}function wD(e){return[...Wh(e),C2]}function _n(e){return[...Wh(e),M2]}function Vh(e){return[..._n(e),_2]}function kD(e,t){return[...Vh(e),`bin-${t}.bin`]}function PD(e){return[...Vh(e),D2]}function Zh(e){return[..._n(e),F2]}function TD(e,t,r){return[...Zh(e),`${t}.${r}`]}function ED(e){return[...Zh(e),Gh]}function sa(e){return[..._n(e),z2]}function L2(e){return[..._n(e),A2]}function CD(e,t){return[...L2(e),`frame-${t}.png`]}function B2(e){return[..._n(e),O2]}function MD(e,t,r){return[...B2(e),`${t}-${r}.bin`]}function Da(e){return[..._n(e),qh]}function Ra(e,t){return[...Da(e),`${t}.json`]}function _D(e){return[...Da(e),Kh]}function FD(e){return[...Da(e),"captions-embeddings.bin"]}function AD(e){return[...Da(e),"captions-image-embeddings.bin"]}function id(e){return[...N2(e),qh]}function U2(e){return e===void 0||!Number.isFinite(e)||e<=0?null:`si-${Math.round(e*100)}`}function Li(e,t){const r=U2(t);return r?[...id(e),r]:id(e)}function OD(e,t){return[...Li(e,t),R2]}function zD(e,t){return[...Li(e,t),"captions.json"]}function DD(e,t){return[...Li(e,t),"captions-embeddings.bin"]}function RD(e,t){return[...Li(e,t),"captions-image-embeddings.bin"]}function LD(e,t){return[...Li(e,t),Kh]}function BD(e){return[...e,Gh]}function N2(e){const t=e.slice(0,2);return[E2,t,e]}const $2=1,la=He("WorkspaceFS:AiOutputs");async function UD(e,t){return fc(Ra(e,t),t,`readAiOutput(${e}, ${t})`)}async function fc(e,t,r=`readAiOutputAt(${e.join("/")}, ${t})`){const n=$r();try{return await w2(n,e)??void 0}catch(i){if(i instanceof Uh){la.warn(`${r}: corrupt cache entry treated as a miss`,i);return}throw la.error(`${r} failed`,i),new Error(`Failed to load AI output ${t}`)}}async function ND(e){return H2(Ra(e.mediaId,e.kind),e)}async function H2(e,t){const r=$r(),n=Date.now(),i=await fc(e,t.kind),o={schemaVersion:$2,kind:t.kind,mediaId:t.mediaId,service:t.service,model:t.model,params:t.params??{},createdAt:i?.createdAt??n,updatedAt:n,data:t.data};try{return await k2(r,e,o),o}catch(a){throw la.error(`writeAiOutputAt(${e.join("/")}, ${t.kind}) failed`,a),new Error(`Failed to save AI output ${t.kind}`)}}async function j2(e,t){return q2(Ra(e,t),`deleteAiOutput(${e}, ${t})`)}async function q2(e,t=`deleteAiOutputAt(${e.join("/")})`){const r=$r();try{await Hh(r,e)}catch(n){throw la.error(`${t} failed`,n),new Error("Failed to delete AI output")}}async function K2(e,t){if(e.length===0)return new Set;const r=new Set,n=await Promise.all(e.map(async i=>await fc(Ra(i,t),t)?i:null));for(const i of n)i&&r.add(i);return r}const Yh=He("WorkspaceFS:Transcripts");async function G2(e){if(e.length===0)return new Set;try{return await K2(e,"transcript")}catch(t){throw Yh.error("getTranscriptMediaIds failed",t),new Error("Failed to enumerate transcripts")}}async function $D(e){try{await j2(e,"transcript")}catch(t){throw Yh.error(`deleteTranscript(${e}) failed`,t),new Error(`Failed to delete transcript: ${e}`)}}const Xh=["typewriter","fade-up","rise","cascade","pop","blur-in","slide-mask","wave-in"],Qh=["fade-down","sink","pop-out","blur-out","typewriter-erase"],Jh=["pulse","wave","shimmer","swing"],od={"projects.copySuffix":"{{name}} (Bản sao)","projects.upgradeBackupName":"{{name}} (Bản sao lưu trước khi nâng cấp v{{from}} lên v{{to}})","projects.relativeTime.justNow":"vừa xong","projects.relativeTime.minutesAgo":"{{count}} phút trước","projects.relativeTime.hoursAgo":"{{count}} giờ trước","projects.relativeTime.daysAgo":"{{count}} ngày trước","projects.relativeTime.weeksAgo":"{{count}} tuần trước","projects.relativeTime.monthsAgo":"{{count}} tháng trước","projects.relativeTime.yearsAgo":"{{count}} năm trước","projects.validation.nameRequired":"Tên project là bắt buộc","projects.validation.nameTooLong":"Tên project phải ngắn hơn 100 ký tự","projects.validation.nameWhitespace":"Tên project không được chỉ gồm khoảng trắng","projects.validation.descriptionTooLong":"Mô tả phải ngắn hơn 500 ký tự","projects.validation.widthInteger":"Chiều rộng phải là số nguyên","projects.validation.widthMin":"Chiều rộng tối thiểu 320px","projects.validation.widthMax":"Chiều rộng tối đa 7680px (8K)","projects.validation.heightInteger":"Chiều cao phải là số nguyên","projects.validation.heightMin":"Chiều cao tối thiểu 240px","projects.validation.heightMax":"Chiều cao tối đa 4320px (8K)","projects.validation.fpsInteger":"FPS phải là số nguyên","projects.validation.fpsMin":"FPS tối thiểu 1","projects.validation.fpsMax":"FPS tối đa 240","projects.validation.fpsUnsupported":"FPS phải là một tốc độ khung hình được hỗ trợ","projects.validation.invalidHexColor":"Phải là mã màu hex hợp lệ (ví dụ #000000)","preview.mediaOffline.title":"Media ngoại tuyến","preview.mediaOffline.relink":"Liên kết lại trong panel Media","preview.mediaOffline.loading":"Đang tải…"};function W2(e,t){typeof t=="string"&&(t={defaultValue:t});let n=od[e]??od[e.replace(/_(one|other|few|many|zero|two)$/,"")]??(typeof t?.defaultValue=="string"?t.defaultValue:e);if(t)for(const[i,o]of Object.entries(t))o!=null&&(n=n.replaceAll(`{{${i}}}`,String(o)));return n}const V2={t:W2,language:"vi"},Ht=_e()(gp(N1((e,t)=>({projects:[],currentProject:null,isLoading:!1,error:null,searchQuery:"",sortField:"updatedAt",sortDirection:"desc",filterResolution:void 0,filterFps:void 0,loadProjects:async()=>{e({isLoading:!0,error:null});try{const r=await l2();e({projects:r,isLoading:!1})}catch(r){const n=r instanceof Error?r.message:"Failed to load projects";throw e({error:n,isLoading:!1}),r}},loadProject:async r=>{e({isLoading:!0,error:null});try{const n=await c2(r);return n?(e({currentProject:n,isLoading:!1}),n):(e({error:`Project not found: ${r}`,isLoading:!1}),null)}catch(n){const i=n instanceof Error?n.message:"Failed to load project";throw e({error:i,isLoading:!1}),n}},createProject:async r=>{e({error:null});try{const n=await u2({name:r.name,description:r.description,width:r.width,height:r.height,fps:r.fps,backgroundColor:r.backgroundColor});return e(i=>({projects:[...i.projects,n],currentProject:n})),n}catch(n){const i=n instanceof Error?n.message:"Failed to create project";throw e({error:i}),n}},updateProject:async(r,n)=>{e({error:null});const i=t().projects,o=t().currentProject,a=i.findIndex(c=>c.id===r);let s=null;if(a!==-1?s=i[a]??null:o?.id===r&&(s=o),!s)throw e({error:`Project not found: ${r}`}),new Error(`Project not found: ${r}`);const l={...s,name:n.name??s.name,description:n.description??s.description,metadata:{width:n.width??s.metadata.width,height:n.height??s.metadata.height,fps:n.fps??s.metadata.fps,backgroundColor:n.backgroundColor??s.metadata.backgroundColor},updatedAt:Date.now()};if(a!==-1){const c=[...i];c[a]=l,e({projects:c})}o?.id===r&&e({currentProject:l});try{await i2(r);const c=await d2(r,{name:l.name,description:l.description,metadata:l.metadata});return t().currentProject?.id===r&&e({currentProject:c}),c}catch(c){a!==-1&&e({projects:i}),o?.id===r&&e({currentProject:o});const u=c instanceof Error?c.message:"Failed to update project";throw e({error:u}),c}},deleteProject:async r=>{e({error:null});const n=t().projects,i=t().currentProject;e({projects:n.filter(o=>o.id!==r),currentProject:i?.id===r?null:i});try{await p2(r),e(o=>({projects:o.projects.filter(a=>a.id!==r),currentProject:o.currentProject?.id===r?null:o.currentProject}))}catch(o){e({projects:n,currentProject:i});const a=o instanceof Error?o.message:"Failed to delete project";throw e({error:a}),o}},duplicateProject:async r=>{e({error:null});const n=t().projects.find(i=>i.id===r);if(!n)throw e({error:`Project not found: ${r}`}),new Error(`Project not found: ${r}`);try{const i=await f2(r,V2.t("projects.copySuffix",{name:n.name}));return e(o=>({projects:[...o.projects,i]})),i}catch(i){const o=i instanceof Error?i.message:"Failed to duplicate project";throw e({error:o}),i}},setCurrentProject:r=>e({currentProject:r}),setSearchQuery:r=>e({searchQuery:r}),setSortField:r=>e({sortField:r}),setSortDirection:r=>e({sortDirection:r}),setFilterResolution:r=>e({filterResolution:r}),setFilterFps:r=>e({filterFps:r}),clearFilters:()=>e({searchQuery:"",filterResolution:void 0,filterFps:void 0}),clearError:()=>e({error:null})}),{partialize:e=>({projects:e.projects,currentProject:e.currentProject})}),{name:"ProjectStore",enabled:!1}));Ht.temporal.getState().setOnSave(()=>{const e=_h.getState().maxUndoHistory,{pastStates:t,futureStates:r}=Ht.temporal.getState();(t.length>e||r.length>e)&&Ht.temporal.setState({pastStates:t.slice(-e),futureStates:r.slice(-e)})});_h.subscribe((e,t)=>{if(e.maxUndoHistory!==t.maxUndoHistory){const{pastStates:r,futureStates:n}=Ht.temporal.getState();(r.length>e.maxUndoHistory||n.length>e.maxUndoHistory)&&Ht.temporal.setState({pastStates:r.slice(-e.maxUndoHistory),futureStates:n.slice(-e.maxUndoHistory)})}});const eg=["items","tracks","transitions","keyframes","markers","compositions","mediaItems"];function yl(e,t){if(!e||typeof e!="object")throw new Error(`Invalid ${t} entity`);const r=Reflect.get(e,t==="keyframes"?"itemId":"id");if(typeof r!="string")throw new Error(`Missing ${t} entity id`);return r}function Ge(){const e=S.getState(),t=at.getState(),r=W.getState(),n=be.getState(),i=Ht.getState().currentProject,o=Oe.getState(),a=new Set(o.importingIds);return{projectName:i?.name,mediaItems:a.size?o.mediaItems.filter(s=>!a.has(s.id)):o.mediaItems,stashStack:ye.getState().stashStack,mainHolder:ye.getState().mainHolder,items:e.items,tracks:e.tracks,transitions:q.getState().transitions,keyframes:$.getState().keyframes,markers:t.markers,compositions:ae.getState().compositions,topLevelSequenceIds:_r.getState().topLevelSequenceIds,inPoint:t.inPoint,outPoint:t.outPoint,fps:r.fps,scrollPosition:r.scrollPosition,snapEnabled:r.snapEnabled,currentFrame:n.currentFrame,busAudioEq:n.busAudioEq,masterBusDb:n.masterBusDb,projectId:i?.id??null,projectMetadata:i?{...i.metadata}:null}}function Pr(e){const t={...e};if(e.entities){const r=Ge();for(const n of eg){const i=e.entities[n];if(!i)continue;const o=new Map((r[n]??[]).map(s=>[yl(s,n),s]));for(const[s,l]of Object.entries(i.values))l===null?o.delete(s):o.set(s,l);const a=i.order??[...o.keys()];Reflect.set(t,n,a.flatMap(s=>o.has(s)?[o.get(s)]:[]))}}if("items"in t&&S.getState().setItems(t.items),"tracks"in t&&S.setState({tracks:t.tracks}),"transitions"in t&&q.getState().setTransitions(t.transitions),"keyframes"in t&&$.getState().setKeyframes(t.keyframes),"compositions"in t&&ae.getState().setCompositions(t.compositions),"topLevelSequenceIds"in t&&_r.getState().setTopLevelSequenceIds(t.topLevelSequenceIds),"stashStack"in t&&ye.setState({stashStack:t.stashStack??[]}),"mainHolder"in t&&ye.setState({mainHolder:t.mainHolder??null}),"markers"in t&&at.getState().setMarkers(t.markers),"inPoint"in t&&at.getState().setInPoint(t.inPoint),"outPoint"in t&&at.getState().setOutPoint(t.outPoint),"fps"in t&&W.getState().setFps(t.fps),"scrollPosition"in t&&W.getState().setScrollPosition(t.scrollPosition),"snapEnabled"in t&&W.getState().setSnapEnabled(t.snapEnabled),"currentFrame"in t&&be.getState().setCurrentFrame(t.currentFrame),"busAudioEq"in t&&be.getState().setBusAudioEq(t.busAudioEq),"masterBusDb"in t&&be.getState().setMasterBusDb(t.masterBusDb),"mediaItems"in t){const r=Oe.getState(),n=new Set(r.importingIds),o=[...r.mediaItems.filter(a=>n.has(a.id)),...(t.mediaItems??[]).filter(a=>!n.has(a.id))];Oe.setState({mediaItems:o,mediaById:Object.fromEntries(o.map(a=>[a.id,a]))})}("projectId"in t||"projectMetadata"in t||"projectName"in t)&&Ht.setState(r=>{if("projectId"in t&&t.projectId===null)return{currentProject:null};const n=r.currentProject,i=t.projectId??n?.id??"command-project",o=t.projectMetadata??n?.metadata??{width:1920,height:1080,fps:t.fps??30},a={description:"",createdAt:0,updatedAt:0,duration:0,...n,id:i,name:t.projectName??n?.name??"Dự án",metadata:o};return{currentProject:a,projects:r.projects.map(s=>s.id===i?a:s)}})}function Z2(){const e={};for(const t of Object.keys(Ge()))Object.defineProperty(e,t,{enumerable:!0,get:()=>Ge()[t],set:r=>Pr({[t]:r})});return Object.defineProperty(e,"nav",{get:()=>ye.getState()}),Object.defineProperty(e,"sequenceViews",{get:()=>_r.getState().sequenceViewById}),e}function Rn(e,t){const r={};for(const n of Object.keys(e))if(e[n]!==t[n])if(eg.includes(n)){const i=n,o=e[i]??[],a=t[i]??[],s=new Map(o.map(p=>[yl(p,i),p])),l=new Map(a.map(p=>[yl(p,i),p])),c={};for(const[p,m]of s){const h=l.get(p);m!==h&&JSON.stringify(m)!==JSON.stringify(h)&&(c[p]=m)}for(const p of l.keys())s.has(p)||(c[p]=null);const u=[...s.keys()],d=[...l.keys()],f=u.length!==d.length||u.some((p,m)=>p!==d[m]);(Object.keys(c).length||f)&&(r.entities??={},r.entities[i]={values:c,...f?{order:u}:{}})}else JSON.stringify(e[n])!==JSON.stringify(t[n])&&Reflect.set(r,n,e[n]);return r}function Y2(e,t){const r=ye.getState();if((r.activeCompositionId??"__root__")===e)return Ge();r.activeCompositionId&&ai(r.activeCompositionId);const n=Ge(),i=r.stashStack.find(o=>(o.compositionId??"__root__")===e)??(e==="__root__"?r.mainHolder:null);return i?{...n,items:i.items,tracks:i.tracks,transitions:i.transitions,keyframes:i.keyframes,markers:i.markers,inPoint:i.inPoint,outPoint:i.outPoint,currentFrame:i.currentFrame,busAudioEq:i.busAudioEq,stashStack:t.stashStack,mainHolder:t.mainHolder}:n}const tg=new Map;function A(e){tg.set(e.type,e)}function vl(e){return tg.get(e)}const X2="__root__";function bl(e,t){const r={...e,...t};if(e.entities||t.entities){r.entities={...e.entities};for(const[n,i]of Object.entries(t.entities??{})){const o=n,a=r.entities[o];r.entities[o]={...a,...i,values:{...a?.values,...i.values}}}}return r}class Q2{contexts=new Map;stacks(t){let r=this.contexts.get(t);return r||(r={undo:[],redo:[]},this.contexts.set(t,r)),r}record(t){const r=this.stacks(t.contextKey),n=r.undo.at(-1);if(t.mergeKey&&n?.mergeKey===t.mergeKey)if(n.commands=t.commands,n.redoPatch=bl(n.redoPatch,t.redoPatch),n.inverses.every(i=>i.kind==="patch")&&t.inverses.every(i=>i.kind==="patch")){let i={};for(const o of[...t.inverses,...n.inverses].reverse())o.kind==="patch"&&(i=bl(o.patch,i));n.inverses=[{kind:"patch",patch:i}]}else n.inverses=[...t.inverses,...n.inverses];else r.undo.push(t);r.redo.length=0}clear(){this.contexts.clear()}removeContext(t){this.contexts.delete(t)}}const Sl=_e(()=>({canUndo:!1,canRedo:!1,revision:0}));let ao;class J2{constructor(t=rp()){this.ids=t}ids;history=new Q2;depth=0;listeners=new Set;get contextKey(){return ye.getState().activeCompositionId??"__root__"}get canUndo(){return this.history.stacks(this.contextKey).undo.length>0}get canRedo(){return this.history.stacks(this.contextKey).redo.length>0}on(t,r){return this.listeners.add(r),()=>{this.listeners.delete(r)}}executeCommands(t,r={}){const n=Ge(),i=this.contextKey,o=ye.getState(),a=W.getState().isDirty,s=this.ids.checkpoint?.();let l=-1;const c=[],u=t.map((g,v)=>{const x=vl(g.type);if(!x)return c.push({message:`Unknown command: ${g.type}`,commandIndex:v}),null;const b=x.params.safeParse(g.params);return b.success?{definition:x,params:b.data,command:{type:g.type,params:b.data}}:(c.push(...b.error.issues.map(w=>({message:w.message,path:w.path.join("."),commandIndex:v}))),null)});if(c.length)return{ok:!1,issues:c};const d=[],f=new Map,p=this.depth===0;this.depth++;try{$c(this.ids,()=>{for(let g=0;g<u.length;g++){l=g;const v=u[g];if(!v)continue;const x=Ge(),b=this.contextKey,w=v.definition.validate?.(x,v.params)??[];if(w.length)throw c.push(...w.map(T=>({...T,commandIndex:g}))),new Error(w.map(T=>T.message).join("; "));const E=v.definition.apply({draft:Z2(),ids:this.ids,contextKey:b},v.params),k="inverse"in E?E.inverse:E;if(d.push("inverse"in E?E.result:void 0),v.definition.history!=="none"){const T=Y2(b,x),O=Rn(x,T);if(Object.keys(O).length){let C=f.get(b);C||(C={commands:[],inverses:[],redoPatch:{},contextKey:b,mergeKey:u.length===1?ao!==void 0?ao:v.definition.mergeKey?.(v.params)??null:null,timestamp:Date.now()},f.set(b,C)),C.commands.push(v.command),C.inverses.unshift(k.kind==="patch"&&Object.keys(k.patch).length===0?{kind:"patch",patch:O}:k),C.redoPatch=bl(C.redoPatch,Rn(T,x))}}}})}catch(g){return Pr(n),ye.setState(o),er(o.activeCompositionId),W.getState().setIsDirty(a),this.ids.checkpoint&&this.ids.restore&&this.ids.restore(s),{ok:!1,issues:c.length?c:[{message:g instanceof Error?g.message:String(g),commandIndex:l}],error:g instanceof Error?g.message:String(g)}}finally{this.depth--}const m=Ge();if(r.dryRun)return Pr(n),ye.setState(o),er(o.activeCompositionId),W.getState().setIsDirty(a),this.ids.checkpoint&&this.ids.restore&&this.ids.restore(s),{ok:!0,commands:t,results:d};const h=u.flatMap(g=>g?[g.command]:[]);if(p){for(const g of f.values())this.history.record(g);this.emit({kind:"execute",commands:h,contextKey:i,before:n,after:m})}return{ok:!0,commands:h,results:d}}executeAnonymous(t,r,n){if(this.depth)return r();const i=Ge(),o=this.contextKey;this.depth++;let a;try{a=$c(this.ids,r)}catch(u){throw Pr(i),u}finally{this.depth--}const s=Ge(),l=Rn(i,s),c=[{type:t,params:n??{}}];return Object.keys(l).length&&this.history.record({commands:c,inverses:[{kind:"patch",patch:l}],redoPatch:Rn(s,i),contextKey:o,mergeKey:ao??null,timestamp:Date.now()}),this.emit({kind:"execute",commands:c,contextKey:o,before:i,after:s}),a}undo(){const t=this.history.stacks(this.contextKey),r=t.undo.at(-1);if(!r)return{ok:!1,issues:[{message:"Nothing to undo"}]};const n=Ge();this.depth++;try{for(const i of r.inverses)if(i.kind==="patch")Pr(i.patch);else{const o=this.executeCommands(i.commands);if(!o.ok)throw new Error(o.issues.map(a=>a.message).join("; "))}}catch(i){return Pr(n),{ok:!1,issues:[{message:String(i)}]}}finally{this.depth--}return t.undo.pop(),t.redo.push(r),this.emit({kind:"undo",commands:r.commands,contextKey:r.contextKey,before:n,after:Ge()}),{ok:!0,commands:r.commands,results:[]}}redo(){const t=this.history.stacks(this.contextKey),r=t.redo.pop();if(!r)return{ok:!1,issues:[{message:"Nothing to redo"}]};const n=Ge();return Pr(r.redoPatch),t.undo.push(r),this.emit({kind:"redo",commands:r.commands,contextKey:r.contextKey,before:n,after:Ge()}),{ok:!0,commands:r.commands,results:[]}}closeMergeGroup(){ao=void 0;const t=this.history.stacks(this.contextKey).undo.at(-1);t&&(t.mergeKey=null)}clearHistory(){this.history.clear(),Sl.setState(t=>({canUndo:!1,canRedo:!1,revision:t.revision+1}))}emit(t){t.commands.some(n=>{const i=vl(n.type);return i?.dirty??i?.history!=="none"})&&Object.keys(Rn(t.before,t.after)).length&&W.getState().markDirty(),Sl.setState(n=>({canUndo:this.canUndo,canRedo:this.canRedo,revision:n.revision+1}));for(const n of this.listeners)n(t)}}let ad,sd=!1;function pc(){return ad??=new J2,sd||(sd=!0,ye.subscribe((e,t)=>{if(e.activeCompositionId===t.activeCompositionId)return;const r=pc();Sl.setState(n=>({canUndo:r.canUndo,canRedo:r.canRedo,revision:n.revision+1}))})),ad}function qr(e,t){return pc().executeCommands(e,t)}y({type:I().min(1),params:Fa()}).strict();function fi(e,t,r){return Math.max(t,Math.min(r,e))}function eP(e){if(e.length<=1)return[0];const t=e.length-1,r=new Array(t);for(let i=0;i<t;i+=1){const o=e[i],a=e[i+1],s=Math.max(1e-6,a.x-o.x);r[i]=(a.y-o.y)/s}const n=new Array(e.length);n[0]=r[0],n[e.length-1]=r[t-1];for(let i=1;i<e.length-1;i+=1){const o=r[i-1],a=r[i];n[i]=o*a<=0?0:(o+a)/2}for(let i=0;i<t;i+=1){const o=r[i];if(Math.abs(o)<1e-6){n[i]=0,n[i+1]=0;continue}const a=n[i]/o,s=n[i+1]/o,l=a*a+s*s;if(l>9){const c=3/Math.sqrt(l);n[i]=c*a*o,n[i+1]=c*s*o}}return n}function tP(e){if(!e||e.length===0)return[{x:0,y:0},{x:1,y:1}];const t=e.map(i=>({x:fi(i.x,0,1),y:fi(i.y,0,1)})).sort((i,o)=>i.x-o.x),r=[];for(const i of t){const o=r[r.length-1];(!o||Math.abs(o.x-i.x)>5e-4)&&r.push(i)}if(r.length===0)return[{x:0,y:0},{x:1,y:1}];r[0].x>1e-4?r.unshift({x:0,y:r[0].y}):r[0]={x:0,y:r[0].y};const n=r[r.length-1];return n.x<.9999?r.push({x:1,y:n.y}):Math.abs(n.x-1)>1e-4&&(r[r.length-1]={x:1,y:n.y}),r}function so(e,t){const r=tP(e),n=fi(t,0,1);if(n<=r[0].x)return r[0].y;if(n>=r[r.length-1].x)return r[r.length-1].y;let i=r.length-2;for(let b=0;b<r.length-1;b+=1){const w=r[b],E=r[b+1];if(n>=w.x&&n<=E.x){i=b;break}}const o=r[i],a=r[i+1],s=Math.max(1e-6,a.x-o.x),l=fi((n-o.x)/s,0,1),c=l*l,u=c*l,d=eP(r),f=d[i],p=d[i+1],m=2*u-3*c+1,h=u-2*c+l,g=-2*u+3*c,v=u-c,x=m*o.y+h*s*f+g*a.y+v*s*p;return fi(x,0,1)}const Le={shadow:.25,highlight:.75},Il=["master","red","green","blue"],ns=.02,is=.98,Qr=.04;function nt(e,t,r){return Math.max(t,Math.min(r,e))}function ze(e,t){return typeof e=="number"&&Number.isFinite(e)?e:t}function rP(e,t){const r=ze(e[t],0);return nt(r/200,-.5,.5)}function nP(e,t){const r=1-e;return e+r*r*t*.5}function iP(e,t){const r=4*e*(1-e);return e+r*t*.25}function oP(e,t){return e+e*e*t*.5}function aP(e,t){return(e-.5)*(1+t)+.5}function ld(e,t){const r=ze(e.shadows,0)/100,n=ze(e.midtones,0)/100,i=ze(e.highlights,0)/100,o=ze(e.contrast,0)/100;let a=t;return a=nP(a,r),a=iP(a,n),a=oP(a,i),a=aP(a,o),nt(a,0,1)}function mc(e){const t=e.charAt(0).toUpperCase()+e.slice(1);return{shadowX:`${e}ShadowX`,shadowY:`${e}ShadowY`,highlightX:`${e}HighlightX`,highlightY:`${e}HighlightY`,prefix:t}}function rg(){return{shadow:{x:Le.shadow,y:Le.shadow},highlight:{x:Le.highlight,y:Le.highlight}}}function Lo(e){const t=rg();let r=nt(ze(e.shadow.x,t.shadow.x),ns,is),n=nt(ze(e.highlight.x,t.highlight.x),ns,is);if(r>n&&([r,n]=[n,r]),n-r<Qr){const i=nt((r+n)/2,ns+Qr/2,is-Qr/2);r=i-Qr/2,n=i+Qr/2}return{shadow:{x:r,y:nt(ze(e.shadow.y,t.shadow.y),0,1)},highlight:{x:n,y:nt(ze(e.highlight.y,t.highlight.y),0,1)}}}function sP(e){const t=Lo(e);return[{x:0,y:0},t.shadow,t.highlight,{x:1,y:1}]}function lP(e,t){const r=mc(t);if([r.shadowX,r.shadowY,r.highlightX,r.highlightY].some(o=>typeof e[o]=="number"&&Number.isFinite(e[o])))return Lo({shadow:{x:ze(e[r.shadowX],Le.shadow),y:ze(e[r.shadowY],Le.shadow)},highlight:{x:ze(e[r.highlightX],Le.highlight),y:ze(e[r.highlightY],Le.highlight)}});if(t==="master")return Lo({shadow:{x:Le.shadow,y:ld(e,Le.shadow)},highlight:{x:Le.highlight,y:ld(e,Le.highlight)}});const i=rP(e,t);return Lo({shadow:{x:Le.shadow,y:Le.shadow+i},highlight:{x:Le.highlight,y:Le.highlight+i}})}const xl=256,cP=16;function ca(e){return`${e}Points`}function uP(e){const t=e.filter(n=>Number.isFinite(n.x)&&Number.isFinite(n.y)).map(n=>({x:nt(n.x,0,1),y:nt(n.y,0,1)})).sort((n,i)=>n.x-i.x).slice(0,cP),r=[];for(const n of t){const i=r[r.length-1];i&&n.x-i.x<Qr/2||r.push(n)}return r.length<2?[{x:0,y:0},{x:1,y:1}]:r}function dP(e){try{const t=JSON.parse(e);if(!Array.isArray(t))return null;const r=[];for(const n of t){if(!Array.isArray(n)||n.length<2)return null;const[i,o]=n;if(typeof i!="number"||typeof o!="number")return null;r.push({x:i,y:o})}return r.length>=2?uP(r):null}catch{return null}}function lo(e,t){const r=e[ca(t)];if(typeof r=="string"&&r.length>0){const n=dP(r);if(n)return n}return sP(lP(e,t))}function fP(e){const t=xl,r=lo(e,"master"),n=lo(e,"red"),i=lo(e,"green"),o=lo(e,"blue"),a=new Uint8Array(t*4);for(let s=0;s<t;s++){const l=s/(t-1),c=so(r,l);a[s*4]=Math.round(nt(so(n,c),0,1)*255),a[s*4+1]=Math.round(nt(so(i,c),0,1)*255),a[s*4+2]=Math.round(nt(so(o,c),0,1)*255),a[s*4+3]=255}return a}function pP(e){const t=[];for(const r of Il){const n=mc(r);t.push(ze(e[n.shadowX],-1),ze(e[n.shadowY],-1),ze(e[n.highlightX],-1),ze(e[n.highlightY],-1),typeof e[ca(r)]=="string"?e[ca(r)]:"")}return t.join("|")}function Ze(e,t,r){const n=e[t];return typeof n=="number"?n:r}function mP(e,t){if(typeof e!="string"||!e.startsWith("#"))return t;const r=e.slice(1),n=r.length===3?r.split("").map(s=>s+s).join(""):r;if(!/^[0-9a-fA-F]{6}$/.test(n))return t;const i=parseInt(n.slice(0,2),16)/255,o=parseInt(n.slice(2,4),16)/255,a=parseInt(n.slice(4,6),16)/255;return[i,o,a].every(Number.isFinite)?[i,o,a]:t}const hP={id:"gpu-brightness",name:"Brightness",category:"color",entryPoint:"brightnessFragment",uniformSize:16,shader:`
struct BrightnessParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: BrightnessParams;
@fragment
fn brightnessFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let adjusted = color.rgb + params.amount;
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:0,min:-1,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??0,0,0,0])},gP={id:"gpu-contrast",name:"Contrast",category:"color",entryPoint:"contrastFragment",uniformSize:16,shader:`
struct ContrastParams { amount: f32, _p1: f32, _p2: f32, _p3: f32 };
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(2) var<uniform> params: ContrastParams;
@fragment
fn contrastFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  let adjusted = (color.rgb - 0.5) * params.amount + 0.5;
  return vec4f(clamp(adjusted, vec3f(0.0), vec3f(1.0)), color.a);
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:3,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},yP={id:"gpu-exposure",name:"Exposure",category:"color",entryPoint:"exposureFragment",uniformSize:16,shader:`
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
}`,params:{exposure:{type:"number",label:"Exposure (EV)",default:0,min:-3,max:3,step:.1,animatable:!0},offset:{type:"number",label:"Offset",default:0,min:-.5,max:.5,step:.01,animatable:!0},gamma:{type:"number",label:"Gamma",default:1,min:.2,max:3,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.exposure??0,e.offset??0,e.gamma??1,0])},vP={id:"gpu-hue-shift",name:"Hue Shift",category:"color",entryPoint:"hueShiftFragment",uniformSize:16,shader:`
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
}`,params:{shift:{type:"number",label:"Shift",default:0,min:0,max:1,step:.01,animatable:!0},span:{type:"number",label:"Span",default:1,min:0,max:2,step:.01,animatable:!0},flow:{type:"number",label:"Flow",default:0,min:0,max:2,step:.05,animatable:!1}},packUniforms:e=>new Float32Array([e.shift??0,e.span??1,e.flow??0,performance.now()/1e3])},bP={id:"gpu-invert",name:"Invert",category:"color",entryPoint:"invertFragment",uniformSize:0,shader:`
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@fragment
fn invertFragment(input: VertexOutput) -> @location(0) vec4f {
  let color = textureSample(inputTex, texSampler, input.uv);
  return vec4f(1.0 - color.rgb, color.a);
}`,params:{},packUniforms:()=>null},SP={id:"gpu-levels",name:"Levels",category:"color",entryPoint:"levelsFragment",uniformSize:32,shader:`
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
}`,params:{inputBlack:{type:"number",label:"Input Black",default:0,min:0,max:1,step:.01,animatable:!0},inputWhite:{type:"number",label:"Input White",default:1,min:0,max:1,step:.01,animatable:!0},gamma:{type:"number",label:"Gamma",default:1,min:.1,max:3,step:.01,animatable:!0},outputBlack:{type:"number",label:"Output Black",default:0,min:0,max:1,step:.01,animatable:!0},outputWhite:{type:"number",label:"Output White",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.inputBlack??0,e.inputWhite??1,e.gamma??1,e.outputBlack??0,e.outputWhite??1,0,0,0])},IP={id:"gpu-saturation",name:"Saturation",category:"color",entryPoint:"saturationFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:3,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},xP={id:"gpu-temperature",name:"Temperature",category:"color",entryPoint:"temperatureFragment",uniformSize:16,shader:`
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
}`,params:{temperature:{type:"number",label:"Temperature",default:0,min:-1,max:1,step:.01,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-1,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.temperature??0,e.tint??0,0,0])},wP={id:"gpu-grayscale",name:"Grayscale",category:"color",entryPoint:"grayscaleFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},kP={id:"gpu-sepia",name:"Sepia",category:"color",entryPoint:"sepiaFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,0,0,0])},PP={id:"gpu-curves",name:"Curves",category:"color",entryPoint:"curvesFragment",uniformSize:0,shader:`
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var inputTex: texture_2d<f32>;
@group(0) @binding(3) var curveLut: texture_2d<f32>;

fn sampleCurveLut(value: f32) -> vec3f {
  let lutWidth = ${xl}.0;
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
}`,params:Object.fromEntries([...Il.flatMap(e=>{const t=mc(e),r=rg(),n=e.charAt(0).toUpperCase()+e.slice(1);return[[t.shadowX,{type:"number",label:`${n} Shadow X`,default:r.shadow.x,min:0,max:1,step:.001,animatable:!0}],[t.shadowY,{type:"number",label:`${n} Shadow Y`,default:r.shadow.y,min:0,max:1,step:.001,animatable:!0}],[t.highlightX,{type:"number",label:`${n} Highlight X`,default:r.highlight.x,min:0,max:1,step:.001,animatable:!0}],[t.highlightY,{type:"number",label:`${n} Highlight Y`,default:r.highlight.y,min:0,max:1,step:.001,animatable:!0}]]}),...Il.map(e=>{const t=e.charAt(0).toUpperCase()+e.slice(1);return[ca(e),{type:"json",label:`${t} Points`,default:""}]})]),packUniforms:()=>null,dataTexture:{dimension:"2d",key:pP,build:e=>({width:xl,height:1,depth:1,data:fP(e)})}},TP={id:"gpu-color-wheels",name:"Color Wheels",category:"color",entryPoint:"colorWheelsFragment",uniformSize:112,shader:`
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
}`,params:{shadowsHue:{type:"number",label:"Lift Hue",default:0,min:0,max:360,step:1,animatable:!0},shadowsAmount:{type:"number",label:"Lift Amount",default:0,min:0,max:1,step:.01,animatable:!0},midtonesHue:{type:"number",label:"Gamma Hue",default:0,min:0,max:360,step:1,animatable:!0},midtonesAmount:{type:"number",label:"Gamma Amount",default:0,min:0,max:1,step:.01,animatable:!0},highlightsHue:{type:"number",label:"Gain Hue",default:0,min:0,max:360,step:1,animatable:!0},highlightsAmount:{type:"number",label:"Gain Amount",default:0,min:0,max:1,step:.01,animatable:!0},offsetHue:{type:"number",label:"Offset Hue",default:0,min:0,max:360,step:1,animatable:!0},offsetAmount:{type:"number",label:"Offset Amount",default:0,min:0,max:1,step:.01,animatable:!0},temperature:{type:"number",label:"Temperature",default:0,min:-100,max:100,step:1,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-100,max:100,step:1,animatable:!0},saturation:{type:"number",label:"Saturation",default:0,min:-100,max:100,step:1,animatable:!0},exposure:{type:"number",label:"Exposure",default:0,min:-3,max:3,step:.05,animatable:!0},contrast:{type:"number",label:"Contrast",default:1,min:0,max:2,step:.01,animatable:!0},pivot:{type:"number",label:"Pivot",default:.5,min:0,max:1,step:.01,animatable:!0},lift:{type:"number",label:"Lift",default:0,min:-2,max:2,step:.01,animatable:!0},gamma:{type:"number",label:"Gamma",default:1,min:0,max:4,step:.01,animatable:!0},gain:{type:"number",label:"Gain",default:1,min:0,max:16,step:.01,animatable:!0},offset:{type:"number",label:"Offset",default:0,min:-2,max:2,step:.0025,animatable:!0},blackPoint:{type:"number",label:"Black Point",default:0,min:0,max:.5,step:.005,animatable:!0},whitePoint:{type:"number",label:"White Point",default:1,min:.5,max:1.5,step:.005,animatable:!0},midDetail:{type:"number",label:"Mid/Detail",default:0,min:-100,max:100,step:1,animatable:!0},colorBoost:{type:"number",label:"Color Boost",default:0,min:-100,max:100,step:1,animatable:!0},shadows:{type:"number",label:"Shadows",default:0,min:-100,max:100,step:1,animatable:!0},highlights:{type:"number",label:"Highlights",default:0,min:-100,max:100,step:1,animatable:!0},hue:{type:"number",label:"Hue",default:50,min:0,max:100,step:1,animatable:!0},lumMix:{type:"number",label:"Lum Mix",default:100,min:0,max:100,step:1,animatable:!0}},packUniforms:e=>new Float32Array(EP.map(([t,r])=>Ze(e,t,r)))},EP=[["shadowsHue",0],["shadowsAmount",0],["midtonesHue",0],["midtonesAmount",0],["highlightsHue",0],["highlightsAmount",0],["temperature",0],["tint",0],["saturation",0],["exposure",0],["contrast",1],["pivot",.5],["lift",0],["gamma",1],["gain",1],["offset",0],["blackPoint",0],["whitePoint",1],["offsetHue",0],["offsetAmount",0],["midDetail",0],["colorBoost",0],["shadows",0],["highlights",0],["hue",50],["lumMix",100],["_pad1",0],["_pad2",0]],CP={id:"gpu-secondary-qualifier",name:"Secondary Qualifier",category:"color",entryPoint:"secondaryQualifierFragment",uniformSize:64,shader:`
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
}`,params:{hueCenter:{type:"number",label:"Hue Center",default:0,min:0,max:360,step:1,animatable:!0},hueWidth:{type:"number",label:"Hue Width",default:35,min:0,max:180,step:1,animatable:!0},hueSoftness:{type:"number",label:"Hue Softness",default:20,min:0,max:120,step:1,animatable:!0},satLow:{type:"number",label:"Sat Low",default:0,min:0,max:1,step:.01,animatable:!0},satHigh:{type:"number",label:"Sat High",default:1,min:0,max:1,step:.01,animatable:!0},satSoftness:{type:"number",label:"Sat Softness",default:.1,min:0,max:1,step:.01,animatable:!0},lumaLow:{type:"number",label:"Luma Low",default:0,min:0,max:1,step:.01,animatable:!0},lumaHigh:{type:"number",label:"Luma High",default:1,min:0,max:1,step:.01,animatable:!0},lumaSoftness:{type:"number",label:"Luma Softness",default:.1,min:0,max:1,step:.01,animatable:!0},invertMask:{type:"boolean",label:"Invert Mask",default:!1},showMask:{type:"boolean",label:"Show Mask",default:!1},exposure:{type:"number",label:"Exposure",default:0,min:-3,max:3,step:.05,animatable:!0},saturation:{type:"number",label:"Saturation",default:0,min:-100,max:100,step:1,animatable:!0},temperature:{type:"number",label:"Temperature",default:0,min:-100,max:100,step:1,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-100,max:100,step:1,animatable:!0},strength:{type:"number",label:"Strength",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array(MP.map(([t,r])=>{const n=e[t];return typeof n=="boolean"?n?1:0:typeof n=="number"?n:r}))},MP=[["hueCenter",0],["hueWidth",35],["hueSoftness",20],["satLow",0],["satHigh",1],["satSoftness",.1],["lumaLow",0],["lumaHigh",1],["lumaSoftness",.1],["invertMask",0],["showMask",0],["exposure",0],["saturation",0],["temperature",0],["tint",0],["strength",1]],_P={ellipse:0,rectangle:1},FP={id:"gpu-power-window",name:"Power Window",category:"color",entryPoint:"powerWindowFragment",uniformSize:64,shader:`
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
}`,params:{shape:{type:"select",label:"Shape",default:"ellipse",options:[{value:"ellipse",label:"Ellipse"},{value:"rectangle",label:"Rectangle"}]},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},sizeX:{type:"number",label:"Width",default:.5,min:.02,max:1.5,step:.01,animatable:!0},sizeY:{type:"number",label:"Height",default:.5,min:.02,max:1.5,step:.01,animatable:!0},rotation:{type:"number",label:"Rotation",default:0,min:-180,max:180,step:1,animatable:!0},feather:{type:"number",label:"Feather",default:.3,min:0,max:1,step:.01,animatable:!0},invertMask:{type:"boolean",label:"Invert Mask",default:!1},showMask:{type:"boolean",label:"Show Mask",default:!1},exposure:{type:"number",label:"Exposure",default:.3,min:-3,max:3,step:.05,animatable:!0},saturation:{type:"number",label:"Saturation",default:0,min:-100,max:100,step:1,animatable:!0},temperature:{type:"number",label:"Temperature",default:0,min:-100,max:100,step:1,animatable:!0},tint:{type:"number",label:"Tint",default:0,min:-100,max:100,step:1,animatable:!0},strength:{type:"number",label:"Strength",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([_P[e.shape]??0,Ze(e,"centerX",.5),Ze(e,"centerY",.5),Ze(e,"sizeX",.5),Ze(e,"sizeY",.5),Ze(e,"rotation",0),Ze(e,"feather",.3),e.invertMask===!0?1:0,e.showMask===!0?1:0,Ze(e,"exposure",.3),Ze(e,"saturation",0),Ze(e,"temperature",0),Ze(e,"tint",0),Ze(e,"strength",1),t,r])},AP={id:"gpu-vibrance",name:"Vibrance",category:"color",entryPoint:"vibranceFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:0,min:-1,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??0,0,0,0])},ua={inferno:["#000004","#420a68","#932667","#dd513a","#fca50a","#f0f921"],magma:["#000004","#3b0f70","#8c2981","#de4968","#fe9f6d","#fcfdbf"],plasma:["#0d0887","#6a00a8","#b12a90","#e16462","#fca636","#f0f921"],viridis:["#440154","#3b528b","#21918c","#5ec962","#fde725"],turbo:["#30123b","#4675ed","#1bcfd4","#a4fc3b","#fe9b2d","#cb2a04","#7a0403"],fire:["#000000","#7a0000","#ff4800","#ffd000","#ffffff"],ice:["#000010","#003b6f","#1b78c2","#7ec8ff","#ffffff"],sunset:["#241634","#c2456b","#ffd9a0"],grayscale:["#000000","#ffffff"]},OP=ua.inferno.join(", ");function zP(e,t){const n=(e==="custom"?t.split(",").map(i=>i.trim()).filter(i=>i.length>0):ua[e]??ua.inferno).map(i=>mP(i,[0,0,0]));return n.length===0?[[0,0,0],[1,1,1]]:n.length===1?[n[0],n[0]]:n}function DP(e){const r=new Uint8Array(1024),n=e.length-1;for(let i=0;i<256;i++){const a=i/255*n,s=Math.min(Math.floor(a),n-1),l=a-s,c=e[s],u=e[s+1];r[i*4]=Math.round((c[0]+(u[0]-c[0])*l)*255),r[i*4+1]=Math.round((c[1]+(u[1]-c[1])*l)*255),r[i*4+2]=Math.round((c[2]+(u[2]-c[2])*l)*255),r[i*4+3]=255}return{width:256,height:1,depth:1,data:r}}const RP={id:"gpu-gradient-map",name:"Gradient Map",category:"color",entryPoint:"gradientMapFragment",uniformSize:16,shader:`
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
}`,params:{preset:{type:"select",label:"Palette",default:"inferno",options:[{value:"inferno",label:"Inferno"},{value:"magma",label:"Magma"},{value:"plasma",label:"Plasma"},{value:"viridis",label:"Viridis"},{value:"turbo",label:"Turbo"},{value:"fire",label:"Fire"},{value:"ice",label:"Ice"},{value:"sunset",label:"Sunset"},{value:"grayscale",label:"Grayscale"},{value:"custom",label:"Custom"}]},customStops:{type:"text",label:"Custom Stops",default:OP,visibleWhen:e=>e.preset==="custom"},mix:{type:"number",label:"Mix",default:1,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([Ze(e,"mix",1),0,0,0]),dataTexture:{dimension:"2d",key:e=>{const t=e.preset??"inferno";return t==="custom"?`custom:${e.customStops??""}`:`preset:${t}`},build:e=>DP(zP(e.preset??"inferno",e.customStops??""))}};var LP=Object.freeze({__proto__:null,GRADIENT_MAP_PRESETS:ua,brightness:hP,colorWheels:TP,contrast:gP,curves:PP,exposure:yP,gradientMap:RP,grayscale:wP,hueShift:vP,invert:bP,levels:SP,powerWindow:FP,saturation:IP,secondaryQualifier:CP,sepia:kP,temperature:xP,vibrance:AP});function BP(e){const t=atob(e),r=new Uint8Array(t.length);for(let n=0;n<t.length;n++)r[n]=t.charCodeAt(n);return r}function UP(e){const t=new Uint8Array(e*e*e*4),r=e-1;let n=0;for(let i=0;i<e;i++){const o=Math.round(i/r*255);for(let a=0;a<e;a++){const s=Math.round(a/r*255);for(let l=0;l<e;l++)t[n]=Math.round(l/r*255),t[n+1]=s,t[n+2]=o,t[n+3]=255,n+=4}}return t}const Qn=2,NP={id:"gpu-lut",name:"LUT (.cube)",category:"color",entryPoint:"lutFragment",uniformSize:16,shader:`
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
}`,params:{intensity:{type:"number",label:"Intensity",default:1,min:0,max:1,step:.01,animatable:!0},lutName:{type:"json",label:"LUT Name",default:""},lutSize:{type:"json",label:"LUT Size",default:"0"},lutData:{type:"json",label:"LUT Data",default:""}},packUniforms:e=>{const t=typeof e.intensity=="number"?e.intensity:1,r=cd(e);return new Float32Array([t,r,0,0])},dataTexture:{dimension:"3d",key:e=>typeof e.lutData=="string"?e.lutData:"",build:e=>{const t=cd(e),r=typeof e.lutData=="string"?e.lutData:"";if(t>=2&&r.length>0)try{const n=BP(r);if(n.length===t*t*t*4)return{width:t,height:t,depth:t,data:n}}catch{}return{width:Qn,height:Qn,depth:Qn,data:UP(Qn)}}}};function cd(e){const t=e.lutSize,r=typeof t=="number"?t:typeof t=="string"?Number(t):0;return!Number.isFinite(r)||r<2||r>129?Qn:Math.floor(r)}var $P=Object.freeze({__proto__:null,lut3d:NP});const HP={id:"gpu-gaussian-blur",name:"Gaussian Blur",category:"blur",entryPoint:"gaussianBlurFragment",uniformSize:16,shader:`
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
}`,params:{radius:{type:"number",label:"Radius",default:10,min:0,max:50,step:1,animatable:!0},samples:{type:"number",label:"Samples",default:5,min:1,max:64,step:1,animatable:!1,quality:!0}},packUniforms:(e,t,r)=>new Float32Array([e.radius??10,t,r,e.samples??5])},jP={id:"gpu-box-blur",name:"Box Blur",category:"blur",entryPoint:"boxBlurFragment",uniformSize:16,shader:`
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
}`,params:{radius:{type:"number",label:"Radius",default:5,min:0,max:20,step:1,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.radius??5,t,r,0])},qP={id:"gpu-motion-blur",name:"Motion Blur",category:"blur",entryPoint:"motionBlurFragment",uniformSize:32,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:.05,min:0,max:.3,step:.005,animatable:!0},angle:{type:"number",label:"Angle",default:0,min:0,max:6.28318,step:.01,animatable:!0},samples:{type:"number",label:"Samples",default:16,min:4,max:32,step:1,animatable:!1,quality:!0},shutterAngle:{type:"number",label:"Shutter Angle",default:180,min:0,max:360,step:1,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.05,e.angle??0,e.shutterAngle??(Object.keys(e).length>0?360:180),e.samples??16,.2,0,0,0])},KP={id:"gpu-radial-blur",name:"Radial Blur",category:"blur",entryPoint:"radialBlurFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:.5,min:0,max:2,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},samples:{type:"number",label:"Samples",default:32,min:4,max:256,step:1,animatable:!1,quality:!0}},packUniforms:e=>new Float32Array([e.amount??.5,e.centerX??.5,e.centerY??.5,e.samples??32])},GP={id:"gpu-zoom-blur",name:"Zoom Blur",category:"blur",entryPoint:"zoomBlurFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:.3,min:0,max:1,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},samples:{type:"number",label:"Samples",default:16,min:4,max:256,step:1,animatable:!1,quality:!0}},packUniforms:e=>new Float32Array([e.amount??.3,e.centerX??.5,e.centerY??.5,e.samples??16])};var WP=Object.freeze({__proto__:null,boxBlur:jP,gaussianBlur:HP,motionBlur:qP,radialBlur:KP,zoomBlur:GP});function ur(e,t){if(!e.startsWith("#"))return t;const r=e.slice(1);if(r.length===3||r.length===4){const n=r.split("").map(i=>parseInt(i+i,16)/255);return n.slice(0,3).every(i=>Number.isFinite(i))?[n[0]??t[0],n[1]??t[1],n[2]??t[2],n[3]??1]:t}if(r.length===6||r.length===8){const n=[parseInt(r.slice(0,2),16)/255,parseInt(r.slice(2,4),16)/255,parseInt(r.slice(4,6),16)/255,r.length===8?parseInt(r.slice(6,8),16)/255:1];if(n.every(i=>Number.isFinite(i)))return n}return t}const VP={id:"gpu-pixelate",name:"Pixelate",category:"distort",entryPoint:"pixelateFragment",uniformSize:16,shader:`
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
}`,params:{size:{type:"number",label:"Pixel Size",default:8,min:1,max:64,step:1,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.size??8,t,r,0])},ZP={id:"gpu-rgb-split",name:"RGB Split",category:"distort",entryPoint:"rgbSplitFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:.01,min:0,max:.1,step:.001,animatable:!0},angle:{type:"number",label:"Angle",default:0,min:0,max:6.28318,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.01,e.angle??0,0,0])},YP={id:"gpu-twirl",name:"Twirl",category:"distort",entryPoint:"twirlFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:1,min:-10,max:10,step:.1,animatable:!0},radius:{type:"number",label:"Radius",default:.5,min:.1,max:1,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??1,e.radius??.5,e.centerX??.5,e.centerY??.5])},XP={id:"gpu-wave",name:"Wave",category:"distort",entryPoint:"waveFragment",uniformSize:16,shader:`
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
}`,params:{amplitudeX:{type:"number",label:"Horizontal Amp",default:.02,min:0,max:.1,step:.001,animatable:!0},amplitudeY:{type:"number",label:"Vertical Amp",default:.02,min:0,max:.1,step:.001,animatable:!0},frequencyX:{type:"number",label:"Horizontal Freq",default:5,min:1,max:20,step:.5,animatable:!0},frequencyY:{type:"number",label:"Vertical Freq",default:5,min:1,max:20,step:.5,animatable:!0}},packUniforms:e=>new Float32Array([e.amplitudeX??.02,e.amplitudeY??.02,e.frequencyX??5,e.frequencyY??5])},QP={id:"gpu-trigger-wave",name:"Trigger Wave",category:"distort",entryPoint:"triggerWaveFragment",uniformSize:64,shader:`
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
}`,params:{strength:{type:"number",label:"Strength",default:.035,min:-.15,max:.15,step:.001,animatable:!0},radius:{type:"number",label:"Radius",default:.85,min:.1,max:1.5,step:.01,animatable:!0},frequency:{type:"number",label:"Frequency",default:18,min:2,max:64,step:1,animatable:!0},decay:{type:"number",label:"Decay",default:.08,min:.01,max:.3,step:.01,animatable:!0},phase:{type:"number",label:"Phase",default:0,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:0,max:4,step:.1,animatable:!1},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},chroma:{type:"number",label:"Chroma",default:.006,min:0,max:.05,step:.001,animatable:!0},scanlineMix:{type:"number",label:"Scanlines",default:.18,min:0,max:1,step:.01,animatable:!0},glowColor:{type:"color",label:"Glow Color",default:"#2e6b8c",animatable:!0}},packUniforms:(e,t,r)=>{const n=performance.now()/1e3,i=ur(e.glowColor??"#2e6b8c",[.18,.42,.55,1]);return new Float32Array([e.strength??.035,e.radius??.85,e.frequency??18,e.decay??.08,e.centerX??.5,e.centerY??.5,e.phase??0,e.speed??1,e.chroma??.006,e.scanlineMix??.18,n,t/Math.max(r,1),i[0],i[1],i[2],i[3]])}},JP={id:"gpu-bulge",name:"Bulge/Pinch",category:"distort",entryPoint:"bulgeFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:.5,min:.1,max:3,step:.1,animatable:!0},radius:{type:"number",label:"Radius",default:.5,min:.1,max:1,step:.01,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.5,e.radius??.5,e.centerX??.5,e.centerY??.5])},eT={id:"gpu-kaleidoscope",name:"Kaleidoscope",category:"distort",entryPoint:"kaleidoscopeFragment",uniformSize:16,shader:`
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
}`,params:{segments:{type:"number",label:"Segments",default:6,min:2,max:16,step:1,animatable:!0},rotation:{type:"number",label:"Rotation",default:0,min:0,max:6.28318,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.segments??6,e.rotation??0,0,0])},tT={id:"gpu-mirror",name:"Mirror",category:"distort",entryPoint:"mirrorFragment",uniformSize:16,shader:`
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
}`,params:{horizontal:{type:"boolean",label:"Horizontal",default:!0},vertical:{type:"boolean",label:"Vertical",default:!1}},packUniforms:e=>new Float32Array([e.horizontal!==!1?1:0,e.vertical===!0?1:0,0,0])},rT={id:"gpu-fluted-glass",name:"Fluted Glass",category:"distort",entryPoint:"flutedGlassFragment",uniformSize:128,shader:`
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
}`,params:{colorBack:{type:"color",label:"Back Color",default:"#00000000",animatable:!0},colorShadow:{type:"color",label:"Shadow Color",default:"#000000",animatable:!0},colorHighlight:{type:"color",label:"Highlight Color",default:"#ffffff",animatable:!0},shadows:{type:"number",label:"Shadows",default:.25,min:0,max:1,step:.01,animatable:!0},highlights:{type:"number",label:"Highlights",default:.1,min:0,max:1,step:.01,animatable:!0},size:{type:"number",label:"Size",default:.5,min:0,max:1,step:.01,animatable:!0},shape:{type:"select",label:"Shape",default:"lines",options:[{value:"lines",label:"Lines"},{value:"linesIrregular",label:"Irregular Lines"},{value:"wave",label:"Wave"},{value:"zigzag",label:"Zigzag"},{value:"pattern",label:"Pattern"}]},angle:{type:"number",label:"Angle",default:0,min:0,max:180,step:1,animatable:!0},distortionShape:{type:"select",label:"Distortion Shape",default:"prism",options:[{value:"prism",label:"Prism"},{value:"lens",label:"Lens"},{value:"contour",label:"Contour"},{value:"cascade",label:"Cascade"},{value:"flat",label:"Flat"}]},distortion:{type:"number",label:"Distortion",default:.5,min:0,max:1,step:.01,animatable:!0},shift:{type:"number",label:"Shift",default:0,min:-1,max:1,step:.01,animatable:!0},stretch:{type:"number",label:"Stretch",default:0,min:0,max:1,step:.01,animatable:!0},blur:{type:"number",label:"Blur",default:0,min:0,max:1,step:.01,animatable:!0},edges:{type:"number",label:"Edges",default:.25,min:0,max:1,step:.01,animatable:!0},margin:{type:"number",label:"Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginLeft:{type:"number",label:"Left Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginRight:{type:"number",label:"Right Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginTop:{type:"number",label:"Top Margin",default:0,min:0,max:1,step:.01,animatable:!0},marginBottom:{type:"number",label:"Bottom Margin",default:0,min:0,max:1,step:.01,animatable:!0},grainMixer:{type:"number",label:"Grain Mixer",default:0,min:0,max:1,step:.01,animatable:!0},grainOverlay:{type:"number",label:"Grain Overlay",default:0,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n={lines:1,linesIrregular:2,wave:3,zigzag:4,pattern:5},i={prism:1,lens:2,contour:3,cascade:4,flat:5},o=e.margin??0,a=e.marginLeft??o,s=e.marginRight??o,l=e.marginTop??o,c=e.marginBottom??o,u=ur(e.colorBack??"#00000000",[0,0,0,0]),d=ur(e.colorShadow??"#000000",[0,0,0,1]),f=ur(e.colorHighlight??"#ffffff",[1,1,1,1]);return new Float32Array([u[0],u[1],u[2],u[3],d[0],d[1],d[2],d[3],f[0],f[1],f[2],f[3],e.size??.5,e.shadows??.25,e.angle??0,e.stretch??0,n[e.shape]??1,e.distortion??.5,e.highlights??.1,i[e.distortionShape]??1,e.shift??0,e.blur??0,e.edges??.25,e.grainMixer??0,e.grainOverlay??0,t,r,t/Math.max(r,1),a,l,s,c])}},nT={id:"gpu-ripple-glass",name:"Ripple Glass",category:"distort",entryPoint:"rippleGlassFragment",uniformSize:80,shader:`
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
}`,params:{colorShadow:{type:"color",label:"Shadow Color",default:"#000000",animatable:!0},colorHighlight:{type:"color",label:"Highlight Color",default:"#ffffff",animatable:!0},amount:{type:"number",label:"Amount",default:.5,min:0,max:1,step:.01,animatable:!0},rings:{type:"number",label:"Rings",default:14,min:1,max:64,step:1,animatable:!0},shadows:{type:"number",label:"Shadows",default:.25,min:0,max:1,step:.01,animatable:!0},highlights:{type:"number",label:"Highlights",default:.1,min:0,max:1,step:.01,animatable:!0},originX:{type:"number",label:"Origin X",default:.5,min:0,max:1,step:.01,animatable:!0},originY:{type:"number",label:"Origin Y",default:.5,min:0,max:1,step:.01,animatable:!0},phase:{type:"number",label:"Phase",default:0,min:-1,max:1,step:.01,animatable:!0},falloff:{type:"number",label:"Falloff",default:.35,min:.05,max:2,step:.01,animatable:!0},aberration:{type:"number",label:"Aberration",default:0,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n=ur(e.colorShadow??"#000000",[0,0,0,1]),i=ur(e.colorHighlight??"#ffffff",[1,1,1,1]);return new Float32Array([n[0],n[1],n[2],n[3],i[0],i[1],i[2],i[3],e.amount??.5,e.rings??14,e.shadows??.25,e.highlights??.1,e.originX??.5,e.originY??.5,e.phase??0,e.falloff??.35,e.aberration??0,t,r,t/Math.max(r,1)])}},iT={id:"gpu-glass-mosaic",name:"Glass Mosaic",category:"distort",entryPoint:"glassMosaicFragment",uniformSize:64,shader:`
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
}`,params:{colorShadow:{type:"color",label:"Shadow Color",default:"#000000",animatable:!0},colorHighlight:{type:"color",label:"Highlight Color",default:"#ffffff",animatable:!0},amount:{type:"number",label:"Amount",default:.55,min:0,max:1,step:.01,animatable:!0},cells:{type:"number",label:"Cells",default:18,min:2,max:80,step:1,animatable:!0},shadows:{type:"number",label:"Shadows",default:.3,min:0,max:1,step:.01,animatable:!0},highlights:{type:"number",label:"Highlights",default:.12,min:0,max:1,step:.01,animatable:!0},aberration:{type:"number",label:"Aberration",default:0,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n=ur(e.colorShadow??"#000000",[0,0,0,1]),i=ur(e.colorHighlight??"#ffffff",[1,1,1,1]);return new Float32Array([n[0],n[1],n[2],n[3],i[0],i[1],i[2],i[3],e.amount??.55,e.cells??18,e.shadows??.3,e.highlights??.12,e.aberration??0,t,r,t/Math.max(r,1)])}},oT={id:"gpu-blocks",name:"Blocks",category:"distort",entryPoint:"blocksFragment",uniformSize:32,shader:`
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
}`,params:{size:{type:"number",label:"Block Size",default:24,min:4,max:120,step:1,animatable:!0},depth:{type:"number",label:"Depth",default:.5,min:0,max:1.5,step:.01,animatable:!0},studSize:{type:"number",label:"Stud Size",default:.55,min:0,max:1,step:.01,animatable:!0},gap:{type:"number",label:"Gap",default:.06,min:0,max:.4,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.size??24,e.depth??.5,e.studSize??.55,e.gap??.06,t,r,0,0])},aT={id:"gpu-droste",name:"Droste",category:"distort",entryPoint:"drosteFragment",uniformSize:32,shader:`
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
}`,params:{strength:{type:"number",label:"Spiral",default:1,min:0,max:2,step:.01,animatable:!0},scale:{type:"number",label:"Scale",default:2,min:1.1,max:6,step:.05,animatable:!0},centerX:{type:"number",label:"Center X",default:.5,min:0,max:1,step:.01,animatable:!0},centerY:{type:"number",label:"Center Y",default:.5,min:0,max:1,step:.01,animatable:!0},spin:{type:"number",label:"Spin",default:0,min:-6.28318,max:6.28318,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.strength??1,e.scale??2,e.centerX??.5,e.centerY??.5,e.spin??0,t,r,0])};var sT=Object.freeze({__proto__:null,blocks:oT,bulge:JP,droste:aT,flutedGlass:rT,glassMosaic:iT,kaleidoscope:eT,mirror:tT,pixelate:VP,rgbSplit:ZP,rippleGlass:nT,triggerWave:QP,twirl:YP,wave:XP});const lT={id:"gpu-vignette",name:"Vignette",category:"stylize",entryPoint:"vignetteFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:.5,min:0,max:1,step:.01,animatable:!0},size:{type:"number",label:"Size",default:.5,min:0,max:1.5,step:.01,animatable:!0},softness:{type:"number",label:"Softness",default:.5,min:0,max:1,step:.01,animatable:!0},roundness:{type:"number",label:"Roundness",default:1,min:.5,max:2,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.amount??.5,e.size??.5,e.softness??.5,e.roundness??1])},cT={id:"gpu-grain",name:"Film Grain",category:"stylize",entryPoint:"grainFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:.1,min:0,max:.5,step:.01,animatable:!0},size:{type:"number",label:"Size",default:1,min:.5,max:5,step:.1,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:0,max:5,step:.1,animatable:!1}},packUniforms:e=>{const t=performance.now()/1e3;return new Float32Array([e.amount??.1,e.size??1,e.speed??1,t])}},uT={id:"gpu-sharpen",name:"Sharpen",category:"stylize",entryPoint:"sharpenFragment",uniformSize:16,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:5,step:.1,animatable:!0},radius:{type:"number",label:"Radius",default:1,min:.5,max:5,step:.1,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.amount??1,e.radius??1,t,r])},dT={id:"gpu-posterize",name:"Posterize",category:"stylize",entryPoint:"posterizeFragment",uniformSize:16,shader:`
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
}`,params:{levels:{type:"number",label:"Levels",default:6,min:2,max:32,step:1,animatable:!0}},packUniforms:e=>new Float32Array([e.levels??6,0,0,0])},fT={id:"gpu-glow",name:"Glow",category:"stylize",entryPoint:"glowFragment",uniformSize:32,shader:`
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
}`,params:{amount:{type:"number",label:"Amount",default:1,min:0,max:5,step:.1,animatable:!0},threshold:{type:"number",label:"Threshold",default:.6,min:0,max:1,step:.01,animatable:!0},radius:{type:"number",label:"Radius",default:20,min:1,max:100,step:1,animatable:!0},softness:{type:"number",label:"Softness",default:.5,min:.1,max:1,step:.05,animatable:!0},rings:{type:"number",label:"Rings",default:4,min:1,max:32,step:1,animatable:!1,quality:!0},samplesPerRing:{type:"number",label:"Samples/Ring",default:16,min:4,max:64,step:1,animatable:!1,quality:!0}},packUniforms:(e,t,r)=>new Float32Array([e.amount??1,e.threshold??.6,e.radius??20,e.softness??.5,t,r,e.rings??4,e.samplesPerRing??16])},pT={id:"gpu-edge-detect",name:"Edge Detect",category:"stylize",entryPoint:"edgeDetectFragment",uniformSize:16,shader:`
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
}`,params:{strength:{type:"number",label:"Strength",default:1,min:0,max:5,step:.1,animatable:!0},invert:{type:"boolean",label:"Invert",default:!1}},packUniforms:(e,t,r)=>new Float32Array([e.strength??1,t,r,e.invert?1:0])},mT={id:"gpu-scanlines",name:"Scanlines",category:"stylize",entryPoint:"scanlinesFragment",uniformSize:16,shader:`
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
}`,params:{density:{type:"number",label:"Density",default:5,min:1,max:20,step:.5,animatable:!0},opacity:{type:"number",label:"Opacity",default:.3,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Scroll Speed",default:0,min:0,max:5,step:.1,animatable:!1}},packUniforms:e=>{const t=performance.now()/1e3;return new Float32Array([e.density??5,e.opacity??.3,e.speed??0,t])}},hT={id:"gpu-color-glitch",name:"Color Glitch",category:"stylize",entryPoint:"colorGlitchFragment",uniformSize:16,shader:`
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
}`,params:{intensity:{type:"number",label:"Intensity",default:.5,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:.1,max:5,step:.1,animatable:!1}},packUniforms:e=>{const t=performance.now()/1e3;return new Float32Array([e.intensity??.5,e.speed??1,t,0])}},gT={id:"gpu-block-glitch",name:"Block Glitch",category:"stylize",entryPoint:"blockGlitchFragment",uniformSize:32,shader:`
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
}`,params:{coverage:{type:"number",label:"Coverage",default:.3,min:0,max:1,step:.01,animatable:!0},intensity:{type:"number",label:"Intensity",default:.6,min:0,max:1,step:.01,animatable:!0},blockSize:{type:"number",label:"Block Size",default:40,min:8,max:200,step:1,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:.1,max:5,step:.1,animatable:!1}},packUniforms:(e,t,r)=>new Float32Array([e.coverage??.3,e.intensity??.6,e.blockSize??40,e.speed??1,performance.now()/1e3,t,r,0])},yT={id:"gpu-crt",name:"CRT",category:"stylize",entryPoint:"crtFragment",uniformSize:32,shader:`
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
}`,params:{curvature:{type:"number",label:"Curvature",default:.3,min:0,max:1,step:.01,animatable:!0},scanlines:{type:"number",label:"Scanlines",default:.3,min:0,max:1,step:.01,animatable:!0},vignette:{type:"number",label:"Vignette",default:.3,min:0,max:1,step:.01,animatable:!0},chroma:{type:"number",label:"Chroma",default:.4,min:0,max:2,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.curvature??.3,e.scanlines??.3,e.vignette??.3,e.chroma??.4,t,r,0,0])};function vT(e){return Math.min(1,Math.max(0,e))}function kn(e,t){if(!e.startsWith("#"))return t;const r=e.slice(1);if(r.length===3||r.length===4){const n=r.split("").map(i=>parseInt(i+i,16)/255);return[n[0]??t[0],n[1]??t[1],n[2]??t[2],n[3]??1]}if(r.length===6||r.length===8){const n=[parseInt(r.slice(0,2),16)/255,parseInt(r.slice(2,4),16)/255,parseInt(r.slice(4,6),16)/255,r.length===8?parseInt(r.slice(6,8),16)/255:1];if(n.every(i=>Number.isFinite(i)))return n}return t}function bT(e,t){if(!e||e<=0||!Number.isFinite(e))return .5;const r=Math.max(7,Math.min(300,t/e)),n=vT((300-r)/293);return Math.pow(n,1/.7)}function ST(e,t){return!e||!t||t<=0?1.25:Math.max(0,Math.min(2,e/t*2))}const IT={id:"gpu-halftone",name:"Halftone",category:"stylize",entryPoint:"halftoneFragment",uniformSize:80,shader:`
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
}`,params:{colorFront:{type:"color",label:"Front Color",default:"#2b2b2b",animatable:!0},colorBack:{type:"color",label:"Back Color",default:"#f2f1e8",animatable:!0},originalColors:{type:"boolean",label:"Original Colors",default:!1},inverted:{type:"boolean",label:"Inverted",default:!1},grid:{type:"select",label:"Grid",default:"hex",options:[{value:"hex",label:"Hex"},{value:"square",label:"Square"}]},type:{type:"select",label:"Type",default:"gooey",options:[{value:"classic",label:"Classic"},{value:"gooey",label:"Gooey"},{value:"holes",label:"Holes"},{value:"soft",label:"Soft"}]},size:{type:"number",label:"Size",default:.5,min:0,max:1,step:.01,animatable:!0},radius:{type:"number",label:"Radius",default:1.25,min:0,max:2,step:.01,animatable:!0},contrast:{type:"number",label:"Contrast",default:.4,min:0,max:1,step:.01,animatable:!0},grainMixer:{type:"number",label:"Grain Mixer",default:.2,min:0,max:1,step:.01,animatable:!0},grainOverlay:{type:"number",label:"Grain Overlay",default:.2,min:0,max:1,step:.01,animatable:!0},grainSize:{type:"number",label:"Grain Size",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>{const n={square:0,hex:1},i={classic:0,gooey:1,holes:2,soft:3},o=kn(e.colorFront??"#2b2b2b",[43/255,43/255,43/255,1]),a=kn(e.colorBack??"#f2f1e8",[242/255,241/255,232/255,1]),s=e.size??bT(e.spacing,r),l=e.radius??ST(e.dotSize,e.spacing),c=e.contrast??.4,u=e.originalColors??!1,d=e.inverted??e.invert??!1,f=e.grainMixer??.2,p=e.grainOverlay??.2,m=e.grainSize??.5,h=e.grid??"hex",g=e.type??e.dotStyle??"gooey";return new Float32Array([o[0],o[1],o[2],o[3],a[0],a[1],a[2],a[3],s,l,c,t/Math.max(r,1),u?1:0,d?1:0,f,p,m,n[h]??1,i[g]??1,0])}},ud={bayer2:0,bayer4:1,bayer8:2,halftone:3,lines:4,crosses:5,dots:6,grid:7,scales:8},dd={image:0,linear:1,radial:2},fd={threshold:0,scaled:1},pd={circle:0,square:1,diamond:2},md={bw:0,gameboy:1,cga:2,sepia:3},hd={standard:0,simple:1,blocks:2,dots:3,minimal:4},xT={ascii:"@%#*+=-:. ",dense:"@WB#$oahkbn+=-:. ",binary:"01",symbols:"#@&$%*+!=;:-. "},wT=24,ng=64,gd={monospace:"monospace",courier:'"Courier New", Courier, monospace',consolas:'Consolas, "Lucida Console", monospace',lucida:'"Lucida Console", Monaco, monospace'};function co(e,t){if(e==="custom"){const r=[...t].slice(0,ng);return r.length>0?r.join(""):null}return xT[e]??null}function kT(e,t){const r=[...e].slice(0,ng),n=wT,o=Math.max(r.length,1)*n,a=n,s=new Uint8Array(o*a*4),c=(typeof OffscreenCanvas<"u"?new OffscreenCanvas(o,a):null)?.getContext("2d",{willReadFrequently:!0});if(!c)return s.fill(255),{width:o,height:a,depth:1,data:s};const u=gd[t]??gd.monospace;c.clearRect(0,0,o,a),c.fillStyle="#ffffff",c.font=`${Math.round(n*.82)}px ${u}`,c.textAlign="center",c.textBaseline="middle";for(let f=0;f<r.length;f++)c.fillText(r[f],f*n+n/2,n/2+1);const d=c.getImageData(0,0,o,a).data;for(let f=0;f<o*a;f++){const p=d[f*4+3]??0;s[f*4]=p,s[f*4+1]=p,s[f*4+2]=p,s[f*4+3]=p}return{width:o,height:a,depth:1,data:s}}const PT={id:"gpu-dither",name:"Dither",category:"stylize",entryPoint:"ditherFragment",uniformSize:48,shader:`
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
}`,params:{pattern:{type:"select",label:"Pattern",default:"bayer4",options:[{value:"bayer2",label:"Bayer 2x2"},{value:"bayer4",label:"Bayer 4x4"},{value:"bayer8",label:"Bayer 8x8"},{value:"halftone",label:"Halftone"},{value:"lines",label:"Lines"},{value:"crosses",label:"Crosses"},{value:"dots",label:"Dots"},{value:"grid",label:"Grid"},{value:"scales",label:"Scales"}]},mode:{type:"select",label:"Mode",default:"image",options:[{value:"image",label:"Image"},{value:"linear",label:"Linear"},{value:"radial",label:"Radial"}]},style:{type:"select",label:"Style",default:"threshold",options:[{value:"threshold",label:"Threshold"},{value:"scaled",label:"Scaled"}]},shape:{type:"select",label:"Shape",default:"square",options:[{value:"circle",label:"Circle"},{value:"square",label:"Square"},{value:"diamond",label:"Diamond"}]},palette:{type:"select",label:"Palette",default:"gameboy",options:[{value:"bw",label:"B&W"},{value:"gameboy",label:"Game Boy"},{value:"cga",label:"CGA"},{value:"sepia",label:"Sepia"}]},cellSize:{type:"number",label:"Cell Size",default:8,min:2,max:32,step:1,animatable:!0},angle:{type:"number",label:"Angle",default:45,min:0,max:360,step:1,animatable:!0,visibleWhen:e=>e.mode==="linear"},scale:{type:"number",label:"Scale",default:100,min:25,max:200,step:1,animatable:!0,visibleWhen:e=>e.mode==="radial"},offsetX:{type:"number",label:"Offset X",default:0,min:-100,max:100,step:1,animatable:!0,visibleWhen:e=>e.mode==="radial"},offsetY:{type:"number",label:"Offset Y",default:0,min:-100,max:100,step:1,animatable:!0,visibleWhen:e=>e.mode==="radial"}},packUniforms:(e,t,r)=>new Float32Array([e.cellSize??8,e.angle??45,e.scale??100,t,r,e.offsetX??0,e.offsetY??0,ud[e.pattern]??ud.bayer4??0,dd[e.mode]??dd.image??0,fd[e.style]??fd.threshold??0,pd[e.shape]??pd.square??0,md[e.palette]??md.gameboy??0])},TT={id:"gpu-ascii",name:"ASCII",category:"stylize",entryPoint:"asciiFragment",uniformSize:96,shader:`
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
}`,params:{charSet:{type:"select",label:"Character Set",default:"ascii",options:[{value:"ascii",label:"ASCII Ramp"},{value:"dense",label:"Dense"},{value:"binary",label:"Binary"},{value:"symbols",label:"Symbols"},{value:"custom",label:"Custom"},{value:"standard",label:"Standard (shapes)"},{value:"simple",label:"Simple (shapes)"},{value:"blocks",label:"Blocks (shapes)"},{value:"dots",label:"Dots (shapes)"},{value:"minimal",label:"Minimal (shapes)"}]},customChars:{type:"text",label:"Custom Characters",default:"FREECUT 01",visibleWhen:e=>e.charSet==="custom"},font:{type:"select",label:"Font",default:"monospace",options:[{value:"monospace",label:"Monospace"},{value:"courier",label:"Courier"},{value:"consolas",label:"Consolas"},{value:"lucida",label:"Lucida Console"}],visibleWhen:e=>co(e.charSet??"ascii",e.customChars??"")!==null},fontSize:{type:"number",label:"Font Size",default:8,min:4,max:24,step:1,animatable:!0},letterSpacing:{type:"number",label:"Letter Spacing",default:0,min:-2,max:5,step:.1,animatable:!0},lineHeight:{type:"number",label:"Line Height",default:1,min:.5,max:2,step:.1,animatable:!0},matchSourceColor:{type:"boolean",label:"Match Source Color",default:!0},textColor:{type:"color",label:"Text Color",default:"#ffffff",animatable:!0,visibleWhen:e=>e.matchSourceColor!==!0},bgColor:{type:"color",label:"Background",default:"#0a0a0f",animatable:!0,visibleWhen:e=>e.transparentBg!==!0},transparentBg:{type:"boolean",label:"Transparent Background",default:!1},edgeDetect:{type:"boolean",label:"Edge Detection",default:!1},colorSaturation:{type:"number",label:"Saturation",default:100,min:0,max:200,step:1,animatable:!0,visibleWhen:e=>e.matchSourceColor===!0},asciiOpacity:{type:"number",label:"ASCII Opacity",default:100,min:0,max:100,step:1,animatable:!0},originalOpacity:{type:"number",label:"Original Opacity",default:0,min:0,max:100,step:1,animatable:!0},contrast:{type:"number",label:"Contrast",default:100,min:50,max:200,step:1,animatable:!0},brightness:{type:"number",label:"Brightness",default:0,min:-100,max:100,step:1,animatable:!0},invert:{type:"boolean",label:"Invert",default:!1}},packUniforms:(e,t,r)=>{const n=kn(e.textColor??"#ffffff",[1,1,1,1]),i=kn(e.bgColor??"#0a0a0f",[10/255,10/255,15/255,1]),o=co(e.charSet??"ascii",e.customChars??""),a=o?[...o].length:0;return new Float32Array([e.fontSize??8,e.letterSpacing??0,e.lineHeight??1,hd[e.charSet]??hd.standard??0,e.matchSourceColor===!1?0:1,e.invert===!0?1:0,(e.asciiOpacity??100)/100,(e.originalOpacity??0)/100,(e.contrast??100)/100,(e.brightness??0)/255,(e.colorSaturation??100)/100,t,r,e.transparentBg===!0?1:0,e.edgeDetect===!0?1:0,a,n[0],n[1],n[2],n[3],i[0],i[1],i[2],i[3]])},dataTexture:{dimension:"2d",key:e=>{const t=co(e.charSet??"ascii",e.customChars??"");return t?`${t}|${e.font??"monospace"}`:""},build:e=>{const t=co(e.charSet??"ascii",e.customChars??"");return t?kT(t,e.font??"monospace"):{width:1,height:1,depth:1,data:new Uint8Array([0,0,0,0])}}}},ET={id:"gpu-threshold",name:"Threshold",category:"stylize",entryPoint:"thresholdFragment",uniformSize:16,shader:`
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
}`,params:{level:{type:"number",label:"Level",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>new Float32Array([e.level??.5,0,0,0])},CT={id:"gpu-vhs",name:"VHS",category:"stylize",entryPoint:"vhsFragment",uniformSize:32,shader:`
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
}`,params:{bleed:{type:"number",label:"Chroma Bleed",default:.4,min:0,max:2,step:.01,animatable:!0},waviness:{type:"number",label:"Tracking",default:.3,min:0,max:2,step:.01,animatable:!0},noise:{type:"number",label:"Tape Noise",default:.25,min:0,max:1,step:.01,animatable:!0},scanline:{type:"number",label:"Scanlines",default:.35,min:0,max:1,step:.01,animatable:!0},speed:{type:"number",label:"Speed",default:1,min:0,max:4,step:.1,animatable:!1}},packUniforms:(e,t,r)=>new Float32Array([e.bleed??.4,e.waviness??.3,e.noise??.25,e.scanline??.35,performance.now()/1e3*(e.speed??1),t,r,0])},MT={id:"gpu-ink",name:"Ink",category:"stylize",entryPoint:"inkFragment",uniformSize:64,shader:`
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
}`,params:{strength:{type:"number",label:"Ink Amount",default:1,min:0,max:1,step:.01,animatable:!0},spacing:{type:"number",label:"Line Spacing",default:6,min:2,max:24,step:.5,animatable:!0},thickness:{type:"number",label:"Line Width",default:1.2,min:.5,max:4,step:.1,animatable:!0},edgeStrength:{type:"number",label:"Outline",default:1.5,min:0,max:5,step:.1,animatable:!0},tone:{type:"number",label:"Shading",default:1,min:.2,max:2.5,step:.05,animatable:!0},inkColor:{type:"color",label:"Ink",default:"#141414",animatable:!0},paperColor:{type:"color",label:"Paper",default:"#f4f1e8",animatable:!0}},packUniforms:(e,t,r)=>{const n=kn(e.inkColor??"#141414",[.08,.08,.08,1]),i=kn(e.paperColor??"#f4f1e8",[.96,.95,.91,1]);return new Float32Array([e.strength??1,e.spacing??6,e.thickness??1.2,e.edgeStrength??1.5,e.tone??1,t,r,0,n[0],n[1],n[2],i[0],i[1],i[2],0,0])}},_T={id:"gpu-pixel-sort",name:"Pixel Sort (Streak)",category:"stylize",entryPoint:"pixelSortFragment",uniformSize:32,shader:`
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
}`,params:{direction:{type:"select",label:"Direction",default:"right",options:[{value:"right",label:"Right"},{value:"left",label:"Left"},{value:"down",label:"Down"},{value:"up",label:"Up"}]},order:{type:"select",label:"Carry",default:"bright",options:[{value:"bright",label:"Bright streaks"},{value:"dark",label:"Dark streaks"}]},low:{type:"number",label:"Threshold Low",default:.25,min:0,max:1,step:.01,animatable:!0},high:{type:"number",label:"Threshold High",default:1,min:0,max:1,step:.01,animatable:!0},length:{type:"number",label:"Length",default:60,min:2,max:400,step:1,quality:!0}},packUniforms:(e,t,r)=>{const n=e.direction??"right";let i=1,o=0;return n==="left"?(i=-1,o=0):n==="down"?(i=0,o=1):n==="up"&&(i=0,o=-1),new Float32Array([i,o,e.low??.25,e.high??1,e.length??60,e.order==="dark"?0:1,t,r])}},FT={id:"gpu-pixel-sort-hq",name:"Pixel Sort",category:"stylize",entryPoint:"pixelSortHqMain",uniformSize:32,compute:{dispatch:(e,t)=>[Math.ceil(e/8),Math.ceil(t/8),1]},shader:`
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
}`,params:{orientation:{type:"select",label:"Orientation",default:"horizontal",options:[{value:"horizontal",label:"Horizontal"},{value:"vertical",label:"Vertical"}]},order:{type:"select",label:"Order",default:"ascending",options:[{value:"ascending",label:"Dark → Bright"},{value:"descending",label:"Bright → Dark"}]},low:{type:"number",label:"Threshold Low",default:.25,min:0,max:1,step:.01,animatable:!0},high:{type:"number",label:"Threshold High",default:.9,min:0,max:1,step:.01,animatable:!0}},packUniforms:(e,t,r)=>new Float32Array([e.low??.25,e.high??.9,e.orientation==="vertical"?1:0,e.order==="descending"?1:0,t,r,0,0])};var AT=Object.freeze({__proto__:null,ascii:TT,blockGlitch:gT,colorGlitch:hT,crt:yT,dither:PT,edgeDetect:pT,glow:fT,grain:cT,halftone:IT,ink:MT,pixelSort:_T,pixelSortHq:FT,posterize:dT,scanlines:mT,sharpen:uT,threshold:ET,vhs:CT,vignette:lT});function OT(e){if(typeof e=="string"){const t=/^#([0-9a-f]{6})(?:[0-9a-f]{2})?$/i.exec(e.trim());if(t){const r=t[1];return[parseInt(r.slice(0,2),16)/255,parseInt(r.slice(2,4),16)/255,parseInt(r.slice(4,6),16)/255]}}return[0,1,0]}const zT={id:"gpu-chroma-key",name:"Chroma Key",category:"keying",entryPoint:"chromaKeyFragment",uniformSize:32,shader:`
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
}`,params:{keyColor:{type:"color",label:"Key Color",default:"#00ff00",animatable:!0},tolerance:{type:"number",label:"Tolerance",default:.2,min:0,max:1,step:.01,animatable:!0},softness:{type:"number",label:"Edge Softness",default:.1,min:0,max:.5,step:.01,animatable:!0},spillSuppression:{type:"number",label:"Spill Suppression",default:.5,min:0,max:1,step:.01,animatable:!0}},packUniforms:e=>{const[t,r,n]=OT(e.keyColor);return new Float32Array([t,r,n,e.tolerance??.2,e.softness??.1,e.spillSuppression??.5,0,0])}};var DT=Object.freeze({__proto__:null,chromaKey:zT});const ig=new Map,RT={color:[],blur:[],distort:[],stylize:[],keying:[]};function LT(e){return typeof e=="object"&&e!==null&&"id"in e&&"name"in e&&"category"in e&&"shader"in e&&"entryPoint"in e&&"packUniforms"in e}function Fn(e){Object.values(e).forEach(t=>{LT(t)&&(ig.set(t.id,t),RT[t.category]?.push(t))})}Fn(LP);Fn($P);Fn(WP);Fn(sT);Fn(AT);Fn(DT);function BT(e){return ig.get(e)}const UT={id:"dissolve",name:"Cross Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"dissolveFragment",uniformSize:16,shader:`
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
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},Bi=`
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var leftTex: texture_2d<f32>;
@group(0) @binding(2) var rightTex: texture_2d<f32>;
`,og=`
fn sampleSoft(tex: texture_2d<f32>, uv: vec2f, radius: vec2f) -> vec4f {
  let center = textureSample(tex, texSampler, uv);
  let a = textureSample(tex, texSampler, clamp(uv + vec2f(radius.x, 0.0), vec2f(0.0), vec2f(1.0)));
  let b = textureSample(tex, texSampler, clamp(uv - vec2f(radius.x, 0.0), vec2f(0.0), vec2f(1.0)));
  let c = textureSample(tex, texSampler, clamp(uv + vec2f(0.0, radius.y), vec2f(0.0), vec2f(1.0)));
  let d = textureSample(tex, texSampler, clamp(uv - vec2f(0.0, radius.y), vec2f(0.0), vec2f(1.0)));
  return center * 0.36 + (a + b + c + d) * 0.16;
}
`,NT={id:"additiveDissolve",name:"Additive Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"additiveDissolveFragment",uniformSize:16,shader:`
struct AdditiveDissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  _pad: f32,
};

${Bi}
@group(0) @binding(3) var<uniform> params: AdditiveDissolveParams;

@fragment
fn additiveDissolveFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);
  let left = textureSample(leftTex, texSampler, input.uv);
  let right = textureSample(rightTex, texSampler, input.uv);
  let base = left.rgb * (1.0 - p) + right.rgb * p;
  let flash = (left.rgb + right.rgb) * sin(p * PI) * 0.22;
  return vec4f(clamp(base + flash, vec3f(0.0), vec3f(1.0)), mix(left.a, right.a, p));
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},$T={id:"blurDissolve",name:"Blur Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"blurDissolveFragment",uniformSize:16,shader:`
struct BlurDissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  strength: f32,
};

${Bi}
@group(0) @binding(3) var<uniform> params: BlurDissolveParams;
${og}

@fragment
fn blurDissolveFragment(input: VertexOutput) -> @location(0) vec4f {
  let p = clamp(params.progress, 0.0, 1.0);
  let envelope = sin(p * PI);
  let radius = vec2f(1.0 / params.width, 1.0 / params.height) * params.strength * envelope;
  let left = sampleSoft(leftTex, input.uv, radius);
  let right = sampleSoft(rightTex, input.uv, radius);
  let t = 0.5 - 0.5 * cos(p * PI);
  return mix(left, right, t);
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.strength??9;return new Float32Array([e,t,r,o])}},HT={id:"dipToColorDissolve",name:"Dip To Color Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"dipToColorDissolveFragment",uniformSize:32,shader:`
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

${Bi}
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.color,a=Array.isArray(o)?o:[0,0,0];return new Float32Array([e,t,r,a[0]??0,a[1]??0,a[2]??0,0,0])}},jT={id:"nonAdditiveDissolve",name:"Non-Additive Dissolve",category:"dissolve",hasDirection:!1,entryPoint:"nonAdditiveDissolveFragment",uniformSize:16,shader:`
struct NonAdditiveDissolveParams {
  progress: f32,
  width: f32,
  height: f32,
  _pad: f32,
};

${Bi}
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
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},qT={id:"smoothCut",name:"Smooth Cut",category:"dissolve",hasDirection:!1,entryPoint:"smoothCutFragment",uniformSize:16,shader:`
struct SmoothCutParams {
  progress: f32,
  width: f32,
  height: f32,
  strength: f32,
};

${Bi}
@group(0) @binding(3) var<uniform> params: SmoothCutParams;
${og}

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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.strength??.9;return new Float32Array([e,t,r,o])}},KT={id:"sparkles",name:"Sparkles",category:"custom",hasDirection:!1,entryPoint:"sparklesFragment",uniformSize:32,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.sparkleScale??1,a=i?.intensity??1,s=i?.density??1,l=i?.glow??1;return new Float32Array([e,t,r,o,a,s,l,0])}},GT={id:"glitch",name:"Glitch",category:"custom",hasDirection:!1,entryPoint:"glitchFragment",uniformSize:32,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.intensity??1,a=i?.blockSize??30,s=i?.rgbSplit??1;return new Float32Array([e,t,r,o,a,s,0,0])}},WT={id:"pixelate",name:"Pixelate",category:"custom",hasDirection:!1,entryPoint:"pixelateFragment",uniformSize:32,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.maxBlockSize??48,a=1-Math.abs(e*2-1),s=a*a,l=Math.max(1,s*o),c=l/t,u=l/r,d=Math.max(0,Math.min(1,(e-.45)/.1)),f=d*d*(3-2*d);return new Float32Array([e,c,u,f,0,0,0,0])}},VT={id:"chromatic",name:"Chromatic",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"chromaticFragment",uniformSize:32,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.spread??1.5,a=i?.intensity??1;return new Float32Array([e,t,r,n,o,a,0,0])}},ZT={id:"radialBlur",name:"Radial Blur",category:"custom",hasDirection:!1,entryPoint:"radialBlurFragment",uniformSize:32,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.blurStrength??1,a=i?.spin??.3,s=i?.samples??12;return new Float32Array([e,t,r,o,a,s,0,0])}},YT={id:"fade",name:"Fade",category:"basic",hasDirection:!1,entryPoint:"fadeFragment",uniformSize:16,shader:`
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
}`,packUniforms:(e,t,r)=>new Float32Array([e,t,r,0])},XT={id:"wipe",name:"Wipe",category:"wipe",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"wipeFragment",uniformSize:16,shader:`
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
}`,packUniforms:(e,t,r,n)=>new Float32Array([e,t,r,n])},QT={id:"slide",name:"Slide",category:"slide",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"slideFragment",uniformSize:16,shader:`
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
}`,packUniforms:(e,t,r,n)=>new Float32Array([e,t,r,n])},JT={id:"flip",name:"Flip",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"flipFragment",uniformSize:16,shader:`
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
}`,packUniforms:(e,t,r,n)=>new Float32Array([e,t,r,n])},ag=`
fn scaleUv(uv: vec2f, scale: f32) -> vec2f {
  let safeScale = max(scale, 0.001);
  return ((uv - vec2f(0.5, 0.5)) / safeScale) + vec2f(0.5, 0.5);
}
`,eE={id:"clockWipe",name:"Clock Wipe",category:"mask",hasDirection:!1,entryPoint:"clockWipeFragment",uniformSize:16,shader:`
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

${ag}

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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.edgeSoftness??8;return new Float32Array([e,t,r,o])}},tE={id:"iris",name:"Iris",category:"iris",hasDirection:!1,entryPoint:"irisFragment",uniformSize:16,shader:`
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

${ag}

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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.edgeSoftness??6;return new Float32Array([e,t,r,o])}},rE={id:"liquidDistort",name:"Liquid Distort",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"liquidDistortFragment",uniformSize:48,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.intensity??1,a=i?.scale??4.5,s=i?.turbulence??1,l=i?.edgeSoftness??.18,c=i?.chroma??.75,u=i?.swirl??.8,d=i?.shine??1;return new Float32Array([e,t,r,n,o,a,s,l,c,u,d,0])}},nE={id:"lensWarpZoom",name:"Lens Warp Zoom",category:"custom",hasDirection:!1,entryPoint:"lensWarpZoomFragment",uniformSize:48,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.zoomStrength??1,a=i?.warpStrength??.75,s=i?.blurStrength??1,l=i?.chroma??.65,c=i?.vignette??.7,u=i?.centerX??.5,d=i?.centerY??.5,f=i?.glow??1;return new Float32Array([e,t,r,o,a,s,l,c,u,d,f,0])}},iE={id:"lightLeakBurn",name:"Light Leak Burn",category:"custom",hasDirection:!0,directions:["from-left","from-right","from-top","from-bottom"],entryPoint:"lightLeakBurnFragment",uniformSize:48,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.intensity??1.25,a=i?.spread??1,s=i?.warmth??.75,l=i?.burn??1.1,c=i?.edgeSoftness??.16,u=i?.grain??.5;return new Float32Array([e,t,r,n,o,a,s,l,c,u,0,0])}},oE={id:"filmGateSlip",name:"Film Gate Slip",category:"custom",hasDirection:!1,entryPoint:"filmGateSlipFragment",uniformSize:48,shader:`
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
}`,packUniforms:(e,t,r,n,i)=>{const o=i?.slip??1,a=i?.shake??1,s=i?.exposure??.85,l=i?.gateWidth??.075,c=i?.grain??.6,u=i?.chroma??.55,d=i?.roll??.75;return new Float32Array([e,t,r,o,a,s,l,c,u,d,0,0])}},aE=new Map;function Ee(e){aE.set(e.id,e)}Ee(UT);Ee(NT);Ee($T);Ee(HT);Ee(jT);Ee(qT);Ee(KT);Ee(GT);Ee(WT);Ee(VT);Ee(ZT);Ee(YT);Ee(XT);Ee(QT);Ee(JT);Ee(eE);Ee(tE);Ee(rE);Ee(nE);Ee(iE);Ee(oE);const sE={normal:0,dissolve:1,darken:2,multiply:3,"color-burn":4,"linear-burn":5,lighten:6,screen:7,"color-dodge":8,"linear-dodge":9,overlay:10,"soft-light":11,"hard-light":12,"vivid-light":13,"linear-light":14,"pin-light":15,"hard-mix":16,difference:17,exclusion:18,subtract:19,divide:20,hue:21,saturation:22,color:23,luminosity:24};function sg(e){return e}function lE(e){return 0}function cE(e){return e*e}function uE(e){return e*(2-e)}function dE(e){return e<.5?2*e*e:-1+(4-2*e)*e}function hc(e,t){const{x1:r,y1:n,x2:i,y2:o}=t;if(e===0)return 0;if(e===1)return 1;const a=3*r,s=3*(i-r)-a,l=1-a-s,c=3*n,u=3*(o-n)-c,d=1-c-u;let f=e;for(let p=0;p<8;p++){const h=((l*f+s)*f+a)*f-e;if(Math.abs(h)<1e-6)break;const g=(3*l*f+2*s)*f+a;if(Math.abs(g)<1e-6)break;f-=h/g,f=Math.max(0,Math.min(1,f))}return((d*f+u)*f+c)*f}function lg(e,t){const{tension:r,friction:n,mass:i}=t;if(e===0)return 0;if(e===1)return 1;const o=Math.sqrt(r/i),a=n/(2*Math.sqrt(r*i)),s=4/(a*o),l=e*s;let c;if(a<1){const u=o*Math.sqrt(1-a*a);c=1-Math.exp(-a*o*l)*(Math.cos(u*l)+a*o/u*Math.sin(u*l))}else if(a===1)c=1-Math.exp(-o*l)*(1+o*l);else{const u=-o*(a-Math.sqrt(a*a-1)),d=-o*(a+Math.sqrt(a*a-1));c=1-(d*Math.exp(u*l)-u*Math.exp(d*l))/(d-u)}return Math.max(0,Math.min(1.2,c))}const fE={linear:sg,"ease-in":cE,"ease-out":uE,"ease-in-out":dE,hold:lE,"cubic-bezier":e=>hc(e,xh),spring:e=>lg(e,Ih)};function cg(e){return fE[e]??sg}function ug(e,t){const r=Math.max(0,Math.min(1,e));return cg(t)(r)}function dg(e,t){const r=Math.max(0,Math.min(1,e));switch(t.type){case"cubic-bezier":return hc(r,t.bezier??xh);case"spring":return lg(r,t.spring??Ih);default:return cg(t.type)(r)}}const ke={fixedImages:{min:1,max:60},perMinutes:{min:1,max:60},motionIntensity:{min:.25,max:2},motionSpeed:{min:.25,max:3},imagePrompt:{max:2e3},sceneOpacity:{min:0,max:1},backgroundAppearance:{opacity:{min:0,max:1},blurRadius:{min:0,max:50},brightness:{min:-1,max:1},contrast:{min:0,max:3},saturation:{min:0,max:3},hueShift:{min:0,max:1},speed:{min:.25,max:3},zoom:{min:1,max:3}},backdropChannels:{min:1,max:20},kindsPerChannel:{min:1,max:3},pinnedPerChannel:{max:200},backdropClips:{min:1,max:40},backdropClipSec:{min:10,max:600}},pE=["clean","hook","circle-arrow","hook-arrow","split","closeup"],fg=y({style:U(["full","cutout","cutout-close"]),images:_a("mode",[y({mode:ne("fixed"),count:P().int().min(ke.fixedImages.min).max(ke.fixedImages.max)}).strict(),y({mode:ne("per-minutes"),minutes:P().int().min(ke.perMinutes.min).max(ke.perMinutes.max)}).strict(),y({mode:ne("none")}).strict()]),motion:y({kind:U(["ken-burns","wander"]),intensity:P().min(ke.motionIntensity.min).max(ke.motionIntensity.max),speed:P().min(ke.motionSpeed.min).max(ke.motionSpeed.max)}).strict(),thumbnail:U(pE).optional(),imagePrompt:I().min(1).max(ke.imagePrompt.max).refine(e=>e===e.trim(),"imagePrompt must be trimmed").optional(),opacity:P().finite().min(ke.sceneOpacity.min).max(ke.sceneOpacity.max).optional(),coverMediaId:I().min(1).optional(),compress:ne(!1).optional()}).strict(),pg=y({channels:F(y({channelId:I().regex(/^(twitch:[a-z0-9_]{1,25}|bilibili:\d{1,20})$/),kinds:F(U(["clip","highlight","vod","video"])).min(ke.kindsPerChannel.min).max(ke.kindsPerChannel.max),pinned:F(I().min(1).max(64)).max(ke.pinnedPerChannel.max)}).strict()).min(ke.backdropChannels.min).max(ke.backdropChannels.max),clipCount:P().int().min(ke.backdropClips.min).max(ke.backdropClips.max).optional(),clipSec:P().int().min(ke.backdropClipSec.min).max(ke.backdropClipSec.max)}).strict(),Re=ke.backgroundAppearance,mg=y({opacity:P().finite().min(Re.opacity.min).max(Re.opacity.max),blurRadius:P().finite().min(Re.blurRadius.min).max(Re.blurRadius.max),brightness:P().finite().min(Re.brightness.min).max(Re.brightness.max),contrast:P().finite().min(Re.contrast.min).max(Re.contrast.max),saturation:P().finite().min(Re.saturation.min).max(Re.saturation.max),hueShift:P().finite().min(Re.hueShift.min).max(Re.hueShift.max),speed:P().finite().min(Re.speed.min).max(Re.speed.max),zoom:P().finite().min(Re.zoom.min).max(Re.zoom.max),flipX:B(),flipY:B()}).strict(),Bo=50,yd=1440*60,wl=y({windowSeconds:P().finite().min(.1).max(10),bars:P().int().min(4).max(256),amplitude:P().finite().min(0).max(2),color:I().regex(/^#[a-fA-F0-9]{6}$/),strokeWidth:P().finite().min(.5).max(20),baseline:P().finite().min(0).max(1),visible:B()}).strict(),Cr=y({sourceHash:I().regex(/^sha256:[a-f0-9]{64}$/),samplesPerSecond:ne(Bo),durationSeconds:P().finite().positive().max(yd),envelopeBase64:I().max(Math.ceil(yd*Bo/3)*4).regex(/^[A-Za-z0-9+/]*={0,2}$/).refine(e=>e.length%4===0,"Base64 cần đủ nhóm bốn ký tự"),normalization:P().finite().nonnegative()}).strict().superRefine((e,t)=>{const r=e.envelopeBase64.endsWith("==")?2:e.envelopeBase64.endsWith("=")?1:0;e.envelopeBase64.length/4*3-r!==Math.ceil(e.durationSeconds*Bo-1e-8)&&t.addIssue({code:"custom",path:["envelopeBase64"],message:"Số mẫu sóng không khớp thời lượng nguồn"})}),mE=wl.extend({version:ne(1),sourceHash:Cr.shape.sourceHash,samplesPerSecond:ne(Bo),durationSeconds:Cr.shape.durationSeconds.optional(),envelopeBase64:Cr.shape.envelopeBase64.optional(),normalization:Cr.shape.normalization.optional()}).superRefine((e,t)=>{if(e.envelopeBase64!==void 0||e.durationSeconds!==void 0||e.normalization!==void 0){const r=Cr.safeParse({sourceHash:e.sourceHash,samplesPerSecond:e.samplesPerSecond,durationSeconds:e.durationSeconds,envelopeBase64:e.envelopeBase64,normalization:e.normalization});if(!r.success)for(const n of r.error.issues)t.addIssue({code:"custom",path:n.path,message:n.message})}}),vd={windowSeconds:1,bars:64,amplitude:1,color:"#ffffff",strokeWidth:3,baseline:.5,visible:!0};function hg(e,t){if(e.waveformVisualizer===void 0)return null;if(e.type!=="shape"||e.shapeType!=="rectangle"||typeof e.mediaId!="string"||!e.mediaId||e.isMask)return"Sóng giọng cần shape chữ nhật, nguồn giọng và không được làm mask";const r=mE.safeParse(e.waveformVisualizer);return r.success?t!==void 0&&r.data.sourceHash!==t&&r.data.envelopeBase64!==void 0?"Dữ liệu sóng giọng đã cũ; tạo lại từ nguồn hiện tại":null:`Dữ liệu sóng giọng không hợp lệ: ${r.error.issues[0]?.message}`}const hE=typeof performance<"u"&&typeof performance.mark=="function"&&typeof performance.measure=="function";let gE=0;function gg(e,t){if(!hE||!globalThis.__TL_PERF__)return t();const r=++gE,n=`${e}:s:${r}`,i=`${e}:e:${r}`;try{performance.mark(n)}catch{return t()}try{return t()}finally{try{performance.mark(i),performance.measure(e,n,i),performance.clearMarks(n),performance.clearMarks(i)}catch{}}}class yE{listeners=new Map;on(t,r){let n=this.listeners.get(t);return n||(n=new Set,this.listeners.set(t,n)),n.add(r),()=>{const i=this.listeners.get(t);i&&(i.delete(r),i.size===0&&this.listeners.delete(t))}}emit(t,r){const n=this.listeners.get(t);if(!(!n||n.size===0))for(const i of[...n])i(r)}clear(){this.listeners.clear()}}const vE=new yE;function bE(e,t){vE.emit(e,t)}const yg=1;function Pn(e,t){return Math.abs(e-t)<=yg}function da(e,t){return t<e-yg}function bd(e,t){if(e.originId!==t.originId||e.trackId!==t.trackId)return!1;const r=e.compositionId??e.mediaId,n=t.compositionId??t.mediaId;if(!r||r!==n||!Pn(e.from+e.durationInFrames,t.from)||(e.speed||1)!==(t.speed||1))return!1;const i=e.sourceEnd??(e.sourceStart??0)+e.durationInFrames*(e.speed||1),o=t.sourceStart??0;return!(Math.abs(i-o)>.5)}function vg(e,t,r,n){if(t===void 0)return 0;let i=n;return e+i<0&&(i=-e),r!==void 0&&t+i>r&&(i=r-t),i}function bg(e,t,r,n,i){if(n===0||!t||!r||!bd(t,e)||!bd(e,r)||!sr(e)||e.sourceEnd===void 0)return 0;const o=e.speed??1,a=e.sourceFps??i,s=e.sourceStart??0,l=e.sourceEnd,c=De(n,o,i,a);return vg(s,l,e.sourceDuration,c)}function gc(e,t,r){const n={id:e.id,from:e.from+t,durationInFrames:e.durationInFrames-t};if(e.type==="video"||e.type==="audio"||e.type==="composition"){const i=e.sourceStart??0,o=e.sourceEnd,a=e.speed??1,s=e.sourceFps??r,l=De(t,a,r,s);n.sourceStart=i+l,o!==void 0&&(n.sourceEnd=o)}return n}function yc(e,t,r){const n={id:e.id,durationInFrames:e.durationInFrames+t};if(e.type==="video"||e.type==="audio"||e.type==="composition"){const i=e.sourceStart??0,o=e.speed??1,a=e.sourceFps??r,s=De(n.durationInFrames??e.durationInFrames,o,r,a);n.sourceEnd=i+s}return n}function SE(e,t){return{id:e.id,from:e.from+t}}function Sg(e,t,r,n=30){const i=Math.floor(Math.min(e.durationInFrames,t.durationInFrames)-1);if(i<1)return 0;const o=fa(e,"end",n),a=fa(t,"start",n),s=d=>{const f=gn(d,r);return f.rightPortion<=o&&f.leftPortion<=a};let l=1,c=i,u=0;for(;l<=c;){const d=Math.floor((l+c)/2);s(d)?(u=d,l=d+1):c=d-1}return u}function Ui(e,t,r,n,i=30){if(e.trackId!==t.trackId)return{canAdd:!1,reason:"Clips must be on the same track"};const o=e.from+e.durationInFrames,a=Pn(o,t.from),s=da(o,t.from);if(!a&&!s)return{canAdd:!1,reason:"Clips must meet at the cut"};const l=["video","image","composition"];if(!l.includes(e.type)||!l.includes(t.type))return{canAdd:!1,reason:"Transitions only work with video and image clips"};const c=Math.min(e.durationInFrames,t.durationInFrames)-1;if(r>c)return{canAdd:!1,reason:`Transition too long. Max: ${c} frames (shorter clip duration - 1)`};const u=fa(e,"end",i),d=fa(t,"start",i);if(a){const f=gn(r,n),p=f.rightPortion,m=f.leftPortion;if(p>u||m>d)return{canAdd:!1,reason:`Insufficient handle for transition: ${[p>u?`left clip needs ${p} tail-handle frames but only has ${u}`:null,m>d?`right clip needs ${m} head-handle frames but only has ${d}`:null].filter(Boolean).join("; ")}`,leftHandle:u,rightHandle:d}}return{canAdd:!0,leftHandle:u,rightHandle:d}}function IE(e,t,r,n,i=30){if(t===0)return t;const o=n.filter(s=>s.leftClipId===e.id||s.rightClipId===e.id);return o.length===0?t:Ig(t,s=>{const l=xE(e,s);return o.every(c=>{const u=c.leftClipId===e.id?l:r.find(f=>f.id===c.leftClipId)??null,d=c.rightClipId===e.id?l:r.find(f=>f.id===c.rightClipId)??null;return!u||!d?!0:Ui(u,d,c.durationInFrames,c.alignment,i).canAdd})})}function Sd(e,t,r,n,i,o,a=30){if(t===0)return t;const s=new Set([e.id]);r&&s.add(r.id),n&&s.add(n.id);const l=o.filter(d=>s.has(d.leftClipId)||s.has(d.rightClipId));if(l.length===0)return t;const c=new Map(i.map(d=>[d.id,d]));return Ig(t,d=>{const f=new Map;r&&f.set(r.id,os(r,yc(r,d,a))),n&&f.set(n.id,os(n,gc(n,d,a)));let p=os(e,SE(e,d));const m=bg(e,r,n,d,a);return m!==0&&(p.type==="video"||p.type==="audio"||p.type==="composition")&&p.sourceEnd!==void 0&&(p={...p,sourceStart:(p.sourceStart??0)+m,sourceEnd:p.sourceEnd+m}),f.set(e.id,p),l.every(h=>{const g=f.get(h.leftClipId)??c.get(h.leftClipId)??null,v=f.get(h.rightClipId)??c.get(h.rightClipId)??null;return!g||!v?!0:Ui(g,v,h.durationInFrames,h.alignment,a).canAdd})})}function Ig(e,t){if(!t(0))return 0;if(t(e))return e;const r=e<0?-1:1;let n=0,i=Math.abs(e);for(;n<i;){const o=Math.ceil((n+i)/2),a=r*o;t(a)?n=o:i=o-1}return r*n}function fa(e,t,r){if(e.type==="text"||e.type==="shape"||e.type==="adjustment"||e.type==="image")return 1/0;if(e.type==="audio")return 0;const{sourceStart:n,sourceEnd:i,sourceDuration:o,sourceFps:a,speed:s}=Fi(e),l=a??r,c=i??o??0,u=o??0;if(t==="start")return Kt(n,s,l,r);{const d=Qf(u,c);return Kt(d,s,l,r)}}function xE(e,t){return{...e,sourceStart:(e.sourceStart??0)+t,sourceEnd:e.sourceEnd!==void 0?e.sourceEnd+t:e.sourceEnd}}function os(e,t){return{...e,...t}}const pa=new Set(["video","image","composition"]);function xg(e,t,r,n=new Set,i=30){const o=[],a=[],s=[],l=new Map;for(const d of t)l.set(d.id,d);const c=new Map;for(const d of t)pa.has(d.type)&&(c.has(d.trackId)||c.set(d.trackId,[]),c.get(d.trackId).push(d));for(const d of c.values())d.sort((f,p)=>f.from-p.from);const u=new Set(e);for(const d of r){const f=u.has(d.leftClipId),p=u.has(d.rightClipId);if(!f&&!p){o.push(d);continue}const m=wE(d,l,n,c,i);m.status==="valid"?o.push(d):m.status==="repaired"?a.push({original:d,repaired:m.repaired,action:m.action}):s.push(m.breakage)}return{valid:o,repaired:a,broken:s}}function wE(e,t,r,n,i){const o=t.get(e.leftClipId),a=t.get(e.rightClipId);if(!o||!a){const d=kE(e,o,a,t,r,n);if(d)return d;const f=[];return o||f.push(e.leftClipId),a||f.push(e.rightClipId),{status:"broken",breakage:Sr(e,"clip_deleted",`Transition removed: clip${f.length>1?"s":""} deleted`,f)}}if(o.trackId!==a.trackId)return{status:"broken",breakage:Sr(e,"cross_track","Transition removed: clips moved to different tracks",[e.leftClipId,e.rightClipId])};const s=o.from+o.durationInFrames,l=Pn(s,a.from),c=da(s,a.from);if(!l&&!c)return{status:"broken",breakage:Sr(e,"not_adjacent","Transition removed: clips no longer meet at the cut",[e.leftClipId,e.rightClipId])};if(!pa.has(o.type)||!pa.has(a.type))return{status:"broken",breakage:Sr(e,"invalid_type","Transition removed: clip type does not support transitions",[e.leftClipId,e.rightClipId])};if(l){const d=Sg(o,a,e.alignment,i);return d<=0?{status:"broken",breakage:Sr(e,"insufficient_handle","Transition removed: clips do not have enough handle for any duration",[e.leftClipId,e.rightClipId])}:e.durationInFrames>d?{status:"repaired",repaired:{...e,durationInFrames:d},action:`Duration adjusted from ${e.durationInFrames} to ${d} frames (handle limited)`}:{status:"valid"}}const u=s-a.from;if(u<e.durationInFrames){const d=Math.max(2,Math.floor(u));return d>=2&&d<o.durationInFrames&&d<a.durationInFrames?{status:"repaired",repaired:{...e,durationInFrames:d},action:`Duration adjusted from ${e.durationInFrames} to ${d} frames (legacy overlap shrunk)`}:{status:"broken",breakage:Sr(e,"invalid_duration","Transition removed: legacy overlap too small for any transition duration",[e.leftClipId,e.rightClipId])}}if(e.durationInFrames>=o.durationInFrames||e.durationInFrames>=a.durationInFrames){const d=Math.min(o.durationInFrames,a.durationInFrames)-1,f=Math.max(2,d);return f>=2?{status:"repaired",repaired:{...e,durationInFrames:f},action:`Duration adjusted from ${e.durationInFrames} to ${f} frames (clip too short)`}:{status:"broken",breakage:Sr(e,"invalid_duration","Transition removed: clips too short for any transition duration",[e.leftClipId,e.rightClipId])}}return{status:"valid"}}function kE(e,t,r,n,i,o){if(!t&&!r)return null;const a=t||r;if(!a)return null;const s=!t,l=o.get(a.trackId);if(!l)return null;const c=l.findIndex(f=>f.id===a.id);if(c===-1)return null;let u;if(s){if(u=l[c-1],u){const f=u.from+u.durationInFrames;!Pn(f,a.from)&&!da(f,a.from)&&(u=void 0)}}else if(u=l[c+1],u){const f=a.from+a.durationInFrames;!Pn(f,u.from)&&!da(f,u.from)&&(u=void 0)}return!u||!pa.has(u.type)?null:{status:"repaired",repaired:{...e,leftClipId:s?u.id:e.leftClipId,rightClipId:s?e.rightClipId:u.id},action:`Clip reassigned: ${s?"left":"right"} clip replaced with adjacent clip`}}function Sr(e,t,r,n){return{transitionId:e.id,transition:e,reason:t,message:r,affectedClipIds:n}}function PE(e,t,r){const n=t.frame-e.frame;if(n<=0)return e.value;const i=(r-e.frame)/n,o=e.easingConfig?dg(i,e.easingConfig):ug(i,e.easing);return e.value+(t.value-e.value)*o}function Ni(e,t,r){if(e.length===0)return r;const n=e[0];if(e.length===1||t<=n.frame)return n.value;const i=e[e.length-1];if(t>=i.frame)return i.value;for(let o=0;o<e.length-1;o++){const a=e[o],s=e[o+1];if(a.frame<=t&&s.frame>t)return PE(a,s,t)}return r}function La(e,t){return e?e.properties.find(n=>n.property===t)?.keyframes??[]:[]}const TE=30,EE=100/3;function CE(e){return Math.max(0,Math.min(100,e))/100}function Id(e,t,r){return r?dg(e,r):ug(e,t)}function ME(e,t){return Math.hypot(t.x-e.x,t.y-e.y)}function xd(e,t){return{influence:CE(e?.influence??EE),speed:Math.max(0,e?.speed??t)}}function _E(e,t,r,n){const i=e.temporalEase?.out,o=t.temporalEase?.in;if(!i&&!o)return Id(r,e.easing,e.easingConfig);const s=(t.frame-e.frame)/Math.max(1,n),l=ME(e.value,t.value);if(s<=0||l<=Number.EPSILON)return Id(r,e.easing,e.easingConfig);const c=l/s,u=xd(i,c),d=xd(o,c);return hc(r,{x1:u.influence,y1:u.influence*(u.speed/c),x2:1-d.influence,y2:1-d.influence*(d.speed/c)})}function FE(e,t,r){return{x:e.x+(t.x-e.x)*r,y:e.y+(t.y-e.y)*r}}function AE(e,t,r,n,i){const o=1-i,a=o*o*o,s=3*o*o*i,l=3*o*i*i,c=i*i*i;return{x:a*e.x+s*t.x+l*r.x+c*n.x,y:a*e.y+s*t.y+l*r.y+c*n.y}}function OE(e,t,r){return e==="position"&&!!(t.spatial||r.spatial)}function zE(e,t,r){const n=e.spatial?.outTangent??{x:0,y:0},i=t.spatial?.inTangent??{x:0,y:0};return AE(e.value,{x:e.value.x+n.x,y:e.value.y+n.y},{x:t.value.x+i.x,y:t.value.y+i.y},t.value,r)}function DE(e,t,r,n,i){const o=r.frame-t.frame;if(o<=0||t.easing==="hold")return t.value;const a=(n-t.frame)/o,s=_E(t,r,a,i);return OE(e,t,r)?zE(t,r,s):FE(t.value,r.value,s)}function Uo(e,t,r,n,i=TE){if(t.length===0)return n;const o=t[0];if(t.length===1||r<=o.frame)return o.value;const a=t[t.length-1];if(r>=a.frame)return a.value;for(let s=0;s<t.length-1;s++){const l=t[s],c=t[s+1];if(l.frame<=r&&c.frame>r)return DE(e,l,c,r,i)}return n}function as(e,t){return e?.vectorProperties?.find(r=>r.property===t)?.keyframes??[]}function wg(e,t){const r=e.findLast(i=>i.frame<t),n=e.find(i=>i.frame>t);return r?n?r.source?.applicationId===n.source?.applicationId?n.source:void 0:r.source:n?.source}function RE(e,t,r){const n=e.toSorted((l,c)=>l.frame-c.frame),i=n.filter(l=>l.frame>t);if(i.length===0)return{keyframes:e,removedCount:0,insertedBoundary:!1};const o=n.filter(l=>l.frame<=t);if(o.some(l=>l.frame===t))return{keyframes:o,removedCount:i.length,insertedBoundary:!1};const a=o.at(-1)??i[0],s={id:r(),frame:t,value:Ni(n,t,a.value),easing:a.easing,easingConfig:a.easingConfig,source:wg(n,t)};return{keyframes:[...o,s],removedCount:i.length,insertedBoundary:!0}}function LE(e,t,r,n){const i=e.keyframes.toSorted((c,u)=>c.frame-u.frame),o=i.filter(c=>c.frame>t);if(o.length===0)return{property:e,removedCount:0,insertedBoundary:!1};const a=i.filter(c=>c.frame<=t);if(a.some(c=>c.frame===t))return{property:{...e,keyframes:a},removedCount:o.length,insertedBoundary:!1};const s=a.at(-1)??o[0],l={id:n(),frame:t,value:Uo(e.property,i,t,s.value,r),easing:s.easing,easingConfig:s.easingConfig,source:wg(i,t)};return{property:{...e,keyframes:[...a,l]},removedCount:o.length,insertedBoundary:!0}}function BE(e,t,r,n=()=>Z()){const i=Math.max(0,t-1);let o=0,a=0;const s=e.properties.map(c=>{const u=RE(c.keyframes,i,n);return o+=u.removedCount,a+=u.insertedBoundary?1:0,u.removedCount>0?{...c,keyframes:u.keyframes}:c}),l=e.vectorProperties?.map(c=>{const u=LE(c,i,r,n);return o+=u.removedCount,a+=u.insertedBoundary?1:0,u.property});return o===0?{itemKeyframes:e,removedCount:0,insertedBoundaryCount:0}:{itemKeyframes:{...e,properties:s,vectorProperties:l},removedCount:o,insertedBoundaryCount:a}}const UE=[{value:"Roboto",label:"Roboto",family:"Roboto",weights:[100,200,300,400,500,600,700,800,900]},{value:"Open Sans",label:"Open Sans",family:"Open Sans",weights:[300,400,500,600,700,800]},{value:"Google Sans",label:"Google Sans",family:"Google Sans",weights:[400,500,600,700]},{value:"Noto Sans JP",label:"Noto Sans JP",family:"Noto Sans JP",weights:[100,200,300,400,500,600,700,800,900]},{value:"Inter",label:"Inter",family:"Inter",weights:[100,200,300,400,500,600,700,800,900]},{value:"Montserrat",label:"Montserrat",family:"Montserrat",weights:[100,200,300,400,500,600,700,800,900]},{value:"Poppins",label:"Poppins",family:"Poppins",weights:[100,200,300,400,500,600,700,800,900]},{value:"Lato",label:"Lato",family:"Lato",weights:[100,300,400,700,900]},{value:"Roboto Condensed",label:"Roboto Condensed",family:"Roboto Condensed",weights:[100,200,300,400,500,600,700,800,900]},{value:"Roboto Mono",label:"Roboto Mono",family:"Roboto Mono",weights:[100,200,300,400,500,600,700]},{value:"Arimo",label:"Arimo",family:"Arimo",weights:[400,500,600,700]},{value:"Oswald",label:"Oswald",family:"Oswald",weights:[200,300,400,500,600,700]},{value:"Noto Sans",label:"Noto Sans",family:"Noto Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Raleway",label:"Raleway",family:"Raleway",weights:[100,200,300,400,500,600,700,800,900]},{value:"Nunito",label:"Nunito",family:"Nunito",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Nunito Sans",label:"Nunito Sans",family:"Nunito Sans",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Playfair Display",label:"Playfair Display",family:"Playfair Display",weights:[400,500,600,700,800,900]},{value:"Rubik",label:"Rubik",family:"Rubik",weights:[300,400,500,600,700,800,900]},{value:"Ubuntu",label:"Ubuntu",family:"Ubuntu",weights:[300,400,500,700]},{value:"Roboto Slab",label:"Roboto Slab",family:"Roboto Slab",weights:[100,200,300,400,500,600,700,800,900]},{value:"DM Sans",label:"DM Sans",family:"DM Sans",weights:[100,200,300,400,500,600,700,800,900,1e3]},{value:"Merriweather",label:"Merriweather",family:"Merriweather",weights:[300,400,700,900]},{value:"Noto Sans KR",label:"Noto Sans KR",family:"Noto Sans KR",weights:[100,200,300,400,500,600,700,800,900]},{value:"Work Sans",label:"Work Sans",family:"Work Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"PT Sans",label:"PT Sans",family:"PT Sans",weights:[400,700]},{value:"Kanit",label:"Kanit",family:"Kanit",weights:[100,200,300,400,500,600,700,800,900]},{value:"Lora",label:"Lora",family:"Lora",weights:[400,500,600,700]},{value:"Mulish",label:"Mulish",family:"Mulish",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Fjalla One",label:"Fjalla One",family:"Fjalla One",weights:[400]},{value:"Manrope",label:"Manrope",family:"Manrope",weights:[200,300,400,500,600,700,800]},{value:"Archivo",label:"Archivo",family:"Archivo",weights:[100,200,300,400,500,600,700,800,900]},{value:"Quicksand",label:"Quicksand",family:"Quicksand",weights:[300,400,500,600,700]},{value:"Outfit",label:"Outfit",family:"Outfit",weights:[100,200,300,400,500,600,700,800,900]},{value:"Figtree",label:"Figtree",family:"Figtree",weights:[300,400,500,600,700,800,900]},{value:"Fira Sans",label:"Fira Sans",family:"Fira Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Inconsolata",label:"Inconsolata",family:"Inconsolata",weights:[200,300,400,500,600,700,800,900]},{value:"Barlow",label:"Barlow",family:"Barlow",weights:[100,200,300,400,500,600,700,800,900]},{value:"Bebas Neue",label:"Bebas Neue",family:"Bebas Neue",weights:[400]},{value:"Noto Sans TC",label:"Noto Sans TC",family:"Noto Sans TC",weights:[100,200,300,400,500,600,700,800,900]},{value:"Archivo Black",label:"Archivo Black",family:"Archivo Black",weights:[400]},{value:"Source Sans 3",label:"Source Sans 3",family:"Source Sans 3",weights:[200,300,400,500,600,700,800,900]},{value:"Hind Siliguri",label:"Hind Siliguri",family:"Hind Siliguri",weights:[300,400,500,600,700]},{value:"IBM Plex Sans",label:"IBM Plex Sans",family:"IBM Plex Sans",weights:[100,200,300,400,500,600,700]},{value:"Prompt",label:"Prompt",family:"Prompt",weights:[100,200,300,400,500,600,700,800,900]},{value:"Bungee",label:"Bungee",family:"Bungee",weights:[400]},{value:"Titillium Web",label:"Titillium Web",family:"Titillium Web",weights:[200,300,400,600,700,900]},{value:"Karla",label:"Karla",family:"Karla",weights:[200,300,400,500,600,700,800]},{value:"Heebo",label:"Heebo",family:"Heebo",weights:[100,200,300,400,500,600,700,800,900]},{value:"Saira",label:"Saira",family:"Saira",weights:[100,200,300,400,500,600,700,800,900]},{value:"Noto Serif",label:"Noto Serif",family:"Noto Serif",weights:[100,200,300,400,500,600,700,800,900]},{value:"PT Serif",label:"PT Serif",family:"PT Serif",weights:[400,700]},{value:"Jost",label:"Jost",family:"Jost",weights:[100,200,300,400,500,600,700,800,900]},{value:"Plus Jakarta Sans",label:"Plus Jakarta Sans",family:"Plus Jakarta Sans",weights:[200,300,400,500,600,700,800]},{value:"Bricolage Grotesque",label:"Bricolage Grotesque",family:"Bricolage Grotesque",weights:[200,300,400,500,600,700,800]},{value:"Noto Color Emoji",label:"Noto Color Emoji",family:"Noto Color Emoji",weights:[400]},{value:"Share Tech",label:"Share Tech",family:"Share Tech",weights:[400]},{value:"Smooch Sans",label:"Smooch Sans",family:"Smooch Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Source Code Pro",label:"Source Code Pro",family:"Source Code Pro",weights:[200,300,400,500,600,700,800,900]},{value:"Libre Baskerville",label:"Libre Baskerville",family:"Libre Baskerville",weights:[400,700]},{value:"Dancing Script",label:"Dancing Script",family:"Dancing Script",weights:[400,500,600,700]},{value:"Lobster Two",label:"Lobster Two",family:"Lobster Two",weights:[400,700]},{value:"EB Garamond",label:"EB Garamond",family:"EB Garamond",weights:[400,500,600,700,800]},{value:"Josefin Sans",label:"Josefin Sans",family:"Josefin Sans",weights:[100,200,300,400,500,600,700]},{value:"Cairo",label:"Cairo",family:"Cairo",weights:[200,300,400,500,600,700,800,900,1e3]},{value:"Libre Franklin",label:"Libre Franklin",family:"Libre Franklin",weights:[100,200,300,400,500,600,700,800,900]},{value:"Gravitas One",label:"Gravitas One",family:"Gravitas One",weights:[400]},{value:"Noto Serif JP",label:"Noto Serif JP",family:"Noto Serif JP",weights:[200,300,400,500,600,700,800,900]},{value:"Public Sans",label:"Public Sans",family:"Public Sans",weights:[100,200,300,400,500,600,700,800,900]},{value:"Anton",label:"Anton",family:"Anton",weights:[400]},{value:"Mukta",label:"Mukta",family:"Mukta",weights:[200,300,400,500,600,700,800]},{value:"Schibsted Grotesk",label:"Schibsted Grotesk",family:"Schibsted Grotesk",weights:[400,500,600,700,800,900]},{value:"Dosis",label:"Dosis",family:"Dosis",weights:[200,300,400,500,600,700,800]},{value:"Barlow Condensed",label:"Barlow Condensed",family:"Barlow Condensed",weights:[100,200,300,400,500,600,700,800,900]},{value:"Bitter",label:"Bitter",family:"Bitter",weights:[100,200,300,400,500,600,700,800,900]},{value:"Roboto Flex",label:"Roboto Flex",family:"Roboto Flex",weights:[100,200,300,400,500,600,700,800,900,1e3]},{value:"Cabin",label:"Cabin",family:"Cabin",weights:[400,500,600,700]},{value:"Noto Sans SC",label:"Noto Sans SC",family:"Noto Sans SC",weights:[100,200,300,400,500,600,700,800,900]},{value:"Ramabhadra",label:"Ramabhadra",family:"Ramabhadra",weights:[400]},{value:"Noto Sans Telugu",label:"Noto Sans Telugu",family:"Noto Sans Telugu",weights:[100,200,300,400,500,600,700,800,900]},{value:"Nanum Gothic",label:"Nanum Gothic",family:"Nanum Gothic",weights:[400,700,800]},{value:"Space Grotesk",label:"Space Grotesk",family:"Space Grotesk",weights:[300,400,500,600,700]},{value:"Changa One",label:"Changa One",family:"Changa One",weights:[400]},{value:"Anek Telugu",label:"Anek Telugu",family:"Anek Telugu",weights:[100,200,300,400,500,600,700,800]},{value:"Assistant",label:"Assistant",family:"Assistant",weights:[200,300,400,500,600,700,800]},{value:"Oxygen",label:"Oxygen",family:"Oxygen",weights:[300,400,700]},{value:"Pacifico",label:"Pacifico",family:"Pacifico",weights:[400]},{value:"Alfa Slab One",label:"Alfa Slab One",family:"Alfa Slab One",weights:[400]},{value:"Lexend",label:"Lexend",family:"Lexend",weights:[100,200,300,400,500,600,700,800,900]},{value:"Hind",label:"Hind",family:"Hind",weights:[300,400,500,600,700]},{value:"M PLUS Rounded 1c",label:"M PLUS Rounded 1c",family:"M PLUS Rounded 1c",weights:[100,300,400,500,700,800,900]},{value:"Red Hat Display",label:"Red Hat Display",family:"Red Hat Display",weights:[300,400,500,600,700,800,900]},{value:"Exo 2",label:"Exo 2",family:"Exo 2",weights:[100,200,300,400,500,600,700,800,900]},{value:"Slabo 27px",label:"Slabo 27px",family:"Slabo 27px",weights:[400]},{value:"Lobster",label:"Lobster",family:"Lobster",weights:[400]},{value:"Cormorant Garamond",label:"Cormorant Garamond",family:"Cormorant Garamond",weights:[300,400,500,600,700]},{value:"Sora",label:"Sora",family:"Sora",weights:[100,200,300,400,500,600,700,800]},{value:"Crimson Text",label:"Crimson Text",family:"Crimson Text",weights:[400,600,700]},{value:"Inter Tight",label:"Inter Tight",family:"Inter Tight",weights:[100,200,300,400,500,600,700,800,900]},{value:"Overpass",label:"Overpass",family:"Overpass",weights:[100,200,300,400,500,600,700,800,900]},{value:"Caveat",label:"Caveat",family:"Caveat",weights:[400,500,600,700]},{value:"Urbanist",label:"Urbanist",family:"Urbanist",weights:[100,200,300,400,500,600,700,800,900]},{value:"Comfortaa",label:"Comfortaa",family:"Comfortaa",weights:[300,400,500,600,700]},{value:"Tajawal",label:"Tajawal",family:"Tajawal",weights:[200,300,400,500,700,800,900]},{value:"Arvo",label:"Arvo",family:"Arvo",weights:[400,700]},{value:"PT Sans Narrow",label:"PT Sans Narrow",family:"PT Sans Narrow",weights:[400,700]},{value:"Rajdhani",label:"Rajdhani",family:"Rajdhani",weights:[300,400,500,600,700]},{value:"DM Serif Display",label:"DM Serif Display",family:"DM Serif Display",weights:[400]},{value:"Abel",label:"Abel",family:"Abel",weights:[400]},{value:"Teko",label:"Teko",family:"Teko",weights:[300,400,500,600,700]},{value:"Source Serif 4",label:"Source Serif 4",family:"Source Serif 4",weights:[200,300,400,500,600,700,800,900]},{value:"Noto Sans Arabic",label:"Noto Sans Arabic",family:"Noto Sans Arabic",weights:[100,200,300,400,500,600,700,800,900]},{value:"Almarai",label:"Almarai",family:"Almarai",weights:[300,400,700,800]},{value:"Merriweather Sans",label:"Merriweather Sans",family:"Merriweather Sans",weights:[300,400,500,600,700,800]},{value:"Lexend Deca",label:"Lexend Deca",family:"Lexend Deca",weights:[100,200,300,400,500,600,700,800,900]},{value:"Satisfy",label:"Satisfy",family:"Satisfy",weights:[400]},{value:"Orbitron",label:"Orbitron",family:"Orbitron",weights:[400,500,600,700,800,900]},{value:"Barlow Semi Condensed",label:"Barlow Semi Condensed",family:"Barlow Semi Condensed",weights:[100,200,300,400,500,600,700,800,900]},{value:"Domine",label:"Domine",family:"Domine",weights:[400,500,600,700]},{value:"Shadows Into Light",label:"Shadows Into Light",family:"Shadows Into Light",weights:[400]},{value:"M PLUS 1p",label:"M PLUS 1p",family:"M PLUS 1p",weights:[100,300,400,500,700,800,900]}],NE={normal:400,medium:500,semibold:600,bold:700},wd=new Map;function $E(e){const t=[...new Set(e)].filter(r=>Number.isFinite(r)&&r>0).sort((r,n)=>r-n);return t.length>0?t:[400]}function kd(e,t){return{family:e,weights:$E(t),display:"swap"}}function HE(e,t=e,r=[400]){if(typeof t=="string"){wd.set(e,kd(t,r));return}wd.set(e,kd(e,t))}function jE(e){for(const t of e)HE(t.value,t.family,t.weights)}jE(UE);function qE(e){if(Array.isArray(e.textSpans)&&e.textSpans.length>0){const t=e.textSpans.filter(r=>typeof r.text=="string").map(r=>({...r}));if(t.length>0)return t}return[{text:e.text??""}]}function KE(e){return(e.split(`
`)[0]?.trim()??"")||"Text"}const Ye={fontSize:60,fontFamily:"Inter",fontWeight:"normal",fontStyle:"normal",lineHeight:1.2,letterSpacing:0,textAlign:"center",verticalAlign:"middle",textPadding:16,color:"#ffffff",underline:!1};function GE(e,t,r,n){return`${e} ${t} ${r}px "${n}", sans-serif`}function kg(e){return{lineHeight:e.lineHeight??Ye.lineHeight,letterSpacing:e.letterSpacing??Ye.letterSpacing,textAlign:e.textAlign??Ye.textAlign,verticalAlign:e.verticalAlign??Ye.verticalAlign,textPadding:Math.max(0,e.textPadding??Ye.textPadding),color:e.color??Ye.color,backgroundColor:e.backgroundColor,backgroundRadius:Math.max(0,e.backgroundRadius??0),textShadow:e.textShadow,stroke:e.stroke}}function WE(e){const t=e.fontSize??Ye.fontSize,r=e.fontFamily??Ye.fontFamily,n=e.fontStyle??Ye.fontStyle,i=e.fontWeight??Ye.fontWeight,o=e.letterSpacing??Ye.letterSpacing,a=e.color??Ye.color,s=e.underline??Ye.underline;return qE(e).map(l=>{const c=l.fontSize??t,u=l.fontFamily??r,d=l.fontStyle??n,f=l.fontWeight??i,p=NE[f]??400;return{text:l.text??"",fontSize:c,fontFamily:u,fontStyle:d,fontWeightName:f,fontWeight:p,letterSpacing:l.letterSpacing??o,color:l.color??a,underline:l.underline??s,cssFont:GE(d,p,c,u)}})}function VE(e,t,r,n,i){const o=[];let a="";for(const s of e){const l=a+s;i.measure(l,t,r)>n&&a?(o.push(a),a=s):a=l}return a&&o.push(a),o}function ZE(e,t,r,n,i){const o=[];for(const a of e.split(`
`)){if(a===""){o.push("");continue}let s="";for(const l of a.split(" ")){const c=s?`${s} ${l}`:l;if(i.measure(c,t,r)>n&&s){if(o.push(s),s=l,i.measure(l,t,r)>n){const u=VE(l,t,r,n,i);for(let d=0;d<u.length-1;d++)o.push(u[d]??"");s=u[u.length-1]??""}}else s=c}s&&o.push(s)}return o.length>0?o:[""]}function YE(e){const t=[[]];let r=null,n="",i=[];const o=()=>{const l={text:"",charStyles:[],leading:n,leadingStyles:i};return n="",i=[],l},a=()=>{r&&t[t.length-1].push(r),r=null},s=()=>{n.length>0&&t[t.length-1].push(o())};for(let l=0;l<e.length;l++)for(const c of e[l].text)c===`
`?(a(),s(),t.push([])):c===" "?(a(),n+=c,i.push(l)):(r||(r=o()),r.text+=c,r.charStyles.push(l));return a(),s(),t}function XE(e,t,r,n){const i=[];let o=0;for(let a=1;a<=e.length;a++){if(a!==e.length&&t[a]===t[o])continue;const s=r[t[o]??0]??r[0],l=o===0?0:n(e.slice(0,o));i.push({text:e.slice(o,a),color:s.color,underline:s.underline,offsetX:l,width:n(e.slice(0,a))-l}),o=a}return i}function QE(e,t,r,n){const i=e[0],o=n.fontMetrics(i.cssFont),a=i.fontSize*t,l=(a-(o.ascent+o.descent))/2+o.ascent,c=YE(e),u=p=>n.measure(p,i.cssFont,i.letterSpacing),d=[],f=(p,m=!1)=>{let h="";const g=[];for(const[x,b]of p.entries())(x>0||!m)&&(h+=b.leading,g.push(...b.leadingStyles)),h+=b.text,g.push(...b.charStyles);const v=XE(h,g,e,u);d.push({text:h,cssFont:i.cssFont,fontSize:i.fontSize,color:i.color,letterSpacing:i.letterSpacing,underline:!1,width:u(h),top:0,baselineY:l,startX:0,lineHeightPx:a,runs:v})};for(const p of c){if(p.length===0){f([]);continue}let m=[],h="",g=!1;for(const v of p){const x=m.length>0||!g?v.leading:"",b=h+x+v.text;u(b)>r&&m.length>0?(f(m,g),g=!0,m=[v],h=v.text):(m.push(v),h=b)}m.length>0&&f(m,g)}return d}function JE(e,t,r,n){const i=kg(e),o=WE(e),a=i.textPadding,s=Math.max(1,t-a*2),l=r-a*2,c=e.spanLayout==="inline"&&o.length>0,u=c?QE(o,i.lineHeight,s,n):[];if(!c)for(const h of o){const g=n.fontMetrics(h.cssFont),v=h.fontSize*i.lineHeight,b=(v-(g.ascent+g.descent))/2+g.ascent;for(const w of ZE(h.text,h.cssFont,h.letterSpacing,s,n))u.push({text:w,cssFont:h.cssFont,fontSize:h.fontSize,color:h.color,letterSpacing:h.letterSpacing,underline:h.underline,width:n.measure(w,h.cssFont,h.letterSpacing),top:0,baselineY:b,startX:0,lineHeightPx:v})}const d=u.reduce((h,g)=>h+g.lineHeightPx,0),f=i.verticalAlign==="top"?a:i.verticalAlign==="bottom"?r-a-d:a+(l-d)/2;let p=f;for(const h of u)h.top=p,h.baselineY=p+h.baselineY,h.startX=i.textAlign==="left"?a:i.textAlign==="right"?t-a-h.width:(t-h.width)/2,p+=h.lineHeightPx;let m;if(i.backgroundColor&&u.length>0){const h=Math.max(...u.map(b=>b.width)),g=i.textAlign==="left"?a+h/2:i.textAlign==="right"?t-a-h/2:t/2,v=Math.min(t,h+a*2),x=d+a*2;m={x:g-v/2,y:f-a,width:v,height:x,radius:Math.max(0,Math.min(i.backgroundRadius,v/2,x/2))}}return{lines:u,totalHeight:d,background:m}}function kl(e){const t=/(\d+(?:\.\d+)?)px/.exec(e);return t?parseFloat(t[1]):16}function eC(e,t){"letterSpacing"in e&&(e.letterSpacing=`${t}px`)}function tC(e,t){return"fontKerning"in e&&(e.fontKerning="normal"),{measure(r,n,i){return e.font!==n&&(e.font=n),eC(e,i),t?t(r,i):e.measureText(r).width},fontMetrics(r){e.font!==r&&(e.font=r);const n=kl(r),i=e.measureText("Hg");return{ascent:i.fontBoundingBoxAscent||n*.8,descent:i.fontBoundingBoxDescent||n*.2}}}}let Ot;function rC(){return Ot!==void 0||typeof OffscreenCanvas<"u"&&(Ot=new OffscreenCanvas(1,1).getContext("2d"),Ot)?Ot:typeof document<"u"?(Ot=document.createElement("canvas").getContext("2d"),Ot):(Ot=null,Ot)}function nC(){const e=rC();return e?tC(e):{measure:(t,r,n)=>{const i=kl(r);return t.length*i*.6+t.length*n},fontMetrics:t=>{const r=kl(t);return{ascent:r*.8,descent:r*.2}}}}function iC(e,t){return e}function oC(e,t,r){const n=iC(e),i=kg(n),o=JE(n,t,0,nC()),a=(i.stroke?.width??0)*2,s=i.textShadow?Math.abs(i.textShadow.offsetY)+i.textShadow.blur:0;return o.totalHeight+i.textPadding*2+a+s*2}function aC(e,t,r){const n=oC(e,t.width);return n<=t.height+.5?t:{...t,height:n}}const Pd=.999;function uo(e){return!Number.isFinite(e)||e<=0?0:e>=1?1:e}function Pg(e){return Number.isFinite(e)?e<=-1?-1:e>=1?1:e:0}function Td(e,t){const r=e+t;if(r<=Pd)return[e,t];if(r<=0)return[0,0];const n=Pd/r;return[e*n,t*n]}function Ed(e,t,r){if(!Number.isFinite(r)||r<=0)return[0,0];const n=Math.max(0,Math.min(e,r)),i=Math.max(0,Math.min(t,r)),o=n+i;if(o<=r)return[n,i];if(o<=0)return[0,0];const a=r/o;return[n*a,i*a]}function sC(e,t){const r=Number.isFinite(e)&&e>0?e:0,n=Number.isFinite(t)&&t>0?t:0;return r===0||n===0?Math.max(r,n):Math.min(r,n)}function Tg(e){const[t,r]=Td(uo(e?.left??0),uo(e?.right??0)),[n,i]=Td(uo(e?.top??0),uo(e?.bottom??0)),o=Pg(e?.softness??0);return{left:t,right:r,top:n,bottom:i,softness:o}}function lC(e){if(!e)return;const t=Tg(e);if(!(t.left===0&&t.right===0&&t.top===0&&t.bottom===0))return t}function cC(e,t){return!Number.isFinite(t)||t<=0?0:Pg(e??0)*t}function Eg(e,t,r,n){if(!Number.isFinite(e)||e<=0||!Number.isFinite(t)||t<=0||!Number.isFinite(r)||r<=0||!Number.isFinite(n)||n<=0)return{x:0,y:0,width:Math.max(0,r),height:Math.max(0,n)};const i=Math.min(r/e,n/t),o=e*i,a=t*i;return{x:(r-o)/2,y:(n-a)/2,width:o,height:a}}function uC(e){return Number.isFinite(e)&&e>0}function dC(e,t,r,n,i){const o=Math.max(1e-6,1-i.left-i.right),a=Math.max(1e-6,1-i.top-i.bottom),s=e*o,l=t*a,c=Eg(s,l,r,n),u=c.width/s;return{x:c.x-e*i.left*u,y:c.y-t*i.top*u,width:e*u,height:t*u}}function fC(e,t,r,n,i,o,a){const s=[e,t,r,n].every(uC);return i?.refit===!0&&s?dC(e,t,r,n,o):a==="fill"?{x:0,y:0,width:Math.max(0,r),height:Math.max(0,n)}:Eg(e,t,r,n)}function pC(e,t,r,n,i,o="contain"){const a=Tg(i),s=fC(e,t,r,n,i,a,o),l={left:s.width*a.left,right:s.width*a.right,top:s.height*a.top,bottom:s.height*a.bottom},c={x:s.x+l.left,y:s.y+l.top,width:Math.max(0,s.width-l.left-l.right),height:Math.max(0,s.height-l.top-l.bottom)},u=cC(a.softness,sC(s.width,s.height)),d=Math.abs(u),f=u>0?{left:l.left>0?Math.min(d,l.left):0,right:l.right>0?Math.min(d,l.right):0,top:l.top>0?Math.min(d,l.top):0,bottom:l.bottom>0?Math.min(d,l.bottom):0}:{left:0,right:0,top:0,bottom:0},p=c.x-f.left,m=c.y-f.top,h=p+Math.max(0,c.width+f.left+f.right),g=m+Math.max(0,c.height+f.top+f.bottom),v={x:Math.floor(p),y:Math.floor(m),width:Math.ceil(h)-Math.floor(p),height:Math.ceil(g)-Math.floor(m)},x={left:l.left>0?u>0?f.left:d:0,right:l.right>0?u>0?f.right:d:0,top:l.top>0?u>0?f.top:d:0,bottom:l.bottom>0?u>0?f.bottom:d:0},[b,w]=Ed(x.left,x.right,v.width),[E,k]=Ed(x.top,x.bottom,v.height);return{mediaRect:s,cropViewportRect:c,viewportRect:v,cropPixels:l,crop:a,softnessPixels:d,featherPixels:{left:b,right:w,top:E,bottom:k}}}function mC(e){return e?e.topLeft[0]!==0||e.topLeft[1]!==0||e.topRight[0]!==0||e.topRight[1]!==0||e.bottomRight[0]!==0||e.bottomRight[1]!==0||e.bottomLeft[0]!==0||e.bottomLeft[1]!==0:!1}function hC(e,t,r){return r?.sourceWidth!==void 0&&r.sourceWidth>0&&r.sourceHeight!==void 0&&r.sourceHeight>0?pC(r.sourceWidth,r.sourceHeight,e,t,r.crop,r.fitMode).mediaRect:{x:0,y:0,width:e,height:t}}const gC={trimPathStart:0,trimPathEnd:100,trimPathOffset:0,taperStartWidth:100,taperEndWidth:100,taperStartLength:0,taperEndLength:0};function yC(e,t){return e[t]??gC[t]}const vC=2048,bC=512,SC=64;function Nt(e){return typeof e!="number"}function Cg(e,t){return Nt(e)?{x:t(e.x),y:t(e.y)}:t(e)}function Tn(e,t,r){if(Nt(e)&&Nt(t))return{x:r(e.x,t.x),y:r(e.y,t.y)};if(Nt(e)){const n=t;return{x:r(e.x,n),y:r(e.y,n)}}if(Nt(t)){const n=e;return{x:r(n,t.x),y:r(n,t.y)}}return r(e,t)}function IC(e){if((Nt(e)?[e.x,e.y]:[e]).some(r=>!Number.isFinite(r)))throw new Error("Expression produced a non-finite value");return e}function xC(e,t){let r=t,n=0;for(;r<e.length&&/[0-9.]/.test(e[r]);)e[r]==="."&&(n+=1),r+=1;const i=e.slice(t,r);if(n>1||i==="."||!Number.isFinite(Number(i)))throw new Error(`Invalid number at ${t+1}`);return{token:{type:"number",value:i,position:t},nextIndex:r}}function wC(e,t){let r=t+1;for(;r<e.length&&/[A-Za-z0-9_]/.test(e[r]);)r+=1;return{token:{type:"identifier",value:e.slice(t,r),position:t},nextIndex:r}}function kC(e,t){const r=e[t];let n=t+1,i="";for(;n<e.length&&e[n]!==r;){if(e[n]==="\\"&&(n+=1),n>=e.length)throw new Error(`Unterminated string at ${t+1}`);i+=e[n],n+=1}if(e[n]!==r)throw new Error(`Unterminated string at ${t+1}`);return{token:{type:"string",value:i,position:t},nextIndex:n+1}}function PC(e,t){const r=e[t];if(/[0-9.]/.test(r))return xC(e,t);if(/[A-Za-z_]/.test(r))return wC(e,t);if(r==='"'||r==="'")return kC(e,t);if("+-*/(),[]".includes(r))return{token:{type:"punctuation",value:r,position:t},nextIndex:t+1};throw new Error(`Unexpected character "${r}" at ${t+1}`)}function TC(e){if(e.length>vC)throw new Error("Expression is too long");const t=[];let r=0;for(;r<e.length;){if(/\s/.test(e[r])){r+=1;continue}const n=PC(e,r);if(t.push(n.token),r=n.nextIndex,t.length>bC)throw new Error("Expression has too many tokens")}return t.push({type:"eof",value:"",position:e.length}),t}function ss(e,t){if(t.length!==1)throw new Error(`${e} expects 1 argument`);const r={abs:Math.abs,sin:Math.sin,cos:Math.cos};return Cg(t[0],r[e])}function Cd(e,t){if(t.length<2)throw new Error(`${e} expects at least 2 arguments`);const r=e==="min"?Math.min:Math.max;return t.slice(1).reduce((n,i)=>Tn(n,i,r),t[0])}function EC(e){if(e.length!==3)throw new Error("clamp expects 3 arguments");return Tn(Tn(e[0],e[1],Math.max),e[2],Math.min)}function CC(e,t){if(e.length!==3)throw new Error("lerp expects 3 arguments");const r=t(e[2],"lerp amount");return Tn(e[0],e[1],(n,i)=>n+(i-n)*r)}const MC={abs:e=>ss("abs",e),sin:e=>ss("sin",e),cos:e=>ss("cos",e),min:e=>Cd("min",e),max:e=>Cd("max",e),clamp:EC,lerp:CC};function _C(e,t,r){const n=MC[e];if(n)return n(t,r);throw new Error(`Unknown function "${e}"`)}class FC{constructor(t,r){this.tokens=t,this.context=r}tokens;context;index=0;depth=0;parse(){const t=this.parseAdditive();if(this.current().type!=="eof")throw new Error(`Unexpected token at ${this.current().position+1}`);return IC(t)}current(){return this.tokens[this.index]}consume(t){const r=this.current();if(t!==void 0&&r.value!==t)throw new Error(`Expected "${t}" at ${r.position+1}`);return this.index+=1,r}withDepth(t){if(this.depth+=1,this.depth>SC)throw new Error("Expression is too deeply nested");try{return t()}finally{this.depth-=1}}parseAdditive(){let t=this.parseMultiplicative();for(;this.current().value==="+"||this.current().value==="-";){const r=this.consume().value,n=this.parseMultiplicative();t=Tn(t,n,r==="+"?(i,o)=>i+o:(i,o)=>i-o)}return t}parseMultiplicative(){let t=this.parseUnary();for(;this.current().value==="*"||this.current().value==="/";){const r=this.consume().value,n=this.parseUnary();if(r==="/"&&(Nt(n)?n.x===0||n.y===0:n===0))throw new Error("Division by zero");t=Tn(t,n,r==="*"?(i,o)=>i*o:(i,o)=>i/o)}return t}parseUnary(){return this.current().value==="+"?(this.consume("+"),this.parseUnary()):this.current().value==="-"?(this.consume("-"),Cg(this.parseUnary(),t=>-t)):this.parsePrimary()}parsePrimary(){return this.withDepth(()=>this.parsePrimaryValue())}parsePrimaryValue(){const t=this.current();if(t.type==="number")return Number(this.consume().value);if(t.value==="(")return this.parseGroupedValue();if(t.value==="[")return this.parseVectorValue();if(t.type==="identifier")return this.parseIdentifierValue();throw new Error(`Expected a value at ${t.position+1}`)}parseGroupedValue(){this.consume("(");const t=this.parseAdditive();return this.consume(")"),t}parseVectorValue(){this.consume("[");const t=this.requireScalar(this.parseAdditive(),"Vector X");this.consume(",");const r=this.requireScalar(this.parseAdditive(),"Vector Y");return this.consume("]"),{x:t,y:r}}parseIdentifierValue(){const t=this.consume().value;if(t==="value"||t==="preValue")return this.context.preValue;if(t==="frame")return this.context.globalFrame;if(t==="time")return this.context.globalFrame/Math.max(1,this.context.fps);if(this.current().value!=="(")throw new Error(`Unknown identifier "${t}"`);return this.parseCall(t)}parseCall(t){if(this.consume("("),t==="prop")return this.parsePropertyReference();const r=this.parseCallArguments();return this.consume(")"),_C(t,r,(n,i)=>this.requireScalar(n,i))}parseCallArguments(){const t=[];if(this.current().value===")")return t;for(;;){if(t.push(this.parseAdditive()),this.current().value!==",")return t;this.consume(",")}}parsePropertyReference(){const t=this.consumeString("Layer ID");this.consume(",");const r=this.consumeString("Property");if(this.consume(")"),!bh(r))throw new Error(`Unknown property "${r}"`);const n=this.context.resolveProperty(t,r);if(n===null)throw new Error("Property reference is unavailable");return n}consumeString(t){const r=this.current();if(r.type!=="string")throw new Error(`${t} must be a quoted string`);return this.consume(),r.value}requireScalar(t,r){if(Nt(t))throw new Error(`${r} must be a scalar`);return t}}function AC(e,t){return Ur(e)===Nt(t)}function OC(e,t){try{return{value:new FC(TC(e),t).parse()}}catch(r){return{value:t.preValue,error:r instanceof Error?r.message:"Expression failed"}}}const zC=["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius"];function Mg(e,t,r){let n;if(Oa(t))return Sc(Wi(e,r.canvas,On(e)),r.getKeyframes(e.id),r.globalFrame-e.from)[t];if(vh(t)&&e.type==="shape")n=yC(e,t);else return null;const i=La(r.getKeyframes(e.id),t),o=r.globalFrame-e.from;return Ni(i,o,n)}function vc(e,t,r,n,i={cache:new Map,vectorCache:new Map,expressionCache:new Map,active:new Set}){const o=`${e}:${t}`,a=`${o}@${n.globalFrame}`,s=i.cache.get(a);if(s!==void 0)return s;if(i.active.has(o))return r;const l=Et(n.getKeyframes(e)).find(m=>m.targetProperty===t&&m.enabled);if(!l)return i.cache.set(a,r),r;const c=n.getItem(l.sourceItemId);if(!c)return i.cache.set(a,r),r;i.active.add(o);const u=l.timeOffsetFrames===0?n:{...n,globalFrame:n.globalFrame-l.timeOffsetFrames,getPreviewTransform:void 0};if(Ur(l.sourceProperty))return i.active.delete(o),i.cache.set(a,r),r;const d=Mg(c,l.sourceProperty,u);if(d===null)return i.active.delete(o),i.cache.set(a,r),r;const f=vc(c.id,l.sourceProperty,d,u,i),p=DC(c.id,l.sourceProperty,f,u);return i.active.delete(o),i.cache.set(a,p),p}function _g(e,t,r){const n=Wi(e,r.canvas,On(e)),i=Sc(n,r.getKeyframes(e.id),r.globalFrame-e.from);return t==="position"?{x:i.x,y:i.y}:t==="anchor"?{x:i.anchorX,y:i.anchorY}:{x:n.width===0?100:i.width/n.width*100,y:n.height===0?100:i.height/n.height*100}}function DC(e,t,r,n){if(!Oa(t))return r;const i=n.getPreviewTransform?.(e)?.[t];return typeof i=="number"?i:r}function RC(e,t,r,n){const i=n.getPreviewTransform?.(e.id);return i?t==="position"?LC(i,r):t==="anchor"?BC(i,r):UC(e,i,r,n.canvas):r}function LC(e,t){return{x:e.x??t.x,y:e.y??t.y}}function BC(e,t){return{x:e.anchorX??t.x,y:e.anchorY??t.y}}function UC(e,t,r,n){const i=Wi(e,n,On(e));return{x:Md(t.width,r.x,i.width),y:Md(t.height,r.y,i.height)}}function Md(e,t,r){return e===void 0||r===0?t:e/r*100}function bc(e,t,r,n,i){const o=`${e}:${t}`,a=`${o}@${n.globalFrame}`,s=i.vectorCache.get(a);if(s)return s;if(i.active.has(o))return r;const l=Et(n.getKeyframes(e)).find(m=>m.targetProperty===t&&m.enabled);if(!l||!Ur(l.sourceProperty))return i.vectorCache.set(a,r),r;const c=n.getItem(l.sourceItemId);if(!c)return i.vectorCache.set(a,r),r;i.active.add(o);const u=l.timeOffsetFrames===0?n:{...n,globalFrame:n.globalFrame-l.timeOffsetFrames,getPreviewTransform:void 0},d=_g(c,l.sourceProperty,u),f=bc(c.id,l.sourceProperty,d,u,i),p=RC(c,l.sourceProperty,f,u);return i.active.delete(o),i.vectorCache.set(a,p),p}function NC(e,t,r,n){const i=r.getItem(e);if(!i)return null;if(Ur(t)){const s=_g(i,t,r),l=bc(i.id,t,s,r,n);return ma(i.id,t,l,r,n).value}const o=Mg(i,t,r);if(o===null)return null;const a=vc(i.id,t,o,r,n);return ma(i.id,t,a,r,n).value}function ma(e,t,r,n,i={cache:new Map,vectorCache:new Map,expressionCache:new Map,active:new Set}){const o=rn(n.getKeyframes(e)).find(u=>u.targetProperty===t&&u.enabled);if(!o)return{value:r};const a=`expression:${e}:${t}`,s=`${a}@${n.globalFrame}`,l=i.expressionCache.get(s);if(l!==void 0)return{value:l};if(i.active.has(a))return{value:r,error:"Expression dependency cycle"};i.active.add(a);const c=OC(o.source,{preValue:r,globalFrame:n.globalFrame,fps:n.canvas.fps,resolveProperty:(u,d)=>NC(u,d,n,i)});return i.active.delete(a),c.error||!AC(t,c.value)?{value:r,error:c.error??"Expression result has the wrong value type"}:(i.expressionCache.set(s,c.value),c)}function $C(e,t,r,n){const i={...e},o=as(t,"position"),a=as(t,"scale"),s=as(t,"anchor");if(o.length>0){const l=Uo("position",o,r,{x:e.x,y:e.y},n);i.x=l.x,i.y=l.y}if(a.length>0){const l=Uo("scale",a,r,{x:100,y:100},n);i.width=e.width*(l.x/100),i.height=e.height*(l.y/100)}if(s.length>0){const l=Uo("anchor",s,r,{x:e.anchorX,y:e.anchorY},n);i.anchorX=l.x,i.anchorY=l.y}return{result:i,hasPosition:o.length>0,hasScale:a.length>0,hasAnchor:s.length>0}}function HC(e,t){return t.hasPosition&&(e==="x"||e==="y")||t.hasScale&&(e==="width"||e==="height")?!0:t.hasAnchor&&(e==="anchorX"||e==="anchorY")}function Fg(e,t,r){return e==="position"?{x:r.result.x,y:r.result.y}:e==="anchor"?{x:r.result.anchorX,y:r.result.anchorY}:{x:t.width===0?100:r.result.width/t.width*100,y:t.height===0?100:r.result.height/t.height*100}}function Ag(e,t,r,n){if(e==="position"){n.result.x=t.x,n.result.y=t.y,n.hasPosition=!0;return}if(e==="anchor"){n.result.anchorX=t.x,n.result.anchorY=t.y,n.hasAnchor=!0;return}n.result.width=r.width*(t.x/100),n.result.height=r.height*(t.y/100),n.hasScale=!0}function jC(e){const{baseResolved:t,itemKeyframes:r,vectorAnimation:n,context:i,state:o}=e;for(const a of["position","scale","anchor"]){if(!Et(r).some(u=>u.targetProperty===a&&u.enabled))continue;const l=Fg(a,t,n),c=bc(r.itemId,a,l,i,o);Ag(a,c,t,n)}}function qC(e){const{baseResolved:t,itemKeyframes:r,vectorAnimation:n,context:i,state:o}=e;for(const a of["position","scale","anchor"]){if(!rn(r).some(u=>u.targetProperty===a&&u.enabled))continue;const l=Fg(a,t,n),c=ma(r.itemId,a,l,i,o).value;typeof c!="number"&&Ag(a,c,t,n)}}function KC(e){const{property:t,baseResolved:r,vectorAnimation:n,itemKeyframes:i,frame:o,expressionContext:a,expressionState:s}=e,l=La(i,t),c=HC(t,n),u=c?n.result[t]:r[t],d=c?u:Ni(l,o,u);if(a){const f=vc(i.itemId,t,d,a,s),p=ma(i.itemId,t,f,a,s).value;return typeof p=="number"?p:f}return l.length>0?d:n.result[t]}function Sc(e,t,r,n){if(!t)return e;const i=$C(e,t,r,n?.canvas.fps),o={cache:new Map,vectorCache:new Map,expressionCache:new Map,active:new Set};n&&(jC({baseResolved:e,itemKeyframes:t,vectorAnimation:i,context:n,state:o}),qC({baseResolved:e,itemKeyframes:t,vectorAnimation:i,context:n,state:o}));for(const a of zC)i.result[a]=KC({property:a,baseResolved:e,vectorAnimation:i,itemKeyframes:t,frame:r,expressionContext:n,expressionState:o});return i.result}function GC(e){return e?e.properties.some(t=>t.keyframes.length>0)||(e.vectorProperties?.some(t=>t.keyframes.length>0)??!1)||Et(e).some(t=>t.enabled)||(e.expressions?.some(t=>t.type==="expression"&&t.enabled)??!1):!1}const WC=["positionX","positionY","inX","inY","outX","outY"];function VC(e){return e?e.flatMap((t,r)=>WC.map(n=>L1(r,n))):[]}function ZC(e,t,r){const n=[];let i,o,a=0,s=0;for(const l of r){if(l.leftClipId===e){const c=gn(l.durationInFrames,l.alignment);o=l,s=c.leftPortion}if(l.rightClipId===e){const c=gn(l.durationInFrames,l.alignment);i=l,a=c.rightPortion}}if(i&&o){const l=Vf(t.durationInFrames,a,s);a=l.incomingPortion,s=l.outgoingPortion}return i&&a>0&&n.push({start:0,end:Math.min(a,t.durationInFrames),transition:i,role:"incoming"}),o&&s>0&&n.push({start:Math.max(0,t.durationInFrames-s),end:t.durationInFrames,transition:o,role:"outgoing"}),n}function Pi(e,t,r,n){const i=ZC(t,r,n);for(const o of i)if(e>=o.start&&e<o.end)return o}const YC=4294967296;function XC(e){const t=e.trim(),r=/^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.exec(t);if(!r)return null;const n=r[1];return n.length===3||n.length===4?`#${n.split("").map(i=>i+i).join("").toLowerCase()}`:`#${n.toLowerCase()}`}function QC(e){const t=XC(e);if(!t)return null;const r=t.slice(1);return r.length===6?Number.parseInt(r,16):YC+Number.parseInt(r,16)}const JC={"gpu-gaussian-blur":new Set(["samples"]),"gpu-motion-blur":new Set(["samples"]),"gpu-radial-blur":new Set(["samples"]),"gpu-zoom-blur":new Set(["samples"]),"gpu-grain":new Set(["speed"]),"gpu-glow":new Set(["rings","samplesPerRing"]),"gpu-scanlines":new Set(["speed"]),"gpu-color-glitch":new Set(["speed"]),"gpu-block-glitch":new Set(["speed"]),"gpu-pixel-sort":new Set(["length"])};function eM(e,t,r){switch(e){case"gpu-dither":return r==="angle"?t.mode==="linear":r==="scale"||r==="offsetX"||r==="offsetY"?t.mode==="radial":!0;case"gpu-ascii":return r==="textColor"?t.matchSourceColor!==!0:r==="colorSaturation"?t.matchSourceColor===!0:!0;default:return!0}}function tM(e,t){return typeof e.params[t]=="number"&&!JC[e.gpuEffectType]?.has(t)}function rM(e,t){const n=BT(e.gpuEffectType)?.params[t],i=e.params[t];return n?.type==="color"&&n.animatable===!0&&typeof i=="string"&&QC(i)!==null}function nM(e,t){return tM(e,t)||rM(e,t)}function iM(e){if(!e.effects||e.effects.length===0)return[];const t=[];for(const r of e.effects){if(r.effect.type!=="gpu-effect")continue;const n=r.effect;for(const[i,o]of Object.entries(n.params))typeof o!="number"&&typeof o!="string"||!nM(n,i)||eM(n.gpuEffectType,n.params,i)&&t.push(Sh(n.gpuEffectType,r.id,i))}return t}const Og={"clean-title":{id:"clean-title",label:"Clean",previewKind:"clean",layout:"single",sample:{title:"Main",subtitle:"Title"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title"},color:"#ffffff",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.95,letterSpacing:"tight",textPadding:"sm",textShadow:{token:"md",color:"#111827"},stroke:void 0}},poster:{id:"poster",label:"Poster",previewKind:"poster",layout:"single",sample:{title:"Tonight"},style:{fontFamily:"Anton",fontWeight:"normal",fontStyle:"normal",underline:!1,size:{token:"display",multiplier:.98},color:"#fef3c7",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.88,letterSpacing:"tight",textPadding:"sm",textShadow:{token:"xl",color:"#7f1d1d"},stroke:{width:2,color:"#991b1b"}}},"outline-pill":{id:"outline-pill",label:"Outline",previewKind:"outline-pill",layout:"single",sample:{title:"Featured"},style:{fontFamily:"Inter",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"badge",multiplier:.96},color:"#e2e8f0",backgroundColor:"#0f172a",backgroundRadius:"pill",textAlign:"center",verticalAlign:"middle",lineHeight:1,letterSpacing:"badge",textPadding:"badge",textShadow:{token:"sm",color:"#020617"},stroke:{width:1,color:"#38bdf8"}}},"lower-third":{id:"lower-third",label:"Lower Third",previewKind:"lower-third",layout:"two",sample:{title:"Name",subtitle:"Role or subtitle"},style:{fontFamily:"Inter",fontWeight:"semibold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.587},color:"#f9fafb",backgroundColor:"#111827",backgroundRadius:"md",textAlign:"left",verticalAlign:"middle",lineHeight:1.05,letterSpacing:"normal",textPadding:"lg",textShadow:{token:"sm",color:"#030712"},stroke:void 0}},"speaker-card":{id:"speaker-card",label:"Speaker",previewKind:"speaker",layout:"two",sample:{title:"Alex Morgan",subtitle:"Product Designer"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.61},color:"#f8fafc",backgroundColor:"#1e293b",backgroundRadius:"lg",textAlign:"left",verticalAlign:"middle",lineHeight:1,letterSpacing:"tight",textPadding:"xl",textShadow:{token:"md",color:"#020617"},stroke:void 0}},cinematic:{id:"cinematic",label:"Cinematic",previewKind:"cinematic",layout:"single",sample:{title:"CINEMA",subtitle:"PRESENTS"},style:{fontFamily:"Bebas Neue",fontWeight:"normal",fontStyle:"normal",underline:!1,size:{token:"display"},color:"#f8e6b8",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.92,letterSpacing:"cinematic",textPadding:"sm",textShadow:{token:"lg",color:"#111827"},stroke:{width:1,color:"#2b2112"}}},quote:{id:"quote",label:"Quote",previewKind:"quote",layout:"two",sample:{title:"Quote",subtitle:"Attribution"},style:{fontFamily:"Playfair Display",fontWeight:"semibold",fontStyle:"italic",underline:!1,size:{token:"title",multiplier:.83},color:"#f8fafc",backgroundColor:"#1f2937",backgroundRadius:"lg",textAlign:"center",verticalAlign:"middle",lineHeight:1.08,letterSpacing:"normal",textPadding:"quote",textShadow:{token:"lg",color:"#020617"},stroke:void 0}},neon:{id:"neon",label:"Neon",previewKind:"neon",layout:"single",sample:{title:"NEON",subtitle:"Glow"},style:{fontFamily:"Orbitron",fontWeight:"semibold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.935},color:"#67e8f9",backgroundColor:"#082f49",backgroundRadius:"sm",textAlign:"center",verticalAlign:"middle",lineHeight:1,letterSpacing:"wide",textPadding:"md",textShadow:{token:"glow",color:"#22d3ee"},stroke:{width:1,color:"#22d3ee"}}},"headline-stack":{id:"headline-stack",label:"Headline",previewKind:"stacked",layout:"three",sample:{eyebrow:"TOP STORY",title:"Headline",subtitle:"Subhead"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.913},color:"#f8fafc",backgroundColor:void 0,backgroundRadius:"none",textAlign:"center",verticalAlign:"middle",lineHeight:.96,letterSpacing:"tight",textPadding:"sm",textShadow:{token:"lg",color:"#020617"},stroke:void 0}},"breaking-update":{id:"breaking-update",label:"Breaking",previewKind:"breaking",layout:"three",sample:{eyebrow:"BREAKING",title:"Major Update",subtitle:"Developing now"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.92},color:"#f8fafc",backgroundColor:"#111827",backgroundRadius:"md",textAlign:"left",verticalAlign:"middle",lineHeight:.94,letterSpacing:"tight",textPadding:"lg",textShadow:{token:"lg",color:"#020617"},stroke:void 0}},"event-card":{id:"event-card",label:"Event",previewKind:"event",layout:"three",sample:{eyebrow:"LIVE",title:"Summer Fest",subtitle:"Friday 8 PM"},style:{fontFamily:"Inter Tight",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.848},color:"#f8fafc",backgroundColor:"#0f172a",backgroundRadius:"lg",textAlign:"center",verticalAlign:"middle",lineHeight:.98,letterSpacing:-.5,textPadding:"xl",textShadow:{token:"xl",color:"#020617"},stroke:void 0}},"launch-stack":{id:"launch-stack",label:"Launch",previewKind:"launch",layout:"three",sample:{eyebrow:"NOW LIVE",title:"New Collection",subtitle:"Shop the drop"},style:{fontFamily:"Space Grotesk",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"title",multiplier:.88},color:"#f8fafc",backgroundColor:"#0f172a",backgroundRadius:"lg",textAlign:"center",verticalAlign:"middle",lineHeight:.96,letterSpacing:"tight",textPadding:"xl",textShadow:{token:"glow",color:"#60a5fa"},stroke:{width:1,color:"#1d4ed8"}}},badge:{id:"badge",label:"Badge",previewKind:"badge",layout:"single",sample:{title:"NEW DROP",subtitle:"Tag"},style:{fontFamily:"Inter",fontWeight:"bold",fontStyle:"normal",underline:!1,size:{token:"badge"},color:"#f8fafc",backgroundColor:"#111827",backgroundRadius:"pill",textAlign:"center",verticalAlign:"middle",lineHeight:1,letterSpacing:"badge",textPadding:"badge",textShadow:{token:"sm",color:"#020617"},stroke:{width:1,color:"#334155"}}}};Object.values(Og).map(({id:e,label:t,previewKind:r,layout:n,sample:i})=>({id:e,label:t,previewKind:r,layout:n,sample:i}));function zg(e,t,r){return Math.min(r,Math.max(t,e))}function ls(e,t,r,n){return zg(Math.round(e.height*t),r,n)}function zt(e,t,r,n){const i=Math.min(e.width,e.height);return zg(Math.round(i*t),r,n)}function Ir(e){return e.map(t=>t.text).join(`
`)}function Dg(e){const t=Og[e];if(!t)throw new Error(`Unknown text style preset: ${e}`);return t}function oM(e,t){const r=e.sizes[t.token];return Math.round(r*(t.multiplier??1))}function aM(e,t){return typeof t=="number"?t:e.tracking[t]}function sM(e){return{sizes:{badge:ls(e,.048,32,68),title:ls(e,.085,56,132),display:ls(e,.11,72,164)},spacing:{sm:zt(e,.0148,16,16),badge:zt(e,.0167,18,18),md:zt(e,.0185,20,20),lg:zt(e,.0222,24,24),xl:zt(e,.0259,28,28),quote:zt(e,.0278,30,30)},radius:{none:0,sm:zt(e,.0167,18,18),md:zt(e,.0185,20,20),lg:zt(e,.0259,28,28),pill:999},tracking:{tight:-1,normal:0,wide:1,badge:2,cinematic:4},shadows:{sm:{offsetX:0,offsetY:4,blur:14},md:{offsetX:0,offsetY:6,blur:18},lg:{offsetX:0,offsetY:8,blur:24},xl:{offsetX:0,offsetY:10,blur:24},glow:{offsetX:0,offsetY:0,blur:22}}}}function lM(e,t,r=1){const n=Dg(e),i=sM(t),o=n.style.textShadow?i.shadows[n.style.textShadow.token]:void 0;return{fontFamily:n.style.fontFamily,fontWeight:n.style.fontWeight,fontStyle:n.style.fontStyle,underline:n.style.underline,fontSize:Math.round(oM(i,n.style.size)*r),color:n.style.color,backgroundColor:n.style.backgroundColor,backgroundRadius:n.style.backgroundRadius==="pill"?i.radius[n.style.backgroundRadius]:Math.round(i.radius[n.style.backgroundRadius]*r),textAlign:n.style.textAlign,verticalAlign:n.style.verticalAlign,lineHeight:n.style.lineHeight,letterSpacing:aM(i,n.style.letterSpacing)*r,textPadding:Math.round(i.spacing[n.style.textPadding]*r),textShadow:o?{offsetX:o.offsetX*r,offsetY:o.offsetY*r,blur:o.blur*r,color:n.style.textShadow?.color??"#000000"}:void 0,stroke:n.style.stroke?{...n.style.stroke,width:n.style.stroke.width*r}:void 0,textStylePresetId:e,textStyleScale:r}}function cM(e,t,r=1){const n=Dg(e),i=lM(e,t,r),o=i.fontSize??60;switch(e){case"speaker-card":{const a=[{text:n.sample.title,fontWeight:"bold"},{text:n.sample.subtitle??"Product Designer",fontSize:Math.max(20,Math.round(o*.44)),fontWeight:"medium",color:"#cbd5e1"}];return{...i,label:n.label,text:Ir(a),textSpans:a}}case"lower-third":{const a=[{text:n.sample.title,fontWeight:"bold"},{text:n.sample.subtitle??"Role or subtitle",fontSize:Math.max(22,Math.round(o*.54)),fontWeight:"medium",color:"#cbd5e1"}];return{...i,label:n.label,text:Ir(a),textSpans:a}}case"quote":{const a=[{text:n.sample.title,fontStyle:"italic"},{text:n.sample.subtitle??"Attribution",fontSize:Math.max(18,Math.round(o*.4)),fontStyle:"normal",fontWeight:"medium",color:"#cbd5e1",letterSpacing:1}];return{...i,label:n.label,text:Ir(a),textSpans:a}}case"breaking-update":{const a=[{text:n.sample.eyebrow??"BREAKING",fontSize:Math.max(16,Math.round(o*.28)),fontWeight:"bold",color:"#fca5a5",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Developing now",fontSize:Math.max(20,Math.round(o*.38)),fontWeight:"semibold",color:"#fde68a"}];return{...i,label:n.label,text:Ir(a),textSpans:a}}case"headline-stack":{const a=[{text:n.sample.eyebrow??"TOP STORY",fontSize:Math.max(16,Math.round(o*.3)),fontWeight:"semibold",color:"#fbbf24",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Subhead",fontSize:Math.max(20,Math.round(o*.42)),fontWeight:"medium",color:"#cbd5e1"}];return{...i,label:n.label,text:Ir(a),textSpans:a}}case"launch-stack":{const a=[{text:n.sample.eyebrow??"NOW LIVE",fontSize:Math.max(16,Math.round(o*.26)),fontWeight:"bold",color:"#67e8f9",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Shop the drop",fontSize:Math.max(20,Math.round(o*.4)),fontWeight:"medium",color:"#bfdbfe"}];return{...i,label:n.label,text:Ir(a),textSpans:a}}case"event-card":{const a=[{text:n.sample.eyebrow??"LIVE",fontSize:Math.max(18,Math.round(o*.28)),fontWeight:"bold",color:"#fca5a5",letterSpacing:2},{text:n.sample.title},{text:n.sample.subtitle??"Friday 8 PM",fontSize:Math.max(22,Math.round(o*.38)),fontWeight:"semibold",color:"#bfdbfe",letterSpacing:1}];return{...i,label:n.label,text:Ir(a),textSpans:a}}case"badge":return{...i,label:n.label,text:n.sample.title,textSpans:[{text:n.sample.title,letterSpacing:2}]};default:return{...i,label:n.label,text:n.sample.title,textSpans:void 0}}}function uM(e){if(!e)return{};const t={...e};return delete t.text,t}function dM(e,t,r,n=1){const i=cM(t,r,n),o=Array.isArray(e.textSpans)&&e.textSpans.length>0?e.textSpans:void 0,a=Array.isArray(i.textSpans)&&i.textSpans.length>0?i.textSpans:void 0,s=o?o.map((c,u)=>{const d=a?.[Math.min(u,Math.max(0,(a?.length??1)-1))];return{...c,...uM(d),text:c.text}}):void 0,l=s?s.map(c=>c.text).join(`
`):e.text;return{...i,text:l,textSpans:s,label:KE(l)}}const Rg=["textStyleScale","fontSize","lineHeight","textPadding","backgroundRadius","textShadowOffsetX","textShadowOffsetY","textShadowBlur","strokeWidth"];new Set(Rg);const fM="#000000",pM="#111827",mM={textStyleScale:e=>e.textStyleScale??1,fontSize:e=>e.fontSize??60,lineHeight:e=>e.lineHeight??1.2,textPadding:e=>e.textPadding??16,backgroundRadius:e=>e.backgroundRadius??0,textShadowOffsetX:e=>e.textShadow?.offsetX??0,textShadowOffsetY:e=>e.textShadow?.offsetY??0,textShadowBlur:e=>e.textShadow?.blur??0,strokeWidth:e=>e.stroke?.width??0};function hM(e,t){return mM[t](e)}function gM(e,t,r,n){return Ni(La(t,n),r,hM(e,n))}function yM(e,t){return La(e,t).length>0}function vM(e,t,r,n,i,o){const a=yM(t,n);if(!a)return{isAnimated:a,value:i};const s=gM(e,t,r,n);return{isAnimated:a,value:o===void 0?s:Math.max(o,s)}}function bM(e){if(!(e.offsetX===0&&e.offsetY===0&&e.blur===0))return e}function SM(e){if(!(e.width<=0))return e}function IM(e,t,r){return!e.textStylePresetId||!r.isAnimated?e:{...e,...dM(e,e.textStylePresetId,t,r.value??1)}}function xM(e,t,r){return e.textStylePresetId&&r.isAnimated?r.value:t.textStyleScale}function wM(e){return e.some(t=>t.isAnimated)}function cs(e,t){return e.value===void 0?t:e.value}function kM(e,t){return t.textShadow?.color?t.textShadow.color:e.textShadow?.color?e.textShadow.color:fM}function PM(e,t,r,n,i){if(!(!wM([r,n,i])&&!t.textShadow))return bM({offsetX:cs(r,0),offsetY:cs(n,0),blur:cs(i,0),color:kM(e,t)})}function TM(e,t,r){if(!(!r.isAnimated&&!t.stroke))return SM({width:r.value??0,color:t.stroke?.color??e.stroke?.color??pM})}function EM(e,t,r,n){const i=(h,g,v)=>vM(e,t,r,h,g,v),o=i("textStyleScale",e.textStyleScale),a=IM(e,n,o),s=i("fontSize",a.fontSize,1),l=i("lineHeight",a.lineHeight,.1),c=i("textPadding",a.textPadding,0),u=i("backgroundRadius",a.backgroundRadius,0),d=i("textShadowOffsetX",a.textShadow?.offsetX),f=i("textShadowOffsetY",a.textShadow?.offsetY),p=i("textShadowBlur",a.textShadow?.blur,0),m=i("strokeWidth",a.stroke?.width,0);return{...a,fontSize:s.value,lineHeight:l.value,textPadding:c.value,backgroundRadius:u.value,textStyleScale:xM(e,a,o),textShadow:PM(e,a,d,f,p),stroke:TM(e,a,m)}}const Ln=["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius"],_d=["cropLeft","cropRight","cropTop","cropBottom","cropSoftness"],us=["volume"],CM=["x","y","width","height","rotation"];function Lg(e){const t=iM(e);switch(e.type){case"audio":return[...us,...t];case"video":return[...Ln,..._d,...us,...t];case"composition":return[...Ln,..._d,...us,...t];case"text":return[...Ln,...Rg,...t];case"shape":return[...Ln,"trimPathStart","trimPathEnd","trimPathOffset","taperStartWidth","taperEndWidth","taperStartLength","taperEndLength",...e.shapeType==="path"?VC(e.pathVertices):[],...t];case"controller":return CM;default:return[...Ln,...t]}}const Xe=Math.PI*2,MM={"float-drift":["x","y","rotation"],"breath-pulse":["width","height","opacity"],"micro-shake":["x","y","rotation"],sway:["rotation"],spin:["rotation"],wander:["x","y","width","height"]};function hr(e,t,r){return Math.max(t,Math.min(r,e))}function $e(e,t){if(!MM[e.type].includes(t))return 0;const r=e.channelGains?.[t];return typeof r=="number"&&Number.isFinite(r)?hr(r,0,2):1}function pi(e){const t=Math.sin(e*12.9898)*43758.5453;return(t-Math.floor(t))*2-1}function _M(e){return e*e*(3-2*e)}function ds(e,t){const r=Math.floor(t),n=t-r,i=pi(e+r),o=pi(e+r+1);return i+(o-i)*_M(n)}const Fd={dx:0,dy:0,dRotation:0,dOpacity:0,scaleWidth:1,scaleHeight:1};function FM(e,t,r){const n=t.frame/Math.max(1,t.fps),i=e.phaseFrames/Math.max(1,t.fps)*e.frequency*Xe,o=hr(t.frameWidth*.008,4,18)*e.amplitude,a=hr(t.frameHeight*.014,6,28)*e.amplitude,s=1.2*e.amplitude;r.dx+=$e(e,"x")*o*Math.sin(Xe*e.frequency*n+i+Math.PI/2),r.dy+=$e(e,"y")*a*Math.sin(Xe*e.frequency*n+i),r.dRotation+=$e(e,"rotation")*s*Math.sin(Math.PI*e.frequency*n+i+Math.PI)}function AM(e,t,r){const n=t.frame/Math.max(1,t.fps),i=e.phaseFrames/Math.max(1,t.fps)*e.frequency*Xe,o=.035*e.amplitude,a=Math.min(.08,.04*e.amplitude),s=Math.sin(Xe*e.frequency*n+i);r.scaleWidth*=1+$e(e,"width")*o*s,r.scaleHeight*=1+$e(e,"height")*o*s,r.dOpacity+=$e(e,"opacity")*a*s}function OM(e,t,r){const i=t.frame/Math.max(1,t.fps)*e.frequency,o=hr(t.frameWidth*.004,2,10)*e.amplitude,a=hr(t.frameHeight*.004,2,10)*e.amplitude,s=.55*e.amplitude,l=e.seed*97;r.dx+=$e(e,"x")*ds(l+11,i)*o,r.dy+=$e(e,"y")*ds(l+23,i)*a,r.dRotation+=$e(e,"rotation")*ds(l+37,i)*s}function zM(e,t,r){const n=t.frame/Math.max(1,t.fps),i=e.phaseFrames/Math.max(1,t.fps)*e.frequency*Xe;r.dRotation+=$e(e,"rotation")*4*e.amplitude*Math.sin(Xe*e.frequency*n+i)}function DM(e,t,r){const n=t.frame/Math.max(1,t.fps);r.dRotation+=$e(e,"rotation")*360*e.frequency*e.amplitude*n}const Ad=[1,1.618,2.414],RM=[.55,.3,.15],Od=[.6,.97],LM=[.65,.35];function BM(e,t,r){const n=(t.frame+e.phaseFrames)/Math.max(1,t.fps),i=hr(t.frameWidth*.012,6,32)*e.amplitude,o=hr(t.frameHeight*.012,6,24)*e.amplitude,a=e.seed*97;let s=0,l=0;for(let d=0;d<Ad.length;d+=1){const f=Ad[d],p=RM[d];s+=p*Math.sin(Xe*e.frequency*f*n+Xe*pi(a+d)),l+=p*Math.sin(Xe*e.frequency*f*n+Xe*pi(a+16+d))}r.dx+=$e(e,"x")*i*s,r.dy+=$e(e,"y")*o*l;let c=0;for(let d=0;d<Od.length;d+=1)c+=LM[d]*Math.sin(Xe*e.frequency*Od[d]*n+Xe*pi(a+32+d));const u=.025*e.amplitude*c;r.scaleWidth*=1+$e(e,"width")*u,r.scaleHeight*=1+$e(e,"height")*u}function UM(e,t,r){if(!(!e.enabled||e.amplitude<=0))switch(e.type){case"float-drift":return FM(e,t,r);case"breath-pulse":return AM(e,t,r);case"micro-shake":return OM(e,t,r);case"sway":return zM(e,t,r);case"spin":return DM(e,t,r);case"wander":return BM(e,t,r);default:return e.type}}function NM(e,t){if(!e||e.length===0)return Fd;const r={...Fd};for(const n of e)UM(n,t,r);return r}function $M(e,t,r){if(!t||t.length===0)return e;const n=NM(t,r);return n.dx===0&&n.dy===0&&n.dRotation===0&&n.dOpacity===0&&n.scaleWidth===1&&n.scaleHeight===1?e:{...e,x:e.x+n.dx,y:e.y+n.dy,rotation:e.rotation+n.dRotation,width:Math.max(1,e.width*n.scaleWidth),height:Math.max(1,e.height*n.scaleHeight),opacity:hr(e.opacity+n.dOpacity,0,1)}}const zd={dx:0,dy:0,dRotation:0,dOpacity:0,scaleWidth:1,scaleHeight:1};function HM(e){return e==="multiply"?1:0}function jM(e,t={}){return{...e,id:t.freshIds?Z():e.id,tracks:e.tracks.map(r=>({...r,keyframes:r.keyframes.map(n=>({...n,id:t.freshIds?Z():n.id,...n.easingConfig&&{easingConfig:{...n.easingConfig,...n.easingConfig.bezier&&{bezier:{...n.easingConfig.bezier}},...n.easingConfig.spring&&{spring:{...n.easingConfig.spring}}}}}))}))}}function qM(e,t){if(!e||e.length===0)return zd;const r={...zd};for(const n of e)if(n.enabled)for(const i of n.tracks){if(i.keyframes.length===0)continue;const o=Ni(i.keyframes,t,HM(i.blend));switch(i.property){case"x":r.dx+=o;break;case"y":r.dy+=o;break;case"rotation":r.dRotation+=o;break;case"opacity":r.dOpacity+=o;break;case"width":r.scaleWidth*=o;break;case"height":r.scaleHeight*=o;break}}return r}function KM(e,t,r){return Math.max(t,Math.min(r,e))}function GM(e,t,r){if(!t||t.length===0)return e;const n=qM(t,r);return{...e,x:e.x+n.dx,y:e.y+n.dy,rotation:e.rotation+n.dRotation,width:Math.max(1,e.width*n.scaleWidth),height:Math.max(1,e.height*n.scaleHeight),opacity:KM(e.opacity+n.dOpacity,0,1)}}function WM(e,t){if(e.sourceItemType!==t.type)return{compatible:!1,reason:"type-mismatch"};const r=new Set(Lg(t));for(const n of e.properties)if(!aa(n.property)&&!r.has(n.property))return{compatible:!1,reason:"missing-property"};for(const n of e.vectorProperties??[])if(!(n.property==="position"?["x","y"]:n.property==="scale"?["width","height"]:["anchorX","anchorY"]).every(o=>r.has(o)))return{compatible:!1,reason:"missing-property"};return{compatible:!0}}function Me(){return He("TimelineActions")}function Ic(e,t,r){return gg(`tl.action.${e}`,()=>pc().executeAnonymous(e,t,r))}function pe(e,t){const r=S.getState().items,n=q.getState().transitions,{valid:i,repaired:o,broken:a}=gg("tl.repairTransitions",()=>xg(e,r,n,t,W.getState().fps)),s=o.map(l=>l.repaired);if(q.getState().setTransitions([...i,...s]),o.length>0)for(const l of o)Me().info(`[TransitionRepair] ${l.action}`);a.length>0&&bE("timeline.transitionBreakagesDetected",{breakages:[...a]})}function et(e,t){const n=S.getState().items.find(a=>a.id===e);if(!n)return!1;const i=q.getState().transitions;return Pi(t,e,n,i)===void 0}function Ba(){return{items:S.getState().items,tracks:S.getState().tracks,transitions:q.getState().transitions,keyframes:$.getState().keyframes}}function VM(e){const t=ye.getState(),r=(t.breadcrumbs[0]?.compositionId??null)===null,n={compositionId:null,items:[],tracks:[],transitions:[],keyframes:[]};if(r){if(t.activeCompositionId===null)return{compositionId:null,...e};const o=t.stashStack[0];return o?{compositionId:o.compositionId,items:o.items,tracks:o.tracks,transitions:o.transitions,keyframes:o.keyframes}:n}const i=t.mainHolder;return i?{compositionId:null,items:i.items,tracks:i.tracks,transitions:i.transitions,keyframes:i.keyframes}:n}function ZM(e){const{activeCompositionId:t}=ye.getState();return ae.getState().compositions.map(n=>n.id!==t?n:{...n,items:e.items,tracks:e.tracks,transitions:e.transitions,keyframes:e.keyframes})}function Bg(e,t){return e.type==="video"&&t.type==="audio"||e.type==="audio"&&t.type==="video"}function YM(e,t){return!Bg(e,t)||!e.originId||e.originId!==t.originId||!e.mediaId||e.mediaId!==t.mediaId?!1:e.from===t.from&&e.durationInFrames===t.durationInFrames}function $i(e,t){const r=e.find(i=>i.id===t);if(!r)return[];if(r.linkedGroupId)return e.filter(i=>i.linkedGroupId===r.linkedGroupId);const n=e.filter(i=>i.id===t||YM(r,i));return n.length>1?n:[r]}function Hi(e,t){return $i(e,t).map(r=>r.id)}function xc(e,t){const r=e.find(n=>n.id===t);return!r||r.type==="text"?[]:e.filter(n=>n.type==="text"&&(n.textRole==="caption"||n.captionSource!==void 0)&&n.captionSource?.clipId===r.id).map(n=>n.id)}function Ti(e,t){const r=new Set,n=new Set;for(const i of t){r.add(i);for(const o of xc(e,i))n.add(o)}for(const i of n)r.add(i);return Array.from(r)}function Ug(e,t){const r=[],n=new Set;for(const i of t){if(n.has(i))continue;const o=Hi(e,i);if(o.length!==0){r.push(i);for(const a of o)n.add(a)}}return r}function Ei(e,t){const r=$i(e,t),n=r.find(o=>o.id===t);if(!n)return[];const i=r.filter(o=>o.id===n.id||o.from===n.from&&o.durationInFrames===n.durationInFrames&&(o.sourceStart??null)===(n.sourceStart??null)&&(o.sourceEnd??null)===(n.sourceEnd??null)&&(o.speed??1)===(n.speed??1));return i.length>0?i:[n]}function XM(e,t,r,n){return Ei(e,t).find(i=>i.id!==t&&i.trackId===r&&i.type===n)??null}function QM(e,t,r){const n=Ei(e,t).filter(o=>o.id!==t),i=Ei(e,r).filter(o=>o.id!==r);for(const o of n){const a=i.find(s=>s.trackId===o.trackId&&s.type===o.type);if(a)return{leftCounterpart:o,rightCounterpart:a}}return null}function JM(e){if(e.length!==2)return!1;const[t,r]=e;return!(!t||!r||!Bg(t,r)||!t.mediaId||t.mediaId!==r.mediaId||t.from!==r.from||t.durationInFrames!==r.durationInFrames||(t.sourceStart??null)!==(r.sourceStart??null)||(t.sourceEnd??null)!==(r.sourceEnd??null))}function e_(e,t){const r=Array.from(new Set(t)).filter(a=>e.some(s=>s.id===a));if(r.length<2)return!1;const n=Ci(e,r);if(n.length<2)return!1;const[i]=n;if(!i)return!1;const o=new Set(Hi(e,i));return o.size!==n.length||n.some(a=>!o.has(a))}function Ci(e,t){const r=new Set;for(const n of t)for(const i of Hi(e,n))r.add(i);return Array.from(r)}function Ng(e,t){const r=e.find(n=>n.id===t);return r?[r]:[]}function Rr(e,t,r){return r?Ti(e,Ci(e,t)):Ti(e,Array.from(new Set(t)))}function ji(e,t,r){return r?$i(e,t):Ng(e,t)}function An(e,t,r){return r?Ei(e,t):Ng(e,t)}function $g(e,t,r,n){return n?QM(e,t,r):null}function Dd(e,t,r,n,i){return i?XM(e,t,r,n):null}function t_(e,t,r){if(!r){const o=new Map(t);for(const[a,s]of t)if(!(s<=0))for(const l of Ti(e,[a]))o.set(l,Math.max(o.get(l)??0,s));return e.flatMap(a=>{const s=o.get(a.id)??0;return s>0?[{id:a.id,from:a.from-s}]:[]})}const n=new Map(t),i=new Set;for(const o of e){if(i.has(o.id))continue;const a=$i(e,o.id);for(const l of a)i.add(l.id);if(a.length<=1)continue;let s=0;for(const l of a)s=Math.max(s,t.get(l.id)??0);if(!(s<=0))for(const l of a)n.set(l.id,s)}for(const[o,a]of n)if(!(a<=0))for(const s of Ti(e,[o]))n.set(s,Math.max(n.get(s)??0,a));return e.flatMap(o=>{const a=n.get(o.id)??0;return a>0?[{id:o.id,from:o.from-a}]:[]})}function ha(e){return e?!e.locked&&e.syncLock!==!1:!0}function r_(e){if(e.length===0)return;const t=new Map(e.map(n=>[n.originalId,n.result.rightItem.id])),r=q.getState().transitions.map(n=>{const i=t.get(n.leftClipId);return i?{...n,leftClipId:i}:(t.has(n.rightClipId),n)});q.getState().setTransitions(r)}function n_(e){const t=e.filter(o=>!!o.originalLinkedGroupId);if(t.length===0)return;const r=S.getState(),n=t.length>1?Z():void 0,i=t.length>1?Z():void 0;for(const o of t)r._updateItem(o.result.leftItem.id,{linkedGroupId:n}),r._updateItem(o.result.rightItem.id,{linkedGroupId:i})}function qi(e){e.length!==0&&(r_(e),n_(e))}function Mi(e){return Array.from(new Set(e))}function i_(e){const t=e.map(n=>({start:Math.max(0,Math.round(n.start)),end:Math.max(0,Math.round(n.end))})).filter(n=>n.end>n.start).sort((n,i)=>n.start-i.start);if(t.length===0)return[];const r=[t[0]];for(let n=1;n<t.length;n+=1){const i=t[n],o=r[r.length-1];if(i.start<=o.end){o.end=Math.max(o.end,i.end);continue}r.push({...i})}return r}function o_(e,t,r){const n=new Set;for(const i of t)!r.has(i.id)&&ha(i)&&n.add(i.id);for(const i of e){if(r.has(i.trackId)||n.has(i.trackId))continue;const o=t.find(a=>a.id===i.trackId);ha(o)&&n.add(i.trackId)}return[...n]}function Hg(e){const{items:t,tracks:r}=S.getState();return o_(t,r,e)}function Jn(e,t){const r=S.getState().itemById[e];if(!r)return null;const n=S.getState()._splitItem(e,t);return n?(qi([{originalId:r.id,originalLinkedGroupId:r.linkedGroupId,result:n}]),n):null}function a_(e,t){const r=S.getState(),n=[],i=[],o=S.getState().items.filter(a=>a.trackId===e&&a.from<t.end&&a.from+a.durationInFrames>t.start).sort((a,s)=>a.from-s.from);for(const a of o){const s=S.getState().itemById[a.id];if(!s||s.trackId!==e)continue;const l=s.from+s.durationInFrames,c=s.from<t.start,u=l>t.end;if(!c&&!u){r._removeItems([s.id]),i.push(s.id);continue}if(c&&u){const f=Jn(s.id,t.start);if(!f)continue;n.push(f.leftItem.id,f.rightItem.id);const p=Jn(f.rightItem.id,t.end);if(!p)continue;r._removeItems([p.leftItem.id]),i.push(p.leftItem.id),n.push(p.rightItem.id);continue}if(c){const f=Jn(s.id,t.start);if(!f)continue;r._removeItems([f.rightItem.id]),i.push(f.rightItem.id),n.push(f.leftItem.id);continue}const d=Jn(s.id,t.end);d&&(r._removeItems([d.leftItem.id]),i.push(d.leftItem.id),n.push(d.rightItem.id))}return{affectedIds:Mi(n),removedIds:Mi(i)}}function jg(e,t,r){if(r===0)return[];const n=S.getState(),i=S.getState().items.filter(o=>o.trackId===e&&t(o)).map(o=>({id:o.id,from:Math.max(0,o.from+r)}));return i.length>0&&n._moveItems(i),i.map(o=>o.id)}function Ua(e){const t=i_(e.intervals);if(t.length===0)return{affectedIds:[],removedIds:[]};const r=Hg(e.editedTrackIds),n=[],i=[];for(const o of r){let a=0;for(const s of t){const l={start:s.start-a,end:s.end-a},c=l.end-l.start;if(c<=0)continue;const u=a_(o,l);n.push(...u.affectedIds),i.push(...u.removedIds),n.push(...jg(o,d=>d.from>=l.end,-c)),a+=c}}return{affectedIds:Mi(n),removedIds:Mi(i)}}function s_(e){const t=Math.max(0,Math.round(e.cutFrame)),r=Math.max(0,Math.round(e.amount));if(r===0)return{affectedIds:[],removedIds:[]};const n=Hg(e.editedTrackIds),i=[];for(const o of n){const a=S.getState().items.filter(s=>s.trackId===o&&s.from<t&&s.from+s.durationInFrames>t).sort((s,l)=>s.from-l.from);for(const s of a){const l=S.getState().itemById[s.id];if(!l||l.trackId!==o)continue;const c=Jn(l.id,t);c&&i.push(c.leftItem.id,c.rightItem.id)}i.push(...jg(o,s=>s.from>=t,r))}return{affectedIds:Mi(i),removedIds:[]}}function l_(e,t,r){let n=Math.max(0,e);for(const i of r){const o=i.from+i.durationInFrames;if(!(o<=n)){if(i.from>=n+t)break;n=o}}return n}function Pl(e){const t=new Map,r=[];for(const i of S.getState().items){const o=t.get(i.trackId);o?o.push(i):t.set(i.trackId,[i])}for(const i of t.values())i.sort((o,a)=>o.from-a.from);const n=new Map;for(const i of e){const o=n.get(i.trackId);o?o.push(i):n.set(i.trackId,[i])}for(const i of n.values())i.sort((o,a)=>o.from-a.from);for(const i of n.values())for(const o of i){let a=t.get(o.trackId);a||(a=[],t.set(o.trackId,a));const s=l_(o.from,o.durationInFrames,a),l=s===o.from?o:{...o,from:s};r.push(l),a.push({trackId:l.trackId,from:l.from,durationInFrames:l.durationInFrames}),a.sort((c,u)=>c.from-u.from)}return r}function c_(e){return e.type==="video"||e.type==="composition"}function u_(e){return[...e].sort((t,r)=>(t.order??0)-(r.order??0))}function d_(e,t){return Math.max(0,t-e-1)}function qg(e){return e.type!=="audio"}function f_(e,t){const r=e.map(a=>{const s=t.get(a.id)??[],l=s.some(qg),c=s.some(u=>u.type==="audio");return l?"video":c?"audio":gr(a)}),n=r.findIndex(a=>a==="audio"),i=n===0?"audio":"video",o=new Map;return e.forEach((a,s)=>{const l=r[s]??(n!==-1&&s>=n?"audio":i);o.set(a.id,l)}),o}function fs(e){const{sourceTrack:t,kind:r,name:n,order:i,usedTrackIds:o,createId:a}=e,s=t&&!o.has(t.id)?t.id:`track-${a()}`;return o.add(s),{id:s,name:n,kind:r,height:t?.height??jr,locked:t?.locked??!1,syncLock:t?.syncLock??!0,visible:t?.visible??!0,muted:t?.muted??!1,solo:t?.solo??!1,volume:t?.volume??0,color:t?.color,order:i,items:[]}}function p_(e){const t=e.items.filter(o=>c_(o)).toSorted((o,a)=>{const s=e.trackOrderMap.get(o.trackId)??0,l=e.trackOrderMap.get(a.trackId)??0;return s!==l?s-l:o.from!==a.from?o.from-a.from:o.id.localeCompare(a.id)}),r=e.items.filter(o=>o.type==="audio"),n=new Set,i=[];for(const o of t){const a=e.videoTrackLaneIndex.get(o.trackId);if(a===void 0)continue;const s=ip(e.items,o);let l=s&&!n.has(s.id)?s:null;!l&&o.type==="video"&&(l=r.find(c=>!n.has(c.id)&&JM([o,c]))??null),l&&(n.add(l.id),i.push({video:o,audio:l,videoLaneIndex:a}))}return i}function Rd(e,t){const r=e?.properties.find(i=>i.property==="volume");if(!r||r.keyframes.length===0)return null;const n={property:"volume",keyframes:r.keyframes.map(i=>({...i}))};return{itemId:t,properties:[n]}}function Kg(e,t,r){return{id:r(),type:"audio",trackId:t,from:e.from,durationInFrames:e.durationInFrames,label:e.label,mediaId:e.mediaId,originId:e.originId,linkedGroupId:e.linkedGroupId,src:e.src,trimStart:e.trimStart,trimEnd:e.trimEnd,sourceStart:e.sourceStart,sourceEnd:e.sourceEnd,sourceDuration:e.sourceDuration,sourceFps:e.sourceFps,speed:e.speed,volume:e.volume,audioFadeIn:e.audioFadeIn,audioFadeOut:e.audioFadeOut,audioFadeInCurve:e.audioFadeInCurve,audioFadeOutCurve:e.audioFadeOutCurve,audioFadeInCurveX:e.audioFadeInCurveX,audioFadeOutCurveX:e.audioFadeOutCurveX,audioPitchSemitones:e.audioPitchSemitones,audioPitchCents:e.audioPitchCents,audioEqEnabled:e.audioEqEnabled,audioEqOutputGainDb:e.audioEqOutputGainDb,audioEqBand1Enabled:e.audioEqBand1Enabled,audioEqBand1Type:e.audioEqBand1Type,audioEqBand1FrequencyHz:e.audioEqBand1FrequencyHz,audioEqBand1GainDb:e.audioEqBand1GainDb,audioEqBand1Q:e.audioEqBand1Q,audioEqBand1SlopeDbPerOct:e.audioEqBand1SlopeDbPerOct,audioEqLowCutEnabled:e.audioEqLowCutEnabled,audioEqLowCutFrequencyHz:e.audioEqLowCutFrequencyHz,audioEqLowCutSlopeDbPerOct:e.audioEqLowCutSlopeDbPerOct,audioEqLowEnabled:e.audioEqLowEnabled,audioEqLowType:e.audioEqLowType,audioEqLowGainDb:e.audioEqLowGainDb,audioEqLowFrequencyHz:e.audioEqLowFrequencyHz,audioEqLowQ:e.audioEqLowQ,audioEqLowMidEnabled:e.audioEqLowMidEnabled,audioEqLowMidType:e.audioEqLowMidType,audioEqLowMidGainDb:e.audioEqLowMidGainDb,audioEqLowMidFrequencyHz:e.audioEqLowMidFrequencyHz,audioEqLowMidQ:e.audioEqLowMidQ,audioEqMidGainDb:e.audioEqMidGainDb,audioEqHighMidEnabled:e.audioEqHighMidEnabled,audioEqHighMidType:e.audioEqHighMidType,audioEqHighMidGainDb:e.audioEqHighMidGainDb,audioEqHighMidFrequencyHz:e.audioEqHighMidFrequencyHz,audioEqHighMidQ:e.audioEqHighMidQ,audioEqHighEnabled:e.audioEqHighEnabled,audioEqHighType:e.audioEqHighType,audioEqHighGainDb:e.audioEqHighGainDb,audioEqHighFrequencyHz:e.audioEqHighFrequencyHz,audioEqHighQ:e.audioEqHighQ,audioEqBand6Enabled:e.audioEqBand6Enabled,audioEqBand6Type:e.audioEqBand6Type,audioEqBand6FrequencyHz:e.audioEqBand6FrequencyHz,audioEqBand6GainDb:e.audioEqBand6GainDb,audioEqBand6Q:e.audioEqBand6Q,audioEqBand6SlopeDbPerOct:e.audioEqBand6SlopeDbPerOct,audioEqHighCutEnabled:e.audioEqHighCutEnabled,audioEqHighCutFrequencyHz:e.audioEqHighCutFrequencyHz,audioEqHighCutSlopeDbPerOct:e.audioEqHighCutSlopeDbPerOct,offset:e.offset}}function m_(e){const t=e.createId??Z,r=u_(e.tracks),n=new Map(r.map(z=>[z.id,z])),i=new Map;for(const z of e.items){const j=i.get(z.trackId);j?j.push(z):i.set(z.trackId,[z])}const o=f_(r,i),a=r.filter(z=>o.get(z.id)==="video").map(z=>z.id),s=r.filter(z=>o.get(z.id)==="audio").map(z=>z.id),l=new Map(a.map((z,j)=>[z,d_(j,a.length)])),c=[...a].reverse(),u=new Map(r.map(z=>[z.id,z.order??0])),d=p_({items:e.items,videoTrackLaneIndex:l,trackOrderMap:u}),f=new Map(d.map(z=>[z.video.id,z.audio])),p=new Map;for(const z of d)p.has(z.videoLaneIndex)||p.set(z.videoLaneIndex,z.audio.trackId);const m=new Set(d.map(z=>z.videoLaneIndex));for(const z of e.items){if(z.type!=="video"||!z.mediaId||e.videoHasAudioByMediaId[z.mediaId]!==!0)continue;const j=l.get(z.trackId);j!==void 0&&m.add(j)}const h=[...m].sort((z,j)=>z-j),g=h.length,v=s.filter(z=>(i.get(z)?.length??0)===0),x=new Set,b=[];for(const z of h){const j=p.get(z)??v.shift()??c[z]??null;j?(b.push(j),x.add(j)):b.push(c[z]??`generated-audio-source-${z}`)}const w=new Set(d.map(z=>z.audio.id)),E=e.items.filter(z=>z.type==="audio"&&!w.has(z.id)).toSorted((z,j)=>{const le=u.get(z.trackId)??0,ce=u.get(j.trackId)??0;return le!==ce?le-ce:z.from!==j.from?z.from-j.from:z.id.localeCompare(j.id)}).map(z=>z.trackId).filter((z,j,le)=>le.indexOf(z)===j),k=s.filter(z=>!x.has(z)),T=[...E,...k.filter(z=>!E.includes(z))],O=new Set,C=[],R=new Map,M=new Map,D=new Map;a.forEach((z,j)=>{const le=l.get(z)??0,ce=fs({sourceTrack:n.get(z)??null,kind:"video",name:`V${le+1}`,order:j,usedTrackIds:O,createId:t});C.push(ce),R.set(le,ce.id)}),b.forEach((z,j)=>{const le=h[j]??j,ce=fs({sourceTrack:n.get(z)??null,kind:"audio",name:`A${j+1}`,order:a.length+j,usedTrackIds:O,createId:t});C.push(ce),M.set(le,ce.id)}),T.forEach((z,j)=>{const le=g+j+1,ce=fs({sourceTrack:n.get(z)??null,kind:"audio",name:`A${le}`,order:a.length+g+j,usedTrackIds:O,createId:t});C.push(ce),D.set(z,ce.id)});const L=new Map(e.keyframes.map(z=>[z.itemId,z])),H=[...e.keyframes.map(z=>({...z,properties:z.properties.map(j=>({...j,keyframes:j.keyframes.map(le=>({...le}))}))}))],J=[],fe=[],se=new Map,At=T[0]?D.get(T[0]):M.get(0);for(const z of e.items){if(z.type==="audio"){const j=d.find(ce=>ce.audio.id===z.id);if(j){const ce=M.get(j.videoLaneIndex)??z.trackId,Fe=se.get(j.video.id)??j.video.linkedGroupId??j.audio.linkedGroupId??t();se.set(j.video.id,Fe);const qe=j.audio.trackId===ce&&j.audio.linkedGroupId===Fe?j.audio:{...j.audio,trackId:ce,linkedGroupId:Fe};if(!L.get(z.id)?.properties.some(ct=>ct.property==="volume")){const ct=Rd(L.get(j.video.id),z.id);ct&&H.push(ct)}J.push(qe);continue}const le=D.get(z.trackId)??At;J.push(le&&z.trackId!==le?{...z,trackId:le}:z);continue}if(z.type==="video"){const j=l.get(z.trackId)??0,le=R.get(j)??z.trackId,ce=f.get(z.id);let Fe=le!==z.trackId?{...z,trackId:le}:z;if(ce){const qe=se.get(z.id)??Fe.linkedGroupId??ce.linkedGroupId??t();Fe.linkedGroupId!==qe&&(Fe={...Fe,linkedGroupId:qe}),se.set(z.id,qe)}if(!ce&&z.mediaId&&e.videoHasAudioByMediaId[z.mediaId]===!0){const qe=M.get(j);if(qe){const vr=Fe.linkedGroupId??t();Fe.linkedGroupId!==vr&&(Fe={...Fe,linkedGroupId:vr});const ct=Kg(Fe,qe,t);fe.push(ct);const zn=Rd(L.get(z.id),ct.id);zn&&H.push(zn)}}J.push(Fe);continue}if(qg(z)){const j=l.get(z.trackId)??0,le=R.get(j)??z.trackId;J.push(le!==z.trackId?{...z,trackId:le}:z);continue}J.push(z)}J.push(...fe);const je=JSON.stringify({tracks:e.tracks,items:e.items,keyframes:e.keyframes})!==JSON.stringify({tracks:C,items:J,keyframes:H});return{tracks:C,items:J,keyframes:H,changed:je}}function h_(e,t,r,n){return e<r+n&&r<e+t}function g_(e,t){const r=e.reduce((n,i)=>Math.max(n,i.order??0),-1);return{id:`track-${t()}`,name:Bl(e,"audio"),kind:"audio",height:jr,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,volume:0,order:r+1,items:[]}}function y_(e){const t=e.createId??(()=>Z()),r=e.video.linkedGroupId??t(),n=e.video.linkedGroupId===r?e.video:{...e.video,linkedGroupId:r},{from:i,durationInFrames:o}=n;let a=null;const s=e.tracks.filter(u=>!u.isGroup&&gr(u)==="audio").toSorted((u,d)=>(u.order??0)-(d.order??0));for(const u of s)if(!(e.itemsByTrackId.get(u.id)??[]).some(p=>h_(i,o,p.from,p.durationInFrames))){a=u.id;break}let l=null;a||(l=g_(e.tracks,t),a=l.id);const c=Kg(n,a,t);return{updatedVideo:n,audioItem:c,newTrack:l}}function v_(e){const{trackId:t,from:r,durationInFrames:n,canvasWidth:i,canvasHeight:o}=e,a=Math.max(48,Math.min(i,o)*.08);return{id:Z(),type:"controller",controllerKind:"null",trackId:t,from:r,durationInFrames:n,label:"Null Object",transform:{x:0,y:0,width:a,height:a,rotation:0,opacity:1,aspectRatioLocked:!0}}}function b_(e){const t=new Map,r=new Map,n=new Map;for(const i of e)n.set(i.id,i),r.has(i.trackId)||r.set(i.trackId,[]),r.get(i.trackId).push(i),t.has(i.leftClipId)||t.set(i.leftClipId,{}),t.get(i.leftClipId).outgoing=i,t.has(i.rightClipId)||t.set(i.rightClipId,{}),t.get(i.rightClipId).incoming=i;return{transitionsByClipId:t,transitionsByTrackId:r,transitionsById:n}}function S_(e,t){const r=new Set(e),n=new Set,i=[];for(const o of e){if(n.has(o))continue;let a=o;const s=new Set;for(;;){const u=t.get(a);if(u?.incoming&&r.has(u.incoming.leftClipId)&&!n.has(u.incoming.leftClipId)&&!s.has(u.incoming.leftClipId))s.add(a),a=u.incoming.leftClipId;else break}const l=[];let c=a;for(;c&&r.has(c)&&!n.has(c);)n.add(c),l.push(c),c=t.get(c)?.outgoing?.rightClipId;l.length>0&&i.push(l)}return i}function I_(e){if(e<=0)return{cols:0,rows:0};const t=Math.ceil(Math.sqrt(e)),r=Math.ceil(e/t);return{cols:t,rows:r}}function fn(e,t,r,n){const i=Math.min(r/e,n/t);return{width:Math.round(e*i),height:Math.round(t*i)}}function x_(e,t,r,n){const i=new Map;if(e.length===0)return null;const o=n.gap??0,a=n.padding??0;return{result:i,gap:o,padding:a,availableWidth:t-a*2,availableHeight:r-a*2,canvasCenterX:t/2,canvasCenterY:r/2}}function w_(e,t,r){const{result:n,gap:i,padding:o,availableWidth:a,availableHeight:s,canvasCenterX:l,canvasCenterY:c}=r;let u,d;switch(t.preset){case"row":u=e.length,d=1;break;case"column":u=1,d=e.length;break;case"grid":u=t.cols??2,d=t.rows??2;break;default:{const m=I_(e.length);u=m.cols,d=m.rows;break}}const f=(a-i*(u-1))/u,p=(s-i*(d-1))/d;for(let m=0;m<e.length;m++){const h=e[m],g=m%u,v=Math.floor(m/u),x=o+g*(f+i),b=o+v*(p+i),w=fn(h.sourceWidth,h.sourceHeight,f,p),E=x+f/2,k=b+p/2;n.set(h.id,{x:E-l,y:k-c,width:w.width,height:w.height,rotation:0})}return n}function k_(e,t){const{result:r,gap:n,padding:i,availableWidth:o,availableHeight:a,canvasCenterX:s,canvasCenterY:l}=t,c=e[0],u=fn(c.sourceWidth,c.sourceHeight,o,a);if(r.set(c.id,{x:0,y:0,width:u.width,height:u.height,rotation:0}),e.length<2)return r;const d=o/4,f=e.slice(1),p=f.reduce((x,b)=>{const w=fn(b.sourceWidth,b.sourceHeight,d,d);return x+w.height},0)+n*(f.length-1),m=i+o;let h=i+a;const g=a,v=p>g?g/p:1;for(let x=f.length-1;x>=0;x--){const b=f[x],w=d*v,E=fn(b.sourceWidth,b.sourceHeight,w,w),k=m-w/2-n,T=h-E.height/2-n;r.set(b.id,{x:k-s,y:T-l,width:E.width,height:E.height,rotation:0}),h-=E.height+n*v}return r}function P_(e,t){const{result:r,gap:n,padding:i,availableWidth:o,availableHeight:a,canvasCenterX:s,canvasCenterY:l}=t,c=(o-n)*2/3,u=a,d=e[0],f=fn(d.sourceWidth,d.sourceHeight,c,u),p=i+c/2,m=i+u/2;if(r.set(d.id,{x:p-s,y:m-l,width:f.width,height:f.height,rotation:0}),e.length<2)return r;const h=(o-n)/3,g=i+c+n,v=e.slice(1),x=(a-n*(v.length-1))/v.length;for(let b=0;b<v.length;b++){const w=v[b],E=fn(w.sourceWidth,w.sourceHeight,h,x),k=i+b*(x+n),T=g+h/2,O=k+x/2;r.set(w.id,{x:T-s,y:O-l,width:E.width,height:E.height,rotation:0})}return r}function T_(e,t,r,n){const i=x_(e,t,r,n);if(!i)return new Map;switch(n.preset){case"pip":return k_(e,i);case"focus-sidebar":return P_(e,i);case"auto":case"row":case"column":case"grid":return w_(e,n,i)}}const ei=1e-6;function ga(e,t){return{a:e.a*t.a+e.c*t.b,b:e.b*t.a+e.d*t.b,c:e.a*t.c+e.c*t.d,d:e.b*t.c+e.d*t.d,e:e.a*t.e+e.c*t.f+e.e,f:e.b*t.e+e.d*t.f+e.f}}function Gg(e){const t=e.a*e.d-e.b*e.c;return Math.abs(t)<ei?null:{a:e.d/t,b:-e.b/t,c:-e.c/t,d:e.a/t,e:(e.c*e.f-e.d*e.e)/t,f:(e.b*e.e-e.a*e.f)/t}}function nn(e){const t=e.rotation*Math.PI/180,r=Math.cos(t),n=Math.sin(t);return{a:r*e.width,b:n*e.width,c:-n*e.height,d:r*e.height,e:e.x,f:e.y}}function E_(e,t){const r=Math.max(ei,Math.hypot(t.a,t.b)),n=t.a*t.d-t.b*t.c,i=Math.max(ei,Math.abs(n)/r),o=r/Math.max(ei,e.width),a=i/Math.max(ei,e.height);return{...e,x:t.e,y:t.f,width:r,height:i,anchorX:e.anchorX*o,anchorY:e.anchorY*a,rotation:Math.atan2(t.b,t.a)*180/Math.PI,cornerRadius:e.cornerRadius*Math.sqrt(o*a)}}function C_(e,t){const r=Gg(nn(t.childLocalReference));return r?ga(nn(t.childWorldReference),ga(r,nn(e))):nn(e)}function ps(e){return{x:e.x,y:e.y,width:e.width,height:e.height,rotation:e.rotation}}function M_(e){return{...e.parentItemId&&{parentItemId:e.parentItemId},...e.parentWorld&&{parentReference:ps(e.parentWorld)},childLocalReference:ps(e.childLocal),childWorldReference:ps(e.childWorld)}}function __(e,t,r){let n=C_(e,t);if(r&&t.parentReference){const i=Gg(nn(t.parentReference));if(i){const o=ga(nn(r),i);n=ga(o,n)}}return E_(e,n)}function Wg(e,t,r={cache:new Map,active:new Set}){const n=r.cache.get(e.id);if(n)return n;const i=t.resolveLocal(e),o=e.transformParent;if(!o||r.active.has(e.id))return i;r.active.add(e.id);const a=o.parentItemId?t.getItem(o.parentItemId):void 0,s=a?Wg(a,t,r):void 0,l=__(i,o,s);return r.active.delete(e.id),r.cache.set(e.id,l),l}function F_(e,t,r,n=()=>{}){return Vg(e,t,r,n)}function Vg(e,t,r,n){const i=new Set,o=[t];for(;o.length>0;){const a=o.pop();if(a===e)return!0;if(i.has(a))continue;i.add(a);const s=r(a)?.transformParent?.parentItemId;s&&o.push(s);for(const l of Et(n(a)))l.enabled&&o.push(l.sourceItemId)}return!1}const A_=new Set(["position","scale","x","y","width","height","rotation"]);function Zg(e){return A_.has(e)}function O_(e,t,r){const n=new Set;let i=r(e)?.transformParent?.parentItemId;for(;i;){if(i===t)return!0;if(n.has(i))return!1;n.add(i),i=r(i)?.transformParent?.parentItemId}return!1}function z_(e,t,r,n){const i=new Set,o=new Set;let a=t;for(;a&&!o.has(a);)o.add(a),i.add(a),a=r(a)?.transformParent?.parentItemId;return Et(n(e)).some(s=>s.enabled&&Zg(s.targetProperty)&&i.has(s.sourceItemId))}const ue=I().min(1),ft=F(ue).min(1),dr=P().finite(),D_=Ct(I(),Fa()),R_=["video","audio","text","image","lottie","shape","adjustment","controller","composition","subtitle"],on=zi({id:ue.optional(),type:U(R_),trackId:ue,from:dr,durationInFrames:P().finite().positive(),label:I(),templateAnchor:U(["full","start","end"]).optional()}).superRefine((e,t)=>{const r=hg(e);if(r&&t.addIssue({code:"custom",path:["waveformVisualizer"],message:r}),e.templateAnchor==="full"&&e.type==="video"&&zl(e)!==null&&t.addIssue({code:"custom",path:["templateAnchor"],message:"Clip neo toàn video phải lặp được (đã tắt tiếng, có khoảng nguồn)"}),e.loop!==!0)return;if(e.type!=="video"){t.addIssue({code:"custom",path:["loop"],message:"Chỉ clip video mới lặp được"});return}if(e.embeddedAudioMuted!==!0){t.addIssue({code:"custom",path:["loop"],message:"Chỉ lặp được clip đã tắt tiếng"});return}const n=e.sourceEnd??e.sourceDuration,i=e.sourceStart??e.trimStart??e.offset??0;(n===void 0||!Number.isFinite(n)||n<=i)&&t.addIssue({code:"custom",path:["loop"],message:"Clip cần biết khoảng nguồn để lặp"})}),Yg=U(["scenes","captions","source-audio","background-video"]),pn=zi({id:ue,name:I(),height:P().finite().positive(),locked:B(),visible:B(),muted:B(),solo:B(),order:P().finite(),kind:U(["video","audio"]).optional(),templateSlot:Yg.optional(),templateLoudnessLufs:P().min(-24).max(-9).optional(),templateScenes:fg.optional(),templateBackdrop:pg.optional(),templateBackgroundAppearance:mg.optional()}).superRefine((e,t)=>{e.templateBackgroundAppearance!==void 0&&(e.templateSlot!=="background-video"||e.kind==="audio")&&t.addIssue({code:"custom",path:["templateBackgroundAppearance"],message:"Cài đặt nền chỉ đặt trên track video vai trò Nền video"})}),Ld=y({id:ue,from:dr,trackId:ue.optional()}).strict(),L_=y({from:dr,trackId:ue}).strict(),Bd=y({width:P().finite().positive(),height:P().finite().positive(),fps:P().finite().positive()}).strict(),fo=y({x:P().finite().optional(),y:P().finite().optional(),width:P().finite().optional(),height:P().finite().optional(),anchorX:P().finite().optional(),anchorY:P().finite().optional(),rotation:P().finite().optional(),flipHorizontal:B().optional(),flipVertical:B().optional(),opacity:P().finite().optional(),cornerRadius:P().finite().optional(),aspectRatioLocked:B().optional()}).strict(),po=U(["move","resize","rotate","opacity","corner_radius","transform"]),Ud=U(["linear","ease-in","ease-out","ease-in-out","hold","cubic-bezier","spring"]),Nd=y({x:P().finite(),y:P().finite()}).strict(),ms=_a("type",[y({type:ne("add"),itemId:ue,property:I().min(1),frame:dr,value:P().finite(),easing:Ud.optional()}).strict(),y({type:ne("update"),itemId:ue,property:I().min(1),keyframeId:ue,updates:y({value:P().finite().optional()}).strict()}).strict(),y({type:ne("vector-add"),itemId:ue,property:U(["position","scale","anchor"]),frame:dr,value:Nd,easing:Ud.optional()}).strict(),y({type:ne("vector-update"),itemId:ue,property:U(["position","scale","anchor"]),keyframeId:ue,updates:y({value:Nd}).strict()}).strict()]),B_=y({position:ui([P().finite(),P().finite()]),inHandle:ui([P().finite(),P().finite()]),outHandle:ui([P().finite(),P().finite()]),tangentMode:U(["corner","smooth","continuous","broken"]).optional()}).strict(),U_=y({preset:U(["auto","row","column","pip","focus-sidebar","grid"]),cols:P().int().positive().optional(),rows:P().int().positive().optional(),gap:P().finite().optional(),padding:P().finite().optional()}).strict(),N_=y({src:I(),path:I(),key:I(),quality:U(["preview","full"]),usesProxy:B(),isSourceLevel:B(),sourceDuration:P().finite().optional(),conformFps:P().finite().optional()}).strict(),$_=["reverseConformSrc","reverseConformPath","reverseConformKey","reverseConformPreviewSrc","reverseConformPreviewPath","reverseConformPreviewKey","reverseConformPreviewUsesProxy","reverseConformPreviewIsSourceLevel","reverseConformPreviewSourceDuration","reverseConformPreviewFps","reverseConformStatus"],V=()=>({kind:"patch",patch:{}});function mo(e,t){return{...t,id:t.id??e.ids.mint()}}function Fr(e,t){const r={...e};for(const n of t??[])r[n]=void 0;return r}function Ne(e,t,r="itemIds"){const n=new Set(e.items.map(i=>i.id));return t.filter(i=>!n.has(i)).map(i=>({message:`Không tìm thấy clip ${i}`,code:"item.notFound",path:r}))}function Bn(e,t,r="itemIds"){const n=new Set(t);return[...Ne(e,t,r),...e.items.filter(i=>n.has(i.id)&&i.type==="audio").map(i=>({message:`Clip âm thanh ${i.id} không hỗ trợ biến đổi hình ảnh`,code:"item.unsupportedTransform",path:r}))]}function Tr(e,t,r="trackId"){const n=new Set(e.map(i=>i.id));return[...new Set(t)].filter(i=>i!==void 0&&!n.has(i)).map(i=>({message:`Không tìm thấy track ${i}`,code:"track.notFound",path:r}))}function Xg(e,t){return new Set(e).size===e.length?[]:[{message:"Danh sách id bị trùng lặp",code:"id.duplicate",path:t}]}function H_(e,t,r){const n=new Set(e.items.map(i=>i.id));return t.filter(i=>i!==void 0&&n.has(i)).map(i=>({message:`Clip ${i} đã tồn tại`,code:"item.exists",path:r}))}function Qg(){const e=S.getState(),t=ey(e.tracks,e.items);t!==e.tracks&&e.setTracks(t)}function j_(e){const t=S.getState().items,n=q.getState().transitions.map(i=>{if(!e.has(i.leftClipId)||!e.has(i.rightClipId))return i;const o=t.find(s=>s.id===i.leftClipId),a=t.find(s=>s.id===i.rightClipId);return o&&a&&o.trackId===a.trackId?{...i,trackId:o.trackId}:i});q.getState().setTransitions(n)}function wc(e){return e.type!=="audio"&&e.type!=="adjustment"}function ho(e,t,r){const n=t.flatMap(i=>i.id?[i.id]:[]);return[...Xg(n,"items"),...H_(e,n,"items"),...Tr(r,t.map(i=>i.trackId),"items.trackId"),...t.flatMap(i=>i.type!=="video"||i.templateAnchor!=="full"||!i.linkedGroupId?[]:[...e.items,...t].some(a=>a.type==="audio"&&a.linkedGroupId===i.linkedGroupId)?[{message:"Clip neo toàn video phải tắt tiếng và không gắn âm thanh",path:"items.templateAnchor"}]:[]),...t.flatMap(i=>{if(!i.waveformVisualizer)return[];const o=e.mediaItems?.find(s=>s.id===i.mediaId);if(!o||!o.mimeType.startsWith("audio/"))return[{message:"Chọn tệp âm thanh làm nguồn giọng",path:"items.mediaId"}];const a=i.waveformVisualizer;return a.envelopeBase64!==void 0&&a.sourceHash!==o.contentHash?[{message:"Dữ liệu sóng giọng đã cũ; tạo lại từ nguồn hiện tại",path:"items.waveformVisualizer"}]:[]})]}function $d(e){S.getState()._removeItems(e),q.getState()._removeTransitionsForItems(e),$.getState()._removeKeyframesForItems(e),Qg()}function q_(e,t){const r=S.getState().items,n=Rr(r,e,t);if(n.length===0)return[];const i=new Set(n),o=r.filter(k=>!i.has(k.id)),a=new Map,s=new Set(r.filter(k=>i.has(k.id)).map(k=>k.trackId)),l=r.filter(k=>i.has(k.id)).map(k=>({start:k.from,end:k.from+k.durationInFrames}));for(const k of o){const T=r.filter(O=>i.has(O.id)).filter(O=>O.trackId===k.trackId&&O.from+O.durationInFrames<=k.from).reduce((O,C)=>O+C.durationInFrames,0);T>0&&a.set(k.id,T)}const c=new Map(S.getState().tracks.map(k=>[k.id,k])),u=new Map(o.map(k=>[k.id,k])),d=new Map;for(const[k,T]of a){if(T<=0)continue;const O=Rr(o,[k],t);for(const C of O){const R=u.get(C);!R||!s.has(R.trackId)&&ha(c.get(R.trackId))||d.set(C,Math.max(d.get(C)??0,T))}}const f=o.flatMap(k=>{const T=d.get(k.id)??0;return T>0?[{id:k.id,from:k.from-T}]:[]}),p=new Map(f.map(k=>[k.id,k.from])),m=[];for(const k of o){if(p.has(k.id)||i.has(k.id))continue;const T=k.from+k.durationInFrames;for(const O of o){const C=p.get(O.id);if(C===void 0||O.trackId!==k.trackId)continue;const R=C+O.durationInFrames;if(C<T&&R>k.from){m.push(k.id);break}}}const h=Rr(o,m,t),g=[...n,...h],v=new Set(h),x=v.size>0?f.filter(k=>!v.has(k.id)):f;S.getState()._removeItems(g),x.length>0&&S.getState()._moveItems(x);const b=Ua({editedTrackIds:s,intervals:l}),w=Array.from(new Set([...g,...b.removedIds]));q.getState()._removeTransitionsForItems(w),$.getState()._removeKeyframesForItems(w),x.length>0&&pe(x.map(k=>k.id));const E=Array.from(new Set([...f.map(k=>k.id),...b.affectedIds]));return E.length>0&&pe(E,new Set(w)),Qg(),g}function K_(e,t){const r=S.getState().items,n=Math.max(0,Math.round(t)),i=r.filter(d=>d.trackId===e).sort((d,f)=>d.from-f.from);if(i.length===0)return;let o=0,a=0;for(const d of i){if(n>=o&&n<d.from){a=d.from;break}o=d.from+d.durationInFrames}if(a<=o)return;const s=a-o,l=r.filter(d=>d.trackId===e&&d.from>=a).map(d=>({id:d.id,from:d.from-s}));if(l.length===0)return;S.getState()._moveItems(l);const c=Ua({editedTrackIds:new Set([e]),intervals:[{start:o,end:a}]}),u=c.removedIds;u.length>0&&(q.getState()._removeTransitionsForItems(u),$.getState()._removeKeyframesForItems(u)),pe(Array.from(new Set([...l.map(d=>d.id),...c.affectedIds])),u.length>0?new Set(u):void 0)}function G_(e,t){const r=S.getState().items,n=r.filter(s=>s.trackId===e).sort((s,l)=>s.from-l.from);if(n.length===0)return;let i=0;const o=new Map;for(const s of n){const l=s.from>i?i:s.from,c=s.from-l;c>0&&o.set(s.id,c),i=l+s.durationInFrames}const a=t_(r,o,t);a.length!==0&&(S.getState()._moveItems(a),pe(a.map(s=>s.id)))}function W_(e,t,r){if(r.length===0)return;const n=new Map;for(let o=0;o<t.length;o+=1){const a=r[o];a&&n.set(t[o],a)}const i=[];for(const o of q.getState().transitions){const a=n.get(o.leftClipId),s=n.get(o.rightClipId);!a||!s||a.trackId!==s.trackId||i.push({...o,id:e.ids.mint(),leftClipId:a.id,rightClipId:s.id,trackId:a.trackId})}i.length>0&&q.getState().setTransitions([...q.getState().transitions,...i])}function Jg(e){if(e)return{...e,...e.bezier&&{bezier:{...e.bezier}},...e.spring&&{spring:{...e.spring}}}}function V_(e,t){return{...e,id:t(),easingConfig:Jg(e.easingConfig)}}function Z_(e,t){return{...e,id:t(),value:{...e.value},easingConfig:Jg(e.easingConfig),temporalEase:e.temporalEase?{...e.temporalEase.in&&{in:{...e.temporalEase.in}},...e.temporalEase.out&&{out:{...e.temporalEase.out}}}:void 0,spatial:e.spatial?{...e.spatial,inTangent:{...e.spatial.inTangent},outTangent:{...e.spatial.outTangent}}:void 0}}function Y_(e,t){return{property:e.property,keyframes:e.keyframes.map(r=>V_(r,t))}}function X_(e,t){return{property:e.property,keyframes:e.keyframes.map(r=>Z_(r,t))}}function e0(e,t){return{...e,sourceItemId:t.get(e.sourceItemId)??e.sourceItemId}}function Q_(e,t){return e.type==="link"?e0(e,t):{...e}}function J_(e,t,r,n){const i={itemId:t,properties:e.properties.map(o=>Y_(o,n))};return e.animationVersion&&(i.animationVersion=e.animationVersion),e.expressions?.length&&(i.expressions=e.expressions.map(o=>Q_(o,r))),e.propertyLinks?.length&&(i.propertyLinks=e.propertyLinks.map(o=>e0(o,r))),e.vectorProperties?.length&&(i.vectorProperties=e.vectorProperties.map(o=>X_(o,n))),e.separatedVectorProperties?.length&&(i.separatedVectorProperties=[...e.separatedVectorProperties]),i}function eF(e,t,r){if(r.length===0)return;const n=$.getState(),i=new Map(t.flatMap((s,l)=>{const c=r[l];return c?[[s,c.id]]:[]})),o=()=>e.ids.mint(),a=t.flatMap((s,l)=>{const c=r[l],u=n.keyframesByItemId[s];return!c||!u?[]:[J_(u,c.id,i,o)]});a.length>0&&n.setKeyframes([...n.keyframes,...a])}function Hd(e,t){const r=new Set;for(const i of t){const o=Ei(e,i);if(o.length===0){r.add(i);continue}for(const a of o)r.add(a.id)}const n=Array.from(r).map(i=>e.find(o=>o.id===i)).filter(i=>i!==void 0&&(i.type==="video"||i.type==="audio"));return{reversibleItems:n,shouldReverse:!n.every(i=>i.isReversed===!0)}}function tF(e,t,r){return!t||!wc(t)?!0:!!(r.parentItemId&&(F_(e,r.parentItemId,r.getItem,r.getKeyframes)||z_(e,r.parentItemId,r.getItem,r.getKeyframes)))}function rF(e,t,r){return tF(e,t,r)?{status:"invalid"}:r.behavior==="preserve-world"&&t?.transformParent?.parentItemId===r.parentItemId?{status:"unchanged"}:!r.parentItemId&&r.behavior==="restore-local"?t?.transformParent?{status:"update",binding:void 0}:{status:"unchanged"}:null}function nF(e,t,r,n){return e==="snap-to-parent"&&n?{...t,x:n.x,y:n.y}:e==="restore-local"?r:t}function iF(e,t){const{parentItemId:r,parentWorld:n,behavior:i,frame:o,canvas:a,getItem:s,getKeyframes:l}=t,c=s(e),u=rF(e,c,t);if(u)return u;if(!c)return{status:"invalid"};const d=l(c.id),f=O0(c,{canvas:a,frame:o,keyframes:d,getItem:s,getKeyframes:l}),p=A0(c,{canvas:a,relativeFrame:o-c.from,keyframes:d,expressionContext:{globalFrame:o,canvas:a,getItem:s,getKeyframes:l}});return{status:"update",binding:M_({childLocal:p,childWorld:nF(i,f,p,n),parentItemId:r,parentWorld:n})}}function jd(e,t){const r=new Map;for(const n of e){const i=iF(n,t);if(i.status==="invalid")return null;i.status==="update"&&r.set(n,i.binding)}return r}function qd(e){const{parentItemId:t,parent:r,behavior:n="preserve-world",frame:i,canvas:o,getItem:a,getKeyframes:s}=e,l=r?O0(r,{canvas:o,frame:i,keyframes:s(r.id),getItem:a,getKeyframes:s}):void 0;return{parentItemId:t,parentWorld:l,behavior:n,frame:i,canvas:o,getItem:a,getKeyframes:s}}function oF(e,t,r){return e.length===0||t&&e.includes(t)?!1:t?!!(r&&wc(r)):!0}function aF(e){const t="Null Object",r=new Set(e.map(i=>i.label));if(!r.has(t))return t;let n=2;for(;r.has(`${t} ${n}`);)n+=1;return`${t} ${n}`}function sF(e,t){const r=new Map(t.map(u=>[u.id,u])),n=e.map(u=>r.get(u.trackId)).filter(u=>!!u);if(n.length===0)return{order:t.reduce((u,d)=>Math.max(u,d.order),-1)+1};const i=new Set(n.map(u=>u.parentTrackId??null)),o=i.size===1?n[0]?.parentTrackId??void 0:void 0,a=i.size===1?n:n.map(u=>u.parentTrackId&&r.get(u.parentTrackId)||u),s=t.filter(u=>(u.parentTrackId??null)===(o??null)),l=a.filter(u=>(u.parentTrackId??null)===(o??null)).map(u=>u.order);return{order:l.length>0?Math.min(...l):s.reduce((u,d)=>Math.max(u,d.order),-1)+1,...o&&{parentTrackId:o}}}function lF(e,t){const{children:r,items:n,tracks:i,canvas:o}=t,a=e.ids.mint(),s=Math.min(...r.map(p=>p.from)),l=Math.max(...r.map(p=>p.from+p.durationInFrames)),c=aF(n),u=sF(r,i),d={...v_({trackId:a,from:s,durationInFrames:Math.max(1,l-s),canvasWidth:o.width,canvasHeight:o.height,fps:o.fps}),id:e.ids.mint(),label:c},f={id:a,name:c,kind:"video",height:34,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,order:u.order,...u.parentTrackId&&{parentTrackId:u.parentTrackId},items:[]};return{parent:d,track:f}}function Kd(){const e=S.getState(),t=$.getState();return{getItem:r=>e.itemById[r],getKeyframes:r=>t.keyframesByItemId[r]}}function hs(e){if(e.length===0)return!1;const t=$.getState();let r=!1;for(const n of e){if(n.type==="vector-update"){t._updateVectorKeyframe(n.itemId,n.property,n.keyframeId,n.updates),r=!0;continue}if(n.type==="update"){t._updateKeyframe(n.itemId,n.property,n.keyframeId,n.updates),r=!0;continue}if(!et(n.itemId,n.frame)){Me().warn("Cannot add auto keyframe in transition region",{itemId:n.itemId,property:n.property,frame:n.frame});continue}n.type==="vector-add"?t._upsertVectorKeyframe(n.itemId,n.property,{frame:n.frame,value:n.value,easing:n.easing}):t._addKeyframe(n.itemId,n.property,n.frame,n.value,n.easing),r=!0}return r}const cF=["x","y","width","height","rotation"];function gs(e){return Object.keys(e).sort().join(",")}function ys(e,t){return Ne(e,[...new Set((t??[]).map(r=>r.itemId))],"autoKeyframeOperations")}function go(e,t){const r=e.items.filter(i=>t.has(i.id)).map(i=>i.id);if(r.length===0)return{...e,removedItemIds:r};const n=new Set(r);return{...e,items:e.items.filter(i=>!n.has(i.id)),transitions:e.transitions.filter(i=>!n.has(i.leftClipId)&&!n.has(i.rightClipId)),keyframes:e.keyframes.filter(i=>!n.has(i.itemId)),removedItemIds:r}}function yo(e,t,r){let n=!1;const i=e.map(o=>o.id!==t?o:(n=!0,{...o,...r}));return{items:n?i:e,changed:n}}function uF(e){const t=new Set(e),r=Ba(),n=go(r,t);let i=n.removedItemIds.length>0;n.removedItemIds.length>0&&(S.getState().setItems(n.items),q.getState().setTransitions(n.transitions),$.getState().setKeyframes(n.keyframes));const o=ye.getState().activeCompositionId,a=ae.getState().compositions.map(d=>{if(d.id===o&&n.removedItemIds.length>0)return{...d,items:n.items,tracks:n.tracks,transitions:n.transitions,keyframes:n.keyframes};const f=d.id===o?{items:n.items,tracks:n.tracks,transitions:n.transitions,keyframes:n.keyframes}:{items:d.items,tracks:d.tracks,transitions:d.transitions,keyframes:d.keyframes},p=go(f,t);return p.removedItemIds.length>0&&(i=!0),p.removedItemIds.length===0?d:{...d,items:p.items,tracks:p.tracks,transitions:p.transitions,keyframes:p.keyframes}}),s=ye.getState(),l=s.stashStack.map(d=>{const f=go(d,t);return f.removedItemIds.length>0&&(i=!0),f.removedItemIds.length===0?d:{...d,items:f.items,transitions:f.transitions,keyframes:f.keyframes}}),c=s.mainHolder;let u=c;if(c){const d=go(c,t);d.removedItemIds.length>0&&(i=!0,u={...c,items:d.items,transitions:d.transitions,keyframes:d.keyframes})}return i&&(ae.getState().setCompositions(a),ye.setState(d=>({...d,stashStack:l,mainHolder:u}))),i}function dF(e,t){const r=Ba(),n=yo(r.items,e,t);let i=n.changed;n.changed&&S.getState().setItems(n.items);const o=ye.getState().activeCompositionId,a=ae.getState().compositions.map(f=>{if(f.id===o&&n.changed)return{...f,items:n.items};const p=f.id===o?n.items:f.items,m=yo(p,e,t);return m.changed&&(i=!0),m.changed?{...f,items:m.items}:f}),s=ye.getState(),l=s.stashStack.map(f=>{const p=yo(f.items,e,t);return p.changed&&(i=!0),p.changed?{...f,items:p.items}:f}),c=s.mainHolder;let u=c;if(c){const f=yo(c.items,e,t);f.changed&&(i=!0,u={...c,items:f.items})}return i?(ae.getState().setCompositions(a),ye.setState(f=>({...f,stashStack:l,mainHolder:u})),("from"in t||"durationInFrames"in t||"trackId"in t)&&n.changed&&pe([e]),!0):!1}const vs={updates:D_,clearFields:F(I().min(1)).optional()};function bs(e,t){const r=[];return"templateAnchor"in e&&!U(["full","start","end"]).safeParse(e.templateAnchor).success&&r.push({message:"Mốc neo template không hợp lệ",path:"updates.templateAnchor"}),("id"in e||t?.includes("id"))&&r.push({message:"Không thể đổi id của clip",code:"item.idImmutable",path:"updates.id"}),"from"in e&&!(typeof e.from=="number"&&Number.isFinite(e.from))&&r.push({message:"from phải là số frame hợp lệ",path:"updates.from"}),"durationInFrames"in e&&!(typeof e.durationInFrames=="number"&&Number.isFinite(e.durationInFrames)&&e.durationInFrames>0)&&r.push({message:"durationInFrames phải lớn hơn 0",path:"updates.durationInFrames"}),r}const Gd={waveformVisualizer:!0,type:!0,shapeType:!0,mediaId:!0,isMask:!0};function kc(e){return[e.items,...e.compositions.map(t=>t.items),...(e.stashStack??[]).map(t=>t.items),...e.mainHolder?[e.mainHolder.items]:[]]}function Wd(e,t,r,n){return r.loop!==!0?[]:kc(e).flatMap(i=>i.filter(o=>o.id===t).flatMap(o=>{if(o.type!=="video")return[{message:"Chỉ clip video mới lặp được",code:"item.loopVideoOnly",path:"updates.loop"}];const a={...o,...Fr(r,n)},s=i.some(c=>c.type==="audio"&&!!o.linkedGroupId&&c.linkedGroupId===o.linkedGroupId),l=zl(a,s);return l==="unmuted"?[{message:"Chỉ lặp được clip đã tắt tiếng",code:"item.loopNeedsMuted",path:"updates.loop"}]:l==="no-window"?[{message:"Clip cần biết khoảng nguồn để lặp",code:"item.loopNeedsWindow",path:"updates.loop"}]:[]}))}function fF(e,t,r,n){return kc(e).flatMap(i=>i.filter(o=>o.id===t).flatMap(o=>{const a={...o,...Fr(r,n)};if(a.type!=="video"||a.templateAnchor!=="full")return[];const s=i.some(c=>c.type==="audio"&&!!a.linkedGroupId&&c.linkedGroupId===a.linkedGroupId),l=zl(a,s);return l==="unmuted"?[{message:"Clip neo toàn video phải tắt tiếng",code:"item.anchorNeedsMuted",path:"updates.templateAnchor"}]:l==="no-window"?[{message:"Clip neo toàn video cần biết khoảng nguồn",code:"item.anchorNeedsWindow",path:"updates.templateAnchor"}]:[]}))}function Vd(e,t){if(e.type!=="video"||e.loop!==!0)return{};const r={...e,...t};if(r.loop===!0&&r.embeddedAudioMuted===!0)return{};const n={loop:void 0},i=W.getState().fps,o=e.sourceFps??i,a=e.speed??1,s=e.sourceStart??e.trimStart??e.offset??0,l=e.sourceEnd??e.sourceDuration;if(l!==void 0&&Number.isFinite(l)&&l>s&&r.durationInFrames!==void 0){const c=Math.max(1,Kt(l-s,a,o,i));r.durationInFrames>c&&(n.durationInFrames=c)}return n}function Zd(e,t,r,n){let i=!1;for(const o in r)if(Gd[o]){i=!0;break}if(!i&&n){for(const o of n)if(Gd[o]){i=!0;break}}return i?kc(e).flatMap(o=>o.filter(a=>a.id===t).flatMap(a=>{const s={...a,...Fr(r,n)},l=e.mediaItems?.find(u=>u.id===s.mediaId),c=hg(s,l?.contentHash);return s.waveformVisualizer&&(!l||!l.mimeType.startsWith("audio/"))?[{message:"Chọn tệp âm thanh làm nguồn giọng",path:"updates.mediaId"}]:c?[{message:c,path:"updates.waveformVisualizer"}]:[]})):[]}function pF(){A({type:"item.add",params:y({item:on}).strict(),validate:(r,n)=>ho(r,[n.item],r.tracks),apply(r,n){const[i]=Pl([mo(r,n.item)]);return i?(S.getState()._addItem(i),{inverse:V(),result:i}):{inverse:V(),result:null}}}),A({type:"item.addMany",params:y({items:F(on).min(1)}).strict(),validate:(r,n)=>ho(r,n.items,r.tracks),apply(r,n){const i=Pl(n.items.map(o=>mo(r,o)));return S.getState()._addItems(i),{inverse:V(),result:i}}}),A({type:"item.addWithLinkedAudio",params:y({video:on.refine(r=>r.type==="video",{message:"Chỉ hỗ trợ clip video"}).refine(r=>r.loop!==!0,{message:"Clip lặp không gắn âm thanh được"})}).strict(),validate:(r,n)=>ho(r,[n.video],r.tracks),apply(r,n){const i=S.getState(),o=new Map;for(const c of i.items){const u=o.get(c.trackId);u?u.push(c):o.set(c.trackId,[c])}const{updatedVideo:a,audioItem:s,newTrack:l}=y_({video:mo(r,n.video),tracks:i.tracks,itemsByTrackId:o,createId:()=>r.ids.mint()});return l&&i.setTracks([...i.tracks,l]),i._addItems([a,s]),{inverse:V(),result:{video:a,audio:s,track:l}}}}),A({type:"item.addOnNewTracks",params:y({items:F(on).min(1),tracks:F(pn).min(1)}).strict(),validate:(r,n)=>ho(r,n.items,n.tracks),apply(r,n){const i=S.getState(),o=n.items.map(a=>mo(r,a));return i.setTracks(n.tracks),i._addItems(o),{inverse:V(),result:o}}}),A({type:"item.update",params:y({id:ue,...vs}).strict(),validate:(r,n)=>[...Ne(r,[n.id],"id"),...bs(n.updates,n.clearFields),...Zd(r,n.id,n.updates,n.clearFields),...Wd(r,n.id,n.updates,n.clearFields),...fF(r,n.id,n.updates,n.clearFields),...typeof n.updates.trackId=="string"?Tr(r.tracks,[n.updates.trackId],"updates.trackId"):[]],mergeKey:r=>`item.update:${r.id}:${[...Object.keys(r.updates),...r.clearFields??[]].sort().join(",")}`,apply(r,n){const i=S.getState().itemById[n.id],o={...Fr(n.updates,n.clearFields),...i?Vd(i,Fr(n.updates,n.clearFields)):{}};return S.getState()._updateItem(n.id,o),("from"in o||"durationInFrames"in o||"trackId"in o)&&pe([n.id]),V()}}),A({type:"item.move",params:Ld,validate:(r,n)=>[...Ne(r,[n.id],"id"),...Tr(r.tracks,[n.trackId],"trackId")],mergeKey:r=>`item.move:${r.id}`,apply(r,n){return S.getState()._moveItem(n.id,n.from,n.trackId),pe([n.id]),V()}}),A({type:"item.moveMany",params:y({updates:F(Ld).min(1),tracks:F(pn).optional()}).strict(),validate:(r,n)=>[...Ne(r,n.updates.map(i=>i.id),"updates.id"),...Xg(n.updates.map(i=>i.id),"updates.id"),...Tr(n.tracks??r.tracks,n.updates.map(i=>i.trackId),"updates.trackId")],mergeKey:r=>`item.moveMany:${r.updates.map(n=>n.id).sort().join(",")}${r.tracks?":tracks":""}`,apply(r,n){const i=S.getState();return n.tracks&&i.setTracks(n.tracks),i._moveItems(n.updates),j_(new Set(n.updates.map(o=>o.id))),pe(n.updates.map(o=>o.id)),V()}}),A({type:"item.trackPush",params:y({anchorId:ue,delta:P().finite().refine(r=>r!==0,{message:"delta phải khác 0"})}).strict(),validate:(r,n)=>Ne(r,[n.anchorId],"anchorId"),mergeKey:r=>`item.trackPush:${r.anchorId}`,apply(r,n){const i=S.getState().items,o=i.find(s=>s.id===n.anchorId);if(!o)return V();const a=i.filter(s=>s.from>=o.from).map(s=>({id:s.id,from:Math.max(0,s.from+n.delta)}));return a.length===0||(S.getState()._moveItems(a),pe(a.map(s=>s.id))),V()}}),A({type:"item.duplicate",params:y({itemIds:ft,positions:F(L_).min(1),tracks:F(pn).optional()}).strict(),validate:(r,n)=>[...Ne(r,n.itemIds),...n.itemIds.length===n.positions.length?[]:[{message:"positions phải cùng số lượng với itemIds",path:"positions"}],...Tr(n.tracks??r.tracks,n.positions.map(i=>i.trackId),"positions.trackId")],apply(r,n){const i=S.getState();n.tracks&&i.setTracks(n.tracks);const o=i._duplicateItems(n.itemIds,n.positions);return W_(r,n.itemIds,o),eF(r,n.itemIds,o),{inverse:V(),result:o}}});const e=(r,n)=>{const i=new Set(r.items.map(o=>o.id));return n.some(o=>i.has(o))?[]:[{message:"Không tìm thấy clip nào để xóa",code:"item.notFound",path:"itemIds"}]};A({type:"item.remove",params:y({itemIds:ft,linkedSelection:B()}).strict(),validate:(r,n)=>e(r,n.itemIds),apply(r,n){const i=Rr(S.getState().items,n.itemIds,n.linkedSelection);return i.length>0&&$d(i),{inverse:V(),result:i}}}),A({type:"item.removeExact",params:y({itemIds:ft}).strict(),validate:(r,n)=>e(r,n.itemIds),apply(r,n){const i=new Set(S.getState().items.map(a=>a.id)),o=[...new Set(n.itemIds)].filter(a=>i.has(a));return o.length>0&&$d(o),{inverse:V(),result:o}}}),A({type:"item.rippleDelete",params:y({itemIds:ft,linkedSelection:B()}).strict(),validate:(r,n)=>e(r,n.itemIds),apply(r,n){const i=q_(n.itemIds,n.linkedSelection);return{inverse:V(),result:i}}}),A({type:"item.closeGap",params:y({trackId:ue,frame:dr}).strict(),validate:(r,n)=>Tr(r.tracks,[n.trackId],"trackId"),apply(r,n){return K_(n.trackId,n.frame),V()}}),A({type:"item.closeAllGaps",params:y({trackId:ue,linkedSelection:B()}).strict(),validate:(r,n)=>Tr(r.tracks,[n.trackId],"trackId"),apply(r,n){return G_(n.trackId,n.linkedSelection),V()}}),A({type:"item.link",params:y({itemIds:ft}).strict(),validate:(r,n)=>{const i=Ne(r,n.itemIds);if(i.length>0)return i;const o=Ci(r.items,n.itemIds);return e_(r.items,n.itemIds)&&o.length>=2?[]:[{message:"Không thể liên kết các clip đã chọn",code:"item.cannotLink",path:"itemIds"}]},apply(r,n){const i=S.getState().items,o=Ci(i,n.itemIds).map(c=>i.find(u=>u.id===c)).filter(c=>c!==void 0),a=r.ids.mint(),s=S.getState(),l=o.some(c=>c.type==="audio");for(const c of o)s._updateItem(c.id,{linkedGroupId:a,...c.type==="video"&&{embeddedAudioMuted:void 0,...l&&{loop:void 0}}});return{inverse:V(),result:o.map(c=>c.id)}}}),A({type:"item.unlink",params:y({itemIds:ft}).strict(),validate:(r,n)=>Ne(r,n.itemIds),apply(r,n){const i=S.getState().items,o=new Set;for(const c of n.itemIds)for(const u of Hi(i,c))o.add(u);const a=i.filter(c=>o.has(c.id)&&c.linkedGroupId);if(a.length===0)return{inverse:V(),result:[]};const s=new Set;for(const c of a){if(c.type!=="video")continue;a.some(d=>d.type==="audio"&&d.linkedGroupId===c.linkedGroupId)&&s.add(c.id)}const l=S.getState();for(const c of a)l._updateItem(c.id,{linkedGroupId:c.id,...s.has(c.id)&&{embeddedAudioMuted:!0}});return{inverse:V(),result:a.map(c=>c.id)}}}),A({type:"item.reverse",params:y({itemIds:ft}).strict(),validate:(r,n)=>{const i=Ne(r,n.itemIds);return i.length>0?i:Hd(r.items,n.itemIds).reversibleItems.length>0?[]:[{message:"Chỉ clip video hoặc âm thanh mới đảo ngược được",code:"item.notReversible",path:"itemIds"}]},apply(r,n){const{reversibleItems:i,shouldReverse:o}=Hd(S.getState().items,n.itemIds),a=S.getState();for(const s of i){const l={isReversed:o?!0:void 0};if(!o)for(const c of $_)l[c]=void 0;a._updateItem(s.id,l)}return{inverse:V(),result:{ids:i.map(s=>s.id),reversed:o}}}}),A({type:"item.commitPreparedReverse",params:y({entries:F(y({itemId:ue,conform:N_.optional()}).strict()).min(1)}).strict(),validate:(r,n)=>Ne(r,n.entries.map(i=>i.itemId),"entries.itemId"),apply(r,n){const i=S.getState();for(const o of n.entries){const a=i.itemById[o.itemId];if(!a)continue;const s=o.conform,l=a.type==="video"&&s?{...s.quality==="full"?{reverseConformSrc:s.src,reverseConformPath:s.path,reverseConformKey:s.key}:{reverseConformPreviewSrc:s.src,reverseConformPreviewPath:s.path,reverseConformPreviewKey:s.key,reverseConformPreviewUsesProxy:s.usesProxy,reverseConformPreviewIsSourceLevel:s.isSourceLevel,reverseConformPreviewSourceDuration:s.sourceDuration,reverseConformPreviewFps:s.conformFps},reverseConformStatus:"ready"}:{};i._updateItem(a.id,{isReversed:!0,...l})}return V()}});const t=y({childItemIds:ft,parentItemId:ue.optional(),behavior:U(["preserve-world","snap-to-parent","restore-local"]).optional(),frame:dr,canvas:Bd}).strict();A({type:"item.setTransformParents",params:t,validate:(r,n)=>{const i=Ne(r,[...n.childItemIds,...n.parentItemId?[n.parentItemId]:[]]);if(i.length>0)return i;const o=r.items.find(a=>a.id===n.parentItemId);return oF([...new Set(n.childItemIds)],n.parentItemId,o)?[]:[{message:"Yêu cầu gán parent transform không hợp lệ",code:"item.invalidTransformParent",path:"parentItemId"}]},apply(r,n){const i=[...new Set(n.childItemIds)],o=S.getState(),a=n.parentItemId?o.itemById[n.parentItemId]:void 0,{getItem:s,getKeyframes:l}=Kd(),c=qd({parentItemId:n.parentItemId,parent:a,behavior:n.behavior,frame:n.frame,canvas:n.canvas,getItem:s,getKeyframes:l}),u=jd(i,c);if(!u||u.size===0)return{inverse:V(),result:!1};for(const[d,f]of u)o._updateItem(d,{transformParent:f});return{inverse:V(),result:!0}}}),A({type:"item.createNullParent",params:y({childItemIds:ft,frame:dr,canvas:Bd}).strict(),validate:(r,n)=>{const i=Ne(r,n.childItemIds,"childItemIds");return i.length>0?i:n.childItemIds.every(o=>{const a=r.items.find(s=>s.id===o);return a!==void 0&&wc(a)})?[]:[{message:"Clip này không thể có parent transform",code:"item.invalidTransformParent",path:"childItemIds"}]},apply(r,n){const i=[...new Set(n.childItemIds)],o=S.getState(),a=i.map(p=>o.itemById[p]).filter(p=>p!==void 0);if(a.length!==i.length)return{inverse:V(),result:null};const{parent:s,track:l}=lF(r,{children:a,items:o.items,tracks:o.tracks,canvas:n.canvas}),{getItem:c,getKeyframes:u}=Kd(),d=qd({parentItemId:s.id,parent:s,frame:n.frame,canvas:n.canvas,getItem:p=>p===s.id?s:c(p),getKeyframes:u}),f=jd(i,d);if(!f||f.size!==i.length)return{inverse:V(),result:null};o.setTracks([...o.tracks.map(p=>(p.parentTrackId??null)===(l.parentTrackId??null)&&p.order>=l.order?{...p,order:p.order+1}:p),l]),o._addItem(s);for(const[p,m]of f)o._updateItem(p,{transformParent:m});return{inverse:V(),result:s}}}),A({type:"item.updateTransform",params:y({id:ue,transform:fo,operation:po.optional(),autoKeyframeOperations:F(ms).optional()}).strict(),validate:(r,n)=>[...Bn(r,[n.id],"id"),...ys(r,n.autoKeyframeOperations)],mergeKey:r=>`item.updateTransform:${r.id}:${gs(r.transform)}:${r.autoKeyframeOperations?.length?"kf":""}`,apply(r,n){return Object.keys(n.transform).length>0&&S.getState()._updateItemTransform(n.id,n.transform),hs(n.autoKeyframeOperations??[]),V()}}),A({type:"item.commitMaskEdit",params:y({id:ue,pathVertices:F(B_).optional(),transform:fo.optional(),operation:po.optional(),autoKeyframeOperations:F(ms).optional()}).strict(),validate:(r,n)=>[...Bn(r,[n.id],"id"),...ys(r,n.autoKeyframeOperations)],mergeKey:r=>`item.commitMaskEdit:${r.id}:${r.pathVertices?"path":""}:${gs(r.transform??{})}:${r.autoKeyframeOperations?.length?"kf":""}`,apply(r,n){const i=S.getState();return n.pathVertices&&i._updateItem(n.id,{pathVertices:n.pathVertices}),n.transform&&Object.keys(n.transform).length>0&&i._updateItemTransform(n.id,n.transform),hs(n.autoKeyframeOperations??[]),V()}}),A({type:"item.resetTransform",params:y({id:ue}).strict(),validate:(r,n)=>Bn(r,[n.id],"id"),apply(r,n){return S.getState()._resetItemTransform(n.id),V()}}),A({type:"item.updateTransforms",params:y({ids:ft,transform:fo,operation:po.optional()}).strict(),validate:(r,n)=>Bn(r,n.ids,"ids"),mergeKey:r=>`item.updateTransforms:${[...r.ids].sort().join(",")}:${gs(r.transform)}`,apply(r,n){return S.getState()._updateItemsTransform(n.ids,n.transform),V()}}),A({type:"item.updateTransformsMap",params:y({entries:F(y({id:ue,transform:fo}).strict()),itemUpdates:F(y({id:ue,...vs}).strict()).optional(),operation:po.optional(),autoKeyframeOperations:F(ms).optional()}).strict(),validate:(r,n)=>[...Ne(r,[...n.entries.map(i=>i.id),...(n.itemUpdates??[]).map(i=>i.id)],"entries"),...Bn(r,n.entries.map(i=>i.id),"entries"),...(n.itemUpdates??[]).flatMap(i=>bs(i.updates,i.clearFields)),...ys(r,n.autoKeyframeOperations)],mergeKey:r=>`item.updateTransformsMap:${[...r.entries.map(n=>n.id),...(r.itemUpdates??[]).map(n=>`u:${n.id}`)].sort().join(",")}:${r.autoKeyframeOperations?.length?"kf":""}`,apply(r,n){const i=new Map(n.entries.map(a=>[a.id,a.transform])),o=n.itemUpdates?.length?new Map(n.itemUpdates.map(a=>[a.id,Fr(a.updates,a.clearFields)])):void 0;return S.getState()._updateItemsTransformMap(i,o),hs(n.autoKeyframeOperations??[]),V()}}),A({type:"item.applyBentoLayout",params:y({itemIds:F(ue).min(2),canvasWidth:P().finite().positive(),canvasHeight:P().finite().positive(),config:U_.optional(),orderedChains:F(F(ue)).optional()}).strict(),validate:(r,n)=>[...Ne(r,n.itemIds),...n.itemIds.filter(i=>r.items.find(o=>o.id===i)?.type!=="audio").length>=2?[]:[{message:"Cần ít nhất 2 clip hình ảnh để xếp bố cục",code:"item.bentoNeedsVisuals",path:"itemIds"}]],apply(r,n){const i=S.getState().items,o=n.itemIds.filter(f=>{const p=i.find(m=>m.id===f);return p!=null&&p.type!=="audio"});if(o.length<2)return V();let a;if(n.orderedChains&&n.orderedChains.length>0){const f=new Set(o);a=n.orderedChains.map(p=>p.filter(m=>f.has(m))).filter(p=>p.length>0)}else{const{transitionsByClipId:f}=b_(q.getState().transitions);a=S_(o,f)}const s=a.map(f=>{const p=i.find(g=>g.id===f[0]),m=p&&"sourceWidth"in p&&p.sourceWidth||n.canvasWidth,h=p&&"sourceHeight"in p&&p.sourceHeight||n.canvasHeight;return{id:f[0],sourceWidth:m,sourceHeight:h}}),l=n.config??{preset:"auto"},c=T_(s,n.canvasWidth,n.canvasHeight,l),u=new Map;for(const f of a){const p=c.get(f[0]);if(p)for(const m of f)u.set(m,p)}const d=$.getState();for(const f of o)for(const p of cF)d._removeKeyframesForProperty(f,p);return S.getState()._updateItemsTransformMap(u),V()}}),A({type:"item.updateProjectItem",params:y({itemId:ue,...vs}).strict(),validate:(r,n)=>[...bs(n.updates,n.clearFields),...Zd(r,n.itemId,n.updates,n.clearFields),...Wd(r,n.itemId,n.updates,n.clearFields)],apply(r,n){const i=Fr(n.updates,n.clearFields),o=S.getState().itemById[n.itemId],a=dF(n.itemId,{...i,...o?Vd(o,i):{}});return{inverse:V(),result:a}}}),A({type:"item.removeProjectItems",params:y({itemIds:ft}).strict(),apply(r,n){const i=uF([...new Set(n.itemIds)]);return{inverse:V(),result:i}}})}const ya=P().finite(),bt=B().optional(),te=ya.optional(),vo=Wt([ne(6),ne(12),ne(18),ne(24)]).optional(),bo=U(["low-shelf","peaking","high-shelf","notch"]).optional(),mn=y({enabled:bt,outputGainDb:te,band1Enabled:bt,band1Type:U(["low-shelf","peaking","high-shelf","high-pass"]).optional(),band1FrequencyHz:te,band1GainDb:te,band1Q:te,band1SlopeDbPerOct:vo,lowCutEnabled:bt,lowCutFrequencyHz:te,lowCutSlopeDbPerOct:vo,lowEnabled:bt,lowType:bo,lowGainDb:te,lowFrequencyHz:te,lowQ:te,lowMidEnabled:bt,lowMidType:bo,lowMidGainDb:te,lowMidFrequencyHz:te,lowMidQ:te,midGainDb:te,highMidEnabled:bt,highMidType:bo,highMidGainDb:te,highMidFrequencyHz:te,highMidQ:te,highEnabled:bt,highType:bo,highGainDb:te,highFrequencyHz:te,highQ:te,band6Enabled:bt,band6Type:U(["low-pass","low-shelf","peaking","high-shelf"]).optional(),band6FrequencyHz:te,band6GainDb:te,band6Q:te,band6SlopeDbPerOct:vo,highCutEnabled:bt,highCutFrequencyHz:te,highCutSlopeDbPerOct:vo}).strict(),mF=Object.fromEntries(Object.entries(mn.shape).map(([e,t])=>[`audioEq${e[0].toUpperCase()}${e.slice(1)}`,t])),hF=y({...mF,volume:te,audioFadeIn:te,audioFadeOut:te,audioFadeInCurve:te,audioFadeOutCurve:te,audioFadeInCurveX:te,audioFadeOutCurveX:te,audioPitchSemitones:te,audioPitchCents:te,embeddedAudioMuted:bt,audioDucking:y({duckOthersDb:ya,attackSec:te,releaseSec:te,targetTrackIds:F(I().min(1)).optional()}).strict().nullable().optional()}).strict(),Vr=I().min(1),Qt=()=>({kind:"patch",patch:{}});function Yd(e,t){const r=e.items.find(n=>n.id===t.itemId);return r?r.type!=="audio"&&r.type!=="video"&&r.type!=="composition"?[{message:"Clip này không có âm thanh",path:"itemId"}]:[]:[{message:`Không tìm thấy clip ${t.itemId}`,path:"itemId"}]}function So(e,t){return e.tracks.some(r=>r.id===t.trackId)?[]:[{message:`Không tìm thấy rãnh ${t.trackId}`,path:"trackId"}]}function gF(e){return Object.fromEntries(Object.keys(mn.shape).map(t=>[`audioEq${t[0].toUpperCase()}${t.slice(1)}`,e?.[t]]))}function yF(){A({type:"audio.setClip",params:y({itemId:Vr,updates:hF}).strict(),validate:(e,t)=>Yd(e,t),mergeKey:e=>`audio:clip:${e.itemId}:${Object.keys(e.updates).sort().join(",")}`,apply(e,t){const r={...t.updates};return"audioDucking"in t.updates&&(r.audioDucking=t.updates.audioDucking??void 0),r.embeddedAudioMuted===!1&&(r.loop=void 0),e.draft.items=e.draft.items.map(n=>n.id===t.itemId?we({...n,...r}):n),Qt()}}),A({type:"audio.setClipEq",params:y({itemId:Vr,eq:mn.nullable()}).strict(),validate:(e,t)=>Yd(e,t),mergeKey:e=>`audio:clip-eq:${e.itemId}`,apply(e,t){const r=gF(t.eq);return e.draft.items=e.draft.items.map(n=>n.id===t.itemId?we({...n,...r}):n),Qt()}}),A({type:"audio.setTrackVolume",params:y({trackId:Vr,volume:ya}).strict(),validate:(e,t)=>So(e,t),mergeKey:e=>`audio:track-volume:${e.trackId}`,apply(e,t){return S.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,volume:t.volume}:r)),Qt()}}),A({type:"audio.setTrackMute",params:y({trackId:Vr,muted:B()}).strict(),validate:(e,t)=>So(e,t),apply(e,t){return S.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,muted:t.muted}:r)),Qt()}}),A({type:"audio.setTrackSolo",params:y({trackId:Vr,solo:B()}).strict(),validate:(e,t)=>So(e,t),apply(e,t){return S.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,solo:t.solo}:r)),Qt()}}),A({type:"audio.setTrackEq",params:y({trackId:Vr,eq:mn.nullable()}).strict(),validate:(e,t)=>So(e,t),mergeKey:e=>`audio:track-eq:${e.trackId}`,apply(e,t){return S.getState().setTracks(e.draft.tracks.map(r=>r.id===t.trackId?{...r,audioEq:t.eq??void 0}:r)),Qt()}}),A({type:"audio.setBusEq",params:y({eq:mn.nullable()}).strict(),mergeKey:()=>"audio:bus-eq",apply(e,t){return e.draft.busAudioEq=t.eq??void 0,Qt()}}),A({type:"audio.setMasterGain",params:y({db:ya}).strict(),mergeKey:()=>"audio:master-gain",apply(e,t){return e.draft.masterBusDb=Math.max(-60,Math.min(12,t.db)),Qt()}})}const or=I().min(1),vF=y({id:or,frame:P().int().nonnegative(),color:I(),label:I().optional()}),t0=zi({id:or,leftClipId:or,rightClipId:or,trackId:or,durationInFrames:P().positive()}),r0=zi({itemId:or,properties:F(Fa()).optional()}),bF=zi({id:or,name:I(),items:F(on),tracks:F(pn),transitions:F(t0),keyframes:F(r0),fps:P().positive(),width:P().positive(),height:P().positive(),durationInFrames:P().nonnegative()}),SF=y({items:F(on).optional(),tracks:F(pn).optional(),transitions:F(t0).optional(),keyframes:F(r0).optional(),markers:F(vF).optional(),compositions:F(bF).optional(),topLevelSequenceIds:F(or).optional(),inPoint:P().nonnegative().nullable().optional(),outPoint:P().nonnegative().nullable().optional(),fps:P().positive().max(240).optional(),scrollPosition:P().nonnegative().optional(),snapEnabled:B().optional(),currentFrame:P().int().nonnegative().optional(),busAudioEq:mn.optional(),masterBusDb:P().min(-60).max(12).optional()}).strict();function IF(){A({type:"project.setView",history:"none",dirty:!1,params:y({scrollPosition:P().nonnegative()}).strict(),apply({draft:e},t){return e.scrollPosition=t.scrollPosition,{kind:"patch",patch:{}}}}),A({type:"project.clearTimeline",history:"none",dirty:!1,params:y({}).strict(),apply({draft:e}){return e.items=[],e.tracks=[],e.transitions=[],e.keyframes=[],e.markers=[],e.inPoint=null,e.outPoint=null,{kind:"patch",patch:{}}}}),A({type:"project.rename",params:y({name:I().trim().min(1)}).strict(),apply({draft:e},t){return e.projectName=t.name,{kind:"patch",patch:{}}}}),A({type:"project.updateSettings",params:y({width:P().int().positive().optional(),height:P().int().positive().optional(),fps:P().positive().max(240).optional(),backgroundColor:I().optional()}).strict(),apply({draft:e},t){const r=e.projectMetadata,n={width:r?.width??1920,height:r?.height??1080,fps:r?.fps??e.fps,...r,...t};return e.projectMetadata=n,t.fps!==void 0&&(e.fps=t.fps),{kind:"patch",patch:{}}}}),A({type:"project.setTimelineSettings",params:y({snapEnabled:B().optional(),scrollPosition:P().nonnegative().optional()}).strict(),apply({draft:e},t){return t.snapEnabled!==void 0&&(e.snapEnabled=t.snapEnabled),t.scrollPosition!==void 0&&(e.scrollPosition=t.scrollPosition),{kind:"patch",patch:{}}}}),A({type:"project.patch",params:y({patch:SF}).strict(),apply({draft:e},{patch:t}){for(const[r,n]of Object.entries(t))Reflect.set(e,r,n);return{kind:"patch",patch:{}}}})}const Xd=y({name:I().optional(),height:P().positive().optional(),locked:B().optional(),visible:B().optional(),muted:B().optional(),solo:B().optional(),syncLock:B().optional(),parentTrackId:I().nullable().optional(),isGroup:B().optional(),isCollapsed:B().optional(),kind:U(["video","audio"]).optional(),order:P().optional(),templateSlot:Yg.nullable().optional(),templateLoudnessLufs:P().min(-24).max(-9).nullable().optional(),templateScenes:fg.nullable().optional(),templateBackdrop:pg.nullable().optional(),templateBackgroundAppearance:mg.nullable().optional()}).strict(),Qd={scenes:"Ảnh cảnh",captions:"Phụ đề","source-audio":"Âm thanh gốc","background-video":"Nền video"};function Ss(e,t,r,n,i="updates.templateSlot"){const o=[];(t==="source-audio"?r!=="audio":r==="audio")&&o.push({code:"track.slotKind",message:t==="source-audio"?"Vai trò Âm thanh gốc chỉ gán cho track âm thanh":`Vai trò ${Qd[t]} chỉ gán cho track video`,path:i});const a=e.find(s=>s.templateSlot===t&&s.id!==n);return a&&o.push({code:"track.slotTaken",message:`Vai trò ${Qd[t]} đã thuộc track "${a.name??a.id}"`,path:i}),o}function Is(e,t){return e==="scenes"?[]:[{code:"track.scenesOnly",message:"Công thức ảnh chỉ đặt trên track vai trò Ảnh cảnh",path:t}]}function xs(e,t,r){const n=[];e!=="background-video"&&n.push({code:"track.backdropOnly",message:"Kênh nền chỉ đặt trên track vai trò Nền video",path:r});const i=t.channels.map(o=>o.channelId);return new Set(i).size!==i.length&&n.push({code:"track.backdropDuplicate",message:"Một kênh nền chỉ được thêm một lần",path:r}),n}function xF(){A({type:"track.add",params:Xd.extend({id:I().optional(),beforeTrackId:I().nullable().optional()}).strict(),validate:(e,t)=>[...t.templateSlot==null?[]:Ss(e.tracks,t.templateSlot,t.kind,t.id,"templateSlot"),...t.templateScenes?Is(t.templateSlot??void 0,"templateScenes"):[],...t.templateBackdrop?xs(t.templateSlot??void 0,t.templateBackdrop,"templateBackdrop"):[],...t.templateBackgroundAppearance&&t.templateSlot!=="background-video"?[{code:"track.backgroundAppearanceOnly",message:"Cài đặt nền chỉ đặt trên track vai trò Nền video",path:"templateBackgroundAppearance"}]:[]],apply({draft:e,ids:t},r){const{id:n,beforeTrackId:i,parentTrackId:o,templateSlot:a,templateLoudnessLufs:s,templateScenes:l,templateBackdrop:c,templateBackgroundAppearance:u,...d}=r,f={...d,id:n??t.mint(),name:d.name??"Track mới",height:d.height??Pp(),locked:d.locked??!1,visible:d.visible??!0,muted:d.muted??!1,solo:d.solo??!1,order:d.order??0,items:[],...o?{parentTrackId:o}:{},...a?{templateSlot:a}:{},...s!=null?{templateLoudnessLufs:s}:{},...l?{templateScenes:l}:{},...c?{templateBackdrop:c}:{},...u?{templateBackgroundAppearance:u}:{}};if(e.tracks.some(g=>g.id===f.id))throw new Error(`Duplicate track: ${f.id}`);const p=i?e.tracks.findIndex(g=>g.id===i):0,m=Math.max(0,p),h=[...e.tracks];return h.splice(m,0,f),e.tracks=h.map((g,v)=>({...g,order:v})),{inverse:{kind:"patch",patch:{}},result:f}}}),A({type:"track.remove",params:y({ids:F(I()).min(1)}).strict(),apply({draft:e},t){const r=new Set(t.ids);let n=!0;for(;n;){n=!1;for(const o of e.tracks)o.parentTrackId&&r.has(o.parentTrackId)&&!r.has(o.id)&&(r.add(o.id),n=!0)}const i=new Set(e.items.filter(o=>r.has(o.trackId)).map(o=>o.id));return e.items=e.items.filter(o=>!i.has(o.id)),e.transitions=e.transitions.filter(o=>!i.has(o.leftClipId)&&!i.has(o.rightClipId)),e.keyframes=e.keyframes.filter(o=>!i.has(o.itemId)),e.tracks=e.tracks.filter(o=>!r.has(o.id)),{kind:"patch",patch:{}}}}),A({type:"track.update",params:y({id:I(),updates:Xd}).strict(),validate:(e,t)=>{const r=e.tracks.find(c=>c.id===t.id);if(!r)return[{message:`Unknown track: ${t.id}`}];const n=t.updates.templateSlot===null?void 0:t.updates.templateSlot??r.templateSlot,i=t.updates.kind??r.kind,o=n?Ss(e.tracks,n,i,t.id):[],a=t.updates.templateScenes===null?void 0:t.updates.templateScenes??(n==="scenes"?r.templateScenes:void 0);o.push(...a?Is(n,"updates.templateScenes"):[]);const s=t.updates.templateBackdrop===null?void 0:t.updates.templateBackdrop??(n==="background-video"?r.templateBackdrop:void 0);return o.push(...s?xs(n,s,"updates.templateBackdrop"):[]),t.updates.templateBackgroundAppearance!==null&&(t.updates.templateBackgroundAppearance??(n==="background-video"?r.templateBackgroundAppearance:void 0))&&n!=="background-video"&&o.push({code:"track.backgroundAppearanceOnly",message:"Cài đặt nền chỉ đặt trên track vai trò Nền video",path:"updates.templateBackgroundAppearance"}),o},mergeKey:e=>`track.update:${e.id}`,apply({draft:e},t){if(!e.tracks.some(n=>n.id===t.id))throw new Error(`Unknown track: ${t.id}`);if(t.updates.parentTrackId){let n=e.tracks.find(i=>i.id===t.updates.parentTrackId);if(!n?.isGroup)throw new Error("Parent track must be a group");for(;n;){if(n.id===t.id)throw new Error("Circular track group");n=e.tracks.find(i=>i.id===n?.parentTrackId)}}const r=t.updates.templateSlot;return e.tracks=e.tracks.map(n=>n.id===t.id?{...n,...t.updates,templateSlot:r===null?void 0:r??n.templateSlot,templateLoudnessLufs:t.updates.templateLoudnessLufs===null?void 0:t.updates.templateLoudnessLufs??n.templateLoudnessLufs,templateScenes:t.updates.templateScenes===null||r!==void 0&&r!=="scenes"?void 0:t.updates.templateScenes??n.templateScenes,templateBackdrop:t.updates.templateBackdrop===null||r!==void 0&&r!=="background-video"?void 0:t.updates.templateBackdrop??n.templateBackdrop,templateBackgroundAppearance:t.updates.templateBackgroundAppearance===null||r!==void 0&&r!=="background-video"?void 0:t.updates.templateBackgroundAppearance??n.templateBackgroundAppearance,parentTrackId:t.updates.parentTrackId===null?void 0:t.updates.parentTrackId??n.parentTrackId}:n),{kind:"patch",patch:{}}}}),A({type:"track.reorder",params:y({ids:F(I())}).strict(),apply({draft:e},t){if(new Set(t.ids).size!==e.tracks.length||t.ids.length!==e.tracks.length)throw new Error("Track order must contain every track once");return e.tracks=t.ids.map((r,n)=>{const i=e.tracks.find(o=>o.id===r);if(!i)throw new Error(`Unknown track: ${r}`);return{...i,order:n}}),{kind:"patch",patch:{}}}}),A({type:"track.set",params:y({tracks:F(pn)}).strict(),validate:(e,t)=>t.tracks.flatMap(r=>[...r.templateSlot?Ss(t.tracks,r.templateSlot,r.kind,r.id,"tracks.templateSlot"):[],...r.templateScenes?Is(r.templateSlot,"tracks.templateScenes"):[],...r.templateBackdrop?xs(r.templateSlot,r.templateBackdrop,"tracks.templateBackdrop"):[]]),apply({draft:e},t){return e.tracks=t.tracks.map(r=>({...r,items:[]})),{kind:"patch",patch:{}}}})}function wF(e){const t=Yf(e),r=t.some(n=>n.solo);return t.filter(n=>r?n.solo===!0:n.visible!==!1)}function kF(e){const t=wF(e.tracks),r=new Set(t.map(i=>i.id)),n=new Map(t.map(i=>[i.id,i.order??0]));return e.items.filter(i=>r.has(i.trackId)).toSorted((i,o)=>{const a=n.get(i.trackId)??0,s=n.get(o.trackId)??0;return a!==s?a-s:i.from!==o.from?i.from-o.from:i.id.localeCompare(o.id)})}function va(e){return e.type==="video"||e.type==="audio"?e.sourceStart??e.trimStart??e.offset??0:e.type==="composition"?e.sourceStart??e.trimStart??0:0}function PF(e){const{subItem:t,wrapper:r,parentFps:n,subCompFps:i}=e,o=va(r),a=Xf({itemStart:t.from,itemDuration:t.durationInFrames,wrapperDuration:r.durationInFrames,wrapperSpeed:r.speed,wrapperSourceFps:r.sourceFps,wrapperSourceStart:o,wrapperSourceEnd:r.sourceEnd,timelineFps:n,fallbackSourceFps:i});if(!a)return null;const s={...t,from:a.mappedFrom,durationInFrames:a.mappedDuration,speed:(t.speed??1)*a.wrapperSpeed};if(t.type==="video"||t.type==="audio"||t.type==="composition"){const l=t.sourceFps??i,c=t.speed??1,u=va(t)+De(a.clippedStartFrames,c,i,l);s.sourceStart=u,s.sourceFps=l,t.sourceEnd!==void 0&&(s.sourceEnd=Math.max(u+1,t.sourceEnd-De(a.clippedEndFrames,c,i,l)))}return s}function Jd(e){const{wrapper:t,parentFps:r,compositionById:n,activeCompositionPath:i,mediaFpsById:o}=e;if(!n)return[];if(i.has(t.compositionId))return[];const a=n[t.compositionId];if(!a)return[];const s=new Set(i);s.add(t.compositionId);const l=a.items.flatMap(c=>{const u=PF({subItem:c,wrapper:t,parentFps:r,subCompFps:a.fps});return u?[u]:[]});return n0({items:l,tracks:a.tracks,fps:r,mediaFpsById:o,compositionById:n,activeCompositionPath:s})}function n0(e){const t=kF(e),r=new Map(Yf(e.tracks).map(i=>[i.id,i])),n=e.activeCompositionPath??new Set;return t.flatMap(i=>r.get(i.trackId)?.muted?[]:i.type==="audio"&&i.compositionId?Jd({wrapper:i,parentFps:e.fps,compositionById:e.compositionById,activeCompositionPath:n,mediaFpsById:e.mediaFpsById}):i.type==="composition"?Gs(t,i)?[]:Jd({wrapper:i,parentFps:e.fps,compositionById:e.compositionById,activeCompositionPath:n,mediaFpsById:e.mediaFpsById}):i.mediaId?i.type==="audio"?[{itemId:i.id,mediaId:i.mediaId,from:i.from,durationInFrames:i.durationInFrames,sourceStart:va(i),sourceFps:i.sourceFps??e.mediaFpsById?.[i.mediaId]??e.fps,speed:i.speed??1}]:i.type==="video"&&!i.embeddedAudioMuted&&!Gs(t,i)?[{itemId:i.id,mediaId:i.mediaId,from:i.from,durationInFrames:i.durationInFrames,sourceStart:va(i),sourceFps:i.sourceFps??e.mediaFpsById?.[i.mediaId]??e.fps,speed:i.speed??1}]:[]:[])}function ba(e,t){return gr(e??{name:""})??(t.every(r=>r.type==="audio")?"audio":"video")}function TF(e){return e.some(t=>t.type!=="audio")}function i0(e){return{sourceStart:0,sourceEnd:e.durationInFrames,sourceDuration:e.durationInFrames,sourceFps:e.fps,speed:1}}function EF(e,t){const r=new Set(t);for(const n of e)if(r.has(n.id)){if(n.type==="composition"){const i=Rl(e,n);i&&r.add(i.id);continue}if(bn(n)){const i=np(e,n);i&&r.add(i.id)}}return Array.from(r)}function Io(e,t,r){const n=new Set([t]);let i=!1;const o=e.map(a=>!o0(a,n)||a.label===r?a:(i=!0,{...a,label:r}));return i?o:e}function o0(e,t){return!!e.compositionId&&(e.type==="composition"||bn(e))&&t.has(e.compositionId)}function ef(e,t){return e.filter(r=>o0(r,t)).length}function CF(e){const t=new Set(e.filter(Boolean));if(t.size===0)return{rootReferenceCount:0,nestedReferenceCount:0,totalReferenceCount:0};const r=Ba(),n=ef(VM(r).items,t),i=ZM(r).filter(o=>!t.has(o.id)).reduce((o,a)=>o+ef(a.items,t),0);return{rootReferenceCount:n,nestedReferenceCount:i,totalReferenceCount:n+i}}function MF(e,t){const r=t.trim();return!e||!r?!1:Ic("RENAME_COMPOUND_CLIP",()=>{const n=ae.getState().getComposition(e);if(!n||n.name===r)return!1;ae.getState().updateComposition(e,{name:r});const i=Ba(),o=Io(i.items,e,r);o!==i.items&&S.getState().setItems(o);const a=ye.getState().activeCompositionId,s=ae.getState().compositions.map(l=>{if(l.id===e)return l;const c=l.id===a?o:l.items,u=Io(c,e,r);return u===c?l:{...l,items:u}});return ae.getState().setCompositions(s),ye.setState(l=>({breadcrumbs:l.breadcrumbs.map(c=>c.compositionId===e?{...c,label:r}:c),stashStack:l.stashStack.map(c=>({...c,items:Io(c.items,e,r)})),mainHolder:l.mainHolder?{...l.mainHolder,items:Io(l.mainHolder.items,e,r)}:null})),!0},{compositionId:e,nextName:r})}function tf(e){const{subTracks:t,subItems:r,anchorTrackId:n,kind:i,compFrom:o,existingTracks:a,currentItems:s,trackIdMapping:l,newTracks:c}=e;if(t.length===0||!n)return;const u=[...t].sort((h,g)=>(h.order??0)-(g.order??0)),d=[...a,...c].find(h=>h.id===n);if(!d)return;const f=new Set(l.values()),p=u.length-1;l.set(u[p].id,n),f.add(n);const m=[...a,...c].filter(h=>h.id!==n&&gr(h)===i).filter(h=>(h.order??0)<(d.order??0)).sort((h,g)=>(g.order??0)-(h.order??0));for(let h=p-1;h>=0;h-=1){const g=u[h],v=r.filter(E=>E.trackId===g.id).map(E=>({from:E.from+o,end:E.from+o+E.durationInFrames}));let x=null;for(const E of m){if(f.has(E.id))continue;if(!s.filter(O=>O.trackId===E.id).some(O=>{const C=O.from+O.durationInFrames;return v.some(R=>R.from<C&&O.from<R.end)})){x=E.id,f.add(E.id);break}}if(x){l.set(g.id,x);continue}const b=p-h,w=Z();l.set(g.id,w),f.add(w),c.push({...g,id:w,kind:i,order:(d.order??0)-b*.01})}}function _F(e){const{subItem:t,wrapper:r,timelineFps:n,subCompFps:i}=e,o=r.sourceStart??r.trimStart??0,a=Xf({itemStart:t.from,itemDuration:t.durationInFrames,wrapperDuration:r.durationInFrames,wrapperSpeed:r.speed,wrapperSourceFps:r.sourceFps,wrapperSourceStart:o,wrapperSourceEnd:r.sourceEnd,timelineFps:n,fallbackSourceFps:i});if(!a)return null;const s=r.from+a.mappedFrom,l={...t,from:s,durationInFrames:a.mappedDuration,speed:(t.speed??1)*a.wrapperSpeed};if(t.type==="video"||t.type==="audio"||t.type==="composition"){const c=t.sourceFps??i,u=t.speed??1,d=(t.sourceStart??0)+De(a.clippedStartFrames,u,i,c);l.sourceStart=d,t.sourceEnd!==void 0&&(l.sourceEnd=Math.max(d+1,t.sourceEnd-De(a.clippedEndFrames,u,i,c)))}return l}function FF(e,t,r){const n=r?Ci(e,t):Array.from(new Set(t));return EF(e,n)}function AF(e,t,r){return t===null?!1:Ll(e).some(n=>lp({parentCompositionId:t,insertedCompositionId:n,compositionById:r}))}function OF(e){const{items:t,tracks:r}=S.getState(),n=q.getState().transitions,i=$.getState().keyframes,o=W.getState().fps,a=e??yt.getState().selectedItemIds,s=FF(t,a,Ul.getState().linkedSelectionEnabled);if(s.length===0)return null;const l=new Set(s),c=t.filter(f=>l.has(f.id));if(c.length===0)return null;const u=ye.getState().activeCompositionId,d=ae.getState().compositionById;return AF(c,u,d)?null:{items:t,tracks:r,transitions:n,keyframes:i,fps:o,selectedIds:s,selectedItems:c,activeCompositionId:u,compositionById:d}}function zF(e){const t=Math.min(...e.map(n=>n.from)),r=Math.max(...e.map(n=>n.from+n.durationInFrames));return{minFrom:t,durationInFrames:r-t}}function DF(){const e=Ht.getState().currentProject?.metadata;return{width:e?.width??ep,height:e?.height??tp,backgroundColor:e?.backgroundColor}}function RF(e,t,r,n){const i=r??{name:`Track ${t+1}`,kind:ba(void 0,n),height:jr,visible:!0,muted:!1,solo:!1,volume:0};return{id:Z(),name:i.name,kind:ba(r,n),height:i.height,locked:!1,visible:i.visible,muted:i.muted,solo:i.solo,volume:i.volume,color:i.color,order:t,items:[]}}function LF(e,t){const r=new Map(t.map(a=>[a.id,a])),n=[...new Set(e.map(a=>a.trackId))].sort((a,s)=>(r.get(a)?.order??0)-(r.get(s)?.order??0)),i=n.map((a,s)=>RF(a,s,r.get(a),e.filter(l=>l.trackId===a))),o=new Map;return n.forEach((a,s)=>{o.set(a,i[s].id)}),{sourceTrackIds:n,subCompTracks:i,trackIdMapping:o}}function BF(e,t,r,n){const i=e.map(a=>({...a,id:Z(),from:a.from-t,trackId:r.get(a.trackId)??n})),o=new Map;return e.forEach((a,s)=>o.set(a.id,i[s].id)),{items:i,itemIdMapping:o}}function UF(e,t,r,n){return e.filter(i=>t.has(i.leftClipId)&&t.has(i.rightClipId)).map(i=>({...i,id:Z(),leftClipId:r.get(i.leftClipId)??i.leftClipId,rightClipId:r.get(i.rightClipId)??i.rightClipId,trackId:n.get(i.trackId)??i.trackId}))}function NF(e,t,r){return e.filter(n=>t.has(n.itemId)).map(n=>({...n,itemId:r.get(n.itemId)??n.itemId}))}function $F(e,t){return e.editorKind?e.editorKind:t===null?"sequence":ae.getState().getComposition(t)?.editorKind==="composite-2d"?"composite-2d":"sequence"}function HF(e,t,r){const{minFrom:n,durationInFrames:i}=zF(e.selectedItems),o=DF(),{sourceTrackIds:a,subCompTracks:s,trackIdMapping:l}=LF(e.selectedItems,e.tracks),{items:c,itemIdMapping:u}=BF(e.selectedItems,n,l,s[0].id),d=new Set(e.selectedIds);return{composition:{id:Z(),name:t??`Compound Clip ${ae.getState().compositions.length+1}`,editorKind:$F(r,e.activeCompositionId),items:c,tracks:s,transitions:UF(e.transitions,d,u,l),keyframes:NF(e.keyframes,d,u),fps:e.fps,width:o.width,height:o.height,durationInFrames:i,backgroundColor:o.backgroundColor},minFrom:n,durationInFrames:i,selectedItemIds:d,sourceTrackIds:a}}function rf(e,t,r){return e.filter(n=>t.some(i=>r==="audio"?i.trackId===n&&i.type==="audio":i.trackId===n&&i.type!=="audio"))}function jF(e,t){return t?e.find(r=>r.id===t)??null:null}function qF(e,t){return t?hp({tracks:e,targetTrack:t,kind:"audio",direction:"below"}):e.filter(r=>!r.isGroup).filter(r=>gr(r)==="audio").sort((r,n)=>(r.order??0)-(n.order??0)).at(-1)??null}function KF(e,t){const r=t??e.at(-1)??null,n=r?pp(e,r,"below"):0;return mp({tracks:e,kind:"audio",order:n,height:r?.height??jr})}function GF(e){if(!e.hasOwnedAudio)return{tracks:e.tracks,audioTargetTrackId:null};const t=e.audioSourceTrackIds.at(-1)??null;if(t)return{tracks:e.tracks,audioTargetTrackId:t};const r=jF(e.tracks,e.visualTargetTrackId),n=qF(e.tracks,r);if(n)return{tracks:e.tracks,audioTargetTrackId:n.id};const i=KF(e.tracks,r);return{tracks:[...e.tracks,i],audioTargetTrackId:i.id}}function WF(e,t){const r=TF(t.composition.items),n=n0({items:t.composition.items,tracks:t.composition.tracks,fps:e.fps,compositionById:e.compositionById}).length>0,i=rf(t.sourceTrackIds,e.selectedItems,"visual"),o=rf(t.sourceTrackIds,e.selectedItems,"audio"),a=r?i.at(-1)??null:null,s=GF({tracks:e.tracks,hasOwnedAudio:n,audioSourceTrackIds:o,visualTargetTrackId:a});return{hasVisualWrapper:r,hasOwnedAudio:n,visualTargetTrackId:a,audioTargetTrackId:s.audioTargetTrackId,tracks:s.tracks}}function VF(e,t){S.getState()._removeItems(e.selectedIds),q.getState().setTransitions(e.transitions.filter(r=>!t.selectedItemIds.has(r.leftClipId)&&!t.selectedItemIds.has(r.rightClipId))),$.getState()._removeKeyframesForItems(e.selectedIds)}function ZF(e,t,r,n){return!t||!r?null:{id:Z(),type:"composition",trackId:r,from:e.minFrom,durationInFrames:e.durationInFrames,label:e.composition.name,compositionId:e.composition.id,linkedGroupId:n,compositionWidth:e.composition.width,compositionHeight:e.composition.height,transform:{x:0,y:0,rotation:0,opacity:1},...i0(e.composition)}}function YF(e,t,r,n){return!t||!r?null:{id:Z(),type:"audio",trackId:r,from:e.minFrom,durationInFrames:e.durationInFrames,label:e.composition.name,compositionId:e.composition.id,linkedGroupId:n,src:"",...i0(e.composition)}}function XF(e){return e.hasVisualWrapper&&e.hasOwnedAudio?Z():void 0}function QF(e,t){const r=[e,t].filter(n=>n!==null);for(const n of r)S.getState()._addItem(n);return r.length>0&&yt.getState().selectItems(r.map(n=>n.id)),r[0]??null}function JF(e,t){const r=XF(t),n=ZF(e,t.hasVisualWrapper,t.visualTargetTrackId,r),i=YF(e,t.hasOwnedAudio,t.audioTargetTrackId,r);return QF(n,i)}function e3(e,t,r){const n=OF(t);if(!n)return null;const i=HF(n,e,r);ae.getState().addComposition(i.composition);const o=WF(n,i);return VF(n,i),o.tracks!==n.tracks&&S.getState().setTracks(o.tracks),JF(i,o)}function t3(e,t,r={}){const n=Ic("CREATE_PRE_COMP",()=>e3(e,t,r),{name:e,editorKind:r.editorKind});return r.openAfterCreate&&n?.compositionId&&n3(n.compositionId,n.label,n.id),n}function r3(e){return Ic("DISSOLVE_PRE_COMP",()=>{const{items:t}=S.getState(),r=t.find(M=>M.id===e&&(M.type==="composition"||bn(M)));if(!r)return!1;const n=r.compositionId;if(!n)return!1;const i=bn(r),o=r.type==="composition"?r:i?np(t,r):null,a=r.type==="composition"?Rl(t,r):i?r:null,s=ae.getState().getComposition(n);if(!s)return!1;const{tracks:l}=S.getState(),c=[o?.id,a?.id].filter(M=>!!M),u=o??a;if(!u)return!1;const d=u.from,f=W.getState().fps;let p=l;const m=(M,D,L,H)=>{if(M)return M;if(!D)return null;const J=p.find(At=>At.id===D)??null;if(!J)return null;const fe=hp({tracks:p,targetTrack:J,kind:L,direction:H});if(fe)return fe.id;const se=mp({tracks:p,kind:L,order:pp(p,J,H),height:J.height??jr});return p=[...p,se],se.id},h=[...s.tracks].sort((M,D)=>(M.order??0)-(D.order??0)),g=h.filter(M=>{const D=s.items.filter(L=>L.trackId===M.id);return ba(M,D)==="video"}),v=h.filter(M=>{const D=s.items.filter(L=>L.trackId===M.id);return ba(M,D)==="audio"}),x=m(o?.trackId??null,a?.trackId??null,"video","above"),b=m(a?.trackId??null,o?.trackId??null,"audio","below"),w=new Map,E=[],k=t.filter(M=>!c.includes(M.id));tf({subTracks:g,subItems:s.items,anchorTrackId:x,kind:"video",compFrom:d,existingTracks:p,currentItems:k,trackIdMapping:w,newTracks:E}),tf({subTracks:v,subItems:s.items,anchorTrackId:b,kind:"audio",compFrom:d,existingTracks:p,currentItems:k,trackIdMapping:w,newTracks:E}),E.length>0&&(p=[...p,...E],S.getState().setTracks(p));const T=new Map,O=s.items.flatMap(M=>{const D=_F({subItem:M,wrapper:u,timelineFps:f,subCompFps:s.fps});if(!D)return[];const L=Z();return T.set(M.id,L),[{...D,id:L,trackId:w.get(M.trackId)??x??b??M.trackId}]});c.length>0&&S.getState()._removeItems(c),O.length>0&&S.getState()._addItems(O),pe(c,new Set(c));const C=s.transitions??[];if(C.length>0){const M=q.getState().transitions,D=C.flatMap(L=>{const H=T.get(L.leftClipId),J=T.get(L.rightClipId);return!H||!J?[]:[{...L,id:Z(),leftClipId:H,rightClipId:J,trackId:w.get(L.trackId)??L.trackId}]});q.getState().setTransitions([...M,...D]),D.length>0&&pe(O.map(L=>L.id))}const R=s.keyframes??[];if(R.length>0){const M=$.getState().keyframes,D=R.map(L=>({...L,itemId:T.get(L.itemId)??L.itemId}));$.getState().setKeyframes([...M,...D])}return CF([n]).totalReferenceCount===0&&ae.getState().removeComposition(n),yt.getState().selectItems(O.map(M=>M.id)),!0},{compositionItemId:e})}function n3(e,t,r){const n=qr([{type:"composition.enter",params:{compositionId:e,...t===void 0?{}:{label:t},...r===void 0?{}:{entryItemId:r}}}]);if(!n.ok)throw new Error(n.issues.map(i=>i.message).join("; "));typeof window<"u"&&ae.getState().getComposition(e)?.editorKind==="composite-2d"&&Ul.getState().setWorkspace("motion")}function i3(e){const t=ae.getState().getComposition(e.compositionId);if(t?.editorKind!=="composite-2d")return 0;const r=new Set(e.editorialItemIds),n=new Set(t.items.filter(p=>r.has(p.id)).map(p=>p.id));if(n.size===0)return 0;const i=t.items.filter(p=>!n.has(p.id)),o=new Map(t.tracks.map(p=>[p.id,p])),a=new Set(i.map(p=>p.trackId));for(const p of[...a]){let m=o.get(p)?.parentTrackId;for(;m&&!a.has(m);)a.add(m),m=o.get(m)?.parentTrackId}const s=t.tracks.filter(p=>a.has(p.id)),l=t.transitions.filter(p=>!n.has(p.leftClipId)&&!n.has(p.rightClipId)),c=t.keyframes.filter(p=>!n.has(p.itemId)),u=i.reduce((p,m)=>Math.max(p,m.from+m.durationInFrames),1),d=e.suggestedDurationInFrames,f=typeof d=="number"&&Number.isFinite(d)?Math.max(u,Math.round(d),1):t.durationInFrames;return ae.getState().updateComposition(e.compositionId,{items:i,tracks:s,transitions:l,keyframes:c,durationInFrames:f}),ye.getState().activeCompositionId===e.compositionId&&(S.getState().setItems(i),S.getState().setTracks(s),q.getState().setTransitions(l),$.getState().setKeyframes(c)),n.size}function o3(){A({type:"composition.create",params:y({name:I().optional(),itemIds:F(I()).min(1),openAfterCreate:B().optional(),editorKind:U(["sequence","composite-2d"]).optional()}).strict(),apply({draft:e},t){for(const n of t.itemIds)if(!e.items.some(i=>i.id===n))throw new Error(`Unknown item: ${n}`);const r=t3(t.name,t.itemIds,{openAfterCreate:!1,editorKind:t.editorKind});if(!r)throw new Error("Unable to create composition from selection");return t.openAfterCreate&&r.compositionId&&(t.editorKind==="composite-2d"?e.nav.switchToSequence(r.compositionId):e.nav.enterComposition(r.compositionId,t.name??r.label,r.id)),{inverse:{kind:"patch",patch:{}},result:r}}}),A({type:"composition.dissolve",params:y({itemId:I()}).strict(),apply(e,t){if(!r3(t.itemId))throw new Error(`Unable to dissolve composition: ${t.itemId}`);return{kind:"patch",patch:{}}}}),A({type:"composition.rename",params:y({compositionId:I(),name:I().trim().min(1)}).strict(),apply(e,t){if(!MF(t.compositionId,t.name))throw new Error(`Unknown composition: ${t.compositionId}`);return{kind:"patch",patch:{}}}}),A({type:"composition.updateSettings",params:y({compositionId:I(),width:P().int().positive().optional(),height:P().int().positive().optional(),fps:P().positive().max(240).optional(),durationInFrames:P().int().positive().optional(),backgroundColor:I().optional()}).strict(),apply({draft:e},t){if(!e.compositions.some(i=>i.id===t.compositionId))throw new Error(`Unknown composition: ${t.compositionId}`);const{compositionId:r,...n}=t;return e.compositions=e.compositions.map(i=>i.id===r?{...i,...n}:i),{kind:"patch",patch:{}}}}),A({type:"composition.enter",history:"none",params:y({compositionId:I(),label:I().optional(),entryItemId:I().optional()}).strict(),apply({draft:e},t){const r=e.compositions.find(n=>n.id===t.compositionId);if(!r)throw new Error(`Unknown composition: ${t.compositionId}`);return r.editorKind==="composite-2d"||e.topLevelSequenceIds.includes(r.id)?e.nav.switchToSequence(r.id):e.nav.enterComposition(r.id,t.label??r.name,t.entryItemId),{kind:"commands",commands:[{type:"composition.exit",params:{}}]}}}),A({type:"composition.exit",history:"none",params:y({}).strict(),apply({draft:e}){const t=e.nav.activeCompositionId;return e.nav.exitComposition(),{kind:"commands",commands:t?[{type:"composition.enter",params:{compositionId:t}}]:[]}}}),A({type:"composition.openTab",history:"none",params:y({compositionId:I()}).strict(),apply({draft:e},t){const r=e.compositions.find(n=>n.id===t.compositionId);if(!r||r.editorKind==="composite-2d")throw new Error("Sequence tab requires an editorial composition");return e.topLevelSequenceIds.includes(t.compositionId)||(e.topLevelSequenceIds=[...e.topLevelSequenceIds,t.compositionId]),e.nav.switchToSequence(t.compositionId),{kind:"commands",commands:[{type:"composition.closeTab",params:t}]}}}),A({type:"composition.closeTab",history:"none",params:y({compositionId:I()}).strict(),apply({draft:e},t){return e.nav.breadcrumbs[0]?.compositionId===t.compositionId&&e.nav.switchToSequence(null),e.topLevelSequenceIds=e.topLevelSequenceIds.filter(r=>r!==t.compositionId),{kind:"commands",commands:[{type:"composition.openTab",params:t}]}}}),A({type:"composition.repairEditorialLeak",params:y({compositionId:I(),editorialItemIds:F(I()),suggestedDurationInFrames:P().optional()}).strict(),apply(e,t){const r=i3(t);return{inverse:{kind:"patch",patch:{}},result:r}}}),A({type:"composition.switchSequence",history:"none",params:y({compositionId:I().nullable()}).strict(),apply(e,t){return ye.getState().switchToSequence(t.compositionId),{kind:"patch",patch:{}}}})}function ws(e,t){const r=Dl();if(r!==null){const n=ae.getState().getComposition(r);if(n)return $y(n.durationInFrames)}return Tp(S.getState().items,W.getState().fps)}const nf=y({frame:P().int().nonnegative(),color:I().default("#3B82F6"),label:I().default("")});function a3(){A({type:"marker.add",params:nf.extend({id:I().optional()}).strict(),apply({draft:e,ids:t},r){const n={...r,id:r.id??t.mint()};if(e.markers.some(i=>i.id===n.id))throw new Error(`Duplicate marker: ${n.id}`);return e.markers=[...e.markers,n],{inverse:{kind:"commands",commands:[{type:"marker.remove",params:{id:n.id}}]},result:n.id}}}),A({type:"marker.update",params:y({id:I(),updates:nf.partial()}).strict(),mergeKey:e=>`marker.update:${e.id}`,apply({draft:e},t){if(!e.markers.find(n=>n.id===t.id))throw new Error(`Unknown marker: ${t.id}`);return e.markers=e.markers.map(n=>n.id===t.id?{...n,...t.updates}:n),{kind:"patch",patch:{}}}}),A({type:"marker.remove",params:y({id:I()}).strict(),apply({draft:e},t){if(!e.markers.some(r=>r.id===t.id))throw new Error(`Unknown marker: ${t.id}`);return e.markers=e.markers.filter(r=>r.id!==t.id),{kind:"patch",patch:{}}}}),A({type:"marker.clear",params:y({}).strict(),apply({draft:e}){return e.markers=[],{kind:"patch",patch:{}}}}),A({type:"marker.setInOut",params:y({inPoint:P().int().nonnegative().nullable(),outPoint:P().int().nonnegative().nullable()}).strict(),apply({draft:e},t){const r=ws(),n=Ep({...t,maxFrame:r});return e.inPoint=n.inPoint,e.outPoint=n.outPoint,{kind:"patch",patch:{}}}}),A({type:"marker.setIn",params:y({frame:P().finite()}).strict(),apply({draft:e},t){const r=ws(),n=Math.max(0,Math.min(t.frame,r));return(e.outPoint===null||n>=e.outPoint)&&(e.outPoint=r),e.inPoint=n,{kind:"patch",patch:{}}}}),A({type:"marker.setOut",params:y({frame:P().finite()}).strict(),apply({draft:e},t){const r=ws(),n=Math.max(1,Math.min(t.frame,r));return(e.inPoint===null||n<=e.inPoint)&&(e.inPoint=0),e.outPoint=n,{kind:"patch",patch:{}}}})}function s3(e,t,r,n){if(e===0)return 0;const i=new Map;for(const c of t)for(const u of[c.item,c.leftNeighbor,c.rightNeighbor]){if(!u||i.has(u.id))continue;const d=n[u.id];if(!d)continue;const f=d.properties.flatMap(p=>p.keyframes).map(p=>p.frame).filter(p=>p>=0&&p<u.durationInFrames&&!Pi(p,u.id,u,r));f.length>0&&i.set(u.id,{item:u,frames:f})}if(i.size===0)return e;const o=c=>{const u=new Map;for(const d of t)u.set(d.item.id,{...d.item,from:d.item.from+c}),d.leftNeighbor&&u.set(d.leftNeighbor.id,{...d.leftNeighbor,durationInFrames:d.leftNeighbor.durationInFrames+c}),d.rightNeighbor&&u.set(d.rightNeighbor.id,{...d.rightNeighbor,from:d.rightNeighbor.from+c,durationInFrames:d.rightNeighbor.durationInFrames-c});for(const[d,f]of i){const p=u.get(d)??f.item;for(const m of f.frames)if(m>=p.durationInFrames||Pi(m,d,p,r))return!1}return!0};if(o(e))return e;const a=e<0?-1:1;let s=0,l=Math.abs(e);for(;s<l;){const c=Math.ceil((s+l)/2);o(a*c)?s=c:l=c-1}return a*s}function Tl(e,t){return{...e,...t}}function a0(e,t){if(!t(0))return 0;if(t(e))return e;const r=e<0?-1:1;let n=0,i=Math.abs(e);for(;n<i;){const o=Math.ceil((n+i)/2);t(r*o)?n=o:i=o-1}return r*n}function l3(e,t,r,n){const i=new Map;for(const o of e){const a=t.get(o),s=n[o];if(!a||!s)continue;const l=s.properties.flatMap(c=>c.keyframes).map(c=>c.frame).filter(c=>c>=0&&c<a.durationInFrames&&!Pi(c,o,a,r));l.length>0&&i.set(o,l)}return i}function s0({items:e,transitions:t,affectedIds:r,keyframesByItemId:n,timelineFps:i,preserveKeyframes:o,buildPreview:a}){const s=new Map(e.map(u=>[u.id,u])),l=t.filter(u=>r.has(u.leftClipId)||r.has(u.rightClipId)),c=o?l3(r,s,t,n):new Map;return u=>{const d=a(u);for(const f of l){const p=d.get(f.leftClipId)??s.get(f.leftClipId),m=d.get(f.rightClipId)??s.get(f.rightClipId);if(p&&m&&!Ui(p,m,f.durationInFrames,f.alignment,i).canAdd)return!1}for(const[f,p]of c){const m=d.get(f)??s.get(f);if(m){for(const h of p)if(h>=m.durationInFrames||Pi(h,f,m,t))return!1}}return!0}}function of(e,t,r,n,i,o,a,s=30,l=!0){if(r===0)return 0;const c=e,u=n,d=new Set([c.id,u.id]),f=s0({items:i,transitions:o,affectedIds:d,keyframesByItemId:a,timelineFps:s,preserveKeyframes:l,buildPreview:p=>new Map([[c.id,Tl(c,yc(c,p,s))],[u.id,Tl(u,gc(u,p,s))]])});return a0(r,f)}function c3(e,t,r,n,i,o,a=30,s=new Set,l=!0){if(r===0)return 0;const c=e.from+e.durationInFrames,u=new Set(n.filter(p=>p.id!==e.id&&!s.has(p.id)&&p.trackId===e.trackId&&p.from>=c).map(p=>p.id));for(const p of i)p.leftClipId===e.id&&!s.has(p.rightClipId)&&u.add(p.rightClipId);const d=new Set([e.id,...u]),f=s0({items:n,transitions:i,affectedIds:d,keyframesByItemId:o,timelineFps:a,preserveKeyframes:l,buildPreview:p=>{const m=new Map,h=t==="end"?yc(e,p,a):{...gc(e,p,a),from:e.from};m.set(e.id,Tl(e,h));const g=t==="end"?p:-p;for(const v of u){const x=n.find(b=>b.id===v);x&&m.set(v,{...x,from:x.from+g})}return m}});return a0(r,f)}function Pc(e,t){if(!e||e.length!==t.length)return!1;for(let r=0;r<t.length;r++)if(e[r]!==t[r])return!1;return!0}function u3(e,t){const r={};for(const i of e)(r[i.trackId]??=[]).push(i);const n={};for(const[i,o]of Object.entries(r)){const a=t[i];n[i]=a&&Pc(a,o)?a:o}return n}function d3(e,t){const r={};for(const i of e)i.linkedGroupId&&(r[i.linkedGroupId]??=[]).push(i);const n={};for(const[i,o]of Object.entries(r)){const a=t[i];n[i]=a&&Pc(a,o)?a:o}return n}function f3(e){return(e.type==="audio"||e.type==="video")&&typeof e.mediaId=="string"&&e.mediaId.length>0}function p3(e,t){return e.type==="video"&&t.type==="audio"||e.type==="audio"&&t.type==="video"}function m3(e){return!e.linkedGroupId&&f3(e)&&typeof e.originId=="string"&&e.originId.length>0}function h3(e,t){return!p3(e,t)||!e.originId||e.originId!==t.originId||!e.mediaId||e.mediaId!==t.mediaId?!1:e.from===t.from&&e.durationInFrames===t.durationInFrames}function g3(e,t,r){const n={};for(const o of Object.values(t))if(!(o.length<=1))for(const a of o)n[a.id]=o;const i={};for(const o of e){if(!m3(o))continue;const a=`${o.originId}|${o.mediaId}|${o.from}|${o.durationInFrames}`;(i[a]??=[]).push(o)}for(const o of Object.values(i))if(!(o.length<=1))for(const a of o){const s=o.filter(c=>c.id===a.id||h3(a,c));if(s.length<=1)continue;const l=r[a.id];n[a.id]=l&&Pc(l,s)?l:s}return n}function y3(e,t){const r={};for(const n of e){const i=t[n.id];r[n.id]=i!==void 0&&i===n?i:n}return r}function v3(e){const t=new Set;for(const r of e)r.mediaId&&t.add(r.mediaId);return[...t].sort()}function l0(e){return e.join("|")}function b3(e){let t=0;for(const r of e){const n=r.from+r.durationInFrames;n>t&&(t=n)}return t}function xe(e,t){const r=d3(e,t.itemsByLinkedGroupId);return{items:e,itemsByTrackId:u3(e,t.itemsByTrackId),itemById:y3(e,t.itemById),itemsByLinkedGroupId:r,linkedItemsByItemId:g3(e,r,t.linkedItemsByItemId),maxItemEndFrame:b3(e)}}function El(e){const t=q.getState().transitions,r=new Set;for(const n of t)n.leftClipId===e&&r.add(n.rightClipId),n.rightClipId===e&&r.add(n.leftClipId);return r}function S3(e,t){const r=new Map;for(const i of e){let o=0;for(const a of t)a.trackId===i.trackId&&a.from+a.durationInFrames<=i.from&&(o+=a.durationInFrames);r.set(i.id,o)}const n=new Set;for(const i of e){if(n.has(i.id))continue;const o=$i(e,i.id);for(const s of o)n.add(s.id);if(o.length<=1)continue;let a=0;for(const s of o)a=Math.max(a,r.get(s.id)??0);if(!(a<=0))for(const s of o)r.set(s.id,a)}return r}function Ar(e,t){return e<0?Math.max(e,t):Math.min(e,t)}function I3(e,t,r,n){const i=[t,...(r??[]).filter(a=>a!==t)],o=new Map;if(r!==void 0){const a=new Map(e.map(s=>[s.id,s]));for(const s of i){const l=a.get(s);l&&o.set(l.id,l)}return Array.from(o.values())}for(const a of i)for(const s of An(e,a,n))o.set(s.id,s);return Array.from(o.values())}function af(e,t,r,n,i,o){let a=Math.max(e,-t.from);const s=new Set([t.id,r?.id,n?.id].filter(Boolean));if(r){const l=yi(r,"end",a,o).clampedAmount;a=Ar(a,l),a=Ar(a,vi(r,"end",a,i,s))}if(n){const l=yi(n,"start",a,o).clampedAmount;a=Ar(a,l),a=Ar(a,vi(n,"start",a,i,s))}for(const l of i){if(l.trackId!==t.trackId||s.has(l.id))continue;const c=l.from+l.durationInFrames;c<=t.from&&a<0&&(a=Math.max(a,c-t.from));const u=t.from+t.durationInFrames;l.from>=u&&a>0&&(a=Math.min(a,l.from-u))}return a}function x3(e){const t=S.getState(),r=t.items,n=[],i=new Set;for(const o of e){const a=t.itemById[o];if(!a||a.type==="text")continue;const s=a.from,l=a.from+a.durationInFrames;for(const c of xc(r,o)){const u=t.itemById[c];if(!u||u.type!=="text")continue;const d=u.from,f=u.from+u.durationInFrames,p=Math.max(d,s),m=Math.min(f,l);if(m<=p){i.add(u.id);continue}const h=m-p;(p!==u.from||h!==u.durationInFrames)&&n.push({id:u.id,from:p,durationInFrames:h,words:u.words?vn(u.words,p-d,m-d):void 0})}}if(i.size>0){const o=Array.from(i);t._removeItems(o),$.getState()._removeKeyframesForItems(o)}for(const o of n)i.has(o.id)||t._updateItem(o.id,{from:o.from,durationInFrames:o.durationInFrames,...o.words?{words:o.words}:{}});return[...n.map(o=>o.id),...i]}function w3(e,t,r,n,i){const o=S.getState(),a=o.items,s=I3(a,e,n,i),l=s.find(g=>g.id===e);if(!l)return null;const c=W.getState().fps;let u=r;for(const g of s){const v=yi(g,t,u,c).clampedAmount;u=Ar(u,vi(g,t,v,a,El(g.id)))}t==="start"?o._trimItemStart(e,u,{skipAdjacentClamp:!0}):o._trimItemEnd(e,u,{skipAdjacentClamp:!0});const d=S.getState().itemById[e],f=t==="start"?d?d.from-l.from:0:d?d.durationInFrames-l.durationInFrames:0;if(f!==0)for(const g of s)g.id!==e&&(t==="start"?o._trimItemStart(g.id,f,{skipAdjacentClamp:!0}):o._trimItemEnd(g.id,f,{skipAdjacentClamp:!0}));const m=(t==="start"?f>0:f<0)?x3(s.map(g=>g.id)):[],h=s.map(g=>g.id);return pe(h),[...h,...m]}function ks(e){const{id:t,handle:r,amount:n,itemIds:i,linkedSelection:o,removeTransitionIds:a}=e,s=(a?.length??0)>0;return s&&q.getState()._removeTransitions([...a]),w3(t,r,n,i,o)??(s?[]:null)}function k3(e){const{id:t,handle:r,delta:n,linkedSelection:i}=e;if(n===0)return null;const o=S.getState(),a=o.items.find(O=>O.id===t);if(!a)return null;const s=An(o.items,t,i),l=new Set(s.map(O=>O.id)),c=new Map(s.map(O=>[O.id,O])),u=q.getState().transitions,d=$.getState().keyframesByItemId,f=W.getState().fps;let p=n;for(const O of s)p=Ar(p,c3(O,r,p,o.items,u,d,f,l,!1));if(p===0)return null;const m=a.from,h=a.from+a.durationInFrames;r==="start"?o._trimItemStart(t,p,{skipAdjacentClamp:!0}):o._trimItemEnd(t,p,{skipAdjacentClamp:!0});const g=S.getState().itemById[t];if(!g)return null;let v=0,x=null,b=null;if(r==="end"){const O=g.from+g.durationInFrames;v=O-h,v<0?x={start:O,end:h}:v>0&&(b=h);for(const C of s)C.id!==t&&v!==0&&o._trimItemEnd(C.id,v,{skipAdjacentClamp:!0})}else{const O=g.from-m;if(O!==0){o._moveItem(t,m);for(const C of s){if(C.id===t)continue;o._trimItemStart(C.id,O,{skipAdjacentClamp:!0});const R=c.get(C.id);R&&o._moveItem(C.id,R.from)}}v=-O,v<0?x={start:h+v,end:h}:v>0&&(b=h)}let w=[],E=[];if(v!==0){const O=S.getState().items,C=new Map;for(const D of s){const L=c.get(D.id);if(!L)continue;const H=L.from+L.durationInFrames,J=new Set;for(const fe of u)fe.leftClipId===D.id&&J.add(fe.rightClipId);for(const fe of O)l.has(fe.id)||fe.trackId!==L.trackId||(fe.from>=H||J.has(fe.id))&&C.set(fe.id,v)}const R=O.flatMap(D=>{const L=C.get(D.id)??0;return L===0?[]:[{id:D.id,from:D.from+L}]});R.length>0&&o._moveItems(R);const M=new Set(s.map(D=>D.trackId));if(x){const D=Ua({editedTrackIds:M,intervals:[x]});w=D.affectedIds,E=D.removedIds}else b!==null&&(w=s_({editedTrackIds:M,cutFrame:b,amount:v}).affectedIds)}const k=S.getState().items,T=Array.from(new Set([...s.map(O=>O.id),...w,...k.filter(O=>!l.has(O.id)&&s.some(C=>O.trackId===C.trackId&&O.from>=C.from)).map(O=>O.id)]));return E.length>0&&(q.getState()._removeTransitionsForItems(E),$.getState()._removeKeyframesForItems(E)),pe(T,E.length>0?new Set(E):void 0),T}function P3(e){const{leftId:t,rightId:r,delta:n,linkedSelection:i}=e;if(n===0)return null;const o=S.getState(),a=o.items,s=$g(a,t,r,i),l=a.find(v=>v.id===r),c=a.find(v=>v.id===t);if(!c||!l)return null;const u=q.getState().transitions,d=$.getState().keyframesByItemId,f=W.getState().fps;let p=of(c,"end",n,l,a,u,d,f,!1);if(s&&(p=Ar(p,of(s.leftCounterpart,"end",p,s.rightCounterpart,a,u,d,f,!1))),p===0)return null;p>0?(o._trimItemStart(r,p),o._trimItemEnd(t,p)):(o._trimItemEnd(t,p),o._trimItemStart(r,p));const m=S.getState().itemById[r],h=m?m.from-l.from:0;s&&h!==0&&(h>0?(o._trimItemStart(s.rightCounterpart.id,h,{skipAdjacentClamp:!0}),o._trimItemEnd(s.leftCounterpart.id,h,{skipAdjacentClamp:!0})):(o._trimItemEnd(s.leftCounterpart.id,h,{skipAdjacentClamp:!0}),o._trimItemStart(s.rightCounterpart.id,h,{skipAdjacentClamp:!0})));const g=s?[t,r,s.leftCounterpart.id,s.rightCounterpart.id]:[t,r];return pe(g),g}function T3(e){const{id:t,delta:r,linkedSelection:n}=e;if(r===0)return null;const i=S.getState(),o=i.items,a=o.find(m=>m.id===t);if(!a||a.type!=="video"&&a.type!=="audio"&&a.type!=="composition")return null;const s=An(o,t,n),l=a.sourceStart??0,c=a.sourceEnd;if(c===void 0)return null;const u=q.getState().transitions,d=W.getState().fps;let f=r;for(const m of s)f=vg(m.sourceStart??0,m.sourceEnd,m.sourceDuration,f),f=IE(m,f,o,u,d);if(f===0)return null;i._updateItem(t,{sourceStart:l+f,sourceEnd:c+f});for(const m of s)m.id===t||m.sourceEnd===void 0||i._updateItem(m.id,{sourceStart:(m.sourceStart??0)+f,sourceEnd:m.sourceEnd+f});const p=s.map(m=>m.id);return pe(p),p}function E3(e){const{id:t,delta:r,leftNeighborId:n,rightNeighborId:i,linkedSelection:o}=e;if(r===0)return null;const a=S.getState(),s=a.items,l=q.getState().transitions,c=s.find(M=>M.id===t);if(!c)return null;const u=n?s.find(M=>M.id===n)??null:null,d=i?s.find(M=>M.id===i)??null:null,f=An(s,t,o).find(M=>M.id!==t)??null,p=f&&n?Dd(s,n,f.trackId,f.type,o):null,m=f&&i?Dd(s,i,f.trackId,f.type,o):null,h=f?f.from+f.durationInFrames:0,g=f?s.find(M=>M.trackId===f.trackId&&M.id!==f.id&&M.from+M.durationInFrames===f.from)??p:null,v=f?s.find(M=>M.trackId===f.trackId&&M.id!==f.id&&M.from===h)??m:null,x=W.getState().fps;let b=af(r,c,u,d,s,x);if(f&&(b=af(b,f,g,v,s,x)),b=Sd(c,b,u,d,s,l,x),f&&(b=Sd(f,b,g,v,s,l,x)),b=s3(b,[{item:c,leftNeighbor:u,rightNeighbor:d},...f?[{item:f,leftNeighbor:g,rightNeighbor:v}]:[]],l,$.getState().keyframesByItemId),b===0)return null;const w=c.from,E=c.sourceStart??0,k=bg(c,u,d,b,x);b>0?(i&&a._trimItemStart(i,b,{skipAdjacentClamp:!0}),n&&a._trimItemEnd(n,b,{skipAdjacentClamp:!0})):(n&&a._trimItemEnd(n,b,{skipAdjacentClamp:!0}),i&&a._trimItemStart(i,b,{skipAdjacentClamp:!0})),a._moveItem(t,c.from+b),k!==0&&(c.type==="video"||c.type==="audio"||c.type==="composition")&&c.sourceEnd!==void 0&&a._updateItem(t,{sourceStart:(c.sourceStart??0)+k,sourceEnd:c.sourceEnd+k});const T=S.getState().itemById[t],O=T?T.from-w:0,C=T&&T.sourceStart!==void 0?T.sourceStart-E:0;f&&O!==0&&(O>0?(v&&a._trimItemStart(v.id,O,{skipAdjacentClamp:!0}),g&&a._trimItemEnd(g.id,O,{skipAdjacentClamp:!0})):(g&&a._trimItemEnd(g.id,O,{skipAdjacentClamp:!0}),v&&a._trimItemStart(v.id,O,{skipAdjacentClamp:!0})),a._moveItem(f.id,f.from+O),C!==0&&sr(f)&&f.sourceEnd!==void 0&&a._updateItem(f.id,{sourceStart:(f.sourceStart??0)+C,sourceEnd:f.sourceEnd+C}));const R=[t];return n&&R.push(n),i&&R.push(i),f&&(R.push(f.id),g&&R.push(g.id),v&&R.push(v.id)),pe(R),R}function Tc(e,t,r,n){return e.some(i=>{const o=gn(i.durationInFrames,i.alignment);return i.leftClipId===t&&r>=n-o.leftPortion||i.rightClipId===t&&r<o.rightPortion})}function Ec(e,t,r){return Tc(q.getState().transitions,e,t,r)}function C3(e,t,r,n,i){const o=ji([...e],r,i);if(o.length===0)return"missing-item";for(const a of o){if(n<=a.from||n>=a.from+a.durationInFrames)return"out-of-range";if(Tc(t,a.id,n-a.from,a.durationInFrames))return"transition-overlap"}return null}function Cc(e,t){return e.map(r=>({originalId:r.id,originalLinkedGroupId:r.linkedGroupId,result:S.getState()._splitItem(r.id,t)})).filter(r=>r.result!==null)}function M3(e){const{id:t,frame:r,linkedSelection:n}=e,i=ji(S.getState().items,t,n),o=Cc(i,r),a=o.find(s=>s.originalId===t)?.result??null;return a?(qi(o),{...a,leftItemIds:o.map(s=>s.result.leftItem.id)}):null}function _3(e){const{frame:t,linkedSelection:r}=e,n=S.getState().items,i=n.filter(s=>t>s.from&&t<s.from+s.durationInFrames).map(s=>s.id),o=Ug(n,i),a={splitCount:0,blockedByTransitionCount:0,leftItemIds:[]};for(const s of o){const l=ji(S.getState().items,s,r);if(l.length===0)continue;let c=!1;if(!l.every(p=>t<=p.from||t>=p.from+p.durationInFrames?!1:Ec(p.id,t-p.from,p.durationInFrames)?(c=!0,!1):!0)){c&&(a.blockedByTransitionCount+=1);continue}const d=Cc(l,t);(d.find(p=>p.originalId===s)?.result??null)&&(qi(d),a.leftItemIds=d.map(p=>p.result.leftItem.id),a.splitCount+=1)}return a}function F3(e){const{id:t,linkedSelection:r}=e,n=[...e.frames].sort((s,l)=>l-s);let i=0;const o=ji(S.getState().items,t,r);if(o.length===0)return{splitCount:i};const a=new Map(o.map(s=>[s.id,[]]));for(const s of n){const l=S.getState().itemById;if(!o.every(d=>{const f=l[d.id];return!f||s<=f.from||s>=f.from+f.durationInFrames?!1:!Ec(f.id,s-f.from,f.durationInFrames)}))continue;const u=Cc(o,s);if(u.length===o.length){qi(u),i++;for(const d of u)a.get(d.originalId)?.push(d.result.rightItem.id),pe([d.result.leftItem.id,d.result.rightItem.id])}}if(i>0){const s=S.getState();for(const[l,c]of a){for(const u of c)s._updateItem(u,{fadeIn:0});for(let u=1;u<c.length;u+=1)s._updateItem(c[u],{fadeOut:0});s._updateItem(l,{fadeOut:0})}}return{splitCount:i}}function A3(e){const{itemIds:t,linkedSelection:r}=e,n=S.getState().items,i=n.filter(c=>t.includes(c.id)).toSorted((c,u)=>c.from-u.from);if(i.length<2)return null;const o=[t];if(i.length===2){const[c,u]=i;if(c&&u){const d=$g(n,c.id,u.id,r);d&&o.push([d.leftCounterpart.id,d.rightCounterpart.id])}}const a=o.map(c=>n.filter(u=>c.includes(u.id)).toSorted((u,d)=>u.from-d.from)).filter(c=>c.length>=2).map(c=>({itemIds:c.map(u=>u.id),primaryId:c[0].id,removedIds:c.slice(1).map(u=>u.id)}));for(const c of a)S.getState()._joinItems(c.itemIds);const s=new Map;for(const c of a)for(const u of c.removedIds)s.set(u,c.primaryId);if(s.size>0){const u=q.getState().transitions.flatMap(d=>{const f={...d,leftClipId:s.get(d.leftClipId)??d.leftClipId,rightClipId:s.get(d.rightClipId)??d.rightClipId};return f.leftClipId===f.rightClipId?[]:[f]});q.getState().setTransitions(u),pe(a.map(d=>d.primaryId))}const l=a.flatMap(c=>c.removedIds);return l.length>0&&$.getState()._removeKeyframesForItems(l),a.map(c=>c.primaryId)}function ti(e,t,r,n,i){const o=e.find(s=>s.id===t);if(!o||n.has(t))return;n.add(t),i.push({id:t,from:r(o.from)});const a=Ti(e,Hi(e,t));for(const s of a){if(s===t||n.has(s))continue;const l=e.find(c=>c.id===s);l&&(n.add(s),i.push({id:s,from:r(l.from)}))}}function c0(e,t,r,n,i){if(r<=0||i<=0)return[];const o=S.getState(),a=i/r,s=n+i,l=[];for(const c of xc(o.items,e)){const u=o.itemById[c];if(!u||u.type!=="text")continue;const d=n+Math.round((u.from-t)*a),f=n+Math.round((u.from+u.durationInFrames-t)*a),p=Math.max(d,n),m=Math.min(f,s);if(m<=p){o._removeItems([u.id]),$.getState()._removeKeyframesForItems([u.id]),l.push(u.id);continue}const h=m-p,g=p!==u.from||h!==u.durationInFrames,v=u.words&&(g||a!==1)?vn(cy(u.words,f-d,u.durationInFrames),p-d,m-d):void 0;!g&&v===void 0||(o._updateItem(u.id,{from:p,durationInFrames:h,...v?{words:v}:{}}),l.push(u.id))}return l}function O3(e){const{id:t,newFrom:r,newDuration:n,newSpeed:i,linkedSelection:o}=e,a=S.getState(),s=a.items,l=An(s,t,o),c=l.find(M=>M.id===t);if(!c)return null;const u=c.durationInFrames,d=c.from,f=d+u;a._rateStretchItem(t,r,n,i);const p=S.getState().itemById[t];if(!p)return null;const m=p.from,h=p.durationInFrames,g=p.speed??i,v=m-c.from;for(const M of l)M.id!==t&&a._rateStretchItem(M.id,M.from+v,h,g);if(u!==h)for(const M of l)$.getState()._scaleKeyframesForItem(M.id,M.durationInFrames,h);const x=[];if(u!==h||v!==0)for(const M of l){const D=S.getState().itemById[M.id];D&&x.push(...c0(M.id,M.from,M.durationInFrames,D.from,D.durationInFrames))}const b=m+h-f,w=new Set(l.map(M=>M.id)),E=S.getState().items,k=q.getState().transitions,T=new Set,O=[],C=new Set;for(const M of l){const D=E.find(L=>L.id===M.id);D&&C.add(D.trackId)}if(b!==0){const M=D=>D+b;for(const D of C){const L=E.filter(H=>H.trackId===D&&!w.has(H.id)&&H.from>=f).sort((H,J)=>H.from-J.from);for(const H of L)ti(E,H.id,M,T,O)}for(const D of l)for(const L of k)L.leftClipId===D.id&&!w.has(L.rightClipId)&&ti(E,L.rightClipId,M,T,O)}if(v!==0){const M=D=>Math.max(0,D+v);for(const D of C){const L=E.filter(H=>H.trackId!==D||w.has(H.id)?!1:H.from+H.durationInFrames<=d).sort((H,J)=>H.from-J.from);for(const H of L)ti(E,H.id,M,T,O)}for(const D of l)for(const L of k)L.rightClipId===D.id&&!w.has(L.leftClipId)&&ti(E,L.leftClipId,M,T,O)}O.length>0&&S.getState()._moveItems(O);const R=[...w,...T,...x];return pe(R),R}const z3=.01;function D3(e){const{itemIds:t,linkedSelection:r}=e,n=S.getState(),i=W.getState().fps,o=new Set,a=[];for(const p of t){if(o.has(p))continue;const m=n.items.find(b=>b.id===p);if(!m||m.type!=="video"&&m.type!=="audio")continue;const h=m.speed||1;if(Math.abs(h-1)<=z3)continue;const g=An(n.items,p,r);for(const b of g)o.add(b.id);const v=m.sourceFps??i,x=m.sourceEnd!==void 0&&m.sourceStart!==void 0?m.sourceEnd-m.sourceStart:De(m.durationInFrames,h,i,v);a.push({id:p,oldEnd:m.from+m.durationInFrames,newDuration:Math.max(1,Kt(x,1,v,i)),synchronizedIds:g.map(b=>b.id)})}if(a.length===0)return null;const s=[];for(const p of a){const m=n.items.find(w=>w.id===p.id);if(!m)continue;const h=m.durationInFrames,g=new Map(S.getState().items.filter(w=>p.synchronizedIds.includes(w.id)).map(w=>[w.id,{from:w.from,durationInFrames:w.durationInFrames}]));n._rateStretchItem(p.id,m.from,p.newDuration,1);const v=S.getState().itemById[p.id];if(!v)continue;const x=v.durationInFrames,b=v.from-m.from;for(const w of p.synchronizedIds){if(w===p.id)continue;const E=S.getState().items.find(k=>k.id===w);E&&n._rateStretchItem(w,E.from+b,x,v.speed??1)}if(h!==x)for(const w of p.synchronizedIds)$.getState()._scaleKeyframesForItem(w,h,x);for(const w of p.synchronizedIds){const E=g.get(w),k=S.getState().itemById[w];!E||!k||E.durationInFrames===k.durationInFrames&&E.from===k.from||s.push(...c0(w,E.from,E.durationInFrames,k.from,k.durationInFrames))}}const l=S.getState().items,c=new Set(a.flatMap(p=>p.synchronizedIds)),u=[],d=new Set;for(const p of a){const m=l.find(v=>v.id===p.id);if(!m)continue;const h=m.from+m.durationInFrames-p.oldEnd;if(h<=0)continue;const g=new Set;for(const v of p.synchronizedIds){const x=l.find(b=>b.id===v);x&&g.add(x.trackId)}for(const v of g){const x=l.filter(b=>b.trackId===v&&!c.has(b.id)&&b.from>=p.oldEnd).sort((b,w)=>b.from-w.from);for(const b of x)ti(l,b.id,w=>w+h,d,u)}}u.length>0&&S.getState()._moveItems(u);const f=[...c,...d,...s];return pe(f),f}function R3(e){if(!e)return;const t=Oe.getState().mediaById[e]?.fps;return typeof t=="number"&&Number.isFinite(t)&&t>0?t:void 0}function u0(e,t){if(e.type!=="video"&&e.type!=="audio"&&e.type!=="composition")return t;if(e.sourceFps!==void 0)return e.sourceFps;const r=e.type==="composition"?void 0:e.mediaId;return R3(r)??t}function Cl(e){return e.type==="video"||e.type==="audio"||e.type==="composition"?e.speed??1:1}function sf(e,t,r){const n=u0(e,r),i=Math.round(t*n),o=e.type==="video"||e.type==="audio"?e.sourceStart??0:0,a=e.type==="video"||e.type==="audio"?e.sourceEnd??o+De(e.durationInFrames,Cl(e),r,n):o,s=e.isReversed?a-i:i-o,l=Kt(s,Cl(e),n,r);return Math.round(e.from+l)}function L3(e,t){if(e.type!=="video"&&e.type!=="audio")return null;const r=u0(e,t),n=e.sourceStart??0,i=De(e.durationInFrames,Cl(e),t,r),o=e.sourceEnd??n+i;return{start:n/r,end:o/r}}const B3=.75;function lf(){return{analyzedItemCount:0,removedRangeCount:0,removedItemCount:0,splitCount:0}}function U3(e,t){const r=e.end-e.start;return r<=0?!1:t.reduce((i,o)=>{const a=Math.max(e.start,o.start),s=Math.min(e.end,o.end);return i+Math.max(0,s-a)},0)/r>=B3}function N3(e,t){const r=S.getState().items,n=Rr(r,e,t);if(n.length===0)return{removedIds:[],affectedIds:[]};const i=new Set(n),o=r.filter(T=>!i.has(T.id)),a=new Map,s=new Set(r.filter(T=>i.has(T.id)).map(T=>T.trackId)),l=r.filter(T=>i.has(T.id)).map(T=>({start:T.from,end:T.from+T.durationInFrames}));for(const T of o){const O=r.filter(C=>i.has(C.id)).filter(C=>C.trackId===T.trackId&&C.from+C.durationInFrames<=T.from).reduce((C,R)=>C+R.durationInFrames,0);O>0&&a.set(T.id,O)}const c=new Map(S.getState().tracks.map(T=>[T.id,T])),u=new Map(o.map(T=>[T.id,T])),d=new Map;for(const[T,O]of a){if(O<=0)continue;const C=Rr(o,[T],t);for(const R of C){const M=u.get(R);!M||!s.has(M.trackId)&&ha(c.get(M.trackId))||d.set(R,Math.max(d.get(R)??0,O))}}const f=o.flatMap(T=>{const O=d.get(T.id)??0;return O>0?[{id:T.id,from:T.from-O}]:[]}),p=new Map(f.map(T=>[T.id,T.from])),m=[];for(const T of o){if(p.has(T.id))continue;const O=T.from+T.durationInFrames;for(const C of o){const R=p.get(C.id);if(R===void 0||C.trackId!==T.trackId)continue;const M=R+C.durationInFrames;if(R<O&&M>T.from){m.push(T.id);break}}}const h=Rr(o,m,t),g=[...n,...h],v=new Set(h),x=v.size>0?f.filter(T=>!v.has(T.id)):f,b=S.getState();b._removeItems(g),x.length>0&&b._moveItems(x);const w=Ua({editedTrackIds:s,intervals:l}),E=Array.from(new Set([...g,...w.removedIds]));q.getState()._removeTransitionsForItems(E),$.getState()._removeKeyframesForItems(E),x.length>0&&pe(x.map(T=>T.id));const k=Array.from(new Set([...f.map(T=>T.id),...w.affectedIds]));return k.length>0&&pe(k,new Set(E)),{removedIds:E,affectedIds:Array.from(new Set([...k,...x.map(T=>T.id)]))}}function $3(e){const{itemIds:t,rangesByMediaId:r,linkedSelection:n}=e;if(t.length===0)return{summary:lf(),affectedIds:[]};const i=W.getState().fps,o=S.getState().items,s=Ug(o,t).map(m=>o.find(h=>h.id===m)).filter(m=>m!==void 0&&(m.type==="video"||m.type==="audio")&&!!m.mediaId&&(r[m.mediaId]?.length??0)>0);if(s.length===0)return{summary:lf(),affectedIds:[]};const l=s.map(m=>({id:m.id,mediaId:m.mediaId,originId:m.originId??m.id}));let c=0;for(const m of s){const h=r[m.mediaId];if(!h||h.length===0)continue;const g=Array.from(new Set(h.flatMap(x=>[sf(m,x.start,i),sf(m,x.end,i)]))).filter(x=>x>m.from&&x<m.from+m.durationInFrames).sort((x,b)=>b-x);if(g.length===0)continue;const v=ji(S.getState().items,m.id,n);if(v.length!==0)for(const x of g){const b=S.getState().itemById;if(!v.every(k=>{const T=b[k.id];return!T||x<=T.from||x>=T.from+T.durationInFrames?!1:!Ec(T.id,x-T.from,T.durationInFrames)}))continue;const E=v.map(k=>({originalId:k.id,originalLinkedGroupId:k.linkedGroupId,result:S.getState()._splitItem(k.id,x)})).filter(k=>k.result!==null);if(E.length===v.length){qi(E),c+=1;for(const k of E)pe([k.result.leftItem.id,k.result.rightItem.id])}}}const u=S.getState().items,d=new Set,f=new Set;for(const m of l){const h=r[m.mediaId];if(!(!h||h.length===0))for(const g of u){if(g.type!=="video"&&g.type!=="audio"||g.mediaId!==m.mediaId||(g.originId??g.id)!==m.originId)continue;const v=L3(g,i);v!==null&&U3(v,h)&&(d.add(g.id),h.forEach((x,b)=>{x.end>v.start&&x.start<v.end&&f.add(`${m.originId}:${b}`)}))}}if(d.size===0)return{summary:{analyzedItemCount:s.length,removedRangeCount:0,removedItemCount:0,splitCount:c},affectedIds:[]};const p=N3(Array.from(d),n);return{summary:{analyzedItemCount:s.length,removedRangeCount:f.size,removedItemCount:p.removedIds.length,splitCount:c},affectedIds:Array.from(new Set([...d,...p.affectedIds]))}}function H3(e,t,r,n){const i=e.find(o=>o.id===r);return!i||i.type!=="video"?"not-video":n<=i.from||n>=i.from+i.durationInFrames?"out-of-range":Tc(t,r,n-i.from,i.durationInFrames)?"transition-overlap":null}function j3(e){const{itemId:t,playheadFrame:r,freezeDurationFrames:n,image:i}=e,o=S.getState().itemById[t];if(!o)return null;const a=S.getState()._splitItem(t,r);if(!a)return null;const{leftItem:s,rightItem:l}=a,c=q.getState().transitions;q.getState().setTransitions(c.map(f=>f.leftClipId===t?{...f,leftClipId:l.id}:f));const u={id:Z(),type:"image",trackId:o.trackId,from:r,durationInFrames:n,label:i.label,mediaId:i.mediaId,src:i.src,sourceWidth:i.sourceWidth,sourceHeight:i.sourceHeight,transform:o.transform?{...o.transform}:void 0};S.getState()._addItem(u),S.getState()._moveItem(l.id,l.from+n);const d=S.getState().items.filter(f=>f.trackId===o.trackId&&f.id!==l.id&&f.id!==s.id&&f.id!==u.id&&f.from>r);for(const f of d)S.getState()._moveItem(f.id,f.from+n);return pe([s.id,l.id]),{freezeFrameItemId:u.id}}function q3(e){const{insertFrame:t,clipDurationFrames:r,tracks:n,items:i}=e,o=S.getState();o.setTracks(n);const a=Array.from(new Set(i.map(u=>u.trackId))),s=[],l=[];for(const u of a){const d=S.getState().items.find(p=>p.trackId===u&&p.from<t&&p.from+p.durationInFrames>t);if(d){const p=o._splitItem(d.id,t);p&&s.push(p.leftItem.id,p.rightItem.id)}const f=S.getState().items.filter(p=>p.trackId===u&&p.from>=t);for(const p of f)o._moveItem(p.id,p.from+r),l.push(p.id)}for(const u of i)o._addItem(u);const c=[...i.map(u=>u.id),...l,...s];return pe(c),c}function K3(e){const{overwriteStart:t,overwriteEnd:r,tracks:n,items:i}=e,o=S.getState();o.setTracks(n);const a=Array.from(new Set(i.map(l=>l.trackId))),s=[];for(const l of a){const c=S.getState().items.filter(u=>u.trackId===l&&u.from<r&&u.from+u.durationInFrames>t);for(const u of c){const d=u.from+u.durationInFrames,f=u.from<t,p=d>r;if(!f&&!p)o._removeItems([u.id]);else if(f&&p){const m=o._splitItem(u.id,t);if(m){s.push(m.leftItem.id);const h=S.getState()._splitItem(m.rightItem.id,r);h&&(o._removeItems([h.leftItem.id]),s.push(h.rightItem.id))}}else if(f){const m=o._splitItem(u.id,t);m&&(o._removeItems([m.rightItem.id]),s.push(m.leftItem.id))}else{const m=o._splitItem(u.id,r);m&&(o._removeItems([m.leftItem.id]),s.push(m.rightItem.id))}}}for(const l of i)o._addItem(l),s.push(l.id);return pe(s),s}function G3(e){const{items:t,tracks:r}=S.getState(),n=m_({tracks:r,items:t,keyframes:$.getState().keyframes,fps:W.getState().fps,videoHasAudioByMediaId:e.videoHasAudioByMediaId,createId:Z});if(!n.changed)return!1;S.getState().setTracks(n.tracks),S.getState().setItems(n.items),$.getState().setKeyframes(n.keyframes);const i=ye.getState().activeCompositionId;return i&&ae.getState().updateComposition(i,{tracks:n.tracks,items:n.items,keyframes:n.keyframes}),!0}const We=I().min(1),Te=P().finite(),Ki=F(We),ot=B(),d0=U(["start","end"]);function jt(e){return{inverse:{kind:"patch",patch:{}},result:e}}function wt(e){return jt(e?{affectedIds:e}:null)}function xt(e,t,r){return e.items.some(n=>n.id===t)?[]:[{message:`Không tìm thấy clip ${t}`,code:"missing-item",path:r}]}const Mc={id:We,amount:Te,itemIds:Ki.optional(),linkedSelection:ot},W3=y({...Mc,handle:ne("start")}).strict(),V3=y({...Mc,handle:ne("end")}).strict(),Z3=y({...Mc,handle:d0,removeTransitionIds:Ki}).strict();function Y3(){A({type:"edit.trimItemStart",params:W3,validate:(i,o)=>xt(i,o.id,"id"),apply:(i,o)=>wt(ks(o))}),A({type:"edit.trimItemEnd",params:V3,validate:(i,o)=>xt(i,o.id,"id"),apply:(i,o)=>wt(ks(o))}),A({type:"edit.trimItemBreakingTransition",params:Z3,validate:(i,o)=>{const a=new Set(i.transitions.map(s=>s.id));return[...xt(i,o.id,"id"),...o.removeTransitionIds.filter(s=>!a.has(s)).map(s=>({message:`Không tìm thấy transition ${s}`,code:"missing-transition",path:"removeTransitionIds"}))]},apply:(i,o)=>wt(ks(o))});const e=y({id:We,handle:d0,delta:Te,linkedSelection:ot}).strict();A({type:"edit.rippleTrim",params:e,validate:(i,o)=>xt(i,o.id,"id"),apply:(i,o)=>wt(k3(o))});const t=y({leftId:We,rightId:We,delta:Te,linkedSelection:ot}).strict();A({type:"edit.rollingTrim",params:t,validate:(i,o)=>[...xt(i,o.leftId,"leftId"),...xt(i,o.rightId,"rightId")],apply:(i,o)=>wt(P3(o))});const r=y({id:We,delta:Te,linkedSelection:ot}).strict();A({type:"edit.slip",params:r,validate:(i,o)=>xt(i,o.id,"id"),apply:(i,o)=>wt(T3(o))});const n=y({id:We,delta:Te,leftNeighborId:We.nullable(),rightNeighborId:We.nullable(),linkedSelection:ot}).strict();A({type:"edit.slide",params:n,validate:(i,o)=>xt(i,o.id,"id"),apply:(i,o)=>wt(E3(o))})}function X3(){const e=y({id:We,frame:Te,linkedSelection:ot}).strict();A({type:"edit.splitItem",params:e,validate:(i,o)=>{const a=C3(i.items,i.transitions,o.id,o.frame,o.linkedSelection);return a?[{message:a==="transition-overlap"?"Không thể cắt bên trong vùng chuyển cảnh":a==="out-of-range"?"Điểm cắt nằm ngoài clip":`Không tìm thấy clip ${o.id}`,code:a,path:"frame"}]:[]},apply:(i,o)=>jt(M3(o))});const t=y({frame:Te,linkedSelection:ot}).strict();A({type:"edit.splitAllAtFrame",params:t,apply:(i,o)=>jt(_3(o))});const r=y({id:We,frames:F(Te).min(1),linkedSelection:ot}).strict();A({type:"edit.splitItemAtFrames",params:r,validate:(i,o)=>xt(i,o.id,"id"),apply:(i,o)=>jt(F3(o))});const n=y({itemIds:Ki.min(2),linkedSelection:ot}).strict();A({type:"edit.joinItems",params:n,apply:(i,o)=>wt(A3(o))})}function Q3(){const e=y({id:We,newFrom:Te,newDuration:Te,newSpeed:Te.positive(),linkedSelection:ot,mergeKey:I().min(1).optional()}).strict();A({type:"edit.rateStretch",params:e,validate:(r,n)=>xt(r,n.id,"id"),apply:(r,n)=>wt(O3(n)),mergeKey:r=>r.mergeKey??null});const t=y({itemIds:Ki,linkedSelection:ot}).strict();A({type:"edit.resetSpeedWithRipple",params:t,apply:(r,n)=>wt(D3(n))})}const J3=y({start:Te,end:Te}).strict(),eA=y({itemIds:Ki,rangesByMediaId:Ct(I(),F(J3)),linkedSelection:ot}).strict();function tA(){for(const e of["edit.removeSilence","edit.removeFillerWords","edit.removeTranscriptRanges"])A({type:e,params:eA,apply:(t,r)=>jt($3(r))})}function rA(){const e=y({itemId:We,playheadFrame:Te,freezeDurationFrames:Te.min(1),image:y({label:I(),mediaId:We,src:I().min(1),sourceWidth:Te.positive(),sourceHeight:Te.positive()}).strict()}).strict();A({type:"edit.insertFreezeFrame",params:e,validate:(t,r)=>{const n=H3(t.items,t.transitions,r.itemId,r.playheadFrame);return n?[{message:`Không thể chèn khung hình tĩnh (${n})`,code:n,path:"playheadFrame"}]:[]},apply:(t,r)=>jt(j3(r))})}const cf=rh(e=>{if(typeof e!="object"||e===null)return!1;const t=e;return typeof t.id=="string"&&t.id.length>0&&typeof t.type=="string"&&typeof t.trackId=="string"&&typeof t.from=="number"&&Number.isFinite(t.from)&&typeof t.durationInFrames=="number"&&Number.isFinite(t.durationInFrames)&&t.durationInFrames>0},{message:"Clip không hợp lệ"}),uf=rh(e=>typeof e=="object"&&e!==null&&typeof e.id=="string",{message:"Track không hợp lệ"});function df(e,t,r){const n=[],i=new Set(t.map(s=>s.id)),o=new Set(e.items.map(s=>s.id)),a=new Set;for(const s of r)if((o.has(s.id)||a.has(s.id))&&n.push({message:`Clip ${s.id} đã tồn tại`,code:"duplicate-id",path:"items"}),a.add(s.id),i.has(s.trackId)||n.push({message:`Không tìm thấy track ${s.trackId}`,code:"missing-track",path:"items"}),s.type==="video"||s.type==="audio"){const{sourceStart:l,sourceEnd:c}=s;(l!==void 0&&(!Number.isFinite(l)||l<0)||c!==void 0&&(!Number.isFinite(c)||c<=(l??0)))&&n.push({message:`Giới hạn nguồn của clip ${s.id} không hợp lệ`,code:"source-bounds",path:"items"})}return n}function nA(){const e=y({insertFrame:Te.min(0),clipDurationFrames:Te.min(1),tracks:F(uf),items:F(cf).min(1)}).strict();A({type:"edit.sourceInsert",params:e,validate:(n,i)=>df(n,i.tracks,i.items),apply:(n,i)=>jt(q3(i))});const t=y({overwriteStart:Te.min(0),overwriteEnd:Te.min(1),tracks:F(uf),items:F(cf).min(1)}).strict();A({type:"edit.sourceOverwrite",params:t,validate:(n,i)=>[...i.overwriteEnd>i.overwriteStart?[]:[{message:"Vùng ghi đè phải có độ dài dương",code:"empty-range",path:"overwriteEnd"}],...df(n,i.tracks,i.items)],apply:(n,i)=>jt(K3(i))});const r=y({videoHasAudioByMediaId:Ct(I(),B())}).strict();A({type:"edit.repairLegacyAvTracks",params:r,apply:(n,i)=>jt(G3(i))})}function iA(){Y3(),X3(),Q3(),tA(),rA(),nA()}function oA(e,t){const r=S.getState(),n=$.getState();if(t.enabled&&(Vg(e,t.sourceItemId,i=>r.itemById[i],i=>n.keyframesByItemId[i])||Zg(t.targetProperty)&&O_(e,t.sourceItemId,i=>r.itemById[i]))){Me().warn("Cannot create a cyclic or redundant property link",{itemId:e,property:t.targetProperty,sourceItemId:t.sourceItemId,sourceProperty:t.sourceProperty});return}$.getState()._setDirectPropertyLink(e,t)}function aA(e,t){$.getState()._removeDirectPropertyLink(e,t)}function sA(e,t){$.getState()._setPropertyExpression(e,t)}function lA(e,t){$.getState()._removePropertyExpression(e,t)}function cA(e,t,r){return et(e,r.frame)?$.getState()._upsertVectorKeyframe(e,t,r):(Me().warn("Cannot add vector keyframe in transition region",{itemId:e,property:t,frame:r.frame}),"")}function uA(e,t,r,n){if(typeof n.frame=="number"&&!et(e,n.frame)){Me().warn("Cannot move vector keyframe into transition region",{itemId:e,property:t,keyframeId:r,frame:n.frame});return}$.getState()._updateVectorKeyframe(e,t,r,n)}function dA(e,t,r){$.getState()._removeVectorKeyframe(e,t,r)}function fA(e,t,r){$.getState()._replaceScalarPropertiesWithVectorProperty(e,t,r)}function pA(e,t,r,n={}){$.getState()._setVectorDimensionsSeparated(e,t,{separated:r,...n})}function Ps(e){if(e.length===0)return[];const t=e.filter(r=>et(r.itemId,r.frame));return t.length===0?(Me().warn("All keyframes blocked by transition regions",{originalCount:e.length}),[]):(t.length<e.length&&Me().warn("Some keyframes blocked by transition regions",{originalCount:e.length,validCount:t.length}),$.getState()._addKeyframes(t))}const mA={position:["x","y"],scale:["width","height"],anchor:["anchorX","anchorY"]};function hA(e,t=[],r=[]){if(e.length===0&&r.length===0)return[];const n=e.filter(a=>et(a.itemId,a.frame)),i=r.flatMap(a=>a.keyframes.map(s=>({itemId:a.itemId,frame:s.frame}))),o=i.filter(a=>et(a.itemId,a.frame));return n.length<e.length||o.length<i.length?(Me().warn("Preset keyframes blocked by transition regions; skipping apply",{originalCount:e.length,validCount:n.length,vectorCount:i.length,validVectorCount:o.length}),[]):(()=>{const a=$.getState();for(const l of t){if(l.fromFrame===void 0||l.toFrame===void 0){a._removeKeyframesForProperty(l.itemId,l.property);continue}const c=a.keyframesByItemId[l.itemId]?.properties.find(d=>d.property===l.property);if(!c)continue;const u=c.keyframes.filter(d=>d.frame>=l.fromFrame&&d.frame<=l.toFrame).map(d=>({itemId:l.itemId,property:l.property,keyframeId:d.id}));u.length>0&&a._removeKeyframes(u)}const s=a._addKeyframes(n);for(const l of r){const c=a.keyframesByItemId[l.itemId]?.vectorProperties?.find(d=>d.property===l.property)?.keyframes??[],u=new Map(c.filter(d=>!l.replaceRange||d.frame<l.replaceRange.fromFrame||d.frame>l.replaceRange.toFrame).map(d=>[d.frame,d]));for(const d of l.keyframes){if(!l.replaceRange&&u.has(d.frame))continue;const p=u.get(d.frame)?.id??Z();u.set(d.frame,{id:p,frame:d.frame,value:d.value,easing:d.easing??"linear",easingConfig:d.easingConfig,temporalEase:d.temporalEase,spatial:d.spatial,source:d.source}),s.push(p)}a._replaceScalarPropertiesWithVectorProperty(l.itemId,{property:l.property,keyframes:[...u.values()].sort((d,f)=>d.frame-f.frame)},mA[l.property])}return s})()}function gA(e,t,r,n){if(typeof n.frame=="number"&&!et(e,n.frame)){Me().warn("Cannot move keyframe into transition region",{itemId:e,property:t,keyframeId:r,frame:n.frame});return}$.getState()._updateKeyframe(e,t,r,n)}function ff(e){if(e.length===0)return;const t=e.filter(r=>{if(typeof r.updates.frame!="number")return!0;const n=et(r.itemId,r.updates.frame);return n||Me().warn("Cannot move keyframe into transition region",{itemId:r.itemId,property:r.property,keyframeId:r.keyframeId,frame:r.updates.frame}),n});t.length!==0&&$.getState()._updateKeyframes(t)}function yA(e){e.length!==0&&(()=>{const t=$.getState();for(const r of e){if(r.type==="vector-update"){t._updateVectorKeyframe(r.itemId,r.property,r.keyframeId,r.updates);continue}if(r.type==="vector-add"){if(!et(r.itemId,r.frame)){Me().warn("Cannot add vector auto keyframe in transition region",{itemId:r.itemId,property:r.property,frame:r.frame});continue}t._upsertVectorKeyframe(r.itemId,r.property,{frame:r.frame,value:r.value,easing:r.easing});continue}if(r.type==="update"){t._updateKeyframe(r.itemId,r.property,r.keyframeId,r.updates);continue}if(!et(r.itemId,r.frame)){Me().warn("Cannot add auto keyframe in transition region",{itemId:r.itemId,property:r.property,frame:r.frame});continue}t._addKeyframe(r.itemId,r.property,r.frame,r.value,r.easing)}})()}function vA(e,t,r){$.getState()._removeKeyframe(e,t,r)}function bA(e){$.getState()._removeKeyframesForItem(e)}function SA(e,t){$.getState()._removeKeyframesForProperty(e,t)}function IA(e,t){$.getState()._removeKeyframesByApplication(e,t)}function xA(e){$.getState()._removeManualKeyframes(e)}function wA(e){return(()=>{const t=S.getState().itemById[e],r=$.getState(),n=r.getKeyframesForItem(e);if(!t||!n)return 0;const i=BE(n,t.durationInFrames,W.getState().fps);return i.removedCount===0?0:(r.setKeyframes(r.keyframes.map(o=>o.itemId===e?i.itemKeyframes:o)),i.removedCount)})()}function kA(e){e.length!==0&&$.getState()._removeKeyframes(e)}function f0(){return He("MotionModifierActions")}function PA(e){return e.length===0?0:(()=>{const t=S.getState();let r=0;for(const{itemId:n,layer:i}of e){const o=t.itemById[n];o&&(t._updateItem(n,{motionLayers:[...o.motionLayers??[],i]}),r+=1)}return r})()}function TA(e,t){return e.length===0?0:(()=>{const r=S.getState();let n=0;for(const i of e){const o=r.itemById[i];o?.motionLayers?.some(a=>a.id===t)&&(r._updateItem(i,{motionLayers:o.motionLayers.filter(a=>a.id!==t)}),n+=1)}return n})()}function EA(e,t){return[...(e??[]).filter(n=>n.type!==t.type),t]}function CA(e){if(e.length===0)return 0;const t=f0().startEvent("applyMotionModifiers",rr());t.merge({requested:e.length,modifierTypes:[...new Set(e.map(r=>r.modifier.type))]});try{const r=(()=>{const n=S.getState();let i=0;for(const{itemId:o,modifier:a}of e){const s=n.itemById[o];s&&(n._updateItem(o,{motionModifiers:EA(s.motionModifiers,a)}),i+=1)}return i})();return t.success({updated:r}),r}catch(r){throw t.failure(r),r}}function MA(e){if(e.length===0)return 0;const t=f0().startEvent("bakeMotionToKeyframes",rr());t.merge({plannedItems:e.length,plannedKeyframes:e.reduce((r,n)=>r+n.keyframes.length,0),clearedProperties:e.reduce((r,n)=>r+n.clearProperties.length,0)});try{let r=0,n=0;const i=(()=>{const o=S.getState(),a=$.getState();let s=0;for(const l of e){const c=o.itemById[l.itemId];if(!c)continue;for(const f of l.clearProperties)a._removeKeyframesForProperty(l.itemId,f);const u=l.keyframes.filter(f=>et(f.itemId,f.frame));n+=l.keyframes.length-u.length,u.length>0&&(a._addKeyframes(u),r+=u.length);const d={};if(l.clearMotionModifiers&&(d.motionModifiers=[]),l.clearMotionLayers&&(d.motionLayers=[]),l.clearAudioPulseEffectIds.length>0&&c.effects){const f=new Set(l.clearAudioPulseEffectIds);d.effects=c.effects.map(p=>f.has(p.id)?{...p,audioPulse:void 0}:p)}Object.keys(d).length>0&&o._updateItem(l.itemId,d),s+=1}return s})();return t.success({baked:i,addedKeyframes:r,blockedKeyframes:n}),i}catch(r){throw t.failure(r),r}}function _A(e,t){return e.length===0?0:(()=>{const r=S.getState();let n=0;for(const i of e){const o=r.itemById[i];o?.motionModifiers?.some(a=>a.type===t)&&(r._updateItem(i,{motionModifiers:o.motionModifiers.filter(a=>a.type!==t)}),n+=1)}return n})()}function FA(e){return e.length===0?0:(()=>{const t=S.getState();let r=0;for(const n of e){const i=t.itemById[n];i?.effects?.some(o=>o.audioPulse?.enabled)&&(t._updateItem(n,{effects:i.effects.map(o=>o.audioPulse?{...o,audioPulse:void 0}:o)}),r+=1)}return r})()}function pf(){return He("TextMotionActions")}const AA={in:Xh,out:Qh,loop:Jh};function OA(e,t){return AA[e].includes(t.presetId)}function p0(e,t,r){return{...e,[t]:r}}function zA(e,t){if(!e)return;const r={...e};if(delete r[t],!(!r.in&&!r.out&&!r.loop))return r}function DA(e,t,r){if(e.length===0)return 0;if(!OA(t,r))return pf().warn(`applyTextMotionEffect: preset '${r.presetId}' is not a '${t}' preset`),0;const n=pf().startEvent("applyTextMotionEffect",rr());n.merge({requested:e.length,slot:t,presetId:r.presetId});try{const i=(()=>{const o=S.getState();let a=0;for(const s of e){const l=o.itemById[s];!l||l.type!=="text"||(o._updateItem(s,{textMotion:p0(l.textMotion,t,r)}),a+=1)}return a})();return n.success({updated:i}),i}catch(i){throw n.failure(i),i}}function RA(e,t,r){if(e.length===0)return;const n=S.getState();for(const i of e){const o=n.itemById[i];if(!o||o.type!=="text")continue;const a=o.textMotion?.[t];a&&n._updateItem(i,{textMotion:p0(o.textMotion,t,{...a,...r})})}}function LA(e,t){return e.length===0?0:(()=>{const r=S.getState();let n=0;for(const i of e){const o=r.itemById[i];!o||o.type!=="text"||!o.textMotion?.[t]||(r._updateItem(i,{textMotion:zA(o.textMotion,t)}),n+=1)}return n})()}function BA(e){return{...e,id:Z(),...e.channelGains&&{channelGains:{...e.channelGains}}}}function UA(e){return{...e.in&&{in:{...e.in}},...e.out&&{out:{...e.out}},...e.loop&&{loop:{...e.loop}}}}function NA(e,t){const r=new Set(t.map(n=>n.type));return[...(e??[]).filter(n=>!r.has(n.type)),...t]}const $A={position:["x","y"],scale:["width","height"],anchor:["anchorX","anchorY"]};function HA(e){const{targetItemId:t,keyframes:r,anchorFrame:n,maxFrame:i,retimeFrame:o,source:a,timeScale:s}=e,l=new Map;let c=0,u=0;for(const d of r){const f=Math.max(0,Math.min(i,n+o(d.frame)));if(!et(t,f)){u+=1;continue}l.set(f,{...z1(d,{id:Z(),frame:f}),source:a,...d.temporalEase&&{temporalEase:{...d.temporalEase.in&&{in:{...d.temporalEase.in,speed:s>0?d.temporalEase.in.speed/s:d.temporalEase.in.speed}},...d.temporalEase.out&&{out:{...d.temporalEase.out,speed:s>0?d.temporalEase.out.speed/s:d.temporalEase.out.speed}}}}}),c+=1}return{keyframes:[...l.values()].sort((d,f)=>d.frame-f.frame),applied:c,skipped:u}}function jA(e,t,r,n,i){const o=new Map;for(const a of t)n&&i&&a.frame>=i.fromFrame&&a.frame<=i.toFrame||o.set(a.frame,a);for(const a of r)o.get(a.frame)||o.set(a.frame,a);return{property:e,keyframes:[...o.values()].sort((a,s)=>a.frame-s.frame)}}function qA(e,t,r=0,n={}){const i=S.getState().itemById[e];if(!i)return{applied:0,addedEffects:0,appliedProcedural:0,skipped:0,incompatible:!0,reason:"no-item"};const o=WM(t,i);if(!o.compatible)return{applied:0,addedEffects:0,appliedProcedural:0,skipped:0,incompatible:!0,reason:o.reason};const s={applicationId:Z(),kind:"saved-preset",presetId:t.id,presetName:t.name},l=new Set;for(const C of t.properties){const R=aa(C.property);R&&l.add(R.gpuEffectType)}const c=new Map;for(const C of l){const R=i.effects?.find(M=>M.effect.gpuEffectType===C);R&&c.set(C,R.id)}const u=[];for(const C of l){if(c.has(C))continue;const R=t.effects.find(M=>M.gpuEffectType===C);R&&u.push({...R,params:{...R.params}})}let d=0;if(u.length>0){S.getState()._addEffects([{itemId:e,effects:u}]),d=u.length;const C=S.getState().itemById[e]?.effects??[];for(const R of l)if(!c.has(R))for(let M=C.length-1;M>=0;M--){const D=C[M];if(D&&D.effect.gpuEffectType===R){c.set(R,D.id);break}}}const f=S.getState().itemById[e]??i,p=new Set(Lg(f)),m=Math.max(0,f.durationInFrames-1),h=Math.max(1,t.sourceDurationInFrames-1),g=Math.max(0,m-r),v=n.retime===!1?1:g/h,x=C=>n.retime===!1?C:Math.round(C*v),b=[];let w=0;for(const C of t.properties){let R=C.property;const M=aa(C.property);if(M){const D=c.get(M.gpuEffectType);if(!D){w+=C.keyframes.length;continue}R=Sh(M.gpuEffectType,D,M.paramKey)}else if(!p.has(R)){w+=C.keyframes.length;continue}for(const D of C.keyframes){const L=Math.max(0,Math.min(m,r+x(D.frame)));if(!et(e,L)){w+=1;continue}b.push({itemId:e,property:R,frame:L,value:D.value,easing:D.easing,easingConfig:D.easingConfig,source:s})}}let E=0;if(b.length>0){const C=$.getState();let R=b;if(n.replace){const M=new Set(b.map(D=>D.property));for(const D of M){const L=b.filter(se=>se.property===D),H=Math.min(...L.map(se=>se.frame)),J=Math.max(...L.map(se=>se.frame)),fe=C.getKeyframesForItem(e)?.properties.find(se=>se.property===D)?.keyframes.filter(se=>se.frame>=H&&se.frame<=J).map(se=>({itemId:e,property:D,keyframeId:se.id}))??[];fe.length>0&&C._removeKeyframes(fe)}}else{const M=C.getKeyframesForItem(e);R=b.filter(D=>!M?.properties.find(H=>H.property===D.property)?.keyframes.some(H=>H.frame===D.frame)),w+=b.length-R.length}E=C._addKeyframes(R).length}const k=$.getState();for(const C of t.vectorProperties??[]){const R=HA({targetItemId:e,keyframes:C.keyframes,anchorFrame:r,maxFrame:m,retimeFrame:x,source:s,timeScale:v});if(w+=R.skipped,R.keyframes.length===0)continue;const M=k.getKeyframesForItem(e)?.vectorProperties?.find(H=>H.property===C.property)?.keyframes??[],D=n.replace?R.keyframes:R.keyframes.filter(H=>!M.some(J=>J.frame===H.frame));if(w+=R.keyframes.length-D.length,D.length===0)continue;const L=n.replace===!0?{fromFrame:Math.min(...D.map(H=>H.frame)),toFrame:Math.max(...D.map(H=>H.frame))}:void 0;k._replaceScalarPropertiesWithVectorProperty(e,jA(C.property,M,D,n.replace===!0,L),$A[C.property]),E+=D.length}let T=0;const O=S.getState();if(t.motionModifiers&&t.motionModifiers.length>0){const C=t.motionModifiers.map(BA);O._updateItem(e,{motionModifiers:n.replace?C:NA(i.motionModifiers,C)}),T+=C.length}if(t.motionLayers&&t.motionLayers.length>0){const C=t.motionLayers.map(R=>({...jM(R,{freshIds:!0}),source:"saved-preset",sourcePresetId:t.id}));O._updateItem(e,{motionLayers:n.replace?C:[...i.motionLayers??[],...C]}),T+=C.length}if(t.textMotion&&i.type==="text"){const C=UA(t.textMotion);O._updateItem(e,{textMotion:n.replace?C:{...i.textMotion,...C}}),T+=+!!C.in,T+=+!!C.out,T+=+!!C.loop}return E>0||d>0||T>0||Me().warn("applyAnimationPreset produced no animation",{targetItemId:e,skipped:w}),{applied:E,addedEffects:d,appliedProcedural:T,skipped:w,incompatible:!1}}const K=I().min(1),tr=P().int().nonnegative(),ie=P().finite(),ar=Wt([U(["x","y","width","height","anchorX","anchorY","rotation","opacity","cornerRadius","cropLeft","cropRight","cropTop","cropBottom","cropSoftness","volume","textStyleScale","fontSize","lineHeight","textPadding","backgroundRadius","textShadowOffsetX","textShadowOffsetY","textShadowBlur","strokeWidth","trimPathStart","trimPathEnd","trimPathOffset","taperStartWidth","taperEndWidth","taperStartLength","taperEndLength"]),I().regex(/^effect:[^:]+:[^:]+:[^:]+$/),I().regex(/^pathVertex:\d+:(positionX|positionY|inX|inY|outX|outY)$/)]),Bt=U(["position","scale","anchor"]),Un=I().refine(bh),m0=I().refine(Oa),Hr=U(["linear","ease-in","ease-out","ease-in-out","hold","cubic-bezier","spring"]),No=y({x:ie,y:ie}),h0=y({type:Hr,bezier:y({x1:ie.min(0).max(1),y1:ie,x2:ie.min(0).max(1),y2:ie}).optional(),spring:y({tension:ie.nonnegative(),friction:ie.nonnegative(),mass:ie.positive()}).optional()}),g0=y({applicationId:K,kind:U(["built-in-preset","saved-preset"]),presetId:K,presetName:I()}),_c=y({frame:tr,value:ie,easing:Hr.optional(),easingConfig:h0.optional(),source:g0.optional()}),mi=y({frame:tr,value:No,easing:Hr.optional(),easingConfig:h0.optional(),temporalEase:y({in:y({speed:ie,influence:ie.min(0).max(100)}).optional(),out:y({speed:ie,influence:ie.min(0).max(100)}).optional()}).optional(),spatial:y({inTangent:No,outTangent:No,continuous:B().optional()}).optional(),source:g0.optional()}),Ml=_c.extend({id:K}),y0=mi.extend({id:K,easing:Hr}),an=y({itemId:K,property:ar,keyframeId:K}),hi=_c.extend({itemId:K,property:ar}),mf=an.extend({updates:_c.partial(),clearFields:F(U(["easingConfig","source"])).optional()}),KA=_a("type",[hi.extend({type:ne("add")}),an.extend({type:ne("update"),updates:y({value:ie.optional()})}),mi.extend({type:ne("vector-add"),itemId:K,property:Bt}),y({type:ne("vector-update"),itemId:K,property:Bt,keyframeId:K,updates:y({value:No})})]);function pt(e){if(!S.getState().itemById[e])throw new Error(`Không tìm thấy clip: ${e}`)}function Q(e,t,r,n,i){A({type:e,params:t,validate:i,mergeKey:n,apply:(o,a)=>({inverse:{kind:"patch",patch:{}},result:r(a)})})}function Ts(e,t=[]){return{...e,...Object.fromEntries(t.map(r=>[r,void 0]))}}function GA(){Q("keyframe.add",hi,r=>(pt(r.itemId),Ps([r])[0]??"")),Q("keyframe.addMany",y({payloads:F(hi)}),r=>{for(const n of r.payloads)pt(n.itemId);return Ps(r.payloads)}),Q("keyframe.update",mf,r=>gA(r.itemId,r.property,r.keyframeId,Ts(r.updates,r.clearFields)),r=>`keyframe:${r.itemId}:${r.property}:${r.keyframeId}`),Q("keyframe.updateMany",y({updates:F(mf)}),r=>ff(r.updates.map(n=>({...n,updates:Ts(n.updates,n.clearFields)}))),r=>`keyframes:${r.updates.map(n=>`${n.itemId}:${n.property}:${n.keyframeId}`).sort().join(",")}`),Q("keyframe.move",y({moves:F(y({ref:an,newFrame:tr}))}),r=>ff(r.moves.map(n=>({...n.ref,updates:{frame:n.newFrame}}))),r=>`keyframes:${r.moves.map(n=>`${n.ref.itemId}:${n.ref.property}:${n.ref.keyframeId}`).sort().join(",")}`),Q("keyframe.remove",an,r=>vA(r.itemId,r.property,r.keyframeId)),Q("keyframe.removeMany",y({refs:F(an)}),r=>kA(r.refs)),Q("keyframe.removeForItem",y({itemId:K}),r=>bA(r.itemId)),Q("keyframe.removeForProperty",y({itemId:K,property:ar}),r=>SA(r.itemId,r.property)),Q("keyframe.removePresetApplication",y({itemId:K,applicationId:K}),r=>IA(r.itemId,r.applicationId)),Q("keyframe.removeManual",y({itemId:K}),r=>xA(r.itemId)),Q("keyframe.trimToBounds",y({itemId:K}),r=>wA(r.itemId)),Q("keyframe.auto",y({operations:F(KA)}),r=>{for(const n of r.operations)pt(n.itemId);return yA(r.operations)}),Q("keyframe.vectorUpsert",y({itemId:K,property:Bt,input:mi}),r=>(pt(r.itemId),cA(r.itemId,r.property,r.input))),Q("keyframe.vectorUpdate",y({itemId:K,property:Bt,keyframeId:K,updates:mi.partial(),clearFields:F(U(["easingConfig","temporalEase","spatial","source"])).optional()}),r=>uA(r.itemId,r.property,r.keyframeId,Ts(r.updates,r.clearFields)),r=>`vector:${r.itemId}:${r.property}:${r.keyframeId}`),Q("keyframe.vectorRemove",y({itemId:K,property:Bt,keyframeId:K}),r=>dA(r.itemId,r.property,r.keyframeId));const e=y({property:Bt,keyframes:F(y0)}),t=y({property:ar,keyframes:F(Ml.extend({easing:Hr}))});Q("keyframe.vectorPromote",y({itemId:K,vectorProperty:e,removeScalarProperties:F(m0)}),r=>(pt(r.itemId),fA(r.itemId,r.vectorProperty,r.removeScalarProperties))),Q("keyframe.vectorSeparate",y({itemId:K,property:Bt,separated:B(),conversion:y({scalarProperties:F(t).optional(),vectorProperty:e.optional()}).optional()}),r=>(pt(r.itemId),pA(r.itemId,r.property,r.separated,r.conversion))),Q("keyframe.setLink",y({itemId:K,link:y({type:ne("link"),targetProperty:Un,sourceItemId:K,sourceProperty:Un,enabled:B(),timeOffsetFrames:P().int()})}),r=>(pt(r.itemId),pt(r.link.sourceItemId),oA(r.itemId,r.link))),Q("keyframe.removeLink",y({itemId:K,property:Un}),r=>aA(r.itemId,r.property)),Q("keyframe.setExpression",y({itemId:K,expression:y({type:ne("expression"),targetProperty:Un,source:I(),enabled:B()})}),r=>(pt(r.itemId),sA(r.itemId,r.expression))),Q("keyframe.removeExpression",y({itemId:K,property:Un}),r=>lA(r.itemId,r.property)),Q("keyframe.applyMotionPreset",y({payloads:F(hi),clearProperties:F(y({itemId:K,property:ar,fromFrame:tr.optional(),toFrame:tr.optional()})).default([]),vectorApplies:F(y({itemId:K,property:Bt,keyframes:F(mi),replaceRange:y({fromFrame:tr,toFrame:tr}).optional()})).default([])}),r=>{for(const n of[...r.payloads.map(i=>i.itemId),...r.vectorApplies.map(i=>i.itemId)])pt(n);return hA(r.payloads,r.clearProperties,r.vectorApplies)}),Q("keyframe.duplicate",y({refs:F(an),frameOffset:P().int(),targetItemId:K.optional(),targetProperty:ar.optional()}),r=>{const n=$.getState(),i=r.refs.flatMap(o=>{const a=n.getKeyframeById(o.itemId,o.property,o.keyframeId);return a?[{...a,itemId:r.targetItemId??o.itemId,property:r.targetProperty??o.property,frame:a.frame+r.frameOffset}]:[]});if(i.some(o=>o.frame<0))throw new Error("Khung hình keyframe không được âm");for(const o of i)pt(o.itemId);return Ps(i)}),WA()}function WA(){const e=U(["float-drift","breath-pulse","micro-shake","sway","spin","wander"]),t=y({version:ne(2).optional(),id:K,type:e,enabled:B(),amplitude:ie,frequency:ie,phaseFrames:P().int(),seed:ie,channelGains:y({x:ie.optional(),y:ie.optional(),width:ie.optional(),height:ie.optional(),rotation:ie.optional(),opacity:ie.optional()}).optional()}),r=y({id:K,name:I(),enabled:B(),source:U(["built-in-preset","saved-preset"]),sourcePresetId:K,tracks:F(y({property:m0,blend:U(["add","multiply"]),keyframes:F(Ml.extend({easing:Hr}))}))}),n=F(y({itemId:K,modifier:t}));Q("keyframe.motionLayersAdd",y({assignments:F(y({itemId:K,layer:r}))}),d=>PA(d.assignments)),Q("keyframe.motionLayerRemove",y({itemIds:F(K),layerId:K}),d=>TA(d.itemIds,d.layerId)),Q("keyframe.motionModifiersSet",y({assignments:n,gestureId:K.optional()}),d=>CA(d.assignments),d=>d.gestureId?`motion:${d.gestureId}`:null),Q("keyframe.motionModifierRemove",y({itemIds:F(K),type:e}),d=>_A(d.itemIds,d.type)),Q("keyframe.audioPulseRemove",y({itemIds:F(K)}),d=>FA(d.itemIds)),Q("keyframe.motionBake",y({plan:F(y({itemId:K,keyframes:F(hi),clearProperties:F(ar),clearMotionModifiers:B(),clearMotionLayers:B(),clearAudioPulseEffectIds:F(K)}))}),d=>MA(d.plan));const i=U(["in","out","loop"]),o=y({durationFrames:ie.nonnegative(),offsetFrames:ie.nonnegative().optional(),staggerFrames:ie.nonnegative(),intensity:ie,order:U(["forward","backward","center","random"]),easing:U(["linear","ease-in","ease-out","ease-in-out","overshoot"]),seed:ie,unit:U(["character","word","line","whole-clip"]).optional()}),a=o.extend({presetId:U(Xh)}),s=o.extend({presetId:U(Qh)}),l=o.extend({presetId:U(Jh)}),c=Wt([a,s,l]);Q("keyframe.textMotionSet",y({itemIds:F(K),slot:i,effect:c}),d=>DA(d.itemIds,d.slot,d.effect)),Q("keyframe.textMotionUpdate",y({itemIds:F(K),slot:i,partial:o.partial(),gestureId:K.optional()}),d=>RA(d.itemIds,d.slot,d.partial),d=>d.gestureId?`text-motion:${d.gestureId}`:null),Q("keyframe.textMotionRemove",y({itemIds:F(K),slot:i}),d=>LA(d.itemIds,d.slot));const u=y({id:K,name:I(),sourceItemType:U(["video","audio","image","lottie","text","shape","adjustment","composition","controller","subtitle"]),properties:F(y({property:ar,keyframes:F(Ml.extend({easing:Hr}))})),vectorProperties:F(y({property:Bt,keyframes:F(y0)})).optional(),effects:F(y({type:ne("gpu-effect"),gpuEffectType:K,params:Ct(I(),Wt([ie,B(),I()]))})),motionModifiers:F(t).optional(),motionLayers:F(r).optional(),textMotion:y({in:a.optional(),out:s.optional(),loop:l.optional()}).optional(),sourceDurationInFrames:P().int().positive(),createdAt:ie});Q("keyframe.applyPreset",y({targetItemId:K,preset:u,anchorFrame:tr.default(0),options:y({replace:B().optional(),retime:B().optional()}).default({})}),d=>qA(d.targetItemId,d.preset,d.anchorFrame,d.options))}function VA(e,t,r="crossfade",n,i,o,a=.5){return(()=>{const s=S.getState().items,l=q.getState().transitions,c=s.find(w=>w.id===e),u=s.find(w=>w.id===t);if(!c||!u)return Me().warn("[addTransition] Clips not found"),!1;const d=Math.floor(Math.min(c.durationInFrames,u.durationInFrames)-1);if(d<1)return Me().warn("[addTransition] Cannot add transition: clips are too short"),!1;const f=yh[r],p=n??f.defaultDuration;let m=Math.max(1,Math.min(Math.round(p),d));const h=c.from+c.durationInFrames,g=Pn(h,u.from),v=W.getState().fps;if(g){const w=Sg(c,u,a,v);if(w<1)return Me().warn("[addTransition] Cannot add transition: insufficient source handle at cut"),!1;m=Math.min(m,w)}const x=Ui(c,u,m,a,v);return x.canAdd?l.find(w=>w.leftClipId===e&&w.rightClipId===t)?(Me().warn("[addTransition] Transition already exists between these clips"),!1):(q.getState()._addTransition(e,t,c.trackId,r,m,i,o,a),!0):(Me().warn("[addTransition] Cannot add transition:",x.reason),!1)})()}function v0(e,t){const n=q.getState().transitions.find(l=>l.id===e);if(!n)return!1;const i=S.getState().items,o=i.find(l=>l.id===n.leftClipId),a=i.find(l=>l.id===n.rightClipId),s={...n,...t};if(o&&a){const l=Ui(o,a,s.durationInFrames,s.alignment,W.getState().fps);if(!l.canAdd)return Me().warn("[updateTransition] Cannot update transition:",l.reason),!1}return q.getState()._updateTransition(e,t),!0}function hf(e,t){v0(e,t)}function ZA(e){e.length!==0&&(()=>{for(const{id:t,updates:r}of e){if(r.durationInFrames!==void 0||r.alignment!==void 0){const n=q.getState().transitions.find(i=>i.id===t);if(n&&(r.durationInFrames!==void 0&&r.durationInFrames!==n.durationInFrames||r.alignment!==void 0&&r.alignment!==n.alignment)){v0(t,r);continue}}q.getState()._updateTransition(t,r)}})()}function YA(e){q.getState()._removeTransition(e)}const mt=I().min(1),ri=P().finite(),b0=U(["from-left","from-right","from-top","from-bottom"]),S0=ri.min(0).max(1),Es=y({durationInFrames:P().int().positive().optional(),type:ne("crossfade").optional(),presentation:mt.optional(),direction:b0.optional(),timing:U(["linear","ease-in","ease-out","ease-in-out","cubic-bezier"]).optional(),alignment:S0.optional(),bezierPoints:y({x1:ri.min(0).max(1),y1:ri,x2:ri.min(0).max(1),y2:ri}).optional(),presetId:mt.optional(),properties:Ct(I(),Fa()).optional()});function Zr(e,t,r,n){A({type:e,params:t,mergeKey:n,apply:(i,o)=>({inverse:{kind:"patch",patch:{}},result:r(o)})})}function XA(){Zr("transition.add",y({leftClipId:mt,rightClipId:mt,type:ne("crossfade").default("crossfade"),durationInFrames:P().int().positive().optional(),presentation:mt.optional(),direction:b0.optional(),alignment:S0.default(.5),timing:Es.shape.timing}),e=>{const t=VA(e.leftClipId,e.rightClipId,e.type,e.durationInFrames,e.presentation,e.direction,e.alignment);if(t&&e.timing){const r=q.getState().transitions.find(n=>n.leftClipId===e.leftClipId&&n.rightClipId===e.rightClipId);r&&hf(r.id,{timing:e.timing})}return t}),Zr("transition.update",y({id:mt,updates:Es}),e=>hf(e.id,e.updates),e=>`transition:${e.id}`),Zr("transition.updateMany",y({updates:F(y({id:mt,updates:Es}))}),e=>ZA(e.updates),e=>`transitions:${e.updates.map(t=>t.id).sort().join(",")}`),Zr("transition.remove",y({id:mt}),e=>YA(e.id)),Zr("transition.removeMany",y({ids:F(mt)}),e=>q.getState()._removeTransitions(e.ids)),Zr("transition.repair",y({changedClipIds:F(mt),deletedClipIds:F(mt).optional()}),e=>{const{valid:t,repaired:r,broken:n}=xg(e.changedClipIds,S.getState().items,q.getState().transitions,e.deletedClipIds?new Set(e.deletedClipIds):void 0,W.getState().fps);return q.getState().setTransitions([...t,...r.map(i=>i.repaired)]),{repaired:r,broken:n}})}const $o=y({type:ne("gpu-effect"),gpuEffectType:I().min(1),params:Ct(I(),Wt([P().finite(),B(),I()]))}).strict(),QA=y({id:I().min(1),effect:$o,enabled:B(),audioPulse:y({enabled:B(),beats:F(y({frame:P().finite(),amplitude:P().finite()}).strict()),durationFrames:P().finite(),strength:P().finite(),chroma:P().finite(),glowColorBase:P().finite()}).strict().optional()}).strict(),gi=I().min(1),_l=y({itemId:gi}).strict(),Cs=_l.extend({effectId:gi}),xr=()=>({kind:"patch",patch:{}});function ni(e,t){return t.flatMap(r=>{const n=e.items.find(i=>i.id===r);return n?n.type==="audio"?[{message:"Clip âm thanh không hỗ trợ hiệu ứng hình ảnh",path:"itemId"}]:[]:[{message:`Không tìm thấy clip ${r}`,path:"itemId"}]})}function Ms(e,t){const r=ni(e,[t.itemId]);return r.length?r:e.items.find(n=>n.id===t.itemId)?.effects?.some(n=>n.id===t.effectId)?[]:[{message:`Không tìm thấy hiệu ứng ${t.effectId}`,path:"effectId"}]}function Nn(e,t,r){e.draft.items=e.draft.items.map(n=>n.id===t&&n.type!=="audio"?r(n):n)}function JA(){A({type:"effect.add",params:_l.extend({effect:$o,effectId:gi.optional()}),validate:(e,t)=>ni(e,[t.itemId]),apply(e,t){const r=t.effectId??e.ids.mint();return Nn(e,t.itemId,n=>({...n,effects:[...n.effects??[],{id:r,effect:t.effect,enabled:!0}]})),{inverse:xr(),result:r}}}),A({type:"effect.addMany",params:y({updates:F(y({itemId:gi,effects:F($o)}).strict())}).strict(),validate:(e,t)=>ni(e,t.updates.map(r=>r.itemId)),apply(e,t){const r=new Map(t.updates.map(n=>[n.itemId,n.effects]));return e.draft.items=e.draft.items.map(n=>{const i=r.get(n.id);return!i||n.type==="audio"?n:{...n,effects:[...n.effects??[],...i.map(o=>({id:e.ids.mint(),effect:o,enabled:!0}))]}}),xr()}}),A({type:"effect.update",params:Cs.extend({updates:y({effect:$o.optional(),enabled:B().optional()}).strict()}),validate:(e,t)=>Ms(e,t),mergeKey:e=>`effect:${e.itemId}:${e.effectId}:${[...Object.keys(e.updates).filter(t=>t!=="effect"),...Object.keys(e.updates.effect?.params??{}).map(t=>`${e.updates.effect?.gpuEffectType}:${t}`)].sort().join(",")}`,apply(e,t){return Nn(e,t.itemId,r=>({...r,effects:(r.effects??[]).map(n=>n.id===t.effectId?{...n,...t.updates}:n)})),xr()}}),A({type:"effect.remove",params:Cs,validate:Ms,apply(e,t){return Nn(e,t.itemId,r=>({...r,effects:(r.effects??[]).filter(n=>n.id!==t.effectId)})),xr()}}),A({type:"effect.toggle",params:Cs,validate:Ms,apply(e,t){return Nn(e,t.itemId,r=>({...r,effects:(r.effects??[]).map(n=>n.id===t.effectId?{...n,enabled:!n.enabled}:n)})),xr()}}),A({type:"effect.setItems",params:y({updates:F(y({itemId:gi,effects:F(QA)}).strict())}).strict(),validate:(e,t)=>ni(e,t.updates.map(r=>r.itemId)),apply(e,t){const r=new Map(t.updates.map(n=>[n.itemId,n.effects]));return e.draft.items=e.draft.items.map(n=>r.has(n.id)&&n.type!=="audio"?{...n,effects:r.get(n.id)}:n),xr()}}),A({type:"effect.setBlendMode",params:_l.extend({blendMode:U(Object.keys(sE))}),validate:(e,t)=>ni(e,[t.itemId]),apply(e,t){return Nn(e,t.itemId,r=>({...r,blendMode:r.type==="shape"&&r.isMask?"normal":t.blendMode})),xr()}})}const re=P().finite(),_s=I().min(1),I0=y({fontSize:re.optional(),fontFamily:I().optional(),fontWeight:U(["normal","medium","semibold","bold"]).optional(),fontStyle:U(["normal","italic"]).optional(),underline:B().optional(),color:I().optional(),letterSpacing:re.optional()}).strict(),xo=I0.extend({text:I()}),Fl=I0.extend({backgroundColor:I().optional(),backgroundRadius:re.optional(),textAlign:U(["left","center","right"]).optional(),verticalAlign:U(["top","middle","bottom"]).optional(),lineHeight:re.optional(),textPadding:re.optional(),textShadow:y({offsetX:re,offsetY:re,blur:re,color:I()}).strict().optional(),stroke:y({width:re,color:I()}).strict().optional()}),gf=Fl.extend({text:I().optional(),textSpans:F(xo).optional(),spanLayout:U(["stack","inline"]).optional(),textLayoutDrafts:y({single:xo.optional(),twoSpans:F(xo).optional(),threeSpans:F(xo).optional()}).strict().optional(),textStylePresetId:I().optional(),textStyleScale:re.optional()}),Ho=ui([re,re]),yf=y({shapeType:U(["rectangle","circle","triangle","ellipse","star","polygon","heart","path"]).optional(),fillColor:I().optional(),fillEnabled:B().optional(),fillType:U(["solid","linear"]).optional(),gradientStartColor:I().optional(),gradientEndColor:I().optional(),gradientAngle:re.optional(),strokeColor:I().optional(),strokeWidth:re.optional(),strokeEnabled:B().optional(),strokeLineCap:U(["butt","round","square"]).optional(),strokeLineJoin:U(["miter","round","bevel"]).optional(),strokeMiterLimit:re.optional(),trimPathStart:re.optional(),trimPathEnd:re.optional(),trimPathOffset:re.optional(),taperStartWidth:re.optional(),taperEndWidth:re.optional(),taperStartLength:re.optional(),taperEndLength:re.optional(),cornerRadius:re.optional(),direction:U(["up","down","left","right"]).optional(),points:re.optional(),innerRadius:re.optional(),pathVertices:F(y({position:Ho,inHandle:Ho,outHandle:Ho,tangentMode:U(["corner","smooth","continuous","broken"]).optional()}).strict()).optional(),pathClosed:B().optional(),isMask:B().optional(),maskType:U(["clip","alpha"]).optional(),maskFeather:re.optional(),maskOpacity:re.optional(),maskInvert:B().optional()}).strict(),vf=y({frameRate:re.optional(),totalFrames:re.optional(),loop:B().optional(),reversed:B().optional(),loopMode:U(["loop","pingpong"]).optional(),speed:re.optional(),segmentStart:re.optional(),segmentEnd:re.optional(),animationId:I().optional(),themeId:I().optional(),textOverrides:Ct(I(),I()).optional(),colorOverrides:Ct(I(),I()).optional(),slotOverrides:Ct(I(),Wt([re,Ho])).optional()}).strict();function Fs(e,t,r){const n=e.items.find(i=>i.id===t);return n?n.type!==r?[{message:`Clip không phải loại ${r}`,path:"itemId"}]:[]:[{message:`Không tìm thấy clip ${t}`,path:"itemId"}]}function As(e,t,r,n){const i={...r};for(const o of n)i[o]=void 0;return e.draft.items=e.draft.items.map(o=>o.id===t?we({...o,...i}):o),{kind:"patch",patch:{}}}function Os(e,t){return Object.keys(e.shape).filter(r=>!t.includes(r))}function eO(){A({type:"text.update",params:y({itemId:_s,updates:gf,clearFields:F(U(Os(gf,["text","color"]))).default([])}).strict(),validate:(e,t)=>Fs(e,t.itemId,"text"),mergeKey:e=>`text:${e.itemId}:${[...Object.keys(e.updates),...e.clearFields].sort().join(",")}`,apply:(e,t)=>As(e,t.itemId,t.updates,t.clearFields)}),A({type:"text.updateShape",params:y({itemId:_s,updates:yf,clearFields:F(U(Os(yf,["shapeType","fillColor"]))).default([])}).strict(),validate:(e,t)=>Fs(e,t.itemId,"shape"),mergeKey:e=>`shape:${e.itemId}:${[...Object.keys(e.updates),...e.clearFields].sort().join(",")}`,apply:(e,t)=>As(e,t.itemId,t.updates,t.clearFields)}),A({type:"text.updateLottie",params:y({itemId:_s,updates:vf,clearFields:F(U(Os(vf,["frameRate","totalFrames"]))).default([])}).strict(),validate:(e,t)=>Fs(e,t.itemId,"lottie"),mergeKey:e=>`lottie:${e.itemId}:${[...Object.keys(e.updates),...e.clearFields].sort().join(",")}`,apply:(e,t)=>As(e,t.itemId,t.updates,t.clearFields)})}const tO=/^(?:(\d{1,2}):)?(\d{1,2}):(\d{2})([,.])(\d{1,3})\s+-->\s+(?:(\d{1,2}):)?(\d{1,2}):(\d{2})([,.])(\d{1,3})(?:\s+.*)?$/;function bf(e){return x0(w0(e).replace(/\r\n?/g,`
`),"srt")}function Sf(e){const r=w0(e).replace(/\r\n?/g,`
`).replace(/^WEBVTT(?:[^\n]*)\n+/,"");return x0(r,"vtt")}function x0(e,t){const r=[],n=[],i=e.split(/\n{2,}/).map(a=>a.trim()).filter(Boolean);for(const[a,s]of i.entries()){const l=s.split(`
`),c=l.findIndex(p=>p.includes("-->"));if(c===-1){iO(l[0]??"",t)||r.push(`Skipped block ${a+1}: missing timestamp`);continue}const u=rO(l[c].trim());if(!u){r.push(`Skipped block ${a+1}: invalid timestamp`);continue}if(u.endSeconds<=u.startSeconds){r.push(`Skipped block ${a+1}: end time must be after start time`);continue}const f=l.slice(c+1).map(p=>nO(p).trimEnd()).join(`
`).trim();if(!f){r.push(`Skipped block ${a+1}: empty cue text`);continue}n.push({id:`cue-${n.length+1}`,startSeconds:u.startSeconds,endSeconds:u.endSeconds,text:f})}const o=[...n].sort((a,s)=>a.startSeconds-s.startSeconds);for(let a=1;a<o.length;a++){const s=o[a-1];o[a].startSeconds<s.endSeconds&&r.push(`Cue ${a+1} overlaps the previous cue`)}return{cues:o,warnings:r}}function rO(e){const t=e.match(tO);if(!t)return null;const r=If(t[1],t[2],t[3],t[5]),n=If(t[6],t[7],t[8],t[10]);return!Number.isFinite(r)||!Number.isFinite(n)?null:{startSeconds:r,endSeconds:n}}function If(e,t,r,n){const i=Number.parseInt(e??"0",10),o=Number.parseInt(t??"0",10),a=Number.parseInt(r??"0",10),s=Number.parseInt((n??"0").padEnd(3,"0").slice(0,3),10);return i*3600+o*60+a+s/1e3}function w0(e){return e.charCodeAt(0)===65279?e.slice(1):e}function nO(e){return e.replace(/<[^>]+>/g,"")}function iO(e,t){return t!=="vtt"?!1:e.startsWith("NOTE")||e==="STYLE"||e==="REGION"}function oO(e){const t=e.reduce((r,n)=>Math.max(r,n.order),-1);return{id:`track-captions-${Date.now()}`,name:Bl([...e],"video"),kind:"video",height:jr,locked:!1,syncLock:!0,visible:!0,muted:!1,solo:!1,volume:0,order:t+1,items:[]}}function aO(e,t,r="transcript",n){return{type:r,mediaId:e,clipId:t,...n}}function sO({trackId:e,cues:t,timelineFps:r,canvasWidth:n,canvasHeight:i,fileName:o,format:a,startFrame:s=0,styleTemplate:l,sourceType:c="subtitle-import",idFactory:u=()=>crypto.randomUUID(),importedAt:d=Date.now()}){return t.flatMap(f=>{const p=s+Math.max(0,Math.round(f.startSeconds*r)),m=Math.max(p+1,s+Math.round(f.endSeconds*r)),h=Math.max(1,m-p);return[{...{id:u(),type:"text",textRole:"caption",trackId:e,from:p,durationInFrames:h,mediaId:"",captionSource:aO("","",c,{fileName:o,format:a,importedAt:d}),label:f.text.slice(0,48),text:f.text,fontSize:Math.max(36,Math.round(i*.045)),fontFamily:"Inter",fontWeight:"semibold",fontStyle:"normal",underline:!1,color:"#ffffff",backgroundColor:"rgba(0, 0, 0, 0.55)",textAlign:"center",verticalAlign:"middle",lineHeight:1.15,letterSpacing:0,textShadow:{offsetX:0,offsetY:3,blur:10,color:"rgba(0, 0, 0, 0.75)"},transform:{x:0,y:Math.round(i*.32),width:n*.82,height:i*.16,rotation:0,opacity:1}},...l}]})}const Jr=I().min(1),sn=P().finite(),xf=y({id:Jr,startSeconds:sn.nonnegative(),endSeconds:sn.nonnegative(),text:I()}).strict().refine(e=>e.endSeconds>e.startSeconds,{message:"Thời điểm kết thúc phải sau thời điểm bắt đầu",path:["endSeconds"]}),wo=()=>({kind:"patch",patch:{}});function zs(e,t){const r=e.items.find(n=>n.id===t.itemId);return r?r.type!=="subtitle"?[{message:"Clip không phải đoạn phụ đề",path:"itemId"}]:[]:[{message:`Không tìm thấy phụ đề ${t.itemId}`,path:"itemId"}]}function lO(){A({type:"subtitle.import",params:y({trackId:Jr,fileName:I().min(1),format:U(["srt","vtt"]),content:I(),startFrame:sn.nonnegative().default(0),importedAt:sn.default(0),createTrack:B().default(!1),style:Fl.optional()}).strict(),validate(e,t){const r=[],n=e.tracks.find(o=>o.id===t.trackId);return!n&&!t.createTrack&&r.push({message:`Không tìm thấy rãnh ${t.trackId}`,path:"trackId"}),(n?.locked||n?.isGroup||n?.kind==="audio")&&r.push({message:"Rãnh này không nhận phụ đề",path:"trackId"}),(t.format==="srt"?bf(t.content):Sf(t.content)).cues.length||r.push({message:"Tệp không có phụ đề hợp lệ",path:"content"}),r},apply(e,t){const r=t.format==="srt"?bf(t.content):Sf(t.content);e.draft.tracks.some(i=>i.id===t.trackId)||(e.draft.tracks=[...e.draft.tracks,{...oO(e.draft.tracks),id:t.trackId}]);const n=sO({trackId:t.trackId,cues:r.cues,timelineFps:e.draft.fps,canvasWidth:e.draft.projectMetadata?.width??1920,canvasHeight:e.draft.projectMetadata?.height??1080,fileName:t.fileName,format:t.format,startFrame:t.startFrame,styleTemplate:t.style?{...t.style,color:t.style.color??"#ffffff"}:void 0,idFactory:()=>e.ids.mint(),importedAt:t.importedAt});return e.draft.items=[...e.draft.items,...Pl(n).map(we)],{inverse:wo(),result:{itemIds:n.map(i=>i.id),warnings:r.warnings}}}}),A({type:"subtitle.setCues",params:y({itemId:Jr,cues:F(xf)}).strict(),validate(e,t){const r=zs(e,t);return new Set(t.cues.map(n=>n.id)).size!==t.cues.length&&r.push({message:"Mã câu phụ đề bị trùng",path:"cues"}),r},apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="subtitle"?{...r,cues:t.cues}:r),wo()}}),A({type:"subtitle.updateCue",params:y({itemId:Jr,cueId:Jr,updates:y({startSeconds:sn.nonnegative().optional(),endSeconds:sn.nonnegative().optional(),text:I().optional()}).strict()}).strict(),mergeKey:e=>`subtitle:${e.itemId}:${e.cueId}:${Object.keys(e.updates).sort().join(",")}`,validate(e,t){const r=zs(e,t);if(r.length)return r;const n=e.items.find(o=>o.id===t.itemId),i=n?.type==="subtitle"?n.cues.find(o=>o.id===t.cueId):void 0;return i?xf.safeParse({...i,...t.updates}).success?[]:[{message:"Thời điểm kết thúc phải sau thời điểm bắt đầu",path:"updates"}]:[{message:`Không tìm thấy câu phụ đề ${t.cueId}`,path:"cueId"}]},apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="subtitle"?{...r,cues:r.cues.map(n=>n.id===t.cueId?{...n,...t.updates}:n)}:r),wo()}}),A({type:"subtitle.setStyle",params:y({itemId:Jr,style:Fl}).strict(),validate:(e,t)=>zs(e,t),mergeKey:e=>`subtitle:style:${e.itemId}:${Object.keys(e.style).sort().join(",")}`,apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="subtitle"?we({...r,...t.style}):r),wo()}})}const cO="ytf-editor-media-sources",uO="ytf-editor-media-source-changed",dO=["thumbnailUrl","waveformData","reverseConformSrc","reverseConformPath","reverseConformKey","reverseConformPreviewSrc","reverseConformPreviewPath","reverseConformPreviewKey","reverseConformPreviewUsesProxy","reverseConformStatus","reverseConformPreviewIsSourceLevel","reverseConformPreviewSourceDuration","reverseConformPreviewFps","reverseConformLocalStart"];function fO(e,t,r,n,i){for(const o of e)if(o.mediaId===t){if(o.type==="shape"){if(!o.waveformVisualizer)continue;if(o.waveformVisualizer.sourceHash!==n){const a={...o.waveformVisualizer};delete a.envelopeBase64,delete a.durationSeconds,delete a.normalization,o.waveformVisualizer=a}continue}for(const a of dO)Reflect.deleteProperty(o,a);o.src=r,(o.type==="video"||o.type==="audio")&&(o.audioSrc=r),i&&(Reflect.set(o,"sourceWidth",i.width),Reflect.set(o,"sourceHeight",i.height),Reflect.set(o,"sourceFps",i.fps),Reflect.set(o,"sourceDuration",Math.round(i.duration*i.fps)))}}function HD(e){if(!(typeof window>"u")&&(window.dispatchEvent(new CustomEvent(uO,{detail:e})),typeof BroadcastChannel<"u")){const t=new BroadcastChannel(cO);t.postMessage(e),t.close()}}const ln=I().min(1),he=P().finite(),pO=y({provider:I(),author:I().optional(),authorUrl:I().optional(),sourceUrl:I().optional(),sourceId:I().optional(),license:I().optional()}).strict(),ko=y({id:ln,storageType:ne("workspace"),contentHash:I().optional(),fileLastModified:he.optional(),fileName:I(),fileSize:he.nonnegative(),mimeType:I(),duration:he,width:he,height:he,fps:he,codec:I(),bitrate:he,audioCodec:I().optional(),audioCodecSupported:B().optional(),videoCodecSupported:B().optional(),previewAudioOpfsPath:I().optional(),previewAudioMimeType:I().optional(),previewAudioConformedAt:he.optional(),keyframeTimestamps:F(he).optional(),gopInterval:he.optional(),thumbnailId:I().optional(),tags:F(I()),attribution:pO.optional(),aiCaptions:F(y({timeSec:he,text:I(),sceneData:y({caption:I().optional(),shotType:I().optional(),subjects:F(I()).optional(),action:I().optional(),setting:I().optional(),lighting:I().optional(),timeOfDay:I().optional(),weather:I().optional()}).strict().optional(),thumbRelPath:I().optional(),embedding:F(he).optional(),palette:F(y({l:he,a:he,b:he,weight:he}).strict()).optional()}).strict()).optional(),createdAt:he,updatedAt:he,background:y({kind:U(["black","green","none"]),keyColor:I().optional(),version:he,detectedAt:he,contentHash:I()}).strict().optional(),overlay:B().optional()}).strict(),mO=y({mediaId:ln.optional(),label:I().optional(),src:I().optional(),audioSrc:I().optional(),thumbnailUrl:I().optional(),waveformData:F(he).optional(),sourceWidth:he.optional(),sourceHeight:he.optional(),sourceFps:he.optional(),sourceDuration:he.optional()}).strict(),hO=U(["src","audioSrc","thumbnailUrl","waveformData","reverseConformSrc","reverseConformPath","reverseConformKey","reverseConformPreviewSrc","reverseConformPreviewPath","reverseConformPreviewKey","reverseConformPreviewUsesProxy","reverseConformStatus","reverseConformPreviewIsSourceLevel","reverseConformPreviewSourceDuration","reverseConformPreviewFps","reverseConformLocalStart"]),wr=()=>({kind:"patch",patch:{}});function gO(e,t){return e.mediaItems?.some(r=>r.id===t.mediaId)?[]:[{message:`Không tìm thấy tệp ${t.mediaId}`,path:"mediaId"}]}function wf(e){const t=qr(e);if(!t.ok)throw new Error(t.issues.map(r=>r.message).join("; "))}function yO(){A({type:"media.add",history:"none",dirty:!0,params:y({media:ko}).strict(),apply(e,t){const r=e.draft.mediaItems??[];return e.draft.mediaItems=r.some(n=>n.id===t.media.id)?r.map(n=>n.id===t.media.id?t.media:n):[t.media,...r],wr()}}),A({type:"media.refreshSource",history:"none",dirty:!1,params:y({media:ko,src:I().min(1)}).strict(),apply(e,t){e.draft.mediaItems=(e.draft.mediaItems??[]).map(i=>i.id===t.media.id?t.media:i);const r={width:t.media.width,height:t.media.height,fps:t.media.fps||e.draft.fps,duration:t.media.duration},n=i=>i.map(o=>{if(o.mediaId!==t.media.id)return o;const a={...o};return fO([a],t.media.id,t.src,t.media.contentHash,r),a});return e.draft.items=n(e.draft.items),e.draft.compositions=e.draft.compositions.map(i=>({...i,items:n(i.items)})),e.draft.stashStack=e.draft.stashStack?.map(i=>({...i,items:n(i.items)})),e.draft.mainHolder&&(e.draft.mainHolder={...e.draft.mainHolder,items:n(e.draft.mainHolder.items)}),wr()}}),A({type:"media.update",history:"none",dirty:!0,params:y({mediaId:ln,updates:ko.omit({id:!0}).partial()}).strict(),validate:(e,t)=>gO(e,t),apply(e,t){return e.draft.mediaItems=(e.draft.mediaItems??[]).map(r=>r.id===t.mediaId?{...r,...t.updates}:r),wr()}}),A({type:"media.setRecords",history:"none",params:y({mediaItems:F(ko),resetImporting:B().default(!1)}).strict(),validate:(e,t)=>new Set(t.mediaItems.map(r=>r.id)).size===t.mediaItems.length?[]:[{message:"Danh sách tệp có mã trùng nhau",path:"mediaItems"}],apply(e,t){return t.resetImporting&&Oe.setState({importingIds:[]}),e.draft.mediaItems=t.mediaItems,wr()}}),A({type:"media.removeRecords",history:"none",dirty:!0,params:y({mediaIds:F(ln)}).strict(),apply(e,t){const r=new Set(t.mediaIds);return e.draft.mediaItems=(e.draft.mediaItems??[]).filter(n=>!r.has(n.id)),wr()}}),A({type:"media.remove",params:y({mediaIds:F(ln)}).strict(),apply(e,t){const r=new Set(t.mediaIds),n=[e.draft.items,...e.draft.compositions.map(o=>o.items),...(e.draft.stashStack??[]).map(o=>o.items),...e.draft.mainHolder?[e.draft.mainHolder.items]:[]],i=[...new Set(n.flatMap(o=>o.filter(a=>a.mediaId&&r.has(a.mediaId)).map(a=>a.id)))];return i.length&&wf([{type:"item.removeProjectItems",params:{itemIds:i}}]),e.draft.mediaItems=(e.draft.mediaItems??[]).filter(o=>!r.has(o.id)),{inverse:wr(),result:i}}}),A({type:"media.relinkItems",params:y({itemIds:F(ln),updates:mO,clearFields:F(hO).default([])}).strict(),validate(e,t){const r=[e.items,...e.compositions.map(i=>i.items),...(e.stashStack??[]).map(i=>i.items),...e.mainHolder?[e.mainHolder.items]:[]],n=new Set(r.flatMap(i=>i.map(o=>o.id)));return t.itemIds.filter(i=>!n.has(i)).map(i=>({message:`Không tìm thấy clip ${i}`,path:"itemIds"}))},apply(e,t){return wf(t.itemIds.map(r=>({type:"item.updateProjectItem",params:{itemId:r,updates:t.updates,clearFields:t.clearFields}}))),wr()}})}const oe=()=>P().finite(),Fc=y({frequencyBins:F(oe().nonnegative()).min(2).max(8193),magnitudes:F(oe().nonnegative()).min(2).max(8193),standardDeviations:F(oe().nonnegative()).min(2).max(8193),sampleRate:oe().int().min(8e3).max(192e3),fftSize:oe().int().min(256).max(16384).refine(e=>(e&e-1)===0)}).strict().superRefine((e,t)=>{const r=e.fftSize/2+1;(e.frequencyBins.length!==r||e.magnitudes.length!==r||e.standardDeviations.length!==r)&&t.addIssue({code:"custom",message:"Số dải tần của mẫu nhiễu không khớp FFT"});for(let n=0;n<e.frequencyBins.length;n++)if(Math.abs(e.frequencyBins[n]-n*e.sampleRate/e.fftSize)>1e-6){t.addIssue({code:"custom",message:"Dải tần của mẫu nhiễu không hợp lệ"});break}}),Or=()=>oe().min(0).max(1),vO=y({threshold:oe().min(-60).max(0),ratio:oe().min(1).max(20),attack:oe().min(.001).max(1),release:oe().min(.01).max(3),knee:oe().min(0).max(40),makeupGain:oe().min(0).max(24)}).strict(),bO=y({threshold:oe().min(-60).max(0),reduction:Or(),smoothing:Or(),profile:Fc.optional()}).strict(),SO=y({roomSize:Or(),damping:Or(),wetLevel:Or(),dryLevel:Or(),preDelay:oe().min(0).max(100)}).strict(),IO=y({time:oe().min(0).max(2),feedback:oe().min(0).max(.95),wetLevel:Or()}).strict(),Po={id:I().min(1),enabled:B()},Ds=_a("type",[y({...Po,type:ne("compressor"),params:vO}).strict(),y({...Po,type:ne("noiseReduction"),params:bO}).strict(),y({...Po,type:ne("reverb"),params:SO}).strict(),y({...Po,type:ne("delay"),params:IO}).strict()]).superRefine((e,t)=>{e.type==="noiseReduction"&&e.enabled&&!e.params.profile&&t.addIssue({code:"custom",message:"Cần lấy mẫu nhiễu trước khi bật giảm nhiễu"})}),xO=y({threshold:oe().optional(),ratio:oe().optional(),attack:oe().optional(),release:oe().optional(),knee:oe().optional(),makeupGain:oe().optional(),reduction:oe().optional(),smoothing:oe().optional(),profile:Fc.optional(),roomSize:oe().optional(),damping:oe().optional(),wetLevel:oe().optional(),dryLevel:oe().optional(),preDelay:oe().optional(),time:oe().optional(),feedback:oe().optional()}).strict(),$n=Wt([y({itemId:I().min(1)}).strict(),y({trackId:I().min(1)}).strict()]);function St(e,t){return"itemId"in t?e.items.find(r=>r.id===t.itemId)?.audioFx:e.tracks.find(r=>r.id===t.trackId)?.audioFx}function Hn(e,t,r){const n="itemId"in t?e.items.find(i=>i.id===t.itemId):e.tracks.find(i=>i.id===t.trackId);return n?"itemId"in t&&"type"in n&&n.type!=="audio"&&n.type!=="video"?[{code:"invalid-target",message:"Hiệu ứng âm thanh chỉ áp dụng cho clip âm thanh hoặc video"}]:r&&!St(e,t)?.some(i=>i.id===r)?[{code:"not-found",message:"Không tìm thấy hiệu ứng âm thanh"}]:[]:[{code:"not-found",message:"Không tìm thấy clip hoặc lớp âm thanh"}]}function jn(e,t,r){"itemId"in t?e.draft.items=e.draft.items.map(n=>n.id===t.itemId?{...n,audioFx:r}:n):e.draft.tracks=e.draft.tracks.map(n=>n.id===t.trackId?{...n,audioFx:r}:n)}const qn=()=>({kind:"patch",patch:{}});function wO(){A({type:"audioFx.add",params:y({target:$n,effect:Ds}).strict(),validate(e,t){const r=Hn(e,t.target);return St(e,t.target)?.some(n=>n.id===t.effect.id)&&r.push({code:"duplicate-id",message:"Mã hiệu ứng âm thanh đã tồn tại"}),r},apply(e,t){return jn(e,t.target,[...St(e.draft,t.target)??[],t.effect]),qn()}}),A({type:"audioFx.remove",params:y({target:$n,effectId:I().min(1)}).strict(),validate:(e,t)=>Hn(e,t.target,t.effectId),apply(e,t){return jn(e,t.target,St(e.draft,t.target).filter(r=>r.id!==t.effectId)),qn()}}),A({type:"audioFx.update",params:y({target:$n,effectId:I().min(1),params:xO.optional(),enabled:B().optional()}).strict(),validate(e,t){const r=Hn(e,t.target,t.effectId),n=St(e,t.target)?.find(o=>o.id===t.effectId);if(!n)return r;const i=Ds.safeParse({...n,enabled:t.enabled??n.enabled,params:{...n.params,...t.params}});return i.success||r.push(...i.error.issues.map(o=>({code:"invalid-effect",message:o.message}))),r},apply(e,t){return jn(e,t.target,St(e.draft,t.target).map(r=>r.id!==t.effectId?r:Ds.parse({...r,enabled:t.enabled??r.enabled,params:{...r.params,...t.params}}))),qn()},mergeKey:e=>`audio-fx:${JSON.stringify(e.target)}:${e.effectId}`}),A({type:"audioFx.reorder",params:y({target:$n,effectIds:F(I().min(1))}).strict(),validate(e,t){const r=Hn(e,t.target),n=St(e,t.target)??[];return(t.effectIds.length!==n.length||new Set(t.effectIds).size!==n.length||t.effectIds.some(i=>!n.some(o=>o.id===i)))&&r.push({code:"invalid-order",message:"Thứ tự phải chứa mỗi hiệu ứng đúng một lần"}),r},apply(e,t){const r=St(e.draft,t.target);return jn(e,t.target,t.effectIds.map(n=>r.find(i=>i.id===n))),qn()}}),A({type:"audioFx.learnNoiseProfile",params:y({target:$n,effectId:I().min(1),profile:Fc,range:y({startSeconds:P().finite().nonnegative(),endSeconds:P().finite().positive()}).strict().refine(e=>e.endSeconds>e.startSeconds)}).strict(),validate(e,t){const r=Hn(e,t.target,t.effectId),n=St(e,t.target)?.find(i=>i.id===t.effectId);return n&&n.type!=="noiseReduction"&&r.push({code:"invalid-effect",message:"Mẫu nhiễu chỉ áp dụng cho hiệu ứng giảm nhiễu"}),(t.range.endSeconds-t.range.startSeconds)*t.profile.sampleRate<t.profile.fftSize&&r.push({code:"invalid-range",message:"Khoảng nhiễu phải chứa ít nhất một khung FFT"}),r},apply(e,t){return jn(e,t.target,St(e.draft,t.target).map(r=>r.id===t.effectId&&r.type==="noiseReduction"?{...r,params:{...r.params,profile:t.profile}}:r)),qn()}})}const k0=y({text:I().min(1),startFrame:P().int().nonnegative(),endFrame:P().int().positive(),estimated:B().optional()}).refine(e=>e.endFrame>e.startFrame,{message:"Mốc kết thúc phải sau mốc bắt đầu."}),kO=U(["none","word-highlight","word-by-word","karaoke","typewriter","bounce","pop","fade"]),PO=y({words:F(k0),estimated:B(),cues:F(y({id:I(),text:I(),startSeconds:P().nonnegative(),endSeconds:P().positive()})),durationInFrames:P().int().nonnegative()});function Kn(e,t,r){const n=e.items.find(i=>i.id===t);return!n||n.type!=="text"&&n.type!=="subtitle"?[{code:"caption-target",message:"Chọn một mục chữ hoặc phụ đề."}]:r?.some((i,o)=>i.endFrame>n.durationInFrames||i.endFrame<=i.startFrame||o>0&&i.startFrame<r[o-1].startFrame)?[{code:"caption-timing",message:"Mốc từ phải tăng dần và nằm trong thời lượng mục."}]:[]}function TO(){A({type:"caption.setWords",params:y({itemId:I(),words:F(k0),estimated:B().optional()}),validate:(e,t)=>Kn(e,t.itemId,t.words),apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&(r.type==="text"||r.type==="subtitle")?{...r,words:t.words,wordsEstimated:t.estimated??!1}:r),{kind:"patch",patch:{}}),mergeKey:e=>`caption-words:${e.itemId}`}),A({type:"caption.updateWord",params:y({itemId:I(),index:P().int().nonnegative(),word:y({text:I().min(1).optional(),startFrame:P().int().nonnegative().optional(),endFrame:P().int().positive().optional()})}),validate:(e,t)=>{const r=Kn(e,t.itemId),n=e.items.find(i=>i.id===t.itemId);return!n||n.type!=="text"&&n.type!=="subtitle"||!n.words?.[t.index]?[...r,{message:"Không tìm thấy từ."}]:Kn(e,t.itemId,n.words.map((i,o)=>o===t.index?{...i,...t.word}:i))},apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&(r.type==="text"||r.type==="subtitle")?{...r,words:r.words?.map((n,i)=>i===t.index?{...n,...t.word}:n)}:r),{kind:"patch",patch:{}}),mergeKey:e=>`caption-word:${e.itemId}:${e.index}`}),A({type:"caption.setPreset",params:y({itemId:I(),preset:kO}),validate:(e,t)=>Kn(e,t.itemId),apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&(r.type==="text"||r.type==="subtitle")?{...r,captionPreset:t.preset}:r),{kind:"patch",patch:{}})}),A({type:"caption.importTimed",params:y({itemId:I(),data:PO}),validate:(e,t)=>Kn(e,t.itemId,t.data.words),apply:(e,t)=>(e.draft.items=e.draft.items.map(r=>{if(r.id!==t.itemId||r.type!=="text"&&r.type!=="subtitle")return r;const n={...r,words:t.data.words,wordsEstimated:t.data.estimated};if(n.type==="subtitle")return{...n,cues:t.data.cues};if(!t.data.cues.length)return{...n,text:t.data.words.map(m=>m.text).join(" ")};const{text:i,textSpans:o,spanLayout:a,textLayoutDrafts:s,textStylePresetId:l,textStyleScale:c,textMotion:u,textRole:d,captionSource:f,...p}=n;return{...p,type:"subtitle",cues:t.data.cues,source:{type:"subtitle-import",fileName:f?.fileName??r.label,format:f?.format??(r.label.toLowerCase().endsWith(".srt")?"srt":"vtt"),importedAt:f?.importedAt??0}}}),{kind:"patch",patch:{}})})}const EO=7,CO=42,Rs=350,MO=e=>/<|\{|\}/.test(e);function P0(e,t,r,n){const i=e.reduce((s,l)=>s+l.length,0);if(i===0||r<=t)return[];const o=[];let a=0;for(const[s,l]of e.entries()){const c=t+Math.floor((r-t)*a/i);a+=l.length;const u=s===e.length-1?r:t+Math.floor((r-t)*a/i);u>c&&o.push({text:l,startFrame:c,endFrame:u,estimated:n})}return o}function _O(e,t,r){const n=e.text.trim().split(/\s+/).filter(Boolean);return n.length===0?[]:n.length===1?[{text:n[0],startFrame:t,endFrame:r,estimated:e.estimated}]:P0(n,t,r,e.estimated===!0)}function FO(e,t){const r=Math.max(0,e.durationInFrames),n=e.cues;if(n?.length){const a=[...n].filter(l=>l.endSeconds>l.startSeconds&&l.endSeconds>0).sort((l,c)=>l.startSeconds-c.startSeconds),s=[];for(const[l,c]of a.entries()){const u=Math.max(0,Math.round(c.startSeconds*t)),d=a[l+1]?Math.round(a[l+1].startSeconds*t):r,f=Math.min(Math.round(c.endSeconds*t),d,r);f>u&&s.push({id:c.id,text:c.text,startFrame:u,endFrame:f})}return s}const i=e.words??[];if(i.length===0)return[];const o=Math.min(...i.map(a=>a.startFrame));return[{id:"words",text:i.map(a=>a.text.trim()).filter(Boolean).join(" "),startFrame:Math.max(0,o),endFrame:r}]}function AO(e,t){const r=FO(e,t),n=[...e.words??[]].sort((s,l)=>s.startFrame-l.startFrame||s.endFrame-l.endFrame),i=[];let o=e.wordsEstimated===!0,a=0;for(const s of r){for(;a<n.length&&n[a].startFrame<s.startFrame;)o=!0,a++;const l=[];for(;a<n.length&&n[a].startFrame<s.endFrame;)l.push(n[a++]);const c=[];if(l.length===0)s.text.trim()&&!MO(s.text)&&(c.push(...P0(s.text.trim().split(/\s+/).filter(Boolean),s.startFrame,s.endFrame,!0)),c.length&&(o=!0));else for(const p of l){const m=Math.min(Math.max(p.startFrame,s.startFrame),s.endFrame),h=Math.min(Math.max(p.endFrame,s.startFrame),s.endFrame);if(h<=m){o=!0;continue}const g=_O(p,m,h);if(g.length===0){o=!0;continue}c.push(...g)}const u=[];let d=[],f=0;for(const p of c){const m=p.text.length+(d.length>0?1:0),h=d.at(-1),g=h&&!e.wordsEstimated&&!h.estimated&&!p.estimated&&(p.startFrame-h.endFrame)*1e3/t>=Rs;d.length>0&&(g||d.length>=EO||f+m>CO)&&(u.push(d),d=[],f=0),d.push(p),f+=p.text.length+(d.length>1?1:0)}if(d.length>0&&u.push(d),u.length===0){if(!s.text.trim())continue;i.push({id:s.id,text:s.text,startFrame:s.startFrame,endFrame:s.endFrame,words:[],estimated:e.wordsEstimated===!0});continue}for(const[p,m]of u.entries()){const h=m[0].startFrame,g=u[p+1]?.[0],v=m.at(-1),b=g&&!e.wordsEstimated&&!v.estimated&&!g.estimated&&(g.startFrame-v.endFrame)*1e3/t>=Rs?v.endFrame:g?.startFrame??s.endFrame;i.push({id:p===0?s.id:`${s.id}.${p+1}`,text:m.map(w=>w.text).join(" "),startFrame:h,endFrame:b,words:m,estimated:e.wordsEstimated===!0||m.some(w=>w.estimated===!0)})}}a<n.length&&(o=!0);for(let s=i.length-1;s>=0;s--){const l=i[s];if(l.endFrame>l.startFrame)continue;const c=i[s-1]??i[s+1];c&&c.words.length>0&&l.words.length>0&&(c.words.push(...l.words),c.text=c.words.map(u=>u.text).join(" "),c.estimated=c.estimated||l.estimated),i.splice(s,1)}for(let s=0;s+1<i.length;s++){const l=i[s+1].startFrame-i[s].endFrame;l>0&&l*1e3/t<Rs&&(i[s].endFrame=i[s+1].startFrame)}return{pages:i,degraded:o}}function OO(e,t){if(!t.idPrefix||!Number.isInteger(t.durationInFrames)||t.durationInFrames<=0||t.fps<=0||t.width<=0||t.height<=0)throw new Error("Thông số mẫu không hợp lệ.");if(t.sections.some(r=>r.from<0||r.durationInFrames<=0||r.from+r.durationInFrames>t.durationInFrames))throw new Error("Phân đoạn phải nằm trong thời lượng mẫu.");return e.resolve(t)}function zO(e,t,r,n,i){const o=[];for(const a of["width","height"]){const s=a==="width"?t.width:t.height;o.push({type:"keyframe.add",params:{itemId:e,property:a,frame:0,value:s*n,easing:"linear"}},{type:"keyframe.add",params:{itemId:e,property:a,frame:r-1,value:s*i,easing:"linear"}})}return o}function Ls(e,t){const r=[],n=e.idPrefix,i=e.from??0,o=(u,d,f,p="video")=>{const m={id:`${n}-${u}`,name:d,order:f,height:64,locked:!1,visible:!0,muted:!1,solo:!1,kind:p};return r.push({type:"track.add",params:m}),m.id},a=o("visuals","Hình ảnh",0),s=o("titles","Tiêu đề",1),l=o("captions","Phụ đề",2),c=u=>r.push({type:"item.add",params:{item:u}});for(const[u,d]of e.sections.entries()){if(d.image){const f=`${n}-still-${u}`;c({id:f,type:"image",trackId:a,from:i+d.from,durationInFrames:d.durationInFrames,label:d.title,mediaId:d.image.mediaId,src:d.image.src,transform:{x:0,y:0,width:e.width,height:e.height}}),r.push(...zO(f,{width:e.width,height:e.height},d.durationInFrames,1,1.12))}if(t!=="narration"){const f=t==="documentary";c({id:`${n}-title-${u}`,type:"text",trackId:s,from:i+d.from,durationInFrames:Math.min(d.durationInFrames,Math.round(e.fps*(f?6:3))),label:d.title,text:f?d.lowerThird??d.title:`${u+1}. ${d.title}`,fontSize:e.height*(f?.05:.08),color:"#ffffff",backgroundColor:"#111827",textAlign:f?"left":"center",verticalAlign:"middle",transform:{x:f?-e.width*.2:0,y:f?e.height*.3:0,width:e.width*(f?.5:.8),height:e.height*.18}})}}if(e.narration){const u=o("narration","Lời dẫn",3,"audio");c({id:`${n}-voice`,type:"audio",trackId:u,from:i,durationInFrames:e.durationInFrames,label:"Lời dẫn",src:e.narration.src,mediaId:e.narration.mediaId})}if(e.words?.length){const{pages:u,degraded:d}=AO({words:e.words,wordsEstimated:e.wordsEstimated,durationInFrames:e.durationInFrames},e.fps),f=e.wordsEstimated===!0||d,p=u.map((h,g)=>({id:`${n}-cue-${g}`,text:h.text,startSeconds:h.startFrame/e.fps,endSeconds:h.endFrame/e.fps})),m=u.flatMap(h=>h.words.map(g=>({text:g.text,startFrame:g.startFrame,endFrame:g.endFrame,...g.estimated?{estimated:!0}:{}})));c({id:`${n}-caption`,type:"subtitle",trackId:l,from:i,durationInFrames:e.durationInFrames,label:"Phụ đề theo từ",source:{type:"subtitle-import",fileName:"captions.vtt",format:"vtt",importedAt:0},cues:p,words:m,wordsEstimated:f,captionPreset:f?"fade":"karaoke",color:"#ffffff",fontSize:e.height*.055,textAlign:"center",verticalAlign:"middle",backgroundColor:"#111827",transform:{x:0,y:e.height*.35,width:e.width*.85,height:e.height*.2}})}return r}const DO=[{id:"narration-ken-burns-captions",name:"Lời dẫn và ảnh chuyển động",description:"Video dài với lời dẫn, ảnh Ken Burns và phụ đề karaoke theo từ.",category:"narration",resolve:e=>Ls(e,"narration")},{id:"listicle-sections",name:"Danh sách theo phân đoạn",description:"Mỗi mục có thẻ tiêu đề đánh số, hình ảnh chuyển động và phụ đề.",category:"listicle",resolve:e=>Ls(e,"listicle")},{id:"documentary-lower-thirds",name:"Tư liệu và chú thích",description:"Video tư liệu dài với chú thích lower-third, lời dẫn và phụ đề.",category:"documentary",resolve:e=>Ls(e,"documentary")}],RO=Object.fromEntries(DO.map(e=>[e.id,e]));function kf(e){return RO[e]}const Pf={introSec:.4,outroSec:.3},T0=y({idPrefix:I().min(1),trackIds:ui([I().min(1),I().min(1),I().min(1)]),from:P().int().nonnegative(),durationInFrames:P().int().positive(),fps:P().positive(),width:P().positive(),height:P().positive(),label:I().optional(),accentColor:I().regex(/^#[a-fA-F0-9]{6}$/).optional(),textColor:I().regex(/^#[a-fA-F0-9]{6}$/).optional(),x:P().finite().optional(),y:P().finite().optional()}).strict();function LO(e,t){const r=Math.max(1,Math.min(Math.round(Pf.introSec*t),Math.max(1,e-1))),n=Math.max(e>r?1:0,Math.min(Math.round(Pf.outroSec*t),e-r)),i=Math.max(0,e-r-n);return{introFrames:r,holdFrames:i,outroFrames:n}}function BO(e){const t=Math.round(e.height*.064),r=Math.round(e.width*.156),n=Math.round(e.width*.033),i=e.x??e.width/2-n-r/2,o=e.y??-(e.height/2)+n+t/2,a=Math.round(t*.32);return{buttonWidth:r,buttonHeight:t,margin:n,x:i,y:o,pad:a}}function UO(e){const t=T0.parse(e),r=t.idPrefix,n=BO(t),i=LO(t.durationInFrames,t.fps),o=t.accentColor??"#cc3327",a=t.textColor??"#ffffff",s=t.label??"Subscribe",l=`${r}-subscribe-group`,c={from:t.from,durationInFrames:t.durationInFrames,linkedGroupId:l},u={...c,id:`${r}-subscribe-backdrop`,type:"shape",shapeType:"rectangle",label:"Nền nút Subscribe",trackId:t.trackIds[0],fillColor:o,cornerRadius:n.buttonHeight/2,transform:{x:n.x,y:n.y,width:n.buttonWidth,height:n.buttonHeight}},d=Math.round(n.buttonHeight*.46),f={...c,id:`${r}-subscribe-play`,type:"shape",shapeType:"triangle",direction:"right",label:"Biểu tượng phát",trackId:t.trackIds[1],fillColor:a,transform:{x:n.x-n.buttonWidth/2+n.pad+d/2,y:n.y,width:d,height:d}},p={...c,id:`${r}-subscribe-text`,type:"text",label:"Chữ Subscribe",trackId:t.trackIds[2],text:s,color:a,fontSize:Math.round(n.buttonHeight*.5),fontWeight:"semibold",textAlign:"center",verticalAlign:"middle",transform:{x:n.x+(n.pad+d)/2,y:n.y,width:n.buttonWidth-n.pad-d,height:n.buttonHeight}},m=[{type:"item.add",params:{item:u}},{type:"item.add",params:{item:f}},{type:"item.add",params:{item:p}}],{holdFrames:h,outroFrames:g}=i,v=n.buttonWidth+n.margin,x=t.durationInFrames-1,b=Math.min(i.introFrames,x),w=Math.min(x,t.durationInFrames-g);for(const E of[u,f,p]){const k=E.transform.x,T=[[0,0],[b,1]];h>0&&T.push([w,1]),T.push([x,0]);const O=[[0,k+v],[b,k]];h>0&&O.push([w,k]),O.push([x,k]);const C=(R,M)=>{const D=[];for(const L of M){const H=D[D.length-1];H&&H[0]===L[0]?H[1]=L[1]:D.push([...L])}for(const[L,H]of D)m.push({type:"keyframe.add",params:{itemId:E.id,property:R,frame:L,value:H,easing:L===b?"ease-out":"linear"}})};C("opacity",T),C("x",O)}return{commands:m,itemIds:[u.id,f.id,p.id],groupId:l,timing:i}}const Tf=y({mediaId:I(),src:I()}),NO=y({idPrefix:I().min(1),fps:P().positive(),width:P().positive(),height:P().positive(),durationInFrames:P().int().positive(),from:P().int().nonnegative().optional(),title:I().optional(),narration:Tf.optional(),sections:F(y({title:I(),from:P().int().nonnegative(),durationInFrames:P().int().positive(),image:Tf.optional(),lowerThird:I().optional()})),words:F(y({text:I(),startFrame:P().int().nonnegative(),endFrame:P().int().positive(),estimated:B().optional()})).optional(),wordsEstimated:B().optional()});function Ef(e,t){for(const r of t){const n=vl(r.type);if(!n)throw new Error(`Lệnh chưa đăng ký: ${r.type}`);const i=n.params.parse(r.params),o=n.validate?.(e.draft,i)??[];if(o.length)throw new Error(o.map(a=>a.message).join("; "));n.apply(e,i)}}function $O(){A({type:"template.apply",params:y({templateId:I(),context:NO}),validate:(e,t)=>kf(t.templateId)?[]:[{code:"template-missing",message:"Không tìm thấy mẫu."}],apply:(e,t)=>{const r=kf(t.templateId);if(!r)throw new Error("Không tìm thấy mẫu.");const n=OO(r,t.context);return Ef(e,n),{inverse:{kind:"patch",patch:{}},result:{templateId:r.id,commands:n}}}}),A({type:"template.subscribe",params:T0,validate:(e,t)=>t.trackIds.map(r=>e.tracks.find(n=>n.id===r)).flatMap((r,n)=>r&&r.kind!=="audio"&&!r.isGroup?[]:[{code:"subscribe-track",message:`Rãnh Subscribe ${n+1} không tồn tại hoặc là rãnh âm thanh.`,path:`trackIds.${n}`}]),apply:(e,t)=>{const{commands:r,itemIds:n,groupId:i,timing:o}=UO(t);return Ef(e,r),{inverse:{kind:"patch",patch:{}},result:{itemIds:n,groupId:i,timing:o,commands:r}}}})}const Yr=I().min(1);function Cf(e,t){const r=e.items.find(n=>n.id===t);return r?.type==="shape"&&r.waveformVisualizer?[]:[{path:"itemId",message:"Không tìm thấy mục Sóng giọng"}]}function Mf(e,t,r){const n=e.mediaItems?.find(i=>i.id===t);return!n||!n.mimeType.startsWith("audio/")?[{path:"mediaId",message:"Chọn tệp âm thanh làm nguồn giọng"}]:n.contentHash===r?[]:[{path:"sourceHash",message:"Dữ liệu sóng giọng không khớp hash nguồn hiện tại"}]}function HO(){A({type:"waveform.add",params:y({id:Yr.optional(),trackId:Yr,from:P().int().nonnegative(),durationInFrames:P().int().positive(),mediaId:Yr,envelope:Cr,style:wl.partial().optional(),transform:y({x:P().finite().optional(),y:P().finite().optional(),width:P().positive().optional(),height:P().positive().optional()}).strict().optional()}).strict(),validate:(e,t)=>Mf(e,t.mediaId,t.envelope.sourceHash),apply(e,t){const r={id:t.id??e.ids.mint(),type:"shape",shapeType:"rectangle",label:"Sóng giọng",trackId:t.trackId,from:t.from,durationInFrames:t.durationInFrames,mediaId:t.mediaId,fillColor:t.style?.color??vd.color,sourceStart:0,sourceEnd:t.durationInFrames,sourceFps:e.draft.fps,sourceDuration:t.envelope.durationSeconds*e.draft.fps,transform:{width:960,height:160,x:0,y:360,...t.transform},waveformVisualizer:{version:1,...vd,...t.style,...t.envelope}},n=qr([{type:"item.add",params:{item:r}}]);if(!n.ok)throw new Error(n.issues.map(i=>i.message).join("; "));return{inverse:{kind:"patch",patch:{}},result:r.id}}}),A({type:"waveform.setStyle",params:y({itemId:Yr,style:wl.partial()}).strict(),validate:(e,t)=>Cf(e,t.itemId),mergeKey:e=>`waveform.setStyle:${e.itemId}:${Object.keys(e.style).sort().join(",")}`,apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="shape"&&r.waveformVisualizer?{...r,waveformVisualizer:{...r.waveformVisualizer,...t.style}}:r),{kind:"patch",patch:{}}}}),A({type:"waveform.setEnvelope",params:y({itemId:Yr,mediaId:Yr.optional(),envelope:Cr}).strict(),validate(e,t){const r=e.items.find(n=>n.id===t.itemId);return[...Cf(e,t.itemId),...Mf(e,t.mediaId??r?.mediaId??"",t.envelope.sourceHash)]},apply(e,t){return e.draft.items=e.draft.items.map(r=>r.id===t.itemId&&r.type==="shape"&&r.waveformVisualizer?{...r,mediaId:t.mediaId??r.mediaId,sourceDuration:t.envelope.durationSeconds*(r.sourceFps??e.draft.fps),waveformVisualizer:{...r.waveformVisualizer,...t.envelope}}:r),{kind:"patch",patch:{}}}})}function jO(){IF(),xF(),o3(),a3(),pF(),iA(),GA(),XA(),JA(),yF(),eO(),lO(),yO(),wO(),TO(),$O(),HO()}jO();const qO=[{description:"Media files",accept:{"video/*":[".mp4",".webm",".mov",".avi",".mkv"],"audio/*":[".mp3",".wav",".ogg",".m4a",".aac"],"image/*":[".jpg",".jpeg",".png",".gif",".webp",".svg"],"application/lottie+json":[".json",".lottie"]}}];function KO(e={}){const{promise:t,resolve:r,reject:n}=Promise.withResolvers(),i=document.createElement("input");i.type="file",i.multiple=e.multiple??!0,i.accept=Object.values(qO[0].accept).flat().join(","),i.hidden=!0;const o=()=>i.remove();return i.addEventListener("change",()=>{const a=Array.from(i.files??[]);o(),a.length?r(a):n(new DOMException("Đã hủy chọn media","AbortError"))},{once:!0}),i.addEventListener("cancel",()=>{o(),n(new DOMException("Đã hủy chọn media","AbortError"))},{once:!0}),document.body.append(i),i.click(),t}function Bs(e,t){return`${e}:${t}`}const Gi=_e()(e=>({tasks:new Map,queueTask:(t,r)=>e(n=>{const i=Bs(t,r),o=new Map(n.tasks);return o.set(i,{id:i,mediaId:t,type:r,status:"queued",progress:0,updatedAt:Date.now()}),{tasks:o}}),updateTask:(t,r,n)=>e(i=>{const o=Bs(t,r),a=i.tasks.get(o);if(!a)return i;const s=new Map(i.tasks);return s.set(o,{...a,...n,progress:Math.max(0,Math.min(1,n.progress??a.progress)),updatedAt:Date.now()}),{tasks:s}}),completeTask:(t,r)=>e(n=>{const i=new Map(n.tasks);return i.delete(Bs(t,r)),{tasks:i}}),clearMedia:t=>e(r=>{const n=new Map(r.tasks);for(const[i,o]of n)o.mediaId===t&&n.delete(i);return{tasks:n}}),clearAll:()=>e({tasks:new Map})})),ii=He("MediaImport"),GO=2;function WO(e,t){const r=Date.now();return{id:t,storageType:"workspace",fileName:e.name,fileSize:e.size,fileLastModified:e.lastModified,mimeType:ql(e),duration:0,width:0,height:0,fps:30,codec:"importing...",bitrate:0,tags:[],createdAt:r,updatedAt:r}}function VO(e){const t={...e};return delete t.isDuplicate,delete t.hasUnsupportedCodec,t}function E0(e){const t=qr([{type:"media.add",params:{media:VO(e)}}]);if(!t.ok)throw new Error(t.issues.map(r=>r.message).join("; ")||t.error)}function C0(e,t){Gi.getState().clearMedia(t),e(r=>({mediaItems:r.mediaItems.filter(n=>n.id!==t),importingIds:r.importingIds.filter(n=>n!==t)}))}function ZO(e,t,r,n){C0(e,r);const i=t().mediaItems.some(o=>o.id===n.id);return E0(n),i}function _f(e,t){E0(t),e({error:null,errorLink:null})}function M0(e){Mt.canGenerateProxy(e.mimeType)&&Mt.setProxyKey(e.id,sh(e))}function YO(e){const t=Gi.getState();t.queueTask(e,"import"),t.updateTask(e,"import",{status:"queued",progress:.05})}function XO(e){Gi.getState().updateTask(e,"import",{status:"running",progress:.2})}function Ff(e,t,r,n,i){const o=[],a=[],s=[];let l=0,c=0;return e.forEach((u,d)=>{const f=t[d];if(f)if(u.status==="fulfilled"){const{metadata:p,tempId:m,file:h}=u.value,g=ZO(r,n,m,p);p.isDuplicate&&g?(a.push(h.name),i?.includeDuplicatesInResults&&o.push(p)):(M0(p),o.push(p),l+=1,p.hasUnsupportedCodec&&p.audioCodec&&s.push({fileName:h.name,audioCodec:p.audioCodec}))}else c++,C0(r,f.tempId),ii.error(`Failed to import ${f.file.name}`,u.reason)}),{results:o,importedCount:l,duplicateNames:a,unsupportedCodecFiles:s,failedCount:c}}function QO(e){return e.length===0?"":e.length<=3?e.join(", "):`${e.slice(0,3).join(", ")} và ${e.length-3} tệp khác`}function JO({importedCount:e,duplicateNames:t,unsupportedCodecFiles:r,failedCount:n}){if(!(t.length>0||r.length>0||n>0))return null;const o=[];if(e>0&&o.push(`Đã nhập ${e} tệp.`),t.length>0&&(e===0&&r.length===0&&n===0?o.push(t.length===1?`"${t[0]}" đã có trong thư viện`:`${t.length} tệp đã có trong thư viện`):t.length===1?o.push(`Đã bỏ qua 1 tệp trùng: ${t[0]}.`):o.push(`Đã bỏ qua ${t.length} tệp trùng: ${QO(t)}.`)),r.length>0){const a=[...new Set(r.map(s=>s.audioCodec))].join(", ");o.push(`${r.length} tệp có codec âm thanh không được hỗ trợ (${a}). Có thể không có dạng sóng âm thanh.`)}return n>0&&o.push(n===1?"1 tệp nhập thất bại. Hãy kiểm tra tệp rồi thử lại.":`${n} tệp nhập thất bại. Hãy kiểm tra các tệp rồi thử lại.`),o.length>0?o.join(" "):null}function Xr(e,t,r,n,i){const o=JO({importedCount:e,duplicateNames:t,unsupportedCodecFiles:r,failedCount:n});if(!o)return;const a=n>0||r.length>0?"warning":"info";i().showNotification({type:a,message:o})}function ez(e,t){const r=o=>{const a=[];for(const s of o){const l=crypto.randomUUID(),c=WO(s,l);e(u=>({mediaItems:[c,...u.mediaItems],importingIds:[...u.importingIds,l],error:null})),YO(l),a.push({tempId:l,file:s})}return a},n=async(o,a,s)=>{const l=new Array(o.length);let c=0;const{mediaLibraryService:u}=await s,d=async()=>{for(;c<o.length;){const p=c++,m=o[p];if(m)try{XO(m.tempId);const h=await u.importFile(m.file,{projectId:a});l[p]={status:"fulfilled",value:{metadata:h,tempId:m.tempId,file:m.file}}}catch(h){l[p]={status:"rejected",reason:h}}}},f=Math.min(GO,o.length);return await Promise.all(Array.from({length:f},d)),l},i=async(o,a)=>{const{currentProjectId:s}=t();if(!s)return e({error:"Chưa chọn dự án"}),[];const l=rr(),c=ii.startEvent("import",l);c.merge({source:"drag-drop",projectId:s,fileCount:o.length});const u=Tt(),d=r(o),f=await n(d,s,u),{results:p,importedCount:m,duplicateNames:h,unsupportedCodecFiles:g,failedCount:v}=Ff(f,d,e,t,a);if(Xr(m,h,g,v,t),a?.waitForPreparation&&p.length>0){const{mediaLibraryService:x}=await u;await x.waitForMediaPreparation(p.map(b=>b.id))}return c.success({imported:m,duplicates:h.length,failed:v,unsupportedCodecs:g.length}),p};return{importMedia:async()=>{const{currentProjectId:o}=t();if(!o)return e({error:"Chưa chọn dự án"}),[];const a=rr(),s=ii.startEvent("import",a);s.set("source","picker"),s.set("projectId",o);try{const l=await KO({multiple:!0});s.set("fileCount",l.length);const c=Tt(),u=r(l),d=await n(u,o,c),{results:f,importedCount:p,duplicateNames:m,unsupportedCodecFiles:h,failedCount:g}=Ff(d,u,e,t);return Xr(p,m,h,g,t),s.success({imported:p,duplicates:m.length,failed:g,unsupportedCodecs:h.length}),f}catch(l){return l instanceof Error&&l.name!=="AbortError"?(e({error:l.message}),s.failure(l)):s.success({outcome:"cancelled",imported:0,duplicates:0,failed:0,unsupportedCodecs:0}),[]}},importMediaFromUrl:async o=>{const{currentProjectId:a}=t(),s=o.trim();if(!a)return e({error:"Chưa chọn dự án",errorLink:null}),[];if(s.length===0)return e({error:"Hãy nhập URL media.",errorLink:null}),[];e({error:null,errorLink:null});const l=rr(),c=ii.startEvent("import",l);c.set("source","url"),c.set("projectId",a);try{const u=new URL(s);c.set("urlHost",u.hostname)}catch{c.set("urlHost","invalid")}try{const{mediaLibraryService:u}=await Tt(),d=await u.importMediaFromUrl(s,a);if(d.isDuplicate)return Xr(0,[d.fileName],[],0,t),c.success({imported:0,duplicates:1,failed:0,unsupportedCodecs:0}),[];_f(e,d),M0(d);const f=d.hasUnsupportedCodec&&d.audioCodec?[{fileName:d.fileName,audioCodec:d.audioCodec}]:[];return Xr(1,[],f,0,t),c.success({imported:1,duplicates:0,failed:0,unsupportedCodecs:f.length}),[d]}catch(u){const d=u instanceof Error?u:new Error(String(u));return e({error:d.message,errorLink:null}),c.failure(d),[]}},importRemoteLottie:async o=>{const{currentProjectId:a}=t();if(!a)return e({error:"Chưa chọn dự án",errorLink:null}),null;e({error:null,errorLink:null});const s=rr(),l=ii.startEvent("import",s);l.set("source","lottiefiles"),l.set("projectId",a),l.set("provider",o.attribution?.provider??"unknown");try{const{mediaLibraryService:c}=await Tt(),u=await c.importLottieFromUrl(o.url,a,{fileName:o.fileName,attribution:o.attribution});return u.isDuplicate?(Xr(0,[u.fileName],[],0,t),l.success({imported:0,duplicates:1,failed:0,unsupportedCodecs:0}),u):(_f(e,u),Xr(1,[],[],0,t),l.success({imported:1,duplicates:0,failed:0,unsupportedCodecs:0}),u)}catch(c){const u=c instanceof Error?c:new Error(String(c));return e({error:u.message,errorLink:null}),l.failure(u),null}},importFiles:o=>i(o),importFilesForPlacement:async o=>i(o,{includeDuplicatesInResults:!0,waitForPreparation:!0})}}const Af=He("BlobUrlManager");class tz{entries=new Map;version=0;listeners=new Set;notify(){this.version++;for(const t of this.listeners)t()}subscribe=t=>(this.listeners.add(t),()=>this.listeners.delete(t));getSnapshot=()=>this.version;acquire(t,r,n){const i=this.entries.get(t);if(i)return i.refCount++,n&&i.blob&&(i.metadata=n,Xs(i.url,i.blob,n)),i.url;const o=URL.createObjectURL(r);return Xs(o,r,n),this.entries.set(t,{url:o,refCount:1,blob:r,metadata:n}),this.notify(),o}registerUrl(t,r){const n=this.entries.get(t);return n?(n.refCount++,n.url):(this.entries.set(t,{url:r,refCount:1,external:!0}),this.notify(),r)}get(t){return this.entries.get(t)?.url??null}has(t){return this.entries.has(t)}getMediaIdByUrl(t){for(const[r,n]of this.entries)if(n.url===t)return r;return null}invalidate(t){const r=this.entries.get(t);r&&(this.revokeEntry(r),this.entries.delete(t),this.notify())}revokeEntry(t){t.external||(im(t.url),URL.revokeObjectURL(t.url))}release(t){const r=this.entries.get(t);r&&(r.refCount--,r.refCount<=0&&(this.revokeEntry(r),this.entries.delete(t),this.notify(),Af.debug(`Revoked blob URL for media ${t}`)))}invalidateAll(){for(const t of this.entries.values())this.revokeEntry(t);this.entries.clear(),this.notify()}releaseAll(){for(const[t,r]of this.entries)this.revokeEntry(r),Af.debug(`Revoked blob URL for media ${t}`);this.entries.clear(),this.notify()}get size(){return this.entries.size}}const _i=new tz,rz=new Set;function nz(e,t=[]){for(const r of rz)r(e,t)}function _0(e){const t=qr([e]);if(!t.ok)throw new Error(t.issues.map(r=>r.message).join("; "))}function iz(e,t){_0({type:"media.removeRecords",params:{mediaIds:t}}),e(r=>({selectedMediaIds:r.selectedMediaIds.filter(n=>!t.includes(n))}))}function oz(e){return e?.aiCaptions?.map(t=>t.thumbRelPath)??[]}function az(e,t){const r=new Map(t.map(n=>[n.id,n]));for(const n of e)_i.release(n),Mt.clearProxyKey(n),nz(n,oz(r.get(n))),Gi.getState().clearMedia(n)}function sz(e,t){const r=async(n,i,o)=>{e({error:null});const a=new Set(t().importingIds),s=t().mediaItems.filter(c=>!a.has(c.id)),l=t().selectedMediaIds;iz(e,n);try{await i(),az(n,s)}catch(c){throw _0({type:"media.setRecords",params:{mediaItems:s}}),e({selectedMediaIds:l,error:c instanceof Error?c.message:o}),c}};return{deleteMedia:async n=>{const{currentProjectId:i}=t();await r([n],async()=>{const{mediaLibraryService:o}=await Tt();return i?o.deleteMediaFromProject(i,n):o.deleteMedia(n)},"Không thể xoá tệp")},deleteMediaBatch:async n=>{const{currentProjectId:i}=t();await r(n,async()=>{const{mediaLibraryService:o}=await Tt();return i?o.deleteMediaBatchFromProject(i,n):o.deleteMediaBatch(n)},"Không thể xoá các tệp")}}}let lz=null;function hn(){return lz}const Rt=He("MediaRelinkingActions");function cz(e,t){const r={...t};delete r.id;const n=qr([{type:"media.update",params:{mediaId:e,updates:r}}]);if(!n.ok)throw new Error(n.issues.map(i=>i.message).join("; "))}function Of(e,t,r){_i.invalidate(t),cz(t,r);const n=hn();if(n)for(const i of n.getItemsState().items)i.mediaId===t&&n.updateProjectItem(i.id,{src:void 0,reverseConformSrc:void 0,reverseConformPath:void 0,reverseConformKey:void 0,reverseConformPreviewSrc:void 0,reverseConformPreviewPath:void 0,reverseConformPreviewKey:void 0,reverseConformPreviewUsesProxy:void 0,reverseConformPreviewIsSourceLevel:void 0,reverseConformPreviewSourceDuration:void 0,reverseConformPreviewFps:void 0,reverseConformStatus:void 0,reverseConformLocalStart:void 0});e().markMediaHealthy(t)}function uz(e,t){const r={mediaId:e.id,label:e.fileName,src:void 0,thumbnailUrl:void 0,waveformData:void 0};if(e.width>0&&e.height>0&&(r.sourceWidth=e.width,r.sourceHeight=e.height),e.duration>0){const n=e.fps>0?e.fps:t;r.sourceFps=n,r.sourceDuration=Math.round(e.duration*n)}return r}function F0(e,t){const r=hn();if(!r)return Rt.error("[getOrphanedRelinkTargetIds] Timeline relinking actions are not registered"),[e];const{items:n,itemById:i}=r.getItemsState(),o=i[e];if(!o)return[e];const a=r.getSynchronizedLinkedItems(n,e),s=o.mediaId;if(!s||s===t){const c=a.filter(u=>u.mediaId===t).map(u=>u.id);return c.length>0?c:[e]}const l=a.filter(c=>c.mediaId===s).map(c=>c.id);return l.length>0?l:[e]}function dz(e){const t=new Set;for(const r of e){const i=hn()?.getItemsState().itemById[r];F0(r,i?.mediaId??"").forEach(a=>t.add(a))}return t.size>0?Array.from(t):e}function fz(e,t){return{markMediaBroken:(r,n)=>{e(i=>{if(i.brokenMediaIds.includes(r))return i;const o=new Map(i.brokenMediaInfo);return o.set(r,n),{brokenMediaIds:[...i.brokenMediaIds,r],brokenMediaInfo:o}})},markMediaHealthy:r=>{e(n=>{if(!n.brokenMediaIds.includes(r)&&!n.brokenMediaInfo.has(r)&&!(n.dismissedMissingMediaIds??[]).includes(r))return n;const i=new Map(n.brokenMediaInfo);return i.delete(r),{brokenMediaIds:n.brokenMediaIds.filter(o=>o!==r),brokenMediaInfo:i,dismissedMissingMediaIds:(n.dismissedMissingMediaIds??[]).filter(o=>o!==r)}})},dismissMissingMediaWarnings:r=>{e(n=>({dismissedMissingMediaIds:Array.from(new Set([...n.dismissedMissingMediaIds??[],...r])),showMissingMediaDialog:!1}))},relinkMedia:async(r,n)=>{try{const{mediaLibraryService:i}=await Tt(),o=await i.relinkMediaFile(r,n);return Of(t,r,o),t().showNotification({type:"success",message:`Đã liên kết lại "${o.fileName}"`}),!0}catch(i){return Rt.error("[relinkMedia] error:",i),t().showNotification({type:"error",message:i instanceof Error?i.message:"Không thể liên kết lại tệp"}),!1}},relinkMediaBatch:async r=>{const n=[],i=[],{mediaLibraryService:o}=await Tt();for(const{mediaId:a,file:s}of r)try{const l=await o.relinkMediaFile(a,s);Of(t,a,l),n.push(a)}catch(l){Rt.error(`[relinkMediaBatch] error for ${a}:`,l),i.push(a)}return n.length>0?t().showNotification({type:i.length>0?"warning":"success",message:`Đã liên kết lại ${n.length} tệp${i.length>0?`, ${i.length} tệp lỗi`:""}`}):i.length>0&&t().showNotification({type:"error",message:`Không thể liên kết lại ${i.length} tệp`}),{success:n,failed:i}},openMissingMediaDialog:()=>e({showMissingMediaDialog:!0}),closeMissingMediaDialog:()=>e({showMissingMediaDialog:!1}),setOrphanedClips:r=>e({orphanedClips:r}),clearOrphanedClips:()=>e({orphanedClips:[]}),openOrphanedClipsDialog:()=>e({showOrphanedClipsDialog:!0}),closeOrphanedClipsDialog:()=>e({showOrphanedClipsDialog:!1}),relinkOrphanedClip:async(r,n)=>{try{const{mediaLibraryService:i}=await Tt(),o=await i.getMedia(n);if(!o)return Rt.error(`[relinkOrphanedClip] Media not found: ${n}`),t().showNotification({type:"error",message:"Selected media not found"}),!1;const a=F0(r,n);if(a.length===0)return!1;if(a.every(d=>hn()?.getItemsState().itemById[d]?.mediaId===n))return e(d=>({orphanedClips:d.orphanedClips.filter(f=>!a.includes(f.itemId))})),!0;const l=hn();if(!l)return Rt.error("[relinkOrphanedClip] Timeline relinking actions are not registered"),t().showNotification({type:"error",message:"Timeline actions are unavailable"}),!1;const c=uz(o,l.getFps()),u=a.filter(d=>l.updateProjectItem(d,c));return u.length===0?(t().showNotification({type:"error",message:"Clip not found"}),!1):(Rt.debug(`[relinkOrphanedClip] Relinked clip ${r} to media ${n}`,{relinkedItemIds:u}),e(d=>({orphanedClips:d.orphanedClips.filter(f=>!u.includes(f.itemId))})),t().showNotification({type:"success",message:u.length>1?`Linked clips relinked to "${o.fileName}"`:`Clip relinked to "${o.fileName}"`}),!0)}catch(i){return Rt.error("[relinkOrphanedClip] error:",i),t().showNotification({type:"error",message:i instanceof Error?i.message:"Failed to relink clip"}),!1}},removeOrphanedClips:r=>{try{const n=dz(r),i=hn();if(!i){Rt.error("[removeOrphanedClips] Timeline relinking actions are not registered"),t().showNotification({type:"error",message:"Timeline actions are unavailable"});return}i.removeProjectItems(n),e(o=>({orphanedClips:o.orphanedClips.filter(a=>!r.includes(a.itemId))})),t().showNotification({type:"info",message:`Removed ${r.length} orphaned clip${r.length!==1?"s":""}`})}catch(n){Rt.error("[removeOrphanedClips] error:",n)}}}}async function pz(e,t){if(e.length===0)return{healthyIds:[],broken:[]};const r=await vt().listMedia(t),n=new Set(r.map(a=>a.id)),i=[],o=[];for(const a of e)n.has(a.id)?i.push(a.id):o.push({mediaId:a.id,fileName:a.fileName,errorType:"file_missing"});return{healthyIds:i,broken:o}}function Ac(e){return Math.max(0,Math.min(1,e))}function zf(e){return e==="downloading"||e==="preparing"}function Us(e){return e.stage==="preparing"&&Ac(e.progress)>=1}function Df(e){const t=Ac(e.progress);switch(e.stage){case"queued":return 0;case"downloading":return t*.6;case"preparing":return .6+t*.15;case"decoding":return .75+t*.05;case"transcribing":return .8+t*.2}}function mz(e,t){const r={...t,progress:Ac(t.progress)};if(!e||e.stage==="queued")return r;const n=zf(r.stage),i=zf(e.stage);return n===i?r.stage==="downloading"&&r.restarted&&e.stage==="preparing"&&!Us(e)||Df(r)>=Df(e)?r:e:n?Us(r)?e:r:Us(e)?r:e}const oi=He("MediaLibraryStore");let kr=null;function Al(e,t){const r=qr([{type:e,params:t}]);if(!r.ok)throw new Error(r.issues.map(n=>n.message).join("; "))}function Rf(e){Al("media.setRecords",{mediaItems:e,resetImporting:!0})}function To(e,t){if(e===t)return!0;if(e.length!==t.length)return!1;for(let r=0;r<e.length;r+=1)if(e[r]!==t[r])return!1;return!0}function hz(e){const t={};for(const r of e)t[r.id]=r;return t}function Lf(e,t){const r=new Map;for(const n of e)r.set(n.id,t?.has(n.id)?"ready":"idle");return r}async function gz(e){try{const t=await G2(e.map(r=>r.id));return Lf(e,t)}catch(t){return oi.warn("[MediaLibraryStore] Failed to load transcript availability:",t),Lf(e)}}function yz(e){return e.filter(t=>Mt.canGenerateProxy(t.mimeType))}async function vz(e){const t=yz(e);if(t.length!==0){for(const r of t)Mt.setProxyKey(r.id,sh(r));await Mt.loadExistingProxies(t.map(r=>r.id))}}const bz=(e,t)=>({currentProjectId:null,mediaItems:[],mediaById:{},isLoading:!1,importingIds:[],error:null,errorLink:null,notification:null,selectedMediaIds:[],selectedCompositionIds:[],searchQuery:"",filterByType:null,sortBy:"date",viewMode:"grid",mediaItemSize:1,brokenMediaIds:[],brokenMediaInfo:new Map,dismissedMissingMediaIds:[],showMissingMediaDialog:!1,isScanningMediaHealth:!1,orphanedClips:[],showOrphanedClipsDialog:!1,unsupportedCodecFiles:[],showUnsupportedCodecDialog:!1,unsupportedCodecResolver:null,proxyStatus:new Map,proxyProgress:new Map,interpolationStatus:new Map,interpolationProgress:new Map,interpolationStage:new Map,interpolationEtaSeconds:new Map,upscaleStatus:new Map,upscaleProgress:new Map,upscaleStage:new Map,upscaleEtaSeconds:new Map,transcriptStatus:new Map,transcriptProgress:new Map,taggingMediaIds:new Set,analysisProgress:null,setCurrentProject:r=>{t().currentProjectId;const n=t().mediaItems.map(i=>i.id);for(const i of n)Mt.clearProxyKey(i);Rf([]),e({currentProjectId:r,selectedMediaIds:[],selectedCompositionIds:[],isLoading:!!r,proxyStatus:new Map,proxyProgress:new Map,interpolationStatus:new Map,interpolationProgress:new Map,interpolationStage:new Map,interpolationEtaSeconds:new Map,upscaleStatus:new Map,upscaleProgress:new Map,upscaleStage:new Map,upscaleEtaSeconds:new Map,transcriptStatus:new Map,transcriptProgress:new Map,taggingMediaIds:new Set,analysisProgress:null,dismissedMissingMediaIds:[],isScanningMediaHealth:!1}),Gi.getState().clearAll()},loadMediaItems:async()=>{const{currentProjectId:r}=t();if(!r){e({isLoading:!1});return}e({isLoading:!0,error:null});const n=rr(),i=oi.startEvent("loadMediaItems",n);i.set("projectId",r);try{const{mediaLibraryService:o}=await Tt(),a=await o.getMediaForProject(r);Rf(a),e({isLoading:!1}),i.set("mediaCount",a.length),o.prefetchThumbnails(a.map(l=>({id:l.id,thumbnailId:l.thumbnailId}))).catch(l=>oi.warn("[MediaLibraryStore] Thumbnail prefetch failed:",l)),t().scanMediaHealth();const s=await gz(a);e({transcriptStatus:s,transcriptProgress:new Map}),i.set("transcriptsReady",[...s.values()].filter(l=>l==="ready").length);try{await vz(a)}catch(l){oi.warn("[MediaLibraryStore] Proxy initialization failed:",l)}i.success({mediaCount:a.length})}catch(o){i.failure(o instanceof Error?o:new Error(String(o)));const a=o instanceof Error?o.message:"Failed to load media";throw e({error:a,isLoading:!1}),o}},...ez(e,t),...sz(e,t),...fz(e,t),scanMediaHealth:async()=>{const{currentProjectId:r,mediaItems:n}=t();if(!r||n.length===0){e({isScanningMediaHealth:!1});return}e({isScanningMediaHealth:!0});try{const i=await pz(n,r);if(t().currentProjectId!==r)return;for(const o of i.healthyIds)t().markMediaHealthy(o);for(const o of i.broken)t().markMediaBroken(o.mediaId,o)}catch(i){oi.warn("[MediaLibraryStore] Workspace health scan failed:",i)}finally{t().currentProjectId===r&&e({isScanningMediaHealth:!1})}},prependMediaItem:r=>{Al("media.add",{media:r})},setSelection:({mediaIds:r,compositionIds:n})=>{const i=t(),o=To(i.selectedMediaIds,r),a=To(i.selectedCompositionIds,n);o&&a||e({selectedMediaIds:o?i.selectedMediaIds:r,selectedCompositionIds:a?i.selectedCompositionIds:n})},selectMedia:r=>{const n=t();To(n.selectedMediaIds,r)||e({selectedMediaIds:r})},selectCompositions:r=>{const n=t();To(n.selectedCompositionIds,r)||e({selectedCompositionIds:r})},toggleMediaSelection:r=>e(n=>({selectedMediaIds:n.selectedMediaIds.includes(r)?n.selectedMediaIds.filter(i=>i!==r):[...n.selectedMediaIds,r]})),toggleCompositionSelection:r=>e(n=>({selectedCompositionIds:n.selectedCompositionIds.includes(r)?n.selectedCompositionIds.filter(i=>i!==r):[...n.selectedCompositionIds,r]})),clearSelection:()=>{const r=t();r.selectedMediaIds.length===0&&r.selectedCompositionIds.length===0||e({selectedMediaIds:[],selectedCompositionIds:[]})},setSearchQuery:r=>e({searchQuery:r}),setFilterByType:r=>e({filterByType:r}),setSortBy:r=>e({sortBy:r}),setViewMode:r=>e({viewMode:r}),setMediaItemSize:r=>e({mediaItemSize:Math.max(1,Math.min(5,r))}),clearError:()=>e({error:null,errorLink:null}),showNotification:r=>{e({notification:r}),kr!==null&&clearTimeout(kr),kr=setTimeout(()=>{kr=null,e(n=>n.notification===r?{notification:null}:n)},4e3)},clearNotification:()=>{kr!==null&&(clearTimeout(kr),kr=null),e({notification:null})},confirmUnsupportedCodecs:r=>new Promise(n=>{e({unsupportedCodecFiles:r,showUnsupportedCodecDialog:!0,unsupportedCodecResolver:n})}),resolveUnsupportedCodecDialog:r=>{const{unsupportedCodecResolver:n}=t();n&&n(r),e({unsupportedCodecFiles:[],showUnsupportedCodecDialog:!1,unsupportedCodecResolver:null})},setProxyStatus:(r,n)=>{e(i=>{const o=new Map(i.proxyStatus);return o.set(r,n),{proxyStatus:o}})},clearProxyStatus:r=>{e(n=>{const i=new Map(n.proxyStatus);i.delete(r);const o=new Map(n.proxyProgress);return o.delete(r),{proxyStatus:i,proxyProgress:o}})},setProxyProgress:(r,n)=>{e(i=>{const o=new Map(i.proxyProgress);return o.set(r,n),{proxyProgress:o}})},setInterpolationStatus:(r,n)=>{e(i=>{const o=new Map(i.interpolationStatus);return o.set(r,n),{interpolationStatus:o}})},clearInterpolationStatus:r=>{e(n=>{const i=new Map(n.interpolationStatus);i.delete(r);const o=new Map(n.interpolationProgress);o.delete(r);const a=new Map(n.interpolationStage);a.delete(r);const s=new Map(n.interpolationEtaSeconds);return s.delete(r),{interpolationStatus:i,interpolationProgress:o,interpolationStage:a,interpolationEtaSeconds:s}})},setInterpolationProgress:(r,n,i,o)=>{e(a=>{const s=new Map(a.interpolationProgress);s.set(r,n);const l=new Map(a.interpolationStage);l.set(r,i);const c=new Map(a.interpolationEtaSeconds);return typeof o=="number"&&c.set(r,o),{interpolationProgress:s,interpolationStage:l,interpolationEtaSeconds:c}})},setUpscaleStatus:(r,n)=>{e(i=>{const o=new Map(i.upscaleStatus);return o.set(r,n),{upscaleStatus:o}})},clearUpscaleStatus:r=>{e(n=>{const i=new Map(n.upscaleStatus);i.delete(r);const o=new Map(n.upscaleProgress);o.delete(r);const a=new Map(n.upscaleStage);a.delete(r);const s=new Map(n.upscaleEtaSeconds);return s.delete(r),{upscaleStatus:i,upscaleProgress:o,upscaleStage:a,upscaleEtaSeconds:s}})},setUpscaleProgress:(r,n,i,o)=>{e(a=>{const s=new Map(a.upscaleProgress);s.set(r,n);const l=new Map(a.upscaleStage);l.set(r,i);const c=new Map(a.upscaleEtaSeconds);return typeof o=="number"&&c.set(r,o),{upscaleProgress:s,upscaleStage:l,upscaleEtaSeconds:c}})},setTranscriptStatus:(r,n)=>{e(i=>{const o=new Map(i.transcriptStatus);return o.set(r,n),{transcriptStatus:o}})},setTranscriptProgress:(r,n)=>{e(i=>{const o=new Map(i.transcriptProgress);return o.set(r,mz(o.get(r),n)),{transcriptProgress:o}})},clearTranscriptProgress:r=>{e(n=>{const i=new Map(n.transcriptProgress);return i.delete(r),{transcriptProgress:i}})},setTaggingMedia:(r,n)=>{e(i=>{const o=new Set(i.taggingMediaIds);return n?o.add(r):o.delete(r),{taggingMediaIds:o}})},updateMediaCaptions:(r,n)=>{t().mediaItems.some(i=>i.id===r)&&Al("media.update",{mediaId:r,updates:{aiCaptions:n,updatedAt:Date.now()}})},beginAnalysisRun:r=>{r<=0||e(n=>{const i=n.analysisProgress;return i?{analysisProgress:{total:i.total+r,completed:i.completed,cancelRequested:i.cancelRequested}}:{analysisProgress:{total:r,completed:0,cancelRequested:!1}}})},incrementAnalysisCompleted:(r=1)=>{e(n=>n.analysisProgress?{analysisProgress:{...n.analysisProgress,completed:Math.min(n.analysisProgress.total,n.analysisProgress.completed+r)}}:n)},requestAnalysisCancel:()=>{e(r=>r.analysisProgress?{analysisProgress:{...r.analysisProgress,cancelRequested:!0}}:r)},endAnalysisRun:()=>{e({analysisProgress:null})}}),Sz=_e()(gp(bz,{name:"MediaLibraryStore",enabled:!1})),Oe=Sz;{let e=Oe.getState().mediaItems;Oe.subscribe(t=>{t.mediaItems!==e&&(e=t.mediaItems,Oe.setState({mediaById:hz(t.mediaItems)}))}),Mt.onStatusChange((t,r,n)=>{const i=Oe.getState();if(r==="idle"){i.clearProxyStatus(t);return}i.setProxyStatus(t,r),n!==void 0&&i.setProxyProgress(t,n)}),Mt.setMediaResolver(t=>Oe.getState().mediaById[t]),Mt.setFilmstripPrewarm(async(t,r,n,i)=>{const{importFilmstripCache:o}=await import("./timeline-services-BXWC70-e.js"),{filmstripCache:a}=await o();await a.prewarmPriorityWindow(t,r,n,i)})}const Iz=()=>{const e=Oe(a=>a.mediaItems),t=Oe(a=>a.searchQuery),r=Oe(a=>a.filterByType),n=Oe(a=>a.sortBy);let i=e;if(t){const a=t.toLowerCase();i=i.filter(s=>s.fileName.toLowerCase().includes(a)||s.aiCaptions?.some(l=>l.text.toLowerCase().includes(a)))}return r&&(i=i.filter(a=>Hv(a.mimeType)===r)),[...i].sort((a,s)=>{switch(n){case"name":return a.fileName.localeCompare(s.fileName);case"date":return s.createdAt-a.createdAt;case"size":return s.fileSize-a.fileSize;default:return 0}})};var jD=Object.freeze({__proto__:null,useFilteredMediaItems:Iz,useMediaLibraryStore:Oe});function Wi(e,t,r){const n=e.transform,i=r?.width??t.width,o=r?.height??t.height,a=t.width/i,s=t.height/o,l=Math.min(a,s),c=i*l,u=o*l,d=n?.width??c,f=n?.height??u,p=n?.anchorX??d/2,m=n?.anchorY??f/2;return{x:n?.x??0,y:n?.y??0,width:d,height:f,anchorX:p,anchorY:m,rotation:n?.rotation??0,opacity:n?.opacity??1,cornerRadius:n?.cornerRadius??0}}function On(e){if(e.type==="video"){const t=e;if(t.sourceWidth&&t.sourceHeight)return{width:t.sourceWidth,height:t.sourceHeight};if(t.mediaId){const r=Oe.getState().mediaById[t.mediaId];if(r?.width&&r?.height)return{width:r.width,height:r.height}}}if(e.type==="image"||e.type==="lottie"){const t=e;if(t.sourceWidth&&t.sourceHeight)return{width:t.sourceWidth,height:t.sourceHeight};if(t.mediaId){const r=Oe.getState().mediaById[t.mediaId];if(r?.width&&r?.height)return{width:r.width,height:r.height}}}if(e.type==="composition"){const t=e;return t.compositionWidth&&t.compositionHeight?{width:t.compositionWidth,height:t.compositionHeight}:void 0}}function xz(e,t){if(!t)return e;const r=Object.prototype.hasOwnProperty.call(t,"anchorX"),n=Object.prototype.hasOwnProperty.call(t,"anchorY");return{...e,...t,anchorX:r&&t.anchorX===void 0?(t.width??e.width)/2:t.anchorX??e.anchorX,anchorY:n&&t.anchorY===void 0?(t.height??e.height)/2:t.anchorY??e.anchorY,opacity:t.opacity??e.opacity,cornerRadius:t.cornerRadius??e.cornerRadius}}function A0(e,{canvas:t,relativeFrame:r,keyframes:n,previewTransform:i,expressionContext:o}){const a=Wi(e,t,On(e)),s=n&&GC(n)?Sc(a,n,r,o):a,l=GM(s,e.motionLayers,r),c=$M(l,e.motionModifiers,{frame:r,fps:t.fps,frameWidth:t.width,frameHeight:t.height}),u=xz(c,i);return e.type==="text"&&!mC(e.cornerPin)?aC(EM(e,n,r,t),u):u}function O0(e,{canvas:t,frame:r,keyframes:n,previewTransform:i,getItem:o,getKeyframes:a,getPreviewTransform:s}){const l=c=>A0(c,{canvas:t,relativeFrame:r-c.from,keyframes:c.id===e.id?n:a?.(c.id),previewTransform:c.id===e.id?i??s?.(c.id):s?.(c.id),expressionContext:o&&a?{globalFrame:r,canvas:t,getItem:o,getKeyframes:a,getPreviewTransform:s}:void 0});return o?Wg(e,{getItem:o,resolveLocal:l}):l(e)}function Ns(e){const t=Math.max(-1,Math.min(1,e));return t<0?Math.round(t*32768):Math.round(t*32767)}const Oc=44;function z0(e,t,r){const n=e*2,i=t*n,o=new ArrayBuffer(Oc+i),a=new DataView(o),s=(l,c)=>{for(let u=0;u<c.length;u+=1)a.setUint8(l+u,c.charCodeAt(u))};return s(0,"RIFF"),a.setUint32(4,36+i,!0),s(8,"WAVE"),s(12,"fmt "),a.setUint32(16,16,!0),a.setUint16(20,1,!0),a.setUint16(22,e,!0),a.setUint32(24,r,!0),a.setUint32(28,r*n,!0),a.setUint16(32,n,!0),a.setUint16(34,16,!0),s(36,"data"),a.setUint32(40,i,!0),o}function wz(e){const t=Math.max(1,Math.min(2,e.numberOfChannels)),r=e.length,n=z0(t,r,e.sampleRate),i=new Int16Array(n,Oc,r*t),o=e.getChannelData(0);if(t>1){const a=e.getChannelData(1);let s=0;for(let l=0;l<r;l+=1)i[s]=Ns(o[l]??0),i[s+1]=Ns(a[l]??0),s+=2}else for(let a=0;a<r;a+=1)i[a]=Ns(o[a]??0);return new Blob([n],{type:"audio/wav"})}function kz(e,t,r){const n=Math.min(e.length,t.length),i=z0(2,n,r),o=new Int16Array(i,Oc,n*2);let a=0;for(let s=0;s<n;s+=1)o[a]=e[s],o[a+1]=t[s],a+=2;return new Blob([i],{type:"audio/wav"})}const Pz=He("PreviewAudioConform"),Tz=new Map,jo=new Map;function D0(e){return`preview-audio:${e}`}function R0(e){Tz.delete(e),jo.delete(e),_i.invalidate(D0(e))}m2(({mediaId:e})=>R0(e));async function qD(e){try{return await jh($r(),sa(e))}catch{return!1}}function L0(e,t,r){const n=jo.get(e);if(n)return n;const i=(async()=>{const o=r??await uc(e);if(await jh($r(),sa(e)))return;const a=D0(e),s=t();_i.get(a)||_i.acquire(a,s),await P2($r(),sa(e),new Uint8Array(await s.arrayBuffer()),o)})().catch(o=>{Pz.warn("Failed to persist preview audio conform asset",{mediaId:e,err:o})}).finally(()=>{jo.delete(e)});return jo.set(e,i),i}function KD(e,t,r){return L0(e,()=>wz(t),r)}function GD(e,t,r,n,i){return L0(e,()=>kz(t,r,n),i)}async function WD(e){const t=typeof e=="string"?e:e.id;R0(t),await Hh($r(),sa(t))}const Ez=-1,Cz=1,Mz=.04,_z=.96,Fz=.52,Sa=1e-4,Bf=12;function Ia(e){const t=typeof e=="number"&&Number.isFinite(e)?e:0;return Math.max(Ez,Math.min(Cz,Math.round(t*100)/100))}function En(e){const t=typeof e=="number"&&Number.isFinite(e)?e:Fz;return Math.max(Mz,Math.min(_z,Math.round(t*1e3)/1e3))}function Az(e,t){const r=En(t),n=Ia(e),i=r,o=1-i,a=i;return n>=0?i+n*o:i+n*a}function Oz(e,t){const r=En(t),n=Ia(e),i=1-r,o=1-i,a=i;return n>=0?i+n*o:i+n*a}function zc(e){return Math.max(0,Math.min(1,e))}function Uf(e){return Math.max(Sa,Math.min(1-Sa,e))}function xa(e,t){const r=Math.log(Uf(t))/Math.log(Uf(e));return Number.isFinite(r)?Math.max(1,Math.min(Bf,r)):Bf}function wa(e,t){return Math.pow(zc(e),t)}function VD(e,t,r){const n=zc(e),i=En(r),o=Az(t,r);if(Math.abs(o-i)<=Sa)return n;if(o>i){const s=xa(1-i,1-o);return 1-wa(1-n,s)}const a=xa(i,o);return wa(n,a)}function ZD(e,t,r){const n=zc(e),i=En(r),o=Oz(t,r),a=1-i;if(Math.abs(o-a)<=Sa)return 1-n;if(o>a){const l=xa(i,1-o);return 1-wa(n,l)}const s=xa(1-i,o);return wa(1-n,s)}const zz=-12,Dz=12,Rz=-100,Lz=100,Bz=1e-4;function Uz(e){return Number.isFinite(e)?e??0:0}function B0(e){return Number.isFinite(e)?Math.max(zz,Math.min(Dz,Math.round(e))):0}function U0(e){return Number.isFinite(e)?Math.max(Rz,Math.min(Lz,Math.round(e))):0}function YD(e){return e?B0(e.audioPitchSemitones??0)+U0(e.audioPitchCents??0)/100:0}function XD(e){return Math.abs(e??0)>Bz}function QD(e){const t=Uz(e);return Math.pow(2,t/12)}function Ae(e,t=0){return Number.isFinite(e)?Math.max(0,Math.round(e)):t}function Mr(e,t=1){return Number.isFinite(e)?Math.max(1,Math.round(e)):t}function Nz(e){if(!(e===void 0||!Number.isFinite(e)||e<=0))return Math.round(e*1e3)/1e3}const $z=[{prefix:"audioEqBand1",hasEnabled:!0,hasGain:!0,hasSlope:!0,qDefault:zr,freq:{min:Go,max:Wo,def:Ko}},{prefix:"audioEqLowCut",hasEnabled:!0,hasSlope:!0,freq:{min:Go,max:Wo,def:Ko}},{prefix:"audioEqLow",hasEnabled:!0,hasGain:!0,qDefault:zr,freq:{min:Mp,max:_p,def:Cp}},{prefix:"audioEqLowMid",hasEnabled:!0,hasGain:!0,qDefault:zr,freq:{min:Ap,max:Op,def:Fp}},{prefix:"audioEqMid",hasGain:!0},{prefix:"audioEqHighMid",hasEnabled:!0,hasGain:!0,qDefault:Ao,freq:{min:Lp,max:Bp,def:Rp}},{prefix:"audioEqHigh",hasEnabled:!0,hasGain:!0,qDefault:Ao,freq:{min:Np,max:$p,def:Up}},{prefix:"audioEqBand6",hasEnabled:!0,hasGain:!0,hasSlope:!0,qDefault:Ao,freq:{min:Zo,max:Yo,def:Vo}},{prefix:"audioEqHighCut",hasEnabled:!0,hasSlope:!0,freq:{min:Zo,max:Yo,def:Vo}}];function Hz(e){const t=[];if(e.hasEnabled&&t.push({key:`${e.prefix}Enabled`,clamp:r=>!!r}),e.hasGain&&t.push({key:`${e.prefix}GainDb`,clamp:r=>It(r)}),e.freq){const{min:r,max:n,def:i}=e.freq;t.push({key:`${e.prefix}FrequencyHz`,clamp:o=>Dt(o,r,n,i)})}if(e.qDefault!==void 0){const r=e.qDefault;t.push({key:`${e.prefix}Q`,clamp:n=>Lt(n,r)})}return e.hasSlope&&t.push({key:`${e.prefix}SlopeDbPerOct`,clamp:r=>Gn(r)}),t}const jz=[{key:"trimStart",clamp:e=>Ae(e)},{key:"trimEnd",clamp:e=>Ae(e)},{key:"sourceStart",clamp:e=>Ae(e)},{key:"sourceEnd",clamp:e=>Ae(e)},{key:"sourceDuration",clamp:e=>Ae(e)},{key:"sourceFps",clamp:e=>Nz(e)},{key:"crop",clamp:e=>lC(e)},{key:"audioFadeInCurve",clamp:e=>Ia(e)},{key:"audioFadeOutCurve",clamp:e=>Ia(e)},{key:"audioFadeInCurveX",clamp:e=>En(e)},{key:"audioFadeOutCurveX",clamp:e=>En(e)},{key:"audioPitchSemitones",clamp:e=>B0(e)},{key:"audioPitchCents",clamp:e=>U0(e)},{key:"audioEqOutputGainDb",clamp:e=>It(e)},...$z.flatMap(Hz)];function N0(e){for(const{key:t,clamp:r}of jz){const n=e[t];n!==void 0&&(e[t]=r(n))}}const ka=1;function Eo(e,t=100){return e===void 0?void 0:Math.max(0,Math.min(t,e))}function we(e){const t={...e};t.from=Ae(e.from),t.durationInFrames=Mr(e.durationInFrames),N0(t);const r=t;if(r.cornerPin){const n=hC(r.transform?.width??0,r.transform?.height??0,r.type==="video"||r.type==="image"?{sourceWidth:r.sourceWidth,sourceHeight:r.sourceHeight,crop:r.crop}:void 0);r.cornerPin={...r.cornerPin,referenceWidth:r.cornerPin.referenceWidth??(n.width>0?n.width:void 0),referenceHeight:r.cornerPin.referenceHeight??(n.height>0?n.height:void 0)}}return r.type==="shape"&&(r.taperStartWidth=Eo(r.taperStartWidth,200),r.taperEndWidth=Eo(r.taperEndWidth,200),r.taperStartLength=Eo(r.taperStartLength),r.taperEndLength=Eo(r.taperEndLength),r.shapeType==="path"&&(r.pathClosed=r.isMask?!0:r.pathClosed??!0,r.pathClosed===!1?r.fillEnabled=!1:r.fillEnabled??=!0,r.strokeEnabled??=!0,r.strokeLineCap??="butt",r.strokeLineJoin??="miter",r.strokeMiterLimit??=4,r.pathVertices=r.pathVertices?.map(n=>({...n,tangentMode:n.tangentMode??(n.inHandle[0]===0&&n.inHandle[1]===0&&n.outHandle[0]===0&&n.outHandle[1]===0?"corner":"smooth")}))),r.strokeEnabled===!0&&(r.strokeWidth??0)<ka&&(r.strokeWidth=ka),r.isMask&&(r.blendMode="normal")),(r.type==="video"||r.type==="audio")&&r.sourceEnd!==void 0&&r.sourceStart===void 0&&(r.sourceStart=0),r}function Nf(e){const t={...e};return t.from!==void 0&&(t.from=Ae(t.from)),t.durationInFrames!==void 0&&(t.durationInFrames=Mr(t.durationInFrames)),N0(t),t.strokeEnabled===!0&&typeof t.strokeWidth=="number"&&t.strokeWidth<ka&&(t.strokeWidth=ka),t.sourceEnd!==void 0&&t.sourceStart===void 0&&(t.sourceStart=0),t}function qz(e){return{...e,volume:e.volume===void 0?void 0:Math.max(-60,Math.min(12,e.volume)),audioEq:ov(e.audioEq)}}function Kz(e,t,r){if(t===0)return null;const n=t/r,i=[];for(const o of e.cues){if(o.endSeconds<=n)continue;const a=Math.max(0,o.startSeconds-n),s=o.endSeconds-n;s<=a||i.push({...o,startSeconds:a,endSeconds:s})}return{cues:i}}function Gz(e,t,r){const n=t/r,i=[];for(const o of e.cues){if(o.startSeconds>=n)continue;const a=Math.min(o.endSeconds,n);a<=o.startSeconds||i.push({...o,endSeconds:a})}return{cues:i}}function Wz(e,t,r,n){const i=r/e,o=n/t,a=Math.min(i,o);return{x:0,y:0,width:Math.round(e*a),height:Math.round(t*a),rotation:0}}function $f(){return He("ItemsStore")}function $0(){const e=Dl(),t=e?ae.getState().getComposition(e):void 0,r=Ht.getState().currentProject?.metadata;return{width:t?.width??r?.width??ep,height:t?.height??r?.height??tp,fps:W.getState().fps}}function $s(e){return{...Wi(e,$0(),On(e)),...e.transform}}function Vz(e){const t=$0(),r=On(e);return Wz(r?.width??t.width,r?.height??t.height,t.width,t.height)}function Co(e,t,r){const n=e.items.map(i=>i.id!==t||i.type==="audio"?i:{...i,effects:r(i.effects||[])});return xe(n,e)}const S=_e()((e,t)=>({items:[],itemsByTrackId:{},itemById:{},itemsByLinkedGroupId:{},linkedItemsByItemId:{},maxItemEndFrame:0,mediaDependencyIds:[],mediaDependencyVersion:0,tracks:[],setItems:r=>e(n=>{const i=r.map(o=>we(o));return xe(i,n)}),setTracks:r=>e(n=>{const i=new Map(n.tracks.map(l=>[l.id,l])),o=Zf(r).map(l=>{const c=i.get(l.id),u=c===l?c:qz(l),d=Ly(u.id);return u.height===d?u:{...u,height:d}}).sort((l,c)=>(l.order??0)-(c.order??0)),a=hy(o);return{tracks:a.length===n.tracks.length&&a.every((l,c)=>l===n.tracks[c])?n.tracks:a}}),_addItem:r=>e(n=>{const i=[...n.items,we(r)];return xe(i,n)}),_addItems:r=>e(n=>{const i=[...n.items,...r.map(o=>we(o))];return xe(i,n)}),_updateItem:(r,n)=>{const i=Nf(n);return e(o=>{const a=o.items.map(s=>s.id===r?we({...s,...i}):s);return xe(a,o)})},_removeItems:r=>e(n=>{const i=new Set(r),o=n.items.filter(a=>!i.has(a.id));return xe(o,n)}),_rippleDeleteItems:r=>e(n=>{const i=new Set(r),o=n.items.filter(c=>i.has(c.id));if(o.length===0)return n;const a=n.items.filter(c=>!i.has(c.id)),s=S3(a,o),l=a.map(c=>{const u=s.get(c.id)??0;return u>0?we({...c,from:c.from-u}):c});return xe(l,n)}),_closeGapAtPosition:(r,n)=>e(i=>{const o=Ae(n),a=i.items.filter(d=>d.trackId===r).sort((d,f)=>d.from-f.from);if(a.length===0)return i;let s=0,l=0;for(const d of a){if(o>=s&&o<d.from){l=d.from;break}s=d.from+d.durationInFrames}if(l<=s)return i;const c=l-s,u=i.items.map(d=>d.trackId===r&&d.from>=l?we({...d,from:d.from-c}):d);return xe(u,i)}),_moveItem:(r,n,i)=>{const o=Ae(n);return e(a=>{const s=a.items.map(l=>l.id===r?we({...l,from:o,...i&&{trackId:i}}):l);return xe(s,a)})},_moveItems:r=>e(n=>{const i=new Map(r.map(a=>[a.id,{...a,from:Ae(a.from)}])),o=n.items.map(a=>{const s=i.get(a.id);return s?we({...a,from:s.from,...s.trackId&&{trackId:s.trackId}}):a});return xe(o,n)}),_duplicateItems:(r,n)=>{const i=t(),o=new Map(i.items.map(f=>[f.id,f])),a=[],s=Dl(),l=ae.getState().compositionById,c=new Map,u=new Map;for(let f=0;f<r.length;f++){const p=o.get(r[f]),m=n[f];if(!p||!m||s!==null&&ap(p)&&lp({parentCompositionId:s,insertedCompositionId:p.compositionId,compositionById:l}))continue;const h={...p,id:Z(),from:Ae(m.from),trackId:m.trackId,originId:Z(),linkedGroupId:p.linkedGroupId?c.get(p.linkedGroupId)??c.set(p.linkedGroupId,Z()).get(p.linkedGroupId):void 0};a.push(we(h)),u.set(p.id,h.id)}const d=a.map(f=>{const p=f.transformParent?.parentItemId,m=p?u.get(p):void 0;return m?{...f,transformParent:{...f.transformParent,parentItemId:m}}:f});return e(f=>{const p=[...f.items,...d];return xe(p,f)}),d},_trimItemStart:(r,n,i)=>e(o=>{const a=o.items.map(s=>{if(s.id!==r)return s;const l=W.getState().fps;let{clampedAmount:c}=yi(s,"start",n,l);if(!i?.skipAdjacentClamp){const h=El(r);c=vi(s,"start",c,o.items,h)}const u=s.from+c,d=s.durationInFrames-c;if(d<=0)return s;const f=jc(s,"start",c,d,l),p=s.type==="subtitle"?Kz(s,c,l):null,m=(s.type==="text"||s.type==="subtitle")&&s.words?{words:vn(s.words,c,c+d)}:null;return{...s,from:Ae(u),durationInFrames:Mr(d),...f,...p??{},...m??{}}});return xe(a,o)}),_trimItemEnd:(r,n,i)=>e(o=>{const a=o.items.map(s=>{if(s.id!==r)return s;const l=W.getState().fps;let{clampedAmount:c}=yi(s,"end",n,l);if(!i?.skipAdjacentClamp){const m=El(r);c=vi(s,"end",c,o.items,m)}const u=s.durationInFrames+c;if(u<=0)return s;const d=jc(s,"end",c,u,l),f=s.type==="subtitle"?Gz(s,u,l):null,p=(s.type==="text"||s.type==="subtitle")&&s.words?{words:vn(s.words,0,u)}:null;return{...s,durationInFrames:Mr(u),...d,...f??{},...p??{}}});return xe(a,o)}),_splitItem:(r,n)=>{const o=t().items.find(h=>h.id===r);if(!o)return null;const a=Ae(n),s=Ae(o.from),l=Mr(o.durationInFrames),c=s+l;if(a<=s||a>=c)return null;const u=a-s,d=c-a,f=o.originId??o.id,p={...o,from:s,originId:f,durationInFrames:u},m={...o,id:Z(),originId:f,from:a,durationInFrames:d};if(o.type==="subtitle"){const h=W.getState().fps,g=u/h,v=[],x=[];for(const b of o.cues){const w=b.startSeconds<g,E=b.endSeconds>g;w&&!E?v.push(b):w?(v.push({...b,endSeconds:g}),x.push({...b,id:`${b.id}-r`,startSeconds:0,endSeconds:b.endSeconds-g})):x.push({...b,startSeconds:b.startSeconds-g,endSeconds:b.endSeconds-g})}p.cues=v,m.cues=x}if((o.type==="text"||o.type==="subtitle")&&o.words){const[h,g]=ly(o.words,u,l);p.words=h,m.words=g}if(sr(o)&&!(o.type==="video"&&o.loop===!0)){const h=W.getState().fps,{sourceStart:g,speed:v,sourceFps:x}=Fi(o),w=ay(g,u,d,v,h,x??h);if(o.isReversed===!0){const E=w.right.sourceEnd-g,k=w.left.sourceEnd-g,T=E-k,O=g+T;if(p.sourceStart=O,p.sourceEnd=w.right.sourceEnd,m.sourceStart=g,m.sourceEnd=O,o.reverseConformPreviewIsSourceLevel===!0)p.reverseConformLocalStart=void 0,m.reverseConformLocalStart=void 0;else{const C=o.reverseConformLocalStart??0;p.reverseConformLocalStart=C,m.reverseConformLocalStart=C+u}}else p.sourceStart=g,p.sourceEnd=w.left.sourceEnd,m.sourceStart=w.right.sourceStart,m.sourceEnd=w.right.sourceEnd;$f().debug(`_splitItem: Original sourceStart:${g} speed:${v} leftDuration:${u} rightDuration:${d} reversed:${o.isReversed===!0}`),$f().debug(`_splitItem: leftItem.sourceStart:${p.sourceStart} leftItem.sourceEnd:${p.sourceEnd} rightItem.sourceStart:${m.sourceStart} rightItem.sourceEnd:${m.sourceEnd}`)}return e(h=>{const g=h.items.map(v=>v.id===r?we(p):v).concat(we(m));return xe(g,h)}),{leftItem:we(p),rightItem:we(m)}},_joinItems:r=>e(n=>{if(r.length<2)return n;const i=n.items.filter(p=>r.includes(p.id)).sort((p,m)=>p.from-m.from);if(i.length<2)return n;const o=i[0],a=i[i.length-1],s=i.every(p=>p.type===o.type),l=i.every(p=>p.trackId===o.trackId);if(!s||!l)return n;const c=a.from+a.durationInFrames-o.from,u={...o,from:Ae(o.from),durationInFrames:Mr(c),sourceEnd:a.sourceEnd,trimEnd:a.trimEnd},d=new Set(i.slice(1).map(p=>p.id)),f=n.items.filter(p=>!d.has(p.id)).map(p=>p.id===o.id?we(u):p);return xe(f,n)}),_rateStretchItem:(r,n,i,o)=>e(a=>{const s=a.items.map(l=>{if(l.id!==r)return l;const c=l.type==="image"&&l.label?.toLowerCase().endsWith(".gif");if(!sr(l)&&!c)return l;const u=sr(l)&&l.sourceEnd!==void 0,d=l.sourceStart??0,f=W.getState().fps,p=l.sourceFps??f,m=Mr(i);let h=o;if(u){const w=Math.max(1,(l.sourceEnd??d)-d);h=oy(iy(w,m,p,f))}const g=De(m,h,f,p),v=d+g,x=l.sourceDuration?Math.min(v,l.sourceDuration):v,b={...l,from:Ae(n),durationInFrames:m,speed:h};return u||(b.sourceEnd=Ae(x)),b});return xe(s,a)}),_updateItemTransform:(r,n)=>e(i=>{const o=i.items.map(a=>a.id!==r||a.type==="audio"?a:{...a,transform:{...$s(a),...n}});return xe(o,i)}),_resetItemTransform:r=>e(n=>{const i=n.items.map(o=>o.id!==r||o.type==="audio"?o:{...o,transform:Vz(o)});return xe(i,n)}),_updateItemsTransform:(r,n)=>e(i=>{const o=new Set(r),a=i.items.map(s=>!o.has(s.id)||s.type==="audio"?s:{...s,transform:{...$s(s),...n}});return xe(a,i)}),_updateItemsTransformMap:(r,n)=>e(i=>{const o=i.items.map(a=>{const s=r.get(a.id),l=n?.get(a.id);if(!s&&!l)return a;const c=l?we({...a,...Nf(l)}):a;return!s||a.type==="audio"?c:we({...c,transform:{...$s(c),...s}})});return xe(o,i)}),_addEffect:(r,n)=>e(i=>Co(i,r,o=>{const a={id:Z(),effect:n,enabled:!0};return[...o,a]})),_addEffects:r=>e(n=>{const i=new Map(r.map(a=>[a.itemId,a.effects])),o=n.items.map(a=>{const s=i.get(a.id);if(!s||a.type==="audio")return a;const l=a.effects||[],c=s.map(u=>({id:Z(),effect:u,enabled:!0}));return{...a,effects:[...l,...c]}});return xe(o,n)}),_updateEffect:(r,n,i)=>e(o=>Co(o,r,a=>a.map(s=>s.id===n?{...s,...i.effect&&{effect:i.effect},...i.enabled!==void 0&&{enabled:i.enabled}}:s))),_removeEffect:(r,n)=>e(i=>Co(i,r,o=>o.filter(a=>a.id!==n))),_toggleEffect:(r,n)=>e(i=>Co(i,r,o=>o.map(a=>a.id===n?{...a,enabled:!a.enabled}:a))),_setItemEffects:r=>e(n=>{const i=new Map(r.map(a=>[a.itemId,a.effects])),o=n.items.map(a=>{const s=i.get(a.id);return!s||a.type==="audio"?a:{...a,effects:s}});return xe(o,n)})}));let Hf=S.getState().items,Ol=S.getState().mediaDependencyIds,jf=l0(Ol);S.subscribe(e=>{if(e.items===Hf)return;Hf=e.items;const t=v3(e.items),r=l0(t);r!==jf&&(Ol=t,jf=r,S.setState({mediaDependencyIds:Ol,mediaDependencyVersion:e.mediaDependencyVersion+1}))});function H0(e,t){const r=at.getState(),n=Ep({inPoint:r.inPoint,outPoint:r.outPoint,maxFrame:Tp(e,t)});n.inPoint===r.inPoint&&n.outPoint===r.outPoint||at.setState({inPoint:n.inPoint,outPoint:n.outPoint})}function Zz(e){const t=yt.getState(),r=t.selectedItemIds.filter(s=>e.itemById[s]!==void 0);if(r.length!==t.selectedItemIds.length&&t.selectItems(r),t.selectedTrackIds.length===0&&t.activeTrackId===null)return;const n=new Set(e.tracks.map(s=>s.id)),i=t.selectedTrackIds.filter(s=>n.has(s)),o=t.activeTrackId===null||n.has(t.activeTrackId);if(i.length===t.selectedTrackIds.length&&o)return;const a=o?t.activeTrackId:i[0]??null;yt.setState({selectedTrackIds:i,selectedTrackId:i[0]??null,activeTrackId:a,selectionType:t.selectedItemIds.length>0?t.selectionType:i.length>0?"track":null})}let qf=S.getState().items,Kf=S.getState().tracks;S.subscribe(e=>{e.items===qf&&e.tracks===Kf||(qf=e.items,Kf=e.tracks,Zz(e))});let Gf=S.getState().maxItemEndFrame;S.subscribe(e=>{e.maxItemEndFrame!==Gf&&(Gf=e.maxItemEndFrame,H0(e.items,W.getState().fps))});W.subscribe((e,t)=>{e.fps!==t.fps&&H0(S.getState().items,e.fps)});function Yz(e){return e[0]?.compositionId??null}function Mo(e){return e??X2}function Xz(){return{currentFrame:be.getState().currentFrame,zoomLevel:Br.getState().level,scrollPosition:W.getState().scrollPosition,selectedItemIds:yt.getState().selectedItemIds}}function Hs(e){e?(Br.getState().setZoomLevel(e.zoomLevel),W.getState().setScrollPosition(e.scrollPosition),be.getState().setCurrentFrame(e.currentFrame),yt.getState().selectItems(e.selectedItemIds)):(Br.getState().setZoomLevel(1),W.getState().setScrollPosition(0))}function js(e){const t=at.getState();return{compositionId:e,items:S.getState().items,tracks:S.getState().tracks,transitions:q.getState().transitions,keyframes:$.getState().keyframes,currentFrame:be.getState().currentFrame,zoomLevel:Br.getState().level,scrollPosition:W.getState().scrollPosition,busAudioEq:be.getState().busAudioEq,markers:t.markers,inPoint:t.inPoint,outPoint:t.outPoint}}function qs(e){S.getState().setItems(e.items),S.getState().setTracks(e.tracks),q.getState().setTransitions(e.transitions),$.getState().setKeyframes(e.keyframes),yt.getState().clearSelection(),be.getState().setCurrentFrame(e.currentFrame),be.getState().setBusAudioEq(e.busAudioEq),at.getState().setMarkers(e.markers),at.getState().setInOutPoints(e.inPoint,e.outPoint)}function ai(e){const t=S.getState().items,r=ae.getState().getComposition(e),n=t.length>0?Math.max(...t.map(a=>a.from+a.durationInFrames)):0,i=r?.editorKind==="composite-2d"?Math.max(1,r.durationInFrames):n,o=at.getState();ae.getState().updateComposition(e,{items:t,tracks:S.getState().tracks,transitions:q.getState().transitions,keyframes:$.getState().keyframes,durationInFrames:i,busAudioEq:be.getState().busAudioEq,markers:o.markers,inPoint:o.inPoint,outPoint:o.outPoint})}function Ks(e){const t=ae.getState().getComposition(e);return t?(S.getState().setItems(t.items),S.getState().setTracks(t.tracks),q.getState().setTransitions(t.transitions??[]),$.getState().setKeyframes(t.keyframes??[]),yt.getState().clearSelection(),be.getState().setBusAudioEq(t.busAudioEq),at.getState().setMarkers(t.markers??[]),at.getState().setInOutPoints(t.inPoint??null,t.outPoint??null),!0):!1}function Qz(e,t,r){if(r){const i=e.find(o=>o.id===r&&o.compositionId===t&&(o.type==="composition"||o.type==="audio"));if(i)return i}const n=e.find(i=>i.type==="composition"&&i.compositionId===t);return n||(e.find(i=>i.type==="audio"&&i.compositionId===t)??null)}const Jz=16,ye=_e()((e,t)=>({breadcrumbs:[{compositionId:null,label:"Main Timeline"}],activeCompositionId:null,stashStack:[],mainHolder:null,enterComposition:(r,n,i)=>{be.getState().pause();const o=t();if(o.breadcrumbs.length>=Jz||o.activeCompositionId===r||o.breadcrumbs.some(u=>u.compositionId===r))return;o.activeCompositionId!==null&&ai(o.activeCompositionId);const a=js(o.activeCompositionId);if(!Ks(r))return;const s=be.getState().currentFrame,l=Qz(a.items,r,i);let c=0;if(l){const u=s-l.from;u>=0&&u<l.durationInFrames&&(c=u)}be.getState().setCurrentFrame(c),er(r),e({breadcrumbs:[...o.breadcrumbs,{compositionId:r,label:n,...l?.id&&{entryItemId:l.id}}],activeCompositionId:r,stashStack:[...o.stashStack,a]})},exitComposition:()=>{be.getState().pause();const r=t();if(r.breadcrumbs.length<=1||r.stashStack.length===0)return;r.activeCompositionId!==null&&ai(r.activeCompositionId);const n=r.stashStack[r.stashStack.length-1];qs(n);const i=r.breadcrumbs.slice(0,-1),o=i[i.length-1];er(o.compositionId),e({breadcrumbs:i,activeCompositionId:o.compositionId,stashStack:r.stashStack.slice(0,-1)})},navigateTo:r=>{be.getState().pause();const n=t();if(r<0||r>=n.breadcrumbs.length||r===n.breadcrumbs.length-1)return;n.activeCompositionId!==null&&ai(n.activeCompositionId);const i=n.breadcrumbs.length-1-r,o=n.stashStack[n.stashStack.length-i];o&&qs(o);const a=n.breadcrumbs.slice(0,r+1),s=a[a.length-1];er(s.compositionId),e({breadcrumbs:a,activeCompositionId:s.compositionId,stashStack:n.stashStack.slice(0,n.stashStack.length-i)})},resetToRoot:()=>{for(be.getState().pause();t().breadcrumbs.length>1;)t().exitComposition();const r=t();r.mainHolder&&(r.activeCompositionId!==null&&ai(r.activeCompositionId),qs(r.mainHolder)),er(null),e({breadcrumbs:[{compositionId:null,label:"Main Timeline"}],activeCompositionId:null,stashStack:[],mainHolder:null})},switchToSequence:r=>{be.getState().pause();const n=t(),i=Yz(n.breadcrumbs);if(i===r){if(r!==null&&n.mainHolder===null){const o=ae.getState().getComposition(r);if(!o)return;const a=js(null);Ks(r),be.getState().setCurrentFrame(0),er(r),e({breadcrumbs:[{compositionId:r,label:o.name}],activeCompositionId:r,stashStack:[],mainHolder:a}),Hs(_r.getState().getSequenceView(Mo(r)));return}n.breadcrumbs.length>1&&t().navigateTo(0);return}if(_r.getState().saveSequenceView(Mo(i),Xz()),t().resetToRoot(),r!==null){const o=ae.getState().getComposition(r);if(!o){Hs(_r.getState().getSequenceView(Mo(null)));return}const a=js(null);Ks(r),be.getState().setCurrentFrame(0),er(r),e({breadcrumbs:[{compositionId:r,label:o.name}],activeCompositionId:r,stashStack:[],mainHolder:a})}Hs(_r.getState().getSequenceView(Mo(r)))}}));export{Ni as $,yD as A,pD as B,ED as C,TD as D,ID as E,Zh as F,Xs as G,im as H,Oe as I,XD as J,cD as K,QD as L,bn as M,aD as N,La as O,oD as P,YD as Q,Rl as R,$t as S,lD as T,nD as U,_i as V,Kt as W,rD as X,De as Y,Vs as Z,iD as _,em as a,VD as a0,ZD as a1,ip as a2,sD as a3,wD as a4,B2 as a5,MD as a6,zh as a7,OD as a8,U2 as a9,HD as aA,gD as aB,CD as aC,BD as aD,L2 as aE,f1 as aF,jD as aG,Li as aa,id as ab,zD as ac,DD as ad,RD as ae,LD as af,UD as ag,j2 as ah,ND as ai,H2 as aj,q2 as ak,_D as al,FD as am,AD as an,qD as ao,KD as ap,GD as aq,dD as ar,ql as as,$D as at,Mt as au,sh as av,WD as aw,Gu as ax,jv as ay,uD as az,$r as b,k2 as c,P2 as d,kD as e,w2 as f,bD as g,PD as h,mD as i,uc as j,Nh as k,SD as l,jh as m,xD as n,qv as o,Lv as p,m2 as q,Hh as r,hD as s,fD as t,vt as u,Dh as v,Vh as w,v2 as x,vD as y,Lh as z};
