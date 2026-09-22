package output

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"mdlink/internal/diagnostic"
	"mdlink/internal/query"
)

var ErrTextPath = errors.New("path contains CR, LF, or tab; use --format json")

func Text(stdout io.Writer, target string, results []query.Result, diagnostics []diagnostic.Diagnostic, counts bool) error {
	for _, name := range resultPaths(target, results, diagnostics) {
		if strings.ContainsAny(name, "\r\n\t") {
			return ErrTextPath
		}
	}
	for _, result := range results {
		line := result.Path
		if counts {
			line = fmt.Sprintf("%d\t%s", result.Count, line)
		}
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

func resultPaths(target string, results []query.Result, diagnostics []diagnostic.Diagnostic) []string {
	paths := []string{target}
	for _, result := range results {
		paths = append(paths, result.Path)
	}
	for _, item := range diagnostics {
		paths = append(paths, item.Source, item.Target)
		paths = append(paths, item.Candidates...)
	}
	return paths
}
