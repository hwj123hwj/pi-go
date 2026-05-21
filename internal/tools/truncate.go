package tools

import (
	"fmt"
	"strings"
)

const DefaultMaxOutputLen = 30000

// TruncateOutput truncates output to maxLen characters.
// It keeps the front 80% and back 20%, inserting a truncation marker in between.
// If maxLen <= 0, DefaultMaxOutputLen is used.
// If output fits within maxLen, it is returned unchanged.
func TruncateOutput(output string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = DefaultMaxOutputLen
	}
	if len(output) <= maxLen {
		return output
	}

	frontLen := int(float64(maxLen) * 0.8)
	backLen := maxLen - frontLen

	truncated := len(output) - maxLen
	marker := fmt.Sprintf("\n... [truncated %d characters] ...\n", truncated)

	var b strings.Builder
	b.Grow(maxLen + len(marker))
	b.WriteString(output[:frontLen])
	b.WriteString(marker)
	b.WriteString(output[len(output)-backLen:])
	return b.String()
}
