// Tab Video nền của Thư viện (artboard Library › tab=backdrops): thanh thêm
// nguồn bằng link (parse → kênh / video lẻ / chưa hỗ trợ / sai link), cột
// SourceRail trái, trang kênh hoặc trang Video lẻ bên phải. Nguồn đang chọn
// giữ trong query ?nguon=&kenh= để tải lại và link "Đổi đoạn ở Thư viện" đúng.
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Banner, Icon, Spinner } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/vi'
import { navigate, useRoute } from '../../../router'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import {
  addBackdropChannel, addBackdropVideo, listBackdropChannels, listBackdropVideos, parseSource,
  requestPlatformSupport, type BackdropSegment,
} from '../../../api/backdrop'
import { listPlatforms, type Platform } from '../../../api/platforms'
import { ActionButton } from '../../../components'
import { ChannelPage } from './ChannelPage'
import { SegmentPicker } from './SegmentPicker'
import { SourceRail } from './SourceRail'
import { VideosPage } from './VideosPage'
import './backdrops.css'

type AddState =
  | { step: 'idle' }
  | { step: 'busy' }
  | { step: 'segment'; url: string; platform: string }
  | { step: 'addedChannel' }
  | { step: 'addedVideo' }
  | { step: 'unsupported'; host: string; label: string }
  | { step: 'invalid'; message: string; recovery?: string }
  | { step: 'error'; message: string }

function AddBar({ onAdded }: { onAdded: (id: string) => void }) {
  const copy = t.backdrop.tab
  const [input, setInput] = useState('')
  const [state, setState] = useState<AddState>({ step: 'idle' })
  const [requested, setRequested] = useState(false)
  const [segment, setSegment] = useState<BackdropSegment>({ fromSec: 0, toSec: 60 })
  const [videoSec, setVideoSec] = useState<number | null>(null)
  const [askBusy, setAskBusy] = useState(false)
  const alive = useRef(true)
  useEffect(() => {
    alive.current = true
    return () => { alive.current = false }
  }, [])

  const reset = () => { setState({ step: 'idle' }); setRequested(false) }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const link = input.trim()
    if (!link || state.step === 'busy') return
    setState({ step: 'busy' })
    setRequested(false)
    try {
      const parsed = await parseSource(link)
      if (!alive.current) return
      if (parsed.kind === 'channel') {
        const created = await addBackdropChannel(parsed.url ?? link)
        if (!alive.current) return
        setState({ step: 'addedChannel' })
        void invalidate(keys.backdropChannels())
        onAdded(created.id)
      } else if (parsed.kind === 'video') {
        const dur = parsed.durationSec ?? null
        setVideoSec(dur)
        setSegment({ fromSec: 0, toSec: Math.min(600, dur ?? 60) })
        setState({ step: 'segment', url: parsed.url ?? link, platform: parsed.platform })
      } else if (parsed.kind === 'unsupported') {
        setState({ step: 'unsupported', host: parsed.host, label: parsed.label })
      } else {
        setState({ step: 'invalid', message: parsed.message ?? copy.invalidLink, recovery: parsed.recovery })
      }
    } catch (err) {
      if (alive.current) setState({ step: 'error', message: err instanceof Error ? err.message : String(err) })
    }
  }

  const addVideo = async () => {
    if (state.step !== 'segment') return
    const url = state.url
    setState({ step: 'busy' })
    try {
      await addBackdropVideo(url, segment)
      if (!alive.current) return
      setState({ step: 'addedVideo' })
      void invalidate(keys.backdropVideos())
      onAdded('videos')
    } catch (err) {
      if (alive.current) setState({ step: 'error', message: err instanceof Error ? err.message : String(err) })
    }
  }

  // "Gửi yêu cầu hỗ trợ" gộp một lần mỗi lần nhập link (server cũng gộp theo host).
  const ask = async () => {
    if (state.step !== 'unsupported' || requested || askBusy) return
    setAskBusy(true)
    try {
      await requestPlatformSupport(state.host)
      if (alive.current) setRequested(true)
    } catch (err) {
      if (alive.current) setState({ step: 'error', message: err instanceof Error ? err.message : String(err) })
    } finally {
      if (alive.current) setAskBusy(false)
    }
  }

  const busy = state.step === 'busy'
  const failed = state.step === 'unsupported' || state.step === 'invalid' || state.step === 'error'
  return (
    <div className="vbd-add">
      <form className="vbd-addbar" data-failed={failed || undefined} onSubmit={(event) => void submit(event)}>
        <Icon name="link" size={16} className="vbd-addic" />
        <label htmlFor="vbd-link" className="sr-only">{copy.linkLabel}</label>
        <input
          id="vbd-link"
          className="vbd-addinput"
          type="text"
          value={input}
          placeholder={copy.linkPlaceholder}
          aria-invalid={failed || undefined}
          aria-describedby="vbd-addhint"
          disabled={busy}
          onChange={(event) => { setInput(event.currentTarget.value); if (state.step !== 'idle' && state.step !== 'segment') reset() }}
        />
        <ActionButton variant="primary" type="submit" pending={busy} disabled={!input.trim()}>
          {busy ? copy.addingChannel : copy.add}
        </ActionButton>
      </form>

      {state.step === 'segment' && (
        <div className="vbd-addseg">
          <SegmentPicker value={segment} videoSec={videoSec} onChange={setSegment} />
          <div className="vbd-addseg-actions">
            <ActionButton variant="primary" pending={busy} onClick={() => void addVideo()}>{copy.add}</ActionButton>
            <ActionButton variant="quiet" onClick={reset}>{t.library.cancel}</ActionButton>
          </div>
        </div>
      )}

      <span id="vbd-addhint" className="vbd-addhint">
        <Icon name={state.step === 'addedChannel' || state.step === 'addedVideo' ? 'tick-b' : 'info'} size={13}
          className={state.step === 'addedChannel' || state.step === 'addedVideo' ? 'vbd-ok-ic' : undefined} />
        {state.step === 'addedChannel' ? copy.hintChannelOk
          : state.step === 'addedVideo' ? copy.hintVideoOk
          : copy.hint}
      </span>

      {(state.step === 'invalid' || state.step === 'error') && (
        <p className="vbd-adderr" role="alert">
          <Icon name="danger" size={14} />
          {state.step === 'invalid' ? (state.recovery ? `${state.message} ${state.recovery}` : state.message) : state.message}
        </p>
      )}
      {state.step === 'unsupported' && (
        <p className="vbd-adderr" role="alert">
          <Icon name="danger" size={14} />
          <span className="vbd-adderr-text">{copy.unsupported(state.label)}</span>
          <ActionButton variant="quiet" size="sm" pending={askBusy} disabled={requested}
            onClick={() => void ask()}>
            {requested ? copy.supportSent : copy.askSupport(state.label)}
          </ActionButton>
        </p>
      )}
    </div>
  )
}

export function BackdropsTab({ viewport }: {
  /** kích thước ép cho test (happy-dom không layout) */
  viewport?: { width: number; height: number }
}) {
  const copy = t.backdrop.tab
  const { route } = useRoute()
  const channels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))
  const videos = useResource(keys.backdropVideos(), ({ signal }) => listBackdropVideos(signal))
  const platforms = useResource(keys.platforms(), ({ signal }) => listPlatforms(signal))

  const channelList = channels.data ?? []
  const videoList = videos.data ?? []

  const selected = route.name === 'library'
    ? route.nguon === 'videos' ? 'videos' : (route.kenh ?? channelList[0]?.id ?? '')
    : ''
  const select = (id: string) => {
    navigate(`/library?tab=backdrops&${id === 'videos' ? 'nguon=videos' : `nguon=channels&kenh=${encodeURIComponent(id)}`}`)
  }

  const channel = channelList.find((item) => item.id === selected)
  const platform: Platform | undefined = channel
    ? platforms.data?.find((item) => item.id === channel.platform)
    : undefined

  const loading = channels.initialLoading || videos.initialLoading
  const queryError = channels.error ?? videos.error

  return (
    <section className="vbd-tab" aria-label={copy.aria}>
      <AddBar onAdded={(id) => select(id)} />
      {!!queryError && <Banner tone="error" title={t.library.loadFail} action={{ label: t.library.retry, onClick: () => void invalidate(['backdrop']) }} />}
      <div className="vbd-cols">
        <SourceRail
          channels={channelList}
          videos={videoList}
          selected={selected || (channelList[0]?.id ?? 'videos')}
          onSelect={select}
        />
        {loading ? (
          <div className="vbd-pane vbd-pane--load" aria-busy="true"><Spinner size={16} color="var(--color-fg-3)" /></div>
        ) : channel ? (
          <ChannelPage channel={channel} platform={platform} viewport={viewport} />
        ) : selected === 'videos' || channelList.length === 0 ? (
          videoList.length > 0 ? (
            <VideosPage videos={videoList} loading={videos.initialLoading} error={videos.error} />
          ) : (
            <div className="vbd-pane vbd-pane--empty">
              <p className="vbd-empty-title">{copy.emptyTitle}</p>
              <p className="vbd-hint">{copy.emptyHint}</p>
            </div>
          )
        ) : null}
      </div>
    </section>
  )
}
