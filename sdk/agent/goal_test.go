package agent

import "testing"

func TestGoalCompleted(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		// Should NOT match
		{"empty text", "", false},
		{"unrelated text", "I have read the file and found some issues.", false},
		{"partial match should not trigger", "The goal is to optimize performance.", false},
		{"working on it", "I'm working on the goal. So far I've done 3 things.", false},
		{"goal mentioned but not done", "The goal requires more work.", false},
		{"vague all done", "All done!", false},
		{"vague task completed", "Task completed.", false},
		{"vague goal achieved", "The goal achieved! All optimizations are done.", false},
		{"vague goal is complete", "The goal is complete now.", false},
		{"vague 全部完成", "全部完成，以下是总结。", false},
		{"vague 任务完成", "所有任务完成。", false},

		// Should match — explicit, unambiguous completion signals
		{"goal has been achieved", "The goal has been achieved.", true},
		{"Goal has been completed caps", "Goal has been completed!", true},
		{"objective has been achieved", "The objective has been achieved successfully.", true},
		{"all tasks have been completed", "All tasks have been completed.", true},
		{"goal is now complete", "The goal is now complete.", true},
		{"chinese 目标已达成", "目标已达成。", true},
		{"chinese 目标已实现", "目标已实现。", true},
		{"chinese 全部实施完毕", "全部实施完毕。", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := goalCompleted(tt.text)
			if result != tt.expected {
				t.Errorf("goalCompleted(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}
