"""Prepare the fixed v1.0.0 release from committed source; never publish."""

import argparse
from datetime import datetime, timezone
import gzip
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile

VERSION = "v1.0.0"
TARGETS = (("darwin", "arm64"), ("darwin", "amd64"), ("linux", "amd64"), ("linux", "arm64"))
BINARIES = tuple(f"mdlink-{VERSION}-{system}-{arch}" for system, arch in TARGETS)
DOCUMENTS = (
    "LICENSE", "README.md", "THIRD_PARTY_NOTICES.md", "docs/usage.md",
    "docs/limitations.md", "docs/json-schema-v1.md",
    "docs/release-notes/v1.0.0.md",
)
ARTIFACT_FILES = BINARIES + DOCUMENTS
FIXED_GO = {
    "GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local", "GO111MODULE": "on",
    "GOFLAGS": "", "CGO_ENABLED": "0", "GOAMD64": "v1", "GOARM64": "v8.0",
    "GOEXPERIMENT": "", "GOFIPS140": "off",
}
REMOVE_GO = ("GOROOT", "GOCACHEPROG", "GODEBUG", "GO_EXTLINK_ENABLED")


def run(args, **kwargs):
    return subprocess.run(args, check=True, capture_output=True, text=True, **kwargs).stdout


def sha256(path):
    with path.open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def build_date(epoch):
    return datetime.fromtimestamp(int(epoch), timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def go_environment(inherited, target):
    child = inherited.copy()
    for key in REMOVE_GO:
        child.pop(key, None)
    child.update(FIXED_GO, GOOS=target[0], GOARCH=target[1])
    return child


def verify_go(go, env, cwd):
    if run([go, "version"], env=env, cwd=cwd).split()[2] != "go1.27.1":
        raise ValueError("Go 1.27.1 required")
    keys = list(FIXED_GO) + ["GOOS", "GOARCH", "GOVERSION"]
    actual = json.loads(run([go, "env", "-json", *keys], env=env, cwd=cwd))
    expected = FIXED_GO | {"GOENV": "", "GOOS": env["GOOS"], "GOARCH": env["GOARCH"], "GOVERSION": "go1.27.1"}
    if any(actual[key] != value for key, value in expected.items()):
        raise ValueError("unexpected Go environment")
    return actual


def check_documents(snapshot):
    if any((snapshot / name).is_symlink() or not (snapshot / name).is_file() or not (snapshot / name).read_bytes().strip() for name in DOCUMENTS):
        raise ValueError("required release document missing or empty")


def write_checksums(output):
    lines = [f"{sha256(output / name)}  {name}\n" for name in sorted(ARTIFACT_FILES)]
    (output / "SHA256SUMS").write_text("".join(lines))


def verify_checksums(output):
    paths = list(output.rglob("*"))
    if any(path.is_symlink() for path in paths):
        raise ValueError("unexpected release symlink")
    present = {str(path.relative_to(output)) for path in paths if not path.is_dir()}
    if present != set(ARTIFACT_FILES) | {"SHA256SUMS"}:
        raise ValueError("unexpected release file set")
    expected = "".join(f"{sha256(output / name)}  {name}\n" for name in sorted(ARTIFACT_FILES))
    if (output / "SHA256SUMS").read_text() != expected:
        raise ValueError("checksum mismatch")


def verify_build(go, binary, target, env, cwd):
    lines = run([go, "version", "-m", str(binary)], env=env, cwd=cwd).splitlines()
    if not lines or not lines[0].endswith(": go1.27.1"):
        raise ValueError("unexpected binary toolchain")
    settings = dict(line.strip().removeprefix("build\t").split("=", 1) for line in lines if line.strip().startswith("build\t") and "=" in line)
    expected = {"-trimpath": "true", "CGO_ENABLED": "0", "GOOS": target[0], "GOARCH": target[1], "GOAMD64" if target[1] == "amd64" else "GOARM64": "v1" if target[1] == "amd64" else "v8.0"}
    if any(settings.get(key) != value for key, value in expected.items()):
        raise ValueError("unexpected binary build settings")
    if any(key.startswith("vcs") for key in settings):
        raise ValueError("unexpected VCS build settings")
    return expected


def accept_host(binary, commit, date):
    def invoke(*args, expected_code=0):
        child = os.environ.copy()
        child.pop("MDLINK_ROOT", None)
        child.pop("MDLINK_FOLLOW_SYMLINKS", None)
        completed = subprocess.run([str(binary), *args], env=child, capture_output=True, text=True, check=False)
        if completed.returncode != expected_code:
            raise ValueError("unexpected binary exit code")
        if completed.stderr:
            raise ValueError("unexpected binary stderr")
        return completed.stdout

    if invoke("version") != f"mdlink {VERSION}\ncommit: {commit}\nbuild date: {date}\ngo: go1.27.1\n":
        raise ValueError("unexpected binary version")
    with tempfile.TemporaryDirectory(prefix="mdlink-release-fixture-") as directory:
        root = Path(directory)
        (root / "Alpha.md").write_text("# Alpha\n\n[[Beta]]\n[Beta](Beta.md)\n")
        (root / "Beta.md").write_text("# Beta\n")
        for command, target, result in (("outgoing", "Alpha", "Beta.md"), ("backlinks", "Beta", "Alpha.md")):
            if command not in invoke(command, "--help"):
                raise ValueError("unexpected binary help")
            if invoke(command, "--root", directory, "--counts", target) != f"2\t{result}\n":
                raise ValueError("unexpected binary counts")
            expected = {"schema_version": 1, "command": command, "root": ".", "target": target + ".md", "results": [{"path": result, "count": 2}], "diagnostics": []}
            if json.loads(invoke(command, "--root", directory, "--format", "json", target)) != expected:
                raise ValueError("unexpected binary JSON")
        if "unresolved" not in invoke("unresolved", "--help"):
            raise ValueError("unexpected binary help")
        for options in ((), ("--counts",)):
            if invoke("unresolved", "--root", directory, *options) != "":
                raise ValueError("unexpected empty unresolved output")
        expected = {"schema_version": 1, "command": "unresolved", "root": ".", "results": [], "diagnostics": []}
        if json.loads(invoke("unresolved", "--root", directory, "--format", "json")) != expected:
            raise ValueError("unexpected empty unresolved JSON")
        (root / "Check.md").write_text("[[Missing]] [[Missing#Heading|label]]\n")
        expected["results"] = [{"target": "Missing", "count": 2, "sources": [
            {"path": "Check.md", "offset": offset} for offset in (0, 12)]}]
        expected["diagnostics"] = [{"code": "unresolved-link", "source": "Check.md",
                                    "offset": offset, "raw_target": "Missing"} for offset in (0, 12)]
        for options, code in (((), 0), (("--strict",), 3)):
            if json.loads(invoke("unresolved", "--root", directory, "--format", "json", *options, expected_code=code)) != expected:
                raise ValueError("unexpected unresolved JSON")
    accept_link_features(invoke)
    accept_gitignore(invoke)
    accept_mdlinkignore(invoke)
    accept_backlinks_order(invoke)
    if platform.system() == "Linux":
        if "statically linked" not in run(["file", str(binary)]):
            raise ValueError("Linux binary is not static")
        if "INTERP" in run(["readelf", "-l", str(binary)]) or "NEEDED" in run(["readelf", "-d", str(binary)]):
            raise ValueError("Linux binary has dynamic dependencies")


def accept_backlinks_order(invoke):
    with tempfile.TemporaryDirectory(prefix="mdlink-release-backlinks-") as directory:
        root = Path(directory)
        (root / "Target.md").write_text("")
        source = "[[Target]] [[Target#Heading]] [[MissingA]] [[MissingB]]\n"
        paths = [f"Source{i}.md" for i in range(6)]
        for path in paths:
            (root / path).write_text(source)
        expected = {"schema_version": 1, "command": "backlinks", "root": ".", "target": "Target.md",
            "results": [{"path": path, "count": 2} for path in paths],
            "diagnostics": [{"code": "unresolved-link", "source": path,
                             "offset": source.index("[[" + missing + "]]"), "raw_target": missing}
                            for path in paths for missing in ("MissingA", "MissingB")]}
        for options, code in (((), 0), (("--strict",), 3)):
            actual = json.loads(invoke("backlinks", "--root", directory, "--format", "json", *options,
                                       "Target", expected_code=code))
            if actual != expected:
                raise ValueError("unexpected multi-source backlinks JSON")


def accept_mdlinkignore(invoke):
    with tempfile.TemporaryDirectory(prefix="mdlink-release-mdlinkignore-") as directory:
        root = Path(directory)
        (root / ".mdlinkignore").write_text("Hidden.md\narchive/\n")
        (root / "Source.md").write_text("[[Hidden]] [[archive/Noise]]\n")
        (root / "Hidden.md").write_text("[[HiddenMissing]]\n")
        (root / "archive").mkdir()
        (root / "archive" / "Noise.md").write_text("[[NoiseMissing]]\n")
        missing = [("Hidden", 0), ("archive/Noise", 11)]
        expected = {"schema_version": 1, "command": "unresolved", "root": ".",
            "results": [{"target": target, "count": 1, "sources": [{"path": "Source.md", "offset": offset}]}
                        for target, offset in missing],
            "diagnostics": [{"code": "unresolved-link", "source": "Source.md", "offset": offset,
                             "raw_target": target} for target, offset in missing]}
        for options in ((), ("--no-gitignore",)):
            for strict, code in (((), 0), (("--strict",), 3)):
                actual = json.loads(invoke("unresolved", "--root", directory, "--format", "json",
                                           *options, *strict, expected_code=code))
                if actual != expected:
                    raise ValueError("unexpected mdlinkignore acceptance JSON")


def accept_gitignore(invoke):
    with tempfile.TemporaryDirectory(prefix="mdlink-release-gitignore-") as directory:
        root = Path(directory)
        source = "[[Keep]] [[Hidden]] [[Manual]] [[nested/Restored]]\n"
        files = {
            ".gitignore": "*.md\n!Source.md\n!Keep.md\nnode_modules/\n",
            "Source.md": source, "Keep.md": "", "Hidden.md": "", "Manual.md": "",
            "nested/.gitignore": "!Restored.md\n", "nested/Restored.md": "",
            "node_modules/Noise.md": "[[NoiseMissing]]\n",
        }
        for name, body in files.items():
            filename = root / name
            filename.parent.mkdir(parents=True, exist_ok=True)
            filename.write_text(body)
        cases = (
            ((), [("Hidden", "Source.md", source.index("[[Hidden]]")),
                  ("Manual", "Source.md", source.index("[[Manual]]"))]),
            (("--no-gitignore",), [("NoiseMissing", "node_modules/Noise.md", 0)]),
            (("--no-gitignore", "--exclude", "Manual.md", "--exclude", "node_modules"),
             [("Manual", "Source.md", source.index("[[Manual]]"))]),
        )
        for options, missing in cases:
            expected = {"schema_version": 1, "command": "unresolved", "root": ".",
                "results": [{"target": target, "count": 1, "sources": [{"path": path, "offset": offset}]}
                            for target, path, offset in missing],
                "diagnostics": [{"code": "unresolved-link", "source": path, "offset": offset,
                                 "raw_target": target} for target, path, offset in missing]}
            actual = json.loads(invoke("unresolved", "--root", directory, "--format", "json", *options))
            if actual != expected:
                raise ValueError("unexpected gitignore acceptance JSON")


def accept_link_features(invoke):
    with tempfile.TemporaryDirectory(prefix="mdlink-release-links-") as directory:
        root = Path(directory)
        (root / "Source.md").write_text("[[Target]] [[ScopedMissing]]\n")
        (root / "Target.md").write_text("# Alpha\n\nText. ^item\n")
        (root / "Noise.md").write_text("[[NoiseMissing]]\n")
        base = {"schema_version": 1, "command": "unresolved", "root": "."}
        expected = base | {"sources": ["Source.md"], "results": [{"target": "ScopedMissing", "count": 1,
            "sources": [{"path": "Source.md", "offset": 11}]}], "diagnostics": [{"code": "unresolved-link",
            "source": "Source.md", "offset": 11, "raw_target": "ScopedMissing", "phase": "source"}]}
        sources = root / "sources0"
        sources.write_bytes(b"Source.md\0Source.md\0")
        for selection in (("--source", "Source.md"), ("--sources0-from", str(sources))):
            for strict, code in (((), 0), (("--strict",), 3)):
                if json.loads(invoke("unresolved", "--root", directory, "--format", "json", *selection,
                                     *strict, expected_code=code)) != expected:
                    raise ValueError("unexpected selected-source JSON")
        sources.write_bytes(b"")
        empty = base | {"sources": [], "results": [], "diagnostics": []}
        if json.loads(invoke("unresolved", "--root", directory, "--format", "json", "--sources0-from",
                             str(sources), "--strict")) != empty:
            raise ValueError("unexpected empty-source JSON")

        (root / "Frag.md").write_text("[[Target#Alpha]] [[Target#Absent|label]] [[Target#^item]]\n")
        outgoing = {"schema_version": 1, "command": "outgoing", "root": ".", "target": "Frag.md",
                    "results": [{"path": "Target.md", "count": 3}], "diagnostics": []}
        if json.loads(invoke("outgoing", "--root", directory, "--format", "json", "Frag")) != outgoing:
            raise ValueError("unexpected fragment-off JSON")
        fragment = {"code": "missing-fragment", "phase": "fragment", "source": "Frag.md", "offset": 17,
                    "raw_target": "Target", "raw_link": "[[Target#Absent|label]]", "fragment": "#Absent",
                    "target": "Target.md"}
        outgoing["diagnostics"] = [fragment]
        if json.loads(invoke("outgoing", "--root", directory, "--format", "json", "--check-fragments",
                             "--strict", "Frag", expected_code=3)) != outgoing:
            raise ValueError("unexpected fragment-on JSON")
        if json.loads(invoke("unresolved", "--root", directory, "--format", "json", "--source", "Frag.md",
                             "--check-fragments", "--strict", expected_code=3)) != base | {
                                 "sources": ["Frag.md"], "results": [], "diagnostics": [fragment]}:
            raise ValueError("unexpected unresolved fragment JSON")

    with tempfile.TemporaryDirectory(prefix="mdlink-release-syntax-") as directory:
        root = Path(directory)
        (root / "Target.md").write_text("# Target\n")
        (root / "Syntax.md").write_text(
            '[one](Target.md "title") ![two](<Target.md> \'title\')\n'
            '[three][id] ![four][] [five]\n[id]: Target.md\n[four]: Target.md\n[five]: Target.md\n')
        for command, target, result in (("outgoing", "Syntax", "Target.md"), ("backlinks", "Target", "Syntax.md")):
            expected = {"schema_version": 1, "command": command, "root": ".", "target": target + ".md",
                        "results": [{"path": result, "count": 5}], "diagnostics": []}
            if json.loads(invoke(command, "--root", directory, "--format", "json", target)) != expected:
                raise ValueError("unexpected title/reference JSON")


def source_identity():
    repo = Path.cwd()
    if Path(run(["git", "rev-parse", "--show-toplevel"]).strip()) != repo:
        raise ValueError("run from repository root")
    commit = run(["git", "rev-parse", "HEAD"]).strip()
    epoch = int(run(["git", "show", "-s", "--format=%ct", commit]).strip())
    return repo, commit, build_date(epoch), epoch


def host_target():
    arch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64"}.get(platform.machine())
    host = (platform.system().lower(), arch)
    if host not in TARGETS:
        raise ValueError("unsupported validation host")
    return host


def write_archive(output, archive, epoch):
    with archive.open("xb") as destination:
        try:
            with gzip.GzipFile(filename="", mode="wb", fileobj=destination, mtime=0) as compressed:
                with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as package:
                    for name in sorted((*ARTIFACT_FILES, "SHA256SUMS")):
                        path = output / name
                        member = tarfile.TarInfo(name)
                        member.size = path.stat().st_size
                        member.mode = 0o755 if name in BINARIES else 0o644
                        member.mtime = epoch
                        with path.open("rb") as content:
                            package.addfile(member, content)
        except BaseException:
            archive.unlink()
            raise


def create_archive(output, archive, epoch):
    archive = Path(os.path.abspath(archive))
    checksum = archive.with_name(archive.name + ".sha256")
    if archive.name != f"mdlink-{VERSION}.tar.gz" or archive.resolve().is_relative_to(output.resolve()):
        raise ValueError("archive must have the release filename outside the prepared directory")
    for path in (archive, checksum):
        if path.exists() or path.is_symlink():
            raise FileExistsError(path)
    created = []
    try:
        verify_checksums(output)
        write_archive(output, archive, epoch)
        created.append(archive)
        digest = sha256(archive)
        with tempfile.TemporaryDirectory(prefix="mdlink-release-archive-") as directory:
            repeated = Path(directory) / archive.name
            write_archive(output, repeated, epoch)
            if sha256(repeated) != digest:
                raise ValueError("repeated archive checksum differs")
        with checksum.open("x") as destination:
            created.append(checksum)
            destination.write(f"{digest}  {archive.name}\n")
        return {"name": archive.name, "sha256": digest, "rebuild_equal": True}
    except BaseException:
        for path in reversed(created):
            path.unlink()
        raise


def check_archive(package, epoch):
    members = package.getmembers()
    names = [member.name for member in members]
    if len(names) != len(set(names)) or set(names) != set(ARTIFACT_FILES) | {"SHA256SUMS"}:
        raise ValueError("unexpected archive file set")
    for member in members:
        mode = 0o755 if member.name in BINARIES else 0o644
        if (member.type != tarfile.REGTYPE or member.mode != mode or member.uid != 0 or member.gid != 0
                or member.uname or member.gname or member.mtime != epoch
                or member.linkname or member.pax_headers):
            raise ValueError("unexpected archive metadata")
    return members


def verify_archive(archive, expected_sha256):
    if not re.fullmatch(r"[0-9a-f]{64}", expected_sha256):
        raise ValueError("expected SHA-256 must be 64 lowercase hexadecimal characters")
    repo, commit, date, epoch = source_identity()
    # Keep the trusted-hash check and extraction on the same open file.
    with archive.open("rb") as source:
        if hashlib.file_digest(source, "sha256").hexdigest() != expected_sha256:
            raise ValueError("archive checksum mismatch")
        source.seek(0)
        with tarfile.open(fileobj=source, mode="r:gz") as package:
            members = check_archive(package, epoch)
            # Tar end markers can precede the gzip trailer; validate its CRC/length too.
            while package.fileobj.read(1024 * 1024):
                pass
            with tempfile.TemporaryDirectory(prefix="mdlink-release-verify-") as directory:
                output = Path(directory)
                for member in members:
                    destination = output / member.name
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    with package.extractfile(member) as content, destination.open("xb") as target:
                        shutil.copyfileobj(content, target)
                    destination.chmod(member.mode)
                verify_checksums(output)
                check_documents(output)
                go = shutil.which("go")
                if go is None:
                    raise ValueError("Go not found")
                go = str(Path(go).resolve())
                report = {"mode": "verify-archive", "version": VERSION, "source_commit": commit,
                          "build_date": date, "script_sha256": sha256(Path(__file__)), "targets": {}}
                for target, name in zip(TARGETS, BINARIES):
                    env = go_environment(os.environ, target)
                    actual = verify_go(go, env, repo)
                    settings = verify_build(go, output / name, target, env, repo)
                    report["targets"]["/".join(target)] = {"binary": name, "sha256": sha256(output / name),
                        "go_environment": actual, "build_settings": settings}
                host = host_target()
                accept_host(output / BINARIES[TARGETS.index(host)], commit, date)
                report.update(accepted_host="/".join(host), checksums_complete=True,
                              archive={"name": archive.name, "sha256": expected_sha256})
                return report


def prepare(output, prepare_only=False, archive=None):
    if prepare_only and archive is not None:
        raise ValueError("prepare-only cannot create an archive")
    output = Path(os.path.abspath(output))
    output.mkdir()  # Exclusive creation: existing files, directories and symlinks fail.
    try:
        repo, commit, date, epoch = source_identity()
        go = shutil.which("go")
        if go is None:
            raise ValueError("Go not found")
        go = str(Path(go).resolve())
        report = {"mode": "prepare-only" if prepare_only else "artifact", "version": VERSION, "source_commit": commit, "build_date": date, "script_sha256": sha256(Path(__file__)), "targets": {}}
        with tempfile.TemporaryDirectory(prefix="mdlink-release-build-") as scratch:
            scratch = Path(scratch)
            snapshot = scratch / "source"
            snapshot.mkdir()
            source_archive = scratch / "source.tar"
            run(["git", "archive", "--format=tar", "--output", str(source_archive), commit])
            run(["tar", "-xf", str(source_archive), "-C", str(snapshot)])
            source_archive.unlink()
            if not prepare_only:
                check_documents(snapshot)
            repeated = scratch / "repeat"
            repeated.mkdir()
            ldflags = f"-X mdlink/internal/app.Version={VERSION} -X mdlink/internal/app.Commit={commit} -X mdlink/internal/app.BuildDate={date}"
            for target, name in zip(TARGETS, BINARIES):
                env = go_environment(os.environ, target)
                actual = verify_go(go, env, snapshot)
                arguments = [go, "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags]
                for destination in (output, repeated):
                    run([*arguments, "-o", str(destination / name), "./cmd/mdlink"], cwd=snapshot, env=env)
                digest = sha256(output / name)
                if digest != sha256(repeated / name):
                    raise ValueError("rebuild checksum differs")
                settings = verify_build(go, output / name, target, env, snapshot)
                report["targets"]["/".join(target)] = {"binary": name, "sha256": digest, "rebuild_equal": True, "go_environment": actual, "build_settings": settings}
            host = host_target()
            accept_host(output / BINARIES[TARGETS.index(host)], commit, date)
            report["accepted_host"] = "/".join(host)
            if not prepare_only:
                for name in DOCUMENTS:
                    destination = output / name
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(snapshot / name, destination)
                write_checksums(output)
                verify_checksums(output)
            report["checksums_complete"] = not prepare_only
            if archive is not None:
                report["archive"] = create_archive(output, archive, epoch)
            return report
    except BaseException:
        shutil.rmtree(output)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--output", type=Path, help="new child of an owned temporary directory")
    mode.add_argument("--verify-archive", type=Path, help="verify a downloaded archive against committed HEAD")
    parser.add_argument("--prepare-only", action="store_true", help="build and validate without completing distributable documents/checksums")
    parser.add_argument("--archive", type=Path, help=f"new path outside output named mdlink-{VERSION}.tar.gz")
    parser.add_argument("--expected-sha256", help="trusted prepared-archive SHA-256 for verification")
    args = parser.parse_args()
    if args.verify_archive:
        if args.prepare_only or args.archive or not args.expected_sha256 or not re.fullmatch(r"[0-9a-f]{64}", args.expected_sha256):
            parser.error("archive verification requires a valid --expected-sha256 and no preparation flags")
    elif args.expected_sha256 or (args.prepare_only and args.archive):
        parser.error("incompatible preparation flags")
    try:
        result = (verify_archive(args.verify_archive, args.expected_sha256) if args.verify_archive
                  else prepare(args.output, args.prepare_only, args.archive))
        print(json.dumps(result, indent=2))
    except (OSError, ValueError, EOFError, tarfile.TarError, subprocess.CalledProcessError):
        print("release preparation or verification failed", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
