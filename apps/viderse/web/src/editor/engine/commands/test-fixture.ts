import type { ProjectState } from './project-state'
import type { TimelineItem, TimelineTrack } from '../types/timeline'
import { getPresetTrackHeight } from '../features/timeline/utils/track-heights'
export function createCommandFixture(seed = 1): ProjectState {
  let value = seed
  const random = () => { value = (Math.imul(value,1664525)+1013904223) >>> 0; return value / 4294967296 }
  const duration = 120 + Math.floor(random()*60)
  const tracks: TimelineTrack[] = ['video','audio'].map((kind,index) => ({id:`track-${kind}`,name:kind,kind:kind as 'video'|'audio',height:getPresetTrackHeight(),locked:false,visible:true,muted:false,solo:false,order:index,items:[]}))
  const items: TimelineItem[] = [
    {id:'video-1',type:'video',trackId:'track-video',from:0,durationInFrames:duration,label:'Video',src:'/fixture.mp4',sourceStart:0,sourceEnd:duration,sourceDuration:900,sourceFps:30,speed:1,linkedGroupId:'linked-av',transform:{x:0,y:0,rotation:0,opacity:1}},
    {id:'audio-1',type:'audio',trackId:'track-audio',from:0,durationInFrames:duration,label:'Audio',src:'/fixture.wav',sourceStart:0,sourceEnd:duration,sourceDuration:900,sourceFps:30,speed:1,linkedGroupId:'linked-av'},
    {id:'video-2',type:'video',trackId:'track-video',from:duration+30,durationInFrames:duration,label:'Second',src:'/fixture.mp4',sourceStart:0,sourceEnd:duration,sourceDuration:900,sourceFps:30,speed:1},
  ]
  return {items,tracks,transitions:[],keyframes:[],markers:[{id:'marker-1',frame:20,label:'Marker',color:'#ff0000'}],compositions:[],topLevelSequenceIds:[],inPoint:null,outPoint:null,fps:30,scrollPosition:0,snapEnabled:true,currentFrame:0,busAudioEq:undefined,masterBusDb:0,projectId:'fixture-project',projectName:'Fixture',projectMetadata:{width:1920,height:1080,fps:30,backgroundColor:'#000000'},stashStack:[],mainHolder:null,mediaItems:[]}
}
