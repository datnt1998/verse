import { getJob } from '../api/client'
import { useResource } from '../data'
import { navigate, paths, type Parsed } from '../router'
import { MobileCommandResults } from '../screens/cmdresults/mobile'
import { Spinner } from '@noverse/ui'
import { MobileFeedback, MobilePage } from './primitives'
import { toastBottomFor } from './logic'
import { MobileRuntimeProvider, useTabMode } from './runtime'
import { useToastBottom } from './toastBottom'

function Result({ parsed }: { parsed: Parsed }) {
  useTabMode('hide')
  useToastBottom(toastBottomFor('command'))
  const job = parsed.route.name === 'command' ? parsed.route.job : parsed.search.get('result') ?? 'new'
  const detail = useResource(['job', job], ({ signal }) => getJob(job, signal), { enabled: job !== 'new' })
  const project = detail.data?.project ?? parsed.search.get('project') ?? ''
  const story = detail.data?.story ?? parsed.search.get('story') ?? ''
  const command = parsed.search.get('command') ?? (parsed.search.get('cmd') ? `/${parsed.search.get('cmd')}` : undefined)
  if (job !== 'new' && !detail.data) return <MobilePage title="Kết quả lệnh"><div className="m-desktop-note">{detail.error ? <><p role="alert">{detail.error instanceof Error ? detail.error.message : 'Không tải được kết quả lệnh.'}</p><button className="m-button" onClick={() => void detail.refresh()}>Thử lại</button></> : <Spinner size={24} />}</div></MobilePage>
  return <MobileCommandResults key={`${project}/${story}/${job}/${command ?? ''}`} project={project} story={story} job={job} command={command} onBack={() => navigate(project && story ? paths.workspace(project, story) : '/')} onGuide={(name) => navigate(`${paths.settings('mobile')}?page=guide&cmd=${encodeURIComponent(name)}`)} onRead={(chapter) => navigate(paths.reader(project, story, chapter))} onWrite={() => navigate(paths.workspace(project, story))} onJob={(id) => { const query = new URLSearchParams({ project, story, ...(command ? { command } : {}) }); navigate(`/cmd/${encodeURIComponent(id)}?${query}`, true) }} />
}
export function CommandDeepLink({ parsed }: { parsed: Parsed }) {
  return <MobileFeedback><MobileRuntimeProvider><div className="m-app" data-tab-mode="hide"><div className="m-content"><Result parsed={parsed} /></div></div></MobileRuntimeProvider></MobileFeedback>
}
