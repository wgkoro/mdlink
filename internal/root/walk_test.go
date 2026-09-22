package root

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWalkEmpty(t *testing.T) {
	files, _, err := Walk(t.TempDir(), Options{})
	if err != nil || len(files) != 0 {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func writeFile(t *testing.T, directory, name string) {
	t.Helper()
	physical := filepath.Join(directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(physical), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(physical, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkRegularFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"nested/child.md", "plain.txt"} {
		writeFile(t, directory, name)
	}
	files, _, err := Walk(directory, Options{})
	want := []File{{LogicalPath: "nested/child.md", PhysicalPath: filepath.Join(directory, "nested", "child.md")}, {LogicalPath: "plain.txt", PhysicalPath: filepath.Join(directory, "plain.txt")}}
	var paths []File
	for _, file := range files {
		paths = append(paths, File{LogicalPath: file.LogicalPath, PhysicalPath: file.PhysicalPath})
	}
	if err != nil || !reflect.DeepEqual(paths, want) {
		t.Fatalf("Walk() = %v, %v; want %v", files, err, want)
	}
}

func TestWalkJapaneseAndSpaces(t *testing.T) {
	directory := t.TempDir()
	name := "日本語/メモ name.md"
	writeFile(t, directory, name)
	files, _, err := Walk(directory, Options{})
	if err != nil || len(files) != 1 || files[0].LogicalPath != name {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func TestWalkExcludesObsidian(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, ".obsidian/ignored.md")
	files, _, err := Walk(directory, Options{})
	if err != nil || len(files) != 0 {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func TestWalkExcludesGit(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, ".git/ignored.md")
	files, _, err := Walk(directory, Options{})
	if err != nil || len(files) != 0 {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func TestWalkExcludesTrash(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, ".trash/ignored.md")
	files, _, err := Walk(directory, Options{})
	if err != nil || len(files) != 0 {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func TestWalkExclusionScope(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"nested/.obsidian/ignored.md", "nested/.git/ignored.md", "nested/.trash/ignored.md", "node_modules/kept.md", ".obsidian-old/kept.md", "file/.git"} {
		writeFile(t, directory, name)
	}
	files, _, err := Walk(directory, Options{})
	var names []string
	for _, file := range files {
		names = append(names, file.LogicalPath)
	}
	want := []string{".obsidian-old/kept.md", "file/.git", "node_modules/kept.md"}
	if err != nil || !reflect.DeepEqual(names, want) {
		t.Fatalf("Walk names = %v, %v", names, err)
	}
}

func TestWalkSkipsSymlinks(t *testing.T) {
	directory := t.TempDir()
	external := t.TempDir()
	writeFile(t, external, "outside.md")
	for name, target := range map[string]string{"directory-link": external, "file-link": filepath.Join(external, "outside.md")} {
		if err := os.Symlink(target, filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
	files, _, err := Walk(directory, Options{})
	if err != nil || len(files) != 0 {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func TestWalkInvalidRoot(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, "file.md")
	for _, root := range []string{filepath.Join(directory, "missing"), filepath.Join(directory, "file.md")} {
		if _, _, err := Walk(root, Options{}); err == nil {
			t.Errorf("Walk(%q) must reject invalid root", root)
		}
	}
}

func TestWalkOrderIndependentOfCreation(t *testing.T) {
	var previous []string
	for _, order := range [][]string{{"z.md", "a/child.md", "a.md"}, {"a.md", "a/child.md", "z.md"}} {
		directory := t.TempDir()
		for _, name := range order {
			writeFile(t, directory, name)
		}
		files, _, err := Walk(directory, Options{})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, file := range files {
			names = append(names, file.LogicalPath)
		}
		if previous != nil && !reflect.DeepEqual(names, previous) {
			t.Fatalf("order changed: %v != %v", names, previous)
		}
		previous = names
	}
}

func TestLogicalPath(t *testing.T) {
	for _, input := range []string{"", "/outside", "../outside", "folder/../../outside", "."} {
		if value, err := CleanLogicalPath(input); err == nil {
			t.Errorf("CleanLogicalPath(%q) = %q, want error", input, value)
		}
	}
}

func TestLogicalPathCleansRelative(t *testing.T) {
	for input, want := range map[string]string{"./folder/note.md": "folder/note.md", "folder/../note.md": "note.md", "日本語/space name.md": "日本語/space name.md"} {
		got, err := CleanLogicalPath(input)
		if err != nil || got != want {
			t.Errorf("CleanLogicalPath(%q) = %q, %v", input, got, err)
		}
	}
}

func FuzzLogicalPath(f *testing.F) {
	for _, seed := range []string{"", "/outside", "../outside", "a/../../b", "./日本語/note.md", "a/b", "a\x00b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := CleanLogicalPath(input)
		if err != nil {
			return
		}
		if got == "" || got == "." || got == ".." || path.IsAbs(got) || strings.HasPrefix(got, "../") {
			t.Fatalf("unsafe logical path: %q -> %q", input, got)
		}
		if !withinPath(got, got) || !withinPath(got+"/child", got) || withinPath(got+"-old", got) {
			t.Fatalf("component boundary violated for %q", got)
		}
		again, err := CleanLogicalPath(got)
		if err != nil || again != got {
			t.Fatalf("not idempotent: %q -> %q, %v", got, again, err)
		}
	})
}

func TestWalkRejectsSymlinkRoot(t *testing.T) {
	directory := t.TempDir()
	external := t.TempDir()
	writeFile(t, external, "note.md")
	link := filepath.Join(directory, "linked-root")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Walk(link, Options{}); err == nil {
		t.Fatal("symlink root must not return empty success")
	}
}

func TestWalkExplicitExcludeBoundary(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"tmp/a.md", "tmp-old/b.md", "nested/tmp/c.md"} {
		writeFile(t, directory, name)
	}
	files, _, err := Walk(directory, Options{Exclude: []string{"tmp"}})
	var names []string
	for _, file := range files {
		names = append(names, file.LogicalPath)
	}
	if err != nil || !reflect.DeepEqual(names, []string{"nested/tmp/c.md", "tmp-old/b.md"}) {
		t.Fatalf("Walk names = %v, %v", names, err)
	}
}

func TestWalkMultipleNormalizedExcludes(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"tmp/a.md", "a/tmp/b.md", "single.md", "kept.md"} {
		writeFile(t, directory, name)
	}
	files, _, err := Walk(directory, Options{Exclude: []string{"./tmp/", "a/tmp", "single.md"}})
	if err != nil || len(files) != 1 || files[0].LogicalPath != "kept.md" {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func TestWalkRejectsUnsafeExcludesBeforeWalking(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, excluded := range []string{"", ".", "./", "/absolute", "..", "../outside", "a/../../outside", "a\x00b"} {
		_, _, err := Walk(missing, Options{Exclude: []string{excluded}})
		if err == nil || !strings.Contains(err.Error(), "invalid exclude") {
			t.Errorf("exclude %q: error = %v", excluded, err)
		}
	}
}

func TestWalkExcludeOrderIndependent(t *testing.T) {
	for _, order := range [][]string{{"kept", "a"}, {"a", "kept"}} {
		directory := t.TempDir()
		for _, name := range order {
			writeFile(t, directory, name+"/note.md")
		}
		writeFile(t, directory, "remaining.md")
		for _, excludes := range [][]string{{"tmp/../kept", "a"}, {"a", "tmp/../kept"}} {
			files, _, err := Walk(directory, Options{Exclude: excludes})
			if err != nil || len(files) != 1 || files[0].LogicalPath != "remaining.md" {
				t.Fatalf("Walk() = %v, %v", files, err)
			}
		}
	}
}

func TestWalkReportsSkippedSymlinks(t *testing.T) {
	directory := t.TempDir()
	external := t.TempDir()
	writeFile(t, external, "outside.md")
	if err := os.Symlink(external, filepath.Join(directory, "docs")); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := Walk(directory, Options{})
	if err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "skipped-symlink" || diagnostics[0].Source != "docs" {
		t.Fatalf("diagnostics/error = %v/%v", diagnostics, err)
	}
}

func TestWalkFollowsExplicitExternalSymlink(t *testing.T) {
	directory := t.TempDir()
	external := t.TempDir()
	writeFile(t, external, "note.md")
	if err := os.Symlink(external, filepath.Join(directory, "docs")); err != nil {
		t.Fatal(err)
	}
	files, diagnostics, err := Walk(directory, Options{FollowSymlinks: []string{"docs"}})
	if err != nil || len(diagnostics) != 0 || len(files) != 1 || files[0].LogicalPath != "docs/note.md" {
		t.Fatalf("files/diagnostics/error = %v/%v/%v", files, diagnostics, err)
	}
}

func TestWalkDoesNotInheritFollowPermission(t *testing.T) {
	directory, external, deeper := t.TempDir(), t.TempDir(), t.TempDir()
	writeFile(t, deeper, "note.md")
	if err := os.Symlink(deeper, filepath.Join(external, "nested")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(directory, "docs")); err != nil {
		t.Fatal(err)
	}
	files, diagnostics, err := Walk(directory, Options{FollowSymlinks: []string{"docs"}})
	if err != nil || len(files) != 0 || len(diagnostics) != 1 || diagnostics[0].Source != "docs/nested" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
	files, diagnostics, err = Walk(directory, Options{FollowSymlinks: []string{"docs", "docs/nested"}})
	if err != nil || len(diagnostics) != 0 || len(files) != 1 || files[0].LogicalPath != "docs/nested/note.md" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
}

func TestWalkRejectsFollowWithoutParentPermission(t *testing.T) {
	directory, external := t.TempDir(), t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(external, "nested")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(directory, "docs")); err != nil {
		t.Fatal(err)
	}
	files, _, err := Walk(directory, Options{FollowSymlinks: []string{"docs/nested"}})
	if err == nil || len(files) != 0 {
		t.Fatalf("result = %v/%v", files, err)
	}
}

func TestWalkStopsAncestorCycle(t *testing.T) {
	if os.Getenv("MDLINK_TEST_CYCLE") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWalkStopsAncestorCycle$")
		cmd.Env = append(os.Environ(), "MDLINK_TEST_CYCLE=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cycle subprocess: %v, context=%v\n%s", err, ctx.Err(), output)
		}
		return
	}
	directory := t.TempDir()
	if err := os.Symlink(directory, filepath.Join(directory, "loop")); err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := Walk(directory, Options{FollowSymlinks: []string{"loop"}})
	if err == nil || len(diagnostics) != 1 || diagnostics[0].Code != "duplicate-physical-file" {
		t.Fatalf("cycle result = %v/%v", diagnostics, err)
	}
	for _, kind := range []string{"self", "mutual", "broken"} {
		directory := t.TempDir()
		target := "loop"
		if kind == "mutual" {
			target = "other"
			if err := os.Symlink("loop", filepath.Join(directory, "other")); err != nil {
				t.Fatal(err)
			}
		}
		if kind == "broken" {
			target = "missing"
		}
		if err := os.Symlink(target, filepath.Join(directory, "loop")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Walk(directory, Options{FollowSymlinks: []string{"loop"}}); err == nil || strings.Contains(err.Error(), directory) {
			t.Fatalf("%s: error = %v", kind, err)
		}
	}
}

func TestWalkRejectsDuplicatePhysicalFiles(t *testing.T) {
	for _, kind := range []string{"direct-and-symlink", "two-symlinks", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			directory, external := t.TempDir(), t.TempDir()
			writeFile(t, external, "note.md")
			options := Options{}
			switch kind {
			case "direct-and-symlink":
				writeFile(t, directory, "a.md")
				if err := os.Symlink(filepath.Join(directory, "a.md"), filepath.Join(directory, "b.md")); err != nil {
					t.Fatal(err)
				}
				options.FollowSymlinks = []string{"b.md"}
			case "two-symlinks":
				for _, name := range []string{"a.md", "b.md"} {
					if err := os.Symlink(filepath.Join(external, "note.md"), filepath.Join(directory, name)); err != nil {
						t.Fatal(err)
					}
				}
				options.FollowSymlinks = []string{"a.md", "b.md"}
			case "hardlink":
				writeFile(t, directory, "a.md")
				if err := os.Link(filepath.Join(directory, "a.md"), filepath.Join(directory, "b.md")); err != nil {
					t.Fatal(err)
				}
			}
			files, diagnostics, err := Walk(directory, options)
			if err == nil || len(files) != 0 || len(diagnostics) != 1 || diagnostics[0].Code != "duplicate-physical-file" {
				t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
			}
		})
	}
}

func TestWalkExcludePrecedesFollow(t *testing.T) {
	directory, external := t.TempDir(), t.TempDir()
	writeFile(t, external, "note.md")
	for _, name := range []string{"tmp", "tmp-old"} {
		if err := os.Symlink(external, filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
	files, diagnostics, err := Walk(directory, Options{Exclude: []string{"tmp"}, FollowSymlinks: []string{"tmp", "tmp-old"}})
	if err != nil || len(diagnostics) != 0 || len(files) != 1 || files[0].LogicalPath != "tmp-old/note.md" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
}

func TestWalkFileSymlinkPermission(t *testing.T) {
	directory, external := t.TempDir(), t.TempDir()
	writeFile(t, external, "note.md")
	if err := os.Symlink(filepath.Join(external, "note.md"), filepath.Join(directory, "file.md")); err != nil {
		t.Fatal(err)
	}
	files, diagnostics, err := Walk(directory, Options{})
	if err != nil || len(files) != 0 || len(diagnostics) != 1 || diagnostics[0].Code != "skipped-symlink" || diagnostics[0].Source != "file.md" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
	files, diagnostics, err = Walk(directory, Options{FollowSymlinks: []string{"./file.md"}})
	if err != nil || len(diagnostics) != 0 || len(files) != 1 || files[0].LogicalPath != "file.md" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
}

func TestWalkRejectsInvalidFollow(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, "plain/file.md")
	for _, value := range []string{"plain", "plain/file.md", "missing", "", ".", "..", "../outside", "/absolute"} {
		if _, _, err := Walk(directory, Options{FollowSymlinks: []string{value}}); err == nil {
			t.Errorf("follow %q must be rejected", value)
		}
	}
}

func TestWalkFollowIsExact(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"tmp", "tmp-old"} {
		external := t.TempDir()
		writeFile(t, external, "note.md")
		if err := os.Symlink(external, filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
	files, diagnostics, err := Walk(directory, Options{FollowSymlinks: []string{"tmp"}})
	if err != nil || len(files) != 1 || files[0].LogicalPath != "tmp/note.md" || len(diagnostics) != 1 || diagnostics[0].Source != "tmp-old" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
}

func TestWalkDefaultExcludedDirectoryDoesNotClaimIdentity(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, ".git/note.md")
	if err := os.Symlink(filepath.Join(directory, ".git"), filepath.Join(directory, "docs")); err != nil {
		t.Fatal(err)
	}
	files, diagnostics, err := Walk(directory, Options{FollowSymlinks: []string{"docs"}})
	if err != nil || len(diagnostics) != 0 || len(files) != 1 || files[0].LogicalPath != "docs/note.md" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
}

func TestWalkUnreadableDirectoryDiagnostic(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, "denied/private.md")
	writeFile(t, directory, "kept.md")
	denied := filepath.Join(directory, "denied")
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o700) })
	if _, err := os.ReadDir(denied); err == nil {
		t.Skip("current user bypasses directory permissions")
	}
	files, diagnostics, err := Walk(directory, Options{})
	if err != nil || len(files) != 1 || files[0].LogicalPath != "kept.md" || len(diagnostics) != 1 || diagnostics[0].Code != "unreadable-file" || diagnostics[0].Source != "denied" {
		t.Fatalf("result = %v/%v/%v", files, diagnostics, err)
	}
}

func TestInspectRemovedEntry(t *testing.T) {
	info, diagnostic := inspectFile(filepath.Join(t.TempDir(), "removed.md"), "removed.md")
	if info != nil || diagnostic == nil || diagnostic.Code != "unreadable-file" || diagnostic.Source != "removed.md" {
		t.Fatalf("inspection = %v/%v", info, diagnostic)
	}
}

func TestWalkUnreadableRootIsFatal(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
	if _, err := os.ReadDir(directory); err == nil {
		t.Skip("current user bypasses directory permissions")
	}
	if _, _, err := Walk(directory, Options{}); err == nil {
		t.Fatal("unreadable root must be fatal")
	}
}
