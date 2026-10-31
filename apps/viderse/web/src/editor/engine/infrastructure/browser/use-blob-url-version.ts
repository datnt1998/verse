import { useSyncExternalStore } from 'react'
import { blobUrlManager } from './blob-url-manager'

/**
 * React hook that re-renders when blob URLs are acquired or released.
 * Use as a dependency in useMemo to react to URL availability changes.
 * Lives apart from the manager so server code (headless command engine) can
 * import the manager without pulling client-only React APIs.
 */
export function useBlobUrlVersion(): number {
  return useSyncExternalStore(blobUrlManager.subscribe, blobUrlManager.getSnapshot)
}
