// MTablet.dc.html › `.pop role=listbox aria-label="Lệnh"`: popover lệnh neo trên ô chat (không phải sheet).
// Cùng dữ liệu & phím ↑↓/Enter với SlashPopover bản máy tính (commands/model + store); khi đã chọn lệnh và đang gõ
// tham số/giá trị thì dùng lại SlashPopover (artboard chỉ vẽ danh sách lệnh).
import { useEffect, useMemo, useRef } from 'react'
import { CMD_UI } from '../i18n/commands.vi'
import { paths } from '../router'
import { buildSlashView, slashContext } from '../commands/model'
import { SlashPopover, type SlashPopoverProps } from '../commands/SlashPopover'
import { useCommands, useSelection } from '../commands/store'
import { rememberHelpTarget } from '../commands/target'

export function TabletSlashPopover(props: SlashPopoverProps) {
  const { value, onChange, snapshot, recentPaths } = props
  const { specs } = useCommands()
  const [selected, setSelected] = useSelection(value)
  const view = useMemo(
    () => (specs.length ? buildSlashView(value, slashContext(snapshot ?? null, specs, recentPaths), selected) : null),
    [specs, value, snapshot, recentPaths, selected],
  )
  const listRef = useRef<HTMLDivElement>(null)
  useEffect(() => { listRef.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }) }, [selected, view?.items.length])
  if (!view) return null
  if (view.mode !== 'list') return <SlashPopover {...props} />
  if (!view.items.length) return null
  return <div className="tablet-pop tablet-cmd" role="listbox" aria-label="Lệnh">
    <div className="tablet-cmd-head"><span>LỆNH · {view.items.length}</span><a href={`#${paths.help()}`} onClick={() => rememberHelpTarget()}>Hướng dẫn</a></div>
    <div className="tablet-cmd-list" ref={listRef}>
      {view.items.map((item) => {
        const on = item.index === selected
        const idle = item.tag === CMD_UI.tagIdle && !!snapshot?.isRunning
        return <button type="button" role="option" aria-selected={on} key={`${item.index}:${item.key}`} className={`tablet-prow${on ? ' hi' : ''}`} onMouseEnter={() => setSelected(item.index)} onClick={() => onChange(item.insert)}>
          <span className="tablet-cmd-slash">{item.before}{item.match}{item.after}</span>
          <span className="tablet-cmd-copy"><span>{item.desc}</span>{idle && <small>Cần rảnh · đang viết</small>}</span>
        </button>
      })}
    </div>
    <div className="tablet-cmd-foot">↑↓ chọn · Enter chạy · có bàn phím ngoài thì dùng như máy tính</div>
  </div>
}
