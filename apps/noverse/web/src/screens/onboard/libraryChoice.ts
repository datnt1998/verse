// Bước “Nơi lưu truyện” (09-auth-onboard.md §10 · Onboard.dc.html › libVals): toàn bộ trạng thái chọn thư mục.
// Ba lựa chọn: Đang dùng (thư viện hiện tại) · Gợi ý (“Tài liệu › Truyện Noverse”, tạo khi bấm Tiếp tục) · Bạn chọn.
// OnboardScreen chỉ gọi hook này một lần, gọi commit() khi bấm Tiếp tục và đọc `blocked` để khoá nút.
import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { ApiError, getFsPlaces, getLibrary, inspectLibraryPath, mkFsFolder, putLibraryRoot } from '../../api/client'
import type { FsPlace, LibraryInspect, LibraryResponse } from '../../api/types'
import { useResource } from '../../data/useResource'
import { SUGGESTED_FOLDER, suggestedPath, useFolderPicker } from '../dialogs/FolderBrowserDialog'
import { displayLeaf, inspectNote, inspectView, type InspectView } from '../dialogs/libraryInspect'
import { t } from '../../i18n/vi'

export type LibOptionKey = 'current' | 'suggest' | 'custom'
export type LibOption = {
  key: LibOptionKey
  icon: string
  title: string
  /** “Nơi › … › Thư mục” — không bao giờ là đường dẫn tuyệt đối. */
  where: string
  tag: 'Đang dùng' | 'Gợi ý' | 'Bạn chọn'
  note: string
  noteFg: string
}

export type LibraryChoice = {
  loading: boolean
  options: LibOption[]
  sel: LibOptionKey | null
  select: (key: LibOptionKey) => void
  /** Hộp trạng thái dưới danh sách; null khi chưa có gì để nói. */
  status: InspectView | null
  /** Thư mục đã chọn không ghi được → khoá Tiếp tục. */
  blocked: boolean
  /** Chưa chọn được gì (cài mới mà chưa đọc xong các nơi lưu) → chưa cho Tiếp tục. */
  empty: boolean
  /** “Nơi › … › Thư mục” của lựa chọn hiện tại (ô tóm tắt “Lưu ở”). */
  display: string
  picker: { pick: () => Promise<void>; picking: boolean; error: string; dialog: ReactNode }
  /** Tạo thư mục gợi ý (nếu cần) rồi PUT /api/library {root}. Trả thư viện mới khi đã đổi gốc, null khi giữ nguyên. */
  commit: () => Promise<LibraryResponse | null>
}

const newFolderStatus = (parentLabel: string, oldName: string): InspectView => ({
  tone: 'info', icon: 'info-circle', bg: 'rgba(157,176,247,0.06)', bd: 'rgba(157,176,247,0.2)', fg: '#C8D3F8',
  text: `Noverse sẽ tạo thư mục “${SUGGESTED_FOLDER}” trong ${parentLabel}.${oldName ? ` Truyện cũ ở “${oldName}” vẫn giữ nguyên, nhập lại được bất cứ lúc nào.` : ''}`,
})

function useInspect(path: string, enabled: boolean) {
  return useResource<LibraryInspect>(['library', 'inspect', path], ({ signal }) => inspectLibraryPath(path, signal), { enabled: enabled && !!path, staleTime: 5000 })
}

export function useLibraryChoice({ library, enabled }: { library: LibraryResponse | null; enabled: boolean }): LibraryChoice {
  const filesystem = !!library && (library.storageMode ?? 'file') === 'file'
  enabled = enabled && filesystem
  const places = useResource<FsPlace[]>(['fs', 'places'], () => getFsPlaces(), { enabled, staleTime: 30000 })
  const [sel, setSel] = useState<LibOptionKey | null>(null)
  const [custom, setCustom] = useState<{ path: string; info: LibraryInspect } | null>(null)

  const configured = !!library?.configured
  const currentPath = configured ? library?.rootPath ?? '' : ''
  const currentInfo = useInspect(currentPath, enabled && configured)
  const suggestion = useMemo(() => suggestedPath(places.data ?? null), [places.data])
  const suggestPath = suggestion && suggestion.path !== currentPath && suggestion.path !== custom?.path ? suggestion.path : ''
  const suggestInfo = useInspect(suggestPath, enabled)

  const picker = useFolderPicker({
    start: currentPath || undefined,
    onPicked: (path, info) => {
      // Chọn lại đúng thư mục đang dùng / thư mục gợi ý → dùng lại thẻ có sẵn, không thêm thẻ thứ ba.
      if (path === currentPath) { setCustom(null); setSel('current'); return }
      if (suggestion && path === suggestion.path) { setCustom(null); setSel('suggest'); return }
      setCustom({ path, info }); setSel('custom')
    },
  })

  const options: LibOption[] = []
  if (configured && library && currentInfo.data) {
    const note = inspectNote(currentInfo.data)
    options.push({ key: 'current', icon: 'folder-open', title: displayLeaf(library.rootDisplay), where: library.rootDisplay, tag: 'Đang dùng', note: note.text, noteFg: note.fg })
  }
  if (suggestion && suggestInfo.data && suggestPath) {
    const note = inspectNote(suggestInfo.data)
    options.push({ key: 'suggest', icon: 'document-text', title: SUGGESTED_FOLDER, where: `${suggestion.parent.label} › ${SUGGESTED_FOLDER}`, tag: 'Gợi ý', note: note.text, noteFg: note.fg })
  }
  if (custom) {
    const note = inspectNote(custom.info)
    options.push({ key: 'custom', icon: 'cloud-connection', title: displayLeaf(custom.info.display), where: custom.info.display, tag: 'Bạn chọn', note: note.text, noteFg: note.fg })
  }

  // Mặc định: thư viện đang dùng, chưa có thì gợi ý (tạo khi bấm Tiếp tục).
  const effective: LibOptionKey | null = sel && options.some((option) => option.key === sel) ? sel : options[0]?.key ?? null
  const selectedInfo = effective === 'current' ? currentInfo.data : effective === 'suggest' ? suggestInfo.data : effective === 'custom' ? custom?.info : undefined
  const oldName = configured && library ? displayLeaf(library.rootDisplay) : ''
  const status = !selectedInfo ? null
    : effective === 'suggest' && selectedInfo.kind === 'missing' && !selectedInfo.storageWarning ? newFolderStatus(suggestion?.parent.label ?? '', oldName)
    : inspectView(selectedInfo)
  const display = selectedInfo?.display ?? (effective === 'suggest' && suggestion ? `${suggestion.parent.label} › ${SUGGESTED_FOLDER}` : library?.rootDisplay ?? '')

  const commit = useCallback(async () => {
    if (!filesystem) return null
    if (effective === null || effective === 'current') return null
    let path = custom?.path ?? ''
    if (effective === 'suggest') {
      if (!suggestion) throw new Error('Chưa đọc được các nơi lưu trên máy chủ')
      path = suggestion.path
      if (suggestInfo.data?.kind === 'missing') {
        try { await mkFsFolder(suggestion.parent.path, SUGGESTED_FOLDER) } catch (error) { if (!(error instanceof ApiError && error.status === 409)) throw error }
      }
    }
    await putLibraryRoot(path)
    setSel(null); setCustom(null)
    return getLibrary()
  }, [custom, effective, suggestInfo.data?.kind, suggestion, filesystem])

  return {
    loading: !library || (enabled && places.initialLoading && !places.data),
    options, sel: effective, select: setSel, status,
    blocked: selectedInfo?.kind === 'readonly', empty: filesystem && effective === null,
    display: library?.storageMode === 'postgres' ? t.libraryStorage.hostedDisplay
      : library?.storageMode === 'container' ? t.libraryStorage.container : display, picker, commit,
  }
}
