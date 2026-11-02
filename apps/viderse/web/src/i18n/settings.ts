// Copy của khung Cài đặt (desktop 3 cột + mobile danh sách) — Settings.dc.html,
// MSettings.dc.html và 06-copy.md.
import { accountsCopy } from './accounts'
import { aiCopy } from './ai'
import { base } from './vi'
import { errorsCopy } from './errorsCopy'
import { logs } from './logs'
import { setup } from './setup'
import { storage } from './storage'
import { tools } from './tools'

export const settings = {
  backMobile: 'Quay lại',
  nav: {
    machine: 'Máy này',
    data: 'Dữ liệu',
    elsewhere: 'Ở trang khác',
    tools: 'Công cụ trên máy',
    connect: 'Kết nối AI',
    agents: 'Trợ lý AI',
    accounts: 'Tài khoản nền tảng',
    storage: 'Lưu trữ',
    logs: 'Báo lỗi',
    notifications: 'Thông báo',
    queue: 'Luồng chạy · hàng chờ',
    footerNote: 'Mọi cài đặt ở đây áp dụng cho cả máy, mọi kênh dùng chung.',
    version: 'Viderse {version}',
    versionNoVersion: 'Viderse',
  },
  /** Tiêu đề + câu dẫn từng mục (META của artboard Settings). */
  meta: {
    tools: { title: 'Công cụ trên máy', lede: 'Viderse tự tải, cài và cập nhật các công cụ này. Mọi việc xử lý video đều chạy trên máy của bạn.' },
    connect: { title: 'Kết nối AI', lede: 'Tài khoản và khoá dùng chung cho cả máy này. Đăng nhập gói đang trả (ChatGPT, Grok) hoặc dán khoá API; model lấy thẳng từ nhà cung cấp.' },
    agents: { title: 'Trợ lý AI', lede: 'Chọn model cho từng trợ lý và nguồn vẽ ảnh. Chỉ hiện model đã chọn ở Kết nối AI.' },
    accounts: { title: 'Tài khoản nền tảng', lede: 'Các nền tảng app lấy video từ đó: YouTube cho video đối thủ, Twitch và Bilibili cho nguồn nền. Đăng nhập chỉ cần khi nền tảng đòi; chỉ dùng tài khoản phụ.' },
    storage: { title: 'Lưu trữ', lede: 'Chọn thư mục lưu dự án, xem dung lượng và lấy lại chỗ trống. Mỗi mục nói rõ cái gì bị xoá và cái gì được giữ.' },
    logs: { title: 'Báo lỗi', lede: 'Gửi nhật ký đã che thông tin cá nhân khi cần nhờ sửa lỗi.' },
    notifications: { title: 'Thông báo', lede: 'Chọn việc nào báo cho bạn, báo trong app hay trên máy, và giờ không làm phiền.' },
  } as Record<string, { title: string; lede: string }>,
  /** Số bên phải tên mục (artboard: chỉ hiện khi có nghĩa). */
  counts: {
    toolsInstalling: '{done}/{total}',
    storageFallback: '—',
    logsFallback: '0',
  },
  /** Thẻ phải của khung — mỗi mục một cặp [tiêu đề thẻ, dòng giải thích]. */
  help: {
    tools: { title: 'Khi nào cần làm gì', lines: ['Tải video lỗi hàng loạt thường do YouTube vừa đổi cách tải. Bấm Cập nhật ở Tải video và phụ đề là đủ.', 'Thiếu công cụ khi đang làm video, app chỉ cách sửa rồi quay về đúng video đó.'], tips: { title: 'Mẹo', items: ['App tự kiểm tra mỗi lần mở.', 'Chuyển sang máy khác thì không cần chép công cụ, app tự cài lại.'] } },
    connect: { title: 'Hai cách kết nối', lines: ['Đăng nhập gói (ChatGPT, Grok): dùng hạn mức gói bạn đang trả, không tốn thêm tiền.', 'Khoá API (Gemini, OpenRouter, OpenAI…): trả theo lượt dùng, không có hạn mức ngày. Ollama chạy model ngay trên máy, miễn phí.', 'Danh mục model lấy thẳng từ nhà cung cấp, nên model mới hiện ra ngay khi họ phát hành.'], tips: { title: 'An toàn', items: ['Khoá và đăng nhập chỉ lưu trên máy này, luôn hiện dạng che.', 'Xoá một nhà cung cấp đang được trợ lý dùng thì app hỏi chọn model thay thế trước.'] } },
    agents: { title: 'Khi nào có hiệu lực', lines: ['Đổi model hay nguồn vẽ có hiệu lực từ bước chạy tiếp theo, kể cả vẽ lại. Ảnh đã vẽ và đã duyệt giữ nguyên.', 'Hết hạn mức thì bước vẽ ảnh tạm chờ, giờ thử lại hiện trong Hàng chờ, trừ khi có nguồn dự phòng.'], tips: { title: 'Mẹo', items: ['Giữ ChatGPT làm nguồn vẽ chính, thêm Gemini làm dự phòng để ít khi phải chờ.', 'Một video 6 cảnh dùng khoảng 7 ảnh.', 'Trinh sát và Kiểm định chạy bằng công cụ trên máy, không cần model.'] } },
    accounts: { title: 'An toàn tài khoản', lines: ['Cookie và đăng nhập chỉ nằm trên máy này. App chỉ hiện giá trị đã che.', 'Tài khoản phụ bị giới hạn cũng không ảnh hưởng kênh đang kiếm tiền.'], tips: { title: 'Khi nào cần', items: ['YouTube báo “xác nhận bạn không phải bot”: chọn hồ sơ trình duyệt có đăng nhập Google phụ.', 'Bilibili hay tạm chặn khi quét nhiều kênh: app tự đợi rồi quét lại. Đăng nhập bằng mã QR giúp ít bị chặn hơn; app tự gia hạn mỗi ngày.', 'Thêm kênh hay video lẻ ở Thư viện › Video nền.'] } },
    storage: { title: 'Không mất gì quan trọng', lines: ['Video đang chạy, video chờ duyệt và media template đang dùng không bao giờ bị dọn.', 'Tiêu đề, mô tả, ảnh bìa và project luôn được giữ.'], tips: { title: 'Mẹo', items: ['Xoá MP4 sau khi đã đăng là cách lấy lại chỗ nhanh nhất.'] } },
    notifications: { title: 'Chỉ báo việc của bạn', lines: ['Mỗi loại việc bật riêng trong app và trên máy. Tắt hết vẫn thấy việc trong Hàng chờ, chỉ không bị làm phiền.', 'Thông báo hệ thống cần trình duyệt cho phép; nếu đã chặn thì bật lại trong cài đặt trình duyệt.'], tips: { title: 'Mẹo', items: ['Bật Gộp để nhiều video xong gần nhau thành một thông báo.', 'Không làm phiền chỉ tắt toast và thông báo hệ thống — lịch sử vẫn ghi.'] } },
    logs: { title: 'Gửi log thế nào', lines: ['Gửi tệp .zip kèm một câu bạn đang làm gì khi lỗi xảy ra.', 'Tệp không chứa cookie, mật khẩu hay nội dung tài khoản.'], tips: { title: 'Mẹo', items: ['Lỗi có giờ thử lại thì app tự xử lý, thường không cần báo.'] } },
  } as Record<string, { title: string; lines: string[]; tips: { title: string; items: string[] } }>,
  m: {
    // MSettings — thẻ tổng, giá trị từng hàng của danh sách.
    summary: {
      ok: 'Máy sẵn sàng',
      okSub: 'Mọi công cụ đã cài và chạy được',
      updateSub: '{n} bản cập nhật nên cài',
      installingSub: 'Đang cài {n} công cụ',
      failedTitle: '{n} công cụ cần sửa',
      failedSub: 'Video mới chờ ở bước bị lỗi',
    },
    values: {
      updates: '{n} bản cập nhật',
      toolsOk: 'Mọi công cụ đã cài',
      toolsInstalling: 'Đang cài {n} công cụ',
      providers: '{n} nhà cung cấp',
      artist: 'Vẽ bằng {name}',
      artistNone: 'Chưa chọn nguồn vẽ',
      platforms: '{n} nền tảng',
      logs: 'Xuất log',
    },
    rows: {
      tools: { title: 'Công cụ trên máy' },
      connect: { title: 'Kết nối AI' },
      agents: { title: 'Trợ lý AI' },
      accounts: { title: 'Tài khoản nền tảng' },
      storage: { title: 'Lưu trữ' },
      logs: { title: 'Báo lỗi' },
      queue: { title: 'Luồng chạy, nhịp, tạm dừng', hint: 'Nằm ở Hàng chờ · chạm viên ở trên cùng' },
    },
    foot: 'Viderse {version} · cài đặt áp dụng cho máy này',
    footNoVersion: 'Viderse · cài đặt áp dụng cho máy này',
  },
} as const

/** `t` của khung Cài đặt: chữ khung app + mọi tab cài đặt (một chunk duy nhất). */
export const t = {
  ...base,
  settings,
  tools,
  storage,
  logs,
  ai: aiCopy,
  accounts: accountsCopy,
  errors: errorsCopy.errors,
  prep: setup,
} as const
