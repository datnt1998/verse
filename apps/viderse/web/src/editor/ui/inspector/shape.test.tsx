import { Children, isValidElement, type ReactNode } from 'react'
import { beforeEach, describe, expect, it } from 'vitest'
import { getCommandExecutor } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { makeTimelineTrack, resetTimelineCompositionTestState } from '@editor/features/timeline/test-helpers'
import type { ShapeItem } from '@editor/types/timeline'
import { ShapeSection } from './shape'

const shape: ShapeItem = {
  id: 'shape-1', type: 'shape', trackId: 'v1', from: 0, durationInFrames: 60,
  label: 'Hình', shapeType: 'rectangle', fillColor: '#112233',
}

type ControlProps = {
  children?: ReactNode
  label?: string
  'aria-label'?: string
  property?: string
  value?: number | string
  onChange?: (value: number) => void
  onValueChange?: (value: string) => void
  onCheckedChange?: (value: boolean) => void
}

// Inspect the section's element contract without mounting controls or mocking
// history: edit callbacks below execute real item.update commands.
function controls(item: ShapeItem) {
  const entries: { type: unknown; props: ControlProps }[] = []
  function visit(node: ReactNode) {
    Children.forEach(node, child => {
      if (!isValidElement<ControlProps>(child)) return
      entries.push({ type: child.type, props: child.props })
      visit(child.props.children)
    })
  }
  visit(ShapeSection({ item }))
  return entries
}

function field(item: ShapeItem, label: string) {
  const entry = controls(item).find(({ props }) => props['aria-label'] === label || (props.label === label && (props.onChange || props.onValueChange)))
  if (!entry) throw new Error(`Missing control: ${label}`)
  return entry.props
}

function storedShape(): ShapeItem {
  const item = useItemsStore.getState().items.find(item => item.id === shape.id)
  if (item?.type !== 'shape') throw new Error('Missing shape fixture')
  return item
}

describe('ShapeSection', () => {
  beforeEach(() => {
    resetTimelineCompositionTestState()
    getCommandExecutor().clearHistory()
    useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'v1', name: 'Hình', kind: 'video', order: 0 })])
    useItemsStore.getState().setItems([shape])
  })

  it('updates real geometry while preserving fill, and supports undo', () => {
    field(shape, 'Bo góc hình').onChange!(24)
    expect(storedShape()).toMatchObject({ cornerRadius: 24, fillColor: '#112233', shapeType: 'rectangle' })
    getCommandExecutor().undo()
    expect(storedShape().cornerRadius).toBeUndefined()
  })

  it('enables visible stroke in one undoable edit without dropping other style', () => {
    field(shape, 'Bật đường viền').onCheckedChange!(true)
    expect(storedShape()).toMatchObject({ strokeEnabled: true, strokeWidth: 1, strokeColor: '#000000', fillColor: '#112233' })
    getCommandExecutor().undo()
    expect(storedShape().strokeEnabled).toBeUndefined()
  })


})
