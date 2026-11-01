// screens/dialogs — F3 (Dialogs.dc.html): bảng lệnh · Dự án mới · Viết tiếp · Nhập tiểu thuyết.
// App.tsx chuyển tiếp mọi ?dlg= trừ nhóm tài khoản/locked sang đây.
import type { ReactNode } from 'react'
import { startFromFile, startImportSim } from '../../api/client'
import { t } from '../../i18n/vi'
import { navigate, paths, setDialog, type Parsed } from '../../router'
import { Button } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { EmptyState } from '@noverse/ui'
import { keys, useAction } from '../../data'
import { refreshProjectData } from '../project/useProject'
import { CoCreateDialog } from './CoCreateDialog'
import { CommandPalette } from './Palette'
import { DiagPanel, ModelPanel } from '../cmdresults'
import { ImportDialog } from './ImportDialog'
import { NewProjectDialog } from './NewProjectDialog'
import { PathDialog } from './PathDialog'
import { ReopenDialog } from './ReopenDialog'

function NeedStory({ onClose }: { onClose: () => void }) {
  return (
    <Dialog padBody={false} title="Cần chọn truyện" width={420} onClose={onClose}
      footer={<Button onClick={onClose}>{t.dialogs.close}</Button>}>
      <EmptyState icon="book-1" title="Chọn truyện trước"
        hint="Lệnh này áp dụng cho một truyện cụ thể — mở truyện trong Dự án rồi thử lại." />
    </Dialog>
  )
}

function StartFileDialog({ project, story, onClose }: { project: string; story: string; onClose: () => void }) {
  const action = useAction(async (path: string) => {
    await startFromFile(project, story, path)
    refreshProjectData()
    navigate(paths.workspace(project, story), true)
    onClose()
    return true
  }, { success: 'Đã bắt đầu từ tệp', invalidate: [keys.story(project, story)] })
  return (
    <PathDialog title="Bắt đầu từ tệp" label="Đường dẫn tệp" placeholder="thiet-lap.md"
      hint="/start <đường-dẫn>" submitLabel="Bắt đầu" busy={action.pending} disabled={action.busy} onClose={onClose}
      onSubmit={(path) => { void action.run(path) }} />
  )
}

function ImportSimDialog({ project, story, onClose }: { project: string; story: string; onClose: () => void }) {
  const action = useAction(async (path: string) => {
    const result = await startImportSim(project, story, path)
    onClose()
    return result
  }, { success: 'Đã bắt đầu nhập hồ sơ mô phỏng', invalidate: [keys.jobs()] })
  return (
    <PathDialog title="Nhập hồ sơ mô phỏng" label="Đường dẫn hồ sơ" placeholder="van-phong/co-trang.json"
      hint="/importsim <tệp>" submitLabel="Nhập hồ sơ" busy={action.pending} disabled={action.busy} onClose={onClose}
      onSubmit={(path) => { void action.run(path) }} />
  )
}

export function AppDialogs({ dialog, route }: { dialog: string; route: Parsed['route'] }) {
  const close = () => setDialog(null)
  const project = 'project' in route ? route.project : null
  const story = 'story' in route ? route.story : null

  const withStory = (render: (project: string, story: string) => ReactNode) =>
    project && story ? render(project, story) : <NeedStory onClose={close} />

  switch (dialog) {
    case 'palette':
      return <CommandPalette route={route} />
    case 'newProject':
      return <NewProjectDialog onClose={close} />
    case 'reopen':
      return withStory((p, s) => <ReopenDialog project={p} story={s} onClose={close} />)
    case 'import':
      return withStory((p, s) => <ImportDialog project={p} story={s} onClose={close} />)
    case 'diag':
    case 'diag:last':
      return withStory((p, s) => <DiagPanel project={p} story={s} mode={dialog === 'diag:last' ? 'last' : 'run'} onClose={close} />)
    case 'startFile':
      return withStory((p, s) => <StartFileDialog project={p} story={s} onClose={close} />)
    case 'importsim':
      return withStory((p, s) => <ImportSimDialog project={p} story={s} onClose={close} />)
    case 'cocreate':
      return withStory((p, s) => <CoCreateDialog project={p} story={s} onClose={close} />)
    default:
      // /model hoặc /model:<vai> (14-jobs-dock §2)
      if (dialog === 'model' || dialog.startsWith('model:')) {
        return withStory((p, s) => <ModelPanel project={p} story={s} role={dialog.slice(6) || undefined} onClose={close} />)
      }
      return null
  }
}
