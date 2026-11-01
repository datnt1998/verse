// Phạm vi Khám phá dùng chung cho bản máy tính và điện thoại: ?rival= của
// /explore là handle (@…) nên đối chiếu handle trước rồi mới tới id.
import type { Rival } from '../../api/rivals'

export type Scope = 'all' | string

export const scopeOf = (param: string | null | undefined, rivals: Rival[]): Scope => {
  if (!param) return 'all'
  const hit = rivals.find((rival) => rival.handle === param || rival.id === param)
  return hit ? hit.id : 'all'
}
