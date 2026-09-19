package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"testing"
	"time"
)

// Measure the full list query, including counts and media attachment, with
// 10,000 synthetic fabrics. Use -benchtime=30x for comparable samples.
func BenchmarkList10000(b *testing.B) {
	s, err := Open(b.TempDir(), true)
	if err != nil {
		b.Fatal(err)
	}
	defer s.DB.Close()
	tx, err := s.DB.Begin()
	if err != nil {
		b.Fatal(err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare("INSERT INTO fabrics VALUES(?,1,?,?,?,NULL)")
	if err != nil {
		b.Fatal(err)
	}
	defer stmt.Close()
	for i := 0; i < 10000; i++ {
		f := example()
		f.ID = fmt.Sprintf("%032x", i+1)
		f.Revision = 1
		f.Name = fmt.Sprintf("合成棉麻布料 %05d", i)
		f.Materials = []string{"棉", "麻"}
		f.Tags = []string{"素色", "服装"}
		f.Location = fmt.Sprintf("箱 %d", i%10)
		f.CreatedAt = "2026-09-19T00:00:00Z"
		f.UpdatedAt = f.CreatedAt
		if err = f.Validate(); err != nil {
			b.Fatal(err)
		}
		body, _ := json.Marshal(f)
		if _, err = stmt.Exec(f.ID, string(body), f.CreatedAt, f.UpdatedAt); err != nil {
			b.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		b.Fatal(err)
	}
	for _, query := range []string{"status=stock", "q=棉麻&material=棉&location=箱+2&min_width=100&min_length=100"} {
		b.Run(query, func(b *testing.B) {
			v, _ := url.ParseQuery(query)
			var elapsed []time.Duration
			for b.Loop() {
				start := time.Now()
				result, err := s.List(context.Background(), v, false)
				elapsed = append(elapsed, time.Since(start))
				if err != nil || len(result.Items) != 24 {
					b.Fatalf("query failed: %v, items %d", err, len(result.Items))
				}
			}
			b.StopTimer()
			sort.Slice(elapsed, func(i, j int) bool { return elapsed[i] < elapsed[j] })
			b.ReportMetric(float64(elapsed[(len(elapsed)*95+99)/100-1])/float64(time.Millisecond), "p95-ms")
		})
	}
}
