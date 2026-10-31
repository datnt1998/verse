import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, Spinner } from '@noverse/ui'
import { testStorage } from '../../../api/client'
import type { StorageView } from '../../../api/storage-settings'
import { storageCopy as c } from '../../../i18n/vi'
import { bytes, completeBrowserTest, migrating, num, percent, time } from './logic'
import { AdminDetails, CopyCode, providerOf } from './form'
import { sx } from './styles'

export function storageName(view: StorageView) { return view.active?.driver === 's3' ? providerOf(view.active.provider).name : c.local }
export function storageSummary(view: StorageView) {
  const migration = view.migration
  if (migrating(migration) && migration) return migration.errorCount > 0 || migration.state === 'error' ? `Chuyển kho: ${num(migration.errorCount)} tệp lỗi` : migration.state === 'paused' ? `Chuyển kho tạm dừng · ${percent(migration.copiedCount, migration.totalCount)}%` : `Đang chuyển · ${percent(migration.copiedCount, migration.totalCount)}%`
  if (view.health?.error) return c.down
  if (view.mode === 'file') return 'Trong thư mục truyện'
  return `${storageName(view)} · ${bytes(view.total?.bytes ?? 0)}`
}
export function MigrationCard({ view, mobile = false, busy = false, onControl }: { view: StorageView; mobile?: boolean; busy?: boolean; onControl?: (action: 'pause' | 'resume' | 'cancel' | 'start') => void }) {
  const [cancel, setCancel] = useState(false)
  const migration = view.migration
  if (!migration || !migrating(migration)) return null
  const target = providerOf(view.pending?.provider).name
  const error = migration.state === 'error' || migration.errorCount > 0
  const paused = migration.state === 'paused'
  const pct = percent(migration.copiedCount, migration.totalCount)
  if (mobile) return <section aria-label="Chuyển kho tệp" {...stylex.props(sx.card, sx.mobileCard, sx.infoBorder)}>
    <div {...stylex.props(sx.row)}><b {...stylex.props(sx.heading, sx.grow)}>{error ? `Chuyển kho: ${num(migration.errorCount)} tệp lỗi` : `Đang chuyển sang ${target}`}</b><span {...stylex.props(sx.small, sx.num, error ? sx.warnText : sx.infoText)}>{paused ? c.paused : `${pct}%`}</span></div>
    <span role="progressbar" aria-label={c.files} aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} {...stylex.props(sx.bar)}><span {...stylex.props(sx.barFill(pct), error && sx.warnFill)} /></span>
    <span {...stylex.props(sx.note, sx.num)}>{num(migration.copiedCount)} / {num(migration.totalCount)} tệp · {bytes(migration.copiedBytes)} / {bytes(migration.totalBytes)}{migration.bytesPerSecond > 0 && !paused ? ` · còn khoảng ${Math.max(1, Math.ceil((migration.totalBytes - migration.copiedBytes) / migration.bytesPerSecond / 60))} phút` : ''}</span>
    <span {...stylex.props(sx.note)}>{error ? c.migrationErrors : c.mobileMigration}</span>
  </section>
  return <section aria-label="Chuyển kho tệp" {...stylex.props(sx.card, sx.infoBorder, mobile && sx.mobileCard)}>
    <div {...stylex.props(sx.row, sx.wrap)}><Icon name="arrow-swap-horizontal" size={18} {...stylex.props(sx.infoText)} /><b {...stylex.props(sx.heading, sx.grow)}>{error ? `Chuyển sang ${target} · còn ${num(migration.errorCount)} tệp lỗi` : `Đang chuyển sang ${target}`}</b><span {...stylex.props(sx.chip, error ? sx.warnChip : paused ? sx.pausedChip : sx.infoChip)}>{error ? c.hasErrors : paused ? c.paused : c.running}</span></div>
    {!mobile && <p {...stylex.props(sx.sub)}>Bắt đầu lúc {time(migration.startedAt)}. {c.migrationBody}</p>}
    <div {...stylex.props(sx.grid, mobile && sx.mobileGrid)}>{[[c.files, `${num(migration.copiedCount)} / ${num(migration.totalCount)} · ${pct}%`, pct], [c.volume, `${bytes(migration.copiedBytes)} / ${bytes(migration.totalBytes)}`, percent(migration.copiedBytes, migration.totalBytes)]].map(([label, value, progress]) => <div key={label} {...stylex.props(sx.stack)}><div {...stylex.props(sx.row, sx.small)}><span {...stylex.props(sx.grow, sx.muted)}>{label}</span><span {...stylex.props(sx.num)}>{value}</span></div><span role="progressbar" aria-label={String(label)} aria-valuenow={Number(progress)} aria-valuemin={0} aria-valuemax={100} {...stylex.props(sx.bar)}><span {...stylex.props(sx.barFill(Number(progress)), error && sx.warnFill)} /></span></div>)}</div>
    <div {...stylex.props(sx.row, sx.wrap, sx.small, sx.muted)}><span {...stylex.props(sx.grow)}>Tốc độ trung bình {bytes(migration.bytesPerSecond)}/giây{migration.bytesPerSecond > 0 ? ` · còn khoảng ${Math.max(1, Math.ceil((migration.totalBytes - migration.copiedBytes) / migration.bytesPerSecond / 60))} phút` : ''}</span><span>{c.verified} {num(migration.copiedCount)}</span><span {...stylex.props(error && sx.warnText)}>{c.errors} {num(migration.errorCount)}</span></div>
    {error && <><div {...stylex.props(sx.table)}><div {...stylex.props(sx.errorRow, sx.muted)}><span>{c.fileKind}</span><span>{c.story}</span><span>{c.reason}</span></div>{migration.errors.map((item, index) => <div key={index} {...stylex.props(sx.errorRow)}><span>{c.kinds[item.kind] ?? c.title}</span><span title={item.storyTitle || c.shared} {...stylex.props(sx.truncate)}>{item.storyTitle || c.shared}</span><span>{c.migrationReasons[item.code] ?? item.reason}</span></div>)}</div><span {...stylex.props(sx.small)}>{c.migrationErrors}</span></>}
    {mobile ? <p {...stylex.props(sx.sub)}>{c.mobileMigration}</p> : <>{cancel && <div role="alertdialog" aria-label={c.cancelMigration} {...stylex.props(sx.inset, sx.warning, sx.stack)}><b>{c.cancelQuestion}</b><span {...stylex.props(sx.small)}>App vẫn dùng {storageName(view)}, không mất tệp nào. {num(migration.copiedCount)} tệp đã chép sang {target} vẫn nằm ở đó.</span><div {...stylex.props(sx.row)}><button type="button" {...stylex.props(sx.button)} disabled={busy} onClick={() => setCancel(false)}>{c.keep}</button><button type="button" {...stylex.props(sx.button, sx.errorText)} disabled={busy} onClick={() => onControl?.('cancel')}>{c.cancelMigration}</button></div></div>}<div {...stylex.props(sx.row)}><button type="button" {...stylex.props(sx.button, error && sx.primary)} disabled={busy} onClick={() => onControl?.(error || paused ? 'resume' : 'pause')}>{error ? c.retryFailed : paused ? c.resume : c.pause}</button><button type="button" {...stylex.props(sx.button, sx.ghost)} disabled={busy} onClick={() => setCancel(true)}>{c.cancelMigration}</button></div></>}
  </section>
}

export function StorageOverview({ view, mobile = false, onPick, onRefresh }: { view: StorageView; mobile?: boolean; onPick?: () => void; onRefresh: () => Promise<unknown> }) {
  const [testing, setTesting] = useState(false)
  const [result, setResult] = useState('')
  const [testFailed, setTestFailed] = useState(false)
  const active = view.active
  const provider = providerOf(active?.provider)
  const down = !!view.health?.error
  const run = async () => {
    if (testing) return
    setTesting(true); setResult(''); setTestFailed(false)
    try {
      const test = await completeBrowserTest(await testStorage({ current: true }), active?.delivery ?? 'proxy')
      setTestFailed(!test.ok)
      const failed = test.steps.find((step) => !step.ok)
      setResult(test.ok ? `Kết nối đạt · 6 bước · tổng ${test.steps.reduce((sum, step) => sum + step.ms, 0)} ms` : failed ? c.failTitles[failed.code] ?? failed.reason : c.actionError)
      await onRefresh()
    } catch (error) { setTestFailed(true); setResult(error instanceof Error ? error.message : c.actionError) }
    finally { setTesting(false) }
  }
  if (view.mode === 'file') return <section aria-label={c.fileTitle} {...stylex.props(sx.card, mobile && sx.mobileCard)}><Icon name="folder-open" size={20} /><h2 {...stylex.props(sx.heading)}>{c.fileTitle}</h2><span {...stylex.props(sx.sub)}>{c.fileBody}</span><span {...stylex.props(sx.note)}>{c.fileHelp}</span></section>
  if (!active) return <p role="alert">{c.loadError}</p>
  const isS3 = active.driver === 's3'
  const rows = [
    ...(isS3 ? [[c.bucket, active.bucket || '—'], [c.region, active.region || '—'], [c.endpoint, active.endpointHost]] : []),
    [c.delivery, active.delivery === 'proxy' ? c.proxy : c.direct], [c.env, c.envNames[active.env] ?? '—'],
    ...(isS3 && !mobile ? [[c.accessKeyId, `••••${active.accessKeyLast4}`]] : []),
    ...(!mobile ? [[c.source, view.locked ? c.locked : isS3 ? c.appSource : c.defaultSource]] : []),
  ].filter(([label]) => !mobile || ![c.region, c.endpoint].includes(label))
  const kinds = [['audio', 'headphone'], ['export', 'export-square'], ['cover', 'layer'], ['music', 'music'], ['voiceSample', 'microphone-2'], ['import', 'document-text']]
  const maximum = Math.max(1, ...kinds.map(([kind]) => view.usage?.[kind]?.bytes ?? 0))
  const ops = ['put', 'get', ...(active.delivery === 'redirect' ? ['presign'] : []), 'delete'].map((op) => {
    const items = view.operations.filter((item) => item.op === op || op === 'get' && item.op === 'open' || op === 'presign' && item.op === 'presign_get')
    return { op, count: items.reduce((sum, item) => sum + item.count, 0), errors: items.filter((item) => item.outcome !== 'ok').reduce((sum, item) => sum + item.count, 0), bytes: items.reduce((sum, item) => sum + item.bytes, 0), seconds: items.reduce((sum, item) => sum + item.durationSeconds, 0), max: Math.max(0, ...items.map((item) => item.maxDurationSeconds)) }
  })
  return <>
    {down && <section role="alert" {...stylex.props(sx.card, sx.error, mobile && sx.mobileCard)}><div {...stylex.props(sx.row)}><Icon name="warning-2" size={20} /><b>{c.downTitle} {storageName(view)}</b></div><span {...stylex.props(sx.small)}>{view.health?.error}</span><span {...stylex.props(sx.note)}>Lần thử gần nhất {time(view.health?.lastCheck)}{view.health?.lastCheck ? ` · ${Math.max(0, Math.floor((Date.now() - Date.parse(view.health.lastCheck)) / 1000))} giây trước` : ''}</span><button type="button" {...stylex.props(sx.button)} disabled={testing} onClick={() => { void run() }}>{c.retest}</button></section>}
    {mobile && <h2 {...stylex.props(sx.heading)}>{c.active}</h2>}
    <section aria-label={c.active} {...stylex.props(sx.card, mobile && sx.mobileCard)}><div {...stylex.props(sx.row, sx.wrap)}><span {...stylex.props(sx.logo, !isS3 && sx.diskLogo)}>{isS3 ? provider.initials : <Icon name="box" size={20} />}</span><div {...stylex.props(sx.stack, sx.compactStack, sx.grow)}><div {...stylex.props(sx.row, sx.wrap)}><b {...stylex.props(sx.heading)}>{storageName(view)}</b>{!mobile && <span {...stylex.props(sx.chip, down && sx.errorChip)}>{down ? c.down : c.healthy}</span>}</div><span {...stylex.props(sx.note, mobile && (down ? sx.errorText : sx.okText))}>{mobile ? down ? c.down : c.healthy : c.active} · {view.health?.lastCheck ? `Kiểm tra gần nhất lúc ${time(view.health.lastCheck)}` : 'Chưa kiểm tra'}</span></div>{!mobile && <><button type="button" {...stylex.props(sx.button)} disabled={testing} onClick={() => { void run() }}><Icon name="refresh-2" size={14} />{c.test}</button>{isS3 && !view.locked && !migrating(view.migration) && <button type="button" {...stylex.props(sx.button)} onClick={onPick}>{c.change}</button>}</>}</div>
      {!mobile && (testing || result) && <div role="status" {...stylex.props(sx.inset, sx.row, testFailed ? sx.errorText : sx.okText)}>{testing ? <Spinner size={14} /> : <Icon name={testFailed ? 'warning-2' : 'tick-circle'} size={16} />}{testing ? c.testing : result}</div>}
      <div {...stylex.props(sx.grid, sx.metadataGrid, mobile && sx.mobileGrid)}>{rows.map(([label, value]) => <div key={label} {...stylex.props(sx.kv)}><span {...stylex.props(sx.kvLabel)}>{label}</span><span title={value} {...stylex.props(sx.grow, sx.truncate)}>{value}</span>{view.locked && !mobile && <span {...stylex.props(sx.chip, sx.warnChip)}>{c.locked}</span>}</div>)}</div>
      {view.locked && !mobile && <div {...stylex.props(sx.inset, sx.warning, sx.stack)}><span {...stylex.props(sx.small)}>{c.lockedBody}</span><AdminDetails><pre {...stylex.props(sx.code)}>{'STORAGE_DRIVER=s3\nS3_ENDPOINT  S3_REGION  S3_BUCKET\nS3_FORCE_PATH_STYLE  S3_ALLOW_INSECURE_ENDPOINT\nS3_PRESIGN_ENDPOINT  BLOB_DELIVERY=proxy|redirect\nS3_ACCESS_KEY_ID  S3_SECRET_ACCESS_KEY'}</pre></AdminDetails></div>}
    </section>
    {mobile && <section {...stylex.props(sx.card, sx.mobileCard)}><button type="button" {...stylex.props(sx.button)} disabled={testing} onClick={() => { void run() }}>{testing ? <Spinner size={14} /> : <Icon name="refresh-2" size={16} />}{c.test}</button>{result && <p role="status" {...stylex.props(sx.note, testFailed ? sx.errorText : sx.okText)}>{result}</p>}</section>}
    {!mobile && !isS3 && !view.locked && !migrating(view.migration) && <section aria-label={c.invite} {...stylex.props(sx.card, sx.row)}><Icon name="cloud-connection" size={22} /><div {...stylex.props(sx.stack, sx.grow)}><h2 {...stylex.props(sx.heading)}>{c.invite}</h2><span {...stylex.props(sx.sub)}>{c.inviteBody}</span></div><button type="button" {...stylex.props(sx.button, sx.primary)} onClick={onPick}>{c.pick}</button></section>}
    <section aria-label={c.usage} {...stylex.props(sx.card, mobile && sx.mobileCard)}><div {...stylex.props(sx.row)}><h2 {...stylex.props(sx.heading, sx.grow)}>{c.usage}</h2><span {...stylex.props(sx.small, sx.num)}>{num(view.total?.count ?? 0)} tệp · {bytes(view.total?.bytes ?? 0)}</span></div><div>{kinds.map(([kind, icon]) => { const usage = view.usage?.[kind] ?? { count: 0, bytes: 0 }; return <div key={kind} {...stylex.props(sx.usageRow, mobile && sx.mobileUsage)}><Icon name={icon} size={16} {...stylex.props(sx.muted)} /><span>{c.kinds[kind]}</span>{!mobile && <span {...stylex.props(sx.bar)}><span {...stylex.props(sx.barFill(percent(usage.bytes, maximum)), sx.usageFill)} /></span>}<span {...stylex.props(sx.num, sx.right, sx.muted)}>{num(usage.count)} tệp</span><span {...stylex.props(sx.num, sx.right)}>{bytes(usage.bytes)}</span></div> })}</div></section>
    {!mobile && <>{isS3 && view.cache && <section aria-label={c.cache} {...stylex.props(sx.card)}><div {...stylex.props(sx.row)}><h2 {...stylex.props(sx.heading, sx.grow)}>{c.cache}</h2><span {...stylex.props(sx.num, sx.small)}>{bytes(view.cache.usedBytes)} / {bytes(view.cache.maxBytes)}</span></div><span {...stylex.props(sx.bar)}><span {...stylex.props(sx.barFill(percent(view.cache.usedBytes, view.cache.maxBytes)))} /></span><span {...stylex.props(sx.note)}>{c.cacheHint}</span></section>}
      <section aria-label={c.operations} {...stylex.props(sx.card)}><h2 {...stylex.props(sx.heading)}>{c.operations}</h2><span {...stylex.props(sx.note)}>{c.opsHint}</span><div {...stylex.props(sx.table)}><div {...stylex.props(sx.opRow, sx.muted)}>{c.opColumns.map((label) => <span key={label}>{label}</span>)}</div>{ops.map((op) => <div key={op.op} {...stylex.props(sx.opRow, op.errors > 0 && sx.warning)}><span>{c.opNames[op.op]}</span><span {...stylex.props(sx.num, sx.right)}>{num(op.count)}</span><span {...stylex.props(sx.num, sx.right)}>{num(op.errors)}</span><span {...stylex.props(sx.num, sx.right)}>{bytes(op.bytes)}</span><span {...stylex.props(sx.num, sx.right)}>{Math.round(op.count ? op.seconds * 1000 / op.count : 0)} ms</span><span {...stylex.props(sx.num, sx.right)}>{Math.round(op.max * 1000)} ms</span></div>)}</div></section>
      {!view.locked && view.migration?.state === 'completed' && !view.migration.purgedAt && <section aria-label={c.old} {...stylex.props(sx.card)}><div {...stylex.props(sx.row)}><Icon name="box" size={17} /><h2 {...stylex.props(sx.heading, sx.grow)}>{c.old} vẫn giữ {bytes(view.migration.totalBytes)}</h2><span {...stylex.props(sx.note)}>{num(view.migration.totalCount)} tệp{view.migration.completedAt ? ` · chuyển xong lúc ${time(view.migration.completedAt)}` : ''}</span></div><span {...stylex.props(sx.sub)}>{c.oldBody}</span><AdminDetails><CopyCode text={`ainovel-cli storage migration purge-source ${view.migration.id} --dry-run=true\nainovel-cli storage migration purge-source ${view.migration.id} --dry-run=false`} /></AdminDetails></section>}
    </>}
    {mobile && <p {...stylex.props(sx.note)}>{c.mobileHint}</p>}
  </>
}
