import type { AudioVoice } from '../../api/types'

/**
 * Một giọng nghe được bằng cách nào.
 *
 * - `sample`: có tệp mẫu trong dự án (giọng tải lên, và preset ở bản cá nhân vì
 *   máy chủ tự tạo mẫu) — phát thẳng bằng URL.
 * - `tts`: preset của máy đọc ở bản máy chủ. Preset remote không có blob mẫu
 *   trong dự án, nên chỉ nghe được khi máy đọc báo nhận nghe thử theo yêu cầu
 *   (`capabilities.preview`).
 * - `none`: không có tuyến nào. Nút phải tắt — không giả vờ có mẫu.
 */
export type VoicePreviewSource = 'sample' | 'tts' | 'none'

export function voicePreviewSource(
  voice: Pick<AudioVoice, 'builtIn' | 'sampleUrl'> | null | undefined,
  ttsPreview: boolean,
): VoicePreviewSource {
  if (!voice) return 'none'
  if (voice.sampleUrl) return 'sample'
  return voice.builtIn && ttsPreview ? 'tts' : 'none'
}

/**
 * URL trả thẳng `audio/wav` của một preset trên máy đọc. Gắn `src` rồi gọi
 * `play()` ngay trong cử chỉ chạm, không chờ fetch blob như POST.
 */
export function ttsPreviewURL(voiceId: string): string {
  return `/api/tts/voices/${encodeURIComponent(voiceId)}/sample`
}
