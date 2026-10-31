"use client";

import { useEffect, useRef } from "react";


type SourceComparisonProps = {
  sourceUrl: string;
  /** Current project/source timestamp, in seconds. */
  time: number;
  /** Bumps a re-seek even when `time` is unchanged (jump back to a reference). */
  syncKey?: number | string;
  playing: boolean;
  label?: string;
  onSeek?: (seconds: number) => void;
  onTimeChange?: (seconds: number) => void;
  onPlayingChange?: (playing: boolean) => void;
};

/** Source audio stays muted so comparison never doubles the project's sound. */
export function SourceComparison({ sourceUrl, time, syncKey, playing, label = "Video gốc · tiếng đã tắt khi so sánh", onSeek, onTimeChange, onPlayingChange }: SourceComparisonProps) {
  const video = useRef<HTMLVideoElement>(null);
  const externallySeeking = useRef(false);
  const lastSync = useRef<number | string | undefined>(undefined);
  useEffect(() => {
    const element = video.current;
    if (!element || !Number.isFinite(time)) return;
    const forced = syncKey !== undefined && syncKey !== lastSync.current;
    lastSync.current = syncKey;
    if (Math.abs(element.currentTime - time) > (forced ? 0.001 : 0.05)) {
      externallySeeking.current = true;
      element.currentTime = Math.max(0, time);
    }
  }, [time, syncKey]);
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    if (playing) void element.play().catch(() => onPlayingChange?.(false));
    else element.pause();
  }, [playing, onPlayingChange]);
  return (
    <figure className="min-w-0 overflow-hidden rounded-lg border bg-background">
      <video ref={video} src={sourceUrl} muted controls preload="metadata" aria-label="Video gốc, đồng bộ thời gian với bản dựng" className="aspect-video w-full object-contain"
        onVolumeChange={(event) => { if (!event.currentTarget.muted) event.currentTarget.muted = true; }}
        onTimeUpdate={(event) => onTimeChange?.(event.currentTarget.currentTime)}
        onPlay={() => onPlayingChange?.(true)} onPause={() => onPlayingChange?.(false)}
        onLoadedMetadata={() => { const element = video.current; if (element && Number.isFinite(time)) element.currentTime = Math.max(0, Math.min(time, element.duration)); }}
        onSeeked={() => { if (externallySeeking.current) externallySeeking.current = false; else if (video.current) onSeek?.(video.current.currentTime); }} />
      <figcaption className="border-t px-3 py-2 text-xs text-muted-foreground">{label}</figcaption>
    </figure>
  );
}
