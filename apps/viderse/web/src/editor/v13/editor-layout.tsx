import { useImperativeHandle, useMemo, useRef, type KeyboardEvent, type ReactNode, type Ref } from 'react'
import * as stylex from '@stylexjs/stylex'
import { useDefaultLayout, type PanelImperativeHandle } from 'react-resizable-panels'
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '../vendor/components/ui/resizable'
import { createLayoutStorage } from './editor-layout-storage'
import { sx } from './editor.stylex'

export interface EditorLayoutHandle { reset: () => void }
interface Props {
  projectId: string
  ref: Ref<EditorLayoutHandle>
  locked: boolean
  rail: ReactNode
  library: ReactNode
  preview: ReactNode
  inspector: ReactNode
  timeline: ReactNode
}
const horizontalIds = ['library', 'preview', 'inspector']
const verticalIds = ['workspace', 'timeline']
const defaults = () => ({ library: window.innerWidth >= 1280 ? 316 : 240, inspector: window.innerWidth >= 1280 ? 316 : 268, timeline: 286 })

export function EditorLayout({ projectId, ref, locked, rail, library, preview, inspector, timeline }: Props) {
  const storage = useMemo(() => ({
    horizontal: createLayoutStorage(horizontalIds, () => window.localStorage),
    vertical: createLayoutStorage(verticalIds, () => window.localStorage),
  }), [])
  const horizontal = useDefaultLayout({ id: `viderse-v13-${projectId}-horizontal-v1`, storage: storage.horizontal, onlySaveAfterUserInteractions: false })
  const vertical = useDefaultLayout({ id: `viderse-v13-${projectId}-vertical-v1`, storage: storage.vertical, onlySaveAfterUserInteractions: false })
  const libraryRef = useRef<PanelImperativeHandle>(null)
  const inspectorRef = useRef<PanelImperativeHandle>(null)
  const timelineRef = useRef<PanelImperativeHandle>(null)
  const host = useRef<HTMLDivElement>(null)
  const initial = defaults()
  const reset = () => {
    const sizes = defaults()
    // Shrink both sides first so the other panel's constraint cannot block reset.
    libraryRef.current?.resize(240); inspectorRef.current?.resize(268)
    libraryRef.current?.resize(sizes.library); inspectorRef.current?.resize(sizes.inspector)
    timelineRef.current?.resize(sizes.timeline)
  }
  useImperativeHandle(ref, () => ({ reset }))
  const resizeKey = (event: KeyboardEvent<HTMLDivElement>, panel: PanelImperativeHandle | null, axis: 'horizontal' | 'vertical', initialSize: number, reverse = false) => {
    if (event.altKey || event.ctrlKey || event.metaKey || !panel) return
    const negative = axis === 'horizontal' ? 'ArrowLeft' : 'ArrowUp'
    const positive = axis === 'horizontal' ? 'ArrowRight' : 'ArrowDown'
    if (![negative, positive, 'Home'].includes(event.key)) return
    event.preventDefault(); event.stopPropagation()
    panel.resize(event.key === 'Home' ? initialSize : panel.getSize().inPixels + (event.key === negative ? -1 : 1) * (reverse ? -1 : 1) * (event.shiftKey ? 32 : 8))
  }
  const help = 'Kéo hoặc dùng phím mũi tên để đổi kích thước; Shift để đổi nhanh; Home để khôi phục.'
  return <div ref={host} {...stylex.props(sx.layoutHost)} inert={locked || undefined}>
    <ResizablePanelGroup orientation="vertical" {...stylex.props(sx.layoutGroup)} defaultLayout={vertical.defaultLayout} onLayoutChanged={(layout, meta) => {
      if (host.current?.getBoundingClientRect().height) vertical.onLayoutChanged(layout, meta)
    }} resizeTargetMinimumSize={{ fine: 12, coarse: 44 }}>
      <ResizablePanel id="workspace" minSize={220} {...stylex.props(sx.layoutPanel)}>
        <div {...stylex.props(sx.body, sx.workspace)}>
          {rail}
          <ResizablePanelGroup orientation="horizontal" {...stylex.props(sx.horizontalGroup)} defaultLayout={horizontal.defaultLayout} onLayoutChanged={(layout, meta) => {
            if (host.current?.getBoundingClientRect().width) horizontal.onLayoutChanged(layout, meta)
          }} resizeTargetMinimumSize={{ fine: 12, coarse: 44 }}>
            <ResizablePanel id="library" panelRef={libraryRef} defaultSize={initial.library} minSize={240} maxSize={480} groupResizeBehavior="preserve-pixel-size" {...stylex.props(sx.layoutPanel)}>{library}</ResizablePanel>
            <ResizableHandle disableDoubleClick aria-label="Chiều rộng bảng nội dung" title={help} {...stylex.props(sx.resizeHandle, sx.resizeHorizontal)} onKeyDownCapture={event => resizeKey(event, libraryRef.current, 'horizontal', defaults().library)} />
            <ResizablePanel id="preview" minSize={400} {...stylex.props(sx.layoutPanel)}><div {...stylex.props(sx.previewHost)}>{preview}</div></ResizablePanel>
            <ResizableHandle disableDoubleClick aria-label="Chiều rộng bảng thuộc tính" title={help} {...stylex.props(sx.resizeHandle, sx.resizeHorizontal)} onKeyDownCapture={event => resizeKey(event, inspectorRef.current, 'horizontal', defaults().inspector, true)} />
            <ResizablePanel id="inspector" panelRef={inspectorRef} defaultSize={initial.inspector} minSize={268} maxSize={440} groupResizeBehavior="preserve-pixel-size" {...stylex.props(sx.layoutPanel)}><div {...stylex.props(sx.inspectorHost)}>{inspector}</div></ResizablePanel>
          </ResizablePanelGroup>
        </div>
      </ResizablePanel>
      <ResizableHandle disableDoubleClick aria-label="Chiều cao dòng thời gian" title={help} {...stylex.props(sx.resizeHandle, sx.resizeVertical)} onKeyDownCapture={event => resizeKey(event, timelineRef.current, 'vertical', 286, true)} />
      <ResizablePanel id="timeline" panelRef={timelineRef} defaultSize={286} minSize={140} maxSize="65%" groupResizeBehavior="preserve-pixel-size" {...stylex.props(sx.layoutPanel)}><div data-shortcut-scope="timeline" {...stylex.props(sx.timelineHost)}>{timeline}</div></ResizablePanel>
    </ResizablePanelGroup>
  </div>
}
