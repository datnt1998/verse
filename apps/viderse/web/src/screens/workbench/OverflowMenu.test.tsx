import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { CloneJobView } from '../../api/workbench'
import { OverflowMenu } from './OverflowMenu'

const render = (stageGroup: string) => renderToStaticMarkup(
  <OverflowMenu
    job={{ id: 'j1', channelId: 'c1', stageGroup, source: { title: 'Đêm ở trạm', url: '' }, steps: [] } as unknown as CloneJobView}
    onClose={() => {}}
    onRefresh={() => {}}
    onError={() => {}}
  />,
)

describe('menu ⋯ của Bàn dựng', () => {
  it('khoá Xoá video khi video đang chạy và ghi lý do ngay trên mục', () => {
    const html = render('working')
    expect(html).toContain('Tạm dừng trước rồi xoá')
    expect(html).toMatch(/aria-disabled="true"[^>]*>Xoá video/)
  })
  it('cho xoá khi video không chạy', () => {
    const html = render('ready')
    expect(html).not.toContain('Tạm dừng trước rồi xoá')
    expect(html).not.toContain('aria-disabled="true" title="Tạm dừng')
    expect(html).toContain('Xoá MP4, ảnh và project của video này')
  })
})
