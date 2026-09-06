package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchGatewayModels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[
			{"id":"glm-4.7","object":"model"},
			{"id":"claude-sonnet-4-6","object":"model"},
			{"id":"","object":"model"}
		]}`))
	}))
	defer ts.Close()

	ids, err := FetchGatewayModels(context.Background(), ts.URL+"/v1", "key")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 空 ID 应被过滤
	if len(ids) != 2 || ids[0] != "glm-4.7" || ids[1] != "claude-sonnet-4-6" {
		t.Fatalf("got %v", ids)
	}
}

func TestFetchGatewayModelsUnreachable(t *testing.T) {
	_, err := FetchGatewayModels(context.Background(), "http://127.0.0.1:1/v1", "")
	if err == nil {
		t.Fatal("unreachable server should return error")
	}
}

func TestMergeGateway(t *testing.T) {
	r := NewDefaultRegistry("")

	added := r.MergeGateway("openai", []string{
		"claude-sonnet-4-6", // 已存在：应保留本地元数据
		"glm-4.7",           // 新模型：应注册
	})

	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	known, ok := r.Get("claude-sonnet-4-6")
	if !ok || known.ContextWindow != 200000 {
		t.Fatalf("known model metadata lost: %+v", known)
	}
	fresh, ok := r.Get("glm-4.7")
	if !ok || fresh.Provider != "openai" || fresh.Name != "glm-4.7" {
		t.Fatalf("gateway model not registered: %+v", fresh)
	}
}
