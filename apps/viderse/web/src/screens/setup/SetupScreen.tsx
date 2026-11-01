import * as stylex from '@stylexjs/stylex'
import { setupStyles } from './setup.stylex'
// Chuẩn bị máy #/setup (W1, Onboard · step=setup): khung riêng không nav —
// header “Viderse · Lần đầu mở” + Cài đặt nâng cao; hero orb bên trái tiêu đề;
// 4 chip trợ lý; thẻ tiến độ riêng rồi thẻ 6 bước riêng; câu chân. Bước ChatGPT
// là nút primary duy nhất khi tới lượt đăng nhập, đang cài thì là nút nhỏ
// “Đăng nhập ngay”. Xong → #/channels/new (W2, phase sau; ở đây chỉ chuyển bước).
// Mobile chỉ hiện lời nhắn mở trên máy tính. Trạng thái chạy:
// idle|running|login|failed|done do server quyết; màn tự POST start khi idle.
import { useEffect } from 'react'
import { Icon, ProgressBar, Spinner, useAsyncAction } from '@noverse/ui'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { invalidate, setData } from '@noverse/ui/data'
import { navigate, paths, routePath } from '../../router'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { vi as t } from '../../i18n/vi'
import { getSetup, retrySetupStep, startSetup, type SetupStatus, type SetupStep } from '../../api/setup'
import { ActionButton } from '../../components/ActionButton'
import { LensLogo } from '../../components/LensLogo'
import { Orb } from '../../components/Orb'
import { fill, fmtBytes, fmtEta } from '../settings/fmt'
import { ChatGPTLoginAction } from '../settings/connect/ChatGPTLogin'
import { box } from '../../styles/shared.stylex'

/** ETA của bước đang cài, tính từ byte đã tải và giây đã trôi (null = chưa đo được). */
function runningEta(step: SetupStep | undefined): number | null {
  const p = step?.progress
  if (!p?.startedAt || !p.bytesTotal || p.bytesDone <= 0) return null
  const elapsed = (Date.now() - new Date(p.startedAt).getTime()) / 1000
  if (!(elapsed > 3)) return null
  const rate = p.bytesDone / elapsed
  if (!(rate > 0)) return null
  return Math.max(1, Math.round((p.bytesTotal - p.bytesDone) / rate))
}

/** Lý do lỗi đọc được từ câu lỗi thô của server (artboard: “Mạng bị ngắt…”). */
function reasonOf(raw: string | null | undefined): string {
  const s = (raw ?? '').toLowerCase()
  const r = t.prep.states.reasons
  if (!s) return ''
  if (s.includes('checksum') || s.includes('chữ ký')) return r.checksum
  if (s.includes('cancel') || s.includes('huỷ') || s.includes('hủy')) return r.cancelled
  if (s.includes('no space') || s.includes('disk') || s.includes('chỗ')) return r.space
  if (s.includes('timeout') || s.includes('deadline') || s.includes('quá chậm')) return r.timeout
  if (s.includes('network') || s.includes('connection') || s.includes('dns') || s.includes('eof') ||
    s.includes('unreachable') || s.includes('mạng') || s.includes('kết nối')) return r.network
  return ''
}

/** Câu phụ của hàng lỗi: lý do + phần đã tải còn giữ (artboard setup=failed). */
function failedHint(step: SetupStep): string {
  const reason = reasonOf(step.error)
  const kept = step.progress?.bytesDone
    ? fill(t.prep.states.failedKept, { kept: fmtBytes(step.progress.bytesDone) })
    : t.prep.states.failedRetry
  return reason ? `${reason} ${kept}` : kept
}

function StepRow({ step, index, focus }: { step: SetupStep; index: number; focus: boolean }) {
  const copy = t.prep.steps[step.id] ?? { title: step.id, hint: '', agent: 'scout', unlocks: '' }
  const retry = useAsyncAction(async () => { await retrySetupStep(step.id) })
  const { state, progress } = step
  const done = state === 'done'
  const running = state === 'running'
  const failed = state === 'failed'
  const pending = state === 'pending'
  const needsUser = state === 'needs_user'
  const size = step.sizeBytes > 0 ? fmtBytes(step.sizeBytes) : ''
  // pct null = pha chưa biết tổng (tải mô hình, dựng venv): chỉ ghi tên pha,
  // không vẽ “0%” và thanh đứng yên suốt mấy phút.
  const pct = progress?.pct == null ? null : Math.round(progress.pct)
  const phase = progress?.phaseLabel || progress?.phase || t.prep.card.running

  const hint = running
    ? progress?.bytesTotal
      ? fill(t.prep.states.phaseBytes, { phase, done: fmtBytes(progress.bytesDone), total: fmtBytes(progress.bytesTotal) })
      : pct == null ? phase : fill(t.prep.states.phasePct, { phase, pct })
    : failed
      ? failedHint(step)
      : !step.provided && copy.hintManaged
        ? copy.hintManaged
        : copy.hint

  const glyph = done ? 'tick-b' : failed ? 'danger' : needsUser ? 'flag' : 'download'
  return (
    <li className={`vv-setup__row is-${state}${needsUser && focus ? ' is-focus' : ''} ${stylex.props(setupStyles.row, running && setupStyles.rowRunning, failed && setupStyles.rowFailed, needsUser && focus && setupStyles.rowFocus).className}`}>
      <span className={`vv-setup__state ${stylex.props(setupStyles.state, done && setupStyles.stateDone, running && setupStyles.stateRunning, failed && setupStyles.stateFailed, needsUser && focus && setupStyles.stateFocus).className}`} aria-hidden="true">
        {running ? <span className={`vv-setup__spin ${stylex.props(setupStyles.spin).className}`} /> : pending ? <span className="num">{index + 1}</span> : <Icon name={glyph} size={15} />}
      </span>
      <div className={`vv-setup__row-main ${stylex.props(setupStyles.rowMain).className}`}>
        <span className={`vv-setup__row-title ${stylex.props(setupStyles.rowTitle).className}`}>
          {copy.title}
          <span role="img" aria-label={copy.agent === 'artist' && needsUser ? `Hoạ sĩ và Đạo diễn` : undefined} title={copy.agent === 'artist' && needsUser ? 'Dùng cho Hoạ sĩ và Đạo diễn' : undefined}>
            <Orb of={copy.agent as 'scout' | 'director' | 'artist' | 'qa'} size={10} />
          </span>
        </span>
        <span className={`vv-setup__row-hint ${stylex.props(setupStyles.rowHint, running && setupStyles.hintRunning, failed && setupStyles.hintFailed).className}`} title={failed && step.error ? step.error : undefined}>{hint}</span>
        {running && pct != null ? (
          <span className={`vv-setup__bar ${stylex.props(setupStyles.bar).className}`} role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}>
            <span className={`vv-setup__bar-fill ${stylex.props(setupStyles.barFill).className}`} style={{ width: `${pct}%` }} />
          </span>
        ) : null}
      </div>
      {done ? <span className={`vv-setup__row-side ${stylex.props(setupStyles.rowSide).className}`}>{step.provided ? t.prep.states.provided : size ? fill(t.prep.states.done, { size }) : t.prep.states.doneNoSize}</span> : null}
      {running && pct != null ? <span className={`vv-setup__row-side num ${stylex.props(setupStyles.rowSide, setupStyles.rowSideInfo).className}`}>{pct}%</span> : null}
      {pending ? <span className={`vv-setup__row-side ${stylex.props(setupStyles.rowSide).className}`}>{size ? fill(t.prep.states.waiting, { size }) : t.prep.states.waitingNoSize}</span> : null}
      {failed ? (
        <ActionButton variant="outline" size="compact" pending={retry.pending} onClick={() => void retry.run()}>{t.prep.actions.retry}</ActionButton>
      ) : null}
      {/* Onboard dùng `primary` 36/r10/13.5 của artboard, còn ghost “Đăng nhập ngay” là
          `.ghost` 32 — hai cỡ khác nhau trong cùng một hàng, nên truyền cỡ theo variant. */}
      {needsUser ? <ChatGPTLoginAction waitStyle={setupStyles.loginWait} linkStyle={setupStyles.loginLink} variant={focus ? 'primary' : 'ghost'} size={focus ? 'md' : 'compact'} onDone={() => invalidate(keys.setup())} /> : null}
    </li>
  )
}

/** Đăng nhập là việc kế tiếp: không còn bước nào đang chờ/đang chạy trước nó. */
function loginFocusOf(rows: SetupStep[]): boolean {
  const login = rows.find((s) => s.id === 'codex-login')
  if (!login || login.state !== 'needs_user') return false
  return !rows.some((s) => s.state === 'pending' || s.state === 'running')
}

/** Thẻ tiến độ riêng: câu theo trạng thái máy + số bước + thanh + câu trấn an. */
function ProgressCard({ data, rows, startPending, startError }: {
  data: SetupStatus; rows: SetupStep[]; startPending: boolean; startError: string | null
}) {
  const total = rows.length
  const done = rows.filter((s) => s.state === 'done').length
  const failedRow = rows.find((s) => s.state === 'failed')
  const running = rows.find((s) => s.state === 'running')
  const eta = runningEta(running)
  let headline: string = t.prep.card.running
  let sub: string = t.prep.card.sub.running
  let tone: 'writer' | 'warn' = 'writer'
  if (startError) { headline = t.prep.card.startFailed; sub = startError; tone = 'warn' }
  else if (data.state === 'idle') { headline = startPending ? t.prep.card.starting : t.prep.card.idle; sub = t.prep.card.sub.idle }
  else if (data.complete || data.state === 'done') { headline = t.prep.card.done; sub = t.prep.card.sub.done }
  else if (data.state === 'failed' || failedRow) {
    headline = t.prep.card.failed
    const n = failedRow ? rows.indexOf(failedRow) + 1 : 3
    const unlocks = failedRow ? (t.prep.steps[failedRow.id]?.unlocks ?? '') : ''
    sub = unlocks ? fill(t.prep.card.sub.failed, { n, what: unlocks }) : t.prep.card.sub.failed.replace('Cần cài xong bước {n} thì mới tạo được {what}.', '')
    tone = 'warn'
  } else if (data.state === 'login') { headline = t.prep.card.login; sub = t.prep.card.sub.login }
  else if (eta) headline = fill(t.prep.card.runningEta, { eta: fmtEta(eta) })
  return (
    <section className={`vv-setup__card ${stylex.props(setupStyles.card, box.card).className}`} aria-live="polite">
      <div className={`vv-setup__card-head ${stylex.props(setupStyles.cardHead).className}`}>
        <b {...stylex.props(setupStyles.cardHeading)}>{headline}</b>
        <span className={`num ${stylex.props(setupStyles.cardCount).className}`}>{fill(t.prep.card.count, { done, total })}</span>
      </div>
      <div className={`vv-setup__track${tone === 'warn' ? ' is-fail' : ''} ${stylex.props(setupStyles.track).className}`} role="progressbar" aria-valuenow={total ? Math.round((done / total) * 100) : 0} aria-valuemin={0} aria-valuemax={100}>
        <span className={`vv-setup__track-fill ${stylex.props(setupStyles.trackFill, tone === 'warn' && setupStyles.trackFillFailed).className}`} style={{ width: `${total ? Math.round((done / total) * 100) : 0}%` }} />
      </div>
      <p className={`vv-setup__card-sub ${stylex.props(setupStyles.cardSub).className}`}>{sub}</p>
    </section>
  )
}

export default function SetupScreen() {
  const mobile = useMediaQuery('(max-width: 767px)')
  const res = useResource(keys.setup(), ({ signal }) => getSetup({ signal }))
  // POST start trả ảnh chụp mới: ghi thẳng vào cache để thẻ tiến độ và các hàng
  // đổi ngay sang trạng thái thật, không đứng ở “Chưa bắt đầu” trong lúc chờ SSE.
  const start = useAsyncAction(async () => {
    const next = await startSetup()
    if (next) setData(keys.setup(), next)
  })
  const data = res.data

  useEffect(() => {
    if (mobile || !data?.complete) return
    const timer = setTimeout(() => navigate(routePath({ name: 'channel-new' })), 1200)
    return () => clearTimeout(timer)
  }, [mobile, data?.complete])

  useEffect(() => {
    // Server chỉ chạy khi được POST start — đúng một lần cho mỗi lần tải màn.
    // Máy mới luôn `complete=false`, nên điều kiện là “chưa xong + đang idle +
    // đủ chỗ” (không phải `!complete` — điều đó chặn chính máy mới).
    if (mobile || !data || data.complete || !data.disk.enough) return
    if (data.state === 'idle') void start.run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mobile, data?.state, data?.complete, data?.disk.enough])

  if (mobile) {
    return (
      <div className={`vv-setup ${stylex.props(setupStyles.page).className}`}>
        <div className={`vv-setup__mob ${stylex.props(setupStyles.mob).className}`}>
          <Icon name="cpu" size={28} />
          <h1 {...stylex.props(setupStyles.mobileTitle)}>{t.prep.mobile.title}</h1>
          <p {...stylex.props(setupStyles.mobileBody)}>{t.prep.mobile.body}</p>
          <span>{t.prep.mobile.noSkip}</span>
          <ActionButton variant="quiet" size="compact" onClick={() => navigate(paths.workbench())}>{t.prep.back}</ActionButton>
        </div>
      </div>
    )
  }

  if (res.status === 'loading' && !data) return <div className={`vv-setup ${stylex.props(setupStyles.page).className}`}><Spinner size={20} /></div>
  if (!data) {
    return (
      <div className={`vv-setup ${stylex.props(setupStyles.page).className}`}>
        <span {...stylex.props(box.errline)}><span {...stylex.props(box.grow)}>{(res.error as Error | undefined)?.message ?? ''}</span><ActionButton variant="outline" size="compact" onClick={() => void res.refresh()}>{t.common.retry}</ActionButton></span>
      </div>
    )
  }

  const rows = data.steps
  // “khoảng X” là tổng cài đặt xấp xỉ của các bước còn thiếu (approxBytes) —
  // KHÔNG phải requiredBytes (chỗ trống dự phòng của hàm kiểm ổ, lớn hơn 2–4 lần).
  const sizeLine = data.disk.approxBytes > 0 ? fill(t.prep.hero.size, { size: fmtBytes(data.disk.approxBytes) }) : t.prep.hero.noSize
  const loginFocus = loginFocusOf(rows) // bước đăng nhập thành focus khi không còn bước trước
  return (
    <div className={`vv-setup ${stylex.props(setupStyles.page).className}`}>
      <header className={`vv-setup__head ${stylex.props(setupStyles.head).className}`}>
        <LensLogo size={26} />
        <span className={`vv-setup__brand ${stylex.props(setupStyles.brand).className}`}>
          <b {...stylex.props(setupStyles.brandName)}>{t.prep.brand}</b>
          <span className={`vv-setup__first ${stylex.props(setupStyles.first).className}`}>{t.prep.firstRun}</span>
        </span>
        <a className={`vv-setup__adv ${stylex.props(setupStyles.adv).className}`} href="#/settings">
          <Icon name="setting" size={15} className="vv-ic" />
          {t.prep.actions.settingsLink}
        </a>
      </header>
      <main className={`vv-setup__main ${stylex.props(setupStyles.main).className}`}>
        <div className={`vv-setup__col ${stylex.props(setupStyles.col).className}`}>
          <div className={`vv-setup__hero ${stylex.props(setupStyles.hero).className}`}>
            <LensLogo size={64} hero />
            <div className={`vv-setup__hero-text ${stylex.props(setupStyles.heroText).className}`}>
              <h1 {...stylex.props(setupStyles.heroTitle)}>{t.prep.hero.title}</h1>
              <p {...stylex.props(setupStyles.heroBody)}>{sizeLine}</p>
            </div>
          </div>
          <div className={`vv-setup__chips ${stylex.props(setupStyles.chips).className}`}>
            {t.prep.chips.map((c) => (
              <span key={c.key} className={`vv-setup__chip ${stylex.props(setupStyles.chip).className}`}>
                <Orb of={c.key as 'scout' | 'director' | 'artist' | 'qa'} size={12} />
                <b {...stylex.props(setupStyles.chipName)}>{c.name}</b>
                <span>{c.role}</span>
              </span>
            ))}
          </div>
          {!data.disk.enough ? (
            <p className={`vv-setup__warn ${stylex.props(setupStyles.warn).className}`} role="alert">
              <Icon name="warning" size={15} className="vv-ic" />
              {fill(t.prep.disk.low, { free: fmtBytes(data.disk.freeBytes), need: fmtBytes(data.disk.requiredBytes) })}
            </p>
          ) : null}
          <ProgressCard data={data} rows={rows} startPending={start.pending} startError={start.error ? (start.error as Error).message : null} />
          <ol className={`vv-setup__steps ${stylex.props(setupStyles.steps, box.card).className}`} aria-label={t.prep.hero.title}>
            {rows.map((s, i) => <StepRow key={s.id} step={s} index={i} focus={loginFocus} />)}
          </ol>
          {data.complete ? (
            <ActionButton variant="primary" size="md" onClick={() => navigate(routePath({ name: 'channel-new' }))}>
              {t.prep.card.doneAction}
            </ActionButton>
          ) : null}
          <p className={`vv-setup__note ${stylex.props(setupStyles.note).className}`}>
            <Icon name="info" size={15} className={`vv-ic ${stylex.props(setupStyles.noteIcon).className}`} />
            <span>{t.prep.footnote}</span>
          </p>
        </div>
      </main>
    </div>
  )
}
