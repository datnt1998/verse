import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { channelStyles } from './channel.stylex'
// Tab Nguồn & tự động (artboard Channel › tab=sources): danh sách đối thủ đã
// liên kết (avatar, huy hiệu "Tham chiếu nhận diện"), dòng phụ đúng artboard
// (`1,24 Tr người theo dõi · 3 video/tuần · đang quét lịch sử 1.284/2.000` +
// chip "Đang quét lịch sử"), hộp chọn đối thủ là popover neo dưới nút như
// artboard (role=dialog, role=checkbox rows, Esc đóng, focus trả về nút mở),
// ô theo dõi đối thủ mới rồi liên kết luôn, công tắc Tự lấy + nhịp 1–50 lưu
// ngay khi đổi. Bỏ liên kết hỏi xác nhận cam nhạt (copy sheet MChannel); bỏ
// đối thủ cuối → khối nhịp hiện lý do tạm dừng.
// Mức tự chủ + Ưu tiên kênh (§20) thay công tắc Tự lấy: AutonomyCard.
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { ActionSheet, Button, Checkbox, Icon, confirm, toast, useEscapeLayer } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/channels'
import { navigate, paths } from '../../router'
import { keys } from '../../data/keys'
import { listRivals, getRivalScan, getPostingSlots, addRival, type Rival } from '../../api/rivals'
import { useResource } from '../../data/useResource'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { linkSource, patchChannel, unlinkSource, type Channel, type ChannelSource } from '../../api/channels'
import { addRivalError } from '../explore/AddRivalForm'
import { channelTint } from '../../lib/channelTint'
import { PaceStepper } from './PaceStepper'
import { makeLatestRivalVideo } from './latest'
import { autonomyStyles } from '../../components/autonomy/autonomy.stylex'
import { Orb } from '../../components/Orb'
import { TodayRow } from '../../components/autonomy/TodayRow'
import { AutonomyCard, PrioritySeg } from '../../components/autonomy/AutonomyCard'
import { ScheduleCard } from './ScheduleCard'
import { AutonomyCardMobile, PrioritySegMobile } from './mobile/AutonomyMobile'
import { mch } from './mobile/mchannel.stylex'
import { ASIDE_HIDDEN } from './breakpoint'
import { fmtAge, fmtSubs } from '../explore/fmt'

const num = new Intl.NumberFormat('vi-VN')
const perWeekText = (n: number) => num.format(Math.round(n * 10) / 10)

export function SourcesTab({ channel, channels }: {
  channel: Channel
  /** mọi kênh — nhận ra đối thủ đang dùng cho kênh khác */
  channels: Channel[]
}) {
  const tt = t.channels.sources
  // Cột phải ẩn dưới 1181 px → khối Lịch đăng nằm trong tab này thay vì cột phải.
  const narrow = useMediaQuery(ASIDE_HIDDEN)
  const phone = useMediaQuery('(max-width: 767px)')
  const ta = t.autonomy
  const rivalsRes = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const rivals = rivalsRes.data?.rivals ?? []
  const [pickerOpen, setPickerOpen] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  const save = async (patch: Parameters<typeof patchChannel>[1]) => {
    setError(null)
    try {
      const next = await patchChannel(channel.id, patch)
      setData(keys.channel(channel.id), next)
      void invalidate(keys.channels())
    } catch (err) {
      setError(err instanceof Error ? err.message : tt.saveError)
    }
  }

  const rivalOf = (id: string): Rival | undefined => rivals.find((rival) => rival.id === id)

  const closePicker = () => {
    setPickerOpen(false)
    // Esc / bấm ngoài / liên kết xong: trả tiêu điểm về nút mở popover.
    requestAnimationFrame(() => triggerRef.current?.focus())
  }

  const askUnlink = (rivalId: string) => {
    const row = channel.sources.find((source) => source.rivalId === rivalId)
    if (!row) return
    const nextRef = channel.sources.find((source) => source.rivalId !== rivalId)
    void confirm({
      tone: 'dsoft',
      title: tt.unlinkTitle(row.title),
      body: (
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
          {tt.unlinkBody}
          {row.reference && nextRef ? tt.unlinkRef(nextRef.title) : ''}
        </p>
      ),
      confirmLabel: tt.unlinkOk,
      successMessage: tt.unlinkDone,
      onConfirm: async () => {
        await unlinkSource(channel.id, rivalId)
        void invalidate(keys.channels())
      },
    })
  }

  const linkedElsewhere = (rivalId: string) =>
    channels.filter((other) => other.id !== channel.id && other.sources.some((source) => source.rivalId === rivalId))

  if (phone) {
    return (
      <section {...stylex.props(mch.sources)} aria-label={t.channels.page.tabs.sources}>
        <section {...stylex.props(mch.card16)} aria-label={ta.pacing.title}>
          <div {...stylex.props(mch.headRow)}>
            <span {...stylex.props(mch.orbs)} aria-hidden="true"><Orb of="scout" size={22} /></span>
            <div {...stylex.props(mch.headText)}>
              <h2 {...stylex.props(mch.cardTitle)}>{ta.pacing.title}</h2>
              <span {...stylex.props(mch.cardSub)}>{ta.pacing.hint}</span>
              {channel.sources.length === 0 && <span {...stylex.props(mch.hint, mch.footNoteBlocked)}>{ta.pacing.paused}</span>}
            </div>
          </div>
          <div {...stylex.props(mch.paceBox)}>
            <span {...stylex.props(mch.paceLabel)}>{tt.pace}</span>
            <PaceStepper value={channel.pacePerDay} onSave={(pacePerDay) => void save({ pacePerDay })} />
          </div>
          <span {...stylex.props(mch.hint)}>{ta.pacing.note}</span>
          <PrioritySegMobile channel={channel} />
          <TodayRow channel={channel} rivals={rivals} />
          {error && <p role="alert" {...stylex.props(mch.error)}>{error}</p>}
        </section>

        <AutonomyCardMobile channel={channel} />

        <section {...stylex.props(mch.card16)} aria-label={tt.rivalsTitle}>
          <div {...stylex.props(mch.rivalHead)}>
            <span {...stylex.props(mch.rivalHeadTitle)}>{tt.rivalsTitle}</span>
            <span className={`num ${stylex.props(mch.rivalCount).className}`}>{channel.sources.length}</span>
            <span {...stylex.props(mch.spacer)} />
            <Button variant="quiet" size="lg" ref={triggerRef}
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
                anchor={triggerRef}
                onClose={closePicker}
              />
            )}
          </div>
          {channel.sources.length === 0 ? (
            <div role="status" {...stylex.props(mch.emptyBox)}>
              <div {...stylex.props(mch.emptyText)}>
                <b {...stylex.props(mch.emptyTitle)}>{tt.emptyTitle}</b>
                <span {...stylex.props(mch.emptyBody)}>{tt.emptyBody}</span>
              </div>
            </div>
          ) : channel.sources.map((source) => (
            <SourceRow
              key={source.rivalId}
              channelId={channel.id}
              variant="mobile"
              source={source}
              rival={rivalOf(source.rivalId)}
              onUnlink={() => askUnlink(source.rivalId)}
            />
          ))}
          <AddRivalRow
            mobile
            channelId={channel.id}
            onLinked={() => { void invalidate(keys.channels()); void invalidate(keys.rivals()) }}
          />
          <p {...stylex.props(mch.footNote)}>{t.channels.sources.linkHint}</p>
        </section>

        <ScheduleCard channel={channel} variant="mobile" />
      </section>
    )
  }

  return (
    <section className={`vch-sources ${stylex.props(channelStyles.sources).className}`} aria-label={t.channels.page.tabs.sources}>
      <div className={`vch-card vch-card--flush ${stylex.props(channelStyles.card, channelStyles.cardFlush).className}`}>
        <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`}>
          <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="radar" size={16} />{tt.rivalsTitle} <span className={`vch-row__count num ${stylex.props(channelStyles.rowCount).className}`}>{channel.sources.length}</span></h2>
          <div className={`vch-linkslot ${stylex.props(channelStyles.linkslot).className}`}>
            <Button variant="quiet" size="sm" ref={triggerRef}
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
                anchor={triggerRef}
                onClose={closePicker}
              />
            )}
          </div>
        </div>
        {channel.sources.length === 0 ? (
          <div className={`vch-empty ${stylex.props(channelStyles.empty).className}`} style={{ margin: '0 var(--vv-s-14) var(--vv-s-12)' }} role="status">
            <b {...stylex.props(channelStyles.emptyTitle)}>{tt.emptyTitle}</b>
            <span className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.emptyBody}</span>
            <ActionButton size="sm" onClick={() => setPickerOpen(true)}>{tt.linkBtn}</ActionButton>
          </div>
        ) : channel.sources.map((source) => (
          <SourceRow
            key={source.rivalId}
            channelId={channel.id}
            source={source}
            rival={rivalOf(source.rivalId)}
            onUnlink={() => askUnlink(source.rivalId)}
          />
        ))}
        <div style={{ padding: 'var(--vv-s-8) var(--vv-s-14) var(--vv-s-12)' }}>
          <AddRivalRow
            channelId={channel.id}
            onLinked={() => { void invalidate(keys.channels()); void invalidate(keys.rivals()) }}
          />
        </div>
      </div>

      <div {...stylex.props(autonomyStyles.rightCol)}>
        <section {...stylex.props(channelStyles.card, autonomyStyles.paceCard)} aria-label={ta.pacing.title}>
          <div {...stylex.props(autonomyStyles.head)}>
            <span {...stylex.props(autonomyStyles.orbs)} aria-hidden="true"><Orb of="scout" size={22} /></span>
            <div {...stylex.props(autonomyStyles.headText)}>
              <h2 {...stylex.props(autonomyStyles.title)}>{ta.pacing.title}</h2>
              <span {...stylex.props(autonomyStyles.sub)}>{ta.pacing.hint}</span>
              {channel.sources.length === 0 && <span {...stylex.props(autonomyStyles.paused)}>{ta.pacing.paused}</span>}
            </div>
          </div>
          <div {...stylex.props(autonomyStyles.paceBlock)}>
            <span {...stylex.props(autonomyStyles.paceLabel)}>{tt.pace}</span>
            <PaceStepper value={channel.pacePerDay} onSave={(pacePerDay) => void save({ pacePerDay })} />
            <span {...stylex.props(autonomyStyles.paceUnit)}>{tt.paceUnit}</span>
            <span {...stylex.props(autonomyStyles.paceNote)}>{ta.pacing.note}</span>
          </div>
          <PrioritySeg channel={channel} />
          <TodayRow channel={channel} rivals={rivals} />
          {error && <p className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert">{error}</p>}
        </section>
        <AutonomyCard channel={channel} />
      </div>

      {narrow && <ScheduleCard channel={channel} variant="tab" />}
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
  const [making, setMaking] = useState(false)
  const scan = useResource(keys.rivalScan(source.rivalId), ({ signal }) => getRivalScan(source.rivalId, signal))
  const posting = useResource(keys.rivalPosting(source.rivalId, 'vn'), ({ signal }) => getPostingSlots(source.rivalId, 'vn', signal))
  // Dữ liệu quét có thể lỗi/không đúng hình (404 trong test, máy chưa quét) → chỉ
  // tin khi có `history`; thiếu thì rơi về số video đã lập chỉ mục của danh sách.
  const history = scan.data?.history
  const scanning = history?.state === 'queued' || history?.state === 'running'
  const indexed = scan.data?.indexedVideos ?? rival?.videoCount ?? 0
  const meta = [
    rival?.subscriberCount != null ? `${fmtSubs(rival.subscriberCount)} người theo dõi` : rival?.handle ?? source.handle,
    posting.data?.perWeek != null ? tt.perWeek(perWeekText(posting.data.perWeek)) : '',
    scanning && history
      ? tt.scanningHistory(indexed, scan.data?.historyCap ?? 0)
      : tt.scanned(num.format(indexed)),
    rival?.scannedAt ? tt.seen(fmtAge(rival.scannedAt)) : '',
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
          title={source.title}
          actions={[
            { label: tt.topVideos, icon: 'chart', onSelect: () => navigate(href.slice(1)) },
            { label: tt.openExplore, icon: 'global', onSelect: () => navigate(href.slice(1)) },
            { label: tt.unlink, icon: 'link', destructive: true, onSelect: onUnlink },
          ]}
        />
      </div>
    )
  }

  return (
    <div className={`vch-rival ${stylex.props(channelStyles.rival).className}`}>
      <ChannelAvatar
        name={source.title}
        src={rival?.avatarUrl ?? undefined}
        color={{ bg: channelTint(source.rivalId) }}
        size={32}
        style={{ fontSize: 11, fontWeight: 700 }}
      />
      <div className={`vch-rival__text ${stylex.props(channelStyles.rivalText).className}`}>
        <span className={`vch-rival__name ${stylex.props(channelStyles.rivalName).className}`}>
          {source.title}
          {source.reference && <span className={`vch-refbadge ${stylex.props(channelStyles.refbadge).className}`}>{tt.linkedBadge}</span>}
          {scanning && <span className={`vch-chip vch-chip--warn ${stylex.props(channelStyles.chip, channelStyles.chipWarn).className}`}>{tt.scanChip}</span>}
        </span>
        <span className={`vch-rival__meta ${stylex.props(channelStyles.rivalMeta).className}`}>{meta}</span>
        <div className={`vch-rival__actions ${stylex.props(channelStyles.rivalActions).className}`}>
          <ActionButton size="sm" pending={making} onClick={() => void makeLatest()} icon={<Icon name="video-add" size={14} />}>
            {tt.makeLatest}
          </ActionButton>
          <a href={`#${paths.explore({ rival: rival?.handle ?? source.rivalId })}`} {...stylex.props(channelStyles.appliedLink)}>{tt.topVideos}</a>
          <a href={`#${paths.explore({ rival: rival?.handle ?? source.rivalId })}`} {...stylex.props(channelStyles.appliedLink)}>{tt.openExplore}</a>
        </div>
      </div>
      <div className={`vch-rival__actions ${stylex.props(channelStyles.rivalActions).className}`}>
        <Button variant="quiet" size="sm" title={tt.unlinkTip} onClick={onUnlink}>{tt.unlink}</Button>
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

  const submit = async () => {
    const input = value.trim()
    if (!input || pending) return
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
        className={`vch-taginput ${stylex.props(mobile ? mch.input44 : channelStyles.taginput, mch.rivalInput, mobile && mch.rivalInputMobile).className}`}
        value={value}
        placeholder={tt.placeholder}
        onChange={(event) => setValue(event.target.value)}
      />
      <ActionButton size={mobile ? 'lg' : 'sm'} block={mobile} type="submit" disabled={pending || !value.trim()} pending={pending}>
        {tt.submit}
      </ActionButton>
      {error && <span className={`vch-field__error ${stylex.props(channelStyles.fieldError).className}`} role="alert" style={{ flexBasis: '100%' }}>{error}</span>}
    </form>
  )
}

/**
 * Popover chọn đối thủ neo dưới nút "Liên kết đối thủ" (artboard Channel
 * `linkOpen`): role=dialog, hàng role=checkbox, Esc đóng (lớp escape của kit),
 * bấm ngoài đóng, focus trả về nút mở ở `closePicker()` của cha.
 */
function LinkPickerPopover({ channel, rivals, linkedElsewhere, anchor, onClose }: {
  channel: Channel
  rivals: Rival[]
  linkedElsewhere: (rivalId: string) => Channel[]
  /** nút mở popover — đo chỗ trống để neo xuống hoặc lật lên, không bị thanh tab che */
  anchor: React.RefObject<HTMLButtonElement | null>
  onClose: () => void
}) {
  const tt = t.channels.sources.picker
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const panel = useRef<HTMLDivElement>(null)
  // Chỗ trống dưới nút (trừ thanh tab điện thoại) không đủ cho hộp thì lật lên
  // trên; luôn giới hạn chiều cao để nút "Liên kết" còn bấm được.
  const [placement, setPlacement] = useState<{ up: boolean; maxHeight: number }>({ up: false, maxHeight: 560 })
  const inChannel = new Set(channel.sources.map((source) => source.rivalId))

  useEscapeLayer(true, onClose)

  useLayoutEffect(() => {
    const trigger = anchor.current
    if (!trigger) return
    const measure = () => {
      const rect = trigger.getBoundingClientRect()
      const nav = window.matchMedia('(max-width: 767px)').matches ? 64 : 0
      const below = window.innerHeight - rect.bottom - nav - 16
      const above = rect.top - 16
      const up = below < 320 && above > below
      setPlacement({ up, maxHeight: Math.max(240, Math.min(560, up ? above : below)) })
    }
    measure()
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [anchor])

  useEffect(() => {
    // Mở ra là đặt tiêu điểm vào hàng chọn được đầu tiên (bàn phím đi tiếp bằng Tab).
    panel.current?.querySelector<HTMLElement>('[role="checkbox"]:not([aria-disabled="true"])')?.focus()
  }, [])

  useEffect(() => {
    const outside = (event: PointerEvent) => {
      const target = event.target as Node
      if (panel.current?.contains(target)) return
      onClose()
    }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [onClose])

  const toggle = (id: string) => setPicked((prev) => {
    const next = new Set(prev)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    return next
  })

  const link = async () => {
    if (picked.size === 0 || pending) return
    setPending(true)
    setError(null)
    try {
      for (const rivalId of picked) await linkSource(channel.id, rivalId)
      void invalidate(keys.channels())
      toast.ok(tt.done)
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : t.channels.sources.saveError)
      setPending(false)
    }
  }

  return (
    <div
      className={`vch-linkpop ${stylex.props(channelStyles.linkpop).className}`}
      role="dialog"
      aria-label={tt.title(channel.name || t.channels.page.noName)}
      ref={panel}
      style={{ maxHeight: placement.maxHeight, ...(placement.up ? { top: 'auto', bottom: 'calc(100% + var(--vv-s-6))' } : null) }}
    >
      <div className={`vch-linkpop__head ${stylex.props(channelStyles.linkpopHead).className}`}>
        <span className={`vch-linkpop__title ${stylex.props(channelStyles.linkpopTitle).className}`}>{tt.title(channel.name || t.channels.page.noName)}</span>
        <span className={`vch-linkpop__hint ${stylex.props(channelStyles.linkpopHint).className}`}>{t.channels.sources.linkHint}</span>
      </div>
      {rivals.length === 0 ? (
        <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`} style={{ padding: '0 var(--vv-s-10) var(--vv-s-6)' }}>{tt.empty}</p>
      ) : (
        <div role="group" aria-label={t.channels.sources.rivalsTitle} className={`vch-pickerlist ${stylex.props(channelStyles.pickerlist).className}`}>
          {rivals.map((rival) => {
            const linked = inChannel.has(rival.id)
            const others = linkedElsewhere(rival.id)
            const blocked = linked || others.length > 0
            const reason = linked ? tt.alreadyLinked : others.length > 0 ? tt.linkedElsewhere(others[0].name || t.channels.page.noName) : ''
            return (
              <Checkbox key={rival.id} checked={picked.has(rival.id) || linked}
                aria-disabled={blocked || undefined} title={reason || undefined}
                xstyle={channelStyles.pickerrow} onChange={() => { if (!blocked) toggle(rival.id) }}
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
                  <span className={`vch-pickerrow__sub ${stylex.props(channelStyles.pickerrowSub).className}`}>
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
      <div className={`vch-linkpop__foot ${stylex.props(channelStyles.linkpopFoot).className}`}>
        <a href={`#${paths.explore()}`} className={`vch-linkpop__more ${stylex.props(channelStyles.linkpopMore).className}`}>
          <Icon name="radar" size={13} />
          {tt.followNew}
        </a>
        <ActionButton
          variant="primary"
          size="sm"
          disabled={picked.size === 0}
          pending={pending}
          onClick={() => void link()}
        >
          {picked.size === 0 ? tt.none : tt.selected(picked.size)}
        </ActionButton>
      </div>
    </div>
  )
}
