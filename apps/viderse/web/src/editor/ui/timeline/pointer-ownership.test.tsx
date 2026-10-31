import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from '@/components/ui/context-menu'
import { ownsPointerEvent } from './pointer-ownership'

function Lane({ onGesture }: { onGesture: () => void }) {
  return (
    <div
      data-testid="lane"
      onPointerDown={(event) => {
        if (!ownsPointerEvent(event)) return
        onGesture()
      }}
    >
      <ContextMenu>
        <ContextMenuTrigger asChild>
          <button type="button">clip</button>
        </ContextMenuTrigger>
        <ContextMenuContent>
          <ContextMenuItem>Xoá</ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
    </div>
  )
}

describe('timeline lane pointer ownership', () => {
  it('does not start a lane gesture (which would clear the selection) when a context-menu item is pressed', async () => {
    const onGesture = vi.fn()
    render(<Lane onGesture={onGesture} />)

    fireEvent.contextMenu(screen.getByRole('button', { name: 'clip' }))
    const item = await screen.findByRole('menuitem', { name: 'Xoá' })
    fireEvent.pointerDown(item)

    expect(onGesture).not.toHaveBeenCalled()
  })

  it('still starts a gesture when the pointer lands inside the lane', () => {
    const onGesture = vi.fn()
    render(<Lane onGesture={onGesture} />)

    fireEvent.pointerDown(screen.getByRole('button', { name: 'clip' }))
    fireEvent.pointerDown(screen.getByTestId('lane'))

    expect(onGesture).toHaveBeenCalledTimes(2)
  })
})
