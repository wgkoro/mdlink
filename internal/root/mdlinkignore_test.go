package root

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMdlinkignoreRulesAndComposition(t *testing.T) {
	for _, tc := range []struct {
		name, rules string
		options     Options
		want        []string
	}{
		{"absent", "", Options{}, []string{"keep.md", "nested/gitkeep.md", "nested/mdhide.md", "parent/child.md"}},
		{"union", "mdhide.md\n!githide.md\n!nested/githide.md\nparent/\n!parent/child.md\n", Options{}, []string{"keep.md", "nested/gitkeep.md"}},
		{"negation", "*.md\n!keep.md\n!parent/\n!parent/child.md\n!githide.md\n", Options{}, []string{"keep.md", "parent/child.md"}},
		{"parent", "parent/\n!parent/\n", Options{}, []string{"keep.md", "nested/gitkeep.md", "nested/mdhide.md", "parent/child.md"}},
		{"no-gitignore", "mdhide.md\n!githide.md\n", Options{NoGitignore: true}, []string{"githide.md", "keep.md", "nested/githide.md", "nested/gitkeep.md", "parent/child.md"}},
		{"manual-default", "!keep.md\n!.git/\n!.obsidian/\n!.trash/\n", Options{Exclude: []string{"keep.md"}}, []string{"nested/gitkeep.md", "nested/mdhide.md", "parent/child.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := ignoreFixture(t, map[string]string{
				".gitignore": "githide.md\n!mdhide.md\n", "nested/.gitignore": "!mdhide.md\n", ".mdlinkignore": tc.rules,
				"keep.md": "", "githide.md": "", "nested/githide.md": "", "nested/gitkeep.md": "", "nested/mdhide.md": "", "parent/child.md": "",
				".git/no.md": "", ".obsidian/no.md": "", ".trash/no.md": "",
			})
			if tc.name == "absent" {
				if err := os.Remove(filepath.Join(dir, ".mdlinkignore")); err != nil {
					t.Fatal(err)
				}
			}
			files, diags, err := Walk(dir, tc.options)
			if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), tc.want) {
				t.Fatal(markdownPaths(files), diags, err)
			}
		})
	}
}

func TestMdlinkignorePrunesBeforeTraversal(t *testing.T) {
	dir := ignoreFixture(t, map[string]string{".mdlinkignore": "skip/\n!skip/keep.md\nignored\n", "skip/keep.md": "", "skip/.gitignore": "[", "keep.md": ""})
	if err := os.Symlink("missing", filepath.Join(dir, "skip", "broken")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(dir, "ignored")); err != nil {
		t.Fatal(err)
	}
	files, diags, err := Walk(dir, Options{FollowSymlinks: []string{"skip/broken", "skip/missing", "ignored"}})
	if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md"}) {
		t.Fatal(files, diags, err)
	}
	if _, _, err := Walk(dir, Options{FollowSymlinks: []string{"unrelated"}}); !errors.Is(err, ErrInvalidOptions) {
		t.Fatal(err)
	}
}

func TestMdlinkignoreScope(t *testing.T) {
	for _, rules := range []string{"", "# comment\r\n\\#drop.md\r\n/drop.md\nmount/drop.md\n"} {
		parent := ignoreFixture(t, map[string]string{".mdlinkignore": "[", "root/.mdlinkignore": rules, "root/keep.md": "", "root/drop.md": "", "root/#drop.md": "", "root/nested/.mdlinkignore": "[", "root/nested/keep.md": ""})
		dir := filepath.Join(parent, "root")
		ext := ignoreFixture(t, map[string]string{".mdlinkignore": "[", "keep.md": "", "drop.md": ""})
		if err := os.Symlink(ext, filepath.Join(dir, "mount")); err != nil {
			t.Fatal(err)
		}
		files, diags, err := Walk(dir, Options{FollowSymlinks: []string{"mount"}})
		want := []string{"keep.md", "mount/keep.md", "nested/keep.md"}
		if rules == "" {
			want = []string{"#drop.md", "drop.md", "keep.md", "mount/drop.md", "mount/keep.md", "nested/keep.md"}
		}
		if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), want) {
			t.Fatal(files, diags, err)
		}
	}
}

func TestMdlinkignoreOwnExclusion(t *testing.T) {
	for _, excludedBy := range []string{"none", ".mdlinkignore", ".gitignore", "--exclude"} {
		t.Run(excludedBy, func(t *testing.T) {
			bodies := map[string]string{".mdlinkignore": "drop.md\n", "drop.md": "", "keep.md": ""}
			options := Options{}
			if excludedBy == "--exclude" {
				options.Exclude = []string{".mdlinkignore"}
			} else if excludedBy != "none" {
				bodies[excludedBy] += ".mdlinkignore\n"
			}
			dir := ignoreFixture(t, bodies)
			files, diags, err := Walk(dir, options)
			if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md"}) {
				t.Fatal(files, diags, err)
			}
			found := false
			for _, file := range files {
				if file.LogicalPath == ".mdlinkignore" {
					found = true
				}
			}
			if found != (excludedBy == "none") {
				t.Fatal(files)
			}
		})
	}
}

func TestMdlinkignoreInvalidRules(t *testing.T) {
	for _, pattern := range []string{"[", "[[:digit:]].md"} {
		dir := ignoreFixture(t, map[string]string{".mdlinkignore": pattern, "keep.md": ""})
		for _, noGitignore := range []bool{false, true} {
			files, diags, err := Walk(dir, Options{NoGitignore: noGitignore})
			if err == nil || len(files) != 0 || len(diags) != 1 || diags[0].Code != "invalid-mdlinkignore" || diags[0].Source != ".mdlinkignore" || strings.Contains(err.Error(), dir) {
				t.Fatal(files, diags, err)
			}
		}
	}
}
