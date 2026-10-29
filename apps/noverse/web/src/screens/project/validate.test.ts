import { describe, expect, it } from 'vitest'
import type { ValidationIssue } from '../../api/types'
import { fixableCodes, plainIssues, type PlainContext } from './validate'

const dirs = { ideas: 'y-tuong', assets: 'tai-san', stories: 'kich_ban', exports: 'xuat-ban' }
const context: PlainContext = {
  projectName: 'Kênh truyện audio',
  dirs,
  stories: [{ story: '01-tra-xanh', folder: 'kich_ban/01-tra-xanh', title: 'Trà Xanh Cướp Kịch Bản', tempName: false }],
}
const issue = (code: string, path: string, fixable: boolean, level: ValidationIssue['level'] = 'warn'): ValidationIssue =>
  ({ code, path, fixable, level, message: `RAW ${path}` })
const texts = (issues: ValidationIssue[], ctx = context) => plainIssues(issues, ctx).map((entry) => entry.text)

describe('plainIssues — bảng mã kiểm tra (20-settings-rules.md §4)', () => {
  it('missing-project-json', () => {
    expect(texts([issue('missing-project-json', 'project.json', true, 'error')])).toEqual([
      'Dự án chưa có thông tin — Noverse sẽ tạo mới với tên “Kênh truyện audio”.',
    ])
  })

  it('invalid-project-json is not fixable', () => {
    const [entry] = plainIssues([issue('invalid-project-json', 'project.json', false, 'error')], context)
    expect(entry.text).toBe('Thông tin dự án bị hỏng, Noverse không đọc được. Nhờ quản trị kiểm tra.')
    expect(entry.fixable).toBe(false)
  })

  it('missing-dir names the section, not the folder', () => {
    expect(texts([
      issue('missing-dir', 'y-tuong', true),
      issue('missing-dir', 'tai-san', true),
      issue('missing-dir', 'kich_ban', true),
      issue('missing-dir', '.ainovel/rules', true),
    ])).toEqual([
      'Thiếu mục Ý tưởng — sẽ tạo mục trống.',
      'Thiếu mục Tư liệu — sẽ tạo mục trống.',
      'Thiếu mục Truyện — sẽ tạo mục trống.',
      'Thiếu mục Quy tắc viết — sẽ tạo mục trống.',
    ])
  })

  it('missing-dir follows a customised folder name', () => {
    expect(texts([issue('missing-dir', 'ban-nhap', true)], { ...context, dirs: { ...dirs, ideas: 'ban-nhap' } }))
      .toEqual(['Thiếu mục Ý tưởng — sẽ tạo mục trống.'])
  })

  it('story codes use the story title', () => {
    expect(texts([
      issue('missing-story-json', 'kich_ban/01-tra-xanh/story.json', true),
      issue('invalid-story-json', 'kich_ban/01-tra-xanh/story.json', false, 'error'),
    ])).toEqual([
      'Truyện “Trà Xanh Cướp Kịch Bản” chưa có thông tin trạng thái — sẽ tạo mới với trạng thái “Đang viết”.',
      'Thông tin của truyện “Trà Xanh Cướp Kịch Bản” bị hỏng. Nhờ quản trị kiểm tra.',
    ])
  })

  it('story title never falls back to a slug', () => {
    const [text] = texts([issue('missing-story-json', 'kich_ban/02-cuoi-gia/story.json', true)])
    expect(text).toBe('Truyện “Cuoi Gia” chưa có thông tin trạng thái — sẽ tạo mới với trạng thái “Đang viết”.')
    expect(text).not.toMatch(/02-|cuoi-gia/)
  })

  it('invalid-name is never shown', () => {
    expect(plainIssues([issue('invalid-name', 'kich_ban/Bad Name', false, 'info')], context)).toEqual([])
  })

  it('legacy-rules is counted, as on the artboard', () => {
    expect(texts([
      issue('legacy-rules', '.ainovel/chung/a.md', true),
      issue('legacy-rules', '.ainovel/chung/b.md', true),
    ])).toEqual(['Có 2 quy tắc viết kiểu cũ — sẽ chuyển vào Quy tắc viết của dự án.'])
    expect(texts([issue('legacy-rules', '.ainovel/chung/a.md', true)]))
      .toEqual(['Có 1 quy tắc viết kiểu cũ — sẽ chuyển vào Quy tắc viết của dự án.'])
  })

  it('rules-symlink collapses to one sentence', () => {
    expect(texts([
      issue('rules-symlink', 'kich_ban/01-tra-xanh/.ainovel/rules', true),
      issue('rules-symlink', 'kich_ban/02-x/.ainovel/rules/a.md', true),
    ])).toEqual(['Quy tắc của truyện đang trỏ sang dự án — sẽ gỡ để tránh áp hai lần.'])
  })

  it('an unknown code never leaks the raw server message', () => {
    const [entry] = plainIssues([issue('something-new', 'kich_ban/x/output/novel/outline.json', false)], context)
    expect(entry.text).toBe('Có một điểm chưa đúng chuẩn trong dự án. Nhờ quản trị kiểm tra.')
  })

  it('no sentence contains a path, a file name or a code', () => {
    const all = texts([
      issue('missing-project-json', 'project.json', true, 'error'),
      issue('invalid-project-json', 'project.json', false, 'error'),
      issue('missing-dir', 'y-tuong', true),
      issue('missing-story-json', 'kich_ban/01-tra-xanh/story.json', true),
      issue('invalid-story-json', 'kich_ban/01-tra-xanh/story.json', false, 'error'),
      issue('legacy-rules', '.ainovel/chung/a.md', true),
      issue('rules-symlink', 'kich_ban/01-tra-xanh/.ainovel/rules', true),
    ]).join('\n')
    expect(all).not.toMatch(/y-tuong|tai-san|kich_ban|xuat-ban|\.json|\.md|output\/novel|RAW|-json|symlink/)
  })

  it('collapsed rows stay fixable only when every source issue is fixable', () => {
    const entries = plainIssues([issue('legacy-rules', 'a.md', true), issue('legacy-rules', 'b.md', true)], context)
    expect(entries).toHaveLength(1)
    expect(entries[0].fixable).toBe(true)
  })
})

describe('fixableCodes', () => {
  it('sends only codes the server can fix, once each', () => {
    const entries = plainIssues([
      issue('missing-story-json', 'kich_ban/01-tra-xanh/story.json', true),
      issue('invalid-story-json', 'kich_ban/01-tra-xanh/story.json', false, 'error'),
      issue('legacy-rules', 'a.md', true),
      issue('legacy-rules', 'b.md', true),
      issue('invalid-name', 'kich_ban/Bad', false, 'info'),
    ], context)
    expect(fixableCodes(entries)).toEqual(['missing-story-json', 'legacy-rules'])
  })
})
