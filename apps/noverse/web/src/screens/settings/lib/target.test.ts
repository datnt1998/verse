import { describe, expect, it } from 'vitest'
import { parseHash, paths } from '../../../router'
import { settingsLink } from '../../project/links'
import { parseSettingsTarget, resolveSection } from './target'

describe('đọc mục + cấp từ đường dẫn Tuỳ chỉnh', () => {
  it('liên kết từ màn Dự án quay lại đúng mục, cấp, dự án, truyện', () => {
    const parsed = parseHash(`#${settingsLink('info', { project: 'kenh', story: '01-tra-xanh' })}`)
    expect(parsed.route).toEqual({ name: 'settings', section: 'info' })
    expect(parseSettingsTarget('info', parsed.search)).toEqual({ section: 'info', scope: 'story', project: 'kenh', story: '01-tra-xanh' })
    const project = parseHash(`#${settingsLink('info', { project: 'kenh' })}`)
    expect(parseSettingsTarget('info', project.search)).toMatchObject({ scope: 'project', project: 'kenh', story: '' })
  })

  it('chấp nhận ?section= khi đường dẫn không có mục, bỏ cấp lạ', () => {
    const search = new URLSearchParams('section=budget&scope=weird&project=p')
    expect(parseSettingsTarget(null, search)).toEqual({ section: 'budget', scope: null, project: 'p', story: '' })
  })

  it('tên mục cũ đổi sang mục mới; quản trị mới thấy nhóm Hệ thống', () => {
    expect(resolveSection('members', true)).toBe('members')
    expect(resolveSection('members', false)).toBe('rules')
    expect(resolveSection('roles', false)).toBe('models')
    expect(resolveSection('providers', true)).toBe('connect')
    expect(resolveSection('library', true)).toBe('export')
    expect(resolveSection('providers', false)).toBe('rules')
    expect(resolveSection('adv', false)).toBe('rules')
    expect(resolveSection(null, false)).toBe('rules')
    expect(resolveSection('nonsense', true)).toBe('rules')
    expect(paths.settings(resolveSection('style', false))).toBe('/settings/writing')
  })
})
