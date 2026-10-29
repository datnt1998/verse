# Previous chapter context experiment

Decision: keep the existing 800-rune `previous_tail`. Remove the temporary runtime knob and runtime-only variants.

Saved run: `workspace/evals/p6-tail-ab/report.json` (ignored), 13 dev cases × 3 repeats × 2 arms. Same binary, prompts and fixtures; variant used the entire previous chapter.

| Metric | 800 runes | Full chapter |
|---|---:|---:|
| Runs without hard failure | 35/39 | 35/39 |
| Critical findings hit | 9/9 | 9/9 |
| False-positive findings | 1 | 0 |
| Writer violations | 0 | 1 |
| Tool errors | 3 | 2 |
| Writer input tokens, 12 runs | 2,882,356 | 3,556,444 |
| Total cost USD | 2.238152 | 2.234677 |

Writer input increased 23.4%. `wr-intent-vs-arrival` regressed from 3/3 to 2/3. The required improvement in writer violations did not occur; violations increased instead. The experiment therefore does not justify changing the default, independently of the measurement limitations below.

## Measurement limitations discovered after the run

- Session logging compacted large `novel_context` tool results. The collector read those placeholders, not the delivered JSON. All six `wr-thread-overdue` runs failed the context assertion for this reason; they do not demonstrate a recall failure.
- The reported `context_trims=0` is **unmeasurable**, not evidence that nothing was trimmed. The saved compact logs cannot recover this information. No context was reconstructed to replace the missing evidence.
- Quote coverage included seeded chapter facts. Those old aggregate quote rates are not used to judge writer compliance.

The eval recording/collection path is being corrected before final acceptance. Retaining the established 800-rune default does not require a new full-tail experiment because the independently observable writer-violation criterion already failed.
