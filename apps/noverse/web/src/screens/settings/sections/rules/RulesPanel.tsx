import { useEffect, useRef, useState } from 'react'
import type { ConfigScope } from '../../../../api/types'
import { navigate, paths } from '../../../../router'
import { Icon, Checkbox, Skeleton, toast } from '@noverse/ui'
import { SaveBar, dirtyText } from '../../savebar'
import { useProjectName, useStoryName } from '../../../../shell/useNames'
import { CHIP_STYLE, effectiveHint, learnedMeta, moveTarget, whenText, type RuleEntry } from './logic'
import { RetryCard, ThinkingCard, UnsureFrame } from './parts'
import { useRulesEditor } from './useRulesEditor'
import { AntiAiCard } from './AntiAiCard'
import './rules.css'
import './apRules.css'

const TEMPLATES = [['Không dùng từ…', 'Không dùng từ “”'], ['Giới hạn từ lặp…', '“” tối đa 2 lần mỗi chương'], ['Xưng hô…', 'Nhân vật  luôn xưng “” với '], ['Nhân vật…', ' luôn '], ['Giọng văn…', 'Giọng văn ']]
const LABELS = { ban: 'Cấm từ', chr: 'Cấm ký tự', max: 'Giới hạn', genre: 'Thể loại', pref: 'Lời dặn' }
export function RuleKind({ entry }: { entry: RuleEntry }) {
  const kind = entry.status === 'degraded' ? 'pref' : entry.chips[0]?.kind ?? 'pref'
  return <span className={`ap-kind ap-kind-${kind}`}>{LABELS[kind]}</span>
}
export type RulesPanelProps = {
  scope: ConfigScope; project?: string; story?: string
  onDirty?: (count: number, apply: () => Promise<string>, cancel: () => void, hint?: string) => void
}

/** List, anti-AI controls and timing share one pending edit transaction. */
export function RulesPanel({ scope, project = '', story = '', onDirty }: RulesPanelProps) {
  const editor = useRulesEditor({ scope, project, story })
  const projectName = useProjectName(project || null)
  const storyName = useStoryName(project || null, story || null)
  const [draft, setDraft] = useState('')
  const [under, setUnder] = useState(false)
  const [saving, setSaving] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const hint = scope === 'story' ? effectiveHint(editor.timing) : undefined
  const latest = useRef(editor); latest.current = editor
  const stable = useRef({ apply: () => latest.current.apply(scope === 'story' ? `truyện “${storyName}”` : `dự án “${projectName}”`), cancel: () => { latest.current.cancel(); setDraft('') } }).current
  useEffect(() => { onDirty?.(editor.count, stable.apply, stable.cancel, hint) }, [editor.count, hint])
  useEffect(() => () => onDirty?.(0, stable.apply, stable.cancel), [])
  useEffect(() => { if (editor.draftSeed) { setDraft(editor.draftSeed.text); input.current?.focus() } }, [editor.draftSeed?.n])
  const seed = (text: string) => { setDraft(text); input.current?.focus() }
  const submit = () => { if (!draft.trim() || !editor.canEdit || editor.thinking || editor.retry) return; const text = draft; setDraft(''); void editor.add(text) }
  const apply = async () => { setSaving(true); try { toast(await stable.apply()) } catch (error) { toast(error instanceof Error ? error.message : 'Không áp dụng được. Vui lòng thử lại.') } finally { setSaving(false) } }
  if (scope === 'global') return null
  const total = editor.lists[scope].length + (scope === 'story' ? editor.runtime.length + editor.lists.project.length : 0)
  const groups = scope === 'story' ? [scope, 'project'] as const : ['project'] as const
  return <div className="rl-root ap-rules" data-rules-level={scope}>
    <div className="ap-rules-heading"><Icon name="edit-2" size={17} style={{ color: 'var(--color-accent)' }} /><b>Quy tắc</b><span className="ap-meta num">· {total} đang áp dụng</span><span className="ap-spacer" /><button type="button" className="ap-button" onClick={() => setUnder(!under)} aria-expanded={under}><Icon name="magic-star" size={14} style={{ color: 'var(--color-accent)' }} />{under ? 'Ẩn bản tổng hợp' : 'Noverse hiểu thế nào?'}</button></div>
    <p className="ap-rules-intro">{scope === 'story' ? 'Dặn bằng lời thường, như dặn một biên tập viên. Quy tắc của truyện được ưu tiên hơn quy tắc của dự án.' : 'Dặn bằng lời thường. Áp dụng cho mọi truyện trong dự án, trừ chỗ một truyện dặn khác.'}</p>
    {under && <section className="card ap-under" aria-label="Noverse đang hiểu"><span className="ap-meta">{scope === 'story' ? 'Tổng hợp mọi quy tắc đang áp dụng cho truyện này, kể cả từ dự án và mặc định — đây là thứ trợ lý thật sự nhận được.' : 'Tổng hợp quy tắc của dự án và mặc định — mỗi truyện nhận thêm quy tắc riêng của nó.'}</span>{editor.summary.map((row) => <div key={row.kind} className="ap-under-row"><span style={{ width: 120, flexShrink: 0, color: CHIP_STYLE[row.kind].fg }}>{row.label}</span><span className="ap-ban-list">{row.items.map((text, index) => <span key={index} className="rl-uchip" style={{ background: CHIP_STYLE[row.kind].bg, color: CHIP_STYLE[row.kind].fg }}>{text}</span>)}</span></div>)}</section>}
    <div className="ap-addbox"><Icon name="add" size={16} /><input ref={input} id="rule-in" value={draft} disabled={!editor.canEdit} aria-label="Thêm quy tắc mới" placeholder={scope === 'story' ? 'Thêm quy tắc cho truyện này, ví dụ: Tri Hạ luôn gọi Nhược Vy là “cô”' : 'Thêm quy tắc cho cả dự án, ví dụ: không dùng biểu tượng cảm xúc trong lời thoại'} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.nativeEvent.isComposing) { event.preventDefault(); submit() } }} /><button type="button" className="ap-add" disabled={!draft.trim() || !editor.canEdit || !!editor.thinking || !!editor.retry} onClick={submit}>Thêm</button></div>
    <div className="ap-templates"><span className="ap-meta">Mẫu</span>{TEMPLATES.map(([label, text]) => <button key={label} type="button" className="ap-template" disabled={!editor.canEdit} onClick={() => seed(text)}>{label}</button>)}</div>
    {editor.resource.initialLoading && <Skeleton height={120} radius={14} />}
    {!!editor.resource.error && <div role="alert" className="ap-meta">Không tải được quy tắc. <button type="button" onClick={editor.refresh}>Thử lại</button></div>}
    {editor.thinking && <ThinkingCard text={editor.thinking} />}{editor.retry && <RetryCard retry={editor.retry} onSaveRaw={editor.saveRaw} onCancel={editor.cancelRetry} />}
    {scope === 'story' && editor.runtime.length > 0 && <section className="card ap-learned" aria-label="Học từ góp ý"><div className="ap-learned-head"><span className="rl-orb" aria-hidden="true" style={{ width: 16, height: 16 }} /><span><b>Học từ góp ý của bạn <span className="ap-meta">· {editor.runtime.length}</span></b><span className="ap-meta">Đang áp dụng. Giữ để thành quy tắc của truyện, hoặc bỏ nếu Noverse hiểu sai.</span></span></div>{editor.runtime.map((entry) => { const source = editor.resource.data?.runtime?.find((row) => row.id === entry.fromRuntime); return <div className="ap-rule-row" key={entry.key}><span className="ap-rule-text">{entry.text}{source && <span className="ap-meta ap-learned-meta">{learnedMeta(source)}</span>}</span><RuleKind entry={entry} /><span className="ap-learned-actions"><button type="button" className="ap-small" disabled={!editor.canEdit} onClick={() => editor.keepLearned(entry)}><Icon name="tick-circle" size={13} />Giữ</button><button type="button" className="ap-small" disabled={!editor.canEdit} onClick={() => editor.dropLearned(entry)}>Bỏ</button></span></div> })}</section>}
    <section className="card ap-rule-list" aria-label="Danh sách quy tắc">{groups.map((level) => { const inherited = level !== scope; const list = editor.lists[level]; const title = inherited ? `Từ dự án “${projectName}”` : scope === 'story' ? 'Của truyện này' : 'Của dự án'; return <div key={level} role="group" aria-label={title}><div className="ap-group-heading"><Icon name={inherited ? 'folder-open' : 'edit-2'} size={14} style={{ color: inherited ? 'var(--kit-info-fg)' : 'var(--color-accent)' }} /><b>{title}</b><span className="num">{list.length}</span><span className="ap-spacer" />{inherited && <button type="button" className="ap-link" onClick={() => { if (editor.count) { toast('Áp dụng hoặc Huỷ thay đổi trước khi chuyển cấp.'); return } navigate(`${paths.project(project)}?section=prules`) }}>Sửa ở trang dự án ›</button>}</div>{list.length === 0 && <div className="ap-rule-row ap-meta">{inherited ? 'Dự án chưa có quy tắc chung.' : scope === 'story' ? 'Chưa dặn riêng. Truyện đang theo quy tắc của dự án bên dưới.' : 'Chưa có quy tắc nào. Thêm ở ô trên.'}</div>}{list.map((entry) => <div key={entry.key} className={`ap-rule-row${inherited ? ' ap-inherited' : ''}${entry.fresh ? ' new' : ''}`}><span className="ap-rule-text">{entry.text}</span><RuleKind entry={entry} /><span className="ap-actions">{!inherited && editor.canEdit && <><button type="button" aria-label="Sửa quy tắc" title="Sửa" onClick={() => editor.edit(level, entry.key)}><Icon name="edit-2" size={15} /></button>{moveTarget(scope) && <button type="button" aria-label="Chuyển lên dự án" title="Chuyển lên dự án" onClick={() => editor.move(level, entry.key)}><Icon name="arrow-up" size={15} /></button>}<button type="button" aria-label="Xoá quy tắc" title="Xoá" onClick={() => editor.remove(level, entry.key)}><Icon name="trash" size={15} /></button></>}</span><div className="ap-uncertain"><UnsureFrame entry={entry} busy={!editor.canEdit || !!editor.resolving} readOnly={inherited || !editor.canEdit} onPick={(option) => { void editor.resolve(level, entry.key, option) }} /></div></div>)}</div> })}</section>
    <AntiAiCard editor={editor} onAddPhrase={() => seed('Không dùng cụm “”')} />
    {scope === 'story' && <section className="card ap-timing"><b><Icon name="clock" size={16} style={{ color: 'var(--color-accent)' }} />Khi nào có hiệu lực</b><span>{whenText(scope, editor.timing)}</span><Checkbox className="ap-recheck" checked={editor.recheck} onChange={editor.setRecheck} style={{ alignItems: 'flex-start', margin: 0, padding: '2px 0 0', color: 'var(--color-fg-2)', lineHeight: 1.5 }} label={`Kiểm tra lại ${editor.timing?.completed || 'các'} chương đã viết theo cách mới và báo chỗ lệch (không tự sửa)`} /></section>}
    {!onDirty && editor.count > 0 && <SaveBar text={dirtyText(editor.count, scope, hint)} applyLabel="Áp dụng" saving={saving} disabled={saving || !editor.canEdit} onApply={() => void apply()} onCancel={stable.cancel} />}
  </div>
}
