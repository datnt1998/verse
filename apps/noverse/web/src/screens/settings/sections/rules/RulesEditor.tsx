// Quy tắc viết dùng chung cho tab truyện, trang dự án và Tuỳ chỉnh; cấp kế thừa chỉ xem.
import { useEffect, useRef, useState } from 'react'
import type { ConfigScope } from '../../../../api/types'
import { navigate, paths } from '../../../../router'
import { rulesCopy as copy } from '../../../../i18n/vi'
import { SaveBar, dirtyText } from '../../savebar'
import { useProjectName, useStoryName } from '../../../../shell/useNames'
import { Card } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Checkbox } from '@noverse/ui'
import { toast } from '@noverse/ui'
import {
  CHIP_STYLE, DEFAULT_BAN, DEFAULT_FATIGUE, GROUP_ORDER, effectiveHint, groupMeta, isOwnLevel, learnedMeta, moveLabel, moveTarget, whenText,
  type Level, type RuleEntry,
} from './logic'
import { GroupHead, RetryCard, ThinkingCard, UChips, UnsureFrame } from './parts'
import { useRulesEditor } from './useRulesEditor'
import './rules.css'

/** “Thêm quy tắc” — 5 mẫu của artboard (TPL). */
const TEMPLATES: [string, string][] = [
  ['Không dùng từ…', 'Không dùng từ “”'], ['Giới hạn từ lặp…', '“” tối đa 2 lần mỗi chương'], ['Xưng hô…', 'Nhân vật  luôn xưng “” với '],
  ['Nhân vật…', ' luôn '], ['Giọng văn…', 'Giọng văn '],
]

export type RulesEditorProps = {
  scope: ConfigScope
  project?: string
  story?: string
  /** Gọi mỗi khi số thay đổi chưa áp dụng đổi. `count` 0 = sạch (ẩn thanh). `hint` chỉ ở cấp truyện: “có hiệu lực từ chương 26”. */
  onDirty?: (count: number, apply: () => Promise<string>, cancel: () => void, hint?: string) => void
}

export function RulesEditor({ scope, project = '', story = '', onDirty }: RulesEditorProps) {
  const editor = useRulesEditor({ scope, project, story })
  const projectName = useProjectName(project || null)
  const storyName = useStoryName(project || null, story || null)
  const [draft, setDraft] = useState('')
  const [focused, setFocused] = useState(false)
  const [under, setUnder] = useState(false)
  const [defaults, setDefaults] = useState(false)
  const [saving, setSaving] = useState(false)
  const area = useRef<HTMLTextAreaElement>(null)

  const object = scope === 'story' ? `truyện “${storyName}”` : scope === 'project' ? `các truyện trong dự án “${projectName}”` : 'mọi truyện'
  const hint = scope === 'story' ? effectiveHint(editor.timing) : undefined

  // thanh Áp dụng của PkgC: gọi lại khi số thay đổi đổi; apply/cancel đọc bản mới nhất qua ref
  const latest = useRef(editor)
  latest.current = editor
  const applyRef = useRef<() => Promise<string>>(() => Promise.resolve(''))
  applyRef.current = () => latest.current.apply(object)
  const stable = useRef({ apply: () => applyRef.current(), cancel: () => { latest.current.cancel(); setDraft('') } }).current
  useEffect(() => { onDirty?.(editor.count, stable.apply, stable.cancel, hint) }, [editor.count, hint])
  useEffect(() => () => onDirty?.(0, stable.apply, stable.cancel), [])

  // “Sửa”: đưa chữ lên ô thêm
  useEffect(() => {
    if (!editor.draftSeed) return
    setDraft(editor.draftSeed.text)
    area.current?.focus()
  }, [editor.draftSeed?.n])

  const editable = editor.canEdit
  const empty = !draft.trim()
  const submit = () => {
    const text = draft
    if (!text.trim() || editor.thinking || editor.retry || !editable) return
    setDraft('')
    void editor.add(text)
  }
  const pickTemplate = (text: string) => { setDraft(text); area.current?.focus() }

  const levels = GROUP_ORDER[scope]
  const entriesOf = (level: Level): RuleEntry[] => level === 'learned' ? editor.runtime : editor.lists[level]
  const fg = (inherited: boolean) => inherited ? '#B8B8C2' : '#EDEDEF'

  const remove = (level: ConfigScope, key: string) => { editor.remove(level, key); toast('Đã xoá · bấm Huỷ ở thanh dưới để lấy lại') }
  const move = (level: ConfigScope, key: string) => {
    const target = editor.move(level, key)
    if (target) toast(target === 'project' ? 'Đã chuyển lên dự án' : 'Đã chuyển lên “Mọi truyện”')
  }
  const keep = (entry: RuleEntry) => { editor.keepLearned(entry); toast('Đã chuyển thành quy tắc của truyện') }
  const editParent = (level: ConfigScope) => {
    if (editor.count) { toast(copy.changeLevel); return }
    navigate(level === 'project' ? `${paths.project(project)}?section=prules` : paths.settings('rules'))
  }
  const apply = async () => {
    setSaving(true)
    try { toast(await stable.apply()) }
    catch (error) { toast(error instanceof Error ? error.message : copy.saveError) }
    finally { setSaving(false) }
  }
  const timing = <Card radius={scope === 'story' ? 14 : 16} padding={16} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
    <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13.5, fontWeight: 600 }}><Icon name="clock" size={16} style={{ color: 'var(--color-accent)' }} />{copy.when}</span>
    <span style={{ fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.55 }}>{whenText(scope, editor.timing)}</span>
    <Checkbox checked={editor.recheck} onChange={editor.setRecheck}
      style={{ alignItems: 'flex-start', gap: 10, padding: '2px 0 0', margin: 0, fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.5 }}
      label={copy.recheck(editor.timing?.completed)} />
  </Card>

  return <div className="rl-root" data-rules-level={scope} style={{ display: 'flex', gap: 24, alignItems: 'flex-start', marginTop: scope === 'global' ? 18 : 0 }}>
    <div style={{ flex: '1 1 auto', minWidth: 0, maxWidth: scope === 'global' ? 780 : 860, display: 'flex', flexDirection: 'column', gap: scope === 'project' ? 10 : 14 }}>
      <div style={{ display: 'flex', alignItems: scope === 'global' ? 'flex-end' : 'flex-start', gap: 16 }}>
        <div style={{ flex: '1 1 auto' }}>
          {scope === 'global' && <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.01em' }}>{copy.globalTitle}</h1>}
          {scope !== 'global' && <h2 style={{ margin: 0, fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}><Icon name="edit-2" size={17} style={{ color: 'var(--color-accent)' }} />Quy tắc</h2>}
        </div>
        <button type="button" className="rl-btn-ghost" onClick={() => setUnder((open) => !open)} aria-expanded={under}
          style={{ height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#C9C9D1', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', flex: '0 0 auto' }}>
          <Icon name="magic-star" size={15} style={{ color: '#C9A6F5' }} />{under ? 'Ẩn bản tổng hợp' : 'Noverse hiểu thế nào?'}
        </button>
      </div>
      <p style={{ margin: '-6px 2px 0', fontSize: 13, color: 'var(--color-fg-4)', lineHeight: 1.55 }}>{scope === 'story' ? copy.storyIntro : scope === 'project' ? copy.projectIntro(projectName) : copy.globalIntro}</p>

      {under && <section className="card rl-drawer" aria-label="Noverse đang hiểu" style={{ borderRadius: 16, padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 12 }}>
        <span style={{ fontSize: 13, color: '#A0A0AB' }}>Tổng hợp mọi quy tắc đang áp dụng cho {object} — đây là thứ trợ lý thật sự nhận được.</span>
        {editor.summary.map((row) => {
          const style = CHIP_STYLE[row.kind]
          return <div key={row.kind} style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
            <span style={{ width: 120, flex: '0 0 auto', fontSize: 12.5, color: style.fg, display: 'flex', alignItems: 'center', gap: 6, paddingTop: 2 }}><Icon name={row.icon} size={14} />{row.label}</span>
            <span style={{ flex: '1 1 auto', display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {row.items.map((item, index) => <span key={`${item}:${index}`} className="rl-uchip" style={{ background: style.bg, color: style.fg }}>{item}</span>)}
            </span>
          </div>
        })}
      </section>}

      {scope === 'global' && <Card radius={14} padding="12px 14px" role="note" style={{ display: 'flex', alignItems: 'center', gap: 12, borderColor: 'var(--kit-acc-22)', background: 'var(--frame-acc-4)' }}>
        <Icon name="folder-open" size={18} style={{ color: 'var(--color-accent)', flex: '0 0 auto' }} />
        <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.45 }}>
          <span style={{ fontSize: 13.5, fontWeight: 500 }}>{copy.guideTitle}</span>
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{copy.guideText}</span>
        </span>
        <a className="rl-btn-ghost" href={`#${project && story ? `${paths.story(project, story)}?tab=style` : project ? `${paths.project(project)}?section=pstyle` : '/'}`}
          style={{ height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid var(--kit-line-12)', color: 'var(--color-fg)', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', flex: '0 0 auto' }}>{copy.guideLink}<Icon name="arrow-right-2" size={13} /></a>
      </Card>}
      <div className={`kit-frame rl-addbox${focused ? ' f' : ''}`}>
        <textarea ref={area} id="r-add" value={draft} rows={2} disabled={!editable} aria-label="Thêm quy tắc mới"
          placeholder={scope === 'project' ? copy.projectPlaceholder : scope === 'story' ? copy.storyPlaceholder : copy.placeholder}
          onChange={(event) => setDraft(event.target.value)} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
          onKeyDown={(event) => { if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) { event.preventDefault(); submit() } }} />
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '4px 6px 6px 8px', flexWrap: 'wrap' }}>
          {TEMPLATES.map(([label, text]) => <button key={label} type="button" className="rl-tpl" disabled={!editable} onMouseDown={(event) => { event.preventDefault(); pickTemplate(text) }}>{label}</button>)}
          <span style={{ flex: '1 1 auto' }} />
          <button type="button" className="rl-btn-primary" disabled={empty || !editable || !!editor.thinking || !!editor.retry} onMouseDown={(event) => { event.preventDefault(); submit() }}
            onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); submit() } }}
            style={{ height: 32, padding: '0 14px', borderRadius: 9, border: 'none', background: empty ? 'rgba(255,255,255,0.08)' : '#EDEDEF', color: empty ? '#7E7E8A' : '#0B0B0D', fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 6 }}>
            <Icon name="add" size={15} />Thêm
          </button>
        </div>
      </div>

      {editor.thinking && <ThinkingCard text={editor.thinking} />}
      {editor.retry && <RetryCard retry={editor.retry} onSaveRaw={editor.saveRaw} onCancel={editor.cancelRetry} />}

      {levels.map((level) => {
        const list = entriesOf(level)
        const meta = groupMeta(level, projectName)
        const inherited = !isOwnLevel(level, scope)
        if (!list.length && level !== scope) return null
        const rowEditable = editable && isOwnLevel(level, scope) && level !== 'learned'
        return <div key={level} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <GroupHead icon={meta.icon} color={meta.color} label={level === scope && scope !== 'story' ? copy.ownProject : meta.label} sub={inherited ? copy.readOnly : level === 'learned' ? 'ghi lại khi bạn dặn trong chat, giữ hoặc bỏ' : ''}
            link={inherited ? (level === 'project' ? copy.editProject : copy.editGlobal) : undefined}
            onLink={() => editParent(level as ConfigScope)} />
          {list.map((entry) => {
            const learned = level === 'learned'
            const rule = learned ? editor.resource.data?.runtime?.find((item) => item.id === entry.fromRuntime) : undefined
            return <div key={entry.key} className={`rl-rule${inherited ? ' rl-inherited' : ''}${entry.fresh ? ' new' : ''}`} aria-busy={editor.resolving === entry.key} style={inherited ? { borderStyle: 'dashed', textAlign: 'left', alignItems: 'stretch' } : undefined}>
              <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start', paddingRight: rowEditable ? 96 : 0 }}>
                {learned && <span className="rl-orb" aria-hidden="true" style={{ width: 14, height: 14, marginTop: 4 }} />}
                <span style={{ fontSize: 14.5, lineHeight: 1.55, color: fg(inherited) }}>{entry.text}</span>
              </div>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, alignItems: 'center' }}>
                <UChips entry={entry} />
                {learned && rule && <span style={{ fontSize: 12, color: '#7E7E8A', marginLeft: 4 }}>{learnedMeta(rule)}</span>}
              </div>
              <UnsureFrame entry={entry} busy={!rowEditable || !!editor.resolving} readOnly={!rowEditable} onPick={(option) => { void editor.resolve(level as ConfigScope, entry.key, option) }} />
              {learned && editable && <div style={{ display: 'flex', gap: 8 }}>
                <button type="button" className="rl-btn-ghost" onClick={() => keep(entry)} style={{ height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 5 }}>
                  <Icon name="tick-circle" size={13} style={{ color: '#A6EDC1' }} />Giữ làm quy tắc của truyện
                </button>
                <button type="button" className="rl-btn-ghost" onClick={() => editor.dropLearned(entry)} style={{ height: 28, padding: '0 10px', borderRadius: 8, border: 'none', background: 'transparent', color: '#A0A0AB', fontSize: 12.5 }}>Bỏ</button>
              </div>}
              {rowEditable && <div className="rl-acts">
                <button type="button" aria-label="Sửa quy tắc" onClick={() => editor.edit(level as ConfigScope, entry.key)}><Icon name="edit-2" size={15} /></button>
                {moveTarget(scope) && <button type="button" aria-label={moveLabel(level as ConfigScope)} title={moveLabel(level as ConfigScope)} onClick={() => move(level as ConfigScope, entry.key)}><Icon name="arrow-up" size={15} /></button>}
                <button type="button" aria-label="Xoá quy tắc" onClick={() => remove(level as ConfigScope, entry.key)}><Icon name="trash" size={15} /></button>
              </div>}
            </div>
          })}
        </div>
      })}

      <Card radius={14} padding={0} style={{ overflow: 'hidden' }}>
        <button type="button" className="rl-row" onClick={() => setDefaults((open) => !open)} aria-expanded={defaults}
          style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '14px 16px', border: 'none', background: 'transparent', color: '#EDEDEF', textAlign: 'left' }}>
          <Icon name="shield-security" size={17} style={{ color: '#9DB0F7' }} />
          <span style={{ flex: '1 1 auto', display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
            <span style={{ fontSize: 14, fontWeight: 500 }}>Mặc định của Noverse · chống giọng văn máy</span>
            <span style={{ fontSize: 12.5, color: '#8E8E99' }}>Luôn bật</span>
          </span>
          <Icon name="arrow-down-2" size={15} style={{ color: '#7E7E8A', transform: `rotate(${defaults ? 180 : 0}deg)`, transition: 'transform .25s' }} />
        </button>
        {defaults && <div className="rl-drawer" style={{ padding: '0 16px 16px 45px', display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>{DEFAULT_BAN.map((word) => <span key={word} className="rl-uchip" style={{ background: 'rgba(255,159,168,0.08)', color: '#FFC4CA' }}>“{word}”</span>)}</div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>{DEFAULT_FATIGUE.map(([word, limit]) => <span key={word} className="rl-uchip" style={{ background: 'rgba(246,203,140,0.08)', color: '#F3D9AE' }}>“{word}” ≤ {limit}</span>)}</div>
          <span style={{ fontSize: 12.5, color: '#8E8E99' }}>Muốn nới một từ? Thêm quy tắc như “cho phép ‘dường như’ tới 4 lần mỗi chương” — quy tắc của bạn được ưu tiên.</span>
        </div>}
      </Card>
      {scope !== 'global' && timing}
    </div>

    {scope === 'global' && <aside style={{ flex: '0 0 300px', display: 'flex', flexDirection: 'column', gap: 12, position: 'sticky', top: 0 }}>
      {timing}
      <section style={{ padding: '4px 6px', display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12.5, color: '#8E8E99', lineHeight: 1.5 }}>
        <span style={{ color: '#A0A0AB', fontWeight: 500 }}>Mẹo viết quy tắc</span>
        <span>• Mỗi quy tắc một ý, càng cụ thể càng tốt.</span>
        <span>• Từ bị cấm đặt trong ngoặc kép: “bỗng nhiên”.</span>
        <span>• Muốn nói một lần cho chương sau thì nhắn trong ô chat ở Bàn viết, không cần thêm ở đây.</span>
      </section>
    </aside>}
    {!onDirty && editor.count > 0 && <SaveBar text={dirtyText(editor.count, scope, hint)} applyLabel="Áp dụng" saving={saving} disabled={saving || !editable} onApply={() => void apply()} onCancel={stable.cancel} />}
  </div>
}
