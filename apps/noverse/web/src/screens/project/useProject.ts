import { useCallback } from 'react'
import { getOutline, getProject, getSystem, validateProject } from '../../api/client'
import { setNextRoute } from '../../api/storage'
import type { ValidateResult } from '../../api/types'
import { cache, invalidate, keys, useResource } from '../../data'
import { navigate } from '../../router'
import { useLibrary } from '../../api/library'
import { useCandidateProject } from './CandidateProject'

export function useCanReveal(): boolean {
  const resource = useResource(keys.system(), () => getSystem(), { staleTime: 60000 })
  return !!resource.data?.canReveal
}
export const refreshProjectData = () => { void invalidate(['project']); void invalidate(['story']); void invalidate(keys.library()) }

export function useProjectDetail(project: string | null) {
  const { library } = useLibrary()
  const listed = !!project && !!library?.candidates.includes(project)
  const resource = useResource(keys.project(project ?? '', 'meta'), () => getProject(project!), { enabled: !!project && !listed })
  const candidate = useCandidateProject(project, resource.error)
  return { ...resource, candidate, detail: resource.data ?? null, loading: !!project && !listed && resource.initialLoading, refresh: resource.refresh }
}
export function useValidation(project: string | null) {
  const { library } = useLibrary()
  const candidate = !!project && !!library?.candidates.includes(project)
  const key = keys.project(project ?? '', 'meta', 'validation')
  const resource = useResource(key, () => validateProject(project!), { enabled: !!project && !candidate })
  return { result: resource.data ?? null, error: resource.error, loading: resource.initialLoading, refresh: resource.refresh, setResult: (result: ValidateResult | null) => { if (result) cache.setData(key, result) } }
}
export function useSseUnauthorized() {
  return useCallback(() => {
    if (location.hash.startsWith('#/login')) return
    setNextRoute(location.hash || '#/')
    navigate('/login?expired=1', true)
  }, [])
}
export function useOutline(project: string | null, story: string | null) {
  const resource = useResource(keys.story(project ?? '', story ?? '', 'outline'), ({ signal }) => getOutline(project!, story!, signal), { enabled: !!project && !!story })
  return { ...resource, outline: resource.data ?? null, loading: !!project && !!story && resource.initialLoading, error: !!resource.error, refresh: resource.refresh }
}
