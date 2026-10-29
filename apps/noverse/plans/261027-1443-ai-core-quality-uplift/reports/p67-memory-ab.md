# Evidence and thread prompt experiment

Saved run: `workspace/evals/p67-ab/report.json` (ignored). Thirteen dev cases, three repetitions per arm. Both arms used identical evidence/thread code and fixtures; variant changed writer, editor and long-architect guidance.

| Metric | Baseline | Variant |
|---|---:|---:|
| Runs without hard failure | 33/39 | 32/39 |
| Critical findings hit | 9/9 | 9/9 |
| False-positive findings | 3 | 5 |
| Writer violations | 1 | 0 |
| Tool errors | 0 | 1 |
| Quote-backed writer facts, regraded | 73/73 | 62/62 |
| Cost USD | 2.360549 | 2.510510 |

The whole variant is **not accepted as an improvement**: editor false positives increased. `ed-goal-all-met` fell 2/3 → 1/3; `ed-ctrl-future-plan` fell 3/3 → 2/3. `wr-intent-vs-arrival` improved 2/3 → 3/3. These are small, stochastic samples, not proof of causal superiority.

## Selected implementation for final validation

- Writer: retain explicit quote guidance and thread lifecycle instructions. They describe new tool contracts; no claim of a statistically distinguished writer improvement.
- Long architect: describe the expected payoff chapter inside the existing `promise` field, not a new outline schema. The dev set does not independently exercise long-architect planning.
- Editor: reject the broad variant. Keep existing thread-review guidance and add only the evidence contract: passages are original prose, but selected excerpts are not complete history. Absence from an excerpt is not evidence that an event never happened; read the relevant chapter or return unknown.
- Correct wording: the minimum quote length is six Unicode characters, not six words.
- Final dev + held-out runs decide the remaining quality gate. No held-out result was used to tune these prompts.

## Instrumentation caveat

All six `wr-thread-overdue` results in this experiment fail because session logging discarded the actual context payload; this is not evidence of a recall failure. Old context-trim counts are N/A. Corrected collection recovers quote coverage from successful writer commits only, excluding seeded history and editor-only runs. See [capture correction](eval-capture-correction.md).

The recording fix was smoke-tested with the actual seeded overdue fixture, real `ContextTool.Execute`, real session recording, and real collector. The full 15,010-byte payload survives recording; the collected result contains the expected overdue thread and reason without any missing-context marker.
