// Brand Noverse: toàn bộ vai trò ghim đúng literal hiện hữu → theme mặc định
// tái tạo tokens.stylex.ts 1:1 (xem brand.test.ts). roles pin hết nên không
// công thức derive nào chạy; mọi token chromatic nằm ngoài ROLE_LITERALS
// (biểu tượng, avatar nhân vật) được FIXED_TOKENS giữ nguyên.
import { defineBrand, type Brand, type BrandCore, type RoleName } from './brand'
import { ROLE_LITERALS } from './tokenDefaults'

const pinnedRoles = Object.fromEntries(
  (Object.keys(ROLE_LITERALS) as (keyof typeof ROLE_LITERALS)[]).map((k) => [k, ROLE_LITERALS[k][0]]),
) as Record<RoleName, string>

/** Core Noverse — tham số cho defineBrand; app mặc định không cần gọi (token đã là literal). */
export const noverseCore: BrandCore = {
  name: 'noverse',
  primary: ROLE_LITERALS.accent[0],
  roles: pinnedRoles,
}

/** Brand mặc định của kit — applyBrand(NOVERSE) tương đương theme hiện tại. */
export const NOVERSE: Brand = defineBrand(noverseCore)
