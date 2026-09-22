# CLI reference

Examples assume `mdlink` is on your PATH; see the [quick start](../README.md#quick-start). mdlink reads files without modifying them. For supported syntax and resolution rules, see [scope and limitations](limitations.md); for JSON fields, see the [output contract](json-schema-v1.md).

## Commands

```text
mdlink outgoing [OPTIONS] TARGET
mdlink backlinks [OPTIONS] TARGET
mdlink unresolved [OPTIONS]
mdlink version
```

| Command | Result |
| --- | --- |
| `outgoing` | Files linked from one Markdown source |
| `backlinks` | Markdown sources linking to `TARGET`, which may be an attachment |
| `unresolved` | Unresolved links from all or selected Markdown sources, grouped by original target text |
| `version` | Version, source commit, build date, and Go version |

`TARGET` is a root-relative path or a name that resolves to one file. **Place options before `TARGET`.** `unresolved` takes no target.

## Options and root selection

| Option | Applies to | Meaning |
| --- | --- | --- |
| `--root DIR` | All queries | Markdown directory to scan |
| `--counts` | All queries | Prefix each text result with its count and a tab |
| `--format text\|json` | All queries | Output format; default `text`. JSON always includes counts |
| `--strict` | All queries | Exit 3 on diagnostics; retain results |
| `--exclude PATH` | All queries | Exclude an exact root-relative file or subtree; repeatable, no globs |
| `--no-gitignore` | All queries | Disable `.gitignore` rules only |
| `--follow-symlink PATH` | All queries | Follow this logical symlink path; repeatable |
| `--check-fragments` | `outgoing`, `unresolved` | Validate supported Wikilink headings and block IDs |
| `--source PATH` | `unresolved` | Inspect this exact root-relative `.md` path; repeatable |
| `--sources0-from FILE` | `unresolved` | Read UTF-8, NUL-delimited source paths; at most once; `-` reads stdin |
| `--help` | All queries | Show command usage |

The root is selected in this order:

1. `--root DIR`.
2. The `MDLINK_ROOT` environment variable.
3. The nearest directory containing `.obsidian/`, searching upward from the current directory.

An explicit root can be any Markdown directory; `.obsidian/` is not required. An empty `--root` or no matching root is an input error.

```sh
MDLINK_ROOT=/path/to/notes mdlink outgoing Note
mdlink outgoing --root /path/to/notes --exclude drafts --exclude archive Note
mdlink outgoing --help
```

## Exclude files

mdlink applies `.gitignore` rules at the root and in visited subdirectories. For exclusions specific to mdlink, create `.mdlinkignore` **at the selected root**:

```gitignore
archive/
templates/
generated.md
```

Excluded directories are not traversed. Excluded files are omitted from link scanning and resolution. Links to them become unresolved; selecting an excluded `TARGET` or `--source` causes exit 2.

```sh
mdlink unresolved --root /notes --exclude scratch --strict
mdlink unresolved --root /notes --no-gitignore --exclude node_modules
```

`--no-gitignore` leaves `.mdlinkignore`, `--exclude`, and the default `.git/`, `.obsidian/`, and `.trash/` exclusions active. To exclude `node_modules/`, add an ignore rule or `--exclude`.

Ignore rules support patterns and `!` negation within the same rule set. Neither ignore file can override exclusions from the other. To re-include a child, re-include its excluded parent first. See [ignore rules](limitations.md#gitignore-scope) for syntax, nesting, symlinks, and read errors.

## Follow selected symlinks

Symlinks are skipped with diagnostics by default. Allow each required logical path relative to the root:

```sh
mdlink backlinks --root /path/to/notes --follow-symlink shared Target
```

Alternatively, set `MDLINK_FOLLOW_SYMLINKS` to logical paths separated by the OS path-list separator (`:` on macOS/Linux). Any `--follow-symlink` option replaces this list. Following a directory symlink does not permit nested symlinks. See [filesystem safety](limitations.md#filesystem-safety).

## Select sources for unresolved checks

```sh
mdlink unresolved --root /notes --source Note/A.md --source Note/B.md --strict --format json
printf 'Note/A.md\0Note/B.md\0' | mdlink unresolved --root /notes --sources0-from - --strict --format json
mdlink unresolved --root /notes --sources0-from changed-markdown-paths.nul --strict --format json
```

Combine `--source` and `--sources0-from` as needed. Paths are deduplicated by exact spelling and inspected in stable order. Resolution still uses the **whole root**, after exclusions.

Use exact root-relative `.md` paths. There is no extension completion, basename lookup, Unicode normalization, percent decoding, globbing, or directory expansion. Empty paths, NUL, leading/trailing or doubled `/`, and `.` or `..` components are invalid. Backslash is literal.

Terminate every source-list record with NUL. Empty records and invalid UTF-8 are rejected. An empty list selects **zero sources** unless `--source` adds paths. Omit both options to inspect all sources. Missing, excluded, unfollowed symlink, or non-Markdown sources cause exit 2 before content inspection. Input I/O errors cause exit 1. Prepare any Git diff selection yourself, removing deleted paths.

Even an empty selection builds the root-wide catalog. Traversal diagnostics outside the selection remain visible and can cause strict exit 3. JSON's top-level `sources` includes all selected paths, even unreadable ones. The diagnostic `phase` identifies catalog, source, or fragment problems; see the [JSON contract](json-schema-v1.md#diagnostic).

## Check headings and block IDs

```sh
mdlink outgoing --root /notes --check-fragments --strict --format json Note/A.md
mdlink unresolved --root /notes --source Note/A.md --check-fragments --strict --format json
```

After resolving a file, `--check-fragments` validates supported Wikilink fragments such as `[[Note#Heading]]` and `[[Note#^block-id]]`. Missing, ambiguous, unsupported, or unverifiable fragments produce diagnostics and, with `--strict`, exit 3. File counts are unchanged; fragment failures do not appear in `unresolved.results`.

Markdown anchors, attachment fragments, and hierarchical heading references are unsupported and produce diagnostics when checked. This option is unavailable for `backlinks`. See the [fragment profile](limitations.md#fragment-validation) for supported syntax and matching rules.

## Output and exit codes

Text output lists one path per line for `outgoing`/`backlinks`, or one original target per line for `unresolved`. `--counts` prefixes each line with its occurrence count and a tab. With no results, text output leaves stdout empty.

Each link occurrence counts once, including embeds, links with display aliases or fragments, and self-links. Resolved paths are sorted by `(NFC(path), original path)` without merging distinct spellings. `unresolved` groups exact `raw_target` strings: extension, case, percent-encoding, and Unicode differences remain separate. Identical relative targets from different directories form one group even if they refer to different files.

Text stderr shows the first 100 diagnostics and the total count. JSON includes all diagnostics and leaves stderr empty for normal responses, including strict exit 3. Its `root` is always the logical root `.`. Paths or unresolved targets containing CR, LF, or tab require JSON; text output rejects them as input errors.

| Exit code | Meaning |
| --- | --- |
| 0 | Completed; diagnostics alone do not change the exit code without `--strict` |
| 1 | Runtime error, such as I/O, traversal, or output failure |
| 2 | Input error: invalid arguments or root, or invalid, unresolved, or ambiguous `TARGET` |
| 3 | Diagnostics with `--strict`; results are retained |

Fatal errors take precedence over strict mode and do not produce a JSON response; write failures can leave partial output. See [error handling](json-schema-v1.md#paths-exit-codes-and-fatal-errors).

Try an unresolved-link check in a temporary directory:

```sh
demo=$(mktemp -d)
printf '[[Missing]] [[Missing#Heading|label]]\n' > "$demo/Check.md"
mdlink unresolved --root "$demo" --counts
# stdout: 2<TAB>Missing; stderr: two unresolved-link diagnostics
mdlink unresolved --root "$demo" --format json --strict
# JSON results and diagnostics; exit code 3
rm -r -- "$demo"
```

## Troubleshooting

| Symptom | Check or action |
| --- | --- |
| A file exists but its link is unresolved | Check exclusions, symlink permissions, exact case, and the [resolution rules](limitations.md#resolution-rules). Frontmatter aliases are not used |
| `TARGET` is ambiguous | Use its root-relative path, including the extension |
| No unresolved results, but exit 3 | Inspect diagnostics for ambiguity, skipped symlinks, read failures, or fragment problems |
| Options are rejected or treated as extra arguments | Put all options before `TARGET`; source selection is only for `unresolved` |
| A heading cannot be verified | Check the [fragment profile](limitations.md#fragment-validation) for supported target syntax |
| A source list is rejected | Use exact existing `.md` paths and terminate each record with NUL |
| Results change during editing or sync | Pause edits and sync for repeatable checks; reads are not an atomic snapshot |

Zero unresolved results does not prove a complete scan. Inspect diagnostics, especially in CI.
