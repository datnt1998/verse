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
import { apCopy as ap } from './copy'
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
  const [lview, setLview] = useState(parsed.search.get('lview') === 'freq' ? 'freq' : 'table')
  const [only, setOnly] = useState(parsed.search.get('lview') === 'only')
  const [pick, setPick] = useState<{ axis: number; value: string } | null>(parsed.search.get('lview') === 'pick' ? { axis: 2, value: 'Thợ may' } : null)
  const [entityType, setEntityType] = useState('all')
  const [needOnly, setNeedOnly] = useState(false)
  const [entityQuery, setEntityQuery] = useState('')
  const [searchKind, setSearchKind] = useState(parsed.search.get('sfilter') ?? 'all')
  const [searchStory, setSearchStory] = useState<string | null>(null)
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
  const searchRefs = new Map<string, string>()
  for (const entity of filteredSearch.entities) for (const story of entity.stories) searchRefs.set(storySlug(project, story.id), story.title)
  for (const hit of filteredSearch.passages) searchRefs.set(storySlug(project, hit.story), hit.storyTitle)
  const searchStories = searchRefs.size
  const searchCount = (id: string | null) => filteredSearch.entities.filter(entity => !id || entity.stories.some(story => storySlug(project, story.id) === id)).length + filteredSearch.passages.filter(hit => !id || storySlug(project, hit.story) === id).length
  const searchEntities = filteredSearch.entities.filter(entity => !searchStory || entity.stories.some(story => storySlug(project, story.id) === searchStory))
  const searchPassages = filteredSearch.passages.filter(hit => !searchStory || storySlug(project, hit.story) === searchStory)
  const checks = universe.data?.checks ?? null
  const needsReview = (entity: ProjectUniverseEntity) => !!checks?.some(check => entity.stories.some(story => storySlug(project, story.id) === storySlug(project, check.story)))
  const shownEntities = entities.filter(entity => (entityType === 'all' || entity.type === entityType) && (!needOnly || needsReview(entity)) && entity.name.toLocaleLowerCase('vi').includes(entityQuery.trim().toLocaleLowerCase('vi')))
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
  const prior = stories.filter(row => row.id !== activeCandidate?.story)
  const recent = prior.slice(0, 10)
  const same = (a: string, b: string) => !!a.trim() && a.trim().replace(/\s+/g, ' ').toLocaleLowerCase('vi') === b.trim().replace(/\s+/g, ' ').toLocaleLowerCase('vi')
  const useCount = (axis: number, value: string) => recent.filter(row => same(row.values[axis] ?? '', value)).length
  const isPicked = (axis: number, value: string) => pick?.axis === axis && same(pick.value, value)
  const shownPrior = only && activeCandidate ? prior.filter(row => row.values.some((value, axis) => same(value, activeCandidate.values[axis] ?? ''))) : prior
  const rows = activeCandidate ? [{ id: activeCandidate.story, title: activeCandidate.title, values: activeCandidate.values, createdAt: null, candidate: true }, ...shownPrior.map(row => ({ ...row, candidate: false }))] : shownPrior.map(row => ({ ...row, candidate: false }))
  const errors = checkRows.filter(check => check.level === 'error').length
  const warnings = checkRows.filter(check => check.level === 'warning').length
  const frequency = (ledger.data?.axes ?? []).map((axis, index) => {
    const counts = new Map<string, number>()
    for (const row of recent) {
      const value = row.values[index]
      if (!value?.trim()) continue
      let label = value
      for (const existing of counts.keys()) if (same(existing, value)) { label = existing; break }
      counts.set(label, (counts.get(label) ?? 0) + 1)
    }
    const chosen = activeCandidate?.values[index]
    if (chosen && useCount(index, chosen) === 0) counts.set(chosen, 0)
    return { ...axis, index, limited: [3, 4, 7].includes(axis.n), note: [[3, 4, 7].includes(axis.n) ? ap.limit : '', [1, 3, 4, 6].includes(axis.n) ? ap.combination : '', axis.n === 9 ? ap.opening : ''].filter(Boolean).join(' · '), values: [...counts].sort(([a, an], [b, bn]) => bn - an || a.localeCompare(b, 'vi')) }
  })

  return <main {...stylex.props(s.root)} aria-label={copy.title}>
    <div {...stylex.props(s.heading)}>
      <div {...stylex.props(s.intro)}>
        <a {...stylex.props(s.back)} style={{ color: 'var(--kit-fg-icon)' }} href={`#${paths.project(project)}`}><Icon name="arrow-left-2" size={13} />{projectName} · {copy.stories(stories.length)}</a>
        <h1 {...stylex.props(s.title)}>{copy.title}</h1>
        <span {...stylex.props(s.sub)}>{copy.description}</span>
      </div>
      <Segmented label={copy.section} value={tab} onChange={pickTab}
        tabs={{ id: 'projbrain-tab', panel: 'projbrain-body' }}
        style={{ flexShrink: 0, maxWidth: '100%', gap: 0, background: 'var(--color-card)', borderColor: 'var(--line-2)' }}
        thumbStyle={{ background: 'var(--projbrain-seg-on)', boxShadow: 'none' }}
        options={[
        { id: 'ledger', icon: <Icon name="category" size={14} />, style: { flex: '0 1 auto', minWidth: 0, padding: '0 11px', fontWeight: tab === 'ledger' ? 500 : 400 }, label: <span style={{ display: 'inline-flex', gap: 6 }}>{copy.ledger}<span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{stories.length}</span></span> },
        { id: 'universe', icon: <Icon name="global" size={14} />, style: { flex: '0 1 auto', minWidth: 0, padding: '0 11px', fontWeight: tab === 'universe' ? 500 : 400 }, label: <span style={{ display: 'inline-flex', gap: 6 }}>{copy.universe}<span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{entities.length}</span></span> },
        { id: 'search', icon: <Icon name="search-normal-1" size={14} />, style: { flex: '0 1 auto', minWidth: 0, padding: '0 11px', fontWeight: tab === 'search' ? 500 : 400 }, label: <span style={{ display: 'inline-flex', gap: 6 }}>{copy.search}<span /></span> },
      ]} />
    </div>
    {error && <div role="alert" {...stylex.props(s.body)}>{error}</div>}
    {!!ledger.error && <div role="alert">{copy.error} <button type="button" {...stylex.props(s.button)} onClick={() => void ledger.refresh()}>{copy.retry}</button></div>}
    {!ledger.data && !ledger.error && <span {...stylex.props(s.body)}>{copy.loading}</span>}
    <div role="tabpanel" id="projbrain-body" aria-labelledby={`projbrain-tab-${tab}`} {...stylex.props(s.panel)}>
    {tab === 'ledger' && <>
      {activeCandidate && <section aria-label={ap.check} {...stylex.props(s.card, errors ? s.blocked : warnings ? s.warned : s.passed)}>
        <div {...stylex.props(s.checkHeader)}><span {...stylex.props(s.checkIntro)}><span {...stylex.props(s.note)}>{ap.comparison}</span><span {...stylex.props(s.row)}><b {...stylex.props(s.newTitle)}>“{activeCandidate.title}”</b><span {...stylex.props(s.chip, s.draftChip)}>{copy.draft}</span></span></span><span role="status" {...stylex.props(s.verdict, errors ? s.verdictError : warnings ? s.verdictWarning : s.verdictOk)}><Icon name={errors ? 'danger' : warnings ? 'warning-2' : 'tick-circle'} size={15} />{ap.verdict(errors, warnings)}</span></div>
        {checkRows.map(check => <div key={check.rule} data-ledger-check={check.rule} {...stylex.props(s.check)}><span {...stylex.props(s.checkName)}>{ap.checkNames[check.rule] ?? check.rule}</span><span><Level level={check.level} /></span><span {...stylex.props(s.checkText)}>{check.text}</span>{check.fix !== null && <span data-check-fix {...stylex.props(s.checkFix)}>{check.fix}</span>}</div>)}
        <div {...stylex.props(s.actions)}><button type="button" {...withGhost(stylex.props(s.button))} disabled={busy} aria-busy={busy} onClick={() => void suggest()}><Icon name="magic-star" size={14} />{activeReplacement ? copy.undo : copy.suggest}</button><a {...withGhost(stylex.props(s.button))} href={`#${paths.newstory(project)}?brief=${encodeURIComponent(activeCandidate.brief)}`}><Icon name="edit-2" size={14} />{copy.edit}</a></div>
      </section>}
      <div {...stylex.props(s.toolbar)}>
        <Segmented label={ap.view} fit="content" value={lview} onChange={value => { setLview(value); setPick(null) }} style={{ gap: 0, background: 'var(--color-card)', borderColor: 'var(--line-2)' }} thumbStyle={{ background: 'var(--projbrain-seg-on)', boxShadow: 'none' }} options={[{ id: 'table', label: ap.table, style: { padding: '0 11px', fontWeight: lview === 'table' ? 500 : 400 } }, { id: 'freq', label: ap.frequency, style: { padding: '0 11px', fontWeight: lview === 'freq' ? 500 : 400 } }]} />
        {lview === 'table' && <button type="button" aria-pressed={only} onClick={() => setOnly(value => !value)} {...stylex.props(s.filterChip, only && s.filterOn)}><Icon name="tick-circle" size={13} style={{ opacity: only ? 1 : .35 }} />{ap.only}</button>}
        <span {...stylex.props(s.grow)} /><span {...stylex.props(s.note)}>{ap.ledgerCount(prior.length, shownPrior.length, only)}</span>
      </div>
      {pick && lview === 'table' && <div role="status" {...stylex.props(s.pickbar)}><Icon name="search-normal-1" size={14} /><span {...stylex.props(s.grow)}>{ap.pick(pick.value, ledger.data?.axes[pick.axis]?.name ?? '', useCount(pick.axis, pick.value), same(activeCandidate?.values[pick.axis] ?? '', pick.value))}</span><button type="button" onClick={() => setPick(null)} {...withGhost(stylex.props(s.button, s.clearPick))}>{ap.clear}</button></div>}
      {lview === 'table' && <>
        {ledger.data && <section aria-label={copy.ledger} {...stylex.props(s.card, s.table)}>
          <div {...stylex.props(s.ledgerRow, s.tableHead, s.stickyHead)}><span>{copy.story}</span>{ledger.data.axes.map(axis => <span key={axis.n} title={axis.name} {...stylex.props(s.axis)}><span {...stylex.props(s.axisNumber)}>{axis.n}</span><span {...stylex.props(s.axisLabel)}>{copy.axisLabels[axis.n - 1] ?? axis.name}</span></span>)}</div>
          {rows.map(row => { const added = row.createdAt ? dayMonth(row.createdAt) : ''; return <div key={row.id} data-story={row.id} {...stylex.props(s.ledgerRow, row.candidate && s.candidateRow, !row.candidate && marks.some(mark => mark.story === row.id && mark.mark === 'dup') && s.duplicateRow, !row.candidate && !!pick && same(row.values[pick.axis] ?? '', pick.value) && s.pickedRow)}>
            <span {...stylex.props(s.storyCell)}><span title={row.title} {...stylex.props(s.storyTitle, row.candidate && s.storyTitleDraft)}>{row.title}</span>{row.candidate ? <span {...stylex.props(s.briefMeta)}>{copy.draftMeta}</span> : added ? <span {...stylex.props(s.rowMeta)}>{added}</span> : null}</span>
            {ledger.data!.axes.map((axis, index) => {
              const value = row.values[index] || '—'; const count = useCount(index, value); const picked = isPicked(index, value); const mark = markFor(row.id, axis.n, row.candidate)
              return <button type="button" key={axis.n} data-axis={copy.axisLabels[index] ?? axis.name} title={ap.cell(axis.name, value, count)} data-mark={mark} data-picked={picked || undefined} aria-pressed={picked} onClick={() => setPick(picked ? null : { axis: index, value })} {...stylex.props(s.cell, s.cellButton, row.candidate && s.candidateCell, mark === 'dup' && s.duplicate, mark === 'dup' && s.duplicateCell, mark === 'overused' && s.overused, mark === 'overused' && s.overusedCell, picked && s.pickedCell)}><span data-cell-text="" {...stylex.props(s.cellText)}>{value}{row.candidate && count > 0 && <span {...stylex.props(s.rep)}>×{count}</span>}</span></button>
            })}
          </div> })}
          {!rows.length && <div {...stylex.props(s.padded, s.body)}>{copy.empty}</div>}
          {only && !shownPrior.length && <div {...stylex.props(s.padded, s.body)}>{ap.noRows}</div>}
        </section>}
        <div {...stylex.props(s.legend)}><span {...stylex.props(s.legendItem)}><span {...stylex.props(s.cell, s.duplicate)}>{copy.value}</span>{ap.duplicate}</span><span {...stylex.props(s.legendItem)}><span {...stylex.props(s.cell, s.overused)}>{copy.value}</span>{ap.overused}</span><span><span {...stylex.props(s.rep)}>×3</span> {ap.repetitions}</span><span>{ap.inspect}</span></div>
      </>}
      {lview === 'freq' && <>
        <section aria-label={ap.frequency} {...stylex.props(s.card)}>{frequency.map(axis => <div key={axis.n} {...stylex.props(s.frequencyRow)}><span {...stylex.props(s.frequencyAxis)}><span {...stylex.props(s.frequencyName)}><span {...stylex.props(s.frequencyAxisNumber)}>{axis.n}</span>{axis.name}</span>{axis.note && <span {...stylex.props(s.note)}>{axis.note}</span>}</span><span {...stylex.props(s.frequencyValues)}>{axis.values.map(([value, count]) => <span key={value} {...stylex.props(s.frequencyChip, same(activeCandidate?.values[axis.index] ?? '', value) && s.frequencyChosen, axis.limited && count >= 2 && s.frequencyMax)}>{value}<span {...stylex.props(s.frequencyCount, axis.limited && count >= 2 && s.frequencyMaxCount)}>{count}</span></span>)}</span></div>)}</section>
        <div {...stylex.props(s.legend)}><span {...stylex.props(s.legendItem)}><span {...stylex.props(s.frequencyChip, s.frequencyChosen)}>{copy.value}</span>{ap.chosen}</span><span {...stylex.props(s.legendItem)}><span {...stylex.props(s.frequencyChip, s.frequencyMax)}>{copy.value}</span>{ap.maximum}</span><span>{ap.frequencyNote}</span></div>
      </>}
    </>}
    {tab === 'universe' && <>
      {!!universe.error && <div role="alert">{copy.error} <button type="button" {...stylex.props(s.button)} onClick={() => void universe.refresh()}>{copy.retry}</button></div>}
      <div {...stylex.props(s.toolbar)}>
        <Segmented label={ap.entityType} fit="content" value={entityType} onChange={setEntityType} style={{ gap: 0, background: 'var(--color-card)', borderColor: 'var(--line-2)' }} thumbStyle={{ background: 'var(--projbrain-seg-on)', boxShadow: 'none' }} options={['all', 'char', 'place'].map(id => ({ id, label: <span {...stylex.props(s.filterLabel)}>{id === 'all' ? ap.all : id === 'char' ? ap.char : ap.place}<span {...stylex.props(s.filterCount)}>{entities.filter(entity => id === 'all' || entity.type === id).length}</span></span>, style: { padding: '0 11px', fontWeight: entityType === id ? 500 : 400 } }))} />
        {checks !== null && <button type="button" aria-pressed={needOnly} onClick={() => setNeedOnly(value => !value)} {...stylex.props(s.filterChip, needOnly && s.filterOn)}><Icon name="warning-2" size={13} style={{ color: 'var(--color-warn)' }} />{ap.need(entities.filter(needsReview).length)}</button>}
        <label {...stylex.props(s.nameFilter)}><Icon name="search-normal-1" size={14} {...stylex.props(s.nameIcon)} /><input type="search" aria-label={ap.nameLabel} placeholder={ap.name} value={entityQuery} onChange={event => setEntityQuery(event.target.value)} {...stylex.props(s.nameInput)} /></label>
      </div>
      <div {...stylex.props(s.universe)}><section aria-label={copy.universeLabel} {...stylex.props(s.card)}>
        <div {...stylex.props(s.universeRow, s.tableHead)}><span>{copy.entity}</span><span>{copy.appears}</span><span>{copy.role}</span><span {...stylex.props(s.checkCol)}>{copy.check}</span></div>
        {shownEntities.map(entity => <div key={entity.id} data-entity={entity.id} {...stylex.props(s.universeRow)}>
          <div {...stylex.props(s.entity)}>{entity.type === 'char'
            ? <CharAvatar name={entity.name} kind={entity.kind} size={32} />
            : <span {...stylex.props(s.avatar, s.place)}><Icon name="global" size={15} /></span>}<span {...stylex.props(s.nameCell)}><span title={entity.name} {...stylex.props(s.entityName)}>{entity.name}</span><span {...stylex.props(s.note)}>{copy.types[entity.type]}</span></span></div>
          <span {...stylex.props(s.storyChips)}>{entity.stories.map(story => <span key={story.id} title={story.title} {...stylex.props(s.chip, s.neutralChip)}>{story.title}</span>)}</span>
          <span title={entity.lastRole} {...stylex.props(s.role)}>{entity.lastRole || '—'}</span>
          <span {...stylex.props(s.checkCol)}><UniverseCheck checks={universe.data?.checks ?? null} stories={entity.stories} /></span>
        </div>)}
        {!shownEntities.length && <span {...stylex.props(s.padded, s.body)}>{ap.noEntities}</span>}
      </section><aside {...stylex.props(s.stack)}><UniverseSummary name={universe.data?.name ?? ''} counts={universe.data?.counts ?? { stories: 0, chars: 0, places: 0 }} />
        {(universe.data?.checks ?? []).filter(check => visible(check.story)).map((check, index) => <RecapCard key={index} project={project} check={check} />)}
        {universe.data && checks === null && <section {...stylex.props(s.card, s.uncheckedCard)}><span {...stylex.props(s.uncheckedTitle)}><Icon name="clock" size={16} {...stylex.props(s.note)} />{ap.uncheckedTitle}</span><span {...stylex.props(s.body)}>{ap.uncheckedBody}</span></section>}
        {/* §J câu 1 — dòng nhỏ ngay dưới thẻ nhắc cuối: mở danh sách mục đã “Giữ nguyên”. */}
        {(keptCount > 0 || everKept) && <>
          <button type="button" data-universe-kept="" aria-expanded={keptExpanded} aria-controls={keptExpanded ? 'universe-kept' : undefined} onClick={() => setShowKept(open => !open)} {...withGhost(stylex.props(s.keptLine))}><Icon name="tick-circle" size={13} />{keptCount > 0 ? copy.kept(keptCount) : copy.keptNone}<span data-kept-chevron="" style={{ display: 'inline-flex' }}><Icon name="arrow-down-2" size={12} style={{ transform: keptExpanded ? 'rotate(180deg)' : 'none', transition: 'transform .15s' }} /></span></button>
          {keptExpanded && <div id="universe-kept" {...stylex.props(s.keptList)}>{keptItems.map((item, index) => <RecapCard key={index} project={project} check={item} kept onUnkeep={user && canWriteStory(user, storyId(item.story)) ? unkeep : undefined} busy={unkeeping} />)}</div>}
        </>}
      </aside></div>
    </>}
    {tab === 'search' && <>
      <label {...(() => { const p = stylex.props(s.searchField); return { className: `kit-frame ${p.className}`, style: p.style } })()}><Icon name="search-normal-1" size={17} /><input type="search" value={q} onChange={event => setQ(event.target.value)} aria-label={copy.searchLabel} {...stylex.props(s.input)} /><span {...stylex.props(s.searchCount)}>{copy.results(filteredSearch.passages.length, filteredSearch.entities.length, searchStories)}</span></label>
      {!!results.error && !!q.trim() && <div role="alert">{copy.error} <button type="button" {...stylex.props(s.button)} onClick={() => void results.refresh()}>{copy.retry}</button></div>}
      <div {...stylex.props(s.toolbar)}>
        <Segmented label={ap.resultType} fit="content" value={searchKind} onChange={setSearchKind} style={{ gap: 0, background: 'var(--color-card)', borderColor: 'var(--line-2)' }} thumbStyle={{ background: 'var(--projbrain-seg-on)', boxShadow: 'none' }} options={[['all', ap.all, filteredSearch.entities.length + filteredSearch.passages.length], ['ent', copy.entity, filteredSearch.entities.length], ['hits', copy.passages, filteredSearch.passages.length]].map(([id, label, count]) => ({ id: String(id), label: <span {...stylex.props(s.filterLabel)}>{label}<span {...stylex.props(s.filterCount)}>{count}</span></span>, style: { padding: '0 11px', fontWeight: searchKind === id ? 500 : 400 } }))} />
        <span {...stylex.props(s.filterCount)}>{ap.inStory}</span>
        {[[null, ap.allStories] as const, ...searchRefs].map(([id, title]) => <button type="button" key={id ?? 'all'} title={title} aria-pressed={searchStory === id} onClick={() => setSearchStory(id)} {...stylex.props(s.filterChip, s.storyFilter, searchStory === id && s.filterOn)}><span {...stylex.props(s.axisLabel)}>{title}</span><span {...stylex.props(s.filterCount)}>{searchCount(id)}</span></button>)}
      </div>
      <div {...stylex.props(s.searchGrid, searchKind !== 'all' && s.searchSingle)}>{searchKind !== 'hits' && <section aria-label={copy.entityLabel} {...stylex.props(s.card, s.entityCard)}>
        <span {...stylex.props(s.label)}>{copy.entity}</span>
        {searchEntities.map((entity, index) => {
          const sharedCount = entities.find(shared => shared.id === entity.id)?.stories.length ?? 0
          return <div key={entity.id} {...stylex.props(s.entityHit, index > 0 && s.entityDivider)}>
          <span {...stylex.props(s.entityRow)}><Icon name={entityIcon[entity.type]} size={15} {...stylex.props(s.entityIcon, entityIconStyle[entity.type])} /><span title={entity.name} {...stylex.props(s.entityNameLoose)}>{entity.name}</span></span>
          <span {...stylex.props(s.entityWhere)}>{entity.stories.map(story => story.title).join(' · ')} · {copy.types[entity.type].toLocaleLowerCase('vi')}{sharedCount > 1 ? ` · ${ap.shared(sharedCount)}` : ''}</span>
          <span {...stylex.props(s.entityRel)}>{entity.lastRole}</span>
        </div> })}
      </section>}
        {searchKind !== 'ent' && <section aria-label={copy.passagesLabel} {...stylex.props(s.card, s.passageCard)}>
          <span {...stylex.props(s.label, s.labelPassages)}>{copy.passages}</span>
          {searchPassages.map((hit, index) => <a key={index} {...withRow(stylex.props(s.passageHit, index > 0 && s.passageDivider))} href={`#${paths.reader(project, storySlug(project, hit.story), hit.ch)}`}>
            <span {...stylex.props(s.passageMeta)}><span {...stylex.props(s.passageStory)}>{hit.storyTitle}</span><span>· {copy.chapter(hit.ch)}</span></span>
            <span {...stylex.props(s.quote)}><Highlighted quote={hit.quote} ranges={hit.hl} /></span>
          </a>)}
          <span {...stylex.props(s.resultsNote)}>{copy.allVisible}</span>
        </section>}
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
  // A null result means no chapter has been checked yet, not a passing check.
  if (checks === null) return <span {...stylex.props(s.chip, s.uncheckedChip)}>{ap.unchecked}</span>
  const found = checks.some(check => stories.some(story => story.id === check.story))
  if (!found) return <Level level="ok" />
  return <span {...stylex.props(s.chip, s.warningChip)}>{copy.recapBadge}</span>
}
