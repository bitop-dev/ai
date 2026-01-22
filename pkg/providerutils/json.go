package providerutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

// JSONParseError reports failures when parsing JSON text.
type JSONParseError struct {
	Text  string
	Cause error
}

func (err *JSONParseError) Error() string {
	if err == nil {
		return ""
	}
	if err.Text != "" {
		return fmt.Sprintf("json parsing failed: text: %s", err.Text)
	}
	if err.Cause != nil {
		return fmt.Sprintf("json parsing failed: %v", err.Cause)
	}
	return "json parsing failed"
}

func (err *JSONParseError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// JSONValidationError reports validation failures for parsed JSON values.
type JSONValidationError struct {
	Cause    error
	RawValue provider.JSONValue
	Schema   provider.JSONSchema
}

func (err *JSONValidationError) Error() string {
	if err == nil {
		return ""
	}
	if err.Cause != nil {
		return fmt.Sprintf("json validation failed: %v", err.Cause)
	}
	return "json validation failed"
}

func (err *JSONValidationError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// ParseOptions configures JSON parsing helpers.
type ParseOptions struct {
	Text   string
	Schema *Schema
}

// ParseResult captures JSON parsing output with raw values preserved.
type ParseResult struct {
	Value    provider.JSONValue
	RawValue provider.JSONValue
	Err      error
}

// Success reports whether parsing succeeded.
func (result ParseResult) Success() bool {
	return result.Err == nil
}

// ParseJSON parses JSON text and optionally validates it against a schema.
func ParseJSON(options ParseOptions) (provider.JSONValue, error) {
	value, err := parseJSONValue(options.Text)
	if err != nil {
		return nil, err
	}

	if options.Schema == nil || options.Schema.Validator == nil {
		return value, nil
	}

	validation := options.Schema.Validate(context.Background(), value)
	if validation.Err != nil {
		return nil, &JSONValidationError{Cause: validation.Err, RawValue: value, Schema: options.Schema.JSONSchema}
	}

	return validation.Value, nil
}

// SafeParseJSON parses JSON text and returns a structured result.
func SafeParseJSON(options ParseOptions) ParseResult {
	value, err := parseJSONValue(options.Text)
	if err != nil {
		return ParseResult{Err: err}
	}

	if options.Schema == nil || options.Schema.Validator == nil {
		return ParseResult{Value: value, RawValue: value}
	}

	validation := options.Schema.Validate(context.Background(), value)
	if validation.Err != nil {
		return ParseResult{
			Err:      &JSONValidationError{Cause: validation.Err, RawValue: value, Schema: options.Schema.JSONSchema},
			RawValue: value,
		}
	}

	return ParseResult{Value: validation.Value, RawValue: value}
}

// IsParsableJSON reports whether input is valid JSON.
func IsParsableJSON(input string) bool {
	_, err := parseJSONValue(input)
	return err == nil
}

// ToJSONValue converts arbitrary data into a JSONValue.
func ToJSONValue(value any) (provider.JSONValue, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case bool:
		return typed, nil
	case string:
		return typed, nil
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return typed, nil
	case int64:
		return typed, nil
	case int32:
		return int64(typed), nil
	case uint:
		return typed, nil
	case uint64:
		return typed, nil
	case uint32:
		return uint64(typed), nil
	case json.Number:
		if intValue, err := typed.Int64(); err == nil {
			return intValue, nil
		}
		if floatValue, err := typed.Float64(); err == nil {
			return floatValue, nil
		}
		return nil, fmt.Errorf("invalid json number %q", typed)
	case provider.JSONObject:
		return convertJSONObject(typed)
	case map[string]provider.JSONValue:
		return convertJSONValueMap(typed)
	case map[string]any:
		return convertAnyMap(typed)
	case provider.JSONArray:
		return convertJSONArray(typed)
	case []provider.JSONValue:
		return convertJSONValueSlice(typed)
	case []any:
		return convertAnySlice(typed)
	default:
		return nil, fmt.Errorf("unsupported json value type %T", value)
	}
}

func parseJSONValue(text string) (provider.JSONValue, error) {
	if strings.TrimSpace(text) == "" {
		return nil, &JSONParseError{Text: text, Cause: errors.New("empty json")}
	}

	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()

	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return nil, &JSONParseError{Text: text, Cause: err}
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected trailing data")
		}
		return nil, &JSONParseError{Text: text, Cause: err}
	}

	value, err := ToJSONValue(raw)
	if err != nil {
		return nil, &JSONParseError{Text: text, Cause: err}
	}

	return value, nil
}

func convertJSONObject(value provider.JSONObject) (provider.JSONValue, error) {
	converted := make(provider.JSONObject, len(value))
	for key, entry := range value {
		parsed, err := ToJSONValue(entry)
		if err != nil {
			return nil, err
		}
		converted[key] = parsed
	}
	return converted, nil
}

func convertJSONValueMap(value map[string]provider.JSONValue) (provider.JSONValue, error) {
	converted := make(provider.JSONObject, len(value))
	for key, entry := range value {
		parsed, err := ToJSONValue(entry)
		if err != nil {
			return nil, err
		}
		converted[key] = parsed
	}
	return converted, nil
}

func convertAnyMap(value map[string]any) (provider.JSONValue, error) {
	converted := make(provider.JSONObject, len(value))
	for key, entry := range value {
		parsed, err := ToJSONValue(entry)
		if err != nil {
			return nil, err
		}
		converted[key] = parsed
	}
	return converted, nil
}

func convertJSONArray(value provider.JSONArray) (provider.JSONValue, error) {
	converted := make(provider.JSONArray, len(value))
	for index, entry := range value {
		parsed, err := ToJSONValue(entry)
		if err != nil {
			return nil, err
		}
		converted[index] = parsed
	}
	return converted, nil
}

func convertJSONValueSlice(value []provider.JSONValue) (provider.JSONValue, error) {
	converted := make(provider.JSONArray, len(value))
	for index, entry := range value {
		parsed, err := ToJSONValue(entry)
		if err != nil {
			return nil, err
		}
		converted[index] = parsed
	}
	return converted, nil
}

func convertAnySlice(value []any) (provider.JSONValue, error) {
	converted := make(provider.JSONArray, len(value))
	for index, entry := range value {
		parsed, err := ToJSONValue(entry)
		if err != nil {
			return nil, err
		}
		converted[index] = parsed
	}
	return converted, nil
}
