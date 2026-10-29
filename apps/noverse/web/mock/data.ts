// Dữ liệu mẫu mock — lấy từ artboard sample data (Main/Phone/Outline/Login/NewStory).
// Giữ trung thực mô hình: stories/outlines/chapters/facts/reviews/export thật.
import type { ChapterFacts, ChapterPlan, CommandSpec, Frame, RetentionChapter, RetentionCheck, ReviewEntry, ReviewItem, ReviewPack, Snapshot, User } from '../src/api/types'

export const NOW = Date.now()
const iso = (ms: number) => new Date(ms).toISOString()
/** Mốc giờ hôm nay (giờ máy) cho phụ đề thẩm duyệt — artboard ghi “· 21:52”. */
const clockToday = (hour: number, minute: number) => {
  const date = new Date(NOW)
  date.setHours(hour, minute, 0, 0)
  return date.toISOString()
}

// ─── Tài khoản (09-auth-onboard.md) ─────────────────────────────────────
export const users: { user: User; password: string }[] = [
  {
    password: 'mat-khau-123',
    user: {
      username: 'dat', name: 'Dat', role: 'admin', projects: {}, disabled: false,
      createdAt: iso(NOW - 90 * 86400000), updatedAt: iso(NOW - 86400000),
      onboardedAt: iso(NOW - 89 * 86400000), mustChangePassword: false,
      lastSeenAt: iso(NOW - 3600000), sessionCount: 2,
    },
  },
  {
    password: 'vien-12345',
    user: {
      username: 'vien', name: 'Viện', role: 'member', projects: { 'dit-dang-audio': 'collab' }, disabled: false,
      createdAt: iso(NOW - 30 * 86400000), updatedAt: iso(NOW - 86400000),
      onboardedAt: iso(NOW - 29 * 86400000), mustChangePassword: false,
      lastSeenAt: iso(NOW - 7200000), sessionCount: 1,
    },
  },
  {
    password: 'lan-123456',
    user: {
      username: 'lan', name: 'Lan', role: 'member', projects: { 'dit-dang-audio': 'view' }, disabled: false,
      createdAt: iso(NOW - 15 * 86400000), updatedAt: iso(NOW - 86400000),
      onboardedAt: iso(NOW - 15 * 86400000), mustChangePassword: false,
      lastSeenAt: iso(NOW - 86400000), sessionCount: 1,
    },
  },
]
for (const [username, name, projects, temporary, disabled] of [
  ['linh', 'Linh', { 'dit-dang-audio': 'collab' }, false, false],
  ['minh', 'Minh', { 'dit-dang-audio': 'view', 'cu-dan': 'view' }, true, false],
  ['ha', 'Hà', {}, false, true],
] as const) {
  if (users.some((record) => record.user.username === username)) continue
  users.push({ password: 'mat-khau-123', user: {
    username, name, role: 'member', projects: { ...projects }, disabled,
    createdAt: '2026-10-12T10:00:00Z', updatedAt: '2026-10-13T10:00:00Z',
    onboardedAt: iso(NOW), mustChangePassword: temporary, lastSeenAt: username === 'linh' ? iso(NOW - 300000) : null,
    ...(disabled ? { disabledAt: '2026-10-22T10:00:00Z', disabledBy: 'dat', disabledReason: 'Nghỉ cộng tác' } : {}),
  } })
}
// ─── Nội dung chương — Trà Xanh Cướp Kịch Bản (artboard sample) ─────────
const para = (s: string) => s

export const TRAXANH_CHAPTERS: Record<number, { title: string; text: string }> = {
  1: {
    title: 'Hợp đồng giữa phòng thu',
    text: [
      para('Nhà xuất bản Trà Xanh tọa lạc ở tầng bốn của một tòa nhà cũ, nơi thang máy luôn kêu như sắp hết hơi. Kiều My ôm tập kịch bản đầu tay — bốn trăm trang cô viết trong hai năm — và tự nhủ hôm nay là ngày đổi đời.'),
      para('Phòng hợp đồng sáng đèn trắng. Người ngồi đối diện cô tự giới thiệu là Quách, "phụ trách tác quyền", với bộ vest xám và nụ cười chuẩn bị sẵn. Ông ta lật nhanh từng trang, gật đầu từng nhịp — kiểu gật đầu của người đã đọc rồi nhưng muốn thể hiện là mình đang đọc.'),
      para('"Tác giả có quà lưu niệm nào kèm theo không?" Quách hỏi. Kiều My hơi bối rối: cô chỉ mang theo tập kịch bản, một chiếc bút bi và niềm tin rằng chuyện tốt sẽ tự lo phần mình. Bà Hiền, chủ trà cũ bán hàng ở quán ngay cạnh nhà xuất bản, từng khuyên cô: "Con cứ để chữ nó nói, đừng để người ta nói thay chữ con."'),
      para('Hợp đồng đặt trước mặt cô mười bốn trang. Trang mười hai có một mục nhỏ, in cỡ chữ bé hơn, ghi rằng quyền tác giả "được tham gia" quá trình chuyển thể. Khi Kiều My chỉ vào đó, Quách xoay nhẹ chiếc bút: "Đó là mục chuẩn của ngành ạ. Cô yên tâm."'),
      para('Cô ký. Ngòi bút chưa kịp khô thì Quách đã đứng dậy, đút bản ký vào ngăn kéo khóa đến tận năm sau cô mới biết rằng chiếc ngăn đó cũng là nơi chất đầy hợp đồng giống hệt của mình.'),
    ].join('\n\n'),
  },
  2: {
    title: 'Dấu ấn của người biên tập',
    text: [
      para('Nhân Phát gặp Kiều My lần đầu ở hành lang tầng bốn — lúc cô đang bối rối tìm phòng in để photocopy hợp đồng. Anh là người duy nhất trong cả nhà xuất bản hỏi cô "chị ký chưa?" trước khi hỏi "chị viết truyện gì?"'),
      para('Anh đưa cô bản photocopy không tính phí, rồi bấm nút in lại thêm một bản nữa. "Tập đầu là của cô. Bản này của tôi — tôi sẽ đọc."'),
      para('Kiều My không biết rằng Nhân Phát vừa trải qua một năm chữa lại ba cuốn truyện hỏng do tác giả đầu hàng, và đây là lần đầu tiên anh tình nguyện đọc một tập kịch bản không có lệnh của ai.'),
    ].join('\n\n'),
  },
  3: {
    title: 'Phòng quy hoạch kịch bản',
    text: [
      para('Phòng quy hoạch kịch bản nằm ở tầng hai — nơi nhân viên cũ vẫn gọi là "phòng chiến lược" dù trên bảng tên ghi khác. Chín nghìn ba trăm mươi bảy cuốn sách đứng im trên giá, chất kín từ sàn đến trần theo kích thước, màu gáy và độ ăn mòn của keo.'),
      para('Trần Nga làm chủ căn phòng. Với nhân viên mới, bà là "người phụ trách kho"; với giám đốc, bà là "bộ não hậu cần". Còn với bản thân, bà chỉ nói: "Tôi giữ nơi này."'),
      para('Khi Kiều My được Quách dẫn vào tham quan — nghi thức chào mừng tác giả mới — Trần Nga đang kiểm kê bằng mắt. Bà không bấm máy tính; bà chạm gáy từng cuốn và lắc đầu ở hai chục cuốn cuối cùng, rồi ghi một con số vào sổ mà không để ai nhìn thấy.'),
      para('"Nơi này có bao nhiêu cuốn?" Quách hỏi thường trực. "Đủ để công ty đứng," Trần Nga đáp, rồi quay sang Kiều My, "và đủ để một tác giả biết mình đã thua như thế nào."'),
    ].join('\n\n'),
  },
  4: {
    title: 'Bản thảo đầu tay của Trần Nga',
    text: [
      para('Trước khi "Trà Xanh Cướp Kịch Bản" xuất hiện trong mục lục dự án của nhà xuất bản, nó đã sống trong ngăn cuối chiếc tủ tài liệu của Trần Nga suốt mười một năm — lúc đó còn có tên khác: "Tuyển tập hợp đồng thực hành".'),
      para('Bà viết nó bằng tay, tối tối, sau khi lại một ngày chứng kiến ai đó ký đi quyền của chính mình. Bốn mươi một trang, bốn mươi một vụ, mỗi vụ hai trang, đọc như sổ cái kế toán của nỗi nhục.'),
      para('Nhân Phát đánh máy bản gốc ấy. Anh đánh từng chữ như người gõ mã vạch của lời nói dối, và khi đánh đến câu cuối cùng — "không ai trả lời cho sự thật, chỉ có người viết nó xuống" — anh gõ sai một chữ, rồi giữ nguyên.'),
    ].join('\n\n'),
  },
  5: {
    title: 'Giấu tên trong danh sách dự án',
    text: [
      para('Từng phân mục trong "Tuyển tập hợp đồng thực hành" trở thành một vụ án độc lập — một tuyến nhân vật, một phòng khác nhau của tòa nhà Trà Xanh, một cách người ta bị lừa dối khác nhau. Mười hai chương, mười hai lần ai đó tin vào cái bẫy khác màu.'),
      para('Kiều My đọc bản thảo trên tàu từ thành phố về, trong hai chuyến đi đều bị người phụ nữ đối diện nhắc nhở rằng cô đang cười ở câu không nên cười và khóc ở câu không nên khóc.'),
      para('"Tôi không thể tin là người ta bị lừa nhiều thế mà vẫn viết đẹp được," Kiều My nói. "Tôi cũng không tin," Trần Nga đáp, "nên tôi mới phải viết."'),
    ].join('\n\n'),
  },
}

/** Sự kiện lõi từng chương — nội dung thật của mạch truyện (không phải trạng thái tiến độ). */
export const CHAPTER_EVENTS: Record<number, string> = {
  1: 'Kiều My ký hợp đồng độc quyền với phòng thu Trà Xanh, giao lại toàn bộ bản thảo cũ.',
  2: 'Trần Nga nhận ra dấu ấn biên tập của mình trên bản thảo đã bị đổi tên tác giả.',
  3: 'Phòng quy hoạch chốt danh sách mười hai chương mở và gạch tên Kiều My khỏi đó.',
  4: 'Bản thảo đầu tay của Trần Nga xuất hiện trong ngăn kéo của Kiều My.',
  5: 'Kiều My giấu tên mình khỏi danh sách dự án để điều tra nội bộ.',
  6: 'Bữa cơm trưa của người vắng mặt: chiếc ghế trống của Trần Nga bị xóa khỏi lịch.',
  7: 'Cuộc họp cấp trên quyết định chuyển bản quyền cuốn sách sang tên phòng thu.',
  8: 'Kiều My ký tên mình giữa hành lang, trước mặt toàn bộ biên tập viên.',
  9: 'Tuyên bố đẫy bình đẳng: Kiều My công bố mười hai chương mở là của Trần Nga.',
  10: 'Phe bảo vệ tác quyền dựng thế phòng thủ mới quanh hiệu in nhỏ.',
  11: 'Các mã lệnh cũ lần lượt được giải, mở đường tới nghị định xuất bản.',
  12: 'Bản đồ các nước đi cho thấy Quách đứng sau vụ đổi tên tác giả.',
  13: 'Con số bị xóa trong sổ đăng ký bản quyền được phục hồi.',
  14: 'Đêm mở công bố: bản thảo gốc được đọc trước hội đồng.',
}

export const CHAPTER_TITLES: Record<number, string> = {
  1: 'Hợp đồng giữa phòng thu',
  2: 'Dấu ấn của người biên tập',
  3: 'Phòng quy hoạch kịch bản',
  4: 'Bản thảo đầu tay của Trần Nga',
  5: 'Giấu tên trong danh sách dự án',
  6: 'Bữa cơm trưa của người vắng mặt',
  7: 'Cuộc họp cấp trên',
  8: 'Ký tên giữa hành lang',
  9: 'Tuyên bố đẫy bình đẳng',
  10: 'Thế phòng thủ mới',
  11: 'Lần lượt các mã lệnh',
  12: 'Bản đồ các nước đi',
  13: 'Con số bị xóa',
  14: 'Đêm mở công bố',
}

export const plans: Record<number, ChapterPlan> = {
  5: {
    chapter: 5, title: CHAPTER_TITLES[5],
    goal: 'Kết nối bốn mươi một vụ trong bản thảo của Trần Nga với mười hai chương mở của cuốn sách; để Kiều My nhìn ra cấu trúc của cái bẫy.',
    conflict: 'Kiều My muốn ghi danh Trần Nga là tác giả đích thực; Trần Nga yêu cầu giấu tên — đây là lần đầu tiên hai người không đồng ý.',
    hook: 'Trong vụ cuối cùng của bản thảo, tên tác giả bị bỏ trống.',
    emotion_arc: 'xúc động → trầm ngâm → cảnh giác',
    opening_state: 'Kiều My vừa đọc xong bản thảo trên chuyến tàu về; Trần Nga đã đồng ý gặp nhưng giữ nguyên yêu cầu giấu tên.',
    notes: 'Mỗi tuyến nhân vật ứng với một phòng khác nhau của tòa nhà; chương này là cơ sở của hồi hai.',
  },
  6: {
    chapter: 6, title: CHAPTER_TITLES[6],
    goal: 'Mở rộng phe bảo vệ tác quyền qua một bữa trưa đông đủ — ai đến, ai vắng mặt, tại sao.',
    conflict: 'Quách bắt đầu ngờ có bản sao hợp đồng bị rò rỉ ra ngoài.',
    hook: 'Người vắng mặt ở bữa trưa đang ở chính phòng quy hoạch.',
  },
}

export const facts: Record<number, ChapterFacts> = {
  5: {
    title: CHAPTER_TITLES[5],
    summary: 'Kiều My đọc xong bản thảo của Trần Nga, hiểu cấu trúc cuốn sách, đề nghị giấu tên tác giả và chính thức nhận phần viết tiếp cho hồi hai.',
    characters: ['Kiều My', 'Trần Nga', 'Nhân Phát'],
    key_events: [
      'Kiều My đọc bản thảo 41 vụ trên tàu, phản ứng theo nhịp mạch của câu chuyện.',
      'Trần Nga tiết lộ nguồn gốc bản thảo: viết tay trong 11 năm ngầm.',
      'Hai người thống nhất giấu tên tác giả trong danh sách dự án nội bộ.',
    ],
    timeline_events: [{ chapter: 5, time: 'cuối chiều', event: 'Kiều My đọc xong bản thảo trên tàu', characters: ['Kiều My'] }],
    foreshadow_updates: [
      { id: 'fs-ngoac-ky-ten', action: 'advance', description: 'Kiểu ký hợp đồng bị chiếm đoạt lặp lại ở tuyến khác' },
      { id: 'fs-tac-gia-giau-ten', action: 'plant', description: 'Vụ cuối cùng trong bản thảo bỏ trống tên tác giả' },
    ],
    relationship_changes: [{ character_a: 'Kiều My', character_b: 'Trần Nga', relation: 'đồng minh → đồng tác giả', chapter: 5 }],
    state_changes: [
      { character: 'Kiều My', field: 'vai trò', from: 'tác giả mới', to: 'người giữ danh sách' },
      { character: 'Trần Nga', field: 'vị trí', to: 'đồng tác giả ẩn danh' },
    ],
    cast_intros: [{ name: 'bà Hiền', brief_role: 'chủ trà cũ, người cảnh báo' }],
    hook_type: 'điểm hứa', dominant_strand: 'dư âm',
  },
}

export const review5: ReviewEntry = {
  chapter: 5, scope: 'chapter',
  items: [
    {
      id: 'rv5-consistency', level: 'error', ref: 'Master §15', refName: 'Xưng hô, tên, timeline',
      text: 'Sự kiện “tám người đọc bản thảo” mâu thuẫn với chương 3: chỉ Nhân Phát và Trần Nga biết bản thảo tồn tại.',
      quote: 'Tám người đọc xong bản thảo trước khi tàu tới ga.',
      fix: 'Sửa thành “hai người”, hoặc đưa nhân vật thứ ba vào trước chương 3.',
    },
    {
      id: 'rv5-plausibility', level: 'warning', ref: 'Master §2', refName: 'Câu hỏi treo và nhịp',
      text: 'Đọc xong 41 vụ trong một chuyến tàu ~90 phút là hơi gấp so với nhịp mạch trước.',
      quote: 'Chuyến tàu chưa tới ga mà bốn mươi mốt vụ đã khép lại.',
      fix: 'Trải 41 vụ qua hai chuyến tàu, hoặc rút còn mười hai vụ.',
    },
  ],
  round: 1, pack: { id: 'nu-tan-bao-thu', ten: 'Nữ tần báo thù' }, blocking: true, at: clockToday(20, 40), previous: [],
  issues: [
    { type: 'consistency', severity: 'error', description: 'Sự kiện "tám người đọc bản thảo" mâu thuẫn với chương 3 (chỉ Nhân Phát và Trần Nga biết tồn tại)', evidence: 'Đoạn 2, câu 14', suggestion: 'Sửa thành "hai người" hoặc đưa nhân vật thứ ba vào trước', requires_change: true },
    { type: 'plausibility', severity: 'warning', description: 'Đọc xong 41 vụ trong một chuyến tàu ~90 phút là hơi gấp so với nhịp mạch trước', requires_change: false },
  ],
  // 7 chiều theo internal/domain/review.go — thang 0–100
  dimensions: [
    { dimension: 'consistency', score: 85, comment: 'Một mâu thuẫn "tám người" cần sửa' },
    { dimension: 'character', score: 82, comment: 'Kiều My giữ giọng, Trần Nga còn mờ' },
    { dimension: 'pacing', score: 78, comment: 'Chuyến tàu 90 phút hơi gấp' },
    { dimension: 'continuity', score: 84, comment: 'Khớp chương 3–4' },
    { dimension: 'foreshadow', score: 80, comment: 'Vỏ ổ cứng đã cài từ chương 2' },
    { dimension: 'hook', score: 76, comment: 'Câu kết chưa đủ bất ngờ' },
    { dimension: 'aesthetic', score: 81, comment: 'Câu dài, nhịp đều' },
  ],
  contract_status: 'partial',
  contract_misses: ['Điểm "vụ cuối bỏ trống tên" chưa gợi lại đủ rõ'],
  verdict: 'polish',
  summary: 'Chương đạt cấu trúc nhưng có một lỗi nhất quán cần sửa trước khi duyệt.',
}

const PACK_NU_TAN: ReviewPack = { id: 'nu-tan-bao-thu', ten: 'Nữ tần báo thù' }

/**
 * Câu 4 — vòng chấm đầu tiên phát `rule`: Master §19 (`s19-recap`, nhắc chuyện cũ quá 2 câu) ở chương 6
 * của truyện dùng chung “Đài truyền hình Vân Châu”. Chỉ vòng này mang `rule_aware`; mọi vòng khác để trống
 * = “chưa kiểm được”, nên cả nhánh đã kiểm và nhánh chưa kiểm đều nhìn thấy được trong mock.
 */
export const review6RuleAware: ReviewEntry = {
  chapter: 6, scope: 'chapter', issues: [], round: 1, pack: PACK_NU_TAN, blocking: false, at: clockToday(22, 4), previous: [],
  rule_aware: true, verdict: 'polish', summary: 'Một chỗ nhắc chuyện cũ quá dài; phần còn lại đạt.',
  items: [{
    id: 's19-recap-tra-xanh-6', level: 'warning', ref: 'Master §19', refName: 'Nhắc chuyện cũ',
    rule: 's19-recap',
    text: 'Đoạn mở chương nhắc lại 4 câu về buổi ghi hình cũ; Master §19 cho tối đa 2 câu, lồng vào hành động.',
    quote: 'Tôi nhớ lại buổi ghi hình hôm ấy, cái ống kính, ánh đèn, tiếng đạo diễn hô, và cả câu nói của Tri Hạ.',
    fix: 'Giữ một câu hồi tưởng, chuyển phần còn lại thành hành động đang diễn ra.',
  }],
}

/** Mục của lần chấm 2 chương 3 (Reader.dc.html › tab=review): 1 bắt sửa + 2 cảnh báo. */
const ch3Items: ReviewItem[] = [
  {
    id: 'ch3-hook', level: 'error', ref: 'Master §7', refName: 'Đầu chương và cuối chương',
    text: 'Chương kết ở trạng thái an toàn: Tri Hạ về nhà, không còn câu hỏi nào treo.',
    quote: 'Tôi tắt đèn, kéo chăn lên, lần đầu sau nhiều ngày ngủ một giấc yên.',
    fix: 'Kết bằng tin nhắn từ số lạ, hoặc một chi tiết chưa giải.',
  },
  {
    id: 'ch3-dialogue', level: 'warning', ref: 'Master §13', refName: 'Văn phong, thoại, hài',
    text: 'Thoại chiếm 34%, chuẩn của gói là 40–55%.',
    quote: 'Tôi kể lại cho anh nghe chuyện buổi thử vai, từ lúc bước vào phòng đến khi đạo diễn gật đầu…',
    fix: 'Chuyển đoạn kể buổi thử vai thành đối thoại với đạo diễn.',
  },
  {
    id: 'ch3-villain', level: 'warning', ref: 'Bộ A §4.3', refName: 'Trà xanh và phản diện',
    text: 'Nhược Vy thắng ván nhỏ nhưng chưa đủ để người nghe ghét.',
    quote: 'Nhược Vy chỉ cười, rồi đi thẳng ra cửa.',
    fix: 'Cho Nhược Vy một câu thoại đắc thắng trước mặt người khác.',
  },
]

/**
 * Dòng “Thẩm chương 3 · 2 bắt sửa · 3 cảnh báo” ở Nhật ký (Activity.dc.html › nhóm
 * “Thẩm duyệt, đo bằng máy & xưng hô”): lỗi lint Xưng hô mức Bắt sửa chặn duyệt y như
 * Master §7; cảnh báo cùng nguồn xưng hô vẫn giữ nguyên được.
 */
export const reviewEvent3Items: ReviewItem[] = [
  {
    id: 'ch3-hook', level: 'error', ref: 'Master §7', refName: 'Đầu chương và cuối chương',
    text: 'Chương kết ở trạng thái an toàn, không còn câu hỏi nào treo.',
    quote: 'Tôi tắt đèn, kéo chăn lên, lần đầu sau nhiều ngày ngủ một giấc yên.',
    fix: 'Kết bằng tin nhắn từ số lạ, hoặc một chi tiết chưa giải.',
  },
  {
    id: 'ch3-xh-direction', level: 'error', ref: 'Xưng hô', refName: 'Tri Hạ → Nhược Vy',
    text: 'Tri Hạ gọi Nhược Vy là “em”. Từ chương 2 đã đổi thành “cô” vì hai người cắt đứt quan hệ.',
    quote: 'Em về trước đi, chuyện này để chị lo.',
    fix: 'Đổi “em” thành “cô” ở câu có cách gọi đó.',
  },
  {
    id: 'ch3-xh-table', level: 'warning', ref: 'Xưng hô', refName: 'Bảng xưng hô của truyện',
    text: 'Kỳ Dương gọi Tri Hạ là “cậu”, bảng xưng hô ghi “chị” từ chương 2.',
    quote: 'Cậu về trước đi, để tớ lo.',
    fix: 'Sửa câu có “cậu” thành “chị”, hoặc ghi lý do giữ nguyên.',
  },
  {
    id: 'ch3-dialogue', level: 'warning', ref: 'Master §13', refName: 'Văn phong, thoại, hài',
    text: 'Thoại chiếm 34%, chuẩn của gói là 40–55%.',
    quote: 'Tôi kể lại cho anh nghe chuyện buổi thử vai, từ lúc bước vào phòng đến khi đạo diễn gật đầu…',
    fix: 'Chuyển đoạn kể buổi thử vai thành đối thoại với đạo diễn.',
  },
  {
    id: 'ch3-your-rule', level: 'warning', ref: 'Quy tắc của bạn', refName: 'Giới hạn từ lặp',
    text: '“khẽ” xuất hiện 3,1 lần trên 1.000 tiếng, quy tắc cho tối đa 2.',
    quote: 'Cô khẽ cười, khẽ đặt chén xuống, rồi khẽ nói một câu.',
    fix: 'Bỏ hai trong ba chữ “khẽ” ở đoạn này.',
  },
]

/** 1 bắt sửa + 2 cảnh báo → chương vào hàng chờ sửa, cửa duyệt đóng, /next trả 409. */
export const review3Blocking: ReviewEntry = {
  chapter: 3, scope: 'chapter', issues: [], items: ch3Items, round: 2, pack: PACK_NU_TAN, blocking: true,
  at: clockToday(21, 52), previous: [],
  dimensions: [
    { dimension: 'consistency', score: 86, comment: 'Khớp chương 1–2' },
    { dimension: 'character', score: 83, comment: 'Tri Hạ giữ giọng' },
    { dimension: 'pacing', score: 79, comment: 'Nhịp chậm ở cảnh hai' },
    { dimension: 'continuity', score: 85, comment: 'Liền mạch' },
    { dimension: 'foreshadow', score: 82, comment: 'Vỏ ổ cứng đã cài' },
    { dimension: 'hook', score: 58, comment: 'Kết an toàn, không treo câu hỏi' },
    { dimension: 'aesthetic', score: 84, comment: 'Câu ngắn, nhịp đều' },
  ],
  contract_status: 'partial',
  contract_misses: ['Móc cuối chương chưa đạt Master §7'],
  verdict: 'polish',
  summary: 'Chương đạt cấu trúc nhưng móc cuối chương an toàn nên cần gọt lại trước khi duyệt.',
}

/** Lần chấm 2 đã sạch mục bắt sửa (Main.dc.html › phase=gate): còn 2 cảnh báo. */
export const review3Clean: ReviewEntry = {
  chapter: 3, scope: 'chapter', issues: [], items: ch3Items.filter((item) => item.level !== 'error'),
  round: 2, pack: PACK_NU_TAN, blocking: false, at: clockToday(21, 58),
  previous: [{
    round: 1, verdict: 'polish', rule_aware: false,
    items: [{
      id: 'ch3-hook-r1', level: 'error', ref: 'Master §7', refName: 'Đầu chương và cuối chương',
      text: 'chương kết ở trạng thái an toàn', quote: 'Tôi tắt đèn, kéo chăn lên, lần đầu sau nhiều ngày ngủ một giấc yên.',
      fix: 'Kết bằng tin nhắn từ số lạ, hoặc một chi tiết chưa giải.',
    }],
  }],
  dimensions: [
    { dimension: 'consistency', score: 88, comment: 'Khớp chương 1–2' },
    { dimension: 'character', score: 85, comment: 'Tri Hạ giữ giọng' },
    { dimension: 'pacing', score: 81, comment: 'Nhịp chậm ở cảnh hai' },
    { dimension: 'continuity', score: 87, comment: 'Liền mạch' },
    { dimension: 'foreshadow', score: 84, comment: 'Vỏ ổ cứng đã cài' },
    { dimension: 'hook', score: 86, comment: 'Kết bằng tin nhắn từ số lạ' },
    { dimension: 'aesthetic', score: 85, comment: 'Câu ngắn, nhịp đều' },
  ],
  contract_status: 'met',
  verdict: 'accept',
  summary: 'Chương đã qua thẩm duyệt; còn hai cảnh báo không chặn duyệt.',
}

/** Cửa duyệt còn mục bắt sửa (Activity › “Duyệt chương 9” → 409). */
export const review9Blocking: ReviewEntry = {
  chapter: 9, scope: 'chapter', issues: [], round: 1, pack: PACK_NU_TAN, blocking: true, at: clockToday(22, 10),
  previous: [],
  items: [ch3Items[0]!],
  verdict: 'rewrite',
  summary: 'Móc cuối chương chưa đạt nên chương chưa duyệt được.',
}

export const foreshadow = [
  { id: 'fs-ngoac-ky-ten', description: 'Kiểu ký hợp đồng bị chiếm đoạt lặp lại ở tuyến khác — Quách xoay bút khi ai chỉ vào mục nhỏ', planted_at: 1, status: 'advanced' },
  { id: 'fs-phong-chien-luoc', description: 'Phòng quy hoạch ghi nhiều hơn mức cần: ai đã vào kho, đọc những gì, ký những gì', planted_at: 3, status: 'planted', due_by: 8 },
  { id: 'fs-ban-thao-tran-nga', description: 'Bản thảo tay của Trần Nga — người giữ chìa khóa phòng đã từng bị đánh cắp', planted_at: 4, status: 'planted' },
  { id: 'fs-tac-gia-giau-ten', description: 'Vụ cuối cùng trong bản thảo bỏ trống tên tác giả', planted_at: 5, status: 'planted' },
  { id: 'fs-tuyen-bo-binh-dang', description: '"Đẩy bình đẳng" — lời nói ngược dòng ở chương 9 đang được chuẩn bị từ chương 6', planted_at: 6, status: 'planted' },
  { id: 'fs-chiec-o-xanh', description: 'Chiếc ô xanh bỏ quên ở sảnh', planted_at: 2, status: 'abandoned', abandoned_at: 7, abandon_reason: 'Tác giả bỏ tuyến khi đổi nhân vật giữ chìa khóa' },
]

// ─── Snapshot mẫu (06-data-contract) ─────────────────────────────────────
export const snapshotTraXanh: Snapshot = {
  provider: 'codex', bookTitle: 'Trà Xanh Cướp Kịch Bản',
  modelName: 'gpt-5.4', modelContextWindow: 320000, thinkingLevel: 'high', style: 'satira',
  runtimeState: 'running', statusLabel: '写第 6 章', phase: 'writing', flow: 'writing',
  waitingFor: '',
  currentChapter: 6, totalChapters: 14, completedCount: 5, totalWordCount: 11840,
  inProgressChapter: 6, pendingRewrites: null, rewriteReason: '', pendingSteer: '',
  advanceMode: 'auto', advancePermitChapter: 0, hasAdvanceHold: false, advanceHoldReason: '',
  recoveryLabel: '', isRunning: true,
  agents: [
    {
      name: 'writer', state: 'running', taskId: 'task-6-write', taskKind: 'write',
      summary: 'Viết chương 6: Bữa cơm trưa của người vắng mặt', tool: 'edit_chapter', turn: 7,
      context: { tokens: 41200, contextWindow: 320000, percent: 12.9, scope: 'projected', strategy: 'none', activeMessages: 38, summaryMessages: 0, compactedCount: 0, keptCount: 38 },
      updatedAt: iso(NOW - 4000),
    },
    {
      name: 'editor', state: 'idle', taskId: '', taskKind: '', summary: 'Chờ lệnh từ điều phối', tool: '', turn: 0,
      context: { tokens: 0, contextWindow: 320000, percent: 0, scope: 'baseline', strategy: 'none', activeMessages: 0, summaryMessages: 0, compactedCount: 0, keptCount: 0 },
      updatedAt: iso(NOW - 60000),
    },
    {
      name: 'architect', state: 'standby', taskId: '', taskKind: '', summary: 'Gọi khi cần', tool: '', turn: 0,
      context: { tokens: 0, contextWindow: 320000, percent: 0, scope: 'baseline', strategy: 'none', activeMessages: 0, summaryMessages: 0, compactedCount: 0, keptCount: 0 },
      updatedAt: iso(NOW - 60000),
    },
    {
      name: 'arbiter', state: 'running', taskId: '', taskKind: 'arbitration',
      summary: '用户干预裁定', tool: 'arbitrate', turn: 1,
      context: { tokens: 8300, contextWindow: 320000, percent: 2.6, scope: 'baseline', strategy: 'none', activeMessages: 6, summaryMessages: 0, compactedCount: 0, keptCount: 6 },
      updatedAt: iso(NOW - 30000),
    },
  ],
  totalInputTokens: 284000, totalOutputTokens: 61200, totalCacheReadTokens: 198000, totalCacheWriteTokens: 86000,
  totalCostUSD: 8.42, totalSavedUSD: 21.7, budgetLimitUSD: 0,
  overallCacheCapable: true, overallRecentCacheRead: 18000, overallRecentInput: 21000, overallRecentSamples: 12,
  totalCacheBreaks: 2, missingAssistantUsage: 0,
  cachePerAgent: [
    { role: 'writer', model: 'gpt-5.4', input: 180000, output: 42000, cacheRead: 132000, cacheWrite: 51000, cost: 5.9, saved: 14.9, cacheCapable: true, recentCacheRead: 12000, recentInput: 15000, recentSamples: 12 },
    { role: 'editor', model: 'gpt-5.4', input: 72000, output: 14200, cacheRead: 49000, cacheWrite: 23000, cost: 1.8, saved: 4.9, cacheCapable: true, recentCacheRead: 4000, recentInput: 5000, recentSamples: 8 },
    { role: 'architect', model: 'o4-mini', input: 24000, output: 3800, cacheRead: 12000, cacheWrite: 9000, cost: 0.5, saved: 1.1, cacheCapable: true, recentCacheRead: 2000, recentInput: 1000, recentSamples: 4 },
    { role: 'arbiter', model: 'o4-mini', input: 8000, output: 1200, cacheRead: 5000, cacheWrite: 3000, cost: 0.22, saved: 0.8, cacheCapable: true, recentCacheRead: 0, recentInput: 0, recentSamples: 0 },
  ],
  cachePerModel: [
    { role: '', model: 'gpt-5.4', input: 252000, output: 56200, cacheRead: 181000, cacheWrite: 74000, cost: 7.7, saved: 19.8, cacheCapable: true, recentCacheRead: 16000, recentInput: 20000, recentSamples: 20 },
    { role: '', model: 'o4-mini', input: 32000, output: 5000, cacheRead: 17000, cacheWrite: 12000, cost: 0.72, saved: 1.9, cacheCapable: true, recentCacheRead: 2000, recentInput: 1000, recentSamples: 4 },
  ],
  synopsis: 'Câu chuyện đô thị về nhà xuất bản Trà Xanh — nơi mỗi bản ký kết là một lát cắt của quyền lực.',
  premise: 'Một tác giả mới ký sai hợp đồng và phải đoạt lại tác quyền bằng chính chữ viết của mình.',
  outline: Object.entries(CHAPTER_TITLES).map(([n, title]) => ({ chapter: Number(n), title, coreEvent: CHAPTER_EVENTS[Number(n)] ?? '' })),
  // Dữ liệu cũ kiểu “Tên (vai; …)” như serve thật — FE tách qua splitCastLabel (04 › dòng tên).
  characters: [
    'Kiều My (tác giả mới; người ký sai hợp đồng)', 'Trần Nga', 'Nhân Phát', 'Quách', 'bà Hiền',
    'Luật sư Hàn (cố vấn pháp lý; giữ bản ký kết)', 'Cô lao công Lụa', 'Bà Tư (chủ nhà trọ)',
  ],
  supportingCount: 3, recentSupporting: ['biên tập viên Minh (bạn đồng nghiệp)', 'giám đốc Hùng', 'thợ bảo vệ tòa nhà'],
  layered: true, currentVolumeArc: 'Hồi 2 · phe bảo vệ tác quyền đang lớn lên',
  nextVolumeTitle: 'Tập 2 · Tuyên bố của người vắng mặt',
  compassDirection: 'tập trung vào việc đoạt lại tác quyền', compassScale: 'mạch tiểu thuyết đô thị',
  // Chuỗi host thật (snapshot serve phát tiếng Trung) — UI phải dịch qua translateHost.
  lastCommitSummary: '第3章 7406字', lastReviewSummary: 'verdict=polish 1个问题',
  lastCheckpointName: 'chapter:3.commit', recentSummaries: ['第3章: phòng quy hoạch 9.337 cuốn', '第2章: Nhân Phát xuất hiện', '第1章: hợp đồng giữa phòng thu'],
}

// ─── Registry lệnh — GET /api/commands (13-commands.md › GAP-23/G11) ─────
// Bản mock của registry Go (internal/commands): name không có '/', usage giữ '/',
// group là khoá tiếng Anh, prose tiếng Việt nằm ở web/src/i18n/commands.vi.ts.
export const COMMANDS: CommandSpec[] = [
  { name: 'help', aliases: [], usage: '/help', group: 'sys', autoExecute: true, needsIdle: false, params: [] },
  {
    name: 'model', aliases: [], usage: '/model [vai-trò]', group: 'sys', autoExecute: true, needsIdle: false,
    params: [{
      key: 'role', token: '[vai-trò]', kind: 'enum', required: false,
      options: [
        { value: 'architect', note: 'Kiến trúc sư — lên dàn ý, quy hoạch tập' },
        { value: 'writer', note: 'Người viết — viết từng chương' },
        { value: 'editor', note: 'Biên tập viên — soát và chấm chương' },
      ],
    }],
  },
  { name: 'config', aliases: [], usage: '/config', group: 'sys', autoExecute: true, needsIdle: false, params: [] },
  { name: 'diag', aliases: [], usage: '/diag', group: 'write', autoExecute: true, needsIdle: false, params: [] },
  {
    name: 'review', aliases: [], usage: '/review on|off', group: 'write', autoExecute: false, needsIdle: false,
    params: [{
      key: 'mode', token: 'on|off', kind: 'enum', required: true,
      options: [
        { value: 'on', note: 'Dừng sau mỗi chương, chờ /next' },
        { value: 'off', note: 'Viết liền, không dừng' },
      ],
    }],
  },
  { name: 'next', aliases: [], usage: '/next', group: 'write', autoExecute: true, needsIdle: true, params: [] },
  {
    name: 'start', aliases: [], usage: '/start <đường-dẫn-tệp>', group: 'begin', autoExecute: false, needsIdle: false,
    only: 'welcome', params: [{ key: 'path', token: '<đường-dẫn-tệp>', kind: 'path', required: true }],
  },
  {
    name: 'import', aliases: [],
    usage: '/import <đường-dẫn> [--yes] [--story=open|closed] [--continue] [--guide=<hướng-dẫn>]',
    group: 'begin', autoExecute: false, needsIdle: true,
    params: [
      { key: 'path', token: '<đường-dẫn>', kind: 'path', required: false },
      { key: 'yes', token: '--yes', kind: 'flag', required: false, flag: '--yes' },
      {
        key: 'story', token: '--story=open|closed', kind: 'enum', required: false, prefix: '--story=',
        options: [
          { value: 'open', note: 'Truyện còn dang dở, sẽ viết tiếp' },
          { value: 'closed', note: 'Truyện đã có kết thúc' },
        ],
      },
      { key: 'cont', token: '--continue', kind: 'flag', required: false, flag: '--continue' },
      { key: 'guide', token: '--guide=<hướng-dẫn>', kind: 'text', required: false, prefix: '--guide=' },
    ],
  },
  {
    name: 'reopen', aliases: [], usage: '/reopen [hướng-viết-tiếp]', group: 'end', autoExecute: false, needsIdle: true,
    params: [{ key: 'dir', token: '[hướng-viết-tiếp]', kind: 'text', required: false }],
  },
  {
    name: 'cocreate', aliases: ['plan'], usage: '/cocreate', group: 'write', autoExecute: true, needsIdle: false,
    only: 'running', params: [],
  },
  { name: 'simulate', aliases: [], usage: '/simulate', group: 'style', autoExecute: false, needsIdle: true, params: [] },
  {
    name: 'importsim', aliases: [], usage: '/importsim <tệp-profile.json>', group: 'style', autoExecute: false, needsIdle: true,
    params: [{ key: 'path', token: '<tệp-profile.json>', kind: 'path', required: true }],
  },
  {
    name: 'sync', aliases: [], usage: '/sync [--check]', group: 'write', autoExecute: true, needsIdle: true,
    params: [{ key: 'check', token: '[--check]', kind: 'flag', required: false, flag: '--check' }],
  },
  {
    name: 'export', aliases: [],
    usage: '/export [đường-dẫn|txt|epub|narration] [from=N] [to=M] [--overwrite|--keep] [doc-so]', group: 'end',
    autoExecute: true, needsIdle: false,
    params: [
      { key: 'path', token: '[đường-dẫn]', kind: 'path', required: false },
      { key: 'from', token: 'from=N', kind: 'num', required: false, prefix: 'from=' },
      { key: 'to', token: 'to=M', kind: 'num', required: false, prefix: 'to=' },
      {
        key: 'format', token: 'format=txt|epub|narration', kind: 'enum', required: false, prefix: 'format=',
        options: [
          { value: 'txt', note: '' },
          { value: 'epub', note: '' },
          { value: 'narration', note: 'Bản đọc sạch cho TTS (không tiêu đề, không Markdown)' },
        ],
      },
      { key: 'ow', token: '--overwrite', kind: 'flag', required: false, flag: '--overwrite' },
      { key: 'keep', token: '--keep', kind: 'flag', required: false, flag: '--keep' },
      { key: 'docso', token: 'doc-so', kind: 'flag', required: false, flag: 'doc-so' },
    ],
  },
]

// M2 · mêmes paragraphes que MReader.dc.html, sans modifier les scénarios desktop.
export const MOBILE_READER_PARAGRAPHS = [
  'Đèn trường quay bật lên cùng lúc, trắng đến mức tôi phải nheo mắt. Trên màn hình lớn phía sau sân khấu, tên tôi chạy thành một dòng chữ đỏ — đúng cái tên mà ba tháng trước người ta đã gạch khỏi kịch bản.',
  'Nhược Vy đứng cách tôi hai bước, nụ cười vẫn còn nguyên, nhưng ngón tay cô đang siết chặt mép tờ giấy. Tôi biết tờ giấy đó. Tôi đã viết nó vào một đêm mất điện, bằng cây bút chì cùn duy nhất còn lại trong nhà.',
  '“Mời khách mời đọc đoạn mở đầu,” người dẫn chương trình nói. Không ai nhìn tôi. Mọi máy quay đều đã quay về phía cô ấy.',
  'Tôi bước lên, cầm lấy micro, và đọc — không phải bản trên tay cô, mà bản tôi vẫn thuộc lòng từng dấu phẩy.',
  'Khán phòng im lặng đến mức tôi nghe được tiếng chiếc máy quay số ba xoay ống kính. Ở hàng ghế đầu, đạo diễn Phùng đặt cốc trà xuống, chậm rãi, như thể sợ làm vỡ khoảnh khắc này.',
  'Nhược Vy cười, rất khẽ. “Chị thuộc kịch bản của em kỹ thật đấy.” Câu nói nhẹ như gió, nhưng tôi thấy cổ tay cô run lên một nhịp.',
  'Tôi không trả lời. Tôi chỉ lật trang cuối của bản gốc — trang mà chưa ai từng thấy — và đọc tiếp.',
]
export const MOBILE_READER_CHAPTERS = Object.fromEntries(Array.from({ length: 24 }, (_, index) => {
  const chapter = index + 1
  const titles = ['Vai của tôi là đồ canh', 'Chiếc cặp ở giữa bàn', 'Hai chiếc bát trên bàn', 'Bảy mươi ba giây', 'Tôi cướp vai cái nồi à?', 'Chiếc vỏ ổ cứng', 'Dòng tên bị gạch', 'Tên tôi trên màn hình']
  return [chapter, { title: titles[chapter - 17] ?? `Chương ${chapter}`, text: MOBILE_READER_PARAGRAPHS.join('\n\n') }]
}))

// ─── D5 — Khung truyện & bản đồ giữ chân (Outline.dc.html › tab outline · omap) ────
// Bảng Master §1.2 lấy nguyên từ assets/packs/nu-tan-bao-thu/goi.json (gói của truyện mẫu).
export const MOCK_AUDIENCES: Frame['audiences'] = [
  { n: 0, label: 'Phụ nữ trung niên, nghe khi làm việc nhà', wants: 'Thương nhân vật, rồi hả dạ khi kẻ ác bị quả báo' },
  { n: 1, label: 'Nữ 18–35', wants: 'Được làm người thông minh, sắc sảo, thắng đẹp' },
  { n: 2, label: 'Nam trung niên', wants: 'Làm lại đời, gánh vác gia đình, hoài niệm' },
  { n: 3, label: 'Nam trẻ', wants: 'Từ yếu thành mạnh, nâng cấp liên tục' },
  { n: 4, label: 'Người cao tuổi', wants: 'Được thấu hiểu, thấy đời mình trong truyện' },
]

/** Trạng thái đầu của khung truyện: gói thể loại điền sẵn (chưa ai sửa trên web). */
export const MOCK_FRAME: Frame = {
  audience: [1],
  difference: 'Nghề lồng tiếng và trò đọc thoại live; phản đòn bằng bản ghi âm gốc thay vì bằng chứng giấy.',
  sequelSeed: 'Ai đã gửi lại kịch bản gốc cho Tri Hạ, không ký tên?',
  source: 'pack',
  audiences: MOCK_AUDIENCES,
}

/** Một dòng bảng mẫu §21 của artboard — chương, câu hỏi, lời hứa, trả thưởng, móc cuối chương. */
export type MockRetentionRow = Pick<RetentionChapter, 'n' | 'questions' | 'promise' | 'payoff' | 'hook'>

export const MOCK_RETENTION_ROWS: MockRetentionRow[] = [
  {
    n: 1, questions: ['Ai đổi tên tác giả trên kịch bản?'],
    promise: 'Tri Hạ sẽ đòi lại tên mình trên sóng (lời hứa của tiêu đề)',
    payoff: { tag: '', text: 'Áp lực: bị xếp vai “đồ canh” trong chính kịch bản của mình' },
    hook: { type: 'line', waiting: 'Nhược Vy sẽ đáp lại câu “Kịch bản đó là của tôi” thế nào?' },
  },
  {
    n: 2, questions: ['Ai đổi tên tác giả?', 'Chiếc cặp giữa bàn đựng gì?'],
    promise: 'Đếm ngược: buổi đọc thoại live sau 3 ngày',
    payoff: { tag: '', text: 'Tri Hạ chỉ ra lỗi thoại của Nhược Vy trước cả đoàn' },
    hook: { type: 'decision', waiting: 'Nhận lời lên show, Tri Hạ định làm gì?' },
  },
  {
    n: 3, questions: ['Chiếc cặp đựng gì?', 'Ai đứng sau Nhược Vy?'],
    promise: 'Đếm ngược: live còn 1 ngày',
    payoff: { tag: '', text: 'Bác thẻ Nhược Vy nhường, lấy lại lượt đúng luật' },
    hook: { type: 'threat', waiting: 'Số lạ là ai, và họ biết gì?' },
  },
  {
    n: 4, questions: ['Số lạ là ai?', '73 giây bị cắt có gì?', 'Ai đứng sau Nhược Vy?'],
    promise: 'Cảnh lời hứa: nối đúng câu bị ngắt ngay trên live',
    payoff: { tag: 'promise', text: 'Vả mặt trên sóng: Nhược Vy không đọc nổi đoạn mình “viết”' },
    hook: { type: 'info', waiting: 'Đoạn 73 giây bị cắt là giọng đọc gốc của Tri Hạ' },
  },
  {
    n: 5, questions: ['Ai đứng sau?', 'Vỏ ổ cứng ở đâu?'],
    promise: 'Lời hứa thứ hai: bản gốc trong ổ cứng sẽ lộ ở tập cuối',
    payoff: { tag: 'midpoint', text: 'Phùng Chấn là người đặt hàng vụ cướp kịch bản' },
    hook: { type: 'threat', waiting: 'Phùng Chấn cho người tới nhà Tri Hạ' },
  },
  {
    n: 6, questions: ['Vỏ ổ cứng ở đâu?', 'Hoài Châu đứng về phía ai?'],
    promise: 'Đếm ngược: tập cuối phát sóng sau 5 ngày',
    payoff: { tag: '', text: 'Tìm được vỏ ổ cứng, nhưng ruột đã bị tháo' },
    hook: { type: 'threat', waiting: 'Ai đã tháo ruột ổ cứng?' },
  },
  {
    n: 7, questions: ['Hoài Châu đứng về phía ai?', 'Tên trong hồ sơ học bổng là ai?'],
    promise: 'Đếm ngược: tập cuối phát sóng ngày mai',
    payoff: { tag: '', text: 'Lộ dòng tên bị gạch trong hồ sơ học bổng cũ' },
    hook: { type: 'threat', waiting: 'Nhược Vy đổ hết cho Tri Hạ trên báo' },
  },
  {
    n: 8, questions: ['—'], promise: 'Trả lời mọi câu hỏi đã mở',
    payoff: { tag: 'climax', text: 'Tên Tri Hạ lên màn hình; Nhược Vy tự nhận trước khán giả' },
    // Câu 19: chương cuối có `hook.type = "seed"` (Hạt giống phần hai), vẫn có `waiting`.
    hook: { type: 'seed', waiting: 'Chương cuối · gieo hạt giống phần hai' },
  },
]

/** Bốn dòng kiểm của artboard (máy chủ thật tính ở retention_api.go; mock chép nguyên câu). */
export const MOCK_RETENTION_CHECKS: RetentionCheck[] = [
  { ref: 'Master §3', ok: true, text: 'Cảnh lời hứa (Tri Hạ nối đúng câu bị ngắt trên live) nằm ở 44%, trong vùng 35–60%.', chapters: [] },
  { ref: 'Master §8', ok: true, text: 'Cú lật giữa truyện (Phùng Chấn đặt hàng vụ cướp kịch bản) nằm ở 56%, trong vùng 45–60%.', chapters: [] },
  { ref: 'Master §7', ok: false, text: 'Chương 5, 6 và 7 cùng kết bằng “Đe dọa mới”. Đổi chương 6 sang kiểu móc khác.', chapters: [5, 6, 7] },
  { ref: 'Master §2', ok: false, text: 'Chương 1 chỉ có 1 câu hỏi đang treo, nên có 2–4.', chapters: [1] },
]

/** Vị trí 0..1 của cảnh lời hứa và cú lật trong artboard (44% và 56%). */
export const MOCK_RETENTION_AT = { promiseAt: 0.44, twistAt: 0.5625 }
