// ThemeProvider: gắn biến của một Brand lên phần tử bọc — hỗ trợ phạm vi lồng nhau
// (brand con thắng brand cha theo cascade var() tự nhiên). display: contents nên
// không thay đổi layout. applyBrand(brand) dùng khi muốn gắn thẳng lên :root.
import { useMemo, type CSSProperties, type ReactNode } from 'react'
import type { Brand } from './brand'

export interface ThemeProviderProps {
  brand: Brand
  children: ReactNode
  /** Thêm style riêng lên wrapper (hiếm khi cần). */
  style?: CSSProperties
}

export function ThemeProvider({ brand, children, style }: ThemeProviderProps) {
  const merged = useMemo<CSSProperties>(
    () => ({ display: 'contents', ...(brand.vars as CSSProperties), ...style }),
    [brand, style],
  )
  return <div data-brand={brand.name} style={merged}>{children}</div>
}
