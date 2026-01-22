package provider

// JSONValue represents a JSON-compatible value.
// Valid values include nil, bool, string, float64/int, JSONObject, and JSONArray.
type JSONValue interface{}

// JSONObject represents a JSON object.
type JSONObject map[string]JSONValue

// JSONArray represents a JSON array.
type JSONArray []JSONValue

// JSONSchema represents a JSON schema payload.
type JSONSchema = JSONValue
