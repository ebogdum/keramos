package repo

import (
	"fmt"
	"io"
	"os"
	"time"
)

const (
	progressMinSize     = 1024 * 1024 // 1MB threshold
	progressInterval    = 500 * time.Millisecond
	progressMBDivisor   = 1024.0 * 1024.0
)

// ProgressWriter wraps an io.Writer and prints download progress to stderr.
// Progress is only shown for downloads larger than 1MB.
type ProgressWriter struct {
	total      int64
	downloaded int64
	writer     io.Writer
	lastPrint  time.Time
	label      string
}

// NewProgressWriter creates a ProgressWriter that wraps w.
// If total is below the 1MB threshold, writes pass through without progress output.
func NewProgressWriter(w io.Writer, total int64, label string) *ProgressWriter {
	return &ProgressWriter{
		total:     total,
		writer:    w,
		lastPrint: time.Now(),
		label:     label,
	}
}

// Write implements io.Writer, forwarding bytes to the underlying writer
// and printing progress to stderr at regular intervals.
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	pw.downloaded += int64(n)

	if pw.total < progressMinSize {
		return n, err
	}

	if nil != err {
		return n, err
	}

	now := time.Now()
	if now.Sub(pw.lastPrint) < progressInterval && pw.downloaded < pw.total {
		return n, nil
	}

	pw.lastPrint = now
	pct := float64(0)
	if pw.total > 0 {
		pct = float64(pw.downloaded) / float64(pw.total) * 100
	}

	fmt.Fprintf(os.Stderr, "\rDownloading %s... %.1fMB / %.1fMB (%.0f%%)",
		pw.label,
		float64(pw.downloaded)/progressMBDivisor,
		float64(pw.total)/progressMBDivisor,
		pct,
	)

	if pw.downloaded >= pw.total {
		fmt.Fprintln(os.Stderr)
	}

	return n, nil
}
