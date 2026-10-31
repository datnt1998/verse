import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
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
    <div className={`vbd-add ${stylex.props(sx.add).className}`}>
      <form className={`vbd-addbar ${stylex.props(sx.addbar, (Boolean((failed))) && sx.addbarDataFailed).className}`} data-failed={failed || undefined} onSubmit={(event) => void submit(event)}>
        <Icon xstyle={[sx.addic]} name="link" size={16} className="vbd-addic" />
        <label htmlFor="vbd-link" className="sr-only">{copy.linkLabel}</label>
        <input
          id="vbd-link"
          className={`vbd-addinput ${stylex.props(sx.addinput).className}`}
          type="text"
          value={input}
          placeholder={copy.linkPlaceholder}
          aria-invalid={failed || undefined}
          aria-describedby="vbd-addhint"
          disabled={busy}
          onChange={(event) => { setInput(event.currentTarget.value); if (state.step !== 'idle' && state.step !== 'segment') reset() }}
        />
        <ActionButton xstyle={[sx.vbdAddbarActionButton]} variant="primary" type="submit" pending={busy} disabled={!input.trim()}>
          {busy ? copy.addingChannel : copy.add}
        </ActionButton>
      </form>

      {state.step === 'segment' && (
        <div className={`vbd-addseg ${stylex.props(sx.addseg).className}`}>
          <SegmentPicker value={segment} videoSec={videoSec} onChange={setSegment} />
          <div className={`vbd-addseg-actions ${stylex.props(sx.addsegActions).className}`}>
            <ActionButton xstyle={[sx.vbdAddbarActionButton]} variant="primary" pending={busy} onClick={() => void addVideo()}>{copy.add}</ActionButton>
            <ActionButton xstyle={[sx.vbdAddbarActionButton]} variant="quiet" onClick={reset}>{t.library.cancel}</ActionButton>
          </div>
        </div>
      )}

      <span id="vbd-addhint" className={`vbd-addhint ${stylex.props(sx.addhint).className}`}>
        <Icon xstyle={[(state.step === 'addedChannel' || state.step === 'addedVideo') && sx.vbdAddhintIconVbdOkIc]} name={state.step === 'addedChannel' || state.step === 'addedVideo' ? 'tick-b' : 'info'} size={13}
           />
        {state.step === 'addedChannel' ? copy.hintChannelOk
          : state.step === 'addedVideo' ? copy.hintVideoOk
          : copy.hint}
      </span>

      {(state.step === 'invalid' || state.step === 'error') && (
        <p className={`vbd-adderr ${stylex.props(sx.adderr).className}`} role="alert">
          <Icon name="danger" size={14} />
          {state.step === 'invalid' ? (state.recovery ? `${state.message} ${state.recovery}` : state.message) : state.message}
        </p>
      )}
      {state.step === 'unsupported' && (
        <p className={`vbd-adderr ${stylex.props(sx.adderr).className}`} role="alert">
          <Icon name="danger" size={14} />
          <span className={`vbd-adderr-text ${stylex.props(sx.adderrText).className}`}>{copy.unsupported(state.label)}</span>
          <ActionButton xstyle={[sx.vbdAddbarActionButton]} variant="quiet" size="sm" pending={askBusy} disabled={requested}
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
    <section className={`vbd-tab ${stylex.props(sx.tab).className}`} aria-label={copy.aria}>
      <AddBar onAdded={(id) => select(id)} />
      {!!queryError && <Banner tone="error" title={t.library.loadFail} action={{ label: t.library.retry, onClick: () => void invalidate(['backdrop']) }} />}
      <div className={`vbd-cols ${stylex.props(sx.cols).className}`}>
        <SourceRail
          channels={channelList}
          videos={videoList}
          selected={selected || (channelList[0]?.id ?? 'videos')}
          onSelect={select}
        />
        {loading ? (
          <div className={`vbd-pane vbd-pane--load ${stylex.props(sx.panePaneLoad).className}`} aria-busy="true"><Spinner size={16} color="var(--color-fg-3)" /></div>
        ) : channel ? (
          <ChannelPage channel={channel} platform={platform} viewport={viewport} />
        ) : selected === 'videos' || channelList.length === 0 ? (
          videoList.length > 0 ? (
            <VideosPage videos={videoList} loading={videos.initialLoading} error={videos.error} />
          ) : (
            <div className={`vbd-pane vbd-pane--empty ${stylex.props(sx.panePaneEmpty).className}`}>
              <p className={`vbd-empty-title ${stylex.props(sx.emptyTitle).className}`}>{copy.emptyTitle}</p>
              <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.emptyHint}</p>
            </div>
          )
        ) : null}
      </div>
    </section>
  )
}

const sx = stylex.create({
  add: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  addbar: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":focus-within": color["--color-accent"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":focus-within": color["--color-accent"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":focus-within": color["--color-accent"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":focus-within": color["--color-accent"] }, borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-5)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-5)", paddingLeft: "var(--vv-s-12)", backgroundColor: color["--color-bg"], backgroundImage: "none" },
  addbarDataFailed: { borderTopColor: color["--color-err"], borderRightColor: color["--color-err"], borderBottomColor: color["--color-err"], borderLeftColor: color["--color-err"] },
  addic: { color: color["--color-fg-4"], flexShrink: 0 },
  addinput: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", outlineWidth: 0, outlineStyle: "none", outlineColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", fontSize: "var(--vv-s-12)", color: { default: color["--color-fg"], "::placeholder": color["--color-fg-4"] }, height: "var(--vv-s-28)", alignSelf: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "stretch" }, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  vbdAddbarActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  addseg: { borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-14)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  addsegActions: { display: "flex", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", justifyContent: "flex-end" },
  addhint: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-11)", color: color["--color-fg-3"], lineHeight: libraryVars.bodyLineHeight },
  vbdAddhintIconVbdOkIc: { color: color["--color-ok"] },
  adderr: { display: "flex", alignItems: "center", flexWrap: "wrap", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", fontSize: "var(--vv-s-11)", color: color["--color-err"], marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, lineHeight: libraryVars.bodyLineHeight },
  adderrText: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: libraryVars.backdropsBackdropsTabAdderrTextMinWidth },
  tab: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-14)", columnGap: "var(--vv-s-14)", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minHeight: 0 },
  cols: { display: "grid", gridTemplateColumns: { default: libraryVars.backdropsBackdropsTabColsGridTemplateColumns, "@media (max-width: 700px)": libraryVars.templateBackdropPickerDlgBodyGridTemplateColumns2 }, gridTemplateRows: { default: libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3, "@media (max-width: 700px)": libraryVars.backdropsBackdropsTabColsGridTemplateRows2 }, rowGap: "var(--vv-s-14)", columnGap: "var(--vv-s-14)", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minHeight: 0, alignItems: "stretch" },
  panePaneLoad: { borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-16)", paddingRight: "var(--vv-s-18)", paddingBottom: "var(--vv-s-18)", paddingLeft: "var(--vv-s-18)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-14)", columnGap: "var(--vv-s-14)", alignItems: "center", justifyContent: "center", minHeight: "var(--vv-s-240)" },
  panePaneEmpty: { borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-16)", paddingRight: "var(--vv-s-18)", paddingBottom: "var(--vv-s-18)", paddingLeft: "var(--vv-s-18)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", alignItems: "center", justifyContent: "center", textAlign: "center", minHeight: "var(--vv-s-240)" },
  emptyTitle: { fontSize: "var(--vv-s-14)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"], marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0 },
  hint: { fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, display: "flex", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", alignItems: "flex-start" },
})
