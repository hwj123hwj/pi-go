package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoopDetector_ConsecutiveRepeats(t *testing.T) {
	d := loopDetector{}
	const threshold = 5

	// 前 4 次不触发，第 5 次触发
	for i := 1; i <= 4; i++ {
		assert.False(t, d.observe(threshold, "read", `{"path":"/a"}`),
			"第 %d 次不应触发（阈值 5）", i)
	}
	assert.True(t, d.observe(threshold, "read", `{"path":"/a"}`),
		"第 5 次相同调用应触发")
	assert.Equal(t, 5, d.repeatCount)
}

func TestLoopDetector_DifferentCallResets(t *testing.T) {
	d := loopDetector{}
	// 连续 3 次 read /a
	d.observe(5, "read", `{"path":"/a"}`)
	d.observe(5, "read", `{"path":"/a"}`)
	d.observe(5, "read", `{"path":"/a"}`)
	// 换一个不同 call → 计数应重置为 1
	assert.False(t, d.observe(5, "grep", `{"pattern":"x"}`))
	assert.Equal(t, 1, d.repeatCount, "不同工具应重置计数")
	// 再来相同 read 也不应立刻触发（计数从 1 重新开始）
	for i := 0; i < 3; i++ {
		assert.False(t, d.observe(5, "read", `{"path":"/a"}`))
	}
}

func TestLoopDetector_SameNameDifferentArgsNotRepeat(t *testing.T) {
	d := loopDetector{}
	// read /a 与 read /b 同工具不同参数，不应算重复
	d.observe(5, "read", `{"path":"/a"}`)
	assert.False(t, d.observe(5, "read", `{"path":"/b"}`))
	assert.Equal(t, 1, d.repeatCount, "不同参数应视为不同调用")
}

func TestLoopDetector_Reset(t *testing.T) {
	d := loopDetector{}
	for i := 0; i < 5; i++ {
		d.observe(5, "read", `{"path":"/a"}`)
	}
	assert.Equal(t, 5, d.repeatCount)

	d.reset()
	assert.Equal(t, 0, d.repeatCount)
	assert.Equal(t, "", d.lastFingerprint)
	// reset 后重新连续 5 次才触发
	for i := 1; i <= 4; i++ {
		assert.False(t, d.observe(5, "read", `{"path":"/a"}`))
	}
	assert.True(t, d.observe(5, "read", `{"path":"/a"}`))
}

func TestLoopDetector_ThresholdBoundary(t *testing.T) {
	// 阈值 3：第 3 次触发
	d := loopDetector{}
	assert.False(t, d.observe(3, "edit", `{"path":"/x"}`))
	assert.False(t, d.observe(3, "edit", `{"path":"/x"}`))
	assert.True(t, d.observe(3, "edit", `{"path":"/x"}`))
}

func TestDefaultLoopDetectSettings(t *testing.T) {
	s := DefaultLoopDetectSettings()
	assert.True(t, s.Enabled)
	assert.Equal(t, 5, s.Threshold)
	assert.Contains(t, s.ReminderTemplate, "%q")
}

func TestSummarizeArgs(t *testing.T) {
	assert.Equal(t, "(none)", summarizeArgs(""))
	assert.Equal(t, `(none)`, summarizeArgs("   \n  "))
	short := `{"path":"/a"}`
	assert.Equal(t, short, summarizeArgs(short))
	// 超长截断
	long := strings.Repeat("x", 100)
	got := summarizeArgs(long)
	assert.Len(t, got, 83) // 80 + "..."
	assert.True(t, got != long)
}
