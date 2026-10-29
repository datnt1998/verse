import { useEffect, useState } from 'react'
import { getLibrary, getProject } from '../api/client'
import { navigate, paths, type Parsed } from '../router'
import { Spinner } from '@noverse/ui'
import { MobilePage } from './primitives'
import { setNextRoute } from '../api/storage'
import { toast } from '@noverse/ui'
import { copyText } from '../lib/clipboard'

export function DeepLink({ parsed }: { parsed: Parsed }) {
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    if (parsed.route.name !== 'mobile-link') return
    const route = parsed.route
    let active = true
    setError('')
    void (async () => {
      const library = await getLibrary()
      const projects = await Promise.all(library.projects.map((project) => getProject(project.id)))
      if (!active) return
      const matches = projects.flatMap((project) => project.stories).filter((story) => story.id === route.id || story.story === route.id)
      const projectHint = parsed.search.get('project')
      const match = projectHint ? matches.find((story) => story.project === projectHint) : matches.length === 1 ? matches[0] : undefined
      if (!match) throw new Error(matches.length > 1 ? 'Có nhiều truyện cùng tên. Hãy mở truyện từ dự án tương ứng.' : 'Không tìm thấy truyện hoặc bạn chưa được cấp quyền đọc truyện này.')
      const path = route.kind === 'read' ? paths.reader(match.project, match.story, route.chapter ?? undefined) : route.kind === 'write' ? paths.workspace(match.project, match.story) : paths.audio(match.project, match.story, undefined, 'listen')
      const query = parsed.search.toString()
      navigate(`${path}${query ? `?${query}` : ''}`, true)
    })().catch((failure: unknown) => { if (active) setError(failure instanceof Error ? failure.message : 'Không mở được liên kết. Vui lòng thử lại.') })
    return () => { active = false }
  }, [parsed.route, parsed.search.toString(), attempt])
  return <MobilePage title="Mở liên kết"><div className="m-desktop-note">{error ? <><p role="alert">{error}</p><button className="m-button" onClick={() => setAttempt((value) => value + 1)}>Thử lại</button><button type="button" className="m-config-hint" onClick={() => navigate('/')}>Về thư viện</button></> : <><Spinner size={24} /><p>Đang tìm truyện…</p></>}</div></MobilePage>
}

export function installDeepLinks() {
  if (!location.hash && location.pathname !== '/') {
    const path = location.pathname + location.search
    if (/^\/(read|write|listen|more|cmd)\//.test(path)) {
      history.replaceState(null, '', `/#${path}`)
      dispatchEvent(new HashChangeEvent('hashchange'))
    }
  }
  const offerShare = (path: string) => {
    const url = new URL(`/#${path}`, location.origin).href
    toast('Sách nói đã sẵn sàng để chia sẻ', 'info', {
      label: typeof navigator.share === 'function' ? 'Chia sẻ' : 'Chép liên kết',
      onClick: async () => {
        try {
          if (typeof navigator.share === 'function') await navigator.share({ title: 'Sách nói Noverse', url })
          else { await copyText(url); toast.ok('Đã chép liên kết') }
        } catch (error) { if (!(error instanceof DOMException && error.name === 'AbortError')) throw error }
      },
    }, { duration: 6000 })
  }
  if (new URLSearchParams(location.hash.split('?')[1] ?? '').get('share') === '1') offerShare(location.hash.slice(1))
  const message = (event: MessageEvent) => {
    if (event.data?.type !== 'ainovel.navigate' || typeof event.data.url !== 'string') return
    const url = new URL(event.data.url, location.origin)
    if (url.origin !== location.origin) return
    const path = url.hash.startsWith('#/') ? url.hash.slice(1) : url.pathname + url.search
    if (!path.startsWith('/') || path.startsWith('//')) return
    if (event.data.expired) { setNextRoute(`#${path}`); navigate('/login?expired=1') }
    else navigate(path)
    if (event.data.action === 'share' && !event.data.expired) offerShare(path)
  }
  navigator.serviceWorker?.addEventListener('message', message)
  return () => navigator.serviceWorker?.removeEventListener('message', message)
}
