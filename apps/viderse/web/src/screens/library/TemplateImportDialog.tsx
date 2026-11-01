// Hộp nhập template cũ: vùng thả tệp + từng dòng tiến độ thật.
// Artboard Library.dc.html › import=ready|drag|importing|done|partial|zip|offline:
// mỗi tệp là một yêu cầu riêng nên dòng nào cũng có trạng thái thật (chờ / đang
// nhập / xong / lỗi) — không bịa phần trăm. Huỷ chỉ bỏ phần CHƯA nhập (tệp đang
// chạy vẫn xong); mất kết nối thì giữ nguyên tệp đã chọn để "Thử lại".
import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Banner, Button, Dialog, Dropzone, FileRow, Icon, type FileRowStatus } from '@noverse/ui'
import { color } from '@noverse/ui/tokens.stylex'
import { corners } from '../../styles/viderse.stylex'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { importTemplates, type TemplateImportItem } from '../../api/library'
import { uploadTooLarge } from '../../api/upload-limits'
import { ApiError } from '../../api/request'
import { keys } from '../../data/keys'
import { libraryVars } from './libraryTokens.stylex'

const copy = t.library.templates

type Phase = 'ready' | 'importing' | 'done' | 'partial' | 'offline'

type Row = {
  file: File
  status: FileRowStatus
  item?: TemplateImportItem
  /** Lý do lỗi của riêng tệp này (máy chủ trả về hoặc lỗi mạng). */
  error?: string
  /** Tệp chưa kịp nhập vì người dùng bấm huỷ. */
  cancelled?: boolean
}

/** Hộp nhập template; `onClose` do nơi mở quyết định (Dialog không tự đóng). */
export function TemplateImportDialog({ onClose }: { onClose: () => void }) {
  const input = useRef<HTMLInputElement>(null)
  const alive = useRef(true)
  const cancelRef = useRef(false)
  const [phase, setPhase] = useState<Phase>('ready')
  const [rows, setRows] = useState<Row[]>([])
  const [error, setError] = useState<string | null>(null)
  const [cancelling, setCancelling] = useState(false)

  useEffect(() => () => { alive.current = false }, [])

  const update = (index: number, patch: Partial<Row>) => {
    if (!alive.current) return
    setRows((current) => current.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  // Nhập lần lượt từng tệp: một yêu cầu một tệp nên dòng tiến độ và lỗi là thật.
  const run = async (picked: File[]) => {
    if (picked.length === 0) return
    if (picked.some((file) => !/\.zip$/i.test(file.name))) {
      setError(copy.importOnlyZip)
      setPhase('ready')
      return
    }
    // Qua internet, gói vượt trần bị edge chặn bằng trang 413 HTML — tự chặn
    // trước để người dùng đọc được trần thay vì một trang lỗi lạ.
    for (const file of picked) {
      const tooLarge = await uploadTooLarge('template', file.size)
      if (tooLarge) {
        if (!alive.current) return
        setError(`${file.name}: ${tooLarge}`)
        setPhase('ready')
        return
      }
    }
    if (!alive.current) return
    cancelRef.current = false
    setCancelling(false)
    setError(null)
    setRows(picked.map((file) => ({ file, status: 'wait' })))
    setPhase('importing')
    await importRows(picked.map((file) => ({ file, status: 'wait' })))
  }

  // Nhập những dòng chưa xong (giữ nguyên dòng đã nhập khi Thử lại).
  const importRows = async (queue: Row[]) => {
    let offline = false
    let imported = 0
    for (let index = 0; index < queue.length; index += 1) {
      const row = queue[index]
      if (row.status === 'ok') { imported += 1; continue }
      if (cancelRef.current) break
      update(index, { status: 'active', error: undefined })
      try {
        const result = await importTemplates([row.file])
        const item = result.items[0]
        if (!item) throw new Error(copy.importItemFail)
        if (item.ok) imported += 1
        update(index, { status: item.ok ? 'ok' : 'fail', item, error: item.ok ? undefined : item.error })
      } catch (caught) {
        if (caught instanceof ApiError && caught.status === 0) {
          offline = true
          update(index, { status: 'wait', error: undefined })
          break
        }
        update(index, { status: 'fail', error: caught instanceof Error ? caught.message : String(caught) })
      }
    }
    if (imported > 0) await invalidate(keys.templates())
    if (!alive.current) return
    setCancelling(false)
    setRows((current) => current.map((row) => {
      if (row.status === 'ok' || row.status === 'fail') return row
      if (offline) return { ...row, status: 'wait', error: undefined }
      if (cancelRef.current) return { ...row, status: 'wait', error: undefined, cancelled: true }
      return row
    }))
    setPhase(offline ? 'offline' : imported === queue.length ? 'done' : 'partial')
  }

  const pick = () => {
    setError(null)
    input.current?.click()
  }

  const chosen = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.currentTarget.files ?? [])
    event.currentTarget.value = ''
    void run(files)
  }

  const cancel = () => {
    cancelRef.current = true
    setCancelling(true)
  }

  const retry = () => {
    const pending = rows.filter((row) => row.status !== 'ok')
    if (pending.length === 0) return
    setError(null)
    setPhase('importing')
    cancelRef.current = false
    setCancelling(false)
    setRows((current) => current.map((row) => row.status === 'ok' ? row : { ...row, status: 'wait', error: undefined, cancelled: false }))
    void importRows(rows.map((row) => row.status === 'ok' ? row : { ...row, status: 'wait', error: undefined, cancelled: false }))
  }

  const close = () => {
    cancelRef.current = true
    onClose()
  }

  const imported = rows.filter((row) => row.status === 'ok').length
  const pending = rows.filter((row) => row.status !== 'ok').length
  const head = phase === 'done' ? 'tick' : phase === 'partial' ? 'info' : phase === 'offline' ? 'danger' : 'document-download'

  const footer = phase === 'ready'
    ? <Button size="sm" onClick={close}>{copy.importClose}</Button>
    : phase === 'importing'
      ? <Button size="sm" onClick={cancel} disabled={cancelling}>{cancelling ? copy.importCancelling : copy.importCancel}</Button>
      : phase === 'offline'
        ? <>
            <Button size="sm" onClick={close}>{copy.importClose}</Button>
            <Button size="sm" variant="primary" onClick={retry}>{copy.importRetry}</Button>
          </>
        : <>
            <Button size="sm" onClick={pick}>{copy.importAgain}</Button>
            <Button size="sm" variant="primary" onClick={close}>{copy.importDone}</Button>
          </>

  return (
    <Dialog
      title={copy.importTitle}
      icon={<span {...stylex.props(sx.headIcon, phase === 'done' && sx.headOk, phase === 'partial' && sx.headWarn, phase === 'offline' && sx.headErr)}><Icon name={head} size={16} /></span>}
      onClose={close}
      width={520}
      footer={footer}
    >
      <input ref={input} type="file" accept=".zip,application/zip" multiple hidden onChange={chosen} />
      <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.importHint}</p>

      {phase === 'ready' && (
        <Dropzone
          label={copy.importDropAria}
          title={copy.importDrop}
          hint={copy.importAccept}
          dragTitle={copy.importDragTitle}
          dragHint={copy.importDragHint}
          onPick={pick}
          onDrop={(files) => void run(files)}
          action={<Button size="sm" icon={<Icon name="folder" size={14} />} onClick={(event) => { event.stopPropagation(); pick() }}>{copy.importPick}</Button>}
        />
      )}

      {error && <Banner tone="error" title={error} />}

      {phase !== 'ready' && (
        <div {...stylex.props(sx.results)}>
          {phase === 'offline'
            ? <Banner tone="error" title={copy.importOffline} />
            : (
              <p {...stylex.props(sx.summary)} role="status">
                {phase === 'importing' ? copy.importProgress(imported, rows.length)
                  : phase === 'partial' && pending > 0 && rows.some((row) => row.cancelled)
                    ? copy.importCancelSummary(imported, pending)
                  : copy.importSummary(imported, pending)}
              </p>
            )}
          <div {...stylex.props(sx.list)}>
            {rows.map((row, index) => (
              <FileRow
                key={`${row.file.name}-${index}`}
                name={row.file.name}
                status={row.status}
                meta={metaFor(row, phase)}
                warnings={row.item?.warnings}
                warningsLabel={row.item && row.item.warnings.length > 0 ? copy.importWarnings(row.item.warnings.length) : undefined}
                defaultWarningsOpen={false}
              />
            ))}
          </div>
        </div>
      )}
    </Dialog>
  )
}

// metaFor là dòng phụ của từng tệp — luôn nói đúng việc đã xảy ra.
function metaFor(row: Row, phase: Phase) {
  if (row.status === 'ok') {
    const item = row.item
    if (!item) return copy.importItemOk
    const media = item.media + item.mediaReused
    return media > 0 ? `${copy.importItemOk} · ${copy.importMedia(item.media, item.mediaReused)}` : copy.importItemOk
  }
  if (row.status === 'fail') return row.error ? `${copy.importItemFail} · ${row.error}` : copy.importItemFail
  if (row.status === 'active') return copy.importActive
  if (phase === 'offline') return copy.importPending
  if (row.cancelled) return copy.importCancelled
  return copy.importWait
}

const sx = stylex.create({
  headIcon: {
    width: 30, height: 30, flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'center',
    borderRadius: corners['--vv-r-10'], backgroundColor: color['--color-surface-4'], color: color['--color-fg-3'],
  },
  headOk: { backgroundColor: 'var(--tint-ok)', color: color['--color-ok'] },
  headWarn: { backgroundColor: 'var(--tint-warn)', color: color['--color-warn'] },
  headErr: { backgroundColor: 'var(--tint-err)', color: color['--color-err'] },
  hint: {
    marginTop: 0, marginRight: 0, marginBottom: 'var(--vv-s-12)', marginLeft: 0,
    fontSize: 'var(--vv-s-12)', color: color['--color-fg-4'], lineHeight: libraryVars.bodyLineHeight,
  },
  results: { display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-10)', columnGap: 'var(--vv-s-10)' },
  summary: {
    marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0,
    fontSize: libraryVars.controlFontSize, color: color['--color-fg-2'],
  },
  list: {
    display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-6)', columnGap: 'var(--vv-s-6)',
    maxHeight: 'var(--vv-s-280)', overflowY: 'auto',
  },
})
