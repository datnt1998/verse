// Copy của mục Báo lỗi — Settings.dc.html section=logs: thẻ xuất .zip, danh
// sách “Lỗi gần đây” 7 ngày, thẻ phiên bản. Nhật ký thô chỉ nằm trong tệp .zip.
export const logs = {
  export: {
    title: 'Xuất log để báo lỗi',
    desc: 'Gom nhật ký của app và 5 video gần nhất vào một tệp .zip, khoảng 3 MB. Cookie, tài khoản và đường dẫn cá nhân đã được che.',
    action: 'Xuất log',
  },
  errors: {
    title: 'Lỗi gần đây',
    subtitle: '7 ngày qua · {n} lỗi',
    empty: 'Không có lỗi nào trong 7 ngày qua.',
  },
  version: {
    latest: 'Bản mới nhất',
    hasUpdate: 'Có bản {version}',
    check: 'Kiểm tra bản mới',
  },
} as const
