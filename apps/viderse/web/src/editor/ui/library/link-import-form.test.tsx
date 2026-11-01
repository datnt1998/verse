// @vitest-environment jsdom
// "Lấy từ link" giờ là job phía server: form chỉ chờ trạng thái, xong thì thư
// viện nạp lại và chọn media mới. Tải dài không còn giữ request mở.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ServerMediaRecord } from '@editor/storage/types'

const mocks = vi.hoisted(() => ({
  importMediaFromLink: vi.fn(),
  announce: vi.fn(),
  toastError: vi.fn(),
  loadMediaItems: vi.fn(async () => {}),
  selectMedia: vi.fn(),
}))

vi.mock('@editor/storage/storage-backend', () => ({ getEditorStorage: () => ({ importMediaFromLink: mocks.importMediaFromLink }) }))
vi.mock('@/editor/ui/shell/announcer', () => ({ announce: mocks.announce }))
vi.mock('@/editor/ui/toast', () => ({ toast: { error: mocks.toastError, success: vi.fn(), warning: vi.fn() } }))
vi.mock('@editor/features/media-library/stores/media-library-store', () => ({
  useMediaLibraryStore: { getState: () => ({ loadMediaItems: mocks.loadMediaItems, selectMedia: mocks.selectMedia }) },
}))
vi.mock('@editor/features/projects/stores/project-store', () => ({
  useProjectStore: { getState: () => ({ currentProject: { id: 'project' } }) },
}))

import { LinkImportForm } from './link-import-form'

const media: ServerMediaRecord = {
  id: 'media-1', fileName: 'clip.mp4', mime: 'video/mp4', size: 4, contentHash: 'sha256:clip', createdAt: 1,
  probe: { kind: 'video', mime: 'video/mp4', durationSec: 30, width: 1920, height: 1080, fps: 30, videoCodec: 'h264', audioCodec: 'aac', sampleRate: null, channels: null },
}

function submitLink(url = 'https://www.twitch.tv/videos/2899740602') {
  render(<LinkImportForm onImported={vi.fn()} />)
  fireEvent.change(screen.getByLabelText('Link video Twitch hoặc Bilibili'), { target: { value: url } })
  fireEvent.submit(screen.getByRole('button', { name: 'Lấy' }).closest('form')!)
}

beforeEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('link import form', () => {
  it('shows the pending state while the server job runs and completes when it finishes', async () => {
    const imported = Promise.withResolvers<{ media: ServerMediaRecord; reused: boolean }>()
    mocks.importMediaFromLink.mockReturnValue(imported.promise)
    submitLink()
    expect(await screen.findByText('Đang tải video từ link — có thể mất một lúc.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Đang lấy…' }).hasAttribute('disabled')).toBe(true)
    expect(mocks.importMediaFromLink).toHaveBeenCalledWith(expect.objectContaining({ projectId: 'project', url: 'https://www.twitch.tv/videos/2899740602', signal: expect.any(AbortSignal) }))

    imported.resolve({ media, reused: false })
    await waitFor(() => expect(mocks.selectMedia).toHaveBeenCalledWith(['media-1']))
    expect(mocks.loadMediaItems).toHaveBeenCalled()
    expect(mocks.announce).toHaveBeenCalledWith('Đã nhập clip.mp4')
    expect(screen.queryByText('Đang tải video từ link — có thể mất một lúc.')).toBeNull()
  })

  it('keeps the reused copy wording when the file was already in the library', async () => {
    mocks.importMediaFromLink.mockResolvedValue({ media, reused: true })
    submitLink()
    await waitFor(() => expect(mocks.announce).toHaveBeenCalledWith('clip.mp4 đã có trong thư viện'))
  })

  it('shows the server Vietnamese error copy when the job fails', async () => {
    mocks.importMediaFromLink.mockRejectedValue(new Error('Đoạn cắt tối đa 10 phút'))
    submitLink()
    await waitFor(() => expect(mocks.toastError).toHaveBeenCalledWith('Đoạn cắt tối đa 10 phút'))
    expect(screen.getByRole('button', { name: 'Lấy' })).toBeTruthy()
  })

  it('aborts the job when Huỷ is pressed', async () => {
    mocks.importMediaFromLink.mockImplementation(({ signal }: { signal?: AbortSignal }) => {
      const { promise, reject } = Promise.withResolvers<never>()
      signal?.addEventListener('abort', () => reject(new DOMException('Đã huỷ', 'AbortError')))
      return promise
    })
    submitLink()
    fireEvent.click(await screen.findByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(mocks.announce).toHaveBeenCalledWith('Đã huỷ lấy video'))
    expect(mocks.toastError).not.toHaveBeenCalled()
  })
})
