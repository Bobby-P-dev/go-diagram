package repository

import (
	"encoding/json"
	"fmt"
)

type nullableJSON struct{ target *json.RawMessage }

func (value nullableJSON) Scan(source any) error {
	switch encoded := source.(type) {
	case nil:
		*value.target = nil
	case []byte:
		*value.target = append(json.RawMessage(nil), encoded...)
	case string:
		*value.target = json.RawMessage(encoded)
	default:
		return fmt.Errorf("unexpected JSON database type %T", source)
	}
	return nil
}

func jsonParameter(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}
