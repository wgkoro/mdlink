package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	for name, body := range files {
		filename := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func checkRun(t *testing.T, args []string, wantCode int, wantOutput string, wantDiagnostic bool) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	if code != wantCode || stdout.String() != wantOutput || (stderr.Len() > 0) != wantDiagnostic {
		t.Fatalf("%v: code/stdout/stderr = %d/%q/%q", args, code, stdout.String(), stderr.String())
	}
	return stderr.String()
}

func TestQueryRootSelection(t *testing.T) {
	files := map[string]string{"Source.md": "[[Target]]", "Target.md": ""}
	for _, mode := range []string{"explicit", "relative", "symlink", "environment", "nearest ancestor"} {
		t.Run(mode, func(t *testing.T) {
			directory := fixture(t, files)
			other := fixture(t, map[string]string{"Source.md": "[[Wrong]]", "Wrong.md": ""})
			t.Setenv("MDLINK_ROOT", other)
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			args := []string{"outgoing"}
			switch mode {
			case "explicit":
				args = append(args, "--root", directory)
			case "relative":
				t.Chdir(filepath.Dir(directory))
				args = append(args, "--root", filepath.Base(directory))
			case "symlink":
				alias := filepath.Join(t.TempDir(), "alias")
				if err := os.Symlink(directory, alias); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--root", alias)
			case "environment":
				t.Setenv("MDLINK_ROOT", directory)
				if err := os.Mkdir(filepath.Join(other, ".obsidian"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Chdir(other)
			case "nearest ancestor":
				t.Setenv("MDLINK_ROOT", "")
				if err := os.Mkdir(filepath.Join(directory, ".obsidian"), 0700); err != nil {
					t.Fatal(err)
				}
				child := filepath.Join(directory, "child")
				if err := os.Mkdir(child, 0700); err != nil {
					t.Fatal(err)
				}
				// A regular marker in the child must not stop the ancestor search.
				if err := os.WriteFile(filepath.Join(child, ".obsidian"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				t.Chdir(child)
			}
			checkRun(t, append(args, "Source"), 0, "Target.md\n", false)
			args[0] = "backlinks"
			checkRun(t, append(args, "Target"), 0, "Source.md\n", false)
		})
	}
}

func TestOutgoingInvalidRoot(t *testing.T) {
	directory := fixture(t, map[string]string{"file": ""})
	t.Setenv("MDLINK_ROOT", "")
	t.Chdir(directory)
	for _, args := range [][]string{
		{"outgoing", "--root", "", "Source"},
		{"outgoing", "--root", filepath.Join(directory, "missing"), "Source"},
		{"outgoing", "--root", filepath.Join(directory, "file"), "Source"},
		{"outgoing", "Source"},
	} {
		checkRun(t, args, 2, "", true)
	}
}

func TestOutgoingChoosesClosestMarker(t *testing.T) {
	directory := fixture(t, map[string]string{
		".obsidian/config": "", "Source.md": "[[Wrong]]", "Wrong.md": "",
		"inner/.obsidian/config": "", "inner/Source.md": "[[Target]]", "inner/Target.md": "",
		"inner/deep/file": "",
	})
	t.Setenv("MDLINK_ROOT", "")
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	t.Chdir(filepath.Join(directory, "inner", "deep"))
	checkRun(t, []string{"outgoing", "Source"}, 0, "Target.md\n", false)
}

func TestQueryFollowAndExclude(t *testing.T) {
	for _, mode := range []string{"environment", "flags override", "exclude", "multiple excludes"} {
		t.Run(mode, func(t *testing.T) {
			directory := fixture(t, map[string]string{"Source.md": "[[a/Target]] [[b/Target]]"})
			for _, name := range []string{"a", "b"} {
				external := fixture(t, map[string]string{"Target.md": "[[Source]]"})
				if err := os.Symlink(external, filepath.Join(directory, name)); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "a"+string(os.PathListSeparator)+"b")
			args := []string{"outgoing", "--root", directory}
			want, diagnostic := "a/Target.md\nb/Target.md\n", false
			switch mode {
			case "flags override":
				t.Setenv("MDLINK_FOLLOW_SYMLINKS", "missing")
				args = append(args, "--follow-symlink", "a", "--follow-symlink", "b")
			case "exclude":
				args = append(args, "--exclude", "b")
				want, diagnostic = "a/Target.md\n", true
			case "multiple excludes":
				args = append(args, "--exclude", "a", "--exclude", "b")
				want, diagnostic = "", true
			}
			checkRun(t, append(args, "Source"), 0, want, diagnostic)
			args[0] = "backlinks"
			checkRun(t, append(args, "Source"), 0, want, diagnostic)
		})
	}
}

func TestOutgoingInvalidOptions(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": ""})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	for _, option := range [][]string{
		{"--exclude", ""}, {"--exclude", "/absolute"}, {"--exclude", "../outside"},
		{"--follow-symlink", ""}, {"--follow-symlink", "../outside"},
		{"--follow-symlink", "missing"}, {"--follow-symlink", "Source.md"},
	} {
		args := append([]string{"outgoing", "--root", directory}, option...)
		checkRun(t, append(args, "Source"), 2, "", true)
	}
}

func TestOutgoingTargetContract(t *testing.T) {
	directory := fixture(t, map[string]string{
		"folder/Source.md": "[[Target]]", "Target.md": "",
		"a/Shared.md": "", "b/Shared.md": "", "image.png": "",
		"Name#Part.md": "[[Target]]", "A%20B.md": "[[Target]]",
	})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	for _, target := range []string{"folder/Source.md", "folder/Source", "Source.md", "Source", "Name#Part", "A%20B"} {
		checkRun(t, []string{"outgoing", "--root", directory, target}, 0, "Target.md\n", false)
	}
	for _, target := range []string{"Shared", "missing", "../Source", "/Source", "image.png"} {
		checkRun(t, []string{"outgoing", "--root", directory, target}, 2, "", true)
	}
}

func TestOutgoingSelfAndEmptySource(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[#Heading]]", "Empty.md": ""})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	checkRun(t, []string{"outgoing", "--root", directory, "Source"}, 0, "Source.md\n", false)
	checkRun(t, []string{"outgoing", "--root", directory, "Empty"}, 0, "", false)
}

func TestOutgoingDiagnostics(t *testing.T) {
	directory := fixture(t, map[string]string{
		"Source.md": "[[Target]] [[missing]] [[Shared]] [[../outside]]",
		"Target.md": "", "a/Shared.md": "", "b/Shared.md": "",
	})
	if err := os.Symlink("missing", filepath.Join(directory, "alias")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"outgoing", "--root", directory, "Source"}, &stdout, &stderr)
	if code != 0 || stdout.String() != "Target.md\n" {
		t.Fatalf("code/output = %d/%q", code, stdout.String())
	}
	for _, diagnostic := range []string{"skipped-symlink", "unresolved-link", "ambiguous-link", "unsafe-path"} {
		if !strings.Contains(stderr.String(), diagnostic+":") {
			t.Fatalf("missing %s: %s", diagnostic, &stderr)
		}
	}
}

func TestOutgoingPermissionBoundaries(t *testing.T) {
	for _, mode := range []string{"source", "unrelated body", "root", "child directory"} {
		t.Run(mode, func(t *testing.T) {
			directory := fixture(t, map[string]string{"Source.md": "[[Target]]", "Target.md": "", "child/Other.md": ""})
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			denied := filepath.Join(directory, "Source.md")
			wantCode, wantOutput, diagnostic := 1, "", true
			switch mode {
			case "unrelated body":
				denied = filepath.Join(directory, "Target.md")
				wantCode, wantOutput, diagnostic = 0, "Target.md\n", false
			case "root":
				denied = directory
			case "child directory":
				denied = filepath.Join(directory, "child")
				wantCode, wantOutput = 0, "Target.md\n"
			}
			if err := os.Chmod(denied, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(denied, 0700) })
			if file, err := os.Open(denied); err == nil {
				file.Close()
				t.Skip("current user bypasses permissions")
			}
			args := []string{"outgoing", "--root", directory, "Source"}
			gotDiagnostic := checkRun(t, args, wantCode, wantOutput, diagnostic)
			if mode == "source" && !strings.Contains(gotDiagnostic, "unreadable-file") {
				t.Fatalf("missing reader diagnostic: %s", gotDiagnostic)
			}
			strictCode := wantCode
			if mode == "child directory" {
				strictCode = 3
			}
			checkRun(t, []string{"outgoing", "--root", directory, "--strict", "Source"}, strictCode, wantOutput, diagnostic)
		})
	}
}

func TestOutgoingFatalWalkHasNoOutput(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[Target]]", "Target.md": ""})
	if err := os.Symlink("Target.md", filepath.Join(directory, "alias.md")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"outgoing", "--root", directory, "--follow-symlink", "alias.md", "Source"}, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "duplicate-physical-file") {
		t.Fatalf("code/output/diagnostic = %d/%q/%q", code, &stdout, &stderr)
	}
}

func TestOutgoingBrokenAllowedSymlink(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": ""})
	if err := os.Symlink("missing", filepath.Join(directory, "alias")); err != nil {
		t.Fatal(err)
	}
	checkRun(t, []string{"outgoing", "--root", directory, "--follow-symlink", "alias", "Source"}, 1, "", true)
}

func TestQueryOutputFailures(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[Target]]", "Target.md": "", "Warning.md": "[[missing]]"})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	for _, command := range []string{"outgoing", "backlinks"} {
		for _, args := range [][]string{{command, "--help"}, {command, "--unknown"}} {
			if code := Run(args, failingWriter{}, failingWriter{}); code != 1 {
				t.Fatalf("%v: code = %d", args, code)
			}
		}
		target := "Source"
		if command == "backlinks" {
			target = "Target"
		}
		for _, counts := range []bool{false, true} {
			args := []string{command, "--root", directory}
			if counts {
				args = append(args, "--counts")
			}
			var stderr bytes.Buffer
			if code := Run(append(args, target), failingWriter{}, &stderr); code != 1 {
				t.Fatalf("%s counts=%v: stdout failure code = %d", command, counts, code)
			}
		}
		var stdout bytes.Buffer
		if code := Run([]string{command, "--root", directory, "Warning"}, &stdout, failingWriter{}); code != 1 {
			t.Fatalf("%s: diagnostic stderr failure code = %d", command, code)
		}
	}
}

func TestQueryHelpDoesNotAccessRoot(t *testing.T) {
	for _, command := range []string{"outgoing", "backlinks"} {

		t.Setenv("MDLINK_ROOT", filepath.Join(t.TempDir(), "missing"))
		t.Setenv("MDLINK_FOLLOW_SYMLINKS", "../invalid")
		var stdout, stderr bytes.Buffer
		if code := Run([]string{command, "--root", "", "--help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "Usage: mdlink "+command+" ") || stderr.Len() != 0 {
			t.Fatalf("code/output/diagnostic = %d/%q/%q", code, &stdout, &stderr)
		}

	}
}
func TestOutgoingRejectsControlPaths(t *testing.T) {
	for _, location := range []string{"target", "result", "diagnostic source", "candidate"} {
		t.Run(location, func(t *testing.T) {
			files := map[string]string{"Source.md": "[[A]]", "A.md": ""}
			target := "Source"
			switch location {
			case "target":
				target = "Bad\nName"
				files[target+".md"] = "[[A]]"
			case "result":
				files["Source.md"] += " [[Bad\tName]]"
				files["Bad\tName.md"] = ""
			case "candidate":
				files["Source.md"] += " [[Shared]]"
				files["zbad\t/Shared.md"], files["a/Shared.md"] = "", ""
			}
			directory := fixture(t, files)
			if location == "diagnostic source" {
				if err := os.Symlink("missing", filepath.Join(directory, "bad\rname")); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			diagnostic := checkRun(t, []string{"outgoing", "--root", directory, target}, 2, "", true)
			if !strings.Contains(diagnostic, "--format json") {
				t.Fatalf("missing JSON guidance: %s", diagnostic)
			}
		})
	}
}

func TestOutgoingAggregatedText(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[Z]] [[A]] [[Z|alias]] ![[A#Heading]]", "A.md": "", "Z.md": ""})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	checkRun(t, []string{"outgoing", "--root", directory, "Source"}, 0, "A.md\nZ.md\n", false)
}

func TestOutgoingCounts(t *testing.T) {
	directory := fixture(t, map[string]string{
		"Source.md": "[[Z]] [[A]] [[A|alias]] [[A#Heading]] ![[A]]",
		"A.md":      "", "Z.md": "", "Empty.md": "",
	})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	checkRun(t, []string{"outgoing", "--root", directory, "--counts", "Source"}, 0, "4\tA.md\n1\tZ.md\n", false)
	checkRun(t, []string{"outgoing", "--root", directory, "--counts", "Empty"}, 0, "", false)
	checkRun(t, []string{"outgoing", "--root", directory, "Source"}, 0, "A.md\nZ.md\n", false)
}

func TestOutgoingQuotesInvalidArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"outgoing", "--bad\nargument"}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("code/output/diagnostic = %d/%q/%q", code, &stdout, &stderr)
	}
}
