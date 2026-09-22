package query

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mdlink/internal/root"
)

func TestOutgoingAggregatesInLogicalOrder(t *testing.T) {
	for _, body := range []string{
		"[[Z]] [[A\u030a]] [[Å]] [[Z|alias]] [[Z#Heading]] ![[Z]]",
		"![[Z]] [[Å]] [[Z#Heading]] [[Z|alias]] [[A\u030a]] [[Z]]",
	} {
		directory := t.TempDir()
		if err := os.WriteFile(filepath.Join(directory, "Source.md"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		files, _, err := root.Walk(directory, root.Options{})
		if err != nil {
			t.Fatal(err)
		}
		// Only source is read. Synthetic targets preserve names that some filesystems conflate.
		for _, name := range []string{"Å.md", "Z.md", "A\u030a.md"} {
			files = append(files, root.File{LogicalPath: name})
		}
		catalog := root.NewCatalog(files)
		got, diagnostics, err := Outgoing(catalog, *catalog.ByExactPath["Source.md"], false)
		want := []Result{{Path: "Z.md", Count: 4}, {Path: "A\u030a.md", Count: 1}, {Path: "Å.md", Count: 1}}
		if err != nil || len(diagnostics) != 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("results/diagnostics/error = %+v/%+v/%v", got, diagnostics, err)
		}
	}
}

func TestOutgoingExcludesExternalAndResolvesReferenceLinks(t *testing.T) {
	directory := t.TempDir()
	body := "[url](https://host/Note.md) ![image](file:Trap.md) [encoded](%66ile:Trap.md) ![network](%2F%2Fhost/Note.md)\n[label][id]\n[id]: Note.md"
	if err := os.WriteFile(filepath.Join(directory, "Source.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	files, _, err := root.Walk(directory, root.Options{})
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, root.File{LogicalPath: "file:Trap.md"}, root.File{LogicalPath: "Note.md"})
	catalog := root.NewCatalog(files)
	results, diagnostics, err := Outgoing(catalog, *catalog.ByExactPath["Source.md"], false)
	if err != nil || len(results) != 1 || results[0].Path != "Note.md" || results[0].Count != 1 || len(diagnostics) != 0 {
		t.Fatalf("results/diagnostics/error = %+v/%+v/%v", results, diagnostics, err)
	}
}
