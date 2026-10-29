// Logic thuần của Não truyện (22-brief-2510.md › D10, artboard Brain.dc.html › build()/paint()).
// Không JSX: bảng TY/ET/CHR, quan hệ – xưng hô – diễn biến tại chương N,
// thanh chương, kích thước thẻ và hình học cạnh dùng chung.
import type {
  AddressTerm, AddressTerms, Brain, BrainEdge, BrainEdgeType, BrainEntity, BrainEntityType, CharacterKind,
} from '../../api/types'
import { t } from '../../i18n/vi'
import { charInitials } from '../../lib/cast'
import { KIND_META, charKindOf, isKindHidden, type KindMeta } from '../../lib/charKind'

export type TypeMeta = { label: string; icon: string; color: string }
export type EdgeMeta = { label: string; color: string; dashed: boolean }

/** Artboard TY — thứ tự này cũng là thứ tự nhóm ở rail và ở chế độ Danh sách. */
export const TYPE_ORDER: BrainEntityType[] = ['char', 'place', 'obj', 'ev', 'fs', 'rule']
export const TYPE_META: Record<BrainEntityType, TypeMeta> = {
  char: { label: t.brain.types.char, icon: 'user', color: 'var(--brain-char)' },
  place: { label: t.brain.types.place, icon: 'global', color: 'var(--brain-place)' },
  obj: { label: t.brain.types.obj, icon: 'box', color: 'var(--brain-obj)' },
  ev: { label: t.brain.types.ev, icon: 'clock', color: 'var(--brain-ev)' },
  fs: { label: t.brain.types.fs, icon: 'flag', color: 'var(--brain-fs)' },
  rule: { label: t.brain.types.rule, icon: 'shield-security', color: 'var(--brain-rule)' },
}

/** Artboard ET — nét liền cho đối đầu/đồng minh/công việc/sở hữu, nét đứt cho ẩn và liên quan. */
export const EDGE_ORDER: BrainEdgeType[] = ['enemy', 'ally', 'work', 'own', 'secret', 'rel']
export const EDGE_META: Record<BrainEdgeType, EdgeMeta> = {
  enemy: { label: t.brain.edges.enemy, color: 'var(--brain-e-enemy)', dashed: false },
  ally: { label: t.brain.edges.ally, color: 'var(--brain-e-ally)', dashed: false },
  work: { label: t.brain.edges.work, color: 'var(--brain-e-work)', dashed: false },
  own: { label: t.brain.edges.own, color: 'var(--brain-e-own)', dashed: false },
  secret: { label: t.brain.edges.secret, color: 'var(--brain-e-secret)', dashed: true },
  rel: { label: t.brain.edges.rel, color: 'var(--brain-e-rel)', dashed: true },
}

/** Vai hiển thị dưới tên nhân vật trên thẻ nút — chuỗi của máy chủ, hiện nguyên văn. */
export const entityRole = (entity: BrainEntity) => entity.role || TYPE_META[entity.type].label

export type EntityView = {
  isChar: boolean
  initials: string
  role: string
  /** Loại vai (Gói S): màu nút/avatar trên đồ thị và ở ngăn chi tiết. */
  kind: CharacterKind
  kindMeta: KindMeta
  /** Chưa lộ với người nghe ở chương đang xem → viền nét đứt + nhãn “· chưa lộ”. */
  hidden: boolean
  /** Viền thẻ khi đang chọn = màu loại vai (nhân vật) hoặc màu loại thực thể (còn lại). */
  ring: string
  type: TypeMeta
  narrator: boolean
}

/** `ch` = chương đang xem (quyết định “chưa lộ”); bỏ trống thì coi như đã lộ hết. */
export const entityView = (entity: BrainEntity, ch = 0, narratorId: string | null = null): EntityView => {
  const isChar = entity.type === 'char'
  const kind = charKindOf(entity)
  const kindMeta = KIND_META[kind]
  const hidden = isChar && isKindHidden(entity.revealAt, ch)
  return {
    isChar,
    initials: charInitials(entity.name, entity.ini),
    role: hidden ? t.brain.hiddenRole(entityRole(entity)) : entityRole(entity),
    kind,
    kindMeta,
    hidden,
    ring: isChar ? kindMeta.border : TYPE_META[entity.type].color,
    type: TYPE_META[entity.type],
    narrator: isChar && entity.id === narratorId,
  }
}

/** Người kể = nhân vật đầu tiên của máy chủ (artboard đặt người kể ở tâm đồ thị). */
export const narratorOf = (entities: BrainEntity[]): string | null =>
  entities.find((entity) => entity.type === 'char')?.id ?? null

// ─── Theo chương ─────────────────────────────────────────────────────────

export type BrainPhase = BrainEdge['phases'][number]

/** Kiểu + nhãn có hiệu lực tại chương `ch` (phase gần nhất có `fromChapter` ≤ ch). */
export const phaseAt = (phases: BrainPhase[], ch: number): BrainPhase | null =>
  phases.reduce<BrainPhase | null>((current, phase) => (phase.fromChapter <= ch ? phase : current), null)

export const aliveAt = (entity: Pick<BrainEntity, 'fromChapter'>, ch: number) => entity.fromChapter <= ch

/** Thẻ mới xuất hiện đúng ở chương đang xem (chương 1 không gắn badge “mới”). */
export const isNewAt = (entity: Pick<BrainEntity, 'fromChapter'>, ch: number) => entity.fromChapter === ch && ch > 1

/** Cạnh đổi kiểu/nhãn đúng ở chương đang xem → nhãn thêm “MỚI”. */
export const edgeFresh = (phase: Pick<BrainPhase, 'fromChapter'>, ch: number) => phase.fromChapter === ch && ch > 1

export type ActiveEdge = { edge: BrainEdge; index: number; phase: BrainPhase; fresh: boolean }

/** Cạnh còn hiệu lực tại `ch`: có phase, hai đầu đã xuất hiện. */
export const edgesAt = (brain: Pick<Brain, 'edges' | 'entities'>, ch: number): ActiveEdge[] => {
  const alive = new Set(brain.entities.filter((entity) => aliveAt(entity, ch)).map((entity) => entity.id))
  return brain.edges
    .map((edge, index) => ({ edge, index, phase: phaseAt(edge.phases, ch) }))
    .filter((item): item is ActiveEdge => item.phase !== null && alive.has(item.edge.a) && alive.has(item.edge.b))
    .map((item) => ({ ...item, fresh: edgeFresh(item.phase, ch) }))
}

export type RelationRow = {
  id: string
  label: string
  otherId: string
  otherName: string
  type: BrainEdgeType
  sinceChapter: number
  color: string
  dashed: boolean
  /** Cạnh bắt đầu đúng ở chương đang xem → dòng phụ “mới ở chương N” (ngăn kéo Main). */
  fresh: boolean
  /** “Đối đầu · từ chương 2” */
  since: string
}

const nameOf = (brain: Pick<Brain, 'entities'>, id: string) => brain.entities.find((entity) => entity.id === id)?.name ?? id

/** Quan hệ của một thực thể tại `ch` (không lọc theo kiểu — artboard dùng `allAct`). */
export const relsAt = (brain: Pick<Brain, 'edges' | 'entities'>, id: string, ch: number): RelationRow[] =>
  edgesAt(brain, ch)
    .filter((item) => item.edge.a === id || item.edge.b === id)
    .map((item) => {
      const otherId = item.edge.a === id ? item.edge.b : item.edge.a
      const meta = EDGE_META[item.phase.type]
      return {
        id: `${item.edge.a}-${item.edge.b}-${item.phase.fromChapter}`,
        label: item.phase.label,
        otherId,
        otherName: nameOf(brain, otherId),
        type: item.phase.type,
        sinceChapter: item.phase.fromChapter,
        color: meta.color,
        dashed: meta.dashed,
        fresh: item.fresh,
        since: `${meta.label} · ${t.brain.sinceChapter(item.phase.fromChapter)}`,
      }
    })

export const neighbourIds = (brain: Pick<Brain, 'edges' | 'entities'>, id: string, ch: number): string[] => {
  const out: string[] = []
  for (const item of edgesAt(brain, ch)) {
    if (item.edge.a === id) out.push(item.edge.b)
    else if (item.edge.b === id) out.push(item.edge.a)
  }
  return out
}

/** Số thực thể mỗi loại tại `ch` — số đếm cạnh checkbox của rail. */
export const countsAt = (brain: Pick<Brain, 'entities'>, ch: number): Record<BrainEntityType, number> => {
  const counts = { char: 0, place: 0, obj: 0, ev: 0, fs: 0, rule: 0 } as Record<BrainEntityType, number>
  for (const entity of brain.entities) if (aliveAt(entity, ch)) counts[entity.type] += 1
  return counts
}

/** Số cạnh mỗi kiểu tại `ch` — số đếm của nhóm “Kiểu quan hệ”. */
export const edgeCountsAt = (brain: Pick<Brain, 'edges' | 'entities'>, ch: number): Record<BrainEdgeType, number> => {
  const counts = { enemy: 0, ally: 0, work: 0, own: 0, secret: 0, rel: 0 } as Record<BrainEdgeType, number>
  for (const item of edgesAt(brain, ch)) counts[item.phase.type] += 1
  return counts
}

/** Thực thể/cạnh đổi đúng ở `ch` — dòng phụ “N thay đổi so với chương trước”. */
export const changedAt = (brain: Pick<Brain, 'edges'>, id: string, ch: number) =>
  brain.edges.some((edge) => (edge.a === id || edge.b === id) && edge.phases.some((phase) => phase.fromChapter === ch))

export const changedCountAt = (brain: Pick<Brain, 'edges' | 'entities'>, ch: number) =>
  brain.edges.filter((edge) => edge.phases.some((phase) => phase.fromChapter === ch)).length +
  brain.entities.filter((entity) => entity.fromChapter === ch).length

// ─── Xưng hô tại chương N (D8) ────────────────────────────────────────────

export type AddressRow = { id: string; from: string; to: string; call: string; self: string; was?: string }

const isActiveAt = (term: AddressTerm, ch: number) =>
  term.fromChapter <= ch && (!term.untilChapter || term.untilChapter >= ch)

/** Dòng “Đổi ở chương N, trước đó gọi “…” · lý do” — chỉ khi dòng mới thay dòng cũ cùng chiều. */
const changedLine = (terms: AddressTerm[], term: AddressTerm): string | undefined => {
  const previous = terms.find((item) =>
    item.from === term.from && item.to === term.to &&
    (item.untilChapter ?? 0) > 0 && (item.untilChapter ?? 0) < term.fromChapter)
  if (!previous) return undefined
  const reason = term.reason ? ` · ${term.reason}` : ''
  return `${t.brain.changedLine(term.fromChapter, previous.call)}${reason}`
}

/** Xưng hô trỏ tới thực thể: máy chủ gửi id + tên hiển thị, mock cũ gửi tên — nhận cả ba. */
const touches = (term: AddressTerm, entity: Pick<BrainEntity, 'id' | 'name'>) =>
  [term.from, term.to, term.fromName, term.toName].some((value) => value === entity.id || value === entity.name)

export const addressRowsAt = (address: Pick<AddressTerms, 'terms'>, entity: Pick<BrainEntity, 'id' | 'name'>, ch: number): AddressRow[] =>
  address.terms
    .filter((term) => touches(term, entity) && isActiveAt(term, ch))
    .map((term) => ({
      id: term.id,
      from: term.fromName || term.from,
      to: term.toName || term.to,
      call: term.call,
      self: term.self,
      was: changedLine(address.terms, term),
    }))

// ─── Đổi ở chương N (tab Truyện của Bàn viết, Gói P) ──────────────────────

export type ChangeRow = {
  id: string; text: string; color: string; entity: string
  who: string; rel: string; to: string; changed: boolean
}

/** Quan hệ bắt đầu/đổi ở chương N, chạm nhân vật; Đổi trước Mới, tối đa 3 dòng (Gói U). */
export const changesAt = (brain: Pick<Brain, 'edges' | 'entities'>, ch: number): ChangeRow[] => {
  const characters = new Set(brain.entities.filter((entity) => entity.type === 'char').map((entity) => entity.id))
  return edgesAt(brain, ch)
    .filter(({ edge, phase }) => phase.fromChapter === ch && (characters.has(edge.a) || characters.has(edge.b)))
    .map(({ edge, phase, index }) => ({
      id: `rel-${index}-${ch}`,
      text: `${phase.label} · ${nameOf(brain, edge.a)} – ${nameOf(brain, edge.b)}`,
      color: EDGE_META[phase.type].color,
      entity: edge.a,
      who: nameOf(brain, edge.a),
      rel: phase.label,
      to: nameOf(brain, edge.b),
      changed: edge.phases.some((item) => item.fromChapter < ch),
    }))
    .sort((a, b) => Number(b.changed) - Number(a.changed))
    .slice(0, 3)
}

// ─── Diễn biến ───────────────────────────────────────────────────────────

export type TimelineItem = { chapter: number; text: string; future: boolean }

/**
 * Diễn biến của một thực thể. Máy chủ gửi `entity.timeline` (BeBrain3) thì dùng nguyên văn;
 * chưa có thì suy từ đổi quan hệ + đổi xưng hô như artboard (Brain.dc.html › hist).
 */
export const timelineAt = (
  entity: BrainEntity,
  brain: Pick<Brain, 'edges' | 'entities'>,
  address: Pick<AddressTerms, 'terms'>,
  ch: number,
): TimelineItem[] => {
  const rows: { chapter: number; text: string }[] = []
  if (entity.timeline?.length) {
    for (const item of entity.timeline) rows.push({ chapter: item.chapter, text: item.text })
  } else {
    for (const edge of brain.edges) {
      if (edge.a !== entity.id && edge.b !== entity.id) continue
      const other = nameOf(brain, edge.a === entity.id ? edge.b : edge.a)
      edge.phases.forEach((phase, index) => {
        rows.push({ chapter: phase.fromChapter, text: `${index ? t.brain.changedTo : ''}${phase.label} · ${other}` })
      })
    }
    for (const term of address.terms) {
      if (term.fromChapter <= 1) continue
      if (!touches(term, entity)) continue
      rows.push({ chapter: term.fromChapter, text: t.brain.calls(term.fromName || term.from, term.toName || term.to, term.call) })
    }
  }
  const seen = new Set<string>()
  return rows
    .sort((left, right) => left.chapter - right.chapter)
    .filter((row) => {
      const key = `${row.chapter}|${row.text}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
    .slice(0, 7)
    .map((row) => ({ ...row, future: row.chapter > ch }))
}

// ─── Hàng “Xuất hiện” ────────────────────────────────────────────────────

export type AppearCell = { n: number; title: string; bg: string; fg: string; border: string }

/** Ô 1..N: có mặt = nền tím nhạt; chương đang xem viền tím; chưa có mặt chữ mờ (artboard `appear`). */
export const appearRow = (chapters: Brain['chapters'], appears: number[], ch: number): AppearCell[] =>
  chapters.map((chapter) => {
    const present = appears.includes(chapter.n)
    return {
      n: chapter.n,
      title: chapter.title,
      bg: present ? 'var(--brain-appear-bg)' : 'transparent',
      fg: present ? 'var(--color-fg)' : 'var(--brain-tick-num)',
      border: chapter.n === ch ? 'var(--line-focus)' : 'var(--line-3)',
    }
  })

// ─── Chương & thanh chương ───────────────────────────────────────────────

/** Chương đang viết (lần đầu `writing` hoặc sửa lại `rewriting`) — thanh chương vẽ cả hai như nhau (Câu 23). */
export const isLiveChapter = (status: Brain['chapters'][number]['status']) => status === 'writing' || status === 'rewriting'

/** Chương mở mặc định: chương đang viết, không thì chương đã viết cuối, không thì chương 1. */
export const initialChapter = (chapters: Brain['chapters']): number => {
  const live = chapters.find((chapter) => isLiveChapter(chapter.status))
  if (live) return live.n
  const written = chapters.filter((chapter) => chapter.status === 'written')
  return written.length ? written[written.length - 1].n : 1
}

/**
 * “Chương đang xem” ở Bàn viết = chương mới nhất đã nộp (Gói S · “ở Bàn viết là chương mới
 * nhất đã nộp”): quyết định avatar nào còn “chưa lộ”. 0 = chưa có chương nào đã nộp.
 */
export const latestWritten = (chapters: Brain['chapters']): number => {
  const written = chapters.filter((chapter) => chapter.status === 'written')
  return written.length ? written[written.length - 1].n : 0
}

export type ChapterTone = 'done' | 'live' | 'todo'
export type RailTick = { n: number; title: string; x: string; cls: ChapterTone; fg: string; aria: string }

const STATUS_LABEL = t.brain.status

/** Ba trạng thái chương: xanh = đã viết, tím = đang viết / viết lại, xám = chưa viết. */
export const chapterTone = (status: Brain['chapters'][number]['status']): ChapterTone =>
  status === 'written' ? 'done' : isLiveChapter(status) ? 'live' : 'todo'

/** Nấc thanh chương: chấm luôn tròn, vị trí tuyệt đối theo %, số chương cách chấm 15 px. */
export const railTicks = (chapters: Brain['chapters'], ch: number): RailTick[] => {
  const last = Math.max(1, chapters.length - 1)
  return chapters.map((chapter, index) => {
    const on = chapter.n === ch
    const cls = chapterTone(chapter.status)
    return {
      n: chapter.n,
      title: chapter.title,
      x: `${(index / last * 100).toFixed(3)}%`,
      cls,
      fg: on ? 'var(--color-fg)' : cls === 'todo' ? 'var(--brain-tick-num)' : 'var(--color-fg-3)',
      aria: t.brain.railAria(chapter.n, chapter.title, STATUS_LABEL[chapter.status]),
    }
  })
}

/** Đường nối tô tới chương đang viết (tiến độ truyện), không chạy theo chương đang xem. */
export const railFill = (chapters: Brain['chapters']): string => {
  const last = Math.max(1, chapters.length - 1)
  const live = chapters.findIndex((chapter) => isLiveChapter(chapter.status))
  const written = chapters.reduce((index, chapter, i) => (chapter.status === 'written' ? i : index), -1)
  const index = live >= 0 ? live : written
  return `${Math.max(0, index) / last * 100}%`
}

export type ChapterNote = { text: string; live: boolean }

/** Dòng phụ cạnh “Chương N”: đang viết / đang viết lại / N thay đổi / không đổi. */
export const chapterNote = (brain: Pick<Brain, 'chapters' | 'edges' | 'entities'>, ch: number): ChapterNote => {
  const status = brain.chapters.find((chapter) => chapter.n === ch)?.status
  if (status === 'writing') return { text: t.brain.noteWriting, live: true }
  if (status === 'rewriting') return { text: t.brain.noteRewriting, live: true }
  const changed = changedCountAt(brain, ch)
  return changed > 0
    ? { text: t.brain.noteChanged(changed), live: false }
    : { text: t.brain.noteSame, live: false }
}

// ─── Danh sách (bản thay thế cho trình đọc màn hình) ──────────────────────

export type ListItem = { id: string; name: string; since: string; rels: string; changed: string }
export type ListGroup = { type: BrainEntityType; label: string; icon: string; color: string; count: number; items: ListItem[] }

/** Nhóm theo loại; mỗi dòng: tên · từ chương · quan hệ tại N · “đổi ở ch. N”. */
export const listGroups = (brain: Pick<Brain, 'edges' | 'entities' | 'chapters'>, ch: number, query = ''): ListGroup[] => {
  const q = query.trim().toLowerCase()
  const alive = edgesAt(brain, ch)
  return TYPE_ORDER.map((type) => {
    const ids = brain.entities.filter((entity) =>
      entity.type === type && (!q || entity.name.toLowerCase().includes(q)))
    return {
      type,
      label: TYPE_META[type].label,
      icon: TYPE_META[type].icon,
      color: TYPE_META[type].color,
      count: ids.length,
      items: ids.map((entity) => {
        const rows = alive.filter((item) => item.edge.a === entity.id || item.edge.b === entity.id)
        return {
          id: entity.id,
          name: entity.name,
          since: aliveAt(entity, ch)
            ? t.brain.sinceChapter(entity.fromChapter)
            : `${t.brain.sinceChapter(entity.fromChapter)}${t.brain.notYetShort}`,
          rels: rows.length
            ? rows.map((item) => `${item.phase.label} · ${nameOf(brain, item.edge.a === entity.id ? item.edge.b : item.edge.a)}`).join(', ')
            : '—',
          changed: changedAt(brain, entity.id, ch) ? t.brain.changedAt(ch) : '',
        }
      }),
    }
  }).filter((group) => group.items.length > 0)
}

// ─── Đồ thị: vị trí, kích thước thẻ, cung, nhãn ───────────────────────────

export const VIEW = { x: 0, y: 0, w: 860, h: 690, cx: 430, cy: 345 } as const


export type Point = { x: number; y: number }


/** Bề rộng chữ tối đa của thẻ/nhãn (D10: 132 / 150 / 136 / 128 px). */
export const MAX_NAME = 132
export const MAX_NARRATOR = 150
export const MAX_ENTITY = 136
export const MAX_LABEL = 128

export type Box = { w: number; h: number }

export const charRadius = (narrator: boolean) => (narrator ? 18 : 15)
export const charHeight = (narrator: boolean) => (narrator ? 54 : 46)
/** Thẻ nhân vật: 6 + avatar + 10 + chữ + 16. */
export const charWidth = (narrator: boolean, textWidth: number) => Math.round(6 + charRadius(narrator) * 2 + 10 + textWidth + 16)
/** Thẻ thực thể khác: 10 + ô glyph 16 + 8 + chữ + 13, cao 32. */
export const otherWidth = (textWidth: number) => Math.round(10 + 16 + 8 + textWidth + 13)

export type GraphBounds = { x: number; y: number; w: number; h: number; cx: number; cy: number }

/** Preserve the artboard viewport while it fits; expand to include cards and curve extrema. */
export const graphBounds = (positions: ReadonlyMap<string, Point>, boxes: Readonly<Record<string, Box>>, curves: Curve[] = []): GraphBounds => {
  let x0 = VIEW.x as number
  let y0 = VIEW.y as number
  let x1 = VIEW.w as number
  let y1 = VIEW.h as number
  for (const [id, point] of positions) {
    const box = boxes[id]
    if (!box) continue
    x0 = Math.min(x0, point.x - box.w / 2 - 4)
    y0 = Math.min(y0, point.y - box.h / 2 - 4)
    x1 = Math.max(x1, point.x + box.w / 2 + 4)
    y1 = Math.max(y1, point.y + box.h / 2 + 4)
  }
  for (const curve of curves) {
    const tx = (curve.ax - curve.qx) / (curve.ax - 2 * curve.qx + curve.bx)
    const ty = (curve.ay - curve.qy) / (curve.ay - 2 * curve.qy + curve.by)
    const px = tx > 0 && tx < 1 ? pointOnCurve(curve, tx).x : curve.ax
    const py = ty > 0 && ty < 1 ? pointOnCurve(curve, ty).y : curve.ay
    x0 = Math.min(x0, curve.ax, curve.bx, px)
    y0 = Math.min(y0, curve.ay, curve.by, py)
    x1 = Math.max(x1, curve.ax, curve.bx, px)
    y1 = Math.max(y1, curve.ay, curve.by, py)
  }
  if (x0 < VIEW.x) x0 -= 48
  if (y0 < VIEW.y) y0 -= 48
  if (x1 > VIEW.w) x1 += 48
  if (y1 > VIEW.h) y1 += 48
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0, cx: (x0 + x1) / 2, cy: (y0 + y1) / 2 }
}
export type Curve = { ax: number; ay: number; qx: number; qy: number; bx: number; by: number; d: string }

/** Điểm cắt mép thẻ theo hướng tới điểm điều khiển (artboard `hit`). */
const hit = (pos: Point, box: Box, qx: number, qy: number): [number, number] => {
  const dx = qx - pos.x
  const dy = qy - pos.y
  const hw = box.w / 2 + 5
  const hh = box.h / 2 + 5
  const t = Math.min(hw / Math.max(Math.abs(dx), 1e-3), hh / Math.max(Math.abs(dy), 1e-3))
  return [pos.x + dx * t, pos.y + dy * t]
}

/** Cung nối hai mép thẻ, cong ra ngoài tâm 16% chiều dài. */
export const edgeCurve = (a: Point, b: Point, aBox: Box, bBox: Box, index: number): Curve => {
  const mx = (a.x + b.x) / 2
  const my = (a.y + b.y) / 2
  const dx = b.x - a.x
  const dy = b.y - a.y
  const len = Math.max(1e-3, Math.hypot(dx, dy))
  const nx = -dy / len
  const ny = dx / len
  const side = (mx - VIEW.cx) * nx + (my - VIEW.cy) * ny
  const sign = Math.abs(side) < 10 ? (index % 2 ? 1 : -1) : Math.sign(side)
  const bend = len * 0.16 * sign
  const qx = mx + nx * bend
  const qy = my + ny * bend
  const [ax, ay] = hit(a, aBox, qx, qy)
  const [bx, by] = hit(b, bBox, qx, qy)
  return { ax, ay, qx, qy, bx, by, d: `M${ax.toFixed(1)} ${ay.toFixed(1)} Q${qx.toFixed(1)} ${qy.toFixed(1)} ${bx.toFixed(1)} ${by.toFixed(1)}` }
}

export const pointOnCurve = (curve: Pick<Curve, 'ax' | 'ay' | 'qx' | 'qy' | 'bx' | 'by'>, t: number): Point => {
  const u = 1 - t
  return {
    x: u * u * curve.ax + 2 * u * t * curve.qx + t * t * curve.bx,
    y: u * u * curve.ay + 2 * u * t * curve.qy + t * t * curve.by,
  }
}

export type Rect = [number, number, number, number]

export const overlap = (a: Rect, b: Rect) =>
  Math.max(0, Math.min(a[2], b[2]) - Math.max(a[0], b[0])) * Math.max(0, Math.min(a[3], b[3]) - Math.max(a[1], b[1]))

/** Bề rộng nhãn pill: 10 + chấm 5 + 7 + chữ + 11 (+ “MỚI” khi cạnh đổi ở chương này). */
export const labelWidth = (textWidth: number, fresh: boolean, newWidth = 22) => 10 + 5 + 7 + textWidth + 11 + (fresh ? newWidth + 7 : 0)


/** Khung thẻ (đã nới 4 px) để tính chồng lấn của nhãn. */
export const nodeRect = (point: Point, box: Box): Rect => [
  point.x - box.w / 2 - 4, point.y - box.h / 2 - 4, point.x + box.w / 2 + 4, point.y + box.h / 2 + 4,
]

// ─── Cắt chữ bằng font thật ──────────────────────────────────────────────

/**
 * Cắt “…” theo font thật, tìm nhị phân để không đo lại từng ký tự khi có nhiều nhãn dài.
 * Bỏ dấu cách/·/, ở cuối; tên đầy đủ nằm trong `<title>`.
 */
export const fitText = (full: string, max: number, measure: (text: string) => number): string => {
  if (measure(full) <= max) return full
  let lo = 0
  let hi = Math.max(0, full.length - 1)
  let fitted = ''
  while (lo <= hi) {
    const mid = Math.floor((lo + hi) / 2)
    const text = `${full.slice(0, mid).replace(/[\s·,]+$/, '')}…`
    if (measure(text) <= max) {
      fitted = text
      lo = mid + 1
    } else {
      hi = mid - 1
    }
  }
  return fitted
}

// ─── Khoá sửa (D8) ───────────────────────────────────────────────────────

/** Banner khoá phân biệt viết lần đầu và viết lại theo `writing.state` (Câu 23). */
export const lockSentence = (writing: NonNullable<AddressTerms['writing']>) => {
  const agent = t.brain.agents[writing.role] ?? t.brain.agents.writer
  const verb = writing.state === 'writing' ? 'đang viết' : 'đang viết lại'
  const doing = writing.chapter > 0 ? `${verb} chương ${writing.chapter}` : 'đang viết'
  return `${agent} ${doing}, nên đang khoá sửa.`
}
