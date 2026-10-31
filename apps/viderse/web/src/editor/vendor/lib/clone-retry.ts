/**
 * Whether the app retries a stopped clone step by itself. The runner acts on
 * this decision and the job page words its message from it, so both always
 * agree. Pure and client-safe.
 */

/** Attempts after which a worker-scheduled retry stops being automatic. */
export const CLONE_AUTO_RETRY_ATTEMPTS = 3;
/** Longest worker-scheduled backoff the runner sleeps through in place. */
export const CLONE_AUTO_RETRY_MAX_WAIT_MS = 30 * 60_000;

export type CloneRetryStep = {
  step: string;
  attempt: number;
  error: { code: string; retryAt: string | null; retryAtSource?: "provider" | "worker" } | null;
};

/** Epoch ms at which the worker retries the step on its own, or null when the owner has to resume it. */
export function autoRetryAt(step: CloneRetryStep, now = Date.now()): number | null {
  const { error } = step;
  if (!error?.retryAt || error.retryAtSource === "provider") return null;
  // An image quota pause is resumed by the owner, whatever set its time.
  if (step.step === "images" && error.code === "quota") return null;
  const at = Date.parse(error.retryAt);
  if (Number.isNaN(at)) return null;
  return step.attempt < CLONE_AUTO_RETRY_ATTEMPTS && at - now <= CLONE_AUTO_RETRY_MAX_WAIT_MS ? at : null;
}
