package ai

import (
	"fmt"
	"math"
)

func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, fmt.Errorf("vectors must be non-empty")
	}
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector length mismatch: %d != %d", len(a), len(b))
	}

	var dot, na, nb float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0, fmt.Errorf("zero vector")
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb)), nil
}
