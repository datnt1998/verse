/** Máy chủ trả 409 `{needsConfig:true}` khi lệnh cần model mà chưa cấu hình nhà cung cấp (ApiError.payload). Thử lại không sửa được. */
export function isNeedsConfig(error: unknown): boolean {
  const payload = typeof error === 'object' && error !== null && 'payload' in error ? error.payload : null
  return typeof payload === 'object' && payload !== null && 'needsConfig' in payload && payload.needsConfig === true
}
