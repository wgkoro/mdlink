package root

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func ignoreFixture(t *testing.T, bodies map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range bodies {
		writeFile(t, dir, name)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func markdownPaths(files []File) []string {
	var names []string
	for _, file := range files {
		if strings.HasSuffix(file.LogicalPath, ".md") {
			names = append(names, file.LogicalPath)
		}
	}
	return names
}

func TestWalkGitignore(t *testing.T) {
	dir := ignoreFixture(t, map[string]string{
		".gitignore": "node_modules/\n*.md\n!keep.md\n!nested/\n!parent/child.md\nparent/\n",
		"keep.md":    "", "lost.md": "", "node_modules/bad/.gitignore": "[", "node_modules/pkg.md": "",
		"nested/.gitignore": "!child.md\n", "nested/child.md": "", "nested/no.md": "", "nested/node_modules/pkg.md": "", "parent/child.md": "",
	})
	if err := os.Symlink("missing", filepath.Join(dir, "node_modules", "bad-link")); err != nil {
		t.Fatal(err)
	}
	files, diags, err := Walk(dir, Options{})
	if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md", "nested/child.md"}) {
		t.Fatal(markdownPaths(files), diags, err)
	}
	files, _, err = Walk(dir, Options{NoGitignore: true, Exclude: []string{"node_modules", "nested/node_modules"}})
	if err != nil || len(markdownPaths(files)) != 5 {
		t.Fatal(markdownPaths(files), err)
	}
	files, _, err = Walk(dir, Options{Exclude: []string{"keep.md", "nested", ".gitignore"}})
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
}

func TestGitignorePatternsAgainstGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git oracle unavailable")
	}
	cases := []struct {
		pattern string
		names   []string
	}{
		{"/root.md", []string{"root.md", "deep/root.md"}},
		{"*.md\n!keep.md", []string{"lost.md", "keep.md", "deep/keep.md"}},
		{"a/***/z.md", []string{"a/z.md", "a/b/z.md", "a/b/c/z.md"}},
		{"a/***b/z.md", []string{"a/b/z.md", "a/xb/z.md", "a/x/b/z.md"}},
		{`a/\**/z.md`, []string{"a/*/z.md", "a/**/z.md", "a/b/z.md", "a/x/y/z.md"}},
		{"a/****/z.md", []string{"a/z.md", "a/b/z.md", "a/b/c/z.md"}},
		{"[]a].md", []string{"].md", "a.md", "b.md"}},
		{"[!]].md", []string{"].md", "a.md", "b.md"}},
		{"a/**/b.md", []string{"a/b.md", "a/x/b.md", "a/x/y/b.md", "a/x/a/y/b.md", "x/a/b.md"}},
		{"**/foo.md", []string{"foo.md", "a/foo.md", "a/b/foo.md", "food.md"}},
		{"a*/**\n!abc/keep.md", []string{"abc/no.md", "abc/keep.md", "abc/x/no.md"}},
		{"ab**cd.md", []string{"abcd.md", "abxcd.md", "ab/x/cd.md"}},
		{"q?.md\n[!a-c].md\n[a-c]x.md", []string{"q.md", "q1.md", "q12.md", "d.md", "b.md", "ax.md", "dx.md"}},
		{"\\#x.md\n\\!x.md\nspace.md   \nend.md\\ \r\n{a,b}.md", []string{"#x.md", "!x.md", "space.md", "end.md ", "{a,b}.md", "a.md", "b.md"}},
		{"build/", []string{"build/no.md", "sub/build/no.md", "build.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			bodies := map[string]string{".gitignore": tc.pattern + "\n"}
			for _, name := range tc.names {
				bodies[name] = ""
			}
			dir := ignoreFixture(t, bodies)
			env := append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
			init := exec.Command("git", "init", "-q", dir)
			init.Env = env
			if out, err := init.CombinedOutput(); err != nil {
				t.Fatal(string(out), err)
			}
			cmd := exec.Command("git", "-C", dir, "-c", "core.excludesFile=/dev/null", "check-ignore", "--no-index", "-z", "--stdin")
			cmd.Env = env
			cmd.Stdin = strings.NewReader(strings.Join(tc.names, "\x00") + "\x00")
			out, err := cmd.Output()
			if err != nil {
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
					t.Fatal(err)
				}
			}
			ignored := map[string]bool{}
			for _, name := range strings.Split(string(out), "\x00") {
				ignored[name] = true
			}
			files, diags, err := Walk(dir, Options{})
			if err != nil || len(diags) != 0 {
				t.Fatal(diags, err)
			}
			present := map[string]bool{}
			for _, file := range files {
				present[file.LogicalPath] = true
			}
			for _, name := range tc.names {
				if present[name] == ignored[name] {
					t.Errorf("pattern %q: %q present=%v git ignored=%v", tc.pattern, name, present[name], ignored[name])
				}
			}
		})
	}
}

func TestGitignoreScopeAndFollow(t *testing.T) {
	dir := ignoreFixture(t, map[string]string{".gitignore": "ignored\n", "keep.md": ""})
	ext := ignoreFixture(t, map[string]string{".gitignore": "drop.md\n", "drop.md": "", "keep.md": ""})
	for _, name := range []string{"ignored", "mount"} {
		if err := os.Symlink(ext, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	files, diags, err := Walk(dir, Options{FollowSymlinks: []string{"ignored", "ignored/missing", "mount"}})
	if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md", "mount/keep.md"}) {
		t.Fatal(markdownPaths(files), diags, err)
	}
	if _, _, err = Walk(dir, Options{FollowSymlinks: []string{"ignored", "unrelated"}}); err == nil {
		t.Fatal("unrelated follow must fail")
	}
	parent := ignoreFixture(t, map[string]string{".gitignore": "*.md\n", "root/keep.md": ""})
	files, _, err = Walk(filepath.Join(parent, "root"), Options{})
	if err != nil || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md"}) {
		t.Fatal(files, err)
	}
}

func TestGitignoreRuleFiles(t *testing.T) {
	for _, pattern := range []string{"[", "[[:digit:]].md"} {
		t.Run(pattern, func(t *testing.T) {
			dir := ignoreFixture(t, map[string]string{".gitignore": pattern, "keep.md": ""})
			files, diags, err := Walk(dir, Options{})
			if err == nil || len(files) != 0 || len(diags) != 1 || diags[0].Source != ".gitignore" || strings.Contains(err.Error(), dir) {
				t.Fatal(files, diags, err)
			}
			if _, _, err = Walk(dir, Options{NoGitignore: true}); err != nil {
				t.Fatal(err)
			}
		})
	}
	dir := ignoreFixture(t, map[string]string{"rules": "*.md\n", "keep.md": ""})
	if err := os.Symlink("rules", filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	files, diags, err := Walk(dir, Options{})
	if err != nil || len(diags) != 1 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md"}) {
		t.Fatal(files, diags, err)
	}
}

func TestGitignoreDirectoryOnlySymlinkAndOwnExclusion(t *testing.T) {
	for _, filename := range []string{".gitignore", ".mdlinkignore"} {
		t.Run(filename, func(t *testing.T) {
			dir := ignoreFixture(t, map[string]string{filename: "link/\n" + filename + "\n", "keep.md": ""})
			external := ignoreFixture(t, map[string]string{"file.md": ""})
			if err := os.Symlink(external, filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
			files, diags, err := Walk(dir, Options{FollowSymlinks: []string{"link"}})
			if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md", "link/file.md"}) {
				t.Fatal(files, diags, err)
			}
			for _, file := range files {
				if file.LogicalPath == filename {
					t.Fatal("ignored rule file in catalog")
				}
			}

		})
	}
}

func TestGitignoreReadFailureIsFatal(t *testing.T) {
	for _, filename := range []string{".gitignore", ".mdlinkignore"} {
		t.Run(filename, func(t *testing.T) {
			for _, mode := range []string{"unreadable", "oversize", "directory"} {
				t.Run(mode, func(t *testing.T) {
					dir := ignoreFixture(t, map[string]string{filename: "*.md", "keep.md": ""})
					name := filepath.Join(dir, filename)
					switch mode {
					case "unreadable":
						if err := os.Chmod(name, 0); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = os.Chmod(name, 0600) })
						if file, err := os.Open(name); err == nil {
							file.Close()
							t.Skip("permission bypass")
						}
					case "oversize":
						if err := os.Truncate(name, MaxBodySize+1); err != nil {
							t.Fatal(err)
						}
					case "directory":
						if err := os.Remove(name); err != nil {
							t.Fatal(err)
						}
						if err := os.Mkdir(name, 0700); err != nil {
							t.Fatal(err)
						}
					}
					files, diags, err := Walk(dir, Options{})
					if mode == "directory" {
						if err != nil || len(diags) != 0 || len(markdownPaths(files)) != 1 {
							t.Fatal(files, diags, err)
						}
						return
					}
					if err == nil || len(files) != 0 || len(diags) != 1 || diags[0].Source != filename || strings.Contains(err.Error(), dir) {
						t.Fatal(files, diags, err)
					}
					if mode == "oversize" && diags[0].Code != "file-too-large" {
						t.Fatal(diags)
					}
				})
			}

		})
	}
}

func TestGitignoreReadRejectsReplacement(t *testing.T) {
	dir := ignoreFixture(t, map[string]string{".gitignore": "*.md"})
	files, _, err := Walk(dir, Options{NoGitignore: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, ".gitignore"), filepath.Join(dir, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("!*.md"), 0600); err != nil {
		t.Fatal(err)
	}
	if body, failure := readFile(files[0]); body != nil || failure == nil || failure.Source != ".gitignore" {
		t.Fatal(string(body), failure)
	}
}

func TestGitignoreReincludedParentLoadsRules(t *testing.T) {
	dir := ignoreFixture(t, map[string]string{".gitignore": "parent/\n!parent/\n", "parent/.gitignore": "*.md\n!keep.md\n", "parent/keep.md": "", "parent/no.md": ""})
	files, diags, err := Walk(dir, Options{})
	if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"parent/keep.md"}) {
		t.Fatal(files, diags, err)
	}
}

func TestGitignoreDoesNotUseGitMetadata(t *testing.T) {
	dir := ignoreFixture(t, map[string]string{".gitignore": "tracked.md\n", "tracked.md": "", "keep.md": "", ".git/info/exclude": "keep.md\n", ".git/index": "not an index", ".git/config": "not a config"})
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, ".git", "info", "exclude"))
	files, diags, err := Walk(dir, Options{})
	if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md"}) {
		t.Fatal(files, diags, err)
	}
}

func TestGitignoreSymlinkIsNotRulesWhenFollowed(t *testing.T) {
	for _, filename := range []string{".gitignore", ".mdlinkignore"} {
		t.Run(filename, func(t *testing.T) {
			dir := ignoreFixture(t, map[string]string{"keep.md": ""})
			ext := ignoreFixture(t, map[string]string{"rules": "*.md\n"})
			if err := os.Symlink(filepath.Join(ext, "rules"), filepath.Join(dir, filename)); err != nil {
				t.Fatal(err)
			}
			files, diags, err := Walk(dir, Options{FollowSymlinks: []string{filename}})
			if err != nil || len(diags) != 0 || !reflect.DeepEqual(markdownPaths(files), []string{"keep.md"}) {
				t.Fatal(files, diags, err)
			}

		})
	}
}
