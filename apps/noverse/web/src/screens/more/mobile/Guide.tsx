import { useState } from 'react'
import type { CommandSpec } from '../../../api/types'
import { composeTokens, emptyValues, groupsOf, matchesQuery, usageTokens } from '../../../commands/model'
import { useCommands } from '../../../commands/store'
import { CMD_COLOR } from '../../../commands/tokens'
import { copyOf, paramCopy } from '../../../i18n/commands.vi'
import { t } from '../../../i18n/vi'
import { Icon } from '@noverse/ui'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

const SHORT: Record<string, string> = { help: 'Hướng dẫn lệnh', model: 'Đổi model trợ lý', config: 'Nhà cung cấp AI', diag: 'Khám sức khoẻ truyện', review: 'Duyệt từng chương', next: 'Viết chương tiếp', start: 'Tạo truyện từ tệp', import: 'Nhập truyện có sẵn', reopen: 'Mở lại truyện đã xong', cocreate: 'Bàn hướng đi tới', simulate: 'Học văn phong', importsim: 'Nhập hồ sơ văn phong', sync: 'Nhận chỗ sửa tay', export: 'Xuất TXT / EPUB' }
const DESKTOP: Record<string, true> = { config: true, start: true, simulate: true, importsim: true }
const GUIDE_ORDER = ['start', 'import', 'review', 'next', 'cocreate', 'sync', 'diag', 'simulate', 'importsim', 'export', 'reopen', 'model', 'config', 'help']
export const desktopCommand = (name: string) => !!DESKTOP[name]

export function MobileGuide({ command, onPick, onUse, onOpenNewStory }: { command: string | null; onPick: (name: string) => void; onUse: (line: string) => void; onOpenNewStory: () => void }) {
  const registry = useCommands()
  const [query, setQuery] = useState('')
  const spec = registry.specs.find((item) => item.name === command)
  return <div className="mm-guide mm-root">
    <PanelRefreshBar resourceKeys={[[ 'system', 'commands' ]]} updatedAt={registry.updatedAt} />
    {registry.state === 'loading' && <div className="mm-skeleton" role="status" aria-label="Đang tải hướng dẫn"><span /><span /><span /></div>}
    {registry.error && <div className="mm-error" role="alert">{registry.error}<button onClick={registry.retry}>Thử lại</button></div>}
    {command ? spec && <GuideDetail spec={spec} onPick={onPick} onUse={onUse} onOpenNewStory={onOpenNewStory} /> : <>
      <div className="mm-guide-search"><Icon name="search-normal-1" size={18} /><input className="kit-frame mm-field" type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm lệnh, ví dụ: xuất, model…" aria-label="Tìm lệnh" /></div>
      {groupsOf(registry.specs.filter((item) => matchesQuery(item, query))).map((group) => <section key={group.key}><div className="mm-heading">{group.label.toLocaleUpperCase('vi')}</div><div className="mm-group">{[...group.specs].sort((a, b) => GUIDE_ORDER.indexOf(a.name) - GUIDE_ORDER.indexOf(b.name)).map((item) => <button key={item.name} className="mm-row mm-guide-row" data-desktop={desktopCommand(item.name)} type="button" onClick={() => onPick(item.name)}><span className="mm-guide-slash mm-mono">/{item.name}</span><span className="mm-copy"><span>{SHORT[item.name] || copyOf(item.name)?.title || item.name}</span>{(desktopCommand(item.name) || item.needsIdle) && <span className="mm-sub mm-idle">{desktopCommand(item.name) ? 'Chỉ trên máy tính' : 'Cần hệ thống rảnh'}</span>}</span><Icon name="arrow-right-2" size={16} className="mm-chevron" /></button>)}</div></section>)}
      {registry.state === 'ready' && !registry.specs.some((item) => matchesQuery(item, query)) && <p className="mm-guide-empty">Không có lệnh nào khớp “{query}”.</p>}
      <p className="mm-note mm-center">Gõ <span className="mm-check mm-mono">/</span> trong ô chat ở màn Viết để chạy lệnh.</p>
    </>}
    {command && registry.state === 'ready' && !spec && <p className="mm-note">Không tìm thấy lệnh /{command}. Chọn một lệnh khác từ danh sách.</p>}
  </div>
}

function GuideDetail({ spec, onPick, onUse, onOpenNewStory }: { spec: CommandSpec; onPick: (name: string) => void; onUse: (line: string) => void; onOpenNewStory: () => void }) {
  const copy = copyOf(spec.name)
  const desk = desktopCommand(spec.name)
  // Gói AR: /start không gõ được trên điện thoại nữa — Truyện mới (M11) đã có ngăn “Bắt đầu từ tệp”.
  const startHere = spec.name === 'start'
  return <article>
    <div className="mm-guide-intro"><div><span className="mm-guide-name mm-mono">/{spec.name}</span>{spec.aliases.map((alias) => <span className="mm-guide-badge mm-mono" key={alias}>/{alias}</span>)}{spec.needsIdle && <span className="mm-guide-badge mm-idle">Cần hệ thống rảnh</span>}</div><h1>{copy?.title || spec.usage}</h1><p>{copy?.lead}</p>{startHere ? <p className="mm-note">{t.newStory.phoneOnly}</p> : desk && <p className="mm-note">Chỉ trên máy tính — mở Noverse trên máy tính để dùng lệnh này.</p>}</div>
    <div className="mm-heading">CÁCH GÕ</div><div className="mm-tokenbox">{usageTokens(spec).map((token, index) => <span key={index} style={{ color: CMD_COLOR[token.tone] }}>{index ? ' ' : ''}{token.text}</span>)}</div>
    {spec.params.length > 0 && <><div className="mm-heading">THAM SỐ</div><div className="mm-group">{spec.params.map((param) => { const text = paramCopy(spec, param.key); return <div className="mm-guide-param mm-row" key={param.key}><div><code style={{ color: CMD_COLOR[param.kind === 'flag' || param.prefix ? 'flag' : 'value'] }}>{param.kind === 'flag' ? param.flag || param.token : param.token}</code><span className={param.required ? 'mm-idle' : 'mm-muted'}>{param.required ? 'Bắt buộc' : 'Tuỳ chọn'}</span></div><strong>{text?.label || param.token}</strong><p>{text?.desc}</p></div> })}</div></>}
    <div className="mm-heading">VÍ DỤ</div><div className="mm-guide-cards">{copy?.ex.map((example, index) => { const tokens = composeTokens(spec, { ...emptyValues(spec).values, ...example.vals }, example.slash); return <div className="mm-guide-card" key={index}><div className="mm-guide-example mm-mono">{tokens.map((token, i) => <span style={{ color: CMD_COLOR[token.tone] }} key={i}>{token.text}</span>)}</div><div className="mm-example-copy"><span>{example.text}</span><button type="button" disabled={desk} onClick={() => onUse(tokens.map((token) => token.text).join(''))}>Dùng</button></div></div> })}</div>
    <div className="mm-heading">NÊN DÙNG KHI</div><div className="mm-group">{copy?.when.map((line) => <div className="mm-row mm-guide-advice" key={line}><Icon name="tick-circle" size={16} className="mm-current" /><span>{line}</span></div>)}{copy?.avoid.map((line) => <div className="mm-row mm-guide-advice" key={line}><Icon name="close-circle-bulk" size={16} className="mm-danger" /><span>{line}</span></div>)}</div>
    <div className="mm-heading">SAU KHI CHẠY</div><ol className="mm-group mm-guide-after">{copy?.after.map((line, index) => <li className="mm-row mm-guide-advice" key={line}><span>{index + 1}</span><span>{line}</span></li>)}</ol>
    {!!copy?.errs.length && <><div className="mm-heading">LỖI THƯỜNG GẶP</div><div className="mm-guide-cards">{copy.errs.map((error) => <div className="mm-guide-card mm-guide-error" key={error.msg}><span>{error.msg}</span><p>→ {error.fix}</p></div>)}</div></>}
    {copy?.tip && <div className="mm-guide-tip"><Icon name="lamp-on" size={17} /><span>{copy.tip}</span></div>}
    <div className="mm-heading">LIÊN QUAN</div><div className="mm-related">{copy?.rel.map((name) => <button className="m-chip mm-mono" key={name} type="button" onClick={() => onPick(name)}>/{name}</button>)}</div>
    {startHere && <div className="mm-guide-open"><button type="button" className="m-button" onClick={onOpenNewStory}>{t.newStory.openNewStory}</button></div>}
  </article>
}
