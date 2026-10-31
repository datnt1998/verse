// Nhóm “Vị trí lưu trên máy chủ” — Settings.dc.html advGroups[0]: Nơi lưu truyện · Mở nơi lưu truyện · Tệp cấu hình · Thư mục quy tắc.
// Hai hàng đầu do LibraryRootRows giữ; “Tệp cấu hình” có nút Mở (chọn đúng tệp, target=config) hoặc Chép đường dẫn khi không ở trên máy chủ.
import { useState } from 'react'
import { ApiError, getConfig, getRevealPath, getRules, revealFolder } from '../../../api/client'
import type { RevealRequest } from '../../../api/types'
import { keys, useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { Skeleton } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { LibraryRootRows } from '../sections/LibraryRootRow'
import type { SettingsContext } from '../types'
import { AdvButton, AdvRow } from './AdvGroup'
import { configPathValue, rulesPathValue } from './logic'
import { copyText } from '../../../lib/clipboard'

/** Máy chủ từ chối mở thư mục khi không dùng ngay trên máy chủ (reveal.go revealLoopbackError). */
const NEED_SERVER = 'Chỉ mở được thư mục khi dùng ngay trên máy chủ'

export function LocationRows({ ctx }: { ctx: SettingsContext }) {
  const scope = ctx.viewScope
  const hosted = ctx.library?.storageMode === 'postgres'
  const project = scope === 'global' ? undefined : ctx.project || undefined
  const story = scope === 'story' ? ctx.story || undefined : undefined
  const rules: RevealRequest = { target: 'rules', scope, project, story }
  const configTarget: RevealRequest = { target: 'config', scope, project, story }
  const ready = scope === 'global' || (!!project && (scope !== 'story' || !!story))
  const rulesPath = useResource(['reveal', 'rules', scope, project ?? '', story ?? ''], () => getRevealPath(rules), { enabled: !hosted })
  // Đường dẫn tệp cấu hình của cấp đang chọn (mục Hệ thống chỉ tải bản toàn cục nên hỏi riêng cấp được chọn).
  const configFile = useResource(keys.config(scope, project, story), ({ signal }) => getConfig(scope, project, story, signal), { enabled: ready })
  const ruleFiles = useResource(['rules', scope, project ?? '', story ?? ''], ({ signal }) => getRules(scope, project, story, signal), { enabled: ready })
  const configPath = configFile.data?.target.path
  // “· N tệp” chỉ ở cấp dự án/truyện (artboard: cấp toàn cục chỉ ghi đường dẫn).
  const ruleCount = scope === 'global' ? null : ruleFiles.data?.chain.find((level) => level.scope === scope)?.files.length ?? null
  const rulesValue = rulesPath.data?.path
  const [opening, setOpening] = useState<'rules' | 'config' | null>(null)

  const run = async (key: 'rules' | 'config', work: () => Promise<string>) => {
    setOpening(key)
    try {
      toast.ok(await work())
    } catch (cause) {
      toast.err(cause instanceof ApiError ? cause.message : 'Không mở được thư mục trên máy chủ.')
    } finally {
      setOpening(null)
    }
  }
  const openRules = () => run('rules', async () => { await revealFolder(rules); return 'Đã mở thư mục trên máy chủ' })
  // Mở được thì chọn đúng tệp trên máy chủ; ngoài loopback thì chép đường dẫn tuyệt đối (GET không mở gì).
  const openConfig = () => run('config', async () => {
    if (ctx.canReveal) { await revealFolder(configTarget); return 'Đã mở thư mục trên máy chủ' }
    const { path } = await getRevealPath(configTarget)
    try { await copyText(path) } catch { throw new Error('Trình duyệt chặn sao chép — hãy chọn và sao chép thủ công.') }
    return 'Đã chép đường dẫn'
  })

  if (hosted) return <LibraryRootRows library={ctx.library} canReveal={false} onChanged={ctx.reloadLibrary} />
  return <>
    <LibraryRootRows library={ctx.library} canReveal={ctx.canReveal} onChanged={ctx.reloadLibrary} />
    <AdvRow label="Tệp cấu hình (cấp đang chọn)" value={configPath ? configPathValue(configPath, scope) : (configFile.error ? t.adv.access.loadFailed : <Skeleton width={220} height={14} />)}
      valueTitle={configPath}>
      <AdvButton pending={opening === 'config'} disabled={!configPath} onClick={() => void openConfig()}>{ctx.canReveal ? 'Mở' : 'Chép đường dẫn'}</AdvButton>
    </AdvRow>
    <AdvRow label="Thư mục quy tắc"
      value={rulesValue ? (ruleCount === null ? rulesPathValue(rulesValue, scope) : `${rulesPathValue(rulesValue, scope)} · ${ruleCount} tệp`) : (rulesPath.error ? t.adv.access.loadFailed : <Skeleton width={220} height={14} />)}
      valueTitle={rulesValue}>
      <AdvButton pending={opening === 'rules'} disabled={!ctx.canReveal} title={ctx.canReveal ? undefined : NEED_SERVER}
        onClick={() => void openRules()}>Sửa dạng văn bản</AdvButton>
    </AdvRow>
  </>
}
