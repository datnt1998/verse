type MediaLibraryServiceModule = typeof import('../services/media-library-service')

/**
 * Lazily loads the media library service. A dynamic import keeps the service (and its
 * storage/codec dependencies) out of the store's module graph without relying on a
 * side-effect registration that hot reloads can reset.
 */
export function loadMediaLibraryService(): Promise<MediaLibraryServiceModule> {
  return import('../services/media-library-service')
}
