// Hiệu ứng đang vẽ / vẽ xong của một ảnh — 04-components › DevelopEffect.
//
// `state="drawing"`: lớp phủ lên khung 16:9 đang giữ bản trước (nền mờ đậm,
// vệt sáng, hạt nhiễu, thẻ 3 bước, vòng chờ khi `compact`).
// `state="done"`: bọc ảnh thật — lộ dần từ mờ + viền sáng chạy một lần.
//
// Dùng chung cho Duyệt (máy tính + điện thoại), ô ảnh trong Editor và Phòng vẽ
// ở Bàn dựng; bản `compact` chỉ giữ nền mờ + vệt sáng + vòng chờ như dải ảnh.
//
// Móc kiểm chứng ổn định: `data-develop="drawing" | "done"` trên phần tử gốc của
// hiệu ứng, `data-develop-grain`, `data-develop-dot`, `data-develop-glint` bên trong
// (ReviewScreen.test truy vấn `[data-develop="drawing"]`).
import { Fragment, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button } from '@noverse/ui'
import { useMediaQuery } from '../lib/useMediaQuery'
import { Orb } from './Orb'
import { sx } from './develop.stylex'

export type DevelopEffectState = 'drawing' | 'done'

/** 3 bước trong thẻ trạng thái (artboard Review › .vv-devstep). */
const STEPS = [
  { label: 'Gửi prompt', done: 'Đã gửi prompt' },
  { label: 'Đang vẽ', done: 'Đã vẽ' },
  { label: 'Tải về', done: 'Tải về' },
] as const

export interface DevelopEffectProps {
  state: DevelopEffectState
  /** Bản đang dùng (ảnh cũ) — nền mờ dưới hiệu ứng. */
  baseUrl?: string | null
  /** Ảnh xem dần nhà cung cấp gửi qua SSE `previewUrl`. */
  previewUrl?: string | null
  /** “Hoạ sĩ đang vẽ bản 3” */
  title?: string
  /** “0:24 · thường khoảng 40 giây” */
  detail?: string
  /** Bước đang làm: 0 Gửi prompt · 1 Đang vẽ · 2 Tải về. */
  step?: number
  /** Bản nhỏ cho dải ảnh, Phòng vẽ, ô ảnh Editor: không thẻ, vòng chờ ở góc. */
  compact?: boolean
  /**
   * `card` = thẻ kính mờ giữa khung (Duyệt); `inline` = thẻ neo đáy cho ô ảnh
   * nhỏ trong Editor (không minWidth, chữ 12,5px, giờ bên phải).
   */
  variant?: 'card' | 'inline'
  /** Ẩn thẻ trạng thái nhưng vẫn giữ hiệu ứng nền. */
  hideCard?: boolean
  onCancel?: () => void
  /** Ảnh thật của `state="done"`. */
  children?: ReactNode
}

export function DevelopEffect({
  state, baseUrl, previewUrl, title, detail, step = 0, compact = false, variant = 'card', hideCard = false, onCancel, children,
}: DevelopEffectProps) {
  const reduced = useMediaQuery('(prefers-reduced-motion: reduce)')
  if (state === 'done') {
    return (
      <span data-develop="done" {...stylex.props(sx.reveal, reduced && sx.revealStill)}>
        {children}
        <span aria-hidden="true" data-develop-glint="" {...stylex.props(sx.glint, reduced && sx.still)} />
      </span>
    )
  }
  const inline = variant === 'inline'
  const showCard = inline || (!compact && !hideCard)
  return (
    <>
      {baseUrl ? <img alt="" src={baseUrl} {...stylex.props(sx.base, compact && sx.baseCompact, !compact && !!previewUrl && sx.basePartial)} /> : null}
      <span aria-hidden="true" data-develop="drawing" {...stylex.props(sx.root)}>
        <span {...stylex.props(sx.sweep, reduced && sx.still)} />
        {!reduced && <span data-develop-grain="" {...stylex.props(sx.grain)} />}
      </span>
      {previewUrl ? <img alt="" src={previewUrl} {...stylex.props(sx.preview)} /> : null}
      {compact && <span aria-hidden="true" {...stylex.props(sx.ring, reduced && sx.still)} />}
      {showCard && inline && (
        <div role="status" aria-live="polite" {...stylex.props(sx.cardInline)}>
          <span {...stylex.props(sx.cardInlineHead)}>
            <Orb of="artist" size={16} live={!reduced} />
            {title}
            {detail && <span {...stylex.props(sx.cardInlineTimer)}>{detail}</span>}
          </span>
          <span {...stylex.props(sx.stepsInline)}>
            {STEPS.map((item, index) => (
              <span key={item.label} {...stylex.props(sx.step, index < step && sx.stepDone, index === step && sx.stepOn)}>
                <span aria-hidden="true" data-develop-dot={index === step ? 'on' : 'idle'} {...stylex.props(sx.stepDot, index === step && !reduced && sx.stepDotOn)} />
                {index < step ? item.done : item.label}
              </span>
            ))}
          </span>
          {onCancel && <Button variant="quiet" size="sm" onClick={onCancel} xstyle={sx.cancel}>Huỷ lần vẽ</Button>}
        </div>
      )}
      {showCard && !inline && (
        <div role="status" aria-live="polite" {...stylex.props(sx.card)}>
          <div {...stylex.props(sx.cardHead)}>
            <Orb of="artist" size={26} live={!reduced} />
            <span {...stylex.props(sx.cardText)}>
              <span {...stylex.props(sx.cardTitle)}>{title}</span>
              {detail && <span {...stylex.props(sx.cardSub)}>{detail}</span>}
            </span>
            {onCancel && <Button variant="quiet" size="sm" onClick={onCancel} xstyle={sx.cancel}>Huỷ lần vẽ</Button>}
          </div>
          <div {...stylex.props(sx.steps)}>
            {STEPS.map((item, index) => (
              <Fragment key={item.label}>
                <span {...stylex.props(sx.step, index < step && sx.stepDone, index === step && sx.stepOn)}>
                  <span aria-hidden="true" data-develop-dot={index === step ? 'on' : 'idle'} {...stylex.props(sx.stepDot, index === step && !reduced && sx.stepDotOn)} />
                  {index < step ? item.done : item.label}
                </span>
                {index < STEPS.length - 1 && <span aria-hidden="true" {...stylex.props(sx.stepLine)} />}
              </Fragment>
            ))}
          </div>
          <span {...stylex.props(sx.hint)}>Bạn duyệt tiếp ảnh khác được; xong sẽ báo ở đây và trên dải ảnh.</span>
        </div>
      )}
    </>
  )
}
