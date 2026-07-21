package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Cohere config defaults for the v3 multilingual model. That model
// produces 1024-dim vectors and accepts up to 96 inputs per call.
const (
	defaultCohereBaseURL   = "https://api.cohere.com/v2"
	defaultCohereModel     = "embed-multilingual-v3.0"
	defaultCohereInputType = "search_document"
	cohereMaxBatchSize     = 96
)

type CohereClient struct {
	apiKey    string
	baseURL   string
	Model     string
	InputType string
	http      *http.Client
}

func NewCohereClient(apiKey string) *CohereClient {
	return &CohereClient{
		apiKey:    apiKey,
		baseURL:   defaultCohereBaseURL,
		Model:     defaultCohereModel,
		InputType: defaultCohereInputType,
		http:      &http.Client{Timeout: 60 * time.Second},
	}
}

type cohereRequest struct {
	Model          string   `json:"model"`
	Texts          []string `json:"texts"`
	InputType      string   `json:"input_type"`
	EmbeddingTypes []string `json:"embedding_types"`
}

type cohereResponse struct {
	Embeddings struct {
		Float [][]float32 `json:"float"`
	} `json:"embeddings"`
	Message string `json:"message"` // populated on error responses
}

func (c *CohereClient) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	result := make([][]float32, 0, len(inputs))
	for start := 0; start < len(inputs); start += cohereMaxBatchSize {
		end := min(start+cohereMaxBatchSize, len(inputs))
		batch, err := c.embedBatch(ctx, inputs[start:end])
		if err != nil {
			return nil, err
		}
		result = append(result, batch...)
	}
	return result, nil
}

func (c *CohereClient) embedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(cohereRequest{
		Model:          c.Model,
		Texts:          inputs,
		InputType:      c.InputType,
		EmbeddingTypes: []string{"float"},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed cohereResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode cohere response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if parsed.Message != "" {
			return nil, fmt.Errorf("cohere api error (status %d): %s", resp.StatusCode, parsed.Message)
		}
		return nil, fmt.Errorf("cohere api status %d", resp.StatusCode)
	}
	if len(parsed.Embeddings.Float) != len(inputs) {
		return nil, fmt.Errorf("cohere: expected %d embeddings, got %d", len(inputs), len(parsed.Embeddings.Float))
	}
	return parsed.Embeddings.Float, nil
}
