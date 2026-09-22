//go:build darwin || linux

package root

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestWalkSkipsFIFO(t *testing.T) {
	directory := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(directory, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, _, err := Walk(directory, Options{})
	if err != nil || len(files) != 0 {
		t.Fatalf("Walk() = %v, %v", files, err)
	}
}

func TestGitignoreFIFOIsNotRead(t *testing.T) {
	for _, filename := range []string{".gitignore", ".mdlinkignore"} {
		t.Run(filename, func(t *testing.T) {
			directory := t.TempDir()
			writeFile(t, directory, "keep.md")
			if err := syscall.Mkfifo(filepath.Join(directory, filename), 0600); err != nil {
				t.Fatal(err)
			}
			files, diags, err := Walk(directory, Options{})
			if err != nil || len(diags) != 0 || len(files) != 1 {
				t.Fatal(files, diags, err)
			}

		})
	}
}
