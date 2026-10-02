package util

import (
	"bytes"
	"encoding/json"
)

// NormalizeJSONValue converts a value to a JSON-compatible tree, preserving
// numbers as json.Number rather than converting them to float64.
func NormalizeJSONValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var normalized any
	err = decoder.Decode(&normalized)
	return normalized, err
}
