# Development and release preparation

Run all commands from the repository root. To install a release binary, see the [README](../README.md#installation).

## Prerequisites

- Go 1.27.1, as specified in [go.mod](../go.mod) and CI. The first build may download dependencies.
- Python 3.11 or later for the scripts and their tests. No third-party Python packages are required.
- A C compiler and CGO enabled for `go test -race`.

Ensure `go` and `gofmt` are on your PATH. A directory-aware version manager may make them unavailable outside the repository.

## Build

```sh
go build -o mdlink ./cmd/mdlink
./mdlink version
```

This build reports version `dev`, with the commit and build date set to `unknown`.

## Run checks

The [CI workflow](../.github/workflows/ci.yml) runs the Go checks below. The [release workflow](../.github/workflows/release.yml) also runs the Python tests and validates release artifacts.

```sh
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
python3 -m unittest discover -s scripts
```

`gofmt -l .` must print no paths. If it lists files, format them with `gofmt -w path/to/file.go` and check again. Formatting differences alone do not cause a nonzero exit status. All other commands must succeed.

Tests use temporary fixtures; no Obsidian vault or application is required.

For benchmarks and measurement conditions, see [Performance](performance/README.md).

## Contributing

For bug reports, include `mdlink version` output, your OS and architecture, the command with its options, and the expected and actual results. For link-resolution issues, provide a small synthetic Markdown example with relevant ignore rules. Include JSON output when diagnostics are relevant.

Keep changes focused, explain their purpose, and list the checks you ran. Add regression tests for behavior changes and update the relevant [CLI reference](usage.md), [limitations](limitations.md), or [JSON contract](json-schema-v1.md).

## Prepare release artifacts

Follow the [release procedure](releasing.md) for additional prerequisites, source approval, artifact preparation, publication, and verification of downloaded assets.

The release script builds each of the four platform binaries twice from committed `HEAD`. Acceptance tests run only on the current host.
