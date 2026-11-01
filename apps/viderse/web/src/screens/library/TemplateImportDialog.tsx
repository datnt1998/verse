// Hộp nhập template cũ: vùng thả tệp + danh sách kết quả.
// Artboard Library.dc.html › tab=templates chỉ vẽ nút "Nhập template" trên
// thanh đầu (không có ô thả), nên nút ở đó mở hộp này; ô thả nằm trong hộp.
// Mỗi tệp một dòng {tên, đã nhập/chưa, cảnh báo} — cảnh báo là những gì không
// quy đổi được sang Viderse, không bao giờ bỏ im lặng.
import { useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Banner, Button, Dialog, Icon } from '@noverse/ui'
import { color, line } from '@noverse/ui/tokens.stylex'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { importTemplates, type TemplateImportItem, type TemplateImportResult } from '../../api/library'
import { uploadTooLarge } from '../../api/upload-limits'
import { ApiError } from '../../api/request'
import { keys } from '../../data/keys'
import { libraryVars } from './libraryTokens.stylex'

const copy = t.library.templates

/** Hộp nhập template; `onClose` do nơi mở quyết định (Dialog không tự đóng). */
export function TemplateImportDialog({ onClose }: { onClose: () => void }) {
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [result, setResult] = useState<TemplateImportResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const run = async (picked: FileList | File[] | null | undefined) => {
    const files = Array.from(picked ?? []).filter((file) => /\.zip$/i.test(file.name))
    if (files.length === 0) {
      setError(copy.importOnlyZip)
      return
    }
    // Qua internet, gói vượt trần bị edge chặn bằng trang 413 HTML — tự chặn
    // trước để người dùng đọc được trần thay vì một trang lỗi lạ.
    for (const file of files) {
      const tooLarge = await uploadTooLarge('template', file.size)
      if (tooLarge) {
        setError(`${file.name}: ${tooLarge}`)
        if (input.current) input.current.value = ''
        return
      }
    }
    setBusy(true)
    setError(null)
    try {
      const imported = await importTemplates(files)
      setResult(imported)
      if (imported.imported > 0) await invalidate(keys.templates())
    } catch (err) {
      const offline = err instanceof ApiError && err.status === 0
      setError(offline ? copy.importOffline : err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
      if (input.current) input.current.value = ''
    }
  }

  return (
    <Dialog
      title={copy.importTitle}
      onClose={onClose}
      width={560}
      footer={
        <>
          <Button size="sm" onClick={() => { setResult(null); setError(null); input.current?.click() }}>
            {copy.importAgain}
          </Button>
          <Button size="sm" variant="primary" onClick={onClose}>{copy.importClose}</Button>
        </>
      }
    >
      <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.importHint}</p>
      <input
        ref={input}
        type="file"
        accept=".zip,application/zip"
        multiple
        hidden
        onChange={(event) => void run(event.currentTarget.files)}
      />
      <button
        type="button"
        className={stylex.props(sx.drop, dragging && sx.dropDragging, busy && sx.dropBusy).className}
        aria-label={copy.importDrop}
        aria-busy={busy || undefined}
        disabled={busy}
        onClick={() => input.current?.click()}
        onDragOver={(event) => { event.preventDefault(); if (!busy) setDragging(true) }}
        onDragLeave={() => setDragging(false)}
        onDrop={(event) => {
          event.preventDefault()
          setDragging(false)
          if (!busy) void run(event.dataTransfer.files)
        }}
      >
        <Icon name={busy ? 'refresh' : 'document-download'} size={22} />
        <span className={stylex.props(sx.dropTitle).className}>{busy ? copy.importing : copy.importDrop}</span>
        <span className={stylex.props(sx.dropHint).className}>{copy.importAccept}</span>
        {!busy && <span className={stylex.props(sx.dropPick).className}>{copy.importPick}</span>}
      </button>

      {error && <Banner tone="error" title={error} />}
      {result && <ImportResults result={result} />}
    </Dialog>
  )
}

// ImportResults là danh sách kết quả: tổng kết, rồi từng template.
function ImportResults({ result }: { result: TemplateImportResult }) {
  return (
    <div className={stylex.props(sx.results).className}>
      <p className={stylex.props(sx.summary).className} role="status">
        {copy.importSummary(result.imported, result.failed)}
      </p>
      {result.items.length === 0 && <p className={stylex.props(sx.dropHint).className}>{copy.importEmptyTitle}</p>}
      <ul className={stylex.props(sx.list).className}>
        {result.items.map((item, index) => (
          <li key={`${item.file}-${index}`} className={stylex.props(sx.item).className}>
            <Icon name={item.ok ? 'tick-circle' : 'close-circle'} size={15} />
            <div className={stylex.props(sx.itemBody).className}>
              <p className={stylex.props(sx.itemName).className}>{item.name}</p>
              <p className={stylex.props(sx.itemMeta).className}>{itemState(item)}</p>
              {item.error && <p className={stylex.props(sx.itemError).className} role="alert">{item.error}</p>}
              {item.warnings.length > 0 && (
                <ul className={stylex.props(sx.warnings).className}>
                  <li className={stylex.props(sx.itemMeta).className}>{copy.importWarnings(item.warnings.length)}</li>
                  {item.warnings.map((warning) => <li key={warning}>{warning}</li>)}
                </ul>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

// itemState là dòng phụ: đã nhập hay chưa, kèm số media vào kho.
function itemState(item: TemplateImportItem): string {
  if (!item.ok) return copy.importItemFail
  const media = item.media + item.mediaReused
  return media > 0 ? `${copy.importItemOk} · ${copy.importMedia(item.media, item.mediaReused)}` : copy.importItemOk
}

const sx = stylex.create({
  hint: {
    marginTop: 0, marginRight: 0, marginBottom: 'var(--vv-s-12)', marginLeft: 0,
    fontSize: 'var(--vv-s-12)', color: color['--color-fg-4'], lineHeight: libraryVars.bodyLineHeight,
  },
  drop: {
    display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
    rowGap: 'var(--vv-s-6)', columnGap: 'var(--vv-s-6)', width: '100%',
    paddingTop: 'var(--vv-s-22)', paddingBottom: 'var(--vv-s-22)',
    paddingInline: 'var(--vv-s-16)', marginBottom: 'var(--vv-s-12)',
    borderWidth: 'var(--vv-s-1)', borderStyle: 'dashed', borderColor: line['--line-4'],
    borderRadius: 'var(--vv-r-12)', backgroundColor: color['--color-surface-2'],
    color: color['--color-fg-2'], textAlign: 'center', cursor: 'pointer',
  },
  dropDragging: {
    borderColor: color['--color-accent'], backgroundColor: color['--color-surface-4'],
  },
  dropBusy: { cursor: 'progress', color: color['--color-fg-3'] },
  dropTitle: { fontSize: 'var(--vv-s-13)', fontWeight: libraryVars.labelWeight },
  dropHint: {
    marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0,
    fontSize: 'var(--vv-s-12)', color: color['--color-fg-4'], lineHeight: libraryVars.bodyLineHeight,
  },
  dropPick: {
    marginTop: 'var(--vv-s-4)', fontSize: 'var(--vv-s-12)', fontWeight: libraryVars.labelWeight,
    color: color['--color-accent'],
  },
  results: { display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-8)', columnGap: 'var(--vv-s-8)' },
  summary: {
    marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0,
    fontSize: libraryVars.controlFontSize, color: color['--color-fg-2'],
  },
  list: {
    display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-8)', columnGap: 'var(--vv-s-8)',
    marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0,
    paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, listStyle: 'none',
    maxHeight: 'var(--vv-s-280)', overflowY: 'auto',
  },
  item: {
    display: 'flex', alignItems: 'flex-start', rowGap: 'var(--vv-s-8)', columnGap: 'var(--vv-s-8)',
    paddingTop: 'var(--vv-s-8)', paddingBottom: 'var(--vv-s-8)',
    paddingInline: 'var(--vv-s-10)',
    borderWidth: 'var(--vv-s-1)', borderStyle: 'solid', borderColor: line['--line-3'],
    borderRadius: 'var(--vv-r-10)', backgroundColor: color['--color-surface-2'],
  },
  itemBody: { display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-2)', columnGap: 'var(--vv-s-2)', minWidth: 0 },
  itemName: {
    marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0,
    fontSize: 'var(--vv-s-13)', fontWeight: libraryVars.labelWeight, color: color['--color-fg'],
    overflowWrap: 'anywhere',
  },
  itemMeta: {
    marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0,
    fontSize: 'var(--vv-s-12)', color: color['--color-fg-4'], lineHeight: libraryVars.bodyLineHeight,
  },
  itemError: {
    marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0,
    fontSize: 'var(--vv-s-12)', color: color['--color-err'], lineHeight: libraryVars.bodyLineHeight,
  },
  warnings: {
    display: 'flex', flexDirection: 'column', rowGap: 'var(--vv-s-2)', columnGap: 'var(--vv-s-2)',
    marginTop: 'var(--vv-s-2)', marginRight: 0, marginBottom: 0, marginLeft: 0,
    paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 'var(--vv-s-14)',
    fontSize: 'var(--vv-s-12)', color: color['--color-fg-3'], lineHeight: libraryVars.bodyLineHeight,
  },
})
