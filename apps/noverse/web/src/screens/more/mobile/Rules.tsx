// M4 › Quy tắc viết (MMore.dc.html · rules / rulesAdd / rulesThinking). Dữ liệu và thao tác dùng chung với máy tính (useRulesEditor).
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit } from '@noverse/ui/tokens.stylex'
import type { ConfigScope } from '../../../api/types'
import { rulesCopy } from '../../../i18n/vi'
import { BottomSheet } from '../../../mobile/primitives'
import { useTabMode } from '../../../mobile/runtime'
import { useProjectName, useStoryName } from '../../../shell/useNames'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import {
  DEFAULT_BAN, DEFAULT_FATIGUE, GROUP_ORDER, groupMeta, isOwnLevel, learnedMeta, moveTarget, type Level, type RuleEntry,
} from '../../settings/sections/rules/logic'
import { UChips, UnsureFrame } from '../../settings/sections/rules/parts'
import { useRulesEditor } from '../../settings/sections/rules/useRulesEditor'
import type { ScopeTarget } from '../../settings/scope/model'
import { ApplyBar } from './StorySet'
import './rules.css'

const TEMPLATES: [string, string][] = [
  ['Không dùng từ…', 'Không dùng từ “”'], ['Giới hạn lặp…', '“” tối đa 2 lần mỗi chương'], ['Xưng hô…', 'Nhân vật  luôn xưng “”'], ['Giọng văn…', 'Giọng văn '],
]
const styles = stylex.create({
  level: { margin: '8px 16px 0', paddingInline: 14, display: 'flex', flexDirection: 'column', gap: 2 },
  title: { fontSize: 16, fontWeight: 600 },
  note: { fontSize: 12.5, color: color['--color-fg-4'], lineHeight: 1.5 },
  back: { margin: '4px 16px 0', height: 36, padding: '0 12px 0 8px', borderWidth: 0, borderRadius: 10, backgroundColor: kit['--kit-acc-8'], color: color['--color-accent-soft'], fontSize: 13.5, display: 'inline-flex', alignItems: 'center', gap: 6 },
})

export function MobileRules({ project, story, online, toast }: {
  project: string | null; story: string | null
  online: boolean; toast: (text: string) => void
}) {
  const [view, setView] = useState<ScopeTarget>({
    scope: story ? 'story' : project ? 'project' : 'global', project: project ?? '', story: story ?? '',
  })
  const editor = useRulesEditor({ scope: view.scope, project: view.project, story: view.story })
  const storyName = useStoryName(view.project || null, view.story || null)
  const projectLabel = useProjectName(view.project || null)
  const scope = view.scope
  const [draft, setDraft] = useState('')
  const [focused, setFocused] = useState(false)
  const [open, setOpen] = useState<{ level: Level; key: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const area = useRef<HTMLTextAreaElement>(null)

  // Bàn phím đang mở để gõ → thanh tab ẩn đi (MMore `tabMode`).
  useTabMode(focused ? 'hide' : undefined)

  useEffect(() => {
    if (!editor.draftSeed) return
    setDraft(editor.draftSeed.text)
    area.current?.focus()
  }, [editor.draftSeed?.n])

  const editable = editor.canEdit && online
  const empty = !draft.trim()
  const submit = () => {
    if (empty || editor.thinking || !editable) return
    const text = draft
    setDraft('')
    void editor.add(text)
  }
  const note = scope === 'story' ? rulesCopy.storyNote : scope === 'project' ? `Mọi truyện trong dự án “${projectLabel}”.` : 'Mặc định cho mọi dự án và truyện.'
  const when = scope === 'story' && editor.timing ? `Quy tắc mới có hiệu lực từ chương ${editor.timing.effectiveFrom}. Chương đã viết không bị sửa.` : scope === 'story' ? 'Quy tắc mới có hiệu lực từ chương kế tiếp. Chương đã viết không bị sửa.' : 'Áp dụng cho truyện mới và từ chương kế tiếp của truyện đang viết.'
  const entriesOf = (level: Level): RuleEntry[] => level === 'learned' ? editor.runtime : editor.lists[level]

  const target = open ? entriesOf(open.level).find((entry) => entry.key === open.key) : undefined
  const close = () => setOpen(null)
  const actions: { label: string; red?: boolean; run: () => void }[] = !open || !target ? [] : open.level === 'learned'
    ? [{ label: 'Giữ làm quy tắc của truyện', run: () => editor.keepLearned(target) }, { label: 'Bỏ quy tắc này', red: true, run: () => editor.dropLearned(target) }]
    : [
      { label: 'Sửa', run: () => editor.edit(open.level as ConfigScope, target.key) },
      { label: open.level === 'story' ? 'Chuyển lên dự án' : 'Chuyển lên “Mọi truyện”', run: () => { editor.move(open.level as ConfigScope, target.key) } },
      { label: 'Xoá quy tắc', red: true, run: () => { editor.remove(open.level as ConfigScope, target.key) } },
    ]
  // “Chuyển lên” không có ở cấp cao nhất
  const shown = open && open.level !== 'learned' && !moveTarget(open.level as ConfigScope) ? actions.filter((action) => !action.label.startsWith('Chuyển lên')) : actions

  const apply = async () => {
    setBusy(true)
    try { toast(await editor.apply(scope === 'story' ? `truyện “${storyName}”` : scope === 'project' ? `dự án “${projectLabel}”` : 'mọi truyện')) }
    catch (error) { toast(error instanceof Error ? error.message : 'Không áp dụng được. Vui lòng thử lại.') }
    finally { setBusy(false) }
  }
  const barText = `${editor.count} thay đổi${scope === 'story' && editor.timing ? ` · từ chương ${editor.timing.effectiveFrom}` : ''}`

  const changeLevel = (next: ConfigScope) => {
    if (editor.count) { toast(rulesCopy.changeLevel); return }
    setDraft('')
    setOpen(null)
    setView((old) => ({ ...old, scope: next }))
    area.current?.closest('main')?.scrollTo({ top: 0 })
  }

  return <>
    {scope !== 'story' && project && story && <button type="button" {...stylex.props(styles.back)} onClick={() => changeLevel('story')}><Icon name="arrow-left-2" size={15} />{rulesCopy.returnStory}</button>}
    <div {...stylex.props(styles.level)}>
      <span {...stylex.props(styles.title)}>{rulesCopy.mobileTitle(scope, storyName, projectLabel)}</span>
      <span {...stylex.props(styles.note)}>{note}</span>
    </div>
    {!editable && <p className="rm-note">{online ? 'Chỉ người có quyền mới đổi được.' : 'Đang ngoại tuyến — chưa thêm hay sửa quy tắc được.'}</p>}
    <div className={`kit-frame rm-add${focused ? ' f' : ''}`}>
      <textarea ref={area} value={draft} rows={2} disabled={!editable} aria-label="Thêm quy tắc" placeholder="Dặn bằng lời thường, ví dụ: Không dùng từ “bỗng nhiên”"
        onChange={(event) => setDraft(event.target.value)} onFocus={() => setFocused(true)} onBlur={() => window.setTimeout(() => setFocused(false), 120)} />
      <div className="rm-add-bar">
        <div className="rm-hscroll">{TEMPLATES.map(([label, text]) => <button key={label} className="rm-chip" type="button" disabled={!editable} onMouseDown={(event) => { event.preventDefault(); setDraft(text); setFocused(true); area.current?.focus() }}>{label}</button>)}</div>
        <button className="rm-add-go" type="button" disabled={empty || !editable} onMouseDown={(event) => { event.preventDefault(); submit() }}
          onClick={submit} style={{ background: empty ? 'rgba(255,255,255,0.08)' : '#EDEDEF', color: empty ? '#7E7E8A' : '#0B0B0D' }}>Thêm</button>
      </div>
    </div>
    {editor.thinking && <div className="rm-rule new" role="status" style={{ marginTop: 12 }}>
      <span style={{ fontSize: 15 }}>{editor.thinking}</span>
      <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}><Spinner size={12} /><span className="rl-thinking">Noverse đang đọc quy tắc…</span></span>
    </div>}
    {GROUP_ORDER[scope].map((level) => {
      const list = entriesOf(level)
      const meta = groupMeta(level)
      if (!list.length && level !== scope) return null
      const own = isOwnLevel(level, scope)
      return <div key={level}>
        <div className="rm-ghead"><Icon name={meta.icon} size={14} style={{ color: meta.color }} /><span style={{ flex: '1 1 auto' }}>{level === scope && scope !== 'story' ? scope === 'project' ? rulesCopy.ownProject : rulesCopy.nav : meta.label}</span><span className="num">{list.length}</span></div>
        {!own && <div className="mm-group"><button className="rm-grow" type="button" onClick={() => changeLevel(level as ConfigScope)}>
          <span className="rm-preview"><span>{list.map((entry) => entry.text).join(' · ')}</span><span>Kế thừa · chạm để sửa ở cấp này</span></span>
          <Icon name="arrow-right-2" size={16} style={{ color: '#5E5E68' }} />
        </button></div>}
        {own && <div className="rm-list">{list.map((entry) => {
          const learned = level === 'learned'
          const rule = learned ? editor.resource.data?.runtime?.find((item) => item.id === entry.fromRuntime) : undefined
          return <div key={entry.key}>
            <button className={`rm-rule${entry.fresh ? ' new' : ''}`} type="button" disabled={!editable} onClick={() => setOpen({ level, key: entry.key })}>
              <span style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
                {learned && <span className="rl-orb" aria-hidden="true" style={{ width: 13, height: 13, marginTop: 4 }} />}
                <span style={{ fontSize: 15, lineHeight: 1.5 }}>{entry.text}</span>
              </span>
              <span style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}><UChips entry={entry} className="rm-uch" /></span>
              {rule && <span style={{ fontSize: 12.5, color: '#8E8E99' }}>{learnedMeta(rule, true)}</span>}
            </button>
            <UnsureFrame entry={entry} busy={!!editor.resolving} onPick={(option) => { void editor.resolve(level as ConfigScope, entry.key, option) }} />
          </div>
        })}</div>}
      </div>
    })}
    <div className="mm-group" style={{ marginTop: 16 }}>
      <div className="rm-grow" style={{ alignItems: 'flex-start', padding: '12px 14px' }}>
        <Icon name="shield-security" size={20} style={{ color: '#9DB0F7', marginTop: 2 }} />
        <span style={{ flex: '1 1 auto', display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
          <span>Mặc định chống giọng văn máy</span>
          <span style={{ fontSize: 12.5, color: '#8E8E99' }}>{DEFAULT_BAN.length} cụm từ không dùng · {DEFAULT_FATIGUE.length} từ dễ lặp · luôn bật</span>
        </span>
      </div>
    </div>
    <p className="rm-note" style={{ margin: '12px 30px 0', lineHeight: 1.5 }}>{when}</p>
    {editor.count > 0 && <ApplyBar text={barText} busy={busy} onCancel={() => { editor.cancel(); setDraft('') }} onApply={() => { void apply() }} />}
    <BottomSheet open={!!target} onClose={close} height="auto" grab={false} label="Quy tắc" style={{ background: 'transparent', border: 0, boxShadow: 'none', padding: '0 10px 30px', gap: 10 }}>
      <div className="rm-sheet-group"><div className="rm-sheet-text">{target?.text}</div>
        {shown.map((action) => <button key={action.label} className="rm-act" type="button" style={{ color: action.red ? '#FF6B76' : action.label.startsWith('Giữ') ? '#C9A6F5' : '#EDEDEF' }}
          onClick={() => { close(); action.run() }}>{action.label}</button>)}
      </div>
      <div className="rm-sheet-group"><button className="rm-act" type="button" style={{ fontWeight: 600 }} onClick={close}>Huỷ</button></div>
    </BottomSheet>
  </>
}
