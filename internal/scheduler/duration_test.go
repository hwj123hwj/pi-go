package scheduler

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		ok       bool
	}{
		{"5m", 5 * time.Minute, true},
		{"1h", 1 * time.Hour, true},
		{"30s", 30 * time.Second, true},
		{"10m", 10 * time.Minute, true},
		{"2h", 2 * time.Hour, true},
		{"", 0, false},
		{"5", 0, false},
		{"abc", 0, false},
		{"5x", 0, false},
		{"-5m", 0, false},
		{"0s", 0, true},
		{"0m", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseDuration(tt.input)
			if ok != tt.ok {
				t.Errorf("ParseDuration(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.expected {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{5 * time.Minute, "5 minutes"},
		{1 * time.Hour, "1 hour"},
		{2 * time.Hour, "2 hours"},
		{30 * time.Second, "30 seconds"},
		{90 * time.Minute, "1 hour 30 minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			got := FormatDuration(tt.input)
			if got != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
