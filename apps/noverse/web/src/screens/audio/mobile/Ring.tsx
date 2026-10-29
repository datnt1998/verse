import type { ReactNode } from 'react'
import { ProgressRing } from '@noverse/ui'

/** MAudio sizes and centre content follow the summary, chapter and export artboards. */
export function Ring({ percent, size = 'sm', children }: { percent: number; size?: 'sm' | 'lg' | 'export' | number; children: ReactNode }) {
  return <ProgressRing className={`ma-ring ${typeof size === 'number' ? '' : size}`} size={typeof size === 'number' ? size : size === 'lg' ? 72 : size === 'export' ? 84 : 38}
    percent={percent * 100} color={size === 'export' ? 'var(--color-ok)' : 'var(--color-accent)'}>{children}</ProgressRing>
}
