// 09-auth-onboard.md §10.1/§10.4 — chữ, màu, icon của hộp trạng thái “kết luận kiểm tra thư mục”.
// Màu lấy nguyên từ Onboard.dc.html (CHK / kindChk); dùng chung cho bước Nơi lưu truyện, bộ duyệt thư mục và Cài đặt.
import type { LibraryInspect } from '../../api/types'

export type InspectTone = 'ok' | 'info' | 'warn' | 'err'
export type InspectView = { tone: InspectTone; icon: string; bg: string; bd: string; fg: string; text: string }

const TONES: Record<InspectTone, Omit<InspectView, 'text'>> = {
  ok: { tone: 'ok', icon: 'tick-circle', bg: 'rgba(166,237,193,0.06)', bd: 'rgba(166,237,193,0.22)', fg: '#C6F3D6' },
  info: { tone: 'info', icon: 'info-circle', bg: 'rgba(157,176,247,0.06)', bd: 'rgba(157,176,247,0.2)', fg: '#C8D3F8' },
  warn: { tone: 'warn', icon: 'warning-2', bg: 'rgba(246,203,140,0.06)', bd: 'rgba(246,203,140,0.22)', fg: '#F6CB8C' },
  err: { tone: 'err', icon: 'warning-2', bg: 'rgba(255,154,162,0.06)', bd: 'rgba(255,154,162,0.25)', fg: '#FFC4CA' },
}

/** Nơi đồng bộ đám mây: nhắc “Luôn giữ trên máy” để Noverse không phải chờ tải về. */
const CLOUD_PLACE = /icloud|drive|onedrive|dropbox/i

export const foundText = (info: Pick<LibraryInspect, 'projects' | 'stories' | 'projectName'>) =>
  `Đã có ${info.projects} dự án Noverse${info.projects === 1 && info.projectName ? ` (“${info.projectName}”)` : ''} với ${info.stories} truyện — Noverse dùng luôn, không đổi gì.`

/** Hộp trạng thái đầy đủ (dưới danh sách ở bước Nơi lưu truyện). */
export function inspectView(info: LibraryInspect): InspectView {
  switch (info.kind) {
    case 'found': return { ...TONES.ok, text: foundText(info) }
    case 'empty': return {
      ...TONES.ok,
      text: CLOUD_PLACE.test(info.place)
        ? `Thư mục trống, ghi được. ${info.place} tự sao lưu truyện của bạn — nên bật “Luôn giữ trên máy” để Noverse không phải chờ tải về.`
        : 'Thư mục trống, ghi được.',
    }
    case 'other': return { ...TONES.warn, text: 'Có tệp khác không phải truyện — Noverse không đụng tới, nhưng nên chọn thư mục riêng' }
    case 'readonly': return { ...TONES.err, text: 'Noverse không ghi được vào thư mục này (chỉ đọc). Chọn thư mục khác hoặc cấp quyền ghi trên máy.' }
    case 'missing': return { ...TONES.info, text: 'Thư mục này chưa có — Noverse sẽ tạo khi bạn bấm Tiếp tục.' }
  }
}

/** Kết luận một dòng ở chân bộ duyệt (Onboard.dc.html › kindChk): màu chữ + chữ. */
export function footerView(info: LibraryInspect): { icon: string; fg: string; text: string } {
  switch (info.kind) {
    case 'found': return { icon: 'tick-circle', fg: '#A6EDC1', text: 'Thư viện Noverse có sẵn — dùng luôn' }
    case 'empty': return { icon: 'tick-circle', fg: '#A6EDC1', text: 'Thư mục trống, ghi được' }
    case 'other': return { icon: 'warning-2', fg: '#F6CB8C', text: 'Có tệp khác không phải truyện — Noverse không đụng tới, nhưng nên chọn thư mục riêng' }
    case 'readonly': return { icon: 'warning-2', fg: '#FF9AA2', text: 'Không ghi được vào thư mục này' }
    case 'missing': return { icon: 'info-circle', fg: '#C8D3F8', text: 'Thư mục chưa có' }
  }
}

/** Ghi chú bên phải thẻ chọn (“1 dự án · 1 truyện”, “Thư mục trống”, “Không ghi được”). */
export function inspectNote(info: LibraryInspect): { text: string; fg: string } {
  switch (info.kind) {
    case 'found': return { text: `${info.projects} dự án · ${info.stories} truyện`, fg: '#A6EDC1' }
    case 'readonly': return { text: 'Không ghi được', fg: '#FF9AA2' }
    case 'missing': return { text: 'Thư mục mới', fg: '#8E8E99' }
    case 'empty': return { text: 'Thư mục trống', fg: '#8E8E99' }
    case 'other': return { text: `${info.items} mục`, fg: '#8E8E99' }
  }
}

/** Tên hiển thị của thư mục = đoạn cuối của “Nơi › … › Thư mục”. */
export const displayLeaf = (display: string) => display.split(' › ').pop() ?? display
