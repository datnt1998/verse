import { describe, expect, it } from 'vitest'
import type { Config, ConfigDoc } from '../../../api/types'
import { countChanges, effectiveFromChapter, hasOwn, inheritedFrom, ovrOf } from './labels'

describe('nhãn kế thừa (ovr)', () => {
  it('cấp toàn cục luôn “Mặc định”, không có nút đặt lại', () => {
    expect(ovrOf('global', true)).toEqual({ cls: 'inh', text: 'Mặc định', reset: false })
  })
  it('có khoá ở chính cấp → tím “Riêng …” kèm nút đặt lại', () => {
    expect(ovrOf('story', true)).toEqual({ cls: 'own', text: 'Riêng truyện này', reset: true })
    expect(ovrOf('project', true)).toEqual({ cls: 'own', text: 'Riêng dự án', reset: true })
  })
  it('không có khoá: truyện theo dự án khi dự án cung cấp, theo “Mọi truyện” khi chỉ toàn cục cung cấp', () => {
    expect(ovrOf('story', false, 'project').text).toBe('Theo dự án')
    expect(ovrOf('story', false, 'global').text).toBe('Theo “Mọi truyện”')
    expect(ovrOf('story', false).text).toBe('Theo dự án')
    expect(ovrOf('project', false).text).toBe('Theo “Mọi truyện”')
  })
})

describe('inheritedFrom', () => {
  const chain: ConfigDoc[] = [
    { scope: 'global' as const, path: '', exists: true, config: { roles: { writer: { provider: 'a', model: 'm1' } } } },
    { scope: 'project' as const, path: '', exists: true, config: {} },
  ]
  it('dùng `sources` khi nguồn là cấp khác cấp đang sửa', () => {
    expect(inheritedFrom({ 'roles.writer.model': 'project' }, ['roles.writer.model'], 'story')).toBe('project')
  })
  it('khoá vừa bị xoá khỏi bản nháp (sources vẫn báo chính cấp này) → quét chuỗi cấp dưới', () => {
    expect(inheritedFrom({ 'roles.writer.model': 'story' }, ['roles.writer.model'], 'story', chain)).toBe('global')
    const withProject: ConfigDoc[] = [chain[0], { ...chain[1], config: { roles: { writer: { provider: 'a', model: 'm2' } } } }]
    expect(inheritedFrom({ 'roles.writer.model': 'story' }, ['roles.writer.model'], 'story', withProject)).toBe('project')
  })
})

describe('đếm thay đổi cho thanh nổi', () => {
  it('đổi cả provider lẫn model của một vai = một thay đổi; thêm khoá riêng ở vai khác = hai', () => {
    const base: Config = { roles: { writer: { provider: 'a', model: 'm1' } } }
    const one: Config = { roles: { writer: { provider: 'b', model: 'm2' } } }
    expect(countChanges(base, one)).toBe(1)
    expect(countChanges(base, { roles: { ...one.roles, editor: { provider: 'a', model: 'm1' } } })).toBe(2)
    expect(countChanges(base, base)).toBe(0)
  })
  it('xoá khoá cũng tính', () => {
    expect(countChanges({ style: 'wuxia' }, {})).toBe(1)
  })
  it('hasOwn chỉ nhìn khoá có trong bản nháp của cấp', () => {
    expect(hasOwn({ roles: { writer: { provider: 'a', model: 'm' } } }, ['roles.writer.model'])).toBe(true)
    expect(hasOwn({}, ['roles.writer.model'])).toBe(false)
  })
})

describe('có hiệu lực từ chương', () => {
  it('sau chương đang viết dở hoặc chương xong cuối cùng', () => {
    expect(effectiveFromChapter({ completedCount: 24, inProgressChapter: 25 })).toBe(26)
    expect(effectiveFromChapter({ completedCount: 24, inProgressChapter: 0 })).toBe(25)
    expect(effectiveFromChapter(null)).toBe(1)
  })
})
