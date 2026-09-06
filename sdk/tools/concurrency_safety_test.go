package tools

import (
	"github.com/hwj123hwj/pi-go/sdk/agent"
)

// 编译期断言：只读工具实现 ConcurrencySafeChecker
var _ agent.ConcurrencySafeChecker = (*ReadTool)(nil)
var _ agent.ConcurrencySafeChecker = (*GrepTool)(nil)
var _ agent.ConcurrencySafeChecker = (*FindTool)(nil)
var _ agent.ConcurrencySafeChecker = (*LsTool)(nil)
