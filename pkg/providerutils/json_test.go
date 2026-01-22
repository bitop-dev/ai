package providerutils

import (
	"context"
	"errors"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestParseJSONWithoutSchema(t *testing.T) {
	value, err := ParseJSON(ParseOptions{Text: "{\"foo\": \"bar\"}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, ok := value.(provider.JSONObject)
	if !ok {
		t.Fatalf("expected object, got %T", value)
	}
	if obj["foo"] != "bar" {
		t.Fatalf("unexpected value: %#v", obj)
	}
}

func TestParseJSONWithSchema(t *testing.T) {
	schema := Schema{
		JSONSchema: provider.JSONObject{"type": "object"},
		Validator: SchemaValidatorFunc(func(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error {
			obj, ok := value.(provider.JSONObject)
			if !ok {
				return errors.New("not object")
			}
			if obj["foo"] != "bar" {
				return errors.New("missing value")
			}
			return nil
		}),
	}

	value, err := ParseJSON(ParseOptions{Text: "{\"foo\": \"bar\"}", Schema: &schema})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := value.(provider.JSONObject)
	if obj["foo"] != "bar" {
		t.Fatalf("unexpected value: %#v", obj)
	}
}

func TestParseJSONInvalid(t *testing.T) {
	_, err := ParseJSON(ParseOptions{Text: "invalid json"})
	var parseErr *JSONParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected JSONParseError, got %v", err)
	}
}

func TestParseJSONValidationError(t *testing.T) {
	schema := Schema{
		JSONSchema: provider.JSONObject{"type": "object"},
		Validator: SchemaValidatorFunc(func(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error {
			return errors.New("invalid value")
		}),
	}

	_, err := ParseJSON(ParseOptions{Text: "{\"foo\": \"bar\"}", Schema: &schema})
	var validationErr *JSONValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected JSONValidationError, got %v", err)
	}
	if validationErr.RawValue == nil {
		t.Fatalf("expected raw value to be preserved")
	}
}

func TestSafeParseJSONSuccess(t *testing.T) {
	result := SafeParseJSON(ParseOptions{Text: "{\"foo\": \"bar\"}"})
	if !result.Success() {
		t.Fatalf("expected success, got %v", result.Err)
	}
	if result.RawValue == nil {
		t.Fatalf("expected raw value")
	}
}

func TestSafeParseJSONValidationError(t *testing.T) {
	schema := Schema{
		JSONSchema: provider.JSONObject{"type": "object"},
		Validator: SchemaValidatorFunc(func(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error {
			return errors.New("invalid")
		}),
	}

	result := SafeParseJSON(ParseOptions{Text: "{\"foo\": \"bar\"}", Schema: &schema})
	if result.Success() {
		t.Fatalf("expected failure")
	}
	if result.RawValue == nil {
		t.Fatalf("expected raw value to be preserved")
	}
	var validationErr *JSONValidationError
	if !errors.As(result.Err, &validationErr) {
		t.Fatalf("expected JSONValidationError, got %v", result.Err)
	}
}

func TestIsParsableJSON(t *testing.T) {
	if !IsParsableJSON("{\"foo\": \"bar\"}") {
		t.Fatalf("expected valid JSON to be parsable")
	}
	if IsParsableJSON("invalid") {
		t.Fatalf("expected invalid JSON to be rejected")
	}
}

func TestToJSONValueConvertsMap(t *testing.T) {
	value, err := ToJSONValue(map[string]any{"count": 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj := value.(provider.JSONObject)
	if obj["count"] != 2 {
		t.Fatalf("unexpected value: %#v", obj)
	}
}
