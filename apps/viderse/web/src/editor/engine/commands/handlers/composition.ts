import { z } from 'zod'
import { registerCommand } from '../registry'
import { createPreComp, dissolvePreComp, renameCompoundClip, performRepairCompositeCompositionEditorialLeak } from '../../features/timeline/stores/actions/composition-actions'
import { useCompositionNavigationStore } from '../../features/timeline/stores/composition-navigation-store'
export function registerCompositionCommands(): void {
  registerCommand({ type:'composition.create',params:z.object({name:z.string().optional(),itemIds:z.array(z.string()).min(1),openAfterCreate:z.boolean().optional(),editorKind:z.enum(['sequence','composite-2d']).optional()}).strict(),apply({draft},params) {
    for (const id of params.itemIds) if (!draft.items.some(item=>item.id===id)) throw new Error(`Unknown item: ${id}`)
    const result=createPreComp(params.name,params.itemIds,{openAfterCreate:false,editorKind:params.editorKind})
    if (!result) throw new Error('Unable to create composition from selection')
    if (params.openAfterCreate && result.compositionId) {
      if (params.editorKind === 'composite-2d') draft.nav.switchToSequence(result.compositionId)
      else draft.nav.enterComposition(result.compositionId, params.name ?? result.label, result.id)
    }
    return {inverse:{kind:'patch',patch:{}},result}
  } })
  registerCommand({type:'composition.dissolve',params:z.object({itemId:z.string()}).strict(),apply(_ctx,params) {
    if (!dissolvePreComp(params.itemId)) throw new Error(`Unable to dissolve composition: ${params.itemId}`)
    return {kind:'patch',patch:{}}
  }})
  registerCommand({type:'composition.rename',params:z.object({compositionId:z.string(),name:z.string().trim().min(1)}).strict(),apply(_ctx,params) {
    if (!renameCompoundClip(params.compositionId,params.name)) throw new Error(`Unknown composition: ${params.compositionId}`)
    return {kind:'patch',patch:{}}
  }})
  registerCommand({type:'composition.updateSettings',params:z.object({compositionId:z.string(),width:z.number().int().positive().optional(),height:z.number().int().positive().optional(),fps:z.number().positive().max(240).optional(),durationInFrames:z.number().int().positive().optional(),backgroundColor:z.string().optional()}).strict(),apply({draft},params) {
    if(!draft.compositions.some(composition=>composition.id===params.compositionId)) throw new Error(`Unknown composition: ${params.compositionId}`)
    const {compositionId,...updates}=params
    draft.compositions=draft.compositions.map(composition=>composition.id===compositionId?{...composition,...updates}:composition)
    return {kind:'patch',patch:{}}
  }})
  registerCommand({type:'composition.enter',history:'none',params:z.object({compositionId:z.string(),label:z.string().optional(),entryItemId:z.string().optional()}).strict(),apply({draft},params) {
    const composition=draft.compositions.find(entry=>entry.id===params.compositionId)
    if(!composition) throw new Error(`Unknown composition: ${params.compositionId}`)
    if (composition.editorKind === 'composite-2d' || draft.topLevelSequenceIds.includes(composition.id)) draft.nav.switchToSequence(composition.id)
    else draft.nav.enterComposition(composition.id, params.label ?? composition.name, params.entryItemId)
    return {kind:'commands',commands:[{type:'composition.exit',params:{}}]}
  }})
  registerCommand({type:'composition.exit',history:'none',params:z.object({}).strict(),apply({draft}) {
    const compositionId=draft.nav.activeCompositionId
    draft.nav.exitComposition()
    return {kind:'commands',commands:compositionId?[{type:'composition.enter',params:{compositionId}}]:[]}
  }})
  registerCommand({type:'composition.openTab',history:'none',params:z.object({compositionId:z.string()}).strict(),apply({draft},params) {
    const composition=draft.compositions.find(entry=>entry.id===params.compositionId)
    if(!composition || composition.editorKind==='composite-2d') throw new Error('Sequence tab requires an editorial composition')
    if(!draft.topLevelSequenceIds.includes(params.compositionId)) draft.topLevelSequenceIds=[...draft.topLevelSequenceIds,params.compositionId]
    draft.nav.switchToSequence(params.compositionId)
    return {kind:'commands',commands:[{type:'composition.closeTab',params}]}
  }})
  registerCommand({type:'composition.closeTab',history:'none',params:z.object({compositionId:z.string()}).strict(),apply({draft},params) {
    if(draft.nav.breadcrumbs[0]?.compositionId===params.compositionId) draft.nav.switchToSequence(null)
    draft.topLevelSequenceIds=draft.topLevelSequenceIds.filter(id=>id!==params.compositionId)
    return {kind:'commands',commands:[{type:'composition.openTab',params}]}
  }})
  registerCommand({type:'composition.repairEditorialLeak',params:z.object({compositionId:z.string(),editorialItemIds:z.array(z.string()),suggestedDurationInFrames:z.number().optional()}).strict(),apply(_ctx,params) {
    const result=performRepairCompositeCompositionEditorialLeak(params)
    return {inverse:{kind:'patch',patch:{}},result}
  }})
  registerCommand({type:'composition.switchSequence',history:'none',params:z.object({compositionId:z.string().nullable()}).strict(),apply(_ctx,params) {
    useCompositionNavigationStore.getState().switchToSequence(params.compositionId)
    return {kind:'patch',patch:{}}
  }})
}
