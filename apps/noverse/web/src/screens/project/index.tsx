// screens/project — F3 (Outline.dc.html: view project + view story).
// ProjectScreen (#/ · #/p/:p) và StoryScreen (#/p/:p/s/:s) — cây dự án · nội dung · cây thư mục.
import { ApiError } from '../../api/client'
import { useLibrary } from '../../api/library'
import { useEffect, type ReactNode } from 'react'
import type { LibraryConfig, Outline, ProjectDetail, StorySummary } from '../../api/types'
import { navigate, paths, setDialog, useRoute } from '../../router'
import { useMediaQuery } from '../../shell/AppShell'
import { storyLabel } from '../../shell/labels'
import { EmptyState, Skeleton } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { RightColumn } from './RightColumn'
import { useHealth } from './health'
import './project.css'
import { ProjectOverview } from './ProjectOverview'
import { ProjectTree } from './ProjectTree'
import { StoryView, resolveStoryTab, type TabId } from './StoryView'
import { useOutline, useProjectDetail } from './useProject'
import { StaleBanner } from '@noverse/ui'
import { useConnection } from '../../data'
import { CandidateProjectNotice } from './CandidateProject'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

const DEFAULT_DIRS: LibraryConfig['dirs'] = { ideas: 'y-tuong', assets: 'tai-san', stories: 'kich_ban', exports: 'xuat-ban' }

function ScreenSkeleton() {
  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex' }}>
      <div style={{ flex: '0 0 272px', padding: 12, display: 'flex', flexDirection: 'column', gap: 8 }}>
        {[0, 1, 2, 3, 4].map((key) => <Skeleton key={key} height={30} radius={9} />)}
      </div>
      <div style={{ flex: 1, minWidth: 0, padding: 20, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <Skeleton width="40%" height={26} />
        <Skeleton height={140} radius={16} />
        <Skeleton height={160} radius={16} />
      </div>
      <div style={{ flex: '0 0 360px', padding: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
        {[0, 1, 2, 3, 4, 5].map((key) => <Skeleton key={key} height={24} radius={6} />)}
      </div>
    </div>
  )
}

function LoadFailure({ title, error, onRetry }: { title: string; error: unknown; onRetry: () => void }) {
  const message = error instanceof ApiError ? error.message : 'Máy chủ không phản hồi. Kiểm tra kết nối rồi thử lại.'
  return (
    <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
      <EmptyState icon="warning-2" title={title} hint={message}
        action={<Button onClick={onRetry} icon={<Icon name="refresh-2" size={15} />}>Thử lại</Button>} />
    </div>
  )
}


/** Ba cột của Outline.dc.html: cột trái · giữa · “Đang áp dụng”. */
function ProjectBody({ project, detail, dirs, rootDisplay, narrow, outline, activeStory, summary, children }: {
  project: string; detail: ProjectDetail; dirs: LibraryConfig['dirs']; rootDisplay: string | null; narrow: boolean
  outline: Outline | null; activeStory: StorySummary | null; summary: StorySummary | null; children?: ReactNode
}) {
  const health = useHealth(project, detail, dirs)
  return (
    <>
      <ProjectTree project={project} detail={detail} activeStory={activeStory} outline={outline} health={health} narrow={narrow} />
      {children ?? <ProjectOverview project={project} detail={detail} />}
      <RightColumn project={project} detail={detail} story={summary} health={health} rootDisplay={rootDisplay} narrow={narrow} />
    </>
  )
}

/** Tổng quan dự án (#/ · #/p/:project). project null = chưa chọn dự án đầu. */
export function ProjectScreen({ project }: { project: string | null }) {
  const connection = useConnection()
  const { library, loading: libraryLoading } = useLibrary()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const p = project ?? library?.projects[0]?.id ?? null
  const resource = useProjectDetail(p)
  const { detail, loading, error, refresh } = resource

  if (!p) {
    if (libraryLoading) {
      return <ScreenSkeleton />
    }
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <EmptyState icon="folder-open" title="Thư viện chưa có dự án"
          hint="Tạo dự án đầu tiên để bắt đầu viết truyện."
          action={<Button variant="primary" onClick={() => setDialog('newProject')} icon={<Icon name="add" size={16} />}>Dự án mới</Button>} />
      </div>
    )
  }
  if (resource.candidate) return <CandidateProjectNotice project={p} error={error} />
  if (loading && !detail) return <ScreenSkeleton />
  if (error && !detail) return <LoadFailure title="Không tải được dự án" error={error} onRetry={refresh} />
  if (!detail) return <ScreenSkeleton />

  const dirs = library?.config.dirs ?? DEFAULT_DIRS

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <PanelRefreshBar resourceKeys={[resource.key]} updatedAt={resource.updatedAt} />
      <StaleBanner error={detail && (error || (connection.state === 'retrying' ? 'Mất luồng cập nhật' : null))} onRetry={() => { connection.retry(); void refresh() }} />
      <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: narrow ? 'column' : 'row' }}>
      <ProjectBody project={p} detail={detail} dirs={dirs} rootDisplay={library?.rootDisplay || null} narrow={narrow} outline={null} activeStory={null} summary={null} />
      </div>
    </div>
  )
}

/** Truyện trong dự án (#/p/:p/s/:s) — 5 tab; tab giữ trong ?tab=. */
export function StoryScreen({ project, story }: { project: string; story: string }) {
  const { library } = useLibrary()
  const connection = useConnection()
  const parsed = useRoute()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const resource = useProjectDetail(project)
  const { detail, loading, error, refresh } = resource
  const summary: StorySummary | null = detail?.stories.find((item) => item.story === story) ?? null
  const { outline, loading: outlineLoading, error: outlineError, refresh: refreshOutline } =
    useOutline(project, summary ? story : null)

  const rawTab = parsed.search.get('tab')
  const tab = resolveStoryTab(rawTab)
  useEffect(() => {
    if (rawTab === 'rules') {
      const search = new URLSearchParams(parsed.search)
      search.set('tab', 'style')
      navigate(`${paths.story(project, story)}?${search}`, true)
    }
  }, [rawTab, project, story, parsed.search])

  if (resource.candidate) return <CandidateProjectNotice project={project} error={error} />
  if (loading && !detail) return <ScreenSkeleton />
  if (error && !detail) return <LoadFailure title="Không tải được dự án" error={error} onRetry={refresh} />
  if (!detail) return <ScreenSkeleton />

  const dirs = library?.config.dirs ?? DEFAULT_DIRS
  const rootDisplay = library?.rootDisplay || null

  if (!summary) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <EmptyState icon="book-1" title="Không tìm thấy truyện"
          hint={`Truyện “${storyLabel({ story })}” không có trong dự án này hoặc bạn không có quyền đọc.`}
          action={<Button onClick={() => navigate(paths.project(project))}>Về tổng quan dự án</Button>} />
      </div>
    )
  }

  const openTab = (id: TabId) => {
    navigate(`${paths.story(project, story)}${id === 'outline' ? '' : `?tab=${id}`}`, true)
  }
  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <PanelRefreshBar resourceKeys={[resource.key]} updatedAt={resource.updatedAt} />
      <StaleBanner error={detail && (error || (connection.state === 'retrying' ? 'Mất luồng cập nhật' : null))} onRetry={() => { connection.retry(); void refresh() }} />
      <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: narrow ? 'column' : 'row' }}>
      <ProjectBody project={project} detail={detail} dirs={dirs} rootDisplay={rootDisplay} narrow={narrow} outline={outline} activeStory={summary} summary={summary}>
        <StoryView project={project} summary={summary} narrow={narrow}
          tab={tab} onTab={openTab} outline={outline} outlineLoading={outlineLoading} outlineError={outlineError}
          onRetryOutline={refreshOutline} />
      </ProjectBody>
      </div>
    </div>
  )
}
