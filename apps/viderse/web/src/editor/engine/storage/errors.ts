/**
 * Client-side storage error carrying the wire code the `/api/editor/*`
 * routes return. Codes mirror `src/server/editor/errors.ts` — keep in sync.
 */

export type StorageErrorCode =
  | 'not_found'
  | 'conflict'
  | 'invalid_request'
  | 'forbidden_origin'
  | 'payload_too_large'
  | 'insufficient_storage'
  | 'unsupported_format'
  | 'probe_failed'
  | 'locked'
  | 'internal_error'
  | 'network'

export class StorageClientError extends Error {
  readonly code: StorageErrorCode
  readonly status: number
  /** Server-provided extras, e.g. the current `revision` on a 409 conflict. */
  readonly extra?: Record<string, unknown>

  constructor(
    code: StorageErrorCode,
    message: string,
    status: number,
    extra?: Record<string, unknown>,
  ) {
    super(message)
    this.name = 'StorageClientError'
    this.code = code
    this.status = status
    this.extra = extra
  }
}

/** Conflict raised by a revisioned write (project save, rename, sidecar). */
export class StorageConflictError extends StorageClientError {
  /** Current revision on the server — the value that beat ours. */
  readonly serverRevision: string | null

  constructor(message: string, serverRevision: string | null, extra?: Record<string, unknown>) {
    super('conflict', message, 409, extra)
    this.name = 'StorageConflictError'
    this.serverRevision = serverRevision
  }
}

export function isStorageConflict(error: unknown): error is StorageConflictError {
  return error instanceof StorageConflictError
}
