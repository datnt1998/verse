"use client";

import { toast } from "@/editor/ui/toast";
import { executeCommands } from "@editor/commands";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { NumberField, ColorField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";

export function CanvasSection() {
  const metadata = useProjectStore(s => s.currentProject?.metadata);
  if (!metadata) return <p className="py-6 text-xs text-muted-foreground">Chưa mở project.</p>;
  const update = (updates: Partial<typeof metadata>) => {
    const result = executeCommands([{ type: "project.updateSettings", params: updates }]);
    if (!result.ok) toast.error("Không đổi được khung hình", { description: result.issues.map(i => i.message).join("; ") });
    else announce("Đã đổi khung hình");
  };
  return <SectionHeader title="Khung hình"><PropertyRow label="Rộng"><NumberField label="Chiều rộng khung hình" value={metadata.width} min={16} max={16384} unit="px" onChange={width => update({ width: Math.round(width) })} /></PropertyRow><PropertyRow label="Cao"><NumberField label="Chiều cao khung hình" value={metadata.height} min={16} max={16384} unit="px" onChange={height => update({ height: Math.round(height) })} /></PropertyRow><PropertyRow label="Tốc độ khung"><NumberField label="Tốc độ khung hình (fps)" value={metadata.fps} min={1} max={120} step={0.001} unit="fps" onChange={fps => update({ fps })} /></PropertyRow><PropertyRow label="Màu nền"><ColorField label="Màu nền khung hình" value={metadata.backgroundColor ?? "#000000"} onChange={backgroundColor => update({ backgroundColor })} /></PropertyRow><p className="text-xs text-muted-foreground">Đổi tốc độ khung giữ nguyên mốc và thời lượng theo khung.</p></SectionHeader>;
}
