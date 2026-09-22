package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestGitignoreCommands(t *testing.T) {
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	dir := fixture(t, map[string]string{".gitignore": "node_modules/\nTarget.md\n", "Source.md": "[[Target]]", "Target.md": "", "node_modules/Noise.md": "[[NoiseMissing]]"})
	for _, command := range []string{"outgoing", "backlinks", "unresolved"} {
		var out, errOut bytes.Buffer
		if code := Run([]string{command, "--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "--no-gitignore") {
			t.Fatal(command, code, out.String(), errOut.String())
		}
	}
	checkRun(t, []string{"outgoing", "--root", dir, "--strict", "Source"}, 3, "", true)
	checkRun(t, []string{"outgoing", "--root", dir, "--no-gitignore", "--exclude", "node_modules", "Source"}, 0, "Target.md\n", false)
	checkRun(t, []string{"backlinks", "--root", dir, "Target"}, 2, "", true)
	checkRun(t, []string{"backlinks", "--root", dir, "--no-gitignore", "--exclude", "node_modules", "Target"}, 0, "Source.md\n", false)
	checkRun(t, []string{"unresolved", "--root", dir, "--source", "Target.md"}, 2, "", true)
	var out, errOut bytes.Buffer
	code := Run([]string{"unresolved", "--root", dir, "--format", "json", "--strict"}, &out, &errOut)
	var result struct {
		SchemaVersion int `json:"schema_version"`
		Results       []struct{ Target string }
	}
	if code != 3 || errOut.Len() != 0 || json.Unmarshal(out.Bytes(), &result) != nil || result.SchemaVersion != 1 || len(result.Results) != 1 || result.Results[0].Target != "Target" {
		t.Fatal(code, out.String(), errOut.String())
	}
	checkRun(t, []string{"unresolved", "--root", dir, "--no-gitignore", "--exclude", "node_modules", "--strict"}, 0, "", false)
}

func TestGitignoreFatalOutput(t *testing.T) {
	dir := fixture(t, map[string]string{".gitignore": "[", "Source.md": ""})
	for _, command := range []string{"outgoing", "backlinks", "unresolved"} {
		args := []string{command, "--root", dir, "--format", "json", "--strict"}
		if command != "unresolved" {
			args = append(args, "Source")
		}
		stderr := checkRun(t, args, 1, "", true)
		if !strings.Contains(stderr, "invalid-gitignore") || !strings.Contains(stderr, ".gitignore") || strings.Contains(stderr, dir) {
			t.Fatal(stderr)
		}
	}
	checkRun(t, []string{"unresolved", "--root", dir, "--no-gitignore", "--strict"}, 0, "", false)
}
