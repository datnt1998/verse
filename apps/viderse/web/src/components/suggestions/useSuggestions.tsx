// Đề xuất mở của một phạm vi (mọi kênh | một kênh) và hai việc người dùng làm trên dòng:
// nhận (một việc chính, hỏi lại khi tốn ảnh/xoá tệp), Bỏ qua. Đề xuất xuất hiện không
// bao giờ toast; toast chỉ cho việc người dùng vừa bấm nhận.
import { useCallback, useState } from 'react'
import { confirm, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { acceptSuggestion, dismissSuggestion, listSuggestions, type AcceptResult, type Suggestion } from '../../api/suggestions'
import { ApiError } from '../../api/request'
import { useOnline } from '../../data/status'
import { useResource } from '../../data/useResource'
import { navigate } from '../../router'
import { t } from '../../i18n/learn'

const c = t.learn.suggest

/** Làm mới mọi nơi đọc đề xuất và những thứ một đề xuất vừa nhận có thể đổi. */
export const refreshSuggestions = () => {
  invalidate(['suggestions'])
  invalidate(['workbench'])
  invalidate(['channels'])
  invalidate(['activity'])
}

const toPath = (href: string) => href.replace(/^#/, '')

function announce(result: AcceptResult) {
  toast(result.summary, 'ok')
}

/** Danh sách đề xuất mở của phạm vi; mọi nơi đọc chung một khoá cache. */
export const useSuggestionList = (scope: string) =>
  useResource(['suggestions', scope], ({ signal }) => listSuggestions(scope === 'all' ? undefined : scope, signal))

export function useSuggestions(scope: string) {
  const online = useOnline()
  const list = useSuggestionList(scope)
  const [busy, setBusy] = useState<string | null>(null)
  const [notice, setNotice] = useState<{ id: string; text: string } | null>(null)

  const run = useCallback(async (suggestion: Suggestion, work: () => Promise<void>) => {
    setBusy(suggestion.id)
    setNotice(null)
    try { await work() } catch (error) {
      setNotice({ id: suggestion.id, text: error instanceof ApiError && error.status !== 0 ? error.message : error instanceof ApiError ? c.offlineAction : c.failed })
    } finally { setBusy(null) }
  }, [])

  const accept = useCallback((suggestion: Suggestion) => run(suggestion, async () => {
    const { primary } = suggestion
    // Chỉ điều hướng (Mở template, Xem số liệu…): không gọi nhận, đề xuất vẫn mở.
    if (primary.href) { navigate(toPath(primary.href)); return }
    const cleanup = suggestion.kind === 'cleanup'
    // Việc xoá tệp hoặc tốn ảnh phải hỏi lại; tốn ảnh = xếp video vào hàng chờ → nút đỏ.
    const ask = (images: number) => confirm({
      title: cleanup ? c.confirm.cleanupTitle : primary.label,
      body: cleanup ? `${suggestion.title} ${suggestion.evidence}. ${c.confirm.cleanupBody}` : images > 0 ? `${c.confirm.cost(images)} ${c.confirm.body}` : c.confirm.body,
      tone: images > 0 || cleanup ? 'danger' : 'normal',
      confirmLabel: primary.label,
      cancelLabel: c.confirm.cancel,
      onConfirm: () => undefined,
    })
    const images = primary.cost?.images ?? 0
    const needsConfirmation = images > 0 || cleanup
    if (needsConfirmation && !await ask(images)) return
    let outcome = await acceptSuggestion(suggestion.id, needsConfirmation)
    if (outcome.kind === 'confirm') {
      if (!await ask(outcome.images)) return
      outcome = await acceptSuggestion(suggestion.id, true)
    }
    if (outcome.kind === 'gone') { toast(c.gone); refreshSuggestions(); return }
    if (outcome.kind === 'done') {
      announce(outcome.result)
      if (outcome.result.href) navigate(toPath(outcome.result.href))
      refreshSuggestions()
    }
  }), [run])

  const dismiss = useCallback((suggestion: Suggestion) => run(suggestion, async () => {
    await dismissSuggestion(suggestion.id)
    refreshSuggestions()
  }), [run])

  return { list, accept, dismiss, busy, notice, online }
}
