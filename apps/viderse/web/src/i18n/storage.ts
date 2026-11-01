// Copy của mục Lưu trữ (V22 + MSettings store=*) — thư mục dự án, khối chọn
// tại chỗ, biểu đồ dung lượng, dọn dẹp và xoá nhóm với xác nhận cam inline.
// Chữ khớp artboard `Settings` · `section=storage` và `MSettings` ·
// `screen=storage` (chỉ thay số liệu sống).
export const storage = {
  project: {
    title: 'Thư mục lưu dự án',
    assets: 'Assets của mọi video: video gốc, ảnh đã vẽ, MP4, project editor · {bytes}',
    assetsM: 'Video gốc, ảnh đã vẽ, MP4, project editor · {bytes}',
    ledeM: 'Chọn thư mục lưu assets của mọi video, xem dung lượng và lấy lại chỗ trống.',
    stayNote: 'Luôn ở máy này: cơ sở dữ liệu, mô hình và công cụ ({bytes} GB) — để app mở nhanh và không hỏng khi rút ổ ngoài.',
    finder: 'Mở trong Finder',
    change: 'Đổi thư mục…',
    changeMobile: 'Đổi thư mục lưu',
    /** Loại ổ trên dòng vị trí (artboard: “Ổ trong máy · còn 212 GB”). */
    kinds: {
      internal: 'Ổ trong máy',
      external: 'Ổ ngoài',
      network: 'Ổ mạng',
      cloud_sync: 'Thư mục đồng bộ đám mây',
    } as Record<string, string>,
    kindFree: '{kind} · còn {free}',
    kindFsFree: '{kind} · {fs} · còn {free}',
    /** Ổ ngoài bị rút: “Ổ ngoài USB · APFS · chưa cắm” (#79). */
    kindFsOffline: '{kind} · {fs} · chưa cắm',
    /** Ổ rút mà máy chủ chưa từng đọc được hệ tệp (mở app khi ổ đã rút). */
    kindOffline: '{kind} · chưa cắm',
    /** Thư mục trong máy biến mất (không phải ổ rút) — không đọc được chỗ trống. */
    kindUnreadable: '{kind} · chưa đọc được chỗ trống',
  },
  picker: {
    title: 'Chọn thư mục dự án',
    titleM: 'Đổi thư mục lưu',
    titleMSub: 'Chọn ổ đang cắm vào máy; Viderse tạo thư mục “Viderse Projects” trong đó.',
    newDirLabel: 'Thư mục mới',
    browse: 'Chọn thư mục…',
    browseHelp: 'Mở hộp chọn thư mục của máy. Chọn gốc một ổ thì Viderse tạo thư mục con “Viderse Projects”; thư mục đã có tệp khác vẫn dùng được, Viderse chỉ ghi vào thư mục con của mình.',
    drivesLabel: 'Gợi ý từ ổ đang cắm:',
    /** Nhãn một ổ trong sheet điện thoại. */
    drive: '{kind} · {fs} · còn {free}',
    cloudRow: 'Không dùng được: thư mục đồng bộ có thể khoá tệp đang dựng.',
    lowspaceHint: 'Chọn thư mục khác hoặc “Chỉ video mới lưu ở thư mục mới”',
    cloudHint: 'Chọn thư mục ngoài {name}',
    roots: 'Ổ đang cắm',
    empty: 'Không có thư mục con.',
    noMatch: 'Không có thư mục nào khớp.',
    filter: 'Lọc tên thư mục',
    filterPlaceholder: 'Lọc thư mục…',
    prevPage: 'Trước',
    nextPage: 'Sau',
    pageOf: '{page} / {pages}',
    back: 'Lên một mục',
    useThis: 'Dùng thư mục này',
    modesLabel: 'Dữ liệu đang có',
    /** Segmented của sheet điện thoại. */
    segMove: 'Chuyển {size} sang',
    segNew: 'Chỉ video mới',
    modes: {
      move: {
        label: 'Chuyển {size} đang có sang thư mục mới',
        hint: 'Khoảng {eta} · bản cũ giữ 7 ngày rồi mới xoá',
        hintNoEta: 'Bản cũ giữ 7 ngày rồi mới xoá',
      },
      new_only: {
        label: 'Chỉ video mới lưu ở thư mục mới',
        hint: '{size} cũ ở lại {volume}, vẫn mở được',
      },
    },
    volumeGone: 'Ổ chưa cắm hoặc đã rút. Cắm lại ổ rồi chọn lại thư mục.',
    cloudWarn: 'Thư mục đồng bộ đám mây (Google Drive, iCloud, Dropbox) không dùng được: tệp đang dựng có thể bị khoá hoặc tải lên giữa chừng.',
    jobsNote: '{n} việc nền đang chạy sẽ tạm dừng trong lúc chuyển và tự chạy tiếp khi xong. Bàn dựng và Duyệt vẫn mở được, chỉ không sửa được video.',
    jobsNoteZero: 'Bàn dựng và Duyệt vẫn mở được trong lúc chuyển, chỉ không sửa được video.',
    /** Chân sheet điện thoại: máy tính mới chọn được thư mục bất kỳ (hỏi đáp #10). */
    mobileAnyway: 'Muốn chọn một thư mục bất kỳ: làm trên máy tính, Cài đặt › Lưu trữ › Chọn thư mục…',
    submitMove: { long: 'Chuyển sang thư mục mới' },
    submitNew: { long: 'Dùng thư mục mới cho video mới' },
    cancel: 'Huỷ',
  },
  moveFailed: {
    title: 'Lần chuyển trước không xong',
    retry: 'Thử lại',
    dismiss: 'Gác bản ghi',
  },
  moving: {
    title: 'Đang chuyển sang {volume}',
    stats: '{done} / {total} GB · còn khoảng {eta}',
    note: '{n} việc nền đang tạm dừng, tự chạy tiếp khi chuyển xong. Không rút ổ. Có thể đóng trang này; việc chuyển hiện trong Việc nền.',
    noteNoJobs: 'Không rút ổ. Có thể đóng trang này; việc chuyển hiện trong Việc nền.',
    noteM: 'Việc nền tạm dừng, tự chạy tiếp khi xong. Không rút ổ.',
    waiting: 'chờ',
    cancel: 'Huỷ chuyển · dữ liệu ở {volume} vẫn nguyên',
    cancelShort: 'Huỷ chuyển',
    cancelConfirm: 'Huỷ chuyển giữa chừng? Phần đã copy sẽ được dọn, thư mục dự án giữ nguyên chỗ cũ.',
    groups: {
      images: 'Ảnh đã vẽ',
      editor: 'Project editor',
      sources: 'Video gốc tải về',
      exports: 'MP4 đã xuất',
    } as Record<string, string>,
  },
  done: {
    title: 'Đã chuyển {size} sang {volume}',
    sub: 'Bản cũ ở {path} được giữ tới {date} phòng khi cần, rồi tự xoá.',
    subNoDate: 'Bản cũ ở {path} vẫn mở được như thường; máy không tự xoá.',
    /** Điện thoại gộp một câu (MSettings · store=done). */
    mTitle: 'Đã chuyển {size}',
    mSub: 'Bản cũ ở {path} giữ tới {date} rồi tự xoá.',
    deleteNow: 'Xoá bản cũ ngay · {size}',
    deleteNowM: 'Xoá bản cũ ngay',
    deleteConfirm: 'Xoá luôn bản cũ ({bytes} GB) tại {path}? Không hoàn tác được.',
    deleteAction: 'Xoá ngay',
    back: 'Chuyển về {volume}',
    backConfirm: 'Chuyển dự án về {path}? Dữ liệu sẽ được copy ngược lại trong nền.',
    backAction: 'Chuyển về',
    doneNewNoPrev: 'Video mới lưu ở {volume}. Dữ liệu cũ vẫn ở chỗ cũ, mở được như thường.',
    doneNew: 'Video mới lưu ở {volume}. {bytes} GB dự án cũ vẫn ở {old} và mở được như thường; muốn gom về một chỗ thì bấm Đổi thư mục và chọn Chuyển.',
  },
  offline: {
    title: 'Không thấy ổ {name}',
    titleFolder: 'Không thấy thư mục dự án',
    desc: 'Ổ được rút lúc {time}. {n} việc nền đang chạy đã tạm dừng, hàng chờ chưa chạy tiếp. Cắm lại ổ thì máy tự chạy tiếp, không mất gì.',
    descNone: 'Ổ được rút lúc {time}. Hàng chờ chưa chạy tiếp. Cắm lại ổ thì máy tự chạy tiếp, không mất gì.',
    descM: 'Ổ được rút lúc {time}. {n} việc nền đã tạm dừng, hàng chờ chưa chạy. Cắm lại ổ thì máy tự chạy tiếp.',
    retry: 'Thử lại',
    fallback: 'Lưu video mới vào {volume}',
  },
  usage: {
    title: 'Viderse đang dùng {total}',
    titleM: 'Đang dùng {total}',
    /** Chỗ trống của ổ đang giữ thư mục lưu dự án, luôn kèm tên ổ (#79). */
    free: '{volume} còn trống {free}',
    freeM: '{volume} còn {free}',
    freeOffline: '{volume} chưa cắm · chưa đọc được chỗ trống',
    freeOfflineM: '{volume} chưa cắm',
    freeTitle: 'Chỗ trống của ổ đang giữ thư mục lưu dự án',
    groups: {
      sources: 'Video gốc tải về',
      exports: 'MP4 đã xuất',
      editor: 'Project editor',
      images: 'Ảnh đã vẽ',
      stay: 'Mô hình và công cụ',
      other: 'Khác',
    } as Record<string, string>,
  },
  cleanup: {
    title: 'Tệp tạm và thứ không ai dùng',
    sub: 'App tự dọn lúc mở và mỗi 6 giờ · lần cuối {when}.',
    subNever: 'App tự dọn lúc mở và mỗi 6 giờ.',
    preview: 'Xem trước những gì sẽ dọn',
    previewNote: 'Không đụng tới video đang chạy, video chờ duyệt và mọi thứ template đang dùng.',
    stopped: 'dừng lại',
    /** Dòng tiến trình chạy dở: “2 tiến trình chạy dở bị bỏ quên”. */
    stoppedCount: '{n} {label}',
    later: 'Để sau',
    run: 'Dọn {bytes}',
    ran: 'Đã dọn {bytes} lúc {time}',
    empty: 'Không có gì để dọn.',
    stale: 'Danh sách vừa đổi — xem lại rồi bấm Dọn.',
    /** Hậu tố số tệp của nhóm media (artboard: “· 23 tệp”). */
    files: '{n} tệp',
  },
  reclaim: {
    // Server trả title/desc/bytes/count sẵn; copy dưới chỉ cho nút, xác nhận
    // và trạng thái.
    actions: { src: 'Xoá video gốc', mp4: 'Xoá MP4 đã đăng' } as Record<string, string>,
    mobileAction: 'Xoá',
    rowNotes: {
      src: '{bytes} · project vẫn mở và xuất lại được',
      mp4: '{bytes} · cần lại thì xuất lại',
    } as Record<string, string>,
    confirms: {
      src: 'Xoá {bytes} video gốc của {n} video đã xong? Không hoàn tác được.',
      mp4: 'Xoá {bytes} MP4 của {n} video đã đăng? Xuất lại mất khoảng 3 phút mỗi video.',
    } as Record<string, string>,
    confirmAction: 'Xoá {bytes}',
    gone: 'Đã xoá · lấy lại {bytes}',
    nothing: 'Chưa có video nào thuộc nhóm này.',
  },
} as const
