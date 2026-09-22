package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBacklinksCLI(t *testing.T) {
	directory := fixture(t, map[string]string{
		"Target.md": "[[#Self]]", "A.md": "[[Target|alias]] [link](Target.md#Heading)",
		"B.md": "![link](/Target.md)", "Empty.md": "",
		"Attachment.md": "![asset](asset.png)", "asset.png": "\xff[[Target]]",
	})
	checkRun(t, []string{"backlinks", "--root", directory, "Target"}, 0, "A.md\nB.md\nTarget.md\n", false)
	checkRun(t, []string{"backlinks", "--root", directory, "Empty"}, 0, "", false)
	checkRun(t, []string{"backlinks", "--root", directory, "asset.png"}, 0, "Attachment.md\n", false)
}

func TestBacklinksRejectsInvalidTarget(t *testing.T) {
	directory := fixture(t, map[string]string{"a/Note.md": "", "b/Note.md": "", "asset.png": ""})
	for _, tt := range []struct{ target, status string }{{"Note", "ambiguous"}, {"Missing", "unresolved"}, {"../Note.md", "unsafe"}, {"/Note.md", "unsafe"}, {"asset", "unresolved"}} {
		message := checkRun(t, []string{"backlinks", "--root", directory, tt.target}, 2, "", true)
		if !strings.Contains(message, "invalid target: "+tt.status) {
			t.Fatalf("%q: stderr = %q", tt.target, message)
		}
	}
}

func TestQueryJSONCommand(t *testing.T) {
	directory := fixture(t, map[string]string{"Source.md": "[[Target]]", "Target.md": ""})
	for _, command := range []string{"outgoing", "backlinks"} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{command, "--root", directory, "--format", "json", "Target"}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("%s: code/stderr = %d/%q", command, code, stderr.String())
		}
		var response struct {
			Command string `json:"command"`
			Target  string `json:"target"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Command != command || response.Target != "Target.md" {
			t.Fatalf("%s: response = %+v", command, response)
		}
	}
}
