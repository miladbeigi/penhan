package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ParseFile reads a YAML or JSON file and returns the key-value pairs.
func ParseFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return Parse(data, filepath.Ext(path))
}

// Parse decodes YAML or JSON content (selected by ext) into key-value pairs.
func Parse(data []byte, ext string) (map[string]string, error) {
	switch ext {
	case ".yaml", ".yml":
		return parseYAML(data)
	case ".json":
		return parseJSON(data)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

func parseYAML(data []byte) (map[string]string, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	return toStringMap(raw)
}

func parseJSON(data []byte) (map[string]string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return toStringMap(raw)
}

func toStringMap(raw map[string]interface{}) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range raw {
		switch v.(type) {
		case map[string]interface{}, map[interface{}]interface{}, []interface{}:
			// Stringifying structure would corrupt the secret ("map[a:1]").
			return nil, fmt.Errorf("secret key %q has a nested value; only flat key-value pairs are supported", k)
		}
		result[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}
