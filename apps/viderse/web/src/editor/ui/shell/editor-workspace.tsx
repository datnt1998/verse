"use client";

import { memo, useRef, type KeyboardEvent, type ReactNode } from "react";
import { useDefaultLayout, type PanelImperativeHandle } from "react-resizable-panels";
import { LayoutPanelLeft, PanelLeftClose, PanelRightClose, RotateCcw } from "lucide-react";
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { EditorIconButton } from "@/editor/ui/controls";
import { LibraryPanel } from "@/editor/ui/library";
import { PreviewPanel } from "@/editor/ui/preview";
import { InspectorPanel } from "@/editor/ui/inspector";
import { TimelinePanel } from "@/editor/ui/timeline";
import { useShortcutAction } from "@/editor/ui/shortcuts";
import { TemplateBanner } from "@/editor/ui/template/template-banner";
import { announce } from "./announcer";

const panelIds = ["library", "preview", "inspector"];
const verticalIds = ["workspace", "timeline"];
// Each panel reads its own stores. Memoized, a header re-render (dirty flag, autosave status) stays in the
// header instead of re-rendering the whole editor every few seconds while the owner edits.
const Library = memo(LibraryPanel);
const Preview = memo(PreviewPanel);
const Inspector = memo(InspectorPanel);
const Timeline = memo(TimelinePanel);

export function EditorWorkspace({ projectId, header }: { projectId: string; header: (layoutControl: ReactNode) => ReactNode }) {
  const horizontal = useDefaultLayout({ id: `ytf-editor-${projectId}-horizontal`, panelIds, storage: localStorage, onlySaveAfterUserInteractions: false });
  const vertical = useDefaultLayout({ id: `ytf-editor-${projectId}-vertical`, panelIds: verticalIds, storage: localStorage, onlySaveAfterUserInteractions: false });
  const library = useRef<PanelImperativeHandle>(null);
  const inspector = useRef<PanelImperativeHandle>(null);
  const timeline = useRef<PanelImperativeHandle>(null);
  const workspaceRef = useRef<HTMLDivElement>(null);
  const defaultLibrary = window.innerWidth >= 1280 ? 320 : 240;
  const defaultTimeline = window.innerHeight >= 900 ? 330 : 306;
  const focusPreview = () => workspaceRef.current?.querySelector<HTMLElement>("[data-shell-preview]")?.focus();
  const toggleLibrary = () => {
    if (library.current?.isCollapsed()) { library.current.expand(); announce("Đã mở Thư viện"); }
    else { library.current?.collapse(); announce("Đã đóng panel"); }
    workspaceRef.current?.querySelector<HTMLButtonElement>("[data-shell-library] button")?.focus();
  };
  const toggleInspector = () => {
    if (inspector.current?.isCollapsed()) { inspector.current.expand(); announce("Đã mở Thuộc tính"); }
    else { inspector.current?.collapse(); announce("Đã đóng panel"); focusPreview(); }
  };
  const focusMode = () => {
    if (library.current?.isCollapsed() && inspector.current?.isCollapsed()) { library.current.expand(); inspector.current.expand(); announce("Đã mở các panel"); }
    else { library.current?.collapse(); inspector.current?.collapse(); announce("Chế độ tập trung preview"); }
    focusPreview();
  };
  const reset = () => { library.current?.resize(window.innerWidth >= 1280 ? 320 : 240); inspector.current?.resize(288); timeline.current?.resize(window.innerHeight >= 900 ? 330 : 306); announce("Đã khôi phục bố cục"); };
  useShortcutAction({ id: "toggle-library", label: "Đóng hoặc mở Thư viện", scope: "global", run: toggleLibrary });
  useShortcutAction({ id: "toggle-inspector", label: "Đóng hoặc mở Thuộc tính", scope: "global", run: toggleInspector });
  useShortcutAction({ id: "focus", label: "Tập trung xem trước", scope: "global", run: focusMode });
  const resizeKey = (event: KeyboardEvent<HTMLDivElement>, panel: PanelImperativeHandle | null, name: string, axis: "horizontal" | "vertical", initial: number, reverse = false) => {
    const negative = axis === "horizontal" ? "ArrowLeft" : "ArrowUp";
    const positive = axis === "horizontal" ? "ArrowRight" : "ArrowDown";
    if (![negative, positive, "Home"].includes(event.key) || !panel) return;
    event.preventDefault(); event.stopPropagation();
    const step = event.shiftKey ? 32 : 8;
    const size = event.key === "Home" ? initial : panel.getSize().inPixels + (event.key === negative ? -step : step) * (reverse ? -1 : 1);
    panel.resize(size);
    announce(`${name}: ${Math.round(panel.getSize().inPixels)} pixel`);
  };
  const layoutControl = <DropdownMenu><DropdownMenuTrigger asChild><EditorIconButton icon={LayoutPanelLeft} label="Bố cục panel" /></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem onSelect={toggleLibrary}><PanelLeftClose aria-hidden />Đóng hoặc mở Thư viện</DropdownMenuItem><DropdownMenuItem onSelect={toggleInspector}><PanelRightClose aria-hidden />Đóng hoặc mở Thuộc tính</DropdownMenuItem><DropdownMenuItem onSelect={focusMode}>Tập trung xem trước</DropdownMenuItem><DropdownMenuItem onSelect={reset}><RotateCcw aria-hidden />Khôi phục bố cục</DropdownMenuItem></DropdownMenuContent></DropdownMenu>;
  const verticalPersist: typeof vertical.onLayoutChanged = (layout, meta) => { if (workspaceRef.current?.getBoundingClientRect().height) vertical.onLayoutChanged(layout, meta); };
  const horizontalPersist: typeof horizontal.onLayoutChanged = (layout, meta) => { if (workspaceRef.current?.getBoundingClientRect().width) horizontal.onLayoutChanged(layout, meta); };
  return <div ref={workspaceRef} className="flex h-dvh min-w-0 flex-col overflow-hidden bg-card">
    {header(layoutControl)}
    <TemplateBanner projectId={projectId} />
    <div className="min-h-0 flex-1">
      <ResizablePanelGroup id={`editor-${projectId}-vertical`} orientation="vertical" defaultLayout={vertical.defaultLayout} onLayoutChanged={verticalPersist} resizeTargetMinimumSize={{ fine: 24, coarse: 44 }}>
        <ResizablePanel id="workspace" minSize="20%">
          <ResizablePanelGroup id={`editor-${projectId}-horizontal`} orientation="horizontal" defaultLayout={horizontal.defaultLayout} onLayoutChanged={horizontalPersist} resizeTargetMinimumSize={{ fine: 24, coarse: 44 }}>
            <ResizablePanel id="library" panelRef={library} defaultSize={defaultLibrary} minSize={240} maxSize={560} collapsedSize={44} collapsible className="min-w-0 overflow-hidden"><div className="h-full min-w-0 overflow-hidden" data-shortcut-scope="library" data-shell-library><Library /></div></ResizablePanel>
        <ResizableHandle aria-label="Chiều rộng Thư viện" onKeyDownCapture={(event) => resizeKey(event, library.current, "Thư viện", "horizontal", defaultLibrary)}><span data-hit-area className="absolute inset-y-0 left-1/2 w-6 -translate-x-1/2" /></ResizableHandle>
            <ResizablePanel id="preview" minSize={280} className="min-w-0 overflow-hidden"><div className="h-full min-w-0" data-shortcut-scope="preview" data-shell-preview tabIndex={-1}><Preview /></div></ResizablePanel>
        <ResizableHandle aria-label="Chiều rộng Thuộc tính" onKeyDownCapture={(event) => resizeKey(event, inspector.current, "Thuộc tính", "horizontal", 288, true)}><span data-hit-area className="absolute inset-y-0 left-1/2 w-6 -translate-x-1/2" /></ResizableHandle>
            <ResizablePanel id="inspector" panelRef={inspector} defaultSize={288} minSize={280} maxSize={420} collapsedSize={0} collapsible className="min-w-0 overflow-hidden"><div className="h-full min-w-0 overflow-hidden" data-shortcut-scope="inspector"><Inspector /></div></ResizablePanel>
          </ResizablePanelGroup>
        </ResizablePanel>
        <ResizableHandle aria-label="Chiều cao Timeline" onKeyDownCapture={(event) => resizeKey(event, timeline.current, "Timeline", "vertical", defaultTimeline, true)}><span data-hit-area className="absolute inset-x-0 top-1/2 h-6 -translate-y-1/2" /></ResizableHandle>
        <ResizablePanel id="timeline" panelRef={timeline} defaultSize={defaultTimeline} minSize="14%" maxSize="80%" className="min-h-0 overflow-hidden"><div className="h-full min-w-0" data-shortcut-scope="timeline"><Timeline /></div></ResizablePanel>
      </ResizablePanelGroup>
    </div>
    <footer className="flex h-6 shrink-0 items-center justify-between border-t px-3 text-[10px] text-muted-foreground"><span>Mọi thay đổi được tự động lưu trên máy của bạn</span></footer>
  </div>;
}
