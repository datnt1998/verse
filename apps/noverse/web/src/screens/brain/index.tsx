// Não truyện (22-brief-2510.md › D10, artboard Brain.dc.html): 3 cột 244 · giữa · 360.
// Rail lọc loại thực thể + kiểu quan hệ; giữa là Đồ thị | Danh sách + ô tìm + thanh chương;
// phải là chi tiết thực thể đang chọn (quan hệ, xưng hô, diễn biến, xuất hiện, banner khoá D8).
import { useEffect, useMemo, useState } from 'react'
import type { BrainEdgeType, BrainEntityType } from '../../api/types'
import { paths, useRoute } from '../../router'
import { Icon } from '@noverse/ui'
import { Checkbox } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { EmptyState, Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { useAddressTerms, useBrain } from '../project/cast/data'
import { ChapterRail } from './ChapterRail'
import { DetailColumn } from './DetailColumn'
import { StoryGraph } from './StoryGraph'
import {
  EDGE_META, EDGE_ORDER, TYPE_META, TYPE_ORDER, countsAt, edgeCountsAt, listGroups, narratorOf,
} from './model'
import { useChapterQuery } from './useChapterQuery'
import './brain.css'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

export function BrainScreen({ project, story }: { project: string; story: string }) {
  const brain = useBrain(project, story)
  const address = useAddressTerms(project, story)
  const [hiddenTypes, setHiddenTypes] = useState<ReadonlySet<BrainEntityType>>(new Set())
  const [hiddenEdges, setHiddenEdges] = useState<ReadonlySet<BrainEdgeType>>(new Set())
  const [view, setView] = useState<'graph' | 'list'>('graph')
  const [query, setQuery] = useState('')
  // Tab Truyện › “Tất cả” mở thẳng dạng Danh sách (`?view=list`); đổi `?view=` lúc màn đang mở cũng chọn lại.
  const wantedView = useRoute().search.get('view')
  useEffect(() => {
    if (wantedView === 'list' || wantedView === 'graph') setView(wantedView)
  }, [wantedView])
  const [chapter, setChapter] = useChapterQuery(brain.data?.chapters ?? null)
  const [selected, setSelected] = useState<string | null>(null)
  const data = brain.data

  // Thực thể đang chọn: giữ lựa chọn nếu còn trong dữ liệu mới, không thì người kể chuyện.
  useEffect(() => {
    if (!data) return
    setSelected((value) => {
      if (value && data.entities.some((entity) => entity.id === value)) return value
      return narratorOf(data.entities) ?? data.entities[0]?.id ?? ''
    })
  }, [data])

  const counts = useMemo(() => (data && chapter !== null ? countsAt(data, chapter) : null), [data, chapter])
  const edgeCounts = useMemo(() => (data && chapter !== null ? edgeCountsAt(data, chapter) : null), [data, chapter])
  // Câu 22: loại chưa có dữ kiện nào (địa điểm, vật khi engine chưa trích) thì không có chip lọc.
  const presentTypes = useMemo(
    () => (data ? TYPE_ORDER.filter((type) => data.entities.some((entity) => entity.type === type)) : []),
    [data],
  )
  const groups = useMemo(
    () => (data && chapter !== null ? listGroups(data, chapter, query) : []),
    [data, chapter, query],
  )

  const toggleType = (type: BrainEntityType, on: boolean) => setHiddenTypes((current) => {
    const next = new Set(current)
    if (on) next.delete(type)
    else next.add(type)
    return next
  })
  const toggleEdge = (type: BrainEdgeType, on: boolean) => setHiddenEdges((current) => {
    const next = new Set(current)
    if (on) next.delete(type)
    else next.add(type)
    return next
  })

  const rail = (
    <aside aria-label="Bộ lọc" className="scroll"
      style={{ borderRight: '1px solid var(--line-1)', padding: '14px 10px', display: 'flex', flexDirection: 'column', gap: 14 }}>
      <a className="hov-ghost" href={`#${paths.story(project, story)}`} style={{
        height: 30, padding: '0 10px', borderRadius: 8, display: 'flex', alignItems: 'center', gap: 8,
        fontSize: 12.5, color: 'var(--color-fg-3)',
      }}>
        <Icon name="arrow-left-2" size={14} />Về truyện
      </a>
      <div style={{ padding: '0 10px', display: 'flex', flexDirection: 'column', gap: 2 }}>
        <span style={{ fontSize: 17, fontWeight: 600 }}>Não truyện</span>
        <span style={{ fontSize: 12, color: 'var(--brain-muted)', lineHeight: 1.5 }}>
          Nhân vật, quan hệ và xưng hô theo từng chương. Trợ lý ghi, Biên tập viên soát, bạn sửa được.
        </span>
      </div>
      <div role="group" aria-label="Loại thực thể" style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        <span className="lbl" style={{ padding: '0 10px 6px' }}>Loại</span>
        {presentTypes.map((type) => (
          <Checkbox key={type} tone="mono" size={16} className="hov-row"
            checked={!hiddenTypes.has(type)} onChange={(on) => toggleType(type, on)}
            style={{ width: '100%', minHeight: 34, padding: '0 10px', margin: 0, borderRadius: 9, fontSize: 13, color: 'var(--color-fg-2)' }}
            label={
              <>
                <Icon name={TYPE_META[type].icon} size={15} style={{ color: TYPE_META[type].color }} />
                <span style={{ flex: 1 }}>{TYPE_META[type].label}</span>
                <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{counts?.[type] ?? 0}</span>
              </>
            } />
        ))}
      </div>
      <div role="group" aria-label="Kiểu quan hệ" style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        <span className="lbl" style={{ padding: '0 10px 6px' }}>Kiểu quan hệ</span>
        {EDGE_ORDER.map((type) => (
          <Checkbox key={type} tone="mono" size={16} className="hov-row"
            checked={!hiddenEdges.has(type)} onChange={(on) => toggleEdge(type, on)}
            style={{ width: '100%', minHeight: 34, padding: '0 10px', margin: 0, borderRadius: 9, fontSize: 13, color: 'var(--color-fg-2)' }}
            label={
              <>
                <span aria-hidden="true" style={{
                  width: 18, height: 0, flex: 'none',
                  borderTop: `2px ${EDGE_META[type].dashed ? 'dashed' : 'solid'} ${EDGE_META[type].color}`,
                }} />
                <span style={{ flex: 1 }}>{EDGE_META[type].label}</span>
                <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{edgeCounts?.[type] ?? 0}</span>
              </>
            } />
        ))}
      </div>
      <div style={{
        marginTop: 'auto', padding: 10, borderRadius: 12, background: 'var(--color-scope-bg)',
        border: '1px solid var(--line-1)', fontSize: 12, color: 'var(--brain-muted)', lineHeight: 1.5, display: 'flex', gap: 8,
      }}>
        <Icon name="info-circle" size={14} style={{ marginTop: 2 }} />
        <span>Đây là dữ kiện gốc của truyện. Sửa ở đây thì Người viết và Biên tập viên dùng ngay từ chương sau.</span>
      </div>
    </aside>
  )

  if (brain.error && !data) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <EmptyState icon="warning-2" title="Không tải được Não truyện"
          hint="Máy chủ chưa đọc được dữ kiện của truyện."
          action={
            <button type="button" className="hov-ghost" onClick={() => void brain.refresh()}
              style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
              Thử lại
            </button>
          } />
      </div>
    )
  }

  if (!data || chapter === null || selected === null) {
    return (
      <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'grid', gridTemplateColumns: '244px minmax(0, 1fr) 360px' }}>
        <div style={{ borderRight: '1px solid var(--line-1)', padding: 12, display: 'flex', flexDirection: 'column', gap: 8 }}>
          {[0, 1, 2, 3, 4, 5, 6].map((key) => <Skeleton key={key} height={30} radius={9} />)}
        </div>
        <div style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Skeleton width={260} height={34} radius={10} />
          <Skeleton height={420} radius={16} />
        </div>
        <div style={{ borderLeft: '1px solid var(--line-1)', padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
          {[0, 1, 2, 3, 4].map((key) => <Skeleton key={key} height={24} radius={6} />)}
        </div>
      </div>
    )
  }

  if (data.entities.length === 0) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <EmptyState icon="routing-2" title="Não truyện chưa có dữ kiện"
          hint="Não truyện có dữ kiện sau khi truyện viết xong chương đầu." />
      </div>
    )
  }

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'grid', gridTemplateColumns: '244px minmax(0, 1fr) 360px', gridTemplateRows: 'minmax(0, 1fr)' }}>
      {rail}
      <main style={{ minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
        <PanelRefreshBar resourceKeys={[['story', project, story, 'brain'], ['story', project, story, 'address']]} updatedAt={brain.updatedAt}
          style={{ minHeight: 2, height: 2, overflow: 'hidden' }} />
        <StaleBanner error={brain.error || address.error}
          onRetry={() => { void brain.refresh(); void address.refresh() }} />
        <div style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 10, padding: '12px 20px' }}>
          <Segmented label="Cách xem" value={view} onChange={(next) => setView(next === 'list' ? 'list' : 'graph')} size={28} radius={7}
            style={{ padding: 3, borderRadius: 10, background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', gap: 0 }}
            thumbStyle={{ background: 'var(--color-surface-6)' }}
            options={[
              { id: 'graph', label: 'Đồ thị', icon: <Icon name="routing-2" size={14} />, style: { flex: '0 0 auto', padding: '0 10px' } },
              { id: 'list', label: 'Danh sách', icon: <Icon name="document-text" size={14} />, style: { flex: '0 0 auto', padding: '0 10px' } },
            ]} />
          <label className="kit-frame" style={{
            flex: 1, maxWidth: 320, height: 34, display: 'flex', alignItems: 'center', gap: 8, padding: '0 10px',
            borderRadius: 9, background: 'var(--color-surface-0)', border: '1px solid var(--line-4)',
          }}>
            <Icon name="search-normal-1" size={15} style={{ color: 'var(--color-fg-4)' }} />
            <input type="search" value={query} onChange={(event) => setQuery(event.target.value)}
              placeholder="Tìm nhân vật, vật, sự kiện…" aria-label="Tìm trong Não truyện"
              style={{ flex: 1, minWidth: 0, border: 'none', background: 'transparent', color: 'var(--color-fg)', fontSize: 13, outline: 'none' }} />
          </label>
          <span style={{ flex: 1 }} />
        </div>

        {view === 'graph' ? (
          <StoryGraph brain={data} chapter={chapter} selected={selected} onSelect={setSelected}
            hiddenTypes={hiddenTypes} hiddenEdges={hiddenEdges} query={query} />
        ) : (
          <div className="scroll" style={{ flex: '1 1 auto', minHeight: 0, padding: '0 20px 12px' }}>
            {groups.map((group) => (
              <section key={group.type} className="card" style={{ borderRadius: 14, overflow: 'hidden', marginBottom: 10 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 14px', borderBottom: '1px solid var(--line-1)' }}>
                  <Icon name={group.icon} size={15} style={{ color: group.color }} />
                  <span style={{ fontSize: 13.5, fontWeight: 600 }}>{group.label}</span>
                  <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{group.count}</span>
                </div>
                {group.items.map((item) => (
                  <button key={item.id} className="brain-lrow hov-row" type="button" aria-pressed={item.id === selected}
                    onClick={() => setSelected(item.id)}
                    style={{ background: item.id === selected ? 'var(--kit-acc-6)' : 'transparent' }}>
                    <span style={{ fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.name}</span>
                    <span style={{ fontSize: 12, color: 'var(--brain-muted)' }}>{item.since}</span>
                    <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.rels}</span>
                    <span style={{ fontSize: 12, color: 'var(--brain-chg)', textAlign: 'right' }}>{item.changed}</span>
                  </button>
                ))}
              </section>
            ))}
          </div>
        )}

        <ChapterRail brain={data} chapter={chapter} onChapter={setChapter} />
      </main>
      <DetailColumn brain={data} chapter={chapter} entityId={selected} address={address.data ?? null}
        writing={address.data?.writing ?? null} project={project} story={story} />
    </div>
  )
}
