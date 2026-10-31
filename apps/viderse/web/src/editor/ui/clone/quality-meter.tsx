"use client";

/**
 * Clone "Độ hoàn thiện" meter + export-gate popover for the editor header.
 * Renders only for clone projects: total score badge, per-item pass/fail
 * checklist with evidence and "Sửa" deep links, measurement trigger and the
 * freshness gate reason.
 */
import { useState } from "react";
import { BadgeCheck, CircleAlert, CircleHelp, Download, RefreshCw, ShieldCheck, Wrench } from "lucide-react";

import { CLONE_EXPORT_MIN_SCORE, CLONE_QUALITY_ITEM_LABELS, type CloneQualityItem } from "@/lib/clone-quality";
import type { CloneQuality } from "./use-clone-quality";

const STATUS_ICON = {
  pass: <BadgeCheck className="h-4 w-4 text-emerald-500" aria-label="đạt" />,
  fail: <CircleAlert className="h-4 w-4 text-rose-500" aria-label="chưa đạt" />,
  unknown: <CircleHelp className="h-4 w-4 text-zinc-400" aria-label="chưa đo" />,
} as const;


export function CloneQualityMeter({ quality, roomy = false, measureBlockedReason }: { quality: CloneQuality; roomy?: boolean; measureBlockedReason?: string | null }) {
  const [open, setOpen] = useState(false);
  const [accepting, setAccepting] = useState(false);
  const [acceptError, setAcceptError] = useState<string | null>(null);
  if (!quality.clone) return null;

  const score = quality.score;
  const total = score?.total ?? null;
  const blocked = quality.gateBlockers;
  const busy = quality.measuring;
  return (
    <div className="relative" data-testid="clone-quality-meter">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        // Accessible name starts with the visible text (WCAG 2.5.3 label-in-name).
        aria-label={`${total === null ? "Chưa đo" : `${total}/100`} Độ hoàn thiện`}
        className={roomy ? "flex min-h-11 items-center gap-1.5 rounded-md border border-border px-3 text-xs hover:bg-accent" : "flex items-center gap-1.5 rounded-md border border-border px-2 py-1 text-xs hover:bg-accent"}
      >
        <span
          className={
            total === null
              ? "text-muted-foreground"
              : quality.canExport
                ? "text-emerald-600 dark:text-emerald-400 font-semibold"
                : "text-rose-600 dark:text-rose-400 font-semibold"
          }
        >
          {total === null ? "Chưa đo" : `${total}/100`}
        </span>{" "}
        <span className={roomy ? "text-muted-foreground" : "hidden sm:inline text-muted-foreground"}>Độ hoàn thiện</span>
      </button>
      {open && (
        <div className={`${roomy ? "left-0 w-[min(24rem,calc(100vw-6rem))]" : "right-0 w-96"} absolute top-full z-50 mt-1 max-h-[70dvh] overflow-y-auto rounded-md border border-border bg-popover p-3 shadow-lg`} role="dialog" aria-label="Chi tiết Độ hoàn thiện" onKeyDown={(event) => { if (event.key === "Escape") setOpen(false); }}>
          <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
            <div className="text-sm font-semibold">Độ hoàn thiện clone</div>
            <button
              type="button"
              onClick={() => { if (!measureBlockedReason && !busy) void quality.measure(); }}
              disabled={roomy ? undefined : busy}
              aria-disabled={roomy && (busy || Boolean(measureBlockedReason)) ? true : undefined}
              title={measureBlockedReason ?? undefined}
              className={roomy ? "flex min-h-11 max-w-full items-center gap-1 rounded bg-primary px-3 text-xs text-primary-foreground aria-disabled:opacity-50" : "flex items-center gap-1 rounded bg-primary px-2 py-1 text-xs text-primary-foreground disabled:opacity-50"}
            >
              <RefreshCw className={`h-3 w-3 ${busy ? "animate-spin" : ""}`} />
              {busy ? "Đang đo…" : score ? "Đo lại" : "Đo độ hoàn thiện"}
            </button>
          </div>
          {measureBlockedReason ? <p className="mb-2 text-xs text-amber-300">{measureBlockedReason}</p> : null}
          {quality.progress && (
            <div className="mb-2 text-xs text-muted-foreground" role="status">
              {quality.progress.phase === "claim" && "Đang nhận yêu cầu đo…"}
              {quality.progress.phase === "decode" && "Đang trộn audio engine…"}
              {quality.progress.phase === "upload" && `Đang gửi PCM (${(quality.progress.bytesSent / 1024 ** 2).toFixed(1)} MB)…`}
              {quality.progress.phase === "finish" && "Đang đo LUFS phía server…"}
            </div>
          )}
          {quality.measureError && (
            <div className="mb-2 rounded bg-rose-500/10 px-2 py-1 text-xs text-rose-600 dark:text-rose-400">{quality.measureError}</div>
          )}
          <ul className="space-y-1.5">
            {(score?.items ?? []).map((item) => (
              <QualityItemRow key={item.id} item={item} roomy={roomy} onFix={(target) => quality.openFix(target)} />
            ))}
            {!score && <li className="text-xs text-muted-foreground">Chưa có điểm — bấm “Đo độ hoàn thiện”.</li>}
          </ul>
          {blocked.length > 0 && (
            <div className="mt-2 border-t border-border pt-2">
              <div className="mb-1 text-xs font-semibold text-rose-600 dark:text-rose-400">Điều kiện bắt buộc chưa đạt</div>
              <ul className="space-y-1 text-xs text-muted-foreground">
                {blocked.map((blocker) => (
                  <li key={blocker.id}>
                    • {blocker.message}
                    {roomy && score?.blockers.find((entry) => entry.id === blocker.id)?.fix ? (
                      <button type="button" className="ml-2 inline-flex min-h-11 items-center gap-1 rounded border px-3" onClick={() => {
                        const fix = score?.blockers.find((entry) => entry.id === blocker.id)?.fix;
                        if (fix) quality.openFix(fix);
                      }}><Wrench aria-hidden className="size-3" /> Sửa</button>
                    ) : null}
                  </li>
                ))}
              </ul>
            </div>
          )}
          <div className="mt-2 flex flex-col gap-2 border-t border-border pt-2 text-xs text-muted-foreground">
            <p>
              {quality.accepted
                ? (quality.gateReason ?? "Bạn đã chọn Vẫn xuất — video được xuất dù chưa đạt hết các mục trên.")
                : quality.canExport
                  ? `Đạt ${CLONE_EXPORT_MIN_SCORE}+ và đủ điều kiện — được xuất.`
                  : (quality.gateReason ?? `Cần ≥${CLONE_EXPORT_MIN_SCORE} và đủ điều kiện bắt buộc để xuất.`)}
            </p>
            {quality.accepted || quality.checklistBlocked ? (
              <button
                type="button"
                aria-busy={accepting || undefined}
                onClick={() => {
                  if (accepting) return;
                  setAccepting(true);
                  setAcceptError(null);
                  quality.acceptQuality(!quality.accepted)
                    .catch((error: unknown) => setAcceptError(error instanceof Error ? error.message : String(error)))
                    .finally(() => setAccepting(false));
                }}
                className={`${roomy ? "min-h-11 px-3" : "px-2 py-1"} flex w-fit items-center gap-1 rounded border border-border text-foreground hover:bg-accent aria-busy:cursor-progress`}
              >
                {quality.accepted ? <ShieldCheck aria-hidden className="h-3 w-3" /> : <Download aria-hidden className="h-3 w-3" />}
                {accepting ? "Đang lưu…" : quality.accepted ? "Bật lại kiểm tra Độ hoàn thiện" : "Vẫn xuất như hiện tại"}
              </button>
            ) : null}
            {acceptError ? <p className="text-rose-600 dark:text-rose-400">{acceptError}</p> : null}
          </div>
        </div>
      )}
    </div>
  );
}

function QualityItemRow({
  item,
  onFix,
  roomy,
}: {
  item: CloneQualityItem;
  onFix: (target: NonNullable<CloneQualityItem["fix"]>) => void;
  roomy: boolean;
}) {
  const meta = CLONE_QUALITY_ITEM_LABELS[item.id];
  return (
    <li className="flex items-start gap-2" data-testid={`quality-item-${item.id}`}>
      <span className="mt-0.5">{STATUS_ICON[item.status]}</span>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <span className="text-xs font-medium">{meta?.label ?? item.id}</span>
          <span className="text-[10px] text-muted-foreground">{item.points}/{item.weight}</span>
        </div>
        {item.evidence && <div className="truncate text-[11px] text-muted-foreground" title={item.evidence}>{item.evidence}</div>}
      </div>
      {item.status !== "pass" && item.fix && (
        <button
          type="button"
          onClick={() => onFix(item.fix!)}
          className={roomy
            ? "flex min-h-11 items-center gap-1.5 rounded-md border border-border px-3 text-xs hover:bg-accent"
            : "flex items-center gap-0.5 rounded border border-border px-1.5 py-0.5 text-[10px] hover:bg-accent"}
          aria-label={`Sửa ${meta?.label ?? item.id}`}
        >
          <Wrench className="h-3 w-3" />
          Sửa
        </button>
      )}
    </li>
  );
}
