"use client";
import { useRef } from 'react';
import { Flag, Trash2 } from 'lucide-react';
import { useTimelineStore } from '@editor/features/timeline/stores/timeline-store';
import { removeMarker, updateMarker } from '@editor/features/timeline/stores/timeline-actions';
import { useSelectionStore } from '@editor/shared/state/selection';
import { usePlaybackStore } from '@editor/shared/state/playback';
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { EditorIconButton, NumberField } from '@/editor/ui/controls';
import { announce } from '@/editor/ui/shell/announcer';
import { timecode } from './geometry';

export function MarkerListDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const markers = useTimelineStore((state) => state.markers);
  const fps = useTimelineStore((state) => state.fps);
  const listRef = useRef<HTMLUListElement>(null);
  return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent showCloseButton={false} data-shortcut-scope="timeline" onCloseAutoFocus={(event) => { event.preventDefault(); document.querySelector<HTMLElement>('[aria-label="Vị trí phát"]')?.focus(); }}>
    <DialogHeader><DialogTitle>Danh sách marker</DialogTitle><DialogDescription>Đổi tên, vị trí hoặc đi tới marker trên timeline.</DialogDescription></DialogHeader>
    <ul ref={listRef} className="max-h-96 space-y-3 overflow-y-auto pr-1" tabIndex={0} aria-label="Marker trên timeline">
      {[...markers].sort((a, b) => a.frame - b.frame).map((marker, index) => <li key={marker.id} className="rounded-md border border-border p-3" data-marker-entry-id={marker.id}>
        <div className="flex items-center gap-2"><EditorIconButton icon={Flag} label={`Đi tới marker ${marker.label || index + 1}`} onClick={() => { usePlaybackStore.getState().setCurrentFrame(marker.frame); useSelectionStore.getState().selectMarker(marker.id); onOpenChange(false); announce(`Marker ${marker.label || index + 1}, ${timecode(marker.frame, fps)}`); }} />
          <input className="h-8 min-w-0 flex-1 rounded border border-input bg-background px-2 text-xs focus-visible:outline-2 focus-visible:outline-ring" aria-label={`Tên marker ${index + 1}`} defaultValue={marker.label ?? ''} placeholder={`Marker ${index + 1}`} onBlur={(event) => { if (event.target.value !== marker.label) { updateMarker(marker.id, { label: event.target.value }); announce(`Marker ${event.target.value || index + 1}`); } }} onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); }} />
          <EditorIconButton icon={Trash2} label={`Xoá marker ${marker.label || index + 1}`} onClick={() => { removeMarker(marker.id); useSelectionStore.getState().selectMarker(null); requestAnimationFrame(() => listRef.current?.querySelector<HTMLButtonElement>('button')?.focus()); announce('Đã xoá marker'); }} />
        </div>
        <div className="mt-2 flex items-center justify-between gap-4"><span className="font-mono text-xs tabular-nums text-muted-foreground">{timecode(marker.frame, fps)}</span><NumberField label={`Vị trí marker ${index + 1} (khung)`} value={marker.frame} min={0} step={1} onChange={(frame) => updateMarker(marker.id, { frame })} /></div>
      </li>)}
      {!markers.length && <li className="py-6 text-center text-sm text-muted-foreground">Chưa có marker. Nhấn M trên timeline để thêm.</li>}
    </ul>
    <DialogFooter><DialogClose asChild><Button variant="secondary">Đóng</Button></DialogClose></DialogFooter>
  </DialogContent></Dialog>;
}
