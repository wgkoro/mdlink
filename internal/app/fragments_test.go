package app

import (
	"bytes"
	"encoding/json"
	"mdlink/internal/diagnostic"
	"strings"
	"testing"
)

func TestFragmentCLI(t *testing.T) {
	dir := fixture(t, map[string]string{"Source.md": "日本 ![[Target#Absent|表示]] [[Target#Alpha]] [[Missing#No]]", "Target.md": "# Alpha\n[[DoNotScan]]"})
	for _, command := range []string{"outgoing", "unresolved"} {
		args := []string{command, "--root", dir, "--check-fragments", "--strict", "--format", "json"}
		if command == "outgoing" {
			args = append(args, "Source")
		} else {
			args = append(args, "--source", "Source.md")
		}
		var out, errOut bytes.Buffer
		code := Run(args, &out, &errOut)
		var value struct {
			Diagnostics []diagnostic.Diagnostic
			Results     []struct {
				Path, Target string
				Count        int
			}
		}
		if code != 3 || json.Unmarshal(out.Bytes(), &value) != nil || len(value.Diagnostics) != 2 || errOut.Len() != 0 {
			t.Fatal(code, out.String(), errOut.String())
		}
		if value.Diagnostics[0].Code != "missing-fragment" || value.Diagnostics[0].Phase != "fragment" || value.Diagnostics[0].RawLink != "![[Target#Absent|表示]]" || *value.Diagnostics[0].Offset != len("日本 ") {
			t.Fatal(value.Diagnostics)
		}
		if command == "outgoing" && (len(value.Results) != 1 || value.Results[0].Path != "Target.md" || value.Results[0].Count != 2) {
			t.Fatal(value.Results)
		}
		if command == "unresolved" && (len(value.Results) != 1 || value.Results[0].Target != "Missing") {
			t.Fatal(value.Results)
		}
		if strings.Contains(out.String(), "DoNotScan") {
			t.Fatal(out.String())
		}
	}
	checkRun(t, []string{"backlinks", "--root", dir, "--check-fragments", "Target"}, 2, "", true)
}

func TestFragmentStrictTextAndFailurePriority(t *testing.T) {
	dir := fixture(t, map[string]string{"Source.md": "[[Target#Absent]]", "Target.md": "# Alpha"})
	var out, errOut bytes.Buffer
	args := []string{"outgoing", "--root", dir, "--check-fragments", "--strict", "Source"}
	if code := Run(args, &out, &errOut); code != 3 || out.String() != "Target.md\n" || !strings.Contains(errOut.String(), `phase=fragment`) || !strings.Contains(errOut.String(), `fragment="#Absent"`) {
		t.Fatal(code, out.String(), errOut.String())
	}
	for _, format := range []string{"text", "json"} {
		args := []string{"outgoing", "--root", dir, "--check-fragments", "--strict", "--format", format, "Source"}
		if code := Run(args, failingWriter{}, &errOut); code != 1 {
			t.Fatal(code)
		}
	}
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"outgoing", "--root", dir, "--check-fragments=false", "--strict", "--format", "json", "Source"}, &out, &errOut); code != 0 || strings.Contains(out.String(), `"phase"`) || strings.Contains(out.String(), `"raw_link"`) {
		t.Fatal(code, out.String(), errOut.String())
	}
	good := fixture(t, map[string]string{"Source.md": "[[Target#Alpha]]", "Target.md": "# Alpha"})
	checkRun(t, []string{"outgoing", "--root", good, "--check-fragments", "--strict", "Source"}, 0, "Target.md\n", false)
}

func TestFragmentHelpLimit(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"outgoing", "--help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "limited Wikilink") || !strings.Contains(out.String(), "Markdown anchors are unsupported") {
		t.Fatal(code, out.String(), errOut.String())
	}
}
