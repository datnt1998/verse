// Tiêu đề tab trình duyệt theo màn: nhãn màn tiếng Việt + "· Viderse"; tên kênh
// và template lấy từ dữ liệu đã có, mã kênh/template không bao giờ vào tiêu đề.
import { describe, expect, it } from 'vitest'
import { documentTitle } from './labels'
import type { Route } from '../router'

describe('tiêu đề tab theo màn', () => {

  it('không route nào lộ mã kênh/template/project', () => {
    const ids = ['twitch:nightdrivefm', 'tpl-9f3', 'p-1']
    const routes: Route[] = [
      { name: 'channel', channelId: ids[0], tab: 'sources' },
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
