package app

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"

	"mdlink/internal/root"
	"strings"
	"testing"
)

func TestBacklinksOutputGolden(t *testing.T) {
	directory := fixture(t, map[string]string{
		"target.png": "", "Z.md": "[[target.png]]", "a/Source.md": "[[target.png]] ![image](/target.png)", "A\u030a.md": "[[target.png]]", "Unrelated.md": "[[missing]]",
	})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	for _, format := range []string{"text", "counts", "json"} {
		want, err := os.ReadFile("testdata/backlinks." + format)
		if err != nil {
			t.Fatal(err)
		}
		args := []string{"backlinks", "--root", directory}
		if format == "counts" {
			args = append(args, "--counts")
		}
		if format == "json" {
			args = append(args, "--format", "json")
		}
		var stdout, stderr bytes.Buffer
		code := Run(append(args, "target.png"), &stdout, &stderr)
		wantErr := "unresolved-link: \"Unrelated.md\" offset=0 raw_target=\"missing\"\ndiagnostics: 1\n"
		if format == "json" {
			wantErr = ""
		}
		if code != 0 || stdout.String() != string(want) || stderr.String() != wantErr {
			t.Fatalf("%s: code/stdout/stderr = %d/%q/%q", format, code, &stdout, &stderr)
		}
	}
}

func TestBacklinksEmptyOutput(t *testing.T) {
	directory := fixture(t, map[string]string{"Target.md": ""})
	checkRun(t, []string{"backlinks", "--root", directory, "--counts", "--strict", "Target"}, 0, "", false)
	want := "{\"schema_version\":1,\"command\":\"backlinks\",\"root\":\".\",\"target\":\"Target.md\",\"results\":[],\"diagnostics\":[]}\n"
	for _, counts := range []bool{false, true} {
		args := []string{"backlinks", "--root", directory, "--format", "json", "--strict"}
		if counts {
			args = append(args, "--counts")
		}
		checkRun(t, append(args, "Target"), 0, want, false)
	}
}

func TestBacklinksControlPaths(t *testing.T) {
	for _, location := range []string{"source", "target"} {
		target, source := "Target.md", "Source.md"
		if location == "source" {
			source = "Source\r\n\t.md"
		} else {
			target = "Target\r\n\t.md"
		}
		body := "[link](" + url.PathEscape(target) + ")"
		directory := fixture(t, map[string]string{target: "", source: body, "A.md": body})
		t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
		message := checkRun(t, []string{"backlinks", "--root", directory, target}, 2, "", true)
		if !strings.Contains(message, "--format json") {
			t.Fatalf("missing guidance: %s", message)
		}
		var stdout, stderr bytes.Buffer
		code := Run([]string{"backlinks", "--root", directory, "--format", "json", target}, &stdout, &stderr)
		var got struct {
			Target  string
			Results []struct {
				Path  string
				Count int
			}
		}
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if code != 0 || stderr.Len() != 0 || got.Target != target || len(got.Results) != 2 || got.Results[0].Path != "A.md" || got.Results[1].Path != source || got.Results[0].Count != 1 || got.Results[1].Count != 1 || strings.Count(stdout.String(), "\n") != 1 {
			t.Fatalf("%s: code/output/diagnostic = %d/%q/%q", location, code, &stdout, &stderr)
		}
	}
}

func TestBacklinksSkipsSourceFailures(t *testing.T) {
	for _, diagnostic := range []string{"unreadable-file", "file-too-large", "invalid-encoding"} {
		t.Run(diagnostic, func(t *testing.T) {
			directory := fixture(t, map[string]string{"Bad.md": "[[Target]]\xff", "Healthy.md": "[[Target]]", "Target.md": ""})
			filename := filepath.Join(directory, "Bad.md")
			if diagnostic == "file-too-large" {
				if err := os.Truncate(filename, root.MaxBodySize+1); err != nil {
					t.Fatal(err)
				}
			}
			if diagnostic == "unreadable-file" {
				if err := os.Chmod(filename, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(filename, 0600) })
				if f, err := os.Open(filename); err == nil {
					f.Close()
					t.Skip("current user bypasses permissions")
				}
			}
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			for _, format := range []string{"text", "json"} {
				for _, strict := range []bool{false, true} {
					args := []string{"backlinks", "--root", directory, "--format", format}
					wantCode := 0
					if strict {
						args = append(args, "--strict")
						wantCode = 3
					}
					var stdout, stderr bytes.Buffer
					code := Run(append(args, "Target"), &stdout, &stderr)
					if code != wantCode {
						t.Fatalf("code = %d: %s", code, &stderr)
					}
					if format == "text" {
						if stdout.String() != "Healthy.md\n" || !strings.Contains(stderr.String(), diagnostic+": \"Bad.md\"") {
							t.Fatalf("output/diagnostic = %q/%q", &stdout, &stderr)
						}
					} else {
						var got struct {
							Results []struct {
								Path  string
								Count int
							}
							Diagnostics []map[string]any
						}
						if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
							t.Fatal(err)
						}
						if stderr.Len() != 0 || len(got.Results) != 1 || got.Results[0].Path != "Healthy.md" || got.Results[0].Count != 1 || len(got.Diagnostics) != 1 || got.Diagnostics[0]["code"] != diagnostic || got.Diagnostics[0]["source"] != "Bad.md" || len(got.Diagnostics[0]) != 2 {
							t.Fatalf("output/diagnostic = %q/%q", &stdout, &stderr)
						}
					}
				}
			}
		})
	}
}
