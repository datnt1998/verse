"use client";

import type { LucideIcon } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";
import { cn } from "cn";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

/**
 * Icon-only button; the label is the tooltip and the accessible name.
 * With `href` it renders an external link that opens in a new tab.
 * `pending` spins the icon in the same render as the click and swallows further clicks
 * (aria-disabled, not `disabled`, so focus stays on the control).
 */
export function IconButton({
  icon: Icon,
  label,
  pending = false,
  href,
  variant = "ghost",
  size = "icon",
  className,
  onClick,
  "aria-disabled": ariaDisabled,
  ...props
}: Omit<ComponentProps<typeof Button>, "asChild" | "children"> & {
  icon: LucideIcon;
  label: string;
  pending?: boolean;
  href?: string;
}) {
  const icon = <Icon aria-hidden className={cn(pending && "animate-spin")} />;
  const locked = pending || ariaDisabled === true || ariaDisabled === "true";
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant={variant}
          size={size}
          className={cn("size-11", pending ? "cursor-progress" : locked && "opacity-50", className)}
          aria-label={label}
          aria-busy={pending || undefined}
          aria-disabled={locked || ariaDisabled}
          asChild={href !== undefined}
          onClick={(event) => {
            if (locked) event.preventDefault();
            else onClick?.(event);
          }}
          {...props}
        >
          {href !== undefined ? (
            <a href={href} target="_blank" rel="noreferrer">
              {icon}
            </a>
          ) : (
            icon
          )}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

/**
 * Icon + short value; its explanation is available on hover, focus and touch.
 * `icon` is an element (not a component) so Server Components can pass it.
 * Stays focusable so sighted keyboard users can open the tooltip too (aria-label alone would
 * cover screen readers, but they are not the only ones relying on the keyboard).
 */
export function Stat({
  icon,
  label,
  children,
  className,
}: {
  icon: ReactNode;
  label: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          role="group"
          tabIndex={0}
          className={cn(
            "inline-flex min-w-0 items-center gap-1.5 rounded-sm text-xs tabular-nums text-muted-foreground [&_svg]:size-3.5 [&_svg]:shrink-0",
            className,
          )}
        >
          {icon}
          <span className="min-w-0 break-all">{children}</span>
        </span>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
