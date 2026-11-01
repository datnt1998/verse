// Đọc payload JSON ở biên mà không cần thư viện schema: đường khởi động chỉ có
// vài kiểu (activity, thông báo, cài đặt thông báo) nên viết tay rẻ hơn nhiều so
// với kéo cả zod vào entry. Sai dạng thì ném ngay tại trường sai, thông điệp chỉ
// đúng đường dẫn để log đọc được.
export class ShapeError extends Error {
  constructor(public path: string) {
    super(`Dữ liệu máy chủ sai dạng: ${path}`)
    this.name = 'ShapeError'
  }
}

const fail = (path: string): never => {
  throw new ShapeError(path)
}

/** Type guard: thu hẹp `unknown` thành object thường (không mảng, không null). */
export const record = (value: unknown, path: string): Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : fail(path)

/** Type guard: thu hẹp `unknown` thành string. */
export const str = (value: unknown, path: string): string => (typeof value === 'string' ? value : fail(path))

/** Type guard: thu hẹp `unknown` thành number hữu hạn. */
export const num = (value: unknown, path: string): number =>
  typeof value === 'number' && Number.isFinite(value) ? value : fail(path)

/** Type guard: thu hẹp `unknown` thành boolean. */
export const bool = (value: unknown, path: string): boolean => (typeof value === 'boolean' ? value : fail(path))

export const oneOf = <T extends string>(value: unknown, allowed: readonly T[], path: string): T =>
  typeof value === 'string' && (allowed as readonly string[]).includes(value) ? (value as T) : fail(path)

export const list = <T>(value: unknown, item: (entry: unknown, path: string) => T, path: string): T[] =>
  Array.isArray(value) ? value.map((entry, index) => item(entry, `${path}[${index}]`)) : fail(path)

/** Trường vắng mặt (`undefined`/`null`) đọc thành `undefined`; có thì đọc bình thường. */
export const maybe = <T>(value: unknown, read: (entry: unknown, path: string) => T, path: string): T | undefined =>
  value == null ? undefined : read(value, path)

/** Trường bắt buộc-nhưng-có-thể-null: `null` giữ nguyên `null`. */
export const nullable = <T>(value: unknown, read: (entry: unknown, path: string) => T, path: string): T | null =>
  value === null ? null : read(value, path)

/** `.nullish()` của zod cũ: `null` giữ `null`, vắng mặt đọc thành `undefined`. */
export const nullish = <T>(value: unknown, read: (entry: unknown, path: string) => T, path: string): T | null | undefined =>
  value === null ? null : value === undefined ? undefined : read(value, path)
