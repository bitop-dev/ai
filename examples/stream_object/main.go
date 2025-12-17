package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bitop-dev/ai"
	"github.com/bitop-dev/ai/openai"
)

type Recipe struct {
	Recipe struct {
		Name        string   `json:"name"`
		Ingredients []string `json:"ingredients"`
		Steps       []string `json:"steps"`
	} `json:"recipe"`
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
  "required": ["recipe"],
  "properties": {
    "recipe": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "ingredients", "steps"],
      "properties": {
        "name": { "type": "string" },
        "ingredients": { "type": "array", "items": { "type": "string" } },
        "steps": { "type": "array", "items": { "type": "string" } }
      }
    }
  }
}`)

	stream, err := ai.StreamObject[Recipe](context.Background(), ai.StreamObjectRequest[Recipe]{
		BaseRequest: ai.BaseRequest{
			Model: openai.Chat(getenv("OPENAI_MODEL", "gpt-4.1")),
			Messages: []ai.Message{
				ai.User(getenv("PROMPT", "Generate a lasagna recipe.")),
			},
		},
		Schema: ai.JSONSchema(schema),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer stream.Close()

	for stream.Next() {
		if p := stream.Partial(); p != nil {
			b, _ := json.MarshalIndent(p, "", "  ")
			fmt.Print("\033[H\033[2J")
			fmt.Println(string(b))
		}
	}
	if err := stream.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if obj := stream.Object(); obj != nil {
		b, _ := json.MarshalIndent(obj, "", "  ")
		fmt.Println(string(b))
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
