# Targeted finding recheck evaluation

Implementation adds the development case `evals/cases/calibration/dev/ed-recheck-unfixed.json` and a new isolated fixture under `evals/fixtures/ed-recheck-unfixed/`. Existing fixture prose is unchanged. The fixture seeds chapter-scope source reviews and `pending_rechecks` through the real store. The eval collector retains `recheck_results` in current reviews and the archived source rounds; grading now checks `recheck_status` rather than recording SKIP.

No provider A/B evaluation was run: this assignment prohibits real-provider evals. Therefore there is no observed success rate, token/cost delta, or evidence that `unfixed` reaches the acceptance threshold of two out of three runs. That criterion remains unverified, not passed.

The controller owns integrated checks, web build/dist, prompt integration, screenshots and subsequent authorized provider measurements. The bounded host regression uses real store and tools with scripted models; it is not a substitute for provider quality evidence. Unit, integration, race, frontend and build checks were not run by this implementation agent.
