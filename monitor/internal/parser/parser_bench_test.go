package parser

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// BenchmarkParseDebateMeta parses a typical meta.json.
func BenchmarkParseDebateMeta(b *testing.B) {
	data := readBenchdata(b, "meta.json")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDebateMeta(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseScores parses a 100-line scores.jsonl.
func BenchmarkParseScores(b *testing.B) {
	// Generate 100 score lines.
	var lines []byte
	for i := 0; i < 100; i++ {
		ts := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute)
		line := fmt.Sprintf(
			`{"timestamp":"%s","debate_id":"d%d","pair":"bench-pair","milestone":1,"rounds_to_consensus":2,"escalated":false,"issues_found":3,"issues_resolved":3}`,
			ts.Format(time.RFC3339), i,
		)
		lines = append(lines, []byte(line+"\n")...)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries, _ := ParseScores(lines)
		if len(entries) != 100 {
			b.Fatalf("expected 100 entries, got %d", len(entries))
		}
	}
}

// BenchmarkParseWorkflow parses a typical workflow.yaml.
func BenchmarkParseWorkflow(b *testing.B) {
	data := readBenchdata(b, "workflow.yaml")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseWorkflow(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParsePlan parses a plan with milestones.
func BenchmarkParsePlan(b *testing.B) {
	data := readBenchdata(b, "plan.yaml")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParsePlan(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// readBenchdata reads a testdata file for benchmarks.
func readBenchdata(b *testing.B, name string) []byte {
	b.Helper()
	path := testdataPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("failed to read testdata/%s: %v", name, err)
	}
	return data
}
