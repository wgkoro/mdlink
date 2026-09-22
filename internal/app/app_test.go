package app

import (
	"bytes"
	"errors"
	"runtime"
	"testing"
)

func TestMissingCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("code/stdout/stderr = %d/%q/%q", code, stdout.String(), stderr.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"unknown", "Note"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("code/stdout/stderr = %d/%q/%q", code, stdout.String(), stderr.String())
	}
}

func TestInvalidQueryArguments(t *testing.T) {
	for _, command := range []string{"outgoing", "backlinks"} {
		for _, tail := range [][]string{nil, {"Note", "Extra"}, {"--unknown", "Note"}} {
			args := append([]string{command}, tail...)
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Errorf("%v: code/stdout/stderr = %d/%q/%q", args, code, stdout.String(), stderr.String())
			}
		}
	}
}

func TestOutgoingHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"outgoing", "--help"}, &stdout, &stderr); code != 0 || stdout.Len() == 0 || stderr.Len() != 0 {
		t.Fatalf("code/stdout/stderr = %d/%q/%q", code, stdout.String(), stderr.String())
	}
}

func TestOutgoingConnectsSourceToTarget(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[Target]]", "Target.md": ""})
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"outgoing", "--root", directory, "Source"}, &stdout, &stderr); code != 0 || stdout.String() != "Target.md\n" || stderr.Len() != 0 {
		t.Fatalf("code/stdout/stderr = %d/%q/%q", code, stdout.String(), stderr.String())
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	want := "mdlink dev\ncommit: unknown\nbuild date: unknown\ngo: " + runtime.Version() + "\n"
	if code != 0 || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code/stdout/stderr = %d/%q/%q", code, stdout.String(), stderr.String())
	}
}

func TestVersionRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version", "extra"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("code/stdout/stderr = %d/%q/%q", code, stdout.String(), stderr.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestVersionWriteFailure(t *testing.T) {
	var stderr bytes.Buffer
	if code := Run([]string{"version"}, failingWriter{}, &stderr); code != 1 || stderr.Len() == 0 {
		t.Fatalf("code/stderr = %d/%q", code, stderr.String())
	}
}
