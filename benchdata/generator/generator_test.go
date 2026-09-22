package generator_test

import (
	"fmt"
	"mdlink/internal/markdown"
	"mdlink/internal/query"
	"mdlink/internal/root"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mdlink/benchdata/generator"
)

func contents(t *testing.T, directory string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(directory, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, name)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestGenerateRatesAndQueryExpectations(t *testing.T) {
	config := generator.Config{Notes: 10, BytesPerNote: 1024, LinksPerNote: 5, DuplicatePercent: 30, UnicodePercent: 30, UnresolvedPercent: 12, AmbiguousPercent: 10, Seed: 20260903}
	directory := filepath.Join(t.TempDir(), "fixture")
	stats, err := generator.Generate(directory, config)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Notes != 10 || stats.Bytes != 10240 || stats.Links != 50 || stats.DuplicateNotes != 2 || stats.UnicodeNotes != 3 || stats.Unresolved != 6 || stats.Ambiguous != 5 || stats.Resolved != 39 {
		t.Fatalf("stats = %+v", stats)
	}
	names := make(map[string]int)
	unicodeNotes := 0
	for name, body := range contents(t, directory) {
		names[path.Base(name)]++
		if strings.Contains(name, "日本 語") {
			unicodeNotes++
		}
		if len(body) != config.BytesPerNote || len(markdown.Scan(name, []byte(body))) != config.LinksPerNote {
			t.Fatalf("unexpected body size/links for %s", name)
		}
	}
	duplicates := 0
	for _, count := range names {
		if count > 1 {
			duplicates += count
		}
	}
	if duplicates != 2 || unicodeNotes != 3 {
		t.Fatalf("duplicate/Unicode notes = %d/%d", duplicates, unicodeNotes)
	}
	files, walkDiagnostics, err := root.Walk(directory, root.Options{})
	if err != nil || len(walkDiagnostics) != 0 || len(files) != 10 {
		t.Fatalf("Walk = %d/%v/%v", len(files), walkDiagnostics, err)
	}
	catalog := root.NewCatalog(files)
	totals := make(map[string]int)
	for _, source := range catalog.MarkdownFiles {
		results, diagnostics, err := query.Outgoing(catalog, *source, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, result := range results {
			totals["resolved"] += result.Count
		}
		for _, diagnostic := range diagnostics {
			totals[diagnostic.Code]++
		}
		if source.LogicalPath == stats.Source && !reflect.DeepEqual(resultCounts(results), stats.Outgoing) {
			t.Fatalf("outgoing = %v, want %v", results, stats.Outgoing)
		}
	}
	if !reflect.DeepEqual(totals, map[string]int{"resolved": 39, "unresolved-link": 6, "ambiguous-link": 5}) {
		t.Fatalf("totals = %v", totals)
	}
	results, diagnostics := query.Backlinks(catalog, *catalog.ByExactPath[stats.Target])
	if !reflect.DeepEqual(resultCounts(results), stats.Backlinks) || len(diagnostics) != 11 {
		t.Fatalf("backlinks/diagnostics = %v/%d, want %v/11", results, len(diagnostics), stats.Backlinks)
	}
	second := filepath.Join(t.TempDir(), "fixture")
	repeated, err := generator.Generate(second, config)
	if err != nil || !reflect.DeepEqual(stats, repeated) || !reflect.DeepEqual(contents(t, directory), contents(t, second)) {
		t.Fatalf("seed mismatch: %v", err)
	}
}

func resultCounts(results []query.Result) map[string]int {
	counts := make(map[string]int)
	for _, result := range results {
		counts[result.Path] = result.Count
	}
	return counts
}

func TestGenerateRejectsInvalidConfig(t *testing.T) {
	base := generator.Config{Notes: 2, BytesPerNote: 128, Seed: 20260903}
	cases := []struct {
		name   string
		change func(*generator.Config)
	}{
		{"zero notes", func(c *generator.Config) { c.Notes = 0 }},
		{"negative notes", func(c *generator.Config) { c.Notes = -1 }},
		{"too many notes", func(c *generator.Config) { c.Notes = 10001 }},
		{"huge notes", func(c *generator.Config) { c.Notes = int(^uint(0) >> 1) }},
		{"zero bytes", func(c *generator.Config) { c.BytesPerNote = 0 }},
		{"negative bytes", func(c *generator.Config) { c.BytesPerNote = -1 }},
		{"too many bytes", func(c *generator.Config) { c.BytesPerNote = 50*1024 + 1 }},
		{"negative links", func(c *generator.Config) { c.LinksPerNote = -1 }},
		{"too many links", func(c *generator.Config) { c.LinksPerNote = 101 }},
		{"negative duplicate", func(c *generator.Config) { c.DuplicatePercent = -1 }},
		{"excess duplicate", func(c *generator.Config) { c.DuplicatePercent = 101 }},
		{"negative Unicode", func(c *generator.Config) { c.UnicodePercent = -1 }},
		{"excess Unicode", func(c *generator.Config) { c.UnicodePercent = 101 }},
		{"negative unresolved", func(c *generator.Config) { c.UnresolvedPercent = -1 }},
		{"excess unresolved", func(c *generator.Config) { c.UnresolvedPercent = 101 }},
		{"negative ambiguous", func(c *generator.Config) { c.AmbiguousPercent = -1 }},
		{"excess ambiguous", func(c *generator.Config) { c.AmbiguousPercent = 101 }},
		{"combined rates", func(c *generator.Config) { c.UnresolvedPercent = 60; c.AmbiguousPercent = 41 }},
		{"ambiguous without pair", func(c *generator.Config) { c.LinksPerNote = 1; c.AmbiguousPercent = 100 }},
		{"body does not fit", func(c *generator.Config) { c.BytesPerNote = 1; c.LinksPerNote = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if value := recover(); value != nil {
					t.Fatalf("invalid config panicked: %v", value)
				}
			}()
			config := base
			tc.change(&config)
			directory := filepath.Join(t.TempDir(), "fixture")
			stats, err := generator.Generate(directory, config)
			if err == nil || !reflect.DeepEqual(stats, generator.Stats{}) {
				t.Fatalf("stats/error = %+v/%v", stats, err)
			}
			if _, err := os.Lstat(directory); !os.IsNotExist(err) {
				t.Fatalf("invalid config created output: %v", err)
			}
		})
	}
}

func TestGenerateRoundingAndZeroLinks(t *testing.T) {
	cases := []struct {
		name                                                string
		config                                              generator.Config
		duplicate, unicode, resolved, unresolved, ambiguous int
	}{
		{"odd duplicate and floor", generator.Config{Notes: 3, BytesPerNote: 128, LinksPerNote: 1, DuplicatePercent: 100, UnicodePercent: 34, UnresolvedPercent: 34, AmbiguousPercent: 34}, 2, 1, 1, 1, 1},
		{"ambiguous rounds to zero", generator.Config{Notes: 3, BytesPerNote: 128, LinksPerNote: 1, AmbiguousPercent: 1}, 0, 0, 3, 0, 0},
		{"zero links", generator.Config{Notes: 1, BytesPerNote: 1, AmbiguousPercent: 100, UnicodePercent: 100}, 0, 1, 0, 0, 0},
		{"no resolved", generator.Config{Notes: 2, BytesPerNote: 128, LinksPerNote: 2, DuplicatePercent: 100, UnresolvedPercent: 50, AmbiguousPercent: 50}, 2, 0, 0, 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "fixture")
			stats, err := generator.Generate(directory, tc.config)
			if err != nil {
				t.Fatal(err)
			}
			if stats.DuplicateNotes != tc.duplicate || stats.UnicodeNotes != tc.unicode || stats.Resolved != tc.resolved || stats.Unresolved != tc.unresolved || stats.Ambiguous != tc.ambiguous {
				t.Fatalf("stats = %+v", stats)
			}
			for name, body := range contents(t, directory) {
				if len(body) != tc.config.BytesPerNote || len(markdown.Scan(name, []byte(body))) != tc.config.LinksPerNote {
					t.Fatal("size/link count mismatch")
				}
			}
		})
	}
}

func TestGenerateRejectsExistingOutput(t *testing.T) {
	for _, kind := range []string{"empty directory", "regular file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			directory := filepath.Join(parent, "fixture")
			protected := filepath.Join(parent, "protected")
			if err := os.WriteFile(protected, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "empty directory":
				err = os.Mkdir(directory, 0700)
			case "regular file":
				err = os.WriteFile(directory, []byte("keep"), 0600)
			case "symlink":
				err = os.Symlink(protected, directory)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(directory)
			if err != nil {
				t.Fatal(err)
			}
			stats, err := generator.Generate(directory, generator.Config{Notes: 1, BytesPerNote: 128})
			if err == nil || !reflect.DeepEqual(stats, generator.Stats{}) {
				t.Fatalf("stats/error = %+v/%v", stats, err)
			}
			after, err := os.Lstat(directory)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("existing output replaced: %v", err)
			}
			if kind == "empty directory" {
				entries, err := os.ReadDir(directory)
				if err != nil || len(entries) != 0 {
					t.Fatal("directory changed")
				}
			} else {
				body, err := os.ReadFile(directory)
				if err != nil || string(body) != "keep" {
					t.Fatal("file changed")
				}
			}
		})
	}
	parent := filepath.Join(t.TempDir(), "absent")
	stats, err := generator.Generate(filepath.Join(parent, "fixture"), generator.Config{Notes: 1, BytesPerNote: 128})
	if err == nil || !reflect.DeepEqual(stats, generator.Stats{}) {
		t.Fatalf("missing parent stats/error = %+v/%v", stats, err)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Fatal("generator created parent")
	}
}

func TestGenerateSupportedScales(t *testing.T) {
	cases := []generator.Config{
		{Notes: 100, BytesPerNote: 50 * 1024, LinksPerNote: 100, Seed: 20260903},
		{Notes: 1000, BytesPerNote: 2 * 1024, LinksPerNote: 5, Seed: 20260903},
		{Notes: 10000, BytesPerNote: 1, Seed: 20260903},
		{Notes: 3000, BytesPerNote: 10 * 1024, LinksPerNote: 20, DuplicatePercent: 10, UnicodePercent: 20, UnresolvedPercent: 5, AmbiguousPercent: 5, Seed: 20260903},
	}
	for _, config := range cases {
		t.Run(fmt.Sprintf("notes=%d", config.Notes), func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "fixture")
			stats, err := generator.Generate(directory, config)
			if err != nil {
				t.Fatal(err)
			}
			files, diagnostics, err := root.Walk(directory, root.Options{})
			if err != nil || len(diagnostics) != 0 || len(files) != config.Notes {
				t.Fatalf("Walk count/diagnostics/error = %d/%v/%v", len(files), diagnostics, err)
			}
			bytes := 0
			for _, file := range files {
				info, err := os.Stat(file.PhysicalPath)
				if err != nil {
					t.Fatal(err)
				}
				if info.Size() != int64(config.BytesPerNote) {
					t.Fatalf("file size = %d", info.Size())
				}
				bytes += int(info.Size())
			}
			if stats.Bytes != bytes || stats.Links != config.Notes*config.LinksPerNote {
				t.Fatalf("stats = %+v, bytes = %d", stats, bytes)
			}
			if config.Notes != 3000 {
				return
			}
			if stats.DuplicateNotes != 300 || stats.UnicodeNotes != 600 || stats.Resolved != 54000 || stats.Unresolved != 3000 || stats.Ambiguous != 3000 || len(stats.Outgoing) == 0 || len(stats.Backlinks) == 0 {
				t.Fatalf("primary stats = %+v", stats)
			}
			catalog := root.NewCatalog(files)
			outgoing, _, err := query.Outgoing(catalog, *catalog.ByExactPath[stats.Source], false)
			if err != nil || !reflect.DeepEqual(resultCounts(outgoing), stats.Outgoing) {
				t.Fatalf("primary outgoing mismatch: %v", err)
			}
			backlinks, diagnostics := query.Backlinks(catalog, *catalog.ByExactPath[stats.Target])
			if !reflect.DeepEqual(resultCounts(backlinks), stats.Backlinks) || len(diagnostics) != 6000 {
				t.Fatal("primary backlinks/diagnostic mismatch")
			}
			t.Logf("primary: notes=%d bytes=%d links=%d duplicate=%d Unicode=%d resolved=%d unresolved=%d ambiguous=%d outgoing_files=%d backlinks_sources=%d", stats.Notes, stats.Bytes, stats.Links, stats.DuplicateNotes, stats.UnicodeNotes, stats.Resolved, stats.Unresolved, stats.Ambiguous, len(stats.Outgoing), len(stats.Backlinks))
		})
	}
}
