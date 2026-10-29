import { useEffect, type RefObject } from 'react'
import type { Snapshot } from '../../../api/types'
import { useAccountDraft } from '../../../mobile/accounts'
import { paths } from '../../../router'
import { takeComposerDraft } from '../../../shell/stores'
import { Icon } from '@noverse/ui'
import { TabletSlashPopover } from '../../../tablet/TabletSlashPopover'
import { handleSlashKey } from '../../../commands/store'
import { composerPrefill, MAX_COMPOSER } from './logic'

export function WorkspaceComposer({ project, story, tablet, inputRef, keyboard, down, actions, permitted, writable, exportable, stale, running, pauseRequested, snapshot, send, command, openCommand, onPause, onSubmit, onCommand }: {
  project: string; story: string; tablet: boolean; inputRef: RefObject<HTMLTextAreaElement | null>
  keyboard: number; down: boolean; actions: { quickCommands: { name: string; label: string }[] }; permitted: boolean; writable: boolean; exportable: boolean; stale: boolean; running: boolean; pauseRequested: boolean; snapshot: Snapshot | null
  send: { busy: boolean; pending: boolean; error: unknown }; command: { busy: boolean; pending: boolean }
  openCommand: (line?: string) => void; onPause: () => void; onSubmit: (text: string) => void; onCommand: (line: string) => void
}) {
  const [text, setText] = useAccountDraft(`${project}/${story}`)
  useEffect(() => {
    const prefill = () => {
      const path = paths.workspace(project, story)
      const hash = window.location.hash
      if (hash !== `#${path}` && !hash.startsWith(`#${path}?`)) return
      const offered = composerPrefill(hash, path)
      const draft = takeComposerDraft(project, story)
      if (offered || draft) setText(offered?.command || draft)
      if (offered) window.history.replaceState(window.history.state, '', offered.hash)
    }
    prefill()
    window.addEventListener('hashchange', prefill)
    return () => window.removeEventListener('hashchange', prefill)
  }, [project, story, setText])
  useEffect(() => {
    const field = inputRef.current
    if (field) { field.style.height = 'auto'; field.style.height = `${Math.min(field.scrollHeight, tablet ? 90 : MAX_COMPOSER)}px` }
  }, [text, tablet, inputRef])
  return <div className={`mw-composer-wrap${keyboard ? ' mw-typing' : ''}`}>
    {actions.quickCommands.length > 0 && <div className={`m-hscroll mw-quick${down || keyboard ? ' off' : ''}`}>{actions.quickCommands.map((item) => <button className="m-chip" key={item.name} disabled={item.name === 'export' ? !exportable : !writable} onClick={() => openCommand(`/${item.name}`)}><span className="mono">/{item.name}</span>{item.label}</button>)}{tablet && permitted && <button className="m-chip" disabled={!writable || !running || pauseRequested} onClick={onPause}>Tạm dừng</button>}</div>}
    <form className={`kit-frame m-composer mw-composer${text.trim() ? ' has-text' : ''}`} onSubmit={(event) => { event.preventDefault(); onSubmit(text) }}>
      {tablet && permitted && <TabletSlashPopover value={text} onChange={setText} onSubmit={onCommand} snapshot={snapshot} />}
      {!tablet && permitted && <button type="button" className="m-icon-button mono mw-slash" aria-label="Mở danh sách lệnh" onClick={() => openCommand()}>/</button>}
      <textarea ref={inputRef} rows={1} aria-label={tablet ? 'Nhắn trợ lý' : 'Góp ý cho truyện'} placeholder={permitted ? (tablet ? 'Nhắn trợ lý hoặc gõ / để dùng lệnh' : stale ? 'Góp ý — sẽ gửi khi có mạng' : 'Góp ý cho truyện…') : 'Bạn chỉ có quyền đọc'} value={text} maxLength={5000} disabled={!permitted} onChange={(event) => setText(event.target.value)} onKeyDown={(event) => {
        if (tablet && !event.nativeEvent.isComposing) { const result = handleSlashKey(event, text, snapshot); if (result.handled) { if (result.next !== undefined) setText(result.next); if (result.submit && writable) onCommand(result.submit); return } }
        if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') onSubmit(text)
      }} />
      <button type="submit" className={`m-icon-button mw-send${text.trim() ? ' on' : ''}`} aria-label={stale ? 'Xếp hàng góp ý' : tablet ? 'Gửi' : 'Gửi góp ý'} disabled={!permitted || !text.trim() || send.busy || command.busy || (stale && text.trim().startsWith('/'))} aria-busy={send.pending || command.pending}><Icon name="arrow-up" size={20} /></button>
    </form>
    {keyboard > 0 && <div className="mw-typing-note"><span className="m-orb arbiter" aria-hidden="true" />Trọng tài xét góp ý · áp dụng từ chương sau<button type="button" onClick={() => inputRef.current?.blur()}>Xong</button></div>}
    {send.error != null && <p className="m-error" role="alert">{send.error instanceof Error ? send.error.message : 'Không gửi được góp ý'}</p>}
  </div>
}
