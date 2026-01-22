package ai

import "context"

// StopCondition determines when an agent should stop after a step.
type StopCondition func(ctx context.Context, steps []StepResult) (bool, error)

// StepCountIs stops after the specified number of steps.
func StepCountIs(stepCount int) StopCondition {
	return func(_ context.Context, steps []StepResult) (bool, error) {
		return len(steps) >= stepCount, nil
	}
}

// HasToolCall stops when the last step contains a tool call with the given name.
func HasToolCall(toolName string) StopCondition {
	return func(_ context.Context, steps []StepResult) (bool, error) {
		if len(steps) == 0 {
			return false, nil
		}
		for _, call := range steps[len(steps)-1].ToolCalls {
			if call.Name == toolName {
				return true, nil
			}
		}
		return false, nil
	}
}

// IsStopConditionMet evaluates stop conditions for the current steps.
func IsStopConditionMet(ctx context.Context, conditions []StopCondition, steps []StepResult) (bool, error) {
	for _, condition := range conditions {
		met, err := condition(ctx, steps)
		if err != nil {
			return false, err
		}
		if met {
			return true, nil
		}
	}
	return false, nil
}
