import { describe, expect, it } from 'vitest'
import { createNodeSession, createSeededIdFactory, serializeCommand, deserializeCommand, captureProjectState, commitProjectState, getCommandExecutor, registerCommand, withCommandMergeKey } from './index'
import { z } from 'zod'
import { createCommandFixture } from './test-fixture'
import type { ProjectState, SerializedCommand } from './index'
import { captureSnapshot, restoreSnapshot } from '@editor/test/freecut-snapshot'
import { useCompositionNavigationStore } from '../features/timeline/stores/composition-navigation-store'

const scenarios: SerializedCommand[] = [
  {type:'project.rename',params:{name:'Changed'}},
  {type:'project.updateSettings',params:{width:1280,height:720,fps:24,backgroundColor:'#112233'}},
  {type:'project.setTimelineSettings',params:{snapEnabled:false,scrollPosition:44}},
  {type:'project.patch',params:{patch:{masterBusDb:-8}}},
  {type:'track.add',params:{id:'new-track',name:'New',kind:'audio'}},
  {type:'track.remove',params:{ids:['track-video']}},
  {type:'track.update',params:{id:'track-video',updates:{name:'Renamed',locked:true,visible:false,height:100}}},
  {type:'track.reorder',params:{ids:['track-audio','track-video']}},
  {type:'track.set',params:{tracks:createCommandFixture().tracks.map(track=>({...track,name:`${track.name} changed`}))}},
  {type:'marker.add',params:{frame:50}},
  {type:'marker.update',params:{id:'marker-1',updates:{frame:40,label:'Moved',color:'#00ff00'}}},
  {type:'marker.remove',params:{id:'marker-1'}},
  {type:'marker.clear',params:{}},
  {type:'marker.setIn',params:{frame:50}},
  {type:'marker.setOut',params:{frame:200}},
  {type:'marker.setInOut',params:{inPoint:10,outPoint:70}},
  {type:'composition.create',params:{itemIds:['video-1','audio-1'],name:'Compound'}},
]
function checkParity(command: SerializedCommand, fixture: ProjectState, seed: number): void {
  const session=createNodeSession(fixture,createSeededIdFactory(`parity-${seed}`))
  try {
    const before=structuredClone(captureProjectState())
    const snapshot=captureSnapshot()
    const result=session.executor.executeCommands([command])
    expect(result.ok).toBe(true)
    const after=structuredClone(captureProjectState())
    expect(session.executor.undo().ok).toBe(true)
    expect(captureProjectState()).toEqual(before)
    expect(session.executor.redo().ok).toBe(true)
    expect(captureProjectState()).toEqual(after)
    restoreSnapshot(snapshot)
    const snapshotRestored=captureProjectState()
    for(const key of Object.keys(snapshot) as (keyof typeof snapshot)[]) expect(snapshotRestored[key]).toEqual(before[key])
    commitProjectState(before)
    const replay=createNodeSession(before,createSeededIdFactory(`parity-${seed}`))
    try { expect(replay.executor.executeCommands([deserializeCommand(serializeCommand(command))]).ok).toBe(true); expect(captureProjectState()).toEqual(after) } finally { replay.dispose() }
  } finally {session.dispose()}
}
describe('core command snapshot parity',()=>{
  for(const command of scenarios) it(`${command.type} restores seeded snapshots, redoes and replays`,()=>{
    for(const seed of [1,17,911]) checkParity(command,createCommandFixture(seed),seed)
  })
  it('composition metadata, dissolve and leak repair restore scoped state',()=>{
    for(const seed of [1,17,911]) {
      const session=createNodeSession(createCommandFixture(seed),createSeededIdFactory(`setup-${seed}`))
      let created: ProjectState
      try { expect(session.executor.executeCommands([{type:'composition.create',params:{itemIds:['video-1','audio-1'],name:'Compound'}}]).ok).toBe(true); created=structuredClone(captureProjectState()) } finally {session.dispose()}
      const composition=created.compositions[0]
      const wrapper=created.items.find(item=>item.type==='composition' && item.compositionId===composition.id)!
      checkParity({type:'composition.dissolve',params:{itemId:wrapper.id}},created,seed)
      checkParity({type:'composition.rename',params:{compositionId:composition.id,name:'Renamed'}},created,seed)
      checkParity({type:'composition.updateSettings',params:{compositionId:composition.id,width:1280,height:720,fps:24,durationInFrames:300}},created,seed)
      const leaked=structuredClone(created)
      leaked.compositions[0].editorKind='composite-2d'
      checkParity({type:'composition.repairEditorialLeak',params:{compositionId:composition.id,editorialItemIds:[composition.items[0].id]}},leaked,seed)
    }
  })
  it('context navigation parks history and persists unbalanced edits at root',()=>{
    const session=createNodeSession(createCommandFixture(),createSeededIdFactory('nav'))
    try {
      expect(session.executor.executeCommands([{type:'composition.create',params:{itemIds:['video-1','audio-1']}}]).ok).toBe(true)
      const root=structuredClone(captureProjectState())
      const compositionId=root.compositions[0].id
      expect(session.executor.executeCommands([{type:'composition.enter',params:{compositionId}}]).ok).toBe(true)
      expect(session.executor.canUndo).toBe(false)
      expect(session.executor.executeCommands([{type:'marker.add',params:{id:'inside-marker',frame:8}}]).ok).toBe(true)
      const persisted=session.getState()
      expect(persisted.items).toEqual(root.items)
      expect(persisted.compositions[0].markers?.some(marker=>marker.id==='inside-marker')).toBe(true)
      expect(useCompositionNavigationStore.getState().activeCompositionId).toBe(compositionId)
      expect(session.executor.executeCommands([{type:'composition.exit',params:{}}]).ok).toBe(true)
      expect(session.executor.canUndo).toBe(true)
      expect(session.executor.executeCommands([{type:'composition.openTab',params:{compositionId}}]).ok).toBe(true)
      expect(session.executor.executeCommands([{type:'composition.switchSequence',params:{compositionId:null}}]).ok).toBe(true)
      expect(session.executor.executeCommands([{type:'composition.closeTab',params:{compositionId}}]).ok).toBe(true)
      expect(captureProjectState().topLevelSequenceIds).toEqual([])
    } finally {session.dispose()}
  })
  it('create-and-open records the root domain for undo and redo',()=>{
    const session=createNodeSession(createCommandFixture(),createSeededIdFactory('open'))
    try {
      const before=session.getState()
      expect(session.executor.executeCommands([{type:'composition.create',params:{itemIds:['video-1','audio-1'],openAfterCreate:true}}]).ok).toBe(true)
      expect(session.executor.contextKey).not.toBe('__root__')
      expect(session.executor.executeCommands([{type:'composition.exit',params:{}}]).ok).toBe(true)
      const after=session.getState()
      expect(session.executor.undo().ok).toBe(true)
      expect(session.getState()).toEqual(before)
      expect(session.executor.redo().ok).toBe(true)
      expect(session.getState()).toEqual(after)
    } finally {session.dispose()}
  })
  it('view and reset commands do not record undo or dirty hydrated projects',()=>{
    const session=createNodeSession(createCommandFixture())
    try {
      expect(session.executor.executeCommands([{type:'project.setView',params:{scrollPosition:120}},{type:'project.clearTimeline',params:{}}]).ok).toBe(true)
      expect(session.executor.canUndo).toBe(false)
      expect(captureProjectState().scrollPosition).toBe(120)
      expect(captureProjectState().items).toEqual([])
      expect(captureProjectState().tracks).toEqual([])
    } finally {session.dispose()}
  })
  it('track commands write canonical state verbatim and keep it after apply/undo/redo',()=>{
    const session=createNodeSession(createCommandFixture(),createSeededIdFactory('tracks'))
    try {
      const before=structuredClone(captureProjectState())
      expect(session.executor.executeCommands([{type:'track.update',params:{id:'track-video',updates:{height:140}}}]).ok).toBe(true)
      expect(captureProjectState().tracks.find(track=>track.id==='track-video')?.height).toBe(140)
      const verbatim=[...captureProjectState().tracks.map(track=>({...track})),{id:'extra',name:'Extra',kind:'audio' as const,height:42,locked:false,visible:true,muted:false,solo:false,order:2,items:[]}]
      expect(session.executor.executeCommands([{type:'track.set',params:{tracks:verbatim}}]).ok).toBe(true)
      expect(captureProjectState().tracks.find(track=>track.id==='extra')?.height).toBe(42)
      expect(captureProjectState().tracks.find(track=>track.id==='track-video')?.height).toBe(140)
      expect(session.executor.undo().ok).toBe(true)
      expect(captureProjectState().tracks).toHaveLength(2)
      expect(captureProjectState().tracks.find(track=>track.id==='track-video')?.height).toBe(140)
      expect(session.executor.undo().ok).toBe(true)
      expect(captureProjectState()).toEqual(before)
      expect(session.executor.redo().ok).toBe(true)
      expect(session.executor.redo().ok).toBe(true)
      expect(captureProjectState().tracks.find(track=>track.id==='extra')?.height).toBe(42)
      expect(session.executor.executeCommands([{type:'project.patch',params:{patch:{tracks:verbatim.map(track=>track.id==='extra'?{...track,height:70}:track)}}}]).ok).toBe(true)
      expect(captureProjectState().tracks.find(track=>track.id==='extra')?.height).toBe(70)
    } finally {session.dispose()}
  })
  it('track.remove cascades items, transitions and keyframes through the draft',()=>{
    const fixture=createCommandFixture()
    fixture.keyframes=[{itemId:'video-1',properties:[{property:'opacity',keyframes:[{id:'k1',frame:0,value:1,easing:'linear'}]}]}]
    const session=createNodeSession(fixture,createSeededIdFactory('cascade'))
    try {
      expect(session.executor.executeCommands([{type:'track.remove',params:{ids:['track-video']}}]).ok).toBe(true)
      const state=captureProjectState()
      expect(state.tracks.map(track=>track.id)).toEqual(['track-audio'])
      expect(state.items.some(item=>item.id==='video-1')).toBe(false)
      expect(state.keyframes.some(entry=>entry.itemId==='video-1')).toBe(false)
      expect(session.executor.undo().ok).toBe(true)
      expect(captureProjectState().keyframes.some(entry=>entry.itemId==='video-1')).toBe(true)
    } finally {session.dispose()}
  })
  it('validates dependent batches, rolls back failure and dry runs without events',()=>{
    const session=createNodeSession(createCommandFixture(),createSeededIdFactory('atomic'))
    let commits=0
    session.executor.on('commit',()=>{commits++})
    try {
      const before=structuredClone(captureProjectState())
      const commands=[{type:'track.add',params:{id:'batch-track',name:'Batch'}},{type:'track.update',params:{id:'batch-track',updates:{height:99}}}]
      expect(session.executor.executeCommands(commands,{dryRun:true}).ok).toBe(true)
      expect(captureProjectState()).toEqual(before)
      expect(commits).toBe(0)
      const failed=session.executor.executeCommands([...commands,{type:'track.update',params:{id:'missing',updates:{height:99}}}])
      expect(failed.ok).toBe(false)
      if(!failed.ok) expect(failed.issues[0].commandIndex).toBe(2)
      expect(captureProjectState()).toEqual(before)
      expect(commits).toBe(0)
      expect(session.executor.executeCommands(commands).ok).toBe(true)
      expect(commits).toBe(1)
      expect(session.executor.undo().ok).toBe(true)
      expect(captureProjectState()).toEqual(before)
    } finally {session.dispose()}
  })
  it('gesture merging keeps earliest values for newly touched fields',()=>{
    registerCommand({type:'test.change',params:z.object({gain:z.number().optional(),scroll:z.number().optional()}),apply({draft},params){if(params.gain!==undefined)draft.masterBusDb=params.gain;if(params.scroll!==undefined)draft.scrollPosition=params.scroll;return{kind:'patch',patch:{}}}})
    const session=createNodeSession(createCommandFixture())
    try {
      const before=structuredClone(captureProjectState())
      withCommandMergeKey('gesture',()=>session.executor.executeCommands([{type:'test.change',params:{gain:-4}}]))
      withCommandMergeKey('gesture',()=>session.executor.executeCommands([{type:'test.change',params:{gain:-8,scroll:30}}]))
      expect(getCommandExecutor().history.stacks(session.executor.contextKey).undo).toHaveLength(1)
      expect(session.executor.undo().ok).toBe(true)
      expect(captureProjectState()).toEqual(before)
      session.executor.closeMergeGroup()
    } finally {session.dispose()}
  })
})
