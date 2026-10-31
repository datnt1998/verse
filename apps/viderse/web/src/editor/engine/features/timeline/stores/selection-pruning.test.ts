import { beforeEach, describe, expect, it } from 'vitest'
import { executeCommands, getCommandExecutor } from '@editor/commands'
import { useSelectionStore } from '@editor/shared/state/selection'
import { makeTimelineTrack, makeTimelineVideoItem, resetTimelineCompositionTestState } from '../test-helpers'
import { useItemsStore } from './items-store'
import { useTimelineStore } from './timeline-store'

const itemIds = () => useItemsStore.getState().items.map((item) => item.id)
const trackIds = () => useItemsStore.getState().tracks.map((track) => track.id)

describe('selection after deleting', () => {
  beforeEach(() => {
    resetTimelineCompositionTestState()
    getCommandExecutor().clearHistory()
    useSelectionStore.getState().selectItems([])
    useSelectionStore.getState().setActiveTrack(null)
    useItemsStore.getState().setTracks([
      makeTimelineTrack({ id: 'v1', name: 'V1', kind: 'video', order: 0 }),
      makeTimelineTrack({ id: 'v2', name: 'V2', kind: 'video', order: 1 }),
    ])
    useItemsStore.getState().setItems([
      makeTimelineVideoItem({ id: 'a', trackId: 'v1' }),
      makeTimelineVideoItem({ id: 'b', trackId: 'v1', from: 60 }),
      makeTimelineVideoItem({ id: 'c', trackId: 'v2' }),
    ])
  })

  it('drops a deleted clip from the selection and brings the clip back on undo', () => {
    useSelectionStore.getState().selectItems(['a', 'c'])

    useTimelineStore.getState().removeItems(['a'])

    expect(itemIds()).toEqual(['b', 'c'])
    expect(useSelectionStore.getState().selectedItemIds).toEqual(['c'])

    getCommandExecutor().undo()
    expect(itemIds()).toEqual(['a', 'b', 'c'])
  })

  it('clears the selection when the only selected clip is deleted', () => {
    useSelectionStore.getState().selectItems(['b'])

    useTimelineStore.getState().removeItems(['b'])

    expect(useSelectionStore.getState().selectedItemIds).toEqual([])
    expect(useSelectionStore.getState().selectionType).toBeNull()
  })

  it('forgets clips and the active track when their track is deleted, and restores the track on undo', () => {
    useSelectionStore.getState().setActiveTrack('v1')
    useSelectionStore.getState().selectItems(['a', 'c'])

    expect(executeCommands([{ type: 'track.remove', params: { ids: ['v1'] } }]).ok).toBe(true)

    expect(trackIds()).toEqual(['v2'])
    expect(itemIds()).toEqual(['c'])
    expect(useSelectionStore.getState().selectedItemIds).toEqual(['c'])
    expect(useSelectionStore.getState().activeTrackId).toBeNull()
    expect(useSelectionStore.getState().selectedTrackIds).toEqual([])

    getCommandExecutor().undo()
    expect(trackIds()).toEqual(['v1', 'v2'])
    expect(itemIds()).toEqual(['a', 'b', 'c'])
  })

  it('deleting an empty track keeps the clips that are selected elsewhere', () => {
    useItemsStore.getState().setTracks([
      ...useItemsStore.getState().tracks,
      makeTimelineTrack({ id: 'v3', name: 'V3', kind: 'video', order: 2 }),
    ])
    useSelectionStore.getState().setActiveTrack('v3')
    useSelectionStore.getState().selectItems(['a'])

    executeCommands([{ type: 'track.remove', params: { ids: ['v3'] } }])

    expect(trackIds()).toEqual(['v1', 'v2'])
    expect(useSelectionStore.getState().selectedItemIds).toEqual(['a'])
    expect(useSelectionStore.getState().activeTrackId).toBeNull()

    getCommandExecutor().undo()
    expect(trackIds()).toEqual(['v1', 'v2', 'v3'])
  })
})
