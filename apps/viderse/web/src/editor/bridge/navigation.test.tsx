import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import Link, { Image, useParams, useRouter, useSearchParams } from './navigation'
import { appRoutes } from '../vendor/lib/app-routes'

afterEach(() => { cleanup(); history.replaceState(null, '', '#/') })

describe('the nine Next-dependent editor surfaces use Viderse browser behavior', () => {
  it.each([
    ['clone context', appRoutes.cloneJob('job /1'), '#/?video=job%20%2F1'],
    ['project list', appRoutes.editor('project /1'), '#/editor/project%20%2F1'],
    ['editor empty/error state', appRoutes.board('channel-one'), '#/?channel=channel-one'],
    ['template banner', appRoutes.template('template /1'), '#/templates/template%20%2F1'],
    ['template popover', appRoutes.channel('channel-one', 'templates'), '#/channels/channel-one?tab=templates'],
  ])('%s links navigate to the correct hash route', (_surface, href, expected) => {
    render(<Link href={href}>Mở</Link>)
    expect(screen.getByRole('link', { name: 'Mở' }).getAttribute('href')).toBe(expected)
  })
  it('library previews use the original authenticated media URL without an optimizer', () => {
    render(<Image alt="Ảnh cảnh" src="/api/editor/media/one?hash=abc" width={160} height={90} unoptimized />)
    const image = screen.getByRole('img', { name: 'Ảnh cảnh' })
    expect(image.getAttribute('src')).toBe('/api/editor/media/one?hash=abc')
    expect(image.getAttribute('width')).toBe('160')
  })
  it('editor back reads the query inside the hash rather than the server query', () => {
    history.replaceState(null, '', '/?cue=wrong#/editor/project?fix=captionAlignment&cue=first')
    function BackQuery() { const query = useSearchParams(); return <output>{query.get('fix')}:{query.get('cue')}</output> }
    render(<BackQuery />)
    expect(screen.getByText('captionAlignment:first')).toBeTruthy()
  })
  it('clone quality resolves the encoded project ID from the active hash', () => {
    history.replaceState(null, '', '#/editor/project%2Fone')
    function QualityProject() { const params = useParams<{ projectId: string }>(); return <output>{params.projectId}</output> }
    render(<QualityProject />)
    expect(screen.getByText('project/one')).toBeTruthy()
  })
  it('editor shell navigation updates the hash without leaving the application pathname', () => {
    history.replaceState(null, '', '/#/editor/project')
    function ShellBack() { const router = useRouter(); return <button onClick={() => router.push('/?video=job-one')}>Bàn dựng</button> }
    render(<ShellBack />)
    act(() => { fireEvent.click(screen.getByRole('button', { name: 'Bàn dựng' })) })
    expect(location.hash).toBe('#/?video=job-one')
    expect(location.pathname).toBe('/')
  })
})
