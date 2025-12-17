package ai

import (
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/openai"
)

func toProviderRequest(req BaseRequest) (provider.Request, error) {
	if req.Model == nil {
		return provider.Request{}, fmt.Errorf("model is required")
	}
	if req.Model.Name() == "" {
		return provider.Request{}, fmt.Errorf("model name is required")
	}

	msgs, err := toProviderMessages(req.Messages)
	if err != nil {
		return provider.Request{}, err
	}

	tools, err := toProviderTools(req.Tools)
	if err != nil {
		return provider.Request{}, err
	}

	var providerData any
	if c, ok := openAIClientFromModel(req.Model); ok {
		providerData = c
	}

	return provider.Request{
		Model:        req.Model.Name(),
		Messages:     msgs,
		Tools:        tools,
		Headers:      cloneStringMap(req.Headers),
		MaxRetries:   req.MaxRetries,
		ProviderData: providerData,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		TopP:         req.TopP,
		Stop:         append([]string(nil), req.Stop...),
		Metadata:     cloneStringMap(req.Metadata),
	}, nil
}

type openAIClientModel interface {
	Client() *openai.Client
}

func openAIClientFromModel(m ModelRef) (*openai.Client, bool) {
	v, ok := m.(openAIClientModel)
	if !ok || v.Client() == nil {
		return nil, false
	}
	return v.Client(), true
}

func toProviderTools(tools []Tool) ([]provider.ToolDefinition, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]provider.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		if t.Name == "" {
			return nil, fmt.Errorf("tool name is required")
		}
		out = append(out, provider.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema.JSON,
		})
	}
	return out, nil
}

func toProviderMessages(msgs []Message) ([]provider.Message, error) {
	if len(msgs) == 0 {
		return nil, nil
	}
	out := make([]provider.Message, 0, len(msgs))
	for _, m := range msgs {
		pm, err := toProviderMessage(m)
		if err != nil {
			return nil, err
		}
		out = append(out, pm)
	}
	return out, nil
}

func toProviderMessage(m Message) (provider.Message, error) {
	pr, err := toProviderRole(m.Role)
	if err != nil {
		return provider.Message{}, err
	}
	content, err := toProviderContentParts(m.Content)
	if err != nil {
		return provider.Message{}, err
	}
	return provider.Message{
		Role:       pr,
		Content:    content,
		Name:       m.Name,
		ToolCallID: m.ToolCallID,
	}, nil
}

func toProviderRole(r Role) (provider.Role, error) {
	switch r {
	case RoleSystem:
		return provider.RoleSystem, nil
	case RoleUser:
		return provider.RoleUser, nil
	case RoleAssistant:
		return provider.RoleAssistant, nil
	case RoleTool:
		return provider.RoleTool, nil
	default:
		return "", fmt.Errorf("unknown role %q", r)
	}
}

func toProviderContentParts(parts []ContentPart) ([]provider.ContentPart, error) {
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]provider.ContentPart, 0, len(parts))
	for _, p := range parts {
		switch v := p.(type) {
		case TextPart:
			out = append(out, provider.TextPart{Text: v.Text})
		case ToolCallPart:
			out = append(out, provider.ToolCallPart{ID: v.ID, Name: v.Name, Args: v.Args})
		case ImagePart:
			out = append(out, provider.ImagePart{URL: v.URL, MediaType: v.MediaType, Bytes: append([]byte(nil), v.Bytes...), Base64: v.Base64})
		case AudioPart:
			out = append(out, provider.AudioPart{Format: v.Format, Bytes: append([]byte(nil), v.Bytes...), Base64: v.Base64})
		default:
			return nil, fmt.Errorf("unknown content part type %T", p)
		}
	}
	return out, nil
}

func cloneStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
