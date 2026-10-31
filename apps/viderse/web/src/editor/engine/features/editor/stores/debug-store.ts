import { create } from 'zustand'

interface DebugState {
  /** Show debug overlay on video clips */
  showVideoDebugOverlay: boolean
  setShowVideoDebugOverlay: (show: boolean) => void
  toggleVideoDebugOverlay: () => void
  /** Show preview performance diagnostics panel */
  showPreviewPerfPanel: boolean
  setShowPreviewPerfPanel: (show: boolean) => void
  togglePreviewPerfPanel: () => void
  /** Debug panel open state */
  debugPanelOpen: boolean
  setDebugPanelOpen: (open: boolean) => void
  toggleDebugPanel: () => void
}

/**
 * Debug store for development-only settings
 * In production, all values are false/no-op for tree-shaking
 *
 * Vite supplies DEV in browser pages/workers; Node command sessions fall back
 * to NODE_ENV without installing a process shim in the browser.
 */
const isDev = import.meta.env?.DEV ??
  (typeof process !== 'undefined' && process.env.NODE_ENV !== 'production')

export const useDebugStore = isDev
  ? create<DebugState>((set) => ({
      showVideoDebugOverlay: false,
      setShowVideoDebugOverlay: (show) => set({ showVideoDebugOverlay: show }),
      toggleVideoDebugOverlay: () =>
        set((s) => ({ showVideoDebugOverlay: !s.showVideoDebugOverlay })),
      showPreviewPerfPanel: false,
      setShowPreviewPerfPanel: (show) => set({ showPreviewPerfPanel: show }),
      togglePreviewPerfPanel: () => set((s) => ({ showPreviewPerfPanel: !s.showPreviewPerfPanel })),
      debugPanelOpen: false,
      setDebugPanelOpen: (open) => set({ debugPanelOpen: open }),
      toggleDebugPanel: () => set((s) => ({ debugPanelOpen: !s.debugPanelOpen })),
    }))
  : create<DebugState>(() => ({
      showVideoDebugOverlay: false,
      setShowVideoDebugOverlay: () => {},
      toggleVideoDebugOverlay: () => {},
      showPreviewPerfPanel: false,
      setShowPreviewPerfPanel: () => {},
      togglePreviewPerfPanel: () => {},
      debugPanelOpen: false,
      setDebugPanelOpen: () => {},
      toggleDebugPanel: () => {},
    }))
