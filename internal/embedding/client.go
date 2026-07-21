package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultModel   = "text-embedding-3-large"
	defaultBaseURL = "https://api.openai.com/v1"
	maxBatchSize   = 2048

	// text-embedding-3-large native dimension. Full-fat vector — the
	// small model at 512 dim gave nonsense pairs like fotka × fotbal
	// (subword prefix bias); the larger model at native width separates
	// morphologically similar but semantically unrelated Czech words.
	Dimensions = 3072
)

type Client struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   defaultModel,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

type embeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed returns embeddings for the given inputs, in the same order.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	result := make([][]float32, 0, len(inputs))
	for start := 0; start < len(inputs); start += maxBatchSize {
		end := min(start+maxBatchSize, len(inputs))
		batch, err := c.embedBatch(ctx, inputs[start:end])
		if err != nil {
			return nil, err
		}
		result = append(result, batch...)
	}
	return result, nil
}

func (c *Client) embedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(embeddingRequest{Model: c.model, Input: inputs, Dimensions: Dimensions})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("openai api error (status %d): %s", resp.StatusCode, parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai api status %d", resp.StatusCode)
	}
	if len(parsed.Data) != len(inputs) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(inputs), len(parsed.Data))
	}

	out := make([][]float32, len(inputs))
	for _, d := range parsed.Data {
		out[d.Index] = d.Embedding
	}
	return out, nil
}
