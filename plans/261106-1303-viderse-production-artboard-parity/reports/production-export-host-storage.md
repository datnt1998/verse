# Production export: host and storage evidence · 2026-11-07

## Outcome

Production exports use **Garage/S3, not the local QA overlay**. Native bounded reads were fast at observation time, but the historical 78-minute browser interval has no saved render/upload/publication phase timing. Its root cause remains unresolved. Source proves an avoidable full-file acquisition before compression; for an original larger than the production cache budget, successful compression reacquires and downloads the same original again. This is a backend-path finding, not an explanation of the browser interval.

No production project, DB row, provider object or configuration was changed. No production deployment, restart, render, generation, pruning or concurrency increase occurred. Only the expressly approved standalone read-measurement helper was built and run; no application build, tests, lint or formatting ran.

## Observed production route and resources

Read-only SQL used `docker exec noverse-postgres-1 psql -X -U postgres -d viderse_go`, explicit `BEGIN READ ONLY`. An initial documentation-example database name `viderse_db` did not exist; database discovery established `viderse_go`. Config fields use Go JSON capitalization; the initial lowercase projection returned null fields and was corrected.

- Active store `9cee9c6da88bf6b12c360466656a0df5`, generation 2: driver `s3`, provider/region `garage`, bucket `viderse`, delivery `proxy`, encrypted credentials present. Inactive previous store is local. No credentials, URLs or launch arguments were printed.
- The current compressed export binding for project `clone-d6d72224-a2ad-45c6-86fa-b4e263555674-g1`, export `e8d0f53f-2ee8-4082-8e69-607bf9ac7015`, is **444,607,339 bytes**, with Garage location verified **2026-11-06 19:32:43.999398 UTC**.
- Garage container data bind is `/Volumes/SSD/garage/data`; metadata is a Docker volume. At **09:36:51 UTC**, the external SSD had **927 GiB available, 3% used** (954 GiB capacity). Targeted `du -sk`: Garage data **6,485,284 KiB**; snapshots **16,936 KiB**. Thus Garage object data is not on the nearly full internal data volume; container metadata/VM and application scratch remain separate concerns.
- At **09:33:55 UTC**, host RAM was **16 GiB**, logical CPUs **10**, uptime **7 days 8:29**, load **3.78 / 4.29 / 4.22**. Internal data volume: **191 GiB used, 6.9 GiB available, 97% capacity**. Swap **6,320.38 MiB used / 7,168 MiB allocated**. VM compressor occupied **424,545 × 16 KiB ≈ 6.48 GiB**; `memory_pressure` reported **38% system-wide memory free**. These are distinct macOS accounting measures, not an additive RAM breakdown.
- At **09:36:04 UTC**, swap was **6,304.38 MiB**. Between the two VM samples, swapins increased **1,350 pages ≈ 21.1 MiB** and swapouts did not increase. Large existing swap use alone does not prove active swap-out thrashing. At **09:36:51 UTC**, `kern.memorystatus_vm_pressure_level` was **2**.
- Initial CPU/RSS snapshot used `comm`, never full arguments: Chrome Remote Desktop **84.2% CPU / 510,832 KiB RSS**, WindowServer **45.3% / 35,776 KiB**, VM process **35.5% / 1,443,472 KiB**, a Chrome renderer **34.0% / 381,536 KiB**, Viderse **0.2% / 32,784 KiB**. These point samples are not historical or peak utilization. At 09:36:51 Viderse RSS was **33,072 KiB**.
- Two live `iostat` intervals after its cumulative first row showed disk0 **15.75 and 1.16 MB/s**. This does not establish latency/saturation or isolate provider I/O. Docker point sample: Garage **0.09% CPU / 48.41 MiB**, Postgres **3.11% / 259.4 MiB**.
- Targeted sizes: production blob cache **1,248,988 KiB**, staging **0 KiB**, clone-staging **2,784,956 KiB**, retained isolated QA root **1,613,888 KiB**. The cache workspace directory was empty. Clone-staging contained two job directories; no costly whole-filesystem walk was performed. These retained files contribute to occupancy, but are not proof of leaked original-upload/cache copies.

## Bounded native provider read

At **09:38:11 UTC**, an approved temporary Go helper read the existing active descriptor and encrypted credentials privately, then called the actual `blobstore.Open` implementation. Its database transaction was read-only; it never called Media.Acquire, publication, lease mutation, cache initialization or object writes.

The retained active-store object selected by the historical original's exact **2,656,758,643-byte** size passed provider Stat size/SHA256-metadata comparison against the blob DB row. The probe labels this `historical-original`; independent comparison against the export lineage's saved original SHA was not available in this probe, so exact lineage identity is not proven solely by the byte-size selection. This does not weaken the route/throughput observation on a real retained production object.

| Operation | Observed wall time | Open time | Read rate |
|---|---:|---:|---:|
| Stat | 6.064 ms | — | — |
| Head range, offset 0, 16 MiB | 103.363 ms | 9.580 ms | 154.79 MiB/s |
| Middle range, offset 1,328,379,321, 16 MiB | 81.521 ms | 2.083 ms | 196.27 MiB/s |

Only **32 MiB total** was read, hashed and discarded. Range SHA256 values were `958f01ba05c3c98e3613ebb2c0b2a3de52ba92f98c362c80b265b4ddfdf155ec` and `6b7fa11c3e3bc7a92936230a1bc0cd911ae6e228e8b2c0d153738874d1c6723b`. This was neither a full-object integrity verification nor a sustained/cold-cache throughput benchmark. Provider/OS cache state was not controlled. Do not extrapolate full-file upload or historical transfer duration from these samples.

Owned remote/local probe binaries and its temporary command directory were removed after measurement. Source is archived outside worktrees at `/tmp/viderse-export-perf-artifacts-20261107/harness-source/readonly-storage-probe.go`; bounded inspection/launcher scripts remain under `/tmp/viderse-*-readonly-20261107.py` and `/tmp/viderse-readonly-storage-launch.py`, without embedded credentials.

## Historical timeline and eliminated explanations

The retained request `73480d9e-f6e6-4ba5-ac3a-d41445fd287a` records:

1. **2026-11-06 18:03:23.226383 UTC**: request created.
2. **18:03:25.531607 UTC**: claimed, **2.305224 seconds** later.
3. **19:21:46.893931 UTC**: done. Created→finished **4,703.667548 seconds**. Terminal progress still says `uploading`, with **2,656,758,643 / 2,656,758,643 bytes**; it contains no per-phase counters.
4. **19:32:43.999398 UTC**: current compressed object location verified. This is **657.105467 seconds after request completion**, but is a location-verification timestamp, not a dedicated compression timer.

Existing `export-performance.md` separately records total export-step **5,361.163 seconds**, browser supervision **4,704.437 seconds**, original **2,656,758,643 bytes**, compressed **444,607,339 bytes**. Those existing totals support an approximately 11-minute post-browser interval, not a pure ffmpeg attribution. A bounded literal search of the 149,401-byte production app log found no export/compression/project/ENOSPC markers. Absence of these strings is a logging gap, not proof no I/O or disk-pressure event occurred.

Competing hypotheses:

- **Uniformly slow current production reads:** not supported by the measured native provider ranges (155–196 MiB/s). Historical stalls, full-file reads/writes, verification and contention remain untested.
- **Original acquisition is duplicated before successful compression:** confirmed by source control flow below, conditional on the active remote route and original exceeding the cache budget. This can contribute only after browser supervision ends; duration contribution remains unmeasured.
- **Internal disk/RAM pressure causes slowdown:** current low internal headroom, compression and swap residency are observed. Historical causal attribution is unproven; no historical resource series exists. External Garage data-volume exhaustion is eliminated for this current snapshot, not for all historical times.
- **Queue delay dominates the historical run:** eliminated by the 2.305-second claim delay. The historical browser phase still cannot be split into rendering, encoding, upload or publication. Prior fast local-output QA runs cannot eliminate production publication latency.

Recent deployment context: existing baseline report records main `2b55c0241ee3c394f09dba89099cb132a1a02c9f`, production PID23940, Nov7 rollout. Today's resource/read samples follow that rollout; the long request was Nov6. They are not same-environment historical measurements.

## Proven critical path and precise timing seams

Line references describe the investigated source before the parent's subsequent edits.

- `internal/editor/exports.go:254–367`: UploadExport appends staged bytes, updates incremental hash, and calls file Sync for chunks. Measure the upload handler separately from commit. Do not put CommitExport wall time into browser byte-upload throughput.
- `exports.go:403–484`: whole CommitExport includes export lock, metadata lookup, full staged `exportFileHash` (420), revision/caption validation, `Gate.Commit` (455), diagnostics persistence, `publishExportFile` (478), staging removal (481). A whole-call `publishMs` is a valid explicitly documented server-commit bucket, but is not pure remote transfer.
- `exports.go:489–535`: publishExportFile performs pointer/resource CAS and calls the existing media publication route. A nested media.Publish timer separates storage publication from other commit/gate work; do not sum inclusive parent and child counters as disjoint phases.
- `internal/media/media.go:152–249`: Publish first hashes each local artifact (158–174), enters repository Publication transaction/fences and key/path locks (177–198), then for S3 performs `store.Put` (216), **full-byte provider Verify readback** (224), Bind (229), and domain commit callback (233–235). It does not warm the working cache. `platform/blobstore/verify.go:15–36` proves Verify reads the full expected object, bounded at size+1, not merely HEAD/ETag.
- `internal/render/export.go:27–35`: browser supervisor returns before `AcquireExport` (31), then the handle is immediately released (35). This acquisition exists even when compression is disabled.
- `internal/media/media.go:388–425`: remote Acquire resolves and hydrates through the working cache. Local Acquire returns the existing file directly; these paths are not interchangeable.
- `platform/blobstore/config.go:30–54`, `cmd/viderse/media_wiring.go:86–90`: production default cache budget is **2 GiB**. All BLOB_CACHE_* variables were unset in the deployed app environment. **512 MiB is the prior QA helper budget, not the production default.** The original 2,656,758,643 bytes exceeds both.
- `internal/media/cache.go:49–62` and `cache_recovery.go:65–98`: oversized remote input always uses a new workspace/full download; Release removes that workspace (85–90). `cache.go:182–215` downloads all bytes, hashes, verifies and Syncs. The first Acquire→Release therefore cannot leave a reusable oversized cache entry.
- `render/export.go:60–73` → `exports.go:740–837`: ReplaceCloneExportFile reacquires the same original (782), runs the transform (800), releases its source handle before publication (809), hashes replacement (810), then publishes it (834). Preserve release-before-publication: retaining the lease can deadlock pointer retirement, as the source comment explicitly warns.

Recommended backend timing boundaries for the parent: initial validation/acquisition wall time; replacement acquisition/download (782); complete CompressExportFile callback (800), with ffmpeg/process/probe subdivisions only if separately instrumented; replacement hash (810); replacement publication whole call (834), optionally nested hash/Put/Verify/DB commit. Record ffmpeg/tool-resolution separately if needed. Keep all timers finite/nonnegative and label inclusive/overlapping buckets explicitly. Browser renderMs and residual encoderWaitMs are not additive pure GPU/encoder hardware durations.

The parent's proposed targeted fix—avoid initial Acquire on successful compression, leaving ReplaceCloneExportFile as the single acquisition—fits the proven duplication. Preserve fallback integrity checking for compression-disabled, compression-failed and tool-resolution-failed paths, and original cancellation/error semantics. Moving template/tool parsing before acquisition changes which original record accompanies their errors unless deliberately preserved. No cache-budget change, verification bypass or borrowed-handle lease change is needed.

## Parent integration evidence

After this investigation, Main reported an actual native RunExport 30-second smoke with compression applied and unchanged quality flags. Across the complete path, original-file acquisitions decreased **3→2**, not 2→1: the browser-completion integrity acquisition at `internal/web/render.go:261` remains intentionally intact, while backend postprocessing acquisitions decreased **2→1**. For its 21,471,028-byte source, acquired source bytes decreased **64,413,084→42,942,056**. This is parent-reported evidence, not a check executed by this investigator.

That QA smoke used an **8 MiB export cache** to force oversized behavior, with **512 MiB for input assets and local output overlay**. It was not a production wire-throughput benchmark. Observed local wall times **10.561→10.831 seconds** establish no speed improvement. Main also added a separate `export_completion_acquire` timer because the preserved completion acquisition sits outside publishMs. Production rollout and rollout smoke belong to Main.

## Prevention and remaining uncertainty

Prioritize measured cumulative frontend phases and backend commit/acquire/compression/publication timers over further speculative rendering changes. Observe RAM/swap deltas and both internal scratch headroom and external Garage-volume headroom during the next authorized bounded run. An internal-disk headroom alert and actual swap-I/O/pressure trend are more discriminating than swap allocation alone. Do not prune user data as an investigative shortcut.

Unresolved: the historical rendering versus upload/publication split; historical CPU/GPU/memory/disk state; actual full-file acquisition/Put/Verify/compression durations; independently binding the ranged retained object's SHA to the original export lineage. The report establishes current route/resources, a safe bounded native read result, and a proven duplicate-acquisition mechanism—not the historical 89-minute root cause.
