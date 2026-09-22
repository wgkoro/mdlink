package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"mdlink/internal/root"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

var errInvalidSources = errors.New("invalid source list")

func readSources0(input io.Reader) ([]string, error) {
	reader := bufio.NewReader(input)
	names := []string{}
	for {
		record, err := reader.ReadString(0)
		if err != nil {
			if err == io.EOF {
				if record == "" {
					return names, nil
				}
				return nil, errInvalidSources
			}
			return nil, err
		}
		name := record[:len(record)-1]
		if name == "" || !utf8.ValidString(name) {
			return nil, errInvalidSources
		}
		names = append(names, name)
	}
}

func validSourceName(name string) bool {
	if !utf8.ValidString(name) || name == "" || strings.ContainsRune(name, 0) || path.Ext(name) != ".md" {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func selectSources(catalog *root.Catalog, names []string) ([]*root.File, []string, error) {
	unique := make(map[string]*root.File)
	for _, name := range names {
		file := catalog.ByExactPath[name]
		if !validSourceName(name) || file == nil {
			return nil, nil, fmt.Errorf("invalid source: %q", name)
		}
		unique[name] = file
	}
	names = make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	slices.SortFunc(names, root.CompareLogicalPaths)
	files := make([]*root.File, 0, len(names))
	for _, name := range names {
		files = append(files, unique[name])
	}
	return files, names, nil
}
