package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestOpenAIIntegrationGenerate(t *testing.T) {
	apiKey := requireOpenAIAPIKey(t)
	modelID := openAIIntegrationModel()

	client := CreateOpenAI(Settings{APIKey: apiKey, BaseURL: openAIIntegrationBaseURL()})
	model, err := client.LanguageModel(provider.ModelID(modelID))
	if err != nil {
		t.Fatalf("language model: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	_, err = model.DoGenerate(ctx, provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{
			Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "Say hello in two words."}},
			}},
		},
		MaxOutputTokens: 16,
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestOpenAIIntegrationStream(t *testing.T) {
	apiKey := requireOpenAIAPIKey(t)
	modelID := openAIIntegrationModel()

	client := CreateOpenAI(Settings{APIKey: apiKey, BaseURL: openAIIntegrationBaseURL()})
	model, err := client.LanguageModel(provider.ModelID(modelID))
	if err != nil {
		t.Fatalf("language model: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result, err := model.DoStream(ctx, provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{
			Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "Write a short greeting."}},
			}},
		},
		MaxOutputTokens: 24,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var sawText bool
	var sawFinish bool
	for part := range result.Stream {
		if part.Type == provider.StreamPartTypeError && part.Error != nil {
			t.Fatalf("stream error: %v", part.Error.Err)
		}
		if part.TextStart != nil || part.TextDelta != nil {
			sawText = true
		}
		if part.Finish != nil {
			sawFinish = true
		}
	}

	if !sawText {
		t.Fatalf("expected text parts in stream")
	}
	if !sawFinish {
		t.Fatalf("expected finish part in stream")
	}
}

func requireOpenAIAPIKey(t *testing.T) string {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}
	return apiKey
}

func openAIIntegrationModel() string {
	if model := os.Getenv("OPENAI_INTEGRATION_MODEL"); model != "" {
		return model
	}
	return "gpt-4o-mini"
}

func openAIIntegrationBaseURL() string {
	if baseURL := os.Getenv("OPENAI_INTEGRATION_BASE_URL"); baseURL != "" {
		return baseURL
	}
	return ""
}
