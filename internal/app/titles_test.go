package app

import (
	"bytes"
	"encoding/json"
	"mdlink/internal/diagnostic"
	"strings"
	"testing"
)

func TestTitleCommands(t *testing.T) {
	body := `日本 [label](Target.md "title") ![alt](<Target.md> 'image') [missing](Missing.md "title")`
	dir := fixture(t, map[string]string{"Source.md": body, "Target.md": ""})
	for _, test := range []struct {
		command, target string
		count           int
	}{{"outgoing", "Source", 2}, {"backlinks", "Target", 2}, {"unresolved", "", 1}} {
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
		if code != 3 || json.Unmarshal(out.Bytes(), &value) != nil || len(value.Results) != 1 || value.Results[0].Count != test.count || len(value.Diagnostics) != 1 || *value.Diagnostics[0].Offset != strings.Index(body, "[missing]") || *value.Diagnostics[0].RawTarget != "Missing.md" {
			t.Fatal(code, out.String(), errOut.String())
		}
	}
	var out, errOut bytes.Buffer
	directory := fixture(t, map[string]string{"Source.md": `![alt](Target.md#Absent "literal ) (")`, "Target.md": "# Real"})
	if code := Run([]string{"outgoing", "--root", directory, "--check-fragments", "--format", "json", "Source"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), `"raw_link":"![alt](Target.md#Absent \"literal ) (\")"`) || !strings.Contains(out.String(), `"fragment":"#Absent"`) {
		t.Fatal(code, out.String(), errOut.String())
	}
}
