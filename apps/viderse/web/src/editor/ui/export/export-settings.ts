import type { ExportSettings, ExtendedExportSettings } from '@editor/types/export'
import type { ExportPreflightCheck } from '@editor/features/export/utils/export-preflight'

export const EXPORT_PRESETS = [
  { id: 'youtube', label: 'YouTube ngang · Full HD', width: 1920, height: 1080, fps: 30 },
  { id: '1440p', label: 'YouTube ngang · Siêu nét (2K)', width: 2560, height: 1440, fps: 30 },
  { id: '4k', label: 'YouTube ngang · Cực nét (4K)', width: 3840, height: 2160, fps: 30 },
  { id: 'shorts', label: 'Video dọc · Shorts, TikTok', width: 1080, height: 1920, fps: 30 },
] as const
export type ExportPreset = typeof EXPORT_PRESETS[number]['id']

export const EXPORT_QUALITY_OPTIONS = [
  { value: 'low', label: 'Tiết kiệm dung lượng' },
  { value: 'medium', label: 'Tiêu chuẩn' },
  { value: 'high', label: 'Chất lượng cao' },
  { value: 'ultra', label: 'Chất lượng tối đa' },
] as const

/** Engineering summary of what is about to be rendered; shown only behind "Chi tiết". */
export function exportTechnicalSummary(settings: Pick<ExtendedExportSettings, 'resolution' | 'fps'>): string {
  return `${settings.resolution.width}×${settings.resolution.height} · ${settings.fps} khung hình/giây · MP4 (H.264 + AAC)`
}

export function exportSettings(preset: ExportPreset, quality: ExportSettings['quality'], whole: boolean): ExtendedExportSettings {
  const chosen = EXPORT_PRESETS.find(entry => entry.id === preset) ?? EXPORT_PRESETS[0]
  return { mode: 'video', codec: 'h264', videoContainer: 'mp4', subtitleMode: 'burn', smartCopy: false, quality, fps: chosen.fps, resolution: { width: chosen.width, height: chosen.height }, renderWholeProject: whole }
}

/**
 * Background export of a clone video. The render is only an intermediate: the server re-encodes it with x264
 * before handing it over (`export-compress.ts`), which sets the delivered size. Re-encoding a 1.2 Mbps render
 * smeared the per-frame waveform and karaoke captions; from this rate the same delivered size keeps them sharp
 * (caption-region SSIM 0.935 → 0.963, 60 s 1080p30 test clip with an overlay).
 */
export const CLONE_EXPORT_VIDEO_BITRATE = 6_000_000
export const CLONE_EXPORT_AUDIO_BITRATE = 128_000

export function cloneExportSettings(): ExtendedExportSettings {
  return { ...exportSettings('youtube', 'medium', true), rateControl: 'variable', videoBitrate: CLONE_EXPORT_VIDEO_BITRATE, audioBitrate: CLONE_EXPORT_AUDIO_BITRATE }
}

export function preflightMessage(check: ExportPreflightCheck): string {
  const params = check.detailParams ?? {}
  switch (check.id) {
    case 'empty-range': return 'Chưa có gì để xuất. Hãy thêm clip vào video hoặc chọn lại đoạn cần xuất.'
    case 'export-range-ready': return `Đoạn xuất dài ${params.seconds} giây.`
    case 'missing-media-blocks-export': return `Thiếu ${params.count} file ảnh/video/âm thanh. Thay file trong thư viện rồi xuất lại.`
    case 'waveform-envelope-blocks-export': return `${params.count} hiệu ứng sóng giọng cần được tạo lại. Chọn nó trên timeline và bấm tạo lại trong bảng Thuộc tính.`
    case 'video-codec-unavailable': return 'Trình duyệt này chưa xuất được video. Hãy mở bằng Chrome bản mới nhất rồi thử lại.'
    case 'video-codec-fallback': return 'Trình duyệt dùng cách xuất dự phòng nên video vẫn xuất được, có thể hơi lâu hơn.'
    case 'video-codec-supported': return 'Trình duyệt xuất được video.'
    case 'audio-codec-unavailable': return 'Trình duyệt này chưa xuất được âm thanh. Hãy mở bằng Chrome bản mới nhất rồi thử lại.'
    case 'audio-codec-supported': return 'Trình duyệt xuất được âm thanh.'
    case 'worker-unavailable-fallback': return 'Máy sẽ dựng video theo cách chậm hơn, màn hình có thể đứng hình một lúc.'
    case 'worker-animated-image-fallback': return 'Ảnh động khiến việc dựng video chậm hơn, màn hình có thể đứng hình một lúc.'
    case 'worker-export-ready': return 'Máy sẵn sàng dựng video nền, bạn vẫn dùng editor được.'
    case 'large-file-risk': return `File xuất khoảng ${params.size}. Hãy chắc rằng máy còn đủ dung lượng trống.`
    case 'long-export-risk': return `Video dài ${params.minutes} phút nên xuất sẽ lâu. Giữ tab này mở đến khi xong.`
    default: return 'Kiểm tra trước khi xuất cần được xem lại.'
  }
}
