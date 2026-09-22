package query

import (
	"mdlink/internal/diagnostic"
	"mdlink/internal/markdown"
	"mdlink/internal/root"
	"path"
	"unicode/utf8"
)

type fragmentTarget struct {
	index   markdown.FragmentIndex
	failure string
}

type fragmentChecker struct {
	source  root.File
	body    []byte
	read    func(root.File) ([]byte, *diagnostic.Diagnostic)
	targets map[string]fragmentTarget
}

func (checker *fragmentChecker) check(raw markdown.RawLink, target root.File) *diagnostic.Diagnostic {
	if raw.Subpath == "" {
		return nil
	}
	item := diagnostic.Diagnostic{Code: "unsupported-fragment", Phase: "fragment", Source: raw.SourceLogicalPath, Offset: &raw.Offset, RawTarget: &raw.RawTarget, RawLink: raw.RawLink, Fragment: raw.Subpath, Target: target.LogicalPath}
	if raw.Kind != markdown.Wikilink || !markdown.ValidFragment(raw.Subpath) {
		item.Reason = "syntax"
		return &item
	}
	if path.Ext(target.LogicalPath) != ".md" {
		item.Reason = "target-format"
		return &item
	}
	cached, ok := checker.targets[target.LogicalPath]
	if !ok {
		body := checker.body
		if target.LogicalPath != checker.source.LogicalPath {
			var failure *diagnostic.Diagnostic
			body, failure = checker.read(target)
			if failure != nil {
				cached.failure = failure.Code
			}
		}
		if cached.failure == "" {
			if !utf8.Valid(body) {
				cached.failure = "invalid-encoding"
			} else {
				cached.index = markdown.BuildFragments(body)
			}
		}
		if checker.targets == nil {
			checker.targets = make(map[string]fragmentTarget)
		}
		checker.targets[target.LogicalPath] = cached
	}
	if cached.failure != "" {
		item.Code = "unverifiable-fragment"
		item.Reason = cached.failure
		return &item
	}
	item.Code = cached.index.Match(raw.Subpath)
	if item.Code == "" {
		return nil
	}
	if item.Code == "unsupported-fragment" {
		item.Reason = "target-syntax"
	}
	return &item
}
