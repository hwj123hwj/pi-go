package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FetchGatewayModels 从 OpenAI 兼容端点拉取模型列表（GET {baseURL}/models）。
// 适配 LiteLLM / pi-go 网关等任何返回 {"data":[{"id":"..."}]} 的端点。
// 网关不可达或响应异常时返回 error，调用方降级为本地清单，不阻塞启动。
func FetchGatewayModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	url := strings.TrimRight(baseURL, "/") + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	ids := make([]string, 0, len(out.Data))
	for _, m := range out.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// MergeGateway 把网关返回的模型合并进注册表：
// 已存在的 ID 保留本地定义（含上下文窗口等元数据）不动；
// 网关新增的 ID 以最小定义注册（Name 即 ID，元数据未知置零）。
// 返回本次新增的模型数。
func (r *Registry) MergeGateway(provider string, ids []string) int {
	added := 0
	for _, id := range ids {
		if _, exists := r.models[id]; exists {
			continue
		}
		r.Register(ModelDef{
			ID:      id,
			Provider: provider,
			Name:    id,
		})
		added++
	}
	return added
}
