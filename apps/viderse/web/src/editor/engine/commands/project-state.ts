import type { TimelineSnapshot } from '@editor/types/timeline-state'
import { useItemsStore } from '../features/timeline/stores/items-store'
import { useTransitionsStore } from '../features/timeline/stores/transitions-store'
import { useKeyframesStore } from '../features/timeline/stores/keyframes-store'
import { useMarkersStore } from '../features/timeline/stores/markers-store'
import { useCompositionsStore } from '../features/timeline/stores/compositions-store'
import { useSequencesStore } from '../features/timeline/stores/sequences-store'
import { useTimelineSettingsStore } from '../features/timeline/stores/timeline-settings-store'
import { useCompositionNavigationStore, saveCurrentToComposition, type CompositionNavigationState, type CompositionNavigationActions, type StashedTimeline } from '../features/timeline/stores/composition-navigation-store'
import { usePlaybackStore } from '../shared/state/playback'
import { useProjectStore } from '../features/projects/stores/project-store'
import type { MediaMetadata } from '../types/storage'
import { useMediaLibraryStore } from '../features/media-library/stores/media-library-store'
import type { EntityCollection, StatePatch } from './types'
const entityCollections: EntityCollection[] = ['items', 'tracks', 'transitions', 'keyframes', 'markers', 'compositions', 'mediaItems']
function entityId(entity: unknown, collection: EntityCollection): string {
  if (!entity || typeof entity !== 'object') throw new Error(`Invalid ${collection} entity`)
  const id = Reflect.get(entity, collection === 'keyframes' ? 'itemId' : 'id')
  if (typeof id !== 'string') throw new Error(`Missing ${collection} entity id`)
  return id
}

export type ProjectState = TimelineSnapshot & { projectName?: string; mediaItems?: MediaMetadata[]; stashStack?: StashedTimeline[]; mainHolder?: StashedTimeline | null }
export type ProjectStateDraft = ProjectState & { readonly nav: CompositionNavigationState & CompositionNavigationActions }
export function captureProjectState(): ProjectState {
  const items = useItemsStore.getState()
  const markers = useMarkersStore.getState()
  const settings = useTimelineSettingsStore.getState()
  const playback = usePlaybackStore.getState()
  const project = useProjectStore.getState().currentProject
  const media = useMediaLibraryStore.getState()
  const pendingMediaIds = new Set(media.importingIds)
  return {
    projectName: project?.name,
    mediaItems: pendingMediaIds.size ? media.mediaItems.filter(item => !pendingMediaIds.has(item.id)) : media.mediaItems,
    stashStack: useCompositionNavigationStore.getState().stashStack,
    mainHolder: useCompositionNavigationStore.getState().mainHolder,
    items: items.items, tracks: items.tracks,
    transitions: useTransitionsStore.getState().transitions,
    keyframes: useKeyframesStore.getState().keyframes,
    markers: markers.markers, compositions: useCompositionsStore.getState().compositions,
    topLevelSequenceIds: useSequencesStore.getState().topLevelSequenceIds,
    inPoint: markers.inPoint, outPoint: markers.outPoint,
    fps: settings.fps, scrollPosition: settings.scrollPosition, snapEnabled: settings.snapEnabled,
    currentFrame: playback.currentFrame, busAudioEq: playback.busAudioEq, masterBusDb: playback.masterBusDb,
    projectId: project?.id ?? null, projectMetadata: project ? { ...project.metadata } : null,
  }
}
export function commitProjectState(input: StatePatch): void {
  const patch = { ...input }
  if (input.entities) {
    const current = captureProjectState()
    for (const collection of entityCollections) {
      const delta = input.entities[collection]
      if (!delta) continue
      const existing = new Map<string, unknown>((current[collection] ?? []).map(entity => [entityId(entity, collection), entity]))
      for (const [id, value] of Object.entries(delta.values)) {
        if (value === null) existing.delete(id)
        else existing.set(id, value)
      }
      const order = delta.order ?? [...existing.keys()]
      Reflect.set(patch, collection, order.flatMap(id => existing.has(id) ? [existing.get(id)] : []))
    }
  }
  if ('items' in patch) useItemsStore.getState().setItems(patch.items!)
  // Canonical restores must preserve authored heights and empty groups; the UI setter derives local view preferences.
  if ('tracks' in patch) useItemsStore.setState({ tracks: patch.tracks! })
  if ('transitions' in patch) useTransitionsStore.getState().setTransitions(patch.transitions!)
  if ('keyframes' in patch) useKeyframesStore.getState().setKeyframes(patch.keyframes!)
  if ('compositions' in patch) useCompositionsStore.getState().setCompositions(patch.compositions!)
  if ('topLevelSequenceIds' in patch) useSequencesStore.getState().setTopLevelSequenceIds(patch.topLevelSequenceIds!)
  if ('stashStack' in patch) useCompositionNavigationStore.setState({ stashStack: patch.stashStack ?? [] })
  if ('mainHolder' in patch) useCompositionNavigationStore.setState({ mainHolder: patch.mainHolder ?? null })
  if ('markers' in patch) useMarkersStore.getState().setMarkers(patch.markers!)
  if ('inPoint' in patch) useMarkersStore.getState().setInPoint(patch.inPoint!)
  if ('outPoint' in patch) useMarkersStore.getState().setOutPoint(patch.outPoint!)
  if ('fps' in patch) useTimelineSettingsStore.getState().setFps(patch.fps!)
  if ('scrollPosition' in patch) useTimelineSettingsStore.getState().setScrollPosition(patch.scrollPosition!)
  if ('snapEnabled' in patch) useTimelineSettingsStore.getState().setSnapEnabled(patch.snapEnabled!)
  if ('currentFrame' in patch) usePlaybackStore.getState().setCurrentFrame(patch.currentFrame!)
  if ('busAudioEq' in patch) usePlaybackStore.getState().setBusAudioEq(patch.busAudioEq)
  if ('masterBusDb' in patch) usePlaybackStore.getState().setMasterBusDb(patch.masterBusDb!)
  if ('mediaItems' in patch) {
    const media = useMediaLibraryStore.getState()
    const pendingMediaIds = new Set(media.importingIds)
    const placeholders = media.mediaItems.filter(item => pendingMediaIds.has(item.id))
    const mediaItems = [...placeholders, ...(patch.mediaItems ?? []).filter(item => !pendingMediaIds.has(item.id))]
    useMediaLibraryStore.setState({ mediaItems, mediaById: Object.fromEntries(mediaItems.map(media => [media.id, media])) })
  }
  if ('projectId' in patch || 'projectMetadata' in patch || 'projectName' in patch) {
    useProjectStore.setState(state => {
      if ('projectId' in patch && patch.projectId === null) return { currentProject: null }
      const current = state.currentProject
      const id = patch.projectId ?? current?.id ?? 'command-project'
      const metadata = patch.projectMetadata ?? current?.metadata ?? { width: 1920, height: 1080, fps: patch.fps ?? 30 }
      const project = { description: '', createdAt: 0, updatedAt: 0, duration: 0, ...current, id, name: patch.projectName ?? current?.name ?? 'Dự án', metadata }
      return { currentProject: project, projects: state.projects.map(existing => existing.id === id ? project : existing) }
    })
  }
}
export function createProjectStateDraft(): ProjectStateDraft {
  const draft = {} as ProjectStateDraft
  for (const key of Object.keys(captureProjectState()) as (keyof ProjectState)[]) {
    Object.defineProperty(draft, key, { enumerable: true, get: () => captureProjectState()[key], set: (value: unknown) => commitProjectState({ [key]: value }) })
  }
  Object.defineProperty(draft, 'nav', { get: () => useCompositionNavigationStore.getState() })
  Object.defineProperty(draft, 'sequenceViews', { get: () => useSequencesStore.getState().sequenceViewById })
  return draft
}
export function diffProjectState(before: ProjectState, after: ProjectState): StatePatch {
  const patch: StatePatch = {}
  for (const key of Object.keys(before) as (keyof ProjectState)[]) {
    if (before[key] === after[key]) continue
    if (entityCollections.includes(key as EntityCollection)) {
      const collection = key as EntityCollection
      const previous = before[collection] ?? []
      const next = after[collection] ?? []
      const previousById = new Map(previous.map(entity => [entityId(entity, collection), entity]))
      const nextById = new Map(next.map(entity => [entityId(entity, collection), entity]))
      const values: Record<string, unknown | null> = {}
      // Edits replace only the entities they touch, so a shared reference is unchanged; serializing every
      // entity (a caption's word list, media metadata) on each colour-drag step cost most of the edit.
      for (const [id, entity] of previousById) {
        const nextEntity = nextById.get(id)
        if (entity !== nextEntity && JSON.stringify(entity) !== JSON.stringify(nextEntity)) values[id] = entity
      }
      for (const id of nextById.keys()) if (!previousById.has(id)) values[id] = null
      const oldOrder = [...previousById.keys()]
      const newOrder = [...nextById.keys()]
      const orderChanged = oldOrder.length !== newOrder.length || oldOrder.some((id, index) => id !== newOrder[index])
      if (Object.keys(values).length || orderChanged) {
        patch.entities ??= {}
        patch.entities[collection] = { values, ...(orderChanged ? { order: oldOrder } : {}) }
      }
    } else if (JSON.stringify(before[key]) !== JSON.stringify(after[key])) Reflect.set(patch, key, before[key])
  }
  return patch
}
export function captureContextState(contextKey: string, before: ProjectState): ProjectState {
  const navigation = useCompositionNavigationStore.getState()
  if ((navigation.activeCompositionId ?? '__root__') === contextKey) return captureProjectState()
  if (navigation.activeCompositionId) saveCurrentToComposition(navigation.activeCompositionId)
  const state = captureProjectState()
  const holder = navigation.stashStack.find(stash => (stash.compositionId ?? '__root__') === contextKey)
    ?? (contextKey === '__root__' ? navigation.mainHolder : null)
  if (!holder) return state
  return {
    ...state, items: holder.items, tracks: holder.tracks, transitions: holder.transitions, keyframes: holder.keyframes,
    markers: holder.markers, inPoint: holder.inPoint, outPoint: holder.outPoint, currentFrame: holder.currentFrame, busAudioEq: holder.busAudioEq,
    stashStack: before.stashStack, mainHolder: before.mainHolder,
  }
}
