package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalVars_Struct(t *testing.T) {
	type server struct {
		Host string   `json:"host"`
		Port int      `json:"port"`
		Tags []string `json:"tags"`
	}

	obj := server{
		Host: "{{host}}",
		Port: 8080,
		Tags: []string{"{{env}}", "static"},
	}
	vars := map[string]string{
		"host": "example.com",
		"env":  "prod",
	}

	err := EvalVars(&obj, vars)
	require.NoError(t, err)

	// obj is mutated in place, with substitutions applied.
	assert.Equal(t, server{
		Host: "example.com",
		Port: 8080,
		Tags: []string{"prod", "static"},
	}, obj)
}

func TestEvalVars_Map(t *testing.T) {
	obj := map[string]string{
		"a": "{{one}}",
		"b": "value-{{two}}",
	}
	vars := map[string]string{
		"one": "1",
		"two": "2",
	}

	err := EvalVars(&obj, vars)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"a": "1",
		"b": "value-2",
	}, obj)
}

func TestEvalVars_MultipleOccurrences(t *testing.T) {
	obj := map[string]string{
		"path": "{{root}}/sub/{{root}}",
	}
	vars := map[string]string{"root": "/data"}

	err := EvalVars(&obj, vars)
	require.NoError(t, err)
	assert.Equal(t, "/data/sub//data", obj["path"])
}

func TestEvalVars_MissingVarLeftIntact(t *testing.T) {
	obj := map[string]string{"x": "{{unknown}}"}

	err := EvalVars(&obj, map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, "{{unknown}}", obj["x"])
}

func TestEvalVars_NilVars(t *testing.T) {
	obj := map[string]string{"x": "{{a}}", "y": "plain"}

	err := EvalVars(&obj, nil)
	require.NoError(t, err)
	assert.Equal(t, "{{a}}", obj["x"])
	assert.Equal(t, "plain", obj["y"])
}

func TestEvalVars_Nested(t *testing.T) {
	type inner struct {
		Name string `json:"name"`
	}
	type outer struct {
		Inner  inner             `json:"inner"`
		Labels map[string]string `json:"labels"`
	}

	obj := outer{
		Inner:  inner{Name: "{{name}}"},
		Labels: map[string]string{"region": "{{region}}"},
	}
	vars := map[string]string{
		"name":   "node-1",
		"region": "eu",
	}

	err := EvalVars(&obj, vars)
	require.NoError(t, err)
	assert.Equal(t, "node-1", obj.Inner.Name)
	assert.Equal(t, "eu", obj.Labels["region"])
}

func TestEvalVars_NonStringValuesUnaffected(t *testing.T) {
	type cfg struct {
		Count   int  `json:"count"`
		Enabled bool `json:"enabled"`
	}

	obj := cfg{Count: 42, Enabled: true}

	err := EvalVars(&obj, map[string]string{"count": "999"})
	require.NoError(t, err)
	assert.Equal(t, cfg{Count: 42, Enabled: true}, obj)
}
