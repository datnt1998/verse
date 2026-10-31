
export function rulesSub(count: number | undefined): string {
  if (count === undefined) return 'Đang đếm…'
  return count === 0 ? 'Chưa có quy tắc nào' : `${count} quy tắc`
}
