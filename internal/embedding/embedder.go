// Package embedding provides pluggable embedding providers behind a
// common interface so the runtime can be swapped between OpenAI and
// Cohere without touching the game logic.
package embedding

import (
	"context"
	"fmt"
	"os"
)

// Embedder returns an embedding vector for each input word, in the same
// order. Implementations decide model, dimensions, tokenization, and
// batch semantics — callers see only float32 slices.
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// NewFromEnv wires up an Embedder from environment variables:
//
//	EMBED_PROVIDER = openai (default) | cohere
//	OPENAI_API_KEY = required when provider is openai
//	COHERE_API_KEY = required when provider is cohere
//	COHERE_MODEL   = optional, defaults to embed-multilingual-v3.0
//	COHERE_INPUT_TYPE = optional, defaults to search_document
func NewFromEnv() (Embedder, error) {
	provider := os.Getenv("EMBED_PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	switch provider {
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is required when EMBED_PROVIDER=openai")
		}
		return NewClient(key), nil
	case "cohere":
		key := os.Getenv("COHERE_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("COHERE_API_KEY is required when EMBED_PROVIDER=cohere")
		}
		c := NewCohereClient(key)
		if m := os.Getenv("COHERE_MODEL"); m != "" {
			c.Model = m
		}
		if t := os.Getenv("COHERE_INPUT_TYPE"); t != "" {
			c.InputType = t
		}
		return c, nil
	default:
		return nil, fmt.Errorf("unknown EMBED_PROVIDER %q (want openai or cohere)", provider)
	}
}
