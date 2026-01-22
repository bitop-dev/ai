package main

import (
	"log"
	"net/http"

	"github.com/bitop-dev/ai/pkg/ai"
	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openai"
)

func main() {
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.LanguageModel("gpt-4o-mini")
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		promptText := r.URL.Query().Get("prompt")
		if promptText == "" {
			promptText = "Write a short poem about Go concurrency."
		}
		prompt := provider.Prompt{
			Messages: []provider.ModelMessage{
				{
					Role: provider.RoleUser,
					Content: []provider.ContentPart{
						provider.TextContent{Text: promptText},
					},
				},
			},
		}

		result, err := ai.StreamText(r.Context(), model, ai.StreamTextOptions{Prompt: prompt})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := ai.PipeStream(r.Context(), w, result.Stream); err != nil {
			log.Printf("stream error: %v", err)
		}
	})

	log.Println("listening on :8080 (GET /stream)")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
