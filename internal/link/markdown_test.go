package link

import (
	"slices"
	"testing"

	"mdlink/internal/markdown"
	"mdlink/internal/root"
)

func TestResolveMarkdownEncoding(t *testing.T) {
	for destination, name := range map[string]string{
		"Note%20name.md":                 "Note name.md",
		"%E6%97%A5%E6%9C%AC%E8%AA%9E.md": "日本語.md",
		"A+B.md":                         "A+B.md",
		"%252F.md":                       "%2F.md",
	} {
		raw := markdown.Scan("Source.md", []byte("[label]("+destination+")"))
		if len(raw) != 1 || raw[0].RawTarget != destination {
			t.Fatalf("raw = %+v", raw)
		}
		catalog := root.NewCatalog([]root.File{{LogicalPath: name}})
		got := Resolve(catalog, raw[0])
		if got.Status != Resolved || got.Target != catalog.ByExactPath[name] {
			t.Fatalf("%q: Resolve = %+v", destination, got)
		}
	}
}

func TestResolveMarkdownParsedTargets(t *testing.T) {
	for destination, name := range map[string]string{
		"  Note name.md \t":     "Note name.md",
		"<Note name.md>":        "Note name.md",
		`<A\>B.md>`:             "A>B.md",
		"Note.md#Missing":       "Note.md",
		"Note.md?view=1#^block": "Note.md",
		"Name%23part%3F.md":     "Name#part?.md",
		`Name\#part\?.md`:       "Name#part?.md",
		`Name\q.md`:             `Name\q.md`,
		`Name\ space.md`:        `Name\ space.md`,
		`A\\B.md`:               `A\B.md`,
		"#Missing":              "Source.md",
		`Note\(draft\).md`:      "Note(draft).md",
		`Note(a(b)c).md`:        "Note(a(b)c).md",
		`Note\\(draft).md`:      `Note\(draft).md`,
	} {
		raw := markdown.Scan("Source.md", []byte("[label]("+destination+")"))
		if len(raw) != 1 {
			t.Fatalf("%q: raw = %+v", destination, raw)
		}
		catalog := root.NewCatalog([]root.File{{LogicalPath: name}})
		got := Resolve(catalog, raw[0])
		if got.Status != Resolved || got.Target != catalog.ByExactPath[name] {
			t.Fatalf("%q: Resolve = %+v", destination, got)
		}
	}
}

func TestResolveMarkdownDoesNotLookupExternalOrBrokenTarget(t *testing.T) {
	for _, target := range []string{"file:Note.md", "%66ile:Note.md", "//host/Note.md", "%2F%2Fhost/Note.md", "%", "%GG", "%FF"} {
		catalog := root.NewCatalog([]root.File{{LogicalPath: target}})
		got := Resolve(catalog, markdown.RawLink{RawTarget: target, Kind: markdown.MarkdownLink})
		if got.Status != Unresolved || got.Target != nil {
			t.Fatalf("%q: Resolve = %+v", target, got)
		}
	}
}

func TestResolveMarkdownExplicitContext(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "Note.md"}, {LogicalPath: "a/Note.md"}, {LogicalPath: "other/Missing.md"}})
	for _, tt := range []struct {
		target, want string
		status       ResolutionStatus
	}{
		{"./Note.md", "a/Note.md", Resolved},
		{"../Note.md", "Note.md", Resolved},
		{"part/../Note.md", "a/Note.md", Resolved},
		{"/Note.md", "Note.md", Resolved},
		{"./Note", "a/Note.md", Resolved},
		{"/Note", "Note.md", Resolved},
		{"..%2FNote.md", "Note.md", Resolved},
		{"%2FNote.md", "Note.md", Resolved},
		{"../../Note.md", "", Unsafe},
		{"..%2F..%2FNote.md", "", Unsafe},
		{"/../Note.md", "", Unsafe},
		{"%00", "", Unsafe},
		{"./Missing.md", "", Unresolved},
		{"/Missing.md", "", Unresolved},
		{"/missing/Note.md", "", Unresolved},
	} {
		t.Run(tt.target, func(t *testing.T) {
			got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: "a/Source.md", RawTarget: tt.target, Kind: markdown.MarkdownLink})
			if got.Status != tt.status || got.Target != catalog.ByExactPath[tt.want] {
				t.Fatalf("Resolve = %+v", got)
			}
		})
	}
	// Wikilinks keep their root-based contract for the same source.
	for _, tt := range []struct {
		target, want string
		status       ResolutionStatus
	}{
		{"./Note.md", "Note.md", Resolved}, {"../Note.md", "", Unsafe}, {"/Note.md", "", Unsafe},
	} {
		got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: "a/Source.md", RawTarget: tt.target, Kind: markdown.Wikilink})
		if got.Status != tt.status || got.Target != catalog.ByExactPath[tt.want] {
			t.Fatalf("Wiki %q: %+v", tt.target, got)
		}
	}
}

func TestResolveMarkdownContextPriority(t *testing.T) {
	for _, tt := range []struct {
		name, target, want string
		paths              []string
		status             ResolutionStatus
	}{
		{"root exact", "Note.md", "Note.md", []string{"Note.md", "a/Note.md"}, Resolved},
		{"source exact", "Note.md", "a/Note.md", []string{"a/Note.md", "other/Note.md"}, Resolved},
		{"folder root exact", "folder/Note.md", "folder/Note.md", []string{"folder/Note.md", "a/folder/Note.md"}, Resolved},
		{"folder source exact", "folder%2FNote.md", "a/folder/Note.md", []string{"a/folder/Note.md"}, Resolved},
		{"source exact before root extension", "Design.v1", "a/Design.v1", []string{"a/Design.v1", "Design.v1.md"}, Resolved},
		{"root extension before source stem", "Note", "Note.md", []string{"Note.md", "a/Note.md"}, Resolved},
		{"source stem stays global ambiguous", "Note", "", []string{"a/Note.md", "other/Note.md"}, Ambiguous},
		{"source folder extension not inserted", "folder/Note", "", []string{"a/folder/Note.md"}, Unresolved},
		{"source extensionless file excluded", "Note", "Note.md", []string{"a/Note", "Note.md"}, Resolved},
		{"missing folder remains explicit", "missing/Note.md", "", []string{"a/Note.md"}, Unresolved},
		{"NFC root exact", "Å.md", "A\u030a.md", []string{"A\u030a.md", "a/A\u030a.md"}, Resolved},
		{"NFC source exact", "folder/Å.md", "a/folder/A\u030a.md", []string{"a/folder/A\u030a.md"}, Resolved},
		{"NFC source exact before root extension", "Å.v1", "a/A\u030a.v1", []string{"a/A\u030a.v1", "A\u030a.v1.md"}, Resolved},
		{"NFC root extension", "Å", "A\u030a.md", []string{"A\u030a.md", "a/A\u030a.md"}, Resolved},
		{"raw source before NFC root", "Å.md", "a/Å.md", []string{"A\u030a.md", "a/Å.md"}, Resolved},
		{"raw root extension before NFC source", "Å.v1", "Å.v1.md", []string{"a/A\u030a.v1", "Å.v1.md"}, Resolved},
		{"raw basename before NFC source", "Å.md", "other/Å.md", []string{"a/A\u030a.md", "other/Å.md"}, Resolved},
		{"NFC explicit relative", "./Å", "a/A\u030a.md", []string{"a/A\u030a.md", "A\u030a.md"}, Resolved},
		{"NFC root relative", "/Å", "A\u030a.md", []string{"a/A\u030a.md", "A\u030a.md"}, Resolved},
		{"NFC explicit missing", "./Å", "", []string{"other/A\u030a.md"}, Unresolved},
		{"NFC root missing", "/Å", "", []string{"other/A\u030a.md"}, Unresolved},
		{"nul before dot cleanup", "bad%00/../Note.md", "", []string{"a/Note.md"}, Unsafe},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := make([]root.File, len(tt.paths))
			for i, name := range tt.paths {
				files[i] = root.File{LogicalPath: name}
			}
			for range 2 {
				catalog := root.NewCatalog(files)
				got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: "a/Source.md", RawTarget: tt.target, Kind: markdown.MarkdownLink})
				if got.Status != tt.status || got.Target != catalog.ByExactPath[tt.want] {
					t.Fatalf("Resolve = %+v", got)
				}
				slices.Reverse(files)
			}
		})
	}
}

func TestResolveMarkdownNFCAmbiguity(t *testing.T) {
	for _, tt := range []struct {
		target, prefix string
		extra          []string
	}{
		{"Å.md", "", []string{"a/A\u030a.md"}},
		{"folder/Å.md", "a/folder/", nil},
		{"./Å", "a/", []string{"A\u030a.md"}},
		{"/Å", "", []string{"a/A\u030a.md"}},
	} {
		files := []root.File{{LogicalPath: tt.prefix + "A\u030a.md"}, {LogicalPath: tt.prefix + "Å.md"}}
		for _, name := range tt.extra {
			files = append(files, root.File{LogicalPath: name})
		}
		for range 2 {
			catalog := root.NewCatalog(files)
			got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: "a/Source.md", RawTarget: tt.target, Kind: markdown.MarkdownLink})
			if got.Status != Ambiguous || got.Target != nil || len(got.Candidates) != 2 || got.Candidates[0].LogicalPath != tt.prefix+"A\u030a.md" || got.Candidates[1].LogicalPath != tt.prefix+"Å.md" {
				t.Fatalf("%q: %+v", tt.target, got)
			}
			slices.Reverse(files)
		}
	}
}

func TestResolveMarkdownSourceBoundary(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "Note.md"}, {LogicalPath: "a/Note.md"}, {LogicalPath: "a/Other.md"}})
	for _, source := range []string{"../Source.md", "/a/Source.md", "bad\x00/../a/Source.md", "."} {
		for _, target := range []string{"./Note.md", "Other.md"} {
			got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: source, RawTarget: target, Kind: markdown.MarkdownLink})
			if got.Status != Unsafe || got.Target != nil {
				t.Fatalf("source %q target %q: %+v", source, target, got)
			}
		}
		for _, target := range []string{"Note.md", "/Note.md"} {
			got := Resolve(catalog, markdown.RawLink{SourceLogicalPath: source, RawTarget: target, Kind: markdown.MarkdownLink})
			if got.Status != Resolved || got.Target != catalog.ByExactPath["Note.md"] {
				t.Fatalf("unused source %q target %q: %+v", source, target, got)
			}
		}
	}
	for _, target := range []string{"./Note.md", "Other.md"} {
		got := Resolve(catalog, markdown.RawLink{RawTarget: target, Kind: markdown.MarkdownLink})
		want := "Note.md"
		if target == "Other.md" {
			want = "a/Other.md"
		}
		if got.Status != Resolved || got.Target != catalog.ByExactPath[want] {
			t.Fatalf("empty source target %q: %+v", target, got)
		}
	}
	if got := Resolve(catalog, markdown.RawLink{Subpath: "#Heading", Kind: markdown.MarkdownLink}); got.Status != Unresolved {
		t.Fatalf("empty source self: %+v", got)
	}
}

func TestResolveMarkdownImages(t *testing.T) {
	catalog := root.NewCatalog([]root.File{{LogicalPath: "image.png"}, {LogicalPath: "image name.png"}, {LogicalPath: "image"}})
	for _, tt := range []struct {
		destination, want string
		status            ResolutionStatus
	}{
		{"image.png", "image.png", Resolved},
		{"<image name.png>", "image name.png", Resolved},
		{"image%20name.png#Part", "image name.png", Resolved},
		{"image name.png?view=1", "image name.png", Resolved},
		{"image", "", Unresolved},
	} {
		links := markdown.Scan("Source.md", []byte("![alt]("+tt.destination+")"))
		if len(links) != 1 || !links[0].Embed || links[0].Offset != 0 {
			t.Fatalf("%q: Scan = %+v", tt.destination, links)
		}
		got := Resolve(catalog, links[0])
		if got.Status != tt.status || got.Target != catalog.ByExactPath[tt.want] {
			t.Fatalf("%q: Resolve = %+v", tt.destination, got)
		}
	}
}
