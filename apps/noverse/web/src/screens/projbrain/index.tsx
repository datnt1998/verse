import { Fragment, useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { canReadStory, canWriteStory, checkProjectLedger, getProjectLedger, getProjectUniverse, searchProjectBrain, suggestProjectLedger, unkeepReviewItem } from '../../api/client'
import { useSession } from '../../api/session'
import type { LedgerCheck, LedgerCheckResult, ProjectSearch, ProjectUniverse, ProjectUniverseEntity, UniverseCheckItem, UniverseKeptItem } from '../../api/types'
import { keys, useResource } from '../../data'
import { t } from '../../i18n/vi'
import { paths, navigate, useRoute } from '../../router'
import { useProjectName } from '../../shell/useNames'
import { Icon } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { styles as s } from './styles'
import { CharAvatar } from '../../ui/CharAvatar'

const copy = t.projbrain
const levelStyle = { error: s.errorChip, warning: s.warningChip, ok: s.okChip }
/** Icon + màu theo loại thực thể, cùng bảng với Não truyện (artboard: nhân vật tím, đồ vật xanh). */
const entityIcon: Record<ProjectUniverseEntity['type'], string> = { char: 'user', place: 'global', obj: 'box', ev: 'clock', fs: 'flag', rule: 'shield-security' }
const entityIconStyle = { char: s.iconChar, place: s.iconPlace, obj: s.iconObj, ev: s.iconEv, fs: s.iconFs, rule: s.iconRule }
function Level({ level }: { level: LedgerCheck['level'] }) {
  return <span {...stylex.props(s.chip, levelStyle[level])}>{copy.level[level]}</span>
}
const storySlug = (project: string, id: string) => id.startsWith(`${project}/`) ? id.slice(project.length + 1) : id
const formatDay7 = (n: number | null) => n === null ? '—' : n.toLocaleString('vi-VN', { notation: 'compact', maximumFractionDigits: 1 })
/** RFC3339 → `dd/mm`; thiếu hoặc không đọc được → '' (dòng lý do bỏ đoạn đó, §J câu 1). */
const dayMonth = (iso: string) => {
  const date = new Date(iso)
  return iso && !Number.isNaN(date.getTime()) ? `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}` : ''
}
/** `stylex.props` trả `className` nên spread nó sẽ xoá lớp tiện ích `.hov-ghost` (và ngược lại) — gộp cả hai. */
const withGhost = (props: { className?: string; style?: object }) => ({ ...props, className: `hov-ghost${props.className ? ` ${props.className}` : ''}` })
/** Như `withGhost` nhưng cho hàng kết quả tìm (`.hov-row` = nền 4% như artboard). */
const withRow = (props: { className?: string; style?: object }) => ({ ...props, className: `hov-row${props.className ? ` ${props.className}` : ''}` })

export function ProjectBrainScreen({ project }: { project: string }) {
  const parsed = useRoute()
  const tab = parsed.search.get('tab') ?? 'ledger'
  const { user } = useSession()
  const projectName = useProjectName(project)
  const ledger = useResource(keys.project(project, 'stories', 'ledger'), ({ signal }) => getProjectLedger(project, signal))
  const universe = useResource(keys.project(project, 'stories', 'universe'), ({ signal }) => getProjectUniverse(project, signal))
  const storyId = (id: string) => id.includes('/') ? id : `${project}/${id}`
  const visible = (id: string) => !!user && canReadStory(user, storyId(id))
  const stories = (ledger.data?.stories ?? []).filter(row => visible(row.id))
  const candidate = ledger.data?.candidate && visible(ledger.data.candidate.story) ? ledger.data.candidate : null
  const [replacement, setReplacement] = useState<(LedgerCheckResult & { brief: string; sourceBrief: string; sourceStory: string }) | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [q, setQ] = useState(parsed.search.get('q') ?? '')
  const [showKept, setShowKept] = useState(false)
  const [everKept, setEverKept] = useState(false)
  const [unkeeping, setUnkeeping] = useState(false)
  const results = useResource(keys.project(project, 'stories', 'search', q.trim()), ({ signal }) => searchProjectBrain(project, q.trim(), signal), { enabled: tab === 'search' && !!q.trim() })
  const activeReplacement = candidate && replacement?.sourceStory === candidate.story && replacement.sourceBrief === candidate.brief ? replacement : null
  const activeCandidate = candidate && activeReplacement ? { ...candidate, ...activeReplacement } : candidate
  const checkRows = activeCandidate?.checks ?? []
  const marks = activeCandidate?.cells ?? []
  const entities = (universe.data?.entities ?? []).map(entity => ({ ...entity, stories: entity.stories.filter(story => visible(story.id)) })).filter(entity => entity.stories.length > 0)
  // §J câu 1: mục đã “Giữ nguyên” bị lọc khỏi `checks`; dòng nhỏ nằm ngay dưới thẻ nhắc cuối ở cột phải.
  // `everKept` giữ dòng lại sau khi bỏ giữ mục cuối để câu “Không còn mục nào…” hiện được; 0 từ đầu thì ẩn.
  const keptCount = universe.data?.keptCount ?? 0
  const keptItems = (universe.data?.kept ?? []).filter(item => visible(item.story))
  const keptExpanded = showKept && keptItems.length > 0
  useEffect(() => { if (keptCount > 0) setEverKept(true) }, [keptCount])
  const filteredSearch: ProjectSearch = {
    total: results.data?.total ?? 0,
    entities: (results.data?.entities ?? []).map(entity => ({ ...entity, stories: entity.stories.filter(story => visible(story.id)) })).filter(entity => entity.stories.length > 0),
    passages: (results.data?.passages ?? []).filter(hit => visible(hit.story)),
  }
  const searchStories = new Set([...filteredSearch.entities.flatMap(entity => entity.stories.map(story => story.id)), ...filteredSearch.passages.map(hit => hit.story)]).size
  const pickTab = (value: string) => {
    const search = new URLSearchParams(parsed.search); search.set('tab', value)
    navigate(`${paths.projectBrain(project)}?${search}`)
  }
  const suggest = async () => {
    if (!candidate || busy) return
    if (activeReplacement) { setReplacement(null); return }
    setBusy(true); setError('')
    try {
      const suggestion = await suggestProjectLedger(project, candidate.brief)
      const checked = await checkProjectLedger(project, suggestion.brief)
      setReplacement({ ...checked, values: suggestion.values, brief: suggestion.brief, sourceBrief: candidate.brief, sourceStory: candidate.story })
    } catch (cause) { setError(cause instanceof Error ? cause.message : copy.error) }
    finally { setBusy(false) }
  }
  const unkeep = async (check: UniverseCheckItem) => {
    if (unkeeping) return
    setUnkeeping(true); setError('')
    try {
      // GAP-47: cùng hành động “Bỏ giữ” của Trình đọc; xong thì tải lại vũ trụ để mục về `checks`.
      await unkeepReviewItem(project, storySlug(project, check.story), check.quoteId)
      await universe.refresh()
    } catch (cause) { setError(cause instanceof Error ? cause.message : t.review.unkeepFailed) }
    finally { setUnkeeping(false) }
  }
  const markFor = (story: string, axis: number, isCandidate: boolean) => marks.find(cell => cell.axis === axis && (cell.story === story || (isCandidate && cell.story === 'new')))?.mark
  const rows = activeCandidate ? [{ id: activeCandidate.story, title: activeCandidate.title, values: activeCandidate.values, day7: null, createdAt: null, opening: '', candidate: true }, ...stories.filter(row => row.id !== activeCandidate.story).map(row => ({ ...row, candidate: false }))] : stories.map(row => ({ ...row, candidate: false }))
  // §H câu 5: chưa có nguồn Ngày 7 → mọi dòng null thì ẩn cả cột lẫn dòng chú thích trung vị.
  const showDay7 = rows.some(row => row.day7 !== null)

  return <main {...stylex.props(s.root)} aria-label={copy.title}>
    <div {...stylex.props(s.heading)}>
      <div {...stylex.props(s.intro)}>
        <a {...stylex.props(s.back)} style={{ color: 'var(--kit-fg-icon)' }} href={`#${paths.project(project)}`}><Icon name="arrow-left-2" size={13} />{projectName} · {copy.stories(stories.length)}</a>
        <h1 {...stylex.props(s.title)}>{copy.title}</h1>
        <span {...stylex.props(s.sub)}>{copy.description}</span>
      </div>
      <Segmented label={copy.section} value={tab} onChange={pickTab}
        tabs={{ id: 'projbrain-tab', panel: 'projbrain-body' }}
        style={{ flexShrink: 0, gap: 0, background: 'var(--color-card)', borderColor: 'var(--line-2)' }}
        thumbStyle={{ background: 'var(--projbrain-seg-on)', boxShadow: 'none' }}
        options={[
        { id: 'ledger', icon: <Icon name="category" size={14} />, style: { flex: '0 0 auto', padding: '0 11px', fontWeight: tab === 'ledger' ? 500 : 400 }, label: <span style={{ display: 'inline-flex', gap: 6 }}>{copy.ledger}<span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{stories.length}</span></span> },
        { id: 'universe', icon: <Icon name="global" size={14} />, style: { flex: '0 0 auto', padding: '0 11px', fontWeight: tab === 'universe' ? 500 : 400 }, label: <span style={{ display: 'inline-flex', gap: 6 }}>{copy.universe}<span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{entities.length}</span></span> },
        { id: 'search', icon: <Icon name="search-normal-1" size={14} />, style: { flex: '0 0 auto', padding: '0 11px', fontWeight: tab === 'search' ? 500 : 400 }, label: <span style={{ display: 'inline-flex', gap: 6 }}>{copy.search}<span /></span> },
      ]} />
    </div>
    {error && <div role="alert" {...stylex.props(s.body)}>{error}</div>}
    {!!ledger.error && <div role="alert">{copy.error} <button type="button" {...stylex.props(s.button)} onClick={() => void ledger.refresh()}>{copy.retry}</button></div>}
    {!ledger.data && !ledger.error && <span {...stylex.props(s.body)}>{copy.loading}</span>}
    <div role="tabpanel" id="projbrain-body" aria-labelledby={`projbrain-tab-${tab}`} {...stylex.props(s.panel)}>
    {tab === 'ledger' && <>
      {activeCandidate && <section aria-label={copy.checkLabel} {...stylex.props(s.card, s.padded, checkRows.some(check => check.level === 'error') ? s.blocked : s.passed)}>
        <div {...stylex.props(s.row)}><span {...stylex.props(s.strong)}>{copy.newStory(activeCandidate.title)}</span><span {...stylex.props(s.chip, s.draftChip)}>{copy.draft}</span><span {...stylex.props(s.grow)} /><span {...stylex.props(s.note)}>{copy.comparison}</span></div>
        {checkRows.map(check => <div key={check.rule} data-ledger-check={check.rule} {...stylex.props(s.check)}><Level level={check.level} /><span {...stylex.props(s.grow)}>{check.text}</span>{check.fix !== null && <span data-check-fix {...stylex.props(s.checkFix)}>{check.fix}</span>}</div>)}
        <div {...stylex.props(s.actions)}><button type="button" {...withGhost(stylex.props(s.button))} disabled={busy} aria-busy={busy} onClick={() => void suggest()}><Icon name="magic-star" size={14} />{activeReplacement ? copy.undo : copy.suggest}</button><a {...withGhost(stylex.props(s.button))} href={`#${paths.newstory(project)}?brief=${encodeURIComponent(activeCandidate.brief)}`}><Icon name="edit-2" size={14} />{copy.edit}</a></div>
      </section>}
      {ledger.data && <section aria-label={copy.ledger} {...stylex.props(s.card, s.table)}>
        <div {...stylex.props(s.ledgerRow, !showDay7 && s.ledgerRowNoDay7, s.tableHead)}><span>{copy.story}</span>{ledger.data.axes.map(axis => <span key={axis.n} title={axis.name} {...stylex.props(s.axis)}><span {...stylex.props(s.axisNumber)}>{axis.n}</span><span>{copy.axisLabels[axis.n - 1] ?? axis.name}</span></span>)}{showDay7 && <span {...stylex.props(s.number)}>{copy.day7}</span>}</div>
        {rows.map(row => { const added = row.createdAt ? dayMonth(row.createdAt) : ''; return <div key={row.id} data-story={row.id} {...stylex.props(s.ledgerRow, !showDay7 && s.ledgerRowNoDay7, row.candidate && s.candidateRow)}>
          <span {...stylex.props(s.storyCell)}><span title={row.title} {...stylex.props(s.storyTitle, row.candidate && s.storyTitleDraft)}>{row.title}</span>{row.candidate ? <span {...stylex.props(s.briefMeta)}>{copy.draftMeta}</span> : added ? <span {...stylex.props(s.rowMeta)}>{added}</span> : null}</span>
          {ledger.data!.axes.map((axis, index) => { const mark = markFor(row.id, axis.n, row.candidate); return <span key={axis.n} title={row.values[index] || '—'} data-mark={mark} {...stylex.props(s.cell, row.candidate && s.candidateCell, mark === 'dup' && s.duplicate, mark === 'overused' && s.overused)}><span data-cell-text="" {...stylex.props(s.cellText)}>{row.values[index] || '—'}</span></span> })}
          {showDay7 && <span {...stylex.props(s.number)}>{formatDay7(row.day7)}</span>}
        </div> })}
        {!rows.length && <div {...stylex.props(s.padded, s.body)}>{copy.empty}</div>}
      </section>}
      <div {...stylex.props(s.legend)}><span {...stylex.props(s.legendItem)}><span {...stylex.props(s.cell, s.duplicate)}>{copy.value}</span>{copy.duplicate}</span><span {...stylex.props(s.legendItem)}><span {...stylex.props(s.cell, s.overused)}>{copy.value}</span>{copy.overused}</span>{showDay7 && <span>{copy.median(formatDay7(ledger.data?.channelMedianDay7 ?? null))}</span>}</div>
    </>}
    {tab === 'universe' && <>
      {!!universe.error && <div role="alert">{copy.error} <button type="button" {...stylex.props(s.button)} onClick={() => void universe.refresh()}>{copy.retry}</button></div>}
      <div {...stylex.props(s.universe)}><section aria-label={copy.universeLabel} {...stylex.props(s.card)}>
        <div {...stylex.props(s.universeRow, s.tableHead)}><span>{copy.entity}</span><span>{copy.appears}</span><span>{copy.role}</span><span {...stylex.props(s.checkCol)}>{copy.check}</span></div>
        {entities.map(entity => <div key={entity.id} {...stylex.props(s.universeRow)}>
          <div {...stylex.props(s.entity)}>{entity.type === 'char'
            ? <CharAvatar name={entity.name} kind={entity.kind} size={32} />
            : <span {...stylex.props(s.avatar, s.place)}><Icon name="global" size={15} /></span>}<span {...stylex.props(s.nameCell)}><span title={entity.name} {...stylex.props(s.entityName)}>{entity.name}</span><span {...stylex.props(s.note)}>{copy.types[entity.type]}</span></span></div>
          <span {...stylex.props(s.storyChips)}>{entity.stories.map(story => <span key={story.id} title={story.title} {...stylex.props(s.chip, s.neutralChip)}>{story.title}</span>)}</span>
          <span title={entity.lastRole} {...stylex.props(s.role)}>{entity.lastRole || '—'}</span>
          <span {...stylex.props(s.checkCol)}><UniverseCheck checks={universe.data?.checks ?? null} stories={entity.stories} /></span>
        </div>)}
        {!entities.length && <span {...stylex.props(s.padded, s.body)}>{copy.empty}</span>}
      </section><aside {...stylex.props(s.stack)}><UniverseSummary name={universe.data?.name ?? ''} counts={universe.data?.counts ?? { stories: 0, chars: 0, places: 0 }} />
        {(universe.data?.checks ?? []).filter(check => visible(check.story)).map((check, index) => <RecapCard key={index} project={project} check={check} />)}
        {/* §J câu 1 — dòng nhỏ ngay dưới thẻ nhắc cuối: mở danh sách mục đã “Giữ nguyên”. */}
        {(keptCount > 0 || everKept) && <>
          <button type="button" data-universe-kept="" aria-expanded={keptExpanded} aria-controls={keptExpanded ? 'universe-kept' : undefined} onClick={() => setShowKept(open => !open)} {...withGhost(stylex.props(s.keptLine))}><Icon name="tick-circle" size={13} />{keptCount > 0 ? copy.kept(keptCount) : copy.keptNone}<span data-kept-chevron="" style={{ display: 'inline-flex' }}><Icon name="arrow-down-2" size={12} style={{ transform: keptExpanded ? 'rotate(180deg)' : 'none', transition: 'transform .15s' }} /></span></button>
          {keptExpanded && <div id="universe-kept" {...stylex.props(s.keptList)}>{keptItems.map((item, index) => <RecapCard key={index} project={project} check={item} kept onUnkeep={user && canWriteStory(user, storyId(item.story)) ? unkeep : undefined} busy={unkeeping} />)}</div>}
        </>}
      </aside></div>
    </>}
    {tab === 'search' && <>
      <label {...(() => { const p = stylex.props(s.searchField); return { className: `kit-frame ${p.className}`, style: p.style } })()}><Icon name="search-normal-1" size={17} /><input type="search" value={q} onChange={event => setQ(event.target.value)} aria-label={copy.searchLabel} {...stylex.props(s.input)} /><span {...stylex.props(s.searchCount)}>{copy.results(searchStories, filteredSearch.entities.length, new Set(filteredSearch.passages.map(hit => hit.story)).size)}</span></label>
      {!!results.error && !!q.trim() && <div role="alert">{copy.error} <button type="button" {...stylex.props(s.button)} onClick={() => void results.refresh()}>{copy.retry}</button></div>}
      <div {...stylex.props(s.searchGrid)}><section aria-label={copy.entityLabel} {...stylex.props(s.card, s.entityCard)}><span {...stylex.props(s.label)}>{copy.entity}</span>{filteredSearch.entities.map((entity, index) => <div key={entity.id} {...stylex.props(s.entityHit, index > 0 && s.entityDivider)}><span {...stylex.props(s.entityRow)}><Icon name={entityIcon[entity.type]} size={15} {...stylex.props(s.entityIcon, entityIconStyle[entity.type])} /><span title={entity.name} {...stylex.props(s.entityNameLoose)}>{entity.name}</span></span><span {...stylex.props(s.entityWhere)}>{entity.stories.map(story => story.title).join(' · ')}</span><span {...stylex.props(s.entityRel)}>{entity.lastRole}</span></div>)}</section>
        <section aria-label={copy.passagesLabel} {...stylex.props(s.card, s.passageCard)}><span {...stylex.props(s.label, s.labelPassages)}>{copy.passages}</span>{filteredSearch.passages.map((hit, index) => <a key={index} {...withRow(stylex.props(s.passageHit, index > 0 && s.passageDivider))} href={`#${paths.reader(project, storySlug(project, hit.story), hit.ch)}`}><span><span {...stylex.props(s.passageStory)}>{hit.storyTitle}</span><span {...stylex.props(s.passageChapter)}> · {copy.chapter(hit.ch)}</span></span><span {...stylex.props(s.quote)}><Highlighted quote={hit.quote} ranges={hit.hl} /></span></a>)}<span {...stylex.props(s.resultsNote)}>{copy.allVisible}</span></section>
      </div>
    </>}
    </div>
  </main>
}

export function Highlighted({ quote, ranges }: { quote: string; ranges: [number, number][] }) {
  let end = 0
  return <>{ranges.map(([from, to], index) => { const before = quote.slice(end, from); end = to; return <Fragment key={index}>{before}<mark {...stylex.props(s.mark)}>{quote.slice(from, to)}</mark></Fragment> })}{quote.slice(end)}</>
}

/**
 * Thẻ phát hiện Master §19 — dùng cho cột phải Vũ trụ chung và danh sách mục đã “Giữ nguyên”.
 * Biến thể *đã giữ* (§J câu 1): ✓ xám, chip “Đã giữ nguyên”, dòng lý do, nút “Bỏ giữ”.
 */
/**
 * `check` là mục Master §19 thường; biến thể đã giữ mang thêm quyết định GAP-47 —
 * cùng một thẻ vẽ cả hai nên quyết định là phần tuỳ chọn của mục (checks[] không có).
 */
type RecapCheck = UniverseCheckItem & Partial<UniverseKeptItem>

export function RecapCard({ project, check, kept = false, onUnkeep, busy = false }: { project: string; check: RecapCheck; kept?: boolean; onUnkeep?: (check: RecapCheck) => void; busy?: boolean }) {
  // Đoạn rỗng thì bỏ; cả ba rỗng thì không vẽ dòng (bản ghi GAP-47 cũ có thể thiếu trường).
  const who = [check.decidedBy ?? '', dayMonth(check.decidedAt ?? '')].filter(Boolean).join(', ')
  const reason = [check.reason ? copy.reason(check.reason) : '', who].filter(Boolean).join(' · ')
  const openPassage = <a href={`#${paths.reader(project, storySlug(project, check.story), check.ch)}?quote=${encodeURIComponent(check.quoteId)}`} {...stylex.props(s.openLink)}>{copy.openPassage}</a>
  return <section data-kept={kept ? '' : undefined} {...stylex.props(s.card, kept ? s.keptCard : s.universeCard, !kept && s.recapCard)}>
    <span {...stylex.props(kept ? s.keptHead : s.strong, !kept && s.recapTitle)}><Icon name={kept ? 'tick-circle-bulk' : 'warning-2'} size={kept ? 15 : 16} style={{ color: kept ? 'var(--kit-fg-icon)' : 'var(--color-warn)' }} /><span {...stylex.props(s.grow)}>{copy.recapTitle}</span>{kept && <span {...stylex.props(s.chip, s.keptChip)}>{copy.keptBadge}</span>}</span>
    <span {...stylex.props(kept ? s.keptText : s.body)}>{check.text}</span>
    {kept && reason ? <span data-kept-reason="" {...stylex.props(s.keptReason)}>{reason}</span> : null}
    {kept
      ? <span {...stylex.props(s.keptFoot)}>{openPassage}<span {...stylex.props(s.grow)} />{onUnkeep && <button type="button" disabled={busy} aria-busy={busy} onClick={() => onUnkeep(check)} {...withGhost(stylex.props(s.button, s.unkeep))}>{t.review.unkeep}</button>}</span>
      : openPassage}
  </section>
}

/** Thẻ tóm tắt cột phải Vũ trụ chung. §H câu 3: tên rỗng (dự án ẩn danh) thì ẩn hẳn dòng tên. */
export function UniverseSummary({ name, counts }: { name: string; counts: ProjectUniverse['counts'] }) {
  return <section {...stylex.props(s.card, s.universeCard)}>
    {name ? <span {...stylex.props(s.strong)}>{name}</span> : null}
    <span {...stylex.props(s.body)}>{copy.universeSummary(counts.stories, counts.chars, counts.places)}</span>
    <div {...stylex.props(s.universeRules)}>
      <div {...stylex.props(s.universeRule)}><Level level="error" /><span>{copy.standalone}</span></div>
      <div {...stylex.props(s.universeRule)}><Level level="warning" /><span>{copy.recap}</span></div>
    </div>
  </section>
}

export function UniverseCheck({ checks, stories }: { checks: ProjectUniverse['checks']; stories: ProjectUniverseEntity['stories'] }) {
  // §H câu 4: chưa có vòng chấm nào phát `rule` → chưa kiểm được; đã chấm mà sạch → Đạt.
  if (checks === null) return <span title={copy.notChecked} {...stylex.props(s.note)}>—</span>
  const found = checks.some(check => stories.some(story => story.id === check.story))
  if (!found) return <Level level="ok" />
  return <span {...stylex.props(s.chip, s.warningChip)}>{copy.recapBadge}</span>
}
