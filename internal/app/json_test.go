package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOutgoingJSONLogicalRoot(t *testing.T) {
	for _, mode := range []string{"explicit", "environment", "ancestor"} {
		t.Run(mode, func(t *testing.T) {
			directory := fixture(t, map[string]string{"Source.md": "[[missing]] [[docs/Target]] [[docs/Target]]"})
			external := fixture(t, map[string]string{"Target.md": ""})
			if err := os.Symlink(external, filepath.Join(directory, "docs")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("MDLINK_ROOT", "")
			t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
			args := []string{"outgoing", "--format", "json", "--follow-symlink", "docs"}
			switch mode {
			case "explicit":
				args = append(args, "--root", directory)
			case "environment":
				t.Setenv("MDLINK_ROOT", directory)
			case "ancestor":
				if err := os.Mkdir(filepath.Join(directory, ".obsidian"), 0700); err != nil {
					t.Fatal(err)
				}
				t.Chdir(directory)
			}
			var stdout, stderr bytes.Buffer
			code := Run(append(args, "Source"), &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 {
				t.Fatalf("code/diagnostic = %d/%q", code, &stderr)
			}
			decoder := json.NewDecoder(strings.NewReader(stdout.String()))
			var got map[string]any
			if err := decoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				t.Fatalf("more than one JSON value: %v", err)
			}
			want := map[string]any{
				"schema_version": float64(1), "command": "outgoing", "root": ".", "target": "Source.md",
				"results":     []any{map[string]any{"path": "docs/Target.md", "count": float64(2)}},
				"diagnostics": []any{map[string]any{"code": "unresolved-link", "source": "Source.md", "offset": float64(0), "raw_target": "missing"}},
			}
			if !reflect.DeepEqual(got, want) || strings.Contains(stdout.String(), directory) || strings.Contains(stdout.String(), external) {
				t.Fatalf("response = %s", &stdout)
			}
			var counted bytes.Buffer
			code = Run(append(args, "--counts", "Source"), &counted, &stderr)
			if code != 0 || counted.String() != stdout.String() || stderr.Len() != 0 {
				t.Fatalf("counts changed JSON: %d/%q/%q", code, &counted, &stderr)
			}
		})
	}
}

func TestOutgoingJSONDiagnostics(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[missing]] [[Shared]] [[../bad]]", "a/Shared.md": "", "b/Shared.md": ""})
	if err := os.Symlink("missing", filepath.Join(directory, "alias")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"outgoing", "--root", directory, "--format", "json", "Source"}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code/diagnostic = %d/%q", code, &stderr)
	}
	var got struct{ Diagnostics []map[string]any }
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]any{
		"skipped-symlink": {"code": "skipped-symlink", "source": "alias"},
		"unresolved-link": {"code": "unresolved-link", "source": "Source.md", "offset": float64(0), "raw_target": "missing"},
		"ambiguous-link":  {"code": "ambiguous-link", "source": "Source.md", "offset": float64(12), "raw_target": "Shared", "candidates": []any{"a/Shared.md", "b/Shared.md"}},
		"unsafe-path":     {"code": "unsafe-path", "source": "Source.md", "offset": float64(23), "raw_target": "../bad"},
	}
	if len(got.Diagnostics) != len(want) {
		t.Fatalf("diagnostics = %+v", got.Diagnostics)
	}
	for _, item := range got.Diagnostics {
		if !reflect.DeepEqual(item, want[item["code"].(string)]) {
			t.Fatalf("diagnostic = %+v", item)
		}
	}
}

func TestOutgoingJSONControlPaths(t *testing.T) {
	target := "Source\r\n\t.md"
	directory := fixture(t, map[string]string{target: "[[A\tB]]", "A\tB.md": ""})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"outgoing", "--root", directory, "--format", "json", target}, &stdout, &stderr)
	var got struct {
		Target  string
		Results []struct{ Path string }
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if code != 0 || stderr.Len() != 0 || got.Target != target || len(got.Results) != 1 || got.Results[0].Path != "A\tB.md" || strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("code/output/diagnostic = %d/%q/%q", code, &stdout, &stderr)
	}
}

func TestQueryJSONFailures(t *testing.T) {
	for _, command := range []string{"outgoing", "backlinks"} {

		directory := fixture(t, map[string]string{"Source.md": ""})
		t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
		checkRun(t, []string{command, "--root", directory, "--format", "json", "missing"}, 2, "", true)
		if err := os.Symlink("missing", filepath.Join(directory, "alias")); err != nil {
			t.Fatal(err)
		}
		checkRun(t, []string{command, "--root", directory, "--format", "json", "--follow-symlink", "alias", "Source"}, 1, "", true)
		var stderr bytes.Buffer
		if code := Run([]string{command, "--root", directory, "--format", "json", "Source"}, failingWriter{}, &stderr); code != 1 || stderr.Len() == 0 {
			t.Fatalf("code/diagnostic = %d/%q", code, &stderr)
		}
		message := checkRun(t, []string{command, "--root", "", "--format", "unknown", "Source"}, 2, "", true)
		if !strings.Contains(message, "format") {
			t.Fatalf("format was not validated before root: %s", message)
		}

	}
}
