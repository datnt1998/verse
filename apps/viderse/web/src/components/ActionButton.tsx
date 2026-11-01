// ActionButton — nút của Viderse, bọc Button của kit theo tên biến thể của design
// (02 §3): primary (sáng, một màn một nút) · outline · quiet · danger-soft · danger.
// Đỏ không làm nền nút; cam đặc chỉ trong hộp xác nhận xoá vĩnh viễn.
import * as stylex from '@stylexjs/stylex'
import { text } from '../styles/shared.stylex'
import { forwardRef, useId } from 'react'
import { Button, type ButtonProps } from '@noverse/ui'

export type ActionVariant = 'primary' | 'outline' | 'quiet' | 'danger-soft' | 'danger'

const KIT_VARIANT: Record<ActionVariant, ButtonProps['variant']> = {
  primary: 'primary',
  outline: 'ghost',
  quiet: 'quiet',
  'danger-soft': 'dsoft',
  danger: 'danger',
}

export type ActionButtonProps = Omit<ButtonProps, 'variant'> & {
  variant?: ActionVariant
  /** lý do bị khoá — hiện bên cạnh tooltip/title khi disabled */
  disabledReason?: string
  /** Khoá nhưng vẫn nhận focus (chức năng chưa mở, vd cần kênh của bạn):
   *  aria-disabled + aria-describedby tới lý do, tooltip là lý do, bấm không
   *  làm gì. Lý do đã hiện ngay dưới nút thì truyền `aria-describedby` trỏ tới
   *  dòng đó; không thì nút tự kèm một dòng chỉ trình đọc màn hình đọc. */
  lockedReason?: string
}

export const ActionButton = forwardRef<HTMLButtonElement, ActionButtonProps>(function ActionButton(
  { variant = 'outline', disabledReason, lockedReason, disabled, tooltip, ...rest }, ref,
) {
  const ownId = useId()
  if (lockedReason) {
    const describedBy = rest['aria-describedby']
    return (
      <>
        <Button ref={ref} variant={KIT_VARIANT[variant]} tooltip={lockedReason} {...rest}
          aria-disabled aria-describedby={describedBy ?? ownId} />
        {describedBy === undefined && <span id={ownId} {...stylex.props(text.srOnly)}>{lockedReason}</span>}
      </>
    )
  }
  const reason = disabled && typeof disabledReason === 'string' ? disabledReason : undefined
  const tip = reason ?? tooltip
  return (
    <Button
      ref={ref}
      variant={KIT_VARIANT[variant]}
      disabled={disabled}
      tooltip={tip}
      title={typeof reason === 'string' ? reason : undefined}
      {...rest}
    />
  )
})
