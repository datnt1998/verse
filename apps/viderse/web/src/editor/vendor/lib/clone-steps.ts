/**
 * The clone step chain: step order, which steps depend on which, the next open
 * step, when the chain is done and which board stage a step belongs to. Every
 * other module asks this one instead of keeping its own copy of the list.
 *
 * Client-safe: imports only the step names from the schema, which
 * `clone-contract.ts` already bundles into client pages.
 */
import { CLONE_STEP_NAMES, type CloneStepName, type CloneStepStatus } from "./clone-contract";

/** Pipeline steps in execution order. */
export const CLONE_STEPS = CLONE_STEP_NAMES;

export type CloneStage = "preparing" | "images" | "rendering";

/**
 * Invalidation graph: when a step's input changes, every step listed here must
 * be marked stale so a resume re-runs it. Each list is the transitive closure.
 * `metadata` outputs (title, description, tags) are informational only, so
 * nothing depends on them.
 */
const DEPENDENTS = {
  download: ["backdrop", "transcript", "metadata", "scenes", "images", "build", "score", "export"],
  backdrop: ["build", "score", "export"],
  transcript: ["metadata", "scenes", "images", "build", "score", "export"],
  metadata: [],
  scenes: ["images", "build", "score", "export"],
  images: ["build", "score", "export"],
  build: ["score", "export"],
  score: ["export"],
  export: [],
} as const satisfies Record<CloneStepName, readonly CloneStepName[]>;

const STAGES = {
  download: "preparing",
  backdrop: "preparing",
  transcript: "preparing",
  metadata: "preparing",
  scenes: "preparing",
  images: "images",
  build: "rendering",
  score: "rendering",
  export: "rendering",
} as const satisfies Record<CloneStepName, CloneStage>;

const INDEX = new Map<string, number>(CLONE_STEPS.map((step, index) => [step, index]));

export function isCloneStep(raw: string): raw is CloneStepName {
  return INDEX.has(raw);
}

/** True when `a` runs strictly before `b`; a step is never before itself. */
export function stepBefore(a: CloneStepName, b: CloneStepName): boolean {
  return INDEX.get(a)! < INDEX.get(b)!;
}

/** Steps that run strictly before `step`, in chain order. */
export function stepsBefore(step: CloneStepName): readonly CloneStepName[] {
  return CLONE_STEPS.slice(0, INDEX.get(step)!);
}

/** Steps that must re-run when `step`'s output or inputs change. */
export function dependentsOf(step: CloneStepName): readonly CloneStepName[] {
  return DEPENDENTS[step];
}

type StepRow = { step: string; status: CloneStepStatus };

/** Rows of the current chain in chain order; rows of retired steps are dropped. */
export function inChainOrder<Row extends { step: string }>(rows: readonly Row[]): Row[] {
  return CLONE_STEPS.flatMap((step) => rows.filter((row) => row.step === step));
}

/** The first row in chain order that has not succeeded, whatever the order of `rows`. */
export function nextOpenStep<Row extends StepRow>(rows: readonly Row[]): Row | null {
  for (const step of CLONE_STEPS) {
    const row = rows.find((candidate) => candidate.step === step);
    if (row && row.status !== "succeeded") return row;
  }
  return null;
}

/** Every chain step has a row and every row succeeded; a missing row (legacy job) is not done. */
export function chainDone(rows: readonly StepRow[]): boolean {
  return CLONE_STEPS.every((step) => rows.find((row) => row.step === step)?.status === "succeeded");
}

/** The board stage a step belongs to. */
export function stageOf(step: CloneStepName): CloneStage {
  return STAGES[step];
}
