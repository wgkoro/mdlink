package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"mdlink/internal/diagnostic"
)

func TestDiagnosticOrder(t *testing.T) {
	zero, one := 0, 1
	a, b := "a", "b"
	want := []diagnostic.Diagnostic{
		{Source: "A.md", Code: "z"},
		{Source: "A.md", Code: "a", Offset: &zero, RawTarget: &a},
		{Source: "A.md", Code: "b", Offset: &zero, RawTarget: &a},
		{Source: "A.md", Code: "b", Offset: &zero, RawTarget: &b, Candidates: []string{"A.md"}},
		{Source: "A.md", Code: "b", Offset: &zero, RawTarget: &b, Candidates: []string{"Z.md"}},
		{Source: "A.md", Code: "a", Offset: &one, RawTarget: &a},
		{Source: "Z.md", Code: "a"},
		{Source: "A\u030a.md", Code: "a"},
		{Source: "Å.md", Code: "a"},
	}
	got := slices.Clone(want)
	slices.Reverse(got)
	var stdout bytes.Buffer
	if err := JSON(&stdout, "outgoing", "Source.md", nil, got); err != nil {
		t.Fatal(err)
	}
	var response struct{ Diagnostics []diagnostic.Diagnostic }
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.Diagnostics, want) {
		t.Fatalf("diagnostics = %+v", response.Diagnostics)
	}
}

func TestDiagnosticsTextLimitAndDetails(t *testing.T) {
	for _, count := range []int{0, 100, 101} {
		items := make([]diagnostic.Diagnostic, count)
		for i := range items {
			items[i] = diagnostic.Diagnostic{Code: "unresolved-link", Source: fmt.Sprintf("%03d.md", i)}
		}
		slices.Reverse(items)
		var stderr bytes.Buffer
		if err := Diagnostics(&stderr, items); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			if stderr.Len() != 0 {
				t.Fatalf("zero diagnostics: %q", &stderr)
			}
			continue
		}
		if strings.Count(stderr.String(), "unresolved-link:") != min(count, 100) || !strings.HasPrefix(stderr.String(), "unresolved-link: \"000.md\"") || !strings.HasSuffix(stderr.String(), fmt.Sprintf("diagnostics: %d\n", count)) {
			t.Fatalf("diagnostics = %q", &stderr)
		}
	}
	offset, raw := 0, "bad\nname"
	var stderr bytes.Buffer
	err := Diagnostics(&stderr, []diagnostic.Diagnostic{{Code: "ambiguous-link", Source: "Source.md", Offset: &offset, RawTarget: &raw, Candidates: []string{"a/Name.md", "b/Name.md"}}})
	want := "ambiguous-link: \"Source.md\" offset=0 raw_target=\"bad\\nname\" candidates=[\"a/Name.md\" \"b/Name.md\"]\ndiagnostics: 1\n"
	if err != nil || stderr.String() != want {
		t.Fatalf("output/error = %q/%v", &stderr, err)
	}
}

func TestDiagnosticPhases(t *testing.T) {
	items := []diagnostic.Diagnostic{{Code: "unreadable-file", Source: "A.md", Phase: "source"}, {Code: "unreadable-file", Source: "A.md", Phase: "catalog"}}
	var out bytes.Buffer
	if err := UnresolvedJSON(&out, nil, items, []string{"A.md"}); err != nil {
		t.Fatal(err)
	}
	var response struct{ Diagnostics []diagnostic.Diagnostic }
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.Diagnostics[0].Phase != "catalog" || response.Diagnostics[1].Phase != "source" {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if err := Diagnostics(&out, items); err != nil || out.String() != "unreadable-file: \"A.md\" phase=catalog\nunreadable-file: \"A.md\" phase=source\ndiagnostics: 2\n" {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if err := UnresolvedJSON(&out, nil, nil, []string{"bad\xff.md"}); err == nil || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
}

func TestFragmentDiagnosticDetailsAndOrder(t *testing.T) {
	zero := 0
	raw := "Target"
	base := diagnostic.Diagnostic{Code: "missing-fragment", Source: "Source.md", Offset: &zero, RawTarget: &raw, Phase: "fragment", Fragment: "#a", Target: "Target.md", RawLink: "[[Target#a|display\tname]]"}
	second := base
	second.Fragment = "#b"
	third := second
	third.Target = "Z.md"
	fourth := third
	fourth.Reason = "reason"
	fifth := fourth
	fifth.RawLink = "zzz"
	items := []diagnostic.Diagnostic{fifth, fourth, third, second, base}
	var out bytes.Buffer
	if err := JSON(&out, "outgoing", "Source.md", nil, items); err != nil {
		t.Fatal(err)
	}
	var response struct{ Diagnostics []diagnostic.Diagnostic }
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || !reflect.DeepEqual(response.Diagnostics, []diagnostic.Diagnostic{base, second, third, fourth, fifth}) {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if err := Diagnostics(&out, []diagnostic.Diagnostic{base}); err != nil || !strings.Contains(out.String(), `raw_link="[[Target#a|display\tname]]" fragment="#a" target="Target.md"`) {
		t.Fatal(err, out.String())
	}
	out.Reset()
	base.Target = "bad\xff.md"
	if err := JSON(&out, "outgoing", "Source.md", nil, []diagnostic.Diagnostic{base}); err == nil || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if err := UnresolvedJSON(&out, nil, []diagnostic.Diagnostic{base}, nil); err == nil || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
}
