package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 步骤缓存：签名 → 产出，按工作流名字持久化。
// 签名 = hash(workflowName, stepID, itemKey, renderedPrompt, model)，
// 因此 vars/上游产出/模型任一变化都会 miss，重跑只对真正未变化的步骤免费。

type cacheEntry struct {
	Output    string    `json:"output"`
	Outputs   []string  `json:"outputs,omitempty"`
	RunID     string    `json:"run_id"`
	Step      string    `json:"step"`
	CreatedAt time.Time `json:"created_at"`
}

type stepCache struct {
	mu   sync.Mutex
	path string
	data map[string]cacheEntry
}

func newStepCache(cacheRoot, workflowName string) (*stepCache, error) {
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	c := &stepCache{
		path: filepath.Join(cacheRoot, workflowName+".json"),
		data: make(map[string]cacheEntry),
	}
	data, err := os.ReadFile(c.path)
	if err == nil {
		_ = json.Unmarshal(data, &c.data)
	}
	return c, nil
}

// signature 计算步骤执行的缓存键。
func signature(workflowName, stepID, itemKey, renderedPrompt, model string) string {
	h := sha256.Sum256([]byte(
		workflowName + "\x00" + stepID + "\x00" + itemKey + "\x00" + renderedPrompt + "\x00" + model))
	return hex.EncodeToString(h[:16])
}

func (c *stepCache) get(sig string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[sig]
	return e, ok
}

func (c *stepCache) put(sig string, e cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[sig] = e
	data, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, c.path)
}
