/** "0:07" or "1:02:03": the player's time label, whole seconds only. */
export function clockLabel(frame: number, fps: number): string {
  const rate = Math.max(1, Math.round(fps));
  const total = Math.floor(Math.max(0, frame) / rate);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor(total / 60) % 60;
  const seconds = String(total % 60).padStart(2, "0");
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${seconds}` : `${minutes}:${seconds}`;
}

/** Spoken form for the scrub bar: "7 giây trên 2 phút 5 giây". */
export function spokenClock(frame: number, fps: number): string {
  const rate = Math.max(1, Math.round(fps));
  const total = Math.floor(Math.max(0, frame) / rate);
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  return minutes > 0 ? `${minutes} phút ${seconds} giây` : `${seconds} giây`;
}
