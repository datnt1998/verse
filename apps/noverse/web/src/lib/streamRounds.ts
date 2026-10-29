// Parser `streamText` (SSE `delta`) → round ✻ + văn xuôi + khối suy nghĩ.
// Định dạng nguồn: internal/host/stream_extract.go (JSON args → YAML-like):
//   ✻ <header>\nkey: value\n  - item\n  key_con:\n    - name: …
// Giá trị chuỗi nhiều dòng nối tiếp trên dòng không có `key:` và không thụt lề.
// `\x02` (ThinkingSep) bao đoạn suy nghĩ: tách theo dấu, đoạn lẻ là suy nghĩ.
// 06-data-contract.md › Luồng viết (`streamText`) → StreamRound.
import { MAX_ROUNDS, ROUND_MARKER, THINKING_MARK, roundSpecFor, type RoundKind } from '../i18n/hostStrings'

/** Một dòng `key: value` hoặc phần tử mảng `- …` đã parse. */
export type StreamNode = {
  /** '' cho phần tử mảng dạng primitive */
  key: string
  /** null khi node chỉ có con (hoặc giá trị rỗng) */
  value: string | null
  /** true khi bắt đầu bằng `- ` */
  item: boolean
  children: StreamNode[]
}

export type StreamRound = {
  /** khoá React ổn định trong phạm vi một lần parse */
  id: string
  /** header gốc sau ✻ (tiếng Trung hoặc tên tool) */
  header: string
  /** nhãn tiếng Việt theo bảng 06 */
  labelVi: string
  tool: string
  kind: RoundKind
  /** thân round sau khi bỏ khối suy nghĩ */
  body: string
  nodes: StreamNode[]
  /** khối suy nghĩ nằm trong round (đã bỏ \x02) */
  thinking: string
  /** round cuối khi agent đang chạy */
  live: boolean
  /** mở sẵn: round đang chạy, edit_chapter, commit_chapter */
  defaultOpen: boolean
}

export type StreamBlock =
  | { kind: 'prose'; text: string }
  | { kind: 'thinking'; text: string }

/** Một mục của luồng theo đúng thứ tự xuất hiện (văn xuôi, suy nghĩ, round). */
export type StreamItem =
  | { kind: 'prose'; text: string }
  | { kind: 'thinking'; text: string }
  | { kind: 'round'; round: StreamRound }

export type ParsedStream = {
  rounds: StreamRound[]
  /** mọi mục theo thứ tự — dùng để render luồng */
  items: StreamItem[]
  /** văn xuôi / suy nghĩ nằm ngoài round, giữ đúng thứ tự */
  blocks: StreamBlock[]
  /** văn xuôi ngoài round (bản nháp chương) */
  prose: string
  /** toàn bộ suy nghĩ (ngoài + trong round) */
  thinking: string
  /** tool của round đang chạy — '' khi không chạy */
  liveTool: string
}

const MARK = ROUND_MARKER.trim()
const KEY_LINE = /^([A-Za-z0-9_.-]+):(?:[ \t]+([\s\S]*))?$/

/**
 * Field có thể chứa chuỗi nhiều dòng (host ghi newline thật, dòng nối không thụt lề).
 * Dòng không khớp `key:` sau các field khác được coi là văn xuôi của round — nếu nối
 * nhầm vào chip (vd. `character: null` + đoạn văn) sẽ hỏng dữ liệu hiển thị.
 */
const MULTILINE_KEYS: Record<string, true> = {
  old_string: true, new_string: true, description: true, summary: true, notes: true, title: true,
  goal: true, conflict: true, hook: true, emotion_arc: true, opening_state: true, brief_role: true, text: true,
  content: true, reason: true, feedback: true, suggestion: true, message: true,
}

const trimBlank = (text: string) => text.replace(/^\n+/, '').replace(/\n+$/, '')

/** Tách đoạn theo \x02: đoạn chẵn = văn xuôi, đoạn lẻ = suy nghĩ. */
export function splitThinking(text: string): StreamBlock[] {
  const blocks: StreamBlock[] = []
  text.split(THINKING_MARK).forEach((part, index) => {
    const value = trimBlank(part)
    if (!value) return
    blocks.push({ kind: index % 2 === 1 ? 'thinking' : 'prose', text: value })
  })
  return blocks
}

const parseLine = (content: string): StreamNode | null => {
  if (content === '-' || content.startsWith('- ')) {
    const inline = content === '-' ? '' : content.slice(2).trim()
    if (!inline) return { key: '', value: null, item: true, children: [] }
    const match = KEY_LINE.exec(inline)
    if (match) return { key: match[1], value: match[2] === undefined ? null : match[2].trim(), item: true, children: [] }
    return { key: '', value: inline, item: true, children: [] }
  }
  const match = KEY_LINE.exec(content)
  if (!match) return null
  return { key: match[1], value: match[2] === undefined ? null : match[2].trim(), item: false, children: [] }
}

/** Parse thân YAML-like thành cây node. Dòng không khớp `key:` → nối vào giá trị trước. */
export function parseNodes(body: string): StreamNode[] {
  const root: StreamNode[] = []
  const stack: { node: StreamNode; indent: number }[] = []
  for (const raw of body.split('\n')) {
    if (!raw.trim()) continue
    const indent = raw.length - raw.trimStart().length
    const content = raw.trim()
    while (stack.length > 0 && indent <= stack[stack.length - 1].indent) stack.pop()
    const parent = stack.length > 0 ? stack[stack.length - 1].node : null
    const target = parent ? parent.children : root
    const node = parseLine(content)
    if (node === null) {
      const last = target[target.length - 1]
      if (last && last.value !== null && MULTILINE_KEYS[last.key]) last.value = `${last.value}\n${content}`
      else target.push({ key: '', value: content, item: false, children: [] })
      continue
    }
    target.push(node)
    // node có thể nhận con: object rỗng (key:), phần tử mảng (kể cả có key inline)
    if (node.value === null || node.item) stack.push({ node, indent })
  }
  return root
}

type RawSection =
  | { kind: 'text'; lines: string[] }
  | { kind: 'round'; header: string; lines: string[] }

type RawRound = { header: string; body: string; thinking: string }

const splitSections = (source: string): RawSection[] => {
  const sections: RawSection[] = [{ kind: 'text', lines: [] }]
  let current = sections[0] as RawSection
  for (const line of source.split('\n')) {
    if (line.trimStart().startsWith(MARK)) {
      const header = line.trimStart().slice(MARK.length).trim()
      current = { kind: 'round', header, lines: [] }
      sections.push(current)
      continue
    }
    current.lines.push(line)
  }
  return sections
}

/**
 * Parse `streamText` thành round + văn xuôi. `isRunning` quyết định round cuối là `live`.
 * Giữ tối đa MAX_ROUNDS round gần nhất (06-data-contract.md).
 */
export function parseStream(text: string, isRunning: boolean): ParsedStream {
  const source = text.replace(/\r\n?/g, '\n')
  const rawItems: ({ type: 'block'; block: StreamBlock } | { type: 'round'; round: RawRound })[] = []
  const rawRounds: RawRound[] = []

  for (const section of splitSections(source)) {
    const sectionBlocks = splitThinking(section.lines.join('\n'))
    if (section.kind === 'text') {
      sectionBlocks.forEach((block) => rawItems.push({ type: 'block', block }))
      continue
    }
    const thinking = sectionBlocks.filter((b) => b.kind === 'thinking').map((b) => b.text).join('\n\n')
    const body = sectionBlocks.filter((b) => b.kind === 'prose').map((b) => b.text).join('\n\n')
    const round: RawRound = { header: section.header, body, thinking }
    rawRounds.push(round)
    rawItems.push({ type: 'round', round })
  }

  const kept = rawRounds.slice(-MAX_ROUNDS)
  const built = new Map<RawRound, StreamRound>()
  kept.forEach((round, index) => {
    const spec = roundSpecFor(round.header)
    const live = isRunning && index === kept.length - 1
    built.set(round, {
      id: `${index}-${round.header}`,
      header: round.header,
      labelVi: spec.labelVi,
      tool: spec.tool,
      kind: spec.kind,
      body: round.body,
      nodes: parseNodes(round.body),
      thinking: round.thinking,
      live,
      defaultOpen: live || spec.kind === 'diff' || spec.kind === 'commit',
    })
  })

  const items: StreamItem[] = []
  const blocks: StreamBlock[] = []
  for (const item of rawItems) {
    if (item.type === 'block') {
      blocks.push(item.block)
      items.push(item.block.kind === 'prose' ? { kind: 'prose', text: item.block.text } : { kind: 'thinking', text: item.block.text })
      continue
    }
    const round = built.get(item.round)
    if (round) items.push({ kind: 'round', round })
  }
  const rounds = [...built.values()]

  const prose = blocks.filter((b) => b.kind === 'prose').map((b) => b.text).join('\n\n')
  const thinking = [...blocks.filter((b) => b.kind === 'thinking').map((b) => b.text), ...rounds.map((r) => r.thinking)]
    .filter(Boolean).join('\n\n')

  return {
    rounds,
    items,
    blocks,
    prose,
    thinking,
    liveTool: rounds.length > 0 && rounds[rounds.length - 1].live ? rounds[rounds.length - 1].tool : '',
  }
}

// ─── Truy cập node ──────────────────────────────────────────────────────

export const fieldNode = (nodes: StreamNode[], key: string): StreamNode | undefined =>
  nodes.find((node) => !node.item && node.key === key)

/** Giá trị chuỗi của field; null khi thiếu / rỗng / literal `null`. */
export function fieldValue(nodes: StreamNode[], key: string): string | null {
  const node = fieldNode(nodes, key)
  if (!node || node.value === null) return null
  const value = node.value.trim()
  return value === '' || value === 'null' ? null : value
}

/** Các field con của `key` (dùng cho characters, foreshadow_updates, key_events…). */
export const fieldChildren = (nodes: StreamNode[], key: string): StreamNode[] =>
  fieldNode(nodes, key)?.children ?? []

/** Field con đơn giản (không phải mảng) — cho thẻ trường của ✻ 规划. */
export const plainFields = (nodes: StreamNode[]): { key: string; value: string }[] =>
  nodes.filter((node) => !node.item && node.key !== '' && node.value !== null).map((node) => ({ key: node.key, value: node.value ?? '' }))

/** Văn xuôi và danh sách Markdown nằm ngoài các trường có cấu trúc của round. */
export function looseText(nodes: StreamNode[]): string {
  const rows = nodes.filter((node) => node.key === '' && node.value !== null)
  return rows.map((node, index) => {
    const separator = index === 0 ? '' : node.item && rows[index - 1]?.item ? '\n' : '\n\n'
    return `${separator}${node.item ? '- ' : ''}${node.value ?? ''}`
  }).join('')
}

/** Nhãn hiển thị cho giá trị thiếu: '—' (không bao giờ hiện placeholder của artboard). */
export const displayValue = (value: string | null | undefined): string => {
  const trimmed = value?.trim()
  return !trimmed || trimmed === 'null' ? '—' : trimmed
}
