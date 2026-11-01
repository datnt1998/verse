import * as stylex from '@stylexjs/stylex'
import { channelStyles, channelRootMarker } from './channel.stylex'
// Trang Kênh '#/channels/<id>?tab=videos|sources|templates|settings' (artboard
// Channel): cột trái "Kênh và thư viện", đầu kênh (avatar nhận diện, ngôn ngữ,
// tên, hành động), tab dưới gạch 38 px rồi nội dung tab, cột phải "Đang áp
// dụng cho kênh" + "Lịch đăng". Điện thoại (artboard MChannel): không cột trái,
// tab nhãn ngắn một hàng; mọi số hiện ra đều đọc từ API kênh.
import { useEffect, useState } from 'react'
import { Icon, Spinner } from '@noverse/ui'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { t } from '../../i18n/vi'
import { navigate, paths, type ChannelTab, type Route } from '../../router'
import { ActionButton } from '../../components'
import { ScreenEmpty } from '../../components'
import { getChannel, listChannels } from '../../api/channels'
import { listRivals } from '../../api/rivals'
import { listTemplates, listPrompts, listOverlays } from '../../api/library'
import { listBackdropChannels, listBackdropVideos } from '../../api/backdrop'
import { getChannelJobs } from '../../api/channelVideos'
import { AssistantComposer } from '../../components/assistant/AssistantComposer'
import { assistantCopy } from '../../i18n/assistant'
import { rememberChannel } from '../../lib/lastChannel'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { ChannelSideRail } from './ChannelSideRail'
import { ChannelAside } from './ChannelAside'
import { ChannelHeader } from './ChannelHeader'
import { VideosTab } from './VideosTab'
import { VideoStats } from './VideoStats'
import { videoStyles } from './videos.stylex'
import { SourcesTab } from './SourcesTab'
import { TemplatesTab } from './TemplatesTab'
import { SettingsTab } from './SettingsTab'

const TABS: ChannelTab[] = ['videos', 'sources', 'templates', 'settings']

export function ChannelScreen({ route }: { route: Extract<Route, { name: 'channel' }> }) {
  const tt = t.channels
  const tab: ChannelTab = route.tab && TABS.includes(route.tab) ? route.tab : 'videos'
  const tabLabels = useMediaQuery('(max-width: 767px)') ? tt.page.tabsShort : tt.page.tabs
  const channel = useResource(
    keys.channel(route.channelId),
    ({ signal }) => getChannel(route.channelId, signal),
    { keepPrevious: true },
  )
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const rivals = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const prompts = useResource(keys.prompts('', ''), ({ signal }) => listPrompts({}, signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const backdropChannels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))
  const backdropVideos = useResource(keys.backdropVideos(), ({ signal }) => listBackdropVideos(signal))
  const jobs = useResource(keys.channelJobs(route.channelId), ({ signal }) => getChannelJobs(route.channelId, signal), { keepPrevious: true })
  const [selecting, setSelecting] = useState(false)
  const [dockSlot, setDockSlot] = useState<HTMLDivElement | null>(null)

  const list = channels.data?.channels ?? []
  const data = channel.data

  useEffect(() => { if (data) rememberChannel(data.id) }, [data])

  if (channel.initialLoading && !data) {
    return (
      <div aria-busy="true" style={{ flex: '1 1 auto', display: 'grid', placeItems: 'center' }}>
        <Spinner size={18} color="var(--color-fg-3)" />
      </div>
    )
  }

  if (!data) {
    return (
      <ScreenEmpty
        icon="channel"
        title={tt.page.notFound}
        hint={tt.page.notFoundHint}
        action={(
          <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
            <ActionButton className={stylex.props(channelStyles.touchTarget).className} onClick={() => void channel.refresh()}>{t.common.retry}</ActionButton>
            <ActionButton className={stylex.props(channelStyles.touchTarget).className} variant="primary" onClick={() => navigate(paths.workbench())}>{tt.page.backToChannels}</ActionButton>
          </div>
        )}
      />
    )
  }

  const referenceId = data.sources.find((source) => source.reference)?.rivalId ?? data.sources[0]?.rivalId
  const referenceRival = rivals.data?.rivals.find((rival) => rival.id === referenceId)

  const counts: Record<ChannelTab, string> = {
    videos: jobs.data ? String(jobs.data.jobs.length) : '',
    sources: String(data.sources.length),
    templates: String(data.templates.length),
    settings: '',
  }

  const pickTab = (next: ChannelTab) => navigate(paths.channel(data.id, next), true)

  return (
    <div className={`vch-shell ${stylex.props(channelStyles.shell, channelRootMarker).className}`}>
      <ChannelSideRail
        channels={list}
        currentId={data.id}
        tab={tab}
        counts={{
          templates: templates.data?.items.length,
          prompts: prompts.data?.items.length,
          overlays: overlays.data?.items.length,
          // Video nền = kênh nền + video lẻ, đúng con số trong mục "Thư viện dùng chung".
          backdrops: backdropChannels.data && backdropVideos.data
            ? backdropChannels.data.length + backdropVideos.data.length
            : undefined,
        }}
      />
      <main className={`vch-center ${stylex.props(channelStyles.center).className}`} aria-label={tt.page.title}>
        <ChannelHeader channel={data} />
        <VideoStats channel={data} jobs={jobs} />
        <div
          role="tablist"
          aria-label={tt.page.tablist}
          className={`vch-tabs ${stylex.props(channelStyles.tabs).className}`}
          onKeyDown={(event) => {
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
            event.preventDefault()
            const index = TABS.indexOf(tab)
            const next = event.key === 'Home' ? TABS[0]
              : event.key === 'End' ? TABS[TABS.length - 1]
                : TABS[(index + (event.key === 'ArrowRight' ? 1 : TABS.length - 1)) % TABS.length]
            pickTab(next)
            event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]')[TABS.indexOf(next)]?.focus()
          }}
        >
          {TABS.map((id) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              tabIndex={tab === id ? 0 : -1}
              className={`vch-tab ${stylex.props(channelStyles.tab).className}`}
              onClick={() => pickTab(id)}
            >
              {tabLabels[id]}
              {counts[id] && <span className={`vch-tab__count num ${stylex.props(channelStyles.tabCount).className}`}>{counts[id]}</span>}
            </button>
          ))}
        </div>

        {tab === 'videos' && <VideosTab channel={data} rival={referenceRival} jobs={jobs} dockSlot={dockSlot} onSelecting={setSelecting} />}
        {tab === 'sources' && <SourcesTab channel={data} channels={list} />}
        {tab === 'templates' && <TemplatesTab channel={data} />}
        {tab === 'settings' && <SettingsTab channel={data} />}
        <div ref={setDockSlot} {...stylex.props(videoStyles.dock)}>
          {!selecting && <AssistantComposer scope={{ channelId: data.id }} label={assistantCopy.composer.channel} placeholder={assistantCopy.composer.channelPlaceholder} recipient="scout" />}
        </div>
      </main>
      <ChannelAside channel={data} />
    </div>
  )
}

export default ChannelScreen
