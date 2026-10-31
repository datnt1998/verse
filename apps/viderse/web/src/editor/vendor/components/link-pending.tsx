"use client";

import { useLinkStatus } from "@/editor/bridge/navigation";
import { Loader2 } from "lucide-react";
import { cn } from "cn";

export type LinkPendingVariant = "ring" | "overlay" | "inline";

/**
 * Feedback for a hash-router Link that is still navigating. Render it inside
 * the `<Link>`; the parent must be `relative` for `ring` and `overlay`.
 *
 * The reveal is delayed in CSS: the indicator stays at `opacity-0` while a pending navigation fades
 * it in with a 400 ms `transition-delay`, so a navigation that finishes inside 400 ms never paints
 * anything and no per-link timer exists. When pending clears the delay is removed so the element
 * hides instantly.
 */
export function LinkPending({ variant = "inline", className }: { variant?: LinkPendingVariant; className?: string }) {
  const { pending } = useLinkStatus();
  const reveal = cn("transition-opacity duration-150", pending ? "opacity-100 delay-[400ms]" : "opacity-0");
  return (
    <>
      {variant === "ring" ? (
        <span
          aria-hidden
          data-link-pending={pending ? "true" : undefined}
          className={cn("pointer-events-none absolute inset-0", reveal, className)}
        >
          <span className={cn("absolute inset-0 rounded-[inherit] ring-2 ring-inset ring-primary/60", pending && "motion-safe:animate-pulse")} />
        </span>
      ) : (
        <span
          aria-hidden
          data-link-pending={pending ? "true" : undefined}
          className={cn(
            "pointer-events-none grid place-items-center text-muted-foreground",
            variant === "overlay" ? "absolute inset-0 rounded-[inherit] bg-background/30 text-foreground" : "size-4 shrink-0",
            reveal,
            className,
          )}
        >
          <Loader2 className={cn("motion-safe:animate-spin", variant === "overlay" ? "size-6" : "size-4")} />
        </span>
      )}
      <span className="sr-only" role="status">{pending ? "Đang mở…" : ""}</span>
    </>
  );
}
