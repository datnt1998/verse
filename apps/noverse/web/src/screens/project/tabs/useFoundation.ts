import { getPremise, getWorldRules, type PremiseSections, type WorldRule } from '../../../api/client'
import { premiseRows, type FoundationRow } from './foundation'
import { keys, useResource, type Resource } from '../../../data'

export type FoundationResource = {
  premise: Resource<PremiseSections>
  world: Resource<WorldRule[]>
  rows: FoundationRow[]
  required: FoundationRow[]
  ready: boolean
}
export function useFoundation(project: string, story: string): FoundationResource {
  const premise = useResource(keys.story(project, story, 'premise'), ({ signal }) => getPremise(project, story, signal))
  const world = useResource(keys.story(project, story, 'world-rules'), ({ signal }) => getWorldRules(project, story, signal))
  const sections = premise.data?.sections ?? []
  const missing = premise.data?.missing ?? []
  const rows = premiseRows(sections, missing)
  return { premise, world, rows, required: rows, ready: premise.data !== undefined }
}
