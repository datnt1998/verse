"use client";

import { useEffect, useId, useRef, useState, type DragEvent } from 'react';
import { Image } from '../../bridge/navigation';
import { Film, Type, Sparkles, Scissors, Captions, Folder, Upload, Plus, Trash2, Music, ImageIcon, AlertCircle, X, Search, Layers, Link2 } from 'lucide-react';
import { toast } from '@/editor/ui/toast';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem } from '@/components/ui/context-menu';
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel, AlertDialogAction } from '@/components/ui/alert-dialog';
import { EditorIconButton, EditorSelect } from '@/editor/ui/controls';
import { useShortcutAction } from '@/editor/ui/shortcuts';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { announce } from '@/editor/ui/shell/announcer';
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store';
import { useMediaPreparationStore } from '@editor/features/media-library/stores/media-preparation-store';
import { useItemsStore } from '@editor/features/timeline/stores/items-store';
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store';
import { useSelectionStore } from '@editor/shared/state/selection';
import { mediaLibraryService } from '@editor/features/media-library/services/media-library-service';
import { resolveMediaUrl } from '@editor/features/media-library/utils/media-resolver';
import { getMediaType } from '@editor/features/media-library/utils/validation';
import { setMediaDragData, clearMediaDragData } from '@editor/features/media-library/utils/drag-data-cache';
import { transitionRegistry } from '@editor/shared/timeline/transitions';
import { parseCaptionTiming } from '@editor/features/captions/parsers';
import { CAPTION_ANIMATION_PRESETS } from '@editor/donor/openreel/text/text-animation-presets';
import { executeCommands, captureProjectState } from '@editor/commands';
import type { MediaMetadata } from '@editor/types/storage';
import { getBuiltInEditingTemplates } from '@editor/features/editing-templates/registry';
import { useProjectStore } from '@editor/features/projects/stores/project-store';
import { usePlaybackStore } from '@editor/shared/state/playback';
import { insertMedia, insertText, applyLibraryDrop, LIBRARY_DRAG_MIME, type LibraryDrop } from './library-actions';
import { uploadKindForMime, uploadTooLarge } from '../../../api/upload-limits';
import { OPEN_LIBRARY_TAB_EVENT } from '@/editor/ui/inspector/effects';
import { EffectsTab } from './effects-tab';
import { OverlaysTab } from './overlays-tab';
import { LinkImportForm } from './link-import-form';
import { transitionNames } from '@/editor/ui/inspector/transitions';

const tabs = [
  { id: 'media', label: 'Media', icon: Film, key: '1' },
  { id: 'text', label: 'Chữ', icon: Type, key: '2' },
  { id: 'effects', label: 'Hiệu ứng', icon: Sparkles, key: '4' },
  { id: 'transitions', label: 'Chuyển cảnh', icon: Scissors, key: '5' },
  { id: 'captions', label: 'Phụ đề', icon: Captions, key: '7' },
  { id: 'overlays', label: 'Lớp phủ', icon: Layers, key: '6' },
  { id: 'templates', label: 'Mẫu', icon: Folder, key: '8' },
] as const;
type TabId = typeof tabs[number]['id'];
const smallButton = 'hit-area min-h-7 rounded-md border border-ring/50 bg-card px-2 text-xs';
const showError = (error: unknown) => toast.error(error instanceof Error ? error.message : 'Không thực hiện được. Thử lại.');

export function MediaThumbnail({ media }: { media: MediaMetadata }) {
  const [src, setSrc] = useState(''); const [failed, setFailed] = useState(false);
  const kind = getMediaType(media.mimeType);
  useEffect(() => { let active = true; const resolve = async () => { const thumbnail = kind === 'audio' ? null : await mediaLibraryService.getThumbnailBlobUrl(media.id, media.thumbnailId); const url = thumbnail ?? (kind === 'image' ? await resolveMediaUrl(media.id) : ''); if (active) { setSrc(url); setFailed(false); } }; void resolve().catch(() => { if (active) setFailed(true); }); return () => { active = false; }; }, [media.id, media.contentHash, media.thumbnailId, kind]);
  const Icon = kind === 'audio' ? Music : kind === 'image' ? ImageIcon : Film;
  const kindLabel = kind === 'audio' ? 'Audio' : kind === 'image' ? 'Ảnh' : kind === 'video' ? 'Video' : 'File';
  return <div className="relative flex aspect-video w-full items-center justify-center overflow-hidden bg-muted text-muted-foreground">
    {src && !failed ? <Image src={src} alt="" width={320} height={180} unoptimized className="size-full object-contain" onError={() => setFailed(true)} /> : <Icon aria-hidden className="size-6" />}
    <span className="absolute left-1 top-1 flex items-center gap-1 rounded bg-background/90 px-1 py-0.5 text-[10px]"><Icon aria-hidden className="size-3" />{kindLabel}</span>
    {media.duration > 0 && <span className="absolute bottom-1 right-1 rounded bg-background/90 px-1 text-[10px] tabular-nums">{Math.floor(media.duration / 60).toString().padStart(2, '0')}:{Math.floor(media.duration % 60).toString().padStart(2, '0')}</span>}
  </div>;
}

function TabShortcut({ tab, choose }: { tab: typeof tabs[number]; choose: (id: TabId) => void }) {
  useShortcutAction({ id: `library-tab-${tab.id}`, label: tab.label, scope: 'global', bindings: [`ctrl+Digit${tab.key}`], run: () => choose(tab.id) }); return null;
}

export function LibraryPanel() {
  const [tab, setTab] = useState<TabId>('media'); const [query, setQuery] = useState('');
  const searchInputId = useId();
  const [filter, setFilter] = useState('all'); const [dropping, setDropping] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<MediaMetadata | null>(null);
  const [uploads, setUploads] = useState<{ id: string; name: string; progress: number; controller: AbortController; processing: boolean }[]>([]);
  const picker = useRef<HTMLInputElement>(null); const replacer = useRef<HTMLInputElement>(null); const captionsPicker = useRef<HTMLInputElement>(null); const replaceId = useRef<string | null>(null);
  const cards = useRef<Record<string, HTMLButtonElement | null>>({}); const tabButtons = useRef<Record<string, HTMLButtonElement | null>>({});
  const importTrigger = useRef<HTMLButtonElement>(null);
  const mediaItems = useMediaLibraryStore(s => s.mediaItems); const selected = useMediaLibraryStore(s => s.selectedMediaIds); const broken = useMediaLibraryStore(s => s.brokenMediaIds);
  const tasks = useMediaPreparationStore(s => s.tasks); const fps = useTimelineSettingsStore(s => s.fps);
  const choose = (id: TabId) => { setTab(id); setQuery(''); requestAnimationFrame(() => tabButtons.current[id]?.focus()); };
  const replace = (id: string) => { replaceId.current = id; replacer.current?.click(); };
  const add = (media: MediaMetadata) => { void insertMedia(media).catch(showError); };
  useEffect(() => {
    const request = () => picker.current?.click();
    window.addEventListener('ytf-import-media', request);
    return () => window.removeEventListener('ytf-import-media', request);
  }, []);
  useEffect(() => {
    const open = (event: Event) => { const id = event instanceof CustomEvent ? tabs.find(t => t.id === event.detail)?.id : undefined; if (id) { setTab(id); setQuery(''); } };
    window.addEventListener(OPEN_LIBRARY_TAB_EVENT, open);
    return () => window.removeEventListener(OPEN_LIBRARY_TAB_EVENT, open);
  }, []);
  const importFiles = async (files: File[], replacing?: string) => {
    for (const file of files) {
      // Qua internet, tệp vượt trần bị edge chặn bằng trang 413 HTML: tự chặn
      // trước và nói rõ trần thay vì để request chết giữa đường.
      const tooLarge = await uploadTooLarge(uploadKindForMime(file.type), file.size);
      if (tooLarge) { showError(new Error(tooLarge)); continue; }
      const id = crypto.randomUUID(); const controller = new AbortController();
      setUploads(current => [...current, { id, name: file.name, progress: 0, controller, processing: true }]);
      try {
        const options = { signal: controller.signal, onProgress: (sent: number, total: number) => setUploads(current => current.map(u => u.id === id ? { ...u, progress: total ? sent / total : 0, processing: sent === total } : u)) };
        const imported: MediaMetadata & { isDuplicate?: boolean; hasUnsupportedCodec?: boolean } = replacing ? await mediaLibraryService.relinkMediaFile(replacing, file, options) : await mediaLibraryService.importFile(file, { ...options, projectId: useProjectStore.getState().currentProject?.id });
        const { isDuplicate, hasUnsupportedCodec, ...media } = imported;
        if (replacing) {
          if (!media.contentHash) throw new Error('Media chưa có phiên bản nội dung');
          const src = `/api/editor/media/${encodeURIComponent(media.id)}?v=${encodeURIComponent(media.contentHash)}`;
          const result = executeCommands([{ type: 'media.refreshSource', params: { media, src } }]);
          if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '));
          await useMediaLibraryStore.getState().loadMediaItems();
        }
        else if (!isDuplicate) useMediaLibraryStore.getState().prependMediaItem(media);
        useMediaLibraryStore.getState().selectMedia([media.id]);
        announce(replacing ? 'Đã thay file' : isDuplicate ? `${file.name} đã có trong thư viện` : `Đã nhập ${file.name}`);
        if (hasUnsupportedCodec) toast.warning('Media có codec trình duyệt không đọc được. Hãy thay file.');
        requestAnimationFrame(() => cards.current[media.id]?.focus());
      } catch (error) { if (!controller.signal.aborted) showError(error); else announce('Đã huỷ nhập media'); }
      finally { setUploads(current => current.filter(u => u.id !== id)); }
    }
  };
  const remove = (media: MediaMetadata) => {
    const index = mediaItems.findIndex(m => m.id === media.id); const next = mediaItems[index + 1] ?? mediaItems[index - 1];
    void useMediaLibraryStore.getState().deleteMedia(media.id).then(() => { announce(`Đã xoá ${media.fileName}`); requestAnimationFrame(() => { const nextCard = next ? cards.current[next.id] : null; (nextCard ?? importTrigger.current)?.focus(); }); }).catch(showError);
    setPendingDelete(null);
  };
  const requestDelete = (media: MediaMetadata) => { const state = captureProjectState(); const scopes = [state.items, ...state.compositions.map(composition => composition.items), ...(state.stashStack ?? []).map(stash => stash.items), ...(state.mainHolder ? [state.mainHolder.items] : [])]; if (scopes.some(items => items.some(item => item.mediaId === media.id))) setPendingDelete(media); else remove(media); };
  useShortcutAction({ id: 'import', label: 'Nhập media', scope: 'global', run: () => picker.current?.click() });
  useShortcutAction({ id: 'delete', label: 'Xoá media khỏi thư viện', scope: 'library', bindings: ['Delete'], enabled: tab === 'media' && !!selected[0], run: () => { const media = mediaItems.find(m => m.id === selected[0]); if (media) requestDelete(media); } });
  const drop = (event: DragEvent<HTMLElement>) => { event.preventDefault(); setDropping(false); const files = Array.from(event.dataTransfer.files); if (files.length) void importFiles(files); };
  const startMediaDrag = (event: DragEvent<HTMLButtonElement>, media: MediaMetadata) => { const data = { type: 'media-item' as const, mediaId: media.id, mediaType: getMediaType(media.mimeType), fileName: media.fileName, duration: media.duration }; setMediaDragData(data); event.dataTransfer.effectAllowed = 'copy'; event.dataTransfer.setData('application/json', JSON.stringify(data)); };
  const startDrop = (event: DragEvent<HTMLButtonElement>, data: LibraryDrop) => { event.dataTransfer.effectAllowed = 'copy'; event.dataTransfer.setData(LIBRARY_DRAG_MIME, JSON.stringify(data)); };
  const apply = (data: LibraryDrop) => { try { applyLibraryDrop(data); } catch (error) { showError(error); } };
  const importCaptions = async (file: File) => {
    try {
      const extension = file.name.split('.').at(-1)?.toLowerCase(); const format = extension === 'srt' ? 'srt' : extension === 'vtt' ? 'vtt' : 'json3';
      const data = parseCaptionTiming(await file.text(), format, fps);
      if (!data.words.length) throw new Error('File phụ đề không có nội dung');
      const itemId = insertText({ label: file.name, textRole: 'caption', text: data.words.map(w => w.text).join(' '), durationInFrames: data.durationInFrames, transform: { y: 400, width: 1600, height: 160 }, fontSize: 52 });
      const result = executeCommands([{ type: 'caption.importTimed', params: { itemId, data } }]);
      if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '));
      announce(`Đã nhập phụ đề${data.estimated ? ', mốc từ ước lượng' : ''}`);
    } catch (error) { showError(error); }
  };
  const results = mediaItems.filter(m => m.fileName.toLocaleLowerCase('vi').includes(query.toLocaleLowerCase('vi')) && (filter === 'all' || getMediaType(m.mimeType) === filter));
    const transitions = transitionRegistry.getDefinitions().filter(t => t.id in transitionNames && transitionNames[t.id].toLocaleLowerCase('vi').includes(query.toLocaleLowerCase('vi')));
  return <aside aria-label="Thư viện" data-shortcut-scope="library" className="grid h-full min-h-0 min-w-0 grid-cols-[44px_minmax(0,1fr)] bg-card text-xs">
    <div role="toolbar" aria-label="Panel thư viện" className="flex flex-col items-center gap-1 overflow-y-auto border-r border-border py-1">{tabs.map(t => <div key={t.id} data-tour={t.id === 'captions' ? 'editor-captions-tab' : `editor-library-${t.id}`}><TabShortcut tab={t} choose={choose} /><EditorIconButton ref={node => { tabButtons.current[t.id] = node; }} icon={t.icon} label={t.label} shortcut={`Ctrl+${t.key}`} aria-pressed={tab === t.id} onClick={() => choose(t.id)} className="size-9" /></div>)}</div>
    <div className={`flex min-h-0 min-w-0 flex-col overflow-hidden ${dropping ? 'ring-2 ring-inset ring-ring' : ''}`} onDragOver={e => { if (e.dataTransfer.types.includes('Files')) { e.preventDefault(); setDropping(true); } }} onDragLeave={e => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setDropping(false); }} onDrop={drop}>
      <header className="flex min-h-10 items-center justify-between gap-1 px-3"><h2 className="text-xs font-semibold">{tab === 'media' ? 'Thư viện' : tabs.find(t => t.id === tab)?.label}</h2>{tab === 'media' && <div className="flex items-center gap-1"><EditorIconButton icon={Link2} label="Lấy từ link" aria-pressed={linkOpen} onClick={() => setLinkOpen(open => !open)} /><EditorIconButton ref={importTrigger} icon={Upload} label="Nhập media" shortcut="⌘I" onClick={() => picker.current?.click()} /></div>}</header>
      <input ref={picker} type="file" accept="video/*,audio/*,image/*" multiple className="hidden" aria-label="Chọn media" onChange={e => { void importFiles(Array.from(e.target.files ?? [])); e.target.value = ''; }} />
      <input ref={replacer} type="file" accept="video/*,audio/*,image/*" className="hidden" aria-label="Chọn file thay thế" onChange={e => { const file = e.target.files?.[0]; if (file && replaceId.current) void importFiles([file], replaceId.current); e.target.value = ''; }} />
      <input ref={captionsPicker} type="file" accept=".srt,.vtt,.json3,.json" className="hidden" aria-label="Chọn phụ đề" onChange={e => { const file = e.target.files?.[0]; if (file) void importCaptions(file); e.target.value = ''; }} />
      {(tab === 'media' || tab === 'effects' || tab === 'transitions' || tab === 'overlays') && <div className="relative mx-3 mb-2"><Search aria-hidden className="pointer-events-none absolute left-2 top-2 size-3.5 text-muted-foreground" /><label htmlFor={searchInputId} className="hit-area absolute -inset-y-1 inset-x-0"><span className="sr-only">{`Tìm ${tab === 'media' ? 'media' : tab === 'effects' ? 'hiệu ứng' : tab === 'overlays' ? 'lớp phủ' : 'chuyển cảnh'}`}</span></label><Input id={searchInputId} aria-label={`Tìm ${tab === 'media' ? 'media' : tab === 'effects' ? 'hiệu ứng' : tab === 'overlays' ? 'lớp phủ' : 'chuyển cảnh'}`} placeholder="Tìm…" value={query} onChange={e => setQuery(e.target.value)} className="h-8 border-ring/60 pl-7 text-xs" /></div>}
      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-3">
        {tab === 'media' && <>{linkOpen && <div className="mb-3"><LinkImportForm onImported={id => requestAnimationFrame(() => cards.current[id]?.focus())} /></div>}<div className="@container mb-3"><div className="grid min-w-0 grid-cols-2 gap-1 @[16rem]:grid-cols-4">{[['all', 'Tất cả'], ['video', 'Video'], ['audio', 'Audio'], ['image', 'Ảnh']].map(([id, name]) => <button key={id} type="button" className={`${smallButton} min-w-0 truncate px-1 aria-pressed:border-primary`} aria-pressed={filter === id} onClick={() => setFilter(id)}>{name}</button>)}</div></div>
          {uploads.map(u => <div key={u.id} role="status" className="mb-2 rounded-md border border-ring/50 p-2"><div className="flex items-center gap-1"><span className="min-w-0 flex-1 break-all">{u.processing ? 'Đang xử lý' : 'Đang nhập'} {u.name}</span><EditorIconButton icon={X} label={`Huỷ nhập ${u.name}`} onClick={() => u.controller.abort()} /></div><progress aria-label={`Tiến độ ${u.name}`} max="1" value={u.progress} className="h-1 w-full accent-primary" /></div>)}
          {results.length ? <div className="grid grid-cols-2 gap-2">{results.map(media => {
            const missing = broken.includes(media.id); const codecError = media.videoCodecSupported === false || media.audioCodecSupported === false;
            const processing = Array.from(tasks.values()).find(t => t.mediaId === media.id);
            return <ContextMenu key={media.id}><ContextMenuTrigger asChild><div className={`min-w-0 overflow-hidden rounded-md border bg-background transition-colors has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring ${selected.includes(media.id) ? 'border-primary ring-1 ring-primary' : 'border-border'}`}><button ref={node => { cards.current[media.id] = node; }} type="button" draggable={!missing && !codecError} onDragStart={e => startMediaDrag(e, media)} onDragEnd={clearMediaDragData} aria-pressed={selected.includes(media.id)} className="block w-full text-left focus-visible:outline-none" onClick={() => useMediaLibraryStore.getState().selectMedia([media.id])} onDoubleClick={() => add(media)}><MediaThumbnail media={media} /><span className="block truncate px-1.5 py-1 text-[10px]" title={media.fileName}>{media.fileName}</span></button>
              {missing || codecError ? <div className="border-t border-border/60 px-1.5 py-1"><span className="flex items-center gap-1 text-[10px] text-muted-foreground"><AlertCircle aria-hidden className="size-3 shrink-0" />{missing ? 'Thiếu file' : 'Không đọc được codec'}</span><button type="button" className={`${smallButton} mt-1 w-full`} onClick={() => replace(media.id)}>Thay file</button></div> : <div className="flex justify-end gap-1 border-t border-border/60 px-1 py-0.5"><EditorIconButton icon={Plus} label={`Thêm ${media.fileName} vào timeline tại playhead`} onClick={() => add(media)} /><EditorIconButton icon={Trash2} label={`Xoá ${media.fileName}`} onClick={() => requestDelete(media)} /></div>}
              {processing && <p className="px-1 py-1 text-[10px] text-muted-foreground">{processing.status === 'error' ? 'Xử lý lỗi' : `Đang xử lý ${Math.round(processing.progress * 100)}%`}</p>}
            </div></ContextMenuTrigger><ContextMenuContent><ContextMenuItem onSelect={() => add(media)} disabled={missing || codecError}>Thêm vào timeline tại playhead</ContextMenuItem><ContextMenuItem onSelect={() => replace(media.id)}>Thay file</ContextMenuItem><ContextMenuItem onSelect={() => requestDelete(media)}>Xoá khỏi thư viện</ContextMenuItem></ContextMenuContent></ContextMenu>;
          })}</div> : <div className="flex flex-col items-center gap-2 py-8 text-center text-muted-foreground"><Folder aria-hidden className="size-7" /><p>{mediaItems.length ? 'Không có kết quả' : 'Chưa có media'}</p><Button variant="outline" size="sm" onClick={() => mediaItems.length ? (setQuery(''), setFilter('all')) : picker.current?.click()}>{mediaItems.length ? 'Bỏ bộ lọc' : 'Nhập media'}</Button><p className="text-[10px]">Kéo ảnh, video hoặc âm thanh vào đây.</p></div>}
          <p className="mt-3 text-[10px] text-muted-foreground tabular-nums">{mediaItems.length} media · Lưu trên máy này</p>
        </>}
        {tab === 'text' && <div className="space-y-3"><Button variant="outline" size="sm" className="w-full" onClick={() => { try { insertText(); } catch (error) { showError(error); } }}><Plus aria-hidden />Thêm chữ</Button><h3 className="font-semibold">Preset phụ đề</h3>{CAPTION_ANIMATION_PRESETS.map(p => <button key={p.id} type="button" className={`${smallButton} w-full text-left`} onClick={() => { const selectedItem = useItemsStore.getState().items.find(i => i.id === useSelectionStore.getState().selectedItemIds[0]); if (!selectedItem || (selectedItem.type !== 'text' && selectedItem.type !== 'subtitle')) { toast.error('Chọn một chữ hoặc phụ đề trên timeline'); return; } const result = executeCommands([{ type: 'caption.setPreset', params: { itemId: selectedItem.id, preset: p.id } }]); if (!result.ok) { toast.error(result.issues.map(issue => issue.message).join('; ')); return; } announce(`Đã áp dụng ${p.name}`); }}>{p.name}</button>)}</div>}
        {tab === 'effects' && <EffectsTab query={query} />}
        {tab === 'transitions' && <div className="space-y-1"><p className="mb-3 text-[10px] text-muted-foreground">Chọn clip trước điểm cắt, rồi áp dụng.</p>{transitions.map(transition => <button key={transition.id} type="button" draggable onDragStart={e => startDrop(e, { type: 'transition', presentation: transition.id })} className={`${smallButton} flex w-full items-center gap-2 text-left`} onClick={() => apply({ type: 'transition', presentation: transition.id })}><Scissors aria-hidden className="size-3.5 shrink-0" /><span>{transitionNames[transition.id]}</span></button>)}{!transitions.length && <p className="py-6 text-muted-foreground">Không có chuyển cảnh phù hợp</p>}</div>}
        {tab === 'captions' && <div className="space-y-3"><Button variant="outline" size="sm" className="w-full" onClick={() => captionsPicker.current?.click()}><Upload aria-hidden />Nhập phụ đề</Button><p className="text-muted-foreground">SRT, VTT hoặc json3. Mốc từ chia đều được ghi là ước lượng.</p></div>}
        {tab === 'overlays' && <OverlaysTab query={query} />}
        {tab === 'templates' && <TemplateList />}
      </div>
    </div>
    <AlertDialog open={!!pendingDelete} onOpenChange={open => { if (!open) setPendingDelete(null); }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Xoá media đang dùng?</AlertDialogTitle><AlertDialogDescription>“{pendingDelete?.fileName}” đang được dùng trên timeline. Xoá khỏi thư viện sẽ làm các clip này thiếu file.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Giữ lại</AlertDialogCancel><AlertDialogAction className="bg-primary text-background" onClick={() => { if (pendingDelete) remove(pendingDelete); }}>Xoá media</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </aside>;
}

function TemplateList() {
  const templates = getBuiltInEditingTemplates();
  const [templateId, setTemplateId] = useState<string | null>(null);
  const [title, setTitle] = useState('');
  const [seconds, setSeconds] = useState(30);
  const [busy, setBusy] = useState(false);
  const media = useMediaLibraryStore(s => s.mediaItems);
  const project = useProjectStore(s => s.currentProject);
  const images = media.filter(m => getMediaType(m.mimeType) === 'image');
  const [audioId, setAudioId] = useState('');
  const apply = async () => {
    if (!templateId || !images.length) return;
    setBusy(true);
    try {
      const fps = useTimelineSettingsStore.getState().fps;
      const audio = media.find(m => m.id === audioId);
      const duration = Math.max(images.length, Math.round(seconds * fps));
      const sections = await Promise.all(images.map(async (image, index) => {
        const from = Math.round(index * duration / images.length);
        const end = Math.round((index + 1) * duration / images.length);
        return { title: image.fileName, from, durationInFrames: end - from, image: { mediaId: image.id, src: await resolveMediaUrl(image.id) } };
      }));
      const result = executeCommands([{ type: 'template.apply', params: { templateId, context: { idPrefix: crypto.randomUUID(), fps, width: project?.metadata.width ?? 1920, height: project?.metadata.height ?? 1080, from: usePlaybackStore.getState().currentFrame, title, durationInFrames: duration, sections, ...(audio && { narration: { mediaId: audio.id, src: await resolveMediaUrl(audio.id) } }) } } }]);
      if (!result.ok) throw new Error(result.issues.map(issue => issue.message).join('; '));
      announce('Đã áp dụng mẫu');
      setTemplateId(null);
    } catch (error) { showError(error); }
    finally { setBusy(false); }
  };
  return <div className="space-y-2">{templates.map(template => <button key={template.id} type="button" className={`${smallButton} w-full p-3 text-left`} onClick={() => { setTemplateId(template.id); setTitle(project?.name ?? ''); }}><span className="block font-medium">{template.name}</span><span className="mt-1 block text-[10px] text-muted-foreground">{template.description}</span></button>)}
    <Dialog open={!!templateId} onOpenChange={open => { if (!open && !busy) setTemplateId(null); }}><DialogContent><DialogHeader><DialogTitle>{templates.find(t => t.id === templateId)?.name}</DialogTitle><DialogDescription>Dùng {images.length} ảnh trong thư viện, chèn tại playhead. Các ảnh được chia đều theo thời lượng.</DialogDescription></DialogHeader>
      <label className="space-y-1 text-xs">Tiêu đề<Input value={title} onChange={e => setTitle(e.target.value)} /></label>
      <label className="space-y-1 text-xs">Thời lượng (giây)<Input type="number" min="1" max="3600" value={seconds} onChange={e => setSeconds(Math.max(1, Number(e.target.value)))} className="tabular-nums" /></label>
      <div className="grid gap-1 text-xs"><label htmlFor="template-narration">Lời dẫn</label><EditorSelect id="template-narration" size="field" value={audioId || 'none'} onValueChange={value => setAudioId(value === 'none' ? '' : value)} options={[{ value: 'none', label: 'Không thêm' }, ...media.filter(m => getMediaType(m.mimeType) === 'audio').map(m => ({ value: m.id, label: m.fileName }))]} /></div>
      {!images.length && <p className="text-sm text-muted-foreground">Nhập ít nhất một ảnh để áp dụng mẫu.</p>}
      <DialogFooter><Button variant="outline" disabled={busy} onClick={() => setTemplateId(null)}>Huỷ</Button><Button disabled={busy || !images.length} className="text-background" onClick={() => { void apply(); }}>{busy ? 'Đang áp dụng…' : 'Áp dụng mẫu'}</Button></DialogFooter>
    </DialogContent></Dialog>
  </div>;
}
