package prompt

import (
	"fmt"
	"strings"
	"time"

	"github.com/earendil-works/pi-go/internal/agent"
)

type Options struct {
	CustomPrompt string
	CWD          string
	Tools        []agent.Tool
}

func BuildSystemPrompt(opts Options) string {
	base := opts.CustomPrompt
	if base == "" {
		base = "You are Pi Go, a server-side coding agent. Be concise, technical, and safe."
	}
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\nRuntime:\n")
	b.WriteString(fmt.Sprintf("- Date: %s\n", time.Now().Format("2006-01-02")))
	if opts.CWD != "" {
		b.WriteString(fmt.Sprintf("- CWD: %s\n", opts.CWD))
	}
	if len(opts.Tools) > 0 {
		b.WriteString("\nAvailable tools:\n")
		for _, tool := range opts.Tools {
			b.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description()))
		}
	}
	return b.String()
}
