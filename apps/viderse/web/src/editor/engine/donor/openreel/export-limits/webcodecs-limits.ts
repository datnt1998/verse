export interface WebCodecsVideoSettings { codec: string; width: number; height: number; frameRate: number }
export interface WebCodecsExportWarning { code: 'resolution' | 'frame-rate'; message: string; recommended: { width: number; height: number; frameRate: number } }

/** OpenReel safety policy, warning-only in YT Factory; portrait orientation is retained. */
export function resolveWebCodecsExportLimits<T extends WebCodecsVideoSettings>(settings: T, durationSeconds: number): { settings: T; wasAdjusted: boolean; resolutionAdjusted: boolean; frameRateAdjusted: boolean } {
  const long = durationSeconds > 120
  const hd = long || ['vp9', 'av1', 'h265', 'hevc'].includes(settings.codec)
  const scale = Math.min((hd ? 1920 : 3840) / Math.max(settings.width, settings.height), (hd ? 1080 : 2160) / Math.min(settings.width, settings.height), 1)
  const width = Math.max(2, Math.round(settings.width * scale / 2) * 2)
  const height = Math.max(2, Math.round(settings.height * scale / 2) * 2)
  const frameRate = long ? Math.min(settings.frameRate, 30) : settings.frameRate
  const resolutionAdjusted = width !== settings.width || height !== settings.height
  const frameRateAdjusted = frameRate !== settings.frameRate
  return { settings: { ...settings, width, height, frameRate }, wasAdjusted: resolutionAdjusted || frameRateAdjusted, resolutionAdjusted, frameRateAdjusted }
}

export function getWebCodecsExportWarnings(settings: WebCodecsVideoSettings, durationSeconds: number): WebCodecsExportWarning[] {
  const resolved = resolveWebCodecsExportLimits(settings, durationSeconds)
  const recommended = { width: resolved.settings.width, height: resolved.settings.height, frameRate: resolved.settings.frameRate }
  const warnings: WebCodecsExportWarning[] = []
  if (resolved.resolutionAdjusted) warnings.push({ code: 'resolution', message: `Độ phân giải này có thể dùng nhiều bộ nhớ khi xuất trên trình duyệt. Khuyến nghị ${recommended.width}×${recommended.height}.`, recommended })
  if (resolved.frameRateAdjusted) warnings.push({ code: 'frame-rate', message: 'Video dài trên 2 phút: khuyến nghị 30 khung hình/giây để giảm tải bộ mã hóa.', recommended })
  return warnings
}

/** Pure interpretation of probes; callers supply browser/container codec results. */
export function checkBrowserExportCapability(settings: WebCodecsVideoSettings, probes: { supportedContainerCodecs: readonly string[]; encodableVideoCodecs: readonly string[]; audioSupported: boolean; includeAudio?: boolean; colorDepth?: number; pixelFormat?: string }): { supported: boolean; reason: string | null } {
  if (settings.codec === 'prores' || (probes.colorDepth ?? 8) > 8 || (probes.pixelFormat && probes.pixelFormat !== 'yuv420')) return { supported: false, reason: 'Trình duyệt không hỗ trợ ProRes, alpha hoặc độ sâu màu này. Hãy chọn H.264.' }
  if (!probes.supportedContainerCodecs.includes(settings.codec)) return { supported: false, reason: 'Codec không phù hợp với định dạng tệp đã chọn.' }
  if (!probes.encodableVideoCodecs.includes(settings.codec)) return { supported: false, reason: 'Trình duyệt không mã hóa được codec hoặc độ phân giải đã chọn.' }
  if (probes.includeAudio !== false && !probes.audioSupported) return { supported: false, reason: 'Trình duyệt không mã hóa được âm thanh cho định dạng này.' }
  return { supported: true, reason: null }
}
