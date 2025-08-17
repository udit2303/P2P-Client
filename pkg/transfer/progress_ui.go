package transfer

import (
	"fmt"
	"strings"
)

// progressBar creates a simple progress bar string
func progressBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}
	
	completed := int(float64(width) * percent / 100)
	if completed > width {
		completed = width
	}
	return fmt.Sprintf("%s%s", 
		strings.Repeat("=", completed),
		strings.Repeat(" ", width-completed),
	)
}

// formatBytes converts bytes to a human-readable string
func formatBytes(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.1f B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", 
		float64(bytes)/float64(div), "KMGTPE"[exp])
}
