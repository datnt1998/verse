// Copy màn “Chuẩn bị máy cho 4 trợ lý” — Onboard.dc.html (step=setup với
// setup=running|login|failed) và W1. Không có nav: một việc duy nhất là cài máy.
export const setup = {
  header: 'Chuẩn bị máy cho 4 trợ lý',
  /** Tiêu đề tab trình duyệt — tên màn ngắn, bỏ "cho 4 trợ lý". */
  titleShort: 'Chuẩn bị máy',
  skippedWarn: 'Máy này chưa cài đủ công cụ cho 4 trợ lý.',
  skippedAction: 'Cài tiếp',
  back: 'Quay lại',
  stepLabel: 'Bước {n}',
  brand: 'Viderse',
  firstRun: '· Lần đầu mở',
  advanced: 'Cài đặt nâng cao',
  hero: {
    title: 'Chuẩn bị máy cho 4 trợ lý',
    size: 'Viderse tự tải và cài lần lượt, khoảng {size}. Bạn có thể đóng cửa sổ, lần mở sau app cài tiếp từ chỗ dừng.',
    noSize: 'Viderse tự tải và cài lần lượt. Bạn có thể đóng cửa sổ, lần mở sau app cài tiếp từ chỗ dừng.',
  },
  chips: [
    { key: 'scout', name: 'Trinh sát', role: 'tải nguồn' },
    { key: 'director', name: 'Đạo diễn', role: 'viết chữ' },
    { key: 'artist', name: 'Hoạ sĩ', role: 'vẽ ảnh' },
    { key: 'qa', name: 'Kiểm định', role: 'dựng, xuất' },
  ],
  card: {
    running: 'Đang cài',
    runningEta: 'Đang cài · còn khoảng {eta}',
    starting: 'Đang bắt đầu…',
    idle: 'Chưa bắt đầu',
    startFailed: 'Không bắt đầu được',
    login: 'Còn một bước cần bạn',
    failed: 'Một bước chưa cài được',
    done: 'Máy sẵn sàng',
    doneAction: 'Tạo kênh đầu tiên',
    count: '{done} / {total} xong',
    sub: {
      idle: 'Các bước sẽ tự chạy ngay; chờ một lát.',
      running: 'Bạn có thể đăng nhập ChatGPT ngay trong lúc chờ.',
      login: 'Mọi công cụ đã cài xong. Đăng nhập ChatGPT để Hoạ sĩ bắt đầu vẽ.',
      failed: 'Bạn vẫn đăng nhập được ngay. Cần cài xong bước {n} thì mới tạo được {what}.',
      done: 'Mọi công cụ đã cài xong — Viderse mở bước tạo kênh đầu tiên.',
    },
  },
  /** Tên việc + câu mô tả gốc của 6 bước (artboard DEF) và việc mà bước đó
   *  mở khoá khi cài xong (dùng cho câu “mới tạo được …” khi một bước lỗi). */
  steps: {
    'yt-dlp': { title: 'Tải video và phụ đề', hint: 'Lấy video, thông tin và lời thoại từ YouTube', agent: 'scout', unlocks: 'video mới' },
    ffmpeg: { title: 'Xử lý hình và tiếng', hint: 'Cắt cảnh, xử lý âm thanh và dựng video', agent: 'qa', unlocks: 'video mới' },
    transcription: { title: 'Tách tiếng và tạo phụ đề', hint: 'Chép lời thoại thành chữ ngay trên máy', agent: 'scout', unlocks: 'phụ đề' },
    chrome: { title: 'Dựng và kiểm tra video', hint: 'Dùng Chrome có sẵn trên máy, không cần tải thêm', hintManaged: 'Dùng Chrome chạy ngầm để xuất video và đo âm thanh', agent: 'qa', unlocks: 'video' },
    codex: { title: 'Công cụ vẽ ảnh', hint: 'Vẽ ảnh minh hoạ cho từng cảnh và ảnh bìa', agent: 'artist', unlocks: 'ảnh' },
    'codex-login': { title: 'Đăng nhập ChatGPT', hint: 'Ảnh vẽ bằng tài khoản ChatGPT của bạn, nên bước này cần bạn tự đăng nhập', agent: 'artist', unlocks: 'ảnh' },
  } as Record<string, { title: string; hint: string; hintManaged?: string; agent: string; unlocks: string }>,
  states: {
    done: 'Đã cài · {size}',
    doneNoSize: 'Đã cài',
    provided: 'Có sẵn',
    waiting: 'Chờ lượt · {size}',
    waitingNoSize: 'Chờ lượt',
    failedRetry: 'Thử lại sẽ cài tiếp từ chỗ dừng.',
    failedKept: 'Đã giữ {kept}, thử lại sẽ tải tiếp.',
    phaseBytes: '{phase} · {done} / {total}',
    phasePct: '{phase} · {pct}%',
    /** Lý do đọc được cho lỗi cài (server gửi lỗi thô trong `error`). */
    reasons: {
      network: 'Mạng bị ngắt khi tải.',
      timeout: 'Tải quá chậm nên bị ngắt.',
      space: 'Ổ đĩa hết chỗ.',
      checksum: 'Tệp tải về lệch chữ ký.',
      cancelled: 'Đã huỷ giữa chừng.',
    },
  },
  disk: {
    low: 'Ổ đĩa sắp đầy — còn {free}, cần khoảng {need} để cài đủ.',
  },
  actions: {
    retry: 'Thử lại',
    loginGhost: 'Đăng nhập ngay',
    loginPrimary: 'Đăng nhập ChatGPT',
    cancelLogin: 'Huỷ',
    waiting: 'Chờ bạn đăng nhập trong trình duyệt',
    settingsLink: 'Cài đặt nâng cao',
  },
  footnote: 'Dùng tài khoản ChatGPT của bạn để vẽ ảnh, không cần khoá API. Muốn dùng Grok, Gemini hay nhà cung cấp khác thì thêm ở Cài đặt › Kết nối AI sau khi xong. Cài xong, Viderse mở bước tạo kênh đầu tiên.',
  mobile: {
    title: 'Chuẩn bị máy chỉ có trên máy tính',
    body: 'Lần đầu mở cần tải công cụ và model về máy. Mở Viderse trên máy tính để cài xong một lần, sau đó mọi thứ chạy trên máy đó.',
    noSkip: 'Lần đầu mở thì không bỏ qua được — app cần đủ công cụ mới dựng được video.',
  },
} as const
