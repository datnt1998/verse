import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, cleanlistMarker } from '../settings.stylex'
import { useState } from 'react'
import { Button, Icon, useAsyncAction } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { previewCleanup, runCleanup, type CleanupPreview, type StorageStatus } from '../../../api/storage'
import { ApiError } from '../../../api/request'
import { t } from '../../../i18n/settings'
import { serverStorageCopy } from '../../../i18n/storageServer'
import { keys } from '../../../data/keys'
import { useSession } from '../../../api/session'
import { fill, fmtBytes, fmtTime } from '../fmt'

/** Thẻ “Tệp tạm và thứ không ai dùng” — xem trước rồi dọn (hai bước previewId). */
export function CleanupCard({ data, serverRole }: { data?: StorageStatus; serverRole?: 'admin' | 'member' }) {
  const { user } = useSession()
  const [preview, setPreview] = useState<CleanupPreview | null>(null)
  const [done, setDone] = useState<{ bytes: number; at: string } | null>(null)
  const [stale, setStale] = useState(false)
  const previewAct = useAsyncAction(async () => { setStale(false); setPreview(await previewCleanup() ?? null) })
  const runAct = useAsyncAction(async () => {
    if (!preview) return
    try {
      const res = await runCleanup(preview.previewId)
      if (res) { setDone({ bytes: res.freedBytes, at: new Date().toISOString() }); setPreview(null); invalidate(keys.storage()) }
    } catch (err) {
      // 409 preview_required: bản xem trước cũ (quá 15 phút hoặc đã đổi) —
      // xem lại ngay, người dùng chỉ cần bấm Dọn lần nữa.
      const payload = err instanceof ApiError ? err.payload : null
      if (!payload || typeof payload !== 'object' || !('code' in payload) || payload.code !== 'preview_required') throw err
      setPreview(await previewCleanup())
      setStale(true)
    }
  })
  // Dọn dữ liệu là thao tác của quản trị viên; máy chủ từ chối thành viên (403).
  if (serverRole === 'member' || user?.role === 'member') return null
  const items = (preview?.items ?? []).filter(item => !serverRole || item.kind === 'orphan_objects' || serverRole === 'admin' && item.kind === 'work_cache').map(item =>
    serverRole ? { ...item, label: serverStorageCopy.cleanupLabels[item.kind as keyof typeof serverStorageCopy.cleanupLabels] } : item)
  const freedBytes = serverRole ? items.reduce((sum, item) => sum + item.bytes, 0) : preview?.freedBytes ?? 0
  return (
    <section className={`vv-sto__card ${stylex.props(settingsStyles.card, settingsStyles.stoCard).className}`} aria-labelledby="h-tmp">
      <div className={`vv-sto__cardHead ${stylex.props(settingsStyles.stoCardHead).className}`}>
        <div className={`vv-sto__cardText ${stylex.props(settingsStyles.stoCardText).className}`}>
          <h2 id="h-tmp" className={`vv-sto__h2 ${stylex.props(settingsStyles.stoH2).className}`}>{t.storage.cleanup.title}</h2>
          <span className={`vv-sto__sub ${stylex.props(settingsStyles.stoSub).className}`}>
            {data?.clean.lastRunAt ? fill(t.storage.cleanup.sub, { when: fmtTime(data.clean.lastRunAt) }) : t.storage.cleanup.subNever}
          </span>
          {stale ? <span className={`vv-err ${stylex.props(settingsStyles.err).className}`}>{t.storage.cleanup.stale}</span> : null}
        </div>
        {!preview && !done ? (
          <Button variant="ghost" size="compact" pending={previewAct.pending}
            icon={<Icon name="eye" size={14} />} onClick={() => void previewAct.run()}>
            {t.storage.cleanup.preview}
          </Button>
        ) : null}
      </div>
      {preview ? (
        <>
          <div role="list" className={`vv-cleanlist ${stylex.props(cleanlistMarker, settingsStyles.cleanlist).className}`}>
            {items.length === 0 ? <span {...stylex.props(settingsStyles.VvCleanlistSpan)}><span className={`vv-lb ${stylex.props(settingsStyles.lb).className}`}>{t.storage.cleanup.empty}</span></span> : null}
            {items.map((it) => (
              <span key={it.kind} role="listitem" {...stylex.props(settingsStyles.VvCleanlistSpan)}>
                <span className={`vv-lb ${stylex.props(settingsStyles.lb).className}`}>
                  {!serverRole && it.bytes === 0 && it.count > 0
                    ? fill(t.storage.cleanup.stoppedCount, { n: it.count, label: it.label })
                    : (it.kind === 'media' || serverRole && it.kind === 'orphan_objects') && it.count > 1
                      ? `${it.label} · ${fill(t.storage.cleanup.files, { n: it.count })}`
                      : it.label}
                </span>
                <span className={`vv-num ${stylex.props(settingsStyles.num, (!(it.bytes > 0)) && settingsStyles.numIsMuted).className}`}>{it.bytes > 0 || serverRole ? fmtBytes(it.bytes) : t.storage.cleanup.stopped}</span>
              </span>
            ))}
          </div>
          <div className={`vv-sto__go ${stylex.props(settingsStyles.stoGo).className}`}>
            <span className={`vv-sto__previewNote ${stylex.props(settingsStyles.stoPreviewNote).className}`}>{serverRole ? serverStorageCopy.cleanupWarning : t.storage.cleanup.previewNote}</span>
            <Button variant="quiet" size="compact" onClick={() => setPreview(null)}>{t.storage.cleanup.later}</Button>
            <Button variant="primary" size="compact" pending={runAct.pending}
              disabled={freedBytes <= 0 && (!!serverRole || preview.stoppedProcesses <= 0)}
              title={freedBytes <= 0 && (!!serverRole || preview.stoppedProcesses <= 0) ? t.storage.cleanup.empty : undefined}
              onClick={() => void runAct.run()}>
              {fill(t.storage.cleanup.run, { bytes: fmtBytes(freedBytes) })}
            </Button>
          </div>
        </>
      ) : null}
      {done ? (
        <span role="status" className={`vv-sto__doneLine ${stylex.props(settingsStyles.stoDoneLine).className}`}>
          <Icon name="tick-b" size={16} />{fill(t.storage.cleanup.ran, { bytes: fmtBytes(done.bytes), time: fmtTime(done.at) })}
        </span>
      ) : null}
    </section>
  )
}
