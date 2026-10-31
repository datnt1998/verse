import { CLONE_QUALITY_ITEMS, cloneScoreAllowsExport } from "@/lib/clone-quality";
import type { CloneScore } from "@/lib/clone-quality";

/**
 * Combine the persisted gate with unsaved editor state; a stored pass is not authoritative.
 * Once the owner accepted the checklist ("Vẫn xuất"), only unsaved state still blocks export.
 */
export function cloneEditorExportGate(input: {
  score: CloneScore | null;
  gate: { allowed: boolean; reason: string | null; accepted?: boolean };
  dirty: boolean;
  conflict: unknown;
  saving: boolean;
}): { canExport: boolean; gateReason: string | null; checklistBlocked: boolean } {
  const saved = !input.dirty && !input.conflict && !input.saving;
  const unsavedReason = input.dirty || input.conflict ? "Project chưa lưu hoặc có xung đột — lưu rồi đo lại" : null;
  if (input.gate.accepted) return { canExport: saved, gateReason: unsavedReason, checklistBlocked: false };
  const checklistPasses = input.gate.allowed &&
    input.score !== null && input.score.measurementId !== null &&
    CLONE_QUALITY_ITEMS.every((id) => input.score!.items.some((item) => item.id === id)) &&
    cloneScoreAllowsExport(input.score);
  return {
    canExport: saved && checklistPasses,
    gateReason: unsavedReason ?? input.gate.reason,
    // Only the checklist is the owner's call; unsaved edits must be saved first.
    checklistBlocked: saved && !checklistPasses,
  };
}

/** Clone renders must use the complete, fixed 1080p30 timeline. */
export function cloneExportSettingsAllowed(input: {
  resolution: { width: number; height: number };
  fps?: number;
  renderWholeProject?: boolean;
}): boolean {
  return input.resolution.width === 1920 && input.resolution.height === 1080 &&
    input.fps === 30 && input.renderWholeProject === true;
}
