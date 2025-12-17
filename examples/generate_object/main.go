package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bitop-dev/ai"
	"github.com/bitop-dev/ai/openai"
)

type Holiday struct {
	Name       string   `json:"name"`
	Traditions []string `json:"traditions"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY is required")
		os.Exit(1)
	}

	openai.Configure(openai.Config{
		APIKey:    apiKey,
		BaseURL:   getenv("OPENAI_BASE_URL", ""),
		APIPrefix: getenv("OPENAI_API_PREFIX", ""),
	})

	schema := []byte(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["name", "traditions"],
  "properties": {
    "name": { "type": "string" },
    "traditions": { "type": "array", "items": { "type": "string" } }
  }
}`)

	resp, err := ai.GenerateObject[Holiday](context.Background(), ai.GenerateObjectRequest[Holiday]{
		BaseRequest: ai.BaseRequest{
			Model: openai.Chat(getenv("OPENAI_MODEL", "gpt-5-mini")),
			Messages: []ai.Message{
				ai.User(getenv("PROMPT", "Invent a new holiday and describe its traditions.")),
			},
		},
		Schema: ai.JSONSchema(schema),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(resp.Object, "", "  ")
	fmt.Println(string(out))
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
