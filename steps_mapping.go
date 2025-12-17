package ai

import (
	"github.com/bitop-dev/ai/internal/provider"
	internalText "github.com/bitop-dev/ai/internal/text"
)

func stepFromProviderStep(s internalText.Step) (Step, error) {
	msg, err := fromProviderMessage(s.Response.Message)
	if err != nil {
		return Step{}, err
	}

	toolCalls := make([]ToolCallPart, 0, len(s.ToolCalls))
	for _, tc := range s.ToolCalls {
		toolCalls = append(toolCalls, ToolCallPart{ID: tc.ID, Name: tc.Name, Args: tc.Args})
	}

	toolResults, err := messagesFromProviderMessages(s.ToolResults)
	if err != nil {
		return Step{}, err
	}

	return Step{
		StepNumber:   s.StepNumber,
		Text:         extractTextFromMessage(msg),
		Message:      msg,
		ToolCalls:    toolCalls,
		ToolResults:  toolResults,
		FinishReason: FinishReason(s.Response.FinishReason),
		Usage: Usage{
			PromptTokens:     s.Response.Usage.PromptTokens,
			CompletionTokens: s.Response.Usage.CompletionTokens,
			TotalTokens:      s.Response.Usage.TotalTokens,
		},
		ActiveTools: append([]string(nil), s.ActiveTools...),
	}, nil
}

func stepsFromProviderSteps(steps []internalText.Step) ([]Step, error) {
	if len(steps) == 0 {
		return nil, nil
	}
	out := make([]Step, 0, len(steps))
	for _, s := range steps {
		step, err := stepFromProviderStep(s)
		if err != nil {
			return nil, err
		}
		out = append(out, step)
	}
	return out, nil
}

func messagesFromProviderMessages(msgs []provider.Message) ([]Message, error) {
	if len(msgs) == 0 {
		return nil, nil
	}
	out := make([]Message, 0, len(msgs))
	for _, pm := range msgs {
		m, err := fromProviderMessage(pm)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
