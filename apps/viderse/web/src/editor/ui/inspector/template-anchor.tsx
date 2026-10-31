"use client";

/**
 * "Trong template" — where a clip sits when a clone fills this template
 * project: cover the whole clone (`full`), keep its offset from the start, or
 * keep its distance from the end. `item.update` carries the validation, so a
 * `full` anchor on a video that cannot loop is rejected with the engine's own
 * message.
 */
import { toast } from "sonner";
import type { TimelineItem } from "@editor/types/timeline";
import { executeCommands } from "@editor/commands";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { TEMPLATE_ANCHOR_LABELS, TEMPLATE_SLOT_LABELS } from "@editor/features/editing-templates/channel-template";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { EditorSelect, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";
import { useTemplateStatus } from "@/editor/ui/template/template-status";

const VIETNAMESE_TEXT = /[À-ỹ]/u;

const ANCHOR_OPTIONS = (["full", "start", "end"] as const).map((anchor) => ({
  value: anchor,
  label: TEMPLATE_ANCHOR_LABELS[anchor],
}));

const ANCHOR_EXPLANATION: Record<"full" | "start" | "end", string> = {
  full: "Clip phủ hết video clone; clip video sẽ lặp cho đủ độ dài.",
  start: "Clip giữ nguyên vị trí tính từ đầu; phần dư bị cắt nếu video clone ngắn hơn.",
  end: "Clip giữ nguyên khoảng cách tới cuối; clip bị bỏ nếu video clone ngắn hơn vị trí đó.",
};

export function TemplateAnchorSection({ item }: { item: TimelineItem }) {
  const projectId = useProjectStore((state) => state.currentProject?.id ?? "");
  const { status, ready } = useTemplateStatus(projectId);
  const slot = useTimelineStore((state) => state.tracks.find((track) => track.id === item.trackId)?.templateSlot);
  if (!ready || !status?.template) return null;
  const anchor = item.templateAnchor ?? "start";
  const onChange = (value: string) => {
    const result = executeCommands([{ type: "item.update", params: { id: item.id, updates: { templateAnchor: value } } }]);
    if (!result.ok) {
      const message = result.issues.find((issue) => VIETNAMESE_TEXT.test(issue.message))?.message ?? result.issues[0]?.message ?? "Không áp dụng được mốc neo.";
      toast.error(message);
      announce(message, "assertive");
      return;
    }
    announce(`Mốc neo template: ${TEMPLATE_ANCHOR_LABELS[value as keyof typeof TEMPLATE_ANCHOR_LABELS]}`);
  };
  return (
    <SectionHeader title="Trong template">
      {slot ? (
        <p className="text-[11px] text-muted-foreground">
          Clip nằm trên track vai trò {TEMPLATE_SLOT_LABELS[slot]} — clone thay thế nội dung track này, mốc neo không áp dụng.
        </p>
      ) : (
        <>
          <PropertyRow label="Mốc neo">
            <EditorSelect label="Mốc neo trong template" value={anchor} onValueChange={onChange} options={ANCHOR_OPTIONS} />
          </PropertyRow>
          <p className="text-[11px] text-muted-foreground">{ANCHOR_EXPLANATION[anchor]}</p>
        </>
      )}
    </SectionHeader>
  );
}
