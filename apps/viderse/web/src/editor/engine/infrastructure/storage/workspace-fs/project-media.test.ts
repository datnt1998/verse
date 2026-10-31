// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/fake-editor-storage')).fakeStorageBackendMock,
)

import { fakeServer } from './__tests__/fake-editor-storage'
import { updateMedia } from './media'
import {
  associateMediaWithProject,
  getMediaForProject,
  getProjectMediaIds,
  getProjectsUsingMedia,
  removeMediaBatchFromProject,
  removeMediaFromProject,
} from './project-media'

beforeEach(() => {
  fakeServer.reset()
  fakeServer.addProject('p1')
  fakeServer.addProject('p2')
  fakeServer.seed({ id: 'm1' })
  fakeServer.seed({ id: 'm2' })
  fakeServer.seed({ id: 'm3' })
})

describe('project media links (server-backed)', () => {
  it('associates media with a project and lists it back', async () => {
    await associateMediaWithProject('p1', 'm1')
    await associateMediaWithProject('p1', 'm2')

    expect(new Set(await getProjectMediaIds('p1'))).toEqual(new Set(['m1', 'm2']))
    expect(await getProjectMediaIds('p2')).toEqual([])
  })

  it('association is idempotent', async () => {
    await associateMediaWithProject('p1', 'm1')
    await associateMediaWithProject('p1', 'm1')

    expect(await getProjectMediaIds('p1')).toEqual(['m1'])
  })

  it('refuses to link a media the server does not have', async () => {
    await expect(associateMediaWithProject('p1', 'ghost')).rejects.toMatchObject({
      code: 'not_found',
    })
  })

  it('removes one link without touching the media or other projects', async () => {
    await associateMediaWithProject('p1', 'm1')
    await associateMediaWithProject('p2', 'm1')

    await removeMediaFromProject('p1', 'm1')

    expect(await getProjectMediaIds('p1')).toEqual([])
    expect(await getProjectMediaIds('p2')).toEqual(['m1'])
    expect(fakeServer.records.has('m1')).toBe(true)
  })

  it('removes a batch, ignoring blank and repeated ids', async () => {
    await associateMediaWithProject('p1', 'm1')
    await associateMediaWithProject('p1', 'm2')
    await associateMediaWithProject('p1', 'm3')
    const unlink = vi.spyOn(fakeServer, 'unlink')

    await removeMediaBatchFromProject('p1', ['m1', 'm1', '', 'm2'])

    expect(unlink).toHaveBeenCalledTimes(2)
    expect(await getProjectMediaIds('p1')).toEqual(['m3'])
  })

  it('finds every project that still references a media', async () => {
    await associateMediaWithProject('p1', 'm1')
    await associateMediaWithProject('p2', 'm1')
    await associateMediaWithProject('p2', 'm2')

    expect((await getProjectsUsingMedia('m1')).sort()).toEqual(['p1', 'p2'])
    expect(await getProjectsUsingMedia('m2')).toEqual(['p2'])
    expect(await getProjectsUsingMedia('m3')).toEqual([])
  })

  it('a project deleted while scanning counts as not referencing the media', async () => {
    await associateMediaWithProject('p1', 'm1')
    fakeServer.links.delete('p2')

    expect(await getProjectsUsingMedia('m1')).toEqual(['p1'])
  })

  it('hydrates project media with the browser-derived overlay', async () => {
    await associateMediaWithProject('p1', 'm1')
    await updateMedia('m1', { fps: 60 })

    const media = await getMediaForProject('p1')

    expect(media).toHaveLength(1)
    expect(media[0]).toMatchObject({ id: 'm1', fps: 60, storageType: 'workspace' })
  })

  it('shares one server listing between concurrent callers of the same project', async () => {
    await associateMediaWithProject('p1', 'm1')
    const list = vi.spyOn(fakeServer, 'listMedia')

    const [a, b] = await Promise.all([getMediaForProject('p1'), getMediaForProject('p1')])

    expect(list).toHaveBeenCalledTimes(1)
    expect(a).toEqual(b)
  })

  it('does not reuse a finished listing, so a later call sees new links', async () => {
    await getMediaForProject('p1')
    await associateMediaWithProject('p1', 'm2')

    expect((await getMediaForProject('p1')).map((media) => media.id)).toEqual(['m2'])
  })

  it('propagates a listing failure and lets the next call retry', async () => {
    const failing = vi.spyOn(fakeServer, 'listMedia').mockImplementationOnce(() => {
      throw new Error('server down')
    })

    await expect(getMediaForProject('p1')).rejects.toThrow('server down')
    failing.mockRestore()
    await expect(getMediaForProject('p1')).resolves.toEqual([])
  })
})
