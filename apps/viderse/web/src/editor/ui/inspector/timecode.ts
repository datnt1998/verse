function frameRate(fps: number): number {
  return Math.max(1, Math.round(fps));
}

/** Formats a frame count as `mm:ss:ff`, or `hh:mm:ss:ff` once it reaches one hour. */
export function formatTimecode(frame: number, fps: number): string {
  const rate = frameRate(fps);
  const seconds = Math.floor(frame / rate);
  const parts = [Math.floor(seconds / 3600), Math.floor(seconds / 60) % 60, seconds % 60, frame % rate];
  const text = parts.map((part) => String(part).padStart(2, "0"));
  return parts[0] === 0 ? text.slice(1).join(":") : text.join(":");
}

/** Parses `ss:ff`, `mm:ss:ff` or `hh:mm:ss:ff`; returns null when malformed or the frame part is not below the frame rate. */
export function parseTimecode(text: string, fps: number): number | null {
  const rate = frameRate(fps);
  const parts = text.trim().split(/[:;]/);
  if (parts.length < 2 || parts.length > 4 || !parts.every((part) => /^\d+$/.test(part))) return null;
  const numbers = parts.map(Number);
  const frames = numbers[numbers.length - 1]!;
  const seconds = numbers.slice(0, -1).reduce((total, part) => total * 60 + part, 0);
  return frames < rate ? seconds * rate + frames : null;
}
