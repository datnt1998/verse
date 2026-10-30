// Chuỗi giao diện tiếng Việt — mọi chữ hiển thị của app đi qua `t`.
// Thuật ngữ, mẫu câu và câu lỗi theo design 06-copy.md; không lộ kỹ thuật
// (tên công cụ, đường dẫn, mã lỗi) ngoài “Chi tiết”.
import { accountsCopy } from './accounts'
import { aiCopy } from './ai'
import { logs } from './logs'
import { settings } from './settings'
import { setup } from './setup'
import { storage } from './storage'
import { tools } from './tools'

export const vi = {
  brand: { name: 'Viderse', studio: 'Studio' },

  common: {
    loading: 'Đang tải…',
    retry: 'Thử lại',
    retryNow: 'Thử lại ngay',
    continue: 'Tiếp tục',
    cancel: 'Huỷ',
    open: 'Mở',
    copy: 'Sao chép',
    close: 'Đóng',
    details: 'Chi tiết',
    logout: 'Đăng xuất',
    loggingOut: 'Đang đăng xuất…',
    undo: 'Hoàn tác',
    save: 'Lưu',
  },

  // Bốn trợ lý (01-ia-flows §4 — tên hiển thị luôn là tên Viderse).
  agents: {
    scout: { name: 'Trinh sát', role: 'Tải nguồn, phụ đề, bốc clip nền' },
    director: { name: 'Đạo diễn', role: 'Tiêu đề, mô tả, tag, chia cảnh' },
    artist: { name: 'Hoạ sĩ', role: 'Ảnh cảnh, ảnh bìa, bộ nhận diện' },
    qa: { name: 'Kiểm định', role: 'Dựng, đo độ hoàn thiện, xuất MP4' },
  },

  // Trạng thái video (01-ia-flows §3.1 — một tên, một màu, mọi nơi giống nhau).
  states: {
    queued: 'Chờ lượt',
    working: 'Đang làm',
    review: 'Chờ duyệt',
    issue: 'Cần xử lý',
    paused: 'Tạm dừng',
    ready: 'Sẵn sàng',
    posted: 'Đã đăng',
  },

  nav: {
    main: 'Điều hướng chính',
    bottom: 'Điều hướng',
    workbench: 'Bàn dựng',
    review: 'Duyệt',
    channels: 'Kênh',
    explore: 'Khám phá',
    new: 'Video mới',
    allChannels: 'Mọi kênh',
    myChannels: 'Kênh của tôi',
    createChannel: 'Tạo kênh',
  },

  shell: {
    jobsIdle: (queued: number) => `Máy rảnh · ${queued} chờ lượt`,
    jobsRunning: (running: number, queued: number) => `${running} đang chạy · ${queued} chờ lượt`,
    jobsPaused: (queued: number) => `Đã tạm dừng · ${queued} chờ lượt`,
    jobsOffline: 'Mất kết nối',
    jobsAriaLabel: 'Việc nền',
    jobsUnread: 'Có thông báo chưa đọc',
    jobsTip: (running: number, lanes: number) => `Máy đang chạy ${running}/${lanes} luồng`,
    openQueue: 'Mở Hàng chờ',
    settings: 'Cài đặt',
    settingsCrumb: 'Cài đặt · máy này',
    settingsAttention: (n: number) => `Cài đặt · ${n} việc cần xem`,
    account: 'Tài khoản của bạn',
    channelSwitcher: 'Đổi kênh',
    viewingAllChannels: 'đang xem Mọi kênh',
  },

  // Dải mất kết nối (System artboard · state=offline).
  offline: {
    title: 'Mất kết nối với máy chủ Viderse',
    retryingIn: (seconds: number) => `đang thử lại sau ${seconds} giây`,
    body: 'Video đang chạy trên máy vẫn tiếp tục nếu app còn mở. Màn hình dừng cập nhật tới khi kết nối lại; thao tác của bạn chưa được gửi.',
  },

  auth: {
    serverOnline: 'Máy chủ Viderse · đang hoạt động',
    serverOffline: 'Không kết nối được máy chủ Viderse',
    loginTitle: 'Đăng nhập',
    loginSub: 'Dùng tài khoản quản trị viên đã tạo lúc thiết lập',
    username: 'Tên đăng nhập',
    usernameHint: 'Chữ thường không dấu, số và . _ - · 2–32 ký tự',
    usernamePlaceholder: 'vd: linh',
    password: 'Mật khẩu',
    login: 'Đăng nhập',
    loggingIn: 'Đang đăng nhập…',
    sessionNote: 'Phiên giữ 30 ngày trên thiết bị này.',
  },

  setup: {
    title: 'Thiết lập Viderse',
    stepsAria: 'Các bước thiết lập',
    steps: ['Mã thiết lập', 'Tài khoản', 'Mã khôi phục'],
    codeLabel: 'Mã thiết lập',
    codeHint: 'Mã in trong terminal khi chạy `viderse serve` lần đầu.',
    codePlaceholder: 'XXXX-XXXX',
    verifying: 'Đang kiểm tra…',
    attemptsLeft: (n: number) => `Mã chưa đúng — còn ${n} lần thử.`,
    nameLabel: 'Tên hiển thị',
    usernameLabel: 'Tên đăng nhập',
    passwordLabel: 'Mật khẩu',
    passwordHint: 'Ít nhất 8 ký tự',
    createAdmin: 'Tạo quản trị viên',
    creating: 'Đang tạo…',
    recoveryTitle: 'Đã tạo quản trị viên',
    recoveryWarning:
      'Đây là mã khôi phục duy nhất — Viderse chỉ hiện một lần này. Lưu lại ở nơi an toàn: mất cả mật khẩu lẫn mã này là mất quyền vào máy chủ.',
    recoveryLabel: 'Mã khôi phục',
    enter: 'Vào Viderse',
  },

  // Trạng thái hệ thống (System artboard) và trạng thái trống của từng trang.
  system: {
    loadingWorkbench: 'Đang tải Bàn dựng',
    notFoundTitle: 'Không tìm thấy video này',
    notFoundBody: 'Video có thể đã bị xoá, hoặc link đã cũ.',
    notFoundAction: 'Về Bàn dựng',
    emptyWorkbenchTitle: 'Chưa có video nào',
    emptyWorkbenchBody: 'Dán link video đối thủ hoặc chọn video nổi ở Khám phá — Viderse lo phần còn lại.',
    emptyWorkbenchAction: 'Làm video',
    emptyWorkbenchAlt: 'Khám phá',
  },

  screens: {
    workbench: {
      title: 'Bàn dựng',
      videosOfChannel: 'Video của kênh',
      emptyHint: 'Video bạn làm sẽ hiện ở đây theo nhóm Chờ bạn · Đang làm · Chờ lượt · Sẵn sàng.',
    },
    review: {
      title: 'Duyệt',
      emptyTitle: 'Đã duyệt hết',
      emptyHint: 'Video mới cần duyệt sẽ hiện ở đây, kèm thông báo.',
      emptyAction: 'Về Bàn dựng',
    },
    queue: {
      title: 'Hàng chờ',
      emptyTitle: 'Hàng chờ trống',
      emptyHint: 'Thêm video từ Khám phá hoặc dán link ở Video mới.',
      emptyAction: 'Làm video',
    },
    channel: {
      title: 'Kênh',
      emptyTitle: 'Chưa có video nào',
      emptyHint: 'Video của kênh sẽ hiện ở đây. Liên kết đối thủ để máy tự lấy video mới.',
      emptyAction: 'Làm video',
    },
    channelNew: {
      title: 'Tạo kênh',
      emptyTitle: 'Kênh mới',
      emptyHint: 'Luồng tạo kênh sẽ dựng ở đây.',
    },
    explore: {
      title: 'Khám phá',
      emptyTitle: 'Chưa theo dõi đối thủ nào',
      emptyHint: 'Thêm kênh đối thủ bằng @handle hoặc link — Trinh sát sẽ quét video nổi để bạn chọn làm.',
    },
    new: {
      title: 'Video mới',
      emptyTitle: 'Dán link video đối thủ',
      emptyHint: 'Luồng Làm video sẽ dựng ở đây.',
    },
    library: {
      title: 'Thư viện',
      emptyTitle: 'Thư viện dùng chung',
      emptyHint: 'Template, prompt ảnh, video nền và lớp phủ sẽ nằm ở đây.',
    },
    template: {
      title: 'Cài đặt template',
      emptyTitle: 'Template',
      emptyHint: 'Cài đặt template sẽ dựng ở đây.',
    },
    editor: {
      title: 'Editor',
      emptyTitle: 'Chưa mở project nào',
      emptyHint: 'Mở editor từ một video Chưa đạt độ hoàn thiện trên Bàn dựng.',
    },
    settings: {
      title: 'Cài đặt',
      emptyTitle: 'Cài đặt',
      emptyHint: 'Công cụ trên máy, kết nối AI, tài khoản nền tảng và lưu trữ sẽ nằm ở đây.',
    },
  },

  // Hộp xác nhận — 7 tình huống của Kit › Xác nhận / Dialogs.
  confirm: {
    pause: {
      title: (agent: string) => `Tạm dừng ${agent}?`,
      body: (video: string, scene: string) =>
        `“${video}” đang vẽ ${scene}. Bạn có thể chạy tiếp bất cứ lúc nào.`,
      choiceAfter: 'Sau cảnh này',
      choiceAfterNote: 'Khuyên dùng — giữ ảnh cảnh đang vẽ',
      choiceNow: 'Ngay bây giờ',
      choiceNowNote: 'Bỏ ảnh đang vẽ dở của cảnh',
      cancel: 'Tiếp tục vẽ',
      ok: 'Tạm dừng',
      busy: 'Đang tạm dừng…',
      done: (agent: string) => `Đã tạm dừng ${agent} sau cảnh này`,
      error: 'Không tạm dừng được: trợ lý không phản hồi. Thử lại sau vài giây.',
    },
    overwrite: {
      title: 'Thay bản MP4 cũ?',
      body: (file: string) => `${file} đã có trong thư mục xuất. Bản cũ sẽ được thay bằng bản mới.`,
      cancel: 'Huỷ',
      alt: 'Giữ cả hai',
      ok: 'Thay bản cũ',
      busy: 'Đang xuất…',
      done: (file: string) => `Đã thay ${file}`,
      error: 'Không ghi được tệp: thư mục xuất đang chỉ cho đọc.',
      current: 'Bản đang có',
      next: 'Bản mới',
    },
    logout: {
      title: (provider: string) => `Đăng xuất gói ${provider}?`,
      body: (fallback: string) => `Hoạ sĩ chuyển sang vẽ bằng ${fallback} (dự phòng) cho tới khi bạn đăng nhập lại. Video đang làm không bị mất.`,
      ok: (provider: string) => `Đăng xuất ${provider}`,
      busy: 'Đang đăng xuất…',
      done: (provider: string, fallback: string) => `Đã đăng xuất ${provider} · Hoạ sĩ vẽ bằng ${fallback}`,
      error: 'Không đăng xuất được: mất kết nối mạng.',
    },
    deleteVideo: {
      title: (name: string) => `Xoá video “${name}”?`,
      body: 'Việc này không hoàn tác được.',
      willDelete: 'Sẽ xoá',
      willKeep: 'Giữ lại',
      cancel: 'Huỷ',
      ok: 'Xoá video',
      busy: 'Đang xoá…',
      done: 'Đã xoá video · MP4 vẫn còn trong thư mục xuất',
      error: 'Không xoá được: video đang được dựng. Tạm dừng rồi thử lại.',
      runningHint: 'Video đang chạy phải Tạm dừng trước rồi mới xoá.',
    },
    deleteChannel: {
      title: (name: string) => `Xoá kênh “${name}”?`,
      body: 'Xoá khỏi Viderse. Kênh trên YouTube không bị động tới.',
      typePhrase: 'để xác nhận',
      cancel: 'Huỷ',
      ok: 'Xoá kênh',
      busy: 'Đang xoá…',
      done: 'Đã xoá kênh · MP4 vẫn còn trong thư mục xuất',
      error: 'Không xoá được: kênh đang có video đang làm.',
      runningHint: (n: number) => `Kênh có ${n} video đang làm: chúng chạy xong rồi mới xoá`,
      idleHint: 'Kênh không có video đang chạy',
    },
    stopAll: {
      title: 'Dừng mọi việc đang chạy?',
      body: (n: number) => `${n} video đang làm dừng ngay, ảnh đang vẽ dở bị bỏ. Hàng chờ giữ nguyên.`,
      holdLabel: 'Giữ để dừng mọi việc',
      cancel: 'Huỷ',
      done: (n: number) => `Đã dừng ${n} video`,
    },
    leave: {
      title: 'Bỏ thay đổi chưa lưu?',
      body: (n: number, name: string) => `Bạn đã sửa ${n} mục trong template “${name}” nhưng chưa lưu.`,
      cancel: 'Ở lại',
      alt: 'Bỏ thay đổi',
      ok: 'Lưu rồi rời đi',
      busy: 'Đang lưu…',
      done: (n: number) => `Đã lưu ${n} mục`,
      error: 'Không lưu được: template đang sửa ở cửa sổ khác. Tải lại để xem.',
    },
    autonomy: {
      title: (name: string) => `Chuyển ${name} sang Tự chạy?`,
      body: 'Máy tự duyệt video khi mọi ảnh từ 85 điểm, và tự sửa video chưa đạt trước khi hỏi bạn.',
      belowThreshold: 'Có ảnh dưới 85 thì cả video vẫn chờ bạn duyệt, ảnh đó được đánh dấu',
      fromNow: 'Áp dụng cho ảnh vẽ từ bây giờ. Video đang chờ duyệt vẫn chờ bạn',
      never: 'Đổi lại bất cứ lúc nào ở Mức tự chủ. Không bao giờ tự đăng, tự xoá hay đổi template',
      ok: 'Chuyển sang Tự chạy',
    },
  },

  // Cài đặt: khung + Công cụ/Lưu trữ/Nhật ký (i18n/settings,tools,storage,logs),
  // Chuẩn bị máy (i18n/setup), Kết nối AI + Trợ lý AI (ai.ts), Tài khoản nền tảng (accounts.ts).
  ai: aiCopy,
  accounts: accountsCopy,
  settings,
  tools,
  storage,
  logs,
  prep: setup,

  // Mã lỗi → câu (06-copy.md › Từ mã lỗi tới câu). Trả null khi mã không nên
  // hiện lỗi (vd rate_limited chỉ là dòng phụ).
  errors: {
    login_required: 'Cần đăng nhập ChatGPT để Hoạ sĩ vẽ tiếp.',
    cookies_failed: 'YouTube yêu cầu đăng nhập để tải video này.',
    unsupported_platform: (platform: string) => `Chưa hỗ trợ ${platform}. App lấy nguồn nền từ Twitch và Bilibili.`,
    unavailable: 'Video nguồn không còn xem được (đã ẩn hoặc xoá).',
    no_subtitles: 'Video không có phụ đề. Trinh sát sẽ tự chép lời từ âm thanh.',
    quality_gate: (score: number) => `Chưa đạt độ hoàn thiện ${score}/100`,
    auth_failed: (provider: string) => `${provider} từ chối khoá API này. Kiểm tra lại khoá.`,
    template_changed: 'Template đã đổi số ảnh hoặc prompt ảnh. Đạo diễn cần chia cảnh lại.',
    failed: 'Có lỗi xảy ra. Thử lại.',
  },

  home: {
    title: 'Trang chủ',
  },

  account: {
    title: 'Tài khoản',
    menuLabel: 'Tài khoản của bạn',
    username: 'Tên đăng nhập',
    role: 'Vai trò',
    memberSince: 'Tạo lúc',
    logout: 'Đăng xuất',
    logoutFailed: 'Không đăng xuất được. Vui lòng thử lại.',
  },

  role: {
    admin: 'Quản trị viên',
    member: 'Thành viên',
  },

  devkit: {
    title: 'Kit Viderse',
    boards: { kit: 'Kit (máy tính)', mkit: 'MKit (điện thoại)' },
    groups: {
      foundation: 'Nền tảng',
      confirm: 'Xác nhận',
      menus: 'Menu',
      select: 'Select & Combobox',
      inputs: 'Ô nhập & nút',
      overlays: 'Gợi ý & trạng thái',
      video: 'Video & thẻ',
      agent: 'Trợ lý',
      bars: 'Đầu trang',
      controls: 'Điều khiển',
      lists: 'Hàng',
      sheets: 'Sheet',
    },
    reopenDialog: 'Mở lại hộp thoại',
    demoToast: 'Đã thêm 3 video vào hàng chờ',
  },
} as const

export type ViStrings = typeof vi
export const t = vi
