package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mdlink/internal/root"
)

func TestQueryStrictPreservesResults(t *testing.T) {
	for _, command := range []string{"outgoing", "backlinks"} {
		selected, resultPath := "Source", "Target.md"
		if command == "backlinks" {
			selected, resultPath = "Target", "Source.md"
		}

		for code, body := range map[string]string{
			"unresolved-link": "[[missing]]", "ambiguous-link": "[[Shared]]",
			"unsafe-path": "[[../outside]]", "skipped-symlink": "",
		} {
			t.Run(code, func(t *testing.T) {
				directory := fixture(t, map[string]string{"Source.md": "[[Target]] " + body, "Target.md": "", "a/Shared.md": "", "b/Shared.md": ""})
				if code == "skipped-symlink" {
					if err := os.Symlink("missing", filepath.Join(directory, "alias")); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
				for _, format := range []string{"text", "json"} {
					for _, strict := range []bool{false, true} {
						args := []string{command, "--root", directory, "--format", format}
						wantCode := 0
						if strict {
							args = append(args, "--strict")
							wantCode = 3
						}
						var stdout, stderr bytes.Buffer
						got := Run(append(args, selected), &stdout, &stderr)
						if got != wantCode {
							t.Fatalf("format/strict/code = %s/%v/%d: %s", format, strict, got, &stderr)
						}
						if format == "text" {
							if stdout.String() != resultPath+"\n" || !strings.Contains(stderr.String(), code+":") {
								t.Fatalf("output/diagnostic = %q/%q", &stdout, &stderr)
							}
						} else {
							var response struct {
								Results     []struct{ Path string }
								Diagnostics []struct{ Code string }
							}
							if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
								t.Fatal(err)
							}
							if stderr.Len() != 0 || len(response.Results) != 1 || response.Results[0].Path != resultPath || len(response.Diagnostics) != 1 || response.Diagnostics[0].Code != code {
								t.Fatalf("output/diagnostic = %q/%q", &stdout, &stderr)
							}
						}
					}
				}
			})
		}

	}
}
func TestQueryDiagnosticsLimit(t *testing.T) {
	for _, command := range []string{"outgoing", "backlinks"} {

		for _, count := range []int{100, 101} {
			directory := fixture(t, map[string]string{"Source.md": strings.Repeat("[[missing]] ", count)})
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			for _, format := range []string{"text", "json"} {
				var stdout, stderr bytes.Buffer
				code := Run([]string{command, "--root", directory, "--format", format, "--strict", "Source"}, &stdout, &stderr)
				if code != 3 {
					t.Fatalf("code = %d: %s", code, &stderr)
				}
				if format == "text" {
					if stdout.Len() != 0 || strings.Count(stderr.String(), "unresolved-link:") != 100 || !strings.HasSuffix(stderr.String(), fmt.Sprintf("diagnostics: %d\n", count)) {
						t.Fatalf("output/diagnostic = %q/%q", &stdout, &stderr)
					}
				} else {
					var response struct{ Diagnostics []any }
					if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					if len(response.Diagnostics) != count || stderr.Len() != 0 {
						t.Fatalf("JSON diagnostic count = %d", len(response.Diagnostics))
					}
				}
			}
		}

	}
}
func TestOutgoingSkippedSourceDiagnostics(t *testing.T) {
	for _, diagnostic := range []string{"invalid-encoding", "file-too-large"} {
		t.Run(diagnostic, func(t *testing.T) {
			directory := fixture(t, map[string]string{"Source.md": "[[Target]]\xff", "Target.md": ""})
			if diagnostic == "file-too-large" {
				if err := os.Truncate(filepath.Join(directory, "Source.md"), root.MaxBodySize+1); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			for _, format := range []string{"text", "json"} {
				for _, strict := range []bool{false, true} {
					args := []string{"outgoing", "--root", directory, "--format", format}
					wantCode := 0
					if strict {
						args = append(args, "--strict")
						wantCode = 3
					}
					var stdout, stderr bytes.Buffer
					code := Run(append(args, "Source"), &stdout, &stderr)
					if code != wantCode {
						t.Fatalf("code = %d: %s", code, &stderr)
					}
					if format == "text" {
						if stdout.Len() != 0 || !strings.Contains(stderr.String(), diagnostic+":") {
							t.Fatalf("output/diagnostic = %q/%q", &stdout, &stderr)
						}
					} else {
						var response struct {
							Results     []any
							Diagnostics []map[string]any
						}
						if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
							t.Fatal(err)
						}
						if stderr.Len() != 0 || len(response.Results) != 0 || len(response.Diagnostics) != 1 || response.Diagnostics[0]["code"] != diagnostic || len(response.Diagnostics[0]) != 2 {
							t.Fatalf("output/diagnostic = %q/%q", &stdout, &stderr)
						}
					}
				}
			}
		})
	}
}

func TestOutgoingStrictWithoutDiagnostics(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[Target]]", "Target.md": "", "Empty.md": ""})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	for _, format := range []string{"text", "json"} {
		for _, target := range []string{"Source", "Empty"} {
			var stdout, stderr bytes.Buffer
			code := Run([]string{"outgoing", "--root", directory, "--format", format, "--strict", target}, &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 {
				t.Fatalf("format/target/code/diagnostic = %s/%s/%d/%q", format, target, code, &stderr)
			}
		}
	}
}

func TestOutgoingStrictDoesNotOverrideFailure(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[Target]] [[missing]]", "Target.md": "", "Control.md": "[[bad\tname]] [[missing]]", "bad\tname.md": ""})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	for _, format := range []string{"text", "json"} {
		args := []string{"outgoing", "--root", directory, "--format", format, "--strict"}
		checkRun(t, append(args, "missing"), 2, "", true)
		if format == "text" {
			checkRun(t, append(args, "Control"), 2, "", true)
		}
		var stderr bytes.Buffer
		if code := Run(append(args, "Source"), failingWriter{}, &stderr); code != 1 {
			t.Fatalf("stdout failure code = %d", code)
		}
	}
	var stdout bytes.Buffer
	if code := Run([]string{"outgoing", "--root", directory, "--strict", "Source"}, &stdout, failingWriter{}); code != 1 {
		t.Fatalf("stderr failure code = %d", code)
	}
}
