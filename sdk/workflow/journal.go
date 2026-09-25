package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// journal 事件类型。
const (
	EventRunStart   = "run_start"
	EventStepStart  = "step_start"
	EventStepEnd    = "step_end"
	EventItemEnd    = "item_end"
	EventStepError  = "step_error"
	EventGateWait   = "gate_wait"
	EventGateResult = "gate_result"
	EventRunEnd     = "run_end"
)

// Event 是 journal.jsonl 的一行，也是 GET /workflows/{id} 返回的事件记录。
type Event struct {
	Ts         time.Time      `json:"ts"`
	Event      string         `json:"event"`
	Step       string         `json:"step,omitempty"`
	Item       *int           `json:"item,omitempty"`
	Attempt    int            `json:"attempt,omitempty"`
	Signature  string         `json:"signature,omitempty"`
	Prompt     string         `json:"prompt,omitempty"`
	Output     string         `json:"output,omitempty"`
	Cached     bool           `json:"cached,omitempty"`
	Error      string         `json:"error,omitempty"`
	Status     string         `json:"status,omitempty"` // run_end 时为最终状态
	Name       string         `json:"name,omitempty"`
	Vars       map[string]any `json:"vars,omitempty"`
	DurationMs int64          `json:"duration_ms,omitempty"`
}

// StepMeta 是 meta.json 中单个步骤的聚合状态。
type StepMeta struct {
	Status     string `json:"status"`
	Cached     bool   `json:"cached,omitempty"`
	ItemsDone  int    `json:"items_done,omitempty"`
	ItemsTotal int    `json:"items_total,omitempty"`
	Error      string `json:"error,omitempty"`
}

// RunMeta 是一次运行的聚合状态（data/workflows/<run-id>/meta.json）。
type RunMeta struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Status     string               `json:"status"`
	Error      string               `json:"error,omitempty"`
	Vars       map[string]any       `json:"vars,omitempty"`
	StartedAt  time.Time            `json:"started_at"`
	FinishedAt *time.Time           `json:"finished_at,omitempty"`
	Steps      map[string]StepMeta  `json:"steps"`
	Outputs    map[string]StepOutput `json:"outputs,omitempty"` // 完成步骤的最终产出
}

// journal 是单次运行的事件写入器 + meta 维护器，并发安全。
type journal struct {
	mu   sync.Mutex
	dir  string
	file *os.File
	meta RunMeta
}

func newJournal(runsRoot, runID, workflowName string, vars map[string]any, stepIDs []string) (*journal, error) {
	dir := filepath.Join(runsRoot, runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "journal.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	now := time.Now()
	meta := RunMeta{
		ID:        runID,
		Name:      workflowName,
		Status:    StatusRunning,
		Vars:      vars,
		StartedAt: now,
		Steps:     make(map[string]StepMeta, len(stepIDs)),
	}
	for _, id := range stepIDs {
		meta.Steps[id] = StepMeta{Status: "pending"}
	}
	j := &journal{dir: dir, file: f, meta: meta}
	j.emit(Event{Ts: now, Event: EventRunStart, Name: workflowName, Vars: vars})
	if err := j.saveMeta(); err != nil {
		f.Close()
		return nil, err
	}
	return j, nil
}

// emit 追加一条事件；写失败只影响可观测性，不中断运行。
func (j *journal) emit(e Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = j.file.Write(append(data, '\n'))
}

// updateMeta 原子修改 meta 并落盘。
func (j *journal) updateMeta(fn func(m *RunMeta)) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	fn(&j.meta)
	return j.saveMetaLocked()
}

func (j *journal) saveMeta() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.saveMetaLocked()
}

func (j *journal) saveMetaLocked() error {
	data, err := json.MarshalIndent(j.meta, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(j.dir, "meta.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	return os.Rename(tmp, filepath.Join(j.dir, "meta.json"))
}

func (j *journal) close() error {
	return j.file.Close()
}
