package markdown

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestScanInlineMarkdownLink(t *testing.T) {
	got := Scan("folder/Source.md", []byte("前 [label](Note.md)"))
	want := []RawLink{{SourceLogicalPath: "folder/Source.md", RawTarget: "Note.md", Kind: MarkdownLink, Offset: 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanMarkdownLabels(t *testing.T) {
	for _, input := range []string{
		`[a\]b](Note.md)`, `[a\[b](Note.md)`,
		`[outer [inner] label](Note.md)`, `[outer [nested [deep]] label](Note.md)`,
		`[a\\[inner]](Note.md)`,
	} {
		got := Scan("Source.md", []byte(input))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Note.md", Kind: MarkdownLink}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanMixedLinkKinds(t *testing.T) {
	input := "[one](One.md) [[Wiki|[hidden](Hidden.md)]] [two](Two.md)"
	got := Scan("Source.md", []byte(input))
	want := []RawLink{
		{SourceLogicalPath: "Source.md", RawTarget: "One.md", Kind: MarkdownLink},
		{SourceLogicalPath: "Source.md", RawTarget: "Wiki", Kind: Wikilink, Offset: strings.Index(input, "[[Wiki")},
		{SourceLogicalPath: "Source.md", RawTarget: "Two.md", Kind: MarkdownLink, Offset: strings.Index(input, "[two]")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v", got)
	}
}

func TestScanMalformedMarkdownRecovery(t *testing.T) {
	for _, input := range []string{"", "[unclosed label", "[label](unclosed destination"} {
		if got := Scan("Source.md", []byte(input)); len(got) != 0 {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
	for _, input := range []string{
		"[unclosed label [good](Good.md)",
		"[bad](unfinished [good](Good.md)",
		"[bad](first\nsecond) [good](Good.md)",
		"[bad](first\rsecond) [good](Good.md)",
		"[bad](first\n[good](Good.md))",
	} {
		got := Scan("Source.md", []byte(input))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanMarkdownUsesCodeAndCommentExclusions(t *testing.T) {
	for _, hidden := range []string{
		"`[hidden](Hidden.md)`", "``[hidden](Hidden.md)``",
		"```\n[hidden](Hidden.md)\n```\n", "~~~\n[hidden](Hidden.md)\n~~~\n",
		"<!-- [hidden](Hidden.md) -->", "%% [hidden](Hidden.md) %%",
	} {
		input := hidden + " [visible](Visible.md)"
		got := Scan("Source.md", []byte(input))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Visible.md", Kind: MarkdownLink, Offset: strings.Index(input, "[visible]")}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanLongMalformedMarkdown(t *testing.T) {
	if os.Getenv("MDLINK_TEST_LONG_MARKDOWN") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScanLongMalformedMarkdown$")
		cmd.Env = append(os.Environ(), "MDLINK_TEST_LONG_MARKDOWN=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("long Markdown subprocess: %v, context=%v\n%s", err, ctx.Err(), output)
		}
		return
	}
	for _, prefix := range []string{
		strings.Repeat("[a", 100000),
		strings.Repeat("[a", 100000) + strings.Repeat("]", 100000),
		strings.Repeat("[bad](unfinished ", 100000),
		strings.Repeat("[bad](<unfinished ", 100000),
		strings.Repeat("[bad](Note(draft ", 100000),
		strings.Repeat("[bad](<unfinished ", 100000) + ">" + strings.Repeat(" ", 100000) + "X ",
		`[bad](Note.md "` + strings.Repeat("[inner](N(", 100000) + strings.Repeat("))", 100000) + `") `,
	} {
		input := prefix + " [good](Good.md)"
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: len(prefix) + 1}}
		if strings.HasPrefix(prefix, `[bad](Note.md "`) {
			want = append([]RawLink{{SourceLogicalPath: "Source.md", RawTarget: "Note.md", Kind: MarkdownLink}}, want...)
		}
		if got := Scan("Source.md", []byte(input)); !reflect.DeepEqual(got, want) {
			t.Fatalf("long input: Scan = %+v", got)
		}
	}
	input := strings.Repeat("[a", 100000) + "`[hidden](Hidden.md)` <!-- [hidden](Hidden.md) --> [[Wiki]] [good](Good.md)"
	want := []RawLink{
		{SourceLogicalPath: "Source.md", RawTarget: "Wiki", Kind: Wikilink, Offset: strings.Index(input, "[[Wiki")},
		{SourceLogicalPath: "Source.md", RawTarget: "Good.md", Kind: MarkdownLink, Offset: strings.Index(input, "[good]")},
	}
	if got := Scan("Source.md", []byte(input)); !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed long input: Scan = %+v", got)
	}
}

func TestInlineLabelIndex(t *testing.T) {
	for _, test := range []struct {
		input string
		want  map[int]int
	}{
		{"[a](A)", map[int]int{0: 2}},
		{"[[a]](A)", map[int]int{0: 4}},
		{"[[a](A)](B)", map[int]int{0: 7, 1: 3}},
		{`\[a](A)`, nil},
		{`\\[a](A)`, map[int]int{2: 4}},
		{`[a\]](A)`, map[int]int{0: 4}},
		{"] [a](A)", map[int]int{2: 4}},
		{"[x [a](A)", map[int]int{3: 5}},
		{"a](A)", nil},
	} {
		if got := inlineLabels([]byte(test.input)); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%q: index = %v; want %v", test.input, got, test.want)
		}
	}
	if got := inlineLabels([]byte(strings.Repeat("[a", 100000) + strings.Repeat("]", 100000))); len(got) != 0 {
		t.Fatalf("index grew without destination candidates: %d", len(got))
	}
}

func BenchmarkMalformedMarkdown(b *testing.B) {
	for _, repeats := range []int{1000, 10000, 100000} {
		for name, prefix := range map[string]string{
			"closed=false": strings.Repeat("[a", repeats),
			"closed=true":  strings.Repeat("[a", repeats) + strings.Repeat("]", repeats),
			"bare":         strings.Repeat("[bad](Note(draft ", repeats),
			"angle":        strings.Repeat("[bad](<unfinished ", repeats) + ">" + strings.Repeat(" ", repeats) + "X ",
			"title":        `[bad](Note.md "` + strings.Repeat("[inner](N(", repeats) + strings.Repeat("))", repeats) + `") `,
		} {
			input := []byte(prefix + "[good](Good.md)")
			b.Run(fmt.Sprintf("%s/n=%d/bytes=%d", name, repeats, len(input)), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(input)))
				for b.Loop() {
					Scan("Source.md", input)
				}
			})
		}
	}
}
