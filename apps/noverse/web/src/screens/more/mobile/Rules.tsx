// M4 › Quy tắc viết (MMore.dc.html · rules / rulesAdd / rulesThinking). Dữ liệu và thao tác dùng chung với máy tính (useRulesEditor).
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit } from '@noverse/ui/tokens.stylex'
import type { ConfigScope } from '../../../api/types'
import { getConfig, getConfigPresets, getStyleSamples } from '../../../api/client'
import { keys, useAction, useResource } from '../../../data'
import { PackSummaryCard, saveStyle, styleCards, styleDisplayName } from '../../settings/sections/style'
import { findPack, packBadge, packCards, packDesc } from '../../settings/lib/packs'
import { amWritingCopy } from '../../../i18n/vi'
import { rulesCopy } from '../../../i18n/vi'
import { BottomSheet } from '../../../mobile/primitives'
import { useTabMode } from '../../../mobile/runtime'
import { useProjectName, useStoryName } from '../../../shell/useNames'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import {
  GROUP_ORDER, groupMeta, isOwnLevel, learnedMeta, moveTarget, type Level, type RuleEntry,
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
  choice: { width: '100%', borderWidth: 0, backgroundColor: 'transparent', textAlign: 'left' },
  choiceName: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  tag: { height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11.5, display: 'inline-flex', alignItems: 'center', backgroundColor: kit['--kit-acc-8'], color: kit['--kit-info-fg'] },
  badge: { height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11.5, display: 'inline-flex', alignItems: 'center', backgroundColor: kit['--kit-ok-8'], color: color['--color-ok'] },
  tick: { color: color['--color-accent'], flexShrink: 0 },
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
  const [styleOpen, setStyleOpen] = useState(false)
  const config = useResource(keys.config(scope, view.project, view.story), ({ signal }) => getConfig(scope, view.project, view.story, signal))
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const samples = useResource(keys.story(view.project, view.story, 'style-samples'), ({ signal }) => getStyleSamples(view.project, view.story, signal), { enabled: scope === 'story' && !!view.story })
  const packs = packCards(presets.data?.packs)
  const activePack = findPack(presets.data?.packs, config.data?.effective.style)
  const choices = styleCards(presets.data?.styles ?? [], config.data?.effective.style ?? '', { packs: packs.map((p) => p.id), custom: 'story', samples: samples.data?.total })
  const currentStyle = config.data?.effective.style ?? 'default'
  const inherited = config.data?.sources.style !== scope
  const parentStyle = config.data?.chain.find((d) => d.scope === 'project')?.config.style ?? config.data?.chain.find((d) => d.scope === 'global')?.config.style ?? 'default'
  const pickStyle = useAction(async (style: string | null) => {
    if (!config.data || scope === 'global') return
    await saveStyle(scope, view.project, scope === 'story' ? view.story : undefined, style ?? undefined, config.data)
    setStyleOpen(false)
    toast(`Đã đổi văn phong · áp dụng từ chương ${editor.timing?.effectiveFrom ?? 'kế tiếp'}`)
  }, { invalidate: [['project', view.project, 'style-by']] })

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
  const note = scope === 'story' ? amWritingCopy.intro : scope === 'project' ? `Mọi truyện trong dự án “${projectLabel}”.` : 'Mặc định cho mọi dự án và truyện.'
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
      {scope !== 'story' && <span {...stylex.props(styles.title)}>{rulesCopy.mobileTitle(scope, storyName, projectLabel)}</span>}
      <span {...stylex.props(styles.note)}>{note}</span>
    </div>
    {scope === 'story' && <>
      <div className="mm-heading">Văn phong</div>
      <div className="mm-group"><button className="mm-row" type="button" aria-expanded={styleOpen} disabled={!editable || !config.data} onClick={() => setStyleOpen(!styleOpen)}><Icon name="brush-2" size={22} {...stylex.props(styles.tick)} /><span className="mm-copy"><span {...stylex.props(styles.choiceName)}>{styleDisplayName(presets.data, currentStyle)}{inherited && <span {...stylex.props(styles.tag)}>theo dự án</span>}</span><span className="mm-sub">Chạm để đổi · áp dụng từ chương sau</span></span><Icon name="arrow-down-2" size={16} /></button></div>
      {styleOpen && <>
        <div className="mm-group" role="radiogroup" aria-label="Theo dự án" style={{ marginTop: 8 }}><button type="button" role="radio" aria-checked={inherited} className={`mm-row ${stylex.props(styles.choice).className}`} disabled={!editable || pickStyle.busy} onClick={() => void pickStyle.run(null)}><Icon name="folder-open" size={20} /><span className="mm-copy"><span>Theo dự án · {styleDisplayName(presets.data, parentStyle)}</span><span className="mm-sub">Đổi theo khi dự án đổi</span></span>{inherited && <Icon name="tick-circle" size={20} {...stylex.props(styles.tick)} />}</button></div>
        <div className="mm-heading">Gói thể loại · có bộ luật, chấm từng chương</div>
        <div className="mm-group" role="radiogroup" aria-label="Gói thể loại">{packs.map((p) => {
          const on = !inherited && currentStyle === p.id
          const badge = packBadge(p.pack)
          return <button key={p.id} type="button" role="radio" aria-checked={on} className={`mm-row ${stylex.props(styles.choice).className}`} disabled={!editable || pickStyle.busy || p.pack.available === false} onClick={() => void pickStyle.run(p.id)}><span className="mm-copy"><span {...stylex.props(styles.choiceName)}>{p.label}<span {...stylex.props(styles.badge)}>{badge.label}</span></span><span className="mm-sub">{packDesc(p.pack)}</span></span>{on && <Icon name="tick-circle" size={20} {...stylex.props(styles.tick)} />}</button>
        })}</div>
        {activePack && <div style={{ margin: '8px 16px 0' }}><PackSummaryCard pack={activePack} /></div>}
        <div className="mm-heading">Kiểu viết · không chấm theo bộ luật</div>
        <div className="mm-group" role="radiogroup" aria-label="Kiểu viết">{choices.map((c) => {
          const on = !inherited && currentStyle === c.id
          return <button key={c.id || 'custom-empty'} type="button" role="radio" aria-checked={on} className={`mm-row ${stylex.props(styles.choice).className}`} disabled={!editable || pickStyle.busy || c.disabled} onClick={() => void pickStyle.run(c.id)}><span className="mm-copy"><span>{c.label}</span><span className="mm-sub">{c.desc}</span></span>{on && <Icon name="tick-circle" size={20} {...stylex.props(styles.tick)} />}</button>
        })}</div>
      </>}
    </>}
    {!!config.error && <p className="mm-error" role="alert">Không tải được văn phong. <button type="button" onClick={() => void config.refresh()}>Thử lại</button></p>}
    <div className="mm-heading">Quy tắc</div>
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
      const meta = groupMeta(level, projectLabel)
      if (!list.length && level !== scope) return null
      const own = isOwnLevel(level, scope)
      return <div key={level}>
        <div className="rm-ghead"><Icon name={meta.icon} size={14} style={{ color: meta.color }} /><span style={{ flex: '1 1 auto' }}>{level === scope ? 'Bạn đã dặn' : meta.label}</span>{!own && <button type="button" className="rm-text-btn" onClick={() => changeLevel(level as ConfigScope)}>{level === 'project' ? 'Sửa ở dự án ›' : 'Sửa ở Mọi truyện ›'}</button>}</div>
        {!own && <div className="rm-list">{list.map((entry) => <div key={entry.key} className="rm-rule" style={{ borderStyle: 'dashed', background: 'transparent', textAlign: 'left', alignItems: 'stretch' }}><span style={{ fontSize: 15, lineHeight: 1.5 }}>{entry.text}</span><span style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}><UChips entry={entry} className="rm-uch" /></span></div>)}</div>}
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
          <span>Mặc định của Noverse · chống giọng văn máy</span>
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>Luôn bật</span>
        </span>
      </div>
    </div>
    <div className="mm-heading">Khi nào có hiệu lực</div>
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
