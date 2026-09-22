package root

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatalogExactFiles(t *testing.T) {
	files := []File{{LogicalPath: "note.md", PhysicalPath: "/example/note.md"}, {LogicalPath: "image.png", PhysicalPath: "/example/image.png"}}
	catalog := NewCatalog(files)
	for _, file := range files {
		got := catalog.ByExactPath[file.LogicalPath]
		if got == nil || *got != file {
			t.Errorf("exact %q = %v", file.LogicalPath, got)
		}
	}
	if len(catalog.MarkdownFiles) != 1 || catalog.MarkdownFiles[0].LogicalPath != "note.md" {
		t.Fatalf("MarkdownFiles = %v", catalog.MarkdownFiles)
	}
}

func TestCatalogKeepsDuplicateNames(t *testing.T) {
	catalog := NewCatalog([]File{{LogicalPath: "a/Note.md"}, {LogicalPath: "b/Note.md"}, {LogicalPath: "a/image.png"}, {LogicalPath: "b/image.png"}})
	if len(catalog.ByBaseName["Note.md"]) != 2 || len(catalog.ByStem["Note"]) != 2 || len(catalog.ByBaseName["image.png"]) != 2 || len(catalog.ByStem["image"]) != 0 {
		t.Fatalf("basename=%v stem=%v", catalog.ByBaseName, catalog.ByStem)
	}
}

func TestCatalogNormalizedPathsPreserveSpelling(t *testing.T) {
	catalog := NewCatalog([]File{{LogicalPath: "é.md"}, {LogicalPath: "e\u0301.md"}})
	if len(catalog.ByNormalizedPath["é.md"]) != 2 {
		t.Fatalf("NFC candidates = %v", catalog.ByNormalizedPath)
	}
	for _, name := range []string{"é.md", "e\u0301.md"} {
		if catalog.ByExactPath[name] == nil || catalog.ByExactPath[name].LogicalPath != name || len(catalog.ByBaseName[name]) != 1 {
			t.Errorf("original spelling lost: %q", name)
		}
	}
}

func TestCatalogNormalizedNamesPreserveFiles(t *testing.T) {
	files := []File{
		{LogicalPath: "a/é.md", PhysicalPath: "/physical/note", physicalIdentity: "file-id", rootPath: "/physical", rootIdentity: "root-id", relativePath: "note"},
		{LogicalPath: "b/e\u0301.md"}, {LogicalPath: "c/é.md"},
		{LogicalPath: "image/e\u0301.png"}, {LogicalPath: "upper/e\u0301.MD"}, {LogicalPath: "plain/e\u0301"},
	}
	catalog := NewCatalog(files)
	wantBase := map[string][]string{
		"é.md":  {"a/é.md", "b/e\u0301.md", "c/é.md"},
		"é.png": {"image/e\u0301.png"}, "é.MD": {"upper/e\u0301.MD"}, "é": {"plain/e\u0301"},
	}
	wantStem := map[string][]string{"é": {"a/é.md", "b/e\u0301.md", "c/é.md"}}
	for _, test := range []struct {
		index map[string][]*File
		want  map[string][]string
	}{{catalog.ByNormalizedBaseName, wantBase}, {catalog.ByNormalizedStem, wantStem}} {
		got := make(map[string][]string)
		for key, candidates := range test.index {
			for _, file := range candidates {
				got[key] = append(got[key], file.LogicalPath)
				if file != catalog.ByExactPath[file.LogicalPath] {
					t.Fatalf("index replaced File pointer: %q", file.LogicalPath)
				}
			}
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("normalized names = %v; want %v", got, test.want)
		}
	}
	for _, file := range files {
		if *catalog.ByExactPath[file.LogicalPath] != file {
			t.Fatalf("File metadata changed: %q", file.LogicalPath)
		}
	}
	slices.Reverse(files)
	if !reflect.DeepEqual(catalog, NewCatalog(files)) {
		t.Fatal("normalized names depend on input order")
	}
}

func TestCatalogOrderIndependentOfInput(t *testing.T) {
	files := []File{{LogicalPath: "é.md"}, {LogicalPath: "z.md"}, {LogicalPath: "e\u0301.md"}, {LogicalPath: "b/Note.md"}, {LogicalPath: "a/Note.md"}}
	forward := NewCatalog(files)
	slices.Reverse(files)
	backward := NewCatalog(files)
	if !reflect.DeepEqual(forward, backward) {
		t.Fatal("catalog order depends on input")
	}
	want := []string{"a/Note.md", "b/Note.md", "z.md", "e\u0301.md", "é.md"}
	var got []string
	for _, file := range forward.MarkdownFiles {
		got = append(got, file.LogicalPath)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NFC order = %q; want %q", got, want)
	}
}

func TestCatalogUppercaseExtensionIsAttachment(t *testing.T) {
	catalog := NewCatalog([]File{{LogicalPath: "Note.MD"}})
	if len(catalog.MarkdownFiles) != 0 || len(catalog.ByStem) != 0 || len(catalog.ByBaseName["Note.MD"]) != 1 || len(catalog.ByNormalizedPath["Note.MD"]) != 1 || catalog.ByExactPath["Note.MD"] == nil {
		t.Fatalf("uppercase extension must remain attachment: %+v", catalog)
	}
}
