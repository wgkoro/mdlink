package query_test

import (
	"path/filepath"
	"testing"

	"mdlink/benchdata/generator"
	"mdlink/internal/link"
	"mdlink/internal/markdown"
	"mdlink/internal/query"
	"mdlink/internal/root"
)

func BenchmarkOutgoing(b *testing.B) { benchmarkQuery(b, false) }

func BenchmarkBacklinks(b *testing.B) { benchmarkQuery(b, true) }

func benchmarkQuery(b *testing.B, backlinks bool) {
	config := generator.Config{
		Notes: 3000, BytesPerNote: 10 * 1024, LinksPerNote: 20,
		DuplicatePercent: 10, UnicodePercent: 20,
		UnresolvedPercent: 5, AmbiguousPercent: 5, Seed: 20260903,
	}
	directory := filepath.Join(b.TempDir(), "fixture")
	stats, err := generator.Generate(directory, config)
	if err != nil {
		b.Fatal(err)
	}
	target, want := stats.Source, stats.Outgoing
	wantDiagnostics := config.LinksPerNote
	for _, count := range want {
		wantDiagnostics -= count
	}
	if backlinks {
		target, want = stats.Target, stats.Backlinks
		wantDiagnostics = stats.Unresolved + stats.Ambiguous
	}
	b.ReportAllocs()
	for b.Loop() {
		files, diagnostics, err := root.Walk(directory, root.Options{})
		if err != nil || len(diagnostics) != 0 || len(files) != stats.Notes {
			b.Fatalf("Walk = %d/%v/%v", len(files), diagnostics, err)
		}
		catalog := root.NewCatalog(files)
		resolved := link.Resolve(catalog, markdown.RawLink{RawTarget: target, Kind: markdown.Wikilink})
		if resolved.Status != link.Resolved || resolved.Target == nil {
			b.Fatalf("target = %+v", resolved)
		}
		var results []query.Result
		if backlinks {
			results, diagnostics = query.Backlinks(catalog, *resolved.Target)
		} else {
			results, diagnostics, err = query.Outgoing(catalog, *resolved.Target, false)
		}
		if err != nil || len(results) != len(want) || len(diagnostics) != wantDiagnostics {
			b.Fatalf("results/diagnostics/error = %d/%d/%v", len(results), len(diagnostics), err)
		}
		for _, result := range results {
			if count, exists := want[result.Path]; !exists || count != result.Count {
				b.Fatalf("unexpected result: %+v", result)
			}
		}
	}
}
