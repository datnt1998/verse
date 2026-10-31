// Tab Nguồn & tự động (artboard Channel › tab=sources): danh sách đối thủ đã
// liên kết (avatar, huy hiệu "Tham chiếu nhận diện"), dòng phụ đúng artboard
// (`1,24 Tr người theo dõi · 3 video/tuần · đang quét lịch sử 1.284/2.000` +
// chip "Đang quét lịch sử"), hộp chọn đối thủ là popover neo dưới nút như
// artboard (role=dialog, role=checkbox rows, Esc đóng, focus trả về nút mở),
// ô theo dõi đối thủ mới rồi liên kết luôn, công tắc Tự lấy + nhịp 1–50 lưu
// ngay khi đổi. Bỏ liên kết hỏi xác nhận cam nhạt (copy sheet MChannel); bỏ
// đối thủ cuối → server trả autoPickPaused, hiện lý do tạm dừng.
// Mức tự chủ/Ưu tiên (§20) không dựng — không vẽ khối giả.
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Icon, confirm, Switch, toast, useEscapeLayer } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar } from '../../components'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { keys } from '../../data/keys'
import { listRivals, getRivalScan, getPostingSlots, addRival, type Rival } from '../../api/rivals'
import { useResource } from '../../data/useResource'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { linkSource, patchChannel, unlinkSource, type Channel, type ChannelSource } from '../../api/channels'
import { addRivalError } from '../explore/AddRivalForm'
import { channelTint } from '../../lib/channelTint'
import { PaceStepper } from './PaceStepper'
import { ScheduleCard } from './ScheduleCard'
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
  const rivalsRes = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const rivals = rivalsRes.data?.rivals ?? []
  const [pickerOpen, setPickerOpen] = useState(false)
  const [paused, setPaused] = useState(!channel.autoPick || channel.sources.length === 0)
  const [error, setError] = useState<string | null>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  useEffect(() => { setPaused(!channel.autoPick || channel.sources.length === 0) }, [channel.autoPick, channel.sources.length])

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
        const result = await unlinkSource(channel.id, rivalId)
        if (result.autoPickPaused) setPaused(true)
        void invalidate(keys.channels())
      },
    })
  }

  const linkedElsewhere = (rivalId: string) =>
    channels.filter((other) => other.id !== channel.id && other.sources.some((source) => source.rivalId === rivalId))

  return (
    <section className="vch-sources" aria-label={t.channels.page.tabs.sources}>
      <div className="vch-card vch-card--flush">
        <div className="vch-cardhead">
          <h2><Icon name="radar" size={16} />{tt.rivalsTitle} <span className="vch-row__count num">{channel.sources.length}</span></h2>
          <div className="vch-linkslot">
            <button
              ref={triggerRef}
              type="button"
              className="vch-linkbtn"
              aria-haspopup="dialog"
              aria-expanded={pickerOpen}
              onClick={() => (pickerOpen ? closePicker() : setPickerOpen(true))}
            >
              <Icon name="add" size={14} />
              {tt.linkBtn}
            </button>
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
          <div className="vch-empty" style={{ margin: '0 var(--vv-s-14) var(--vv-s-12)' }} role="status">
            <b>{tt.emptyTitle}</b>
            <span className="vch-hint">{tt.emptyBody}</span>
            <ActionButton size="sm" onClick={() => setPickerOpen(true)}>{tt.linkBtn}</ActionButton>
          </div>
        ) : channel.sources.map((source) => (
          <SourceRow
            key={source.rivalId}
            source={source}
            rival={rivalOf(source.rivalId)}
            onUnlink={() => askUnlink(source.rivalId)}
          />
        ))}
      </div>

      <div className="vch-card vch-card--flush">
        <div className="vch-cardhead">
          <h2><Icon name="refresh" size={16} />{tt.pacingTitle}</h2>
        </div>
        <p className="vch-hint" style={{ padding: '0 var(--vv-s-14)' }}>{tt.pacingHint}</p>
        {paused && (
          <p className="vch-alert" role="status" style={{ margin: 'var(--vv-s-8) var(--vv-s-14) 0' }}>
            {channel.sources.length === 0 ? tt.emptyBody : tt.pausedReason}
          </p>
        )}
        <div className="vch-settingrow">
          <span className="vch-settingrow__text">
            <span className="vch-settingrow__label">{tt.autoPick}</span>
            <span className="vch-settingrow__sub">{tt.autoPickHint}</span>
          </span>
          <Switch
            checked={channel.autoPick}
            onChange={(on) => { setPaused(!on || channel.sources.length === 0); void save({ autoPick: on }) }}
            label={tt.autoPick}
            size="sm"
          />
        </div>
        <div className="vch-settingrow">
          <span className="vch-settingrow__text">
            <span className="vch-settingrow__label">{tt.pace}</span>
            <span className="vch-settingrow__sub">{tt.paceRange} · {tt.paceUnit}</span>
          </span>
          <PaceStepper value={channel.pacePerDay} onSave={(pacePerDay) => void save({ pacePerDay })} />
        </div>
        {error && <p className="vch-field__error" role="alert" style={{ padding: 'var(--vv-s-8) var(--vv-s-14)' }}>{error}</p>}
        <div style={{ padding: 'var(--vv-s-8) var(--vv-s-14) var(--vv-s-12)' }}>
          <AddRivalRow
            channelId={channel.id}
            onLinked={() => { void invalidate(keys.channels()); void invalidate(keys.rivals()) }}
          />
        </div>
      </div>

      {narrow && <ScheduleCard channel={channel} variant="tab" />}
    </section>
  )
}

/** Một đối thủ đã liên kết: quét + lịch đăng đọc riêng hàng này (cache theo id). */
function SourceRow({ source, rival, onUnlink }: {
  source: ChannelSource
  rival: Rival | undefined
  onUnlink: () => void
}) {
  const tt = t.channels.sources
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

  return (
    <div className="vch-rival">
      <ChannelAvatar
        name={source.title}
        src={rival?.avatarUrl ?? undefined}
        color={{ bg: channelTint(source.rivalId) }}
        size={32}
        style={{ fontSize: 11, fontWeight: 700 }}
      />
      <div className="vch-rival__text">
        <span className="vch-rival__name">
          {source.title}
          {source.reference && <span className="vch-refbadge">{tt.linkedBadge}</span>}
          {scanning && <span className="vch-chip vch-chip--warn">{tt.scanChip}</span>}
        </span>
        <span className="vch-rival__meta">{meta}</span>
        <div className="vch-rival__actions">
          <ActionButton size="sm" lockedReason={t.channels.queueLock} icon={<Icon name="video-add" size={14} />}>
            {tt.makeLatest}
          </ActionButton>
          <a className="vch-chip" href={`#${paths.explore({ rival: rival?.handle ?? source.rivalId })}`}>
            {tt.topVideos}
          </a>
          <a className="vch-chip" href={`#${paths.explore({ rival: rival?.handle ?? source.rivalId })}`}>
            {tt.openExplore}
          </a>
        </div>
      </div>
      <div className="vch-rival__actions">
        <button
          type="button"
          className="vch-chip"
          title={tt.unlinkTip}
          onClick={onUnlink}
          style={{ color: 'var(--color-fg-3)' }}
        >
          {tt.unlink}
        </button>
      </div>
    </div>
  )
}

/** Theo dõi đối thủ mới rồi liên kết luôn vào kênh (W2: "Theo dõi & chọn"). */
function AddRivalRow({ channelId, onLinked }: { channelId: string; onLinked: () => void }) {
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
      style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 'var(--vv-s-8)' }}
    >
      <span className="sr-only"><label htmlFor="vch-add-rival">{tt.label}</label></span>
      <input
        id="vch-add-rival"
        className="vch-taginput"
        style={{ flex: '1 1 220px', width: 'auto', height: 'var(--vv-s-34)' }}
        value={value}
        placeholder={tt.placeholder}
        onChange={(event) => setValue(event.target.value)}
      />
      <ActionButton size="sm" type="submit" disabled={pending || !value.trim()} pending={pending}>
        {tt.submit}
      </ActionButton>
      {error && <span className="vch-field__error" role="alert" style={{ flexBasis: '100%' }}>{error}</span>}
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
      const nav = window.matchMedia('(max-width: 700px)').matches ? 64 : 0
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
      className="vch-linkpop"
      role="dialog"
      aria-label={tt.title(channel.name || t.channels.page.noName)}
      ref={panel}
      style={{ maxHeight: placement.maxHeight, ...(placement.up ? { top: 'auto', bottom: 'calc(100% + var(--vv-s-6))' } : null) }}
    >
      <div className="vch-linkpop__head">
        <span className="vch-linkpop__title">{tt.title(channel.name || t.channels.page.noName)}</span>
        <span className="vch-linkpop__hint">{t.channels.sources.linkHint}</span>
      </div>
      {rivals.length === 0 ? (
        <p className="vch-hint" style={{ padding: '0 var(--vv-s-10) var(--vv-s-6)' }}>{tt.empty}</p>
      ) : (
        <div role="group" aria-label={t.channels.sources.rivalsTitle} className="vch-pickerlist">
          {rivals.map((rival) => {
            const linked = inChannel.has(rival.id)
            const others = linkedElsewhere(rival.id)
            const blocked = linked || others.length > 0
            const reason = linked ? tt.alreadyLinked : others.length > 0 ? tt.linkedElsewhere(others[0].name || t.channels.page.noName) : ''
            return (
              <button
                key={rival.id}
                type="button"
                role="checkbox"
                aria-checked={picked.has(rival.id) || linked}
                aria-disabled={blocked || undefined}
                title={reason || undefined}
                className="vch-pickerrow"
                onClick={() => { if (!blocked) toggle(rival.id) }}
              >
                <span aria-hidden="true" className="vch-pickbox" data-on={picked.has(rival.id) || linked || undefined}>
                  {(picked.has(rival.id) || linked) && <Icon name="tick" size={12} />}
                </span>
                <ChannelAvatar
                  name={rival.title}
                  src={rival.avatarUrl ?? undefined}
                  color={{ bg: channelTint(rival.id) }}
                  size={28}
                  style={{ fontSize: 10.5 }}
                />
                <span className="vch-pickerrow__text">
                  <span className="vch-pickerrow__name">{rival.title}</span>
                  <span className="vch-pickerrow__sub">
                    {reason || (rival.subscriberCount != null
                      ? `${fmtSubs(rival.subscriberCount)} người theo dõi`
                      : rival.handle ?? '')}
                  </span>
                </span>
              </button>
            )
          })}
        </div>
      )}
      {error && <p className="vch-field__error" role="alert" style={{ padding: '0 var(--vv-s-10) var(--vv-s-6)' }}>{error}</p>}
      <div className="vch-linkpop__foot">
        <a href={`#${paths.explore()}`} className="vch-linkpop__more">
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
