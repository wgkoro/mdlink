package query

import (
	"cmp"
	"slices"
	"unicode/utf8"

	"mdlink/internal/diagnostic"
	"mdlink/internal/link"
	"mdlink/internal/markdown"
	"mdlink/internal/root"
)

type Occurrence struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
}

type UnresolvedResult struct {
	Target  string       `json:"target"`
	Count   int          `json:"count"`
	Sources []Occurrence `json:"sources"`
}

func Unresolved(catalog *root.Catalog, sources []*root.File, checkFragments bool) ([]UnresolvedResult, []diagnostic.Diagnostic) {
	return unresolved(catalog, sources, checkFragments, root.ReadMarkdown)
}

func unresolved(catalog *root.Catalog, sources []*root.File, checkFragments bool, read func(root.File) ([]byte, *diagnostic.Diagnostic)) ([]UnresolvedResult, []diagnostic.Diagnostic) {
	occurrences := make(map[string][]Occurrence)
	var diagnostics []diagnostic.Diagnostic
	if sources == nil {
		sources = catalog.MarkdownFiles
	}
	for _, source := range sources {
		body, failure := read(*source)
		if failure != nil {
			diagnostics = append(diagnostics, *failure)
			continue
		}
		if !utf8.Valid(body) {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Code: "invalid-encoding", Source: source.LogicalPath})
			continue
		}
		scan := markdown.Scan
		if checkFragments {
			scan = markdown.ScanWithRaw
		}
		checker := fragmentChecker{source: *source, body: body, read: read}
		for _, raw := range scan(source.LogicalPath, body) {
			resolved := link.Resolve(catalog, raw)
			if resolved.Status == link.Resolved {
				if checkFragments {
					if item := checker.check(raw, *resolved.Target); item != nil {
						diagnostics = append(diagnostics, *item)
					}
				}
				continue
			}
			diagnostics = append(diagnostics, linkDiagnostic(raw, resolved))
			if resolved.Status == link.Unresolved {
				occurrences[raw.RawTarget] = append(occurrences[raw.RawTarget], Occurrence{Path: source.LogicalPath, Offset: raw.Offset})
			}
		}
	}
	results := make([]UnresolvedResult, 0, len(occurrences))
	for target, sources := range occurrences {
		slices.SortFunc(sources, func(a, b Occurrence) int {
			if order := root.CompareLogicalPaths(a.Path, b.Path); order != 0 {
				return order
			}
			return cmp.Compare(a.Offset, b.Offset)
		})
		results = append(results, UnresolvedResult{Target: target, Count: len(sources), Sources: sources})
	}
	slices.SortFunc(results, func(a, b UnresolvedResult) int { return root.CompareLogicalPaths(a.Target, b.Target) })
	return results, diagnostics
}
