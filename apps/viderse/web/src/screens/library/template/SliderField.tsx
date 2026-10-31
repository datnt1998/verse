// Slider giữ giá trị tại chỗ khi kéo, chỉ gọi onChange lúc nhả tay/phím — thanh
// trượt không bắn một PATCH mỗi pixel, điều khiển rời rạc khác vẫn lưu ngay.
import { useEffect, useRef, useState } from 'react'
import type { ComponentProps } from 'react'
import { Slider } from '@noverse/ui'

export function SliderField({ value, onChange, ...rest }: ComponentProps<typeof Slider>) {
  const [local, setLocal] = useState(value)
  const dragging = useRef(false)
  useEffect(() => {
    if (!dragging.current) setLocal(value)
  }, [value])
  const commit = () => {
    dragging.current = false
    if (local !== value) onChange(local)
  }
  return <Slider {...rest} value={local} onChange={(next) => {
    dragging.current = true
    setLocal(next)
  }} onPointerUp={commit} onKeyUp={commit} onBlur={commit} />
}
