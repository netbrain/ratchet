package datasource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkDebates lists 10 debates from disk.
func BenchmarkDebates(b *testing.B) {
	dir := b.TempDir()

	// Create 10 debate directories with valid meta.json files.
	for i := 0; i < 10; i++ {
		debateDir := filepath.Join(dir, "debates", fmt.Sprintf("debate-%d", i))
		if err := os.MkdirAll(debateDir, 0o755); err != nil {
			b.Fatal(err)
		}
		started := time.Date(2026, 3, 13, 16, 45, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour)
		meta := map[string]any{
			"id":          fmt.Sprintf("debate-%d", i),
			"pair":        "bench-pair",
			"phase":       "review",
			"milestone":   1,
			"files":       []string{"file.go"},
			"status":      "consensus",
			"round_count": 1,
			"max_rounds":  3,
			"started":     started.Format(time.RFC3339),
		}
		data, _ := json.Marshal(meta)
		if err := os.WriteFile(filepath.Join(debateDir, "meta.json"), data, 0o644); err != nil {
			b.Fatal(err)
		}
	}

	ds := NewFileDataSource(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ds.Debates()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScores reads and filters 100 score entries.
func BenchmarkScores(b *testing.B) {
	dir := b.TempDir()
	scoresDir := filepath.Join(dir, "scores")
	if err := os.MkdirAll(scoresDir, 0o755); err != nil {
		b.Fatal(err)
	}

	// Generate 100 score lines.
	var lines []byte
	for i := 0; i < 100; i++ {
		ts := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute)
		pair := "pair-a"
		if i%2 == 0 {
			pair = "pair-b"
		}
		entry := fmt.Sprintf(
			`{"timestamp":"%s","debate_id":"d%d","pair":"%s","milestone":1,"rounds_to_consensus":2,"escalated":false,"issues_found":3,"issues_resolved":3}`,
			ts.Format(time.RFC3339), i, pair,
		)
		lines = append(lines, []byte(entry+"\n")...)
	}
	if err := os.WriteFile(filepath.Join(scoresDir, "scores.jsonl"), lines, 0o644); err != nil {
		b.Fatal(err)
	}

	ds := NewFileDataSource(dir)

	b.Run("all", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ds.Scores("")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("filtered", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ds.Scores("pair-a")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
