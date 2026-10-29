// ActionButton — nút của Viderse, bọc Button của kit theo tên biến thể của design
// (02 §3): primary (sáng, một màn một nút) · outline · quiet · danger-soft · danger.
// Đỏ không làm nền nút; cam đặc chỉ trong hộp xác nhận xoá vĩnh viễn.
import { forwardRef } from 'react'
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
}

export const ActionButton = forwardRef<HTMLButtonElement, ActionButtonProps>(function ActionButton(
  { variant = 'outline', disabledReason, disabled, tooltip, ...rest }, ref,
) {
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
