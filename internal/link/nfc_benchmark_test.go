package link

import (
	"fmt"
	"path"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
	"mdlink/internal/markdown"
	"mdlink/internal/root"
)

func missingNFCCatalog(prefix string, count int) *root.Catalog {
	files := make([]root.File, count)
	for i := range files {
		files[i].LogicalPath = fmt.Sprintf("notes/%s-%04d.md", prefix, i)
	}
	return root.NewCatalog(files)
}

func TestMissingNFCFixture(t *testing.T) {
	for _, prefix := range []string{"sample", "架空"} {
		for _, count := range []int{100, 1000, 3000} {
			t.Run(fmt.Sprintf("%s/N=%d", prefix, count), func(t *testing.T) {
				catalog := missingNFCCatalog(prefix, count)
				if len(catalog.ByExactPath) != count || len(catalog.ByNormalizedPath) != count ||
					len(catalog.ByBaseName) != count || len(catalog.ByStem) != count {
					t.Fatal("fixture must have N distinct keys in every index")
				}
				for i := range count {
					logical := fmt.Sprintf("notes/%s-%04d.md", prefix, i)
					ascii := fmt.Sprintf("notes/sample-%04d.md", i)
					if len(logical) != len(ascii) || len(path.Base(logical)) != len(path.Base(ascii)) || !norm.NFC.IsNormalString(logical) {
						t.Fatalf("fixture byte length or NFC differs: %q", logical)
					}
					file := catalog.ByExactPath[logical]
					if file == nil || len(catalog.ByNormalizedPath[logical]) != 1 ||
						len(catalog.ByBaseName[path.Base(logical)]) != 1 ||
						len(catalog.ByStem[strings.TrimSuffix(path.Base(logical), ".md")]) != 1 {
						t.Fatalf("missing or nonunique fixture key: %q", logical)
					}
				}
				for _, target := range []string{"missing-note.md", "missing-note"} {
					if catalog.ByExactPath[target] != nil || len(catalog.ByNormalizedPath[target]) != 0 ||
						len(catalog.ByBaseName[target]) != 0 || len(catalog.ByStem[target]) != 0 {
						t.Fatal("missing target exists in fixture")
					}
					got := Resolve(catalog, markdown.RawLink{RawTarget: target, Kind: markdown.Wikilink})
					if got.Status != Unresolved || got.Target != nil || len(got.Candidates) != 0 {
						t.Fatalf("missing target resolution = %+v", got)
					}
				}
			})
		}
	}
}

func BenchmarkResolveMissingNFC(b *testing.B) {
	for _, names := range []struct{ label, prefix string }{{"ASCII", "sample"}, {"Japanese", "架空"}} {
		for _, count := range []int{100, 1000, 3000} {
			for _, target := range []string{"missing-note.md", "missing-note"} {
				b.Run(fmt.Sprintf("%s/N=%d/target=%s", names.label, count, target), func(b *testing.B) {
					catalog := missingNFCCatalog(names.prefix, count)
					raw := markdown.RawLink{RawTarget: target, Kind: markdown.Wikilink}
					b.ReportAllocs()
					for b.Loop() {
						got := Resolve(catalog, raw)
						if got.Status != Unresolved || got.Target != nil || len(got.Candidates) != 0 {
							b.Fatalf("missing target resolution = %+v", got)
						}
					}
				})
			}
		}
	}
}
