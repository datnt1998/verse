# Screens — bản đồ route → component

Mỗi thư mục thuộc một lát wave-2 (12-implementation-plan.md). Entry `index.tsx`
export component với **tên + props chốt dưới đây** — stub hiện tại sẽ bị thay 1:1,
giữ nguyên chữ ký để `App.tsx` không đổi. Mọi screen render bên trong `AppShell`
(header/nav/tab-bar/toast/banner đã có sẵn).

Hộp thoại phủ lên route hiện tại qua query `?dlg=<id>` trong hash
(vd `#/p/x/s/y/work?dlg=palette`). `router.ts` cung cấp `setDialog(id|null)`.

| Route | Component | Props | Chủ sở hữu |
|---|---|---|---|
| `#/` · `#/p/:project` | `ProjectScreen` (project/) | `{ project: string \| null }` — null khi `#/` (chọn dự án đầu) | F3 |
| `#/p/:p/s/:s` | `StoryScreen` (project/) | `{ project, story }` | F3 |
| `#/p/:p/s/:s/work` | `WorkspaceScreen` (workspace/) | `{ project, story }` | F1 |
| `#/p/:p/s/:s/activity` | `ActivityScreen` (activity/) | `{ project, story }` | F1 |
| `#/p/:p/s/:s/read/:chapter?` | `ReaderScreen` (reader/) | `{ project, story, chapter: number \| null }` | F2 |
| `#/p/:p/s/:s/audio/:volume?` · `/audio/:volume/make` · `/audio/:volume/listen` | `AudioScreen` (audio/) | `{ project, story, volume: number \| null, mode: 'review' \| 'make' \| 'listen' }` | F6 (v1.1, `10-audio.md`; stub ở v1) |
| `#/p/:p/new` | `NewStoryScreen` (newstory/) | `{ project }` | F3 |
| `#/settings/:section?` | `SettingsScreen` (settings/) | `{ section: string \| null }` | F5 |
| `#/settings/members` | `MembersSettings` (settings/members) | admin only; accounts, project roles, devices, one-time passwords and admin audit (§AD) | Gói AD |
| `#/login` | `AuthScreen` (auth/) | `{ expired?: boolean }` (`?expired=1` sau 401) | FA |
| `#/pair/:code` | `PairScreen` (auth/) | `{ code: string }` | FA |
| `#/welcome/:step?` | `OnboardScreen` (onboard/) | `{ step: string \| null }` | FA |

## Hộp thoại (`?dlg=`)

| dlg | Component | Ghi chú |
|---|---|---|
| `qr` · `devices` | `AccountDialogs` (account/) `{ dialog }` | menu tài khoản · QR · thiết bị (FA); Thành viên mở `#/settings/members`, không còn hộp `users` |
| `palette` · `newProject` · `reopen` · `import` | `AppDialogs` (dialogs/) `{ dialog, route }` | bảng lệnh · dự án mới · viết tiếp · nhập truyện (F3) |
| `locked` | `LockedDialog` (shell) | 409 truyện bị khóa — AppShell tự render |

## Quy ước dùng chung

- SSE: `useStoryEvents(project, story, onUnauthorized)` → `{ snapshot, events,
  streamText, cocreateText, jobs, connected, reconnectAt, revision, retryNow }`.
  Screen đang hiển thị workspace nên gọi `setConnection(reconnectAt, retryNow)`
  để banner mất kết nối của shell hoạt động.
- Panel `[`/`]`: `usePanels()` từ `shell/stores` (đã persist `ainovel.panels`).
- Quyền: `canReadStory/canWriteStory/canReadProject/canCreateStory` (api/client).
- Chuỗi: `t` từ `i18n/vi`; chuỗi host zh→vi: `translateHost` (i18n/hostStrings).
