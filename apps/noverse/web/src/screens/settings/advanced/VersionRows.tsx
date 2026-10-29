// Nâng cao › nhóm “Phiên bản Noverse” (Settings.dc.html advGroups[0]) — D1.
// Ba hàng: Đang chạy · Mã build (Chép) · Bản mới nhất (Có gì mới). Dữ liệu từ `GET /api/system`.
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { buildStamp, latestLine, runningLine, shortCommit, useVersionInfo, versionText } from '../../../shell/version'
import { AdvButton, AdvRow } from './AdvGroup'
import { copyText } from '../../../lib/clipboard'

export function VersionRows({ onWhatsNew }: { onWhatsNew: () => void }) {
  const info = useVersionInfo()
  const commit = (info?.commit ?? '').trim()
  const short = shortCommit(commit)

  const copy = async () => {
    try {
      await copyText(commit)
      toast.ok('Đã chép mã build')
    } catch {
      toast.err('Trình duyệt chặn sao chép — hãy chọn và sao chép thủ công.')
    }
  }

  return (
    <>
      <AdvRow label="Đang chạy" value={runningLine(info) || '—'} valueTitle={buildStamp(info?.built) ? `Build ${buildStamp(info?.built)}` : undefined} />
      <AdvRow label="Mã build" value={short || '—'} valueTitle={commit || undefined}>
        {short && <AdvButton onClick={() => void copy()}>Chép</AdvButton>}
      </AdvRow>
      <AdvRow label="Bản mới nhất" value={latestLine(info) || '—'}>
        <AdvButton onClick={onWhatsNew} title={`Có gì mới · ${versionText(info?.latestVersion) || versionText(info?.version)}`}>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
            Có gì mới<Icon name="arrow-right-2" size={12} />
          </span>
        </AdvButton>
      </AdvRow>
    </>
  )
}
