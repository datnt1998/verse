// Mô hình hiển thị của một ảnh trên màn Duyệt: ghép dữ liệu §26 (phiên bản,
// prompt đã gửi, ảnh tham chiếu) với các trường cũ của cùng endpoint (lời kể,
// điểm Kiểm định, khung giờ, trạng thái duyệt) và tiến độ SSE đang chạy.
import type { AiProvider } from '../../api/ai'
import type { ImageProgress, ImageRef, ImageVersion, JobImage, PromptPartKind } from '../../api/images'
import { reviewLabel, type ReviewImage } from './useReview'

/** Trường cũ đi kèm payload §26 (schema của client để chúng đi qua). */
type JobImageExtra = {
  narration?: string
  direction?: string
  score?: unknown
  status?: string
  approved?: boolean
  referenceUrl?: string
  reference?: string
  startSec?: number
  endSec?: number
  sceneId?: string
  width?: number
  height?: number
}

export interface ReviewItemView {
  id: string
  kind: 'scene' | 'cover'
  label: string
  index: number
  status: string
  approved: boolean
  narration?: string
  direction?: string
  score?: unknown
  scoreIssue?: string
  referenceUrl?: string
  reference?: string
  startSec?: number
  endSec?: number
  usedVersion: number
  refLimit: number
  versions: ImageVersion[]
  progress?: ImageProgress
  /** URL ảnh gốc theo dữ liệu cũ khi chưa có phiên bản §26. */
  legacyUrl?: string
}

/** Bản §26 chưa có → dựng một phiên bản đọc được từ dữ liệu cũ của endpoint. */
function legacyVersion(item: ReviewImage): ImageVersion[] {
  const version = item.versions.find((candidate) => candidate.version === (item.reviewVersion ?? item.currentVersion))
  const url = version?.originalUrl ?? null
  if (!url) return []
  return [{
    n: version?.version ?? 1,
    state: 'ready',
    url,
    previewUrl: null,
    width: item.width ?? 1280,
    height: item.height ?? 720,
    parts: [],
    sentText: item.prompt ?? '',
    refs: item.referenceUrl ? [{
      id: 'reference',
      role: 'layout',
      url: item.referenceUrl,
      source: { type: 'rival_frame', label: item.reference || 'Khung đối thủ làm mẫu' },
    }] : [],
    provider: '',
    model: '',
    params: { size: `${item.width ?? 1280}x${item.height ?? 720}` },
    feedback: null,
    diffFromPrev: null,
    startedAt: '',
    finishedAt: null,
    durationSec: null,
    costImages: 0,
    error: null,
    score: version?.scoreIssue ? undefined : version?.score,
    scoreIssue: version?.scoreIssue,
  }]
}

/**
 * Ghép danh sách ảnh để hiển thị. `images` (§26) thắng về phiên bản/prompt;
 * danh sách cũ vẫn là nguồn của lời kể, điểm, khung giờ và trạng thái duyệt.
 */
export function toReviewItems(
  legacy: ReviewImage[] | undefined,
  images: JobImage[] | undefined,
  progress: Record<string, ImageProgress>,
): ReviewItemView[] {
  if (images?.length) {
    return images
      .map((image) => {
        const extra = image as JobImage & JobImageExtra
        const old = legacy?.find((item) => item.id === image.id)
        return {
          id: image.id,
          kind: image.kind,
          label: image.label,
          index: image.index,
          status: extra.status ?? old?.status ?? 'review',
          approved: extra.approved ?? old?.status === 'approved',
          narration: extra.narration ?? old?.narration,
          direction: extra.direction ?? old?.direction,
          score: image.scoreIssue || old?.scoreIssue ? undefined : extra.score ?? old?.score,
          scoreIssue: image.scoreIssue ?? old?.scoreIssue,
          referenceUrl: extra.referenceUrl ?? old?.referenceUrl,
          reference: extra.reference ?? old?.reference,
          startSec: extra.startSec ?? old?.startSec,
          endSec: extra.endSec ?? old?.endSec,
          usedVersion: image.usedVersion,
          refLimit: image.refLimit,
          versions: image.versions,
          progress: progress[image.id],
          legacyUrl: old ? imageURLOf(old) : undefined,
        } satisfies ReviewItemView
      })
      .sort((a, b) => Number(b.kind === 'cover') - Number(a.kind === 'cover') || a.index - b.index)
  }
  return (legacy ?? []).map<ReviewItemView>((item, index) => ({
    id: item.id,
    kind: item.kind === 'thumbnail' ? 'cover' : 'scene',
    label: reviewLabel(item, index),
    index: index + 1,
    status: item.status,
    approved: item.status === 'approved',
    narration: item.narration,
    direction: item.direction,
    score: item.scoreIssue ? undefined : item.score,
    scoreIssue: item.scoreIssue,
    referenceUrl: item.referenceUrl,
    reference: item.reference,
    startSec: item.startSec,
    endSec: item.endSec,
    usedVersion: item.currentVersion ?? 1,
    refLimit: 4,
    versions: legacyVersion(item),
    progress: progress[item.id],
    legacyUrl: imageURLOf(item),
  })).sort((a, b) => Number(b.kind === 'cover') - Number(a.kind === 'cover') || a.index - b.index)
}

function imageURLOf(item: ReviewImage): string | undefined {
  return item.versions.find((version) => version.version === (item.reviewVersion ?? item.currentVersion))?.originalUrl
}

/** Ảnh hiển thị của một bản (bản đang vẽ thì ảnh xem dần, không có thì ảnh cũ). */
export function versionImage(item: ReviewItemView, version: ImageVersion | undefined, fallback: string | undefined): string | undefined {
  if (!version) return fallback
  return version.previewUrl || version.url || (version.state === 'drawing' ? fallback : undefined)
}

export const SUBMITTED_PARTS = ['scene', 'style', 'refs', 'avoid'] as const
export const PART_LABELS: Partial<Record<PromptPartKind, string>> = {
  scene: 'Cảnh',
  style: 'Phong cách',
  refs: 'Tham chiếu',
  avoid: 'Tránh',
}

/** Phần người dùng sửa được (Tham chiếu sửa bằng thêm/bỏ ảnh, không bằng chữ). */
export const EDITABLE_PARTS: PromptPartKind[] = ['scene', 'style', 'avoid']

export const PROVIDER_NAMES: Record<string, string> = {
  chatgpt: 'ChatGPT',
  grok: 'Grok',
  gemini: 'Gemini',
  openai: 'OpenAI',
  ollama: 'Ollama',
}

/** Tên hiển thị của nguồn vẽ: nhãn thẻ Kết nối AI theo id (như Cài đặt/Trợ lý AI), sau đó tên loại nguồn; id lạ giữ nguyên. */
export const providerName = (provider: string, providers?: readonly Pick<AiProvider, 'id' | 'label'>[]) =>
  providers?.find((item) => item.id === provider)?.label ?? PROVIDER_NAMES[provider] ?? provider

const hhmm = (iso: string | null) => {
  if (!iso) return ''
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

/**
 * Bản mới nhất còn dùng được: bản đã huỷ vẫn nằm trong lịch sử để xem, nhưng
 * không bao giờ tự được chọn làm bản đang xem hay nguồn vẽ lại.
 */
export function defaultVersion(versions: readonly ImageVersion[] | undefined): ImageVersion | undefined {
  return versions?.reduce<ImageVersion | undefined>(
    (latest, version) => version.state !== 'cancelled' && (!latest || version.n > latest.n) ? version : latest,
    undefined,
  )
}

/** “Bản 2 · Theo góp ý “…” · 23:58” hoặc “Lần đầu · Hoạ sĩ viết từ lời kể · 23:58”. */
export function versionNote(version: ImageVersion | undefined): string {
  if (!version) return ''
  if (version.state === 'cancelled') return `Bản ${version.n} · đã huỷ · không dùng làm nguồn vẽ lại`
  const origin = version.feedback ? `Theo góp ý “${version.feedback}”` : 'Lần đầu · Hoạ sĩ viết từ lời kể'
  const when = hhmm(version.finishedAt ?? version.startedAt)
  return [`Bản ${version.n}`, origin, when].filter(Boolean).join(' · ')
}

/** Dòng dưới prompt: “ChatGPT · gpt-image-2 · 1280×720 · vẽ 44 giây · 23:58 · tốn 1 ảnh”. */
export function versionMeta(version: ImageVersion | undefined, providers?: readonly Pick<AiProvider, 'id' | 'label'>[]): string {
  if (!version) return ''
  const size = version.params.size ? version.params.size.replace('x', '×') : version.width && version.height ? `${version.width}×${version.height}` : ''
  return [
    providerName(version.provider, providers),
    version.model,
    size,
    version.durationSec != null ? `vẽ ${Math.round(version.durationSec)} giây` : '',
    hhmm(version.finishedAt),
    version.costImages ? `tốn ${version.costImages} ảnh` : '',
  ].filter(Boolean).join(' · ')
}

/** Số chỗ Hoạ sĩ đổi: mỗi cụm thêm/bỏ liền nhau tính một chỗ. */
export function diffGroups(diff: Array<[string, '' | 'ins' | 'del']> | null): number {
  if (!diff?.length) return 0
  let groups = 0
  let inside = false
  for (const [, kind] of diff) {
    if (kind) {
      if (!inside) groups += 1
      inside = true
    } else inside = false
  }
  return groups
}

/** “So với bản 2: Hoạ sĩ đổi 2 chỗ theo góp ý của bạn.” */
export function diffSummary(version: ImageVersion | undefined): string | null {
  if (!version?.diffFromPrev) return null
  const groups = diffGroups(version.diffFromPrev)
  if (!groups) return null
  const previous = version.n - 1
  return version.feedback
    ? `So với bản ${previous}: Hoạ sĩ đổi ${groups} chỗ theo góp ý của bạn.`
    : `So với bản ${previous}: Hoạ sĩ đổi ${groups} chỗ.`
}

/**
 * “Xem nguyên văn gửi máy chủ”: đúng chuỗi nhà cung cấp nhận, từng byte —
 * hàng rào an toàn, các phần sửa được, rồi chỉ dẫn công cụ của nguồn vẽ.
 * Tham số gửi kèm là dòng riêng bên dưới khối này, không nằm trong prompt.
 */
export function rawSentText(version: ImageVersion | undefined): string {
  return version?.sentText ?? ''
}

/** “size: 1280x720 · quality: high · references: 2” — tham số gửi kèm lời gọi. */
export function rawParams(version: ImageVersion | undefined): string {
  if (!version) return ''
  return [
    version.params.size ? `size: ${version.params.size}` : '',
    version.params.quality ? `quality: ${version.params.quality}` : '',
    `references: ${version.refs.length}`,
  ].filter(Boolean).join(' · ')
}

/** Bề rộng gốc của ảnh → khung không bao giờ phóng quá (`--nat-w`). */
export function stageWidthVar(version: ImageVersion | undefined): string {
  const width = version?.width ?? 0
  return `${width > 0 ? width : 1280}px`
}

/** “đặt lại lúc 07:00” — giờ thật nhà cung cấp báo; không có thì “đặt lại 00:00”. */
export function resetAtLabel(retryAt: string | undefined): string {
  const time = hhmm(retryAt ?? null)
  return `đặt lại lúc ${time || '00:00'}`
}

/** Chỉ những phần đã sửa so với bản đang dùng — đúng thứ gửi lên `POST …/redraw`. */
export function changedParts(
  base: ImageVersion | undefined,
  edits: Partial<Record<PromptPartKind, string>>,
): Partial<Record<PromptPartKind, string>> {
  const changed: Partial<Record<PromptPartKind, string>> = {}
  for (const [kind, text] of Object.entries(edits) as Array<[PromptPartKind, string | undefined]>) {
    if (text === undefined) continue
    const original = base?.parts.find((part) => part.kind === kind)?.text ?? ''
    if (text !== original) changed[kind] = text
  }
  return changed
}

export const REF_ROLES: Array<{ role: ImageRef['role']; label: string }> = [
  { role: 'character', label: 'Nhân vật' },
  { role: 'layout', label: 'Bố cục' },
  { role: 'style', label: 'Phong cách' },
  { role: 'object', label: 'Đồ vật' },
]
