// Cài đặt › Báo lỗi — Settings.dc.html (section=logs): thẻ xuất .zip (một nút
// primary), “Lỗi gần đây” 7 ngày từ /api/errors, thẻ phiên bản (Kiểm tra bản
// mới gọi POST thật). Artboard không có bảng log: nhật ký thô chỉ đi trong tệp
// .zip đã che thông tin.
import { useState } from 'react'
import { Icon, Spinner, useAsyncAction } from '@noverse/ui'
import { setData } from '@noverse/ui/data'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { vi as t } from '../../../i18n/vi'
import { getErrors } from '../../../api/errors'
import { getVersion, checkVersion, type AppVersion } from '../../../api/version'
import { logsExportUrl } from '../../../api/logs'
import { semverOf } from '../../../lib/version'
import { ActionButton } from '../../../components/ActionButton'
import { fill } from '../fmt'
import './logs.css'

/** Thẻ “Lỗi gần đây” — chấm cam khi chưa xong, xám khi app đã tự xử lý. */
function ErrorsCard() {
  const res = useResource(keys.errors(), ({ signal }) => getErrors(signal))
  const items = res.data?.items ?? []
  return (
    <section className="card vv-logs__card" style={{ padding: 0 }} aria-labelledby="h-err">
      <div className="vv-logs__err-head">
        <h2 id="h-err">{t.logs.errors.title}</h2>
        <span>{fill(t.logs.errors.subtitle, { n: res.data?.count7d ?? items.length })}</span>
      </div>
      {res.status === 'loading' && !res.data ? <Spinner size={16} /> : null}
      {res.status !== 'loading' && items.length === 0 ? (
        <div className="vv-logs__err-it is-empty">{t.logs.errors.empty}</div>
      ) : null}
      {items.map((e) => (
        <div key={e.id} className="vv-logs__err-it">
          <span className={`vv-dot ${e.tone === 'warn' ? 'vv-dot--err' : 'vv-dot--muted'}`} aria-hidden="true" />
          <span className="vv-logs__err-time num">{e.time}</span>
          <div className="vv-logs__err-main">
            <span className="vv-logs__err-what">{e.what}</span>
            <span className="vv-logs__err-why">{e.why}</span>
            <span className={`vv-logs__err-st${e.tone === 'warn' ? ' is-err' : ' is-muted'}`}>{e.status}</span>
          </div>
          {e.href ? <a className="vv-logs__err-link" href={e.href}>{e.link ?? e.href}</a> : null}
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
    <section className="card vv-logs__version">
      <div className="vv-logs__version-main">
        <span className="vv-logs__version-name">Viderse {v?.version ? semverOf(v.version) : '…'}</span>
        <span className="vv-logs__version-sub">
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
      <section className="card vv-logs__export" aria-labelledby="h-exp">
        <div className="vv-logs__export-main">
          <h2 id="h-exp">{t.logs.export.title}</h2>
          <span className="vv-logs__sub">{t.logs.export.desc}</span>
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
