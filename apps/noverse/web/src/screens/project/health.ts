// Tình trạng dự án: GET /validate → lời thường + “Sửa tự động” + “Kiểm tra lại” (Outline.dc.html › health).
import { useMemo, useState } from 'react'
import { ApiError, fixProject, validateProject } from '../../api/client'
import type { LibraryConfig, ProjectDetail, ValidationIssue } from '../../api/types'
import { keys, useAction } from '../../data'
import { projectLabel } from '../../shell/labels'
import { toast } from '@noverse/ui'
import { fixableCodes, plainIssues, type PlainIssue } from './validate'
import { useValidation } from './useProject'

export type Health = {
  /** `loading` lần đầu · `error` không đọc được · `ok` · `issues` */
  state: 'loading' | 'error' | 'ok' | 'issues'
  issues: PlainIssue[]
  /** Chỉ cho “Chi tiết cho quản trị”: đường dẫn và mã gốc của máy chủ, kể cả `invalid-name`. */
  raw: ValidationIssue[]
  canFix: boolean
  fixing: boolean
  /** Đã bấm Kiểm tra lại / Sửa tự động trong phiên này (đổi câu “Noverse kiểm tra tự động…”). */
  checked: boolean
  checking: boolean
  errorText: string
  recheck: () => Promise<void>
  fix: () => Promise<void>
}

export function useHealth(project: string, detail: ProjectDetail, dirs: LibraryConfig['dirs']): Health {
  const validation = useValidation(project)
  const [checked, setChecked] = useState(false)
  const [checking, setChecking] = useState(false)
  const context = useMemo(() => ({ projectName: projectLabel(detail.project, project), dirs, stories: detail.stories }), [detail, project, dirs])
  const raw = validation.result?.issues ?? []
  const issues = useMemo(() => plainIssues(raw, context), [raw, context])

  const fixAction = useAction(async () => {
    const before = issues.length
    const next = await fixProject(project, fixableCodes(issues))
    validation.setResult(next)
    const after = plainIssues(next.issues ?? [], context).length
    setChecked(true)
    const fixed = Math.max(before - after, 0)
    toast.ok(after === 0 ? `Đã sửa ${fixed} việc · dự án sẵn sàng` : fixed > 0 ? `Đã sửa ${fixed} việc` : 'Chưa sửa được thêm việc nào')
  }, { invalidate: [keys.project(project, 'meta', 'validation')] })

  const recheck = async () => {
    setChecking(true)
    try {
      const next = await validateProject(project)
      validation.setResult(next)
      setChecked(true)
      if (plainIssues(next.issues ?? [], context).length === 0) toast.ok('Đã kiểm tra · không thiếu gì')
    } catch (error) {
      toast.err(error instanceof ApiError ? error.message : 'Chưa kiểm tra được — thử lại sau.')
    } finally { setChecking(false) }
  }

  const state = !validation.result
    ? (validation.error ? 'error' : 'loading')
    : issues.length === 0 ? 'ok' : 'issues'
  const error = validation.error
  return {
    state, issues, raw, checked, checking,
    canFix: issues.some((issue) => issue.fixable),
    fixing: fixAction.busy,
    errorText: error instanceof ApiError ? error.message : 'Máy chủ không phản hồi.',
    recheck,
    fix: async () => { await fixAction.run() },
  }
}
