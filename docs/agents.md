# Agents

Agents run multi-step tool loops on top of language models. The Go SDK provides
`ToolLoopAgent`, which repeatedly calls the model, executes tools, and continues
until the model stops or a stop condition is met.

## Quickstart

```go
agent := ai.NewToolLoopAgent(ai.ToolLoopAgentSettings[any]{
    Model: model,
    Instructions: "You are a concise assistant.",
    Tools: []providerutils.ToolDefinition{tool},
})

result, err := agent.Generate(ctx, ai.AgentCallOptions[any]{
    Prompt: "What time is it in Tokyo?",
})
if err != nil {
    log.Fatal(err)
}

log.Println(result.Text)
```

`ToolLoopAgent` automatically passes tool specifications to the underlying
provider using `ProviderOptions[providerID]["tools"]` when tools are present.

## Streaming

```go
stream, err := agent.Stream(ctx, ai.AgentStreamOptions[any]{
    Prompt: "Summarize the latest tool output.",
})
if err != nil {
    log.Fatal(err)
}
defer stream.Stream.Close()

for stream.Stream.Next() {
    part := stream.Stream.Value()
    _ = part
}
if err := stream.Stream.Err(); err != nil {
    log.Fatal(err)
}

final, err := stream.Collect()
if err != nil {
    log.Fatal(err)
}
_ = final
```

## Stop conditions

`ToolLoopAgent` defaults to `StepCountIs(20)`. You can customize with stop
conditions:

```go
settings := ai.ToolLoopAgentSettings[any]{
    Model: model,
    Tools: []providerutils.ToolDefinition{tool},
    StopWhen: []ai.StopCondition{
        ai.StepCountIs(10),
        ai.HasToolCall("handoff"),
    },
}
```

## Callbacks

Use callbacks for step-level telemetry:

```go
settings := ai.ToolLoopAgentSettings[any]{
    Model: model,
    Tools: []providerutils.ToolDefinition{tool},
    OnStepFinish: func(ctx context.Context, step ai.StepResult) {
        log.Printf("step text: %s", step.Text)
    },
    OnFinish: func(ctx context.Context, result ai.AgentResult) {
        log.Printf("total tokens: %d", result.TotalUsage.TotalTokens)
    },
}
```

## Call option preparation

Use `PrepareCall` to map typed call options to `TextOptions`:

```go
type MyOptions struct {
    Temperature float64
}

agent := ai.NewToolLoopAgent(ai.ToolLoopAgentSettings[MyOptions]{
    Model: model,
    PrepareCall: func(ctx context.Context, state ai.ToolLoopAgentPrepareCallState[MyOptions]) (ai.TextOptions, error) {
        options := state.TextOptions
        options.Temperature = &state.Options.Temperature
        return options, nil
    },
})

_, _ = agent.Generate(ctx, ai.AgentCallOptions[MyOptions]{
    Prompt: "Write a short summary.",
    Options: MyOptions{Temperature: 0.7},
})
```

## UI streams

The TypeScript SDK includes UI message streams for framework integrations.
The Go SDK does not include UI stream helpers; use `StreamText` or
`ToolLoopAgent.Stream` with SSE helpers if needed.
