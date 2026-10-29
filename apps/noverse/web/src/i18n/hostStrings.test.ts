// Bảng dịch host: mọi mẫu trong 06-data-contract.md phải dịch hết, và một dòng gồm
// nhiều mảnh (tiền tố + ngữ cảnh + chương) phải được áp MỌI mẫu, không dừng ở mẫu đầu.
import { describe, expect, it } from 'vitest'
import { STREAM_ROUNDS, roundSpecFor, translateHost } from './hostStrings'

describe('translateHost — bảng 06', () => {
  it('dịch chương, ưu tiên 重写/打磨 trước 写', () => {
    expect(translateHost('写第 3 章')).toBe('Viết chương 3')
    expect(translateHost('重写第 4 章')).toBe('Viết lại chương 4')
    expect(translateHost('打磨第 5 章')).toBe('Gọt giũa chương 5')
  })

  it('dịch phán định và mở lại', () => {
    expect(translateHost('启动裁定')).toBe('Phán định khởi động')
    expect(translateHost('启动补裁')).toBe('Phán định khởi động bổ sung')
    expect(translateHost('用户干预裁定')).toBe('Phán định can thiệp')
    expect(translateHost('僵局裁定')).toBe('Phán định bế tắc')
    expect(translateHost('失败裁定')).toBe('Phán định thất bại')
    expect(translateHost('启动裁定已补齐(规划师: architect——outline)')).toBe('Đã bổ sung phán định khởi động (người lập kế hoạch: Kiến trúc sư — Đại cương)')
    expect(translateHost('已重开全书返工: 第 [3 5] 章入队')).toBe('Đã mở lại toàn bộ để viết lại: chương 3 5 vào hàng đợi')
    expect(translateHost('已重开本书为创作状态（用户撤销完结裁定）')).toBe('Đã mở lại truyện để sáng tác tiếp (bạn hủy phán định hoàn thành)')
    expect(translateHost('重试 (1/3): timeout')).toBe('Thử lại (1/3): timeout')
  })

  it('dịch ngữ cảnh + chiến lược (khoá tiếng Anh và nhãn tiếng Trung)', () => {
    expect(translateHost('writer 上下文 42% (84k/200k) 策略: light_trim')).toBe('Người viết ngữ cảnh 42% (84k/200k) · chiến lược: cắt tỉa nhẹ')
    expect(translateHost('第 5 章 草稿 上下文 12% (41.2k tokens) 策略: 无')).toBe('Chương 5 · bản nháp ngữ cảnh 12% (41.2k đơn vị ngữ cảnh) · chiến lược: không')
  })

  it('dịch vòng đời sáng tác', () => {
    expect(translateHost('开始创作（规划师: architect——outline）')).toBe('Bắt đầu sáng tác (Kiến trúc sư lập kế hoạch truyện dài — outline)')
    expect(translateHost('恢复创作: 第 3 章')).toBe('Tiếp tục sáng tác: Chương 3')
    expect(translateHost('一致性告警: chương 5 mâu thuẫn ch.3')).toBe('Cảnh báo nhất quán: chương 5 mâu thuẫn ch.3')
    expect(translateHost('用户手动暂停当前创作')).toBe('Bạn đã tạm dừng sáng tác')
    expect(translateHost('引擎停止 (已完成 8 章)')).toBe('Hệ thống dừng (đã xong 8 chương)')
    expect(translateHost('《Trà Xanh》创作完成: 共 8 章 32000 字')).toBe('«Trà Xanh» hoàn thành: 8 chương, 32.000 chữ')
    expect(translateHost('写作规则已更新并持久化')).toBe('Quy tắc viết đã cập nhật và lưu lại')
    expect(translateHost('章节推进模式已切换为自动推进')).toBe('Đã chuyển cách đẩy chương: Tự động')
    expect(translateHost('章节推进模式已切换为逐章验收')).toBe('Đã chuyển cách đẩy chương: Nghiệm thu từng chương')
  })

  it('dịch cổng nghiệm thu, không lặp đuôi câu', () => {
    expect(translateHost('已放行第 6 章；使用 /next 生成，或输入修改意见')).toBe('Đã duyệt chương 6; bấm Duyệt hoặc nhập ý kiến sửa')
    expect(translateHost('已完成至第 5 章，逐章验收等待放行第 6 章；使用 /next 生成，或输入修改意见'))
      .toBe('Đã xong đến chương 5, chờ duyệt chương 6; bấm Duyệt hoặc nhập ý kiến sửa')
    expect(translateHost('已完成至第 5 章，逐章验收等待放行第 6 章；')).toBe('Đã xong đến chương 5, chờ duyệt chương 6;')
  })

  it('dịch ngân sách, model, tiền tố người dùng', () => {
    expect(translateHost('预算告警')).toBe('Cảnh báo ngân sách')
    expect(translateHost('预算用尽')).toBe('Hết ngân sách')
    expect(translateHost('预算停机')).toBe('Dừng do ngân sách')
    expect(translateHost('预算盲区')).toBe('Không đo được ngân sách')
    expect(translateHost('模型已切换：writer → codex/gpt-5.4')).toBe('Đã đổi mô hình cho Người viết → codex/gpt-5.4')
    expect(translateHost('推理强度已切换：writer → high')).toBe('Đã đổi mức suy luận: Người viết → Cao')
    expect(translateHost('[继续] 写第 7 章')).toBe('[Tiếp tục] Viết chương 7')
    expect(translateHost('[用户干预] 打磨第 5 章')).toBe('[Can thiệp] Gọt giũa chương 5')
  })

  it('dịch phạm vi, nhãn trạng thái, nhãn khôi phục', () => {
    expect(translateHost('第3章')).toBe('Chương 3')
    expect(translateHost('第 5 章·草稿')).toBe('Chương 5 · bản nháp')
    expect(translateHost('本弧')).toBe('hồi này')
    expect(translateHost('全局')).toBe('toàn cục')
    expect(translateHost('就绪')).toBe('Sẵn sàng')
    expect(translateHost('重写恢复: 3 章待处理')).toBe('Khôi phục viết lại: 3 chương chờ xử lý')
    expect(translateHost('打磨恢复: 2 章待处理')).toBe('Khôi phục gọt giũa: 2 chương chờ xử lý')
    expect(translateHost('恢复: 第 2 章进行中')).toBe('Khôi phục: chương 2 đang viết')
    expect(translateHost('恢复: 审阅中断')).toBe('Khôi phục: thẩm duyệt bị gián đoạn')
  })

  it('không khớp mẫu → nguyên văn; chuỗi vi đã dịch giữ nguyên', () => {
    expect(translateHost('Free-form model text')).toBe('Free-form model text')
    expect(translateHost('Viết chương 3')).toBe('Viết chương 3')
    expect(translateHost('')).toBe('')
  })

  // Câu host Việt hiện phát (internal/host) phải đi qua nguyên văn — bảng chỉ dịch dữ liệu Trung cũ.
  it('chuỗi host tiếng Việt hiện tại đi qua không đổi', () => {
    const current = [
      'Chương 8 · 8.766 chữ',
      'Cần sửa · 2 vấn đề · ảnh hưởng chương 8, 9',
      'Chương 8: Ngày 18 tại lễ Kim Đăng',
      '«Trà Xanh» hoàn thành: 8 chương, 32.000 chữ',
      'Khôi phục viết lại: 3 chương chờ xử lý',
      'Khôi phục: chương 2 đang viết',
      'Khôi phục: thẩm duyệt cuối cung đang chờ (Tập 1 · Cung 2)',
      'Đã xong đến chương 5, chờ duyệt chương 6; dùng /next hoặc nhập ý kiến sửa',
      'Đã duyệt chương 6; sau khi nộp chương này sẽ hoàn tất thẩm duyệt và cập nhật cấu trúc cung/tập cần thiết, rồi lại chờ duyệt',
      'Thử lại (2/7, sau 2s): stream read error',
      'Cảnh báo ngân sách: đã chi $4.00 trên ngân sách $5.00 (80%)',
      '[Can thiệp] Gọt giũa chương 5',
      'Lâm Tri Hạ (Nữ chính; chương 3)',
      'thư mục truyện đang được một phiên Noverse khác sử dụng',
      'Lâm Tri Hạ (Nữ chính)',
    ]
    for (const line of current) expect(translateHost(line)).toBe(line)
    expect(translateHost('writer ngữ cảnh 42% (84000/200000) · chiến lược: cắt tỉa nhẹ')).toBe(
      'Người viết ngữ cảnh 42% (84000/200000) · chiến lược: cắt tỉa nhẹ',
    )
    expect(translateHost('Bắt đầu sáng tác (người lập kế hoạch: architect — outline)')).toBe(
      'Bắt đầu sáng tác (người lập kế hoạch: Kiến trúc sư — outline)',
    )
  })

  // Dữ liệu host thật (snapshot truyện dit-dang-audio/01-tra-xanh-cuop-kich-ban).
  it('dịch tóm tắt snapshot: chương + số chữ + số vấn đề', () => {
    expect(translateHost('第8章 8766字')).toBe('Chương 8 · 8.766 chữ')
    expect(translateHost('第 12 章 12345字')).toBe('Chương 12 · 12.345 chữ')
    expect(translateHost('verdict=accept 2个问题')).toBe('Đạt · 2 vấn đề')
    expect(translateHost('verdict=polish 1个问题')).toBe('Cần sửa · 1 vấn đề')
    expect(translateHost('第8章: Ngày 18 tại lễ Kim Đăng')).toBe('Chương 8: Ngày 18 tại lễ Kim Đăng')
    expect(translateHost('第7章：Ngày 15')).toBe('Chương 7: Ngày 15')
    expect(translateHost('8766字')).toBe('8.766 chữ')
    expect(translateHost('2个问题')).toBe('2 vấn đề')
  })

  it('statusLabel engine phát chữ hoa + dấu câu toàn khối', () => {
    expect(translateHost('COMPLETE')).toBe('Hoàn thành')
    expect(translateHost('READY')).toBe('Sẵn sàng')
    expect(translateHost('RUNNING')).toBe('Đang chạy')
    expect(translateHost('REVIEW')).toBe('Thẩm duyệt')
    expect(translateHost('REWRITE')).toBe('Viết lại')
    expect(translateHost('idle')).toBe('Đang rảnh')
    expect(translateHost('paused')).toBe('Đã tạm dừng')
    expect(translateHost('Lâm Tri Hạ（Nữ chính; biên kịch độc lập）')).toBe('Lâm Tri Hạ (Nữ chính; biên kịch độc lập)')
  })

  it('chuỗi còn chữ Hán (văn model) giữ nguyên văn, không đổi dấu câu', () => {
    const prose = '前5章审阅通过。已阅读全部草稿原文，主线从被安排做节目笑料，递进到公开要求核验作者身份。'
    expect(translateHost(prose)).toBe(prose)
  })
})

describe('roundSpecFor — header ✻ → nhãn vi', () => {
  it('ánh xạ header Việt hiện tại', () => {
    expect(roundSpecFor('Truy vấn ngữ cảnh')).toMatchObject({ labelVi: 'Đọc bối cảnh', tool: 'novel_context', kind: 'kv' })
    expect(roundSpecFor('Nộp chương')).toMatchObject({ labelVi: 'Lưu chương', tool: 'commit_chapter', kind: 'commit' })
    expect(roundSpecFor('Thẩm duyệt')).toMatchObject({ labelVi: 'Thẩm duyệt', tool: 'save_review', kind: 'review' })
    expect(roundSpecFor('Sửa đại cương')).toMatchObject({ labelVi: 'Sửa đại cương', tool: 'revise_outline', kind: 'text' })
    expect(roundSpecFor('draft_chapter')).toMatchObject({ labelVi: 'Viết bản nháp', tool: 'draft_chapter', kind: 'text' })
    expect(roundSpecFor('export_book')).toMatchObject({ labelVi: 'export book', kind: 'raw' })
  })

  it('header Trung cũ vẫn ra cùng spec', () => {
    expect(roundSpecFor('查询上下文')).toMatchObject({ labelVi: 'Đọc bối cảnh', kind: 'kv' })
    expect(roundSpecFor('章节提交')).toMatchObject({ labelVi: 'Lưu chương', kind: 'commit' })
    expect(roundSpecFor('审阅')).toMatchObject({ labelVi: 'Thẩm duyệt', kind: 'review' })
    expect(roundSpecFor('弧摘要')).toMatchObject({ labelVi: 'Tóm tắt hồi', kind: 'text' })
    for (const spec of STREAM_ROUNDS) expect(roundSpecFor(spec.legacy ?? '')).toBe(spec)
  })

  it('header lạ giữ nguyên văn', () => {
    expect(roundSpecFor('khác')).toMatchObject({ header: 'khác', labelVi: 'khác', kind: 'raw' })
  })
})
