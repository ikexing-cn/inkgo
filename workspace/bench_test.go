package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkLog(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			s, err := Create(b.TempDir(), "基准作品", "非虚构", "zh-Hans")
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			r := Record{Schema: Schema, At: time.Now().UTC(), Feature: FeatureTalk, Model: "deepseek/deepseek-flash", Outcome: OutcomeDone,
				Parts: []Part{{PartRules, 1100}, {PartTalk, 9600}}, Usage: Usage{Input: 12000, CacheRead: 9000, Output: 300}}
			line, err := json.Marshal(r)
			if err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(s.Root(), logFile), bytes.Repeat(append(line, '\n'), count), 0600); err != nil {
				b.Fatal(err)
			}
			if err := s.Log(r); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := s.Log(r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkAppend(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			s, err := Create(b.TempDir(), "基准作品", "非虚构", "zh-Hans")
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			session, err := s.NewSession()
			if err != nil {
				b.Fatal(err)
			}
			var data bytes.Buffer
			enc := json.NewEncoder(&data)
			for i := range count {
				e := Event{Schema: Schema, ID: fmt.Sprintf("20261021-1200-%04x", i), At: time.Now().UTC(), Role: RoleUser, Text: "林晚走过雨中的街道。"}
				if err := enc.Encode(e); err != nil {
					b.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(s.Root(), "sessions", session+".jsonl"), data.Bytes(), 0600); err != nil {
				b.Fatal(err)
			}
			e := Event{Role: RoleAssistant, Text: "她在巷口停了下来。", ReplyTo: "20261021-1200-0000"}
			if _, err := s.Append(session, e); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := s.Append(session, e); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
