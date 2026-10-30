// Mọi chữ của Cài đặt › Tài khoản nền tảng (design 06-copy.md, V23/V24, artboard
// Settings section=accounts · MSettings screen=accounts + sheet mã QR). Câu chữ
// chép nguyên văn artboard; chỉ giá trị sống (giờ, số kênh, cookie đã che, tên
// tài khoản) đi qua hàm. Cookie/khoá không bao giờ hiện đầy đủ trên UI.

export const accountsCopy = {
  title: 'Tài khoản nền tảng',
  loading: 'Đang tải nền tảng…',
  offline: 'Mất kết nối máy chủ — mọi nút trong mục này tắt cho tới khi kết nối lại.',
  loadFail: 'Không tải được nền tảng — thử lại.',

  // ── Thẻ nền tảng ──────────────────────────────────────────────────────────
  // Câu “dùng cho” của từng nền tảng (artboard Settings · plats[].uses). Nền tảng
  platformUses: {
    youtube: 'Video đối thủ: tải video gốc, phụ đề, tiêu đề, lượt xem',
    twitch: 'Video nền: clip, highlight, phát lại',
    bilibili: 'Video nền: video đã đăng',
  } as Record<string, string>,
  // Câu “dùng cho” ngắn hơn của điện thoại (MSettings · plats[].uses).
  platformUsesMobile: {
    youtube: 'Video đối thủ: tải video gốc, phụ đề, lượt xem',
  } as Record<string, string>,
  // Phần đầu dòng phụ (trước số nguồn): chất lượng tải của từng nền tảng.
  platformMetaLead: {
    youtube: 'Tải 360p + phụ đề',
    twitch: 'Tải 480p',
    bilibili: 'Tải 1080p',
  } as Record<string, string>,
  // Ô chữ cái của thẻ (artboard Settings · plats[].ini).
  platformIni: {
    youtube: 'YT',
    twitch: 'TW',
    bilibili: 'BL',
  } as Record<string, string>,
  platformLink: {
    youtube: 'Mở Khám phá',
    twitch: 'Mở Video nền',
    bilibili: 'Mở Video nền',
  } as Record<string, string>,
  useCompetitor: 'Video đối thủ',
  useBackdrop: 'Video nền',
  kindClip: 'clip',
  kindHighlight: 'highlight',
  kindVod: 'phát lại',
  kindVideo: 'video đã đăng',
  metaQuality: (height: number) => `Tải ${height}p`,
  metaChannels: (n: number) => `${n} kênh`,
  metaRivals: (n: number) => `${n} đối thủ`,
  metaVideos: (n: number) => `${n} video lẻ`,
  stateReady: 'Sẵn sàng',
  statePaused: (time: string) => `Tạm chặn tới ${time}`,
  stateNeedCookie: 'Cần cookie',
  stateRelogin: 'Cần đăng nhập lại',
  stateExpired: 'Hết hạn',

  // ── YouTube ───────────────────────────────────────────────────────────────
  ytLoginLabel: 'Lấy cookie từ trình duyệt',
  ytLoginSub: 'Chỉ cần khi YouTube đòi đăng nhập mới cho tải video hoặc phụ đề.',
  ytOff: 'Tắt',
  ytNoProfile: 'Chưa chọn hồ sơ',
  ytStatusOn: 'Đang dùng · lần tải gần nhất không lỗi',
  ytStatusOff: 'Đang tắt · app tải video không đăng nhập',
  ytProfilesEmpty: 'Chưa thấy hồ sơ trình duyệt nào.',
  ytBotTitle: (time: string) => `YouTube báo “xác nhận bạn không phải bot”${time ? ` lúc ${time}` : ''}`,
  ytBotBody: 'Trinh sát chưa tải được phụ đề của một số video. Chọn hồ sơ trình duyệt có đăng nhập Google phụ rồi thử lại.',
  ytRetry: 'Thử lại',
  ytRetrying: 'Đang kiểm tra…',
  ytSave: 'Lưu hồ sơ',
  ytSaved: 'Đã lưu hồ sơ trình duyệt cho YouTube.',
  ytSaving: 'Đang kiểm tra…',
  ytChecked: 'Đã kiểm tra lại cookie YouTube.',
  ytClearTitle: 'Tắt cookie YouTube?',
  ytClearBody: 'App sẽ tải video/phụ đề không đăng nhập — dễ bị YouTube đòi xác nhận hơn. Hồ sơ trình duyệt không bị ảnh hưởng.',
  ytCleared: 'Đã tắt cookie YouTube — app tải không đăng nhập.',

  // ── Twitch ────────────────────────────────────────────────────────────────
  twitchNoLogin: 'Không cần đăng nhập: app chỉ đọc clip, highlight và video phát lại công khai.',
  twitchLoginK: 'Không cần đăng nhập',
  twitchLoginV: 'Chỉ đọc clip và video công khai',

  // ── Bilibili ──────────────────────────────────────────────────────────────
  biliSubAccount: 'Tài khoản phụ',
  biliMethodQr: 'Mã QR',
  biliMethodManual: 'Cookie dán tay',
  biliMethodExpired: 'Mã QR · hết hạn',
  biliSubDaily: (when: string) => `Tự gia hạn mỗi ngày${when ? ` · lần cuối ${when}` : ''}`,
  biliSubRenewed: (time: string) => `Vừa kiểm tra lúc ${time}: đăng nhập vẫn còn hạn`,
  biliSubManual: (masked: string) => (masked ? `Không tự gia hạn · ${masked}` : 'Không tự gia hạn'),
  biliSubExpired: (time: string) =>
    `Gia hạn không được${time ? ` lúc ${time}` : ''} · đang quét kênh không đăng nhập`,
  biliRenew: 'Gia hạn ngay',
  biliRenewing: 'Đang gia hạn…',
  biliRenewed: 'Vừa kiểm tra: đăng nhập vẫn còn hạn.',
  biliQr: 'Đăng nhập bằng mã QR',
  biliClear: 'Xoá đăng nhập',
  biliClearTitle: 'Xoá đăng nhập Bilibili?',
  biliClearBody: 'Cookie đã lưu bị xoá — app sẽ quét kênh không đăng nhập (dễ bị chặn hơn) cho tới khi đăng nhập lại.',
  biliCleared: 'Đã xoá đăng nhập Bilibili.',
  clearConfirmCancel: 'Huỷ',
  biliEmptyTitle: 'Đăng nhập tài khoản phụ',
  biliEmptyOptional: '· tuỳ chọn',
  biliEmptyBody: 'Có đăng nhập thì đọc được nhiều video hơn và ít bị chặn hơn. Quét mã một lần bằng app 哔哩哔哩; app tự gia hạn mỗi ngày.',
  biliManualHelp: 'Cookie dán tay không tự gia hạn: khi hết hạn, app quét kênh không đăng nhập và dễ bị chặn hơn. Quét mã QR một lần để app tự gia hạn.',
  biliManualToggle: 'Dán cookie thủ công',
  biliManualToggleOther: 'Dán cookie khác',
  biliManualLabel: 'Cookie tài khoản phụ',
  biliManualHint: 'Dán SESSDATA hoặc cả chuỗi cookie. Chỉ nhận cookie bilibili.com, không nhận bilibili.tv. App kiểm tra với Bilibili trước khi lưu.',
  biliManualPlaceholder: 'SESSDATA=… hoặc buvid3=…; SESSDATA=…; bili_jct=…',
  biliSave: 'Lưu cookie',
  biliSaving: 'Đang kiểm tra…',
  biliSaved: 'Cookie đã lưu',
  biliSavedAccount: (account: string) => `Đã đăng nhập Bilibili: ${account}`,
  // Bị chặn: tiêu đề máy tính kèm “từ mạng này”, điện thoại ngắn (artboard);
  // desktop nói số lần + lúc bắt đầu chặn, điện thoại nói ngắn.
  biliBlockedTitle: (label: string) => `${label} đang giới hạn yêu cầu từ mạng này`,
  biliBlockedTitleMobile: (label: string) => `${label} đang giới hạn yêu cầu`,
  biliBlockedBody: (count: number, time: string, since?: string) =>
    `Bị chặn ${count} lần liên tiếp${since ? ` từ ${since}` : ''}. App tự quét lại lúc ${time}, không cần làm gì; video đang làm vẫn dùng clip đã tải.`,
  biliBlockedBodyMobile: (time: string) =>
    `App tự quét lại lúc ${time}, không cần làm gì; video đang làm vẫn dùng clip đã tải.`,
  biliExpiredTitle: 'Đăng nhập Bilibili đã hết hạn',
  biliExpiredBody: (time: string) =>
    `App không gia hạn được${time ? ` lúc ${time}` : ''}. Quét mã QR lại; trong lúc chờ, app vẫn quét kênh không đăng nhập nên dễ bị chặn hơn.`,
  biliExpiredBodyMobile: (time: string) =>
    `App không gia hạn được${time ? ` lúc ${time}` : ''}. Quét mã QR lại; trong lúc chờ, app quét kênh không đăng nhập nên dễ bị chặn hơn.`,
  biliCookieEmpty: 'Dán cookie trước đã.',
  biliCookieInvalid: 'Cookie thiếu SESSDATA — lấy lại từ trình duyệt đang đăng nhập bilibili.com.',
  biliCookieRejected: 'Cookie này không đăng nhập được Bilibili. Dán lại cookie bilibili.com hoặc quét mã QR.',
  biliRescan: 'Quét mã QR',

  // ── Điện thoại (MSettings · screen=accounts) ──────────────────────────────
  mLead: 'Các nền tảng app lấy video từ đó. Đăng nhập chỉ cần khi nền tảng đòi; chỉ dùng tài khoản phụ.',
  mFoot: 'Thêm kênh hoặc video lẻ ở Thư viện › Video nền trên máy tính. Cookie và đăng nhập chỉ lưu trên máy chạy Viderse.',
  mYtLoginK: 'Cookie từ trình duyệt',
  mYtLoginV: (profile: string) => `${profile} · đổi trên máy tính`,
  mYtLoginVOff: 'Chưa chọn · đổi trên máy tính',
  mBiliEmptyK: 'Tài khoản phụ · tuỳ chọn',
  mBiliEmptyV: 'Chưa đăng nhập',
  mBiliManualV: 'Không tự gia hạn · quét mã để bật',
  mBiliDaily: (time: string) => `Tự gia hạn mỗi ngày${time ? ` · lần cuối ${time}` : ''}`,
  mBiliMethodQr: 'mã QR',
  mBiliMethodManual: 'cookie dán tay',
  mBiliExpiredV: 'Hết hạn · đang quét không đăng nhập',
  mBiliRenew: 'Gia hạn',
  mBiliScan: 'Quét mã QR',
  mBiliRescan: 'Quét lại',
  mBiliLogin: 'Đăng nhập',

  // ── Hộp thoại / sheet mã QR (07 §24.1 — BilibiliQrState) ───────────────────
  qrTitle: 'Đăng nhập Bilibili bằng mã QR',
  qrTitleMobile: 'Đăng nhập Bilibili',
  qrSub: 'Dùng tài khoản phụ trên app 哔哩哔哩 (bản Trung Quốc). Tài khoản bilibili.tv (bản quốc tế) không dùng được.',
  qrSubMobile: 'Tài khoản phụ trên app 哔哩哔哩 (bản Trung Quốc). Tài khoản bilibili.tv không dùng được.',
  qrSteps: [
    'Mở app 哔哩哔哩 trên điện thoại, đăng nhập tài khoản phụ.',
    'Bấm biểu tượng quét mã, đưa camera vào mã bên trái.',
    'Bấm xác nhận đăng nhập trên điện thoại.',
  ],
  qrStepsMobile: [
    'Mở app 哔哩哔哩, đăng nhập tài khoản phụ.',
    'Bấm biểu tượng quét mã, quét mã ở trên.',
    'Bấm xác nhận đăng nhập.',
  ],
  qrStatusScannedMobile: 'Đã quét. Bấm xác nhận trên điện thoại.',
  qrStatusLoading: 'Đang tạo mã…',
  qrStatusWaiting: 'Đang chờ quét',
  qrStatusScanned: 'Đã quét. Bấm xác nhận trên điện thoại để xong.',
  qrStatusExpired: 'Mã đã hết hạn sau 3 phút.',
  qrStatusSuccess: (account: string) => `Đã đăng nhập: ${account}`,
  qrTimer: (left: string) => `Mã còn hiệu lực ${left}`,
  qrTimerLabel: 'Thời gian còn lại của mã',
  qrOverlayScanned: 'Đã quét',
  qrOverlayExpired: 'Mã đã hết hạn',
  qrOverlayRejected: 'Chưa đăng nhập được',
  qrOverlayError: 'Chưa có mã',
  qrOverlaySuccess: 'Đăng nhập xong',
  qrRejectedTitle: 'Bilibili chưa chấp nhận đăng nhập',
  qrRejectedBody: 'Có thể đã bấm Huỷ trên điện thoại, hoặc Bilibili cần kiểm tra thêm tài khoản này. Tạo mã mới rồi xác nhận lại.',
  qrErrorTitle: 'Chưa kết nối được Bilibili',
  qrErrorBody: 'Máy chạy Viderse chưa gọi được Bilibili. Kiểm tra mạng của máy đó rồi tạo mã mới.',
  qrBusyTitle: 'Bilibili đang bận',
  qrBusyBody: 'Đang có một thao tác đăng nhập khác — đợi vài giây rồi tạo mã mới.',
  qrSuccessExtraBlocked: (time: string) =>
    `Đã bỏ giờ chờ ${time}: app quét lại các kênh Bilibili ngay. Đăng nhập được tự gia hạn mỗi ngày.`,
  qrSuccessExtraBlockedMobile: (time: string) => `Đã bỏ giờ chờ ${time}: app quét lại kênh Bilibili ngay.`,
  qrSuccessExtra: 'App tự gia hạn đăng nhập mỗi ngày khi quét kênh.',
  qrSuccessExtraMobile: 'App tự gia hạn đăng nhập mỗi ngày.',
  qrManualLink: 'Không quét được? Dán cookie thủ công',
  qrManualFootMobile: 'Dán cookie thủ công: làm trên máy tính, Cài đặt › Tài khoản nền tảng.',
  qrClose: 'Đóng',
  qrNew: 'Tạo mã mới',
  qrDone: 'Xong',
  qrHint: 'Cookie chỉ lưu trên máy chạy Viderse. Đăng nhập bằng mã QR được app tự gia hạn mỗi ngày khi quét kênh.',
  qrSameTitle: 'Bilibili trên chính máy này?',
  qrSameBody: 'Mở link đăng nhập trong app rồi bấm xác nhận, không cần quét.',
  qrSameAct: 'Mở app',

  // ── Cuối danh sách ────────────────────────────────────────────────────────
  moreTitle: 'Nền tảng khác',
  moreBody: 'Nền tảng mới được hỗ trợ sẽ hiện thêm một thẻ ở đây. Dán link nền tảng chưa hỗ trợ ở Thư viện › Video nền: app báo rõ và cho gửi yêu cầu.',
  moreLink: 'Mở Video nền',
  secureNote: 'Cookie và đăng nhập chỉ lưu trên máy này, không gửi sang nền tảng khác. Chỉ dùng tài khoản phụ, không dùng tài khoản đang kiếm tiền.',
} as const
