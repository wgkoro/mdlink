package markdown

import (
	"strings"
	"testing"
)

func TestFragmentRepresentativeFixtures(t *testing.T) {
	tests := []struct{ body, fragment, want string }{
		{"# Alpha\n", "#Alpha", ""}, {"# Alpha\n\n- item\n", "#Alpha", ""}, {"# **Alpha**\n", "#Alpha", ""}, {"Alpha\n=====\n", "#Alpha", ""}, {"Paragraph. ^item-1\n", "#^item-1", ""}, {"> # Alpha\n", "#Alpha", ""}, {"# `Alpha`\n", "#Alpha", ""}, {"# Alpha\n\n- item\n", "#Absent", "missing-fragment"}, {"# Alpha\n\n> # Alpha\n", "#Alpha", "ambiguous-fragment"}, {"- Paragraph. ^item-1\n", "#^item-1", ""},
	}
	for i, test := range tests {
		if got := BuildFragments([]byte(test.body)).Match(test.fragment); got != test.want {
			t.Fatalf("F%d: got %q want %q", i+1, got, test.want)
		}
	}
}

func TestFragmentStructureAndExclusions(t *testing.T) {
	for _, test := range []struct{ body, fragment, want string }{
		{"# [Alpha](dest) ![Beta](image)\n", "#Alpha Beta", ""},
		{"# é\n", "#e\u0301", ""},
		{"# Alpha\n", "#alpha", "missing-fragment"},
		{"# Alpha\n", "# Alpha", "missing-fragment"},
		{"# A  B\n", "#A B", "missing-fragment"},
		{"# A  B\n", "#A  B", ""},
		{"# `a&b\\c`\n", "#a&b\\c", ""},
		{"First\nSecond\n======\n", "#First Second", ""},
		{"#\n", "#x", "missing-fragment"},
		{"# [x](dest)\n# Alpha\n", "#Alpha", ""},
		{"# <em>Alpha</em>\n# Known\n", "#Known", "unsupported-fragment"},
		{"# &amp;\n# Known\n", "#Known", "unsupported-fragment"},
		{"# \\*Alpha\\*\n# Known\n", "#Known", "unsupported-fragment"},
		{"# [[Alpha]]\n# Known\n", "#Known", "unsupported-fragment"},
		{"# <https://example.com>\n# Known\n", "#Known", "unsupported-fragment"},
		{"# &amp;\n\nParagraph ^ok\n", "#^ok", ""},
		{"---\n# Fake\n^fake\n---\n# Real\n", "#Fake", "missing-fragment"},
		{"---\n^fake\n...\n# Real\n", "#^fake", "missing-fragment"},
		{"\ufeff---\r\n# Fake\r\n---\r\n# Real\r\n", "#Real", ""},
		{"---\n# Fake\n", "#Fake", "unsupported-fragment"},
		{"    # Fake\n    ^fake\n\n# Real\n", "#Fake", "missing-fragment"},
		{"```\n# Fake\n^fake\n```\n# Real\n", "#^fake", "missing-fragment"},
		{"<!--\n# Fake\n^fake\n-->\n# Real\n", "#Fake", "missing-fragment"},
		{"%%\n# Fake\n^fake\n%%\n# Real\n", "#^fake", "missing-fragment"},
		{"`%%`\n\n# Real\n", "#Real", ""},
		{"```\n%%\n```\n# Real\n", "#Real", ""},
		{"<div>\n# Fake\n^fake\n</div>\n\n# Real\n", "#^fake", "missing-fragment"},
		{"# Heading ^id\n", "#^id", "missing-fragment"},
		{"Paragraph ^id\ncontinued\n", "#^id", "missing-fragment"},
		{"^id\n", "#^id", ""},
		{"Paragraph ^id  \n", "#^id", ""},
		{"> Paragraph ^id\n", "#^id", ""},
		{"- Paragraph ^id\n- Other\n", "#^id", ""},
		{"Paragraph ^id\n\nSecond ^id\n", "#^id", "ambiguous-fragment"},
		{"Paragraph `^id`\n", "#^id", "missing-fragment"},
		{"Paragraph [^id](dest)\n", "#^id", "missing-fragment"},
		{"Paragraph ![^id](dest)\n", "#^id", "missing-fragment"},
		{"Paragraph \\^id\n", "#^id", "missing-fragment"},
		{"Paragraph ^id\n", "#^ID", "missing-fragment"},
		{"Paragraph ^id\n", "#^bad_id", "unsupported-fragment"},
		{"# Alpha\n", "#", "unsupported-fragment"},
		{"# Alpha\n", "#Alpha#Beta", "unsupported-fragment"},
	} {
		t.Run(test.body+test.fragment, func(t *testing.T) {
			if got := BuildFragments([]byte(test.body)).Match(test.fragment); got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
		})
	}
}

func TestScanRawOffsets(t *testing.T) {
	body := []byte("日本 ![[Target#不存在|表示]] [text](Target.md#Absent)")
	plain, raw := Scan("Source.md", body), ScanWithRaw("Source.md", body)
	if len(plain) != 2 || len(raw) != 2 {
		t.Fatal(plain, raw)
	}
	for i, item := range raw {
		if item.RawLink != string(body[item.Offset:item.End]) || item.RawLink == "" {
			t.Fatal(item)
		}
		item.RawLink = ""
		item.End = 0
		if item != plain[i] {
			t.Fatal(item, plain[i])
		}
	}
}

func FuzzFragments(f *testing.F) {
	for _, body := range []string{"# Alpha\n", "---\n# x\n---", "# `x`\n\n- item ^id", "%%<!--\n#x"} {
		f.Add(body, "#Alpha")
	}
	f.Fuzz(func(t *testing.T, body, fragment string) {
		index := BuildFragments([]byte(body))
		got := index.Match(fragment)
		if got != "" && got != "missing-fragment" && got != "unsupported-fragment" && got != "ambiguous-fragment" {
			t.Fatal(got)
		}
	})
}

func TestFragmentLiteralCommentOwnership(t *testing.T) {
	for _, body := range []string{
		"    <!--\n\n# Real\n",
		"> ~~~\n> <!--\n> ~~~\n\n# Real\n",
		"# [Real](foo<!--bar)\n",
		"# [Real](foo%%bar)\n",
		"# ![Real](foo%%bar)\n",
		"# [Real](https://x/%%foo)\n",
		"# ![Real](%GG<!--literal)\n",
	} {
		if got := BuildFragments([]byte(body)).Match("#Real"); got != "" {
			t.Errorf("%q: %s", body, got)
		}
	}
}

func TestFragmentAmbiguousCommentCodeBoundary(t *testing.T) {
	for _, body := range []string{"%%\n~~~\n%%\n# Real\n", "%%\n~~~ %%\n# Real\n", "%%\n`literal %%\nreal`\n# Real\n"} {
		index := BuildFragments([]byte(body))
		for _, fragment := range []string{"#Real", "#^id"} {
			if got := index.Match(fragment); got != "unsupported-fragment" {
				t.Fatalf("%q %s: %s", body, fragment, got)
			}
		}
	}
}

func TestFragmentBOMView(t *testing.T) {
	for indent := 0; indent <= 3; indent++ {
		body := []byte("\ufeff" + strings.Repeat(" ", indent) + "# Real\n")
		if got := BuildFragments(body).Match("#Real"); got != "" {
			t.Fatal(indent, got)
		}
	}
	plain := ScanWithRaw("Source.md", []byte("[[#Real]]"))[0]
	bom := ScanWithRaw("Source.md", []byte("\ufeff[[#Real]]"))[0]
	if bom.Offset != plain.Offset+3 || bom.End != plain.End+3 || bom.RawLink != plain.RawLink {
		t.Fatal(plain, bom)
	}
}

func TestFragmentCommentCrossesHTMLStructure(t *testing.T) {
	body := []byte("%%\n<div>\n%%\n> ~~~\n> <!--\n> ~~~\n\n# Real\n")
	if got := BuildFragments(body).Match("#Real"); got != "unsupported-fragment" {
		t.Fatal(got)
	}
}

func TestFragmentCommentCrossesParagraph(t *testing.T) {
	if got := BuildFragments([]byte("%%\ntext\n%%\n    <!--\n\n# Real\n")).Match("#Real"); got != "unsupported-fragment" {
		t.Fatal(got)
	}
	for _, body := range []string{"%%hidden%%\n# Real\n", "%%\nhidden\n%%\n# Real\n", "<!--hidden-->\n# Real\n", "<!--\nhidden\n-->\n# Real\n", "%%hidden%%  \r\n# Real\r\n"} {
		if got := BuildFragments([]byte(body)).Match("#Real"); got != "" {
			t.Fatalf("%q: %s", body, got)
		}
	}
}

func TestFragmentCommentsWithoutLinkBrackets(t *testing.T) {
	for _, body := range []string{"%%\n# Hidden\n%%\n# Visible", "<!--\n# Hidden\n-->\n# Visible"} {
		index := BuildFragments([]byte(body))
		if got := index.Match("#Hidden"); got != "missing-fragment" {
			t.Fatalf("hidden heading: %q", got)
		}
		if got := index.Match("#Visible"); got != "" {
			t.Fatalf("visible heading: %q", got)
		}
	}
}
