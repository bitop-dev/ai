package ai

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bitop-dev/ai/openai"
	"github.com/joho/godotenv"
)

func requireIntegration(t *testing.T) {
	t.Helper()

	_ = godotenv.Load()

	if os.Getenv("AI_INTEGRATION") == "" {
		t.Skip("set AI_INTEGRATION=1 to run integration tests")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("set OPENAI_API_KEY to run integration tests")
	}
}

func configureOpenAIFromEnv() {
	openai.Configure(openai.Config{
		APIKey:    os.Getenv("OPENAI_API_KEY"),
		BaseURL:   os.Getenv("OPENAI_BASE_URL"),
		APIPrefix: os.Getenv("OPENAI_API_PREFIX"),
	})
}

func TestIntegration_GenerateText(t *testing.T) {
	requireIntegration(t)
	configureOpenAIFromEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4.1"
	}

	resp, err := GenerateText(ctx, GenerateTextRequest{
		BaseRequest: BaseRequest{
			Model: openai.Chat(model),
			Messages: []Message{
				User("Say the word 'ok' and nothing else."),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text == "" {
		t.Fatalf("expected non-empty text")
	}
}

func TestIntegration_StreamText(t *testing.T) {
	requireIntegration(t)
	configureOpenAIFromEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4.1"
	}

	stream, err := StreamText(ctx, StreamTextRequest{
		BaseRequest: BaseRequest{
			Model: openai.Chat(model),
			Messages: []Message{
				User("Write exactly 10 words, separated by spaces."),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var combined string
	for stream.Next() {
		combined += stream.Delta()
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if combined == "" {
		t.Fatalf("expected some streamed text")
	}
	if msg := stream.Message(); msg == nil {
		t.Fatalf("expected final message")
	}
}

func TestIntegration_GenerateObject(t *testing.T) {
	requireIntegration(t)
	configureOpenAIFromEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4.1"
	}

	type out struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}

	schema := JSONSchema([]byte(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["foo", "bar"],
  "properties": {
    "foo": { "type": "string" },
    "bar": { "type": "integer" }
  }
}`))

	resp, err := GenerateObject[out](ctx, GenerateObjectRequest[out]{
		BaseRequest: BaseRequest{
			Model: openai.Chat(model),
			Messages: []Message{
				User("Return foo='hello' and bar=7."),
			},
		},
		Schema: schema,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Object.Foo == "" || resp.Object.Bar == 0 {
		t.Fatalf("unexpected object: %#v", resp.Object)
	}
	if !json.Valid(resp.RawJSON) {
		t.Fatalf("RawJSON is not valid JSON: %s", string(resp.RawJSON))
	}
}
