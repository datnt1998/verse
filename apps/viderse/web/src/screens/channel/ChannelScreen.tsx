import * as stylex from '@stylexjs/stylex'
import { channelStyles, channelRootMarker } from './channel.stylex'
// Kênh dùng cùng dữ liệu và ba tab trên PC/điện thoại.
// Chỉ dẫn theo kênh ở dock PC và Sheet Nhập nhanh trong Hồ sơ điện thoại.
import { useEffect, useRef, useState } from 'react'
import { Button, Icon, Spinner, Tabs, Segmented, Sheet, toast } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { t } from '../../i18n/channels'
import { navigate, paths, type ChannelTab, type Route } from '../../router'
import { ActionButton } from '../../components'
import { ResponsiveSelect } from '../../components/responsive-overlays'
import { text } from '../../styles/shared.stylex'
import { ScreenEmpty } from '../../components'
import { getChannel, listChannels, patchChannel, type Channel } from '../../api/channels'
import { getChannelJobs } from '../../api/channelVideos'
import { rememberChannel } from '../../lib/lastChannel'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { Orb } from '../../components/Orb'
import { ChannelSideRail } from './ChannelSideRail'
import { ChannelHeader } from './ChannelHeader'
import { VideoStats } from './VideoStats'
import { SourcesTab } from './SourcesTab'
import { TemplatesTab } from './TemplatesTab'
import { SettingsTab } from './SettingsTab'

const TABS: ChannelTab[] = ['sources', 'templates', 'settings']

export function ChannelScreen({ route }: { route: Extract<Route, { name: 'channel' }> }) {
  const tt = t.channels
  const tab: ChannelTab = route.tab && TABS.includes(route.tab) ? route.tab : 'sources'
  const narrow = useMediaQuery('(max-width: 767px)')
  const [instructionsOpen, setInstructionsOpen] = useState(false)
  const tabLabels = narrow ? tt.page.tabsShort : tt.page.tabs
  const channel = useResource(
    keys.channel(route.channelId),
    ({ signal }) => getChannel(route.channelId, signal),
    { keepPrevious: true },
  )
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const jobs = useResource(keys.channelJobs(route.channelId), ({ signal }) => getChannelJobs(route.channelId, signal), { keepPrevious: true })

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

  const counts: Record<ChannelTab, string> = {
    sources: String(data.sources.length),
    templates: '',
    settings: '',
  }

  const pickTab = (next: ChannelTab) => navigate(paths.channel(data.id, next), true)
  const jobCount = jobs.data?.jobs.length ?? 0
  const note = jobs.data
    ? (jobCount === 0 ? (narrow ? tt.page.listNoteNoneM : tt.page.listNoteNone)
      : narrow ? tt.page.listNoteM(jobCount) : tt.page.listNote(jobCount))
    : ''

  return (
    <div className={`vch-shell ${stylex.props(channelStyles.shell, channelRootMarker).className}`}>
      <ChannelSideRail
        channels={list}
        currentId={data.id}
        tab={tab}
      />
      <main className={`vch-center vv-wide ${stylex.props(channelStyles.center).className}`} aria-label={tt.page.title}>
        <ChannelHeader channel={data} />
        <VideoStats channel={data} jobs={jobs} />
        {jobs.data && (narrow ? (
          <a className={`vch-row ${stylex.props(channelStyles.listNoteRowM).className}`} href={`#${paths.workbench({ channel: data.id })}`}>
            <Icon name="workbench" size={16} xstyle={channelStyles.listNoteIc} />
            <span {...stylex.props(channelStyles.listNoteText)}>{note}</span>
            <Icon name="arrow-r" size={14} xstyle={channelStyles.listNoteIc} />
          </a>
        ) : (
          <div {...stylex.props(channelStyles.listNoteRow)}>
            <span {...stylex.props(channelStyles.listNote)}>{note}</span>
            <a href={`#${paths.workbench({ channel: data.id })}`} {...stylex.props(channelStyles.listNoteLink)}>
              {tt.page.openWorkbench}
              <Icon name="arrow-r" size={13} />
            </a>
          </div>
        ))}
        <div {...stylex.props(channelStyles.tabRegion)}>
        <Tabs
          variant="underline"
          label={tt.page.tablist}
          buttonStyle={channelStyles.touchTarget}
          value={tab}
          onChange={(id) => pickTab(id as ChannelTab)}
          tabs={{ id: 'channel-tabs', panel: 'channel-panel' }}
          items={TABS.map((id) => ({
            id, label: <>{tabLabels[id]}{counts[id] && <span className={`num ${stylex.props(channelStyles.tabCount, tab === id && channelStyles.tabCountOn).className}`}>{counts[id]}</span>}</>,
          }))}
        />
        </div>

        <div id="channel-panel" role="tabpanel" aria-labelledby={`channel-tabs-${tab}`} {...stylex.props(channelStyles.tabRegion)}>
        {tab === 'sources' && <SourcesTab channel={data} channels={list} />}
        {tab === 'templates' && <TemplatesTab channel={data} />}
        {tab === 'settings' && <SettingsTab channel={data} onInstructions={() => setInstructionsOpen(true)} />}
        </div>
        {!narrow && <AssistantComposer key={data.id} channel={data} />}
      </main>
      {narrow && (
        <Sheet open={instructionsOpen} onClose={() => setInstructionsOpen(false)} height="auto"
          title={tt.composer.label} subtitle={tt.composer.sheetDescription(data.name || tt.page.noName)}>
          <AssistantComposer key={data.id} channel={data} sheet onSaved={() => setInstructionsOpen(false)} />
        </Sheet>
      )}
    </div>
  )
}

export default ChannelScreen

const COMPOSER_AGENTS = [
  { id: 'scout', name: 'Trinh sát', example: 'chỉ lấy video trên 20 phút' },
  { id: 'director', name: 'Đạo diễn', example: 'tiêu đề ngắn dưới 60 ký tự' },
  { id: 'artist', name: 'Hoạ sĩ', example: 'ảnh ấm hơn, bớt tối' },
  { id: 'qa', name: 'Kiểm định', example: 'kiểm kỹ lỗi tay, mặt và khung hình bị cắt' },
] as const
type ComposerAgent = (typeof COMPOSER_AGENTS)[number]['id']

/** Cùng một trạng thái/form cho dock PC và Sheet điện thoại.
 *  Lưu chỉ dẫn không gọi model; lỗi giữ bản nháp theo từng người nhận. */
function AssistantComposer({ channel, sheet = false, onSaved }: { channel: Channel; sheet?: boolean; onSaved?: () => void }) {
  const tt = t.channels.composer
  const [agent, setAgent] = useState<ComposerAgent>('scout')
  const [draft, setDraft] = useState(() => channel.agentInstructions.scout ?? '')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const drafts = useRef<Partial<Record<ComposerAgent, string>>>({})
  const current = COMPOSER_AGENTS.find((item) => item.id === agent)!

  const choose = (next: string) => {
    const recipient = COMPOSER_AGENTS.find((item) => item.id === next)
    if (pending || !recipient || recipient.id === agent) return
    drafts.current[agent] = draft
    setAgent(recipient.id)
    setDraft(drafts.current[recipient.id] ?? channel.agentInstructions[recipient.id] ?? '')
    setError(null)
  }
  const send = async () => {
    const value = draft.trim()
    if (!value || pending) return
    setPending(true)
    setError(null)
    try {
      const next = await patchChannel(channel.id, { agentInstructions: { [agent]: value } })
      setData(keys.channel(channel.id), next)
      void invalidate(keys.channels())
      delete drafts.current[agent]
      setDraft('')
      toast.ok(tt.sent(current.name, channel.name || t.channels.page.noName))
      onSaved?.()
    } catch (err) {
      setError(err instanceof Error ? err.message : tt.error)
    } finally { setPending(false) }
  }

  return (
    <div {...stylex.props(!sheet && channelStyles.dock)}>
      <form {...stylex.props(sheet ? channelStyles.composerSheet : channelStyles.composer)} aria-label={tt.label}
        onSubmit={(event) => { event.preventDefault(); void send() }}>
        {!sheet && <Icon name="msg" size={16} xstyle={channelStyles.listNoteIc} />}
        <label htmlFor="vch-composer" {...stylex.props(text.srOnly)}>{tt.label}</label>
        {sheet ? (
          <>
            <Segmented value={agent} onChange={choose} label={tt.choose}
              columns={2} fit="equal" buttonStyle={channelStyles.touchTarget}
              options={COMPOSER_AGENTS.map((item) => ({ id: item.id, disabled: pending, label: <span {...stylex.props(channelStyles.composerRecipient)}><Orb of={item.id} size={14} />{item.name}</span> }))} />
            <textarea id="vch-composer" {...stylex.props(channelStyles.composerTextArea)}
              value={draft} disabled={pending} placeholder={tt.placeholder(current.example)}
              onChange={(event) => setDraft(event.target.value)} />
            <p {...stylex.props(channelStyles.fieldHint)}>{tt.scopeHint}</p>
          </>
        ) : <input id="vch-composer" {...stylex.props(channelStyles.composerInput)}
          value={draft} disabled={pending} placeholder={tt.placeholder(current.example)}
          onChange={(event) => setDraft(event.target.value)} />}
        {!sheet && <ResponsiveSelect
          aria-label={tt.choose}
          value={agent}
          options={COMPOSER_AGENTS.map((item) => ({ value: item.id, label: item.name, icon: <Orb of={item.id} size={14} /> }))}
          onChange={choose}
          disabled={pending}
          icon={<Orb of={agent} size={12} />}
          prefix={tt.withAgent(current.name)}
          valueStyle={text.srOnly}
          triggerStyle={channelStyles.composerAgent}
          popWidth={220}
        />}
        <Button variant="primary" type="submit" aria-label={tt.send}
          pending={pending} disabled={!draft.trim()} xstyle={sheet ? channelStyles.touchTarget : channelStyles.composerSend}
          icon={!sheet ? <Icon name="arrow-up" size={20} /> : undefined}>{sheet ? tt.send : null}</Button>
      </form>
      {error && <p role="alert" {...stylex.props(channelStyles.composerError)}>{error}</p>}
    </div>
  )
}
