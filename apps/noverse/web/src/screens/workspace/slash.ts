// G13 (19-gap-closure.md): dòng bắt đầu bằng `/` trong ô chat web đi đâu.
// `/help` do giao diện xử lý (mở trang Hướng dẫn lệnh); `/reopen`, `/import` mở hộp thoại sẵn có;
// `/cocreate` (alias `/plan`) giữ luồng đồng sáng tác của Workspace; còn lại gửi `POST …/command`.
import { paths } from '../../router'

export type SlashRoute =
  | { kind: 'help' }
  | { kind: 'dialog'; dialog: 'reopen' | 'import' }
  | { kind: 'cocreate' }
  | { kind: 'server' }

/** Đường dẫn trang Hướng dẫn lệnh (13-commands.md §2) — `paths.help()` của router. */
export const helpPath = (cmd?: string | null): string => paths.help(cmd)

export function slashRoute(line: string): SlashRoute {
  const head = line.trim().split(/\s+/)[0] ?? ''
  switch (head) {
    case '/help': return { kind: 'help' }
    case '/reopen': return { kind: 'dialog', dialog: 'reopen' }
    case '/import': return { kind: 'dialog', dialog: 'import' }
    case '/cocreate':
    case '/plan': return { kind: 'cocreate' }
    default: return { kind: 'server' }
  }
}
