import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from '@/components/ui/context-menu'

async function openMenu(onSelect: () => void) {
  render(
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <button type="button">clip</button>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem onSelect={onSelect}>Xoá</ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>,
  )
  fireEvent.contextMenu(screen.getByRole('button', { name: 'clip' }))
  return screen.findByRole('menuitem', { name: 'Xoá' })
}

describe('context menu item activation', () => {
  it('ignores the release of the right-click that opened the menu, even when it lands on an item', async () => {
    const onSelect = vi.fn()
    const item = await openMenu(onSelect)

    fireEvent.pointerUp(item, { pointerType: 'mouse', button: 2 })

    expect(onSelect).not.toHaveBeenCalled()
  })

  it('activates an item after a deliberate press and release on it', async () => {
    const onSelect = vi.fn()
    const item = await openMenu(onSelect)

    fireEvent.pointerDown(item, { pointerType: 'mouse', button: 0 })
    fireEvent.pointerUp(item, { pointerType: 'mouse', button: 0 })
    fireEvent.click(item)

    expect(onSelect).toHaveBeenCalledTimes(1)
  })

  it('does not count a press that moved off the item before release', async () => {
    const onSelect = vi.fn()
    const item = await openMenu(onSelect)

    fireEvent.pointerDown(item, { pointerType: 'mouse', button: 0 })
    fireEvent.pointerLeave(item, { pointerType: 'mouse' })
    fireEvent.pointerUp(item, { pointerType: 'mouse', button: 0 })

    expect(onSelect).not.toHaveBeenCalled()
  })

  it('activates with the keyboard', async () => {
    const onSelect = vi.fn()
    const item = await openMenu(onSelect)

    item.focus()
    fireEvent.keyDown(item, { key: 'Enter' })

    expect(onSelect).toHaveBeenCalledTimes(1)
  })
})
