/**
 * Thin bridge between the exported edit actions and the typed command layer.
 * Wrappers build serializable params, run exactly one command and then perform
 * their browser side effects (selection, toast, warm-up) from the result.
 */

import { executeCommands } from '@editor/commands/executor'
import type { CommandIssue } from '@editor/commands/types'
import { getLogger } from '../shared'
import { finishEdit } from './shared'

export type EditCommandOutcome<R> =
  | { ok: true; result: R }
  | { ok: false; issues: CommandIssue[] }

/** Result of every edit command that only needs to report which items it touched. */
export type EditAffected = { affectedIds: string[] } | null

export function runEditCommand<R>(type: string, params: unknown): EditCommandOutcome<R> {
  const outcome = executeCommands([{ type, params }])
  if (outcome.ok) return { ok: true, result: outcome.results[0] as R }

  // Domain rejections carry a `code` (expected no-ops such as an out-of-range split)
  // and are reported to the caller. Anything else is either a malformed command
  // (schema issue, logged) or an exception thrown by the mutation body (rethrown).
  if (outcome.issues.some((issue) => issue.code)) return { ok: false, issues: outcome.issues }
  if (outcome.error) throw new Error(outcome.error)
  getLogger().warn(
    `[${type}] rejected: ${outcome.issues.map((issue) => issue.message).join('; ')}`,
  )
  return { ok: false, issues: outcome.issues }
}

/**
 * Run a command whose result is `{ affectedIds } | null` and perform the shared
 * post-edit side effects (preview warm-up, dirty flag) when it changed something.
 */
export function runAffectedEdit(type: string, params: unknown): string[] | null {
  const outcome = runEditCommand<EditAffected>(type, params)
  if (!outcome.ok || !outcome.result) return null
  finishEdit(outcome.result.affectedIds)
  return outcome.result.affectedIds
}
