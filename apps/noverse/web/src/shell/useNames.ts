// Hook tên hiển thị: đọc tên dự án/truyện từ thư viện (tự nạp chi tiết dự án), không bao giờ trả slug thô.
import { useEffect } from 'react'
import { useLibrary } from '../api/library'
import type { Route } from '../router'
import { documentTitle, projectLabel, storyLabel } from './labels'
import { shortStoryTitle } from '../lib/audioName'

/**
 * Tên truyện (`story.json.title`). Chưa tải xong thì dùng `pending` (vd. tên trong snapshot) nếu có,
 * không thì '…' — không bịa tên từ slug. Chưa chọn truyện → ''.
 */
export function useStoryName(project: string | null, story: string | null, pending?: string | null): string {
  const { storyOf, ensureProject } = useLibrary()
  useEffect(() => { if (project) ensureProject(project) }, [project, ensureProject])
  if (!story) return ''
  const summary = project ? storyOf(project, story) : null
  if (summary) return storyLabel(summary)
  if (pending) return pending
  // Chưa có tên thật thì không bịa tên từ slug — hiện khung trống trung tính thay cho
  // một nhịp tên sai (không dấu) rồi nháy sang tên thật khi project meta về.
  return '…'
}

/**
 * Tên ngắn truyện cho tệp audio/metadata (§AF): `story.json.shortTitle` đã lưu,
 * chưa có thì cắt tên hiển thị ở ranh giới từ ≤ 32 ký tự (xem `shortStoryTitle`).
 */
export function useStoryShortTitle(project: string | null, story: string | null): string {
  const { storyOf, ensureProject } = useLibrary()
  useEffect(() => { if (project) ensureProject(project) }, [project, ensureProject])
  const summary = project && story ? storyOf(project, story) : null
  return shortStoryTitle(storyLabel(summary), summary?.shortTitle)
}

/** Tên dự án (`project.json.name`). Chưa chọn dự án → ''. */
export function useProjectName(project: string | null): string {
  const { library } = useLibrary()
  if (!project) return ''
  return projectLabel(library?.projects.find((entry) => entry.id === project), project)
}

/** Đặt tiêu đề tab trình duyệt theo màn đang mở (tên truyện · tên dự án · Noverse). */
export function useDocumentTitle(route: Route): void {
  const project = 'project' in route ? route.project : null
  const story = 'story' in route ? route.story : null
  const projectName = useProjectName(project) || null
  const storyName = useStoryName(project, story) || null
  const title = documentTitle(route, projectName, storyName)
  useEffect(() => { document.title = title }, [title])
}
