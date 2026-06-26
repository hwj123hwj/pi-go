package kbvector

// ────────────────────────────────────────────────────────────────────────────
//  Embedding client for SiliconFlow (OpenAI-compatible /v1/embeddings).
//
//  Uses BAAI/bge-m3 model (1024-dim) for semantic search of the KB.
//  Batch API supported to embed multiple texts in one request.
// ────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL   = "https://api.siliconflow.cn"
	defaultModel     = "BAAI/bge-m3"
	defaultTimeout   = 30 * time.Second
	maxBatchSize     = 64 // SiliconFlow batch limit
)

// EmbeddingClient calls the SiliconFlow embedding API.
type EmbeddingClient struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// NewEmbeddingClient creates a client. If apiKey is empty, returns nil-safe client.
func NewEmbeddingClient(apiKey, baseURL, model string) *EmbeddingClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if model == "" {
		model = defaultModel
	}
	return &EmbeddingClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// Available returns true if the client has an API key configured.
func (c *EmbeddingClient) Available() bool {
	return c != nil && c.apiKey != ""
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Embed sends texts to the API and returns their vector embeddings.
// Automatically batches requests if len(texts) > maxBatchSize.
func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if !c.Available() {
		return nil, fmt.Errorf("embedding client not configured (no API key)")
	}

	if len(texts) == 0 {
		return nil, nil
	}

	var allEmbeddings [][]float32
	// Process in batches
	for start := 0; start < len(texts); start += maxBatchSize {
		end := start + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]

		embeddings, err := c.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch embed failed (offset %d): %w", start, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// EmbedOne embeds a single text. Convenience wrapper.
func (c *EmbeddingClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	embs, err := c.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embs) == 0 {
		return nil, fmt.Errorf("empty response")
	}
	return embs[0], nil
}

func (c *EmbeddingClient) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := embeddingRequest{
		Model: c.model,
		Input: texts,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	// Sort by index to ensure correct order
	embeddings := make([][]float32, len(result.Data))
	for _, item := range result.Data {
		if item.Index >= 0 && item.Index < len(embeddings) {
			embeddings[item.Index] = item.Embedding
		}
	}

	return embeddings, nil
}

// Dim returns the expected embedding dimension for bge-m3.
func Dim() int { return 1024 }
