/**
 * The shared overlay library: every media file tagged as an overlay, across the
 * whole workspace. Media is stored once per import and linked into projects, so
 * one upload serves every template and the editor. Earlier re-uploads of the
 * same bytes left duplicate records; the library collapses each content hash to
 * its oldest record, so an effect shows once however many copies exist.
 */
export type OverlayLibraryRecord = { id: string; contentHash: string; createdAt: number; overlay?: boolean }

/** One record per overlay content hash (the oldest), newest effect first. */
export function selectOverlayLibrary<T extends OverlayLibraryRecord>(records: readonly T[]): T[] {
  const byHash = new Map<string, T>()
  for (const record of records) {
    if (record.overlay !== true) continue
    const kept = byHash.get(record.contentHash)
    if (!kept || record.createdAt < kept.createdAt) byHash.set(record.contentHash, record)
  }
  return [...byHash.values()].sort((a, b) => b.createdAt - a.createdAt)
}
