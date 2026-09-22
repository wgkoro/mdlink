# Releasing mdlink

This is the maintainer procedure for preparing, publishing, and verifying a release. The current script and workflow are configured for **v1.0.0**, the pending initial public release, which maintains the established CLI behavior. Never replace an existing published tag or asset.

## Prerequisites and source checks

Run commands from the repository root and stop if any required command fails. Required tools are Go **1.27.1**, Python **3.11 or later**, Git, and `tar`. Host acceptance testing supports macOS and Linux on arm64 or amd64; Linux also requires `file` and `readelf`. Race tests require a C compiler and CGO. The first Go build may download modules. Publishing additionally requires the GitHub CLI (`gh`), authenticated with access to the target repository.

```sh
export PYTHONDONTWRITEBYTECODE=1
command -v go
command -v gofmt
go version
go env GOPATH GOCACHE
```

A directory-aware version manager may remove Go from PATH outside the repository. If testing a separate checkout, resolve the Go and gofmt executable paths here and explicitly add their directory to PATH in that shell. Do not change global Go settings to accommodate a temporary directory.

For a future release, update `VERSION` and the release-note path in `DOCUMENTS` in [release.py](../scripts/release.py), the version-contract test in [test_release.py](../scripts/test_release.py), the tag trigger and artifact/archive names in the [release workflow](../.github/workflows/release.yml), along with the release notes and documentation. Keep the platform matrix and exact toolchain aligned.

Add a concise entry to the top of [CHANGELOG.md](../CHANGELOG.md), linking to that version's release notes. Keep detailed notes in `docs/release-notes/<version>.md` for inclusion in the release archive.

Before source approval, run:

```sh
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
python3 -m unittest discover -s scripts

go test ./internal/markdown -run='^$' -fuzz=FuzzScanner -fuzztime=10s
go test ./internal/markdown -run='^$' -fuzz=FuzzFragments -fuzztime=10s
go test ./internal/root -run='^$' -fuzz=FuzzLogicalPath -fuzztime=10s
go test ./internal/link -run='^$' -fuzz=FuzzResolvePath -fuzztime=10s
go test ./internal/app -run='^$' -fuzz=FuzzSourceInput -fuzztime=10s
git diff --check
```

Formatting must print no paths; every other command must succeed. These checks use fixtures. No personal Markdown directory, Obsidian application, agent skill, internal log, or old tag is needed.

Commit all reviewed changes and ensure the checkout is clean, including the release script itself:

```sh
test -z "$(git status --porcelain --untracked-files=all)"
release_sha=$(git rev-parse HEAD)
release_version=$(python3 -c 'import sys; sys.path.insert(0, "scripts"); import release; print(release.VERSION)')
```

The script builds binaries and copies documents extracted from `git archive HEAD`. Because the release script executes directly from the working tree, uncommitted script edits must not be used. The embedded date is the source commit's timestamp in UTC. No prior Git history is required; this procedure works even in a repository with only one commit. Any records added afterward belong in a separate commit; never move the release tag to a later verification-log commit.

## Prepare and verify locally

Choose new output paths inside a dedicated temporary directory:

```sh
release_parent=$(mktemp -d)
mkdir "$release_parent/upload"
release_archive="mdlink-$release_version.tar.gz"
python3 scripts/release.py --output "$release_parent/artifact" \
  --archive "$release_parent/upload/$release_archive" \
  > "$release_parent/upload/verification-local.json"
```

Require a zero exit status before using the outputs. The upload directory should contain only the archive, `<archive>.sha256`, and the public verification JSON report. Keep raw command transcripts and debugging output outside it.

The archive contains four binaries, `SHA256SUMS`, and exactly these documents: `LICENSE`, `README.md`, `THIRD_PARTY_NOTICES.md`, `docs/usage.md`, `docs/limitations.md`, `docs/json-schema-v1.md`, and the current release notes. Development/release guides and benchmark reports stay in the repository. View any repository-only files referenced by bundled documents at the reported source commit.

Each target is built twice with CGO disabled and `-trimpath -buildvcs=false`. Binary hashes must match. Each binary's toolchain, target, CPU baseline, and recorded build settings are checked. Host execution checks the embedded version, source commit, and date against the checkout, plus help, text and JSON results, counts, diagnostics, strict exits, sources, fragments, syntax, exclusions, and backlink order. Linux acceptance also checks static linkage. A successful cross-build does not verify execution on that target platform.

Archive members have fixed ordering, permissions, and ownership, with the source-commit timestamp. The gzip header omits the source filename and sets the timestamp to zero. Two independently written archives must have identical hashes. This checks reproducibility with the current compression toolchain; compare binary hashes across hosts separately.

The JSON records source/script/binary/archive hashes, source date, fixed Go settings, repeated-build/archive equality, and the actual acceptance host. It omits machine paths and inherited environment dumps. Output paths must not exist, even as symlinks. Archive and external checksum paths must be outside the prepared directory. On failure, only incomplete outputs created by that operation are cleaned up.

Read the trusted hash from the successful preparation report and verify the archive:

```sh
release_hash=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["archive"]["sha256"])' \
  "$release_parent/upload/verification-local.json")
python3 scripts/release.py --verify-archive "$release_parent/upload/$release_archive" \
  --expected-sha256 "$release_hash" > "$release_parent/verified-local.json"
```

Verification first checks the supplied trusted hash, then the complete archive member set and metadata before extraction. It verifies internal checksums and all four binary build settings, then executes host acceptance against the checkout's version, `HEAD`, and source date. It does not rebuild or access the network, and does not report repeated-build success.

`--output NEW_DIRECTORY` without `--archive` still prepares a checked directory. `--prepare-only` builds and validates binaries without documents or checksums; that output is incomplete and cannot be combined with archive creation. Verification requires `--expected-sha256` and cannot be combined with preparation flags.

## Select the destination and verify GitHub runs

For a real release, choose the destination explicitly and inspect it before any remote write. Replace the values below with the approved repository and a branch containing the reviewed source:

```sh
release_repo='OWNER/REPOSITORY'
release_ref='APPROVED_SOURCE_BRANCH'
release_remote="https://github.com/$release_repo.git"
gh repo view "$release_repo" --json nameWithOwner,url,visibility
git remote -v
git ls-remote "$release_remote" "refs/heads/$release_ref" "refs/tags/$release_version" "refs/tags/$release_version^{}"
gh release list --repo "$release_repo"
```

Confirm that the branch points to `$release_sha` and that neither the tag nor the release exists. An authentication or network error is not evidence of absence. Stop immediately if a conflicting tag or release already exists. Push the reviewed source only when both the destination repository and content are authorized; CI starts upon push. The preparation workflow must already exist on the repository's default branch for manual dispatch.

```sh
gh workflow run release.yml --repo "$release_repo" --ref "$release_ref"
gh run list --repo "$release_repo" --commit "$release_sha" \
  --json databaseId,workflowName,event,headSha,status,conclusion,url
```

Select the CI and release-preparation run IDs for the exact source SHA, not simply the latest runs. Wait for completion and require success for both; recheck `headSha` after selecting each ID:

```sh
ci_run='CI_RUN_ID'
prepare_run='PREPARATION_RUN_ID'
gh run watch "$ci_run" --repo "$release_repo" --exit-status
gh run watch "$prepare_run" --repo "$release_repo" --exit-status
gh run view "$ci_run" --repo "$release_repo" --json headSha,conclusion,url
gh run view "$prepare_run" --repo "$release_repo" --json headSha,conclusion,url

gh run download "$prepare_run" --repo "$release_repo" \
  --name "mdlink-$release_version-$release_sha" --dir "$release_parent/ci"
```

The workflow runs with `contents: read` permissions and uploads prepared assets; it does not publish. Compare the local and CI reports' version, source SHA, source date, script hash, four binary hashes, and checked settings. Require repeated-build/archive success in both preparation reports. Verify the CI archive and its external checksum against the CI report before accepting them as evidence. Local macOS arm64 acceptance and workflow Linux amd64 acceptance are distinct; record only hosts that actually ran acceptance tests. Keep both reports. If preparing locally on a different host, record its actual `accepted_host` in the evidence.

## Publish the approved assets

Before publication, present the intended repository, version, source SHA, archive hash, notes, and assets for authorization. Use the validated local archive below; its external checksum and local report must identify those exact bytes. Also attach the CI report as separate platform evidence. The CI archive hash may differ depending on the compression toolchain, but all four binary hashes must match.

Prepare `$release_parent/release-notes.md` from the current notes, replacing repository-relative links with absolute links to the chosen public repository and `$release_sha`. Review the notes and file list. Confirm once more that the source checkout is clean and that neither the remote tag nor the release exists before running:

```sh
test "$(git rev-parse HEAD)" = "$release_sha"
test -z "$(git status --porcelain --untracked-files=all)"
git tag -a "$release_version" "$release_sha" -m "mdlink $release_version"
git push "$release_remote" "refs/tags/$release_version"

gh release create "$release_version" --repo "$release_repo" --verify-tag \
  --title "mdlink $release_version" --notes-file "$release_parent/release-notes.md" \
  "$release_parent/upload/$release_archive" \
  "$release_parent/upload/$release_archive.sha256" \
  "$release_parent/upload/verification-local.json" \
  "$release_parent/ci/verification-linux-amd64.json"
```

Pushing the tag triggers the release workflow. Confirm that the run targets `$release_sha` and succeeds. `--verify-tag` prevents accidental tag creation from a different branch. Do not use force pushes, asset replacements, or tag moves to repair a failed publication. Inspect the existing state before deciding how to recover. See the official [workflow-run](https://cli.github.com/manual/gh_workflow_run), [run-list](https://cli.github.com/manual/gh_run_list), and [release-create](https://cli.github.com/manual/gh_release_create) options.

## Download and verify the published release

Use a fresh directory and keep the previously prepared report and hash as the trusted reference:

```sh
download_parent=$(mktemp -d)
gh release view "$release_version" --repo "$release_repo" \
  --json tagName,targetCommitish,isDraft,isPrerelease,assets,url > "$download_parent/release.json"
git ls-remote "$release_remote" "refs/tags/$release_version" "refs/tags/$release_version^{}"
gh release download "$release_version" --repo "$release_repo" --dir "$download_parent" \
  --pattern "$release_archive" --pattern "$release_archive.sha256" \
  --pattern 'verification-local.json' --pattern 'verification-linux-amd64.json'
```

Confirm that the annotated tag's peeled target (`^{}`) matches `$release_sha` (the tag object itself has a different SHA). Inspect the release status, repository, asset names, counts, sizes, and notes. `targetCommitish` alone does not prove the tag target. Compare the downloaded checksum file with the local checksum file (which contains the trusted hash and archive basename), and verify that the downloaded reports match the saved local and CI reports:

```sh
cmp "$release_parent/upload/$release_archive.sha256" "$download_parent/$release_archive.sha256"
cmp "$release_parent/upload/verification-local.json" "$download_parent/verification-local.json"
cmp "$release_parent/ci/verification-linux-amd64.json" "$download_parent/verification-linux-amd64.json"
test "$(git rev-parse HEAD)" = "$release_sha"
python3 scripts/release.py --verify-archive "$download_parent/$release_archive" \
  --expected-sha256 "$release_hash" > "$download_parent/verified-download.json"
```

Run verification from the matching clean source checkout. The trusted hash catches archive corruption before any extracted binary executes. Do not substitute a hash obtained only from the download for the saved preparation hash. See the official [release-download](https://cli.github.com/manual/gh_release_download) options.

Publication is complete only after fresh-download verification succeeds. Record the exact source SHA, remote tag target, run URLs and outcomes, downloaded hashes, and tested platforms. Keep these subsequent records separate from the release source commit. Cross-built darwin/amd64 and linux/arm64 binaries remain unexecuted unless those platforms were actually tested.
