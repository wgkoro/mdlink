package root

import (
	"golang.org/x/text/unicode/norm"
	"path"
	"slices"
	"strings"
)

type Catalog struct {
	ByExactPath          map[string]*File
	ByNormalizedPath     map[string][]*File
	ByBaseName           map[string][]*File
	ByStem               map[string][]*File
	ByNormalizedBaseName map[string][]*File
	ByNormalizedStem     map[string][]*File
	MarkdownFiles        []*File
}

func NewCatalog(files []File) *Catalog {
	files = slices.Clone(files)
	slices.SortFunc(files, func(a, b File) int {
		return CompareLogicalPaths(a.LogicalPath, b.LogicalPath)
	})
	catalog := &Catalog{
		ByExactPath: make(map[string]*File), ByNormalizedPath: make(map[string][]*File),
		ByBaseName: make(map[string][]*File), ByStem: make(map[string][]*File),
		ByNormalizedBaseName: make(map[string][]*File), ByNormalizedStem: make(map[string][]*File),
	}
	for _, file := range files {
		catalog.ByExactPath[file.LogicalPath] = &file
		normalized := norm.NFC.String(file.LogicalPath)
		catalog.ByNormalizedPath[normalized] = append(catalog.ByNormalizedPath[normalized], &file)
		base := path.Base(file.LogicalPath)
		catalog.ByBaseName[base] = append(catalog.ByBaseName[base], &file)
		normalizedBase := norm.NFC.String(base)
		catalog.ByNormalizedBaseName[normalizedBase] = append(catalog.ByNormalizedBaseName[normalizedBase], &file)
		if path.Ext(file.LogicalPath) == ".md" {
			stem := base[:len(base)-len(".md")]
			catalog.ByStem[stem] = append(catalog.ByStem[stem], &file)
			normalizedStem := norm.NFC.String(stem)
			catalog.ByNormalizedStem[normalizedStem] = append(catalog.ByNormalizedStem[normalizedStem], &file)
			catalog.MarkdownFiles = append(catalog.MarkdownFiles, &file)
		}
	}
	return catalog
}

func CompareLogicalPaths(a, b string) int {
	if order := strings.Compare(norm.NFC.String(a), norm.NFC.String(b)); order != 0 {
		return order
	}
	return strings.Compare(a, b)
}
