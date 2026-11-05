// Hợp đồng của Trình xem ảnh dùng chung. ID ảnh là nguồn sự thật của lựa chọn:
// mọi thay đổi (polling, đổi job, xoá ảnh) đều đi qua `selectedId`/`onSelect`,
// không truyền chỉ số dễ trôi.
import type { ReactNode } from 'react'

export type ImageViewerMode = 'review' | 'view' | 'view-replace' | 'sample'

export interface ImageViewerItem {
  id: string
  label: string
  /** Ảnh hiển thị được. Không có URL thì ô là sọc, không bấm được. */
  url?: string | null
  /** Ảnh chưa vẽ/đang vẽ/đã lỗi: hiện ô sọc, ← → bỏ qua. */
  disabled?: boolean
  /** Nhãn ngắn trên ô dải ảnh; mặc định số thứ tự (hoặc “Bìa”). */
  badge?: string
}

export interface ModalImageViewerProps {
  inline?: false
  mode: ImageViewerMode
  title: string
  subtitle?: string
  modeLabel?: string
  stripLabel?: string
  items: readonly ImageViewerItem[]
  selectedId: string
  onSelect: (id: string) => void
  side: ReactNode
  footer?: ReactNode
  onClose: () => void
}

/** Review owns its selection and keys; the inline frame does not duplicate them. */
export interface InlineImageViewerProps {
  inline: true
  mode: 'review'
  stage: ReactNode
  side: ReactNode
}

export type ImageViewerProps = ModalImageViewerProps | InlineImageViewerProps
