package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOutgoingMarkdownContract(t *testing.T) {
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	binary := filepath.Join(t.TempDir(), "mdlink")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	directory := t.TempDir()
	files := map[string]string{
		"a/Source.md": "[[Note]] [root](../Note.md) [relative](./Local.md#Heading) [root again](/Note.md) ![image](../asset.png) [local](Local.md) ![[Note#Missing]] [external](https://example.invalid/x) ![external](data:image/png,abc) `[hidden](Note.md)` <!-- [hidden](Note.md) -->",
		"a/Unsafe.md": "[escape](..%2F..%2FNote.md) [[Note]]",
		"Note.md":     "", "a/Local.md": "", "a/Note.md": "", "asset.png": "",
	}
	for name, body := range files {
		filename := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name, target, text, counts, json, diagnostic string
		strictCode                                   int
	}{
		{"mixed", "a/Source.md", "Note.md\na/Local.md\nasset.png\n", "4\tNote.md\n2\ta/Local.md\n1\tasset.png\n", `{"schema_version":1,"command":"outgoing","root":".","target":"a/Source.md","results":[{"path":"Note.md","count":4},{"path":"a/Local.md","count":2},{"path":"asset.png","count":1}],"diagnostics":[]}`, "", 0},
		{"unsafe", "a/Unsafe.md", "Note.md\n", "1\tNote.md\n", `{"schema_version":1,"command":"outgoing","root":".","target":"a/Unsafe.md","results":[{"path":"Note.md","count":1}],"diagnostics":[{"code":"unsafe-path","source":"a/Unsafe.md","offset":0,"raw_target":"..%2F..%2FNote.md"}]}`, "unsafe-path: \"a/Unsafe.md\" offset=0 raw_target=\"..%2F..%2FNote.md\"\ndiagnostics: 1\n", 3},
	} {
		for _, format := range []string{"text", "counts", "json"} {
			for _, strict := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/strict=%t", tt.name, format, strict), func(t *testing.T) {
					args := []string{"outgoing", "--root", directory}
					wantOut, wantErr := tt.text, tt.diagnostic
					if format == "counts" {
						args = append(args, "--counts")
						wantOut = tt.counts
					}
					if format == "json" {
						args = append(args, "--format", "json")
						wantOut, wantErr = tt.json, ""
					}
					wantCode := 0
					if strict {
						args = append(args, "--strict")
						wantCode = tt.strictCode
					}
					args = append(args, tt.target)
					cmd := exec.Command(binary, args...)
					var stdout, stderr bytes.Buffer
					cmd.Stdout, cmd.Stderr = &stdout, &stderr
					err := cmd.Run()
					code := 0
					if exit, ok := err.(*exec.ExitError); ok {
						code = exit.ExitCode()
					} else if err != nil {
						t.Fatal(err)
					}
					if code != wantCode || stderr.String() != wantErr {
						t.Fatalf("code/stderr = %d/%q; want %d/%q", code, stderr.String(), wantCode, wantErr)
					}
					if format == "json" {
						var got, want any
						if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal([]byte(wantOut), &want); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("JSON = %s; want %s", stdout.String(), wantOut)
						}
					} else if stdout.String() != wantOut {
						t.Fatalf("stdout = %q; want %q", stdout.String(), wantOut)
					}
				})
			}
		}
	}
}
