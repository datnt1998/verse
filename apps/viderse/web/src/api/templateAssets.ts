import { list, record, str } from './guard'

export type TemplateSharedAsset = {
  id: string
  name: string
  kind: 'video' | 'audio' | 'image' | 'text' | 'shape' | 'group'
  loop: boolean
  visible: boolean
}

/** Dynamic slots are replaced for each video; the remaining layers are shared. */
export function readTemplateSharedAssets(snapshot: Record<string, unknown>, path: string): TemplateSharedAsset[] | undefined {
  if (snapshot.project == null) return undefined
  const project = record(snapshot.project, `${path}.project`)
  if (project.timeline == null) return undefined
  const timeline = record(project.timeline, `${path}.project.timeline`)
  const tracks = list(timeline.tracks ?? [], record, `${path}.project.timeline.tracks`)
  const byTrack = new Map<string, Record<string, unknown>[]>()
  for (const item of list(timeline.items ?? [], record, `${path}.project.timeline.items`)) {
    if (typeof item.trackId !== 'string') continue
    const items = byTrack.get(item.trackId)
    if (items) items.push(item)
    else byTrack.set(item.trackId, [item])
  }
  const assets: TemplateSharedAsset[] = []
  for (const track of tracks) {
    if (track.templateSlot) continue
    const id = str(track.id, `${path}.project.timeline.track.id`)
    const items = byTrack.get(id)
    if (!items?.length) continue
    const type = items[0].type
    const kind = items.every(item => item.type === type) && (type === 'video' || type === 'audio' || type === 'image' || type === 'text' || type === 'shape') ? type : 'group'
    const name = typeof track.name === 'string' && track.name.trim() ? track.name.trim()
      : typeof items[0].label === 'string' && items[0].label.trim() ? items[0].label.trim() : 'Lớp dùng chung'
    assets.push({ id, name, kind, loop: items.some(item => item.loop === true), visible: track.visible !== false && !(track.kind === 'audio' && track.muted === true) })
  }
  return assets
}
