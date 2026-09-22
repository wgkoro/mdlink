package output

import (
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"mdlink/internal/diagnostic"
	"mdlink/internal/query"
)

func JSON(stdout io.Writer, command, target string, results []query.Result, diagnostics []diagnostic.Diagnostic) error {
	for _, name := range resultPaths(target, results, diagnostics) {
		if !utf8.ValidString(name) {
			return errors.New("JSON path is not valid UTF-8")
		}
	}
	if results == nil {
		results = []query.Result{}
	}
	if diagnostics == nil {
		diagnostics = []diagnostic.Diagnostic{}
	}
	sortDiagnostics(diagnostics)
	response := struct {
		SchemaVersion int                     `json:"schema_version"`
		Command       string                  `json:"command"`
		Root          string                  `json:"root"`
		Target        string                  `json:"target"`
		Results       []query.Result          `json:"results"`
		Diagnostics   []diagnostic.Diagnostic `json:"diagnostics"`
	}{1, command, ".", target, results, diagnostics}
	return json.NewEncoder(stdout).Encode(response)
}
