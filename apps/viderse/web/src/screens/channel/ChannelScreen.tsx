import * as stylex from '@stylexjs/stylex'
import { channelStyles, channelRootMarker } from './channel.stylex'
// Trang Kênh '#/channels/<id>?tab=videos|sources|templates|settings' (artboard
// Channel): cột trái "Kênh và thư viện", đầu kênh (avatar nhận diện, ngôn ngữ,
// tên, hành động), tab dưới gạch 38 px rồi nội dung tab, cột phải "Đang áp
// dụng cho kênh" + "Lịch đăng". Điện thoại (artboard MChannel): không cột trái,
// tab nhãn ngắn một hàng; mọi số hiện ra đều đọc từ API kênh.
import { useEffect, useState } from 'react'
import { Icon, Spinner, Tabs } from '@noverse/ui'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { t } from '../../i18n/channels'
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
import { mch } from './mobile/mchannel.stylex'
import { SourcesTab } from './SourcesTab'
import { TemplatesTab } from './TemplatesTab'
import { SettingsTab } from './SettingsTab'

const TABS: ChannelTab[] = ['videos', 'sources', 'templates', 'settings']

export function ChannelScreen({ route }: { route: Extract<Route, { name: 'channel' }> }) {
  const tt = t.channels
  const tab: ChannelTab = route.tab && TABS.includes(route.tab) ? route.tab : 'videos'
  const narrow = useMediaQuery('(max-width: 767px)')
  const tabLabels = narrow ? tt.page.tabsShort : tt.page.tabs
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
            <ActionButton size="lg" onClick={() => void channel.refresh()}>{t.common.retry}</ActionButton>
            <ActionButton size="lg" variant="primary" onClick={() => navigate(paths.workbench())}>{tt.page.backToChannels}</ActionButton>
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
      <main className={`vch-center vv-wide ${stylex.props(channelStyles.center).className}`} aria-label={tt.page.title}>
        <ChannelHeader channel={data} />
        <VideoStats channel={data} jobs={jobs} />
        <div {...stylex.props(channelStyles.tabRegion)}>
        <Tabs
          variant="underline"
          label={tt.page.tablist}
          value={tab}
          onChange={(id) => pickTab(id as ChannelTab)}
          tabs={{ id: 'channel-tabs', panel: 'channel-panel' }}
          items={TABS.map((id) => ({
            id, label: <>{tabLabels[id]}{counts[id] && <span className={`num ${stylex.props(channelStyles.tabCount, tab === id && channelStyles.tabCountOn).className}`}>{counts[id]}</span>}</>,
          }))}
        />
        </div>

        <div id="channel-panel" role="tabpanel" aria-labelledby={`channel-tabs-${tab}`} {...stylex.props(channelStyles.tabRegion)}>
        {tab === 'videos' && <VideosTab channel={data} rival={referenceRival} jobs={jobs} dockSlot={dockSlot} onSelecting={setSelecting} />}
        {tab === 'sources' && <SourcesTab channel={data} channels={list} />}
        {tab === 'templates' && <TemplatesTab channel={data} />}
        {tab === 'settings' && <SettingsTab channel={data} />}
        </div>
        <div ref={setDockSlot} {...stylex.props(narrow ? mch.composerHidden : videoStyles.dock)}>
          {!selecting && <AssistantComposer scope={{ channelId: data.id }} label={assistantCopy.composer.channel} placeholder={assistantCopy.composer.channelPlaceholder} recipient="scout" />}
        </div>
      </main>
      <ChannelAside channel={data} />
    </div>
  )
}

export default ChannelScreen
