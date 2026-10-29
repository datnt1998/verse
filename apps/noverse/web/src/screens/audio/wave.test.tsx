// Bộ màu sóng âm (17-kit.md › “Vòng tiến độ và sóng âm”, 22-brief Câu 18):
// nghe = đã phát tím / chưa phát #2A2A2E (điện thoại trắng 16%);
// tạo giọng = đã tạo #C9C9D1 / 3 cột cuối tím / chưa tạo #2A2A2E.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { generationTone, NEWEST_BARS } from './logic'
import { Waveform } from './ui'
import { MobileWave } from './mobile/Wave'

const TOKENS = readFileSync(fileURLToPath(new URL('../../../packages/ui/src/tokens.stylex.ts', import.meta.url)), 'utf8')
const tokenValue = (name: string) => TOKENS.match(new RegExp(`'${name}':\\s*'([^']+)'`))?.[1]

describe('design chốt bảng màu sóng (Câu 18)', () => {
  it('nghe: đã phát #C9A6F5, chưa phát #2A2A2E, điện thoại rgba(255,255,255,.16)', () => {
    expect(tokenValue('--wave-played')).toBe('#C9A6F5')
    expect(tokenValue('--wave-unplayed')).toBe('#2A2A2E')
    expect(tokenValue('--wave-unplayed-phone')).toBe('rgba(255,255,255,.16)')
  })
  it('tạo giọng: đã tạo #C9C9D1, 3 cột mới nhất #C9A6F5, chưa tạo #2A2A2E', () => {
    expect(tokenValue('--wave-generated')).toBe('#C9C9D1')
    expect(tokenValue('--wave-newest')).toBe('#C9A6F5')
    expect(NEWEST_BARS).toBe(3)
  })
})

describe('sóng nghe dùng bộ màu nghe', () => {
  it('vạch trước kim tím, vạch sau kim #2A2A2E', () => {
    const html = renderToStaticMarkup(<Waveform peaks={[1]} bars={2} height={20} played={0.5} />)
    expect(html).toContain('background:var(--wave-played)')
    expect(html).toContain('background:var(--wave-unplayed)')
    expect(html).not.toContain('var(--color-accent)')
    expect(html).not.toContain('var(--color-surface-7)')
  })
  it('sóng nghe trên điện thoại: chưa phát dùng màu trắng 16%, không dùng viền --line-5', () => {
    const idle = renderToStaticMarkup(<MobileWave bars={[1, 1]} fraction={0} time={0} duration={10} disabled={false} onSeek={() => {}} />)
    expect(idle.split('background:var(--wave-unplayed-phone)').length - 1).toBe(2)
    expect(idle).not.toContain('var(--line-5)')
    const played = renderToStaticMarkup(<MobileWave bars={[1, 1]} fraction={1} time={10} duration={10} disabled={false} onSeek={() => {}} />)
    expect(played.split('background:var(--wave-played)').length - 1).toBe(2)
  })
})

describe('sóng tạo giọng: đúng 3 cột mới nhất', () => {
  it('vạch thứ 8–10 (filled 10) là mới nhất, 1–7 là đã tạo, 11 trở đi chưa tạo', () => {
    const tones = Array.from({ length: 12 }, (_, index) => generationTone(index, 10))
    expect(tones).toEqual(['done', 'done', 'done', 'done', 'done', 'done', 'done', 'newest', 'newest', 'newest', 'pending', 'pending'])
    expect(tones.filter((tone) => tone === 'newest')).toHaveLength(3)
  })
  it('chưa đủ 3 vạch thì mọi vạch đã tạo đều là mới nhất; chưa tạo gì thì tất cả chờ', () => {
    expect([generationTone(0, 2), generationTone(1, 2), generationTone(2, 2)]).toEqual(['newest', 'newest', 'pending'])
    expect(Array.from({ length: 4 }, (_, index) => generationTone(index, 0))).toEqual(['pending', 'pending', 'pending', 'pending'])
  })
  it('sóng đầy 96 cột vẫn chỉ 3 cột mới nhất', () => {
    const tones = Array.from({ length: 96 }, (_, index) => generationTone(index, 96))
    expect(tones.filter((tone) => tone === 'newest')).toHaveLength(3)
    expect(tones.slice(-3)).toEqual(['newest', 'newest', 'newest'])
  })
})
