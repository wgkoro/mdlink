package root

import (
	"os"
	"path/filepath"
	"testing"
)

type withoutSystemInfo struct{ os.FileInfo }

func (withoutSystemInfo) Sys() any { return nil }

func TestPhysicalIDFallbackCanonicalPath(t *testing.T) {
	directory := t.TempDir()
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, directory)
	if err != nil {
		t.Fatal(err)
	}
	got, err := physicalID(withoutSystemInfo{info}, relative)
	want, wantErr := filepath.EvalSymlinks(directory)
	if err != nil || wantErr != nil || got != want {
		t.Fatalf("fallback = %q, %v; want %q, %v", got, err, want, wantErr)
	}
}
