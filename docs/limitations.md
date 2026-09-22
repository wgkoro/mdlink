# Scope and limitations

This document defines mdlink's supported syntax and behavior. See the [CLI reference](usage.md) for options and [JSON schema v1](json-schema-v1.md) for output fields and diagnostics. mdlink does not implement all CommonMark syntax or validate Markdown anchors.

## Supported links

| Form | Behavior |
| --- | --- |
| `[[Note]]`, `[[Note\|label]]`, `![[Note]]` | Resolve the file; count each occurrence |
| `[[Note#Heading]]`, `[[Note#^id]]` | Resolve the file; validate the fragment only with `--check-fragments` |
| `[label](destination)`, `![alt](destination)` | Resolve a Markdown destination |
| `[text][id]`, `[id][]`, `[id]`, and reference images | Use a single-line definition in the same source |
| Links in inline/fenced code, HTML comments, or `%%...%%` comments | Ignore |
| External URI schemes and `//` destinations | Ignore in both results and diagnostics |

Aliases, fragments, embeds, and self-links count toward the resolved file. Frontmatter alias properties are not searched.

Markdown destinations can be bare or enclosed in `<angle brackets>`. Bare destinations allow balanced or escaped parentheses; angle brackets make parentheses literal. Backslash escapes apply only to ASCII punctuation. Percent decoding runs once, preserves `+`, and must produce valid UTF-8. Queries and fragments are separated before decoding, so encoded `#` and `?` remain part of the path. External URI detection runs before and after decoding.

## Titles and malformed input

Single-line quoted titles are supported, including empty titles:

```markdown
[label](Target.md "title")
![alt](<Target.md> 'title')
```

Separate the destination and title with an unescaped space or tab. Parenthesized and multiline titles are unsupported. Only a valid trailing title is removed: a quote followed by path characters such as `.md` can belong to a bare path. Use angle brackets or percent encoding for quoted filenames. After an angle destination's `>`, only spaces/tabs, an optional quoted title, and the closing `)` are allowed.

Unclosed labels or destinations, unbalanced bare parentheses, multiline destinations, invalid percent escapes, and invalid decoded UTF-8 produce no link. Scanning continues for later candidates. A malformed title consumes the candidate through its balanced closing `)` or the end of the line; apparent links within it are ignored.

## Resolution rules

Resolved paths stay within the selected root's logical directory tree, including explicitly followed symlinks. Paths containing NUL or escaping the root are rejected: link occurrences receive `unsafe-path`, while invalid CLI targets are input errors. Attachments require an extension; extensionless files are not candidates.

Matching is case-sensitive. Stages run in order and stop at the first match; multiple candidates at that stage produce an ambiguity diagnostic.

| Input | Resolution order |
| --- | --- |
| CLI `TARGET` and Wikilink file target | Exact root-relative path → root-relative path with `.md` appended → basename/Markdown stem for names without `/` → NFC fallback |
| Markdown target containing a `.` or `..` path component | Resolve relative to the source directory, then try that exact path → append `.md` → NFC fallback; no basename search |
| Markdown target starting with `/` | Resolve from the root, then try that exact path → append `.md` → NFC fallback; no basename search (`//` is external) |
| Other Markdown target | Exact root-relative path → exact source-relative path → root-relative path with `.md` appended → basename/Markdown stem for names without `/` → NFC fallback |
| `--source PATH` | Exact original root-relative `.md` path only; no fallback |

`.md` is appended only if the target does not already end in `.md`. NFC fallback repeats the applicable stages using Unicode NFC-equivalent spellings; distinct matching files remain ambiguous. Targets with an explicit directory never fall back to an unrelated basename. Use `./Note` to request source-relative extension completion.

An empty file target with a fragment refers to the source file; without a fragment, it is unresolved. Fragment validation is a [separate check](#fragment-validation).

## Reference links

Definitions occupy one physical line with zero to three leading spaces and may precede or follow their uses:

```markdown
See [the guide][guide].

[guide]: <Guide with spaces.md> "Optional title"
```

List/blockquote prefixes such as `- [id]:` or `> [id]:` and multiline definitions are unsupported. Surrounding containers are not interpreted: a definition with up to three leading spaces is recognized even after a list. Bare destinations cannot contain unescaped spaces/tabs; use angle brackets or percent encoding.

Labels must be nonempty, single-line, at most 999 Unicode code points, and contain no unescaped brackets. Matching folds Unicode case, collapses runs of Unicode Zs spaces, tabs, and form feeds to one space, and trims the ends. It does not normalize to NFC, decode entities, or remove backslashes. Display text in a full reference may be empty or contain balanced brackets and has no 999-code-point limit.

The first syntactically valid definition wins, even with an external or undecodable destination. Undefined references are ordinary text. Valid inline links take precedence; full and collapsed references consume their adjacent suffix even when undefined, without falling back to shortcuts. An unclosed suffix allows later independent links to be scanned.

Lines beginning with a definition prefix are excluded from link scanning, even if malformed. Trailing comments are unsupported; links, backticks, and comment delimiters within a definition are not reinterpreted. Definitions inside code or comments are ignored. Shortcut candidates do not hide code/comment delimiters extending beyond their label.

Counts and offsets refer to each use. Targets and fragments come from the definition, while a fragment diagnostic's `raw_link` contains the reference at the use site. Definitions apply only within their source file.

## Fragment validation

`--check-fragments` enables the following checks for `outgoing` and `unresolved`. Fragment failures appear only in diagnostics; they do not change resolved file counts or add entries to `unresolved.results`.

| Target | Supported matching |
| --- | --- |
| Wikilink `#Heading` | ATX/Setext headings, including lists and blockquotes; plain text, emphasis, code spans, link text, and image alt text |
| Wikilink `#^id` | Explicit block IDs at the end of a paragraph/text block, including lists and blockquotes |
| Markdown anchors, non-Markdown targets, empty fragments, hierarchical heading references | `unsupported-fragment` |

Heading text is concatenated, line breaks become spaces, surrounding spaces/tabs are trimmed, and Unicode NFC normalization is applied. Fragments receive only NFC normalization. Matching is case-sensitive and preserves internal whitespace and percent encoding.

A heading containing unsupported syntax makes **all heading references to that target** unsupported. This includes backslashes/entities in ordinary text, raw HTML, autolinks, and Wikilink-like text. Backslashes and `&` in code spans are literal. Block-ID checks remain independent. HTML `h1`, `id`, and `name` anchors are not checked.

Block IDs are case-sensitive: `^` followed by ASCII letters, digits, or hyphens, at the start of text or after a space/tab. An ID-only paragraph is supported. IDs at intermediate line endings or within headings, table syntax, code, link/image labels, or escaped text are unsupported.

Frontmatter begins with a standalone `---` on the first line, after an optional BOM, and ends at a standalone `---` or `...`. It is excluded from matching; unclosed frontmatter makes the entire target unsupported. Code and HTML blocks contribute no headings or IDs. HTML and `%%...%%` comments are excluded, respecting code and link boundaries. A code, HTML, or inline construct that starts inside a comment and extends into non-whitespace text outside it makes the target structurally ambiguous and unsupported.

No matches produce `missing-fragment`; multiple matches produce `ambiguous-fragment`. Read failures, excessive size, changes during reading, and invalid UTF-8 produce `unverifiable-fragment`. Failures are reported per use. Target links are not recursively scanned.

## .gitignore scope

mdlink reads regular `.gitignore` files at the root and in visited subdirectories. Rules are relative to their containing directory and apply even to files tracked by Git. Git is not required. Ancestor rules outside the root, global excludes, `.git/info/exclude`, and Git index/config are not read.

Supported syntax includes blank lines, leading `#` comments, `!` negation, backslash escapes, CRLF, leading/intermediate `/`, directory-only trailing `/`, `*`, `?`, character sets/ranges/negated sets, and `**`. Unescaped trailing spaces are removed. Matching is case-sensitive without Unicode normalization; braces are literal. Invalid patterns and POSIX character classes such as `[[:digit:]]` produce `invalid-gitignore` and exit code 1. Full Git compatibility is not guaranteed.

Later lines and deeper rule files take precedence. Excluded directories are not visited: re-include the parent before re-including a child. Rules inside the parent are read only once it is visited. Neither `!` nor `--no-gitignore` overrides `--exclude` or the default `.git/`, `.obsidian/`, and `.trash/` exclusions at every level. Rule files are not implicitly excluded from the catalog; excluding them does not disable their rules.

Exclusions apply before symlink resolution and duplicate-file detection. Follow requests inside excluded subtrees are treated as handled; other unreachable follow requests are input errors. Directory-only patterns do not match symlinks. Regular `.gitignore` files inside explicitly followed directories apply beneath their logical mount paths, without physical ancestor rules. Symlinked rule files are never read as rules, even when followed, but retain normal catalog behavior. Other non-regular rule files are also ignored.

Rule files have the same safe-read checks and 64 MiB limit as Markdown bodies. Read failures, replacement, excessive size, or detected changes are fatal: mdlink reports the logical path on stderr, exits 1, and leaves stdout empty. Excluded files cannot resolve as link targets or be explicitly selected as sources.

## .mdlinkignore scope

Only the regular `.mdlinkignore` file directly under the selected root is read, once. A missing or empty file adds no exclusions. Syntax, read checks and failures, symlink handling, and rule-file catalog behavior follow the [.gitignore rules](#gitignore-scope). Invalid patterns instead produce `invalid-mdlinkignore` with source `.mdlinkignore`.

A file is excluded if either rule set excludes it. `!` only overrides earlier rules in the same set; it cannot override the other set, `--exclude`, or default exclusions. Re-include an excluded parent before its child. `.mdlinkignore` remains active with `--no-gitignore`.

Nested, ancestor, and external followed-directory `.mdlinkignore` files are not read as rules. Root rules apply to followed targets through their logical paths. Excluded follow requests, links, and explicit sources behave as described above.

## Filesystem safety

mdlink does not modify documents. Symlinks are skipped with diagnostics unless explicitly permitted; permissions do not extend to nested symlinks. Cycles and duplicate physical files are rejected. Reads check root boundaries and regular-file identity, with a 64 MiB limit per Markdown body.

Sources that exceed the size limit, contain invalid UTF-8, or change during reading are skipped with diagnostics. `backlinks` and `unresolved` continue after individual read failures. An unreadable `outgoing` source causes a fatal runtime error; size, encoding, and file-change diagnostics alone are nonfatal.

Size and modification time are checked before and after reading, without automatic retries. Reads are not atomic snapshots: checks can miss changes that restore both size and time, or a pathname replacement that leaves an open file unchanged. Pause editing and sync for repeatable results.

## What unresolved checks establish

`unresolved` checks links against the root's catalog after exclusions and symlink settings. An unresolved link does not prove the file is absent from disk. Unsupported syntax may produce no link; fragments are checked only when requested.

Results group occurrences by exact original target text. Case, percent encoding, NFC/NFD, and extension differences remain separate; identical relative text in different source directories is grouped together. Result counts therefore do not count distinct missing files.

Every unresolved occurrence has a diagnostic. Ambiguous/unsafe links and read or fragment failures appear only in diagnostics. Zero results with diagnostics does not establish a complete scan. Use `--strict` and inspect diagnostics for linting.

[Source selection](usage.md#select-sources-for-unresolved-checks) limits content inspection, not the root-wide catalog. An empty selection never falls back to all sources. Traversal diagnostics outside the selection remain visible.

## Resource use

`backlinks` reads and parses up to four sources concurrently with a fixed worker count and stable output ordering. Concurrent bodies, parser data, and open files increase resource use. The 64 MiB body limit is not a process memory limit; speedups depend on CPU, storage, and content.

`unresolved` processes sources sequentially and retains all unresolved occurrences and diagnostics, so memory and JSON size grow with output. Fragment checks also read target bodies and retain fragment indexes for the current source. There is no persistent cache, saved graph, or collection of every source body. See [performance measurements](performance/README.md) for tested versions and conditions.
