package query

import (
	"mdlink/internal/diagnostic"
	"mdlink/internal/link"
	"mdlink/internal/markdown"
)

func linkDiagnostic(raw markdown.RawLink, result link.Resolution) diagnostic.Diagnostic {
	code := "unresolved-link"
	if result.Status == link.Ambiguous {
		code = "ambiguous-link"
	} else if result.Status == link.Unsafe {
		code = "unsafe-path"
	}
	offset, target := raw.Offset, raw.RawTarget
	item := diagnostic.Diagnostic{Code: code, Source: raw.SourceLogicalPath, Offset: &offset, RawTarget: &target}
	for _, candidate := range result.Candidates {
		item.Candidates = append(item.Candidates, candidate.LogicalPath)
	}
	return item
}
