package ai

import (
	"strings"

	"github.com/bitop-dev/ai/internal/provider"
)

type toolInputLifecycle struct {
	toolsByName map[string]Tool

	byIndex   map[int]*toolInputState
	indexByID map[string]int
}

type toolInputState struct {
	id      string
	name    string
	started bool
	buffer  strings.Builder
}

func newToolInputLifecycle(tools []Tool) *toolInputLifecycle {
	byName := make(map[string]Tool, len(tools))
	for _, t := range tools {
		if t.Name == "" {
			continue
		}
		byName[t.Name] = t
	}
	return &toolInputLifecycle{
		toolsByName: byName,
		byIndex:     map[int]*toolInputState{},
		indexByID:   map[string]int{},
	}
}

func (l *toolInputLifecycle) onDelta(d provider.Delta) {
	if l == nil {
		return
	}
	for _, tc := range d.ToolCalls {
		l.onToolCallDelta(tc)
	}
}

func (l *toolInputLifecycle) toolCallIndexByID(toolCallID string) int {
	if l == nil {
		return -1
	}
	if i, ok := l.indexByID[toolCallID]; ok {
		return i
	}
	return -1
}

func (l *toolInputLifecycle) onToolCallDelta(tc provider.ToolCallDelta) {
	if l == nil {
		return
	}

	st := l.byIndex[tc.Index]
	if st == nil {
		st = &toolInputState{}
		l.byIndex[tc.Index] = st
	}

	if tc.ID != "" {
		st.id = tc.ID
		l.indexByID[tc.ID] = tc.Index
	}
	if tc.Name != "" {
		st.name = tc.Name
	}

	tool, ok := l.toolsByName[st.name]
	if !ok {
		// Tool unknown or name not available yet; buffer args until we can map.
		if !st.started && tc.ArgumentsDelta != "" {
			st.buffer.WriteString(tc.ArgumentsDelta)
		}
		return
	}

	// Start event: first time we can resolve the tool name.
	if !st.started {
		st.started = true
		if tool.OnInputStart != nil {
			tool.OnInputStart(ToolInputStartEvent{
				ToolName:      st.name,
				ToolCallID:    st.id,
				ToolCallIndex: tc.Index,
			})
		}
		if tool.OnInputDelta != nil && st.buffer.Len() > 0 {
			tool.OnInputDelta(ToolInputDeltaEvent{
				ToolName:       st.name,
				ToolCallID:     st.id,
				ToolCallIndex:  tc.Index,
				InputTextDelta: st.buffer.String(),
			})
		}
	}

	if tool.OnInputDelta != nil && tc.ArgumentsDelta != "" {
		tool.OnInputDelta(ToolInputDeltaEvent{
			ToolName:       st.name,
			ToolCallID:     st.id,
			ToolCallIndex:  tc.Index,
			InputTextDelta: tc.ArgumentsDelta,
		})
	}
}

func (l *toolInputLifecycle) onInputAvailable(tool Tool, call provider.ToolCallPart, toolCallIndex int) {
	if tool.OnInputAvailable == nil {
		return
	}
	tool.OnInputAvailable(ToolInputAvailableEvent{
		ToolName:      tool.Name,
		ToolCallID:    call.ID,
		ToolCallIndex: toolCallIndex,
		Input:         append([]byte(nil), call.Args...),
	})
}
