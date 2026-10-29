// Gallery tab “Menu & tài khoản” — Kit.dc.html › sec.menus (menu tài khoản + menu thao tác ⋯ có menu con).
import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { Avatar, Kbd } from '@noverse/ui'
import { AgentOrb } from '@noverse/ui'
import { ContextMenu, type ActionMenuItem } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import '@noverse/ui/kit.css'

type AccountItem = { sep: true } | { lbl: string } | { label: string; icon?: string; avatar?: string; sub?: string; count?: string; kbd?: string; chev?: boolean; danger?: boolean }
const ACCOUNT_MENU: AccountItem[] = [
  { icon: 'mobile', label: 'Đăng nhập điện thoại bằng QR' }, { icon: 'devices', label: 'Thiết bị đang đăng nhập', count: '3' }, { icon: 'password-check', label: 'Đổi mật khẩu' }, { icon: 'profile-2user', label: 'Quản lý tài khoản', count: '4' },
  { sep: true }, { icon: 'setting-2', label: 'Cài đặt', kbd: '⌘ ,' }, { icon: 'message-question', label: 'Hướng dẫn lệnh', kbd: '?' },
  { sep: true }, { lbl: 'Chuyển tài khoản' }, { avatar: 'L', label: 'Linh', sub: 'Người viết · cần nhập mật khẩu', chev: true }, { icon: 'user-add', label: 'Đăng nhập tài khoản khác' },
  { sep: true }, { icon: 'logout', label: 'Đăng xuất', danger: true },
]
const ACCOUNT_ACTIONS = ACCOUNT_MENU.filter((item): item is Extract<AccountItem, { label: string }> => 'label' in item)

const CHAPTERS = ['Vai của tôi là đồ canh', 'Chiếc cặp ở giữa bàn', 'Hai chiếc bát trên bàn', 'Bảy mươi ba giây', 'Tôi cướp vai cái nồi à?', 'Chiếc vỏ ổ cứng', 'Dòng tên bị gạch', 'Tên tôi trên màn hình']
const WORDS = ['3.104', '3.390', '2.980', '3.251', '3.012', '3.420', '2.877', '3.102']
const VOLUMES: [string, string][] = [['Tập 1 · Kịch bản bị cướp', '24 ch'], ['Tập 2 · Sân khấu thật', '0 ch'], ['Tập 3 · Bản gốc', '0 ch']]

const boxStyle: CSSProperties = { position: 'relative', height: 640, borderRadius: 18, border: '1px solid rgba(255,255,255,0.07)', background: '#0D0D0F' }
const captionStyle: CSSProperties = { fontSize: 12, color: '#7E7E8A', padding: '0 4px' }
const pickedStyle: CSSProperties = { position: 'absolute', left: 16, bottom: 16, height: 32, padding: '0 12px', borderRadius: 999, background: '#1C1C1F', border: '1px solid rgba(255,255,255,0.1)', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 8 }

function Picked({ children }: { children: ReactNode }) {
  return <span className="kit-toast-in" role="status" style={pickedStyle}><Icon name="arrow-right-2" size={14} style={{ color: '#C9A6F5' }} />{children}</span>
}

function AccountMenuDemo() {
  const [open, setOpen] = useState(true)
  const [index, setIndex] = useState(0)
  const [picked, setPicked] = useState('')
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open])
  const choose = (label: string) => { setOpen(false); setPicked(label) }
  const onKeyDown = (event: React.KeyboardEvent) => {
    const last = ACCOUNT_ACTIONS.length
    if (event.key === 'ArrowDown') { event.preventDefault(); if (!open) { setOpen(true); setIndex(0) } else setIndex((index + 1) % last) }
    else if (event.key === 'ArrowUp') { event.preventDefault(); setIndex((index - 1 + last) % last) }
    else if (event.key === 'Enter' && open) { event.preventDefault(); choose(ACCOUNT_ACTIONS[index].label) }
    else if (event.key === 'Escape') setOpen(false)
  }
  let action = -1
  return <div style={boxStyle}>
    <div style={{ height: 52, display: 'flex', alignItems: 'center', gap: 10, padding: '0 14px', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
      <span aria-hidden="true" style={{ display: 'flex' }}><AgentOrb agent="architect" size={12} /><AgentOrb agent="writer" size={12} style={{ marginLeft: -3 }} /></span>
      <span style={{ fontSize: 13, color: '#A0A0AB' }}>Workspace</span><span style={{ flex: 1 }} />
      <span className="hov-ghost" style={{ width: 30, height: 30, borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#A0A0AB' }}><Icon name="setting-2" size={17} /></span>
      <div ref={root} style={{ position: 'relative', zIndex: open ? 30 : undefined }}>
        <button type="button" onClick={() => setOpen(!open)} onKeyDown={onKeyDown} aria-haspopup="menu" aria-expanded={open} aria-label="Tài khoản: Dat" style={{ width: 32, height: 32, borderRadius: '50%', border: 'none', padding: 0, background: 'transparent', boxShadow: open ? '0 0 0 2px #0D0D0F, 0 0 0 4px rgba(201,166,245,0.6)' : 'none', transition: 'box-shadow .15s' }}><Avatar name="Dat" size={32} style={{ fontSize: 12.5 }} /></button>
        {open && <div className="kit-pop" role="menu" aria-label="Tài khoản" style={{ position: 'absolute', right: 0, top: 'calc(100% + 8px)', width: 320, transformOrigin: 'top right' }}>
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', padding: '8px 8px 10px' }}>
            <Avatar name="Dat" size={40} style={{ fontSize: 15 }} />
            <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
              <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}><span style={{ fontSize: 14, fontWeight: 600 }}>Dat</span><span style={{ height: 19, padding: '0 6px', borderRadius: 5, background: 'rgba(201,166,245,0.12)', color: '#DCC8F8', fontSize: 10.5, display: 'flex', alignItems: 'center', gap: 4 }}><Icon name="crown" size={11} />Quản trị</span></span>
              <span className="mono" style={{ fontSize: 11.5, color: '#7E7E8A' }}>@dat</span>
            </span>
          </div>
          <div style={{ margin: '0 8px 6px', padding: '8px 10px', borderRadius: 10, background: 'rgba(255,255,255,0.03)', display: 'flex', flexDirection: 'column', gap: 6 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 11.5, color: '#A0A0AB' }}><span style={{ display: 'flex', gap: 6, alignItems: 'center' }}><Icon name="monitor" size={13} />Phiên trên Mac · Chrome</span><span className="num">còn 30 ngày</span></div>
            <span style={{ height: 3, borderRadius: 2, background: 'rgba(255,255,255,0.07)', overflow: 'hidden' }}><span style={{ display: 'block', width: '100%', height: '100%', background: '#A6EDC1' }} /></span>
          </div>
          {ACCOUNT_MENU.map((item, i) => {
            if ('sep' in item) return <div key={i} role="separator" className="kit-separator" />
            if ('lbl' in item) return <span key={i} className="lbl" style={{ display: 'block', fontSize: 10.5, padding: '6px 10px 2px' }}>{item.lbl}</span>
            const mine = ++action
            return <button key={i} type="button" role="menuitem" className={`kit-option${item.danger ? ' kit-danger' : ''}`} data-hi={index === mine || undefined} onClick={() => choose(item.label)} onMouseEnter={() => { if (index !== mine) setIndex(mine) }}>
              {item.avatar && <Avatar name={item.avatar} size={24} style={{ fontSize: 11 }} />}
              {item.icon && <Icon name={item.icon} size={16} style={{ color: item.danger ? '#FF9FA8' : '#A0A0AB' }} />}
              <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' }}><span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.label}</span>{item.sub && <span style={{ fontSize: 11.5, color: '#7E7E8A' }}>{item.sub}</span>}</span>
              {item.count && <span className="num" style={{ minWidth: 20, height: 20, padding: '0 6px', borderRadius: 6, background: 'rgba(255,255,255,0.06)', fontSize: 11, color: '#A0A0AB', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>{item.count}</span>}
              {item.kbd && <Kbd>{item.kbd}</Kbd>}
              {item.chev && <Icon name="arrow-right-2" size={14} style={{ color: '#7E7E8A' }} />}
            </button>
          })}
        </div>}
      </div>
    </div>
    {picked && !open && <Picked>Đã chọn: {picked}</Picked>}
  </div>
}

function RowMenuDemo({ onDanger }: { onDanger: () => void }) {
  const [picked, setPicked] = useState('')
  return <div style={{ ...boxStyle, padding: 14 }}>
    <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      {CHAPTERS.map((title, i) => {
        const chapter = i + 17
        const done = (text: string) => () => setPicked(text)
        const items: ActionMenuItem[] = [
          { id: 'read', label: 'Mở trong Trình đọc', icon: <Icon name="book-1" size={16} />, kbd: '↵', onSelect: done(`Mở trong Trình đọc · chương ${chapter}`) },
          { id: 'listen', label: 'Nghe thử chương', icon: <Icon name="play" size={16} />, onSelect: done(`Nghe thử chương · chương ${chapter}`) },
          { id: 'rewrite', label: 'Nhờ viết lại chương…', icon: <Icon name="magic-star" size={16} />, onSelect: done(`Nhờ viết lại chương… · chương ${chapter}`) },
          { id: 'copy', label: 'Chép đường dẫn tệp', icon: <Icon name="copy" size={16} />, onSelect: done(`Chép đường dẫn tệp · chương ${chapter}`) },
          { id: 'volume', label: 'Chuyển sang tập', icon: <Icon name="layer" size={16} />, children: [
            ...VOLUMES.map(([label, meta], v) => ({ id: `v${v}`, label, meta, selected: v === 0, icon: <Icon name="layer" size={15} style={{ color: '#7E7E8A' }} />, onSelect: done(`Chuyển chương ${chapter} sang ${label.split(' · ')[0]}`) })),
            { id: 'new', label: 'Tập mới…', meta: '', selected: false, icon: <Icon name="add" size={15} style={{ color: '#C9A6F5' }} />, onSelect: done('Tạo tập mới') },
          ] },
          { id: 'delete', label: 'Xoá bản nháp…', icon: <Icon name="trash" size={16} />, danger: true, onSelect: onDanger },
        ]
        return <ContextMenu key={chapter} className="kit-gallery-row" anchorToRow actionProps={i === 4 ? { label: 'Thao tác cho chương', defaultOpen: true, defaultSubmenu: 'volume', onOpen: () => setPicked('') } : { label: 'Thao tác cho chương', onOpen: () => setPicked('') }} items={items} style={{ gap: 12 }}>
          <span className="mono num" style={{ fontSize: 12, color: '#5E5E68', width: 22 }}>{chapter}</span>
          <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13, color: '#D6D6DC', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{title}</span>
          <span className="num" style={{ fontSize: 11.5, color: '#7E7E8A' }}>{WORDS[i]} chữ</span>
        </ContextMenu>
      })}
    </div>
    {picked && <Picked>{picked}</Picked>}
  </div>
}

export function MenusGallery({ onDanger }: { onDanger: () => void }) {
  return <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 24, alignItems: 'start' }}>
    <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <span className="lbl" style={{ padding: '0 4px' }}>Menu tài khoản · bấm ảnh đại diện</span>
      <AccountMenuDemo />
      <span style={captionStyle}>Mở: bấm hoặc <Kbd>↓</Kbd> khi đang chọn ảnh đại diện · <Kbd>↑</Kbd> <Kbd>↓</Kbd> di chuyển · <Kbd>Enter</Kbd> chọn · <Kbd>Esc</Kbd> đóng. Đăng xuất không cần hỏi lại — đăng nhập lại rất dễ.</span>
    </section>
    <section style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <span className="lbl" style={{ padding: '0 4px' }}>Menu thao tác ⋯ · có menu con</span>
      <RowMenuDemo onDanger={onDanger} />
      <span style={captionStyle}>Nút ⋯ chỉ hiện khi rê chuột vào dòng (luôn hiện trên màn cảm ứng). Chuột phải vào dòng mở đúng menu này. Mục nguy hiểm ở cuối, màu đỏ nhạt, mở hộp xác nhận. Menu con tự lật sang trái khi sát mép phải.</span>
    </section>
  </div>
}
