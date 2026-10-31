import type { z } from 'zod'
import type { ProjectState, ProjectStateDraft } from './project-state'
import type { IdFactory } from './id-factory'

export type SerializedCommand = { type: string; params: unknown }
export type EntityCollection = 'items' | 'tracks' | 'transitions' | 'keyframes' | 'markers' | 'compositions' | 'mediaItems'
export interface EntityPatch { values: Record<string, unknown | null>; order?: string[] }
export type StatePatch = Partial<ProjectState> & { entities?: Partial<Record<EntityCollection, EntityPatch>> }
export type Inverse = { kind: 'commands'; commands: SerializedCommand[] } | { kind: 'patch'; patch: StatePatch }
export interface CommandIssue { message: string; code?: string; path?: string; commandIndex?: number }
export type ProjectStateView = Readonly<ProjectState>
export interface ApplyContext { draft: ProjectStateDraft; ids: IdFactory; contextKey: string }
export interface ApplyOutcome { inverse: Inverse; result?: unknown }
export interface CommandDefinition<P> {
  type: string
  params: z.ZodType<P>
  history?: 'record' | 'none'
  dirty?: boolean
  validate?(state: ProjectStateView, params: P): CommandIssue[]
  apply(ctx: ApplyContext, params: P): Inverse | ApplyOutcome
  mergeKey?(params: P): string | null
}
export type ExecuteResult = { ok: true; results: unknown[]; commands: SerializedCommand[] } | { ok: false; issues: CommandIssue[]; error?: string }
export interface CommitEvent { kind: 'execute' | 'undo' | 'redo'; commands: SerializedCommand[]; contextKey: string; before: ProjectState; after: ProjectState }
