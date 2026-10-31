import { hookTypeLabel } from './labels'
import { copy as readerCopy } from '../screens/reader/copy'
import { WORLD_CATEGORIES } from '../screens/project/tabs/foundation'

export type RunCardTone = 'b' | 'w' | 'r' | 'g'
export type RunCardBlock =
  | { kind: 'kv'; items: [label: string, value: string, tone?: RunCardTone][] }
  | { kind: 'list'; label: string; items: string[] }
  | { kind: 'text'; label: string; text: string }
  | { kind: 'quote'; text: string }
  | { kind: 'diff'; old: string; next: string }
  | { kind: 'issues'; items: { level: 'i' | 'w' | 'e'; ref: string; text: string; fix: string }[] }
  | { kind: 'cast'; names: string[] }
  | { kind: 'tags'; label: string; items: [action: string, text: string, tone?: RunCardTone][] }
  | { kind: 'scores'; items: [label: string, score: string][] }
  | { kind: 'ol'; items: string[] }
  | { kind: 'note'; text: string }
  | { kind: 'link'; text: string; href: 'reader' | 'premise'; chapter?: number }
export type RunCardModel = { name: string; role: string; summary: string; blocks: RunCardBlock[] }

type Data = Record<string, unknown>
const object = (value: unknown): Data => value && typeof value === 'object' && !Array.isArray(value) ? value as Data : {}
function parse(raw: string): Data {
  try { return object(JSON.parse(raw)) } catch { return {} }
}
const text = (value: unknown): string => typeof value === 'string' || typeof value === 'number' ? String(value) : ''
const list = (value: unknown): unknown[] => Array.isArray(value) ? value : []
const join = (...values: (string | false | undefined)[]) => values.filter(Boolean).join(' · ')
const numberText = (value: unknown): string => {
  const n = Number(value)
  return value !== undefined && value !== null && value !== '' && Number.isFinite(n) ? n.toLocaleString('vi-VN') : ''
}
const label = (value: unknown): string => {
  if (typeof value === 'string' || typeof value === 'number') return String(value)
  const o = object(value)
  return text(o.name ?? o.title ?? o.text ?? o.description ?? o.rule ?? o.summary)
}
const levelOf = (value: unknown): 'i' | 'w' | 'e' => {
  const t = text(value).toLowerCase()
  return t === 'e' || t === 'error' || t === 'err' || t === 'high' ? 'e' : t === 'w' || t === 'warn' || t === 'warning' ? 'w' : 'i'
}
const words = (value: unknown): string => { const t = numberText(value); return t ? `${t} chữ` : '' }

/** Mô hình thẻ luồng viết theo §10 — khối thân giống khung board, không lộ tên thao tác/tham số thô. */
export function runCardModel(key: string, params: string, result: string): RunCardModel {
  const p = parse(params), r = parse(result), content = object(p.content)
  const contract = object(p.contract)
  const get = (field: string): unknown => p[field] ?? content[field] ?? contract[field] ?? r[field]
  const s = (field: string) => { const v = text(get(field)); return v === 'null' || v === '0' ? '' : v }
  const arr = (field: string) => list(get(field))
  const labels = (field: string) => arr(field).map(label).filter(Boolean)
  const chapter = s('chapter') ? `chương ${s('chapter')}` : ''
  const blocks: RunCardBlock[] = []
  const add = (...bs: RunCardBlock[]) => blocks.push(...bs)
  const kv = (...items: ([label: string, value: string, tone?: RunCardTone] | string | false | undefined)[]) => {
    const real = items.filter((i): i is [string, string, RunCardTone?] => Array.isArray(i) && i[1] !== '')
    if (real.length) add({ kind: 'kv', items: real })
  }
  const textBlock = (lbl: string, value: unknown) => { const t = label(value); if (t) add({ kind: 'text', label: lbl, text: t }) }
  const listBlock = (lbl: string, values: string[]) => { if (values.length) add({ kind: 'list', label: lbl, items: values }) }
  let name = 'Thao tác khác', role = '', summary = 'loại thao tác mới · xem chi tiết kỹ thuật'
  switch (key) {
    case 'novel_context': {
      name = 'Đọc bối cảnh'; role = 'Người viết'
      const parts = [r.premise && 'tiền đề', r.outline && 'dàn ý', Array.isArray(r.characters) && `${r.characters.length} nhân vật`, Array.isArray(r.foreshadow) && `${r.foreshadow.length} cài cắm`].filter((part): part is string => typeof part === 'string' && part.length > 0)
      summary = join(chapter, parts.join(', '))
      kv(['Chương', s('chapter')])
      if (parts.length) add({ kind: 'list', label: 'Đã nạp cho Người viết', items: parts.map(x => x === 'tiền đề' ? 'tiền đề và khung truyện' : x === 'dàn ý' ? `dàn ý ${chapter || 'chương'}` : x) })
      break
    }
    case 'read_chapter': {
      name = 'Đọc chương'; role = 'Người viết'
      const edition = s('source') === 'draft' ? 'bản nháp' : 'bản cuối'
      const range = s('from') && s('to') ? `đoạn ${s('from')}–${s('to')}` : ''
      summary = join(chapter, edition, range, s('character') && `lời thoại của ${s('character')}`)
      kv(['Chương', s('chapter')], ['Bản', edition], range && ['Đoạn', range.replace('đoạn ', '')], s('character') && ['Chỉ đọc', `lời thoại của ${s('character')}`], words(r.word_count ?? r.words) && ['Độ dài', words(r.word_count ?? r.words)])
      const quote = label(r.content ?? r.text); if (quote) add({ kind: 'quote', text: quote })
      if (Number(s('chapter')) > 0) add({ kind: 'link', text: `Mở chương ${s('chapter')} trong Trình đọc ›`, href: 'reader', chapter: Number(s('chapter')) })
      break
    }
    case 'plan_chapter': {
      name = 'Lập kế hoạch chương'; role = 'Người viết'; summary = join(chapter, s('title'))
      kv(['Mục tiêu', s('goal')], ['Móc cuối', s('hook')], ['Cảm xúc', s('emotion')])
      textBlock('Xung đột', get('conflict')); listBlock('Bắt buộc có', labels('required_beats')); listBlock('Tránh', labels('forbidden_moves')); listBlock('Kiểm liên tục', labels('continuity_checks')); break
    }
    case 'draft_chapter': {
      name = 'Viết bản nháp'; role = 'Người viết'
      const mode = s('mode') === 'append' ? 'viết tiếp' : 'viết mới'
      summary = join(chapter, mode, words(r.words ?? get('word_count')))
      kv(['Chương', s('chapter')], ['Cách ghi', s('mode') === 'append' ? 'viết tiếp' : 'viết mới, thay bản cũ'], words(r.words ?? get('word_count')) && ['Số chữ', numberText(r.words ?? get('word_count'))])
      const quote = label(get('content')); if (quote && quote !== '…') add({ kind: 'quote', text: quote })
      if (Number(s('chapter')) > 0) add({ kind: 'link', text: `Mở bản nháp chương ${s('chapter')} ›`, href: 'reader', chapter: Number(s('chapter')) })
      break
    }
    case 'edit_chapter': {
      name = 'Gọt giũa'; role = 'Người viết'; summary = join(chapter, get('replace_all') === true ? 'thay mọi chỗ' : 'sửa 1 đoạn')
      add({ kind: 'diff', old: label(get('old_string')), next: label(get('new_string')) })
      kv(['Phạm vi', get('replace_all') === true ? 'mọi chỗ' : 'một chỗ']); break
    }
    case 'check_consistency': {
      name = 'Soát nhất quán'; role = 'Người viết'
      const sources = [['world_rules', 'quy tắc thế giới'], ['foreshadow', 'chi tiết cài cắm'], ['relations', 'quan hệ nhân vật'], ['aliases', 'biệt danh và cách gọi'], ['summaries', 'tóm tắt']].filter(([f]) => Array.isArray(r[f]))
      summary = join(chapter, `nạp ${sources.length} nguồn đối chiếu`)
      add({ kind: 'list', label: 'Đã nạp để đối chiếu', items: sources.map(([f, l]) => `${list(r[f]).length} ${l}`) })
      add({ kind: 'note', text: 'Người viết tự đối chiếu bản nháp; chỗ lệch (nếu có) thành thẻ Gọt giũa ngay sau.' }); break
    }
    case 'commit_chapter': {
      name = 'Lưu chương'; role = 'Người viết'
      const fs = arr('foreshadow_updates').length ? arr('foreshadow_updates') : list(r.foreshadow)
      const addr = arr('address_terms')
      summary = join(chapter, words(r.words ?? get('word_count')), fs.length > 0 && `${fs.length} cài cắm`, addr.length > 0 && `${addr.length} xưng hô`)
      kv(['Chương', s('chapter')], ['Số chữ', numberText(r.words ?? get('word_count'))], s('hook_type') && ['Kiểu móc', hookTypeLabel(s('hook_type')), 'b'], arr('fixed_comments').length > 0 && ['Góp ý đã sửa', text(arr('fixed_comments').length)])
      const cast = labels('characters'); if (cast.length) add({ kind: 'cast', names: cast })
      const actions: Record<string, [string, RunCardTone]> = { plant: ['Đã gieo', 'b'], advance: ['Đẩy tiếp', 'w'], resolve: ['Đã thu', 'g'], abandon: ['Đã bỏ', 'r'] }
      const tags: [string, string, RunCardTone?][] = fs.flatMap(item => {
        const o = object(item); const action = actions[text(o.action)]
        const description = label(o.description ?? o.text ?? item)
        return action && description ? [[action[0], description, action[1]]] : []
      })
      if (tags.length) add({ kind: 'tags', label: 'Chi tiết cài cắm', items: tags })
      const terms = addr.map(item => { const o = object(item); const pair = label(o.pair) || [label(o.from), label(o.to)].filter(Boolean).join(' → '); const term = label(o.calls ?? o.term); const from = text(o.from_chapter); return pair && term ? `${pair}: “${term}”${from ? ` từ chương ${from}` : ''}` : '' }).filter(Boolean)
      if (terms.length) add({ kind: 'kv', items: terms.map(t => ['Xưng hô mới', t] as [string, string]) })
      textBlock('Sự kiện chính', get('events')); break
    }
    case 'audit_foundation': {
      name = 'Soát nền truyện'; role = 'Kiến trúc sư'; summary = join(get('ready') === true ? 'sẵn sàng' : 'chưa sẵn sàng', `${arr('issues').length} lưu ý`)
      kv(['Kết luận', get('ready') === true ? 'sẵn sàng viết' : 'chưa sẵn sàng', get('ready') === true ? 'g' : 'r'])
      textBlock('Tóm tắt', get('summary'))
      const issues = arr('issues').map(item => { const o = object(item); return { level: levelOf(o.severity ?? o.level), ref: label(o.ref ?? o.rule ?? o.scope) || 'Mục luật', text: label(o.text ?? o.description ?? item), fix: label(o.fix ?? o.suggestion) } }).filter(i => i.text)
      if (issues.length) add({ kind: 'issues', items: issues }); break
    }
    case 'revise_outline': {
      name = 'Sửa dàn ý'; role = 'Kiến trúc sư'; summary = join(s('from_chapter') && `từ chương ${s('from_chapter')}`, `đổi ${arr('chapters').length || list(r.changed).length} chương`)
      textBlock('Lý do', get('reason'))
      const changed = (arr('chapters').length ? arr('chapters') : list(r.changed))
      const diffs = changed.map(item => { const o = object(item); const from = label(o.before ?? o.old ?? o.from); const to = label(o.after ?? o.next ?? o.to); return from && to ? { kind: 'diff' as const, old: from, next: to } : null }).filter(Boolean)
      if (diffs.length) add(...(diffs as RunCardBlock[]))
      else kv(['Các chương', labels('chapters').join(', ') || text(list(r.changed).map(label).join(', '))])
      break
    }
    case 'resolve_outline_feedback': {
      name = 'Giữ dàn ý'; role = 'Kiến trúc sư'; summary = 'không cần đổi'; textBlock('Lý do giữ', get('reason')); break
    }
    case 'save_book': {
      name = 'Lưu thông tin truyện'; role = 'Kiến trúc sư'; summary = 'tên + giới thiệu'
      kv(['Tên', s('title')]); textBlock('Giới thiệu', get('synopsis')); break
    }
    case 'save_foundation': {
      role = 'Kiến trúc sư'
      const kind = s('type'), contents = p.content
      if (kind === 'premise') {
        name = 'Lưu tiền đề'
        const axes = Array.isArray(get('axes')) ? arr('axes') : Object.keys(object(get('axes')))
        summary = join(`${axes.length} mục`, s('nguoi_nghe') && `người nghe ${s('nguoi_nghe')}`)
        listBlock('Các mục', axes.map((a, i) => { const t = label(a); return i < 5 || axes.length <= 6 ? t : '' }).filter((t, i) => t && i < 6).concat(axes.length > 6 ? [`… và ${axes.length - 5} mục khác`] : []))
        kv(['Người nghe', s('nguoi_nghe')], ['Điểm khác', s('diem_khac')])
        add({ kind: 'link', text: 'Xem tiền đề ở tab Tiền đề ›', href: 'premise' })
      } else if (kind === 'characters') {
        name = 'Lưu nhân vật'; const chars = list(contents)
        summary = join(`${chars.length} nhân vật`, `${chars.filter(i => object(i).kind === 'lead' || object(i).tier === 'core' || object(i).role === 'main').length} chính`)
        const names = chars.map(label).filter(Boolean); if (names.length) add({ kind: 'cast', names })
        const late = chars.filter(i => object(i).revealAt).map(i => `${label(i)} · chương ${text(object(i).revealAt)}`)
        if (late.length) kv(['Lộ vai muộn', late.join('; ')])
      } else if (kind === 'world_rules') {
        name = 'Lưu quy tắc thế giới'; const rules = list(contents)
        const categoryLabel = (value: unknown) => WORLD_CATEGORIES.find(category => category.category === text(value))?.label
        const categories = [...new Set(rules.map(rule => categoryLabel(object(rule).category)).filter((category): category is NonNullable<typeof category> => category !== undefined))]
        summary = join(`${rules.length} quy tắc`, categories.map(category => category.toLocaleLowerCase('vi')).join(', '))
        const examples = rules.slice(0, 3).map(rule => { const o = object(rule); const category = categoryLabel(o.category); const description = label(o.rule ?? rule); return category && description ? `${category}: ${description}` : description }).filter(Boolean)
        listBlock('Quy tắc', examples.concat(rules.length > 3 ? [`… và ${rules.length - 3} quy tắc khác`] : []))
      } else if (['outline', 'layered_outline', 'expand_arc', 'append_volume'].includes(kind)) {
        name = 'Lưu dàn ý'
        const chapters = arr('chapters').length ? arr('chapters') : list(contents)
        summary = join(`${chapters.length} chương`, arr('volumes').length > 0 && `${arr('volumes').length} tập`)
        const titles = chapters.map(item => { const o = object(item); return label(o.title ?? o.name ?? item) }).filter(Boolean)
        if (titles.length) add({ kind: 'ol', items: titles })
      } else if (kind === 'update_compass') {
        name = 'Cập nhật hướng đi'; summary = `${arr('open_threads').length} tuyến còn mở`
        textBlock('Hướng kết', get('ending_direction')); listBlock('Tuyến còn mở', labels('open_threads'))
      } else if (kind === 'complete_book') {
        name = 'Hoàn tất truyện'; summary = join(`${arr('chapters').length} chương`, 'đã khép mọi tuyến')
        textBlock('Lý do', get('reason'))
      } else { role = ''; add({ kind: 'note', text: 'Noverse chưa có thẻ riêng cho thao tác này. Tham số và kết quả nằm ở Chi tiết kỹ thuật.' }) }
      break
    }
    case 'save_review': {
      name = 'Lưu thẩm duyệt'; role = 'Biên tập viên'
      const verdict: Record<string, [string, RunCardTone]> = { accept: ['Đạt', 'g'], polish: ['Gọt thêm', 'w'], rewrite: ['Viết lại', 'r'] }
      const [vt, tone] = verdict[s('verdict')] ?? ['', 'w']
      summary = join(chapter, vt, `${arr('issues').length} cảnh báo`)
      kv(['Kết luận', vt, tone], ['Chương', s('chapter')], s('pack') && ['Chấm theo', `gói ${s('pack')}`])
      const scores: [string, string][] = arr('dimensions').map(item => {
        const o = object(item); const dimension = label(o.name ?? o.dimension); return [readerCopy.dimension[dimension] ?? dimension, text(o.score ?? o.value)] as [string, string]
      }).filter(([l, v]) => l && v)
      if (scores.length) add({ kind: 'scores', items: scores })
      const issues = arr('issues').map(item => { const o = object(item); return { level: levelOf(o.severity ?? o.level), ref: label(o.ref ?? o.rule ?? o.source) || 'Mục luật', text: label(o.text ?? o.description ?? item), fix: label(o.fix ?? o.suggestion) } }).filter(i => i.text)
      if (issues.length) add({ kind: 'issues', items: issues }); break
    }
    case 'save_arc_summary': {
      name = 'Tóm tắt cung'; role = 'Biên tập viên'; summary = join(s('volume') && `tập ${s('volume')}`, s('arc') && `cung ${s('arc')}`, `${arr('key_events').length} sự kiện`)
      textBlock('Tóm tắt', get('summary')); listBlock('Sự kiện chính', labels('key_events'))
      const snaps = arr('character_snapshots').map(item => { const o = object(item); return [label(o.name ?? o.character ?? item), label(o.status ?? o.state ?? o.note)] as [string, string] }).filter(([n]) => n)
      if (snaps.length) add({ kind: 'kv', items: snaps })
      break
    }
    case 'save_volume_summary': {
      name = 'Tóm tắt tập'; role = 'Biên tập viên'; summary = join(s('volume') && `tập ${s('volume')}`, `${arr('chapters').length} chương`)
      textBlock('Tóm tắt', get('summary')); break
    }
    default:
      add({ kind: 'note', text: 'Noverse chưa có thẻ riêng cho thao tác này. Tham số và kết quả nằm ở Chi tiết kỹ thuật.' })
  }
  return { name, role, summary, blocks }
}
