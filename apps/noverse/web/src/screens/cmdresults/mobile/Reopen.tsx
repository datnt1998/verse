import { useState } from 'react'
import { getReopenChecks, reopenStory } from '../../../api/client'
import { keys, useAction, useResource } from '../../../data'
import { Icon } from '@noverse/ui'
import { formatInt } from '../logic'
import { blockedNote } from './logic'
import { Actions, Button, Chk, CmdActionSheet, Failure, Label, Loading, useCmdToast } from './shared'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

/** “21/10” — dd/MM cố định, không phụ thuộc locale của trình duyệt. */
function shortDate(iso: string): string {
  const date = new Date(iso)
  return `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}`
}

const SUGGESTIONS = ['Thêm 5 chương ngoại truyện', 'Tập 2: lên phim điện ảnh', 'Mở rộng tuyến truyện còn dang dở']
export function Reopen({ project, story, onRead, onWrite }: { project: string; story: string; onRead: (chapter?: number) => void; onWrite: () => void }) {
  const resource = useResource(['story', project, story, 'reopen'], ({ signal }) => getReopenChecks(project, story, signal), { staleTime: 0 })
  const checks = resource.data
  const blocked = !checks?.allowed
  const [direction, setDirection] = useState('')
  const [confirm, setConfirm] = useState(false)
  const [reopened, setReopened] = useState(false)
  const notify = useCmdToast()
  const reopen = useAction(async () => {
    await reopenStory(project, story, direction.trim())
    setReopened(true)
    notify('Đã mở lại · Kiến trúc sư đang lập dàn ý', onWrite)
    return true
  }, { error: false, invalidate: [keys.story(project, story), ['library']] })
  const closedDate = checks && (checks.closedAt || checks.lockedAt)
  return <>
    <div className="mc-scroll mc-reopen">
      <PanelRefreshBar resourceKeys={[resource.key]} />
      <Failure error={resource.error} onRetry={() => void resource.refresh()} />
      {!checks && !resource.error ? <Loading text="Đang kiểm tra điều kiện mở lại…" /> : checks && <>
        <section className="mc-book"><span className="mc-book-cover"><i /></span><div><strong>{checks.title}</strong><span>Đã hoàn · {formatInt(checks.chapters)} chương · {formatInt(checks.words)} chữ</span><small><Icon name="lock" size={13} />Khoá từ {closedDate ? shortDate(closedDate) : 'khi hoàn thành'}</small></div></section>
        <Label>ĐIỀU KIỆN MỞ LẠI</Label>
        <div className="mc-group">{checks.checks.map((check) => <div className="mc-row mc-check-row" key={check.id}>
          <Chk state={check.ok ? 'done' : 'bad'} />
          <div><span>{check.label}</span>{check.note && <small className={check.ok ? '' : 'mc-error-text'}>{check.note}</small>}{check.chapters.length > 0 && !check.ok && check.chapters.slice(0, 1).map((chapter) => <button type="button" className="mc-check-link" key={chapter} onClick={() => onRead(chapter)}>Mở chương {chapter} để duyệt <Icon name="arrow-right-2" size={13} /></button>)}</div>
        </div>)}</div>
        <Label>VIẾT TIẾP THEO HƯỚNG NÀO? <span className="mc-label-optional">(không bắt buộc)</span></Label>
        <div className="mc-inset">
          <textarea className="kit-frame mc-textarea" value={direction} onChange={(event) => setDirection(event.target.value)} disabled={blocked || reopened || reopen.busy} aria-label="Hướng viết tiếp" placeholder="Ví dụ: thêm 5 chương ngoại truyện sau khi phim đóng máy" />
          <div className="mc-wrap">{SUGGESTIONS.map((text) => <button type="button" className="mc-chip" key={text} disabled={blocked || reopened || reopen.busy} onClick={() => setDirection(text)}>{text}</button>)}</div>
          <p className="mc-note">Kiến trúc sư lập dàn ý mới từ chương {checks.chapters + 1}. {checks.chapters} chương cũ giữ nguyên, bản EPUB đã xuất không bị ảnh hưởng.</p>
        </div>
      </>}
      <Failure error={reopen.error} />
    </div>
    <Actions stack>{reopened ? <Button primary onClick={onWrite}>Xem Kiến trúc sư lập dàn ý</Button> : <>
      {blocked && checks && <p className="mc-blocked-note">{blockedNote(checks.checks)}</p>}
      <Button primary disabled={blocked || reopen.busy || !!resource.error} onClick={() => setConfirm(true)}><Icon name="unlock" size={18} />{reopen.busy ? 'Đang mở lại…' : 'Mở lại truyện'}</Button>
    </>}</Actions>
    <CmdActionSheet open={confirm} onClose={() => setConfirm(false)} title="Mở lại truyện đã hoàn?" description={`Truyện chuyển về “Đang viết”. Kiến trúc sư sẽ lập dàn ý từ chương ${(checks?.chapters ?? 0) + 1}${direction.trim() ? ' theo hướng bạn ghi' : ''}.`} action={{ label: 'Mở lại & lập dàn ý', onSelect: () => { setConfirm(false); void reopen.run() } }} />
  </>
}
