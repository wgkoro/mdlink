package app

import (
	"bytes"
	"encoding/json"
	"mdlink/internal/diagnostic"
	"strings"
	"testing"
)

func TestReferenceCommandsAndSourceScope(t *testing.T) {
	body := "日本 [text][id] ![id][] [id] [missing][bad] [unsafe][escape]\n[id]: Target.md\n[bad]: Missing.md\n[escape]: %2E%2E/outside.md\n"
	dir := fixture(t, map[string]string{"Source.md": body, "Other.md": "[id]\n", "Target.md": ""})
	for _, test := range []struct {
		command, target string
		count           int
	}{{"outgoing", "Source", 3}, {"backlinks", "Target", 3}, {"unresolved", "", 1}} {
		args := []string{test.command, "--root", dir, "--format", "json", "--strict"}
		if test.target != "" {
			args = append(args, test.target)
		}
		var out, errOut bytes.Buffer
		code := Run(args, &out, &errOut)
		var value struct {
			Results     []struct{ Count int }
			Diagnostics []diagnostic.Diagnostic
		}
		if code != 3 || json.Unmarshal(out.Bytes(), &value) != nil || len(value.Results) != 1 || value.Results[0].Count != test.count || len(value.Diagnostics) != 2 {
			t.Fatal(code, out.String(), errOut.String())
		}
		if *value.Diagnostics[0].Offset != strings.Index(body, "[missing]") || *value.Diagnostics[0].RawTarget != "Missing.md" || value.Diagnostics[1].Code != "unsafe-path" || *value.Diagnostics[1].RawTarget != "%2E%2E/outside.md" {
			t.Fatal(value.Diagnostics)
		}
	}
}

func TestReferenceFragmentDiagnosticUsage(t *testing.T) {
	body := "日本 ![表示][id]\n[id]: Target.md#Absent \"title\"\n"
	dir := fixture(t, map[string]string{"Source.md": body, "Target.md": "# Real\n"})
	var out, errOut bytes.Buffer
	if code := Run([]string{"unresolved", "--root", dir, "--source", "Source.md", "--check-fragments", "--strict", "--format", "json"}, &out, &errOut); code != 3 {
		t.Fatal(code, out.String(), errOut.String())
	}
	var value struct {
		Results     []any
		Diagnostics []diagnostic.Diagnostic
	}
	if err := json.Unmarshal(out.Bytes(), &value); err != nil || len(value.Results) != 0 || len(value.Diagnostics) != 1 {
		t.Fatal(err, out.String())
	}
	item := value.Diagnostics[0]
	if item.RawLink != "![表示][id]" || item.Fragment != "#Absent" || item.Code != "unsupported-fragment" || item.Reason != "syntax" || item.Target != "Target.md" || *item.RawTarget != "Target.md" || *item.Offset != len("日本 ") {
		t.Fatal(item)
	}
}

func TestReferenceRawTargetAggregation(t *testing.T) {
	dir := fixture(t, map[string]string{"A.md": "[x][id]\n[id]: Missing.md\n", "B.md": "[x][id]\n[id]: Missing.md\n", "C.md": "[x][id]\n[id]: M%69ssing.md\n"})
	var out, errOut bytes.Buffer
	if code := Run([]string{"unresolved", "--root", dir, "--format", "json"}, &out, &errOut); code != 0 {
		t.Fatal(code)
	}
	var value struct {
		Results []struct {
			Target string
			Count  int
		}
	}
	if err := json.Unmarshal(out.Bytes(), &value); err != nil || len(value.Results) != 2 || value.Results[0].Target != "M%69ssing.md" || value.Results[0].Count != 1 || value.Results[1].Target != "Missing.md" || value.Results[1].Count != 2 {
		t.Fatal(err, out.String())
	}
}
