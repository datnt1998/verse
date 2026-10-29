import { styleLabel } from './storySetLogic'

export function rulesSub(count: number | undefined): string {
  if (count === undefined) return 'Đang đếm…'
  return count === 0 ? 'Chưa có quy tắc nào' : `${count} quy tắc`
}

export function storySetSub(style: string | undefined, review: boolean): string {
  return [style ? styleLabel(style) : style, review ? 'duyệt từng chương' : 'tự động'].filter(Boolean).join(' · ')
}
