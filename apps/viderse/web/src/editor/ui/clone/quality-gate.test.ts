import assert from "node:assert/strict";
import { test } from "vitest";
import { CLONE_QUALITY_ITEMS, CLONE_QUALITY_WEIGHTS } from "@/lib/clone-quality";
import type { CloneScore } from "@/lib/clone-quality";
import { cloneEditorExportGate, cloneExportSettingsAllowed } from "./quality-gate";

function measuredScore(total = 90): CloneScore {
  return {
    version: 1, total, passed: true, blockers: [],
    binding: { projectRevision: "sha256:project", settingsHash: "sha256:settings", artifactHash: "sha256:artifacts" },
    items: CLONE_QUALITY_ITEMS.map((id) => ({ id, status: "pass", points: CLONE_QUALITY_WEIGHTS[id], weight: CLONE_QUALITY_WEIGHTS[id] })),
    measurementId: "measured-full-mix", measuredAt: "2026-10-12T00:00:00Z", generation: 1,
  };
}
function decision(score: CloneScore | null, state: { dirty?: boolean; conflict?: unknown; saving?: boolean } = {}) {
  return cloneEditorExportGate({ score, gate: { allowed: true, reason: null }, dirty: state.dirty ?? false, conflict: state.conflict, saving: state.saving ?? false });
}

test("quality meter unlocks at 90, not 89, irrespective of stored passed", () => {
  assert.equal(decision(measuredScore(89)).canExport, false);
  assert.equal(decision({ ...measuredScore(90), passed: false }).canExport, true);
});

test("revision, settings and artifact changes close the consumer gate with recovery", () => {
  const score = measuredScore();
  for (const key of ["projectRevision", "settingsHash", "artifactHash"] as const) {
    // Go owns binding validation; this consumer receives its denied wire result.
    const gate = cloneEditorExportGate({ score, gate: { allowed: false, reason: `Độ hoàn thiện đã cũ: ${key}` }, dirty: false, conflict: null, saving: false });
    assert.equal(gate.canExport, false, key);
    assert.ok(gate.gateReason, `${key} must explain why export is locked`);
  }
});

test("dirty edits, conflicts and in-flight saves cannot export a previous measured pass", () => {
  for (const state of [{ dirty: true }, { conflict: { revision: "changed" } }, { saving: true }]) {
    assert.equal(decision(measuredScore(100), state).canExport, false);
  }
  assert.match(decision(measuredScore(), { dirty: true }).gateReason!, /lưu/);
});

test("after Vẫn xuất only unsaved state blocks export; the checklist alone is what the owner may override", () => {
  const accepted = (score: CloneScore | null, state: { dirty?: boolean } = {}) =>
    cloneEditorExportGate({ score, gate: { allowed: false, reason: "Điểm đã cũ", accepted: true }, dirty: state.dirty ?? false, conflict: null, saving: false });
  assert.equal(accepted(measuredScore(40)).canExport, true);
  assert.equal(accepted(null).canExport, true);
  assert.equal(accepted(measuredScore(40)).checklistBlocked, false);
  const unsaved = accepted(measuredScore(40), { dirty: true });
  assert.equal(unsaved.canExport, false);
  assert.match(unsaved.gateReason!, /lưu/);

  assert.equal(decision(measuredScore(40)).checklistBlocked, true, "a failing checklist offers Vẫn xuất");
  assert.equal(decision(measuredScore(40), { dirty: true }).checklistBlocked, false, "unsaved edits must be saved first");
  assert.equal(decision(measuredScore(100)).checklistBlocked, false);
});

test("missing score or measurement evidence cannot unlock the meter", () => {
  assert.equal(decision(null).canExport, false);
  const noScore = cloneEditorExportGate({ score: null, gate: { allowed: false, reason: "Cần đo độ hoàn thiện" }, dirty: false, conflict: null, saving: false });
  assert.equal(noScore.canExport, false);
  assert.match(noScore.gateReason!, /đo/);
  const missingLoudness = measuredScore(100);
  missingLoudness.items = missingLoudness.items.map((item) => item.id === "loudness" ? { ...item, status: "unknown", points: 0 } : item);
  assert.equal(decision(missingLoudness).canExport, false);
  assert.equal(decision({ ...measuredScore(100), measurementId: null }).canExport, false);
  assert.equal(decision({ ...measuredScore(100), items: [] }).canExport, false);
  assert.equal(decision({ ...measuredScore(100), items: measuredScore().items.filter((item) => item.id !== "loudness") }).canExport, false);
  const blocked = { ...measuredScore(100), blockers: [{ id: "measurement", message: "Chưa đo mix đã lưu" }] };
  assert.equal(decision(blocked).canExport, false);
});

test("clone settings reject low resolution, partial ranges and changed fps", () => {
  const full = { resolution: { width: 1920, height: 1080 }, fps: 30, renderWholeProject: true };
  assert.equal(cloneExportSettingsAllowed(full), true);
  assert.equal(cloneExportSettingsAllowed({ ...full, resolution: { width: 1280, height: 720 } }), false);
  assert.equal(cloneExportSettingsAllowed({ ...full, renderWholeProject: false }), false);
  assert.equal(cloneExportSettingsAllowed({ ...full, fps: 24 }), false);
  assert.equal(cloneExportSettingsAllowed({ resolution: full.resolution }), false);
});
