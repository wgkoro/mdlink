package markdown

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestReferenceFormsAndDefinitions(t *testing.T) {
	body := "[text][id] [id][] [id] ![alt][id] ![id][] ![id]\n[id]: <Target.md#Heading> \"title\"\n"
	got := ScanWithRaw("Source.md", []byte(body))
	if len(got) != 6 {
		t.Fatal(got)
	}
	for _, raw := range got {
		if raw.RawTarget != "Target.md" || raw.Subpath != "#Heading" || raw.RawLink != body[raw.Offset:raw.End] {
			t.Fatal(raw)
		}
	}
}

func TestReferenceNoFallbackAndRecovery(t *testing.T) {
	body := "[a][missing] [a][] [a] [a][broken [good](Good.md)\n[a]: A.md\n"
	got := Scan("Source.md", []byte(body))
	if len(got) != 3 || got[0].RawTarget != "A.md" || got[1].RawTarget != "A.md" || got[2].RawTarget != "Good.md" {
		t.Fatal(got)
	}
}

func TestReferenceDefinitionOwnership(t *testing.T) {
	body := "[id]\n[id]: <Target<!--name.md> \"%%literal%%\"\n[bad]: Target.md <!-- [fake](Fake.md) -->\n[good](Good.md)"
	got := Scan("Source.md", []byte(body))
	if len(got) != 2 || got[0].RawTarget != "Target<!--name.md" || got[1].RawTarget != "Good.md" {
		t.Fatal(got)
	}
}

func TestReferenceDefinitionLiteralBoundary(t *testing.T) {
	body := []byte(`<Target%%name.md> "<!--literal-->"`)
	destination, ok := referenceDefinitionDestination(body)
	if !ok || destination.target != "Target%%name.md" {
		t.Fatalf("%+v %v %+v", destination, ok, inlineDestinations([]byte(`[x](`+string(body)+`)`)))
	}
}

func TestReferenceLabelsAndFullDisplay(t *testing.T) {
	for _, test := range []struct {
		body  string
		count int
	}{
		{"[a [b]][id] [][id]\n[id]: Target.md\n", 2},
		{"[" + strings.Repeat("x", 1000) + "][id]\n[id]: Target.md\n", 1},
		{"[Straße] [STRASSE]\n[straße]: Target.md\n", 2},
		{"[ a\t\u00a0 b ] [A B]\n[a b]: Target.md\n", 2},
		{"[a\vb]\n[a b]: Target.md\n", 0},
		{"[e\u0301]\n[é]: Target.md\n", 0},
		{"[a\\[b\\]]\n[a\\[b\\]]: Target.md\n", 1},
		{"[a\\*]\n[a*]: Target.md\n", 0},
		{"[" + strings.Repeat("界", 999) + "]\n[" + strings.Repeat("界", 999) + "]: Target.md\n", 1},
		{"[" + strings.Repeat("界", 1000) + "]\n[" + strings.Repeat("界", 1000) + "]: Target.md\n", 0},
		{"[a][ ] [a][" + strings.Repeat("x", 1000) + "] [a][x[y]]\n[a]: Target.md\n", 0},
		{"[a] [b]\n[a]: Target.md\n[b]: Target.md\n", 2},
		{"[a](Target.md)\n[a]: Other.md\n", 1},
	} {
		got := Scan("Source.md", []byte(test.body))
		if len(got) != test.count {
			t.Fatalf("%.100q: %+v", test.body, got)
		}
		for _, raw := range got {
			if raw.RawTarget != "Target.md" {
				t.Fatal(raw)
			}
		}
	}
}

func TestReferenceDefinitionPriorityAndFailures(t *testing.T) {
	for _, test := range []struct {
		definitions string
		want        []string
	}{
		{"[id]: First.md\n[id]: Second.md\n", []string{"First.md"}},
		{"[id]: https://host/x\n[id]: Second.md\n", nil},
		{"[id]: %GG\n[id]: Second.md\n", nil},
		{"[id]: <unclosed\n[id]: Second.md\n", []string{"Second.md"}},
		{"[id]: Target.md <!--comment-->\n[id]: Second.md\n", []string{"Second.md"}},
		{"[id]: unescaped space.md\n[id]: Second.md\n", []string{"Second.md"}},
		{"[id]:Escaped\\ space.md 'title'\n", []string{`Escaped\ space.md`}},
		{"   [id]: <Space name.md> \"title\"\r\n", []string{"Space name.md"}},
		{"    [id]: Hidden.md\n", nil},
		{"- item\n  [id]: Target.md\n", []string{"Target.md"}},
		{"> [id]: Hidden.md\n", nil},
		{"- [id]: Hidden.md\n", nil},
		{"[id]: ../unsafe.md\n", []string{"../unsafe.md"}},
	} {
		body := "[id]\n" + test.definitions
		got := Scan("Source.md", []byte(body))
		var targets []string
		for _, raw := range got {
			targets = append(targets, raw.RawTarget)
		}
		if !reflect.DeepEqual(targets, test.want) {
			t.Fatalf("%q: %+v", body, got)
		}
	}
}

func TestReferenceCodeCommentAndMalformedDefinition(t *testing.T) {
	for _, definitions := range []string{
		"```\n[id]: Target.md\n```\n", "<!--\n[id]: Target.md\n-->\n", "%%\n[id]: Target.md\n%%\n", "`code\n[id]: Target.md\ncode`\n", "<!--x-->[id]: Target.md\n",
	} {
		got := Scan("Source.md", []byte("[id]\n"+definitions+"[good](Good.md)"))
		if len(got) != 1 || got[0].RawTarget != "Good.md" {
			t.Fatal(definitions, got)
		}
	}
	for _, bad := range []string{"[id]: <unclosed [fake](Fake.md)", "[id]: Target.md \"unclosed [fake](Fake.md)", "[id]: Target.md <!-- [fake](Fake.md) -->"} {
		got := Scan("Source.md", []byte(bad+"\n[id]: Target.md\n[id] [good](Good.md)"))
		if len(got) != 2 || got[0].RawTarget != "Target.md" || got[1].RawTarget != "Good.md" {
			t.Fatal(bad, got)
		}
	}
	got := Scan("Source.md", []byte("[ ]: [good](Good.md)\n"))
	if len(got) != 1 || got[0].RawTarget != "Good.md" {
		t.Fatal(got)
	}
	if got := Scan("Other.md", []byte("[id]")); len(got) != 0 {
		t.Fatal("definition leaked", got)
	}
}

func TestReferenceFragmentMaskDefinitionOwnership(t *testing.T) {
	body := []byte("# [Real][id]\n\n[id]: <Target%%name.md> \"<!--literal-->\"\n")
	if got := BuildFragments(body).Match("#Real"); got != "" {
		t.Fatal(got)
	}
}

func BenchmarkReferenceCandidates(b *testing.B) {
	for _, n := range []int{1000, 10000, 100000} {
		for _, kind := range []string{"uses", "definitions", "malformed"} {
			var body []byte
			switch kind {
			case "uses":
				body = []byte(strings.Repeat("[text][id] ", n) + "\n[id]: Target.md\n")
			case "definitions":
				var text strings.Builder
				for i := 0; i < n; i++ {
					fmt.Fprintf(&text, "[id%d]: Target.md \"title\"\n", i)
				}
				text.WriteString("[id0]")
				body = []byte(text.String())
			case "malformed":
				body = []byte(strings.Repeat("[a][broken ", n) + " [good](Good.md)")
			}
			b.Run(fmt.Sprintf("%s/n=%d", kind, n), func(b *testing.B) {
				b.SetBytes(int64(len(body)))
				b.ReportAllocs()
				for b.Loop() {
					Scan("Source.md", body)
				}
			})
		}
	}
}

func TestUndefinedShortcutPreservesCommentOwnership(t *testing.T) {
	for _, body := range []string{"[unknown <!--]\n[[Target]]\n-->\n", "[x][id]\n[unknown %%]\n[id]: Target.md\n%%\n"} {
		if got := Scan("Source.md", []byte(body)); len(got) != 0 {
			t.Fatal(body, got)
		}
	}
}

func TestShortcutCrossingDelimiterAndDefinitionOrder(t *testing.T) {
	for _, delimiter := range []struct{ open, close string }{{"<!--", "-->"}, {"%%", "%%"}, {"`", "`"}} {
		label := "unknown " + delimiter.open
		definition := "[" + label + "]: Defined.md\n"
		for _, known := range []bool{false, true} {
			for _, before := range []bool{false, true} {
				body := "[" + label + "]\n[[Target]]\n" + delimiter.close + "\n"
				if known {
					if before {
						body = definition + body
					} else {
						body += definition
					}
				}
				if got := Scan("Source.md", []byte(body)); len(got) != 0 {
					t.Fatalf("%q: %+v", body, got)
				}
			}
		}
		// Fully contained comment/code syntax can be owned by a known shortcut.
		complete := "label " + delimiter.open + "literal" + delimiter.close
		definition = "[" + complete + "]: Defined.md\n"
		for _, before := range []bool{false, true} {
			body := "[" + complete + "]\n"
			if before {
				body = definition + body
			} else {
				body += definition
			}
			got := Scan("Source.md", []byte(body))
			if len(got) != 1 || got[0].RawTarget != "Defined.md" {
				t.Fatalf("%q: %+v", body, got)
			}
		}
	}
}

func TestFullReferenceWithoutDefinitionConsumesNestedWikilink(t *testing.T) {
	body := []byte("[x][[[Target]]]")
	if got := Scan("Source.md", body); len(got) != 0 {
		t.Fatalf("Scan = %+v", got)
	}
	if got := ScanWithRaw("Source.md", body); len(got) != 0 {
		t.Fatalf("ScanWithRaw = %+v", got)
	}
}
