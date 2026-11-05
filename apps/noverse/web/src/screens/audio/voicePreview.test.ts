import { describe, expect, it } from 'vitest'
import { ttsPreviewURL, voicePreviewSource } from './voicePreview'

const voice = (over: Partial<{ builtIn: boolean; sampleUrl?: string }>) =>
  ({ builtIn: true, ...over })

describe('nguồn nghe thử giọng', () => {
  it('giọng tải lên phát bằng tệp mẫu, kể cả khi máy đọc không nhận nghe thử', () => {
    expect(voicePreviewSource(voice({ builtIn: false, sampleUrl: '/api/projects/p/voices/v1/sample' }), false)).toBe('sample')
    expect(voicePreviewSource(voice({ builtIn: false, sampleUrl: '/api/projects/p/voices/v1/sample' }), true)).toBe('sample')
  })

  it('preset ở bản cá nhân có mẫu máy chủ tự tạo nên vẫn phát bằng URL', () => {
    expect(voicePreviewSource(voice({ sampleUrl: '/api/projects/p/voices/preset/sample' }), false)).toBe('sample')
  })

  it('preset remote không có mẫu chỉ nghe được khi máy đọc nhận nghe thử', () => {
    expect(voicePreviewSource(voice({}), false)).toBe('none')
    expect(voicePreviewSource(voice({}), true)).toBe('tts')
  })

  it('không có giọng thì không có nguồn nào', () => {
    expect(voicePreviewSource(undefined, true)).toBe('none')
    expect(voicePreviewSource(null, true)).toBe('none')
  })

  it('URL mẫu của preset là đường GET phát trực tiếp, id được mã hoá', () => {
    expect(ttsPreviewURL('vieneu-nu-bac')).toBe('/api/tts/voices/vieneu-nu-bac/sample')
    expect(ttsPreviewURL('a/b')).toBe('/api/tts/voices/a%2Fb/sample')
  })
})
