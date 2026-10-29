// Trang Hướng dẫn lệnh — Commands.dc.html: danh sách 300px · nội dung · ghép lệnh 360px.
// Route #/help/commands/:name? (router.ts); mở bằng nút Hướng dẫn, phím ?, hoặc /help.
import { useEffect, useState } from 'react'
import type { CommandSpec } from '../../api/types'
import { groupsOf, initialValues, matchesQuery, orderedSpecs, type CommandValues } from '../../commands/model'
import { useCommands } from '../../commands/store'
import { useCommandSnapshot } from '../../commands/target'
import { CMD_UI, copyOf } from '../../i18n/commands.vi'
import { canWriteStory } from '../../api/client'
import { useSession } from '../../api/session'
import { navigate, paths } from '../../router'
import { setComposerDraft, useMediaQuery } from '../../shell'
import { Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useConnection } from '../../data'
import { Icon } from '@noverse/ui'
import { Kbd } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { Builder } from './Builder'
import { CommandDetail } from './Detail'
import { CommandIntro } from './Intro'
import { FooterNote, HelpError, KeysPanel } from './parts'
import './help.css'
import { copyText as writeClipboard } from '../../lib/clipboard'

/** Nền chip cảnh báo dùng chung cho “Cần rảnh” và “Cần quyền viết”. */
const WARN_TINT = 'rgba(246,203,140,0.14)'

export function HelpScreen({ cmd }: { cmd: string | null }) {
  const { specs, state, error, refreshing, updatedAt, retry } = useCommands()
  const connection = useConnection()
  const { target, snapshot } = useCommandSnapshot()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const [query, setQuery] = useState('')
  const [values, setValues] = useState<Record<string, CommandValues>>({})
  const [copied, setCopied] = useState<string | null>(null)
  const [flash, setFlash] = useState(0)

  useEffect(() => {
    if (!copied) return
    // Commands.dc.html: nút “Đã chép” trở lại sau 1,4 s
    const timer = setTimeout(() => setCopied(null), 1400)
    return () => clearTimeout(timer)
  }, [copied])

  const ordered = orderedSpecs(specs)
  const current = cmd ? ordered.find((spec) => spec.name === cmd) ?? null : null
  const filtered = query ? ordered.filter((spec) => matchesQuery(spec, query)) : ordered
  const groups = groupsOf(filtered)
  const busy = Boolean(snapshot?.isRunning)
  const { user } = useSession()
  /** 13-commands.md §/reopen — trang Hướng dẫn vẫn liệt kê, chỉ thêm nhãn khi thiếu quyền viết. */
  const reopenNeedsWrite = !!target && (!user || !canWriteStory(user, `${target.project}/${target.story}`))

  const pick = (name: string) => navigate(paths.help(name))
  const copyText = async (text: string, id: string) => {
    try {
      await writeClipboard(text)
      setCopied(id)
    } catch {
      // 16 §3: không nuốt lỗi — báo bằng toast đỏ, cho thử lại
      toast(CMD_UI.copyFailed, 'err', { label: CMD_UI.retry, onClick: () => void copyText(text, id) })
    }
  }
  const valuesFor = (spec: CommandSpec) => values[spec.name] ?? initialValues(spec)
  const runLine = (spec: CommandSpec, line: string) => {
    if (!target) return
    if (spec.name === 'start') { navigate(paths.newstory(target.project)); return }
    setComposerDraft(target.project, target.story, line)
    navigate(paths.workspace(target.project, target.story))
  }
  const rowStyle = (on: boolean, minHeight: number, padding: string) => ({
    display: 'flex', alignItems: 'center', gap: 10, minHeight, padding,
    border: `1px solid ${on ? 'rgba(255,255,255,0.1)' : 'transparent'}`, borderRadius: 10,
    background: on ? '#1C1C1F' : 'transparent', color: 'var(--color-fg)', textAlign: 'left' as const,
  })

  return (
    <div style={{
      flex: '1 1 auto', minWidth: 0, minHeight: 0, display: 'flex',
      flexDirection: narrow ? 'column' : 'row', overflow: 'hidden',
    }}>
      <aside aria-label="Danh sách lệnh" style={{
        flex: narrow ? '0 0 auto' : '0 0 300px', minWidth: 0, display: 'flex', flexDirection: 'column',
        background: '#0D0D0F', borderRight: narrow ? undefined : '1px solid var(--line-1)',
        borderBottom: narrow ? '1px solid var(--line-1)' : undefined, overflow: 'hidden', position: 'relative',
      }}>
        <PanelRefreshBar resourceKeys={[[ 'system', 'commands' ]]} />
        <StaleBanner error={(state === 'ready' && error) || (!connection.writable ? new Error('Mất kết nối') : null)}
          onRetry={() => { connection.retry(); retry() }} />
        <div style={{ padding: '18px 16px 12px', display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <h1 title={updatedAt ? `Cập nhật lúc ${new Date(updatedAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}` : undefined}
              style={{ margin: 0, fontSize: 17, fontWeight: 600, letterSpacing: '-0.01em' }}>{CMD_UI.helpTitle}</h1>
            <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>
              {CMD_UI.listSub(ordered.length)}<Kbd>/</Kbd>{CMD_UI.listSubAfter}
            </span>
          </div>
          <label style={{ position: 'relative', display: 'block' }}>
            <Icon name="search-normal-1" size={16} style={{ position: 'absolute', left: 10, top: 9, color: 'var(--color-fg-4)' }} />
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={CMD_UI.search}
              aria-label={CMD_UI.searchAria}
              style={{
                height: 36, padding: '0 10px 0 34px', borderRadius: 9, width: '100%',
                background: 'var(--color-surface-0)', border: '1px solid var(--line-4)',
                color: 'var(--color-fg)', fontSize: 13,
              }}
            />
          </label>
        </div>
        <nav className="scroll" aria-label="Các lệnh" style={{
          flex: '1 1 auto', minHeight: 0, overflowY: 'auto', padding: '0 8px 16px',
          display: 'flex', flexDirection: 'column', gap: 2,
        }}>
          <button type="button" onClick={() => navigate(paths.help())} aria-current={!current ? 'true' : undefined}
            className="hov-row" style={rowStyle(!current, 44, '6px 10px')}>
            <span style={{
              width: 28, height: 28, borderRadius: 8, background: 'rgba(201,166,245,0.12)',
              display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
            }}>
              <Icon name="lamp-on" size={16} style={{ color: 'var(--cmd-name)' }} />
            </span>
            <span style={{ minWidth: 0, display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap' }}>{CMD_UI.introNav}</span>
              <span style={{
                fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
              }}>{CMD_UI.introNavSub}</span>
            </span>
          </button>
          {groups.map((group) => (
            <div key={group.key} style={{ display: 'flex', flexDirection: 'column', gap: 1, marginTop: 10 }}>
              <span className="lbl" style={{ padding: '4px 10px' }}>{group.label}</span>
              {group.specs.map((spec) => {
                const on = spec.name === current?.name
                const needsWrite = spec.name === 'reopen' && reopenNeedsWrite
                return (
                  <button key={spec.name} type="button" onClick={() => pick(spec.name)}
                    aria-current={on ? 'true' : undefined} className="hov-row" style={rowStyle(on, 46, '5px 10px')}>
                    <span style={{ minWidth: 0, flex: '1 1 auto', display: 'flex', flexDirection: 'column' }}>
                      <span className="mono" style={{
                        fontSize: 13, color: on ? 'var(--cmd-name)' : '#D6D6DC', whiteSpace: 'nowrap',
                      }}>/{spec.name}{spec.aliases[0] ? ` · /${spec.aliases[0]}` : ''}</span>
                      <span style={{
                        fontSize: 12, color: '#8E8E99', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                      }}>{copyOf(spec.name)?.title ?? spec.usage}</span>
                    </span>
                    {needsWrite ? (
                      <span title={CMD_UI.needsWrite} style={{
                        height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'flex',
                        alignItems: 'center', whiteSpace: 'nowrap', flex: '0 0 auto',
                        background: WARN_TINT, color: 'var(--color-warn)',
                      }}>{CMD_UI.needsWrite}</span>
                    ) : spec.needsIdle && (
                      <span title={CMD_UI.idleLong} style={{
                        height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'flex',
                        alignItems: 'center', whiteSpace: 'nowrap', flex: '0 0 auto',
                        background: busy ? WARN_TINT : 'rgba(255,255,255,0.05)',
                        color: busy ? 'var(--color-warn)' : 'var(--color-fg-4)',
                      }}>{CMD_UI.idle}</span>
                    )}
                  </button>
                )
              })}
            </div>
          ))}
          {query && filtered.length === 0 && (
            <div role="status" style={{
              margin: '14px 6px 0', padding: 14, borderRadius: 12, border: '1px dashed rgba(255,255,255,0.12)',
              display: 'flex', flexDirection: 'column', gap: 8,
            }}>
              <span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>{CMD_UI.noMatch(query)}</span>
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{CMD_UI.tryLabel}</span>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {CMD_UI.tryWords.map((word) => (
                  <button key={word} type="button" onClick={() => setQuery(word)} className="help-chip" style={{
                    height: 26, padding: '0 10px', borderRadius: 999, border: '1px solid var(--line-4)',
                    background: 'transparent', color: 'var(--color-fg-3)', fontSize: 12, whiteSpace: 'nowrap',
                  }}>{word}</button>
                ))}
              </div>
            </div>
          )}
        </nav>
      </aside>

      <main className="scroll" style={{ flex: '1 1 auto', minWidth: 0, minHeight: 0, overflowY: 'auto' }}>
        {state === 'error' ? (
          <HelpError message={error} onRetry={retry} />
        ) : state !== 'ready' ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16, width: '100%', padding: 32 }}>
            <Skeleton height={28} width="55%" />
            {[0, 1, 2, 3, 4].map((row) => <Skeleton key={row} height={16} width={`${90 - row * 7}%`} />)}
          </div>
        ) : current ? (
          <CommandDetail
            spec={current}
            snapshot={snapshot}
            needsWrite={current.name === 'reopen' && reopenNeedsWrite}
            copiedId={copied}
            onCopy={(text, id) => { void copyText(text, id) }}
            onTry={(next) => {
              setValues((all) => ({
                ...all,
                [current.name]: { values: next, quoted: {}, extra: '' },
              }))
              setFlash((n) => n + 1)
            }}
            onPick={pick}
          />
        ) : (
          <CommandIntro specs={ordered} onPick={pick} />
        )}
      </main>

      {!narrow && (
        <aside className="scroll" aria-label="Thử lệnh và phím tắt" style={{
          flex: '0 0 360px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 14,
          background: '#0D0D0F', borderLeft: '1px solid var(--line-1)', padding: '18px 16px 24px',
          overflowY: 'auto',
        }}>
          {current ? (
            <Builder
              spec={current}
              values={valuesFor(current)}
              onChange={(next) => setValues((all) => ({ ...all, [current.name]: next }))}
              snapshot={snapshot}
              target={target}
              flash={flash}
              copiedId={copied}
              onCopy={(text, id) => { void copyText(text, id) }}
              onRun={(line) => runLine(current, line)}
            />
          ) : (
            <Tasks onPick={pick} />
          )}
          <KeysPanel />
          <FooterNote />
        </aside>
      )}
    </div>
  )
}

function Tasks({ onPick }: { onPick: (name: string) => void }) {
  return (
    <section className="card" aria-labelledby="h-tasks" style={{
      borderRadius: 16, padding: 14, display: 'flex', flexDirection: 'column', gap: 4,
    }}>
      <h3 id="h-tasks" style={{ margin: '0 0 6px', fontSize: 14, fontWeight: 600 }}>{CMD_UI.tasksTitle}</h3>
      {CMD_UI.tasks.map((task) => (
        <button key={task.key} type="button" onClick={() => onPick(task.key)} className="hov-row" style={{
          display: 'flex', alignItems: 'center', gap: 10, minHeight: 38, padding: '4px 8px',
          borderRadius: 8, border: 'none', background: 'transparent', color: 'var(--color-fg-2)',
          fontSize: 12.5, textAlign: 'left',
        }}>
          <span style={{ flex: '1 1 auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{task.label}</span>
          <span className="mono" style={{ flex: '0 0 auto', fontSize: 12, color: 'var(--cmd-name)', whiteSpace: 'nowrap' }}>{task.cmd}</span>
        </button>
      ))}
    </section>
  )
}
