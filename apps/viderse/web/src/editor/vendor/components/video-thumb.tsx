"use client";

import { useState } from "react";
import { Flame, ImageOff } from "lucide-react";
import { cn } from "cn";
import { OUTLIER_MIN_SCORE } from "@/lib/video-sort";

/** Badges appear from ×1.5; at OUTLIER_MIN_SCORE they switch to the solid amber "outlier" style. */
const BADGE_MIN_SCORE = 1.5;

type Props = {
  /** The YouTube video id for the 320w/480w sources; null for any other image (a template thumbnail preview). */
  videoId: string | null;
  src: string | null;
  score: number | null;
  /** Rendered duration ("12:34"), bottom-right. */
  duration?: string | null;
  /** CSS width of the slot, for choosing between the 320w and 480w sources. */
  sizes: string;
  /** "high" for the likely LCP image, "eager" for other above-the-fold images. */
  priority?: "high" | "eager" | "lazy";
  compact?: boolean;
  className?: string;
};

/**
 * 16:9 YouTube thumbnail with outlier badge and a missing-image fallback.
 * hqdefault is 4:3 letterboxed; object-cover crops the bars inside the 16:9 frame.
 */
export function VideoThumb({ videoId, src, score, duration, sizes, priority = "lazy", compact = false, className }: Props) {
  const [failed, setFailed] = useState<string | null>(null);
  const showBadge = score !== null && score >= BADGE_MIN_SCORE;
  const outlier = score !== null && score >= OUTLIER_MIN_SCORE;

  return (
    <div className={cn("relative grid aspect-video place-items-center overflow-hidden bg-muted", compact ? "rounded-md" : "rounded-lg", className)}>
      <ImageOff className={cn("text-muted-foreground", compact ? "size-5" : "size-8")} aria-hidden />
      {src && failed !== src ? (
        // eslint-disable-next-line @next/next/no-img-element -- YouTube thumbnail with a missing-image fallback
        <img
          src={src}
          srcSet={videoId ? `https://i.ytimg.com/vi/${videoId}/mqdefault.jpg 320w, https://i.ytimg.com/vi/${videoId}/hqdefault.jpg 480w` : undefined}
          sizes={videoId ? sizes : undefined}
          alt=""
          // Bilibili's image host refuses foreign referers; thumbnails never need one.
          referrerPolicy="no-referrer"
          loading={priority === "lazy" ? "lazy" : "eager"}
          fetchPriority={priority === "high" ? "high" : undefined}
          decoding="async"
          // An SSR image can fail before React attaches its error listener.
          ref={(image) => { if (image?.complete && image.naturalWidth === 0) setFailed(src); }}
          onError={() => setFailed(src)}
          className="absolute inset-0 size-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
        />
      ) : null}
      {showBadge && (
        <span
          className={cn(
            "absolute inline-flex items-center rounded-md font-semibold tabular-nums",
            // Compact: bottom-left, no icon, so ~60px thumbnails on 320px screens stay readable.
            compact ? "bottom-1 left-1 rounded px-1 text-[10px] leading-4" : "top-2 left-2 gap-1 px-2 py-1 text-xs",
            outlier ? "bg-orange-400 text-zinc-950" : "bg-zinc-950 text-orange-200",
          )}
          role="img"
          aria-label={`Được xem gấp ${score.toFixed(1)} lần mức thường thấy của kênh${outlier ? ", video nổi bật" : ""}`}
          title={`Được xem gấp ${score.toFixed(1)} lần mức thường thấy của kênh`}
        >
          {!compact && <Flame className="size-3" aria-hidden />}×{score.toFixed(1)}
        </span>
      )}
      {duration && (
        <span className="absolute right-2 bottom-2 rounded bg-zinc-950 px-1.5 py-0.5 text-xs tabular-nums text-white">
          {duration}
        </span>
      )}
    </div>
  );
}
