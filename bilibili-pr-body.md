## Summary

B站搜索首条质量参差——实测搜"Love Story"首条是动态鼓谱教学，搜"七里香"首条可能是 reaction。当前 playByQuery 取首条就播，会播到错内容。

### 变更内容
在 bilibili client Search 内部加两道闸门（return 前过滤），所有上层调用自动生效：
- **黑名单过滤**：按 UP 主名、标题关键词屏蔽低质量内容
- **同名过滤**：排除与搜索词无关的同名/ remix 版本

### 文件变更
- `internal/music/bilibili/filter.go` — 过滤逻辑（新增）
- `internal/music/bilibili/filter_test.go` — 单测（新增）
- `internal/music/bilibili/search.go` — 接入过滤（修改）
