import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color } from './tokens.stylex'
import { getKitLabels } from './labels'

const sx = stylex.create({
  note: { fontSize: 12, color: color['--color-fg-4'] },
})

/** Mount cùng khung xương; không thông báo chậm với lần tải nhanh. */
export function SlowLoadingNotice() {
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    const timer = setTimeout(() => setSlow(true), 8000)
    return () => clearTimeout(timer)
  }, [])
  return slow ? <span role="status" aria-live="polite" {...stylex.props(sx.note)}>{getKitLabels().slow.notice}</span> : null
}
