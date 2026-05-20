package agent

import "errors"

// ErrAgentBusy 当 Agent 正在处理请求时，再次调用 Prompt / PromptStream 会返回此错误。
var ErrAgentBusy = errors.New("agent is busy processing another request")
