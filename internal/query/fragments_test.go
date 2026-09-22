package query

import (
	"fmt"
	"mdlink/internal/diagnostic"
	"mdlink/internal/root"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestFragmentReadScopeAndCache(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{
		"Source.md": "# Self\n[[#Self]] [[Target#Alpha]] [[Target#Absent]] [[Target#Absent]] [[Other#Other]] [[image.pdf#page=2]] [x](Target.md#Alpha) [[Target#]]",
		"Target.md": "# Alpha\n[[Unscanned]]", "Other.md": "# Other\n", "image.pdf": "",
	})
	reads := map[string]int{}
	read := func(file root.File) ([]byte, *diagnostic.Diagnostic) {
		reads[file.LogicalPath]++
		return root.ReadMarkdown(file)
	}
	results, items, err := outgoing(catalog, *catalog.ByExactPath["Source.md"], true, read)
	if err != nil || len(results) != 4 || len(items) != 5 || !reflect.DeepEqual(reads, map[string]int{"Source.md": 1, "Target.md": 1, "Other.md": 1}) {
		t.Fatal(results, items, reads, err)
	}
	for _, item := range items {
		if item.Phase != "fragment" || item.Target == "" || item.RawLink == "" || item.Fragment == "" {
			t.Fatal(item)
		}
	}
	reads = map[string]int{}
	_, items, err = outgoing(catalog, *catalog.ByExactPath["Source.md"], false, read)
	if err != nil || len(items) != 0 || !reflect.DeepEqual(reads, map[string]int{"Source.md": 1}) {
		t.Fatal(items, reads, err)
	}
	reads = map[string]int{}
	_, items = unresolved(catalog, []*root.File{catalog.ByExactPath["Source.md"], catalog.ByExactPath["Source.md"]}, true, read)
	if len(items) != 10 || !reflect.DeepEqual(reads, map[string]int{"Source.md": 2, "Target.md": 2, "Other.md": 2}) {
		t.Fatal(items, reads)
	}
}

func TestFragmentReadFailures(t *testing.T) {
	for _, reason := range []string{"unreadable-file", "file-too-large", "changed-during-read", "invalid-encoding"} {
		t.Run(reason, func(t *testing.T) {
			catalog := backlinkCatalog(t, map[string]string{"Source.md": "[[Target#A]] [[Target#B]]", "Target.md": ""})
			calls := 0
			read := func(file root.File) ([]byte, *diagnostic.Diagnostic) {
				if file.LogicalPath == "Source.md" {
					return root.ReadMarkdown(file)
				}
				calls++
				if reason == "invalid-encoding" {
					return []byte{0xff}, nil
				}
				return nil, &diagnostic.Diagnostic{Code: reason, Source: file.LogicalPath}
			}
			results, items, err := outgoing(catalog, *catalog.ByExactPath["Source.md"], true, read)
			if err != nil || len(results) != 1 || results[0].Count != 2 || len(items) != 2 || calls != 1 {
				t.Fatal(results, items, err, calls)
			}
			for _, item := range items {
				if item.Code != "unverifiable-fragment" || item.Reason != reason || item.Source != "Source.md" {
					t.Fatal(item)
				}
			}
		})
	}
	// Exercise actual safe Reader failures as well as deterministic failure injection.
	for _, mode := range []string{"deleted", "large", "invalid"} {
		catalog := backlinkCatalog(t, map[string]string{"Source.md": "[[Target#A]]", "Target.md": ""})
		target := catalog.ByExactPath["Target.md"]
		switch mode {
		case "deleted":
			if err := os.Remove(target.PhysicalPath); err != nil {
				t.Fatal(err)
			}
		case "large":
			if err := os.Truncate(target.PhysicalPath, root.MaxBodySize+1); err != nil {
				t.Fatal(err)
			}
		case "invalid":
			if err := os.WriteFile(target.PhysicalPath, []byte{0xff}, 0600); err != nil {
				t.Fatal(err)
			}
		}
		_, items, err := Outgoing(catalog, *catalog.ByExactPath["Source.md"], true)
		if err != nil || len(items) != 1 || items[0].Code != "unverifiable-fragment" {
			t.Fatal(mode, items, err)
		}
	}
}

func TestFileFailuresDoNotCheckFragments(t *testing.T) {
	catalog := backlinkCatalog(t, map[string]string{"Source.md": "[[Missing#A]] [[Same#A]] [[../unsafe#A]]", "a/Same.md": "", "b/Same.md": ""})
	reads := 0
	_, items, err := outgoing(catalog, *catalog.ByExactPath["Source.md"], true, func(file root.File) ([]byte, *diagnostic.Diagnostic) { reads++; return root.ReadMarkdown(file) })
	if err != nil || reads != 1 || len(items) != 3 {
		t.Fatal(items, reads, err)
	}
	for _, item := range items {
		if item.Phase == "fragment" {
			t.Fatal(items)
		}
	}
}

func BenchmarkRepeatedFragmentTarget(b *testing.B) {
	directory := b.TempDir()
	source := root.File{LogicalPath: "Source.md", PhysicalPath: directory + "/Source.md"}
	target := root.File{LogicalPath: "Target.md", PhysicalPath: directory + "/Target.md"}
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			body := []byte(strings.Repeat("[[Target#Alpha]] ", n))
			catalog := root.NewCatalog([]root.File{source, target})
			b.ReportAllocs()
			for b.Loop() {
				reads := 0
				_, _, err := outgoing(catalog, source, true, func(file root.File) ([]byte, *diagnostic.Diagnostic) {
					if file.LogicalPath == "Source.md" {
						return body, nil
					}
					reads++
					return []byte("# Alpha\n"), nil
				})
				if err != nil || reads != 1 {
					b.Fatal(err, reads)
				}
			}
		})
	}
}
