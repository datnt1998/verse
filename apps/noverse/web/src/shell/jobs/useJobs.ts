// Hook dùng chung của việc chạy nền: dock góc phải (máy tính) và bottom sheet việc nền (điện thoại, 18-mobile.md).
// Tự khởi động store (GET /api/jobs + sự kiện job.*) khi đã đăng nhập; gọi ở nhiều nơi vẫn chỉ một kết nối.
import { useCallback, useEffect, useMemo } from 'react'
import { useSession } from '../../api/session'
import { useRoute } from '../../router'
import { byPriority, isActive, needsYou, visibleJobs, type Where } from './logic'
import {
  cancelJob, dispatchDock, openHref, pauseJob, resumeJob, retryJob, runAction, startJobs, toggleNotify, useJobsState, useJobsSelector, type JobsState,
} from './store'

export const jobActions = { pause: pauseJob, resume: resumeJob, cancel: cancelJob, retry: retryJob, runAction, openHref, toggleNotify }

/** Màn hiện tại — Workspace ẩn việc agent của chính truyện đang mở (đã có ở cột agent). */
export function useJobsWhere(): Where {
  const { route } = useRoute()
  const workspace = route.name === 'workspace'
  const project = 'project' in route ? route.project ?? undefined : undefined
  const story = 'story' in route ? route.story : undefined
  return useMemo(() => ({ workspace, project, story }), [workspace, project, story])
}

export function useJobs(enabled = true) {
  const { user } = useSession()
  const state = useJobsState(enabled)
  const where = useJobsWhere()
  useEffect(() => (user && enabled ? startJobs(user.username) : undefined), [user?.username, enabled])
  const visible = useMemo(() => byPriority(visibleJobs(state.jobs, where)), [state.jobs, where])
  const active = useMemo(() => visible.filter(isActive), [visible])
  return {
    state, where,
    /** việc hiển thị được ở màn này, theo ưu tiên (Cần bạn → Lỗi → Đang chạy → Tạm dừng → Xong) */
    jobs: visible,
    active,
    needCount: active.filter(needsYou).length,
    notify: state.notify,
    actions: jobActions,
    dock: dispatchDock,
  }
}

/** Subscribe only to the job slice a visible control actually presents. */
export function useJobSelection<S>(select: (value: JobsState) => S): S {
  const { user } = useSession()
  useEffect(() => user ? startJobs(user.username) : undefined, [user?.username])
  return useJobsSelector(select)
}

export function useActiveJobs() {
  const where = useJobsWhere()
  const select = useCallback((value: JobsState) => byPriority(visibleJobs(value.jobs, where)).filter(isActive), [where])
  return useJobSelection(select)
}
