import * as stylex from '@stylexjs/stylex'
import { Banner, Button, Icon, Spinner, toast } from '@noverse/ui'
import { connectionPassed, type ConnectionResult, type S3Config } from '../../../api/storageServer'
import { serverStorageCopy as t } from '../../../i18n/storageServer'
import { fill } from '../fmt'
import { styles as s } from './serverStorage.styles'

export function ConnectionTest({ config, result, busy, disabled, onRun, onProxy }: { config: S3Config; result: ConnectionResult | null; busy: boolean; disabled: boolean; onRun: () => void; onProxy?: () => void }) {
  const passed = connectionPassed(result)
  const error = result?.error
  const errorCopy = error ? t.errors[error.code] ?? t.errors.storage_error : null
  const substitution = { provider: t.providers[config.provider].name, bucket: config.bucket, host: config.presignEndpoint || config.endpoint || '', origin: window.location.origin }
  const ids = ['write', 'read', 'range', 'presign', config.delivery === 'proxy' ? 'app_range' : 'cors_probe', 'delete'] as const
  const corsRules = [{ AllowedOrigins: [window.location.origin], AllowedMethods: ['GET', 'HEAD'], AllowedHeaders: ['range'], ExposeHeaders: ['content-length', 'content-range', 'accept-ranges', 'etag'], MaxAgeSeconds: 3600 }]
  const corsCode = JSON.stringify(config.provider === 'garage' ? { CORSRules: corsRules } : corsRules, null, 2)
  const totalMs = result?.steps.reduce((sum, step) => sum + (step.ms ?? 0), 0) ?? 0
  const canFillPublic = error?.code === 'browser_unreachable' && (config.provider === 'garage' || config.provider === 'other')
  return <section {...stylex.props(s.card)} aria-label={t.check}>
    <div {...stylex.props(s.row)}><span {...stylex.props(s.grow, s.column)}><h2 {...stylex.props(s.heading)}>{t.check}</h2><span {...stylex.props(s.hint)}>{t.testHint}</span></span><Button variant={!passed && error?.code !== 'cors_blocked' && error?.code !== 'browser_unreachable' ? 'primary' : 'ghost'} size="compact" pending={busy} disabled={disabled} onClick={onRun}>{result ? t.recheck : t.check}</Button></div>
    {disabled ? <span {...stylex.props(s.hint)}>{t.missing}</span> : null}
    <div role="list" aria-label={t.stepsLabel}>
      {ids.map(id => {
        const step = result?.steps.find(item => item.id === id)
        const status = step?.status
        return <div key={id} role="listitem" {...stylex.props(s.step, status === 'fail' && s.error)}>
          {status === 'pending' || (busy && !step && id === 'write') ? <Spinner size={13} /> : status === 'ok' || status === 'fail' || status === 'na' ? <span {...stylex.props(status === 'ok' ? s.good : status === 'fail' ? s.error : s.muted)}><Icon name={status === 'ok' ? 'tick-b' : status === 'fail' ? 'danger' : 'minus'} size={17} /></span> : <span {...stylex.props(s.waiting)} aria-hidden="true" />}
          <span {...stylex.props(s.column)}><span>{t.steps[id][0]}</span><span {...stylex.props(s.hint)}>{id === 'presign' && config.delivery === 'proxy' ? t.proxyPresign : t.steps[id][1]}</span></span>
          <span {...stylex.props(s.numeric)}>{status === 'na' ? t.na : status === 'fail' ? t.fail : status === 'skip' ? t.skipped : step?.ms != null ? `${step.ms} ms` : ''}</span>
        </div>
      })}
    </div>
    {passed ? <div data-test-success><Banner tone="success" title={fill(t.passed, { seconds: (totalMs / 1000).toLocaleString('vi-VN', { maximumFractionDigits: 2 }) })} /></div> : null}
    {errorCopy ? <Banner tone="error" title={fill(errorCopy[0], substitution)}><div {...stylex.props(s.stack)}><span>{fill(errorCopy[1], substitution)}</span><span>{t.fix}: {fill(errorCopy[2], substitution)}</span>
      {error?.code === 'wrong_key' && config.provider === 'garage' ? <details {...stylex.props(s.detail)}><summary>{t.details}</summary><pre {...stylex.props(s.code)}>garage key info viderse-app --show-secret</pre></details> : null}
      {error?.code === 'cors_blocked' || error?.code === 'browser_unreachable' ? <><details open {...stylex.props(s.detail)}><summary>{t.details}</summary><pre {...stylex.props(s.code)}>{corsCode}{config.provider === 'garage' ? `\n\naws s3api put-bucket-cors --endpoint-url ${config.presignEndpoint || config.endpoint || ''} --bucket ${config.bucket} --cors-configuration file://cors.json` : ''}</pre><Button variant="ghost" size="compact" onClick={() => { void navigator.clipboard.writeText(corsCode).then(() => toast.ok(t.copied)).catch(() => toast.err(t.genericError)) }}>{t.copy}</Button></details><span>{t.corsWait}</span></> : null}
      {error?.code === 'cors_blocked' || error?.code === 'browser_unreachable' ? <div {...stylex.props(s.row)}><Button variant="primary" size="compact" pending={busy} onClick={canFillPublic ? () => document.querySelector<HTMLInputElement>('[name="presignEndpoint"]')?.focus() : onRun}>{canFillPublic ? t.fillPublic : t.recheck}</Button>{onProxy ? <Button variant="ghost" size="compact" onClick={onProxy}>{t.toProxy}</Button> : null}</div> : null}
    </div></Banner> : null}
  </section>
}
