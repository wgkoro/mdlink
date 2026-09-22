package query

import (
	"slices"
	"sync"
	"unicode/utf8"

	"mdlink/internal/diagnostic"
	"mdlink/internal/link"
	"mdlink/internal/markdown"
	"mdlink/internal/root"
)

func Backlinks(catalog *root.Catalog, target root.File) ([]Result, []diagnostic.Diagnostic) {
	var results []Result
	var diagnostics []diagnostic.Diagnostic
	type sourceResult struct {
		count    int
		observed []diagnostic.Diagnostic
	}
	slots := make([]sourceResult, len(catalog.MarkdownFiles))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for i := range jobs {
				source := catalog.MarkdownFiles[i]
				slots[i].count, slots[i].observed = backlinksFromSource(catalog, *source, target.LogicalPath)
			}
		})
	}
	for i := range catalog.MarkdownFiles {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	for i, slot := range slots {
		diagnostics = append(diagnostics, slot.observed...)
		if slot.count > 0 {
			results = append(results, Result{Path: catalog.MarkdownFiles[i].LogicalPath, Count: slot.count})
		}
	}

	slices.SortFunc(results, func(a, b Result) int { return root.CompareLogicalPaths(a.Path, b.Path) })
	return results, diagnostics
}

func backlinksFromSource(catalog *root.Catalog, source root.File, target string) (int, []diagnostic.Diagnostic) {
	body, failure := root.ReadMarkdown(source)
	if failure != nil {
		return 0, []diagnostic.Diagnostic{*failure}
	}
	if !utf8.Valid(body) {
		return 0, []diagnostic.Diagnostic{{Code: "invalid-encoding", Source: source.LogicalPath}}
	}
	count := 0
	var diagnostics []diagnostic.Diagnostic
	for _, raw := range markdown.Scan(source.LogicalPath, body) {
		resolved := link.Resolve(catalog, raw)
		if resolved.Status == link.Resolved {
			if resolved.Target.LogicalPath == target {
				count++
			}
		} else {
			diagnostics = append(diagnostics, linkDiagnostic(raw, resolved))
		}
	}
	return count, diagnostics
}
