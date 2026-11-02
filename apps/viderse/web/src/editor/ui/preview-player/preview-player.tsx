"use client";

import { useEffect, useRef, type KeyboardEvent, type ReactNode } from "react";
import { Loader2, Pause, Play, TriangleAlert, Volume2, VolumeX } from "lucide-react";
import { HeadlessPlayer as Player, type PlayerRef } from "@editor/runtime/player/Player";
import { MainComposition } from "@editor/runtime/composition-runtime/compositions/main-composition";
import { useItemsStore } from "@editor/features/timeline/stores/items-store";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { IconButton } from "@/components/icon-tip";
import { Button } from "@/components/ui/button";
import { Slider } from "@/components/ui/slider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { CanvasPreview } from "@/editor/ui/preview/canvas-preview";
import { usePlayerStoreSync, usePreviewComposition } from "@/editor/ui/preview/use-preview-composition";
import { clockLabel, spokenClock } from "./clock";
import { usePreviewSession } from "./use-preview-session";

/** Half resolution keeps a 1080p composition smooth in a pane about 800 px wide. */
const QUALITY = 0.5;

function Frame({ children }: { children: ReactNode }) {
  return <div className="flex aspect-video w-full flex-col items-center justify-center gap-3 overflow-hidden rounded-lg border bg-muted p-4 text-center text-sm text-muted-foreground">{children}</div>;
}

/**
 * Read-only player for one project: the editor's own preview canvas and player with a small transport
 * (play, scrub, time, mute). Nothing here edits, saves or registers shortcuts; Space plays when the
 * player has focus. A different `projectId` resets everything and loads that project.
 *
 * The transport's icon buttons carry Radix tooltips, so the standalone route supplies the
 * `TooltipProvider` the editor root normally owns.
 */
export function PreviewPlayer({ projectId }: { projectId: string }) {
  return (
    <TooltipProvider>
      <PreviewPlayerBody projectId={projectId} />
    </TooltipProvider>
  );
}

function PreviewPlayerBody({ projectId }: { projectId: string }) {
  const { state, retry } = usePreviewSession(projectId);
  const loaded = useProjectStore((store) => store.currentProject?.id === projectId);
  if (state === "not-found") {
    return <Frame><TriangleAlert aria-hidden className="size-6" /><p role="alert">Bản xem thử này không còn nữa.</p></Frame>;
  }
  if (state === "error") {
    return (
      <Frame>
        <TriangleAlert aria-hidden className="size-6" />
        <p role="alert">Chưa mở được bản xem thử.</p>
        <Button variant="outline" className="min-h-11" onClick={retry}>Thử lại</Button>
      </Frame>
    );
  }
  if (state === "loading" || !loaded) {
    return <Frame><Loader2 aria-hidden className="size-6 animate-spin" /><p role="status">Đang mở bản xem thử…</p></Frame>;
  }
  return <PlayerStage key={projectId} />;
}

function PlayerStage() {
  const { fps, width, height, duration, resolved, transitions, keyframes, backgroundColor, composition } = usePreviewComposition();
  const player = useRef<PlayerRef>(null);
  usePlayerStoreSync(player);
  const playing = usePlaybackStore((store) => store.isPlaying);
  const muted = usePlaybackStore((store) => store.muted);
  const frame = usePlaybackStore((store) => store.currentFrame);
  const last = Math.max(1, duration - 1);

  // Leaving the page or switching project releases playback; the Player unmount releases the decoders.
  useEffect(() => () => usePlaybackStore.getState().pause(), []);

  const seek = (next: number) => {
    const state = usePlaybackStore.getState();
    state.pause();
    state.setCurrentFrame(Math.max(0, Math.min(last, next)));
  };
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.code !== "Space" || event.target !== event.currentTarget) return;
    event.preventDefault();
    usePlaybackStore.getState().togglePlayPause();
  };

  return (
    <section aria-label="Xem thử" tabIndex={0} onKeyDown={onKeyDown} className="flex min-w-0 flex-col gap-2 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring">
      <div className="relative w-full overflow-hidden rounded-lg border bg-black" style={{ aspectRatio: `${width} / ${height}` }}>
        <Player
          ref={player}
          fps={fps}
          durationInFrames={duration}
          width={Math.round(width * QUALITY)}
          height={Math.round(height * QUALITY)}
          style={{ width: "100%", height: "100%" }}
          onFrameChange={(next) => usePlaybackStore.getState().setCurrentFrame(next)}
          onPlayStateChange={(isPlaying) => {
            const state = usePlaybackStore.getState();
            if (isPlaying !== state.isPlaying) { if (isPlaying) state.play(); else state.pause(); }
          }}
        >
          <MainComposition fps={fps} tracks={resolved} transitions={transitions} keyframes={keyframes} width={width} height={height} backgroundColor={backgroundColor} liveItemTransformSource={useItemsStore} />
        </Player>
        <CanvasPreview composition={composition} quality={QUALITY} />
      </div>
      <div className="flex min-w-0 items-center gap-2">
        <IconButton icon={playing ? Pause : Play} label={playing ? "Tạm dừng" : "Phát"} onClick={() => usePlaybackStore.getState().togglePlayPause()} />
        <Slider aria-label="Tua video" min={0} max={last} step={1} value={[Math.min(frame, last)]} onValueChange={([next]) => seek(next)} className="min-h-11 min-w-0 flex-1" />
        <span role="timer" aria-label={`Thời gian ${spokenClock(frame, fps)}`} className="shrink-0 font-mono text-xs tabular-nums">
          {clockLabel(frame, fps)} <span className="text-muted-foreground">/ {clockLabel(duration, fps)}</span>
        </span>
        <IconButton icon={muted ? VolumeX : Volume2} label={muted ? "Bật tiếng" : "Tắt tiếng"} aria-pressed={muted} onClick={() => usePlaybackStore.getState().toggleMute()} />
      </div>
    </section>
  );
}
