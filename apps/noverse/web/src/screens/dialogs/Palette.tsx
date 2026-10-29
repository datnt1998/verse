// CommandPalette — Dialogs.dc.html › palette. ⌘K/Ctrl+K (shell) hoặc gõ “/” ở ô nhập.
// Chỉ liệt kê lệnh có API thật; lệnh thiếu API hiện nhãn “Chưa có API” và không chạy.
import { useEffect, useMemo, useRef, useState } from 'react'
import {
  abortStory, canWriteStory, checkSync, getSnapshot, nextChapter, resumeStory, runSync, setReview, startSimulate,
} from '../../api/client'
import { useSession } from '../../api/session'
import { t } from '../../i18n/vi'
import { WRITE_COMMANDS } from '../../commands/model'
import { rememberHelpTarget } from '../../commands/target'
import { navigate, paths, setDialog, type Route } from '../../router'
import { Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { invalidate, keys, setData, useAction, useConnection, useResource } from '../../data'
import { TextField } from '@noverse/ui'
import { useEscapeLayer } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { confirmPause } from '../../ui/confirmPresets'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

type Ctx = { project: string; story: string; review?: boolean }

type Cmd = {
  group: string
  cmd: string
  desc: string
  needsStory?: boolean
  needsIdle?: boolean
  run?: (ctx: Ctx) => Promise<void> | void
}

const base = (cmd: string) => cmd.split(' ')[0]
const describe = (cmd: string, fallback: string) => {
  const value = t.commands[base(cmd) as keyof typeof t.commands]
  return typeof value === 'string' ? value : fallback
}

const guard = async (action: () => Promise<void>, ok: string) => {
  await action()
  if (ok) toast.ok(ok)
}

const COMMANDS: Cmd[] = [
  {
    group: 'Điều khiển lượt chạy', cmd: '/next', desc: describe('/next', 'Nghiệm thu và cho phép viết tiếp một chương mới'),
    needsStory: true, needsIdle: true,
    run: ({ project, story }) => guard(async () => { await nextChapter(project, story) }, 'Đã duyệt chương tiếp'),
  },
  {
    group: 'Điều khiển lượt chạy', cmd: '/abort', desc: 'Tạm dừng sáng tác đang chạy', needsStory: true,
  },
  {
    group: 'Điều khiển lượt chạy', cmd: '/resume', desc: 'Tiếp tục sáng tác đã tạm dừng', needsStory: true,
    run: ({ project, story }) => guard(async () => {
      const result = await resumeStory(project, story)
      toast.ok(result.label || 'Đã tiếp tục sáng tác')
    }, ''),
  },
  {
    group: 'Điều khiển lượt chạy', cmd: '/review on|off', desc: describe('/review', 'Bật/tắt chế độ nghiệm thu từng chương'),
    needsStory: true,
    run: ({ project, story, review }) => guard(async () => {
      const next = !review
      await setReview(project, story, next)
      toast.ok(next ? 'Đã bật nghiệm thu từng chương' : 'Đã tắt nghiệm thu từng chương')
    }, ''),
  },
  {
    group: 'Điều khiển lượt chạy', cmd: '/reopen [hướng]', desc: describe('/reopen', 'Mở lại cuốn sách đã hoàn thành để tiếp tục sáng tác'),
    needsStory: true, needsIdle: true, run: () => setDialog('reopen'),
  },
  {
    group: 'Truyện', cmd: '/export', desc: describe('/export', 'Xuất các chương đã hoàn thành ra tệp TXT hoặc EPUB'),
    needsStory: true,
    run: ({ project, story }) => navigate(`${paths.story(project, story)}?tab=export`),
  },
  {
    group: 'Truyện', cmd: '/cocreate', desc: describe('/cocreate', 'Tạm dừng viết để đồng sáng tác quy hoạch giai đoạn tiếp theo'),
    needsStory: true, run: () => setDialog('cocreate'),
  },
  {
    group: 'Truyện', cmd: '/sync --check', desc: describe('/sync', 'Kiểm tra hoặc tiếp nhận các chương đã sửa đổi thủ công'),
    needsStory: true, needsIdle: true,
    run: ({ project, story }) => guard(async () => {
      const { changed } = await checkSync(project, story)
      toast.ok(changed.length === 0 ? 'Không có chương nào thay đổi' : `${changed.length} chương đã sửa: ${changed.map((c) => c.chapter).join(', ')}`)
    }, ''),
  },
  {
    group: 'Truyện', cmd: '/sync', desc: 'Tiếp nhận các chương đã sửa đổi thủ công', needsStory: true, needsIdle: true,
    run: ({ project, story }) => guard(async () => { await runSync(project, story) }, 'Đã bắt đầu tiếp nhận chương sửa tay'),
  },
  {
    group: 'Truyện', cmd: '/diag', desc: describe('/diag', 'Bảng chẩn đoán sức khỏe và chất lượng truyện'),
    needsStory: true, run: () => setDialog('diag'),
  },
  {
    group: 'Cấu hình', cmd: '/model [vai-trò]', desc: describe('/model', 'Chuyển đổi Model và mức độ suy luận cho từng vai trò'),
    needsStory: true, run: () => setDialog('model'),
  },
  {
    group: 'Cấu hình', cmd: '/config', desc: describe('/config', 'Thêm hoặc chỉnh sửa Provider, Model và Cửa sổ ngữ cảnh'),
    run: () => navigate(paths.settings('providers')),
  },
  {
    group: 'Tạo & nhập', cmd: '/start <tệp>', desc: describe('/start', 'Tạo truyện mới từ tệp thiết lập hoặc đại cương có sẵn'),
    needsStory: true, run: () => setDialog('startFile'),
  },
  {
    group: 'Tạo & nhập', cmd: '/import …', desc: describe('/import', 'Nhập tiểu thuyết từ bên ngoài theo ngữ nghĩa'),
    needsStory: true, run: () => setDialog('import'),
  },
  {
    group: 'Tạo & nhập', cmd: '/importsim <tệp>', desc: describe('/importsim', 'Nhập hồ sơ mô phỏng văn phong có sẵn'),
    needsStory: true, run: () => setDialog('importsim'),
  },
  {
    group: 'Tạo & nhập', cmd: '/simulate', desc: describe('/simulate', 'Học văn phong từ các văn mẫu bạn đã thêm'),
    needsStory: true, needsIdle: true,
    run: ({ project, story }) => guard(async () => { await startSimulate(project, story) }, 'Đã bắt đầu mô phỏng văn phong'),
  },
  {
    group: 'Tạo & nhập', cmd: '/help', desc: describe('/help', 'Xem danh sách và hướng dẫn các lệnh'),
    run: () => { rememberHelpTarget(); navigate(paths.help()) },
  },
  {
    group: 'Điều hướng', cmd: 'Truyện mới', desc: 'Tạo truyện mới trong dự án đang mở', run: ({ project }) => navigate(paths.newstory(project)),
  },
  {
    group: 'Điều hướng', cmd: 'Dự án mới', desc: 'Tạo dự án (kênh / series) mới trong thư viện', run: () => setDialog('newProject'),
  },
  {
    group: 'Điều hướng', cmd: 'Nhật ký tiến độ', desc: 'Toàn bộ sự kiện của truyện đang mở', needsStory: true,
    run: ({ project, story }) => navigate(paths.activity(project, story)),
  },
  {
    group: 'Điều hướng', cmd: 'Tuỳ chỉnh', desc: 'Thư viện, nhà cung cấp, model, ngân sách, quy tắc viết', run: () => navigate(paths.settings()),
  },
]

const contextOf = (route: Route): Ctx | null => {
  if (route.name === 'project' || route.name === 'newstory') {
    return route.project ? { project: route.project, story: '' } : null
  }
  if ('project' in route && 'story' in route) return { project: route.project, story: route.story }
  return null
}

export function CommandPalette({ route }: { route: Route }) {
  const connection = useConnection()
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const baseContext = contextOf(route)
  const snapshotKey = keys.story(baseContext?.project ?? '', baseContext?.story ?? '', 'snapshot')
  const snapshot = useResource(snapshotKey, ({ signal }) => getSnapshot(baseContext!.project, baseContext!.story, signal), { enabled: !!baseContext?.story })
  const running = snapshot.data?.isRunning ?? null
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  const ctx = baseContext ? { ...baseContext, review: snapshot.data?.advanceMode === 'review' } : null
  const close = () => setDialog(null)
  // Bảng lệnh là một lớp: Esc đóng nó trước mọi thứ bên dưới (17-kit, chốt 25/10).
  useEscapeLayer(true, close)

  const commandAction = useAction(async (command: Cmd) => {
    if (!ctx) return
    await command.run?.(ctx)
    if (!['/reopen', '/cocreate', '/diag', '/model', '/start', '/import', '/importsim'].includes(base(command.cmd))) close()
    return true
  }, {
    invalidate: [keys.story(ctx?.project ?? '', ctx?.story ?? ''), keys.jobs()],
    optimistic: (command) => {
      const previous = snapshot.data
      const toggling = previous && base(command.cmd) === '/review'
      if (toggling) setData(snapshotKey, { ...previous, advanceMode: previous.advanceMode === 'review' ? 'auto' : 'review' })
      return () => { if (toggling) setData(snapshotKey, previous) }
    },
  })
  const resumeAction = useAction(async (project: string, story: string) => {
    await resumeStory(project, story)
    return true
  }, { success: 'Đã tiếp tục sáng tác', invalidate: [keys.story(ctx?.project ?? '', ctx?.story ?? ''), keys.project(ctx?.project ?? '')] })

  const { user } = useSession()
  // Người chỉ có quyền đọc không thấy lệnh cần quyền viết trong bảng lệnh (brief 25/10 › `/reopen`).
  const canWrite = !!user && (ctx?.story ? canWriteStory(user, `${ctx.project}/${ctx.story}`) : user.role === 'admin' || Object.values(user.projects).some((role) => role === 'owner' || role === 'collab'))

  const groups = useMemo(() => {
    const needle = query.trim().toLowerCase()
    const rows = COMMANDS.filter((command) => {
      if (!canWrite && WRITE_COMMANDS.includes(base(command.cmd).replace(/^\//, ''))) return false
      if (!needle) return true
      return command.cmd.toLowerCase().includes(needle) || command.desc.toLowerCase().includes(needle)
    })
    const order: string[] = []
    const map = new Map<string, Cmd[]>()
    for (const row of rows) {
      if (!map.has(row.group)) { map.set(row.group, []); order.push(row.group) }
      map.get(row.group)?.push(row)
    }
    return order.map((title) => ({ title, items: map.get(title) ?? [] }))
  }, [query, canWrite])

  const flat = useMemo(() => groups.flatMap((group) => group.items), [groups])
  const selected = flat[Math.min(active, Math.max(0, flat.length - 1))]

  const reasonOf = (command: Cmd): string | null => {
    if (!connection.writable && ['/next', '/abort', '/resume', '/review', '/sync', '/simulate'].includes(base(command.cmd))) return 'Đang mất kết nối'
    if (command.needsStory && !ctx?.story) return 'Cần chọn truyện'
    if (command.needsIdle && running !== false) return t.dialogs.paletteNeedIdle
    return null
  }

  const execute = (command: Cmd | undefined) => {
    if (!command || commandAction.busy || reasonOf(command) || !ctx) return
    if (base(command.cmd) === '/abort') {
      close()
      void confirmPause({
        body: 'Bạn có thể tiếp tục sáng tác bất cứ lúc nào.',
        choices: [{ value: 'after', label: 'Sau bước này', description: 'Khuyên dùng — giữ nguyên phần đã viết' }, { value: 'now', label: 'Ngay bây giờ', description: 'Bỏ phần đang viết dở' }],
        successMessage: '',
        onConfirm: async (choice) => {
          const previous = snapshot.data
          if (previous) setData(snapshotKey, { ...previous, isRunning: false })
          try {
            await abortStory(ctx.project, ctx.story, choice !== 'now')
            await Promise.all([invalidate(keys.story(ctx.project, ctx.story)), invalidate(keys.project(ctx.project))])
            toast('Đã tạm dừng sáng tác', 'ok', { label: 'Hoàn tác', onClick: async () => { await resumeAction.run(ctx.project, ctx.story) } }, { undo: true })
          } catch (error) {
            if (previous) setData(snapshotKey, previous)
            throw error
          }
        },
      })
      return
    }
    void commandAction.run(command)
  }

  useEffect(() => {
    const node = listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)
    node?.scrollIntoView({ block: 'nearest' })
  }, [active])

  useEffect(() => { setActive(0) }, [query])

  return (
    <div style={{ position: 'fixed', inset: 0, zIndex: 40 }}>
      <div aria-hidden="true" onClick={close} style={{ position: 'absolute', inset: 0, background: 'rgba(5,5,7,0.62)' }} />
      <div style={{ position: 'absolute', left: '50%', bottom: 96, transform: 'translateX(-50%)', width: 'min(720px, calc(100% - 32px))' }}>
        <div className="dlg" role="dialog" aria-label="Bảng lệnh" aria-modal="true"
          style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 14px', borderBottom: '1px solid var(--line-2)' }}>
            <Icon name="command" size={18} style={{ color: 'var(--color-fg-3)' }} />
            <label htmlFor="palette-q" className="sr-only">Tìm lệnh</label>
            <TextField id="palette-q" ref={inputRef} autoFocus type="text" value={query}
              onChange={(value) => setQuery(value.replace(/^\//, ''))} clearable
              onKeyDown={(event) => {
                if (event.key === 'ArrowDown') { event.preventDefault(); setActive((index) => Math.min(flat.length - 1, index + 1)) }
                else if (event.key === 'ArrowUp') { event.preventDefault(); setActive((index) => Math.max(0, index - 1)) }
                else if (event.key === 'Enter' || event.key === 'Tab') { event.preventDefault(); execute(selected) }
                else if (event.key === 'Escape') { event.preventDefault(); close() }
              }}
              placeholder={t.dialogs.paletteFilter}
              className="mono"
              style={{
                flex: 1, height: 32, background: 'transparent', border: 'none', outline: 'none',
                color: 'var(--color-fg)', fontSize: 15,
              }} />
            <Kbd>Esc</Kbd>
          </div>

          <PanelRefreshBar resourceKeys={[snapshot.key]} updatedAt={snapshot.updatedAt} />
          <StaleBanner error={snapshot.error} onRetry={snapshot.refresh} />
          <div ref={listRef} style={{ maxHeight: 460, overflowY: 'auto', padding: 6 }}>
            {groups.map((group) => (
              <div key={group.title}>
                <div className="lbl" style={{ padding: '8px 10px 4px' }}>{group.title}</div>
                {group.items.map((command) => {
                  const index = flat.indexOf(command)
                  const reason = reasonOf(command)
                  const on = command === selected
                  return (
                    <button key={command.cmd} type="button" data-index={index} disabled={Boolean(reason) || commandAction.busy} aria-busy={commandAction.pending && on || undefined}
                      onMouseEnter={() => setActive(index)}
                      onClick={() => execute(command)}
                      style={{
                        width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '8px 10px',
                        borderRadius: 10, border: 'none', textAlign: 'left',
                        background: on && !reason ? 'var(--color-surface-6)' : 'transparent',
                        color: 'var(--color-fg)', opacity: reason ? 0.6 : 1,
                        cursor: reason ? 'default' : 'pointer',
                      }}>
                      <span className="mono" style={{
                        flex: '0 0 170px', fontSize: 13,
                        color: reason === 'Chưa có API' ? 'var(--color-fg-3)' : 'var(--color-fg)',
                      }}>{command.cmd}</span>
                      <span style={{
                        flex: 1, minWidth: 0, fontSize: 13, color: 'var(--color-fg-3)',
                        whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                      }}>{command.desc}</span>
                      {commandAction.pending && on && <Spinner size={14} />}
                      {reason && (
                        <span style={{
                          height: 20, padding: '0 7px', display: 'flex', alignItems: 'center', borderRadius: 6,
                          background: reason === 'Chưa có API' ? 'var(--tint-warn)' : 'var(--color-surface-5)',
                          color: reason === 'Chưa có API' ? 'var(--color-warn)' : 'var(--color-fg-3)', fontSize: 11,
                          whiteSpace: 'nowrap',
                        }}>{reason}</span>
                      )}
                    </button>
                  )
                })}
              </div>
            ))}
            {groups.length === 0 && (
              <p style={{ margin: 0, padding: 24, textAlign: 'center', color: 'var(--color-fg-4)' }}>{t.empty.palette(query)}</p>
            )}
          </div>

          <div style={{
            display: 'flex', gap: 14, padding: '8px 14px', borderTop: '1px solid var(--line-2)',
            fontSize: 12, color: 'var(--color-fg-4)',
          }}>
            <span style={{ display: 'flex', gap: 6, alignItems: 'center' }}><Kbd>↑ ↓</Kbd>chọn</span>
            <span style={{ display: 'flex', gap: 6, alignItems: 'center' }}><Kbd>Tab</Kbd><Kbd>Enter</Kbd>chấp nhận</span>
            <span style={{ display: 'flex', gap: 6, alignItems: 'center' }}><Kbd>Esc</Kbd>đóng</span>
          </div>
        </div>
      </div>
    </div>
  )
}
