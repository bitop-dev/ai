package mcp

import (
	"encoding/base64"
	"fmt"

	"github.com/bitop-dev/ai"
)

// PromptMessagesToAIMessages converts MCP prompt messages into ai.Messages.
// Unknown roles are mapped to ai.RoleUser.
func PromptMessagesToAIMessages(prompt *GetPromptResult) []ai.Message {
	if prompt == nil || len(prompt.Messages) == 0 {
		return nil
	}
	out := make([]ai.Message, 0, len(prompt.Messages))
	for _, m := range prompt.Messages {
		switch m.Role {
		case "system":
			out = append(out, ai.System(m.Content))
		case "assistant":
			out = append(out, ai.Assistant(m.Content))
		case "user":
			out = append(out, ai.User(m.Content))
		default:
			out = append(out, ai.User(m.Content))
		}
	}
	return out
}

// ResourceToSystemMessages converts MCP resource contents into ai.System messages.
// Text contents are included directly; blob contents are included as base64 (and
// decoded bytes are omitted to avoid surprises in prompts).
func ResourceToSystemMessages(resource *ReadResourceResult) []ai.Message {
	if resource == nil || len(resource.Contents) == 0 {
		return nil
	}
	out := make([]ai.Message, 0, len(resource.Contents))
	for _, c := range resource.Contents {
		if c.Text != "" {
			out = append(out, ai.System(fmt.Sprintf("MCP resource %s:\n%s", c.URI, c.Text)))
			continue
		}
		if c.BlobBase64 != "" {
			// Ensure it's valid base64 to avoid injecting invalid data.
			if _, err := base64.StdEncoding.DecodeString(c.BlobBase64); err != nil {
				out = append(out, ai.System(fmt.Sprintf("MCP resource %s: (invalid base64 blob)", c.URI)))
				continue
			}
			mt := c.MediaType
			if mt == "" {
				mt = "application/octet-stream"
			}
			out = append(out, ai.System(fmt.Sprintf("MCP resource %s (%s) base64:\n%s", c.URI, mt, c.BlobBase64)))
		}
	}
	return out
}
