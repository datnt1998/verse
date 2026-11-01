// Đồng sáng tác cho truyện đang mở — /cocreate (chip trong ô can thiệp màn Truyện).
import { useMediaQuery } from '../../shell/AppShell'
import { navigate, paths } from '../../router'
import { Dialog } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { CoCreatePanel } from '../newstory/CoCreate'
import { refreshProjectData } from '../project/useProject'

export function CoCreateDialog({ project, story, onClose }: { project: string; story: string; onClose: () => void }) {
  const narrow = useMediaQuery('(max-width: 900px)')
  return (
    <Dialog padBody={false} title="Đồng sáng tác" width={1000} onClose={onClose}
      icon={<Icon name="messages-2" size={18} style={{ color: 'var(--color-accent)' }} />}
      style={{ height: narrow ? 'min(88vh, 720px)' : 'min(72vh, 620px)' }}>
      <div style={{ flex: 1, minHeight: 0, display: 'flex', padding: 14 }}>
        <CoCreatePanel project={project} story={story} narrow={narrow} onCancel={onClose}
          onCommitted={() => {
            refreshProjectData()
            navigate(paths.workspace(project, story), true)
            onClose()
          }} />
      </div>
    </Dialog>
  )
}
