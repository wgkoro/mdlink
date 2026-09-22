# Performance measurements

These reports document historical builds, workloads, and tradeoffs. They are not v1.0.0 benchmarks or performance guarantees for other versions, machines, or inputs.

## Read historical measurements

| Report | Scope |
| --- | --- |
| [Release build, 2026-09-21](release-build-2026-09-21.md) | Local v0.6.0 rebuild: outgoing, backlinks, and all-source unresolved on 3,437 private notes |
| [Backlinks investigation, 2026-09-14](backlinks-2026-09-14.md) | Scanner and worker changes: latency, CPU use, memory, and missed targets |
| [Link-resolution comparison, 2026-09-13](link-resolution-2026-09-13.md) | Synthetic CLI and Go benchmarks before and after link-resolution changes |
| [Development history, 2026-09-03](history-2026-09-03.md) | Safety checks, baselines, profiles, and adopted or rejected optimizations |

Each report links its numerical evidence. Source, candidate, and binary hashes identify the measured builds. The historical code, candidate patches, and private inputs are unavailable for exact reproduction; source hashes do not refer to downloadable public commits.

The JSON files retain sample precision and keep experiments separate. Aggregates without underlying samples are labeled as reported values. These files are evidence, not a stable API. [SHA256SUMS](SHA256SUMS) covers the five Markdown and four JSON files, with paths relative to this directory. Verify them from the repository root:

```sh
(cd docs/performance && shasum -a 256 -c SHA256SUMS)
```

These checksums verify the published files, not the unavailable historical binaries or inputs.

## Recalculate published statistics

Run this example from the repository root with Python 3. It recalculates statistics from the published samples without running a benchmark:

```sh
python3 - <<'PY'
import json
import sys
from pathlib import Path
sys.path.insert(0, "scripts")
from measure import percentile

data = json.loads(Path("docs/performance/data/release-build-2026-09-21.json").read_text())
for command, samples in data["samples"].items():
    seconds = samples["wall"]["seconds"]
    p50 = percentile(seconds, 0.50) * 1000
    p95 = percentile(seconds, 0.95) * 1000
    rss = max(samples["rss"]["bytes"]) / 1048576
    print(f"{command}: p50={p50:.3f} ms p95={p95:.3f} ms max RSS={rss:.3f} MiB")
PY
```

| Command | p50 ms | p95 ms | Max RSS MiB |
| --- | ---: | ---: | ---: |
| outgoing | 31.526 | 33.565 | 14.016 |
| backlinks | 129.925 | 143.626 | 21.141 |
| unresolved | 254.243 | 262.639 | 21.453 |

Quantiles use sorted samples and linear interpolation at `(n - 1) * q`; p50 is the median and p95 the 95th percentile. Use the same helper for other wall-time samples and `statistics.median` for Go ns/op, B/op, allocs/op, or resource CPU times.

Convert seconds to ms by multiplying by 1,000, ns to ms by dividing by 1,000,000, and RSS bytes to MiB by dividing by 1,048,576. Relative change is `(after / before - 1) * 100`; negative wall-time changes indicate faster execution. Max RSS is the largest value from separate resource runs and differs from Go allocation metrics. Keep experiments separate; pair samples only when the recorded method supports it.

## Measure the current checkout

With Go 1.27.1 available, run from the repository root:

```sh
go test ./internal/query -run='^$' -bench='^Benchmark(Outgoing|Backlinks)$' -benchmem -count=5
```

The [query benchmarks](../../internal/query/benchmark_test.go) use the [synthetic generator](../../benchdata/generator/generator.go): seed 20260903, 3,000 notes of 10 KiB, 20 links per note, 10% duplicate-name notes, 20% Unicode-path notes, 5% unresolved links, and 5% ambiguous links. Fixtures are created in temporary directories; no personal input is required.

The timer includes Walk, Catalog, target resolution, query execution, and result checks. It excludes fixture generation, expected-result preparation, CLI startup, and serialization. For a smoke check, replace `-count=5` with `-benchtime=1x -count=1`; do not use that result as a performance baseline.

For new measurements, record the source, toolchain, build flags, CLI binary hash, input selection and exclusions, sample counts, timing and output checks, OS, hardware, runtime settings, power, background load, and limitations. Use the same method before and after a change. These query benchmarks do not reproduce historical CLI timings or all-source unresolved results.

The optional [measurement script](../../scripts/measure.py) requires macOS and hyperfine. It measures outgoing/backlinks with fixed options and discards timed output. It does not reproduce the 2026-09-21 Python timing method or measure unresolved. Define the input, flags, and output checks for each new CLI study. See [Development](../development.md) for routine checks and [Releasing](../releasing.md) for release builds.
