// Package generator creates anonymous benchmark fixtures inside a caller-owned temporary directory.
package generator

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Config struct {
	Notes, BytesPerNote, LinksPerNote                                     int
	DuplicatePercent, UnicodePercent, UnresolvedPercent, AmbiguousPercent int
	Seed                                                                  int64
}

type Stats struct {
	Notes, Bytes, Links                                           int
	DuplicateNotes, UnicodeNotes, Resolved, Unresolved, Ambiguous int
	Source, Target                                                string
	Outgoing, Backlinks                                           map[string]int
}

// Generate requires a nonexistent child of a temporary directory owned by the caller.
// The caller is responsible for removing that parent, including partial output after an I/O error.
func Generate(directory string, config Config) (Stats, error) {
	if config.Notes < 1 || config.Notes > 10000 || config.BytesPerNote < 1 || config.BytesPerNote > 50*1024 || config.LinksPerNote < 0 || config.LinksPerNote > 100 {
		return Stats{}, fmt.Errorf("notes, bytes, or links outside supported range")
	}
	for _, percent := range []int{config.DuplicatePercent, config.UnicodePercent, config.UnresolvedPercent, config.AmbiguousPercent} {
		if percent < 0 || percent > 100 {
			return Stats{}, fmt.Errorf("percent outside 0..100")
		}
	}
	if config.UnresolvedPercent+config.AmbiguousPercent > 100 {
		return Stats{}, fmt.Errorf("unresolved and ambiguous exceed 100 percent")
	}

	stats := Stats{
		Notes: config.Notes, Bytes: config.Notes * config.BytesPerNote, Links: config.Notes * config.LinksPerNote,
		DuplicateNotes: config.Notes * config.DuplicatePercent / 100 / 2 * 2,
		UnicodeNotes:   config.Notes * config.UnicodePercent / 100,
		Outgoing:       make(map[string]int), Backlinks: make(map[string]int),
	}
	stats.Unresolved = stats.Links * config.UnresolvedPercent / 100
	stats.Ambiguous = stats.Links * config.AmbiguousPercent / 100
	stats.Resolved = stats.Links - stats.Unresolved - stats.Ambiguous
	if stats.Ambiguous > 0 && stats.DuplicateNotes == 0 {
		return Stats{}, fmt.Errorf("ambiguous links require a duplicate pair")
	}
	random := rand.New(rand.NewSource(config.Seed))
	names := make([]string, config.Notes)
	for i := range names {
		names[i] = fmt.Sprintf("note-%05d.md", i)
		if i < stats.DuplicateNotes {
			names[i] = fmt.Sprintf("group-%05d/duplicate-%05d.md", i, i/2)
		}
	}
	for _, i := range random.Perm(config.Notes)[:stats.UnicodeNotes] {
		names[i] = path.Join("日本 語", names[i])
	}
	stats.Source, stats.Target = names[0], names[0]
	classes := make([]byte, stats.Links)
	for i := range stats.Unresolved {
		classes[i] = 1
	}
	for i := range stats.Ambiguous {
		classes[stats.Unresolved+i] = 2
	}
	random.Shuffle(len(classes), func(i, j int) { classes[i], classes[j] = classes[j], classes[i] })

	// Regenerate from the same seed after checking every body's size, without retaining all bodies.
	render := func(write bool) error {
		random := rand.New(rand.NewSource(config.Seed))
		for i, source := range names {
			var body strings.Builder
			for j := range config.LinksPerNote {
				index := i*config.LinksPerNote + j
				var target string
				switch classes[index] {
				case 1:
					target = fmt.Sprintf("missing-%08d.md", index)
				case 2:
					target = fmt.Sprintf("duplicate-%05d.md", random.Intn(stats.DuplicateNotes/2))
				default:
					target = names[random.Intn(len(names))]
					if !write {
						if source == stats.Source {
							stats.Outgoing[target]++
						}
						if target == stats.Target {
							stats.Backlinks[source]++
						}
					}
				}
				fmt.Fprintf(&body, "[[%s]]\n", target)
			}
			if body.Len() > config.BytesPerNote {
				return fmt.Errorf("links do not fit in %d bytes per note", config.BytesPerNote)
			}
			if !write {
				continue
			}
			body.WriteString(strings.Repeat("x", config.BytesPerNote-body.Len()))
			filename := filepath.Join(directory, filepath.FromSlash(source))
			if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(filename, []byte(body.String()), 0600); err != nil {
				return err
			}
		}
		return nil
	}
	if err := render(false); err != nil {
		return Stats{}, err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return Stats{}, err
	}
	if err := render(true); err != nil {
		return Stats{}, err
	}
	return stats, nil
}
