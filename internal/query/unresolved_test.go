package query

import (
	"os"
	"reflect"
	"testing"

	"mdlink/internal/root"
)

func TestUnresolvedOccurrencesAndDiagnostics(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{
		"a/Source.md": "日本 [[Missing]] [[Missing#H|label]] ![[Missing]] [x](Missing) [[./missing.md]] [[Missing.md]] [[M%69ssing]] [[é]] [[e\u0301]] [[Shared]] [[../../bad]] [[Known]] `[[Hidden]]` <!-- [[Hidden]] --> [x](https://host)",
		"b/Source.md": "[[Missing]] [[./missing.md]]", "Known.md": "", "a/Shared.md": "", "b/Shared.md": "",
	})
	got, diagnostics := Unresolved(catalog, nil, false)
	wantTargets := []string{"./missing.md", "M%69ssing", "Missing", "Missing.md", "e\u0301", "é"}
	var targets []string
	for _, result := range got {
		targets = append(targets, result.Target)
		if result.Count != len(result.Sources) {
			t.Fatal(result)
		}
	}
	if !reflect.DeepEqual(targets, wantTargets) || len(diagnostics) != 13 {
		t.Fatalf("results/diagnostics = %+v/%+v", got, diagnostics)
	}
	want := []Occurrence{{"a/Source.md", len("日本 ")}, {"a/Source.md", len("日本 [[Missing]] ")}, {"a/Source.md", len("日本 [[Missing]] [[Missing#H|label]] ")}, {"a/Source.md", len("日本 [[Missing]] [[Missing#H|label]] ![[Missing]] ")}, {"b/Source.md", 0}}
	if !reflect.DeepEqual(got[2].Sources, want) {
		t.Fatalf("sources = %+v", got[2].Sources)
	}
	if got[0].Count != 2 {
		t.Fatal(got[0])
	}
}

func TestUnresolvedContinuesAfterSourceFailures(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{"Deleted.md": "", "Invalid.md": "\xff", "Large.md": "", "Source.md": "[[Missing]]"})
	if err := os.Remove(catalog.ByExactPath["Deleted.md"].PhysicalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(catalog.ByExactPath["Large.md"].PhysicalPath, root.MaxBodySize+1); err != nil {
		t.Fatal(err)
	}
	got, diagnostics := Unresolved(catalog, nil, false)
	if len(got) != 1 || got[0].Target != "Missing" || len(diagnostics) != 4 {
		t.Fatalf("%+v/%+v", got, diagnostics)
	}
	for i, code := range []string{"unreadable-file", "invalid-encoding", "file-too-large", "unresolved-link"} {
		if diagnostics[i].Code != code {
			t.Fatal(diagnostics)
		}
	}
	got, diagnostics = Unresolved(root.NewCatalog(nil), nil, false)
	if len(got) != 0 || len(diagnostics) != 0 {
		t.Fatal(got, diagnostics)
	}
}

func TestUnresolvedRawIdentityAndSourceOrder(t *testing.T) {
	source := backlinkCatalog(t, map[string]string{"Source.md": "[[missing]] [[Missing]] [[Missing.md]] [[M%69ssing]] [[é]] [[e\u0301]] [[Missing]]"}).ByExactPath["Source.md"]
	var files []root.File
	for _, name := range []string{"é-source.md", "e\u0301-source.md", "Z.md"} {
		file := *source
		file.LogicalPath = name
		files = append(files, file)
	}
	catalog := root.NewCatalog(files)
	// Reverse traversal so result ordering cannot rely on Catalog's iteration order.
	for i, j := 0, len(catalog.MarkdownFiles)-1; i < j; i, j = i+1, j-1 {
		catalog.MarkdownFiles[i], catalog.MarkdownFiles[j] = catalog.MarkdownFiles[j], catalog.MarkdownFiles[i]
	}
	got, _ := Unresolved(catalog, nil, false)
	var targets []string
	for _, result := range got {
		targets = append(targets, result.Target)
	}
	if !reflect.DeepEqual(targets, []string{"M%69ssing", "Missing", "Missing.md", "missing", "e\u0301", "é"}) {
		t.Fatal(targets)
	}
	last := len("[[missing]] [[Missing]] [[Missing.md]] [[M%69ssing]] [[é]] [[e\u0301]] ")
	want := []Occurrence{{"Z.md", 12}, {"Z.md", last}, {"e\u0301-source.md", 12}, {"e\u0301-source.md", last}, {"é-source.md", 12}, {"é-source.md", last}}
	if !reflect.DeepEqual(got[1].Sources, want) {
		t.Fatal(got[1].Sources, want)
	}
}

func TestUnresolvedSelectedReadsOnlySources(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{"A.md": "[[Target]] [[Missing]]", "Ignored.md": "", "Target.md": ""})
	for _, name := range []string{"Ignored.md", "Target.md"} {
		if err := os.Remove(catalog.ByExactPath[name].PhysicalPath); err != nil {
			t.Fatal(err)
		}
	}
	got, diagnostics := Unresolved(catalog, []*root.File{catalog.ByExactPath["A.md"]}, false)
	if len(got) != 1 || got[0].Target != "Missing" || len(diagnostics) != 1 || diagnostics[0].Code != "unresolved-link" {
		t.Fatal(got, diagnostics)
	}
	got, diagnostics = Unresolved(catalog, []*root.File{}, false)
	if len(got) != 0 || len(diagnostics) != 0 {
		t.Fatal(got, diagnostics)
	}
}
