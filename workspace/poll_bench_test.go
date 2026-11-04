package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Measure an unchanged book, including five trashed chapters. Fixture
// creation and .order reconciliation are outside the timed reads.
func BenchmarkChapterPoll(b *testing.B) {
	for _, chapters := range []int{15, 300, 1000} {
		b.Run(fmt.Sprint(chapters), func(b *testing.B) {
			s, err := Create(b.TempDir(), "读盘基准", "网文", "zh-Hans")
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			prose := []byte(strings.Repeat("字", 3000))
			for i := 0; i < chapters; i++ {
				dir := filepath.Join(s.Root(), "chapters", fmt.Sprintf("卷%02d", i/25))
				if err := os.MkdirAll(dir, 0755); err != nil {
					b.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("章%04d.txt", i)), prose, 0644); err != nil {
					b.Fatal(err)
				}
			}
			for i := 0; i < 5; i++ {
				id, err := s.NewChapter("", fmt.Sprintf("废稿%d", i), false)
				if err != nil {
					b.Fatal(err)
				}
				if err := s.TrashChapter(id); err != nil {
					b.Fatal(err)
				}
			}
			tree, err := s.Chapters()
			if err != nil {
				b.Fatal(err)
			}
			id := tree.Root.Children[0].Children[0].ID
			b.Run("separate", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := s.Chapters(); err != nil {
						b.Fatal(err)
					}
					if _, err := s.ChapterStamp(id); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("combined", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, _, err := s.ChaptersAndStamp(id); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
