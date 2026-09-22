package markdown

import (
	"fmt"
	"strings"
	"testing"
)

func TestQuotedTitles(t *testing.T) {
	for _, test := range []struct{ destination, target string }{
		{`Note.md "title"`, "Note.md"}, {`Note.md 'title'`, "Note.md"}, {`Note.md ""`, "Note.md"}, {"Note.md\t\"title\"", "Note.md"},
		{`Note name.md "title"`, "Note name.md"}, {`<Note name.md> 'title'`, "Note name.md"},
		{`Note.md "unbalanced ) (( [[Fake]] [x](Fake.md) %% <!--"`, "Note.md"},
		{`<Note.md> "unbalanced ) (( [[Fake]] [x](Fake.md) %% <!--"`, "Note.md"},
		{`Notes "draft".md 'title'`, `Notes "draft".md`},
		{`Note.md "escaped \" quote"`, "Note.md"},
	} {
		for _, prefix := range []string{"[label](", "![label]("} {
			input := prefix + test.destination + ") [good](Good.md)"
			got := ScanWithRaw("Source.md", []byte(input))
			if len(got) != 2 || got[0].RawTarget != test.target || got[0].RawLink != prefix+test.destination+")" || got[1].RawTarget != "Good.md" {
				t.Fatalf("%q: %+v", input, got)
			}
		}
	}
}

func TestTitleRecovery(t *testing.T) {
	for _, prefix := range []string{
		`[bad](Note.md "unclosed)`, `[bad](Note.md "[fake](Fake.md))`, `[bad](<Note.md> "title" junk)`, `[bad](<Note.md>suffix)`,
	} {
		input := prefix + " [good](Good.md)"
		got := Scan("Source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Good.md" || got[0].Offset != strings.Index(input, "[good]") {
			t.Fatalf("%q: %+v", input, got)
		}
	}
	input := "[bad](Note.md \"unclosed [[Fake]]\n[good](Good.md)"
	got := Scan("Source.md", []byte(input))
	if len(got) != 1 || got[0].RawTarget != "Good.md" {
		t.Fatal(got)
	}
}

func TestTitlePathEscapeCompatibility(t *testing.T) {
	for _, target := range []string{`Don't.md`, `Notes "draft".md`, `Notes "draft".md 'suffix'.md`, `Note.md \"title\"`, `Note\ "title"`, `Note\\\ "title"`, `<quote"file.md>`} {
		input := "[label](" + target + ")"
		got := Scan("Source.md", []byte(input))
		want := strings.Trim(target, "<>")
		if len(got) != 1 || got[0].RawTarget != want {
			t.Fatalf("%s: %+v", input, got)
		}
	}
	for _, input := range []string{
		`[bad](Note.md "unclosed [[Fake]]) [good](Good.md)`,
		`[bad](<Note.md> "[fake](Fake.md)" junk) [good](Good.md)`,
		"[bad](Note.md \"multiline\n[good](Good.md)",
	} {
		got := Scan("Source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Good.md" {
			t.Fatalf("%q: %+v", input, got)
		}
	}
}

func TestTitleFragmentMaskOwnership(t *testing.T) {
	for _, body := range []string{`# [Real](file.md "literal ) (( %% <!--")`, `# ![Real](<file.md> "literal ) (( %% <!--")`} {
		if got := BuildFragments([]byte(body + "\n")).Match("#Real"); got != "" {
			t.Fatalf("%s: %s", body, got)
		}
	}
}

func TestClosedPathQuoteWithMissingOuterParen(t *testing.T) {
	body := `[bad](Notes "draft ".md [good](Good.md)`
	got := Scan("Source.md", []byte(body))
	if len(got) != 1 || got[0].RawTarget != "Good.md" {
		t.Fatal(got)
	}
}

func BenchmarkTitleCandidates(b *testing.B) {
	for _, n := range []int{1000, 10000, 100000} {
		for _, unit := range []string{`[x](Target.md "title ) ( %%") `, `[bad](Target.md "unclosed) `, `[bad](Notes "draft ".md `} {
			body := []byte(strings.Repeat(unit, n) + "[good](Good.md)")
			b.Run(fmt.Sprintf("n=%d/bytes=%d", n, len(body)), func(b *testing.B) {
				b.SetBytes(int64(len(body)))
				b.ReportAllocs()
				for b.Loop() {
					Scan("Source.md", body)
				}
			})
		}
	}
}
