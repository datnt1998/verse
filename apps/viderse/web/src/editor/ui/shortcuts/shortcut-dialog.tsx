"use client";

import { memo, useState } from "react";
import { Keyboard } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { EditorIconButton } from "@/editor/ui/controls";
import { shortcutLabel, useRegisteredShortcuts, useShortcutAction, type ShortcutScope } from ".";

const scopeLabels: Record<ShortcutScope, string> = { global: "Toàn cục", timeline: "Timeline", preview: "Xem trước", inspector: "Thuộc tính", library: "Thư viện" };

// Memoized: it sits in the editor header, which re-renders on every dirty/autosave status change.
export const ShortcutDialog = memo(function ShortcutDialog({ visible = true }: { visible?: boolean }) {
  const [open, setOpen] = useState(false);
  const registered = useRegisteredShortcuts();
  useShortcutAction({ id: "shortcuts", label: "Xem phím tắt", scope: "global", enabled: visible, run: () => setOpen(true) });
  return <Dialog open={open && visible} onOpenChange={setOpen}>
    <DialogTrigger asChild><EditorIconButton icon={Keyboard} label="Xem phím tắt" shortcut="?" /></DialogTrigger>
    <DialogContent showCloseButton={false} className="max-h-[85dvh] sm:max-w-2xl">
      <DialogHeader><DialogTitle>Phím tắt</DialogTitle><DialogDescription>Phím chữ chỉ chạy trong vùng editor. Ô nhập giữ nguyên phím gõ và hoàn tác.</DialogDescription></DialogHeader>
      <div role="region" aria-label="Danh sách phím tắt" tabIndex={0} className="max-h-[60dvh] overflow-y-auto overscroll-contain rounded-sm pr-2">
        {(Object.keys(scopeLabels) as ShortcutScope[]).map((scope) => {
          const entries = registered.filter((action) => action.scope === scope && action.bindings?.length);
          if (!entries.length) return null;
          return <section key={scope} className="mb-6"><h2 className="mb-2 text-sm font-semibold">{scopeLabels[scope]}</h2><dl className="grid gap-1">{entries.map((action) => <div key={`${action.scope}-${action.id}`} className="flex items-center justify-between gap-4 rounded-md px-2 py-1.5 odd:bg-card"><dt className="text-sm">{action.label}</dt><dd className="flex shrink-0 flex-wrap justify-end gap-1">{action.bindings?.map((binding) => <kbd key={binding} className="rounded border border-ring/50 bg-secondary px-2 py-0.5 font-mono text-xs">{binding === "shift+Slash" ? "?" : shortcutLabel(binding)}</kbd>)}</dd></div>)}</dl></section>;
        })}
      </div>
      <DialogClose asChild><Button variant="outline" className="ml-auto min-h-7 [@media(pointer:coarse)]:min-h-11">Đóng</Button></DialogClose>
    </DialogContent>
  </Dialog>;
});
