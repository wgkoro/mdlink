package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOutgoingRequiresTarget(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mdlink")
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	cmd := exec.Command(binary, "outgoing")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
		t.Fatalf("exit = %v; want code 2", err)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestVersionBuildMetadata(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "mdlink")
	ldflags := "-X mdlink/internal/app.Version=v0.1.0-test -X mdlink/internal/app.Commit=testcommit -X mdlink/internal/app.BuildDate=2026-09-03T00:00:00Z"
	if output, err := exec.Command("go", "build", "-ldflags", ldflags, "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	cmd := exec.Command(binary, "version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	want := "mdlink v0.1.0-test\ncommit: testcommit\nbuild date: 2026-09-03T00:00:00Z\ngo: " + runtime.Version() + "\n"
	if err != nil || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("error/stdout/stderr = %v/%q/%q", err, stdout.String(), stderr.String())
	}
}
