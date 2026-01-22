package main

import (
	"context"
	"log"

	"github.com/vercel/ai-sdk-go/pkg/ai"
	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providers/openai"
)

type localReranker struct{}

func (localReranker) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}
func (localReranker) ProviderID() provider.ProviderID { return "local" }
func (localReranker) ModelID() provider.ModelID       { return "demo-reranker" }
func (localReranker) DoRerank(ctx context.Context, options provider.RerankingModelV3CallOptions) (provider.RerankingModelV3Result, error) {
	return provider.RerankingModelV3Result{}, nil
}

func main() {
	ctx := context.Background()
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.EmbeddingModel("text-embedding-3-small")
	if err != nil {
		log.Fatal(err)
	}

	_, err = ai.Embed(ctx, model, provider.EmbeddingModelV3CallOptions{
		Values: []string{
			"Go makes concurrency approachable.",
			"The cloud skyline glowed at dusk.",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("embedding request sent")

	_, err = ai.Rerank(ctx, localReranker{}, provider.RerankingModelV3CallOptions{})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("rerank request complete")
}
