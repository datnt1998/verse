export function frameTimecode(frame: number, fps: number): string {
  const rate = Math.max(1, Math.round(fps));
  const current = Math.max(0, Math.floor(frame));
  const seconds = Math.floor(current / rate);
  return [Math.floor(seconds / 3600), Math.floor(seconds / 60) % 60, seconds % 60, current % rate].map(value => String(value).padStart(2, '0')).join(':');
}
