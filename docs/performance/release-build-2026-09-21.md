# Release build measurements: 2026-09-21

Backlinks took **129.925 ms p50 / 143.626 ms p95** on 3,437 private notes using a local v0.6.0 rebuild with release settings. The run used a warm filesystem on darwin/arm64. It did not measure a downloaded Release asset or v1.0.0.

[Environment and samples](data/release-build-2026-09-21.json) record `wall.seconds`, `wall.exits`, `rss.bytes`, `rss.exits`, `rss.user_seconds`, and `rss.system_seconds` for each command. Each array has 30 values; resource runs were separate from wall-time runs. See the [statistics example](README.md#recalculate-published-statistics) to recalculate the results.

## Build and environment

| Identity | Recorded value |
| --- | --- |
| Product source | `7a5725e069e4b80a8215e6caacc93b913ffb0347` |
| Measured darwin/arm64 binary SHA-256 | `f2fb2dcfe6eca8ea1f59b010dbd2cbbffaa6d1f91b688ca6a40a3960903628f2` |
| Version / embedded date | v0.6.0 / `2026-09-21T06:08:44Z` |
| Build | Go 1.27.1, CGO_ENABLED=0, `-trimpath`, `-buildvcs=false`, GOARM64=v8.0, release version/commit/date metadata |
| Preparation | Each of four targets built twice with matching hashes; acceptance tests ran only on darwin/arm64 |
| Machine | macOS 27.0, arm64, Apple M4 Pro, 14 logical CPUs, 64 GiB RAM |
| Timer runtime | Python 3.12.13 |
| Measurement window | 2026-09-21 15:13:02–15:13:30 JST |
| Runtime / power | GOMAXPROCS, GOGC, GOMEMLIMIT unset; AC power before and after |
| Load average, 1/5/15 minutes | Before 2.40 / 2.09 / 1.88; after 2.35 / 2.11 / 1.90 |

The JSON includes the historical release-script and helper hashes. The script has since changed; [Releasing](../releasing.md) describes the current procedure. Historical source availability is explained in the [guide](README.md#read-historical-measurements).

## Input and command selection

The catalog contained 3,437 Markdown files (15,525,684 bytes) and 3,505 regular files (52,681,686 bytes). Default `.git`, `.obsidian`, `.trash`, and `.gitignore` exclusions applied, plus `.claude`, `CLAUDE.md`, `Scripts/CLAUDE.md`, and `node_modules` (including one nested dependency directory). There was no root `.mdlinkignore`, and no symlink was followed.

For outgoing/backlinks, Markdown files directly under `Index` were sorted lexically, excluding `AGENTS.md`. Each command used the first candidate with a nonempty result, independent of speed: candidate 1 (242 bytes) for outgoing and candidate 2 (257 bytes) for backlinks. Unresolved selected all 3,437 Markdown sources through a NUL-delimited list, so its JSON included `sources`. All commands used JSON with strict mode and fragment checking disabled.

| Command | p50 ms | p95 ms | Max RSS MiB | Result groups / occurrences | Diagnostics |
| --- | ---: | ---: | ---: | --- | --- |
| outgoing | 31.526 | 33.565 | 14.016 | 3 / 3 | 0 |
| backlinks | 129.925 | 143.626 | 21.141 | 5 / 5 | 1,583 unresolved + 2 ambiguous |
| unresolved, all sources | 254.243 | 262.639 | 21.453 | 917 / 1,583 | 1,583 unresolved + 2 ambiguous |

## Method and limits

Each command had five warmups, 30 wall-time samples, and 30 separate resource samples. Python `perf_counter_ns` measured subprocess startup through completion without a shell. Timed stdout/stderr were discarded. macOS `time -l` supplied RSS bytes and user/system seconds. Every recorded exit code was zero.

Exit codes, stdout, and stderr matched before and after each command and at the final checkpoint, with no read-error, size, encoding, or changed-during-read diagnostics. **Output was checked at checkpoints, not on every timed run.** The full catalog, ancestor metadata, ignore rules, and Git state were unchanged across checks; these were not atomic snapshots.

Caches were warm, and desktop background activity was uncontrolled. Results do not establish performance on cold filesystems or other hosts, or memory use at the maximum input size. The earlier 133.490 ms backlinks result used a different OS, input, build settings, and output handling; comparison with this 129.925 ms result does not establish a speedup.
