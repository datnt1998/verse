/**
 * Cache root owner.
 *
 * Durable state lives on the server (`EditorStorage`); everything left in
 * workspace-fs is a disposable OPFS cache. The "root" every module passes to
 * fs-primitives is therefore the opaque `CACHE_ROOT` token — it carries no
 * directory handle, no permission state, and nothing to set or revoke.
 */

import { CACHE_ROOT, type CacheRoot } from '@editor/storage/opfs-cache'

/** The cache root token every storage operation passes to fs-primitives. */
export function requireWorkspaceRoot(): CacheRoot {
  return CACHE_ROOT
}

/** Same token as {@link requireWorkspaceRoot}. */
export function getWorkspaceRoot(): CacheRoot {
  return CACHE_ROOT
}
