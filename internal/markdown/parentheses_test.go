package markdown

import (
	"reflect"
	"strings"
	"testing"
)

func TestScanMarkdownParentheses(t *testing.T) {
	for _, destination := range []string{
		`Note\(draft\).md`, `Note(draft).md`, `Note(a(b)c).md`,
		`Note\\(draft).md`, `Note\\\(draft\).md`, `Note(draft\)).md`,
	} {
		input := "[label](" + destination + ") [good](Good.md)"
		got := Scan("Source.md", []byte(input))
		want := []RawLink{
			{SourceLogicalPath: "Source.md", RawTarget: destination, Kind: MarkdownLink},
			{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v; want %+v", destination, got, want)
		}
	}
	for _, prefix := range []string{"[bad](Note(draft.md)", "[bad](Note(a(b).md)", `[bad](Note\(draft\).md`, "[bad](Note(draft\n.md))"} {
		input := prefix + " [good](Good.md)"
		got := Scan("Source.md", []byte(input))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v", prefix, got)
		}
	}
}

func TestScanMarkdownAngleParentheses(t *testing.T) {
	for _, destination := range []string{"Note(draft.md", "Note)draft.md", `Note\>draft(.md`} {
		input := "[angle](<" + destination + ">) [bare](Note(draft).md)"
		got := Scan("Source.md", []byte(input))
		want := []RawLink{
			{SourceLogicalPath: "Source.md", RawTarget: destination, Kind: MarkdownLink},
			{SourceLogicalPath: "Source.md", RawTarget: "Note(draft).md", Kind: MarkdownLink, Offset: strings.Index(input, "[bare]")},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestDestinationIndex(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  map[int]destinationBounds
	}{
		{"[a](A)", map[int]destinationBounds{3: {begin: 4, end: 5, next: 6}}},
		{"[a](A(B))", map[int]destinationBounds{3: {begin: 4, end: 8, next: 9}}},
		{"([a](A)", map[int]destinationBounds{4: {begin: 5, end: 6, next: 7}}},
		{")[a](A)", map[int]destinationBounds{4: {begin: 5, end: 6, next: 7}}},
		{`[a](A\))`, map[int]destinationBounds{3: {begin: 4, end: 7, next: 8}}},
		{`[a](A\\(B))`, map[int]destinationBounds{3: {begin: 4, end: 10, next: 11}}},
		{"[a](A\n)", nil}, {"[a](A\r)", nil},
		{"[a](<A)> )", map[int]destinationBounds{3: {begin: 5, end: 7, next: 10, angle: true}}},
		{"[a](<A> X)", map[int]destinationBounds{3: {begin: 4, end: 9, next: 10, invalid: true}}}, {"[a](<A\n>)", nil},
	} {
		got := inlineDestinations([]byte(tt.input))
		if len(got) != len(tt.want) {
			t.Fatalf("%q: index = %+v; want %+v", tt.input, got, tt.want)
		}
		for opening, want := range tt.want {
			if got[opening] != want {
				t.Fatalf("%q: index = %+v; want %+v", tt.input, got, tt.want)
			}
		}
	}
	if got := inlineDestinations([]byte(strings.Repeat("(x)", 100000))); len(got) != 0 {
		t.Fatalf("index without candidates: %d", len(got))
	}
}

func TestScanMarkdownTitleConsumesEnclosedLinks(t *testing.T) {
	for _, input := range []string{
		`[bad](Note.md "[hidden](Hidden.md)) [good](Good.md)`,
	} {
		got := Scan("Source.md", []byte(input))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}
