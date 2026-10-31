export type MetadataPart = 'title' | 'description' | 'tags'

export function togglePart(parts: MetadataPart[], part: MetadataPart): MetadataPart[] {
  return parts.includes(part) ? parts.filter(value => value !== part) : [...parts, part]
}

export function metadataCounts(title: string, description: string, tags: string[], footer = '') {
  return { title: [...title].length, description: [...description].length + [...footer].length, tags: tags.length, tagCharacters: [...tags.join(',')].length }
}

export function coverageAfterRemoval(coverage: { fixedSec: number; channelSec: number; videoSec: number | null }, removedSeconds: number) {
  const covered = Math.max(0, coverage.fixedSec + coverage.channelSec - removedSeconds)
  return { covered, total: coverage.videoSec, short: coverage.videoSec !== null && covered < coverage.videoSec }
}
