package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/vercel/ai-sdk-go/pkg/ai"
	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providers/openai"
)

func main() {
	filePath := flag.String("file", "", "path to audio file")
	mediaType := flag.String("type", "audio/mpeg", "audio media type")
	flag.Parse()
	if *filePath == "" {
		log.Fatal("missing -file path")
	}

	ctx := context.Background()
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.TranscriptionModel("whisper-1")
	if err != nil {
		log.Fatal(err)
	}

	audio, err := os.ReadFile(*filePath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = ai.Transcribe(ctx, model, provider.TranscriptionModelV3CallOptions{
		Audio:     audio,
		MediaType: *mediaType,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("transcription request sent")
}
