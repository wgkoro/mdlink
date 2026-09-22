# Link-resolution comparison: 2026-09-13

On synthetic input, CLI p50 changed by **-6.4% for outgoing, +16.9% for backlinks, and +16.0% for unresolved** after changes to link resolution, fragment checks, and source selection. The results cover the historical builds below.

[Numerical evidence](data/link-resolution-2026-09-13.json) includes ten CLI sample sets, Go query samples, fixture statistics, and validation hashes. Each CLI case used five warmups, 30 wall-time runs, and 30 separate resource runs. `cli_samples` records seconds, exit codes, RSS bytes, and user/system seconds. Each `summary.query` series has five `[ns/op, B/op, allocs/op]` samples, summarized by medians.

## Builds and fixtures

| Build | Original source | Binary SHA-256 | Binary bytes |
| --- | --- | --- | ---: |
| Before | `be4fa07e6af7f71e96e743ade5c9aa321cf93de4` | `e90c69025e37abbacb191994083d4fa5c7e56ffb0b432c120dde9a2bf68e7792` | 2,989,282 |
| After | `38a33ac2b28006d23a18170be67e661e8dafa813` | `d83d6d31bd1f3daecd6360dd3455b634368e343b3589d72d7e8eab7acee4632c` | 3,970,194 |

CLI binaries used Go 1.27.1, CGO_ENABLED=0, `-trimpath`, and `-ldflags='-s -w'`, without the current release metadata. Go query benchmarks used normal test builds. The machine ran macOS 26.6.2 / Darwin 25.6.0 on an Apple M4 Pro (arm64), with 14 logical CPUs, 64 GiB RAM, and Python 3.12.13. Measurement tasks ran sequentially. Desktop load was uncontrolled, and the recorded environment does not establish equivalent power or runtime settings.

Fixture A used seed 20260903: 3,000 notes of 10,240 bytes with 20 links each, totaling 30,720,000 bytes and 60,000 links. It contained 300 duplicate-name notes, 600 Unicode-path notes, 54,000 resolved links, 3,000 unresolved links, and 3,000 ambiguous links. Source and target were the first generated note. The JSON includes generated filenames and expected results.

Fixture B added `FragmentSource.md` with 1,000 `[[FragmentTarget#Alpha]]` links and a 10 KiB `FragmentTarget.md` with heading `Alpha`. The `selected20` set contained FragmentSource and the first 19 Fixture A Markdown paths sorted by `(NFC(path), path)`. Cases measured only after the changes used B; before/after comparisons used A with fragment checks disabled.

## CLI time and memory

| Command | p50 before → after ms | Change | p95 before → after ms | Max RSS before → after MiB |
| --- | ---: | ---: | ---: | ---: |
| outgoing | 26.511 → 24.820 | -6.4% | 28.033 → 26.685 | 12.266 → 14.109 |
| backlinks | 238.854 → 279.139 | +16.9% | 295.549 → 286.157 | 18.375 → 20.297 |
| unresolved | 242.434 → 281.316 | +16.0% | 248.069 → 287.626 | 21.719 → 23.328 |

| Case measured after changes | p50 ms | p95 ms | Max RSS MiB |
| --- | ---: | ---: | ---: |
| fragment-outgoing-off | 26.141 | 27.256 | 14.297 |
| fragment-outgoing-on | 26.006 | 27.424 | 13.875 |
| selected-unresolved-off | 27.501 | 28.773 | 14.172 |
| selected-unresolved-on | 28.073 | 29.712 | 14.406 |

Python `perf_counter_ns` measured subprocess startup through completion without a shell, including CLI work and serialization. Timed stdout/stderr were discarded. Checkpoints verified full output and that selected-source results matched the corresponding subset of all-source results; individual timed outputs were not checked. All timed and resource runs exited zero. macOS `time -l` measured resources in separate runs.

Further investigation would be triggered by either:

- A p50 increase of at least 20% **and** more than 5 ms for outgoing or 50 ms for backlinks/unresolved.
- RSS growth of at least 25% **or** RSS above 200 MiB.

Neither threshold was met, so no additional optimization, profiling, or measurement followed. Earlier targets—outgoing p50 100 ms, backlinks p50 500 / p95 800 ms, RSS 200 MiB, and binary size 20 MiB—were study criteria, not general guarantees.

## Go query and repeated-target results

| Query | Median ns/op before → after | B/op before → after | Allocs/op before → after |
| --- | ---: | ---: | ---: |
| outgoing | 22,560,457 → 22,622,283 | 8,107,950 → 8,118,528 | 66,623 → 66,633 |
| backlinks | 247,335,058 → 280,882,010 | 106,833,763 → 122,022,388 | 275,343 → 302,354 |

Query timing included Walk, Catalog, query execution, and result checks; it excluded startup, serialization, and fixture generation. Allocation metrics are separate from process RSS.

| Repeated links to one fragment target | Median ns/op | B/op | Allocs/op | Target reads per invocation |
| --- | ---: | ---: | ---: | ---: |
| 100 | 37,654 | 109,905 | 665 | 1 |
| 1,000 | 322,813 | 727,111 | 4,286 | 1 |

The repeated-fragment benchmark used an injected reader; its one-read result applies only to that benchmark. CLI outputs matched before and after the changes. Fragment checks found 1,000 links with zero diagnostics. Selected-source results matched the all-source subset, with 13 result groups and 39 diagnostics. The JSON retains output hashes and check results.

## Limits

The study used warm synthetic inputs and did not establish statistical significance or test private input or cold filesystems. CLI, Go query, earlier hyperfine, and later private-input measurements differ in workload and timing boundaries; they cannot establish a single combined speedup. See the [guide](README.md) for source availability, recalculation, and current benchmarks.
