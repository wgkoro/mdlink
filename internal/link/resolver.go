package link

import (
	"mdlink/internal/markdown"
	"mdlink/internal/root"
	"path"
	"slices"
	"strings"

	"golang.org/x/text/unicode/norm"
)

type ResolutionStatus string

const (
	Resolved   ResolutionStatus = "resolved"
	Unresolved ResolutionStatus = "unresolved"
	Ambiguous  ResolutionStatus = "ambiguous"
	Unsafe     ResolutionStatus = "unsafe"
)

type Resolution struct {
	Status     ResolutionStatus
	Target     *root.File
	Candidates []*root.File
}

func Resolve(catalog *root.Catalog, raw markdown.RawLink) Resolution {
	if raw.Kind == markdown.MarkdownLink {
		var ok bool
		raw.RawTarget, ok = markdown.DecodeInternalTarget(raw.RawTarget)
		if !ok {
			return Resolution{Status: Unresolved}
		}
	}
	if raw.RawTarget == "" {
		if raw.Subpath == "" || raw.SourceLogicalPath == "" {
			return Resolution{Status: Unresolved}
		}
		source, err := root.CleanLogicalPath(raw.SourceLogicalPath)
		if err != nil {
			return Resolution{Status: Unsafe}
		}
		if file := catalog.ByExactPath[source]; file != nil && path.Ext(file.LogicalPath) != "" {
			return Resolution{Status: Resolved, Target: file}
		}
		return Resolution{Status: Unresolved}
	}
	explicit := false
	if raw.Kind == markdown.MarkdownLink {
		if strings.ContainsRune(raw.RawTarget, 0) {
			return Resolution{Status: Unsafe}
		}
		if strings.HasPrefix(raw.RawTarget, "/") {
			raw.RawTarget = strings.TrimPrefix(raw.RawTarget, "/")
			explicit = true
		} else {
			for component := range strings.SplitSeq(raw.RawTarget, "/") {
				if component == "." || component == ".." {
					relative, err := sourceRelativePath(raw.SourceLogicalPath, raw.RawTarget)
					if err != nil {
						return Resolution{Status: Unsafe}
					}
					raw.RawTarget = relative
					explicit = true
					break
				}
			}
		}
	}
	targetPath, err := root.CleanLogicalPath(raw.RawTarget)
	if err != nil {
		return Resolution{Status: Unsafe}
	}
	if target := catalog.ByExactPath[targetPath]; target != nil && path.Ext(target.LogicalPath) != "" {
		return Resolution{Status: Resolved, Target: target}
	}
	sourcePath := ""
	if raw.Kind == markdown.MarkdownLink && !explicit {
		sourcePath, err = sourceRelativePath(raw.SourceLogicalPath, targetPath)
		if err != nil {
			return Resolution{Status: Unsafe}
		}
		if target := catalog.ByExactPath[sourcePath]; target != nil && path.Ext(target.LogicalPath) != "" {
			return Resolution{Status: Resolved, Target: target}
		}
	}
	if !strings.HasSuffix(targetPath, ".md") {
		if target := catalog.ByExactPath[targetPath+".md"]; target != nil {
			return Resolution{Status: Resolved, Target: target}
		}
	}
	if !explicit && !strings.Contains(targetPath, "/") {
		files := slices.Clone(catalog.ByBaseName[targetPath])
		if !strings.HasSuffix(targetPath, ".md") {
			files = append(files, catalog.ByStem[targetPath]...)
		}
		if result := resolveCandidates(files); result.Status != Unresolved {
			return result
		}
	}
	normalized := norm.NFC.String(targetPath)
	if result := resolveCandidates(catalog.ByNormalizedPath[normalized]); result.Status != Unresolved {
		return result
	}
	if sourcePath != "" {
		if result := resolveCandidates(catalog.ByNormalizedPath[norm.NFC.String(sourcePath)]); result.Status != Unresolved {
			return result
		}
	}
	if !strings.HasSuffix(normalized, ".md") {
		if result := resolveCandidates(catalog.ByNormalizedPath[normalized+".md"]); result.Status != Unresolved {
			return result
		}
	}
	if !explicit && !strings.Contains(normalized, "/") {
		files := slices.Clone(catalog.ByNormalizedBaseName[normalized])
		if !strings.HasSuffix(normalized, ".md") {
			files = append(files, catalog.ByNormalizedStem[normalized]...)
		}
		return resolveCandidates(files)
	}
	return Resolution{Status: Unresolved}
}

func sourceRelativePath(source, target string) (string, error) {
	if source != "" {
		var err error
		source, err = root.CleanLogicalPath(source)
		if err != nil {
			return "", err
		}
	}
	return path.Join(path.Dir(source), target), nil
}

func resolveCandidates(files []*root.File) Resolution {
	seen := make(map[string]bool, len(files))
	var candidates []*root.File
	for _, file := range files {
		if path.Ext(file.LogicalPath) == "" || seen[file.LogicalPath] {
			continue
		}
		seen[file.LogicalPath] = true
		candidates = append(candidates, file)
	}
	slices.SortFunc(candidates, func(a, b *root.File) int {
		return root.CompareLogicalPaths(a.LogicalPath, b.LogicalPath)
	})
	if len(candidates) == 1 {
		return Resolution{Status: Resolved, Target: candidates[0]}
	}
	if len(candidates) > 1 {
		return Resolution{Status: Ambiguous, Candidates: candidates}
	}
	return Resolution{Status: Unresolved}
}
