package link

import (
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"mdlink/internal/markdown"
	"mdlink/internal/root"
)

func TestResolveExactPath(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/Note.md"}})
	got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: "elsewhere/source.md", RawTarget: "folder/Note.md", Kind: markdown.Wikilink})
	if got.Status != Resolved || got.Target != catalog.ByExactPath["folder/Note.md"] {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveMarkdownExtension(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/Note.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "folder/Note", Kind: markdown.Wikilink})
	if got.Status != Resolved || got.Target != catalog.ByExactPath["folder/Note.md"] {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveDottedNamePrefersExactAttachment(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "Design.v1.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "Design.v1"})
	if got.Status != Resolved || got.Target.LogicalPath != "Design.v1.md" {
		t.Fatalf("Resolve = %+v", got)
	}
	catalog = root.NewCatalog([]root.File{{LogicalPath: "Design.v1.md"}, {LogicalPath: "Design.v1"}})
	got = Resolve(catalog, markdown.RawLink{RawTarget: "Design.v1"})
	if got.Status != Resolved || got.Target.LogicalPath != "Design.v1" {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveDoesNotDoubleMarkdownExtension(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "Note.md.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "Note.md"})
	if got.Status != Unresolved {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveCleansLogicalPath(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/Note.md"}, {LogicalPath: "Note.md"}})
	for input, want := range map[string]string{"./folder/Note": "folder/Note.md", "folder/../Note": "Note.md"} {
		got := Resolve(catalog, markdown.RawLink{RawTarget: input})
		if got.Status != Resolved || got.Target.LogicalPath != want {
			t.Fatalf("%q: Resolve = %+v", input, got)
		}
	}
}

func TestResolveEmptyTarget(t *testing.T) {
	got := Resolve(root.NewCatalog(nil), markdown.RawLink{})
	if got.Status != Unresolved {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveLiteralCLITarget(t *testing.T) {
	catalog := root.NewCatalog([]root.File{
		{LogicalPath: "Note.md"}, {LogicalPath: "Note#Heading.md"},
		{LogicalPath: "#Heading.md"}, {LogicalPath: "A%20B.md"}, {LogicalPath: "A B.md"},
	})
	for target, want := range map[string]string{
		"Note#Heading.md": "Note#Heading.md",
		"Note#Heading":    "Note#Heading.md",
		"#Heading":        "#Heading.md",
		"A%20B":           "A%20B.md",
	} {
		got := Resolve(catalog, markdown.RawLink{RawTarget: target, Kind: markdown.Wikilink})
		if got.Status != Resolved || got.Target != catalog.ByExactPath[want] {
			t.Fatalf("%q: Resolve = %+v", target, got)
		}
	}
}

func TestResolveUnsafePaths(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "Note.md"}})
	for _, input := range []string{"/Note.md", "../Note", "a/../../Note", "a\x00b", "."} {
		got := Resolve(catalog, markdown.RawLink{RawTarget: input})
		if got.Status != Unsafe || got.Target != nil {
			t.Fatalf("%q: Resolve = %+v", input, got)
		}
	}
	if got := Resolve(catalog, markdown.RawLink{RawTarget: "missing"}); got.Status != Unresolved {
		t.Fatalf("missing: %+v", got)
	}
}

func FuzzResolvePath(f *testing.F) {
	for _, target := range []string{"", "Note", "folder/Note.md", "./Note", "../Note", "/Note.md", "a\x00b", "Design.v1", "é", "e\u0301", "Å.md", "Shared", "docs/../../Note", "%2e%2e/Note", "\xff"} {
		f.Add(target, "source.md")
	}
	f.Add("", "../source.md")
	f.Add("", "\xff")
	for _, target := range []string{"../Note.md", "../../Note.md", "%2FNote.md", "..%2F..%2FNote.md", "bad%00/../Note.md", "./Å"} {
		f.Add(target, "folder/source.md")
	}
	f.Add("Note.md", "bad\x00/../source.md")
	for _, target := range []string{"%4Eote", "%252F", "%2e%2e/Note", "%00", "%FF", "Name%23part", "file:Note", "//host/Note"} {
		f.Add(target, "source.md")
	}
	files := []root.File{{LogicalPath: "Note.md"}, {LogicalPath: "folder/Note.md"}, {LogicalPath: "Design.v1.md"}, {LogicalPath: "a/Shared.md"}, {LogicalPath: "b/Shared.md"}, {LogicalPath: "A\u030a.md"}, {LogicalPath: "Å.md"}, {LogicalPath: "source.md"}}
	catalog, reversed := root.NewCatalog(files), root.NewCatalog(files)
	for _, index := range []map[string][]*root.File{reversed.ByBaseName, reversed.ByStem, reversed.ByNormalizedPath, reversed.ByNormalizedBaseName, reversed.ByNormalizedStem} {
		for _, candidates := range index {
			slices.Reverse(candidates)
		}
	}
	f.Fuzz(func(t *testing.T, target, source string) {
		for _, raw := range []markdown.RawLink{
			{RawTarget: target, SourceLogicalPath: source, Kind: markdown.Wikilink},
			{SourceLogicalPath: source, Subpath: "#Heading", Kind: markdown.Wikilink},
			{RawTarget: target, SourceLogicalPath: source, Kind: markdown.MarkdownLink},
			{SourceLogicalPath: source, Subpath: "#Heading", Kind: markdown.MarkdownLink},
		} {
			got := Resolve(catalog, raw)
			if got.Status == Resolved && (got.Target == nil || catalog.ByExactPath[got.Target.LogicalPath] != got.Target) {
				t.Fatalf("resolved outside catalog: %+v", got)
			}
			value := raw.RawTarget
			if raw.Kind == markdown.MarkdownLink && value != "" {
				var ok bool
				value, ok = markdown.DecodeInternalTarget(value)
				if !ok && got.Status != Unresolved {
					t.Fatalf("external or malformed destination resolved: %+v", got)
				}
				if strings.ContainsRune(value, 0) && got.Status != Unsafe {
					t.Fatalf("decoded NUL accepted: %+v", got)
				}
				if strings.HasPrefix(value, "/") {
					value = strings.TrimPrefix(value, "/")
				} else {
					for component := range strings.SplitSeq(value, "/") {
						if component != "." && component != ".." {
							continue
						}
						if _, err := root.CleanLogicalPath(source); source != "" && err != nil {
							value = ""
						} else {
							value = path.Join(path.Dir(source), value)
						}
						break
					}
				}
			}
			if raw.RawTarget == "" && raw.Subpath != "" {
				value = raw.SourceLogicalPath
			}
			clean := path.Clean(value)
			if value != "" && (path.IsAbs(value) || strings.ContainsRune(value, 0) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../")) && got.Status != Unsafe {
				t.Fatalf("unsafe input %q: %+v", value, got)
			}
			if other := Resolve(reversed, raw); !reflect.DeepEqual(got, other) {
				t.Fatalf("candidate order changed result: %+v vs %+v", got, other)
			}
		}
	})
}

func TestResolveUniqueStem(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/Note.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "Note"})
	if got.Status != Resolved || got.Target != catalog.ByExactPath["folder/Note.md"] {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveUniqueBasename(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/Note.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "Note.md"})
	if got.Status != Resolved || got.Target != catalog.ByExactPath["folder/Note.md"] {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveAmbiguousStem(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "b/Note.md"}, {LogicalPath: "a/Note.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "Note"})
	if got.Status != Ambiguous || got.Target != nil || len(got.Candidates) != 2 || got.Candidates[0].LogicalPath != "a/Note.md" || got.Candidates[1].LogicalPath != "b/Note.md" {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveExactPrecedesNames(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "a/Note.md"}, {LogicalPath: "b/Note.md"}, {LogicalPath: "Note.md"}})
	for target, want := range map[string]string{"a/Note.md": "a/Note.md", "a/Note": "a/Note.md", "Note": "Note.md", "Note.md": "Note.md"} {
		got := Resolve(catalog, markdown.RawLink{RawTarget: target})
		if got.Status != Resolved || got.Target.LogicalPath != want {
			t.Fatalf("%q: Resolve = %+v", target, got)
		}
	}
}

func TestResolveKeepsExplicitFolder(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/Note.md"}})
	if got := Resolve(catalog, markdown.RawLink{RawTarget: "missing/Note"}); got.Status != Unresolved {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveCombinesBasenameAndStem(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "a/Spec.v1"}, {LogicalPath: "b/Spec.v1.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "Spec.v1"})
	if got.Status != Ambiguous || len(got.Candidates) != 2 || got.Candidates[0].LogicalPath != "a/Spec.v1" || got.Candidates[1].LogicalPath != "b/Spec.v1.md" {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveCandidateOrderAndDuplicates(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "a/Note.md"}, {LogicalPath: "b/Note.md"}})
	forward := Resolve(catalog, markdown.RawLink{RawTarget: "Note"})
	slices.Reverse(catalog.ByStem["Note"])
	catalog.ByStem["Note"] = append(catalog.ByStem["Note"], catalog.ByStem["Note"][0])
	before := slices.Clone(catalog.ByStem["Note"])
	backward := Resolve(catalog, markdown.RawLink{RawTarget: "Note"})
	if !reflect.DeepEqual(forward, backward) {
		t.Fatalf("candidate ordering changed: %+v != %+v", forward, backward)
	}
	if !reflect.DeepEqual(before, catalog.ByStem["Note"]) {
		t.Fatal("resolver modified catalog")
	}
}

func TestResolveNormalizedExactPath(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/e\u0301.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "folder/é.md"})
	if got.Status != Resolved || got.Target.LogicalPath != "folder/e\u0301.md" {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveNormalizedMarkdownExtension(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/e\u0301.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "folder/é"})
	if got.Status != Resolved || got.Target.LogicalPath != "folder/e\u0301.md" {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveNormalizedBasenameAndStem(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/e\u0301.md"}})
	for _, target := range []string{"é.md", "é"} {
		got := Resolve(catalog, markdown.RawLink{RawTarget: target})
		if got.Status != Resolved || got.Target.LogicalPath != "folder/e\u0301.md" {
			t.Fatalf("%q: Resolve = %+v", target, got)
		}
	}
}

func TestResolveNormalizedPriorityAndAmbiguity(t *testing.T) {
	cases := []struct {
		name, target string
		paths        []string
		status       ResolutionStatus
		path         string
		count        int
	}{
		{"full collision", "Å.md", []string{"A\u030a.md", "Å.md"}, Ambiguous, "", 2},
		{"extension collision", "Å", []string{"A\u030a.md", "Å.md"}, Ambiguous, "", 2},
		{"stem collision", "Å", []string{"a/A\u030a.md", "b/Å.md"}, Ambiguous, "", 2},
		{"raw exact first", "Å.md", []string{"Å.md", "A\u030a.md"}, Resolved, "Å.md", 0},
		{"raw basename first", "Å.md", []string{"A\u030a.md", "folder/Å.md"}, Resolved, "folder/Å.md", 0},
		{"NFC path before NFC basename", "Å.md", []string{"A\u030a.md", "folder/A\u030a.md"}, Resolved, "A\u030a.md", 0},
		{"raw stem first", "Å", []string{"A\u030a.md", "folder/Å.md"}, Resolved, "folder/Å.md", 0},
		{"NFC path before NFC stem", "Å", []string{"A\u030a.md", "folder/A\u030a.md"}, Resolved, "A\u030a.md", 0},
		{"case sensitive", "note", []string{"Note.md"}, Unresolved, "", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var files []root.File
			for _, name := range test.paths {
				files = append(files, root.File{LogicalPath: name})
			}
			catalog := root.NewCatalog(files)
			got := Resolve(catalog, markdown.RawLink{RawTarget: test.target})
			if got.Status != test.status || len(got.Candidates) != test.count || (test.path != "" && (got.Target == nil || got.Target.LogicalPath != test.path)) {
				t.Fatalf("Resolve = %+v", got)
			}
			for _, index := range []map[string][]*root.File{catalog.ByNormalizedPath, catalog.ByBaseName, catalog.ByStem, catalog.ByNormalizedBaseName, catalog.ByNormalizedStem} {
				for _, values := range index {
					slices.Reverse(values)
				}
			}
			if reversed := Resolve(catalog, markdown.RawLink{RawTarget: test.target}); !reflect.DeepEqual(got, reversed) {
				t.Fatalf("candidate order changed result: %+v vs %+v", got, reversed)
			}
		})
	}
}

func TestResolveScannedFragments(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "Note.md"}})
	for _, input := range []string{"[[Note#Heading]]", "[[Note#^block]]", "[[Note#Missing|alias]]", "![[Note#Heading]]"} {
		raw := markdown.Scan("source.md", []byte(input))
		if len(raw) != 1 {
			t.Fatalf("Scan = %+v", raw)
		}
		got := Resolve(catalog, raw[0])
		if got.Status != Resolved || got.Target != catalog.ByExactPath["Note.md"] {
			t.Fatalf("%q: Resolve = %+v", input, got)
		}
	}
}

func TestResolveSelfHeading(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/source.md"}, {LogicalPath: "other/source.md"}})
	raw := markdown.Scan("folder/source.md", []byte("[[#Heading]]"))
	got := Resolve(catalog, raw[0])
	if got.Status != Resolved || got.Target != catalog.ByExactPath["folder/source.md"] {
		t.Fatalf("Resolve = %+v", got)
	}
}

func TestResolveSelfHeadingRequiresExactSafeSource(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "folder/source.md"}, {LogicalPath: "source.md"}})
	for _, source := range []string{"missing/source.md", "source", ""} {
		got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: source, Subpath: "#Heading"})
		if got.Status != Unresolved {
			t.Fatalf("%q: Resolve = %+v", source, got)
		}
	}
	for _, source := range []string{"../source.md", "/source.md", "a\x00b"} {
		got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: source, Subpath: "#Heading"})
		if got.Status != Unsafe {
			t.Fatalf("%q: Resolve = %+v", source, got)
		}
	}
	if got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: "source.md"}); got.Status != Unresolved {
		t.Fatalf("empty target/subpath is not self: %+v", got)
	}
}

func TestResolveAttachmentExtensionRules(t *testing.T) {
	cases := []struct {
		name, target string
		paths        []string
		status       ResolutionStatus
		path         string
		count        int
	}{
		{"exact attachment", "files/image.png", []string{"files/image.png"}, Resolved, "files/image.png", 0},
		{"basename attachment", "image.png", []string{"files/image.png"}, Resolved, "files/image.png", 0},
		{"no attachment stem", "image", []string{"files/image.png"}, Unresolved, "", 0},
		{"ambiguous attachment", "image.png", []string{"a/image.png", "b/image.png"}, Ambiguous, "", 2},
		{"Markdown stem wins", "image", []string{"a/image.png", "b/image.md"}, Resolved, "b/image.md", 0},
		{"extensionless exact excluded", "LICENSE", []string{"LICENSE"}, Unresolved, "", 0},
		{"extensionless basename excluded", "LICENSE", []string{"files/LICENSE"}, Unresolved, "", 0},
		{"extensionless does not mask Markdown", "LICENSE", []string{"LICENSE", "LICENSE.md"}, Resolved, "LICENSE.md", 0},
		{"uppercase md attachment", "Note.MD", []string{"files/Note.MD"}, Resolved, "files/Note.MD", 0},
		{"uppercase md no stem", "Note", []string{"files/Note.MD"}, Unresolved, "", 0},
		{"NFC attachment", "é.png", []string{"files/e\u0301.png"}, Resolved, "files/e\u0301.png", 0},
		{"NFC md target excludes stem", "é.md", []string{"files/e\u0301.md.md"}, Unresolved, "", 0},
		{"NFC extensionless does not mask Markdown", "é", []string{"files/e\u0301", "notes/e\u0301.md"}, Resolved, "notes/e\u0301.md", 0},
		{"NFC attachment no stem", "é", []string{"files/e\u0301.png"}, Unresolved, "", 0},
		{"NFC extensionless path excluded", "é", []string{"e\u0301"}, Unresolved, "", 0},
		{"NFC extensionless basename excluded", "é", []string{"files/e\u0301"}, Unresolved, "", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var files []root.File
			for _, name := range test.paths {
				files = append(files, root.File{LogicalPath: name})
			}
			got := Resolve(root.NewCatalog(files), markdown.RawLink{RawTarget: test.target})
			if got.Status != test.status || len(got.Candidates) != test.count || (test.path != "" && (got.Target == nil || got.Target.LogicalPath != test.path)) {
				t.Fatalf("Resolve = %+v", got)
			}
		})
	}
}

func TestResolveUsesLogicalPathsForExternalFiles(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "docs/Note.md", PhysicalPath: "/outside/Note.md"}, {LogicalPath: "Note.md"}})
	if got := Resolve(catalog, markdown.RawLink{RawTarget: "docs/Note"}); got.Status != Resolved || got.Target != catalog.ByExactPath["docs/Note.md"] {
		t.Fatalf("logical target = %+v", got)
	}
	if got := Resolve(catalog, markdown.RawLink{RawTarget: "docs/../../Note"}); got.Status != Unsafe {
		t.Fatalf("symlink traversal = %+v", got)
	}
}

func TestResolveWikilinkDoesNotURLDecode(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "%2e%2e/Note.md"}, {LogicalPath: "Note.md"}})
	got := Resolve(catalog, markdown.RawLink{RawTarget: "%2e%2e/Note"})
	if got.Status != Resolved || got.Target != catalog.ByExactPath["%2e%2e/Note.md"] {
		t.Fatalf("literal percent path = %+v", got)
	}
}

func TestResolveNormalizedUnionDoesNotMutateIndexes(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "a/A\u030a.v1"}, {LogicalPath: "b/A\u030a.v1.md"}, {LogicalPath: "c/Å.v1"}})
	raw := markdown.RawLink{RawTarget: "Å.v1", Kind: markdown.Wikilink}
	want := Resolution{Status: Ambiguous, Candidates: []*root.File{
		catalog.ByExactPath["a/A\u030a.v1"], catalog.ByExactPath["b/A\u030a.v1.md"], catalog.ByExactPath["c/Å.v1"],
	}}
	if got := Resolve(catalog, raw); !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized union = %+v; want %+v", got, want)
	}
	for _, index := range []map[string][]*root.File{catalog.ByNormalizedBaseName, catalog.ByNormalizedStem} {
		for key, candidates := range index {
			slices.Reverse(candidates)
			index[key] = append(candidates, candidates[0])
		}
	}
	base := catalog.ByNormalizedBaseName[raw.RawTarget]
	stem := slices.Clone(catalog.ByNormalizedStem[raw.RawTarget])
	backing := make([]*root.File, len(base)+len(stem))
	copy(backing, base)
	catalog.ByNormalizedBaseName[raw.RawTarget] = backing[:len(base)]
	unchanged := slices.Clone(backing)
	for range 3 {
		if got := Resolve(catalog, raw); !reflect.DeepEqual(got, want) {
			t.Fatalf("reordered/duplicate candidates changed union: %+v", got)
		}
		if !slices.Equal(backing, unchanged) || !slices.Equal(catalog.ByNormalizedStem[raw.RawTarget], stem) {
			t.Fatal("Resolve mutated normalized index storage")
		}
	}
}
