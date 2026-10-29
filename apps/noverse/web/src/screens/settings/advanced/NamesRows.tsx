// Nhóm “Tên thư mục & tên tệp xuất” — Settings.dc.html advGroups[1]: ba hàng, mỗi hàng “Đổi” mở bộ sửa ngay dưới hàng.
// Ghi vào bản nháp library.json (`libDraft.dirs` · `slugWords` · `autoExport.pattern`), lưu cùng thanh Lưu.
import { useState, type ReactNode } from 'react'
import type { LibraryConfig } from '../../../api/types'
import { Field } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { ReadOnlyValue } from '../components'
import { clampSlugWords, dirNameState, renderPattern, storyFolderName, withKeepSuffix } from '../../../lib/slug'
import type { SettingsContext } from '../types'
import { AdvButton, AdvRow } from './AdvGroup'
import { dirsValue, slugWordsValue } from './logic'

const DEFAULT_DIRS: LibraryConfig['dirs'] = { ideas: 'y-tuong', assets: 'tai-san', stories: 'kich_ban', exports: 'xuat-ban' }

const DIR_ROWS: { key: keyof LibraryConfig['dirs']; label: string }[] = [
  { key: 'ideas', label: 'Ý tưởng' },
  { key: 'assets', label: 'Tư liệu' },
  { key: 'stories', label: 'Truyện' },
  { key: 'exports', label: 'Bản xuất' },
]

const LOCKED = [
  ['project.json', 'Thông tin dự án'],
  ['.ainovel/config.json', 'Model/provider chung của dự án'],
  ['story.json', 'Trạng thái truyện'],
  ['output/novel/', 'Engine ainovel quản lý'],
] as const

const editor = { display: 'flex', flexDirection: 'column', gap: 8, padding: '4px 0 12px' } as const
const hint = { fontSize: 12, color: '#8E8E99', lineHeight: 1.5 } as const

function Editor({ id, children }: { id: string; children: ReactNode }) {
  return <div id={id} style={editor}>{children}</div>
}

export function NamesRows({ ctx }: { ctx: SettingsContext }) {
  const lib = ctx.libDraft
  const [editing, setEditing] = useState<'dirs' | 'slug' | 'pattern' | null>(null)
  const toggle = (id: 'dirs' | 'slug' | 'pattern') => setEditing((current) => (current === id ? null : id))
  if (!lib) {
    return <AdvRow label="Tên thư mục & tên tệp xuất" value={<Skeleton width={220} height={14} />} />
  }
  const readOnly = !ctx.admin
  const patchDirs = (key: keyof LibraryConfig['dirs'], value: string) => ctx.setLibDraft({ ...lib, dirs: { ...lib.dirs, [key]: value } })
  const patchExport = (next: Partial<LibraryConfig['autoExport']>) => ctx.setLibDraft({ ...lib, autoExport: { ...lib.autoExport, ...next } })

  const storyFolder = storyFolderName(ctx.sampleStoryIndex, ctx.sampleStoryTitle, lib.slugWords)
  const rendered = renderPattern(lib.autoExport.pattern, { storyFolder, formats: lib.autoExport.formats })
  const previewPath = `${lib.dirs.stories}/${storyFolder}/${lib.dirs.exports}/${rendered}`

  return <>
    <AdvRow label="Ý tưởng · Tư liệu · Truyện · Bản xuất" value={dirsValue(lib.dirs)}
      below={editing === 'dirs' ? (
        <Editor id="adv-names-dirs">
          {DIR_ROWS.map((row) => {
            const value = lib.dirs[row.key]
            const state = dirNameState(value, DEFAULT_DIRS[row.key])
            const invalid = state === 'invalid'
            return (
              <div key={row.key} style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 8 }}>
                <label htmlFor={`dir-${row.key}`} style={{ flex: '0 0 120px', fontSize: 13, color: '#C9C9D1' }}>{row.label}</label>
                <Field id={`dir-${row.key}`} className="mono" value={value} disabled={readOnly}
                  aria-invalid={invalid || undefined} aria-describedby={`dir-${row.key}-state`}
                  onChange={(event) => patchDirs(row.key, event.target.value)}
                  style={{ flex: '1 1 140px', height: 34, borderRadius: 8, fontSize: 12, minWidth: 110 }} />
                <span id={`dir-${row.key}-state`} style={{
                  flex: '0 0 auto', display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 12,
                  color: invalid ? 'var(--color-err)' : 'var(--color-ok)',
                }}>
                  <Icon name={invalid ? 'warning-2' : 'tick-circle-bulk'} size={14} />
                  {invalid ? 'Sai chuẩn' : state === 'standard' ? 'Chuẩn' : 'Hợp lệ'}
                </span>
              </div>
            )
          })}
          <span style={hint}>Đổi được, nhưng chỉ dùng chữ thường không dấu, số, “-” hoặc “_”.</span>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {LOCKED.map(([name, why]) => <ReadOnlyValue key={name} title={why} icon="shield-tick">{name}</ReadOnlyValue>)}
          </div>
          <span style={hint}>Các tệp khoá là chuẩn chung, không đổi tên: engine và lệnh serve đọc theo tên cố định.</span>
        </Editor>
      ) : undefined}>
      <AdvButton expanded={editing === 'dirs'} disabled={readOnly} onClick={() => toggle('dirs')}>Đổi</AdvButton>
    </AdvRow>

    <AdvRow label="Số từ giữ trong tên thư mục truyện" value={slugWordsValue(lib.slugWords)}
      below={editing === 'slug' ? (
        <Editor id="adv-names-slug">
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, flexWrap: 'wrap' }}>
            <label htmlFor="slugw" style={{ color: '#C9C9D1' }}>Giữ tối đa</label>
            <Field id="slugw" className="num" type="number" min={2} max={12} value={lib.slugWords} disabled={readOnly}
              onChange={(event) => ctx.setLibDraft({ ...lib, slugWords: clampSlugWords(Number(event.target.value)) })}
              style={{ width: 70, height: 34, borderRadius: 8, fontSize: 13 }} />
            <span style={{ color: '#8E8E99' }}>từ đầu của tên truyện</span>
          </div>
          <span style={hint}>Số thứ tự 2 chữ số, slug bỏ dấu tiếng Việt, chữ thường, nối bằng “-”.</span>
          <div style={{ padding: '10px 12px', borderRadius: 10, background: '#0F0F11', border: '1px solid rgba(255,255,255,0.07)', display: 'flex', flexDirection: 'column', gap: 4 }}>
            <span style={{ fontSize: 12, color: '#8E8E99' }}>{ctx.sampleStoryTitle}</span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
              <Icon name="arrow-right-2" size={13} style={{ color: '#8E8E99' }} />
              <code className="mono" style={{ fontSize: 12, color: 'var(--color-ok)', wordBreak: 'break-all' }}>{lib.dirs.stories}/{storyFolder}/</code>
            </span>
          </div>
        </Editor>
      ) : undefined}>
      <AdvButton expanded={editing === 'slug'} disabled={readOnly} onClick={() => toggle('slug')}>Đổi</AdvButton>
    </AdvRow>

    <AdvRow label="Mẫu tên tệp xuất" value={lib.autoExport.pattern}
      below={editing === 'pattern' ? (
        <Editor id="adv-names-pattern">
          <Field id="xpat" className="mono" aria-label="Mẫu tên tệp xuất" value={lib.autoExport.pattern} disabled={readOnly}
            onChange={(event) => patchExport({ pattern: event.target.value })} style={{ height: 34, borderRadius: 8, fontSize: 12 }} />
          <span style={hint}>Biến: {'{truyen} {tap} {tu} {den} {ngay} {ext}'}</span>
          <code className="mono" style={{ fontSize: 12, color: 'var(--color-ok)', wordBreak: 'break-all' }}>{previewPath}</code>
          {lib.autoExport.onExists === 'keep' && (
            <span style={{ fontSize: 11, color: '#8E8E99' }}>Nếu tệp đã có → <code className="mono" style={{ wordBreak: 'break-all' }}>{withKeepSuffix(previewPath)}</code></span>
          )}
        </Editor>
      ) : undefined}>
      <AdvButton expanded={editing === 'pattern'} disabled={readOnly} onClick={() => toggle('pattern')}>Đổi</AdvButton>
    </AdvRow>
  </>
}
