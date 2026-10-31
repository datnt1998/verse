// roundSummary — một vòng ✻ (công cụ + tham số gốc) → MỘT câu tiếng Việt thường, và
// ghép vòng với dòng Nhật ký của cùng lần gọi để lấy kết quả (“đã đọc 1.546 chữ”).
//
// Đây là phần nối tiếp của bảng nhãn `web/src/lib/labels.ts` (nhãn công cụ/trường), nên
// chuỗi tiếng Việt nằm ngay trong tệp này — invariant #7: tên công cụ gốc, tên tham số và
// giá trị mặc định không bao giờ ra ngoài “Chi tiết kỹ thuật”.
//
// Tham số đọc từ `parseNodes` (internal/host/stream_extract.go): mọi giá trị 0 / null /
// false / rỗng là “không truyền”, không phải một chương 0 hay một bản nháp rỗng.
import type { ActivityEvent, HostEvent } from '../api/types'
import { errorLabel, saneChapter } from './labels'
import { formatNumber } from './format'
import { translateHost } from '../i18n/hostStrings'
import { fieldChildren, fieldValue, type StreamNode, type StreamRound } from './streamRounds'

/** Dòng Nhật ký của một lần gọi công cụ (HostEvent trên dây mang thêm các trường này). */
export type ToolEvent = HostEvent & Partial<Pick<ActivityEvent, 'tool' | 'args' | 'result' | 'resultHint' | 'status' | 'chapter' | 'durationMs'>>

/** Cắt chữ dài bằng “…” — bản đầy đủ nằm trong tooltip của dòng tóm tắt. */
const clip = (text: string, max = 60): string => {
  const chars = [...text.trim()]
  return chars.length > max ? `${chars.slice(0, max).join('').trimEnd()}…` : text.trim()
}

/** Giá trị tham số thật: 0 / `null` / `false` / rỗng = không truyền. */
function arg(nodes: StreamNode[], key: string): string {
  const raw = (fieldValue(nodes, key) ?? '').trim()
  if (raw === '' || raw === 'null' || raw === 'false') return ''
  if (/^\d+$/.test(raw) && Number(raw) === 0) return ''
  return raw
}

const count = (nodes: StreamNode[], key: string): number => Number(arg(nodes, key)) || 0

/** Số phần tử của mảng tên `key`, tìm ở gốc rồi trong `content` (args thật bọc trong content). */
function listCount(nodes: StreamNode[], key: string): number {
  const direct = fieldChildren(nodes, key)
  if (direct.length > 0) return direct.length
  const content = fieldChildren(nodes, 'content')
  const nested = fieldChildren(content, key)
  if (nested.length > 0) return nested.length
  return content.filter((node) => node.item).length
}

/** Dòng stream rút gọn không có `content:` — các phần tử mảng nằm ngay dưới `type:`. */
const rootItemCount = (nodes: StreamNode[]): number => nodes.filter((node) => node.item).length

/** Rebuild full structured action data, including nested objects and arrays. */
export function nodesFromArgs(raw: string | undefined): StreamNode[] {
  if (!raw) return []
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return []
  }
  if (!parsed || typeof parsed !== 'object') return []
  const convert = (key: string, value: unknown, item = false): StreamNode => {
    if (Array.isArray(value)) {
      return { key, value: null, item, children: value.map((entry) => convert('', entry, true)) }
    }
    if (value !== null && typeof value === 'object') {
      return { key, value: null, item, children: Object.entries(value).map(([name, entry]) => convert(name, entry)) }
    }
    return { key, value: value === null ? null : String(value), item, children: [] }
  }
  const nodes = Object.entries(parsed)
    .filter(([, value]) => value !== null && value !== undefined && value !== '' && value !== 0 && value !== false)
    .map(([key, value]) => convert(key, value))
  return nodes
}

/** Một câu tóm tắt cho một lần gọi công cụ; công cụ chưa có bảng → '' (giữ nguyên nhãn bước). */
export function toolSummary(tool: string, nodes: StreamNode[], total = 0): string {
  // `total` = tổng số chương của truyện: số chương vô lý (canh 2147483647, lớn hơn tổng) bị bỏ.
  const chapter = saneChapter(count(nodes, 'chapter'), total) ?? 0
  const from = saneChapter(count(nodes, 'from'), total) ?? 0
  const to = saneChapter(count(nodes, 'to'), total) ?? 0
  const draft = arg(nodes, 'source') === 'draft'
  switch (tool) {
    case 'novel_context':
    case 'read_context':
      return chapter > 0 ? `Xem bối cảnh chương ${chapter}` : 'Xem bối cảnh truyện'
    case 'read_chapter':
      if (chapter > 0) return draft ? `Đọc lại bản nháp chương ${chapter}` : `Đọc lại chương ${chapter}`
      if (from > 0 && to > 0) return `Đọc chương ${from}–${to}${draft ? ' · bản nháp' : ''}`
      if (from > 0) return `Đọc từ chương ${from}${draft ? ' · bản nháp' : ''}`
      if (to > 0) return `Đọc đến chương ${to}${draft ? ' · bản nháp' : ''}`
      return draft ? 'Đọc lại bản nháp' : 'Đọc chương'
    case 'check_consistency':
      return chapter > 0 ? `Soát nhất quán chương ${chapter}` : 'Soát nhất quán'
    case 'plan_chapter': {
      const head = chapter > 0 ? `Lập kế hoạch chương ${chapter}` : 'Lập kế hoạch chương'
      const title = arg(nodes, 'title')
      return title ? `${head} · “${clip(title)}”` : head
    }
    case 'draft_chapter':
      if (chapter === 0) return 'Viết bản nháp'
      return arg(nodes, 'mode') === 'append' ? `Viết tiếp chương ${chapter}` : `Viết bản nháp chương ${chapter}`
    case 'edit_chapter':
      if (chapter === 0) return 'Gọt giũa'
      return arg(nodes, 'replace_all') === 'true' ? `Sửa cả chương ${chapter}` : `Gọt giũa chương ${chapter}`
    case 'commit_chapter':
    case 'save_chapter':
      return chapter > 0 ? `Nộp chương ${chapter}` : 'Nộp chương'
    case 'save_review': {
      const scope = arg(nodes, 'scope')
      if (scope === 'global') return 'Thẩm duyệt toàn truyện'
      if (scope === 'arc') {
        const arc = count(nodes, 'arc')
        return arc > 0 ? `Thẩm duyệt cung ${arc}` : 'Thẩm duyệt cung'
      }
      return chapter > 0 ? `Thẩm duyệt chương ${chapter}` : 'Thẩm duyệt'
    }
    case 'save_arc_summary': {
      const arc = count(nodes, 'arc')
      const volume = count(nodes, 'volume')
      if (arc > 0 && volume > 0) return `Tóm tắt cung ${arc} · tập ${volume}`
      if (arc > 0) return `Tóm tắt cung ${arc}`
      return 'Tóm tắt cung'
    }
    case 'save_volume_summary': {
      const volume = count(nodes, 'volume')
      return volume > 0 ? `Tóm tắt tập ${volume}` : 'Tóm tắt tập'
    }
    case 'save_book': {
      const title = arg(nodes, 'title')
      return title ? `Lưu thông tin truyện “${clip(title)}”` : 'Lưu thông tin truyện'
    }
    case 'save_foundation':
      return foundationSummary(nodes)
    case 'revise_outline': {
      const fromChapter = saneChapter(count(nodes, 'from_chapter'), total)
      return fromChapter ? `Sửa đại cương từ chương ${fromChapter}` : 'Sửa đại cương'
    }
    case 'resolve_outline_feedback':
      return 'Xử lý góp ý đại cương'
    case 'audit_foundation':
      return 'Soát lại nền truyện'
    case 'reopen_book': {
      const chapters = fieldChildren(nodes, 'chapters').length || listCount(nodes, 'chapters')
      return chapters > 0 ? `Mở lại truyện · ${chapters} chương` : 'Mở lại truyện'
    }
    default: {
      const pending = [...nodes]
      while (pending.length > 0) {
        const node = pending.shift()!
        if (node.value !== null && node.value.trim() !== '') return translateHost(node.value)
        pending.push(...node.children)
      }
      return ''
    }
  }
}

/** `save_foundation` gói nhiều loại thiết lập trong cùng một công cụ — câu nói theo `type`. */
function foundationSummary(nodes: StreamNode[]): string {
  const listed = (key: string) => listCount(nodes, key) || rootItemCount(nodes)
  switch (arg(nodes, 'type')) {
    case 'premise':
      return 'Lưu tiền đề truyện'
    case 'characters': {
      const n = listed('characters')
      return n > 0 ? `Lưu hồ sơ ${n} nhân vật` : 'Lưu hồ sơ nhân vật'
    }
    case 'world_rules': {
      const n = listed('world_rules')
      return n > 0 ? `Lưu ${n} quy tắc thế giới` : 'Lưu quy tắc thế giới'
    }
    case 'outline':
    case 'expand_arc': {
      const n = listCount(nodes, 'outline') || listCount(nodes, 'chapters') || rootItemCount(nodes)
      return n > 0 ? `Lưu dàn ý · ${n} chương` : 'Lưu dàn ý'
    }
    case 'layered_outline': {
      const n = listCount(nodes, 'volumes') || rootItemCount(nodes)
      return n > 0 ? `Lưu dàn ý · ${n} tập` : 'Lưu dàn ý'
    }
    case 'append_volume':
      return 'Thêm tập mới'
    case 'update_compass':
      return 'Cập nhật hướng đi'
    case 'complete_book':
      return 'Hoàn tất truyện'
    default:
      return 'Lưu thiết lập'
  }
}

/** Trợ lý chắc chắn sở hữu công cụ; công cụ dùng chung (đọc/soát/kế hoạch) → '' (lấy từ dòng Nhật ký). */
const TOOL_AGENT: Record<string, string> = {
  save_book: 'architect', save_foundation: 'architect', revise_outline: 'architect',
  resolve_outline_feedback: 'architect', audit_foundation: 'architect',
  save_review: 'editor', save_arc_summary: 'editor', save_volume_summary: 'editor',
}

/** Mã vai của công cụ ('' khi công cụ dùng chung cho nhiều trợ lý). */
export const roundAgentOf = (tool: string): string => TOOL_AGENT[tool] ?? ''

const TECHNICAL = /^[[{]|[a-z0-9]_[a-z0-9]/i

/** Một dòng kết quả cho thẻ đã xong: “đã đọc 1.546 chữ”, “2 vấn đề”, “đã lưu”… '' = không có gì để nói. */
export function toolResultLine(event: ToolEvent): string {
  if (event.failed || event.level === 'error') return errorLabel(event.result) || 'không xong'
  if (event.status === 'running') return ''
  // `resultHint`: số liệu trích từ kết quả gốc trước khi host cắt `result` 800 ký tự.
  const raw = (event.resultHint || event.result || '').trim()
  if (raw === '') return ''
  const tool = event.tool ?? ''
  const words = /"word_count":(\d+)/.exec(raw)
  if (words) {
    const size = `${formatNumber(Number(words[1]))} chữ`
    switch (tool) {
      case 'read_chapter': return `đã đọc ${size}`
      case 'check_consistency': return `đã soát ${size}`
      case 'commit_chapter':
      case 'save_chapter': return `đã lưu · ${size}`
      case 'edit_chapter': return `đã sửa · ${size}`
      default: return `đã viết ${size}`
    }
  }
  if (/"skipped":\s*true/.test(raw)) return 'bỏ qua vì chương đã nộp'
  if (/"saved":\s*true/.test(raw)) {
    const issues = /"issues":(\d+)/.exec(raw)
    if (issues && Number(issues[1]) > 0) return `đã lưu · ${formatNumber(Number(issues[1]))} vấn đề`
    const count = /"count":(\d+)/.exec(raw)
    if (count && tool === 'save_foundation') {
      let type = ''
      try {
        const args: unknown = JSON.parse(event.args ?? '')
        if (args && typeof args === 'object' && 'type' in args && typeof args.type === 'string') type = args.type
      } catch { /* args cắt cụt: chỉ cần số lượng, câu chung là đủ */ }
      const size = formatNumber(Number(count[1]))
      if (type === 'characters') return `đã lưu ${size} nhân vật`
      if (type === 'world_rules') return `đã lưu ${size} quy tắc`
      return `đã lưu ${size} mục`
    }
    return 'đã lưu'
  }
  if (/"foundation_ready":\s*true/.test(raw)) return 'nền truyện đã sẵn sàng'
  if (/"planned":\s*true/.test(raw)) return 'đã lập kế hoạch'
  if (/"written":\s*true/.test(raw)) return 'đã viết'
  if (/"committed":\s*true/.test(raw)) return 'đã lưu chương'
  if (/"ok":\s*true/.test(raw) || raw === 'ok') return 'xong'
  if (TECHNICAL.test(raw)) return 'đã xong'
  return translateHost(clip(raw, 90))
}

/** Cặp tham số dùng để nhận ra dòng Nhật ký của đúng lần gọi (0 = không truyền). */
const MATCH_KEYS = ['chapter', 'from', 'to', 'type', 'source', 'scope', 'arc', 'volume'] as const

const sameArgs = (round: StreamRound, event: ToolEvent): boolean => {
  const wanted = MATCH_KEYS.map((key) => [key, arg(round.nodes, key)] as const).filter(([, value]) => value !== '')
  if (wanted.length === 0) return true
  let parsed: Record<string, unknown>
  try {
    parsed = JSON.parse(event.args ?? '') as Record<string, unknown>
  } catch {
    return true
  }
  if (!parsed || typeof parsed !== 'object') return true
  return wanted.every(([key, value]) => {
    const actual = parsed[key]
    if (actual === undefined || actual === null || actual === 0 || actual === '') return true
    return String(actual) === value
  })
}

/**
 * Ghép từng vòng ✻ với dòng Nhật ký của cùng lần gọi. Cùng một công cụ gọi nhiều lần thì so
 * tham số (chương/khoảng/loại), ghép từ mới về cũ để không lấy lần gọi trước khi có bản ghi luồng.
 * Trả về map `round.id` → sự kiện; thiếu thì thẻ chỉ hiện tóm tắt.
 */
export function matchRoundEvents(rounds: StreamRound[], events: ToolEvent[]): Map<string, ToolEvent> {
  const queues = new Map<string, ToolEvent[]>()
  for (const event of events) {
    if ((event.category ?? '').toUpperCase() !== 'TOOL' || !event.tool) continue
    const list = queues.get(event.tool) ?? []
    // Một lần gọi phát hai dòng cùng id (đang chạy → xong): giữ dòng sau (đã có kết quả).
    const at = event.id ? list.findIndex((item) => item.id === event.id) : -1
    if (at >= 0) list[at] = event
    else list.push(event)
    queues.set(event.tool, list)
  }
  const matched = new Map<string, ToolEvent>()
  for (const round of [...rounds].reverse()) {
    const list = queues.get(round.tool)
    if (!list || list.length === 0) continue
    let index = list.length - 1
    while (index >= 0 && !sameArgs(round, list[index])) index--
    matched.set(round.id, list.splice(index >= 0 ? index : list.length - 1, 1)[0])
  }
  return matched
}
