// Thao tác hàng loạt của Kênh › Video: gọi POST /api/clone/bulk, báo kết quả
// từng video bằng toast, làm mới các khoá dữ liệu dùng chung. Hộp xác nhận
// cho việc không hoàn tác (Huỷ, Xoá) giống Bàn dựng.
import { confirm, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { BULK_MAX, BUNDLE_MAX, bundleUrl, runBulk, type BulkAction } from '../../api/channelVideos'
import { keys } from '../../data/keys'
import { t } from '../../i18n/vi'

const tt = t.channels.videos.bulk

export type BulkOutcome = { ok: string[]; failed: number }

const refreshShared = () => {
  void invalidate(['channels', 'jobs'])
  void invalidate(keys.queue())
  void invalidate(keys.status())
  void invalidate(['channels', 'performance'])
}

/** Chạy một thao tác cho các video hợp lệ. Trả id thành công để nơi gọi bỏ chọn. */
export async function runBulkAction(action: BulkAction, ids: readonly string[]): Promise<BulkOutcome> {
  const outcome: BulkOutcome = { ok: [], failed: 0 }
  try {
    for (let from = 0; from < ids.length; from += BULK_MAX) {
      const rows = await runBulk(action, ids.slice(from, from + BULK_MAX))
      for (const row of rows) {
        if (row.ok) outcome.ok.push(row.jobId)
        else outcome.failed++
      }
    }
  } catch (error) {
    toast.err(error instanceof Error ? error.message : tt.failed)
    refreshShared()
    return { ok: outcome.ok, failed: ids.length - outcome.ok.length }
  }
  if (outcome.failed > 0) toast.err(tt.partial(outcome.ok.length, outcome.failed))
  else toast.ok(tt.done[action](outcome.ok.length))
  refreshShared()
  return outcome
}

/** Huỷ/Xoá: hỏi trước vì không hoàn tác được. */
export async function confirmBulk(action: 'cancel' | 'delete', ids: readonly string[]): Promise<BulkOutcome | null> {
  let outcome: BulkOutcome | null = null
  const remove = action === 'delete'
  const accepted = await confirm({
    title: remove ? tt.removeTitle(ids.length) : tt.cancelTitle(ids.length),
    body: remove ? tt.removeBody : tt.cancelBody,
    cancelLabel: tt.keep,
    confirmLabel: remove ? tt.removeOk : tt.cancelOk,
    onConfirm: async () => { outcome = await runBulkAction(action, ids) },
  })
  return accepted ? outcome : null
}

/** Tải gói ZIP: server gộp tối đa 50 video mỗi tệp, nhiều hơn thì mỗi 50 video một tệp, tải lần lượt. */
export function downloadBundle(ids: readonly string[]) {
  toast.ok(tt.downloading(ids.length))
  for (let from = 0, turn = 0; from < ids.length; from += BUNDLE_MAX, turn++) {
    const href = bundleUrl(ids.slice(from, from + BUNDLE_MAX))
    window.setTimeout(() => {
      const link = document.createElement('a')
      link.href = href
      link.rel = 'noreferrer'
      document.body.append(link)
      link.click()
      link.remove()
    }, turn * 600)
  }
}
