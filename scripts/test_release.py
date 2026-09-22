import copy
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import tarfile
import unittest
from unittest.mock import patch

import release


class ReleaseTests(unittest.TestCase):
    def test_host_acceptance_checks_unresolved(self):
        expected = {
            "schema_version": 1, "command": "unresolved", "root": ".",
            "results": [{"target": "Missing", "count": 2, "sources": [
                {"path": "Check.md", "offset": offset} for offset in (0, 12)]}],
            "diagnostics": [{"code": "unresolved-link", "source": "Check.md",
                             "offset": offset, "raw_target": "Missing"} for offset in (0, 12)],
        }
        for fault in (None, "sources", "diagnostics", "strict", "stderr"):
            with self.subTest(fault=fault):
                calls = []

                def invoke(args, **kwargs):
                    command = args[1]
                    calls.append(args[1:])
                    code, stderr = 0, ""
                    if command == "version":
                        stdout = f"mdlink {release.VERSION}\ncommit: sha\nbuild date: date\ngo: go1.27.1\n"
                    elif "--help" in args:
                        stdout = f"Usage: mdlink {command}\n"
                    elif command == "unresolved":
                        root = Path(args[args.index("--root") + 1])
                        missing = (root / "Check.md").exists()
                        if missing:
                            self.assertEqual((root / "Check.md").read_text(), "[[Missing]] [[Missing#Heading|label]]\n")
                            response = copy.deepcopy(expected)
                            if fault == "sources":
                                response["results"][0]["sources"][1]["offset"] = 13
                            elif fault == "diagnostics":
                                response["diagnostics"].pop()
                            code = 3 if "--strict" in args and fault != "strict" else 0
                            stderr = "unexpected\n" if fault == "stderr" else ""
                        else:
                            response = expected | {"results": [], "diagnostics": []}
                        stdout = json.dumps(response) + "\n" if "--format" in args else ""
                    else:
                        target, result = ("Alpha.md", "Beta.md") if command == "outgoing" else ("Beta.md", "Alpha.md")
                        stdout = f"2\t{result}\n" if "--counts" in args else json.dumps({
                            "schema_version": 1, "command": command, "root": ".", "target": target,
                            "results": [{"path": result, "count": 2}], "diagnostics": []})
                    return subprocess.CompletedProcess(args, code, stdout, stderr)

                with patch("release.subprocess.run", side_effect=invoke), patch("release.platform.system", return_value="Darwin"), patch("release.accept_link_features"), patch("release.accept_gitignore"), patch("release.accept_mdlinkignore"), patch("release.accept_backlinks_order"):
                    if fault:
                        with self.assertRaises(ValueError):
                            release.accept_host(Path("/fixture/mdlink"), "sha", "date")
                    else:
                        release.accept_host(Path("/fixture/mdlink"), "sha", "date")
                        unresolved = [args for args in calls if args[0] == "unresolved"]
                        self.assertEqual(len(unresolved), 6)
                        self.assertIn(["unresolved", "--help"], unresolved)
                        self.assertEqual(sum("--counts" in args for args in unresolved), 1)
                        self.assertEqual(sum("--format" in args for args in unresolved), 3)
                        self.assertEqual(sum("--strict" in args for args in unresolved), 1)

    def test_release_tag_trigger_matches_fixed_version(self):
        workflow = Path(".github/workflows/release.yml").read_text()
        self.assertIn(f"  push:\n    tags: ['{release.VERSION}']\n", workflow)
        self.assertIn("  workflow_dispatch:", workflow)
        self.assertIn(f"name: mdlink-{release.VERSION}-", workflow)
        self.assertIn(f'--archive "$release_parent/upload/mdlink-{release.VERSION}.tar.gz"', workflow)

    def test_v100_release_contract(self):
        self.assertEqual(release.VERSION, "v1.0.0")
        self.assertEqual(release.DOCUMENTS, (
            "LICENSE", "README.md", "THIRD_PARTY_NOTICES.md", "docs/usage.md",
            "docs/limitations.md", "docs/json-schema-v1.md",
            "docs/release-notes/v1.0.0.md",
        ))

    def test_host_feature_acceptance_rejects_corruption(self):
        # The real host binary supplies valid responses; mutate each new contract
        # independently to prove acceptance does not just check successful exits.
        temporary = tempfile.TemporaryDirectory(prefix="mdlink-release-test-")
        self.addCleanup(temporary.cleanup)
        binary = str(Path(temporary.name) / "mdlink")
        subprocess.run([shutil.which("go"), "build", "-o", binary, "-ldflags",
                        f"-X mdlink/internal/app.Version={release.VERSION} -X mdlink/internal/app.Commit=sha -X mdlink/internal/app.BuildDate=date",
                        "./cmd/mdlink"], check=True)
        actual_run = subprocess.run
        for fault in (None, "sources", "phase", "empty", "fragment", "raw_link", "count", "strict", "syntax", "stderr", "gitignore-default", "gitignore-restore", "gitignore-manual", "backlinks-order", "backlinks-count", "backlinks-strict", "mdlinkignore-default", "mdlinkignore-no-gitignore", "mdlinkignore-strict"):
            with self.subTest(fault=fault):
                changed = []
                def invoke(args, **kwargs):
                    result = actual_run(args, **kwargs)
                    if "--format" not in args:
                        return result
                    response = json.loads(result.stdout)
                    command = args[1]
                    gitignore = (Path(args[args.index("--root") + 1]) / ".gitignore").exists()
                    if fault and fault.startswith("backlinks-") and command == "backlinks" and len(response.get("results", [])) == 6:
                        if fault == "backlinks-order":
                            response["diagnostics"].reverse()
                        elif fault == "backlinks-count":
                            response["results"][0]["count"] = 1
                        elif "--strict" in args:
                            changed.append(True)
                            return subprocess.CompletedProcess(args, 0, result.stdout, result.stderr)
                        else:
                            return result
                    elif (Path(args[args.index("--root") + 1]) / ".mdlinkignore").exists() and (
                        (fault == "mdlinkignore-default" and "--no-gitignore" not in args)
                        or (fault == "mdlinkignore-no-gitignore" and "--no-gitignore" in args)
                        or (fault == "mdlinkignore-strict" and "--strict" in args)
                    ):
                        if fault == "mdlinkignore-strict":
                            changed.append(True)
                            return subprocess.CompletedProcess(args, 0, result.stdout, result.stderr)
                        response["results"] = []
                    elif gitignore and ((fault == "gitignore-default" and "--no-gitignore" not in args)
                                      or (fault == "gitignore-restore" and "--no-gitignore" in args and "--exclude" not in args)
                                      or (fault == "gitignore-manual" and "--exclude" in args)):
                        response["results"] = ([{"target": "Keep", "count": 1, "sources": [{"path": "Source.md", "offset": 0}]}]
                                               if fault == "gitignore-default" else [])
                    elif fault == "sources" and response.get("sources") == ["Source.md"]:
                        response["sources"] = []
                    elif fault == "phase" and response.get("sources") == ["Source.md"]:
                        response["diagnostics"][0].pop("phase")
                    elif fault == "empty" and response.get("sources") == []:
                        response.pop("sources")
                    elif fault in ("fragment", "raw_link") and (response.get("diagnostics") or [{}])[0].get("phase") == "fragment":
                        response["diagnostics"][0]["code" if fault == "fragment" else "raw_link"] = "wrong"
                    elif fault == "count" and command == "outgoing" and args[-1] == "Frag":
                        response["results"][0]["count"] = 2
                    elif fault == "strict" and "--strict" in args and "--source" in args:
                        changed.append(True)
                        return subprocess.CompletedProcess(args, 0, result.stdout, result.stderr)
                    elif fault == "syntax" and command == "outgoing" and args[-1] == "Syntax":
                        response["results"][0]["count"] = 4
                    elif fault == "stderr" and "--source" in args:
                        changed.append(True)
                        return subprocess.CompletedProcess(args, result.returncode, result.stdout, "unexpected")
                    else:
                        return result
                    changed.append(True)
                    return subprocess.CompletedProcess(args, result.returncode, json.dumps(response), result.stderr)
                with patch("release.subprocess.run", side_effect=invoke):
                    if fault:
                        with self.assertRaises(ValueError):
                            release.accept_host(Path(binary), "sha", "date")
                        self.assertTrue(changed)
                    else:
                        release.accept_host(Path(binary), "sha", "date")

    def test_commit_date_is_utc(self):
        self.assertEqual(release.build_date(0), "1970-01-01T00:00:00Z")
        self.assertEqual(release.build_date(1788393600), "2026-09-03T00:00:00Z")

    def test_required_documents_and_license(self):
        with tempfile.TemporaryDirectory() as directory:
            snapshot = Path(directory)
            for name in release.DOCUMENTS:
                file = snapshot / name
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text("fixture document\n")
            release.check_documents(snapshot)
            for name in release.DOCUMENTS:
                path = snapshot / name
                for fault in ("missing", "empty", "symlink"):
                    with self.subTest(name=name, fault=fault):
                        path.unlink()
                        if fault == "empty":
                            path.write_text("")
                        elif fault == "symlink":
                            path.symlink_to(snapshot / "README.md")
                        with self.assertRaises(ValueError):
                            release.check_documents(snapshot)
                        if path.exists() or path.is_symlink():
                            path.unlink()
                        path.write_text("fixture document\n")
            self.assertNotIn("HANDOVER.md", release.DOCUMENTS)
            self.assertFalse(any("benchmarks/" in p for p in release.DOCUMENTS))

    def test_existing_output_is_preserved(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            file = parent / "file"
            file.write_text("keep")
            folder = parent / "folder"
            folder.mkdir()
            symlink = parent / "link"
            symlink.symlink_to(parent / "missing")
            for existing in (file, folder, symlink):
                with self.subTest(kind=existing.name), self.assertRaises(FileExistsError):
                    release.prepare(existing, prepare_only=True)
            self.assertEqual(file.read_text(), "keep")
            self.assertTrue(folder.is_dir())
            self.assertTrue(symlink.is_symlink())

    def test_checksums_reject_corruption_missing_and_extra(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            for name in release.ARTIFACT_FILES:
                file = output / name
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_bytes(b"fixture")
            release.write_checksums(output)
            release.verify_checksums(output)
            checksum = (output / "SHA256SUMS").read_text()
            self.assertEqual([line.split("  ", 1)[1] for line in checksum.splitlines()], sorted(release.ARTIFACT_FILES))
            selected = output / release.ARTIFACT_FILES[0]
            selected.write_bytes(b"corrupt")
            with self.assertRaises(ValueError):
                release.verify_checksums(output)
            selected.unlink()
            with self.assertRaises(ValueError):
                release.verify_checksums(output)
            selected.write_bytes(b"fixture")
            (output / "unexpected").write_text("not part of release")
            with self.assertRaises(ValueError):
                release.verify_checksums(output)

    def test_child_go_reads_fixed_environment(self):
        go = shutil.which("go")
        self.assertIsNotNone(go)
        inherited = os.environ | {
            "GOENV": "/missing-goenv", "GOWORK": "/missing-work", "GOROOT": "/missing-root",
            "GOTOOLCHAIN": "go1.0+auto", "GO111MODULE": "off", "GOFLAGS": "-invalid-flag",
            "GOAMD64": "v4", "GOARM64": "v9.5", "GOEXPERIMENT": "invalid-experiment",
            "GOFIPS140": "invalid-fips", "GOCACHEPROG": "/missing-cache", "GODEBUG": "invalid",
            "GO_EXTLINK_ENABLED": "1", "CGO_ENABLED": "1",
        }
        original = copy.deepcopy(inherited)
        for target in release.TARGETS:
            child = release.go_environment(inherited, target)
            actual = release.verify_go(str(Path(go).resolve()), child, Path.cwd())
            for key, value in release.FIXED_GO.items():
                self.assertEqual(actual[key], "" if key == "GOENV" else value)
            self.assertEqual(actual["GOOS"], target[0])
            self.assertEqual(actual["GOARCH"], target[1])
            self.assertEqual(actual["GOVERSION"], "go1.27.1")
            self.assertNotIn("GOROOT", actual)
            self.assertEqual(set(actual), set(release.FIXED_GO) | {"GOOS", "GOARCH", "GOVERSION"})
        self.assertEqual(inherited, original)

    def artifact_fixture(self, root):
        root.mkdir()
        for name in release.ARTIFACT_FILES:
            path = root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(name + "\n")
        release.write_checksums(root)
        return root

    def test_archive_is_reproducible_and_exact(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            first = self.artifact_fixture(parent / "first")
            second = self.artifact_fixture(parent / "second")
            for path in second.rglob("*"):
                if path.is_file():
                    path.chmod(0o700)
                    os.utime(path, (100, 100))
            archives = [parent / "first.tar.gz", parent / "second.tar.gz"]
            for source, archive in zip((first, second), archives):
                release.write_archive(source, archive, 123)
            self.assertEqual(archives[0].read_bytes(), archives[1].read_bytes())
            header = archives[0].read_bytes()[:10]
            self.assertEqual(header[3], 0)  # No original filename.
            self.assertEqual(header[4:8], b"\0" * 4)
            with tarfile.open(archives[0], "r:gz") as package:
                members = release.check_archive(package, 123)
                self.assertEqual([item.name for item in members], sorted((*release.ARTIFACT_FILES, "SHA256SUMS")))
                for item in members:
                    self.assertEqual(package.extractfile(item).read(), (first / item.name).read_bytes())
                    self.assertEqual((item.uid, item.gid, item.uname, item.gname, item.mtime), (0, 0, "", "", 123))
                    self.assertEqual(item.mode, 0o755 if item.name in release.BINARIES else 0o644)
            with self.assertRaises(FileExistsError):
                release.write_archive(first, archives[0], 123)

    def test_archive_rejects_unsafe_members_before_acceptance(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            source = self.artifact_fixture(parent / "artifact")
            archive = parent / "good.tar.gz"
            release.write_archive(source, archive, 123)
            with tarfile.open(archive, "r:gz") as package:
                good = [(member, package.extractfile(member).read()) for member in package.getmembers()]
            for fault in ("duplicate", "traversal", "absolute", "symlink", "hardlink", "missing", "extra", "mode", "owner", "mtime", "pax", "corrupt"):
                with self.subTest(fault=fault):
                    entries = copy.deepcopy(good)
                    member = entries[0][0]
                    if fault == "duplicate":
                        entries.append(copy.deepcopy(entries[0]))
                    elif fault in ("traversal", "absolute", "extra"):
                        member.name = {"traversal": "../outside", "absolute": "/outside", "extra": "extra"}[fault]
                    elif fault in ("symlink", "hardlink"):
                        member.type = tarfile.SYMTYPE if fault == "symlink" else tarfile.LNKTYPE
                        member.linkname = "outside"
                        member.size = 0
                    elif fault == "missing":
                        entries.pop()
                    elif fault == "mode":
                        member.mode = 0o777
                    elif fault == "owner":
                        member.uname = "private-owner"
                    elif fault == "mtime":
                        member.mtime = 124
                    elif fault == "pax":
                        member.pax_headers = {"comment": "private-path"}
                    else:
                        entries[0] = (member, b"x" * member.size)
                    bad = parent / "bad.tar.gz"
                    with tarfile.open(bad, "w:gz") as package:
                        for item, content in entries:
                            package.addfile(item, io.BytesIO(content))
                    with patch("release.source_identity", return_value=(Path.cwd(), "sha", "date", 123)), patch("release.accept_host") as accept:
                        with self.assertRaises(ValueError):
                            release.verify_archive(bad, release.sha256(bad))
                        accept.assert_not_called()
            with patch("release.source_identity", return_value=(Path.cwd(), "sha", "date", 123)), patch("release.tarfile.open") as opened:
                with self.assertRaises(ValueError):
                    release.verify_archive(archive, "0" * 64)
                opened.assert_not_called()

    def test_archive_output_protection_and_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            source = self.artifact_fixture(parent / "artifact")
            archive = parent / f"mdlink-{release.VERSION}.tar.gz"
            checksum = archive.with_name(archive.name + ".sha256")
            for occupied in (archive, checksum):
                for kind in ("file", "directory", "symlink"):
                    with self.subTest(occupied=occupied.name, kind=kind):
                        if kind == "file":
                            occupied.write_text("keep")
                        elif kind == "directory":
                            occupied.mkdir()
                        else:
                            occupied.symlink_to(parent / "missing")
                        with self.assertRaises(FileExistsError):
                            release.create_archive(source, archive, 123)
                        self.assertTrue(occupied.exists() or occupied.is_symlink())
                        if kind == "directory":
                            occupied.rmdir()
                        else:
                            occupied.unlink()
            with patch("release.write_archive", side_effect=OSError("failed")):
                with self.assertRaises(OSError):
                    release.create_archive(source, archive, 123)
            self.assertFalse(archive.exists())
            self.assertFalse(checksum.exists())
            with self.assertRaises(ValueError):
                release.create_archive(source, source / archive.name, 123)
            result = release.create_archive(source, archive, 123)
            self.assertEqual(result, {"name": archive.name, "sha256": release.sha256(archive), "rebuild_equal": True})
            self.assertEqual(checksum.read_text(), f"{result['sha256']}  {archive.name}\n")

    def test_verification_report_contains_only_performed_checks(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            source = self.artifact_fixture(parent / "artifact")
            archive = parent / f"mdlink-{release.VERSION}.tar.gz"
            release.write_archive(source, archive, 123)
            private = {"GOROOT": "/private-owner/go", "PWD": "/private-owner/checkout", "TMPDIR": "/private-owner/tmp"}
            with patch.dict(os.environ, private), patch("release.source_identity", return_value=(Path.cwd(), "sha", "date", 123)), patch("release.verify_build", return_value={}) as build, patch("release.host_target", return_value=("darwin", "arm64")), patch("release.accept_host") as accept:
                report = release.verify_archive(archive, release.sha256(archive))
            self.assertEqual(report["mode"], "verify-archive")
            self.assertEqual(report["source_commit"], "sha")
            self.assertEqual(report["build_date"], "date")
            self.assertEqual(report["accepted_host"], "darwin/arm64")
            self.assertTrue(report["checksums_complete"])
            self.assertEqual(report["script_sha256"], release.sha256(Path(release.__file__)))
            self.assertEqual(report["archive"]["sha256"], release.sha256(archive))
            self.assertEqual(set(report["targets"]), {"/".join(target) for target in release.TARGETS})
            self.assertNotIn("rebuild_equal", json.dumps(report))
            self.assertNotIn("private-owner", json.dumps(report))
            self.assertNotIn("GOROOT", json.dumps(report))
            self.assertEqual(build.call_count, 4)
            accept.assert_called_once()

    def test_truncated_gzip_is_rejected_even_with_matching_hash(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            source = self.artifact_fixture(parent / "artifact")
            archive = parent / f"mdlink-{release.VERSION}.tar.gz"
            release.write_archive(source, archive, 123)
            archive.write_bytes(archive.read_bytes()[:-8])
            with patch("release.source_identity", return_value=(Path.cwd(), "sha", "date", 123)), patch("release.verify_build"), patch("release.accept_host") as accept:
                with self.assertRaises((EOFError, tarfile.TarError)):
                    release.verify_archive(archive, release.sha256(archive))
                accept.assert_not_called()

    def test_archive_partial_writes_and_repeat_failure_are_cleaned(self):
        with tempfile.TemporaryDirectory() as directory:
            parent = Path(directory)
            source = self.artifact_fixture(parent / "artifact")
            archive = parent / f"mdlink-{release.VERSION}.tar.gz"
            with patch("release.tarfile.open", side_effect=OSError("partial archive failure")):
                with self.assertRaises(OSError):
                    release.write_archive(source, archive, 123)
            self.assertFalse(archive.exists())
            original = release.write_archive
            calls = []
            def fail_repeat(*args):
                calls.append(True)
                if len(calls) == 2:
                    raise OSError("repeat failure")
                return original(*args)
            with patch("release.write_archive", side_effect=fail_repeat):
                with self.assertRaises(OSError):
                    release.create_archive(source, archive, 123)
            self.assertFalse(archive.exists())
            self.assertFalse(archive.with_name(archive.name + ".sha256").exists())

    def test_archive_cli_rejects_invalid_combinations(self):
        invalid = (
            ["--verify-archive", "file"],
            ["--verify-archive", "file", "--expected-sha256", "invalid"],
            ["--verify-archive", "file", "--expected-sha256", "0" * 64, "--output", "out"],
            ["--verify-archive", "file", "--expected-sha256", "0" * 64, "--prepare-only"],
            ["--verify-archive", "file", "--expected-sha256", "0" * 64, "--archive", "out"],
            ["--output", "out", "--archive", "file", "--prepare-only"],
            ["--output", "out", "--expected-sha256", "0" * 64],
        )
        for arguments in invalid:
            with self.subTest(arguments=arguments):
                result = subprocess.run(["python3", "scripts/release.py", *arguments], capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")

    def test_subprocess_failure_stops(self):
        with self.assertRaises(subprocess.CalledProcessError):
            release.run(["git", "--invalid-release-fixture-option"])


if __name__ == "__main__":
    unittest.main()
