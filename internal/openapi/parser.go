package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func Parse(data []byte) (*Document, error) {
	jsonBytes, err := toJSON(bytes.TrimSpace(data))
	if err != nil {
		return nil, err
	}
	return unmarshalDocument(jsonBytes)
}

func toJSON(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty document")
	}
	if data[0] == '{' {
		return data, nil
	}
	return yamlToJSON(data)
}

func yamlToJSON(data []byte) ([]byte, error) {
	var parsed any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	jsonBytes, err := json.Marshal(normaliseYAMLValue(parsed))
	if err != nil {
		return nil, fmt.Errorf("re-encode yaml as json: %w", err)
	}
	return jsonBytes, nil
}

func unmarshalDocument(jsonBytes []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		return nil, fmt.Errorf("unsupported OpenAPI version %q (only 3.x is supported)", doc.OpenAPI)
	}
	return &doc, nil
}

// normaliseYAMLValue converts map[interface{}]interface{} produced by the YAML decoder
// into map[string]interface{} so the value can be marshalled to JSON.
func normaliseYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[interface{}]interface{}:
		normalised := make(map[string]any, len(typed))
		for key, val := range typed {
			normalised[fmt.Sprint(key)] = normaliseYAMLValue(val)
		}
		return normalised
	case map[string]any:
		for key, val := range typed {
			typed[key] = normaliseYAMLValue(val)
		}
		return typed
	case []any:
		for index, val := range typed {
			typed[index] = normaliseYAMLValue(val)
		}
		return typed
	default:
		return value
	}
}

