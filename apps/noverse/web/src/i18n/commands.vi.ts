// Nội dung tiếng Việt của hai màn lệnh — chép nguyên văn từ artboard
// docs/design/v1/artboards/Slash.dc.html + Commands.dc.html (khối renderVals `C`).
// Phần *cơ chế* (tên, cú pháp, loại tham số) lấy từ API G11; file này chỉ chứa chữ.
import type { CommandSpec } from '../api/types'

/** Một ví dụ: giá trị điền sẵn cho từng tham số + câu giải thích. */
export type CmdExample = { vals: Record<string, string | boolean>; slash?: string; text: string }

export type CmdParamCopy = {
  /** nhãn trong bảng tham số + câu "Còn thiếu …" */
  label: string
  desc: string
  /** placeholder ô nhập ở bộ ghép lệnh */
  ph?: string
  /** mô tả ngắn cho công tắc */
  hint?: string
  /** dòng "khi bỏ trống" */
  defText?: string
  /** giá trị điền sẵn khi mở bộ ghép lệnh */
  init?: string | boolean
  /** nút "không ghi" của nhóm chọn không bắt buộc */
  none?: string
  /** nhãn ngắn trên nút chọn (artboard dùng label, note lấy từ API) */
  optLabels?: Record<string, string>
}

export type CmdCopy = {
  title: string
  lead: string
  when: string[]
  avoid: string[]
  params: Record<string, CmdParamCopy>
  ex: CmdExample[]
  after: string[]
  log: { cat: string; text: string }[]
  errs: { msg: string; fix: string }[]
  tip: string
  rel: string[]
  /** từ khoá không dấu cho bộ lọc */
  kw: string
}

/** Nhóm + thứ tự lệnh trong nhóm — GROUPS của Slash.dc.html / Commands.dc.html. */
export const CMD_GROUPS = [
  { key: 'begin', label: 'Bắt đầu truyện', cmds: ['start', 'import'] },
  { key: 'write', label: 'Trong lúc viết', cmds: ['review', 'next', 'cocreate', 'sync', 'diag'] },
  { key: 'style', label: 'Văn phong', cmds: ['simulate', 'importsim'] },
  { key: 'end', label: 'Khi truyện đã xong', cmds: ['export', 'reopen'] },
  { key: 'sys', label: 'Cài đặt & trợ giúp', cmds: ['model', 'config', 'help'] },
] as const

/** Thứ tự hiển thị của artboard (ORDER trong renderVals). */
export const CMD_ORDER = [
  'help', 'model', 'config', 'diag', 'review', 'next', 'start',
  'import', 'reopen', 'cocreate', 'simulate', 'importsim', 'sync', 'export',
]

export const CMD_COPY: Record<string, CmdCopy> = {
  help: {
    title: 'Mở hướng dẫn lệnh',
    lead: 'Hiện danh sách mọi lệnh, cách gõ và các phím tắt — chính là trang bạn đang xem.',
    when: ['Quên tên lệnh hoặc cách gõ', 'Muốn xem lại phím tắt'],
    avoid: ['Không có — dùng lúc nào cũng được, kể cả khi trợ lý đang viết'],
    params: {},
    ex: [{ vals: {}, text: 'Mở trang hướng dẫn này.' }],
    after: ['Trang hướng dẫn mở ra, truyện vẫn tiếp tục viết bình thường', 'Nhấn Esc hoặc bấm Workspace để quay lại'],
    log: [], errs: [],
    tip: 'Trên web còn nhanh hơn: bấm phím ? ở bất kỳ màn hình nào.',
    rel: ['model', 'config'],
    kw: 'tro giup huong dan phim tat',
  },
  model: {
    title: 'Đổi model cho từng trợ lý',
    lead: 'Noverse có ba trợ lý: Kiến trúc sư lên dàn ý, Người viết viết chương, Biên tập viên soát chương. Mỗi trợ lý có thể dùng một model và mức suy luận riêng.',
    when: ['Muốn Người viết dùng model viết hay hơn', 'Muốn tiết kiệm bằng model rẻ hơn cho Biên tập viên', 'Muốn tăng hoặc giảm mức suy luận'],
    avoid: ['Chưa thêm nhà cung cấp AI nào — dùng /config trước'],
    params: {
      role: {
        label: 'Mở sẵn ở vai trò',
        desc: 'Trợ lý muốn đổi. Bỏ trống để xem cả ba.',
        none: 'Cả ba',
        optLabels: { architect: 'Kiến trúc sư', writer: 'Người viết', editor: 'Biên tập viên' },
      },
    },
    ex: [
      { vals: {}, text: 'Mở bảng chọn model cho cả ba trợ lý.' },
      { vals: { role: 'writer' }, text: 'Mở thẳng tới phần của Người viết.' },
    ],
    after: ['Mở bảng chọn model, nếu có ghi vai trò thì mở sẵn ở vai trò đó', 'Chọn model và mức suy luận, nhấn Enter để lưu', 'Nhấn Esc để đóng mà không đổi gì'],
    log: [],
    errs: [{ msg: 'Vai trò không hợp lệ: writter', fix: 'Chỉ dùng một trong ba: architect, writer, editor.' }],
    tip: 'Danh sách model là của các nhà cung cấp bạn đã thêm. Thiếu model mong muốn thì thêm nhà cung cấp bằng /config.',
    rel: ['config', 'diag'],
    kw: 'doi model ai gpt claude suy luan reasoning thinking',
  },
  config: {
    title: 'Thêm hoặc sửa nhà cung cấp AI',
    lead: 'Thêm nhà cung cấp AI, chọn model, và chỉnh độ dài ngữ cảnh — lượng chữ model đọc được trong một lần.',
    when: ['Lần đầu dùng, chưa có nhà cung cấp nào', 'Đổi khoá API hoặc thêm nhà cung cấp mới'],
    avoid: ['Chỉ muốn đổi model cho một trợ lý — /model nhanh hơn'],
    params: {},
    ex: [{ vals: {}, text: 'Mở bảng nhà cung cấp AI.' }],
    after: ['Mở bảng nhà cung cấp, model và độ dài ngữ cảnh', 'Lưu xong là dùng được ngay'],
    log: [],
    errs: [{ msg: 'Cách dùng: /config', fix: 'Lệnh này không nhận thêm gì phía sau — chỉ gõ /config.' }],
    tip: 'Trên web, việc này nằm ở Cài đặt › Nhà cung cấp AI, có sẵn nút đăng nhập ChatGPT.',
    rel: ['model'],
    kw: 'cai dat nha cung cap provider api key khoa ngu canh context',
  },
  diag: {
    title: 'Khám sức khoẻ truyện',
    lead: 'Mở bảng chẩn đoán: tiến độ, các tuyến truyện còn bỏ ngỏ, và những điểm chất lượng cần để ý.',
    when: ['Viết đã lâu, muốn biết truyện có đang đi đúng hướng', 'Trước khi xuất bản hoặc mở lại truyện'],
    avoid: ['Truyện mới có vài chương — chưa đủ dữ liệu để đánh giá'],
    params: {},
    ex: [{ vals: {}, text: 'Xem bảng chẩn đoán của truyện đang mở.' }],
    after: ['Bảng chẩn đoán mở ra', 'Truyện vẫn tiếp tục viết trong lúc bạn xem'],
    log: [], errs: [],
    tip: 'Thấy nhiều tuyến truyện bỏ ngỏ? Dùng /cocreate để cùng Kiến trúc sư sắp xếp lại giai đoạn tới.',
    rel: ['cocreate', 'review'],
    kw: 'chan doan suc khoe chat luong tien do kiem tra',
  },
  review: {
    title: 'Bật/tắt duyệt từng chương',
    lead: 'Khi bật, Noverse dừng lại sau mỗi chương để bạn đọc trước. Bạn hài lòng thì gõ /next để cho viết tiếp.',
    when: ['Muốn kiểm soát chặt từng chương', 'Những chương quan trọng: mở đầu, cao trào, kết tập'],
    avoid: ['Muốn Noverse viết liền mạch qua đêm — để off'],
    params: {
      mode: {
        label: 'Chế độ',
        desc: 'Chọn một trong hai.',
        init: 'on',
        optLabels: { on: 'on · duyệt', off: 'off · tự viết' },
      },
    },
    ex: [
      { vals: { mode: 'on' }, text: 'Từ chương tới, dừng lại chờ bạn duyệt.' },
      { vals: { mode: 'off' }, text: 'Quay lại viết liền mạch.' },
    ],
    after: ['Chế độ mới được ghi lại ngay', 'Bật: viết xong chương hiện tại, trợ lý dừng và chờ /next', 'Tắt khi đang dừng: gõ tiếp tục để chạy lại'],
    log: [{ cat: 'SYSTEM', text: 'Đã chuyển sang chế độ Duyệt từng chương' }],
    errs: [{ msg: 'Cách dùng: /review on|off', fix: 'Thiếu hoặc sai giá trị. Gõ đúng on hoặc off.' }],
    tip: 'Trên web, nút Duyệt từng chương ở thanh trên Workspace làm đúng việc này.',
    rel: ['next', 'diag'],
    kw: 'duyet nghiem thu tung chuong dung lai kiem soat',
  },
  next: {
    title: 'Cho viết chương tiếp theo',
    lead: 'Dùng khi đang bật duyệt từng chương: bạn đọc xong chương vừa viết và đồng ý cho Noverse viết thêm một chương.',
    when: ['Đã đọc và hài lòng với chương vừa viết', 'Đang bật /review on'],
    avoid: ['Trợ lý vẫn đang viết — lệnh sẽ bị từ chối', 'Muốn sửa chương trước — gõ góp ý vào ô chat thay vì /next'],
    params: {},
    ex: [{ vals: {}, text: 'Duyệt chương vừa xong và viết tiếp một chương.' }],
    after: ['Chương vừa viết được đánh dấu đã duyệt', 'Người viết bắt đầu chương kế tiếp, rồi lại dừng chờ bạn'],
    log: [{ cat: 'SYSTEM', text: 'Đã duyệt chương 24 · bắt đầu viết chương 25' }],
    errs: [
      { msg: 'Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /next', fix: 'Trợ lý vẫn đang viết. Chờ chương xong rồi gõ lại.' },
      { msg: 'Cho phép viết chương tiếp theo thất bại: …', fix: 'Đọc chi tiết trong nhật ký. Nếu lặp lại, chạy /diag.' },
    ],
    tip: 'Trên web, nút Duyệt & viết tiếp ở cuối Trình đọc chính là /next.',
    rel: ['review', 'diag'],
    kw: 'tiep theo duyet chuong tiep viet tiep',
  },
  start: {
    title: 'Tạo truyện mới từ tệp có sẵn',
    lead: 'Bạn đã có sẵn thiết lập nhân vật, thế giới hoặc đại cương trong một tệp? /start đọc tệp đó và bắt đầu một truyện mới từ nó.',
    when: ['Đã viết sẵn thiết lập hoặc đại cương', 'Muốn dùng lại mẫu thiết lập cho nhiều truyện'],
    avoid: ['Đang mở một truyện — lệnh chỉ chạy ở màn hình chào mừng', 'Có cả bộ truyện đã viết — dùng /import'],
    params: {
      path: {
        label: 'Tệp thiết lập',
        desc: 'Tệp văn bản chứa thiết lập hoặc đại cương. Tên có khoảng trắng thì đặt trong ngoặc kép.',
        ph: 'ideas/thiet-lap.md',
        init: 'ideas/thiet-lap.md',
      },
    },
    ex: [
      { vals: { path: 'ideas/thiet-lap.md' }, text: 'Tạo truyện mới từ tệp thiết lập.' },
      { vals: { path: '"ideas/y tuong moi.txt"' }, text: 'Tên tệp có khoảng trắng: đặt trong ngoặc kép.' },
    ],
    after: ['Đọc tệp và chuẩn bị yêu cầu sáng tác', 'Kiến trúc sư lên dàn ý, rồi Người viết bắt đầu chương 1'],
    log: [{ cat: 'SYSTEM', text: 'Đang tạo truyện mới từ ideas/thiet-lap.md' }],
    errs: [
      { msg: 'Lệnh /start chỉ có thể dùng ở màn hình chào mừng', fix: 'Đang mở một truyện rồi. Bấm Truyện mới ở thanh trên.' },
      { msg: 'Cách dùng: /start <đường dẫn tệp thiết lập hoặc đại cương>', fix: 'Bạn quên ghi đường dẫn tệp.' },
    ],
    tip: 'Trên web, màn Truyện mới cho phép kéo thả tệp thiết lập — không cần gõ đường dẫn.',
    rel: ['import'],
    kw: 'bat dau tao truyen moi thiet lap dai cuong file tep',
  },
  import: {
    title: 'Nhập truyện có sẵn để viết tiếp',
    lead: 'Đưa một bộ truyện đã viết vào Noverse. Hệ thống đọc, chia chương, ghi lại nhân vật và cốt truyện để có thể viết tiếp đúng mạch.',
    when: ['Có truyện viết dở ở nơi khác', 'Muốn Noverse học mạch truyện cũ trước khi viết phần mới'],
    avoid: ['Trợ lý đang viết — lệnh sẽ bị từ chối', 'Chỉ có thiết lập, chưa có chương — dùng /start'],
    params: {
      path: {
        label: 'Tệp truyện',
        desc: 'Tệp truyện cần nhập. Để trống để tiếp tục lần nhập đang dở.',
        ph: 'nhap/tra-xanh.txt',
        init: 'nhap/tra-xanh.txt',
        defText: 'Trống: tiếp tục lần nhập đang dở',
      },
      yes: {
        label: 'Tự đồng ý cách chia chương',
        desc: 'Bỏ bước xem trước, tự đồng ý cách chia chương hệ thống đề xuất.',
        hint: 'Không hỏi lại cách chia chương',
      },
      story: {
        label: 'Truyện gốc',
        desc: 'Cho biết truyện gốc đã kết thúc hay còn dang dở. Bỏ trống thì hệ thống hỏi khi cần.',
        none: 'Để hỏi',
        optLabels: { open: 'Còn dở', closed: 'Đã kết' },
      },
      cont: {
        label: 'Viết tiếp ngay',
        desc: 'Nhập xong thì bắt đầu viết tiếp, không cần lệnh khác.',
        hint: 'Nhập xong thì viết tiếp luôn',
      },
      guide: {
        label: 'Hướng dẫn chia chương',
        desc: 'Nói bằng lời thường cách chia chương. Luôn đặt cuối cùng — mọi chữ phía sau đều được tính là hướng dẫn.',
        ph: 'Hồi thứ X cũng là chương độc lập',
      },
    },
    ex: [
      { vals: { path: 'nhap/tra-xanh.txt' }, text: 'Nhập và xem trước cách chia chương trước khi đồng ý.' },
      { vals: { path: 'nhap/tra-xanh.txt', yes: true, story: 'open', cont: true }, text: 'Nhập, tự đồng ý, báo truyện còn dở và viết tiếp ngay.' },
      { vals: { path: '', guide: 'Hồi thứ X cũng là chương độc lập' }, text: 'Chia lại chương cho lần nhập đang dở theo hướng dẫn của bạn.' },
      { vals: { path: '' }, text: 'Tiếp tục lần nhập bị dừng giữa chừng.' },
    ],
    after: ['Đọc tệp và đề xuất cách chia chương', 'Nhấn y để đồng ý, hoặc Esc rồi chạy lại kèm --guide để chia khác', 'Ghi lại nhân vật, tuyến truyện và tóm tắt từng chương'],
    log: [{ cat: 'SYSTEM', text: 'Đã chia 42 chương từ nhap/tra-xanh.txt · nhấn y để xác nhận' }],
    errs: [
      { msg: '--story chỉ nhận giá trị open hoặc closed: "mo"', fix: 'Gõ --story=open hoặc --story=closed.' },
      { msg: 'chỉ nhận một đường dẫn tệp nguồn: thừa "moi.txt"', fix: 'Mỗi lần nhập một tệp. Tên tệp có khoảng trắng thì đổi tên cho liền, ví dụ tra-xanh-moi.txt.' },
      { msg: 'tùy chọn không hợp lệ "--auto"', fix: 'Chỉ có --yes, --story=, --continue và --guide=.' },
      { msg: '--guide cần kèm hướng dẫn ngôn ngữ tự nhiên…', fix: 'Ghi hướng dẫn ngay sau dấu =.' },
    ],
    tip: 'Truyện dài? Lần đầu đừng dùng --yes — xem cách chia chương trước sẽ đỡ phải nhập lại.',
    rel: ['start', 'sync', 'simulate'],
    kw: 'nhap truyen cu co san viet tiep import file',
  },
  reopen: {
    title: 'Mở lại truyện đã xong để viết tiếp',
    lead: 'Truyện đã hoàn thành nhưng bạn muốn viết thêm? /reopen mở lại truyện để viết tiếp. Có thể ghi kèm hướng bạn muốn, trợ lý sẽ cân nhắc trước khi lên kế hoạch.',
    when: ['Muốn viết thêm tập mới sau kết thúc', 'Kết thúc chưa ưng, muốn kéo dài mạch truyện'],
    avoid: ['Truyện chưa hoàn thành — không cần mở lại', 'Còn chương sửa tay chưa tiếp nhận — chạy /sync trước'],
    params: {
      dir: {
        label: 'Hướng viết tiếp',
        desc: 'Mô tả bằng lời thường. Bỏ trống thì trợ lý tự lên hướng đi.',
        ph: 'Viết thêm tập 2: nữ chính trở về quê',
        init: 'Viết thêm tập 2: nữ chính trở về quê',
      },
    },
    ex: [
      { vals: { dir: '' }, text: 'Mở lại, để trợ lý tự lên hướng đi.' },
      { vals: { dir: 'Viết thêm tập 2: nữ chính trở về quê' }, text: 'Mở lại và gợi ý hướng cho tập mới.' },
    ],
    after: ['Truyện chuyển từ Hoàn thành sang Đang viết', 'Hướng bạn ghi (nếu có) được xét trước khi lên kế hoạch', 'Kiến trúc sư lên dàn ý tập tiếp theo rồi viết tiếp'],
    log: [{ cat: 'SYSTEM', text: 'Đã mở lại truyện để viết tiếp' }],
    errs: [
      { msg: 'Phát hiện chương đã bị sửa ngoài ứng dụng: [3]; hãy chạy /sync trước', fix: 'Chạy /sync để tiếp nhận chỗ sửa, rồi /reopen lại.' },
      { msg: 'Đang đồng sáng tác, hãy kết thúc trước', fix: 'Kết thúc phiên đồng sáng tác rồi thử lại.' },
    ],
    tip: 'Muốn sửa lại vài chương thay vì viết thêm? Chỉ cần nói trong ô chat, ví dụ “viết lại chương 12”.',
    rel: ['sync', 'cocreate', 'export'],
    kw: 'mo lai viet them tap moi hoan thanh ket thuc',
  },
  cocreate: {
    title: 'Cùng bàn hướng đi giai đoạn tới',
    lead: 'Tạm dừng viết để bạn và Kiến trúc sư trò chuyện, chốt hướng đi cho giai đoạn tiếp theo, rồi mới viết tiếp.',
    when: ['Sắp sang tập hoặc giai đoạn mới', 'Thấy mạch truyện lệch, muốn chỉnh trước khi viết thêm'],
    avoid: ['Truyện đã hoàn thành — dùng /reopen', 'Chưa có truyện nào đang viết'],
    params: {},
    ex: [
      { vals: {}, text: 'Tạm dừng và vào phòng đồng sáng tác.' },
      { vals: {}, slash: '/plan', text: 'Tên ngắn, làm đúng việc như /cocreate.' },
    ],
    after: ['Trợ lý viết xong bước đang dở rồi tạm dừng', 'Bạn và Kiến trúc sư trao đổi trong ô chat', 'Chốt xong, Noverse viết tiếp theo hướng mới'],
    log: [{ cat: 'SYSTEM', text: 'Đã vào đồng sáng tác giai đoạn' }],
    errs: [
      { msg: 'Đồng sáng tác giai đoạn chỉ khả dụng khi đang viết truyện', fix: 'Mở hoặc tạo một truyện trước.' },
      { msg: 'Không thể vào đồng sáng tác: truyện đã hoàn thành hoặc đang trong đồng sáng tác', fix: 'Truyện đã xong thì dùng /reopen. Đang đồng sáng tác rồi thì cứ tiếp tục trò chuyện.' },
    ],
    tip: 'Ô chat trống, nhấn Tab cũng chuyển nhanh giữa chế độ Nhanh và Đồng sáng tác.',
    rel: ['reopen', 'diag'],
    kw: 'dong sang tac ban bac quy hoach ke hoach plan huong di',
  },
  simulate: {
    title: 'Học văn phong từ văn mẫu',
    lead: 'Thêm vài đoạn văn bạn thích vào Văn mẫu (Tuỳ chỉnh › Cách viết › Văn phong của bạn). Noverse đọc chúng rồi tạo hoặc cập nhật hồ sơ văn phong để các chương sau viết theo giọng đó.',
    when: ['Muốn truyện mang giọng văn riêng', 'Vừa thêm văn mẫu mới'],
    avoid: ['Chưa có văn mẫu nào', 'Trợ lý đang viết — lệnh sẽ bị từ chối'],
    params: {},
    ex: [{ vals: {}, text: 'Đọc các văn mẫu và tạo hồ sơ văn phong.' }],
    after: ['Đọc mọi văn mẫu bạn đã thêm', 'Tạo hoặc cập nhật hồ sơ văn phong', 'Các chương viết sau sẽ theo giọng văn này'],
    log: [{ cat: 'SYSTEM', text: 'Đã cập nhật hồ sơ văn phong từ 6 văn mẫu' }],
    errs: [{ msg: 'Khởi động mô phỏng văn phong thất bại: …', fix: 'Thường do chưa có văn mẫu. Thêm ít nhất một đoạn văn mẫu rồi chạy lại.' }],
    tip: 'Văn mẫu càng đa dạng cảnh (đối thoại, miêu tả, hành động) thì hồ sơ càng sát giọng.',
    rel: ['importsim'],
    kw: 'van phong giong van mo phong van mau hoc',
  },
  importsim: {
    title: 'Nhập hồ sơ văn phong có sẵn',
    lead: 'Dùng lại hồ sơ văn phong đã tạo ở truyện khác hoặc do người khác gửi. Nếu hồ sơ được tạo từ cùng bộ văn mẫu, Noverse gộp vào hồ sơ hiện có thay vì tạo bản trùng.',
    when: ['Muốn nhiều truyện dùng chung một giọng văn', 'Được chia sẻ một hồ sơ văn phong'],
    avoid: ['Chưa có tệp hồ sơ — dùng /simulate để tạo'],
    params: {
      path: {
        label: 'Tệp hồ sơ',
        desc: 'Tệp hồ sơ văn phong, thường có đuôi .json.',
        ph: 'van-phong/co-trang.json',
        init: 'van-phong/co-trang.json',
      },
    },
    ex: [{ vals: { path: 'van-phong/co-trang.json' }, text: 'Nhập hồ sơ văn phong cổ trang.' }],
    after: ['Đọc tệp hồ sơ', 'Gộp vào hồ sơ cùng nguồn, hoặc thêm hồ sơ mới'],
    log: [{ cat: 'SYSTEM', text: 'Đã nhập hồ sơ văn phong co-trang.json' }],
    errs: [{ msg: 'Nhập hồ sơ mô phỏng thất bại: …', fix: 'Kiểm tra lại đường dẫn và tệp có đúng là hồ sơ văn phong.' }],
    tip: 'Hồ sơ văn phong là tệp nhỏ, gửi qua tin nhắn được — tiện để cả nhóm viết cùng một giọng.',
    rel: ['simulate'],
    kw: 'nhap ho so van phong profile json',
  },
  sync: {
    title: 'Nhận chỉnh sửa bạn tự làm',
    lead: 'Bạn sửa trực tiếp tệp chương bằng một ứng dụng khác? /sync giúp Noverse đọc lại, cập nhật tóm tắt và sự thật cốt truyện, đồng thời báo nếu chỗ sửa mâu thuẫn với các chương sau.',
    when: ['Vừa sửa tay một vài chương', 'Trước khi /reopen hoặc viết tiếp sau khi sửa'],
    avoid: ['Trợ lý đang viết — lệnh sẽ bị từ chối'],
    params: {
      check: {
        label: 'Chỉ kiểm tra',
        desc: 'Chỉ liệt kê chương nào đã bị sửa, chưa tiếp nhận gì.',
        hint: 'Chỉ liệt kê chương đã bị sửa',
        defText: 'Không ghi: tiếp nhận luôn mọi chỗ sửa',
      },
    },
    ex: [
      { vals: { check: true }, text: 'Xem chương nào đã bị sửa, chưa thay đổi gì.' },
      { vals: {}, text: 'Tiếp nhận mọi chỗ sửa và cập nhật cốt truyện.' },
    ],
    after: ['So từng chương với bản Noverse đã lưu', 'Tóm tắt lại chỗ sửa, cập nhật sự thật cốt truyện', 'Báo các mâu thuẫn ở chương sau (nếu có)'],
    log: [{ cat: 'SYSTEM', text: 'Đã tiếp nhận tu chỉnh các chương: [3 7]; Chương 3: đổi tên quán trà (sự thật cốt truyện đã cập nhật); Chương 7: thêm cảnh mưa (phát hiện 1 xung đột phía sau)' }],
    errs: [
      { msg: 'tham số không hợp lệ "--dry" (hỗ trợ: --check)', fix: 'Chỉ có --check, hoặc không ghi gì.' },
      { msg: 'Không phát hiện chương nào bị chỉnh sửa từ bên ngoài', fix: 'Không phải lỗi — không có gì cần tiếp nhận.' },
    ],
    tip: 'Sửa trong Trình đọc trên web thì không cần /sync — chỉ cần khi bạn sửa bằng ứng dụng khác.',
    rel: ['diag', 'reopen'],
    kw: 'dong bo sua tay chinh sua ngoai cap nhat',
  },
  export: {
    title: 'Xuất truyện ra văn bản, ebook hoặc bản đọc TTS',
    lead: 'Gom các chương đã hoàn thành thành một tệp để đọc, gửi hoặc đăng. Đuôi .txt cho văn bản thường, .epub cho sách điện tử. Gõ narration để có bản đọc TTS: không tiêu đề chương, không in nghiêng hay in đậm, các chương cách nhau hai dòng trống.',
    when: ['Muốn đọc truyện trên điện thoại hoặc máy đọc sách', 'Gửi bản thảo cho người khác', 'Sao lưu một đoạn chương'],
    avoid: ['Cần audio — dùng màn Audio'],
    params: {
      path: {
        label: 'Lưu vào',
        desc: 'Tên tệp hoặc thư mục. Đuôi tệp quyết định định dạng: .txt hoặc .epub. Gõ txt, epub hoặc narration để chọn định dạng mà không cần đặt tên.',
        ph: 'exports/tra-xanh.epub',
        init: 'exports/tra-xanh.epub',
        defText: 'Trống: lưu vào thư mục truyện, đặt tên theo tên truyện, dạng .txt',
      },
      from: { label: 'Từ chương', desc: 'Chương đầu tiên cần xuất.', ph: '1', init: '1', defText: 'Trống: từ chương đầu' },
      to: {
        label: 'Đến chương',
        desc: 'Chương cuối cùng cần xuất. Chương chưa viết xong trong khoảng sẽ được bỏ qua và báo lại.',
        ph: '24', init: '30', defText: 'Trống: tới chương mới nhất đã xong',
      },
      format: {
        label: 'Định dạng',
        desc: 'Chọn định dạng khi không đặt tên tệp: văn bản, ebook hoặc bản đọc TTS.',
        optLabels: { txt: 'Văn bản .txt', epub: 'Ebook .epub', narration: 'Bản đọc TTS' },
        defText: 'Trống: theo đuôi tệp, không có đuôi thì dùng .txt',
      },
      ow: {
        label: 'Ghi đè tệp cũ',
        desc: 'Cho phép thay tệp trùng tên. Không ghi cờ nào thì gặp tệp đã có sẽ dừng lại để không mất bản cũ.',
        hint: 'Thay tệp trùng tên nếu đã có',
      },
      keep: {
        label: 'Giữ cả hai',
        desc: 'Gặp tệp trùng tên thì giữ bản cũ, lưu bản mới với ngày giờ thêm vào tên. Không dùng chung với --overwrite.',
        hint: 'Thêm ngày giờ vào tên tệp mới',
      },
      docso: {
        label: 'Đọc số thành chữ',
        desc: 'Viết số thành chữ để giọng đọc AI đọc đúng, ví dụ 18 thành mười tám.',
        hint: 'Chỉ cho bản đọc TTS',
      },
    },
    ex: [
      { vals: { path: '', from: '', to: '' }, text: 'Xuất mọi chương đã xong thành tệp .txt trong thư mục truyện.' },
      { vals: { path: 'exports/tra-xanh.epub', from: '', to: '' }, text: 'Xuất ebook EPUB để đọc trên điện thoại.' },
      { vals: { path: 'exports/', from: '1', to: '12' }, text: 'Xuất chương 1–12 vào thư mục exports.' },
      { vals: { path: 'exports/tra-xanh.epub', from: '', to: '', ow: true }, text: 'Xuất lại, thay bản EPUB cũ.' },
      { vals: { path: 'exports/tra-xanh.epub', from: '', to: '', keep: true }, text: 'Xuất lại nhưng giữ bản cũ, bản mới có thêm ngày giờ trong tên.' },
      { vals: { path: 'narration', from: '', to: '', docso: true }, text: 'Xuất bản đọc TTS, viết số thành chữ.' },
    ],
    after: ['Gom các chương đã hoàn thành trong khoảng bạn chọn', 'Định dạng theo đuôi tệp (.txt, .epub) hoặc theo chữ narration', 'Ghi xong mới thay tệp, nên lỡ lỗi giữa chừng cũng không hỏng bản cũ'],
    log: [
      { cat: 'Hệ thống', text: 'Đang tiến hành xuất bản...' },
      { cat: 'Hệ thống', text: '✓ Đã xuất 24 chương (1,2 MB) vào exports/tra-xanh.epub (bỏ qua 6 chương chưa hoàn thành: 25…30)' },
    ],
    errs: [
      { msg: 'tệp đã tồn tại: exports/tra-xanh.epub (dùng --overwrite để ghi đè)', fix: 'Thêm --overwrite để thay, --keep để giữ cả hai, hoặc đổi tên tệp.' },
      { msg: 'chỉ dùng một trong --overwrite hoặc --keep', fix: 'Bỏ một trong hai cờ.' },
      { msg: 'khoảng chương không hợp lệ: from=30 > to=12', fix: 'Số from phải nhỏ hơn hoặc bằng to.' },
      { msg: 'không suy ra được định dạng từ đuôi ".pdf" (hỗ trợ .txt / .epub)', fix: 'Chỉ xuất được .txt hoặc .epub.' },
      { msg: 'khoảng 25..30 không có chương hoàn thành', fix: 'Các chương này chưa viết xong. Chọn khoảng khác.' },
    ],
    tip: 'Ghi thư mục (kết thúc bằng /) thay vì tên tệp, Noverse tự đặt tên theo tên truyện.',
    rel: ['sync', 'reopen'],
    kw: 'xuat tai xuong luu file epub txt ebook narration tts ban doc doc so in gui ban thao',
  },
}

/** Chữ dùng chung của popover `/` và trang Hướng dẫn lệnh. */
export const CMD_UI = {
  helpButton: 'Hướng dẫn',
  helpTitle: 'Hướng dẫn lệnh',
  listSub: (n: number) => `${n} lệnh · gõ `,
  listSubAfter: ' trong ô chat để dùng',
  search: 'Tìm lệnh hoặc việc muốn làm…',
  searchAria: 'Tìm lệnh',
  introNav: 'Cách dùng lệnh',
  introNavSub: 'Đọc cú pháp, phím tắt, khi nào bị từ chối',
  noMatch: (q: string) => `Không thấy lệnh nào khớp “${q}”.`,
  tryLabel: 'Thử gõ việc bạn muốn làm, ví dụ:',
  tryWords: ['xuất epub', 'đổi model', 'nhập truyện', 'sửa tay'],

  // Nhãn trạng thái lệnh
  auto: 'Chạy ngay khi chọn',
  autoLong: 'Chạy ngay khi chọn',
  manual: 'Cần gõ thêm',
  manualLong: 'Cần gõ thêm thông tin',
  idle: 'Cần rảnh',
  idleLong: 'Cần hệ thống rảnh',
  usableBusy: 'Dùng được cả khi đang viết',
  alias: (name: string) => `Tên khác: /${name}`,
  aliasShort: (name: string) => `Tên khác /${name}`,
  /** `/reopen` với tài khoản không có quyền viết truyện đang mở (13-commands.md §/reopen). */
  needsWrite: 'Cần quyền viết',
  only: { welcome: 'Chỉ ở màn hình chào mừng', running: 'Chỉ khi đang viết' } as Record<string, string>,

  // Intro
  introEyebrow: 'Bắt đầu từ đây',
  introTitle: 'Cách dùng lệnh',
  introLeadBefore: 'Lệnh là cách nhanh để bảo Noverse làm đúng một việc — xuất truyện, đổi model, nhập truyện có sẵn… Mọi lệnh đều bắt đầu bằng dấu ',
  introLeadSlash: '/',
  introLeadAfter: ' và được gõ ngay trong ô chat ở Workspace.',
  introSteps: [
    { title: 'Gõ dấu / trong ô chat', text: 'Danh sách lệnh hiện ra ngay phía trên ô chat.' },
    { title: 'Gõ vài chữ đầu để lọc', text: 'Ví dụ /ex chỉ còn /export. Dùng ↑ ↓ để chọn.' },
    { title: 'Nhấn Tab hoặc Enter để điền', text: 'Lệnh “chạy ngay” sẽ chạy luôn. Lệnh cần thêm thông tin sẽ điền sẵn để bạn gõ tiếp.' },
    { title: 'Nhấn Enter để chạy', text: 'Kết quả hiện trong nhật ký. Sai cú pháp thì Noverse nói rõ cần sửa gì.' },
  ],
  syntaxTitle: 'Đọc cú pháp trong 30 giây',
  syntaxNote: 'Ngoặc chỉ để chú thích — khi gõ thật thì bỏ ngoặc đi',
  legend: [
    { tok: '/export', tone: 'name', means: 'Tên lệnh, luôn có dấu / ở đầu', ex: '/export' },
    { tok: '<tệp>', tone: 'value', means: 'Bắt buộc ghi, thay bằng giá trị thật', ex: '/start ideas/thiet-lap.md' },
    { tok: '[tệp]', tone: 'optional', means: 'Có thể bỏ qua', ex: '/export   hoặc   /export exports/' },
    { tok: 'on|off', tone: 'value', means: 'Chọn một trong các giá trị', ex: '/review on' },
    { tok: '--overwrite', tone: 'flag', means: 'Công tắc: ghi vào là bật', ex: '/export --overwrite' },
    { tok: 'from=N', tone: 'flag', means: 'Tên=giá trị, viết liền không cách', ex: '/export from=1 to=12' },
  ] as const,
  kindAuto: { title: 'Chạy ngay khi chọn', text: 'Chọn trong danh sách là chạy, không cần gõ thêm. Muốn thêm tuỳ chọn thì gõ tiếp trước khi nhấn Enter.' },
  kindManual: { title: 'Cần gõ thêm thông tin', text: 'Noverse điền sẵn tên lệnh, bạn gõ tiếp tệp, giá trị hoặc hướng dẫn rồi mới nhấn Enter.' },
  deniesTitle: 'Khi lệnh bị từ chối',
  denies: [
    { title: 'Hệ thống đang bận viết', msg: 'Lệnh chỉ có thể thực hiện khi hệ thống đang rảnh: /import', fix: 'Các lệnh có nhãn “Cần rảnh” phải chờ trợ lý viết xong. Chờ một lát hoặc tạm dừng trước.' },
    { title: 'Gõ sai tên lệnh', msg: 'Lệnh không xác định: /exprt', fix: 'Gõ / để xem đúng tên. Tên khác như /plan vẫn dùng được.' },
    { title: 'Sai hoặc thiếu thông tin', msg: 'Cách dùng: /review on|off', fix: 'Noverse in lại cú pháp đúng. So với mục Cú pháp của lệnh đó trong trang này.' },
    { title: 'Không đúng lúc', msg: 'Lệnh /start chỉ có thể dùng ở màn hình chào mừng', fix: 'Một số lệnh chỉ dùng được ở màn hình nhất định — xem nhãn ngay dưới tên lệnh.' },
  ],
  tasksTitle: 'Bạn muốn…',
  tasks: [
    { label: 'Xuất truyện ra ebook', cmd: '/export', key: 'export' },
    { label: 'Nhập truyện cũ để viết tiếp', cmd: '/import', key: 'import' },
    { label: 'Đổi model cho Người viết', cmd: '/model writer', key: 'model' },
    { label: 'Dừng sau mỗi chương để duyệt', cmd: '/review on', key: 'review' },
    { label: 'Báo Noverse chương mình vừa sửa tay', cmd: '/sync', key: 'sync' },
    { label: 'Bàn hướng đi cho tập sau', cmd: '/cocreate', key: 'cocreate' },
    { label: 'Viết theo giọng văn mình thích', cmd: '/simulate', key: 'simulate' },
    { label: 'Viết thêm sau khi truyện đã xong', cmd: '/reopen', key: 'reopen' },
  ],

  // Chi tiết lệnh
  whenTitle: 'Dùng khi',
  avoidTitle: 'Chưa nên dùng khi',
  syntaxHeading: 'Cú pháp',
  noParams: 'Lệnh này không cần thêm gì phía sau. Gõ đúng tên lệnh rồi nhấn Enter.',
  required: 'bắt buộc',
  optional: 'có thể bỏ',
  examplesTitle: 'Ví dụ',
  copy: 'Chép',
  copied: 'Đã chép',
  copyAria: 'Sao chép lệnh',
  copyFailed: 'Không chép được — trình duyệt chặn bộ nhớ tạm. Hãy chọn dòng lệnh rồi chép tay.',
  tryIt: 'Thử bên phải',
  afterTitle: 'Sau khi chạy',
  logTitle: 'Nhật ký sẽ hiện',
  errsTitle: 'Lỗi hay gặp & cách sửa',
  tipTitle: 'Mẹo',
  relTitle: 'Lệnh liên quan',

  // Bộ ghép lệnh
  builderTitle: 'Ghép lệnh thử',
  reset: 'Đặt lại',
  retry: 'Thử lại',
  builderEmpty: 'Không có gì để chọn — lệnh này chạy nguyên như vậy.',
  yourLine: 'Lệnh của bạn',
  fillWorkspace: 'Điền vào ô chat Workspace',
  openNewStory: 'Mở Truyện mới & điền sẵn',
  fixFirst: 'Sửa lỗi phía trên trước',
  waitIdle: 'Đợi hệ thống rảnh',
  needStory: 'Mở một truyện trước',
  okLine: 'Đúng cú pháp — dán vào ô chat và nhấn Enter.',
  noValue: 'Không ghi',

  // Phím tắt
  keysTitle: 'Phím tắt',
  keys: [
    {
      label: 'Ô chat',
      rows: [
        { what: 'Mở danh sách lệnh', keys: ['/'] },
        { what: 'Chọn lệnh', keys: ['↑', '↓'] },
        { what: 'Điền lệnh đã chọn', keys: ['Tab'] },
        { what: 'Chạy lệnh', keys: ['Enter'] },
        { what: 'Đóng danh sách', keys: ['Esc'] },
      ],
    },
    {
      label: 'Mọi màn hình trên web',
      rows: [
        { what: 'Tìm & chạy lệnh nhanh', keys: ['⌘', 'K'] },
        { what: 'Mở trang hướng dẫn này', keys: ['?'] },
      ],
    },
    {
      label: 'Bản dòng lệnh (Terminal)',
      rows: [
        { what: 'Đổi chế độ Nhanh / Đồng sáng tác', keys: ['Tab'] },
        { what: 'Chế độ chọn chữ để sao chép', keys: ['Ctrl', 'R'] },
        { what: 'Lưu & thoát (nhấn 2 lần)', keys: ['Ctrl', 'C'] },
      ],
    },
  ],
  footerNoteBefore: 'Mở lại trang này bất cứ lúc nào: gõ ',
  footerNoteSlash: '/help',
  footerNoteAfter: ', bấm ',
  footerNoteKey: '?',
  footerNoteTail: ' hoặc nút Hướng dẫn ở góc trên.',

  // Popover
  popAria: 'Gợi ý lệnh',
  popListTitle: 'Lệnh',
  popListSub: (n: number) => `${n} lệnh · gõ tiếp để lọc`,
  popFilterTitle: (q: string) => `Lệnh khớp “/${q}”`,
  popArgsTitle: (name: string) => `Tuỳ chọn cho /${name}`,
  popUnknownTitle: (name: string) => `Không có lệnh /${name}`,
  popUnknownSub: 'Gửi nguyên như vậy sẽ bị báo “Lệnh không xác định”',
  recent: 'Dùng gần đây',
  matches: (n: number) => `${n} lệnh khớp`,
  canAdd: 'Có thể thêm',
  valuesFor: (prefix: string) => `Giá trị cho ${prefix}`,
  didYouMean: 'Có phải bạn muốn',
  emptyList: (q: string) => `Không có lệnh bắt đầu bằng /${q}`,
  emptyListHint: 'Nhấn Enter sẽ gửi nguyên dòng này cho trợ lý như một lời nhắn.',
  emptyTyping: (label: string) => `Đang gõ ${label.toLowerCase()}`,
  emptyTypingHint: 'Gõ bằng lời thường, bao nhiêu chữ cũng được. Xong thì nhấn Enter.',
  emptyDone: 'Đã đủ thông tin',
  emptyDoneHint: 'Không còn tuỳ chọn nào để thêm. Nhấn Enter để chạy.',
  slotTitle: (label: string, required: boolean) => `Đang nhập: ${label} · ${required ? 'bắt buộc' : 'có thể bỏ'}`,
  enterRun: 'chạy',
  enterFill: 'điền & gõ tiếp',
  enterSend: 'gửi như lời nhắn',
  enterBlocked: 'chưa chạy được',
  enterCommand: 'chạy lệnh',
  enterFix: (name: string) => `sửa thành /${name}`,
  keysHint: { pick: 'chọn', fill: 'điền', close: 'đóng', closeDraft: 'đóng gợi ý, giữ chữ đã gõ' },
  fullHelp: 'Hướng dẫn đầy đủ',
  tagIdle: 'Cần rảnh',
  tagSuggestion: 'gợi ý',
  tagSwitch: 'công tắc',
  sentToast: 'Đã chạy',
  /** Chú thích dòng gợi ý đường dẫn (Slash.dc.html › PATHS) — suy từ chính đường dẫn đã dùng. */
  recentPathNote: (command: string, path: string): string => {
    if (command === 'export') {
      if (path.endsWith('/')) return `Thư mục ${path.replace(/\/+$/, '')} · tự đặt tên`
      const ext = /\.([^./]+)$/.exec(path)?.[1]
      return ext ? `Lần xuất trước · ${ext.toUpperCase()}` : 'Lần xuất trước'
    }
    if (command === 'importsim') return 'Hồ sơ đã dùng ở truyện khác'
    const dir = path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : ''
    return dir ? `Tệp trong thư mục ${dir}` : 'Dùng gần đây'
  },
  okSyntax: 'Đúng cú pháp — nhấn Enter để chạy.',
  rejected: 'Sẽ bị từ chối: hệ thống đang viết.',
  busyChip: (n: number) => `Đang viết chương ${n}`,
  busyTitle: (n: number) => `Người viết đang viết chương ${n}`,
  busyIdleText: (n: number) => `Lệnh này cần hệ thống rảnh. Chờ chương ${n} xong, hoặc bấm Tạm dừng ở Workspace rồi chạy.`,
  busyUsableText: 'Lệnh này vẫn dùng được trong lúc trợ lý đang viết.',
  busyWarnBefore: (n: number) => `Người viết đang viết chương ${n}. Lệnh này cần hệ thống rảnh — chờ chương xong, hoặc nhấn `,
  busyWarnAfter: ' hai lần để tạm dừng rồi chạy.',
  unknownTip: (name: string) => `Nếu cứ gửi, Noverse sẽ báo “Lệnh không xác định: /${name}”. Tên khác như /plan vẫn dùng được.`,
  extraValue: (value: string) => `Thừa “${value}” — lệnh này không nhận thêm giá trị đó.`,
  fixEpub: 'Đổi thành .epub',
  examplesHeading: 'Gợi ý cách dùng',
  enterHeading: 'Khi nhấn Enter',
} as const

/** Câu kiểm tra cú pháp phía client — phản chiếu lỗi thật của backend (13 §1.3). */
export const CMD_CHECK = {
  missing: (label: string, token: string) => `Còn thiếu ${label.toLowerCase()} (${token}).`,
  unterminated: (usage: string) => `Cách dùng: ${usage}`,
  pathSpaces: 'Đường dẫn có khoảng trắng sẽ bị tách làm nhiều phần. Đổi tên tệp cho liền, ví dụ tra-xanh-moi.txt.',
  exportRange: (from: number, to: number) => `khoảng chương không hợp lệ: from=${from} > to=${to}`,
  exportExt: (ext: string) => `Không xuất được đuôi ${ext} — chỉ hỗ trợ .txt hoặc .epub.`,
  exportEmpty: (lo: number, hi: number, done: number) =>
    `Khoảng ${lo}..${hi} không có chương hoàn thành (mới xong tới chương ${done}).`,
  exportTarget: (format: string, where: string) => `Sẽ tạo ${format} trong ${where}.`,
  exportRangeInfo: (lo: number, top: number, skippedFrom: number, skippedTo: number | null) =>
    skippedTo === null
      ? `Gồm chương ${lo}–${top}.`
      : `Gồm chương ${lo}–${top} · bỏ qua chương ${skippedFrom}–${skippedTo} vì chưa viết xong.`,
  exportKeep: 'Nếu tệp đã có, lệnh sẽ dừng để giữ bản cũ.',
  exportKept: 'Tệp trùng tên sẽ được viết ra bản mới, không đụng bản cũ.',
  importResume: 'Không ghi tệp: tiếp tục lần nhập đang dở.',
  importPreview: 'Hệ thống sẽ cho xem cách chia chương, nhấn y để đồng ý.',
  importsimExt: 'Hồ sơ văn phong thường là tệp .json — kiểm tra lại tên tệp.',
  startOnly: 'Chỉ chạy ở màn hình chào mừng. Đang mở truyện thì dùng màn Truyện mới.',
  reviewOn: (n: number) => `Viết xong chương ${n}, trợ lý sẽ dừng chờ /next.`,
  reopenState: (title: string) => `“${title}” cần ở trạng thái Hoàn thành mới mở lại được.`,
  syncCheck: 'Chỉ liệt kê, không thay đổi gì.',
  /** Lệnh cần quyền viết nhưng người dùng chỉ đọc truyện này (brief 25/10 › câu hỏi treo `/reopen`). */
  readOnly: 'Bạn chỉ có quyền đọc truyện này',
  busy: (n: number) => `Trợ lý đang viết chương ${n} — lệnh này sẽ bị từ chối cho tới khi hệ thống rảnh.`,
} as const

/** Gợi ý số cho from=/to= lấy từ tiến độ truyện (13 §1.4). */
export const CMD_NUMS = {
  from: (done: number) => [
    { value: '1', note: 'Từ chương đầu' },
    ...(done > 0 ? [{ value: String(done + 1), note: `Tiếp sau lần xuất trước (1–${done})` }] : []),
  ],
  to: (done: number) => (done > 0 ? [{ value: String(done), note: 'Chương mới nhất đã xong' }] : []),
} as const

/** Gợi ý số cho khoá lạ (không có dữ liệu) → rỗng, không bịa. */
export function numSuggestions(key: string, done: number): { value: string; note: string }[] {
  if (key === 'from') return [...CMD_NUMS.from(done)]
  if (key === 'to') return [...CMD_NUMS.to(done)]
  return []
}

/** Nhãn nhóm theo khoá API; khoá lạ rơi về nhóm cuối để không mất dòng. */
export function groupLabel(key: string): string {
  return CMD_GROUPS.find((group) => group.key === key)?.label ?? CMD_GROUPS[CMD_GROUPS.length - 1].label
}

export function copyOf(name: string): CmdCopy | undefined {
  return CMD_COPY[name]
}

/** Tham số chưa có chữ trong i18n → dùng token làm nhãn (không bịa nội dung). */
export function paramCopy(spec: CommandSpec, key: string): CmdParamCopy | undefined {
  return copyOf(spec.name)?.params[key]
}
