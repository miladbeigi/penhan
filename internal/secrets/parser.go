package secrets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Parse decodes YAML or JSON content (selected by ext) into key-value pairs.
// Values are kept exactly as written: a secret like `pin: 0012` must stay
// "0012", not become the number 10.
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

// parseYAML reads the document as a node tree rather than into Go values, so
// each scalar's literal text is used instead of YAML's type resolution
// (octal and hex ints, floats, timestamps, null).
func parseYAML(data []byte) (map[string]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	result := make(map[string]string)
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return result, nil // empty file
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse yaml: expected a map of key-value pairs")
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i].Value, root.Content[i+1]
		if value.Kind == yaml.AliasNode {
			value = value.Alias
		}
		if value.Kind != yaml.ScalarNode {
			// Stringifying structure would corrupt the secret ("map[a:1]").
			return nil, fmt.Errorf("secret key %q has a nested value; only flat key-value pairs are supported", key)
		}
		if _, dup := result[key]; dup {
			return nil, fmt.Errorf("secret key %q is defined more than once", key)
		}
		if value.Tag == "!!null" {
			result[key] = ""
		} else {
			result[key] = value.Value
		}
	}
	return result, nil
}

func parseJSON(data []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // keep 12345678901234567890 and 1.10 as written
	var raw map[string]interface{}
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	result := make(map[string]string, len(raw))
	for k, v := range raw {
		switch v := v.(type) {
		case string:
			result[k] = v
		case json.Number:
			result[k] = v.String()
		case bool:
			result[k] = strconv.FormatBool(v)
		case nil:
			result[k] = ""
		default:
			return nil, fmt.Errorf("secret key %q has a nested value; only flat key-value pairs are supported", k)
		}
	}
	return result, nil
}
