// M8 · Não truyện trên điện thoại (MBrain.dc.html, 18-mobile.md › M8).
// Không vẽ đồ thị ở 390 px: danh sách nhóm theo loại + thẻ chương + bottom sheet chi tiết.
import { useEffect, useMemo, useRef, useState } from 'react'
import { getAddressTerms, getBrain } from '../../../api/client'
import type { BrainEntityType } from '../../../api/types'
import { keys, useResource } from '../../../data'
import { BottomSheet } from '../../../mobile/primitives'
import { useTabMode } from '../../../mobile/runtime'
import { navigate, paths } from '../../../router'
import { useMediaQuery } from '../../../shell/AppShell'
import { Icon } from '@noverse/ui'
import { CharAvatar } from '../../../ui/CharAvatar'
import { radioGroupKeys } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { initialChapter, lockSentence } from '../model'
import {
  PHONE_FOOTNOTE, RAIL_SCROLL_OVER, chapterCells, chapterHead, countText, filterChips, phoneGroups,
  scrollLeftToCenter, sheetModel,
} from './view'
import './mbrain.css'

/** Nền ô biểu tượng loại: màu loại ở 12% (artboard `glBg = color + '1F'`). */
const glyphFill = (color: string) => `color-mix(in srgb, ${color} 12.2%, transparent)`

export function MobileBrain({ project, story, chapter, person = null }: {
  project: string
  story: string
  chapter: number | null
  /** `?person=` — tab Truyện của màn Viết chạm tên nhân vật: mở sẵn tấm kéo lên của người đó. */
  person?: string | null
}) {
  const brain = useResource(keys.story(project, story, 'brain'), ({ signal }) => getBrain(project, story, signal))
  const address = useResource(keys.story(project, story, 'address'), ({ signal }) => getAddressTerms(project, story, signal))
  const [ch, setCh] = useState<number | null>(chapter)
  const [filter, setFilter] = useState<'all' | BrainEntityType>('all')
  const [searching, setSearching] = useState(false)
  const [query, setQuery] = useState('')
  const [sheetId, setSheetId] = useState<string | null>(null)
  const openedFor = useRef<string | null>(null)
  const data = brain.data
  const current = ch ?? (data ? chapter ?? initialChapter(data.chapters) : null)
  useTabMode(sheetId ? 'hide' : undefined)

  useEffect(() => {
    setCh(chapter); setFilter('all'); setSearching(false); setQuery(''); setSheetId(null); openedFor.current = null
  }, [project, story, chapter, person])

  // Người mở từ màn Viết: chờ dữ liệu Não truyện rồi mở tấm kéo lên đúng một lần (đóng rồi không tự mở lại).
  useEffect(() => {
    if (!person || !data || openedFor.current === person) return
    const entity = data.entities.find((item) => item.id === person || item.name === person)
    if (!entity) return
    openedFor.current = person
    setSheetId(entity.id)
  }, [person, data])

  // Câu 17: truyện > 10 chương thì hàng ô cuộn ngang — ô đang xem luôn được đưa vào giữa khi mở và khi đổi chương.
  // Hàng ≤ 10 chương chia đều cả khung nên không cuộn.
  const cellsRef = useRef<HTMLDivElement>(null)
  const reduce = useMediaQuery('(prefers-reduced-motion: reduce)')
  const loaded = data != null
  const scrolls = data ? data.chapters.length > RAIL_SCROLL_OVER : false
  useEffect(() => {
    if (!scrolls) return
    const box = cellsRef.current
    const cell = box?.querySelector<HTMLElement>('[aria-checked="true"]')
    if (!box || !cell) return
    const cellLeft = cell.getBoundingClientRect().left - box.getBoundingClientRect().left + box.scrollLeft
    const left = scrollLeftToCenter(cellLeft, cell.offsetWidth, box.clientWidth)
    if (Math.abs(left - box.scrollLeft) < 1) return
    box.scrollTo({ left, behavior: reduce ? 'auto' : 'smooth' })
  }, [current, loaded, scrolls, reduce])

  const chips = useMemo(() => (data && current !== null ? filterChips(data, current, query, filter) : []), [data, current, query, filter])
  const groups = useMemo(() => (data && current !== null ? phoneGroups(data, current, filter, query) : []), [data, current, filter, query])
  const sheet = data && current !== null && sheetId ? sheetModel(data, address.data ?? null, sheetId, current) : null
  const head = data && current !== null ? chapterHead(data, current) : null
  const writing = address.data?.writing ?? null
  const selectedChapter = data && current !== null ? data.chapters.find((item) => item.n === current) : undefined

  const back = () => { if (history.length > 1) history.back(); else navigate(paths.settings('mobile')) }
  const close = () => setSheetId(null)

  return <div className="mb-root">
    <header className="mb-top">
      <button className="mb-back" type="button" aria-label="Quay lại Thêm" onClick={back}><Icon name="arrow-left-2" size={22} /></button>
      {searching ? <>
        <label className="kit-frame mb-search">
          <Icon name="search-normal-1" size={17} className="mb-search-icon" />
          <input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm nhân vật, vật, sự kiện…" aria-label="Tìm trong Não truyện" autoFocus />
        </label>
        <button className="mb-cancel" type="button" onClick={() => { setSearching(false); setQuery('') }}>Huỷ</button>
      </> : <>
        <span className="mb-title"><span>Não truyện</span><span className="mb-count">{data && current !== null ? countText(data, current, query) : 'Đang tải Não truyện…'}</span></span>
        <button className="mb-icon" type="button" aria-label="Tìm trong Não truyện" onClick={() => setSearching(true)}><Icon name="search-normal-1" size={21} /></button>
      </>}
    </header>

    {brain.error != null && !data && <div className="mb-notice" role="alert"><span>{brain.error instanceof Error ? brain.error.message : 'Không tải được Não truyện.'}</span><button className="m-button secondary" type="button" onClick={() => { void brain.refresh() }}>Thử lại</button></div>}
    {!data && !brain.error && <div className="mb-loading" aria-busy="true" aria-label="Đang tải Não truyện">{[72, 100, 88, 96].map((width, index) => <Skeleton key={index} width={`${width}%`} height={index === 0 ? 64 : 48} />)}</div>}

    {data && current !== null && <>
      <section className="mb-chapter" aria-label="Chọn chương">
        <div className="mb-chapter-row">
          <span className="mb-chapter-mid">
            <span className="mb-chapter-n" title={`Chương ${current} · ${selectedChapter?.title ?? ''}`}>Chương <span className="num">{current}</span> <span className="mb-chapter-title">· {selectedChapter?.title ?? ''}</span></span>
            <span className="mb-chapter-note" title={head?.note}>{head?.note}</span>
          </span>
          {head && <span className={`mb-chapter-st ${head.chip.tone}`}><i aria-hidden="true" />{head.chip.label}</span>}
        </div>
        <div ref={cellsRef} className={`mb-cells${scrolls ? ' scroll' : ''}`} role="radiogroup" aria-label="Chọn chương">
          {chapterCells(data.chapters, current).map((cell) => <button key={cell.n} className={`mb-cell ${cell.tone}`} type="button" role="radio" aria-checked={cell.on} tabIndex={cell.on ? 0 : -1}
            aria-label={cell.aria} title={cell.aria} onClick={() => setCh(cell.n)} onKeyDown={radioGroupKeys}>
            <span className="num">{cell.n}</span>
          </button>)}
        </div>
      </section>

      <div className="mb-filters" role="tablist" aria-label="Lọc theo loại">
        {chips.map((chip) => <button key={chip.key} className={`m-chip${chip.on ? ' on' : ''}`} type="button" role="tab" aria-selected={chip.on} onClick={() => setFilter(chip.key)}>
          {chip.icon && <Icon name={chip.icon} size={15} style={{ color: chip.color }} />}
          {chip.label}
          <span className="mb-chip-count num">{chip.count}</span>
        </button>)}
      </div>

      <div className="mb-list">
        {groups.length === 0 && <div className="mb-empty">
          <Icon name="search-normal-1" size={26} />
          <span className="mb-empty-title">Không có gì khớp “{query}”</span>
          <span className="mb-empty-hint">Thử tên khác, hoặc chọn “Tất cả”.</span>
        </div>}
        {groups.map((group) => <section key={group.type} aria-label={group.label}>
          <div className="mb-group-head"><span className="mb-lbl">{group.label}</span><span className="mb-group-count num">{group.count}</span></div>
          <div className="mb-group">
            {group.items.map((item) => <button key={item.id} className="mb-ent" type="button" aria-haspopup="dialog" onClick={() => setSheetId(item.id)}>
              {item.isChar
                ? <CharAvatar name={item.name} ini={item.ini} kind={item.kind} hidden={item.hidden} size={38} />
                : <span className="mb-gl" aria-hidden="true" style={{ background: glyphFill(item.color) }}><Icon name={item.icon} size={18} style={{ color: item.color }} /></span>}
              <span className="mb-ent-copy">
                <span className="mb-ent-name">
                  <span>{item.name}</span>
                  {item.isNew && <span className="mb-new">mới</span>}
                  {item.changed && <span className="mb-chg">đổi ở ch. {current}</span>}
                </span>
                <span className="mb-ent-sub">{item.role}</span>
                {item.rel && <span className="mb-ent-rel">
                  <span className="mb-rel-line" aria-hidden="true" style={{ borderTopColor: item.relColor, borderTopStyle: item.relDashed ? 'dashed' : 'solid' }} />
                  <span className="mb-rel-text">{item.rel}</span>
                  {item.more > 0 && <span className="mb-more num">+{item.more}</span>}
                </span>}
              </span>
              <Icon name="arrow-right-2" size={16} className="mb-chev" />
            </button>)}
          </div>
        </section>)}
        <p className="mb-foot">{PHONE_FOOTNOTE}</p>
      </div>
    </>}

    <BottomSheet open={!!sheet} onClose={close} label={sheet ? `${sheet.name} tại chương ${current ?? ''}` : 'Chi tiết'} height="auto" style={{ maxHeight: '86%' }}>
      {sheet && <div className="mb-sheet">
        <div className="mb-sheet-head">
          {sheet.isChar
            ? <CharAvatar name={sheet.name} ini={sheet.ini} kind={sheet.kind} hidden={sheet.hidden} size={46} />
            : <span className="mb-gl" aria-hidden="true" style={{ background: glyphFill(sheet.color) }}><Icon name={sheet.icon} size={21} style={{ color: sheet.color }} /></span>}
          <span className="mb-sheet-title"><span>{sheet.name}</span><span className="mb-sheet-sub">{sheet.sub}</span></span>
          <button className="mb-sheet-close" type="button" aria-label="Đóng" onClick={close}><Icon name="add" size={18} style={{ transform: 'rotate(45deg)' }} /></button>
        </div>
        <div className="mb-sheet-body">
          <p className="mb-desc">{sheet.desc}</p>
          {sheet.notYet && <div className="mb-notyet">Chưa xuất hiện ở chương {current}. Có mặt từ chương {sheet.fromChapter}.</div>}
          <div>
            <span className="mb-lbl">Quan hệ tại chương {current}</span>
            {sheet.relations.length === 0
              ? <p className="mb-none">Chưa có quan hệ nào ở chương này.</p>
              : <div>{sheet.relations.map((relation, index) => <div className="mb-rrow" key={index}>
                <span className="mb-rel-line" aria-hidden="true" style={{ borderTopColor: relation.color, borderTopStyle: relation.dashed ? 'dashed' : 'solid' }} />
                <span className="mb-rrow-copy"><span className="mb-rrow-main"><span style={{ color: relation.color }}>{relation.label}</span> · {relation.other}</span><span className="mb-rrow-since">{relation.since}</span></span>
              </div>)}</div>}
          </div>
          {sheet.isChar && <div>
            <span className="mb-lbl">Xưng hô tại chương {current}</span>
            {sheet.address.length === 0
              ? <p className="mb-none">Chưa ghi xưng hô.</p>
              : <div>{sheet.address.map((row, index) => <div className="mb-xrow" key={index}>
                <span className="mb-xdir"><span className="mb-xfrom">{row.from}</span><Icon name="arrow-right-2" size={11} /><span>{row.to}</span></span>
                <span className="mb-xmain">gọi <b>“{row.call}”</b> · tự xưng <b>“{row.self}”</b></span>
                {row.was && <span className="mb-was">{row.was}</span>}
              </div>)}</div>}
          </div>}
          <div>
            <span className="mb-lbl">Diễn biến</span>
            <ol className="mb-hist">{sheet.events.map((event, index) => <li key={index} style={{ opacity: event.future ? 0.5 : 1 }}>
              <span className="num">Ch. {event.chapter}</span>
              <span>{event.text}{event.future ? ` · sau chương ${current}` : ''}</span>
            </li>)}</ol>
          </div>
          {writing && <div className="mb-lock"><Icon name="lock" size={15} style={{ marginTop: 2 }} /><span>{lockSentence(writing).replace(/\.$/, '')} xưng hô và quan hệ. Tạm dừng ở tab Viết để sửa.</span></div>}
          {sheet.isChar && <button type="button" className="mb-person" onClick={() => navigate(`${paths.story(project, story)}?tab=cast`)}>
            <Icon name="user" size={17} />Mở trang nhân vật
          </button>}
        </div>
      </div>}
    </BottomSheet>
  </div>
}
