package providerutils

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestSchemaValidateUsesValidator(t *testing.T) {
	validator := SchemaValidatorFunc(func(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error {
		if ctx == nil {
			return errors.New("missing context")
		}
		if schema == nil {
			return errors.New("missing schema")
		}
		payload, ok := value.(provider.JSONObject)
		if !ok {
			return errors.New("unexpected value type")
		}
		if payload["name"] != "Ada" {
			return errors.New("unexpected name")
		}
		return nil
	})

	schema := Schema{
		JSONSchema: provider.JSONObject{"type": "object"},
		Validator:  validator,
	}

	result := schema.Validate(context.Background(), provider.JSONObject{"name": "Ada"})
	if !result.Success() {
		t.Fatalf("expected validation success, got %v", result.Err)
	}
	if result.Value == nil {
		t.Fatalf("expected validated value to be returned")
	}
}

func TestSchemaValidateReturnsError(t *testing.T) {
	boom := errors.New("bad")
	schema := Schema{
		JSONSchema: provider.JSONObject{"type": "object"},
		Validator: SchemaValidatorFunc(func(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error {
			return boom
		}),
	}

	result := schema.Validate(context.Background(), provider.JSONObject{})
	if result.Err != boom {
		t.Fatalf("expected validation error")
	}
	if result.Success() {
		t.Fatalf("expected validation failure")
	}
}

func TestSchemaValidateWithoutValidator(t *testing.T) {
	schema := NewSchema(provider.JSONObject{"type": "object"})
	value := provider.JSONObject{"ok": true}

	result := schema.Validate(context.Background(), value)
	if !result.Success() {
		t.Fatalf("expected validation success")
	}
	if !reflect.DeepEqual(result.Value, value) {
		t.Fatalf("expected value to pass through")
	}
}

func TestStrictJSONSchemaAddsAdditionalProperties(t *testing.T) {
	base := provider.JSONObject{
		"type": "object",
		"properties": provider.JSONObject{
			"name": provider.JSONObject{"type": "string"},
		},
	}

	strict := StrictJSONSchema(base)
	strictObj, ok := strict.(provider.JSONObject)
	if !ok {
		t.Fatalf("expected object schema")
	}
	if strictObj["additionalProperties"] != false {
		t.Fatalf("expected strict additionalProperties")
	}
	if _, ok := base["additionalProperties"]; ok {
		t.Fatalf("expected base schema to remain unchanged")
	}
}

func TestLenientJSONSchemaSetsAdditionalPropertiesWhenMissing(t *testing.T) {
	base := provider.JSONObject{"type": "object"}
	lenient := LenientJSONSchema(base)
	lenientObj, ok := lenient.(provider.JSONObject)
	if !ok {
		t.Fatalf("expected object schema")
	}
	if lenientObj["additionalProperties"] != true {
		t.Fatalf("expected lenient additionalProperties")
	}
}

func TestLenientJSONSchemaPreservesExistingAdditionalProperties(t *testing.T) {
	base := provider.JSONObject{
		"type":                 "object",
		"additionalProperties": false,
	}
	lenient := LenientJSONSchema(base)
	lenientObj, ok := lenient.(provider.JSONObject)
	if !ok {
		t.Fatalf("expected object schema")
	}
	if lenientObj["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties to be preserved")
	}
}
