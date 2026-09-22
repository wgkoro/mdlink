package root

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type eofActionFile struct {
	*os.File
	onEOF func()
}

func (f *eofActionFile) Read(p []byte) (int, error) {
	n, err := f.File.Read(p)
	if err == io.EOF && f.onEOF != nil {
		action := f.onEOF
		f.onEOF = nil
		action()
	}
	return n, err
}

func TestReadStableBody(t *testing.T) {
	for _, mode := range []string{"unchanged", "size", "mtime", "stat failure", "read failure"} {
		t.Run(mode, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "note.md")
			const content = "[[Target]]"
			if err := os.WriteFile(filename, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			originalTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			if err := os.Chtimes(filename, originalTime, originalTime); err != nil {
				t.Fatal(err)
			}
			input, err := os.Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			before, err := input.Stat()
			if err != nil {
				t.Fatal(err)
			}
			observed := 0
			wrapped := &eofActionFile{File: input, onEOF: func() {
				observed++
				switch mode {
				case "size":
					if err := os.Truncate(filename, before.Size()+1); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(filename, before.ModTime(), before.ModTime()); err != nil {
						t.Fatal(err)
					}
				case "mtime":
					changedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
					if err := os.Chtimes(filename, changedTime, changedTime); err != nil {
						t.Fatal(err)
					}
					after, err := input.Stat()
					if err != nil || after.Size() != before.Size() || after.ModTime().Equal(before.ModTime()) {
						t.Fatalf("mtime fixture did not change time alone: %v/%v", after, err)
					}
				case "stat failure":
					if err := input.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}}
			if mode == "read failure" {
				if err := input.Close(); err != nil {
					t.Fatal(err)
				}
			}
			body, code := readStableBody(wrapped, before)
			wantCode := ""
			if mode == "size" || mode == "mtime" {
				wantCode = "changed-during-read"
			}
			if mode == "stat failure" || mode == "read failure" {
				wantCode = "unreadable-file"
			}
			if code != wantCode || (code != "" && body != nil) || (code == "" && string(body) != content) {
				t.Fatalf("body/code = %q/%q; want code %q", body, code, wantCode)
			}
			wantObserved := 1
			if mode == "read failure" {
				wantObserved = 0
			}
			if observed != wantObserved {
				t.Fatalf("EOF action count = %d; want %d", observed, wantObserved)
			}
		})
	}
}
