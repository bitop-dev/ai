package ai

import internalEmbeddings "github.com/bitop-dev/ai/internal/embeddings"

func CosineSimilarity(a, b []float32) (float64, error) {
	return internalEmbeddings.CosineSimilarity(a, b)
}
