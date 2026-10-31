import { create } from 'zustand'

/** A revisioned project write that lost against a newer server revision. */
export interface StorageConflict {
  projectId: string
  /** Revision our snapshot was loaded from (the one the rejected write sent as If-Match). */
  baseRevision: string
  /** Current server revision reported by the 409, or null if the server did not say. */
  serverRevision: string | null
  /** ms since epoch when the conflict was detected. */
  timestamp: number
}

export interface StorageConflictEvents {
  conflict: StorageConflict
}

type Listener<K extends keyof StorageConflictEvents> = (payload: StorageConflictEvents[K]) => void

const listeners: { [K in keyof StorageConflictEvents]: Set<Listener<K>> } = {
  conflict: new Set(),
}

/** Typed subscription for storage conflict events; `on` returns an unsubscribe function. */
export const storageConflictEvents = {
  on<K extends keyof StorageConflictEvents>(event: K, listener: Listener<K>): () => void {
    listeners[event].add(listener)
    return () => {
      listeners[event].delete(listener)
    }
  },
}

interface StorageConflictState {
  conflicts: Record<string, StorageConflict>
}

interface StorageConflictActions {
  reportConflict: (conflict: StorageConflict) => void
  clearConflict: (projectId: string) => void
}

/**
 * Conflicts block autosave of the affected project until the user reloads
 * (`loadProject`) or explicitly keeps their version (`acceptServerRevision`).
 */
export const useStorageConflictStore = create<StorageConflictState & StorageConflictActions>()(
  (set) => ({
    conflicts: {},
    reportConflict: (conflict) => {
      set((state) => ({ conflicts: { ...state.conflicts, [conflict.projectId]: conflict } }))
      for (const listener of [...listeners.conflict]) listener(conflict)
    },
    clearConflict: (projectId) =>
      set((state) => {
        if (!(projectId in state.conflicts)) return state
        const conflicts = { ...state.conflicts }
        delete conflicts[projectId]
        return { conflicts }
      }),
  }),
)

export function getStorageConflict(projectId: string): StorageConflict | undefined {
  return useStorageConflictStore.getState().conflicts[projectId]
}
