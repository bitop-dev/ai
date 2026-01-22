package main

import (
	"context"
	"log"

	"github.com/bitop-dev/ai/pkg/ai"
	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openai"
)

type localReranker struct{}

func (localReranker) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionCurrent
}
func (localReranker) ProviderID() provider.ProviderID { return "local" }
func (localReranker) ModelID() provider.ModelID       { return "demo-reranker" }
func (localReranker) DoRerank(ctx context.Context, options provider.RerankingModelCallOptions) (provider.RerankingModelResult, error) {
	return provider.RerankingModelResult{}, nil
}

func main() {
	ctx := context.Background()
	openaiProvider := openai.CreateOpenAI(openai.Settings{})
	model, err := openaiProvider.EmbeddingModel("text-embedding-3-small")
	if err != nil {
		log.Fatal(err)
	}

	_, err = ai.Embed(ctx, model, provider.EmbeddingModelCallOptions{
		Values: []string{
			"Go makes concurrency approachable.",
			"The cloud skyline glowed at dusk.",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("embedding request sent")

	_, err = ai.Rerank(ctx, localReranker{}, provider.RerankingModelCallOptions{})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("rerank request complete")
}
