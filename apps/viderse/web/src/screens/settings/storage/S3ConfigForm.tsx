import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, RadioCard, Switch, toast } from '@noverse/ui'
import type { S3Config, ServerStorage } from '../../../api/storageServer'
import { serverStorageCopy as t } from '../../../i18n/storageServer'
import { styles as s } from './serverStorage.styles'

const regions = {
  aws: [['ap-southeast-1', 'Singapore'], ['ap-northeast-1', 'Tokyo'], ['us-east-1', 'Bắc Virginia'], ['eu-central-1', 'Frankfurt']],
  wasabi: [['ap-southeast-1', 'Singapore'], ['us-east-1', 'Bắc Virginia'], ['eu-central-1', 'Amsterdam']],
  b2: [['us-west-004', 'Mỹ, miền Tây'], ['us-east-005', 'Mỹ, miền Đông'], ['eu-central-003', 'Châu Âu']],
  do: [['sgp1', 'Singapore'], ['nyc3', 'New York'], ['fra1', 'Frankfurt']],
}
export function S3ConfigForm({ config, storage, onChange, busy }: { config: S3Config; storage: ServerStorage; onChange: (config: S3Config) => void; busy: boolean }) {
  const [replaceKeys, setReplaceKeys] = useState(false)
  const saved = !replaceKeys && storage.provider === config.provider && !!storage.accessKeyLast4
  const update = (patch: Partial<S3Config>) => onChange({ ...config, ...patch })
  const internal = config.provider === 'garage' || config.provider === 'other'
  const regionOptions = config.provider in regions ? regions[config.provider as keyof typeof regions] : undefined
  const field = (name: 'endpoint' | 'accountId' | 'region' | 'bucket' | 'accessKeyId' | 'secretAccessKey' | 'presignEndpoint', label: string, hint: string, full = false) => <label key={name} {...stylex.props(s.column, full && s.full)}>
    <span {...stylex.props(s.label)}>{label}</span>
    {name === 'region' && regionOptions ? <select {...stylex.props(s.field)} name={name} value={config.region ?? ''} disabled={busy} onChange={e => update({ region: e.target.value })}>
      {config.region && !regionOptions.some(([id]) => id === config.region) ? <option value={config.region}>{config.region}</option> : null}
      {regionOptions.map(([id, title]) => <option key={id} value={id}>{title} · {id}</option>)}
    </select> : <input {...stylex.props(s.field)} name={name} value={config[name] ?? ''} type={name === 'secretAccessKey' ? 'password' : 'text'} autoComplete={name === 'secretAccessKey' ? 'new-password' : 'off'} spellCheck={false} disabled={busy} onInput={e => update({ [name]: e.currentTarget.value })} />}
    <span {...stylex.props(s.hint)}>{hint}</span>
  </label>
  return <>
    <section {...stylex.props(s.card)} aria-label={t.info}>
      <div {...stylex.props(s.row)}><h2 {...stylex.props(s.heading, s.grow)}>{t.info}</h2><details {...stylex.props(s.detail)}><summary>{t.guide}</summary><b>{t.guideTitle}</b><ol>{(config.provider === 'garage' ? t.guideGarage : t.guideOther).map(line => <li key={line}>{line}</li>)}</ol>
        {config.provider === 'garage' ? <><pre {...stylex.props(s.code)}>{t.garageCommands}</pre><Button variant="ghost" size="compact" onClick={() => { void navigator.clipboard.writeText(t.garageCommands).then(() => toast.ok(t.copied)).catch(() => toast.err(t.genericError)) }}>{t.copy}</Button></> : null}
      </details></div>
      <div {...stylex.props(s.grid)}>
        {internal ? field('endpoint', config.provider === 'garage' ? t.fields.internalEndpoint : t.fields.endpoint, t.hints.endpoint, true) : null}
        {config.provider === 'r2' ? field('accountId', t.fields.accountId, t.hints.accountId, true) : null}
        {config.provider !== 'r2' && config.provider !== 'gcs' ? field('region', t.fields.region, config.provider === 'garage' ? t.hints.garageRegion : t.hints.region) : null}
        {field('bucket', t.fields.bucket, t.hints.bucket, config.provider === 'r2' || config.provider === 'gcs')}
        {saved ? <>
          <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{t.fields.accessKeyId}</span><div {...stylex.props(s.row)}><span>••••{storage.accessKeyLast4}</span><span {...stylex.props(s.hint)}>{t.saved}</span><Button variant="quiet" size="compact" onClick={() => { setReplaceKeys(true); update({ accessKeyId: '', secretAccessKey: '' }) }}>{t.changeKeys}</Button></div><span {...stylex.props(s.hint)}>{t.savedHint}</span></div>
          <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{t.fields.secretAccessKey}</span><span>••••••••</span><span {...stylex.props(s.hint)}>{t.savedSecret}</span></div>
        </> : <>{field('accessKeyId', t.fields.accessKeyId, t.hints.accessKeyId)}{field('secretAccessKey', t.fields.secretAccessKey, t.hints.secretAccessKey)}</>}
      </div>
      {config.provider === 'other' ? <div {...stylex.props(s.row, s.separator)}><span {...stylex.props(s.grow, s.column)}><span>{t.pathStyle}</span><span {...stylex.props(s.hint)}>{t.pathHint}</span></span><Switch checked={!!config.forcePathStyle} disabled={busy} label={t.pathStyle} onChange={value => update({ forcePathStyle: value })} /></div> : null}
      {internal ? <div {...stylex.props(s.row, s.separator)}><span {...stylex.props(s.grow, s.column)}><span>{t.http}</span><span {...stylex.props(s.hint)}>{t.httpHint}</span>{config.allowInsecure ? <span {...stylex.props(s.hint, s.warning)}>{t.httpWarning}</span> : null}</span><Switch checked={!!config.allowInsecure} disabled={busy} label={t.http} onChange={value => update({ allowInsecure: value })} /></div> : null}
      {config.provider === 'garage' ? <span {...stylex.props(s.hint)}>{t.separate}</span> : null}
    </section>
    <section {...stylex.props(s.card)} aria-label={t.deliveryTitle}>
      <h2 {...stylex.props(s.heading)}>{t.deliveryTitle}</h2><span {...stylex.props(s.hint)}>{t.deliveryHint}</span>
      <div {...stylex.props(s.grid)} role="radiogroup" aria-label={t.fields.delivery}>
        {(['redirect', 'proxy'] as const).map(delivery => <RadioCard key={delivery} checked={config.delivery === delivery} disabled={busy} onChange={() => update({ delivery })} label={t[delivery]} description={<span {...stylex.props(s.column)}><span {...stylex.props(s.good)}>+ {delivery === 'proxy' ? t.proxyPlus : t.directPlus}</span><span {...stylex.props(s.warning)}>− {delivery === 'proxy' ? t.proxyMinus : t.directMinus}</span></span>} />)}
      </div>
      {config.delivery === 'redirect' && internal ? <>{field('presignEndpoint', t.fields.presignEndpoint, t.hints.presignEndpoint, true)}{!config.presignEndpoint?.trim() ? <span {...stylex.props(s.hint, s.warning)}>{t.publicWarning}</span> : null}</> : null}
    </section>
  </>
}
