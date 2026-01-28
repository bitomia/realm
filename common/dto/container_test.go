package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBindMount_JSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		mount    BindMount
		expected string
	}{
		{
			name: "read-write mount",
			mount: BindMount{
				Source:      "/host/path",
				Destination: "/container/path",
				ReadOnly:    false,
			},
			expected: `{"source":"/host/path","destination":"/container/path"}`,
		},
		{
			name: "read-only mount",
			mount: BindMount{
				Source:      "/host/path",
				Destination: "/container/path",
				ReadOnly:    true,
			},
			expected: `{"source":"/host/path","destination":"/container/path","readonly":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.mount)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestBindMount_JSONDeserialization(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		expected BindMount
	}{
		{
			name:    "read-write mount without readonly field",
			jsonStr: `{"source":"/host/path","destination":"/container/path"}`,
			expected: BindMount{
				Source:      "/host/path",
				Destination: "/container/path",
				ReadOnly:    false,
			},
		},
		{
			name:    "read-only mount with readonly true",
			jsonStr: `{"source":"/host/path","destination":"/container/path","readonly":true}`,
			expected: BindMount{
				Source:      "/host/path",
				Destination: "/container/path",
				ReadOnly:    true,
			},
		},
		{
			name:    "read-write mount with explicit readonly false",
			jsonStr: `{"source":"/host/path","destination":"/container/path","readonly":false}`,
			expected: BindMount{
				Source:      "/host/path",
				Destination: "/container/path",
				ReadOnly:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mount BindMount
			err := json.Unmarshal([]byte(tt.jsonStr), &mount)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, mount)
		})
	}
}

func TestCreateContainerRequest_WithBindMounts(t *testing.T) {
	req := CreateContainerRequest{
		Image: "nginx:latest",
		BindMounts: []BindMount{
			{Source: "./data", Destination: "/app/data", ReadOnly: false},
			{Source: "/opt/config", Destination: "/opt/config", ReadOnly: true},
		},
		Env: []string{"FOO=bar"},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var decoded CreateContainerRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, req.Image, decoded.Image)
	assert.Len(t, decoded.BindMounts, 2)
	assert.Equal(t, "./data", decoded.BindMounts[0].Source)
	assert.Equal(t, "/app/data", decoded.BindMounts[0].Destination)
	assert.False(t, decoded.BindMounts[0].ReadOnly)
	assert.Equal(t, "/opt/config", decoded.BindMounts[1].Source)
	assert.Equal(t, "/opt/config", decoded.BindMounts[1].Destination)
	assert.True(t, decoded.BindMounts[1].ReadOnly)
}

func TestCreateContainerRequest_WithoutBindMounts(t *testing.T) {
	req := CreateContainerRequest{
		Image: "nginx:latest",
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var decoded CreateContainerRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Nil(t, decoded.BindMounts)
}
