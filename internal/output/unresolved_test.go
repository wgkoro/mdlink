package output

import (
	"bytes"
	"mdlink/internal/diagnostic"
	"mdlink/internal/query"
	"strings"
	"testing"
)

func TestUnresolvedJSONAndText(t *testing.T) {
	results := []query.UnresolvedResult{{Target: "Missing", Count: 2, Sources: []query.Occurrence{{Path: "A.md", Offset: 0}, {Path: "A.md", Offset: 12}}}}
	var out bytes.Buffer
	if err := UnresolvedJSON(&out, results, nil, nil); err != nil {
		t.Fatal(err)
	}
	want := "{\"schema_version\":1,\"command\":\"unresolved\",\"root\":\".\",\"results\":[{\"target\":\"Missing\",\"count\":2,\"sources\":[{\"path\":\"A.md\",\"offset\":0},{\"path\":\"A.md\",\"offset\":12}]}],\"diagnostics\":[]}\n"
	if out.String() != want {
		t.Fatal(out.String())
	}
	out.Reset()
	if err := UnresolvedJSON(&out, nil, nil, nil); err != nil || out.String() != "{\"schema_version\":1,\"command\":\"unresolved\",\"root\":\".\",\"results\":[],\"diagnostics\":[]}\n" {
		t.Fatal(out.String(), err)
	}
	for _, counts := range []bool{false, true} {
		out.Reset()
		want := "Missing\n"
		if counts {
			want = "2\t" + want
		}
		if err := UnresolvedText(&out, results, nil, counts); err != nil || out.String() != want {
			t.Fatal(out.String(), err)
		}
	}
}

func TestUnresolvedOutputValidatesBeforeWriting(t *testing.T) {
	for _, invalid := range []string{"bad\rname", "bad\nname", "bad\tname", "bad\xffname"} {
		for _, where := range []string{"target", "occurrence", "source", "candidate"} {
			results := []query.UnresolvedResult{{Target: "Good", Count: 1, Sources: []query.Occurrence{{Path: "A.md", Offset: 0}}}}
			diagnostics := []diagnostic.Diagnostic{{Code: "ambiguous-link", Source: "A.md", Candidates: []string{"B.md"}}}
			switch where {
			case "target":
				results = append(results, query.UnresolvedResult{Target: invalid})
			case "occurrence":
				results[0].Sources[0].Path = invalid
			case "source":
				diagnostics[0].Source = invalid
			case "candidate":
				diagnostics[0].Candidates[0] = invalid
			}
			var out bytes.Buffer
			if strings.Contains(invalid, "\xff") {
				if err := UnresolvedJSON(&out, results, diagnostics, nil); err == nil || out.Len() != 0 {
					t.Fatalf("%s: %q/%v", where, &out, err)
				}
			} else {
				if err := UnresolvedText(&out, results, diagnostics, false); err != ErrTextPath || out.Len() != 0 {
					t.Fatalf("%s: %q/%v", where, &out, err)
				}
				if err := UnresolvedJSON(&out, results, diagnostics, nil); err != nil || strings.Count(out.String(), "\n") != 1 {
					t.Fatal(out.String(), err)
				}
			}
		}
	}
}
