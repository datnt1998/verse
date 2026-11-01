import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import type { MediaMetadata } from '@editor/types/storage'
import { ContentPanels } from './ContentPanels'

const originalMedia = useMediaLibraryStore.getState().mediaItems
const image: MediaMetadata = {
  id: 'scene-image', storageType: 'workspace', fileName: 'scene.png', fileSize: 128,
  mimeType: 'image/png', duration: 0, width: 1280, height: 720, fps: 0,
  codec: 'png', bitrate: 0, tags: [], createdAt: 0, updatedAt: 0,
}

beforeEach(() => {
  useMediaLibraryStore.setState({ mediaItems: [image, { ...image, id: 'overlay-image', fileName: 'overlay.png', overlay: true }] })
})
afterEach(() => {
  cleanup()
  useMediaLibraryStore.setState({ mediaItems: originalMedia })
})

describe('editor media type filtering', () => {
  it('separates scene images from overlays and changes the actual media choices', async () => {
    render(<ContentPanels panel="media" />)
    const group = within(screen.getByRole('radiogroup', { name: 'Loại media' }))
    expect(group.getByRole('radio', { name: 'Ảnh 1' })).toHaveAttribute('aria-checked', 'true')
    expect(await screen.findByRole('button', { name: 'Thêm scene.png vào video tại vị trí đang xem' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Thêm overlay.png vào video tại vị trí đang xem' })).toBeNull()

    fireEvent.click(group.getByRole('radio', { name: 'Phủ 1' }))
    expect(group.getByRole('radio', { name: 'Phủ 1' })).toHaveAttribute('aria-checked', 'true')
    expect(await screen.findByRole('button', { name: 'Thêm overlay.png vào video tại vị trí đang xem' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Thêm scene.png vào video tại vị trí đang xem' })).toBeNull()
  })

  it('supports keyboard filtering, empty kinds and a single roving focus target', async () => {
    render(<ContentPanels panel="media" />)
    const group = within(screen.getByRole('radiogroup', { name: 'Loại media' }))
    const images = group.getByRole('radio', { name: 'Ảnh 1' })
    images.focus()
    fireEvent.keyDown(images, { key: 'ArrowRight' })
    const video = group.getByRole('radio', { name: 'Nền 0' })
    expect(video).toHaveFocus()
    expect(video).toHaveAttribute('aria-checked', 'true')
    expect(screen.queryByRole('button', { name: /^Thêm /u })).toBeNull()

    fireEvent.keyDown(video, { key: 'End' })
    const audio = group.getByRole('radio', { name: 'Âm 0' })
    expect(audio).toHaveFocus()
    expect(audio).toHaveAttribute('aria-checked', 'true')
    fireEvent.keyDown(audio, { key: 'Home' })
    expect(images).toHaveFocus()
    expect(group.getAllByRole('radio').filter(radio => radio.tabIndex === 0)).toEqual([images])
    expect(await screen.findByRole('button', { name: 'Thêm scene.png vào video tại vị trí đang xem' })).toBeInTheDocument()
  })
})
