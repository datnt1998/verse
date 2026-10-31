import { captureProjectState, captureContextState, commitProjectState, createProjectStateDraft, diffProjectState } from './project-state'
import { createIdFactory, withIdFactory, type IdFactory } from './id-factory'
import { CommandHistory, mergeStatePatches, type HistoryEntry } from './history'
import { getCommand } from './registry'
import type { CommandIssue, CommitEvent, ExecuteResult, SerializedCommand } from './types'
import { useCompositionNavigationStore } from '../features/timeline/stores/composition-navigation-store'
import { useTimelineSettingsStore } from '../features/timeline/stores/timeline-settings-store'
import { useCommandHistory } from './history-view'
import { setActiveCompositionId } from '../features/timeline/stores/composition-navigation-active'
let mergeOverride: string | null | undefined
export function withCommandMergeKey<T>(key: string | null, action: () => T): T {
  const previous = mergeOverride
  mergeOverride = key
  try { return action() } finally { mergeOverride = previous }
}

export class CommandExecutor {
  readonly history = new CommandHistory()
  private depth = 0
  private listeners = new Set<(event: CommitEvent) => void>()
  constructor(readonly ids: IdFactory = createIdFactory()) {}
  get contextKey(): string { return useCompositionNavigationStore.getState().activeCompositionId ?? '__root__' }
  get canUndo(): boolean { return this.history.stacks(this.contextKey).undo.length > 0 }
  get canRedo(): boolean { return this.history.stacks(this.contextKey).redo.length > 0 }
  on(event: 'commit', listener: (event: CommitEvent) => void): () => void {
    void event
    this.listeners.add(listener)
    return () => { this.listeners.delete(listener) }
  }
  executeCommands(commands: SerializedCommand[], options: { dryRun?: boolean } = {}): ExecuteResult {
    const before = captureProjectState()
    const contextKey = this.contextKey
    const navigationBefore = useCompositionNavigationStore.getState()
    const dirtyBefore = useTimelineSettingsStore.getState().isDirty
    const idCheckpoint = this.ids.checkpoint?.()
    let commandIndex = -1
    const issues: CommandIssue[] = []
    const parsed = commands.map((command, commandIndex) => {
      const definition = getCommand(command.type)
      if (!definition) { issues.push({ message: `Unknown command: ${command.type}`, commandIndex }); return null }
      const result = definition.params.safeParse(command.params)
      if (!result.success) {
        issues.push(...result.error.issues.map(issue => ({ message: issue.message, path: issue.path.join('.'), commandIndex })))
        return null
      }
      // Referential validation runs on the staged state in command order.
      return { definition, params: result.data, command: { type: command.type, params: result.data } }
    })
    if (issues.length) return { ok: false, issues }
    // Dry runs stage the full sequence and restore it without history or commit events.
    const results: unknown[] = []
    const groups = new Map<string, HistoryEntry>()
    const outer = this.depth === 0
    this.depth++
    try {
      withIdFactory(this.ids, () => {
        for (let index = 0; index < parsed.length; index++) {
          commandIndex = index
          const entry = parsed[index]
          if (!entry) continue
          const stepBefore = captureProjectState()
          const stepContextKey = this.contextKey
          const validation = entry.definition.validate?.(stepBefore, entry.params) ?? []
          if (validation.length) {
            issues.push(...validation.map(issue => ({ ...issue, commandIndex: index })))
            throw new Error(validation.map(issue => issue.message).join('; '))
          }
          const outcome = entry.definition.apply({ draft: createProjectStateDraft(), ids: this.ids, contextKey: stepContextKey }, entry.params)
          const inverse = 'inverse' in outcome ? outcome.inverse : outcome
          results.push('inverse' in outcome ? outcome.result : undefined)
          if (entry.definition.history !== 'none') {
            const stepAfter = captureContextState(stepContextKey, stepBefore)
            const change = diffProjectState(stepBefore, stepAfter)
            if (Object.keys(change).length) {
              let group = groups.get(stepContextKey)
              if (!group) {
                group = { commands: [], inverses: [], redoPatch: {}, contextKey: stepContextKey, mergeKey: parsed.length === 1 ? (mergeOverride !== undefined ? mergeOverride : entry.definition.mergeKey?.(entry.params) ?? null) : null, timestamp: Date.now() }
                groups.set(stepContextKey, group)
              }
              group.commands.push(entry.command)
              group.inverses.unshift(inverse.kind === 'patch' && Object.keys(inverse.patch).length === 0 ? { kind: 'patch', patch: change } : inverse)
              group.redoPatch = mergeStatePatches(group.redoPatch, diffProjectState(stepAfter, stepBefore))
            }
          }
        }
      })
    } catch (error) {
      commitProjectState(before)
      useCompositionNavigationStore.setState(navigationBefore)
      setActiveCompositionId(navigationBefore.activeCompositionId)
      useTimelineSettingsStore.getState().setIsDirty(dirtyBefore)
      if (this.ids.checkpoint && this.ids.restore) this.ids.restore(idCheckpoint)
      return { ok: false, issues: issues.length ? issues : [{ message: error instanceof Error ? error.message : String(error), commandIndex }], error: error instanceof Error ? error.message : String(error) }
    } finally { this.depth-- }
    const after = captureProjectState()
    if (options.dryRun) {
      commitProjectState(before)
      useCompositionNavigationStore.setState(navigationBefore)
      setActiveCompositionId(navigationBefore.activeCompositionId)
      useTimelineSettingsStore.getState().setIsDirty(dirtyBefore)
      if (this.ids.checkpoint && this.ids.restore) this.ids.restore(idCheckpoint)
      return { ok: true, commands, results }
    }
    const applied = parsed.flatMap(entry => entry ? [entry.command] : [])
    if (outer) {
      for (const group of groups.values()) this.history.record(group)
      this.emit({ kind: 'execute', commands: applied, contextKey, before, after })
    }
    return { ok: true, commands: applied, results }
  }
  executeAnonymous<T>(type: string, action: () => T, params?: unknown): T {
    if (this.depth) return action()
    const before = captureProjectState()
    const contextKey = this.contextKey
    this.depth++
    let result: T
    try { result = withIdFactory(this.ids, action) } catch (error) { commitProjectState(before); throw error } finally { this.depth-- }
    const after = captureProjectState()
    const patch = diffProjectState(before, after)
    const commands = [{ type, params: params ?? {} }]
    if (Object.keys(patch).length) this.history.record({ commands, inverses: [{ kind: 'patch', patch }], redoPatch: diffProjectState(after, before), contextKey, mergeKey: mergeOverride ?? null, timestamp: Date.now() })
    this.emit({ kind: 'execute', commands, contextKey, before, after })
    return result
  }
  undo(): ExecuteResult {
    const stacks = this.history.stacks(this.contextKey)
    const entry = stacks.undo.at(-1)
    if (!entry) return { ok: false, issues: [{ message: 'Nothing to undo' }] }
    const before = captureProjectState()
    this.depth++
    try {
      for (const inverse of entry.inverses) {
        if (inverse.kind === 'patch') commitProjectState(inverse.patch)
        else {
          const result = this.executeCommands(inverse.commands)
          if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
        }
      }
    } catch (error) { commitProjectState(before); return { ok: false, issues: [{ message: String(error) }] } } finally { this.depth-- }
    stacks.undo.pop()
    stacks.redo.push(entry)
    this.emit({ kind: 'undo', commands: entry.commands, contextKey: entry.contextKey, before, after: captureProjectState() })
    return { ok: true, commands: entry.commands, results: [] }
  }
  redo(): ExecuteResult {
    const stacks = this.history.stacks(this.contextKey)
    const entry = stacks.redo.pop()
    if (!entry) return { ok: false, issues: [{ message: 'Nothing to redo' }] }
    const before = captureProjectState()
    commitProjectState(entry.redoPatch)
    stacks.undo.push(entry)
    this.emit({ kind: 'redo', commands: entry.commands, contextKey: entry.contextKey, before, after: captureProjectState() })
    return { ok: true, commands: entry.commands, results: [] }
  }
  closeMergeGroup(): void {
    mergeOverride = undefined
    const entry = this.history.stacks(this.contextKey).undo.at(-1)
    if (entry) entry.mergeKey = null
  }
  clearHistory(): void {
    this.history.clear()
    useCommandHistory.setState(state => ({ canUndo: false, canRedo: false, revision: state.revision + 1 }))
  }
  private emit(event: CommitEvent): void {
    const dirty = event.commands.some(command => {
      const definition = getCommand(command.type)
      return definition?.dirty ?? definition?.history !== 'none'
    })
    if (dirty && Object.keys(diffProjectState(event.before, event.after)).length) useTimelineSettingsStore.getState().markDirty()
    useCommandHistory.setState(state => ({ canUndo: this.canUndo, canRedo: this.canRedo, revision: state.revision + 1 }))
    for (const listener of this.listeners) listener(event)
  }
}
let activeExecutor: CommandExecutor | undefined
let navigationSubscribed = false
export function getCommandExecutor(): CommandExecutor {
  activeExecutor ??= new CommandExecutor()
  if (!navigationSubscribed) {
    navigationSubscribed = true
    useCompositionNavigationStore.subscribe((state, previous) => {
      if (state.activeCompositionId === previous.activeCompositionId) return
      const executor = getCommandExecutor()
      useCommandHistory.setState(view => ({ canUndo: executor.canUndo, canRedo: executor.canRedo, revision: view.revision + 1 }))
    })
  }
  return activeExecutor
}
export function bindCommandExecutor(executor: CommandExecutor): () => void {
  const previous = activeExecutor
  activeExecutor = executor
  return () => { activeExecutor = previous }
}
export function executeCommands(commands: SerializedCommand[], options?: { dryRun?: boolean }): ExecuteResult { return getCommandExecutor().executeCommands(commands, options) }
