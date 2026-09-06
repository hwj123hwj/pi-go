package sdk_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestSDKDoesNotImportInternal 强制执行架构规则：
// sdk/ 是公共 API 面，不得依赖 pi-go 自身的应用层（internal/）。
// 这条测试把 PROJECT_CONTEXT.md 的层间规则变成 CI 可拦截的硬约束。
func TestSDKDoesNotImportInternal(t *testing.T) {
	// 测试运行时 cwd 就是 sdk/，./... 覆盖 sdk 全部子包
	out, err := exec.Command("go", "list", "-deps", "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps ./... failed: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		// 只匹配本模块的 internal/，标准库自己的 crypto/internal 等不算
		if strings.HasPrefix(line, "github.com/hwj123hwj/pi-go/internal/") {
			t.Errorf("sdk 包不得依赖 internal/，发现: %s", line)
		}
	}
}
