package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/subosito/gotenv"
	"gopkg.in/yaml.v3"
)

// resolveConfig reads YAML from r and recursively resolves !include tags and expands env variables.
// Included file paths are resolved relative to baseDir.
func resolveConfig(r io.Reader, baseDir string) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base directory %s: %w", baseDir, err)
	}

	dotEnv, err := readDotEnv(absBaseDir)
	if err != nil {
		return nil, err
	}

	if err := expandEnvNode(&doc, dotEnv); err != nil {
		return nil, err
	}

	if err := resolveNode(&doc, absBaseDir, dotEnv); err != nil {
		return nil, err
	}

	return yaml.Marshal(&doc)
}

// readDotEnv reads .env file found in dir.
func readDotEnv(dir string) (map[string]string, error) {
	path := filepath.Join(dir, ".env")
	env, err := gotenv.Read(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return env, nil
}

// expandEnvNode recursively expands ${VAR}/$VAR references found in scalar node values.
// env variables take precedence over the ones defined in dotEnv.
func expandEnvNode(node *yaml.Node, dotEnv map[string]string) error {
	var missingEnvVars []string

	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil {
			return
		}
		if n.Kind == yaml.ScalarNode {
			n.Value = os.Expand(n.Value, func(key string) string {
				val, ok := os.LookupEnv(key)
				if !ok {
					val, ok = dotEnv[key]
				}
				if !ok {
					missingEnvVars = append(missingEnvVars, key)
				}
				return val
			})
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(node)

	if len(missingEnvVars) > 0 {
		return fmt.Errorf("missing env variables %s", missingEnvVars)
	}
	return nil
}

func resolveNode(node *yaml.Node, baseDir string, dotEnv map[string]string) error {
	if node == nil {
		return nil
	}

	if node.Tag == "!include" {
		return resolveIncludeNode(node, baseDir, dotEnv)
	}

	for _, child := range node.Content {
		if err := resolveNode(child, baseDir, dotEnv); err != nil {
			return err
		}
	}

	return nil
}

func resolveIncludeNode(node *yaml.Node, baseDir string, dotEnv map[string]string) error {
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

	if err := expandEnvNode(&included, dotEnv); err != nil {
		return fmt.Errorf("included file %s: %w", absPath, err)
	}

	includeDir := filepath.Dir(absPath)
	if err := resolveNode(&included, includeDir, dotEnv); err != nil {
		return err
	}

	if included.Kind == yaml.DocumentNode && len(included.Content) > 0 {
		*node = *included.Content[0]
	} else {
		*node = included
	}

	return nil
}
