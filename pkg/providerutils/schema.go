package providerutils

import (
	"context"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

// ValidationResult captures schema validation output.
type ValidationResult struct {
	Value provider.JSONValue
	Err   error
}

// Success reports whether validation succeeded.
func (result ValidationResult) Success() bool {
	return result.Err == nil
}

// SchemaValidator validates JSON values against a schema.
type SchemaValidator interface {
	Validate(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error
}

// SchemaValidatorFunc adapts a function to a SchemaValidator.
type SchemaValidatorFunc func(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error

// Validate invokes the validator function.
func (validator SchemaValidatorFunc) Validate(ctx context.Context, schema provider.JSONSchema, value provider.JSONValue) error {
	return validator(ctx, schema, value)
}

// Schema wraps a JSON schema with optional validation.
type Schema struct {
	JSONSchema provider.JSONSchema
	Validator  SchemaValidator
}

// NewSchema builds a Schema wrapper for a JSON schema payload.
func NewSchema(jsonSchema provider.JSONSchema) Schema {
	return Schema{JSONSchema: jsonSchema}
}

// Validate validates a JSON value using the configured validator.
func (schema Schema) Validate(ctx context.Context, value provider.JSONValue) ValidationResult {
	if schema.Validator == nil {
		return ValidationResult{Value: value}
	}

	if err := schema.Validator.Validate(ctx, schema.JSONSchema, value); err != nil {
		return ValidationResult{Err: err}
	}

	return ValidationResult{Value: value}
}

// StrictJSONSchema forces object schemas to disallow additional properties.
func StrictJSONSchema(schema provider.JSONSchema) provider.JSONSchema {
	return applyAdditionalProperties(schema, false, true)
}

// LenientJSONSchema ensures object schemas allow additional properties by default.
func LenientJSONSchema(schema provider.JSONSchema) provider.JSONSchema {
	return applyAdditionalProperties(schema, true, false)
}

func applyAdditionalProperties(schema provider.JSONSchema, allow bool, override bool) provider.JSONSchema {
	if schema == nil {
		return schema
	}

	switch typed := schema.(type) {
	case provider.JSONObject:
		if !isObjectSchema(typed) {
			return schema
		}
		if !override {
			if _, ok := typed["additionalProperties"]; ok {
				return schema
			}
		}
		clone := make(provider.JSONObject, len(typed)+1)
		for key, value := range typed {
			clone[key] = value
		}
		clone["additionalProperties"] = allow
		return clone
	case map[string]any:
		if !isObjectSchemaMap(typed) {
			return schema
		}
		if !override {
			if _, ok := typed["additionalProperties"]; ok {
				return schema
			}
		}
		clone := make(map[string]any, len(typed)+1)
		for key, value := range typed {
			clone[key] = value
		}
		clone["additionalProperties"] = allow
		return clone
	default:
		return schema
	}
}

func isObjectSchema(schema provider.JSONObject) bool {
	if isObjectType(schema["type"]) {
		return true
	}
	_, ok := schema["properties"]
	return ok
}

func isObjectSchemaMap(schema map[string]any) bool {
	if isObjectType(schema["type"]) {
		return true
	}
	_, ok := schema["properties"]
	return ok
}

func isObjectType(value provider.JSONValue) bool {
	switch typed := value.(type) {
	case string:
		return typed == "object"
	case []string:
		for _, entry := range typed {
			if entry == "object" {
				return true
			}
		}
	case []any:
		for _, entry := range typed {
			if text, ok := entry.(string); ok && text == "object" {
				return true
			}
		}
	}
	return false
}
