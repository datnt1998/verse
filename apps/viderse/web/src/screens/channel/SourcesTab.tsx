import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { channelStyles } from './channel.stylex'
// Nguồn & tự động (Channel / MChannel): cùng lựa chọn và request thật;
// desktop neo popover, điện thoại dùng Sheet chung. Một đối thủ có thể làm
// nguồn cho nhiều kênh; chỉ khoá hàng đã liên kết với kênh đang mở.
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { ActionSheet, Button, Checkbox, Icon, Sheet, confirm, toast, useEscapeLayer } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/channels'
import { navigate, paths } from '../../router'
import { keys } from '../../data/keys'
import { listRivals, getRivalScan, getPostingSlots, addRival, type Rival } from '../../api/rivals'
import { useResource } from '../../data/useResource'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { linkSource, unlinkSource, type Channel, type ChannelSource } from '../../api/channels'
import { addRivalError } from '../explore/AddRivalForm'
import { channelTint } from '../../lib/channelTint'
import { AutomationCard } from './AutomationCard'
import { makeLatestRivalVideo } from './latest'
import { mch } from './mobile/mchannel.stylex'
import { fmtAge, fmtSubs } from '../explore/fmt'
import { line } from '@noverse/ui/tokens.stylex'
import { space, typography } from '../../styles/viderse.stylex'

const sx = stylex.create({
  sources: {
    gridTemplateColumns: {
      default: 'minmax(0, 1.3fr) minmax(0, 1fr)',
      '@media (max-width: 1100px)': 'minmax(0, 1fr)',
    },
  },
  card: { gap: 0 },
  rival: {
    display: 'flex', flexDirection: 'column', gap: space['--vv-s-10'],
    padding: 'var(--vv-s-12) var(--vv-s-14)',
    borderTopWidth: space['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-1'],
  },
  info: { display: 'flex', alignItems: 'center', gap: space['--vv-s-10'], minWidth: 0 },
  name: { fontSize: typography['--vv-fs-body'] },
  status: { flexShrink: 0 },
  actions: { display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: space['--vv-s-6'] },
  unlink: { marginInlineStart: 'auto' },
  sheetBody: { display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden' },
  sheetList: { padding: 'var(--vv-s-4) var(--vv-s-8)' },
  sheetRow: { minHeight: space['--vv-s-56'] },
  sheetSub: { whiteSpace: 'normal', overflowWrap: 'anywhere' },
  sheetFoot: { padding: 'var(--vv-s-8) var(--vv-s-16)' },
  sheetMore: { minHeight: space['--vv-s-44'] },
})

const num = new Intl.NumberFormat('vi-VN')
const perWeekText = (n: number) => num.format(Math.round(n * 10) / 10)

export function SourcesTab({ channel, channels }: {
  channel: Channel
  /** mọi kênh — nhận ra đối thủ đang dùng cho kênh khác */
  channels: Channel[]
}) {
  const tt = t.channels.sources
  const phone = useMediaQuery('(max-width: 767px)')
  const rivalsRes = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const rivals = rivalsRes.data?.rivals ?? []
  const [pickerOpen, setPickerOpen] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)

  const rivalOf = (id: string): Rival | undefined => rivals.find((rival) => rival.id === id)

  const closePicker = () => {
    setPickerOpen(false)
    // Esc / bấm ngoài / liên kết xong: trả tiêu điểm về nút mở popover.
    requestAnimationFrame(() => triggerRef.current?.focus())
  }

  const askUnlink = (rivalId: string) => {
    const row = channel.sources.find((source) => source.rivalId === rivalId)
    if (!row) return
    const name = channel.name || t.channels.page.noName
    const nextRef = channel.sources.find((source) => source.rivalId !== rivalId)
    const bullets = [
      tt.unlinkKeep,
      row.reference
        ? (nextRef ? tt.unlinkRefNext(nextRef.title) : tt.unlinkRefNone)
        : tt.unlinkSameIdentity,
      tt.unlinkExplore(row.title),
    ]
    void confirm({
      tone: 'dsoft',
      title: tt.unlinkTitle(row.title, name),
      body: (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
          <p style={{ margin: 0 }}>{tt.unlinkLead}</p>
          <ul style={{ margin: 0, paddingInlineStart: 18, display: 'flex', flexDirection: 'column', gap: 3 }}>
            {bullets.map((line) => <li key={line}>{line}</li>)}
          </ul>
        </div>
      ),
      confirmLabel: tt.unlinkOk,
      onConfirm: async () => {
        const result = await unlinkSource(channel.id, rivalId)
        void invalidate(keys.channels())
        void invalidate(keys.channel(channel.id))
        // `autoFetchPaused` thật từ server: báo thêm khi kênh hết nguồn.
        toast.ok(result.autoFetchPaused ? tt.unlinkPaused : tt.unlinkDone(row.title))
      },
    })
  }

  const linkedElsewhere = (rivalId: string) =>
    channels.filter((other) => other.id !== channel.id && other.sources.some((source) => source.rivalId === rivalId))

  const rivalsCard = (
    <section {...stylex.props(phone ? mch.card16 : [channelStyles.card, channelStyles.cardFlush, sx.card])} aria-label={tt.rivalsTitle}>
      <div {...stylex.props(phone ? mch.rivalHead : channelStyles.cardhead)}>
        {!phone && <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="radar" size={16} />{tt.rivalsTitle} <span className={`num ${stylex.props(channelStyles.rowCount).className}`}>{channel.sources.length}</span></h2>}
        {phone && (
          <>
            <span {...stylex.props(mch.rivalHeadTitle)}>{tt.rivalsTitle}</span>
            <span className={`num ${stylex.props(mch.rivalCount).className}`}>{channel.sources.length}</span>
            <span {...stylex.props(mch.spacer)} />
          </>
        )}
        <div className={`vch-linkslot ${stylex.props(channelStyles.linkslot).className}`}>
          <Button variant="quiet" size={phone ? 'lg' : 'sm'} ref={triggerRef}
            aria-haspopup="dialog"
            aria-expanded={pickerOpen}
            onClick={() => (pickerOpen ? closePicker() : setPickerOpen(true))}
          >
            <Icon name="add" size={14} />
            {tt.linkBtn}
          </Button>
          {pickerOpen && (
            <LinkPickerPopover
              channel={channel}
              rivals={rivals}
              linkedElsewhere={linkedElsewhere}
              mobile={phone}
              anchor={triggerRef}
              onClose={closePicker}
            />
          )}
        </div>
      </div>
      {channel.sources.length === 0 ? (
        <div role="status" {...stylex.props(phone ? mch.emptyBox : channelStyles.empty)} style={phone ? undefined : { margin: '0 var(--vv-s-14) var(--vv-s-12)' }}>
          {phone ? (
            <div {...stylex.props(mch.emptyText)}>
              <b {...stylex.props(mch.emptyTitle)}>{tt.emptyTitle}</b>
              <span {...stylex.props(mch.emptyBody)}>{tt.emptyBody}</span>
            </div>
          ) : (
            <>
              <b {...stylex.props(channelStyles.emptyTitle)}>{tt.emptyTitle}</b>
              <span className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.emptyBody}</span>
              <ActionButton size="sm" onClick={() => setPickerOpen(true)}>{tt.linkBtn}</ActionButton>
            </>
          )}
        </div>
      ) : channel.sources.map((source) => (
        <SourceRow
          key={source.rivalId}
          channelId={channel.id}
          variant={phone ? 'mobile' : 'desktop'}
          source={source}
          rival={rivalOf(source.rivalId)}
          onUnlink={() => askUnlink(source.rivalId)}
        />
      ))}
      <div style={phone ? undefined : { padding: 'var(--vv-s-8) var(--vv-s-14) var(--vv-s-12)' }}>
        <AddRivalRow
          mobile={phone}
          channelId={channel.id}
          onLinked={() => { void invalidate(keys.channels()); void invalidate(keys.rivals()) }}
        />
      </div>
      {phone && <p {...stylex.props(mch.footNote)}>{tt.linkHint}</p>}
    </section>
  )

  if (phone) {
    return (
      <section {...stylex.props(mch.sources)} aria-label={t.channels.page.tabs.sources}>
        <AutomationCard channel={channel} rivals={rivals} mobile />
        {rivalsCard}
      </section>
    )
  }

  return (
    <section className={`vch-sources ${stylex.props(channelStyles.sources, sx.sources).className}`} aria-label={t.channels.page.tabs.sources}>
      {rivalsCard}
      <AutomationCard channel={channel} rivals={rivals} />
    </section>
  )
}

/** Một đối thủ đã liên kết: quét + lịch đăng đọc riêng hàng này (cache theo id). */
function SourceRow({ channelId, source, rival, onUnlink, variant = 'desktop' }: {
  channelId: string
  source: ChannelSource
  rival: Rival | undefined
  onUnlink: () => void
  variant?: 'desktop' | 'mobile'
}) {
  const tt = t.channels.sources
  const [sheet, setSheet] = useState(false)
  const [pendingUnlink, setPendingUnlink] = useState(false)
  const [making, setMaking] = useState(false)
  const scan = useResource(keys.rivalScan(source.rivalId), ({ signal }) => getRivalScan(source.rivalId, signal))
  const posting = useResource(keys.rivalPosting(source.rivalId, 'vn'), ({ signal }) => getPostingSlots(source.rivalId, 'vn', signal))
  // Chỉ hiển thị số quét/lịch đăng khi API có dữ liệu, không thay dữ liệu thiếu bằng 0.
  const history = scan.data?.history
  const scanState = history?.state ?? rival?.scan.state
  const scanning = scanState === 'queued' || scanState === 'running'
  const indexed = scan.data?.indexedVideos ?? rival?.videoCount
  const seenAt = scan.data?.discovery.checkedAt ?? rival?.scannedAt
  const status = scanning ? tt.scanChip : seenAt ? tt.seen(fmtAge(seenAt)) : indexed != null ? tt.scanned(num.format(indexed)) : ''
  const meta = [
    rival?.subscriberCount != null ? `${fmtSubs(rival.subscriberCount)} người theo dõi` : rival?.handle ?? source.handle,
    posting.data?.perWeek != null ? tt.perWeek(perWeekText(posting.data.perWeek)) : '',
    scanning && indexed != null && scan.data?.historyCap != null
      ? tt.scanningHistory(indexed, scan.data.historyCap)
      : indexed != null ? tt.scanned(num.format(indexed)) : '',
    variant === 'mobile' && seenAt ? tt.seen(fmtAge(seenAt)) : '',
  ].filter(Boolean).join(' · ')

  const makeLatest = async () => {
    if (making) return
    setMaking(true)
    try {
      const result = await makeLatestRivalVideo(channelId, source.rivalId)
      toast.ok(t.channels.latestQueued)
      navigate(result.jobId ? paths.workbench({ video: result.jobId }) : paths.queue())
    } catch (err) {
      toast.err(err instanceof Error ? err.message : t.channels.latestFailed)
    } finally {
      setMaking(false)
    }
  }

  if (variant === 'mobile') {
    const href = `#${paths.explore({ rival: rival?.handle ?? source.rivalId })}`
    return (
      <div {...stylex.props(mch.rivalRow)}>
        <ChannelAvatar
          name={source.title}
          src={rival?.avatarUrl ?? undefined}
          color={{ bg: channelTint(source.rivalId) }}
          size={36}
          fontSize={12}
        />
        <div {...stylex.props(mch.rivalText)}>
          <span {...stylex.props(mch.rivalName)} title={source.title}>{source.title}</span>
          {source.reference && <span {...stylex.props(mch.refBadge)}>{tt.linkedBadge}</span>}
          {scanning && <span {...stylex.props(mch.autonomyCur)}>{tt.scanChip}</span>}
          <span {...stylex.props(mch.rivalMeta)} title={meta}>{meta}</span>
        </div>
        <Button variant="icon" size="lg"
          aria-label={`${tt.unlink}: ${source.title}`}
          aria-haspopup="dialog"
          onClick={() => setSheet(true)}
        >
          <Icon name="more" size={16} />
        </Button>
        <ActionButton
          size="lg"
          block
          xstyle={mch.rivalAction}
          pending={making}
          onClick={() => void makeLatest()}
          icon={<Icon name="video-add" size={14} />}
        >
          {tt.makeLatestShort}
        </ActionButton>
        <ActionSheet
          open={sheet}
          onClose={() => setSheet(false)}
          onExited={() => { if (pendingUnlink) { setPendingUnlink(false); onUnlink() } }}
          title={source.title}
          actions={[
            { label: tt.topVideos, icon: 'chart', onSelect: () => navigate(href.slice(1)) },
            { label: tt.openExplore, icon: 'global', onSelect: () => navigate(href.slice(1)) },
            { label: tt.unlink, icon: 'link', destructive: true, onSelect: () => setPendingUnlink(true) },
          ]}
        />
      </div>
    )
  }

  return (
    <div className={`vch-rival ${stylex.props(sx.rival).className}`}>
      <div {...stylex.props(sx.info)}>
        <ChannelAvatar
          name={source.title}
          src={rival?.avatarUrl ?? undefined}
          color={{ bg: channelTint(source.rivalId) }}
          size={34}
          fontSize={11.5}
        />
        <div className={`vch-rival__text ${stylex.props(channelStyles.rivalText).className}`}>
          <span className={`vch-rival__name ${stylex.props(channelStyles.rivalName, sx.name).className}`}>
            {source.title}
            {source.reference && <span className={`vch-refbadge ${stylex.props(channelStyles.refbadge).className}`} title={tt.linkedBadgeTitle}>{tt.linkedBadge}</span>}
          </span>
          <span className={`vch-rival__meta ${stylex.props(channelStyles.rivalMeta).className}`}>{meta}</span>
        </div>
        {status && <span className={`vch-chip ${stylex.props(channelStyles.chip, scanning && channelStyles.chipWarn, sx.status).className}`}>{status}</span>}
      </div>
      <div className={`vch-rival__actions ${stylex.props(sx.actions).className}`}>
        <ActionButton size="sm" pending={making} onClick={() => void makeLatest()} icon={<Icon name="video-add" size={14} />}>
          {tt.makeLatest}
        </ActionButton>
        <ActionButton size="sm" icon={<Icon name="chart" size={14} />} onClick={() => navigate(paths.explore({ rival: rival?.handle ?? source.rivalId }))}>{tt.topVideos}</ActionButton>
        <ActionButton variant="quiet" size="sm" onClick={() => navigate(paths.explore({ rival: rival?.handle ?? source.rivalId }))}>
          {tt.openExplore}<Icon name="arrow-r" size={13} />
        </ActionButton>
        <Button variant="quiet" size="sm" xstyle={sx.unlink} title={tt.unlinkTip} onClick={onUnlink}>{tt.unlink}</Button>
      </div>
    </div>
  )
}

/** Theo dõi đối thủ mới rồi liên kết luôn vào kênh (W2: "Theo dõi & chọn"). */
function AddRivalRow({ channelId, onLinked, mobile = false }: { channelId: string; onLinked: () => void; mobile?: boolean }) {
  const tt = t.channels.sources.add
  const [value, setValue] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const submit = async () => {
    const input = value.trim()
    if (pending) return
    if (!input) {
      setError('Dán @handle hoặc link kênh YouTube của đối thủ trước')
      inputRef.current?.focus()
      return
    }
    setPending(true)
    setError(null)
    try {
      const rival = await addRival(input)
      await linkSource(channelId, rival.id)
      setValue('')
      onLinked()
      toast.ok(t.channels.sources.picker.done)
    } catch (err) {
      setError(addRivalError(err).message || (err instanceof Error ? err.message : tt.failed))
    } finally {
      setPending(false)
    }
  }

  return (
    <form
      aria-label={tt.label}
      onSubmit={(event) => { event.preventDefault(); void submit() }}
      {...stylex.props(mch.rivalForm, mch.sourceRivalForm, mobile && mch.sourceRivalFormMobile)}
    >
      <span {...stylex.props(text.srOnly)}><label htmlFor="vch-add-rival">{tt.label}</label></span>
      <input
        id="vch-add-rival"
        ref={inputRef}
        aria-invalid={!!error || undefined}
        aria-describedby={error ? 'vch-add-rival-error' : undefined}
        style={error ? { borderColor: 'var(--color-err)' } : undefined}
        className={`vch-taginput ${stylex.props(mobile ? mch.input44 : channelStyles.taginput, mch.rivalInput, mobile && mch.rivalInputMobile).className}`}
        value={value}
        placeholder={tt.placeholder}
        onChange={(event) => { setValue(event.target.value); setError(null) }}
      />
      <ActionButton size={mobile ? 'lg' : 'sm'} block={mobile} type="submit" disabled={pending} pending={pending}>
        {tt.submit}
      </ActionButton>
      {error && <span id="vch-add-rival-error" className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert" style={{ flexBasis: '100%' }}>{error}</span>}
    </form>
  )
}

/** Cùng lựa chọn: popover desktop và Sheet modal trên điện thoại. */
function LinkPickerPopover({ channel, rivals, linkedElsewhere, mobile, anchor, onClose }: {
  channel: Channel
  rivals: Rival[]
  linkedElsewhere: (rivalId: string) => Channel[]
  mobile: boolean
  /** Nút mở popover — đo chỗ trống để neo xuống hoặc lật lên. */
  anchor: React.RefObject<HTMLButtonElement | null>
  onClose: () => void
}) {
  const tt = t.channels.sources.picker
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sheetOpen, setSheetOpen] = useState(true)
  const panel = useRef<HTMLDivElement>(null)
  // Desktop giới hạn chiều cao để danh sách cuộn và nút liên kết luôn hiện.
  const [placement, setPlacement] = useState<{ up: boolean; maxHeight: number }>({ up: false, maxHeight: 560 })
  const inChannel = new Set(channel.sources.map((source) => source.rivalId))
  const selected = [...picked].filter((id) => !inChannel.has(id))
  const close = () => { if (mobile) setSheetOpen(false); else onClose() }

  useEscapeLayer(!mobile, onClose)

  useLayoutEffect(() => {
    if (mobile) return
    const trigger = anchor.current
    if (!trigger) return
    const measure = () => {
      const rect = trigger.getBoundingClientRect()
      const below = window.innerHeight - rect.bottom - 16
      const above = rect.top - 16
      const up = below < 320 && above > below
      setPlacement({ up, maxHeight: Math.max(240, Math.min(560, up ? above : below)) })
    }
    measure()
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [anchor, mobile])

  useEffect(() => {
    if (mobile) return
    // Mở ra là đặt tiêu điểm vào hàng chọn được đầu tiên (bàn phím đi tiếp bằng Tab).
    panel.current?.querySelector<HTMLElement>('[role="checkbox"]:not([aria-disabled="true"])')?.focus()
  }, [mobile])

  useEffect(() => {
    if (mobile) return
    const outside = (event: PointerEvent) => {
      const target = event.target as Node
      if (panel.current?.contains(target) || anchor.current?.contains(target)) return
      onClose()
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [anchor, mobile, onClose])

  const toggle = (id: string) => setPicked((prev) => {
    const next = new Set(prev)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    return next
  })

  const link = async () => {
    if (selected.length === 0 || pending) return
    setPending(true)
    setError(null)
    try {
      for (const rivalId of selected) await linkSource(channel.id, rivalId)
      void invalidate(keys.channels())
      void invalidate(keys.channel(channel.id))
      toast.ok(tt.done)
      close()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.channels.sources.saveError)
      setPending(false)
    }
  }

  const content = (
    <>
      {!mobile && <div className={`vch-linkpop__head ${stylex.props(channelStyles.linkpopHead).className}`}>
        <span className={`vch-linkpop__title ${stylex.props(channelStyles.linkpopTitle).className}`}>{tt.title(channel.name || t.channels.page.noName)}</span>
        <span className={`vch-linkpop__hint ${stylex.props(channelStyles.linkpopHint).className}`}>{t.channels.sources.linkHint}</span>
      </div>}
      {rivals.length === 0 ? (
        <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`} style={{ padding: '0 var(--vv-s-10) var(--vv-s-6)' }}>{tt.empty}</p>
      ) : (
        <div role="group" aria-label={t.channels.sources.rivalsTitle} className={`vch-pickerlist ${stylex.props(channelStyles.pickerlist, mobile && sx.sheetList).className}`}>
          {rivals.map((rival) => {
            const linked = inChannel.has(rival.id)
            const others = linkedElsewhere(rival.id)
            const reason = linked ? tt.alreadyLinked : others.length > 0
              ? `${tt.linkedElsewhere(others.map((other) => other.name || t.channels.page.noName).join(', '))} · dùng chung được`
              : ''
            return (
              <Checkbox key={rival.id} checked={picked.has(rival.id) || linked}
                aria-disabled={linked || undefined} title={reason || undefined}
                xstyle={[channelStyles.pickerrow, mobile && sx.sheetRow]} onChange={() => { if (!linked) toggle(rival.id) }}
                label={<>
                <ChannelAvatar
                  name={rival.title}
                  src={rival.avatarUrl ?? undefined}
                  color={{ bg: channelTint(rival.id) }}
                  size={28}
                  style={{ fontSize: 10.5 }}
                />
                <span className={`vch-pickerrow__text ${stylex.props(channelStyles.pickerrowText).className}`}>
                  <span className={`vch-pickerrow__name ${stylex.props(channelStyles.pickerrowName).className}`}>{rival.title}</span>
                  <span className={`vch-pickerrow__sub ${stylex.props(channelStyles.pickerrowSub, mobile && sx.sheetSub).className}`}>
                    {reason || (rival.subscriberCount != null
                      ? `${fmtSubs(rival.subscriberCount)} người theo dõi`
                      : rival.handle ?? '')}
                  </span>
                </span>
                </>} />
            )
          })}
        </div>
      )}
      {error && <p className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert" style={{ padding: '0 var(--vv-s-10) var(--vv-s-6)' }}>{error}</p>}
      <div className={`vch-linkpop__foot ${stylex.props(channelStyles.linkpopFoot, mobile && sx.sheetFoot).className}`}>
        <a href={`#${paths.explore()}`} className={`vch-linkpop__more ${stylex.props(channelStyles.linkpopMore, mobile && sx.sheetMore).className}`}>
          <Icon name="radar" size={13} />
          {tt.followNew}
        </a>
        <ActionButton
          variant="primary"
          size={mobile ? 'lg' : 'sm'}
          disabled={selected.length === 0}
          pending={pending}
          onClick={() => void link()}
        >
          {selected.length === 0 ? tt.none : tt.selected(selected.length)}
        </ActionButton>
      </div>
    </>
  )

  if (mobile) {
    return (
      <Sheet open={sheetOpen} onClose={close} onExited={onClose}
        title={tt.title(channel.name || t.channels.page.noName)}
        subtitle={t.channels.sources.linkHint} size="md" height="auto" homeIndicator bodyStyle={sx.sheetBody}
      >
        {content}
      </Sheet>
    )
  }

  return (
    <div
      className={`vch-linkpop ${stylex.props(channelStyles.linkpop).className}`}
      role="dialog"
      aria-label={tt.title(channel.name || t.channels.page.noName)}
      ref={panel}
      style={{ maxHeight: placement.maxHeight, ...(placement.up ? { top: 'auto', bottom: 'calc(100% + var(--vv-s-6))' } : null) }}
    >
      {content}
    </div>
  )
}
