export const ROOT_HISTORY_CONTEXT = '__root__'
import type { Inverse, SerializedCommand, StatePatch } from './types'
export interface HistoryEntry { commands: SerializedCommand[]; inverses: Inverse[]; redoPatch: StatePatch; contextKey: string; mergeKey: string | null; timestamp: number }
export interface HistoryStacks { undo: HistoryEntry[]; redo: HistoryEntry[] }
export function mergeStatePatches(first: StatePatch, last: StatePatch): StatePatch {
  const merged: StatePatch = { ...first, ...last }
  if (first.entities || last.entities) {
    merged.entities = { ...first.entities }
    for (const [name, delta] of Object.entries(last.entities ?? {})) {
      const key = name as keyof NonNullable<StatePatch['entities']>
      const previous = merged.entities[key]
      merged.entities[key] = { ...previous, ...delta, values: { ...previous?.values, ...delta.values } }
    }
  }
  return merged
}
export class CommandHistory {
  private readonly contexts = new Map<string, HistoryStacks>()
  stacks(contextKey: string): HistoryStacks {
    let stacks = this.contexts.get(contextKey)
    if (!stacks) { stacks = { undo: [], redo: [] }; this.contexts.set(contextKey, stacks) }
    return stacks
  }
  record(entry: HistoryEntry): void {
    const stacks = this.stacks(entry.contextKey)
    const previous = stacks.undo.at(-1)
    if (entry.mergeKey && previous?.mergeKey === entry.mergeKey) {
      previous.commands = entry.commands
      previous.redoPatch = mergeStatePatches(previous.redoPatch, entry.redoPatch)
      if (previous.inverses.every(inverse => inverse.kind === 'patch') && entry.inverses.every(inverse => inverse.kind === 'patch')) {
        let patch: StatePatch = {}
        for (const inverse of [...entry.inverses, ...previous.inverses].reverse()) {
          if (inverse.kind === 'patch') patch = mergeStatePatches(inverse.patch, patch)
        }
        previous.inverses = [{ kind: 'patch', patch }]
      } else previous.inverses = [...entry.inverses, ...previous.inverses]
    } else stacks.undo.push(entry)
    stacks.redo.length = 0
  }
  clear(): void { this.contexts.clear() }
  removeContext(contextKey: string): void { this.contexts.delete(contextKey) }
}
