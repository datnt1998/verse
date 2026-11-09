import type { LayoutStorage } from 'react-resizable-panels'

// Layout is an optional device preference: blocked/quota-limited storage must not
// prevent opening or resizing a project. Reject corrupt preferences at the boundary.
export function createLayoutStorage(panelIds: readonly string[], storage: () => LayoutStorage): LayoutStorage {
  return {
    getItem(key) {
      try {
        const raw = storage().getItem(key)
        if (!raw) return null
        const value: unknown = JSON.parse(raw)
        if (!value || typeof value !== 'object' || Array.isArray(value)) return null
        const entries = Object.entries(value)
        if (entries.length !== panelIds.length || entries.some(([id, size]) => !panelIds.includes(id) || typeof size !== 'number' || !Number.isFinite(size) || size <= 0 || size >= 100)) return null
        if (Math.abs(entries.reduce((sum, [, size]) => sum + (size as number), 0) - 100) > 0.1) return null
        return raw
      } catch { return null /* Use artboard defaults when preferences cannot be read. */ }
    },
    setItem(key, value) {
      try { storage().setItem(key, value) }
      catch { /* Resizing remains available when this device cannot save preferences. */ }
    },
  }
}
