# JSON schema version 1

The query commands with `--format json` write one JSON object followed by a newline to stdout. A normal response, even with diagnostics or exit code 3 in strict mode, leaves stderr empty. Counts are included regardless of `--counts`.

```json
{
  "schema_version": 1,
  "command": "outgoing",
  "root": ".",
  "target": "Source.md",
  "results": [
    {
      "path": "Target.md",
      "count": 2
    }
  ],
  "diagnostics": []
}

```

This is the output contract, not a machine-readable JSON Schema file. For command options, see the [CLI reference](usage.md).

<a id="outgoing-and-backlinks-required-fields"></a>

## outgoing and backlinks

| Required field | Type | Meaning |
| --- | --- | --- |
| `schema_version` | integer | `1` |
| `command` | string | `outgoing` or `backlinks` |
| `root` | string | Always `.`: the response's logical root, not the process working directory |
| `target` | string | Resolved CLI target's logical path, preserving its original spelling |
| `results` | array | Aggregated results; `[]` when empty |
| `diagnostics` | array | All diagnostics; `[]` when empty |

Each result has `path` (string) and `count` (integer, at least 1). For `outgoing`, the path is the destination and the count is the number of links to it in the selected source. For `backlinks`, the path is a source and the count is the number of links from it to the selected target. Aliases, fragments, embeds, and self-links count as occurrences.

Results are ordered by `(NFC(path), original path)`. Distinct logical paths are not merged by normalization.

<a id="unresolved-required-fields"></a>

## unresolved

Required top-level fields are `schema_version: 1`, `command: "unresolved"`, `root: "."`, `results`, and `diagnostics`. There is **no top-level `target`**. Empty results and diagnostics are `[]`.

```json
{
  "schema_version": 1,
  "command": "unresolved",
  "root": ".",
  "results": [
    {
      "target": "Missing",
      "count": 2,
      "sources": [
        {
          "path": "notes/a.md",
          "offset": 0
        },
        {
          "path": "notes/b.md",
          "offset": 0
        }
      ]
    }
  ],
  "diagnostics": [
    {
      "code": "unresolved-link",
      "source": "notes/a.md",
      "offset": 0,
      "raw_target": "Missing"
    },
    {
      "code": "unresolved-link",
      "source": "notes/b.md",
      "offset": 0,
      "raw_target": "Missing"
    }
  ]
}

```

| Required result field | Type | Meaning |
| --- | --- | --- |
| `target` | string | The diagnostic's `raw_target`, not a resolved logical path |
| `count` | integer | At least 1; equals the length of `sources` |
| `sources` | array | One entry per occurrence, each with `path` (source logical path string) and `offset` (zero-based byte offset integer) |

Grouping uses exact `raw_target` equality and retains duplicate occurrences within one source. The scanner removes aliases and subpaths; grouping does not additionally decode, expand paths or extensions, change case, or normalize Unicode. Identical relative text from different directories can form one result without referring to the same missing file.

Results are ordered by `(NFC(target), original target)`; occurrences by `(NFC(path), original path, offset)`. Every unresolved occurrence also appears in `diagnostics`. Ambiguous, unsafe, unreadable, and fragment-related problems appear only in diagnostics. Resolution depends on the selected root, exclusions, and symlink permissions, not solely on existence on disk.

When `--source` or `--sources0-from` is used, the optional top-level `sources` field contains the selected logical paths (strings), deduplicated by exact spelling and ordered by `(NFC(path), original path)`. It includes sources that failed to read. Explicitly selecting zero sources produces `"sources":[]`; omitting source selection omits the field.

## Diagnostic

| Field | Type | Presence and meaning |
| --- | --- | --- |
| `code` | string | Required; diagnostic category |
| `source` | string | Required; source or traversal problem's logical path |
| `phase` | string | Present on all diagnostics when source selection or fragment checking is enabled. Values: `catalog` for traversal, `source` for source reading/scanning/resolution, `fragment` for fragment checks; otherwise omitted |
| `offset` | integer | Link-derived diagnostics only; zero-based byte offset in the original source, including zero |
| `raw_target` | string | Link-derived diagnostics only; target extracted by the scanner, including an empty string |
| `candidates` | string array | Ambiguous file links only; all candidate logical paths in stable path order |
| `raw_link` | string | Fragment diagnostics only; complete link syntax at the use site |
| `fragment` | string | Fragment diagnostics only; original fragment including `#` |
| `target` | string | Fragment diagnostics only; resolved logical target path |
| `reason` | string | `unsupported-fragment`: `syntax`, `target-format`, or `target-syntax`; `unverifiable-fragment`: reader code or `invalid-encoding`; omitted for missing/ambiguous fragments |

Offsets count bytes, not characters. Embeds and images point to `!`; reference links point to `[` at the use site. `raw_target` excludes aliases and subpaths and is not the complete link syntax. Reference targets and fragments come from their definitions, while `raw_link` is the usage, such as `[text][id]`. Traversal and source-read diagnostics have no `offset` or `raw_target`.

| Code | Meaning |
| --- | --- |
| `unresolved-link` | No file candidate resolves the target |
| `ambiguous-link` | Multiple file candidates remain |
| `unsafe-path` | Target violates logical path constraints |
| `skipped-symlink` | Symlink was not explicitly permitted |
| `unreadable-file` | File could not be read |
| `file-too-large` | File exceeds the read limit |
| `invalid-encoding` | Source body is not valid UTF-8 |
| `changed-during-read` | File size or modification time changed during reading |
| `missing-fragment` | No matching heading/block ID in the supported profile |
| `ambiguous-fragment` | Multiple matching headings/block IDs |
| `unsupported-fragment` | Fragment form, target format, or target syntax is unsupported |
| `unverifiable-fragment` | Target could not be read or decoded for validation |

Oversized, invalid UTF-8, and changed-during-read sources are skipped. The `backlinks` and `unresolved` commands retain individual source-read diagnostics and continue processing. An unreadable selected `outgoing` source is fatal, so it does not return a JSON response; diagnostics for size, encoding, or change issues alone are nonfatal.

Diagnostics are ordered by source `(NFC(path), original path)`, offset, code, raw target, candidates, phase, fragment, target, reason, and raw link. Missing offsets are compared as `-1`, missing strings as empty strings, and missing candidates as an empty array. Candidate paths use stable path comparison; other strings are compared by original bytes. JSON never truncates diagnostics. Text stderr shows only the first 100 diagnostics plus the total count.

<a id="fragment-diagnostics"></a>

## Fragment diagnostics

With `--check-fragments`, a resolved file's result and count are preserved even if its fragment fails. Fragment failures do not appear in `unresolved.results`.

```json
{
  "code": "missing-fragment",
  "source": "Source.md",
  "offset": 0,
  "raw_target": "Target",
  "raw_link": "[[Target#Absent|label]]",
  "fragment": "#Absent",
  "target": "Target.md",
  "phase": "fragment"
}

```

Target read failures become the reason value for a per-use `unverifiable-fragment` diagnostic, without an additional target reader diagnostic. If the same file is also inspected as a source and fails, its source diagnostic remains separate. Original fragment strings are JSON-escaped without decoding or normalization. Text diagnostics also quote and escape the extra fields. With checking disabled, no fragment diagnostics or fragment-specific fields are added.

See the [supported fragment profile](limitations.md#fragment-validation).

<a id="paths-exit-codes-and-fatal-errors"></a>

## Paths, exit codes, and fatal errors

Source and resolved-target paths use `/` separators relative to the logical root and retain their original spelling. Metadata does not include the configured physical root, canonical path, or external symlink destination. Unresolved targets retain their original text rather than being converted into logical paths.

CR, LF, and tab are represented with JSON escapes. Invalid UTF-8 in output paths is rejected rather than replaced: stdout remains empty, stderr reports an error, and the command exits with code 1. This covers outgoing and backlinks target and result paths, unresolved occurrence paths and result targets, diagnostic source, candidates, and target paths, and optional top-level source paths.

| Exit code | JSON behavior |
| --- | --- |
| 0 | A complete response, even with diagnostics unless strict mode is enabled |
| 3 | A complete response with diagnostics under `--strict` |
| 2 | Invalid CLI arguments, format, root, or target: no response, error on stderr |
| 1 | Fatal I/O, traversal, or output error: no response, error on stderr; a write failure may leave partial output |

Fatal errors take precedence over strict mode. Examples include duplicate physical files and failure to read the selected outgoing source.

Regular `.gitignore` and root `.mdlinkignore` read failures, excessive size, or detected changes are fatal. Invalid patterns, including unsupported POSIX character classes, are reported as `invalid-gitignore` or `invalid-mdlinkignore` on stderr with the logical rule-file path (root `.mdlinkignore` uses `.mdlinkignore`). These failures result in exit code 1 with empty stdout, not a JSON diagnostic response. Links to excluded files use the standard `unresolved-link` code. See [ignore rules](limitations.md#gitignore-scope).

## Consuming version 1

Check `schema_version` and `command`, then consume the fields needed by your application. Do not assume all commands have `target`, or that `sources` is always present. Source selection and fragment checking add fields while keeping the version at 1. Consumers that reject unknown fields must account for these additions; tolerating unused fields avoids coupling to an exact field set. This describes the current format and does not introduce a new compatibility guarantee for future releases.
