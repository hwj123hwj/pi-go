package hooks

import (
	"fmt"
	"strings"

	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// Aggregator merges results from multiple hook executions into a single outcome.
type Aggregator struct{}

// NewAggregator creates a new hook aggregator.
func NewAggregator() *Aggregator { return &Aggregator{} }

// AggregateBeforeTool merges multiple before-tool-hook results.
// Strategy: any error blocks execution (OR logic). Last modified context wins.
func (a *Aggregator) AggregateBeforeTool(results []Result, contexts []agent.ToolCallContext) (agent.ToolCallContext, error) {
	if len(results) == 0 {
		return agent.ToolCallContext{}, nil
	}

	var errs []string
	var lastCtx agent.ToolCallContext

	for i, r := range results {
		if r.Err != nil {
			errs = append(errs, r.Err.Error())
		}
		if i < len(contexts) {
			lastCtx = contexts[i]
		}
	}

	if len(errs) > 0 {
		return lastCtx, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return lastCtx, nil
}

// AggregateAfterTool merges multiple after-tool-hook results.
// Strategy: last successful result wins, any error overrides.
func (a *Aggregator) AggregateAfterTool(results []Result, toolResults []agent.ToolResult) (agent.ToolResult, error) {
	if len(results) == 0 {
		return agent.ToolResult{}, nil
	}

	var lastErr error
	var lastResult agent.ToolResult
	hasResult := false

	for i, r := range results {
		if r.Err != nil {
			lastErr = r.Err
		}
		if i < len(toolResults) {
			lastResult = toolResults[i]
			hasResult = true
		}
	}

	if lastErr != nil {
		if hasResult {
			return lastResult, &AfterToolAggregateError{Err: lastErr, LastResult: lastResult}
		}
		return agent.ToolResult{}, lastErr
	}
	return lastResult, nil
}

// AfterToolAggregateError wraps an after-tool error while preserving the last result.
type AfterToolAggregateError struct {
	Err        error
	LastResult agent.ToolResult
}

func (e *AfterToolAggregateError) Error() string { return e.Err.Error() }
func (e *AfterToolAggregateError) Unwrap() error  { return e.Err }
