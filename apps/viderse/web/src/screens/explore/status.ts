// Nhãn + màu trạng thái thật của video đã làm cho một kênh (ia-flows §3.1).
// Màu khớp artboard Explore và bảng trạng thái; không suy diễn ngoài stage/
// bước đang chạy mà API trả.
import type { UsageStage, VideoUsage } from '../../api/rivals'
import { t } from '../../i18n/explore'
import { paths } from '../../router'

const STAGE_COLOR: Record<UsageStage, string> = {
  queued: '#8E8E99',
  working: '#B9D0F7',
  review: '#F6CB8C',
  attention: '#FF9A5C',
  paused: '#8E8E99',
  ready: '#A6EDC1',
  posted: '#8B8B97',
}

export function stageLabel(usage: VideoUsage): string {
  const stage = t.screens.explore.usageStage
  if (usage.stage === 'working' && usage.activeStep === 'images') return stage.workingImages
  return stage[usage.stage]
}

export function stageColor(stage: UsageStage): string {
  return STAGE_COLOR[stage] ?? STAGE_COLOR.queued
}

/** Dòng phụ của kênh bị khoá ở Video đã chọn. */
export function usageSubline(usage: VideoUsage): string {
  const tt = t.screens.explore
  return usage.stage === 'queued' ? tt.usageQueuedSub : tt.usageMadeSub(stageLabel(usage))
}

/** Đường mở video: Bàn dựng khi đã có job, Hàng chờ khi còn ở hàng chờ. */
export function usageHref(usage: VideoUsage): string {
  return `#${usage.jobId ? paths.workbench({ video: usage.jobId }) : paths.queue()}`
}
