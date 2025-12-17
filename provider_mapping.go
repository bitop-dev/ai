package ai

import (
	"encoding/json"
	"fmt"

	"github.com/bitop-dev/ai/internal/provider"
)

func fromProviderResponse(resp provider.Response) (Message, Usage, FinishReason, error) {
	msg, err := fromProviderMessage(resp.Message)
	if err != nil {
		return Message{}, Usage{}, FinishReason(""), err
	}
	return msg, Usage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}, FinishReason(resp.FinishReason), nil
}

func fromProviderMessage(m provider.Message) (Message, error) {
	r, err := fromProviderRole(m.Role)
	if err != nil {
		return Message{}, err
	}
	parts, err := fromProviderContentParts(m.Content)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Role:       r,
		Content:    parts,
		Name:       m.Name,
		ToolCallID: m.ToolCallID,
	}, nil
}

func fromProviderRole(r provider.Role) (Role, error) {
	switch r {
	case provider.RoleSystem:
		return RoleSystem, nil
	case provider.RoleUser:
		return RoleUser, nil
	case provider.RoleAssistant:
		return RoleAssistant, nil
	case provider.RoleTool:
		return RoleTool, nil
	default:
		return "", fmt.Errorf("unknown provider role %q", r)
	}
}

func fromProviderContentParts(parts []provider.ContentPart) ([]ContentPart, error) {
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]ContentPart, 0, len(parts))
	for _, p := range parts {
		switch v := p.(type) {
		case provider.TextPart:
			out = append(out, TextPart{Text: v.Text})
		case provider.ToolCallPart:
			out = append(out, ToolCallPart{ID: v.ID, Name: v.Name, Args: json.RawMessage(v.Args)})
		default:
			return nil, fmt.Errorf("unknown provider content part type %T", p)
		}
	}
	return out, nil
}

func extractTextFromMessage(m Message) string {
	var b []byte
	for _, p := range m.Content {
		if t, ok := p.(TextPart); ok {
			b = append(b, t.Text...)
		}
	}
	return string(b)
}
