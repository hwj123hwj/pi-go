package scheduler

import (
	"strings"
	"time"
)

// ParseDuration converts a simple duration string (e.g. "5m", "1h", "30s")
// to a time.Duration. Returns false if the format is invalid.
//
// This mirrors hwjcode's parseDuration for /loop, but in Go.
func ParseDuration(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, false
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	// Parse the numeric part
	var n int
	for i, c := range numStr {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		_ = i
	}

	switch unit {
	case 's':
		return time.Duration(n) * time.Second, true
	case 'm':
		return time.Duration(n) * time.Minute, true
	case 'h':
		return time.Duration(n) * time.Hour, true
	default:
		return 0, false
	}
}

// FormatDuration returns a human-friendly duration string.
func FormatDuration(d time.Duration) string {
	if d >= time.Hour {
		hours := int(d / time.Hour)
		remainder := d % time.Hour
		if remainder == 0 {
			return pluralize(hours, "hour")
		}
		minutes := int(remainder / time.Minute)
		return pluralize(hours, "hour") + " " + pluralize(minutes, "minute")
	}
	if d >= time.Minute {
		minutes := int(d / time.Minute)
		return pluralize(minutes, "minute")
	}
	seconds := int(d / time.Second)
	return pluralize(seconds, "second")
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return itoa(n) + " " + unit + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
