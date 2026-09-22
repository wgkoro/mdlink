package app

import (
	"strings"
	"testing"
)

func TestMdlinkignoreCommands(t *testing.T) {
	t.Setenv("MDLINK_FOLLOW_SYMLINKS", "")
	dir := fixture(t, map[string]string{".mdlinkignore": "archive/\nTarget.md\n", "Source.md": "[[Target]]", "Target.md": "[[Source]]", "archive/Noise.md": "[[Source]] [[NoiseMissing]]"})
	for _, options := range [][]string{nil, {"--no-gitignore"}} {
		args := append([]string{"--root", dir}, options...)
		checkRun(t, append(append([]string{"outgoing"}, args...), "--strict", "Source"), 3, "", true)
		checkRun(t, append(append([]string{"outgoing"}, args...), "Target"), 2, "", true)
		checkRun(t, append(append([]string{"backlinks"}, args...), "Target"), 2, "", true)
		checkRun(t, append(append([]string{"backlinks"}, args...), "Source"), 0, "", true)
		checkRun(t, append(append([]string{"unresolved"}, args...), "--source", "Target.md"), 2, "", true)
		checkRun(t, append([]string{"unresolved"}, args...), 0, "Target\n", true)
		checkRun(t, append(append([]string{"unresolved"}, args...), "--strict"), 3, "Target\n", true)
	}
}

func TestMdlinkignoreFatalOutput(t *testing.T) {
	dir := fixture(t, map[string]string{".mdlinkignore": "[", "Source.md": ""})
	for _, command := range []string{"outgoing", "backlinks", "unresolved"} {
		args := []string{command, "--root", dir, "--no-gitignore", "--format", "json", "--strict"}
		if command != "unresolved" {
			args = append(args, "Source")
		}
		stderr := checkRun(t, args, 1, "", true)
		if !strings.Contains(stderr, "invalid-mdlinkignore") || !strings.Contains(stderr, ".mdlinkignore") || strings.Contains(stderr, dir) {
			t.Fatal(stderr)
		}
	}
}
