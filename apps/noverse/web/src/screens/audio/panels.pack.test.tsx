// D11 — Transcript › Phân vai khi gói chọn một giọng kể: thẻ khoá thay hai thẻ Phân vai | Một giọng.
import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import type { AudioCast, AudioVoice } from '../../api/types'
import { CastPanel } from './panels'

const cast: AudioCast = {
  mode: 'cast',
  single: { voiceId: 'nu-tre-am', dialogueShift: true, keepTones: true, dialoguePauseMs: 0 },
  speakers: [{ id: 'dan', name: 'Dẫn truyện', role: 'Ngôi thứ nhất', voiceId: 'giong-tri-ha', sameAs: 'th' }],
}
const voices: AudioVoice[] = [
  { id: 'giong-tri-ha', name: 'Mẫu giọng Tri Hạ', durationSec: 18, gender: 'nu', builtIn: false },
  { id: 'nu-tre-am', name: 'Nữ trẻ, ấm', durationSec: 0, gender: 'nu', builtIn: true },
]
const render = (pack: { ten: string; gender: 'nu' | 'nam' | '' } | null) => renderToStaticMarkup(
  <CastPanel cast={cast} voices={voices} lineCounts={{}} writable pack={pack} onCast={() => {}} onAddVoice={() => {}} />,
)

it('gói một giọng kể: thẻ khoá “Một giọng kể · theo gói” thay hai thẻ cách giao giọng', () => {
  const html = render({ ten: 'Nữ tần báo thù', gender: 'nu' })
  expect(html).toContain('Một giọng kể')
  expect(html).toContain('theo gói')
  expect(html).toContain('Gói Nữ tần báo thù dùng một giọng nữ trẻ cho cả truyện, kể cả lời thoại.')
  expect(html).toContain('Đổi gói trong Tuỳ chỉnh ›')
  expect(html).not.toContain('Mỗi nhân vật một giọng')
  expect(html).not.toContain('Một người đọc tất cả')
  expect(html).not.toContain('Phân vai · cả truyện')
  expect(html).toContain('Giọng đọc cho cả truyện')
})

it('giọng gói đề xuất được ghi rõ, mẫu giọng của bạn không hiện tên tệp', () => {
  const html = render({ ten: 'Nữ tần báo thù', gender: 'nu' })
  expect(html).toContain('Giọng có sẵn · gói đề xuất')
  expect(html).toContain('Bạn tải lên · 18 giây · nữ')
  expect(html).not.toContain('.wav')
})

it('không có gói: vẫn hai thẻ cách giao giọng, không có nhãn gói', () => {
  const html = render(null)
  expect(html).toContain('Mỗi nhân vật một giọng')
  expect(html).toContain('Một người đọc tất cả')
  expect(html).not.toContain('theo gói')
  expect(html).not.toContain('gói đề xuất')
})
