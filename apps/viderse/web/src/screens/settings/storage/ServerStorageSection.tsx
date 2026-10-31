import { styles as settingsStyles } from '../settings.stylex'
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Banner, Button, Dialog, Icon, ProgressBar, useAsyncAction } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { configFromStorage, connectionPassed, saveServerStorage, startStorageMigration, testServerStorage, type ConnectionResult, type MigrationPreview, type S3Config, type S3Provider, type ServerStorageResponse, type UsageGroup } from '../../../api/storageServer'
import { serverStorageCopy as t } from '../../../i18n/storageServer'
import { keys } from '../../../data/keys'
import { useStatus } from '../../../data/status'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { fill, fmtBytes, fmtTime } from '../fmt'
import { StorageProviderPicker } from './StorageProviderPicker'
import { S3ConfigForm } from './S3ConfigForm'
import { ConnectionTest } from './ConnectionTest'
import { MigrationCard } from './MigrationCard'
import { CleanupCard } from './CleanupCard'
import { styles as s } from './serverStorage.styles'
import { StorageGlyph } from './StorageGlyph'

function Usage({ groups }: { groups: UsageGroup[] }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  const files = groups.reduce((sum, group) => sum + (group.files ?? 0), 0)
  const bytes = groups.reduce((sum, group) => sum + (group.bytes ?? 0), 0)
  const partial = groups.some(group => group.files === null || group.bytes === null)
  const icons = { images: 'gallery', editor: 'scissor', library: 'layer', sources: 'video-play', exports: 'export' }
  return <section {...stylex.props(s.card, mobile && s.mobileCard)} aria-label={t.usage}>
    <div {...stylex.props(s.row)}><h2 {...stylex.props(s.heading, s.grow)}>{t.usage}</h2><span {...stylex.props(s.hint)}>{partial ? t.counted : ''}{fill(t.total, { files: files.toLocaleString('vi-VN'), bytes: fmtBytes(bytes) })}</span></div>
    <div role="list">{groups.map(group => <div key={group.group} role="listitem" data-usage-gap={group.bytes === null || group.files === null ? '' : undefined} {...stylex.props(s.usage)}>
      <Icon name={icons[group.group]} size={16} /><span {...stylex.props(s.column)}>{t.groups[group.group]}{group.group === 'library' ? <small {...stylex.props(s.hint)}>{t.libraryHint}</small> : null}</span>
      <div aria-hidden="true" {...stylex.props(s.usageBar, s.usageTrack)}><div {...stylex.props(s.usageFill(group.bytes !== null && bytes > 0 ? group.bytes / bytes * 100 : 0), group.group === 'images' && s.usageImages, group.group === 'editor' && s.usageEditor, group.group === 'sources' && s.usageSources, group.group === 'exports' && s.usageExports)} /></div>
      <span {...stylex.props(s.numeric, s.muted)}>{group.files === null ? '—' : fill(t.files, { n: group.files.toLocaleString('vi-VN') })}</span><span {...stylex.props(s.numeric, group.bytes === null && s.muted)}>{group.bytes === null ? t.unknown : fmtBytes(group.bytes)}</span>
    </div>)}</div>
    {partial ? <span {...stylex.props(s.hint)}>{t.gapHint}</span> : null}
  </section>
}

export function ServerStorageSection({ data }: { data: ServerStorageResponse }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  const status = useStatus()
  const storage = data.role === 'admin' ? data.storage : undefined
  const [view, setView] = useState<'overview' | 'picker' | 'form'>('overview')
  const [config, setConfig] = useState<S3Config>({ provider: 'garage', bucket: '', delivery: 'proxy' })
  const [result, setResult] = useState<ConnectionResult | null>(null)
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState<string | null>(null)
  const [preview, setPreview] = useState<MigrationPreview | null>(null)
  const controller = useRef<AbortController | null>(null)
  const sequence = useRef(0)
  useEffect(() => () => { sequence.current++; controller.current?.abort() }, [])
  const locked = storage && Object.entries(storage.sources).some(([key, value]) => key !== 'cache' && key !== 'delivery' && value === 'env')
  const passed = connectionPassed(result)
  const count = data.usage.reduce((sum, group) => sum + (group.files ?? 0), 0)
  const size = data.usage.reduce((sum, group) => sum + (group.bytes ?? 0), 0)
  const total = fill(t.total, { files: count.toLocaleString('vi-VN'), bytes: fmtBytes(size) })
  const moving = data.role === 'admin' && data.migration && !['done', 'cancelled'].includes(data.migration.state) ? data.migration : null
  const change = (next: S3Config) => { sequence.current++; controller.current?.abort(); setTesting(false); setConfig(next); setResult(null); setTestError(null); setPreview(null) }
  const check = async () => {
    if (!storage || testing) return
    const id = ++sequence.current
    controller.current?.abort()
    const abort = new AbortController(); controller.current = abort
    setTesting(true); setResult(null); setTestError(null)
    try {
      const tested = await testServerStorage(view === 'form' ? config : configFromStorage(storage), abort.signal, progress => { if (sequence.current === id) setResult(progress) })
      if (sequence.current === id) { setResult(tested); invalidate(keys.status()); invalidate(['storage', 'server']) }
    } catch (error) {
      if (!abort.signal.aborted && sequence.current === id) setTestError(error instanceof Error ? error.message : t.genericError)
    } finally { if (sequence.current === id) setTesting(false) }
  }
  const save = useAsyncAction(async () => {
    if (!passed || !result || !storage?.encryptionKey.ok || locked) return
    const response = await saveServerStorage(config, result.testId)
    if (response.needsMigration) setPreview(response.needsMigration)
    else { setView('overview'); setResult(null); invalidate(keys.storage()) }
  })
  const start = useAsyncAction(async () => {
    await startStorageMigration()
    setPreview(null); setView('overview'); setResult(null)
    invalidate(keys.storage()); invalidate(keys.activity())
  })
  const pick = (provider: S3Provider) => {
    if (!storage) return
    change(provider === storage.provider ? configFromStorage(storage) : { provider, bucket: '', region: provider === 'garage' ? 'garage' : provider === 'aws' || provider === 'wasabi' ? 'ap-southeast-1' : provider === 'b2' ? 'us-west-004' : provider === 'do' ? 'sgp1' : 'us-east-1', delivery: provider === 'garage' ? 'proxy' : 'redirect', forcePathStyle: provider === 'garage', allowInsecure: provider === 'garage' })
    setView('form')
  }
  const validKeys = !!(config.accessKeyId?.trim() && config.secretAccessKey?.trim()) || (!config.accessKeyId && !config.secretAccessKey && config.provider === storage?.provider && !!storage.accessKeyLast4)
  const complete = !!config.bucket.trim() && validKeys && (!(config.provider === 'garage' || config.provider === 'other') || !!(config.endpoint?.trim() && config.region?.trim())) && (config.provider !== 'r2' || !!config.accountId?.trim())
  const provider = storage?.provider ? t.providers[storage.provider].name : t.disk
  const isDown = storage ? !storage.online : !!status.data?.storageDown
  const kv: [string, string][] = storage?.kind === 's3' ? [
    [t.fields.provider, storage.provider === 'garage' ? t.garageProvider : provider],
    [t.fields.bucket, storage.bucket ?? '—'],
    [t.fields.address, storage.endpointHost ?? '—'],
    [t.fields.region, storage.region ?? '—'],
    [t.fields.delivery, t[storage.delivery]],
    [t.fields.pathStyle, storage.forcePathStyle ? t.pathValue : '—'],
    [t.keys, storage.accessKeyLast4 ? `••••${storage.accessKeyLast4}` : '—'],
    [t.fields.config, locked ? t.envLabel : t.savedConfig],
  ] : [[t.fields.free, fmtBytes(storage?.disk?.freeBytes ?? 0)], [t.fields.delivery, t.proxy]]
  const summary = mobile && storage?.kind === 's3' ? [[t.fields.bucket, storage.bucket ?? '—'], [t.fields.region, storage.region ?? '—'], [t.fields.delivery, t[storage.delivery]], [t.fields.config, locked ? t.envLabel : t.appSource]] : kv
  return <div {...stylex.props(s.stack)}>
    {!mobile ? <div>
      {storage && view !== 'overview' ? <Button variant="quiet" size="compact" disabled={save.pending} onClick={() => { change(config); setView(view === 'form' ? 'picker' : 'overview') }}>{view === 'form' ? t.backPick : t.back}</Button> : null}
      <h1 className={`vv-set__title ${stylex.props(settingsStyles.setTitle).className}`}>{view === 'picker' ? t.pick : view === 'form' ? fill(t.formTitle, { provider: t.providers[config.provider].name }) : t.title}</h1>
      <p className={`vv-set__lede ${stylex.props(settingsStyles.setLede).className}`}>{data.role === 'member' ? t.memberLede : view === 'picker' ? t.pickerLede : view === 'form' ? fill(t.formLede, { hint: t.providers[config.provider].hint }) : t.lede}</p>
    </div> : <span {...stylex.props(s.hint)}>{data.role === 'admin' ? t.mobileLede : t.memberLede}</span>}
    {!storage && isDown ? <Banner tone="info" title="">{t.memberDown}</Banner> : null}
    {storage && view === 'overview' || mobile && storage ? <>
      {isDown ? <Banner tone="error" align="top" title={fill(t.downTitle, { label: provider })} actions={[{ label: t.recheck, variant: 'primary', onClick: () => void check() }]}><div {...stylex.props(s.column)}><span>{t.downBody}</span>{status.data?.running ? <span>{fill(t.held, { n: status.data.running })}</span> : null}<span>{fill(t.retryMeta, { time: fmtTime(storage.checkedAt) })}</span></div></Banner> : null}
      {moving ? <MigrationCard migration={moving} mobile={mobile} /> : null}
      {!mobile && data.migration?.state === 'done' ? <Banner tone="success" title={fill(t.migratedTitle, { provider })}>{fill(t.migratedBody, { provider, files: data.migration.checked })}</Banner> : null}
      {!mobile && data.migration?.state === 'cancelled' ? <Banner tone="info" title={t.cancelledTitle}>{fill(t.cancelBody, { from: data.migration.from.label, files: data.migration.files.done, to: data.migration.to.label })}</Banner> : null}
      <section {...stylex.props(s.card, mobile && s.mobileCard)} aria-label={provider}>
        <div {...stylex.props(s.row)}><span {...stylex.props(s.logo, !storage.provider && s.storageLogo)}>{storage.provider ? t.providers[storage.provider].initials : <StorageGlyph kind="drive" />}</span><span {...stylex.props(s.grow, s.column)}><span {...stylex.props(s.row, mobile && s.mobileHeading)}><h2 {...stylex.props(s.heading)}>{provider}</h2><span {...stylex.props(s.badge, isDown && s.badBadge)}>{isDown ? t.down : t.good}</span></span>{!mobile ? <span {...stylex.props(s.hint)}>{storage.kind === 's3' ? `${t.current} · ${fill(t.checked, { time: fmtTime(storage.checkedAt) })}` : t.diskSub}</span> : null}{locked && !mobile ? <span {...stylex.props(s.hint)}>{t.envLabel}</span> : null}</span>
          {storage.kind === 's3' && !isDown && !mobile ? <Button variant="ghost" size="compact" pending={testing} onClick={() => void check()}>{t.check}</Button> : null}
          {!locked && !moving && !mobile && storage.kind === 's3' ? <Button variant="ghost" size="compact" onClick={() => { setView('picker'); setResult(null) }}>{t.change}</Button> : null}
        </div>
        {testing ? <span role="status">{t.checking}</span> : result && passed ? <span role="status" {...stylex.props(s.good)}>{fill(t.passed, { seconds: ((result.steps.reduce((sum, step) => sum + (step.ms ?? 0), 0)) / 1000).toLocaleString('vi-VN') })}</span> : result?.error ? <Banner tone="error" title={t.errors[result.error.code][0]}>{t.errors[result.error.code][2]}</Banner> : null}
        <div {...stylex.props(s.grid, s.kvGrid, mobile && s.mobileKvList)}>{summary.map(([key, value]) => <div key={key} {...stylex.props(s.kv, mobile && s.mobileKv)}><span {...stylex.props(s.kvKey, mobile && s.mobileKvKey)}>{key}</span><span {...stylex.props(s.ellipsis, mobile && s.mobileValue)} title={value}>{value}</span></div>)}</div>
        {mobile && storage.kind === 's3' && !isDown && !moving ? <Button {...stylex.props(s.mobileButton)} variant="ghost" size="lg" pending={testing} icon={<Icon name="refresh" size={16} />} onClick={() => void check()}>{t.check}</Button> : null}
        {locked ? <Banner tone="info" title="">{t.envBody}</Banner> : null}
        {!mobile && (locked || storage.disk) ? <details {...stylex.props(s.detail)}><summary>{t.details}</summary><pre {...stylex.props(s.code)}>{storage.disk?.path}{locked ? `\nSTORAGE_DRIVER=${storage.kind === 'disk' ? 'local' : 's3'}\nS3_PROVIDER=${storage.provider ?? ''}\nS3_ENDPOINT=${storage.endpointHost ? `${storage.allowInsecure ? 'http' : 'https'}://${storage.endpointHost}` : ''}\nS3_REGION=${storage.region ?? ''}\nS3_BUCKET=${storage.bucket ?? ''}\nBLOB_DELIVERY=${storage.delivery}\nS3_PRESIGN_ENDPOINT=${storage.presignEndpoint ?? ''}\nS3_FORCE_PATH_STYLE=${storage.forcePathStyle ?? false}\nS3_ALLOW_INSECURE_ENDPOINT=${storage.allowInsecure ?? false}` : ''}</pre></details> : null}
      </section>
      {storage.kind === 'disk' && !locked && !moving && !mobile ? <section {...stylex.props(s.card)}><div {...stylex.props(s.row)}><StorageGlyph kind="cloud" /><span {...stylex.props(s.grow, s.column)}><h2 {...stylex.props(s.heading)}>{t.invite}</h2><span {...stylex.props(s.hint)}>{t.inviteBody}</span></span><Button variant="primary" size="compact" onClick={() => setView('picker')}>{t.pick}</Button></div></section> : null}
    </> : null}
    {storage && !mobile && view === 'picker' ? <StorageProviderPicker current={storage.provider} onPick={pick} /> : null}
    {storage && !mobile && view === 'form' ? <>
      {!storage.encryptionKey.ok ? <div role="status" {...stylex.props(s.keyBanner)}><StorageGlyph kind="lock" /><div {...stylex.props(s.column, s.grow)}><b>{t.noKey}</b><span {...stylex.props(s.hint)}>{t.noKeyBody}</span><details {...stylex.props(s.detail)}><summary>{t.details}</summary><pre {...stylex.props(s.code)}>{t.keyFile}{'\n'}{storage.encryptionKey.path}{'\n'}{storage.encryptionKey.issue ? fill(t.keyIssues[storage.encryptionKey.issue], { directory: storage.encryptionKey.path.replace(/\/[^/]*$/, '') }) : ''}</pre></details></div></div> : null}
      <S3ConfigForm key={config.provider} config={config} storage={storage} onChange={change} busy={testing || save.pending} />
      <ConnectionTest config={config} result={result} busy={testing} disabled={!complete} onRun={() => void check()} onProxy={() => change({ ...config, delivery: 'proxy' })} />
      <div {...stylex.props(s.save)} role="region" aria-label={t.save}><span {...stylex.props(s.grow)}>{!storage.encryptionKey.ok ? t.noKey : testing ? t.checking : passed ? fill(t.saveReady, { total, provider: t.providers[config.provider].name }) : result?.error ? t.saveFailed : t.saveIdle}</span><Button variant="quiet" size="compact" disabled={save.pending} onClick={() => { change(config); setView('overview') }}>{t.cancel}</Button><Button variant={passed && storage.encryptionKey.ok ? 'primary' : 'ghost'} size="compact" disabled={!passed || !complete || !storage.encryptionKey.ok || !!locked} pending={save.pending} onClick={() => void save.run()}>{t.save}</Button></div>
    </> : null}
    {testError ? <Banner tone="error" title={testError} /> : null}
    {view === 'overview' || mobile || data.role === 'member' ? <>
      <Usage groups={data.usage} />
      {storage?.kind === 's3' && !mobile && data.workCache ? <section {...stylex.props(s.card)}><div {...stylex.props(s.row)}><h2 {...stylex.props(s.heading, s.grow)}>{t.cache}</h2><span {...stylex.props(s.numeric)}>{fmtBytes(data.workCache.usedBytes)} / {fmtBytes(data.workCache.limitBytes)}</span></div><ProgressBar value={data.workCache.limitBytes > 0 ? Math.min(100, data.workCache.usedBytes / data.workCache.limitBytes * 100) : 0} height={6} tone="info" /><span {...stylex.props(s.hint)}>{t.cacheHint}</span></section> : null}
      {storage?.kind === 's3' && !mobile && data.ops ? <section {...stylex.props(s.card)}><h2 {...stylex.props(s.heading)}>{t.ops}</h2><span {...stylex.props(s.hint)}>{t.opsHint} {storage.delivery === 'proxy' ? t.proxyOpsHint : ''}</span><div {...stylex.props(s.overflow)}><table {...stylex.props(s.table)}><thead><tr>{t.opHeaders.map(text => <th key={text} {...stylex.props(s.cell)}>{text}</th>)}</tr></thead><tbody>{data.ops.filter(op => storage.delivery !== 'proxy' || op.op !== 'presign').map(op => <tr key={op.op} {...stylex.props(op.errors > 0 && s.error)}>{[t.opLabels[op.op], op.count, op.errors, op.bytes ? fmtBytes(op.bytes) : '—', `${Math.round(op.avgMs)} ms`, `${Math.round(op.maxMs)} ms`].map((value, index) => <td key={index} {...stylex.props(s.cell, index === 0 && s.left)}>{value}</td>)}</tr>)}</tbody></table></div></section> : null}
      {storage?.previous && !mobile ? <section {...stylex.props(s.card)}><div {...stylex.props(s.row)}><StorageGlyph kind="drive" size={16} /><h2 {...stylex.props(s.heading, s.grow)}>{fill(t.previous, { label: storage.previous.label, bytes: fmtBytes(storage.previous.bytes) })}</h2><span {...stylex.props(s.hint)}>{fill(t.files, { n: storage.previous.files })}</span></div><span {...stylex.props(s.hint)}>{t.previousHint}</span><details {...stylex.props(s.detail)}><summary>{t.details}</summary><pre {...stylex.props(s.code)}>{storage.previous.path}{'\n'}{t.previousDetails}</pre></details></section> : null}
      {!mobile ? <CleanupCard serverRole={data.role} /> : storage ? <span {...stylex.props(s.hint)}>{storage.kind === 's3' ? t.mobileS3 : t.mobileDisk}</span> : null}
    </> : null}
    {preview && storage && !mobile ? <Dialog alertDialog title={fill(t.confirm, { total: fill(t.total, { files: preview.files, bytes: fmtBytes(preview.bytes) }), provider: t.providers[config.provider].name })} onClose={() => { if (!start.pending) setPreview(null) }} width={540}>
      <div {...stylex.props(s.stack)}><span {...stylex.props(s.hint)}>{fill(t.confirmSub, { from: provider })}</span><div>{preview.groups.map(group => <div key={group.group} {...stylex.props(s.kv)}><span {...stylex.props(s.grow)}>{t.groups[group.group]}</span><span>{group.files === null ? '—' : fill(t.files, { n: group.files })}</span><span>{group.bytes === null ? t.unknown : fmtBytes(group.bytes)}</span></div>)}</div>{t.promises.map(line => <span key={line} {...stylex.props(s.hint)}>{line}</span>)}<div {...stylex.props(s.row)}><Button variant="quiet" disabled={start.pending} onClick={() => setPreview(null)}>{t.cancel}</Button><Button variant="primary" pending={start.pending} onClick={() => void start.run()}>{t.start}</Button></div></div>
    </Dialog> : null}
  </div>
}
