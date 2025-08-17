package transfer

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Progress represents the current state of a file transfer
type Progress struct {
	FileName    string
	FileSize    int64
	Transferred int64
	Speed       float64   // bytes per second
	ETA         float64   // estimated time remaining in seconds
	StartTime   time.Time
	LastUpdate  time.Time
	mu          sync.Mutex
}

// ProgressCallback is a function type for progress updates
type ProgressCallback func(p *Progress) bool

// NewProgress creates a new Progress tracker
func NewProgress(fileName string, fileSize int64) *Progress {
	now := time.Now()
	return &Progress{
		FileName:   fileName,
		FileSize:   fileSize,
		StartTime:  now,
		LastUpdate: now,
	}
}

// Update updates the progress with the number of bytes transferred
func (p *Progress) Update(bytesTransferred int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	timeElapsed := now.Sub(p.LastUpdate).Seconds()
	if timeElapsed > 0 {
		// Calculate speed in bytes per second
		p.Speed = float64(bytesTransferred) / timeElapsed
		
		// Calculate ETA if we're making progress
		if p.Speed > 0 {
			remainingBytes := p.FileSize - p.Transferred - bytesTransferred
			p.ETA = float64(remainingBytes) / p.Speed
		}
	}

	p.Transferred += bytesTransferred
	p.LastUpdate = now
}

// Percent returns the completion percentage (0-100)
func (p *Progress) Percent() float64 {
	if p.FileSize <= 0 {
		return 0
	}
	return float64(p.Transferred) / float64(p.FileSize) * 100
}

// Elapsed returns the duration since the transfer started
func (p *Progress) Elapsed() time.Duration {
	return time.Since(p.StartTime)
}

// Remaining returns the estimated time remaining
func (p *Progress) Remaining() time.Duration {
	return time.Duration(p.ETA) * time.Second
}

// String returns a human-readable progress string
func (p *Progress) String() string {
	return fmt.Sprintf("%s: %.2f%% - %s/s - ETA: %s",
		p.FileName,
		p.Percent(),
		formatBytes(p.Speed),
		time.Duration(p.ETA)*time.Second,
	)
}

// TrackProgress creates a writer that updates the progress as data is written
type progressWriter struct {
	progress *Progress
	writer   io.Writer
	callback ProgressCallback
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err == nil {
		pw.progress.Update(int64(n))
		if pw.callback != nil {
			if !pw.callback(pw.progress) {
				return n, io.EOF // Signal cancellation
			}
		}
	}
	return n, err
}

// NewProgressWriter wraps an io.Writer to track progress
func NewProgressWriter(w io.Writer, progress *Progress, callback ProgressCallback) io.Writer {
	return &progressWriter{
		progress: progress,
		writer:   w,
		callback: callback,
	}
}
