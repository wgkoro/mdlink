package query

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mdlink/internal/diagnostic"
	"mdlink/internal/root"
)

func backlinkCatalog(t *testing.T, contents map[string]string) *root.Catalog {
	t.Helper()
	directory := t.TempDir()
	for name, body := range contents {
		filename := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	files, diagnostics, err := root.Walk(directory, root.Options{})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("walk: %+v/%v", diagnostics, err)
	}
	return root.NewCatalog(files)
}

func TestBacklinksCountsResolvedTarget(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{
		"Note.md": "[[#Self]]", "Unused.md": "",
		"A.md":      "[[Note]] [markdown](Note.md) [[Note|alias]] [[Note#Heading]] ![[Note]]",
		"B.md":      "![image](/Note.md)",
		"Hidden.md": "`[[Note]]` <!-- [link](Note.md) --> %% [[Note]] %% [external](https://host/Note.md)",
		"asset.png": "[[Note]]",
	})
	if err := os.Chmod(catalog.ByExactPath["asset.png"].PhysicalPath, 0); err != nil {
		t.Fatal(err)
	}
	got, diagnostics := Backlinks(catalog, *catalog.ByExactPath["Note.md"])
	want := []Result{{Path: "A.md", Count: 5}, {Path: "B.md", Count: 1}, {Path: "Note.md", Count: 1}}
	if !reflect.DeepEqual(got, want) || len(diagnostics) != 0 {
		t.Fatalf("results/diagnostics = %+v/%+v", got, diagnostics)
	}
	got, diagnostics = Backlinks(catalog, *catalog.ByExactPath["Unused.md"])
	if len(got) != 0 || len(diagnostics) != 0 {
		t.Fatalf("empty results/diagnostics = %+v/%+v", got, diagnostics)
	}
}

func TestBacklinksKeepsTargetIdentityAndAllDiagnostics(t *testing.T) {
	body := "[[a/Note]] [other](b/Note.md) [[Note]]"
	catalog := backlinkCatalog(t, map[string]string{
		"Source.md": body, "Unrelated.md": "[[Missing]] [[../../Outside]]", "a/Note.md": "", "b/Note.md": "",
	})
	got, diagnostics := Backlinks(catalog, *catalog.ByExactPath["a/Note.md"])
	if !reflect.DeepEqual(got, []Result{{Path: "Source.md", Count: 1}}) || len(diagnostics) != 3 {
		t.Fatalf("results/diagnostics = %+v/%+v", got, diagnostics)
	}
	for i, tt := range []struct {
		code, source, raw string
		offset            int
	}{
		{"ambiguous-link", "Source.md", "Note", len("[[a/Note]] [other](b/Note.md) ")},
		{"unresolved-link", "Unrelated.md", "Missing", 0},
		{"unsafe-path", "Unrelated.md", "../../Outside", len("[[Missing]] ")},
	} {
		item := diagnostics[i]
		if item.Code != tt.code || item.Source != tt.source || item.Offset == nil || *item.Offset != tt.offset || item.RawTarget == nil || *item.RawTarget != tt.raw {
			t.Fatalf("diagnostic %d = %+v", i, item)
		}
	}
	if !reflect.DeepEqual(diagnostics[0].Candidates, []string{"a/Note.md", "b/Note.md"}) {
		t.Fatalf("candidates = %v", diagnostics[0].Candidates)
	}
}

func TestBacklinksPreservesNormalizedFileIdentity(t *testing.T) {
	source := backlinkCatalog(t, map[string]string{"Source.md": "[[A\u030a.png]] [[A\u030a.png]] [[Å.png]]"}).ByExactPath["Source.md"]
	catalog := root.NewCatalog([]root.File{*source, {LogicalPath: "A\u030a.png"}, {LogicalPath: "Å.png"}})
	for name, count := range map[string]int{"A\u030a.png": 2, "Å.png": 1} {
		got, diagnostics := Backlinks(catalog, *catalog.ByExactPath[name])
		if !reflect.DeepEqual(got, []Result{{Path: "Source.md", Count: count}}) || len(diagnostics) != 0 {
			t.Fatalf("%q: results/diagnostics = %+v/%+v", name, got, diagnostics)
		}
	}
}

func TestBacklinksContinuesAfterUnreadableSource(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{"Deleted.md": "[[Note]]", "Source.md": "[[Note]]", "Note.md": ""})
	if err := os.Remove(catalog.ByExactPath["Deleted.md"].PhysicalPath); err != nil {
		t.Fatal(err)
	}
	got, diagnostics := Backlinks(catalog, *catalog.ByExactPath["Note.md"])
	if !reflect.DeepEqual(got, []Result{{Path: "Source.md", Count: 1}}) || len(diagnostics) != 1 || diagnostics[0].Code != "unreadable-file" || diagnostics[0].Source != "Deleted.md" {
		t.Fatalf("results/diagnostics = %+v/%+v", got, diagnostics)
	}
}

func TestBacklinksPreservesSequentialResultsAndDiagnostics(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{
		"A.md":             strings.Repeat("plain text\n", 1<<16) + "[[Target]] [[Missing]] [[Same]] [[../../Outside]]",
		"B.md":             "[[Target#Heading]] [[Missing]]",
		"C.md":             "[target](Target.md) [[Missing]]",
		"D.md":             "[[Target]] [[Missing]]",
		"E.md":             "[[Target]] [[Missing]]",
		"Deleted.md":       "[[Target]]",
		"Invalid.md":       "\xff[[Target]]",
		"Target.md":        "[[#Self]]",
		"a/Same.md":        "[[Target]]",
		"b/Same.md":        "[[Target]]",
		"nested/Source.md": "[target](../Target.md) [[Missing]]",
		"日本語.md":           "[[Target]] [[Missing]]",
	})
	if err := os.Remove(catalog.ByExactPath["Deleted.md"].PhysicalPath); err != nil {
		t.Fatal(err)
	}
	target := *catalog.ByExactPath["Target.md"]
	var want []Result
	var wantDiagnostics []diagnostic.Diagnostic
	for _, source := range catalog.MarkdownFiles {
		count, observed := backlinksFromSource(catalog, *source, target.LogicalPath)
		wantDiagnostics = append(wantDiagnostics, observed...)
		if count > 0 {
			want = append(want, Result{Path: source.LogicalPath, Count: count})
		}
	}
	for range 20 {
		got, observed := Backlinks(catalog, target)
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(observed, wantDiagnostics) {
			t.Fatalf("results/diagnostics = %+v/%+v; want %+v/%+v", got, observed, want, wantDiagnostics)
		}
	}
}

func TestBacklinksEmptyAndSmallCatalog(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3, 4, 5, 17} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			contents := make(map[string]string)
			for i := range count {
				contents[fmt.Sprintf("Source%02d.md", i)] = "[[Target.png]]"
			}
			catalog := backlinkCatalog(t, contents)
			files := []root.File{{LogicalPath: "Target.png"}}
			for _, file := range catalog.MarkdownFiles {
				files = append(files, *file)
			}
			catalog = root.NewCatalog(files)
			got, diagnostics := Backlinks(catalog, *catalog.ByExactPath["Target.png"])
			if len(got) != count || len(diagnostics) != 0 {
				t.Fatalf("results/diagnostics = %+v/%+v", got, diagnostics)
			}
			for i, result := range got {
				if result.Path != fmt.Sprintf("Source%02d.md", i) || result.Count != 1 {
					t.Fatal(result)
				}
			}
		})
	}
}
