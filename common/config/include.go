package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// resolveConfig reads YAML from r and recursively resolves !include tags and expands env variables.
// Included file paths are resolved relative to baseDir.
func resolveConfig(r io.Reader, baseDir string) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Expand env variables
	var missingEnvVars []string
	data = []byte(os.Expand(string(data), func(key string) string {
		val, ok := os.LookupEnv(key)
		if !ok {
			missingEnvVars = append(missingEnvVars, key)
		}
		return val
	}))
	if len(missingEnvVars) > 0 {
		return nil, fmt.Errorf("missing env variables %s", missingEnvVars)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base directory %s: %w", baseDir, err)
	}

	if err := resolveNode(&doc, absBaseDir); err != nil {
		return nil, err
	}

	return yaml.Marshal(&doc)
}

func resolveNode(node *yaml.Node, baseDir string) error {
	if node == nil {
		return nil
	}

	if node.Tag == "!include" {
		return resolveIncludeNode(node, baseDir)
	}

	for _, child := range node.Content {
		if err := resolveNode(child, baseDir); err != nil {
			return err
		}
	}

	return nil
}

func resolveIncludeNode(node *yaml.Node, baseDir string) error {
	includePath := filepath.Join(baseDir, node.Value)

	absPath, err := filepath.Abs(includePath)
	if err != nil {
		return fmt.Errorf("failed to resolve include path %s: %w", includePath, err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read included file %s: %w", absPath, err)
	}

	var included yaml.Node
	if err := yaml.Unmarshal(data, &included); err != nil {
		return fmt.Errorf("failed to parse included YAML %s: %w", absPath, err)
	}

	includeDir := filepath.Dir(absPath)
	if err := resolveNode(&included, includeDir); err != nil {
		return err
	}

	if included.Kind == yaml.DocumentNode && len(included.Content) > 0 {
		*node = *included.Content[0]
	} else {
		*node = included
	}

	return nil
}
