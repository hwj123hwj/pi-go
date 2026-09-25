package workflow

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFactory 记录调用并按脚本决定成败，用于不依赖 LLM 的引擎测试。
type fakeFactory struct {
	mu        sync.Mutex
	responder func(prompt string) string
	failLeft  map[string]int // prompt 子串 → 剩余失败次数
	calls     []string
	runners   int
	inflight  int
	maxFlight int
	delay     time.Duration
}

func (f *fakeFactory) NewRunner(ctx context.Context, opts RunnerOptions) (Runner, error) {
	f.mu.Lock()
	f.runners++
	f.mu.Unlock()
	return &fakeRunner{f: f}, nil
}

func (f *fakeFactory) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeFactory) peakInflight() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxFlight
}

type fakeRunner struct{ f *fakeFactory }

func (r *fakeRunner) Prompt(ctx context.Context, prompt string) (string, error) {
	f := r.f
	f.mu.Lock()
	f.calls = append(f.calls, prompt)
	f.inflight++
	if f.inflight > f.maxFlight {
		f.maxFlight = f.inflight
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inflight--
		f.mu.Unlock()
	}()

	for sub, left := range f.failLeft {
		if left > 0 && contains(prompt, sub) {
			f.mu.Lock()
			f.failLeft[sub] = left - 1
			f.mu.Unlock()
			return "", fmt.Errorf("scripted failure: %s", sub)
		}
	}

	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if f.responder != nil {
		return f.responder(prompt), nil
	}
	return "out:" + prompt, nil
}

func (r *fakeRunner) Close() error { return nil }

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// runSync 同步执行一次工作流（runID 固定），供引擎断言。
func runSync(t *testing.T, eng *Engine, yamlSrc string, vars map[string]any) (string, *RunMeta, []Event) {
	t.Helper()
	spec, err := ParseSpec([]byte(yamlSrc))
	require.NoError(t, err)
	status, _ := eng.Run(context.Background(), spec, vars, "wf-1-aaaaaaaa")
	meta, err := loadMeta(eng.Dir, "wf-1-aaaaaaaa")
	require.NoError(t, err)
	events, err := loadEvents(eng.Dir, "wf-1-aaaaaaaa")
	require.NoError(t, err)
	return status, meta, events
}

func newTestEngine(t *testing.T, f *fakeFactory) *Engine {
	t.Helper()
	return &Engine{
		Factory:      f,
		Dir:          t.TempDir(),
		RetryBackoff: time.Millisecond,
	}
}

// ─── 基本执行 ────────────────────────────────────────────────────────────────

func TestEngine_SequentialChain(t *testing.T) {
	f := &fakeFactory{}
	eng := newTestEngine(t, f)
	status, meta, _ := runSync(t, eng, `
name: chain
steps:
  - id: a
    prompt: "first"
  - id: b
    prompt: "second after {{steps.a.output}}"
`, nil)

	assert.Equal(t, StatusCompleted, status)
	assert.Equal(t, StatusCompleted, meta.Steps["a"].Status)
	assert.Equal(t, StatusCompleted, meta.Steps["b"].Status)
	assert.Equal(t, "out:first", meta.Outputs["a"].Output)
	// 隐式顺序链：b 等待 a 完成后才能渲染，产出已注入模板
	assert.Equal(t, "out:second after out:first", meta.Outputs["b"].Output)
	assert.Len(t, f.calls, 2)
}

func TestEngine_ExplicitDeps(t *testing.T) {
	f := &fakeFactory{}
	eng := newTestEngine(t, f)
	status, meta, _ := runSync(t, eng, `
name: dag
steps:
  - id: a
    prompt: "a"
  - id: b
    prompt: "b"
    depends_on: [a]
  - id: c
    prompt: "c uses {{steps.a.output}} and {{steps.b.output}}"
    depends_on: [a, b]
`, nil)
	assert.Equal(t, StatusCompleted, status)
	assert.Equal(t, "out:c uses out:a and out:b", meta.Outputs["c"].Output)
}

func TestEngine_UpstreamFailureSkipsDownstream(t *testing.T) {
	f := &fakeFactory{failLeft: map[string]int{"boom": 99}} // 永远失败
	eng := newTestEngine(t, f)
	status, meta, _ := runSync(t, eng, `
name: failskip
steps:
  - id: a
    prompt: "boom please"
  - id: b
    prompt: "never"
`, nil)

	assert.Equal(t, StatusFailed, status)
	assert.Equal(t, StatusFailed, meta.Steps["a"].Status)
	assert.Equal(t, StatusSkipped, meta.Steps["b"].Status)
	assert.Contains(t, meta.Error, "scripted failure")
	assert.Len(t, f.calls, 1)
}

func TestEngine_RetryThenSucceed(t *testing.T) {
	f := &fakeFactory{failLeft: map[string]int{"flaky": 1}}
	eng := newTestEngine(t, f)
	status, meta, events := runSync(t, eng, `
name: retry
steps:
  - id: a
    prompt: "flaky call"
    retries: 2
`, nil)

	assert.Equal(t, StatusCompleted, status)
	assert.Equal(t, StatusCompleted, meta.Steps["a"].Status)
	errEvents := 0
	for _, e := range events {
		if e.Event == EventStepError {
			errEvents++
		}
	}
	assert.Equal(t, 1, errEvents)
	assert.Len(t, f.calls, 2)
}

func TestEngine_FanOutConcurrencyAndJoin(t *testing.T) {
	f := &fakeFactory{delay: 30 * time.Millisecond}
	eng := newTestEngine(t, f)
	status, meta, _ := runSync(t, eng, `
name: fan
max_concurrency: 2
vars:
  topics: [x, y, z, w]
steps:
  - id: research
    foreach: topics
    concurrency: 2
    prompt: "study {{item}} ({{index}})"
  - id: merge
    prompt: "merge: {{steps.research.output}}"
`, nil)

	assert.Equal(t, StatusCompleted, status)
	assert.Equal(t, StatusCompleted, meta.Steps["research"].Status)
	assert.Equal(t, 4, meta.Steps["research"].ItemsDone)
	assert.Equal(t, 4, meta.Steps["research"].ItemsTotal)
	assert.Equal(t, 2, f.peakInflight(), "concurrency 应被限制在 2")
	assert.Contains(t, meta.Outputs["merge"].Output, "out:study x (0)")
	assert.Contains(t, meta.Outputs["merge"].Output, "out:study w (3)")
	assert.Len(t, meta.Outputs["research"].Outputs, 4)
}

func TestEngine_CacheHitOnRerun(t *testing.T) {
	f := &fakeFactory{}
	eng := newTestEngine(t, f)
	specSrc := `
name: cached
vars:
  v: hello
steps:
  - id: a
    prompt: "say {{vars.v}}"
  - id: b
    prompt: "echo {{steps.a.output}}"
`
	spec, err := ParseSpec([]byte(specSrc))
	require.NoError(t, err)

	st1, err := eng.Run(context.Background(), spec, nil, "wf-1-aaaaaaaa")
	require.NoError(t, err)
	require.Equal(t, StatusCompleted, st1)
	firstCalls := f.callCount()
	require.Equal(t, 2, firstCalls)

	// 同一引擎目录重跑：全部命中缓存，不再调用 runner
	st2, err := eng.Run(context.Background(), spec, nil, "wf-2-bbbbbbbb")
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, st2)
	assert.Equal(t, firstCalls, f.callCount(), "缓存命中时不应再调用 runner")

	// 变量变化 → 签名变化 → 缓存 miss
	st3, err := eng.Run(context.Background(), spec, map[string]any{"v": "world"}, "wf-3-cccccccc")
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, st3)
	assert.Equal(t, firstCalls+2, f.callCount())
}

func TestEngine_Cancel(t *testing.T) {
	f := &fakeFactory{delay: 5 * time.Second}
	eng := &Engine{Factory: f, Dir: t.TempDir()}
	spec, err := ParseSpec([]byte(`
name: slow
steps:
  - id: a
    prompt: "slow"
`))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	status, err := eng.Run(ctx, spec, nil, "wf-1-aaaaaaaa")
	assert.Equal(t, StatusCancelled, status)
	assert.ErrorIs(t, err, context.Canceled)
}

// ─── 确认门（经 Registry）───────────────────────────────────────────────────

func TestRegistry_GateApprove(t *testing.T) {
	f := &fakeFactory{}
	eng := &Engine{Factory: f, Dir: t.TempDir(), RetryBackoff: time.Millisecond}
	reg := NewRegistry(eng)

	spec, err := ParseSpec([]byte(`
name: gated
steps:
  - id: plan
    prompt: "plan"
  - id: execute
    prompt: "execute {{steps.plan.output}}"
    confirm: true
`))
	require.NoError(t, err)

	runID, err := reg.Start(spec, nil)
	require.NoError(t, err)

	waitForStatus(t, reg, runID, StatusWaiting, 2*time.Second)
	require.NoError(t, reg.Approve(runID, ""))
	waitForStatus(t, reg, runID, StatusCompleted, 2*time.Second)

	meta, _, err := reg.Get(runID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, meta.Steps["execute"].Status)
}

func TestRegistry_GateReject(t *testing.T) {
	f := &fakeFactory{}
	eng := &Engine{Factory: f, Dir: t.TempDir()}
	reg := NewRegistry(eng)

	spec, err := ParseSpec([]byte(`
name: gated-reject
steps:
  - id: risky
    prompt: "risky"
    confirm: true
  - id: after
    prompt: "after"
`))
	require.NoError(t, err)

	runID, err := reg.Start(spec, nil)
	require.NoError(t, err)

	waitForStatus(t, reg, runID, StatusWaiting, 2*time.Second)
	require.NoError(t, reg.Reject(runID, "risky"))
	waitForStatus(t, reg, runID, StatusRejected, 2*time.Second)

	meta, _, err := reg.Get(runID)
	require.NoError(t, err)
	assert.Equal(t, StatusRejected, meta.Steps["risky"].Status)
	assert.Equal(t, StatusSkipped, meta.Steps["after"].Status)
	assert.Equal(t, 0, f.callCount(), "被拒绝的步骤不应执行")
}

func TestRegistry_Cancel(t *testing.T) {
	f := &fakeFactory{delay: 5 * time.Second}
	eng := &Engine{Factory: f, Dir: t.TempDir()}
	reg := NewRegistry(eng)

	spec, err := ParseSpec([]byte(`
name: slow-run
steps:
  - id: a
    prompt: "slow"
`))
	require.NoError(t, err)

	runID, err := reg.Start(spec, nil)
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond) // 让步骤进入 Prompt

	require.NoError(t, reg.Cancel(runID))
	waitForStatus(t, reg, runID, StatusCancelled, 2*time.Second)
}

func waitForStatus(t *testing.T, reg *Registry, runID, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		meta, _, err := reg.Get(runID)
		require.NoError(t, err)
		if meta.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach status %q in time", runID, want)
}
