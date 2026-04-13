package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// resolveIncludes reads a YAML file and recursively resolves !include tags.
// Included file paths are resolved relative to the directory of the file containing the tag.
func resolveIncludes(filePath string) ([]byte, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", filePath, err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", absPath, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", absPath, err)
	}

	baseDir := filepath.Dir(absPath)
	if err := resolveNode(&doc, baseDir); err != nil {
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
