package query

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"mdlink/internal/diagnostic"
	"mdlink/internal/link"
	"mdlink/internal/markdown"
	"mdlink/internal/root"
)

type Result struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

func Outgoing(catalog *root.Catalog, source root.File, checkFragments bool) ([]Result, []diagnostic.Diagnostic, error) {
	return outgoing(catalog, source, checkFragments, root.ReadMarkdown)
}

func outgoing(catalog *root.Catalog, source root.File, checkFragments bool, read func(root.File) ([]byte, *diagnostic.Diagnostic)) ([]Result, []diagnostic.Diagnostic, error) {
	body, failure := read(source)
	if failure != nil {
		if failure.Code == "unreadable-file" {
			return nil, []diagnostic.Diagnostic{*failure}, fmt.Errorf("cannot read source %q", source.LogicalPath)
		}
		return nil, []diagnostic.Diagnostic{*failure}, nil
	}
	if !utf8.Valid(body) {
		return nil, []diagnostic.Diagnostic{{Code: "invalid-encoding", Source: source.LogicalPath}}, nil
	}
	counts := make(map[string]int)
	var diagnostics []diagnostic.Diagnostic
	scan := markdown.Scan
	if checkFragments {
		scan = markdown.ScanWithRaw
	}
	checker := fragmentChecker{source: source, body: body, read: read}
	for _, raw := range scan(source.LogicalPath, body) {
		result := link.Resolve(catalog, raw)
		if result.Status == link.Resolved {
			counts[result.Target.LogicalPath]++
			if checkFragments {
				if item := checker.check(raw, *result.Target); item != nil {
					diagnostics = append(diagnostics, *item)
				}
			}
			continue
		}
		diagnostics = append(diagnostics, linkDiagnostic(raw, result))
	}
	results := make([]Result, 0, len(counts))
	for name, count := range counts {
		results = append(results, Result{Path: name, Count: count})
	}
	slices.SortFunc(results, func(a, b Result) int { return root.CompareLogicalPaths(a.Path, b.Path) })
	return results, diagnostics, nil
}
