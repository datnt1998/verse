export type RunCardBlock = { label: string; value: string }
export type RunCardModel = { name: string; role: string; summary: string; blocks: RunCardBlock[] }

type Data = Record<string, unknown>
const object = (value: unknown): Data => value && typeof value === 'object' && !Array.isArray(value) ? value as Data : {}
function parse(raw: string): Data {
  try { return object(JSON.parse(raw)) } catch { return {} }
}
const text = (value: unknown): string => typeof value === 'string' || typeof value === 'number' ? String(value) : ''
const list = (value: unknown): unknown[] => Array.isArray(value) ? value : []
const numberText = (value: unknown): string => {
  const n = Number(value)
  return value !== undefined && value !== null && value !== '' && Number.isFinite(n) ? n.toLocaleString('vi-VN') : ''
}
const join = (...values: (string | false | undefined)[]) => values.filter(Boolean).join(' · ')
const describe = (value: unknown): string => {
  if (Array.isArray(value)) return value.map(describe).filter(Boolean).join('; ')
  if (value && typeof value === 'object') return Object.values(value).map(describe).filter(Boolean).join(' · ')
  return text(value)
}

/** Dữ liệu hiển thị không chứa tên thao tác hay tên trường kỹ thuật. */
export function runCardModel(key: string, params: string, result: string): RunCardModel {
  const p = parse(params), r = parse(result), content = object(p.content)
  const get = (field: string): unknown => p[field] ?? content[field] ?? r[field]
  const s = (field: string) => { const value = text(get(field)); return value === 'null' || value === '0' ? '' : value }
  const arr = (field: string) => list(get(field))
  const chapter = s('chapter') ? `chương ${s('chapter')}` : ''
  const blocks: RunCardBlock[] = []
  const add = (label: string, value: unknown) => { const v = describe(value); if (v) blocks.push({ label, value: v }) }
  let name = 'Thao tác khác', role = '', summary = 'loại thao tác mới · xem chi tiết kỹ thuật'
  switch (key) {
    case 'novel_context': {
      name = 'Đọc bối cảnh'; role = 'Người viết'
      const loaded = [r.premise && 'tiền đề', r.outline && 'dàn ý', Array.isArray(r.characters) && `${r.characters.length} nhân vật`, Array.isArray(r.foreshadow) && `${r.foreshadow.length} cài cắm`].filter(Boolean).join(', ')
      summary = join(chapter, loaded); add('Chương', s('chapter')); add('Đã nạp cho Người viết', loaded); break
    }
    case 'read_chapter':
      name = 'Đọc chương'; summary = join(chapter, s('source') === 'draft' ? 'bản nháp' : 'bản cuối', s('from') && s('to') ? `đoạn ${s('from')}–${s('to')}` : '', s('character') && `lời thoại của ${s('character')}`)
      add('Chương', s('chapter')); add('Bản', s('source') === 'draft' ? 'Bản nháp' : 'Bản cuối'); add('Nhân vật', s('character')); add('Độ dài', numberText(r.word_count ?? r.words) && `${numberText(r.word_count ?? r.words)} chữ`); add('Nội dung', r.content ?? r.text); break
    case 'plan_chapter':
      name = 'Lập kế hoạch chương'; role = 'Người viết'; summary = join(chapter, s('title'))
      for (const [label, field] of [['Mục tiêu','goal'],['Móc cuối','hook'],['Cảm xúc','emotion'],['Xung đột','conflict'],['Bắt buộc có','required_beats'],['Tránh','forbidden_moves'],['Kiểm liên tục','continuity_checks']]) add(label, get(field)); break
    case 'draft_chapter':
      name = 'Viết bản nháp'; role = 'Người viết'; summary = join(chapter, s('mode') === 'append' ? 'viết tiếp' : 'viết mới', numberText(r.words ?? get('word_count')) && `${numberText(r.words ?? get('word_count'))} chữ`)
      add('Cách ghi', s('mode') === 'append' ? 'Viết tiếp' : 'Viết mới, thay bản cũ'); add('Số chữ', r.words ?? get('word_count')); add('Trích', get('content')); break
    case 'edit_chapter':
      name = 'Gọt giũa'; role = 'Người viết'; summary = join(chapter, get('replace_all') === true ? 'thay mọi chỗ' : 'sửa 1 đoạn')
      add('Trước', get('old_string')); add('Sau', get('new_string')); add('Phạm vi', get('replace_all') === true ? 'Mọi chỗ' : 'Một chỗ'); break
    case 'check_consistency':
      name = 'Soát nhất quán'; role = 'Người viết'
      { const sources = [['world_rules', 'quy tắc thế giới'], ['foreshadow', 'cài cắm'], ['relations', 'quan hệ nhân vật'], ['aliases', 'biệt danh và cách gọi'], ['summaries', 'tóm tắt']].filter(([field]) => Array.isArray(r[field])); summary = join(chapter, `nạp ${sources.length} nguồn đối chiếu`); for (const [field, label] of sources) add('Đã nạp để đối chiếu', `${list(r[field]).length} ${label}`) }
      add('Đối chiếu', 'Người viết tự đối chiếu bản nháp; chỗ lệch (nếu có) thành thẻ Gọt giũa ngay sau.'); break
    case 'commit_chapter':
      name = 'Lưu chương'; role = 'Người viết'; summary = join(chapter, numberText(r.words ?? get('word_count')) && `${numberText(r.words ?? get('word_count'))} chữ`, Array.isArray(r.foreshadow ?? get('foreshadow_updates')) && `${list(r.foreshadow ?? get('foreshadow_updates')).length} cài cắm`, Array.isArray(get('address_terms')) && `${arr('address_terms').length} xưng hô`)
      for (const [label, value] of [['Chương',get('chapter')],['Số chữ',r.words ?? get('word_count')],['Kiểu móc',get('hook_type')],['Nhân vật',get('characters')],['Cài cắm',r.foreshadow ?? get('foreshadow_updates')],['Xưng hô mới',get('address_terms')],['Sự kiện chính',get('events')]] as const) add(label, value); break
    case 'audit_foundation':
      name = 'Soát nền truyện'; role = 'Kiến trúc sư'; summary = join(get('ready') === true ? 'sẵn sàng' : 'chưa sẵn sàng', `${arr('issues').length} lưu ý`); add('Tóm tắt', get('summary')); add('Vấn đề', get('issues')); break
    case 'revise_outline':
      name = 'Sửa dàn ý'; role = 'Kiến trúc sư'; summary = join(s('from_chapter') && `từ chương ${s('from_chapter')}`, `đổi ${list(r.changed ?? get('chapters')).length} chương`); add('Lý do', get('reason')); add('Các chương', r.changed ?? get('chapters')); break
    case 'resolve_outline_feedback':
      name = 'Giữ dàn ý'; role = 'Kiến trúc sư'; summary = 'không cần đổi'; add('Lý do giữ', get('reason')); break
    case 'save_book':
      name = 'Lưu thông tin truyện'; role = 'Kiến trúc sư'; summary = 'tên + giới thiệu'; add('Tên', get('title')); add('Giới thiệu', get('synopsis')); break
    case 'save_foundation': {
      role = 'Kiến trúc sư'
      const kind = s('type'), contents = p.content
      if (kind === 'premise') { name = 'Lưu tiền đề'; summary = join(`${Array.isArray(get('axes')) ? arr('axes').length : Object.keys(object(get('axes'))).length} mục`, s('nguoi_nghe') && `người nghe ${s('nguoi_nghe')}`); add('Các mục', get('axes')); add('Người nghe', get('nguoi_nghe')); add('Điểm khác', get('diem_khac')); add('Kết quả', r.content) }
      else if (kind === 'characters') { const characters = list(contents); name = 'Lưu nhân vật'; summary = join(`${characters.length} nhân vật`, `${characters.filter(item => object(item).role === 'main').length} chính`); add('Nhân vật', contents) }
      else if (kind === 'world_rules') { name = 'Lưu quy tắc thế giới'; summary = `${list(contents).length} quy tắc`; add('Quy tắc', contents) }
      else if (['outline','layered_outline','expand_arc','append_volume'].includes(kind)) { name = 'Lưu dàn ý'; summary = join(`${arr('chapters').length || list(contents).length} chương`, arr('volumes').length > 0 && `${arr('volumes').length} tập`); add('Các chương', get('chapters') ?? contents) }
      else if (kind === 'update_compass') { name = 'Cập nhật hướng đi'; summary = `${arr('open_threads').length} tuyến còn mở`; add('Hướng kết', get('ending_direction')); add('Tuyến còn mở', get('open_threads')) }
      else if (kind === 'complete_book') { name = 'Hoàn tất truyện'; summary = join(`${arr('chapters').length} chương`, 'đã khép mọi tuyến'); add('Lý do', get('reason')) }
      else { role = ''; add('Thông tin', 'Noverse chưa có thẻ riêng cho thao tác này.') }
      break
    }
    case 'save_review': {
      name = 'Lưu thẩm duyệt'; role = 'Biên tập viên'; const verdict: Record<string,string> = { accept:'Đạt', polish:'Gọt thêm', rewrite:'Viết lại' }
      summary = join(chapter, verdict[s('verdict')], `${arr('issues').length} cảnh báo`); add('Kết luận', verdict[s('verdict')]); add('Điểm theo tiêu chí', get('dimensions')); add('Vấn đề', get('issues')); break
    }
    case 'save_arc_summary':
      name = 'Tóm tắt cung'; role = 'Biên tập viên'; summary = join(s('volume') && `tập ${s('volume')}`, s('arc') && `cung ${s('arc')}`, `${arr('key_events').length} sự kiện`); add('Tóm tắt', get('summary')); add('Sự kiện chính', get('key_events')); add('Ảnh chụp nhân vật', get('character_snapshots')); break
    case 'save_volume_summary':
      name = 'Tóm tắt tập'; role = 'Biên tập viên'; summary = join(s('volume') && `tập ${s('volume')}`, `${arr('chapters').length} chương`); add('Tóm tắt', get('summary')); break
    default: add('Thông tin', 'Noverse chưa có thẻ riêng cho thao tác này.')
  }
  return { name, role, summary, blocks }
}
