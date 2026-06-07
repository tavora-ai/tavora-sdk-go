package fetchpolicy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FileConfig is the YAML shape of `expected-headers.yaml`. Top-level
// `expectedHeaders` maps name → template; everything else is ignored
// for forward-compat.
type FileConfig struct {
	ExpectedHeaders map[string]string `yaml:"expectedHeaders"`
}

// LoadFile reads `expected-headers.yaml` (or whatever path the
// caller supplies) and returns the parsed config. Missing file →
// (empty, nil). Malformed → (nil, err).
func LoadFile(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f FileConfig
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return f.ExpectedHeaders, nil
}
