/**
 * Globals left over from the upstream Vite HMR guards. Next/Turbopack never
 * sets `__vite_hmr__`, so the guarded dispose calls are no-ops; the
 * declaration keeps the call sites typed instead of casting to `any`.
 */
declare global {
  var __vite_hmr__: { dispose(cb: () => void): void } | undefined
}

export {}
