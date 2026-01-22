package anthropic

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestAnthropicIntegrationGenerate(t *testing.T) {
	apiKey := requireAnthropicAPIKey(t)
	modelID := anthropicIntegrationModel()

	client := CreateAnthropic(Settings{APIKey: apiKey, BaseURL: anthropicIntegrationBaseURL()})
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

func TestAnthropicIntegrationStream(t *testing.T) {
	apiKey := requireAnthropicAPIKey(t)
	modelID := anthropicIntegrationModel()

	client := CreateAnthropic(Settings{APIKey: apiKey, BaseURL: anthropicIntegrationBaseURL()})
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

func requireAnthropicAPIKey(t *testing.T) string {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY is not set")
	}
	return apiKey
}

func anthropicIntegrationModel() string {
	if model := os.Getenv("ANTHROPIC_INTEGRATION_MODEL"); model != "" {
		return model
	}
	return "claude-3-haiku-20240307"
}

func anthropicIntegrationBaseURL() string {
	if baseURL := os.Getenv("ANTHROPIC_INTEGRATION_BASE_URL"); baseURL != "" {
		return baseURL
	}
	return ""
}
