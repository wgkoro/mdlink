package markdown

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestScanWikilink(t *testing.T) {
	got := Scan("source.md", []byte("[[Note]]"))
	want := []RawLink{{SourceLogicalPath: "source.md", RawTarget: "Note", Kind: Wikilink, Offset: 0}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanMultipleWikilinks(t *testing.T) {
	got := Scan("source.md", []byte("x [[One]] y [[Two]] [[One]] z"))
	want := []RawLink{
		{SourceLogicalPath: "source.md", RawTarget: "One", Kind: Wikilink, Offset: 2},
		{SourceLogicalPath: "source.md", RawTarget: "Two", Kind: Wikilink, Offset: 12},
		{SourceLogicalPath: "source.md", RawTarget: "One", Kind: Wikilink, Offset: 20},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanWikilinkAlias(t *testing.T) {
	got := Scan("source.md", []byte("[[Note|label#Heading|more]]"))
	want := []RawLink{{SourceLogicalPath: "source.md", RawTarget: "Note", Kind: Wikilink}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanNoLinksAndUnchangedInput(t *testing.T) {
	for _, input := range []string{"", "plain text", "[[Note]]"} {
		content := []byte(input)
		before := bytes.Clone(content)
		links := Scan("source.md", content)
		if input != "[[Note]]" && len(links) != 0 {
			t.Errorf("unexpected links: %+v", links)
		}
		if !bytes.Equal(content, before) {
			t.Fatal("Scan modified input")
		}
	}
}

func FuzzScanner(f *testing.F) {
	for _, seed := range []string{"", "plain", "[[Note]]", "x [[One]] [[Two]]", "[[Note|alias#Heading]]", "[[Note#Heading]] [[#Heading]]", "![[picture.png]]", "![[Note#^block|label]]", "`[[Hidden]]` [[Visible]]", "``code ` text`` [[Visible]]", "```go\n[[Hidden]]\n```", "~~~\n[[Hidden]]", "   ````\r\n[[Hidden]]\r\n````", "<!-- [[Hidden]] -->[[Visible]]", "%% [[Hidden]]", "`<!--` [[Visible]]", "%% `\n~~~\n[[Hidden]] %%[[Visible]]"} {
		f.Add([]byte(seed))
	}
	for _, seed := range []string{"[[Broken\n[[Good]]", "[[Broken [[Good]]", "[[ ]] [[#Heading]]", "\\[[Escaped]]", "\\\\![[Embed]]", "---\nrelated: \"[[Property]]\"\n---", "\xff[[Note]]\xfe", "前 [[日本語]] [[e\u0301]]"} {
		f.Add([]byte(seed))
	}
	for _, seed := range []string{"[label](Note.md)", "[[Wiki|[hidden](Hidden.md)]] [label](Note.md)", `[outer [a\]b]](Note.md)`, "[bad](unfinished [good](Good.md)", "[bad](first\nsecond) [good](Good.md)", "`[hidden](Hidden.md)` [good](Good.md)"} {
		f.Add([]byte(seed))
	}
	for _, prefix := range []string{strings.Repeat("[a", 1000), strings.Repeat("[a", 1000) + strings.Repeat("]", 1000), strings.Repeat("[bad](unfinished ", 1000)} {
		f.Add([]byte(prefix + "[good](Good.md)"))
	}
	for _, seed := range []string{`[x](<Note name.md>)`, `[x](Note.md?view=1#Heading)`, `[x](Name\#part\?.md)`, `[x](%252F.md)`, `[bad](%FF) [x](Good.md)`, `[bad](Note.md "unclosed) [x](Good.md)`, `[x](%68ttps%3A%2F%2Fhost/Note.md)`, strings.Repeat("[bad](<unfinished ", 1000) + "[good](Good.md)"} {
		f.Add([]byte(seed))
	}
	for _, seed := range []string{
		`![alt](image%20name.png#Part)`, `\![alt](image.png)`, `!\[alt](image.png)`,
		`[label](Note(a(b)c).md)`, `[label](Note\(draft\).md)`, `[label](<Note)draft.md>)`,
		`[bad](Note.md "[hidden](Hidden.md)") [good](Good.md)`,
		strings.Repeat("[bad](Note(draft ", 1000) + "[good](Good.md)",
		strings.Repeat("[bad](<unfinished ", 1000) + ">" + strings.Repeat(" ", 1000) + "X [good](Good.md)",
	} {
		f.Add([]byte(seed))
	}
	for _, seed := range []string{`![image](FiLe:Trap.md) [good](Good.md)`, `[url](data:text/plain,Note.md)`, "[label][id]\n![image][]\n[id]: Note.md\n[good](Good.md)"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, content []byte) {
		before := bytes.Clone(content)
		links := Scan("source.md", content)
		previous := -1
		for _, link := range links {
			if link.Offset < 0 || link.Offset >= len(content) || link.Offset <= previous {
				t.Fatalf("invalid offset %d after %d for %d bytes", link.Offset, previous, len(content))
			}
			previous = link.Offset
		}
		if !bytes.Equal(before, content) {
			t.Fatal("Scan modified input")
		}
		if !reflect.DeepEqual(links, Scan("source.md", content)) {
			t.Fatal("Scan is nondeterministic")
		}
	})
}

func TestScanWikilinkSubpath(t *testing.T) {
	for _, subpath := range []string{"#Heading", "#^block-id"} {
		got := Scan("source.md", []byte("[[Note"+subpath+"]]"))
		want := []RawLink{{SourceLogicalPath: "source.md", RawTarget: "Note", Subpath: subpath, Kind: Wikilink}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Scan = %+v; want %+v", got, want)
		}
	}
}

func TestScanWikilinkSubpathAndAlias(t *testing.T) {
	got := Scan("source.md", []byte("[[Note#Heading|label#Other]] [[#Heading]]"))
	want := []RawLink{
		{SourceLogicalPath: "source.md", RawTarget: "Note", Subpath: "#Heading", Kind: Wikilink},
		{SourceLogicalPath: "source.md", Subpath: "#Heading", Kind: Wikilink, Offset: 29},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanWikilinkEmbed(t *testing.T) {
	for _, target := range []string{"Note", "picture.png"} {
		got := Scan("source.md", []byte("x ![["+target+"]]"))
		want := []RawLink{{SourceLogicalPath: "source.md", RawTarget: target, Kind: Wikilink, Embed: true, Offset: 2}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Scan = %+v; want %+v", got, want)
		}
	}
}

func TestScanWikilinkEmbedWithSubpathAndAlias(t *testing.T) {
	got := Scan("source.md", []byte("![[Note#^block|label#ignored]]"))
	want := []RawLink{{SourceLogicalPath: "source.md", RawTarget: "Note", Subpath: "#^block", Kind: Wikilink, Embed: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanExcludesInlineCode(t *testing.T) {
	got := Scan("source.md", []byte("[[Before]] `[[Hidden]]` [[After]]"))
	want := []RawLink{
		{SourceLogicalPath: "source.md", RawTarget: "Before", Kind: Wikilink},
		{SourceLogicalPath: "source.md", RawTarget: "After", Kind: Wikilink, Offset: 24},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanInlineCodeMatchingRuns(t *testing.T) {
	for _, input := range []string{"``one ` [[Hidden]] `` [[Visible]]", "``[[Hidden]]```still hidden`` [[Visible]]", "`line one\n[[Hidden]]\n` [[Visible]]"} {
		got := Scan("source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Visible" {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanUnclosedInlineCodeIsText(t *testing.T) {
	for _, input := range []string{"`unclosed [[Visible]]", "``unclosed ` [[Visible]]"} {
		got := Scan("source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Visible" {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanExcludesBacktickFence(t *testing.T) {
	got := Scan("source.md", []byte("```go\n[[Hidden]]\n```\n[[Visible]]"))
	if len(got) != 1 || got[0].RawTarget != "Visible" {
		t.Fatalf("Scan = %+v", got)
	}
}

func TestScanExcludesTildeFence(t *testing.T) {
	got := Scan("source.md", []byte("~~~go\n[[Hidden]]\n~~~\n[[Visible]]"))
	if len(got) != 1 || got[0].RawTarget != "Visible" {
		t.Fatalf("Scan = %+v", got)
	}
}

func TestScanFenceBoundaries(t *testing.T) {
	cases := []struct {
		name, input string
		targets     []string
	}{
		{"short closing", "````\n[[Hidden]]\n```\n[[Hidden2]]\n`````\n[[Visible]]", []string{"Visible"}},
		{"wrong marker", "~~~\n[[Hidden]]\n```\n[[Hidden2]]\n~~~\n[[Visible]]", []string{"Visible"}},
		{"closing suffix", "~~~\n[[Hidden]]\n~~~ suffix\n[[Hidden2]]\n~~~ \t\n[[Visible]]", []string{"Visible"}},
		{"three space CRLF", "   ```go\r\n[[Hidden]]\r\n  ````\t\r\n[[Visible]]", []string{"Visible"}},
		{"one space", " ~~~\n[[Hidden]]\n ~~~\n[[Visible]]", []string{"Visible"}},
		{"unclosed backtick", "```go\n[[Hidden]]", nil},
		{"unclosed tilde", "~~~\n[[Hidden]]", nil},
		{"invalid backtick info", "```bad`info\n[[Visible]]", []string{"Visible"}},
		{"four spaces", "    ~~~\n[[Visible]]", []string{"Visible"}},
		{"not line start", "text ~~~\n[[Visible]]", []string{"Visible"}},
		{"short opener", "~~\n[[Visible]]", []string{"Visible"}},
		{"inline contains fence", "`code\n~~~\n[[Hidden]]\n` [[Visible]]", []string{"Visible"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var targets []string
			for _, link := range Scan("source.md", []byte(test.input)) {
				targets = append(targets, link.RawTarget)
			}
			if !reflect.DeepEqual(targets, test.targets) {
				t.Fatalf("targets = %v; want %v", targets, test.targets)
			}
		})
	}
}

func TestScanExcludesHTMLComment(t *testing.T) {
	got := Scan("source.md", []byte("[[Before]] <!-- [[Hidden]] --> [[After]]"))
	var targets []string
	for _, link := range got {
		targets = append(targets, link.RawTarget)
	}
	if !reflect.DeepEqual(targets, []string{"Before", "After"}) {
		t.Fatalf("targets = %v", targets)
	}
}

func TestScanHTMLCommentBoundaries(t *testing.T) {
	for _, input := range []string{"<!--\n[[Hidden]]\n-->[[Visible]]", "<!-- [[Hidden]] --><!-- [[Hidden2]] -->[[Visible]]", "[[Visible]]<!-- [[Hidden]]"} {
		got := Scan("source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Visible" {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanExcludesObsidianComment(t *testing.T) {
	got := Scan("source.md", []byte("[[Before]] %% [[Hidden]] %% [[After]]"))
	var targets []string
	for _, link := range got {
		targets = append(targets, link.RawTarget)
	}
	if !reflect.DeepEqual(targets, []string{"Before", "After"}) {
		t.Fatalf("targets = %v", targets)
	}
}

func TestScanObsidianCommentBoundaries(t *testing.T) {
	for _, input := range []string{"%%\n[[Hidden]]\n%%[[Visible]]", "%% [[Hidden]] %%%% [[Hidden2]] %%[[Visible]]", "[[Visible]]%% [[Hidden]]"} {
		got := Scan("source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Visible" {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanCommentAndCodeBoundaries(t *testing.T) {
	for _, input := range []string{
		"`<!--` [[Visible]]",
		"`%%` [[Visible]]",
		"```\n<!-- [[Hidden]]\n```\n[[Visible]]",
		"~~~\n%% [[Hidden]]\n~~~\n[[Visible]]",
		"<!-- `\n~~~\n[[Hidden]] -->[[Visible]]",
		"%% `\n~~~\n[[Hidden]] %%[[Visible]]",
		"<!-- <!-- -->[[Visible]] -->",
	} {
		got := Scan("source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Visible" {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanRejectsEmptyWikilinks(t *testing.T) {
	for _, input := range []string{"[[]]", "[[ ]]", "[[\t]]", "[[|label]]"} {
		if links := Scan("source.md", []byte(input)); len(links) != 0 {
			t.Fatalf("%q: links = %+v", input, links)
		}
	}
}

func TestScanMalformedWikilinkRecovery(t *testing.T) {
	for _, input := range []string{"[[Broken\n[[Good]]", "[[Broken\r\n[[Good]]", "[[Broken [[Good]]", "[[Broken\n]] [[Good]]"} {
		got := Scan("source.md", []byte(input))
		if len(got) != 1 || got[0].RawTarget != "Good" {
			t.Fatalf("%q: Scan = %+v", input, got)
		}
	}
}

func TestScanWikilinkOpeningEscape(t *testing.T) {
	for count := 0; count <= 4; count++ {
		input := strings.Repeat("\\", count) + "[[Note]]"
		links := Scan("source.md", []byte(input))
		if count%2 == 1 {
			if len(links) != 0 {
				t.Fatalf("%q: unexpected links %+v", input, links)
			}
		} else if len(links) != 1 || links[0].RawTarget != "Note" || links[0].Offset != count {
			t.Fatalf("%q: links = %+v", input, links)
		}
	}
}

func TestScanWikilinkEmbedEscape(t *testing.T) {
	for count := 0; count <= 4; count++ {
		input := strings.Repeat("\\", count) + "![[Note]]"
		links := Scan("source.md", []byte(input))
		wantEmbed, wantOffset := count%2 == 0, count
		if !wantEmbed {
			wantOffset++
		}
		if len(links) != 1 || links[0].RawTarget != "Note" || links[0].Embed != wantEmbed || links[0].Offset != wantOffset {
			t.Fatalf("%q: links = %+v", input, links)
		}
	}
}

func TestScanTruncatedWikilinks(t *testing.T) {
	input := "[[Note]]"
	for end := 0; end < len(input); end++ {
		if links := Scan("source.md", []byte(input[:end])); len(links) != 0 {
			t.Fatalf("prefix %q: links = %+v", input[:end], links)
		}
	}
}

func TestScanPreservesUnicodeAndSpaces(t *testing.T) {
	input := "前 [[ 日本語 メモ ]] [[é]] [[e\u0301]]"
	got := Scan("source.md", []byte(input))
	want := []RawLink{
		{SourceLogicalPath: "source.md", RawTarget: " 日本語 メモ ", Kind: Wikilink, Offset: len("前 ")},
		{SourceLogicalPath: "source.md", RawTarget: "é", Kind: Wikilink, Offset: len("前 [[ 日本語 メモ ]] ")},
		{SourceLogicalPath: "source.md", RawTarget: "e\u0301", Kind: Wikilink, Offset: len("前 [[ 日本語 メモ ]] [[é]] ")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v; want %+v", got, want)
	}
}

func TestScanKeepsFrontmatterLinks(t *testing.T) {
	links := Scan("source.md", []byte("---\nrelated: \"[[Property]]\"\n---\n[[Body]]"))
	if len(links) != 2 || links[0].RawTarget != "Property" || links[1].RawTarget != "Body" {
		t.Fatalf("links = %+v", links)
	}
}

func TestScanInvalidUTF8AndLiteralBackslash(t *testing.T) {
	content := []byte("\xff[[dir\\name]]\xfe[[\xff]]")
	before := bytes.Clone(content)
	links := Scan("source.md", content)
	if len(links) != 2 || links[0].RawTarget != "dir\\name" || links[0].Offset != 1 || links[1].RawTarget != "\xff" || !bytes.Equal(content, before) {
		t.Fatalf("links = %+v", links)
	}
}
