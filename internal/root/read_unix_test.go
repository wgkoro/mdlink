//go:build darwin || linux

package root

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadMarkdownRejectsFIFOReplacement(t *testing.T) {
	if os.Getenv("MDLINK_TEST_FIFO") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestReadMarkdownRejectsFIFOReplacement$")
		cmd.Env = append(os.Environ(), "MDLINK_TEST_FIFO=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("FIFO subprocess: %v context=%v\n%s", err, ctx.Err(), output)
		}
		return
	}
	directory := t.TempDir()
	writeFile(t, directory, "note.md")
	files, _, err := Walk(directory, Options{})
	if err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(directory, "note.md")
	if err := os.Remove(physical); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(physical, 0o600); err != nil {
		t.Fatal(err)
	}
	body, diagnostic := ReadMarkdown(files[0])
	if len(body) != 0 || diagnostic == nil || diagnostic.Code != "unreadable-file" {
		t.Fatalf("body/diagnostic = %q/%v", body, diagnostic)
	}
}
