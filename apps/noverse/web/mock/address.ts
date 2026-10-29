// Mock GAP-38 (xưng hô & quan hệ — 22-brief-2510.md D7/D8) và GAP-39 (Não truyện — D10).
// Chỉ chạy ở chế độ mock; dữ liệu sống trong bộ nhớ của tiến trình mock.
import type { IncomingMessage, ServerResponse } from 'node:http'
import type { AddressTerm, Brain, BrainEdge, BrainEntity, RelationChange } from '../src/api/types'

type Json = Record<string, unknown>
const send = (res: ServerResponse, status: number, data: unknown) => {
  const body = JSON.stringify(data)
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'content-length': Buffer.byteLength(body) })
  res.end(body)
}
const fail = (res: ServerResponse, status: number, message: string) => send(res, status, { error: message })

export type MockAddress = { terms: AddressTerm[]; relations: RelationChange[] }
const states = new Map<string, MockAddress>()

/** Bảng xưng hô khởi tạo — dòng đầu đã hết hiệu lực (artboard Outline.dc.html › person=detail).
    `from`/`to`/`a`/`b` là id thực thể như contract (06-data-contract › Não truyện), tên hiển thị ở `*Name`. */
const seed = (cast: string[]): MockAddress => {
  const [a, b, c, d] = cast
  if (!a || !b) return { terms: [], relations: [] }
  const idOf = (name: string) => `nt-${name}`
  const term = (id: string, from: string, to: string, call: string, self: string, fromChapter: number, untilChapter: number, reason: string): AddressTerm =>
    ({ id, from: idOf(from), fromName: from, to: idOf(to), toName: to, call, self, fromChapter, untilChapter, reason })
  return {
    terms: [
      term('xh-1', a, b, 'chị Nga', 'em', 1, 1, 'Đồng nghiệp cũ, còn giữ phép'),
      term('xh-2', a, b, 'cô Nga', 'tôi', 2, 0, 'Cắt đứt sau khi biết bị cướp tên'),
      term('xh-3', b, a, 'em', 'chị', 1, 0, 'Giữ vẻ đàn chị trước người khác'),
      ...(c ? [term('xh-4', a, c, 'anh Phát', 'tôi', 1, 0, 'Quan hệ công việc')] : []),
      ...(d ? [term('xh-5', d, a, 'cô My', 'tôi', 1, 0, 'Xa cách, trịch thượng')] : []),
    ],
    relations: [
      { chapter: 1, a: idOf(a), aName: a, b: idOf(b), bName: b, before: 'Bạn đồng nghiệp', after: 'Đối thủ', reason: `${b} nhận vai chính trong kịch bản ${a} viết, đẩy ${a} xuống vai phụ.`, type: 'enemy' },
      { chapter: 2, a: idOf(a), aName: a, b: idOf(b), bName: b, before: 'Đối thủ', after: 'Cắt đứt', reason: `${a} biết ${b} đứng tên tác giả. Đổi cách gọi từ “chị Nga” sang “cô”.`, type: 'enemy' },
      ...(d ? [{ chapter: 2, a: idOf(a), aName: a, b: idOf(d), bName: d, before: 'Người lạ', after: 'Đồng minh', reason: `Cùng nhóm trong show, ${d} bênh ${a} trước máy quay.`, type: 'ally' as const }] : []),
      ...(c ? [{ chapter: 3, a: idOf(a), aName: a, b: idOf(c), bName: c, before: 'Nghi ngờ', after: 'Tin tưởng một phần', reason: `${c} giữ lại bản live đọc thoại thay vì xoá.`, type: 'rel' as const }] : []),
    ],
  }
}

/** `seeded` = bảng xưng hô dựng sẵn của truyện mẫu (mock/brain.ts); không có thì seed từ dàn nhân vật. */
export const addressStateOf = (key: string, cast: string[], seeded?: MockAddress | null): MockAddress => {
  const existing = states.get(key)
  if (existing) return existing
  const created = seeded ?? seed(cast)
  states.set(key, created)
  return created
}

const range = (from: number, to: number) => Array.from({ length: Math.max(0, to - from + 1) }, (_, index) => from + index)

export type AddressApiInput = {
  req: IncomingMessage
  res: ServerResponse
  p: string
  s: string
  rest: string[]
  state: MockAddress
  latestWritten: number
  writing: { chapter: number; role: string; state: 'writing' | 'rewriting' } | null
  canWrite: boolean
  readBody: () => Promise<Json>
  changed: () => void
}

let newTermSeq = 0

/** Id thực thể → tên hiển thị (contract: `a`/`from` là id, `aName`/`fromName` là tên). */
const nameOfId = (id: string) => MOCK_CAST.find((item) => `nt-${item.name}` === id)?.name ?? id

/** `GET/PUT S/address-terms` và `POST S/relations` — cùng mã lỗi với contract. */
export async function handleAddressApi(input: AddressApiInput) {
  const { req, res, rest, state, latestWritten, writing, canWrite, readBody, changed } = input
  const payload = () => ({ terms: state.terms, relations: state.relations, latestWritten, writing })
  const isRelations = rest[0] === 'relations'
  if (!isRelations && req.method === 'GET') return send(res, 200, payload())
  if (!canWrite) return fail(res, 404, 'Không tìm thấy truyện')
  if (writing) {
    return send(res, 423, {
      error: `Người viết ${writing.state === 'writing' ? 'đang viết' : 'đang viết lại'} chương ${writing.chapter}, nên xưng hô và quan hệ đang khoá. Hãy tạm dừng để sửa.`,
      writing,
    })
  }
  const body = await readBody()
  if (body.apply && body.apply !== 'next' && body.apply !== 'mark') return fail(res, 400, 'Cách áp dụng không hợp lệ')
  const apply = body.apply === 'next' || body.apply === 'mark' ? body.apply : undefined
  const fromChapter = Number(isRelations ? body.fromChapter : (body.term as Json | undefined)?.fromChapter)
  if (!Number.isInteger(fromChapter) || fromChapter < 1) return fail(res, 400, 'Chương bắt đầu phải từ 1 trở lên')

  if (isRelations) {
    const a = String(body.a ?? '')
    const b = String(body.b ?? '')
    const after = String(body.after ?? '').trim()
    const reason = String(body.reason ?? '').trim()
    if (!a || !b || a === b) return fail(res, 400, 'Quan hệ cần hai nhân vật khác nhau')
    if (!after) return fail(res, 400, 'Chưa nhập quan hệ sau khi đổi')
    // Câu 20: sửa trên web bắt buộc lý do — máy chủ trả 400 `{field:"reason"}`.
    if (!reason) return send(res, 400, { error: 'Chưa nhập lý do cho thay đổi quan hệ', field: 'reason' })
    if (fromChapter <= latestWritten && !apply) {
      return send(res, 409, { error: 'Thay đổi này chạm chương đã viết', affected: [fromChapter] })
    }
    const previous = state.relations.filter((item) => (item.a === a && item.b === b) || (item.a === b && item.b === a)).at(-1)
    const type = body.type === 'enemy' || body.type === 'ally' || body.type === 'work' || body.type === 'own' || body.type === 'secret' || body.type === 'rel' ? body.type : 'rel'
    state.relations.push({ chapter: fromChapter, a, aName: nameOfId(a), b, bName: nameOfId(b), before: previous?.after ?? '', after, reason, type })
    changed()
    return send(res, 200, payload())
  }

  const term = (body.term ?? {}) as Json
  const from = String(term.from ?? '')
  const to = String(term.to ?? '')
  const call = String(term.call ?? '').trim()
  const self = String(term.self ?? '').trim()
  const reason = String(term.reason ?? '').trim()
  if (!from || !to || from === to) return fail(res, 400, 'Một dòng xưng hô cần hai nhân vật khác nhau')
  if (!call) return fail(res, 400, 'Chưa nhập cách gọi')
  if (!self) return fail(res, 400, 'Chưa nhập cách tự xưng')
  if (!reason) return send(res, 400, { error: 'Chưa nhập lý do cho dòng xưng hô', field: 'reason' })
  const id = term.id ? String(term.id) : ''
  const existing = id ? state.terms.find((item) => item.id === id) : undefined
  if (id && !existing) return fail(res, 404, 'Không tìm thấy dòng xưng hô')
  if (existing?.untilChapter && existing.untilChapter <= latestWritten) return fail(res, 409, 'Dòng xưng hô đã hết hiệu lực')
  if (fromChapter <= latestWritten && !apply) {
    return send(res, 409, { error: `Sửa này chạm tới ${range(fromChapter, latestWritten).length} chương đã viết`, affected: range(fromChapter, latestWritten) })
  }
  // “next” chỉ đổi chương chưa viết → dòng mới bắt đầu sau chương mới nhất đã viết (D8).
  const effectiveFrom = apply === 'next' ? Math.max(fromChapter, latestWritten + 1) : fromChapter
  if (existing) {
    const until = effectiveFrom - 1
    if (until >= existing.fromChapter) existing.untilChapter = until
    else state.terms = state.terms.filter((item) => item !== existing)
  }
  state.terms.push({ id: `xh-new-${++newTermSeq}`, from, fromName: nameOfId(from), to, toName: nameOfId(to), call, self, fromChapter: effectiveFrom, untilChapter: 0, reason })
  changed()
  return send(res, 200, payload())
}

/** Nhân vật của truyện mẫu (khớp GET characters) — Não truyện dựng entity từ danh sách này. */
export const MOCK_CAST: { name: string; role: string; description: string }[] = [
  { name: 'Kiều My', role: 'main', description: 'Tác giả mới — ký sai hợp đồng rồi đoạt lại tác quyền bằng chữ' },
  { name: 'Trần Nga', role: 'main', description: 'Chủ phòng quy hoạch — người giữ bản thảo tay và chìa khóa của mọi cuộc đấu' },
  { name: 'Nhân Phát', role: 'main', description: 'Biên tập viên — người đánh máy bản thảo đầu tiên và tin vào Kiều My' },
  { name: 'Quách', role: 'antagonist', description: 'Phụ trách tác quyền — chủ của những bản ký kết mờ ám' },
  { name: 'bà Hiền', role: 'supporting', description: 'Chủ trà cũ' },
  { name: 'biên tập viên Minh', role: 'supporting', description: 'Bạn đồng nghiệp của Nhân Phát' },
  { name: 'giám đốc Hùng', role: 'supporting', description: 'Đứng đầu nhà xuất bản' },
  // Tên có danh xưng (04 › Avatar nhân vật) — Não truyện của truyện thường cần đủ hình dạng tên.
  { name: 'Luật sư Hàn', role: 'supporting', description: 'Cố vấn pháp lý của nhà xuất bản' },
  { name: 'Cô lao công Lụa', role: 'supporting', description: 'Người lao công của toà nhà thu' },
]
/** Số chương nhân vật có mặt (D7 dùng cho hàng “Xuất hiện” và dòng meta của thẻ). */
const APPEARS: Record<string, number[]> = {
  'Kiều My': [1, 2, 3, 4, 5, 6, 7, 8],
  'Trần Nga': [1, 2, 3, 4, 5, 6, 7],
  'Nhân Phát': [1, 2, 4, 5, 6],
  Quách: [1, 2, 3, 5, 6, 7],
  'bà Hiền': [1, 3, 4, 5],
  'biên tập viên Minh': [3, 6],
  'giám đốc Hùng': [1, 4, 7],
  'Luật sư Hàn': [2, 5],
  'Cô lao công Lụa': [4],
}

export type BrainInput = {
  chapters: { n: number; title: string }[]
  written: number
  writingChapter: number | null
  /** `rewriting` khi Người viết đang sửa lại; `writing` khi viết lần đầu (Câu 23). */
  writingStatus?: 'writing' | 'rewriting'
  state: MockAddress
}

/** `GET S/brain` — đủ hình dạng contract; màn D7 chỉ đọc `chapters` + `entities[].appears`. */
export const mockBrainOf = ({ chapters, written, writingChapter, writingStatus = 'rewriting', state }: BrainInput): Brain => {
  const entities: BrainEntity[] = MOCK_CAST.map((item) => ({
    id: `nt-${item.name}`,
    type: 'char',
    name: item.name,
    role: item.role,
    desc: item.description,
    fromChapter: (APPEARS[item.name] ?? [1])[0],
    appears: (APPEARS[item.name] ?? chapters.map((chapter) => chapter.n)).filter((n) => n <= chapters.length),
    aliases: [],
    timeline: [],
  }))
  const edges: BrainEdge[] = []
  for (const relation of state.relations) {
    const key = [relation.a, relation.b].sort().join('|')
    let edge = edges.find((item) => [item.a, item.b].sort().join('|') === key)
    if (!edge) {
      edge = { a: relation.a, b: relation.b, phases: [] }
      edges.push(edge)
    }
    edge.phases.push({ fromChapter: relation.chapter, type: relation.type ?? 'rel', label: relation.after })
  }
  for (const edge of edges) edge.phases.sort((left, right) => left.fromChapter - right.fromChapter)
  return {
    chapters: chapters.map((chapter) => ({
      n: chapter.n,
      title: chapter.title,
      status: chapter.n <= written ? 'written' : chapter.n === writingChapter ? writingStatus : 'unwritten',
    })),
    entities,
    edges,
  }
}
