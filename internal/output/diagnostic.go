package output

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"mdlink/internal/diagnostic"
	"mdlink/internal/root"
)

func sortDiagnostics(items []diagnostic.Diagnostic) {
	slices.SortFunc(items, func(a, b diagnostic.Diagnostic) int {
		if order := root.CompareLogicalPaths(a.Source, b.Source); order != 0 {
			return order
		}
		aOffset, bOffset := -1, -1
		if a.Offset != nil {
			aOffset = *a.Offset
		}
		if b.Offset != nil {
			bOffset = *b.Offset
		}
		if order := cmp.Compare(aOffset, bOffset); order != 0 {
			return order
		}
		if order := strings.Compare(a.Code, b.Code); order != 0 {
			return order
		}
		aRaw, bRaw := "", ""
		if a.RawTarget != nil {
			aRaw = *a.RawTarget
		}
		if b.RawTarget != nil {
			bRaw = *b.RawTarget
		}
		if order := strings.Compare(aRaw, bRaw); order != 0 {
			return order
		}
		if order := slices.CompareFunc(a.Candidates, b.Candidates, root.CompareLogicalPaths); order != 0 {
			return order
		}
		for _, pair := range [][2]string{{a.Phase, b.Phase}, {a.Fragment, b.Fragment}, {a.Target, b.Target}, {a.Reason, b.Reason}, {a.RawLink, b.RawLink}} {
			if order := strings.Compare(pair[0], pair[1]); order != 0 {
				return order
			}
		}
		return 0
	})
}

func Diagnostics(stderr io.Writer, items []diagnostic.Diagnostic) error {
	if len(items) == 0 {
		return nil
	}
	sortDiagnostics(items)
	for _, item := range items[:min(len(items), 100)] {
		line := fmt.Sprintf("%s: %q", item.Code, item.Source)
		if item.Phase != "" {
			line += fmt.Sprintf(" phase=%s", item.Phase)
		}
		if item.Offset != nil {
			line += fmt.Sprintf(" offset=%d", *item.Offset)
		}
		if item.RawTarget != nil {
			line += fmt.Sprintf(" raw_target=%q", *item.RawTarget)
		}
		for _, field := range []struct{ name, value string }{{"raw_link", item.RawLink}, {"fragment", item.Fragment}, {"target", item.Target}, {"reason", item.Reason}} {
			if field.value != "" {
				line += fmt.Sprintf(" %s=%q", field.name, field.value)
			}
		}
		if len(item.Candidates) > 0 {
			line += fmt.Sprintf(" candidates=%q", item.Candidates)
		}
		if _, err := fmt.Fprintln(stderr, line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(stderr, "diagnostics: %d\n", len(items))
	return err
}
