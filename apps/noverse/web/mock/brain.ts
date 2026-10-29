// Mock Não truyện cho truyện mẫu (22-brief-2510.md › D10, artboard Brain.dc.html):
// 8 chương, 13 thực thể, 16 cạnh đổi kiểu theo chương, 7 dòng xưng hô. Dữ liệu lấy nguyên bảng
// N / E / XH / TITLES của artboard — không thêm bớt thực thể hay quan hệ nào.
import type { AddressTerm, Brain, BrainEdge, BrainEdgeType, BrainEntity, BrainEntityType, Character, CharacterKind } from '../src/api/types'
import type { MockAddress } from './address'

export const SAMPLE_STORY = '01-tra-xanh-cuop-kich-ban'

const TITLES = [
  'Vai của tôi là đồ canh', 'Chiếc cặp ở giữa bàn', 'Hai chiếc bát trên bàn', 'Bảy mươi ba giây',
  'Tôi cướp vai cái nồi à?', 'Chiếc vỏ ổ cứng', 'Dòng tên bị gạch', 'Tên tôi trên màn hình',
]

/** [type, name, vai hiển thị, chương xuất hiện, mô tả, các chương có mặt] */
const N: Record<string, [BrainEntityType, string, string, number, string, number[]]> = {
  th: ['char', 'Lâm Tri Hạ', 'Nữ chính · người kể', 1, 'Biên kịch độc lập 26 tuổi, người kể chuyện. Viết kịch bản gốc nhưng bị xếp vai phụ.', [1, 2, 3, 4, 5, 6, 7, 8]],
  nv: ['char', 'Tống Nhược Vy', 'Phản diện chính', 1, 'Ảnh hậu 28 tuổi, đứng tên tác giả kịch bản của Tri Hạ.', [1, 2, 3, 4, 5, 6, 7]],
  pc: ['char', 'Phùng Chấn', 'Đồng phạm', 1, 'Giám đốc điều hành Tinh Mang. Người kể chưa biết ông đứng sau vụ cướp kịch bản cho tới chương 5.', [1, 4, 5, 7]],
  hc: ['char', 'Hứa Hoài Châu', 'Nam chính phụ trợ', 1, 'Đạo diễn 32 tuổi của chương trình, giữ lại bản live đọc thoại.', [1, 3, 4, 6, 7]],
  kd: ['char', 'Kỳ Dương', 'Đồng minh hài', 2, 'Nam ca sĩ thần tượng 24 tuổi, cùng nhóm với Tri Hạ trong show.', [2, 3, 4, 5, 7, 8]],
  dm: ['char', 'Đỗ Mẫn', 'Phụ chức năng', 3, 'Tổng đạo diễn show, giữ luật chơi công bằng.', [3, 7]],
  kb: ['obj', 'Kịch bản gốc', '', 1, 'Bản Tri Hạ viết, đang đứng tên Nhược Vy.', [1, 2, 4, 5, 6, 8]],
  rec: ['obj', 'Bản live đọc thoại', '', 4, 'Bản ghi buổi đọc thoại trực tiếp, có đoạn 73 giây bị cắt.', [4, 5, 6]],
  ev: ['ev', 'Buổi đọc thoại live', '', 2, 'Trò đọc thoại trực tiếp. Được báo trước từ chương 2, diễn ra ở chương 4.', [2, 3, 4]],
  tq: ['place', 'Trường quay số ba', '', 3, 'Nơi quay buổi đọc thoại live.', [3, 4]],
  solao: ['fs', 'Số lạ nhắn tin', '', 3, 'Tin nhắn đe doạ cuối chương 3. Chưa rõ người gửi.', [3, 4, 6]],
  vo: ['fs', 'Vỏ ổ cứng', '', 5, 'Vỏ ổ cứng cũ, có thể từng chứa bản gốc. Ruột đã bị tháo.', [5, 6, 8]],
  luat: ['rule', 'Luật thẻ ngẫu nhiên', '', 2, 'Thẻ rút ngẫu nhiên, xáo công khai, không ai được nhường lượt.', [2, 4]],
}

/** [a, b, phases [[từ chương, kiểu, nhãn]]] */
const E: [string, string, [number, BrainEdgeType, string][]][] = [
  ['th', 'nv', [[1, 'work', 'Bạn diễn cũ'], [2, 'enemy', 'Đối đầu']]],
  ['th', 'kb', [[1, 'own', 'Người viết thật']]],
  ['nv', 'kb', [[1, 'own', 'Đứng tên tác giả']]],
  ['th', 'hc', [[1, 'work', 'Công việc'], [4, 'ally', 'Tin tưởng một phần']]],
  ['th', 'kd', [[2, 'ally', 'Đồng minh']]],
  ['pc', 'nv', [[5, 'secret', 'Chống lưng']]],
  ['pc', 'th', [[1, 'work', 'Công việc'], [4, 'enemy', 'Ép xoá bản live']]],
  ['hc', 'rec', [[4, 'own', 'Giữ lại']]],
  ['th', 'solao', [[3, 'rel', 'Nhận tin đe doạ']]],
  ['vo', 'kb', [[5, 'secret', 'Chứa bản gốc?']]],
  ['th', 'ev', [[4, 'rel', 'Nối đúng câu bị ngắt']]],
  ['nv', 'ev', [[4, 'rel', 'Đọc hỏng']]],
  ['luat', 'ev', [[2, 'rel', 'Áp dụng']]],
  ['ev', 'tq', [[3, 'rel', 'Diễn ra tại']]],
  ['dm', 'th', [[3, 'work', 'Công việc']]],
  ['ev', 'rec', [[4, 'rel', 'Ghi lại']]],
]

/** Xưng hô: [from, to, gọi, tự xưng, từ chương, tới chương (0 = còn hiệu lực), lý do] */
const XH: [string, string, string, string, number, number, string][] = [
  ['th', 'nv', 'chị Vy', 'em', 1, 1, 'đồng nghiệp cũ'],
  ['th', 'nv', 'cô', 'tôi', 2, 0, 'cắt đứt sau khi biết bị cướp tên'],
  ['nv', 'th', 'em', 'chị', 1, 0, 'giữ vẻ đan chị'],
  ['th', 'hc', 'đạo diễn Hứa', 'tôi', 1, 0, 'công việc'],
  ['kd', 'th', 'chị', 'em', 2, 0, 'kém hai tuổi'],
  ['pc', 'th', 'cô Lâm', 'tôi', 1, 0, 'xa cách'],
  ['pc', 'nv', 'Vy', 'anh', 5, 0, 'lộ quan hệ ngầm'],
]

/**
 * Diễn biến mỗi thực thể, suy đúng như artboard (`hist`): mỗi phase của cạnh chạm thực thể
 * (“Đổi thành … · người kia” từ phase thứ hai) + mỗi dòng xưng hô đổi từ chương 2 trở đi.
 */
const timelineOf = (id: string): { chapter: number; text: string }[] => {
  const rows: { chapter: number; text: string }[] = []
  for (const [a, b, phases] of E) {
    if (a !== id && b !== id) continue
    const other = N[a === id ? b : a][1]
    phases.forEach(([chapter, , label], index) => {
      rows.push({ chapter, text: `${index ? 'Đổi thành ' : ''}${label} · ${other}` })
    })
  }
  for (const [from, to, call, , chapter] of XH) {
    if (chapter <= 1 || (from !== id && to !== id)) continue
    rows.push({ chapter, text: `${N[from][1]} gọi ${N[to][1]} là “${call}”` })
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
}

/** Loại vai + chương lộ vai của 6 nhân vật truyện mẫu (Gói S · artboard Main.dc.html › CH). */
const KIND: Record<string, { kind: CharacterKind; revealAt?: number }> = {
  th: { kind: 'lead' },
  nv: { kind: 'antagonist' },
  pc: { kind: 'antagonist', revealAt: 5 },
  kd: { kind: 'ally' },
  hc: { kind: 'support' },
  dm: { kind: 'support' },
}
/** Chữ viết tắt avatar người dùng tự đặt (artboard `ini`). */
const INI: Record<string, string> = { th: 'TH', nv: 'NV', pc: 'PC', kd: 'KD', hc: 'HC', dm: 'ĐM' }

const entities: BrainEntity[] = Object.entries(N).map(([id, [type, name, role, fromChapter, desc, appears]]) => ({
  id, type, name, role, desc, fromChapter, appears, aliases: [], timeline: timelineOf(id),
  ...(KIND[id] ?? {}),
  ini: INI[id],
}))

const edges: BrainEdge[] = E.map(([a, b, phases]) => ({
  a, b, phases: phases.map(([fromChapter, type, label]) => ({ fromChapter, type, label })),
}))

/** 8 chương như artboard: 1–3 đã viết, 4 đang viết lại, 5–8 chưa viết. */
const chapters: Brain['chapters'] = TITLES.map((title, index) => ({
  n: index + 1,
  title,
  status: index < 3 ? 'written' : index === 3 ? 'rewriting' : 'unwritten',
}))

/** `GET S/brain` cho truyện mẫu; `null` để server rơi về `mockBrainOf` (dữ liệu theo dàn nhân vật). */
export const mockBrainDetailOf = (story: string): Brain | null =>
  story === SAMPLE_STORY ? { chapters, entities, edges }
    : story === DRAWER_STORY ? { chapters: drawerChapters, entities: drawerEntities, edges: drawerEdges }
      : null

/** `?upTo=N` — cắt thực thể/cạnh về những gì đã có tới chương N; `chapters[]` giữ nguyên như `naoTrimBrain`. */
export const trimBrain = (brain: Brain, upTo: number): Brain => {
  const entities = brain.entities
    .filter((entity) => entity.fromChapter <= upTo)
    .map((entity) => ({
      ...entity,
      appears: entity.appears.filter((n) => n <= upTo),
      timeline: entity.timeline.filter((item) => item.chapter <= upTo),
    }))
  const kept = new Set(entities.map((entity) => entity.id))
  return {
    chapters: brain.chapters,
    entities,
    edges: brain.edges
      .filter((edge) => kept.has(edge.a) && kept.has(edge.b))
      .map((edge) => ({ ...edge, phases: edge.phases.filter((phase) => phase.fromChapter <= upTo) }))
      .filter((edge) => edge.phases.length > 0),
  }
}

/** Vai ở trang Nhân vật (D7): khoá đúng bảng ROLE_TAG của artboard, không phải nhãn thẻ đồ thị. */
const CAST_ROLE: Record<string, string> = {
  th: 'nữ chính', nv: 'phản diện chính', pc: 'đồng phạm', hc: 'nam chính phụ trợ', kd: 'đồng minh hài', dm: 'phụ chức năng',
}

/**
 * `GET S/characters` của truyện mẫu — dẫn xuất từ chính bảng N ở trên (một nguồn, không chép lại).
 * `supporting` rỗng: Não truyện của artboard không có nhân vật phụ nào ngoài 6 người này.
 */
export const sampleCharacters = (story: string) => story !== SAMPLE_STORY ? null : {
  // Đúng 6 nhân vật của artboard Main.dc.html (TH/NV/PC/KD/HC/ĐM): chụp nghiệm thu Gói S
  // đối chiếu 1–1 với danh sách Nhân vật của artboard. Tên có danh xưng/ini tự đặt vẫn có
  // trong dàn "Kiều My" của truyện khác (server.ts) nên luật tính chữ tắt vẫn được phủ.
  characters: [
    ...Object.entries(N)
      .filter(([, entity]) => entity[0] === 'char')
      .map(([id, [, name, , , desc]]): Character => ({
        name, aliases: [], role: CAST_ROLE[id] ?? 'phụ chức năng', description: desc, arc: '', traits: [], tier: 'main',
        kind: KIND[id]?.kind ?? 'support', revealAt: KIND[id]?.revealAt ?? null, ini: INI[id],
      })),
  ],
  supporting: [],
}

/**
 * Bảng xưng hô của truyện mẫu — dựng từ bảng XH (id thực thể + tên hiển thị như máy chủ thật).
 * `relations` để rỗng: artboard Não truyện chỉ có bảng xưng hô, không có nhật ký đổi quan hệ.
 */
export const sampleAddress = (story: string): MockAddress | null =>
  story === DRAWER_STORY ? { terms: DRAWER_TERMS, relations: [] }
    : story !== SAMPLE_STORY ? null : {
      terms: XH.map(([from, to, call, self, fromChapter, untilChapter, reason], index): AddressTerm => ({
        id: `xh-${index + 1}`, from, fromName: N[from][1], to, toName: N[to][1], call, self, fromChapter,
        untilChapter: untilChapter || null, reason,
      })),
      relations: [],
    }

/**
 * Truyện mẫu thứ hai cho ngăn kéo Não truyện (bug 2710 — “Thiên Kim Thật Đòi Lại Giọng Nói”).
 * Dựng đúng quy mô truyện thật: 36 thực thể (7 nhân vật, 6 địa điểm, 10 vật, 10 sự kiện, 3 luật),
 * 60 cạnh, phần lớn là `rel` với `label` là câu mô tả dài như máy chủ thật gửi, hai thực thể chỉ
 * xuất hiện sau chương 4. Mở ngăn kéo ở chương 4 là thấy cảnh từng bị lỗi: nhãn cạnh dài tràn
 * khung, nút của chương sau hiện mờ, hàng chục nhãn “Liên quan” đè nhau.
 */
export const DRAWER_STORY = '07-thien-kim-giong-noi'

const DRAWER_TITLES = [
  'Giọng thu thử', 'Bản gốc có người giữ', 'Buổi thu không có giấy', 'Tín hiệu không trả được tiền thuê',
  'Điện thoại trong túi xách', 'Hợp đồng lồng tiếng', 'Sân khấu hội thảo', 'Giọng của ai',
]

const drawerChapters: Brain['chapters'] = DRAWER_TITLES.map((title, index) => ({
  n: index + 1,
  title,
  status: index < 3 ? 'written' : index === 3 ? 'writing' : 'unwritten',
}))

const drawerAppears = (from: number) => Array.from({ length: 8 - from + 1 }, (_, index) => from + index)

/** [id, type, name, vai, chương xuất hiện, mô tả] */
const DRAWER_N: [string, BrainEntityType, string, string, number, string][] = [
  ['la', 'char', 'Lâm An', 'Nữ chính · người kể xưng tôi', 1, 'Biên kịch bị gán cáo buộc dùng giọng người khác; đang đối chiếu bản ghi âm với hợp đồng giọng mẫu.'],
  ['ln', 'char', 'Lâm Nhã', 'Phản diện · em gái', 1, 'Em gái của An, giữ bản ghi âm đã cắt và ký hợp đồng dưới tên mình.'],
  ['lk', 'char', 'Lâm Khải', 'Cha ruột · chủ dự án Mùa tro', 1, 'Ép An nhận lỗi để giữ dự án và thương hiệu gia đình, đặt hạn 48 giờ.'],
  ['hv', 'char', 'Hạ Vy', 'Ca sĩ · người bị huỷ lịch thu', 2, 'Mất show vì đoạn ghi âm, nhận tiền bù nhưng chưa phục hồi lòng tin.'],
  ['bh', 'char', 'Bùi Hạnh', 'Quản lý phòng thu Mùa tro', 2, 'Người giao chìa khoá và phiếu gọi thu cho Nhã.'],
  ['pc', 'char', 'Phạm Chi', 'Luật sư của Lâm Khải', 3, 'Soạn điều khoản bồi thường dựa trên hợp đồng giọng mẫu.'],
  ['td', 'char', 'Trần Duy', 'Kỹ thuật viên hậu kỳ', 3, 'Dựng lại đoạn cắt của bản ghi âm trong phòng thu.'],
  ['pt', 'place', 'Phòng thu Mùa tro', '', 1, 'Nơi thu bản gốc và là chỗ An quay lại tìm thiết bị.'],
  ['hc', 'place', 'Phòng chờ hậu trường', '', 2, 'Phòng chờ riêng của hội thảo, nơi An bị chặn trước giờ lên sân khấu.'],
  ['vp', 'place', 'Văn phòng luật sư Phạm Chi', '', 3, 'Nơi ký điều khoản bồi thường và đối chiếu bản gốc.'],
  ['cg', 'place', 'Cánh gà sân khấu', '', 4, 'Chỗ An đứng nghe lại đoạn ghi âm trước khi ra hội thảo.'],
  ['htl', 'place', 'Hội trường lễ giới thiệu dự án An Thanh', '', 4, 'Sân khấu ra mắt dự án, nơi bản ghi âm được phát trước báo giới.'],
  ['qd', 'place', 'Quầy dữ liệu Vọng Âm', '', 2, 'Quầy lưu phiên nghe và biên nhận tải bản gốc.'],
  ['dt', 'obj', 'Điện thoại của An', '', 1, 'Máy chứa tin nhắn đe doạ và ảnh phiếu gọi thu.'],
  ['mc', 'obj', 'Micro sân khấu', '', 1, 'Micro của buổi thu, còn lưu đoạn thoại chưa qua xử lý.'],
  ['hdgm', 'obj', 'Hợp đồng giọng mẫu', '', 1, 'Hợp đồng cấp quyền giọng có chữ ký điện tử thật của An.'],
  ['apt', 'obj', 'Ảnh phiếu gọi thu', '', 2, 'Ảnh chụp phiếu gọi thu do Vy gửi, có giờ và số hiệu phiên.'],
  ['pgt', 'obj', 'Phiếu gọi thu Mùa tro', '', 2, 'Phiếu gọi thu ghi tên Nhã nhưng giọng trong bản là của An.'],
  ['mh', 'obj', 'Màn hình sân khấu', '', 3, 'Màn hình phát đoạn ghi âm trước hội trường.'],
  ['kbmt', 'obj', 'Kịch bản Mùa tro', '', 1, 'Kịch bản gốc của dự án, bị đổi tên tác giả ở trang bìa.'],
  ['bnpt', 'obj', 'Biên nhận phiên thu', '', 2, 'Biên nhận do quầy dữ liệu Vọng Âm in ra, có mã phiên.'],
  ['sl', 'obj', 'Số lạ nhắn đe doạ', '', 3, 'Số gửi tin đe doạ vụ bám đạo diễn; chưa xác định người gửi.'],
  ['tka', 'obj', 'Tài khoản Vọng Âm', '', 5, 'Tài khoản phát hành bản ghi âm; chỉ mở sau chương 4.'],
  ['e1', 'ev', 'Hội thảo ra mắt dự án audio', '', 1, 'Buổi giới thiệu dự án, nơi phát đoạn ghi âm gây tranh cãi.'],
  ['e2', 'ev', 'An nhận email yêu cầu xin lỗi', '', 2, 'Email yêu cầu bồi thường trong 48 giờ kèm hợp đồng đính kèm.'],
  ['e3', 'ev', 'An đối chiếu hợp đồng giọng nói', '', 2, 'An so chữ ký trên hợp đồng với bản lưu trong máy.'],
  ['e4', 'ev', 'An nộp yêu cầu bảo toàn dữ liệu', '', 3, 'Yêu cầu gửi phòng thu, buộc giữ nguyên bản ghi và phiên nghe.'],
  ['e5', 'ev', 'Phòng thu từ chối quyền nghe bản gốc', '', 3, 'Bùi Hạnh từ chối cho An nghe lại bản gốc tại quầy.'],
  ['e6', 'ev', 'An trả phí tư vấn buổi đầu', '', 3, 'An trả phí để Phạm Chi xem hồ sơ buổi thu.'],
  ['e7', 'ev', 'Vy gửi ảnh phiếu gọi thu', '', 4, 'Vy gửi ảnh trong nhóm chung, kèm giờ và số hiệu phiên.'],
  ['e8', 'ev', 'An đăng phiếu gọi thu', '', 4, 'An đăng phiếu gọi thu lên trang cá nhân trước hội thảo.'],
  ['e9', 'ev', 'An có thiết bị cơ bản', '', 4, 'Thiết bị ghi âm cơ bản của An được kiểm tra trước khi lên sân khấu.'],
  ['e10', 'ev', 'Hải Thành lá đổi thị hiếu', '', 5, 'Sự kiện sau chương 4, chỉ để kiểm tra lọc thực thể chưa xuất hiện.'],
  ['r1', 'rule', 'Điều khoản lồng tiếng', '', 1, 'Điều khoản buộc người thu nhường quyền sử dụng giọng cho bên đặt hàng.'],
  ['r2', 'rule', 'Luật bảo toàn dữ liệu', '', 2, 'Bên giữ bản ghi phải giữ nguyên dữ liệu khi có yêu cầu bảo toàn.'],
  ['r3', 'rule', 'Quy định quyền nghe bản gốc', '', 3, 'Người thu chỉ được nghe lại bản gốc khi có mặt hai bên.'],
]

/** Tên gọi trần xuất hiện trong chính văn của truyện thứ hai (bug 2710: “Nhã”, “An”, “Duy”…). */
const DRAWER_ALIASES: Record<string, string[]> = {
  la: ['An'], ln: ['Nhã'], lk: ['Khải'], hv: ['Vy'], bh: ['Hạnh'], pc: ['Chi'], td: ['Duy'],
}

const drawerEntities: BrainEntity[] = DRAWER_N.map(([id, type, name, role, fromChapter, desc]) => ({
  id, type, name, role, desc, fromChapter, appears: drawerAppears(fromChapter), aliases: DRAWER_ALIASES[id] ?? [], timeline: [],
}))

const drawerName = new Map(DRAWER_N.map(([id, , name]) => [id, name]))
const drawerFrom = new Map(DRAWER_N.map(([id, , , , fromChapter]) => [id, fromChapter]))

/** Cạnh quanh Lâm An (mục chọn mặc định): kiểu chính trước, `rel` sau cùng. */
const DRAWER_MAIN_EDGES: [string, string, BrainEdgeType, number, string][] = [
  ['la', 'ln', 'enemy', 2, 'Lâm Nhã đứng tên bản ghi âm của An trong hợp đồng lồng tiếng, rồi đăng ảnh phiếu gọi thu để đẩy cáo buộc sang An.'],
  ['la', 'lk', 'enemy', 1, 'Lâm Khải đặt hạn 48 giờ buộc An nhận lỗi, nếu không sẽ cắt quyền khai thác kịch bản Mùa tro.'],
  ['la', 'hv', 'ally', 2, 'Hạ Vy nhận tiền bù nhưng vẫn giữ tin nhắn và ảnh phiếu gọi thu làm chứng cho An.'],
  ['la', 'bh', 'work', 1, 'Bùi Hạnh giao phiếu gọi thu và chìa khoá phòng thu, sau đó từ chối cho An nghe lại bản gốc.'],
  ['la', 'pc', 'enemy', 3, 'Phạm Chi soạn điều khoản bồi thường dựa trên hợp đồng giọng mẫu có chữ ký thật của An.'],
  ['la', 'td', 'ally', 3, 'Trần Duy dựng lại đoạn cắt trong bản ghi âm và chỉ ra chỗ ghép giọng.'],
  ['la', 'pt', 'work', 1, 'An thu bản gốc tại phòng thu Mùa tro đúng buổi chiều Nhã ghé qua và nhận phiếu gọi thu.'],
  ['la', 'vp', 'own', 3, 'Văn phòng luật sư Phạm Chi giữ bản sao hợp đồng giọng mẫu và hồ sơ bồi thường.'],
  ['la', 'cg', 'work', 4, 'Cánh gà sân khấu là nơi An nghe lại đoạn ghi âm trước khi ra hội thảo.'],
  ['la', 'htl', 'own', 4, 'Hội trường lễ giới thiệu dự án An Thanh đã đặt lịch phát đoạn ghi âm trước báo giới.'],
  ['la', 'dt', 'own', 1, 'Điện thoại của An giữ tin nhắn đe doạ, ảnh phiếu gọi thu và bản sao hợp đồng.'],
  ['la', 'mc', 'work', 1, 'Micro sân khấu còn lưu đoạn thoại chưa qua xử lý của buổi thu.'],
  ['la', 'hdgm', 'work', 1, 'Hợp đồng giọng mẫu có chữ ký điện tử thật của An, phạm vi cấp quyền chưa được đối chiếu.'],
  ['la', 'apt', 'work', 2, 'Ảnh phiếu gọi thu do Vy gửi cho thấy giờ và số hiệu phiên thu khớp với bản gốc.'],
  ['la', 'pgt', 'own', 2, 'Phiếu gọi thu Mùa tro ghi tên Nhã nhưng giọng trong bản thu là của An.'],
  ['la', 'mh', 'own', 3, 'Màn hình sân khấu được lệnh phát đoạn ghi âm trước hội trường lúc ra mắt dự án.'],
  ['la', 'sl', 'secret', 3, 'Số lạ nhắn đe doạ An về vụ bám đạo diễn; người gửi chưa lộ.'],
  ['la', 'kbmt', 'rel', 1, 'Kịch bản Mùa tro bị đổi tên tác giả ở trang bìa, An còn bản lưu trong máy.'],
  ['la', 'bnpt', 'rel', 2, 'Biên nhận phiên thu do quầy dữ liệu Vọng Âm in ra, có mã phiên trùng với ảnh phiếu.'],
  ['la', 'tka', 'rel', 5, 'Tài khoản Vọng Âm phát hành bản ghi âm, chỉ mở sau chương 4.'],
  ['ln', 'bh', 'own', 2, 'Bùi Hạnh giữ chìa khoá phòng thu và bản sao phiếu gọi thu đứng tên Nhã.'],
  ['hv', 'bh', 'work', 2, 'Hạ Vy và Bùi Hạnh từng làm chung buổi thu trước khi lịch diễn bị huỷ.'],
  ['lk', 'vp', 'own', 3, 'Lâm Khải uỷ quyền cho văn phòng Phạm Chi xử lý hồ sơ bồi thường.'],
  ['pc', 'td', 'work', 3, 'Phạm Chi và Trần Duy trao đổi về đoạn cắt trong bản ghi âm.'],
]

const DRAWER_RING = [
  'la', 'ln', 'lk', 'hv', 'bh', 'pc', 'td', 'pt', 'hc', 'vp', 'cg', 'htl', 'qd', 'dt', 'mc',
  'hdgm', 'apt', 'pgt', 'mh', 'kbmt', 'bnpt', 'sl', 'e1', 'e2', 'e3', 'e4', 'e5', 'e6', 'e7', 'e8', 'e9', 'r1', 'r2',
]

const drawerEdges: BrainEdge[] = (() => {
  const edges: BrainEdge[] = DRAWER_MAIN_EDGES.map(([a, b, type, fromChapter, label]) =>
    ({ a, b, phases: [{ fromChapter, type, label }] }))
  const seen = new Set(edges.map((edge) => [edge.a, edge.b].sort().join('|')))
  for (const step of [5, 11, 17]) {
    for (let i = 0; i < DRAWER_RING.length && edges.length < 60; i++) {
      const a = DRAWER_RING[i]
      const b = DRAWER_RING[(i + step) % DRAWER_RING.length]
      const key = [a, b].sort().join('|')
      if (a === b || seen.has(key)) continue
      seen.add(key)
      const chapter = Math.max(drawerFrom.get(a) ?? 1, drawerFrom.get(b) ?? 1)
      edges.push({
        a, b,
        phases: [{
          fromChapter: chapter, type: 'rel',
          label: `${drawerName.get(a)} và ${drawerName.get(b)} còn ràng buộc ở chương ${chapter} vì buổi thu tại phòng thu Mùa tro chưa được đối chiếu xong.`,
        }],
      })
    }
    if (edges.length >= 60) break
  }
  return edges
})()

/** Bảng xưng hô của truyện thứ hai: đủ dài để cột chi tiết phải cuộn trong ngăn kéo. */
const DRAWER_TERMS: AddressTerm[] = [
  { id: 'xh-1', from: 'la', fromName: 'Lâm An', to: 'ln', toName: 'Lâm Nhã', call: 'em', self: 'chị', fromChapter: 1, untilChapter: 1, reason: 'Chị em trong nhà, còn giữ phép' },
  { id: 'xh-2', from: 'la', fromName: 'Lâm An', to: 'ln', toName: 'Lâm Nhã', call: 'cô Nhã', self: 'tôi', fromChapter: 2, untilChapter: null, reason: 'Cắt đứt sau khi biết Nhã đứng tên bản ghi âm' },
  { id: 'xh-3', from: 'la', fromName: 'Lâm An', to: 'lk', toName: 'Lâm Khải', call: 'ba', self: 'con', fromChapter: 1, untilChapter: null, reason: 'Cha ruột, nhưng đã cắt quyền khai thác kịch bản' },
  { id: 'xh-4', from: 'la', fromName: 'Lâm An', to: 'hv', toName: 'Hạ Vy', call: 'chị Vy', self: 'tôi', fromChapter: 2, untilChapter: null, reason: 'Đồng minh tạm, chưa phục hồi lòng tin' },
  { id: 'xh-5', from: 'la', fromName: 'Lâm An', to: 'pc', toName: 'Phạm Chi', call: 'luật sư Chi', self: 'tôi', fromChapter: 3, untilChapter: null, reason: 'Luật sư phía đối diện trong hồ sơ bồi thường' },
  { id: 'xh-6', from: 'la', fromName: 'Lâm An', to: 'bh', toName: 'Bùi Hạnh', call: 'chị Hạnh', self: 'tôi', fromChapter: 1, untilChapter: null, reason: 'Quản lý phòng thu, người giao phiếu gọi thu' },
]

/**
 * Chính văn của truyện mẫu dùng đúng dàn nhân vật của Não truyện: chỉ đổi tên, không viết lại câu.
 * Bảng đổi tên theo vai — Kiều My (tác giả mất tên) → Lâm Tri Hạ, Trần Nga (giữ bản thảo) → Tống Nhược Vy,
 * Nhân Phát (người đánh máy, đồng minh) → Hứa Hoài Châu, Quách (tác quyền) → Phùng Chấn.
 */
const PROSE_CAST: [RegExp, string][] = [
  [/Kiều My/g, 'Lâm Tri Hạ'],
  [/Trần Nga/g, 'Tống Nhược Vy'],
  [/Nhân Phát/g, 'Hứa Hoài Châu'],
  [/Quách/g, 'Phùng Chấn'],
]

/**
 * Chính văn truyện thứ hai, chép từ ca bug 2710: “Nhãn phân loại” bị gạch vì chứa “Nhã”, “Anh bắt máy”
 * bị gạch vì chứa “An”; “Duy” thì đúng. Dùng làm mốc để mở Trình đọc trên điện thoại và soi gạch chấm.
 */
const DRAWER_PROSE = [
  'cứ ký rồi tính. Có chỉ chuyển tiếp biểu phí và nhắc hạn nộp. Tôi tải xuống, lưu cùng hóa đơn bán thiết bị và những khoản đã trả cho việc này. Con số trên email không biến mất khi tôi nhìn chỗ khác.',
  'Danh mục tệp sơ bộ đến vào buổi chiều. Hai dòng được xếp thành “diễn thử” và “xác nhận cấp phép thương mại”. Tôi phóng to chữ trên màn hình, rồi lại thu nhỏ. Nhãn phân loại chỉ cho biết người lưu kho chia chúng ra sao; nó không nói tôi đã nói câu gì, càng không cho biết lúc đó ai giải thích mục đích ghi âm.',
  'Tôi gọi cho Duy. Anh bắt máy sau hồi chuông thứ hai.',
  '“Tôi muốn hỏi anh có thể giải thích quy trình xuất bản đối chiếu không.”',
  '“Tôi có thể giải thích hệ thống làm gì, trong phạm vi công việc tôi từng phụ trách.” Giọng Duy gọn như lúc đọc tên một trường dữ liệu. “Nhưng tôi không ký xác nhận toàn bộ gói là đầy đủ hay nguyên trạng. Tôi từng tham gia thiết kế quy trình nhập chấp thuận theo lô.”',
  'Tôi kéo danh mục về phía mình. “Tôi đâu bảo…” Lâm An đọc lại từng dòng, còn Nhã thì không nhắc tới.',
].join('\n\n')

/** Tiêu đề chương lấy từ chính Não truyện để rail và trình đọc không lệch nhau. */
export const sampleChapterTitle = (story: string, chapter: number): string | null =>
  story === SAMPLE_STORY ? chapters.find((item) => item.n === chapter)?.title ?? null
    : story === DRAWER_STORY ? drawerChapters.find((item) => item.n === chapter)?.title ?? null
      : null

export const sampleProse = (story: string, text: string): string =>
  story === SAMPLE_STORY ? PROSE_CAST.reduce((out, [pattern, name]) => out.replace(pattern, name), text)
    : story === DRAWER_STORY ? DRAWER_PROSE
      : text
