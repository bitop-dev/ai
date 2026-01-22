package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/mcp"
)

func main() {
	command := flag.String("command", "", "path to MCP server command")
	args := flag.String("args", "", "command arguments")
	toolName := flag.String("tool", "", "tool name to call")
	toolArgs := flag.String("tool-args", "{}", "JSON object with tool arguments")
	flag.Parse()

	if *command == "" {
		log.Fatal("missing -command")
	}

	ctx := context.Background()
	transport := mcp.NewStdioTransport(mcp.StdioConfig{
		Command: *command,
		Args:    strings.Fields(*args),
		Stderr:  os.Stderr,
	})

	client, err := mcp.CreateClient(ctx, mcp.ClientConfig{Transport: transport})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("client close: %v", err)
		}
	}()

	tools, err := client.ListTools(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("mcp tools available: %d", len(tools.Tools))
	for _, tool := range tools.Tools {
		log.Printf("- %s", tool.Name)
	}

	if *toolName == "" {
		return
	}

	var argsPayload map[string]any
	if err := json.Unmarshal([]byte(*toolArgs), &argsPayload); err != nil {
		log.Fatalf("parse tool args: %v", err)
	}

	result, err := client.CallTool(ctx, *toolName, argsPayload)
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tool result: %s", string(encoded))
}
