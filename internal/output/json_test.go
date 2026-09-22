package output

import (
	"bytes"
	"os"
	"testing"

	"mdlink/internal/diagnostic"
	"mdlink/internal/query"
)

func TestJSONGolden(t *testing.T) {
	want, err := os.ReadFile("testdata/outgoing.json")
	if err != nil {
		t.Fatal(err)
	}
	offset, raw := 0, "missing"
	var stdout bytes.Buffer
	err = JSON(&stdout, "outgoing", "Source.md", []query.Result{{Path: "A.md", Count: 2}}, []diagnostic.Diagnostic{
		{Code: "unresolved-link", Source: "Source.md", Offset: &offset, RawTarget: &raw},
		{Code: "skipped-symlink", Source: "alias"},
	})
	if err != nil || stdout.String() != string(want) {
		t.Fatalf("output/error = %q/%v", &stdout, err)
	}
}

func TestJSONEmptyArrays(t *testing.T) {
	var stdout bytes.Buffer
	err := JSON(&stdout, "outgoing", "Source.md", nil, nil)
	want := "{\"schema_version\":1,\"command\":\"outgoing\",\"root\":\".\",\"target\":\"Source.md\",\"results\":[],\"diagnostics\":[]}\n"
	if err != nil || stdout.String() != want {
		t.Fatalf("output/error = %q/%v", &stdout, err)
	}
}

func TestJSONRejectsInvalidUTF8PathsBeforeWriting(t *testing.T) {
	for _, location := range []string{"target", "result", "source", "candidate"} {
		t.Run(location, func(t *testing.T) {
			target := "Source.md"
			results := []query.Result{{Path: "A.md", Count: 1}}
			var diagnostics []diagnostic.Diagnostic
			invalid := "invalid\xff.md"
			switch location {
			case "target":
				target = invalid
			case "result":
				results = append(results, query.Result{Path: invalid, Count: 1})
			case "source":
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Code: "unreadable-file", Source: invalid})
			case "candidate":
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Code: "ambiguous-link", Source: target, Candidates: []string{"A.md", invalid}})
			}
			var stdout bytes.Buffer
			if err := JSON(&stdout, "outgoing", target, results, diagnostics); err == nil || stdout.Len() != 0 {
				t.Fatalf("output/error = %q/%v", &stdout, err)
			}
		})
	}
}
