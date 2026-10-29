// Logic thuần của tab Nhân vật (Outline.dc.html › person · xedit): bảng xưng hô, ô “Xuất hiện”,
// hộp thoại xác nhận khi “từ chương” chạm chương đã viết. Không JSX → test được bằng vitest.
import type { AddressApply, AddressTerm, Brain, RelationChange } from '../../../api/types'

/** Dòng đã hết hiệu lực: `until_chapter` > 0 (BE: 0 = còn hiệu lực) và đã nằm trong phần đã viết. */
export const isExpired = (term: Pick<AddressTerm, 'untilChapter'>, latestWritten: number) =>
  term.untilChapter !== null && term.untilChapter > 0 && term.untilChapter <= latestWritten

/** Các dòng của một nhân vật — một dòng là một chiều, nên tính cả hai chiều. */
export const termsFor = (terms: AddressTerm[], name: string) =>
  terms.filter((term) => term.from === name || term.to === name)

/** “Xưng hô 5 dòng · xuất hiện 8/8 chương”; nhân vật chưa có dòng nào thì bỏ vế đầu (artboard: Đỗ Mẫn). */
export const castMeta = (lines: number, appear: number, total: number) =>
  `${lines > 0 ? `Xưng hô ${lines} dòng · ` : ''}xuất hiện ${appear}/${total} chương`

/** Cột “Áp dụng”: “từ ch.2” khi còn hiệu lực (`until_chapter` 0 = chưa đóng), “ch.1” / “ch.1–3” khi đã đóng. */
export const appliedLabel = (term: Pick<AddressTerm, 'fromChapter' | 'untilChapter'>) =>
  term.untilChapter === null || term.untilChapter <= 0
    ? `từ ch.${term.fromChapter}`
    : term.untilChapter === term.fromChapter
      ? `ch.${term.fromChapter}`
      : `ch.${term.fromChapter}–${term.untilChapter}`

/** Chương đã viết bị ảnh hưởng: từ `fromChapter` tới chương mới nhất đã viết (rỗng nếu chưa chạm). */
export const affectedChapters = (fromChapter: number, latestWritten: number): number[] => {
  if (!Number.isInteger(fromChapter) || fromChapter < 1 || fromChapter > latestWritten) return []
  return Array.from({ length: latestWritten - fromChapter + 1 }, (_, index) => fromChapter + index)
}

/** “2–3” cho dải liên tiếp, “2, 4” khi đứt quãng, “2” khi một chương. */
export const chapterRange = (chapters: number[]): string => {
  if (chapters.length === 0) return ''
  if (chapters.length === 1) return String(chapters[0])
  const consecutive = chapters.every((n, index) => index === 0 || n === chapters[index - 1] + 1)
  return consecutive ? `${chapters[0]}–${chapters[chapters.length - 1]}` : chapters.join(', ')
}

const WORDS: Record<number, string> = { 1: 'Một', 2: 'Hai', 3: 'Ba', 4: 'Bốn', 5: 'Năm' }

/** “Chương 2 và 3” · “Chương 2, 3 và 4”. */
const chapterSentence = (chapters: number[]): string => {
  if (chapters.length <= 1) return `Chương ${chapters[0] ?? ''}`
  const head = chapters.slice(0, -1).join(', ')
  return `Chương ${head} và ${chapters[chapters.length - 1]}`
}

/**
 * Ba thao tác dùng chung một hộp xác nhận (Câu 21): `edit` sửa dòng đang có · `add` thêm dòng ·
 * `rel` ghi thay đổi quan hệ. Câu chữ lấy nguyên từ Outline.dc.html › `xcT`/`xcD`/`xopts`.
 */
export type ConfirmInput =
  | {
    mode: 'edit'; affected: number[]; latestWritten: number
    fromName: string; toName: string; call: string; fromChapter: number; oldCall: string
  }
  | {
    mode: 'add'; affected: number[]; latestWritten: number
    fromName: string; toName: string; call: string; fromChapter: number
  }
  | {
    mode: 'rel'; chapter: number; latestWritten: number
    other: string; before: string; after: string
  }

export const confirmTitle = (input: ConfirmInput): string => {
  if (input.mode === 'edit') return `Sửa này chạm tới ${input.affected.length} chương đã viết`
  if (input.mode === 'add') return 'Dòng mới chạm tới chương đã viết'
  return `Thay đổi này nằm ở chương ${input.chapter} đã viết`
}

export const confirmBody = (input: ConfirmInput): string => {
  if (input.mode === 'rel') {
    const head = `Quan hệ với ${input.other} đổi thành “${input.after}” từ chương ${input.chapter}.`
    return input.before
      ? `${head} Chương ${input.chapter} đã viết theo quan hệ cũ “${input.before}”.`
      : head
  }
  const call = `${input.fromName} gọi ${input.toName} là “${input.call}” từ chương ${input.fromChapter}.`
  if (input.mode === 'add') {
    return `${call} Chương ${input.fromChapter} đã viết khi chưa có dòng này, có thể đang dùng cách gọi khác.`
  }
  return `${call} ${chapterSentence(input.affected)} đã viết theo cách gọi cũ “${input.oldCall}”.`
}

export type ApplyOption = { value: AddressApply; label: string; description: string }

/** Hai lựa chọn của hộp thoại xác nhận; “đánh dấu cần sửa” là mặc định (`mark`). */
export const applyOptions = (input: ConfirmInput): ApplyOption[] => {
  const next = input.latestWritten + 1
  if (input.mode === 'rel') {
    return [
      {
        value: 'next',
        label: `Chỉ ghi nhận, chương ${input.chapter} giữ nguyên`,
        description: `Não truyện và Biên tập viên dùng quan hệ mới từ chương ${input.chapter}. Chương đã viết không bị đánh dấu.`,
      },
      {
        value: 'mark',
        label: `Ghi nhận và đánh dấu chương ${input.chapter} cần sửa`,
        description: `Chương ${input.chapter} vào hàng chờ sửa. Người viết chỉ sửa đoạn có quan hệ này.`,
      },
    ]
  }
  const range = chapterRange(input.affected)
  const tail = 'vào hàng chờ sửa. Người viết chỉ sửa câu có cách gọi này, không viết lại cả chương.'
  return [
    {
      value: 'next',
      label: 'Chỉ áp dụng cho chương chưa viết',
      description: `Chương ${input.mode === 'add' ? input.fromChapter : range} giữ nguyên. Từ chương ${next} trở đi, Người viết dùng cách gọi mới.`,
    },
    {
      value: 'mark',
      label: `Áp dụng và đánh dấu chương ${range} cần sửa`,
      // Câu của artboard: thêm dòng chỉ đếm “Chương vào hàng chờ sửa”, sửa dòng đếm bằng chữ (“Hai chương”).
      description: input.mode === 'add'
        ? `Chương ${tail}`
        : `${WORDS[input.affected.length] ?? String(input.affected.length)} chương ${tail}`,
    },
  ]
}

/** Toast sau khi lưu trong hộp thoại (khớp câu của artboard). */
export const applyToast = (input: ConfirmInput, apply: AddressApply): string => {
  const next = input.latestWritten + 1
  if (input.mode === 'rel') {
    return apply === 'mark' ? `Đã ghi · chương ${input.chapter} vào hàng chờ sửa` : 'Đã ghi thay đổi quan hệ'
  }
  if (apply === 'next') {
    return input.mode === 'add' ? `Đã thêm · áp dụng từ chương ${next}` : `Đã lưu · áp dụng từ chương ${next}`
  }
  const range = chapterRange(input.affected)
  return input.mode === 'add' ? `Đã thêm · chương ${range} vào hàng chờ sửa` : `Đã lưu · chương ${range} vào hàng chờ sửa`
}

/** Dòng ghi chú dưới form thêm dòng; bỏ vế “từ chương đã viết” khi chưa viết chương nào. */
export const addFormNote = (latestWritten: number) =>
  'Mỗi dòng một chiều. Nếu chiều này đã có dòng đang áp dụng, dòng cũ tự kết thúc ở chương liền trước.'
  + (latestWritten > 0 ? ` Từ chương đã viết (1–${latestWritten}) sẽ hỏi cách áp dụng.` : '')

/** Kiểm tra tại chỗ trước khi gọi máy chủ; trả câu lỗi tiếng Việt hoặc null. */
export const validateTerm = (input: { from: string; to: string; call: string; self: string; fromChapter: number; reason: string }): string | null => {
  if (!input.from || !input.to) return 'Chọn đủ hai nhân vật của dòng xưng hô.'
  if (input.from === input.to) return 'Một dòng xưng hô cần hai nhân vật khác nhau.'
  if (!input.call.trim()) return 'Chưa nhập cách gọi.'
  if (!input.self.trim()) return 'Chưa nhập cách tự xưng.'
  if (!Number.isInteger(input.fromChapter) || input.fromChapter < 1) return 'Chương bắt đầu phải từ 1 trở lên.'
  // Câu 20: sửa trên web thì lý do bắt buộc — máy chủ trả 400 `{field:"reason"}` nếu thiếu.
  if (!input.reason.trim()) return 'Chưa nhập lý do cho dòng xưng hô.'
  return null
}

export const validateRelation = (input: { a: string; b: string; after: string; fromChapter: number; reason: string }): string | null => {
  if (!input.a || !input.b) return 'Chọn đủ hai nhân vật của quan hệ.'
  if (input.a === input.b) return 'Quan hệ cần hai nhân vật khác nhau.'
  if (!input.after.trim()) return 'Chưa nhập quan hệ sau khi đổi.'
  if (!Number.isInteger(input.fromChapter) || input.fromChapter < 1) return 'Chương bắt đầu phải từ 1 trở lên.'
  if (!input.reason.trim()) return 'Chưa nhập lý do cho thay đổi quan hệ.'
  return null
}

/**
 * Ba nhóm màu chip quan hệ của Câu 20: đối đầu (enemy) · thân (ally) · còn lại và `rel`.
 * Chỉ trả khoá nhóm; màu nằm ở token (`--tint-err`/`--tint-ok`/`--frame-info-12`).
 */
export type RelationTone = 'hostile' | 'close' | 'other'

export const relationTone = (type: string | undefined): RelationTone =>
  type === 'enemy' ? 'hostile' : type === 'ally' ? 'close' : 'other'

/** Danh sách “quan hệ mới” của form ghi thay đổi (Outline.dc.html › xedit=rel) + “Khác…”. */
export const RELATION_OPTIONS = ['Đối đầu công khai', 'Đồng minh', 'Tin tưởng một phần', 'Người thân'] as const
export const RELATION_OTHER = 'Khác…'

export type AppearCell = { n: number; present: boolean; title: string; bg: string; fg: string; border: string }

/**
 * Ô 1..N của hàng “Xuất hiện”: nền theo trạng thái chương đã viết (xanh) / đang viết (tím),
 * chữ sáng khi nhân vật có mặt ở chương đã viết hoặc đang viết, chương đang viết viền tím.
 */
export const appearCells = (brain: Pick<Brain, 'chapters'>, appears: number[], writingChapter: number | null): AppearCell[] =>
  brain.chapters.map((chapter) => {
    const present = appears.includes(chapter.n)
    const written = chapter.status === 'written'
    const writing = chapter.status === 'rewriting' || chapter.status === 'writing' || chapter.n === writingChapter
    return {
      n: chapter.n,
      present,
      title: chapter.title,
      bg: present ? (written ? 'var(--kit-ok-8)' : writing ? 'var(--tint-accent)' : 'transparent') : 'transparent',
      fg: present && (written || writing) ? 'var(--color-fg)' : 'var(--kit-fg-icon)',
      border: writing ? 'var(--kit-acc-45)' : 'var(--line-3)',
    }
  })

/** Dòng lịch sử quan hệ của một nhân vật, kèm tên người còn lại. */
export const relationsFor = (relations: RelationChange[], name: string) =>
  relations
    .filter((relation) => relation.a === name || relation.b === name)
    .map((relation) => ({
      ...relation,
      other: relation.a === name ? relation.bName : relation.aName,
    }))

/** Nhân vật khác trong truyện (cho ô chọn của dòng mới). */
export const otherNames = (names: string[], name: string) => names.filter((candidate) => candidate !== name)

/**
 * Quan hệ đang áp dụng ngay trước chương `chapter` cho cặp hai nhân vật — chip “Quan hệ đang áp dụng
 * ở chương liền trước” của form ghi thay đổi (Outline.dc.html › xedit=rel).
 */
export const relationBefore = (relations: RelationChange[], a: string, b: string, chapter: number) =>
  relations
    .filter((item) => item.chapter < chapter && ((item.a === a && item.b === b) || (item.a === b && item.b === a)))
    .at(-1) ?? null

/** Vai của BE (`writing.role`) → tên người đang giữ khoá sửa (D8). */
const WRITING_AGENT: Record<string, string> = {
  writer: 'Người viết',
  editor: 'Biên tập viên',
  architect: 'Kiến trúc sư',
}

/**
 * Banner tím của D8 — câu của artboard, đổi người giữ khoá theo `writing.role` và đọc đúng
 * “đang viết” / “đang viết lại” theo `writing.state` (Câu 23).
 */
export const lockMessage = (writing: { chapter: number; role: string; state?: 'writing' | 'rewriting' }) => {
  const agent = WRITING_AGENT[writing.role] ?? 'Người viết'
  const verb = writing.state === 'writing' ? 'đang viết' : 'đang viết lại'
  const doing = writing.chapter > 0 ? `${verb} chương ${writing.chapter}` : 'đang viết'
  return `${agent} ${doing}, nên xưng hô và quan hệ đang khoá để không lệch với chương đang viết.`
}
