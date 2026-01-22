package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/bitop-dev/ai/pkg/mcp"
)

func main() {
	endpoint := flag.String("url", "", "MCP HTTP endpoint URL")
	token := flag.String("token", os.Getenv("MCP_TOKEN"), "bearer token for Authorization header")
	toolName := flag.String("tool", "", "tool name to call")
	toolArgs := flag.String("tool-args", "{}", "JSON object with tool arguments")
	flag.Parse()

	if *endpoint == "" {
		log.Fatal("missing -url")
	}

	headers := map[string]string{}
	if *token != "" {
		headers["Authorization"] = "Bearer " + *token
	}

	ctx := context.Background()
	transport := mcp.NewHTTPTransport(mcp.HTTPConfig{
		URL:     *endpoint,
		Headers: headers,
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
