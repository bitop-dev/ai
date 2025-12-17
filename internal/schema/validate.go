package schema

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func Validate(schemaJSON json.RawMessage, raw json.RawMessage) error {
	if len(schemaJSON) == 0 {
		return nil
	}
	if len(raw) == 0 {
		return fmt.Errorf("empty json")
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", bytes.NewReader(schemaJSON)); err != nil {
		return fmt.Errorf("schema resource: %w", err)
	}
	s, err := c.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}
	return s.Validate(doc)
}
