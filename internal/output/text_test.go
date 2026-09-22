package output

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"mdlink/internal/diagnostic"
	"mdlink/internal/query"
)

func TestTextGolden(t *testing.T) {
	want, err := os.ReadFile("testdata/outgoing.txt")
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err = Text(&stdout, "Source.md", []query.Result{{Path: "A.md", Count: 2}, {Path: "Z.md", Count: 1}}, nil, false)
	if err != nil || stdout.String() != string(want) {
		t.Fatalf("output/error = %q/%v", &stdout, err)
	}
}

func TestTextRejectsControlPathsBeforeWriting(t *testing.T) {
	for _, control := range []string{"\r", "\n", "\t"} {
		for _, location := range []string{"target", "result", "source", "candidate"} {
			t.Run(location+control, func(t *testing.T) {
				target := "Source.md"
				results := []query.Result{{Path: "A.md", Count: 1}}
				var diagnostics []diagnostic.Diagnostic
				invalid := "name" + control + ".md"
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
				for _, counts := range []bool{false, true} {
					var stdout bytes.Buffer
					if err := Text(&stdout, target, results, diagnostics, counts); !errors.Is(err, ErrTextPath) || stdout.Len() != 0 {
						t.Fatalf("counts/output/error = %v/%q/%v", counts, &stdout, err)
					}
				}
			})
		}
	}
}
