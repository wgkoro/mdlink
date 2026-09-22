package root

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkReadMarkdown(b *testing.B) {
	for _, size := range []int{2 << 10, 10 << 10} {
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			random := rand.New(rand.NewSource(20260903))
			want := make([]byte, size)
			for i := range want {
				want[i] = byte('a' + random.Intn(26))
			}
			directory := b.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "note.md"), want, 0600); err != nil {
				b.Fatal(err)
			}
			files, diagnostics, err := Walk(directory, Options{})
			if err != nil || len(diagnostics) != 0 || len(files) != 1 {
				b.Fatalf("Walk = %v/%v/%v", files, diagnostics, err)
			}
			source := NewCatalog(files).MarkdownFiles[0]
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				got, diagnostic := ReadMarkdown(*source)
				if diagnostic != nil || !bytes.Equal(got, want) {
					b.Fatalf("read length/diagnostic = %d/%v", len(got), diagnostic)
				}
			}
		})
	}
}
