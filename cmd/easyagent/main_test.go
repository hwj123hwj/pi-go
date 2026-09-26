package main

import "testing"

func TestInferredDefaultMode(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no args opens chat", want: "chat"},
		{name: "approval flag opens chat", args: []string{"-y"}, want: "chat"},
		{name: "chat subcommand", args: []string{"chat"}, want: "chat"},
		{name: "interactive subcommand", args: []string{"interactive"}, want: "chat"},
		{name: "serve subcommand", args: []string{"serve"}, want: "serve"},
		{name: "run subcommand", args: []string{"run"}, want: "run"},
		{name: "short prompt flag selects one shot", args: []string{"-p", "hello"}, want: "run"},
		{name: "long prompt flag selects one shot", args: []string{"--prompt", "hello"}, want: "run"},
		{name: "prompt equals form selects one shot", args: []string{"--prompt=hello"}, want: "run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferredDefaultMode(tt.args); got != tt.want {
				t.Fatalf("inferredDefaultMode(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
