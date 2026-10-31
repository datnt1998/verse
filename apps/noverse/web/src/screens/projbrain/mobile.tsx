import { useState, type ComponentProps } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, Sheet as KitSheet } from '@noverse/ui'
import type { LedgerAxis, LedgerCandidate, LedgerCheck, ProjectSearch, ProjectUniverse, ProjectUniverseEntity } from '../../api/types'
import { paths } from '../../router'
import { t } from '../../i18n/vi'
import { apCopy as ap } from './copy'
import { Highlighted, UniverseCheck } from './index'

type Row = { id: string; title: string; values: string[]; createdAt: string | null; candidate: boolean }
type Frequency = LedgerAxis & { index: number; limited: boolean; note: string; values: [string, number][] }
type Props = {
  project: string; projectName: string; tab: string; pickTab: (tab: string) => void
  axes: LedgerAxis[]; rows: Row[]; candidate: LedgerCandidate | null; checks: LedgerCheck[]; frequency: Frequency[]
  errors: number; warnings: number; busy: boolean; suggest: () => Promise<void>; replacement: boolean
  entities: ProjectUniverseEntity[]; universe: ProjectUniverse | undefined; needsReview: (entity: ProjectUniverseEntity) => boolean
  q: string; setQ: (q: string) => void; search: ProjectSearch; error: string; loading: boolean
}
const same = (a: string, b: string) => !!a.trim() && a.trim().replace(/\s+/g, ' ').toLocaleLowerCase('vi') === b.trim().replace(/\s+/g, ' ').toLocaleLowerCase('vi')
const date = (s: string | null) => { if (!s) return ''; const d = new Date(s); return Number.isNaN(d.getTime()) ? '' : `${String(d.getDate()).padStart(2, '0')}/${String(d.getMonth() + 1).padStart(2, '0')}` }
const styles = stylex.create({
  root: { height: '100%', minHeight: 0, display: 'flex', flexDirection: 'column', backgroundColor: 'var(--color-bg)', color: 'var(--color-fg)' },
  header: { minHeight: 52, display: 'flex', alignItems: 'center', gap: 4, paddingTop: 'env(safe-area-inset-top)' },
  back: { width: 44, height: 44, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'inherit' },
  title: { display: 'flex', flexDirection: 'column', lineHeight: 1.25 }, heading: { fontSize: 17, fontWeight: 600 }, sub: { fontSize: 12.5, color: 'var(--color-fg-3)' },
  tabs: { display: 'flex', margin: '0 16px 12px', padding: 3, borderRadius: 12, backgroundColor: 'var(--color-card)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-1)' },
  tab: { flex: 1, height: 36, border: 0, borderRadius: 9, backgroundColor: 'transparent', color: 'var(--color-fg-3)', fontSize: 14 }, selected: { backgroundColor: 'var(--projbrain-seg-on)', color: 'var(--color-fg)', fontWeight: 600 },
  scroll: { overflowY: 'auto', overflowX: 'hidden', flex: 1, minHeight: 0, paddingBottom: 40 },
  card: { margin: '0 16px', borderRadius: 16, backgroundColor: 'var(--color-card)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-1)', overflow: 'hidden' },
  checkHead: { padding: '13px 14px 10px', display: 'flex', flexDirection: 'column', gap: 6 }, check: { display: 'flex', flexDirection: 'column', gap: 4, padding: '11.5px 14px', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--line-1)', fontSize: 14, lineHeight: 1.5 },
  actions: { padding: '10px 14px 14px', borderTop: '1px solid var(--line-1)' },
  filters: { display: 'flex', overflowX: 'auto', gap: 8, padding: '14px 16px 8px' }, chip: { flexShrink: 0, minHeight: 34, padding: '0 13px', borderRadius: 999, border: '1px solid var(--line-2)', backgroundColor: 'var(--color-card)', color: 'var(--color-fg)', fontSize: 13.5, whiteSpace: 'nowrap' }, chipOn: { borderColor: 'var(--color-accent)', color: 'var(--color-accent)' },
  universeFilters: { paddingTop: 8 }, searchFilters: { paddingTop: 0, marginTop: 12 }, universeSummary: { marginBottom: 9 }, universeSub: { fontSize: 13, lineHeight: 1.5, color: 'var(--color-fg-3)' },
  hint: { margin: '0 30px 8px', fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.5 }, row: { width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'stretch', gap: 6, padding: '12px 14px', border: 0, borderBottom: '1px solid var(--line-1)', backgroundColor: 'transparent', color: 'inherit', textAlign: 'left', font: 'inherit' },
  values: { display: 'flex', flexWrap: 'wrap', gap: 6 }, value: { minHeight: 32, padding: '0 11px', borderRadius: 9, border: 0, backgroundColor: 'var(--line-1)', color: 'var(--color-fg)', fontSize: 13.5 }, chosen: { boxShadow: 'inset 0 0 0 1px var(--color-accent)', color: 'var(--color-accent)' }, count: { marginLeft: 7, fontSize: 12, color: 'var(--color-fg-3)' },
  entity: { display: 'flex', gap: 12, alignItems: 'center', padding: '10px 14px', minHeight: 62, borderBottom: '1px solid var(--line-1)' }, avatar: { flexShrink: 0, width: 38, height: 38, borderRadius: 999, display: 'flex', justifyContent: 'center', alignItems: 'center', color: 'var(--color-accent)', backgroundColor: 'var(--line-2)' },
  search: { display: 'block', width: 'calc(100% - 32px)', height: 46, margin: '0 16px', padding: '0 14px', borderRadius: 14, border: '1px solid var(--line-2)', backgroundColor: 'var(--color-card)', color: 'var(--color-fg)', fontSize: 16 }, sectionTitle: { padding: '14px 30px 8px', color: 'var(--color-fg-3)', fontSize: 12.5 },
  status: { alignSelf: 'flex-start', borderRadius: 7, padding: '3px 8px', fontSize: 12, backgroundColor: 'var(--line-2)' }, danger: { color: 'var(--color-err)' }, warning: { color: 'var(--color-warn)' }, ok: { color: 'var(--color-ok)' },
  close: { position: 'absolute', top: 16, right: 16, width: 40, height: 40, border: 0, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--line-2)', color: 'var(--color-fg-2)' },
  sheetSub: { margin: '0 0 10px', fontSize: 13, color: 'var(--color-fg-3)' }, sheetCard: { margin: 0, borderRadius: 16, backgroundColor: 'var(--color-card)', border: '1px solid var(--line-1)', overflow: 'hidden' },
  sheetBody: { overflowY: 'auto', padding: '0 16px' }, sheetRow: { display: 'flex', alignItems: 'center', minHeight: 48, gap: 10, padding: '8px 14px', borderBottom: '1px solid var(--line-1)', fontSize: 14 }, sheetValue: { marginLeft: 'auto', textAlign: 'right', maxWidth: '55%' }, accent: { color: 'var(--color-accent)' }, grow: { flex: 1, minWidth: 0 },
  sheetOuter: { paddingBottom: '20px !important', maxHeight: '80vh !important' },
})

function Sheet({ children, onClose, ...props }: ComponentProps<typeof KitSheet>) {
  return <KitSheet {...props} xstyle={styles.sheetOuter} onClose={onClose}><button type="button" aria-label="Đóng" onClick={onClose} {...stylex.props(styles.close)}><Icon name="close-circle-bulk" size={18} /></button>{children}</KitSheet>
}

export function MobileProjectBrain(p: Props) {
  const [view, setView] = useState<'freq' | 'stories' | 'only'>('freq')
  const [filter, setFilter] = useState('all')
  const [kind, setKind] = useState('all')
  const [sheet, setSheet] = useState<{ kind: 'value'; axis: number; value: string } | { kind: 'story'; row: Row } | null>(null)
  const shown = p.rows.filter(row => row.candidate || view !== 'only' || row.values.some((v, i) => same(v, p.candidate?.values[i] ?? '')))
  const prior = p.rows.filter(row => !row.candidate).slice(0, 10)
  const using = sheet?.kind === 'value' ? prior.filter(row => same(row.values[sheet.axis] ?? '', sheet.value)) : []
  const chosen = sheet?.kind === 'value' && same(p.candidate?.values[sheet.axis] ?? '', sheet.value)
  const entities = p.entities.filter(e => filter === 'all' || (filter === 'need' ? p.needsReview(e) : e.type === filter))
  const sheetTitle = sheet?.kind === 'value' ? `“${sheet.value}”` : sheet?.kind === 'story' ? sheet.row.title : ''
  const checkText = (c: LedgerCheck) => {
    const against = c.rule === 'combination' && c.level === 'error' && p.rows.find(row => row.id === c.against)
    if (against) return `Trùng hẳn với “${against.title}”.`
    if (c.rule === 'overuse' && c.level === 'warning' && p.candidate) {
      const values = c.axes.map(n => {
        const value = p.candidate!.values[n - 1]
        const count = p.frequency[n - 1]?.values.find(([v]) => same(v, value ?? ''))?.[1]
        return value && count !== undefined ? `${t.projbrain.axisLabels[n - 1]} “${value}” đã dùng ${count} lần` : ''
      }).filter(Boolean)
      if (values.length) return `${values.join(' · ')}, tối đa 2.`
    }
    return c.text
  }
  return <main {...stylex.props(styles.root)} aria-label="Não dự án">
    <header {...stylex.props(styles.header)}><a href={`#${paths.settings('mobile')}`} aria-label="Quay lại Thêm" {...stylex.props(styles.back)}><Icon name="arrow-left-2" size={22} /></a><span {...stylex.props(styles.title)}><b {...stylex.props(styles.heading)}>Não dự án</b><span {...stylex.props(styles.sub)}>{p.projectName} · {p.rows.length} truyện</span></span></header>
    <div role="tablist" aria-label="Phần" {...stylex.props(styles.tabs)}>{[['ledger','Sổ cái'],['universe','Vũ trụ'],['search','Tìm']].map(([id,label]) => <button key={id} type="button" role="tab" aria-selected={p.tab === id} onClick={() => p.pickTab(id)} {...stylex.props(styles.tab, p.tab === id && styles.selected)}>{label}</button>)}</div>
    <div {...stylex.props(styles.scroll)}>{p.error && <div role="alert">{p.error}</div>}{p.loading && <p>Đang tải…</p>}
    {p.tab === 'ledger' && <>
      {p.candidate && <section aria-label="Kiểm truyện mới" {...stylex.props(styles.card)}><div {...stylex.props(styles.checkHead)}><span {...stylex.props(styles.sub)}>Kiểm truyện mới · so với 10 truyện gần nhất</span><b>“{p.candidate.title}”</b><span role="status" {...stylex.props(styles.status, p.errors ? styles.danger : p.warnings ? styles.warning : styles.ok)}>{ap.verdict(p.errors,p.warnings)}</span></div>{p.checks.map(c => <div key={c.rule} {...stylex.props(styles.check)}><span><span {...stylex.props(styles.status, c.level === 'error' ? styles.danger : c.level === 'warning' ? styles.warning : styles.ok)}>{t.projbrain.level[c.level]}</span> {ap.checkNames[c.rule] ?? c.rule}</span><span>{checkText(c)}</span>{c.fix && <span {...stylex.props(styles.sub)}>{c.fix}</span>}</div>)}<div {...stylex.props(styles.actions)}><button type="button" disabled={p.busy} onClick={() => void p.suggest()} {...stylex.props(styles.chip)}><Icon name="magic-star" size={15} /> {p.replacement ? t.projbrain.undo : t.projbrain.suggest}</button></div></section>}
      <div role="radiogroup" aria-label="Cách xem" {...stylex.props(styles.filters)}>{([['freq','Tần suất theo trục'],['stories','Truyện'],['only','Chỉ truyện trùng']] as const).map(([id,label]) => <button key={id} role="radio" aria-checked={view === id} onClick={() => setView(id)} {...stylex.props(styles.chip, view === id && styles.chipOn)}>{label}</button>)}</div>
      {view === 'freq' ? <><p {...stylex.props(styles.hint)}>Số bên cạnh là số truyện đã dùng giá trị đó. Viền tím: truyện mới chọn. Chạm một giá trị để xem truyện nào đã dùng.</p><div {...stylex.props(styles.card)}>{p.frequency.map(axis => <div key={axis.n} {...stylex.props(styles.row)}><span><span {...stylex.props(styles.sub)}>{axis.n}　</span>{axis.name}</span>{axis.note && <span {...stylex.props(styles.sub)}>{axis.note}</span>}<span {...stylex.props(styles.values)}>{axis.values.map(([value,count]) => <button key={value} type="button" onClick={() => setSheet({kind:'value',axis:axis.index,value})} {...stylex.props(styles.value, same(p.candidate?.values[axis.index] ?? '', value) && styles.chosen)}>{value}<span {...stylex.props(styles.count)}>{count}</span></button>)}</span></div>)}</div></> : <><div {...stylex.props(styles.card)}>{shown.map(row => <button key={row.id} type="button" onClick={() => setSheet({kind:'story',row})} {...stylex.props(styles.row)}><span>{row.title} ›</span><span {...stylex.props(styles.sub, row.candidate && styles.accent)}>{row.candidate ? 'Truyện mới · brief' : date(row.createdAt)}</span><span {...stylex.props(styles.values)}>{[0,2,3,5].map(i => <span key={i} {...stylex.props(styles.status, !row.candidate && same(row.values[i] ?? '', p.candidate?.values[i] ?? '') && styles.chosen)}>{row.values[i] || '—'}</span>)}</span></button>)}</div><p {...stylex.props(styles.hint)}>Mỗi thẻ hiện tổ hợp trục 1 · 3 · 4 · 6. Chạm để xem đủ 10 trục.</p></>}
    </>}
    {p.tab === 'universe' && <><div {...stylex.props(styles.card, styles.checkHead, styles.universeSummary)}><b>{p.universe?.name}</b>{p.universe && <span {...stylex.props(styles.universeSub)}>{p.universe.counts.stories} truyện, {p.universe.counts.chars} nhân vật và {p.universe.counts.places} địa điểm dùng lại. Nhắc chuyện cũ tối đa hai câu, lồng vào hành động.</span>}</div><div role="radiogroup" aria-label="Lọc" {...stylex.props(styles.filters, styles.universeFilters)}>{[['all','Tất cả'],['char','Nhân vật'],['place','Địa điểm'],...(p.universe?.checks !== null && p.universe ? [['need','Cần xem']] : [])].map(([id,label]) => <button key={id} role="radio" aria-checked={filter === id} onClick={() => setFilter(id)} {...stylex.props(styles.chip, filter === id && styles.chipOn)}>{label} <span {...stylex.props(styles.count)}>{p.entities.filter(e => id === 'all' || (id === 'need' ? p.needsReview(e) : e.type === id)).length}</span></button>)}</div>{p.universe && p.universe.checks === null && <p {...stylex.props(styles.hint)}>Chưa có kết quả kiểm. Biên tập viên kiểm phần nhắc chuyện cũ khi duyệt chương có nhân vật hoặc địa điểm dùng chung.</p>}<div {...stylex.props(styles.card)}>{entities.map(e => <div key={e.id} {...stylex.props(styles.entity)}><span {...stylex.props(styles.avatar)}>{e.type === 'char' ? e.name.split(' ').slice(-2).map(w => w[0]).join('').toLocaleUpperCase('vi') : <Icon name="global" size={17} />}</span><span {...stylex.props(styles.grow, styles.title)}><b>{e.name}</b><span {...stylex.props(styles.sub)}>{t.projbrain.types[e.type]} · {e.stories.length} truyện</span></span><UniverseCheck checks={p.universe?.checks ?? null} stories={e.stories} /></div>)}{!entities.length && <p {...stylex.props(styles.hint)}>Không có mục nào cần xem.</p>}</div></>}
    {p.tab === 'search' && <><input type="search" aria-label="Tìm trong mọi truyện của dự án" value={p.q} onChange={e => p.setQ(e.target.value)} {...stylex.props(styles.search)} /><div role="radiogroup" aria-label="Loại kết quả" {...stylex.props(styles.filters, styles.searchFilters)}>{[['all','Tất cả',p.search.entities.length+p.search.passages.length],['ent','Thực thể',p.search.entities.length],['hits','Đoạn văn',p.search.passages.length]].map(([id,label,n]) => <button key={id} role="radio" aria-checked={kind === id} onClick={() => setKind(String(id))} {...stylex.props(styles.chip, kind === id && styles.chipOn)}>{label}<span {...stylex.props(styles.count)}>{n}</span></button>)}</div>{kind !== 'hits' && <><div {...stylex.props(styles.sectionTitle)}>Thực thể</div><div {...stylex.props(styles.card)}>{p.search.entities.map(e => <div key={e.id} {...stylex.props(styles.row)}><b>{e.name}</b><span {...stylex.props(styles.sub)}>{e.stories.map(s => s.title).join(' · ')} · {t.projbrain.types[e.type]}</span><span>{e.lastRole}</span></div>)}</div></>}{kind !== 'ent' && <><div {...stylex.props(styles.sectionTitle)}>Đoạn văn</div><div {...stylex.props(styles.card)}>{p.search.passages.map((hit,i) => <a key={i} href={`#${paths.reader(p.project, hit.story.startsWith(`${p.project}/`) ? hit.story.slice(p.project.length+1) : hit.story, hit.ch)}`} {...stylex.props(styles.row)}><span {...stylex.props(styles.sub)}>{hit.storyTitle} · chương {hit.ch}</span><span><Highlighted quote={hit.quote} ranges={hit.hl} /></span></a>)}</div></>}</>}
    </div><Sheet open={!!sheet} onClose={() => setSheet(null)} title={sheetTitle} height="auto"><div {...stylex.props(styles.sheetBody)}>{sheet?.kind === 'value' && <><p {...stylex.props(styles.sheetSub)}>{p.axes[sheet.axis]?.name} · {using.length} truyện trong 10 truyện gần nhất{chosen ? ' · truyện mới cũng chọn' : ''}</p><div {...stylex.props(styles.sheetCard)}>{chosen && p.candidate && <div {...stylex.props(styles.sheetRow)}><span {...stylex.props(styles.grow)}>{p.candidate.title}</span><span {...stylex.props(styles.accent)}>Truyện mới</span></div>}{using.map(row => <div key={row.id} {...stylex.props(styles.sheetRow)}><span {...stylex.props(styles.grow)}>{row.title}</span><span {...stylex.props(styles.sub)}>{date(row.createdAt)}</span></div>)}</div></>}{sheet?.kind === 'story' && <><p {...stylex.props(styles.sheetSub)}>{sheet.row.candidate ? 'Truyện mới' : date(sheet.row.createdAt)} · 10 trục</p><div {...stylex.props(styles.sheetCard)}>{p.axes.map((axis,i) => <div key={axis.n} {...stylex.props(styles.sheetRow)}><span {...stylex.props(styles.sub)}>{axis.n}</span><span {...stylex.props(styles.grow, styles.sub)}>{axis.name}</span><span {...stylex.props(styles.sheetValue, !sheet.row.candidate && same(sheet.row.values[i] ?? '', p.candidate?.values[i] ?? '') && styles.accent)}>{sheet.row.values[i] || '—'}</span></div>)}</div></>}</div></Sheet>
  </main>
}
