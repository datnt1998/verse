// Chuỗi giao diện tiếng Việt — mọi chữ hiển thị của app đi qua `t`.
// Giọng: viết cho người dùng, không phải lập trình viên.

export const vi = {
  brand: { name: 'Viderse', studio: 'Studio' },

  common: {
    loading: 'Đang tải…',
    retry: 'Thử lại',
    continue: 'Tiếp tục',
    logout: 'Đăng xuất',
    loggingOut: 'Đang đăng xuất…',
  },

  nav: {
    main: 'Điều hướng chính',
    bottom: 'Điều hướng',
    home: 'Trang chủ',
    account: 'Tài khoản',
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

  home: {
    title: 'Trang chủ',
    emptyTitle: 'Chưa có màn hình kịch bản',
    emptyHint: 'Giao diện Viderse đang ở bước dựng khung: đăng nhập và tài khoản đã dùng được, các màn brief và kịch bản sẽ dựng tiếp trên khung này.',
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
} as const

export type ViStrings = typeof vi
export const t = vi
