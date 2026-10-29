// Bảng dịch chuỗi host cũ (zh → vi) — 06-data-contract.md › Bảng dịch.
// Host hiện phát tiếng Việt (19-gap-closure §B); bảng này chỉ còn để đọc sự kiện/snapshot
// đã lưu từ bản tiếng Trung. Đầu ra mỗi mẫu = đúng câu host Việt hiện phát cho cùng sự kiện,
// nên dữ liệu cũ và mới hiển thị như nhau; chuỗi Việt không khớp mẫu nào → trả nguyên văn.
// Thứ tự quan trọng: mẫu cụ thể đứng trước mẫu tổng quát, và MỌI mẫu đều được áp
// (một dòng có thể gồm nhiều mảnh: tiền tố + ngữ cảnh + chiến lược + số chương).
import { enumLabel } from './enums'
import { agentLabel, humanizeToolText, toolLabel } from '../lib/labels'
import { vi } from './vi'

type Rule = { re: RegExp; to: string | ((m: RegExpMatchArray) => string) }

// Số trong chuỗi host theo vi-VN: 8766 → 8.766 (cùng quy ước với số chữ trên UI).
const numberFormat = new Intl.NumberFormat('vi-VN')

// Giá trị `strategy` engine phát: khoá tiếng Anh (TUI dịch) hoặc nhãn tiếng Trung.
const STRATEGY_ZH: Record<string, string> = {
  '无': 'không', '压缩': 'nén', '微压缩': 'nén nhỏ', '轻裁剪': 'cắt tỉa nhẹ', '全量摘要': 'tóm tắt toàn diện',
}

const RULES: Rule[] = [
  // ─── Động từ + chương cụ thể trước (nếu không, `第N章:` bên dưới nuốt mất số chương) ───
  { re: /重写第\s*(\d+)\s*章/, to: (m) => `Viết lại chương ${m[1]}` },
  { re: /打磨第\s*(\d+)\s*章/, to: (m) => `Gọt giũa chương ${m[1]}` },
  { re: /写第\s*(\d+)\s*章/, to: (m) => `Viết chương ${m[1]}` },

  // ─── Tóm tắt snapshot (lastCommitSummary/lastReviewSummary/recentSummaries) ───
  // Host thật phát: `第8章 8766字`, `verdict=accept 2个问题`, `第8章: …`.
  { re: /第\s*(\d+)\s*章\s*(\d+)\s*字/, to: (m) => `Chương ${m[1]} · ${numberFormat.format(Number(m[2]))} chữ` },
  { re: /第\s*(\d+)\s*章\s*[:：]\s*/, to: (m) => `Chương ${m[1]}: ` },
  { re: /verdict=(\w+)\s*(\d+)\s*个问题/, to: (m) => `${enumLabel('verdict', m[1])} · ${m[2]} vấn đề` },
  { re: /verdict=(\w+)/, to: (m) => enumLabel('verdict', m[1]) },
  { re: /(\d+)\s*个问题/, to: (m) => `${m[1]} vấn đề` },

  // ─── Phán định (Arbiter) ───
  { re: /启动裁定已补齐\(规划师[:：]\s*(.+?)\s*[—-]{1,2}\s*(.+?)\)/, to: (m) => `Đã bổ sung phán định khởi động (người lập kế hoạch: ${agentLabel(m[1])} — ${enumLabel('phase', m[2])})` },
  { re: /启动补裁/, to: 'Phán định khởi động bổ sung' },
  { re: /启动裁定/, to: 'Phán định khởi động' },
  { re: /用户干预裁定/, to: 'Phán định can thiệp' },
  { re: /僵局裁定/, to: 'Phán định bế tắc' },
  { re: /失败裁定/, to: 'Phán định thất bại' },
  { re: /裁定[:：]\s*(.+)/, to: (m) => `Phán định: ${m[1]}` },
  { re: /干预裁定失败[:：](.+)（未做任何修改）/, to: (m) => `Phán định can thiệp thất bại: ${m[1]} (chưa thay đổi gì)` },

  // ─── Mở lại / thử lại ───
  { re: /已重开全书返工[:：]\s*第\s*\[([^\]]+)\]\s*章入队/, to: (m) => `Đã mở lại toàn bộ để viết lại: chương ${m[1]} vào hàng đợi` },
  { re: /已重开本书为创作状态（用户撤销完结裁定）/, to: 'Đã mở lại truyện để sáng tác tiếp (bạn hủy phán định hoàn thành)' },
  { re: /重试\s*\((\d+)\/(\d+)\)[:：]\s*(.+)/, to: (m) => `Thử lại (${m[1]}/${m[2]}): ${m[3]}` },

  // ─── Ngữ cảnh / chiến lược ───
  {
    re: /(.+?)\s*上下文\s*(\d+)%\s*\(([^)]+)\)\s*策略[:：]\s*(.+)/,
    to: (m) => {
      const strategy = m[4].trim()
      return `${agentLabel(m[1])} ngữ cảnh ${m[2]}% (${m[3].replace(/\btokens?\b/gi, 'đơn vị ngữ cảnh')}) · chiến lược: ${vi.ctxStrategy[strategy] ?? STRATEGY_ZH[strategy] ?? 'chưa xác định'}`
    },
  },

  // ─── Vòng đời sáng tác ───
  { re: /开始创作（规划师[:：]\s*(.+?)\s*[—-]{1,2}\s*(.+?)）/, to: (m) => `Bắt đầu sáng tác (Kiến trúc sư lập kế hoạch truyện dài — ${m[2]})` },
  { re: /恢复创作[:：]\s*(.+)/, to: (m) => `Tiếp tục sáng tác: ${m[1]}` },
  { re: /一致性告警[:：]\s*(.+)/, to: (m) => `Cảnh báo nhất quán: ${m[1]}` },
  { re: /用户手动暂停当前创作/, to: 'Bạn đã tạm dừng sáng tác' },
  { re: /引擎停止\s*\(已完成\s*(\d+)\s*章\)/, to: (m) => `Hệ thống dừng (đã xong ${m[1]} chương)` },
  { re: /《(.+?)》创作完成[:：]\s*共\s*(\d+)\s*章\s*(\d+)\s*字/, to: (m) => `«${m[1]}» hoàn thành: ${m[2]} chương, ${numberFormat.format(Number(m[3]))} chữ` },
  { re: /写作规则已更新并持久化/, to: 'Quy tắc viết đã cập nhật và lưu lại' },
  { re: /章节推进模式已切换为自动推进/, to: 'Đã chuyển cách đẩy chương: Tự động' },
  { re: /章节推进模式已切换为逐章验收/, to: 'Đã chuyển cách đẩy chương: Nghiệm thu từng chương' },

  // ─── Cổng nghiệm thu ───
  {
    re: /已完成至第\s*(\d+)\s*章[，,]\s*逐章验收等待放行第\s*(\d+)\s*章[；;]\s*(.*)/,
    to: (m) => `Đã xong đến chương ${m[1]}, chờ duyệt chương ${m[2]}; ${translateHost(m[3])}`.trimEnd(),
  },
  {
    re: /规划已就绪[，,]\s*逐章验收等待放行第\s*(\d+)\s*章[；;]\s*(.*)/,
    to: (m) => `Kế hoạch đã sẵn sàng, chờ duyệt chương ${m[1]}; ${translateHost(m[2])}`.trimEnd(),
  },
  {
    re: /已放行第\s*(\d+)\s*章[；;]\s*(.*)/,
    to: (m) => `Đã duyệt chương ${m[1]}; ${translateHost(m[2])}`.trimEnd(),
  },
  { re: /已放行第\s*(\d+)\s*章/, to: (m) => `Đã duyệt chương ${m[1]}` },
  { re: /^diag\s*完成$/, to: 'Chẩn đoán đã xong' },
  { re: /使用\s*\/next\s*生成[，,]\s*或输入修改意见/, to: 'bấm Duyệt hoặc nhập ý kiến sửa' },

  // ─── Ngân sách / StopGuard ───
  { re: /预算告警/, to: 'Cảnh báo ngân sách' },
  { re: /预算用尽/, to: 'Hết ngân sách' },
  { re: /预算停机/, to: 'Dừng do ngân sách' },
  { re: /预算盲区/, to: 'Không đo được ngân sách' },
  { re: /StopGuard[:：]\s*(.+?)\s*已拦截催促（连续第\s*(\d+)\s*次）/, to: (m) => `Bộ chống lặp: ${m[1]} đã chặn lệnh thúc giục (lần thứ ${m[2]} liên tiếp)` },

  // ─── Đổi model / mức suy luận ───
  { re: /模型已切换[:：]\s*(.+?)\s*→\s*(\S+)\/(\S+)/, to: (m) => `Đã đổi mô hình cho ${agentLabel(m[1])} → ${m[2]}/${m[3]}` },
  { re: /推理强度已切换[:：]\s*(.+?)\s*→\s*(\S+)/, to: (m) => `Đã đổi mức suy luận: ${agentLabel(m[1])} → ${vi.thinkingLevel[m[2]] ?? 'Mức riêng'}` },

  // ─── Tiền tố người dùng ───
  { re: /\[继续\]\s*(.+)/, to: (m) => `[Tiếp tục] ${m[1]}` },
  { re: /\[用户干预\]\s*(.+)/, to: (m) => `[Can thiệp] ${m[1]}` },

  // ─── Nhãn khôi phục (recoveryLabel) ───
  { re: /重写恢复[:：]\s*(\d+)\s*章待处理/, to: (m) => `Khôi phục viết lại: ${m[1]} chương chờ xử lý` },
  { re: /打磨恢复[:：]\s*(\d+)\s*章待处理/, to: (m) => `Khôi phục gọt giũa: ${m[1]} chương chờ xử lý` },
  { re: /恢复[:：]\s*第\s*(\d+)\s*章进行中/, to: (m) => `Khôi phục: chương ${m[1]} đang viết` },
  { re: /恢复[:：]\s*审阅中断/, to: 'Khôi phục: thẩm duyệt bị gián đoạn' },

  // ─── Phạm vi / vị trí — dạng ghép, để cuối ───
  { re: /第\s*(\d+)\s*章\s*[·・]?\s*草稿/, to: (m) => `Chương ${m[1]} · bản nháp` },
  { re: /第\s*(\d+)\s*章/, to: (m) => `Chương ${m[1]}` },
  { re: /本弧/, to: 'hồi này' },
  { re: /全局/, to: 'toàn cục' },
  // Số chữ trần (`8766字`) — sau mọi mẫu có `字` cụ thể ở trên.
  { re: /(\d+)\s*字/, to: (m) => `${numberFormat.format(Number(m[1]))} chữ` },
]

/** Dịch một chuỗi host. Không khớp → trả nguyên văn. */
function translateHostRaw(text: string): string {
  if (!text) return text
  const trimmed = text.trim()
  // statusLabel: tiếng Trung thuần (就绪…) hoặc mã engine chữ hoa (READY|COMPLETE…) —
  // cùng bảng với runtimeState qua enumLabel('status').
  const direct = vi.statusLabel[trimmed] ?? (/\s/.test(trimmed) ? undefined : enumLabel('status', trimmed))
  if (direct && direct !== trimmed) return direct
  // Lời gọi công cụ (`commit_chapter(chương 8)`, `writer → commit_chapter(…)`) → lời thường (labels.ts).
  const humanized = humanizeToolText(text)
  if (humanized !== text) return humanized
  let out = text
  for (const { re, to } of RULES) {
    out = out.replace(re, (full, ...rest) => {
      if (typeof to === 'string') return to
      // rest = [p1..pn, offset, input, groups?] — cắt trước offset để giữ đúng vị trí nhóm (kể cả nhóm tuỳ chọn undefined).
      const offsetAt = rest.findIndex((g) => typeof g === 'number')
      const match = [full, ...rest.slice(0, offsetAt)] as unknown as RegExpMatchArray
      return to(match)
    })
  }
  // Chuỗi còn chữ Hán (vd tóm tắt thẩm duyệt model viết) → giữ NGUYÊN VĂN, kể cả dấu toàn khối:
  // bảng dịch chỉ thay mẫu đã biết, không "việt hoá" dở dang văn bản chưa dịch.
  if (/[\u3400-\u9fff]/.test(out)) return out
  // Dấu câu toàn khối còn lại → ASCII (vd characters: `Lâm Tri Hạ（Nữ chính; …）`).
  // Đặt sau RULES để không phá các mẫu vốn viết bằng dấu toàn khối.
  return out
    .replace(/(\S)\s*（/g, '$1 (').replace(/（/g, '(').replace(/\s*）/g, ')')
    .replace(/，\s*/g, ', ').replace(/；\s*/g, '; ').replace(/：\s*/g, ': ')
}

/**
 * Dịch một chuỗi host (nhớ kết quả).
 *
 * Bảng RULES có ~80 biểu thức chính quy; mỗi dòng nhật ký gọi hàm này một lần, và nhật ký
 * vẽ lại mỗi giây (`useNow`). Đo trên mock: 43–50 ms mỗi lần mở Bàn viết. Chuỗi sự kiện
 * gần như bất biến giữa các lần vẽ nên nhớ theo chuỗi gốc là đủ.
 */
const HOST_MEMO = new Map<string, string>()
const HOST_MEMO_MAX = 800

export function translateHost(text: string): string {
  if (!text) return text
  const hit = HOST_MEMO.get(text)
  if (hit !== undefined) return hit
  const out = translateHostRaw(text)
  if (HOST_MEMO.size >= HOST_MEMO_MAX) HOST_MEMO.clear()
  HOST_MEMO.set(text, out)
  return out
}

// ─── Luồng viết: bản đồ header ✻ → nhãn vi + tool (06-data-contract.md › Stream) ─
// `header` = đúng chuỗi host phát hiện nay (internal/host/stream_extract.go toolDisplays);
// `legacy` = header tiếng Trung của bản cũ, còn nằm trong streamText/sự kiện đã lưu.

export type RoundKind = 'kv' | 'diff' | 'commit' | 'plan' | 'review' | 'text' | 'raw'

export type RoundSpec = { header: string; legacy?: string; labelVi: string; tool: string; kind: RoundKind }

export const STREAM_ROUNDS: RoundSpec[] = [
  { header: 'Truy vấn ngữ cảnh', legacy: '查询上下文', labelVi: 'Đọc bối cảnh', tool: 'novel_context', kind: 'kv' },
  { header: 'Đọc chương', legacy: '读章节', labelVi: 'Đọc chương', tool: 'read_chapter', kind: 'kv' },
  { header: 'Kiểm tra nhất quán', legacy: '一致性检查', labelVi: 'Soát nhất quán', tool: 'check_consistency', kind: 'kv' },
  { header: 'Gọt giũa', legacy: '打磨', labelVi: 'Gọt giũa', tool: 'edit_chapter', kind: 'diff' },
  { header: 'Nộp chương', legacy: '章节提交', labelVi: 'Lưu chương', tool: 'commit_chapter', kind: 'commit' },
  { header: 'Lập kế hoạch', legacy: '规划', labelVi: 'Lập kế hoạch chương', tool: 'plan_chapter', kind: 'plan' },
  { header: 'Thẩm duyệt', legacy: '审阅', labelVi: 'Thẩm duyệt', tool: 'save_review', kind: 'review' },
  { header: 'Tóm tắt cung', legacy: '弧摘要', labelVi: 'Tóm tắt hồi', tool: 'save_arc_summary', kind: 'text' },
  { header: 'Tóm tắt tập', legacy: '卷摘要', labelVi: 'Tóm tắt tập', tool: 'save_volume_summary', kind: 'text' },
  { header: 'Thiết lập', legacy: '设定', labelVi: 'Thiết lập', tool: 'save_foundation', kind: 'text' },
  { header: 'Sửa đại cương', legacy: '修订大纲', labelVi: 'Sửa đại cương', tool: 'revise_outline', kind: 'text' },
]

// Tiền tố dòng round trong streamText
export const ROUND_MARKER = '✻ '
// Khối suy nghĩ được bao bởi \x02
export const THINKING_MARK = '\x02'
// Con trỏ đang stream: chu kỳ glyph 200ms/khung
export const STREAM_CURSOR_GLYPHS = ['·', '✢', '✳', '✶', '✻', '✽']
// Giữ tối đa số round gần nhất
export const MAX_ROUNDS = 32

/**
 * Header ✻ → spec: khớp header hiện tại hoặc header Trung cũ. Host cũng phát `✻ <tên công cụ>` cho
 * công cụ không có extractor (save_book, audit_foundation…) → nhãn thường từ labels.ts, tên gốc nằm ở `tool`.
 * Không khớp gì → nhãn nguyên văn, kind raw.
 */
export const roundSpecFor = (header: string): RoundSpec => {
  const known = STREAM_ROUNDS.find((r) => r.header === header || r.legacy === header || r.labelVi === header)
  if (known) return known
  if (/^[a-z][a-z0-9]*(_[a-z0-9]+)+$/.test(header)) {
    return { header, labelVi: toolLabel(header), tool: header, kind: header === 'draft_chapter' ? 'text' : 'raw' }
  }
  return { header, labelVi: header, tool: '', kind: 'raw' }
}
