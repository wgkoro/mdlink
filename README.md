# mdlink

A read-only CLI for inspecting outgoing links, backlinks, and unresolved references in Markdown files.

- No persistent index
- No database
- No daemon
- No initialization 

Just a single binary that reads your files as they are.

```sh
mdlink outgoing --root . architecture.md   # What this file links to
mdlink backlinks --root . architecture.md  # What links to this file
mdlink unresolved --root . --strict        # Check for unresolved links
```

Use mdlink in any Markdown directory. It supports Markdown links and Wikilinks, with no runtime dependency on Git or a network connection. JSON output and exit codes make it easy to use in shell scripts and CI.

## Installation

Download the archive from the [latest release](https://github.com/wgkoro/mdlink/releases/latest), then place the binary for your OS and CPU on your PATH as `mdlink`. You do not need Go installed to run it.

The release archive includes binaries for all four platforms. `<version>` below is the release tag:

| Platform | Binary |
| --- | --- |
| macOS / Apple Silicon | `mdlink-<version>-darwin-arm64` |
| macOS / Intel | `mdlink-<version>-darwin-amd64` |
| Linux / x86_64 | `mdlink-<version>-linux-amd64` |
| Linux / ARM64 | `mdlink-<version>-linux-arm64` |

For example, on macOS with Apple Silicon, download the archive into an empty directory and run the following there. Keep only one release archive in this directory so each wildcard matches a single file:

```sh
tar -xzf mdlink-*.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 mdlink-*-darwin-arm64 "$HOME/.local/bin/mdlink"
export PATH="$HOME/.local/bin:$PATH"
mdlink version
```

To keep the command available in future shell sessions, add `$HOME/.local/bin` to your shell's PATH configuration.

### Build from source

Install the Go version specified in [go.mod](go.mod), then run the following from the root of your cloned repository. The first build downloads the required Go modules.

```sh
go build -o mdlink ./cmd/mdlink
./mdlink version
```

A regular source build reports its version as `dev`. Place the binary on your PATH to use the examples below.

## Quick start

Create two Markdown files in a temporary directory:

```sh
demo=$(mktemp -d)
printf '# Alpha\n\n[[Beta]]\n[Beta](Beta.md)\n' > "$demo/Alpha.md"
printf '# Beta\n' > "$demo/Beta.md"

mdlink outgoing --root "$demo" --counts Alpha.md
# 2<TAB>Beta.md

mdlink backlinks --root "$demo" --counts Beta.md
# 2<TAB>Alpha.md

mdlink unresolved --root "$demo" --strict
# No output; exit code 0
```

`--counts` reports link occurrences. A Wikilink and a Markdown link to the same file each count as one occurrence. `<TAB>` represents a tab character.

## Usage

| Command | Result |
| --- | --- |
| `outgoing [OPTIONS] TARGET` | Files linked from a Markdown file |
| `backlinks [OPTIONS] TARGET` | Markdown files that reference the target; the target may also be an image or another attachment |
| `unresolved [OPTIONS]` | Links that cannot be resolved, grouped by their original target text |

`TARGET` is a path relative to the root, or a name that resolves to a single file. **Place options before `TARGET`.**

### Choose a directory

Use `--root DIR` to select the Markdown directory to scan. Use `--root .` for the current directory.

If you omit `--root`, mdlink uses the `MDLINK_ROOT` environment variable, then falls back to the nearest directory containing `.obsidian/`, searching from the current directory up through its ancestors. If neither is available, the command returns an input error.

### JSON output

Using the files from the quick start:

```sh
mdlink outgoing --root "$demo" --format json Alpha.md
```

```json
{
  "schema_version": 1,
  "command": "outgoing",
  "root": ".",
  "target": "Alpha.md",
  "results": [
    {
      "path": "Beta.md",
      "count": 2
    }
  ],
  "diagnostics": []
}

```

Results have a stable order. JSON output includes occurrence counts and diagnostics. See [JSON schema v1](docs/json-schema-v1.md) for field definitions.

### Check unresolved links in CI

After installing `mdlink` in your CI environment, run:

```sh
mdlink unresolved --root docs --strict --format json
```

`--strict` covers **all diagnostics**, including unresolved links, skipped symlinks, and read failures.

| Exit code | Meaning |
| --- | --- |
| 0 | Success. Without `--strict`, diagnostics alone do not change the exit code. |
| 1 | Runtime error, such as an I/O, traversal, or output failure |
| 2 | Input error, such as an invalid argument, root, or target |
| 3 | Diagnostics were reported with `--strict`; results are still returned |

Text output sends results to stdout and diagnostics to stderr. JSON output includes diagnostics in the `diagnostics` field. See the [output contract](docs/json-schema-v1.md) for details.

## Common options

| Option | Purpose |
| --- | --- |
| `--counts` | Include link occurrence counts in text output |
| `--format json` | Return JSON for use in scripts |
| `--strict` | Exit with code 3 if any diagnostics are reported |
| `--exclude PATH` | Exclude a file or directory relative to the root. Repeatable; no glob patterns |
| `--no-gitignore` | Disable automatic `.gitignore` rules |
| `--source PATH` | Limit the Markdown sources inspected by `unresolved`. Repeatable |
| `--check-fragments` | Validate Wikilink headings and block IDs in `outgoing` or `unresolved` |

Run any command with `--help` for usage information. See the [CLI reference](docs/usage.md) for advanced options, including NUL-delimited source lists and explicitly following symlinks.

<a id="mdlinkignore-による追加除外"></a>

## Exclude files with .mdlinkignore

mdlink applies `.gitignore` rules by default. To exclude files from mdlink while keeping them in Git, add a `.mdlinkignore` file at the root:

```gitignore
archive/
templates/
generated.md
```

Excluded files are also removed from link resolution candidates, so links to them become unresolved. `.mdlinkignore` remains active even with `--no-gitignore`. See the [exclusion rules](docs/limitations.md#mdlinkignore-scope) for scope and re-inclusion behavior.

## Supported links

| Syntax | Behavior |
| --- | --- |
| `[text](file.md)` / `![alt](image.png)` | Resolved to the destination file |
| Reference links such as `[text][ref]` | Resolved using single-line definitions within the same file |
| `[[Note]]` / `[[Note\|label]]` / `![[Note]]` | Resolved to the destination file |
| `[[Note#Heading]]` / `[[Note#^block-id]]` | Resolved to the destination file; add `--check-fragments` to validate the heading or block ID |
| External URLs and links in code, HTML comments, or `%%...%%` comments | Ignored |

mdlink supports the syntax and resolution rules described in [scope and limitations](docs/limitations.md). It does not implement all CommonMark syntax or validate Markdown `#anchor` targets.

## Performance and limitations

Each invocation reads the file tree and the content it needs, without maintaining a persistent index. In a local release-settings build of the v0.6.0 code, `backlinks` across 3,437 Markdown files completed with a p50 of 130 ms and p95 of 144 ms. That measurement used an Apple M4 Pro under macOS 27.0 with a warm filesystem ([2026-09-21 measurement report](docs/performance/release-build-2026-09-21.md)).

The [performance guide](docs/performance/README.md) covers all three commands, conditions, and limits.

- Documents are never modified. Symlinks are skipped and reported by default.
- Name resolution is case-sensitive and does not use frontmatter aliases. Ambiguous names are reported rather than resolved to an arbitrary file.
- A link is unresolved when no file matches it under the selected root after applying exclusions. This does not necessarily mean the file is absent from disk.
- Zero unresolved links do not guarantee a complete scan if other diagnostics, such as read failures, were reported.

<a id="配布物の準備"></a>

## Documentation

Use the CLI reference for commands, the scope and limitations guide for supported behavior, and the JSON schema for scripting.

- [CLI reference](docs/usage.md): all options, source selection, output, and fragment validation
- [Scope and limitations](docs/limitations.md)
- [JSON schema v1](docs/json-schema-v1.md)
- [Development](docs/development.md): prerequisites, checks, and contributing
- [Release procedure](docs/releasing.md): packaging, publication, and downloaded-asset verification
- [Performance](docs/performance/README.md): release-build measurements and benchmark commands
- [Changelog](CHANGELOG.md): changes by version
- [v1.0.0 release notes](docs/release-notes/v1.0.0.md)
- [Releases](https://github.com/wgkoro/mdlink/releases)

Release archives include the CLI reference, scope and limitations, the JSON schema, current release notes, README, license, and third-party notices. The changelog, build inputs, development/release guides, and performance reports are repository-only references: open those links in the [source repository](https://github.com/wgkoro/mdlink) at the full source commit reported by `mdlink version`. This also applies to repository-only links within the bundled documents.

Releases prepared with the current procedure attach a public verification JSON report alongside the archive and its external checksum. These reports identify the source commit, build settings, hashes, and executed acceptance host. Verification recorded after the build is separate from the source commit; see the [release procedure](docs/releasing.md).

## License

[MIT License](LICENSE). See [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES.md) for notices covering the Go runtime, standard library, and dependencies.
