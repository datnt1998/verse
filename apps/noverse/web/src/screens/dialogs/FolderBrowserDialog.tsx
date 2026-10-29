// GAP-34 · 09-auth-onboard.md §10.3 — bộ duyệt thư mục trong app (Onboard.dc.html › tweak `browse`).
// Dùng khi không có hộp chọn của hệ điều hành: truy cập từ xa / điện thoại, hoặc máy chủ trả 501/403.
// Desktop: hộp thoại kit 760×540 (nền đặc #18181A). ≤640px: trang đẩy kiểu MMore (danh sách Nơi → thư mục,
// nút “Thư mục mới” ở thanh trên, chân cố định có kết luận + “Dùng thư mục này”).
import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { ApiError, getFsPlaces, getSystem, inspectLibraryPath, listFsFolders, mkFsFolder, pickFolder } from '../../api/client'
import type { FsFolder, FsPlace, LibraryInspect, SystemInfo } from '../../api/types'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { useMediaQuery } from '../../shell/AppShell'
import { Dialog } from '@noverse/ui'
import { useEscapeLayer } from '@noverse/ui'
import { Field } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { footerView } from './libraryInspect'
import '../more/mobile/more.css'

export const SUGGESTED_FOLDER = 'Truyện Noverse'
const MOBILE_QUERY = '(max-width: 640px)'

const sepOf = (path: string) => (path.includes('/') ? '/' : '\\')
const segmentsUnder = (place: FsPlace, path: string) => path.slice(place.path.length).split(/[\\/]/).filter(Boolean)
const joinPath = (base: string, ...parts: string[]) => [base.replace(/[\\/]+$/, ''), ...parts].join(sepOf(base))

/** Nơi chứa path (cụ thể nhất) — dùng để mở bộ duyệt đúng chỗ khi đã có thư mục xuất phát. */
export function placeOf(places: FsPlace[], path: string): FsPlace | null {
  let best: FsPlace | null = null
  for (const place of places) {
    const inside = path === place.path || path.startsWith(place.path.replace(/[\\/]+$/, '') + sepOf(place.path))
    if (inside && (!best || place.path.length > best.path.length)) best = place
  }
  return best
}

/** Đường dẫn gợi ý “Tài liệu › Truyện Noverse”; rơi về Thư mục nhà nếu máy không có Tài liệu. */
export function suggestedPath(places: FsPlace[] | null): { parent: FsPlace; path: string } | null {
  const parent = places?.find((place) => place.id === 'documents') ?? places?.find((place) => place.id === 'home') ?? null
  return parent ? { parent, path: joinPath(parent.path, SUGGESTED_FOLDER) } : null
}

function rowMeta(row: FsFolder): string {
  if (row.kind === 'library') return `Thư viện Noverse · ${row.projects ?? 0} dự án`
  if (row.kind === 'readonly') return 'chỉ đọc'
  return row.items === 0 ? 'trống' : `${row.items} mục`
}

/** Dòng danh sách thư mục: huy hiệu `Noverse` / `chỉ đọc` + số mục. */
function badgeOf(row: FsFolder) {
  if (row.kind === 'library') return { text: 'Noverse', bg: 'rgba(201,166,245,0.12)', fg: '#DCC8F8' }
  if (row.kind === 'readonly') return { text: 'chỉ đọc', bg: 'rgba(255,154,162,0.1)', fg: '#FFB3BA' }
  return null
}
const rowIcon = (row: FsFolder) => ({
  name: row.kind === 'library' ? 'book-1' : 'folder-open',
  color: row.kind === 'library' ? '#C9A6F5' : row.kind === 'readonly' ? '#7E7E8A' : '#A0A0AB',
})

function nextFolderName(rows: FsFolder[] | null): string {
  const taken = new Set((rows ?? []).map((row) => row.name.toLowerCase()))
  if (!taken.has(SUGGESTED_FOLDER.toLowerCase())) return SUGGESTED_FOLDER
  for (let n = 2; ; n += 1) if (!taken.has(`${SUGGESTED_FOLDER} ${n}`.toLowerCase())) return `${SUGGESTED_FOLDER} ${n}`
}

const errorText = (error: unknown, fallback: string) => (error instanceof ApiError ? error.message : fallback)

type Browser = ReturnType<typeof useFolderBrowser>

function useFolderBrowser(start: string | undefined, mobile: boolean) {
  const [places, setPlaces] = useState<FsPlace[] | null>(null)
  const [placesError, setPlacesError] = useState('')
  const [place, setPlace] = useState<FsPlace | null>(null)
  const [path, setPath] = useState('')
  const [rows, setRows] = useState<FsFolder[] | null>(null)
  const [listError, setListError] = useState('')
  const [sel, setSel] = useState<string | null>(null)
  const [info, setInfo] = useState<LibraryInspect | null>(null)
  const [inspectError, setInspectError] = useState('')
  const [mkOpen, setMkOpen] = useState(false)
  const [mkName, setMkName] = useState('')
  const [mkError, setMkError] = useState('')
  const [mkBusy, setMkBusy] = useState(false)
  const autoSelected = useRef('')

  const enter = useCallback((nextPlace: FsPlace | null, nextPath: string) => {
    setPlace(nextPlace); setPath(nextPath); setSel(null); setMkOpen(false); setMkError('')
    setRows(null); setListError(''); setInfo(null)
  }, [])

  useEffect(() => {
    let alive = true
    getFsPlaces().then((list) => {
      if (!alive) return
      setPlaces(list)
      if (mobile) return // điện thoại bắt đầu ở danh sách Nơi
      const home = start ? placeOf(list, start) : null
      const first = home ?? list.find((item) => item.id === 'documents') ?? list[0] ?? null
      if (first) enter(first, home && start ? start : first.path)
    }).catch((error) => alive && setPlacesError(errorText(error, 'Không đọc được danh sách nơi lưu trên máy chủ.')))
    return () => { alive = false }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- khởi tạo một lần
  }, [])

  useEffect(() => {
    if (!place || !path) return
    const controller = new AbortController()
    listFsFolders(path, controller.signal).then((list) => {
      setRows(list)
      // Desktop: chọn sẵn thư mục “Truyện Noverse” (nếu có) như artboard; nơi khác không chọn gì → dùng chính thư mục đang mở.
      if (!mobile && autoSelected.current !== path) {
        autoSelected.current = path
        const suggested = list.find((row) => row.name === SUGGESTED_FOLDER)
        if (suggested) setSel(suggested.path)
      }
    }).catch((error) => {
      if (controller.signal.aborted) return
      setRows([])
      setListError(errorText(error, 'Không mở được thư mục này.'))
    })
    return () => controller.abort()
  }, [place, path, mobile])

  const target = mobile ? path : sel ?? path
  const selectedRow = useMemo(() => rows?.find((row) => row.path === sel) ?? null, [rows, sel])
  useEffect(() => {
    if (!place || !target) return
    const controller = new AbortController()
    setInfo(null); setInspectError('')
    const timer = window.setTimeout(() => {
      inspectLibraryPath(target, controller.signal).then(setInfo)
        .catch((error) => { if (!controller.signal.aborted) setInspectError(errorText(error, 'Không kiểm tra được thư mục này.')) })
    }, 120)
    return () => { window.clearTimeout(timer); controller.abort() }
  }, [place, target])

  const open = useCallback((folder: string) => { if (place) enter(place, folder) }, [enter, place])
  const up = useCallback(() => {
    if (!place) return false
    const segments = segmentsUnder(place, path)
    if (segments.length === 0) { enter(null, ''); return true }
    enter(place, joinPath(place.path, ...segments.slice(0, -1)))
    return true
  }, [enter, path, place])

  const startMk = () => { setMkName(nextFolderName(rows)); setMkError(''); setMkOpen(true) }
  const doMk = async () => {
    const name = mkName.trim()
    if (!name || mkBusy || !path) return
    setMkBusy(true); setMkError('')
    try {
      const made = await mkFsFolder(path, name)
      const list = await listFsFolders(path)
      setRows(list); setMkOpen(false)
      setSel(mobile ? null : made.path)
      if (mobile) enter(place, made.path)
    } catch (error) {
      setMkError(errorText(error, 'Không tạo được thư mục.'))
    } finally {
      setMkBusy(false)
    }
  }

  const blocked = !info || info.kind === 'readonly' || selectedRow?.kind === 'readonly'
  return {
    places, placesError, place, path, rows, listError, sel, setSel, info, inspectError, target, blocked,
    mkOpen, setMkOpen, mkName, setMkName, mkError, setMkError, mkBusy, startMk, doMk,
    enter, open, up, segments: place ? segmentsUnder(place, path) : [],
  }
}

function useLockBackground(enabled: boolean) {
  useEffect(() => {
    if (!enabled) return
    const root = document.getElementById('root')
    root?.setAttribute('inert', ''); root?.setAttribute('aria-hidden', 'true')
    return () => { root?.removeAttribute('inert'); root?.removeAttribute('aria-hidden') }
  }, [enabled])
}

export function FolderBrowserDialog({ start, onClose, onUse }: {
  /** Thư mục xuất phát (đường dẫn máy chủ); chỉ dùng nếu nằm trong một Nơi. */
  start?: string
  onClose: () => void
  onUse: (path: string, info: LibraryInspect) => void
}) {
  const mobile = useMediaQuery(MOBILE_QUERY)
  const browser = useFolderBrowser(start, mobile)
  const use = () => { if (browser.info && !browser.blocked) onUse(browser.target, browser.info) }
  return mobile ? <MobileBrowser b={browser} onClose={onClose} onUse={use} /> : <DesktopBrowser b={browser} onClose={onClose} onUse={use} />
}

function Conclusion({ b }: { b: Browser }) {
  const view = b.info ? footerView(b.info) : null
  const fg = view?.fg ?? '#7E7E8A'
  return <>
    {view ? <Icon name={view.icon} size={16} style={{ color: fg }} /> : b.inspectError ? <Icon name="warning-2" size={16} style={{ color: '#FF9AA2' }} /> : <Spinner size={14} />}
    <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
      <span style={{ fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{b.info?.display ?? ''}</span>
      <span aria-live="polite" style={{ fontSize: 12, color: b.inspectError ? '#FF9AA2' : fg }}>{view?.text ?? (b.inspectError || 'Đang kiểm tra…')}</span>
    </span>
  </>
}

function NewFolderRow({ b, mobile }: { b: Browser; mobile?: boolean }) {
  // Dòng “thư mục mới” là lớp trên cùng khi mở: Esc = Huỷ dòng này, không đóng hộp chọn thư mục.
  useEscapeLayer(b.mkOpen, () => b.setMkOpen(false))
  const submit = (event: KeyboardEvent) => {
    if (event.key === 'Enter') { event.preventDefault(); void b.doMk() }
    if (event.key === 'Escape') { event.stopPropagation(); b.setMkOpen(false) }
  }
  return <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, height: mobile ? 54 : 46, padding: mobile ? '0 14px' : '0 10px' }}>
      <Icon name="folder-open" size={18} style={{ color: '#C9A6F5' }} />
      <Field type="text" value={b.mkName} autoFocus onChange={(event) => { b.setMkName(event.target.value); b.setMkError('') }} onKeyDown={submit}
        aria-label="Tên thư mục mới" aria-invalid={!!b.mkError || undefined} style={{ height: 32, fontSize: 13.5, flex: '1 1 auto' }} />
      <button className="primary hov-primary" type="button" onClick={() => void b.doMk()} disabled={b.mkBusy || !b.mkName.trim()}
        style={{ height: 32, padding: '0 12px', borderRadius: 8, border: 'none', background: '#EDEDEF', color: '#0B0B0D', fontSize: 12.5, fontWeight: 600 }}>Tạo</button>
    </div>
    {b.mkError && <span role="alert" style={{ padding: mobile ? '0 14px 8px 42px' : '0 10px 0 38px', fontSize: 12, color: '#FF9AA2' }}>{b.mkError}</span>}
  </div>
}

function DesktopBrowser({ b, onClose, onUse }: { b: Browser; onClose: () => void; onUse: () => void }) {
  const titleRef = useRef<HTMLSpanElement>(null)
  useEffect(() => { // Dialog chỉ gắn aria-labelledby khi có `title`; tiêu đề ở đây là phần đầu tuỳ biến của artboard.
    const dialog = titleRef.current?.closest('[role="dialog"]')
    if (dialog && titleRef.current) dialog.setAttribute('aria-labelledby', titleRef.current.id)
  }, [])
  const crumbs = b.place ? [b.place.label, ...b.segments] : []
  return <Dialog width={760} scrim="strong" onClose={onClose} style={{ height: 540, maxHeight: '100%', background: '#18181A', overflow: 'hidden' }}>
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '14px 16px', borderBottom: '1px solid rgba(255,255,255,0.07)' }}>
      <Icon name="folder-open" size={18} style={{ color: '#C9A6F5' }} />
      <span ref={titleRef} id="fb-t" style={{ flex: '1 1 auto', fontSize: 15, fontWeight: 600 }}>Chọn thư mục lưu truyện</span>
      <button className="hov-ghost" type="button" onClick={onClose} aria-label="Đóng" style={{ width: 30, height: 30, borderRadius: 8, border: 'none', background: 'transparent', color: '#A0A0AB', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Icon name="close-circle-bulk" size={17} />
      </button>
    </div>
    <div style={{ flex: '1 1 auto', minHeight: 0, display: 'flex' }}>
      <nav aria-label="Vị trí" style={{ flex: '0 0 190px', padding: '10px 8px', borderRight: '1px solid rgba(255,255,255,0.06)', display: 'flex', flexDirection: 'column', gap: 2, overflowY: 'auto' }}>
        {b.places?.map((item) => {
          const on = item.id === b.place?.id
          return <button key={item.id} className="hov-row" type="button" aria-current={on ? 'true' : undefined} onClick={() => b.enter(item, item.path)}
            style={{ display: 'flex', alignItems: 'center', gap: 9, height: 34, padding: '0 10px', border: 'none', borderRadius: 8, background: on ? '#222225' : 'transparent', color: on ? '#EDEDEF' : '#A0A0AB', fontSize: 13, textAlign: 'left' }}>
            <Icon name={item.icon} size={16} /><span style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item.label}</span>
          </button>
        })}
      </nav>
      <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '10px 14px', fontSize: 13, color: '#A0A0AB', borderBottom: '1px solid rgba(255,255,255,0.05)' }}>
          {crumbs.map((text, index) => {
            const last = index === crumbs.length - 1
            const go = () => b.place && b.enter(b.place, index === 0 ? b.place.path : joinPath(b.place.path, ...b.segments.slice(0, index)))
            return <span key={index} style={{ display: 'flex', alignItems: 'center', gap: 4, minWidth: 0 }}>
              {last ? <span style={{ color: '#EDEDEF' }}>{text}</span>
                : <button type="button" onClick={go} style={{ border: 'none', background: 'transparent', padding: 0, color: '#A0A0AB', fontSize: 13 }}>{text}</button>}
              {!last && <Icon name="arrow-right-2" size={12} style={{ color: '#5E5E68' }} />}
            </span>
          })}
          <span style={{ flex: '1 1 auto' }} />
          <button className="hov-ghost" type="button" onClick={b.startMk} disabled={!b.place}
            style={{ height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#C9C9D1', fontSize: 12, display: 'flex', alignItems: 'center', gap: 5 }}>
            <Icon name="add" size={13} />Thư mục mới
          </button>
        </div>
        <div className="scroll" style={{ flex: '1 1 auto', minHeight: 0, padding: '6px 8px' }}>
          {b.placesError && <p role="alert" style={{ margin: 12, fontSize: 13, color: '#FF9AA2' }}>{b.placesError}</p>}
          {b.listError && <p role="alert" style={{ margin: 12, fontSize: 13, color: '#FF9AA2' }}>{b.listError}</p>}
          {b.place && !b.rows && !b.listError && <div style={{ display: 'flex', justifyContent: 'center', padding: 24 }}><Spinner size={16} /></div>}
          {b.rows?.map((row) => {
            const on = row.path === b.sel
            const badge = badgeOf(row)
            const icon = rowIcon(row)
            return <button key={row.path} className="hov-row" type="button" aria-pressed={on} onClick={() => b.setSel(row.path)} onDoubleClick={() => b.open(row.path)}
              onKeyDown={(event) => { if (event.key === 'ArrowRight') { event.preventDefault(); b.open(row.path) } }}
              style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 12, height: 46, padding: '0 10px', border: 'none', borderRadius: 9, background: on ? 'rgba(201,166,245,0.1)' : 'transparent', color: row.kind === 'readonly' ? '#8E8E99' : '#EDEDEF', textAlign: 'left' }}>
              <Icon name={icon.name} size={18} style={{ color: icon.color }} />
              <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{row.name}</span>
              {badge && <span style={{ height: 20, padding: '0 7px', borderRadius: 6, background: badge.bg, color: badge.fg, fontSize: 11, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap' }}>{badge.text}</span>}
              <span style={{ fontSize: 12, color: '#6E6E78', whiteSpace: 'nowrap' }}>{rowMeta(row)}</span>
            </button>
          })}
          {b.rows && b.rows.length === 0 && !b.listError && !b.mkOpen && <p style={{ margin: 12, fontSize: 13, color: '#7E7E8A' }}>Thư mục này chưa có thư mục con.</p>}
          {b.mkOpen && <NewFolderRow b={b} />}
        </div>
      </div>
    </div>
    <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px', borderTop: '1px solid rgba(255,255,255,0.07)' }}>
      {b.place ? <Conclusion b={b} /> : <span style={{ flex: '1 1 auto' }} />}
      <button className="hov-ghost" type="button" onClick={onClose} style={{ height: 34, padding: '0 14px', borderRadius: 9, border: 'none', background: 'transparent', color: '#A0A0AB', fontSize: 13 }}>Huỷ</button>
      <button className="primary hov-primary" type="button" onClick={onUse} disabled={b.blocked}
        style={{ height: 34, padding: '0 16px', borderRadius: 9, border: 'none', background: b.blocked ? 'rgba(255,255,255,0.08)' : '#EDEDEF', color: b.blocked ? '#7E7E8A' : '#0B0B0D', fontSize: 13, fontWeight: 600 }}>Dùng thư mục này</button>
    </div>
  </Dialog>
}

/** ≤640px: trang đẩy kiểu MMore (.mm-push): Nơi → thư mục; chân cố định như .mm-cta. */
function MobileBrowser({ b, onClose, onUse }: { b: Browser; onClose: () => void; onUse: () => void }) {
  useLockBackground(true)
  const ref = useRef<HTMLDivElement>(null)
  useEscapeLayer(true, onClose)
  useEffect(() => {
    ref.current?.querySelector<HTMLElement>('button')?.focus()
  }, [])
  const atPlaces = !b.place
  const title = atPlaces ? 'Chọn thư mục lưu truyện' : b.segments.length ? b.segments[b.segments.length - 1] : b.place!.label
  const goBack = () => { if (!b.up()) onClose() }
  return createPortal(
    <div ref={ref} className="mm-root" role="dialog" aria-modal="true" aria-label="Chọn thư mục lưu truyện"
      style={{ position: 'fixed', inset: 0, zIndex: 40, padding: 0, background: '#0B0B0D', display: 'flex', flexDirection: 'column', paddingTop: 'var(--m-top)' }}>
      <header style={{ flex: '0 0 auto', height: 52, padding: '0 12px 0 6px', display: 'flex', alignItems: 'center', gap: 10 }}>
        <button type="button" onClick={goBack} style={{ height: 44, padding: '0 8px 0 4px', border: 'none', background: 'transparent', color: '#C9A6F5', fontSize: 16, display: 'flex', alignItems: 'center', gap: 2, flex: '0 0 auto', minWidth: 72 }}>
          {atPlaces ? <span style={{ paddingLeft: 8 }}>Huỷ</span> : <><Icon name="arrow-left-2" size={22} />Quay lại</>}
        </button>
        <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 17, fontWeight: 700, letterSpacing: '-0.01em', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', textAlign: 'center' }}>{title}</span>
        {atPlaces ? <span style={{ width: 72, flex: '0 0 auto' }} />
          : <button type="button" onClick={b.startMk} style={{ height: 44, padding: 0, border: 'none', background: 'transparent', color: '#C9A6F5', fontSize: 15, flex: '0 0 auto', minWidth: 72, textAlign: 'right', whiteSpace: 'nowrap' }}>Thư mục mới</button>}
      </header>
      <div key={b.place ? b.path : 'places'} className="mm-push" style={{ flex: '1 1 auto', minHeight: 0, overflowY: 'auto', overflowX: 'hidden', paddingBottom: b.place ? 150 : 24 }}>
        {b.placesError && <p role="alert" style={{ margin: '12px 30px', fontSize: 13, color: '#FF9AA2' }}>{b.placesError}</p>}
        {atPlaces && <>
          <div className="mm-heading">Vị trí</div>
          <div className="mm-group">
            {!b.places && !b.placesError && <div style={{ display: 'flex', justifyContent: 'center', padding: 18 }}><Spinner size={16} /></div>}
            {b.places?.map((item) => <button key={item.id} type="button" className="mm-row" onClick={() => b.enter(item, item.path)}>
              <Icon name={item.icon} size={20} style={{ color: '#A0A0AB' }} />
              <span className="mm-copy"><span>{item.label}</span></span>
              <Icon name="arrow-right-2" size={18} className="mm-chevron" />
            </button>)}
          </div>
        </>}
        {!atPlaces && <>
          <div className="mm-heading"><span>Thư mục con</span></div>
          {b.listError && <p role="alert" style={{ margin: '0 30px 12px', fontSize: 13, color: '#FF9AA2' }}>{b.listError}</p>}
          <div className="mm-group">
            {b.mkOpen && <NewFolderRow b={b} mobile />}
            {!b.rows && !b.listError && <div style={{ display: 'flex', justifyContent: 'center', padding: 18 }}><Spinner size={16} /></div>}
            {b.rows?.map((row) => {
              const badge = badgeOf(row)
              const icon = rowIcon(row)
              return <button key={row.path} type="button" className="mm-row" onClick={() => b.open(row.path)}>
                <Icon name={icon.name} size={20} style={{ color: icon.color }} />
                <span className="mm-copy">
                  <span style={{ color: row.kind === 'readonly' ? '#8E8E99' : undefined }}>{row.name}</span>
                  <span className="mm-sub">{rowMeta(row)}</span>
                </span>
                {badge && <span style={{ height: 20, padding: '0 7px', borderRadius: 6, background: badge.bg, color: badge.fg, fontSize: 11, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap', flex: '0 0 auto' }}>{badge.text}</span>}
                <Icon name="arrow-right-2" size={18} className="mm-chevron" />
              </button>
            })}
          </div>
          {b.rows && b.rows.length === 0 && !b.listError && <p style={{ margin: '14px 30px 0', fontSize: 12.5, color: '#7E7E8A', textAlign: 'center' }}>Thư mục này chưa có thư mục con.</p>}
        </>}
      </div>
      {b.place && <div className="mm-cta" style={{ position: 'absolute', left: 0, right: 0, bottom: 0, zIndex: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}><Conclusion b={b} /></div>
        <button type="button" className="mm-primary" onClick={onUse} disabled={b.blocked}>Dùng thư mục này</button>
      </div>}
    </div>,
    document.body,
  )
}

/**
 * “Chọn thư mục khác…” (§10.3): máy chủ truy cập trực tiếp → hộp chọn của hệ điều hành
 * (`{cancelled}` → không làm gì; 501/403 → bộ duyệt trong app); điện thoại / máy khác → bộ duyệt ngay.
 * `onPicked` nhận đường dẫn + kết luận kiểm tra (kể cả `readonly`, để UI giải thích).
 */
export function useFolderPicker({ start, onPicked }: { start?: string; onPicked: (path: string, info: LibraryInspect) => void }) {
  const system = useResource(keys.system(), ({ signal }) => getSystem(signal), { staleTime: 60000 })
  const [picking, setPicking] = useState(false)
  const [browsing, setBrowsing] = useState(false)
  const [error, setError] = useState('')

  const accept = useCallback(async (path: string) => {
    try { onPicked(path, await inspectLibraryPath(path)) }
    catch (cause) { setError(errorText(cause, 'Không kiểm tra được thư mục này.')) }
  }, [onPicked])

  const pick = useCallback(async () => {
    setError('')
    // canReveal chưa về kịp (bấm rất nhanh sau khi mở màn) → chờ một lần rồi mới quyết,
    // tránh mở bộ duyệt trong app trên chính máy chủ. Hỏi thất bại → vẫn bộ duyệt trong app.
    let canReveal = system.data?.canReveal
    if (canReveal === undefined) canReveal = ((await system.refresh()) as SystemInfo | undefined)?.canReveal
    if (!canReveal) { setBrowsing(true); return }
    setPicking(true)
    try {
      const result = await pickFolder(start)
      if (result.path) await accept(result.path)
    } catch (cause) {
      if (cause instanceof ApiError && (cause.status === 501 || cause.status === 403)) setBrowsing(true)
      else setError(errorText(cause, 'Không mở được hộp chọn thư mục trên máy chủ.'))
    } finally {
      setPicking(false)
    }
  }, [accept, start, system.data?.canReveal, system.refresh])

  const dialog = browsing
    ? <FolderBrowserDialog start={start} onClose={() => setBrowsing(false)} onUse={(path, info) => { setBrowsing(false); onPicked(path, info) }} />
    : null
  return { pick, picking, error, dialog }
}
