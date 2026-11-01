// Mọi chữ của Cài đặt › Kết nối AI và Cài đặt › Trợ lý AI — copy lấy nguyên
// văn từ artboard Settings + MSettings (06-copy.md). Giá trị sống (số model,
// giờ, hạn mức, tên model) là tham số, không viết cứng trong component.

/** Thứ tự năng lực của một pill: Chữ · đọc ảnh · Vẽ ảnh. */
export function capsLine(caps: readonly string[]): string {
  const text = caps.includes('text')
  const vision = caps.includes('vision')
  const image = caps.includes('image')
  if (image) return text ? `${aiCopy.connect.caps.text} · ${aiCopy.connect.caps.image}` : aiCopy.connect.caps.image
  if (vision) return text ? `${aiCopy.connect.caps.text} · ${aiCopy.connect.caps.vision}` : aiCopy.connect.caps.vision
  return aiCopy.connect.caps.text
}

/** Câu năng lực cho dòng phụ của thẻ trên điện thoại (“chữ và vẽ ảnh”). */
export function capsPhrase(caps: readonly string[]): string {
  const text = caps.includes('text')
  const vision = caps.includes('vision')
  const image = caps.includes('image')
  if (image && text) return 'chữ và vẽ ảnh'
  if (image) return 'chỉ vẽ ảnh'
  if (vision && text) return 'chữ và đọc ảnh'
  if (vision) return 'đọc ảnh'
  return 'chỉ chữ'
}

const WEEKDAY = ['', 'Hai', 'Ba', 'Tư', 'Năm', 'Sáu', 'Bảy', 'CN']

/** Giờ địa phương “HH:mm” của một mốc ISO. */
export function clockOf(iso: string | undefined | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

export const aiCopy = {
  connect: {
    title: 'Kết nối AI',
    /** Câu dẫn của artboard MSettings › Kết nối AI (khác câu máy tính). */
    mobileLede: 'Tài khoản và khoá dùng chung cho cả máy. Đăng nhập gói đang trả hoặc dán khoá API; model lấy thẳng từ nhà cung cấp.',
    loading: 'Đang tải kết nối…',
    offline: 'Mất kết nối máy chủ — mọi nút trong mục này tắt cho tới khi kết nối lại.',
    empty: 'Chưa có nhà cung cấp nào — chọn một nhà cung cấp bên dưới.',
    loadFail: 'Không tải được danh sách nhà cung cấp.',

    // Thẻ nhà cung cấp — nhãn loại kết nối.
    kind: { subscription: 'Đăng nhập gói', api_key: 'Khoá API', local: 'Trên máy này' },
    kindShort: { subscription: 'Đăng nhập gói', api_key: 'Khoá API', local: 'Trên máy' },
    // Artboard 04-components §ModelCatalog: “Chữ · đọc ảnh” — chữ sau dấu
    // chấm giữa viết thường, riêng pill chỉ-vẽ vẫn là “Vẽ ảnh”.
    caps: { text: 'Chữ', vision: 'đọc ảnh', image: 'Vẽ ảnh' },

    // Dòng phụ (desktop).
    /** `planPhrase`: “gói Plus” (ChatGPT); Grok không ghi gói — artboard. */
    subSignedIn: (models: number, checkedAt: string, planPhrase: string) =>
      `Đã đăng nhập${planPhrase ? ` · ${planPhrase}` : ''} · ${models} model đã chọn${checkedAt ? ` · kiểm tra lúc ${checkedAt}` : ''}`,
    subLoggedOut: (pkg: string) => `Chưa đăng nhập · cần gói ${pkg}`,
    pkg: { chatgpt: 'ChatGPT Plus hoặc Pro', grok: 'SuperGrok hoặc X Premium' },
    pkgFallback: 'trả phí',
    subPending: (where: string) => `Đang đăng nhập · chờ bạn ${where}`,
    pendingWhere: { chatgpt: 'xác nhận trong trình duyệt', grok: 'nhập mã trên trang xAI' },
    subKey: (hint: string, models: number) => `Khoá API ${hint} · ${models} model đã chọn`,
    subKeyNone: (hint: string) => `Khoá API ${hint} · chưa chọn model, trợ lý chưa dùng được`,
    subKeyFailed: (hint: string) => `Khoá API ${hint} · chưa lấy được model`,
    subLocal: (host: string, models: number) => `${host} · ${models} model đã chọn · miễn phí, không cần mạng`,
    subLocalNone: (host: string) => `${host} · chưa chọn model, trợ lý chưa dùng được`,
    subUnreachable: (host: string) => `Không kết nối được ${host}`,
    subExpired: 'Phiên đăng nhập đã hết hạn · đăng nhập lại để tiếp tục',

    // Hạn mức.
    quotaDay: (used: number, limit: number) => `Ảnh hôm nay ${used}/${limit}`,
    quotaWeek: (used: number, limit: number) => `Ảnh tuần này ${used}/${limit}`,
    resetDay: (hm: string) => `đặt lại ${hm}`,
    resetWeek: (dow: number) => `đặt lại thứ ${WEEKDAY[dow] ?? 'Hai'}`,
    resetSoon: 'đặt lại sớm',

    // Nút của thẻ.
    modelsCount: (n: number) => `Model · ${n}`,
    modelsPick: 'Chọn model',
    test: 'Thử kết nối',
    testing: 'Đang thử…',
    testOk: (ms: number) => `Kết nối tốt · ${ms} ms`,
    testFail: 'Không kết nối được — kiểm tra mạng hoặc khoá.',
    logout: 'Đăng xuất',
    changeKey: 'Đổi khoá',
    changeAddress: 'Đổi địa chỉ',
    loginChatGPT: 'Đăng nhập ChatGPT',
    loginGrok: 'Đăng nhập Grok',
    loginNow: 'Đăng nhập ngay',
    loginAgain: 'Đăng nhập lại',
    cancelLogin: 'Huỷ',
    close: 'Đóng',
    copy: 'Chép mã',

    // Đăng nhập Grok (mã thiết bị).
    grokIntro: 'Mở trang xAI, đăng nhập tài khoản có SuperGrok hoặc X Premium, rồi nhập mã:',
    grokOpen: 'Mở trang xAI',
    grokWait: 'Đang chờ bạn xác nhận',
    codeExpiry: (mmss: string) => `mã hết hạn sau ${mmss}`,
    codeExpired: 'mã đã hết hạn — đăng nhập lại',
    grokFailed: 'Không kết nối được Grok',
    grokFailedText: 'Phiên đăng nhập hết hạn hoặc bị từ chối.',
    restart: 'Bắt đầu lại',
    // Đăng nhập ChatGPT (mở trình duyệt).
    chatgptWait: 'Đang chờ bạn xác nhận trong trình duyệt',
    loginReconnect: 'Mất kết nối — đang thử lại…',
    openAuth: 'Mở trang đăng nhập',
    codePlaceholder: 'Mã trên trang đăng nhập…',
    pasteCode: 'Nhập mã',
    pasteCodeHint: 'Máy khác? Dán mã từ trang đăng nhập',
    codeInvalid: 'Mã không đúng — kiểm tra lại trên trang đăng nhập.',

    // Ghi chú / lỗi của thẻ.
    addedNote: (label: string, models: number) =>
      `Đã kết nối ${label} và lấy được ${models} model. Chọn ít nhất một model để các trợ lý dùng được. Viderse lọc sẵn model hợp làm video ở mục Gợi ý.`,
    failedTitle: 'Không lấy được danh sách model.',
    failedText: (label: string) =>
      `${label} từ chối khoá API này (401). Kiểm tra lại khoá, hoặc thêm model theo tên nếu bạn biết chắc tên model.`,
    retry: 'Thử lại',
    addManualShort: 'Thêm theo tên',

    // Đổi khoá / địa chỉ tại chỗ.
    keyLabel: 'Khoá API',
    addressLabel: 'Địa chỉ',
    addressPlaceholder: 'http://localhost:11434/v1',
    saveKey: 'Lưu',
    cancelEdit: 'Huỷ',

    // Danh mục model.
    searchIn: (n: number) => `Tìm trong ${n} model`,
    searchLabel: 'Tìm model',
    filterSelected: (n: number) => `Đã chọn · ${n}`,
    filterRecommended: 'Gợi ý',
    filterAll: 'Tất cả',
    filterLabel: 'Lọc model',
    recommended: 'Gợi ý',
    catalogFoot: (src: string, at: string, shown: number, total: number) =>
      `Lấy từ ${src}${at ? ` lúc ${at}` : ''} · đang hiện ${shown} trong ${total}${total > 10 ? ', gõ tên để tìm' : ''}`,
    catalogPriceNote: ' · giá tính trên 1 triệu token hoặc mỗi ảnh',
    catalogLoading: 'Đang tải danh mục…',
    catalogLoadingMore: 'Đang tải thêm…',
    catalogEmpty: 'Không có model nào khớp.',
    catalogFail: 'Không tải được danh mục model của nhà cung cấp này.',
    catalogPrev: 'Trang trước',
    catalogNext: 'Trang sau',
    inPkg: 'trong gói',
    free: 'miễn phí',
    perImage: (price: string) => `${price} mỗi ảnh`,
    tokens: (inPerM: string, outPerM: string) => `${inPerM} vào · ${outPerM} ra`,
    context: (k: number) => `${k}K ngữ cảnh`,
    manualLabel: 'Tên model',
    manualPlaceholder: 'deepseek/deepseek-v3.2',
    manualCheck: 'Kiểm tra & thêm',
    manualClose: 'Đóng',
    manualBusy: 'Đang kiểm tra…',
    manualOk: (id: string, ms: number, ctx: string, caps: string) =>
      `${id} trả lời được · ${ms} ms${ctx} · ${caps}. Đã thêm và chọn.`,
    manualCtx: (k: number) => ` · ngữ cảnh ${k}K (tự nhận)`,
    manualError: (model: string, provider: string) => `Không thấy “${model}” trên ${provider} — kiểm tra lại tên.`,
    manualOpen: 'Không thấy model? Thêm theo tên',
    selectSum: (n: number) => `Đã chọn ${n} model. Model đã chọn hiện trong Trợ lý AI để gán cho từng trợ lý.`,
    selectNone: 'Chưa chọn model nào.',
    /** Dòng chân sheet model điện thoại — luôn nói đơn vị giá (artboard). */
    sheetSum: (n: number) =>
      `${n ? `Đã chọn ${n} model` : 'Chưa chọn model nào'} · giá tính trên 1 triệu token hoặc mỗi ảnh`,
    /** Nhãn hàng model thêm tay (artboard: “Thêm theo tên”). */
    manualNote: 'Thêm theo tên',
    assign: 'Gán cho trợ lý',

    // Thêm nhà cung cấp.
    addTitle: 'Thêm nhà cung cấp',
    // Sheet model của điện thoại.
    modelsTitle: (label: string) => `Model của ${label}`,
    modelsSub: (src: string, at: string, total: number) =>
      `Lấy từ ${src}${at ? ` lúc ${at}` : ''} · ${total} model. Model đã chọn hiện trong Trợ lý AI.`,
    modelsSubDefault: 'Model đã chọn hiện trong Trợ lý AI.',
    addSubMobile: 'Dán khoá API; Viderse gọi thử một lần để lấy danh mục model, không tốn tiền.',
    connected: 'Đã kết nối',
    alreadyConnected: (name: string) => `${name} đã kết nối · bấm để Đổi khoá`,
    keyLabelOf: (name: string) => `Khoá API ${name}`,
    addressLabelOf: (name: string) => `Địa chỉ ${name}`,
    addHint: 'Khoá chỉ lưu trên máy này và luôn hiện dạng che. Viderse gọi thử một lần để lấy danh mục model, không tốn tiền.',
    addHintMobile: 'Khoá chỉ lưu trên máy đang chạy Viderse, luôn hiện dạng che.',
    connectFetch: 'Kết nối và lấy model',
    connecting: 'Đang kết nối…',
    addOpen: 'Thêm nhà cung cấp — OpenAI, Gemini, OpenRouter, DeepSeek, xAI, Ollama…',
    addOpenShort: 'Thêm nhà cung cấp',

    // Hành động phá huỷ.
    removeTitle: (provider: string) => `Gỡ ${provider}?`,
    removeBody: 'Mất khoá/địa chỉ đã lưu và toàn bộ model đã chọn của nhà cung cấp này. Trợ lý đang dùng thì phải đổi model trước.',
    removeOk: 'Gỡ',
    removed: (provider: string) => `Đã gỡ ${provider}.`,
    inUseError: 'Không gỡ được: trợ lý đang dùng model của nhà cung cấp này — đổi model ở Trợ lý AI trước.',

    // Thanh chưa lưu.
    dirty: (n: number) => `${n} thay đổi chưa lưu ở Kết nối AI · dùng cho cả máy này`,
    save: 'Lưu',
    cancel: 'Huỷ',
    savedNote: (n: number) => `Đã lưu ${n} thay đổi.`,
  },

  agents: {
    title: 'Trợ lý AI',
    mobileLede: 'Chọn model cho từng trợ lý và nguồn vẽ ảnh. Chỉ hiện model đã chọn ở Kết nối AI.',
    loading: 'Đang tải cấu hình trợ lý…',
    loadFail: 'Không tải được cấu hình trợ lý.',
    noModel: 'Không dùng model',
    /** Vai trò dưới tên trợ lý (desktop / điện thoại có câu ngắn hơn). */
    role: {
      scout: 'Quét kênh đối thủ, tìm video nổi trội',
      director: 'Chia cảnh, viết tiêu đề, mô tả và chữ trên ảnh bìa',
      artist: 'Viết mô tả ảnh từ lời kể, rồi vẽ ảnh cảnh, ảnh bìa và bộ nhận diện kênh',
      qa: 'Chấm độ hoàn thiện, đo âm lượng và xuất video',
    },
    roleMobile: {
      scout: 'Quét kênh, tìm video nổi trội',
      director: 'Chia cảnh, tiêu đề, mô tả, chữ bìa',
      artist: 'Viết mô tả ảnh từ lời kể',
      qa: 'Độ hoàn thiện, âm lượng, xuất',
    },
    artistMobile: 'Hoạ sĩ · mô tả ảnh',
    modelLabel: 'Model',
    pickModel: 'Chọn model…',
    directorTitle: 'Model của Đạo diễn',
    artistTitle: 'Model của Hoạ sĩ',
    effort: 'Mức suy nghĩ',
    effortAuto: 'Tự động',
    effortLow: 'Thấp',
    effortMedium: 'Vừa',
    effortHigh: 'Cao',
    chainDirector: 'Khi model chính lỗi, thử lần lượt',
    chainArtist: 'Nguồn dự phòng, thử lần lượt',
    chainAdd: 'Thêm nguồn',
    /** Chuỗi dự phòng của Đạo diễn: artboard chỉ ghi “Thêm”. */
    chainAddShort: 'Thêm',
    promptModel: 'Model viết mô tả ảnh',
    promptHint: 'Cần model đọc được ảnh, vì Hoạ sĩ xem khung đối thủ làm mẫu.',
    drawLabel: 'Vẽ ảnh bằng',
    drawLabelMobile: 'Hoạ sĩ vẽ ảnh bằng',
    default: 'Mặc định',
    /** Thẻ nguồn vẽ của gói: tên gói đã lưu + hạn mức (artboard “Gói Plus · 50 ảnh mỗi ngày”). */
    /** Tên gói như artboard: ChatGPT “Gói Plus”, Grok “SuperGrok”. */
    planName: { plus: 'Gói Plus', pro: 'Gói Pro', supergrok: 'SuperGrok' } as Record<string, string | undefined>,
    planSub: (plan: string, limit: number, period: 'day' | 'week') =>
      `${plan} · ${limit} ảnh mỗi ${period === 'week' ? 'tuần' : 'ngày'}`,
    planKey: 'Khoá API · trả theo ảnh',
    planSubPaid: 'Gói đang trả · theo hạn mức của gói',
    /** Gói không lưu tên gói trong wire (xAI): chỉ nói hạn mức thật. */
    planSubGeneric: (limit: number, period: 'day' | 'week') =>
      `Gói đang trả · ${limit} ảnh mỗi ${period === 'week' ? 'tuần' : 'ngày'}`,
    /** Nguồn trả phí: trần tự tiêu mỗi ngày do máy đặt (D7). */
    planPaid: (limit: number) => `Khoá API · trả theo ảnh · máy tự tiêu tối đa ${limit} ảnh/ngày`,
    /** Dạng rút gọn trên điện thoại (artboard MSettings agents). */
    planPaidMobile: (limit: number) => `Khoá API · trả theo ảnh · tự tiêu tối đa ${limit}/ngày`,
    planPaidOff: 'Khoá API · trả theo ảnh · máy không tự tiêu',
    planLocal: 'Chạy trên máy · miễn phí',
    usedDay: (used: number, limit: number) => `${used}/${limit} hôm nay`,
    usedWeek: (used: number, limit: number) => `${used}/${limit} tuần này`,
    resetAt: (hm: string) => `đặt lại ${hm}`,
    resetWeekAt: (dow: number) => `đặt lại thứ ${['CN', 'Hai', 'Ba', 'Tư', 'Năm', 'Sáu', 'Bảy'][dow] ?? 'Hai'}`,
    usedPrice: (price: string) => `${price} mỗi ảnh`,
    needLogin: (name: string) => `Cần đăng nhập ${name} ở Kết nối AI`,
    needLoginTip: (name: string) => `Đăng nhập ${name} ở Kết nối AI trước`,
    retryOther: 'Vẽ lại bằng nguồn khác khi lỗi',
    retryHint: 'Ảnh vẽ lỗi hoặc gặp hết hạn mức được vẽ lại ngay bằng nguồn kế tiếp đã kết nối.',
    retryHintMobile: 'Ảnh lỗi hoặc hết hạn mức được vẽ lại bằng nguồn kế tiếp.',
    footer: 'Chỉ hiện model đã chọn ở Kết nối AI. Đổi model có hiệu lực từ bước chạy tiếp theo; video đang ở giữa một bước chạy nốt bằng model cũ.',
    pickerSubText: 'Model đã chọn ở Kết nối AI. Mờ là model chỉ vẽ ảnh.',
    pickerSubVision: 'Model đã chọn ở Kết nối AI. Mờ là model không đọc được ảnh hoặc chỉ vẽ ảnh.',
    /** Sheet điện thoại dùng câu việc-phải-làm thay cho câu quy tắc danh mục. */
    sheetSubDirector: 'Chia cảnh, viết tiêu đề, mô tả và chữ trên ảnh bìa. Mờ là model chỉ vẽ ảnh.',
    sheetSubVision: 'Cần đọc được ảnh vì Hoạ sĩ xem khung đối thủ làm mẫu.',
    /** Tiêu đề sheet viết-mô-tả-ảnh — khác “Model của Hoạ sĩ” (nguồn vẽ). */
    artistSheetTitle: 'Model viết mô tả ảnh',
    pickerFooter: 'Thêm model ở Kết nối AI',
    pickerEmpty: 'Chưa chọn model nào — vào Kết nối AI tick model trước.',
    blockedImage: 'Chỉ vẽ ảnh, không viết chữ',
    blockedVision: 'Không đọc được ảnh',
    blockedText: 'Không viết được chữ',
    blockedDraw: 'Không vẽ được ảnh',
    groupOf: (provider: string, auth: string) => `${provider} · ${auth}`,
    groupPlan: (provider: string, plan: string) => `${provider} · gói ${plan}`,
    groupAuth: { subscription: 'đăng nhập gói', api_key: 'khoá API', local: 'máy này' },
    // 422 capability_missing → câu inline dưới trường.
    needVision: 'Model này không đọc được ảnh — chọn model có tag Đọc ảnh.',
    needImage: 'Model này không vẽ được ảnh — chọn model có tag Vẽ ảnh.',
    needText: 'Model này không viết được chữ — chọn model khác.',
    savedHint: 'Đổi model có hiệu lực từ bước chạy tiếp theo',
    saving: 'Đang lưu…',
    done: 'Xong',
  },
} as const
