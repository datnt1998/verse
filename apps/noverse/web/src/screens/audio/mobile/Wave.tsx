/** Sóng âm ở trình phát MAudio — bộ màu “nghe” bản điện thoại (17-kit.md › Sóng âm). */
export function MobileWave({ bars, fraction, time, duration, disabled, onSeek }: {
  bars: number[]; fraction: number; time: number; duration: number; disabled: boolean
  onSeek: (seconds: number) => void
}) {
  return (
    <div className="ma-wave">
      <div aria-hidden="true">{bars.map((height, index) => (
        <span key={index} style={{
          height: `${Math.max(10, height * 100)}%`,
          background: index / bars.length < fraction ? 'var(--wave-played)' : 'var(--wave-unplayed-phone)',
        }} />
      ))}</div>
      <input type="range" aria-label="Tua chương" min={0} max={duration || 1} step={.1} disabled={disabled}
        value={Math.min(time, duration || 1)} onChange={(event) => onSeek(Number(event.target.value))} />
    </div>
  )
}
