"use client";
import { memo, useEffect, useRef, useState } from 'react';
import type { TimelineItem } from '@editor/types/timeline';
import { filmstripCache, type Filmstrip } from '@editor/features/timeline/services/filmstrip-cache';
import { waveformCache, type CachedWaveform } from '@editor/features/timeline/services/waveform-cache';

export const ItemVisual = memo(function ItemVisual({ item, fps, width, offset, pixelsPerFrame }: { item: TimelineItem; fps: number; width: number; offset: number; pixelsPerFrame: number }) {
  const [filmstrip, setFilmstrip] = useState<Filmstrip | null>(null);
  const [waveform, setWaveform] = useState<CachedWaveform | null>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const filmstripCanvas = useRef<HTMLCanvasElement>(null);
  const src = 'src' in item ? item.src : undefined;
  useEffect(() => {
    if (!item.mediaId || !src) return;
    let mounted = true;
    if (item.type === 'video') {
      const unsubscribe = filmstripCache.subscribe(item.mediaId, (value) => { if (mounted) setFilmstrip(value); });
      void filmstripCache.getFilmstrip(item.mediaId, src, (item.sourceDuration ?? item.durationInFrames) / (item.sourceFps ?? fps), undefined, undefined, { targetFrameCount: 24 }).then((value) => { if (mounted) setFilmstrip(value); }).catch(() => {});
      return () => { mounted = false; unsubscribe(); };
    }
    if (item.type === 'audio') {
      const unsubscribe = waveformCache.subscribe(item.mediaId, (value) => { if (mounted) setWaveform(value); });
      void waveformCache.getWaveform(item.mediaId, src).then((value) => { if (mounted) setWaveform(value); }).catch(() => {});
      return () => { mounted = false; unsubscribe(); };
    }
  }, [item.mediaId, item.type, src, fps, item.sourceDuration, item.sourceFps, item.durationInFrames]);
  useEffect(() => {
    const element = canvas.current;
    if (!element || !waveform || width <= 0) return;
    const ratio = window.devicePixelRatio || 1;
    const height = 22;
    element.width = Math.ceil(width * ratio); element.height = height * ratio;
    const context = element.getContext('2d'); if (!context) return;
    context.scale(ratio, ratio);
    context.strokeStyle = getComputedStyle(element).getPropertyValue('--ed-waveform');
    context.lineWidth = 1;
    const channels = waveform.stereo ? 2 : 1;
    const speed = item.speed ?? 1;
    const sourceStart = (item.sourceStart ?? item.trimStart ?? 0) / (item.sourceFps ?? fps);
    context.beginPath();
    for (let x = 0; x < width; x += 2) {
      const time = sourceStart + (offset + x) / pixelsPerFrame / fps * speed;
      const index = Math.floor(time * waveform.sampleRate) * channels;
      const next = Math.min(waveform.peaks.length, index + Math.max(channels, Math.ceil(2 / pixelsPerFrame / fps * speed * waveform.sampleRate) * channels));
      let peak = 0;
      for (let n = Math.max(0, index); n < next; n++) peak = Math.max(peak, Math.abs(waveform.peaks[n]));
      const amplitude = Math.min(1, peak / Math.max(waveform.maxPeak, 0.01)) * 10;
      context.moveTo(x, 11 - amplitude); context.lineTo(x, 11 + amplitude);
    }
    context.stroke();
  }, [waveform, width, offset, fps, pixelsPerFrame, item.speed, item.sourceStart, item.trimStart, item.sourceFps]);
  useEffect(() => {
    const element = filmstripCanvas.current;
    if (!element || !filmstrip?.frames.length || width <= 0) return;
    const ratio = window.devicePixelRatio || 1;
    element.width = Math.ceil(width * ratio); element.height = 24 * ratio;
    const context = element.getContext('2d'); if (!context) return;
    context.scale(ratio, ratio);
    let mounted = true;
    const images = new Map<string, HTMLImageElement>();
    const removeLoadListeners: Array<() => void> = [];
    const tiles = Math.min(24, Math.ceil(width / 56));
    const draw = (source: CanvasImageSource, sourceWidth: number, sourceHeight: number, tile: number) => {
      if (!mounted || sourceWidth <= 0 || sourceHeight <= 0) return;
      const scale = Math.max(56 / sourceWidth, 24 / sourceHeight);
      const cropWidth = 56 / scale; const cropHeight = 24 / scale;
      context.drawImage(source, (sourceWidth - cropWidth) / 2, (sourceHeight - cropHeight) / 2, cropWidth, cropHeight, tile * 56, 0, 56, 24);
    };
    for (let tile = 0; tile < tiles; tile++) {
      const seconds = (item.sourceStart ?? item.trimStart ?? 0) / (item.sourceFps ?? fps) + (offset + tile * 56) / pixelsPerFrame / fps * (item.speed ?? 1);
      const frame = filmstrip.frames.reduce((best, next) => Math.abs(next.timestamp - seconds) < Math.abs(best.timestamp - seconds) ? next : best, filmstrip.frames[0]);
      if (frame.bitmap) draw(frame.bitmap, frame.bitmap.width, frame.bitmap.height, tile);
      else if (frame.url) {
        let image = images.get(frame.url);
        if (!image) { image = new Image(); image.src = frame.url; images.set(frame.url, image); }
        const source = image;
        if (source.complete && source.naturalWidth > 0) draw(source, source.naturalWidth, source.naturalHeight, tile);
        else {
          const onLoad = () => draw(source, source.naturalWidth, source.naturalHeight, tile);
          source.addEventListener('load', onLoad, { once: true });
          removeLoadListeners.push(() => source.removeEventListener('load', onLoad));
        }
      }
    }
    return () => {
      mounted = false;
      for (const remove of removeLoadListeners) remove();
    };
  }, [filmstrip, width, offset, fps, pixelsPerFrame, item.speed, item.sourceStart, item.trimStart, item.sourceFps]);
  if (item.type === 'audio') return <canvas ref={canvas} aria-hidden style={{ position: 'absolute', left: offset, bottom: 3, width, height: 22 }} />;
  if (item.type === 'video') return <canvas ref={filmstripCanvas} aria-hidden className="absolute bottom-0 h-6" style={{ left: offset, width }} />;
  if (item.type === 'image') {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={item.src} alt="" aria-hidden draggable={false} className="absolute bottom-0 h-6 w-12 object-cover" style={{ left: offset }} />;
  }
  return null;
});
