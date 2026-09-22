# Backlinks investigation: 2026-09-14

Scanner changes and four workers reduced backlinks p50 on private input from **242.902 to 133.490 ms (-45.0%)**, with higher CPU use and RSS. The candidate missed the 121.451 ms target for halving runtime. These changes were adopted in v0.5.0; the measurements used development builds.

[Numerical evidence](data/backlinks-2026-09-14.json) includes all four final comparisons, exploratory trials, fragment checks, fixture statistics, and correctness checks. Wall times in `final` and exploratory `samples` are seconds; `resources` holds RSS bytes and user/system seconds. `signature` combines the exit code and stdout/stderr hashes. Phase timing fields use milliseconds.

## Builds and method

| Identity | Recorded value |
| --- | --- |
| Base source | `93fa94409b14a5f7d103e29fa7370056537fe079` |
| Candidate patch SHA-256 | `7802a4aeadabf9b63d1b5cb62d087f54691e1f2eb1e9aa956019874b28731770` |
| Candidate scanner SHA-256 | `530c45b236771b09fa0fdf7bb300083c34d2f2a439fa6ffe2243c06ea1f2d50c` |
| Candidate backlinks SHA-256 | `0e5024b5c72be684b9aaecfb488eb913d7a6d8b14704bdc66cf1c0c94abe249d` |
| Baseline binary SHA-256 | `dba1a4252e4e77190c7f368e595fd05c97ce26c1e679592b8c4c315d5f914f69` |
| Combined-four binary SHA-256 | `55aa69c44e95d859acac80ec82d27652cf904c0f285ecebf31a79d816dfb251f` |

Candidates modified the base source; their patch and product hashes identify those changes. Builds used Go 1.27.1 without release ldflags or `-trimpath`, on an Apple M4 Pro running macOS 26.6.2 (darwin/arm64), with default GOMAXPROCS=14. GOMAXPROCS, GOGC, GOMEMLIMIT, and GOFLAGS were recorded as unset after measurement. Power, background load, and exact start/end times were not recorded; the environment record was saved at 01:40:11 JST. Background load was uncontrolled.

The baseline and the candidate combining scanner changes with four workers (`combined4`) were interleaved for 30 timed runs and ten separate resource runs each. Python `perf_counter_ns` measured subprocess startup, output capture, and completion. Every run checked exit/stdout/stderr signatures, with hashing outside the timer. Earlier exploration, correctness checks, and a baseline signature run warmed caches. macOS `time -l` supplied RSS.

Synthetic inputs used seed 20260903, 10 KiB per note, and 20 links per note, as described in the [guide](README.md#measure-the-current-checkout), with the note counts below. Private input contained 3,384 Markdown files, 15,241,881 bytes, and 6,756 links after selection and ignore rules, down from 3,386 files before filtering.

## Final comparison

| Input | p50 baseline → candidate ms | Wall change | p95 baseline → candidate ms | Max RSS baseline → candidate MiB | Half-time target |
| --- | ---: | ---: | ---: | ---: | --- |
| 100 synthetic notes | 10.886 → 6.556 | -39.8% | 11.178 → 6.898 | 11.656 → 13.188 | Missed |
| 3,000 synthetic notes | 251.083 → 106.979 | -57.4% | 257.061 → 111.113 | 20.484 → 25.422 | Met |
| 10,000 synthetic notes | 842.001 → 319.918 | -62.0% | 861.106 → 323.804 | 48.188 → 61.172 | Met |
| 3,384 private-input notes | 242.902 → 133.490 | -45.0% | 246.115 → 138.092 | 18.562 → 20.203 | Missed |

All signatures matched, with exit code zero. On private input, median CPU time rose from 0.16 to 0.22 user seconds and 0.13 to 0.29 system seconds; max RSS grew by about 1.64 MiB. Latency fell while CPU time and memory use rose.

## Exploratory measurements

Five instrumented runs on private input gave approximate phase medians of 105 ms for reading, 84 ms for scanning, 23 ms for Walk, 6.5 ms for UTF-8 validation, 4.1 ms for Catalog, and 2.1 ms for Resolve. They recorded 3,451 files and 1,585 diagnostics. Instrumentation and GC differ from the CLI, so these values cannot be summed to estimate CLI wall time.

Scanner changes alone reduced scanning from about 84 to 79 ms on private input and 136 to 84 ms on 3,000 synthetic notes. The work that can safely be skipped depends on content. The evidence did not show that small resolver changes could remove the remaining approximately 12 ms needed to halve runtime.

Five-sample trials covered baseline, scanner-only, worker-only, and combined variants with 2/4/8 workers, plus later 3/6-worker trials. On private input, combined4 initially gave p50 131.934 ms and combined8 142.743 ms. A later trial gave combined3/4/6 144.201 / 133.759 / 142.562 ms. Eight workers were faster on 10,000 synthetic notes; four was selected based on private and small inputs. This choice may not be optimal on other machines.

A separate five-sample trial varied GOMAXPROCS:

| Runtime | Baseline p50 ms | Combined4 p50 ms |
| --- | ---: | ---: |
| Default GOMAXPROCS | 242.586 | 143.501 |
| GOMAXPROCS=4 | 233.835 | 121.885 |

At GOMAXPROCS=4, candidate/baseline runtime was about 0.521. Compare builds at the same GOMAXPROCS to avoid mixing code changes with runtime-setting effects. GOMAXPROCS limits concurrent Go execution; workers limit concurrent source processing, not CPU count or I/O calls. These five-sample trials are separate from the final 30-sample comparison.

The shared scanner was checked on a small synthetic fragment fixture, with five samples per condition:

| Command / fragments | Baseline p50 ms | Combined4 p50 ms |
| --- | ---: | ---: |
| outgoing-off | 3.955 | 4.014 |
| outgoing-on | 5.200 | 5.269 |
| unresolved-off | 11.821 | 10.131 |
| unresolved-on | 12.578 | 11.047 |

All outputs matched. These results apply only to this small fixture.

## Correctness and limits

Scan, ScanWithRaw, and BuildFragments matched across 5,009 synthetic cases and all 3,384 private Markdown files. Twenty CLI edge cases across twelve variants matched exit/stdout/stderr. Tests, race checks, and vet passed. Stable ordering, root identity checks, no-follow/nonblocking reads, size limits, and before/after read checks were retained.

Rejected shortcuts would have changed behavior: skipping `referenceLabels` without definitions can expose an inner Wikilink; returning early when `[` is absent can remove masks needed for fragment checks; filtering by target name loses read, UTF-8, and unresolved-link diagnostics. [Root-handle reuse](history-2026-09-03.md#root-handle-reuse) also failed to improve performance while retaining safety checks. Persistent indexing and caching were not tested.

Snapshot hashes matched for Markdown size, mtime, and inode outside hidden directories. These checks were not atomic and excluded other catalog entries and ignore-rule metadata. Profiling limitations in this local setup prevented function-level attribution; earlier studies did use pprof.

The filesystem was warm and desktop load uncontrolled. Cold filesystems, other OS/CPU/storage, high concurrent load, and statistical significance were not evaluated. Peak file-descriptor use was not measured: four concurrent reads and four explicit root/input handles do not bound process-wide descriptors. With up to four 64 MiB sources in flight, the observed approximately 20 MiB RSS is not a memory limit. See the [guide](README.md) for historical source availability, recalculation, and current benchmarks.
