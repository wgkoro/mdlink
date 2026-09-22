package output

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"mdlink/internal/diagnostic"
	"mdlink/internal/query"
)

func UnresolvedJSON(stdout io.Writer, results []query.UnresolvedResult, diagnostics []diagnostic.Diagnostic, sources []string) error {
	for _, name := range append(unresolvedPaths(results, diagnostics), sources...) {
		if !utf8.ValidString(name) {
			return errors.New("JSON path is not valid UTF-8")
		}
	}
	if results == nil {
		results = []query.UnresolvedResult{}
	}
	if diagnostics == nil {
		diagnostics = []diagnostic.Diagnostic{}
	}
	sortDiagnostics(diagnostics)
	response := struct {
		SchemaVersion int                      `json:"schema_version"`
		Command       string                   `json:"command"`
		Root          string                   `json:"root"`
		Sources       *[]string                `json:"sources,omitempty"`
		Results       []query.UnresolvedResult `json:"results"`
		Diagnostics   []diagnostic.Diagnostic  `json:"diagnostics"`
	}{SchemaVersion: 1, Command: "unresolved", Root: ".", Results: results, Diagnostics: diagnostics}
	if sources != nil {
		response.Sources = &sources
	}
	return json.NewEncoder(stdout).Encode(response)
}

func UnresolvedText(stdout io.Writer, results []query.UnresolvedResult, diagnostics []diagnostic.Diagnostic, counts bool) error {
	for _, name := range unresolvedPaths(results, diagnostics) {
		if strings.ContainsAny(name, "\r\n\t") {
			return ErrTextPath
		}
	}
	rows := make([]query.Result, 0, len(results))
	for _, result := range results {
		rows = append(rows, query.Result{Path: result.Target, Count: result.Count})
	}
	return Text(stdout, "", rows, nil, counts)
}

func unresolvedPaths(results []query.UnresolvedResult, diagnostics []diagnostic.Diagnostic) []string {
	paths := resultPaths("", nil, diagnostics)
	for _, result := range results {
		paths = append(paths, result.Target)
		for _, source := range result.Sources {
			paths = append(paths, source.Path)
		}
	}
	return paths
}
