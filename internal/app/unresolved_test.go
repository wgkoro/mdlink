package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnresolvedCLI(t *testing.T) {
	directory := fixture(t, map[string]string{"A.md": "[[Missing]] [[Missing#H]]", "B.md": "[[Missing]]"})
	checkRun(t, []string{"unresolved", "--root", directory}, 0, "Missing\n", true)
	checkRun(t, []string{"unresolved", "--root", directory, "--counts"}, 0, "3\tMissing\n", true)
	for _, extra := range [][]string{{"Target"}, {"--unknown"}, {"--format", "bad"}, {"--exclude", "../outside"}} {
		checkRun(t, append([]string{"unresolved", "--root", directory}, extra...), 2, "", true)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"unresolved", "--root", "/does/not/exist", "--help"}, &out, &errOut); code != 0 || strings.Contains(out.String(), "TARGET") || !strings.Contains(out.String(), "Usage: mdlink unresolved") || errOut.Len() != 0 {
		t.Fatal(code, out.String(), errOut.String())
	}
	for _, body := range []string{"", "[[Missing]]", "[[../unsafe]]", "[[Missing]] [[../unsafe]]"} {
		directory := fixture(t, map[string]string{"A.md": body})
		for _, format := range []string{"text", "json"} {
			for _, strict := range []bool{false, true} {
				args := []string{"unresolved", "--root", directory, "--format", format}
				want := 0
				if strict {
					args = append(args, "--strict")
					if body != "" {
						want = 3
					}
				}
				out.Reset()
				errOut.Reset()
				if code := Run(args, &out, &errOut); code != want {
					t.Fatal(code, out.String(), errOut.String())
				}
				if format == "json" {
					var value map[string]json.RawMessage
					if err := json.Unmarshal(out.Bytes(), &value); err != nil {
						t.Fatal(err)
					}
					if _, found := value["target"]; found || string(value["command"]) != "\"unresolved\"" || string(value["root"]) != "\".\"" || errOut.Len() != 0 || strings.Count(out.String(), "\n") != 1 {
						t.Fatal(out.String(), errOut.String())
					}
					var counted bytes.Buffer
					if code := Run(append(args, "--counts"), &counted, &errOut); code != want || counted.String() != out.String() {
						t.Fatal(counted.String())
					}
				}
			}
		}
	}
	checkRun(t, []string{"unresolved", "--root", t.TempDir(), "--strict"}, 0, "", false)
}

func TestUnresolvedLimitsAndFailurePriority(t *testing.T) {
	directory := fixture(t, map[string]string{"A.md": strings.Repeat("[[Missing]] ", 101)})
	var out, errOut bytes.Buffer
	args := []string{"unresolved", "--root", directory, "--strict"}
	if code := Run(args, &out, &errOut); code != 3 || out.String() != "Missing\n" || strings.Count(errOut.String(), "unresolved-link:") != 100 || !strings.HasSuffix(errOut.String(), "diagnostics: 101\n") {
		t.Fatal(code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(append(args, "--format", "json"), &out, &errOut); code != 3 {
		t.Fatal(code)
	}
	var response struct{ Diagnostics []any }
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || len(response.Diagnostics) != 101 || errOut.Len() != 0 {
		t.Fatal(err, out.String())
	}
	for _, format := range []string{"text", "json"} {
		if code := Run(append(args, "--format", format), failingWriter{}, &errOut); code != 1 {
			t.Fatal(code)
		}
	}
	if code := Run(args, &out, failingWriter{}); code != 1 {
		t.Fatal(code)
	}
	checkRun(t, []string{"unresolved", "--root", "", "--strict"}, 2, "", true)
	if err := os.Symlink("missing", filepath.Join(directory, "alias")); err != nil {
		t.Fatal(err)
	}
	checkRun(t, append(args, "--follow-symlink", "alias"), 1, "", true)
	for _, control := range []string{"\r", "\n", "\t"} {
		directory := fixture(t, map[string]string{"A" + control + ".md": "[[Missing]]"})
		checkRun(t, []string{"unresolved", "--root", directory, "--strict"}, 2, "", true)
	}
}

func TestUnresolvedRootAndCatalogOptions(t *testing.T) {
	for _, mode := range []string{"explicit", "environment", "ancestor"} {
		t.Run(mode, func(t *testing.T) {
			directory := fixture(t, map[string]string{"A.md": "[[Known]] [[external/Target]]", "Known.md": ""})
			external := fixture(t, map[string]string{"Target.md": "[[Missing]]"})
			if err := os.Symlink(external, filepath.Join(directory, "external")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("MDLINK_ROOT", "")
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			args := []string{"unresolved", "--format", "json", "--exclude", "Known.md"}
			switch mode {
			case "explicit":
				args = append(args, "--root", directory, "--follow-symlink", "external")
			case "environment":
				t.Setenv("MDLINK_ROOT", directory)
				t.Setenv("MDLINK_FOLLOW_SYMLINKS", "external")
			case "ancestor":
				if err := os.Mkdir(filepath.Join(directory, ".obsidian"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Chdir(directory)
				args = append(args, "--follow-symlink", "external")
			}
			var out, errOut bytes.Buffer
			if code := Run(args, &out, &errOut); code != 0 || errOut.Len() != 0 {
				t.Fatal(code, errOut.String())
			}
			var response struct {
				Results []struct {
					Target  string
					Sources []struct{ Path string }
				}
			}
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Results) != 2 || response.Results[0].Target != "Known" || response.Results[1].Target != "Missing" || response.Results[1].Sources[0].Path != "external/Target.md" || strings.Contains(out.String(), directory) || strings.Contains(out.String(), external) {
				t.Fatal(out.String())
			}
		})
	}
}

func TestUnresolvedControlTarget(t *testing.T) {
	directory := fixture(t, map[string]string{"A.md": "[[bad\tname]]"})
	checkRun(t, []string{"unresolved", "--root", directory, "--strict"}, 2, "", true)
	var out, errOut bytes.Buffer
	if code := Run([]string{"unresolved", "--root", directory, "--format", "json"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), `bad\tname`) || errOut.Len() != 0 {
		t.Fatal(code, out.String(), errOut.String())
	}
}
