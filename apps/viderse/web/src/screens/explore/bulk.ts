// Kết quả xếp hàng loạt từ Khám phá. Server trả `items` (queue item MỚI) và
// `duplicates` (URL đã có job hoặc đã nằm trong hàng chờ của kênh đích). Toast
// "đã làm" chỉ đúng khi thật sự có item mới — hàm này tách hai nhóm để caller
// đếm đúng và giữ lựa chọn cho những video chưa xếp được.
import type { CloneEnqueueResult } from '../../api/clone'

export type BulkEnqueueOutcome = {
  /** số video thật sự vào hàng chờ ở lần bấm này */
  added: number
  /** số video đã chọn mà server báo đã có sẵn cho kênh đích */
  duplicateCount: number
  /** id video đã chọn còn kẹt ở nhóm trùng — giữ lựa chọn để chọn kênh khác */
  duplicateVideoIds: string[]
  /** job của video trùng đầu tiên; null = chỉ mới ở hàng chờ (hoặc không có trùng) */
  duplicateJobId: string | null
}

export function bulkEnqueueOutcome(
  result: CloneEnqueueResult,
  selected: readonly { id: string; url: string }[],
): BulkEnqueueOutcome {
  const duplicates = result.duplicates ?? []
  const duplicatedUrls = new Set(duplicates.map((duplicate) => duplicate.url))
  return {
    added: result.items.length,
    duplicateCount: duplicates.length,
    duplicateVideoIds: selected.filter((video) => duplicatedUrls.has(video.url)).map((video) => video.id),
    duplicateJobId: duplicates.find((duplicate) => duplicate.jobId)?.jobId ?? null,
  }
}
