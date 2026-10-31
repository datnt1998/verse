import { z } from 'zod'
import { registerCommand } from '../registry'
import { sanitizeInOutPoints } from '../../features/timeline/utils/in-out-points'
import { getActiveInOutMaxFrame } from '../../features/timeline/stores/in-out-bound'
const markerFields = z.object({ frame: z.number().int().nonnegative(), color: z.string().default('#3B82F6'), label: z.string().default('') })
export function registerMarkerCommands(): void {
  registerCommand({ type: 'marker.add', params: markerFields.extend({ id: z.string().optional() }).strict(), apply({ draft, ids }, params) {
    const marker = { ...params, id: params.id ?? ids.mint() }
    if (draft.markers.some(existing => existing.id === marker.id)) throw new Error(`Duplicate marker: ${marker.id}`)
    draft.markers = [...draft.markers,marker]
    return { inverse: { kind: 'commands', commands: [{ type:'marker.remove',params:{id:marker.id} }] }, result: marker.id }
  } })
  registerCommand({ type: 'marker.update', params: z.object({ id: z.string(), updates: markerFields.partial() }).strict(), mergeKey: params => `marker.update:${params.id}`, apply({ draft }, params) {
    const marker = draft.markers.find(existing => existing.id === params.id)
    if (!marker) throw new Error(`Unknown marker: ${params.id}`)
    draft.markers = draft.markers.map(existing => existing.id === params.id ? {...existing,...params.updates} : existing)
    return {kind:'patch',patch:{}}
  } })
  registerCommand({ type: 'marker.remove', params: z.object({id:z.string()}).strict(), apply({draft},params) {
    if (!draft.markers.some(marker => marker.id === params.id)) throw new Error(`Unknown marker: ${params.id}`)
    draft.markers = draft.markers.filter(marker => marker.id !== params.id)
    return {kind:'patch',patch:{}}
  } })
  registerCommand({ type: 'marker.clear', params: z.object({}).strict(), apply({draft}) { draft.markers=[]; return {kind:'patch',patch:{}} } })
  registerCommand({ type: 'marker.setInOut', params: z.object({inPoint:z.number().int().nonnegative().nullable(),outPoint:z.number().int().nonnegative().nullable()}).strict(), apply({draft},params) {
    const maxFrame = getActiveInOutMaxFrame()
    const sanitized = sanitizeInOutPoints({...params,maxFrame})
    draft.inPoint = sanitized.inPoint
    draft.outPoint = sanitized.outPoint
    return {kind:'patch',patch:{}}
  } })
  registerCommand({ type: 'marker.setIn', params: z.object({ frame: z.number().finite() }).strict(), apply({draft},params) {
    const maximum = getActiveInOutMaxFrame()
    const frame = Math.max(0,Math.min(params.frame,maximum))
    if (draft.outPoint === null || frame >= draft.outPoint) draft.outPoint = maximum
    draft.inPoint = frame
    return {kind:'patch',patch:{}}
  } })
  registerCommand({ type: 'marker.setOut', params: z.object({ frame: z.number().finite() }).strict(), apply({draft},params) {
    const maximum = getActiveInOutMaxFrame()
    const frame = Math.max(1,Math.min(params.frame,maximum))
    if (draft.inPoint === null || frame <= draft.inPoint) draft.inPoint = 0
    draft.outPoint = frame
    return {kind:'patch',patch:{}}
  } })
}
