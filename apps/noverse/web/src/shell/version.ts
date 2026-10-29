// D1 — phiên bản đang chạy & báo có bản mới (Settings.dc.html `update`, Kit.dc.html menu tài khoản).
// Dữ liệu duy nhất: `GET /api/system` › `{version, commit, built, latestVersion, hasUpdate}`.
// Nhật ký thay đổi (“Có gì mới”) chưa có nguồn từ máy chủ — hộp chỉ hiện bản đang dùng / bản mới nhất.
import { getSystem } from '../api/client'
import type { BuildRange, ReleaseNote, SystemInfo } from '../api/types'
import { keys, useResource } from '../data'

const pad = (n: number) => String(n).padStart(2, '0')

/** `v0.9.3` → `0.9.3` (artboard: “Noverse 0.9.3”); `dev`/rỗng giữ nguyên. */
export function versionText(version: string | undefined): string {
  const value = (version ?? '').trim()
  return value.startsWith('v') ? value.slice(1) : value
}

/** Thời điểm build: ISO/RFC3339 → `24/10/2026 14:02`; `unknown`/rỗng → `''`. */
export function buildStamp(built: string | undefined): string {
  const value = (built ?? '').trim()
  if (!value || value === 'unknown') return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** Mã build: 7 ký tự đầu như artboard (`a41c9e2`); `unknown`/rỗng → `''`. */
export function shortCommit(commit: string | undefined): string {
  const value = (commit ?? '').trim()
  if (!value || value === 'unknown') return ''
  return value.slice(0, 7)
}

/** Dòng “Đang chạy”: `0.9.3 · build 24/10/2026 14:02` (thiếu phần nào bỏ phần đó). */
export function runningLine(info: Pick<SystemInfo, 'version' | 'built'> | null | undefined): string {
  const version = versionText(info?.version)
  const stamp = buildStamp(info?.built)
  return [version, stamp ? `build ${stamp}` : ''].filter(Boolean).join(' · ')
}

/** Dòng “Bản mới nhất”: `0.9.4`; chưa kiểm tra xong thì rơi về bản đang dùng. */
export function latestLine(info: Pick<SystemInfo, 'version' | 'latestVersion'> | null | undefined): string {
  return versionText(info?.latestVersion) || versionText(info?.version)
}

/** Có bản mới chỉ khi máy chủ nói có **và** biết số bản mới. */
export function hasNewVersion(info: Pick<SystemInfo, 'latestVersion' | 'hasUpdate'> | null | undefined): boolean {
  return !!info?.hasUpdate && !!versionText(info.latestVersion)
}

/** Ghi chú nhóm “Phiên bản Noverse” ở Nâng cao. */
export function versionNote(info: Pick<SystemInfo, 'version' | 'latestVersion' | 'hasUpdate'> | null | undefined): string {
  if (hasNewVersion(info)) return `Có bản mới ${versionText(info?.latestVersion)} · bạn đang dùng ${versionText(info?.version)}`
  return `Đang dùng bản mới nhất · ${versionText(info?.version)}`
}

/** Trang tải bản mới (cùng kho phát hành với lệnh tự cập nhật của CLI). */
export const RELEASES_URL = 'https://github.com/datnt1998/noverse/releases'

/**
 * “Viết bằng phiên bản” (Thông tin truyện): mỗi khoảng chương một dòng, đúng thứ tự máy chủ trả.
 * Bản ghi thiếu số chương bị bỏ; một chương lẻ ghi “Chương 6”, khoảng ghi “Chương 1–5”.
 */
export function buildRows(builds: BuildRange[] | undefined): { version: string; chapters: string; commit: string; date: string }[] {
  return (builds ?? [])
    .filter((build) => Number.isFinite(build?.from) && Number.isFinite(build?.to) && build.from > 0 && build.to >= build.from)
    .map((build) => ({
      version: versionText(build.version),
      chapters: build.from === build.to ? `Chương ${build.from}` : `Chương ${build.from}–${build.to}`,
      commit: (build.commit ?? '').trim(),
      // Cột ngày như artboard `verHist` (“20/10/2026”); bản ghi cũ không có ngày → “—” (cùng nhóm “Không rõ”).
      date: buildStamp(build.at).split(' ')[0] || '—',
    }))
}

export type ReleaseItem = { text: string; kind: 'Mới' | 'Sửa' | '' }
export type ReleaseRow = { version: string; date: string; items: ReleaseItem[]; current: boolean }

/** Tiêu đề markdown → loại mục: “Mới/Added/New” = Mới, “Sửa/Fixed” = Sửa, còn lại để trống. */
function itemKind(heading: string): ReleaseItem['kind'] {
  const text = heading.toLowerCase()
  if (/mới|added|new|feat/.test(text)) return 'Mới'
  if (/sửa|fixed|fix|bug/.test(text)) return 'Sửa'
  return ''
}

/**
 * “Có gì mới” từ `GET /api/system` › `releases[{version,date,notes}]`. `notes` là markdown nguyên văn:
 * tiêu đề `#` mở nhóm (quyết định nhãn Mới/Sửa), gạch đầu dòng thành từng mục; chữ thường thành mục không nhãn.
 * Bản không có `notes` vẫn hiện một dòng (số bản + ngày) — không bịa nội dung.
 */
export function releaseRows(releases: ReleaseNote[] | undefined, currentVersion: string): ReleaseRow[] {
  const current = versionText(currentVersion)
  return (releases ?? []).map((release) => {
    const items: ReleaseItem[] = []
    let kind: ReleaseItem['kind'] = ''
    for (const raw of (release.notes ?? '').split('\n')) {
      const line = raw.trim()
      if (!line) continue
      if (/^#{1,6}\s/.test(line)) { kind = itemKind(line.replace(/^#{1,6}\s*/, '')); continue }
      const text = line.replace(/^[-*+]\s+/, '').replace(/\*\*/g, '').trim()
      if (text) items.push({ text, kind })
    }
    // Hộp “Có gì mới” chỉ hiện ngày, không hiện giờ như nhãn build.
    return { version: versionText(release.version), date: buildStamp(release.date).split(' ')[0] ?? '', items, current: versionText(release.version) === current }
  })
}

/** `GET /api/system` dùng chung khoá cache với các màn khác. */
export function useVersionInfo(): SystemInfo | null {
  const resource = useResource(keys.system(), ({ signal }) => getSystem(signal), { staleTime: 60000 })
  return resource.data ?? null
}
