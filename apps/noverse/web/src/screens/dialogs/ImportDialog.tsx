// Nhập tiểu thuyết — Dialogs.dc.html › import: nguồn + tuỳ chọn → job SSE → xác nhận.
// Tiến trình dùng cache /import; sự kiện job.* toàn cục làm mới và dock theo dõi cùng công việc.
// Cột "Giai đoạn" khớp các stage của engine (internal/host/imp/types.go) qua JobEvent.stage.
import { useEffect, useState } from 'react'
import { confirmImport, getImport, previewImport, startImport } from '../../api/client'
import type { JobEvent } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { navigate, paths } from '../../router'
import { Button, Kbd } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { Checkbox, Radio } from '@noverse/ui'
import { FieldError, FieldLabel, TextArea } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { EmptyState, Skeleton, Spinner } from '@noverse/ui'
import { refreshProjectData } from '../project/useProject'
import { keys, subscribeEvents, useAction, useConnection, useResource } from '../../data'
import { TextField } from '@noverse/ui'
import { KitSelect } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

export const IMPORT_STAGES: readonly (readonly [string, string])[] = [
  ['ingesting', 'Đọc tệp'],
  ['segmenting', 'Chia đoạn'],
  ['awaiting_confirmation', 'Chờ xác nhận chia chương'],
  ['analyzing', 'Phân tích'],
  ['synthesizing', 'Tổng hợp thiết lập'],
  ['awaiting_story_status', 'Chờ chọn truyện mở / đã kết'],
  ['validating', 'Kiểm tra'],
  ['publishing', 'Ghi vào truyện'],
  ['done', 'Hoàn tất'],
]

/** Vị trí giai đoạn hiện tại trong IMPORT_STAGES; -1 khi stage chưa xác định. */
export function importStageIndex(job: Pick<JobEvent, 'stage' | 'awaiting' | 'done' | 'error'>): number {
  if (job.done && !job.error) return IMPORT_STAGES.length - 1
  if (job.awaiting === 'confirm') return 2
  if (job.awaiting === 'storyStatus') return 5
  return IMPORT_STAGES.findIndex(([key]) => key === job.stage)
}

type LogLine = { text: string; level: JobEvent['level'] }

export function ImportDialog({ project, story, onClose }: { project: string; story: string; onClose: () => void }) {
  const connection = useConnection()
  const resume = useResource(keys.story(project, story, 'snapshot', 'import'), () => getImport(project, story))
  const [path, setPath] = useState('')
  const [storyStatus, setStoryStatus] = useState<'open' | 'closed'>('open')
  const [continueAfter, setContinueAfter] = useState(false)
  const [guide, setGuide] = useState('')
  const [jobId, setJobId] = useState<string | null>(null)
  const [log, setLog] = useState<LogLine[]>([])
  const [previewPath, setPreviewPath] = useState('')
  const [previewOpen, setPreviewOpen] = useState(false)
  const previewResource = useResource(keys.story(project, story, 'snapshot', 'import-preview', previewPath), () => previewImport(project, story, previewPath), { enabled: !!previewPath && previewOpen })
  const preview = previewOpen ? previewResource.data : null
  const previewBusy = previewResource.initialLoading && previewOpen
  useEffect(() => subscribeEvents(['job.created', 'job.progress', 'job.waiting', 'job.failed', 'job.done'], () => { void resume.refresh() }), [resume.refresh])
  const job = resume.data?.job && (!jobId || resume.data.job.job === jobId) ? resume.data.job : null

  // Nhật ký: mỗi thông điệp mới của job thành một dòng (✓ / ↻ / ✗ theo level).
  const message = job?.error || job?.message || ''
  const level: JobEvent['level'] = job?.error ? 'error' : job?.level ?? 'info'
  useEffect(() => {
    if (!message) return
    setLog((lines) => (lines.length > 0 && lines[lines.length - 1].text === message ? lines : [...lines, { text: message, level }]))
  }, [message, level])

  const startAction = useAction(async () => {
    const started = await startImport(project, story, { path: path.trim(), storyStatus, continueAfter, guide: guide.trim() || undefined })
    setLog([]); setJobId(started.job); setPreviewOpen(false)
    await resume.refresh()
    return started
  }, { success: 'Đã bắt đầu nhập · theo dõi trong Việc nền', invalidate: [keys.jobs()] })
  const confirmAction = useAction(async (accept: boolean, status?: 'open' | 'closed') => {
    const result = await confirmImport(project, story, accept, status)
    if (!accept) { setJobId(null); setLog([]); setPreviewOpen(false) }
    await resume.refresh()
    return result
  }, { success: 'Đã gửi xác nhận nhập', invalidate: [keys.jobs(), keys.story(project, story), keys.project(project)] })
  const busy = startAction.busy || confirmAction.busy || !connection.writable
  const failure = startAction.error || confirmAction.error || previewResource.error
  const error = failure instanceof Error ? failure.message : null
  const pathInvalid = /[\u0000-\u001f]/.test(path)
  // 17-kit.md §2.2/§5: nút bị khoá nói rõ lý do (tooltip + dòng chữ cạnh nút, aria-describedby).
  const startBlockedReason = !path.trim() ? 'Nhập đường dẫn tệp nguồn để bắt đầu nhập.'
    : pathInvalid ? 'Đường dẫn không chứa ký tự điều khiển.'
      : !connection.writable ? 'Chờ kết nối lại để bắt đầu nhập.'
        : startAction.busy || confirmAction.busy ? 'Đang bắt đầu nhập…' : null
  const start = () => { if (!startBlockedReason) void startAction.run() }
  const confirm = (accept: boolean, status?: 'open' | 'closed') => { if (!busy) return confirmAction.run(accept, status) }
  const loadPreview = () => { setPreviewPath(path.trim()); setPreviewOpen(true) }

  // Phím y = xác nhận chia chương (Dialogs.dc.html › kbd "y")
  const awaitingConfirm = job?.awaiting === 'confirm'
  useEffect(() => {
    if (!awaitingConfirm) return
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, select, [contenteditable="true"]')) return
      if (event.key === 'y' && !event.metaKey && !event.ctrlKey && !event.altKey) {
        event.preventDefault()
        void confirm(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const current = job ? importStageIndex(job) : -1
  const failed = Boolean(job?.error)

  const footer = !job ? (
    <>
      {startBlockedReason && (
        <span id="imp-start-reason" style={{ marginRight: 'auto', alignSelf: 'center', fontSize: 12, color: 'var(--color-fg-3)' }}>{startBlockedReason}</span>
      )}
      <Button onClick={onClose}>{t.dialogs.cancel}</Button>
      <Button variant="primary" onClick={start} disabled={Boolean(startBlockedReason)} pending={startAction.pending} success={startAction.success}
        tooltip={startBlockedReason ?? undefined} aria-describedby={startBlockedReason ? 'imp-start-reason' : undefined}>
        Bắt đầu nhập
      </Button>
    </>
  ) : job.done ? (
    <>
      <Button onClick={onClose}>{t.dialogs.close}</Button>
      {!failed && (
        <Button variant="primary" onClick={() => {
          refreshProjectData()
          navigate(paths.workspace(project, story), true)
          onClose()
        }}>Mở truyện</Button>
      )}
    </>
  ) : undefined

  return (
    <Dialog padBody={false} title={t.dialogs.importTitle} meta="/import" width={820} onClose={onClose} footer={footer}
      icon={<Icon name="document-text" size={18} style={{ color: 'var(--color-accent)' }} />}>
      <PanelRefreshBar resourceKeys={[keys.story(project, story, 'snapshot', 'import')]} updatedAt={resume.updatedAt} />
      <StaleBanner error={resume.error} onRetry={resume.refresh} />
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 18, padding: 18, maxHeight: 'calc(100dvh - 180px)', overflowY: 'auto' }}>
        <div style={{ flex: '0 0 230px', display: 'flex', flexDirection: 'column', gap: 2 }}>
          <span className="lbl" style={{ paddingBottom: 6 }}>Giai đoạn</span>
          <ol aria-label="Giai đoạn nhập" style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 2 }}>
            {IMPORT_STAGES.map(([key, label], index) => {
              const state = !job || current < 0 ? 'todo'
                : index < current || (index === current && job.done && !failed) ? 'done'
                  : index === current ? 'cur' : 'todo'
              return (
                <li key={key} aria-current={state === 'cur' ? 'step' : undefined} style={{
                  display: 'flex', alignItems: 'center', gap: 10, minHeight: 30, fontSize: 13,
                  color: state === 'todo' ? 'var(--color-fg-4)' : 'var(--color-fg)',
                }}>
                  {state === 'done' && <Icon name="tick-circle-bulk" size={15} style={{ color: 'var(--color-ok)' }} />}
                  {state === 'cur' && (
                    <span aria-hidden="true" style={{
                      width: 15, height: 15, borderRadius: '50%', flex: '0 0 auto',
                      border: `2px solid ${failed ? 'var(--color-err)' : 'var(--color-warn)'}`,
                    }} />
                  )}
                  {state === 'todo' && (
                    <span aria-hidden="true" style={{
                      width: 15, height: 15, borderRadius: '50%', flex: '0 0 auto', border: '1.5px dashed color-mix(in srgb, var(--color-fg) 25%, transparent)',
                    }} />
                  )}
                  {label}
                </li>
              )
            })}
          </ol>
        </div>

        <div style={{ flex: '1 1 360px', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 12 }}>
          {resume.initialLoading && !resume.data ? <Skeleton height={220} radius={12} /> : resume.error && !resume.data ? <EmptyState icon="warning-2" title="Không tải được phiên nhập" hint={resume.error instanceof Error ? resume.error.message : undefined} action={<Button onClick={() => void resume.refresh()}>Thử lại</Button>} /> : !job ? (
            <form style={{ display: 'flex', flexDirection: 'column', gap: 12 }}
              onSubmit={(event) => { event.preventDefault(); void start() }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <FieldLabel htmlFor="imp-path">Tệp nguồn</FieldLabel>
                <TextField id="imp-path" className="mono" autoFocus value={path} onChange={setPath}
                  validate={(value) => /[\u0000-\u001f]/.test(value) ? 'Đường dẫn không chứa ký tự điều khiển' : undefined} placeholder="./ngoai/ngap-troi.md" />
                <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
                  Đường dẫn tệp .txt / .md hoặc thư mục chương, tính từ thư mục truyện.
                </span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <FieldLabel htmlFor="imp-status">Trạng thái sau khi nhập</FieldLabel>
                <KitSelect id="imp-status" value={storyStatus} onChange={(value) => setStoryStatus(value as 'open' | 'closed')}
                  options={[{ value: 'open', label: 'Đang mở · viết tiếp được' }, { value: 'closed', label: 'Đã khép · chỉ đọc' }]} />
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <FieldLabel htmlFor="imp-guide">Hướng dẫn nhập</FieldLabel>
                <TextArea id="imp-guide" rows={2} value={guide} onChange={(event) => setGuide(event.target.value)}
                  placeholder="Giữ nguyên giọng văn gốc, không viết lại chương đã có…" />
              </div>
              <Checkbox checked={continueAfter} onChange={setContinueAfter}
                style={{ gap: 8, padding: 0, margin: 0, fontSize: 13, color: 'var(--color-fg-2)' }}
                label="Viết tiếp ngay sau khi nhập xong" />
              {error && <FieldError>{error}</FieldError>}
            </form>
          ) : (
            <>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 10, fontSize: 13 }}>
                <div>
                  <div className="lbl">Tệp nguồn</div>
                  <TextField className="mono" aria-label="Tệp nguồn xem trước" value={path} onChange={setPath} placeholder="./ngoai/ngap-troi.md"
                    validate={(value) => /[\u0000-\u001f]/.test(value) ? 'Đường dẫn không chứa ký tự điều khiển' : undefined} />
                </div>
                <div>
                  <div className="lbl">Tiến độ</div>
                  <div className="num">{job.total > 0 ? `${job.current} / ${job.total} đoạn` : '—'}</div>
                </div>
              </div>

              {job.awaiting === 'confirm' && (
                <div className="card" style={{
                  borderRadius: 12, padding: '12px 14px', borderColor: 'color-mix(in srgb, var(--color-warn) 35%, transparent)',
                  display: 'flex', flexDirection: 'column', gap: 8,
                }}>
                  <div style={{ fontSize: 14, fontWeight: 500 }}>Xác nhận cách chia chương</div>
                  <div style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{job.message ? translateHost(job.message) : '—'}</div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                    <Button variant="primary" onClick={() => void confirm(true)} disabled={busy} pending={confirmAction.pending} success={confirmAction.success}>
                      Xác nhận <Kbd>y</Kbd>
                    </Button>
                    <button type="button" className="hov-ghost" onClick={() => void confirm(false)} disabled={busy}
                      style={{
                        height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)',
                        background: 'transparent', color: 'var(--color-fg-2)', fontSize: 13,
                      }}>
                      Huỷ nhập
                    </button>
                    <button type="button" className="hov-ghost" onClick={() => void loadPreview()} disabled={previewBusy || !path.trim() || /[\u0000-\u001f]/.test(path)} aria-busy={previewBusy || undefined}
                      style={{
                        height: 32, padding: '0 12px', display: 'flex', alignItems: 'center', gap: 6, borderRadius: 8,
                        border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg-2)', fontSize: 13,
                      }}>
                      {previewBusy ? <Spinner size={13} /> : <Icon name="document-text" size={14} />}
                      {previewBusy ? 'Đang chia chương · tối đa 20 giây' : 'Xem các chương'}
                    </button>
                  </div>

                  {preview && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 8, paddingTop: 4 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ flex: 1, fontSize: 13, fontWeight: 500 }}>Cách chia chương sẽ dùng</span>
                        <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>
                          {preview.chapters.length} chương · {preview.encoding}
                        </span>
                        <button type="button" className="hov-ghost" title="Đóng xem trước" onClick={() => setPreviewOpen(false)} aria-label="Đóng xem trước"
                          style={{
                            width: 26, height: 26, borderRadius: 6, border: 'none', background: 'transparent',
                            color: 'var(--color-fg-3)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                          }}>
                          <Icon name="close-circle-bulk" size={15} />
                        </button>
                      </div>
                      {preview.warnings.length > 0 && (
                        <div role="status" style={{ display: 'flex', flexDirection: 'column', gap: 2, fontSize: 12, color: 'var(--color-warn)' }}>
                          {preview.warnings.map((warning) => (
                            <span key={warning} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                              <Icon name="warning-2" size={13} />{warning}
                            </span>
                          ))}
                        </div>
                      )}
                      <ol className="scroll" aria-label="Các chương xem trước" style={{
                        listStyle: 'none', margin: 0, padding: 0, maxHeight: 220, overflowY: 'auto',
                        display: 'flex', flexDirection: 'column', gap: 2,
                      }}>
                        {preview.chapters.map((chapter) => (
                          <li key={chapter.n} style={{
                            display: 'flex', alignItems: 'baseline', gap: 10, padding: '6px 8px',
                            borderRadius: 8, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)',
                          }}>
                            <span className="num" style={{ flex: '0 0 22px', fontSize: 12, color: 'var(--color-fg-4)' }}>{chapter.n}</span>
                            <span style={{ flex: 1, minWidth: 0 }}>
                              <span style={{ display: 'block', fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                                {chapter.title}
                              </span>
                              <span className="mono" style={{
                                display: 'block', fontSize: 11, color: 'var(--color-fg-4)',
                                whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                              }}>{chapter.firstLine}</span>
                            </span>
                            <span className="num" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
                              {chapter.words.toLocaleString('vi-VN')} chữ
                            </span>
                          </li>
                        ))}
                      </ol>
                    </div>
                  )}
                </div>
              )}

              {job.awaiting === 'storyStatus' && (
                <div className="card" style={{
                  borderRadius: 12, padding: '12px 14px', borderColor: 'color-mix(in srgb, var(--color-warn) 35%, transparent)',
                  display: 'flex', flexDirection: 'column', gap: 8,
                }}>
                  <div style={{ fontSize: 14, fontWeight: 500 }}>Truyện nhập vào đang mở hay đã kết?</div>
                  <div role="radiogroup" aria-label="Trạng thái truyện" style={{ display: 'flex', gap: 12, fontSize: 13 }}>
                    {(['open', 'closed'] as const).map((value) => (
                      <Radio key={value} name="imp-final-status" checked={storyStatus === value}
                        onChange={() => setStoryStatus(value)}
                        label={value === 'open' ? 'Đang mở · viết tiếp được' : 'Đã kết · chỉ đọc'} />
                    ))}
                  </div>
                  <div>
                    <Button variant="primary" onClick={() => void confirm(true, storyStatus)} disabled={busy} pending={confirmAction.pending} success={confirmAction.success}>Tiếp tục</Button>
                  </div>
                </div>
              )}

              {error && <FieldError>{error}</FieldError>}

              <div className="mono" role="log" aria-label="Nhật ký nhập" style={{
                padding: '10px 12px', borderRadius: 10, background: 'var(--color-surface-0)',
                border: '1px solid var(--line-1)', fontSize: 12, lineHeight: 1.9, color: 'var(--color-fg-3)',
                maxHeight: 200, overflowY: 'auto',
              }}>
                {log.length === 0 && <div style={{ color: 'var(--color-fg-4)' }}>—</div>}
                {log.map((line, index) => (
                  <div key={`${index}-${line.text}`}>
                    <span style={{
                      color: line.level === 'error' ? 'var(--color-err)' : line.level === 'warn' ? 'var(--color-warn)' : 'var(--color-ok)',
                    }}>{line.level === 'error' ? '✗' : line.level === 'warn' ? '↻' : '✓'}</span>{' '}
                    {translateHost(line.text)}
                  </div>
                ))}
              </div>

              {!job.done && !job.awaiting && (
                <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
                  Việc dài chạy nền — có thể đóng hộp thoại, tiến trình vẫn tiếp tục.
                </span>
              )}
            </>
          )}
        </div>
      </div>
    </Dialog>
  )
}
