package root

import (
	"os"
	"path/filepath"
	"testing"
)

func fileID(t *testing.T, physical string) string {
	t.Helper()
	info, err := os.Stat(physical)
	if err != nil {
		t.Fatal(err)
	}
	id, err := physicalID(info, physical)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestReadDirectoryRejectsSymlinkReplacement(t *testing.T) {
	directory, external := t.TempDir(), t.TempDir()
	writeFile(t, directory, "child/original.md")
	writeFile(t, external, "outside.md")
	rootID := fileID(t, directory)
	child := filepath.Join(directory, "child")
	childID := fileID(t, child)
	if err := os.Rename(child, filepath.Join(directory, "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, child); err != nil {
		t.Fatal(err)
	}
	entries, err := readDirectory(directory, rootID, "child", childID)
	if err == nil || len(entries) != 0 {
		t.Fatalf("outside entries returned: %v, %v", entries, err)
	}
}

func TestReadDirectoryRejectsIntermediateSymlinkReplacement(t *testing.T) {
	directory, external := t.TempDir(), t.TempDir()
	writeFile(t, directory, "branch/child/original.md")
	writeFile(t, external, "child/outside.md")
	rootID := fileID(t, directory)
	childID := fileID(t, filepath.Join(directory, "branch", "child"))
	if err := os.Rename(filepath.Join(directory, "branch"), filepath.Join(directory, "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(directory, "branch")); err != nil {
		t.Fatal(err)
	}
	entries, err := readDirectory(directory, rootID, "branch/child", childID)
	if err == nil || len(entries) != 0 {
		t.Fatalf("outside entries returned: %v, %v", entries, err)
	}
}

func TestReadDirectoryRejectsOriginReplacement(t *testing.T) {
	parent := t.TempDir()
	origin := filepath.Join(parent, "origin")
	writeFile(t, origin, "original.md")
	rootID := fileID(t, origin)
	if err := os.Rename(origin, filepath.Join(parent, "old")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, origin, "outside.md")
	entries, err := readDirectory(origin, rootID, ".", rootID)
	if err == nil || len(entries) != 0 {
		t.Fatalf("replacement origin returned: %v, %v", entries, err)
	}
}

func TestReadDirectoryRejectsDifferentDirectory(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, "child/original.md")
	rootID := fileID(t, directory)
	child := filepath.Join(directory, "child")
	childID := fileID(t, child)
	if err := os.Rename(child, filepath.Join(directory, "old")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, child, "replacement.md")
	entries, err := readDirectory(directory, rootID, "child", childID)
	if err == nil || len(entries) != 0 {
		t.Fatalf("different directory returned: %v, %v", entries, err)
	}
}
