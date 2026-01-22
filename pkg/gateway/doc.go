// Package gateway provides the default Gateway provider implementation.
//
// Example:
//
//	provider := gateway.CreateGateway()
//	model, _ := provider.LanguageModel("openai/gpt-4o")
//	result, _ := model.DoStream(ctx, provider.LanguageModelV3CallOptions{
//		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
//			Role:    provider.RoleUser,
//			Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}},
//		}}},
//	})
//	for part := range result.Stream {
//		_ = part
//	}
package gateway
