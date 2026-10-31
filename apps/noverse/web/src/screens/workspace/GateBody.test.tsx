// @vitest-environment happy-dom
// Câu 13 — Chất lượng là việc ngồi máy tính: liên kết ở cửa duyệt chỉ hiện ở bản máy tính.
// `LiveStream` là nơi quyết định (`quality={!mobile}`); `GateBody` chỉ vẽ khi được phép.
import { renderToStaticMarkup } from 'react-dom/server'
import { beforeAll, describe, expect, it } from 'vitest'
import { review3Blocking } from '../../../mock/data'
import { GateBody, LiveStream } from './LiveStream'

const stream = (mobile: boolean) => renderToStaticMarkup(
  <LiveStream project="p" story="s" streamText="" isRunning={false} snapshot={null}
    gate review={review3Blocking} gateChapter={3} mobile={mobile} />,
)

describe('cửa duyệt › lối vào Chất lượng truyện (Câu 13)', () => {
  beforeAll(() => { location.hash = '#/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/work' })
  it('máy tính có liên kết “Chất lượng cả truyện”, điện thoại thì không', () => {
    expect(stream(false)).toContain('Chất lượng cả truyện')
    expect(stream(true)).not.toContain('Chất lượng cả truyện')
  })
  it('GateBody chỉ vẽ liên kết khi được truyền `quality`', () => {
    const withLink = renderToStaticMarkup(<GateBody review={review3Blocking} chapter={3} loading={false} quality />)
    const without = renderToStaticMarkup(<GateBody review={review3Blocking} chapter={3} loading={false} quality={false} />)
    expect(withLink).toContain('Chất lượng cả truyện')
    expect(without).not.toContain('Chất lượng cả truyện')
  })
})
