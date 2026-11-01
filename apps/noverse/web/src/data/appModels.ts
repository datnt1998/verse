// Gói AU — cấu hình model dùng chung cả app cho màn điện thoại và CmdResults.
// Đọc `GET /api/config?scope=global`; ghi một bản hoàn chỉnh (không ghi từng trường) và
// làm mới mọi nơi đang đọc cùng khoá.
import { getConfig, putConfig } from '../api/client'
import type { Config, ConfigResponse } from '../api/types'
import { configForCompare, configForSave } from '../screens/settings/lib/config'
import { keys } from './keys'
import { useResource } from './useResource'
import { invalidate } from '@noverse/ui/data'

export const appConfigKey = keys.config('global')

/** `enabled=false`: màn chỉ cần cấu hình khi thật sự có việc (vd thẻ dừng chỉ khi có lỗi). */
export const useAppConfig = (enabled = true) =>
  useResource(appConfigKey, ({ signal }) => getConfig('global', undefined, undefined, signal), { enabled })

/** Bản cấu hình chung đang hiệu lực (đã bỏ khoá rỗng) — nguồn của mọi ô chọn model. */
export const appConfigDoc = (response: ConfigResponse | null | undefined): Config =>
  response ? configForCompare(response.target.config) : {}

/** Ghi đúng bản đã sửa; `response` chỉ dùng cho If-Match. Trả bản mới để màn hình đối chiếu. */
export async function putAppConfig(response: ConfigResponse, next: Config): Promise<ConfigResponse> {
  const saved = await putConfig('global', configForSave(next), undefined, undefined, response.version)
  void invalidate(['config', 'global'])
  return saved
}
