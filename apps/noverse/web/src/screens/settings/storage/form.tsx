import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, Spinner, Switch, toast } from '@noverse/ui'
import { ApiError, testStorage } from '../../../api/client'
import type { StorageCandidate, StorageStep, StorageTest, StorageView } from '../../../api/storage-settings'
import { storageCopy as c, storageGuides, storageProviders, storageRegions } from '../../../i18n/vi'
import { completeBrowserTest, corsSnippet, displaySteps, endpointOf, privateEndpoint } from './logic'
import { sx } from './styles'

export const providerOf = (id?: string) => storageProviders.find((item) => item.id === id) ?? storageProviders[7]
export function candidateFor(provider: string, view: StorageView): StorageCandidate {
  const same = provider === view.active?.provider
  const active = same ? view.active : undefined
  return { version: view.version ?? 0, driver: 's3', provider, endpoint: active ? endpointOf(active) : provider === 'garage' ? 'http://garage:3900' : '', region: active?.region || (provider === 'garage' ? 'garage' : storageRegions[provider]?.[0][0] ?? 'us-east-1'), bucket: active?.bucket ?? '', presignEndpoint: active?.presignEndpoint ?? '', pathStyle: active?.pathStyle ?? provider === 'garage', allowHTTP: active?.allowHTTP ?? provider === 'garage', delivery: active?.delivery ?? (provider === 'garage' ? 'proxy' : 'redirect'), ...(active ? {} : { accessKeyId: '', secretAccessKey: '' }) }
}
export function providerEndpoint(candidate: StorageCandidate, account: string): string {
  switch (candidate.provider) {
    case 'r2': return account ? `https://${account}.r2.cloudflarestorage.com` : ''
    case 'aws': return `https://s3.${candidate.region}.amazonaws.com`
    case 'wasabi': return `https://s3.${candidate.region}.wasabisys.com`
    case 'b2': return `https://s3.${candidate.region}.backblazeb2.com`
    case 'do': return `https://${candidate.region}.digitaloceanspaces.com`
    case 'gcs': return 'https://storage.googleapis.com'
    default: return candidate.endpoint ?? ''
  }
}

export function CopyCode({ text }: { text: string }) {
  return <div {...stylex.props(sx.stack)}><pre {...stylex.props(sx.code)}>{text}</pre><button type="button" {...stylex.props(sx.button)} onClick={() => { void navigator.clipboard.writeText(text).then(() => toast(c.copied), () => toast(c.copyFailed)) }}><Icon name="copy" size={13} />{c.copy}</button></div>
}
export function AdminDetails({ children }: { children: React.ReactNode }) {
  return <details><summary {...stylex.props(sx.detail)}>{c.details}</summary><div {...stylex.props(sx.stack)}>{children}</div></details>
}

export function StorageForm({ view, draft, onDraft, result, onResult, onSave, onBack, saving, onConflict }: {
  view: StorageView; draft: StorageCandidate; onDraft: (value: StorageCandidate) => void; result: StorageTest | null
  onResult: (value: StorageTest | null) => void; onSave: () => void; onBack: () => void; saving: boolean; onConflict: () => void
}) {
  const provider = providerOf(draft.provider)
  const [account, setAccount] = useState(() => /^https:\/\/([^.]+)\.r2\./.exec(draft.endpoint ?? '')?.[1] ?? '')
  const [guide, setGuide] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)
  const [progress, setProgress] = useState<StorageStep[]>([])
  const [browserPhase, setBrowserPhase] = useState(false)
  const [failure, setFailure] = useState('')
  const [corsTab, setCorsTab] = useState<'r2' | 'aws' | 'garage'>(draft.provider === 'garage' ? 'garage' : draft.provider === 'aws' ? 'aws' : 'r2')
  const generation = useRef(0)
  const request = useRef<AbortController | null>(null)
  const deliveryChosen = useRef(view.active?.provider === draft.provider)
  useEffect(() => () => { generation.current++; request.current?.abort() }, [])
  const invalidate = () => { generation.current++; request.current?.abort(); onResult(null); setProgress([]); setFailure('') }
  const change = (patch: Partial<StorageCandidate>) => {
    invalidate()
    const next = { ...draft, ...patch }
    if (patch.delivery) deliveryChosen.current = true
    if (patch.endpoint !== undefined && draft.provider === 'other' && !deliveryChosen.current) next.delivery = privateEndpoint(patch.endpoint) ? 'proxy' : 'redirect'
    onDraft({ ...next, endpoint: providerEndpoint(next, account), region: next.provider === 'r2' ? 'auto' : next.region })
  }
  const endpoint = providerEndpoint(draft, account)
  const recommendedProxy = provider.proxy || provider.id === 'other' && privateEndpoint(endpoint)
  const candidate = { ...draft, endpoint, region: draft.provider === 'r2' ? 'auto' : draft.region }
  const publicNeeded = draft.delivery === 'redirect' && privateEndpoint(endpoint)
  const required = [draft.bucket, draft.region, endpoint, ...(draft.accessKeyId !== undefined ? [draft.accessKeyId, draft.secretAccessKey] : []), ...(draft.provider === 'r2' ? [account] : [])]
  const missing = required.filter((value) => !value?.trim()).length
  const run = async () => {
    if (testing || missing) return
    const id = generation.current
    setTesting(true); setProgress([]); setBrowserPhase(false); setFailure(''); onResult(null)
    request.current?.abort()
    const controller = new AbortController(); request.current = controller
    onDraft(candidate)
    try {
      const server = await testStorage(candidate, controller.signal, (step) => {
        if (id === generation.current) setProgress((previous) => [...previous.filter((item) => item.step !== step.step), step])
      })
      if (id !== generation.current) return
      setProgress(server.steps); setBrowserPhase(candidate.delivery === 'redirect' && !!server.corsProbeUrl)
      const test = await completeBrowserTest(server, candidate.delivery, controller.signal)
      if (id === generation.current) onResult(test)
    } catch (error) {
      if (id === generation.current) setFailure(error instanceof Error ? error.message : c.actionError)
      if (!controller.signal.aborted && error instanceof ApiError && error.status === 409) onConflict()
    } finally { setTesting(false) }
  }
  const fields: { key: keyof StorageCandidate | 'account'; label: string; hint: string; full?: boolean }[] = [
    ...(['garage', 'other'].includes(draft.provider) ? [{ key: 'endpoint' as const, label: draft.provider === 'garage' ? c.internalEndpoint : c.endpoint, hint: c.endpointHint, full: true }] : []),
    ...(['garage', 'other'].includes(draft.provider) ? [{ key: 'region' as const, label: c.region, hint: c.regionHint }] : []),
    { key: 'bucket', label: c.bucket, hint: c.bucketHint },
    ...(draft.provider === 'r2' ? [{ key: 'account' as const, label: c.account, hint: c.accountHint }] : []),
    ...(!['garage', 'other', 'r2', 'gcs'].includes(draft.provider) ? [{ key: 'region' as const, label: c.region, hint: c.regionHint }] : []),
    { key: 'accessKeyId', label: c.accessKeyId, hint: c.keyHint }, { key: 'secretAccessKey', label: c.secretAccessKey, hint: c.secretHint },
  ]
  const failedStep = result?.steps.find((step) => !step.ok && !['skipped', 'not_applicable'].includes(step.code))
  const failedCode = failedStep?.code ?? ''
  const browserReach = ['browser_endpoint_required', 'browser_unreachable'].includes(failedCode)
  const guides = storageGuides[provider.id] ?? (['wasabi', 'b2', 'do', 'gcs'].includes(provider.id) ? storageGuides.aws : storageGuides.other)
  const guideKey = guide === 'secretAccessKey' && !guides.some(([key]) => key === guide) ? 'accessKeyId' : guide
  const steps: StorageStep[] = displaySteps(result ?? (progress.length ? { ok: false, steps: progress } : null), draft.delivery, result !== null)
  const previousStep = progress.at(-1)?.step ?? ''
  const liveStep = browserPhase ? 'cors_probe' : ({ '': 'write', write: 'read', read: 'range', range: 'presign', clock: 'presign', presign: draft.delivery === 'proxy' ? 'app_range' : 'delete', app_range: 'delete' } as Record<string, string>)[previousStep]
  const totalMS = result?.steps.reduce((total, step) => total + step.ms, 0) ?? 0
  const canSave = !missing && result?.ok === true && view.encryption.available && !view.locked && !testing && !saving
  return <>
    <button type="button" {...stylex.props(sx.button, sx.ghost)} onClick={onBack}><Icon name="arrow-left-2" size={15} />{c.other}</button>
    <div {...stylex.props(sx.row)}><span {...stylex.props(sx.logo)}>{provider.initials}</span><div {...stylex.props(sx.stack)}><h1 {...stylex.props(sx.title)}>Kết nối {provider.name}</h1><span {...stylex.props(sx.sub)}>{provider.hint}</span></div></div>
    {!view.encryption.available && <section role="alert" {...stylex.props(sx.card, sx.warning)}><b>{c.noKey}</b><span {...stylex.props(sx.small)}>{c.noKeyBody}</span><AdminDetails><CopyCode text={'# 32 byte ngẫu nhiên, mã base64 — giữ kín, mất là không mở lại được khoá đã lưu\nopenssl rand -base64 32\n# đặt vào biến CONFIG_ENCRYPTION_KEY của máy chủ rồi khởi động lại Noverse'} /></AdminDetails></section>}
    <section aria-label={c.info} {...stylex.props(sx.card)}>
      <div {...stylex.props(sx.row)}><h2 {...stylex.props(sx.heading, sx.grow)}>{c.info}</h2><span {...stylex.props(sx.note)}>{c.helpHint}</span></div>
      {guide && <div {...stylex.props(sx.inset, sx.stack)} role="region" aria-label={`Lấy thông tin ở ${provider.name}`}><div {...stylex.props(sx.row)}><b {...stylex.props(sx.grow)}>Lấy thông tin ở {provider.name}</b><button type="button" {...stylex.props(sx.button, sx.ghost)} onClick={() => setGuide(null)}>{c.close}</button></div>{guides.map(([key, text], index) => <div key={key} {...stylex.props(sx.inset, sx.small, key === guideKey && sx.selected)}>{index + 1}. {text}</div>)}</div>}
      <div {...stylex.props(sx.grid)}>{fields.map((field) => {
        const saved = (field.key === 'accessKeyId' || field.key === 'secretAccessKey') && draft.accessKeyId === undefined
        return <div key={field.key} {...stylex.props(sx.field, field.full && sx.full)}><div {...stylex.props(sx.row)}><label htmlFor={`storage-${field.key}`} {...stylex.props(sx.label, sx.grow)}>{field.label}</label><button type="button" {...stylex.props(sx.help)} aria-expanded={guide === field.key} onClick={() => setGuide(guide === field.key ? null : field.key)}>{c.where}</button></div>
          {saved ? <div {...stylex.props(sx.input, sx.row)}><span>{field.key === 'accessKeyId' ? `••••${view.active?.accessKeyLast4}` : '••••••••'}</span><span {...stylex.props(sx.note, sx.grow)}>{c.saved}{field.key === 'secretAccessKey' ? ` · ${c.encrypted}` : ''}</span>{field.key === 'accessKeyId' && <button type="button" {...stylex.props(sx.help)} onClick={() => change({ accessKeyId: '', secretAccessKey: '' })}>{c.changeKey}</button>}</div>
            : field.key === 'region' && storageRegions[draft.provider] ? <select id={`storage-${field.key}`} {...stylex.props(sx.input)} value={draft.region} onChange={(event) => change({ region: event.target.value })}>{!storageRegions[draft.provider].some(([value]) => value === draft.region) && <option value={draft.region}>{draft.region}</option>}{storageRegions[draft.provider].map(([value, label]) => <option key={value} value={value}>{label} · {value}</option>)}</select>
              : <input id={`storage-${field.key}`} {...stylex.props(sx.input)} type={field.key === 'secretAccessKey' ? 'password' : 'text'} autoComplete={field.key === 'secretAccessKey' ? 'new-password' : 'off'} spellCheck={false} value={field.key === 'account' ? account : String(draft[field.key] ?? '')} onChange={(event) => { const value = event.target.value; if (field.key === 'account') { setAccount(value); invalidate(); onDraft({ ...draft, endpoint: providerEndpoint(draft, value), region: 'auto' }) } else change({ [field.key]: value }) }} />}
          <span {...stylex.props(sx.note)}>{field.key === 'account' && account ? endpoint : field.hint}</span></div>
      })}</div>
      {['garage', 'other'].includes(draft.provider) && <>{draft.provider === 'other' && <div {...stylex.props(sx.row)}><span {...stylex.props(sx.grow, sx.small)}>{c.pathStyle}</span><Switch checked={draft.pathStyle} onChange={(pathStyle) => change({ pathStyle })} label={c.pathStyle} /></div>}<div {...stylex.props(sx.row)}><span {...stylex.props(sx.grow, sx.small)}>{c.allowHTTP}</span><Switch checked={draft.allowHTTP} onChange={(allowHTTP) => change({ allowHTTP })} label={c.allowHTTP} /></div>{draft.allowHTTP && <p {...stylex.props(sx.note, sx.warnText)}>{c.httpWarning}</p>}</>}
      {draft.provider === 'garage' && <details><summary {...stylex.props(sx.detail)}>{c.garageCommands}</summary><p {...stylex.props(sx.note)}>{c.garageHint}</p><CopyCode text={`garage bucket create '${(draft.bucket || 'noverse-media').replaceAll("'", "'\\''")}'\ngarage key create noverse-app\ngarage bucket allow --read --write --owner '${(draft.bucket || 'noverse-media').replaceAll("'", "'\\''")}' --key noverse-app`} /></details>}
    </section>
    <section aria-label={c.delivery} {...stylex.props(sx.card)}><h2 {...stylex.props(sx.heading)}>{c.delivery}</h2><span {...stylex.props(sx.note)}>{c.deliveryHint}</span><div role="radiogroup" aria-label={c.delivery} {...stylex.props(sx.grid)}>{(['redirect', 'proxy'] as const).map((delivery) => <button type="button" role="radio" aria-checked={draft.delivery === delivery} key={delivery} {...stylex.props(sx.inset, sx.stack, draft.delivery === delivery && sx.selected)} onClick={() => change({ delivery })}><span {...stylex.props(sx.row, sx.small)}>{delivery === 'proxy' ? c.proxy : c.direct}{recommendedProxy === (delivery === 'proxy') && <span {...stylex.props(sx.chip, sx.accentChip)}>{c.recommend}</span>}</span><span {...stylex.props(sx.note, sx.okText)}>{delivery === 'proxy' ? c.proxyPro : c.directPro}</span><span {...stylex.props(sx.note, sx.warnText)}>{delivery === 'proxy' ? c.proxyCon : c.directCon}</span></button>)}</div>
      {publicNeeded && <div {...stylex.props(sx.field)}><label htmlFor="storage-public" {...stylex.props(sx.label)}>{c.publicEndpoint}</label><input id="storage-public" {...stylex.props(sx.input)} value={draft.presignEndpoint ?? ''} placeholder="https://s3.aarverse.studio" onChange={(event) => change({ presignEndpoint: event.target.value })} /><span {...stylex.props(sx.note)}>{c.publicHint}</span>{!draft.presignEndpoint && <p {...stylex.props(sx.inset, sx.warning, sx.small)}>{c.publicWarning}</p>}</div>}
    </section>
    <section aria-label={c.test} {...stylex.props(sx.card)}><div {...stylex.props(sx.row)}><div {...stylex.props(sx.stack, sx.grow)}><h2 {...stylex.props(sx.heading)}>{c.test}</h2><span {...stylex.props(sx.note)}>{c.testHint}</span></div><button type="button" {...stylex.props(sx.button, sx.primary)} disabled={testing || missing > 0} onClick={() => { void run() }}>{testing ? <Spinner size={14} /> : <Icon name="flash" size={14} />}{testing ? c.testing : result ? c.retest : c.test}</button></div>
      {missing > 0 && <span {...stylex.props(sx.note)}>Điền đủ {missing} ô còn trống rồi kiểm tra</span>}
      <div role="list">{steps.map((step) => { const waiting = step.code === 'waiting' || step.code === 'skipped'; const na = step.step === 'presign' && draft.delivery === 'proxy'; return <div role="listitem" key={step.step} {...stylex.props(sx.testRow)}>{testing && step.step === liveStep ? <Spinner size={17} /> : <Icon name={waiting ? 'minus' : step.ok ? na ? 'minus' : 'tick-circle' : 'close-circle-bulk'} size={17} {...stylex.props(waiting || na ? sx.muted : step.ok ? sx.okText : sx.errorText)} />}<div {...stylex.props(sx.stack, sx.grow)}><span>{c.stepNames[step.step] ?? c.test}</span><span {...stylex.props(sx.note)}>{na ? c.unnecessary : c.stepHints[step.step]}</span></div><span {...stylex.props(sx.note, sx.num)}>{testing && step.step === liveStep ? c.testing : waiting ? step.code === 'skipped' ? c.skipped : c.untested : step.ok ? na ? c.unnecessary : `${step.ms} ms` : c.failed}</span></div> })}</div>
      {result?.ok && <div role="status" {...stylex.props(sx.inset, sx.success)}>Đạt cả {steps.length} bước · tổng {totalMS} ms · {draft.delivery === 'proxy' ? c.proxy : 'trình duyệt tải thẳng từ kho'}</div>}
      {failure && <div role="alert" {...stylex.props(sx.inset, sx.error)}>{failure}</div>}
      {failedStep && <div role="alert" {...stylex.props(sx.card, sx.error)}><b>{failedCode === 'no_bucket' ? `Không có kho tên “${draft.bucket}”` : c.failTitles[failedCode] ?? c.failTitles.storage_error}</b><span {...stylex.props(sx.small)}>{failedCode === 'browser_unreachable' ? `Trình duyệt của bạn không mở được ${draft.presignEndpoint || endpoint}` : failedCode === 'browser_cors' ? `Máy chủ ghi và đọc được, nhưng kho chưa cho trang ${location.origin} tải tệp.` : failedStep.reason}</span>{c.fixes[failedCode] && <><span {...stylex.props(sx.note)}>{c.fix}</span><span {...stylex.props(sx.small)}>{c.fixes[failedCode]}</span></>}
        {failedCode === 'browser_cors' && <><p {...stylex.props(sx.small)}>{c.corsIntro}</p><div role="tablist" aria-label={c.fix} {...stylex.props(sx.row)}>{(['r2', 'aws', 'garage'] as const).map((tab) => <button type="button" role="tab" aria-selected={corsTab === tab} key={tab} {...stylex.props(sx.button, corsTab === tab && sx.selected)} onClick={() => setCorsTab(tab)}>{providerOf(tab).name}</button>)}</div><span {...stylex.props(sx.note)}>{c.corsWhere[corsTab]}</span><CopyCode text={corsSnippet(corsTab, location.origin, draft.presignEndpoint || endpoint, draft.bucket || '')} /><span {...stylex.props(sx.note)}>{c.corsWait}</span></>}
        {(browserReach || failedCode === 'browser_cors') && <div {...stylex.props(sx.row, sx.wrap)}><button type="button" {...stylex.props(sx.button)} onClick={() => { if (browserReach) document.getElementById('storage-public')?.focus(); else void run() }}>{browserReach ? c.fillPublic : c.retest}</button><button type="button" {...stylex.props(sx.button)} onClick={() => change({ delivery: 'proxy' })}>{c.toProxy}</button></div>}
      </div>}
    </section>
    <div {...stylex.props(sx.savebar)} role="region" aria-label={c.save}><span>{canSave ? `Kết nối đạt · lưu để dùng ${provider.name}` : c.mustTest}</span><button type="button" {...stylex.props(sx.button, sx.ghost)} disabled={saving} onClick={onBack}>{c.cancel}</button><button type="button" {...stylex.props(sx.button, sx.primary)} disabled={!canSave} onClick={onSave}>{saving ? <Spinner size={14} /> : c.save}</button></div>
  </>
}
