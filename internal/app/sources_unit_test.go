package app

import (
	"mdlink/internal/root"
	"reflect"
	"strings"
	"testing"
)

func TestSelectSourcesExactUnicodeAndBackslash(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "é.md"}, {LogicalPath: "e\u0301.md"}, {LogicalPath: `a\b.md`}})
	for _, names := range [][]string{{"é.md"}, {"e\u0301.md"}, {"é.md", "e\u0301.md", "é.md"}, {`a\b.md`}} {
		files, got, err := selectSources(catalog, names)
		if err != nil {
			t.Fatal(err)
		}
		want := names
		if len(names) == 3 {
			want = []string{"e\u0301.md", "é.md"}
		}
		if !reflect.DeepEqual(got, want) || len(files) != len(want) {
			t.Fatal(got, files)
		}
		for i, file := range files {
			if file != catalog.ByExactPath[want[i]] {
				t.Fatal(file)
			}
		}
	}
	if _, _, err := selectSources(root.NewCatalog([]root.File{{LogicalPath: "é.md"}}), []string{"e\u0301.md"}); err == nil {
		t.Fatal("normalized alias selected")
	}
}

func FuzzSourceInput(f *testing.F) {
	for _, input := range []string{"", "A.md\x00", "A.md", "\x00", "a/../b.md\x00", "e\u0301.md\x00"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		names, err := readSources0(strings.NewReader(input))
		if err != nil {
			return
		}
		catalog := root.NewCatalog([]root.File{{LogicalPath: "A.md"}})
		files, names, err := selectSources(catalog, names)
		if err == nil {
			for i, file := range files {
				if file.LogicalPath != names[i] || file.LogicalPath != "A.md" {
					t.Fatal(files, names)
				}
			}
		}
	})
}
