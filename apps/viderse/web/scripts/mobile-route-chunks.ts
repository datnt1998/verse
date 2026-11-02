import type { Plugin, Rollup } from 'vite'

// Barrel re-exports overstate the raw entry graph. These controls are rendered
// by lazy UI, not the entry. Keep them in the shared lazy dependency chunk so
// desktop routes do not download the phone screen to use a single control.
const phoneControls = [
  '/platform/ui/src/mobile/rows.tsx',
  '/platform/ui/src/ConfirmDialog.tsx',
  '/platform/ui/src/controls.tsx',
  '/platform/ui/src/useAsyncAction.ts',
  '/platform/ui/src/Tabs.tsx',
  '/apps/viderse/web/src/components/DevelopEffect.tsx',
  '/apps/viderse/web/src/components/confirm.tsx',
]

// These global styles are already eager. A single CSS-only partition avoids
// separate blocking requests when shared controls also import kit.css.
const globalStyles = [
  '/apps/viderse/web/src/styles/viderse.css',
  '/apps/viderse/web/src/styles/brand.css',
  '/platform/ui/src/kit.css',
  '/platform/ui/src/Toast.css',
]

/** Coalesce the cold phone graph without coupling desktop routes to its screen.
 * A phone-family partition holds code reused by lazy phone details; cross-route
 * controls stay separate so desktop/editor routes do not inherit phone code. */
export function mobileRouteChunks(): Plugin {
  return {
    name: 'viderse-mobile-route-chunks',
    apply: 'build',
    outputOptions(options) {
      // Each output/watch build gets a fresh graph; no module ids survive rebuilds.
      let phone: Set<string> | undefined
      let shared: Set<string> | undefined
      let familyShared: Set<string> | undefined
      const manualChunks: Rollup.GetManualChunk = (id, graph) => {
        if (globalStyles.some((suffix) => id.endsWith(suffix))) return 'app-styles'
        if (!phone || !shared || !familyShared) {
          const ids = [...graph.getModuleIds()]
          const closure = (roots: string[]) => {
            const seen = new Set<string>()
            const pending = [...roots]
            while (pending.length) {
              const next = pending.pop()!
              if (seen.has(next)) continue
              seen.add(next)
              for (const imported of graph.getModuleInfo(next)?.importedIds ?? []) pending.push(imported)
            }
            return seen
          }
          const roots = ids.filter((module) => module.endsWith('/src/screens/workbench/mobile/MobileWorkbench.tsx'))
          const rootSet = new Set(roots)
          const family = closure(ids.filter((module) => module.includes('/src/screens/workbench/mobile/')))
          const eager = closure(ids.filter((module) => graph.getModuleInfo(module)?.isEntry))
          phone = closure(roots)
          for (const module of eager) {
            if (!phoneControls.some((suffix) => module.endsWith(suffix))) phone.delete(module)
          }
          const sharedRoots = [...phone].filter((module) => {
            const info = graph.getModuleInfo(module)
            return phoneControls.some((suffix) => module.endsWith(suffix))
              || info?.importers.some((importer) => !family.has(importer))
              || (!rootSet.has(module) && info?.dynamicImporters.some((importer) => !family.has(importer)))
          })
          // A shared module cannot statically depend on the private screen
          // chunk: move its phone-local dependencies into the shared partition.
          shared = closure(sharedRoots)
          for (const module of shared) if (!phone.has(module)) shared.delete(module)
          const familyRoots = [...phone].filter((module) => !shared!.has(module) && (
            graph.getModuleInfo(module)?.importers.some((importer) => !phone!.has(importer) && family.has(importer))
            || graph.getModuleInfo(module)?.dynamicImporters.some((importer) => !phone!.has(importer) && family.has(importer))
          ))
          familyShared = closure(familyRoots)
          for (const module of familyShared) if (!phone.has(module) || shared.has(module)) familyShared.delete(module)
        }
        if (shared.has(id)) return 'shared-controls'
        if (familyShared.has(id)) return 'phone-family'
        return phone.has(id) ? 'mobile-workbench' : undefined
      }
      return { ...options, onlyExplicitManualChunks: true, manualChunks }
    },
  }
}
