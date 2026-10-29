// AppHeader — header của app, 1:1 theo artboard:
// Main (Workspace): toggle trái · biểu tượng Noverse · crumb link · nav · Tuỳ chỉnh · avatar · toggle phải.
// Outline (Dự án): biểu tượng · switcher dự án (chevron) · truyện · tập.
// NewStory: biểu tượng · crumb link · "Truyện mới". Settings: biểu tượng · "Noverse / Tuỳ chỉnh".
// Mobile (<760px): header thu gọn — biểu tượng + "dự án · Tập N" + tên truyện + avatar (Phone.dc.html).
import { useEffect } from 'react'
import { t } from '../i18n/vi'
import { CMD_UI } from '../i18n/commands.vi'
import { paths, type Route } from '../router'
import { useLibrary } from '../api/library'
import { useSession } from '../api/session'
import { helpTarget, rememberHelpTarget } from '../commands/target'
import { HelpHeaderChip } from '../commands/HelpHeaderChip'
import { AccountMenu } from './AccountMenu'
import { ContextCrumb } from './ContextCrumb'
import { MainNav } from './MainNav'
import { mobileHeaderCopy } from './labels'
import { useProjectName, useStoryName } from './useNames'
import { usePanels } from './stores'
import { Avatar } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { BrandMark } from '@noverse/ui'

function storyContext(route: Route): { project: string | null; story: string | null } {
  switch (route.name) {
    case 'story': case 'workspace': case 'activity': case 'reader': case 'audio': case 'brain': case 'quality':
      return { project: route.project, story: route.story }
    case 'project': case 'newstory': case 'projbrain':
      return { project: route.project, story: null }
    case 'help': {
      // Giữ crumb của truyện đang mở (Commands.dc.html): nhớ từ lần mở gần nhất.
      const target = helpTarget()
      return { project: target?.project ?? null, story: target?.story ?? null }
    }
    default:
      return { project: null, story: null }
  }
}

/**
 * Context for the main navigation bar: on screens without a story in the path (Dự án, Truyện mới, Não dự án,
 * Cài đặt…), fall back to the story the user opened last (verified to still exist via `helpTarget`), so
 * Bàn viết / Trình đọc / Audio keep working instead of being locked. On the project screen of ANOTHER project,
 * the story is not borrowed from the previous project. The crumb still uses `storyContext`.
 */
export function navContext(project: string | null, story: string | null): { project: string | null; story: string | null } {
  if (project && story) return { project, story }
  const last = helpTarget()
  if (!last) return { project, story }
  if (!project) return last
  return last.project === project ? last : { project, story }
}

/** Dự án/truyện = popover switcher có chevron; các màn khác = link tới Dự án. */
const SWITCHER_ROUTES: Partial<Record<Route['name'], true>> = { project: true, story: true }

/**
 * Nút ẩn/hiện panel ở header — 32×32, bo 8, icon 18 (Main.dc.html, Outline.dc.html).
 * `flex: 0 0 auto`: trước đây là `Button variant="icon"` (36×36, bo 10) mặc định co giãn, nên khi
 * header hẹp (1100–1400px) nó bị ép còn ~19–30px và hiện thành một “viên thuốc” dọc.
 */
function PanelToggle({ side, open, onClick }: { side: 'left' | 'right'; open: boolean; onClick: () => void }) {
  const left = side === 'left'
  return (
    <button type="button" className="hov-ghost" aria-pressed={open}
      aria-label={left ? t.nav.toggleOutline : t.nav.toggleDetail}
      title={`${left ? t.nav.outline : t.nav.detailPanel}  ${left ? '[' : ']'}`}
      onClick={onClick}
      style={{
        flex: '0 0 auto', width: 32, height: 32, padding: 0, borderRadius: 8, border: 'none',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        background: open ? 'var(--color-surface-5)' : 'transparent',
        color: open ? 'var(--color-fg)' : 'var(--color-fg-4)',
        transition: 'background .15s, color .15s',
      }}>
      <Icon name={left ? 'sidebar-left' : 'sidebar-right'} size={18} />
    </button>
  )
}

export function AppHeader({ route, mobile = false }: { route: Route; mobile?: boolean }) {
  const { project: routeProject, story } = storyContext(route)
  const { library, storyOf, ensureProject } = useLibrary()
  // `#/` mở màn Dự án với dự án đầu tiên (screens/project/index.tsx) — header phải chỉ cùng dự án đó.
  const project = routeProject ?? (route.name === 'project' ? library?.projects[0]?.id ?? null : null)
  const panels = usePanels()
  const isWorkspace = route.name === 'workspace'
  const isSettings = route.name === 'settings'
  // Não hệ thống nằm trong Tuỳ chỉnh: nút Tuỳ chỉnh sáng (SysBrain.dc.html), không chấm.
  const settingsOn = isSettings || route.name === 'sysbrain'
  const { pendingLessons } = useSession()
  const lessonsDot = !settingsOn && pendingLessons > 0
  const settingsLabel = lessonsDot ? t.sysbrain.settingsDot(pendingLessons) : t.nav.settings
  const isHelp = route.name === 'help'
  useEffect(() => { if (project) ensureProject(project) }, [project, ensureProject])
  const summary = project && story ? storyOf(project, story) : null

  if (mobile) return <MobileHeader route={route} />

  const switcher = SWITCHER_ROUTES[route.name] === true
  return (
    <header style={{
      position: switcher ? 'relative' : undefined, zIndex: switcher ? 5 : undefined,
      flex: '0 0 auto',
      // Gói Q: lưới 3 cột — thanh điều hướng luôn ở chính giữa bất kể đường dẫn bên trái dài ngắn.
      // Chiều cao 54 + 1 px kẻ dưới; lề ngang 16. Không `flex-wrap`: thanh trên không bao giờ cao thêm.
      display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto minmax(0, 1fr)',
      alignItems: 'center', columnGap: 16,
      height: 54, padding: '0 16px', borderBottom: '1px solid var(--line-1)',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: switcher ? 6 : 10, minWidth: 0, overflow: 'hidden' }}>
        {isWorkspace && (
          <PanelToggle side="left" open={panels.left} onClick={() => panels.toggle('left')} />
        )}
        <BrandMark size={22} label={t.brand.name} />
        {isSettings ? (
          <>
            <span style={{ fontWeight: 600, letterSpacing: '-0.01em' }}>{t.brand.name}</span>
            <span style={{ color: 'var(--color-fg-5)' }} aria-hidden="true">/</span>
            <span style={{ fontSize: 13 }}>{t.nav.settings}</span>
          </>
        ) : (
          <ContextCrumb
            project={project}
            story={summary}
            variant={switcher ? 'switcher' : 'link'}
            volume={route.name === 'brain' ? t.brain.title : route.name === 'quality' ? t.quality.title : undefined}
            trailing={route.name === 'newstory' ? t.nav.newStory : route.name === 'brain' ? t.brain.title : route.name === 'projbrain' ? t.projbrain.title : route.name === 'sysbrain' ? t.sysbrain.title : null}
          />
        )}
      </div>
      <MainNav route={route} {...navContext(project, story)} />
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 8, minWidth: 0 }}>
        {isHelp && <HelpHeaderChip />}
        <a className="hov-ghost" href={`#${paths.help()}`} onClick={() => rememberHelpTarget()}
          aria-label={CMD_UI.helpButton} title={CMD_UI.helpButton}
          aria-current={isHelp ? 'page' : undefined}
          style={{
            height: 32, padding: '0 10px', borderRadius: 8, display: 'flex', alignItems: 'center',
            gap: 6, fontSize: 12.5, whiteSpace: 'nowrap',
            color: isHelp ? 'var(--color-fg)' : 'var(--color-fg-3)',
            background: isHelp ? 'var(--color-surface-6)' : 'transparent',
          }}>
          <Icon name={isHelp ? 'messages-2-bulk' : 'messages-2'} size={17}
            style={{ color: isHelp ? 'var(--color-accent)' : undefined }} />
          {CMD_UI.helpButton}
        </a>
        <a className="hov-ghost" href={`#${paths.settings()}`} aria-label={settingsLabel}
          title={lessonsDot ? t.sysbrain.settingsDotTitle(pendingLessons) : t.nav.settings}
          aria-current={settingsOn ? 'page' : undefined}
          style={{
            position: 'relative', flex: '0 0 auto', width: 32, height: 32, borderRadius: 8, display: 'flex', alignItems: 'center', justifyContent: 'center',
            color: settingsOn ? 'var(--color-accent)' : 'var(--color-fg-3)',
            background: settingsOn ? 'var(--color-surface-6)' : 'transparent',
          }}>
          <Icon name={isSettings ? 'setting-2-bulk' : 'setting-2'} size={18} />
          {/* D15: chấm tím khi Não hệ thống có bài học chờ duyệt (Main/Outline.dc.html; chỉ quản trị có số). */}
          {lessonsDot && <span aria-hidden="true" style={{
            position: 'absolute', top: 6, right: 6, width: 7, height: 7, borderRadius: '50%',
            background: 'var(--color-accent)', boxShadow: '0 0 0 2px var(--color-bg)',
          }} />}
        </a>
        <AccountMenu />
        {isWorkspace && (
          <PanelToggle side="right" open={panels.right} onClick={() => panels.toggle('right')} />
        )}
      </div>
    </header>
  )
}

function MobileHeader({ route }: { route: Route }) {
  const { project, story } = storyContext(route)
  const { user } = useSession()
  const projectName = useProjectName(project) || null
  const storyTitle = useStoryName(project, story) || null
  const { meta, title } = mobileHeaderCopy(route, projectName, storyTitle)
  return (
    <header style={{
      flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 10,
      padding: '12px 14px 10px', borderBottom: '1px solid var(--line-1)',
    }}>
      <BrandMark size={20} label={t.brand.name} />
      <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
        <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{meta}</span>
        <span style={{ fontSize: 14, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {title}
        </span>
      </div>
      {user && <Avatar name={user.name || user.username} size={30} label={t.account.avatarLabel(user.name || user.username)} />}
    </header>
  )
}
