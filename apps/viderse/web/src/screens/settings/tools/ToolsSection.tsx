import * as stylex from '@stylexjs/stylex'
import { box } from '../../../styles/shared.stylex'
import { styles as settingsStyles, toolsListMarker, toolsAdvMarker, toolsIboxMarker, isWarnMarker } from '../settings.stylex'
// Cài đặt › Công cụ trên máy — Settings.dc.html (tools=ok|update|failed|
// installing) và MSettings.dc.html (screen=tools). Thẻ tổng “Máy sẵn sàng …”
// với Kiểm tra lại; mỗi hàng một nhãn trạng thái, dung lượng + câu mô tả
// (server ghép phiên bản), tiến độ khi đang cài, nút chỉ khi có việc; vùng
// “Nâng cao · thư mục dữ liệu và phiên bản” giữ tên binary. Cài/cập nhật chạy
// nền, tiến độ về qua SSE `tools` (invalidate khoá `tools`).
import { Button, Icon, ProgressBar, Spinner, useAsyncAction } from '@noverse/ui'
import { setData } from '@noverse/ui/data'
import { useResource } from '../../../data/useResource'
import { useMediaQuery } from '../../../lib/useMediaQuery'
import { keys } from '../../../data/keys'
import { t } from '../../../i18n/settings'
import { checkTools, getTools, installTool, type ToolId, type ToolRow, type ToolState } from '../../../api/tools'
import { ActionButton } from '../../../components/ActionButton'
import { fill, fmtBytes, fmtEta, fmtTime } from '../fmt'

const TOOL_ICONS: Record<string, string> = {
  'yt-dlp': 'download',
  ffmpeg: 'scissor',
  transcription: 'subtitle',
  chrome: 'video-play',
  codex: 'magicpen',
  'codex-login': 'msg',
}

/** Nhãn + màu của một hàng (artboard T(): một pill, màu theo mức nặng). */
function pillFor(tool: ToolRow, queued = false): { label: string; tone: '' | 'warn' | 'info' | 'err' | 'muted'; spin?: boolean } {
  // Đang có lượt cài chạy: hàng chưa cài đứng sau lượt đó là “Chờ” trong hàng
  // đợi của Chuẩn bị máy, không phải “Chưa cài” (artboard tools=installing).
  if (queued) return { label: t.tools.pills.queued, tone: 'muted' }
  switch (tool.state) {
    case 'update': return { label: t.tools.pills.update, tone: 'warn' }
    case 'missing': return { label: t.tools.pills.missing, tone: 'muted' }
    case 'failed': return { label: t.tools.pills.failed, tone: 'err' }
    case 'installing': return { label: t.tools.pills.installing, tone: 'info', spin: true }
    case 'needs_login': return { label: t.tools.pills.needsLogin, tone: 'warn' }
    case 'ready':
      return tool.id === 'codex-login'
        ? { label: t.tools.pills.loggedIn, tone: '' }
        : { label: t.tools.pills.ready, tone: '' }
  }
}

type RowAction = { label: string; kind: 'install'; warn?: boolean }

/** Nút chỉ hiện khi có việc; hàng đã cài không có nút (V7). Hàng đăng nhập
 *  ChatGPT cũng không nút — việc đó làm ở Kết nối AI (artboard). */
function actionFor(tool: ToolRow): RowAction | null {
  if (tool.id === 'codex-login') return null
  switch (tool.state) {
    case 'missing': return { label: t.tools.actions.install, kind: 'install' }
    case 'update': return { label: t.tools.actions.update, kind: 'install' }
    case 'failed': return { label: t.tools.actions.retry, kind: 'install', warn: true }
    default: return null
  }
}

/** Dung lượng của hàng: Chrome hệ thống hiện “Có sẵn”, hàng đăng nhập không có. */
function sizeLabel(tool: ToolRow): string {
  if (tool.id === 'codex-login') return ''
  if (tool.provided) return t.tools.sizeProvided
  return tool.sizeBytes > 0 ? fmtBytes(tool.sizeBytes) : ''
}

/** ETA của lượt cài đang chạy, tính từ byte đã tải và giây đã trôi (null = chưa đo được). */
function etaSeconds(tool: ToolRow | undefined): number | null {
  const p = tool?.progress
  if (!p?.startedAt || !p.bytesTotal || p.bytesDone <= 0) return null
  const elapsed = (Date.now() - new Date(p.startedAt).getTime()) / 1000
  if (!(elapsed > 3)) return null
  const rate = p.bytesDone / elapsed
  if (!(rate > 0)) return null
  return Math.max(1, Math.round((p.bytesTotal - p.bytesDone) / rate))
}

/** Tông icon-tile theo trạng thái (artboard: cập nhật giữ mực trung lập,
 *  chờ/cần-bạn nhạt hơn, chỉ lỗi và đang cài đổi màu). */
function toneOf(state: ToolState): '' | 'warn' | 'info' | 'err' | 'muted' {
  switch (state) {
    case 'failed': return 'err'
    case 'installing': return 'info'
    case 'missing': case 'needs_login': return 'muted'
    default: return ''
  }
}

/** Câu mô tả của hàng: hàng lỗi = lỗi server + hậu quả ở bước video (artboard
 *  toolsBad); hàng đang cài là mô tả gốc, tiến độ nằm riêng dưới thanh. */
function hintOf(tool: ToolRow, desc: string): { text: string; tone: '' | 'err' } {
  if (tool.state === 'installing') return { text: desc, tone: '' }
  if (tool.state === 'failed') {
    const step = t.tools.blockedStep[tool.id] ?? ''
    const suffix = step ? ' ' + fill(t.tools.failConsequence, { step }) : ''
    return { text: `${tool.hint}${suffix}`, tone: 'err' }
  }
  return { text: tool.hint, tone: '' }
}

/** Câu mô tả bản điện thoại (MSettings): ngắn, không phiên bản — artboard
 *  giữ phiên bản trong thẻ “Chi tiết”. Chỉ thay câu của hàng đã cài/có bản
 *  mới; hàng lỗi/đang cài giữ câu của server vì nó chở lý do. */
function hintMobile(tool: ToolRow, desc: string): { text: string; tone: '' | 'err' | 'note' } {
  const base = t.tools.descsMobile[tool.id]
  if (!base) return hintOf(tool, desc)
  if (tool.state === 'installing') return { text: desc, tone: '' }
  if (tool.state === 'failed') return hintOf(tool, desc)
  // Cần đăng nhập giữ câu riêng của server (nói đúng việc phải đăng nhập),
  // không dùng câu mô tả ngắn “đã đăng nhập” của bản mobile.
  if (tool.state === 'needs_login') return { text: tool.hint || desc, tone: '' }
  return { text: base, tone: tool.state === 'update' ? 'note' : '' }
}

function ToolRowView({ tool, mobile, queued = false }: { tool: ToolRow; mobile: boolean; queued?: boolean }) {
  const pill = pillFor(tool, queued)
  const tone = toneOf(tool.state)
  const act = queued ? null : actionFor(tool)
  const desc = t.tools.descs[tool.id] ?? ''
  const running = tool.state === 'installing'
  const hint = mobile ? hintMobile(tool, desc) : hintOf(tool, desc)
  const size = sizeLabel(tool)

  const run = useAsyncAction(async (id: ToolId) => installTool(id))

  const onClick = () => {
    if (!act) return
    void run.run(tool.id)
  }

  // pct null = pha chưa biết tổng: không vẽ thanh 0% đứng yên, chỉ ghi tên pha.
  const pct = running && tool.progress?.pct != null ? Math.round(tool.progress.pct) : null
  const bytes = running && tool.progress?.bytesTotal
    ? `${fmtBytes(tool.progress.bytesDone)} / ${fmtBytes(tool.progress.bytesTotal)}`
    : ''

  return (
    <div className={`vv-tools__row ${stylex.props(settingsStyles.toolsRow).className}`} role="listitem">
      <span className={`vv-tools__ibox ${stylex.props(toolsIboxMarker, settingsStyles.toolsIbox, tone === 'err' && settingsStyles.toolsIboxIsErr, tone === 'info' && settingsStyles.toolsIboxIsInfo, tone === 'muted' && settingsStyles.toolsIboxIsMuted, tone === 'warn' && settingsStyles.toolsIboxIsWarn).className}`} aria-hidden="true">
        <Icon name={TOOL_ICONS[tool.id] ?? 'cpu'} size={17} />
      </span>
      <div className={`vv-tools__main ${stylex.props(settingsStyles.toolsMain).className}`}>
        <span className={`vv-tools__title-line ${stylex.props(settingsStyles.toolsTitleLine).className}`}>
          <span className={`vv-tools__title ${stylex.props(settingsStyles.toolsTitle).className}`}>{t.tools.jobNames[tool.id] ?? tool.id}</span>
          {size ? <span className={`vv-tools__size ${stylex.props(settingsStyles.toolsSize).className}`}>{size}</span> : null}
        </span>
        <span className={`vv-tools__hint ${stylex.props(settingsStyles.toolsHint, hint.tone === 'err' && settingsStyles.toolsHintIsErr, hint.tone === 'note' && settingsStyles.toolsHintIsNote).className}`}>{hint.text}</span>
        {running ? (
          <div className={`vv-tools__prog ${stylex.props(settingsStyles.toolsProg).className}`} aria-live="polite">
            {pct != null ? <ProgressBar value={pct} tone="info" height={4} /> : null}
            <span className={`vv-tools__prog-line ${stylex.props(settingsStyles.toolsProgLine).className}`}>
              <span className={`vv-tools__shimmer ${stylex.props(settingsStyles.toolsShimmer).className}`}>{tool.progress?.phaseLabel || tool.progress?.phase || t.tools.pills.installing}</span>
              <span>{bytes || (pct != null ? `${pct}%` : '')}</span>
            </span>
            {/* Pha chưa biết tổng (hỏi đáp #30): thanh chạy vô định dưới tên pha, không ghi % */}
            {pct == null ? (
              <div role="progressbar" aria-label={tool.progress?.phaseLabel || tool.progress?.phase || t.tools.pills.installing} className={`vv-progress vv-progress--working ${stylex.props(settingsStyles.toolsIndet).className}`}>
                <span className={stylex.props(settingsStyles.toolsIndetFill).className} />
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
      <span className={`vv-tools__side ${stylex.props(settingsStyles.toolsSide).className}`}>
        <span className={`vv-tools__pill ${stylex.props(settingsStyles.toolsPill, pill.tone === 'warn' && settingsStyles.toolsPillIsWarn, pill.tone === 'info' && settingsStyles.toolsPillIsInfo, pill.tone === 'err' && settingsStyles.toolsPillIsErr, pill.tone === 'muted' && settingsStyles.toolsPillIsMuted).className}`}>
          {pill.spin ? <span className={`vv-tools__spin ${stylex.props(settingsStyles.toolsSpin).className}`} aria-hidden="true" /> : null}
          {pill.label}
        </span>
        {act ? (
          <Button
            xstyle={settingsStyles.toolsAct}
            variant={mobile && tool.state === 'update' ? 'primary' : act.warn ? 'warnGhost' : 'ghost'}
            size={mobile ? 'lg' : 'compact'}
            pending={run.pending}
            icon={<Icon name="download" size={14} className="vv-tools__act-ic" xstyle={[settingsStyles.toolsActIc]} />}
            onClick={onClick}
          >
            {act.label}
            {size && tool.state === 'update' ? <span className={`vv-tools__act-size ${stylex.props(settingsStyles.toolsActSize).className}`}> · {size}</span> : null}
          </Button>
        ) : null}
      </span>
    </div>
  )
}

/** Thẻ tổng của mục — câu + màu theo việc nặng nhất đang có. */
function Summary({ tools, checkedAt }: { tools: ToolRow[]; checkedAt: string }) {
  const failed = tools.filter((x) => x.state === 'failed')
  const installing = tools.filter((x) => x.state === 'installing')
  const missing = tools.filter((x) => x.state === 'missing')
  const updates = tools.filter((x) => x.state === 'update')
  const time = fmtTime(checkedAt)

  let title: string = t.tools.summary.ok
  let sub = fill(t.tools.summary.okSub, { time })
  let icon = 'tick-b'
  let tone: '' | 'warn' | 'info' | 'err' = ''
  if (failed.length > 0) {
    title = fill(t.tools.summary.failed, { n: failed.length })
    const step = t.tools.blockedStep[failed[0].id] ?? ''
    sub = step ? fill(t.tools.summary.failedSub, { step }) : t.tools.summary.failedSub.replace('bước {step}', 'bước bị lỗi')
    icon = 'danger'
    tone = 'err'
  } else if (installing.length > 0) {
    const i = tools.findIndex((x) => x.state === 'installing') + 1
    const eta = etaSeconds(installing[0])
    title = eta
      ? fill(t.tools.summary.installing, { i, eta: fmtEta(eta) })
      : fill(t.tools.summary.installingNoEta, { i })
    sub = t.tools.summary.installingSub
    icon = 'download'
    tone = 'info'
  } else if (updates.length > 0) {
    title = fill(t.tools.summary.update, { n: updates.length })
    sub = fill(t.tools.summary.updateSub, { time })
    icon = 'tick-b'
    tone = 'warn'
  } else if (missing.length > 0) {
    title = fill(t.tools.summary.missing, { n: missing.length })
    sub = t.tools.summary.missingSub
    icon = 'download'
    tone = 'info'
  }

  return (
    <section className={`vv-tools__sum ${stylex.props(settingsStyles.card, settingsStyles.toolsSum, (tone === 'err') && settingsStyles.toolsSumIsErr).className}`} role="status">
      <Icon name={icon} size={20} className="vv-tools__sum-ic" xstyle={[settingsStyles.toolsSumIc, tone === 'warn' && settingsStyles.toolsSumIcIsWarn, tone === 'info' && settingsStyles.toolsSumIcIsInfo, tone === 'err' && settingsStyles.toolsSumIcIsErr]} />
      <div className={`vv-tools__sum-main ${stylex.props(settingsStyles.toolsSumMain).className}`}>
        <span className={`vv-tools__sum-title ${stylex.props((tone === 'err') && isWarnMarker, settingsStyles.toolsSumTitle, (tone === 'err') && settingsStyles.toolsSumTitleIsWarn).className}`}>{title}</span>
        <span className={`vv-tools__sum-sub ${stylex.props(settingsStyles.toolsSumSub).className}`}>{sub}</span>
      </div>
      <RecheckButton />
    </section>
  )
}

/** Nút “Kiểm tra lại” — gọi probe thật rồi ghi thẳng hình mới vào cache.
 *  Thẻ tổng chỉ có ở desktop (MSettings không có), nên dùng cỡ gọn. */
function RecheckButton() {
  const recheck = useAsyncAction(async () => {
    const next = await checkTools()
    if (next) setData(keys.tools(), next)
  })
  return (
    <ActionButton size="compact" pending={recheck.pending} icon={<Icon name="refresh" size={15} />} onClick={() => void recheck.run()}>
      {t.tools.recheck}
    </ActionButton>
  )
}
/** Vùng Nâng cao — hàng cuối của thẻ danh sách trên desktop (artboard);
 *  trên mobile là thẻ riêng “Chi tiết · phiên bản và thư mục” (MSettings). */
function Advanced({ tools, dataDir }: { tools: ToolRow[]; dataDir?: string }) {
  const pairs = tools
    .filter((x) => x.id !== 'codex-login')
    .map((x) => ({
      key: x.id,
      label: t.tools.advanced.techNames[x.id] ?? x.id,
      value: x.version || t.tools.advanced.none,
    }))
  return (
    <details className={`vv-tools__adv ${stylex.props(settingsStyles.card, toolsAdvMarker, settingsStyles.toolsAdv).className}`}>
      <summary className={`vv-tools__adv-sum ${stylex.props(settingsStyles.toolsAdvSum).className}`}>
        <Icon name="info" size={16} className="vv-tools__adv-ic-m vv-ic" xstyle={[settingsStyles.ic, settingsStyles.toolsAdvIcM]} />
        <span className={`vv-tools__adv-label vv-tools__adv-label-d ${stylex.props(settingsStyles.toolsAdvLabelD).className}`}>{t.tools.advanced.title}</span>
        <span className={`vv-tools__adv-label vv-tools__adv-label-m ${stylex.props(settingsStyles.toolsAdvLabelM).className}`}>{t.tools.advanced.mobileTitle}</span>
        <Icon name="arrow-d" size={14} className="vv-tools__adv-ic-d vv-ic vv-tools__adv-caret" xstyle={[settingsStyles.ic, settingsStyles.toolsAdvCaret]} />
      </summary>
      <div className={`vv-tools__adv-body ${stylex.props(settingsStyles.toolsAdvBody).className}`}>
        {dataDir ? (
          <span className={`vv-tools__adv-pair ${stylex.props(settingsStyles.toolsAdvPair).className}`}>
            <span className={`vv-tools__adv-k ${stylex.props(settingsStyles.toolsAdvK).className}`}>{t.tools.advanced.dataDir}</span>
            <span className={`vv-tools__adv-v ${stylex.props(settingsStyles.toolsAdvV).className}`} title={dataDir}>{dataDir}</span>
          </span>
        ) : null}
        {pairs.map((p) => (
          <span key={p.key} className={`vv-tools__adv-pair ${stylex.props(settingsStyles.toolsAdvPair).className}`}>
            <span className={`vv-tools__adv-k ${stylex.props(settingsStyles.toolsAdvK).className}`}>{p.label}</span>
            <span className={`vv-tools__adv-v ${stylex.props(settingsStyles.toolsAdvV).className}`}>{p.value}</span>
          </span>
        ))}
      </div>
    </details>
  )
}

export default function ToolsSection() {
  const res = useResource(keys.tools(), ({ signal }) => getTools(signal))
  const mobile = useMediaQuery('(max-width: 767px)')

  if (res.status === 'loading' && !res.data) return <Spinner size={16} />
  if (res.status === 'error' && !res.data) {
    return (
      <section className={`vv-tools__card ${stylex.props(settingsStyles.card, settingsStyles.toolsCard).className}`}>
        <span {...stylex.props(box.errline)}>
          <span {...stylex.props(box.grow)}>{t.errors.failed}</span>
          <ActionButton variant="outline" size={mobile ? 'lg' : 'compact'} onClick={() => void res.refresh()}>{t.common.retry}</ActionButton>
        </span>
      </section>
    )
  }

  const tools = res.data?.tools ?? []
  // Chỉ hàng đứng sau lượt đang cài mới là “Chờ” — lượt cài đi theo thứ tự
  // của Chuẩn bị máy, nên hàng trước nó không nằm trong hàng đợi.
  const runningIdx = tools.findIndex((x) => x.state === 'installing')
  const rows = tools.map((tool, i) => (
    <ToolRowView key={tool.id} tool={tool} mobile={mobile} queued={runningIdx >= 0 && i > runningIdx && tool.state === 'missing'} />
  ))
  const advanced = <Advanced tools={tools} dataDir={res.data?.dataDir} />
  return (
    <>
      <Summary tools={tools} checkedAt={res.data?.checkedAt ?? ''} />
      <p className={`vv-tools__lead-m ${stylex.props(settingsStyles.toolsLeadM).className}`}>{t.tools.lead}</p>
      <section className={`vv-tools__list ${stylex.props(settingsStyles.card, toolsListMarker, settingsStyles.toolsList).className}`} role="list" aria-label={t.settings.nav.tools}>
        {rows}
        {/* Desktop: vùng Nâng cao là hàng cuối của chính thẻ danh sách
            (artboard: nút 42px nối liền dưới hàng ChatGPT). Mobile tách thẻ
            riêng “Chi tiết · phiên bản và thư mục” (MSettings). */}
        {mobile ? null : advanced}
      </section>
      {mobile ? advanced : null}
    </>
  )
}
