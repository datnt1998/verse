import type { TimelineTrack } from '@editor/types/timeline';

/**
 * Next free short track name for a kind: V1, V2… for video tracks and A1, A2…
 * for audio tracks. Uses the first number not already taken by an existing
 * track name of that prefix, so deleting V2 then adding a track reuses V2.
 */
export function nextTrackName(tracks: readonly Pick<TimelineTrack, 'name'>[], kind: 'video' | 'audio'): string {
  const prefix = kind === 'audio' ? 'A' : 'V';
  const taken = new Set<number>();
  for (const track of tracks) {
    const match = /^([AV])(\d+)$/.exec(track.name.trim());
    if (match && match[1] === prefix) taken.add(Number(match[2]));
  }
  let index = 1;
  while (taken.has(index)) index += 1;
  return `${prefix}${index}`;
}
