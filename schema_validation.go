package ai

import (
	"encoding/json"

	internalSchema "github.com/bitop-dev/ai/internal/schema"
)

func validateJSONAgainstSchema(schema Schema, raw json.RawMessage) error {
	return internalSchema.Validate(schema.JSON, raw)
}
