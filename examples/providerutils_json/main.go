package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

func main() {
	validator := providerutils.SchemaValidatorFunc(func(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error {
		obj, ok := value.(provider.JSONObject)
		if !ok {
			return errors.New("expected object")
		}
		name, ok := obj["name"].(string)
		if !ok || name == "" {
			return errors.New("name is required")
		}
		return nil
	})

	schema := providerutils.Schema{
		JSONSchema: provider.JSONObject{"type": "object"},
		Validator:  validator,
	}

	result := providerutils.SafeParseJSON(providerutils.ParseOptions{
		Text:   `{"name": "Ada", "role": "engineer"}`,
		Schema: &schema,
	})
	if !result.Success() {
		log.Fatal(result.Err)
	}

	obj := result.Value.(provider.JSONObject)
	fmt.Printf("Hello %s!\n", obj["name"])
}
