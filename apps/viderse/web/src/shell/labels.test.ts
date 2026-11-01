// Tiêu đề tab trình duyệt theo màn: nhãn màn tiếng Việt + "· Viderse"; tên kênh
// và template lấy từ dữ liệu đã có, mã kênh/template không bao giờ vào tiêu đề.
import { describe, expect, it } from 'vitest'
import { documentTitle } from './labels'
import type { Route } from '../router'

describe('tiêu đề tab theo màn', () => {
  it.each<[string, Route, string]>([
    ['Bàn dựng', { name: 'workbench' }, 'Bàn dựng · Viderse'],
    ['Duyệt', { name: 'review' }, 'Duyệt · Viderse'],
    ['Hàng chờ', { name: 'queue' }, 'Hàng chờ · Viderse'],
    ['Tạo kênh', { name: 'channel-new' }, 'Tạo kênh · Viderse'],
    ['Khám phá', { name: 'explore' }, 'Khám phá · Viderse'],
    ['Video mới', { name: 'new' }, 'Video mới · Viderse'],
    ['Thư viện', { name: 'library' }, 'Thư viện · Viderse'],
    ['Cài đặt', { name: 'settings' }, 'Cài đặt · Viderse'],
    ['Tài khoản', { name: 'account' }, 'Tài khoản · Viderse'],
    ['Chuẩn bị máy', { name: 'setup' }, 'Chuẩn bị máy · Viderse'],
    ['màn ngoài khung', { name: 'render', requestId: 'r1' }, 'Viderse · Xưởng video'],
    ['dev kit', { name: 'devkit' }, 'Viderse · Xưởng video'],
  ])('%s', (_label, route, want) => {
    expect(documentTitle(route)).toBe(want)
  })

  it('kênh có tên → "Kênh <tên>"', () => {
    expect(documentTitle({ name: 'channel', channelId: 'twitch:nightdrivefm' }, { channel: 'Night Drive' }))
      .toBe('Kênh Night Drive · Viderse')
  })

  it('kênh chưa tải xong hoặc chưa đặt tên → chỉ nhãn màn, không lấy mã kênh thay tên', () => {
    const route: Route = { name: 'channel', channelId: 'twitch:nightdrivefm' }
    expect(documentTitle(route, { channel: null })).toBe('Kênh · Viderse')
    expect(documentTitle(route, { channel: '   ' })).toBe('Kênh · Viderse')
  })

  it('template có tên → "Template <tên>"', () => {
    expect(documentTitle({ name: 'template', templateId: 'tpl-9f3' }, { template: 'Bản tin' }))
      .toBe('Template Bản tin · Viderse')
  })

  it('template chưa có tên → chỉ nhãn màn, không lấy mã template thay tên', () => {
    expect(documentTitle({ name: 'template', templateId: 'tpl-9f3' })).toBe('Template · Viderse')
  })

  it('editor → "<tên project> · Bàn dựng · Viderse", thiếu tên thì bỏ phần đầu', () => {
    expect(documentTitle({ name: 'editor', projectId: 'p-1' }, { project: 'Bản tin 24/10' }))
      .toBe('Bản tin 24/10 · Bàn dựng · Viderse')
    expect(documentTitle({ name: 'editor', projectId: 'p-1' })).toBe('Bàn dựng · Viderse')
  })

  it('không route nào lộ mã kênh/template/project', () => {
    const ids = ['twitch:nightdrivefm', 'tpl-9f3', 'p-1']
    const routes: Route[] = [
      { name: 'channel', channelId: ids[0], tab: 'videos' },
      { name: 'template', templateId: ids[1], section: 'images' },
      { name: 'editor', projectId: ids[2] },
      { name: 'workbench', channel: ids[0], video: ids[2] },
      { name: 'review', channel: ids[0], video: ids[2] },
      { name: 'library', kenh: ids[0], tab: 'templates' },
    ]
    for (const route of routes) {
      const title = documentTitle(route)
      for (const id of ids) expect(title).not.toContain(id)
    }
  })
})
