import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, logsExportMarker, logsErrHeadMarker, logsErrItMarker } from '../settings.stylex'
// Cài đặt › Báo lỗi — Settings.dc.html (section=logs): thẻ xuất .zip (một nút
// primary), “Lỗi gần đây” 7 ngày từ /api/errors, thẻ phiên bản (Kiểm tra bản
// mới gọi POST thật). Artboard không có bảng log: nhật ký thô chỉ đi trong tệp
// .zip đã che thông tin.
import { useState } from 'react'
import { Icon, Spinner, useAsyncAction } from '@noverse/ui'
import { setData } from '@noverse/ui/data'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { t } from '../../../i18n/settings'
import { getErrors } from '../../../api/errors'
import { getVersion, checkVersion, type AppVersion } from '../../../api/version'
import { logsExportUrl } from '../../../api/logs'
import { semverOf } from '../../../lib/version'
import { ActionButton } from '../../../components/ActionButton'
import { fill } from '../fmt'

/** Thẻ “Lỗi gần đây” — chấm cam khi chưa xong, xám khi app đã tự xử lý. */
function ErrorsCard() {
  const res = useResource(keys.errors(), ({ signal }) => getErrors(signal))
  const items = res.data?.items ?? []
  return (
    <section className={`vv-logs__card ${stylex.props(settingsStyles.card, settingsStyles.logsCard).className}`} style={{ padding: 0 }} aria-labelledby="h-err">
      <div className={`vv-logs__err-head ${stylex.props(logsErrHeadMarker, settingsStyles.logsErrHead).className}`}>
        <h2 id="h-err" {...stylex.props(settingsStyles.VvLogsErrHeadH2)}>{t.logs.errors.title}</h2>
        <span {...stylex.props(settingsStyles.VvLogsErrHeadSpan)}>{fill(t.logs.errors.subtitle, { n: res.data?.count7d ?? items.length })}</span>
      </div>
      {res.status === 'loading' && !res.data ? <Spinner size={16} /> : null}
      {res.status !== 'loading' && items.length === 0 ? (
        <div className={`vv-logs__err-it is-empty ${stylex.props(logsErrItMarker, settingsStyles.logsErrIt, settingsStyles.logsErrItIsEmpty).className}`}>{t.logs.errors.empty}</div>
      ) : null}
      {items.map((e) => (
        <div key={e.id} className={`vv-logs__err-it ${stylex.props(logsErrItMarker, settingsStyles.logsErrIt).className}`}>
          <span className={`vv-dot ${stylex.props(settingsStyles.dot, (e.tone === 'warn') && settingsStyles.dotErr, (!(e.tone === 'warn')) && settingsStyles.dotMuted).className}`} aria-hidden="true" />
          <span className={`vv-logs__err-time num ${stylex.props(settingsStyles.logsErrTime).className}`}>{e.time}</span>
          <div className={`vv-logs__err-main ${stylex.props(settingsStyles.logsErrMain).className}`}>
            <span className={`vv-logs__err-what ${stylex.props(settingsStyles.logsErrWhat).className}`}>{e.what}</span>
            <span className={`vv-logs__err-why ${stylex.props(settingsStyles.logsErrWhy).className}`}>{e.why}</span>
            <span className={`vv-logs__err-st ${stylex.props(settingsStyles.logsErrSt, (e.tone === 'warn') && settingsStyles.logsErrStIsErr, (!(e.tone === 'warn')) && settingsStyles.logsErrStIsMuted).className}`}>{e.status}</span>
          </div>
          {e.href ? <a className={`vv-logs__err-link ${stylex.props(settingsStyles.logsErrLink).className}`} href={e.href}>{e.link ?? e.href}</a> : null}
        </div>
      ))}
    </section>
  )
}

/** Thẻ phiên bản — “Kiểm tra bản mới” gọi POST /api/version/check thật. */
function VersionCard() {
  const res = useResource(keys.appVersion(), ({ signal }) => getVersion(signal))
  // Kết quả lần kiểm tra vừa rồi hiện ngay, không chờ lượt tải lại.
  const [checked, setChecked] = useState<AppVersion | null>(null)
  const check = useAsyncAction(async () => {
    const next = await checkVersion()
    if (next) { setChecked(next); setData(keys.appVersion(), next) }
  })
  const v = checked ?? res.data
  return (
    <section className={`vv-logs__version ${stylex.props(settingsStyles.card, settingsStyles.logsVersion).className}`}>
      <div className={`vv-logs__version-main ${stylex.props(settingsStyles.logsVersionMain).className}`}>
        <span className={`vv-logs__version-name ${stylex.props(settingsStyles.logsVersionName).className}`}>Viderse {v?.version ? semverOf(v.version) : '…'}</span>
        <span className={`vv-logs__version-sub ${stylex.props(settingsStyles.logsVersionSub).className}`}>
          {v?.latest ? fill(t.logs.version.hasUpdate, { version: v.latest }) : t.logs.version.latest}
        </span>
      </div>
      <ActionButton size="compact" pending={check.pending} icon={<Icon name="refresh" size={15} />} onClick={() => void check.run()}>
        {t.logs.version.check}
      </ActionButton>
    </section>
  )
}

export default function LogsSection() {
  return (
    <>
      <section className={`vv-logs__export ${stylex.props(settingsStyles.card, logsExportMarker, settingsStyles.logsExport).className}`} aria-labelledby="h-exp">
        <div className={`vv-logs__export-main ${stylex.props(settingsStyles.logsExportMain).className}`}>
          <h2 id="h-exp" {...stylex.props(settingsStyles.VvLogsExportH2)}>{t.logs.export.title}</h2>
          <span className={`vv-logs__sub ${stylex.props(settingsStyles.logsSub).className}`}>{t.logs.export.desc}</span>
        </div>
        {/* Artboard: nút primary của thẻ Xuất log là .btn primary 36/r10/13 (không phải
            .btn ghost 32) — để cỡ mặc định của kit. */}
        <ActionButton variant="primary" icon={<Icon name="export" size={16} />} onClick={() => window.location.assign(logsExportUrl)}>
          {t.logs.export.action}
        </ActionButton>
      </section>
      <ErrorsCard />
      <VersionCard />
    </>
  )
}
