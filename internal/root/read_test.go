package root

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMarkdown(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "note.md"), []byte("[[target]]"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, _, err := Walk(directory, Options{})
	if err != nil {
		t.Fatal(err)
	}
	body, diagnostic := ReadMarkdown(files[0])
	if string(body) != "[[target]]" || diagnostic != nil {
		t.Fatalf("body/diagnostic = %q/%v", body, diagnostic)
	}
}

func TestReadMarkdownRejectsSymlinkReplacement(t *testing.T) {
	directory, external := t.TempDir(), t.TempDir()
	writeFile(t, directory, "note.md")
	if err := os.WriteFile(filepath.Join(external, "outside.md"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, _, err := Walk(directory, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(directory, "note.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "outside.md"), filepath.Join(directory, "note.md")); err != nil {
		t.Fatal(err)
	}
	body, diagnostic := ReadMarkdown(files[0])
	if len(body) != 0 || diagnostic == nil || diagnostic.Code != "unreadable-file" || diagnostic.Source != "note.md" {
		t.Fatalf("body/diagnostic = %q/%v", body, diagnostic)
	}
}

func TestReadMarkdownSizeLimit(t *testing.T) {
	for _, size := range []int64{0, 64 << 20, (64 << 20) + 1} {
		directory := t.TempDir()
		physical := filepath.Join(directory, "note.md")
		f, err := os.Create(physical)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(size); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		files, _, err := Walk(directory, Options{})
		if err != nil {
			t.Fatal(err)
		}
		body, diagnostic := ReadMarkdown(files[0])
		if size > 64<<20 {
			if len(body) != 0 || diagnostic == nil || diagnostic.Code != "file-too-large" {
				t.Fatalf("size %d: length=%d diagnostic=%v", size, len(body), diagnostic)
			}
		} else if int64(len(body)) != size || diagnostic != nil {
			t.Fatalf("size %d: length=%d diagnostic=%v", size, len(body), diagnostic)
		}
	}
}

func TestReadMarkdownSkipsAttachmentBody(t *testing.T) {
	directory := t.TempDir()
	physical := filepath.Join(directory, "image.png")
	f, err := os.Create(physical)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((64 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	files, _, err := Walk(directory, Options{})
	if err != nil || len(files) != 1 {
		t.Fatalf("walk = %v/%v", files, err)
	}
	catalog := NewCatalog(files)
	if catalog.ByExactPath["image.png"] == nil {
		t.Fatal("oversized attachment absent from catalog")
	}
	body, diagnostic := ReadMarkdown(files[0])
	if len(body) != 0 || diagnostic != nil {
		t.Fatalf("attachment body/diagnostic = %d/%v", len(body), diagnostic)
	}
}

type endlessReader struct{ consumed int64 }

func (r *endlessReader) Read(p []byte) (int, error) {
	clear(p)
	r.consumed += int64(len(p))
	return len(p), nil
}

func TestReadBodyNeverConsumesPastLimit(t *testing.T) {
	reader := &endlessReader{}
	body, err := readBody(reader)
	if err != nil || reader.consumed != (64<<20)+1 || len(body) != (64<<20)+1 {
		t.Fatalf("consumed=%d length=%d error=%v", reader.consumed, len(body), err)
	}
}

func TestReadMarkdownRejectsChangedIdentity(t *testing.T) {
	for _, kind := range []string{"ancestor-symlink", "root-directory", "regular-file"} {
		t.Run(kind, func(t *testing.T) {
			parent, external := t.TempDir(), t.TempDir()
			directory := filepath.Join(parent, "root")
			writeFile(t, directory, "branch/note.md")
			if err := os.WriteFile(filepath.Join(external, "note.md"), []byte("outside"), 0o600); err != nil {
				t.Fatal(err)
			}
			files, _, err := Walk(directory, Options{})
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "ancestor-symlink":
				if err := os.Rename(filepath.Join(directory, "branch"), filepath.Join(directory, "old")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, filepath.Join(directory, "branch")); err != nil {
					t.Fatal(err)
				}
			case "root-directory":
				if err := os.Rename(directory, filepath.Join(parent, "old")); err != nil {
					t.Fatal(err)
				}
				writeFile(t, directory, "branch/note.md")
			case "regular-file":
				physical := filepath.Join(directory, "branch", "note.md")
				if err := os.Rename(physical, physical+".old"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(physical, []byte("replacement"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			body, diagnostic := ReadMarkdown(files[0])
			if len(body) != 0 || diagnostic == nil || diagnostic.Code != "unreadable-file" || diagnostic.Source != "branch/note.md" {
				t.Fatalf("body/diagnostic = %q/%v", body, diagnostic)
			}
		})
	}
}

func TestReadMarkdownThroughAllowedSymlink(t *testing.T) {
	for _, kind := range []string{"directory", "file"} {
		t.Run(kind, func(t *testing.T) {
			directory, external := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(external, "note.md"), []byte("[[target]]"), 0o600); err != nil {
				t.Fatal(err)
			}
			name, target := "docs", external
			if kind == "file" {
				name, target = "note.md", filepath.Join(external, "note.md")
			}
			if err := os.Symlink(target, filepath.Join(directory, name)); err != nil {
				t.Fatal(err)
			}
			files, _, err := Walk(directory, Options{FollowSymlinks: []string{name}})
			if err != nil || len(files) != 1 {
				t.Fatalf("Walk = %v/%v", files, err)
			}
			body, diagnostic := ReadMarkdown(files[0])
			if string(body) != "[[target]]" || diagnostic != nil {
				t.Fatalf("body/diagnostic = %q/%v", body, diagnostic)
			}
			old := external + "-old"
			if err := os.Rename(external, old); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(old) })
			writeFile(t, external, "note.md")
			body, diagnostic = ReadMarkdown(files[0])
			if len(body) != 0 || diagnostic == nil || diagnostic.Code != "unreadable-file" {
				t.Fatalf("replaced origin body/diagnostic = %q/%v", body, diagnostic)
			}
		})
	}
}

func TestReadMarkdownUnreadableFile(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, directory, "note.md")
	files, _, err := Walk(directory, Options{})
	if err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(directory, "note.md")
	if err := os.Chmod(physical, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(physical, 0o600) })
	if _, err := os.ReadFile(physical); err == nil {
		t.Skip("current user bypasses file permissions")
	}
	body, diagnostic := ReadMarkdown(files[0])
	if len(body) != 0 || diagnostic == nil || diagnostic.Code != "unreadable-file" || diagnostic.Source != "note.md" {
		t.Fatalf("body/diagnostic = %q/%v", body, diagnostic)
	}
}
