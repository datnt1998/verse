import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import type { Project } from '../types/project'
import { createNodeSession, createSeededIdFactory, getCapabilityManifest, type ProjectState } from './index'
import type { TimelineItem } from '../types/timeline'
test('plain Freecut project JSON executes and undoes without DOM globals',()=>{
  assert.equal(typeof window,'undefined')
  assert.equal(typeof document,'undefined')
  const project = JSON.parse(readFileSync(new URL('./fixtures/project.json',import.meta.url),'utf8')) as Project
  const timeline = project.timeline!
  const state:ProjectState={
    items: timeline.items as TimelineItem[],
    tracks: timeline.tracks.map(track => ({ ...track, items: [] })),
    transitions: timeline.transitions ?? [],
    keyframes: timeline.keyframes ?? [],
    markers: timeline.markers ?? [],
    compositions: (timeline.compositions ?? []) as ProjectState['compositions'],
    topLevelSequenceIds: timeline.topLevelSequenceIds ?? [],
    inPoint: timeline.inPoint ?? null,
    outPoint: timeline.outPoint ?? null,
    fps: project.metadata.fps,
    scrollPosition: timeline.scrollPosition ?? 0,
    snapEnabled: true,
    currentFrame: 0,
    masterBusDb: timeline.masterBusDb ?? 0,
    projectId: project.id,
    projectName: project.name,
    projectMetadata: project.metadata,
    mediaItems: [], stashStack: [], mainHolder: null,
  }
  const session=createNodeSession(state,createSeededIdFactory('node'))
  try {
    const initial=session.getState()
    const result=session.executor.executeCommands([
      {type:'track.add',params:{id:'extra',name:'A1',kind:'audio'}},
      {type:'track.update',params:{id:'extra',updates:{name:'Audio'}}},
      {type:'marker.add',params:{id:'headless-marker',frame:20}},
      {type:'project.updateSettings',params:{fps:24,width:1280,height:720}},
    ])
    assert.equal(result.ok,true)
    const changed=session.getState()
    assert.equal(changed.fps,24)
    assert.equal(changed.projectMetadata?.width,1280)
    assert.equal(changed.markers.length,1)
    assert.ok(changed.tracks.some(track=>track.id==='extra'&&track.name==='Audio'))
    assert.equal(session.executor.undo().ok,true)
    assert.deepEqual(session.getState(),initial)
    assert.equal(session.executor.redo().ok,true)
    assert.deepEqual(session.getState(),changed)
    const capabilities=getCapabilityManifest()
    assert.ok(capabilities.commands.some(command=>command.type==='marker.add'))
    assert.ok(capabilities.effects.length>0&&capabilities.transitions.length>0&&capabilities.blendModes.length>0&&capabilities.easings.length>0)
    assert.equal(typeof window,'undefined')
    assert.equal(typeof document,'undefined')
  } finally {session.dispose()}
})
