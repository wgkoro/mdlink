package markdown

import (
	"reflect"
	"testing"
)

func TestScanMarkdownImage(t *testing.T) {
	for _, tt := range []struct {
		input, target, subpath string
		embed                  bool
		offset                 int
	}{
		{"![alt](image.png)", "image.png", "", true, 0},
		{"前 ![alt](image.png)", "image.png", "", true, 4},
		{"![alt](image%20name.png#Part)", "image%20name.png", "#Part", true, 0},
		{"![alt](<image name.png>)", "image name.png", "", true, 0},
		{"![alt]( image name.png?view=1#Part )", "image name.png", "#Part", true, 0},
		{`\![alt](image.png)`, "image.png", "", false, 2},
		{`\\![alt](image.png)`, "image.png", "", true, 2},
		{`!\\[alt](image.png)`, "image.png", "", false, 3},
	} {
		got := Scan("Source.md", []byte(tt.input))
		want := []RawLink{{SourceLogicalPath: "Source.md", RawTarget: tt.target, Subpath: tt.subpath, Kind: MarkdownLink, Embed: tt.embed, Offset: tt.offset}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q: Scan = %+v; want %+v", tt.input, got, want)
		}
	}
	for _, input := range []string{`!\[alt](image.png)`, `\\!\[alt](image.png)`, `\[alt](image.png)`} {
		if got := Scan("Source.md", []byte(input)); len(got) != 0 {
			t.Fatalf("escaped opening %q: %+v", input, got)
		}
	}
}
