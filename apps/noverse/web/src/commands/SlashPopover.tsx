// SlashPopover — Slash.dc.html 05b: popover ngay trên ô chat Workspace.
// Thuần hiển thị: nhận text ô chat, trả text mới qua onChange; không gọi API.
// Composer gọi handleSlashKey() trong onKeyDown (commands/store.ts) để xử lý ↑ ↓ Tab Enter Esc.
import { useMemo, type CSSProperties } from 'react'
import type { Snapshot } from '../api/types'
import { CMD_UI } from '../i18n/commands.vi'
import { navigate, paths } from '../router'
import { AgentOrb } from '@noverse/ui'
import { Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { useEscapeLayer } from '@noverse/ui'
import {
  buildSlashView, slashContext, type SlashDetail, type SlashItem, type SlashView,
} from './model'
import { dismissSlash, slashDismissed, useCommands, useSelection } from './store'
import { rememberHelpTarget } from './target'
import { BADGE_TONE, CMD_COLOR, STATUS_COLOR } from './tokens'
import './slash.css'

export type SlashPopoverProps = {
  /** toàn bộ text ô chat */
  value: string
  /** popover đổi text (Tab/Enter điền, Esc xoá) */
  onChange: (next: string) => void
  /** Enter “chạy lệnh” — Composer gửi qua G13 */
  onSubmit: (line: string) => void
  /** tiến độ truyện để tính “sẽ làm gì” */
  snapshot?: Snapshot | null
  /** đường dẫn xuất/nhập gần đây, theo truyện (client lưu) */
  recentPaths?: Partial<Record<'export' | 'import' | 'start' | 'importsim', string[]>>
  /** người dùng có quyền viết truyện này không (ẩn lệnh cần quyền viết) */
  writable?: boolean
}

const ICON: Record<string, string> = {
  command: 'command', edit: 'edit-2', warning: 'warning-2', flash: 'flash',
  timer: 'timer-1', info: 'info-circle', swap: 'arrow-swap-horizontal', tick: 'tick-circle',
}

const SELECTED_BG = '#26262A'

export function SlashPopover({ value, onChange, onSubmit, snapshot, recentPaths, writable = true }: SlashPopoverProps) {
  const { specs } = useCommands()
  const [selected, setSelected] = useSelection(value)
  const dismissed = slashDismissed(value)
  const view = useMemo(
    () => (specs.length ? buildSlashView(value, slashContext(snapshot ?? null, specs, recentPaths, { writable, dismissed }), selected) : null),
    [specs, value, snapshot, recentPaths, selected, writable, dismissed],
  )
  // Popover là một lớp: Esc đóng nó, giữ nguyên dòng đang gõ (17-kit, chốt 25/10).
  useEscapeLayer(view !== null, () => dismissSlash(value))
  if (!view) return null

  const pick = (item: SlashItem) => onChange(item.insert)
  const openHelp = () => { rememberHelpTarget(); navigate(paths.help()) }

  return (
    <div
      className="pop-in dlg"
      role="dialog"
      aria-label={CMD_UI.popAria}
      style={{
        position: 'absolute', left: 0, right: 0, bottom: 'calc(100% + 8px)', height: 452,
        display: 'flex', flexDirection: 'column', overflow: 'hidden', zIndex: 5,
      }}
    >
      <PopoverHeader view={view} snapshot={snapshot} />
      <div style={{ flex: '1 1 auto', minHeight: 0, display: 'flex' }}>
        <CommandList view={view} selected={selected} busy={Boolean(snapshot?.isRunning)} onHover={setSelected} onPick={pick} />
        <DetailPanel view={view} onChange={onChange} />
      </div>
      <div style={{
        flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 14, padding: '8px 14px',
        borderTop: '1px solid var(--line-1)', fontSize: 11.5, color: 'var(--color-fg-4)', whiteSpace: 'nowrap',
      }}>
        <span><Kbd>↑</Kbd> <Kbd>↓</Kbd> {CMD_UI.keysHint.pick}</span>
        <span><Kbd>Tab</Kbd> {CMD_UI.keysHint.fill}</span>
        <span><Kbd>Enter</Kbd> {view.enterHint}</span>
        <span><Kbd>Esc</Kbd> {CMD_UI.keysHint.closeDraft}</span>
        <span style={{ flex: 1 }} />
        <a href={`#${paths.help()}`} onClick={(event) => { event.preventDefault(); openHelp() }} style={{
          display: 'flex', alignItems: 'center', gap: 6, color: 'var(--color-fg-2)', fontSize: 12,
        }}>
          {CMD_UI.fullHelp}<Kbd>?</Kbd>
        </a>
      </div>
    </div>
  )
}

function PopoverHeader({ view, snapshot }: { view: SlashView; snapshot?: Snapshot | null }) {
  return (
    <div style={{
      flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 10, padding: '10px 14px',
      borderBottom: '1px solid var(--line-1)',
    }}>
      <Icon name={ICON[view.icon] ?? 'command'} size={16} style={{ color: 'var(--color-accent)' }} />
      <span style={{ fontSize: 12.5, fontWeight: 600, whiteSpace: 'nowrap' }}>{view.title}</span>
      <span style={{
        fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap', overflow: 'hidden',
        textOverflow: 'ellipsis', minWidth: 0,
      }}>{view.sub}</span>
      <span style={{ flex: 1 }} />
      {snapshot?.isRunning && (
        <span style={{
          height: 22, padding: '0 8px', borderRadius: 999, background: 'rgba(246,203,140,0.08)',
          border: '1px solid rgba(246,203,140,0.22)', color: 'var(--color-warn)', fontSize: 11.5,
          display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
        }}>
          <AgentOrb agent="writer" size={10} />
          {CMD_UI.busyChip(snapshot.inProgressChapter || snapshot.currentChapter || snapshot.completedCount + 1)}
        </span>
      )}
    </div>
  )
}

function CommandList({ view, selected, busy, onHover, onPick }: {
  view: SlashView
  selected: number
  /** trợ lý đang viết → nhãn “Cần rảnh” chuyển cam (artboard `busy`) */
  busy: boolean
  onHover: (index: number) => void
  onPick: (item: SlashItem) => void
}) {
  return (
    <div
      className="scroll"
      role="listbox"
      aria-label={view.title}
      style={{
        flex: '0 0 300px', minWidth: 0, borderRight: '1px solid var(--line-1)', padding: 6,
        display: 'flex', flexDirection: 'column', gap: 1, overflowY: 'auto',
      }}
    >
      {view.sections.map((section) => (
        <div key={section.label} style={{ display: 'contents' }}>
          <span className="lbl" style={{ fontSize: 10.5, padding: '8px 10px 4px' }}>{section.label}</span>
          {section.items.map((item) => {
            const on = item.index === selected
            return (
              <button
                key={`${section.label}:${item.index}:${item.key}`}
                type="button"
                role="option"
                aria-selected={on}
                onMouseEnter={() => onHover(item.index)}
                onClick={() => onPick(item)}
                className="slash-crow"
                style={{
                  display: 'flex', alignItems: 'center', gap: 10, minHeight: 40, padding: '4px 10px',
                  border: 'none', borderRadius: 9, background: on ? SELECTED_BG : 'transparent',
                  color: 'var(--color-fg)', textAlign: 'left', flex: '0 0 auto',
                }}
              >
                <span style={{ minWidth: 0, flex: '1 1 auto', display: 'flex', flexDirection: 'column' }}>
                  <span className="mono" style={{
                    fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                  }}>
                    <span style={{ color: item.tone ? CMD_COLOR[item.tone] : '#D6D6DC' }}>{item.before}</span>
                    <span style={{
                      color: item.matchTone ? CMD_COLOR[item.matchTone] : item.tone ? CMD_COLOR[item.tone] : 'var(--color-accent)',
                      fontWeight: 500,
                    }}>{item.match}</span>
                    <span style={{ color: item.tone ? CMD_COLOR[item.tone] : '#D6D6DC' }}>{item.after}</span>
                  </span>
                  <span style={{
                    fontSize: 11.5, color: '#8E8E99', whiteSpace: 'nowrap',
                    overflow: 'hidden', textOverflow: 'ellipsis',
                  }}>{item.desc}</span>
                </span>
                {item.tag && (
                  <span style={{
                    height: 19, padding: '0 6px', borderRadius: 5, fontSize: 10.5, display: 'flex',
                    alignItems: 'center', whiteSpace: 'nowrap', flex: '0 0 auto',
                    // artboard: chỉ nhãn “Cần rảnh” có nền/màu; “gợi ý”/“công tắc” để trống (không nền, màu thừa kế)
                    ...(item.tag === CMD_UI.tagIdle ? {
                      background: busy ? 'rgba(246,203,140,0.14)' : 'rgba(255,255,255,0.05)',
                      color: busy ? 'var(--color-warn)' : 'var(--color-fg-4)',
                    } : {}),
                  }}>{item.tag}</span>
                )}
                {on && <Kbd style={{ flex: '0 0 auto' }}>Tab</Kbd>}
              </button>
            )
          })}
        </div>
      ))}
      {view.empty && (
        <div style={{ padding: '14px 10px', display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>{view.empty.title}</span>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{view.empty.text}</span>
        </div>
      )}
    </div>
  )
}

function DetailPanel({ view, onChange }: { view: SlashView; onChange: (next: string) => void }) {
  const detail = view.detail
  const fix = () => onChange(view.line.replace(/\.(pdf|docx?)\b/i, '.epub'))
  return (
    <div className="scroll" style={{
      flex: '1 1 auto', minWidth: 0, padding: '14px 18px 16px', display: 'flex',
      flexDirection: 'column', gap: 12, overflowY: 'auto',
    }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, flexWrap: 'wrap' }}>
        <span className="mono" style={{ fontSize: 18, color: 'var(--cmd-name)' }}>{detail.slash}</span>
        <span style={{ fontSize: 14.5, fontWeight: 600 }}>{detail.title}</span>
      </div>
      <Badges detail={detail} />
      {view.mode !== 'args' && detail.lead && (
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-2)' }}>{detail.lead}</p>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        <span className="lbl" style={{ fontSize: 10.5 }}>{CMD_UI.syntaxHeading}</span>
        <div className="mono" style={{
          padding: '9px 11px', borderRadius: 9, background: 'var(--color-surface-0)',
          border: '1px solid var(--line-1)', fontSize: 12.5, lineHeight: 1.8,
          whiteSpace: 'pre-wrap', wordBreak: 'break-word',
        }}>
          {detail.usage.map((token, i) => (
            <span key={i}>{i ? ' ' : ''}<span className={token.slot ? 'cmd-slot' : undefined} style={{ color: CMD_COLOR[token.tone], padding: '0 1px' }}>
              {token.text}
            </span></span>
          ))}
        </div>
      </div>

      {detail.slot && (
        <div style={{
          display: 'flex', flexDirection: 'column', gap: 4, padding: '10px 12px', borderRadius: 10,
          background: 'rgba(201,166,245,0.06)', border: '1px solid rgba(201,166,245,0.18)',
        }}>
          <span style={{ fontSize: 12, color: '#DCC8F8', display: 'flex', gap: 6, alignItems: 'center' }}>
            <Icon name="edit-2" size={13} />{detail.slot.title}
          </span>
          <span style={{ fontSize: 12.5, color: '#D6D6DC' }}>{detail.slot.desc}</span>
          {detail.slot.def && <span style={{ fontSize: 11.5, color: '#8E8E99' }}>{detail.slot.def}</span>}
        </div>
      )}

      {view.mode !== 'args' && detail.examples.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span className="lbl" style={{ fontSize: 10.5 }}>{CMD_UI.examplesHeading}</span>
          {detail.examples.map((example, index) => (
            <button
              key={index}
              type="button"
              onClick={() => onChange(example.insert)}
              className="slash-crow"
              style={{
                display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 1,
                padding: '7px 10px', borderRadius: 9, border: '1px solid var(--line-1)',
                background: 'transparent', color: 'var(--color-fg)', textAlign: 'left',
              }}
            >
              <span className="mono" style={{ fontSize: 12.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                {example.tokens.map((token, i) => (
                  <span key={i} style={{ color: CMD_COLOR[token.tone] }}>{token.text}</span>
                ))}
              </span>
              <span style={{ fontSize: 11.5, color: '#8E8E99' }}>{example.text}</span>
            </button>
          ))}
        </div>
      )}

      {view.mode === 'args' && detail.status && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span className="lbl" style={{ fontSize: 10.5 }}>{CMD_UI.enterHeading}</span>
          <div role="status" style={{
            display: 'flex', gap: 8, alignItems: 'flex-start', fontSize: 12.5,
            color: STATUS_COLOR[detail.status.tone],
          }}>
            <Icon name={ICON[detail.status.icon] ?? 'info-circle'} size={14} style={{ marginTop: 3 }} />
            <span>{detail.status.text}</span>
          </div>
          {detail.infos.map((info, index) => (
            <div key={index} style={{
              display: 'flex', gap: 8, alignItems: 'flex-start', fontSize: 12, color: 'var(--color-fg-3)',
            }}>
              <Icon name="info-circle" size={14} style={{ marginTop: 2, color: 'var(--color-fg-4)' }} />
              <span>{info}</span>
            </div>
          ))}
          {detail.fix && (
            <button
              type="button"
              onClick={fix}
              className="hov-ghost"
              style={{
                alignSelf: 'flex-start', height: 28, padding: '0 10px', borderRadius: 8,
                border: '1px solid rgba(255,255,255,0.12)', background: 'transparent',
                color: 'var(--color-fg)', fontSize: 12, display: 'flex', alignItems: 'center',
                gap: 6, whiteSpace: 'nowrap',
              }}
            >
              <Icon name="magic-star" size={14} style={{ color: 'var(--color-accent)' }} />
              {detail.fix.label}
            </button>
          )}
        </div>
      )}

      {detail.busyWarn && (
        <div style={{
          display: 'flex', gap: 10, alignItems: 'flex-start', padding: '10px 12px', borderRadius: 10,
          background: 'rgba(246,203,140,0.07)', border: '1px solid rgba(246,203,140,0.24)',
        }}>
          <AgentOrb agent="writer" size={18} style={{ marginTop: 1 }} />
          <span style={{ fontSize: 12.5, color: '#E9D9BD' }}>
            {CMD_UI.busyWarnBefore(view.writingChapter)}<Kbd>Esc</Kbd>{CMD_UI.busyWarnAfter}
          </span>
        </div>
      )}

      {detail.tip && (
        <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start', fontSize: 12, color: 'var(--color-fg-3)' }}>
          <Icon name="lamp-on" size={15} style={{ marginTop: 1, color: 'var(--color-accent)' }} />
          <span>{detail.tip}</span>
        </div>
      )}
    </div>
  )
}

function Badges({ detail }: { detail: SlashDetail }) {
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: -4 }}>
      {detail.badges.map((badge) => {
        const tone = BADGE_TONE[badge.tone]
        return (
          <span key={badge.label} style={{
            height: 22, padding: '0 8px', borderRadius: 6, background: tone.bg,
            border: `1px solid ${tone.bd}`, color: tone.fg, fontSize: 11.5,
            display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap',
          }}>
            <Icon name={ICON[badge.icon] ?? 'info-circle'} size={13} />
            {badge.label}
          </span>
        )
      })}
    </div>
  )
}

/**
 * Slash.dc.html › `sent`: viên thuốc “Đã chạy /lệnh” ngay trên ô chat sau khi gửi lệnh (popover đóng).
 * Đặt trong khung cha `position: relative` của ô chat; tự ẩn do nơi dùng quyết định (artboard: 2,6 s).
 */
export function SlashSentToast({ line }: { line: string }) {
  return (
    <div className="slash-toast" role="status" style={{
      position: 'absolute', left: 12, bottom: 'calc(100% + 10px)', height: 32, padding: '0 12px',
      borderRadius: 999, background: '#18181A', border: '1px solid rgba(166,237,193,0.3)',
      display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, whiteSpace: 'nowrap',
    }}>
      <Icon name="tick-circle" size={15} style={{ color: 'var(--color-ok)' }} />
      {CMD_UI.sentToast}
      <span className="mono" style={{ color: 'var(--cmd-name)' }}>{line}</span>
    </div>
  )
}

/** Ô chat dùng popover này cần `position: relative` trên khung cha (Slash.dc.html). */
