package gateway

import "github.com/vercel/ai-sdk-go/pkg/provider"

type GatewayModelID string

type GatewayEmbeddingModelID string

type GatewayImageModelID string

type GatewayModelPricing struct {
	Input                    string
	Output                   string
	CachedInputTokens        string
	CacheCreationInputTokens string
}

type GatewayModelType string

const (
	GatewayModelTypeLanguage  GatewayModelType = "language"
	GatewayModelTypeEmbedding GatewayModelType = "embedding"
	GatewayModelTypeImage     GatewayModelType = "image"
)

type GatewayLanguageModelSpecification struct {
	SpecificationVersion provider.SpecificationVersion
	ProviderID           provider.ProviderID
	ModelID              provider.ModelID
}

type GatewayLanguageModelEntry struct {
	ID            string
	Name          string
	Description   string
	Pricing       *GatewayModelPricing
	Specification GatewayLanguageModelSpecification
	ModelType     GatewayModelType
}

type GatewayLanguageModelSettings struct {
	ID              GatewayModelID
	Name            string
	Description     string
	Pricing         *GatewayModelPricing
	MaxTokens       int
	MaxInputTokens  int
	MaxOutputTokens int
	SupportsTools   bool
	SupportsVision  bool
	SupportsJSON    bool
}

type GatewayEmbeddingModelSettings struct {
	ID          GatewayEmbeddingModelID
	Name        string
	Description string
	Pricing     *GatewayModelPricing
	MaxTokens   int
	Dimensions  int
}

type GatewayImageModelSettings struct {
	ID           GatewayImageModelID
	Name         string
	Description  string
	Pricing      *GatewayModelPricing
	MaxImages    int
	Sizes        []string
	AspectRatios []string
	SupportsMask bool
}
