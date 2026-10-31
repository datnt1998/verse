import type { TimelineTrack } from '@editor/types/timeline'
import { executeCommands } from '@editor/commands'
export function setTracks(tracks: TimelineTrack[]): void {
  const result = executeCommands([{ type: 'track.set', params: { tracks } }])
  if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '))
}
