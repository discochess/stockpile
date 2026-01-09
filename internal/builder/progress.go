// Package builder implements the data build pipeline for stockpile.
package builder

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// Progress tracks build progress.
type Progress struct {
	Phase           string
	BytesDownloaded int64
	BytesTotal      int64
	RecordsRead     int64
	RecordsWritten  int64
	ShardsCreated   int
	ShardsTotal     int
	StartTime       time.Time
	Error           error
}

// ProgressFunc is called periodically with progress updates.
type ProgressFunc func(Progress)

// progressWriter wraps an io.Writer to track bytes written.
type progressWriter struct {
	w       io.Writer
	written *atomic.Int64
}

func newProgressWriter(w io.Writer, counter *atomic.Int64) *progressWriter {
	return &progressWriter{w: w, written: counter}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.written.Add(int64(n))
	return n, err
}

// progressReader wraps an io.Reader to track bytes read.
type progressReader struct {
	r    io.Reader
	read *atomic.Int64
}

func newProgressReader(r io.Reader, counter *atomic.Int64) *progressReader {
	return &progressReader{r: r, read: counter}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.read.Add(int64(n))
	return n, err
}

// FormatBytes formats bytes as human-readable string.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration as human-readable string.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// DefaultProgressFunc prints progress to stdout.
func DefaultProgressFunc(p Progress) {
	switch p.Phase {
	case "download":
		pct := float64(0)
		if p.BytesTotal > 0 {
			pct = float64(p.BytesDownloaded) / float64(p.BytesTotal) * 100
		}
		fmt.Printf("\r[Download] %s / %s (%.1f%%)",
			FormatBytes(p.BytesDownloaded), FormatBytes(p.BytesTotal), pct)
	case "sort":
		fmt.Printf("\r[Sort] %d records processed", p.RecordsRead)
	case "shard":
		fmt.Printf("\r[Shard] %d / %d shards created, %d records",
			p.ShardsCreated, p.ShardsTotal, p.RecordsWritten)
	case "done":
		elapsed := time.Since(p.StartTime)
		fmt.Printf("\n[Done] %d records in %d shards (%s)\n",
			p.RecordsWritten, p.ShardsCreated, FormatDuration(elapsed))
	case "error":
		fmt.Printf("\n[Error] %v\n", p.Error)
	}
}
