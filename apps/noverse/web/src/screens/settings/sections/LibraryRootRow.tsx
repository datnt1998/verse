// Cài đặt › Nâng cao › “Vị trí lưu trên máy chủ” (Settings.dc.html › advGroups[0]): dòng Nơi lưu truyện + Mở nơi lưu truyện.
// Tự chứa: không dùng bản nháp/nút Lưu của Cài đặt — đổi nơi lưu là một thao tác riêng có hộp xác nhận (09 §10.5).
import { useState } from 'react'
import { ApiError, putLibraryRoot, revealFolder } from '../../../api/client'
import type { LibraryInspect, LibraryResponse } from '../../../api/types'
import { confirm } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { useFolderPicker } from '../../dialogs/FolderBrowserDialog'
import { inspectView } from '../../dialogs/libraryInspect'
import { AdvButton, AdvRow } from '../advanced/AdvGroup'
import { t } from '../../../i18n/vi'

/** “Tài liệu › Truyện Noverse · 3 dự án” — chỉ dùng chuỗi hiển thị, đường dẫn tuyệt đối chỉ nằm trong tooltip (quản trị). */
export const libraryRootValue = (library: LibraryResponse | null): string => {
  if (!library) return 'Đang tải…'
  if (!library.configured) return 'Chưa chọn nơi lưu truyện'
  return `${library.rootDisplay} · ${library.projects.length} dự án`
}

function ConfirmBody({ info }: { info: LibraryInspect }) {
  const view = inspectView(info)
  return <>
    <p style={{ margin: 0 }}>Chuyển nơi lưu truyện sang <strong>{info.display}</strong>. Truyện cũ không bị di chuyển. Thư mục cũ vẫn mở lại được.</p>
    <p style={{ margin: '10px 0 0', fontSize: 13, color: view.fg }}>{view.text}</p>
  </>
}

export function LibraryRootRows({ library, canReveal, onChanged }: {
  library: LibraryResponse | null
  /** G1 — truy cập trực tiếp từ máy chủ: hiện “Mở nơi lưu truyện”. */
  canReveal: boolean
  /** gọi sau khi PUT /api/library {root} thành công để tải lại thư viện. */
  onChanged: () => void
}) {
  const [opening, setOpening] = useState(false)
  const [error, setError] = useState('')
  const picker = useFolderPicker({
    start: library?.rootPath || undefined,
    onPicked: (path, info) => {
      if (info.kind === 'readonly') { setError(inspectView(info).text); return }
      setError('')
      void confirm({
        title: 'Đổi nơi lưu truyện?',
        body: <ConfirmBody info={info} />,
        confirmLabel: 'Đổi thư mục', pendingLabel: 'Đang đổi…',
        successMessage: 'Đã đổi nơi lưu truyện',
        // 409 “Đang có truyện chạy — hãy đóng tất cả trước khi đổi thư viện” hiện ngay trong hộp xác nhận.
        onConfirm: async () => { await putLibraryRoot(path); onChanged() },
      })
    },
  })

  const reveal = async () => {
    setOpening(true)
    try {
      await revealFolder({ target: 'library' })
      toast.ok('Đã mở thư mục trên máy chủ')
    } catch (cause) {
      toast.err(cause instanceof ApiError ? cause.message : 'Không mở được thư mục trên máy chủ.')
    } finally {
      setOpening(false)
    }
  }

  if (library?.storageMode === 'postgres' || library?.storageMode === 'container') return <AdvRow label={t.libraryStorage.title}
    value={library.storageMode === 'postgres' ? t.libraryStorage.hosted : t.libraryStorage.container}
    below={library.storageWarning ? <span role="alert">{library.storageWarning}</span> : undefined} />
  return <>
    <AdvRow label="Nơi lưu truyện" value={libraryRootValue(library)} valueTitle={library?.rootPath || undefined}
      below={(error || picker.error || library?.storageWarning) ? <span role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '2px 0 8px', fontSize: 12, color: 'var(--color-err)' }}>
        <Icon name="warning-2" size={13} />{error || picker.error || library?.storageWarning}
      </span> : undefined}>
      <AdvButton pending={picker.picking} disabled={!library} onClick={() => void picker.pick()}>Đổi thư mục…</AdvButton>
    </AdvRow>
    {canReveal && library?.configured && <AdvRow label="Mở nơi lưu truyện" value="Trên máy chủ">
      <AdvButton pending={opening} onClick={() => void reveal()}>Mở trong Finder</AdvButton>
    </AdvRow>}
    {picker.dialog}
  </>
}
