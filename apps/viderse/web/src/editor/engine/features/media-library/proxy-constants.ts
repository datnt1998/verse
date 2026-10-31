export const PROXY_FILE_NAME = 'proxy.mp4'
export const PROXY_META_FILE_NAME = 'meta.json'
export const PROXY_SCHEMA_VERSION = 4

/** Path of a file inside a proxy's OPFS directory (segments from `proxyCacheDirSegments`). */
export function proxyOpfsFilePath(dirSegments: readonly string[]): string {
  return [...dirSegments, PROXY_FILE_NAME].join('/')
}
