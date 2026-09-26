package tools

import (
	"github.com/hwj123hwj/easyagent/sdk/agent"
)

// 编译期断言：只读工具实现 ConcurrencySafeChecker
var _ agent.ConcurrencySafeChecker = (*ReadTool)(nil)
var _ agent.ConcurrencySafeChecker = (*GrepTool)(nil)
var _ agent.ConcurrencySafeChecker = (*FindTool)(nil)
var _ agent.ConcurrencySafeChecker = (*LsTool)(nil)
