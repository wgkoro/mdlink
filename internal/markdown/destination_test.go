package markdown

import (
	"reflect"
	"strings"
	"testing"
)

func TestScanMarkdownDestinationParts(t *testing.T) {
	for _, test := range []struct{ destination, target, subpath string }{
		{"  Note name.md \t", "Note name.md", ""},
		{" <Note name.md> \t", "Note name.md", ""},
		{`<Note\>name.md>`, `Note\>name.md`, ""},
		{"Note.md#Heading", "Note.md", "#Heading"},
		{"Note.md#^block", "Note.md", "#^block"},
		{"Note.md?view=1#Heading", "Note.md", "#Heading"},
		{"Note.md#Heading?view=1", "Note.md", "#Heading?view=1"},
		{"Note.md?view=1", "Note.md", ""},
		{"Name%23part%3F.md", "Name%23part%3F.md", ""},
		{`Name\#part\?.md`, `Name\#part\?.md`, ""},
		{`Name\\#Heading`, `Name\\`, "#Heading"},
		{`Note.md?view=\#value#Heading`, "Note.md", "#Heading"},
		{"#Heading", "", "#Heading"},
	} {
		got := Scan("Source.md", []byte("[label]("+test.destination+")"))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: test.target, Subpath: test.subpath, Kind: MarkdownLink}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v; want %+v", test.destination, got, want)
		}
	}
}

func TestScanMarkdownRejectsBrokenDestinations(t *testing.T) {
	for _, destination := range []string{"%", "%GG", "%FF", "<unclosed.md", `<Name\>`, "", " \t", "?view=1"} {
		input := "[bad](" + destination + ") [good](Good.md)"
		got := Scan("Source.md", []byte(input))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v", destination, got)
		}
	}
}

func TestScanMarkdownExcludesRawAndEncodedURI(t *testing.T) {
	for _, destination := range []string{
		"http://host/Note.md", "https://host/Note.md", "mailto:name@example.org", "obsidian://open?vault=Example",
		"HTTP://host/Note.md", "FiLe:Note.md", "data:text/plain,Note.md", "a1+.-:Note.md", "file:Note.md", "//host/Note.md",
		"%68ttps%3A%2F%2Fhost/Note.md", `%66ile\:Note.md`, "%2F%2Fhost/Note.md",
	} {
		for _, marker := range []string{"", "!"} {
			input := marker + "[external](" + destination + ") [good](Good.md)"
			got := Scan("Source.md", []byte(input))
			want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%q: Scan = %+v", input, got)
			}
		}
	}
}

func TestScanMarkdownReferenceStyle(t *testing.T) {
	input := "[label][id]\n![image][id]\n[label][]\n![image][]\n[label]\n![image]\n[id]: Note.md\n[good](Good.md)"
	got := Scan("Source.md", []byte(input))
	want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Note.md", Kind: MarkdownLink}, {SourceLogicalPath: "Source.md", RawTarget: "Note.md", Kind: MarkdownLink, Embed: true, Offset: strings.Index(input, "![image]")}, {SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v", got)
	}
}

func TestScanMarkdownQuotedTitleBoundary(t *testing.T) {
	for _, destination := range []string{`Note.md "unclosed`, `Note.md 'unclosed`, `Notes "draft\".md`, `<Note.md>suffix`} {
		input := "[bad](" + destination + ") [good](Good.md)"
		got := Scan("Source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Good.md" || got[0].Offset != strings.Index(input, "[good]") {
			t.Fatalf("%q: Scan = %+v", destination, got)
		}
	}
	for _, destination := range []string{`Don't.md`, `Note.md \"title\"`, `Notes "draft".md`, `Notes "draft\\".md`, `Note.md %22title%22`, `Notes "draft".md 'suffix'.md`} {
		got := Scan("Source.md", []byte("[label]("+destination+")"))
		if len(got) != 1 || got[0].RawTarget != destination {
			t.Fatalf("%q: Scan = %+v", destination, got)
		}
	}
}
