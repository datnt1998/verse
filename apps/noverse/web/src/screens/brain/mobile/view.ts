// M8 · Não truyện trên điện thoại — các phép suy ra riêng của bản 390 px.
// Mọi thứ dùng chung với bản máy tính (bảng màu, trạng thái tại chương N, xưng hô, dòng thời gian,
// thanh chương) nằm ở `../model`; tệp này chỉ xếp chúng thành hình dạng của MBrain.dc.html.
import type { AddressTerms, Brain, BrainEntity, BrainEntityType, CharacterKind } from '../../../api/types'
import { t } from '../../../i18n/vi'
import { KIND_META, charKindOf } from '../../../lib/charKind'
import {
  EDGE_META, TYPE_META, addressRowsAt, aliveAt, changedAt, changedCountAt, chapterTone,
  edgesAt, entityView, isLiveChapter, isNewAt, narratorOf, relsAt, timelineAt, type ChapterTone,
} from '../model'

/** Thứ tự nhóm của MBrain (`const order` trong artboard) — khác thứ tự nhóm của bản máy tính. */
export const PHONE_ORDER: BrainEntityType[] = ['char', 'obj', 'ev', 'fs', 'place', 'rule']

/** Câu chân danh sách của MBrain (artboard, không có ở bản máy tính). */
export const PHONE_FOOTNOTE = 'Điện thoại chỉ hiện danh sách. Xem đồ thị quan hệ trên máy tính.'

export type FilterChip = { key: 'all' | BrainEntityType; label: string; icon: string | null; color: string; count: number; on: boolean }
export type PhoneItem = {
  id: string; name: string; isChar: boolean; ini: string
  /** Loại vai + “chưa lộ” (Gói S) — MBrain tô nút theo loại, không còn gradient riêng từng người. */
  kind: CharacterKind; hidden: boolean
  icon: string; color: string; role: string
  isNew: boolean; changed: boolean
  rel: string | null; relColor: string; relDashed: boolean; more: number
}
export type PhoneGroup = { type: BrainEntityType; label: string; count: number; items: PhoneItem[] }
export type SheetRelation = { label: string; other: string; color: string; dashed: boolean; since: string }
export type SheetAddress = { from: string; to: string; call: string; self: string; was: string | null }
export type SheetEvent = { chapter: number; text: string; future: boolean }
export type SheetModel = {
  id: string; name: string; isChar: boolean; ini: string; kind: CharacterKind; hidden: boolean
  icon: string; color: string; sub: string; desc: string
  notYet: boolean; fromChapter: number
  relations: SheetRelation[]; address: SheetAddress[]; events: SheetEvent[]
}

const lower = (value: string) => value.trim().toLocaleLowerCase('vi')

const matches = (entity: BrainEntity, query: string) => !query.trim() || lower(entity.name).includes(lower(query))

/** Chip lọc loại tại chương N: số đếm theo chương + từ khoá; chip rỗng bị ẩn trừ khi đang chọn. */
export function filterChips(brain: Brain, chapter: number, query: string, selected: 'all' | BrainEntityType): FilterChip[] {
  const visible = brain.entities.filter((entity) => aliveAt(entity, chapter) && matches(entity, query))
  const chips: FilterChip[] = [{ key: 'all', label: 'Tất cả', icon: null, color: '', count: visible.length, on: selected === 'all' }]
  for (const type of PHONE_ORDER) {
    const meta = TYPE_META[type]
    chips.push({
      key: type, label: meta.label, icon: meta.icon, color: meta.color,
      count: visible.filter((entity) => entity.type === type).length,
      on: selected === type,
    })
  }
  return chips.filter((chip) => chip.count > 0 || chip.on)
}

/** Danh sách nhóm theo loại tại chương N (MBrain không vẽ đồ thị). */
export function phoneGroups(brain: Brain, chapter: number, selected: 'all' | BrainEntityType, query: string): PhoneGroup[] {
  const narrator = narratorOf(brain.entities)
  const visible = brain.entities.filter((entity) => aliveAt(entity, chapter) && matches(entity, query) && (selected === 'all' || entity.type === selected))
  return PHONE_ORDER.flatMap((type) => {
    const items = visible.filter((entity) => entity.type === type)
    if (!items.length) return []
    return [{
      type, label: TYPE_META[type].label, count: items.length,
      items: items.map((entity) => {
        const view = entityView(entity, chapter, narrator)
        const rels = relsAt(brain, entity.id, chapter)
        const top = rels[0]
        const isNew = isNewAt(entity, chapter)
        return {
          id: entity.id,
          name: entity.name,
          isChar: view.isChar,
          ini: entity.ini ?? '',
          kind: view.kind,
          hidden: view.hidden,
          icon: view.type.icon,
          color: view.type.color,
          role: view.role,
          isNew,
          changed: changedAt(brain, entity.id, chapter) && !isNew,
          rel: top ? `${top.label} · ${top.otherName}` : null,
          relColor: top?.color ?? '',
          relDashed: top?.dashed ?? false,
          more: Math.max(0, rels.length - 1),
        }
      }),
    }]
  })
}

/** “N thực thể · M quan hệ” trên thanh trên. */
export function countText(brain: Brain, chapter: number, query: string): string {
  const entities = brain.entities.filter((entity) => aliveAt(entity, chapter) && matches(entity, query)).length
  return `${entities} thực thể · ${edgesAt(brain, chapter).length} quan hệ`
}

export type ChapterHead = { note: string; chip: { label: string; tone: ChapterTone } }
export type ChapterCell = { n: number; tone: ChapterTone; on: boolean; aria: string }

/**
 * Hàng đầu thẻ chương (rail-m8-v3): dòng phụ ghi số thay đổi, chương chưa viết ghi “Theo dàn ý…”;
 * chip trạng thái Đã viết · Đang viết lại / Đang viết · Chưa viết.
 */
export function chapterHead(brain: Brain, chapter: number): ChapterHead {
  const status = brain.chapters.find((item) => item.n === chapter)?.status ?? 'unwritten'
  const label = t.brain.status[status]
  const chip = { label: label.charAt(0).toUpperCase() + label.slice(1), tone: chapterTone(status) }
  if (status === 'unwritten') return { note: 'Theo dàn ý · chưa có trong bản viết', chip }
  const changed = changedCountAt(brain, chapter)
  if (isLiveChapter(status)) return { note: `Người viết ${label} · ${changed ? `${changed} thay đổi` : 'chưa đổi gì'}`, chip }
  return { note: changed ? `${changed} thay đổi so với chương trước` : 'Không đổi so với chương trước', chip }
}

/** Ô số dưới hàng đầu (artboard `.seg8`): sọc đáy theo trạng thái, ô đang xem `aria-checked`. */
export function chapterCells(chapters: Brain['chapters'], chapter: number): ChapterCell[] {
  return chapters.map((item) => ({
    n: item.n,
    tone: chapterTone(item.status),
    on: item.n === chapter,
    aria: t.brain.railAria(item.n, item.title, t.brain.status[item.status]),
  }))
}

/** Câu 17: quá 10 chương thì hàng ô cuộn ngang (`segCls = len > 10 ? 'scroll' : ''` trong artboard);
 *  ≤ 10 chương chia đều cả hàng, không cuộn. */
export const RAIL_SCROLL_OVER = 10

/**
 * Câu 17: `scrollLeft` để ô đang xem nằm chính giữa khung, đúng công thức của artboard
 * (`row.scrollLeft = on.offsetLeft - (row.clientWidth - on.offsetWidth) / 2`).
 * `cellLeft` là khoảng cách từ mép trái khung cuộn (nay đúng bằng mép hàng — hàng không còn đệm) tới mép
 * trái ô, nên không cần `scrollLeft` hiện tại.
 */
export function scrollLeftToCenter(cellLeft: number, cellWidth: number, viewWidth: number): number {
  // Đầu truyện không đủ chỗ để canh giữa: trình duyệt kẹp ở 0, trả luôn 0 cho khớp kết quả thấy được.
  return Math.max(0, cellLeft - (viewWidth - cellWidth) / 2)
}

export function sheetModel(brain: Brain, address: AddressTerms | null, id: string, chapter: number): SheetModel | null {
  const entity = brain.entities.find((item) => item.id === id)
  if (!entity) return null
  const view = entityView(entity, chapter, narratorOf(brain.entities))
  return {
    id: entity.id,
    name: entity.name,
    isChar: view.isChar,
    ini: entity.ini ?? '',
    kind: view.kind,
    hidden: view.hidden,
    icon: view.type.icon,
    color: view.type.color,
    sub: `${view.role} · có từ chương ${entity.fromChapter}`,
    desc: entity.desc,
    notYet: entity.fromChapter > chapter,
    fromChapter: entity.fromChapter,
    relations: relsAt(brain, entity.id, chapter).map((rel) => ({
      label: rel.label,
      other: rel.otherName,
      color: rel.color,
      dashed: rel.dashed,
      since: `${EDGE_META[rel.type].label} · từ chương ${rel.sinceChapter}`,
    })),
    address: address
      ? addressRowsAt(address, entity, chapter).map((row) => ({ from: row.from, to: row.to, call: row.call, self: row.self, was: row.was ?? null }))
      : [],
    events: timelineAt(entity, brain, address ?? { terms: [] }, chapter)
      .slice(0, 6)
      .map((event) => ({ chapter: event.chapter, text: event.text, future: event.future })),
  }
}

export type PersonRow = { label: string; value: string; note: string | null }
export type PersonQuickView = {
  id: string; name: string; role: string; fromChapter: number; desc: string
  kind: CharacterKind; hidden: boolean; ini: string
  rows: PersonRow[]
}

/**
 * Xem nhanh một nhân vật trong Trình đọc (MReader.dc.html `state=person`): vai, quan hệ với nhân vật
 * chính tại chương đang đọc, xưng hô hai chiều. Chỉ dùng dữ liệu tới chương đang đọc.
 */
export function personQuickView(brain: Brain, address: AddressTerms | null, entityId: string, chapter: number): PersonQuickView | null {
  const entity = brain.entities.find((item) => item.id === entityId)
  if (!entity || entity.type !== 'char') return null
  const view = entityView(entity, chapter, narratorOf(brain.entities))
  const main = narratorOf(brain.entities)
  const relations = relsAt(brain, entity.id, chapter)
  const otherId = main && main !== entity.id ? main : relations[0]?.otherId ?? null
  const other = brain.entities.find((item) => item.id === otherId) ?? null
  const rows: PersonRow[] = []
  if (other) {
    const relation = relations.find((rel) => rel.otherId === other.id)
    if (relation) rows.push({ label: `Với ${other.name}`, value: relation.label, note: `từ chương ${relation.sinceChapter}` })
    const terms = address ? addressRowsAt(address, entity, chapter) : []
    const mine = terms.find((row) => row.from === entity.name && row.to === other.name)
    const theirs = terms.find((row) => row.from === other.name && row.to === entity.name)
    if (mine) rows.push({ label: `Gọi ${other.name}`, value: `“${mine.call}”`, note: `tự xưng “${mine.self}”` })
    if (theirs) rows.push({ label: `${other.name} gọi`, value: `“${theirs.call}”`, note: `tự xưng “${theirs.self}”` })
  }
  return {
    id: entity.id,
    name: entity.name,
    role: view.role,
    fromChapter: entity.fromChapter,
    desc: entity.desc,
    kind: view.kind,
    hidden: view.hidden,
    ini: entity.ini ?? '',
    rows,
  }
}

/** Tên dùng để gạch chấm trong chính văn: tên chính + `aliases` của mọi nhân vật đã xuất hiện. */
export function characterNames(brain: Brain, chapter: number): { name: string; id: string; color: string }[] {
  const names: { name: string; id: string; color: string }[] = []
  for (const entity of brain.entities) {
    if (entity.type !== 'char' || !aliveAt(entity, chapter)) continue
    const { fg } = KIND_META[charKindOf(entity)]
    for (const name of [entity.name, ...(entity.aliases ?? [])]) {
      if (name.trim()) names.push({ name: name.trim(), id: entity.id, color: fg })
    }
  }
  // Tên dài trước để "Tống Nhược Vy" thắng "Nhược Vy" khi cùng bắt đầu ở một vị trí.
  return names.sort((a, b) => b.name.length - a.name.length)
}

export type TextSegment = { text: string; id?: string; color?: string }

/** Chữ cái + dấu của mọi hệ chữ. `\b` của JS chỉ hiểu ASCII nên biên từ phải tự kiểm bằng lớp này. */
const WORD_CHAR = /[\p{L}\p{M}]/u

/**
 * Tên gọi trần ngắn — một từ, tối đa 2 chữ cái (“An”, “Vy”). Ở đầu câu tên và đại từ đều viết hoa
 * (“Anh bắt máy”, “An mở cửa”), không phân biệt được bằng chữ; luật chọn phía thà thiếu: KHÔNG gạch
 * tên gọi trần ngắn ở đầu câu, chỉ gạch khi có tên đầy đủ (“Lâm An”) hoặc khi tên nằm giữa câu.
 */
const SHORT_GIVEN_NAME = /^[\p{L}\p{M}]{1,2}$/u
/** Dấu ngắt câu — sau nó là đầu câu mới (kể cả dấu hai chấm mở lời thoại). */
const SENTENCE_PAUSE = /[.!?…:;]/
/** Khoảng trắng và dấu bao quanh đứng trước một từ mà không làm nó thành giữa câu. */
const SENTENCE_WRAP = /[\s"'“‘(\[—–-]/

/** Vị trí này có phải đầu câu: lùi qua khoảng trắng/dấu mở ngoặc, gặp dấu ngắt câu là đúng. */
const startsSentence = (text: string, index: number): boolean => {
  let at = index - 1
  while (at >= 0) {
    const char = text[at]
    if (char === '\n' || SENTENCE_PAUSE.test(char)) return true
    if (!SENTENCE_WRAP.test(char)) return false
    at--
  }
  return true
}

/**
 * Tên khớp ở `index`: đúng nguyên từ (không dính chữ hai đầu), đúng dấu và đúng hoa/thường
 * (so sánh chuỗi sau NFC, không bỏ dấu). `names` đã xếp tên dài trước nên tên dài thắng.
 */
const nameAt = (text: string, index: number, names: { name: string; id: string; color?: string }[]) => {
  for (const item of names) {
    if (!text.startsWith(item.name, index)) continue
    const end = index + item.name.length
    const before = text[index - 1]
    const after = text[end]
    if ((before !== undefined && WORD_CHAR.test(before)) || (after !== undefined && WORD_CHAR.test(after))) continue
    if (SHORT_GIVEN_NAME.test(item.name) && startsSentence(text, index)) continue
    return item
  }
  return null
}

/**
 * Cắt một đoạn văn thành các mảnh: mảnh có `id` là tên nhân vật cần gạch chấm.
 * Chữ được chuẩn hoá NFC trước khi so để NFD (dấu rời, bản sao từ máy khác) vẫn khớp.
 */
export function splitByNames(text: string, names: { name: string; id: string; color?: string }[]): TextSegment[] {
  const source = text.normalize('NFC')
  const candidates = names
    .map((item) => ({ ...item, name: item.name.normalize('NFC').trim() }))
    .filter((item) => item.name)
    .sort((left, right) => right.name.length - left.name.length)
  if (!candidates.length) return [{ text: source }]
  const segments: TextSegment[] = []
  let plain = 0
  let index = 0
  while (index < source.length) {
    const hit = nameAt(source, index, candidates)
    if (!hit) { index++; continue }
    if (index > plain) segments.push({ text: source.slice(plain, index) })
    segments.push({ text: hit.name, id: hit.id, color: hit.color })
    index += hit.name.length
    plain = index
  }
  if (plain < source.length) segments.push({ text: source.slice(plain) })
  return segments
}
